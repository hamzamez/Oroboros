package llvm

import (
	"testing"

	"oroboros/emit"
)

// A LEAF'S ALIGNMENT IS WHAT THE LAYOUT PROVES (llvm.md §6b): min(A, o & −o),
// the region's alignment and the offset's lowest set bit. The cases are the
// measured layouts' (records-research §2.1): FORMATRANGE's hdcTarget at 8
// under pack(4) on x86_64 is 4, not its pointer's 8; SYSTEMTIME's sixth WORD
// at 10 is 2; PRINTDLGA's hInstance at 34 under pack(1) on i686 is 1. The
// fault that states a leaf's natural alignment must claim 8 for the first.
func TestALeafIsAlignedAsTheLayoutProves(t *testing.T) {
	p := &printer{}
	for _, c := range []struct {
		a    int64
		lf   emit.LeafLayout
		want int64
	}{
		{4, emit.LeafLayout{Offset: 8, LL: "ptr"}, 4},
		{4, emit.LeafLayout{Offset: 0, LL: "ptr"}, 4},
		{2, emit.LeafLayout{Offset: 10, LL: "i16"}, 2},
		{1, emit.LeafLayout{Offset: 34, LL: "ptr"}, 1},
		{8, emit.LeafLayout{Offset: 44, LL: "i16"}, 4},
	} {
		if got := p.alignOf(c.a, c.lf); got != c.want {
			t.Errorf("align %d, offset %d: %d, want %d", c.a, c.lf.Offset, got, c.want)
		}
	}
	PlantLeafAlignNatural = true
	defer func() { PlantLeafAlignNatural = false }()
	if p.alignOf(4, emit.LeafLayout{Offset: 8, LL: "ptr"}) == 4 {
		t.Errorf("the natural-alignment fault still answers 4: the case cannot fail")
	}
}
