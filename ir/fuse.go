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
		case (d.Op == OAdd || d.Op == OSub || d.Op == OMul || d.Op == OAnd || d.Op == OOr || d.Op == OXor ||
			d.Op == OShl) && len(d.Args) == 2 && d.Name == "":
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
			case OAnd:
				return new(big.Int).And(a, b), true // math/big's are 2-adic
			case OOr:
				return new(big.Int).Or(a, b), true
			case OXor:
				return new(big.Int).Xor(a, b), true
			case OShl:
				if !b.IsInt64() || b.Int64() < 0 || b.Int64() > 128 {
					return nil, false
				}
				return new(big.Int).Lsh(a, uint(b.Int64())), true
			}
			return new(big.Int).Mul(a, b), true
		case (d.Op == OMod || d.Op == ODiv || d.Op == ORem || d.Op == OShr) && len(d.Args) == 2 && d.Name == "":
			a, ok1 := constant(d.Args[0])
			b, ok2 := constant(d.Args[1])
			if !ok1 || !ok2 || b.Sign() == 0 || d.Op == OMod && b.Sign() < 0 {
				return nil, false
			}
			switch d.Op {
			case OMod:
				return new(big.Int).Mod(a, b), true // Euclidean, as `mod` is
			case ODiv:
				return new(big.Int).Quo(a, b), true // truncating, as `/` is
			case ORem:
				return new(big.Int).Rem(a, b), true
			}
			if !b.IsInt64() || b.Int64() < 0 || b.Int64() > 128 {
				return nil, false
			}
			return new(big.Int).Rsh(a, uint(b.Int64())), true // a floor, as `>>` is
		case (d.Op == ONot || d.Op == ONeg) && len(d.Args) == 1 && d.Name == "":
			a, ok := constant(d.Args[0])
			if !ok {
				return nil, false
			}
			if d.Op == ONot {
				return new(big.Int).Not(a), true
			}
			return new(big.Int).Neg(a), true
		}
		return nil, false
	}
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
	// before holds the statements a rewrite inserts ahead of one, placed in one
	// rebuild per region at the end (an insertion moves a region's array).
	before := map[*Stmt][]Stmt{}
	// residue folds a closed constant subterm of a fused term to its residue at
	// the term's width: what the term reads of it (π_n ∘ π_W = π_n). Go refuses
	// a constant expression that overflows, so a wrapped operation of constants
	// is never printed. Signed at a wrap width, as the wrapping operators hold
	// their values; unsigned at the pair's 64 bits, a spine past int64.
	residue := func(d *Stmt, c *big.Int, width int64, signed bool) {
		m := new(big.Int).Lsh(big.NewInt(1), uint(width))
		r := new(big.Int).Mod(c, m)
		if signed && r.Cmp(new(big.Int).Rsh(m, 1)) >= 0 {
			r.Sub(r, m)
		}
		for _, a := range d.Args {
			deadChain(a) // the subterm's own operands, now unread
		}
		if r.IsInt64() {
			d.Op, d.Mode, d.Name, d.Args, d.Lit = OConst, MNone, "", nil, core.Int(r.Int64())
			return
		}
		e15 := big.NewInt(1_000_000_000_000_000)
		hi, lo := new(big.Int).QuoRem(r, e15, new(big.Int))
		k := func(c int64) V {
			v := V(len(f.Types))
			f.Types = append(f.Types, "int")
			before[d] = append(before[d], Stmt{Op: OConst, Lit: core.Int(c), Res: []V{v}})
			return v
		}
		prod := V(len(f.Types))
		f.Types = append(f.Types, "int")
		before[d] = append(before[d], Stmt{Op: OMul, Mode: MExact, Args: []V{k(hi.Int64()), k(e15.Int64())}, Res: []V{prod}})
		d.Op, d.Mode, d.Name, d.Args, d.Lit = OAdd, MExact, "", []V{prod, k(lo.Int64())}, nil
	}
	fused := 0
	// width and mode are the current `mod`'s: the target's W and mode wrap, or,
	// for 32 < n ≤ 64 on a target holding U as a pair, 64 and wrap64.
	width, mode := w, MWrap
	var mark func(v V) bool
	mark = func(v V) bool {
		d := def[v]
		if d == nil || uses[v] != 1 || d.Name != "" || d.Mode == MTrap || len(d.Res) != 1 {
			return false
		}
		if c, isConst := constant(v); isConst {
			// a literal is its own residue's source; a closed term of literals is
			// folded to its residue, read by the term as it is
			if d.Op != OConst {
				residue(d, c, width, mode == MWrap)
			}
			return false
		}
		switch d.Op {
		case OAdd, OSub, OMul, ONeg, OAnd, OOr, OXor, ONot:
		case OShl:
			// by a constant count below W: a wrapping shift by W or more is the
			// host's own business (JavaScript masks by 31, x86 by 63)
			k, ok := constant(d.Args[1])
			if !ok || k.Sign() < 0 || k.Cmp(big.NewInt(width)) >= 0 {
				return false
			}
		default:
			return false
		}
		d.Mode = mode
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
		kon  *big.Int // the residue, where the operand is a closed constant
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
			// A CLOSED CONSTANT'S RESIDUE is known when compiling (inlining puts
			// one under a `mod`: `(C * (0 + 1)) mod 2^64`), at any n.
			if c, ok := constant(s.Args[0]); ok {
				rws = append(rws, rewrite{s: s, n: n, kon: new(big.Int).Mod(c, m)})
				continue
			}
			width, mode = w, MWrap
			if n > w {
				if !tg.Word.Pair || n > 64 {
					continue // wider than the host wraps: computed exactly
				}
				width, mode = 64, MWrapPair // the pair's 64 bits (ir/pairs.go)
			}
			rws = append(rws, rewrite{s: s, n: n, root: mark(s.Args[0])})
		}
	})
	if len(rws) == 0 {
		return 0
	}
	// THE MOD ITSELF, and the divisor's constant term when nothing else reads it.
	// Statements are changed in place, and the masks' constants inserted and
	// the dead divisors removed in one rebuild per region, after: an insertion
	// moves a region's array, and every pointer above is into it.
	changed := false
	for _, rw := range rws {
		s := rw.s
		divisor, operand := s.Args[1], s.Args[0]
		switch {
		case rw.kon != nil:
			// the residue as a literal, or past int64 as the core holds one, a
			// spine hi·10¹⁵ + lo of words
			konst := func(c int64) V {
				v := V(len(f.Types))
				f.Types = append(f.Types, "int")
				before[s] = append(before[s], Stmt{Op: OConst, Lit: core.Int(c), Res: []V{v}})
				return v
			}
			if rw.kon.IsInt64() {
				s.Op, s.Mode, s.Name, s.Args, s.Lit = OConst, MNone, "", nil, core.Int(rw.kon.Int64())
			} else {
				e15 := big.NewInt(1_000_000_000_000_000)
				hi, lo := new(big.Int).QuoRem(rw.kon, e15, new(big.Int))
				prod := V(len(f.Types))
				f.Types = append(f.Types, "int")
				before[s] = append(before[s], Stmt{Op: OMul, Mode: s.Mode, Args: []V{konst(hi.Int64()), konst(e15.Int64())}, Res: []V{prod}})
				s.Op, s.Name, s.Args = OAdd, "", []V{prod, konst(lo.Int64())}
			}
			deadChain(operand) // the constant's own spine, now unread
		case rw.n <= 63:
			mask := V(len(f.Types))
			f.Types = append(f.Types, "int")
			lit := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(rw.n)), big.NewInt(1))
			before[s] = append(before[s], Stmt{Op: OConst, Lit: core.Int(lit.Int64()), Res: []V{mask}})
			s.Op, s.Name, s.Args = OAnd, "", []V{s.Args[0], mask}
		case rw.root && (tg.Word.Unsigned || tg.Word.Pair):
			s.Op, s.Mode, s.Name, s.Args = OCall, MNone, "u64-of", []V{s.Args[0]}
		default:
			continue
		}
		changed = true
		deadChain(divisor)
	}
	if !changed && len(before) == 0 && len(dead) == 0 {
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
			out = append(out, before[s]...)
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
