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
		(fn (a) (if (go.s< a "x") (go.string-of-bytes a) "y"))
	`)
	if err == nil {
		t.Fatal("a used as both a host string and a table of bytes should conflict")
	}
	if !strings.Contains(err.Error(), "bytestring") || !strings.Contains(err.Error(), "array int 0 255") {
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
	// here returns a host call's table rather than the declared f64.
	src := `
		(use go)
		(sig bad ((a (array f64))) f64)
		(def bad (a) (go.bytes-of-string "x"))
	`
	tg, prog, env := loadWithSigs(t, "go", src)
	err := CheckSignatures(tg, prog, env, nil)
	if err == nil {
		t.Fatal("a definition disagreeing with its own signature must be caught")
	}
	if !strings.Contains(err.Error(), "array int 0 255") {
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

// A TABLE FORM HAS A TABLE'S TYPE (types.md §3.1, tabletype-2026-09-29). Every
// table and map form was typed unknown, which agrees with everything, so a
// signature was never checked against a table-valued body: `(array 1.0 2.0)`
// under a result declared f64 compiled to a Go function returning []float64.
func claimErr(t *testing.T, result, body string) error {
	t.Helper()
	tg, prog, env := loadWithSigs(t, "go", "(use go)\n(sig bad ((a (array f64))) "+result+")\n(def bad (a) "+body+")\n")
	return CheckSignatures(tg, prog, env, nil)
}

func TestATableFormIsNotAScalar(t *testing.T) {
	for _, c := range []struct{ result, body, is string }{
		{"f64", "(array 1.0 2.0)", "is array f64"},
		{"f64", "(table 3 (fn (i) 1.0))", "is array f64"},
		{"f64", "(alloc (table 3 (fn (i) 1.0)))", "is a buffer"},
		{"f64", "(local b (table 3 0) b)", "is a buffer"},
		{"f64", "(local b (table 3 0) (set b 0 1.5))", "is buffer f64"},
		{"f64", "(build-map m 4 m)", "is a map"},
		// and a constructor is not another constructor
		{"(array f64)", "(build-map m 4 m)", "is a map"},
		{"(map int f64)", "(array 1.0)", "is array f64"},
	} {
		err := claimErr(t, c.result, c.body)
		if err == nil || !strings.Contains(err.Error(), c.is+", but") {
			t.Errorf("%s under %s: want a refusal that it %s, got %v", c.body, c.result, c.is, err)
		}
	}
}

// …and one with an unknown element meets every table: the language's, a
// buffer, and a host type that realizes one. A known element is still
// compared, invariantly.
func TestATableFormMeetsATableDemand(t *testing.T) {
	for _, c := range []struct{ result, body string }{
		{"(array f64)", "(array 1.0 2.0)"},
		{"(array f64)", "(local b (table 3 0) b)"},
		{"(array f64)", "(alloc (table 3 (fn (i) 1.0)))"},
		{"(array (int 0 255))", "(array 104 105 33)"},
		{"(map int f64)", "(build-map m 4 m)"},
		{"int", "(len (array 1.0 2.0))"},
	} {
		if err := claimErr(t, c.result, c.body); err != nil {
			t.Errorf("%s under %s: %v", c.body, c.result, err)
		}
	}
	// A HOST TYPE REALIZING A TABLE: Java's double-array is double[], which is
	// ρ(array f64). Go has none left since gotarget-2026-09-30 removed its
	// per-element slice names, so the witness is Java's.
	for _, body := range []string{"(array 1.0 2.0)", "(local b (table 3 0) (set b 0 1.5))"} {
		tg, prog, env := loadWithSigs(t, "java", "(use java)\n(sig bad ((a (array f64))) double-array)\n(def bad (a) "+body+")\n")
		if err := CheckSignatures(tg, prog, env, nil); err != nil {
			t.Errorf("%s under double-array: %v", body, err)
		}
	}
}

// A DEMANDED TABLE'S ENTRIES ARE DEMANDED AT ITS ELEMENT: a graph's entries, a
// rule's body, and a store's value into a buffer whose element is known.
func TestATableFormsEntriesAreDemanded(t *testing.T) {
	for _, c := range []struct{ result, body, in string }{
		{"(array f64)", `(array 1.0 "a")`, "in entry 2 of a table"},
		{"(array f64)", `(table 3 (fn (i) "a"))`, "in a table's rule"},
	} {
		err := claimErr(t, c.result, c.body)
		if err == nil || !strings.Contains(err.Error(), c.in) || !strings.Contains(err.Error(), "string") {
			t.Errorf("%s under %s: want a refusal %q, got %v", c.body, c.result, c.in, err)
		}
	}
	tg, prog, env := loadWithSigs(t, "go", `
		(sig put ((b (buffer f64))) (buffer f64))
		(def put (b) (set b 0 "x"))
	`)
	err := CheckSignatures(tg, prog, env, nil)
	if err == nil || !strings.Contains(err.Error(), "in a store's value") {
		t.Errorf("a string stored into a buffer of f64: want a refusal, got %v", err)
	}
}

// A SCOPE'S VALUE IS FROZEN AS IT LEAVES (ADR 0031): a host call that gives
// back `buffer σ` makes the scope an `array σ`, which is what an `(array σ)`
// result or parameter reads. Without it the encoding/hex acceptance program
// was refused, "enc is buffer int 0 255, but array int 0 255 is required" —
// found by running the acceptance programs, which the emission sweep does not
// compile (tabletype-2026-09-29 §5).
func TestAScopesBufferIsFrozenAsItLeaves(t *testing.T) {
	tg, prog, env := loadWithSigs(t, "go", `
		(use go/encoding/binary)
		(sig enc ((x (int 0 1000))) (array (int 0 255)))
		(def enc (x) (local b (table 0 0) (binary.AppendUvarint b x)))
	`)
	if err := CheckSignatures(tg, prog, env, nil); err != nil {
		t.Errorf("a frozen buffer is a table: %v", err)
	}
}

// THE ELIMINATOR (types.md §3.1, tableelim-2026-09-29): a read (a i) has the
// element's type, and (len a) is an int. Without it a read was unknown, which
// agrees with every demand, and `(len a)` under a result declared `string`
// printed `func F(a []float64) int`.
func TestATablesReadAndLengthAreTyped(t *testing.T) {
	for _, c := range []struct{ target, src, is string }{
		{"go", `(use go) (sig f ((a (array string))) f64 (where (< 0 (len a)))) (def f (a) (go.f+ (a 0) 1.0))`, "(a …) is string, but f64"},
		{"js", `(use js) (sig f ((a (array string))) any (where (< 0 (len a)))) (def f (a) (+ (a 0) 1))`, "(a …) is string, but int"},
		{"go", `(sig f ((a (array f64))) string) (def f (a) (len a))`, "(len …) is int, but string"},
		{"go", `(sig f () string) (def f () (let t (local b (table 2 0) (set b 0 1.5)) (t 0)))`, "(t …) is f64, but string"},
	} {
		tg, prog, env := loadWithSigs(t, c.target, c.src)
		err := CheckSignatures(tg, prog, env, nil)
		if err == nil || !strings.Contains(err.Error(), c.is) {
			t.Errorf("%s: want %q, got %v", c.src, c.is, err)
		}
	}
}

// THE STORE SOLVES AN OPEN ELEMENT, so a second store is checked against the
// first. On JavaScript the program stored the string "x" into a table of f64
// and ran; on Go it reached Go's compiler as generated code.
func TestAStoreSolvesTheElement(t *testing.T) {
	for _, target := range []string{"go", "js"} {
		tg, prog, env := loadWithSigs(t, target, `(sig f () any) (def f () (local b (table 2 0) (set (set b 0 1.5) 1 "x")))`)
		err := CheckSignatures(tg, prog, env, nil)
		if err == nil || !strings.Contains(err.Error(), "in a store's value") {
			t.Errorf("%s: two stores of different types: want a refusal, got %v", target, err)
		}
	}
}

// AN INTEGER SOLVES THE SORT, NOT A REALIZATION (ADR 0033): a store of 104
// makes a table of integers that meets (array (int 0 255)), and still refuses
// a float.
func TestAnIntegerStoreLeavesTheRealizationOpen(t *testing.T) {
	tg, prog, env := loadWithSigs(t, "go", `(sig f () (array (int 0 255))) (def f () (local b (table 2 0) (set b 0 104)))`)
	if err := CheckSignatures(tg, prog, env, nil); err != nil {
		t.Errorf("a stored integer fixed a realization: %v", err)
	}
	tg, prog, env = loadWithSigs(t, "go", `(sig f () any) (def f () (local b (table 2 0) (set (set b 0 104) 1 1.5)))`)
	if err := CheckSignatures(tg, prog, env, nil); err == nil || !strings.Contains(err.Error(), "in a store's value") {
		t.Errorf("a float stored after an integer: want a refusal, got %v", err)
	}
	// and a table of integers, realization open, is still not a table of f64
	tg, prog, env = loadWithSigs(t, "go", `(sig f () (array f64)) (def f () (local b (table 2 0) (set b 0 104)))`)
	if err := CheckSignatures(tg, prog, env, nil); err == nil {
		t.Error("a table of integers met a declared (array f64)")
	}
}

// THE UNCONSTRAINED VALUE IS OF EVERY TYPE, ∀α. α (ADR 0042, Theorem R): two
// unselected slots of two types in one function each agree with their own
// demand. Taken for a name, `#any` took its first demand's type, int here,
// and the second, a string, conflicted with it (local-2026-10-08).
func TestTheUnconstrainedValueIsOfEveryType(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	any := core.Name(core.AnyName)
	term := core.Fn([]string{"x"}, core.App(core.Name("let"),
		core.App(core.Name("+"), any, core.Int(1)),
		core.Fn([]string{"a"}, core.App(core.Name("concat"), any, core.Str("s")))))
	if err := Check(tg, "test", term); err != nil {
		t.Errorf("%s: %v", term, err)
	}
}
