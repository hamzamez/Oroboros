package emit

import (
	"strings"
	"testing"
)

// theories.md §7.4: each admission condition refuses by name, and `lang`'s own
// facts are admitted.
func TestFactAdmissionIsTheFBFragment(t *testing.T) {
	if len(langFacts) != 11 {
		t.Fatalf("lang's theory must load all eleven facts, got %d", len(langFacts))
	}
	refused := map[string]string{
		`fact two (x ∈ Int, y ∈ Int) x % 3 <= y % 3`:          "F-C",
		`fact cover (x ∈ Int, y ∈ Int) 0 <= len(x)`:           "condition 3",
		`fact nest (x ∈ Int) 0 <= x % 3 % 5`:                  "not flat",
		`fact fl (x ∈ F64) 0 <= sqrt(x)`:                      "float",
		`fact q (a ∈ Array(Int)) forall(i(int)(), 0 <= a(i))`: "F-D",
		`lemma l (x ∈ Int) 0 <= x`:                            "F-E",
		`fact lin (x ∈ Int) 0 <= x`:                           "no trigger",
	}
	for src, want := range refused {
		_, err := readFacts(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal naming %q, got %v", src, want, err)
		}
	}
}

// hex.Decode's witness, TestBothHalvesOfDivisionByALiteralAreKnown, as one query.
const decodeTarget = "fn `/`(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} / ${b}\" }\nfn half(dst ∈ Array(Int), src ∈ Array(Int)) (Int)\n    where len(src) <= 2 * len(dst) + 1 {\n    host expr \"half(${dst}, ${src})\"\n}"

const decodeQuery = `use tgt
fn t(a) { build(len(a) / 2, fn(b) { u = tgt.half(b, a); b }) }`

// proves runs the Decode query under a theory and reports whether the
// precondition is PROVEN — neither refused nor merely propagated.
func proves(t *testing.T, theory []*Fact) bool {
	t.Helper()
	saved := langFacts
	langFacts = theory
	defer func() { langFacts = saved }()
	notes, err := refineWith(t, tempTarget(t, decodeTarget), decodeQuery)
	return err == nil && !propagated(notes)
}

// THE EVIDENCE CAN FAIL (theories.md §7.10, item 2): deleting div-floor's upper
// half fails hex.Decode's witness.
func TestDeletingDivFloorsUpperHalfFailsTheDecodeWitness(t *testing.T) {
	if !proves(t, langFacts) {
		t.Fatal("the control, lang's full theory, must prove Decode's precondition")
	}
	weak, err := readFacts(`fact len_nonneg (t ∈ Array(A)) 0 <= len(t)
fact div_floor (x ∈ Int, k ∈ Int) when(0 <= x, 1 <= k) k * (x / k) <= x`)
	if err != nil {
		t.Fatal(err)
	}
	if proves(t, weak) {
		t.Error("with div-floor's upper half deleted Decode's precondition was still proven, " +
			"so nothing checks that half")
	}
}

