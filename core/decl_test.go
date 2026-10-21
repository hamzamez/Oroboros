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

// A BINDER OR A RESULT WITH NO SET CLAIMS NOTHING (ADR 0062 item 8), the core's
// `any` (types.md §3.3). A header that claimed one parameter and left another,
// or left its result, reached D with no set and crashed it (shape-2026-10-21).
func TestAnUnclaimedBinderIsAny(t *testing.T) {
	for src, want := range map[string]string{
		`fn run(s ∈ 0..255) { s + 1 }`:          "s:int 0 255 -> any",
		`fn run(x, s ∈ 0..255) (Int) { x + s }`: "x:any s:int 0 255 -> int",
	} {
		forms, err := ReadEntry(src, "t.oro")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got := ""
		for _, f := range forms {
			if f.Kind == "sig" && f.Sig != nil {
				for _, p := range f.Sig.Params {
					got += p.Name + ":" + p.Type + " "
				}
				got += "-> " + f.Sig.Result
			}
		}
		if got != want {
			t.Errorf("%s: the claim is %q, want %q", src, got, want)
		}
	}
}
