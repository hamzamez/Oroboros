package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// A RAW REGION'S ZERO BYTE IS AT ITS LAYOUT (ADR 0097, 0117; ir.CheckStorageLayout,
// W15). A table born full takes a region with no zeroing, and the allocator
// writes the byte past its elements, a string's NUL. The width Storage asks
// is the table's type at that pass. Its final layout narrows it after: a
// realized term's table (text_v's copy of a path, os.Args) is asked at 8 bytes
// an element and laid out at 1. Its zero byte was then at 8n + 8, and the byte
// after the text was whatever the heap held, so CreateFileA read a path that
// ran on. It failed for some paths and not others, on both triples (ADR 0122's
// windows32 finding).
//
// The allocator now writes the zero at n·w + 8 and at n + 8, and W15 holds
// the final layout to {1, w}. The witness:
//   - crc32 of a file reached by paths of both parities, on windows and
//     windows32, is Go's IEEE CRC of the file;
//   - THE PLANTED FAULT asks W15 for the exact width (PlantW15Strict), and the
//     narrowed byte table is reported, so the second zero is load-bearing.
func TestARawRegionsZeroByteIsAtItsLayout(t *testing.T) {
	dir := t.TempDir()
	content := []byte("twelve bytes")
	want := fmt.Sprintf("%08x", crc32.ChecksumIEEE(content))
	var paths []string
	for _, nm := range []string{"a", "ab", "abc", "abcd", "abcde", "abcdef"} {
		p := filepath.ToSlash(filepath.Join(dir, nm))
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	src, err := os.ReadFile("../../examples/hash/crc32.oro")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"windows", "windows32"} {
		exe, err := buildAndRunNoExec(t, string(src), target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, p := range paths {
			out, err := exec.Command(exe, p).CombinedOutput()
			if got := strings.TrimSpace(string(out)); err != nil || got != want {
				t.Errorf("%s, a path of %d bytes: %q, %v; want %s", target, len(p), got, err, want)
			}
		}
	}
	ir.PlantW15Strict = true
	_, err = buildAndRunNoExec(t, string(src), "windows")
	ir.PlantW15Strict = false
	if err == nil || !strings.Contains(err.Error(), "W15") {
		t.Errorf("with W15 asking the exact width, a narrowed byte table should be reported, so the test reaches the rule; got %v", err)
	}
}
