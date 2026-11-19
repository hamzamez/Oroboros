package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PRINT_INT STOPS PAST 20 DIGITS, WHERE IT PRINTED THE LOW 20 (print-2026-11-19):
// lib/win/print's field holds a value of either word, 20 digits at most, and
// 25! printed as 10043330985984000000 with exit 0. Now a value of more than 20
// digits stops the program with the reason, and the words' ends still print:
// 2⁶⁴ − 1 (20 digits, the unsigned word), −2⁶³ and 0. The bug that shipped is
// the planted fault: against the old library text this test fails, since 25!
// printed and exited 0.
func TestPrintIntStopsPastTwentyDigits(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three windows programs")
	}
	for _, target := range []string{"windows", "windows32"} {
		big := "use win/print\n" +
			"fn fact(n ∈ 0..25) (Nat) { loop acc = 1, i = 2 { i > n -> acc; else -> again(acc * i, i + 1) } }\n" +
			"export fn main() { print.print_int(fact(25)) }\n"
		dir := t.TempDir()
		path := filepath.Join(dir, "p.oro")
		if err := os.WriteFile(path, []byte(big), 0o644); err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(dir, "p.exe")
		if err := run("../../targets", path, target, exe, "../../lib", false, false, ""); err != nil {
			t.Fatalf("%s: 25! should build: %v", target, err)
		}
		out, err := program(exe).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "print_int: more than 20 digits") ||
			strings.Contains(string(out), "10043330985984000000") {
			t.Errorf("%s: print_int(25!) should stop with its reason, got err %v and %q", target, err, out)
		}
		ends := "use win/print\n" +
			"fn u(x ∈ 0..2^64-1) (0..2^64-1) { x }\n" +
			"export fn main() {\n" +
			"    print.print_int(u(18446744073709551615))\n" +
			"    print.print_int(0 - 9223372036854775807 - 1)\n" +
			"    print.print_int(0)\n" +
			"}\n"
		got, err := runOnMayFail(t, ends, target)
		if want := "18446744073709551615 -9223372036854775808 0"; err != nil || got != want {
			t.Errorf("%s: the words' ends: got %q, %v; want %q", target, got, err, want)
		}
	}
}
