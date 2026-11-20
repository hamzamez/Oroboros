package emit

// THE ARRAY PROPERTY FRAGMENT (ADR 0145 stage 3; logic-research §4.2): a
// universal over a table's indices in Γ, `∀ i ∈ 0..n-1 : a[i] <= a[i]`'s kind,
// decided by instantiation over the sequent's index set.
//
// A SEQUENT IS PREPARED ON THE PROGRAM'S OWN TERMS, where a read is still a
// read and a quantifier's binder still a λ, before anything is a formula of
// integers:
//   - the goal's polarity first: a premise P ⇒ Q moves into Γ; a ∀ of the goal
//     is a fresh name with its set's bounds in Γ (Theorem S); a conjunction is
//     one sequent per conjunct;
//   - a ∀ in Γ is a SCHEMA, instantiated at every term of the INDEX SET I: the
//     index of every read a[e] in the sequent, every bound of a quantifier's
//     set, and 0 (Bradley, Manna and Sipma, "What's decidable about arrays?",
//     VMCAI 2006, Theorem 1: for a formula in the fragment, its instances over
//     I are equisatisfiable with it). A schema of k variables gives |I|ᵏ
//     instances, each ground;
//   - a READ IS A FUNCTION'S APPLICATION, so for two reads of one table,
//     e₁ = e₂ ⇒ a[e₁] = a[e₂] joins Γ: Ackermann's reduction (1954) of
//     equality with uninterpreted functions, quadratic in the reads;
//   - a literal table read at an unknown index is the choice among its
//     elements, its graph (finding 101's derivation), so a sorted literal is
//     sorted by the case split.
// What is left is ground, read by the translation (formula.go) with each read
// an unknown, and decided by presburger.Valid.

import (
	"fmt"

	"oroboros/core"
	"oroboros/presburger"
)

// PlantIndexSetShort leaves the reads' indices out of the index set, so a
// schema is instantiated only at its sets' bounds: binary search's claim and
// the least of a sorted table are then refused (TestATablesPropertyIsDecided).
var PlantIndexSetShort bool

// PlantReadsNotCongruent omits Ackermann's e₁ = e₂ ⇒ a[e₁] = a[e₂]: two reads
// at one index proven equal are then two unknowns, and the least of a sorted
// table is refused.
var PlantReadsNotCongruent bool

// sequent is Γ ⊢ φ on the program's terms: hyps the formulas assumed, goal the
// one to prove.
type sequent struct {
	hyps []*core.Term
	goal *core.Term
}

// arrayHolds decides Γ ⊢ cond, Γ the facts f and the formulas assumed in
// scope, by the array property fragment and presburger.Valid.
func (r *refiner) arrayHolds(cond *core.Term, f *facts) bool {
	pr := &prepare{r: r}
	hyps := append(append([]*core.Term(nil), f.formulas...), f.guards...)
	seqs := pr.split(sequent{hyps: hyps, goal: cond})
	for _, s := range seqs {
		// THREE TIERS, each a weaker Γ than the next, so each answer "proven"
		// is sound, and the last is Theorem A's: the schemas at the reads'
		// indices alone, then at the full index set (its sets' bounds too);
		// a literal's reads as a function's, then by its elements (a sorted
		// literal). A sequent proven by fewer instances costs fewer.
		if !r.groundHolds(pr, s, f, false, false) && !r.groundHolds(pr, s, f, false, true) &&
			!r.groundHolds(pr, s, f, true, true) {
			return false
		}
	}
	return true
}

type prepare struct {
	r *refiner
	n int
}

func (pr *prepare) fresh() string {
	pr.n++
	return fmt.Sprintf("#sk%d", pr.n)
}

// split takes a sequent's goal apart by its polarity: a premise into Γ, a ∀
// to a fresh name, a conjunction into one sequent each.
func (pr *prepare) split(s sequent) []sequent {
	g := stripMarks(s.goal)
	if g.Kind == core.KApp && len(g.Kids) == 3 && g.Kids[0].Kind == core.KName && g.Kids[0].Name == core.ImpliesName {
		return pr.split(sequent{hyps: append(append([]*core.Term(nil), s.hyps...), g.Kids[1]), goal: g.Kids[2]})
	}
	if g.Kind == core.KApp && len(g.Kids) == 3 && g.Kids[0].Kind == core.KName && g.Kids[0].Name == core.ForallName &&
		g.Kids[2].Kind == core.KFn && len(g.Kids[2].Params) == 1 {
		c := core.Name(pr.fresh())
		hyps := append(append([]*core.Term(nil), s.hyps...), memberTerm(g.Kids[1], c))
		return pr.split(sequent{hyps: hyps, goal: g.Kids[2].OpenWith([]*core.Term{c})})
	}
	if a, b, ok := conjunction(g); ok {
		return append(pr.split(sequent{hyps: s.hyps, goal: a}), pr.split(sequent{hyps: s.hyps, goal: b})...)
	}
	return []sequent{{hyps: s.hyps, goal: g}}
}

