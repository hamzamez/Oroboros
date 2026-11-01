package ir

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"

	"oroboros/core"
	"oroboros/emit"
)

// THE NUMBERS OF F64 AS DOUBLES (ADR 0086, numbers.md §5, §6.2).
//
// F64 ⊂ N is a set, and a value proven in it is held as a double: the
// abstraction function d(x) = the real a finite binary64 x denotes, which is
// onto F64 and identifies −0 with +0, so = and < on doubles are N's (IEEE 754
// §5.11 compares finite values as reals). An operation of N on doubles is
// exact and has no double in general; it is realized only under a rounding.
//
// REALIZATION IS A REWRITE BY THE TARGET'S EQUATIONS. A declaration whose
// `ensure` is `r == round(F64, t)`, t a term over its parameters, is the
// equation round(F64, t) = f(x̄), read right to left: wherever the program's
// round(F64, u) matches t, it is that call. The match is modulo the
// commutativity of + and · (sound, because u is an exact term of N and a ring
// identity preserves its value), and a pattern's variable binds only a value
// that is not an operation of N: a double, an integer (made a double by the
// target's exact conversion, round(F64, n) = n for |n| ≤ 2⁵³), or a constant
// that is a double. So round(F64, round(F64, a·b) + c) is two calls, never a
// fused one: its inner rounding is a leaf (planted fault plantFuseFault).
//
// COMPARISONS ARE EXACT, by a Galois connection. The inclusion ι : F64 ↪ ℚ has
// both adjoints, ⌊·⌋ and ⌈·⌉ onto F64: ι(x) ≤ q ⟺ x ≤ ⌊q⌋ and q ≤ ι(x) ⟺ ⌈q⌉ ≤
// x. So a double compared with any constant q is compared with a double:
// x < q ⟺ x ≤ ⌊q⌋ when q ∉ F64, x = q is false there, and the same for the
// others.
//
// THE OBLIGATIONS ARE DECIDED HERE, on an interval domain over the reals with
// double ends, each operation rounded outward (Moore 1966): each rounding's
// term inside [−MAX, MAX], which makes the nearest rounding finite (IEEE 754
// §7.4: it overflows only from 2¹⁰²⁴ − 2⁹⁷⁰), each ÷'s divisor away from 0, and
// each conversion's integer within ±2⁵³. Every value of F64 lies in [−MAX, MAX],
// by induction over the execution once every obligation is proven, so that is
// a double's ⊤. A loop's accumulator is bounded by its trip count (accumulate).
//
// Not built here (numbers.md §9): a rounding onto F64 of a decimal, a mode but
// 'nearest (IEEE's directed modes are the host's dynamic state), F32, sqrt,
// an exact interior (a term of doubles not rounded), and `-checked` for a
// rounding.

// realization is a declaration whose result is round(F64, term).
type realization struct {
	prim   string
	params []string
	sets   []string // each parameter's canonical set
	term   *core.Term
}

var realizationCache sync.Map // *emit.Target → []realization

// realizationsOf is the target's realizations, in name order, so the rewrite
// is a function of its input.
func realizationsOf(tg *emit.Target) []realization {
	if v, ok := realizationCache.Load(tg); ok {
		return v.([]realization)
	}
	var out []realization
	for name, p := range tg.Prims {
		e := p.Ensures
		if e == nil || e.Kind != core.KApp || len(e.Kids) != 3 || !isNamed(e.Kids[0], "=") || !isNamed(e.Kids[1], "result") {
			continue
		}
		r := e.Kids[2]
		if r.Kind != core.KApp || len(r.Kids) != 4 || !isNamed(r.Kids[0], core.RoundName) || r.Kids[1].Kind != core.KStr ||
			r.Kids[1].Str != core.F64Name || r.Kids[3].Kind != core.KStr || r.Kids[3].Str != core.RoundNearest {
			continue
		}
		if len(p.Names) != len(p.Args) || !ringTerm(r.Kids[2], p.Names) {
			continue
		}
		out = append(out, realization{prim: name, params: p.Names, sets: p.Args, term: r.Kids[2]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].prim < out[j].prim })
	realizationCache.Store(tg, out)
	return out
}

func isNamed(t *core.Term, name string) bool {
	return t != nil && t.Kind == core.KName && t.Name == name
}

// ringTerm reports a term of the ring's operations (and one ÷ at its root)
// over the parameters: what a realization may state. A transcendental is not
// one, so a declaration claiming round(F64, exp(x)) realizes nothing
// (numbers.md §6.2).
func ringTerm(t *core.Term, params []string) bool {
	var walk func(t *core.Term, root bool) bool
	walk = func(t *core.Term, root bool) bool {
		switch t.Kind {
		case core.KName:
			for _, p := range params {
				if p == t.Name {
					return true
				}
			}
			return false
		case core.KApp:
			if len(t.Kids) != 3 || t.Kids[0].Kind != core.KName {
				return false
			}
			switch t.Kids[0].Name {
			case "+", "-", "*":
			case core.QuoName:
				if !root {
					return false
				}
			default:
				return false
			}
			return walk(t.Kids[1], false) && walk(t.Kids[2], false)
		}
		return false
	}
	return walk(t, true)
}

// realized is one rounding made a call: the obligations decide reads.
type realized struct {
	res  V
	r    *realization
	args map[string]V // each parameter's argument, as passed
	what string
}

type doubles struct {
	tg    *emit.Target
	f     *Func
	pats  []realization
	def   map[V]Stmt
	lit   map[V]*big.Rat
	calls map[V]*realized
	// params' and results' declared intervals, read before their types are
	// made the double's: F64 ∩ [lo, hi] (numbers.md §2)
	declared map[V]rv
	results  []rv
	hasRes   []bool
	hoisted  []Stmt
	// each rounding onto F64 ∩ [lo, hi]: its value inside [lo, hi]
	within []inSet
	err    error
}

type inSet struct {
	v    V
	in   rv
	what string
}

func (d *doubles) fail(format string, args ...any) {
	if d.err == nil {
		d.err = fmt.Errorf(format, args...)
	}
}

func (d *doubles) isDouble(v V) bool { return d.f.Types[v] == core.F64Name }

func (d *doubles) isInt(v V) bool {
	ty := d.f.Types[v]
	if ty == "int" {
		return true
	}
	_, _, ok := core.IntRangeBig(ty)
	return ok
}

