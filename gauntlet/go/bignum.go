package gauntlet

import (
	"math/big"
	"math/bits"
)

// The price of exact integers, at the operation that actually costs.
//
// gauntlet/results/overflow-2026-08-19.md priced ADDITION, and hamza's
// correction is right: addition is the cheap case. Multiplication is where a
// bignum representation is actually paid for, for two reasons that compound.
//
//   - DETECTING overflow on a multiply is harder than on an add. There is no
//     single flag test that survives into a high-level language: you either
//     compute the full 128-bit product and look at the top half, or you divide
//     the result back and compare.
//   - PERFORMING a big multiply is quadratic in the limb count, where a big
//     add is linear. Two limbs is four word-multiplies plus the carries.
//
// So the honest question is not "what does a checked add cost" but "what does a
// checked MULTIPLY cost, and what does the slow path cost when it is taken".

// SHAPE. Each element is ONE independent multiply of two small values, summed.
// A running product overflows after about twenty iterations, so the first
// version of this benchmark measured how fast the checked forms bail out — they
// came in at 31 ns against 2828 ns for the unchecked one, which is the shape of
// a mistake rather than a result.

// MulPlain is the baseline: wrapping machine multiplication, unchecked.
func MulPlain(a, b []int64) int64 {
	s := int64(0)
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// MulCheckedHi computes the full 128-bit product and tests the high half. This
// is what a runtime with access to the hardware does — and what Go exposes
// through math/bits, which compiles to one multiply pair.
func MulCheckedHi(a, b []int64) int64 {
	s := int64(0)
	for i := range a {
		hi, lo := bits.Mul64(uint64(a[i]), uint64(b[i]))
		if hi != 0 || lo > 1<<63-1 {
			return overflowed(int64(hi), int64(lo))
		}
		s += int64(lo)
	}
	return s
}

// MulCheckedDiv is the portable check — divide the product back and compare —
// and is what a language with no high-multiply primitive is left with. Two of
// our four targets are in that position.
func MulCheckedDiv(a, b []int64) int64 {
	s := int64(0)
	for i := range a {
		t := a[i] * b[i]
		if b[i] != 0 && t/b[i] != a[i] {
			return overflowed(t, b[i])
		}
		s += t
	}
	return s
}

// MulBig is math/big used naively — a fresh result each time, which is how a
// `bignum` type would lower if nothing were done about allocation.
func MulBig(a, b []int64) int64 {
	s := int64(0)
	x, y := new(big.Int), new(big.Int)
	for i := range a {
		x.SetInt64(a[i])
		y.SetInt64(b[i])
		s += new(big.Int).Mul(x, y).Int64()
	}
	return s
}

// MulBigReuse is math/big used as carefully as it can be: one receiver, no
// allocation in the loop. The gap between this and MulBig is what naive use
// costs ON TOP of the representation.
func MulBigReuse(a, b []int64) int64 {
	s := int64(0)
	x, y, r := new(big.Int), new(big.Int), new(big.Int)
	for i := range a {
		x.SetInt64(a[i])
		y.SetInt64(b[i])
		r.Mul(x, y)
		s += r.Int64()
	}
	return s
}