// And len-nonneg is load-bearing: without it div-floor's guard 0 <= x is not
// entailed for a length, and neither half is instantiated.
func TestDivFloorNeedsLenNonneg(t *testing.T) {
	noLen, err := readFacts(`fact div_floor (x ∈ Int, k ∈ Int) when(0 <= x, 1 <= k) k * (x / k) <= x && x <=
k * (x / k) + (k - 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if proves(t, noLen) {
		t.Error("div-floor's guard was taken as true without len-nonneg to entail it")
	}
}

// THE MIDPOINT IS PROVEN WHERE ITS GUARD IS (the book's finding 67): binary
// search's lo + (hi − lo) / 2 is an index only if q = (hi − lo) / 2 ≥ 0.
// div_floor gives 2q ≥ d − 1 for d = hi − lo ≥ 0, and the negated goal
// lo + q ≤ −1 (tightened over ℤ) contradicts it with lo ≥ 0, strict or not.
// But div_floor's guard 0 <= d is entailed only inside the `if`, and the root
// instantiated facts once, with the root's facts, so the instance was never
// made. Instantiated where the obligation is (instantiatedAt) both are
// proven; with the root's instances alone (PlantNoLocalInstances) refused.
func TestTheMidpointNeedsTheQuotientsSign(t *testing.T) {
	const target = "fn `/`(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} / ${b}\" }"
	strict := "use tgt" + "\n" + "fn t(a ∈ Array(Int), lo ∈ Int, hi ∈ Int) (Int) {" + "\n" +
		"    if 0 <= lo && lo < hi && hi <= len(a) { a[lo + (hi - lo) / 2] } else { 0 }" + "\n" + "}"
	loose := "use tgt" + "\n" + "fn t(a ∈ Array(Int), lo ∈ Int, hi ∈ Int) (Int) {" + "\n" +
		"    if 0 <= lo && lo <= hi && hi < len(a) { a[lo + (hi - lo) / 2] } else { 0 }" + "\n" + "}"
	run := func(theory []*Fact, query string) error {
		t.Helper()
		saved := langFacts
		langFacts = theory
		defer func() { langFacts = saved }()
		_, err := refineWith(t, tempTarget(t, target), query)
		return err
	}
	for what, q := range map[string]string{"strict": strict, "non-strict": loose} {
		if err := run(langFacts, q); err != nil {
			t.Errorf("the %s midpoint must be proven an index under lang's theory: %v", what, err)
		}
	}
	PlantNoLocalInstances = true
	defer func() { PlantNoLocalInstances = false }()
	for what, q := range map[string]string{"strict": strict, "non-strict": loose} {
		if err := run(langFacts, q); err == nil || !strings.Contains(err.Error(), "0 <= lo") {
			t.Errorf("with the facts instantiated at the root alone the %s midpoint should be refused; got %v", what, err)
		}
	}
}

// F12 IS LOAD-BEARING, EACH HALF (a store owes its index since the book's
// finding 70, and the sieve's crossing loop stores at j from i·i): with the
// square an atom, 0 <= x·x proves the read a[i·i] under i·i < len(a), and
// x <= x·x proves a[i] under i·i + 1 <= len(a), a guard squareBound's
// pattern does not match. Each is refused with its half deleted, and both with
// the square kept outside the fragment (PlantSquareOpaque).
func TestASquareIsAtLeastItsRoot(t *testing.T) {
	const target = "fn `/`(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} / ${b}\" }"
	at := "use tgt\nfn t(a ∈ Array(Int), i ∈ Int) (Int) {\n    if i * i < len(a) { a[i * i] } else { 0 }\n}"
	root := "use tgt\nfn t(a ∈ Array(Int), i ∈ Int) (Int) {\n    if 0 <= i && i * i + 1 <= len(a) { a[i] } else { 0 }\n}"
	run := func(theory []*Fact, query string) error {
		t.Helper()
		saved := langFacts
		langFacts = theory
		defer func() { langFacts = saved }()
		_, err := refineWith(t, tempTarget(t, target), query)
		return err
	}
	for what, q := range map[string]string{"a[i·i]": at, "a[i]": root} {
		if err := run(langFacts, q); err != nil {
			t.Errorf("%s must be proven an index under lang's theory: %v", what, err)
		}
	}
	replace := func(fact string) []*Fact {
		t.Helper()
		var out []*Fact
		for _, f := range langFacts {
			if f.Name != "square" {
				out = append(out, f)
			}
		}
		if len(out) != len(langFacts)-1 {
			t.Fatalf("square not found among lang's facts")
		}
		if fact != "" {
			half, err := readFacts(fact)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, half...)
		}
		return out
	}
	if err := run(replace("fact square (x ∈ Int) x <= x * x"), at); err == nil {
		t.Error("with 0 <= x·x deleted, a[i·i] should be refused")
	}
	if err := run(replace("fact square (x ∈ Int) 0 <= x * x"), root); err == nil {
		t.Error("with x <= x·x deleted, a[i] should be refused")
	}
	PlantSquareOpaque = true
	defer func() { PlantSquareOpaque = false }()
	for what, q := range map[string]string{"a[i·i]": at, "a[i]": root} {
		if err := run(langFacts, q); err == nil {
			t.Errorf("with the square outside the fragment, %s should be refused", what)
		}
	}
}

// F13 IS LOAD-BEARING, EACH CLAUSE: of two naturals x | y and x ⊕ y are
// naturals, so each is an index of a table read under its upper bound. With
// the clause deleted the read is refused.
func TestTheSignOfOrAndXorIsTheirOperands(t *testing.T) {
	const target = "fn `/`(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} / ${b}\" }\n" +
		"fn or(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} | ${b}\" }\n" +
		"fn xor(a ∈ Int, b ∈ Int) (Int) pure { host expr \"${a} ^ ${b}\" }"
	query := func(op string) string {
		return "use tgt\nfn t(a ∈ Array(Int), x ∈ Int, y ∈ Int) (Int) {\n    if 0 <= x && 0 <= y && 16 <= len(a) && x " + op +
			" y < 16 { a[x " + op + " y] } else { 0 }\n}"
	}
	run := func(theory []*Fact, q string) error {
		t.Helper()
		saved := langFacts
		langFacts = theory
		defer func() { langFacts = saved }()
		_, err := refineWith(t, tempTarget(t, target), q)
		return err
	}
	without := func(name string) []*Fact {
		var out []*Fact
		for _, f := range langFacts {
			if f.Name != name && f.Name != strings.ReplaceAll(name, "-", "_") {
				out = append(out, f)
			}
		}
		if len(out) != len(langFacts)-1 {
			t.Fatalf("%s not found among lang's facts", name)
		}
		return out
	}
	for op, name := range map[string]string{"|": "or-nonneg", "⊕": "xor-nonneg"} {
		if err := run(langFacts, query(op)); err != nil {
			t.Errorf("a[x %s y] of two naturals must be proven an index: %v", op, err)
		}
		if err := run(without(name), query(op)); err == nil {
			t.Errorf("with %s deleted a[x %s y] should be refused", name, op)
		}
	}
}