// realizeDoubles realizes every rounding onto F64 by the target's
// declarations, makes every comparison of a double one of doubles, and makes
// each F64 ∩ [lo, hi] the double it is held in. It returns nil when f has no
// number of F64 to realize.
func realizeDoubles(tg *emit.Target, f *Func) (*doubles, error) {
	if !needsDoubles(f) {
		return nil, nil
	}
	d := &doubles{tg: tg, f: f, pats: realizationsOf(tg), def: map[V]Stmt{}, lit: map[V]*big.Rat{},
		calls: map[V]*realized{}, declared: map[V]rv{}}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := r.Stmts[i]
			for _, v := range s.Res {
				d.def[v] = s
			}
			if s.Op == OConst && len(s.Res) == 1 {
				switch s.Lit.Kind {
				case core.KInt:
					d.lit[s.Res[0]] = new(big.Rat).SetInt64(s.Lit.Int)
				case core.KDec:
					d.lit[s.Res[0]] = s.Lit.Dec.Rat()
				}
			}
		}
	})
	d.declare()
	d.closed()
	d.region(f.Body)
	f.Body.Stmts = append(d.hoisted, f.Body.Stmts...)
	d.sweep()
	d.residue()
	d.literals()
	d.hostDoubles()
	if d.err != nil {
		return nil, d.err
	}
	return d, nil
}

// needsDoubles reports a rounding onto F64, an operation of N with a double
// operand, a comparison of a double with anything but a double, or a set
// F64 ∩ [lo, hi].
func needsDoubles(f *Func) bool {
	found := false
	bounded := func(ty string) bool {
		_, ok := f64Bounds(ty)
		_, okEl := f64Bounds(core.ArrayElem(ty))
		return ok || okEl
	}
	for _, p := range f.Params {
		found = found || bounded(f.Types[p])
	}
	for _, ty := range f.Results {
		found = found || bounded(ty)
	}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			if !pi.Len && pi.Other >= 0 && (f.Types[pi.Of] == core.F64Name) != (f.Types[pi.Other] == core.F64Name) {
				found = true
			}
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch {
			case s.Op == ORound && core.IsF64Set(s.Type):
				found = true
			case s.Op.IsCmp() || numberOp(s):
				dbl, other := false, false
				for _, a := range s.Args {
					if f.Types[a] == core.F64Name {
						dbl = true
					} else {
						other = true
					}
				}
				if dbl && (other || s.Op.IsArith()) {
					found = true
				}
			}
		}
	})
	return found
}

// declare reads each parameter's and result's F64 ∩ [lo, hi], and makes the
// type the double's, which is the set's representation.
func (d *doubles) declare() {
	bounds := func(ty string) (rv, string, bool) {
		if b, ok := f64Bounds(ty); ok {
			return b, core.F64Name, true
		}
		if e := core.ArrayElem(ty); e != "" {
			if b, ok := f64Bounds(e); ok {
				return b, strings.TrimSuffix(ty, e) + core.F64Name, true
			}
		}
		return rv{}, ty, false
	}
	for _, p := range d.f.Params {
		if b, ty, ok := bounds(d.f.Types[p]); ok {
			d.declared[p] = b
			d.f.Types[p] = ty
		}
	}
	d.results = make([]rv, len(d.f.Results))
	d.hasRes = make([]bool, len(d.f.Results))
	for j, ty := range d.f.Results {
		if b, nty, ok := bounds(ty); ok {
			d.results[j], d.hasRes[j] = b, true
			d.f.Results[j] = nty
		}
	}
	for v, ty := range d.f.Types {
		if _, nty, ok := bounds(ty); ok {
			d.f.Types[v] = nty
		}
	}
}

// f64Bounds is F64 ∩ [lo, hi]'s interval, with outward double ends.
func f64Bounds(ty string) (rv, bool) {
	lo, hi, ok := core.F64Range(ty)
	if !ok {
		return rv{}, false
	}
	return rv{lo: ratDown(lo.Rat()), hi: ratUp(hi.Rat())}.meet(rvF64), true
}

// closed retypes a negation of a double as the double it is: F64 is closed
// under negation, so −x needs no rounding.
func (d *doubles) closed() {
	for changed := true; changed; {
		changed = false
		d.f.Walk(func(r *Region) {
			for i := range r.Stmts {
				s := &r.Stmts[i]
				if s.Op == ONeg && len(s.Res) == 1 && d.f.Types[s.Res[0]] != core.F64Name && d.isDouble(s.Args[0]) {
					d.f.Types[s.Res[0]] = core.F64Name
					d.def[s.Res[0]] = *s
					changed = true
				}
			}
		})
	}
}

// region rewrites r's statements in order, then its π-parameters' relations.
func (d *doubles) region(r *Region) {
	d.pis(r)
	old := r.Stmts
	r.Stmts = nil
	for i := range old {
		s := old[i]
		for _, sub := range s.Sub {
			d.region(sub)
		}
		switch {
		case s.Op == ORound && core.IsF64Set(s.Type):
			d.round(r, s)
		case s.Op.IsCmp() && (d.isDouble(s.Args[0]) || d.isDouble(s.Args[1])):
			d.compare(r, s)
		default:
			r.Stmts = append(r.Stmts, s)
		}
	}
	if r.T == TBranch {
		d.region(r.Then)
		d.region(r.Else)
	}
}

// ---------------------------------------------------------------- roundings

// pterm is the program's term under a rounding: an operation of N (a value
// of sort N defined by +, −, · or a negation) is a node, any other value a
// leaf.
type pterm struct {
	op   string // "+", "-", "*", "÷", "neg"; "" for a leaf
	kids []*pterm
	v    V
}

func (d *doubles) termOf(v V) *pterm {
	if s, ok := d.def[v]; ok && plantFuseFault && s.Op == ORound && len(s.Args) == 1 {
		return d.termOf(s.Args[0]) // the planted fault: an inner rounding looked through
	}
	if s, ok := d.def[v]; ok && d.f.Types[v] == core.NumType {
		kids := func() []*pterm {
			var ks []*pterm
			for _, a := range s.Args {
				ks = append(ks, d.termOf(a))
			}
			return ks
		}
		switch s.Op {
		case OAdd:
			return &pterm{op: "+", kids: kids(), v: v}
		case OSub:
			return &pterm{op: "-", kids: kids(), v: v}
		case OMul:
			return &pterm{op: "*", kids: kids(), v: v}
		case ONeg:
			return &pterm{op: "neg", kids: kids(), v: v}
		}
	}
	return &pterm{v: v}
}

