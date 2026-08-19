package gauntlet

import (
	"math"
	"testing"
)

var sinkSearch int64
var sinkConv float64

var searchArr = func() []float64 {
	a := make([]float64, 100000)
	for i := range a {
		a[i] = float64(i)
	}
	return a
}()

func TestSearchAgrees(t *testing.T) {
	for _, k := range []float64{-1, 5, 99998, 1e9} {
		if got, want := GenFindFirst(searchArr, k), FindFirstRef(searchArr, k); got != want {
			t.Errorf("k=%v: generated %d, hand-written %d", k, got, want)
		}
	}
	for _, x := range []float64{2, 9, 1e6, 0.25} {
		got, want := GenSqrtNewton(x), SqrtNewtonRef(x)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("x=%v: generated %v, hand-written %v", x, got, want)
		}
	}
}

func BenchmarkSearchRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSearch = FindFirstRef(searchArr, 5)
	}
}
func BenchmarkSearchGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSearch = GenFindFirst(searchArr, 5)
	}
}
func BenchmarkSearchRefLate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSearch = FindFirstRef(searchArr, 99998)
	}
}
func BenchmarkSearchGenLate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSearch = GenFindFirst(searchArr, 99998)
	}
}
func BenchmarkConvergeRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkConv = SqrtNewtonRef(2)
	}
}
func BenchmarkConvergeGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkConv = GenSqrtNewton(2)
	}
}
