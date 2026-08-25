package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/values.md. `values` is the NEGATIVE PRODUCT: reader sugar for
// `(fn (#k) (#k a b))`, so β is its algebra and the reducer needs nothing.
//
// The first attempt at this feature was REVERTED, and these tests exist mostly
// to pin what it got wrong: it carried a `(multi-return …)` target declaration
// and refused on Java and windows, which makes a construct in the core
// declinable — a library with a portability claim rather than part of the
// language. Every target implements it now, in its backend, like `if`/`let`/`loop`.

func genOn(t *testing.T, target, src, name string) (string, error) {
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
	case "java":
		return JavaMethod(tg, name, prog.Sigs[q], nf)
	case "windows":
		return AsmProc(tg, name, prog.Sigs[q], nf)
	default:
		return Func(tg, name, prog.Sigs[q], nf)
	}
}

// The half that costs nothing: consumed in the same reduction, the product is
// gone before any backend sees it — which is why it measured 1.01x with zero
// allocations (product-2026-08-19) and why `values` is not a tuple.
func TestValuesVanishesWhenConsumed(t *testing.T) {
	code, err := genOn(t, "go", `
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

// EVERY TARGET IMPLEMENTS IT. This is the test the reverted version could not
// have passed, and it is the rule: a construct promoted to the language works
// on every target and the compiler finds the implementation.
func TestEveryTargetHasMultipleResults(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"go", `(use go)
			(export d) (sig d ((a int) (b int)) (int int) (where (go.!= b 0)))
			(def d (fn (a b) (values (go./ a b) (go.% a b))))`,
			"(a int, b int) (int, int)"},
		{"js", `(use js)
			(export d) (sig d ((a any) (b any)) (any any))
			(def d (fn (a b) (values (js.+ a b) (js.- a b))))`,
			"return {f0: (a + b), f1: (a - b)};"},
		{"java", `(use java)
			(export d) (sig d ((a int) (b int)) (int int) (where (java.!= b 0)))
			(def d (fn (a b) (values (java./ a b) (java.% a b))))`,
			"return new Tup_long_long("},
		{"windows", `(use x64)
			(export d) (sig d ((a int) (b int)) (int int))
			(def d (fn (a b) (values (x64.idiv a b) (x64.irem a b))))`,
			"mov rdx,"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "d")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s: missing %q:\n%s", c.target, c.want, code)
		}
	}
}

// x86 needs TWO PASSES, and the first draft had one. Placing a result in its
// return register as it is computed clobbers the earlier ones, because rax and
// rdx are `idiv`'s own operands. The bug was visible in the emitted assembly:
//
//	mov rdi, rax     ; quotient held
//	mov rax, rdi     ; result 0 placed
//	mov rax, rbx     ; ...and immediately destroyed
//
// So no write to a convention register may appear before the last computation.
func TestAsmPlacesResultsAfterComputing(t *testing.T) {
	code, err := genOn(t, "windows", `
		(use x64)
		(export d) (sig d ((a int) (b int)) (int int))
		(def d (fn (a b) (values (x64.idiv a b) (x64.irem a b))))
	`, "d")
	if err != nil {
		t.Fatal(err)
	}
	lastIdiv := strings.LastIndex(code, "idiv")
	firstPlace := strings.Index(code, "mov rax, rdi")
	if lastIdiv < 0 {
		t.Fatalf("expected an idiv:\n%s", code)
	}
	if firstPlace >= 0 && firstPlace < lastIdiv {
		t.Errorf("a result is placed in rax before the last computation, "+
			"which clobbers it:\n%s", code)
	}
}

// Java shares one record per result SHAPE rather than generating one per
// function, so two functions returning (int int) do not make two types.
func TestJavaSharesRecordsByShape(t *testing.T) {
	for k := range JavaRecords {
		delete(JavaRecords, k)
	}
	for _, n := range []string{"d", "e"} {
		if _, err := genOn(t, "java", `
			(use java)
			(export `+n+`) (sig `+n+` ((a int) (b int)) (int int))
			(def `+n+` (fn (a b) (values (java.+ a b) (java.- a b))))
		`, n); err != nil {
			t.Fatal(err)
		}
	}
	if len(JavaRecords) != 1 {
		t.Errorf("two functions of one shape must share one record, got %d: %v",
			len(JavaRecords), JavaRecords)
	}
}

// The arity is checked against the signature, not guessed from the term.
func TestResultArityMustMatch(t *testing.T) {
	_, err := genOn(t, "go", `
		(use go)
		(export three) (sig three ((a int)) (int int int))
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
	_, err := genOn(t, "go", `
		(use go)
		(export pair) (sig pair ((a int) (b int)) any)
		(def pair (fn (a b) (values a b)))
	`, "pair")
	if err == nil || !strings.Contains(err.Error(), "escaping closure") {
		t.Errorf("no multi-result sig means no product, got %v", err)
	}
}

// `(values x)` is refused: one value is just the value, so there is exactly one
// spelling for one result and nothing ambiguous reaches a backend.
func TestValuesNeedsTwoOrMore(t *testing.T) {
	_, err := core.Read(`(def f (fn (a) (values a)))`)
	if err == nil || !strings.Contains(err.Error(), "two or more") {
		t.Errorf("(values x) must be refused, got %v", err)
	}
}

// And a single result declared as a one-element list is the same signature as a
// bare type — one spelling reaching the backends.
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

// A loop variable that REUSES a parameter's name emitted `let n = n;` inside
// `function f(n)` on JavaScript — a SyntaxError, so the module did not parse at
// all. Go and Java seeded their fresh-name set from the parameters from the
// start; JS did not, and nothing noticed for five months because no program had
// written `(loop ((n n)) …)`. `match` makes it the common case, because a
// scrutinee that is a bare name becomes the loop variable under that name.
func TestLoopMayShadowAParameter(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"js", `(use js)
			(export f) (sig f ((n any)) any)
			(def f (fn (n) (loop ((n n)) (js.=== n 0) 7 else (again (js.- n 1)))))`,
			"let n2 = n;"},
		{"go", `(use go)
			(export f) (sig f ((n int)) int)
			(def f (fn (n) (loop ((n n)) (= n 0) 7 else (again (go.- n 1)))))`,
			"var n2 int = n"},
		{"java", `(use java)
			(export f) (sig f ((n int)) int)
			(def f (fn (n) (loop ((n n)) (= n 0) 7 else (again (java.- n 1)))))`,
			"n2 = n"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s: a loop variable must not redeclare a parameter; "+
				"wanted %q in:\n%s", c.target, c.want, code)
		}
	}
}

