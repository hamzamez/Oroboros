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
	write("b-wrong", plantOnce(t, string(good), "// expect: 7 7\n", "// expect: 7 9\n"))
	write("c-unexpected", plantOnce(t, string(good), "// expect: 7 7\n", ""))
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

// A SKIP IS A CHECKED CLAIM (gauntlet-2026-11-16's successor, the harness): the
// skipped target is built and run, and must fail for the reason the skip
// states. u128-fact, refused on JavaScript for Go's `math/bits`, which its
// library calls (it is written against Go), is the control, and passes. THE PLANTED FAULTS: a skip on a case that agrees on
// every target (stale, the species that hid the JVM's unsigned word for a
// month), the control's skip given a reason that is not its failure, and a
// skip that states none.
func TestASkipIsACheckedClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("builds four cases on four targets")
	}
	read := func(name string) string {
		b, err := os.ReadFile("../../gauntlet/differential/cases/" + name + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	good, digits := read("alloc-copies"), read("u128-fact")
	const skip = "// skip: js because \"`go/math/bits`.Add64 is not bound\""
	if !strings.Contains(digits, skip+"\n") {
		t.Fatal("u128-fact no longer skips JavaScript for Go's math/bits: restate the control")
	}
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name+".oro"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a-control", digits)
	write("b-stale", "// skip: java because \"anything\"\n"+good)
	write("c-wrong-reason", plantOnce(t, digits, skip, "// skip: js because \"a reason it does not have\""))
	write("d-no-reason", "// skip: windows\n"+good)
	cmd := exec.Command("go", "run", "run.go", "-j", "4", "-cases", dir)
	cmd.Dir = "../../gauntlet/differential"
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err == nil {
		t.Fatalf("a stale skip, a wrong reason and none passed:\n%s", text)
	}
	want := []string{
		"ok   a-control",
		"FAIL b-stale — a skip is not what the target does",
		"java: skipped because \"anything\", and it builds, runs and agrees: a stale skip",
		"FAIL c-wrong-reason — a skip is not what the target does",
		"js: skipped because \"a reason it does not have\", and it fails otherwise",
		"FAIL d-no-reason — `// skip: windows` states no reason",
		"3 case(s) failed",
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
