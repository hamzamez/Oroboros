package gauntlet

// THE GAUNTLET'S PAIRS, INTERLEAVED (CLAUDE.md's measurements' method,
// jsontok-2026-11-19 §3 (b)). A process slows with its age on this laptop (about
// 3 s in, any code about 3x), so two forms timed seconds apart compare two
// windows, not two forms. Here each round times the generated form and its
// hand-written reference back to back, and the order alternates (ABBA): with a
// drift s(t) slow against one round, the round's ratio is a·s(t) / b·s(t+δ), and
// a pair of rounds in opposite orders cancels the drift's first-order term,
// s(t+δ)/s(t) once in each direction. The pair's figure is the median of its
// rounds' ratios, with their range, and each form's median time.
//
//   go test -run TestInterleavedPairs -pairs=all            every pair
//   go test -run TestInterleavedPairs -pairs=TreeGen,TreeFlat -rounds=9
//
// A batch is sized once, so the reference's batch lasts about -batch (default
// 20 ms), and both forms run that many iterations: one count, so the ratio is
// a ratio of times per call.

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	pairsFlag  = flag.String("pairs", "", "interleave these pairs: `all`, or gen,hand[,gen,hand…] benchmark names")
	roundsFlag = flag.Int("rounds", 7, "rounds per pair, the order alternating")
	batchFlag  = flag.Duration("batch", 20*time.Millisecond, "the reference's time per batch")
)

// gauntletPairs are gauntlet-2026-11-16's rows, generated first.
var gauntletPairs = [][2]string{
	{"SmallDotNative", "SmallDotHand"},
	{"G2CentroidNative", "G2Centroid"},
	{"G3SumF64Native", "G3SumF64"},
	{"G4WordCountNativeInc", "G4WordCountIncr"},
	{"G4WordCountNative", "G4WordCountReadWrite"},
	{"G5NativeReport", "G5NativeReportHand"},
	{"StencilFreshNative", "StencilFreshRef"},
	{"StencilIntoNative", "StencilIntoRef"},
	{"SearchNative", "SearchRef"},
	{"SearchNativeLate", "SearchRefLate"},
	{"TokGen", "TokHandBytes"},
	{"TreeGen", "TreeFlat"},
}

var benchByName = map[string]func(*testing.B){
	"SmallDotNative": BenchmarkSmallDotNative, "SmallDotHand": BenchmarkSmallDotHand,
	"G2CentroidNative": BenchmarkG2CentroidNative, "G2Centroid": BenchmarkG2Centroid,
	"G3SumF64Native": BenchmarkG3SumF64Native, "G3SumF64": BenchmarkG3SumF64,
	"G4WordCountNativeInc": BenchmarkG4WordCountNativeInc, "G4WordCountIncr": BenchmarkG4WordCountIncr,
	"G4WordCountNative": BenchmarkG4WordCountNative, "G4WordCountReadWrite": BenchmarkG4WordCountReadWrite,
	"G5NativeReport": BenchmarkG5NativeReport, "G5NativeReportHand": BenchmarkG5NativeReportHand,
	"StencilFreshNative": BenchmarkStencilFreshNative, "StencilFreshRef": BenchmarkStencilFreshRef,
	"StencilIntoNative": BenchmarkStencilIntoNative, "StencilIntoRef": BenchmarkStencilIntoRef,
	"SearchNative": BenchmarkSearchNative, "SearchRef": BenchmarkSearchRef,
	"SearchNativeLate": BenchmarkSearchNativeLate, "SearchRefLate": BenchmarkSearchRefLate,
	"TokGen": BenchmarkTokGen, "TokHandBytes": BenchmarkTokHandBytes,
	"TreeGen": BenchmarkTreeGen, "TreeFlat": BenchmarkTreeFlat,
}

