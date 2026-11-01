package ir

import (
	"strings"
	"testing"
)

// AN OPEN RANGE ON A RESULT IS A CLAIM (resultclaim-2026-11-02): `Nat` and
// `10..` are decided at the definition by their finite end, on ℤ's facts, so a
// body that leaves them is refused, saying which end; and a body above the
// word that keeps the sign is proven, a factorial's [1, +∞) by the product's
// extended corners (mulInf). Planted fault: the rule that shipped, which
// decided above the word only what the author stated, and accepted −5 as Nat.
func TestAnOpenRangeOnAResultIsAClaim(t *testing.T) {
	bad := map[string]string{
		"export fn h(y ∈ -5..5) (Nat) { y }\n":     "0 <= result",
		"export fn h(y ∈ 0..5) (r ∈ 10..) { y }\n": "10 <= result",
		"export fn h(y ∈ 0..5) (..-1) { y }\n":     "result <= -1",
		// bounded above the word, but held in it, where no run-time check runs
		"export fn h(y ∈ -5..5) (0..2^100) { y }\n": "0 <= result",
	}
	for src, reason := range bad {
		if _, err := doublesOn(t, "go", src); err == nil || !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: want it refused with %q, got %v", strings.TrimSpace(src), reason, err)
		}
	}
	good := []string{
		"export fn fact(n ∈ 0..30) (Nat) { loop acc = 1, i = 2 { i > n -> acc; else -> again(acc * i, i + 1) } }\n",
		"export fn h(y ∈ 0..5) (Nat) { y }\n",
	}
	for _, src := range good {
		if _, err := doublesOn(t, "go", src); err != nil {
			t.Errorf("%s: a true claim was refused: %v", strings.TrimSpace(src), err)
		}
	}
	// Planted: the run time taken to check a value the facts keep in the word.
	plantSignFault = true
	_, err := doublesOn(t, "go", "export fn h(y ∈ -5..5) (0..2^100) { y }\n")
	plantSignFault = false
	if err != nil {
		t.Errorf("the planted fault (the run time taken for a word-held value) still refused: %v", err)
	}
	plantOpenClaimFault = true
	defer func() { plantOpenClaimFault = false }()
	if _, err := doublesOn(t, "go", "export fn h(y ∈ -5..5) (Nat) { y }\n"); err == nil {
		return // the fault is seen
	}
	t.Errorf("the planted fault (the old skip) still refused −5 as Nat: the test cannot see it")
}
