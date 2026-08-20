package gauntlet

import "testing"

var mulA, mulB []int64

func init() {
	mulA = make([]int64, 4096)
	mulB = make([]int64, 4096)
	for i := range mulA {
		// Small enough that no product ever overflows, so every benchmark runs
		// the whole loop and the checks are measured rather than escaped.
		mulA[i] = int64(i%1000) + 1
		mulB[i] = int64(i%997) + 1
	}
}

var mulSink int64

func TestMulFormsAgree(t *testing.T) {
	p := MulPlain(mulA, mulB)
	for n, got := range map[string]int64{
		"hi": MulCheckedHi(mulA, mulB), "div": MulCheckedDiv(mulA, mulB),
		"big": MulBig(mulA, mulB), "bigreuse": MulBigReuse(mulA, mulB),
	} {
		if got != p {
			t.Errorf("%s gave %d, plain gave %d", n, got, p)
		}
	}
}

func BenchmarkMulPlain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mulSink = MulPlain(mulA, mulB)
	}
}
func BenchmarkMulCheckedHi(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mulSink = MulCheckedHi(mulA, mulB)
	}
}
func BenchmarkMulCheckedDiv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mulSink = MulCheckedDiv(mulA, mulB)
	}
}
func BenchmarkMulBig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mulSink = MulBig(mulA, mulB)
	}
}
func BenchmarkMulBigReuse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mulSink = MulBigReuse(mulA, mulB)
	}
}