// constOf is a term's exact value when every leaf is a constant.
func (d *doubles) constOf(t *pterm) (*big.Rat, bool) {
	if t.op == "" {
		q, ok := d.lit[t.v]
		return q, ok
	}
	var qs []*big.Rat
	for _, k := range t.kids {
		q, ok := d.constOf(k)
		if !ok {
			return nil, false
		}
		qs = append(qs, q)
	}
	z := new(big.Rat)
	switch t.op {
	case "+":
		return z.Add(qs[0], qs[1]), true
	case "-":
		return z.Sub(qs[0], qs[1]), true
	case "*":
		return z.Mul(qs[0], qs[1]), true
	case "neg":
		return z.Neg(qs[0]), true
	case "÷":
		if qs[1].Sign() == 0 {
			return nil, false
		}
		return z.Quo(qs[0], qs[1]), true
	}
	return nil, false
}

// round realizes one rounding onto F64.
func (d *doubles) round(r *Region, s Stmt) {
	what := "round(F64, " + showSrc(s) + ")"
	if s.Name != core.RoundNearest {
		d.fail("%s: %s, '%s: no target realizes a rounding onto F64 but to the nearest: IEEE 754's directed "+
			"modes are the host's dynamic state (ADR 0086, numbers.md §6.2)", d.f.Name, what, strings.ReplaceAll(s.Name, "-", "_"))
		return
	}
	var t *pterm
	if len(s.Args) == 2 {
		t = &pterm{op: "÷", kids: []*pterm{d.termOf(s.Args[0]), d.termOf(s.Args[1])}}
	} else {
		t = d.termOf(s.Args[0])
	}
	res := s.Res[0]
	d.f.Types[res] = core.F64Name
	if b, ok := f64Bounds(s.Type); ok {
		d.within = append(d.within, inSet{v: res, in: b, what: "round(" + core.ShowSet(s.Type) + ", " + showSrc(s) + ")"})
	}
	// A CONSTANT TERM IS FOLDED, exactly (numbers.md §4): its rounding is
	// computed by rational arithmetic, and is a literal.
	if q, ok := d.constOf(t); ok {
		x, fin := q.Float64()
		if math.IsInf(x, 0) {
			d.fail("%s: %s overflows F64: its value is past 2^1024 - 2^970 (ADR 0086, numbers.md §3)", d.f.Name, what)
			return
		}
		_ = fin
		r.Stmts = append(r.Stmts, Stmt{Op: OConst, Lit: core.Decimal(core.DecOfFloat64(x + 0)), Res: []V{res}, Src: s.Src})
		d.lit[res] = core.DecOfFloat64(x + 0).Rat()
		return
	}
	// A DOUBLE'S ROUNDING IS ITSELF: r is a retraction onto F64.
	if t.op == "" && d.isDouble(t.v) {
		r.Stmts = append(r.Stmts, Stmt{Op: OThe, Type: core.F64Name, Args: []V{t.v}, Res: []V{res}, Src: s.Src})
		return
	}
	if d.realize(r, t, res, what, s.Src) {
		return
	}
	d.fail("%s: %s: no declaration of target %s realizes it (numbers.md §6.2): a target realizes round(F64, t) by "+
		"a declaration whose ensure is r == round(F64, t), each operand a double, an integer or a constant that is "+
		"a double; a rounding inside it is its own operation, and an exact interior (a term of doubles rounded "+
		"once) is step 2", d.f.Name, what, d.tg.Name)
}

