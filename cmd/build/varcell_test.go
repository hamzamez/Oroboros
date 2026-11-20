package main

import (
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir"
)

// A SCALAR CELL AT A HOST'S CELL OF A RANGE (ADR 0048, 0126): SetFilePointer's
// lpDistanceToMoveHigh is a cell of LONG, −2^31..2^31−1, `_Inout_`: the high half
// of the distance in, the high half of the new position out (ADR 0131 keeps
// `_Inout_` a cell). A program's `var hi := 0`, a cell of ℤ, fits it iff its
// value is in the LONG at the call, a claim decided as any is, and the LONG is
// within ℤ, always; the value read back is known in the LONG. The program
// writes five bytes to a file, WriteFile's count a result (ADR 0131) and its
// table one segment (ADR 0132), and seeks to its end: 5, high half 0. Run on
// both triples: once, then in a loop, where the cell is a loop variable whose
// claim the IR's intervals prove from its read-back set; a value outside the
// LONG is refused naming the claim.
//
// THE UNIT, (() (fn () b)) = b, was refused at lowering, so a claim in
// windows' `main` loop, followed by ExitProcess, was left to the refinement
// layer alone, and a map literal read at a run-time key in `main` did not
// build (mapvalue-2026-11-18): 1 is terminal, and what holds it binds nothing.
//
// The faults: the claim dropped, so a value outside the LONG reaches the host;
// and the unit left uneliminated, so the literal read in `main`'s loop is
// refused.
func TestAScalarCellIsTheHostsCell(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "seek.txt"))
	const head = "use win/print\nuse windows/fileapi\nuse windows/handleapi\nuse llvm\n\n"
	// GENERIC_READ | GENERIC_WRITE, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL
	open := "    expect 'some(h) = fileapi.CreateFileA(\"" + path + "\", 3221225472, 0, llvm.null(), 2, 128, 'none) else \"no file\"\n" +
		"    (w, r) = fileapi.WriteFile(h, array { 104, 105, 33, 10, 33 })\n"
	once := func(init string) string {
		return head + "export fn main() {\n" + open + "    var hi := " + init + "\n" +
			"    lo = fileapi.SetFilePointer(h, 0, hi, 2)\n" + // FILE_END
			"    ignore(print.print_int(lo))\n    ignore(print.print_int(hi))\n    ignore(handleapi.CloseHandle(h))\n}\n"
	}
	loop := head + "export fn main() {\n" + open + "    var hi := 0\n" +
		"    loop i = 1 {\n        i > 3 -> ()\n" +
		"        else -> { lo = fileapi.SetFilePointer(h, i, hi, 0); ignore(print.print_int(lo)); ignore(print.print_int(hi)); again(i + 1) }\n" +
		"    }\n    ignore(handleapi.CloseHandle(h))\n}\n"
	const literal = "use io\n\nfn run(n ∈ 0..16) {\n    m = map { 1 : (2, 3), 4 : (5, 6) }\n" +
		"    case m[n] { 'some(p) -> { (a, b) = p; a * b }; 'none -> 0 }\n}\n\n" +
		"export fn main() { loop i = 0 { i >= 6 -> (); else -> { io.print_int(run(i)); again(i + 1) } } }\n"
	for _, target := range []string{"windows", "windows32"} {
		for src, want := range map[string]string{once("0"): "5 0", loop: "1 0 2 0 3 0", literal: "0 6 0 0 30 0"} {
			if got, err := runOn(t, src, target); err != nil || got != want {
				t.Errorf("%s: %q, %v; want %q", target, got, err, want)
			}
		}
		if _, err := runOn(t, once("0 - 2147483649"), target); err == nil ||
			!strings.Contains(err.Error(), "the claim hi ∈") || !strings.Contains(err.Error(), "is false") {
			t.Errorf("%s: a cell of −2^31 − 1 at a LONG's cell: %v, want the claim refused", target, err)
		}
	}
	core.PlantCellSetUnclaimed = true
	_, err := runOnMayFail(t, once("0 - 2147483649"), "windows")
	core.PlantCellSetUnclaimed = false
	if err != nil && strings.Contains(err.Error(), "claim") {
		t.Errorf("with the claim dropped the cell of −2^31 − 1 should build, so the case reaches the rule; got %v", err)
	}
	// the unit's witness is the literal read in `main`'s loop, before
	// ExitProcess: the seek loop ends in a CloseHandle, so its claim does not
	// need the unit's elimination
	ir.PlantUnitUneliminated = true
	_, errLit := runOnMayFail(t, literal, "windows")
	ir.PlantUnitUneliminated = false
	if errLit == nil {
		t.Errorf("with the unit uneliminated the literal read in main's loop should be refused")
	}
}
