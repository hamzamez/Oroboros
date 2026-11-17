package ir

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// THE TWO-DIGIT LEMMA READS THE UNSIGNED WORD (ADR 0124). At B = 2³²
// Algorithm D's window is the word's: q̂ = ⌊(w_n·B + w_{n−1}) / v_{n−1}⌋ is
// `u64/` of `u64+` of a `u64*` by B, its operands converted into the unsigned
// word, and the lemma bounds it below B as it does ℤ's own division. With it,
// windows32's K(ℕ*) `div` and `render` are decided under their parameters'
// sets, so each is one function of the program, as on x86_64. THE PLANTED
// FAULT reads the lemma on ℤ's operations alone, as before: `div` is then not
// decided, q̂'s bound lost to the signed reading of the unsigned word.
func TestTheTwoDigitLemmaReadsTheUnsignedWord(t *testing.T) {
	tg, err := emit.LoadTargetLayers("windows32", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []string{"div", "render"} {
		if _, err := buildNatCallee(tg, fn, "nat."+fn); err != nil {
			t.Errorf("windows32's K(ℕ*) %s is not decided under its parameters' sets: %v", fn, err)
		}
	}
	PlantQuotientWordOnly = true
	_, err = buildNatCallee(tg, "div", "nat.div")
	PlantQuotientWordOnly = false
	if err == nil || !strings.Contains(err.Error(), "cannot be proven") {
		t.Errorf("with the lemma on ℤ's operations alone, windows32's div should not be decided, so the test reaches the rule; got %v", err)
	}
}
