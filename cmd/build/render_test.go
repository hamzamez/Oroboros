package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A LIMB VALUE RENDERS ON WINDOWS, AND EACH LOOP WIDENS UP TO ITS OWN
// THRESHOLDS (llvm-render-2026-11-07). The differential's driver for
// big-negmid: three renderings and three printings inlined into one main. Its
// digit loops' `top` descends one threshold per round, and with the function's
// constants as every loop's thresholds the rounds ran out before 0, so `w − top`
// was refused (a trap under -checked). With each loop's own constants and the
// ends of what it reads from outside it is proven, and the program prints the
// numerals.
func TestALimbValueRendersOnWindows(t *testing.T) {
	const src = "use win/print\n" +
		"export fn calc(n ∈ 0..6) (0..2^200) {\n" +
		"    a = 5000000000000000000000000000000\n" +
		"    b = 1000000000000000000000000000000 * n\n" +
		"    d = a - b\n" +
		"    d + b\n}\n" +
		"fn run(n) { big_str(calc(n)) }\n" +
		"export fn main() {\n" +
		"    print.print_str(run(0))\n" +
		"    print.print_str(run(5))\n" +
		"    print.print_str(run(4))\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "render.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "render.exe")
	if err := run("../../targets", path, "windows", exe, "../../lib", false, false, ""); err != nil {
		t.Fatalf("three-instance rendering on windows: %v", err)
	}
	out, err := program(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("running it: %v\n%s", err, out)
	}
	if got := strings.Fields(string(out)); len(got) != 3 || got[0] != "5000000000000000000000000000000" || got[1] != got[0] || got[2] != got[0] {
		t.Errorf("printed %q", out)
	}

	// THE PLANTED FAULT, every loop widened up to the function's thresholds, no
	// longer reaches this program (strings-2026-11-16): `ascii` is a definition
	// in the language now, so `w - top`'s length is ADR 0028's mark on its
	// parameter, which the refinement layer proves from top ≤ w whatever the
	// loop's thresholds. The thresholds are precision, any set sound, and a
	// fault no program shows is a witness that cannot fail; so it is not
	// asserted here, and a program that needs a loop's own thresholds is owed.
}
