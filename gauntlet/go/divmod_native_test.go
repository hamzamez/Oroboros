package gauntlet

import "testing"

// The negative product at a boundary (examples/native/divmod-go.oro).
//
// product-2026-08-19 measured the SHAPE at 1.01x with zero allocations. This
// measures what we EMIT, against hand-written Go of both forms.

// NOT noinline. The first draft marked these `//go:noinline` and the emitted
// functions came out 3.2x FASTER — 0.82 ns against 2.67 — which is not a
// believable result for a division. It was call overhead on one side only.
// Both sides must be inlinable, because the emitted code is.
func divmodRef(a, b int) (int, int) { return a / b, a % b }

func divmodSumRef(a, b int) int { return a/b + a%b }

// A variable divisor, so neither side can be strength-reduced to a multiply
// and a shift. Dividing by a constant 7 measured ~0.8 ns, which is a multiply,
// not an idiv.
var divisor = 7

var sinkQ, sinkR, sinkS int

func TestNativeDivmodAgrees(t *testing.T) {
	for _, p := range [][2]int{{7, 3}, {-7, 3}, {7, -3}, {1 << 40, 7}} {
		gq, gr := NativeDivmod(p[0], p[1])
		wq, wr := divmodRef(p[0], p[1])
		if gq != wq || gr != wr {
			t.Errorf("divmod(%d,%d): native (%d,%d), hand-written (%d,%d)", p[0], p[1], gq, gr, wq, wr)
		}
		if got, want := NativeDivmodSum(p[0], p[1]), divmodSumRef(p[0], p[1]); got != want {
			t.Errorf("sum(%d,%d): native %d, hand-written %d", p[0], p[1], got, want)
		}
	}
}

func BenchmarkDivmodRef(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkQ, sinkR = divmodRef(i|1, divisor)
	}
}
func BenchmarkDivmodNative(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkQ, sinkR = NativeDivmod(i|1, divisor)
	}
}
func BenchmarkDivmodSumRef(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkS = divmodSumRef(i|1, divisor)
	}
}
func BenchmarkDivmodSumNative(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkS = NativeDivmodSum(i|1, divisor)
	}
}
