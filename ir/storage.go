package ir

import (
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
			out = append(out, Stmt{Op: OCall, Name: "alloc", Args: []V{count, wv}, Res: []V{p}, Src: s.Src, FromTerm: true})
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
