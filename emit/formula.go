package emit

// A CLAIM WITH QUANTIFIERS OR THE LOGIC'S CONNECTIVES (ADR 0145, stage 2;
// logic-research §2–§4.1) is decided here, as the sequent Γ ⊢ φ: Γ the facts in
// scope at the claim (the refinement layer's linear facts), φ the claim, both
// read as formulas of Presburger arithmetic and handed to presburger.Valid,
// which takes the quantifiers by polarity and the rest by the Omega test.
//
// THE TRANSLATION, term by term:
//   - a Boolean term: `if c a b` as (c ∧ a) ∨ (¬c ∧ b), so `&&`, `||` and `!`
//     as D wrote them; the four formers; a comparison of integer terms; a
//     Boolean the procedures do not interpret as a propositional atom;
//   - an integer term: + and − and a constant's ·; `a % k` and `a / k`, k a
//     positive constant, exactly, by the truncated division they are,
//     a = k·q + m with 0 ≤ m < k when a ≥ 0 and −k < m ≤ 0 when a < 0, q and m
//     fresh; any other term the variable the facts already name it by (so a
//     fact about `len(t)` and the claim's `len(t)` are one variable), or, where
//     the facts have no name, a variable keyed by its printed form, which
//     identifies equal terms only when they are written alike: sound,
//     incomplete until congruence closure (stage 4).
//
// A division's definition joins the formula at the atom it occurs in, under
// the atom's own quantifiers: ∃q, m. (a = k·q + m ∧ …) ∧ atom. The procedure
// eliminates or Skolemizes them by their polarity there.

import (
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/presburger"
)

// PlantLetDefsIgnored reads a let-bound name as a variable the logic knows
// nothing of: an assertion over m = lo + (hi − lo) / 2 is then refused
// (TestAClaimWithQuantifiersIsDecided).
var PlantLetDefsIgnored bool

// PlantArraysBypassed decides a formula as stage 2 did, its ∀ hypotheses left
// to Cooper's elimination with each read an unknown: a sorted table's property
// is then not proven (TestATablesPropertyIsDecided).
var PlantArraysBypassed bool

// PlantFormulaHypsDropped decides a claim with no facts in scope: a claim that
// holds only under a parameter's set is then refused (TestAClaimWithQuantifiersIsDecided).
var PlantFormulaHypsDropped bool

// formulaHolds decides Γ ⊢ cond, Γ the facts f, cond a claim holding a quantifier
// or a connective of the logic.
func (r *refiner) formulaHolds(cond *core.Term, f *facts) bool {
	// the sequent prepared on the program's terms and its schemas instantiated
	// over the index set (arrays.go, stage 3); what is left is the procedure below
	if !PlantArraysBypassed {
		return r.arrayHolds(cond, f)
	}
	tr := &formulaTr{r: r, bound: map[string]bool{}, memo: map[string][2]string{}}
	goal, ok := tr.formula(cond)
	if !ok {
		return false
	}
	// a division over free variables is defined once, among the hypotheses
	hyps := append([]*presburger.Formula(nil), tr.global...)
	if !PlantFormulaHypsDropped {
		for _, l := range f.le {
			hyps = append(hyps, presburger.Ge(toLin(l).Scale(-1))) // l ≤ 0
		}
		for name, l := range f.eq {
			hyps = append(hyps, presburger.EqF(presburger.Var(name).Add(toLin(l), -1)))
		}
	}
	return presburger.Valid(hyps, goal) == presburger.Unsat
}

func toLin(l *linear) presburger.Lin {
	out := presburger.Const(l.konst)
	for k, v := range l.coef {
		out = out.Add(presburger.Var(k), v)
	}
	return out
}

type formulaTr struct {
	r     *refiner
	n     int
	bound map[string]bool // the quantifiers' variables in scope
	// defs are the division definitions an integer term needs, collected for
	// the atom that holds it
	defs []*presburger.Formula
	vars []string
	// global are the definitions of the divisions over free variables only,
	// hypotheses of the sequent; memo their (q, m) by the division's text
	global []*presburger.Formula
	memo   map[string][2]string
	depth  int // let definitions unfolded, bounded
}

func (tr *formulaTr) fresh(stem string) string {
	tr.n++
	return fmt.Sprintf("#%s%d", stem, tr.n)
}

