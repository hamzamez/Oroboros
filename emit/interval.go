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

// bottom is the empty interval — no value at all. It exists for one reason: an
// `again` branch of a clause chain is NOT an exit, and joining it into the
// loop's value as ⊤ made every loop with a back edge return "anything".
//
// That was the whole residue. The decimal printer's `m` starts at the count of
// primes, which the count loop bounds perfectly — and the bound was thrown away
// on the way out because the loop's own value was joined with the branches that
// do not produce one.
var bottom = ival{lo: 1, hi: 0}

func (v ival) isBottom() bool { return !v.loInf && !v.hiInf && v.lo > v.hi }

func exact(v int64) ival { return ival{lo: v, hi: v} }

func (v ival) bounded() bool { return !v.loInf && !v.hiInf }

// fits reports whether every value in the interval is inside the portable
// window, which is the condition under which no overflow check is needed.
func (v ival) fits() bool {
	return v.isBottom() || (v.bounded() && v.lo >= iMin && v.hi <= iMax)
}

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
	// A non-negative dividend and a positive divisor keep the sign, and keeping
	// it matters: without this the decimal printer's `m / 10` lost m's floor,
	// and with no floor there is no well-founded descent and the loop could not
	// be shown to terminate.
	if !a.loInf && a.lo >= 0 && !b.loInf && b.lo >= 1 {
		out := ival{lo: 0, hiInf: a.hiInf}
		if !a.hiInf {
			out.hi = a.hi
		}
		return out
	}
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
	if a.isBottom() {
		return b
	}
	if b.isBottom() {
		return a
	}
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

// intersect keeps only what both intervals allow. Sound when both are.
func intersect(a, b ival) ival {
	out := a
	if !b.loInf && (a.loInf || b.lo > a.lo) {
		out.lo, out.loInf = b.lo, false
	}
	if !b.hiInf && (a.hiInf || b.hi < a.hi) {
		out.hi, out.hiInf = b.hi, false
	}
	return out
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

	Loops      int      // loops seen
	Terminates int      // …proven terminating by size change plus a floor
	Trips      int      // …of those, the ones that also yield a trip count
	Diverging  []string // the idempotent cycle nothing was shown to shrink
}

type intervalPass struct {
	tgt     *Target
	env     map[string]ival
	rep     *IntervalReport
	count   bool  // only the final pass counts
	assume  int64 // simulated declared bound on parameters; 0 means none
	assumed bool

	// Size-change collection, filled while walking one loop's back edges.
	scOn     bool
	scRaw    []string
	scOrient []int // +1 the variable descends, -1 it ascends, 0 unusable
	scEdges  []scGraph
	scSteps  []ival    // per-variable change per back edge, joined over edges
	scKnown  []bool    // …and whether that change is known at all
	scSeen   []bool    // …and whether any edge has reported it yet
	scKind   []descent // how variable j descends, per the LAST edge examined
}

