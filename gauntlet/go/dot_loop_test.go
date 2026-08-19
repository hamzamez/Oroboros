package gauntlet

import "testing"

func TestDotLoopAgrees(t *testing.T) {
	if got, want := GenlDot(smallA, smallB), GenDot(smallA, smallB); got != want {
		t.Errorf("loop %v, fold-range %v", got, want)
	}
}

var sinkD float64

// n=1024 — compute-bound, which is where bce-2026-08-15 measured 1.96x.
func BenchmarkSmallDotHand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkD = DotRange(smallA, smallB)
	}
}
func BenchmarkSmallDotFold(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkD = GenDot(smallA, smallB)
	}
}
func BenchmarkSmallDotLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkD = GenlDot(smallA, smallB)
	}
}
