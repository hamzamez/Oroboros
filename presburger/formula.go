package presburger

// FORMULAS OF PRESBURGER ARITHMETIC, AND THEIR VALIDITY (ADR 0145, logic-research §3–§4.1).
//
// A Formula is built from linear atoms over ℤ, t ≥ 0, t = 0 and d | t (d a
// positive constant), propositional atoms (a Boolean the procedures do not
// interpret), the connectives ¬ ∧ ∨ and the quantifiers ∃ and ∀ over ℤ.
//
// VALIDITY IS UNSATISFIABILITY OF THE NEGATION: hyps ⊢ goal iff hyps ∧ ¬goal
// has no model. The negation is put in negation normal form, and then the
// quantifiers are taken by polarity:
//   - AN ∃ NOT UNDER A ∀ is Skolemized: renamed fresh and dropped, its variable
//     free (Theorem S: Γ ∧ ∃x. ψ(x) is satisfiable iff Γ ∧ ψ(c) is, c fresh);
//   - EVERY OTHER QUANTIFIER is eliminated by Cooper's algorithm ("Theorem
//     proving in arithmetic without multiplication", 1972), ∀x. φ as ¬∃x. ¬φ,
//     innermost first, each giving a quantifier-free formula equivalent over ℤ.
//
// The quantifier-free result is put in disjunctive normal form, and each
// conjunct decided by the Omega test (Satisfiable): d | t is t = d·q, and its
// negation t = d·q + m with 1 ≤ m ≤ d − 1, q and m fresh. A propositional atom
// is consistent unless the conjunct holds it and its negation.
//
// Unknown (an overflow, a budget) is never valid: a claim is proven or the
// program is refused.

import (
	"fmt"
	"sort"
	"strings"
)

// Lin is the linear term Σ Coef[x]·x + K.
type Lin struct {
	Coef map[string]int64
	K    int64
}

// Const is the constant term k.
func Const(k int64) Lin { return Lin{Coef: map[string]int64{}, K: k} }

// Var is the term x.
func Var(x string) Lin { return Lin{Coef: map[string]int64{x: 1}} }

func (a Lin) clone() Lin {
	c := Lin{Coef: make(map[string]int64, len(a.Coef)), K: a.K}
	for k, v := range a.Coef {
		c.Coef[k] = v
	}
	return c
}

// Add is a + b·s, checked.
func (a Lin) Add(b Lin, s int64) Lin {
	c := a.clone()
	for k, v := range b.Coef {
		c.Coef[k] = add(c.Coef[k], mul(v, s))
		if c.Coef[k] == 0 {
			delete(c.Coef, k)
		}
	}
	c.K = add(c.K, mul(b.K, s))
	return c
}

// Scale is s·a, checked.
func (a Lin) Scale(s int64) Lin { return Const(0).Add(a, s) }

func (a Lin) String() string {
	var ks []string
	for k := range a.Coef {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%d·%s", a.Coef[k], k))
	}
	parts = append(parts, fmt.Sprint(a.K))
	return strings.Join(parts, " + ")
}

// Kind is a formula's former.
type Kind int

const (
	FTrue Kind = iota
	FFalse
	FGe    // T ≥ 0
	FEq    // T = 0
	FDiv   // D | T
	FNDiv  // ¬(D | T), in negation normal form only
	FProp  // a propositional atom, Name
	FNProp // its negation, in negation normal form only
	FNot
	FAnd
	FOr
	FExists // ∃ Var ∈ ℤ : Xs[0]
	FForall // ∀ Var ∈ ℤ : Xs[0]
)

// Formula is a formula of Presburger arithmetic with propositional atoms.
type Formula struct {
	Kind Kind
	T    Lin
	D    int64
	Name string
	Var  string
	Xs   []*Formula
}

var (
	True  = &Formula{Kind: FTrue}
	False = &Formula{Kind: FFalse}
)

// Ge is t ≥ 0.
func Ge(t Lin) *Formula { return &Formula{Kind: FGe, T: t} }

// Eq is t = 0.
func EqF(t Lin) *Formula { return &Formula{Kind: FEq, T: t} }

// Div is d | t, d ≥ 1.
func Div(d int64, t Lin) *Formula { return &Formula{Kind: FDiv, D: d, T: t} }

// Prop is the propositional atom n.
func Prop(n string) *Formula { return &Formula{Kind: FProp, Name: n} }

