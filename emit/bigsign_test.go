package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// ABOVE THE WORD, ONE SET ON EVERY REPRESENTATION (ADR 0029). The run-time half
// is pinned by the differential cases big-sign and big-negmid, which trap on the
// same value on every host and storage; these pin the compile-time half.

// THE SET HAS A SIGN. A program's enforced set is the join of its types' least
// members of H: [0, 2^b) while every type above the word is non-negative, and
// (−2^b, 2^b) once one admits a negative.
func TestTheEnforcedSetHasASign(t *testing.T) {
	nat := resultSig(t, "(int 0 (pow 2 200))")
	zed := resultSig(t, "(int (- 0 (pow 2 100)) (pow 2 100))")
	if bits, signed, ok := BigHull(goWord, nat); !ok || bits != 201 || signed {
		t.Errorf("(int 0 2^200) enforces %d bits, signed=%v, ok=%v; want [0, 2^201)", bits, signed, ok)
	}
	if bits, signed, ok := BigHull(goWord, nat, zed); !ok || bits != 201 || !signed {
		t.Errorf("with a signed type the join is %d bits, signed=%v, ok=%v; want (−2^201, 2^201)", bits, signed, ok)
	}
}

// promoteSigned reduces a program's first export and selects its representation.
func promoteSigned(t *testing.T, tg *Target, src string) (string, error) {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	var all []*core.Sig
	for _, s := range prog.Sigs {
		all = append(all, s)
	}
	out, _, err := PromoteBig(tg, prog.Sigs[q], nf, all...)
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

const signedProgram = `(export f)
(sig f ((n (int 0 10))) (int (- 0 (pow 2 200)) (pow 2 200)))
(def f (n) (- (- 0 1606938044258990275541962092341162602522202993782792835301376) n))`

const naturalProgram = `(export f)
(sig f ((n (int 0 10))) (int 0 (pow 2 200)))
(def f (n) (+ 1606938044258990275541962092341162602522202993782792835301376 n))`

// EACH SET HAS ITS OWN CHECK, and the limbs realize only [0, 2^k): a signed
// program takes the host's bignum where there is one, and is refused where
// there is none, never trapped on a declared value.
func TestTheLimbsHoldOnlyANonNegativeSet(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	out, err := promoteSigned(t, tg, signedProgram)
	if err != nil || !strings.Contains(out, "big-fit-signed") {
		t.Errorf("a signed program is enforced by big-fit-signed: %v\n%s", err, out)
	}
	out, err = promoteSigned(t, tg, naturalProgram)
	if err != nil || !strings.Contains(out, "(big-fit ") {
		t.Errorf("a non-negative program is enforced by big-fit: %v\n%s", err, out)
	}

	tg.BigRepr = "limbs"
	sig := resultSig(t, "(int (- 0 (pow 2 200)) (pow 2 200))")
	if limbs, _, _ := BigRepr(tg, sig); limbs {
		t.Error("a signed program was put on the limb rung of a target that has a bignum")
	}
	if limbs, _, _ := BigRepr(tg, resultSig(t, "(int 0 (pow 2 200))")); !limbs {
		t.Error("a non-negative program asked for limbs did not get them")
	}

	if _, err := promoteSigned(t, winTarget(t), signedProgram); err == nil || !strings.Contains(err.Error(), "NEGATIVE") {
		t.Errorf("windows has only limbs, so a signed range above the word is refused; got %v", err)
	}
	if _, err := promoteSigned(t, winTarget(t), naturalProgram); err != nil {
		t.Errorf("windows takes a non-negative range above the word on limbs: %v", err)
	}
}

// THE SIGN IS PART OF A PARAMETER'S SET, as an obligation at a call: a negative
// value is not in `(int 0 (pow 2 200))`, whether it arrives as a literal or as
// an interval the analysis found.
func TestTheSignIsPartOfAParametersSet(t *testing.T) {
	const g = `(sig g ((x (int LO (pow 2 200)))) int)
(def g (x) (if (= x 0) 0 1))
(export f)
(sig f ((n (int -5 5))) int)
`
	nat := strings.Replace(g, "LO", "0", 1)
	zed := strings.Replace(g, "LO", "(- 0 (pow 2 200))", 1)
	if _, err := dischargeGo(t, nat+`(def f (n) (g -5))`); err == nil || !strings.Contains(err.Error(), "a call passes -5") {
		t.Errorf("-5 is not in [0, 2^201); got %v", err)
	}
	if _, err := dischargeGo(t, zed+`(def f (n) (g -5))`); err != nil {
		t.Errorf("-5 is in (−2^201, 2^201): %v", err)
	}
	if _, err := dischargeGo(t, nat+`(def f (n) (g n))`); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("n ∈ [−5, 5] is not in [0, 2^201); got %v", err)
	}
	if _, err := dischargeGo(t, nat+`(def f (n) (g (+ n 5)))`); err != nil {
		t.Errorf("n + 5 ∈ [0, 10] is in [0, 2^201): %v", err)
	}
}

// A DECLARED RESULT ABOVE THE WORD TELLS ONLY WHAT THE PROGRAM ENFORCES. The
// bound is one per program, so `h`'s declared 2^100 is checked at the program's
// 2^201, and h may return 2^150: read as its own type, the ascription proved
// g's obligation falsely, and g received 2^150 (ADR 0028's reading, corrected
// by ADR 0029). A result as wide as the program's widest type still proves one.
func TestADeclaredResultIsAFactAboutTheProgramsSet(t *testing.T) {
	const src = `(sig h ((n (int 0 1))) (int 0 (pow 2 100)))
(def h (n) (* 1427247692705959881058285969449495136382746624 (+ n 1)))
(sig g ((x (int 0 (pow 2 100)))) (int 0 (pow 2 200)))
(def g (x) (+ x 0))
(export calc)
(sig calc ((n (int 0 1))) (int 0 (pow 2 WIDEST)))
(def calc (n) (g (h n)))`
	if _, err := dischargeGo(t, strings.Replace(src, "WIDEST", "200", 1)); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("h's 2^100 is enforced only at the program's 2^201; got %v", err)
	}
	// With nothing wider than 2^100 in the program, E_P = [0, 2^101) = g's set.
	one := strings.Replace(strings.Replace(src, "WIDEST", "100", 1),
		"(int 0 (pow 2 200)))\n(def g", "(int 0 (pow 2 100)))\n(def g", 1)
	if !strings.Contains(one, "(sig g ((x (int 0 (pow 2 100)))) (int 0 (pow 2 100)))") {
		t.Fatal("the test did not narrow g's result")
	}
	if _, err := dischargeGo(t, one); err != nil {
		t.Errorf("one bound in the program: the ascription is its set: %v", err)
	}
	// And the sign: once the program enforces (−2^101, 2^101), h's non-negative
	// declaration is not enforced as non-negative, so it proves nothing about
	// g's [0, 2^101).
	signed := strings.Replace(one, "(sig calc ((n (int 0 1))) (int 0 (pow 2 100)))",
		"(sig calc ((n (int 0 1))) (int (- 0 (pow 2 100)) (pow 2 100)))", 1)
	if signed == one {
		t.Fatal("the test did not sign calc's result")
	}
	if _, err := dischargeGo(t, signed); err == nil || !strings.Contains(err.Error(), "g's parameter x") {
		t.Errorf("a signed program's ascription does not prove a non-negative set; got %v", err)
	}
}
