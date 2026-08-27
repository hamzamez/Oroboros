package emit

// LOOP MONOTONICITY — docs/spec/postconditions.md §7.
//
// The fact this exists for: a scanner returns MORE than it was given.
//
//	(def scan-string (fn (src i)
//	  (loop ((j (go.+ i 1)))
//	    (go.>= j (len src))  j
//	    (= (src j) 92)       (again (go.+ j 2))
//	    (= (src j) 34)       (go.+ j 1)
//	    else                 (again (go.+ j 1)))))
//
// `j` starts at `i + 1` and every path only ever adds to it, so the loop's
// value is at least `i + 1`. That is what makes the caller's `i` strictly
// increase, which is the size-change witness the JSON tokeniser's outer loop is
// missing (precision-integers.md §2.1).
//
// It is DERIVED rather than declared, and that is the point. A postcondition on
// an internal definition is redundant — reduction inlines the call, so the
// analysis meets the body at the site with the caller's own values, and
// anything the declaration could say it can work out (postconditions.md §3).
//
// ---------------------------------------------------------------------------
//
// THE RELATION.  For a reference variable `v` and a set S of names already
// known to be at least `v`, define `e ⊒ S`:
//
//	1. x ⊒ S                    if x ∈ S
//	2. (+ a c) ⊒ S, (+ c a) ⊒ S if a ⊒ S and c is a literal ≥ 0
//	3. (- a c) ⊒ S              if a ⊒ S and c is a literal ≤ 0
//	4. (if _ p q) ⊒ S           if p ⊒ S and q ⊒ S
//	5. (let d (fn (x) b)) ⊒ S   if b ⊒ S ∪ {x} when d ⊒ S, else b ⊒ S
//
// LEMMA A.  If every x ∈ S has ⟦x⟧σ ≥ ⟦v⟧σ, and e ⊒ S, then ⟦e⟧σ ≥ ⟦v⟧σ.
//
//	Proof, by structural induction on the derivation.
//	 (1) immediate.
//	 (2) ⟦a + c⟧ = ⟦a⟧ + c ≥ ⟦a⟧ ≥ ⟦v⟧, since c ≥ 0. Symmetric case likewise.
//	 (3) ⟦a - c⟧ = ⟦a⟧ - c ≥ ⟦a⟧ ≥ ⟦v⟧, since c ≤ 0.
//	 (4) whichever branch is evaluated satisfies it by the hypothesis.
//	 (5) if d ⊒ S then ⟦x⟧ = ⟦d⟧ ≥ ⟦v⟧ by the hypothesis, so S ∪ {x} meets the
//	     premise and b's hypothesis applies; otherwise S alone does.  ∎
//
// Rules 2 and 3 use `a + c ≥ a`, which is arithmetic over ℤ and false under
// wrapping. Our integers are exact inside ADR 0012's window and the target's
// outside it — the same caveat every other part of the interval layer carries,
// and the reason none of this is a substitute for a range declaration.
//
// THEOREM (loop monotonicity).  Let L = (loop (fn (v₁…vₘ) B) z₁…zₘ). Say
// position k is NON-DECREASING if for every `again` in B, reached with
// let-environment S ⊇ {vₖ}, the argument aₖ ⊒ S. Then at every iteration
// ⟦vₖ⟧ ≥ ⟦zₖ⟧.
//
//	Proof, by induction on the iteration count n.
//	 n = 0:  vₖ is zₖ.
//	 n → n+1: the next value is ⟦aₖ⟧ for some again. By the hypothesis
//	          ⟦vₖ⟧ ≥ ⟦zₖ⟧, so {vₖ} meets Lemma A's premise, and Lemma A gives
//	          ⟦aₖ⟧ ≥ ⟦vₖ⟧ ≥ ⟦zₖ⟧.  ∎
//
// COROLLARY (exit bound).  If in addition every exit expression e of L
// satisfies e ⊒ {vₖ} in its own environment, then ⟦L⟧ ≥ ⟦zₖ⟧.
//
//	Proof.  An exit is evaluated at some iteration n, where ⟦vₖ⟧ ≥ ⟦zₖ⟧ by the
//	theorem; Lemma A then gives ⟦e⟧ ≥ ⟦vₖ⟧ ≥ ⟦zₖ⟧.  ∎
//
// The corollary is what is used: it turns a loop into a lower bound on the
// VALUE it produces, which is exactly what a caller needs and exactly what an
// interval cannot express when the bound mentions a variable.

