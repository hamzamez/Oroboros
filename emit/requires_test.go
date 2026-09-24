package emit

import (
	"strings"
	"testing"

	"oroboros/core"
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
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	reqs := InstallRequires(env, prog)
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return DischargeRequires(reqs, tg, q, prog.Sigs[q], nf)
}

// A RANGE, AT A CALL: an argument the caller cannot keep inside the declared
// range is refused, one it can is not, and a literal is decided as the call
// reduces. `digits18` given a value past 10^18 printed its low 18 digits.
func TestADeclaredRangeIsAnObligationAtTheCall(t *testing.T) {
	const g = `(sig g ((x (int 0 9))) int)
(def g (x) (+ x 1))
`
	if _, err := dischargeGo(t, g+`(export f)
(sig f ((n (int 0 1000))) int)
(def f (n) (g n))`); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("n ∈ [0, 1000] is not inside g's [0, 9]; got %v", err)
	}
	if _, err := dischargeGo(t, g+`(export f)
(sig f ((n (int 0 9))) int)
(def f (n) (g n))`); err != nil {
		t.Errorf("n ∈ [0, 9] keeps g's contract: %v", err)
	}
	if _, err := dischargeGo(t, g+`(export f)
(sig f ((n int)) int)
(def f (n) (g 12))`); err == nil || !strings.Contains(err.Error(), "a call passes 12") {
		t.Errorf("the literal 12 is outside [0, 9]; got %v", err)
	}
}

// EACH LAYER PROVES WHAT IT CAN. `Rem64(h, l, k)` is ≥ 0 by its declared result,
// which the interval analysis reads, and < k by its `ensures`, which only the
// refinement layer reads: neither proves `[0, k − 1]` alone.
func TestIntervalsAndEnsuresShareARange(t *testing.T) {
	_, err := dischargeGo(t, `(use go/math/bits as b)
(sig d ((g (int 0 999999999999999999))) int)
(def d (g) (+ (% g 10) 1))
(export f)
(sig f ((h (int 0 18446744073709551615)) (l (int 0 18446744073709551615))) int)
(def f (h l) (d (b.Rem64 h l 1000000000000000000)))`)
	if err != nil {
		t.Errorf("Rem64's result is in [0, 10^18 − 1] by its range and its ensures together: %v", err)
	}
	// With no primitive around the argument — the definition returns it — the
	// walk collects nothing on its way down, and the mark must collect the
	// guarantee itself.
	_, err = dischargeGo(t, `(use go/math/bits as b)
(sig d ((g (int 0 999999999999999999))) int)
(def d (g) g)
(export f)
(sig f ((h (int 0 18446744073709551615)) (l (int 0 18446744073709551615))) int)
(def f (h l) (d (b.Rem64 h l 1000000000000000000)))`)
	if err != nil {
		t.Errorf("the mark collects Rem64's ensures when nothing above it does: %v", err)
	}
}

// ABOVE THE WORD, A RANGE IS ITS BIT LENGTH. `fact`'s declared result is
// enforced as "at most 201 bits", and the argument it feeds declares the same
// type: the two are one set only when both are read at bit length. A narrower
// argument range is not implied, and is refused.
func TestARangeAboveTheWordIsItsBitLength(t *testing.T) {
	const src = `(sig fact ((n (int 0 40))) (int 0 (pow 2 200)))
(def fact (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1))))
(sig render ((x (int 0 RANGE))) int)
(def render (x) (if (= x 0) 0 1))
(export f)
(sig f ((n (int 0 15))) int)
(def f (n) (render (fact (+ 25 n))))`
	if _, err := dischargeGo(t, strings.Replace(src, "RANGE", "(pow 2 200)", 1)); err != nil {
		t.Errorf("the same type as argument and as result is one set: %v", err)
	}
	if _, err := dischargeGo(t, strings.Replace(src, "RANGE", "(pow 2 100)", 1)); err == nil {
		t.Error("a 201-bit result does not fit a 101-bit parameter")
	}
	// Where bit length and exactness DISAGREE: a result declared up to 2^201 − 1
	// into a parameter declared up to 2^200. Exactly, [0, 2^201 − 1] ⊄ [0, 2^200];
	// by enforcement both are "at most 201 bits", one set.
	wider := strings.Replace(strings.Replace(src, "RANGE", "(pow 2 200)", 1),
		"(int 0 (pow 2 200)))\n(def fact", "(int 0 (- (pow 2 201) 1)))\n(def fact", 1)
	if !strings.Contains(wider, "(- (pow 2 201) 1)") {
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
	const h = `(sig h ((a int) (b int)) int (where (< a b)))
(def h (a b) (- b a))
`
	if _, err := dischargeGo(t, h+`(export f)
(sig f ((n (int 0 10))) int)
(def f (n) (h n 20))`); err != nil {
		t.Errorf("n ≤ 10 < 20: %v", err)
	}
	if _, err := dischargeGo(t, h+`(export f)
(sig f ((n (int 0 30))) int)
(def f (n) (h n 20))`); err == nil || !strings.Contains(err.Error(), "h requires") {
		t.Errorf("n may reach 30, past 20; got %v", err)
	}
	if _, err := dischargeGo(t, h+`(export f)
(sig f ((n int)) int)
(def f (n) (h 5 3))`); err == nil {
		t.Error("(< 5 3) is false at the call and must be refused")
	}
	// A PURE ARGUMENT GOES INTO THE CONDITION AS ITSELF. The table is used twice,
	// so β binds it to a name whose length nothing knows; read as the literal, its
	// length folds to 3 (examples/json/tokenize.oro's shape).
	if _, err := dischargeGo(t, `(sig s2 ((s (array int))) int (where (< (len s) 10)))
(def s2 (s) (+ (s 0) (s 1)))
(export f)
(sig f ((n int)) int)
(def f (n) (s2 (array 1 2 3)))`); err != nil {
		t.Errorf("a three-element literal is shorter than 10: %v", err)
	}
}

// AN INFINITE ENDPOINT'S FINITE SIDE IS STILL AN OBLIGATION: `(int 0 +inf)` is
// `0 <= n`, carried by the call's own condition.
func TestAnInfiniteRangesFiniteSideIsChecked(t *testing.T) {
	if _, err := dischargeGo(t, `(sig k ((n (int 0 +inf))) int)
(def k (n) (+ n 1))
(export f)
(sig f ((m (int -5 5))) int)
(def f (m) (k m))`); err == nil {
		t.Error("m may be −5, below k's 0")
	}
}

// THE MARKS CHANGE NOTHING THEY DO NOT CHECK. A where-mark sits on a call's
// result, so a result that is a tuple — applied to its eliminator — or a literal
// a fold must see is hoisted past; once discharged and stripped, the residual is
// the one reduction gives with no contracts installed.
func TestTheMarksLeaveTheResidualUnchanged(t *testing.T) {
	const src = `(sig dm ((a int) (b int)) (tuple int int) (where (< 0 b)))
(def dm (a b) (tuple (/ a b) (% a b)))
(sig two ((x (int 0 9))) int (where (<= 0 x)))
(def two (x) 2)
(export f)
(sig f ((a (int 0 100)) (b (int 1 100))) int)
(def f (a b) (+ ((dm a b) (fn (q r) (+ q r))) (* (two 3) (two (% a 10)))))`
	tg := goNative(t)
	forms, _ := core.Read(src)
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := tg.Env(prog)
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
