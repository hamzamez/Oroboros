package emit_test

import (
	"testing"

	"oroboros/emit"
)

// A RANGE MARK THE INTERVALS LEFT IS DECIDED BY THE FACTS (llvm-2026-11-02).
// A table of tuples keeps the interval route from lowering the program, so it
// hands the refinement layer every call's range obligation as it was. The
// fault that shipped: an in-word mark reaching the refinement layer was read
// as one above the word and refused, its bounds among the known facts.
func TestARangeMarkTheIntervalsLeftIsDecidedByTheFacts(t *testing.T) {
	head := "fn w(n ∈ 0..100) (Int) { n }\n" +
		"fn mk(x) {\n" +
		"    var t := array { 0 : _ ∈ 0..3 }\n" +
		"    loop i = 0 { i >= 4 -> t; else -> { t[i] := (i + x, i); again(i + 1) } }\n" +
		"}\n"
	body := func(hi string) string {
		return head + "export fn g(x ∈ 0..9, k ∈ -1000..1000) (Int) {\n" +
			"    t = mk(x)\n" +
			"    s = t[1][0] - t[1][1]\n" +
			"    if k >= 0 && k <= " + hi + " { w(k) + s } else { s }\n" +
			"}"
	}
	accepts(t, "a guard that proves the range", body("100"))
	refuses(t, "a guard that does not", body("200"), "w's parameter n")

	emit.PlantRangeMarkFault(true)
	defer emit.PlantRangeMarkFault(false)
	if _, err := compileGo(t, body("100")); err == nil {
		t.Errorf("the planted fault accepted the program: the test does not reach the rule")
	}
}
