package gauntlet

import (
	"fmt"
	"os"
	"testing"
)

// GAUNTLET PROGRAM 5 ON THE NATIVE GO TARGET.
//
// gauntlet-2026-09-07 found that `report` was still benchmarked on the RETIRED
// portable layer — the last of the three that result named, the other two
// having had native benchmarks all along — and that it was not a one-line fix:
// the native source exports `main`, which takes nothing and BUILDS its data,
// where the portable one takes an array and a label. So the benchmark is what
// had to change, and this is it.
//
// The hand-written reference therefore constructs too. That is the honest
// comparison: `main` cannot take a parameter, and a program that could not
// build data at all until 2026-08-15 is exactly what construction.md is about.
//
// What the number is for is unchanged and is not the clock — `fmt` dominates
// both sides. It is whether the PURE part still fused: an effect between two
// loops must not stop `dot` collapsing into one pass with no intermediate
// vector, which `ReportAllocs` is what shows.

func reportNativeRef() float64 {
	xs := make([]float64, 1000)
	for i := range xs {
		xs[i] = float64(i)
	}
	fmt.Println("report")
	fmt.Println(len(xs))
	var acc float64
	for i := range xs {
		acc += xs[i] * xs[i]
	}
	fmt.Println(acc)
	return acc
}

// Behavioural first, numerical second: three lines, in order, once each, and
// byte-identical to what a person would write.
func TestNativeReportMatchesHandWritten(t *testing.T) {
	got := capture(t, func() { GenMain() })
	want := capture(t, func() { reportNativeRef() })
	if got != want {
		t.Errorf("output differs\n got: %q\nwant: %q", got, want)
	}
	var acc float64
	for i := 0; i < 1000; i++ {
		acc += float64(i) * float64(i)
	}
	if v := GenMain(); v != acc {
		t.Errorf("value %v, want %v", v, acc)
	}
}

func BenchmarkG5NativeReport(b *testing.B) {
	b.ReportAllocs()
	restore := silenceStdout(b)
	defer restore()
	for i := 0; i < b.N; i++ {
		sinkF = GenMain()
	}
}

func BenchmarkG5NativeReportHand(b *testing.B) {
	b.ReportAllocs()
	restore := silenceStdout(b)
	defer restore()
	for i := 0; i < b.N; i++ {
		sinkF = reportNativeRef()
	}
}

// The three lines are the point of the program and noise in a benchmark, so
// stdout goes to the null device for the duration.
func silenceStdout(b *testing.B) func() {
	b.Helper()
	old := os.Stdout
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Skip(err)
	}
	os.Stdout = null
	return func() { os.Stdout = old; null.Close() }
}