// formula reads a Boolean term as a formula.
func (tr *formulaTr) formula(t *core.Term) (*presburger.Formula, bool) {
	switch t.Kind {
	case core.KBool:
		if t.IsTrue() {
			return presburger.True, true
		}
		return presburger.False, true
	case core.KName:
		return presburger.Prop(t.Name), true
	case core.KApp:
	default:
		return nil, false
	}
	op := t.Op()
	args := t.Args()
	if op.Kind != core.KName {
		return presburger.Prop(core.ShowTerm(t)), true
	}
	name := op.Name
	switch {
	case (name == core.ForallName || name == core.ExistsName) && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
		x := tr.fresh("x")
		body := args[1].OpenWith([]*core.Term{core.Name(x)})
		tr.bound[x] = true
		inner, ok := tr.formula(body)
		if !ok {
			return nil, false
		}
		bounds, ok := tr.setBounds(args[0], x)
		if !ok {
			return nil, false
		}
		if name == core.ForallName {
			return presburger.Forall(x, presburger.Implies(bounds, inner)), true
		}
		return presburger.Exists(x, presburger.And(bounds, inner)), true
	case name == core.ImpliesName && len(args) == 2:
		a, ok1 := tr.formula(args[0])
		b, ok2 := tr.formula(args[1])
		return presburger.Implies(a, b), ok1 && ok2
	case name == core.IffName && len(args) == 2:
		a, ok1 := tr.formula(args[0])
		b, ok2 := tr.formula(args[1])
		return presburger.Iff(a, b), ok1 && ok2
	case isOp(name, "if") && len(args) == 3:
		c, ok1 := tr.formula(args[0])
		a, ok2 := tr.formula(args[1])
		b, ok3 := tr.formula(args[2])
		return presburger.Or(presburger.And(c, a), presburger.And(presburger.Not(c), b)), ok1 && ok2 && ok3
	case name == core.RequireWhereName && len(args) == 3:
		return tr.formula(args[2])
	case len(args) == 2 && cmpOp(name) != "" && hasIf(t):
		// AN ATOM OVER A CONDITIONAL IS THE CASE SPLIT, φ(if c a b) ⇔
		// (c ∧ φ(a)) ∨ (¬c ∧ φ(b)): `case` and `if` as values, clamp's body
		c, ta, tb := liftIf(t)
		fc, ok1 := tr.formula(c)
		fa, ok2 := tr.formula(ta)
		fb, ok3 := tr.formula(tb)
		return presburger.Or(presburger.And(fc, fa), presburger.And(presburger.Not(fc), fb)), ok1 && ok2 && ok3
	case len(args) == 2 && cmpOp(name) != "":
		saved, savedVars := tr.defs, tr.vars
		tr.defs, tr.vars = nil, nil
		a, ok1 := tr.term(args[0])
		b, ok2 := tr.term(args[1])
		if !ok1 || !ok2 {
			tr.defs, tr.vars = saved, savedVars
			return presburger.Prop(core.ShowTerm(t)), true
		}
		var atom *presburger.Formula
		d := b.Add(a, -1) // b − a
		switch cmpOp(name) {
		case "lt":
			atom = presburger.Ge(d.Add(presburger.Const(-1), 1))
		case "le":
			atom = presburger.Ge(d)
		case "gt":
			atom = presburger.Ge(d.Scale(-1).Add(presburger.Const(-1), 1))
		case "ge":
			atom = presburger.Ge(d.Scale(-1))
		case "eq":
			atom = presburger.EqF(d)
		}
		// the divisions' definitions, under their own ∃, at this atom
		if len(tr.defs) > 0 {
			atom = presburger.And(append(append([]*presburger.Formula(nil), tr.defs...), atom)...)
			for i := len(tr.vars) - 1; i >= 0; i-- {
				atom = presburger.Exists(tr.vars[i], atom)
			}
		}
		tr.defs, tr.vars = saved, savedVars
		return atom, true
	}
	return presburger.Prop(core.ShowTerm(t)), true
}

func cmpOp(name string) string {
	for _, c := range []string{"lt", "le", "gt", "ge", "eq"} {
		if isOp(name, c) {
			return c
		}
	}
	return ""
}