// realize matches t against each realization and appends its call, which
// defines res.
func (d *doubles) realize(r *Region, t *pterm, res V, what string, src *core.Term) bool {
	for i := range d.pats {
		p := &d.pats[i]
		env := map[string]*pterm{}
		if !match(p.term, t, env) {
			continue
		}
		ok := true
		for j, name := range p.params {
			if b, bound := env[name]; !bound || !d.accepts(p.sets[j], b.v) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		args := make([]V, len(p.params))
		passed := map[string]V{}
		for j, name := range p.params {
			args[j] = d.pass(r, p.sets[j], env[name].v, src)
			passed[name] = args[j]
		}
		r.Stmts = append(r.Stmts, Stmt{Op: OCall, Name: p.prim, Args: args, Res: []V{res}, Src: src})
		d.calls[res] = &realized{res: res, r: p, args: passed, what: what}
		return true
	}
	return false
}

// match binds pattern pat to the term t, modulo the commutativity of + and ·.
// A variable binds a leaf only.
func match(pat *core.Term, t *pterm, env map[string]*pterm) bool {
	switch pat.Kind {
	case core.KName:
		if t.op != "" {
			return false
		}
		if b, ok := env[pat.Name]; ok {
			return b.v == t.v
		}
		env[pat.Name] = t
		return true
	case core.KApp:
		op := pat.Kids[0].Name
		args := pat.Kids[1:]
		if t.op != op || len(args) != len(t.kids) {
			return false
		}
		try := func(order []int) bool {
			saved := map[string]*pterm{}
			for k, v := range env {
				saved[k] = v
			}
			for i, a := range args {
				if !match(a, t.kids[order[i]], env) {
					for k := range env {
						delete(env, k)
					}
					for k, v := range saved {
						env[k] = v
					}
					return false
				}
			}
			return true
		}
		if try([]int{0, 1}) {
			return true
		}
		return len(args) == 2 && (op == "+" || op == "*") && try([]int{1, 0})
	}
	return false
}

// accepts reports whether leaf v may stand for a parameter of set ty.
func (d *doubles) accepts(ty string, v V) bool {
	if core.IsF64Set(ty) {
		if d.isDouble(v) {
			return true
		}
		if q, ok := d.lit[v]; ok {
			_, exact := q.Float64()
			return exact
		}
		return d.isInt(v) && d.conversion() != nil
	}
	if _, _, ok := core.IntRangeBig(ty); ok || ty == "int" {
		return d.isInt(v)
	}
	return false
}

// conversion is the realization of round(F64, n) for an integer n.
func (d *doubles) conversion() *realization {
	for i := range d.pats {
		p := &d.pats[i]
		if p.term.Kind == core.KName && len(p.params) == 1 && !core.IsF64Set(p.sets[0]) {
			return p
		}
	}
	return nil
}

// pass is leaf v as a parameter of set ty: itself, a double constant, or an
// integer's exact conversion.
func (d *doubles) pass(r *Region, ty string, v V, src *core.Term) V {
	if !core.IsF64Set(ty) || d.isDouble(v) {
		return v
	}
	if q, ok := d.lit[v]; ok {
		if x, exact := q.Float64(); exact {
			c := d.fresh(core.F64Name)
			r.Stmts = append(r.Stmts, Stmt{Op: OConst, Lit: core.Decimal(core.DecOfFloat64(x + 0)), Res: []V{c}})
			d.lit[c] = q
			return c
		}
	}
	p := d.conversion()
	res := d.fresh(core.F64Name)
	r.Stmts = append(r.Stmts, Stmt{Op: OCall, Name: p.prim, Args: []V{v}, Res: []V{res}, Src: src})
	what := "an integer made a double"
	if src != nil && src.Kind == core.KApp && len(src.Kids) >= 3 {
		what += " in round(F64, " + core.ShowTerm(src.Kids[2]) + ")"
	}
	d.calls[res] = &realized{res: res, r: p, args: map[string]V{p.params[0]: v}, what: what}
	return res
}

func (d *doubles) fresh(ty string) V {
	d.f.Types = append(d.f.Types, ty)
	return V(len(d.f.Types) - 1)
}

func showSrc(s Stmt) string {
	if s.Src != nil && s.Src.Kind == core.KApp && len(s.Src.Kids) >= 3 {
		return core.ShowTerm(s.Src.Kids[2])
	}
	return "…"
}

// ---------------------------------------------------------------- comparisons

// compare makes a comparison with a double one of doubles (the Galois
// connection, above), or refuses one with an exact term.
func (d *doubles) compare(r *Region, s Stmt) {
	a, b := s.Args[0], s.Args[1]
	rel := relOf(s.Op)
	if !d.isDouble(a) { // the double on the left: q rel x is x flip(rel) q
		a, b, rel = b, a, flip(rel)
	}
	switch {
	case d.isDouble(b):
		r.Stmts = append(r.Stmts, s)
		return
	case d.lit[b] != nil:
		nrel, c, k := galois(rel, d.lit[b])
		if k != nil {
			r.Stmts = append(r.Stmts, Stmt{Op: OConst, Lit: core.Bool(*k), Res: s.Res, Src: s.Src})
			return
		}
		cv := d.fresh(core.F64Name)
		r.Stmts = append(r.Stmts, Stmt{Op: OConst, Lit: core.Decimal(core.DecOfFloat64(c)), Res: []V{cv}})
		d.lit[cv] = core.DecOfFloat64(c).Rat()
		op, _ := OpNamed(nrel)
		r.Stmts = append(r.Stmts, Stmt{Op: op, Args: []V{a, cv}, Res: s.Res, Src: s.Src})
		return
	case d.isInt(b) && d.conversion() != nil:
		bv := d.pass(r, core.F64Name, b, s.Src)
		op, _ := OpNamed(rel)
		r.Stmts = append(r.Stmts, Stmt{Op: op, Args: []V{a, bv}, Res: s.Res, Src: s.Src})
		return
	}
	d.fail("%s: %s compares a double with an exact term of N: not built, a number of N made from doubles and "+
		"not rounded is an exact interior, step 2 (numbers.md §9); round it, round(F64, …)", d.f.Name, core.ShowTerm(s.Src))
	r.Stmts = append(r.Stmts, s)
}

// galois is x rel q, for x ∈ F64 and a constant q, as x rel′ c with c a double,
// or as a constant truth k when it holds of every double or of none.
func galois(rel string, q *big.Rat) (string, float64, *bool) {
	t, f := true, false
	lo, okLo := floorF64(q) // the greatest double ≤ q
	hi, okHi := ceilF64(q)  // the least double ≥ q
	exact := okLo && okHi && lo == hi
	switch rel {
	case "eq":
		if exact {
			return "eq", lo, nil
		}
		return "", 0, &f
	case "ne":
		if exact {
			return "ne", lo, nil
		}
		return "", 0, &t
	case "lt", "le":
		if exact {
			return rel, lo, nil
		}
		if !okLo {
			return "", 0, &f // q below every double
		}
		if !okHi {
			return "", 0, &t // q above every double
		}
		if plantGaloisFault && rel == "lt" {
			return "lt", lo, nil // the planted fault: false of x = ⌊q⌋
		}
		return "le", lo, nil
	case "gt", "ge":
		if exact {
			return rel, lo, nil
		}
		if !okHi {
			return "", 0, &f
		}
		if !okLo {
			return "", 0, &t
		}
		return "ge", hi, nil
	}
	return rel, 0, nil
}

// floorF64 is ⌊q⌋ onto F64, false when q is below −MAX.
func floorF64(q *big.Rat) (float64, bool) {
	x, _ := q.Float64()
	if math.IsInf(x, -1) || (x == -math.MaxFloat64 && new(big.Rat).SetFloat64(x).Cmp(q) > 0) {
		return 0, false
	}
	if math.IsInf(x, 1) {
		return math.MaxFloat64, true
	}
	if new(big.Rat).SetFloat64(x).Cmp(q) > 0 {
		x = math.Nextafter(x, math.Inf(-1))
	}
	return x + 0, true
}

// ceilF64 is ⌈q⌉ onto F64, false when q is above MAX.
func ceilF64(q *big.Rat) (float64, bool) {
	x, ok := floorF64(new(big.Rat).Neg(q))
	return -x + 0, ok
}

// pis makes each π-parameter relating a double to a constant relate it to a
// double, by the Galois connection, and drops one relating it to an exact term
// (the identity, which narrows nothing).
func (d *doubles) pis(r *Region) {
	kept := r.Pis[:0]
	var drop []Pi
	for _, pi := range r.Pis {
		if pi.Len || pi.Other < 0 || !d.isDouble(pi.Of) || d.isDouble(pi.Other) || d.isInt(pi.Other) {
			kept = append(kept, pi)
			continue
		}
		q := d.lit[pi.Other]
		if q == nil {
			drop = append(drop, pi)
			continue
		}
		nrel, c, k := galois(pi.Rel, q)
		if k != nil {
			drop = append(drop, pi)
			continue
		}
		cv := d.fresh(core.F64Name)
		d.hoisted = append(d.hoisted, Stmt{Op: OConst, Lit: core.Decimal(core.DecOfFloat64(c)), Res: []V{cv}})
		d.lit[cv] = core.DecOfFloat64(c).Rat()
		pi.Rel, pi.Other = nrel, cv
		kept = append(kept, pi)
	}
	r.Pis = kept
	for _, pi := range drop {
		renameIn(r, pi.V, pi.Of)
	}
}

// ---------------------------------------------------------------- what is left

// sweep removes the operations of N and the constants the realizations
// consumed: pure, and used by nothing now.
func (d *doubles) sweep() {
	for {
		uses := map[V]int{}
		d.f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				uses[pi.Of]++
				if pi.Other >= 0 {
					uses[pi.Other]++
				}
			}
			for i := range r.Stmts {
				for _, a := range r.Stmts[i].Args {
					uses[a]++
				}
			}
			for _, a := range r.Args {
				uses[a]++
			}
			if r.T == TBranch {
				uses[r.Cond]++
			}
		})
		removed := false
		d.f.Walk(func(r *Region) {
			kept := r.Stmts[:0]
			for _, s := range r.Stmts {
				dead := len(s.Res) == 1 && uses[s.Res[0]] == 0 && len(s.Sub) == 0 &&
					(s.Op == OConst || (numberOp(&s) && d.f.Types[s.Res[0]] == core.NumType))
				if dead {
					removed = true
					continue
				}
				kept = append(kept, s)
			}
			r.Stmts = kept
		})
		if !removed {
			return
		}
	}
}

