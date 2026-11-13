package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// THE PARALLEL DIFFERENTIAL STILL FAILS (differential-2026-11-13): the runner
// takes its cases `-j` at a time, so a verdict must not depend on the order
// they finish in, and every failure must still count. Three cases run four at
// a time: one right, one whose every target agrees on an answer it does not
// expect (THE PLANTED FAULT), and one with no `// expect:`. The run must fail,
// name exactly the two, and report in the cases' order.
func TestTheParallelDifferentialStillFails(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three cases on four targets")
	}
	good, err := os.ReadFile("../../gauntlet/differential/cases/alloc-copies.oro")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(good), "// expect: 7 7\n") {
		t.Fatal("alloc-copies no longer expects 7 7: restate the planted fault")
	}
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name+".oro"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a-right", string(good))
	write("b-wrong", strings.Replace(string(good), "// expect: 7 7\n", "// expect: 7 9\n", 1))
	write("c-unexpected", strings.Replace(string(good), "// expect: 7 7\n", "", 1))
	cmd := exec.Command("go", "run", "run.go", "-j", "4", "-cases", dir)
	cmd.Dir = "../../gauntlet/differential"
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err == nil {
		t.Fatalf("a wrong answer and a missing expectation passed:\n%s", text)
	}
	want := []string{
		"ok   a-right",
		"FAIL b-wrong — every target agrees, and all of them are wrong",
		"FAIL c-unexpected — no `// expect:` line",
		"2 case(s) failed",
	}
	at := 0
	for _, w := range want {
		i := strings.Index(text[at:], w)
		if i < 0 {
			t.Fatalf("want %q after offset %d, in the cases' order:\n%s", w, at, text)
		}
		at += i + len(w)
	}
}
