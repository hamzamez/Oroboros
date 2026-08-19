package gauntlet

import "testing"

func TestCentroidLoopAgrees(t *testing.T) {
	if got, want := GenlCentroid(vecA, vecB), GenCentroid(vecA, vecB); got != want {
		t.Errorf("loop %v, fold-range2 %v", got, want)
	}
}

func BenchmarkCentroidHand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkF2 = CentroidSumRef(vecA, vecB)
	}
}
func BenchmarkCentroidFold2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkF2 = GenCentroid(vecA, vecB)
	}
}
func BenchmarkCentroidLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkF2 = GenlCentroid(vecA, vecB)
	}
}

var sinkF2 float64
