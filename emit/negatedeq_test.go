package emit_test

import (
	"testing"

	"oroboros/emit"
)

// ¬(a = b) IS a ≠ b (llvm-2026-11-02): a `where b != 0` in the language's own
// words is an obligation the facts decide, as the host's `!=`(b, 0) is.
func TestTheLanguagesNotEqualIsADisequality(t *testing.T) {
	head := "fn d(a ∈ 0..9, b ∈ -9..9) (Int) where b != 0 { a / b }\n"
	ok := head + "export fn g(a ∈ 0..9, b ∈ 1..9) (Int) { d(a, b) }"
	accepts(t, "a divisor proven nonzero", ok)
	refuses(t, "a divisor that may be zero", head+"export fn g(a ∈ 0..9, b ∈ 0..9) (Int) { d(a, b) }", "d")

	emit.PlantNegatedEqFault(true)
	defer emit.PlantNegatedEqFault(false)
	if _, err := compileGo(t, ok); err == nil {
		t.Errorf("with ¬(a = b) opaque again the proven divisor was accepted: the test does not reach the rule")
	}
}
