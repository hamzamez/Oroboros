package main

import (
	"fmt"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir"
)

// A CALLBACK IS A FUNCTION OF OURS (ADR 0133, llvm.md §6c). WNDCLASSA's
// lpfnWndProc is a WNDPROC, a named claim, and its value a definition by name
// (its η-expansion) or a λ written at the field. Either is lifted, built under
// the claim's sets, and printed behind a wrapper at the claim's convention,
// stdcall on i686. Windows calls it from CreateWindowExA (WM_NCCREATE and
// WM_CREATE, answered by DefWindowProcA) and from SendMessageA, whose WM_USER
// it answers 42 + w: 49 for w = 7, on both triples.
//
// The faults: the wrapper printed cdecl, so on i686 the host's stdcall caller
// finds its stack unbalanced; and a λ that captures a value of its scope
// lifted, which must then fail other than by the closedness refusal. The
// refusals: a λ that captures, a λ of the wrong arity, a field value that is
// no function.
func TestACallbackIsAFunctionOfOurs(t *testing.T) {
	const head = "use win/print\nuse windows/winuser\nuse windows/windef\nuse windows/minwindef\nuse llvm\n\n"
	prog := func(defs, field string) string {
		return head + defs + "export fn main() {\n" +
			"    k = 1000\n" +
			"    wc = record { style : 0, lpfnWndProc : " + field + ", cbClsExtra : 0, cbWndExtra : 0, hInstance : llvm.null(), hIcon : llvm.null(),\n" +
			"        hCursor : llvm.null(), hbrBackground : llvm.null(), lpszClassName : \"OroCb\" }\n" +
			"    a = winuser.RegisterClassA(wc)\n" +
			"    h = winuser.CreateWindowExA(0, \"OroCb\", \"\", 0, 0, 0, 0, 0, llvm.null(), llvm.null(), llvm.null(), llvm.null())\n" +
			"    ignore(print.print_int(winuser.SendMessageA(h, 1024, 7, 0)))\n" +
			"    ignore(winuser.DestroyWindow(h))\n}\n"
	}
	const wndproc = "fn wndproc(h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (minwindef.LRESULT) {\n" +
		"    case { m == 1024 && w < 100 -> 42 + w; else -> winuser.DefWindowProcA(h, m, w, l) }\n}\n\n"
	byName := prog(wndproc, "wndproc")
	inline := prog("", "fn(h, m, w, l) { case { m == 1024 && w < 100 -> 42 + w; else -> winuser.DefWindowProcA(h, m, w, l) } }")
	for _, target := range []string{"windows", "windows32"} {
		for what, src := range map[string]string{"by name": byName, "a λ at the field": inline} {
			if got, err := runOn(t, src, target); err != nil || got != "49" {
				t.Errorf("%s, %s: %q, %v; want 49", target, what, got, err)
			}
		}
	}
	for _, c := range []struct{ what, field, reason string }{
		{"a λ that captures", "fn(h, m, w, l) { case { m == 1024 -> k; else -> winuser.DefWindowProcA(h, m, w, l) } }", "captures a value of its scope"},
		{"a λ of three parameters", "fn(h, m, w) { 0 }", "takes 4 parameter(s), and this λ takes 3"},
		{"a value that is no function", "0", "is a λ or a function's name"},
	} {
		if _, err := runOnMayFail(t, prog("", c.field), "windows"); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}
	// THE CONVENTION is seen in the wrapper, not in a run: measured, user32
	// restores its stack pointer itself, so a cdecl callee that leaves its 16
	// bytes on i686's stack still answers 49 (callbacks-research §5 predicted
	// a crash).
	wrapper := func(target string) string {
		if _, err := runOnMayFail(t, byName, target); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, l := range strings.Split(lastCode, "\n") {
			if strings.HasPrefix(l, "define ") && strings.Contains(l, `~cb~c"(`) {
				return l
			}
		}
		t.Fatalf("%s: no callback wrapper in the emitted code", target)
		return ""
	}
	if w := wrapper("windows32"); !strings.Contains(w, "x86_stdcallcc i32") {
		t.Errorf("windows32's wrapper is not stdcall at C's i32: %s", w)
	}
	if w := wrapper("windows"); strings.Contains(w, "x86_stdcallcc") {
		t.Errorf("windows' wrapper is stdcall, a convention only on i686: %s", w)
	}
	ir.PlantCallbackCdecl = true
	w := wrapper("windows32")
	ir.PlantCallbackCdecl = false
	if strings.Contains(w, "x86_stdcallcc") {
		t.Errorf("with the convention planted cdecl the wrapper is still stdcall: the case cannot fail")
	}
	// THE CAPTURE: with the check off the lifted λ refers to a binder it does
	// not have, and must fail otherwise, by a refusal or a panic, never build
	core.PlantCallbackOpen = true
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		_, err = runOnMayFail(t, prog("", "fn(h, m, w, l) { case { m == 1024 -> k; else -> winuser.DefWindowProcA(h, m, w, l) } }"), "windows")
		return err
	}()
	core.PlantCallbackOpen = false
	if err == nil || strings.Contains(err.Error(), "captures a value of its scope") {
		t.Errorf("with the closedness check off the capturing λ must fail otherwise, got %v", err)
	}
}

