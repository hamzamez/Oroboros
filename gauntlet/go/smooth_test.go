package gauntlet

import "testing"

func TestGeneratedSmoothMatchesHandWritten(t *testing.T) {
	want := make([]float64, len(vecA)-2)
	SmoothInto(want, vecA)
	got := GenSmooth(vecA)
	if len(got) != len(want) {
		t.Fatalf("length %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("element %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

// One application. Allocation is amortised over the whole loop.
func BenchmarkG7SmoothIntoOnce(b *testing.B) {
	dst := make([]float64, len(vecA)-2)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SmoothInto(dst, vecA)
	}
}

func BenchmarkG7SmoothAllocOnce(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkV = SmoothAlloc(vecA)
	}
}

func BenchmarkG7GenSmoothOnce(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkV = GenSmooth(vecA)
	}
}

// Repeated application — what a stencil is actually for, and where the
// hand-written form reuses two buffers while materialize allocates every pass.
const smoothPasses = 8

func BenchmarkG7SmoothIntoRepeated(b *testing.B) {
	src := make([]float64, len(vecA))
	dst := make([]float64, len(vecA))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		copy(src, vecA)
		for p := 0; p < smoothPasses; p++ {
			SmoothInto(dst, src)
			src, dst = dst, src
		}
		sinkV = src
	}
}

func BenchmarkG7GenSmoothRepeated(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v := vecA
		for p := 0; p < smoothPasses; p++ {
			v = GenSmooth(v)
		}
		sinkV = v
	}
}

var sinkV []float64
