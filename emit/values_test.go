package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/values.md. `values` is the NEGATIVE PRODUCT: reader sugar for
// `(fn (#k) (#k a b))`, so β is its algebra and the reducer needs nothing.
// These tests pin the two halves — that it disappears when consumed in the
// same reduction, and that it becomes the host's own multiple return when it
// does not.

func genFor(t *testing.T, target, src, name string) (string, error) {
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
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	switch target {
	case "js":
		return JSFunc(tg, name, prog.Sigs[q], nf)
	default:
		return Func(tg, name, prog.Sigs[q], nf)
	}
}

// The half that costs nothing: consumed in the same reduction, the product is
// gone before any backend sees it. This is why it measured 1.01x with zero
// allocations (product-2026-08-19) and why `values` is not a tuple.
func TestValuesVanishesWhenConsumed(t *testing.T) {
	code, err := genFor(t, "go", `
		(use go)
		(export f)
		(sig f ((a int) (b int)) int (where (go.!= b 0)))
		(def divmod (fn (a b) (values (go./ a b) (go.% a b))))
		(def f (fn (a b) ((divmod a b) (fn (q r) (go.+ q r)))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "return ((a / b) + (a % b))") {
		t.Errorf("the product must reduce away entirely:\n%s", code)
	}
	if strings.Contains(code, "func(") || strings.Contains(code, "struct") {
		t.Errorf("nothing may be built:\n%s", code)
	}
}

// The half that reaches a boundary: Go returns two values in registers.
func TestValuesBecomesGoMultipleReturn(t *testing.T) {
	code, err := genFor(t, "go", `
		(use go)
		(export divmod)
		(sig divmod ((a int) (b int)) (int int) (where (go.!= b 0)))
		(def divmod (fn (a b) (values (go./ a b) (go.% a b))))
	`, "divmod")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"(a int, b int) (int, int)", "return (a / b), (a % b)"} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q:\n%s", want, code)
		}
	}
}

// JavaScript has no multiple return, so the target declares an array literal
// and the price is in the declaration rather than in the compiler.
func TestValuesBecomesJSArray(t *testing.T) {
	code, err := genFor(t, "js", `
		(use js)
		(export split)
		(sig split ((a any) (b any)) (any any))
		(def split (fn (a b) (values (js.+ a b) (js.- a b))))
	`, "split")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "return [(a + b), (a - b)];") {
		t.Errorf("JS returns an array literal:\n%s", code)
	}
}

// A target that declares no multiple-return form REFUSES, and the refusal
// names the capability rather than complaining about a closure. This is the
// capability graph answering — the same shape as JavaScript declaring no
// `checked` primitive (selection-2026-08-19).
func TestATargetWithoutMultipleReturnRefuses(t *testing.T) {
	_, err := genFor(t, "java", `
		(use java)
		(export split)
		(sig split ((a int) (b int)) (int int))
		(def split (fn (a b) (values (java.+ a b) (java.- a b))))
	`, "split")
	if err == nil {
		t.Fatal("java declares no multiple-return form and must refuse")
	}
	if !strings.Contains(err.Error(), "no multiple-return form") {
		t.Errorf("the refusal must name the capability, got: %v", err)
	}
}

// The arity is checked against the signature, not guessed from the term.
func TestResultArityMustMatch(t *testing.T) {
	_, err := genFor(t, "go", `
		(use go)
		(export three)
		(sig three ((a int)) (int int int))
		(def three (fn (a) (values a a)))
	`, "three")
	if err == nil || !strings.Contains(err.Error(), "does not produce them") {
		t.Errorf("declaring 3 results and producing 2 must be refused, got %v", err)
	}
}

// The disambiguator is the SIGNATURE. `(fn (k) (k a b))` read one way is a
// product and read the other way is a genuine higher-order function; without a
// multi-result signature it stays an escaping closure and stays refused.
func TestWithoutAMultiResultSigItIsStillAClosure(t *testing.T) {
	_, err := genFor(t, "go", `
		(use go)
		(export pair)
		(sig pair ((a int) (b int)) any)
		(def pair (fn (a b) (values a b)))
	`, "pair")
	if err == nil || !strings.Contains(err.Error(), "escaping closure") {
		t.Errorf("no multi-result sig means no product, got %v", err)
	}
}

// `(values x)` is refused: one value is just the value, so there is exactly
// one spelling for one result and no ambiguity in the residual.
func TestValuesNeedsTwoOrMore(t *testing.T) {
	_, err := core.Read(`(def f (fn (a) (values a)))`)
	if err == nil || !strings.Contains(err.Error(), "two or more") {
		t.Errorf("(values x) must be refused, got %v", err)
	}
}

// And a single result declared as a one-element list is the same signature as
// a bare type — one spelling reaching the backends.
func TestOneResultListIsABareType(t *testing.T) {
	forms, err := core.Read(`(sig f ((a int)) (int))`)
	if err != nil {
		t.Fatal(err)
	}
	s := forms[0].Sig
	if s.Result != "int" || len(s.Results) != 0 {
		t.Errorf("(int) must normalise to the bare result, got %q / %v", s.Result, s.Results)
	}
}
