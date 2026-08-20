package gauntlet

import "testing"

// Gauntlet program 3 on the NATIVE Go target (examples/native/generic-go.oro).
//
// g3's claim is that a non-recursive definition IS a rewrite rule, so
// instantiation is a side effect of matching and there is no monomorphization
// pass. The portable form could not fully test it: `aindex` and `sat` are
// different names for the same shape, so matching could have been keying on the
// name. Here the two instantiations reach go.at-float64 and go.at-string —
// genuinely different host functions over []float64 and []string — and the two
// `combine`s do not even have the same shape: one is an expression, one a
// statement. The emitted code is two ordinary Go functions.

func TestNativeGenericInstantiationsAgree(t *testing.T) {
	if got, want := NativeSumOf(vecA), SumF64(vecA); got != want {
		t.Errorf("native sum %v, hand-written %v", got, want)
	}
	got, want := NativeWordTally(text), WordCountIncr(text)
	if len(got) != len(want) {
		t.Fatalf("native has %d keys, hand-written %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%q: native %d, hand-written %d", k, got[k], v)
		}
	}
}

func BenchmarkG3SumF64Native(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkF = NativeSumOf(vecA)
	}
}
func BenchmarkG3WordTallyNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkM = NativeWordTally(text)
	}
}
