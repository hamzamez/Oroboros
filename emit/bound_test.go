package emit

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// THE ENDPOINT IS ℤ ON (−2^126, 2^126), checked against math/big as the oracle
// on values chosen where two's complement goes wrong: either side of every
// word boundary the analysis will now meet, and the saturation edge.

func edgeBigs() []*big.Int {
	one := big.NewInt(1)
	pow := func(k uint) *big.Int { return new(big.Int).Lsh(one, k) }
	var out []*big.Int
	for _, k := range []uint{0, 1, 31, 32, 52, 53, 62, 63, 64, 65, 100, 125} {
		p := pow(k)
		for _, d := range []int64{-1, 0, 1} {
			v := new(big.Int).Add(p, big.NewInt(d))
			out = append(out, v, new(big.Int).Neg(v))
		}
	}
	out = append(out, big.NewInt(0), new(big.Int).Sub(pow(126), one), new(big.Int).Neg(new(big.Int).Sub(pow(126), one)))
	return out
}

func randBig(r *rand.Rand) *big.Int {
	k := uint(r.Intn(127))
	v := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), k+1))
	if r.Intn(2) == 0 {
		v.Neg(v)
	}
	return v
}

func inRange(v *big.Int) bool { return v.CmpAbs(bigSat) < 0 }

func TestBoundAgreesWithBigInt(t *testing.T) {
	r := rand.New(rand.NewSource(26))
	vals := edgeBigs()
	for i := 0; i < 400; i++ {
		vals = append(vals, randBig(r))
	}
	var ins []*big.Int
	for _, v := range vals {
		if inRange(v) {
			ins = append(ins, v)
		}
	}
	for _, x := range ins {
		a, ok := fromBig(x)
		if !ok || a.big().Cmp(x) != 0 || a.String() != x.String() {
			t.Fatalf("round trip %s: got %s ok=%v", x, a, ok)
		}
		if a.neg().big().Cmp(new(big.Int).Neg(x)) != 0 {
			t.Errorf("neg %s", x)
		}
		if v, ok := a.i64(); ok != x.IsInt64() || (ok && v != x.Int64()) {
			t.Errorf("i64 %s: %d %v", x, v, ok)
		}
		for _, k := range []uint{0, 1, 7, 63, 64, 65, 100, 127, 130} {
			want := new(big.Int).Rsh(x, k) // big's Rsh on a negative is floor, as shr is
			if a.shr(k).big().Cmp(want) != 0 {
				t.Errorf("%s >> %d: got %s want %s", x, k, a.shr(k), want)
			}
		}
	}
	for i, x := range ins {
		a, _ := fromBig(x)
		for _, y := range ins[i%7:] {
			b, _ := fromBig(y)
			if got, want := a.cmp(b), x.Cmp(y); got != want {
				t.Fatalf("cmp %s %s: %d want %d", x, y, got, want)
			}
			check := func(op string, got bnd, gotOK bool, want *big.Int) {
				t.Helper()
				if gotOK != inRange(want) || (gotOK && got.big().Cmp(want) != 0) {
					t.Fatalf("%s %s %s: got %s ok=%v, want %s", x, op, y, got, gotOK, want)
				}
			}
			s, ok := a.add(b)
			check("+", s, ok, new(big.Int).Add(x, y))
			d, ok := a.sub(b)
			check("-", d, ok, new(big.Int).Sub(x, y))
			m, ok := a.mul(b)
			check("*", m, ok, new(big.Int).Mul(x, y))
			if y.Sign() != 0 {
				check("/", a.quo(b), true, new(big.Int).Quo(x, y))
			}
		}
	}
}

// The corners a random search is unlikely to hit exactly.
func TestBoundCorners(t *testing.T) {
	min64 := bi(math.MinInt64)
	if q := min64.quo(bi(-1)); q.String() != "9223372036854775808" {
		t.Errorf("MinInt64 / -1 = %s", q)
	}
	if m, ok := min64.mul(min64); ok {
		t.Errorf("(−2^63)^2 = 2^126 is not an endpoint, got %s", m)
	}
	if m, ok := min64.mul(bi(math.MaxInt64)); !ok || m.big().Cmp(new(big.Int).Mul(big.NewInt(math.MinInt64), big.NewInt(math.MaxInt64))) != 0 {
		t.Errorf("−2^63 · (2^63−1) = %s ok=%v", m, ok)
	}
	if bu(math.MaxUint64).String() != "18446744073709551615" {
		t.Errorf("2^64−1 = %s", bu(math.MaxUint64))
	}
	if _, ok := bu(math.MaxUint64).i64(); ok {
		t.Error("2^64−1 is not an int64")
	}
}
