package ir

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
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
	lo, hi   int64
	nlo, phi bool // lo is −∞, hi is +∞
	bot      bool
}

var ivTop = iv{nlo: true, phi: true}
var ivBot = iv{bot: true}

func exactIV(n int64) iv      { return iv{lo: n, hi: n} }
func rangeIV(lo, hi int64) iv { return iv{lo: lo, hi: hi} }

func (a iv) finite() bool { return !a.bot && !a.nlo && !a.phi }

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
		a.lo = 0
	}
	if a.phi {
		a.hi = 0
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
	out.lo, out.hi = min(a.lo, b.lo), max(a.hi, b.hi)
	return out.norm()
}

func meetIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	out := a
	if !b.nlo && (out.nlo || b.lo > out.lo) {
		out.lo, out.nlo = b.lo, false
	}
	if !b.phi && (out.phi || b.hi < out.hi) {
		out.hi, out.phi = b.hi, false
	}
	if !out.nlo && !out.phi && out.lo > out.hi {
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
	if nw.nlo || nw.lo < old.lo {
		out.nlo = true
	}
	if nw.phi || nw.hi > old.hi {
		out.phi = true
	}
	return out.norm()
}

// addE adds two endpoints, saturating: an overflow becomes the infinite end,
// which only loses precision.
func addE(x, y int64) (int64, int) {
	s, _ := bits.Add64(uint64(x), uint64(y), 0)
	r := int64(s)
	if (x >= 0) == (y >= 0) && (r >= 0) != (x >= 0) {
		if x >= 0 {
			return math.MaxInt64, 1
		}
		return math.MinInt64, -1
	}
	return r, 0
}

func addIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	out := iv{nlo: a.nlo || b.nlo, phi: a.phi || b.phi}
	if !out.nlo {
		lo, o := addE(a.lo, b.lo)
		out.lo = lo
		if o < 0 {
			out.nlo = true
		}
		if o > 0 {
			out.phi = true
		}
	}
	if !out.phi {
		hi, o := addE(a.hi, b.hi)
		out.hi = hi
		if o > 0 {
			out.phi = true
		}
		if o < 0 {
			out.nlo = true
		}
	}
	return out
}

func negIV(a iv) iv {
	if a.bot {
		return ivBot
	}
	out := iv{nlo: a.phi, phi: a.nlo}
	if !a.phi {
		if a.hi == math.MinInt64 {
			out.phi = true
		} else {
			out.lo = -a.hi
		}
	}
	if !a.nlo {
		if a.lo == math.MinInt64 {
			out.phi = true
		} else {
			out.hi = -a.lo
		}
	}
	return out
}

func subIV(a, b iv) iv { return addIV(a, negIV(b)) }

func abs64(x int64) int64 {
	if x < 0 {
		if x == math.MinInt64 {
			return math.MaxInt64
		}
		return -x
	}
	return x
}

// mulIV is the hull of the four corners; an infinite or overflowing corner
// makes its side infinite. Each corner is computed EXACTLY, in big.Int: a
// magnitude through abs64 saturates |MinInt64| = 2⁶³ to 2⁶³−1, which put
// 1·MinInt64 outside its own interval (found by the soundness table).
func mulIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	zero := func(x iv) bool { return x.finite() && x.lo == 0 && x.hi == 0 }
	if !a.finite() || !b.finite() {
		if zero(a) || zero(b) {
			return exactIV(0)
		}
		return ivTop
	}
	out, first := iv{}, true
	for _, x := range []int64{a.lo, a.hi} {
		for _, y := range []int64{b.lo, b.hi} {
			p := new(big.Int).Mul(big.NewInt(x), big.NewInt(y))
			if !p.IsInt64() {
				if p.Sign() < 0 {
					out.nlo = true
				} else {
					out.phi = true
				}
				continue
			}
			v := p.Int64()
			if first {
				out.lo, out.hi, first = v, v, false
			} else {
				out.lo, out.hi = min(out.lo, v), max(out.hi, v)
			}
		}
	}
	if first { // every corner overflowed: the finite ends are the word's
		out.lo, out.hi = math.MinInt64, math.MaxInt64
	}
	return out
}

