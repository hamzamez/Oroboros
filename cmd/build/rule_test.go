package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A RULE'S ELEMENTS ARE ρ OF THEIR HULL (ADR 0083 item 5, ADR 0106;
// karatsuba-2026-11-14): a rule's body is its table's elements, as a store's
// value is, so a body whose image lies in [0, 2⁶⁴) and outside the signed word
// is held in the unsigned word. Karatsuba's level-0 tables are such rules, a
// numeral's digits under a guard (natnum.oro's kmul). The body here joins a
// digit of U with a small integer, as the guard's arms do. On windows only: a
// table of U elements is LLVM's i64 under both readings, and no host table is
// retyped by its hull for Go yet (karatsuba-2026-11-14, not built).
const ruleSrc = `use %s

fn run(n ∈ 0..3) (0..999) {
    t = alloc(array { if k < 2 { 18446744073709551615 - k } else { k } : k ∈ 0..3 })
    if n < len(t) { t[n] %% 1000 } else { 0 }
}

export fn main() {
    %s
}
`

// THE PLANTED FAULT receives the body by the table value, the signed word, as
// before: the refusal ("held in the UNSIGNED word") comes back.
func TestARulesElementsAreTheUnsignedWord(t *testing.T) {
	for _, tc := range []struct{ target, use, print string }{
		{"windows", "win/print", "ignore(print.print_int(run(0)))\n    ignore(print.print_int(run(1)))\n    ignore(print.print_int(run(3)))"},
	} {
		src := strings.Replace(strings.Replace(ruleSrc, "%s", tc.use, 1), "%s", tc.print, 1)
		src = strings.ReplaceAll(src, "%%", "%")
		out, err := buildAndRun(t, src, tc.target, "")
		if err != nil || strings.Join(strings.Fields(out), " ") != "615 614 3" {
			t.Fatalf("%s: a rule of U elements: %q, %v; want 615 614 3", tc.target, out, err)
		}
		ir.PlantRuleElemSigned = true
		_, err = buildAndRun(t, src, tc.target, "")
		ir.PlantRuleElemSigned = false
		// refused as a value of U at a word, or, a printed result now ℤ's
		// (ADR 0147), as a value above the word at one: the fault is caught
		if err == nil || !strings.Contains(err.Error(), "held in the UNSIGNED word") &&
			!strings.Contains(err.Error(), "WIDER than the word") {
			t.Errorf("%s: with the rule's body received signed, want the unsigned word's refusal; got %v", tc.target, err)
		}
	}
}
