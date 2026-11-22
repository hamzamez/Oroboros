package core

import (
	"fmt"
	"testing"
)

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
	return normCore(t, src, Word{}, "+", "-", "*", "/", "%", "<", "<=", ">", ">=", "=")
}

// normCore normalizes a core term written as its text (ParseTerm) under the
// named primitives, word the target's or zero for the tests' default: the
// reducer's own laws, below any surface.
func normCore(t *testing.T, src string, word Word, prims ...string) string {
	t.Helper()
	term, err := ParseTerm(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	env := testEnv(&Program{Defs: map[string]*Term{}}, prims...)
	if word != (Word{}) {
		env.Word = word
	}
	out, err := Normalize(term, env, DefaultFuel)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return out.String()
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
// A FOLD IS AN EQUATION OF ℤ (ADR 0114, ADR 0156). A literal is an element of
// ℤ, so a fold of the language's operators on literals gives the integer at any
// size, past JavaScript's 2^53 and past int64 alike, its value past int64 the
// numeral the reader writes for a literal. These rows were once refused for
// the window; the danger they guarded is a WRAPPED value, so each states the
// exact integer, and a fold that wrapped would show another.
//
// DIVISION BY ZERO IS NOT FOLDED: it is a precondition and not a behaviour
// (integers.md §5), refused by the decision with its call site. Folding would
// panic the compiler instead.
func TestAFoldIsExactInTheIntegers(t *testing.T) {
	value := func(src string) (string, bool) {
		t.Helper()
		term, err := ParseTerm(foldNorm(t, src))
		if err != nil {
			t.Fatal(err)
		}
		if term.String() == src {
			return "", false
		}
		if term.Kind == KInt {
			return fmt.Sprint(term.Int), true
		}
		v, ok := EvalEndpoint(term)
		if !ok {
			return "", false
		}
		return v.String(), true
	}
	for _, c := range []struct{ src, want string }{
		{"(+ 9007199254740990 1)", "9007199254740991"},
		{"(+ 9007199254740991 1)", "9007199254740992"},
		{"(- -9007199254740991 1)", "-9007199254740992"},
		{"(* 9007199254740991 2)", "18014398509481982"},
		// int64 itself overflows here: the fold is exact, not a wrapped product
		{"(* 4611686018427387904 4)", "18446744073709551616"},
		{"(* (* 4611686018427387904 4) 3)", "55340232221128654848"},
		{"(/ (* 4611686018427387904 4) 7)", "2635249153387078802"},
		{"(% (* 4611686018427387904 4) 7)", "2"},
		{"(- 0 (* 4611686018427387904 4))", "-18446744073709551616"},
	} {
		if got, ok := value(c.src); !ok || got != c.want {
			t.Errorf("%s folded to %q (folded %v), want %s", c.src, got, ok, c.want)
		}
	}
	for _, src := range []string{"(/ 1 0)", "(% 1 0)", "(/ (* 4611686018427387904 4) 0)"} {
		if _, ok := value(src); ok {
			t.Errorf("%s folded; a division by zero is the decision's to refuse", src)
		}
	}
	// THE PLANTED FAULT: folding only inside the word leaves 2^62 · 4 an
	// operation again
	PlantFoldInWord = true
	_, ok := value("(* 4611686018427387904 4)")
	PlantFoldInWord = false
	if ok {
		t.Error("with the fold held to the word, 2^62 · 4 still folded: the witness cannot fail")
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

// ON A 64-BIT WORD THE FOLD IS STILL EXACT: each row is a place where the
// wrapped int64 would itself lie inside the word, so a fold computed in int64
// and range-tested would give a wrong integer. The fold gives the integer.
func TestFoldingOnAFullWordIsExact(t *testing.T) {
	fold := func(src string) string {
		t.Helper()
		term, err := ParseTerm(normCore(t, src, testGo, "+", "-", "*", "/", "%"))
		if err != nil {
			t.Fatal(err)
		}
		if term.Kind == KInt {
			return fmt.Sprint(term.Int)
		}
		v, ok := closedInt(term, testGo)
		if !ok {
			return term.String()
		}
		return v.String()
	}
	for _, c := range []struct{ src, want string }{
		{"(+ 9007199254740991 1)", "9007199254740992"},
		{"(* 3037000499 3037000499)", "9223372030926249001"},
		{"(+ 9223372036854775807 1)", "9223372036854775808"},
		{"(- -9223372036854775808 1)", "-9223372036854775809"},
		{"(- 0 -9223372036854775808)", "9223372036854775808"},
		{"(* -9223372036854775808 -1)", "9223372036854775808"},
		{"(* -1 -9223372036854775808)", "9223372036854775808"},
		{"(/ -9223372036854775808 -1)", "9223372036854775808"},
		{"(* 4294967296 4294967296)", "18446744073709551616"},
	} {
		if got := fold(c.src); got != c.want {
			t.Errorf("%s gave %s, want %s", c.src, got, c.want)
		}
	}
}