// divIV is truncating division (integers.md §3). Only a strictly positive
// divisor is modelled; anything else is ⊤, which is sound.
func divIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if !b.finite() || b.lo <= 0 || !a.finite() {
		if !b.nlo && b.lo >= 1 && !a.nlo && a.lo >= 0 { // a ≥ 0, b ≥ 1: 0 ≤ a/b ≤ a
			return iv{lo: 0, hi: a.hi, phi: a.phi}
		}
		// |a/b| ≤ |a| for every divisor b ≠ 0, and b ≠ 0 at every division
		// that runs: it is `div`'s domain condition, an obligation discharged
		// or the program refused (refinements.md §3a).
		return dividendBound(a)
	}
	c := []int64{a.lo / b.lo, a.lo / b.hi, a.hi / b.lo, a.hi / b.hi}
	out := exactIV(c[0])
	for _, x := range c[1:] {
		out.lo, out.hi = min(out.lo, x), max(out.hi, x)
	}
	return out
}

// remIV: |a rem b| < |b|, with the dividend's sign (truncating remainder).
func remIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	// The remainder lies between 0 and the dividend, whatever the divisor:
	// |a % b| ≤ |a| with a's sign (truncating remainder, integers.md §3).
	between := iv{lo: min(a.lo, 0), hi: max(a.hi, 0), nlo: a.nlo, phi: a.phi}
	if !b.finite() {
		return between
	}
	// |r| ≤ |b| − 1, the magnitude taken unsigned: |MinInt64| = 2⁶³ is
	// representable there and not in int64. The bound is at most 2⁶³ − 1, so it
	// fits; and r has the dividend's sign (truncating remainder).
	mag := func(x int64) uint64 {
		if x < 0 {
			return uint64(-(x + 1)) + 1
		}
		return uint64(x)
	}
	mu := max(mag(b.lo), mag(b.hi))
	if mu == 0 {
		return between
	}
	bound := int64(mu - 1)
	switch {
	case !a.nlo && a.lo >= 0:
		hi := bound
		if !a.phi && a.hi < hi {
			hi = a.hi
		}
		return rangeIV(0, hi)
	case !a.phi && a.hi <= 0:
		lo := -bound
		if !a.nlo && a.lo > lo {
			lo = a.lo
		}
		return rangeIV(lo, 0)
	}
	return meetIV(rangeIV(-bound, bound), between)
}

// dividendBound is what |q| ≤ |a| says of a quotient: q ∈ [−M, M] for M the
// dividend's largest magnitude, an infinite end where a has one. M = 2⁶³ (a
// reaching MinInt64) is past int64, so that side is open.
func dividendBound(a iv) iv {
	if a.nlo || a.phi {
		return ivTop
	}
	if a.lo == math.MinInt64 {
		return iv{lo: math.MinInt64, phi: true}
	}
	m := max(abs64(a.lo), abs64(a.hi))
	return rangeIV(-m, m)
}

