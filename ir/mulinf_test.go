package ir

import (
	"math/big"
	"math/rand"
	"testing"
)

// memberIV reports v ∈ x over ℤ: an infinite end bounds nothing, and a
// finite one is compared exactly.
func memberIV(x iv, v *big.Int) bool {
	if x.bot {
		return false
	}
	if !x.nlo && v.Cmp(x.lo.big()) < 0 {
		return false
	}
	if !x.phi && v.Cmp(x.hi.big()) > 0 {
		return false
	}
	return true
}

// THE PRODUCT OF UNBOUNDED INTERVALS IS SOUND AND KEEPS ITS SIGN (mulInf): for
// intervals of ℤ with an infinite end, every product of elements lies in the
// computed interval, checked on elements sampled up to 2²⁰⁰ past each finite
// end, where an infinite end lets them go; and the product keeps what the
// signs decide, [1, +∞)·[2, 30] = [2, +∞), which a factorial's claim needs.
// Planted fault: an infinite corner dropped, which the sampling sees.
func TestTheProductOfUnboundedIntervalsIsSound(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	sample := func(x iv) *big.Int {
		huge := new(big.Int).Lsh(big.NewInt(1), uint(r.Intn(200)))
		switch {
		case x.nlo && x.phi:
			v := new(big.Int).Rand(r, huge)
			if r.Intn(2) == 0 {
				v.Neg(v)
			}
			return v
		case x.nlo:
			return new(big.Int).Sub(x.hi.big(), new(big.Int).Rand(r, huge))
		case x.phi:
			return new(big.Int).Add(x.lo.big(), new(big.Int).Rand(r, huge))
		}
		span := new(big.Int).Sub(x.hi.big(), x.lo.big())
		return new(big.Int).Add(x.lo.big(), new(big.Int).Rand(r, span.Add(span, big.NewInt(1))))
	}
	shapes := []iv{
		{lo: ei(1), phi: true}, {lo: ei(0), phi: true}, {lo: ei(-3), phi: true},
		{hi: ei(-1), nlo: true}, {hi: ei(0), nlo: true}, {hi: ei(5), nlo: true},
		ivTop, rangeIV(2, 30), rangeIV(-4, 7), rangeIV(0, 0), rangeIV(-9, -2),
	}
	check := func() int {
		bad := 0
		for _, a := range shapes {
			for _, b := range shapes {
				if a.finite() && b.finite() {
					continue // mulIV's own case
				}
				m := mulIV(a, b)
				for k := 0; k < 200; k++ {
					x, y := sample(a), sample(b)
					if !memberIV(m, new(big.Int).Mul(x, y)) {
						bad++
					}
				}
			}
		}
		return bad
	}
	if bad := check(); bad > 0 {
		t.Fatalf("%d products outside the computed interval", bad)
	}
	got := mulIV(iv{lo: ei(1), phi: true}, rangeIV(2, 30))
	if got.nlo || got.lo != ei(2) || !got.phi {
		t.Errorf("[1, +∞)·[2, 30] is %s, want [2, +inf]", show(got))
	}
	if got := mulIV(iv{lo: ei(0), phi: true}, rangeIV(0, 0)); got != exactIV(0) {
		t.Errorf("[0, +∞)·[0, 0] is %s, want [0, 0]", show(got))
	}
	plantMulInfFault = true
	defer func() { plantMulInfFault = false }()
	if check() == 0 {
		t.Errorf("the planted fault (an infinite corner dropped) went unseen")
	}
}