// stripMarks is t under its marks, a claim's value.
func stripMarks(t *core.Term) *core.Term {
	for t.Kind == core.KApp && len(t.Kids) == 4 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == core.RequireWhereName {
		t = t.Kids[3]
	}
	return t
}

// memberTerm is c ∈ S as a Boolean term: a range's ends, or true for ℤ.
func memberTerm(s, c *core.Term) *core.Term {
	var lo, hi *core.Term
	switch {
	case s.Kind == core.KApp && s.Op().Kind == core.KName && s.Op().Name == ".." && len(s.Args()) == 2:
		lo, hi = s.Args()[0], s.Args()[1]
		if lo.Kind == core.KName && lo.Name == "-inf" {
			lo = nil
		}
		if hi.Kind == core.KName && hi.Name == "+inf" {
			hi = nil
		}
	case s.Kind == core.KStr || s.Kind == core.KName:
		ty := s.Str
		if s.Kind == core.KName {
			ty = core.TypeName(s)
		}
		l, h, ok := core.RangeEnds(ty)
		if !ok {
			return core.Bool(true)
		}
		if l != nil && l.IsInt64() {
			lo = core.Int(l.Int64())
		}
		if h != nil && h.IsInt64() {
			hi = core.Int(h.Int64())
		}
	default:
		return core.Bool(true)
	}
	var out *core.Term = core.Bool(true)
	if hi != nil {
		out = core.App(core.Name("<="), c, hi)
	}
	if lo != nil {
		if hi != nil {
			out = core.App(core.Name("if"), core.App(core.Name("<="), lo, c), out, core.Bool(false))
		} else {
			out = core.App(core.Name("<="), lo, c)
		}
	}
	return out
}

// schema is a ∀ assumed in Γ: its binders' sets and its body, opened at terms.
type schema struct {
	sets []*core.Term
	body func(ts []*core.Term) *core.Term
}

// asSchema reads a hypothesis ∀ x₁ ∈ S₁ : … ∀ xₖ ∈ Sₖ : φ, k ≥ 1.
func asSchema(h *core.Term) (schema, bool) {
	h = stripMarks(h)
	var sets []*core.Term
	var lams []*core.Term
	t := h
	for t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == core.ForallName &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 && len(sets) < 3 {
		sets = append(sets, t.Kids[1])
		lams = append(lams, t.Kids[2])
		// the inner quantifier's set may name the outer binder; opened at a
		// placeholder, it is re-opened at the instance below
		t = t.Kids[2].Closed()
	}
	if len(sets) == 0 {
		return schema{}, false
	}
	k := len(sets)
	return schema{sets: sets, body: func(ts []*core.Term) *core.Term {
		// open the binders one by one, outermost first, each set under the
		// terms already chosen
		var conds []*core.Term
		cur := h
		for i := 0; i < k; i++ {
			conds = append(conds, memberTerm(cur.Kids[1], ts[i]))
			cur = stripMarks(cur.Kids[2].OpenWith([]*core.Term{ts[i]}))
		}
		prem := conds[0]
		for _, c := range conds[1:] {
			prem = core.App(core.Name("if"), prem, c, core.Bool(false))
		}
		return core.App(core.Name(core.ImpliesName), prem, cur)
	}}, true
}

