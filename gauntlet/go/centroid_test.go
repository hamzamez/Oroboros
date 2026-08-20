package gauntlet

import "testing"

// Gauntlet program 2 on the NATIVE Go target (examples/native/centroid-go.oro).
//
// The Church-encoded point reduces away completely and the residual is two
// scalar accumulators. What moving the program REMOVED is `fold-range2`: it
// existed only because there were two accumulators and no product to pair them
// with, and `loop` has n variables and no product at all (ADR 0015).
func TestNativeCentroidAgrees(t *testing.T) {
	if got, want := NativeCentroid(vecA, vecB), CentroidSumRef(vecA, vecB); got != want {
		t.Errorf("native %v, hand-written %v", got, want)
	}
}

func BenchmarkG2CentroidNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkF2 = NativeCentroid(vecA, vecB)
	}
}
