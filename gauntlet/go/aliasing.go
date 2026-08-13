package gauntlet

// Gauntlet program 6: mutation through a slice that may alias.
//
// The five original programs never mutate a shared structure, so uniqueness was
// free everywhere and s2's inference test had a hole. This is the case where it
// stops being free.
//
// Two sub-cases:
//
//	6a  Smooth*   — a stencil, where aliasing changes the ANSWER, not just speed.
//	                This is the problem C's `restrict` exists to solve, and none
//	                of Go, JS, or Java can express it.
//	6b  DictCopy  — the price of failing to prove uniqueness: a defensive copy.

// ---------------------------------------------------------------- 6a: stencil

// Smooth is the naive form. dst and src may be the same slice, and the compiler
// must assume they are — so it cannot keep src[i] in a register across the write
// to dst[i].
func Smooth(dst, src []float64) {
	for i := 1; i < len(src)-1; i++ {
		dst[i] = (src[i-1] + src[i] + src[i+1]) / 3
	}
}

// SmoothNoAlias carries the window in registers, which is what you would write
// if you knew dst and src were disjoint.
//
// Under aliasing this produces a DIFFERENT result from Smooth: it behaves
// as-if-not-aliased, because the values were read before any write. That
// difference is the whole point of program 6.
func SmoothNoAlias(dst, src []float64) {
	if len(dst) != len(src) {
		panic("smooth: length mismatch")
	}
	a, b := src[0], src[1]
	for i := 1; i < len(src)-1; i++ {
		c := src[i+1]
		dst[i] = (a + b + c) / 3
		a, b = b, c
	}
}

// SmoothFresh allocates the destination itself, so the compiler knows it cannot
// alias src. This is the shape the language would emit if mutable slice
// parameters were forbidden and the compiler chose reuse by liveness.
func SmoothFresh(src []float64) []float64 {
	dst := make([]float64, len(src))
	for i := 1; i < len(src)-1; i++ {
		dst[i] = (src[i-1] + src[i] + src[i+1]) / 3
	}
	return dst
}

// ---------------------------------------------------------------- 6b: copy cost

// DictCopy is what the compiler must emit when it cannot prove the dict is
// uniquely referenced. This prices a liveness false negative.
func DictCopy(m map[string]int) map[string]int {
	n := make(map[string]int, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

// DictInsertInPlace is the sound-when-unique version, for comparison.
func DictInsertInPlace(m map[string]int, k string, v int) map[string]int {
	m[k] = v
	return m
}

// DictInsertCopying is what a functional update must do when uniqueness fails.
func DictInsertCopying(m map[string]int, k string, v int) map[string]int {
	n := DictCopy(m)
	n[k] = v
	return n
}

// SliceCopy prices the same false negative for a slice.
func SliceCopy(xs []float64) []float64 {
	ys := make([]float64, len(xs))
	copy(ys, xs)
	return ys
}
