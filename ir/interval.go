package ir

import (
	"math"
	"sort"

	"oroboros/core"
	"oroboros/emit"
)

// THE INTERVAL DOMAIN ON THE IR (research §5, irp3): one factor of the reduced
// product that decides a table's element representation (Finalize).
//
// SPARSE: every value has ONE fact, in a dense vector indexed by the value,
// because a guard's narrowing belongs to the π-parameter that renames the
// guarded value (spec §6, Theorem E). Nothing is copied at a branch.
//
// The domain is a small reduced product per value: an integer interval, and for
// a table its length interval and its ELEMENT interval (array smashing: every
// store joins its value into the element fact, a weak update). Each abstract
// operation over-approximates the concrete one of spec §5 (local soundness,
// Cousot and Cousot 1977), and a loop's fact is a POST-FIXPOINT: reached by
// widening, improved by descending iterations from it, and ⊤ if the iteration
// budget ends first. Global soundness is structural induction over regions.
//
// It is used for exactly one decision, element widths, and there only in MEET
// with the term analysis's own answer: the meet of two sound over-approximations
// is sound, γ(a ⊓ b) ⊇ γ(a) ∩ γ(b).

type iv struct {
	lo, hi   ep
	nlo, phi bool // lo is −∞, hi is +∞
	bot      bool
}

var ivTop = iv{nlo: true, phi: true}
var ivBot = iv{bot: true}

func exactIV(n int64) iv      { return iv{lo: ei(n), hi: ei(n)} }
func rangeIV(lo, hi int64) iv { return iv{lo: ei(lo), hi: ei(hi)} }
func rangeE(lo, hi ep) iv     { return iv{lo: lo, hi: hi} }

// ivU is U = [0, 2⁶⁴ − 1], the unsigned word.
var ivU = rangeE(ep{}, u64Max)

func (a iv) finite() bool { return !a.bot && !a.nlo && !a.phi }

// within reports a ⊆ [lo, hi]: ⊥ is inside everything.
func (a iv) within(lo, hi ep) bool { return a.bot || (a.finite() && lo.le(a.lo) && a.hi.le(hi)) }

// norm is an interval's CANONICAL form: an infinite end carries no number.
// Equality of facts is how a loop's iteration recognises its fixpoint, so two
// spellings of one interval must be one value; a number left under an
// infinite end kept moving (widenIVT, joinIV) and the walk in tree.oro never
// converged, taking ⊤ at the budget.
func (a iv) norm() iv {
	if a.bot {
		return ivBot
	}
	if a.nlo {
		a.lo = ep{}
	}
	if a.phi {
		a.hi = ep{}
	}
	return a
}

func joinIV(a, b iv) iv {
	switch {
	case a.bot:
		return b
	case b.bot:
		return a
	}
	out := iv{nlo: a.nlo || b.nlo, phi: a.phi || b.phi}
	out.lo, out.hi = minE(a.lo, b.lo), maxE(a.hi, b.hi)
	return out.norm()
}

func meetIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	out := a
	if !b.nlo && (out.nlo || out.lo.lt(b.lo)) {
		out.lo, out.nlo = b.lo, false
	}
	if !b.phi && (out.phi || b.hi.lt(out.hi)) {
		out.hi, out.phi = b.hi, false
	}
	if !out.nlo && !out.phi && out.hi.lt(out.lo) {
		return ivBot
	}
	return out.norm()
}

func widenIV(old, nw iv) iv {
	if old.bot {
		return nw
	}
	if nw.bot {
		return old
	}
	out := old
	if nw.nlo || nw.lo.lt(old.lo) {
		out.nlo = true
	}
	if nw.phi || old.hi.lt(nw.hi) {
		out.phi = true
	}
	return out.norm()
}

// addIV adds ends; a sum outside E is the infinity on its side. A low end past
// +2¹²⁶ keeps E's greatest end as its low end, which is sound (it says less)
// and leaves the high end infinite.
func addIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	out := iv{nlo: a.nlo || b.nlo, phi: a.phi || b.phi}
	if !out.nlo {
		if lo, ok := a.lo.add(b.lo); ok {
			out.lo = lo
		} else if lo.sign() < 0 {
			out.nlo = true
		} else {
			out.lo, out.phi = eMax, true
		}
	}
	if !out.phi {
		if hi, ok := a.hi.add(b.hi); ok {
			out.hi = hi
		} else if hi.sign() > 0 {
			out.phi = true
		} else {
			out.hi, out.nlo = eMax.neg(), true
		}
	}
	return out.norm()
}

func negIV(a iv) iv {
	if a.bot {
		return ivBot
	}
	return iv{lo: a.hi.neg(), hi: a.lo.neg(), nlo: a.phi, phi: a.nlo}.norm()
}

func subIV(a, b iv) iv { return addIV(a, negIV(b)) }

// mulIV is the hull of the four corners; a corner outside E makes its side
// infinite, and an infinite factor makes the product unbounded unless the
// other is exactly 0.
func mulIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	zero := func(x iv) bool { return x.finite() && x.lo.sign() == 0 && x.hi.sign() == 0 }
	if !a.finite() || !b.finite() {
		if zero(a) || zero(b) {
			return exactIV(0)
		}
		return ivTop
	}
	out, first := iv{}, true
	for _, x := range []ep{a.lo, a.hi} {
		for _, y := range []ep{b.lo, b.hi} {
			p, ok := x.mul(y)
			if !ok {
				if (x.sign() < 0) != (y.sign() < 0) {
					out.nlo = true
				} else {
					out.phi = true
				}
				continue
			}
			if first {
				out.lo, out.hi, first = p, p, false
			} else {
				out.lo, out.hi = minE(out.lo, p), maxE(out.hi, p)
			}
		}
	}
	if first { // every corner left E: the finite ends are E's
		out.lo, out.hi = eMax.neg(), eMax
	}
	return out.norm()
}

// divIV is truncating division (integers.md §3). A strictly positive divisor
// with a finite dividend takes the corners; otherwise the dividend bound.
func divIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if !b.finite() || b.lo.sign() <= 0 || !a.finite() {
		if !b.nlo && b.lo.sign() > 0 && !a.nlo && a.lo.sign() >= 0 { // a ≥ 0, b ≥ 1: 0 ≤ a/b ≤ a
			return iv{hi: a.hi, phi: a.phi}.norm()
		}
		// |a/b| ≤ |a| for every divisor b ≠ 0, and b ≠ 0 at every division
		// that runs: it is `div`'s domain condition, an obligation discharged
		// or the program refused (refinements.md §3a).
		return dividendBound(a)
	}
	c := []ep{a.lo.quo(b.lo), a.lo.quo(b.hi), a.hi.quo(b.lo), a.hi.quo(b.hi)}
	out := rangeE(c[0], c[0])
	for _, x := range c[1:] {
		out.lo, out.hi = minE(out.lo, x), maxE(out.hi, x)
	}
	return out
}

