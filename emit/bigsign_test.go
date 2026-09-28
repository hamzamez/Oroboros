package emit

import (
	"testing"
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

// THE SIGN IS PART OF A PARAMETER'S SET, and A DECLARED RESULT ABOVE THE WORD
// TELLS ONLY WHAT THE PROGRAM ENFORCES: both are obligations at a call, decided
// through the drivers' pipeline, so their tests are in requires_test.go.

// A SIGNED PROGRAM IS NOT PUT ON LIMBS where the target has a bignum, and a
// non-negative one asked for limbs gets them (ADR 0029, decision 4).
func TestASignedProgramIsNotPutOnLimbs(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.BigRepr = "limbs"
	if limbs, _, _ := BigRepr(tg, resultSig(t, "(int (- 0 (pow 2 200)) (pow 2 200))")); limbs {
		t.Error("a signed program was put on the limb rung of a target that has a bignum")
	}
	if limbs, _, _ := BigRepr(tg, resultSig(t, "(int 0 (pow 2 200))")); !limbs {
		t.Error("a non-negative program asked for limbs did not get them")
	}
}
