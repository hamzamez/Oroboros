package emit

import (
	"strings"
	"testing"
)

// theories.md §7.4: each admission condition refuses by name, and `lang`'s own
// facts are admitted.
func TestFactAdmissionIsTheFBFragment(t *testing.T) {
	if len(langFacts) != 8 {
		t.Fatalf("lang's theory must load all eight facts, got %d", len(langFacts))
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
