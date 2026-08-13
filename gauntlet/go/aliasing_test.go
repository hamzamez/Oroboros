package gauntlet

import "testing"

var (
	aliasSrc = MakeVec(NSmall, 7)
	aliasDst = make([]float64, NSmall)
	bigSrc   = MakeVec(NVec, 7)
	bigDst   = make([]float64, NVec)
	sinkSl   []float64
)

// TestAliasingChangesTheAnswer is the point of program 6: the optimization that
// carries the window in registers is only valid when dst and src are disjoint,
// and no target language can express that they are.
func TestAliasingChangesTheAnswer(t *testing.T) {
	// Not a linear ramp: a 3-point mean is the identity on one, which would make
	// both forms agree for the wrong reason and hide the hazard.
	src := []float64{1, 4, 9, 16, 25, 36, 49, 64}

	// Disjoint: both forms must agree.
	d1 := make([]float64, len(src))
	d2 := make([]float64, len(src))
	Smooth(d1, src)
	SmoothNoAlias(d2, src)
	for i := range d1 {
		if d1[i] != d2[i] {
			t.Fatalf("disjoint: forms disagree at %d: %v vs %v", i, d1[i], d2[i])
		}
	}

	// Aliased: they must NOT agree, or the test is not exercising the hazard.
	a1 := append([]float64(nil), src...)
	a2 := append([]float64(nil), src...)
	Smooth(a1, a1)
	SmoothNoAlias(a2, a2)

	same := true
	for i := range a1 {
		if a1[i] != a2[i] {
			same = false
		}
	}
	if same {
		t.Fatal("aliased: forms agree — the hazard is not being exercised")
	}
	t.Logf("aliased naive:    %v", a1)
	t.Logf("aliased register: %v", a2)
	t.Logf("disjoint (both):  %v", d1)
}

// ---------------------------------------------------------------- 6a benchmarks

func BenchmarkG6SmoothDisjoint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Smooth(bigDst, bigSrc)
	}
}

func BenchmarkG6SmoothNoAliasDisjoint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		SmoothNoAlias(bigDst, bigSrc)
	}
}

func BenchmarkG6SmoothInPlace(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Smooth(bigSrc, bigSrc)
	}
}

func BenchmarkG6SmoothFresh(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkSl = SmoothFresh(bigSrc)
	}
}

// L1-resident, where the compiler's choices are visible.
func BenchmarkG6SmallSmooth(b *testing.B) {
	for b.Loop() {
		Smooth(aliasDst, aliasSrc)
	}
}

func BenchmarkG6SmallSmoothNoAlias(b *testing.B) {
	for b.Loop() {
		SmoothNoAlias(aliasDst, aliasSrc)
	}
}

// ---------------------------------------------------------------- 6b benchmarks

var dictSmall = func() map[string]int {
	m := make(map[string]int)
	for i, w := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		m[w] = i
	}
	return m
}()

var dictBig = WordCountIncr(MakeText(1<<12, 11))

func BenchmarkG6DictInsertInPlace(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = DictInsertInPlace(dictSmall, "z", 1)
	}
}

func BenchmarkG6DictInsertCopyingSmall(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = DictInsertCopying(dictSmall, "z", 1)
	}
}

func BenchmarkG6DictInsertCopyingBig(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = DictInsertCopying(dictBig, "z", 1)
	}
}

func BenchmarkG6SliceCopyBig(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkSl = SliceCopy(bigSrc)
	}
}