// groundHolds instantiates a sequent's schemas over its index set, adds the
// reads' congruence and decides the ground sequent.
func (r *refiner) groundHolds(pr *prepare, s sequent, f *facts, expand, full bool) bool {
	lit := func(t *core.Term) *core.Term {
		if expand {
			return r.literalReads(t)
		}
		return t
	}
	var ground []*core.Term
	var schemas []schema
	for _, h := range s.hyps {
		for _, c := range conjunctsOf(stripMarks(h)) {
			if sc, ok := asSchema(c); ok && mentionsRead(r, c) {
				schemas = append(schemas, sc)
				continue
			}
			ground = append(ground, c)
		}
	}
	goal := lit(s.goal)
	for i := range ground {
		ground[i] = lit(ground[i])
	}
	// THE INDEX SET: every read's index in the ground part and the goal, every
	// bound of a schema's sets, and 0
	idx := map[string]*core.Term{"0": core.Int(0)}
	add := func(t *core.Term) {
		if t != nil {
			idx[core.ShowTerm(t)] = t
		}
	}
	if !PlantIndexSetShort {
		for _, t := range append(append([]*core.Term(nil), ground...), goal) {
			for _, rd := range r.reads(t) {
				add(rd.Kids[1])
			}
		}
	}
	for _, sc := range schemas {
		if !full {
			break // the reads' indices alone
		}
		for _, set := range sc.sets {
			if set.Kind == core.KApp && set.Op().Kind == core.KName && set.Op().Name == ".." && len(set.Args()) == 2 {
				for _, e := range set.Args() {
					if !(e.Kind == core.KName && (e.Name == "-inf" || e.Name == "+inf")) && !hasBoundIndex(e) {
						add(e)
					}
				}
			}
		}
	}
	var I []*core.Term
	for _, t := range idx {
		I = append(I, t)
	}
	var instances []*core.Term
	for _, sc := range schemas {
		k := len(sc.sets)
		n := 1
		for i := 0; i < k; i++ {
			n *= len(I)
		}
		if n > 4096 {
			return false // past the budget: not proven
		}
		ts := make([]*core.Term, k)
		var each func(i int)
		each = func(i int) {
			if i == k {
				instances = append(instances, lit(sc.body(append([]*core.Term(nil), ts...))))
				return
			}
			for _, t := range I {
				ts[i] = t
				each(i + 1)
			}
		}
		each(0)
	}
	all := append(append([]*core.Term(nil), ground...), instances...)
	// ACKERMANN: two reads of one table at equal indices are one value
	if !PlantReadsNotCongruent {
		var rds []*core.Term
		seen := map[string]bool{}
		for _, t := range append(append([]*core.Term(nil), all...), goal) {
			for _, rd := range r.reads(t) {
				if k := core.ShowTerm(rd); !seen[k] {
					seen[k] = true
					rds = append(rds, rd)
				}
			}
		}
		for i := 0; i < len(rds); i++ {
			for j := i + 1; j < len(rds); j++ {
				if core.ShowTerm(rds[i].Kids[0]) != core.ShowTerm(rds[j].Kids[0]) {
					continue
				}
				all = append(all, core.App(core.Name(core.ImpliesName),
					core.App(core.Name("="), rds[i].Kids[1], rds[j].Kids[1]),
					core.App(core.Name("="), rds[i], rds[j])))
			}
		}
	}
	return r.decideGround(all, goal, f)
}

// decideGround translates a ground sequent and decides it with the facts.
func (r *refiner) decideGround(hyps []*core.Term, goal *core.Term, f *facts) (proven bool) {
	// a coefficient past the word in the translation (a fact at ±2⁶³ on a kept
	// definition's word) is "not proven", as the decision's own overflow is
	defer func() {
		if e := recover(); e != nil {
			if !presburger.Overflowed(e) {
				panic(e)
			}
			proven = false
		}
	}()
	tr := &formulaTr{r: r, bound: map[string]bool{}, memo: map[string][2]string{}}
	g, ok := tr.formula(goal)
	if !ok {
		return false
	}
	var fs []*presburger.Formula
	for _, h := range hyps {
		hf, ok := tr.hypothesis(h)
		if !ok {
			continue // a hypothesis not read is one not assumed: sound
		}
		fs = append(fs, hf)
	}
	fs = append(fs, tr.global...)
	if !PlantFormulaHypsDropped {
		for _, l := range f.le {
			if small(l) {
				fs = append(fs, presburger.Ge(toLin(l).Scale(-1)))
			}
		}
		for name, l := range f.eq {
			if small(l) {
				fs = append(fs, presburger.EqF(presburger.Var(name).Add(toLin(l), -1)))
			}
		}
	}
	return presburger.Valid(fs, g) == presburger.Unsat
}

// conjunctsOf is a conjunction's conjuncts under their marks.
func conjunctsOf(t *core.Term) []*core.Term {
	if a, b, ok := conjunction(t); ok {
		return append(conjunctsOf(stripMarks(a)), conjunctsOf(stripMarks(b))...)
	}
	return []*core.Term{t}
}

// hypothesis is h translated, or not read when a coefficient passes the word
// (a guard against a parameter's bound ±2⁶³ on a kept definition): a
// hypothesis dropped is one not assumed, so the sequent is weaker and its
// answer "proven" still sound. The translator's scope is restored; the global
// definitions it made stay, each of a fresh name, a conservative extension.
func (tr *formulaTr) hypothesis(h *core.Term) (out *presburger.Formula, ok bool) {
	defs, vars, depth := tr.defs, tr.vars, tr.depth
	bound := map[string]bool{}
	for x, b := range tr.bound {
		bound[x] = b
	}
	defer func() {
		if e := recover(); e != nil {
			if !presburger.Overflowed(e) {
				panic(e)
			}
			tr.defs, tr.vars, tr.depth, tr.bound = defs, vars, depth, bound
			out, ok = nil, false
		}
	}()
	return tr.formula(h)
}

