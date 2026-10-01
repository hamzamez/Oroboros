package core

import "strconv"

// HYGIENE: A RESIDUAL'S NAMES MUST SAY WHAT ITS INDICES SAY.
//
// Reduction never opens a body, so inside the reducer a binder's parameter
// names are only HINTS and two binders may share one without harm — the
// indices decide what refers to what. But every consumer of the residual (the
// checker, the refiner, the interval pass, four backends) opens a lambda with
// `Body()`, which turns each index into its binder's hint. At that point a
// shared hint IS a capture, if the inner binder's body refers to the outer
// variable: both occurrences open to the same name and the outer one is read
// as the inner.
//
// β made exactly that shape. A comparator `(fn (a b) (str.Compare a b))`
// applied inside a merge sort whose buffers are hinted `a` and `b` let-binds
// its arguments (effects.md §7c — a table read does not move into an impure
// body), so the residual was
//
//	(let (cs (pat a n j)) (fn (a) (let (cs (pat a n i)) (fn (b) …))))
//
// correct by index, and the second value's `a` — the buffer — opened as the
// first value's STRING. The checker said "b is int, but string is required"
// (tally-2026-09-11). It is the `Body()`/`openFresh` family a fifth time,
// arriving from the reducer rather than from a pass.
//
// So the invariant is stated on the RESIDUAL, where every consumer meets it:
// no binder's hint equals a name its body refers to past it — through a loose
// index into an enclosing binder, or as a free name. hygienic renames exactly
// the binders that violate it and no others, so a program with no latent
// capture keeps every name it had and emits byte-identical code. Renaming a
// hint cannot change meaning: the indices are untouched.
//
// AND NO BINDER'S HINT EQUALS AN ENCLOSING BINDER'S, whether or not its body
// refers to that variable (bufio-2026-10-01). The first invariant keeps a TERM
// from capturing. A consumer may also carry a CONTEXT keyed by name: the
// refinement layer's facts are assumptions about names, Γ ⊢ φ, and a binder
// that reuses a name in Γ enters a scope whose assumptions are about another
// variable. With a parameter n declared in [0, 5] and a loop variable n, the
// loop's exit value was given 0 ≤ n ≤ 5 and an index of 101 into a table of 6
// was accepted, on every target. That is the variable convention (Barendregt
// 1984, 2.1.13) along the scope chain, and it is what makes weakening Γ by a
// binder sound without α-renaming the context.
func hygienic(t *Term) *Term { return hyg(t, nil) }

func hyg(t *Term, scope [][]string) *Term {
	switch t.Kind {
	case KFn:
		body := t.Kids[0]
		params := t.Params
		taken := refsPast(body, scope)
		for _, ps := range scope {
			for _, q := range ps {
				taken[q] = true
			}
		}
		var np []string
		for i, p := range params {
			if !taken[p] {
				continue
			}
			if np == nil {
				np = append([]string(nil), params...)
			}
			np[i] = freshHint(p, taken, np)
		}
		if np == nil {
			np = params
		}
		inner := make([][]string, len(scope)+1)
		copy(inner, scope)
		inner[len(scope)] = np
		nb := hyg(body, inner)
		if nb == body && np == nil {
			return t
		}
		return &Term{Kind: KFn, Params: np, Kids: []*Term{nb}}
	case KApp:
		var kids []*Term
		for i, k := range t.Kids {
			nk := hyg(k, scope)
			if nk != k && kids == nil {
				kids = append([]*Term(nil), t.Kids...)
			}
			if kids != nil {
				kids[i] = nk
			}
		}
		if kids == nil {
			return t
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return t
}

// refsPast is every name the body of a binder refers to OUTSIDE that binder:
// the hint of each enclosing parameter a loose index reaches, and each free
// name. A parameter hinted with any of these would capture it on opening.
func refsPast(body *Term, scope [][]string) map[string]bool {
	out := map[string]bool{}
	var walk func(t *Term, d int)
	walk = func(t *Term, d int) {
		switch t.Kind {
		case KBound:
			// Depth d is the binder whose body this is; beyond it is scope.
			if k := t.Depth - d - 1; k >= 0 && k < len(scope) {
				ps := scope[len(scope)-1-k]
				if t.Index < len(ps) {
					out[ps[t.Index]] = true
				}
			}
		case KName:
			out[t.Name] = true
		case KFn:
			walk(t.Kids[0], d+1)
		case KApp:
			for _, k := range t.Kids {
				walk(k, d)
			}
		}
	}
	walk(body, 0)
	return out
}

func freshHint(p string, taken map[string]bool, params []string) string {
	for i := 1; ; i++ {
		n := p + strconv.Itoa(i)
		if taken[n] {
			continue
		}
		clash := false
		for _, q := range params {
			if q == n {
				clash = true
				break
			}
		}
		if !clash {
			return n
		}
	}
}
