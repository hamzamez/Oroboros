package emit

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
)

// Interval analysis over the residual, for one question:
//
//	HOW OFTEN CAN THE COMPILER PROVE AN INTEGER STAYS IN A MACHINE WORD?
//
// That number gates the integer design in docs/spec/data-model.md §8. If
// ranges are usually provable, "exact by default, ranges choose the
// representation" gives correctness and speed at once. If they are not, every
// unprovable operation carries an overflow check — 1.2×–1.9× on addition,
// 1.9×–7.4× on multiplication (gauntlet/results/product-2026-08-19.md) — and the
// design is a trap.
//
// This is an EXPERIMENT, not a component. It is deliberately simple: the
// classical interval domain (Cousot & Cousot 1977) with widening after two
// iterations, plus guard refinement, plus what `sig` declares. It does not
// attempt relational reasoning — `i < n` narrows `i` only as far as `n`'s own
// bound — which is exactly why the numbers it produces are a LOWER bound on
// what a real analysis could prove. A lower bound is the honest thing to gate a
// decision on.

const (
	iMin = -(1<<53 - 1) // the portable window, ADR 0012
	iMax = 1<<53 - 1
)

// ival is [lo, hi] over the integers, with infinities. An empty interval is not
// represented: unreachable code is not what this measures.
type ival struct {
	lo, hi       int64
	loInf, hiInf bool
}

var top = ival{loInf: true, hiInf: true}

func exact(v int64) ival { return ival{lo: v, hi: v} }

func (v ival) bounded() bool { return !v.loInf && !v.hiInf }

// fits reports whether every value in the interval is inside the portable
// window, which is the condition under which no overflow check is needed.
func (v ival) fits() bool { return v.bounded() && v.lo >= iMin && v.hi <= iMax }

func (v ival) String() string {
	l, h := fmt.Sprint(v.lo), fmt.Sprint(v.hi)
	if v.loInf {
		l = "-inf"
	}
	if v.hiInf {
		h = "+inf"
	}
	return "[" + l + ", " + h + "]"
}

// ---------------------------------------------------------------- arithmetic
//
// Saturating, because the analysis must not overflow while reasoning about
// overflow. Anything that saturates becomes infinite, which is sound.

const sat = int64(1) << 62

func clamp(v int64) (int64, bool) {
	if v >= sat || v <= -sat {
		return 0, false
	}
	return v, true
}

func addI(a, b ival) ival {
	out := ival{}
	if a.loInf || b.loInf {
		out.loInf = true
	} else if v, ok := clamp(a.lo + b.lo); ok {
		out.lo = v
	} else {
		out.loInf = true
	}
	if a.hiInf || b.hiInf {
		out.hiInf = true
	} else if v, ok := clamp(a.hi + b.hi); ok {
		out.hi = v
	} else {
		out.hiInf = true
	}
	return out
}

func negI(a ival) ival {
	out := ival{lo: -a.hi, hi: -a.lo, loInf: a.hiInf, hiInf: a.loInf}
	return out
}

func subI(a, b ival) ival { return addI(a, negI(b)) }