// narrowIV is a π's fact: x where `x rel o` holds (Theorem E).
func narrowIV(x iv, rel string, o iv) iv {
	if x.bot || o.bot {
		return ivBot
	}
	switch rel {
	case "lt":
		if !o.phi {
			if h, ov := addE(o.hi, -1); ov == 0 {
				return meetIV(x, iv{hi: h, nlo: true})
			}
		}
	case "le":
		if !o.phi {
			return meetIV(x, iv{hi: o.hi, nlo: true})
		}
	case "gt":
		if !o.nlo {
			if l, ov := addE(o.lo, 1); ov == 0 {
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
		if o.finite() && o.lo == o.hi && x.finite() && x.lo < x.hi {
			if x.lo == o.lo {
				return rangeIV(x.lo+1, x.hi)
			}
			if x.hi == o.lo {
				return rangeIV(x.lo, x.hi-1)
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
	thresh []int64 // the widening's thresholds (widenIVT)
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
	a := &intervals{tg: tg, f: f, maxLen: tg.Word.Hi, fs: make([]fact, f.NV()), def: map[V]*Stmt{}, piOf: map[V]V{}}
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
	a.region(f.Body)
	return a.fs
}

// declared is what a declared type says of a value: an integer range, or a
// table's length and element range.
func (a *intervals) declared(ty string) fact {
	out := topFact
	if lo, hi, ok := core.IntRange(ty); ok {
		out.v = rangeIV(lo, hi)
	}
	if e := ElemOf(a.tg, unbuffer(ty)); e != "" {
		out.hasLn, out.ln = true, rangeIV(0, a.maxLen)
		out.hasEl, out.el = true, ivTop
		if lo, hi, ok := core.IntRange(e); ok {
			out.el = rangeIV(lo, hi)
		}
	}
	return out
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
				lc.pre, lc.hasPre = joinAll(lc.pre, lc.hasPre, a.facts(lc.params)), true
			}
		}
	case TBranch:
		sv := a.cond(r.Cond, true, nil, 3)
		y1, ok1 := a.region(r.Then)
		a.restore(sv)
		sv = a.cond(r.Cond, false, nil, 3)
		y2, ok2 := a.region(r.Else)
		a.restore(sv)
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
		sv := a.cond(s.Args[0], true, nil, 3)
		y1, ok1 := a.region(s.Sub[0])
		a.restore(sv)
		sv = a.cond(s.Args[0], false, nil, 3)
		y2, ok2 := a.region(s.Sub[1])
		a.restore(sv)
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
			i = iv{lo: 0, hi: n.hi - 1, phi: n.phi}
			if n.finite() && n.hi == 0 {
				i = ivBot
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
}

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
		*lc = exitFacts{params: body.Params}
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
	*lc = exitFacts{params: body.Params}
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
		*lc = exitFacts{params: body.Params}
		a.region(body)
	}
	a.loops = a.loops[:len(a.loops)-1]
	a.set(s.Res, lc.brk)
}

// assume is the abstract semantics of `assume c` (Theorem E's reasoning):
// every execution past it satisfies c, so each conjunct comparing a value, or
// a table's length, with another value narrows that value's fact. An `assume`
// sits at the function's entry and values are immutable, so the narrowed fact
// holds wherever the value is used.
func (a *intervals) assume(c V) {
	d := a.def[c]
	if d == nil {
		return
	}
	switch {
	case d.Op == OIf: // (if x y false) is x ∧ y (L10)
		if y := firstYieldOf(d.Sub[1]); len(y) == 1 && isConst(a.def, y[0], false) {
			a.assume(d.Args[0])
			if y0 := firstYieldOf(d.Sub[0]); len(y0) == 1 {
				a.assume(y0[0])
			}
		}
	case d.Op.IsCmp():
		rel := map[Op]string{OEq: "eq", ONe: "ne", OLt: "lt", OLe: "le", OGt: "gt", OGe: "ge"}[d.Op]
		a.narrowAssumed(d.Args[0], rel, a.fs[d.Args[1]].v)
		a.narrowAssumed(d.Args[1], flip(rel), a.fs[d.Args[0]].v)
	}
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
	lo, hi := big.NewInt(a.lo).String(), big.NewInt(a.hi).String()
	if a.nlo {
		lo = "-∞"
	}
	if a.phi {
		hi = "+∞"
	}
	return "[" + lo + ", " + hi + "]"
}

// ProofCount is the IR's interval domain used as a LEGALITY checker: the
// integer operations `add`, `sub`, `mul` and `neg` (the population the term
// analysis counts, emit/interval.go's `record`) and how many the domain proves
// inside the target's signed word. An unreachable one (⊥) is proven. A u64
// operation the unsigned-word pass selected is counted too, and proven inside
// U = [0, 2⁶⁴−1]. It is the shadow measurement of ADR 0032 step 4: the
// analysis moves to the IR when these counts do not fall below the term
// analysis's on any program. unproven describes each operation not proven.
//
// An operation PROMOTED from an overloaded host primitive (lowering's
// `promote`, which keeps the primitive's name) is not in the population: it is
// ℤ's only under the premise of being inside the word, and otherwise the
// host's, which is legal. Those are counted apart, in host and hostProven.
//
// Nor is an operation whose value only feeds assumptions: an export's `where`,
// lowered to statements an `assume` reads. That is specification, a claim in ℤ
// about the arguments at the boundary, which no printer emits and the term
// analysis never counted.
func ProofCount(tg *emit.Target, f *Func) (proven, total, hostProven, host int, unproven []string) {
	fs := analyse(tg, f)
	spec := specOnly(f)
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if len(s.Res) != 1 || spec[s.Res[0]] {
				continue
			}
			v := fs[s.Res[0]].v
			inWord := v.bot || (v.finite() && v.lo >= tg.Word.Lo && v.hi <= tg.Word.Hi)
			arith := s.Op == OAdd || s.Op == OSub || s.Op == OMul || s.Op == ONeg
			if arith && s.Name != "" {
				host++
				if inWord {
					hostProven++
				}
				continue
			}
			var ok bool
			switch {
			case arith:
				ok = inWord
			case s.Op == OCall && (s.Name == "u64+" || s.Name == "u64-" || s.Name == "u64*"):
				// U's top end is past int64, which this domain's endpoints are,
				// so only a finite non-negative interval is known inside U.
				ok = v.bot || (v.finite() && v.lo >= 0)
			default:
				continue
			}
			total++
			if ok {
				proven++
			} else {
				ops := ""
				for _, a := range s.Args {
					ops += fmt.Sprintf(" %%%d%s", a, show(fs[a].v))
				}
				unproven = append(unproven, fmt.Sprintf("%s %s ←%s", describe(s), show(v), ops))
			}
		}
	})
	return proven, total, hostProven, host, unproven
}

