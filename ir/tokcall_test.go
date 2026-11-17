package ir

import (
	"strings"
	"testing"
)

// factLoopSrc is the measured shape (alloc-2026-11-17): a numeral times a
// word at every step, the old numeral dying at each call.
const factLoopSrc = "export fn f(n ∈ 0..300) (String) {\n" +
	"    big_str(loop acc = 1, i = 2 { i > n -> acc; else -> again(acc * i, i + 1) })\n}\n"

func has(calls []string, name string) bool {
	for _, c := range calls {
		if c == name {
			return true
		}
	}
	return false
}

// A DEAD REGION CROSSES A CALL AS THE CALLEE'S TOKEN (ADR 0125). In the
// factorial loop each step's `mul-small` makes its result while the caller's
// last numeral dies after the call. R1d carries the region that died at the
// previous step across the back edge, and the call takes it: the loop calls
// the token variant `nat.mul-small~tok`, whose result's storage takes the
// token by fit. The planted faults:
//   - PlantNoTokenCall hands no token to a call, and the loop calls the plain
//     instance again, so the test reaches the rule;
//   - PlantReuseKeepRelease keeps the release of the region handed on, freed
//     and then written, and W12 refuses it;
//   - PlantFitCount hands the call a count not its token's, and W12 refuses it.
func TestADeadRegionCrossesACall(t *testing.T) {
	calls, err := callsOn(t, factLoopSrc)
	if err != nil {
		t.Fatalf("the factorial loop: %v", err)
	}
	if !has(calls, natPrefix+"mul-small"+TokSuffix) {
		t.Fatalf("the factorial loop should call mul-small's token variant, calls %v", calls)
	}
	PlantNoTokenCall = true
	calls, err = callsOn(t, factLoopSrc)
	PlantNoTokenCall = false
	if err != nil || has(calls, natPrefix+"mul-small"+TokSuffix) || !has(calls, natPrefix+"mul-small") {
		t.Errorf("with no token handed to a call: want the plain instance called, got %v, %v", calls, err)
	}
	for _, c := range []struct {
		name  string
		plant *bool
		want  string
	}{
		{"a token handed on and released", &PlantReuseKeepRelease, "W12"},
		{"a count not the token's", &PlantFitCount, "W12"},
	} {
		*c.plant = true
		_, err := callsOn(t, factLoopSrc)
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want W12's refusal, got %v", c.name, err)
		}
	}
}

// A TOKEN VARIANT EXISTS WHERE ITS RESULT IS ONE TABLE (ADR 0125 item 2):
// mul-small and add, whose every return yields one build's table, have one on
// every target K(ℕ*) serves; div, whose returns yield of(0) on some paths,
// has none. THE PLANTED FAULT takes the first build as the result's, and W12
// refuses the variant whose token is then consumed on some returns only.
func TestATokenVariantIsItsResultsStorage(t *testing.T) {
	for _, tg := range numeralTargets(t) {
		for _, fn := range []string{"mul-small", "add"} {
			if _, err := buildNatCallee(tg, fn+TokSuffix, natPrefix+fn+TokSuffix); err != nil {
				t.Errorf("%s: %s's token variant: %v", tg.Name, fn, err)
			}
		}
		if _, err := buildNatCallee(tg, "div"+TokSuffix, natPrefix+"div"+TokSuffix); err == nil {
			t.Errorf("%s: div returns two tables, and has a token variant", tg.Name)
		}
		PlantTokenAnyBuild = true
		_, err := buildNatCallee(tg, "div"+TokSuffix, natPrefix+"div"+TokSuffix)
		PlantTokenAnyBuild = false
		if err == nil || !strings.Contains(err.Error(), "W12") {
			t.Errorf("%s: with the first build taken as the result's, W12 should refuse div's variant; got %v", tg.Name, err)
		}
	}
}
