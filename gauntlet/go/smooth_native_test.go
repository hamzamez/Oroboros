package gauntlet

import "testing"

// Gauntlet program 7 on the NATIVE Go target, and the experiment ADR 0013 named.
//
// The decision accepted 1.79x against a hand-written buffer-reusing form,
// because `num/vec.materialize` allocates fresh so nothing can alias. Two forms
// are measured here: one that allocates (matching SmoothFresh) and one that
// writes through a destination the caller owns (matching Smooth).
//
// Indexing differs by one from the hand-written references by construction: the
// language's kernel writes j in [0, len-2) reading a[j..j+2]; Smooth writes
// i in [1, len-1) reading src[i-1..i+1]. Same values, shifted.

var smoothSrc = func() []float64 {
	a := make([]float64, 100000)
	for i := range a {
		a[i] = float64(i % 97)
	}
	return a
}()
var smoothDst = make([]float64, 100000)
var sinkSm []float64

func TestNativeSmoothAgrees(t *testing.T) {
	want := SmoothFresh(smoothSrc)
	got := NativeSmooth(smoothSrc)
	if len(got) != len(smoothSrc)-2 {
		t.Fatalf("length %d, want %d", len(got), len(smoothSrc)-2)
	}
	for i := 1; i < len(smoothSrc)-1; i++ {
		if got[i-1] != want[i] {
			t.Fatalf("at %d: native %v, hand-written %v", i, got[i-1], want[i])
		}
	}
	into := make([]float64, len(smoothSrc))
	NativeSmoothInto(into, smoothSrc)
	for i := 1; i < len(smoothSrc)-1; i++ {
		if into[i-1] != want[i] {
			t.Fatalf("into at %d: native %v, hand-written %v", i, into[i-1], want[i])
		}
	}
}

// Allocating forms.
func BenchmarkStencilFreshRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSm = SmoothFresh(smoothSrc)
	}
}
func BenchmarkStencilFreshNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSm = NativeSmooth(smoothSrc)
	}
}

// Buffer-reusing forms — the bar ADR 0013 could not reach.
func BenchmarkStencilIntoRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Smooth(smoothDst, smoothSrc)
	}
}
func BenchmarkStencilIntoRefNoAlias(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SmoothNoAlias(smoothDst, smoothSrc)
	}
}
func BenchmarkStencilIntoNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkSm = NativeSmoothInto(smoothDst, smoothSrc)
	}
}
