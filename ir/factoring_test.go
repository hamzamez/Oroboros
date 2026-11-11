package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// callsOn takes src's one export on windows through the whole pipeline, and
// returns the program functions its entry calls.
func callsOn(t *testing.T, src string) ([]string, error) {
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
	f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
	if err != nil {
		return nil, err
	}
	var calls []string
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall && IsProgramFunc(s.Name) {
				calls = append(calls, s.Name)
			}
		}
	})
	return calls, nil
}

const fibSrc = "export fn f(n ∈ 0..100) (String) {\n" +
	"    big_str(loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) })\n}\n"

// WHICH DEFINITIONS ARE KEPT NEVER CHANGES A VERDICT (ADR 0102, P4), when a
// call's arguments are outside its contract: with a numeral's parameter
// declared 0..1, stronger than any site's facts, each call's obligation fails,
// and the instance is inlined. THE PLANTED FAULT refuses, as ADR 0101 did, and
// the refusal is the obligation's, so the premise is computed, not assumed.
func TestACallOutsideItsContractIsInlined(t *testing.T) {
	calls, err := callsOn(t, fibSrc)
	if err != nil || len(calls) == 0 {
		t.Fatalf("the program as written: want calls of program functions, got %v, %v", calls, err)
	}
	PlantNarrowNumeral = true
	defer func() { PlantNarrowNumeral = false }()
	calls, err = callsOn(t, fibSrc)
	if err != nil {
		t.Fatalf("a contract stronger than the sites refuses the program: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("with every call outside its contract, still called: %v", calls)
	}
	PlantNoInlineFallback = true
	defer func() { PlantNoInlineFallback = false }()
	if _, err := callsOn(t, fibSrc); err == nil || !strings.Contains(err.Error(), "not inside its parameter's") {
		t.Errorf("with the fallback removed: want the call obligation's refusal, got %v: the witness cannot fail", err)
	}
}

// AND WHEN THE CONTRACT DOES NOT PROVE THE INSTANCE (ADR 0102, P4): div-small's
// quotient digit is ⌊(r·B + u)/k⌋, below B because r < k, a relation no
// declared set states. Inlined, the site's constant k bounds it; as a function
// under k ∈ [1, 2²⁸) nothing does, so its double-word step is unproven
// (M4b; at base 2³² its result's claim refused it, progfunc-2026-11-10). Made a
// program function, it is inlined, and the program is accepted. THE PLANTED
// FAULT refuses with the callee's reason.
func TestAnInstanceItsContractDoesNotProveIsInlined(t *testing.T) {
	const src = "export fn f(n ∈ 0..100) (String) {\n" +
		"    big_str(loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) } / 7)\n}\n"
	natCalled["div-small"] = true
	defer delete(natCalled, "div-small")
	calls, err := callsOn(t, src)
	if err != nil {
		t.Fatalf("an instance its contract does not prove refuses the program: %v", err)
	}
	for _, c := range calls {
		if c == natPrefix+"div-small" {
			t.Errorf("div-small, unproven under its parameters' sets, is still called")
		}
	}
	PlantNoInlineFallback = true
	defer func() { PlantNoInlineFallback = false }()
	if _, err := callsOn(t, src); err == nil || !strings.Contains(err.Error(), "div-small is not decided under its parameters' sets") {
		t.Errorf("with the fallback removed: want div-small's own refusal, got %v: the witness cannot fail", err)
	}
}
