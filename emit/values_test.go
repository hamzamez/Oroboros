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

// `(values x)` is refused: one value is just the value, so there is exactly one
// spelling for one result and nothing ambiguous reaches a backend.
func TestValuesNeedsTwoOrMore(t *testing.T) {
	_, err := core.Read(`(def f (fn (a) (tuple a)))`)
	if err == nil || !strings.Contains(err.Error(), "two or more") {
		t.Errorf("(tuple x) must be refused, got %v", err)
	}
}

// And a single result declared as a one-element list is the same signature as a
// bare type — one spelling reaching the backends.
func TestOneResultListIsABareType(t *testing.T) {
	forms, err := core.Read(`(sig f ((a int)) int)`)
	if err != nil {
		t.Fatal(err)
	}
	s := forms[0].Sig
	if s.Result != "int" || len(s.Results) != 0 {
		t.Errorf("(int) must normalise to the bare result, got %q / %v", s.Result, s.Results)
	}
}
