package emit_test

import (
	"strings"
	"testing"
)

// A BOOLEAN OPERATION'S RESULT CHOOSES ITS REALIZATION (ADR 0083 item 2), and
// one in no realization's set is refused naming them. JavaScript's `|` is the
// or only where the result is proven in S₃₂ or, read `>>> 0`, in U₃₂: an or of
// a 40-bit value is in neither, and printing either would truncate it. The same
// program compiles on Go, where the realization is the word's.
func TestABitResultOutsideEveryRealizationIsRefused(t *testing.T) {
	src := "export fn f(a ∈ 0..2^40, b ∈ 0..255) (Int) { a | b }"
	if _, err := genOn(t, "go", src, "f"); err != nil {
		t.Fatalf("Go refused an or in its word: %v", err)
	}
	_, err := genOn(t, "js", src, "f")
	if err == nil || !strings.Contains(err.Error(), "only where the result is proven in one of") {
		t.Fatalf("JavaScript accepted a 40-bit or, or refused it for another reason: %v", err)
	}
	// AND THE U₃₂ REALIZATION IS CHOSEN where the result is proven there, not
	// the S₃₂ one, whose `|` would give the negative int32 of the same bits.
	code, err := genOn(t, "js", "export fn f(a ∈ 0..2^32-1, b ∈ 0..255) (Int) { a | b }", "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, ">>> 0") {
		t.Errorf("an or proven in U₃₂ printed without `>>> 0`:\n%s", code)
	}
	if code, err = genOn(t, "js", "export fn f(a ∈ 0..2^31-1, b ∈ 0..255) (Int) { a | b }", "f"); err != nil {
		t.Fatal(err)
	} else if strings.Contains(code, ">>>") {
		t.Errorf("an or proven in S₃₂ took the U₃₂ realization:\n%s", code)
	}
}

// FUSION STOPS AT EVERY OPERATION THAT DOES NOT DESCEND TO ℤ/2ⁿ (ADR 0082 item
// 3). `(a * b) mod 256` is computed wrapping, the product never held: accepted
// though a·b may reach 2^124, past every word. Under `/`, `%`, `>>`, a
// comparison or a `mod` by a non-power of two, the product is read as an
// integer, and wrapping it changes the answer: π₈((200 + 100) / 2) is 150, and
// wrapped (44 / 2) is 22. So each of those is refused, its product unproven.
func TestFusionStopsAtWhatDoesNotDescend(t *testing.T) {
	head := "export fn f(a ∈ 0..2^62, b ∈ 0..2^62, c ∈ 1..9) (Int) { "
	for _, body := range []string{
		"(a * b) mod 256",
		"((a * b + c) ⊕ (a * 3 << 2)) mod 2^32",
		"(~(a * b) | c) mod 2^63",
	} {
		if err := legalGo(t, head+body+" }"); err != nil {
			t.Errorf("%s was not fused: %v", body, err)
		}
	}
	for _, body := range []string{
		"((a * b) / 2) mod 256",
		"((a * b) % 7) mod 256",
		"((a * b) >> 1) mod 256",
		"((a * b) mod 255) mod 256",
		"(if a * b > c { 1 } else { 0 }) mod 256",
		"(a * b) mod 255",
		"(a * b) mod 2^65",
	} {
		if err := legalGo(t, head+body+" }"); err == nil {
			t.Errorf("%s: fused through an operation that does not descend", body)
		}
	}
}
