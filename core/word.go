package core

import (
	"fmt"
	"math/big"
)

// A TARGET'S WORD — ADR 0026.
//
// `int` means ℤ. A target realizes a sub-lattice of intervals by containment,
// and its WORD is the widest interval its native integer arithmetic is exact
// on: [−2^63, 2^63−1] on Go, the JVM and windows, and [−(2^53−1), 2^53−1] on
// JavaScript, whose integers are float64s. It is DATA — a target declares
// `(repr (int LO HI) word)` — and the compiler holds no default, because a
// default would be W(S) for some set of targets S, and W(S) = ⋂ Exact(T)
// shrinks every time S grows. That antitonicity is what ADR 0012's window was
// and what 0026 removed.
//
// Everything that used to ask "is this inside the portable window?" now asks
// it of a Word: the reader's promotion of a declared range to arbitrary
// precision, constant folding (ADR 0009), and the type a range normalises to.
type Word struct {
	Lo, Hi int64

	// Unsigned says the target ALSO realizes U = [0, 2^64−1] natively, at word
	// width (ADR 0026 (10)): Go's `uint64`. The two realizations are
	// incomparable — neither interval contains the other — and both are sets of
	// representatives of ℤ/2^64, which is the theorem their mixed arithmetic
	// rests on (emit/wordsel.go). It is the one unsigned realization any 64-bit
	// two's-complement host has, so it is a flag and not a second interval.
	Unsigned bool
}

// U64Type is the language-level name of the unsigned realization: the type a
// range inside U and outside the signed word has, on a target that realizes U.
const U64Type = "u64"

var maxU64 = new(big.Int).SetUint64(1<<64 - 1)

// Contains reports [lo, hi] ⊆ w.
func (w Word) Contains(lo, hi *big.Int) bool {
	return lo.Cmp(big.NewInt(w.Lo)) >= 0 && hi.Cmp(big.NewInt(w.Hi)) <= 0
}

// ContainsU reports [lo, hi] ⊆ U, on a target that realizes U.
func (w Word) ContainsU(lo, hi *big.Int) bool {
	return w.Unsigned && lo.Sign() >= 0 && hi.Cmp(maxU64) <= 0
}

// Has reports v ∈ w.
func (w Word) Has(v int64) bool { return w.Lo <= v && v <= w.Hi }

func (w Word) String() string {
	if w.Unsigned {
		return fmt.Sprintf("[%d, %d] and [0, 18446744073709551615]", w.Lo, w.Hi)
	}
	return fmt.Sprintf("[%d, %d]", w.Lo, w.Hi)
}

// Exceeds reports whether a range type names a set the word does not contain —
// the rung above the host's word on this target.
//
// It is the test that separates a REFINEMENT from a WIDENING. Every range inside
// the word satisfies [LO,HI] ⊆ word, which is why ValueType normalises one to
// `int` and an `int` is accepted wherever it is wanted. A range outside it does
// not, so it must be refused there instead — and that refusal is the surface: it
// is where a programmer finds out a value has left the machine word ON THIS
// TARGET. The same range may be a word on Go and arbitrary precision on
// JavaScript; the integer is the same, and the declaration is what makes the
// difference visible.
func (w Word) Exceeds(ty string) bool {
	lo, hi, ok := IntRangeBig(ty)
	if !ok {
		return false
	}
	return !w.Contains(lo, hi) && !w.ContainsU(lo, hi)
}

// ValueType is what a range MEANS on this target, as opposed to how it is
// stored. A range inside the word is an `int` — only a table's element slot ever
// consults the width — and a range outside it, or with an infinite endpoint, is
// arbitrary precision.
//
// Keeping these apart is what stops a narrowed array from narrowing the LOCALS
// that read it — a byte array read into a byte-wide counter would overflow at
// 255 while the language says the value is an integer.
func (w Word) ValueType(ty string) string {
	if lo, hi, ok := IntRangeBig(ty); ok && w.Contains(lo, hi) {
		return "int"
	} else if ok && w.ContainsU(lo, hi) {
		return U64Type // outside the signed word, inside U
	}
	// ABOVE THE WORD A RANGE IS NOT AN `int` AT ALL — it is the rung above the
	// host's word (unbounded-rung.md §3). The promotion is a widening, not a
	// refinement, so `compatible` refuses it against `int`, and that refusal is
	// where a programmer finds out a value became a bignum.
	if w.Exceeds(ty) || UnboundedRange(ty) {
		return BigType
	}
	return ty
}
