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

	// pure is the atom signature Σ every linear reading here uses (linear.go,
	// atoms), so a contract assumed about a pure call and a goal mentioning it
	// name one variable.
	pure atoms

	// probe is set on a DRY walk, one that proves nothing and refuses nothing:
	// it exists to read the facts in scope at a loop's back edges, for the
	// invariant check in `iterate`. An undischarged obligation there is not an
	// error — the real walk will meet it — and `onAgain` is told the facts at
	// every `again` of the loop being probed (loopDepth 0), not of a nested one.
	probe     bool
	onAgain   func(args []*core.Term, f *facts)
	loopDepth int

	// requires collects the contract marks this walk cannot prove (ADR 0028,
	// requires.go). Set only on DischargeRequires' top-level walk, so the dry
	// walks a loop spawns decide nothing.
	requires *[]requireFail

	// splitFresh numbers the names provedBySplit gives a `let`'s binder, so a
	// split never captures a name in the goal around it.
	splitFresh int

	// probeDepth is how many dry walks enclose this one. A loop met inside a
	// probe still gets its invariants and its result summary — a step that reads
	// a scanner's result needs the scanner's summary — but only to a fixed depth,
	// so the rounds do not multiply without bound.
	probeDepth int

	// bodyFacts remembers the facts `iterate` established for a loop's body,
	// keyed by the loop's lambda, so a `let` summarising that loop's RESULT does
	// not run the invariant fixpoint a second time.
	bodyFacts map[string]*facts

	// memo is SHARED by a root refiner and every dry walk under it — see
	// refineMemo. bodyFacts is per refiner, and each Houdini round builds a
	// fresh dry one, so without this a nested loop's fixpoint restarted from
	// nothing in every round of the loop around it.
	memo *refineMemo
}

// refineMemo holds what one Refine computes more than once and may reuse.
//
// houdini is loopInvariants' answer — the invariants that survived — keyed by
// the loop, its initial values, BOTH fact sets and the probe depth. THEOREM:
// loopInvariants is a function of exactly those inputs, up to renaming of the
// dry walks' fresh binders. Its candidates are generated from lam, inits, f and
// g; each is kept or dropped by entailments under facts derived from the same;
// the depth decides where nested probes stop. The candidates mention only
// lam's own parameters, which the key contains, so the answer does not depend
// on the fresh names. So a hit returns exactly what recomputing would.
//
// Measured on the tokeniser (tokenize-compile-2026-09-23): 285 nested fixpoints
// were 60 distinct computations, and 20 top-level ones were 5.
type refineMemo struct {
	houdini map[string][]*core.Term
	// summary is summarizeNamed's answer for a loop it had to walk in a dry
	// refiner: the facts about the bound name that survived, and the name they
	// were computed for. THEOREM: summarizeLoop's candidates are 0 ≤ x, x ≤ s and
	// s ≤ x over frames s of the loop, and each is kept iff it is proven at every
	// exit with the exit's value SUBSTITUTED for x — so x occurs in the answer
	// only as a name, and the answer is a function of the loop, the facts at the
	// binding, the facts it is assumed into and the depth. A hit renames x.
	summary map[string]summaryMemo
}

type summaryMemo struct {
	x    string
	kept []*core.Term
}

func (r *refiner) lin(t *core.Term) (*linear, bool) { return asLinearIn(r.pure, t) }

// pureAtoms is Σ for a target: an application of a declared PURE host primitive
// — an `expr` — that is not an operator the fragment already interprets or keeps
// opaque on purpose. Arithmetic, masks and shifts stay out, so `(* x y)` and a
// non-literal `%` are exactly as opaque as they were; so do comparisons and
// connectives, which are propositions rather than terms.
func pureAtoms(tgt *Target) atoms {
	if tgt == nil {
		return nil
	}
	return func(op string) bool {
		p, ok := tgt.Prims[op]
		if !ok || !p.Pure || p.Kind != "expr" || isLenOp(op) ||
			arithOp(op, 1) != "" || arithOp(op, 2) != "" {
			return false
		}
		for _, w := range []string{"lt", "le", "gt", "ge", "eq", "ne", "and", "or", "not", "if"} {
			if isOp(op, w) {
				return false
			}
		}
		return true
	}
}

