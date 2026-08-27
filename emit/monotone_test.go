package emit

import (
	"testing"

	"oroboros/core"
)

// LOOP MONOTONICITY — monotone.go, docs/spec/postconditions.md §7.
//
// The corollary has TWO halves and needs both: every `again` must not decrease
// the position, AND every exit must be at least it. A test for only the first
// would pass a loop that counts up and then returns zero.

func lowerBound(t *testing.T, src string) string {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	z := LoopLowerBound(tg, forms[0].Term)
	if z == nil {
		return ""
	}
	return z.String()
}

func TestLoopLowerBound(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		// A SCANNER. j starts at i+1 and every path only adds, so the value is
		// at least i+1 — which is what makes a caller's index strictly increase.
		{"a scanner", `
			(loop ((j (go.+ i 1)))
			  (go.>= j (len src))  j
			  (= (src j) 92)       (again (go.+ j 2))
			  (= (src j) 34)       (go.+ j 1)
			  else                 (again (go.+ j 1)))`, "(go.+ i 1)"},

		// Rule 3: subtracting a NEGATIVE literal is an increase.
		{"minus a negative", `
			(loop ((j i)) (go.>= j 10) j else (again (go.- j -2)))`, "i"},

		// Rule 4: a conditional qualifies when BOTH branches do.
		{"a conditional step", `
			(loop ((j i)) (go.>= j 10) j else (again (if c (go.+ j 1) (go.+ j 5))))`, "i"},

		// Rule 5, and ADR 0015's `again` under a `let`.
		{"again under a let", `
			(loop ((j i)) (go.>= j 10) j else (let (go.+ j 3) (fn (n) (again n))))`, "i"},

		// --- and what it must refuse ---

		{"a decreasing step", `
			(loop ((j i)) (go.<= j 0) j else (again (go.- j 1)))`, ""},

		// THE SECOND HALF. Every step increases, but an exit returns 0 — so the
		// loop's value is NOT at least its initial value. A check of the steps
		// alone would wrongly accept this.
		{"an exit below the initial value", `
			(loop ((j i)) (go.>= j 10) 0 else (again (go.+ j 1)))`, ""},

		// One bad branch is enough: the conditional is only as good as its worse
		// side, because either may be the one evaluated.
		{"one bad branch", `
			(loop ((j i)) (go.>= j 10) j else (again (if c (go.+ j 1) (go.- j 1))))`, ""},

		// An opaque step. Nothing says `(f j)` is at least j.
		{"an opaque step", `
			(loop ((j i)) (go.>= j 10) j else (again (go.* j 2)))`, ""},

		// Adding a VARIABLE, not a literal: `j + k` is below j when k < 0.
		{"plus an unknown", `
			(loop ((j i)) (go.>= j 10) j else (again (go.+ j k)))`, ""},
	} {
		if got := lowerBound(t, c.src); got != c.want {
			t.Errorf("%s: lower bound %q, want %q", c.name, got, c.want)
		}
	}
}

// Lemma A, directly. Each rule is a true arithmetic fact and each non-rule is
// refused, because refusing is the safe direction.
func TestAtLeastIsTheRelationAndNothingMore(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	s := map[string]bool{"j": true}
	for _, c := range []struct {
		src  string
		want bool
	}{
		{"j", true},                    // rule 1
		{"(go.+ j 0)", true},           // rule 2, c = 0
		{"(go.+ j 7)", true},           // rule 2
		{"(go.+ 7 j)", true},           // rule 2, commuted
		{"(go.+ j -1)", false},         // c < 0 is a decrease
		{"(go.- j -3)", true},          // rule 3
		{"(go.- j 3)", false},          // subtraction is not commutative
		{"(go.- 3 j)", false},          // and 3 - j is not at least j
		{"(if c j (go.+ j 1))", true},  // rule 4
		{"(if c j (go.- j 1))", false}, // rule 4 needs both
		{"k", false},                   // not in S
		{"(go.* j 2)", false},          // outside the relation
		{"3", false},                   // a literal says nothing about j
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := atLeast(tg, forms[0].Term, s); got != c.want {
			t.Errorf("atLeast(%s) = %v, want %v", c.src, got, c.want)
		}
	}
}

// RUNNING EXTREMUM — monotone.go, monotone-2026-08-27 §5.
//
// `mx = max(mx, sp+1)` hands the variable back in one branch, so the fixpoint's
// `next` always contains `cur` and the bound can never shrink. The theorem says
// the reachable set is {z} ∪ U, so the pass-through contributes nothing and no
// widening is needed.
func TestSelfContained(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, src string
		want      bool
	}{
		{"a running maximum", "(if (go.> (go.+ sp 1) mx) (go.+ sp 1) mx)", true},
		{"the pass-through alone", "mx", true},
		{"a self-free expression", "(go.+ sp 1)", true},
		{"a literal", "7", true},
		{"nested conditionals", "(if a mx (if b (go.+ sp 1) mx))", true},

		// The CONDITION may mention the variable freely: it produces no value.
		{"a condition mentioning it", "(if (go.> mx 3) sp mx)", true},

		// And what it must refuse. `(+ mx 1)` is a genuine accumulator: the
		// reachable set is not closed after one step and the fixpoint is right
		// to widen it.
		{"an accumulator", "(if c (go.+ mx 1) mx)", false},
		{"an accumulator, bare", "(go.+ mx 1)", false},
		{"a branch computing from it", "(if c (go.* mx 2) mx)", false},
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := selfContained(tg, forms[0].Term, "mx"); got != c.want {
			t.Errorf("%s: selfContained(%s) = %v, want %v", c.name, c.src, got, c.want)
		}
	}
}

// The whole point, end to end: a running maximum over a guarded variable is
// bounded, where before it widened to infinity.
func TestRunningMaximumIsBounded(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(use go)
(def run (fn (n)
  (loop ((i 0) (sp 0) (mx 0))
    (go.>= sp 32)  mx
    (go.>= i 100)  mx
    else           (again (go.+ i 1)
                          (if (go.> (go.+ sp 1) 4) 0 (go.+ sp 1))
                          (if (go.> (go.+ sp 1) mx) (go.+ sp 1) mx)))))`
	forms, _ := core.Read(src)
	prog, _, err := core.LoadWith(forms, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, _ := tg.Env(prog)
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := Intervals(tg, nil, nf, 0)
	if rep.Ops != rep.Proven {
		t.Errorf("a running maximum over a guarded variable must be bounded: "+
			"%d of %d operations proven", rep.Proven, rep.Ops)
	}
	if !rep.FitsIndex() {
		t.Errorf("and its range must fit an index: %s", rep.MaxOpRange())
	}
}
