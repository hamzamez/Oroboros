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

// The gauntlet's oldest program on the NATIVE Go target — go.len, go.at-float64
// and a `loop`, with no portable layer underneath (examples/native/dot-go.oro).
//
// The delayed vector is the same three lines it always was, so the question is
// only whether the fusion survives moving the names it calls. If this is not at
// parity with the hand-written form, the native targets cannot carry the bar.
func TestDotNativeAgrees(t *testing.T) {
	if got, want := NativeDot(smallA, smallB), DotRange(smallA, smallB); got != want {
		t.Errorf("native %v, hand-written %v", got, want)
	}
}

func BenchmarkSmallDotNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkD = NativeDot(smallA, smallB)
	}
}

// The same program on the LANGUAGE'S OWN table (examples/table/dot.oro).
//
// `vec`, `vlen`, `vindex` and `of-array` are gone: `(table n f)` is the delayed
// vector, `len` is its domain bound, and indexing is application. The question
// is whether deleting the hand-rolled library costs anything, and the bar is
// the 485 ns NativeDot already reaches.
func TestDotTableAgrees(t *testing.T) {
	if got, want := TableDot(smallA, smallB), DotRange(smallA, smallB); got != want {
		t.Errorf("table %v, hand-written %v", got, want)
	}
}

func BenchmarkSmallDotTable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkD = TableDot(smallA, smallB)
	}
}
