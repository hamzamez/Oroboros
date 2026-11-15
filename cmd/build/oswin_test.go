package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir/llvm"
)

// AN OUT-PARAMETER IS A CELL (ADR 0048, 0112): GetFileSizeEx(h, &size) is
// observably h ↦ (BOOL, size), the size read back from the slot the printer
// allocates. The program prints this file's size, which must be its length.
// THE PLANTED FAULT reads the cell from a fresh slot holding 0, the host's
// write lost: the size must come back 0, which shows the read is the slot's.
func TestAnOutParameterIsACell(t *testing.T) {
	self, err := filepath.Abs("oswin_test.go")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(self)
	if err != nil {
		t.Fatal(err)
	}
	src := "use win/print\nuse windows/kernel32\nuse llvm\n\n" +
		"export fn main() {\n" +
		"    h = kernel32.CreateFileA(\"" + filepath.ToSlash(self) + "\", 2147483648, 1, llvm.null(), 3, 128, llvm.null())\n" +
		"    (sz, ok) = kernel32.GetFileSizeEx(h, out kernel32.LARGE_INTEGER)\n" +
		"    ignore(print.print_int(sz))\n" +
		"    ignore(kernel32.CloseHandle(h))\n}\n"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.TrimSpace(out) != strconv.FormatInt(info.Size(), 10) {
		t.Fatalf("GetFileSizeEx's size: %q, %v; want %d", out, err, info.Size())
	}
	llvm.PlantCellUnread = true
	out, err = buildAndRun(t, src, "windows", "")
	llvm.PlantCellUnread = false
	if err != nil || strings.TrimSpace(out) != "0" {
		t.Errorf("with the cell read from a fresh slot: %q, %v; want 0", out, err)
	}
}

// THE REPAIR'S GUARD IS REACHED ONLY BY A WRONG REPAIR (ADR 0112): text_of on
// windows is ι_V ∘ r, and ι_V's domain V is decided at run time by the guard
// after r. THE PLANTED FAULT rewrites lib/os/windows.oro's repair, as it is
// read, to write U+FFFD as two of its three bytes, EF BF: the output is not in
// V, and the guard's abandon must be what the program does.
func TestTheRepairsGuardCatchesAWrongRepair(t *testing.T) {
	const src = "use win/print\nuse os\n\nexport fn main() {\n    print.print_str(os.text_of(array { 97, 128, 98 }))\n}\n"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.TrimSpace(out) != "a�b" {
		t.Fatalf("text_of(a 0x80 b): %q, %v; want a�b", out, err)
	}
	const fffd = "put(o, k, 239); put(o, k + 1, 191); put(o, k + 2, 189)\n                            again(i + 1, k + 3, i + 1, 0, 128, 191)"
	const two = "put(o, k, 239); put(o, k + 1, 191)\n                            again(i + 1, k + 2, i + 1, 0, 128, 191)"
	planted := false
	emit.PlantFragment = func(path, s string) string {
		if !strings.HasSuffix(path, "lib/os/windows.oro") {
			return s
		}
		if strings.Contains(s, fffd) {
			planted = true
		}
		return strings.Replace(s, fffd, two, 1)
	}
	defer func() { emit.PlantFragment = nil }()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	if err := run("../../targets", path, "windows", exe, "../../lib", false, false, ""); err != nil {
		t.Fatal(err)
	}
	if !planted {
		t.Fatalf("lib/os/windows.oro no longer writes U+FFFD as the test plants it: restate the witness")
	}
	got, err := exec.Command(exe).CombinedOutput()
	if err == nil || !strings.Contains(string(got), "the repair left an ill-formed subpart") {
		t.Errorf("with U+FFFD written as two bytes: %q, %v; want the guard's abandon", got, err)
	}
}
