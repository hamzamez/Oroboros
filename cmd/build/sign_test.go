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
