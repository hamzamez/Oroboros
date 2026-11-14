package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
)

// KARATSUBA'S PRODUCT IS THE INTEGERS' (karatsuba-research.md): the products
// of gauntlet/differential/cases/big-kara.oro, at the threshold, past it, at
// depths 1 to 4 and unbalanced within the factor of two, on windows, where
// K(ℕ*) multiplies, against the answers Python computed (the case's expect).
// THE PLANTED FAULT drops −P₂ from the combine: every product with a level is
// wrong, and the two without one stay right, 64 × 64 (s₀ = T: one leaf, no
// combine) and 63 × 64 (below T: Algorithm M's), which also shows the others
// take the Karatsuba path.
func TestKaratsubasProductIsTheIntegers(t *testing.T) {
	raw, err := os.ReadFile("../../gauntlet/differential/cases/big-kara.oro")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	var want []string
	for _, l := range strings.Split(src, "\n") {
		if w, ok := strings.CutPrefix(l, "// expect: "); ok {
			want = strings.Fields(w)
		}
	}
	if len(want) != 9 {
		t.Fatalf("big-kara.oro's expect line has %d answers, want 9: restate the witness", len(want))
	}
	prog := "use win/print\n\n" + src + "\nexport fn main() {\n"
	for n := 0; n < 9; n++ {
		prog += "    ignore(print.print_int(run(" + string(rune('0'+n)) + ")))\n"
	}
	prog += "}\n"
	out, err := buildAndRun(t, prog, "windows", "")
	if got := strings.Fields(out); err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("Karatsuba on windows: %q, %v; want %v", out, err, want)
	}
	emit.PlantKaratsubaFault = true
	out, err = buildAndRun(t, prog, "windows", "")
	emit.PlantKaratsubaFault = false
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out)
	if len(got) != 9 {
		t.Fatalf("the planted program printed %q", out)
	}
	for n := 0; n < 9; n++ {
		below := n == 0 || n == 8 // no level: one leaf, or Algorithm M
		if (got[n] == want[n]) != below {
			t.Errorf("product %d with −P₂ dropped: %s, the right answer %s; want it wrong exactly when Karatsuba's", n, got[n], want[n])
		}
	}
}
