package emit

import (
	"fmt"

	"oroboros/core"
)

// A TUPLE MADE WHERE β CANNOT REACH IT — tables.md §2.5, ADR 0031.
//
// `(let (tuple x₁ … xₘ) P body)` reads as `(P (fn (x₁ … xₘ) body))`, the
// product's eliminator applied to P (binding.md §5). When P is a `tuple` term,
// an `if` or a `let`, the reducer finishes the job: β substitutes, and
// case-of-case pushes the eliminator inward. When P is a `build` or a `loop`, it
// cannot, and the residual keeps the shape
//
//	((build n (fn (b) … (loop F z̄) …)) (fn (x̄) body))
//
// whose every TAIL is an m-tuple `(fn (#k) (#k c₁ … cₘ))`. This file says what
// that shape is, once, for every pass that meets it.
//
// THE PROJECTION. Pⱼ is P with every tail tuple replaced by its j-th component.
// It is case-of-case with the eliminator (fn (x̄) xⱼ), which is pure and so may
// be copied into every tail, and so
//
//	⟦Pⱼ⟧ = πⱼ ⟦P⟧
//
// by the product's universal property. That is the component law (ADR 0031 §3)
// made executable: whatever an analysis knows of Pⱼ's value, it knows of xⱼ. A
// projection is ANALYSED, never emitted, since emitting m of them would run P's
// effects m times. Emission is a join point instead (each backend's emitJoin).

// tupleArity recognises a tuple term, `(fn (#k) (#k c₁ … cₘ))`, and returns m.
// `#k` is the reader's binder for a tuple and no program can write it.
func tupleArity(t *core.Term) (int, bool) {
	if t == nil || t.Kind != core.KFn || len(t.Params) != 1 || t.Params[0] != "#k" {
		return 0, false
	}
	b := t.Closed()
	if b.Kind != core.KApp || len(b.Kids) < 3 || b.Kids[0].Kind != core.KBound ||
		b.Kids[0].Depth != 0 || b.Kids[0].Index != 0 {
		return 0, false
	}
	return len(b.Kids) - 1, true
}

// tupleElim recognises `(P (fn (x₁ … xₘ) body))` with m ≥ 2 and P a producer
// whose every tail is an m-tuple. The host call's own form, a primitive applied
// to its continuation, is multiPrimCall's and is not this.
func tupleElim(tg *Target, t *core.Term) (prod, k *core.Term, ok bool) {
	if t == nil || t.Kind != core.KApp || len(t.Kids) != 2 {
		return nil, nil, false
	}
	prod, k = t.Kids[0], t.Kids[1]
	if k.Kind != core.KFn || len(k.Params) < 2 || prod.Kind != core.KApp {
		return nil, nil, false
	}
	if _, _, _, isHost := multiPrimCall(tg, t); isHost {
		return nil, nil, false
	}
	if _, ok := projectTail(tg, prod, 0, len(k.Params)); !ok {
		return nil, nil, false
	}
	return prod, k, true
}

// projections returns P₁ … Pₘ.
func projections(tg *Target, prod *core.Term, m int) []*core.Term {
	out := make([]*core.Term, m)
	for j := range out {
		out[j], _ = projectTail(tg, prod, j, m)
	}
	return out
}

