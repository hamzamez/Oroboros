package core

import "fmt"

// A SCOPED BUFFER IS A BINDER — docs/spec/tables.md §2.4.
//
//	(build b n body)                  ≡  (build n (fn (b) body))
//	(build b₁ n₁  …  bₖ nₖ  body)       ≡  (build n₁ (fn (b₁) (build b₂ n₂ … body)))
//
// and the same for `build-map`. The λ of the core form is never a function
// value — it sits where a backend consumes it structurally (tables.md §14.1) —
// so it is a binding occurrence, and the source writes it as one, as `let`
// writes its redex as a binding (binding.md §1). The fold is on the right, so
// the scope is sequential: nᵢ sees b₁ … bᵢ₋₁.
//
// What this returns IS the core form, term for term: `Fn` over the body the
// reader already built, exactly what `(fn (b) body)` reads as. So nothing below
// the reader knows the binder form exists, and emission cannot change.

// isScopeHead reports whether a list headed by this name may be a binder form.
func isScopeHead(t *Term) bool {
	return t.Kind == KName && (t.Name == "build" || t.Name == "build-map")
}

// readScope reads the binder form. `kids[0]` is the head and there are at least
// three elements after it; two is the core form and one is a target file's
// `(build "…")` directive, and neither reaches here.
func readScope(kids []*Term, line int) (*Term, error) {
	head := kids[0].Name
	rest := kids[1:]
	if len(rest)%2 == 0 {
		return nil, fmt.Errorf("line %d: %s binds NAME SIZE pairs and ONE body, "+
			"`(%s b n body)`; or it is the core form `(%s n (fn (b) body))` "+
			"(spec/tables.md §2.4)", line, head, head, head)
	}
	// THE NAMES OF ONE SCOPE ARE ONE PARAMETER LIST. The buffers are one region
	// (tables.md §2.4, law 3), alive together, so a repeated name is `(fn (x x)
	// …)`: the second silently hides the first. paramListOf refuses that, and
	// applies every other rule a binder obeys, in one place.
	var names []*Term
	for i := 0; i+1 < len(rest); i += 2 {
		names = append(names, rest[i])
	}
	params, err := paramListOf(head, &Term{Kind: KApp, Kids: names}, line)
	if err != nil {
		return nil, err
	}
	t := rest[len(rest)-1]
	for i := len(params) - 1; i >= 0; i-- {
		t = &Term{Kind: KApp, Kids: []*Term{Name(head), rest[2*i+1], Fn([]string{params[i]}, t)}}
	}
	return t, nil
}

// LocalName is the scope of local state (spec/local.md).
const LocalName = "local"

// CellName heads a cell's scope until the loader translates it away: the core
// form `(#cell e (fn (x) body))`, a binder x holding e. `#` starts no
// identifier, so no program writes one.
const CellName = "#cell"

// readLocal reads `(local x₁ e₁ … xₖ eₖ body)`, sequential like `let`. THE
// INITIALIZER'S VALUE DECIDES WHAT A BINDER IS (spec/local.md §1): a table,
// written `(table n f)` or `(array …)`, gives a buffer with those contents,
// since a buffer's contents are a table; anything else gives a cell, a buffer
// of length one. The decision is on the form written, so it is the reader's.
//
//	b (table n 0)        ⟶  (build n (fn (b) …))                   build's own form
//	b (table n f)        ⟶  (build n (fn (b) (let b FILL …)))      f applied to each i
//	b (table n v)        ⟶  … FILL with v                          v a literal: K v
//	b (array e₀ … eₖ₋₁)  ⟶  (build k (fn (b) (let b (set … (set b 0 e₀) …) …)))
//	c e                  ⟶  (#cell e (fn (c) …))                   translated by the loader
func readLocal(kids []*Term, line int) (*Term, error) {
	rest := kids[1:]
	if len(rest) < 3 || len(rest)%2 == 0 {
		return nil, fmt.Errorf("line %d: local binds NAME VALUE pairs and ONE body, "+
			"`(local x e body)` (spec/local.md)", line)
	}
	var names []*Term
	for i := 0; i+1 < len(rest); i += 2 {
		names = append(names, rest[i])
	}
	params, err := paramListOf(LocalName, &Term{Kind: KApp, Kids: names}, line)
	if err != nil {
		return nil, err
	}
	t := rest[len(rest)-1]
	for i := len(params) - 1; i >= 0; i-- {
		t = localBinder(params[i], rest[2*i+1], t)
	}
	return t, nil
}

// localBinder is one binder of a `local` around its body.
func localBinder(x string, e, body *Term) *Term {
	head := func(t *Term, name string) bool {
		return t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == name
	}
	rebind := func(fill *Term) *Term { // (let x fill body), inside the scope
		return App(Fn([]string{x}, body), fill)
	}
	switch {
	case head(e, "table") && len(e.Kids) == 3:
		n, f := e.Kids[1], e.Kids[2]
		if f.Kind == KInt && f.Int == 0 {
			return App(Name("build"), n, Fn([]string{x}, body)) // zero-filled: build's own form
		}
		// A FILL LOOP: element i is (f i), or v for a literal v (the constant
		// table K v). The size is bound once, since the loop reads it again.
		var elem *Term
		switch f.Kind {
		case KInt, KFloat, KStr, KBool:
			elem = f
		default:
			elem = App(f, Name("#i"))
		}
		loop := App(Name("loop"),
			Fn([]string{"#i", x}, App(Name("if"), App(Name(">="), Name("#i"), Name("#n")),
				Name(x),
				App(Name("again"), App(Name("+"), Name("#i"), Int(1)),
					App(Name("set"), Name(x), Name("#i"), elem)))),
			Int(0), Name(x))
		return App(Fn([]string{"#n"}, App(Name("build"), Name("#n"), Fn([]string{x}, rebind(loop)))), n)
	case head(e, "array"):
		elems := e.Kids[1:]
		var fill *Term = Name(x)
		for i, el := range elems {
			fill = App(Name("set"), fill, Int(int64(i)), el)
		}
		inner := body
		if len(elems) > 0 {
			inner = rebind(fill)
		}
		return App(Name("build"), Int(int64(len(elems))), Fn([]string{x}, inner))
	}
	return App(Name(CellName), e, Fn([]string{x}, body))
}