// remIV: |a rem b| < |b|, with the dividend's sign (truncating remainder), and
// |a rem b| ≤ |a| whatever the divisor.
func remIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	between := iv{lo: minE(a.lo, ep{}), hi: maxE(a.hi, ep{}), nlo: a.nlo, phi: a.phi}.norm()
	if !b.finite() {
		return between
	}
	mu := maxE(b.lo.abs(), b.hi.abs())
	if mu.sign() == 0 {
		return between
	}
	bound, _ := mu.sub(ei(1))
	switch {
	case !a.nlo && a.lo.sign() >= 0:
		hi := bound
		if !a.phi && a.hi.lt(hi) {
			hi = a.hi
		}
		return rangeE(ep{}, hi)
	case !a.phi && a.hi.sign() <= 0:
		lo := bound.neg()
		if !a.nlo && lo.lt(a.lo) {
			lo = a.lo
		}
		return rangeE(lo, ep{})
	}
	return meetIV(rangeE(bound.neg(), bound), between)
}

// dividendBound is what |q| ≤ |a| says of a quotient: q ∈ [−M, M] for M the
// dividend's largest magnitude, which is an end since E is symmetric.
func dividendBound(a iv) iv {
	if a.nlo || a.phi {
		return ivTop
	}
	m := maxE(a.lo.abs(), a.hi.abs())
	return rangeE(m.neg(), m)
}

// narrowIV is a π's fact: x where `x rel o` holds (Theorem E).
func narrowIV(x iv, rel string, o iv) iv {
	if x.bot || o.bot {
		return ivBot
	}
	one := ei(1)
	switch rel {
	case "lt":
		if !o.phi {
			if h, ok := o.hi.sub(one); ok {
				return meetIV(x, iv{hi: h, nlo: true})
			}
		}
	case "le":
		if !o.phi {
			return meetIV(x, iv{hi: o.hi, nlo: true})
		}
	case "gt":
		if !o.nlo {
			if l, ok := o.lo.add(one); ok {
				return meetIV(x, iv{lo: l, phi: true})
			}
		}
	case "ge":
		if !o.nlo {
			return meetIV(x, iv{lo: o.lo, phi: true})
		}
	case "eq":
		return meetIV(x, o)
	case "ne":
		// x ≠ o for o a single value removes it: from an end of x, or all of
		// x when x is that one value ({k} ∖ {k} = ∅).
		if o.finite() && o.lo == o.hi && x.finite() && x.lo == x.hi && x.lo == o.lo {
			return ivBot
		}
		if o.finite() && o.lo == o.hi && x.finite() && x.lo.lt(x.hi) {
			if x.lo == o.lo {
				l, _ := x.lo.add(one)
				return rangeE(l, x.hi)
			}
			if x.hi == o.lo {
				h, _ := x.hi.sub(one)
				return rangeE(x.lo, h)
			}
		}
	}
	return x
}

// fact is one value's abstraction: its integer interval, and for a table its
// length and its element interval.
type fact struct {
	v            iv
	ln, el       iv
	hasLn, hasEl bool
}

var topFact = fact{v: ivTop}

func joinF(a, b fact) fact {
	out := fact{v: joinIV(a.v, b.v)}
	if a.hasLn && b.hasLn {
		out.ln, out.hasLn = joinIV(a.ln, b.ln), true
	}
	if a.hasEl && b.hasEl {
		out.el, out.hasEl = joinIV(a.el, b.el), true
	}
	return out
}

func widenF(old, nw fact) fact {
	out := fact{v: widenIV(old.v, nw.v)}
	if old.hasLn && nw.hasLn {
		out.ln, out.hasLn = widenIV(old.ln, nw.ln), true
	}
	if old.hasEl && nw.hasEl {
		out.el, out.hasEl = widenIV(old.el, nw.el), true
	}
	return out
}

func meetF(a, b fact) fact {
	out := fact{v: meetIV(a.v, b.v)}
	if a.hasLn && b.hasLn {
		out.ln, out.hasLn = meetIV(a.ln, b.ln), true
	}
	if a.hasEl && b.hasEl {
		out.el, out.hasEl = meetIV(a.el, b.el), true
	}
	return out
}

// intervals analyses one function and returns every value's fact.
type intervals struct {
	tg     *emit.Target
	f      *Func
	maxLen int64
	fs     []fact
	loops  []*exitFacts
	def    map[V]*Stmt
	piOf   map[V]V // a π-parameter's source: the same value, renamed on an arm
	thresh []ep    // the widening's thresholds (widenIVT)
	// halts is each loop's termination verdict (sct.go), from its last
	// evaluation, which is the one under the final facts.
	halts map[*Stmt]bool
}

type exitFacts struct {
	cont, brk       []fact
	hasCont, hasBrk bool
	// The loop's parameters, and their facts AT each continue before the step,
	// joined: under that arm's guards, which is what bounds a ranking
	// parameter (trip.go, Theorem 2).
	params []V
	pre    []fact
	hasPre bool
	// at is each continue's own record, from the last evaluation: its
	// arguments' facts and the parameters' facts there, under that arm's
	// guards. A continue with no record was not evaluated: its arm is dead.
	at map[*Region][2][]fact
}

// loopBudget bounds a loop's ascending iteration. Widening after round 3 makes
// convergence certain in finitely many rounds for this domain; the budget is a
// guard, and a loop that exhausts it takes ⊤, which is sound.
//
// The widening with thresholds (widenIVT) has finite chains, but not SHORT
// ones: an end can stop at every threshold on its way, and a function that
// inlines a tokeniser has hundreds of constants, so a counter climbed one per
// round and exhausted the budget (tree.oro's walk). So thresholds are used for
// thresholdRounds rounds and the plain widening after them, whose chains are
// at most two steps per end (Blanchet et al. 2003 delay and then widen by the
// same schedule). What the jump loses, the descending iterations and the trip
// bounds recover.
const (
	loopBudget      = 64
	thresholdRounds = 16
)

func analyse(tg *emit.Target, f *Func) []fact {
	a := newIntervals(tg, f)
	a.region(f.Body)
	return a.fs
}