// A WINDOW RUNS ITS MESSAGE LOOP (ADR 0133 step 2): MSG is a record with a
// nested POINT, GetMessageA's an `_Out_` result (ADR 0131), and TranslateMessage
// and DispatchMessageA take it back by &msg (ADR 0130). The program posts
// WM_USER + 1 to its window, whose procedure, a λ at the field, posts WM_QUIT
// with exit code w; the loop dispatches until GetMessageA answers 0 and prints
// the quit's wParam: 7, on both triples. The procedure's w is a whole WPARAM
// narrowed by its guard to 0..99, and PostQuitMessage takes an int.
//
// The faults: MSG's pt, bound to a name after `(msg, r) = …`, passed as one
// leaf where the struct has two; and the narrowed WPARAM left unsigned at the
// `void` C function's int parameter.
func TestAWindowRunsItsMessageLoop(t *testing.T) {
	const src = "use win/print\nuse windows/winuser\nuse llvm\n\n" +
		"export fn main() {\n" +
		"    wc = record { style : 0, lpfnWndProc : fn(h, m, w, l) {\n" +
		"            case { m == 1025 && w < 100 -> { winuser.PostQuitMessage(w); 0 }; else -> winuser.DefWindowProcA(h, m, w, l) }\n" +
		"        }, cbClsExtra : 0, cbWndExtra : 0, hInstance : llvm.null(), hIcon : llvm.null(),\n" +
		"        hCursor : llvm.null(), hbrBackground : llvm.null(), lpszClassName : \"OroLoop\" }\n" +
		"    a = winuser.RegisterClassA(wc)\n" +
		"    h = winuser.CreateWindowExA(0, \"OroLoop\", \"\", 0, 0, 0, 0, 0, llvm.null(), llvm.null(), llvm.null(), llvm.null())\n" +
		"    ignore(winuser.PostMessageA(h, 1025, 7, 0))\n" +
		"    loop n = 0, quit = 0 - 1 {\n" +
		"        quit >= 0 -> ignore(print.print_int(quit))\n" +
		"        n > 1000 -> ()\n" +
		"        else -> {\n" +
		"            (msg, r) = winuser.GetMessageA(llvm.null(), 0, 0)\n" +
		"            ignore(winuser.TranslateMessage(msg))\n" +
		"            ignore(winuser.DispatchMessageA(msg))\n" +
		"            again(n + 1, if r > 0 { 0 - 1 } else { if msg.wParam < 1000 { msg.wParam } else { 1000 } })\n" +
		"        }\n" +
		"    }\n}\n"
	for _, target := range []string{"windows", "windows32"} {
		if got, err := runOn(t, src, target); err != nil || got != "7" {
			t.Errorf("%s: %q, %v; want 7", target, got, err)
		}
	}
	for _, c := range []struct {
		what   string
		plant  *bool
		reason string
	}{
		{"pt passed as one leaf", &core.PlantNestedLeafUnprojected, "is 2 values where one is wanted"},
		{"the WPARAM left unsigned", &ir.PlantCStmtUnconverted, "W5"},
	} {
		*c.plant = true
		_, err := runOnMayFail(t, src, "windows")
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want the program refused naming %q, got %v", c.what, c.reason, err)
		}
	}
}

