package ir

import "testing"

// AN ARM'S GUARD BOUNDS ITS OWN DIFFERENCE (guardDiff): under p > q, p − q ≥ 1,
// so p − q − 1 − (2^63 − 1) − 1 stays inside the word, and it does not without
// the guard. THE PLANTED FAULT reads the guard against any value, under which
// p − r for an r the guard never compared is accepted, and it may leave the
// word (p = 1, r = 2^62 gives −2^62 − 2^63).
func TestAGuardBoundsItsOwnDifference(t *testing.T) {
	src := func(arm string) string {
		return "export fn f(p ∈ 0..2^62, q ∈ 0..2^62, r ∈ 0..2^62) (Int) { if p > q { " + arm +
			" - 1 - 9223372036854775807 - 1 } else { 0 } }\n"
	}
	tg, f, leg := doubleEntry(t, src("p - q"))
	if err := leg.Refusal("f", tg); err != nil {
		t.Fatalf("p − q under p > q: want it proven at least 1, got %v", err)
	}
	if err := verifyOne(tg, f); err != nil {
		t.Errorf("does not verify: %v", err)
	}
	// the mirror: q < p is the same guard, read from the other side
	tg, _, leg = doubleEntry(t, "export fn f(p ∈ 0..2^62, q ∈ 0..2^62) (Int) { if q < p { p - q - 1 - 9223372036854775807 - 1 } else { 0 } }\n")
	if err := leg.Refusal("f", tg); err != nil {
		t.Errorf("p − q under q < p: want it proven at least 1, got %v", err)
	}
	tg, _, leg = doubleEntry(t, src("p - r"))
	if leg.Refusal("f", tg) == nil {
		t.Fatal("p − r under p > q: the guard says nothing of r, want it refused")
	}
	PlantGuardDiffAnyOther = true
	defer func() { PlantGuardDiffAnyOther = false }()
	tg, _, leg = doubleEntry(t, src("p - r"))
	if leg.Refusal("f", tg) != nil {
		t.Error("with a guard read against any value p − r is still refused: the witness cannot fail")
	}
}

// EUCLID'S SUBTRACTION (Elements VII.2) is proven in the word by the guard: the
// loop's p − q and q − p are each a natural under its arm's comparison, so the
// claim 0..1000 holds of the result. Before guardDiff both were ⊤.
func TestEuclidsSubtractionIsProven(t *testing.T) {
	const src = "export fn f(a ∈ 0..1000, b ∈ 0..1000) (0..1000) { loop p = a, q = b { p == q -> p; p > q -> again(p - q, q); else -> again(p, q - p) } }\n"
	tg, _, leg := doubleEntry(t, src)
	if err := leg.Refusal("f", tg); err != nil {
		t.Errorf("Euclid's subtractive loop: want every operation and the claim proven, got %v", err)
	}
}
