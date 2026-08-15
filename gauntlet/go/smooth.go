package gauntlet

// Hand-written references for examples/smooth.oro — gauntlet program 7's write
// half, the last gauntlet program to get an Oroboros implementation.
//
// The comparison that matters is not one call. A stencil is applied REPEATEDLY,
// and that is where the two forms diverge: the hand-written one writes into a
// caller-owned buffer and allocates nothing, while `materialize` allocates a
// fresh array every pass. This is the case where "unique by construction" is
// expected to cost something.

// SmoothInto is the hand-written form: the caller owns dst, nothing allocates.
// Offsets are shifted by one against gauntlet.go's Smooth so both forms compute
// the same n-2 outputs from the same n inputs.
func SmoothInto(dst, src []float64) {
	for i := 0; i < len(src)-2; i++ {
		dst[i] = (src[i] + src[i+1] + src[i+2]) / 3
	}
}

// SmoothAlloc is the hand-written form written functionally — allocating a fresh
// result, which is what `materialize` does. Carried so the generated code is
// compared against the same shape as well as against the best shape.
func SmoothAlloc(src []float64) []float64 {
	dst := make([]float64, len(src)-2)
	for i := 0; i < len(src)-2; i++ {
		dst[i] = (src[i] + src[i+1] + src[i+2]) / 3
	}
	return dst
}
