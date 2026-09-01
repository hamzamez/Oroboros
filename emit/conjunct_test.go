package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// ADDING A TRUE FACT MUST NEVER LOSE A PROOF.
//
// `(where (!= b 0))` discharged a division and
// `(where (and (!= b 0) (<= 0 a)))` did NOT — non-monotonic, which is the worst
// shape a prover can have: the programmer says MORE and is believed LESS.
//
// The cause is that `and` does not survive the reader (ADR 0017). `(and a b)`
// is `(if a b false)` by the time the refinement layer sees it, so the branch
// written to recurse into a conjunction matched a name that no longer exists
// and a `where` reached the solver only through `obligation` — which is
// all-or-nothing. A disequality is outside the linear fragment, so one `!=`
// conjunct threw away every conjunct including itself.
//
// Found by trying to ANNOTATE a program, not by reading the code: the first
// range added to `divmod`'s signature lost the division's own precondition.
func TestAConjunctionAssumesEachConjunct(t *testing.T) {
	for _, w := range []string{
		"(go.!= b 0)",
		"(and (go.!= b 0) (go.<= 0 a))",
		"(and (go.<= 0 a) (go.!= b 0))",                    // order must not matter
		"(and (go.!= b 0) (and (go.<= 0 a) (go.< a 100)))", // nested
		"(and (and (go.<= 0 a) (go.< a 100)) (go.!= b 0))", // nested the other way
	} {
		src := "(use go)\n(export q)\n(def q (fn (a b) (go./ a b)))\n" +
			"(sig q ((a int) (b int)) int (where " + w + "))\n"
		if err := refineDivision(t, src); err != nil {
			t.Errorf("%s did not discharge the division: %v", w, err)
		}
	}
}

// AND THE RULE IS NARROW: a `where` that says nothing about the divisor must
// still refuse. Without this the test above would pass against a layer that had
// simply started assuming everything.
func TestADivisionWithoutItsPreconditionStillRefuses(t *testing.T) {
	for _, w := range []string{
		"(go.<= 0 a)",
		"(and (go.<= 0 a) (go.< a 100))",
		"(and (go.<= 0 b) (go.<= 0 a))", // `0 <= b` does NOT give `b != 0`
	} {
		src := "(use go)\n(export q)\n(def q (fn (a b) (go./ a b)))\n" +
			"(sig q ((a int) (b int)) int (where " + w + "))\n"
		if err := refineDivision(t, src); err == nil {
			t.Errorf("%s discharged a division it does not justify", w)
		} else if !strings.Contains(err.Error(), "!= b 0") {
			t.Errorf("%s was refused for the wrong reason: %v", w, err)
		}
	}
}

// erasedAnd matches what the reader leaves, and nothing else. `or` and `not`
// share the `if` shape and must not be read as conjunctions — assuming a
// disjunct as a fact would be unsound.
func TestErasedAndMatchesOnlyAConjunction(t *testing.T) {
	p, q := core.Name("p"), core.Name("q")
	for _, c := range []struct {
		what string
		term *core.Term
		ok   bool
	}{
		{"and", core.App(core.Name("if"), p, q, core.Bool(false)), true},
		{"or", core.App(core.Name("if"), p, core.Bool(true), q), false},
		{"not", core.App(core.Name("if"), p, core.Bool(false), core.Bool(true)), false},
		{"a real conditional", core.App(core.Name("if"), p, q, core.Name("r")), false},
	} {
		if _, _, ok := erasedAnd(c.term); ok != c.ok {
			t.Errorf("%s: erasedAnd = %v, want %v", c.what, ok, c.ok)
		}
	}
}

func refineDivision(t *testing.T, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["q"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Refine(tg, "q", prog.Sigs["q"], nf)
	return err
}
