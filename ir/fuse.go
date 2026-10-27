package ir

import (
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// FUSION (ADR 0082 item 3, bits-research.md §1.5).
//
// π_n(x) = x mod 2ⁿ is at once a ring homomorphism ℤ → ℤ/2ⁿ and a Boolean
// homomorphism ℤ → 𝔽₂ⁿ, so for a term t built from +, −, ·, neg, &, |, ⊕, ~ and
// << by a constant,
//
//	π_n(t(x₁, …, x_k)) = π_n(t_W(x₁, …, x_k))   for any W ≥ n,
//
// where t_W computes each operation's W-bit residue (by induction on t, each
// step one of the two homomorphisms and π_n ∘ π_W = π_n). So the operations of
// such a term, each read only by the term, are computed in the target's
// wrapping operators (mode `wrap`, at its declared W), and their intermediates
// are exempt from the word: nothing observes them but the `mod`. Division,
// remainder, comparison, `>>`, `mod` and any call stop the term: none descends
// to ℤ/2ⁿ, and computing one wrapped is a wrong answer (TestFusionStopsAt…).
//
// The `mod 2ⁿ` itself becomes `& (2ⁿ − 1)`, the same function on every
// integer, while the mask fits the word (n ≤ 63); at n = 64, on a target that
// realizes the unsigned word, the residue map `u64-of`. Elsewhere it stays.
// Fuse reports how many operations it fused.
func Fuse(tg *emit.Target, f *Func) int {
	w := tg.WrapWidth
	if w == 0 {
		return 0
	}
	uses := map[V]int{}
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for _, a := range r.Args {
			uses[a]++
		}
		if r.T == TBranch {
			uses[r.Cond]++
		}
		for _, pi := range r.Pis {
			uses[pi.Of]++
			uses[pi.Other]++
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, a := range s.Args {
				uses[a]++
			}
			for _, v := range s.Res {
				def[v] = s
			}
		}
	})
	// constant is the value of a closed term of integer constants, which is how
	// the core holds a literal past int64 (`2^64` is 18446·10¹⁵ + 744…).
	var constant func(v V) (*big.Int, bool)
	constant = func(v V) (*big.Int, bool) {
		d := def[v]
		if d == nil {
			return nil, false
		}
		switch {
		case d.Op == OConst && d.Lit != nil && d.Lit.Kind == core.KInt:
			return big.NewInt(d.Lit.Int), true
		case (d.Op == OAdd || d.Op == OSub || d.Op == OMul) && len(d.Args) == 2 && d.Name == "":
			a, ok1 := constant(d.Args[0])
			b, ok2 := constant(d.Args[1])
			if !ok1 || !ok2 {
				return nil, false
			}
			switch d.Op {
			case OAdd:
				return new(big.Int).Add(a, b), true
			case OSub:
				return new(big.Int).Sub(a, b), true
			}
			return new(big.Int).Mul(a, b), true
		}
		return nil, false
	}
	fused := 0
	var mark func(v V) bool
	mark = func(v V) bool {
		d := def[v]
		if d == nil || uses[v] != 1 || d.Name != "" || d.Mode == MTrap || len(d.Res) != 1 {
			return false
		}
		if _, isConst := constant(v); isConst {
			return false // a literal is its own residue's source; π_n reads it as it is
		}
		switch d.Op {
		case OAdd, OSub, OMul, ONeg, OAnd, OOr, OXor, ONot:
		case OShl:
			// by a constant count below W: a wrapping shift by W or more is the
			// host's own business (JavaScript masks by 31, x86 by 63)
			k, ok := constant(d.Args[1])
			if !ok || k.Sign() < 0 || k.Cmp(big.NewInt(w)) >= 0 {
				return false
			}
		default:
			return false
		}
		d.Mode = MWrap
		fused++
		for j, a := range d.Args {
			if d.Op == OShl && j == 1 {
				continue
			}
			mark(a)
		}
		return true
	}
	type rewrite struct {
		s    *Stmt
		n    int64
		root bool
	}
	var rws []rewrite
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OMod || len(s.Args) != 2 || s.Name != "" {
				continue
			}
			m, ok := constant(s.Args[1])
			if !ok || m.Sign() <= 0 || m.BitLen() < 2 || new(big.Int).And(m, new(big.Int).Sub(m, big.NewInt(1))).Sign() != 0 {
				continue // not a power of two above 1
			}
			n := int64(m.BitLen() - 1)
			if n > w {
				continue // wider than the host wraps: computed exactly
			}
			rws = append(rws, rewrite{s: s, n: n, root: mark(s.Args[0])})
		}
	})
	if len(rws) == 0 {
		return 0
	}
	// THE MOD ITSELF, and the divisor's constant term when nothing else reads it.
	var dead []*Stmt
	var deadChain func(v V)
	deadChain = func(v V) {
		d := def[v]
		uses[v]--
		if d == nil || uses[v] > 0 {
			return
		}
		dead = append(dead, d)
		if d.Op != OConst {
			for _, a := range d.Args {
				deadChain(a)
			}
		}
	}
	// Statements are changed in place, and the masks' constants inserted and
	// the dead divisors removed in one rebuild per region, after: an insertion
	// moves a region's array, and every pointer above is into it.
	before := map[*Stmt]Stmt{}
	changed := false
	for _, rw := range rws {
		s := rw.s
		divisor := s.Args[1]
		switch {
		case rw.n <= 63:
			mask := V(len(f.Types))
			f.Types = append(f.Types, "int")
			lit := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(rw.n)), big.NewInt(1))
			before[s] = Stmt{Op: OConst, Lit: core.Int(lit.Int64()), Res: []V{mask}}
			s.Op, s.Name, s.Args = OAnd, "", []V{s.Args[0], mask}
		case rw.root && tg.Word.Unsigned:
			s.Op, s.Mode, s.Name, s.Args = OCall, MNone, "u64-of", []V{s.Args[0]}
		default:
			continue
		}
		changed = true
		deadChain(divisor)
	}
	if !changed {
		return fused
	}
	gone := map[*Stmt]bool{}
	for _, d := range dead {
		gone[d] = true
	}
	f.Walk(func(r *Region) {
		out := make([]Stmt, 0, len(r.Stmts)+1)
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if c, ok := before[s]; ok {
				out = append(out, c)
			}
			if !gone[s] {
				out = append(out, *s)
			}
		}
		r.Stmts = out
	})
	Canonicalize(f)
	f.facts = nil
	return fused
}
