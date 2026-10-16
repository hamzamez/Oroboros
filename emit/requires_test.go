package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A DEFINITION'S CONTRACT IS CHECKED AT ITS CALLS (ADR 0028). Each test runs the
// pipeline's own two steps — reduce with the contracts installed, then
// DischargeRequires — on the Go target, and each pins one rule with a program
// whose verdict changes when the rule is removed.

// dischargeGo reduces the program's first export with its contracts marked and
// discharges them, returning the stripped residual or the refusal.
func dischargeGo(t *testing.T, src string) (*core.Term, error) {
	t.Helper()
	tg := goNative(t)
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
	reqs := emit.InstallRequires(env, prog)
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return emit.DischargeRequires(reqs, tg, q, prog.Sigs[q], nf, func(x *core.Term) *core.Term {
		return ir.DischargeRanges(tg, prog.Sigs[q], x)
	})
}

// A RANGE, AT A CALL: an argument the caller cannot keep inside the declared
// range is refused, one it can is not, and a literal is decided as the call
// reduces. `digits18` given a value past 10^18 printed its low 18 digits.
func TestADeclaredRangeIsAnObligationAtTheCall(t *testing.T) {
	const g = `fn g(x ∈ 0..9) (Int) { x + 1 }
`
	if _, err := dischargeGo(t, g+`export fn f(n ∈ 0..1000) (Int) { g(n) }`); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("n ∈ [0, 1000] is not inside g's [0, 9]; got %v", err)
	}
	if _, err := dischargeGo(t, g+`export fn f(n ∈ 0..9) (Int) { g(n) }`); err != nil {
		t.Errorf("n ∈ [0, 9] keeps g's contract: %v", err)
	}
	if _, err := dischargeGo(t, g+`export fn f(n ∈ Int) (Int) { g(12) }`); err == nil || !strings.Contains(err.Error(), "a call passes 12") {
		t.Errorf("the literal 12 is outside [0, 9]; got %v", err)
	}
}

// EACH LAYER PROVES WHAT IT CAN. `Rem64(h, l, k)` is ≥ 0 by its declared result,
// which the interval analysis reads, and < k by its `ensures`, which only the
// refinement layer reads: neither proves `[0, k − 1]` alone.
func TestIntervalsAndEnsuresShareARange(t *testing.T) {
	_, err := dischargeGo(t, `use go/math/bits as b
fn d(g ∈ 0..999999999999999999) (Int) { g % 10 + 1 }
export fn f(h ∈ 0..2^64-1, l ∈ 0..2^64-1) (Int) { d(b.Rem64(h, l, 1000000000000000000)) }`)
	if err != nil {
		t.Errorf("Rem64's result is in [0, 10^18 − 1] by its range and its ensures together: %v", err)
	}
	// With no primitive around the argument — the definition returns it — the
	// walk collects nothing on its way down, and the mark must collect the
	// guarantee itself.
	_, err = dischargeGo(t, `use go/math/bits as b
fn d(g ∈ 0..999999999999999999) (Int) { g }
export fn f(h ∈ 0..2^64-1, l ∈ 0..2^64-1) (Int) { d(b.Rem64(h, l, 1000000000000000000)) }`)
	if err != nil {
		t.Errorf("the mark collects Rem64's ensures when nothing above it does: %v", err)
	}
}

