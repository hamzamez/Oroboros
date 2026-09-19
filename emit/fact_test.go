package emit

import (
	"math/rand"
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
		`(fact two ((x int) (y int)) (<= (% x 3) (% y 3)))`:          "F-C",
		`(fact cover ((x int) (y int)) (<= 0 (len x)))`:              "condition 3",
		`(fact nest ((x int)) (<= 0 (% (% x 3) 5)))`:                 "not flat",
		`(fact fl ((x f64)) (<= 0 (sqrt x)))`:                        "float",
		`(fact q ((a (array int))) (forall ((i int)) (<= 0 (a i))))`: "F-D",
		`(lemma l ((x int)) (<= 0 x))`:                               "F-E",
		`(fact lin ((x int)) (<= 0 x))`:                              "no trigger",
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

// remIHand is the transfer remI replaced, kept here as the bar the induced one
// must meet: |a % b| <= min(|a|, |b| − 1), sign of the dividend.
func remIHand(a, b ival) ival {
	m, have := int64(-1), false
	if a.bounded() {
		m, have = maxAbs(a), true
	}
	if b.bounded() {
		mb := maxAbs(b) - 1
		if mb < 0 {
			mb = 0
		}
		if !have || mb < m {
			m, have = mb, true
		}
	}
	if !have {
		return top
	}
	switch {
	case !a.loInf && a.lo >= 0:
		return ival{lo: 0, hi: m}
	case !a.hiInf && a.hi <= 0:
		return ival{lo: -m, hi: 0}
	}
	return ival{lo: -m, hi: m}
}

func randomIval(r *rand.Rand) ival {
	bounds := []int64{-1000000, -70000, -1000, -7, -1, 0, 1, 7, 1000, 70000, 1000000}
	lo, hi := bounds[r.Intn(len(bounds))], bounds[r.Intn(len(bounds))]
	if lo > hi {
		lo, hi = hi, lo
	}
	v := ival{lo: lo, hi: hi}
	switch r.Intn(8) {
	case 0:
		v.hiInf = true
	case 1:
		v.loInf = true
	}
	return v
}

// inside reports γ(a) ⊆ γ(b).
func inside(a, b ival) bool {
	return (b.loInf || (!a.loInf && a.lo >= b.lo)) && (b.hiInf || (!a.hiInf && a.hi <= b.hi))
}

// THE REFUTATION CONDITION OF theories.md §7.10, AS A TEST: an induced transfer
// proving less than the hand-written one would mean Theorem T needs a second
// encoding after all. It must be contained in the old remI everywhere — and on
// the case that motivated case splitting it must be strictly tighter.
func TestTheInducedRemainderIsNeverLessPrecise(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for i := 0; i < 50000; i++ {
		a, b := randomIval(r), randomIval(r)
		if got, bar := remI(a, b), remIHand(a, b); !inside(got, bar) {
			t.Fatalf("induced remI(%s, %s) = %s is wider than the hand-written %s", a, b, got, bar)
		}
	}
	if got := remI(ival{lo: -5, hi: 7}, exact(10)); got != (ival{lo: -5, hi: 7}) {
		t.Errorf("[-5,7] %% 10 should be [-5,7] by case splitting on the dividend's sign, got %s", got)
	}
	if got := remI(top, ival{lo: -7, hi: 7}); got != (ival{lo: -6, hi: 6}) {
		t.Errorf("a %% [-7,7] should be [-6,6]: the divisor spans both sign guards and 0 is undefined, got %s", got)
	}
}

// THE EVIDENCE CAN FAIL: weaken rem-divisor-pos's upper bound by one, and random
// containment finds a remainder outside the claim.
func TestWeakeningARemainderFactFailsContainment(t *testing.T) {
	src := strings.Replace(langFactsSrc, "(<= (% a b) (- b 1))", "(<= (% a b) (- b 2))", 1)
	if src == langFactsSrc {
		t.Fatal("the fact to weaken was not found in lang-facts.oro")
	}
	weak, err := readFacts(src)
	if err != nil {
		t.Fatal(err)
	}
	saved := langFacts
	langFacts = weak
	defer func() { langFacts = saved }()
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 20000; i++ {
		A, B := randomIval(r), randomIval(r)
		rv := remI(A, B)
		for s := 0; s < 12; s++ {
			a, ok1 := sample(r, A)
			b, ok2 := sample(r, B)
			if ok1 && ok2 && b != 0 && !holds(rv, a%b) {
				return // found: the weakened fact is caught
			}
		}
	}
	t.Error("a remainder fact weakened by one was never contradicted, so nothing checks it")
}

// andIHand is the mask transfer andI replaced, kept as its precision bar.
func andIHand(a, b ival) ival {
	if m, ok := exactNonNeg(b); ok {
		return ival{lo: 0, hi: m}
	}
	if m, ok := exactNonNeg(a); ok {
		return ival{lo: 0, hi: m}
	}
	if a.loInf || b.loInf || a.lo < 0 || b.lo < 0 {
		return top
	}
	hi := b
	if a.hiInf || (!b.hiInf && a.hi < b.hi) {
		hi = a
	}
	if hi.hiInf {
		return ival{lo: 0, hiInf: true}
	}
	return ival{lo: 0, hi: hi.hi}
}

// The same refutation condition for F9: the induced mask transfer inside the
// hand-written one everywhere, and strictly tighter where it should be.
func TestTheInducedMaskIsNeverLessPrecise(t *testing.T) {
	r := rand.New(rand.NewSource(13))
	for i := 0; i < 50000; i++ {
		a, b := randomIval(r), randomIval(r)
		if got, bar := andI(a, b), andIHand(a, b); !inside(got, bar) {
			t.Fatalf("induced andI(%s, %s) = %s is wider than the hand-written %s", a, b, got, bar)
		}
	}
	// A non-negative operand bounds the result though the other spans zero,
	// where the old rule answered top.
	if got := andI(ival{lo: 0, hi: 255}, ival{lo: -7, hi: 7}); got != (ival{lo: 0, hi: 255}) {
		t.Errorf("[0,255] & [-7,7] should be [0,255], got %s", got)
	}
}

// A ⊥ OPERAND IS READ AS UNKNOWN. The first induced transfers answered ⊥ for a
// ⊥ operand — `⊥ & 16777215` was ⊥ where the hand-written rule said
// [0, 16777215] — and the random precision tests never sample ⊥, so nothing
// compared them there. It was suspected of a windows byte narrowing and was not
// its cause (remfacts-2026-09-15 §6); it is pinned because the old transfers
// never answered ⊥ and nothing has measured a consumer that must accept one.
func TestABottomOperandIsReadAsUnknown(t *testing.T) {
	if got := andI(bottom, exact(16777215)); got != (ival{lo: 0, hi: 16777215}) {
		t.Errorf("⊥ & 16777215 must be [0, 16777215] — the mask bounds it whatever the other "+
			"operand is — got %s", got)
	}
	if got := remI(bottom, exact(10)); got != (ival{lo: -9, hi: 9}) {
		t.Errorf("⊥ %% 10 must be [-9, 9], got %s", got)
	}
}

// THE EVIDENCE CAN FAIL: weaken and-right's upper bound by one, and random
// sampling finds a mask outside the claim.
func TestWeakeningAMaskFactFailsContainment(t *testing.T) {
	src := strings.Replace(langFactsSrc, "(<= (& a b) b)", "(<= (& a b) (- b 1))", 1)
	if src == langFactsSrc {
		t.Fatal("the fact to weaken was not found in lang-facts.oro")
	}
	weak, err := readFacts(src)
	if err != nil {
		t.Fatal(err)
	}
	saved := langFacts
	langFacts = weak
	defer func() { langFacts = saved }()
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 20000; i++ {
		A, B := randomIval(r), randomIval(r)
		av := andI(A, B)
		for s := 0; s < 12; s++ {
			a, ok1 := sample(r, A)
			b, ok2 := sample(r, B)
			if ok1 && ok2 && !holds(av, a&b) {
				return
			}
		}
	}
	t.Error("a mask fact weakened by one was never contradicted, so nothing checks it")
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
