package emit

import (
	_ "embed"
	"fmt"
	"strings"

	"oroboros/core"
)

// FACTS ARE AXIOM SCHEMATA OF A LOCAL THEORY EXTENSION (spec/theories.md §7).
//
// A fact is  ∀x̄. G₁(x̄) ∧ … ∧ Gₘ(x̄) → C(x̄)  over the linear fragment extended by
// one uninterpreted TRIGGER term. The refinement layer used to carry each such
// law as Go — `seedDivAxioms` was F1 and F2 — and a law written in the compiler
// is a law nobody can read, override, or check against the others. Here it is
// data, and instantiation is one algorithm for all of them.
//
// THE ALGORITHM IS INSTANTIATION ON PRESENT TERMS, TO A FIXPOINT. For every
// subterm s of the program matching a fact's trigger by σ, if the facts already
// assumed entail σG, assume σC. That is complete for a local extension
// (Ihlemann, Jacobs & Sofronie-Stokkermans, TACAS 2008) and it TERMINATES,
// because an instance adds inequalities and never a term: the candidate set is
// the finite set of (fact, subterm) pairs, and each is used at most once. A
// guard not yet entailed may become entailed by another instance — `div-floor`
// needs `len-nonneg` first — which is why it is a fixpoint and not one pass.
//
// THE GUARD MUST BE ENTAILED, NOT MERELY CONSISTENT: an implication with an
// unproven premise says nothing, and one false assumption makes a conjunctive
// fragment derive everything (postconditions.md §4, Lemma 1).

//go:embed lang-facts.oro
var langFactsSrc string

// langFacts is `lang`'s theory. A package variable so a test can put a WEAKENED
// theory in its place and watch a proof fail — every fact needs such a witness
// (theories.md §7.8).
var langFacts = mustFacts(langFactsSrc)

// Fact is one admitted F-B schema.
type Fact struct {
	Name    string
	Params  map[string]bool
	Trigger *core.Term   // the one extension term, parameters as names
	Guards  []*core.Term // conjoined
	Concl   *core.Term
}

func mustFacts(src string) []*Fact {
	fs, err := readFacts(src)
	if err != nil {
		panic("lang's facts do not load: " + err.Error())
	}
	return fs
}

