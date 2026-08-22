package core

import (
	"strings"
	"testing"
)

// docs/spec/sums.md. A sum is Σ over a finite index set — the dual of a table's
// Π — and the difference is the whole design: a Π can be given by a RULE and
// store nothing, a Σ must carry WHICH. So a sum value is a tag and a payload,
// which is a PRODUCT, and the product was already built on all four targets.
//
// Zero new term kinds. Two reduction additions, both narrow: `=` folds on
// integer literals, and an eliminator commutes through `if` and `let`.

func loadSrc(t *testing.T, src string) (*Program, error) {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		return nil, err
	}
	p, _, err := Load(forms)
	return p, err
}

func mustLoadFail(t *testing.T, src, want string) {
	t.Helper()
	_, err := loadSrc(t, src)
	if err == nil {
		t.Fatalf("expected a refusal mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q, got: %v", want, err)
	}
}

// A sum declaration generates DEFINITIONS and nothing else. That is what makes
// qualification, imports, δ and the occurrence counter apply to a constructor
// without any of them learning that sums exist.
func TestSumGeneratesConstructors(t *testing.T) {
	p, err := loadSrc(t, `(sum result (ok int) (err int))`)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"ok", "err", "ok#tag", "err#tag"} {
		if _, have := p.Defs[n]; !have {
			t.Errorf("%s should be a definition; got %v", n, p.Order)
		}
	}
	if got := p.Defs["ok"].String(); got != "(fn (#p) (fn (#x) (#x 0 #p)))" {
		t.Errorf("a constructor is the tag/payload product: %s", got)
	}
	if got := p.Defs["err#tag"].String(); got != "1" {
		t.Errorf("the tag is a literal, so a clause can compare against it: %s", got)
	}
}

// An enum is a sum with no payloads — the degenerate case, not a new concept.
func TestEnumIsASumWithNoPayloads(t *testing.T) {
	p, err := loadSrc(t, `(sum colour red green blue)`)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Defs["blue#tag"].String(); got != "2" {
		t.Errorf("variants are tagged in declaration order: %s", got)
	}
}

// --- what a sum REFUSES ------------------------------------------------------

func TestSumNeedsTwoVariants(t *testing.T) {
	mustLoadFail(t, `(sum wrapper (only int))`, "two or more variants")
}

func TestSumRefusesDuplicateVariant(t *testing.T) {
	mustLoadFail(t, `(sum r (ok int) (ok int))`, "declared twice")
}

// A constructor names ONE sum. Sums are nominal — `(ok 3)` does not determine
// its type — which is why every language without runtime types went nominal here.
func TestVariantMayNotBelongToTwoSums(t *testing.T) {
	// In one module the constructor-definition collision fires first, and it
	// says more: the two sums cannot both define `ok`. Across modules there is
	// no shared definition namespace, so the byVariant check is what catches it.
	mustLoadFail(t, `(sum a (ok int) (no int)) (sum b (ok int) (yes int))`,
		"sum b declares a variant of that name")
}

// --- case --------------------------------------------------------------------

// Exhaustiveness is decidable by COUNTING, because a sum is closed and finite —
// and it is what earns the last clause its missing test.
func TestCaseMustBeExhaustive(t *testing.T) {
	mustLoadFail(t, `
		(sum result (ok int) (err int))
		(def f (fn (r) (case r (ok v) v)))`,
		"not exhaustive: err is unmatched")
}

// The converse: an `else` under a complete match is dead code, and saying so is
// more useful than silently accepting it.
func TestElseUnderACompleteMatchIsDead(t *testing.T) {
	mustLoadFail(t, `
		(sum result (ok int) (err int))
		(def f (fn (r) (case r (ok v) v (err e) e else 0)))`,
		"`else` is dead code")
}

func TestCaseRefusesUnknownVariant(t *testing.T) {
	mustLoadFail(t, `
		(sum result (ok int) (err int))
		(def f (fn (r) (case r (ok v) v (nope e) e)))`,
		"not a variant of any sum")
}

func TestCaseRefusesMixingSums(t *testing.T) {
	mustLoadFail(t, `
		(sum a (ok int) (no int))
		(sum b (yes int) (nah int))
		(def f (fn (r) (case r (ok v) v (yes e) e)))`,
		"one case eliminates one sum")
}

func TestCaseRefusesRepeatedVariant(t *testing.T) {
	mustLoadFail(t, `
		(sum result (ok int) (err int))
		(def f (fn (r) (case r (ok v) v (ok e) e)))`,
		"matched twice")
}

// A variant that carries nothing has nothing to bind.
func TestCaseRefusesBindingOnAnEmptyVariant(t *testing.T) {
	mustLoadFail(t, `
		(sum colour red green)
		(def f (fn (c) (case c (red x) x (green y) 1)))`,
		"carries no payload")
}

