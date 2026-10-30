package core

import (
	"strings"
	"testing"
)

// A SET HAS NO REPRESENTATION (ADR 0049): a set operator in a computation, or in
// a claim's formula, is refused saying what it is; in a set position it is the
// set. Until docs-2026-10-25, `x ∈ 0..9` as a value was refused as "“ is not
// imported" (the range read as a path), and `where x ∈ 0..9` compiled on an
// entry point, an assumed atom over an unbound name.
func TestASetInATermIsRefusedWithWhatItIs(t *testing.T) {
	for src, want := range map[string]string{
		"export fn f(x ∈ 0..20) { b = x ∈ 0..9; b }":                               "is membership, the test χ_S(e)",
		"export fn f(x ∈ 0..20) { b = x ∉ 0..9; b }":                               "is non-membership",
		"export fn f(x) { b = Int ∪ Bool; b }":                                     "is union",
		"export fn f(x ∈ 0..20) (Int) where x ∈ 0..9 { x }":                        "header: `∈` is membership",
		"export fn f(x ∈ 0..20) (r ∈ Int) ensure r ∈ 0..9 { x }":                   "header: `∈` is membership",
		"fn g(x ∈ 0..20) (Int) where x ∈ 0..9 { x }\nexport fn f() (Int) { g(5) }": "header: `∈` is membership",
	} {
		forms, err := Read(src + "\n")
		if err == nil {
			_, _, err = Load(forms)
		}
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "no representation") {
			t.Errorf("%s: want a refusal saying %q, got %v", src, want, err)
		}
	}
	// The controls: a set in its own position.
	for _, src := range []string{
		"export fn f(x ∈ 0..9) { x }",
		"export fn f(x ∈ 0..20) { b ∈ 0..20 = x; b }",
		"export fn f() { array { k : k ∈ 0..3 } }",
		"export fn f(x ∈ 0..20) (Int) where x <= 9 { x }",
		"export fn f(x ∈ 0..9) (Int × Bool) { (x, true) }",
	} {
		forms, err := Read(src + "\n")
		if err == nil {
			_, _, err = Load(forms)
		}
		if err != nil {
			t.Errorf("%s: a set in a set position must load: %v", src, err)
		}
	}
}

// A RANGE IS A STATIC VALUE (ADR 0084's build, spec/typevalues.md §1b): it
// passes the loader as an argument for inf, sup or a definition over sets, and
// one that reaches the program as a value is refused at staging (Law S).
func TestARangeAsAValueIsRefusedAtStaging(t *testing.T) {
	forms, err := Read("t = fn(x) { b = 0..9; b }\n")
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Normalize(entryTerm(t, prog, terms), testEnv(prog), DefaultFuel)
	if err == nil || !strings.Contains(err.Error(), "is a range") || !strings.Contains(err.Error(), "no representation") {
		t.Errorf("a range as a value: want it refused at staging, got %v", err)
	}
}