// describe names a statement for a report.
func describe(s *Stmt) string {
	if s.Op == OCall {
		return s.Name
	}
	return s.Op.String()
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
func widenIVT(old, nw iv, t []int64) iv {
	if old.bot {
		return nw
	}
	if nw.bot {
		return old
	}
	out := old
	if nw.nlo {
		out.nlo = true
	} else if nw.lo < old.lo {
		// the greatest threshold at or below the new low end
		i := sort.Search(len(t), func(k int) bool { return t[k] > nw.lo })
		if i == 0 {
			out.nlo = true
		} else {
			out.lo = t[i-1]
		}
	}
	if nw.phi {
		out.phi = true
	} else if nw.hi > old.hi {
		// the least threshold at or above the new high end
		i := sort.Search(len(t), func(k int) bool { return t[k] >= nw.hi })
		if i == len(t) {
			out.phi = true
		} else {
			out.hi = t[i]
		}
	}
	return out.norm()
}

func widenFT(old, nw fact, t []int64) fact {
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
// neighbours, and the ends of the target's word.
func thresholds(tg *emit.Target, f *Func) []int64 {
	set := map[int64]bool{tg.Word.Lo: true, tg.Word.Hi: true, 0: true}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OConst && s.Lit.Kind == core.KInt {
				k := s.Lit.Int
				set[k] = true
				if k > math.MinInt64 {
					set[k-1] = true
				}
				if k < math.MaxInt64 {
					set[k+1] = true
				}
			}
		}
	})
	out := make([]int64, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
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
	if d := a.def[v]; d != nil && d.Op == OConst && d.Lit.Kind == core.KInt {
		return d.Lit.Int, true
	}
	return 0, false
}

