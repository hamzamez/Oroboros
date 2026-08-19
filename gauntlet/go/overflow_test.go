package gauntlet

import "testing"

var ovIn []int64

func init() {
	ovIn = make([]int64, 4096)
	for i := range ovIn {
		ovIn[i] = int64(i%1000) - 500 // small, so no path ever overflows
	}
}

func TestOverflowFormsAgree(t *testing.T) {
	a, b, c := SumPlain(ovIn), SumChecked(ovIn), SumWindowed(ovIn)
	if a != b || a != c {
		t.Errorf("forms disagree: %d %d %d", a, b, c)
	}
}

var ovSink int64

func BenchmarkSumPlain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ovSink = SumPlain(ovIn)
	}
}

func BenchmarkSumChecked(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ovSink = SumChecked(ovIn)
	}
}

func BenchmarkSumWindowed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ovSink = SumWindowed(ovIn)
	}
}
