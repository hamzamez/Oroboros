package emit

import (
	"fmt"
	"strconv"
	"strings"

	"oroboros/core"
)

// A KEPT DEFINITION'S POSTCONDITION, DERIVED (ADR 0113, p8-research §9).
//
// A kept definition is a triple {P ∧ φ} f {ψ}, and a call assumes ψ(a⃗, r).
// Inlined, a site knew sp(body, Γ), the strongest postcondition under its own
// facts; kept, it knows Γ ∧ ψ. So ψ is taken as strong as the refinement layer
// decides: the greatest subset of a finite set of candidates T that holds at
// every return of the body. That is Houdini (Flanagan and Leino 2001), the
// greatest fixed point on 2^T. Each candidate is a where-mark at every value
// tail, decided with the facts there (a loop's invariants included); a mark
// that holds is assumed beneath it, so a failing candidate is removed and the
// rest decided again, until none fails.

// PostResult is the name a candidate gives the body's component c; Proven
// renders it as the call's `result` or `result#c`.
func PostResult(c int) string { return "#post-r" + strconv.Itoa(c) }

const postDef = "#post:"

// PlantPostUnchecked accepts every candidate without deciding it:
// TestAKeptResultCarriesItsPostcondition must then see a false ψ prove a
// caller's index.
var PlantPostUnchecked bool

// ProvePostconditions decides each candidate, a term over t's parameters and
// PostResult(c) for c < comps, at every value tail of t, a kept definition's
// body after its obligations were discharged. It answers which hold.
func ProvePostconditions(tgt *Target, sig *core.Sig, t *core.Term, comps int, cands []*core.Term) []bool {
	alive := make([]bool, len(cands))
	for i := range alive {
		alive[i] = true
	}
	if PlantPostUnchecked || len(cands) == 0 || t.Kind != core.KFn {
		return alive
	}
	for round := 0; round <= len(cands); round++ {
		mark := func(vs []*core.Term, inner *core.Term) *core.Term {
			sub := map[string]*core.Term{}
			for c, v := range vs {
				sub[PostResult(c)] = v
			}
			for i := len(cands) - 1; i >= 0; i-- {
				if alive[i] {
					inner = &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name(core.RequireWhereName),
						core.Str(postDef + strconv.Itoa(i)), core.Rename2(cands[i], sub), inner}}
				}
			}
			return inner
		}
		body, opaque := markTails(t.Body(), comps, mark)
		if opaque {
			// a tail no rule reads: no candidate is decided there, so none holds
			for i := range alive {
				alive[i] = false
			}
			return alive
		}
		fails := postFails(tgt, sig, core.Fn(t.Params, body))
		removed := false
		for _, d := range fails {
			if i, err := strconv.Atoi(strings.TrimPrefix(d, postDef)); err == nil && strings.HasPrefix(d, postDef) && alive[i] {
				alive[i] = false
				removed = true
			}
		}
		if !removed {
			return alive
		}
	}
	return alive
}

