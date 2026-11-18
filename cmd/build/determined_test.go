package main

import (
	"strconv"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/llvm"
)

// numbers runs a program and reads the integers it prints, n of them.
func numbers(t *testing.T, src, target string, n int) []int64 {
	t.Helper()
	out, err := buildAndRun(t, src, target, "")
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	var ns []int64
	for _, f := range strings.Fields(out) {
		v, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			t.Fatalf("%s printed %q", target, out)
		}
		ns = append(ns, v)
	}
	if len(ns) != n {
		t.Fatalf("%s printed %q", target, out)
	}
	return ns
}

// A FIELD THE DECLARATION DETERMINES IS THE DECLARATION'S (Theorem F, ADR 0128
// item 12): MEMORYSTATUSEX's dwLength and PRINTDLGA's lStructSize are the
// layout's size, written by the layout, absent from the program's record, and
// PRINTDLGA is 66 bytes on i686 by the data model's pack(1). The hosts check
// both: GlobalMemoryStatusEx refuses a wrong dwLength, and PrintDlgA a wrong
// lStructSize with CommDlgExtendedError's CDERR_STRUCTSIZE, 1; with
// PD_RETURNDEFAULT it shows no dialog, and answers a printer or
// PDERR_NODEFAULTPRN. The faults: the size unwritten, and the layout natural.
func TestADeterminedFieldIsTheDeclarations(t *testing.T) {
	const src = "use win/print\nuse windows/sysinfoapi\nuse windows/commdlg\n\n" +
		"export fn main() {\n" +
		"    (m, ok) = sysinfoapi.GlobalMemoryStatusEx(out sysinfoapi.MEMORYSTATUSEX)\n" +
		"    ignore(print.print_int(ok))\n    ignore(print.print_int(m.dwMemoryLoad))\n" +
		"    ignore(print.print_int(m.ullTotalPhys / 1048576))\n" +
		"    var pd := zero(commdlg.PRINTDLGA) with { Flags : 1024 }\n" +
		"    r = commdlg.PrintDlgA(pd)\n" +
		"    ignore(print.print_int(r))\n    ignore(print.print_int(commdlg.CommDlgExtendedError()))\n}\n"
	for _, target := range []string{"windows", "windows32"} {
		n := numbers(t, src, target, 5)
		if n[0] != 1 || n[1] < 0 || n[1] > 100 || n[2] <= 0 {
			t.Errorf("%s: GlobalMemoryStatusEx answered %d, load %d, %d MB", target, n[0], n[1], n[2])
		}
		if n[4] == 1 || n[3] == 0 && n[4] != 0x1008 {
			t.Errorf("%s: PrintDlgA answered %d, error %#x: lStructSize or the layout is not the host's", target, n[3], n[4])
		}
	}
	llvm.PlantNoDeterminedSize = true
	n := numbers(t, src, "windows", 5)
	llvm.PlantNoDeterminedSize = false
	if n[0] != 0 {
		t.Errorf("with dwLength unwritten GlobalMemoryStatusEx still answers %d: the case cannot fail", n[0])
	}
	emit.PlantNaturalLayout = true
	n = numbers(t, src, "windows32", 5)
	emit.PlantNaturalLayout = false
	if n[3] != 0 || n[4] != 1 {
		t.Errorf("with PRINTDLGA laid out naturally on i686 PrintDlgA answers %d, error %#x, not CDERR_STRUCTSIZE: "+
			"the case cannot fail", n[3], n[4])
	}
}

