package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/types.md. The checker runs on the residual, before emission, and
// exists because the same ill-typed program is rejected by Go and Java and
// silently prints `hello1` on JavaScript.
//
// The programs use the language's own operators and tables where they can, and
// a target's names only where the language has none (floats). Until
// portable-2026-09-29 they were written against the portable layer.

func checkSrc(t *testing.T, target, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target)
	if err != nil {
		t.Fatal(err)
	}
	return Check(tg, "test", reduce(t, src, target))
}

// §1 — the measured bug, on every target including the one with no types at
// run time: `+` is the language's, so JavaScript's `"hello" + 1` is the same
// program as Go's refusal.
func TestCheckerRejectsTheMeasuredBug(t *testing.T) {
	for _, target := range []string{"go", "js", "java", "windows"} {
		err := checkSrc(t, target, `(fn (x) (+ "hello" 1))`)
		if err == nil {
			t.Errorf("%s: expected a type error", target)
			continue
		}
		if !strings.Contains(err.Error(), "string") || !strings.Contains(err.Error(), "int") {
			t.Errorf("%s: error should name both types, got %v", target, err)
		}
	}
}

// §3 — a parameter demanded at two different types is the conflict case, and
// it is what makes this checking rather than inference.
func TestCheckerRejectsConflictingUse(t *testing.T) {
	err := checkSrc(t, "go", `
		(use go)
		(fn (a) (go.f+ (go.at-float64 a 0) (go.float64-of-int (go.len (go.at-string a 0)))))
	`)
	if err == nil {
		t.Fatal("a used as both slice-float64 and slice-string should conflict")
	}
	if !strings.Contains(err.Error(), "slice-float64") || !strings.Contains(err.Error(), "slice-string") {
		t.Errorf("the conflict should name both types, got %v", err)
	}
}

// §3 — `any` demands nothing, so it neither conflicts nor binds. Println takes
// `any` precisely because the host is polymorphic there.
func TestCheckerAcceptsAny(t *testing.T) {
	if err := checkSrc(t, "go", `
		(use go/fmt)
		(fn (label) (fmt.Println label))
	`); err != nil {
		t.Errorf("any must not constrain: %v", err)
	}
}

// §4 — the branches of a conditional must agree.
func TestCheckerRejectsMismatchedBranches(t *testing.T) {
	err := checkSrc(t, "go", `
		(use go)
		(fn (a) (if (go.f< (go.at-float64 a 0) 1.0) 2.0 "no"))
	`)
	if err == nil {
		t.Fatal("branches f64 and string should conflict")
	}
	if !strings.Contains(err.Error(), "f64") || !strings.Contains(err.Error(), "string") {
		t.Errorf("the conflict should name both types, got %v", err)
	}
}

// §6 — the acceptance test that matters more than the negative one: nothing
// currently correct may be rejected. The structural forms are where a wrong
// rule would show up first: a loop with one and with two accumulators, a
// table, a map threaded through a loop, and a conditional.
func TestCheckerAcceptsTheGauntletShapes(t *testing.T) {
	cases := map[string]string{
		"fold": `(use go)
			(fn (a) (loop ((acc 0.0) (i 0))
			  (>= i (go.len a))  acc
			  else               (again (go.f+ acc (go.at-float64 a i)) (+ i 1))))`,
		"map accumulator": `(use go)
			(fn (ws) (loop ((m (go.make-map)) (i 0))
			  (>= i (go.len ws))  m
			  else                (again (go.inc-map m (go.at-string ws i)) (+ i 1))))`,
		"table": `(use go)
			(fn (a) (table (- (go.len a) 1) (fn (i) (go.f* (go.at-float64 a i) 2.0))))`,
		"two accumulators": `(use go)
			(fn (xs ys) (loop ((ax 0.0) (ay 0.0) (i 0))
			  (>= i (go.len xs))  (go.f+ ax ay)
			  else                (again (go.f+ ax (go.at-float64 xs i))
			                             (go.f+ ay (go.at-float64 ys i))
			                             (+ i 1))))`,
		"conditional": `(use go)
			(fn (a) (if (go.f> (go.at-float64 a 0) 0.0) 1.0 2.0))`,
	}
	for name, src := range cases {
		if err := checkSrc(t, "go", src); err != nil {
			t.Errorf("%s should typecheck: %v", name, err)
		}
	}
}

// §5 → built. A signature is a claim checked against BOTH the definition and
// any target's native implementation. The second is the job no host compiler
// can do, since the two live on different targets.

func loadWithSigs(t *testing.T, target, src string) (*Target, *core.Program, *core.Env) {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target)
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
	return tg, prog, env
}

func TestSignatureCheckedAgainstTargetNative(t *testing.T) {
	// blas provides `dot` natively, taking two vectors.
	src := `
		(sig dot ((a vec-f64) (b int)) f64)
		(def dot (fn (a b) (dot a b)))
	`
	tg, prog, env := loadWithSigs(t, "blas", src)
	err := CheckSignatures(tg, prog, env, nil)
	if err == nil {
		t.Fatal("a signature disagreeing with the target's native declaration must be caught")
	}
	if !strings.Contains(err.Error(), "blas") {
		t.Errorf("the error should name the target, got %v", err)
	}
}

func TestSignatureAcceptedWhenItAgrees(t *testing.T) {
	src := `
		(sig dot ((a vec-f64) (b vec-f64)) f64)
		(def dot (fn (a b) (dot a b)))
	`
	tg, prog, env := loadWithSigs(t, "blas", src)
	if err := CheckSignatures(tg, prog, env, nil); err != nil {
		t.Errorf("an agreeing signature must pass: %v", err)
	}
}

func TestSignatureCheckedAgainstDefinition(t *testing.T) {
	// go does not provide `bad`, so the claim is about the definition — which
	// here returns a slice rather than the declared f64.
	src := `
		(use go)
		(sig bad ((a slice-float64)) f64)
		(def bad (a) (go.make-float64 (go.len a)))
	`
	tg, prog, env := loadWithSigs(t, "go", src)
	err := CheckSignatures(tg, prog, env, nil)
	if err == nil {
		t.Fatal("a definition disagreeing with its own signature must be caught")
	}
	if !strings.Contains(err.Error(), "slice-float64") {
		t.Errorf("the error should name the actual type, got %v", err)
	}
}

func TestSignatureArityIsChecked(t *testing.T) {
	src := `
		(sig two ((a f64) (b f64)) f64)
		(def two (a) a)
	`
	tg, prog, env := loadWithSigs(t, "go", src)
	err := CheckSignatures(tg, prog, env, nil)
	if err == nil {
		t.Fatal("an arity mismatch must be caught")
	}
	if !strings.Contains(err.Error(), "signature declares 2") {
		t.Errorf("the refusal should be the arity's, got %v", err)
	}
}