// postFails is DischargeRequires' dry walk over a marked body: the defs of the
// marks that do not follow.
func postFails(tgt *Target, sig *core.Sig, t *core.Term) []string {
	t = Unscope(tgt, sig, t)
	r := &refiner{tgt: tgt, pure: pureAtomsFor(tgt, sig, t), probe: true, requires: &[]requireFail{},
		memo: &refineMemo{houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
	f := newFacts()
	f.pure = r.pure
	if sig != nil && sig.Where != nil {
		assume(f, sig.Where)
	}
	seedFacts(f, t, langFacts)
	_ = r.walk(t, f)
	var out []string
	for _, x := range *r.requires {
		out = append(out, x.raw)
	}
	return out
}

// markTails rewrites each value tail of t, a body whose value has comps
// components, so that its components pass through mark. A tail is what the
// body's value is: an `if`'s arms, a let's body, a call's continuation's body
// (a host call's, a loop's or a sum's, `(E (fn (x…) b))`), a where-mark's
// body, a loop's exits (its `again`s are jumps), and a tuple's components,
// λk. k v⃗. Anything else is a value. opaque is a tail of several components
// that is no tuple, where no candidate can be decided.
func markTails(t *core.Term, comps int, mark func(vs []*core.Term, inner *core.Term) *core.Term) (out *core.Term, opaque bool) {
	var tm func(t *core.Term, inLoop bool) *core.Term
	value := func(vs []*core.Term) *core.Term {
		// BIND EACH COMPONENT, `(let v (fn (x) …))`, the refiner's let: a
		// call's guarantee attaches to its binder there (Lemma 2), so the marks
		// read names and each value is evaluated once
		names := make([]*core.Term, len(vs))
		for c := range vs {
			names[c] = core.Name(fmt.Sprintf("#post-v%d", c))
		}
		var inner *core.Term
		if len(vs) == 1 {
			inner = names[0]
		} else {
			inner = core.Fn([]string{"#post-k"}, &core.Term{Kind: core.KApp, Kids: append([]*core.Term{core.Name("#post-k")}, names...)})
		}
		out := mark(names, inner)
		for c := len(vs) - 1; c >= 0; c-- {
			out = core.App(core.Name("let"), vs[c], core.Fn([]string{names[c].Name}, out))
		}
		return out
	}
	tm = func(t *core.Term, inLoop bool) *core.Term {
		if t == nil {
			return t
		}
		// a tuple, λk. k v⃗
		if comps > 1 && t.Kind == core.KFn && len(t.Params) == 1 {
			if b := t.Body(); b.Kind == core.KApp && b.Op().Kind == core.KName && b.Op().Name == t.Params[0] && len(b.Args()) == comps {
				return value(b.Args())
			}
		}
		if t.Kind != core.KApp {
			if comps == 1 {
				return value([]*core.Term{t})
			}
			opaque = true
			return t
		}
		op := t.Op()
		args := t.Args()
		switch {
		case op.Kind == core.KName && op.Name == "again":
			return t
		case op.Kind == core.KName && op.Name == "if" && len(args) == 3:
			return core.App(op, args[0], tm(args[1], inLoop), tm(args[2], inLoop))
		case core.IsRequireWhere(t):
			return &core.Term{Kind: core.KApp, Kids: []*core.Term{t.Kids[0], t.Kids[1], t.Kids[2], tm(t.Kids[3], inLoop)}}
		case op.Kind == core.KName && op.Name == "let" && len(args) == 2 && args[1].Kind == core.KFn:
			k := args[1]
			return core.App(op, args[0], core.Fn(k.Params, tm(k.Body(), inLoop)))
		case op.Kind == core.KFn && len(op.Params) == len(args):
			return &core.Term{Kind: core.KApp, Kids: append([]*core.Term{core.Fn(op.Params, tm(op.Body(), inLoop))}, args...)}
		case op.Kind == core.KApp && len(args) == 1 && args[0].Kind == core.KFn:
			k := args[0]
			return core.App(op, core.Fn(k.Params, tm(k.Body(), inLoop)))
		case op.Kind == core.KName && op.Name == "loop" && len(args) >= 1 && args[0].Kind == core.KFn:
			body := args[0]
			kids := append([]*core.Term{op, core.Fn(body.Params, tm(body.Body(), true))}, args[1:]...)
			return &core.Term{Kind: core.KApp, Kids: kids}
		}
		if comps == 1 {
			return value([]*core.Term{t})
		}
		opaque = true
		return t
	}
	return tm(t, false), opaque
}

// Proven renders the candidates that hold as one postcondition over the
// call's results: PostResult(c) as `result#c`, or `result` when the call has
// one result (one is the component index it stands for, or −1 for none).
func Proven(cands []*core.Term, holds []bool, comps int, one int) *core.Term {
	sub := map[string]*core.Term{}
	for c := 0; c < comps; c++ {
		if c == one {
			sub[PostResult(c)] = core.Name(core.ResultName)
		} else {
			sub[PostResult(c)] = core.Name(core.ResultComponent(c))
		}
	}
	var out *core.Term
	for i, q := range cands {
		if !holds[i] {
			continue
		}
		if one >= 0 && mentionsOtherComponent(q, comps, one) {
			continue
		}
		out = conjTerm(out, core.Rename2(q, sub))
	}
	return out
}

func mentionsOtherComponent(t *core.Term, comps, one int) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName {
		for c := 0; c < comps; c++ {
			if c != one && t.Name == PostResult(c) {
				return true
			}
		}
	}
	for _, k := range t.Kids {
		if mentionsOtherComponent(k, comps, one) {
			return true
		}
	}
	return false
}
