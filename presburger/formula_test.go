package presburger

import "testing"

// lin builds Σ coefs + k from pairs (name, coef).
func lin(k int64, pairs ...any) Lin {
	t := Const(k)
	for i := 0; i < len(pairs); i += 2 {
		t = t.Add(Var(pairs[i].(string)), int64(pairs[i+1].(int)))
	}
	return t
}

// A FORMULA IS DECIDED BY POLARITY, then Cooper and the Omega test (ADR 0145,
// logic-research §3). The cases:
//   - divisibility by a quantifier, ∃k. y = 4k, is y's remainder 0: the
//     Gregorian rule's atom, both ways;
//   - a ∀ goal is a fresh constant: ∀a ∈ 0..9. a ≤ 9 holds, ∀a. a ≤ 9 does not;
//   - an ∃ goal, the hard polarity, is eliminated: ∃x. x = n + 1 holds for every n;
//   - a false claim has a countermodel: ∃k. y = 4k is not valid alone.
//
// The faults: every Skolem constant one name, so ∀a ∀b. a = b is believed; and
// Cooper without its lower bounds' instances, so ∃x. x = 5 is not.
func TestAFormulaIsDecidedByPolarity(t *testing.T) {
	// y = 4q + m, 0 ≤ m ≤ 3, y ≥ 0: the truncated remainder's definition
	rem := []*Formula{
		EqF(lin(0, "y", 1, "q", -4, "m", -1)),
		Ge(lin(0, "m", 1)), Ge(lin(3, "m", -1)), Ge(lin(0, "y", 1)),
	}
	div4 := Exists("k", EqF(lin(0, "y", 1, "k", -4)))
	cases := []struct {
		name  string
		hyps  []*Formula
		goal  *Formula
		valid bool
	}{
		{"m = 0 ⇔ ∃k. y = 4k", rem, Iff(EqF(Var("m")), div4), true},
		{"∃k. y = 4k alone", nil, div4, false},
		{"∀a ∈ 0..9. a ≤ 9", nil, Forall("a", Implies(And(Ge(Var("a")), Ge(lin(9, "a", -1))), Ge(lin(9, "a", -1)))), true},
		{"∀a. a ≤ 9", nil, Forall("a", Ge(lin(9, "a", -1))), false},
		{"∀n. ∃x. x = n + 1", nil, Forall("n", Exists("x", EqF(lin(-1, "x", 1, "n", -1)))), true},
		{"∀n ∈ 0... ∃x. 2x = n", nil, Forall("n", Implies(Ge(Var("n")), Exists("x", EqF(lin(0, "n", 1, "x", -2))))), false},
		{"∀a ∀b. a = b", nil, Forall("a", Forall("b", EqF(lin(0, "a", 1, "b", -1)))), false},
		{"∃x. x = 5", nil, Exists("x", EqF(lin(-5, "x", 1))), true},
	}
	for _, c := range cases {
		got := Valid(c.hyps, c.goal)
		if want := map[bool]Result{true: Unsat, false: Sat}[c.valid]; got != want {
			t.Errorf("%s: want %v (valid %v), got %v", c.name, want, c.valid, got)
		}
	}
	// (∀a. a ≥ 1) ∨ (∀b. b ≤ 0) is false (a = 0, b = 1); its negation's two
	// Skolem constants, one name, are c ≤ 0 ∧ c ≥ 1, and the claim is believed
	split := Or(Forall("a", Ge(lin(-1, "a", 1))), Forall("b", Ge(lin(0, "b", -1))))
	if r := Valid(nil, split); r != Sat {
		t.Errorf("(∀a. a ≥ 1) ∨ (∀b. b ≤ 0) is false, got %v", r)
	}
	PlantSkolemShared = true
	r := Valid(nil, split)
	PlantSkolemShared = false
	if r != Unsat {
		t.Errorf("with every Skolem constant one name the false claim should be believed, so the witness can fail: got %v", r)
	}
	PlantCooperNoBounds = true
	r = Valid(nil, Forall("n", Exists("x", EqF(lin(-1, "x", 1, "n", -1)))))
	PlantCooperNoBounds = false
	if r == Unsat {
		t.Error("with Cooper's lower-bound instances dropped, ∀n. ∃x. x = n + 1 should not be proven: the witness cannot fail")
	}
}
