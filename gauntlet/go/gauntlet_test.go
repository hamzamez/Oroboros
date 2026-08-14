package gauntlet

import (
	"io"
	"testing"
)

var (
	vecA   = MakeVec(NVec, 1)
	vecB   = MakeVec(NVec, 2)
	ints   = MakeInts(NVec, 3)
	points = MakePoints(NPoints, 4)
	text   = MakeText(NWords, 5)
)

// Sinks, to stop the compiler deleting the work being measured.
var (
	sinkF float64
	sinkI int32
	sinkP Point
	sinkB BBox
	sinkM map[string]int
)

// ---------------------------------------------------------------- G1

func BenchmarkG1DotNaive(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkF = DotNaive(vecA, vecB)
	}
}

func BenchmarkG1DotHoisted(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkF = DotHoisted(vecA, vecB)
	}
}

func BenchmarkG1DotRange(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkF = DotRange(vecA, vecB)
	}
}

// ---------------------------------------------------------------- G2

func BenchmarkG2Centroid(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkP = Centroid(points)
	}
}

func BenchmarkG2CentroidStructAcc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkP = CentroidStructAcc(points)
	}
}

func BenchmarkG2Bounds(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkB = Bounds(points)
	}
}

// ---------------------------------------------------------------- G3

func BenchmarkG3SumF64(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkF = SumF64(vecA)
	}
}

func BenchmarkG3SumF64Generic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkF = SumF64Generic(vecA)
	}
}

func BenchmarkG3CountPositive(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkI = CountPositive(ints)
	}
}

func BenchmarkG3CountPositiveGeneric(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkI = CountPositiveGeneric(ints)
	}
}

// ---------------------------------------------------------------- G4

func BenchmarkG4WordCountIncr(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = WordCountIncr(text)
	}
}

func BenchmarkG4WordCountReadWrite(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = WordCountReadWrite(text)
	}
}

func BenchmarkG4WordCountGetOr(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = WordCountGetOr(text)
	}
}

// ---------------------------------------------------------------- G5

func BenchmarkG5Report(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Report(io.Discard, "label", vecA)
	}
}

func BenchmarkG5ReportFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ReportFast(io.Discard, "label", vecA)
	}
}

// ---------------------------------------------------------------- G6

func BenchmarkG6BuildOps(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkOps = BuildOps()
	}
}

var sinkOps []func(int32) int32

func BenchmarkG6RunOp(b *testing.B) {
	ops := BuildOps()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		sinkI = RunOp(ops, i%3, 7)
		i++
	}
}

func BenchmarkG6MakeScaler(b *testing.B) {
	b.ReportAllocs()
	i := int32(0)
	for b.Loop() {
		sinkFn = MakeScaler(i)
		i++
	}
}

var sinkFn func(int32) int32

// ---------------------------------------------------------------- correctness

func TestReferencesAgree(t *testing.T) {
	if got, want := DotNaive(vecA, vecB), DotHoisted(vecA, vecB); got != want {
		t.Errorf("dot: naive=%v hoisted=%v", got, want)
	}
	if got, want := DotRange(vecA, vecB), DotHoisted(vecA, vecB); got != want {
		t.Errorf("dot: range=%v hoisted=%v", got, want)
	}
	if got, want := Centroid(points), CentroidStructAcc(points); got != want {
		t.Errorf("centroid: scalar=%v structacc=%v", got, want)
	}
	if got, want := SumF64(vecA), SumF64Generic(vecA); got != want {
		t.Errorf("sum: mono=%v generic=%v", got, want)
	}
	if got, want := CountPositive(ints), CountPositiveGeneric(ints); got != want {
		t.Errorf("count: mono=%v generic=%v", got, want)
	}
	a, b2, c := WordCountIncr(text), WordCountReadWrite(text), WordCountGetOr(text)
	if len(a) != len(b2) || len(a) != len(c) {
		t.Fatalf("wordcount sizes differ: %d %d %d", len(a), len(b2), len(c))
	}
	for k, v := range a {
		if b2[k] != v || c[k] != v {
			t.Fatalf("wordcount disagree on %q", k)
		}
	}
}

func TestGeneratedDotAgreesWithHandWritten(t *testing.T) {
	if got, want := GenDot(vecA, vecB), DotNaive(vecA, vecB); got != want {
		t.Errorf("generated=%v hand-written=%v", got, want)
	}
}