// ABOVE THE WORD, A RANGE IS ITS BIT LENGTH. `fact`'s declared result is
// enforced as "at most 201 bits", and the argument it feeds declares the same
// type: the two are one set only when both are read at bit length. A narrower
// argument range is not implied, and is refused.
func TestARangeAboveTheWordIsItsBitLength(t *testing.T) {
	const src = `fn fact(n ∈ 0..40) (0..2^200) {
    loop acc = 1, i = 2 { i > n -> acc; else -> again(acc * i, i + 1) }
}
fn render(x ∈ 0..RANGE) (Int) { if x == 0 { 0 } else { 1 } }
export fn f(n ∈ 0..15) (Int) { render(fact(25 + n)) }`
	if _, err := dischargeGo(t, strings.Replace(src, "RANGE", "2^200", 1)); err != nil {
		t.Errorf("the same type as argument and as result is one set: %v", err)
	}
	if _, err := dischargeGo(t, strings.Replace(src, "RANGE", "2^100", 1)); err == nil {
		t.Error("a 201-bit result does not fit a 101-bit parameter")
	}
	// Where bit length and exactness DISAGREE: a result declared up to 2^201 − 1
	// into a parameter declared up to 2^200. Exactly, [0, 2^201 − 1] ⊄ [0, 2^200];
	// by enforcement both are "at most 201 bits", one set.
	wider := strings.Replace(strings.Replace(src, "RANGE", "2^200", 1),
		"fn fact(n ∈ 0..40) (0..2^200)", "fn fact(n ∈ 0..40) (0..2^201-1)", 1)
	if !strings.Contains(wider, "(0..2^201-1)") {
		t.Fatal("the test did not rewrite the result range")
	}
	if _, err := dischargeGo(t, wider); err != nil {
		t.Errorf("two ranges of one bit length are one set above the word: %v", err)
	}
}

// A `where`, AT A CALL, over the call's arguments — an exported definition's
// included, when the call is inside the program. A closed condition is decided
// with no context; one the caller's facts refute is refused.
func TestAWhereIsAnObligationAtTheCall(t *testing.T) {
	const h = `fn h(a ∈ Int, b ∈ Int) (Int) where a < b { b - a }
`
	if _, err := dischargeGo(t, h+`export fn f(n ∈ 0..10) (Int) { h(n, 20) }`); err != nil {
		t.Errorf("n ≤ 10 < 20: %v", err)
	}
	if _, err := dischargeGo(t, h+`export fn f(n ∈ 0..30) (Int) { h(n, 20) }`); err == nil || !strings.Contains(err.Error(), "h requires") {
		t.Errorf("n may reach 30, past 20; got %v", err)
	}
	if _, err := dischargeGo(t, h+`export fn f(n ∈ Int) (Int) { h(5, 3) }`); err == nil {
		t.Error("(< 5 3) is false at the call and must be refused")
	}
	// A PURE ARGUMENT GOES INTO THE CONDITION AS ITSELF. The table is used twice,
	// so β binds it to a name whose length nothing knows; read as the literal, its
	// length folds to 3 (examples/json/tokenize.oro's shape).
	if _, err := dischargeGo(t, `fn s2(s ∈ Array(Int)) (Int) where len(s) < 10 { s[0] + s[1] }
export fn f(n ∈ Int) (Int) { s2(array { 1, 2, 3 }) }`); err != nil {
		t.Errorf("a three-element literal is shorter than 10: %v", err)
	}
}

// AN INFINITE ENDPOINT'S FINITE SIDE IS STILL AN OBLIGATION: `(int 0 +inf)` is
// `0 <= n`, carried by the call's own condition.
func TestAnInfiniteRangesFiniteSideIsChecked(t *testing.T) {
	if _, err := dischargeGo(t, "fn k(n ∈ 0..`+inf`) (Int) { n + 1 }\nexport fn f(m ∈ -5..5) (Int) { k(m) }"); err == nil {
		t.Error("m may be −5, below k's 0")
	}
}

