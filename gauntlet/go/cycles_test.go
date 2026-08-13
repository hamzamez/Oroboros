package gauntlet

import (
	"runtime"
	"testing"
	"time"
)

const (
	NGraph = 1 << 17 // 131072 nodes
	Deg    = 4
)

var (
	pGraph = BuildPointerGraph(NGraph, Deg)
	iGraph = BuildIndexGraph(NGraph, Deg)
	sink64 int64
)

func TestGraphsAgree(t *testing.T) {
	if a, b := SumPointerGraph(pGraph), SumIndexGraph(iGraph); a != b {
		t.Fatalf("graphs disagree: pointer=%d index=%d", a, b)
	}
}

func BenchmarkZ1SumPointerGraph(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink64 = SumPointerGraph(pGraph)
	}
}

func BenchmarkZ1SumIndexGraph(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink64 = SumIndexGraph(iGraph)
	}
}

// GC cost with each representation live. The pointer graph is 131072 objects
// with 524288 outbound pointers for the collector to trace; the index graph is
// three slices with no pointers in them at all.
func TestGCCost(t *testing.T) {
	measure := func(name string, keep func()) time.Duration {
		runtime.GC()
		runtime.GC()
		var best time.Duration = time.Hour
		for i := 0; i < 10; i++ {
			t0 := time.Now()
			runtime.GC()
			if d := time.Since(t0); d < best {
				best = d
			}
		}
		keep()
		t.Logf("%-24s best GC pause %v", name, best)
		return best
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pg := measure("pointer graph live", func() { runtime.KeepAlive(pGraph) })
	ig := measure("index graph live", func() { runtime.KeepAlive(iGraph) })
	t.Logf("pointer/index GC ratio: %.2fx", float64(pg)/float64(ig))
}

var (
	pGraphR = BuildPointerGraphRandom(NGraph, Deg, 99)
	iGraphR = BuildIndexGraphRandom(NGraph, Deg, 99)
)

func TestRandomGraphsAgree(t *testing.T) {
	if a, b := SumPointerGraph(pGraphR), SumIndexGraph(iGraphR); a != b {
		t.Fatalf("random graphs disagree: pointer=%d index=%d", a, b)
	}
}

func BenchmarkZ2SumPointerGraphRandom(b *testing.B) {
	for b.Loop() {
		sink64 = SumPointerGraph(pGraphR)
	}
}

func BenchmarkZ2SumIndexGraphRandom(b *testing.B) {
	for b.Loop() {
		sink64 = SumIndexGraph(iGraphR)
	}
}
