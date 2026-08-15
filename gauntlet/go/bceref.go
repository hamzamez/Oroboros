package gauntlet

// GenDotChecked is exactly what cmd/gen emitted BEFORE the narrowing pattern.
// Kept so the transformation can be measured with everything else identical —
// same inputs, same package, same call shape.
func GenDotChecked(p []float64, q []float64) float64 {
	acc := 0.0
	n1 := (len(p))
	for i := 0; i < n1; i++ {
		acc = (acc + ((p[i]) * (q[i])))
	}
	return acc
}
