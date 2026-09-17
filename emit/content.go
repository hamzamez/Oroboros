package emit

import (
	"fmt"
	"sort"

	"oroboros/core"
)

// FACTS ABOUT A TABLE'S CONTENTS — F-D₁, spec/theories.md §7.11, array-facts.md.
//
// An F-D₁ fact about a table t is `∀s. 0 ≤ s < len t → φ(t[s], x̄)`, with φ a
// conjunction of linear inequalities over the element and frame terms. It is
// represented as φ itself, a term over the placeholder `#e` standing for the
// element, and kept in facts.content under the table's name.
//
// CONSUMING ONE IS INSTANTIATION (Theorem D): at a read `(t i)` whose domain
// obligation is PROVEN, φ[#e := (t i)] holds. A read whose bound is only
// propagated gives nothing — an undefined read has no value to state a fact of.
//
// ESTABLISHING ONE IS INDUCTION ON A LINEAR BUFFER'S STORE CHAIN (Theorems S and
// S′). ADR 0018 makes every write a `set` on the buffer's current version, so a
// table is the zero fill acted on by a word of stores, and a property the zero
// fill satisfies and every store preserves holds of every slot: the invariant of a
// monoid action. The one judgement below decides it structurally:
//
//	holds(t, F, C) — which φ ∈ C hold of every slot of the table-valued term t
//
// and a loop threading buffers runs Houdini over candidate φ exactly as
// loopInvariants does over scalar templates.

// elemVar is the placeholder an F-D₁ fact uses for the element.
const elemVar = "#e"

// instance is φ with the element replaced by a term.
func instance(phi, elem *core.Term) *core.Term {
	return core.Rename2(phi, map[string]*core.Term{elemVar: elem})
}

// readInstances returns the content facts about a read `(t i)`, instantiated, or
// nothing when t has none or the read is not proven in range.
func (r *refiner) readInstances(read *core.Term, f *facts) []*core.Term {
	var out []*core.Term
	for _, phi := range r.readFacts(read, f) {
		out = append(out, instance(phi, read))
	}
	return out
}

// readFacts is the φ that hold of the value a read `(t i)` produces: the facts
// about t whose component the index is proven to lie in (component.go), and only
// when the read is proven in range.
func (r *refiner) readFacts(read *core.Term, f *facts) []*core.Term {
	if read == nil || read.Kind != core.KApp || len(read.Args()) != 1 || read.Op().Kind != core.KName {
		return nil
	}
	tab := read.Op()
	phis := f.content[tab.Name]
	if len(phis) == 0 {
		return nil
	}
	idx := read.Args()[0]
	lo := core.App(core.Name("<="), &core.Term{Kind: core.KInt}, idx)
	hi := core.App(core.Name("<"), idx, core.App(core.Name("len"), tab))
	for _, want := range []*core.Term{lo, hi} {
		if goals, ok := f.oblig(want); ok && allEntailed(f, goals) {
			continue
		}
		budget := splitBudget
		if !r.provedBySplit(want, f, &budget) {
			return nil
		}
	}
	res := r.residueOf(idx, f)
	var out []*core.Term
	for _, fact := range phis {
		k, c, phi := splitFact(fact)
		if decided, in := res.decides(k, c); decided && in {
			out = append(out, phi)
		}
	}
	return out
}

// isContentRead reports whether t is a read of a table with content facts, which
// makes it an alien a goal or a guard names (provedBySplit, guard).
func isContentRead(t *core.Term, f *facts) bool {
	return f != nil && t != nil && t.Kind == core.KApp && len(t.Args()) == 1 &&
		t.Op().Kind == core.KName && len(f.content[t.Op().Name]) > 0
}

