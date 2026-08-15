package gauntlet

// Hand-written reference for examples/stencil.oro — the three-point window sum.
//
// This is the first gauntlet entry the language could not express before
// integer arithmetic existed (docs/spec/inventory.md §1.2), so it is also the
// first parity measurement of a program written *after* its primitives were
// specified rather than before.

// WindowSum is what a person would write.
func WindowSum(a []float64) float64 {
	acc := 0.0
	for j := 0; j < len(a)-2; j++ {
		acc += a[j] + a[j+1] + a[j+2]
	}
	return acc
}

// WindowSumHoisted is the same loop with the bound lifted out by hand. Go does
// NOT hoist `len(a)-2` out of the loop condition on its own, which is what the
// generated form gets for free by binding the count before the loop.
func WindowSumHoisted(a []float64) float64 {
	acc := 0.0
	n := len(a) - 2
	for j := 0; j < n; j++ {
		acc += a[j] + a[j+1] + a[j+2]
	}
	return acc
}

// WindowSumMaterialised is the form that builds the window sums first, carried
// per the gauntlet rule that the expected loser is measured too.
func WindowSumMaterialised(a []float64) float64 {
	if len(a) < 3 {
		return 0
	}
	w := make([]float64, len(a)-2)
	for j := range w {
		w[j] = a[j] + a[j+1] + a[j+2]
	}
	acc := 0.0
	for _, v := range w {
		acc += v
	}
	return acc
}
