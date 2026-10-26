package emit_test

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// legalGo is the drivers' legality decision on src's first export: the
// pipeline, then the refusal of an operation not proven inside the word.
func legalGo(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		return err
	}
	name := prog.Exports[0]
	_, leg, err := ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	if err != nil {
		return err
	}
	return leg.Refusal(name, tg)
}

// A DECLARED SET IS A CLAIM, AND THE FACTS CHOOSE THE REPRESENTATION (ADR 0079).
// ρ of a value is ρ of its proven fact; the consent to the rung above the word
// is a claim's demand; a declaration is the representation only where the
// claim is the fact, an entry point's parameter and result.

const incNat = "fn inc(n ∈ Nat) (Nat) { n + 1 }\n"

func TestADeclaredSetChoosesNoRepresentation(t *testing.T) {
	// ℕ inlined where the facts are small: the word, at a claimed result and at
	// an unclaimed one. Until ADR 0079 the claim's type was the value's, and f
	// computed x + 1 in the word to widen it into a *big.Int.
	for _, src := range []string{
		incNat + "export fn f(x ∈ 0..100) (0..200) { inc(x) }\n",
		incNat + "export fn f(x ∈ 0..100) { inc(x) }\n",
		"fn inc(n) (Nat) { n + 1 }\nexport fn f(x ∈ 0..100) (0..200) { inc(x) }\n",
	} {
		if out := printsGo(t, src); strings.Contains(out, "big.") {
			t.Errorf("a value proven inside the word is held in it:\n%s\n%s", src, out)
		}
	}
	// The definition checked alone (ADR 0072): an unclaimed result claims
	// nothing, so it is held as its yields are, and is not a word to refuse.
	if err := claimCheck(t, "fn inc(n ∈ Nat) { n + 1 }\nexport fn f(x ∈ 0..100) { inc(x) }\n"); err != nil {
		t.Errorf("an unclaimed result over ℕ must check: %v", err)
	}
	if err := claimCheck(t, "fn inc(n) (Nat) { n + 1 }\nexport fn f(x ∈ 0..100) (0..200) { inc(x) }\n"); err != nil {
		t.Errorf("a claim above the word against a result in it is the IR's to decide, by facts: %v", err)
	}
}

func TestTheClaimIsTheRepresentationOnlyAtAnEntryPoint(t *testing.T) {
	// An entry point's ℕ parameter: the host passes it and nothing proves it
	// smaller. Its Go type printed `/*int 0 +inf?*/` until this build.
	out := printsGo(t, incNat+"export fn e(n ∈ Nat) (Nat) { inc(n) }\n")
	if !strings.Contains(out, "(v0 *big.Int) *big.Int") {
		t.Errorf("an entry point's ℕ parameter and result are the host's bignum:\n%s", out)
	}
}

func TestTheConsentToTheRungAboveIsAClaim(t *testing.T) {
	grow := "fn grow(n ∈ Nat) (Nat) { n * n }\n"
	// A claim above the word demands the value, whose fact leaves the word.
	if out := printsGo(t, grow+"export fn g(x ∈ 0..2^40) { grow(x) }\n"); !strings.Contains(out, "*big.Int") {
		t.Errorf("a demanded value whose fact leaves the word is held exactly:\n%s", out)
	}
	// No claim, no consent: bounded by default (ADR 0019 item 2).
	if err := legalGo(t, "export fn h(x ∈ 0..2^40) { x * x }\n"); err == nil ||
		!strings.Contains(err.Error(), "cannot be proven to stay inside the word") {
		t.Errorf("an unproven operation with no claim above the word is refused: %v", err)
	}
	// An exact value where a word is claimed, refused by its fact.
	if err := claimCheck(t, grow+"fn k(x ∈ 0..2^40) (Int) { grow(x) }\nexport fn m(y ∈ 0..2^40) { k(y) }\n"); err == nil ||
		!strings.Contains(err.Error(), "WIDENING") {
		t.Errorf("a value held exactly where a word is claimed is refused: %v", err)
	}
}