import (
	"oroboros/core"
)

// asLet recognises a binding in BOTH spellings it can arrive in.
//
// `(let v (fn (x) b))` is what the reducer leaves when call-by-need declines to
// substitute, and `((fn (x) b) v)` is what the reader's desugaring produces —
// core/read.go turns `(let v k)` into `(k v)`, so a term that has not been
// through β wears the second shape. They are one construct and rule 5 applies
// to either.
func asLet(tgt *Target, t *core.Term) (value *core.Term, lam *core.Term, ok bool) {
	if t == nil || t.Kind != core.KApp {
		return nil, nil, false
	}
	args := t.Args()
	if op := t.Op(); op.Kind == core.KFn && len(op.Params) == 1 && len(args) == 1 {
		return args[0], op, true
	} else if op.Kind == core.KName && len(args) == 2 &&
		args[1].Kind == core.KFn && len(args[1].Params) == 1 {
		if p, known := tgt.Prims[op.Name]; known && p.Kind == "let" {
			return args[0], args[1], true
		}
	}
	return nil, nil, false
}

// atLeast decides `e ⊒ S`. It is the relation above and nothing more: every
// case is one of the five rules, and anything unrecognised is false, because
// refusing is the safe direction.
func atLeast(tgt *Target, e *core.Term, s map[string]bool) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case core.KName:
		return s[e.Name] // rule 1
	case core.KApp:
		// rule 5 — a binding extends S when its value qualifies.
		if v, lam, ok := asLet(tgt, e); ok {
			body, raw, _ := openFresh(lam, map[string]bool{},
				func(x string) string { return x })
			inner := s
			if atLeast(tgt, v, s) {
				inner = cloneSet(s)
				inner[raw[0]] = true
			}
			return atLeast(tgt, body, inner)
		}
		op := e.Op()
		if op.Kind != core.KName {
			return false
		}
		args := e.Args()
		// rule 4 — a conditional is at least v when both branches are.
		if p, known := tgt.Prims[op.Name]; known && p.Kind == "cond" && len(args) == 3 {
			return atLeast(tgt, args[1], s) && atLeast(tgt, args[2], s)
		}
		if len(args) != 2 {
			return false
		}
		switch arithOp(op.Name, 2) {
		case "add": // rule 2
			if args[1].Kind == core.KInt && args[1].Int >= 0 {
				return atLeast(tgt, args[0], s)
			}
			if args[0].Kind == core.KInt && args[0].Int >= 0 {
				return atLeast(tgt, args[1], s)
			}
		case "sub": // rule 3 — subtraction is not commutative
			if args[1].Kind == core.KInt && args[1].Int <= 0 {
				return atLeast(tgt, args[0], s)
			}
		}
	}
	return false
}

func cloneSet(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s)+1)
	for k := range s {
		out[k] = true
	}
	return out
}

// LoopLowerBound is the corollary, applied: the term a loop's value is at least,
// or nil.
//
// It looks for a non-decreasing position whose EXITS are also at least that
// position's variable, and returns that position's initial value. For
// `scan-string` that is `(go.+ i 1)`, and the caller learns its index strictly
// increases without anything being declared.
func LoopLowerBound(tgt *Target, loop *core.Term) *core.Term {
	if loop == nil || loop.Kind != core.KApp || loop.Op().Kind != core.KName {
		return nil
	}
	if p, known := tgt.Prims[loop.Op().Name]; !known || p.Kind != "iterate" {
		return nil
	}
	args := loop.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return nil
	}
	lam, inits := args[0], args[1:]
	if len(lam.Params) != len(inits) {
		return nil
	}
	body, raw, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
	for k := range raw {
		if monotoneAt(tgt, body, raw, k) {
			return inits[k]
		}
	}
	return nil
}