// A sum crossing a boundary is transmitted as its tag and its payload, so the
// payload needs one type. Inside a program a mixed sum is fine, because
// reduction removes it — which is why the refusal is on the SIGNATURE.
func TestMixedPayloadsRefusedOnlyAtABoundary(t *testing.T) {
	if _, err := loadSrc(t, `
		(sum mixed (num int) (name string))
		(def f (fn (r) (case r (num v) v (name s) 0)))`); err != nil {
		t.Errorf("a mixed sum is fine inside a program: %v", err)
	}
	mustLoadFail(t, `
		(sum mixed (num int) (name string))
		(sig f ((a int)) mixed)
		(def f (fn (a) (num a)))`,
		"different payload types")
}

// And a sum in a signature IS the product of its tag and its payload — which is
// the whole representation story, because two results already exist on all four
// targets.
func TestSumInASignatureIsTwoResults(t *testing.T) {
	p, err := loadSrc(t, `
		(sum result (ok int) (err int))
		(sig f ((a int)) result)
		(def f (fn (a) (ok a)))`)
	if err != nil {
		t.Fatal(err)
	}
	sig := p.Sigs["f"]
	if len(sig.Results) != 2 || sig.Results[0] != "int" || sig.Results[1] != "int" {
		t.Errorf("a sum result is (tag payload): %v / %q", sig.Results, sig.Result)
	}
}

// --- the two reduction additions ---------------------------------------------

func reduceTo(t *testing.T, src, export string) string {
	t.Helper()
	p, err := loadSrc(t, src)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnv(p, "go.+", "go.-", "go./", "go.>", "go.%", "if", "=")
	nf, err := Normalize(p.Defs[export], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return nf.String()
}

// THE STATIC SUM IS FREE, which is the two-level language's claim stated about
// sums: `(case (ok n) …)` has no tag, no test and no product in the residual.
// The Church-encoded sum always worked here (sums-research.md §0); what it
// needed was `=` folding, or the tag became a literal and left `(= 0 0)` behind.
func TestStaticSumVanishes(t *testing.T) {
	got := reduceTo(t, `
		(use go)
		(sum result (ok int) (err int))
		(def f (fn (n) (case (ok n) (ok v) (go.+ v 1) (err e) e)))`, "f")
	if got != "(fn (n) (go.+ n 1))" {
		t.Errorf("a static sum must leave nothing behind: %s", got)
	}
}

// AND SO IS THE DYNAMIC ONE, which is what case-of-case buys. Without it the
// residual is stuck at `((if c A B) F G)` — the constructor under the branch,
// the eliminator outside it, neither able to see the other — and no backend can
// emit that.
func TestDynamicSumVanishesThroughCaseOfCase(t *testing.T) {
	got := reduceTo(t, `
		(use go)
		(sum result (ok int) (err int))
		(def f (fn (n) (case (if (go.> n 0) (ok n) (err 0)) (ok v) (go.+ v 1) (err e) e)))`, "f")
	if got != "(fn (n) (if (go.> n 0) (go.+ n 1) 0))" {
		t.Errorf("case-of-case must reunite each constructor with the eliminator: %s", got)
	}
}

// The `let` companion, which the nested test demanded: β itself puts a `let`
// between a constructor and its eliminator when a shared subterm is not
// duplicable, so handling only `if` stops one level in.
func TestEliminatorCommutesThroughLet(t *testing.T) {
	got := reduceTo(t, `
		(use go)
		(sum result (ok int) (err int))
		(def step (fn (r k)
			(case r (ok v) (if (go.> v k) (ok (go.- v k)) (err v)) (err e) (err e))))
		(def f (fn (a b) (case (step (step (ok a) b) b) (ok v) v (err e) (go.- 0 e))))`, "f")
	// `#x` is the product's SELECTOR. Its absence is the whole claim: no
	// closure and no tag survive. The `(fn (#c) …)` that remain are `let`
	// binders, which is β doing what it always did.
	if strings.Contains(got, "#x") {
		t.Errorf("no product may survive a nested sum: %s", got)
	}
	if !strings.HasPrefix(got, "(fn (a b) (if (go.> a b)") {
		t.Errorf("nested sums must reduce to plain control flow: %s", got)
	}
}

// `=` folds on integer LITERALS only. ADR 0009 permits exactly that: integer
// equality inside the portable window is bit-identical on every target, which
// is the thing that is NOT true of float arithmetic — Go folds `0.1+0.2` two
// different ways, which is why nothing here folds a float.
func TestEqFoldsOnIntegersOnly(t *testing.T) {
	e := testEnv(NewProgram(), "go.f+", "=")
	for src, want := range map[string]string{
		"(= 2 2)":     "true",
		"(= 2 3)":     "false",
		"(= 1.0 1.0)": "(= 1.0 1.0)", // floats are NOT folded, deliberately
	} {
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		nf, err := Normalize(forms[0].Term, e, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if nf.String() != want {
			t.Errorf("%s reduced to %s, want %s", src, nf, want)
		}
	}
}