func readFacts(src string) ([]*Fact, error) {
	terms, err := core.ReadAll(src)
	if err != nil {
		return nil, err
	}
	var out []*Fact
	for _, t := range terms {
		f, err := admitFact(t)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// admitFact reads `(fact NAME ((x τ)…) [(when φ…)] φ)` and decides §7.4's five
// conditions, refusing with the one that failed.
func admitFact(t *core.Term) (*Fact, error) {
	if formWord(t) == "lemma" {
		return nil, fmt.Errorf("`lemma` is F-E, reserved (facts.md §5)")
	}
	if formWord(t) != "fact" || len(t.Kids) < 4 || len(t.Kids) > 5 ||
		t.Kids[1].Kind != core.KName || t.Kids[2].Kind != core.KApp {
		return nil, fmt.Errorf("(fact NAME ((x τ)…) [(when φ…)] φ), got %s", t)
	}
	f := &Fact{Name: t.Kids[1].Name, Params: map[string]bool{}}
	for _, p := range t.Kids[2].Kids {
		if p.Kind != core.KApp || len(p.Kids) != 2 || p.Kids[0].Kind != core.KName {
			return nil, fmt.Errorf("fact %s: a parameter is (x τ), got %s", f.Name, p)
		}
		switch ty := core.TypeName(p.Kids[1]); {
		case ty == "f64":
			return nil, fmt.Errorf("fact %s: condition 5 — a float fact is refused: IEEE "+
				"arithmetic is not an ordered field (facts.md §8, ADR 0009)", f.Name)
		case ty == "":
			return nil, fmt.Errorf("fact %s: %s is not a type", f.Name, p.Kids[1])
		}
		f.Params[p.Kids[0].Name] = true
	}
	if len(t.Kids) == 5 {
		if formWord(t.Kids[3]) != "when" {
			return nil, fmt.Errorf("fact %s: guards are (when φ…), got %s", f.Name, t.Kids[3])
		}
		f.Guards = t.Kids[3].Kids[1:]
	}
	f.Concl = t.Kids[len(t.Kids)-1]
	if strings.Contains(f.Concl.String(), "(forall ") {
		return nil, fmt.Errorf("fact %s: a quantified fact is F-D, reserved (facts.md §5)", f.Name)
	}

	// A CONSTANT PARAMETER is one in a literal-only position: the divisor of `/`,
	// which the linear fragment reads as an atom only when it is a positive
	// literal (asLinear). So a match can bind it only to a literal, and `(* k …)`
	// is multiplication by a constant.
	constant := map[string]bool{}
	var constants func(*core.Term)
	constants = func(x *core.Term) {
		if x.Kind != core.KApp {
			return
		}
		if canonOp(x) == "div" && len(x.Args()) == 2 && x.Args()[1].Kind == core.KName &&
			f.Params[x.Args()[1].Name] {
			constant[x.Args()[1].Name] = true
		}
		for _, k := range x.Kids {
			constants(k)
		}
	}
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		constants(g)
	}

	ext := map[string]*core.Term{}
	var nested bool
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		collectExtensions(g, constant, ext, &nested)
	}
	switch {
	case nested:
		return nil, fmt.Errorf("fact %s: condition 2 — an extension term occurs inside another, "+
			"so the fact is not flat", f.Name)
	case len(ext) > 1:
		return nil, fmt.Errorf("fact %s: a fact relating two terms is F-C, reserved (facts.md §5)", f.Name)
	case len(ext) == 0:
		return nil, fmt.Errorf("fact %s: condition 1 — no trigger term; a fact about the linear "+
			"fragment alone is already decided by it", f.Name)
	}
	for _, x := range ext {
		f.Trigger = x
	}
	for p := range f.Params {
		if !occursName(f.Trigger, p) {
			return nil, fmt.Errorf("fact %s: condition 3 — parameter %s does not occur in the trigger "+
				"%s, so an instance is not determined by a term already present", f.Name, p, f.Trigger)
		}
	}
	// Condition 4: linear once the trigger is READ AS AN ATOM — replaced by one —
	// and constants are literals. Asking `asLinear` to recognise the trigger
	// instead would accept `/` by a literal, which it happens to atomise, and
	// refuse `%`, which it does not.
	probe := map[string]*core.Term{}
	for p := range f.Params {
		if constant[p] {
			probe[p] = core.Int(2)
		}
	}
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		if _, ok := obligation(core.Rename2(substTerm(g, f.Trigger, core.Name(atomName)), probe)); !ok {
			return nil, fmt.Errorf("fact %s: condition 4 — %s is not linear once %s is an atom",
				f.Name, g, f.Trigger)
		}
	}
	return f, nil
}

// canonOp is an application's operator as the fragment names it: `len` for
// every spelling of a length, the alias for a host operator, or the name.
func canonOp(x *core.Term) string {
	if x.Kind != core.KApp || x.Op().Kind != core.KName {
		return ""
	}
	n := x.Op().Name
	if isLenOp(n) {
		return "len"
	}
	if i := strings.LastIndex(n, "."); i >= 0 {
		n = n[i+1:]
	}
	if a, ok := opAlias[n]; ok {
		return a
	}
	return n
}

// collectExtensions finds the terms outside the linear fragment (theories.md
// §7.3): what is not a comparison, a conjunction, `+`, `−`, or `*` by a literal
// or a constant parameter.
func collectExtensions(x *core.Term, constant map[string]bool, ext map[string]*core.Term, nested *bool) {
	if x.Kind != core.KApp {
		return
	}
	lit := func(a *core.Term) bool { return a.Kind == core.KInt || (a.Kind == core.KName && constant[a.Name]) }
	args := x.Args()
	switch op := canonOp(x); {
	case op == "le" || op == "lt" || op == "ge" || op == "gt" || op == "eq" || op == "<=" ||
		op == "<" || op == ">=" || op == ">" || op == "=" || op == "and" || op == "if" ||
		op == "add" || op == "sub":
		for _, a := range args {
			collectExtensions(a, constant, ext, nested)
		}
	case op == "mul" && len(args) == 2 && (lit(args[0]) || lit(args[1])):
		for _, a := range args {
			collectExtensions(a, constant, ext, nested)
		}
	default:
		ext[x.String()] = x
		inner := map[string]*core.Term{}
		for _, a := range args {
			collectExtensions(a, constant, inner, nested)
		}
		if len(inner) > 0 {
			*nested = true
		}
	}
}

