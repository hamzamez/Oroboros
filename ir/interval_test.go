package ir

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// THE SOUNDNESS TABLE OF THE INTERVAL DOMAIN (spec §7): one row per abstract
// operation, each checked against the concrete operation on ℤ. Local soundness
// is the property that makes the whole analysis sound by structural induction
// (Cousot and Cousot 1977):
//
//	for every x ∈ γ(a), y ∈ γ(b):   f(x, y) ∈ γ(f#(a, b))
//
// The concrete side is computed in big.Int, so a result past int64 is a true
// integer the abstract interval must contain (by an infinite end), never a
// wrapped one. The check is exhaustive over every interval inside [−6, 6],
// closed and half-open, at the int64 boundaries, and random elsewhere.

// member reports x ∈ γ(a).
func member(x *big.Int, a iv) bool {
	if a.bot {
		return false
	}
	if !a.nlo && x.Cmp(big.NewInt(a.lo)) < 0 {
		return false
	}
	if !a.phi && x.Cmp(big.NewInt(a.hi)) > 0 {
		return false
	}
	return true
}

// samples are concrete members of a: its ends, a few inside, and far values
// on an infinite side.
func samples(a iv) []int64 {
	var out []int64
	if a.bot {
		return nil
	}
	lo, hi := a.lo, a.hi
	if a.nlo {
		lo = math.MinInt64
		out = append(out, math.MinInt64, -1<<40, -7)
	}
	if a.phi {
		hi = math.MaxInt64
		out = append(out, math.MaxInt64, 1<<40, 7)
	}
	out = append(out, lo, hi)
	if !a.nlo && !a.phi {
		// count, not compare: x++ past MaxInt64 would wrap to a non-member
		for k := int64(0); k < 20 && lo+k <= hi && lo+k >= lo; k++ {
			out = append(out, lo+k)
		}
	}
	for _, x := range []int64{0, 1, -1} {
		if (a.nlo || x >= a.lo) && (a.phi || x <= a.hi) {
			out = append(out, x)
		}
	}
	return out
}

// domains: every closed interval inside [−6, 6], each with its half-open
// variants, the boundary intervals, and ⊤.
func domains() []iv {
	var out []iv
	for lo := int64(-6); lo <= 6; lo++ {
		for hi := lo; hi <= 6; hi++ {
			out = append(out, rangeIV(lo, hi))
		}
		out = append(out, iv{lo: lo, phi: true}, iv{hi: lo, nlo: true})
	}
	out = append(out, ivTop,
		rangeIV(math.MaxInt64-3, math.MaxInt64), rangeIV(math.MinInt64, math.MinInt64+3),
		rangeIV(-1<<62, 1<<62), rangeIV(1<<32, 1<<33), rangeIV(-(1<<33), -(1<<32)))
	return out
}

type binRow struct {
	name string
	abs  func(a, b iv) iv
	con  func(x, y *big.Int) (*big.Int, bool) // false: undefined (division by zero)
}

var binRows = []binRow{
	{"add", addIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Add(x, y), true }},
	{"sub", subIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Sub(x, y), true }},
	{"mul", mulIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Mul(x, y), true }},
	{"div", divIV, func(x, y *big.Int) (*big.Int, bool) {
		if y.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Quo(x, y), true // truncating, integers.md §3
	}},
	{"rem", remIV, func(x, y *big.Int) (*big.Int, bool) {
		if y.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Rem(x, y), true // the dividend's sign
	}},
}

func checkBin(t *testing.T, r binRow, a, b iv) {
	out := r.abs(a, b)
	for _, x := range samples(a) {
		for _, y := range samples(b) {
			bx, by := big.NewInt(x), big.NewInt(y)
			c, ok := r.con(bx, by)
			if !ok {
				continue
			}
			if !member(c, out) {
				t.Fatalf("%s: %d ∈ %s, %d ∈ %s gives %s, not in %s", r.name, x, show(a), y, show(b), c, show(out))
			}
		}
	}
}

func TestIntervalSoundnessExhaustive(t *testing.T) {
	ds := domains()
	for _, r := range binRows {
		for _, a := range ds {
			for _, b := range ds {
				checkBin(t, r, a, b)
			}
		}
	}
	for _, a := range ds {
		out := negIV(a)
		for _, x := range samples(a) {
			c := new(big.Int).Neg(big.NewInt(x))
			if !member(c, out) {
				t.Fatalf("neg: %d ∈ %s gives %s, not in %s", x, show(a), c, show(out))
			}
		}
	}
}

func TestIntervalSoundnessRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pick := func() iv {
		x, y := rng.Int63n(1<<20)-1<<19, rng.Int63n(1<<20)-1<<19
		if rng.Intn(4) == 0 {
			x, y = rng.Int63()-rng.Int63(), rng.Int63()-rng.Int63()
		}
		if x > y {
			x, y = y, x
		}
		return rangeIV(x, y)
	}
	for i := 0; i < 4000; i++ {
		a, b := pick(), pick()
		for _, r := range binRows {
			checkBin(t, r, a, b)
		}
	}
}

// TestNarrowIsSound: a π's fact (Theorem E). For x ∈ a and o ∈ b with
// `x rel o`, x ∈ narrow(a, rel, b).
func TestNarrowIsSound(t *testing.T) {
	rels := map[string]func(x, o int64) bool{
		"lt": func(x, o int64) bool { return x < o }, "le": func(x, o int64) bool { return x <= o },
		"gt": func(x, o int64) bool { return x > o }, "ge": func(x, o int64) bool { return x >= o },
		"eq": func(x, o int64) bool { return x == o }, "ne": func(x, o int64) bool { return x != o },
	}
	ds := domains()
	for rel, holds := range rels {
		for _, a := range ds {
			for _, b := range ds {
				out := narrowIV(a, rel, b)
				for _, x := range samples(a) {
					for _, o := range samples(b) {
						if holds(x, o) && !member(big.NewInt(x), out) {
							t.Fatalf("narrow %s: %d ∈ %s with %d ∈ %s holds, but %d ∉ %s", rel, x, show(a), o, show(b), x, show(out))
						}
					}
				}
			}
		}
	}
}

// TestLatticeLaws: join contains both, meet contains what both contain, and
// widening contains both (it is an upper bound).
func TestLatticeLaws(t *testing.T) {
	ds := domains()
	for _, a := range ds {
		for _, b := range ds {
			j, m, w := joinIV(a, b), meetIV(a, b), widenIV(a, b)
			for _, x := range append(samples(a), samples(b)...) {
				bx := big.NewInt(x)
				inA, inB := member(bx, a), member(bx, b)
				if (inA || inB) && !member(bx, j) {
					t.Fatalf("join: %d ∈ %s ∪ %s but ∉ %s", x, show(a), show(b), show(j))
				}
				if inA && inB && !member(bx, m) {
					t.Fatalf("meet: %d ∈ %s ∩ %s but ∉ %s", x, show(a), show(b), show(m))
				}
				if (inA || inB) && !member(bx, w) {
					t.Fatalf("widen: %d ∈ %s ∪ %s but ∉ %s", x, show(a), show(b), show(w))
				}
			}
		}
	}
}

func show(a iv) string {
	if a.bot {
		return "⊥"
	}
	lo, hi := big.NewInt(a.lo).String(), big.NewInt(a.hi).String()
	if a.nlo {
		lo = "-∞"
	}
	if a.phi {
		hi = "+∞"
	}
	return "[" + lo + ", " + hi + "]"
}