// residue refuses an operation of N on a double that no rounding consumed:
// its value is an exact interior, which no representation holds in step 1.
func (d *doubles) residue() {
	d.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if len(s.Res) != 1 || d.f.Types[s.Res[0]] != core.NumType {
				continue
			}
			for _, a := range s.Args {
				if d.isDouble(a) {
					d.fail("%s: %s is a number of N made from doubles and not rounded: no representation holds "+
						"it yet, an exact interior is step 2 (numbers.md §5, §9); round it, round(F64, …), or "+
						"write round(F64) { … }", d.f.Name, core.ShowTerm(s.Src))
					return
				}
			}
		}
	})
}

// literals makes each integer literal held as a double the numeral it is, so
// a printer writes a double (Go's `v := 0` is an int), and refuses one that
// is not a double: the literal is exact, and nothing rounds it unasked.
func (d *doubles) literals() {
	d.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OConst || len(s.Res) != 1 || !d.isDouble(s.Res[0]) || s.Lit.Kind != core.KInt {
				continue
			}
			q := new(big.Rat).SetInt64(s.Lit.Int)
			if _, exact := q.Float64(); !exact {
				d.fail("%s: the integer %d is held as a double and is not one: a literal is exact (ADR 0086); "+
					"round it, round(F64, %d)", d.f.Name, s.Lit.Int, s.Lit.Int)
				return
			}
			s.Lit = core.Decimal(core.NewDec(big.NewInt(s.Lit.Int), 0))
		}
	})
}

// hostDoubles refuses a double a host computed where the language's numbers
// read it. A host's double is IEEE's, NaN, ±∞ and −0 among its values (§6.3),
// so it is not an element of F64 until decoded, and the decode is not built:
// it may stand where a host's own operation reads it (Tier 2), and nowhere a
// rounding's operand or N's order would give it a meaning it does not have.
// The taint is the least fixed point of: a call's double that no realization
// made, and what the dataflow carries it to (a π, an ascription, a read of a
// table holding one, a store, a join).
func (d *doubles) hostDoubles() {
	host := map[V]bool{}
	carries := func(v V) bool {
		ty := d.f.Types[v]
		return ty == core.F64Name || core.ArrayElem(ty) == core.F64Name
	}
	add := func(v V) bool {
		if host[v] || !carries(v) {
			return false
		}
		host[v] = true
		return true
	}
	from := func(res []V, exits [][]V) bool {
		changed := false
		for _, e := range exits {
			for j, a := range e {
				if j < len(res) && host[a] {
					changed = add(res[j]) || changed
				}
			}
		}
		return changed
	}
	for changed := true; changed; {
		changed = false
		d.f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				if host[pi.Of] {
					changed = add(pi.V) || changed
				}
			}
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch s.Op {
				case OCall, OCellGet, ORead:
					for _, v := range s.Res {
						if d.calls[v] == nil {
							changed = add(v) || changed
						}
					}
				case OThe, ONeg, OIndex, ORestrict, OSet, OArray:
					for _, a := range s.Args {
						if host[a] && len(s.Res) == 1 {
							changed = add(s.Res[0]) || changed
						}
					}
				case OLoop:
					body := s.Sub[0]
					changed = from(body.Params, [][]V{s.Args}) || changed
					changed = from(body.Params, exitsOf(body, TContinue)) || changed
					changed = from(s.Res, exitsOf(body, TBreak)) || changed
				case OIf, OBuild, OTabulate:
					for _, sub := range s.Sub {
						changed = from(s.Res, exitsOf(sub, TYield)) || changed
					}
				}
			}
		})
	}
	if plantHostFault {
		return
	}
	refuse := func(what string) {
		d.fail("%s: %s reads a double a host computed: a host's double is IEEE's, NaN and ±∞ among its "+
			"values, and not an element of F64 until decoded, which is not built (numbers.md §6.3); "+
			"compute it with the language's numbers, or keep it to the host's own operations", d.f.Name, what)
	}
	for _, c := range d.calls {
		for _, v := range c.args {
			if host[v] {
				refuse(c.what)
				return
			}
		}
	}
	d.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op.IsCmp() && (d.isDouble(s.Args[0]) || d.isDouble(s.Args[1])) && (host[s.Args[0]] || host[s.Args[1]]) {
				refuse("the comparison " + core.ShowTerm(s.Src))
				return
			}
		}
	})
}

// ---------------------------------------------------------------- the domain

// rv is an interval of reals whose ends are doubles, ±∞ for none; lo > hi is
// empty. A double's ⊤ is [−MAX, MAX].
type rv struct{ lo, hi float64 }

var rvF64 = rv{lo: -math.MaxFloat64, hi: math.MaxFloat64}
var rvAll = rv{lo: math.Inf(-1), hi: math.Inf(1)}
var rvBot = rv{lo: math.Inf(1), hi: math.Inf(-1)}

func (a rv) empty() bool { return a.lo > a.hi }

func (a rv) join(b rv) rv {
	if a.empty() {
		return b
	}
	if b.empty() {
		return a
	}
	return rv{lo: math.Min(a.lo, b.lo), hi: math.Max(a.hi, b.hi)}
}

func (a rv) meet(b rv) rv { return rv{lo: math.Max(a.lo, b.lo), hi: math.Min(a.hi, b.hi)} }

// mag is sup |x| over the interval.
func (a rv) mag() float64 { return math.Max(math.Abs(a.lo), math.Abs(a.hi)) }

func (a rv) within(b rv) bool { return a.empty() || (b.lo <= a.lo && a.hi <= b.hi) }

func (a rv) String() string {
	if a.empty() {
		return "∅"
	}
	return fmt.Sprintf("[%g, %g]", a.lo, a.hi)
}

