package main

import (
	"oroboros/ir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PRINT_INT IS TOTAL ON ℤ (integers-boundary §3, ADR 0147): lib/win/print's
// field holds a value of either word, 20 digits at most, and 25! printed as
// 10043330985984000000 with exit 0 until print-2026-11-19, then stopped with
// its reason. Now ℤ = W ⊔ (ℤ ∖ W): a value outside the words is `decimal`'s, the
// limbs' render, and 25! prints whole; the words' ends still print by the
// field: 2⁶⁴ − 1 (20 digits, the unsigned word), −2⁶³ and 0. Both bugs that
// shipped are planted faults: against either old library text this test
// fails, since 25! printed its low 20 digits or stopped.
func TestPrintIntIsTotalOnTheIntegers(t *testing.T) {
	// THE WORDS' AND E'S OWN RULE, as -words decides it: past them a value
	// is ℤ's under ADR 0150, and this witness is about the readings
	ir.WordsOnly = true
	defer func() { ir.WordsOnly = false }()
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
		if want := "15511210043330985984000000"; err != nil || strings.TrimSpace(string(out)) != want {
			t.Errorf("%s: print_int(25!) should print %s, got err %v and %q", target, want, err, out)
		}
		// THE WORDS' ENDS: 2⁶⁴ − 1 as a literal and as a hash (the field's
		// unsigned path, the decimal arm dead for it), −2⁶³ and 0
		ends := "use win/print\n" +
			"fn h(n ∈ 0..255) (0..2^64-1) {\n" +
			"    loop i = 0, x = 0xCBF29CE484222325 {\n" +
			"        i < n -> again(i + 1, ((x ⊕ i) * 1099511628211) mod 2^64)\n" +
			"        else -> x\n    }\n}\n" +
			"export fn main() {\n" +
			"    print.print_int(0xFFFFFFFFFFFFFFFF)\n" +
			"    print.print_int(h(7))\n" +
			"    print.print_int(0 - 9223372036854775807 - 1)\n" +
			"    print.print_int(0)\n" +
			"}\n"
		got, err := runOnMayFail(t, ends, target)
		if want := "18446744073709551615 9519443742921688584 -9223372036854775808 0"; err != nil || got != want {
			t.Errorf("%s: the words' ends: got %q, %v; want %q", target, got, err, want)
		}
		// THE PLANTED FAULT: a value in U demanded at windows' `decimal`, and a
		// constant in U exact, as before; the hash is then refused at its loop
		ir.PlantDecimalDemandsU = true
		if _, err := runOnMayFail(t, ends, target); err == nil {
			t.Errorf("%s: with U demanded at decimal, the hash should be refused", target)
		}
		ir.PlantDecimalDemandsU = false
		// A U LITERAL THROUGH A CLAIM prints by the field as well (it was refused
		// while a constant in U was taken for the word's literal everywhere)
		claimed := "use win/print\n" +
			"fn u(x ∈ 0..2^64-1) (0..2^64-1) { x }\n" +
			"export fn main() { print.print_int(u(18446744073709551615)) }\n"
		if got, err := runOnMayFail(t, claimed, target); err != nil || got != "18446744073709551615" {
			t.Errorf("%s: a claimed U literal printed: %q, %v", target, got, err)
		}
	}
}
