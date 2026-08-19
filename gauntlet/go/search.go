package gauntlet

// Gauntlet program 8: SEARCH. None of the first seven exits a loop early, so a
// loop design chosen against them would be chosen blind (docs/spec/iteration.md).
//
// These are the BAR: the best Go a person would write, not Go shaped like our
// loop. Parity is measured against these.

// FindFirstRef returns the index of the first element greater than k, or -1.
// The best Go for this is a range loop with an early return.
func FindFirstRef(a []float64, k float64) int64 {
	for i, x := range a {
		if x > k {
			return int64(i)
		}
	}
	return -1
}

// Gauntlet program 9: CONVERGE. A loop with no trip count at all.
//
// SqrtNewtonRef is Newton's method for a square root, iterated to a fixed
// tolerance. The number of steps depends on the input and cannot be computed
// in advance, which is exactly what fold-range cannot express.
func SqrtNewtonRef(x float64) float64 {
	g := x
	for {
		d := g*g - x
		if d < 0 {
			d = -d
		}
		if d <= 1e-12 {
			return g
		}
		g = (g + x/g) / 2
	}
}
