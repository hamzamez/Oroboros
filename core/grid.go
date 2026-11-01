package core

import (
	"fmt"
	"math/big"
)

// THE GRIDS A ROUNDING TARGETS (ADR 0086, numbers.md §2–§3).
//
// A grid G ⊂ ℚ is closed, discrete and LOCALLY UNIFORM: each x lies in a closed
// cell Ī(x) whose ends are in G and on which G ∩ Ī = u(x)·ℤ ∩ Ī, u(x) the
// spacing there.
//
//	ℤ                 u = 1, one cell
//	10⁻ᵏℤ             u = 10⁻ᵏ, one cell
//	F(p, emin, +∞)    the binade [2ᴱ, 2ᴱ⁺¹], E = max(⌊log₂|x|⌋, emin), and its
//	                  mirror, with u = 2^(E−p+1): its elements are m·u for
//	                  2ᵖ⁻¹ ≤ m ≤ 2ᵖ (m = 2ᵖ is 2ᴱ⁺¹), and below 2^emin, the
//	                  subnormals, m·u for 0 ≤ m ≤ 2ᵖ⁻¹, the same u
//
// Both of x's neighbours in G lie in Ī(x), so
//
//	r_G,μ(x) = u · r_ℤ,μ(x / u)
//
// for every mode μ, and a tie to the even neighbour is m even in the cell's
// generator, the canonical one of §3. One procedure for every grid: F64 is
// F(53, −1022, 1023), F32 F(24, −126, 127), and so on, and the set F(p, emin,
// emax) is G ∩ [−MAX, MAX] with MAX = (2 − 2^(1−p))·2^emax.
//
// The sets' definitions (floats-research §18.1) are not yet written in
// lib/num: F(p, …) is a replacement set, the image { m·2^(e−p+1) : m, e },
// which J does not build (ADR 0061). TestTheGridIsItsDefinition checks this
// procedure against that definition, enumerated, on F(3, −2, 2), and its p = 53
// instance against the host's binary64.
type Grid struct {
	// Kind is 'z' (ℤ), 'd' (10⁻ᴷℤ) or 'f' (F(P, Emin, Emax)).
	Kind          byte
	K             int
	P, Emin, Emax int
}

// The grids by name: ℤ, and binary64's.
var (
	GridZ   = Grid{Kind: 'z'}
	GridF64 = Grid{Kind: 'f', P: 53, Emin: -1022, Emax: 1023}
)

// GridD is 10⁻ᵏℤ, the decimal grid D(k, …) restricts.
func GridD(k int) Grid { return Grid{Kind: 'd', K: k} }

// plantSpacingFault, for TestTheGridIsItsDefinition: the cell's exponent taken
// as ⌈log₂|x|⌉, the binade above.
var plantSpacingFault = false

// Spacing is u(x), the spacing of the cell holding x.
func (g Grid) Spacing(x *big.Rat) *big.Rat {
	switch g.Kind {
	case 'z':
		return big.NewRat(1, 1)
	case 'd':
		return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(ten, big.NewInt(int64(g.K)), nil))
	}
	e := g.Emin
	if x.Sign() != 0 {
		if l := floorLog2(new(big.Rat).Abs(x)); l > e {
			e = l
		}
		if plantSpacingFault && !isPow2(new(big.Rat).Abs(x)) {
			e++
		}
	}
	return pow2(e - g.P + 1)
}

// Round is r_G,μ(x) onto the grid with no upper bound on its exponent: an
// element of G, and the caller decides whether it lies in the set's range.
func (g Grid) Round(x *big.Rat, mode string) *big.Rat {
	u := g.Spacing(x)
	m := RoundInt(new(big.Rat).Quo(x, u), mode)
	return new(big.Rat).Mul(new(big.Rat).SetInt(m), u)
}

// Max is the greatest element of the set F(p, emin, emax), (2 − 2^(1−p))·2^emax;
// nil for an unbounded grid.
func (g Grid) Max() *big.Rat {
	if g.Kind != 'f' {
		return nil
	}
	two := pow2(g.Emax + 1)
	return two.Sub(two, pow2(g.Emax-g.P+1))
}