// dir is x op y rounded toward −∞ (up false) or +∞ (up true): an outward end,
// from the exact result (big.Float's directed rounding, then the double's).
func dir(op byte, x, y float64, up bool) float64 {
	inf := math.Inf(-1)
	if up {
		inf = math.Inf(1)
	}
	if math.IsInf(x, 0) || math.IsInf(y, 0) {
		switch op {
		case '+':
			if math.IsInf(x, 0) && math.IsInf(y, 0) && x != y {
				return inf
			}
			if math.IsInf(x, 0) {
				return x
			}
			return y
		case '-':
			return dir('+', x, -y, up)
		case '*':
			if x == 0 || y == 0 { // an unbounded end times 0: every real times 0
				return 0
			}
			if (x > 0) == (y > 0) {
				return math.Inf(1)
			}
			return math.Inf(-1)
		case '/':
			if math.IsInf(y, 0) && !math.IsInf(x, 0) {
				return 0
			}
			return inf
		}
	}
	X, Y := new(big.Float).SetFloat64(x), new(big.Float).SetFloat64(y)
	z := new(big.Float).SetPrec(53)
	if up {
		z.SetMode(big.ToPositiveInf)
	} else {
		z.SetMode(big.ToNegativeInf)
	}
	switch op {
	case '+':
		z.Add(X, Y)
	case '-':
		z.Sub(X, Y)
	case '*':
		z.Mul(X, Y)
	case '/':
		z.Quo(X, Y)
	}
	f, acc := z.Float64()
	if up && acc == big.Below {
		f = math.Nextafter(f, math.Inf(1))
	}
	if !up && acc == big.Above {
		f = math.Nextafter(f, math.Inf(-1))
	}
	return f
}

func addRV(a, b rv) rv { return rv{lo: dir('+', a.lo, b.lo, false), hi: dir('+', a.hi, b.hi, true)} }
func subRV(a, b rv) rv { return rv{lo: dir('-', a.lo, b.hi, false), hi: dir('-', a.hi, b.lo, true)} }
func negRV(a rv) rv    { return rv{lo: -a.hi, hi: -a.lo} }

func corners(op byte, a, b rv) rv {
	out := rvBot
	for _, x := range []float64{a.lo, a.hi} {
		for _, y := range []float64{b.lo, b.hi} {
			out = out.join(rv{lo: dir(op, x, y, false), hi: dir(op, x, y, true)})
		}
	}
	return out
}

func mulRV(a, b rv) rv { return corners('*', a, b) }

// quoRV is a ÷ b for b of one sign.
func quoRV(a, b rv) (rv, bool) {
	if b.lo <= 0 && b.hi >= 0 {
		return rvAll, false
	}
	return corners('/', a, b), true
}

// ratDown and ratUp are a rational's outward double ends.
func ratDown(q *big.Rat) float64 {
	x, _ := q.Float64()
	if !math.IsInf(x, 0) && new(big.Rat).SetFloat64(x).Cmp(q) > 0 {
		x = math.Nextafter(x, math.Inf(-1))
	}
	return x
}

func ratUp(q *big.Rat) float64 { return -ratDown(new(big.Rat).Neg(q)) }

// ---------------------------------------------------------------- the decision

// dfacts is the domain's analysis of one function: an interval for each
// double, and for each table of doubles its elements' (array smashing).
type dfacts struct {
	d     *doubles
	f     *Func
	ints  []fact
	trips map[*Stmt]*big.Int
	x     map[V]rv
	def   map[V]*Stmt
	piOf  map[V]V
	stack []*dframe
}

type dframe struct {
	loop             bool
	yld, cont, brk   []rv
	hasY, hasC, hasB bool
	conts            [][]V // each continue's arguments
}

func (a *dfacts) relevant(v V) bool {
	ty := a.f.Types[v]
	return ty == core.F64Name || core.ArrayElem(ty) == core.F64Name
}

func (a *dfacts) get(v V) rv {
	if x, ok := a.x[v]; ok {
		return x
	}
	if q, ok := a.d.lit[v]; ok {
		return rv{lo: ratDown(q), hi: ratUp(q)}
	}
	if a.d.isInt(v) {
		return a.intRV(v)
	}
	return rvF64
}

// intRV is an integer value's interval, from the integer analysis.
func (a *dfacts) intRV(v V) rv {
	if int(v) >= len(a.ints) {
		return rvAll
	}
	x := a.ints[v].v
	if x.bot {
		return rvBot
	}
	out := rvAll
	if !x.nlo {
		out.lo = ratDown(new(big.Rat).SetInt(x.lo.big()))
	}
	if !x.phi {
		out.hi = ratUp(new(big.Rat).SetInt(x.hi.big()))
	}
	return out
}

// decide discharges every realized rounding's obligations on the function as
// it now is, or refuses the program saying which.
func (d *doubles) decide() error {
	if d == nil {
		return nil
	}
	a := d.facts()
	var bad []string
	calls := make([]*realized, 0, len(d.calls))
	for _, c := range d.calls {
		calls = append(calls, c)
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].res < calls[j].res })
	for _, c := range calls {
		bad = append(bad, a.obligations(c)...)
	}
	for _, w := range d.within {
		if x := a.get(w.v); !x.within(w.in) {
			bad = append(bad, fmt.Sprintf("%s's value inside its set: not proven (it is in %s)", w.what, x))
		}
	}
	// A DECLARED RESULT F64 ∩ [lo, hi] is a claim decided at its definition
	// (ADR 0072).
	for _, y := range exitsOf(d.f.Body, TYield) {
		for j, v := range y {
			if j < len(d.hasRes) && d.hasRes[j] && !a.get(v).within(d.results[j]) {
				bad = append(bad, fmt.Sprintf("the result is declared in [%g, %g], and its value is in %s: not proven",
					d.results[j].lo, d.results[j].hi, a.get(v)))
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s: %s (ADR 0086, numbers.md §3, §5)", d.f.Name, strings.Join(dedupe(bad), "; "))
	}
	return nil
}

// facts is the domain's analysis of the function as it now is: the interval
// of each double, and of each table of doubles' elements. A function with no
// double to realize has none, and every double is F64's ⊤.
func (d *doubles) facts() *dfacts {
	if d == nil {
		return nil
	}
	ints, trips := analyseTrips(d.tg, d.f)
	a := &dfacts{d: d, f: d.f, ints: ints, trips: trips, x: map[V]rv{}, def: map[V]*Stmt{}, piOf: map[V]V{}}
	d.f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			a.piOf[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				a.def[v] = &r.Stmts[i]
			}
		}
	})
	for _, p := range d.f.Params {
		if b, ok := d.declared[p]; ok {
			a.x[p] = b
		} else if a.relevant(p) {
			a.x[p] = rvF64
		}
	}
	top := &dframe{}
	a.stack = []*dframe{top}
	a.region(d.f.Body)
	return a
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// term is the interval of a realization's exact term over its arguments'.
func (a *dfacts) term(c *realized) (rv, []string) {
	var why []string
	var walk func(t *core.Term) rv
	walk = func(t *core.Term) rv {
		if t.Kind == core.KName {
			return a.get(c.args[t.Name])
		}
		x, y := walk(t.Kids[1]), walk(t.Kids[2])
		switch t.Kids[0].Name {
		case "+":
			return addRV(x, y)
		case "-":
			return subRV(x, y)
		case "*":
			return mulRV(x, y)
		case core.QuoName:
			q, ok := quoRV(x, y)
			if !ok {
				why = append(why, fmt.Sprintf("the divisor of %s may be 0: not proven (it is in %s)", c.what, y))
			}
			return q
		}
		return rvAll
	}
	return walk(c.r.term), why
}

