package ir

import (
	"fmt"
	"math/big"
	"sync"

	"oroboros/core"
	"oroboros/emit"
)

// THE TARGET'S INTERPRETATION OF THE LANGUAGE, APPLIED (ADR 0089).
//
// A target that states its realizations (ADR 0088) interprets each of the
// language's integer operations f by a list ι₁(f), ι₂(f), …: each a derived
// operation of its host's signature, a host operation or a term over host
// operations (a definition the target makes, D_T). The interpretation is a
// derived signature morphism Σ_L → T(Σ_H) (Goguen and Burstall's theory
// interpretations), and two things make it sound:
//
//   - DEFINEDNESS: each ιᵢ(f) is defined on the instances it is used at. A
//     later ιᵢ, i ≥ 2, is a host operation, used only where its domain, its
//     parameters' sets and its `where`, holds of the operands' proven facts;
//     a term is checked once against its own header (its host operations'
//     obligations decided as a program's are, DecideMarks), and inlined.
//   - AGREEMENT: ⟦ι(f)⟧(x) = f(x) for x in f's domain. The site's own
//     obligations (f's result inside the word) are decided before ι is
//     applied, so a term may rely on them: `<<`'s term answers 0 past a count
//     of 63 because a·2ᵏ is in the word only for a = 0. Checked against ℤ's
//     definition on the IR's interpreter (realize_test.go).
//
// A term is over host operations, never over an operation itself realized by
// a term: ι maps into T(Σ_H), so substitution is one step and terminates.

// RealizeOps applies the stated interpretation to f's integer operations: per
// statement, the narrowest later realization whose domain the facts prove,
// recorded as the statement's Via; otherwise the first, a host operation (the
// statement prints by it) or a term (its instance inlined). A trapped
// statement takes the checked form, inlined when it is a term. It reports how
// many it substituted; the decision is to be taken again, which decides the
// instances' word arithmetic under the site's facts.
func RealizeOps(tg *emit.Target, f *Func) (int, error) {
	if !tg.States() {
		return 0, nil
	}
	fs := analyse(tg, f)
	plan := map[*Stmt]string{}
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			// A LANGUAGE NAME THAT IS A CALL (`abandon`, `trap-if`) realized by a
			// term: its instance, whatever its results.
			if s.Op == OCall && err == nil {
				if names, _, ok := tg.RealizedNames(s.Name); ok && tg.IsDefinition(names[0]) {
					plan[s] = names[0]
				}
				continue
			}
			if !(s.Op.IsArith() || s.Op.IsCmp()) || s.Via != "" || len(s.Res) != 1 || err != nil {
				continue
			}
			names, checked, ok := tg.RealizedNames(opLanguageName(s.Op))
			if !ok {
				continue
			}
			if s.Mode == MTrap {
				if tg.IsDefinition(checked) {
					plan[s] = checked
				}
				continue
			}
			if v := narrowest(tg, names[1:], s, fs); v != "" {
				s.Via = v
				continue
			}
			if tg.IsDefinition(names[0]) {
				plan[s] = names[0]
			}
		}
	})
	if err != nil || len(plan) == 0 {
		return 0, err
	}
	ren := map[V]V{}
	count := 0
	var region func(r *Region)
	region = func(r *Region) {
		var out []Stmt
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, sub := range s.Sub {
				region(sub)
			}
			def, ok := plan[s]
			if !ok {
				out = append(out, *s)
				continue
			}
			c, e := Instance(tg, def)
			if e != nil {
				err = e
				return
			}
			count++
			if s.Op == OCall || !s.Op.IsArith() {
				out = inlineInto(f, out, c, s.Args, s.Res, ren)
				continue
			}
			// THE INTERPRETATION IS A HOMOMORPHISM, so what was proven of f(x) holds
			// of ι(f)(x) (agreement, realize_test.go): the instance's value is
			// ascribed the site's fact, met with the word, which a trapped
			// operation's result is in wherever it returns. Recomputed on the
			// term alone, a branch's join or a host result forgot it.
			t := V(len(f.Types))
			f.Types = append(f.Types, f.Types[s.Res[0]])
			out = inlineInto(f, out, c, s.Args, []V{t}, ren)
			// Counted where it stood only if the decision counts its operation:
			// `+ − · neg` and ADR 0082's, not `/` and `%` (decide.go).
			counted := s.Op == OAdd || s.Op == OSub || s.Op == OMul || s.Op == ONeg || s.Op.IsBit()
			out = append(out, Stmt{Op: OThe, Type: factType(tg, fs[s.Res[0]].v), Args: []V{t}, Res: s.Res, Src: s.Src,
				Mode: s.Mode, Realized: counted})
		}
		r.Stmts = out
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(f.Body)
	if err != nil {
		return 0, err
	}
	renameAway(f, ren)
	Canonicalize(f)
	f.facts = nil
	return count, nil
}