// descent is how a measure shrinks: by a fixed amount, or by a factor.
//
// A geometric descent is not exotic — it is what `m / 10` does in the decimal
// printer, and that loop is one of the two residues intervals-2026-08-19
// reported.
type descent struct {
	kind  int   // 0 none, 1 linear, 2 geometric
	delta int64 // linear: the least decrease per step, ≥ 1
	base  int64 // geometric: the divisor, ≥ 2
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
		return bottom // a back edge produces no value
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

	// The collector is per-loop state on a shared pass, so a NESTED loop must
	// not append to its parent's edge list. Saving here rather than at the
	// size-change block below is what makes that true: the fixpoint runs first,
	// and a nested loop's back edge reached during the PARENT's fixpoint was
	// being recorded against the parent's variables — every arc empty, and the
	// parent reported as possibly non-terminating on a cycle nothing was known
	// about. Five edges where the loop has two.
	oRaw, oOr, oEd, oSt, oKn, oSe, oKi, oOn :=
		p.scRaw, p.scOrient, p.scEdges, p.scSteps, p.scKnown, p.scSeen, p.scKind, p.scOn
	p.scOn = false

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

	// SIZE CHANGE, in two sweeps because orientation depends on the steps and
	// the graphs depend on the orientation.
	//
	n := len(raw)
	p.scRaw, p.scOn = raw, true
	p.scOrient = make([]int, n)
	p.scSteps, p.scKnown, p.scSeen = make([]ival, n), make([]bool, n), make([]bool, n)
	p.scKind = make([]descent, n)
	for i := range p.scKnown {
		p.scKnown[i] = true
	}
	p.scEdges = nil
	step(cur) // sweep one: steps only, orientation still zero
	p.scOrient = orient(p.scSteps, p.scKnown)
	p.scEdges = nil
	step(cur) // sweep two: the graphs, now that measures point the right way
	p.scOn = false

	// Descent is only an argument over a WELL-FOUNDED order, and integers are
	// not one without a floor. The floor comes from the interval fixpoint, and
	// it is demanded of the WITNESS rather than of every variable: a loop that
	// counts into an unbounded accumulator still terminates, because the
	// accumulator is not what carries the argument.
	wf := func(j int) bool {
		switch {
		case p.scOrient[j] > 0:
			return !cur[j].loInf
		case p.scOrient[j] < 0:
			return !cur[j].hiInf
		}
		return false
	}
	ok, witness := SizeChangeTerminates(p.scEdges, wf)
	trip, haveTrip := ival{}, false
	if ok {
		trip, haveTrip = p.tripCount(cur)
	}
	// THE POINT: a trip count bounds every variable the guards say nothing
	// about. v ∈ v₀ + T·step, which is what finally bounds an accumulator.
	if haveTrip {
		for j := range raw {
			if !p.scKnown[j] || !p.scSeen[j] {
				continue
			}
			reach := addI(initV[j], mulI(trip, p.scSteps[j]))
			if within(reach, cur[j]) {
				cur[j] = reach
			} else if !reach.loInf || !reach.hiInf {
				cur[j] = intersect(cur[j], reach)
			}
		}
	}
	if wasCounting { // p.count is still false here; the fixpoint runs silently
		p.rep.Loops++
		if ok {
			p.rep.Terminates++
			if haveTrip {
				p.rep.Trips++
			}
		} else {
			p.rep.Diverging = append(p.rep.Diverging,
				fmt.Sprintf("loop(%s): %s", strings.Join(raw, ","), witness.Render(raw)))
		}
	}
	p.scRaw, p.scOrient, p.scEdges, p.scSteps, p.scKnown, p.scSeen, p.scKind, p.scOn =
		oRaw, oOr, oEd, oSt, oKn, oSe, oKi, oOn

	p.count = wasCounting
	p.restore(saved)

	for i, nm := range raw {
		p.env[nm] = cur[i]
		if p.count {
			p.rep.LoopVars++
			if cur[i].fits() {
				p.rep.LoopBound++
			}
		}
	}
	// Collection OFF for the final walk.
	//
	// This pass exists to count and to place intervals, not to gather edges —
	// and a NESTED loop's final walk happens inside its parent's sweeps, where
	// leaving it on made the inner loop's `again` append a graph built from the
	// inner arguments against the OUTER variable names. Every arc came out
	// empty, and the parent was reported as possibly non-terminating on a cycle
	// nothing was known about. It was the analysis lying about the program, in
	// the direction that makes the headline number worse rather than better,
	// which is the only reason it was not mistaken for a result.
	out := p.eval(body)
	for _, nm := range raw {
		delete(p.env, nm)
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
			args := t.Args()
			for i, a := range args {
				if i < len(acc) {
					acc[i] = joinI(acc[i], p.eval(a))
				}
			}
			if p.scOn {
				for j := range p.scRaw {
					if j >= len(args) {
						continue
					}
					st, ok := p.stepOf(args[j], p.scRaw[j])
					switch {
					case !ok:
						p.scKnown[j] = false
					case !p.scSeen[j]:
						p.scSteps[j], p.scSeen[j] = st, true
					default:
						p.scSteps[j] = joinI(p.scSteps[j], st)
					}
				}
				p.scEdges = append(p.scEdges, p.edgeGraph(args))
			}
			return
		}
	}
	p.eval(t)
}

// ---------------------------------------------------------------- size change
//
// Building the graphs, and turning a proof of descent into a NUMBER.
//
// Classical size-change termination proves that a loop stops. What the interval
// residue needs is stronger and comes from the same argument: if a measure
// descends by at least δ from a bounded range, the loop runs at most
// range/δ times — and then every other variable is bounded by its initial value
// plus the trip count times its per-iteration step. That is what bounds an
// accumulator, which no guard in the program mentions.

// orient decides which direction each variable is measured in.
//
// SCT assumes descent toward a well-founded floor. A counter that ASCENDS
// toward a ceiling is the same argument under the change of variable μ = −v,
// which is exactly what a linear ranking function is. Doing it as an
// orientation rather than a special case keeps one algorithm.
func orient(steps []ival, known []bool) []int {
	out := make([]int, len(steps))
	for i := range steps {
		switch {
		case !known[i]:
			// The step is not `v ± e` — `m / 10` is the case that matters, and
			// it is one of the two residues intervals-2026-08-19 reported.
			// Measuring v itself is the natural reading, and it costs nothing to
			// guess: `relate` still has to PROVE the descent, and the floor is
			// still demanded of the witness.
			out[i] = +1
		case !steps[i].hiInf && steps[i].hi <= 0:
			out[i] = +1 // never increases: v itself is the measure
		case !steps[i].loInf && steps[i].lo >= 0:
			out[i] = -1 // never decreases: −v is the measure
		}
	}
	return out
}

