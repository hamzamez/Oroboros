package gauntlet

import "testing"

var andInA, andInB []float64

func init() {
	andInA = make([]float64, 4096)
	andInB = make([]float64, 4096)
	for i := range andInA {
		andInA[i] = float64(i%17) - 8
		andInB[i] = float64(i%13) - 6
	}
}

func TestAndFormsAgree(t *testing.T) {
	for _, p := range []bool{true, false} {
		for _, q := range []bool{true, false} {
			if AndOperator(p, q) != AndNested(p, q) {
				t.Errorf("and(%v,%v) disagrees", p, q)
			}
		}
	}
	if SumWhileOperator(andInA, andInB) != SumWhileNested(andInA, andInB) {
		t.Error("guard forms disagree")
	}
	if AnyRangeOperator(andInA, -3, 3) != AnyRangeNested(andInA, -3, 3) {
		t.Error("three-term forms disagree")
	}
}

var andSinkB bool
var andSinkF float64
var andSinkI int

func BenchmarkAndOperator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkB = AndOperator(i&1 == 0, i&2 == 0)
	}
}

func BenchmarkAndNested(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkB = AndNested(i&1 == 0, i&2 == 0)
	}
}

func BenchmarkSumWhileOperator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkF = SumWhileOperator(andInA, andInB)
	}
}

func BenchmarkSumWhileNested(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkF = SumWhileNested(andInA, andInB)
	}
}

func BenchmarkAnyRangeOperator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkI = AnyRangeOperator(andInA, -3, 3)
	}
}

func BenchmarkAnyRangeNested(b *testing.B) {
	for i := 0; i < b.N; i++ {
		andSinkI = AnyRangeNested(andInA, -3, 3)
	}
}