// `match` reaches all four backends through `loop` and needs nothing from any
// of them — the point of desugaring rather than adding a term kind.
func TestMatchReachesEveryBackend(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"go", `(use go)
			(export g) (sig g ((s int) (n int)) int)
			(def g (fn (s n) (match (s n) 0 v (again 1 (go.- v 1)) else n)))`, "for "},
		{"js", `(use js)
			(export g) (sig g ((s any) (n any)) any)
			(def g (fn (s n) (match (s n) 0 v (again 1 (js.- v 1)) else n)))`, "for (;"},
		{"java", `(use java)
			(export g) (sig g ((s int) (n int)) int)
			(def g (fn (s n) (match (s n) 0 v (again 1 (java.- v 1)) else n)))`, "for (;"},
		{"windows", `(use x64)
			(export g) (sig g ((s int) (n int)) int)
			(def g (fn (s n) (match (s n) 0 v (again 1 (x64.sub v 1)) else n)))`, "Ltop"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "g")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s: match must reach the backend as a loop:\n%s", c.target, code)
		}
	}
}

// A SUM crossing a boundary, on all four targets. It is the tag/payload product
// (docs/spec/sums.md), so nothing new is emitted anywhere — but a function
// returning a sum returns from SEVERAL PLACES, which the single-leaf multiFunc
// could not express and multiTail now does.
func TestSumCrossesEveryBoundary(t *testing.T) {
	cases := []struct {
		target, src string
		want        []string
	}{
		{"go", `(use go)
			(sum result (ok int) (err int))
			(export d) (sig d ((a int) (b int)) result)
			(def d (fn (a b) (if (= b 0) (err 0) (ok (go./ a b)))))`,
			[]string{"(int, int)", "return 1, 0", "return 0, (a / b)"}},
		{"js", `(use js)
			(sum result (ok any) (err any))
			(export d) (sig d ((a any) (b any)) result)
			(def d (fn (a b) (if (= b 0) (err 0) (ok (js./ a b)))))`,
			[]string{"return {f0: 1, f1: 0};", "return {f0: 0, f1: (a / b)};"}},
		{"java", `(use java)
			(sum result (ok int) (err int))
			(export d) (sig d ((a int) (b int)) result)
			(def d (fn (a b) (if (= b 0) (err 0) (ok (java./ a b)))))`,
			[]string{"Tup_long_long", "return new Tup_long_long(1, 0);"}},
		{"windows", `(use x64)
			(sum result (ok int) (err int))
			(export d) (sig d ((a int) (b int)) result)
			(def d (fn (a b) (if (= b 0) (err 0) (ok (x64.idiv a b)))))`,
			[]string{"mov rax, 1", "mov rdx,"}},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "d")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(code, w) {
				t.Errorf("%s: missing %q:\n%s", c.target, w, code)
			}
		}
		// No product is BUILT: the tag and the payload are the two results.
		if strings.Contains(code, "struct{") || strings.Contains(code, "interface{") {
			t.Errorf("%s: a sum must not build anything:\n%s", c.target, code)
		}
	}
}
