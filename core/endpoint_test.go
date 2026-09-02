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

// ═══ THE UNBOUNDED RUNG — `(int 0 +inf)` (unbounded-rung.md §4.2)
//
// The type language is a lattice of subsets of Z and it could not name the top
// of it. Every range was finite, so `(int 0 (pow 2 1000))` and a value that is
// genuinely unbounded were the same declaration — and they are not the same
// thing: a FINITE range has a limb count, a `build` of known length and zero
// allocations, where an unbounded one has none of that and must fall back to
// whatever arbitrary precision the target ships.
//
// So this is not a spelling convenience. It is the distinction the
// representation ladder turns on.

// typeOf reads one range expression back as its canonical type string, or "" if
// it is not a type.
func typeOf(t *testing.T, src string) string {
	t.Helper()
	forms, err := Read("(sig f ((n " + src + ")) int)")
	if err != nil {
		return ""
	}
	if len(forms) == 0 || forms[0].Sig == nil || len(forms[0].Sig.Params) == 0 {
		return ""
	}
	return forms[0].Sig.Params[0].Type
}

func TestInfiniteEndpoints(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"(int 0 +inf)", "int 0 +inf"},
		{"(int -inf 0)", "int -inf 0"},
		{"(int -inf +inf)", "int -inf +inf"},
		{"(int (- 0 5) +inf)", "int -5 +inf"},
	} {
		if got := typeOf(t, c.src); got != c.want {
			t.Errorf("%s is %q, want %q", c.src, got, c.want)
		}
		if !UnboundedRange(typeOf(t, c.src)) {
			t.Errorf("%s is not recognised as unbounded", c.src)
		}
		// AND IT IS NOT AN `int`. Z is not inside the portable window, so the
		// promotion is a WIDENING and not a refinement — which is why
		// `compatible` refuses it where an `int` is required, and why that
		// refusal is the surface a programmer meets.
		if got := ValueType(typeOf(t, c.src)); got != BigType {
			t.Errorf("ValueType(%s) = %q, want %q", c.src, got, BigType)
		}
	}
}

// AN INFINITE ENDPOINT MUST POINT OUTWARD, and arithmetic on one is refused
// rather than defined.
//
// `(int +inf 0)` is empty, `(int +inf +inf)` is a point that is not an integer,
// and `(+ +inf 1)` is a question about the extended reals this language has no
// use for. Refusing costs nothing: nobody writes these on purpose, and defining
// them would mean answering `+inf − +inf`.
func TestInfiniteEndpointsMustPointOutward(t *testing.T) {
	for _, src := range []string{
		"(int +inf 0)", "(int 0 -inf)", "(int +inf +inf)", "(int -inf -inf)",
		"(int 0 (+ +inf 1))", "(int 0 (pow 2 +inf))", "(int (- +inf) 0)",
	} {
		if got := typeOf(t, src); got != "" {
			t.Errorf("%s was accepted as the type %q", src, got)
		}
	}
}

// A HALF-OPEN RANGE STILL SAYS SOMETHING, and saying it is §4.2's whole
// argument for `+inf` over a bare type name: a bignum accumulator that is known
// NON-NEGATIVE needs no sign handling, which bigarith-2026-08-28 measured as a
// real cost.
//
// It matters for the finite case too, and there it is a fix rather than a
// feature: `(int 0 (pow 2 1000))` was contributing NOTHING, because the
// desugaring demanded two `int64` endpoints and the upper one is not. Dropping
// a conjunct is sound — a premise is assumed, so half of it is a weaker
// assumption — while dropping the half that IS expressible says nothing at all.
func TestAHalfOpenRangeIsStillAPremise(t *testing.T) {
	for _, c := range []struct{ ty, want string }{
		{"int 0 +inf", "(<= 0 n)"},
		{"int -inf 100", "(<= n 100)"},
		{"int 0 1000", "(if (<= 0 n) (<= n 1000) false)"},
		{"int -inf +inf", ""},
	} {
		p := rangePremise(c.ty, Name("n"))
		got := ""
		if p != nil {
			got = p.String()
		}
		if got != c.want {
			t.Errorf("premise of %q is %q, want %q", c.ty, got, c.want)
		}
	}
	// AND THE ENDPOINT THAT DOES NOT FIT A TERM IS DROPPED, NOT TRUNCATED.
	// A silently narrowed bound would be a claim the program never made, and
	// this is the failure shape that has landed three times in one week.
	p := rangePremise("int 0 "+
		"10715086071862673209484250490600018105614048117055336074437503883703510511249361224931983788156958581275946729175531468251871452856923140435984577574698574803934567774824230985421074605062371141877954182153046474983581941267398767559165543946077062914571196477686542167660429831652624386837205668069376",
		Name("n"))
	if p == nil || p.String() != "(<= 0 n)" {
		t.Errorf("a 302-digit upper endpoint did not drop cleanly: %v", p)
	}
}
