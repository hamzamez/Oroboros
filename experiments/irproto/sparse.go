package main

// P3 of docs/ir-research.md: an interval analysis on C2 with π-parameters.
//
// SPARSE: every value has exactly ONE fact, kept in a dense vector indexed by
// the value, because a guard's narrowing belongs to the π that renames the
// guarded variable (research §4b). So there is no environment per program point,
// and nothing is ever copied: a loop's fixpoint overwrites its body's facts on
// each iteration. That is the claim P1 left to measure.
//
// The domain is a small reduced product: an integer interval, and for a table a
// length interval and an element interval (array smashing, a weak update per
// store). It is deliberately NOT interval.go's full set of theorems: no trip
// counts, no monotone steps, no content facts. The count it proves is a floor,
// and the operations it does not prove are the list of what a port must carry.

import (
	"fmt"
	"math"
	"math/bits"

	"oroboros/core"
	"oroboros/emit"
)

// ---------------------------------------------------------------- intervals

type iv struct {
	lo, hi   int64
	nlo, phi bool // lo is −∞, hi is +∞
	bot      bool
}

var ivTop = iv{nlo: true, phi: true}
var ivBot = iv{bot: true}

func exactIV(n int64) iv      { return iv{lo: n, hi: n} }
func rangeIV(lo, hi int64) iv { return iv{lo: lo, hi: hi} }
func (a iv) fits(w core.Word) bool {
	return a.bot || (!a.nlo && !a.phi && a.lo >= w.Lo && a.hi <= w.Hi)
}