// monotoneAt decides both halves of the corollary for one position: that every
// `again` does not decrease it, and that every EXIT is at least it.
//
// The two are checked in one walk because both are properties of the same clause
// chain, and because the let-environment they need is the same one.
func monotoneAt(tgt *Target, body *core.Term, raw []string, k int) bool {
	base := map[string]bool{raw[k]: true}
	ok := true
	sawExit := false
	var walk func(t *core.Term, s map[string]bool, tail bool)
	walk = func(t *core.Term, s map[string]bool, tail bool) {
		if !ok || t == nil {
			return
		}
		if v, lam, isLet := asLet(tgt, t); isLet {
			lb, lraw, _ := openFresh(lam, map[string]bool{},
				func(x string) string { return x })
			inner := s
			if atLeast(tgt, v, s) {
				inner = cloneSet(s)
				inner[lraw[0]] = true
			}
			walk(lb, inner, tail)
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			args := t.Args()
			if t.Op().Name == "again" {
				if k >= len(args) || !atLeast(tgt, args[k], s) {
					ok = false
				}
				return
			}
			// A clause chain is nested conditionals; each branch is still a
			// tail position, and only a tail position is an exit.
			if p, known := tgt.Prims[t.Op().Name]; known &&
				p.Kind == "cond" && len(args) == 3 && tail {
				walk(args[1], s, true)
				walk(args[2], s, true)
				return
			}
		}
		if tail {
			// An EXIT. The corollary needs every one of them to be at least the
			// position's variable, or the loop's value is not bounded by its
			// initial value.
			sawExit = true
			if !atLeast(tgt, t, s) {
				ok = false
			}
			return
		}
	}
	walk(body, base, true)
	return ok && sawExit
}

// isLoopTerm reports whether a term is a `loop`, without the caller needing to
// know which name a target spells it with.
func isLoopTerm(tgt *Target, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return false
	}
	p, known := tgt.Prims[t.Op().Name]
	return known && p.Kind == "iterate"
}

