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

// THE SIGN IS PART OF A PARAMETER'S SET, and A DECLARED RESULT ABOVE THE WORD
// TELLS ONLY WHAT THE PROGRAM ENFORCES: both are obligations at a call, decided
// through the drivers' pipeline, so their tests are in requires_test.go.
