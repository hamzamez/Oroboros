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
	if b.bounded() && !a.loInf && a.lo >= 0 && b.lo >= 1 {
		return ival{lo: 0, hi: b.hi - 1}
	}
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

	Selected   int      // operations rewritten to a checked primitive
	Loops      int      // loops seen
	Terminates int      // …proven terminating by size change plus a floor
	Trips      int      // …of those, the ones that also yield a trip count
	Diverging  []string // the idempotent cycle nothing was shown to shrink

	// Result is the interval of what the function returns, which is what an
	// exported definition's POSTCONDITION is checked against. A loop has no
	// linear form, so the refinement layer cannot read a body's value and this
	// is the only machinery that can.
	Result ival

	// MaxOp is the join of EVERY integer operation's result interval, which is
	// the question index narrowing has to ask: computing in the host's 32-bit
	// index type gives the same answer as computing in 64 bits exactly when
	// every intermediate stays inside it. One unbounded operation makes the
	// whole loop unnarrowable, which is the honest answer.
	MaxOp ival

	// Stores is the joined interval of everything written into each `build`
	// buffer, keyed by the buffer's name. It is what lets a buffer be stored
	// narrower than a machine word when no syntactic fact says so — the node
	// table in examples/json/tree.oro holds indices bounded by a loop guard and
	// by nothing a literal can show.
	Stores map[string]ival
}