// timeAt runs f for exactly n iterations and answers ns per iteration, its
// setup excluded by the benchmark's own ResetTimer.
func timeAt(f func(*testing.B), n int) float64 {
	if err := flag.Set("test.benchtime", fmt.Sprintf("%dx", n)); err != nil {
		panic(err)
	}
	r := testing.Benchmark(f)
	return float64(r.T.Nanoseconds()) / float64(r.N)
}

// PairResult is one pair's interleaved measurement.
type PairResult struct {
	Gen, Hand     string
	N             int
	GenNs, HandNs float64   // each form's median ns per call
	Ratio, Lo, Hi float64   // the rounds' median ratio gen/hand, and its range
	Ratios        []float64 // per round, in order
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// Interleave measures gen against hand, rounds times, the order alternating.
func Interleave(gen, hand func(*testing.B), rounds int, batch time.Duration) PairResult {
	// size the batch on the reference, after one warm run of each
	timeAt(gen, 1)
	per := timeAt(hand, 1000)
	n := int(float64(batch.Nanoseconds()) / per)
	if n < 1 {
		n = 1
	}
	var gs, hs, rs []float64
	for r := 0; r < rounds; r++ {
		var g, h float64
		if r%2 == 0 {
			g = timeAt(gen, n)
			h = timeAt(hand, n)
		} else {
			h = timeAt(hand, n)
			g = timeAt(gen, n)
		}
		gs, hs, rs = append(gs, g), append(hs, h), append(rs, g/h)
	}
	s := append([]float64(nil), rs...)
	sort.Float64s(s)
	return PairResult{N: n, GenNs: median(gs), HandNs: median(hs), Ratio: median(rs), Lo: s[0], Hi: s[len(s)-1], Ratios: rs}
}

func TestInterleavedPairs(t *testing.T) {
	if *pairsFlag == "" {
		t.Skip("-pairs not given")
	}
	defer flag.Set("test.benchtime", "1s")
	pairs := gauntletPairs
	if *pairsFlag != "all" {
		names := strings.Split(*pairsFlag, ",")
		if len(names)%2 != 0 {
			t.Fatalf("-pairs takes gen,hand names in pairs, got %d names", len(names))
		}
		pairs = nil
		for i := 0; i < len(names); i += 2 {
			pairs = append(pairs, [2]string{names[i], names[i+1]})
		}
	}
	fmt.Printf("%-22s %-22s %8s %12s %12s %7s %15s\n", "generated", "hand-written", "n", "gen ns", "hand ns", "ratio", "range")
	for _, p := range pairs {
		g, h := benchByName[p[0]], benchByName[p[1]]
		if g == nil || h == nil {
			t.Fatalf("no benchmark %q or %q in benchByName", p[0], p[1])
		}
		r := Interleave(g, h, *roundsFlag, *batchFlag)
		fmt.Printf("%-22s %-22s %8d %12.1f %12.1f %7.3f %7.3f–%-7.3f\n", p[0], p[1], r.N, r.GenNs, r.HandNs, r.Ratio, r.Lo, r.Hi)
	}
}

// THE HARNESS CAN FAIL (CLAUDE.md: a harness that cannot fail proves nothing).
// A form that does twice the work must read about 2, and a form against
// itself about 1, or the ratios mean nothing.
func TestInterleaveMeasuresARatio(t *testing.T) {
	if testing.Short() {
		t.Skip("times about a second")
	}
	defer flag.Set("test.benchtime", "1s")
	one := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkD = DotRange(smallA, smallB)
		}
	}
	two := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkD = DotRange(smallA, smallB)
			sinkD += DotRange(smallB, smallA)
		}
	}
	if r := Interleave(two, one, 5, 10*time.Millisecond); r.Ratio < 1.6 || r.Ratio > 2.5 {
		t.Errorf("twice the work read %.3f (%v)", r.Ratio, r.Ratios)
	}
	if r := Interleave(one, one, 5, 10*time.Millisecond); r.Ratio < 0.85 || r.Ratio > 1.18 {
		t.Errorf("a form against itself read %.3f (%v)", r.Ratio, r.Ratios)
	}
}
