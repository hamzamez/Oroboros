package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// lowerClaimed lowers a program's first export with its signature, and NOT
// through the type checker, so what it shows is the IR's own claim edge.
func lowerClaimed(t *testing.T, src string) error {
	t.Helper()
	return lowerClaimedOn(t, "go", src)
}

func lowerClaimedOn(t *testing.T, target, src string) error {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/" + target)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read("use " + target + "\n" + src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Lower(tg, q, prog.Sigs[q], nf, Options{})
	return err
}

// THE CLAIM EDGE CAN FAIL (claim.go). Each body yields a sort its signature
// does not declare, and lowering refuses it by itself: before tableelim, W5
// compared a non-table result with its own type, and these printed a function
// whose result contradicts the declaration.
func TestTheIRChecksADeclaredResultsSort(t *testing.T) {
	for _, c := range []struct{ result, body string }{
		{"String", "len(a)"},
		{"F64", "array { 1.0, 2.0 }"},
		{"String", "t = { var b := array { 0 : _ ∈ 0..1 }; b[0] := 1.5; b }; t[0]"},
		{"Array(String)", "array { 1.0, 2.0 }"},
	} {
		src := "export fn f(a ∈ Array(F64)) (" + c.result + ") { " + c.body + " }\n"
		err := lowerClaimed(t, src)
		if err == nil || !strings.Contains(err.Error(), "its signature declares result") {
			t.Errorf("%s under %s: want the claim refused, got %v", c.body, c.result, err)
		}
	}
}

// …and it refuses nothing a declaration's sort admits: an integer in any
// realization (ADR 0033), a host alias of a table, a table of integers against
// a declared element range.
func TestTheIRsClaimEdgeAdmitsWhatTheSortAdmits(t *testing.T) {
	for _, c := range []struct{ result, body string }{
		{"0..255", "len(a)"},
		{"0..2^64-1", "len(a)"},
		{"Array(0..255)", "array { 104, 105, 33 }"},
		{"F64", "t = { var b := array { 0 : _ ∈ 0..1 }; b[0] := 1.5; b }; t[0]"},
	} {
		src := "export fn f(a ∈ Array(F64)) (" + c.result + ") { " + c.body + " }\n"
		if err := lowerClaimed(t, src); err != nil {
			t.Errorf("%s under %s: %v", c.body, c.result, err)
		}
	}
	// A host alias of a table: Java's double-array is ρ(array f64).
	if err := lowerClaimedOn(t, "java", `export fn f(a ∈ Array(F64)) (double_array) { array { 1.0, 2.0 } }
`); err != nil {
		t.Errorf("(array 1.0 2.0) under double-array: %v", err)
	}
}
