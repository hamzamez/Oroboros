package emit_test

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A C STRING IS THE KERNEL OF ν (ADR 0091). `where nulfree(s)` is the C-string
// parameter's obligation, decided by ν on the argument's term: a literal by
// its scalars, `++` by both sides, `string_of(c)` by c ≠ 0, and anything else
// only by a guard `if nulfree(s)` on its arm.
func TestACStringIsTheKernelOfNu(t *testing.T) {
	const g = "fn g(s ∈ String) (Int) where nulfree(s) { 1 }\n"
	accepted := map[string]string{
		"a literal":             `export fn h(n ∈ 0..9) (Int) { g("abc") + n }`,
		"a concatenation":       `export fn h(n ∈ 1..9) (Int) { g("ab" ++ string_of(n) ++ "\u{e9}") }`,
		"a scalar proven not 0": `export fn h(n ∈ 0..9) (Int) { g(string_of(955 + n)) }`,
		"a guard":               `export fn h(s ∈ String) (Int) { if nulfree(s) { g(s) } else { 0 } }`,
	}
	for what, src := range accepted {
		if err := legalGo(t, g+src); err != nil {
			t.Errorf("%s: want it accepted, got %v", what, err)
		}
	}
	refused := map[string]string{
		"a literal holding a NUL": `export fn h(n ∈ 0..9) (Int) { g("a\u{0}b") + n }`,
		"a scalar that may be 0":  `export fn h(n ∈ 0..9) (Int) { g("ab" ++ string_of(n)) }`,
		"an unknown string":       `export fn h(s ∈ String) (Int) { g(s) }`,
		"the guard's other arm":   `export fn h(s ∈ String) (Int) { if nulfree(s) { 0 } else { g(s) } }`,
	}
	for what, src := range refused {
		err := legalGo(t, g+src)
		if err == nil || !strings.Contains(err.Error(), "nulfree") {
			t.Errorf("%s: want it refused naming nulfree, got %v", what, err)
		}
	}

	// THE PLANTED FAULT: a literal read as NUL-free without its scalars.
	emit.PlantNulFault = true
	defer func() { emit.PlantNulFault = false }()
	if err := legalGo(t, g+refused["a literal holding a NUL"]); err != nil {
		t.Errorf("with every literal read as NUL-free the program should be accepted, so the test reaches the rule; got %v", err)
	}
}
