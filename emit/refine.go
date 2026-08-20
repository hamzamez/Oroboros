package emit

import (
	"fmt"
	"strings"

	"oroboros/core"
)

// Refinement checking, per docs/spec/refinements.md.
//
// This closes the first of the two holes shaped like a refinement: `aindex` is
// Tier 1 only WITHIN BOUNDS, and outside them Go panics, Java throws, and
// JavaScript silently returns `undefined` (primitives.md §2). Until now that
// obligation was documented and unchecked.
//
// It is a correctness deliverable, not a speed one. The bounds-check
// PERFORMANCE win was collected as an emitter pattern with no types at all
// (bce-2026-08-15.md), and types-direction §2.2 was explicit that a type system
// must not be justified by cases the emitter can already see.

type refiner struct {
	tgt   *Target
	notes []string // refinements propagated rather than proven
}

// Refine discharges every refinement obligation in a residual.
//
// `sig` supplies the assumptions: a definition may assume its own `where`, and
// that is how a precondition moves to the caller.
func Refine(tgt *Target, what string, sig *core.Sig, t *core.Term) ([]string, error) {
	r := &refiner{tgt: tgt}
	f := newFacts()

	if t.Kind == core.KFn && sig != nil {
		for i, p := range t.Params {
			if i < len(sig.Params) && sig.Params[i].Name != "" && sig.Params[i].Name != p {
				// The signature names parameters independently of the
				// definition; bind the signature's name to the definition's.
				f.assumeEQ(sig.Params[i].Name, variable(p))
			}
		}
		if sig.Where != nil {
			assume(f, sig.Where)
		}
	}
	if err := r.walk(t, f); err != nil {
		return r.notes, fmt.Errorf("%s: %w", what, err)
	}
	return r.notes, nil
}

// assume records a `where` clause as facts. An EQUALITY becomes a
// substitution rather than two inequalities, which is what lets
// `(int.eq (alen p) (alen q))` discharge an obligation about q from a fact
// about p — the two-array case every real program has.
func assume(f *facts, where *core.Term) {
	if where.Kind == core.KApp && where.Op().Kind == core.KName &&
		isOp(where.Op().Name, "and") && len(where.Args()) == 2 {
		assume(f, where.Args()[0])
		assume(f, where.Args()[1])
		return
	}
	if where.Kind == core.KApp && where.Op().Kind == core.KName &&
		isOp(where.Op().Name, "eq") && len(where.Args()) == 2 {
		a, ok1 := asLinear(where.Args()[0])
		b, ok2 := asLinear(where.Args()[1])
		if ok1 && ok2 {
			if name, ok := isVar(b); ok {
				f.assumeEQ(name, a)
				return
			}
			if name, ok := isVar(a); ok {
				f.assumeEQ(name, b)
				return
			}
		}
	}
	if k, ok := neKey(where); ok {
		f.assumeOpaque(k)
		return
	}
	if goals, ok := obligation(where); ok {
		for _, g := range goals {
			f.assumeLE(g, "assumed "+g.String()+" <= 0")
		}
		return
	}
	// Outside the fragment. Kept, not dropped: refinements.md §3 says an opaque
	// term is propagated and matched by name, and dropping it also made the
	// diagnostic claim nothing was known when the program had declared a
	// `where`.
	f.assumeOpaque(where.String())
}

func (r *refiner) walk(t *core.Term, f *facts) error {
	switch t.Kind {
	case core.KInt, core.KFloat, core.KStr, core.KBool, core.KName:
		return nil
	case core.KFn:
		return r.walk(t.Body(), f)
	}

	op := t.Op()
	if op.Kind != core.KName {
		return nil
	}
	args := t.Args()
	p, known := r.tgt.Prims[op.Name]

	// A length is never negative on any target — free, and worth having.
	if known && (isOp(op.Name, "alen") || isOp(op.Name, "slen")) && len(args) == 1 {
		f = f.clone()
		v := variable(lengthVar(op.Name, args[0]))
		f.assumeLE(constant(0).addScaled(v, -1), "0 <= "+v.String())
	}

	if known {
		switch p.Kind {
		case "loop":
			return r.loopLike(args, 1, 2, 2, f)
		case "build":
			return r.loopLike(args, 0, 1, 1, f)
		case "loop2":
			// Two steps and a finisher sharing one index. Written out rather
			// than reusing loopLike, because that walks every non-step
			// argument — which for loop2 means walking the OTHER step with no
			// loop facts in scope, and losing them.
			if len(args) == 6 {
				for _, i := range []int{0, 1, 2} {
					if err := r.walk(args[i], f); err != nil {
						return err
					}
				}
				for _, at := range []int{3, 4} {
					step := args[at]
					if step.Kind != core.KFn || len(step.Params) == 0 {
						continue
					}
					if err := r.walk(step.Body(), r.bind(f, step, args[2])); err != nil {
						return err
					}
				}
				return r.walk(args[5], f)
			}
		case "let":
			return r.let(args, f)
		case "iterate":
			return r.iterate(args, f)
		case "cond":
			// A PLAIN `if` assumes its guard too. There was no case for this at
			// all: the other half of Hoare logic ran only inside a loop's
			// clause chain, so `(if (== b 0) 0 (/ a b))` could not discharge
			// the divisor's precondition even though the else-branch says
			// exactly what is needed (integers.md §5).
			return r.clauses(t, f)
		}
	}

	// Discharge this primitive's own refinement before descending.
	if known && p.Where != nil {
		if err := r.discharge(op.Name, p, args, f); err != nil {
			return err
		}
	}
	for _, a := range args {
		if err := r.walk(a, f); err != nil {
			return err
		}
	}
	return nil
}

