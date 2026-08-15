package emit

import (
	"fmt"

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
	if goals, ok := obligation(where); ok {
		for _, g := range goals {
			f.assumeLE(g, "assumed "+g.String()+" <= 0")
		}
	}
}

func (r *refiner) walk(t *core.Term, f *facts) error {
	switch t.Kind {
	case core.KInt, core.KFloat, core.KStr, core.KName:
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
	goals, ok := obligation(core.Rename2(p.Where, sub))
	if !ok {
		// Outside the fragment: an opaque atom, propagated rather than decided
		// (refinements.md §3). Sound — it is not assumed true; it simply cannot
		// be discharged here, and the note says so.
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
