package gauntlet

import "testing"

var (
	nCacheSmall = Cache{Entries: NewBoxed(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5, "f": 6, "g": 7, "h": 8}), Hits: 0}
	nCacheBig   = Cache{Entries: NewBoxed(WordCountIncr(MakeText(1<<12, 11))), Hits: 0}
	nCaches     = func() []Cache {
		cs := make([]Cache, 64)
		for i := range cs {
			cs[i] = Cache{Entries: NewBoxed(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}), Hits: i}
		}
		return cs
	}()
	nGrid = func() Grid {
		rows := make([][]float64, 128)
		for i := range rows {
			rows[i] = make([]float64, 128)
		}
		return Grid{Rows: rows}
	}()
	sinkC Cache
	sinkG Grid
)

func BenchmarkY1CopyShallowSmall(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkC = CopyShallow(nCacheSmall)
	}
}

func BenchmarkY1CopyDeepSmall(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkC = CopyDeep(nCacheSmall)
	}
}

func BenchmarkY1CopyDeepBig(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkC = CopyDeep(nCacheBig)
	}
}

func BenchmarkY2MutateDirect(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkC = MutateDirect(nCacheSmall, "z", 1)
	}
}

func BenchmarkY2MutateCOWUnshared(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkC = MutateCOW(nCacheSmall, "z", 1)
	}
}

func BenchmarkY3IndexedDirect(b *testing.B) {
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		MutateIndexedDirect(nCaches, i%64, "z", 1)
		i++
	}
}

func BenchmarkY3IndexedCOWUnshared(b *testing.B) {
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		MutateIndexedCOW(nCaches, i%64, "z", 1)
		i++
	}
}

func BenchmarkY4GridSetDirect(b *testing.B) {
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		GridSetDirect(nGrid, i%128, i%128, 1.0)
		i++
	}
}

func BenchmarkY4GridSetCopyRow(b *testing.B) {
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		sinkG = GridSetCopyRow(nGrid, i%128, i%128, 1.0)
		i++
	}
}
