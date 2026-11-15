package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
)

// PRINTING IS A TREE OF DIVISIONS (radix-research.md, ADR 0111): the numerals
// of gauntlet/differential/cases/big-render.oro on windows, 32 to 158 digits,
// each above render's threshold, against Python's answers.
//
// THE FIRST PLANTED FAULT drops D6, the in-place add-back. On random digits it
// is needed with probability about 2/B, so the witness computes where it is
// needed: a node c·P_j − 1 is c·v − 2^s normalized, P_j's low 18·2^j bits are
// 0, so the borrow dies below the window's top three words and the last step's
// q̂ is q + 1. Inputs 1 (10^620 − 1), 7 (β^128 − 1) and 10 (1 − 10^700, its
// magnitude 10^700 − 1) have nodes of that shape, 8, 32 and 9 add-backs (a
// model of the tree with kq4's estimate counted them), and no other input has
// one: exactly those three must go wrong.
func TestPrintingIsATreeOfDivisions(t *testing.T) {
	raw, err := os.ReadFile("../../gauntlet/differential/cases/big-render.oro")
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
	const n = 11
	if len(want) != n {
		t.Fatalf("big-render.oro's expect line has %d answers, want %d: restate the witness", len(want), n)
	}
	prog := "use win/print\n\n" + src + "\nexport fn main() {\n"
	for i := 0; i < n; i++ {
		prog += "    print.print_str(run(" + itoa(i) + "))\n"
	}
	prog += "}\n"
	out, err := buildAndRun(t, prog, "windows", "")
	if got := strings.Fields(out); err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("numerals on windows: %v; %d fields, want the %d of the expect line", err, len(got), n)
	}

	emit.PlantTreeNoAddBack = true
	out, err = buildAndRun(t, prog, "windows", "")
	emit.PlantTreeNoAddBack = false
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out)
	if len(got) != n {
		t.Fatalf("the planted program printed %d fields", len(got))
	}
	for i := 0; i < n; i++ {
		if (got[i] == want[i]) == (i == 1 || i == 7 || i == 10) {
			t.Errorf("numeral %d with D6 dropped: right %v; want it wrong exactly at inputs 1, 7 and 10", i, got[i] == want[i])
		}
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
