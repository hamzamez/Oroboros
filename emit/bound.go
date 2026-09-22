package emit

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
)

// AN INTERVAL ENDPOINT, EXACT TO 2^126 — ADR 0026.
//
// The analysis used int64 endpoints saturating at 2^62, and that was enough
// while every legal integer lived inside ±(2^53−1). Under ADR 0026 a target's
// word is its own — [−2^63, 2^63−1] on Go, the JVM and windows, and [0, 2^64−1]
// is native on Go and windows — so an endpoint must hold 2^64 exactly, or
// `(/ x 10)` on a host `uint64` is [0, +inf] and nothing about it is provable.
//
// bnd is an integer in the open interval (−2^126, 2^126), stored as a 128-bit
// two's-complement number h·2^64 + l. The bound on the magnitude is the same
// device as the old 2^62: ONE BIT OF HEADROOM, so the sum of two endpoints and
// the negation of one can never overflow the representation, and the
// operations below only have to decide whether a result is still an endpoint.
// One that is not saturates to an infinity in the caller, which is sound —
// an interval that says less.
//
// The fast paths are int64. Every endpoint of every program the corpus has is
// inside ±2^62, so the 128-bit arithmetic is exercised by host declarations and
// by nothing else; math/big covers the rare products and quotients of two wide
// endpoints, where simplicity is worth more than speed.
type bnd struct {
	h int64  // the high 64 bits, signed
	l uint64 // the low 64 bits
}

// bi is an int64 as an endpoint.
func bi(v int64) bnd {
	if v < 0 {
		return bnd{-1, uint64(v)}
	}
	return bnd{0, uint64(v)}
}

// bu is a uint64 as an endpoint: 2^64−1 is the reason this type exists.
func bu(v uint64) bnd { return bnd{0, v} }

var (
	bZero = bnd{}
	bOne  = bi(1)
	// bSat is 2^126, the first magnitude that is not an endpoint.
	bSat = bnd{1 << 62, 0}
)

// ok reports whether a 128-bit value is an endpoint: |v| < 2^126.
func (a bnd) ok() bool { return a.h < 1<<62 && a.h >= -(1<<62) && !(a.h == -(1<<62) && a.l == 0) }

// i64 is the endpoint as an int64, when it is one.
func (a bnd) i64() (int64, bool) {
	if (a.h == 0 && a.l <= math.MaxInt64) || (a.h == -1 && a.l > math.MaxInt64) {
		return int64(a.l), true
	}
	return 0, false
}

// small is i64 for callers that have already established the endpoint fits —
// a length, a shift count, a literal's bound. It panics otherwise, because a
// wide endpoint reaching one of those is a bug in the caller, not a value.
func (a bnd) small() int64 {
	v, ok := a.i64()
	if !ok {
		panic("interval endpoint " + a.String() + " does not fit in int64")
	}
	return v
}

func (a bnd) sign() int {
	switch {
	case a.h < 0:
		return -1
	case a.h == 0 && a.l == 0:
		return 0
	}
	return 1
}

