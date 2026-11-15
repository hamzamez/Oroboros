package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"oroboros/core"
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

// spanSrc is a kept definition with a relational `where`, called at x, y.
func spanSrc(x, y string) string {
	return `use win/print

fn span(lo ∈ 0..1000, hi ∈ 0..1000) (Int) where lo <= hi {
    loop i = lo, s = 0 {
        i < hi -> {
            t = if i % 3 == 0 { (s + i * 3 + 1) % 1000 } else if i % 3 == 1 { (s * 7 + i) % 1000 } else { (s + 11) % 1000 }
            again(i + 1, t)
        }
        else -> (s + hi - lo) % 1000
    }
}

export fn main() {
    ignore(print.print_int(span(` + x + `, ` + y + `)))
}
`
}

// A KEPT CALL CARRIES ITS DEFINITION'S WHERE (ADR 0028, 0113): span is kept,
// so no β puts its `where` around an unfolded body, and the reducer marks the
// call with lo <= hi instantiated. span(3, 7) is inside it and runs; span(7, 3)
// is refused. THE PLANTED FAULT leaves the kept call unmarked: span(7, 3) is
// then accepted.
func TestAKeptCallCarriesItsWhere(t *testing.T) {
	out, err := buildAndRun(t, spanSrc("3", "7"), "windows", "")
	if err != nil || strings.TrimSpace(out) == "" {
		t.Fatalf("span(3, 7): %q, %v", out, err)
	}
	if _, err := buildAndRunNoExec(t, spanSrc("7", "3"), "windows"); err == nil || !strings.Contains(err.Error(), "where") {
		t.Fatalf("span(7, 3): %v; want its where refused", err)
	}
	core.PlantKeptNoWhere = true
	defer func() { core.PlantKeptNoWhere = false }()
	if _, err := buildAndRunNoExec(t, spanSrc("7", "3"), "windows"); err != nil {
		t.Errorf("with the kept call unmarked: %v; want span(7, 3) accepted", err)
	}
}

// getSrc is a kept definition whose index t[i] its body leaves unproven: an
// obligation over its parameters, i < len(t), which lifting makes a conjunct
// of its `where`, called at i.
func getSrc(i string) string {
	return `use win/print

fn get(t ∈ Array(0..255), i ∈ 0..1000) (Int) {
    loop j = 0, s = 0 {
        j < 8 -> {
            u = if j % 3 == 0 { (s + j * 3 + 1) % 1000 } else if j % 3 == 1 { (s * 7 + j) % 1000 } else { (s + 11) % 1000 }
            again(j + 1, u)
        }
        else -> (s + t[i]) % 1000
    }
}

export fn main() {
    t = array { 1, 2, 3, 4, 5 }
    ignore(print.print_int(get(t, ` + i + `)))
}
`
}

// A LIFTED PRECONDITION IS DECIDED AT THE CALL (Theorem 4, ADR 0113): get's
// t[i] is proven by no fact of its body, and it is a formula over its
// parameters, so it becomes a conjunct of get's `where`, i < len(t), assumed in
// the body and decided at each call by the kept call's mark. get(t, 2) on a
// table of five runs; get(t, 7) is refused. THE PLANTED FAULT lifts into the
// body's assumptions and not into the call's mark: get(t, 7) is then accepted.
func TestALiftedPreconditionIsDecidedAtTheCall(t *testing.T) {
	out, err := buildAndRun(t, getSrc("2"), "windows", "")
	if err != nil || strings.TrimSpace(out) == "" {
		t.Fatalf("get(t, 2): %q, %v", out, err)
	}
	if _, err := buildAndRunNoExec(t, getSrc("7"), "windows"); err == nil {
		t.Fatalf("get(t, 7) on a table of five: accepted; want it refused")
	}
	ir.PlantLiftUnmarked = true
	defer func() { ir.PlantLiftUnmarked = false }()
	if _, err := buildAndRunNoExec(t, getSrc("7"), "windows"); err != nil {
		t.Errorf("with the lift unmarked at the call: %v; want get(t, 7) accepted", err)
	}
}

// mkSrc is a kept definition returning a table over 0..hi, indexed at k.
func mkSrc(hi, k string) string {
	return `use win/print

fn mk(n ∈ 2..1000) {
    var c := array { 0 : _ ∈ 0..` + hi + ` }
    loop i = 0 {
        i >= len(c) -> c
        else -> {
            c[i] := if i % 3 == 0 { (i * 7 + 1) % 200 } else if i % 3 == 1 { (i * 5 + 3) % 200 } else if i % 5 == 2 { (i * i + 9) % 200 } else { (i + 11) % 200 }
            again(i + 1)
        }
    }
}

export fn main() {
    c = mk(10)
    ignore(print.print_int(c[` + k + `]))
}
`
}

