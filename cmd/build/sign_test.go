package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// A SIGNED SUM IS THE GROUP COMPLETION'S (ADR 0098): on windows, which has no
// bignum, a + b with a = 3^100 and b = -(5^60) is a's sign and |a| - |b|, the
// numeral the hosts' bignums print. THE PLANTED FAULT takes the magnitude's sum,
// ignoring the signs, and must print another.
func TestASignedSumIsTheGroupCompletions(t *testing.T) {
	const src = "use win/print\n" +
		"export fn calc(n ∈ 0..1) (0 - pow(2, 200)..2^200) {\n" +
		"    a = 3^100\n" +
		"    b = 0 - 5^60\n" +
		"    if n == 0 { a + b } else { b + b }\n}\n" +
		"export fn main() {\n" +
		"    print.print_str(big_str(calc(0)))\n" +
		"    print.print_str(big_str(calc(1)))\n}\n"
	want := "515376653370273342632913923803380576748738381376 -1734723475976807094411924481391906738281250"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.Join(strings.Fields(out), " ") != want {
		t.Errorf("signed sums on windows: %q, %v; want %s", out, err, want)
	}
	ir.PlantSignFault = true
	defer func() { ir.PlantSignFault = false }()
	out, err = buildAndRun(t, src, "windows", "")
	if err == nil && strings.Join(strings.Fields(out), " ") == want {
		t.Errorf("with the magnitude's sum for a signed one, the numerals are still right: the witness cannot fail")
	}
}

// AN UNBOUNDED DIFFERENCE IS K(ℕ*)'s (ADR 0099): on windows, with ℤ as the set,
// 3^200 − 7^90 is a numeral of any length, and prints Python's. THE PLANTED
// FAULT takes the library's sum for the difference, and must print another.
func TestAnUnboundedDifferenceIsTheNumeralsOfAnyLength(t *testing.T) {
	const src = "use win/print\n" +
		"fn calc(n ∈ 0..1) (Int) { if n == 0 { 3^200 - 7^90 } else { 0 - 7^90 - 3^200 } }\n" +
		"export fn main() {\n" +
		"    print.print_str(big_str(calc(0)))\n" +
		"    print.print_str(big_str(calc(1)))\n}\n"
	want := "265613988875874769327330844441458582469893325939849349897520956915807512043087616622264459016752 " +
		"-265613988875874769350231799630100671188573579366939642051628966562377469759516749366504939071250"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.Join(strings.Fields(out), " ") != want {
		t.Errorf("unbounded differences on windows: %q, %v; want %s", out, err, want)
	}
	ir.PlantNatFault = true
	defer func() { ir.PlantNatFault = false }()
	out, err = buildAndRun(t, src, "windows", "")
	if err == nil && strings.Join(strings.Fields(out), " ") == want {
		t.Errorf("with the sum for the difference, the numerals are still right: the witness cannot fail")
	}
}

// A QUOTIENT BY A NUMERAL IS ALGORITHM D's (ADR 0099, R3c): on windows, x / y
// and x % y take the add-back, D6, and print Python's numerals. At B = 2⁶³ an
// add-back is about 2/B likely, so the pair is CONSTRUCTED: y = (v_t, v₂, B − 1)
// normalized, and x = q·(v_t·B² + v₂·B), whose top digits give q̂ = q and pass
// D3's two-digit test while x − q·y = −q·(B − 1) < 0 (an exact model of the
// library's loop counts the add-back). THE PLANTED FAULT drops the add-back,
// so q̂ one too large stays, and must print others.
func TestAQuotientByANumeralIsAlgorithmDs(t *testing.T) {
	const src = "use win/print\n" +
		"fn calc(n ∈ 0..1) (Int) {\n" +
		"    x = 1176956575385002643219210516851437453019191645837006471168\n" +
		"    y = 392318858461667547739736838950479151015620587315856932863\n" +
		"    if n == 0 { x / y } else { x % y }\n}\n" +
		"export fn main() {\n" +
		"    print.print_str(big_str(calc(0)))\n" +
		"    print.print_str(big_str(calc(1)))\n}\n"
	want := "2 392318858461667547739736838950479150987950471205292605442"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.Join(strings.Fields(out), " ") != want {
		t.Errorf("Algorithm D on windows: %q, %v; want %s", out, err, want)
	}
	emit.PlantNoAddBack = true
	defer func() { emit.PlantNoAddBack = false }()
	out, err = buildAndRun(t, src, "windows", "")
	if err == nil && strings.Join(strings.Fields(out), " ") == want {
		t.Errorf("without the add-back the numerals are still right: the witness cannot fail")
	}
}

// A BOUNDED QUOTIENT MOVES TO THE NUMERALS OF ANY LENGTH (ADR 0099, R3c): on
// windows, which has no bignum, a program in 0..2^300 that divides by a big
// value is beyond the fixed limbs, and K(ℕ*) takes it whole, its one set
// enforced by sign and bit length: the quotients print Python's numerals, and
// a value past 2^300 crashes with the bound's reason. THE PLANTED FAULT keeps
// the program on the fixed limbs, where it must be refused.
func TestABoundedQuotientMovesToTheNumeralsOfAnyLength(t *testing.T) {
	const src = "use win/print\n" +
		"fn calc(n ∈ 0..2) (0..2^300) {\n" +
		"    a = 3^180\n" +
		"    b = 7^40 + n\n" +
		"    if n == 2 { a * a / b } else { a / b + a % b }\n}\n" +
		"export fn main() {\n" +
		"    print.print_str(big_str(calc(0)))\n" +
		"    print.print_str(big_str(calc(1)))\n" +
		"    print.print_str(big_str(calc(2)))\n}\n"
	want := "11964767091463786734584556628644403953403431640331601 " +
		"11964767091463786734582602581825694949127182091174155"
	out, built, err := buildAndCrash(t, src)
	if !built || err == nil || !strings.HasPrefix(strings.Join(strings.Fields(out), " "), want) ||
		!strings.Contains(out, "the declared range is too small") {
		t.Errorf("a bounded quotient on windows: %q, built %v, %v; want %s, then the bound's crash", out, built, err, want)
	}
	ir.PlantNoNatFallback = true
	defer func() { ir.PlantNoNatFallback = false }()
	if out, built, _ := buildAndCrash(t, src); built && strings.HasPrefix(strings.Join(strings.Fields(out), " "), want) {
		t.Errorf("kept on the fixed limbs, the quotient still runs: the witness cannot fail")
	}
}

// buildAndCrash builds src for windows and runs it, a non-zero exit allowed:
// its output, whether it built, and the run's error.
func buildAndCrash(t *testing.T, src string) (string, bool, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	if err := run("../../targets", path, "windows", exe, "../../lib", false, false, ""); err != nil {
		return err.Error(), false, err
	}
	out, err := exec.Command(exe).CombinedOutput()
	return string(out), true, err
}
