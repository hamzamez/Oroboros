package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
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
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("running it: %v\n%s", err, out)
	}
	if got := strings.Fields(string(out)); len(got) != 3 || got[0] != "5000000000000000000000000000000" || got[1] != got[0] || got[2] != got[0] {
		t.Errorf("printed %q", out)
	}

	// THE PLANTED FAULT, the bug that was a trap: every loop widened up to the
	// function's thresholds.
	ir.PlantLoopThresholdFault = true
	defer func() { ir.PlantLoopThresholdFault = false }()
	if err := run("../../targets", path, "windows", exe, "../../lib", false, false, ""); err == nil || !strings.Contains(err.Error(), "- top") {
		t.Errorf("with the function's thresholds, want `w - top` refused; got %v", err)
	}
}