// Contains reports x ∈ G, a fixed point of every rounding (r is a retraction).
func (g Grid) Contains(x *big.Rat) bool { return g.Round(x, "down").Cmp(x) == 0 }

// InRange reports x within the set's range, ±Max; every x for an unbounded grid.
func (g Grid) InRange(x *big.Rat) bool {
	m := g.Max()
	return m == nil || new(big.Rat).Abs(x).Cmp(m) <= 0
}

// RoundInt is r_ℤ,μ(x) on a rational (numbers.md §3): 'down ⌊x⌋, 'up ⌈x⌉,
// 'zero and 'away toward and from zero, 'nearest the nearer with a tie to the
// even integer, 'half-away the nearer with a tie away from zero.
func RoundInt(x *big.Rat, mode string) *big.Int {
	fl := new(big.Int).Div(x.Num(), x.Denom()) // Euclidean: the floor, the denominator being positive
	if x.IsInt() {
		return fl
	}
	c := new(big.Int).Add(fl, big.NewInt(1))
	switch mode {
	case "down":
		return fl
	case "up":
		return c
	case "zero":
		if x.Sign() >= 0 {
			return fl
		}
		return c
	case "away":
		if x.Sign() >= 0 {
			return c
		}
		return fl
	}
	switch new(big.Rat).Sub(x, new(big.Rat).SetInt(fl)).Cmp(big.NewRat(1, 2)) {
	case -1:
		return fl
	case 1:
		return c
	}
	if mode == "half-away" {
		if x.Sign() >= 0 {
			return c
		}
		return fl
	}
	if fl.Bit(0) == 0 {
		return fl
	}
	return c
}

// floorLog2 is ⌊log₂ q⌋ for q > 0.
func floorLog2(q *big.Rat) int {
	e := q.Num().BitLen() - q.Denom().BitLen()
	if new(big.Rat).Quo(q, pow2(e)).Cmp(big.NewRat(1, 1)) < 0 {
		e--
	}
	return e
}

func isPow2(q *big.Rat) bool {
	return q.Sign() > 0 && new(big.Rat).Quo(q, pow2(floorLog2(q))).Cmp(big.NewRat(1, 1)) == 0
}

// pow2 is 2ᵉ.
func pow2(e int) *big.Rat {
	if e >= 0 {
		return new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(e)))
	}
	return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(-e)))
}

// GridOfSet is the grid a set's canonical type restricts, and its interval's
// ends when it is one: a range (ℤ), F64 or F64 ∩ [lo, hi].
func GridOfSet(ty string) (g Grid, lo, hi *big.Rat, ok bool) {
	if ty == F64Name {
		m := GridF64.Max()
		return GridF64, new(big.Rat).Neg(m), m, true
	}
	if l, h, isF := F64Range(ty); isF {
		return GridF64, l.Rat(), h.Rat(), true
	}
	if l, h, isZ := IntRangeBig(ty); isZ {
		return GridZ, new(big.Rat).SetInt(l), new(big.Rat).SetInt(h), true
	}
	return Grid{}, nil, nil, false
}

// ratDec is a rational of N as its canonical form, an error when the
// denominator has a prime other than 2 and 5.
func ratDec(q *big.Rat) (*Dec, error) {
	den := new(big.Int).Set(q.Denom())
	k := 0
	p := big.NewInt(1)
	for new(big.Int).Rem(p, den).Sign() != 0 {
		p.Mul(p, ten)
		k++
		if k > 2000 {
			return nil, fmt.Errorf("%s is not an element of N", q.RatString())
		}
	}
	m := new(big.Int).Mul(q.Num(), new(big.Int).Quo(p, den))
	return NewDec(m, k), nil
}

// DecOfRat is a rational of N as an element, nil when it is not one.
func DecOfRat(q *big.Rat) *Dec {
	d, err := ratDec(q)
	if err != nil {
		return nil
	}
	return d
}