// obligations are one realized call's unproven obligations.
func (a *dfacts) obligations(c *realized) []string {
	t, bad := a.term(c)
	if c.r.term.Kind == core.KName { // the conversion: its integer within ±2⁵³
		lo, hi, _ := core.IntRangeBig(c.r.sets[0])
		n := a.get(c.args[c.r.params[0]])
		if lo == nil || !n.within(rv{lo: ratDown(new(big.Rat).SetInt(lo)), hi: ratUp(new(big.Rat).SetInt(hi))}) {
			bad = append(bad, fmt.Sprintf("%s must be within ±2^53, where it is one: not proven (it is in %s)",
				c.what, n))
		}
		return bad
	}
	if !t.within(rvF64) {
		bad = append(bad, fmt.Sprintf("%s may overflow F64: its term within ±(2^1024 - 2^970) is not proven "+
			"(it is in %s); narrow the operands' sets, F64 ∩ [lo, hi] as set { x ∈ F64 : lo <= x && x <= hi }, "+
			"or bound a loop's trip count", c.what, t))
	}
	return bad
}

func (a *dfacts) set(v V, x rv) {
	if a.relevant(v) {
		a.x[v] = x
	}
}

func (a *dfacts) region(r *Region) {
	for _, pi := range r.Pis {
		if !a.relevant(pi.V) {
			continue
		}
		x := a.get(pi.Of)
		if !pi.Len && pi.Other >= 0 && a.d.isDouble(pi.Of) {
			x = narrowRV(x, pi.Rel, a.get(pi.Other))
		}
		a.x[pi.V] = x
	}
	for i := range r.Stmts {
		a.stmt(&r.Stmts[i])
	}
	switch r.T {
	case TBranch:
		a.region(r.Then)
		a.region(r.Else)
	case TYield, TContinue, TBreak:
		fr := a.frame(r.T != TYield)
		if fr == nil {
			return
		}
		xs := make([]rv, len(r.Args))
		for i, v := range r.Args {
			xs[i] = a.get(v)
		}
		switch r.T {
		case TYield:
			fr.yld, fr.hasY = joinAllRV(fr.yld, fr.hasY, xs), true
		case TContinue:
			fr.cont, fr.hasC = joinAllRV(fr.cont, fr.hasC, xs), true
			fr.conts = append(fr.conts, r.Args)
		case TBreak:
			fr.brk, fr.hasB = joinAllRV(fr.brk, fr.hasB, xs), true
		}
	}
}

func (a *dfacts) frame(loop bool) *dframe {
	for i := len(a.stack) - 1; i >= 0; i-- {
		if a.stack[i].loop == loop {
			return a.stack[i]
		}
	}
	return nil
}

func joinAllRV(acc []rv, has bool, xs []rv) []rv {
	if !has {
		return append([]rv(nil), xs...)
	}
	for i := range acc {
		if i < len(xs) {
			acc[i] = acc[i].join(xs[i])
		}
	}
	return acc
}

// narrowRV is x where x rel o holds, closed: over the reals x < c gives x ≤ c,
// and never c − 1.
func narrowRV(x rv, rel string, o rv) rv {
	switch rel {
	case "lt", "le":
		return x.meet(rv{lo: math.Inf(-1), hi: o.hi})
	case "gt", "ge":
		return x.meet(rv{lo: o.lo, hi: math.Inf(1)})
	case "eq":
		return x.meet(o)
	}
	return x
}

func (a *dfacts) stmt(s *Stmt) {
	res := V(-1)
	if len(s.Res) == 1 {
		res = s.Res[0]
	}
	switch s.Op {
	case OConst:
		// a double's literal: get reads it
	case OCall:
		if c := a.d.calls[res]; c != nil && res >= 0 {
			t, _ := a.term(c)
			// r is monotone and a retraction onto F64, and these ends are
			// doubles: r(t) ∈ [r(lo), r(hi)] = [lo, hi], inside F64 when the
			// obligation holds.
			a.set(res, t.meet(rvF64))
			return
		}
		for _, v := range s.Res {
			a.set(v, rvF64)
		}
	case ONeg:
		if res >= 0 {
			a.set(res, negRV(a.get(s.Args[0])))
		}
	case OThe:
		if res >= 0 {
			a.set(res, a.get(s.Args[0]))
		}
	case OIndex:
		if res >= 0 && len(s.Args) > 0 {
			a.set(res, a.get(s.Args[0]))
		}
	case ORestrict:
		if res >= 0 {
			a.set(res, a.get(s.Args[0]))
		}
	case OSet:
		if res >= 0 && len(s.Args) == 3 {
			a.set(res, a.get(s.Args[0]).join(a.get(s.Args[2])))
		}
	case OArray:
		if res >= 0 {
			x := rvBot
			for _, v := range s.Args {
				x = x.join(a.get(v))
			}
			a.set(res, x)
		}
	case OIf:
		fr := &dframe{}
		a.stack = append(a.stack, fr)
		for _, sub := range s.Sub {
			a.region(sub)
		}
		a.stack = a.stack[:len(a.stack)-1]
		for j, v := range s.Res {
			if fr.hasY && j < len(fr.yld) {
				a.set(v, fr.yld[j])
			} else {
				a.set(v, rvF64)
			}
		}
	case OLoop:
		a.loop(s)
	case OBuild:
		// A BUILD'S BUFFER BEGINS AS ZEROS, a table's zero on creation, and
		// holds what its region stores: its table is the buffer the region
		// yields.
		fr := &dframe{}
		a.stack = append(a.stack, fr)
		for _, sub := range s.Sub {
			for _, p := range sub.Params {
				a.set(p, rv{})
			}
			a.region(sub)
		}
		a.stack = a.stack[:len(a.stack)-1]
		for j, v := range s.Res {
			if fr.hasY && j < len(fr.yld) {
				a.set(v, fr.yld[j])
			} else {
				a.set(v, rvF64)
			}
		}
	default:
		if len(s.Sub) > 0 {
			fr := &dframe{}
			a.stack = append(a.stack, fr)
			for _, sub := range s.Sub {
				for _, p := range sub.Params {
					a.set(p, rvF64)
				}
				a.region(sub)
			}
			a.stack = a.stack[:len(a.stack)-1]
			// A TABLE A RULE BUILDS holds what its region yields.
			if s.Op == OTabulate && res >= 0 && fr.hasY && len(fr.yld) > 0 {
				a.set(res, fr.yld[len(fr.yld)-1])
				return
			}
		}
		for _, v := range s.Res {
			a.set(v, rvF64)
		}
	}
}

