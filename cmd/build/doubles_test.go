package main

import (
	"fmt"
	"strings"
	"testing"

	"oroboros/ir"
)

// ONE DOUBLE COMPARED THRICE VERIFIES (the book's finding 39). For x ∈ F64 and
// q ∉ F64, no double lies between ⌊q⌋ and ⌈q⌉, so x < q ⟺ x ≤ q ⟺ x < ⌈q⌉ and
// x > q ⟺ x ≥ q ⟺ x ≥ ⌈q⌉: one constant for all four relations, which makes
// the rewrite commute with negation. A branch's π, rewritten apart from its
// guard, is then the guard's literal or its negation, as W6 asks (Theorem E).
// x = round(F64, 0.1) is the double above 0.1, so the answers are gt, not lt,
// not eq. THE PLANTED FAULT, the two ends ⌊q⌋ and ⌈q⌉ for one q, fails W6
// again.
func TestOneDoubleComparedThriceVerifies(t *testing.T) {
	const src = "use io\n\nexport fn main() {\n    x = round(F64, 0.1)\n" +
		"    if x > 0.1 { io.print_line(\"gt\") } else { io.print_line(\"not gt\") }\n" +
		"    if x < 0.1 { io.print_line(\"lt\") } else { io.print_line(\"not lt\") }\n" +
		"    if x == 0.1 { io.print_line(\"eq\") } else { io.print_line(\"not eq\") }\n}\n"
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, src, target, "")
		if err != nil || strings.Join(strings.Fields(out), " ") != "gt not lt not eq" {
			t.Errorf("%s: three comparisons of round(F64, 0.1) with 0.1: %q, %v; want gt, not lt, not eq", target, out, err)
		}
	}
	for name, plant := range map[string]*bool{"two ends": &ir.PlantGaloisTwoEnds} {
		*plant = true
		_, err := buildAndRunNoExec(t, src, "go")
		*plant = false
		if err == nil || !strings.Contains(err.Error(), "W6") {
			t.Errorf("with %s the program should fail W6 again, so the test reaches the rule; got %v", name, err)
		}
	}
}

// A DOUBLE BESIDE A ROUNDED DECIMAL IS REFUSED, SAYING WHY (the book's finding
// 40). numbers.md §6.2 refuses comparing a double with an exact term of N; the
// refusal's message showed the comparison's source term, which lowering had not
// attached, and the compiler crashed in its own message. THE PLANTED FAULT shows
// the absent term again: the build must panic, caught here.
func TestADoubleBesideARoundedDecimalIsRefused(t *testing.T) {
	const src = "use io\n\nexport fn main() {\n" +
		"    if round(F64, 1 ÷ 3) < round(D(18, 0..1), 1 ÷ 3) { io.print_line(\"less\") } else { io.print_line(\"not\") }\n}\n"
	for _, target := range []string{"go", "windows"} {
		_, err := buildAndRunNoExec(t, src, target)
		if err == nil || !strings.Contains(err.Error(), "compares a double with an exact term of N") {
			t.Errorf("%s: want the comparison refused for its reason, got %v", target, err)
		}
	}
	ir.PlantSrcUnguarded = true
	defer func() { ir.PlantSrcUnguarded = false }()
	panicked := func() (p bool) {
		defer func() {
			if r := recover(); r != nil {
				p = fmt.Sprint(r) != ""
			}
		}()
		buildAndRunNoExec(t, src, "go")
		return false
	}()
	if !panicked {
		t.Error("with the absent term shown the build should crash again, so the test reaches the rule")
	}
}
