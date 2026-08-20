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
	r.assumeLength(inner, k.Params[0], args[0])
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
