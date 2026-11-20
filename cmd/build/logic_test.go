package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/presburger"
)

// A CLAIM WITH QUANTIFIERS IS DECIDED (ADR 0145, stage 2; logic-research §3,
// §4.1): `∀ x ∈ S : φ`, `∃`, `⇒` and `⇔` in `ensure` and `assert`, each sequent
// Γ ⊢ φ decided as Γ ∧ ¬φ unsatisfiable, the quantifiers by polarity (a fresh
// constant, or Cooper's elimination), the rest by the Omega test, a division by
// a constant read exactly. The programs, on every target:
//   - the Gregorian leap year, its rule stated with divisibility under
//     quantifiers and proven of the remainders' computation: 0 1 1 0;
//   - Bloch's midpoint, lo ≤ m ≤ hi and within one of halfway, proven of
//     lo + (hi − lo) / 2: 5 3 3458764513820540928.
//
// And what must be refused, each its reason checked:
//   - the rule with 200 for 100 (1900 the countermodel);
//   - an `assert` the facts do not give, and one with a quantifier;
//   - a claim that holds only by a fact in scope, lang's 0 ≤ x·x, with the
//     facts dropped (the fault, emit.PlantFormulaHypsDropped).
func TestAClaimWithQuantifiersIsDecided(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{"leap", "0 1 1 0"},
		{"midpoint", "5 3 3458764513820540928"},
	} {
		src, err := os.ReadFile("../../examples/logic/" + c.file + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		for _, tg := range []string{"go", "js", "java", "windows", "windows32"} {
			run := runOnMayFail
			if tg == "js" || tg == "java" {
				run = runOn
			}
			got, err := run(t, string(src), tg)
			// the midpoint's 2⁶² is past JavaScript's word, ±(2⁵³ − 1): refused there,
			// portability computed (ADR 0026)
			if c.file == "midpoint" && tg == "js" {
				if err == nil || !strings.Contains(err.Error(), "inside the word of target js") {
					t.Errorf("midpoint on js: want refused, 2⁶² past the word, got %q, %v", got, err)
				}
				continue
			}
			if err != nil || strings.Join(strings.Fields(got), " ") != c.want {
				t.Errorf("%s on %s: want %s, got %q, %v", c.file, tg, c.want, got, err)
			}
		}
		if c.file == "leap" {
			wrong := strings.Replace(string(src), "y == 100 * k", "y == 200 * k", 1)
			if _, err := buildOnly(t, wrong, "go"); err == nil || !strings.Contains(err.Error(), "does not establish its `ensure`") {
				t.Errorf("the rule with 200 for 100 should be refused, 1900 its countermodel: %v", err)
			}
		}
	}
	refused := []struct{ name, src, why string }{
		{"an assert the facts do not give", "use io\nfn g(a ∈ 0..100, b ∈ 0..100) (Int) {\n    assert a <= b\n    b - a\n}\n" +
			"export fn main() { loop k = 0 { k <= 100 -> { io.print_int(g(k, 50)); again(k + 1) }; else -> () } }\n",
			"an `assert` does not follow"},
		{"a false quantified assert", "use io\nfn f(lo ∈ 0..100, hi ∈ 0..100) (Int) where lo <= hi {\n    m = lo + (hi - lo) / 2\n    assert ∀ j ∈ 0..m : j < lo\n    m\n}\nexport fn main() { io.print_int(f(2, 9)) }\n",
			"an `assert` does not follow"},
		{"a quantified claim, false", "use io\nfn h(x ∈ 0..10) (r ∈ Int) ensure ∀ j ∈ 0..r : j <= 9 {\n    x\n}\n" +
			"export fn main() { io.print_int(h(3)) }\n",
			"does not establish its `ensure`"},
	}
	for _, c := range refused {
		if _, err := buildOnly(t, c.src, "go"); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want refused (%s), got %v", c.name, c.why, err)
		}
	}
	accepted := []struct{ name, src string }{
		{"a true quantified assert", "use io\nfn f(lo ∈ 0..100, hi ∈ 0..100) (Int) where lo <= hi {\n    m = lo + (hi - lo) / 2\n    assert lo <= m && m <= hi\n    assert ∀ j ∈ 0..m : j <= hi\n    m\n}\nexport fn main() { io.print_int(f(2, 9)) }\n"},
		{"a where on a remainder", "use io\nfn quarter(n ∈ 0..1000) (r ∈ Int) where n % 4 == 0 ensure 4 * r == n {\n    n / 4\n}\nexport fn main() { io.print_int(quarter(12)) }\n"},
		{"a quantified claim, true", "use io\nfn h(x ∈ 0..10) (r ∈ Int) ensure ∀ j ∈ 0..r : j <= 10 {\n    x\n}\n" +
			"export fn main() { io.print_int(h(3)) }\n"},
	}
	for _, c := range accepted {
		if _, err := buildOnly(t, c.src, "go"); err != nil {
			t.Errorf("%s: want accepted, got %v", c.name, err)
		}
	}
	// the fault: the facts in scope dropped. A parameter's set reaches the
	// formula through the `where` it is desugared into, so the witness is a fact
	// of lang's theory, 0 ≤ x·x (emit/lang-facts.oro), about a product the
	// formula reads as an atom: with it the square is a natural, without it not
	const square = "use io\nfn sq(x ∈ -100..100) (r ∈ Int) ensure ∃ j ∈ 0.. : r == j {\n    x * x\n}\n" +
		"export fn main() { io.print_int(sq(7)) }\n"
	if _, err := buildOnly(t, square, "go"); err != nil {
		t.Errorf("x·x ∈ 0.. from lang's square fact: want accepted, got %v", err)
	}
	var err error
	emit.PlantFormulaHypsDropped = true
	_, err = buildOnly(t, square, "go")
	emit.PlantFormulaHypsDropped = false
	if err == nil {
		t.Error("with the facts dropped, x·x ∈ 0.. should not be proven, so the witness can fail")
	}
	// a let-bound pure name is its definition, x ≐ e: the book's assertion over
	// m = lo + (hi − lo) / 2 in a loop, proven, and refused with the
	// definitions ignored (emit.PlantLetDefsIgnored)
	book, err := os.ReadFile("../../docs/book/code/claims/assert.oro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildOnly(t, string(book), "go"); err != nil {
		t.Errorf("the book's assertion over a let-bound midpoint: want accepted, got %v", err)
	}
	emit.PlantLetDefsIgnored = true
	_, err = buildOnly(t, string(book), "go")
	emit.PlantLetDefsIgnored = false
	if err == nil {
		t.Error("with let definitions ignored, the assertion over m should be refused, so the witness can fail")
	}
	// and the procedure's own faults reach the programs: Cooper without its
	// bounds' instances cannot prove the leap year's rule
	src, _ := os.ReadFile("../../examples/logic/leap.oro")
	presburger.PlantCooperNoBounds = true
	_, err = buildOnly(t, string(src), "go")
	presburger.PlantCooperNoBounds = false
	if err == nil {
		t.Error("with Cooper's lower-bound instances dropped the leap year's claim should be refused, so the witness can fail")
	}
}