func occursName(t *core.Term, name string) bool {
	if t.Kind == core.KName {
		return t.Name == name
	}
	for _, k := range t.Kids {
		if occursName(k, name) {
			return true
		}
	}
	return false
}

// matchFact is first-order matching of a trigger against a subterm, operators
// compared as the fragment names them so one fact serves every host's spelling.
func matchFact(pat, t *core.Term, params map[string]bool, sigma map[string]*core.Term) bool {
	switch {
	case pat.Kind == core.KName && params[pat.Name]:
		if bound, ok := sigma[pat.Name]; ok {
			return bound.String() == t.String()
		}
		sigma[pat.Name] = t
		return true
	case pat.Kind == core.KInt:
		return t.Kind == core.KInt && t.Int == pat.Int
	case pat.Kind == core.KApp:
		if t.Kind != core.KApp || len(t.Kids) != len(pat.Kids) || canonOp(pat) == "" ||
			canonOp(pat) != canonOp(t) {
			return false
		}
		for i := 1; i < len(pat.Kids); i++ {
			if !matchFact(pat.Kids[i], t.Kids[i], params, sigma) {
				return false
			}
		}
		return true
	}
	return false
}

// seedFacts instantiates a theory on the terms of t, to a fixpoint.
func seedFacts(f *facts, t *core.Term, theory []*Fact) {
	var subterms []*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KFn {
			walk(x.Body())
			return
		}
		if x.Kind == core.KApp {
			subterms = append(subterms, x)
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	used := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, fact := range theory {
			for _, s := range subterms {
				sigma := map[string]*core.Term{}
				if !matchFact(fact.Trigger, s, fact.Params, sigma) {
					continue
				}
				key := fact.Name + " " + s.String()
				if used[key] || !instanceHolds(f, fact, sigma) {
					continue
				}
				goals, ok := f.oblig(core.Rename2(fact.Concl, sigma))
				if !ok {
					continue // not linear at this instance: assume nothing
				}
				used[key] = true
				changed = true
				for _, g := range goals {
					f.assumeLE(g, fmt.Sprintf("assumed %s <= 0 (fact %s)", g, fact.Name))
				}
			}
		}
	}
}

// atomName stands for a fact's trigger when its guards and conclusion are read
// as linear forms. `#` is not an identifier character, so no program term can
// name it.
const atomName = "#T"

// substTerm replaces every occurrence of a subterm, compared by printed form.
func substTerm(x, from, to *core.Term) *core.Term {
	if x.String() == from.String() {
		return to
	}
	if x.Kind != core.KApp {
		return x
	}
	out := *x
	out.Kids = make([]*core.Term, len(x.Kids))
	for i, k := range x.Kids {
		out.Kids[i] = substTerm(k, from, to)
	}
	return &out
}