// THE MARKS CHANGE NOTHING THEY DO NOT CHECK. A where-mark sits on a call's
// result, so a result that is a tuple — applied to its eliminator — or a literal
// a fold must see is hoisted past; once discharged and stripped, the residual is
// the one reduction gives with no contracts installed.
func TestTheMarksLeaveTheResidualUnchanged(t *testing.T) {
	const src = `fn dm(a ∈ Int, b ∈ Int) (Int × Int) where 0 < b { (a / b, a % b) }
fn two(x ∈ 0..9) (Int) where 0 <= x { 2 }
export fn f(a ∈ 0..100, b ∈ 1..100) (Int) { ({ (q, r) = dm(a, b); q + r }) + two(3) * two(a % 10) }`
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	// The plain residual: the same reduction with no contract in the
	// environment, so no mark is ever placed.
	plain, _ := tg.Env(prog)
	plain.Requires, plain.Wheres = nil, nil
	want, err := core.Normalize(prog.Defs["f"], plain, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dischargeGo(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want.String() {
		t.Errorf("the discharged residual differs from the plain one:\n got  %s\n want %s", got, want)
	}
}

// THE SIGN IS PART OF A PARAMETER'S SET, as an obligation at a call: a negative
// value is not in `(int 0 (pow 2 200))`, whether it arrives as a literal or as
// an interval the analysis found.
func TestTheSignIsPartOfAParametersSet(t *testing.T) {
	const g = "fn g(x ∈ LO..2^200) (Int) { if x == 0 { 0 } else { 1 } }\nexport fn f(n ∈ -5..5) (Int) "
	nat := strings.Replace(g, "LO", "0", 1)
	zed := strings.Replace(g, "LO", "-2^200", 1)
	if _, err := dischargeGo(t, nat+`{ g(-5) }`); err == nil || !strings.Contains(err.Error(), "a call passes -5") {
		t.Errorf("-5 is not in [0, 2^201); got %v", err)
	}
	if _, err := dischargeGo(t, zed+`{ g(-5) }`); err != nil {
		t.Errorf("-5 is in (−2^201, 2^201): %v", err)
	}
	if _, err := dischargeGo(t, nat+`{ g(n) }`); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("n ∈ [−5, 5] is not in [0, 2^201); got %v", err)
	}
	if _, err := dischargeGo(t, nat+`{ g(n + 5) }`); err != nil {
		t.Errorf("n + 5 ∈ [0, 10] is in [0, 2^201): %v", err)
	}
}

// A DECLARED RESULT ABOVE THE WORD TELLS ONLY WHAT THE PROGRAM ENFORCES. The
// bound is one per program, so `h`'s declared 2^100 is checked at the program's
// 2^201, and h may return 2^150: read as its own type, the ascription proved
// g's obligation falsely, and g received 2^150 (ADR 0028's reading, corrected
// by ADR 0029). A result as wide as the program's widest type still proves one.
func TestADeclaredResultIsAFactAboutTheProgramsSet(t *testing.T) {
	const src = `fn h(n ∈ 0..1) (0..2^100) { (2^150) * (n + 1) }
fn g(x ∈ 0..2^100) (0..2^200) { x + 0 }
export fn calc(n ∈ 0..1) (0..2^WIDEST) { g(h(n)) }`
	if _, err := dischargeGo(t, strings.Replace(src, "WIDEST", "200", 1)); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("h's 2^100 is enforced only at the program's 2^201; got %v", err)
	}
	// With nothing wider than 2^100 in the program, E_P = [0, 2^101) = g's set.
	one := strings.Replace(strings.Replace(src, "WIDEST", "100", 1),
		"fn g(x ∈ 0..2^100) (0..2^200)", "fn g(x ∈ 0..2^100) (0..2^100)", 1)
	if !strings.Contains(one, "fn g(x ∈ 0..2^100) (0..2^100)") {
		t.Fatal("the test did not narrow g's result")
	}
	if _, err := dischargeGo(t, one); err != nil {
		t.Errorf("one bound in the program: the ascription is its set: %v", err)
	}
	// And the sign: once the program enforces (−2^101, 2^101), h's non-negative
	// declaration is not enforced as non-negative, so it proves nothing about
	// g's [0, 2^101).
	signed := strings.Replace(one, "export fn calc(n ∈ 0..1) (0..2^100)",
		"export fn calc(n ∈ 0..1) (-2^100..2^100)", 1)
	if signed == one {
		t.Fatal("the test did not sign calc's result")
	}
	if _, err := dischargeGo(t, signed); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("a signed program's ascription does not prove a non-negative set; got %v", err)
	}
}
