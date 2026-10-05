//go:build ignore

// Each host's default float-to-text, on the same doubles (win32org-2026-10-05 §4).
package main

import (
	"fmt"
	"math"
	"strconv"
)

func main() {
	a, b, one, three := 0.1, 0.2, 1.0, 3.0 // variables: Go folds an untyped constant exactly
	xs := []float64{a + b, 1.0, 100.0, 1e7, 1.0e21, 1e-7, 2e23, 5e-324,
		1.7976931348623157e308, 9007199254740993.0, math.Copysign(0, -1), one / three}
	for _, x := range xs {
		fmt.Printf("%-24v %s\n", x, strconv.FormatFloat(x, 'g', -1, 64))
	}
}
