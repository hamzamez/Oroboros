package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir/llvm"
)

// A FAMILY IS ONE DECLARATION PER MEMBER (ADR 0084's P5, ADR 0141):
// SendMessageA's messages, each a declaration at its constant, its lParam
// pointer the integer carrying it. TinyRetroPad's status line asks a Rich Edit
// control for its caret, the caret's line and the line's start, and prints
// "Ln 2, Col 5", the line count, and "Ln 2, Col 6" after a replacement, on both
// triples. A constant outside its prototype's set is refused, naming it. The
// faults: a constant typed as the word, which on i686 makes the stdcall frame
// wider than SendMessageA pops; and the family untyped, every member refused.
func TestAFamilyIsOneDeclarationPerMember(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs windows programs")
	}
	src, err := os.ReadFile("../../examples/native/status-win.oro")
	if err != nil {
		t.Fatal(err)
	}
	want := "Ln 2, Col 5 2 Ln 2, Col 6"
	for _, target := range []string{"windows", "windows32"} {
		if got, err := runOnMayFail(t, string(src), target); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", target, got, err, want)
		}
	}
	llvm.PlantFamilyConstWord = true
	got, err := runOnMayFail(t, string(src), "windows32")
	llvm.PlantFamilyConstWord = false
	if err == nil && got == want {
		t.Errorf("with a constant typed as the word, windows32 should not run right; it printed %q", got)
	}
	emit.PlantFamilyUntyped = true
	_, err = buildOnly(t, string(src), "windows")
	emit.PlantFamilyUntyped = false
	if err == nil || !strings.Contains(err.Error(), "member of SendMessageA's family") {
		t.Errorf("with the family untyped every member should be refused naming it, got %v", err)
	}
	// a constant outside the prototype's set: UINT's 2^32
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "windows"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "module windows/badmsg {\n    use windows/windef\n    use windows/minwindef\n" +
		"    fn Bad(w ∈ windef.HWND) (r ∈ minwindef.LRESULT) { host expr call SendMessageA(w, 4294967296, 0, 0) stdcall lib \"user32\" }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "windows", "badmsg.oro"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := emit.LoadTargetLayers("windows", []string{dir, "../../targets"}); err == nil || !strings.Contains(err.Error(), "not in SendMessageA's parameter set") {
		t.Errorf("a constant outside UINT should be refused naming the set, got %v", err)
	}
}
