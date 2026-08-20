package gauntlet

import "testing"

var pairIn []int64

func init() {
	pairIn = make([]int64, 4096)
	for i := range pairIn {
		pairIn[i] = int64(i)*31 + 7
	}
}

func TestPairFormsAgree(t *testing.T) {
	want := PairInline(pairIn)
	for n, got := range map[string]int64{
		"tuple": PairTuple(pairIn), "struct": PairStruct(pairIn), "ptr": PairPtr(pairIn),
	} {
		if got != want {
			t.Errorf("%s gave %d, want %d", n, got, want)
		}
	}
}

var pairSink int64

func BenchmarkPairInline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pairSink = PairInline(pairIn)
	}
}
func BenchmarkPairTuple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pairSink = PairTuple(pairIn)
	}
}
func BenchmarkPairStruct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pairSink = PairStruct(pairIn)
	}
}
func BenchmarkPairPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pairSink = PairPtr(pairIn)
	}
}
