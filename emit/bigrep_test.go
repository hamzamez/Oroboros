package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// ARBITRARY PRECISION — ADR 0019's THIRD ESCAPE (emit/bigrep.go).
//
// Each test here pins one of the three promotion rules, or one of the two
// hazards the build found. The rules are worth pinning separately because they
// disagree on purpose: rule (P) promotes `power`'s `x` and deliberately does
// NOT promote `fact`'s `i`, and a version of the pass that got either of those
// backwards still compiles and still produces the right answer — one slowly,
// one wrongly.

func promoteOn(t *testing.T, dir, src, name string) string {
	t.Helper()
	tg, err := LoadTarget(dir)
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
	nf, err := core.Normalize(prog.Defs[name], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := PromoteBig(tg, prog.Sigs[name], nf)
	return out.String()
}

const factSrc = `(export fact)
(sig fact ((n (int 0 30))) (int 0 (pow 2 110)))
(def fact (fn (n)
  (loop ((acc 1) (i 1))
    (> i n)  acc
    else     (again (* acc i) (+ i 1)))))
`

// RULE (D), AND THE REASON THE SOLVER IS BIDIRECTIONAL.
//
// Every input to `fact` is small — `n` is at most 30 — so nothing flowing
// FORWARD makes the accumulator a bignum. The pressure comes from the declared
// RESULT, backwards through the loop's exit. precision-by-declaration.md named
// this program for exactly that reason and said a forward-only solver "would
// pass the entire current corpus and fail on the first factorial".
func TestADeclaredResultPromotesTheAccumulatorBackwards(t *testing.T) {
	got := promoteOn(t, "../targets/go", factSrc, "fact")
	// Either spelling: the multiply may since have been given a DESTINATION by
	// emit/bigreuse.go, which is a question about allocation rather than about
	// representation. This test is about the latter.
	if !strings.Contains(got, "(big* acc") && !strings.Contains(got, "(big*! acc acc") {
		t.Errorf("the accumulator was not promoted, so the declared result "+
			"bought nothing:\n%s", got)
	}
}

// AND RULE (P)'s NEGATIVE HALF, WHICH IS THE EASY ONE TO GET WRONG.
//
// `i` is READ BY the big multiply, so a solver that promoted everything a big
// operation touches would give this loop a bignum counter, a bignum compare and
// a bignum increment. The gate is provability: `i` is provably 1..31, so it
// stays a machine word and is widened at the multiply — one conversion per
// iteration instead of three bignum operations.
//
// This is the test that fails if rule (P) loses its `!cur[k].fits()` guard, and
// nothing else here does: the program still compiles and still prints the right
// answer, only slower. A wrong answer would have been easier to catch.
func TestAProvablyBoundedCounterStaysAMachineWord(t *testing.T) {
	got := promoteOn(t, "../targets/go", factSrc, "fact")
	if strings.Contains(got, "(big+ i") || strings.Contains(got, "(big> i") ||
		strings.Contains(got, "(big+! i") {
		t.Errorf("the loop counter was promoted; it is provably 1..31 and a "+
			"bignum counter is pure loss:\n%s", got)
	}
	if !strings.Contains(got, "(+ i 1)") {
		t.Errorf("the counter's increment is not ordinary integer arithmetic:\n%s", got)
	}
	if !strings.Contains(got, "(big-of i)") {
		t.Errorf("the counter is not widened at the multiply, so a machine word "+
			"is being handed to a bignum operation:\n%s", got)
	}
}

// AND RULE (P)'s POSITIVE HALF, on the program that needs it.
//
// `power`'s `x` is never returned and is never assigned a big value, so neither
// (S) nor (D) reaches it. It is read by the big multiply and its own update
// `(* x x)` is unbounded, which is what promotes it. Leaving it a machine word
// would silently give the wrong answer while `acc` was scrupulously exact —
// this is the one rule here whose absence is a WRONG ANSWER rather than a slow
// one.
func TestAnUnboundedValueFeedingABigOperationIsPromoted(t *testing.T) {
	src := `(export power)
(sig power ((b (int 0 1000)) (e (int 0 64))) (int 0 (pow 2 640)))
(def power (fn (b e)
  (loop ((acc 1) (x b) (k e))
    (= k 0)        acc
    (= (% k 2) 1)  (again (* acc x) (* x x) (/ k 2))
    else           (again acc (* x x) (/ k 2)))))
`
	got := promoteOn(t, "../targets/go", src, "power")
	if !strings.Contains(got, "(big* x x)") {
		t.Errorf("`x` was not promoted: it feeds a bignum and its own square "+
			"leaves the window, so a machine word here is a wrong answer:\n%s", got)
	}
	if strings.Contains(got, "(big/ k") || strings.Contains(got, "(big= k") {
		t.Errorf("`k` was promoted; it halves and is provably 0..64:\n%s", got)
	}
}

// AN `again` IS A JUMP, NOT A VALUE (ADR 0015), and the representation join at
// an `if` has to know it.
//
// The first version widened whichever arm of a conditional disagreed with the
// other, and a loop's clause chain is a conditional whose one arm returns the
// accumulator and whose other arm iterates — so it emitted `big-of(goto)`. The
// same mistake in the demand direction promoted every bignum loop's counter,
// because `(+ i 1)` is an arithmetic operation sitting in a position that
// demands a bignum, and there is nothing locally wrong with that reading.
func TestABackEdgeIsNeitherWidenedNorDemanded(t *testing.T) {
	got := promoteOn(t, "../targets/go", factSrc, "fact")
	if strings.Contains(got, "(big-of (again") {
		t.Errorf("a jump was widened; `again` has no value to convert:\n%s", got)
	}
}

// AND A LOOP'S INITIALISER IS WIDENED, which is not tidiness: every backend
// reads a loop variable's TYPE off its initialiser, so without this Go declares
// `acc := 1` and then assigns a `*big.Int` to it.
func TestAPromotedLoopVariableIsWidenedAtItsInitialiser(t *testing.T) {
	got := promoteOn(t, "../targets/go", factSrc, "fact")
	if !strings.Contains(got, "(big-of 1)") {
		t.Errorf("the accumulator's initial value was not widened:\n%s", got)
	}
}

// A TARGET MAY DECLINE ARBITRARY PRECISION, and it is the capability model
// answering rather than a hole — the same answer JavaScript already gives for
// the CHECKED primitive it does not declare.
//
// The refusal has to NAME the target, because "int 0 <302 digits> is required
// here" is true and explains nothing about whose limitation it is.
func TestATargetWithoutABignumRefusesAndSaysSo(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	if tg.HasBig() {
		t.Skip("windows now declares a bignum; this test has done its job")
	}
	forms, _ := core.Read(factSrc)
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckSignatures(tg, prog, env)
	if err == nil {
		t.Fatal("windows accepted a program needing arbitrary precision")
	}
	if !strings.Contains(err.Error(), "declares none") ||
		!strings.Contains(err.Error(), "windows") {
		t.Errorf("the refusal does not name the target or its missing capability: %v", err)
	}
	// AND IT DOES NOT PRINT THREE HUNDRED DIGITS, because a diagnostic nobody
	// reads is a diagnostic that does not work.
	if strings.Contains(err.Error(), strings.Repeat("0", 40)) {
		t.Errorf("the range was printed in full: %v", err)
	}
}

// A BIGNUM MAY NOT SILENTLY BECOME AN `int`, which is unbounded-rung.md §3: the
// promotion is a WIDENING and not a refinement, so the narrowing direction is
// refused and that refusal is the surface where a programmer finds out a value
// left the machine word.
//
// `int ⊆ big` is the decidable half of subtyping type-algebra.md already keeps,
// run in the other direction.
func TestABignumIsRefusedWhereAWordIsRequired(t *testing.T) {
	src := `(use go)
(export narrow)
(sig narrow ((n (int 0 (pow 2 70)))) int)
(def narrow (fn (n) (+ n 1)))
`
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, _ := core.Read(src)
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckSignatures(tg, prog, env)
	if err == nil {
		t.Fatal("a value above the portable window was accepted as an `int`")
	}
	if !strings.Contains(err.Error(), "WIDENING") {
		t.Errorf("the refusal does not say what kind of mismatch this is: %v", err)
	}
}

// THE THREE TARGETS THAT HAVE A BIGNUM AGREE ON WHAT IT MEANS, and the two
// places a host would naturally disagree are the two the declarations choose
// against explicitly.
//
// DIVISION AND REMAINDER. integers.md §4 measured all four hosts agreeing
// inside the window: division truncates toward zero and the remainder takes the
// DIVIDEND's sign. Go's `big.Int` offers both conventions and Java's
// `BigInteger` does too, and the Euclidean one — Go's `Div`/`Mod`, Java's `mod`
// — is the WRONG one here: it would make the same source disagree with itself
// either side of 2^53, which is the one thing a representation change may never
// do (ADR 0009's rule, at a different boundary).
func TestBignumDivisionKeepsTheLanguagesConvention(t *testing.T) {
	for _, c := range []struct{ dir, badQuo, badRem string }{
		{"../targets/go", "Div(", "Mod("},
		{"../targets/java", ".mod(", ".mod("},
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if f := tg.Prims["big/"].Form; strings.Contains(f, c.badQuo) {
			t.Errorf("%s: big division is Euclidean (%s); the language truncates "+
				"toward zero", c.dir, f)
		}
		if f := tg.Prims["big%"].Form; strings.Contains(f, c.badRem) {
			t.Errorf("%s: big remainder is Euclidean (%s); the language takes the "+
				"dividend's sign", c.dir, f)
		}
	}
}

// AND JAVASCRIPT'S `BigInt` PRINTS WITH AN `n`. `String(1n)` is "1" but
// `${1n}` and a bare console.log give "1n", so `big-str` has to be `String`,
// and that is a real divergence from the other two rather than a style choice.
func TestJavaScriptBigStringHasNoSuffix(t *testing.T) {
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	if f := tg.Prims["big-str"].Form; !strings.Contains(f, "String(") {
		t.Errorf("js big-str is %q; a BigInt rendered any other way carries an "+
			"`n` suffix the other two targets do not print", f)
	}
	// `===` and not `==`: `1n == 1` is true and `1n === 1` is false, and
	// equality across the two representations is the confusion the separate
	// declaration exists to prevent.
	if f := tg.Prims["big="].Form; !strings.Contains(f, "===") {
		t.Errorf("js big= is %q, which compares across representations", f)
	}
}

// PROMOTION IS OPT-IN, and this is ADR 0019's blast-radius claim checked rather
// than asserted: every source of `big` is a declaration somebody wrote, so a
// program that declares nothing gets byte-identical output.
func TestAProgramThatDeclaresNothingIsUntouched(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (loop ((acc 0) (i 0)) (>= i n) acc else (again (+ acc i) (+ i 1)))))
`
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, _ := core.Read(src)
	prog, _, _ := core.Load(forms)
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, n := PromoteBig(tg, prog.Sigs["f"], nf)
	if n != 0 || out.String() != nf.String() {
		t.Errorf("a program with no declaration above the window was rewritten "+
			"(%d ops):\n%s", n, out)
	}
}