type intervalPass struct {
	tgt     *Target
	env     map[string]ival
	rep     *IntervalReport
	count   bool  // only the final pass counts
	assume  int64 // simulated declared bound on parameters; 0 means none
	assumed bool

	// elem is a table's ELEMENT range, by the name the table is bound to.
	//
	// THEOREM. If t has element type (int lo hi) then ⟦t[i]⟧ ∈ [lo,hi]: the
	// range over-approximates every stored value, and 0 is in it because
	// `build` zero-fills (tables.md §14.3), so a read returns a stored value or
	// the zero fill and both are in range.
	//
	// NON-CIRCULAR BY CONSTRUCTION. Only two sources are used, and neither is
	// this analysis: a range DECLARED on a signature, which is a premise; and
	// the SYNTACTIC range of a buffer's stores — literals and conditionals over
	// them — which never consults an interval. `BufferRange`, which does consult
	// one, is deliberately not a source: it runs its own sub-pass and using its
	// answer here would be a fixpoint feeding itself.
	elem map[string]ival

	// letTerm is what each enclosing `let` bound, by name. The environment
	// records a name's VALUE; this records the TERM it came from, which is what
	// loop monotonicity needs to look at — ADR 0015 permits `again` under a
	// `let`, so the argument advancing an index is often a bound name with the
	// scanner's loop behind it.
	letTerm map[string]*core.Term

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

// BufferRange is the range of everything stored into a `build` buffer, as a
// type — or "" when the analysis cannot bound it.
//
// THE SOUNDNESS ARGUMENT, because this is the first place an analysis result
// decides how many BITS a value gets and a wrong answer is a silent wrong
// answer rather than a slow one.
//
//  1. The pass is run on the `build` LAMBDA ALONE, not on the enclosing
//     function. Less context can only widen an interval, never narrow one, so a
//     subterm analysis is conservative with respect to the whole-program one.
//     Anything free in the lambda is unbounded and the buffer stays wide.
//  2. The range is the JOIN of every store, and 0 is always included because
//     `build` zero-fills and an unwritten slot reads 0 (tables.md §14.3).
//  3. A store the pass cannot bound gives an infinite endpoint and answers "",
//     so the buffer keeps the host's own width. Failure is the safe direction
//     and it is the default.
//  4. It only ever NARROWS storage, never semantics. If the analysis is right
//     the emitted program is identical in behaviour and smaller; if it is
//     wrong, the differential suite's `; expect:` answers are what catch it —
//     and note that AGREEMENT cannot, because every target narrows on the same
//     decision.
func BufferRange(tgt *Target, lam *core.Term, sig *core.Sig, params []string) (string, bool) {
	if lam == nil || lam.Kind != core.KFn || len(lam.Params) != 1 {
		return "", false
	}
	rep, _ := intervalsAssuming(tgt, lam, sig, params)
	var out ival
	found := false
	for _, v := range rep.Stores {
		if v.loInf || v.hiInf {
			return "", false
		}
		if !found {
			out, found = v, true
			continue
		}
		out = joinI(out, v)
	}
	if !found {
		return "", false
	}
	out = joinI(out, exact(0)) // the zero fill is an element
	return fmt.Sprintf("int %d %d", out.lo, out.hi), true
}

// FitsIndex reports whether every integer operation in a term stays inside
// [-2^31, 2^31-1] — the question index narrowing asks, and it is not the same
// question as `fits`, which is the portable window at ±(2^53-1).
//
// A value bounded by 2^53 does not fit a 32-bit index, so the two answers
// diverge on exactly the programs this is for.
func (r *IntervalReport) FitsIndex() bool {
	v := r.MaxOp
	if v.loInf || v.hiInf {
		return false
	}
	if v.lo > v.hi { // bottom: no integer operation at all
		return true
	}
	return v.lo >= -2147483648 && v.hi <= 2147483647
}

// MaxOpRange prints the join of every operation's interval, for the tool that
// answers whether narrowing could fire at all.
func (r *IntervalReport) MaxOpRange() string {
	v := r.MaxOp
	if v.lo > v.hi && !v.loInf && !v.hiInf {
		return "none"
	}
	lo, hi := "-inf", "+inf"
	if !v.loInf {
		lo = fmt.Sprintf("%d", v.lo)
	}
	if !v.hiInf {
		hi = fmt.Sprintf("%d", v.hi)
	}
	return lo + ".." + hi
}

// CheckEnsures decides an exported definition's POSTCONDITION against its body.
//
// This is the direction where Q is an OBLIGATION rather than an assumption
// (postconditions.md §2): the caller is outside the program, so nothing else
// can establish it, and the body is the only evidence there is.
//
// It decides the constant-bounded fragment — `K <= result`, `result <= K`, and
// conjunctions of those — because that is what an interval can settle. A
// RELATIONAL postcondition such as `result > i` is reported rather than
// assumed, which is the same treatment §7 of refinements.md gives everything
// outside its fragment, and for the same reason: an unproven claim that is
// silently believed is worse than one that is reported.
func CheckEnsures(tgt *Target, sig *core.Sig, t *core.Term) (bool, string) {
	if sig == nil || sig.Ensures == nil {
		return true, ""
	}
	rep, _ := Intervals(tgt, sig, t, 0)
	ok, decided := entailsIval(tgt, sig.Ensures, rep.Result)
	if !decided {
		return true, "postcondition is outside the decidable fragment, " +
			"propagated and not proven: " + sig.Ensures.String()
	}
	if !ok {
		return false, "the body does not establish " + sig.Ensures.String() +
			"; its result is " + rep.Result.String()
	}
	return true, ""
}

// entailsIval decides a postcondition against the result's interval, reporting
// whether it holds and whether it could be decided at all.
//
// A conjunction is decided when BOTH halves are — an undecidable half makes the
// whole undecidable, because a conjunction is only as good as its weakest part.
func entailsIval(tgt *Target, q *core.Term, v ival) (bool, bool) {
	if c, ok := connective(tgt, q); ok && c.Op == "and" && len(c.Args) == 2 {
		a, adec := entailsIval(tgt, c.Args[0], v)
		b, bdec := entailsIval(tgt, c.Args[1], v)
		if !adec || !bdec {
			return false, false
		}
		return a && b, true
	}
	if q.Kind != core.KApp || q.Op().Kind != core.KName || len(q.Args()) != 2 {
		return false, false
	}
	lhs, rhs := q.Args()[0], q.Args()[1]
	name := q.Op().Name
	isResult := func(x *core.Term) bool {
		return x.Kind == core.KName && x.Name == core.ResultName
	}
	konst := func(x *core.Term) (int64, bool) {
		if x.Kind == core.KInt {
			return x.Int, true
		}
		return 0, false
	}
	switch {
	case isResult(lhs):
		k, ok := konst(rhs)
		if !ok {
			return false, false
		}
		switch {
		case isOp(name, "le"):
			return !v.hiInf && v.hi <= k, true
		case isOp(name, "lt"):
			return !v.hiInf && v.hi < k, true
		case isOp(name, "ge"):
			return !v.loInf && v.lo >= k, true
		case isOp(name, "gt"):
			return !v.loInf && v.lo > k, true
		}
	case isResult(rhs):
		k, ok := konst(lhs)
		if !ok {
			return false, false
		}
		switch {
		case isOp(name, "le"): // K <= result
			return !v.loInf && v.lo >= k, true
		case isOp(name, "lt"): // K < result
			return !v.loInf && v.lo > k, true
		case isOp(name, "ge"): // K >= result
			return !v.hiInf && v.hi <= k, true
		case isOp(name, "gt"): // K > result
			return !v.hiInf && v.hi < k, true
		}
	}
	return false, false
}

// intervalsAssuming analyses a subterm with the ENCLOSING function's
// precondition in scope.
//
// A `build` lambda's free variables are the enclosing function's parameters,
// and what bounds a buffer's stores is usually something the signature says
// about them — examples/json/tree.oro stores a token length into its node
// table, bounded by `len src` and by nothing inside the lambda.
//
// The alternative was to analyse the whole function once and look the answer up
// per `build`, and it does not work: `openFresh` REBUILDS a term to substitute
// its bound variables, so the build the backend holds is not the pointer the
// analysis saw, and its printed form differs too because the parameters were
// renamed. There is no key. Carrying the assumptions to the subterm is the same
// information arriving by the one route that survives.
//
// SOUNDNESS. The `where` is a premise the program already relies on: on an
// exported definition it is a published contract and is assumed, which is
// refinements.md §6b's rule. Intervals taken with it are ⊑ the ones taken
// without — tighter, and both sound.
func intervalsAssuming(tgt *Target, lam *core.Term, sig *core.Sig, params []string) (*IntervalReport, *core.Term) {
	if sig == nil || sig.Where == nil || len(params) == 0 {
		return Intervals(tgt, nil, lam, 0)
	}
	// Rename the signature's parameter names to the ones the caller opened
	// with, for the reason Refine needs the same thing: a length is keyed by
	// the printed term, so `len(p)` and `len(a)` are different variables.
	sub := map[string]*core.Term{}
	for i, n := range params {
		if i < len(sig.Params) && sig.Params[i].Name != "" && sig.Params[i].Name != n {
			sub[sig.Params[i].Name] = core.Name(n)
		}
	}
	return Intervals(tgt, &core.Sig{Where: core.Rename2(sig.Where, sub)}, lam, 0)
}

func Intervals(tgt *Target, sig *core.Sig, t *core.Term, assume int64) (*IntervalReport, *core.Term) {
	rep := &IntervalReport{ByOp: map[string][2]int{}, Stores: map[string]ival{}}
	rep.MaxOp = bottom
	p := &intervalPass{tgt: tgt, rep: rep, assume: assume, assumed: assume > 0}
	env := map[string]ival{}
	var head *core.Term
	if t.Kind == core.KFn {
		head = t
		for _, n := range t.Params {
			env[n] = p.paramIval(n, sig)
		}
		t = t.Body()
	}
	p.env = env
	p.elem = map[string]ival{}
	// A DECLARED element range on a parameter. `(sig tokens ((src (array (int 0
	// 255)))) int)` says a source byte is 0..255, so `(src i)` is too — which is
	// the premise, not an inference.
	if head != nil && sig != nil {
		for i, n := range head.Params {
			if i >= len(sig.Params) {
				break
			}
			if lo, hi, ok := core.IntRange(core.ArrayElem(sig.Params[i].Type)); ok {
				p.elem[n] = ival{lo: lo, hi: hi}
			}
		}
	}
	// A DECLARED range, read off the signature the language already has.
	//
	// `(sig f ((n int)) int (where (go.&& (go.<= 0 n) (go.< n 65536))))` parses
	// today and `Refine` already assumes it for array bounds. Nothing new had to
	// be added to the language for a programmer to state a range — only this
	// pass had to read it (types-direction.md §6).
	if sig != nil && sig.Where != nil {
		// Renamed into the DEFINITION's parameter names first, for the reason
		// Refine needs the same thing: a length is keyed by the printed term,
		// so `(go.len p)` and `(go.len a)` are different keys and a `where`
		// written against the signature narrows nothing.
		sub := map[string]*core.Term{}
		if head != nil {
			for i, name := range head.Params {
				if i < len(sig.Params) && sig.Params[i].Name != "" && sig.Params[i].Name != name {
					sub[sig.Params[i].Name] = core.Name(name)
				}
			}
		}
		p.assumeWhere(core.Rename2(sig.Where, sub))
	}
	p.count = false
	p.eval(t) // settle loop fixpoints
	p.count = true
	res, out := p.evalR(t)
	rep.Result = res
	sort.Strings(rep.Unproven)
	if head != nil {
		out = core.FnClosed(head.Params, out) // t.Body() was already closed
	}
	return rep, out
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

// envKey is what the interval environment is keyed by.
//
// Names, and ALSO array lengths. `(go.len a)` is not a name, so a `where`
// bounding it could not narrow anything — and an array's length is the most
// common thing a program has to say something about. The linear fragment has
// always treated a length as an opaque variable spelled `go.len(a)`; this is
// the interval domain learning the same trick.
func envKey(t *core.Term) (string, bool) {
	switch t.Kind {
	case core.KName:
		return t.Name, true
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName && len(t.Args()) == 1 && isLenOp(op.Name) {
			// Normalised, for the reason lengthVar is: one quantity, one key.
			return "len(" + t.Args()[0].String() + ")", true
		}
	}
	return "", false
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

// eval returns the interval of a term. Most callers — guard narrowing, size
// change, step reading — want only that.
func (p *intervalPass) eval(t *core.Term) ival { v, _ := p.evalR(t); return v }

// evalR is the same walk, also rebuilding the term.
//
// The rebuild is what turns a proof into different EMITTED CODE: an operation
// whose result is not provably inside the window is rewritten to the checked
// primitive the target declares, and one that is provable keeps the host's own
// operator. Everything else about the term is reconstructed unchanged, so a
// target that declares no checked forms gets back exactly what it gave.
func (p *intervalPass) evalR(t *core.Term) (ival, *core.Term) {
	switch t.Kind {
	case core.KInt:
		return exact(t.Int), t
	case core.KBool, core.KStr, core.KFloat, core.KBound:
		return top, t
	case core.KName:
		return p.lookup(t.Name), t
	case core.KFn:
		v, b := p.evalR(t.Body())
		return v, core.FnClosed(t.Params, b)
	case core.KApp:
		return p.app(t)
	}
	return top, t
}

func (p *intervalPass) app(t *core.Term) (ival, *core.Term) {
	op := t.Op()
	if op.Kind != core.KName {
		return top, t
	}
	prim, known := p.tgt.Prims[op.Name]
	args := t.Args()

	if known {
		switch prim.Kind {
		case "let":
			return p.let(t)
		case "cond":
			return p.cond(t)
		case "iterate":
			return p.iterate(t)
		}
	}

	vals := make([]ival, len(args))
	kids := make([]*core.Term, 0, len(t.Kids))
	kids = append(kids, op)
	for i, a := range args {
		v, na := p.evalR(a)
		vals[i] = v
		kids = append(kids, na)
	}
	rebuilt := &core.Term{Kind: core.KApp, Kids: kids}

	if op.Name == "again" {
		return bottom, rebuilt // a back edge produces no value
	}

	// EVERY STORE INTO A BUFFER, joined, so the buffer can be held narrower
	// than a machine word when the value's bound comes from a loop guard rather
	// than from a literal. Recorded on the counting pass only, which is the one
	// that runs after the loop fixpoints have settled.
	if known && prim.Kind == "table-set" && len(args) == 3 {
		if root := BufferRoot(t); root != "" && p.count {
			if have, ok := p.rep.Stores[root]; ok {
				p.rep.Stores[root] = joinI(have, vals[2])
			} else {
				p.rep.Stores[root] = vals[2]
			}
		}
	}

	// An array length is non-negative, and whatever else has been declared or
	// narrowed about it.
	if len(vals) == 1 && isLenOp(op.Name) {
		out := ival{lo: 0, hiInf: true}
		if p.assumed {
			out = ival{lo: 0, hi: p.assume}
		}
		// A LENGTH THAT IS KNOWN EXACTLY. A table given by its GRAPH has as
		// many elements as it was written with, and one given by a rule or a
		// `build` has the length it was asked for — both are in the term, and
		// treating them as unknown threw away the most certain fact available.
		//
		// It matters because reduction INLINES: `examples/json/tokenize.oro`'s
		// `run` substitutes four literal documents into the tokeniser, so every
		// `(len src)` there is a literal array's length and every loop over it
		// was unbounded for want of reading it off.
		if n, ok := exactLen(p.tgt, args[0]); ok {
			out = intersect(out, exact(n))
		}
		if k, ok := envKey(t); ok {
			if v, have := p.env[k]; have {
				out = intersect(out, v)
			}
		}
		return out, rebuilt
	}

	// A READ FROM A TABLE WHOSE ELEMENTS ARE RANGED. `(src i)` is an
	// application whose operator is a table, not a primitive — indexing IS
	// application (tables.md) — so this is where the element range is spent.
	if !known && len(vals) == 1 {
		if e, ok := p.elem[op.Name]; ok {
			return e, rebuilt
		}
	}
	out, checkable := p.transfer(op.Name, prim, vals)
	if checkable {
		if p.count {
			p.record(op.Name, out, t)
			p.rep.MaxOp = joinI(p.rep.MaxOp, out)
		}
		// THE SELECTION. Provable: keep the host's own operator, which is what
		// every program emits today. Not provable: use the checked primitive
		// the target declares — and if it declares none, that target cannot do
		// exact arithmetic and covering says so, which is the capability model
		// answering rather than a special case.
		if !out.fits() && prim.Checked != "" {
			kids[0] = core.Name(prim.Checked)
			if p.count {
				p.rep.Selected++
			}
		}
	}
	return out, rebuilt
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
	switch arithOp(name, len(v)) {
	case "add":
		return addI(v[0], v[1]), true
	case "sub":
		return subI(v[0], v[1]), true
	case "mul":
		return mulI(v[0], v[1]), true
	case "neg":
		return negI(v[0]), true
	case "div":
		return divI(v[0], v[1]), false
	case "rem":
		return remI(v[0], v[1]), false
	}
	return top, false // an integer from somewhere the analysis cannot see
}

// arithOp names the arithmetic an operator performs, across every spelling the
// four targets use. ONE place, because two things now ask: the transfer
// function, and NarrowByInterval deciding whether MaxOp bounded a value.
//
// Only add, sub, mul and neg are `checkable` and therefore joined into MaxOp.
// Division and remainder are bounded but not counted, so anything trusting
// MaxOp must not trust them — which is why this reports the operation rather
// than a yes/no, and the caller decides.
func arithOp(name string, n int) string {
	switch {
	case n == 2 && (isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add")):
		return "add"
	case n == 2 && (isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub")):
		return "sub"
	case n == 2 && (isOp(name, "mul") || name == "*" || strings.HasSuffix(name, ".imul")):
		return "mul"
	case n == 2 && (name == "/" || strings.HasSuffix(name, ".idiv") || isOp(name, "div")):
		return "div"
	case n == 2 && (name == "%" || strings.HasSuffix(name, ".irem") || isOp(name, "rem")):
		return "rem"
	case n == 1 && (isOp(name, "neg") || strings.HasSuffix(name, ".neg")):
		return "neg"
	}
	return ""
}

// CountedOp reports whether an application is one MaxOp has bounded.
func CountedOp(tgt *Target, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return false
	}
	prim, ok := tgt.Prims[t.Op().Name]
	if !ok || (prim.Result != "int" && prim.Result != "") {
		return false
	}
	switch arithOp(t.Op().Name, len(t.Args())) {
	case "add", "sub", "mul", "neg":
		return true
	}
	return false
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

func (p *intervalPass) let(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return top, t
	}
	v, nv := p.evalR(args[0])
	k := args[1]
	body, raw, _ := openFresh(k, map[string]bool{}, asmIdent)
	old, had := p.env[raw[0]]
	p.env[raw[0]] = v
	// A TABLE'S LENGTH SURVIVES THE BINDING. Call-by-need let-binds an argument
	// used more than once (ADR 0010), so a program that passes a literal
	// document to a tokeniser arrives here as
	// `(let (array 123 …) (fn (src) … (len src) …))` — the length is exactly
	// known at the binding and was thrown away one line later, leaving every
	// loop over it unbounded.
	oldE, hadE := p.bindElem(raw[0], args[0])
	lenKey := "len(" + raw[0] + ")"
	oldL, hadL := p.env[lenKey]
	if n, ok := exactLen(p.tgt, args[0]); ok {
		p.env[lenKey] = exact(n)
	}
	out, nb := p.evalR(body)
	if hadL {
		p.env[lenKey] = oldL
	} else {
		delete(p.env, lenKey)
	}
	p.unbindElem(raw[0], oldE, hadE)
	if had {
		p.env[raw[0]] = old
	} else {
		delete(p.env, raw[0])
	}
	// core.Fn closes an OPEN body, which is exactly what openFresh handed us.
	return out, core.App(t.Op(), nv, core.Fn(raw, nb))
}

func (p *intervalPass) cond(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) != 3 {
		return top, t
	}
	_, nc := p.evalR(args[0])
	saved := p.snapshot()
	p.refine(args[0], true)
	a, na := p.evalR(args[1])
	p.restore(saved)
	p.refine(args[0], false)
	b, nb := p.evalR(args[2])
	p.restore(saved)
	return joinI(a, b), core.App(t.Op(), nc, na, nb)
}

func (p *intervalPass) snapshot() map[string]ival {
	m := make(map[string]ival, len(p.env))
	for k, v := range p.env {
		m[k] = v
	}
	return m
}

// restore reinstalls a snapshot — as a COPY, and that is the whole of it.
//
// A snapshot is taken once and restored MORE THAN ONCE, and what follows a
// restore is `refine`, which narrows the environment IN PLACE. Installing the
// snapshot by reference therefore let the false-branch refinement mutate it, so
// the second restore undid nothing and the environment leaving an `if` carried
// `¬c` — a fact true on only one of the two paths.
//
// THE PROPERTY THIS RESTORES is monotonicity of the abstract step
//
//	F(c⃗) = z⃗ ⊔ ⨆{ ⟦a⃗⟧#(R_branch(c⃗)) : each `again` }
//
// which is what makes the widening sequence converge to a POST-fixpoint and
// what makes narrowing's `within(next, cur)` test legitimate: it descends
// within the post-fixpoints instead of leaving them. `refine` is monotone
// (intersect is monotone in its first argument, and the bound it derives is
// monotone in the environment), the abstract operations are monotone, and join
// is monotone — so F is monotone AS LONG AS each branch really is evaluated in
// R_branch(c⃗) rather than in R applied to something already narrowed.
//
// With the leak, it was not. Measured on examples/json/tree.oro: `[0,0] ⊑ [0,2]`
// and yet `F([0,0])[i] = [0,2]` while `F([0,2])[i] = [0,0]` — non-monotone, so
// the narrowing phase accepted a value that is not an over-approximation and
// `i` settled at its initial value (fixpoint-2026-08-27.md).
func (p *intervalPass) restore(m map[string]ival) {
	n := make(map[string]ival, len(m))
	for k, v := range m {
		n[k] = v
	}
	p.env = n
}

// exactLen is the length of a table the term itself determines.
//
//	(array e₁ … eₙ)   n, the graph's own size
//	(alloc (table n f)) n
//	(build n (fn (b) …)) n
//
// Sound because these are the only three constructors (tables.md §4) and each
// carries its length syntactically. A `set` hands back the buffer it was given,
// so it is followed through.
func exactLen(tgt *Target, t *core.Term) (int64, bool) {
	for i := 0; i < 8 && t != nil; i++ {
		if t.Kind != core.KApp || t.Op().Kind != core.KName {
			return 0, false
		}
		p, known := tgt.Prims[t.Op().Name]
		if !known {
			return 0, false
		}
		args := t.Args()
		switch p.Kind {
		case "array":
			return int64(len(args)), true
		case "table-build":
			if len(args) == 2 && args[0].Kind == core.KInt {
				return args[0].Int, true
			}
			return 0, false
		case "table-set":
			if len(args) != 3 {
				return 0, false
			}
			t = args[0] // a store hands the buffer back unchanged
		case "table-alloc":
			if len(args) != 1 {
				return 0, false
			}
			t = args[0]
		case "table":
			if len(args) == 2 && args[0].Kind == core.KInt {
				return args[0].Int, true
			}
			return 0, false
		default:
			return 0, false
		}
	}
	return 0, false
}

// elemRange is a table term's element range, from a declaration or from the
// syntax of its stores — never from the interval analysis. See intervalPass.elem.
func (p *intervalPass) elemRange(t *core.Term) (ival, bool) {
	for i := 0; i < 8 && t != nil; i++ {
		if t.Kind == core.KName {
			v, ok := p.elem[t.Name]
			return v, ok
		}
		if t.Kind != core.KApp || t.Op().Kind != core.KName {
			return ival{}, false
		}
		pr, known := p.tgt.Prims[t.Op().Name]
		if !known {
			return ival{}, false
		}
		args := t.Args()
		switch pr.Kind {
		case "array":
			// A GRAPH: the hull of the elements it was written with, which is
			// exact and needs no analysis. Reduction inlines, so a literal
			// document reaching a tokeniser arrives exactly like this.
			out, seen := ival{}, false
			for _, e := range args {
				if e.Kind != core.KInt {
					return ival{}, false
				}
				if !seen {
					out, seen = exact(e.Int), true
					continue
				}
				out = joinI(out, exact(e.Int))
			}
			return out, seen
		case "table-set":
			if len(args) != 3 {
				return ival{}, false
			}
			t = args[0] // a store hands the buffer back
		case "table-build":
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return ival{}, false
			}
			lam := args[1]
			body, raw, _ := openFresh(lam, map[string]bool{}, asmIdent)
			// A typeOf that knows nothing, so only literals and conditionals
			// over them decide — the non-circular half of bufferElem.
			ty := bufferElem(body, raw[0], func(*core.Term) string { return "" })
			lo, hi, ok := core.IntRange(ty)
			return ival{lo: lo, hi: hi}, ok
		default:
			return ival{}, false
		}
	}
	return ival{}, false
}

// bindElem records a name's element range, and reports whether it had one so the
// caller can restore.
func (p *intervalPass) bindElem(name string, from *core.Term) (ival, bool) {
	old, had := p.elem[name]
	if v, ok := p.elemRange(from); ok {
		p.elem[name] = v
	} else {
		delete(p.elem, name)
	}
	return old, had
}

func (p *intervalPass) unbindElem(name string, old ival, had bool) {
	if had {
		p.elem[name] = old
	} else {
		delete(p.elem, name)
	}
}

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
	case isOp(name, "eq") || name == "=" || name == "==" || strings.HasSuffix(name, ".sete"):
		rel = "eq"
	case isOp(name, "ne") || name == "!=" || strings.HasSuffix(name, ".setne"):
		rel = "ne"
	default:
		return
	}
	if !taken {
		rel = map[string]string{
			"lt": "ge", "ge": "lt", "le": "gt", "gt": "le", "eq": "ne", "ne": "eq",
		}[rel]
	}
	// EQUALITY narrows to the intersection; DISEQUALITY can only move an
	// endpoint, because `y ≠ 0` on [0, n] is [1, n] but `y ≠ 5` on [0, n] is a
	// hole no interval represents.
	//
	// The endpoint case is the one that matters and it is everywhere: `(== y 0)`
	// as a loop's exit guard means every other clause has y ≥ 1, which is what
	// makes Euclid's remainder a strict descent and what makes `k / 2` shrink.
	// Without it gcd and exponentiation-by-squaring were both unprovable.
	if rel == "eq" || rel == "ne" {
		p.narrowEq(a, rel, p.eval(b))
		p.narrowEq(b, rel, p.eval(a))
		return
	}
	p.narrow(a, rel, p.eval(b))
	p.narrow(b, map[string]string{"lt": "gt", "gt": "lt", "le": "ge", "ge": "le"}[rel], p.eval(a))
}

