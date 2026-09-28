package emit

import (
	"math/big"
	"strings"
	"testing"

	"oroboros/core"
)

// THE FIXED-LIMB RUNG (emit/biglimb.go), ADR 0019's ladder, third step.
//
// Until this, a declared endpoint was compared against the portable window and
// thrown away: `(int 0 (pow 2 1000))` and `(int 0 +inf)` produced identical
// code. Having the spelling was necessary and nowhere near sufficient.

// THE DECLARATION GIVES A BOUND. IT DOES NOT CHOOSE A REPRESENTATION.
//
// That separation is the point and it was got wrong at first: a finite range
// selected fixed limbs and `+inf` selected the host's bignum, so the SHAPE of a
// declaration decided the storage. On V8 that cost 100x for the same
// computation. `(int 0 (pow 2 1300))` says the value is a mathematical integer
// in that interval — a fact about the program, true on every target — and ADR
// 0003 has said since the beginning that this is not the same question as how a
// host stores it.
//
// ℤ has no bound to enforce, which is what `+inf` is for.
// goWord is the word the Go, JVM and windows target files declare (ADR 0026).
var goWord = core.Word{Lo: -1 << 63, Hi: 1<<63 - 1}

// AND THE TARGET CHOOSES THE REPRESENTATION — the same signature, four hosts,
// two answers, and no program changes to get either.
//
// This is `int-repr` one rung up: `(int 0 255)` is a `[]byte` on Go and a
// `short[]` on the JVM because the JVM's byte is signed, and the programmer
// writes neither. There is no total order to select from here the way there is
// for widths, because bigarith-2026-08-28 measured ours winning where the
// operation is LINEAR and the host winning where it is QUADRATIC — so a target
// declares what somebody measured.
func TestTheTargetChoosesTheRepresentation(t *testing.T) {
	sig := resultSig(t, "(int 0 (pow 2 1300))")
	for _, c := range []struct {
		dir   string
		limbs bool
	}{
		{"../targets/go", false},     // math/big: 2,186 ns against 18,150 in limbs
		{"../targets/js", false},     // BigInt: 5,290 against 528,334 — a factor of 100
		{"../targets/java", false},   // BigInteger: 2,905 against 7,948
		{"../targets/windows", true}, // ships no bignum, so limbs or nothing
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		limbs, w, bits := BigRepr(tg, sig)
		if limbs != c.limbs {
			t.Errorf("%s: limbs=%v, want %v", c.dir, limbs, c.limbs)
		}
		if bits != 1301 {
			t.Errorf("%s: bound is %d bits, want 1301 — the BOUND is the "+
				"declaration's and does not depend on the target", c.dir, bits)
		}
		if c.limbs && w != 55 {
			t.Errorf("%s: width %d, want 55", c.dir, w)
		}
	}
}

// AND BOTH REPRESENTATIONS ADMIT EXACTLY THE SAME VALUES, which is the property
// that makes the choice above a choice of STORAGE rather than of ANSWER.
//
// The host rung refuses a value needing more than `bits` bits. The limb rung
// has `w = ceil(bits/24)` limbs, so its carry check alone would admit
// everything under 2^(24w) — up to 23 bits more than was declared. `limbLimit`
// is the ceiling on the top limb that closes exactly that gap.
//
// Without it `(big-repr host)` would not be a change of representation but a
// change of which programs are legal, which is ADR 0009's rule broken at the
// representation boundary.
func TestBothRepresentationsAdmitTheSameValues(t *testing.T) {
	for bits := 2*limbBits + 1; bits <= 2400; bits++ {
		w := (bits + limbBits - 1) / limbBits
		// The largest value the limb rung accepts, plus one: the top limb reaches
		// `limbLimit` and every limb below it is full.
		limb := new(big.Int).Lsh(big.NewInt(limbLimit(bits, w)), uint(limbBits*(w-1)))
		host := new(big.Int).Lsh(big.NewInt(1), uint(bits)) // BitLen() > bits
		if limb.Cmp(host) != 0 {
			t.Fatalf("at %d bits the limb rung admits under %s and the host rung "+
				"under %s; a declaration would mean two different things",
				bits, limb, host)
		}
	}
}

