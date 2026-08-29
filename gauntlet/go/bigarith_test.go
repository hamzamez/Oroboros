package gauntlet

import (
	"math/big"
	"testing"
)

// CORRECTNESS FIRST, and math/big is the oracle. A limb implementation that is
// fast and wrong is the easiest thing in this file to write by accident.
func TestLimbsAgreeWithMathBig(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 20, 21, 50, 200, 2000} {
		want := FactBigReuse(n)
		got := limbsToBig(FactLimbs(n, make([]uint64, FactLimbCount(n))))
		if got.Cmp(want) != 0 {
			t.Fatalf("fact(%d): limbs gave %s, math/big gave %s", n, got, want)
		}
		if FactBigNaive(n).Cmp(want) != 0 {
			t.Fatalf("fact(%d): the two math/big forms disagree", n)
		}
	}
	for _, n := range []int{0, 1, 2, 10, 93, 94, 300, 1000} {
		want := FibBigReuse(n)
		c := FibLimbCount(n)
		got := limbsToBig(FibLimbs(n, make([]uint64, c), make([]uint64, c), make([]uint64, c)))
		if got.Cmp(want) != 0 {
			t.Fatalf("fib(%d): limbs gave %s, math/big gave %s", n, got, want)
		}
	}
	// AND THE WORKLOAD MUST ACTUALLY LEAVE THE WINDOW, or this file is
	// measuring machine arithmetic with extra steps. Checked rather than
	// assumed, because the first version of this test asserted fib(94) and the
	// answer is 93: `21!` and `fib(93)` are the first values past 2^63, and
	// ADR 0012's window at 2^53-1 is passed earlier still.
	if FactBigReuse(20).BitLen() > 63 || FactBigReuse(21).BitLen() <= 63 {
		t.Fatal("21! is meant to be the first factorial past a machine word")
	}
	if FibBigReuse(92).BitLen() > 63 || FibBigReuse(93).BitLen() <= 63 {
		t.Fatal("fib(93) is meant to be the first past a machine word")
	}
}

func limbsToBig(l []uint64) *big.Int {
	out := new(big.Int)
	for i := len(l) - 1; i >= 0; i-- {
		out.Lsh(out, 64)
		out.Or(out, new(big.Int).SetUint64(l[i]))
	}
	return out
}

// ------------------------------------------------------------- benchmarks
//
// Sizes chosen to bracket the interesting region: 50! is four limbs, 200! is
// twenty, and fib(1000) is eleven. A design that only measured one size would
// miss the crossover, and the crossover is the whole question.

func BenchmarkFact50BigNaive(b *testing.B) { benchFactBig(b, 50, false) }
func BenchmarkFact50BigReuse(b *testing.B) { benchFactBig(b, 50, true) }
func BenchmarkFact50Limbs(b *testing.B)    { benchFactLimbs(b, 50) }

func BenchmarkFact200BigNaive(b *testing.B) { benchFactBig(b, 200, false) }
func BenchmarkFact200BigReuse(b *testing.B) { benchFactBig(b, 200, true) }
func BenchmarkFact200Limbs(b *testing.B)    { benchFactLimbs(b, 200) }

func BenchmarkFact2000BigReuse(b *testing.B) { benchFactBig(b, 2000, true) }
func BenchmarkFact2000Limbs(b *testing.B)    { benchFactLimbs(b, 2000) }

func BenchmarkFib1000BigReuse(b *testing.B) { benchFibBig(b, 1000) }
func BenchmarkFib1000Limbs(b *testing.B)    { benchFibLimbs(b, 1000) }

func benchFactBig(b *testing.B, n int, reuse bool) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if reuse {
			sinkBig = FactBigReuse(n)
		} else {
			sinkBig = FactBigNaive(n)
		}
	}
}

func benchFactLimbs(b *testing.B, n int) {
	acc := make([]uint64, FactLimbCount(n))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkLimbs = FactLimbs(n, acc)
	}
}

func benchFibBig(b *testing.B, n int) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkBig = FibBigReuse(n)
	}
}

func benchFibLimbs(b *testing.B, n int) {
	c := FibLimbCount(n)
	x, y, z := make([]uint64, c), make([]uint64, c), make([]uint64, c)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkLimbs = FibLimbs(n, x, y, z)
	}
}

var (
	sinkBig   *big.Int
	sinkLimbs []uint64
)