// newIntervals prepares the analysis of f without running it.
func newIntervals(tg *emit.Target, f *Func) *intervals {
	a := &intervals{tg: tg, f: f, maxLen: tg.Word.Hi, fs: make([]fact, f.NV()), def: map[V]*Stmt{}, piOf: map[V]V{}, halts: map[*Stmt]bool{}}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			a.piOf[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				a.def[v] = &r.Stmts[i]
			}
		}
	})
	if tg.MaxLen > 0 && tg.MaxLen < a.maxLen {
		a.maxLen = tg.MaxLen
	}
	a.thresh = thresholds(tg, f)
	for _, x := range f.Params {
		a.fs[x] = a.declared(f.Types[x])
	}
	return a
}

// declared is what a declared type says of a value: an integer range, or a
// table's length and element range.
func (a *intervals) declared(ty string) fact {
	out := topFact
	out.v = a.rangeOf(ty)
	if e := ElemOf(a.tg, unbuffer(ty)); e != "" {
		out.hasLn, out.ln = true, rangeIV(0, a.maxLen)
		out.hasEl, out.el = true, a.rangeOf(e)
	}
	return out
}

// rangeOf is the interval a type denotes: its declared range read exactly
// (U's top, 2⁶⁴ − 1, included), the realization's own range for a u64, and ⊤
// otherwise.
func (a *intervals) rangeOf(ty string) iv {
	if lo, hi, ok := core.IntRangeBig(ty); ok {
		l, okl := fromBig(lo)
		h, okh := fromBig(hi)
		out := iv{lo: l, hi: h, nlo: !okl, phi: !okh}
		return out.norm()
	}
	if a.tg.ValueType(ty) == core.U64Type {
		return ivU
	}
	return ivTop
}

func (a *intervals) facts(vs []V) []fact {
	out := make([]fact, len(vs))
	for i, v := range vs {
		out[i] = a.fs[v]
	}
	return out
}

func joinAll(acc []fact, has bool, fs []fact) []fact {
	if !has {
		return append([]fact(nil), fs...)
	}
	for i := range acc {
		if i < len(fs) {
			acc[i] = joinF(acc[i], fs[i])
		}
	}
	return acc
}

func (a *intervals) set(res []V, fs []fact) {
	for i, v := range res {
		if i < len(fs) {
			a.fs[v] = fs[i]
		} else {
			a.fs[v] = topFact
		}
	}
}

// region evaluates r and returns what it yields.
func (a *intervals) region(r *Region) ([]fact, bool) {
	var saved []savedFact
	for _, pi := range r.Pis {
		x := a.fs[pi.Of]
		if pi.Len {
			if x.hasLn {
				x.ln = narrowIV(x.ln, pi.Rel, a.fs[pi.Other].v)
			}
		} else {
			x.v = narrowIV(x.v, pi.Rel, a.fs[pi.Other].v)
			// What the guard says of the compared value, it says of what that
			// value was computed from: backward, on this arm only. And `Of Rel
			// Other` is `Other flip(Rel) Of`: lowering makes a π only for a value
			// the arm reads, so the other side (the sieve's i·i against n) is
			// narrowed here, and propagated backward from.
			saved = a.back(pi.Of, x.v, saved, 3)
			if pi.Other >= 0 {
				o := narrowIV(a.fs[pi.Other].v, flip(pi.Rel), a.fs[pi.Of].v)
				saved = a.back(pi.Other, o, saved, 3)
			}
		}
		a.fs[pi.V] = x
		// AN ARM WHOSE π IS EMPTY IS NEVER TAKEN: the π is its source
		// restricted to the guard (Theorem E), and no value satisfies it.
		if (!pi.Len && x.v.bot) || (pi.Len && x.hasLn && x.ln.bot) {
			a.restore(saved)
			a.dead(r)
			return nil, false
		}
	}
	defer a.restore(saved)
	for i := range r.Stmts {
		a.stmt(&r.Stmts[i])
	}
	switch r.T {
	case TYield:
		return a.facts(r.Args), true
	case TBreak, TContinue:
		lc := a.loops[len(a.loops)-1]
		fs := a.facts(r.Args)
		if r.T == TBreak {
			lc.brk, lc.hasBrk = joinAll(lc.brk, lc.hasBrk, fs), true
		} else {
			lc.cont, lc.hasCont = joinAll(lc.cont, lc.hasCont, fs), true
			if lc.params != nil {
				pre := a.facts(lc.params)
				lc.pre, lc.hasPre = joinAll(lc.pre, lc.hasPre, pre), true
				if lc.at != nil {
					lc.at[r] = [2][]fact{fs, pre}
				}
			}
		}
	case TBranch:
		y1, ok1 := a.arm(r.Cond, true, r.Then)
		y2, ok2 := a.arm(r.Cond, false, r.Else)
		switch {
		case ok1 && ok2:
			return joinAll(y1, true, y2), true
		case ok1:
			return y1, true
		case ok2:
			return y2, true
		}
	}
	return nil, false
}

