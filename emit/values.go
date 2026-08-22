package emit

import (
	"fmt"

	"oroboros/core"
)

// multiValue recognises the NEGATIVE PRODUCT in the residual.
//
// `(values a b)` is reader sugar for `(fn (#k) (#k a b))`, and that is the
// whole representation: a function that answers whichever observation you give
// it. A caller in the same reduction reduces it away entirely, which is why it
// measured 1.01x with zero allocations (product-2026-08-19). What reaches the
// emitter is the case where it did NOT reduce away — a function whose value is
// a selector-taking lambda — and that is exactly what a target with a native
// multiple-return should emit.
//
// The shape is `(fn (k) (k e₁ … eₙ))` with k the operator and nothing else.
// That shape is also, read differently, a genuine higher-order function: "apply
// my argument to these n things". The two readings are the same term, so
// **the signature is what decides**, the way a declared range decides an
// integer's representation. A `sig` with several results asks for the product;
// without one this is an escaping closure and stays refused.
func multiValue(t *core.Term, n int) ([]*core.Term, bool) {
	if t == nil || t.Kind != core.KFn || len(t.Params) != 1 {
		return nil, false
	}
	body := t.Body()
	if body.Kind != core.KApp || body.Op().Kind != core.KName ||
		body.Op().Name != t.Params[0] || len(body.Args()) != n {
		return nil, false
	}
	// The selector must be used ONCE, as the operator. An occurrence anywhere
	// else means the term does something a product cannot.
	for _, a := range body.Args() {
		if core.Occurs(a, t.Params[0]) {
			return nil, false
		}
	}
	return body.Args(), true
}

// multiResultErr is the one message every backend gives when a signature asks
// for several results and the definition does not produce them.
//
// There is no "this target cannot" case. A construct promoted to the LANGUAGE
// works on every target and the compiler finds the implementation; a target
// neither declines it nor declares it. The first attempt at this feature got
// that wrong — it carried a (multi-return "…" "…") declaration and refused on
// Java and windows — and was reverted for it.
func multiResultErr(name string, sig *core.Sig, t *core.Term) error {
	return fmt.Errorf("%s declares %d results but its definition does not produce them.\n"+
		"  A function with several results must reduce to (values e1 … e%d); got %s",
		name, len(sig.Results), len(sig.Results), t)
}
