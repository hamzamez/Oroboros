package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A GLUED BASE'S PROVIDES SERVE ITS MEMBERS (ADR 0122 item 1). The windows
// family's libraries say `provides win32 io`, a fragment of the base, and a
// fragment of a glued theory is a fragment of the gluing (target-system.md
// §2.1's ⊔). So the windows target, which glues win32, has `io`, and a program
// printing through it builds and runs. THE PLANTED FAULT reads `provides` for
// the target's own name alone, as before: `io` is then no module of windows,
// and the program is refused.
func TestAGluedBasesProvidesServeItsMembers(t *testing.T) {
	const src = "use io\n\nexport fn main() {\n    io.print_int(42)\n}\n"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.TrimSpace(out) != "42" {
		t.Errorf("io through the base's provides: %q, %v; want 42", out, err)
	}
	emit.PlantProvidesOwnNameOnly = true
	_, err = buildAndRunNoExec(t, src, "windows")
	emit.PlantProvidesOwnNameOnly = false
	if err == nil {
		t.Error("with provides read for the target's own name only, io should be missing, so the test reaches the rule")
	}
}

// A WHERE READS AN IMPORTED NAME (ADR 0122's readall). A header's formula is
// resolved as its body is: `alias.f` through `use M as alias` is M's f. Before,
// a `where` reading llvm/os's max_len kept `los.max-len`, a name nothing
// defines, so the obligation could be neither proven nor refused. A call past
// the address space is now refused as false, and one inside it built. THE
// PLANTED FAULT leaves the name as written, and the false call is no longer
// decided false.
func TestAWhereReadsAnImportedName(t *testing.T) {
	src := func(k string) string {
		return "use win/print\nuse llvm/os as los\n\nfn take(n ∈ 0..2^47-9) (Int) where n <= los.max_len() { n + 1 }\n\n" +
			"export fn main() {\n    print.print_int(take(" + k + "))\n}\n"
	}
	if out, err := buildAndRun(t, src("5"), "windows", ""); err != nil || strings.TrimSpace(out) != "6" {
		t.Errorf("take(5): %q, %v; want 6", out, err)
	}
	const past = "140737488355320" // 2⁴⁷ − 8, one past x64's max_len
	if _, err := buildAndRunNoExec(t, src(past), "windows"); err == nil || !strings.Contains(err.Error(), "is false at a call") {
		t.Errorf("take(2^47 - 8) should be refused as false, got %v", err)
	}
	core.PlantSigAliasUnresolved = true
	_, err := buildAndRunNoExec(t, src(past), "windows")
	core.PlantSigAliasUnresolved = false
	if err != nil && strings.Contains(err.Error(), "is false at a call") {
		t.Error("with the alias unresolved the where should not decide false, so the test reaches the rule")
	}
}
