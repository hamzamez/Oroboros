package core

import "testing"

// SUBSTITUTION SHIFTS AN INDEX BY THE DEPTH IT LANDS AT (locally nameless,
// core-0). Inside a closed body, a value may refer to an enclosing binder by
// index; put under one more λ, that index must count it. The substitution this
// replaced opened the λ and inserted the value unshifted, so its index pointed
// one binder short (sumloop-2026-10-06).
func TestSubstShiftsAnIndexUnderABinder(t *testing.T) {
	// v refers to the enclosing binder's parameter 1, from depth 0.
	v := App(Name("+"), Bound(0, 1), Int(1))
	// t = (k (fn (y) (g y x))), with x free: x lands under one binder.
	body := App(Name("k"), Fn([]string{"y"}, App(Name("g"), Name("y"), Name("x"))))
	got := subst(body, map[string]*Term{"x": v})
	inner := got.Kids[1].Kids[0] // (g #0.0 v′), the closed body
	if w := inner.Kids[2]; w.Kids[1].Kind != KBound || w.Kids[1].Depth != 1 || w.Kids[1].Index != 1 {
		t.Errorf("the index must count the binder it was put under: got %s", got)
	}
	// And at the top, where it crosses nothing, it is unchanged.
	top := subst(App(Name("h"), Name("x")), map[string]*Term{"x": v})
	if b := top.Kids[1].Kids[1]; b.Depth != 0 || b.Index != 1 {
		t.Errorf("crossing no binder, the index stays: got %s", top)
	}
	// A free name equal to a λ's hint is still free: the λ's own parameter is
	// an index in its closed body, so nothing below can shadow x.
	sh := subst(FnClosed([]string{"x"}, App(Name("x"), Bound(0, 0))), map[string]*Term{"x": Int(7)})
	if a := sh.Kids[0].Kids[0]; a.Kind != KInt || a.Int != 7 {
		t.Errorf("a free x under a λ hinted x is substituted: got %s", sh)
	}
}

// η FOR PRODUCTS takes a binding apart only when EVERY tail of its value is a
// tuple of one arity. Each refusal row is a value it must leave bound: taking a
// non-product apart would bind components that do not exist.
func TestEtaForProductsNeedsEveryTailATupleOfOneArity(t *testing.T) {
	e := testEnv(&Program{Defs: map[string]*Term{}}, "if", "loop", "build", "go.f")
	read := func(src string) *Term {
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		return forms[0].Term
	}
	for _, c := range []struct {
		src  string
		want int
	}{
		{`(def v (tuple 1 2))`, 2},
		{`(def v (if c (tuple 1 2) (tuple 3 4)))`, 2},
		{`(def v (loop ((i 0)) c (tuple i 0) else (again (go.f i))))`, 2},
		{`(def v (if c (tuple 1 2) (tuple 1 2 3)))`, 0},                 // two arities
		{`(def v (if c (tuple 1 2) 7))`, 0},                             // a tail that is no tuple
		{`(def v (go.f (tuple 1 2)))`, 0},                               // a shape it does not know
		{`(def v (loop ((i 0)) c (again (go.f i)) else (again i)))`, 0}, // a loop with no exit
	} {
		n, ok := e.tupleTails(read(c.src), false)
		if !ok {
			n = 0
		}
		if n != c.want {
			t.Errorf("%s: tails of arity %d, want %d", c.src, n, c.want)
		}
	}
}