func (a *intervals) stmt(s *Stmt) {
	args := a.facts(s.Args)
	one := func(f fact) { a.fs[s.Res[0]] = f }
	switch s.Op {
	case OConst:
		if s.Lit.Kind == core.KInt {
			one(fact{v: exactIV(s.Lit.Int)})
		} else {
			one(topFact)
		}
	case OGlobal:
		one(a.declared(a.f.Types[s.Res[0]]))
	case OAdd:
		one(fact{v: addIV(args[0].v, args[1].v)})
	case OSub:
		one(fact{v: subIV(args[0].v, args[1].v)})
	case OMul:
		one(fact{v: mulIV(args[0].v, args[1].v)})
	case ONeg:
		one(fact{v: negIV(args[0].v)})
	case ODiv:
		one(fact{v: divIV(args[0].v, args[1].v)})
	case ORem:
		one(fact{v: remIV(args[0].v, args[1].v)})
	case OEq, ONe, OLt, OLe, OGt, OGe:
		one(fact{v: rangeIV(0, 1)})
	case OCall:
		p := a.tg.Prims[s.Name]
		if p.Kind == "stmt" && len(s.Args) > 0 {
			// A statement's value is argument 0, and the host may have written
			// into it: what it holds is what its declaration allows.
			out := args[0]
			if out.hasEl {
				out.el = ivTop
				if len(p.Args) > 0 {
					if lo, hi, ok := core.IntRange(ElemOf(a.tg, unbuffer(p.Args[0]))); ok {
						out.el = rangeIV(lo, hi)
					}
				}
			}
			one(out)
			return
		}
		// THE UNSIGNED WORD'S OPERATIONS ARE ℤ'S on the integers they hold:
		// wordsel selected each one only where the ring homomorphism ℤ → ℤ/2⁶⁴
		// agrees with ℤ (ADR 0026), so its transfer is ℤ's, on non-negative
		// operands. The conversions are the identity. A big value's remainder
		// by a word is bounded by the word (`big%-small`, |r| < |b|).
		if len(s.Res) == 1 {
			// THE MASK AND THE SHIFT, which SelectShifts writes for a division
			// and a remainder by 2ᵏ (shiftdiv-2026-09-03): their laws, or every
			// value computed from a split carry is ⊤ (irstep4c).
			if len(args) == 2 {
				switch emit.ArithOp(s.Name, 2) {
				case "and":
					one(fact{v: andIV(args[0].v, args[1].v)})
					return
				case "shr":
					one(fact{v: shrIV(args[0].v, args[1].v)})
					return
				}
			}
			switch s.Name {
			case "u64-of", "int-of-u64":
				if len(args) == 1 {
					if s.Name == "u64-of" {
						one(fact{v: u64Of(args[0].v)})
					} else {
						one(fact{v: intOfU64(args[0].v)})
					}
					return
				}
			case "u64+", "u64-", "u64*", "u64/", "u64%":
				if len(args) == 2 {
					one(fact{v: u64Arith(s.Name, args[0].v, args[1].v)})
					return
				}
			case "big%-small":
				if len(args) == 2 {
					one(fact{v: remIV(ivTop, args[1].v)})
					return
				}
			}
		}
		for j, v := range s.Res {
			ty := p.Result
			if len(p.Results) >= 2 && j < len(p.Results) {
				ty = p.Results[j]
			}
			a.fs[v] = a.declared(ty)
		}
	case OIndex:
		t := args[0]
		if t.hasEl {
			one(fact{v: t.el})
		} else {
			one(topFact)
		}
	case OLen:
		out := fact{v: rangeIV(0, a.maxLen)}
		if args[0].hasLn {
			out.v = meetIV(args[0].ln, out.v)
		}
		one(out)
	case OArray:
		out := fact{v: ivTop, hasLn: true, ln: exactIV(int64(len(args))), hasEl: true, el: ivBot}
		for _, x := range args {
			out.el = joinIV(out.el, x.v)
		}
		one(out)
	case OMap, OKeys:
		one(a.declared(a.f.Types[s.Res[0]]))
	case ORead:
		a.fs[s.Res[0]] = fact{v: rangeIV(0, 1)}
		a.fs[s.Res[1]] = topFact
	case OSet:
		out := args[0]
		if out.hasEl {
			out.el = joinIV(out.el, args[2].v) // a weak update: smashing
		}
		one(out)
	case OInsert:
		one(args[0])
	case OThe:
		one(meetF(args[0], a.declared(s.Type)))
	case ORequire:
	case OAssume:
		a.assume(s.Args[0])
	case ORestrict:
		out := args[0]
		if out.hasLn {
			out.ln = meetIV(out.ln, args[1].v)
		}
		one(out)
	case OIf:
		y1, ok1 := a.arm(s.Args[0], true, s.Sub[0])
		y2, ok2 := a.arm(s.Args[0], false, s.Sub[1])
		switch {
		case ok1 && ok2:
			a.set(s.Res, joinAll(y1, true, y2))
		case ok1:
			a.set(s.Res, y1)
		case ok2:
			a.set(s.Res, y2)
		default:
			a.set(s.Res, nil)
		}
	case OBuild:
		body := s.Sub[0]
		ln := meetIV(args[0].v, rangeIV(0, a.maxLen))
		a.fs[body.Params[0]] = fact{v: ivTop, hasLn: true, ln: ln, hasEl: true, el: exactIV(0)} // zero-filled (tables.md §14.3)
		y, _ := a.region(body)
		a.set(s.Res, y)
	case OBuildMap:
		a.fs[s.Sub[0].Params[0]] = topFact
		y, _ := a.region(s.Sub[0])
		a.set(s.Res, y)
	case OTabulate:
		body := s.Sub[0]
		n := meetIV(args[0].v, rangeIV(0, a.maxLen))
		i := ivBot
		if !n.bot {
			i = ivBot
			if n.phi || n.hi.sign() > 0 {
				h, _ := n.hi.sub(ei(1))
				i = iv{hi: h, phi: n.phi}.norm() // [0, n − 1]
			}
		}
		a.fs[body.Params[0]] = fact{v: i}
		y, ok := a.region(body)
		out := fact{v: ivTop, hasLn: true, ln: n, hasEl: true, el: ivTop}
		if ok && len(y) == 1 {
			out.el = y[0].v
		}
		one(out)
	case OLoop:
		a.loop(s)
	}
	// A PROMOTED HOST OPERATION IS ℤ'S ONLY INSIDE THE WORD (lowering's
	// promote): JavaScript's `+` is exact within ±(2⁵³ − 1) and rounds past it,
	// where `x + 1` can be x. Where ℤ's result lies in the word the two agree
	// and the transfer is exact; elsewhere the value is the host's, of which
	// nothing is known.
	if s.Name != "" && s.Op.IsArith() && len(s.Res) == 1 && !a.inWord(a.fs[s.Res[0]].v) {
		a.fs[s.Res[0]] = topFact
	}
}

// inWord reports x ⊆ the target's signed word (⊥ included).
func (a *intervals) inWord(x iv) bool { return x.within(ei(a.tg.Word.Lo), ei(a.tg.Word.Hi)) }

