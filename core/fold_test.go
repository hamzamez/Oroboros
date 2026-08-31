package core

import "testing"

// CONSTANT FOLDING FOR THE LANGUAGE'S INTEGER OPERATORS (integers.md §11).
//
// It became possible when the operators became the language's: folding `go.+`
// would have meant assuming one host's semantics, and `+` has semantics the
// language defines and integers.md verified on all four.
//
// The division cases are the ones worth stating, because they are where hosts
// could have disagreed and do not: truncation is TOWARD ZERO, so `-7 / 2` is
// -3 and not -4, and the remainder takes the DIVIDEND's sign, so `-7 % 2` is
// -1 and `7 % -2` is 1. Go's own int64 `/` and `%` are exactly that, so the
// fold and the emission agree by construction rather than by coincidence — and
// the identity `(a/b)*b + a%b == a` holds in every row.
func foldNorm(t *testing.T, src string) string {
	t.Helper()
	prims := "(prim +)\n(prim -)\n(prim *)\n(prim /)\n(prim %)\n" +
		"(prim <)\n(prim <=)\n(prim >)\n(prim >=)\n(prim =)\n"
	return norm(t, prims+src, "")
}

func TestIntegerOperatorsFold(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"(+ 3 4)", "7"},
		{"(- 10 7)", "3"},
		{"(* 3 4)", "12"},
		{"(/ 7 2)", "3"},   // toward zero
		{"(/ -7 2)", "-3"}, // toward zero, NOT floor
		{"(% -7 2)", "-1"}, // the dividend's sign
		{"(% 7 -2)", "1"},  // the dividend's sign
		{"(< 1 2)", "true"},
		{"(<= 2 2)", "true"},
		{"(> 1 2)", "false"},
		{"(>= 2 3)", "false"},
		{"(= 3 3)", "true"},
		// Nested, because the point is that folding composes: this is what
		// makes a fused pipeline reduce to a number.
		{"(+ (* 3 4) (- 10 7))", "15"},
	} {
		if got := foldNorm(t, c.src); got != c.want {
			t.Errorf("%s folded to %s, want %s", c.src, got, c.want)
		}
	}
}

// THE IDENTITY, checked rather than asserted in a comment: `(a/b)*b + a%b == a`
// for every sign combination. integers.md §4 says all four hosts agree on it,
// and the fold has to agree with them.
func TestDivisionIdentityHoldsUnderFolding(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"7", "2"}, {"-7", "2"}, {"7", "-2"}, {"-7", "-2"}, {"6", "3"}, {"-6", "3"},
	} {
		src := "(+ (* (/ " + c.a + " " + c.b + ") " + c.b + ") (% " + c.a + " " + c.b + "))"
		if got := foldNorm(t, src); got != c.a {
			t.Errorf("(a/b)*b + a%%b for a=%s b=%s folded to %s, want %s",
				c.a, c.b, got, c.a)
		}
	}
}

// STAGING MUST NOT CHANGE AN ANSWER (ADR 0009), and for integers that is two
// side conditions rather than one.
//
// A RESULT OUTSIDE THE PORTABLE WINDOW IS NOT FOLDED. Compile time here is Go's
// int64; run time on JavaScript is a float64 whose integers are exact only to
// ±(2^53−1). Folding `9007199254740991 * 2` would produce a value the runtime
// could not have produced — and worse, would HIDE it, where leaving the
// operation alone lets the overflow analysis report it against what the
// programmer wrote.
//
// DIVISION BY ZERO IS NOT FOLDED either: it is a precondition and not a
// behaviour (integers.md §5), so the refinement layer reports it with the call
// site. Folding would panic the compiler instead.
func TestFoldingRespectsThePortableWindowAndDivisionByZero(t *testing.T) {
	for _, c := range []struct {
		src   string
		folds bool
	}{
		{"(+ 9007199254740990 1)", true},  // lands exactly on the boundary
		{"(+ 9007199254740991 1)", false}, // one past it
		{"(- -9007199254740990 1)", true},
		{"(- -9007199254740991 1)", false},
		{"(* 9007199254740991 2)", false},
		// int64 itself overflows here, so a range test on the WRAPPED product
		// would pass. The fold divides back instead.
		{"(* 4611686018427387904 4)", false},
		{"(/ 1 0)", false},
		{"(% 1 0)", false},
	} {
		got := foldNorm(t, c.src)
		folded := got != "" && got[0] != '('
		if folded != c.folds {
			verb := "did not fold"
			if folded {
				verb = "folded to " + got
			}
			t.Errorf("%s %s, want folds=%v", c.src, verb, c.folds)
		}
	}
}

// AND NO FLOAT FOLDS, which is ADR 0009's original case and the reason the
// table is integers-only. Go folds `0.1+0.2` to `0.3` at compile time and
// `0.30000000000000004` at run time, because its untyped constants are
// arbitrary-precision — so a float folder written the natural way in Go makes
// partial evaluation unsound.
func TestNoFloatFolds(t *testing.T) {
	if got := foldNorm(t, "(+ 0.1 0.2)"); got != "(+ 0.1 0.2)" {
		t.Errorf("a float addition folded to %s; ADR 0009 forbids it", got)
	}
}
