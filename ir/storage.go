package ir

import (
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// STORAGE IS AN OPERATION OF THE IR (ADR 0092). On a target that realizes the
// allocator by a term, each statement that makes a table of n elements of w
// bytes is preceded by `p = alloc(n, w)`, and the statement takes p as its
// storage. ι then substitutes alloc's term (RealizeOps), and the decision
// decides its arithmetic, 8 + n·w, at the site: alloc : ℕ × {1, 8} → Region is
// total, a request past the word being exhaustion, the trap a null answer
// already is. The printer had computed the bytes itself, unchecked.
//
// w is ElemBytes, the one layout the printer indexes by, so the allocation and
// every access agree.

// PlantStorageFault allocates every table one byte an element, the layout
// disagreeing with the accesses: TestATablesStorageIsItsLayout must catch it.
var PlantStorageFault bool

// Storage is the pass, and how many tables it gave storage.
func Storage(tg *emit.Target, f *Func) int {
	if !termRealized(tg, "alloc") {
		return 0
	}
	// A TABLE'S STORAGE IS ITS FINAL LAYOUT (Theorem D′, final.go): its class's
	// representation, decided on the whole function, not the type the table
	// had before the classes were taken. Sized by the latter, readfile's buffer
	// was allocated 8 bytes an element and printed 1, and the caller, which
	// holds the result by its declared type, released it as 1: 8 191 bytes took
	// VirtualAlloc's region (8 191·8 + 8 = 2¹⁶) and gave it to HeapFree, which
	// faulted (files-2026-11-16). The classes are computed on a copy and every
	// table's type written back, so the allocation, every release, every token
	// and the printer read one layout.
	if !PlantStorageWidthIRA {
		finalTableTypes(tg, f)
	}
	n := 0
	fresh := func(ty string) V {
		v := V(len(f.Types))
		f.Types = append(f.Types, ty)
		return v
	}
	konst := func(out []Stmt, k int64) ([]Stmt, V) {
		c := fresh("int")
		return append(out, Stmt{Op: OConst, Lit: core.Int(k), Res: []V{c}, FromTerm: true}), c
	}
	f.Walk(func(r *Region) {
		var out []Stmt
		for _, s := range r.Stmts {
			var table, count V
			if len(s.Store) > 0 {
				out = append(out, s)
				continue
			}
			switch s.Op {
			case OBuild:
				table, count = s.Sub[0].Params[0], s.Args[0]
			case OTabulate:
				table, count = s.Res[0], s.Args[0]
			case OArray:
				table = s.Res[0]
				out, count = konst(out, int64(len(s.Args)))
			default:
				out = append(out, s)
				continue
			}
			w := int64(ElemBytes(tg, f.Types[table]))
			if PlantStorageFault {
				w = 1
			}

			var wv V
			out, wv = konst(out, w)
			p := fresh("ptr")
			// A TABLE BORN FULL takes a region with no zeroing (ir/fill.go):
			// a rule and an array literal by construction, a build by the
			// written-prefix theorem.
			name := "alloc"
			if termRealized(tg, "alloc-raw") && (s.Op != OBuild || fillDead(f, &s)) {
				name = "alloc-raw"
			}
			if name == "alloc-raw" {
				s.RawW = w
			}
			out = append(out, Stmt{Op: OCall, Name: name, Args: []V{count, wv}, Res: []V{p}, Src: s.Src, FromTerm: true})
			s.Store = []V{p}
			out = append(out, s)
			n++
		}
		r.Stmts = out
	})
	return n
}

// ElemBytes is how many bytes a table of type t holds each element in, the one
// layout the allocation and every access read (llvm.md §4): a byte for a
// Boolean and for a range the target holds in one, eight otherwise.
func ElemBytes(tg *emit.Target, t string) int {
	e := ElemOf(tg, t)
	if e == "bool" || tg.ValueType(e) == "bool" {
		return 1
	}
	if lo, hi, ok := core.IntRange(e); ok && tg.ReprBytes(lo, hi) == 1 {
		return 1
	}
	return 8
}

// PlantStorageWidthIRA sizes each table by its type before Theorem D′'s
// classes, as before files-2026-11-16: a kept function's byte table is then
// allocated 8 bytes an element and released by its caller as 1.
var PlantStorageWidthIRA bool

// finalTableTypes writes onto each table of f the type Finalize will give it:
// Theorem D′ computed on a copy of f, whose numbering is f's. A function
// Finalize would refuse keeps its types; the refusal is Finalize's to make.
//
// It reports whether it did: false where Finalize refuses the copy or the copy
// is numbered otherwise, and then the tables keep their first types.
func finalTableTypes(tg *emit.Target, f *Func) bool {
	c := f.Clone()
	c.facts = f.facts
	if err := finalizeFunc(tg, c); err != nil {
		return false
	}
	if len(c.Types) < len(f.Types) {
		return false // numbered otherwise, its indices not f's: the first types stand
	}
	// A PARAMETER'S TYPE IS ITS DECLARATION, the callers' contract (ADR 0101,
	// 0113): narrowing a kept definition's buffer parameter to its body's hull
	// made every caller owe it, and P4 inlined them. It is never a birth this
	// function stores or releases, so no width needs it.
	param := map[V]bool{}
	for _, x := range f.Params {
		param[x] = true
	}
	for v, t := range f.Types {
		if !param[V(v)] && (strings.HasPrefix(t, "array ") || strings.HasPrefix(t, "buffer ")) {
			f.Types[v] = c.Types[v]
		}
	}
	return true
}

// CheckStorageLayout is W15: every table Storage gave a raw region is laid
// out at a width the allocator wrote its zero byte for (ADR 0092, 0097,
// 0117). A raw region is zero only where something writes it, and the
// allocator writes it at n·w + 8 and n + 8, w the width it was asked: the
// table's final layout b, ElemBytes of its IR_P type, is fixed only after
// every pass, and narrows w (facts only refine), so b ∈ {1, w}. A layout wider
// than w is an allocation smaller than the table, refused. Run on the final
// function, after every round of storage and ι.
func CheckStorageLayout(tg *emit.Target, f *Func) error {
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if err != nil || s.RawW == 0 {
				continue
			}
			var table V = -1
			switch s.Op {
			case OBuild:
				if len(s.Sub) > 0 && len(s.Sub[0].Params) > 0 {
					table = s.Sub[0].Params[0]
				}
			case OTabulate, OArray:
				if len(s.Res) > 0 {
					table = s.Res[0]
				}
			}
			if table < 0 || int(table) >= len(f.Types) {
				continue
			}
			// the allocator writes the zero at n·RawW + 8 and at n + 8, so the
			// layout must be one of them; wider than asked is an overflow
			if b := int64(ElemBytes(tg, f.Types[table])); (b != s.RawW && b != 1) || PlantW15Strict && b != s.RawW {
				err = fmt.Errorf("W15: table %%%d (%s) took a raw region at %d bytes an element and its layout is %d: "+
					"the allocator's zero bytes are at n·%d + 8 and n + 8, and its elements past what it holds (ADR 0097, 0117)",
					table, f.Types[table], s.RawW, b, s.RawW)
			}
		}
	})
	return err
}

// PlantW15Strict asks the layout to equal the width exactly, without the
// allocator's second zero: TestARawRegionsZeroByteIsAtItsLayout must see the
// corpus's narrowed byte tables reported.
var PlantW15Strict bool
