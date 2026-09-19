package core

import "fmt"

// THE BINDING SURFACE — docs/spec/binding.md.
//
//	(let LHS VALUE  LHS VALUE  …  BODY)
//
// with LHS a name, `_`, or a tuple pattern `(tuple a b …)`. Nothing here
// survives the reader: a name binds by an application of a λ, which is β's own
// redex, and a tuple pattern binds by applying the VALUE to a continuation,
// which is the product's eliminator (data.md §3.2). So the flat form costs the
// reducer, the checker and every backend exactly nothing.
//
// It is n-ary because nesting bindings is ASSOCIATIVE, and `(let b)` is legal
// because the body is nesting's IDENTITY — the same law that forces `(and)` to
// be `true` rather than choosing it (binding.md §4).

// readLet reads the flat form. `kids` is the whole list, `kids[0]` being `let`.
func readLet(kids []*Term, line int) (*Term, error) {
	rest := kids[1:]
	// Pairs, then one body: an ODD number of elements. An even one is either
	// the old spelling or a body someone forgot.
	if len(rest)%2 == 0 {
		if len(rest) == 2 {
			return nil, fmt.Errorf("line %d: let binds a name to a value: "+
				"`(let x VALUE BODY)`; the old `(let VALUE (fn (x) …))` spelling is "+
				"refused (spec/binding.md)", line)
		}
		return nil, fmt.Errorf("line %d: let takes NAME VALUE pairs and ONE body "+
			"(spec/binding.md)", line)
	}
	t := rest[len(rest)-1]
	for i := len(rest) - 3; i >= 0; i -= 2 {
		b, err := bindOne(rest[i], rest[i+1], t, line)
		if err != nil {
			return nil, err
		}
		t = b
	}
	return t, nil
}

// bindOne binds one left-hand side over `body`.
func bindOne(lhs, value, body *Term, line int) (*Term, error) {
	if lhs.Kind == KName {
		// paramListOf, so a binder obeys exactly the rules a λ's does: a simple
		// name, no module qualifier. One rule, checked in one place.
		ps, err := paramListOf("let", &Term{Kind: KApp, Kids: []*Term{lhs}}, line)
		if err != nil {
			return nil, err
		}
		return &Term{Kind: KApp, Kids: []*Term{Fn(ps, body), value}}, nil
	}
	names, ok := patternNames(lhs)
	if !ok {
		return nil, fmt.Errorf("line %d: let binds a name or `(tuple a b …)`; a pattern that "+
			"may not match belongs in `case` (spec/binding.md §5)", line)
	}
	list := &Term{Kind: KApp}
	for _, n := range names {
		if n.Kind != KName {
			return nil, fmt.Errorf("line %d: a tuple pattern binds names; write a second "+
				"`let` for a nested one (spec/binding.md §5)", line)
		}
		list.Kids = append(list.Kids, n)
	}
	ps, err := paramListOf("a tuple pattern", list, line)
	if err != nil {
		return nil, err
	}
	// THE PRODUCT'S ELIMINATOR, in binding order. `(t (fn (a b) body))` is
	// currying — the universal property — and it is the term the multi-result
	// continuation at a call site always was, so every backend's several-results
	// path sees the shape it already handles.
	return &Term{Kind: KApp, Kids: []*Term{value, Fn(ps, body)}}, nil
}

// patternNames recognises a tuple pattern, which has ALREADY been desugared by
// the time it gets here: the kids of a list are read before the head is
// dispatched on, so `(tuple a b)` arrives as `(fn (#k) (#k a b))`.
//
// `#k` is unwritable — `#` is not isIdentStart — so no source term can be
// mistaken for a pattern, and a λ the programmer wrote has a different binder
// name and is refused as a left-hand side.
func patternNames(t *Term) ([]*Term, bool) {
	if t.Kind != KFn || len(t.Params) != 1 || t.Params[0] != "#k" {
		return nil, false
	}
	b := t.Closed()
	if b.Kind != KApp || len(b.Kids) < 3 || b.Kids[0].Kind != KBound {
		return nil, false
	}
	return b.Kids[1:], true
}