// A CELL DECLARED AFTER AN OUT-CALL IS A CELL: the out-call is translated by
// the pass for calls outside a scope, and its continuation holds the `var`
// scope the scope pass translated first. Twice-translated, that scope's call
// was refused, "#s2 is neither" a cell; and the two passes' fresh names, each
// counted from 1, captured an argument. Each fault must fail the case.
func TestACellAfterAnOutCallIsACell(t *testing.T) {
	const src = "use win/print\nuse windows/winuser\nuse windows/windef\nuse windows/sysinfoapi\nuse windows/minwinbase\n\n" +
		"export fn main() {\n" +
		"    (d, u) = sysinfoapi.GetLocalTime(out minwinbase.SYSTEMTIME)\n" +
		"    var rc := zero(windef.RECT)\n" +
		"    w = winuser.GetDesktopWindow()\n" +
		"    ok = winuser.GetClientRect(w, rc)\n" +
		"    ignore(print.print_int(rc.right - rc.left))\n    ignore(print.print_int(d.wYear))\n    ignore(u)\n}\n"
	if n := numbers(t, src, "windows", 2); n[0] <= 0 || n[1] < 2026 {
		t.Errorf("the desktop is %d wide in %d", n[0], n[1])
	}
	for _, c := range []struct {
		name  string
		plant *bool
	}{
		{"the scope's call translated twice", &core.PlantCellCallTwice},
		{"the two passes' names clashing", &core.PlantCellNamesClash},
	} {
		*c.plant = true
		out, err := buildAndRunNoExec(t, src, "windows")
		*c.plant = false
		if err == nil && !strings.Contains(out, "error") {
			// built: it must at least not answer the desktop's width
			t.Logf("%s built: %s", c.name, out)
			t.Errorf("%s still builds: the case cannot fail", c.name)
		}
	}
}

// A C STRING FIELD IS LENT FOR THE CALL (Theorem P′, llvm.md §6b): WNDCLASSA's
// lpszClassName is the string's bytes, and the declaration's
// `where nulfree(wc.lpszClassName)` is decided on the leaf the projection
// reads. RegisterClassA registers the class under the program's name, which
// UnregisterClassA finds. The faults: the string passed with its length first,
// so the class is registered under another name; and the `where`'s projection
// unread, so the obligation is undecided and the program refused. A name that
// may hold a NUL is refused, saying so.
func TestACStringFieldIsLent(t *testing.T) {
	const head = "use win/print\nuse windows/winuser\nuse windows/sysinfoapi\nuse windows/minwinbase\nuse llvm\n\n"
	src := func(name string) string {
		return head + "export fn main() {\n" +
			"    (d, u) = sysinfoapi.GetLocalTime(out minwinbase.SYSTEMTIME)\n    ignore(u)\n" +
			"    wc = record { style : 0, cbClsExtra : 0, cbWndExtra : 0, hInstance : llvm.null(), hIcon : llvm.null(),\n" +
			"        hCursor : llvm.null(), hbrBackground : llvm.null(), lpszClassName : " + name + " }\n" +
			"    a = winuser.RegisterClassA(wc)\n" +
			"    ignore(print.print_int(if a > 0 { 1 } else { 0 }))\n" +
			"    ignore(print.print_int(winuser.UnregisterClassA(\"OroTestClass\", llvm.null())))\n}\n"
	}
	ok := src(`"OroTestClass"`)
	for _, target := range []string{"windows", "windows32"} {
		if n := numbers(t, ok, target, 2); n[0] != 1 || n[1] != 1 {
			t.Errorf("%s: RegisterClassA %d, UnregisterClassA %d", target, n[0], n[1])
		}
	}
	llvm.PlantCStringHeader = true
	n := numbers(t, ok, "windows", 2)
	llvm.PlantCStringHeader = false
	if n[1] != 0 {
		t.Errorf("with the string's length passed first the class is still found: the case cannot fail")
	}
	emit.PlantWhereProjectionUnread = true
	_, err := buildAndRunNoExec(t, ok, "windows")
	emit.PlantWhereProjectionUnread = false
	if err == nil {
		t.Errorf("with the where's projection unread the program still builds: the case cannot fail")
	}
	// a name whose scalar may be U+0000: string_of(d.wDay - 1) is "\x00" on the first
	if _, err := buildAndRunNoExec(t, src("string_of(d.wMonth % 128)"), "windows"); err == nil ||
		!strings.Contains(err.Error(), "nulfree") {
		t.Errorf("a class name that may hold a NUL: %v, want a refusal naming nulfree", err)
	}
}
