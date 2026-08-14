package gauntlet

// Hand-written reference for gauntlet program 2's loop: two accumulators, one
// pass, struct-of-arrays input.
func CentroidSumRef(xs, ys []float64) float64 {
	ax, ay := 0.0, 0.0
	for i := 0; i < len(xs); i++ {
		ax += xs[i]
		ay += ys[i]
	}
	return ax + ay
}
