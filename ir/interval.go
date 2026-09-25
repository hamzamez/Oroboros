package ir

import (
	"math"
	"math/big"
	"math/bits"

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

func joinIV(a, b iv) iv {
	switch {
	case a.bot:
		return b
	case b.bot:
		return a
	}
	out := iv{nlo: a.nlo || b.nlo, phi: a.phi || b.phi}
	out.lo, out.hi = min(a.lo, b.lo), max(a.hi, b.hi)
	return out
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
	return out
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
	return out
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
		return ivTop
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
	if !b.finite() {
		if !a.nlo && a.lo >= 0 {
			return iv{lo: 0, hi: a.hi, phi: a.phi}
		}
		return ivTop
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
		return ivTop
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
	return rangeIV(-bound, bound)
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
}

type exitFacts struct {
	cont, brk       []fact
	hasCont, hasBrk bool
}

// loopBudget bounds a loop's ascending iteration. Widening after round 3 makes
// convergence certain in finitely many rounds for this domain; the budget is a
// guard, and a loop that exhausts it takes ⊤, which is sound.
const loopBudget = 64

func analyse(tg *emit.Target, f *Func) []fact {
	a := &intervals{tg: tg, f: f, maxLen: tg.Word.Hi, fs: make([]fact, f.NV()), def: map[V]*Stmt{}}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				a.def[v] = &r.Stmts[i]
			}
		}
	})
	if tg.MaxLen > 0 && tg.MaxLen < a.maxLen {
		a.maxLen = tg.MaxLen
	}
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
	for _, pi := range r.Pis {
		x := a.fs[pi.Of]
		if pi.Len {
			if x.hasLn {
				x.ln = narrowIV(x.ln, pi.Rel, a.fs[pi.Other].v)
			}
		} else {
			x.v = narrowIV(x.v, pi.Rel, a.fs[pi.Other].v)
		}
		a.fs[pi.V] = x
	}
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
		}
	case TBranch:
		y1, ok1 := a.region(r.Then)
		y2, ok2 := a.region(r.Else)
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
		y1, ok1 := a.region(s.Sub[0])
		y2, ok2 := a.region(s.Sub[1])
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
		*lc = exitFacts{}
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
			if it >= 3 {
				next[i] = widenF(cur[i], joinF(cur[i], next[i]))
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
	*lc = exitFacts{}
	a.region(body)
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
