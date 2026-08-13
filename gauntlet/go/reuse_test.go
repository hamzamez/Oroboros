package gauntlet

import "testing"

var (
	rcSmall = NewRCDict(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5, "f": 6, "g": 7, "h": 8})
	rcBig   = NewRCDict(WordCountIncr(MakeText(1<<12, 11)))
	sinkRC  *RCDict
)

func BenchmarkX1InsertStatic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = InsertStatic(rcSmall, "z", 1)
	}
}

func BenchmarkX1InsertRC(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = InsertRC(rcSmall, "z", 1)
	}
}

func BenchmarkX1InsertRCAtomic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = InsertRCAtomic(rcSmall, "z", 1)
	}
}

func BenchmarkX1InsertCopySmall(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = InsertCopy(rcSmall, "z", 1)
	}
}

func BenchmarkX1InsertCopyBig(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = InsertCopy(rcBig, "z", 1)
	}
}

func BenchmarkX2Thread3Static(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = Thread3Static(rcSmall)
	}
}

func BenchmarkX2Thread3RC(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = Thread3RC(rcSmall)
	}
}

func BenchmarkX2Thread3RCFull(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = Thread3RCFull(rcSmall)
	}
}

func BenchmarkX2Thread3RCFullAtomic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkRC = Thread3RCFullAtomic(rcSmall)
	}
}
