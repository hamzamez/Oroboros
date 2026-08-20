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
// for several results and either the target or the definition cannot deliver.
func multiResultErr(tgt *Target, name string, sig *core.Sig, t *core.Term) error {
	if tgt.MultiReturn == "" {
		return fmt.Errorf("%s declares %d results and target %q has no multiple-return form.\n"+
			"  The program does not cover here — a capability answer, not a syntax error.\n"+
			"  Declare (multi-return \"…\" \"…\") in the target, or return one value.",
			name, len(sig.Results), tgt.Name)
	}
	return fmt.Errorf("%s declares %d results but its definition does not produce them.\n"+
		"  A function with several results must reduce to (values e₁ … e%d); got %s",
		name, len(sig.Results), len(sig.Results), t)
}
