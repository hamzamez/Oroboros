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

// A TABLE'S PROPERTY IS DECIDED (ADR 0145, stage 3; logic-research §4.2): a
// universal over a table's indices, the array property fragment (Bradley,
// Manna and Sipma 2006), its hypotheses instantiated over the sequent's index
// set (Theorem A), two reads at one index congruent (Ackermann), a loop's
// formula invariant Hoare's rule. The programs, on every target:
//   - a sorted table's ends are its least and its greatest: 1 8;
//   - a table's maximum, the loop's invariant ∀ j < i : a[j] ≤ m: 9;
//   - binary search, its claim both ways (found, or absent from the whole
//     table) from sortedness and the invariant's two quantified sides: 3 0 5 -1.
//
// And what must be refused: an unsorted table at least's call; a maximum whose
// step keeps the smaller; each of the procedure's faults on a program.
func TestATablesPropertyIsDecided(t *testing.T) {
	read := func(f string) string {
		src, err := os.ReadFile("../../examples/logic/" + f + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		return string(src)
	}
	for _, c := range []struct{ file, want string }{
		{"sorted", "1 8"},
		{"maximum", "9"},
		{"bsearch", "3 0 5 -1"},
	} {
		src := read(c.file)
		for _, tg := range []string{"go", "js", "java", "windows", "windows32"} {
			run := runOnMayFail
			if tg == "js" || tg == "java" {
				run = runOn
			}
			got, err := run(t, src, tg)
			if err != nil || strings.Join(strings.Fields(got), " ") != c.want {
				t.Errorf("%s on %s: want %s, got %q, %v", c.file, tg, c.want, got, err)
			}
		}
	}
	refuse := func(name, src, why string) {
		t.Helper()
		if _, err := buildOnly(t, src, "go"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want refused (%s), got %v", name, why, err)
		}
	}
	accept := func(name, src string) {
		t.Helper()
		if _, err := buildOnly(t, src, "go"); err != nil {
			t.Errorf("%s: want accepted, got %v", name, err)
		}
	}
	sorted, maximum, bsearch := read("sorted"), read("maximum"), read("bsearch")
	unsorted := strings.Replace(sorted, "array { 1, 3, 5, 8 }", "array { 1, 5, 3, 8 }", 1)
	if unsorted == sorted {
		t.Fatal("the unsorted table's edit did not apply")
	}
	refuse("an unsorted table at least's call", unsorted, "where")
	broken := strings.Replace(maximum, "if a[i] > m { a[i] } else { m }", "if a[i] > m { m } else { a[i] }", 1)
	if broken == maximum {
		t.Fatal("the broken maximum's edit did not apply")
	}
	refuse("a maximum keeping the smaller", broken, "invariant")
	// a read at the Skolem index k, proven equal to i, is the read at i: only
	// Ackermann's congruence relates the two atoms a[k] and a[i]
	const congruent = "use io\nfn at(a ∈ Array(Int), i ∈ Int) (r ∈ Int)\n    where 0 <= i && i < len(a)\n" +
		"    ensure ∀ k ∈ 0..len(a)-1 : k == i ⇒ a[k] == r\n{\n    a[i]\n}\n" +
		"export fn main() { io.print_int(at(array { 4, 7, 2 }, 1)) }\n"
	accept("a read at an index proven equal", congruent)
	// each fault, and the program it fails on
	for _, p := range []struct {
		name  string
		plant *bool
		src   string
		bad   bool // the fault accepts what is false, rather than refusing what is true
	}{
		{"the index set without the reads' indices", &emit.PlantIndexSetShort, sorted, false},
		{"reads not congruent", &emit.PlantReadsNotCongruent, congruent, false},
		{"tables' properties bypassed", &emit.PlantArraysBypassed, sorted, false},
		{"guards' terms unkept", &emit.PlantGuardsUnkept, bsearch, false},
		{"a formula invariant unchecked", &emit.PlantFormulaInvUnchecked, broken, true},
	} {
		*p.plant = true
		_, err := buildOnly(t, p.src, "go")
		*p.plant = false
		if p.bad && err != nil {
			t.Errorf("with %s the false program should be accepted, so the witness can fail: %v", p.name, err)
		}
		if !p.bad && err == nil {
			t.Errorf("with %s the program should be refused, so the witness can fail", p.name)
		}
	}
}

// A HOST'S QUANTIFIED PROMISE (ADR 0145's L5): GetWindowTextA's, that the text
// it wrote is a C string of r characters, believed with its sentence as
// evidence (gauntlet/stdlib/host-ensures-evidence.txt), and strnlen in the
// language proven equal to r from it. On both windows triples, a hidden
// window's text: 1 12 12 TinyRetroPad. And over a segment, t[8:40], the
// promise is of t shifted by 8: the NUL at t[8 + n] proven, at t[n] refused.
//
// Each fault refuses the program, or proves the false assertion:
//   - the written buffer read as the argument before the call;
//   - a quantifier-free invariant decided by linear facts alone where formulas
//     are in scope (cstrlen's i ≤ r needs the reads' congruence);
//   - the segment's reads left unshifted (the false assertion proven).
func TestAHostsQuantifiedPromiseIsBelieved(t *testing.T) {
	read := func(f string) string {
		src, err := os.ReadFile("../../examples/logic/" + f + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		return string(src)
	}
	text, segment := read("windowtext"), read("segment")
	for _, tg := range []string{"windows", "windows32"} {
		got, err := runOnMayFail(t, text, tg)
		if err != nil || strings.Join(strings.Fields(got), " ") != "1 12 12 TinyRetroPad" {
			t.Errorf("windowtext on %s: want 1 12 12 TinyRetroPad, got %q, %v", tg, got, err)
		}
		got, err = runOnMayFail(t, segment, tg)
		if err != nil || strings.TrimSpace(got) != "12" {
			t.Errorf("segment on %s: want 12, got %q, %v", tg, got, err)
		}
	}
	unshifted := strings.Replace(segment, "assert n > 0 ⇒ t[8 + n] == 0", "assert n > 0 ⇒ t[n] == 0", 1)
	if unshifted == segment {
		t.Fatal("the unshifted assertion's edit did not apply")
	}
	if _, err := buildOnly(t, unshifted, "windows"); err == nil || !strings.Contains(err.Error(), "an `assert` does not follow") {
		t.Errorf("the NUL claimed at t[n] of t[8:40]'s text should be refused: %v", err)
	}
	for _, p := range []struct {
		name  string
		plant *bool
	}{
		{"the written buffer read before the call", &emit.PlantInPlaceEnsureOld},
		{"an invariant by linear facts alone", &emit.PlantInvariantLinearWithFormulas},
	} {
		*p.plant = true
		_, err := buildOnly(t, text, "windows")
		*p.plant = false
		if err == nil {
			t.Errorf("with %s windowtext should be refused, so the witness can fail", p.name)
		}
	}
	emit.PlantSegmentReadsUnshifted = true
	_, err := buildOnly(t, unshifted, "windows")
	emit.PlantSegmentReadsUnshifted = false
	if err != nil {
		t.Errorf("with the segment's reads unshifted the false assertion should be proven, so the witness can fail: %v", err)
	}
}
