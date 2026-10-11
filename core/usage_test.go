package core

import "testing"

// TREE USAGE ON THE SHAPES RELEVANCE TURNS ON (abstract.md §8.1). Each case is
// a definition's body and whether its parameter r is used on every path that
// ends normally. The corpus agreement test holds the tree to the reducer on
// definitions' parameters, which are bound outside every loop in their body;
// these reach what that cannot: a value bound inside an iteration, the arms of
// `try` and `expect`, a pattern that shadows, a hand-off to `_`.
func TestTreeUsageOnTheShapesRelevanceTurnsOn(t *testing.T) {
	for _, c := range []struct {
		body string
		used bool
	}{
		{`r`, true},
		{`0`, false},
		{`(if c r r)`, true},
		{`(if c r 0)`, false},
		{`(if r 1 2)`, true},
		{`(if c (abandon "x") r)`, true},
		{`(seq (abandon "x") 0)`, true},
		{`(let x r (f x))`, true},
		{`(let x r 0)`, false},
		{`(seq r 0)`, true},
		{`(let _ r 0)`, false},
		{`(let (tuple a b) r a)`, true},
		{`(drop r)`, false},
		{`(keep r)`, true},
		{`(drop (keep r))`, true},
		{`(case c (ok r) r (err e) 0)`, false},
		{`(case c (ok r) r (err e) r)`, false}, // the first arm's r is the payload
		{`(case r (ok v) v (err e) e)`, true},
		{`(try (ok v) (g c) r)`, false},
		{`(try (ok v) r v)`, true},
		{`(expect (ok v) (g c) "why" r)`, true},
		{`(and c r)`, false},
		{`(or r c)`, true},
		{`(cond c r else r)`, true},
		{`(cond c r else 0)`, false},
		// bound outside a loop: the jump goes round, and the exits decide
		{`(loop ((i 0)) (>= i 3) r else (again (+ i 1)))`, true},
		{`(loop ((i 0)) (>= i 3) 0 else (again (+ i 1)))`, false},
		{`(loop ((i r)) (>= i 3) 0 else (again (+ i 1)))`, true},
		{`(loop ((r 0)) (>= r 3) 0 else (again (+ r 1)))`, false},
		{`(match (c) 0 r else (again 1))`, true}, // the only arm that ends uses r
		{`(match (c) 0 r else 7)`, false},
		{`(local b (table r 0) (f b))`, true},
	} {
		src := `(def drop (x) 0) (def keep (x) x) (def h (r c) ` + c.body + `)`
		decls, err := ReadTrees(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		defs := map[string]*Lam{}
		for _, d := range decls {
			if d, ok := d.(*DefDecl); ok {
				defs[d.Name] = d.Value.(*Lam)
			}
		}
		u := &UsageEnv{Defs: defs}
		if got := u.ParamUsage("h", 0).Relevant(); got != c.used {
			t.Errorf("%s: r used on every path %v, want %v (%+v)", c.body, got, c.used, u.ParamUsage("h", 0))
		}
	}
	// A value bound inside an iteration ends at the jump: dropping it there is
	// dropping it (abstract2-2026-10-11).
	inner := func(clause string) Usage {
		decls, err := ReadTrees(`(def h (n) (loop ((i n)) (>= i 3) 0 else ` + clause + `))`)
		if err != nil {
			t.Fatal(err)
		}
		loop := decls[0].(*DefDecl).Value.(*Lam).Body.(*Loop)
		return (&UsageEnv{}).Of("r", loop.Arms[1].Body.(*TLet).Body, false)
	}
	if inner(`(let r (w i) (again (+ i 1)))`).Relevant() {
		t.Error("a value bound in an iteration and dropped at the jump counted as used")
	}
	if !inner(`(let r (w i) (again (+ r 1)))`).Relevant() {
		t.Error("a value bound in an iteration and passed to the jump counted as unused")
	}
}
