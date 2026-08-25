package gauntlet

import "testing"

// THE SIEVE, WRITTEN PORTABLY (examples/table/sieve.oro).
//
// ADR 0018 was decided on this program: a gather cannot express a scatter, so
// the sieve was inexpressible portably at any speed. `PlainCountPrimes` is the
// same program emitted from examples/native/sieve-go.oro, which names
// go.make-bool, go.set-bool and go.at-bool; the table version names none of
// them and uses `build`, `set` and indexing by application.
//
// The bar is not the compiler's own earlier output but hand-written Go.

func handSieve(n int) int {
	c := make([]bool, n)
	for i := 2; i*i < n; i++ {
		if c[i] {
			continue
		}
		for j := i * i; j < n; j += i {
			c[j] = true
		}
	}
	acc := 0
	for k := 2; k < n; k++ {
		if !c[k] {
			acc++
		}
	}
	return acc
}

func TestTableSieveAgrees(t *testing.T) {
	for _, n := range []int{2, 3, 10, 100, 1000, 20000, 200000} {
		if got, want := TableCountPrimes(n), handSieve(n); got != want {
			t.Errorf("n=%d: table %d, hand-written %d", n, got, want)
		}
	}
	if got, want := TableCountPrimes(200000), PlainCountPrimes(200000); got != want {
		t.Errorf("table %d, native %d", got, want)
	}
}

var tsSink int

func BenchmarkSieveHand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = handSieve(200000)
	}
}

func BenchmarkSieveTable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = TableCountPrimes(200000)
	}
}