func mulI(a, b ival) ival {
	if !a.bounded() || !b.bounded() {
		// A product with an unbounded factor is unbounded, EXCEPT when the
		// other factor is exactly zero.
		if a.bounded() && a.lo == 0 && a.hi == 0 {
			return exact(0)
		}
		if b.bounded() && b.lo == 0 && b.hi == 0 {
			return exact(0)
		}
		return top
	}
	lo, hi := int64(1<<62), int64(-(1 << 62))
	for _, p := range [4]struct{ x, y int64 }{{a.lo, b.lo}, {a.lo, b.hi}, {a.hi, b.lo}, {a.hi, b.hi}} {
		v, ok := clamp(p.x * p.y)
		if !ok || (p.x != 0 && v/p.x != p.y) { // the product itself overflowed
			return top
		}
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return ival{lo: lo, hi: hi}
}

// divI is conservative: truncating division never increases magnitude except
// for the one degenerate case, and a divisor that may be zero says nothing.
func divI(a, b ival) ival {
	if !a.bounded() {
		return top
	}
	m := a.hi
	if -a.lo > m {
		m = -a.lo
	}
	return ival{lo: -m, hi: m}
}

// remI is bounded by the divisor when the divisor is, and by the dividend
// otherwise.
func remI(a, b ival) ival {
	if b.bounded() {
		m := b.hi - 1
		if -b.lo-1 > m {
			m = -b.lo - 1
		}
		if m < 0 {
			m = 0
		}
		return ival{lo: -m, hi: m}
	}
	return divI(a, b)
}

func joinI(a, b ival) ival {
	out := ival{lo: a.lo, hi: a.hi, loInf: a.loInf || b.loInf, hiInf: a.hiInf || b.hiInf}
	if b.lo < out.lo {
		out.lo = b.lo
	}
	if b.hi > out.hi {
		out.hi = b.hi
	}
	return out
}

// widen is the classical widening: a bound that moved goes to infinity. Without
// it a loop counter's fixpoint does not terminate.
func widen(old, new ival) ival {
	out := new
	if new.loInf || (!old.loInf && new.lo < old.lo) {
		out.loInf = true
	}
	if new.hiInf || (!old.hiInf && new.hi > old.hi) {
		out.hiInf = true
	}
	return out
}

// within reports a ⊑ b — every value a admits, b admits too.
func within(a, b ival) bool {
	if a.loInf && !b.loInf {
		return false
	}
	if a.hiInf && !b.hiInf {
		return false
	}
	if !b.loInf && !a.loInf && a.lo < b.lo {
		return false
	}
	if !b.hiInf && !a.hiInf && a.hi > b.hi {
		return false
	}
	return true
}

func eqI(a, b ival) bool {
	return a.loInf == b.loInf && a.hiInf == b.hiInf &&
		(a.loInf || a.lo == b.lo) && (a.hiInf || a.hi == b.hi)
}

// ---------------------------------------------------------------- the pass

// IntervalReport is what the experiment produces.
type IntervalReport struct {
	Ops       int // integer operations that would need an overflow check
	Proven    int // …of those, the ones provably inside the portable window
	Unproven  []string
	ByOp      map[string][2]int // operation -> {proven, total}
	LoopVars  int
	LoopBound int
}

type intervalPass struct {
	tgt     *Target
	env     map[string]ival
	rep     *IntervalReport
	count   bool  // only the final pass counts
	assume  int64 // simulated declared bound on parameters; 0 means none
	assumed bool
}

// Intervals runs the analysis over one residual and reports what it could prove.
//
// `assume` is the experiment's knob: it gives every otherwise-unbounded
// PARAMETER and every array length the range [0, assume], simulating a
// programmer who declared ranges. Zero means declare nothing, which is what
// every program in the repository does today.
func Intervals(tgt *Target, sig *core.Sig, t *core.Term, assume int64) *IntervalReport {
	rep := &IntervalReport{ByOp: map[string][2]int{}}
	p := &intervalPass{tgt: tgt, rep: rep, assume: assume, assumed: assume > 0}
	env := map[string]ival{}
	if t.Kind == core.KFn {
		for _, n := range t.Params {
			env[n] = p.paramIval(n, sig)
		}
		t = t.Body()
	}
	p.env = env
	// A DECLARED range, read off the signature the language already has.
	//
	// `(sig f ((n int)) int (where (go.&& (go.<= 0 n) (go.< n 65536))))` parses
	// today and `Refine` already assumes it for array bounds. Nothing new had to
	// be added to the language for a programmer to state a range — only this
	// pass had to read it (types-direction.md §6).
	if sig != nil && sig.Where != nil {
		p.assumeWhere(sig.Where)
	}
	p.count = false
	p.eval(t) // settle loop fixpoints
	p.count = true
	p.eval(t)
	sort.Strings(rep.Unproven)
	return rep
}

// assumeWhere narrows parameters from a signature's precondition. A conjunction
// is two assumptions; anything else is one relation, and `refine` already knows
// how to read those.
//
// `and` is sugar for a conditional (ADR 0017), so a precondition written with it
// arrives as `(if a b false)` — which `connective` recognises, and which is the
// same seeing-through the refinement fragment had to learn.
func (p *intervalPass) assumeWhere(w *core.Term) {
	if c, ok := connective(p.tgt, w); ok && c.Op == "and" {
		p.assumeWhere(c.Args[0])
		p.assumeWhere(c.Args[1])
		return
	}
	if w.Kind == core.KApp && w.Op().Kind == core.KName && isOp(w.Op().Name, "and") &&
		len(w.Args()) == 2 {
		p.assumeWhere(w.Args()[0])
		p.assumeWhere(w.Args()[1])
		return
	}
	p.refine(w, true)
}

func (p *intervalPass) paramIval(name string, sig *core.Sig) ival {
	if sig != nil {
		for _, sp := range sig.Params {
			if sp.Name == name && sp.Type == "int" && p.assumed {
				return ival{lo: 0, hi: p.assume}
			}
		}
	}
	if p.assumed {
		return ival{lo: 0, hi: p.assume}
	}
	return top
}

func (p *intervalPass) lookup(n string) ival {
	if v, ok := p.env[n]; ok {
		return v
	}
	if p.assumed {
		return ival{lo: 0, hi: p.assume}
	}
	return top
}

// eval returns the interval of a term, counting checkable operations as it goes.
func (p *intervalPass) eval(t *core.Term) ival {
	switch t.Kind {
	case core.KInt:
		return exact(t.Int)
	case core.KBool, core.KStr, core.KFloat:
		return top
	case core.KName:
		return p.lookup(t.Name)
	case core.KFn:
		return p.eval(t.Body())
	case core.KApp:
		return p.app(t)
	}
	return top
}

func (p *intervalPass) app(t *core.Term) ival {
	op := t.Op()
	if op.Kind != core.KName {
		return top
	}
	prim, known := p.tgt.Prims[op.Name]
	args := t.Args()

	if known {
		switch prim.Kind {
		case "let":
			return p.let(args)
		case "cond":
			return p.cond(args)
		case "iterate":
			return p.iterate(args)
		}
	}
	if op.Name == "again" {
		for _, a := range args {
			p.eval(a)
		}
		return top
	}

	vals := make([]ival, len(args))
	for i, a := range args {
		vals[i] = p.eval(a)
	}

	// An array length is non-negative and otherwise unknown — unless the
	// experiment is simulating a declaration.
	if len(vals) == 1 && (isOp(op.Name, "alen") || isOp(op.Name, "len")) {
		if p.assumed {
			return ival{lo: 0, hi: p.assume}
		}
		return ival{lo: 0, hiInf: true}
	}

	out, checkable := p.transfer(op.Name, prim, vals)
	if checkable && p.count {
		p.record(op.Name, out, t)
	}
	return out
}

// transfer is the abstract semantics of one primitive, plus whether an
// exact-by-default representation would have to CHECK it.
//
// Only +, − and × can leave the window. Division cannot grow a value, and
// comparison, indexing and everything else do not produce integers that need
// checking.
func (p *intervalPass) transfer(name string, prim Prim, v []ival) (ival, bool) {
	if prim.Result != "int" && prim.Result != "" {
		return top, false
	}
	switch {
	case len(v) == 2 && (isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add")):
		return addI(v[0], v[1]), true
	case len(v) == 2 && (isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub")):
		return subI(v[0], v[1]), true
	case len(v) == 2 && (isOp(name, "mul") || name == "*" || strings.HasSuffix(name, ".imul")):
		return mulI(v[0], v[1]), true
	case len(v) == 2 && (name == "/" || strings.HasSuffix(name, ".idiv") || isOp(name, "div")):
		return divI(v[0], v[1]), false
	case len(v) == 2 && (name == "%" || strings.HasSuffix(name, ".irem") || isOp(name, "rem")):
		return remI(v[0], v[1]), false
	case len(v) == 1 && (isOp(name, "neg") || strings.HasSuffix(name, ".neg")):
		return negI(v[0]), true
	}
	if prim.Result == "int" {
		return top, false // an integer from somewhere the analysis cannot see
	}
	return top, false
}

func (p *intervalPass) record(name string, out ival, t *core.Term) {
	p.rep.Ops++
	e := p.rep.ByOp[name]
	e[1]++
	if out.fits() {
		p.rep.Proven++
		e[0]++
	} else {
		s := t.String()
		if len(s) > 64 {
			s = s[:61] + "..."
		}
		p.rep.Unproven = append(p.rep.Unproven, fmt.Sprintf("%s %s in %s", name, out, s))
	}
	p.rep.ByOp[name] = e
}

func (p *intervalPass) let(args []*core.Term) ival {
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return top
	}
	v := p.eval(args[0])
	k := args[1]
	body, raw, _ := openFresh(k, map[string]bool{}, asmIdent)
	old, had := p.env[raw[0]]
	p.env[raw[0]] = v
	out := p.eval(body)
	if had {
		p.env[raw[0]] = old
	} else {
		delete(p.env, raw[0])
	}
	return out
}

func (p *intervalPass) cond(args []*core.Term) ival {
	if len(args) != 3 {
		return top
	}
	p.eval(args[0])
	saved := p.snapshot()
	p.refine(args[0], true)
	a := p.eval(args[1])
	p.restore(saved)
	p.refine(args[0], false)
	b := p.eval(args[2])
	p.restore(saved)
	return joinI(a, b)
}

func (p *intervalPass) snapshot() map[string]ival {
	m := make(map[string]ival, len(p.env))
	for k, v := range p.env {
		m[k] = v
	}
	return m
}

func (p *intervalPass) restore(m map[string]ival) { p.env = m }

// refine narrows the environment from a guard. This is the half that makes a
// loop counter bounded at all: `(< i n)` inside the taken branch says i < n,
// and n's own bound carries across.
func (p *intervalPass) refine(c *core.Term, taken bool) {
	if c.Kind != core.KApp || c.Op().Kind != core.KName || len(c.Args()) != 2 {
		return
	}
	name := c.Op().Name
	a, b := c.Args()[0], c.Args()[1]

	// Normalise to `<`-family with the NAME on the left.
	var rel string
	switch {
	case isOp(name, "lt") || name == "<" || strings.HasSuffix(name, ".setl"):
		rel = "lt"
	case isOp(name, "le") || name == "<=" || strings.HasSuffix(name, ".setle"):
		rel = "le"
	case isOp(name, "gt") || name == ">" || strings.HasSuffix(name, ".setg"):
		rel = "gt"
	case isOp(name, "ge") || name == ">=" || strings.HasSuffix(name, ".setge"):
		rel = "ge"
	default:
		return
	}
	if !taken {
		rel = map[string]string{"lt": "ge", "ge": "lt", "le": "gt", "gt": "le"}[rel]
	}
	p.narrow(a, rel, p.eval(b))
	p.narrow(b, map[string]string{"lt": "gt", "gt": "lt", "le": "ge", "ge": "le"}[rel], p.eval(a))
}

func (p *intervalPass) narrow(t *core.Term, rel string, other ival) {
	if t.Kind != core.KName {
		p.narrowSquare(t, rel, other)
		return
	}
	v := p.lookup(t.Name)
	switch rel {
	case "lt":
		if !other.hiInf && (v.hiInf || other.hi-1 < v.hi) {
			v.hi, v.hiInf = other.hi-1, false
		}
	case "le":
		if !other.hiInf && (v.hiInf || other.hi < v.hi) {
			v.hi, v.hiInf = other.hi, false
		}
	case "gt":
		if !other.loInf && (v.loInf || other.lo+1 > v.lo) {
			v.lo, v.loInf = other.lo+1, false
		}
	case "ge":
		if !other.loInf && (v.loInf || other.lo > v.lo) {
			v.lo, v.loInf = other.lo, false
		}
	}
	p.env[t.Name] = v
}

// narrowSquare inverts `x*x REL e` into a bound on x.
//
// Without it a sieve's counter is unbounded, and everything downstream of it —
// `j = i*i`, `i+1`, `j+i` — is unbounded too. Every failure in the first run of
// this experiment traced back here, which is why it is worth the twenty lines:
// `while (i*i < n)` is not an exotic pattern, it is how half of number theory
// is written.
//
// Only the upper half is inverted. `x*x > e` bounds |x| from BELOW, which says
// nothing about whether x fits in a word.
func (p *intervalPass) narrowSquare(t *core.Term, rel string, other ival) {
	if rel != "lt" && rel != "le" {
		return
	}
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return
	}
	name := t.Op().Name
	if !isOp(name, "mul") && name != "*" && !strings.HasSuffix(name, ".imul") {
		return
	}
	a, b := t.Args()[0], t.Args()[1]
	if a.Kind != core.KName || b.Kind != core.KName || a.Name != b.Name {
		return
	}
	if other.hiInf || other.hi < 0 {
		return
	}
	s := isqrt(other.hi)
	v := p.lookup(a.Name)
	if v.hiInf || s < v.hi {
		v.hi, v.hiInf = s, false
	}
	if v.loInf || -s > v.lo {
		v.lo, v.loInf = -s, false
	}
	p.env[a.Name] = v
}

func isqrt(n int64) int64 {
	if n < 0 {
		return 0
	}
	r := int64(0)
	for (r+1)*(r+1) <= n && r < 1<<31 {
		r++
	}
	return r
}

// iterate is the fixpoint. Loop variables start at their initial values and are
// re-joined with whatever `again` produces, widening after two rounds.
func (p *intervalPass) iterate(args []*core.Term) ival {
	if len(args) < 2 || args[0].Kind != core.KFn {
		return top
	}
	lam, inits := args[0], args[1:]
	initV := make([]ival, len(inits))
	for i, z := range inits {
		initV[i] = p.eval(z)
	}
	cur := make([]ival, len(initV))
	copy(cur, initV)
	body, raw, _ := openFresh(lam, map[string]bool{}, asmIdent)

	saved := p.snapshot()
	wasCounting := p.count
	p.count = false

	// ASCENDING with widening, to reach a post-fixpoint in bounded time.
	//
	// The transfer is `init ⊔ F(cur)`: a loop variable holds either its initial
	// value or something an `again` produced, and nothing else.
	step := func(cur []ival) []ival {
		for i, n := range raw {
			p.env[n] = cur[i]
		}
		next := make([]ival, len(initV))
		copy(next, initV)
		p.collectAgain(body, raw, next)
		return next
	}
	for round := 0; round < 8; round++ {
		next := step(cur)
		stable := true
		for i := range cur {
			j := joinI(cur[i], next[i])
			if round >= 2 {
				j = widen(cur[i], j)
			}
			if !eqI(j, cur[i]) {
				stable = false
			}
			cur[i] = j
		}
		if stable {
			break
		}
	}

	// DESCENDING, which is the half that makes the whole thing work.
	//
	// Widening throws a growing bound straight to infinity, and it does so
	// BEFORE the loop's own guard has had a chance to cap it. Re-evaluating from
	// a post-fixpoint recovers the bound: the guard now applies to an infinite
	// interval and cuts it back to something finite. Without this phase every
	// counter in every program came out unbounded, and the experiment's first
	// numbers — 10% to 20% — were measuring its absence rather than anything
	// about the programs.
	for round := 0; round < 4; round++ {
		next := step(cur)
		improved := false
		for i := range cur {
			if within(next[i], cur[i]) && !eqI(next[i], cur[i]) {
				cur[i] = next[i]
				improved = true
			}
		}
		if !improved {
			break
		}
	}

	p.count = wasCounting
	p.restore(saved)

	for i, n := range raw {
		p.env[n] = cur[i]
		if p.count {
			p.rep.LoopVars++
			if cur[i].fits() {
				p.rep.LoopBound++
			}
		}
	}
	out := p.eval(body)
	for _, n := range raw {
		delete(p.env, n)
	}
	return out
}

// collectAgain walks the clause chain, refining by each guard, and joins the
// interval of every `again` argument into acc.
func (p *intervalPass) collectAgain(t *core.Term, raw []string, acc []ival) {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if prim, ok := p.tgt.Prims[t.Op().Name]; ok && prim.Kind == "cond" && len(t.Args()) == 3 {
			saved := p.snapshot()
			p.refine(t.Args()[0], true)
			p.collectAgain(t.Args()[1], raw, acc)
			p.restore(saved)
			p.refine(t.Args()[0], false)
			p.collectAgain(t.Args()[2], raw, acc)
			p.restore(saved)
			return
		}
		if prim, ok := p.tgt.Prims[t.Op().Name]; ok && prim.Kind == "let" && len(t.Args()) == 2 {
			k := t.Args()[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				v := p.eval(t.Args()[0])
				kb, kraw, _ := openFresh(k, map[string]bool{}, asmIdent)
				old, had := p.env[kraw[0]]
				p.env[kraw[0]] = v
				p.collectAgain(kb, raw, acc)
				if had {
					p.env[kraw[0]] = old
				} else {
					delete(p.env, kraw[0])
				}
				return
			}
		}
		if t.Op().Name == "again" {
			for i, a := range t.Args() {
				if i < len(acc) {
					acc[i] = joinI(acc[i], p.eval(a))
				}
			}
			return
		}
	}
	p.eval(t)
}
