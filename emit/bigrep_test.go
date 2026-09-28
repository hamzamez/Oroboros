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

const factSrc = `(export fact)
(sig fact ((n (int 0 30))) (int 0 +inf))
(def fact (fn (n)
  (loop ((acc 1) (i 1))
    (> i n)  acc
    else     (again (* acc i) (+ i 1)))))
`

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
	err = CheckSignatures(tg, prog, env, nil)
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

// A RANGE ABOVE THE WINDOW IS A REPRESENTATION AND NOT A CONTRACT, and the
// asymmetry with a parameter's range is soundness rather than convenience.
//
// A `where` is ASSUMED, so half of it is a weaker assumption and safe. An
// `ensures` on an exported definition is CHECKED AGAINST THE BODY — and the
// interval domain does not model a bignum, reporting [-inf, +inf] for one by
// construction. So synthesising `(<= 0 result)` from `(int 0 +inf)`
// demands of the checker a fact it can never establish, and EVERY
// arbitrary-precision program is refused for a claim nothing can discharge.
//
// That is a refusal standing in front of nothing, which this repository has
// caught once already (scalarrange-2026-08-31, where removing one found a
// silent wrong answer behind it). Here there is nothing behind it: the value is
// exact by construction, and the claim was never checkable.
func TestABigResultRangeSynthesisesNoGuarantee(t *testing.T) {
	for _, result := range []string{"(int 0 +inf)", "(int 0 +inf)"} {
		src := `(export fib)
(sig fib ((n (int 0 1000))) ` + result + `)
(def fib (fn (n) (loop ((a 0) (b 1) (i 0)) (>= i n) a else (again b (+ a b) (+ i 1)))))
`
		tg, err := LoadTarget("../targets/go")
		if err != nil {
			t.Fatal(err)
		}
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := firstSig(forms).Ensures; got != nil {
			t.Errorf("%s synthesised the guarantee %s, which the interval domain "+
				"can never establish for a bignum", result, got)
		}
		prog, _, err := core.Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, err := tg.Env(prog)
		if err != nil {
			t.Fatal(err)
		}
		// And the program must still get through the checks, which is the half a
		// test on the signature alone would not catch.
		if err := CheckSignatures(tg, prog, env, nil); err != nil {
			t.Errorf("%s: %v", result, err)
			continue
		}
		// That the program is ACCEPTED by the postcondition check is
		// TestABigResultRangeIsNotAnObligation (ensures_ir_test.go).
	}
	// THE PARAMETER SIDE STILL SAYS ITS HALF, which is what `+inf` is for: a
	// non-negative bignum needs no sign handling, and bigarith measured sign
	// handling as a real cost.
	forms, err := core.Read(`(sig f ((n (int 0 +inf))) (int 0 +inf))
(def f (fn (n) n))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := firstSig(forms).Where; got == nil || got.String() != "(<= 0 n)" {
		t.Errorf("a non-negative unbounded parameter carries %v, want (<= 0 n)", got)
	}
}

// firstSig finds the signature among a file's forms, which is not forms[0]
// unless the file begins with one.
func firstSig(forms []core.Form) *core.Sig {
	for _, f := range forms {
		if f.Sig != nil {
			return f.Sig
		}
	}
	return &core.Sig{}
}

// A PROGRAM THE LIMB LIBRARY CANNOT SERVE takes the host's bignum, and its
// signature is checked as that rung means it. Checked as limbs, the claim was
// refused with "big-fit is big, but array int is required here", a message
// naming two internal representations, and the fallback never ran.
func TestTheLimbFallBackIsCheckedOnTheHostsRung(t *testing.T) {
	src := `(export run)
(sig run ((a (int 0 (pow 2 200))) (b (int 1 (pow 2 200)))) (int 0 (pow 2 200)))
(def run (a b) (/ (* a 7) b))
`
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.BigRepr = "limbs"
	forms, _ := core.Read(src)
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSignatures(tg, prog, env, nil); err != nil {
		t.Errorf("a bignum division on the limb rung: %v", err)
	}
}
