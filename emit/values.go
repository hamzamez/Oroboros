package emit

import (
	"fmt"
	"strings"

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

// TOTALISATION, AND IT IS WHY A HOST CALL THAT CAN FAIL NEEDS NOTHING NEW.
//
// A host offers a PARTIAL function `f : A ⇀ B`, and partiality is the whole
// content of "it can fail". The standard correspondence is that partial maps
// `A ⇀ B` are exactly total maps `A → B + 1`, and with information on the
// failure `A → B + E` — a COPRODUCT, which sums.md already has and which
// `(values b e)` already carries across a boundary.
//
// So the TYPE of a fallible call is host-independent and the language already
// has it. What varies is the host's MECHANISM for saying which summand you got:
//
//	Go           a second return value          `B × E` plus a convention
//	JavaScript   throw                          a partial map and an escape
//	Java         throw, checked                 the same, with E in the types
//	windows      a sentinel plus GetLastError   `B + 1` niche-encoded
//
// NO HOST GIVES THE COPRODUCT. Go's `(T, error)` is a PRODUCT with a discipline,
// and `(prim err-nil ((e error)) bool expr "%s == nil")` is the discriminator
// that totalises it — so this project has been totalising on Go since the day it
// could open a file and never called it that.
//
// It follows that a target declares two things and the compiler learns nothing
// about exceptions: **the CALL that produces the pair, and the DISCRIMINATOR
// that reads it.** The first is a multi-result prim, which is a language
// construct (values.md) — so a host call with several results working on ONE
// backend of four was never a missing feature. It was the same incoherence
// values.md was reverted for: a construct in the core that most targets decline.
//
// multiPrimDests reports the destination holes a template names — `%r0`, `%r1`,
// … — and how many.
//
// A MULTI-RESULT TEMPLATE IS ONE OF TWO SHAPES, and the two are the two host
// shapes rather than two mechanisms:
//
//	the call IS the tuple      `os.ReadFile(%s)`         — the emitter assigns
//	the call ASSIGNS the tuple `try { %r0 = … } catch`   — the template assigns
//
// The second subsumes the first and exists because a host that signals failure
// out of band cannot be an expression yielding two values; it has to be given
// somewhere to put them. `%r` is `%r0` at arity one, so this is the existing
// result hole at the arity the call actually has.
func multiPrimDests(form string, n int) bool {
	for i := 0; i < n; i++ {
		if !strings.Contains(form, fmt.Sprintf("%%r%d", i)) {
			return false
		}
	}
	return n > 0
}

// fillDests replaces `%r0`…`%rn-1` with the names the emitter chose.
//
// IT RUNS BEFORE `fill`, AND THE ORDER IS NOT A PREFERENCE. `fill` is
// `fmt.Sprintf`, so `%r` reaches it as an unknown verb and comes back
// `%!r(MISSING)` — the whole template, silently, as a string. Running the
// destinations first leaves only `%s` for Sprintf to see. It is also the safe
// order for a different reason: an argument's emitted value is arbitrary text
// and could contain `%r0` (a string literal can), where a destination is always
// an identifier the emitter just made.
func fillDests(form string, dests []string) string {
	for i, d := range dests {
		form = strings.ReplaceAll(form, fmt.Sprintf("%%r%d", i), d)
	}
	return form
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
