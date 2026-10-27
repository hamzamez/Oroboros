package emit

import (
	"math/big"

	"oroboros/core"
)

// closedIntComparison decides a comparison of two closed integer terms, built
// from literals by the language's +, − and ·, exactly in ℤ.
func closedIntComparison(t *core.Term) (bool, bool) {
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