// projectTail builds Pⱼ, or reports that some tail of t is not an m-tuple.
//
// It works on the CLOSED term and opens no binder: a tail tuple is replaced in
// place by its component, and OpenWith is what makes that exact, since it
// instantiates `#k` and moves every reference that looked through the tuple's
// binder one level out. So Pⱼ is valid wherever t was, with no fresh names and
// nothing to capture.
func projectTail(tg *Target, t *core.Term, j, m int) (*core.Term, bool) {
	if n, ok := tupleArity(t); ok {
		if n != m {
			return nil, false
		}
		return t.OpenWith([]*core.Term{core.Name("#k")}).Kids[1+j], true
	}
	if t.Kind != core.KApp || len(t.Kids) == 0 {
		return nil, false
	}
	// A host call with several results passes its continuation's tail through.
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		nb, ok := projectTail(tg, kk.Closed(), j, m)
		if !ok {
			return nil, false
		}
		return core.App(t.Kids[0], core.FnClosed(kk.Params, nb)), true
	}
	op := t.Kids[0]
	if op.Kind != core.KName {
		return nil, false
	}
	if op.Name == core.RequireWhereName && len(t.Kids) == 4 {
		inner, ok := projectTail(tg, t.Kids[3], j, m)
		if !ok {
			return nil, false
		}
		return withKid(t, 3, inner), true
	}
	p, known := tg.Prims[op.Name]
	if !known {
		return nil, false
	}
	switch {
	case (p.Kind == "table-build" || p.Kind == "map-build") && len(t.Kids) == 3 &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1:
		lam := t.Kids[2]
		nb, ok := projectTail(tg, lam.Closed(), j, m)
		if !ok {
			return nil, false
		}
		return withKid(t, 2, core.FnClosed(lam.Params, nb)), true
	case p.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1:
		lam := t.Kids[2]
		nb, ok := projectTail(tg, lam.Closed(), j, m)
		if !ok {
			return nil, false
		}
		return withKid(t, 2, core.FnClosed(lam.Params, nb)), true
	case p.Kind == "cond" && len(t.Kids) == 4:
		a, ok1 := projectTail(tg, t.Kids[2], j, m)
		b, ok2 := projectTail(tg, t.Kids[3], j, m)
		if !ok1 || !ok2 {
			return nil, false
		}
		return withKid(withKid(t, 2, a), 3, b), true
	case p.Kind == "iterate" && len(t.Kids) >= 3 && t.Kids[1].Kind == core.KFn:
		lam := t.Kids[1]
		nb, ok := projectExits(tg, lam.Closed(), j, m)
		if !ok {
			return nil, false
		}
		return withKid(t, 1, core.FnClosed(lam.Params, nb)), true
	}
	return nil, false
}

// projectExits projects a loop body's EXITS: every leaf of its clause chain
// that is not an `again` (iteration.md §2). The four forms a clause chain is
// made of are walked, as every walker of one must (CLAUDE.md): `again`, `if`,
// `let`, and a host call's continuation.
func projectExits(tg *Target, t *core.Term, j, m int) (*core.Term, bool) {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if t.Kids[0].Name == "again" {
			return t, true
		}
		if p, ok := tg.Prims[t.Kids[0].Name]; ok {
			switch {
			case p.Kind == "cond" && len(t.Kids) == 4:
				a, ok1 := projectExits(tg, t.Kids[2], j, m)
				b, ok2 := projectExits(tg, t.Kids[3], j, m)
				if !ok1 || !ok2 {
					return nil, false
				}
				return withKid(withKid(t, 2, a), 3, b), true
			case p.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn:
				lam := t.Kids[2]
				nb, ok := projectExits(tg, lam.Closed(), j, m)
				if !ok {
					return nil, false
				}
				return withKid(t, 2, core.FnClosed(lam.Params, nb)), true
			}
		}
	}
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		nb, ok := projectExits(tg, kk.Closed(), j, m)
		if !ok {
			return nil, false
		}
		return core.App(t.Kids[0], core.FnClosed(kk.Params, nb)), true
	}
	return projectTail(tg, t, j, m)
}

func withKid(t *core.Term, i int, k *core.Term) *core.Term {
	c := *t
	c.Kids = append([]*core.Term(nil), t.Kids...)
	c.Kids[i] = k
	return &c
}

// CheckJoins refuses what tables.md §2.5 names as not built: a scope whose value
// is a function term (R1: a variant or a closure, which could carry a buffer out
// unfrozen), and an `again` inside a join point's body, which would jump out of it. Every walker of a clause chain
// would need it as a fifth form, and one that missed it would be unsound, not
// imprecise. It runs on the residual before any pass reads a clause chain.
func CheckJoins(tg *Target, t *core.Term) error {
	var err error
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if err != nil || t == nil {
			return
		}
		if isScopeTerm(tg, t) {
			if bad := badTail(tg, t.Kids[2].Closed()); bad != nil {
				err = fmt.Errorf("a `build`'s value is a table, a value with no buffer in it, or a " +
					"tuple of them (tables.md §2.5); this one is a function term, which is how a " +
					"variant or a closure is represented.\n  A variant made inside a scope may hold a " +
					"buffer that outlives it unfrozen, so it is not built (R1). Return the payload " +
					"and its tag as a tuple, and build the variant after the scope.")
				return
			}
		}
		if _, k, ok := tupleElim(tg, t); ok && jumpsOut(tg, k.Closed()) {
			err = fmt.Errorf("an `again` sits inside the body of a tuple pattern whose value comes " +
				"from a loop or a `build`.\n  That body is a JOIN POINT: it runs once, after the " +
				"loop that made the tuple (tables.md §2.5, ADR 0031), and a jump out of it to an " +
				"enclosing loop is not built.\n  Bind the tuple, compute what the enclosing loop " +
				"needs, and jump from the clause itself.")
			return
		}
		for _, c := range t.Kids {
			walk(c)
		}
	}
	walk(t)
	return err
}

