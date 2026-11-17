package llvm

import (
	"strings"
	"testing"
)

// SAMESIGN IS PROVEN OF ITS OPERANDS (φ's second kind; samesign). A signed
// order is printed `icmp samesign` only where the IR proves both operands on
// one side of 0, where the signed and unsigned orders agree; elsewhere the
// flag would make the comparison poison. Three comparisons:
//   - x < 3 with x ∈ -5..5 crosses 0, so it takes no flag;
//   - y < 3 with y ∈ 0..9 is of naturals, so it takes one;
//   - z > -3 with z ∈ -9..-1 is of negatives, so it takes one.
//
// THE PLANTED FAULT writes it on every signed comparison, the first included.
// LLVM's use of the flag depends on its own rewrites, so the witness reads
// the printed text: it is the rule's premise, the sign facts, that it checks.
func TestSamesignIsProvenOfItsOperands(t *testing.T) {
	src := func(set, cmp string) string {
		return "export fn f(x ∈ " + set + ") (Bool) { " + cmp + " }\n"
	}
	for _, c := range []struct {
		set, cmp string
		flag     bool
	}{
		{"-5..5", "x < 3", false},
		{"0..9", "x < 3", true},
		{"-9..-1", "x > -3", true},
	} {
		got, err := compile(t, src(c.set, c.cmp))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "icmp samesign") != c.flag {
			t.Errorf("x ∈ %s, %s: samesign printed %v, want %v:\n%s", c.set, c.cmp, !c.flag, c.flag, got)
		}
	}
	PlantSamesignAlways = true
	got, err := compile(t, src("-5..5", "x < 3"))
	PlantSamesignAlways = false
	if err != nil || !strings.Contains(got, "icmp samesign") {
		t.Errorf("with the fault planted x < 3 over -5..5 should take the flag, so the test reaches the rule: %v\n%s", err, got)
	}
}