// loop is the Elgot iterate's abstraction: ascend with widening after three
// rounds to a post-fixpoint, descend twice from it, then evaluate once more at
// the result, which is what the facts of the body's values record.
func (a *intervals) loop(s *Stmt) {
	body := s.Sub[0]
	init := a.facts(s.Args)
	cur := append([]fact(nil), init...)
	lc := &exitFacts{}
	a.loops = append(a.loops, lc)
	round := func() []fact {
		for i, q := range body.Params {
			a.fs[q] = cur[i]
		}
		*lc = exitFacts{params: body.Params, at: map[*Region][2][]fact{}}
		a.region(body)
		if !lc.hasCont {
			return append([]fact(nil), init...)
		}
		return joinAll(append([]fact(nil), init...), true, lc.cont)
	}
	converged := false
	for it := 0; it < loopBudget; it++ {
		next := round()
		same := true
		for i := range next {
			if it >= 3+thresholdRounds {
				next[i] = widenF(cur[i], joinF(cur[i], next[i]))
			} else if it >= 3 {
				next[i] = widenFT(cur[i], joinF(cur[i], next[i]), a.thresh)
			} else {
				next[i] = joinF(cur[i], next[i])
			}
			if next[i] != cur[i] {
				same = false
			}
		}
		cur = next
		if same {
			converged = true
			break
		}
	}
	if !converged {
		// NOT A POST-FIXPOINT: nothing is known. ⊤ is sound; a truncated
		// ascending chain is not.
		for i := range cur {
			cur[i] = topOf(cur[i])
		}
	} else {
		for k := 0; k < 2; k++ { // descending: cur ⊓ F(cur) ⊒ lfp from a post-fixpoint
			next := round()
			for i := range next {
				cur[i] = meetF(cur[i], next[i])
			}
		}
	}
	for i, q := range body.Params {
		a.fs[q] = cur[i]
	}
	*lc = exitFacts{params: body.Params, at: map[*Region][2][]fact{}}
	a.region(body)
	// Trip-bounded parameters (trip.go): met into the post-fixpoint, and the
	// body evaluated once more under the tighter facts.
	// A narrowed cell narrows a step read from it (Theorem 3 feeds Theorem 1),
	// so it is repeated while it tightens: each round meets sound facts into
	// sound facts, and the body re-evaluated under them is sound.
	for k := 0; converged && k < 4 && a.tripRefine(s, init, cur, lc); k++ {
		for i, q := range body.Params {
			a.fs[q] = cur[i]
		}
		*lc = exitFacts{params: body.Params, at: map[*Region][2][]fact{}}
		a.region(body)
	}
	a.halts[s] = a.terminates(s, cur, lc)
	a.loops = a.loops[:len(a.loops)-1]
	a.set(s.Res, lc.brk)
}

// assume is the abstract semantics of `assume c` (Theorem E's reasoning):
// every execution past it satisfies c, so each conjunct comparing a value, or
// a table's length, with another value narrows that value's fact. An `assume`
// sits at the function's entry and values are immutable, so the narrowed fact
// holds wherever the value is used.
func (a *intervals) assume(c V) { a.assumeAs(c, true) }

// assumeAs is `assume` of c having the value holds. A conjunction's shape is
// whatever case-of-case left: (if x y false) is x ∧ y, (if x false y) is
// ¬x ∧ y, and y may be a region that YIELDS or one that BRANCHES again, so a
// nested `where` reaches every conjunct (assumeRegion). A comparison assumed
// false is its negation.
func (a *intervals) assumeAs(c V, holds bool) {
	d := a.def[c]
	if d == nil {
		return
	}
	switch {
	case d.Op == OIf && holds && len(d.Sub) == 2:
		switch {
		case a.yieldsConst(d.Sub[1], false):
			a.assumeAs(d.Args[0], true)
			a.assumeRegion(d.Sub[0])
		case a.yieldsConst(d.Sub[0], false):
			a.assumeAs(d.Args[0], false)
			a.assumeRegion(d.Sub[1])
		}
	case d.Op.IsCmp():
		rel := relOf(d.Op)
		if !holds {
			rel = negate(rel)
		}
		a.narrowAssumed(d.Args[0], rel, a.fs[d.Args[1]].v)
		a.narrowAssumed(d.Args[1], flip(rel), a.fs[d.Args[0]].v)
	}
}

// assumeRegion assumes that region r's one boolean result is true: every
// execution past the assumption took the path through r that yields true.
func (a *intervals) assumeRegion(r *Region) {
	switch r.T {
	case TYield:
		if len(r.Args) == 1 {
			a.assumeAs(r.Args[0], true)
		}
	case TBranch:
		switch {
		case a.yieldsConst(r.Else, false):
			a.assumeAs(r.Cond, true)
			a.assumeRegion(r.Then)
		case a.yieldsConst(r.Then, false):
			a.assumeAs(r.Cond, false)
			a.assumeRegion(r.Else)
		}
	}
}

// yieldsConst reports whether r only yields the boolean literal b.
func (a *intervals) yieldsConst(r *Region, b bool) bool {
	return r != nil && r.T == TYield && len(r.Args) == 1 && isConst(a.def, r.Args[0], b)
}

// narrowAssumed narrows x, or the table whose length x is, by `x rel o`.
func (a *intervals) narrowAssumed(x V, rel string, o iv) {
	if d := a.def[x]; d != nil && d.Op == OLen {
		t := d.Args[0]
		if a.fs[t].hasLn {
			a.fs[t].ln = narrowIV(a.fs[t].ln, rel, o)
		}
		a.fs[x].v = narrowIV(a.fs[x].v, rel, o)
		return
	}
	a.fs[x].v = narrowIV(a.fs[x].v, rel, o)
	// A π-PARAMETER IS ITS SOURCE, renamed on an arm, so what an assumption
	// says of it, it says of the source: `assume c` holds at its point on every
	// execution, and the two names denote one value. A conjunction's second
	// conjunct reads its operands through the first's π, so without this the
	// fact never reached the parameter: `(where (and (<= 0 n) (< n 1048576)))`
	// left n at [0, +∞] and every operation on it unproven.
	if src, ok := a.piOf[x]; ok {
		a.narrowAssumed(src, rel, o)
	}
}

func topOf(f fact) fact {
	out := topFact
	if f.hasLn {
		out.hasLn, out.ln = true, ivTop
	}
	if f.hasEl {
		out.hasEl, out.el = true, ivTop
	}
	return out
}

// show spells an interval: [lo, hi], with -∞ and +∞ for an open end, and ⊥.
func show(a iv) string {
	if a.bot {
		return "⊥"
	}
	lo, hi := a.lo.String(), a.hi.String()
	if a.nlo {
		lo = "-∞"
	}
	if a.phi {
		hi = "+∞"
	}
	return "[" + lo + ", " + hi + "]"
}

