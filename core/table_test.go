package core

import (
	"strings"
	"testing"
)

// docs/spec/tables.md. A table is A FUNCTION WITH A KNOWN FINITE DOMAIN, and
// everything below follows from taking that literally:
//
//	(array e…)   a graph — an extensional presentation
//	(table n f)  a rule  — an intensional one, with NO MEMORY
//	(len t)      the domain bound
//	(t i)        the element at i — APPLICATION, not a named operation
//
// These tests pin the reducer's half. The emitters' half is in emit/.

func tableEnv() *Env {
	e := &Env{Defs: map[string]*Term{}, Prim: map[string]bool{}, Pure: map[string]bool{}}
	for _, n := range []string{"array", "table", "len", "if", "let", "loop", "=", "+"} {
		e.Prim[n], e.Pure[n] = true, true
	}
	return e
}

func tabNorm(t *testing.T, e *Env, src string) string {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	var term *Term
	for _, f := range forms {
		if f.Kind == "term" {
			term = f.Term
		}
		if f.Kind == "def" {
			e.Defs[f.Name] = f.Term
		}
	}
	if term == nil {
		t.Fatal("no term")
	}
	nf, err := Normalize(term, e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return nf.String()
}

// β-tab, THE SECOND CLAUSE OF β. A function given by its graph is applied by
// looking the argument up — the extensional counterpart of substituting into a
// body, and the same judgement in both cases (tables.md §4.1).
func TestBetaTabOnAGraph(t *testing.T) {
	e := tableEnv()
	if got := tabNorm(t, e, `((array 10 20 30) 2)`); got != "30" {
		t.Errorf("looking up element 2 must give 30, got %s", got)
	}
	if got := tabNorm(t, e, `((array 10 20 30) 0)`); got != "10" {
		t.Errorf("element 0, got %s", got)
	}
}

// A rule-table applied to ANY index is the rule applied to it, which is what
// makes a table-of-a-rule fuse. No literal is required, because there is
// nothing to look up.
func TestBetaTabOnARule(t *testing.T) {
	e := tableEnv()
	if got := tabNorm(t, e, `((table 5 (fn (i) (+ i 1))) 3)`); got != "(+ 3 1)" {
		t.Errorf("a rule applies to any index, got %s", got)
	}
}

// A DYNAMIC index into a graph is left alone — there is nothing to look up
// until the index is known, and the backend emits the host's own indexing.
// That is the whole point of indexing being application: `(a i)` is the same
// text whether it reduces here or survives (tables.md §3.1).
func TestDynamicIndexSurvives(t *testing.T) {
	e := tableEnv()
	got := tabNorm(t, e, `(fn (i) ((array 1 2 3) i))`)
	if !strings.Contains(got, "(array 1 2 3) i") {
		t.Errorf("a dynamic index must survive to the backend, got %s", got)
	}
}

// An out-of-domain literal index is NOT an error here. Reduction leaves it
// alone so the diagnostic comes from the refinement layer, which has the bound
// and the call site and can explain itself (tables.md §6).
func TestOutOfDomainIsLeftToTheRefiner(t *testing.T) {
	e := tableEnv()
	got := tabNorm(t, e, `((array 1 2 3) 7)`)
	if !strings.Contains(got, "array") {
		t.Errorf("reduction must not decide an out-of-domain index, got %s", got)
	}
}

// `len` folds on both presentations. This joins `if` on a boolean literal and
// `=` on two integers under one rule — a construct decided by a literal it can
// see — rather than being a new one (tables.md §4.2).
func TestLenFoldsOnBothPresentations(t *testing.T) {
	e := tableEnv()
	if got := tabNorm(t, e, `(len (array 1 2 3 4))`); got != "4" {
		t.Errorf("len of a graph is its element count, got %s", got)
	}
	if got := tabNorm(t, e, `(fn (n) (len (table n (fn (i) i))))`); got != "(fn (n) n)" {
		t.Errorf("len of a rule is its declared length, got %s", got)
	}
}

// `len` also folds through a DEFINITION, which needs the argument normalised
// first — δ is what turns the name into the `(array …)` whose length is
// visible, and the first version checked before δ had run.
func TestLenFoldsThroughADefinition(t *testing.T) {
	e := tableEnv()
	if got := tabNorm(t, e, `(def a (array 1 2 3)) (len a)`); got != "3" {
		t.Errorf("len must see through a definition, got %s", got)
	}
}

// THE FUSION, which is the whole reason the rule form exists.
//
// `sum` mentions its argument twice — as `(len v)` and as `(v i)` — so β would
// normally let-bind the table and the intermediate would survive. A rule-table
// is duplicable because copying it copies a DESCRIPTION, and substituting it
// makes both mentions disappear: `(len (table n f))` folds to `n` and
// `((table n f) i)` is `(f i)`. What looks like duplication is the step that
// erases the intermediate.
func TestARuleTableFuses(t *testing.T) {
	e := tableEnv()
	got := tabNorm(t, e, `
		(def use2 (fn (v) (+ (len v) (v 0))))
		(fn (n) (use2 (table n (fn (i) (+ i 1)))))`)
	if strings.Contains(got, "table") {
		t.Errorf("the intermediate table must not survive:\n%s", got)
	}
	if got != "(fn (n) (+ n (+ 0 1)))" {
		t.Errorf("expected the fused form, got %s", got)
	}
}

// A GRAPH is not duplicable, and the consequence is worth pinning rather than
// fixing.
//
// A rule is a length and a function, so copying it copies a description. A
// graph is DATA, so copying it copies the elements — which is the code growth
// staticdata-2026-08-20 measured as a pure loss on Java and JavaScript. So a
// graph mentioned twice is let-bound and SHARED.
//
// The cost is a missed fold: with the array behind a binder, β-tab cannot see
// it, so `(v 0)` survives even though both the table and the index are
// literals. That is exactly the constant-folding work tables.md §4.3 defers —
// "do β-tab now as a clause of β; build folding when integers need it" — and it
// is a missed optimisation, not a wrong answer.
func TestAGraphIsSharedNotDuplicated(t *testing.T) {
	e := tableEnv()
	got := tabNorm(t, e, `
		(def use2 (fn (v) (+ (v 0) (v 1))))
		(use2 (array 7 9))`)
	if !strings.Contains(got, "let") {
		t.Errorf("a graph used twice must be shared, not copied:\n%s", got)
	}
	if strings.Count(got, "array") != 1 {
		t.Errorf("the elements must appear once:\n%s", got)
	}
}

// Used ONCE, a graph is substituted and folds away completely — which is the
// case that matters, and the one a fused pipeline produces.
func TestAGraphUsedOnceFoldsAway(t *testing.T) {
	e := tableEnv()
	if got := tabNorm(t, e, `
		(def use1 (fn (v) (+ (v 1) 100)))
		(use1 (array 7 9))`); got != "(+ 9 100)" {
		t.Errorf("one mention folds to the element, got %s", got)
	}
}