// A KEPT RESULT CARRIES ITS POSTCONDITION (Houdini, ADR 0113, p8-research §9):
// mk(n) over 0..n - 1 has len(r) = n at its return, decided there, and assumed
// at the call, so c[9] after mk(10) is defined. Over 0..n - 2 only len(r) ≤ n
// holds, and c[9] is refused. THE PLANTED FAULT accepts every candidate
// unchecked: the false len(r) = n then proves c[9], past the table's end.
func TestAKeptResultCarriesItsPostcondition(t *testing.T) {
	if out, err := buildAndRun(t, mkSrc("n - 1", "9"), "windows", ""); err != nil || strings.TrimSpace(out) == "" {
		t.Fatalf("c[9] after mk(10) over 0..n - 1: %q, %v", out, err)
	}
	if !ir.LastKept()["mk"] {
		t.Errorf("mk over 0..n - 1: not kept; kept %v", ir.LastKept())
	}
	if _, err := buildAndRunNoExec(t, mkSrc("n - 2", "9"), "windows"); err == nil {
		t.Fatalf("c[9] after mk(10) over 0..n - 2: accepted; want the index refused")
	}
	emit.PlantPostUnchecked = true
	defer func() { emit.PlantPostUnchecked = false }()
	if _, err := buildAndRunNoExec(t, mkSrc("n - 2", "9"), "windows"); err != nil {
		t.Errorf("with every candidate unchecked: %v; want c[9] accepted", err)
	}
}

// AN INTEGER PARAMETER WIDER THAN THE WORDS IS THE WORD'S AT ITS INSTANCE
// (Proposition D, ADR 0113): print_int(n ∈ Int) claims ℤ, and its instance
// (print_int, W) has n ∈ W, so its body, written for the word, is proven and
// it is kept where its calls pass word values. A hash in [0, 2^64) is held in
// the unsigned word (ADR 0079), outside W: that call is not the instance's
// (CheckCallSets and ADR 0033's conversion both refuse it), so P4 inlines
// print_int, and windows prints the hash as Go computes it. This is the rule's
// completeness, the soundness being those two checks' own witnesses: THE
// PLANTED FAULT leaves ℤ unnarrowed, and print_int is no longer kept.
func TestAnUnboundedParameterIsTheWordsAtItsInstance(t *testing.T) {
	const word = "use win/print\n\nexport fn main() {\n    ignore(print.print_int(len(array { 1, 2, 3 }) * 7))\n}\n"
	if out, err := buildAndRun(t, word, "windows", ""); err != nil || strings.TrimSpace(out) != "21" {
		t.Fatalf("print_int(21): %q, %v", out, err)
	}
	if !ir.LastKept()["win/print.print-int"] {
		t.Errorf("print_int at a word value: not kept; kept %v", ir.LastKept())
	}
	const src = `use win/print

export fn main() {
    h = loop i = 0, h = 14695981039346656037 {
        i < 3 -> again(i + 1, (h * 1099511628211 + 97) mod 2^64)
        else -> h
    }
    ignore(print.print_int(h))
}
`
	h := uint64(14695981039346656037)
	for i := 0; i < 3; i++ {
		h = h*1099511628211 + 97
	}
	if got, err := buildAndRun(t, src, "windows", ""); err != nil || strings.TrimSpace(got) != strconv.FormatUint(h, 10) {
		t.Fatalf("print_int of a hash: %q, %v; want %d", got, err, h)
	}
	ir.PlantNoWordInstance = true
	defer func() { ir.PlantNoWordInstance = false }()
	if _, err := buildAndRun(t, word, "windows", ""); err != nil {
		t.Fatal(err)
	}
	if ir.LastKept()["win/print.print-int"] {
		t.Errorf("with ℤ unnarrowed: print_int kept; want it inlined, its body unproven under ℤ")
	}
}

// A KEPT CALL ON LITERALS IS ITS VALUE (ADR 0009, 0113): width(lo, hi) is
// kept, and width(0, 300) is a closed term, which staging evaluates to 2, so
// t[width(0, 300)] into a table of three is proven at the call as it was
// inlined. THE PLANTED FAULT leaves the call opaque: its caller knows only its
// result's hull, 1..8, the index is refused with K, and P4's net empties K.
func TestAKeptCallOnLiteralsIsItsValue(t *testing.T) {
	const src = `use win/print

fn width(lo, hi) (1..8) {
    case {
        lo >= 0 && hi <= 255 -> 1
        lo >= 0 && hi <= 65535 -> 2
        lo >= 0 && hi <= 16777215 -> 3
        lo >= 0 && hi <= 4294967295 -> 4
        lo >= 0 && hi <= 1099511627775 -> 5
        lo >= 0 && hi <= 281474976710655 -> 6
        lo >= 0 && hi <= 72057594037927935 -> 7
        lo >= 0 -> 8
        lo >= -128 && hi <= 127 -> 1
        lo >= -32768 && hi <= 32767 -> 2
        else -> 4
    }
}

export fn main() {
    t = array { 10, 20, 30 }
    ignore(print.print_int(t[width(0, 300)]))
}
`
	if out, err := buildAndRun(t, src, "windows", ""); err != nil || strings.TrimSpace(out) != "30" {
		t.Fatalf("t[width(0, 300)]: %q, %v; want 30", out, err)
	}
	if !ir.LastKept()["width"] {
		t.Fatalf("width: not kept; kept %v", ir.LastKept())
	}
	core.PlantKeptNoFold = true
	defer func() { core.PlantKeptNoFold = false }()
	if _, err := buildAndRunNoExec(t, src, "windows"); err != nil {
		t.Fatal(err)
	}
	if ir.LastKept()["width"] {
		t.Errorf("with the call on literals opaque: width kept; want the net to inline it")
	}
}