func (p *intervalPass) narrowEq(t *core.Term, rel string, other ival) {
	key, ok := envKey(t)
	if !ok {
		return
	}
	v := p.lookup(key)
	if rel == "eq" {
		p.env[key] = intersect(v, other)
		return
	}
	// A disequality against a KNOWN value, at an endpoint.
	if !other.bounded() || other.lo != other.hi {
		return
	}
	k := other.lo
	if !v.loInf && v.lo == k {
		v.lo = k + 1
	} else if !v.hiInf && v.hi == k {
		v.hi = k - 1
	}
	p.env[key] = v
}

func (p *intervalPass) narrow(t *core.Term, rel string, other ival) {
	key, ok := envKey(t)
	if !ok {
		p.narrowSquare(t, rel, other)
		return
	}
	v := p.lookup(key)
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
	p.env[key] = v
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
func (p *intervalPass) iterate(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return top, t
	}
	lam, inits := args[0], args[1:]
	initV := make([]ival, len(inits))
	nInits := make([]*core.Term, len(inits))
	for i, z := range inits {
		initV[i], nInits[i] = p.evalR(z)
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
	out, nb := p.evalR(body)
	for _, nm := range raw {
		delete(p.env, nm)
	}
	kids := append([]*core.Term{t.Op(), core.Fn(raw, nb)}, nInits...)
	return out, &core.Term{Kind: core.KApp, Kids: kids}
}

// collectAgain walks the clause chain, refining by each guard, and joins the
// interval of every `again` argument into acc.

// updateIval is the interval of an `again` argument's UPDATE SET: the join of
// the branches that do not simply hand the variable back.
//
// Only called where `selfContained` has already said yes, so the recursion is
// total. Conditions are still evaluated, because they contain operations the
// report has to count.
func (p *intervalPass) updateIval(a *core.Term, self string) ival {
	if a.Kind == core.KName && a.Name == self {
		return bottom // the pass-through contributes nothing new
	}
	if !mentionsName(a, self) {
		return p.eval(a)
	}
	if v, lam, ok := asLet(p.tgt, a); ok {
		p.eval(v)
		body, _, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
		return p.updateIval(body, self)
	}
	if a.Kind == core.KApp && a.Op().Kind == core.KName && len(a.Args()) == 3 {
		if pr, known := p.tgt.Prims[a.Op().Name]; known && pr.Kind == "cond" {
			p.eval(a.Args()[0])
			return joinI(p.updateIval(a.Args()[1], self), p.updateIval(a.Args()[2], self))
		}
	}
	return p.eval(a)
}

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
				if p.letTerm == nil {
					p.letTerm = map[string]*core.Term{}
				}
				oldT, hadT := p.letTerm[kraw[0]]
				p.letTerm[kraw[0]] = t.Args()[0]
				p.collectAgain(kb, raw, acc)
				if hadT {
					p.letTerm[kraw[0]] = oldT
				} else {
					delete(p.letTerm, kraw[0])
				}
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
				if i >= len(acc) {
					continue
				}
				// A RUNNING EXTREMUM contributes its UPDATE SET, not its whole
				// value. `mx = max(mx, sp+1)` has `mx` in one branch, so
				// evaluating the argument whole gives back `cur[mx]` and the
				// bound can never shrink — widening sends it to infinity and
				// narrowing cannot take it back. The reachable set is
				// `{z} ∪ U`, and `acc` already starts at `z`, so the
				// pass-through adds nothing (monotone.go, the reachable-set
				// theorem).
				if i < len(raw) && selfContained(p.tgt, a, raw[i]) {
					acc[i] = joinI(acc[i], p.updateIval(a, raw[i]))
					continue
				}
				acc[i] = joinI(acc[i], p.eval(a))
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

// unLet replaces a `let`-bound name by the term it was bound to, so a shape
// hidden behind a binding can still be recognised. It follows a chain and stops
// at anything that is not a bound name.
//
// Only ever used to LOOK for a shape; a name's interval still comes from the
// environment, so this cannot make a value more precise than the fixpoint said.
func (p *intervalPass) unLet(t *core.Term) *core.Term {
	for i := 0; i < 8 && t != nil && t.Kind == core.KName; i++ {
		v, ok := p.letTerm[t.Name]
		if !ok {
			return t
		}
		t = v
	}
	return t
}

// stepOfDerived is the FALLBACK step, for an argument only loop monotonicity
// can read. See DerivedStep for why it must never be tried first.
func (p *intervalPass) stepOfDerived(arg *core.Term, self string) (ival, bool) {
	c, ok := DerivedStep(p.tgt, arg, self, p.unLet)
	if !ok {
		return top, false
	}
	return ival{lo: c, hiInf: true}, true
}

// stepOf reads the per-iteration change of variable `self` from the expression
// that replaces it, when that expression is `self`, `self + e` or `self − e`.
func (p *intervalPass) stepOf(arg *core.Term, self string) (ival, bool) {
	if arg.Kind == core.KName && arg.Name == self {
		return exact(0), true
	}
	if v, ok := p.stepOfLoop(arg, self); ok {
		return v, true
	}
	if arg.Kind != core.KApp || arg.Op().Kind != core.KName || len(arg.Args()) != 2 {
		return p.stepOfDerived(arg, self)
	}
	name := arg.Op().Name
	a, b := arg.Args()[0], arg.Args()[1]
	plus := isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add")
	minus := isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub")
	if !plus && !minus {
		return p.stepOfDerived(arg, self)
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
	return p.stepOfDerived(arg, self)
}

// stepOfLoop is the step of a variable assigned the value of an inlined LOOP.
//
// A scanner is a loop, and after reduction the call to it IS the loop, so a
// counter advanced by one has no recognisable `self + c` shape and size change
// sees nothing. Loop monotonicity supplies the missing fact: if the loop's
// value is at least `self + c` then the step is at least c, and the step is
// unbounded above because the loop may run any number of times
// (monotone.go, the corollary).
func (p *intervalPass) stepOfLoop(arg *core.Term, self string) (ival, bool) {
	if !isLoopTerm(p.tgt, arg) {
		return top, false
	}
	z := LoopLowerBound(p.tgt, arg)
	if z == nil {
		return top, false
	}
	c, ok := selfPlus(p.tgt, z, self)
	if !ok {
		return top, false
	}
	return ival{lo: c, hiInf: true}, true
}

// relate is the size-change abstraction: what is known about the value of
// variable `dst` after this back edge, against variable `src` before it, in the
// ORIENTED measure.
func (p *intervalPass) relate(arg *core.Term, src string, srcSign, dstSign int) (arc, descent) {
	if a, d := p.relateSyntactic(arg, src, srcSign, dstSign); a != noArc {
		return a, d
	}
	// A DERIVED step, and only as a FALLBACK. Under the ascending measure
	// μ = −src a value at least `src + c` descends by c when c ≥ 1 and does not
	// increase when c = 0. Only for the ascending measure: a lower bound says
	// nothing about descent when the measure is +src.
	//
	// It gives the arc and WITHHOLDS the measure. `descent{}` keeps the position
	// out of `tripCount`, because `span / delta + 1` also needs `cur[src]`, and
	// src's value here comes out of the same opaque loop the bound came from.
	if srcSign < 0 && srcSign == dstSign {
		if c, ok := DerivedStep(p.tgt, arg, src, p.unLet); ok && c >= 1 {
			return down, descent{}
		}
	}
	return noArc, descent{}
}

func (p *intervalPass) relateSyntactic(arg *core.Term, src string, srcSign, dstSign int) (arc, descent) {
	if srcSign == 0 || dstSign == 0 || srcSign != dstSign {
		// A cross-arc between measures pointing opposite ways says nothing that
		// this analysis can use.
		return noArc, descent{}
	}
	// μ' = μ, when the argument IS the source variable.
	if arg.Kind == core.KName && arg.Name == src {
		return downEq, descent{}
	}
	// AN INLINED LOOP. A scanner's call reduces to the scanner's loop, so the
	// argument has no `src ± e` shape and this saw nothing at all. Loop
	// monotonicity gives the shape back: the value is at least `src + c`, so
	// under the ascending measure μ = −src it descends by c when c ≥ 1 and does
	// not increase when c = 0 (monotone.go).
	//
	// Only for the ascending measure. A lower bound says nothing about descent
	// when the measure is +src: `src + c` with c ≥ 0 is not smaller than src.
	if isLoopTerm(p.tgt, arg) {
		if z := LoopLowerBound(p.tgt, arg); z != nil {
			if c, ok := selfPlus(p.tgt, z, src); ok && srcSign < 0 {
				if c >= 1 {
					return down, descent{kind: 1, delta: c}
				}
				if c == 0 {
					return downEq, descent{}
				}
			}
		}
		return noArc, descent{}
	}
	if arg.Kind != core.KApp || arg.Op().Kind != core.KName || len(arg.Args()) != 2 {
		return noArc, descent{}
	}
	name := arg.Op().Name
	a, b := arg.Args()[0], arg.Args()[1]

	// EUCLID. `x mod y` is strictly less than y when y ≥ 1, which is an arc
	// from y to y through an expression y does not head — and it is the whole
	// reason gcd terminates. Neither variable descends on its own there: x
	// becomes y, and y becomes x mod y.
	//
	// This is the shape Lee, Jones & Ben-Amram use to motivate the principle,
	// and the analysis could not see it until the corpus contained it.
	if b.Kind == core.KName && b.Name == src && srcSign > 0 &&
		(isOp(name, "rem") || name == "%") {
		if v := p.lookup(src); !v.loInf && v.lo >= 1 {
			if d := p.eval(a); !d.loInf && d.lo >= 0 {
				return down, descent{kind: 1, delta: 1}
			}
		}
	}
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
