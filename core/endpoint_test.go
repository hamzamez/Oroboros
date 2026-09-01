package core

import (
	"math/big"
	"testing"
)

// A RANGE ENDPOINT IS A COMPILE-TIME EXPRESSION (unbounded-rung.md).
//
// `(int 0 (pow 2 70))` names the set [0, 2^70−1] without any big literal ever
// entering the value language. That is the whole trick, and it rests on one
// distinction: ADR 0012 constrains the integers a program COMPUTES WITH, and an
// endpoint DESCRIBES A SET. So the expression is evaluated here, at arbitrary
// precision, and never becomes a term — no eighth term kind, no widening of
// `KInt`, and no big literal for the reader to refuse.
//
// `pow` and not `^`, because `^` is XOR on Go, JavaScript and Java, and a name
// should say what an operation IS.
func TestEndpointExpressions(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"(sig f ((n (int 0 1000))) int)", "int 0 1000"},
		{"(sig f ((n (int 0 (* 1000 1000)))) int)", "int 0 1000000"},
		{"(sig f ((n (int (- 0 5) 5))) int)", "int -5 5"},
		{"(sig f ((n (int 0 (+ (* 2 3) 1)))) int)", "int 0 7"},
		{"(sig f ((n (int 0 (pow 2 10)))) int)", "int 0 1024"},
		// Past the word, which is the point: exact, and no literal was written.
		{"(sig f ((n (int 0 (pow 2 70)))) int)", "int 0 1180591620717411303424"},
		{"(sig f ((n (int 0 (pow 10 30)))) int)", "int 0 1000000000000000000000000000000"},
	} {
		forms, err := Read(c.src)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := forms[0].Sig.Params[0].Type; got != c.want {
			t.Errorf("%s gave %q, want %q", c.src, got, c.want)
		}
	}
}

// A LITERAL PAST THE WINDOW IS STILL REFUSED AS A VALUE, which is the other
// half of the distinction and the thing that must not regress. The endpoint
// grammar exists so that a BOUND can exceed the word; a value may not.
func TestABigLiteralIsStillNotAValue(t *testing.T) {
	if _, err := Read("(def f (fn () 9223372036854775808))"); err == nil {
		t.Error("a literal past int64 was accepted as a value; ADR 0012's window " +
			"is about what a program computes with, and widening a BOUND must " +
			"not widen that")
	}
}

// THE GRAMMAR IS DELIBERATELY TINY, and what it refuses matters as much as what
// it accepts: an endpoint is written by a person to say how big something gets,
// not computed. Division is absent because it has a precondition, and a bound
// with a precondition is not a bound.
func TestEndpointGrammarRefusesTheRest(t *testing.T) {
	for _, src := range []string{
		"(sig f ((n (int 0 (/ 100 5)))) int)",  // division: has a precondition
		"(sig f ((n (int 0 (pow 2 -1)))) int)", // negative exponent
		"(sig f ((n (int 0 (pow 2 99999999)))) int)",
		"(sig f ((n (int 0 x))) int)", // a name is not a compile-time integer
		"(sig f ((n (int 5 0))) int)", // empty: no value inhabits it
	} {
		if _, err := Read(src); err == nil {
			t.Errorf("%s was accepted as a type", src)
		}
	}
}

// ExceedsWindow is the test that separates a REFINEMENT from a WIDENING, which
// is the objection every candidate spelling had to face. A range inside the
// window is an `int` that satisfies a bound; one outside it is not an `int` at
// all, and must be refused where an `int` is required.
func TestExceedsWindowSeparatesRefinementFromWidening(t *testing.T) {
	w := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 53), big.NewInt(1))
	for _, c := range []struct {
		ty   string
		over bool
	}{
		{"int 0 1000", false},
		{"int 0 " + w.String(), false},                                 // exactly the window
		{"int 0 " + new(big.Int).Add(w, big.NewInt(1)).String(), true}, // one past
		{"int -" + new(big.Int).Add(w, big.NewInt(1)).String() + " 0", true},
		{"int 0 1180591620717411303424", true},
		{"int", false}, // not a range at all
		{"f64", false},
		{"array int", false},
	} {
		if got := ExceedsWindow(c.ty); got != c.over {
			t.Errorf("ExceedsWindow(%q) = %v, want %v", c.ty, got, c.over)
		}
	}
	// And a range inside the window still normalises to `int`, or every existing
	// program breaks.
	if got := ValueType("int 0 1000"); got != "int" {
		t.Errorf("an in-window range normalised to %q, want int", got)
	}
	// While one outside it does NOT, which is what makes `compatible` refuse it.
	if got := ValueType("int 0 1180591620717411303424"); got == "int" {
		t.Error("a range wider than the window normalised to `int`; it would then " +
			"be accepted wherever an int is wanted and truncate silently")
	}
}
