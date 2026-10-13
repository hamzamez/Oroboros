package emit

import (
	"testing"

	"oroboros/core"
)

// A DISEQUALITY BESIDE AN ORDER IS STRICT (refine.go, strictFromDisequality):
// over ℤ, k ≥ 0 ∧ k ≠ 0 ⊢ k ≥ 1, the fact freq's `new?` needs to read the word
// before the k-th (claimsdemand-2026-10-13). Each case is checked against the
// integers: a goal the facts do not entail must stay unproven.
func TestADisequalityBesideAnOrderIsStrict(t *testing.T) {
	term := func(src string) *core.Term {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		return forms[0].Term
	}
	proves := func(facts []string, goal string) bool {
		f := newFacts()
		for _, s := range facts {
			assume(f, term(s))
		}
		gs, ok := f.oblig(term(goal))
		return ok && allEntailed(f, gs)
	}
	for _, c := range []struct {
		facts []string
		goal  string
		want  bool
	}{
		{[]string{"(<= 0 k)", "(!= k 0)"}, "(<= 1 k)", true},  // k ≥ 0, k ≠ 0 ⊢ k ≥ 1
		{[]string{"(<= k 9)", "(!= k 9)"}, "(<= k 8)", true},  // the other side
		{[]string{"(<= j k)", "(!= j k)"}, "(< j k)", true},   // between two variables
		{[]string{"(!= k 0)"}, "(<= 1 k)", false},             // k = −1 is a counterexample
		{[]string{"(<= 0 k)", "(!= k 5)"}, "(<= 1 k)", false}, // k = 0 is a counterexample
		{[]string{"(!= k 0)", "(<= 0 k)"}, "(<= 1 k)", false}, // the order after: not strengthened (stated)
	} {
		if got := proves(c.facts, c.goal); got != c.want {
			t.Errorf("%v ⊢ %s: got %v, want %v", c.facts, c.goal, got, c.want)
		}
	}
	// PLANTED: a < b from a ≠ b alone, so k ≠ 0 would give k < 0. The
	// counterexample k = 1 must catch it.
	plantDisequalityFault = true
	defer func() { plantDisequalityFault = false }()
	if !proves([]string{"(!= k 0)"}, "(< k 0)") {
		t.Error("the planted fault (a < b from a ≠ b alone) proved nothing, so this test cannot see it")
	}
}