func join(a, b iv) iv {
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

func meet(a, b iv) iv {
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

func widen(old, nw iv) iv {
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

func eqIV(a, b iv) bool { return a == b }

// saturating endpoint arithmetic: an overflowing endpoint becomes infinite, which
// only ever loses precision (the interval is then outside the word anyway).
func addE(x, y int64) (int64, int) {
	s, c := bits.Add64(uint64(x), uint64(y), 0)
	_ = c
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
		if o > 0 { // the least value is past the word: every value is
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

// mulIV: the hull of the four corners, any infinite or overflowing corner
// making its side infinite.
func mulIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if a.nlo || a.phi || b.nlo || b.phi {
		// a finite zero interval times anything is zero
		if (!a.nlo && !a.phi && a.lo == 0 && a.hi == 0) || (!b.nlo && !b.phi && b.lo == 0 && b.hi == 0) {
			return exactIV(0)
		}
		return ivTop
	}
	out := iv{}
	first := true
	for _, x := range []int64{a.lo, a.hi} {
		for _, y := range []int64{b.lo, b.hi} {
			hi, lo := bits.Mul64(uint64(abs64(x)), uint64(abs64(y)))
			neg := (x < 0) != (y < 0)
			if hi != 0 || lo > math.MaxInt64 {
				if neg {
					out.nlo = true
				} else {
					out.phi = true
				}
				continue
			}
			p := int64(lo)
			if neg {
				p = -p
			}
			if first {
				out.lo, out.hi, first = p, p, false
			} else {
				out.lo, out.hi = min(out.lo, p), max(out.hi, p)
			}
		}
	}
	if first {
		out.lo, out.hi = 0, 0
	}
	return out
}

func abs64(x int64) int64 {
	if x < 0 {
		if x == math.MinInt64 {
			return math.MaxInt64
		}
		return -x
	}
	return x
}

func divIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if b.nlo || b.phi || b.lo <= 0 || a.nlo || a.phi {
		if !b.nlo && b.lo >= 1 && !a.nlo && a.lo >= 0 { // a ≥ 0, b ≥ 1: 0 ≤ a/b ≤ a
			return iv{lo: 0, hi: a.hi, phi: a.phi}
		}
		return ivTop
	}
	c := []int64{a.lo / b.lo, a.lo / b.hi, a.hi / b.lo, a.hi / b.hi}
	out := rangeIV(c[0], c[0])
	for _, x := range c[1:] {
		out.lo, out.hi = min(out.lo, x), max(out.hi, x)
	}
	return out
}

func remIV(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if b.nlo || b.phi {
		if !a.nlo && a.lo >= 0 {
			return iv{lo: 0, hi: a.hi, phi: a.phi}
		}
		return ivTop
	}
	m := max(abs64(b.lo), abs64(b.hi))
	if m == 0 {
		return ivTop
	}
	switch {
	case !a.nlo && a.lo >= 0:
		hi := m - 1
		if !a.phi && a.hi < hi {
			hi = a.hi
		}
		return rangeIV(0, hi)
	case !a.phi && a.hi <= 0:
		lo := -(m - 1)
		if !a.nlo && a.lo > lo {
			lo = a.lo
		}
		return rangeIV(lo, 0)
	}
	return rangeIV(-(m - 1), m-1)
}

// narrow is a π-node's fact: x where `x rel o` holds.
func narrow(x iv, rel string, o iv) iv {
	if x.bot || o.bot {
		return ivBot
	}
	switch rel {
	case "lt":
		if !o.phi {
			h, ov := addE(o.hi, -1)
			if ov == 0 {
				return meet(x, iv{hi: h, nlo: true})
			}
		}
	case "le":
		if !o.phi {
			return meet(x, iv{hi: o.hi, nlo: true})
		}
	case "gt":
		if !o.nlo {
			l, ov := addE(o.lo, 1)
			if ov == 0 {
				return meet(x, iv{lo: l, phi: true})
			}
		}
	case "ge":
		if !o.nlo {
			return meet(x, iv{lo: o.lo, phi: true})
		}
	case "eq":
		return meet(x, o)
	case "ne":
		if !o.nlo && !o.phi && o.lo == o.hi && !x.nlo && !x.phi {
			if x.lo == o.lo && x.lo < x.hi {
				return rangeIV(x.lo+1, x.hi)
			}
			if x.hi == o.lo && x.lo < x.hi {
				return rangeIV(x.lo, x.hi-1)
			}
		}
	}
	return x
}

// ---------------------------------------------------------------- the analysis

type fact struct {
	v            iv
	ln, el       iv
	hasLn, hasEl bool
}

func joinF(a, b fact) fact {
	out := fact{v: join(a.v, b.v)}
	if a.hasLn && b.hasLn {
		out.ln, out.hasLn = join(a.ln, b.ln), true
	}
	if a.hasEl && b.hasEl {
		out.el, out.hasEl = join(a.el, b.el), true
	}
	return out
}

func widenF(old, nw fact) fact {
	out := fact{v: widen(old.v, nw.v)}
	if old.hasLn && nw.hasLn {
		out.ln, out.hasLn = widen(old.ln, nw.ln), true
	}
	if old.hasEl && nw.hasEl {
		out.el, out.hasEl = widen(old.el, nw.el), true
	}
	return out
}

func meetF(a, b fact) fact {
	out := fact{v: meet(a.v, b.v)}
	if a.hasLn && b.hasLn {
		out.ln, out.hasLn = meet(a.ln, b.ln), true
	}
	if a.hasEl && b.hasEl {
		out.el, out.hasEl = meet(a.el, b.el), true
	}
	return out
}

type loopCtx struct {
	cont, brk       []fact
	hasCont, hasBrk bool
}

type Sparse struct {
	tg       *emit.Target
	word     core.Word
	maxLen   int64
	f        []fact
	counting bool
	loops    []*loopCtx
	Ops      int
	Proven   int
	Unproven []string
	Evals    int // operations evaluated, fixpoint iterations included
}

func Analyse(tg *emit.Target, fn *Func, sig *core.Sig) *Sparse {
	a := &Sparse{tg: tg, word: tg.Word, maxLen: tg.Word.Hi, f: make([]fact, fn.NV)}
	if tg.MaxLen > 0 && tg.MaxLen < a.maxLen {
		a.maxLen = tg.MaxLen
	}
	for i, p := range fn.Params {
		a.f[p] = fact{v: ivTop}
		if sig != nil && i < len(sig.Params) {
			ty := sig.Params[i].Type
			if lo, hi, ok := core.IntRange(ty); ok {
				a.f[p].v = rangeIV(lo, hi)
			}
			if el := core.ArrayElem(ty); el != "" {
				a.f[p].hasLn, a.f[p].ln = true, rangeIV(0, a.maxLen)
				a.f[p].hasEl, a.f[p].el = true, ivTop
				if lo, hi, ok := core.IntRange(el); ok {
					a.f[p].el = rangeIV(lo, hi)
				}
			}
		}
	}
	if sig != nil && sig.Where != nil {
		a.assumeWhere(sig.Where, sig, fn)
	}
	a.counting = true
	a.region(fn.Body)
	return a
}

// assumeWhere takes a signature's `where` conjunction of comparisons between a
// parameter's length (or a parameter) and a literal.
func (a *Sparse) assumeWhere(w *core.Term, sig *core.Sig, fn *Func) {
	if w.Kind != core.KApp || w.Kids[0].Kind != core.KName {
		return
	}
	if len(w.Kids) == 4 && w.Kids[3].Kind == core.KBool && !w.Kids[3].IsTrue() { // and
		a.assumeWhere(w.Kids[1], sig, fn)
		a.assumeWhere(w.Kids[2], sig, fn)
		return
	}
	rel := emit.CmpOp(w.Kids[0].Name)
	if rel == "" || len(w.Kids) != 3 {
		return
	}
	param := func(t *core.Term) (int, bool, bool) { // index, is-length
		if t.Kind == core.KName {
			for i, p := range sig.Params {
				if p.Name == t.Name {
					return i, false, true
				}
			}
		}
		if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName && emit.ArithOp(t.Kids[0].Name, 1) == "" &&
			t.Kids[1].Kind == core.KName {
			if p, ok := a.tg.Prims[t.Kids[0].Name]; ok && p.Kind == "len" {
				for i, sp := range sig.Params {
					if sp.Name == t.Kids[1].Name {
						return i, true, true
					}
				}
			}
		}
		return 0, false, false
	}
	x, lit := w.Kids[1], w.Kids[2]
	if lit.Kind != core.KInt {
		x, lit, rel = w.Kids[2], w.Kids[1], flip(rel)
	}
	if lit.Kind != core.KInt {
		return
	}
	i, isLen, ok := param(x)
	if !ok || i >= len(fn.Params) {
		return
	}
	p := fn.Params[i]
	if isLen {
		a.f[p].ln = narrow(a.f[p].ln, rel, exactIV(lit.Int))
	} else {
		a.f[p].v = narrow(a.f[p].v, rel, exactIV(lit.Int))
	}
}

// region evaluates a region and returns what it yields.
func (a *Sparse) region(r *Region) ([]fact, bool) {
	for _, pi := range r.Pis {
		x := a.f[pi.Of]
		if pi.Len {
			if x.hasLn {
				x.ln = narrow(x.ln, pi.Rel, a.f[pi.Other].v)
			}
		} else {
			x.v = narrow(x.v, pi.Rel, a.f[pi.Other].v)
		}
		a.f[pi.V] = x
	}
	for i := range r.Ops {
		a.op(&r.Ops[i])
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
		return nil, false
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

func (a *Sparse) facts(vs []V) []fact {
	out := make([]fact, len(vs))
	for i, v := range vs {
		if v >= 0 {
			out[i] = a.f[v]
		} else {
			out[i] = fact{v: ivTop}
		}
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

func (a *Sparse) set(res []V, fs []fact) {
	for i, v := range res {
		if i < len(fs) {
			a.f[v] = fs[i]
		} else {
			a.f[v] = fact{v: ivTop}
		}
	}
}

func (a *Sparse) op(op *Op) {
	a.Evals++
	switch op.Kind {
	case OpConst:
		if op.Lit.Kind == core.KInt {
			a.f[op.Res[0]] = fact{v: exactIV(op.Lit.Int)}
		} else {
			a.f[op.Res[0]] = fact{v: ivTop}
		}
	case OpGlobal, OpOpaque:
		a.f[op.Res[0]] = fact{v: ivTop}
	case OpIndex:
		t := a.f[op.Args[0]]
		if t.hasEl {
			a.f[op.Res[0]] = fact{v: t.el}
		} else {
			a.f[op.Res[0]] = fact{v: ivTop}
		}
	case OpMapRead:
		a.f[op.Res[0]] = fact{v: rangeIV(0, 1)}
		a.f[op.Res[1]] = fact{v: ivTop}
	case OpIf:
		y1, ok1 := a.region(op.Sub[0])
		y2, ok2 := a.region(op.Sub[1])
		switch {
		case ok1 && ok2:
			a.set(op.Res, joinAll(y1, true, y2))
		case ok1:
			a.set(op.Res, y1)
		case ok2:
			a.set(op.Res, y2)
		default:
			a.set(op.Res, nil)
		}
	case OpBuild:
		body := op.Sub[0]
		n := a.f[op.Args[0]].v
		buf := fact{v: ivTop, hasLn: true, ln: meet(n, rangeIV(0, a.maxLen)), hasEl: true, el: exactIV(0)}
		a.f[body.Params[0]] = buf
		y, _ := a.region(body)
		a.set(op.Res, y)
	case OpLoop:
		a.loop(op)
	case OpPrim:
		a.prim(op)
	}
}

// loop is the fixpoint: widening after three rounds, then two descending rounds,
// then the final evaluation, which alone counts.
func (a *Sparse) loop(op *Op) {
	body := op.Sub[0]
	init := a.facts(op.Args)
	cur := append([]fact(nil), init...)
	lc := &loopCtx{}
	a.loops = append(a.loops, lc)
	counting := a.counting
	a.counting = false
	round := func() []fact {
		for i, p := range body.Params {
			a.f[p] = cur[i]
		}
		*lc = loopCtx{}
		a.region(body)
		if !lc.hasCont {
			return append([]fact(nil), init...)
		}
		return joinAll(append([]fact(nil), init...), true, lc.cont)
	}
	for it := 0; it < 64; it++ {
		next := round()
		for i := range next {
			if it >= 3 {
				next[i] = widenF(cur[i], joinF(cur[i], next[i]))
			} else {
				next[i] = joinF(cur[i], next[i])
			}
		}
		same := true
		for i := range next {
			if next[i] != cur[i] {
				same = false
			}
		}
		cur = next
		if same {
			break
		}
	}
	for k := 0; k < 2; k++ { // descending: each round is ⊑ the post-fixpoint
		next := round()
		for i := range next {
			cur[i] = meetF(cur[i], next[i])
		}
	}
	a.counting = counting
	for i, p := range body.Params {
		a.f[p] = cur[i]
	}
	*lc = loopCtx{}
	a.region(body)
	a.loops = a.loops[:len(a.loops)-1]
	a.set(op.Res, lc.brk)
}

func (a *Sparse) prim(op *Op) {
	p, known := a.tg.Prims[op.Name]
	args := a.facts(op.Args)
	res := op.Res
	if !known {
		a.set(res, nil)
		return
	}
	// declared result ranges, one per result
	if len(p.Results) >= 2 {
		fs := make([]fact, len(res))
		for i := range fs {
			fs[i] = fact{v: ivTop}
			if i < len(p.Results) {
				if lo, hi, ok := core.IntRange(p.Results[i]); ok {
					fs[i].v = rangeIV(lo, hi)
				}
				// A TABLE a host hands back: its length is a length, and its
				// element range is what the declaration says.
				if el := core.ArrayElem(p.Results[i]); el != "" {
					fs[i].hasLn, fs[i].ln = true, rangeIV(0, a.maxLen)
					fs[i].hasEl, fs[i].el = true, ivTop
					if lo, hi, ok := core.IntRange(el); ok {
						fs[i].el = rangeIV(lo, hi)
					}
				}
			}
		}
		a.set(res, fs)
		return
	}
	switch p.Kind {
	case "len":
		out := fact{v: rangeIV(0, a.maxLen)}
		if len(args) == 1 && args[0].hasLn {
			out.v = meet(args[0].ln, out.v)
		}
		a.f[res[0]] = out
		return
	case "table-set":
		if len(args) == 3 {
			out := args[0]
			if out.hasEl {
				out.el = join(out.el, args[2].v)
			}
			a.f[res[0]] = out
			return
		}
	case "table-alloc":
		if len(args) == 1 {
			a.f[res[0]] = args[0]
			return
		}
	case "array":
		out := fact{v: ivTop, hasLn: true, ln: exactIV(int64(len(args))), hasEl: true, el: ivBot}
		for _, x := range args {
			out.el = join(out.el, x.v)
		}
		a.f[res[0]] = out
		return
	}
	if lo, hi, ok := core.IntRange(p.Result); ok {
		a.f[res[0]] = fact{v: rangeIV(lo, hi)}
		return
	}
	if p.Result != "int" && p.Result != "" {
		a.f[res[0]] = fact{v: ivTop}
		return
	}
	var out iv
	counted := false
	switch emit.ArithOp(op.Name, len(args)) {
	case "add":
		out, counted = addIV(args[0].v, args[1].v), true
	case "sub":
		out, counted = subIV(args[0].v, args[1].v), true
	case "mul":
		out, counted = mulIV(args[0].v, args[1].v), true
	case "neg":
		out, counted = negIV(args[0].v), true
	case "div":
		out = divIV(args[0].v, args[1].v)
	case "rem":
		out = remIV(args[0].v, args[1].v)
	default:
		out = ivTop
	}
	a.f[res[0]] = fact{v: out}
	if counted && a.counting {
		a.Ops++
		if out.fits(a.word) {
			a.Proven++
		} else {
			a.Unproven = append(a.Unproven, fmt.Sprintf("%s %s of %s", op.Name, show(out), showArgs(args)))
		}
	}
}

func show(x iv) string {
	if x.bot {
		return "⊥"
	}
	lo, hi := fmt.Sprint(x.lo), fmt.Sprint(x.hi)
	if x.nlo {
		lo = "-inf"
	}
	if x.phi {
		hi = "+inf"
	}
	return "[" + lo + ", " + hi + "]"
}

func showArgs(fs []fact) string {
	s := ""
	for i, f := range fs {
		if i > 0 {
			s += " "
		}
		s += show(f.v)
	}
	return s
}
