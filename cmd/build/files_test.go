package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"oroboros/ir"
)

// A TABLE'S STORAGE IS ITS FINAL LAYOUT, across a kept function's boundary
// (files-2026-11-16). os.ReadFile on windows is `readfile`, kept as a function
// (ADR 0113): it allocated its buffer by the type it had before Theorem D′'s
// classes, 8 bytes an element, while the printer held it in bytes and the
// caller released it as bytes. At 8 191 bytes the allocation, 8 191·8 + 8 =
// 2¹⁶, was VirtualAlloc's, the release HeapFree's, and the process faulted:
// freq and jsonfmt crashed on any input past 8 KiB. THE PLANTED FAULT takes the
// width before the classes again, and the program must fail on that input.
func TestAFileReadIsReleasedByItsOwnLayout(t *testing.T) {
	const src = "use os\nuse io\nuse result\n\nexport fn main() {\n" +
		"    av = os.Args()\n" +
		"    if len(av) < 2 { io.print_line(\"usage\") } else {\n" +
		"        case os.ReadFile(av[1]) {\n" +
		"            result.'ok(src) -> io.print(os.text_of(src))\n" +
		"            result.'err(k) -> io.print_line(\"cannot read\")\n" +
		"        }\n" +
		"    }\n}\n"
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	want := bytes.Repeat([]byte("abcdefg\n"), 1024)[:8191]
	if err := os.WriteFile(in, want, 0o644); err != nil {
		t.Fatal(err)
	}
	build := func() (string, error) {
		path := filepath.Join(dir, "p.oro")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(dir, "p.exe")
		if err := run("../../targets", path, "windows", exe, "../../lib", false, false, ""); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(exe, in).Output()
		return string(out), err
	}
	if out, err := build(); err != nil || out != string(want) {
		t.Fatalf("8 191 bytes read and printed: %v, %d bytes back; want the file", err, len(out))
	}
	ir.PlantStorageWidthIRA = true
	defer func() { ir.PlantStorageWidthIRA = false }()
	if _, err := build(); err == nil {
		t.Errorf("with the width taken before the classes the program ran; want the fault, the release by another allocator")
	}
}
