package main

import (
	"strconv"
	"strings"
	"testing"

	"oroboros/ir/llvm"
)

// A STRUCT IS WRITTEN BY THE HOST, AND READ BY ITS LAYOUT (llvm.md §6b, ADR
// 0128 step 2): GetClientRect fills a RECT and GetLocalTime a SYSTEMTIME, each
// a struct's cell, on both Windows triples, and the program reads them by
// label. The desktop's client area is at 0, 0 with a positive size, and the
// local date is a date. The fault that reads the leaves from a fresh region
// loses the host's write, so the area comes back 0 by 0.
func TestAStructIsWrittenByTheHost(t *testing.T) {
	src := "use win/print\nuse windows/winuser\nuse windows/windef\nuse windows/sysinfoapi\nuse windows/minwinbase\n\n" +
		"export fn main() {\n" +
		"    (rc, ok) = winuser.GetClientRect(winuser.GetDesktopWindow(), out windef.RECT)\n" +
		"    ignore(print.print_int(rc.left))\n    ignore(print.print_int(rc.top))\n" +
		"    ignore(print.print_int(rc.right - rc.left))\n    ignore(print.print_int(rc.bottom - rc.top))\n" +
		"    (d, u) = sysinfoapi.GetLocalTime(out minwinbase.SYSTEMTIME)\n" +
		"    ignore(print.print_int(d.wYear))\n    ignore(print.print_int(d.wMonth))\n    ignore(print.print_int(d.wDay))\n" +
		"    ignore(u)\n}\n"
	read := func(target string) []int64 {
		out, err := buildAndRun(t, src, target, "")
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		var ns []int64
		for _, f := range strings.Fields(out) {
			n, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				t.Fatalf("%s printed %q", target, out)
			}
			ns = append(ns, n)
		}
		if len(ns) != 7 {
			t.Fatalf("%s printed %q", target, out)
		}
		return ns
	}
	for _, target := range []string{"windows", "windows32"} {
		n := read(target)
		if n[0] != 0 || n[1] != 0 || n[2] <= 0 || n[3] <= 0 {
			t.Errorf("%s: the desktop's client area is at %d, %d, %d by %d", target, n[0], n[1], n[2], n[3])
		}
		if n[4] < 2026 || n[5] < 1 || n[5] > 12 || n[6] < 1 || n[6] > 31 {
			t.Errorf("%s: the local date is %d-%d-%d", target, n[4], n[5], n[6])
		}
	}
	llvm.PlantStructUnread = true
	defer func() { llvm.PlantStructUnread = false }()
	if n := read("windows"); n[2] > 0 {
		t.Errorf("with the struct read from a fresh region the area is still %d wide: the case cannot fail", n[2])
	}
}