// factType is the type an interval ascribes, met with the word: an infinite
// end is the word's.
func factType(tg *emit.Target, v iv) string {
	lo, hi := big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi)
	if !v.bot {
		if !v.nlo && v.lo.big().Cmp(lo) > 0 {
			lo = v.lo.big()
		}
		if !v.phi && v.hi.big().Cmp(hi) < 0 {
			hi = v.hi.big()
		}
	}
	if lo.Cmp(hi) > 0 {
		lo, hi = big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi)
	}
	return "int " + lo.String() + " " + hi.String()
}

// narrowest is the later realization whose domain holds of s's operands'
// facts with the least total width of its parameters' sets, or "".
func narrowest(tg *emit.Target, names []string, s *Stmt, fs []fact) string {
	best, bestW := "", (*big.Int)(nil)
	for _, n := range names {
		q, ok := tg.Prims[n]
		if !ok || len(q.Args) != len(s.Args) {
			continue
		}
		w := new(big.Int)
		fits := true
		for k, a := range s.Args {
			lo, hi := setEnds(tg, q.Args[k])
			if !fs[a].v.within(lo.ep, hi.ep) {
				fits = false
				break
			}
			w.Add(w, new(big.Int).Sub(hi.b, lo.b))
		}
		if !fits || (q.Where != nil && whereHolds(q.Where, q.Names, s.Args, fs) != yes) {
			continue
		}
		if best == "" || w.Cmp(bestW) < 0 {
			best, bestW = n, w
		}
	}
	return best
}

type end struct {
	ep ep
	b  *big.Int
}

// setEnds is a parameter set's ends, the word's where it has none.
func setEnds(tg *emit.Target, ty string) (end, end) {
	lo, hi := big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi)
	if l, h, ok := core.IntRangeBig(ty); ok {
		if l != nil {
			lo = l
		}
		if h != nil {
			hi = h
		}
	}
	le, _ := fromBig(lo)
	he, _ := fromBig(hi)
	return end{le, lo}, end{he, hi}
}

type tri int

const (
	no tri = iota
	yes
	unknown
)

// whereHolds decides a `where` on the operands' intervals: a comparison of a
// parameter with a constant, and the connectives as D writes them, `if c a b`
// with Boolean constants. Anything else is unknown, so the domain is not
// proven and the realization not chosen.
func whereHolds(t *core.Term, names []string, args []V, fs []fact) tri {
	switch {
	case t.Kind == core.KBool:
		if t.IsTrue() {
			return yes
		}
		return no
	case t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName:
		return unknown
	}
	head, kids := t.Kids[0].Name, t.Kids[1:]
	if head == "if" && len(kids) == 3 {
		c := whereHolds(kids[0], names, args, fs)
		a, b := whereHolds(kids[1], names, args, fs), whereHolds(kids[2], names, args, fs)
		switch {
		case c == yes:
			return a
		case c == no:
			return b
		case a == b:
			return a
		}
		return unknown
	}
	cmp := emit.CmpOp(head)
	if cmp == "" || len(kids) != 2 {
		return unknown
	}
	x, ok1 := operandIV(kids[0], names, args, fs)
	y, ok2 := operandIV(kids[1], names, args, fs)
	if !ok1 || !ok2 {
		return unknown
	}
	return compareIV(cmp, x, y)
}