// holds is the judgement: which φ ∈ cands hold of every slot of table term t
// under facts f. Each case is a theorem of array-facts.md §4.
//
// A NIL cands means DISCOVER: every φ the term's own loops prove, with no
// candidate set imposed from outside. A table whose contents come entirely from
// the loop that builds it — a sort's output, bound by `let` before any content is
// in scope — has no candidates to be checked against, and its loop's invariants
// are exactly what is known of it.
func (r *refiner) holds(t *core.Term, f *facts, cands []*core.Term, depth int) []*core.Term {
	if t == nil || (cands != nil && len(cands) == 0) || depth > 12 {
		return nil
	}
	switch t.Kind {
	case core.KName:
		// A NAME: its proven content, or — for a buffer still being filled from
		// its zero fill — every candidate the zero satisfies when a slot exists.
		if have := f.content[t.Name]; len(have) > 0 {
			return intersectPhis(cands, have)
		}
		if f.zero[t.Name] && cands != nil {
			return r.zeroFill(t.Name, f, cands)
		}
		return nil
	case core.KApp:
	default:
		return nil
	}
	if t.Op().Kind != core.KName {
		return nil
	}
	args := t.Args()
	p, known := r.tgt.Prims[t.Op().Name]
	switch {
	case known && p.Kind == "table-set" && len(args) == 3:
		// McCARTHY'S AXIOMS: slot i now holds v, and every other slot is b's.
		// A store at an index whose residue proves it outside a fact's component
		// leaves that component untouched (component.go), so the fact survives
		// without being checked against the stored value.
		base := r.holds(args[0], f, cands, depth+1)
		res := r.residueOf(args[1], f)
		var out []*core.Term
		for _, fact := range base {
			k, c, phi := splitFact(fact)
			if decided, in := res.decides(k, c); decided && !in {
				out = append(out, fact)
				continue
			}
			budget := splitBudget
			if r.provedBySplit(instance(phi, args[2]), f, &budget) {
				out = append(out, fact)
			}
		}
		return out
	case known && p.Kind == "table-build" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
		// THEOREM S: the binder starts as the zero fill of length n.
		name := args[1].Params[0]
		inner := f.clone()
		if e, ok := f.lin(args[0]); ok {
			r.assumeLengthEq(inner, name, e)
		}
		inner.zero[name] = true
		return r.holds(args[1].Body(), inner, cands, depth+1)
	case known && p.Kind == "cond" && len(args) == 3:
		taken := f.clone()
		r.guard(taken, args[0], true)
		missed := f.clone()
		r.guard(missed, args[0], false)
		return intersectPhis(r.holds(args[1], taken, cands, depth+1), r.holds(args[2], missed, cands, depth+1))
	}
	if v, lam, isLet := asLet(r.tgt, t); isLet {
		inner := r.letInner(v, lam.Params[0], f)
		return r.holds(lam.Body(), inner, cands, depth+1)
	}
	if loopKinds[t.Op().Name] {
		return intersectPhis(cands, r.loopContent(t, f))
	}
	return nil
}

// zeroFill is Theorem S's base case: φ(0) under `len b ≥ 1`. An empty table
// satisfies every φ vacuously, and a non-empty one holds zeros until written.
func (r *refiner) zeroFill(name string, f *facts, cands []*core.Term) []*core.Term {
	g := f.clone()
	one := core.App(core.Name("<="), &core.Term{Kind: core.KInt, Int: 1}, core.App(core.Name("len"), core.Name(name)))
	assume(g, one)
	var out []*core.Term
	for _, fact := range cands {
		_, _, phi := splitFact(fact)
		budget := splitBudget
		if r.provedBySplit(instance(phi, &core.Term{Kind: core.KInt}), g, &budget) {
			out = append(out, fact)
		}
	}
	return out
}

// letInner is the facts inside `(let v (fn (x) …))`: what the refinement walk's
// own `let` rule would assume, plus x's content when v is a table.
func (r *refiner) letInner(v *core.Term, x string, f *facts) *facts {
	inner := f.clone()
	if e, ok := f.lin(v); ok {
		inner.assumeEQ(x, e)
	}
	r.assumeLength(inner, x, v)
	r.joinConditional(inner, x, v, f)
	r.summarizeLoop(inner, x, v)
	r.bindContent(inner, x, v, f)
	return inner
}

// bindContent gives a let-bound name what is known of its value: the instances of
// a proven read of a table with content (Theorem D, stated about the binder, the
// name the fragment reads), or the contents of a table-valued term.
func (r *refiner) bindContent(inner *facts, x string, v *core.Term, f *facts) {
	// A READ UNDER `let`s is the read: `(let w (fn (y) (t i)))` has the value of
	// `(t i)` with y = w, so the binder's instances are taken there, with y renamed
	// fresh so no frame term of φ can be captured by it.
	at, read := f, v
	for n := 0; n < 4; n++ {
		w, lam, isLet := asLet(r.tgt, read)
		if !isLet {
			break
		}
		r.splitFresh++
		y := fmt.Sprintf("#b%d", r.splitFresh)
		at = r.letInner(w, y, at)
		read = lam.OpenWith([]*core.Term{core.Name(y)})
	}
	if isContentRead(read, at) {
		for _, phi := range r.readFacts(read, at) {
			assume(inner, instance(phi, core.Name(x)))
		}
		return
	}
	if !r.mayBeTable(v, f) {
		return
	}
	if got := r.holds(v, f, nil, 0); len(got) > 0 {
		inner.content[x] = got
	}
}