// stepOf reads the per-iteration change of variable `self` from the expression
// that replaces it, when that expression is `self`, `self + e` or `self − e`.
func (p *intervalPass) stepOf(arg *core.Term, self string) (ival, bool) {
	if arg.Kind == core.KName && arg.Name == self {
		return exact(0), true
	}
	if arg.Kind != core.KApp || arg.Op().Kind != core.KName || len(arg.Args()) != 2 {
		return top, false
	}
	name := arg.Op().Name
	a, b := arg.Args()[0], arg.Args()[1]
	plus := isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add")
	minus := isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub")
	if !plus && !minus {
		return top, false
	}
	if a.Kind == core.KName && a.Name == self {
		e := p.eval(b)
		if minus {
			return negI(e), true
		}
		return e, true
	}
	// `k + x` is the same step as `x + k`; subtraction is not commutative.
	if plus && b.Kind == core.KName && b.Name == self {
		return p.eval(a), true
	}
	return top, false
}

// relate is the size-change abstraction: what is known about the value of
// variable `dst` after this back edge, against variable `src` before it, in the
// ORIENTED measure.
func (p *intervalPass) relate(arg *core.Term, src string, srcSign, dstSign int) (arc, descent) {
	if srcSign == 0 || dstSign == 0 || srcSign != dstSign {
		// A cross-arc between measures pointing opposite ways says nothing that
		// this analysis can use.
		return noArc, descent{}
	}
	// μ' = μ, when the argument IS the source variable.
	if arg.Kind == core.KName && arg.Name == src {
		return downEq, descent{}
	}
	if arg.Kind != core.KApp || arg.Op().Kind != core.KName || len(arg.Args()) != 2 {
		return noArc, descent{}
	}
	name := arg.Op().Name
	a, b := arg.Args()[0], arg.Args()[1]
	if a.Kind != core.KName || a.Name != src {
		return noArc, descent{}
	}
	e := p.eval(b)

	switch {
	case isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add"):
		// μ = +v: v+e descends when e < 0.  μ = −v: it descends when e > 0.
		if srcSign > 0 {
			if !e.hiInf && e.hi < 0 {
				return down, descent{kind: 1, delta: -e.hi}
			}
			if !e.hiInf && e.hi <= 0 {
				return downEq, descent{}
			}
		} else {
			if !e.loInf && e.lo > 0 {
				return down, descent{kind: 1, delta: e.lo}
			}
			if !e.loInf && e.lo >= 0 {
				return downEq, descent{}
			}
		}
	case isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub"):
		if srcSign > 0 {
			if !e.loInf && e.lo > 0 {
				return down, descent{kind: 1, delta: e.lo}
			}
			if !e.loInf && e.lo >= 0 {
				return downEq, descent{}
			}
		} else {
			if !e.hiInf && e.hi < 0 {
				return down, descent{kind: 1, delta: -e.hi}
			}
			if !e.hiInf && e.hi <= 0 {
				return downEq, descent{}
			}
		}
	case name == "/" || strings.HasSuffix(name, ".idiv") || isOp(name, "div"):
		// v / k is strictly smaller than v when k ≥ 2 and v ≥ 1. The floor
		// matters: 0/10 is 0, which does not descend, and a loop relying on it
		// would not terminate.
		if srcSign > 0 && !e.loInf && e.lo >= 2 {
			if v := p.lookup(src); !v.loInf && v.lo >= 1 {
				return down, descent{kind: 2, base: e.lo}
			}
		}
	}
	return noArc, descent{}
}

// edgeGraph builds one size-change graph for one `again`.
func (p *intervalPass) edgeGraph(args []*core.Term) scGraph {
	n := len(p.scRaw)
	g := newGraph(n)
	for j := 0; j < n && j < len(args); j++ {
		for i := 0; i < n; i++ {
			a, d := p.relate(args[j], p.scRaw[i], p.scOrient[i], p.scOrient[j])
			g.set(i, j, a)
			if i == j && d.kind != 0 {
				p.scKind[j] = d
			}
		}
	}
	return g
}

// tripCount turns a proof of descent into a bound on the number of iterations.
//
// It needs MORE than the size-change criterion: a single measure that descends
// on EVERY back edge, from a range bounded at both ends. SCT proves termination
// in cases this cannot number — a loop that alternately shrinks x and shrinks y
// terminates and has no single descending measure — and that gap is real and
// recorded rather than papered over.
func (p *intervalPass) tripCount(cur []ival) (ival, bool) {
	best, found := top, false
	for j := range p.scRaw {
		if p.scOrient[j] == 0 || p.scKind[j].kind == 0 {
			continue
		}
		universal := true
		for _, g := range p.scEdges {
			if g.at(j, j) != down {
				universal = false
				break
			}
		}
		if !universal {
			continue
		}
		v := cur[j]
		if !v.bounded() {
			continue
		}
		span := v.hi - v.lo
		if span < 0 {
			continue
		}
		var t int64
		switch d := p.scKind[j]; d.kind {
		case 1:
			if d.delta < 1 {
				continue
			}
			t = span/d.delta + 1
		case 2:
			m := v.hi
			if -v.lo > m {
				m = -v.lo
			}
			if m < 1 {
				m = 1
			}
			t = 1
			for m >= d.base {
				m /= d.base
				t++
			}
		default:
			continue
		}
		if !found || t < best.hi {
			best, found = ival{lo: 0, hi: t}, true
		}
	}
	return best, found
}
