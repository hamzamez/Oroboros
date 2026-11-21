package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/presburger"
)

const s64 = "-2^63..2^63-1"

// THE MIDPOINT OVER FULL WORDS (the book's §6.9, finding 108), each rule
// against the planted fault that undoes it, which must fail first.
//
//  1. `x ∈ Int = e` is ℤ as a claim, the consent, on every target
//     (core.AscribedSet): PlantZAscribedAsWord leaves it the word's spelling,
//     and Go refuses the sum, where windows accepted it. A helper asks with a
//     set above the word, `(-2^64..2^64)`, on every target; its `(Int)` is a
//     claim and no demand (finding 116).
func TestIntIsAConsentOnEveryTarget(t *testing.T) {
	// THE WORDS' AND E'S OWN RULE, as -words decides it: past them a value
	// is ℤ's under ADR 0150, and this witness is about the readings
	ir.WordsOnly = true
	defer func() { ir.WordsOnly = false }()
	binder := "export fn mid(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (" + s64 + ") {\n    sum ∈ Int = lo + hi\n    sum / 2\n}\n"
	helper := "fn add(a ∈ Int, b ∈ Int) (-2^64..2^64) { a + b }\n\nexport fn mid(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (" + s64 + ") { add(lo, hi) / 2 }\n"
	for what, src := range map[string]string{"a binder": binder, "a helper": helper} {
		for _, tg := range []string{"go", "java", "js", "windows"} {
			if _, err := surfaceGen(t, tg, map[string]string{"p.oro": src}); err != nil {
				t.Errorf("%s on %s: want (lo + hi) / 2 held exactly, got %v", what, tg, err)
			}
		}
	}
	// THE CONSENT IS SEEN PAST E: the midpoint's sum lies in the double word,
	// computed there without asking (ADR 0149); a product of three words, up
	// to 2¹⁸⁹, is ℤ's only where the binder asks
	product := "export fn f(a ∈ " + s64 + ", b ∈ " + s64 + ", c ∈ " + s64 + ") (-6..6) {\n    p ∈ Int = a * b * c\n    p % 7\n}\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": product}); err != nil {
		t.Errorf("a product of three words claimed Int: want it held exactly, got %v", err)
	}
	core.PlantZAscribedAsWord = true
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": product})
	core.PlantZAscribedAsWord = false
	if err == nil {
		t.Error("a binder with Int ascribed as the word: Go should refuse the product, so the witness can fail")
	}
}

//  2. BLOCH'S FORM, lo + (hi − lo) / 2 under lo <= hi, is proven in the word
//     with no big integer: hi − lo ∈ [0, 2⁶⁴ − 1] by the `where`'s difference
//     (assumeDiff), the unsigned word; lo + q ∈ [lo, hi] by Presburger
//     arithmetic (relate), the signed one; the claim by the logic where the
//     intervals' join falls short (DecideRangeClaim). Each fault refuses it:
//     the relation not read (PlantRelateOff), the unsigned word's operations
//     read as atoms by the decision after the words pass
//     (PlantRelateWordsOpaque), the claim left to the intervals
//     (PlantRangeClaimUnread). On Go the code is Bloch's correction itself.
func TestBlochsMidpointIsProven(t *testing.T) {
	const src = "export fn mid(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (" + s64 + ") where lo <= hi {\n    lo + (hi - lo) / 2\n}\n"
	gen := func(tg string) (string, error) { return surfaceGen(t, tg, map[string]string{"p.oro": src}) }
	code, err := gen("go")
	if err != nil {
		t.Fatalf("Bloch's midpoint on Go: %v", err)
	}
	for _, want := range []string{"(uint64(v1))", "(uint64(v0))", "int(v36)", "v0 + v37"} {
		if !strings.Contains(code, want) {
			t.Errorf("on Go, want the unsigned difference and the signed sum, %q, in\n%s", want, code)
		}
	}
	if strings.Contains(code, "big.Int") {
		t.Errorf("on Go, want no big integer:\n%s", code)
	}
	for _, tg := range []string{"java", "js", "windows", "windows32"} {
		if _, err := gen(tg); err != nil {
			t.Errorf("Bloch's midpoint on %s: %v", tg, err)
		}
	}
	// a claim tighter than the word: relate meets the box with the word only,
	// [0, 3·2⁶¹] here, and the claim 0..2⁶² is the logic's, lo ≤ m ≤ hi
	const narrow = "export fn mid(lo ∈ 0..2^62, hi ∈ 0..2^62) (0..2^62) where lo <= hi { lo + (hi - lo) / 2 }\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": narrow}); err != nil {
		t.Fatalf("the midpoint of 0..2^62 claimed in it: %v", err)
	}
	emit.PlantRangeClaimUnread = true
	_, err = surfaceGen(t, "go", map[string]string{"p.oro": narrow})
	emit.PlantRangeClaimUnread = false
	if err == nil || !strings.Contains(err.Error(), "does not establish") {
		t.Errorf("with the claim left to the intervals, 0..2^62 should not be established: %v", err)
	}
	for name, plant := range map[string]*bool{
		"PlantRelateOff":         &ir.PlantRelateOff,
		"PlantRelateWordsOpaque": &ir.PlantRelateWordsOpaque,
	} {
		*plant = true
		_, err := gen("go")
		*plant = false
		if err == nil {
			t.Errorf("with %s, Bloch's midpoint should be refused on Go, so the witness can fail", name)
		}
	}
	// and without the `where`, hi − lo leaves both words and lies in E: the
	// double word's (ADR 0149), Go's math/bits
	if code, err := surfaceGen(t, "go", map[string]string{"p.oro": strings.Replace(src, " where lo <= hi", "", 1)}); err != nil ||
		!strings.Contains(code, "bits.Sub64") {
		t.Errorf("the midpoint without lo <= hi: want hi − lo in the double word, got %v\n%s", err, code)
	}
}

