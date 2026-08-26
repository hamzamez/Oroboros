package gauntlet

import "testing"

// A parser is the first BRANCHY program in the gauntlet.
//
// Everything measured here before is a numeric loop — dot, centroid, smooth,
// stencil, sieve, search, report. Those are countable, predictable and
// array-walking, which is the shape Go's SSA is best at and the shape our
// emitter has been tuned against. A tokeniser is the opposite: a data-dependent
// switch per byte, unpredictable branches, and scanners whose trip count the
// input decides.
//
// It is also the first measurement of what the REFINEMENT layer costs on that
// shape. examples/json/tokenize.oro carries three compares it would not
// otherwise have — `(go.< i 0)` per token and `(go.< j 0)` in each scanner —
// because `j >= i+1` is relational and the interval analysis is not
// (json-2026-08-26 §2). checkcost-2026-08-19 priced an unproven operation on
// arithmetic-bound loops (4.54x on Go) and on memory-bound ones (1.23x), and
// found the isolated microbenchmark wrong in BOTH directions. Branch-bound is a
// third regime and this is its first number.

// Agreement first: a benchmark of three programs that disagree measures nothing.
func TestJSONTokAgree(t *testing.T) {
	for _, n := range []int{0, 1, 3, 17} {
		doc := TokDoc(n)
		ints := TokDocInts(doc)
		b, i, g := TokBytes(doc), TokInts(ints), GenTokens(ints)
		if b != i || b != g {
			t.Fatalf("records=%d: bytes=%d ints=%d generated=%d", n, b, i, g)
		}
	}
	// The malformed cases matter more than the well-formed ones, because that
	// is where the three could most easily differ.
	for _, s := range []string{"]", "{\"a\":1", "[}", "{]", "\x01", "[[[[1]]]]", ""} {
		doc := []byte(s)
		ints := TokDocInts(doc)
		b, i, g := TokBytes(doc), TokInts(ints), GenTokens(ints)
		if b != i || b != g {
			t.Fatalf("%q: bytes=%d ints=%d generated=%d", s, b, i, g)
		}
	}
}

const tokRecords = 64 // ~9 KB of JSON: bigger than L1 as []int, not as []byte

func BenchmarkTokHandBytes(b *testing.B) {
	doc := TokDoc(tokRecords)
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TokBytes(doc)
	}
}

func BenchmarkTokHandInts(b *testing.B) {
	doc := TokDocInts(TokDoc(tokRecords))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TokInts(doc)
	}
}

func BenchmarkTokGen(b *testing.B) {
	doc := TokDocInts(TokDoc(tokRecords))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = GenTokens(doc)
	}
}

var sink int
