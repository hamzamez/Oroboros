package gauntlet

// Hand-written references for the filter-sum program, to price the duplicated
// array read that the generated version contains.
//
// docs/spec/concerns.md §1.1 claims the missing call-by-need discipline costs
// "a silent 2× on the hot loop". Ref binds the element once; Dup reads it twice,
// which is exactly what the generated code does.

// FilterSumRef binds the element once — what call-by-need would produce.
func FilterSumRef(a []float64) float64 {
	acc := 0.0
	for i := 0; i < len(a); i++ {
		x := a[i]
		if x > 0 {
			acc += x
		}
	}
	return acc
}

// FilterSumDup reads the element twice, as the generated code does.
func FilterSumDup(a []float64) float64 {
	acc := 0.0
	for i := 0; i < len(a); i++ {
		if a[i] > 0 {
			acc += a[i]
		}
	}
	return acc
}
