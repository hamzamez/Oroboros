package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// A HANDLE IS A HOST TYPE, AND A FAILED CALL'S HANDLE A NICHE (ADR 0146, P3).
// Each Win32 handle kind is C's own type, `HWND__*`, HANDLE's `void*`: a
// window is refused where a device context is wanted, and the kind's one
// operation is equality within it. A failed call answers no handle: its
// sentinel, CreateFileA's −1 or CreateWindowExA's NULL, is its result's
// niche, the none of an Option, and an `_In_opt_` parameter is an Option
// whose none is NULL. The programs, on both windows triples:
//   - TinyRetroPad's save and load (examples/native/save-win.oro): a window's
//     text written to a file and read back, 12 12 TinyRetroPad 1, its
//     segments proven in bounds across the blocks that hold their buffers (the
//     strongest postcondition of a block's spine, the book's finding 105);
//   - equality: a window equal to itself and not to another, 1 0;
//   - a missing file: CreateFileA's −1 is 'none, `none`.
//
// What must be refused, each its reason checked: a device context where a
// window is wanted; `==` between two kinds; `x.null()` where an Option is
// wanted, in the checker's words and not the lowering's.
//
// And the faults, each failing a program: the niche swapped (a missing file
// opened); `==` on a kind refused, or left as ℤ's equality; the block's tail
// unread, a conjunction with a formula kept whole, an _Out_ result's ensure
// unread (the save program refused); a case on a scalar left to the lowering.
func TestAHandleIsAHostTypeAndItsFailureANiche(t *testing.T) {
	save, err := os.ReadFile("../../examples/native/save-win.oro")
	if err != nil {
		t.Fatal(err)
	}
	const equal = "use windows/winuser\nuse windows/windef\nuse llvm as x\nuse win/print\n" +
		"fn same(a ∈ windef.HWND, b ∈ windef.HWND) (Int) { if a == b { 1 } else { 0 } }\n" +
		"fn two(w ∈ windef.HWND) {\n" +
		"    case winuser.CreateWindowExA(0, \"STATIC\", \"b\", 2^31, 0, 0, 10, 10, 'none, 'none, 'none, x.null()) {\n" +
		"        'some(v) -> { ignore(print.print_int(same(w, w))); ignore(print.print_int(same(w, v))); ignore(winuser.DestroyWindow(v)) }\n" +
		"        'none -> ignore(print.print_str(\"no window\"))\n    }\n    ignore(winuser.DestroyWindow(w))\n}\n" +
		"export fn main() {\n" +
		"    case winuser.CreateWindowExA(0, \"STATIC\", \"a\", 2^31, 0, 0, 10, 10, 'none, 'none, 'none, x.null()) {\n" +
		"        'some(w) -> two(w)\n        'none -> ignore(print.print_str(\"no window\"))\n    }\n}\n"
	const missing = "use windows/fileapi\nuse windows/handleapi\nuse llvm as x\nuse win/print\n" +
		"export fn main() {\n" +
		"    case fileapi.CreateFileA(\"no-such-file-here.txt\", 2^31, 1, x.null(), 3, 128, 'none) {\n" +
		"        'some(h) -> { ignore(handleapi.CloseHandle(h)); ignore(print.print_str(\"opened\")) }\n" +
		"        'none -> ignore(print.print_str(\"none\"))\n    }\n}\n"
	for _, tg := range []string{"windows", "windows32"} {
		for _, c := range []struct{ name, src, want string }{
			{"save and load", string(save), "12 12 TinyRetroPad 1"},
			{"equality", equal, "1 0"},
			{"a missing file", missing, "none"},
		} {
			got, err := runOnMayFail(t, c.src, tg)
			if err != nil || got != c.want {
				t.Errorf("%s on %s: want %s, got %q, %v", c.name, tg, c.want, got, err)
			}
		}
	}
	refuse := func(name, src, why string) {
		t.Helper()
		if _, err := buildOnly(t, src, "windows"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want refused (%s), got %v", name, why, err)
		}
	}
	refuse("a device context where a window is wanted",
		"use windows/winuser\nuse windows/windef\nuse win/print\nfn mix(d ∈ windef.HDC) (Int) { winuser.DestroyWindow(d) }\nexport fn main() { ignore(print.print_int(0)) }\n",
		"is `windows/windef`.HDC, but `windows/windef`.HWND is required here")
	refuse("== between two kinds",
		"use windows/windef\nuse win/print\nfn cmp(w ∈ windef.HWND, d ∈ windef.HDC) (Bool) { w == d }\nexport fn main() { ignore(print.print_int(0)) }\n",
		"a handle's one operation is equality with a handle of its own kind")
	nullOpt := strings.Replace(missing, "128, 'none)", "128, x.null())", 1)
	refuse("x.null() where an Option is wanted", nullOpt, "which is no variant")

	// the faults
	emit.PlantNicheSwapped = true
	got, err := runOnMayFail(t, missing, "windows")
	emit.PlantNicheSwapped = false
	if err != nil || got != "opened" {
		t.Errorf("with the niche swapped, the missing file should be opened, so the witness can fail: %q, %v", got, err)
	}
	for _, p := range []struct {
		name  string
		plant *bool
		src   string
	}{
		{"== on a kind refused", &emit.PlantHandleEqualityRefused, equal},
		{"== on a kind left ℤ's", &ir.PlantHandleEqAsInteger, equal},
		{"the block's tail unread", &emit.PlantProducerTailUnread, string(save)},
		{"a conjunction with a formula kept whole", &emit.PlantFormulaConjunctionWhole, string(save)},
		{"an _Out_ result's ensure unread", &emit.PlantOutEnsureUnread, string(save)},
	} {
		*p.plant = true
		_, err := buildOnly(t, p.src, "windows")
		*p.plant = false
		if err == nil {
			t.Errorf("with %s the program should be refused, so the witness can fail", p.name)
		}
	}
	emit.PlantCaseOnNoVariant = true
	_, err = buildOnly(t, nullOpt, "windows")
	emit.PlantCaseOnNoVariant = false
	if err == nil || strings.Contains(err.Error(), "which is no variant") {
		t.Errorf("with a case on a scalar left to the lowering, the refusal should be the lowering's, so the witness can fail: %v", err)
	}
}
