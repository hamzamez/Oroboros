package emit

import (
	"math/big"

	"oroboros/core"
)

// closedIntComparison decides a comparison of two closed integer terms, built
// from literals by the language's +, − and ·, exactly in ℤ.
//
// A DISEQUALITY TOO, a ≠ b ⟺ ¬(a = b), in both its spellings: a host's `ne`
// and J's `a != b`, D's `(if (= a b) false true)`. A remainder by 2^128 owes
// 2^128 ≠ 0, a literal past int64 that the word's evaluation does not fold
// and the linear fragment cannot read (u128-2026-11-19).
func closedIntComparison(t *core.Term) (bool, bool) {
	if !PlantClosedDisequalityUndecided {
		if a, b, ok := closedNe(t); ok {
			return a.Cmp(b) != 0, true
		}
	}
	if t.Kind != core.KApp || len(t.Kids) != 3 || t.Kids[0].Kind != core.KName {
		return false, false
	}
	a, ok1 := closedZ(t.Kids[1])
	b, ok2 := closedZ(t.Kids[2])
	if !ok1 || !ok2 {
		return false, false
	}
	c := a.Cmp(b)
	switch t.Kids[0].Name {
	case "<":
		return c < 0, true
	case "<=":
		return c <= 0, true
	case ">":
		return c > 0, true
	case ">=":
		return c >= 0, true
	case "=":
		return c == 0, true
	}
	return false, false
}

// PlantClosedDisequalityUndecided leaves a closed disequality to the linear
// fragment again (TestAClosedDisequalityIsDecidedInZ).
var PlantClosedDisequalityUndecided bool

// closedNe reads a ≠ b over closed integer terms, either spelling.
func closedNe(t *core.Term) (*big.Int, *big.Int, bool) {
	if t.Kind != core.KApp || t.Kids[0].Kind != core.KName {
		return nil, nil, false
	}
	var l, r *core.Term
	switch {
	case len(t.Kids) == 3 && isOp(t.Kids[0].Name, "ne"):
		l, r = t.Kids[1], t.Kids[2]
	case len(t.Kids) == 4 && t.Kids[0].Name == "if":
		c, th, el := t.Kids[1], t.Kids[2], t.Kids[3]
		if th.Kind != core.KBool || th.IsTrue() || el.Kind != core.KBool || !el.IsTrue() ||
			c.Kind != core.KApp || len(c.Kids) != 3 || c.Kids[0].Kind != core.KName || !isOp(c.Kids[0].Name, "eq") {
			return nil, nil, false
		}
		l, r = c.Kids[1], c.Kids[2]
	default:
		return nil, nil, false
	}
	a, ok1 := closedZ(l)
	b, ok2 := closedZ(r)
	if !ok1 || !ok2 {
		return nil, nil, false
	}
	return a, b, true
}

// closedZ is the value of a closed integer term of literals and +, −, ·.
func closedZ(t *core.Term) (*big.Int, bool) {
	switch {
	case t.Kind == core.KInt:
		return big.NewInt(t.Int), true
	case t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName:
		a, ok1 := closedZ(t.Kids[1])
		b, ok2 := closedZ(t.Kids[2])
		if !ok1 || !ok2 {
			return nil, false
		}
		switch t.Kids[0].Name {
		case "+":
			return a.Add(a, b), true
		case "-":
			return a.Sub(a, b), true
		case "*":
			return a.Mul(a, b), true
		}
	case t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == "-":
		a, ok := closedZ(t.Kids[1])
		if ok {
			return a.Neg(a), true
		}
	}
	return nil, false
}