//  3. THE RELATIONS ARE READ WHERE THEY HOLD. A `where` assumed false is its
//     negation: under `!(lo <= hi)`, hi − lo < 0, and a result `(Nat)` of it is
//     false of every argument; PlantAssumedDiffUnnegated records lo <= hi as if
//     it held, and accepts it. A guard holds on its own arm: in the else arm of
//     `if lo <= hi`, hi − lo ∈ [−2⁶⁴ + 1, −1] leaves the word;
//     PlantRelateGuardAnyArm reads the then arm's guard there too, a
//     contradiction from which the word follows, and accepts it.
func TestTheRelationsAreReadWhereTheyHold(t *testing.T) {
	negated := "export fn d(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (Nat) where !(lo <= hi) { hi - lo }\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": negated}); err == nil {
		t.Fatal("hi − lo under !(lo <= hi) claimed Nat: want it refused")
	}
	ir.PlantAssumedDiffUnnegated = true
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": negated})
	ir.PlantAssumedDiffUnnegated = false
	if err != nil {
		t.Errorf("with a negated where read as holding, the false claim should be accepted, so the witness can fail: %v", err)
	}
	arm := "export fn f(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (" + s64 + ") { if lo <= hi { 0 } else { hi - lo } }\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": arm}); err == nil {
		t.Fatal("hi − lo on the arm where lo > hi: want it refused, it leaves the word")
	}
	ir.PlantRelateGuardAnyArm = true
	_, err = surfaceGen(t, "go", map[string]string{"p.oro": arm})
	ir.PlantRelateGuardAnyArm = false
	if err != nil {
		t.Errorf("with every arm's guard read on each, hi − lo should be accepted, so the witness can fail: %v", err)
	}
	// and a product of two variables stays the box's: n · n over ±2⁴⁰ reaches
	// 2⁸⁰, past the word, refused
	square := "export fn sq(n ∈ -2^40..2^40) (" + s64 + ") { n * n }\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": square}); err == nil {
		t.Error("n · n over ±2⁴⁰ claimed in the word: want it refused")
	}
}

//  4. CONSTANTS PAST INT64. An obligation at the word's ends,
//     bot + n <= top − n, is −2⁶³ − (2⁶³ − 1) + 2n <= 0, no linear term of the
//     refinement layer's int64 constants: the logic decides it
//     (PlantObligationLinearOnly refuses it). The Omega test's own constants
//     are exact (PlantConstantWord holds them in int64 and the claim
//     lo <= m <= hi of Bloch's form is unknown, so refused). And on JavaScript,
//     whose word is ±(2⁵³ − 1), S₆₄'s end claimed `∈ Int` is held exactly as
//     Horner's form (PlantConstantNotSpined refuses it).
func TestConstantsPastInt64(t *testing.T) {
	// THE WORDS' AND E'S OWN RULE, as -words decides it: past them a value
	// is ℤ's under ADR 0150, and this witness is about the readings
	ir.WordsOnly = true
	defer func() { ir.WordsOnly = false }()
	call := "fn first(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (" + s64 + ") where lo <= hi { lo }\n\n" +
		"export fn g(n ∈ 0..5) (" + s64 + ") {\n    bot ∈ Int = -9223372036854775808\n    top ∈ Int = 9223372036854775807\n    first(bot + n, top - n)\n}\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": call}); err != nil {
		t.Fatalf("an obligation at S₆₄'s ends: %v", err)
	}
	emit.PlantObligationLinearOnly = true
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": call})
	emit.PlantObligationLinearOnly = false
	if err == nil || !strings.Contains(err.Error(), "does not follow") {
		t.Errorf("with the linear fragment alone, the obligation should not follow: %v", err)
	}

	ensure := "export fn mid(lo ∈ " + s64 + ", hi ∈ " + s64 + ") (m ∈ " + s64 + ") where lo <= hi ensure lo <= m && m <= hi {\n    lo + (hi - lo) / 2\n}\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": ensure}); err != nil {
		t.Fatalf("lo <= m <= hi of Bloch's form: %v", err)
	}
	presburger.PlantConstantWord = true
	_, err = surfaceGen(t, "go", map[string]string{"p.oro": ensure})
	presburger.PlantConstantWord = false
	if err == nil {
		t.Error("with the Omega test's constants int64, lo <= m <= hi should be refused, so the witness can fail")
	}

	spine := "export fn g(n ∈ 0..5) (Int) {\n    top ∈ Int = 9223372036854775807\n    top - n\n}\n"
	if code, err := surfaceGen(t, "js", map[string]string{"p.oro": spine}); err != nil || !strings.Contains(code, "BigInt(372036854775807)") {
		t.Fatalf("2⁶³ − 1 claimed Int on JavaScript: want its Horner form in BigInt: %v\n%s", err, code)
	}
	ir.PlantConstantNotSpined = true
	_, err = surfaceGen(t, "js", map[string]string{"p.oro": spine})
	ir.PlantConstantNotSpined = false
	if err == nil {
		t.Error("with a constant past the word refused where widened, 2⁶³ − 1 should be refused on JavaScript")
	}
}