// specOnly is the set of values read only by assumptions, directly or through
// other such values: the greatest fixpoint of "every reader is an `assume`, or
// a statement whose results are all in the set". A value a terminator, a
// branch, a π or any other statement reads is not in it.
func specOnly(f *Func) map[V]bool {
	type reader struct {
		s     *Stmt // a statement; nil for a terminator, a branch condition or a π
		owner *Stmt // for an `if` arm's yield: the `if`, whose results it becomes
	}
	readers := map[V][]reader{}
	arms := map[*Region]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OIf {
				for _, sub := range s.Sub {
					arms[sub] = s
				}
			}
		}
	})
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, a := range s.Args {
				readers[a] = append(readers[a], reader{s: s})
			}
		}
		for _, a := range r.Args {
			if r.T == TYield && arms[r] != nil {
				readers[a] = append(readers[a], reader{owner: arms[r]})
			} else {
				readers[a] = append(readers[a], reader{})
			}
		}
		if r.T == TBranch {
			readers[r.Cond] = append(readers[r.Cond], reader{})
		}
		for _, pi := range r.Pis {
			readers[pi.Of] = append(readers[pi.Of], reader{})
			readers[pi.Other] = append(readers[pi.Other], reader{})
		}
	})
	in := map[V]bool{}
	for v := 0; v < f.NV(); v++ {
		in[V(v)] = len(readers[V(v)]) > 0
	}
	for changed := true; changed; {
		changed = false
		for v, ok := range in {
			if !ok {
				continue
			}
			for _, rd := range readers[v] {
				keep := (rd.owner != nil && allIn(in, rd.owner.Res)) ||
					(rd.s != nil && (rd.s.Op == OAssume ||
						((len(rd.s.Sub) == 0 || rd.s.Op == OIf) && allIn(in, rd.s.Res))))
				if !keep {
					in[v] = false
					changed = true
					break
				}
			}
		}
	}
	return in
}

func allIn(in map[V]bool, vs []V) bool {
	if len(vs) == 0 {
		return false
	}
	for _, v := range vs {
		if !in[v] {
			return false
		}
	}
	return true
}

// WIDENING WITH THRESHOLDS (Blanchet et al., PLDI 2003; Halbwachs's "widening
// up to", 1993). A bound that grows jumps to the nearest THRESHOLD beyond it,
// and past the last one to ∞, instead of straight to ∞. The thresholds are the
// integer constants the function mentions and their neighbours k−1 and k+1:
// `(< sp 32)` makes sp ≤ 31 and sp + 1 ≤ 32 the points an invariant stops at.
//
// It is a widening. (1) The result contains both arguments: each end moves
// only outward, to a threshold at least as far as the new end, or to ∞.
// (2) Every ascending chain is finite: an end takes a threshold value or ∞,
// and moves monotonically, so it changes at most |T| + 1 times.
//
// It is what keeps a maximum an interval: `mx := if (> (+ sp 1) mx) (+ sp 1)
// mx` under `sp < 32` is in [0, 32] on every iteration, and the plain widening
// sent it to +∞ on the third round, where the descending iterations cannot
// recover it because one arm yields mx itself.
func widenIVT(old, nw iv, t []ep) iv {
	if old.bot {
		return nw
	}
	if nw.bot {
		return old
	}
	out := old
	if nw.nlo {
		out.nlo = true
	} else if nw.lo.lt(old.lo) {
		// the greatest threshold at or below the new low end
		i := sort.Search(len(t), func(k int) bool { return nw.lo.lt(t[k]) })
		if i == 0 {
			out.nlo = true
		} else {
			out.lo = t[i-1]
		}
	}
	if nw.phi {
		out.phi = true
	} else if old.hi.lt(nw.hi) {
		// the least threshold at or above the new high end
		i := sort.Search(len(t), func(k int) bool { return nw.hi.le(t[k]) })
		if i == len(t) {
			out.phi = true
		} else {
			out.hi = t[i]
		}
	}
	return out.norm()
}

func widenFT(old, nw fact, t []ep) fact {
	out := fact{v: widenIVT(old.v, nw.v, t)}
	if old.hasLn && nw.hasLn {
		out.ln, out.hasLn = widenIVT(old.ln, nw.ln, t), true
	}
	if old.hasEl && nw.hasEl {
		out.el, out.hasEl = widenIVT(old.el, nw.el, t), true
	}
	return out
}

// thresholds is the sorted set of a function's integer constants and their
// neighbours, 0, the ends of the target's word, and U's top where the target
// realizes U.
func thresholds(tg *emit.Target, f *Func) []ep {
	set := map[ep]bool{ei(tg.Word.Lo): true, ei(tg.Word.Hi): true, {}: true}
	if tg.Word.Unsigned {
		set[u64Max] = true
	}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OConst && s.Lit.Kind == core.KInt {
				k := ei(s.Lit.Int)
				set[k] = true
				if d, ok := k.sub(ei(1)); ok {
					set[d] = true
				}
				if u, ok := k.add(ei(1)); ok {
					set[u] = true
				}
			}
		}
	})
	out := make([]ep, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].lt(out[j]) })
	return out
}

// BACKWARD PROPAGATION on an arm (HC4-revise: Benhamou, Goualard, Granvilliers
// and Puget 1999). A guard narrows the value it compares; the values that one
// was computed from are narrowed too, by the inverse of each operation, and
// only for the arm, whose every execution satisfies the guard. The facts are
// overwritten and restored when the arm is left (restore), so a value defined
// outside keeps its own fact there. Each rule is an interval inverse:
//
//	z = x + y   ⇒  x ∈ Z − Y,  y ∈ Z − X
//	z = x − y   ⇒  x ∈ Z + Y,  y ∈ X − Z
//	z = −x      ⇒  x ∈ −Z
//	z = x · c   ⇒  x ∈ [⌈Zlo/c⌉, ⌊Zhi/c⌋]   (c ≠ 0 constant; ends swapped for c < 0)
//	z = x · x   ⇒  |x| ≤ ⌊√Zhi⌋
//
// and a π-parameter is its source, so what is learnt of it is learnt of that.
// It is what bounds the sieve's `i` from `(< (* i i) n)`: without it the
// guard narrowed the product and nothing it was computed from.
type savedFact struct {
	v V
	f fact
}

func (a *intervals) restore(saved []savedFact) {
	for i := len(saved) - 1; i >= 0; i-- {
		a.fs[saved[i].v] = saved[i].f
	}
}

// back meets v's fact with z on this arm, and propagates to what v was computed
// from, to the given depth.
func (a *intervals) back(v V, z iv, saved []savedFact, depth int) []savedFact {
	if depth < 0 || v < 0 || int(v) >= len(a.fs) {
		return saved
	}
	cur := a.fs[v].v
	m := meetIV(cur, z)
	if m == cur {
		return saved
	}
	saved = append(saved, savedFact{v, a.fs[v]})
	a.fs[v].v = m
	if src, ok := a.piOf[v]; ok {
		saved = a.back(src, m, saved, depth)
	}
	d := a.def[v]
	if d == nil || len(d.Res) != 1 {
		return saved
	}
	arg := func(i int) iv { return a.fs[d.Args[i]].v }
	switch d.Op {
	case OAdd:
		saved = a.back(d.Args[0], subIV(m, arg(1)), saved, depth-1)
		saved = a.back(d.Args[1], subIV(m, arg(0)), saved, depth-1)
	case OSub:
		saved = a.back(d.Args[0], addIV(m, arg(1)), saved, depth-1)
		saved = a.back(d.Args[1], subIV(arg(0), m), saved, depth-1)
	case ONeg:
		saved = a.back(d.Args[0], negIV(m), saved, depth-1)
	case OMul:
		x, y := d.Args[0], d.Args[1]
		if a.pl(x) == a.pl(y) {
			if r, ok := invSquare(m); ok {
				saved = a.back(x, r, saved, depth-1)
			}
			break
		}
		for _, pair := range [][2]V{{x, y}, {y, x}} {
			if c, ok := a.constOf(pair[1]); ok {
				if r, ok := invMulConst(m, c); ok {
					saved = a.back(pair[0], r, saved, depth-1)
				}
			}
		}
	}
	return saved
}

