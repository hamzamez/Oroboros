package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/types.md. The checker runs on the residual, before emission, and
// exists because the same ill-typed program is rejected by Go and Java and
// silently prints `hello1` on JavaScript.

func checkSrc(t *testing.T, target, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target + ".oro")
	if err != nil {
		t.Fatal(err)
	}
	return Check(tg, "test", reduce(t, src, target))
}

// §1 — the measured bug, on every target including the one with no types.
func TestCheckerRejectsTheMeasuredBug(t *testing.T) {
	src := `
		(use num/f64)
		(fn (x) (f64.add "hello" 1.0))
	`
	for _, target := range []string{"portable-go", "js", "java"} {
		err := checkSrc(t, target, src)
		if err == nil {
			t.Errorf("%s: expected a type error", target)
			continue
		}
		if !strings.Contains(err.Error(), "string") || !strings.Contains(err.Error(), "f64") {
			t.Errorf("%s: error should name both types, got %v", target, err)
		}
	}
}

// §3 — a parameter demanded at two different types is the conflict case, and
// it is what makes this checking rather than inference.
func TestCheckerRejectsConflictingUse(t *testing.T) {
	err := checkSrc(t, "portable-go", `
		(use num/f64)
		(fn (a) (f64.add (aindex a 0) (slen a)))
	`)
	if err == nil {
		t.Fatal("a used as both vec-f64 and vec-string should conflict")
	}
}

// §3 — `any` demands nothing, so it neither conflicts nor binds. print-line
// takes `any` precisely because the host is polymorphic there.
func TestCheckerAcceptsAny(t *testing.T) {
	if err := checkSrc(t, "portable-go", `
		(use io)
		(fn (label) (io.print-line label))
	`); err != nil {
		t.Errorf("any must not constrain: %v", err)
	}
}

// §4 — the branches of a conditional must agree.
func TestCheckerRejectsMismatchedBranches(t *testing.T) {
	err := checkSrc(t, "portable-go", `
		(use num/f64)
		(fn (a) (if (f64.lt (aindex a 0) 1.0) 2.0 "no"))
	`)
	if err == nil {
		t.Fatal("branches f64 and string should conflict")
	}
}

// §6 — the acceptance test that matters more than the negative one: nothing
// currently correct may be rejected. The structural primitives are where a
// wrong rule would show up first.
func TestCheckerAcceptsTheGauntletShapes(t *testing.T) {
	cases := map[string]string{
		"fold": `(use num/f64)
			(fn (a) (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (aindex a i)))))`,
		"dict accumulator": `
			(fn (ws) (fold-range (dict-empty) (slen ws) (fn (acc i) (dict-inc acc (sat ws i)))))`,
		"make-vec": `(use num/f64) (use num/int)
			(fn (a) (make-vec (int.sub (alen a) 1) (fn (i) (f64.mul (aindex a i) 2.0))))`,
		"loop2": `(use num/f64)
			(fn (xs ys) (fold-range2 0.0 0.0 (alen xs)
			  (fn (ax ay i) (f64.add ax (aindex xs i)))
			  (fn (ax ay i) (f64.add ay (aindex ys i)))
			  (fn (ax ay) (f64.add ax ay))))`,
		"conditional": `(use num/f64)
			(fn (a) (if (f64.gt (aindex a 0) 0.0) 1.0 2.0))`,
	}
	for name, src := range cases {
		if err := checkSrc(t, "portable-go", src); err != nil {
			t.Errorf("%s should typecheck: %v", name, err)
		}
	}
}

// §5 → built. A signature is a claim checked against BOTH the definition and
// any target's native implementation. The second is the job no host compiler
// can do, since the two live on different targets.

func loadWithSigs(t *testing.T, target, src string) (*Target, *core.Program, *core.Env) {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target + ".oro")
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
	err := CheckSignatures(tg, prog, env)
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
	if err := CheckSignatures(tg, prog, env); err != nil {
		t.Errorf("an agreeing signature must pass: %v", err)
	}
}

func TestSignatureCheckedAgainstDefinition(t *testing.T) {
	// go does NOT provide num/vec.dot natively, so the claim is about the
	// definition — which here returns a vec-f64 rather than the declared f64.
	src := `
		(use num/f64)
		(sig bad ((a vec-f64)) f64)
		(def bad (fn (a) (make-vec (alen a) (fn (i) (aindex a i)))))
	`
	tg, prog, env := loadWithSigs(t, "go", src)
	err := CheckSignatures(tg, prog, env)
	if err == nil {
		t.Fatal("a definition disagreeing with its own signature must be caught")
	}
	if !strings.Contains(err.Error(), "vec-f64") {
		t.Errorf("the error should name the actual type, got %v", err)
	}
}

func TestSignatureArityIsChecked(t *testing.T) {
	src := `
		(use num/f64)
		(sig two ((a f64) (b f64)) f64)
		(def two (fn (a) a))
	`
	tg, prog, env := loadWithSigs(t, "go", src)
	if err := CheckSignatures(tg, prog, env); err == nil {
		t.Fatal("an arity mismatch must be caught")
	}
}