// Not is ¬f.
func Not(f *Formula) *Formula { return &Formula{Kind: FNot, Xs: []*Formula{f}} }

// And is the conjunction of fs, ⊤ when there are none.
func And(fs ...*Formula) *Formula { return &Formula{Kind: FAnd, Xs: fs} }

// Or is the disjunction of fs, ⊥ when there are none.
func Or(fs ...*Formula) *Formula { return &Formula{Kind: FOr, Xs: fs} }

// Exists is ∃x ∈ ℤ : f.
func Exists(x string, f *Formula) *Formula { return &Formula{Kind: FExists, Var: x, Xs: []*Formula{f}} }

// Forall is ∀x ∈ ℤ : f.
func Forall(x string, f *Formula) *Formula { return &Formula{Kind: FForall, Var: x, Xs: []*Formula{f}} }

// Implies is ¬a ∨ b.
func Implies(a, b *Formula) *Formula { return Or(Not(a), b) }

// Iff is (¬a ∨ b) ∧ (a ∨ ¬b).
func Iff(a, b *Formula) *Formula { return And(Or(Not(a), b), Or(a, Not(b))) }

// Valid decides hyps ⊢ goal over ℤ: Unsat when it is valid (no model of hyps ∧
// ¬goal), Sat when a countermodel exists, Unknown when the work overflowed or
// ran past its budget.
func Valid(hyps []*Formula, goal *Formula) Result {
	return Satisfies(And(append(append([]*Formula(nil), hyps...), Not(goal))...))
}

// PlantSkolemShared names every Skolem constant alike, so two existentials
// share one witness: a ∀ goal over two quantifiers is then believed from one
// instance (TestAFormulaIsDecidedByPolarity must catch it).
var PlantSkolemShared bool

// PlantCooperNoBounds drops the lower-bound instances from Cooper's
// elimination, keeping only φ at −∞: ∃x. x = c is then false.
var PlantCooperNoBounds bool

// Satisfies decides whether f has a model over ℤ.
func Satisfies(f *Formula) (r Result) {
	defer func() {
		if e := recover(); e != nil {
			if e == errOverflow || e == errBudget {
				r = Unknown
				return
			}
			panic(e)
		}
	}()
	d := &decider{}
	g := d.nnf(f, false)
	g = d.skolem(g)
	g = d.qe(g)
	return d.search([]*Formula{simplify(g)}, nil)
}

// literals is f as a conjunction of literals, a literal alone a conjunction of
// one; false when f holds a disjunction or a quantifier.
func literals(f *Formula) ([]*Formula, bool) {
	if literal(f) {
		return []*Formula{f}, true
	}
	if f.Kind != FAnd {
		return nil, false
	}
	var out []*Formula
	for _, x := range f.Xs {
		ls, ok := literals(x)
		if !ok {
			return nil, false
		}
		out = append(out, ls...)
	}
	return out, true
}

// literal reports an atom or a negated one.
func literal(f *Formula) bool {
	switch f.Kind {
	case FGe, FEq, FDiv, FNDiv, FProp, FNProp:
		return true
	}
	return false
}

