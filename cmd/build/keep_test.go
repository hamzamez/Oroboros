package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// fillSrc is a kept definition that lends its buffer: |fill| ≥ T, called once.
const fillSrc = `use win/print

fn fill(b ∈ Array(0..255) linear, k ∈ 0..100) (Int) {
    loop i = 0, s = 0 {
        i < len(b) -> {
            if i == 0 { b[i] := k + 100 } else if i == 1 { b[i] := (k * i + 7) % 200 } else if i == 2 { b[i] := (k + i * 3) % 250 } else { b[i] := (s + k) % 256 }
            again(i + 1, (s + i * k + 1) % 1000)
        }
        else -> (k + len(b) + s) % 10
    }
}
`

// A WRITE-BORROW KEEPS ITS LENGTH (ADR 0070): its handed-back buffer is the
// argument's own storage, so len(b′) = len(b) after the call, a theorem of
// B ⊸ B ⊗ R and no declaration's `ensure`. After os.ReadAt into a table of
// four, buf[3] is defined, and it reads byte 5 of the file on Go and on
// windows. THE PLANTED FAULT forgets it in the refinement layer: the index is
// then refused, as roundtrip's buf[0] was before (p8-2026-11-15).
func TestAWriteBorrowKeepsItsLength(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "f.txt"))
	if err := os.WriteFile(path, []byte("abcdefgh"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `use os
use io
use result

export fn main() {
    (buf, read) = os.ReadAt("` + path + `", 2, out set { t ∈ Array(0..255) : len(t) == 4 })
    (n, r) = read
    io.print_line(case r { result.'ok(u) -> string_of(buf[3]); result.'err(k) -> "err" })
}
`
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, src, target, "")
		if err != nil || strings.TrimSpace(out) != "f" {
			t.Fatalf("buf[3] after os.ReadAt on %s: %q, %v; want f", target, out, err)
		}
	}
	emit.PlantInPlaceNoLength = true
	defer func() { emit.PlantInPlaceNoLength = false }()
	if _, err := buildAndRunNoExec(t, src, "windows"); err == nil || !strings.Contains(err.Error(), "indexing") {
		t.Errorf("with the length forgotten: %v; want buf[3] refused", err)
	}
}

// A LENT BUFFER IS WRITTEN BY ITS CALL (ADR 0113, Proposition C): a kept
// definition that lends b is B × X → B × R written in place, so after the call
// the caller knows of b's contents only the elements' declared set, 0..255,
// and show(2^63 - 11 + b[0] + n), n ∈ 0..9 by the result's hull, is refused,
// as it is inlined, where b[0] is the body's 150. THE PLANTED FAULT lets the
// call be pure, as its body calls nothing impure: the caller then keeps b's
// facts from before the call, its zero fill, proves the argument in show's
// set, and the program calls show outside it.
func TestALentBufferIsWrittenByItsCall(t *testing.T) {
	const src = fillSrc + `
fn show(x ∈ 0..9223372036854775807) (Bool) { print.print_int(x) }

export fn main() {
    (b, n) = {
        var b := array { 0, 0, 0, 0 }
        n = fill(b, 50)
        (b, n)
    }
    ignore(show(9223372036854775797 + b[0] + n))
}
`
	_, err := buildAndRun(t, src, "windows", "")
	if err == nil || !strings.Contains(err.Error(), "show") {
		t.Fatalf("show(2^63 - 11 + b[0] + n) after a call that wrote b: %v; want its argument refused", err)
	}
	ir.PlantLentPure = true
	defer func() { ir.PlantLentPure = false }()
	exe, err := buildAndRunNoExec(t, src, "windows")
	if err != nil {
		t.Fatalf("with the lending call pure: %v; want the stale zero fill to prove the sum", err)
	}
	// the wrong verdict the rule exists to refuse: b[0] is 150, so show is
	// called outside its set 0..2^63-1
	out, _ := exec.Command(exe).CombinedOutput()
	if v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil && v >= 0 {
		t.Errorf("the planted build printed %q; want show's argument outside 0..2^63-1", out)
	}
}

// buildAndRunNoExec builds src for target without running it: a planted
// fault's program may be wrong at run time, and only its verdict is asked.
func buildAndRunNoExec(t *testing.T, src, target string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	return exe, run("../../targets", path, target, exe, "../../lib", false, false, "")
}

// KEPT, A SEQUENCE OF CALLS TAKES THE MAXIMUM OF THEIR DEPTHS (ADR 0113,
// Proposition A): inlined, each continuation nests at the previous body's
// tails and the depths add. Six of windows' os calls in sequence, J over
// kernel32 (ADR 0112), pass the reducer's depth with K = ∅ and reduce with
// the os cell's definitions kept, and the program prints what Go's does.
// THE PLANTED FAULT is K = ∅: the build must then fail, which shows K is what
// lets it through and not a limit raised elsewhere.
func TestASequenceOfKeptCallsTakesTheMaximumDepth(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "rt.txt"))
	src := `use os
use io
use result

fn kind_name(k) {
    case k {
        os.'not_found -> "not-found"
        os.'exists -> "exists"
        os.'permission -> "permission"
        os.'other(e) -> "other"
    }
}

export fn main() {
    path = "` + path + `"
    case os.WriteFile(path, array { 104, 105, 10 }) { result.'err(k) -> io.print_line(kind_name(k)); result.'ok(_) -> io.print_line("w") }
    case os.ReadFile(path) { result.'err(k) -> io.print_line(kind_name(k)); result.'ok(src) -> io.print_line("r") }
    case os.ReadFile(path) { result.'err(k) -> io.print_line(kind_name(k)); result.'ok(src) -> io.print(os.text_of(src)) }
    io.print_line(os.Getenv("ORO_KEEP_WITNESS_UNSET"))
    (buf, read) = os.ReadAt(path, 1, out set { t ∈ Array(0..255) : len(t) <= 4 })
    (n, r) = read
    io.print_line(case r { result.'ok(u) -> string_of(48 + n % 10); result.'err(k) -> kind_name(k) })
    av = os.Args()
    io.print_int(len(av))
}
`
	want, err := buildAndRun(t, src, "go", "")
	if err != nil {
		t.Fatalf("on Go: %v", err)
	}
	if strings.TrimSpace(want) != "w\nr\nhi\n\n2\n1" {
		t.Fatalf("on Go: %q", want)
	}
	got, err := buildAndRun(t, src, "windows", "")
	if err != nil || got != want {
		t.Fatalf("on windows: %q, %v; want Go's %q", got, err, want)
	}
	// AN UNSTATED RESULT'S TYPE IS NO FACT: read as one, `open`'s success arm
	// is empty to its callers (the 0 its body returns took `failed`'s
	// 1..2^32-1 by unification), and the program is refused or wrong.
	ir.PlantTypingResult = true
	got, err = buildAndRun(t, src, "windows", "")
	ir.PlantTypingResult = false
	if err == nil && got == want {
		t.Errorf("with an unstated result typed by unification: %q; want it refused or wrong", got)
	}
	ir.PlantKeepNothing = true
	defer func() { ir.PlantKeepNothing = false }()
	if _, err := buildAndRunNoExec(t, src, "windows"); err == nil || !strings.Contains(err.Error(), "depth") && !strings.Contains(err.Error(), "step limit") {
		t.Errorf("with K = ∅: %v; want the reducer's depth or step limit passed", err)
	}
}
