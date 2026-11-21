package main

import (
	"strings"
	"testing"

	"oroboros/ir/golang"
	"oroboros/ir/java"
)

// THE DOUBLE WORD ON EVERY HOST (ADR 0103; the book's finding 118). The
// midpoint over full words and a quotient by 7, each intermediate in E and
// outside the word, the arguments read from a table at a loop's index (so
// nothing folds at compile time), run where E is now realized: Go's two
// uint64 by math/bits, the JVM's two longs, windows' and windows32's i128. The
// sum −2⁶³ + −2⁶³ carries out of its low digits (2⁶³ + 2⁶³ = 2⁶⁴), so a
// printer that drops the carry answers wrongly; and the remainder of a product
// of two words by 7 has a quotient past the word, so on Go and the JVM it is
// Horner's two 2-by-1 divisions and not one. (LLVM's signed i128 division by
// 7, __divti3, is ir/llvm's TestASignedQuotientOfEIsTheMagnitudes.)
func TestTheDoubleWordOnEveryHost(t *testing.T) {
	const src = "use io\n\n" +
		"fn mid(lo ∈ -2^63..2^63-1, hi ∈ -2^63..2^63-1) (-2^63..2^63-1) { (lo + hi) / 2 }\n\n" +
		"fn q7(y ∈ -2^63..2^63-1) (-2^63..2^63-1) { y * 5 / 7 }\n\n" +
		"fn m7(x ∈ -2^63..2^63-1, y ∈ -2^63..2^63-1) (0..6) { x * y mod 7 }\n\n" +
		"export fn main() {\n" +
		"    t = array { -9223372036854775808, 9223372036854775806, 9223372036854775807 }\n" +
		"    loop i = 0 {\n" +
		"        i < 1 -> {\n" +
		"            io.print_int(mid(t[i], t[i]))\n" +
		"            io.print_int(mid(t[i + 2], t[i + 1]))\n" +
		"            io.print_int(q7(t[i]))\n" +
		"            io.print_int(m7(t[i + 1], t[i + 1]))\n" +
		"            io.print_int(m7(t[i], t[i + 1]))\n" +
		"            again(i + 1)\n" +
		"        }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const want = "-9223372036854775808 9223372036854775806 -6588122883467697005 1 1"
	for _, tg := range []string{"go", "java", "windows"} { // windows32 below, its run helper
		got, err := runOn(t, src, tg)
		if err != nil || strings.Join(strings.Fields(got), " ") != want {
			t.Errorf("%s: want %s, got %q, %v", tg, want, got, err)
		}
	}
	// and a remainder whose quotient, ≈ 2¹²³, is past the word: one 2-by-1
	// division (PlantRemOneDivision) breaks its precondition, Horner's two do not
	for _, c := range []struct {
		tg    string
		plant *bool
	}{{"go", &golang.PlantDoubleCarryDropped}, {"java", &java.PlantDoubleCarryDropped},
		{"go", &golang.PlantRemOneDivision}, {"java", &java.PlantRemOneDivision}} {
		tg, plant := c.tg, c.plant
		run := runOn // the JVM's faults answer wrongly; Go's Div64 panics, a run's failure
		if tg == "go" {
			run = runOnMayFail
		}
		*plant = true
		got, err := run(t, src, tg)
		*plant = false
		if err == nil && strings.Join(strings.Fields(got), " ") == want {
			t.Errorf("%s with a planted fault: still %s, so the witness cannot fail", tg, want)
		}
	}
	// windows32 too: LLVM expands an i128 division by a constant inline on i686
	if got, err := runOnMayFail(t, src, "windows32"); err != nil || strings.Join(strings.Fields(got), " ") != want {
		t.Errorf("windows32: want %s, got %q, %v", want, got, err)
	}
}
