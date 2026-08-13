package gauntlet

// The g1 derivation decided `sum` must be specified left-to-right, because
// floating point addition is not associative and an unordered sum would give
// different answers per target. That decision has a price, and these functions
// measure it.
//
// A left-to-right sum is a serial dependency chain: every iteration waits on the
// previous add. Splitting into independent accumulators breaks the chain, which
// is exactly the reassociation `sum-unordered` would permit.

func DotUnordered4(xs, ys []float64) float64 {
	ys = ys[:len(xs)]
	var a0, a1, a2, a3 float64
	i := 0
	for ; i+4 <= len(xs); i += 4 {
		a0 += xs[i] * ys[i]
		a1 += xs[i+1] * ys[i+1]
		a2 += xs[i+2] * ys[i+2]
		a3 += xs[i+3] * ys[i+3]
	}
	acc := (a0 + a1) + (a2 + a3)
	for ; i < len(xs); i++ {
		acc += xs[i] * ys[i]
	}
	return acc
}

func SumF64Unordered4(xs []float64) float64 {
	var a0, a1, a2, a3 float64
	i := 0
	for ; i+4 <= len(xs); i += 4 {
		a0 += xs[i]
		a1 += xs[i+1]
		a2 += xs[i+2]
		a3 += xs[i+3]
	}
	acc := (a0 + a1) + (a2 + a3)
	for ; i < len(xs); i++ {
		acc += xs[i]
	}
	return acc
}