func operandIV(t *core.Term, names []string, args []V, fs []fact) (iv, bool) {
	if t.Kind == core.KInt {
		e, _ := fromBig(big.NewInt(t.Int))
		return iv{lo: e, hi: e}, true
	}
	if t.Kind == core.KName {
		for k, n := range names {
			if n == t.Name && k < len(args) {
				return fs[args[k]].v, true
			}
		}
	}
	return iv{}, false
}

// compareIV decides x REL y for every pair of their elements: yes when it
// holds of all, no when of none, unknown otherwise.
func compareIV(rel string, x, y iv) tri {
	if x.bot || y.bot {
		return yes // no execution reaches it
	}
	lt := func(a, b iv) tri { // a < b for all elements, or for none
		switch {
		case !a.phi && !b.nlo && a.hi.lt(b.lo):
			return yes
		case !a.nlo && !b.phi && b.hi.le(a.lo):
			return no
		}
		return unknown
	}
	point := func(a iv) bool { return !a.nlo && !a.phi && a.lo == a.hi }
	not := func(t tri) tri {
		switch t {
		case yes:
			return no
		case no:
			return yes
		}
		return unknown
	}
	switch rel {
	case "lt":
		return lt(x, y)
	case "gt":
		return lt(y, x)
	case "le":
		return not(lt(y, x))
	case "ge":
		return not(lt(x, y))
	case "eq", "ne":
		eq := unknown
		switch {
		case lt(x, y) == yes || lt(y, x) == yes:
			eq = no
		case point(x) && point(y) && x.lo == y.lo:
			eq = yes
		}
		if rel == "ne" {
			return not(eq)
		}
		return eq
	}
	return unknown
}

// Instance is a term realization's body as an IR function of its parameters:
// the definition normalized, its obligations decided against its own header
// (the theory checked once, ADR 0035's shape), and lowered. Cached per target
// and definition.
func Instance(tg *emit.Target, def string) (*Func, error) {
	key := fmt.Sprintf("%p|%s", tg, def)
	if v, ok := instances.Load(key); ok {
		if err, bad := v.(error); bad {
			return nil, err
		}
		return v.(*Func), nil
	}
	c, err := instance(tg, def)
	if err != nil {
		err = fmt.Errorf("the realization %s: %w", def, err)
		instances.Store(key, err)
		return nil, err
	}
	instances.Store(key, c)
	return c, nil
}

var instances sync.Map

func instance(tg *emit.Target, def string) (*Func, error) {
	mod := def[:lastDot(def)]
	forms, err := core.Read("use " + mod + "\nexport fn realization_probe() { 0 }")
	if err != nil {
		return nil, err
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		return nil, err
	}
	body, ok := prog.Defs[def]
	if !ok {
		return nil, fmt.Errorf("the target defines no %s", def)
	}
	env, err := tg.Env(prog)
	if err != nil {
		return nil, err
	}
	nf, err := core.Normalize(body, env, core.DefaultFuel)
	if err != nil {
		return nil, err
	}
	nf = emit.EtaTails(tg, nf)
	sig := prog.Sigs[def]
	var all []*core.Sig
	for _, s := range prog.Sigs {
		all = append(all, s)
	}
	nf, err = DecideMarks(tg, def, sig, nf, all)
	if err != nil {
		return nil, err
	}
	c, err := Lower(tg, "realize-"+def, sig, nf, Options{Decided: true})
	if err != nil {
		return nil, err
	}
	// ι MAPS INTO T(Σ_H): an operation of the term that is itself realized by
	// a term would make substitution recursive.
	c.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			s.FromTerm = true
			if (s.Op.IsArith() || s.Op.IsCmp()) && s.Via == "" && err == nil {
				if names, _, ok := tg.RealizedNames(opLanguageName(s.Op)); ok && tg.IsDefinition(names[0]) {
					err = fmt.Errorf("its `%s` is itself realized by a term (%s); a realization is a term over the host's operations", opLanguageName(s.Op), names[0])
				}
			}
		}
	})
	return c, err
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return len(s)
}
