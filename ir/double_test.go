package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// doubleEntry takes src's one export through the pipeline on windows.
func doubleEntry(t *testing.T, src string) (*emit.Target, *Func, *Legality) {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
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
	f, leg, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tg, f, leg
}

func verifyOne(tg *emit.Target, f *Func) error {
	return Verify(tg, &Program{Target: "windows", Stage: StageP, Funcs: []*Func{f}})
}

// AN INTERMEDIATE ABOVE THE WORD, READ BACK INTO IT, IS THE DOUBLE WORD'S
// (ADR 0103): the Mersenne hash's h·x < 2¹²², and its remainder by p. Every
// operation is proven, two of them in E, and the program verifies.
func TestADoubleWordIntermediateIsComputedThere(t *testing.T) {
	tg, f, leg := doubleEntry(t, "export fn f(n ∈ 0..100000) (Int) {\n"+
		"    loop i = 0, h = 0 { i >= n -> h; else -> again(i + 1, (h * 1234567890123456789 + i % 251) % (2^61 - 1)) }\n}\n")
	if err := leg.Refusal("f", tg); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if n := len(Doubles(f)); n != 2 {
		t.Errorf("want the product and the sum in the double word, got %d values", n)
	}
	if err := verifyOne(tg, f); err != nil {
		t.Errorf("does not verify: %v", err)
	}
}

// NO VALUE OF THE PROGRAM LEAVES THE WORD (W14): a product returned is a value
// above the word, refused as ADR 0019 refuses it. THE PLANTED FAULT lets it
// into the double word anyway; the decision then proves it, and W14 refuses
// the program for the reason, independently of the pass that chose it.
func TestADoubleWordNeverLeavesItsTerm(t *testing.T) {
	const src = "export fn f(x ∈ 0..2^62, y ∈ 0..2^62) (Int) { x * y + 1 }\n"
	tg, f, leg := doubleEntry(t, src)
	if leg.Refusal("f", tg) == nil || len(Doubles(f)) != 0 {
		t.Fatalf("a product that is the function's result: want it refused and nothing in the double word, got %d values", len(Doubles(f)))
	}
	PlantDoubleEscape = true
	defer func() { PlantDoubleEscape = false }()
	tg, f, _ = doubleEntry(t, src)
	if err := verifyOne(tg, f); err == nil || !strings.Contains(err.Error(), "W14") {
		t.Errorf("an intermediate that leaves its term: want W14, got %v: the witness cannot fail", err)
	}
}

// E BOUNDS THE DOUBLE WORD: a product of three words reaches 2¹⁸⁶, its fact is
// unbounded, and the program is refused, though what reads it is a
// comparison, whose result is a Boolean whatever the operand's fact. THE
// PLANTED FAULT takes any fact into the double word, and the refusal goes.
func TestAnIntermediateOutsideEIsRefused(t *testing.T) {
	const src = "export fn f(x ∈ 0..2^62, y ∈ 0..2^62, z ∈ 0..2^62) (Int) { if x * y * z > 5 { 1 } else { 0 } }\n"
	tg, _, leg := doubleEntry(t, src)
	if leg.Refusal("f", tg) == nil {
		t.Fatal("a product of three words: want it refused")
	}
	PlantDoubleUnbounded = true
	defer func() { PlantDoubleUnbounded = false }()
	tg, _, leg = doubleEntry(t, src)
	if leg.Refusal("f", tg) != nil {
		t.Errorf("with any fact taken into the double word the product is still refused: the witness cannot fail")
	}
}

// THE TWO-DIGIT LEMMA (Knuth): for t = u·B + w, w < B, 0 ≤ u < d, ⌊t/d⌋ < B, so
// the quotient is a word and the triple's 2-by-1 division computes it. With
// u ≤ d it may be B, and the program is refused. THE PLANTED FAULT drops the
// premise u < d, and the refusal goes.
func TestATwoDigitQuotientNeedsItsTopDigitBelowTheDivisor(t *testing.T) {
	const body = "(u * 4611686018427387904 * 2 + w) / d"
	src := func(guard string) string {
		return "export fn f(u ∈ 0..2^62, w ∈ 0..2^62, d ∈ 1..2^62) (Int) { if " + guard + " { " + body + " } else { 0 } }\n"
	}
	tg, f, leg := doubleEntry(t, src("u < d"))
	if err := leg.Refusal("f", tg); err != nil {
		t.Fatalf("a window whose top digit is below the divisor: want a one-digit quotient, got %v", err)
	}
	if err := verifyOne(tg, f); err != nil {
		t.Errorf("does not verify: %v", err)
	}
	tg, _, leg = doubleEntry(t, src("u <= d"))
	if leg.Refusal("f", tg) == nil {
		t.Fatal("a top digit that may equal the divisor: want the quotient refused")
	}
	PlantQuotientUnasked = true
	defer func() { PlantQuotientUnasked = false }()
	tg, _, leg = doubleEntry(t, src("u <= d"))
	if leg.Refusal("f", tg) != nil {
		t.Errorf("with the premise unasked the quotient is still refused: the witness cannot fail")
	}
}
