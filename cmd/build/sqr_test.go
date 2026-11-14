package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
)

// THE SQUARE IS POLARIZATION'S (karatsuba-research.md §12): the squares of
// gauntlet/differential/cases/big-sqr.oro on windows, against Python's answers.
// THE PLANTED FAULT drops polarization's factor 2, T + the diagonal for 2T + the
// diagonal: every square of two digits or more goes wrong, the schoolbook's and
// Karatsuba's leaves alike, and only size 1, whose triangle is empty, stays
// right. Its failing also shows that x·x is planned as the square.
func TestASquareIsPolarizations(t *testing.T) {
	raw, err := os.ReadFile("../../gauntlet/differential/cases/big-sqr.oro")
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
	if len(want) != 10 {
		t.Fatalf("big-sqr.oro's expect line has %d answers, want 10: restate the witness", len(want))
	}
	prog := "use win/print\n\n" + src + "\nexport fn main() {\n"
	for n := 0; n < 10; n++ {
		prog += "    ignore(print.print_int(run(" + string(rune('0'+n)) + ")))\n"
	}
	prog += "}\n"
	out, err := buildAndRun(t, prog, "windows", "")
	if got := strings.Fields(out); err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("squares on windows: %q, %v; want %v", out, err, want)
	}
	emit.PlantSquareFault = true
	out, err = buildAndRun(t, prog, "windows", "")
	emit.PlantSquareFault = false
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out)
	if len(got) != 10 {
		t.Fatalf("the planted program printed %q", out)
	}
	for n := 0; n < 10; n++ {
		if (got[n] == want[n]) != (n == 0) {
			t.Errorf("square %d with the factor 2 dropped: %s, the right answer %s; want it wrong exactly from two digits", n, got[n], want[n])
		}
	}
}
