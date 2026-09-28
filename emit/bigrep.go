package emit

import (
	"fmt"

	"oroboros/core"
)

// ARBITRARY PRECISION, SELECTED — ADR 0019's THIRD ESCAPE.
//
// ADR 0019 says an integer operation the compiler cannot prove stays inside the
// portable window is a compile error, cleared by saying one of three things:
// narrow the range, ask for the trap, or DECLARE A RANGE ABOVE THE WINDOW,
// which promotes that value to arbitrary precision. The refusal in bounded.go
// has been offering that third escape in its own error message since the day it
// was written, and nothing implemented it. This is it.
//
// ═══ WHAT IS DECIDED HERE
//
// A representation, out of a two-point lattice `word ⊑ big` with `big`
// absorbing — precision-by-declaration.md's own proposal, and the reason it
// argued the interval domain does NOT have to go arbitrary-precision: an
// operation on a value that is already exact cannot overflow, so the analysis is
// never asked about it. `emit/interval.go`'s `int64` arithmetic is untouched.
//
// The soundness rule is one line: an operation is an error only if EVERY operand
// is word-represented and its result cannot be proven to fit. A big operation is
// therefore not an operation the window accounting sees at all, which is why
// this needed no change to `record`, to `Unbounded`, or to the report — a big
// primitive's declared result is `big`, `transfer` already returns "not
// checkable" for a result that is not `int`, and the operation simply is not
// counted.
//
// ═══ WHY IT MUST BE BIDIRECTIONAL, WITH FACTORIAL AS THE WITNESS
//
// precision-by-declaration.md named this and it is the whole difficulty:
//
//	(sig fact ((n (int 0 30))) (int 0 (pow 2 110)))
//
// every INPUT is small and the accumulator reaches 30! ≈ 2.65×10³². Nothing
// flowing forward makes `acc` big; the pressure comes from the declared RESULT.
// A forward-only solver passes the entire current corpus and fails on the first
// factorial.
//
// So a name is big if it is in the least set B closed under three rules:
//
//	(S) SUPPLY      — some expression assigned to it is big.
//	(D) DEMAND      — it is returned where a big result is declared, or it is
//	                  assigned to a loop variable that is already big.
//	(P) PRESSURE    — a big operation reads it AND it is not provably inside
//	                  the window.
//
// (S) and (D) are the two directions. (P) is the one that is easy to get wrong
// in either direction, and the gate is exactly right: in `fact`, the counter `i`
// is read by the big multiply and IS provably 1..31, so it stays a machine word
// and is widened at the call — which is what keeps a bignum loop from having a
// bignum loop counter. In `power`, `x` is read by the big multiply and its own
// `(* x x)` is unbounded, so it is promoted, which is the only way that program
// can be right.
//
// The system is monotone over a finite lattice, so the least fixed point is
// reached by iteration and the iteration terminates. Promoting too much costs
// speed; promoting too little is caught by the existing refusal. That is the
// same safety direction as the interval analysis's γ-soundness — containment,
// never tightness — and it is chosen for the same reason.
//
// ═══ WIDENING IS THE ONLY IMPLICIT CONVERSION
//
// `int ⊆ big`, so a word value may be widened where a bignum is wanted and this
// pass inserts `big-of` to do it. The other direction is REFUSED, and that
// refusal is unbounded-rung.md §3 — the promotion is a widening rather than a
// refinement, so it is where a programmer finds out a value became a bignum.
// `core.ValueType` returning `big` is what makes the type checker say so.

var bigOpNames = map[string]bool{
	"big+": true, "big-": true, "big*": true, "big/": true, "big%": true,
	"big<": true, "big<=": true, "big>": true, "big>=": true, "big=": true,
	"big-of": true, "big-of-small": true, "big-str": true,
	"big%-small": true,
}

func isBigOp(name string) bool { return bigOpNames[name] }

// DeclaresBig reports whether a signature mentions a range above the portable
// window, which is the only way a program can ask for arbitrary precision —
// ADR 0019's blast-radius argument, that every source of `big` is a declaration
// somebody wrote.
func DeclaresBig(w core.Word, sig *core.Sig) bool {
	if sig == nil {
		return false
	}
	if w.ValueType(sig.Result) == core.BigType {
		return true
	}
	for _, r := range sig.Results {
		if w.ValueType(r) == core.BigType {
			return true
		}
	}
	for _, sp := range sig.Params {
		if w.ValueType(sp.Type) == core.BigType {
			return true
		}
	}
	return false
}