// invSquare is what x·x ∈ z says of x: |x| ≤ ⌊√z.hi⌋, and nothing when z has
// no finite, non-negative upper end.
func invSquare(z iv) (iv, bool) {
	if z.bot || z.phi || z.hi < 0 {
		return iv{}, false
	}
	r := isqrt(z.hi)
	return rangeIV(-r, r), true
}

// invMulConst is what x·c ∈ z says of x, for a constant c ≠ 0: x = z/c exactly,
// so x ∈ [⌈z.lo/c⌉, ⌊z.hi/c⌋], the ends swapped for c < 0. A finite end of z
// gives an end of x; an infinite one gives the matching infinity.
func invMulConst(z iv, c int64) (iv, bool) {
	if z.bot || c == 0 || (z.nlo && z.phi) {
		return iv{}, false
	}
	out := iv{nlo: true, phi: true}
	lo, hi := z.lo, z.hi
	if c > 0 {
		if !z.nlo {
			out.lo, out.nlo = ceilDiv(lo, c), false
		}
		if !z.phi {
			out.hi, out.phi = floorDiv(hi, c), false
		}
	} else {
		if !z.phi {
			out.lo, out.nlo = ceilDiv(hi, c), false
		}
		if !z.nlo {
			out.hi, out.phi = floorDiv(lo, c), false
		}
	}
	if !out.nlo && !out.phi && out.lo > out.hi {
		return iv{bot: true}, true // no x times c lies in z: the arm is unreachable
	}
	return out, true
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
		rel := map[Op]string{OEq: "eq", ONe: "ne", OLt: "lt", OLe: "le", OGt: "gt", OGe: "ge"}[d.Op]
		if !holds {
			rel = negate(rel)
		}
		x, y := d.Args[0], d.Args[1]
		saved = a.back(x, narrowIV(a.fs[x].v, rel, a.fs[y].v), saved, 3)
		saved = a.back(y, narrowIV(a.fs[y].v, flip(rel), a.fs[x].v), saved, 3)
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

// THE UNSIGNED WORD'S TRANSFERS (targets/go/u64.oro). U = [0, 2⁶⁴) and the
// signed word S = [−2⁶³, 2⁶³) are two sets of representatives of ℤ/2⁶⁴, and
// the primitives are the residue map r between them: `u64-of` is r_U, the
// representative in U; `int-of-u64` is r_S; `u64+ − ·` are r_U of ℤ's result;
// `u64/` and `u64%` are ℤ's on U, where neither wraps. Each transfer is the
// exact image under r, so it is sound for every argument, not only for the
// ones wordsel produces (on S ∩ U, which is all it produces, r is the
// identity and nothing is lost). This domain's ends are int64, so a value in
// [2⁶³, 2⁶⁴) is past the top end: it has hi = +∞.
var ivU = iv{lo: 0, phi: true}

func u64Of(x iv) iv {
	if x.bot {
		return x
	}
	out := meetIV(x, ivU)
	if x.nlo || x.lo < 0 { // a negative x is x + 2⁶⁴ ≥ 2⁶³
		out = joinIV(out, iv{lo: math.MaxInt64, phi: true})
	}
	return out
}

func intOfU64(x iv) iv {
	x = meetIV(x, ivU) // a u64 is in U
	if x.bot || !x.phi {
		return x
	}
	// a value in [2⁶³, 2⁶⁴) is itself − 2⁶⁴, in [−2⁶³, −1]
	return joinIV(x, rangeIV(math.MinInt64, -1))
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
	// r_U is the identity where ℤ's result is in U. Below 0 it wraps up, and
	// past +∞ (an infinite end here) it may wrap down: then only U is known.
	if r.bot || (!r.nlo && r.lo >= 0 && (op == "u64-" || !r.phi)) {
		return r
	}
	return ivU
}