// loopLike collects `0 <= i` and `i < count` for a bound index, which is where
// almost every fact in a real program comes from.
func (r *refiner) loopLike(args []*core.Term, countAt, stepAt, idxAt int, f *facts) error {
	for i, a := range args {
		if i == stepAt {
			continue
		}
		if err := r.walk(a, f); err != nil {
			return err
		}
	}
	step := args[stepAt]
	if step.Kind != core.KFn || len(step.Params) < 1 {
		return nil
	}
	return r.walk(step.Body(), r.bind(f, step, args[countAt]))
}

// bind adds `0 <= i` and `i < count` for a step function's index, which is
// where almost every fact in a real program comes from.
func (r *refiner) bind(f *facts, step *core.Term, count *core.Term) *facts {
	idx := step.Params[len(step.Params)-1]
	inner := f.clone()
	i := variable(idx)
	inner.assumeLE(constant(0).addScaled(i, -1), "0 <= "+idx)
	if n, ok := asLinear(count); ok {
		// i < n  ⟶  i - n + 1 <= 0
		inner.assumeLE(i.addScaled(n, -1).addScaled(constant(1), 1), idx+" < "+n.String())
	}
	return inner
}

func (r *refiner) let(args []*core.Term, f *facts) error {
	if len(args) != 2 {
		return nil
	}
	if err := r.walk(args[0], f); err != nil {
		return err
	}
	k := args[1]
	if k.Kind != core.KFn || len(k.Params) != 1 {
		return nil
	}
	inner := f.clone()
	if e, ok := asLinear(args[0]); ok {
		inner.assumeEQ(k.Params[0], e)
	}
	return r.walk(k.Body(), inner)
}

// discharge proves a primitive's `where` at this call site, with the arguments
// substituted for its parameter names.
func (r *refiner) discharge(name string, p Prim, args []*core.Term, f *facts) error {
	sub := map[string]*core.Term{}
	for i, n := range p.Names {
		if n != "" && i < len(args) {
			sub[n] = args[i]
		}
	}
	want := core.Rename2(p.Where, sub)
	// A DISEQUALITY is a disjunction, so the conjunctive fragment cannot hold
	// the goal — but it can hold each side. `d ≠ 0` is `d < 0 ∨ d > 0`, and
	// proving either proves it (integers.md §5).
	if lo, hi, isNe := disequality(want); isNe {
		if f.entailsEither(lo, hi) {
			return nil
		}
		if k, ok := neKey(want); ok && f.entailsOpaque(k) {
			return nil
		}
		return fmt.Errorf("%s requires %s, which does not follow\n  known: %s",
			name, want, f.known())
	}
	goals, ok := obligation(want)
	if !ok {
		// Outside the fragment: an opaque atom (refinements.md §3). It can be
		// discharged only by an assumption that is the SAME term; otherwise it
		// is propagated, never assumed, and the note says so.
		if f.entailsOpaque(want.String()) {
			return nil
		}
		r.notes = append(r.notes, fmt.Sprintf("%s: refinement propagated, not proven", name))
		return nil
	}
	for _, g := range goals {
		if !f.entails(g) {
			return fmt.Errorf("%s requires %s <= 0, which does not follow\n  known: %s",
				name, g.String(), f.known())
		}
	}
	return nil
}

