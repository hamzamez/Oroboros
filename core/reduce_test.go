package core

import (
	"strings"
	"testing"
)

// Every case here is a worked example from the specification. If the spec and
// the implementation disagree, one of them is wrong and the test says which
// pair to look at.
//
//	core-0 §5.1  β
//	core-0 §5.2  δ
//	core-0 §5.3  the same term, two normal forms
//	core-0 §5.4  a residual λ
//	q5   §3      fusion by δ+β, the delayed vector representation
//	q5b  §3      filter by δ+β, the push representation

func norm(t *testing.T, src, target string) string {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("expected exactly one term to reduce, got %d", len(terms))
	}
	env, err := prog.Env(target)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return out.String()
}

func check(t *testing.T, src, target, want string) {
	t.Helper()
	got := norm(t, src, target)
	if got != want {
		t.Errorf("target %s\n got: %s\nwant: %s", target, got, want)
	}
}

// ---------------------------------------------------------------- core-0 §5.1

func TestBeta(t *testing.T) {
	check(t, `
		(prim add)
		((fn (x) (add x 1)) 4)
	`, "default", "(add 4 1)")
}

// ---------------------------------------------------------------- core-0 §5.2

func TestDelta(t *testing.T) {
	check(t, `
		(prim mul)
		(def double (fn (x) (mul x 2)))
		(def quad   (fn (x) (double (double x))))
		(quad 3)
	`, "default", "(mul (mul 3 2) 2)")
}

// ---------------------------------------------------------------- core-0 §5.3
//
// The thesis, executable: one word of target declaration separates a BLAS call
// from a loop.

const dotProgram = `
	(target go   (prim add mul alen aindex fold-range))
	(target blas (prim add mul alen aindex fold-range dot))

	(def vec     (fn (n f) (fn (sel) (sel n f))))
	(def vlen    (fn (v)   (v (fn (n f) n))))
	(def vindex  (fn (v i) ((v (fn (n f) f)) i)))
	(def of-array (fn (a) (vec (alen a) (fn (i) (aindex a i)))))

	(def zip (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))
	(def sum (fn (v) (fold-range 0.0 (vlen v) (fn (acc i) (add acc (vindex v i))))))
	(def dot (fn (a b) (sum (zip mul (of-array a) (of-array b)))))

	(dot p q)
`

func TestSameTermTwoNormalForms(t *testing.T) {
	// On blas, dot is primitive: reduction halts immediately, zero steps.
	check(t, dotProgram, "blas", "(dot p q)")

	// On go, dot is defined, so it reduces all the way to g1's residual.
	check(t, dotProgram, "go",
		"(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))")
}

// ---------------------------------------------------------------- core-0 §5.4

func TestResidualLambda(t *testing.T) {
	check(t, `
		(prim mul)
		(def make-scaler (fn (f) (fn (v) (mul v f))))
		(make-scaler 3)
	`, "default", "(fn (v) (mul v 3))")
}

// ---------------------------------------------------------------- q5b §3
//
// filter, via the push representation: a collection is its own fold.

func TestFilterFusesToOneLoop(t *testing.T) {
	check(t, `
		(target go (prim add alen aindex fold-range if pos))

		(def push-of-array (fn (a)   (fn (step z) (fold-range z (alen a) (fn (acc i) (step acc (aindex a i)))))))
		(def push-filter   (fn (p c) (fn (step z) (c (fn (acc x) (if (p x) (step acc x) acc)) z))))
		(def push-sum      (fn (c)   (c (fn (acc x) (add acc x)) 0.0)))

		(push-sum (push-filter pos (push-of-array a)))
	`, "go",
		"(fold-range 0.0 (alen a) (fn (acc i) (if (pos (aindex a i)) (add acc (aindex a i)) acc)))")
}

// ---------------------------------------------------------------- termination

func TestRecursiveDefinitionIsNotUnfolded(t *testing.T) {
	// core-0 §6: δ on a recursive definition does not terminate, so it is never
	// applied. `count` must survive into the residual as a target function.
	got := norm(t, `
		(target go (prim add lt if))
		(def count (fn (n) (if (lt n 1) 0 (add 1 (count (add n -1))))))
		(count 5)
	`, "go")
	if !strings.Contains(got, "count") {
		t.Errorf("recursive definition was unfolded; got %s", got)
	}
}

func TestMutualRecursionIsNotUnfolded(t *testing.T) {
	got := norm(t, `
		(target go (prim if lt add))
		(def even? (fn (n) (if (lt n 1) 1 (odd? (add n -1)))))
		(def odd?  (fn (n) (if (lt n 1) 0 (even? (add n -1)))))
		(even? 4)
	`, "go")
	if !strings.Contains(got, "even?") && !strings.Contains(got, "odd?") {
		t.Errorf("mutual recursion was unfolded; got %s", got)
	}
}