// isRead reports a table's read, (t e): t a name no primitive, mark or former
// is, applied to one index.
func (r *refiner) isRead(t *core.Term) bool {
	if t.Kind != core.KApp || len(t.Kids) != 2 || t.Kids[0].Kind != core.KName {
		return false
	}
	n := t.Kids[0].Name
	if core.IsFormulaHead(n) || n == core.RequireWhereName || n == core.RequireName || n == core.AscribeName || n == "if" || n == ".." {
		return false
	}
	if r.tgt != nil {
		if _, prim := r.tgt.Prims[n]; prim {
			return false
		}
	}
	return true
}

// reads are t's table reads, not under a binder.
func (r *refiner) reads(t *core.Term) []*core.Term {
	var out []*core.Term
	var walk func(u *core.Term)
	walk = func(u *core.Term) {
		if u == nil || u.Kind != core.KApp {
			return
		}
		if r.isRead(u) && !hasBoundIndex(u.Kids[1]) {
			out = append(out, u)
		}
		for _, k := range u.Kids {
			walk(k)
		}
	}
	walk(t)
	return out
}

// hasBoundIndex reports a de Bruijn index in t: a term under a binder not yet
// opened, which no instance is.
func hasBoundIndex(t *core.Term) bool {
	if t.Kind == core.KBound {
		return true
	}
	for _, k := range t.Kids {
		if hasBoundIndex(k) {
			return true
		}
	}
	return false
}

func mentionsRead(r *refiner, t *core.Term) bool {
	found := false
	var walk func(u *core.Term)
	walk = func(u *core.Term) {
		if u == nil || found {
			return
		}
		if u.Kind == core.KApp && len(u.Kids) == 2 && u.Kids[0].Kind == core.KName && r.isRead(u) {
			found = true
			return
		}
		for _, k := range u.Kids {
			walk(k)
		}
	}
	walk(t)
	return found
}

// literalReads rewrites a literal table read at an unknown index into the
// choice among its elements: ((array v₀ … vₙ₋₁) e) is if e = 0 then v₀ … else
// vₙ₋₁, the index's domain an obligation elsewhere (the read's own).
func (r *refiner) literalReads(t *core.Term) *core.Term {
	if t == nil || t.Kind != core.KApp {
		return t
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = r.literalReads(k)
	}
	out := &core.Term{Kind: t.Kind, Kids: kids}
	// a name bound to a literal table is that literal, x ≐ e (refine's letDefs)
	if len(kids) == 2 && kids[0].Kind == core.KName {
		if d, ok := r.letDefs[kids[0].Name]; ok && d.Kind == core.KApp && len(d.Kids) >= 2 && d.Kids[0].Kind == core.KName &&
			isOp(d.Kids[0].Name, "array") {
			kids = []*core.Term{d, kids[1]}
		}
	}
	// at a constant index inside it, the element itself (β-tab)
	if len(kids) == 2 && kids[0].Kind == core.KApp && len(kids[0].Kids) >= 2 && kids[0].Kids[0].Kind == core.KName &&
		isOp(kids[0].Kids[0].Name, "array") && kids[1].Kind == core.KInt && kids[1].Int >= 0 && kids[1].Int < int64(len(kids[0].Kids)-1) {
		return kids[0].Kids[1+kids[1].Int]
	}
	if len(kids) == 2 && kids[0].Kind == core.KApp && len(kids[0].Kids) >= 2 && kids[0].Kids[0].Kind == core.KName &&
		isOp(kids[0].Kids[0].Name, "array") && kids[1].Kind != core.KInt {
		elems := kids[0].Kids[1:]
		chain := elems[len(elems)-1]
		for j := len(elems) - 2; j >= 0; j-- {
			chain = core.App(core.Name("if"), core.App(core.Name("="), kids[1], core.Int(int64(j))), elems[j], chain)
		}
		return chain
	}
	return out
}

// small reports a fact whose constants are within ±2⁴⁰. A fact past that is a
// word's bound on a value (an element of Array(Int), ±2⁶³), which these
// sequents never need, and whose products overflow the Omega test's int64
// arithmetic, so the answer was Unknown: left out of Γ, which only weakens it,
// soundly.
func small(l *linear) bool {
	const lim = 1 << 40
	if l.konst > lim || l.konst < -lim {
		return false
	}
	for _, c := range l.coef {
		if c > lim || c < -lim {
			return false
		}
	}
	return true
}
