package main

import (
	"strings"
	"testing"

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