// selfPlus reports the constant `c` when `e` is `self + c` or `self - (-c)`,
// which is the shape a step has to be in for the caller to use a lower bound.
func selfPlus(tgt *Target, e *core.Term, self string) (int64, bool) {
	if e == nil {
		return 0, false
	}
	if e.Kind == core.KName && e.Name == self {
		return 0, true
	}
	if e.Kind != core.KApp || e.Op().Kind != core.KName || len(e.Args()) != 2 {
		return 0, false
	}
	a, b := e.Args()[0], e.Args()[1]
	switch arithOp(e.Op().Name, 2) {
	case "add":
		if a.Kind == core.KName && a.Name == self && b.Kind == core.KInt {
			return b.Int, true
		}
		if b.Kind == core.KName && b.Name == self && a.Kind == core.KInt {
			return a.Int, true
		}
	case "sub":
		if a.Kind == core.KName && a.Name == self && b.Kind == core.KInt {
			return -b.Int, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------- extremum
//
// A RUNNING EXTREMUM — `mx = max(mx, sp+1)` — and why the fixpoint could not
// bound one.
//
//	(if (go.> (go.+ sp 1) mx) (go.+ sp 1) mx)
//
// The `else` branch is `mx` itself, so in the fixpoint `next[mx] ⊇ cur[mx]`
// ALWAYS. Widening throws the bound to infinity, and the narrowing phase — which
// only accepts a value CONTAINED in the previous one — can never take it back,
// because the pass-through pins it. Every one of the fifteen operations the JSON
// tokeniser could not bound traced to this (monotone-2026-08-27 §5).
//
// DEFINITION. Position k is SELF-CONTAINED if every `again` argument aₖ is built
// from vₖ and expressions not mentioning vₖ, using only `if` — whose CONDITION
// may mention vₖ, since a condition produces no value — and `let` whose bound
// value is vₖ-free. Its UPDATE SET U(aₖ) is the vₖ-free leaves.
//
// THEOREM (reachable set).  For a self-contained position, at every iteration
// ⟦vₖ⟧ ∈ {⟦zₖ⟧} ∪ ⋃U.
//
//	Proof, by induction on the iteration count n.
//	 n = 0:  ⟦vₖ⟧ = ⟦zₖ⟧.
//	 n → n+1: the new value is ⟦aₖ⟧, which by the structure of aₖ is either
//	          ⟦vₖ⟧ — in the set by the induction hypothesis — or ⟦e⟧ for some
//	          e ∈ U.  ∎
//
// COROLLARY.  hull({zₖ} ∪ U) is sound, EXACT in one step, and needs no
// widening. And because it no longer mentions cur[k], it narrows as the other
// variables narrow — which is what the pass-through was preventing.
//
// The recurrence is not `v' = f(v)` where f can grow. It is `v' ∈ {v} ∪ U`, so
// the reachable set is closed after one step and a fixpoint was never needed.

// selfContained decides the definition above, syntactically and with no
// evaluation, so the caller can choose which way to evaluate without
// double-counting operations.
func selfContained(tgt *Target, a *core.Term, self string) bool {
	if a == nil {
		return false
	}
	if a.Kind == core.KName && a.Name == self {
		return true // the pass-through: contributes nothing new
	}
	if !mentionsName(a, self) {
		return true // a leaf of the update set
	}
	if v, lam, ok := asLet(tgt, a); ok {
		if mentionsName(v, self) {
			return false
		}
		body, _, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
		return selfContained(tgt, body, self)
	}
	if a.Kind == core.KApp && a.Op().Kind == core.KName {
		if p, known := tgt.Prims[a.Op().Name]; known && p.Kind == "cond" && len(a.Args()) == 3 {
			// The CONDITION may mention self freely — it produces no value.
			return selfContained(tgt, a.Args()[1], self) &&
				selfContained(tgt, a.Args()[2], self)
		}
	}
	return false
}

// mentionsName reports whether a term refers to a name.
func mentionsName(t *core.Term, name string) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName {
		return t.Name == name
	}
	for _, k := range t.Kids {
		if mentionsName(k, name) {
			return true
		}
	}
	return false
}

// loopExitsFit reports whether every value a loop can produce is one the
// enclosing method's index type can hold.
//
// A loop's value is one of its EXIT expressions — the tail positions of its
// clause chain that are not `again`. Its own variables count as acceptable
// sources: they are narrowed by the same whole-method gate that is asking this
// question, so either all of them are held in the host's index type or none is.
func loopExitsFit(tgt *Target, loop *core.Term, raw []string) bool {
	args := loop.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return false
	}
	body, lraw, _ := openFresh(args[0], map[string]bool{}, func(x string) string { return x })
	for _, z := range args[1:] {
		if !fitsIndexSource(tgt, z, raw) {
			return false
		}
	}
	inner := append(append([]string{}, raw...), lraw...)
	ok := true
	var walk func(t *core.Term, tail bool)
	walk = func(t *core.Term, tail bool) {
		if !ok || t == nil {
			return
		}
		if _, lam, isLet := asLet(tgt, t); isLet {
			lb, lr, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
			inner = append(inner, lr...)
			walk(lb, tail)
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if t.Op().Name == "again" {
				return // a back edge is not a value
			}
			if p, known := tgt.Prims[t.Op().Name]; known &&
				p.Kind == "cond" && len(t.Args()) == 3 && tail {
				walk(t.Args()[1], true)
				walk(t.Args()[2], true)
				return
			}
		}
		if tail && !fitsIndexSource(tgt, t, inner) {
			ok = false
		}
	}
	walk(body, true)
	return ok
}