// A WINDOW PROCEDURE HAS A STATE (ADR 0133 items 4–6, Theorem T): the state is
// a cell of main's scope, `var s := record { clicks : 0 }`, which the λ at the
// field captures and passes to the step's cell parameter, `s ∈ cell(S)`, S
// stating its set. The cell is the λ's slot, every read a load and every write
// a store, so a read after any host call sees what an activation inside it
// left. WM_USER + 1 counts a click and answers 0; WM_USER re-enters the
// procedure by sending WM_USER + 1 to itself and answers the clicks the inner
// activation left, 2; main reads 2 after both: "0 2 2", on both triples.
//
// The fault: the λ given a slot of its own, apart from the scope's, the stale
// state Theorem T excludes, so main reads 0. The refusals: a captured cell
// passed to no cell parameter, whose set nothing states; and a field read twice
// across which its fact is not kept (a load is not known to equal the last).
func TestAWindowProcedureHasAState(t *testing.T) {
	const head = "use win/print\nuse windows/winuser\nuse windows/windef\nuse windows/minwindef\nuse llvm\n\n" +
		"set S = record { clicks ∈ 0..1000 }\n\n"
	step := func(arm string) string {
		return "fn step(s ∈ cell(S), h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (minwindef.LRESULT) {\n" +
			"    c = s.clicks\n" +
			"    case {\n" +
			"        m == 1024 -> { r = winuser.SendMessageA(h, 1025, 0, 0); s.clicks }\n" +
			"        " + arm + "\n" +
			"        else -> winuser.DefWindowProcA(h, m, w, l)\n" +
			"    }\n}\n\n"
	}
	main := func(field string) string {
		return "export fn main() {\n" +
			"    var s := record { clicks : 0 }\n" +
			"    wc = record { style : 0, lpfnWndProc : " + field + ", cbClsExtra : 0, cbWndExtra : 0,\n" +
			"        hInstance : llvm.null(), hIcon : llvm.null(), hCursor : llvm.null(), hbrBackground : llvm.null(), lpszClassName : \"OroState\" }\n" +
			"    a = winuser.RegisterClassA(wc)\n" +
			"    h = winuser.CreateWindowExA(0, \"OroState\", \"\", 0, 0, 0, 0, 0, llvm.null(), llvm.null(), llvm.null(), llvm.null())\n" +
			"    ignore(print.print_int(winuser.SendMessageA(h, 1025, 0, 0)))\n" +
			"    ignore(print.print_int(winuser.SendMessageA(h, 1024, 0, 0)))\n" +
			"    ignore(print.print_int(s.clicks))\n" +
			"    ignore(winuser.DestroyWindow(h))\n}\n"
	}
	const count = "m == 1025 && c < 1000 -> { s.clicks := c + 1; 0 }"
	src := head + step(count) + main("fn(h, m, w, l) { step(s, h, m, w, l) }")
	for _, target := range []string{"windows", "windows32"} {
		if got, err := runOn(t, src, target); err != nil || got != "0 2 2" {
			t.Errorf("%s: %q, %v; want \"0 2 2\"", target, got, err)
		}
	}
	core.PlantSlotUnshared = true
	got, err := runOnMayFail(t, src, "windows")
	core.PlantSlotUnshared = false
	if err == nil && got == "0 2 2" {
		t.Errorf("with the λ's slot apart from the scope's main still reads 2: the case cannot fail")
	}
	for _, c := range []struct{ what, src, reason string }{
		{"a captured cell passed to no cell parameter",
			head + step(count) + main("fn(h, m, w, l) { case { m == 1 -> s.clicks; else -> winuser.DefWindowProcA(h, m, w, l) } }"),
			"is passed to no cell parameter"},
		{"a field read twice",
			head + step("m == 1025 && s.clicks < 1000 -> { s.clicks := s.clicks + 1; 0 }") + main("fn(h, m, w, l) { step(s, h, m, w, l) }"),
			"is not proven"},
	} {
		if _, err := runOnMayFail(t, c.src, "windows"); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}
}

