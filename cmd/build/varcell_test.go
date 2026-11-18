package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir"
)

// A SCALAR CELL AT A HOST'S CELL OF A RANGE (ADR 0048, 0126): WriteFile's
// last parameter is a cell of DWORD, 0..2^32-1, which the host reads and
// writes. A program's `var w := 0`, a cell of ℤ, fits it iff its value is in
// the DWORD at the call, a claim decided as any is, and the DWORD is within ℤ,
// always; the value read back is known in 0..2^32-1. Before, the cell was
// refused, cell(Int) where cell(0..2^32-1) was required. Run on both triples:
// once, then in a loop, where the cell is a loop variable whose claim the
// IR's intervals prove from its read-back set; a value outside the DWORD is
// refused naming the claim.
//
// THE UNIT, (() (fn () b)) = b, was refused at lowering, so a claim in
// windows' `main` loop, followed by ExitProcess, was left to the refinement
// layer alone, and a map literal read at a run-time key in `main` did not
// build (mapvalue-2026-11-18): 1 is terminal, and what holds it binds nothing.
//
// The faults: the claim dropped, so −1 reaches the host; and the unit left
// uneliminated, so the loop's claim is unproven and the literal refused.
func TestAScalarCellIsTheHostsCell(t *testing.T) {
	const head = "use win/print\nuse windows/fileapi\nuse windows/processenv\n\n"
	once := func(init string) string {
		return head + "export fn main() {\n    b = array { 104, 105, 10 }\n    var w := " + init + "\n" +
			"    r = fileapi.WriteFile(processenv.GetStdHandle(4294967285), b, 0, 3, w)\n" +
			"    ignore(print.print_int(r))\n    ignore(print.print_int(w))\n}\n"
	}
	const loop = head + "export fn main() {\n    b = array { 104, 105, 33, 10 }\n    var w := 0\n" +
		"    loop i = 1 {\n        i > 4 -> ()\n" +
		"        else -> { r = fileapi.WriteFile(processenv.GetStdHandle(4294967285), b, 4 - i, i, w); " +
		"ignore(print.print_int(w)); again(i + 1) }\n    }\n}\n"
	const literal = "use io\n\nfn run(n ∈ 0..16) {\n    m = map { 1 : (2, 3), 4 : (5, 6) }\n" +
		"    case m[n] { 'some(p) -> { (a, b) = p; a * b }; 'none -> 0 }\n}\n\n" +
		"export fn main() { loop i = 0 { i >= 6 -> (); else -> { io.print_int(run(i)); again(i + 1) } } }\n"
	for _, target := range []string{"windows", "windows32"} {
		for src, want := range map[string]string{once("0"): "hi 1 3", loop: "1 ! 2 i! 3 hi! 4", literal: "0 6 0 0 30 0"} {
			if got, err := runOn(t, src, target); err != nil || got != want {
				t.Errorf("%s: %q, %v; want %q", target, got, err, want)
			}
		}
		if _, err := runOn(t, once("0 - 1"), target); err == nil ||
			!strings.Contains(err.Error(), "the claim w ∈ 0..2^32-1 is false") {
			t.Errorf("%s: a cell of −1 at a DWORD's cell: %v, want the claim refused", target, err)
		}
	}
	core.PlantCellSetUnclaimed = true
	_, err := runOn(t, once("0 - 1"), "windows")
	core.PlantCellSetUnclaimed = false
	if err != nil {
		t.Errorf("with the claim dropped the cell of −1 should build, so the case reaches the rule; got %v", err)
	}
	ir.PlantUnitUneliminated = true
	_, errLoop := runOn(t, loop, "windows")
	_, errLit := runOn(t, literal, "windows")
	ir.PlantUnitUneliminated = false
	if errLoop == nil || errLit == nil {
		t.Errorf("with the unit uneliminated the loop's claim (%v) and the literal (%v) should be refused", errLoop, errLit)
	}
}