// MentionsBig reports whether a residual names an arbitrary-precision primitive
// itself, which is the second way a program can ask.
//
// It is not a convenience. Reduction inlines every non-exported call, so a
// helper's declared big result is gone before this pass runs — the same
// structural limit refinements.md §6b records for a `where` and
// indexnarrow-2026-08-27 records for a narrowed parameter. What survives
// inlining is the BOUNDARY, and for a bignum the boundary is `big-str`: a value
// past 2^53 cannot be printed as an `int` on any of the four hosts, so the one
// place a whole program must name arbitrary precision is exactly the one place
// the demand can be read off the residual.
func MentionsBig(w core.Word, t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName && isBigOp(t.Name) {
		return true
	}
	// AND AN ASCRIPTION IS THE SECOND WAY, which is the whole point of it: a
	// declaration reduction inlined away is a demand the residual still carries.
	if t.Kind == core.KApp && t.Op().Kind == core.KName &&
		t.Op().Name == core.AscribeName && ascribedBig(w, t.Args()) {
		return true
	}
	for _, k := range t.Kids {
		if MentionsBig(w, k) {
			return true
		}
	}
	return false
}

// ═══ THE DECLARED BOUND, ENFORCED ON THE HOST'S OWN BIGNUM
//
// A range is SEMANTICS: `(int 0 (pow 2 1300))` says the value is a mathematical
// integer in that interval, which is a fact about the program and true on every
// target. Which storage a target picks for it is `BigRepr`'s business.
//
// But then the two representations have to enforce the same thing. The limb
// rung traps when a value passes its bound, because a fixed width has no choice
// — it would otherwise truncate. The host's bignum is exact whatever the
// declaration says, so left alone it would silently ACCEPT what the limb rung
// refuses, and `(big-repr host)` would stop being a change of storage and
// become a change of ANSWER. That is ADR 0009's rule at the representation
// boundary, and it is the one thing this project refuses.
//
// So every operation that can GROW a value is wrapped. `big-of` is not, and
// that is a proof rather than an omission: a range reaches this rung only by
// being above the portable window, so the bound is at least 54 bits, and a
// machine word is under 2^53 — widening one can never exceed it. Comparisons
// and `big-str` produce no big value at all.
//
// The cost is one `BitLen` per operation and it is paid only by a program that
// declares a bound. The SIGN is part of the set (ADR 0029): `big-fit` enforces
// [0, 2ᵏ) and `big-fit-signed` (−2ᵏ, 2ᵏ), the one set of the whole program, and
// each host's template makes its own check say exactly that. A program declaring `(int 0 +inf)` has nothing to check,
// which is the honest difference between the two declarations and the reason
// they are two.
var bigGrows = map[string]bool{
	"big+": true, "big-": true, "big*": true, "big/": true, "big%": true,
	"big+!": true, "big-!": true, "big*!": true, "big/!": true, "big%!": true,
}

func fitBig(tgt *Target, t *core.Term, bits int, signed bool) (*core.Term, error) {
	if bits == 0 {
		return t, nil // `(int 0 +inf)` — no bound was declared, so none is enforced
	}
	fit := "big-fit"
	if signed {
		fit = "big-fit-signed"
	}
	if _, ok := tgt.Prims[fit]; !ok {
		return nil, fmt.Errorf("this program declares a range above the portable "+
			"window with a FINITE bound, and target %s stores that in its own "+
			"arbitrary-precision integer but declares no `%s` to enforce the "+
			"bound.\n"+
			"  A bound the target cannot check is a declaration that means one\n"+
			"  thing here and another on the fixed-limb rung, so it is refused\n"+
			"  rather than dropped. Declare `%s`, or `(big-repr limbs)`.",
			tgt.Name, fit, fit)
	}
	return fitWalk(t, fit, bits), nil
}

func fitWalk(t *core.Term, fit string, bits int) *core.Term {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		// Rebuilt closed: the binders are de Bruijn indices and the wrap
		// introduces no free names, so nothing can capture.
		return core.FnClosed(t.Params, fitWalk(t.Closed(), fit, bits))
	}
	if t.Kind != core.KApp {
		return t
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = fitWalk(k, fit, bits)
	}
	out := &core.Term{Kind: core.KApp, Kids: kids}
	if op := out.Op(); op.Kind == core.KName && bigGrows[op.Name] {
		return core.App(core.Name(fit), out, core.Int(int64(bits)))
	}
	return out
}

