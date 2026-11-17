package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A REGION IS ANSWERED BY ITS CAPACITY CLASS (ADR 0107): class(b') = class(b)
// means the block region(b) answers holds b' bytes, so a dead region serves
// every request of its class. The witness asks the heap itself: for every
// b < 2^16, HeapSize(region(b)) is at least the largest request of b's class,
// and from 2^16 the class is VirtualAlloc's, negative, its pages covering b.
// It prints the count of sizes for which the law fails.
const classSrc = "use win/print\n" +
	"use llvm/os\n" +
	"use windows/heapapi\n" +
	"\n" +
	"fn heapsize(b ∈ 0..65535) {\n" +
	"    p = os.region(b)\n" +
	"    s = heapapi.HeapSize(heapapi.GetProcessHeap(), 0, p)\n" +
	"    r = os.release(p, b)\n" +
	"    s\n" +
	"}\n" +
	"\n" +
	"fn put(m linear, c, b) { if c >= 0 && c <= 65536 { m[c] := b } else { () } }\n" +
	"\n" +
	"fn maxes(m linear) {\n" +
	"    loop b = 0 {\n" +
	"        b > 65535 -> ()\n" +
	"        else -> { put(m, os.class(b), b); again(b + 1) }\n" +
	"    }\n" +
	"}\n" +
	"\n" +
	"fn heapbad(m ∈ Array(0..65535)) (Nat) {\n" +
	"    loop b = 0, k = 0 {\n" +
	"        b > 65535 -> k\n" +
	"        else -> {\n" +
	"            c = os.class(b)\n" +
	"            s = heapsize(b)\n" +
	"            again(b + 1, case { c < 0 || c > 65536 -> k + 1; s < m[c] -> k + 1; else -> k })\n" +
	"        }\n" +
	"    }\n" +
	"}\n" +
	"\n" +
	"fn pagebad() (Nat) {\n" +
	"    loop b = 65536, k = 0 {\n" +
	"        b > 16777216 -> k\n" +
	"        else -> {\n" +
	"            c = os.class(b)\n" +
	"            again(b + 4093, case { c >= 0 -> k + 1; (0 - c) * 4096 < b -> k + 1; else -> k })\n" +
	"        }\n" +
	"    }\n" +
	"}\n" +
	"\n" +
	"export fn main() {\n" +
	"    m = { var t := array { 0 : _ ∈ 0..65536 }; maxes(t); t }\n" +
	"    print.print_str(big_str(heapbad(m) + pagebad()))\n" +
	"}\n"

// THE PLANTED FAULT is the allocator before ADR 0107 under its class: region
// asks the heap for b bytes exactly while class still names b's power of two,
// so a longer request of the class would write past its block.
func TestARegionIsAnsweredByItsCapacityClass(t *testing.T) {
	if out, err := buildAndRun(t, classSrc, "windows", ""); err != nil || strings.TrimSpace(out) != "0" {
		t.Fatalf("the capacity law on windows: %q sizes fail, %v; want 0", out, err)
	}
	planted := filepath.Join(t.TempDir(), "targets")
	if err := os.CopyFS(planted, os.DirFS("../../targets")); err != nil {
		t.Fatal(err)
	}
	osFile := filepath.Join(planted, "win32", "os.oro")
	text, err := os.ReadFile(osFile)
	if err != nil {
		t.Fatal(err)
	}
	const sized = "heapapi.HeapAlloc(heapapi.GetProcessHeap(), 8, heapclass(b))"
	exact := plantOnce(t, string(text), sized, "heapapi.HeapAlloc(heapapi.GetProcessHeap(), 8, b)")
	if err := os.WriteFile(osFile, []byte(exact), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(classSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	if err := run(planted, path, "windows", exe, "../../lib", false, false, ""); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("running the planted program: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) == "0" {
		t.Errorf("with exact-size blocks under power-of-two classes the law still holds: the witness cannot fail")
	}
}
