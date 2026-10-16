package core

import "testing"

// A HEADER IS ITS DEFINITION'S CLAIM (ADR 0051, D1–D2; ADR 0062): a definition
// is a named λ, and the λ carries each parameter's set, the result's set, the
// `where` and the `ensure`, over its own binders.
func TestAHeaderIsItsDefinitionsClaim(t *testing.T) {
	ds, err := ReadProgram(`fn f(x ∈ 0..9, y ∈ Int) (r ∈ Int) where x > 0 ensure r > 0 { x + y }`, "", ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	var def *DefDecl
	for _, d := range ds {
		if dd, ok := d.(*DefDecl); ok {
			def = dd
		}
	}
	if def == nil {
		t.Fatal("no definition")
	}
	l := def.Value.(*Lam)
	c := l.Claim
	switch {
	case c == nil:
		t.Fatal("the λ carries no claim")
	case len(c.Params) != 2:
		t.Errorf("the claim's binders: %+v", c.Params)
	case l.Params[0] != "x" || l.Params[1] != "y":
		t.Errorf("the λ's binders: %v", l.Params)
	case c.Pre == nil || c.Post == nil || c.Result == nil:
		t.Errorf("pre, post and result: %+v", c)
	}
}
