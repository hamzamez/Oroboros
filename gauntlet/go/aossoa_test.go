package gauntlet

import "testing"

// AoS against SoA, with the layout as the only variable. See aossoa.go.
//
//	go test -run TestLayoutsAgree
//	go test -bench=Layout -benchtime=3s -count=7

func TestLayoutsAgree(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 20} {
		doc := treeDoc(n)
		f, s, a := TreeFlat(doc), TreeSoA(doc), TreeAoS(doc)
		if f != s || f != a {
			t.Fatalf("records=%d: flat=%d soa=%d aos=%d", n, f, s, a)
		}
	}
	flat, tag, val, _, _, aos := ScanTables()
	f, s, a := ScanFlat(flat), ScanSoA(tag, val), ScanAoS(aos)
	if f != s || f != a {
		t.Fatalf("scan: flat=%d soa=%d aos=%d", f, s, a)
	}
	// A CHECKSUM THAT IS ZERO WOULD AGREE ACROSS BROKEN SCANS, so it is pinned
	// to a value computed independently of all three.
	want := 0
	for i := 0; i < scanN; i++ {
		if n := scanFill(i); n.tag == 2 {
			want += n.val
		}
	}
	if f != want || want == 0 {
		t.Fatalf("scan checksum %d, want %d (nonzero)", f, want)
	}
}

func BenchmarkLayoutTreeFlat(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeFlat(doc)
	}
}

func BenchmarkLayoutTreeSoA(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeSoA(doc)
	}
}

func BenchmarkLayoutTreeAoS(b *testing.B) {
	doc := treeDoc(treeRecords)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = TreeAoS(doc)
	}
}

func BenchmarkLayoutScanFlat(b *testing.B) {
	flat, _, _, _, _, _ := ScanTables()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = ScanFlat(flat)
	}
}

func BenchmarkLayoutScanSoA(b *testing.B) {
	_, tag, val, _, _, _ := ScanTables()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = ScanSoA(tag, val)
	}
}

func BenchmarkLayoutScanAoS(b *testing.B) {
	_, _, _, _, _, aos := ScanTables()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = ScanAoS(aos)
	}
}

// W3: random nodes of a table far past cache, all four fields — the case AoS
// exists for, and the one the first version of this comparison left out.

func TestGatherAgrees(t *testing.T) {
	flat, tag, val, kid, sib, aos := GatherTables()
	idx := GatherIdx()
	want := 0
	for _, k := range idx {
		n := gatherFill(k)
		want += n.tag + n.val + n.kid + n.sib
	}
	f, s, a := GatherFlat(flat, idx), GatherSoA(tag, val, kid, sib, idx), GatherAoS(aos, idx)
	if f != want || s != want || a != want || want == 0 {
		t.Fatalf("gather: flat=%d soa=%d aos=%d want=%d", f, s, a, want)
	}
}

func BenchmarkLayoutGatherFlat(b *testing.B) {
	flat, _, _, _, _, _ := GatherTables()
	idx := GatherIdx()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = GatherFlat(flat, idx)
	}
}

func BenchmarkLayoutGatherSoA(b *testing.B) {
	_, tag, val, kid, sib, _ := GatherTables()
	idx := GatherIdx()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = GatherSoA(tag, val, kid, sib, idx)
	}
}

func BenchmarkLayoutGatherAoS(b *testing.B) {
	_, _, _, _, _, aos := GatherTables()
	idx := GatherIdx()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = GatherAoS(aos, idx)
	}
}
