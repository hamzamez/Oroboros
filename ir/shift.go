package ir

import (
	"oroboros/core"
	"oroboros/emit"
)

// SHIFTS AND MASKS FOR DIVISIONS BY 2ᵏ (shiftdiv-2026-09-03), a rewrite by a
// law under a premise the decision's facts discharge:
//
//	x ∈ [0, 2ʷ) ∩ W, c = 2ᵏ, k ≥ 1:   x div c = x >> k   and   x rem c = x & (c − 1)
//
// Truncating division is the floor on x ≥ 0, and ⌊x / 2ᵏ⌋ drops x's low k
// bits, which the remainder is. w is the target's `shift-width`, the range on
// which its `>>` and `&` are exact (V8 coerces both operands to int32, so it
// declares 31), and the dividend must be a word.
//
// The facts stay valid: ⌊x/2ᵏ⌋ and x >> k have one interval (divIV's corners
// are shrIV's), and so do x rem 2ᵏ and x & (2ᵏ − 1) (remIV, andIV), so the
// step to IR_P reuses them. The divisor's literal becomes k or 2ᵏ − 1 in place
// when nothing else reads it, which keeps the value numbering the term route
// had; otherwise a new literal is added. It returns how many were rewritten.
func SelectShifts(tg *emit.Target, f *Func) int {
	if tg.ShiftWidth == 0 {
		return 0
	}
	shr, and, ok := tg.ShiftNames()
	if !ok {
		return 0
	}
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
		f.facts = fs
	}
	hi := ei(tg.Word.Hi)
	if tg.ShiftWidth < 63 {
		hi = minE(hi, ei(1<<tg.ShiftWidth-1))
	}
	reads := map[V]int{}
	f.Walk(func(r *Region) {
		for _, a := range r.Args {
			reads[a]++
		}
		if r.T == TBranch {
			reads[r.Cond]++
		}
		for _, pi := range r.Pis {
			reads[pi.Of]++
			reads[pi.Other]++
		}
		for i := range r.Stmts {
			for _, a := range r.Stmts[i].Args {
				reads[a]++
			}
		}
	})
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	type plan struct {
		s       *Stmt
		lit     int64
		name    string
		inPlace bool
		r       *Region
		index   int
	}
	var plans []plan
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if (s.Op != ODiv && s.Op != ORem) || len(s.Args) != 2 || len(s.Res) != 1 {
				continue
			}
			d := def[s.Args[1]]
			if d == nil || d.Op != OConst || d.Lit.Kind != core.KInt {
				continue
			}
			c := d.Lit.Int
			if c < 2 || c&(c-1) != 0 || !fs[s.Args[0]].v.within(ep{}, hi) {
				continue
			}
			k := int64(0)
			for b := c; b > 1; b >>= 1 {
				k++
			}
			p := plan{s: s, lit: k, name: shr, inPlace: reads[s.Args[1]] == 1, r: r, index: i}
			if s.Op == ORem {
				p.lit, p.name = c-1, and
			}
			plans = append(plans, p)
		}
	})
	// In place first, while every pointer is into the arrays it was taken
	// from: the divisor's literal becomes k or 2ᵏ − 1, and its fact with it.
	shared := false
	for _, p := range plans {
		if p.inPlace {
			d := def[p.s.Args[1]]
			d.Lit = core.Int(p.lit)
			fs[p.s.Args[1]] = fact{v: exactIV(p.lit)}
			p.s.Op, p.s.Mode, p.s.Name = OCall, MNone, p.name
		} else {
			shared = true
		}
	}
	if shared {
		// A literal something else reads keeps its value; the rewrite gets its
		// own, inserted before it (latest first, so earlier indices hold), and
		// the function is renumbered, which invalidates the facts.
		for k := len(plans) - 1; k >= 0; k-- {
			p := plans[k]
			if p.inPlace {
				continue
			}
			v := V(len(f.Types))
			f.Types = append(f.Types, "int")
			st := p.r.Stmts[p.index]
			st.Args = []V{st.Args[0], v}
			st.Op, st.Mode, st.Name = OCall, MNone, p.name
			p.r.Stmts[p.index] = st
			p.r.Stmts = append(p.r.Stmts[:p.index], append([]Stmt{{Op: OConst, Lit: core.Int(p.lit), Res: []V{v}}}, p.r.Stmts[p.index:]...)...)
		}
		Canonicalize(f)
		f.facts = nil
	}
	return len(plans)
}