// Refine discharges every refinement obligation in a residual.
//
// `sig` supplies the assumptions: a definition may assume its own `where`, and
// that is how a precondition moves to the caller.
func Refine(tgt *Target, what string, sig *core.Sig, t *core.Term) ([]string, error) {
	r := &refiner{tgt: tgt, pure: pureAtoms(tgt), memo: &refineMemo{
		houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
	f := newFacts()
	f.pure = r.pure

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
	// `lang`'S FACTS, INSTANTIATED ONCE AT THE ROOT on every term of the program
	// (emit/fact.go, lang-facts.oro). A DECLARED SOUND AXIOM, NEVER A SEARCH —
	// decidability-map.md's rule for the nonlinear fragment: an operation outside
	// the fragment ENTAILS something inside it, and a local extension instantiated
	// on present terms is complete for it.
	//
	// At the root because a quotient in a loop's GUARD never reaches `walk` —
	// `loopLike` turns a guard into a fact directly — so an axiom added where the
	// term is met is an axiom the one program that needs it never sees.
	//
	// It is what a flattened product needs. `((sp w) 1)` becomes
	// `(sp (+ (* 2 w) 1))` guarded by `w < (len sp)/2`, and without `div-floor`
	// the two facts share no variable: the quotient is an unknown, and nothing
	// says it has anything to do with the length it came from.
	seedFacts(f, t, langFacts)

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
		a, ok1 := f.lin(where.Args()[0])
		b, ok2 := f.lin(where.Args()[1])
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
	if goals, ok := f.oblig(where); ok {
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

	if core.IsRequire(t) || core.IsRequireWhere(t) {
		return r.requireMark(t, f)
	}
	op := t.Op()
	if op.Kind != core.KName {
		// AN OPERATOR THAT IS NOT A NAME IS STILL A TERM WITH OBLIGATIONS IN IT.
		// A host call with several results is eliminated by applying it to its
		// continuation, `((p a…) (fn (x y) body))` (values.go, multiPrimCall),
		// and returning here skipped p's own `where`, its arguments and the
		// whole continuation — every file-reading program's body. Walking the
		// operator discharges the call; walking the argument walks the body.
		// More walking can only find more obligations, so this is sound in the
		// direction that matters (hex-2026-09-14 §3).
		//
		// ONE OPERATOR IS NOT AN INDEXING, and it is the MAP READ being
		// eliminated: `((m k) (fn (tag v) …))` (maps.md F2). A table's domain
		// condition is a proof and a map's is a VALUE, the option the
		// continuation takes apart, so `(m k)` carries no bounds obligation. It
		// is recognised without types: the read is not a primitive, and it is
		// applied to a function — which an ARRAY element never can be, because a
		// closure does not survive staging and a table's elements are data.
		if op.Kind == core.KApp && op.Op().Kind == core.KName && len(t.Args()) == 1 &&
			t.Args()[0].Kind == core.KFn {
			if _, isPrim := r.tgt.Prims[op.Op().Name]; !isPrim {
				for _, a := range op.Args() {
					if err := r.walk(a, f); err != nil {
						return err
					}
				}
				return r.walk(t.Args()[0], f)
			}
		}
		// A HOST CALL WITH SEVERAL RESULTS BINDS EACH ONE WITH ITS RANGE. A range in
		// a result position IS an `ensures` (scalarrange-2026-08-31), and a single
		// result's already reaches this layer as one; a tuple's components did not,
		// so `(b.Add64 x y c)`'s carry, declared (int 0 1), was "known: nothing" at
		// the next Add64's `carry ≤ 1` — a carry chain refused (mathbits-2026-09-23).
		// Each endpoint a term can state is assumed of its binder.
		if pr, _, k, ok := multiPrimCall(r.tgt, t); ok && k.Kind == core.KFn {
			if err := r.walk(op, f); err != nil {
				return err
			}
			g := f
			for i, n := range k.Params {
				if i >= len(pr.Results) {
					break
				}
				lo, hi, haveLo, haveHi := core.RangeBounds(pr.Results[i])
				if g == f && (haveLo || haveHi) {
					g = f.clone()
				}
				if haveLo {
					g.assumeLE(constant(lo).addScaled(variable(n), -1), fmt.Sprintf("%d <= %s", lo, n))
				}
				if haveHi {
					g.assumeLE(variable(n).addScaled(constant(hi), -1), fmt.Sprintf("%s <= %d", n, hi))
				}
			}
			for _, n := range k.Params {
				r.markName(n)
			}
			return r.walk(k.Body(), g)
		}
		if err := r.walk(op, f); err != nil {
			return err
		}
		for _, a := range t.Args() {
			if err := r.walk(a, f); err != nil {
				return err
			}
		}
		return nil
	}
	r.markBound(t)
	args := t.Args()
	if op.Name == "again" && r.onAgain != nil && r.loopDepth == 0 {
		r.onAgain(args, f)
	}
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
				if e, ok := f.lin(args[0]); ok {
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
				// AND A PURE CALL IN THE SIZE BRINGS ITS GUARANTEE. `len b = n` with
				// n = (hex.EncodedLen (len src)) relates len b to an atom, and only the
				// atom's `ensures` — result = 2n — says what that is. Without it the
				// buffer Go's documentation allocates could not satisfy `Encode`'s
				// 2·len src ≤ len dst (postconditions.md §5).
				r.collectEnsures(args[0], inner)
				if e, ok := f.lin(args[0]); ok {
					r.assumeLengthEq(inner, name, e)
				}
				inner.zero[name] = true
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
	if n, ok := f.lin(count); ok {
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
			// AN INSTANCE MAKES ITS TERMS PRESENT, and a fact's trigger is a
			// present term (facts.md §7). DecodedLen's result = x/2 names a
			// quotient no program term contains, so the floor facts that give
			// it meaning — 2q ≤ x < 2q + 2 for x ≥ 0 — are instantiated here.
			seedFacts(f, q, langFacts)
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
	if e, ok := f.lin(args[0]); ok {
		inner.assumeEQ(k.Params[0], e)
	}
	r.assumeLength(inner, k.Params[0], args[0])
	r.joinConditional(inner, k.Params[0], args[0], f)
	r.summarizeLoop(inner, k.Params[0], args[0])
	r.bindContent(inner, k.Params[0], args[0], f)
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

// joinConditional gives a name bound to a CONDITIONAL the facts that hold on
// every branch — which is what a clamp is for, and what was lost the moment a
// clamped value was used twice and call-by-need let-bound it.
//
// THE RULE. Let x = (if c₁ … ℓ₁ … ℓₘ) be a chain whose leaves ℓⱼ are linear,
// each reached under path facts Pⱼ = F ∧ (the guards, or their negations, on
// the way to it). For an inequality φ(x),
//
//	if  Pⱼ ⊢ φ(ℓⱼ)  for every j,  then  F ⊢ φ(x).
//
// Proof: exactly one leaf is evaluated, on a path whose facts hold, and x is
// its value. ∎
//
// The set of φ tried is FINITE and TEMPLATE-shaped: x compared by <, <=, >, >=
// with each linear side of each guard in the chain, and with 0. That is the
// join of template constraint domains (Sankaranarayanan, Sipma & Manna, VMCAI
// 2005) — exact over the templates, and it never needs a convex hull, which a
// conjunction of inequalities cannot represent in general.
//
// For the clamp `(if (< i 0) 0 (if (>= i n) 0 i))` it gives 0 <= x, and x < n
// whenever the facts in scope already give 0 < n — which a caller indexing a
// table it is iterating over always has (hex-2026-09-14 §4).
func (r *refiner) joinConditional(inner *facts, x string, value *core.Term, f *facts) {
	type leaf struct {
		e    *linear
		path *facts
	}
	var leaves []leaf
	var sides []*linear
	// Names bound INSIDE the value. A clamp of a table read is let-bound before it
	// is compared — `(cli n (t j))` reaches here as `(let (t j) (fn (i) (if …)))`
	// — so the walk goes through the `let`; a side mentioning `i` is then not a
	// template, since outside the value `i` may be a different variable.
	binders := map[string]bool{}
	ok := true
	var walk func(t *core.Term, path *facts, depth int)
	walk = func(t *core.Term, path *facts, depth int) {
		if !ok {
			return
		}
		if depth > 8 {
			ok = false
			return
		}
		if v, lam, isLet := asLet(r.tgt, t); isLet {
			binders[lam.Params[0]] = true
			// A NAME BOUND TO A LINEAR VALUE IS THAT VALUE, so the equation holds on
			// every path below it. Without it a clamp written through a helper —
			// `(let (+ lo w) (fn (b) (if (< n b) n b)))`, `min` inlined — had a leaf
			// `b` about which nothing was known.
			if e, ok := path.lin(v); ok {
				path = path.clone()
				path.assumeEQ(lam.Params[0], e)
			}
			walk(lam.Body(), path, depth+1)
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "cond" && len(t.Args()) == 3 {
				c := t.Args()[0]
				if c.Kind == core.KApp && len(c.Args()) == 2 {
					for _, s := range c.Args() {
						if l, isLin := path.lin(s); isLin {
							sides = append(sides, l)
						}
					}
				}
				taken := path.clone()
				r.guard(taken, c, true)
				walk(t.Args()[1], taken, depth+1)
				missed := path.clone()
				r.guard(missed, c, false)
				walk(t.Args()[2], missed, depth+1)
				return
			}
		}
		e, isLin := path.lin(t)
		if !isLin {
			ok = false
			return
		}
		leaves = append(leaves, leaf{e, path})
	}
	walk(value, f.clone(), 0)
	if !ok || len(leaves) < 2 {
		return
	}
	sides = append(sides, constant(0))
	xv := variable(x)
	// Each template is `x - a + c <= 0` (below a: c = 0 for <=, 1 for <) or
	// `a - x + c <= 0` (above a).
	for _, a := range sides {
		inside := false
		for name := range a.coef {
			if binders[name] {
				inside = true
			}
		}
		if inside {
			continue
		}
		for _, t := range []struct {
			below bool
			c     int64
		}{{true, 0}, {true, 1}, {false, 0}, {false, 1}} {
			holds := true
			for _, l := range leaves {
				var g *linear
				if t.below {
					g = l.e.addScaled(a, -1).addScaled(constant(t.c), 1)
				} else {
					g = a.addScaled(l.e, -1).addScaled(constant(t.c), 1)
				}
				if !l.path.entails(g) {
					holds = false
					break
				}
			}
			if !holds {
				continue
			}
			var fact *linear
			if t.below {
				fact = xv.addScaled(a, -1).addScaled(constant(t.c), 1)
			} else {
				fact = a.addScaled(xv, -1).addScaled(constant(t.c), 1)
			}
			inner.assumeLE(fact, fmt.Sprintf("%s joined over its branches", x))
		}
	}
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
		if r.probe {
			return false, nil
		}
		return false, fmt.Errorf("%s requires %s, which does not follow\n  known: %s",
			name, want, f.known())
	}
	goals, ok := f.oblig(want)
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
			if r.probe {
				return false, nil
			}
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
			if e, ok := f.lin(inits[i]); ok {
				// Non-decreasing by the SYNTACTIC rule — every `again` adds a
				// literal — or by loop monotonicity's relation `e ⊒ S`, which also
				// sees an index assigned a scanner's result (monotone.go, rule 6).
				//
				// And the start need not be a literal. The theorem gives v >= z for
				// the value z had on ENTRY, and every name z mentions is immutable,
				// so whatever the entry facts prove about z holds of it at every
				// iteration: `0 <= z` entailed there is `0 <= v` throughout. A
				// literal is the case with no names, decided by the same call.
				if f.entails(constant(0).addScaled(e, -1)) &&
					(nonDecreasing(lam.Body(), i, n) || monotoneStep(r.tgt, lam, i)) {
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
	r.loopInvariants(lam, inits, f, g)
	r.contentInvariants(lam, inits, f, g)
	if r.bodyFacts == nil {
		r.bodyFacts = map[string]*facts{}
	}
	r.bodyFacts[loopKey(lam, inits, f)] = g
	r.loopDepth++
	defer func() { r.loopDepth-- }()
	return r.clauses(lam.Body(), g)
}

// provedBySplit tries to PROVE a goal that is outside the fragment because a
// CONDITIONAL or a `let` sits somewhere inside it — at the top, as a clamped
// index, or nested under arithmetic, as a strided clamp `4·(clamp k) + 2`.
//
// THE ALGEBRA IS CASE-OF-CASE, the commuting conversion reduction already uses
// (sums.md): for any context C[·],
//
//	C[(if c a b)]            =  (if c C[a] C[b])
//	C[(let v (fn (x) e))]    =  (let v (fn (x) C[e]))        x fresh
//
// so, for a goal G,
//
//	G(C[(if c a b)])  ⟺  (c → G(C[a]))  ∧  (¬c → G(C[b]))
//
// and a `let` becomes an assumption `x = v` when v is linear (and x is otherwise
// opaque, which only weakens). That is Nelson & Oppen's purification made EXACT
// rather than approximated: the alien subterm is not replaced by a name carrying
// template facts — the goal is split on it, so no template has to foresee the
// bound (`4·k + 2 < 2048` is not a side of any guard in sight). Each split is
// sound because exactly one branch is evaluated, under the facts that hold on
// its path.
//
// IT IS A PROOF ATTEMPT, never a refusal (joinindex-2026-09-16): a goal it cannot
// prove is propagated exactly as before. The split is bounded — `splitBudget`
// leaves — because it is exponential in the number of aliens, and a goal with
// more is left to the note.
func (r *refiner) provedBySplit(goal *core.Term, f *facts, budget *int) bool {
	if *budget <= 0 {
		return false
	}
	alien := r.firstAlien(goal, f)
	if alien == nil {
		goals, ok := f.oblig(goal)
		if !ok {
			return false
		}
		for _, g := range goals {
			if !f.entails(g) {
				return false
			}
		}
		*budget--
		return true
	}
	// A LOOP is named too: C[loop] = let x = loop in C[x], and x carries the
	// loop's result summary. β substitutes a scanner used once straight into the
	// `again` that consumes it, so a back-edge argument is often the loop itself.
	if alien.Op().Kind == core.KName && (loopKinds[alien.Op().Name] || isContentRead(alien, f)) {
		r.splitFresh++
		x := fmt.Sprintf("#s%d", r.splitFresh)
		inner := f.clone()
		r.summarizeNamed(inner, x, alien, f)
		return r.provedBySplit(replaceTerm(goal, alien, core.Name(x)), inner, budget)
	}
	if v, lam, isLet := asLet(r.tgt, alien); isLet {
		r.splitFresh++
		x := fmt.Sprintf("#s%d", r.splitFresh)
		inner := f.clone()
		if e, ok := f.lin(v); ok {
			inner.assumeEQ(x, e)
		} else {
			r.joinConditional(inner, x, v, f)
		}
		body := lam.OpenWith([]*core.Term{core.Name(x)})
		return r.provedBySplit(replaceTerm(goal, alien, body), inner, budget)
	}
	args := alien.Args()
	c := args[0]
	taken := f.clone()
	r.guard(taken, c, true)
	if !r.provedBySplit(replaceTerm(goal, alien, args[1]), taken, budget) {
		return false
	}
	missed := f.clone()
	r.guard(missed, c, false)
	return r.provedBySplit(replaceTerm(goal, alien, args[2]), missed, budget)
}

// guard assumes a clause guard — or its negation — with every alien subterm of
// its comparison NAMED first.
//
// A hypothesis cannot be split the way a goal is: splitting `C[if c a b]` as a
// fact gives a DISJUNCTION, which a conjunction of inequalities cannot hold. It
// can be NAMED exactly: C[e] is `let x = e in C[x]`, so assuming C[x] together
// with what is known of x — a loop's result summary, a conditional's branch
// join — loses nothing that the opaque atom kept, and reads what it could not.
// β puts a scanner used once straight into the guard that tests it, so
// `(< u (loop …))` is a comparison the fragment could never read before.
func (r *refiner) guard(f *facts, c *core.Term, holds bool) {
	if c != nil && c.Kind == core.KApp && c.Op().Kind == core.KName && len(c.Args()) == 2 {
		if _, _, isLet := asLet(r.tgt, c); !isLet {
			if p, known := r.tgt.Prims[c.Op().Name]; !known || p.Kind != "cond" {
				kids := append([]*core.Term(nil), c.Kids...)
				for i := 1; i < len(kids); i++ {
					for n := 0; n < 4; n++ {
						alien := r.firstAlien(kids[i], f)
						if alien == nil {
							break
						}
						r.splitFresh++
						x := fmt.Sprintf("#g%d", r.splitFresh)
						r.summarizeNamed(f, x, alien, f)
						kids[i] = replaceTerm(kids[i], alien, core.Name(x))
					}
				}
				named := *c
				named.Kids = kids
				// BOTH are true, and both are kept: the original is what an
				// opaque goal written the same way matches syntactically, and the
				// named comparison is what the fragment can read.
				if holds {
					assume(f, c)
				} else if n := negate(c); n != nil {
					assume(f, n)
				}
				c = &named
			}
		}
	}
	if holds {
		assume(f, c)
		return
	}
	if n := negate(c); n != nil {
		assume(f, n)
	}
}

// summarizeNamed gives the fresh name x what is known of the alien term e, under
// facts at: a loop's result summary, or a conditional's (or `let`'s) branch join.
func (r *refiner) summarizeNamed(into *facts, x string, e *core.Term, at *facts) {
	if isContentRead(e, at) {
		for _, phi := range r.readFacts(e, at) {
			assume(into, instance(phi, core.Name(x)))
		}
		return
	}
	if e.Op().Kind == core.KName && loopKinds[e.Op().Name] {
		if args := e.Args(); len(args) > 1 {
			if _, done := r.bodyFacts[loopKey(args[0], args[1:], at)]; done {
				r.summarizeLoop(into, x, e)
				return
			}
		}
		// THE MEMO (refineMemo.summary): a dry walk of the whole loop, repeated in
		// every Houdini round around it, is the cost this avoids.
		key := ""
		if r.memo != nil {
			key = fmt.Sprintf("%d:%s|%d:%s|%d:%s|%d", len(e.String()), e.String(),
				len(at.fingerprint()), at.fingerprint(), len(into.fingerprint()), into.fingerprint(), r.probeDepth)
			if m, hit := r.memo.summary[key]; hit {
				for _, c := range m.kept {
					assume(into, core.Rename2(c, map[string]*core.Term{m.x: core.Name(x)}))
				}
				return
			}
		}
		bound := map[string]bool{}
		for k, v := range r.bound {
			bound[k] = v
		}
		d := &refiner{tgt: r.tgt, bound: bound, pure: r.pure, probe: true, probeDepth: r.probeDepth, memo: r.memo}
		_ = d.walk(e, at)
		kept := d.summarizeLoop(into, x, e)
		if r.memo != nil {
			r.memo.summary[key] = summaryMemo{x: x, kept: kept}
		}
		return
	}
	r.joinConditional(into, x, e, at)
}

// splitBudget bounds provedBySplit's leaves. The corpus's goals carry at most
// three aliens; 64 leaves is two more levels than that needs.
const splitBudget = 64

// firstAlien is the leftmost-outermost conditional or `let` in a goal, outside
// any lambda — the subterm the next split is on.
func (r *refiner) firstAlien(t *core.Term, f *facts) *core.Term {
	if t == nil || t.Kind != core.KApp {
		return nil
	}
	// A READ OF A TABLE WITH CONTENT FACTS is named too, so its instances reach
	// the goal (Theorem D, emit/content.go).
	if isContentRead(t, f) {
		return t
	}
	if _, _, isLet := asLet(r.tgt, t); isLet {
		return t
	}
	if op := t.Op(); op.Kind == core.KName {
		if p, known := r.tgt.Prims[op.Name]; known && p.Kind == "cond" && len(t.Args()) == 3 {
			return t
		}
		if loopKinds[op.Name] {
			return t
		}
	}
	for _, a := range t.Args() {
		if found := r.firstAlien(a, f); found != nil {
			return found
		}
	}
	return nil
}

// replaceTerm rebuilds t with the one subterm `old` (by identity) replaced.
func replaceTerm(t, old, new *core.Term) *core.Term {
	if t == old {
		return new
	}
	if t == nil || t.Kind != core.KApp {
		return t
	}
	kids := make([]*core.Term, len(t.Kids))
	changed := false
	for i, k := range t.Kids {
		kids[i] = replaceTerm(k, old, new)
		changed = changed || kids[i] != k
	}
	if !changed {
		return t
	}
	out := *t
	out.Kids = kids
	return &out
}

// nonNegative reports whether facts f prove 0 <= e. A back-edge argument is
// often a CONDITIONAL rather than a name — β substitutes a let-bound clamp used
// once straight into the `again` — so the goal is split on it (provedBySplit).
func (r *refiner) nonNegative(e *core.Term, f *facts) bool {
	budget := splitBudget
	goal := &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name("<="), &core.Term{Kind: core.KInt}, e}}
	return r.provedBySplit(goal, f, &budget)
}

// loopInvariants assumes every candidate linear inequality over a loop's
// variables that is an INDUCTIVE INVARIANT, decided by the fragment rather than by
// the shape of the step (inductive-2026-09-16, generalised).
//
// THEOREM (inductive invariant). Let C be a set of linear inequalities over the
// loop variables v̄ such that the entry facts F prove each φ ∈ C with v̄ := z̄, and
// at every back edge `(again ā)`, reached under facts P,
// P ∧ ⋀C ⊢ φ[ā/v̄] for every φ ∈ C. Then ⋀C holds at every iteration.
// Proof: induction on iterations, simultaneously over C. On entry by the first
// hypothesis; if ⋀C holds at an iteration, the back edge taken is reached under
// facts that hold there, so ⋀C holds of the next values. ∎
//
// P may be used because the facts `iterate` keeps in a body never include a
// variable's initial value — only what every iteration guarantees.
//
// FINDING C is Houdini (Flanagan & Leino, FME 2001) over a FINITE set of
// templates fixed before the fixpoint — Sankaranarayanan, Sipma & Manna's shape:
//
//	0 ≤ v          v ≤ s,  s ≤ v   for s a linear side of a guard in the clause chain
//	z ≤ v          for z an initial value naming no loop variable (a frame term)
//	u ≤ v          for two loop variables — the difference template, count ≤ trips
//
// Discarding only weakens the hypotheses, so the iteration is monotone and stops
// within |C| + 1 rounds at the greatest jointly inductive subset. A candidate the
// templates do not generate is not searched for.
//
// Each round is a dry walk of the body (refiner.probe), which reads the facts at
// each of this loop's back edges — including the result summaries of loops inside
// it, to a fixed probe depth.
func (r *refiner) loopInvariants(lam *core.Term, inits []*core.Term, f, g *facts) {
	if r.probeDepth >= 3 {
		return
	}
	params := lam.Params
	if len(inits) < len(params) {
		return
	}
	// THE MEMO (refineMemo). Keyed before g is touched; every exit below records
	// what it assumed into g, which is the whole of the answer.
	var key string
	if r.memo != nil {
		key = fmt.Sprintf("%s%d:%s|%d", loopKey(lam, inits, f), len(g.fingerprint()), g.fingerprint(), r.probeDepth)
		if kept, hit := r.memo.houdini[key]; hit {
			for _, c := range kept {
				assume(g, c)
			}
			return
		}
	}
	var kept []*core.Term
	defer func() {
		if r.memo != nil {
			r.memo.houdini[key] = kept
		}
	}()
	entry := map[string]*core.Term{}
	for i, n := range params {
		entry[n] = inits[i]
	}
	var cands []*core.Term
	seen := map[string]bool{}
	add := func(a, b *core.Term) {
		c := core.App(core.Name("<="), a, b)
		k := c.String()
		if seen[k] || a.String() == b.String() {
			return
		}
		seen[k] = true
		// Already known — the syntactic rules gave it — or false on entry.
		if goals, ok := g.oblig(c); ok && allEntailed(g, goals) {
			return
		}
		budget := splitBudget
		if !r.provedBySplit(core.Rename2(c, entry), f, &budget) {
			return
		}
		cands = append(cands, c)
	}
	zero := &core.Term{Kind: core.KInt}
	sides := r.guardSides(lam.Body())
	for i, n := range params {
		v := core.Name(n)
		add(zero, v)
		for _, sd := range sides {
			if mentionsName(sd, n) {
				continue
			}
			add(v, sd)
			add(sd, v)
		}
		if !mentionsAny(inits[i], params) {
			add(inits[i], v)
		}
		for _, u := range params {
			if u != n {
				add(core.Name(u), v)
			}
		}
	}
	for round := 0; round <= len(cands) && len(cands) > 0; round++ {
		h := g.clone()
		for _, c := range cands {
			assume(h, c)
		}
		failed := map[int]bool{}
		bound := map[string]bool{}
		for k, v := range r.bound {
			bound[k] = v
		}
		dry := &refiner{tgt: r.tgt, bound: bound, pure: r.pure, probe: true, probeDepth: r.probeDepth + 1, memo: r.memo}
		dry.onAgain = func(args []*core.Term, at *facts) {
			sub := map[string]*core.Term{}
			for i, n := range params {
				if i < len(args) {
					sub[n] = args[i]
				}
			}
			for i, c := range cands {
				if failed[i] {
					continue
				}
				budget := splitBudget
				if len(args) < len(params) || !dry.provedBySplit(core.Rename2(c, sub), at, &budget) {
					failed[i] = true
				}
			}
		}
		if err := dry.clauses(lam.Body(), h); err != nil {
			return // a probe that cannot walk the body licenses nothing
		}
		if len(failed) == 0 {
			for _, c := range cands {
				assume(g, c)
			}
			kept = cands
			return
		}
		var keep []*core.Term
		for i, c := range cands {
			if !failed[i] {
				keep = append(keep, c)
			}
		}
		cands = keep
	}
}

// summarizeLoop gives a name bound to a LOOP'S RESULT what every exit satisfies.
//
// THEOREM (result summary). Let I be the invariants of the loop's body, and let
// each exit clause return r_j, reached under path facts P_j. If P_j ∧ I ⊢ φ(r_j)
// for every exit j, then φ(result). Proof: a loop has a value only at some exit
// on some iteration, where I and that exit's path facts hold. ∎ (Partial
// correctness: a loop that does not terminate has no value to be wrong about.)
//
// The candidate φ are `result ≤ s` and `s ≤ result` for s a guard side or an
// initial value naming no loop variable, and `0 ≤ result` — the same finite
// template set as the invariants, read at the exits. `nwords`' count is at most
// `len src` this way: the invariants say n ≤ i ≤ len src, and both exits return n.
func (r *refiner) summarizeLoop(inner *facts, x string, loop *core.Term) (kept []*core.Term) {
	if loop.Kind != core.KApp || loop.Op().Kind != core.KName || !loopKinds[loop.Op().Name] {
		return nil
	}
	args := loop.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return nil
	}
	lam, inits := args[0], args[1:]
	body, ok := r.bodyFacts[loopKey(lam, inits, inner)]
	if !ok {
		return nil
	}
	params := lam.Params
	type exit struct {
		value *core.Term
		at    *facts
	}
	var exits []exit
	var walk func(t *core.Term, at *facts)
	walk = func(t *core.Term, at *facts) {
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "cond" && len(t.Args()) == 3 {
				c := t.Args()[0]
				taken := at.clone()
				r.guard(taken, c, true)
				walk(t.Args()[1], taken)
				missed := at.clone()
				r.guard(missed, c, false)
				walk(t.Args()[2], missed)
				return
			}
		}
		if hasAgain(t) {
			return // a back edge, not an exit
		}
		exits = append(exits, exit{t, at})
	}
	walk(lam.Body(), body)
	if len(exits) == 0 {
		return nil
	}
	res := core.Name(x)
	zero := &core.Term{Kind: core.KInt}
	var frames []*core.Term
	for _, sd := range r.guardSides(lam.Body()) {
		if !mentionsAny(sd, params) {
			frames = append(frames, sd)
		}
	}
	for _, z := range inits {
		if !mentionsAny(z, params) {
			frames = append(frames, z)
		}
	}
	try := func(c *core.Term) {
		for _, e := range exits {
			budget := splitBudget
			if !r.provedBySplit(core.Rename2(c, map[string]*core.Term{x: e.value}), e.at, &budget) {
				return
			}
		}
		assume(inner, c)
		kept = append(kept, c)
	}
	try(core.App(core.Name("<="), zero, res))
	for _, sd := range frames {
		try(core.App(core.Name("<="), res, sd))
		try(core.App(core.Name("<="), sd, res))
	}
	return kept
}

// guardSides is the linear-arithmetic sides of every comparison guarding a clause
// of a loop body's chain — the template vocabulary for its invariants.
func (r *refiner) guardSides(body *core.Term) []*core.Term {
	var out []*core.Term
	seen := map[string]bool{}
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
			return
		}
		if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "cond" && len(t.Args()) == 3 {
			c := t.Args()[0]
			if c.Kind == core.KApp && len(c.Args()) == 2 {
				for _, sd := range c.Args() {
					if k := sd.String(); !seen[k] {
						seen[k] = true
						out = append(out, sd)
					}
				}
			}
			walk(t.Args()[2])
		}
	}
	walk(body)
	return out
}

func mentionsAny(t *core.Term, names []string) bool {
	for _, n := range names {
		if mentionsName(t, n) {
			return true
		}
	}
	return false
}

func allEntailed(f *facts, goals []*linear) bool {
	for _, g := range goals {
		if !f.entails(g) {
			return false
		}
	}
	return true
}

// hasAgain reports whether t jumps back to the loop that owns it: an `again`
// outside any nested loop, whose own back edges are its own.
func hasAgain(t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if t.Op().Name == "again" {
			return true
		}
		if loopKinds[t.Op().Name] {
			return false
		}
	}
	if t.Kind == core.KFn {
		return hasAgain(t.Body())
	}
	for _, k := range t.Kids {
		if hasAgain(k) {
			return true
		}
	}
	return false
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
			r.guard(taken, args[0], true)
			if err := r.clauses(args[1], taken); err != nil {
				return err
			}
			// The other half of Hoare logic, and the half that matters here:
			// reaching a later clause means every earlier guard was FALSE. That
			// is where `i < alen a` comes from in a search whose first clause is
			// `(int.ge i (alen a))`.
			missed := f.clone()
			r.guard(missed, args[0], false)
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

// lengthContract reads a primitive's LENGTH POSTCONDITION: the conjunct of its
// `ensures` that is `len(result) = n` (a COUNT, argument n) or
// `len(result) = len(c)` (a PASS-THROUGH, argument c), in either orientation.
// It is the inverse of the respelling of `(length N)` and `(length-of N)`
// (theories.md §8.4), which is what makes that respelling checkable: for every
// declaration, reading the new form back gives the old attribute.
func lengthContract(p Prim) (count bool, at int, ok bool) {
	var scan func(t *core.Term) bool
	scan = func(t *core.Term) bool {
		if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
			return false
		}
		if a, b, conj := erasedAnd(t); conj {
			return scan(a) || scan(b)
		}
		if isOp(t.Op().Name, "and") && len(t.Args()) == 2 {
			return scan(t.Args()[0]) || scan(t.Args()[1])
		}
		if !isOp(t.Op().Name, "eq") || len(t.Args()) != 2 {
			return false
		}
		isLenOf := func(x *core.Term, name string) (string, bool) {
			if x.Kind != core.KApp || x.Op().Kind != core.KName || !isLenOp(x.Op().Name) ||
				len(x.Args()) != 1 || x.Args()[0].Kind != core.KName {
				return "", false
			}
			return x.Args()[0].Name, name == "" || x.Args()[0].Name == name
		}
		for _, side := range [][2]*core.Term{{t.Args()[0], t.Args()[1]}, {t.Args()[1], t.Args()[0]}} {
			if _, isResult := isLenOf(side[0], core.ResultName); !isResult {
				continue
			}
			other, arg := side[1], ""
			if other.Kind == core.KName {
				arg, count = other.Name, true
			} else if name, isLen := isLenOf(other, ""); isLen {
				arg, count = name, false
			}
			for i, n := range p.Names {
				if arg != "" && n == arg {
					at = i
					return true
				}
			}
		}
		return false
	}
	ok = scan(p.Ensures)
	return count, at, ok
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
	count, at, declared := lengthContract(p)
	switch {
	// len(result) = n: argument n is a COUNT. `make([]bool, n)` is n long.
	case declared && count && at < len(args):
		return r.lin(args[at])
	// len(result) = len(c): the result is AS LONG AS argument c. `c[i] = true`
	// returns something as long as c, which is what makes an in-place store
	// usable as a loop variable.
	case declared && !count && at < len(args):
		return r.valueLength(args[at], env, depth+1)
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
		goals, ok := f.oblig(want)
		if !ok {
			budget := splitBudget
			if r.provedBySplit(want, f, &budget) {
				continue
			}
			r.notes = append(r.notes,
				fmt.Sprintf("%s: index bound propagated, not proven", tab))
			continue
		}
		for _, g := range goals {
			if f.entails(g) || r.probe {
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