// setBounds is x ∈ S as a formula: a range's ends, or nothing for ℤ.
func (tr *formulaTr) setBounds(s *core.Term, x string) (*presburger.Formula, bool) {
	var lo, hi *core.Term
	switch {
	case s.Kind == core.KStr || s.Kind == core.KName:
		ty := s.Str
		if s.Kind == core.KName {
			ty = core.TypeName(s)
		}
		if ty == "int" || ty == "Int" {
			return presburger.True, true
		}
		l, h, ok := core.RangeEnds(ty)
		if !ok {
			return nil, false
		}
		var fs []*presburger.Formula
		if l != nil {
			if !l.IsInt64() {
				return nil, false
			}
			fs = append(fs, presburger.Ge(presburger.Var(x).Add(presburger.Const(l.Int64()), -1)))
		}
		if h != nil {
			if !h.IsInt64() {
				return nil, false
			}
			fs = append(fs, presburger.Ge(presburger.Const(h.Int64()).Add(presburger.Var(x), -1)))
		}
		return presburger.And(fs...), true
	case s.Kind == core.KApp && s.Op().Kind == core.KName && s.Op().Name == ".." && len(s.Args()) == 2:
		lo, hi = s.Args()[0], s.Args()[1]
	default:
		return nil, false
	}
	var fs []*presburger.Formula
	saved, savedVars := tr.defs, tr.vars
	tr.defs, tr.vars = nil, nil
	if !(lo.Kind == core.KName && lo.Name == "-inf") {
		l, ok := tr.term(lo)
		if !ok {
			return nil, false
		}
		fs = append(fs, presburger.Ge(presburger.Var(x).Add(l, -1)))
	}
	if !(hi.Kind == core.KName && hi.Name == "+inf") {
		h, ok := tr.term(hi)
		if !ok {
			return nil, false
		}
		fs = append(fs, presburger.Ge(h.Add(presburger.Var(x), -1)))
	}
	b := presburger.And(append(fs, tr.defs...)...)
	for i := len(tr.vars) - 1; i >= 0; i-- {
		b = presburger.Exists(tr.vars[i], b)
	}
	tr.defs, tr.vars = saved, savedVars
	return b, true
}

// term reads an integer term as a linear term, a division by a constant
// defined exactly in tr.defs.
func (tr *formulaTr) term(t *core.Term) (presburger.Lin, bool) {
	switch t.Kind {
	case core.KInt:
		return presburger.Const(t.Int), true
	case core.KName:
		// a let-bound pure name is its definition, x ≐ e, unfolded once deep
		if d, ok := tr.r.letDefs[t.Name]; ok && tr.depth < 8 && !PlantLetDefsIgnored {
			tr.depth++
			l, ok := tr.term(d)
			tr.depth--
			if ok {
				return l, true
			}
		}
		return presburger.Var(t.Name), true
	case core.KApp:
	default:
		return presburger.Lin{}, false
	}
	op := t.Op()
	args := t.Args()
	if op.Kind == core.KName {
		switch {
		case op.Name == core.AscribeName && len(args) == 2:
			return tr.term(args[1])
		case op.Name == core.RequireWhereName && len(args) == 3:
			return tr.term(args[2])
		case op.Name == core.RequireName && len(args) == 4:
			return tr.term(args[3])
		case isOp(op.Name, "add") && len(args) == 2:
			a, ok1 := tr.term(args[0])
			b, ok2 := tr.term(args[1])
			return a.Add(b, 1), ok1 && ok2
		case isOp(op.Name, "sub") && len(args) == 2:
			a, ok1 := tr.term(args[0])
			b, ok2 := tr.term(args[1])
			return a.Add(b, -1), ok1 && ok2
		case isOp(op.Name, "mul") && len(args) == 2:
			a, ok1 := tr.term(args[0])
			b, ok2 := tr.term(args[1])
			if !ok1 || !ok2 {
				return presburger.Lin{}, false
			}
			// a constant factor scales the other term; its coefficients stay
			// words, so a factor past int64 leaves the term unread (sound)
			switch {
			case len(a.Coef) == 0 && (a.K == nil || a.K.IsInt64()):
				return b.Scale(presburger.KInt64(a.K)), true
			case len(b.Coef) == 0 && (b.K == nil || b.K.IsInt64()):
				return a.Scale(presburger.KInt64(b.K)), true
			}
		case (isOp(op.Name, "div") || isOp(op.Name, "rem")) && len(args) == 2 && args[1].Kind == core.KInt && args[1].Int >= 1:
			a, ok := tr.term(args[0])
			if !ok {
				return presburger.Lin{}, false
			}
			k := args[1].Int
			key := fmt.Sprintf("%s/%d", core.ShowTerm(args[0]), k)
			free := !tr.mentionsBound(args[0])
			if qm, ok := tr.memo[key]; ok && free {
				if isOp(op.Name, "div") {
					return presburger.Var(qm[0]), true
				}
				return presburger.Var(qm[1]), true
			}
			q, m := tr.fresh("q"), tr.fresh("m")
			// THE FACTS' OWN NAME for this division, where they have one: a
			// `where n % 4 == 0` is assumed of the atom rem(n, 4), and the
			// definition must be of that variable, or the two never meet
			if free {
				if l, ok := asLinearIn(tr.r.pure, t); ok && l.konst == 0 && len(l.coef) == 1 {
					for v, c := range l.coef {
						if c == 1 {
							if isOp(op.Name, "div") {
								q = v
							} else {
								m = v
							}
						}
					}
				}
			}
			// a = k·q + m, the remainder's sign the dividend's (truncated)
			def := presburger.EqF(a.Add(presburger.Var(q), -k).Add(presburger.Var(m), -1))
			pos := presburger.And(presburger.Ge(a), presburger.Ge(presburger.Var(m)),
				presburger.Ge(presburger.Const(k-1).Add(presburger.Var(m), -1)))
			neg := presburger.And(presburger.Ge(a.Scale(-1).Add(presburger.Const(-1), 1)),
				presburger.Ge(presburger.Var(m).Scale(-1)), presburger.Ge(presburger.Var(m).Add(presburger.Const(k-1), 1)))
			if free {
				// over free variables only: a hypothesis, q and m Skolem constants
				tr.global = append(tr.global, def, presburger.Or(pos, neg))
				tr.memo[key] = [2]string{q, m}
			} else {
				tr.defs = append(tr.defs, def, presburger.Or(pos, neg))
				tr.vars = append(tr.vars, q, m)
			}
			if isOp(op.Name, "div") {
				return presburger.Var(q), true
			}
			return presburger.Var(m), true
		}
	}
	// ANY OTHER TERM is the variable the facts name it by, or its printed form
	if l, ok := asLinearIn(tr.r.pure, t); ok && !tr.mentionsBound(t) {
		return toLin(l), true
	}
	return presburger.Var("‹" + core.ShowTerm(t) + "›"), true
}

