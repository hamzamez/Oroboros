package plan

import (
	"oroboros/core"
	"oroboros/ir"
)

// THE DOUBLE WORD'S HIGH DIGITS (ADR 0103; ir/golang/double.go, ir/java/double.go).
// A host without an integer of 128 bits holds an element of E as two digits,
// (h, l), x = h·2⁶⁴ + l mod 2¹²⁸. Which uses read h is a fact of the
// operations, the same on every such host: an intermediate's + − · and neg
// read their operands' h exactly where their own result's is read; a
// read-back by + − · neg or & into the word reads only l (ℤ → ℤ/2⁶⁴ is a ring
// homomorphism, and a mask in U has h = 0); a shift, a quotient, a remainder,
// a mod and a comparison read h. DoubleHighs is the least fixed point of that
// flow, the values whose high digit is printed: a host that refuses an unused
// variable (Go) prints no other, and none is any use's.

// ReadsDouble says s computes an intermediate in E or reads one back.
func (p *Plan) ReadsDouble(s *ir.Stmt, dbl map[ir.V]bool) bool {
	if s.Mode == ir.MDouble {
		return true
	}
	for _, a := range s.Args {
		if dbl[a] || dbl[p.Res(a)] {
			return true
		}
	}
	return false
}

// joinDigits is where a join of the double word carries its digits (ADR
// 0153): a loop's variable or result in E, and each of its inputs held in E,
// keep both, either being what the next iteration or the exit may read. The
// seed of DoubleHighs and DoubleLows alike, by Res.
func (p *Plan) joinDigits(dbl map[ir.V]bool) map[ir.V]bool {
	need := map[ir.V]bool{}
	isDbl := func(v ir.V) bool { return dbl[v] || dbl[p.Res(v)] }
	mark := func(into []ir.V, args []ir.V) {
		for j, a := range args {
			if j < len(into) && isDbl(into[j]) {
				need[p.Res(into[j])] = true
				if isDbl(a) {
					need[p.Res(a)] = true
				}
			}
		}
	}
	p.F.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != ir.OLoop || len(s.Sub) != 1 {
				continue
			}
			body := s.Sub[0]
			mark(body.Params, s.Args)
			for _, args := range ir.OwnExits(body, ir.TContinue) {
				mark(body.Params, args)
			}
			for _, args := range ir.OwnExits(body, ir.TBreak) {
				mark(s.Res, args)
			}
		}
	})
	return need
}

// DoubleHighs is the set of double-word values, by Res, whose high digit a use
// reads.
func (p *Plan) DoubleHighs(dbl map[ir.V]bool) map[ir.V]bool {
	need := p.joinDigits(dbl)
	var stmts []*ir.Stmt
	p.F.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; p.ReadsDouble(s, dbl) {
				stmts = append(stmts, s)
			}
		}
	})
	ring := func(op ir.Op) bool { return op == ir.OAdd || op == ir.OSub || op == ir.ONeg || op == ir.OMul }
	for changed := true; changed; {
		changed = false
		for _, s := range stmts {
			var reads bool
			switch {
			case s.Mode == ir.MDouble && ring(s.Op):
				reads = need[p.Res(s.Res[0])]
			case s.Mode != ir.MDouble && (ring(s.Op) || s.Op == ir.OAnd):
				reads = false
			default: // a shift, a quotient, a remainder, a mod, a comparison
				reads = true
			}
			if !reads {
				continue
			}
			for _, a := range s.Args {
				if k := p.Res(a); (dbl[a] || dbl[k]) && !need[k] {
					need[k] = true
					changed = true
				}
			}
		}
	}
	return need
}

// DoubleLows is the set of double-word values, by Res, whose low digit a use
// reads: every use but a shift by a constant of 64 or more, which reads the
// high digit alone; an intermediate's + − · neg read their operands' low
// digits wherever either of their own digits is read (a carry, a borrow and a
// product's high digit are the low digits'); a shift's operand's, where its
// result's low digit is read and its count may be below 64. The dual of
// DoubleHighs, so that a host refusing an unused variable is given none.
func (p *Plan) DoubleLows(dbl, highs map[ir.V]bool) map[ir.V]bool {
	need := p.joinDigits(dbl)
	var stmts []*ir.Stmt
	p.F.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; p.ReadsDouble(s, dbl) {
				stmts = append(stmts, s)
			}
		}
	})
	isDbl := func(v ir.V) bool { return dbl[v] || dbl[p.Res(v)] }
	shiftAtLeast64 := func(s *ir.Stmt) bool {
		if s.Op != ir.OShr || len(s.Args) != 2 {
			return false
		}
		d := p.Const[p.Res(s.Args[1])]
		return d != nil && d.Kind == core.KInt && d.Int >= 64
	}
	for changed := true; changed; {
		changed = false
		for _, s := range stmts {
			res := p.Res(s.Res[0])
			var reads bool
			switch {
			case s.Mode != ir.MDouble:
				// a read-back: its operand's low digit, but by a shift of 64 or more
				reads = !shiftAtLeast64(s)
			case s.Op == ir.OShr:
				// x >> c: the result's high digit is xₕ's alone, its low digit
				// xₗ's and xₕ's for c < 64 and xₕ's alone for c ≥ 64
				reads = need[res] && !shiftAtLeast64(s)
			default: // an intermediate: wherever either of its digits is read
				reads = need[res] || highs[res]
			}
			if !reads {
				continue
			}
			for _, a := range s.Args {
				if k := p.Res(a); isDbl(a) && !need[k] {
					need[k] = true
					changed = true
				}
			}
		}
	}
	return need
}
