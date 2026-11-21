package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// A PAIR LEAVES FOR THE INTEGERS WHERE IT CANNOT COMPUTE (ADR 0151). On
// JavaScript the unsigned word U is two 32-bit digits (ADR 0083), and a value
// of U is held there exactly when the pair covers its uses; one it cannot
// compute with, a division by 3, a digit loop's division by 10, is ℤ's, the
// rung's BigInt, with what it touches. Two operations the pair now has:
//
//  1. comparison, the lexicographic order of (h, l): 65537² < 92682² with the
//     low digits the other way round (131073 > 18532), so PlantPairCompareLowOnly
//     answers wrongly; 65537² < 65536² with the high digits equal;
//  2. the signed reading of a result in the word from its residue's digits,
//     h − 2³²·[h ≥ 2³¹]: (a·a) | −1024 is −1015 and −1023, and
//     PlantPairUnsignedReading answers 2⁶⁴ − 1015.
//
// Each program runs on all four targets with one answer; each rule's fault
// fails first.
func TestAPairLeavesForTheIntegers(t *testing.T) {
	const src = "use io\n\n" +
		"fn below(a ∈ 0..2^32-1, b ∈ 0..2^32-1) (0..1) { if a * a < b * b { 1 } else { 0 } }\n\n" +
		"fn low(a ∈ 0..2^32-1) (Int) { (a * a) | -1024 }\n\n" +
		"fn third(a ∈ 0..2^32-1) (Int) { a * a / 3 }\n\n" +
		"fn digits(x ∈ 0..2^64-1) (Int) { loop v = x, s = 0 { v == 0 -> s; else -> again(v / 10, s + v % 10) } }\n\n" +
		"export fn main() {\n" +
		"    t = array { 65536, 65537, 4294967295, 3, 92682 }\n" +
		"    loop i = 0 {\n" +
		"        i < 1 -> {\n" +
		"            io.print_int(below(t[i + 1], t[i]))\n" +
		"            io.print_int(below(t[i], t[i + 1]))\n" +
		"            io.print_int(below(t[i + 1], t[i + 4]))\n" +
		"            io.print_int(low(t[i + 3]))\n" +
		"            io.print_int(low(t[i + 2]))\n" +
		"            io.print_int(third(t[i + 2]))\n" +
		"            io.print_int(digits(t[i + 2] * t[i + 2]))\n" +
		"            again(i + 1)\n" +
		"        }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const want = "0 1 1 -1015 -1023 6148914688373205675 81"
	for _, tg := range []string{"js", "go", "java", "windows"} {
		if got, err := runOn(t, src, tg); err != nil || got != want {
			t.Errorf("%s: want %s, got %q, %v", tg, want, got, err)
		}
	}
	// the operations' faults answer wrongly
	for name, plant := range map[string]*bool{
		"PlantPairCompareLowOnly":  &ir.PlantPairCompareLowOnly,
		"PlantPairUnsignedReading": &ir.PlantPairUnsignedReading,
	} {
		*plant = true
		got, err := runJSMayFail(t, src)
		*plant = false
		if err == nil && got == want {
			t.Errorf("with %s the program still answers %s on JavaScript: the witness cannot fail", name, got)
		}
	}
	// the selection's faults refuse: every value of U kept in its pair, a
	// division by 3 is not built; the rebuilt word unascribed, its hi·2³² + lo
	// is unproven; the rung's facts unascribed, the digit sum's accumulator,
	// bounded by the trip count of a loop whose v is now a BigInt, is unbounded
	for name, c := range map[string]struct {
		plant *bool
		why   string
	}{
		"PlantPairsKeepAll":       {&ir.PlantPairsKeepAll, "not a power of two"},
		"PlantPairWordUnascribed": {&ir.PlantPairWordUnascribed, "cannot be proven"},
		"PlantRungFactsForgotten": {&ir.PlantRungFactsForgotten, "cannot be proven"},
	} {
		*c.plant = true
		_, err := runJSMayFail(t, src)
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("with %s: want a refusal saying %q, got %v", name, c.why, err)
		}
	}

	// A WORD DIVIDED BY A PAIR ABOVE IT is the identity: |a| < b, so a rem b = a
	// and a / b = 0, and no digit of the divisor is read. The Mersenne hash's
	// first step, (0·K + 0) % (2⁶¹ − 1), meets it. The step after reads
	// h = (2⁴¹ − 1)·2²⁰ + 5, past JavaScript's word, made by arithmetic: a
	// table's element is a word by storage.
	const mersenne = "use io\n\n" +
		"fn step(h ∈ 0..2^61-2, i ∈ 0..1000) (0..2^61-2) { (h * 1234567890123456789 + i % 251) % (2^61 - 1) }\n\n" +
		"export fn main() {\n" +
		"    t = array { 0, 7, 2199023255551 }\n" +
		"    loop i = 0 {\n" +
		"        i < 1 -> {\n" +
		"            io.print_int(t[i + 1] % (2^61 - 1))\n" +
		"            io.print_int(t[i + 1] / (2^61 - 1))\n" +
		"            io.print_int(step(t[i + 2] * 2^20 + 5, t[i + 1]))\n" +
		"            again(i + 1)\n" +
		"        }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const wantM = "7 0 1694627943690564991"
	for _, tg := range []string{"js", "go"} {
		if got, err := runOn(t, mersenne, tg); err != nil || got != wantM {
			t.Errorf("the Mersenne step on %s: want %s, got %q, %v", tg, wantM, got, err)
		}
	}
	ir.PlantPairSmallByLarge = true
	_, err := runJSMayFail(t, mersenne)
	ir.PlantPairSmallByLarge = false
	if err == nil || !strings.Contains(err.Error(), "not a power of two") {
		t.Errorf("with PlantPairSmallByLarge: want 7 %% (2^61 - 1) refused on JavaScript, got %v", err)
	}

	// A VALUE OF U HELD EXACTLY LEAVES AS U DOES: at a host's any, the decimal
	// string `pair-box` would give (console.log prints it so), and the
	// division's value reaches it unrefused
	const logged = "use js/console\n\n" +
		"fn third(a ∈ 0..2^32-1) (Int) { a * a / 3 }\n\n" +
		"export fn main() {\n" +
		"    t = array { 4294967295 }\n" +
		"    loop i = 0 {\n" +
		"        i < 1 -> { console.log(third(t[i])); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	if got, err := runOn(t, logged, "js"); err != nil || got != "6148914688373205675" {
		t.Errorf("a division of U logged on JavaScript: want 6148914688373205675, got %q, %v", got, err)
	}
	ir.PlantPairSinks = true
	got, err := runJSMayFail(t, logged)
	ir.PlantPairSinks = false
	if err == nil && got == "6148914688373205675" {
		t.Errorf("with PlantPairSinks the logged division still answers: the witness cannot fail")
	}

	// x·3/4 past a declared S₆₄, which stopped here, answers since ADR 0152:
	// TestTheOneSetIsCheckedWhereItClaims (oneset_test.go)
}

// runJSMayFail builds src for JavaScript and runs it, a build's refusal or a
// run's failure returned, not fatal: a planted fault may cause either.
func runJSMayFail(t *testing.T, src string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(dir, "p.mjs")
	if err := run("../../targets", path, "js", art, "../../lib", false, false, ""); err != nil {
		return "", err
	}
	b, err := program("node", art).Output()
	return strings.Join(strings.Fields(string(b)), " "), err
}
