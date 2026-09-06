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

// multiPrimCall recognises the ELIMINATION of a host call that gives back
// several results: `((p a…) (fn (x y …) body))`, where `p` is a primitive
// declaring more than one result and the continuation takes exactly that many.
//
// Nothing in the reader or the reducer knows about it, and nothing needs to.
// `(values a b)` is already `(fn (#k) (#k a b))` and is already consumed by
// applying it to a continuation — `((divmod2 n) (fn (a b) (+ a b)))` is a
// differential case. When the producer is a `def`, β performs that application
// and the product vanishes. When the producer is a PRIM, β cannot: the operator
// of the outer application is itself an application, so the redex is stuck and
// the shape reaches the backend intact. That is the whole mechanism.
//
// It is the same shape `emitMapCase` already handles — an operator that is
// legitimately not a name — and for the same reason: a fallible map read IS a
// host call with two results, which maps.md lowered to Go's comma-ok before
// this generalisation existed.
func multiPrimCall(tg *Target, t *core.Term) (Prim, []*core.Term, *core.Term, bool) {
	if t == nil || t.Kind != core.KApp || len(t.Args()) != 1 {
		return Prim{}, nil, nil, false
	}
	op := t.Op()
	if op.Kind != core.KApp || op.Op().Kind != core.KName {
		return Prim{}, nil, nil, false
	}
	p, ok := tg.Prims[op.Op().Name]
	if !ok || len(p.Results) < 2 {
		return Prim{}, nil, nil, false
	}
	k := t.Args()[0]
	if k.Kind != core.KFn || len(k.Params) != len(p.Results) {
		return Prim{}, nil, nil, false
	}
	return p, op.Args(), k, true
}

// multiPrimArityErr is the message when the continuation does not take what the
// primitive gives. It is worth its own error because the alternative diagnostic
// is `application of a non-name`, which says nothing about the actual mistake.
func multiPrimArityErr(name string, want, got int) error {
	return fmt.Errorf("%s gives back %d results and is consumed by a function of %d.\n"+
		"  A host call with several results is eliminated by applying it to a\n"+
		"  continuation taking exactly that many: ((%s …) (fn (r1 … r%d) …)).",
		name, want, got, name, want)
}
