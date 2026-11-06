package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A COMPARISON THE FACTS DECIDE IS ITS VALUE (llvm64-2026-11-06). On a target
// with the unsigned word, x ∈ [0, 2⁶⁴) against a negative constant has no word
// holding both operands, but x < c is false whatever x is, so it needs none:
// it is the constant, and the πs that stated it are dropped. Before, it was
// refused, and render : ℤ → Σ* could not print a 64-bit hash.
func TestAComparisonTheFactsDecideIsItsValue(t *testing.T) {
	const src = "export fn f(n ∈ 0..255) (Int) {\n" +
		"    h = (n * 1099511628211 + 14695981039346656037) mod 2^64\n" +
		"    if h < -5 { 1 } else { if h % 7 == 3 { 2 } else { 3 } }\n}\n"
	entry := func() error {
		t.Helper()
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
		f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
		if err != nil {
			return err
		}
		return ToP(tg, f)
	}
	if err := entry(); err != nil {
		t.Errorf("a U value against a negative constant is decided by the facts, and the program verifies; got %v", err)
	}

	// THE PLANTED FAULT: the comparison refused, as the pass did.
	PlantDecidedFault = true
	defer func() { PlantDecidedFault = false }()
	if err := entry(); err == nil || !strings.Contains(err.Error(), "no realization holds both") {
		t.Errorf("with the decided comparison refused, want the refusal it was; got %v", err)
	}

	// AND THE πs: a decided comparison inside a disjunction states literals on
	// the arm, which a constant no longer implies (Theorem E). Kept, W6 refuses.
	PlantDecidedFault = false
	PlantPiFault = true
	defer func() { PlantPiFault = false }()
	if err := entry(); err == nil || !strings.Contains(err.Error(), "Theorem E") {
		t.Errorf("with the decided comparison's πs kept, want W6's refusal; got %v", err)
	}
}