// jumpsOut reports an `again` that is not inside a loop of its own.
func jumpsOut(tg *Target, t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if t.Kids[0].Name == "again" {
			return true
		}
		if p, ok := tg.Prims[t.Kids[0].Name]; ok && p.Kind == "iterate" {
			for _, z := range t.Kids[2:] {
				if jumpsOut(tg, z) {
					return true
				}
			}
			return false // its own body's `again`s are its own
		}
	}
	for _, c := range t.Kids {
		if jumpsOut(tg, c) {
			return true
		}
	}
	return false
}

// eraseExits replaces every exit of a loop body with one marker: the body's
// back-edge skeleton (content.go, bodyKey). The four forms of a clause chain are
// walked; a leaf that is not an `again` is an exit.
func eraseExits(tg *Target, t *core.Term) *core.Term {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if t.Kids[0].Name == "again" {
			return t
		}
		if p, ok := tg.Prims[t.Kids[0].Name]; ok {
			switch {
			case p.Kind == "cond" && len(t.Kids) == 4:
				return withKid(withKid(t, 2, eraseExits(tg, t.Kids[2])), 3, eraseExits(tg, t.Kids[3]))
			case p.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn:
				lam := t.Kids[2]
				return withKid(t, 2, core.FnClosed(lam.Params, eraseExits(tg, lam.Closed())))
			}
		}
	}
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		return core.App(t.Kids[0], core.FnClosed(kk.Params, eraseExits(tg, kk.Closed())))
	}
	return exitMark
}

var exitMark = core.Name("#exit")

// isScopeTerm recognises `(build n (fn (b) …))` and `build-map`'s.
func isScopeTerm(tg *Target, t *core.Term) bool {
	if t.Kind != core.KApp || len(t.Kids) != 3 || t.Kids[0].Kind != core.KName ||
		t.Kids[2].Kind != core.KFn || len(t.Kids[2].Params) != 1 {
		return false
	}
	p, ok := tg.Prims[t.Kids[0].Name]
	return ok && (p.Kind == "table-build" || p.Kind == "map-build")
}

// badTail finds a scope tail that is a function term, alone or as a tuple's
// component: R1's case (tables.md §2.5). Tails are walked through the forms
// projectTail walks, and a loop's through its exits.
func badTail(tg *Target, t *core.Term) *core.Term {
	if t == nil {
		return nil
	}
	if _, ok := tupleArity(t); ok {
		for _, c := range t.OpenWith([]*core.Term{core.Name("#k")}).Args() {
			if c.Kind == core.KFn {
				return c
			}
		}
		return nil
	}
	if t.Kind == core.KFn {
		return t
	}
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		return badTail(tg, kk.Closed())
	}
	if t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return nil
	}
	if t.Kids[0].Name == "again" {
		return nil
	}
	p, ok := tg.Prims[t.Kids[0].Name]
	if !ok {
		return nil
	}
	switch {
	case p.Kind == "cond" && len(t.Kids) == 4:
		if b := badTail(tg, t.Kids[2]); b != nil {
			return b
		}
		return badTail(tg, t.Kids[3])
	case p.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn:
		return badTail(tg, t.Kids[2].Closed())
	case p.Kind == "iterate" && len(t.Kids) >= 3 && t.Kids[1].Kind == core.KFn:
		return badTail(tg, t.Kids[1].Closed())
	case isScopeTerm(tg, t):
		return badTail(tg, t.Kids[2].Closed())
	}
	return nil
}