// inducedTransfer is THEOREM T (facts.md §3.3), MADE COMPLETE BY CASE SPLITTING:
// the interval of `op(args)` induced by every fact whose trigger is `op`.
//
// Theorem T applies a fact only where its guard holds on ALL of γ(X̄), and a
// guard like 1 ≤ b holds on none of [−7, 7] — so a meet of facts alone would be
// ⊤ where the remainder is plainly under 7. The repair is exact rather than a
// heuristic. Each single-variable guard c·x + k ≤ 0 is a half-line, so it CUTS
// x's interval at one point; cutting every argument at every guard's point
// partitions the box into cells on each of which every guard is decided. Then
//
//	⟦op⟧#(X̄)  =  ⊔_{cells C}  ⊓_{facts whose guards hold on C}  [lo s#(C), hi t#(C)]
//
// is sound — each meet is Theorem T on C, and the cells cover γ(X̄) — and it
// never loses a clause to a guard that holds on part of the box.
//
// `divisor` names the argument on which op is UNDEFINED at 0 (lang's semantics
// of division, integers.md §5, facts.md class S): its cell {0} contributes no
// value. −1 for none.
func inducedTransfer(theory []*Fact, op string, args []ival, divisor int) ival {
	type clause struct {
		pos    map[string]int
		guards []*linear
		bounds []*linear
	}
	var clauses []clause
	cuts := make([]map[int64]bool, len(args))
	for i := range cuts {
		cuts[i] = map[int64]bool{}
	}
	if divisor >= 0 && divisor < len(args) {
		cuts[divisor][-1], cuts[divisor][0] = true, true
	}
	for _, f := range theory {
		tr := f.Trigger
		if canonOp(tr) != op || len(tr.Args()) != len(args) {
			continue
		}
		c := clause{pos: map[string]int{}}
		ok := true
		for i, a := range tr.Args() {
			if a.Kind != core.KName || !f.Params[a.Name] {
				ok = false
				break
			}
			c.pos[a.Name] = i
		}
		for _, g := range f.Guards {
			goals, gok := obligation(g)
			ok = ok && gok
			for _, l := range goals {
				c.guards = append(c.guards, l)
				if len(l.coef) != 1 {
					continue
				}
				for name, k := range l.coef {
					if p, in := c.pos[name]; in {
						cuts[p][cutPoint(k, l.konst)] = true
					}
				}
			}
		}
		bounds, bok := obligation(substTerm(f.Concl, tr, core.Name(atomName)))
		if !ok || !bok {
			continue
		}
		c.bounds = bounds
		clauses = append(clauses, c)
	}

	// A ⊥ ARGUMENT IS READ AS ⊤. Strictness, f(⊥) = ⊥, is right only where ⊥
	// means "no value", and in this analysis ⊥ also stands for a value NOT YET
	// KNOWN — an operand before its loop's fixpoint has risen. The hand-written
	// transfers these replace never answered ⊥ for a ⊥ operand, and nothing has
	// measured whether their consumers tolerate one, so the induced transfer keeps
	// that behaviour: a fact's conclusion that does not mention the operand
	// (`and-right` bounds a&b by b for every a) still gives its bound. Reading ⊥ as
	// ⊤ can only widen, so it is sound.
	cells := make([][]ival, len(args))
	for i, a := range args {
		if a.isBottom() {
			a = top
		}
		cells[i] = splitAt(a, cuts[i])
	}
	result, any, defined := ival{}, false, false
	cell := make([]ival, len(args))
	var visit func(int)
	visit = func(i int) {
		if i < len(args) {
			for _, c := range cells[i] {
				cell[i] = c
				visit(i + 1)
			}
			return
		}
		if divisor >= 0 && divisor < len(args) && cell[divisor] == exact(0) {
			return // undefined: no value to contain
		}
		defined = true
		v := top
		for _, c := range clauses {
			if !allHold(c.guards, cell, c.pos) {
				continue
			}
			for _, b := range c.bounds {
				v = tighten(v, b, cell, c.pos)
			}
		}
		if v.isBottom() {
			return
		}
		if !any {
			result, any = v, true
		} else {
			result = joinIval(result, v)
		}
	}
	visit(0)
	switch {
	case !defined:
		// EVERY CELL IS UNDEFINED — the divisor is exactly 0 — so the application
		// has no value, and the exact abstraction of the empty set is ⊥. It is
		// unreachable in an emitted program, whose division by zero is refused.
		return bottom
	case !any:
		return top
	}
	return result
}

// cutPoint is where the half-line c·x + k ≤ 0 begins or ends: the cell boundary
// t such that the guard is decided on x ≤ t and on x ≥ t+1.
func cutPoint(c, k int64) int64 {
	if c > 0 { // x ≤ ⌊−k/c⌋
		return floorDiv(-k, c)
	}
	return ceilDiv(k, -c) - 1 // x ≥ ⌈k/|c|⌉
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func ceilDiv(a, b int64) int64 { return -floorDiv(-a, b) }

// floorDivB and ceilDivB are the same on endpoints. The quotient of two
// endpoints is at most the dividend in magnitude, so it is always one, and the
// correction by one cannot leave the interval either.
func floorDivB(a, b bnd) bnd {
	q := a.quo(b)
	back, _ := q.mul(b)
	if back != a && (a.sign() < 0) != (b.sign() < 0) {
		q, _ = q.sub(bOne)
	}
	return q
}

func ceilDivB(a, b bnd) bnd { return floorDivB(a.neg(), b).neg() }

// splitAt partitions an interval at every cut strictly inside it.
func splitAt(v ival, cuts map[int64]bool) []ival {
	if v.isBottom() {
		return nil
	}
	ts := make([]int64, 0, len(cuts))
	for t := range cuts {
		ts = append(ts, t)
	}
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j] < ts[j-1]; j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
	var out []ival
	cur := v
	for _, t := range ts {
		above := cur.loInf || bi(t).ge(cur.lo)
		below := cur.hiInf || bi(t).lt(cur.hi)
		if !above || !below {
			continue
		}
		out = append(out, ival{lo: cur.lo, loInf: cur.loInf, hi: bi(t)})
		cur = ival{lo: bi(t + 1), hi: cur.hi, hiInf: cur.hiInf}
	}
	return append(out, cur)
}