// loop is a loop's facts: a post-fixpoint by Kleene iteration, widened to
// F64's ⊤ after three rounds, then each accumulator bounded by the loop's trip
// count (accumulate) and the body evaluated again under the bound.
func (a *dfacts) loop(s *Stmt) {
	body := s.Sub[0]
	init := make([]rv, len(s.Args))
	for i, v := range s.Args {
		init[i] = a.get(v)
	}
	cur := append([]rv(nil), init...)
	var fr *dframe
	eval := func() {
		for i, p := range body.Params {
			if i < len(cur) {
				a.set(p, cur[i])
			}
		}
		fr = &dframe{loop: true}
		a.stack = append(a.stack, fr)
		a.region(body)
		a.stack = a.stack[:len(a.stack)-1]
	}
	for it := 0; it < 12; it++ {
		eval()
		next := append([]rv(nil), init...)
		if fr.hasC {
			next = joinAllRV(next, true, fr.cont)
		}
		same := true
		for i := range next {
			if it >= 3 && !next[i].within(cur[i]) {
				w := cur[i].join(next[i])
				if w.lo < cur[i].lo {
					w.lo = math.Min(w.lo, -math.MaxFloat64)
				}
				if w.hi > cur[i].hi {
					w.hi = math.Max(w.hi, math.MaxFloat64)
				}
				next[i] = w
			}
			if next[i] != cur[i] {
				same = false
			}
		}
		cur = next
		if same {
			break
		}
	}
	for k := 0; k < 3 && a.accumulate(s, body, init, cur, fr); k++ {
		eval()
	}
	eval()
	for j, v := range s.Res {
		if fr.hasB && j < len(fr.brk) {
			a.set(v, fr.brk[j])
		} else {
			a.set(v, rvF64)
		}
	}
}

// accumulate bounds each double loop parameter whose every continue passes it
// through roundings onto F64 of additions and of contractions:
//
// LEMMA. For q ∈ F64 and r the nearest rounding onto F64,
//
//	(i)  |r(q ± y) − q| ≤ 2|y|, and
//	(ii) |r(q · y)| ≤ |q| when |y| ≤ 1.
//
// Proof: (i) r(v) is an element of F64 nearest v, and q is one, so
// |r(q ± y) − (q ± y)| ≤ |y|; add |y|. (ii) |q·y| ≤ |q| ∈ F64, and r is monotone,
// odd and fixes F64. ∎ No relative error is used, so it holds of subnormals and
// of every magnitude.
//
// THEOREM A. Let a loop take at most T back edges, and let every continue pass
// for p a value built from p by a chain of such roundings, the additions'
// operands y bounded by sup|y| at every iteration, and set β = 2·Σ sup|y| over
// the chain (the most over the continues). Then |p| ≤ |p₀| + T·β at every
// iteration. Proof: each link of the chain is a rounding of a value of F64 (p,
// or the link before it), so by the lemma one iteration moves |p| by at most
// β; induct on the back edges taken. ∎
//
// It is Theorem 1 (trip.go) for a step the rounding perturbs, and like it needs
// only a sound invariant for each y: the facts read are the loop's.
func (a *dfacts) accumulate(s *Stmt, body *Region, init, cur []rv, fr *dframe) bool {
	T := a.trips[s]
	if T == nil || fr == nil || !fr.hasC {
		return false
	}
	tf, _ := new(big.Float).SetInt(T).Float64()
	tighter := false
	for j, p := range body.Params {
		if !a.d.isDouble(p) || j >= len(cur) {
			continue
		}
		beta := 0.0
		ok := true
		for _, c := range fr.conts {
			if j >= len(c) {
				ok = false
				break
			}
			be, okc := a.chain(c[j], p, 0)
			if !okc {
				ok = false
				break
			}
			beta = math.Max(beta, be)
		}
		if !ok {
			continue
		}
		b := dir('+', init[j].mag(), dir('*', tf, beta, true), true)
		nb := cur[j].meet(rv{lo: -b, hi: b})
		if nb != cur[j] {
			cur[j] = nb
			tighter = true
		}
	}
	return tighter
}

// chain is β for the value v built from p by Theorem A's chain, or false.
func (a *dfacts) chain(v, p V, depth int) (beta float64, ok bool) {
	if depth > 64 {
		return 0, false
	}
	if v == p {
		return 0, true
	}
	if src, isPi := a.piOf[v]; isPi {
		return a.chain(src, p, depth+1)
	}
	s := a.def[v]
	if s == nil {
		return 0, false
	}
	switch s.Op {
	case OThe:
		return a.chain(s.Args[0], p, depth+1)
	case OCall:
		c := a.d.calls[v]
		if c == nil || c.r.term.Kind != core.KApp {
			return 0, false
		}
		t := c.r.term
		x, y := t.Kids[1], t.Kids[2]
		if x.Kind != core.KName || y.Kind != core.KName {
			return 0, false
		}
		for _, side := range [][2]*core.Term{{x, y}, {y, x}} {
			// y − q is −q + y, and −q ∈ F64: the lemma holds of either order.
			be, okc := a.chain(c.args[side[0].Name], p, depth+1)
			if !okc {
				continue
			}
			q := a.get(c.args[side[1].Name]).mag()
			switch t.Kids[0].Name {
			case "+", "-":
				return dir('+', be, dir('*', 2, q, true), true), true
			case "*":
				if q <= 1 || plantChainFault {
					return be, true
				}
			}
			return 0, false
		}
	}
	return 0, false
}

// The planted faults of doubles_test.go: an inner rounding looked through by
// the matcher (two roundings fused into one), a strict comparison kept strict
// against ⌊q⌋, and the host's doubles untainted.
var plantFuseFault, plantGaloisFault, plantHostFault = false, false, false

// plantChainFault, for the planted fault: a product taken as a link whatever
// its other factor, so x ← round(F64, x · 2) is bounded by nothing true.
var plantChainFault = false
