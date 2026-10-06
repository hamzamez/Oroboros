package core

import (
	"strings"
	"testing"
)

// `try` IS THE EXCEPTION MONAD'S BIND (spec/errors.md §5). It expands to an
// exhaustive `case` that continues on the success constructor and returns every
// other one unchanged, so a chain of fallible steps reduces to the tests the
// steps make, with no tag left.

const resultDecl = `
	(use go)
	(variant (result T E) (ok T) (err E) (success ok) (relevant))
	(def half (fn (n) (if (go.> (go.% n 2) 0) (err n) (ok (go./ n 2)))))`

func TestTryIsBindAndLeavesNoTag(t *testing.T) {
	got := reduceWith(t, resultDecl+`
		(def quarter (fn (n) (try (ok h) (half n) (half h))))
		(def f (fn (n) (case (quarter n) (ok v) v (err e) (go.- 0 e))))`,
		"f", "go.>", "go.%", "go./", "go.-", "if", "=")
	if strings.Contains(got, "#k") || strings.Contains(got, "#t") {
		t.Errorf("a tag or a constructor survived the chain: %s", got)
	}
	if !strings.HasPrefix(got, "(fn (n) (if (go.> (go.% n 2) 0) (go.- 0 n)") {
		t.Errorf("the first step's failure must return at once: %s", got)
	}
}

// Left identity, (try (ok x) (ok v) M) = M[v/x]: a known success is the body.
func TestTryLeftIdentity(t *testing.T) {
	got := reduceWith(t, resultDecl+`
		(def f (fn (n) (case (try (ok x) (ok n) (ok (go.- x 1))) (ok v) v (err e) 0)))`,
		"f", "go.-", "if", "=")
	if got != "(fn (n) (go.- n 1))" {
		t.Errorf("left identity: got %s", got)
	}
}

// try reads a map: option marks `some` as its success.
func TestTryOnOption(t *testing.T) {
	got := reduceWith(t, `
		(use go)
		(def f (fn (n) (case (try (some x) (if (go.> n 0) (some n) none) (some (go.- x 1))) (some v) v none 0)))`,
		"f", "go.>", "go.-", "if", "=")
	if got != "(fn (n) (if (go.> n 0) (go.- n 1) 0))" {
		t.Errorf("try on an option: got %s", got)
	}
}

func TestTryIsOnlyForASuccessConstructor(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(variant colour (red int) blue)
		  (def f (fn (c) (try (red x) c (red x))))`, "declares no (success"},
		{`(variant (result T E) (ok T) (err E) (success ok))
		  (def f (fn (r) (try (err x) r (err x))))`, "not the success constructor"},
	} {
		_, err := loadSrc(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want an error saying %q, got %v", c.want, err)
		}
	}
}

// `again` MAY SIT IN A SUCCESS ARM (spec/errors.md §5): the arm binds the
// payload and runs once, now, a binding's tail. Nowhere else in the form: the
// value, the pattern and expect's reason are expressions.
func TestAgainMaySitInASuccessArmOnly(t *testing.T) {
	const ok = `(def f (fn (n) (loop ((i 0)) (>= i n) i else (try (r.ok x) (r.g i) (again (+ i x))))))`
	if _, err := Read(ok); err != nil {
		t.Errorf("again in try's success arm is a binding's tail: %v", err)
	}
	const okExpect = `(def f (fn (n) (loop ((i 0)) (>= i n) i else (expect (r.ok x) (r.g i) "why" (seq (p x) (again (+ i 1)))))))`
	if _, err := Read(okExpect); err != nil {
		t.Errorf("again in expect's success arm, under a seq: %v", err)
	}
	for _, bad := range []string{
		`(def f (fn (n) (loop ((i 0)) (>= i n) i else (try (r.ok x) (again i) x))))`,
		`(def f (fn (n) (loop ((i 0)) (>= i n) i else (expect (r.ok x) (r.g i) (again i) x))))`,
		`(def f (fn (n) (loop ((i 0)) (>= i n) i else (try (r.ok x) (r.g i) (+ 1 (again i))))))`,
	} {
		if _, err := Read(bad); err == nil || !strings.Contains(err.Error(), "again") {
			t.Errorf("want again refused in %s, got %v", bad, err)
		}
	}
}
