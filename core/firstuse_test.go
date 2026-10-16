package core

import "testing"

// A VERSION USED ONCE, FIRST, IS PUT WHERE IT IS USED, effects and all: let x =
// e in E[x] is E[e] when E evaluates its hole first and unconditionally, with
// nothing impure before it (Moggi's let-β for a linear use in evaluation
// position; reduce.go, firstUse). The rule is applied to the versions the state
// translation binds, named VersionPrefix, so the test marks its binder so after
// loading, as the translation would have named it. Each case is one condition
// of the rule, and each has a planted fault that makes it fail
// (storeorder-2026-10-15 §4): the purity of what is evaluated before the hole, a
// branch, a loop's body. A program's own binder keeps its binding, the last case.
func TestALetOfAVersionUsedOnceFirstIsItsPlace(t *testing.T) {
	prims := []string{"!read", "!write", "add", "ge", "if", "loop"}
	for _, c := range []struct {
		name, src, want string
		program         bool
	}{
		{"used first: substituted",
			"t = fn(n) { x = read(); write(add(x, n)) }",
			"(fn (n) (write (add (read) n)))", false},
		{"after a pure argument: substituted",
			"t = fn(n) { x = read(); write(add(n, x)) }",
			"(fn (n) (write (add n (read))))", false},
		{"after an effect: bound, or the effects would be exchanged",
			"t = fn(n) { x = read(); add(write(n), x) }",
			"(fn (n) (let (read) (fn (#vx) (add (write n) #vx))))", false},
		{"in a branch: bound, or the effect would run conditionally",
			"t = fn(n, c) { x = read(); if c { write(x) } else { 0 } }",
			"(fn (n c) (let (read) (fn (#vx) (if c (write #vx) 0))))", false},
		{"in a loop's body: bound, or the effect would run each time round",
			"t = fn(n) { x = read(); loop i = 0 { ge(i, n) -> i; else -> again(add(i, x)) } }",
			"(fn (n) (let (read) (fn (#vx) (loop (fn (i) (if (ge i n) i (again (add i #vx)))) 0))))", false},
		{"a program's own binder: bound, its name what the analyses key on",
			"t = fn(n) { x = read(); write(add(x, n)) }",
			"(fn (n) (let (read) (fn (x) (write (add x n)))))", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			forms, err := Read(c.src)
			if err != nil {
				t.Fatal(err)
			}
			prog, terms, err := Load(forms)
			if err != nil {
				t.Fatal(err)
			}
			term := entryTerm(t, prog, terms)
			if !c.program {
				term = markVersion(term, "x")
			}
			out, err := Normalize(term, testEnv(prog, prims...), DefaultFuel)
			if err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// markVersion renames the λ binder x to a version's name: binders are hints
// over a closed body, so only the hint changes.
func markVersion(t *Term, x string) *Term {
	if t == nil {
		return nil
	}
	c := *t
	if t.Kind == KFn {
		c.Params = append([]string(nil), t.Params...)
		for i, p := range c.Params {
			if p == x {
				c.Params[i] = VersionPrefix + x
			}
		}
	}
	if len(t.Kids) > 0 {
		c.Kids = make([]*Term, len(t.Kids))
		for i, k := range t.Kids {
			c.Kids[i] = markVersion(k, x)
		}
	}
	return &c
}
