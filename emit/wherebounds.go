package emit

import (
	"math/big"

	"oroboros/core"
)

// A BOUND IS THE BINDER'S SET, WHATEVER ITS SPELLING (ADR 0062 item 2: bounds in
// binder sets, relations in `where`). A host declaration's `where` conjunct
// that bounds one parameter by a constant, lo ≤ x or x ≤ hi, says x ∈ [lo, hi],
// which is the claim `x ∈ lo..hi` states. When both ends are given, the pair is
// that parameter's range obligation (Bounds), decided as a ranged parameter's
// is: a literal, then the IR's intervals, then the refinement layer
// (DischargeRequires). The rest of the `where` stays, checked by the refinement
// layer as before. A one-sided bound stays in the `where`, as a definition's
// infinite end does (installContracts).
//
// The host's TYPE is unchanged, so the hand check still holds each parameter to
// the host's: what moves is how the bound is decided, not what the declaration
// claims. Read as a `where` only, the bound was invisible to the IR's intervals,
// which hold a loop's exit value exactly: `gos.Exit(status)` with status a loop
// variable in {0, 1} was refused (examples/os/wc.oro, wc-2026-10-24).
func (tg *Target) whereBounds() {
	for n, p := range tg.Prims {
		if p.Where == nil || len(p.Names) == 0 {
			continue
		}
		lo := make([]*big.Int, len(p.Names))
		hi := make([]*big.Int, len(p.Names))
		cs := whereConjuncts(p.Where)
		for _, c := range cs {
			i, l, h, ok := constantBound(c, p.Names)
			if !ok {
				continue
			}
			if l != nil && (lo[i] == nil || l.Cmp(lo[i]) > 0) {
				lo[i] = l
			}
			if h != nil && (hi[i] == nil || h.Cmp(hi[i]) < 0) {
				hi[i] = h
			}
		}
		moved := false
		bounds := make([]string, len(p.Args))
		for i := range p.Names {
			if i >= len(p.Args) || lo[i] == nil || hi[i] == nil {
				continue
			}
			l, h := lo[i], hi[i]
			if al, ah, ok := core.IntRangeBig(p.Args[i]); ok { // the declared set's own ends
				if al.Cmp(l) > 0 {
					l = al
				}
				if ah.Cmp(h) < 0 {
					h = ah
				}
			}
			bounds[i] = "int " + l.String() + " " + h.String()
			moved = true
		}
		if !moved {
			continue
		}
		// Keep every conjunct that is not one of the moved pairs.
		var kept []*core.Term
		for _, c := range cs {
			if i, _, _, ok := constantBound(c, p.Names); ok && i < len(bounds) && bounds[i] != "" {
				continue
			}
			kept = append(kept, c)
		}
		p.Bounds = bounds
		p.Where = conjoin(kept)
		tg.Prims[n] = p
	}
}

// whereConjuncts flattens a conjunction, (if a b false), into its conjuncts.
func whereConjuncts(t *core.Term) []*core.Term {
	if a, b, ok := erasedAnd(t); ok {
		return append(whereConjuncts(a), whereConjuncts(b)...)
	}
	return []*core.Term{t}
}

// conjoin is the conjunction of ts, nil when there is none.
func conjoin(ts []*core.Term) *core.Term {
	var out *core.Term
	for i := len(ts) - 1; i >= 0; i-- {
		if out == nil {
			out = ts[i]
		} else {
			out = core.App(core.Name("if"), ts[i], out, core.Bool(false))
		}
	}
	return out
}

// constantBound reads c as a bound on parameter names[i] by an integer literal:
// lo ≤ x, x ≥ lo, lo < x, x > lo (a lower end) or x ≤ hi, hi ≥ x, x < hi,
// hi > x (an upper end).
func constantBound(c *core.Term, names []string) (i int, lo, hi *big.Int, ok bool) {
	if c == nil || c.Kind != core.KApp || len(c.Kids) != 3 || c.Kids[0].Kind != core.KName {
		return 0, nil, nil, false
	}
	op, a, b := c.Kids[0].Name, c.Kids[1], c.Kids[2]
	param := func(t *core.Term) int {
		if t.Kind != core.KName {
			return -1
		}
		for j, n := range names {
			if n == t.Name {
				return j
			}
		}
		return -1
	}
	lit := func(t *core.Term) (*big.Int, bool) { return closedInt(t) }
	one := big.NewInt(1)
	if j := param(a); j >= 0 {
		if k, isLit := lit(b); isLit {
			switch {
			case isOp(op, "le"):
				return j, nil, k, true
			case isOp(op, "lt"):
				return j, nil, new(big.Int).Sub(k, one), true
			case isOp(op, "ge"):
				return j, k, nil, true
			case isOp(op, "gt"):
				return j, new(big.Int).Add(k, one), nil, true
			}
		}
	}
	if j := param(b); j >= 0 {
		if k, isLit := lit(a); isLit {
			switch {
			case isOp(op, "le"): // k ≤ x
				return j, k, nil, true
			case isOp(op, "lt"):
				return j, new(big.Int).Add(k, one), nil, true
			case isOp(op, "ge"): // k ≥ x
				return j, nil, k, true
			case isOp(op, "gt"):
				return j, nil, new(big.Int).Sub(k, one), true
			}
		}
	}
	return 0, nil, nil, false
}