// arm evaluates one arm of a branch or an `if` under its guard's value. When
// the guard, applied, empties a value's fact (cond's narrowing reached ⊥), no
// execution takes the arm: it is dead, as it is when a π of it is empty.
func (a *intervals) arm(c V, holds bool, r *Region) ([]fact, bool) {
	sv := a.cond(c, holds, nil, 3)
	defer a.restore(sv)
	for _, x := range sv {
		if a.fs[x.v].v.bot {
			a.dead(r)
			return nil, false
		}
	}
	return a.region(r)
}

// dead gives every value an unreachable region defines the fact ⊥, which is
// exact: no execution defines one. Its exits contribute nothing, since the
// region is not evaluated; and an operation in it is proven (⊥ is inside
// every set), as the term analysis counts one.
func (a *intervals) dead(r *Region) {
	// ⊥ in every component, a table's length and elements included: a dead
	// table holds nothing, so it adds nothing to its class's hull (a missing
	// component would read as ⊤, and did: read-loop's bytes, irstep4c).
	bot := fact{v: ivBot, ln: ivBot, el: ivBot, hasLn: true, hasEl: true}
	var walk func(r *Region)
	walk = func(r *Region) {
		for _, p := range r.Params {
			a.fs[p] = bot
		}
		for _, pi := range r.Pis {
			a.fs[pi.V] = bot
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				a.fs[v] = bot
			}
			if r.Stmts[i].Op == OLoop {
				a.halts[&r.Stmts[i]] = true // never entered: it halts vacuously
			}
			for _, sub := range r.Stmts[i].Sub {
				walk(sub)
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
}

// pl is a value's π-source, so x·x through two π-renamings is still a square.
func (a *intervals) pl(v V) V {
	for {
		src, ok := a.piOf[v]
		if !ok {
			return v
		}
		v = src
	}
}

func (a *intervals) constOf(v V) (int64, bool) {
	d := a.def[v]
	switch {
	case d == nil:
	case d.Op == OConst && d.Lit.Kind == core.KInt:
		return d.Lit.Int, true
	case d.Op == OCall && (d.Name == "u64-of" || d.Name == "int-of-u64") && len(d.Args) == 1:
		// The residue map is the identity on S ∩ U = [0, 2⁶³): a literal
		// there converts to itself, as `(u64-of 10)` does.
		if k, ok := a.constOf(d.Args[0]); ok && k >= 0 {
			return k, true
		}
	}
	return 0, false
}

// invSquare is what x·x ∈ z says of x: |x| ≤ ⌊√z.hi⌋, and nothing when z has
// no finite, non-negative upper end.
func invSquare(z iv) (iv, bool) {
	if z.bot || z.phi || z.hi.sign() < 0 {
		return iv{}, false
	}
	r := isqrtE(z.hi)
	return rangeE(r.neg(), r), true
}

// invMulConst is what x·c ∈ z says of x, for a constant c ≠ 0: x = z/c exactly,
// so x ∈ [⌈z.lo/c⌉, ⌊z.hi/c⌋], the ends swapped for c < 0. A finite end of z
// gives an end of x; an infinite one gives the matching infinity.
func invMulConst(z iv, c int64) (iv, bool) {
	if z.bot || c == 0 || (z.nlo && z.phi) {
		return iv{}, false
	}
	out := iv{nlo: true, phi: true}
	lo, hi, k := z.lo, z.hi, ei(c)
	if c > 0 {
		if !z.nlo {
			out.lo, out.nlo = ceilDivE(lo, k), false
		}
		if !z.phi {
			out.hi, out.phi = floorDivE(hi, k), false
		}
	} else {
		if !z.phi {
			out.lo, out.nlo = ceilDivE(hi, k), false
		}
		if !z.nlo {
			out.hi, out.phi = floorDivE(lo, k), false
		}
	}
	if !out.nlo && !out.phi && out.hi.lt(out.lo) {
		return iv{bot: true}, true // no x times c lies in z: the arm is unreachable
	}
	return out.norm(), true
}

// isqrt is ⌊√n⌋ for n ≥ 0, exact: the float estimate is corrected in integers,
// and the comparison r+1 ≤ n/(r+1) cannot overflow.
func isqrt(n int64) int64 {
	r := int64(math.Sqrt(float64(n)))
	for r > 0 && r > n/r {
		r--
	}
	for r+1 <= n/(r+1) {
		r++
	}
	return r
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func ceilDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) == (b < 0)) {
		q++
	}
	return q
}