// A DEFINITION'S CELL PARAMETER IS THE CALLER'S CELL (ADR 0133 item 6), an
// ordinary cell as well as a callback's: `bump(n)` reads n and writes it, and
// n holds the write afterwards, the state-passing translation's S × X → S × R
// (core/slot.go's value variant). 5 6 7 on Go, windows and windows32 (the
// differential case cell-param runs it on all four targets).
//
// The fault: the variant's last value not written back into the caller's
// cell, so the callee's writes are lost (5 5 5). The refusals: a cell outside
// the parameter's set at the call, and a write outside it in the callee.
func TestADefinitionsCellParameterIsTheCallersCell(t *testing.T) {
	prog := func(init, write string) string {
		return "use io\n\nfn bump(s ∈ cell(0..100)) (Int) {\n    c = s\n" +
			"    case { c < 100 -> { s := " + write + "; c }; else -> c }\n}\n\n" +
			"export fn main() {\n    var n := " + init + "\n    a = bump(n)\n    b = bump(n)\n" +
			"    io.print_int(a)\n    io.print_int(b)\n    io.print_int(n)\n}\n"
	}
	src := prog("5", "c + 1")
	for _, target := range []string{"go", "windows", "windows32"} {
		if got, err := runOn(t, src, target); err != nil || got != "5 6 7" {
			t.Errorf("%s: %q, %v; want \"5 6 7\"", target, got, err)
		}
	}
	core.PlantCellWriteBackDropped = true
	got, err := runOnMayFail(t, src, "go")
	core.PlantCellWriteBackDropped = false
	if err == nil && got == "5 6 7" {
		t.Errorf("with the write-back dropped the caller's cell still holds the callee's writes: the case cannot fail")
	}
	for _, c := range []struct{ what, src, reason string }{
		{"a cell outside the parameter's set", prog("200", "c + 1"), "bump's parameter s is declared in 0..100, and a call passes 200"},
		{"a write outside it", prog("5", "c + 1000"), "the claim s ∈ 0..100"},
	} {
		if _, err := runOnMayFail(t, c.src, "go"); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}
}

// A RECORD AT A LOOP'S BINDER IS CURRIED (ADR 0128, core.curryLoop): the loop's
// variable r becomes one variable per field, so its state is scalars. Without
// the currying (the planted fault PlantRecordLetWhole, which the residual's
// record erasure masks at a let, probes-2026-11-19 §3), the loop carries the
// whole record as a tuple, and its projections are indexings of it, refused.
func TestARecordLoopVariableIsCurried(t *testing.T) {
	src := "use io\n\nexport fn main() {\n    io.print_int(loop r = record { a : 0, b : 1 } {\n        r.a > 5 -> r.a * 10 + r.b\n" +
		"        else -> again(r with { a : r.a + 1 })\n    })\n}\n"
	if got, err := runOn(t, src, "go"); err != nil || got != "61" {
		t.Errorf("a record loop variable: %q, %v; want 61", got, err)
	}
	core.PlantRecordLetWhole = true
	_, err := runOnMayFail(t, src, "go")
	core.PlantRecordLetWhole = false
	if err == nil || !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("with the record bound whole the loop must be refused, its projections indexings; got %v", err)
	}
}
