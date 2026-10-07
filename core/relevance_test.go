package core

import (
	"strings"
	"testing"
)

// RELEVANCE (spec/errors.md §7): weakening is denied for a value of a
// `(relevant)` type. The check is at β, where a binder the program wrote meets
// its value, and the binder must be used on every path of the λ as written.
//
// `write` is the shape of a fallible host call: an effect, then a constructor
// of the relevant `result`. `opt` is the same through a variant that is not
// relevant.
const relevanceDecl = `
	(use go)
	(variant (result T E) (ok T) (err E) (success ok) (relevant))
	(variant (maybe T) (just T) nothing (success just))
	(def write (fn (n) (seq (go.print n) (if (go.> n 0) (ok n) (err n)))))
	(def opt (fn (n) (seq (go.print n) (if (go.> n 0) (just n) nothing))))
	(def drop (fn (x) 0))
	(def check (fn (r) (case r (ok v) v (err e) (go.- 0 e))))`

var relevancePrims = []string{"go.>", "go.-", "if", "=", "!go.print", "!abandon", "ignore", "loop"}

func relevanceReduce(t *testing.T, body string) (string, error) {
	t.Helper()
	p, err := loadSrc(t, relevanceDecl+`(def f (fn (n c) `+body+`))`)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := Normalize(p.Defs["f"], testEnv(p, relevancePrims...), DefaultFuel)
	if err != nil {
		return "", err
	}
	return nf.String(), nil
}

func TestARelevantValueMustBeUsedOnEveryPath(t *testing.T) {
	for _, c := range []struct {
		body string
		ok   bool
	}{
		// in statement position, and bound and never read
		{`(seq (write n) 1)`, false},
		{`(let r (write n) 1)`, false},
		{`(let (tuple a _) (tuple 1 (write n)) a)`, false},
		// handed to a definition that drops it: η keeps the mark on the tuple
		// that stands for r, so drop's β meets the rule
		{`(let r (write n) (drop r))`, false},
		{`(drop (write n))`, false},
		// a pure constructor, as written
		{`(seq (ok 1) 1)`, false},
		// used: eliminated, or handed to a definition that eliminates it
		{`(let r (write n) (check r))`, true},
		{`(case (write n) (ok v) v (err e) e)`, true},
		{`(check (write n))`, true},
		// discarded on purpose
		{`(seq (ignore (write n)) 1)`, true},
		{`(let r (write n) (seq (ignore r) 1))`, true},
		// ADDITIVE: an `if` uses r when its condition does, or both arms do
		{`(let r (write n) (if c (check r) 0))`, false},
		{`(let r (write n) (if c (check r) (seq (ignore r) 0)))`, true},
		{`(let r (write n) (if (go.> (check r) 0) 1 2))`, true},
		// `and` is an `if` whose second arm is false: short-circuit loses r
		{`(let r (write n) (if (and c (go.> (check r) 0)) 1 2))`, false},
		// try's failure arm returns the second error and loses the first
		{`(let w (write n) (try (ok u) (write 1) (ok (check w))))`, false},
		// a path that crashes uses it vacuously: 0 is initial
		{`(let r (write n) (if c (check r) (abandon "x")))`, true},
		// a loop uses it when its exits do; an `again` continues
		{`(let r (write n) (loop ((i 0)) (go.> n i) (again (go.- i 1)) else (check r)))`, true},
		{`(let r (write n) (loop ((i 0)) (go.> n i) (again (go.- i 1)) else 0))`, false},
		// a relevant payload: the slot that binds it is the compiler's, and
		// is read by the one arm that selects it
		{`(case (ok (ok n)) (ok r) (check r) (err e) e)`, true},
		// a variant that is not relevant may be dropped
		{`(seq (opt n) 1)`, true},
		{`(let m (opt n) (drop m))`, true},
	} {
		_, err := relevanceReduce(t, c.body)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused, and it uses its relevant value: %v", c.body, err)
		case !c.ok && err == nil:
			t.Errorf("%s: accepted, and it discards a relevant value", c.body)
		case !c.ok && !strings.Contains(err.Error(), "relevant type"):
			t.Errorf("%s: refused for another reason: %v", c.body, err)
		}
	}
}

// `ignore` IS !_A : A → 1, and erases (spec/errors.md §7.3): on a relevant
// value it is the eliminator with a constant body, whose arms agree, so the
// test goes by idempotence and only the effect is left. η for the terminal
// object then takes the unit's binding apart, so `(seq (ignore e) B)` is what
// `(seq e B)` was before relevance: the effect, then B.
func TestIgnoreLeavesTheEffectAndNothingElse(t *testing.T) {
	got, err := relevanceReduce(t, `(seq (ignore (write n)) 1)`)
	if err != nil {
		t.Fatal(err)
	}
	want, err := relevanceReduce(t, `(seq (opt n) 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || strings.Contains(got, "ignore") || strings.Contains(got, "#k") {
		t.Errorf("ignore must leave the effect alone:\n got  %s\n want %s", got, want)
	}
	// on a value of any other type: pure, the unit; impure, its effect
	for body, want := range map[string]string{
		`(seq (ignore (go.- n 1)) 1)`:   `(fn (n c) 1)`,
		`(seq (ignore (go.print n)) 1)`: `(fn (n c) (let (go.print n) (fn (`,
	} {
		got, err := relevanceReduce(t, body)
		if err != nil || !strings.HasPrefix(got, want) {
			t.Errorf("%s: got %s, %v; want %s…", body, got, err, want)
		}
	}
}

// The refusal says what was discarded, and how.
func TestARelevanceRefusalNamesTheValueAndTheBinder(t *testing.T) {
	for body, want := range map[string]string{
		`(seq (write n) 1)`:                    "the result of write is a value of a relevant type (spec/errors.md §7), and it is discarded",
		`(let r (write n) (if c (check r) 0))`: "the binder r is not used on every path",
	} {
		_, err := relevanceReduce(t, body)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", body, want, err)
		}
	}
}
