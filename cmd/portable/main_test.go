package main

import (
	"os"
	"testing"

	"oroboros/core"
)

// W(S) = ⋂ word_T IS ANTITONE IN S — the fact ADR 0026 rests on. Adding a
// target can only shrink the meet, which is why a window fixed as a LANGUAGE
// constant had to move whenever a target was added, and why it is now a
// derived report instead. Checked on the target files as they are, including
// blas, whose C `int` is the 32-bit target the question was about.
func TestTheMeetIsAntitone(t *testing.T) {
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	within := func(a, b core.Word) bool { return b.Lo <= a.Lo && a.Hi <= b.Hi }
	chain := [][]string{
		{"go"},
		{"go", "java", "windows"},
		{"go", "java", "windows", "js"},
		{"go", "java", "windows", "js", "blas.oro"},
	}
	var prev core.Word
	for i, s := range chain {
		w, err := meet(s)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && !within(w, prev) {
			t.Errorf("W(%v) = %s is not inside W of the smaller set, %s", s, w, prev)
		}
		prev = w
	}
	want := map[int]core.Word{
		1: {Lo: -1 << 63, Hi: 1<<63 - 1},
		2: {Lo: -(1<<53 - 1), Hi: 1<<53 - 1}, // ADR 0012's window, recovered as a meet
		3: {Lo: -1 << 31, Hi: 1<<31 - 1},
	}
	for i, w := range want {
		if got, _ := meet(chain[i]); got != w {
			t.Errorf("W(%v) = %s, want %s", chain[i], got, w)
		}
	}
}
