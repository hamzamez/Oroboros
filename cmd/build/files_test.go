package main

import (
	"bytes"
	"os"
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
// width before the classes again, and the program must not give the file back.
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
		out, err := program(exe, in).Output()
		return string(out), err
	}
	if out, err := build(); err != nil || out != string(want) {
		t.Fatalf("8 191 bytes read and printed: %v, %d bytes back; want the file", err, len(out))
	}
	ir.PlantStorageWidthIRA = true
	defer func() { ir.PlantStorageWidthIRA = false }()
	// The fault shows as a crash (HeapFree on VirtualAlloc's region) or, since
	// the strings are the language's (strings-2026-11-16), as the file not read
	// back: either way the program does not give the file.
	if out, err := build(); err == nil && out == string(want) {
		t.Errorf("with the width taken before the classes the program gave the file back; want the fault")
	}
}

// EVERY SCALAR ENCODES AS ITS UTF-8 (strings-2026-11-16): string_of on windows
// is lang.encode, Table 3-7's rows in the language, and its agreement with
// the language's η is checked on its whole domain, the 1 112 064 scalars, a
// proof by enumeration. The oracle is the Go target, whose string_of is Go's
// own string(rune(c)). The output's length is the domain's UTF-8 length,
// Σ |enc(c)| = 4 382 592, so two empty outputs cannot agree.
func TestEveryScalarEncodes(t *testing.T) {
	if testing.Short() {
		t.Skip("prints every scalar on two targets")
	}
	const src = "use io\n\nexport fn main() {\n    loop c = 0 {\n        c > 1114111 -> 0\n" +
		"        c < 55296 -> { io.print(string_of(c)); again(c + 1) }\n" +
		"        c <= 57343 -> again(c + 1)\n" +
		"        else -> { io.print(string_of(c)); again(c + 1) }\n    }\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	outs := map[string][]byte{}
	for _, target := range []string{"windows", "go"} {
		exe := filepath.Join(dir, target+".exe")
		if err := run("../../targets", path, target, exe, "../../lib", false, false, ""); err != nil {
			t.Fatal(err)
		}
		out, err := program(exe).Output()
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		outs[target] = out
	}
	if len(outs["go"]) != 4382592 {
		t.Fatalf("Go's output is %d bytes, not the scalars' UTF-8, 4 382 592", len(outs["go"]))
	}
	if !bytes.Equal(outs["windows"], outs["go"]) {
		for i := range outs["go"] {
			if i >= len(outs["windows"]) || outs["windows"][i] != outs["go"][i] {
				t.Fatalf("windows differs from Go's encoding at byte %d", i)
			}
		}
		t.Fatalf("windows is %d bytes, Go %d", len(outs["windows"]), len(outs["go"]))
	}
}
