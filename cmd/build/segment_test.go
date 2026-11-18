package main

import (
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A C CALL AS C WRITES IT (ADR 0131, 0132): WriteFile(h, s, len(s), &put,
// NULL) is declared over one table s and returns (put, r), the count written
// a result; a program passes a segment b[lo:hi], and the call is the variant's,
// &b[lo] and hi − lo, rewritten where the loader sees the segment or, when
// it reaches the call only by β (`put`'s t), where the reducer does. ReadFile's
// table is lent, so its segment is the buffer borrowed and threaded, rewritten
// before the state translation, its count read a result too. The program writes
// three of five bytes, seeks back, and reads them into the last three cells of
// a buffer of four: on both triples, 3 written, position 0, 3 read, and the
// buffer 0 105 33 10.
//
// The faults: the out-results left for the caller, so a call of the declared
// parameters is a call of too few; the segment rewritten after the state
// translation, so the borrowed buffer's segment is not threaded; and no variant
// derived, so a segment reaches a declaration that takes a table.
func TestASegmentIsPassed(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "seg.txt"))
	src := `use win/print
use windows/fileapi
use windows/handleapi
use windows/winnt
use llvm

fn put(h ∈ winnt.HANDLE, t ∈ Array(0..255)) {
    fileapi.WriteFile(h, t)
}

fn readback(h ∈ winnt.HANDLE) {
    var b := array { 0, 0, 0, 0 }
    (got, r) = fileapi.ReadFile(h, b[1:4])
    (got, b)
}

export fn main() {
    h = fileapi.CreateFileA("` + path + `", 3221225472, 0, llvm.null(), 2, 128, llvm.null())
    data = array { 104, 105, 33, 10, 33 }
    (n, w) = put(h, data[1:4])
    var hi := 0
    lo = fileapi.SetFilePointer(h, 0, hi, 0)
    ignore(print.print_int(n))
    ignore(print.print_int(lo))
    (g, c) = readback(h)
    ignore(print.print_int(g))
    loop i = 0 { i >= 4 -> (); else -> { ignore(print.print_int(c[i])); again(i + 1) } }
    ignore(handleapi.CloseHandle(h))
}
`
	for _, target := range []string{"windows", "windows32"} {
		if got, err := runOn(t, src, target); err != nil || got != "3 0 3 0 105 33 10" {
			t.Errorf("%s: %q, %v; want %q", target, got, err, "3 0 3 0 105 33 10")
		}
	}
	for _, c := range []struct {
		what   string
		plant  *bool
		reason string
	}{
		{"the out-results left for the caller", &core.PlantOutsUnfilled, "hands back its buffer, and its result has 1 component(s)"},
		{"the segment rewritten late", &core.PlantSegmentsLate, "ReadFile(h, #segment(b, 1, 4), out(\"int 0 4294967295\")) gives 1 cell value(s)"},
		{"no segment variant", &emit.PlantNoSegmentVariant, "ReadFile(h, #segment(b, 1, 4), out(\"int 0 4294967295\")) gives 1 cell value(s)"},
	} {
		*c.plant = true
		_, err := runOnMayFail(t, src, "windows")
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want the program refused naming %q, got %v", c.what, c.reason, err)
		}
	}
}