func (a bnd) cmp(b bnd) int {
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

func (a bnd) lt(b bnd) bool { return a.cmp(b) < 0 }
func (a bnd) le(b bnd) bool { return a.cmp(b) <= 0 }
func (a bnd) gt(b bnd) bool { return a.cmp(b) > 0 }
func (a bnd) ge(b bnd) bool { return a.cmp(b) >= 0 }

func minB(a, b bnd) bnd {
	if a.lt(b) {
		return a
	}
	return b
}

func maxB(a, b bnd) bnd {
	if a.gt(b) {
		return a
	}
	return b
}

// add is exact: two endpoints sum to less than 2^127 in magnitude, which the
// representation holds, so the only question is whether the sum is an endpoint.
func (a bnd) add(b bnd) (bnd, bool) {
	l, c := bits.Add64(a.l, b.l, 0)
	out := bnd{a.h + b.h + int64(c), l}
	return out, out.ok()
}

// neg is total on endpoints, because the interval they live in is symmetric.
func (a bnd) neg() bnd {
	l, b := bits.Sub64(0, a.l, 0)
	return bnd{-a.h - int64(b), l}
}

func (a bnd) sub(b bnd) (bnd, bool) { return a.add(b.neg()) }

// abs is |a|, which is an endpoint whenever a is.
func (a bnd) abs() bnd {
	if a.h < 0 {
		return a.neg()
	}
	return a
}

func (a bnd) mul(b bnd) (bnd, bool) {
	x, okx := a.i64()
	y, oky := b.i64()
	if okx && oky {
		// |x|·|y| < 2^126 always, so the unsigned 128-bit product is exact and the
		// sign is applied afterwards.
		ux, uy := uint64(x), uint64(y)
		if x < 0 {
			ux = uint64(-x) // wraps to 2^63 for MinInt64, which is its magnitude
		}
		if y < 0 {
			uy = uint64(-y)
		}
		hi, lo := bits.Mul64(ux, uy)
		out := bnd{int64(hi), lo}
		if !out.ok() || hi >= 1<<62 {
			return bnd{}, false
		}
		if (x < 0) != (y < 0) {
			out = out.neg()
		}
		return out, true
	}
	return fromBig(new(big.Int).Mul(a.big(), b.big()))
}

// quo truncates toward zero, which is the language's division (integers.md
// §5). b must not be zero; every caller has excluded it.
func (a bnd) quo(b bnd) bnd {
	x, okx := a.i64()
	y, oky := b.i64()
	if okx && oky && !(x == math.MinInt64 && y == -1) {
		return bi(x / y)
	}
	out, _ := fromBig(new(big.Int).Quo(a.big(), b.big())) // |a/b| <= |a|: always an endpoint
	return out
}

// shr is an arithmetic right shift: floor(a / 2^k).
func (a bnd) shr(k uint) bnd {
	switch {
	case k == 0:
		return a
	case k >= 128:
		if a.h < 0 {
			return bi(-1)
		}
		return bZero
	case k >= 64:
		return bnd{a.h >> 63, uint64(a.h >> (k - 64))}
	}
	return bnd{a.h >> k, a.l>>k | uint64(a.h)<<(64-k)}
}

func (a bnd) big() *big.Int {
	if v, ok := a.i64(); ok {
		return big.NewInt(v)
	}
	neg := a.h < 0
	m := a
	if neg {
		m = a.neg()
	}
	out := new(big.Int).SetUint64(uint64(m.h))
	out.Lsh(out, 64)
	out.Or(out, new(big.Int).SetUint64(m.l))
	if neg {
		out.Neg(out)
	}
	return out
}

var bigSat = new(big.Int).Lsh(big.NewInt(1), 126)

// fromBig is a big integer as an endpoint, when it is one.
func fromBig(v *big.Int) (bnd, bool) {
	if v.IsInt64() {
		return bi(v.Int64()), true
	}
	if v.CmpAbs(bigSat) >= 0 {
		return bnd{}, false
	}
	m := new(big.Int).Abs(v)
	lo := new(big.Int).And(m, new(big.Int).SetUint64(math.MaxUint64)).Uint64()
	hi := new(big.Int).Rsh(m, 64).Uint64()
	out := bnd{int64(hi), lo}
	if v.Sign() < 0 {
		out = out.neg()
	}
	return out, true
}

func (a bnd) String() string {
	if v, ok := a.i64(); ok {
		return big.NewInt(v).String()
	}
	return a.big().String()
}

// Format prints the integer for every verb, so a `%d` written when endpoints
// were int64 — in a diagnostic, a report or a type string — still prints the
// number rather than the struct.
func (a bnd) Format(f fmt.State, verb rune) { fmt.Fprint(f, a.String()) }
