package gauntlet

import (
	"math/big"
	"testing"
)

// CORRECTNESS FIRST, against `math/big`'s own answers rather than against our
// other implementation — two forms of ours agreeing proves only that they are
// the same program.

func TestGeneratedBignumsAreExact(t *testing.T) {
	for _, n := range []int{0, 1, 2, 10, 50, 90, 100, 200, 300} {
		want := new(big.Int)
		a, b := big.NewInt(0), big.NewInt(1)
		for i := 0; i < n; i++ {
			a, b = b, new(big.Int).Add(a, b)
		}
		want.Set(a)
		if got := GenFib(n); got.Cmp(want) != 0 {
			t.Errorf("GenFib(%d) = %s, want %s", n, got, want)
		}
	}
	for _, n := range []int{0, 1, 5, 20, 30, 100} {
		// MulRange is math/big's own factorial, so this is an oracle and not a
		// restatement of the loop under test.
		want := new(big.Int).MulRange(1, int64(n))
		if n == 0 {
			want = big.NewInt(1)
		}
		if got := GenFact(n); got.Cmp(want) != 0 {
			t.Errorf("GenFact(%d) = %s, want %s", n, got, want)
		}
	}
	for _, c := range [][2]int{{2, 10}, {3, 40}, {7, 33}, {999, 64}, {1000, 60}, {5, 0}} {
		want := new(big.Int).Exp(big.NewInt(int64(c[0])), big.NewInt(int64(c[1])), nil)
		if got := GenPower(c[0], c[1]); got.Cmp(want) != 0 {
			t.Errorf("GenPower(%d,%d) = %s, want %s", c[0], c[1], got, want)
		}
	}
}

// AND THE PROGRAM THE REFUSAL QUOTES, pinned by its digits. ADR 0019 names
// fib(100) as the case where Go and the JVM both return 3736710778780434371
// and V8 returns 354224848179262000000, none of them right.
func TestFibOneHundredIsTheTrueValue(t *testing.T) {
	want, _ := new(big.Int).SetString("354224848179261915075", 10)
	if got := GenFib(100); got.Cmp(want) != 0 {
		t.Errorf("GenFib(100) = %s, want %s", got, want)
	}
	// The wrapped answer, so a regression to machine words is recognisable
	// rather than merely wrong.
	if GenFib(100).IsInt64() {
		t.Error("GenFib(100) fits an int64, so it is not the true value")
	}
}

var bigSink *big.Int

func BenchmarkBigFibGen(b *testing.B)     { benchFib(b, GenFib) }
func BenchmarkBigFibHand(b *testing.B)    { benchFib(b, FibBigNaive) }
func BenchmarkBigFibReuse(b *testing.B)   { benchFib(b, FibBigReuse) }
func BenchmarkBigFactReuse(b *testing.B)  { benchFact(b, FactBigReuse) }
func BenchmarkBigPowerReuse(b *testing.B) { benchPower(b, PowerBigReuse) }
func BenchmarkBigFactGen(b *testing.B)    { benchFact(b, GenFact) }
func BenchmarkBigFactHand(b *testing.B)   { benchFact(b, FactBigNaive) }
func BenchmarkBigPowerGen(b *testing.B)   { benchPower(b, GenPower) }
func BenchmarkBigPowerHand(b *testing.B)  { benchPower(b, PowerBigNaive) }

func benchFib(b *testing.B, f func(int) *big.Int) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bigSink = f(1000)
	}
}

func benchFact(b *testing.B, f func(int) *big.Int) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bigSink = f(200)
	}
}

func benchPower(b *testing.B, f func(int, int) *big.Int) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bigSink = f(999, 64)
	}
}
