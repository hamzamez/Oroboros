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