// iterate collects facts from a loop's guards — docs/spec/iteration.md §6.
//
// This is the half of the design that GAINS over fold-range. A fold's bound is
// implied by the primitive, and the checker has to know the convention; a
// loop's is written down as a guard, so inside a clause the guard simply holds.
// Ordinary Hoare logic, and the loop variables are bound to their initial
// values on entry, which is what makes `(int.lt i (alen a))` usable at all.
func (r *refiner) iterate(args []*core.Term, f *facts) error {
	if len(args) < 2 || args[0].Kind != core.KFn {
		return nil
	}
	lam := args[0]
	inits := args[1:]
	for _, z := range inits {
		if err := r.walk(z, f); err != nil {
			return err
		}
	}
	g := f.clone()
	// On entry each variable equals its initial value. That is sound for the
	// FIRST iteration only, so it is deliberately not assumed: what is assumed
	// is only what every iteration guarantees — the guard of the clause being
	// entered, below — plus 0 <= i for any variable whose every `again`
	// argument is itself plus a non-negative literal.
	for i, n := range lam.Params {
		if i < len(inits) {
			if e, ok := asLinear(inits[i]); ok {
				if c, isConst := e.constantValue(); isConst && c >= 0 && nonDecreasing(lam.Body(), i, n) {
					g.assumeLE(constant(0).addScaled(variable(n), -1), "0 <= "+n)
				}
			}
		}
	}
	return r.clauses(lam.Body(), g)
}

// clauses walks the if-chain, assuming each guard inside its own branch.
func (r *refiner) clauses(t *core.Term, f *facts) error {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := r.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			args := t.Args()
			if err := r.walk(args[0], f); err != nil {
				return err
			}
			taken := f.clone()
			assume(taken, args[0])
			if err := r.clauses(args[1], taken); err != nil {
				return err
			}
			// The other half of Hoare logic, and the half that matters here:
			// reaching a later clause means every earlier guard was FALSE. That
			// is where `i < alen a` comes from in a search whose first clause is
			// `(int.ge i (alen a))`.
			missed := f.clone()
			if n := negate(args[0]); n != nil {
				assume(missed, n)
			}
			return r.clauses(args[2], missed)
		}
	}
	return r.walk(t, f)
}

// nonDecreasing reports whether loop variable i is only ever advanced by a
// non-negative amount — `(again … (int.add i k) …)` with k >= 0, or unchanged.
// That is what licenses `0 <= i` from a non-negative initial value.
func nonDecreasing(t *core.Term, i int, name string) bool {
	ok := true
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil || !ok {
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "again" {
			as := t.Args()
			if i >= len(as) {
				ok = false
				return
			}
			a := as[i]
			if a.Kind == core.KName && a.Name == name {
				return // unchanged
			}
			if a.Kind == core.KApp && a.Op().Kind == core.KName && isOp(a.Op().Name, "add") &&
				len(a.Args()) == 2 && a.Args()[0].Kind == core.KName && a.Args()[0].Name == name {
				if k, isLit := asLinear(a.Args()[1]); isLit {
					if c, isConst := k.constantValue(); isConst && c >= 0 {
						return
					}
				}
			}
			ok = false
			return
		}
		switch t.Kind {
		case core.KFn:
			walk(t.Body())
		case core.KApp:
			for _, k := range t.Kids {
				walk(k)
			}
		}
	}
	walk(t)
	return ok
}

// negate turns a comparison into its opposite, or reports that it cannot.
// Only the comparison atoms are negated: `not (and p q)` is a disjunction and
// the fragment has none, so it is dropped rather than approximated.
func negate(t *core.Term) *core.Term {
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return nil
	}
	name := t.Op().Name
	pre := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		pre, name = name[:i+1], name[i+1:]
	}
	// Resolve the target's own spelling first — `go.<` and `num/int.lt` are the
	// same relation, and without this negation worked on the portable layer and
	// silently did nothing on every native target.
	if a, ok := opAlias[name]; ok {
		name = a
	}
	opp := map[string]string{
		"lt": "ge", "ge": "lt", "le": "gt", "gt": "le", "eq": "ne", "ne": "eq",
	}[name]
	if opp == "" {
		return nil
	}
	// The CANONICAL spelling, not the target's. This term is only ever consumed
	// by the analysis — `assume` and `obligation` resolve it through the same
	// alias table — and it never reaches an emitter, so it does not matter that
	// no target declares a primitive called `go.ge`.
	return &core.Term{Kind: core.KApp, Kids: []*core.Term{
		core.Name(pre + opp), t.Args()[0], t.Args()[1]}}
}
