package llvm

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/emit"
)

func loadTarget32(t *testing.T) *emit.Target {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows32", []string{"../../targets"}, []string{"../../lib"})
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

// scalarSrc is the measured loop (ADR 0127): every value proven in S₃₂, and a
// shift whose count may pass 31 beside one whose count cannot.
const scalarSrc = "fn run(n ∈ 0..300000000) {\n" +
	"    loop i = 0, s = 0 {\n" +
	"        i >= n -> s\n" +
	"        else -> again(i + 1, if (i * 3 + s) % 7 == 0 { (s + i) % 1000003 } else { s })\n" +
	"    }\n" +
	"}\n" +
	"export fn f(n ∈ 0..300000000, k ∈ 0..40, j ∈ 0..20) (Int) { run(n) + ((n >> k) + (n >> j)) }\n"

// A SCALAR IS HELD WHERE ITS RANGE IS (ADR 0127): on windows32, whose
// preferred register width is 32, the loop's values are i32 and its arithmetic
// is printed at 32 bits; a shift is printed there only where its count lies in
// [0, 31]; and the boundary is the word's, a parameter narrowed at entry and
// the result widened at `ret`. The planted faults:
//   - PlantNoScalar holds every integer in the word, and no i32 is printed, so
//     the test reaches the rule;
//   - PlantScalarNoDomain prints the shift by a count up to 40 at 32 bits,
//     where it is poison past 31, and the test sees it.
func TestAScalarIsHeldWhereItsRangeIs(t *testing.T) {
	tg := loadTarget32(t)
	code, err := compileOn(t, tg, scalarSrc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"add nuw nsw i32",
		"mul nuw nsw i32",
		"urem i32",
		"icmp samesign sge i32",
		"trunc i64 %v0 to i32", // n, narrowed at entry
		"ashr i32",             // n >> j, j ∈ [0, 20]
		"ashr i64",             // n >> k, k ∈ [0, 40]: at the word
		"ret i64",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("windows32's text lacks %q:\n%s", want, code)
		}
	}
	if regexp.MustCompile(`alloca i64`).MatchString(code) && !strings.Contains(code, "alloca i32") {
		t.Errorf("the loop's variables are not i32:\n%s", code)
	}

	// windows declares no 32-bit scalar: its text is the word's
	code64, err := compileOn(t, loadTarget(t), scalarSrc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(code64, "i32 ") && strings.Contains(code64, "add nuw nsw i32") {
		t.Errorf("windows prints at 32 bits, and declares no scalar:\n%s", code64)
	}

	PlantNoScalar = true
	code, err = compileOn(t, loadTarget32(t), scalarSrc)
	PlantNoScalar = false
	if err != nil || strings.Contains(code, "add nuw nsw i32") {
		t.Errorf("with every integer in the word: want no i32 arithmetic, got %v\n%s", err, code)
	}
	PlantScalarNoDomain = true
	code, err = compileOn(t, loadTarget32(t), scalarSrc)
	PlantScalarNoDomain = false
	if err != nil || strings.Contains(code, "ashr i64") {
		t.Errorf("with the 32-bit domain not asked: want the count past 31 shifted at 32 bits, got %v\n%s", err, code)
	}
}
