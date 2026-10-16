package core

import (
	"strings"
	"testing"
)

// A SCOPED BUFFER IS A STATE BINDER (scope.go, tables.md §2.4; ADR 0052, 0054).
// The scope must be sequential, the core form must stay what it is, and a state
// name is bound once in its region.

func mustRead(t *testing.T, src string) *Term {
	t.Helper()
	got, err := ReadTerm(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return got
}

// SEQUENTIAL, checked on the structure and not on the print: a printed term
// shows `a` whether or not a binder holds it (CLAUDE.md, "Printing hides an
// unbound binder"). The second buffer's size names the first, and that `a`
// must be BOUND by the first scope.
func TestTheScopeIsSequential(t *testing.T) {
	outer := mustRead(t, `{ var a := array { 0 : _ ∈ 0..n-1 }; var b := array { 0 : _ ∈ 0..len(a)-1 }; g(a, b) }`)
	var builds []*Term
	var walk func(t *Term)
	walk = func(t *Term) {
		if t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == "build" {
			builds = append(builds, t)
		}
		if t.Kind == KFn {
			walk(t.Kids[0])
			return
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(outer)
	if len(builds) != 2 {
		t.Fatalf("two scopes, two builds; got %d in %s", len(builds), outer)
	}
	// The first size sees nothing the scope binds.
	if n := builds[0].Kids[1]; n.Kind != KName || n.Name != "n" {
		t.Errorf("the first size is %v; want the free name n", n)
	}
	// In the second size, `a` is bound by the first scope.
	size := builds[1].Kids[1] // (len a)
	if size.Kind != KApp || len(size.Kids) != 2 || size.Kids[1].Kind != KBound {
		t.Errorf("in the second size, a is %v; want a bound variable of the first scope", size)
	}
}

// THE CORE FORM IS UNTOUCHED: `build` applied is the region's own constructor.
func TestTheCoreFormIsUntouched(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`build(n, f)`, `(build n f)`},
		{`build(n, fn(b) { b })`, `(build n (fn (b) b))`},
	} {
		if got := mustRead(t, c.src).String(); got != c.want {
			t.Errorf("%s read as %s, want %s", c.src, got, c.want)
		}
	}
}

// A STATE NAME IS BOUND ONCE IN ITS REGION (ADR 0054 item 2, spec/j.md §2.6):
// bound again, it would hide the first; the loader refuses it naming the rule.
func TestAStateNameIsBoundOncePerRegion(t *testing.T) {
	for _, src := range []string{
		`fn f(n) { var a := array { 0 : _ ∈ 0..n-1 }; var a := array { 0 : _ ∈ 0..n-1 }; a }`,
		`fn f(n) { var c := 0; var c := 1; c }`,
	} {
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load(forms); err == nil || !strings.Contains(err.Error(), "hides") {
			t.Errorf("%s: want the second binder refused, got %v", src, err)
		}
	}
}
