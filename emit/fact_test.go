package emit

import (
	"strings"
	"testing"
)

// theories.md §7.4: each admission condition refuses by name, and `lang`'s own
// facts are admitted.
func TestFactAdmissionIsTheFBFragment(t *testing.T) {
	if len(langFacts) != 2 {
		t.Fatalf("lang's theory must load both facts, got %d", len(langFacts))
	}
	refused := map[string]string{
		`(fact two ((x int) (y int)) (<= (% x 3) (% y 3)))`:           "F-C",
		`(fact cover ((x int) (y int)) (<= 0 (len x)))`:               "condition 3",
		`(fact nest ((x int)) (<= 0 (% (% x 3) 5)))`:                  "not flat",
		`(fact fl ((x f64)) (<= 0 (sqrt x)))`:                         "float",
		`(fact q ((a (array int))) (forall ((i int)) (<= 0 (a i))))`: "F-D",
		`(lemma l ((x int)) (<= 0 x))`:                                "F-E",
		`(fact lin ((x int)) (<= 0 x))`:                               "no trigger",
	}
	for src, want := range refused {
		_, err := readFacts(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal naming %q, got %v", src, want, err)
		}
	}
}

// hex.Decode's witness, TestBothHalvesOfDivisionByALiteralAreKnown, as one query.
const decodeTarget = `(sig / ((a int) (b int)) int pure (host expr "%s / %s"))
    (sig half ((dst (array int)) (src (array int))) int
      (where (<= (len src) (+ (* 2 (len dst)) 1))) (host expr "half(%s, %s)"))`

const decodeQuery = `(use tgt) (fn (a) (build (/ (len a) 2) (fn (b) (let (tgt.half b a) (fn (u) b)))))`

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
	weak, err := readFacts(`
(fact len-nonneg ((t (array A))) (<= 0 (len t)))
(fact div-floor ((x int) (k int)) (when (<= 0 x) (<= 1 k)) (<= (* k (/ x k)) x))`)
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
	noLen, err := readFacts(`
(fact div-floor ((x int) (k int)) (when (<= 0 x) (<= 1 k))
  (and (<= (* k (/ x k)) x) (<= x (+ (* k (/ x k)) (- k 1)))))`)
	if err != nil {
		t.Fatal(err)
	}
	if proves(t, noLen) {
		t.Error("div-floor's guard was taken as true without len-nonneg to entail it")
	}
}
