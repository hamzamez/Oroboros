package emit_test

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A QUOTIENT OUTSIDE THE WORD IS REFUSED (plan item 8, quotient-2026-11-03).
// For b ≠ 0, |a / b| ≤ |a| with equality iff |b| = 1, so on the word
// [−2⁶³, 2⁶³ − 1] the quotient leaves it exactly at −2⁶³ / −1, where ℤ's
// answer is 2⁶³. The decision proves a quotient inside the word as it proves a
// sum, by the divisor's sign regions taken at their corners; what it cannot
// prove is refused, as an unproven sum is.
func TestAQuotientOutsideTheWordIsRefused(t *testing.T) {
	refused := "export fn g(a ∈ -2^63..2^63-1, b ∈ -9..9) (Int) where b != 0 { a / b }"
	for what, src := range map[string]string{
		"a positive divisor":                "export fn g(a ∈ -2^63..2^63-1, b ∈ 1..9) (Int) { a / b }",
		"a negative divisor that is not -1": "export fn g(a ∈ -2^63..2^63-1, b ∈ -9..-2) (Int) { a / b }",
		"a dividend that is not -2^63":      "export fn g(a ∈ -2^63+1..2^63-1, b ∈ -9..9) (Int) where b != 0 { a / b }",
		"a constant divisor":                "export fn g(a ∈ -2^63..2^63-1) (Int) { a / 10 }",
	} {
		if err := legalGo(t, src); err != nil {
			t.Errorf("%s: want it accepted, got %v", what, err)
		}
	}
	err := legalGo(t, refused)
	if err == nil || !strings.Contains(err.Error(), "/") || !strings.Contains(err.Error(), "9223372036854775808") {
		t.Errorf("a dividend that may be -2^63 over a divisor that may be -1: want it refused naming the quotient 2^63, got %v", err)
	}

	// THE PLANTED FAULT: `/` not counted, the bug as it shipped.
	ir.PlantQuotientFault = true
	defer func() { ir.PlantQuotientFault = false }()
	if err := legalGo(t, refused); err == nil {
		// as expected under the fault: this is what shipped
	} else {
		t.Errorf("with `/` not counted the program should be accepted, as it was, so the test reaches the rule; got %v", err)
	}
}
