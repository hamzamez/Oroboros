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
	if t == nil || t.Kind != core.KFn || len(t.Params) != 1 || !core.IsTupleBinder(t.Params[0]) {
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
// is a function term (R1: a closure, which could carry a buffer out unfrozen; a
// variant is the tuple (tag, payload), ADR 0041, and is ADR 0031's product), and an `again` inside a join point's body, which would jump out of it. Every walker of a clause chain
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
					"tuple of them (tables.md §2.5); this one is a function term: a closure, or a " +
					"tuple component that is one.\n  A closure made inside a scope may hold a buffer " +
					"that outlives it unfrozen, so it is not built (R1). Return what the closure " +
					"would read, as a tuple, and build the closure after the scope. A variant is " +
					"not this: its value is the tuple (tag, payload) (ADR 0041).")
				return
			}
		}
		// A TUPLE PATTERN OVER A SCOPE OR A LOOP needs every exit to give m
		// values: a tuple of them, or, by η, a host call declaring m results,
		// which EtaTails has already expanded. Anything else would reach
		// lowering with no components to bind.
		if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[1].Kind == core.KFn &&
			len(t.Kids[1].Params) >= 2 && isProducer(tg, t.Kids[0]) {
			if _, _, _, host := multiPrimCall(tg, t); !host {
				m := len(t.Kids[1].Params)
				if _, ok := projectTail(tg, t.Kids[0], 0, m); !ok {
					err = fmt.Errorf("a tuple pattern of %d names takes apart a `build` or a `loop`, so "+
						"every exit must give %d values: a (tuple …) of them, or a host call that "+
						"declares %d results (tables.md §2.5). An exit gives something else: a single "+
						"value, a tuple of another size, or a call declaring another number of results.",
						m, m, m)
					return
				}
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
// isProducer is a scope or a loop: a term whose value only its exits give.
func isProducer(tg *Target, t *core.Term) bool {
	if isScopeTerm(tg, t) {
		return true
	}
	if t.Kind != core.KApp || len(t.Kids) < 2 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := tg.Prims[t.Kids[0].Name]
	return ok && p.Kind == "iterate"
}

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

// ═══ η FOR PRODUCTS (tables.md §2.5, "A tail is any term of the product's
// type")
//
// A producer's exit gives C₁ × … × Cₘ however it is spelled. The only exit of
// that type that is not a tail tuple is a host call with m declared results,
// and η for products, p = ⟨π₁ p, …, πₘ p⟩, rewrites it to one:
//
//	(p ā)  =  ((p ā) (fn (x₁ … xₘ) (tuple x₁ … xₘ)))
//
// The call is evaluated once, where it was, and its continuation runs once
// (ADR 0027), so the rewrite changes no effect. It runs after reduction and
// before any pass reads a clause chain, on the producers a tuple pattern takes
// apart, so that every pass that finds components by their tail tuples
// (projectTail and its readers, lowering) sees one.

// EtaTails rewrites every host-call exit of a producer that a tuple pattern of
// m ≥ 2 names eliminates into its η-expanded tuple.
func EtaTails(tg *Target, t *core.Term) *core.Term {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		return core.FnClosed(t.Params, EtaTails(tg, t.Closed()))
	}
	if t.Kind != core.KApp {
		return t
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = EtaTails(tg, k)
	}
	out := &core.Term{Kind: core.KApp, Kids: kids}
	if len(kids) == 2 && kids[1].Kind == core.KFn && len(kids[1].Params) >= 2 && kids[0].Kind == core.KApp {
		if _, _, _, isHost := multiPrimCall(tg, out); !isHost {
			m := len(kids[1].Params)
			if prod, changed := etaTail(tg, kids[0], m); changed {
				if _, ok := projectTail(tg, prod, 0, m); ok {
					return core.App(prod, kids[1])
				}
			}
		}
	}
	return out
}

// hostTuple is `(p ā)` for a primitive p declaring exactly m results, applied
// to its declared arguments: a term of an m-fold product with no tail tuple.
func hostTuple(tg *Target, t *core.Term, m int) bool {
	if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := tg.Prims[t.Kids[0].Name]
	return ok && len(p.Results) == m && len(t.Kids)-1 == len(p.Args)
}

// etaExpand is the right-hand side of η: `((p ā) (fn (#r0 … #rₘ₋₁) (tuple …)))`.
// The continuation is closed apart from its own binders, so it may stand under
// any binder the call does.
func etaExpand(t *core.Term, m int) *core.Term {
	names := make([]string, m)
	comps := []*core.Term{core.Name("#k")}
	for i := range names {
		names[i] = fmt.Sprintf("#r%d", i)
		comps = append(comps, core.Name(names[i]))
	}
	tuple := core.Fn([]string{"#k"}, &core.Term{Kind: core.KApp, Kids: comps})
	return core.App(t, core.Fn(names, tuple))
}

// etaTail rewrites t's tails, walked as projectTail walks them, and reports
// whether any changed.
func etaTail(tg *Target, t *core.Term, m int) (*core.Term, bool) {
	if hostTuple(tg, t, m) {
		return etaExpand(t, m), true
	}
	if _, ok := tupleArity(t); ok || t.Kind != core.KApp || len(t.Kids) == 0 {
		return t, false
	}
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		nb, ch := etaTail(tg, kk.Closed(), m)
		return core.App(t.Kids[0], core.FnClosed(kk.Params, nb)), ch
	}
	op := t.Kids[0]
	if op.Kind != core.KName {
		return t, false
	}
	if op.Name == core.RequireWhereName && len(t.Kids) == 4 {
		inner, ch := etaTail(tg, t.Kids[3], m)
		return withKid(t, 3, inner), ch
	}
	p, known := tg.Prims[op.Name]
	if !known {
		return t, false
	}
	body := func(i int, walk func(*core.Term, int) (*core.Term, bool)) (*core.Term, bool) {
		lam := t.Kids[i]
		nb, ch := walk(lam.Closed(), m)
		return withKid(t, i, core.FnClosed(lam.Params, nb)), ch
	}
	switch {
	case (p.Kind == "table-build" || p.Kind == "map-build" || p.Kind == "let") && len(t.Kids) == 3 &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1:
		return body(2, func(x *core.Term, m int) (*core.Term, bool) { return etaTail(tg, x, m) })
	case p.Kind == "cond" && len(t.Kids) == 4:
		a, c1 := etaTail(tg, t.Kids[2], m)
		b, c2 := etaTail(tg, t.Kids[3], m)
		return withKid(withKid(t, 2, a), 3, b), c1 || c2
	case p.Kind == "iterate" && len(t.Kids) >= 3 && t.Kids[1].Kind == core.KFn:
		return body(1, func(x *core.Term, m int) (*core.Term, bool) { return etaExits(tg, x, m) })
	}
	return t, false
}

// etaExits rewrites a loop body's exits, walked as projectExits walks them.
func etaExits(tg *Target, t *core.Term, m int) (*core.Term, bool) {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if t.Kids[0].Name == "again" {
			return t, false
		}
		if p, ok := tg.Prims[t.Kids[0].Name]; ok {
			switch {
			case p.Kind == "cond" && len(t.Kids) == 4:
				a, c1 := etaExits(tg, t.Kids[2], m)
				b, c2 := etaExits(tg, t.Kids[3], m)
				return withKid(withKid(t, 2, a), 3, b), c1 || c2
			case p.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn:
				lam := t.Kids[2]
				nb, ch := etaExits(tg, lam.Closed(), m)
				return withKid(t, 2, core.FnClosed(lam.Params, nb)), ch
			}
		}
	}
	if _, _, kk, ok := multiPrimCall(tg, t); ok {
		nb, ch := etaExits(tg, kk.Closed(), m)
		return core.App(t.Kids[0], core.FnClosed(kk.Params, nb)), ch
	}
	return etaTail(tg, t, m)
}
