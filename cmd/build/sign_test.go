package main

import (
	"strings"
	"testing"

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