// evalLinear is interval evaluation of a linear form over a cell (Moore).
func evalLinear(l *linear, cell []ival, pos map[string]int) (ival, bool) {
	out := exact(l.konst)
	for name, c := range l.coef {
		p, in := pos[name]
		if !in {
			return top, false
		}
		out = addI(out, scaleI(c, cell[p]))
	}
	return out, true
}

// scaleI is c·X for a CONSTANT c. Not mulI: interval multiplication gives up on
// an unbounded factor, which is right for two unknowns and wrong for a constant,
// where x ↦ c·x is monotone and maps a half-line to a half-line. A guard like
// a ≤ 0 on (−∞, −1] is exactly that shape, and evaluating it as ⊤ dropped the
// dividend's sign clause from every cell with an unbounded dividend.
func scaleI(c int64, x ival) ival {
	if c == 0 {
		return exact(0)
	}
	end := func(v bnd, inf bool) (bnd, bool) {
		if inf {
			return bZero, true
		}
		p, ok := bi(c).mul(v)
		if !ok {
			return bZero, true // saturates to an infinity, which is sound
		}
		return p, false
	}
	lo, loInf := end(x.lo, x.loInf)
	hi, hiInf := end(x.hi, x.hiInf)
	if c < 0 {
		lo, hi, loInf, hiInf = hi, lo, hiInf, loInf
	}
	return ival{lo: lo, hi: hi, loInf: loInf, hiInf: hiInf}
}

// allHold decides each guard l ≤ 0 on every point of the cell.
func allHold(guards []*linear, cell []ival, pos map[string]int) bool {
	for _, g := range guards {
		v, ok := evalLinear(g, cell, pos)
		if !ok || v.hiInf || v.hi.sign() > 0 {
			return false
		}
	}
	return true
}

// tighten meets v with the bound c·T + rest ≤ 0 on the trigger T.
func tighten(v ival, b *linear, cell []ival, pos map[string]int) ival {
	c := b.coef[atomName]
	if c == 0 {
		return v
	}
	rest := b.clone()
	delete(rest.coef, atomName)
	r, ok := evalLinear(rest, cell, pos)
	if !ok {
		return v
	}
	if c > 0 { // T ≤ −rest / c
		if !r.loInf {
			if hi := floorDivB(r.lo.neg(), bi(c)); v.hiInf || hi.lt(v.hi) {
				v.hi, v.hiInf = hi, false
			}
		}
		return v
	}
	if !r.loInf { // T ≥ rest / |c|
		if lo := ceilDivB(r.lo, bi(-c)); v.loInf || lo.gt(v.lo) {
			v.lo, v.loInf = lo, false
		}
	}
	return v
}

func joinIval(a, b ival) ival {
	out := a
	if b.loInf || (!out.loInf && b.lo.lt(out.lo)) {
		out.lo, out.loInf = b.lo, b.loInf
	}
	if b.hiInf || (!out.hiInf && b.hi.gt(out.hi)) {
		out.hi, out.hiInf = b.hi, b.hiInf
	}
	return out
}

// instanceHolds reports whether every guard of σ(fact) is ENTAILED.
func instanceHolds(f *facts, fact *Fact, sigma map[string]*core.Term) bool {
	for _, g := range fact.Guards {
		goals, ok := f.oblig(core.Rename2(g, sigma))
		if !ok {
			return false
		}
		for _, goal := range goals {
			if !f.entails(goal) {
				return false
			}
		}
	}
	return true
}