// search decides a quantifier-free NNF formula by case splitting with pruning,
// DPLL's search without its learning (Davis, Logemann and Loveland 1962): the
// pending formulas are taken apart, conjunctions and literals before any
// disjunction, each literal added to the branch's conjunction, and the branch
// cut the moment the Omega test finds that conjunction unsatisfiable. A
// disjunction is tried disjunct by disjunct. Sat when a branch closes with no
// pending formula and its conjunction satisfiable.
func (d *decider) search(pending []*Formula, lits []*Formula) Result {
	for {
		d.tick()
		// ABSORB conjunctions and literals, which never branch, each literal
		// checked as it joins, so a contradiction closes the branch at once
		for {
			i := -1
			for j, f := range pending {
				if f.Kind != FOr {
					i = j
					break
				}
			}
			if i < 0 {
				break
			}
			f := pending[i]
			pending = append(append([]*Formula(nil), pending[:i]...), pending[i+1:]...)
			switch f.Kind {
			case FTrue:
			case FFalse:
				return Unsat
			case FAnd:
				pending = append(pending, f.Xs...)
			default:
				lits = append(lits, f)
				if r := d.sat(lits); r != Sat {
					return r
				}
			}
		}
		if len(pending) == 0 {
			return Sat
		}
		// PROPAGATION (DPLL's unit rule, the Omega test the theory): a
		// disjunction the branch already satisfies, one of its literal
		// disjuncts entailed, is dropped; a disjunct the branch contradicts is
		// removed; with none left the branch closes, with one left it is taken
		// without branching
		var keep, units []*Formula
		for _, f := range pending {
			var live []*Formula
			satisfied := false
			for _, x := range f.Xs {
				parts, ok := literals(x)
				if !ok {
					live = append(live, x)
					continue
				}
				if d.sat(append(append([]*Formula(nil), lits...), parts...)) == Unsat {
					continue
				}
				if len(parts) == 1 {
					if neg := d.nnf(parts[0], true); literal(neg) && d.sat(append(append([]*Formula(nil), lits...), neg)) == Unsat {
						satisfied = true
						break
					}
				}
				live = append(live, x)
			}
			switch {
			case satisfied:
			case len(live) == 0:
				return Unsat
			case len(live) == 1:
				units = append(units, live[0])
			default:
				keep = append(keep, &Formula{Kind: FOr, Xs: live})
			}
		}
		pending = append(keep, units...)
		if len(units) == 0 {
			sort.SliceStable(pending, func(i, j int) bool { return len(pending[i].Xs) < len(pending[j].Xs) })
			break
		}
	}
	if len(pending) == 0 {
		return d.sat(lits)
	}
	// a disjunction: each disjunct in turn, the rest pending
	f := pending[0]
	rest := pending[1:]
	unknown := false
	for _, x := range f.Xs {
		next := append(append([]*Formula(nil), rest...), x)
		switch d.search(next, append([]*Formula(nil), lits...)) {
		case Sat:
			return Sat
		case Unknown:
			unknown = true
		}
	}
	if unknown {
		return Unknown
	}
	return Unsat
}

type decider struct {
	n     int
	steps int
}

func (d *decider) fresh(stem string) string {
	d.n++
	if PlantSkolemShared && stem == "#sk" {
		return stem
	}
	return fmt.Sprintf("%s%d", stem, d.n)
}

func (d *decider) tick() {
	d.steps++
	if d.steps > Budget {
		panic(errBudget)
	}
}

// nnf is f, or ¬f when neg, in negation normal form: negation only on atoms,
// ¬(t ≥ 0) as −t − 1 ≥ 0, ¬(t = 0) as t ≥ 1 ∨ −t ≥ 1.
func (d *decider) nnf(f *Formula, neg bool) *Formula {
	d.tick()
	switch f.Kind {
	case FTrue:
		if neg {
			return False
		}
		return True
	case FFalse:
		if neg {
			return True
		}
		return False
	case FGe:
		if neg {
			return Ge(f.T.Scale(-1).Add(Const(-1), 1))
		}
		return f
	case FEq:
		if neg {
			return Or(Ge(f.T.Add(Const(-1), 1)), Ge(f.T.Scale(-1).Add(Const(-1), 1)))
		}
		return f
	case FDiv:
		if neg {
			return &Formula{Kind: FNDiv, D: f.D, T: f.T}
		}
		return f
	case FNDiv:
		if neg {
			return &Formula{Kind: FDiv, D: f.D, T: f.T}
		}
		return f
	case FProp:
		if neg {
			return &Formula{Kind: FNProp, Name: f.Name}
		}
		return f
	case FNProp:
		if neg {
			return Prop(f.Name)
		}
		return f
	case FNot:
		return d.nnf(f.Xs[0], !neg)
	case FAnd, FOr:
		k := f.Kind
		if neg {
			k = FAnd + FOr - k
		}
		xs := make([]*Formula, len(f.Xs))
		for i, x := range f.Xs {
			xs[i] = d.nnf(x, neg)
		}
		return &Formula{Kind: k, Xs: xs}
	case FExists, FForall:
		k := f.Kind
		if neg {
			k = FExists + FForall - k
		}
		return &Formula{Kind: k, Var: f.Var, Xs: []*Formula{d.nnf(f.Xs[0], neg)}}
	}
	panic(fmt.Sprintf("presburger: no formula of kind %d", f.Kind))
}