func (tr *formulaTr) mentionsBound(t *core.Term) bool {
	s := core.ShowTerm(t)
	for x := range tr.bound {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// hasIf reports a conditional among a term's integer subterms, not under a λ.
func hasIf(t *core.Term) bool {
	if t.Kind != core.KApp || len(t.Kids) == 0 {
		return false
	}
	for _, k := range t.Kids[1:] {
		if isIfTerm(k) || hasIf(k) {
			return true
		}
	}
	return false
}

func isIfTerm(t *core.Term) bool {
	return t.Kind == core.KApp && len(t.Kids) == 4 && t.Kids[0].Kind == core.KName && isOp(t.Kids[0].Name, "if")
}

// liftIf finds the first conditional (if c a b) among t's subterms and gives
// its condition and t with it replaced by a, and by b.
func liftIf(t *core.Term) (c, ta, tb *core.Term) {
	var walk func(u *core.Term) (*core.Term, *core.Term, *core.Term, bool)
	walk = func(u *core.Term) (*core.Term, *core.Term, *core.Term, bool) {
		if u.Kind != core.KApp || len(u.Kids) == 0 {
			return nil, nil, nil, false
		}
		for i, k := range u.Kids {
			if i == 0 {
				continue
			}
			if isIfTerm(k) {
				ka := *u
				ka.Kids = append([]*core.Term(nil), u.Kids...)
				kb := ka
				kb.Kids = append([]*core.Term(nil), u.Kids...)
				ka.Kids[i], kb.Kids[i] = k.Kids[2], k.Kids[3]
				return k.Kids[1], &ka, &kb, true
			}
			if cc, a, b, ok := walk(k); ok {
				ka := *u
				ka.Kids = append([]*core.Term(nil), u.Kids...)
				kb := ka
				kb.Kids = append([]*core.Term(nil), u.Kids...)
				ka.Kids[i], kb.Kids[i] = a, b
				return cc, &ka, &kb, true
			}
		}
		return nil, nil, nil, false
	}
	c, ta, tb, _ = walk(t)
	return c, ta, tb
}

// pureTerm reports a term every operator of which is a primitive declared
// `pure`, or a claim's mark: a value the logic may read by its definition.
// Anything else (a host's effect, a λ) is no definition the logic unfolds.
func (r *refiner) pureTerm(t *core.Term) bool {
	switch t.Kind {
	case core.KInt, core.KName, core.KBool, core.KStr, core.KDec:
		return true
	case core.KApp:
	default:
		return false
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	switch op.Name {
	case core.RequireWhereName, core.RequireName, core.AscribeName:
	default:
		if r.tgt == nil {
			return false
		}
		p, ok := r.tgt.Prims[op.Name]
		if !ok || !p.Pure {
			return false
		}
	}
	for _, a := range t.Args() {
		if !r.pureTerm(a) {
			return false
		}
	}
	return true
}
