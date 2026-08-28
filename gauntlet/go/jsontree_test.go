package gauntlet

import "testing"

// The tree builder: emitted against hand-written, with the representation
// isolated from the code generation.
//
//	TreeRec  vs TreeFlat    what pointers-and-allocation costs against a flat
//	                        table plus indices — ADR 0014's question, priced
//	TreeFlat vs TreeMeasure  what OUR emitter and the refinement layer's clamped
//	                        addressing cost, with the representation held fixed
//
// The second comparison is the one requirement 5 is about. The first is the one
// the recursion argument is about, and it is the reason both are here.

func treeDoc(records int) []int {
	return TokDocInts(TokDoc(records))
}

// THE GENERATED PARSER TAKES []byte NOW, because examples/json/tree.oro declares
// `(array (int 0 255))` on its source and the Go target stores that range in a
// byte. The hand-written references still take []int, so this comparison is our
// shape against theirs — which is what the flat/recursive/clamped rows already
// are (fixpoint-2026-08-27 §13).
func treeBytes(records int) []byte { return TokDoc(records) }

func TestTreeAgree(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 20} {
		doc := treeDoc(n)
		r, f, c, g := TreeRec(doc), TreeFlat(doc), TreeFlatClamped(doc), TreeMeasure(TokDocBytes(doc))
		if r != f || r != g || r != c {
			t.Fatalf("records=%d: rec=%d flat=%d clamped=%d generated=%d", n, r, f, c, g)
		}
	}
	for _, s := range []string{"[1,2]", `{"a":1}`, "[[1],2]", `{"a":[1,2],"b":true}`,
		"[]", "{}", "[[[[1]]]]", `["a\"b"]`} {
		doc := TokDocInts([]byte(s))
		r, f, g := TreeRec(doc), TreeFlat(doc), TreeMeasure(TokDocBytes(doc))
		if r != f || r != g {
			t.Fatalf("%q: rec=%d flat=%d generated=%d", s, r, f, g)
		}
	}
}

// 20 records is ~2.6 KB and about 443 nodes, which fits the .oro's 512-node
// table with room to spare. The cap is not a benchmark parameter to tune: a
// `build` needs its length up front, so the table is sized for the largest
// document accepted rather than the one supplied, and that is a real cost of
// having no growable buffer.
const treeRecords = 20

func BenchmarkTreeRec(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeRec(doc)
	}
}

func BenchmarkTreeFlat(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeFlat(doc)
	}
}

func BenchmarkTreeGen(b *testing.B) {
	doc := treeBytes(treeRecords) // built ONCE: the conversion is not the parser
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeMeasure(doc)
	}
}

func BenchmarkTreeFlatClamped(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeFlatClamped(doc)
	}
}
