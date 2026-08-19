package core

import "testing"

func TestBoolSugarDesugars(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(and p q)`, `(if p q false)`},
		{`(and p q r)`, `(if p (if q r false) false)`},
		{`(and)`, `true`},
		{`(and p)`, `p`},
		{`(or p q)`, `(if p true q)`},
		{`(or)`, `false`},
		{`(not p)`, `(if p false true)`},
		{`(cond p 1 q 2 else 3)`, `(if p 1 (if q 2 3))`},
		{`true`, `true`},
		{`(if true 1 2)`, `(if true 1 2)`},
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
	for _, bad := range []string{`(cond p 1)`, `(cond else 1 p 2)`, `(cond)`, `(not)`, `(not a b)`} {
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
		{`(if true 1 2)`, `1`},
		{`(if false 1 2)`, `2`},
		{`(and true p)`, `p`},
		{`(or false p)`, `p`},
		{`(not (not true))`, `true`},
		{`(cond false 1 true 2 else 3)`, `2`},
		{`(if q 1 2)`, `(if q 1 2)`},
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
	term, err := ReadTerm(`(if false (log 1) 2)`)
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