// mayBeTable is a cheap syntactic filter: whether t could be a table value at
// all, so a loop over scalars pays nothing for content analysis.
func (r *refiner) mayBeTable(t *core.Term, f *facts) bool {
	for n := 0; t != nil && n < 16; n++ {
		switch t.Kind {
		case core.KName:
			return len(f.content[t.Name]) > 0 || f.zero[t.Name]
		case core.KApp:
		default:
			return false
		}
		if t.Op().Kind != core.KName {
			return false
		}
		if p, known := r.tgt.Prims[t.Op().Name]; known {
			switch p.Kind {
			case "table-set", "table-build":
				return true
			case "cond":
				return r.mayBeTable(t.Args()[1], f) || r.mayBeTable(t.Args()[2], f)
			}
		}
		if _, lam, isLet := asLet(r.tgt, t); isLet {
			t = lam.Body()
			continue
		}
		if loopKinds[t.Op().Name] && len(t.Args()) > 1 {
			for _, z := range t.Args()[1:] {
				if r.mayBeTable(z, f) {
					return true
				}
			}
		}
		return false
	}
	return false
}

func loopKindsOf(t *core.Term) bool {
	return t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName && loopKinds[t.Op().Name]
}

// loopContent is what the loop term's EXIT values hold, under its content
// invariants — the result summary (loopsum-2026-09-16) one index set over.
func (r *refiner) loopContent(loop *core.Term, f *facts) []*core.Term {
	args := loop.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return nil
	}
	lam := args[0]
	// ONLY A LOOP THIS REFINER HAS WALKED: its body facts, content invariants
	// included, are cached by `iterate`. Re-walking a loop to summarise it made the
	// whole emission sweep three times slower, and every caller either walks the
	// loop first (`let`) or checks after its walk has reached it (a back edge).
	body, ok := r.bodyFacts[loopKey(lam, args[1:], f)]
	if !ok {
		return nil
	}
	cands := allContent(body)
	if len(cands) == 0 {
		return nil
	}
	var result []*core.Term
	first := true
	var walk func(t *core.Term, at *facts)
	walk = func(t *core.Term, at *facts) {
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "cond" && len(t.Args()) == 3 {
				taken := at.clone()
				r.guard(taken, t.Args()[0], true)
				walk(t.Args()[1], taken)
				missed := at.clone()
				r.guard(missed, t.Args()[0], false)
				walk(t.Args()[2], missed)
				return
			}
		}
		if hasAgain(t) {
			return
		}
		got := r.holds(t, at, cands, 0)
		if first {
			result, first = got, false
		} else {
			result = intersectPhis(result, got)
		}
	}
	walk(lam.Body(), body)
	return result
}

