package llvm

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A HOST STRUCT'S CELL (llvm.md §6b, ADR 0128 items 9–11).
//
// Its value is its leaves, in the record view's canonical order, and its
// reference is a region laid out by the struct's C definition (emit.Layout):
// `alloca [N x i8], align A`, zero filled, so padding is 0 and an encoding is
// a function of the record. Each leaf at offset o is written and read at its C
// type through `getelementptr i8, ptr %c, i64 o`, aligned min(A, o & −o): the
// region is A-aligned and o's lowest set bit is the largest power of two the
// field's address is known to keep (records-research §3.2's corollary), so a
// larger `align` would be a claim no proof stands behind.

// isCStruct reports a C type with a layout.
func (p *printer) isCStruct(ty string) bool {
	if ty == "" {
		return false
	}
	_, _, ok := p.tg.CStruct(ty)
	return ok
}

// leafAlign is min(A, o & −o), A when o is 0.
func leafAlign(a, o int64) int64 {
	if o == 0 {
		return a
	}
	if lb := o & -o; lb < a {
		return lb
	}
	return a
}

// PlantStructUnread reads a struct cell's leaves from a fresh zeroed region,
// as if the host's write were lost: TestAStructIsWrittenByTheHost must see
// GetClientRect's desktop come back 0 by 0.
var PlantStructUnread bool

// PlantLeafAlignNatural states each leaf's natural alignment, its size, where
// the layout proves only min(A, o & −o): FINDTEXTEXA's pointer at offset 8
// under pack(4) would claim 8.
var PlantLeafAlignNatural bool

// structCell prints a struct cell's statement, or reports it is not one.
func (p *printer) structCell(s *ir.Stmt) bool {
	switch {
	case (s.Op == ir.OCellRef || s.Op == ir.OCellFresh) && p.isCStruct(s.Type):
		lay, err := p.tg.Layout(s.Type)
		if err != nil {
			p.fail("%v", err)
			return true
		}
		c := "%" + p.fresh("st")
		p.allocas = append(p.allocas, fmt.Sprintf("  %s = alloca [%d x i8], align %d", c, lay.Size, lay.Align))
		p.line("store [%d x i8] zeroinitializer, ptr %s, align %d", lay.Size, c, lay.Align)
		if s.Op == ir.OCellRef {
			if len(s.Args) != len(lay.Leaves) {
				p.fail("a struct cell written with %d values, and %s has %d leaves", len(s.Args), core.ShowSet(s.Type), len(lay.Leaves))
				return true
			}
			for j, lf := range lay.Leaves {
				v := p.val(s.Args[j])
				if lf.Trunc {
					r := "%" + p.fresh("c")
					p.line("%s = trunc i64 %s to %s", r, v, lf.LL)
					v = r
				}
				g := "%" + p.fresh("f")
				p.line("%s = getelementptr i8, ptr %s, i64 %d", g, c, lf.Offset)
				p.line("store %s %s, ptr %s, align %d", lf.LL, v, g, p.alignOf(lay.Align, lf))
			}
		}
		p.line("%%v%d = getelementptr i8, ptr %s, i64 0", s.Res[0], c)
		return true
	case s.Op == ir.OCellGet && s.Lit != nil:
		st := core.CellElem(p.f.Types[p.pl.Res(s.Args[0])])
		lay, err := p.tg.Layout(st)
		if err != nil {
			p.fail("%v", err)
			return true
		}
		j := int(s.Lit.Int)
		if j < 0 || j >= len(lay.Leaves) {
			p.fail("leaf %d of %s, which has %d", j, core.ShowSet(st), len(lay.Leaves))
			return true
		}
		lf := lay.Leaves[j]
		from := p.val(s.Args[0])
		if PlantStructUnread {
			// the planted fault: a fresh zeroed region, not the one the host wrote
			from = "%" + p.fresh("lost")
			p.allocas = append(p.allocas, fmt.Sprintf("  %s = alloca [%d x i8], align %d", from, lay.Size, lay.Align))
			p.line("store [%d x i8] zeroinitializer, ptr %s, align %d", lay.Size, from, lay.Align)
		}
		g := "%" + p.fresh("f")
		p.line("%s = getelementptr i8, ptr %s, i64 %d", g, from, lf.Offset)
		if !lf.Trunc {
			p.line("%%v%d = load %s, ptr %s, align %d", s.Res[0], lf.LL, g, p.alignOf(lay.Align, lf))
			return true
		}
		r := "%" + p.fresh("c")
		p.line("%s = load %s, ptr %s, align %d", r, lf.LL, g, p.alignOf(lay.Align, lf))
		ext := "zext"
		if lf.Signed {
			ext = "sext"
		}
		p.line("%%v%d = %s %s %s to i64", s.Res[0], ext, lf.LL, r)
		return true
	}
	return false
}

func (p *printer) alignOf(a int64, lf emit.LeafLayout) int64 {
	if PlantLeafAlignNatural {
		// the fault: the scalar's own size, as if every field kept it
		return map[string]int64{"i8": 1, "i16": 2, "i32": 4, "i64": 8, "ptr": 8}[lf.LL]
	}
	return leafAlign(a, lf.Offset)
}