// EVERY TARGET CAN FAIL, AND THAT IS WHAT MAKES A FIXED WIDTH SOUND.
//
// The host's bignum is exact whatever the declaration says, so an
// under-declared bound costs nothing there. A fixed width TRUNCATES — and then
// selecting a representation would change the answer, which is ADR 0009's rule
// at a different boundary and the one thing this project refuses.
//
// `panic`, `throw`, `throw`, `ud2`. windows needs it most, because it ships no
// bignum to fall back to.
func TestEveryTargetDeclaresTheCarryTrap(t *testing.T) {
	for _, dir := range []string{"../targets/go", "../targets/js", "../targets/java",
		"../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		p, ok := tg.Prims["trap-if"]
		if !ok {
			t.Errorf("%s declares no `trap-if`; a fixed width that cannot fail "+
				"loudly would silently truncate", dir)
			continue
		}
		if p.Pure {
			t.Errorf("%s: `trap-if` is pure; an operation that can end the program "+
				"is not one to duplicate or elide", dir)
		}
		// JAVA FORBIDS A LAMBDA PARAMETER SHADOWING A LOCAL, and the argument is
		// usually a loop variable the emitter has called `c`. Go and JavaScript
		// both allow the shadow, so this is a one-host rule — and it fired on
		// the first program with a carry.
		if strings.Contains(p.Form, "(c ->") || strings.Contains(p.Form, "(c)") ||
			strings.Contains(p.Form, "func(c ") {
			t.Errorf("%s: `trap-if` names its parameter `c`, which shadows the "+
				"loop variable it is usually handed: %s", dir, p.Form)
		}
	}
}

func resultSig(t *testing.T, result string) *core.Sig {
	t.Helper()
	forms, err := core.Read("(sig f ((n int)) " + result + ")")
	if err != nil || len(forms) != 1 || forms[0].Sig == nil {
		t.Fatalf("read result %s: %v", result, err)
	}
	return forms[0].Sig
}

func allProgSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}

// ═══ DIVISION BY A POWER OF TWO BECOMES A SHIFT (shiftdiv-2026-09-03)
//
// The measured decomposition of the fixed-limb factorial put 2.39x on this one
// operation — more than the clamp, the element mask and the buffer clear put
// together, which are all inside the noise floor. `x / 2^k` on a SIGNED value is
// not a shift: truncation toward zero needs a rounding correction.
//
// It is licensed by a PROOF rather than a declaration, which is what makes it
// general: nothing enters the language and any program with a provably
// non-negative dividend gets it.

// AND THE WIDTH IS THE TARGET'S: TestTheShiftWidthIsTheTargets, in
// biglimb_ir_test.go, where the IR's rewrite runs.

// ═══ THE LIMB LIBRARY'S SURFACE (subdiv-2026-09-03)
//
// windows is the only target that stores a bounded big value as limbs, so what
// the library implements is exactly what arbitrary precision means there. It
// had addition and multiplication and nothing else, which is ADR 0019 item 4
// half delivered: a host that could add two bignums and not subtract them.

// ═══ A DECLARATION SURVIVES INLINING (inlining-and-declarations.md)
//
// A range has three effects: a type, a premise and a representation. Reduction
// erases every non-exported boundary, and the first two survive that — the
// checker re-derives the type, and dropping the premise is a strengthening
// because the body's obligations land on the caller's concrete values.
//
// THE THIRD IS NOT A FACT. A range above the window does not assert something
// the compiler checks; it REQUESTS arbitrary precision. So `core.LoadWith` moves
// it onto the term, where reduction preserves it.

// AND A TARGET MAY NOT DECLARE IT, for the same reason it may not declare `if`.
func TestNoTargetDeclaresTheAscription(t *testing.T) {
	for _, dir := range []string{"../targets/go", "../targets/js", "../targets/java",
		"../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		p, ok := tg.Prims[core.AscribeName]
		if !ok {
			t.Errorf("%s has no `%s`; it is injected into every target", dir, core.AscribeName)
			continue
		}
		if p.Kind != "ascribe" || p.Form != "" {
			t.Errorf("%s: `%s` is not the injected structural form: %+v",
				dir, core.AscribeName, p)
		}
	}
}