func TestFuelStopsSelfApplication(t *testing.T) {
	forms, _ := Read(`(prim x) ((fn (f) (f f)) (fn (f) (f f)))`)
	prog, terms, _ := Load(forms)
	env, _ := prog.Env("default")
	if _, err := Normalize(terms[0], env, 10_000); err == nil {
		t.Fatal("expected the step limit to stop self-application")
	} else if _, ok := err.(*FuelError); !ok {
		t.Fatalf("expected FuelError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------- capture

func TestSubstitutionAvoidsCapture(t *testing.T) {
	// (fn (x) ((fn (y) (fn (x) (f y x))) x))  applied to a
	// The inner binder x must be freshened, or the outer x is captured.
	check(t, `
		(prim f a)
		((fn (x) ((fn (y) (fn (x) (f y x))) x)) a)
	`, "default", "(fn (x') (f a x'))")
}

// ---------------------------------------------------------------- normal form

func TestResidualNamesReported(t *testing.T) {
	forms, _ := Read(`
		(target go (prim add))
		(def use-missing (fn (x) (add x (nowhere x))))
		(use-missing 1)
	`)
	prog, terms, _ := Load(forms)
	env, _ := prog.Env("go")
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	got := Residual(out, env)
	if len(got) != 1 || got[0] != "nowhere" {
		t.Errorf("expected residual [nowhere], got %v (term %s)", got, out)
	}
}

// ---------------------------------------------------------------- reader

func TestReaderRejectsBidiControl(t *testing.T) {
	if _, err := Read("(add 1 ‮2)"); err == nil {
		t.Fatal("expected a bidirectional control to be rejected")
	}
}

func TestReaderAcceptsUTF8Identifiers(t *testing.T) {
	check(t, `
		(prim جمع)
		(def مجموع (fn (س ص) (جمع س ص)))
		(مجموع 1 2)
	`, "default", "(جمع 1 2)")
}

// UAX #31 admits letters, not mathematical symbols, so `＋` and `≤` are not
// identifiers. That is the standard's answer and it may not be the one we want
// — see docs/spec/concerns.md, open question on the identifier profile.
func TestReaderRejectsMathSymbolAsIdentifier(t *testing.T) {
	if _, err := ReadTerm("＋"); err == nil {
		t.Fatal("expected U+FF0B to be rejected as an identifier")
	}
}

// Case must never be semantically significant (core-0 §1.1), so that the
// language is writable in scripts that have no case at all.
func TestCaseIsNotSignificant(t *testing.T) {
	check(t, `
		(prim add)
		(def Double (fn (X) (add X X)))
		(def double (fn (x) (add x 1)))
		(Double 5)
	`, "default", "(add 5 5)")
}

func TestFloatsRoundTrip(t *testing.T) {
	// ADR 0009: a value must not change by being written down.
	for _, src := range []string{"1.0", "0.1", "0.30000000000000004", "1e+21", "-0.0"} {
		tm, err := ReadTerm(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		back, err := ReadTerm(tm.String())
		if err != nil {
			t.Fatalf("reread %s -> %s: %v", src, tm, err)
		}
		if back.Float != tm.Float {
			t.Errorf("%s did not round-trip: %v vs %v", src, tm.Float, back.Float)
		}
	}
}

func TestLambdaSpelledEitherWay(t *testing.T) {
	a := norm(t, "(prim add)\n((fn (x) (add x 1)) 2)", "default")
	b := norm(t, "(prim add)\n((λ (x) (add x 1)) 2)", "default")
	if a != b {
		t.Errorf("fn and λ disagree: %s vs %s", a, b)
	}
}

// ---------------------------------------------------------------- fix

// (def f (f)) denotes ⊥. δ must not unfold it, it survives as a target function
// that calls itself, and that is the correct compilation — so the residual check
// must not report it as a failure. See docs/spec/pcf.md §4.
func TestSelfCallIsBottomNotAnError(t *testing.T) {
	forms, err := Read(`
		(target go (prim))
		(def f (f))
		(f)
	`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, _ := Load(forms)
	env, _ := prog.Env("go")
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got := out.String(); got != "(f)" {
		t.Errorf("expected (f) to survive unreduced, got %s", got)
	}
	if left := Residual(out, env); len(left) != 0 {
		t.Errorf("a recursive definition is a legitimate survivor, not a residual failure; got %v", left)
	}
}
