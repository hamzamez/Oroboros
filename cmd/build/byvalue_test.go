package main

import (
	"testing"

	"oroboros/ir/llvm"
)

// A STRUCT PASSED BY VALUE IS ITS BYTES AS THE ABI PASSES THEM (ADR 0129):
// PtInRect(_In_ CONST RECT *lprc, _In_ POINT pt) takes the rectangle by its
// const T*, `in`, and the point by value, a bare set as its prototype writes
// it. Measured against clang: on x86-64 an aggregate of 1, 2, 4 or 8 bytes is
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
}
