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

	// bound is every name introduced by a binder. It is what tells a TABLE
	// from anything else in operator position: `(a i)` is an indexing when `a`
	// is a local, and `(again …)` is a jump. Without it `again` looked like a
	// table, because it is not a declared primitive either — found by the
	// existing refinement tests the moment indexing became application.
	bound map[string]bool
}

// Refine discharges every refinement obligation in a residual.
//
// `sig` supplies the assumptions: a definition may assume its own `where`, and
// that is how a precondition moves to the caller.
func Refine(tgt *Target, what string, sig *core.Sig, t *core.Term) ([]string, error) {
	r := &refiner{tgt: tgt}
	f := newFacts()

	if t.Kind == core.KFn && sig != nil {
		// The signature names parameters independently of the definition, so
		// the `where` is RENAMED into the definition's names before it is
		// assumed — not merely accompanied by an equality between them.
		//
		// The equality is not enough, and the reason is worth stating: a length
		// is an OPAQUE VARIABLE in the linear fragment, spelled `go.len(p)`,
		// and substituting `p → a` cannot reach inside that string. So
		// `(where (== (len p) (len q)))` on a definition written `(fn (a b) …)`
		// discharged nothing. It never showed up while the gauntlet's `dot`
		// used the same names in both places, and appeared the moment one
		// program did not.
		sub := map[string]*core.Term{}
		for i, name := range t.Params {
			if i < len(sig.Params) && sig.Params[i].Name != "" && sig.Params[i].Name != name {
				sub[sig.Params[i].Name] = core.Name(name)
				f.assumeEQ(sig.Params[i].Name, variable(name))
			}
		}
		if sig.Where != nil {
			assume(f, core.Rename2(sig.Where, sub))
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
// erasedAnd matches a conjunction as the reader leaves it: `(if p q false)`.
//
// Target-free on purpose. The connectives are erased by the READER, which emits
// the language's own `if`, so a `where` written by a programmer always carries
// that name — there is no host spelling to look up and no target to thread in.
//
// `or` is deliberately NOT matched: a disjunction cannot be assumed as facts
// without a case split, and the fragment is conjunctive. Falling through
// assumes nothing, which is the conservative direction.
//
// `(if p false true)` — `not` — is not matched either, its third argument being
// `true`.
func erasedAnd(t *core.Term) (*core.Term, *core.Term, bool) {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return nil, nil, false
	}
	if !isOp(t.Op().Name, "if") {
		return nil, nil, false
	}
	a := t.Args()
	if len(a) != 3 || a[2].Kind != core.KBool || a[2].IsTrue() {
		return nil, nil, false
	}
	return a[0], a[1], true
}

func assume(f *facts, where *core.Term) {
	// A CONJUNCTION IS ASSUMED CONJUNCT BY CONJUNCT, and it has to be matched in
	// its ERASED form.
	//
	// `and` does not survive the reader (ADR 0017): `(and a b)` is `(if a b
	// false)` by the time anything here sees it, so the named match below never
	// fires and this branch was doing the work alone — except it was not there.
	//
	// What that cost was NON-MONOTONIC and is the worst shape a prover can
	// have: `(where (!= b 0))` discharged a division and
	// `(where (and (!= b 0) (<= 0 a)))` did NOT. Adding a TRUE fact lost the
	// proof. The reason is that a conjunction only reached the solver through
	// `obligation`, which is all-or-nothing — a disequality is outside the
	// linear fragment, so one `!=` conjunct threw away every conjunct including
	// itself.
	//
	// Assuming each side separately lets each be taken by whichever mechanism
	// fits it: `!=` as an opaque atom, a linear bound as a linear bound.
	if a, b, ok := erasedAnd(where); ok {
		assume(f, a)
		assume(f, b)
		return
	}
	// The NAMED form, for a target that declares its own `and`. None of the
	// four does — declaring a boolean name is an error (ADR 0017) — so this is
	// unreachable today and is kept only so that the rule reads completely.
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
	// A SQUARE is outside the linear fragment, but it entails a linear fact,
	// and over the integers it entails one unconditionally: `x <= x*x` for
	// every x. For x >= 1 the square dominates; for x <= 0 the square is
	// non-negative and x is not; and 0 <= 0. So from `x*x < e` we get
	// `x <= x*x <= e-1`, hence `x < e`.
	//
	// This is `narrowSquare`'s insight (emit/interval.go) arriving in the
	// decision procedure, and it is what a sieve needs: the outer loop is
	// bounded by `i*i < n` and indexes an array of length n, so `i < n` is the
	// bounds proof and there is no other route to it.
	if e, ok := squareBound(where); ok {
		f.assumeLE(e, "assumed "+e.String()+" <= 0 (x <= x*x)")
	}
	// Outside the fragment. Kept, not dropped: refinements.md §3 says an opaque
	// term is propagated and matched by name, and dropping it also made the
	// diagnostic claim nothing was known when the program had declared a
	// `where`.
	f.assumeOpaque(where.String())
}

// squareBound reads `(< (* x x) e)` or `(<= (* x x) e)` and returns the linear
// goal for `x < e` (respectively `x <= e`), using `x <= x*x`. The comparison
// may be written either way round.
func squareBound(t *core.Term) (*linear, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return nil, false
	}
	op := t.Op().Name
	sq, other, strict := t.Args()[0], t.Args()[1], false
	switch {
	case isOp(op, "lt"):
		strict = true
	case isOp(op, "le"):
	case isOp(op, "gt"):
		sq, other, strict = other, sq, true
	case isOp(op, "ge"):
		sq, other = other, sq
	default:
		return nil, false
	}
	x, ok := isSquare(sq)
	if !ok {
		return nil, false
	}
	e, ok := asLinear(other)
	if !ok {
		return nil, false
	}
	// x - e (+ 1 when strict) <= 0
	g := variable(x).addScaled(e, -1)
	if strict {
		g = g.addScaled(constant(1), 1)
	}
	return g, true
}

// isSquare recognises `(* x x)` for a plain variable x.
func isSquare(t *core.Term) (string, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName || !isOp(t.Op().Name, "mul") ||
		len(t.Args()) != 2 {
		return "", false
	}
	a, b := t.Args()[0], t.Args()[1]
	if a.Kind == core.KName && b.Kind == core.KName && a.Name == b.Name {
		return a.Name, true
	}
	return "", false
}

func (r *refiner) walk(t *core.Term, f *facts) error {
	switch t.Kind {
	case core.KInt, core.KFloat, core.KStr, core.KBool, core.KName:
		return nil
	case core.KFn:
		for _, n := range t.Params {
			r.markName(n)
		}
		return r.walk(t.Body(), f)
	}

	op := t.Op()
	if op.Kind != core.KName {
		return nil
	}
	r.markBound(t)
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
		case "table":
			// A RULE'S PARAMETER IS ITS DOMAIN. `(table n (fn (j) …))` says
			// element j is a function of j for j in [0, n), so the body may
			// assume exactly that — and without it a rule that indexes the
			// array it is built from cannot prove its own index, which is what
			// the stencil does on every element.
			//
			// This is tables.md §6 once more: bounds are the domain. `build`
			// needed the same fact from the other side, as `len(b) = n`.
			if len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1 {
				inner := f.clone()
				j := args[1].Params[0]
				r.markName(j)
				inner.assumeLE(constant(0).addScaled(variable(j), -1), "0 <= "+j)
				if e, ok := asLinear(args[0]); ok {
					// j < n, i.e. j - n + 1 <= 0.
					// j - n + 1 <= 0
					inner.assumeLE(variable(j).addScaled(e, -1).addScaled(constant(1), 1),
						j+" < "+args[0].String())
				}
				if err := r.walk(args[0], f); err != nil {
					return err
				}
				return r.walk(args[1].Body(), inner)
			}
		case "table-build":
			// `(build n (fn (b) …))` binds a buffer whose length IS n, and
			// nothing else says so — the buffer is introduced by this form and
			// has no other definition. Without the equation a program cannot
			// prove its own index: the sieve knows `i < n` from its guard and
			// needs `len(c) = n` to connect that to `(c i)`.
			//
			// It is the same job `let` does for `(let (make-bool n) …)`, one
			// constructor over, and it is why `build` carries `(length 1)`.
			if len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1 {
				inner := f.clone()
				name := args[1].Params[0]
				if e, ok := asLinear(args[0]); ok {
					r.assumeLengthEq(inner, name, e)
				}
				r.markName(name)
				if err := r.walk(args[0], inner); err != nil {
					return err
				}
				return r.walk(args[1].Body(), inner)
			}
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

	// INDEXING IS APPLICATION, so its obligation cannot come from a
	// declaration — there is no primitive to carry one.
	//
	// This was found by building it: `(a i)` with an unconstrained `i` was
	// ACCEPTED, while `(go.at-float64 a i)` was correctly refused. The bounds
	// check had lived in the primitive's `(where …)`, and making indexing
	// application deleted the primitive and the obligation with it — a refactor
	// that looks clean and silently removes a safety property.
	//
	// tables.md §6 already said the right thing and it reads differently now:
	// **bounds are the domain**. A table is a function with a finite domain, so
	// `0 <= i < len(a)` is not a check bolted onto an operation, it is the
	// condition for the application to be defined at all. It is generated from
	// the FORM, which means it cannot be forgotten by a target author and
	// applies on all four targets at once.
	if !known && len(args) == 1 && r.isTable(op.Name) {
		if err := r.indexObligation(op, args[0], f); err != nil {
			return err
		}
		return r.walk(args[0], f)
	}

	// Discharge this primitive's own refinement before descending.
	// A CALL'S ARGUMENTS ARE EVALUATED BEFORE IT, so their guarantees are in
	// scope when its own obligation is discharged. Collecting them first is not
	// an optimisation: in `(need (size v))` the only fact that can discharge
	// `need`'s precondition is `size`'s postcondition, and discharging before
	// collecting would refuse a correct program.
	if known {
		for _, a := range args {
			r.collectEnsures(a, f)
		}
	}
	if known && p.Where != nil {
		if _, err := r.discharge(op.Name, p, args, f); err != nil {
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

// ensuresOf is the postcondition of a call, ready to assume about `result`.
// It re-discharges the precondition against the same facts the call site had,
// which is what Lemma 1 requires and is cheap: the fragment is tiny.
func (r *refiner) ensuresOf(call *core.Term, result *core.Term, f *facts) *core.Term {
	if call == nil || call.Kind != core.KApp || call.Op().Kind != core.KName {
		return nil
	}
	p, known := r.tgt.Prims[call.Op().Name]
	if !known || p.Ensures == nil {
		return nil
	}
	proven := true
	if p.Where != nil {
		// Probing must not report: the real walk discharges this call too, and
		// a diagnostic printed twice reads as two problems.
		n := len(r.notes)
		var err error
		proven, err = r.discharge(call.Op().Name, p, call.Args(), f)
		r.notes = r.notes[:n]
		if err != nil || !proven {
			return nil
		}
	}
	q := r.ensured(p, call.Args(), result, proven)
	return q
}

// collectEnsures assumes the postcondition of every PURE call inside a term,
// about the call term itself.
//
// A pure call is a sound key by referential transparency: the same printed term
// in a closed residual denotes the same value, which is exactly what Lemma 2
// says an impure one does not. An impure call never reaches here as a bare
// application — ADR 0010 let-binds it at the application site, and `let` is
// where its guarantee attaches instead.
func (r *refiner) collectEnsures(t *core.Term, f *facts) {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return
	}
	for _, a := range t.Args() {
		r.collectEnsures(a, f)
	}
	if p, known := r.tgt.Prims[t.Op().Name]; known && p.Ensures != nil && p.Pure {
		if q := r.ensuresOf(t, t, f); q != nil {
			assume(f, q)
		}
	}
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
	r.assumeLength(inner, k.Params[0], args[0])
	// A POSTCONDITION ATTACHES TO THE NAME, not to the call (Lemma 2). Two
	// occurrences of an impure call denote different values and the fact layer
	// is keyed by printed term, so the only sound anchor is the binder — which
	// ADR 0010 guarantees exists, because an impure argument is never
	// substituted and is let-bound at the application site.
	if q := r.ensuresOf(args[0], core.Name(k.Params[0]), f); q != nil {
		r.markName(k.Params[0])
		assume(inner, q)
	}
	return r.walk(k.Body(), inner)
}

// discharge proves a primitive's `where` at this call site, with the arguments
// substituted for its parameter names.
// discharge proves a primitive's `where` at this call site and reports whether
// it was PROVEN, as against merely not refused.
//
// The distinction exists for postconditions (Lemma 1, postconditions.md §4). A
// contract is an implication, so an unproven precondition licenses nothing —
// and this function has a path that reports "propagated, not proven" and
// returns success. Treating that as proof would assume a guarantee whose
// premise is unknown, and one false fact makes a conjunctive fragment derive
// anything.
func (r *refiner) discharge(name string, p Prim, args []*core.Term, f *facts) (bool, error) {
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
			return true, nil
		}
		if k, ok := neKey(want); ok && f.entailsOpaque(k) {
			return true, nil
		}
		return false, fmt.Errorf("%s requires %s, which does not follow\n  known: %s",
			name, want, f.known())
	}
	goals, ok := obligation(want)
	if !ok {
		// Outside the fragment: an opaque atom (refinements.md §3). It can be
		// discharged only by an assumption that is the SAME term; otherwise it
		// is propagated, never assumed, and the note says so.
		if f.entailsOpaque(want.String()) {
			return true, nil
		}
		// PROPAGATED, NOT PROVEN — and the second half of that phrase is what a
		// postcondition depends on. Reporting is not proving, so this returns
		// false and the call's guarantee is not licensed (Lemma 1).
		r.notes = append(r.notes, fmt.Sprintf("%s: refinement propagated, not proven", name))
		return false, nil
	}
	for _, g := range goals {
		if !f.entails(g) {
			return false, fmt.Errorf("%s requires %s <= 0, which does not follow\n  known: %s",
				name, g.String(), f.known())
		}
	}
	return true, nil
}

// ensured is the postcondition a call GUARANTEES, with the arguments
// substituted for the parameter names and `result` for the value, or nil.
//
// It returns nil unless the precondition was PROVEN -- Lemma 1 -- and treats
// a primitive that declares none as having P = true, which holds vacuously.
func (r *refiner) ensured(p Prim, args []*core.Term, result *core.Term,
	proven bool) *core.Term {
	if p.Ensures == nil || !proven {
		return nil
	}
	sub := map[string]*core.Term{core.ResultName: result}
	for i, n := range p.Names {
		if n != "" && i < len(args) {
			sub[n] = args[i]
		}
	}
	return core.Rename2(p.Ensures, sub)
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
	// A THREADED array is a loop variable, and the obligations that index it
	// are inside the loop. Establish its length the same way valueLength does —
	// from the initial value, verified against every back edge — and record the
	// equation so `i < len(c)` has something to resolve against.
	env := map[string]*linear{}
	lens := make([]*linear, len(lam.Params))
	for i, n := range lam.Params {
		if i < len(inits) {
			if e, ok := r.valueLength(inits[i], map[string]*linear{}, 0); ok {
				lens[i], env[n] = e, e
			}
		}
	}
	if r.againAgree(lam.Body(), lens, env, 0) {
		for i, n := range lam.Params {
			if lens[i] != nil {
				r.assumeLengthEq(g, n, lens[i])
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

// loopKinds are the heads that OWN an `again`. `loop` is the only one the
// reader emits (core/read.go), and `fold-range` carries its index differently.
var loopKinds = map[string]bool{"loop": true}

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
		// A NESTED loop's `again` belongs to the nested loop. Descending into
		// one made the sieve's outer counter read `(again (+ j i))` — the
		// crossing loop's back edge, with the outer variable in the wrong
		// position — and conclude the counter might decrease. So `0 <= i` was
		// never derived, and the bounds obligation on `c[i]` sat silently
		// outside the fragment until `go.len` became recognisable and turned
		// the silence into a hard failure.
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, known := loopKinds[t.Op().Name]; known && p {
				return
			}
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

// assumeLength records `len(name) = e` when the length of the bound value can
// be computed. The equation is recorded under EVERY spelling of length the
// target declares, because the goal will be written in whichever one the
// program used and `lengthVar` keys on that name.
func (r *refiner) assumeLength(f *facts, name string, value *core.Term) {
	if n, ok := r.valueLength(value, map[string]*linear{}, 0); ok {
		r.assumeLengthEq(f, name, n)
	}
}

// assumeLengthEq records `len(name) = e` under every spelling of length the
// target declares, because the goal will be written in whichever one the
// program used and `lengthVar` keys on that name.
func (r *refiner) assumeLengthEq(f *facts, name string, e *linear) {
	for prim := range r.tgt.Prims {
		if isOp(prim, "alen") || isOp(prim, "slen") {
			f.assumeEQ(lengthVar(prim, core.Name(name)), e)
		}
	}
}

// valueLength is a length abstraction: it computes the length of an array
// expression, or reports that it cannot.
//
// One call to a `(length N)` primitive is not enough on its own. Reduction
// inlines a helper into its caller, so `(let (sieve n) (fn (c) …))` becomes
// `(let (let (make-bool n) (fn (c) (loop … c …))) (fn (c) …))` — the
// allocation is two binders and a loop away from the name that is indexed.
// The abstraction is the same one the interval analysis keeps under `envKey`;
// this is the decision procedure's copy of it, and it stops at `again`, which
// carries no value.
func (r *refiner) valueLength(t *core.Term, env map[string]*linear, depth int) (*linear, bool) {
	if t == nil || depth > 16 {
		return nil, false
	}
	switch t.Kind {
	case core.KName:
		e, ok := env[t.Name]
		return e, ok
	case core.KApp:
	default:
		return nil, false
	}
	if t.Op().Kind != core.KName {
		return nil, false
	}
	op, args := t.Op().Name, t.Args()
	if op == "again" {
		return nil, false // a jump, not a value
	}
	p, known := r.tgt.Prims[op]
	if !known {
		return nil, false
	}
	switch {
	// `(length N)`: argument N is a COUNT. `make([]bool, n)` is n long.
	case p.Length > 0 && p.Length <= len(args):
		return asLinear(args[p.Length-1])
	// `(length-of N)`: the result is AS LONG AS argument N. `c[i] = true`
	// returns something as long as c, which is what makes an in-place store
	// usable as a loop variable.
	case p.LengthOf > 0 && p.LengthOf <= len(args):
		return r.valueLength(args[p.LengthOf-1], env, depth+1)
	case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
		inner := map[string]*linear{}
		for k, v := range env {
			inner[k] = v
		}
		if e, ok := r.valueLength(args[0], env, depth+1); ok {
			inner[args[1].Params[0]] = e
		} else {
			delete(inner, args[1].Params[0])
		}
		return r.valueLength(args[1].Body(), inner, depth+1)
	case p.Kind == "cond" && len(args) == 3:
		// Both arms, and they must agree. An arm that is `again` yields
		// nothing and the other arm decides — which is exactly the shape of a
		// loop that returns the array it filled.
		a, aok := r.valueLength(args[1], env, depth+1)
		b, bok := r.valueLength(args[2], env, depth+1)
		switch {
		case aok && bok:
			if a.String() == b.String() {
				return a, true
			}
			return nil, false
		case aok:
			return a, true
		case bok:
			return b, true
		}
		return nil, false
	case p.Kind == "iterate" || p.Kind == "loop" || p.Kind == "loop2":
		// A threaded array is a LOOP VARIABLE, and its length is a loop
		// invariant that has to be established rather than assumed: bind each
		// variable to the length of its initial value, then check that every
		// `again` passes something of that same length. Assume-and-verify, the
		// standard shape — and if the check fails the binding is dropped and
		// the loop simply proves nothing.
		//
		// Without it the threaded sieve — the version that carries the array
		// through `again` instead of mutating a let-bound one — cannot show
		// that the array it indexes is as long as the bound it indexes under.
		if len(args) == 0 || args[0].Kind != core.KFn {
			return nil, false
		}
		lam, inits := args[0], args[1:]
		inner := map[string]*linear{}
		for k, v := range env {
			inner[k] = v
		}
		lens := make([]*linear, len(lam.Params))
		for i, n := range lam.Params {
			delete(inner, n)
			if i < len(inits) {
				if e, ok := r.valueLength(inits[i], env, depth+1); ok {
					lens[i], inner[n] = e, e
				}
			}
		}
		if !r.againAgree(lam.Body(), lens, inner, depth+1) {
			for _, n := range lam.Params {
				delete(inner, n)
			}
		}
		return r.valueLength(lam.Body(), inner, depth+1)
	}
	return nil, false
}

// againAgree checks that every back edge of THIS loop passes, at each position
// whose length was established from the initial value, something of that same
// length. A nested loop owns its own `again` and is skipped — the same rule
// `nonDecreasing` needs, and for the same reason.
func (r *refiner) againAgree(t *core.Term, lens []*linear, env map[string]*linear, depth int) bool {
	if t == nil || depth > 16 {
		return false
	}
	ok := true
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil || !ok {
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			op := t.Op().Name
			if op == "again" {
				for i, want := range lens {
					if want == nil || i >= len(t.Args()) {
						continue
					}
					got, have := r.valueLength(t.Args()[i], env, depth+1)
					if !have || got.String() != want.String() {
						ok = false
						return
					}
				}
				return
			}
			if p, known := r.tgt.Prims[op]; known && (p.Kind == "iterate" || p.Kind == "loop" || p.Kind == "loop2") {
				return // a nested loop owns its own back edges
			}
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

// isTable reports whether a name in operator position is a table rather than an
// unknown primitive. In a residual it can only be a table (tables.md §3.2), so
// the test is that it is not a declared primitive and not a definition.
func (r *refiner) isTable(name string) bool {
	if _, isPrim := r.tgt.Prims[name]; isPrim {
		return false
	}
	return r.bound[name]
}

// indexObligation demands `0 <= i` and `i < len(a)` at an indexing.
//
// The length term is built with the LANGUAGE's `len`, and then recorded under
// every spelling the target declares as well — because a program may state its
// precondition with either, and `lengthVar` keys on the name it finds.
func (r *refiner) indexObligation(tab, idx *core.Term, f *facts) error {
	lo := &core.Term{Kind: core.KApp, Kids: []*core.Term{
		core.Name("<="), &core.Term{Kind: core.KInt}, idx}}
	hi := &core.Term{Kind: core.KApp, Kids: []*core.Term{
		core.Name("<"), idx,
		&core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name("len"), tab}}}}
	for _, want := range []*core.Term{lo, hi} {
		goals, ok := obligation(want)
		if !ok {
			r.notes = append(r.notes,
				fmt.Sprintf("%s: index bound propagated, not proven", tab))
			continue
		}
		for _, g := range goals {
			if f.entails(g) {
				continue
			}
			return fmt.Errorf("(%s %s) is an indexing, and %s does not follow\n"+
				"  known: %s\n"+
				"  A table is a function with a finite domain, so 0 <= i < len is the\n"+
				"  condition for the application to be DEFINED — not a check bolted on\n"+
				"  (docs/spec/tables.md §6).", tab, idx, want, f.known())
		}
	}
	return nil
}

// markBound records the parameter names of any abstraction sitting directly
// under an application, which is where `loop`, `let` and the fold forms put
// their binders.
func (r *refiner) markBound(t *core.Term) {
	for _, k := range t.Kids {
		if k.Kind == core.KFn {
			for _, n := range k.Params {
				r.markName(n)
			}
		}
	}
}

func (r *refiner) markName(n string) {
	if r.bound == nil {
		r.bound = map[string]bool{}
	}
	r.bound[n] = true
}