// skolem renames every ∃ not under a ∀ to a fresh free variable and drops it
// (Theorem S). In negation normal form such an ∃ may be pulled to the prefix:
// its variable, renamed fresh, occurs nowhere else.
func (d *decider) skolem(f *Formula) *Formula {
	switch f.Kind {
	case FExists:
		c := d.fresh("#sk")
		return d.skolem(rename(f.Xs[0], f.Var, c))
	case FAnd, FOr:
		xs := make([]*Formula, len(f.Xs))
		for i, x := range f.Xs {
			xs[i] = d.skolem(x)
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	}
	return f // an atom, or a ∀ and everything under it
}

// rename is f with the free variable x renamed y.
func rename(f *Formula, x, y string) *Formula {
	switch f.Kind {
	case FGe, FEq, FDiv, FNDiv:
		c, ok := f.T.Coef[x]
		if !ok {
			return f
		}
		t := f.T.clone()
		delete(t.Coef, x)
		t.Coef[y] = add(t.Coef[y], c)
		return &Formula{Kind: f.Kind, D: f.D, T: t}
	case FAnd, FOr, FNot:
		xs := make([]*Formula, len(f.Xs))
		for i, s := range f.Xs {
			xs[i] = rename(s, x, y)
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	case FExists, FForall:
		if f.Var == x {
			return f
		}
		return &Formula{Kind: f.Kind, Var: f.Var, Xs: []*Formula{rename(f.Xs[0], x, y)}}
	}
	return f
}

// qe eliminates every quantifier of an NNF formula, innermost first.
func (d *decider) qe(f *Formula) *Formula {
	switch f.Kind {
	case FAnd, FOr:
		xs := make([]*Formula, len(f.Xs))
		for i, x := range f.Xs {
			xs[i] = d.qe(x)
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	case FExists:
		// the bound variable renamed apart, so a substitution cannot meet it
		x := d.fresh("#q")
		return d.cooper(x, d.qe(rename(f.Xs[0], f.Var, x)))
	case FForall:
		// ∀x. φ = ¬∃x. ¬φ
		x := d.fresh("#q")
		body := d.qe(rename(f.Xs[0], f.Var, x))
		return d.nnf(d.cooper(x, d.nnf(body, true)), true)
	}
	return f
}

// cooper eliminates x from ∃x. φ, φ quantifier-free in negation normal form.
func (d *decider) cooper(x string, phi *Formula) *Formula {
	d.tick()
	phi = splitEq(phi, x)
	// l: the least common multiple of x's coefficients, so every atom's is ±l
	l := int64(1)
	walkAtoms(phi, func(a *Formula) {
		if c := a.T.Coef[x]; c != 0 {
			l = lcm(l, abs(c))
		}
	})
	// x′ = l·x: each atom scaled to coefficient ±l, then read in x′
	norm := mapAtoms(phi, func(a *Formula) *Formula {
		c := a.T.Coef[x]
		if c == 0 {
			return a
		}
		m := l / abs(c)
		t := a.T.Scale(m)
		t.Coef[x] = sign(c) // ±1 on x′, named x still
		switch a.Kind {
		case FGe:
			return Ge(t)
		case FDiv:
			return Div(mul(a.D, m), t)
		case FNDiv:
			return &Formula{Kind: FNDiv, D: mul(a.D, m), T: t}
		}
		panic("presburger: an equality in cooper")
	})
	if l > 1 {
		norm = And(norm, Div(l, Var(x)))
	}
	// δ: the least common multiple of the divisors over x′; B: its lower bounds
	delta := int64(1)
	var lower []Lin
	walkAtoms(norm, func(a *Formula) {
		c := a.T.Coef[x]
		if c == 0 {
			return
		}
		switch a.Kind {
		case FDiv, FNDiv:
			delta = lcm(delta, a.D)
		case FGe:
			if c == 1 {
				// x′ + t ≥ 0 is x′ > −t − 1
				b := a.T.clone()
				delete(b.Coef, x)
				lower = append(lower, b.Scale(-1).Add(Const(-1), 1))
			}
		}
	})
	if delta > 4096 || len(lower)*int(delta) > 4096 {
		panic(errBudget)
	}
	// φ at −∞: a lower bound false, an upper bound true, divisibility kept
	minusInf := mapAtoms(norm, func(a *Formula) *Formula {
		c := a.T.Coef[x]
		if a.Kind == FGe && c == 1 {
			return False
		}
		if a.Kind == FGe && c == -1 {
			return True
		}
		return a
	})
	var out []*Formula
	for j := int64(1); j <= delta; j++ {
		out = append(out, simplify(substF(minusInf, x, Const(j))))
		if PlantCooperNoBounds {
			continue
		}
		for _, b := range lower {
			out = append(out, simplify(substF(norm, x, b.Add(Const(j), 1))))
		}
	}
	return simplify(Or(out...))
}

// splitEq rewrites each equality mentioning x as two inequalities.
func splitEq(f *Formula, x string) *Formula {
	switch f.Kind {
	case FEq:
		if f.T.Coef[x] != 0 {
			return And(Ge(f.T), Ge(f.T.Scale(-1)))
		}
		return f
	case FAnd, FOr:
		xs := make([]*Formula, len(f.Xs))
		for i, s := range f.Xs {
			xs[i] = splitEq(s, x)
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	}
	return f
}

func walkAtoms(f *Formula, fn func(*Formula)) {
	switch f.Kind {
	case FGe, FEq, FDiv, FNDiv:
		fn(f)
	case FAnd, FOr:
		for _, x := range f.Xs {
			walkAtoms(x, fn)
		}
	}
}

func mapAtoms(f *Formula, fn func(*Formula) *Formula) *Formula {
	switch f.Kind {
	case FGe, FEq, FDiv, FNDiv:
		return fn(f)
	case FAnd, FOr:
		xs := make([]*Formula, len(f.Xs))
		for i, x := range f.Xs {
			xs[i] = mapAtoms(x, fn)
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	}
	return f
}

// substF is f with the variable x replaced by the term v.
func substF(f *Formula, x string, v Lin) *Formula {
	return mapAtoms(f, func(a *Formula) *Formula {
		c := a.T.Coef[x]
		if c == 0 {
			return a
		}
		t := a.T.clone()
		delete(t.Coef, x)
		return &Formula{Kind: a.Kind, D: a.D, T: t.Add(v, c)}
	})
}

// simplify decides the closed atoms and flattens the connectives.
func simplify(f *Formula) *Formula {
	switch f.Kind {
	case FGe, FEq, FDiv, FNDiv:
		if len(f.T.Coef) > 0 {
			return f
		}
		k := f.T.K
		var v bool
		switch f.Kind {
		case FGe:
			v = k >= 0
		case FEq:
			v = k == 0
		case FDiv:
			v = k%f.D == 0
		case FNDiv:
			v = k%f.D != 0
		}
		if v {
			return True
		}
		return False
	case FAnd, FOr:
		unit, zero := FTrue, FFalse
		if f.Kind == FOr {
			unit, zero = FFalse, FTrue
		}
		var xs []*Formula
		for _, x := range f.Xs {
			s := simplify(x)
			if s.Kind == zero {
				return &Formula{Kind: zero}
			}
			if s.Kind == unit {
				continue
			}
			if s.Kind == f.Kind {
				xs = append(xs, s.Xs...)
				continue
			}
			xs = append(xs, s)
		}
		if len(xs) == 0 {
			return &Formula{Kind: unit}
		}
		if len(xs) == 1 {
			return xs[0]
		}
		return &Formula{Kind: f.Kind, Xs: xs}
	}
	return f
}

// sat decides one conjunction of literals by the Omega test.
func (d *decider) sat(lits []*Formula) Result {
	props := map[string]bool{}
	var cs []Constraint
	con := func(t Lin, eq bool) {
		cs = append(cs, Constraint{Coef: t.Coef, K: t.K, Eq: eq})
	}
	for _, a := range lits {
		switch a.Kind {
		case FGe:
			con(a.T, false)
		case FEq:
			con(a.T, true)
		case FDiv:
			q := d.fresh("#dq")
			con(a.T.Add(Var(q), -a.D), true)
		case FNDiv:
			q, m := d.fresh("#dq"), d.fresh("#dm")
			con(a.T.Add(Var(q), -a.D).Add(Var(m), -1), true)
			con(Var(m).Add(Const(-1), 1), false)
			con(Const(a.D-1).Add(Var(m), -1), false)
		case FProp, FNProp:
			want := a.Kind == FProp
			if v, ok := props[a.Name]; ok && v != want {
				return Unsat
			}
			props[a.Name] = want
		case FTrue:
		case FFalse:
			return Unsat
		default:
			panic(fmt.Sprintf("presburger: a literal of kind %d", a.Kind))
		}
	}
	return Satisfiable(cs)
}

func sign(a int64) int64 {
	if a < 0 {
		return -1
	}
	return 1
}

func lcm(a, b int64) int64 { return mul(a/gcd(a, b), b) }
