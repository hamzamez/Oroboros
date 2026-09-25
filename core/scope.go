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
