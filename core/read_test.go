package core

import (
	"strings"
	"testing"
)

// core-0 §1.1 — NFC. Without it, `é` as U+00E9 and as e+U+0301 are two
// distinct identifiers that display identically.
func TestNonNFCIsRejected(t *testing.T) {
	if _, err := Read("(def caf\u0065\u0301 1)"); err == nil {
		t.Fatal("a decomposed é must be rejected")
	}
	if _, err := Read("(def caf\u00e9 1)"); err != nil {
		t.Errorf("a precomposed é is NFC and must pass: %v", err)
	}
	// A combining mark with no precomposed form is NFC-stable, and rejecting
	// it would break every script that needs one.
	if _, err := Read("(def q\u0307x 1)"); err != nil {
		t.Errorf("q + combining dot above has no precomposed form: %v", err)
	}
}

// §1.1 — case is preserved and significant for IDENTITY. What the rule forbids
// is case carrying MEANING, as in Shen's capitals-are-variables or Go's
// capitals-are-exported.
func TestCaseIsSignificantForIdentity(t *testing.T) {
	forms, err := Read("(f Xr xr XR)")
	if err != nil {
		t.Fatal(err)
	}
	kids := forms[0].Term.Kids
	seen := map[string]bool{}
	for _, k := range kids[1:] {
		seen[k.Name] = true
	}
	if len(seen) != 3 {
		t.Errorf("Xr, xr and XR must be three distinct names, got %d: %v", len(seen), seen)
	}
}

// A repeated binder in ONE abstraction is ill-formed: β substituted parameter
// by parameter, so ((fn (x x) x) 1 2) reduced to 2 and the first argument
// vanished with no way to name it.
func TestDuplicateParameterIsRejected(t *testing.T) {
	if _, err := Read("(fn (x x) x)"); err == nil {
		t.Fatal("(fn (x x) x) must be rejected")
	}
	if _, err := Read("(fn (a b) a)"); err != nil {
		t.Errorf("distinct parameters must pass: %v", err)
	}
	// Nested shadowing is two abstractions, and stays legal.
	if _, err := Read("(fn (x) (fn (x) x))"); err != nil {
		t.Errorf("nested shadowing must stay legal: %v", err)
	}
}

// Name resolution — a separate question from the covering check, and one this
// project had been answering only by accident.
//
//	scope    — is this name bound anywhere at all?  A PROGRAM error.
//	covering — can THIS target provide it?          A portability property.
//
// Three holes came from conflating them: `oro` warned and exited 0, `gen` never
// checked, and a name appearing only in an unreached definition was never
// looked at.
func TestScopeCheck(t *testing.T) {
	env := func(src string) (*Env, []*Term) {
		t.Helper()
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, terms, err := Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		e := &Env{Defs: prog.Defs, Prim: map[string]bool{"add": true},
			Pure: map[string]bool{"add": true}, Rec: map[string]bool{}}
		e.MarkRecursive()
		return e, terms
	}

	e, terms := env("(fn (x) y)")
	if err := e.CheckScope(terms); err == nil {
		t.Error("a free variable in a body must be reported")
	}

	// The classic one: a typo in a definition the program never reaches.
	e, terms = env("(def unused (fn (x) nope))\n(add 1 2)")
	err := e.CheckScope(terms)
	if err == nil {
		t.Fatal("a typo in an unreached definition must be reported")
	}
	if !strings.Contains(err.Error(), "unused") {
		t.Errorf("the report should name the definition, got %v", err)
	}

	// Parameters, definitions and primitives are all in scope.
	e, terms = env("(def f (fn (a) (add a 1)))\n(fn (x) (f x))")
	if err := e.CheckScope(terms); err != nil {
		t.Errorf("bound names must pass: %v", err)
	}

	// A recursive definition refers to itself, which is in scope.
	e, terms = env("(def loop-forever (fn (n) (loop-forever n)))\n(loop-forever 1)")
	if err := e.CheckScope(terms); err != nil {
		t.Errorf("a recursive definition is in its own scope: %v", err)
	}
}

// A binder must be a SIMPLE name. Binding a qualified one is meaningless — a λ
// cannot bind into a module — and it let a parameter shadow a module-qualified
// primitive, after which reduction applied a number to two arguments.
func TestQualifiedNameCannotBeAParameter(t *testing.T) {
	if _, err := Read("(fn (a.b) a.b)"); err == nil {
		t.Fatal("(fn (a.b) …) must be rejected")
	}
	if _, err := Read("(fn (num/f64.add) 1)"); err == nil {
		t.Fatal("binding a module-qualified name must be rejected")
	}
	// A path segment separator is still fine inside a simple name.
	if _, err := Read("(fn (a/b) a/b)"); err != nil {
		t.Errorf("`/` is an ordinary identifier character: %v", err)
	}
}
