package ir

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
)

// AN INTERVAL'S FINITE END: an integer of E = (−2¹²⁶, 2¹²⁶).
//
// The domain is intervals over ℤ̄ = ℤ ∪ {±∞} whose finite ends lie in E; an end
// a computation takes outside E is replaced by the infinity on its side, which
// is sound (γ only grows) and is the only approximation the representation
// makes. E is the term analysis's (emit/bound.go), and it is what legality
// needs: U = [0, 2⁶⁴) must be exact, or `(/ x 10)` on a u64 is [0, +∞] and
// nothing about it is provable; and "bounded, but not by this word" (a product
// of two words, < 2¹²⁶ in magnitude) must be distinguishable from unbounded.
//
// ep is h·2⁶⁴ + l in 128-bit two's complement. One bit of headroom (|v| < 2¹²⁶
// in a 128-bit word) means the sum of two ends and the negation of one never
// overflow the REPRESENTATION, so each operation only decides whether its
// result is still an end.
type ep struct {
	h int64
	l uint64
}

// ei is an int64 as an end.
func ei(v int64) ep {
	if v < 0 {
		return ep{-1, uint64(v)}
	}
	return ep{0, uint64(v)}
}

var (
	two63  = ep{0, 1 << 63}
	u64Max = ep{0, math.MaxUint64}         // 2⁶⁴ − 1
	two64  = ep{1, 0}                      // 2⁶⁴
	eMax   = ep{1<<62 - 1, math.MaxUint64} // 2¹²⁶ − 1, the greatest end
)

// ok reports whether a 128-bit value is an end: |v| < 2¹²⁶.
func (a ep) ok() bool { return a.h < 1<<62 && a.h >= -(1<<62) && !(a.h == -(1<<62) && a.l == 0) }

// i64 is the end as an int64, when it is one.
func (a ep) i64() (int64, bool) {
	if (a.h == 0 && a.l <= math.MaxInt64) || (a.h == -1 && a.l > math.MaxInt64) {
		return int64(a.l), true
	}
	return 0, false
}

func (a ep) sign() int {
	switch {
	case a.h < 0:
		return -1
	case a.h == 0 && a.l == 0:
		return 0
	}
	return 1
}

func (a ep) cmp(b ep) int {
	switch {
	case a.h < b.h:
		return -1
	case a.h > b.h:
		return 1
	case a.l < b.l:
		return -1
	case a.l > b.l:
		return 1
	}
	return 0
}

func (a ep) lt(b ep) bool { return a.cmp(b) < 0 }
func (a ep) le(b ep) bool { return a.cmp(b) <= 0 }

func minE(a, b ep) ep {
	if a.lt(b) {
		return a
	}
	return b
}

func maxE(a, b ep) ep {
	if b.lt(a) {
		return a
	}
	return b
}

// add is exact in the representation; ok says whether the sum is an end.
func (a ep) add(b ep) (ep, bool) {
	l, c := bits.Add64(a.l, b.l, 0)
	out := ep{a.h + b.h + int64(c), l}
	return out, out.ok()
}

// neg is total on E, which is symmetric.
func (a ep) neg() ep {
	l, b := bits.Sub64(0, a.l, 0)
	return ep{-a.h - int64(b), l}
}

func (a ep) sub(b ep) (ep, bool) { return a.add(b.neg()) }

func (a ep) abs() ep {
	if a.h < 0 {
		return a.neg()
	}
	return a
}

func (a ep) mul(b ep) (ep, bool) {
	x, okx := a.i64()
	y, oky := b.i64()
	if okx && oky {
		// |x|·|y| ≤ 2¹²⁶, so the unsigned 128-bit product is exact; the sign is
		// applied after.
		ux, uy := uint64(x), uint64(y)
		if x < 0 {
			ux = uint64(-x) // 2⁶³ for MinInt64, which is its magnitude
		}
		if y < 0 {
			uy = uint64(-y)
		}
		hi, lo := bits.Mul64(ux, uy)
		if hi >= 1<<62 {
			return ep{}, false
		}
		out := ep{int64(hi), lo}
		if (x < 0) != (y < 0) {
			out = out.neg()
		}
		return out, true
	}
	return fromBig(new(big.Int).Mul(a.big(), b.big()))
}

// quo truncates toward zero (integers.md §3); b ≠ 0. |a/b| ≤ |a|, so the
// quotient is always an end.
func (a ep) quo(b ep) ep {
	x, okx := a.i64()
	y, oky := b.i64()
	if okx && oky && !(x == math.MinInt64 && y == -1) {
		return ei(x / y)
	}
	out, _ := fromBig(new(big.Int).Quo(a.big(), b.big()))
	return out
}

func (a ep) big() *big.Int {
	if v, ok := a.i64(); ok {
		return big.NewInt(v)
	}
	m := a.abs()
	out := new(big.Int).SetUint64(uint64(m.h))
	out.Lsh(out, 64)
	out.Or(out, new(big.Int).SetUint64(m.l))
	if a.h < 0 {
		out.Neg(out)
	}
	return out
}

var bigSat = new(big.Int).Lsh(big.NewInt(1), 126)

// fromBig is an integer as an end, when it is one.
func fromBig(v *big.Int) (ep, bool) {
	if v.IsInt64() {
		return ei(v.Int64()), true
	}
	if v.CmpAbs(bigSat) >= 0 {
		return ep{}, false
	}
	m := new(big.Int).Abs(v)
	lo := new(big.Int).And(m, new(big.Int).SetUint64(math.MaxUint64)).Uint64()
	hi := new(big.Int).Rsh(m, 64).Uint64()
	out := ep{int64(hi), lo}
	if v.Sign() < 0 {
		out = out.neg()
	}
	return out, true
}

func (a ep) String() string { return a.big().String() }

// Format prints the integer for every verb, so `%d` in a type string prints
// the number and not the struct.
func (a ep) Format(f fmt.State, verb rune) { fmt.Fprint(f, a.String()) }

// floorDivE and ceilDivE are ⌊a/b⌋ and ⌈a/b⌉ for b ≠ 0; each is an end when a
// is, since its magnitude is at most |a|, or |a| + 1 ≤ 2¹²⁶ − 1 when |b| = 1
// cannot round.
func floorDivE(a, b ep) ep {
	if x, ok := a.i64(); ok {
		if y, ok := b.i64(); ok && !(x == math.MinInt64 && y == -1) {
			q := x / y
			if x%y != 0 && (x < 0) != (y < 0) {
				q--
			}
			return ei(q)
		}
	}
	out, _ := fromBig(floorBig(a.big(), b.big()))
	return out
}

func ceilDivE(a, b ep) ep {
	return floorDivE(a.neg(), b).neg()
}

// floorBig is ⌊a/b⌋ in math/big.
func floorBig(a, b *big.Int) *big.Int {
	q, r := new(big.Int).QuoRem(a, b, new(big.Int))
	if r.Sign() != 0 && (r.Sign() < 0) != (b.Sign() < 0) {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

// isqrtE is ⌊√n⌋ for n ≥ 0.
func isqrtE(n ep) ep {
	if x, ok := n.i64(); ok {
		return ei(isqrt(x))
	}
	out, _ := fromBig(new(big.Int).Sqrt(n.big()))
	return out
}
