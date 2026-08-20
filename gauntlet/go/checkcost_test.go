package gauntlet

import "testing"

// WHAT AN UNPROVEN OPERATION COSTS IN A REAL PROGRAM.
//
// gauntlet/results/overflow-2026-08-19.md priced the check in ISOLATION — a
// tight loop doing nothing but the operation — and got 1.65× for a windowed add
// and 1.87× to 7.40× for a multiply. That is the number a microbenchmark gives
// and it is not the number a program pays.
//
// These are the same source compiled twice, differing only in whether the
// signature declares a range, so the checked and unchecked forms are the
// compiler's own output rather than hand-written approximations of it.
//
// Two shapes, deliberately at opposite ends:
//   - sum-range is ARITHMETIC-BOUND: two adds and a compare per iteration, and
//     both adds are checked in the undeclared build.
//   - the sieve is MEMORY-BOUND: the inner loop is a byte store, and the
//     checked operations are the index arithmetic around it.
//
// bce-2026-08-15 already recorded that a 1.96× win in isolation disappears on a
// memory-bound loop. The question is whether a cost behaves the same way.

var ccSink int

func BenchmarkSumRangePlain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ccSink = PlainSumRange(60000)
	}
}

func BenchmarkSumRangeChecked(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ccSink = CheckedSumRange(60000)
	}
}

func BenchmarkSievePlain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ccSink = PlainCountPrimes(200000)
	}
}

func BenchmarkSieveChecked(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ccSink = CheckedCountPrimes(200000)
	}
}

func TestCheckCostFormsAgree(t *testing.T) {
	if PlainSumRange(60000) != CheckedSumRange(60000) {
		t.Error("sum-range forms disagree")
	}
	if PlainCountPrimes(200000) != CheckedCountPrimes(200000) {
		t.Error("sieve forms disagree")
	}
}
