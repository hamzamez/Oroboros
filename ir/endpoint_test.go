package ir

import (
	"math/big"
	"math/rand"
	"testing"
)

// THE END IS ℤ ON E = (−2¹²⁶, 2¹²⁶), checked against math/big on values chosen
// where two's complement goes wrong: either side of every word boundary the
// analysis meets (2³¹, 2⁵³, 2⁶³, 2⁶⁴), and E's edge.
func endpointValues() []*big.Int {
	one := big.NewInt(1)
	pow := func(k uint) *big.Int { return new(big.Int).Lsh(one, k) }
	var out []*big.Int
	for _, k := range []uint{0, 1, 31, 32, 52, 53, 62, 63, 64, 65, 100, 125} {
		for _, d := range []int64{-1, 0, 1} {
			v := new(big.Int).Add(pow(k), big.NewInt(d))
			out = append(out, v, new(big.Int).Neg(v))
		}
	}
	out = append(out, big.NewInt(0), new(big.Int).Sub(pow(126), one), new(big.Int).Neg(new(big.Int).Sub(pow(126), one)))
	r := rand.New(rand.NewSource(26))
	for i := 0; i < 200; i++ {
		v := new(big.Int).Rand(r, pow(uint(r.Intn(126))+1))
		if r.Intn(2) == 0 {
			v.Neg(v)
		}
		out = append(out, v)
	}
	return out
}

func inE(v *big.Int) bool { return v.CmpAbs(bigSat) < 0 }

func TestTheEndIsTheIntegers(t *testing.T) {
	var ins []*big.Int
	for _, v := range endpointValues() {
		if inE(v) {
			ins = append(ins, v)
		}
	}
	for _, x := range ins {
		a, ok := fromBig(x)
		if !ok || a.big().Cmp(x) != 0 {
			t.Fatalf("round trip %s: got %s ok=%v", x, a, ok)
		}
		if a.neg().big().Cmp(new(big.Int).Neg(x)) != 0 {
			t.Fatalf("neg %s", x)
		}
		if x.Sign() >= 0 {
			if isqrtE(a).big().Cmp(new(big.Int).Sqrt(x)) != 0 {
				t.Fatalf("isqrt %s", x)
			}
		}
	}
	for i, x := range ins {
		a, _ := fromBig(x)
		for _, y := range ins[i%5:] {
			b, _ := fromBig(y)
			if a.cmp(b) != x.Cmp(y) {
				t.Fatalf("cmp %s %s", x, y)
			}
			check := func(op string, got ep, ok bool, want *big.Int) {
				t.Helper()
				if ok != inE(want) || (ok && got.big().Cmp(want) != 0) {
					t.Fatalf("%s %s %s: got %s ok=%v, want %s", x, op, y, got, ok, want)
				}
			}
			s, ok := a.add(b)
			check("+", s, ok, new(big.Int).Add(x, y))
			d, ok := a.sub(b)
			check("-", d, ok, new(big.Int).Sub(x, y))
			p, ok := a.mul(b)
			check("·", p, ok, new(big.Int).Mul(x, y))
			if y.Sign() != 0 {
				check("quo", a.quo(b), true, new(big.Int).Quo(x, y))
				check("floor", floorDivE(a, b), true, floorBig(x, y))
				c := new(big.Int).Neg(floorBig(new(big.Int).Neg(x), y))
				check("ceil", ceilDivE(a, b), true, c)
			}
		}
	}
}
