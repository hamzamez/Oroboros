package core

import (
	"strings"
	"testing"
)

// `expect` IS TOTAL (spec/errors.md §8): it continues on the success
// constructor and crashes with its reason on every other, through `abandon`,
// crash : R → 0. Like `try` it expands to an exhaustive `case`, so a known
// constructor reduces it away and a dynamic one leaves the host's own test.

func TestExpectContinuesOnSuccessAndCrashesOtherwise(t *testing.T) {
	got := reduceWith(t, resultDecl+`
		(def f (fn (n) (expect (ok h) (half n) "odd" (go.- h 1))))`,
		"f", "go.>", "go.%", "go./", "go.-", "if", "=", "!abandon")
	want := `(fn (n) (if (go.> (go.% n 2) 0) (abandon "odd") (go.- (go./ n 2) 1)))`
	if got != want {
		t.Errorf("expect:\n got  %s\n want %s", got, want)
	}
}

// Left identity holds for expect as for try: a known success is the body.
func TestExpectOnAKnownSuccessIsItsBody(t *testing.T) {
	got := reduceWith(t, resultDecl+`
		(def f (fn (n) (expect (ok x) (ok n) "never" (go.- x 1))))`,
		"f", "go.-", "if", "=", "!abandon")
	if got != "(fn (n) (go.- n 1))" {
		t.Errorf("got %s", got)
	}
}

func TestExpectIsOnlyForASuccessConstructor(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(variant colour (red int) blue)
		  (def f (fn (c) (expect (red x) c "why" x)))`, "declares no (success"},
		{`(variant (result T E) (ok T) (err E) (success ok))
		  (def f (fn (r) (expect (err x) r "why" x)))`, "not the success constructor"},
		{`(variant (result T E) (ok T) (err E) (success ok))
		  (def f (fn (r) (expect (ok x) r x)))`, "a reason"},
	} {
		_, err := loadSrc(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want a refusal mentioning %q, got %v", c.want, err)
		}
	}
}

// THE CRASH ABSORBS ITS CONTEXT: 0 is initial, so k ∘ abandon = abandon. Each
// row is a strict position, where the crash is the application's value, or a
// lazy one, where it stays.
func TestTheCrashAbsorbsItsContext(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		// a primitive's argument
		{`(go.- 1 (abandon "x"))`, `(abandon "x")`},
		// an `if`'s condition is strict...
		{`(if (go.> (abandon "x") 0) 1 2)`, `(abandon "x")`},
		// ...its branches are not
		{`(if (go.> n 0) (abandon "x") 2)`, `(if (go.> n 0) (abandon "x") 2)`},
		// a binding's value: the continuation never runs
		{`(let x (abandon "x") (go.- x 1))`, `(abandon "x")`},
		// an effect before the crash stays, in order; a pure term is dropped
		{`(go.f (!go.print n) (go.- n 1) (abandon "x"))`, ``},
		// one after it never happens
		{`(go.f (abandon "x") (!go.print n))`, `(abandon "x")`},
		// a λ is a value and is not run
		{`(go.each (fn (i) (abandon "x")))`, `(go.each (fn (i) (abandon "x")))`},
	} {
		body := strings.ReplaceAll(c.body, "!go.print", "go.print")
		got := reduceWith(t, `(use go) (def f (fn (n) `+body+`))`,
			"f", "go.>", "go.-", "go.f", "go.each", "if", "!go.print", "!abandon")
		if c.want == "" {
			// the effect is bound, then the crash; the pure (go.- n 1) is gone
			if !strings.HasPrefix(got, "(fn (n) (let (go.print n) (fn (") ||
				!strings.HasSuffix(got, `(abandon "x"))))`) || strings.Contains(got, "go.-") {
				t.Errorf("%s: the print must stay before the crash, the pure term go: %s", c.body, got)
			}
			continue
		}
		if want := "(fn (n) " + c.want + ")"; got != want {
			t.Errorf("%s:\n got  %s\n want %s", c.body, got, want)
		}
	}
}
