package emit

import (
	"math/big"

	"oroboros/core"
)

// EnsuresBounds reads a postcondition in the constant-bounded fragment as the
// interval it denotes for the result: `result <= K`, `K < result` and the
// rest are half-lines, and a conjunction is their intersection. lo and hi are
// nil for an open end. decided is false for anything outside the fragment (a
// relational claim such as `result > i`, or a disjunction), which an interval
// cannot settle.
//
// It is syntax, not analysis: the IR's decision compares the body's result
// fact with this set (ir.CheckEnsures, postconditions.md §2).
func EnsuresBounds(tgt *Target, q *core.Term) (lo, hi *big.Int, decided bool) {
	if c, ok := connective(tgt, q); ok && c.Op == "and" && len(c.Args) == 2 {
		alo, ahi, adec := EnsuresBounds(tgt, c.Args[0])
		blo, bhi, bdec := EnsuresBounds(tgt, c.Args[1])
		if !adec || !bdec {
			return nil, nil, false
		}
		return maxBig(alo, blo), minBig(ahi, bhi), true
	}
	if q.Kind != core.KApp || q.Op().Kind != core.KName || len(q.Args()) != 2 {
		return nil, nil, false
	}
	lhs, rhs, name := q.Args()[0], q.Args()[1], q.Op().Name
	isResult := func(x *core.Term) bool { return x.Kind == core.KName && x.Name == core.ResultName }
	k := func(x *core.Term) (*big.Int, bool) { return closedInt(x) }
	one := big.NewInt(1)
	switch {
	case isResult(lhs):
		if c, ok := k(rhs); ok {
			switch {
			case isOp(name, "le"):
				return nil, c, true
			case isOp(name, "lt"):
				return nil, new(big.Int).Sub(c, one), true
			case isOp(name, "ge"):
				return c, nil, true
			case isOp(name, "gt"):
				return new(big.Int).Add(c, one), nil, true
			}
		}
	case isResult(rhs):
		if c, ok := k(lhs); ok {
			switch {
			case isOp(name, "le"): // K <= result
				return c, nil, true
			case isOp(name, "lt"):
				return new(big.Int).Add(c, one), nil, true
			case isOp(name, "ge"): // K >= result
				return nil, c, true
			case isOp(name, "gt"):
				return nil, new(big.Int).Sub(c, one), true
			}
		}
	}
	return nil, nil, false
}

func maxBig(a, b *big.Int) *big.Int {
	switch {
	case a == nil:
		return b
	case b == nil || a.Cmp(b) >= 0:
		return a
	}
	return b
}

func minBig(a, b *big.Int) *big.Int {
	switch {
	case a == nil:
		return b
	case b == nil || a.Cmp(b) <= 0:
		return a
	}
	return b
}

// closedInt is the one integer a CLOSED integer term denotes: a literal, or
// `+`, `−`, `·` and negation of closed terms, evaluated exactly. It is how the
// reader writes a literal past int64 (as arithmetic over int64 literals), so a
// bound like 2^200 is a constant too.
func closedInt(x *core.Term) (*big.Int, bool) {
	switch {
	case x.Kind == core.KInt:
		return big.NewInt(x.Int), true
	case x.Kind != core.KApp || x.Op().Kind != core.KName:
		return nil, false
	}
	var vs []*big.Int
	for _, a := range x.Args() {
		v, ok := closedInt(a)
		if !ok {
			return nil, false
		}
		vs = append(vs, v)
	}
	switch op := arithOp(x.Op().Name, len(vs)); {
	case op == "add" && len(vs) == 2:
		return new(big.Int).Add(vs[0], vs[1]), true
	case op == "sub" && len(vs) == 2:
		return new(big.Int).Sub(vs[0], vs[1]), true
	case op == "mul" && len(vs) == 2:
		return new(big.Int).Mul(vs[0], vs[1]), true
	case op == "neg" && len(vs) == 1:
		return new(big.Int).Neg(vs[0]), true
	}
	return nil, false
}