// contentInvariants is Theorem S′ with Houdini: the content facts that are
// jointly inductive for the loop's table-valued variables.
//
// Candidates for a variable are φ over `#e`:
//
//	0 ≤ #e      #e < s,  #e ≤ s   for s a guard side naming no loop variable
//
// together with every content fact already in scope — a store copying a value out
// of another table carries that table's facts. Entry is `holds(init)`; a back edge
// keeps φ for variable k when `holds(again argument k)` does, under the facts at
// the edge with every surviving candidate assumed for every variable at once.
func (r *refiner) contentInvariants(lam *core.Term, inits []*core.Term, f, g *facts) {
	if r.probeDepth >= 3 || len(inits) < len(lam.Params) {
		return
	}
	params := lam.Params
	var templates []*core.Term
	seen := map[string]bool{}
	addT := func(phi *core.Term) {
		if k := phi.String(); !seen[k] {
			seen[k] = true
			templates = append(templates, phi)
		}
	}
	e := core.Name(elemVar)
	addT(core.App(core.Name("<="), &core.Term{Kind: core.KInt}, e))
	for _, sd := range r.guardSides(lam.Body()) {
		if mentionsAny(sd, params) {
			continue
		}
		addT(core.App(core.Name("<"), e, sd))
		addT(core.App(core.Name("<="), e, sd))
	}
	// COMPONENT CANDIDATES (component.go): each base template restricted to each
	// component of each stride the loop's stores use.
	base := append([]*core.Term(nil), templates...)
	for _, k := range r.strides(lam.Body(), f) {
		for c := int64(0); c < k; c++ {
			for _, phi := range base {
				addT(atFact(k, c, phi))
			}
		}
	}
	for _, phi := range allContent(f) {
		if !mentionsAny(phi, params) {
			addT(phi)
		}
	}
	cands := map[string][]*core.Term{}
	for i, n := range params {
		if !r.mayBeTable(inits[i], f) {
			continue
		}
		if got := r.holds(inits[i], f, templates, 0); len(got) > 0 {
			cands[n] = got
		}
	}
	if len(cands) == 0 {
		return
	}
	for round := 0; round <= len(templates)*len(params)+1; round++ {
		h := g.clone()
		for n, phis := range cands {
			h.content[n] = phis
		}
		bound := map[string]bool{}
		for k, v := range r.bound {
			bound[k] = v
		}
		dry := &refiner{tgt: r.tgt, bound: bound, pure: r.pure, probe: true, probeDepth: r.probeDepth + 1}
		changed := false
		next := map[string][]*core.Term{}
		for n, phis := range cands {
			next[n] = phis
		}
		// THE BACK EDGES ARE CHECKED AFTER THE WALK, not as it reaches them: an
		// argument that is a nested loop — the sort's pass, threading the same
		// buffer — has not been walked yet when its `again` is met, and its
		// contents are known only once it has.
		type edge struct {
			args []*core.Term
			at   *facts
		}
		var edges []edge
		dry.onAgain = func(args []*core.Term, at *facts) {
			edges = append(edges, edge{args, at})
		}
		if err := dry.clauses(lam.Body(), h); err != nil {
			return
		}
		for _, ed := range edges {
			for i, n := range params {
				phis := next[n]
				if len(phis) == 0 {
					continue
				}
				if i >= len(ed.args) {
					next[n], changed = nil, true
					continue
				}
				kept := dry.holds(ed.args[i], ed.at, phis, 0)
				if len(kept) != len(phis) {
					next[n], changed = kept, true
				}
			}
		}
		if !changed {
			for n, phis := range cands {
				g.content[n] = phis
			}
			return
		}
		cands = map[string][]*core.Term{}
		for n, phis := range next {
			if len(phis) > 0 {
				cands[n] = phis
			}
		}
		if len(cands) == 0 {
			return
		}
	}
}

// loopKey identifies a loop WALKED UNDER GIVEN FACTS, for the body-facts cache.
//
// Structural, not by pointer: `Body()` opens a lambda into fresh terms on every
// call, so the loop a summary looks for is never the pointer `iterate` walked.
// And the facts are part of the key, because a loop's invariants depend on what
// held on entry — the same text reached under different assumptions is a
// different loop for this purpose, and reusing its facts would be unsound.
func loopKey(lam *core.Term, inits []*core.Term, f *facts) string {
	// Each part length-prefixed, so the encoding is injective whatever the parts
	// contain — a string literal may hold any separator.
	part := func(s string) string { return fmt.Sprintf("%d:%s", len(s), s) }
	k := part(lam.String())
	for _, z := range inits {
		k += part(z.String())
	}
	return k + part(f.fingerprint())
}

// allContent is every content fact in scope, deduplicated, in a stable order.
func allContent(f *facts) []*core.Term {
	if f == nil {
		return nil
	}
	names := make([]string, 0, len(f.content))
	for n := range f.content {
		names = append(names, n)
	}
	sort.Strings(names)
	seen := map[string]bool{}
	var out []*core.Term
	for _, n := range names {
		for _, phi := range f.content[n] {
			if k := phi.String(); !seen[k] {
				seen[k] = true
				out = append(out, phi)
			}
		}
	}
	return out
}

// intersectPhis keeps the φ of a that also occur in b, compared by printed form.
// A nil a is the unrestricted set, so the result is b.
func intersectPhis(a, b []*core.Term) []*core.Term {
	if a == nil {
		return b
	}
	in := map[string]bool{}
	for _, phi := range b {
		in[phi.String()] = true
	}
	var out []*core.Term
	for _, phi := range a {
		if in[phi.String()] {
			out = append(out, phi)
		}
	}
	return out
}

func (f *facts) contentString() string {
	names := make([]string, 0, len(f.content))
	for n := range f.content {
		names = append(names, n)
	}
	sort.Strings(names)
	s := ""
	for _, n := range names {
		s += fmt.Sprintf("%s:%v ", n, f.content[n])
	}
	return s
}
