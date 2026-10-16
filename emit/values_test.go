package emit

import (
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

// And a single result declared as a one-element list is the same signature as a
// bare type — one spelling reaching the backends.
func TestOneResultListIsABareType(t *testing.T) {
	forms, err := core.Read(`fn f(a ∈ Int) (Int) { a }`)
	if err != nil {
		t.Fatal(err)
	}
	var s *core.Sig
	for _, f := range forms {
		if f.Sig != nil {
			s = f.Sig
		}
	}
	if s.Result != "int" || len(s.Results) != 0 {
		t.Errorf("(int) must normalise to the bare result, got %q / %v", s.Result, s.Results)
	}
}