// cond applies what an arm's guard says to the arm: on the true arm the
// condition holds, on the false arm its negation does, on every execution of
// the arm. A comparison `x rel y` narrows both operands and propagates backward
// (back); `(if a b false)`, which is a ∧ b (L10), gives both on its true arm,
// and `(if a true b)`, a ∨ b, gives both negations on its false arm. What it
// narrows is restored when the arm is left.
//
// A π-parameter carries the same fact for a value the arm READS; this is for
// the rest. Lowering makes no π for a compared value the arm never reads, so
// the sieve's arm under `(>= (* i i) 20000)` knew nothing of i without it.
func (a *intervals) cond(c V, holds bool, saved []savedFact, depth int) []savedFact {
	d := a.def[c]
	if d == nil || depth < 0 {
		return saved
	}
	switch {
	case d.Op.IsCmp() && len(d.Args) == 2:
		rel := relOf(d.Op)
		if !holds {
			rel = negate(rel)
		}
		x, y := d.Args[0], d.Args[1]
		saved = a.back(x, narrowIV(a.fs[x].v, rel, a.fs[y].v), saved, 3)
		saved = a.back(y, narrowIV(a.fs[y].v, flip(rel), a.fs[x].v), saved, 3)
	case d.Op == OCall && u64Rel[d.Name] != "" && len(d.Args) == 2:
		// THE UNSIGNED WORD'S ORDER IS ℤ'S ON U (targets/go/u64.oro): wordsel
		// emits these only where both operands are in U, and a u64 value is in
		// U, so each operand is met with U and the comparison read as ℤ's.
		rel := u64Rel[d.Name]
		if !holds {
			rel = negate(rel)
		}
		x, y := d.Args[0], d.Args[1]
		fx, fy := meetIV(a.fs[x].v, ivU), meetIV(a.fs[y].v, ivU)
		saved = a.back(x, narrowIV(fx, rel, fy), saved, 3)
		saved = a.back(y, narrowIV(fy, flip(rel), fx), saved, 3)
	case d.Op == OIf && len(d.Sub) == 2:
		y0, y1 := firstYieldOf(d.Sub[0]), firstYieldOf(d.Sub[1])
		if len(y0) != 1 || len(y1) != 1 {
			return saved
		}
		switch {
		case holds && isConst(a.def, y1[0], false): // a ∧ b holds: both do
			saved = a.cond(d.Args[0], true, saved, depth-1)
			saved = a.cond(y0[0], true, saved, depth-1)
		case !holds && isConst(a.def, y0[0], true): // a ∨ b fails: both fail
			saved = a.cond(d.Args[0], false, saved, depth-1)
			saved = a.cond(y1[0], false, saved, depth-1)
		}
	}
	return saved
}

// u64Rel is the relation each unsigned comparison is, on U.
var u64Rel = map[string]string{"u64<": "lt", "u64<=": "le", "u64>": "gt", "u64>=": "ge", "u64=": "eq"}

// THE UNSIGNED WORD'S TRANSFERS (targets/go/u64.oro). U = [0, 2⁶⁴) and the
// signed word S = [−2⁶³, 2⁶³) are two sets of representatives of ℤ/2⁶⁴, and
// the primitives are the residue map r between them: `u64-of` is r_U, the
// representative in U; `int-of-u64` is r_S; `u64+ − ·` are r_U of ℤ's result;
// `u64/` and `u64%` are ℤ's on U, where neither wraps. Each transfer is the
// exact image under r of an interval inside one period, and U otherwise, so it
// is sound for every argument and not only for the ones wordsel produces (on
// S ∩ U, which is all it produces, r is the identity and nothing is lost).

// shiftIV is x + k for an end k, x finite.
func shiftIV(x iv, k ep) iv {
	if x.bot {
		return x
	}
	return addIV(x, rangeE(k, k))
}

func u64Of(x iv) iv {
	if x.bot {
		return x
	}
	// r_U(x) = x on [0, 2⁶⁴), and x + 2⁶⁴ on [−2⁶⁴, 0): each piece is an exact
	// image; x reaching outside [−2⁶⁴, 2⁶⁴) wraps more than once, and is U.
	if !x.within(two64.neg(), u64Max) {
		return ivU
	}
	out := meetIV(x, ivU)
	if neg := meetIV(x, iv{hi: ei(-1), nlo: true}); !neg.bot {
		out = joinIV(out, shiftIV(neg, two64))
	}
	return out
}

func intOfU64(x iv) iv {
	x = meetIV(x, ivU) // a u64 is in U
	if x.bot {
		return x
	}
	// r_S(x) = x on [0, 2⁶³), and x − 2⁶⁴ on [2⁶³, 2⁶⁴)
	top, _ := two63.sub(ei(1))
	out := meetIV(x, rangeE(ep{}, top))
	if hi := meetIV(x, rangeE(two63, u64Max)); !hi.bot {
		out = joinIV(out, shiftIV(hi, two64.neg()))
	}
	return out
}

func u64Arith(op string, x, y iv) iv {
	x, y = meetIV(x, ivU), meetIV(y, ivU)
	var r iv
	switch op {
	case "u64+":
		r = addIV(x, y)
	case "u64-":
		r = subIV(x, y)
	case "u64*":
		r = mulIV(x, y)
	case "u64/":
		return divIV(x, y)
	default:
		return remIV(x, y)
	}
	// r_U is the identity where ℤ's result is in U; elsewhere only U is known.
	if r.within(ep{}, u64Max) {
		return r
	}
	return ivU
}

// andIV is the mask's law, in two's complement: a NON-NEGATIVE operand m
// bounds x & m to [0, m] whatever x is (every bit of the result is a bit of m,
// and its sign bit is m's, 0), so with both non-negative the result is in
// [0, min]. With neither non-negative nothing is said. These are `and-left`
// and `and-right` (emit/lang-facts.oro, F9), each applied where its premise
// holds on the whole interval.
func andIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	nonNeg := func(x iv) bool { return !x.nlo && x.lo.sign() >= 0 }
	switch {
	case nonNeg(a) && nonNeg(b):
		if a.phi && b.phi {
			return iv{phi: true}
		}
		if a.phi {
			return rangeE(ep{}, b.hi)
		}
		if b.phi {
			return rangeE(ep{}, a.hi)
		}
		return rangeE(ep{}, minE(a.hi, b.hi))
	case nonNeg(a):
		return iv{hi: a.hi, phi: a.phi}.norm()
	case nonNeg(b):
		return iv{hi: b.hi, phi: b.phi}.norm()
	}
	return ivTop
}

// shrIV is the right shift on a NON-NEGATIVE x, where the logical and the
// arithmetic shift agree: x >> k = ⌊x / 2ᵏ⌋, increasing in x and decreasing in
// k, so for k ∈ [kl, kh] ⊆ [0, 62] it lies in [⌊x.lo / 2^kh⌋, ⌊x.hi / 2^kl⌋].
// Anything else is ⊤, as the term analysis's rule is.
func shrIV(a, k iv) iv {
	if a.bot || k.bot {
		return ivBot
	}
	kl, okl := k.lo.i64()
	kh, okh := k.hi.i64()
	if a.nlo || a.lo.sign() < 0 || !k.finite() || !okl || !okh || kl < 0 || kh > 62 {
		return ivTop
	}
	out := iv{lo: floorDivE(a.lo, ei(1<<kh)), phi: a.phi}
	if !a.phi {
		out.hi = floorDivE(a.hi, ei(1<<kl))
	}
	return out.norm()
}

// relOf is the relation a comparison operation is.
func relOf(o Op) string {
	switch o {
	case OEq:
		return "eq"
	case ONe:
		return "ne"
	case OLt:
		return "lt"
	case OLe:
		return "le"
	case OGt:
		return "gt"
	case OGe:
		return "ge"
	}
	return ""
}
