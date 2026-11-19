package emit

import (
	"testing"

	"oroboros/core"
)

// A CLOSED DISEQUALITY IS DECIDED IN ℤ (u128-2026-11-19): `x % 2^128` owes
// 2^128 ≠ 0, a literal past int64, in J's spelling `(if (= d 0) false true)`
// and a host's `ne`. Each is decided, true where the two differ and false where
// they are equal (so a false goal is refused, not passed). The fault leaves it to
// the linear fragment, which reads no constant past int64, and it is undecided.
func TestAClosedDisequalityIsDecidedInZ(t *testing.T) {
	two64 := "(+ (* 18446744073709551 1000) 616)"
	two128 := "(* " + two64 + " " + two64 + ")"
	for _, c := range []struct {
		term string
		want bool
	}{
		{"(if (= " + two128 + " 0) false true)", true},
		{"(!= " + two128 + " 0)", true},
		{"(if (= (- " + two128 + " " + two128 + ") 0) false true)", false},
	} {
		tm, err := core.ParseTerm(c.term)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := closedIntComparison(tm)
		if !ok || v != c.want {
			t.Errorf("%s: decided %v, %v; want %v, true", c.term, v, ok, c.want)
		}
		PlantClosedDisequalityUndecided = true
		_, ok = closedIntComparison(tm)
		PlantClosedDisequalityUndecided = false
		if ok {
			t.Errorf("%s: decided with the fault planted, so the rule is not what decides it", c.term)
		}
	}
}
