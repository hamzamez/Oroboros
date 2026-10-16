package core

import "testing"

// THE CONNECTIVES ARE THEIR CONDITIONALS (ADR 0017, booleans.md): `&&`, `||` and
// `!` erase in the reader, and an `else if` chain is nested conditionals.
func TestBoolSugarDesugars(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`p && q`, `(if p q false)`},
		{`p && q && r`, `(if p (if q r false) false)`},
		{`p || q`, `(if p true q)`},
		{`!p`, `(if p false true)`},
		{`if p { 1 } else if q { 2 } else { 3 }`, `(if p 1 (if q 2 3))`},
		{`true`, `true`},
		{`if true { 1 } else { 2 }`, `(if true 1 2)`},
	} {
		got, err := ReadTerm(c.src)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got.String() != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.src, got, c.want)
		}
	}
	for _, bad := range []string{`!`, `p &&`, `|| q`} {
		if _, err := ReadTerm(bad); err == nil {
			t.Errorf("%s should not read", bad)
		}
	}
}

// The conditional folds when its condition is known — which is conditional
// compilation with no preprocessor, and the first evaluation reduction does.
func TestStaticConditionFolds(t *testing.T) {
	env := &Env{Prim: map[string]bool{"if": true, "log": true, "add": true}}
	for _, c := range []struct{ src, want string }{
		{`if true { 1 } else { 2 }`, `1`},
		{`if false { 1 } else { 2 }`, `2`},
		{`true && p`, `p`},
		{`false || p`, `p`},
		{`!!true`, `true`},
		{`if false { 1 } else if true { 2 } else { 3 }`, `2`},
		{`if q { 1 } else { 2 }`, `(if q 1 2)`},
	} {
		term, err := ReadTerm(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		env.Prim["p"], env.Prim["q"] = true, true
		got, err := Normalize(term, env, DefaultFuel)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got.String() != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

// An untaken branch is dropped even when it is impure, and that is sound for a
// different reason than beta's: the branch does not run.
func TestUntakenBranchIsDroppedEvenWhenImpure(t *testing.T) {
	env := &Env{Prim: map[string]bool{"if": true, "log": true}} // log is NOT pure
	term, err := ReadTerm(`if false { log(1) } else { 2 }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Normalize(term, env, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "2" {
		t.Errorf("got %s, want 2", got)
	}
}

// A NEGATED CONDITION IS A SWAP (booleans.md §4.3).
//
//	if (¬c) a b  =  if c b a
//
// Not a new rule: `!c` is `if c { false } else { true }`, and case-of-case plus
// `if true { a } else { b } → a` give the identity. The reader performs both
// eagerly, at the one place an `if` is built, which is what makes a clause
// chain free — a chain states the negation to put a failure case first, and
// without this the emitted code grows a `!` and its branches come out in the
// opposite order.
func TestANegatedConditionSwapsItsBranches(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`if !p { a } else { b }`, `(if p b a)`},
		// twice, because ¬¬ is the identity and one pass would leave a negation
		{`if !!p { a } else { b }`, `(if p a b)`},
		// a clause chain is where it earns its place
		{`if !p { A } else if x < y { B } else { C }`, `(if p (if (< x y) B C) A)`},
		// and in a loop's clauses, which share the chain
		{`loop i = 0 { !(i < n) -> i; else -> again(i + 1) }`,
			`(loop (fn (i) (if (< i n) (again (+ i 1)) i)) 0)`},
	} {
		got, err := ReadTerm(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got.String() != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

// AND A CONNECTIVE IS NOT A BRANCH SELECTOR. `a && b` is `if a { b } else
// { false }`, so a boolean literal in a branch means the term is an operator
// each backend emits as one (emit/connective.go). Swapping there would turn
// `!p && q` into a conditional — lowering further than the target requires.
func TestTheSwapLeavesConnectivesAlone(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`!p && q`, `(if (if p false true) q false)`},
		{`!p || q`, `(if (if p false true) true q)`},
		{`!!p`, `(if (if p false true) false true)`},
		// one branch a literal is enough to make it a connective
		{`if !p { true } else { b }`, `(if (if p false true) true b)`},
	} {
		got, err := ReadTerm(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got.String() != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}
