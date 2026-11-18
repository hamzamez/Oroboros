package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir/llvm"
)

// A STRUCT PASSED BY VALUE IS ITS BYTES AS THE ABI PASSES THEM (ADR 0129):
// PtInRect(_In_ CONST RECT *lprc, _In_ POINT pt) takes the rectangle by its
// const T*, the call's ref(rc) (ADR 0130), and the point by value, a bare set
// as its prototype writes it. Measured against clang: on x86-64 an aggregate of 1, 2, 4 or 8 bytes is
// the integer of its bytes, POINT an i64; on i686 its bytes are on the stack,
// `byval`. Four points, inside, below, left and the last inside corner (right
// and bottom are exclusive), on both triples. The fault passes the point as
// its copy's address, the _In_ form, which x86-64 reads as a point: the answer
// must differ.
func TestAStructIsPassedByValue(t *testing.T) {
	const src = "use win/print\nuse windows/winuser\nuse windows/windef\n\n" +
		"export fn main() {\n" +
		"    rc = record { left : 10, top : 20, right : 30, bottom : 40 }\n" +
		"    ignore(print.print_int(winuser.PtInRect(rc, record { x : 15, y : 25 })))\n" +
		"    ignore(print.print_int(winuser.PtInRect(rc, record { x : 15, y : 45 })))\n" +
		"    ignore(print.print_int(winuser.PtInRect(rc, record { x : 5, y : 25 })))\n" +
		"    ignore(print.print_int(winuser.PtInRect(rc, record { x : 29, y : 39 })))\n}\n"
	const want = "1 0 0 1"
	for _, target := range []string{"windows", "windows32"} {
		if got, err := runOn(t, src, target); err != nil || got != want {
			t.Errorf("%s: %q, %v; want %s", target, got, err, want)
		}
	}
	llvm.PlantByValueAsPointer = true
	got, err := runOn(t, src, "windows")
	llvm.PlantByValueAsPointer = false
	if err == nil && got == want {
		t.Errorf("with the point passed by its address PtInRect still answers %s: the case cannot fail", got)
	}
	// ref(x)'s fault: the rectangle under ref(rc) passed by value, its sixteen
	// bytes on i686's stack where PtInRect reads an address (ADR 0130).
	emit.PlantRefByValue = true
	_, err = runOnMayFail(t, src, "windows32")
	emit.PlantRefByValue = false
	if err == nil {
		t.Errorf("with ref(rc) passed by value the program still builds and runs: the case cannot fail")
	}
}

// runOnMayFail is runOn for a program that may fail to build or to run: either
// is its error.
func runOnMayFail(t *testing.T, src, target string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	if err := run("../../targets", path, target, exe, "../../lib", false, false, ""); err != nil {
		return "", err
	}
	b, err := exec.Command(exe).Output()
	return strings.Join(strings.Fields(string(b)), " "), err
}
