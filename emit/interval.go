package emit

import (
	"fmt"
	"math/big"
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

// ival is [lo, hi] over the integers, with infinities. An empty interval is not
// represented: unreachable code is not what this measures.
type ival struct {
	lo, hi       bnd
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
var bottom = ival{lo: bOne, hi: bZero}

func (v ival) isBottom() bool { return !v.loInf && !v.hiInf && v.lo.gt(v.hi) }

func exact(v int64) ival { return ival{lo: bi(v), hi: bi(v)} }

// rng is [lo, hi] from two int64 endpoints, which is how nearly every interval
// the pass builds from a literal, a length or a table's range is written.
func rng(lo, hi int64) ival { return ival{lo: bi(lo), hi: bi(hi)} }

func (v ival) bounded() bool { return !v.loInf && !v.hiInf }

// fitsIn reports whether every value in the interval is inside a target's
// word, which is the condition under which native arithmetic computes the
// integer the language means and no overflow check is needed (ADR 0026: the
// word is the target's, where it used to be one window for all of them).
func (v ival) fitsIn(w core.Word) bool {
	return v.isBottom() || (v.bounded() && v.lo.ge(bi(w.Lo)) && v.hi.le(bi(w.Hi)))
}

func (v ival) String() string {
	l, h := v.lo.String(), v.hi.String()
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
// overflow. Anything that saturates becomes infinite, which is sound. The
// endpoint type (bound.go) holds ℤ exactly to 2^126 — enough for every
// target's word, ADR 0026 — and says when a result has left it.

func addI(a, b ival) ival {
	out := ival{}
	if a.loInf || b.loInf {
		out.loInf = true
	} else if v, ok := a.lo.add(b.lo); ok {
		out.lo = v
	} else {
		out.loInf = true
	}
	if a.hiInf || b.hiInf {
		out.hiInf = true
	} else if v, ok := a.hi.add(b.hi); ok {
		out.hi = v
	} else {
		out.hiInf = true
	}
	return out
}

func negI(a ival) ival {
	out := ival{lo: a.hi.neg(), hi: a.lo.neg(), loInf: a.hiInf, hiInf: a.loInf}
	return out
}

func subI(a, b ival) ival { return addI(a, negI(b)) }

func mulI(a, b ival) ival {
	if !a.bounded() || !b.bounded() {
		// A product with an unbounded factor is unbounded, EXCEPT when the
		// other factor is exactly zero.
		if a.bounded() && a.lo == bZero && a.hi == bZero {
			return exact(0)
		}
		if b.bounded() && b.lo == bZero && b.hi == bZero {
			return exact(0)
		}
		return top
	}
	var lo, hi bnd
	for i, p := range [4][2]bnd{{a.lo, b.lo}, {a.lo, b.hi}, {a.hi, b.lo}, {a.hi, b.hi}} {
		v, ok := p[0].mul(p[1])
		if !ok { // the product itself left the endpoints
			return top
		}
		if i == 0 || v.lt(lo) {
			lo = v
		}
		if i == 0 || v.gt(hi) {
			hi = v
		}
	}
	return ival{lo: lo, hi: hi}
}

// divI is conservative: truncating division never increases magnitude except
// for the one degenerate case, and a divisor that may be zero says nothing.
func divI(a, b ival) ival {
	// THE DIVISOR CONTRACTS, and using it is what makes a CARRY CHAIN provable.
	//
	// |a / b| <= |a| / min|b|, because truncation toward zero only ever moves a
	// quotient closer to zero and the SMALLEST divisor produces the largest
	// quotient. Ignoring the divisor is sound and it was throwing away the one
	// fact a bignum needs: `c' = (bounded + c) / 2^24` contracts, so the carry
	// settles in two iterations — and without the contraction it looks like
	// `c' = bounded + c`, which grows, widens to infinity, and takes every limb
	// operation in the program with it.
	//
	// Zero is not excluded from a divisor's interval here. Division by zero is a
	// PRECONDITION the refinement layer discharges separately (integers.md §5),
	// and an abstract transfer function may not assume a precondition it does
	// not check — so a divisor that might be 0 or +-1 contracts by 1, which is
	// exactly the old behaviour.
	d := bOne
	switch {
	case !b.loInf && b.lo.ge(bOne):
		d = b.lo
	case !b.hiInf && b.hi.le(bi(-1)):
		d = b.hi.neg()
	}
	// A non-negative dividend and a positive divisor keep the sign, and keeping
	// it matters: without this the decimal printer's `m / 10` lost m's floor,
	// and with no floor there is no well-founded descent and the loop could not
	// be shown to terminate.
	if !a.loInf && a.lo.sign() >= 0 && !b.loInf && b.lo.ge(bOne) {
		out := ival{lo: bZero, hiInf: a.hiInf}
		if !a.hiInf {
			out.hi = a.hi.quo(d)
		}
		return out
	}
	if !a.bounded() {
		return top
	}
	m := maxB(a.hi, a.lo.neg())
	return ival{lo: m.quo(d).neg(), hi: m.quo(d)}
}

// remI is the remainder's transfer, INDUCED from `lang`'s four remainder facts
// (lang-facts.oro) by Theorem T with case splitting (fact.go, inducedTransfer).
//
// It was hand-written here as |a % b| <= min(|a|, |b| − 1) with the sign of the
// dividend, and the same law was written twice more — `storedRange`'s literal
// divisor (F7) and `big%-small`'s transfer (F8). All three now read one
// declaration. Both halves of the minimum stay load-bearing, as two pairs of
// clauses: `7 % b` for an unbounded b is at most 7, which only the dividend
// gives; `a % 16777216` for an unbounded a is under 2^24, which only the
// divisor gives — every carry split in a bignum.
//
// Never less precise than the hand-written version, and strictly tighter where a
// bounded dividend spans zero: [−5, 7] % 10 is [−5, 7], where min(|a|, |b|−1)
// said [−7, 7]. TestTheInducedRemainderIsNeverLessPrecise holds it to that.
func remI(a, b ival) ival { return inducedTransfer(langFacts, "rem", []ival{a, b}, 1) }

// maxAbs is the largest magnitude a BOUNDED interval contains.
func maxAbs(v ival) bnd { return maxB(v.hi.abs(), v.lo.abs()) }

func joinI(a, b ival) ival {
	if a.isBottom() {
		return b
	}
	if b.isBottom() {
		return a
	}
	out := ival{lo: a.lo, hi: a.hi, loInf: a.loInf || b.loInf, hiInf: a.hiInf || b.hiInf}
	if b.lo.lt(out.lo) {
		out.lo = b.lo
	}
	if b.hi.gt(out.hi) {
		out.hi = b.hi
	}
	return out
}

// widen is the classical widening: a bound that moved goes to infinity. Without
// it a loop counter's fixpoint does not terminate.
func widen(old, new ival) ival {
	out := new
	if new.loInf || (!old.loInf && new.lo.lt(old.lo)) {
		out.loInf = true
	}
	if new.hiInf || (!old.hiInf && new.hi.gt(old.hi)) {
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
	if !b.loInf && !a.loInf && a.lo.lt(b.lo) {
		return false
	}
	if !b.hiInf && !a.hiInf && a.hi.gt(b.hi) {
		return false
	}
	return true
}

// intersect keeps only what both intervals allow. Sound when both are.
func intersect(a, b ival) ival {
	out := a
	if !b.loInf && (a.loInf || b.lo.gt(a.lo)) {
		out.lo, out.loInf = b.lo, false
	}
	if !b.hiInf && (a.hiInf || b.hi.lt(a.hi)) {
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
// RequireResult is one measured argument (core.Env.Requires): the range its
// parameter declares, the interval the analysis gives it at the call, and
// whether the second lies inside the first.
type RequireResult struct {
	Def, Param, Type, Arg string
	Got                   ival
	Proven                bool
}

// inDeclared reports v ⊑ the declared range, at full precision.
func inDeclared(v ival, ty string) bool {
	if v.isBottom() {
		return true // unreachable
	}
	if lo, hi, ok := core.IntRange(ty); ok {
		return within(v, rng(lo, hi))
	}
	if w, ok := wideRange(ty); ok {
		return within(v, w)
	}
	return false
}

// MeasureRequires decides every measurement mark left in a residual and returns
// the residual with the marks erased, which is the term reduction gives without
// them. It changes nothing downstream: the caller continues with the stripped
// term exactly as it would have without measuring.
func MeasureRequires(tgt *Target, sig *core.Sig, t *core.Term) ([]RequireResult, *core.Term) {
	rep, _ := Intervals(tgt, sig, t, 0)
	return rep.Requires, core.StripRequires(t)
}

type IntervalReport struct {
	Requires []RequireResult // measurement marks decided on the counted walk
	Ops      int             // integer operations that would need an overflow check
	Proven   int             // …of those, the ones provably inside the target's word
	Target   string          // the target the report is about (ADR 0026: legality is per target)
	Word     core.Word       // …and its word
	// Outside is set when an unproven operation IS bounded, only not by this
	// target's word — the case where declaring the range is the answer.
	Outside   bool
	Worded    int  // operations and conversions the unsigned-word pass rewrote
	InU       bool // an unproven operation's interval lies in U (wordsel.go)
	Unproven  []string
	ByOp      map[string][2]int // operation -> {proven, total}
	LoopVars  int
	LoopBound int

	BigReuse int // big operations that write into storage instead of allocating

	Selected   int      // operations rewritten to a checked primitive
	Shifted    int      // divisions and remainders rewritten to a shift or a mask
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

	// BigOps is how many operations were selected into the target's
	// arbitrary-precision form (bigrep.go). They are deliberately NOT counted in
	// Ops: a bignum cannot leave the window, so it is not an operation the
	// window accounting has anything to say about.
	BigOps int
}

type intervalPass struct {
	tgt *Target

	// REPRESENTATION SELECTION, the rung above the host's word (bigrep.go).
	//
	// `big` is the set of names held in arbitrary precision, keyed by the fresh
	// names this pass itself hands out — which is why the decision is made HERE
	// and not in a pass of its own: a separate walk would have to reproduce
	// `openFresh`'s naming exactly, and "the obvious design has no key" is
	// already recorded in this repository as a thing that was built and found
	// to record nothing the emitter could find.
	big        map[string]bool
	bigReads   map[string]bool     // names read by a big operation — rule (P)
	bigChanged bool                // did this sweep promote anything?
	wantBig    bool                // the signature declares a result above the window
	demandBig  bool                // this position's value must BE a bignum
	loopTail   bool                // …and the loop being iterated sits in one
	loopRaw    []string            // the enclosing loop's variables, for `again`
	bigVal     map[*core.Term]bool // rebuilt terms whose value is a bignum
	noChecked  bool                // select big, but not the checked arithmetic
	selecting  bool                // this pass SELECTS a representation, rather than reporting on one
	noDest     bool                // …and not the mutable-bignum rewrite either
	shiftOnly  bool                // …and nothing at all except division-to-shift

	// THE UNSIGNED WORD (wordsel.go). `words` is the selection mode; u64 names
	// the binders represented in U, u64Val the rebuilt terms that are, and
	// wordLoop the variables of the loop whose final walk is under way — kept
	// apart from loopRaw, which that walk has already restored to the outer
	// loop's.
	words    bool
	u64      map[string]bool
	u64Val   map[*core.Term]bool
	wordLoop []string
	limbs    bool // big is OUR representation, so the host need not have one

	// bound is every name this pass has already opened a binder with, shared by
	// every `openFresh` call so that two binders never get the same fresh name.
	//
	// IT WAS AN EMPTY MAP AT EVERY CALL SITE, and that is variable CAPTURE, not
	// untidiness. `openFresh` renames only against the set it is given, so a
	// nested loop whose parameter is spelled the same as an enclosing one — `i`
	// inside `i`, which is what an inlined helper produces constantly — got the
	// SAME fresh name, and the inner `core.Fn` then bound the outer's
	// occurrences too.
	//
	// It was invisible because the rebuilt term is DISCARDED unless `-checked`
	// is on, which is the same reason the `FnClosed` bug hid here. Under
	// `-checked` the windows hash table read its probe counter where the key
	// should have been, so every insert after the first hashed to slot 0, found
	// it taken, and reported the table full: a five-entry map answered `len` 1.
	bound   map[string]bool
	env     map[string]ival
	rep     *IntervalReport
	count   bool  // only the final pass counts
	assume  int64 // simulated declared bound on parameters; 0 means none
	assumed bool

	// sig and params are the enclosing contract, carried so that a FROZEN
	// buffer's element range can be asked for with the same premises the
	// emitter asks with. What bounds a node table is usually something the
	// signature says — `(where (< (len src) 1024))` is what makes `nn < 512`
	// reachable — so asking without it gets a refusal, which is what the
	// structural fix in rebench-2026-08-27 was about.
	sig    *core.Sig
	params []string

	// depth caps the recursion in elemRange, which may analyse a nested build
	// to learn what it holds. Nesting is finite, so this is not needed for
	// TERMINATION; it is a cost bound. Refusing is always sound, so a cap can
	// only lose precision.
	depth int

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

	// ARRAY SMASHING (smash.go). tabElem is E for a rebuilt table-valued term;
	// noSmash turns it off, for the buffer sub-pass that chooses storage. The
	// smash* fields are the enclosing loop's back-edge accumulator.
	tabElem      map[*core.Term]ival
	noSmash      bool
	smashRaw     []string
	smashTracked []bool
	smashAcc     []ival
	smashOK      []bool

	// THE DELTA ROUND (smash.go, the copy-closed joint invariant): dgroup is the
	// loop's table variables, whose delta is ⊥; dcell a derived binder's delta;
	// tabDelta a rebuilt term's; smashDAcc the back-edge accumulator.
	dmode     bool
	dgroup    map[string]bool
	dcell     map[string]ival
	tabDelta  map[*core.Term]ival
	smashDAcc []ival

	// BOUNDED INCREMENTS (smash.go): a store of a G-slot read plus a literal is
	// counted rather than joined — dInc joins the literals, dIncEdge counts them
	// along the back edge being evaluated and dIncMax over all edges, dDepth is
	// how many loops deep inside the round's own loop a store sits, and dIncBad
	// records an increment the count cannot bound.
	dReadOf  map[string]*core.Term // a binder bound to a read of a G-derived table: the read
	dInc     ival
	dIncSeen bool
	dIncBad  bool
	dIncEdge int
	dIncMax  int
	dDepth   int

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
	kind  int // 0 none, 1 linear, 2 geometric
	delta bnd // linear: the least decrease per step, ≥ 1
	base  bnd // geometric: the divisor, ≥ 2
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
	return bufferRangeSeeded(tgt, lam, sig, params, nil)
}

func bufferRangeSeeded(tgt *Target, lam *core.Term, sig *core.Sig,
	params []string, seed map[string]ival) (string, bool) {

	if lam == nil || lam.Kind != core.KFn || len(lam.Params) != 1 {
		return "", false
	}
	rep, _ := intervalsAssumingSeeded(tgt, lam, sig, params, seed, true)
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
	// A STORE OUTSIDE THE WORD narrows nothing: every rung of the ladder is
	// inside the word, so the host's own width is the answer — the one an
	// infinite endpoint gives, which is what such a store was while endpoints
	// saturated at 2^62.
	if !out.fitsIn(tgt.Word) {
		return "", false
	}
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
	if v.lo.gt(v.hi) { // bottom: no integer operation at all
		return true
	}
	return v.lo.ge(bi(-2147483648)) && v.hi.le(bi(2147483647))
}

// MaxOpRange prints the join of every operation's interval, for the tool that
// answers whether narrowing could fire at all.
func (r *IntervalReport) MaxOpRange() string {
	v := r.MaxOp
	if v.isBottom() {
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
	// ABOVE THIS TARGET'S WORD a result's range is a representation
	// declaration, not a contract (ADR 0026, and the reader's own comment on
	// why): the value is arbitrary precision and the interval domain reports ⊤
	// for it by construction. Only what the author wrote is checked.
	ens := sig.Ensures
	if tgt.ValueType(sig.Result) == core.BigType {
		if ens = sig.Stated(); ens == nil {
			return true, ""
		}
	}
	rep, _ := Intervals(tgt, sig, t, 0)
	ok, decided := entailsIval(tgt, ens, rep.Result)
	if !decided {
		return true, "postcondition is outside the decidable fragment, " +
			"propagated and not proven: " + ens.String()
	}
	if !ok {
		return false, "the body does not establish " + ens.String() +
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
			return !v.hiInf && v.hi.le(bi(k)), true
		case isOp(name, "lt"):
			return !v.hiInf && v.hi.lt(bi(k)), true
		case isOp(name, "ge"):
			return !v.loInf && v.lo.ge(bi(k)), true
		case isOp(name, "gt"):
			return !v.loInf && v.lo.gt(bi(k)), true
		}
	case isResult(rhs):
		k, ok := konst(lhs)
		if !ok {
			return false, false
		}
		switch {
		case isOp(name, "le"): // K <= result
			return !v.loInf && v.lo.ge(bi(k)), true
		case isOp(name, "lt"): // K < result
			return !v.loInf && v.lo.gt(bi(k)), true
		case isOp(name, "ge"): // K >= result
			return !v.hiInf && v.hi.le(bi(k)), true
		case isOp(name, "gt"): // K > result
			return !v.hiInf && v.hi.lt(bi(k)), true
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
	return intervalsAssumingSeeded(tgt, lam, sig, params, nil, false)
}

func intervalsAssumingSeeded(tgt *Target, lam *core.Term, sig *core.Sig,
	params []string, seed map[string]ival, noSmash bool) (*IntervalReport, *core.Term) {

	flags := []bool{false, false, false, false, noSmash}
	if sig == nil || sig.Where == nil || len(params) == 0 {
		return intervals(tgt, nil, lam, 0, seed, flags...)
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
	return intervals(tgt, &core.Sig{Where: core.Rename2(sig.Where, sub)}, lam, 0, seed, flags...)
}

func Intervals(tgt *Target, sig *core.Sig, t *core.Term, assume int64) (*IntervalReport, *core.Term) {
	return intervals(tgt, sig, t, assume, nil)
}

// intervals is Intervals with a SEED: facts about names that are free in `t`,
// carried in from the enclosing analysis.
//
// It exists for one caller — elemRange asking what a frozen buffer holds — and
// the seed is deliberately restricted to EXACT LENGTHS. The reason is soundness
// and it is worth stating: a length fact comes either from `exactLen` on a
// literal table or from a signature's `where`, so it is syntactic or a premise;
// neither is a fixpoint iterate. Seeding an iterate would be seeding a claim
// that is not yet a post-fixpoint. See lengthFacts.
func intervals(tgt *Target, sig *core.Sig, t *core.Term, assume int64,
	seed map[string]ival, flags ...bool) (*IntervalReport, *core.Term) {

	rep := &IntervalReport{ByOp: map[string][2]int{}, Stores: map[string]ival{}, Target: tgt.Name, Word: tgt.Word}
	rep.MaxOp = bottom
	p := &intervalPass{tgt: tgt, rep: rep, assume: assume, assumed: assume > 0,
		bound: map[string]bool{}, big: map[string]bool{}, bigReads: map[string]bool{},
		noChecked: len(flags) > 0 && flags[0],
		// THE SAME DECISION SEEN FROM TWO SIDES. A pass either selects a
		// representation or reports on one already selected, and the two are
		// exclusive: `PromoteBig` promotes and declines the checked forms,
		// `Intervals` runs afterwards on the promoted term to count and to take
		// `-checked`.
		//
		// Gating the big selection on it is not tidiness. A constant whose exact
		// value leaves the window is a bignum (bigrep.go), and without this the
		// REPORTING pass selected it too — so the operation vanished from the
		// overflow accounting while the emitted term kept the machine-word
		// multiply, and `(+ n 123456789012345678901234567890)` compiled to a
		// wrapping constant with `int` as its declared result.
		selecting: len(flags) > 0 && flags[0],
		// THE FIXED-LIMB RUNG SUPPRESSES THE DESTINATION REWRITE. `big+!` writes
		// into a host bignum object; a limb value is a table, and rule R's
		// liveness argument is about neither. The two are different answers to
		// the same allocation question and a program takes one (biglimb.go).
		noDest: len(flags) > 1 && flags[1],
		// THE LIMB RUNG NEEDS NO HOST BIGNUM, which is the whole reason it
		// exists on windows. Every gate below asks `bigOK` rather than
		// `HasBig`, or a target with nothing to fall back to would promote
		// nothing and then be refused for producing a machine word.
		limbs: len(flags) > 2 && flags[2],
		// SHIFT SELECTION ONLY. A pass that rewrites a division into a shift
		// must not also promote a big constant or take the checked forms:
		// `PromoteBig` has already chosen a representation by the time this
		// runs, and re-selecting would promote what is already promoted.
		shiftOnly: len(flags) > 3 && flags[3],
		// NO ARRAY SMASHING, for the sub-pass whose stores decide a buffer's
		// STORAGE width (smash.go): smashing bounds values, never representation.
		noSmash: len(flags) > 4 && flags[4],
		// THE UNSIGNED WORD'S SELECTION ONLY (wordsel.go): no checked forms, no
		// bignum, no shifts — a representation is being chosen, not reported on.
		words: len(flags) > 5 && flags[5]}
	if p.words {
		p.u64, p.u64Val = map[string]bool{}, map[*core.Term]bool{}
	}
	env := map[string]ival{}
	for k, v := range seed {
		env[k] = v
	}
	var head *core.Term
	if t.Kind == core.KFn {
		head = t
		for _, n := range t.Params {
			env[n] = p.paramIval(n, sig)
			// A RANGE NO TERM CAN STATE is still the parameter's premise: an
			// export ASSUMES its parameters' types, and a premise reaches the
			// analysis as a term — int64 endpoints — so [0, 2^64−1] would arrive
			// as [0, +inf]. Read at full precision instead (ADR 0026).
			if sig != nil {
				for _, sp := range sig.Params {
					if sp.Name != n {
						continue
					}
					if v, ok := wideRange(sp.Type); ok && tgt.ValueType(sp.Type) == core.U64Type {
						env[n] = intersect(env[n], v)
					}
					if p.words && tgt.ValueType(sp.Type) == core.U64Type {
						p.u64[n] = true
					}
				}
			}
		}
		p.sig, p.params = sig, t.Params
		t = t.Body()
	}
	p.env = env
	// REPRESENTATION SEEDS (bigrep.go). A parameter declared above the portable
	// window IS a bignum on the way in; a result declared above it is the DEMAND
	// that makes the pass bidirectional. These two are the only sources — every
	// other big value in the program is derived from one of them, which is ADR
	// 0019's blast-radius claim made structural rather than asserted.
	if p.bigOK() && sig != nil {
		if tgt.ValueType(sig.Result) == core.BigType {
			p.wantBig = true
		}
		if head != nil {
			for i, n := range head.Params {
				if i < len(sig.Params) && tgt.ValueType(sig.Params[i].Type) == core.BigType {
					p.big[n] = true
				}
			}
		}
	}
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
				p.elem[n] = rng(lo, hi)
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
	p.demandBig = p.wantBig // the function's own result is the outermost demand
	p.eval(t)               // settle loop fixpoints, and reach the representation fixpoint
	p.count = true
	p.demandBig = p.wantBig
	res, out := p.evalR(t)
	rep.Result = res
	sort.Strings(rep.Unproven)
	if head != nil {
		// `t = t.Body()` above OPENED this binder, so the rebuilt body carries the
		// parameters as NAMES and `core.Fn` — which closes — is what puts them
		// back. It has never mattered: nothing encloses the head, so no later pass
		// renames it and a free `n` beside a parameter `n` emits correctly. It is
		// fixed because the same mistake one level down is what bigreuse.go's
		// `let` was, and a structural invariant that holds by luck at one site is
		// not an invariant.
		out = core.Fn(head.Params, out)
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
				return rng(0, p.assume)
			}
		}
	}
	if p.assumed {
		return rng(0, p.assume)
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
		return rng(0, p.assume)
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
		// `Body()` OPENS — it turns this lambda's KBound indices into KNames —
		// so the rebuilt body has to be CLOSED again. `FnClosed` does not close;
		// it takes a body whose indices are already intact, which is what the
		// reducer always has and what this walker never has.
		//
		// Getting it wrong leaves the parameter's occurrences as free names, so
		// the binder no longer binds them. The rebuilt term is only USED under
		// `-checked`, which is why it survived: `examples/json/tree.oro`
		// compiled to Go referring to a `nodes` from a different function, and
		// go build refused it. `p.let` has the same shape and gets it right,
		// with a comment saying so.
		type shadow struct {
			v        ival
			had, did bool
		}
		sh := make([]shadow, len(t.Params))
		for i, n := range t.Params {
			if p.dmode && p.dgroup[n] {
				sh[i].v, sh[i].had, sh[i].did = p.shadowDelta(n, top)
			}
		}
		v, b := p.evalR(t.Body())
		fn := core.Fn(t.Params, b)
		if e, ok := p.elemOf(b); ok { // while the body's names are still bound
			p.setElemOf(fn, e)
		}
		if d, ok := p.deltaOf(b); ok {
			p.setDeltaOf(fn, d)
		}
		for i, n := range t.Params {
			p.unshadowDelta(n, sh[i].v, sh[i].had, sh[i].did)
		}
		return v, fn
	case core.KApp:
		return p.app(t)
	}
	return top, t
}

// multiPrim evaluates a host call with SEVERAL RESULTS under its eliminator —
// `((os.ReadFile p) (fn (src err) …))` — binding each continuation parameter to
// what the primitive declares it gives back.
//
// WITHOUT THIS, ADR 0019 IS VACUOUS INSIDE EVERY SUCH CONTINUATION. The operator
// of the outer application is itself an application, so the walk fell through to
// `return top, t` and never entered the body: the same unbounded `(* a n)` was
// REFUSED in an ordinary function and silently ACCEPTED here. Since a
// continuation is how every program that opens a file is written, the hole was
// exactly the shape of the new code — the same failure mode bigrep-2026-09-02
// found on JavaScript, where bounded-by-default was enforced on three targets
// and vacuous on the fourth.
//
// It is also where a declared result becomes a FACT: a scalar range binds the
// parameter's interval, and an `(array (int LO HI))` binds its ELEMENT range, so
// a program that copies bytes out of `os.ReadFile`'s result can have the copy
// narrowed. A primitive has no body; the declaration is the only source.
func (p *intervalPass) multiPrim(t *core.Term) (ival, *core.Term, bool) {
	pr, args, k, ok := multiPrimCall(p.tgt, t)
	if !ok {
		return top, t, false
	}
	nargs := make([]*core.Term, len(args))
	nvals := make([]ival, len(args))
	for i, a := range args {
		nvals[i], nargs[i] = p.evalR(a)
	}
	// Arguments in their parameters' representations, as `app` converts them.
	if p.words {
		for i := range nargs {
			if i < len(pr.Args) {
				switch p.tgt.ValueType(pr.Args[i]) {
				case core.U64Type:
					nargs[i] = p.toRep(nargs[i], true)
				case "int":
					if p.u64Term(nargs[i]) && p.inS(nvals[i]) {
						nargs[i] = p.toRep(nargs[i], false)
					}
				}
			}
		}
	}
	body, raw, unbind := p.bindMulti(pr, k)
	v, nb := p.evalR(body)
	// THE ELIMINATOR'S VALUE IS ITS BODY'S, representation included: a projection
	// `((Div64 …) (fn (q r) q))` is a u64 when q is. Read while the binders'
	// representations are still in scope.
	bodyU := p.words && p.u64Term(nb)
	unbind()
	op := &core.Term{Kind: core.KApp, Kids: append([]*core.Term{t.Op().Op()}, nargs...)}
	rebuilt := &core.Term{Kind: core.KApp, Kids: []*core.Term{op, core.Fn(raw, nb)}}
	if bodyU {
		p.u64Val[rebuilt] = true
	}
	return v, rebuilt, true
}

// bindMulti opens a host call's continuation and binds each parameter to what
// the call's declared result says: its interval, its element range, its
// representation. It returns the opened body and the function that puts back
// whatever the names meant before and hands them back to the pool.
//
// Shared by the two places a continuation is walked, as an expression
// (multiPrim) and as a clause chain (collectAgain): ADR 0027 makes the second
// one's `again` a back edge, and a back edge read under different facts than
// the expression path reads them would be a second analysis of one term.
func (p *intervalPass) bindMulti(pr Prim, k *core.Term) (*core.Term, []string, func()) {
	body, raw, _ := openFresh(k, p.bound, asmIdent)
	uOld := make([][2]bool, len(raw))
	type saved struct {
		v    ival
		had  bool
		e    ival
		hadE bool
	}
	old := make([]saved, len(raw))
	for i := range raw {
		old[i].v, old[i].had = p.env[raw[i]]
		old[i].e, old[i].hadE = p.elem[raw[i]]
		delete(p.env, raw[i])
		delete(p.elem, raw[i])
		if p.words {
			o, had := p.u64[raw[i]]
			uOld[i] = [2]bool{o, had}
			delete(p.u64, raw[i])
		}
		if i < len(pr.Results) {
			if lo, hi, isR := core.IntRange(pr.Results[i]); isR {
				p.env[raw[i]] = rng(lo, hi)
			} else if v, ok := wideRange(pr.Results[i]); ok && p.tgt.ValueType(pr.Results[i]) == core.U64Type {
				p.env[raw[i]] = v // `strconv.ParseUint`'s value, in U
			}
			if p.words && p.tgt.ValueType(pr.Results[i]) == core.U64Type {
				p.u64[raw[i]] = true
			}
			if lo, hi, isR := core.IntRange(core.ArrayElem(pr.Results[i])); isR {
				p.elem[raw[i]] = rng(lo, hi)
			}
		}
	}
	return body, raw, func() {
		for i := range raw {
			restoreVar(p.env, raw[i], old[i].v, old[i].had)
			restoreVar(p.elem, raw[i], old[i].e, old[i].hadE)
			if p.words {
				if uOld[i][1] {
					p.u64[raw[i]] = uOld[i][0]
				} else {
					delete(p.u64, raw[i])
				}
			}
		}
		p.releaseBound(raw)
	}
}

// mapCase evaluates a map read under its eliminator, binding what the two
// continuation parameters can hold.
//
//	#t  the tag, [0, 1] — a sum with two variants and nothing else
//	#p  the payload, the map's VALUE RANGE
//
// The payload's bound is the theorem in elemRange's `map-build` case: every
// value a read produces was inserted, so the hull of the inserts bounds it.
func (p *intervalPass) mapCase(t *core.Term) (ival, *core.Term, bool) {
	op := t.Op()
	args := t.Args()
	if op.Kind != core.KApp || len(args) != 1 || len(op.Args()) != 1 {
		return top, t, false
	}
	k := args[0]
	if k.Kind != core.KFn || len(k.Params) != 2 {
		return top, t, false
	}
	val, ok := p.elemRange(op.Op())
	if !ok {
		return top, t, false
	}
	body, raw, _ := openFresh(k, p.bound, asmIdent)
	oldT, hadT := p.env[raw[0]]
	oldP, hadP := p.env[raw[1]]
	p.env[raw[0]] = rng(0, 1)
	p.env[raw[1]] = val
	v, nb := p.evalR(body)
	restoreVar(p.env, raw[0], oldT, hadT)
	restoreVar(p.env, raw[1], oldP, hadP)
	p.releaseBound(raw)
	return v, &core.Term{Kind: core.KApp, Kids: []*core.Term{op, core.Fn(raw, nb)}}, true
}

func restoreVar(env map[string]ival, n string, old ival, had bool) {
	if had {
		env[n] = old
	} else {
		delete(env, n)
	}
}

// ruleTable binds a rule-table's parameter to its own domain.
//
// `(table n f)` IS `f` restricted to `Fin n` (tables.md §2), so `f`'s parameter
// is bounded by construction — a DEFINITION, not an inference, and the same
// fact `(a i)`'s bounds check discharges.
//
// Without it a rule's index was ⊤, so `(a (+ j 1))` inside a kernel could not
// be bounded and every arithmetic operation on the index went unproven. It
// showed up as an asymmetry rather than as a wrong answer: `smooth-build`
// proved, because its index becomes a LOOP variable with a guard, and
// `smooth-alloc` did not, because its index stays a rule parameter. The same
// program, two presentations of the same table, two different answers.
func (p *intervalPass) ruleTable(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return top, t
	}
	n, nn := p.evalR(args[0])
	body, raw, _ := openFresh(args[1], p.bound, asmIdent)
	// `Fin n` is [0, n-1]. A length is non-negative and bounded (tables.md
	// §2.3.1), so an unknown `n` still gives a non-negative index.
	idx := ival{lo: bZero, hi: n.hi, hiInf: n.hiInf}
	if !n.hiInf && n.hi.sign() > 0 {
		idx.hi, _ = n.hi.sub(bOne)
	}
	old, had := p.env[raw[0]]
	p.env[raw[0]] = idx
	_, nb := p.evalR(body)
	restoreVar(p.env, raw[0], old, had)
	p.releaseBound(raw)
	return top, &core.Term{Kind: core.KApp, Kids: []*core.Term{
		t.Op(), nn, core.Fn(raw, nb)}}
}

// releaseBound drops a binder's fresh names when the pass leaves its scope.
//
// The set must hold exactly the ENCLOSING binders and no more. Holding them
// forever renames siblings too — the second `(loop ((i 0)) …)` in a function
// would get `i2` though no `i` is in scope — and across the pass's several
// sweeps it renames the same binder again on every one, so the rebuilt term
// stops being the input even when nothing was selected. Scoped, a name is
// renamed exactly when it would otherwise be captured.
func (p *intervalPass) releaseBound(raw []string) {
	for _, n := range raw {
		delete(p.bound, asmIdent(n))
	}
}

func (p *intervalPass) app(t *core.Term) (ival, *core.Term) {
	op := t.Op()
	if op.Kind != core.KName {
		// A MAP READ UNDER ITS ELIMINATOR — `((m k) (fn (#t #p) body))`, the one
		// shape whose operator is legitimately not a name.
		//
		// Without this the whole thing is ⊤ and every operation downstream of a
		// map read is unbounded, which is what `examples/map/dynamic.oro`
		// measured at 2 of 4. growth.md called the map's value range FREE; it is
		// free only once the analysis is told about it.
		if v, nt, ok := p.mapCase(t); ok {
			return v, nt
		}
		if v, nt, ok := p.multiPrim(t); ok {
			return v, nt
		}
		return top, t
	}
	prim, known := p.tgt.Prims[op.Name]
	args := t.Args()

	// A MEASUREMENT MARK (core.Env.Requires): the argument's value, and whether
	// it lies in the range its parameter declares, recorded on the counted walk.
	// The mark is dropped from the rebuilt term.
	if op.Name == core.RequireName && len(args) == 4 {
		v, nv := p.evalR(args[3])
		if p.count {
			p.rep.Requires = append(p.rep.Requires, RequireResult{
				Def: args[0].Str, Param: args[1].Str, Type: args[2].Str,
				Arg: args[3].String(), Got: v, Proven: inDeclared(v, args[2].Str)})
		}
		return v, nv
	}

	if known {
		switch prim.Kind {
		case "let":
			return p.let(t)
		case "cond":
			return p.cond(t)
		case "iterate":
			return p.iterate(t)
		case "table":
			return p.ruleTable(t)
		}
	}

	vals := make([]ival, len(args))
	kids := make([]*core.Term, 0, len(t.Kids))
	kids = append(kids, op)
	// A POSITION DEMANDS A BIGNUM WHEN THE PRIMITIVE SAYS SO, and otherwise it
	// demands nothing. `let`, `if` and `loop` are handled above and each carries
	// the demand to where the value actually goes; everything else consumes its
	// arguments, and only a declared `big` parameter makes one a big position.
	//
	// This is what lets a WHOLE PROGRAM use arbitrary precision at all. Reduction
	// inlines every non-exported call, so a helper's declared big result is gone
	// by the time this pass runs — the same structural limit already recorded for
	// a `where` on an internal definition and for index narrowing. What survives
	// is the BOUNDARY: `(big-str …)` is where a value past 2^53 has to be named,
	// because no host can print one as an `int`, and that is exactly where the
	// demand comes from.
	outerDemand := p.demandBig
	for i, a := range args {
		// AN `again` ARGUMENT IS DEMANDED BY ITS LOOP VARIABLE AND BY NOTHING
		// ELSE, because `again` is a JUMP and not a value (ADR 0015). Letting
		// the demand on the position the LOOP sits in reach its back edge
		// promoted the loop counter of every bignum loop — `(+ i 1)` is an
		// arithmetic operation in a big-demanded position and there is nothing
		// locally wrong with that reading, which is why it has to be said here.
		if op.Name == "again" {
			p.demandBig = i < len(p.loopRaw) && p.big[p.loopRaw[i]]
		} else if op.Name == core.AscribeName {
			// AN ASCRIPTION IS A DEMAND, and it is the one the signature used to
			// carry: `(the "int 0 …" e)` says e is in that set, and a set wider
			// than the portable window is a request for arbitrary precision.
			// This is where a declaration that reduction inlined away arrives.
			p.demandBig = i == 1 && ascribedBig(p.tgt.Word, args)
		} else {
			p.demandBig = known && i < len(prim.Args) && prim.Args[i] == core.BigType
		}
		// THE ZERO FILL (smash.go): inside `(build n λx.e)`, E(x) = [0,0].
		zeroed, zOld, zHad := "", ival{}, false
		dOld, dHad, dDid := ival{}, false, false
		if known && prim.Kind == "table-build" && i == 1 && a.Kind == core.KFn && len(a.Params) == 1 && !p.noSmash {
			zeroed = a.Params[0]
			zOld, zHad = p.elem[zeroed]
			p.elem[zeroed] = exact(0)
			dOld, dHad, dDid = p.shadowDelta(zeroed, exact(0)) // a fresh buffer, not G's
		}
		v, na := p.evalR(a)
		if zeroed != "" {
			restoreVar(p.elem, zeroed, zOld, zHad)
			p.unshadowDelta(zeroed, dOld, dHad, dDid)
		}
		// The widen is decided on the REBUILT argument: `selectBig` may have
		// just turned it into a bignum, and wrapping that would be a type error
		// rather than a conversion.
		// GATED ON `selecting`, because INSERTING A WIDEN IS A SELECTION. A pass
		// either selects a representation or reports on one already chosen, and
		// the two are exclusive — the comment on `p.selecting` says so, and this
		// is the second site to have got it wrong.
		//
		// `PromoteBig` erases the ascriptions once it has read them, so the
		// reporting pass cannot see that a `let`-bound value is a bignum and
		// wrapped it again: `big-of(x)` on an `x` that is already a `*big.Int`,
		// which Go refuses. It only showed under `-checked`, whose term is the
		// reporting pass's, and only in a `main` with no signature to seed from.
		if p.selecting && p.demandBig && p.bigOK() && !p.bigTerm(na) {
			na = core.App(core.Name("big-of"), na)
		}
		vals[i] = v
		kids = append(kids, na)
	}
	p.demandBig = outerDemand
	// A BACK EDGE CARRIES EACH VALUE IN ITS LOOP VARIABLE'S REPRESENTATION
	// (wordsel.go): the variable is ρ of its fixpoint interval, and the value's
	// interval is inside it, so the conversion is the identity on it.
	if p.words && op.Name == "again" && p.wordLoop != nil {
		for i := 1; i < len(kids) && i-1 < len(p.wordLoop); i++ {
			kids[i] = p.toRep(kids[i], p.u64[p.wordLoop[i-1]])
		}
	}
	// A HOST CALL TAKES EACH ARGUMENT IN ITS PARAMETER'S REPRESENTATION
	// (wordsel.go). Into a parameter in U an `int` is converted — the range's
	// lower end is an obligation the refinement layer discharges, so the value
	// is non-negative and the conversion is the identity on it; into one in the
	// signed word a u64 is converted only where its interval is inside the word.
	// The language's own operators are selectWord's, not this.
	if p.words && known && !langArith(op.Name) && !langCompare(op.Name) {
		for i := 1; i < len(kids) && i-1 < len(prim.Args); i++ {
			switch p.tgt.ValueType(prim.Args[i-1]) {
			case core.U64Type:
				kids[i] = p.toRep(kids[i], true)
			case "int":
				if p.u64Term(kids[i]) && p.inS(vals[i-1]) {
					kids[i] = p.toRep(kids[i], false)
				}
			}
		}
	}
	rebuilt := &core.Term{Kind: core.KApp, Kids: kids}

	// THE RUNG ABOVE THE WORD (bigrep.go). Selected before `transfer`, because a
	// big operation is not an operation the window accounting sees: its result
	// is `big`, not `int`, so it is neither counted nor refused.
	if nt, ok := p.selectBig(t, kids, vals); ok {
		if p.count {
			p.rep.BigOps++
		}
		return top, nt
	}

	if op.Name == "again" {
		p.setElemOf(rebuilt, bottom) // a jump has no value, so no elements
		p.setDeltaOf(rebuilt, bottom)
		return bottom, rebuilt // a back edge produces no value
	}

	// THE CELL OF A TABLE-VALUED TERM (smash.go): a store is a weak update, a
	// build is its body's cell with the zero fill, a graph is the hull of its
	// elements.
	if known && !p.noSmash {
		switch {
		case prim.Kind == "table-set" && len(args) == 3:
			if e, ok := p.elemOf(kids[1]); ok {
				p.setElemOf(rebuilt, joinI(e, vals[2]))
			}
			if d, ok := p.deltaOf(kids[1]); ok {
				p.setDeltaOf(rebuilt, joinI(d, p.storedDelta(kids[3], vals[2])))
			}
		case prim.Kind == "table-build" && len(args) == 2:
			if e, ok := p.elemOf(kids[2]); ok {
				p.setElemOf(rebuilt, joinI(e, exact(0)))
			}
		case prim.Kind == "array":
			if e, ok := p.elemRange(t); ok {
				p.setElemOf(rebuilt, e)
			}
		}
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

	// A LENGTH IS BOUNDED AT BOTH ENDS, and neither end needs a declaration.
	//
	// Non-negative is obvious. The upper end is ADR 0012: `(len t)` returns an
	// `int`, `int` is exact within ±(2^53−1), and a table with more elements
	// than that has a length the language cannot count — so it is outside the
	// language and every guarantee about indexing it has already failed. A
	// target may declare something tighter (Target.MaxLen); Java can, because
	// `arraylength` returns a 32-bit `int`.
	//
	// This is the fact 32 of the corpus's unproven operations were waiting for,
	// and every one of them is `(+ i 1)` under a guard `(>= i (len a))`: the
	// guard already bounds `i` by `len a` — non-relationally, in `refine` —
	// and `len a` was the thing with no bound.
	if len(vals) == 1 && isLenOp(op.Name) {
		out := rng(0, p.tgt.MaxLenOf())
		if p.assumed {
			out = rng(0, p.assume)
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
	// DIVISION BY A POWER OF TWO IS A SHIFT WHEN THE DIVIDEND IS NON-NEGATIVE,
	// and that is a PROOF rather than a declaration — which is why it needs no
	// new operator in the language and applies to any program.
	//
	// `x / 2^k` on a SIGNED value is not a shift: truncation toward zero needs a
	// rounding correction, and Go, the JVM and x86 all emit one. Measured on our
	// own fixed-limb factorial that correction is worth **2.39x**, the dominant
	// cost in the program and larger than the clamp, the element mask and the
	// buffer clear together (shiftdiv-2026-09-03).
	if p.selecting && !p.words {
		if k, m, ok := shiftFor(p.tgt, arithOp(op.Name, len(vals)), vals); ok {
			if shr, and, have := p.tgt.ShiftNames(); have {
				if m == 0 {
					kids = []*core.Term{core.Name(shr), kids[1], core.Int(k)}
				} else {
					kids = []*core.Term{core.Name(and), kids[1], core.Int(m)}
				}
				rebuilt = &core.Term{Kind: core.KApp, Kids: kids}
				// COUNTED UNCONDITIONALLY, unlike every other tally here. The
				// others report on a program; this one decides whether the
				// rewritten TERM is used at all, and most of these operations
				// sit inside a lambda, where `count` is off. Gating it there
				// made the pass do the work and hand back the original.
				p.rep.Shifted++
			}
		}
	}
	out, checkable := p.transfer(op.Name, prim, vals)
	if p.words {
		if nt := p.selectWord(op.Name, kids, vals, out); nt != nil {
			rebuilt = nt
		}
	}
	// A BUILD'S VALUE IS ITS BODY'S. `(build n λb.e)` is `e` with b the zero-filled
	// buffer, and `vals[1]` — the λ's interval — is e's. A body that hands the
	// buffer back has a table value, whose interval says nothing either way; a
	// body that returns a count (a scratch buffer: a stack, a worklist) has the
	// count's, and that was ⊤ for want of saying so.
	if known && prim.Kind == "table-build" && len(vals) == 2 {
		out = intersect(out, vals[1])
	}
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
		if !out.fitsIn(p.tgt.Word) && prim.Checked != "" && !p.noChecked && !p.shiftOnly {
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
	// THE REMAINDER BY A WORD IS BOUNDED BY THAT WORD, whatever the magnitude of
	// the value it came from — which is the whole reason `big%-small` has an
	// `int` result. Without this the answer is ⊤ and every ordinary word
	// operation reading a digit group is unprovable, so a program that renders a
	// bignum is refused for arithmetic on values under 10^8.
	//
	// `[-(k-1), k-1]` rather than `[0, k-1]`: `%` takes the DIVIDEND's sign on
	// all four hosts and on `math/big`, and nothing here has proved the dividend
	// non-negative. The symmetric bound is sound for either sign.
	if name == "big%-small" && len(v) == 2 {
		return remI(v[0], v[1]), false // the same law as `%`, F8 (lang-facts.oro)
	}
	// A DECLARED RESULT RANGE. The target says what the host call gives back,
	// and that is the only source there is: a primitive has no body, so nothing
	// can be derived from it — which is the same reason `ensures` belongs on a
	// `prim` and is redundant on an internal definition (postconditions.md).
	//
	// Without this, EVERY host call is ⊤, so ADR 0019's bounded-by-default
	// refuses arithmetic on any result from any ecosystem and `-checked` is the
	// only way through. Measured on two independent ecosystems before it was
	// built: `(fmt.print-int (GetCurrentProcessId))` is refused on windows and
	// `bits.OnesCount64(n) + 1` is refused on Go, for one cause
	// (gostdlib-2026-09-06 §4b, win32-2026-09-06 §5). A `DWORD` is
	// 0..4294967295 and the header says so; `OnesCount64` is 0..64 and the doc
	// says so. This is where a target gets to say it.
	//
	// IT MUST COME BEFORE THE `Result != "int"` BAIL BELOW, which is where the
	// first attempt died: a range is an `int` FOR TYPING and a distinct string
	// in the type language, so the guard that keeps `bool` and `f64` out of the
	// arithmetic also kept `int 0 64` out. That is scalarrange-2026-08-31's
	// three effects of a range arriving in a fourth place.
	//
	// It is `(int LO HI)` in the RESULT POSITION rather than an `ensures`,
	// because scalarrange-2026-08-31 established that a range in a result
	// position IS an ensures — the same claim in the type language — and
	// because `ensures` feeds the refinement layer while being in-window is
	// decided here. Two layers, and only one of them was ever told.
	if lo, hi, ok := core.IntRange(prim.Result); ok {
		return rng(lo, hi), false
	}
	// A RESULT IN U (ADR 0026): `strconv.ParseUint` returns [0, 2^64−1], whose
	// upper end no int64 holds.
	if v, ok := wideRange(prim.Result); ok && p.tgt.ValueType(prim.Result) == core.U64Type {
		return v, false
	}
	// THE UNSIGNED WORD'S OPERATIONS ARE THE LANGUAGE'S, on the integers they are
	// proven to hold (wordsel.go), and its conversions are the identity on them.
	if name == "u64-of" || name == "int-of-u64" {
		if len(v) == 1 {
			return v[0], false
		}
		return top, false
	}
	if lang, ok := wordLang(name); ok {
		name, prim.Result = lang, "int"
	}
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
	case "and":
		return andI(v[0], v[1]), false
	case "shr":
		return shrI(v[0], v[1]), false
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
// uncheckedName maps a target's checked spelling back to the operation it
// checks, so the analysis counts it as what it is.
//
// The spellings are the four targets' own — `add-exact` and `mul-exact` on Go,
// `addExact` and `multiplyExact` on the JVM, `add-checked` and `imul-checked`
// on x86 — and they are matched by SUFFIX, the way every other operator name is
// here.
func uncheckedName(name string) (string, bool) {
	seg := name
	if i := strings.LastIndex(seg, "."); i >= 0 {
		seg = seg[i+1:]
	}
	switch seg {
	case "add-exact", "addExact", "add-checked":
		return "add", true
	case "sub-exact", "subtractExact", "sub-checked":
		return "sub", true
	case "mul-exact", "multiplyExact", "imul-checked":
		return "mul", true
	}
	return "", false
}

func arithOp(name string, n int) string {
	// A CHECKED OPERATION IS STILL THE OPERATION, and the analysis has to see
	// it. Under `-checked` the term handed to a backend has `Math.addExact` and
	// `add-exact` where the plain operators were, and none of those matched
	// below — so `MaxOp` came out BOTTOM, which `FitsIndex` reads as "no integer
	// operation at all" and answers true.
	//
	// Java then narrowed a method whose values are genuinely unbounded: `long i`
	// beside `int j = (i + 1)`, which javac refuses. That is the whole-method
	// narrowing rule broken by the analysis being blind rather than by the rule
	// being wrong — and the two conditions are contradictory by construction,
	// since narrowing wants every operation inside a 32-bit index while
	// `-checked` exists because some operation is not even inside the far larger
	// portable window.
	if base, ok := uncheckedName(name); ok {
		name = base
	}
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
	// A MASK AND A SHIFT, because `SelectShifts` PRODUCES them and an analysis
	// blind to what it emitted would throw away everything downstream. It did:
	// with these missing, the fixed-limb factorial lost its element narrowing
	// (`[]uint32` back to `[]int`) and its loop-carried buffer reuse, because
	// the store's value range and the loop variable's went to the top.
	//
	// They are matched here and NOT added to `opAlias`, which the refinement
	// layer's linear fragment also reads: `x & m` is not a linear term and
	// teaching that fragment to think it is would be unsound.
	case n == 2 && (name == "&" || strings.HasSuffix(name, ".&") || isOp(name, "and")):
		return "and"
	case n == 2 && (name == ">>" || strings.HasSuffix(name, ".>>") ||
		isOp(name, "shr") || isOp(name, "sar")):
		return "shr"
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
	// A LANGUAGE OPERATION IS PROVEN ONLY INSIDE THE SIGNED WORD; one the
	// unsigned-word pass rewrote is proven inside U. So an operation in U that
	// the pass did not reach is refused rather than emitted as a wrapping `int`.
	proven := out.fitsIn(p.tgt.Word)
	if isWordOp(name) {
		proven = p.tgt.Word.Unsigned && inU(out)
	}
	if proven {
		p.rep.Proven++
		e[0]++
	} else {
		s := t.String()
		if len(s) > 64 {
			s = s[:61] + "..."
		}
		p.rep.Unproven = append(p.rep.Unproven, fmt.Sprintf("%s %s in %s", name, out, s))
		if out.bounded() {
			p.rep.Outside = true
		}
		if p.tgt.Word.Unsigned && inU(out) {
			p.rep.InU = true // the unsigned word would hold it: see DeclaresWord
		}
	}
	p.rep.ByOp[name] = e
}

func (p *intervalPass) let(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return top, t
	}
	outerDemand := p.demandBig
	p.demandBig = false
	v, nv := p.evalR(args[0])
	p.demandBig = outerDemand
	k := args[1]
	body, raw, _ := openFresh(k, p.bound, asmIdent)
	old, had := p.env[raw[0]]
	p.env[raw[0]] = v
	// RULE (S), SUPPLY: a name bound to a big expression is big. This is where
	// arbitrary precision crosses a binder, and it is why `bigTerm` may answer
	// NO for a `let` body without losing anything — the name inside carries the
	// fact instead of the shape outside having to see through it.
	oldBig, hadBig := p.big[raw[0]], p.big != nil
	if p.big != nil {
		if p.bigTerm(args[0]) {
			p.big[raw[0]] = true
		} else {
			delete(p.big, raw[0])
		}
	}
	// A NAME BOUND TO A u64 IS ONE (wordsel.go) — the binder takes its value's
	// representation, so nothing converts here.
	oldU, hadU := p.u64[raw[0]]
	if p.words {
		if p.u64Term(nv) {
			p.u64[raw[0]] = true
		} else {
			delete(p.u64, raw[0])
		}
	}
	// A TABLE'S LENGTH SURVIVES THE BINDING. Call-by-need let-binds an argument
	// used more than once (ADR 0010), so a program that passes a literal
	// document to a tokeniser arrives here as
	// `(let (array 123 …) (fn (src) … (len src) …))` — the length is exactly
	// known at the binding and was thrown away one line later, leaving every
	// loop over it unbounded.
	oldE, hadE := p.bindElem(raw[0], args[0])
	// A BINDER IS ITS VALUE, cell included (smash.go) — met with what the
	// syntactic and declared sources say, both being sound.
	if e, ok := p.elemOf(nv); ok {
		if have, had := p.elem[raw[0]]; had {
			e = intersect(have, e)
		}
		p.elem[raw[0]] = e
	}
	lvOld, lvHad, lvDid := ival{}, false, false
	if d, ok := p.deltaOf(nv); ok {
		lvOld, lvHad, lvDid = p.shadowDelta(raw[0], d)
	}
	// A BINDER BOUND TO A READ IS THAT READ (smash.go, storedDelta).
	rdOld, rdHad := p.dReadOf[raw[0]], false
	if p.dmode {
		_, rdHad = p.dReadOf[raw[0]]
		if p.derivedRead(nv) {
			p.dReadOf[raw[0]] = nv
		} else {
			delete(p.dReadOf, raw[0])
		}
	}
	lenKey := "len(" + raw[0] + ")"
	oldL, hadL := p.env[lenKey]
	if n, ok := exactLen(p.tgt, args[0]); ok {
		p.env[lenKey] = exact(n)
	}
	out, nb := p.evalR(body)
	wasBigBody := p.bigTerm(nb) // recorded before the binder goes out of scope
	wasU64Body := p.u64Term(nb)
	if p.words {
		if hadU {
			p.u64[raw[0]] = oldU
		} else {
			delete(p.u64, raw[0])
		}
	}
	bodyE, bodyHasE := p.elemOf(nb)
	bodyD, bodyHasD := p.deltaOf(nb)
	p.unshadowDelta(raw[0], lvOld, lvHad, lvDid)
	if p.dmode {
		if rdHad {
			p.dReadOf[raw[0]] = rdOld
		} else {
			delete(p.dReadOf, raw[0])
		}
	}
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
	if hadBig {
		if oldBig {
			p.big[raw[0]] = true
		} else {
			delete(p.big, raw[0])
		}
	}
	p.releaseBound(raw)
	// core.Fn closes an OPEN body, which is exactly what openFresh handed us.
	rebuilt := core.App(t.Op(), nv, core.Fn(raw, nb))
	p.markBig(rebuilt, wasBigBody)
	if p.words && wasU64Body {
		p.u64Val[rebuilt] = true
	}
	if bodyHasE {
		p.setElemOf(rebuilt, bodyE)
	}
	if bodyHasD {
		p.setDeltaOf(rebuilt, bodyD)
	}
	return out, rebuilt
}

func (p *intervalPass) cond(t *core.Term) (ival, *core.Term) {
	args := t.Args()
	if len(args) != 3 {
		return top, t
	}
	outerDemand := p.demandBig
	p.demandBig = false
	_, nc := p.evalR(args[0])
	p.demandBig = outerDemand
	saved := p.snapshot()
	p.refine(args[0], true)
	a, na := p.evalR(args[1])
	p.restore(saved)
	p.refine(args[0], false)
	b, nb := p.evalR(args[2])
	p.restore(saved)
	// THE TWO ARMS MUST AGREE ON A REPRESENTATION, because no host can type a
	// conditional whose branches are a machine word and a bignum — Go and Java
	// would not compile it and JavaScript would throw a TypeError on the first
	// arithmetic that mixed them. The join in the two-point lattice is `big`, so
	// the word arm is widened rather than the big one narrowed; narrowing is the
	// direction that loses a value.
	// Decided on the REBUILT arms, and a JUMP is never widened: an `again` has
	// no value to convert, and wrapping one emitted `big-of(goto)`.
	if p.big != nil && p.bigOK() {
		ba, bb := p.bigTerm(na), p.bigTerm(nb)
		if ba && !bb && !isJumpTerm(nb) {
			nb = core.App(core.Name("big-of"), nb)
		} else if bb && !ba && !isJumpTerm(na) {
			na = core.App(core.Name("big-of"), na)
		}
	}
	// THE ARMS AGREE ON ρ OF THE CONDITIONAL'S INTERVAL (wordsel.go). Each arm's
	// interval is inside the join, so the conversion is the identity on it.
	rebuiltU := false
	if p.words {
		if j := joinI(a, b); !j.isBottom() {
			rebuiltU = p.wantU(j)
			na, nb = p.toRep(na, rebuiltU), p.toRep(nb, rebuiltU)
		}
	}
	rebuilt := core.App(t.Op(), nc, na, nb)
	if rebuiltU {
		p.u64Val[rebuilt] = true
	}
	if ea, ok := p.elemOf(na); ok {
		if eb, ok := p.elemOf(nb); ok {
			p.setElemOf(rebuilt, joinI(ea, eb))
		}
	}
	if da, ok := p.deltaOf(na); ok {
		if db, ok := p.deltaOf(nb); ok {
			p.setDeltaOf(rebuilt, joinI(da, db))
		}
	}
	return joinI(a, b), rebuilt
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
		case "map-insert":
			// AN INSERT HANDS THE MAP BACK, exactly as a store hands a buffer
			// back — arrays-revisited.md §6's point again, that the discipline
			// does not care what the index set is.
			if len(args) != 3 {
				return ival{}, false
			}
			t = args[0]
		case "map":
			// A MAP LITERAL: the hull of the VALUES it was written with, which
			// is exact. The keys are the index set and are not values.
			out, seen := ival{}, false
			for _, row := range args {
				if row.Kind != core.KApp || len(row.Kids) != 2 || row.Kids[1].Kind != core.KInt {
					return ival{}, false
				}
				if !seen {
					out, seen = exact(row.Kids[1].Int), true
					continue
				}
				out = joinI(out, exact(row.Kids[1].Int))
			}
			return out, seen
		case "map-build":
			// THE MAP'S VALUE RANGE, and it is frozen-2026-08-28's buffer
			// theorem one index set over: a slot holds either NOTHING or the
			// most recent `insert`, there being no third source — `build-map`
			// is the only allocator, `insert` the only store, and ADR 0018's
			// linearity means nothing else can have written it.
			//
			// So the hull of the inserted values bounds every value a read can
			// produce. The absent case needs no join, because absence is a
			// SUM: the `none` branch never sees a payload.
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return ival{}, false
			}
			body, raw, _ := openFresh(args[1], p.bound, asmIdent)
			return p.insertedRange(body, raw[0])
		case "table-set":
			if len(args) != 3 {
				return ival{}, false
			}
			// A STORE HANDS THE BUFFER BACK, and its range is the base's only
			// while no buffer being filled has one — which was true of every
			// source this function had: a declared range is a premise the stores
			// must meet. Array smashing gives a build binder the zero fill and a
			// loop variable a computed cell, and then `(set c i v)` read as `c`
			// drops v: `(let (set x 11 234) …)` met the right cell E(x) ⊔ 234
			// with the stale E(x), and a program printing -231 was claimed inside
			// -80..15 (TestArraySmashingContains, seed 315). So in a smashing
			// pass a store's cell is `elemOf`'s weak update and nothing else.
			if !p.noSmash {
				return ival{}, false
			}
			t = args[0]
		case "table-build":
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return ival{}, false
			}
			lam := args[1]
			body, raw, _ := openFresh(lam, p.bound, asmIdent)
			// A typeOf that knows nothing, so only literals and conditionals
			// over them decide.
			noTypes := func(*core.Term) string { return "" }
			if lo, hi, ok := core.IntRange(bufferElem(body, raw[0], noTypes)); ok {
				return rng(lo, hi), true
			}
			// A FROZEN BUFFER CARRIES WHAT WAS PUT IN IT, and this is where the
			// interval analysis is allowed to say so.
			//
			// THEOREM (read containment). Let b = (build n λx.e) and let
			// E = ElemType(b). Then every value read out of b is in γ(E).
			//
			// PROOF. A slot holds either the zero fill or the value of the most
			// recent `set` into it — there is no third source, `build` being
			// the only allocator, `set` the only store, and ADR 0018's
			// linearity meaning no other reference can have written it. E joins
			// the zero fill explicitly and contains every stored value. ∎
			//
			// WHY IT IS NOT CIRCULAR, which is the objection this had to answer
			// before it could be built. Stratify the reads of b:
			//
			//	stratum 0  a read inside λx.e itself, where the buffer is still
			//	           being filled. Nothing binds x's element range — the
			//	           only binder is `let`, and a build term is never in
			//	           scope inside itself — so such a read is ⊤.
			//	stratum 1  a read of b from OUTSIDE λx.e, after the freeze,
			//	           where the value no longer changes.
			//
			// E(b) is computed by analysing λx.e, in which every read of b sits
			// at stratum 0. So computing E(b) never consults E(b), and the
			// induction on build-nesting depth extends it: an outer build may
			// learn what a completed inner one holds, and the inner analysis
			// cannot mention the outer.
			//
			// examples/json/tree.oro is the program that needs it. `walk` takes
			// the node table as a PARAMETER — the buffer is frozen on the way
			// out and read back as an ordinary array — so `(nodes k)` there is
			// stratum 1, and it was ⊤, and 45 of the 50 unproven operations in
			// that program were that one fact missing.
			//
			// The self-referential case still refuses, and correctly: the
			// worklist stores a depth read back out of ITSELF, so its stores
			// are stratum-0 reads and BufferRange declines. Refusing is the
			// safe direction and stays the default.
			if p.depth < 4 {
				p.depth++
				r, ok := bufferRangeSeeded(p.tgt, lam, p.sig, p.params, p.lengthFacts(lam))
				p.depth--
				if ok {
					if lo, hi, ok := core.IntRange(r); ok {
						return rng(lo, hi), true
					}
				}
			}
			return ival{}, false
		default:
			return ival{}, false
		}
	}
	return ival{}, false
}

// insertedRange is the hull of every value inserted into `name` inside a
// `build-map` body — the map's half of `bufferElem`.
//
// SYNTACTIC ON PURPOSE, like the buffer's: a literal is its own exact range and
// anything else refuses. That is a soundness choice rather than laziness — a
// range too narrow would let a read be believed tighter than it is, and only
// facts exact by construction are used. Refusing is always safe.
//
// The stratification is frozen-2026-08-28's and holds for the same reason: this
// walks the build lambda, where the map's own name is the binder, so it never
// consults a range it is in the middle of computing.
func (p *intervalPass) insertedRange(body *core.Term, name string) (ival, bool) {
	out, seen, bad := ival{}, false, false
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil || bad {
			return
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if pr, ok := p.tgt.Prims[t.Op().Name]; ok && pr.Kind == "map-insert" {
				if a := t.Args(); len(a) == 3 {
					if a[2].Kind != core.KInt {
						bad = true
						return
					}
					if !seen {
						out, seen = exact(a[2].Int), true
					} else {
						out = joinI(out, exact(a[2].Int))
					}
				}
			}
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(body)
	if bad {
		return ival{}, false
	}
	return out, seen
}

// bindElem records a name's element range, and reports whether it had one so the
// caller can restore.
func (p *intervalPass) bindElem(name string, from *core.Term) (ival, bool) {
	old, had := p.elem[name]
	v, ok := p.elemRange(from)
	if ok {
		p.elem[name] = v
	} else {
		delete(p.elem, name)
	}
	return old, had
}

// lengthFacts is the seed carried into a sub-analysis: the exactly-known
// lengths the enclosing pass has established, and nothing else.
//
// WHY ONLY LENGTHS. The sub-analysis runs on a lambda in isolation, where
// everything the enclosing program bound is free and therefore ⊤. That is sound
// but it loses the one fact a buffer's contents usually depend on: how long the
// input is. Reduction INLINES, so `examples/json/tree.oro`'s `run` substitutes
// four literal documents, and each `(len src)` is an array literal's length —
// exact at the binding, and invisible one lambda in.
//
// A length fact is either syntactic (`exactLen` on a literal table) or a
// premise (`assumeWhere` on a signature). Neither is a fixpoint iterate, which
// is what makes it safe to carry: seeding an iterate would be seeding a claim
// that is not yet a post-fixpoint, and the sub-analysis would believe it.
//
// SHADOWING. A fact about `len(x)` is dropped when the lambda binds `x` itself,
// because the inner `x` is a different table and the outer length says nothing
// about it.
func (p *intervalPass) lengthFacts(lam *core.Term) map[string]ival {
	var out map[string]ival
	for k, v := range p.env {
		if !strings.HasPrefix(k, "len(") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(k, "len("), ")")
		shadowed := false
		for _, n := range lam.Params {
			if n == inner {
				shadowed = true
			}
		}
		if shadowed {
			continue
		}
		if out == nil {
			out = map[string]ival{}
		}
		out[k] = v
	}
	return out
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
	// AN OPERAND IS EVALUATED ONLY WHEN THE OTHER SIDE CAN BE NARROWED. `cond`
	// has already evaluated the whole condition, and refines once per branch, so
	// evaluating both operands here unconditionally cost five evaluations of every
	// operand per `if` — and an operand containing an `if` pays that again, so the
	// cost was exponential in how deeply conditions nest inside conditions. A
	// clamped table read is two nested `if`s, and `(= (cmp (cs (pat …)) …) 0)` put
	// two of them under a comparison under a clause: tally.oro's build did not
	// finish in five minutes (tally-2026-09-11). Narrowing a literal or a call was
	// always a no-op — `narrow` and `narrowEq` return at once without a key — so
	// skipping the evaluation that fed it loses no fact.
	if rel == "eq" || rel == "ne" {
		if _, ok := envKey(a); ok {
			p.narrowEq(a, rel, p.eval(b))
		}
		if _, ok := envKey(b); ok {
			p.narrowEq(b, rel, p.eval(a))
		}
		return
	}
	flip := map[string]string{"lt": "gt", "gt": "lt", "le": "ge", "ge": "le"}[rel]
	if narrowable(a, rel) {
		p.narrow(a, rel, p.eval(b))
	}
	if narrowable(b, flip) {
		p.narrow(b, flip, p.eval(a))
	}
}

// narrowable is exactly what `narrow` can act on: a key, or — below a bound —
// the square of one.
func narrowable(t *core.Term, rel string) bool {
	if _, ok := envKey(t); ok {
		return true
	}
	if rel != "lt" && rel != "le" {
		return false
	}
	_, ok := squareOf(t)
	return ok
}

// squareOf recognises `(* x x)` and names x — the one non-key shape a guard can
// narrow, by taking a square root of the bound.
func squareOf(t *core.Term) (string, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return "", false
	}
	name := t.Op().Name
	if !isOp(name, "mul") && name != "*" && !strings.HasSuffix(name, ".imul") {
		return "", false
	}
	a, b := t.Args()[0], t.Args()[1]
	if a.Kind != core.KName || b.Kind != core.KName || a.Name != b.Name {
		return "", false
	}
	return a.Name, true
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
	if k1, ok := k.add(bOne); ok && !v.loInf && v.lo == k {
		v.lo = k1
	} else if k1, ok := k.sub(bOne); ok && !v.hiInf && v.hi == k {
		v.hi = k1
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
		if h, ok := other.hi.sub(bOne); ok && !other.hiInf && (v.hiInf || h.lt(v.hi)) {
			v.hi, v.hiInf = h, false
		}
	case "le":
		if !other.hiInf && (v.hiInf || other.hi.lt(v.hi)) {
			v.hi, v.hiInf = other.hi, false
		}
	case "gt":
		if l, ok := other.lo.add(bOne); ok && !other.loInf && (v.loInf || l.gt(v.lo)) {
			v.lo, v.loInf = l, false
		}
	case "ge":
		if !other.loInf && (v.loInf || other.lo.gt(v.lo)) {
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
	x, ok := squareOf(t)
	if !ok {
		return
	}
	if other.hiInf || other.hi.sign() < 0 {
		return
	}
	s := isqrt(other.hi)
	v := p.lookup(x)
	if v.hiInf || s.lt(v.hi) {
		v.hi, v.hiInf = s, false
	}
	if v.loInf || s.neg().gt(v.lo) {
		v.lo, v.loInf = s.neg(), false
	}
	p.env[x] = v
}

// isqrt is ⌊√n⌋. It was a linear search capped at 2^31, which was exact only
// because no endpoint exceeded 2^62; endpoints now reach 2^126, where the cap
// would be an UNSOUND upper bound, so it is computed exactly.
func isqrt(n bnd) bnd {
	if n.sign() <= 0 {
		return bZero
	}
	r, _ := fromBig(new(big.Int).Sqrt(n.big()))
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
	outerDemand, outerLoopTail := p.demandBig, p.loopTail
	p.loopTail = p.demandBig // an exit of THIS loop sits in the enclosing position
	p.demandBig = false
	outerLoopRaw := p.loopRaw
	for i, z := range inits {
		initV[i], nInits[i] = p.evalR(z)
	}
	p.demandBig = outerDemand
	cur := make([]ival, len(initV))
	copy(cur, initV)
	body, raw, _ := openFresh(lam, p.bound, asmIdent)
	p.loopRaw = raw // `again` reads this to know which arguments are bignums

	// A LOOP VARIABLE THAT HOLDS A TABLE GETS AN ELEMENT RANGE, joined from its
	// initialiser and from every `again` argument that is not a pass-through.
	//
	// Those are the only two sources of its value — the same premise the
	// reachable-set theorem rests on — so the join bounds everything a read of
	// it can produce, which is frozen-2026-08-28's buffer theorem at a loop
	// variable instead of at a `let`.
	//
	// WHY IT IS NOT CIRCULAR is a stratification, and it is the same one:
	// `elemRange` of an `again` argument analyses that argument's own `build`,
	// where the loop variable is FREE — so a read of it inside is unknown and
	// the stores that do not read it decide. Binding happens after the join is
	// computed, so nothing consults the answer while producing it. A store that
	// does read the loop variable makes `bufferElem` refuse, which is the safe
	// direction and costs only precision.
	//
	// It is what a bignum accumulator needs and had no other way to get: `acc`
	// is rebuilt every iteration by a `build` whose stores are all `(% t B)`,
	// so its elements are provably under B — and without this, reading `acc`
	// back gave ⊤ and every limb product was unbounded.
	// ARRAY SMASHING (smash.go): a table-valued variable whose initialiser has a
	// cell carries one through the fixpoint below, jointly with the scalars.
	//
	// READ BEFORE ANYTHING BELOW REBINDS A NAME. A loop threading a build's buffer
	// is `(loop ((o o) …))`, `openFresh` keeps the spelling when it is free, and
	// the initialiser `o` is the binder's zero fill — which the static setup that
	// follows deletes under the loop variable's name.
	tracked := make([]bool, len(raw))
	initE := make([]ival, len(raw))
	for k := range raw {
		if k < len(nInits) {
			if e, ok := p.elemOf(nInits[k]); ok {
				tracked[k], initE[k] = true, e
			}
		}
	}
	elemSaved := map[string][2]any{}
	static := make([]ival, len(raw))
	hasStatic := make([]bool, len(raw))
	for k := range raw {
		old, had := p.elem[raw[k]]
		elemSaved[raw[k]] = [2]any{old, had}
		if e, ok := p.loopElem(body, raw, inits, k); ok {
			static[k], hasStatic[k] = e, true
			p.elem[raw[k]] = e
		} else {
			delete(p.elem, raw[k])
		}
	}

	curE := make([]ival, len(raw))
	copy(curE, initE)
	lastE := make([]ival, len(raw))
	lastOK := make([]bool, len(raw))
	oSmRaw, oSmTr, oSmAcc, oSmOK := p.smashRaw, p.smashTracked, p.smashAcc, p.smashOK

	// THE DELTA of each table variable (smash.go). Inside a delta round a nested
	// loop's variable starts from its initialiser's delta and absorbs its back
	// edges', exactly as its cell does; at the root of a round the initial deltas
	// are ⊥ and init(G) is joined separately.
	if p.dmode {
		p.dDepth++
		defer func() { p.dDepth-- }()
	}
	initD := make([]ival, len(raw))
	if p.dmode {
		for k := range raw {
			if tracked[k] {
				initD[k], _ = p.deltaOf(nInits[k])
			}
		}
	}
	curD := make([]ival, len(raw))
	copy(curD, initD)
	lastD := make([]ival, len(raw))
	dSaved := make([][3]any, len(raw))
	if p.dmode {
		for k, n := range raw {
			old, had := p.dcell[n]
			dSaved[k] = [3]any{old, had, true}
		}
	}
	oSmD := p.smashDAcc

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
	// value or something an `again` produced, and nothing else. A smashed
	// table's cell is part of the state: `initE ⊔ ⨆ E(arg)`.
	step := func(cur []ival) []ival {
		for i, n := range raw {
			p.env[n] = cur[i]
		}
		for k, n := range raw {
			switch {
			case tracked[k] && hasStatic[k]:
				p.elem[n] = intersect(curE[k], static[k])
			case tracked[k]:
				p.elem[n] = curE[k]
			case hasStatic[k]:
				p.elem[n] = static[k]
			default:
				delete(p.elem, n)
			}
		}
		if p.dmode {
			for k, n := range raw {
				if tracked[k] {
					p.dcell[n] = curD[k]
				} else if p.dgroup[n] {
					p.dcell[n] = top // shadows a G-table it is not
				}
			}
		}
		copy(lastE, initE)
		copy(lastD, initD)
		for k := range lastOK {
			lastOK[k] = true
		}
		p.smashRaw, p.smashTracked, p.smashAcc, p.smashOK = raw, tracked, lastE, lastOK
		p.smashDAcc = lastD
		if p.dmode && p.dDepth == 0 {
			p.dIncEdge = 0
		}
		next := make([]ival, len(initV))
		copy(next, initV)
		p.collectAgain(body, raw, next)
		p.smashRaw, p.smashTracked, p.smashAcc, p.smashOK = oSmRaw, oSmTr, oSmAcc, oSmOK
		p.smashDAcc = oSmD
		return next
	}
	// dropUncelled removes from the smashed set every variable some back edge
	// hands a value with no cell: its cell was a guess, and reads through it may
	// have been too narrow. Nothing is reset, and that is sound rather than
	// thrifty — the ascent `x ← x ⊔ F(x)` reaches a post-fixpoint of the CORRECT
	// F from any starting point, because every iterate joins the initial values
	// and a post-fixpoint over-approximates every reachable state whatever lies
	// below it. A value a wrong cell produced is only an extra element of a join.
	// Whether an argument has a cell is structural, so a variable is dropped at
	// most once.
	dropUncelled := func() {
		for k := range raw {
			if tracked[k] && !lastOK[k] {
				tracked[k] = false
			}
		}
	}
	for round := 0; round < 8; round++ {
		next := step(cur)
		dropUncelled()
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
		for k := range raw {
			if !tracked[k] {
				continue
			}
			j := joinI(curE[k], lastE[k])
			if round >= 2 {
				j = widen(curE[k], j)
			}
			if !eqI(j, curE[k]) {
				stable = false
			}
			curE[k] = j
			if p.dmode {
				d := joinI(curD[k], lastD[k])
				if round >= 2 {
					d = widen(curD[k], d)
				}
				if !eqI(d, curD[k]) {
					stable = false
				}
				curD[k] = d
			}
		}
		if stable {
			break
		}
	}

	// DESCENDING, which is the half that makes the whole thing work.
	//
	// Widening throws a growing bound straight to infinity, and it does so BEFORE
	// the loop's own guard has had a chance to cap it. Re-evaluating from a
	// post-fixpoint recovers the bound: the guard now applies to an infinite
	// interval and cuts it back to something finite. Without this phase every
	// counter in every program came out unbounded, and the experiment's first
	// numbers — 10% to 20% — were measuring its absence rather than anything
	// about the programs.
	//
	// A drop here cannot happen (cells are structural and the ascent saw every
	// edge), but if one did the step it came from is discarded, not narrowed to.
	descend := func() {
		for round := 0; round < 4; round++ {
			next := step(cur)
			before := append([]bool(nil), tracked...)
			dropUncelled()
			dropped := false
			for k := range raw {
				dropped = dropped || before[k] != tracked[k]
			}
			if dropped {
				break
			}
			improved := false
			for i := range cur {
				if within(next[i], cur[i]) && !eqI(next[i], cur[i]) {
					cur[i] = next[i]
					improved = true
				}
			}
			for k := range raw {
				if tracked[k] && within(lastE[k], curE[k]) && !eqI(lastE[k], curE[k]) {
					curE[k] = lastE[k]
					improved = true
				}
				if p.dmode && tracked[k] && within(lastD[k], curD[k]) && !eqI(lastD[k], curD[k]) {
					curD[k] = lastD[k]
					improved = true
				}
			}
			if !improved {
				break
			}
		}
	}
	descend()

	// THE DELTA ROUND (smash.go): the copy-closed joint invariant J′, evaluated
	// from the post-fixpoint, met into every cell of the group, and the scalars
	// descended again under the narrower cells. Each round is sound on its own,
	// so stopping early costs only precision. It runs at the root of a round and
	// never inside one — a nested loop met while a round is running is part of
	// that round's U.
	//
	// With `trips` negative an increment blocks the round, because nothing
	// bounds how often it runs. After the size-change sweeps the trip count T is
	// known and the round runs again, extending J₀ = init ⊔ U by T·s·Δ — the
	// bounded-increment theorem. It reports whether an increment blocked it.
	deltaRounds := func(trips int64) bool {
		if p.dmode || p.noSmash {
			return false
		}
		group := map[string]bool{}
		for k, n := range raw {
			if tracked[k] {
				group[n] = true
			}
		}
		if len(group) == 0 {
			return false
		}
		// Every term these rounds rebuild is discarded — the final walk is the
		// one that is kept — so the fresh names they take are handed back
		// afterwards. Otherwise every later binder is renumbered and the emitted
		// program changes for nothing.
		boundBefore := make(map[string]bool, len(p.bound))
		for n := range p.bound {
			boundBefore[n] = true
		}
		defer func() { p.bound = boundBefore }()
		blocked := false
		for dround := 0; dround < 3; dround++ {
			oMode, oGroup, oCell, oTab := p.dmode, p.dgroup, p.dcell, p.tabDelta
			p.dmode, p.dgroup, p.dcell, p.tabDelta = true, group, map[string]ival{}, map[*core.Term]ival{}
			p.dReadOf = map[string]*core.Term{}
			p.dInc, p.dIncSeen, p.dIncBad, p.dIncEdge, p.dIncMax = bottom, false, false, 0, 0
			for k := range initD {
				initD[k], curD[k] = bottom, bottom
			}
			step(cur)
			p.dmode, p.dgroup, p.dcell, p.tabDelta = oMode, oGroup, oCell, oTab
			j := bottom
			for k := range raw {
				if tracked[k] {
					j = joinI(joinI(j, initE[k]), lastD[k])
					if !lastOK[k] {
						j = top // a back edge with no cell: no joint invariant this round
					}
				}
			}
			if p.dIncSeen {
				if trips < 0 || p.dIncBad {
					return true
				}
				// J₀ extended by T·s·Δ, with 0 joined into Δ so each end moves
				// only outward.
				ext := mulI(exact(trips), mulI(exact(int64(p.dIncMax)), joinI(p.dInc, exact(0))))
				j = addI(j, ext)
				j = joinI(j, bottom)
			}
			changed := false
			for k := range raw {
				if tracked[k] {
					if n := intersect(curE[k], j); !eqI(n, curE[k]) {
						curE[k], changed = n, true
					}
				}
			}
			if !changed {
				break
			}
			descend()
		}
		return blocked
	}
	blockedByIncrement := deltaRounds(-1)

	// THE REPRESENTATION FIXPOINT (bigrep.go), run on the settled intervals
	// because rule (P) is gated on them: a name a big operation reads is
	// promoted only if it is NOT provably inside the window, which is what keeps
	// a bignum loop from acquiring a bignum loop counter.
	//
	// It re-runs `step`, which is what propagates supply through `let`s and
	// through the operations themselves — so demand and supply are not two
	// walks, they are one walk iterated. Monotone over a two-point lattice with
	// finitely many names, so it terminates; the bound is one promotion per
	// variable plus a sweep to observe stability.
	// `bigOK()` AND NOT `HasBig()`. The two are not the same and the difference
	// is the whole limb rung: `bigOK` is `limbs || HasBig`, so adding `HasBig`
	// as a second conjunct disabled this fixpoint on precisely the target that
	// has no host bignum to fall back on. windows therefore never propagated
	// supply through a loop initialiser, and the example in the comment below —
	// which the comment already names as the program that motivated the rule —
	// was refused there with `v is array int, but int is required here`.
	//
	// That is the mistake biglimb.go warns about in as many words: every gate
	// here asks `bigOK` rather than `HasBig`, or a target with nothing to fall
	// back to promotes nothing and is then refused for producing a word.
	if p.bigOK() && (p.loopTail || len(p.big) > 0) {
		// SUPPLY REACHES A LOOP VARIABLE THROUGH ITS INITIALISER, and this line
		// is rule (S) arriving at the back edge.
		//
		// The fixpoint below is rule (P) — promote a variable a big operation
		// READS and the intervals cannot bound — and rule (D), the demand from
		// the position the loop sits in. Neither fires for a loop seeded from a
		// big value whose result is an ordinary word:
		//
		//	(loop ((v x)) (= v 0) 1 else (again (/ v 10)))
		//
		// with `x` a declared big parameter. A comparison deliberately records
		// no read (a word compared against a bignum is widened at the comparison
		// and is still a word everywhere else), a pass-through `again` reads
		// nothing, and the loop's own value is an `int` — so `v` stayed a
		// machine word and the checker refused the guard by name. The first
		// program to hit it was decimal rendering, whose whole shape is a big
		// value walked down to nothing while the answer is a string.
		for i, nm := range raw {
			if i < len(inits) && p.bigTerm(inits[i]) {
				p.promote(nm)
			}
		}
		for round := 0; round < len(raw)+3; round++ {
			p.bigChanged = false
			p.bigReads = map[string]bool{}
			step(cur)
			for k, nm := range raw {
				if p.bigReads[nm] && !cur[k].fitsIn(p.tgt.Word) {
					p.promote(nm)
				}
			}
			if !p.bigChanged {
				break
			}
		}
		// A PROMOTED VARIABLE HAS NO WINDOW BOUND, and saying so is not a loss:
		// it is the interval domain declining to claim something about a value
		// that is no longer held in a machine word. Leaving a stale finite
		// interval there would let a later operation on it be "proven".
		for k, nm := range raw {
			if p.big[nm] {
				cur[k] = top
			}
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
	// THE BOUNDED-INCREMENT ROUND (smash.go), now that T is known.
	if t, ok := trip.hi.i64(); blockedByIncrement && haveTrip && !trip.hiInf && ok && t >= 0 {
		before := append([]ival(nil), curE...)
		deltaRounds(t)
		narrowed := false
		for k := range raw {
			narrowed = narrowed || !eqI(before[k], curE[k])
		}
		// A NARROWER CELL MAKES A STEP NARROWER, and the trip bound above was
		// taken with the wider one: an accumulator that adds a value read out of
		// the table was given an unbounded step. So the steps are measured again
		// under the new cells and the bound v ∈ v₀ + T·step applied again. T is
		// unchanged — the termination argument did not read a cell — and each
		// step is evaluated under sound cells, so the bound is sound.
		if narrowed {
			p.scOn = true
			p.scSteps, p.scKnown, p.scSeen = make([]ival, n), make([]bool, n), make([]bool, n)
			for i := range p.scKnown {
				p.scKnown[i] = true
			}
			p.scEdges = nil
			step(cur)
			p.scOn = false
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
	p.loopTail, p.loopRaw = outerLoopTail, outerLoopRaw

	p.count = wasCounting
	p.restore(saved)

	for i, nm := range raw {
		p.env[nm] = cur[i]
		if p.count {
			p.rep.LoopVars++
			if cur[i].fitsIn(p.tgt.Word) {
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
	// EACH LOOP VARIABLE IS ρ OF ITS FIXPOINT INTERVAL (wordsel.go), decided
	// here where the interval is final, and every value entering it — the
	// initialiser below, each `again` in the walk — is converted to it.
	oldWordLoop := p.wordLoop
	uSaved := map[string][2]bool{}
	if p.words {
		p.wordLoop = raw
		for i, nm := range raw {
			old, had := p.u64[nm]
			uSaved[nm] = [2]bool{old, had}
			if p.wantU(cur[i]) {
				p.u64[nm] = true
			} else {
				delete(p.u64, nm)
			}
		}
	}
	out, nb := p.evalR(body)
	loopU := p.words && p.u64Term(nb)
	if p.words {
		for i := range nInits {
			if i < len(raw) {
				nInits[i] = p.toRep(nInits[i], p.u64[raw[i]])
			}
		}
		for nm, sv := range uSaved {
			if sv[1] {
				p.u64[nm] = sv[0]
			} else {
				delete(p.u64, nm)
			}
		}
		p.wordLoop = oldWordLoop
	}
	loopE, loopHasE := p.elemOf(nb)
	loopD, loopHasD := p.deltaOf(nb)
	// Whether the loop RETURNS a bignum, recorded while its variables are still
	// in scope: after `releaseBound` the names mean nothing.
	bigExit := p.chainBig(nb)
	// THE INITIAL VALUE OF A PROMOTED VARIABLE IS WIDENED HERE, and it is not a
	// detail: an accumulator starts at the literal `1`, and the emitter reads a
	// loop variable's TYPE off its initialiser. Without this the Go backend
	// declares `acc := 1` and then assigns a `*big.Int` to it.
	if p.bigOK() {
		for i := range nInits {
			if i < len(raw) && p.big[raw[i]] && !p.bigTerm(inits[i]) {
				nInits[i] = core.App(core.Name("big-of"), nInits[i])
			}
		}
	}
	// THE MUTABLE BIGNUM (bigreuse.go), AFTER the widening rather than before —
	// condition (4) asks whether a loop variable's initialiser allocates, and
	// the literal `1` does not become `(big-of 1)` until the line above. Run
	// first, the rule silently declines on every accumulator in the corpus.
	if p.tgt.HasBigDest() && !p.noDest && len(p.big) > 0 {
		nb = p.reuseInLoop(nb, raw, nInits)
	}
	// THE NAMES GO OUT OF SCOPE, AND WHAT THEY SHADOWED COMES BACK. `openFresh`
	// keeps a binder's spelling when p.bound does not hold it, and a PARAMETER
	// is not in p.bound — so `(loop ((h h)) …)` in a function of h binds h
	// again, and deleting it here deleted the parameter's premise with it. The
	// next sweep over the same term then read h as ⊤: a U parameter lost its
	// representation and `(= h 0)` was refused by type (u128-2026-09-23).
	// `elem` and `u64` already restored what they shadowed; env is restored the
	// same way, from the snapshot taken before the loop bound anything.
	for _, nm := range raw {
		old, had := saved[nm]
		restoreVar(p.env, nm, old, had)
	}
	if p.dmode {
		for k, n := range raw {
			if dSaved[k][2].(bool) {
				restoreVar(p.dcell, n, dSaved[k][0].(ival), dSaved[k][1].(bool))
			}
		}
	}
	for nm, sv := range elemSaved {
		if sv[1].(bool) {
			p.elem[nm] = sv[0].(ival)
		} else {
			delete(p.elem, nm)
		}
	}
	bigRaw := make([]bool, len(raw))
	for i, nm := range raw {
		bigRaw[i] = p.big[nm]
	}
	p.releaseBound(raw)
	// The names go out of scope with the binder, but their REPRESENTATION does
	// not travel with them: `p.big` is keyed by fresh names and releaseBound
	// hands those back for reuse, so a stale entry would make the next binder
	// with the same name big for no reason.
	for i, nm := range raw {
		if bigRaw[i] {
			delete(p.big, nm)
		}
	}
	kids := append([]*core.Term{t.Op(), core.Fn(raw, nb)}, nInits...)
	rebuiltLoop := &core.Term{Kind: core.KApp, Kids: kids}
	p.markBig(rebuiltLoop, bigExit)
	if loopU {
		p.u64Val[rebuiltLoop] = true
	}
	if loopHasE {
		p.setElemOf(rebuiltLoop, loopE)
	}
	if loopHasD {
		p.setDeltaOf(rebuiltLoop, loopD)
	}
	return out, rebuiltLoop
}

// loopElem is the element range of loop variable k, or nothing.
//
// A pass-through argument is skipped because it contributes no new value — the
// same reason the running-extremum theorem drops it — and skipping it is what
// makes the join computable at all, since a pass-through's range IS the answer
// being computed.
func (p *intervalPass) loopElem(body *core.Term, raw []string, inits []*core.Term, k int) (ival, bool) {
	if k >= len(inits) {
		return ival{}, false
	}
	out, ok := p.elemRange(inits[k])
	if !ok {
		return ival{}, false
	}
	for _, ag := range p.ownAgains(body) {
		as := ag.Args()
		if k >= len(as) {
			return ival{}, false
		}
		a := as[k]
		if a.Kind == core.KName && a.Name == raw[k] {
			continue
		}
		e, ok := p.elemRange(a)
		if !ok {
			return ival{}, false
		}
		out = joinI(out, e)
	}
	return out, true
}

// ownAgains are the back edges of THIS loop: `collectAgains` walks into a nested
// one, whose `again` has a different arity and a different meaning, and two
// loops whose arities happened to match would silently be read as one.
func (p *intervalPass) ownAgains(t *core.Term) []*core.Term {
	var out []*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KApp && x.Op().Kind == core.KName {
			if x.Op().Name == "again" {
				out = append(out, x)
				return
			}
			if pr, ok := p.tgt.Prims[x.Op().Name]; ok && pr.Kind == "iterate" {
				return // a nested loop's back edges are its own
			}
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return out
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
		body, _, _ := openFresh(lam, p.bound, func(x string) string { return x })
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
	// THE N-ARY LET IS A BINDING (ADR 0027): a host call with several results,
	// under its continuation, may wrap an `again`, and that `again` is a BACK
	// EDGE. Walked as the one-name `let` below is walked — the call's arguments
	// evaluated, the continuation's parameters bound to the declared results —
	// and not treated as an exit. Missing it left the fixpoint without the edge:
	// a counter incremented inside the continuation stayed at its initial value,
	// a product of it was "proven", and a loop that diverges for a negative bound
	// was reported to terminate.
	if pr, args, k, ok := multiPrimCall(p.tgt, t); ok {
		for _, a := range args {
			p.evalR(a)
		}
		body, _, unbind := p.bindMulti(pr, k)
		p.collectAgain(body, raw, acc)
		unbind()
		return
	}
	// RULE (D) AT AN EXIT (bigrep.go). Everything this walker does not recurse
	// into is a clause body that RETURNS, which is the loop's value — so when
	// that value is the function's declared big result, a clause returning a
	// bare loop variable is where the declaration gets its grip. This is the
	// backward half of the solver, and it is here rather than in a walk of its
	// own because this is the one place the clause chain is traversed with its
	// binders already opened.
	p.exitDemand(t)
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if prim, ok := p.tgt.Prims[t.Op().Name]; ok && prim.Kind == "cond" && len(t.Args()) == 3 {
			saved := p.snapshot()
			incAt := p.dIncEdge // each clause path counts its own increments
			p.refine(t.Args()[0], true)
			p.collectAgain(t.Args()[1], raw, acc)
			p.restore(saved)
			p.dIncEdge = incAt
			p.refine(t.Args()[0], false)
			p.collectAgain(t.Args()[2], raw, acc)
			p.restore(saved)
			return
		}
		if prim, ok := p.tgt.Prims[t.Op().Name]; ok && prim.Kind == "let" && len(t.Args()) == 2 {
			k := t.Args()[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				v, nv := p.evalR(t.Args()[0])
				kb, kraw, _ := openFresh(k, p.bound, asmIdent)
				old, had := p.env[kraw[0]]
				p.env[kraw[0]] = v
				if p.letTerm == nil {
					p.letTerm = map[string]*core.Term{}
				}
				oldT, hadT := p.letTerm[kraw[0]]
				p.letTerm[kraw[0]] = t.Args()[0]
				// A BINDER IS ITS VALUE, cell and delta included (smash.go) — the
				// same rule as `p.let`, for the `let` a clause chain binds before
				// its `again`, whose name is usually what the back edge hands on.
				oldE, hadE := p.elem[kraw[0]]
				if e, ok := p.elemOf(nv); ok {
					p.elem[kraw[0]] = e
				}
				dOld, dHad, dDid := ival{}, false, false
				if d, ok := p.deltaOf(nv); ok {
					dOld, dHad, dDid = p.shadowDelta(kraw[0], d)
				}
				rdOld, rdHad := p.dReadOf[kraw[0]]
				if p.dmode && p.derivedRead(nv) {
					p.dReadOf[kraw[0]] = nv
				}
				p.collectAgain(kb, raw, acc)
				if p.dmode {
					if rdHad {
						p.dReadOf[kraw[0]] = rdOld
					} else {
						delete(p.dReadOf, kraw[0])
					}
				}
				p.unshadowDelta(kraw[0], dOld, dHad, dDid)
				restoreVar(p.elem, kraw[0], oldE, hadE)
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
			if p.dmode && p.dDepth == 0 {
				defer func() {
					if p.dIncEdge > p.dIncMax {
						p.dIncMax = p.dIncEdge
					}
				}()
			}
			// RULE (D) AT THE BACK EDGE (bigrep.go), before the intervals,
			// because the representation decides whether the interval matters.
			p.againDemand(args, raw)
			// AND THE SAME REASON `app` HAS TO SAY IT: this walker evaluates the
			// arguments itself, so the ambient demand — the one belonging to the
			// position the whole LOOP sits in — would reach them. It is not a
			// duplicate of the rule in `app`; that one rebuilds the term and this
			// one settles the fixpoint, and both walk the back edge.
			outerDemand := p.demandBig
			for i, a := range args {
				p.demandBig = i < len(raw) && p.big[raw[i]]
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
				smashing := len(raw) > 0 && len(p.smashRaw) == len(raw) && &p.smashRaw[0] == &raw[0] &&
					i < len(p.smashTracked) && p.smashTracked[i]
				if i < len(raw) && selfContained(p.tgt, a, raw[i]) {
					acc[i] = joinI(acc[i], p.updateIval(a, raw[i]))
					if smashing {
						_, na := p.evalR(a)
						p.smashEdge(i, na)
					}
					continue
				}
				v, na := p.evalR(a)
				acc[i] = joinI(acc[i], v)
				if smashing {
					p.smashEdge(i, na)
				}
			}
			p.demandBig = outerDemand
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
		case !steps[i].hiInf && steps[i].hi.sign() <= 0:
			out[i] = +1 // never increases: v itself is the measure
		case !steps[i].loInf && steps[i].lo.sign() >= 0:
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
	return ival{lo: bi(c), hiInf: true}, true
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
	return ival{lo: bi(c), hiInf: true}, true
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
	// THE UNSIGNED WORD'S CONVERSIONS ARE THE IDENTITY on the values they see
	// (wordsel.go), so a descent through one is the descent under it: a digit
	// loop over a `uint64` is still `v ↦ v / 10`.
	arg = peelWord(arg)
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
					return down, descent{kind: 1, delta: bi(c)}
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
		if v := p.lookup(src); !v.loInf && v.lo.ge(bOne) {
			if d := p.eval(a); !d.loInf && d.lo.sign() >= 0 {
				return down, descent{kind: 1, delta: bOne}
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
			if !e.hiInf && e.hi.sign() < 0 {
				return down, descent{kind: 1, delta: e.hi.neg()}
			}
			if !e.hiInf && e.hi.sign() <= 0 {
				return downEq, descent{}
			}
		} else {
			if !e.loInf && e.lo.sign() > 0 {
				return down, descent{kind: 1, delta: e.lo}
			}
			if !e.loInf && e.lo.sign() >= 0 {
				return downEq, descent{}
			}
		}
	case isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub"):
		if srcSign > 0 {
			if !e.loInf && e.lo.sign() > 0 {
				return down, descent{kind: 1, delta: e.lo}
			}
			if !e.loInf && e.lo.sign() >= 0 {
				return downEq, descent{}
			}
		} else {
			if !e.hiInf && e.hi.sign() < 0 {
				return down, descent{kind: 1, delta: e.hi.neg()}
			}
			if !e.hiInf && e.hi.sign() <= 0 {
				return downEq, descent{}
			}
		}
	case name == "/" || strings.HasSuffix(name, ".idiv") || isOp(name, "div"):
		// v / k is strictly smaller than v when k ≥ 2 and v ≥ 1. The floor
		// matters: 0/10 is 0, which does not descend, and a loop relying on it
		// would not terminate.
		if srcSign > 0 && !e.loInf && e.lo.ge(bi(2)) {
			if v := p.lookup(src); !v.loInf && v.lo.ge(bOne) {
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
		span, ok := v.hi.sub(v.lo)
		if !ok || span.sign() < 0 {
			continue
		}
		var t bnd
		switch d := p.scKind[j]; d.kind {
		case 1:
			if d.delta.lt(bOne) {
				continue
			}
			t, _ = span.quo(d.delta).add(bOne) // span/delta < 2^126 − 1
		case 2:
			m := maxB(maxB(v.hi, v.lo.neg()), bOne)
			t = bOne
			for m.ge(d.base) {
				m = m.quo(d.base)
				t, _ = t.add(bOne)
			}
		default:
			continue
		}
		if !found || t.lt(best.hi) {
			best, found = ival{lo: bZero, hi: t}, true
		}
	}
	return best, found
}

// shiftFor decides whether `x / C` or `x % C` may become a shift or a mask, and
// returns the shift count (for a division) or the mask (for a remainder).
//
// Three conditions, and each one is load-bearing.
//
//   - **C is an exact power of two, at least 2.** An interval pins it only when
//     the divisor is a constant, which is what a carry split always is.
//   - **x is non-negative.** This is the whole point: `x >> k` is floor
//     division, and floor and truncation agree exactly on the non-negative
//     numbers. For a negative x they differ by one and the rewrite would be a
//     silent wrong answer.
//   - **x fits the target's declared shift width.** Go, the JVM and x86 shift
//     64-bit values; V8 coerces both operands of `>>` and `&` to int32, so on
//     that host the rewrite is sound only below 2^31 — and saying so as a
//     declared width is what lets it fire there at all instead of being
//     excluded.
//
// The remainder case returns C−1 as a mask and a zero shift count; the division
// case returns the shift count and a zero mask. They are told apart by which is
// zero, which is unambiguous because a shift count of 0 would be a division by
// 1 and C is at least 2.
func shiftFor(tgt *Target, op string, v []ival) (shift, mask int64, ok bool) {
	if tgt.ShiftWidth == 0 || len(v) != 2 {
		return 0, 0, false
	}
	if op != "div" && op != "rem" {
		return 0, 0, false
	}
	cv, isExact := exactNonNeg(v[1])
	if !isExact || cv < 2 || cv&(cv-1) != 0 {
		return 0, 0, false
	}
	x := v[0]
	if x.loInf || x.hiInf || x.lo.sign() < 0 {
		return 0, 0, false
	}
	// A shift is emitted on the target's word, so the dividend must be one.
	if _, ok := x.hi.i64(); !ok {
		return 0, 0, false
	}
	// `1 << 63` OVERFLOWS AN int64 and comes back negative, so a target that
	// shifts full-width would refuse every value it can hold. At 63 the test is
	// vacuous — a non-negative int64 is already under 2^63 — which is why the
	// bug was silent rather than wrong: nothing was rewritten anywhere.
	if tgt.ShiftWidth < 63 && x.hi.ge(bi(int64(1)<<uint(tgt.ShiftWidth))) {
		return 0, 0, false
	}
	if op == "rem" {
		return 0, cv - 1, true
	}
	k := int64(0)
	for b := cv; b > 1; b >>= 1 {
		k++
	}
	return k, 0, true
}

// SelectShifts rewrites `x / 2^k` into a shift and `x % 2^k` into a mask
// wherever the analysis can prove the dividend non-negative and inside the
// target's declared shift width.
//
// IT IS ITS OWN PASS, and running it inside `Intervals` instead would have been
// a mistake worth naming: that pass has `noChecked` false, so using its rebuilt
// term on the default path would turn ADR 0019's trapping arithmetic on for
// every program — reversing ADR 0012 without an ADR, which is exactly what
// assessment-2026-08-20 §2 records happening once already. A demonstration
// wired into the default path is a decision whether or not anyone made one.
//
// It runs LAST, after `PromoteBig` and after the fixed-limb lowering, because
// the library's own carry splits are spliced in by that lowering and are the
// operations this is most for.
func SelectShifts(tgt *Target, sig *core.Sig, t *core.Term) (*core.Term, int) {
	if tgt.ShiftWidth == 0 {
		return t, 0
	}
	if _, _, ok := tgt.ShiftNames(); !ok {
		return t, 0
	}
	rep, out := intervals(tgt, sig, t, 0, nil, true, true, false, true)
	if rep.Shifted == 0 {
		// NOTHING FIRED, SO NOTHING IS RETURNED. A pass that rebuilds a term it
		// did not change still hands back a different pointer, and this compiler
		// has been bitten four times by a map keyed on one — so when there is no
		// work, the original term is what the emitter gets.
		return t, 0
	}
	return out, rep.Shifted
}

// andI and shrI are the abstract semantics of a bitwise and and a right shift.
//
// BOTH ARE DEFINED ONLY ON NON-NEGATIVE OPERANDS, and answer ⊤ otherwise. That
// is not caution: on a negative value `>>` is an arithmetic shift on three hosts
// and an int32 coercion on the fourth, and `&` on a two's-complement negative
// depends on a width the language does not have. `SelectShifts` only ever emits
// these where it has PROVED the operand non-negative, so the precise case is
// the one that occurs; a hand-written `go.&` on a signed value gets the honest ⊤.
func andI(a, b ival) ival {
	// INDUCED FROM `and-left` AND `and-right` (lang-facts.oro, F9) by Theorem T
	// with case splitting (fact.go). A NON-NEGATIVE CONSTANT MASK BOUNDS THE
	// RESULT WHATEVER THE OTHER OPERAND IS, which is the case that matters: a mask
	// over a value the analysis cannot see — every read inside a `build` lambda —
	// is what `SelectShifts` produces from a carry split, and answering ⊤ there
	// cost the fixed-limb factorial its element narrowing the first time round.
	//
	// Never less precise than the hand-written version it replaced, and strictly
	// tighter where one operand is non-negative and the other spans zero: the
	// non-negative operand's clause holds on every cell of the split, where the
	// old rule answered ⊤. TestTheInducedMaskIsNeverLessPrecise holds it to that.
	return inducedTransfer(langFacts, "&", []ival{a, b}, -1)
}

func exactNonNeg(v ival) (int64, bool) {
	if v.loInf || v.hiInf || v.lo != v.hi || v.lo.sign() < 0 {
		return 0, false
	}
	return v.lo.i64()
}

func shrI(a, b ival) ival {
	kk, isExact := exactNonNeg(b)
	if a.loInf || a.lo.sign() < 0 || !isExact || kk > 62 {
		return top
	}
	k := uint(kk)
	out := ival{lo: a.lo.shr(k)}
	if a.hiInf {
		out.hiInf = true
		return out
	}
	out.hi = a.hi.shr(k)
	return out
}
