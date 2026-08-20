package gauntlet

import "testing"

var sinkNS int

// The gauntlet's early exit on the NATIVE Go target: `loop` with two escape
// clauses, no portable layer underneath (examples/native/search-go.oro).
//
// ADR 0015's claim was that `loop` gives early exit at parity with hand-written
// code. It was measured on the portable layer and this is the first time it is
// checked against a target that has no such layer.
func TestNativeSearchAgrees(t *testing.T) {
	for _, k := range []float64{-1, 5, 99998, 1e9} {
		if got, want := int64(NativeFindFirst(searchArr, k)), FindFirstRef(searchArr, k); got != want {
			t.Errorf("k=%v: native %d, hand-written %d", k, got, want)
		}
	}
}

func BenchmarkSearchNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkNS = NativeFindFirst(searchArr, 5)
	}
}
func BenchmarkSearchNativeLate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkNS = NativeFindFirst(searchArr, 99998)
	}
}
