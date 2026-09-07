package gauntlet

import "testing"

// ADR 0020 AT AN EXPORTED BOUNDARY.
//
// The same body, twice, differing only in where the workspace comes from:
// `GenMacInto` takes a `(buffer int)` parameter, `GenMacFresh` builds one. Both
// are exported, so reduction removes neither boundary — which is the point,
// because inside a program reuse was already free (ADR 0018) and rule R closed
// the back edge (bigreuse-2026-09-02). What is left is exactly this.
var (
	wsA = make([]byte, 65536)
	wsB = make([]byte, 65536)
	wsW = make([]int, 65536)
	wsSink []int
)

func init() {
	for i := range wsA {
		wsA[i] = byte(i)
		wsB[i] = byte(i * 3)
	}
}

func BenchmarkWorkspaceParam(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		wsSink = GenMacInto(wsW, wsA, wsB)
	}
}

func BenchmarkWorkspaceFresh(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		wsSink = GenMacFresh(wsA, wsB)
	}
}

// AND THEY MUST AGREE, or the comparison is of two programs rather than of one
// boundary.
func TestWorkspaceFormsAgree(t *testing.T) {
	got := GenMacInto(make([]int, 65536), wsA, wsB)
	want := GenMacFresh(wsA, wsB)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d: parameter form %d, allocating form %d", i, got[i], want[i])
		}
	}
}