// ascribedBig reads the range an ascription carries and reports whether it is
// wider than the portable window.
//
// The payload is a STRING — the canonical spelling `TypeName` already produces
// for a signature — so this is the same `ExceedsWindow` every other consumer of
// a declared range uses. There is no second notion of a type here.
func ascribedBig(w core.Word, args []*core.Term) bool {
	return len(args) == 2 && args[0] != nil && args[0].Kind == core.KStr &&
		w.ValueType(args[0].Str) == core.BigType // above the word, ℤ included
}

// eraseAscriptions removes every `(the T e)`, leaving `e`.
//
// A lambda is rebuilt CLOSED: its binders are de Bruijn indices and this
// introduces no free name, so nothing can capture — the same reason `fitWalk`
// and the limb rewrite may descend into one without opening it.
func eraseAscriptions(t *core.Term) *core.Term {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		return core.FnClosed(t.Params, eraseAscriptions(t.Closed()))
	}
	if t.Kind != core.KApp {
		return t
	}
	if op := t.Op(); op.Kind == core.KName && op.Name == core.AscribeName &&
		len(t.Args()) == 2 {
		return eraseAscriptions(t.Args()[1])
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = eraseAscriptions(k)
	}
	return &core.Term{Kind: core.KApp, Kids: kids}
}

// BigPlan is how one residual takes the rung above the word: not at all, on
// the host's bignum, chosen on the IR (ir.SelectBig, ADR 0033), or on fixed
// limbs, which PromoteBig still selects on the term. Bits and Signed are the
// program's one set (ADR 0029); Bits 0 enforces nothing.
type BigPlan struct {
	Host, Limbs bool
	Bits        int
	Signed      bool
}

// PlanBig decides the rung, with promoteBig's refusals: a signed program on
// limbs, and a finite bound the host's bignum cannot enforce.
func PlanBig(tgt *Target, sig *core.Sig, t *core.Term, all ...*core.Sig) (BigPlan, error) {
	sigs := append([]*core.Sig{sig}, all...)
	limbs, _, bits := BigRepr(tgt, sigs...)
	if !limbs && !tgt.HasBig() {
		return BigPlan{}, nil
	}
	_, signed, _ := BigHull(tgt.Word, sigs...)
	if bits == 0 && !DeclaresBig(tgt.Word, sig) && !MentionsBig(tgt.Word, t) {
		return BigPlan{}, nil
	}
	if limbs {
		if signed {
			// BigRepr keeps a signed program on limbs only when the target has
			// no bignum to take it to (ADR 0029, decision 4).
			return BigPlan{}, fmt.Errorf("this program declares a range above the word that "+
				"admits NEGATIVE values.\n"+
				"  Target %s stores such a value as fixed limbs, which hold a magnitude:\n"+
				"  they realize [0, 2^k) and nothing signed, and it declares no bignum of\n"+
				"  its own. Refused here rather than trapping on a declared value at run\n"+
				"  time (ADR 0029).", tgt.Name)
		}
		return BigPlan{Limbs: true, Bits: bits}, nil
	}
	if bits > 0 {
		if _, err := fitBig(tgt, nil, bits, signed); err != nil {
			return BigPlan{}, err
		}
	}
	return BigPlan{Host: true, Bits: bits, Signed: signed}, nil
}

// EraseAscriptions removes every `(the T e)`, leaving e.
func EraseAscriptions(t *core.Term) *core.Term { return eraseAscriptions(t) }

// EraseWordAscriptions removes every ascription except one above the word,
// which is a DEMAND the IR's selection reads (ir.SelectBig) and then erases.
// One within the word is erased exactly as before: the decision never read
// it, and reading it would take a declared result for a proven one.
func EraseWordAscriptions(w core.Word, t *core.Term) *core.Term {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		return core.FnClosed(t.Params, EraseWordAscriptions(w, t.Closed()))
	}
	if t.Kind != core.KApp {
		return t
	}
	if op := t.Op(); op.Kind == core.KName && op.Name == core.AscribeName &&
		len(t.Args()) == 2 && !ascribedBig(w, t.Args()) {
		return EraseWordAscriptions(w, t.Args()[1])
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = EraseWordAscriptions(w, k)
	}
	return &core.Term{Kind: core.KApp, Kids: kids}
}
