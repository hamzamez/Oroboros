package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE FIXED-LIMB RUNG (emit/biglimb.go), ADR 0019's ladder, third step.
//
// Until this, a declared endpoint was compared against the portable window and
// thrown away: `(int 0 (pow 2 1000))` and `(int 0 +inf)` produced identical
// code. Having the spelling was necessary and nowhere near sufficient.

// THE DECLARATION CHOOSES THE REPRESENTATION, and the two upper rungs are a
// genuine choice rather than an implementation detail.
//
// A FINITE bound is a limb count — a `build` of known length, no allocation per
// operation, and a trap if the value exceeds what was declared. ℤ has no limb
// count, so `+inf` falls back to whatever arbitrary precision the target ships.
func TestAFiniteRangeSelectsLimbsAndInfinityDoesNot(t *testing.T) {
	for _, c := range []struct {
		result string
		want   int
	}{
		{"(int 0 (pow 2 300))", 13},  // 301 bits over 24
		{"(int 0 (pow 2 1300))", 55}, // 1301 bits over 24
		{"(int 0 +inf)", 0},
		{"(int -inf +inf)", 0},
		{"int", 0},
	} {
		sig := resultSig(t, c.result)
		w, on := LimbWidth(sig)
		if c.want == 0 {
			if on {
				t.Errorf("%s selected the limb rung at width %d; ℤ has no limb count",
					c.result, w)
			}
			continue
		}
		if !on || w != c.want {
			t.Errorf("%s gave width %d (on=%v), want %d", c.result, w, on, c.want)
		}
	}
}

// AND THE WIDTH IS THE MAXIMUM OVER THE WHOLE PROGRAM, which is not laziness.
//
// Reduction inlines every non-exported call, so by the time the representation
// is selected a helper's declared range is gone — refinements.md §6b's limit
// for a fourth time — and `main` has no signature at all. A per-function width
// would mean no whole program could ever take this rung, which is exactly what
// happened: the first version compiled `main` with the host's bignum and the
// trap never fired.
func TestTheWidthComesFromTheWholeProgram(t *testing.T) {
	small := resultSig(t, "(int 0 (pow 2 100))")
	big := resultSig(t, "(int 0 (pow 2 1300))")
	w, on := LimbWidth(nil, small, big)
	if !on || w != 55 {
		t.Errorf("width %d (on=%v) over two signatures, want the maximum, 55", w, on)
	}
	// One unbounded declaration anywhere takes the program to the host's bignum,
	// because a program holds ONE representation and ℤ cannot be one of the
	// fixed ones.
	if _, on := LimbWidth(small, resultSig(t, "(int 0 +inf)")); on {
		t.Error("an unbounded declaration did not veto the fixed-limb rung")
	}
}

// A LIMB VALUE IS A TABLE, and the signature has to say so or the checker
// refuses a body that produces one — true of the declaration and false of the
// code.
func TestALimbSignatureIsATableOfLimbs(t *testing.T) {
	sig := resultSig(t, "(int 0 (pow 2 300))")
	got := LimbSig(sig, true)
	if got.Result != "array int" {
		t.Errorf("a limb result types as %q, want array int", got.Result)
	}
	// And off, it is untouched: the same signature means the host's bignum on
	// the other rung.
	if LimbSig(sig, false).Result == "array int" {
		t.Error("LimbSig rewrote a signature the limb rung was not selected for")
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

// THE LIMB RUNG NEEDS NO HOST BIGNUM, which is the whole reason it exists on
// windows — the one target that ships nothing to fall back to, and the one ADR
// 0019 item 4 names.
func TestWindowsTakesTheLimbRung(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	if tg.HasBig() {
		t.Skip("windows now declares a bignum; this test has done its job")
	}
	src := `(export fact)
(sig fact ((n (int 0 50))) (int 0 (pow 2 300)))
(def fact (fn (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1)))))
`
	out := lowerOn(t, tg, src, "fact")
	if !strings.Contains(out, "build 13") {
		t.Errorf("windows did not get a 13-limb buffer:\n%s", out)
	}
	if !strings.Contains(out, "trap-if") {
		t.Errorf("windows got limbs with no carry check, so an under-declared "+
			"bound would truncate silently:\n%s", out)
	}
}

// A MULTIPLY BY A WIDENED WORD IS A DIFFERENT ALGORITHM, worth a factor of w:
// `mul-small` is one pass where `mul` is w passes, and a factorial is nothing
// but this.
//
// It has to be recognised BEFORE the operand is spliced — once `(big-of i)` is
// a `build`, nothing distinguishes it from any other limb table — and getting
// that wrong is a correct program that does 55 times the work.
func TestAMultiplyByAWordIsOnePass(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export fact)
(sig fact ((n (int 0 50))) (int 0 (pow 2 300)))
(def fact (fn (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1)))))
`
	out := lowerOn(t, tg, src, "fact")
	// THREE loops: the factorial's own, the one that spreads `1` over the limbs,
	// and ONE pass for the multiply. `mul` nests a second pass inside its own,
	// so counting is what tells the two algorithms apart -- and getting this
	// wrong is a correct program that does w times the work.
	if n := strings.Count(out, "(loop "); n != 3 {
		t.Errorf("the factorial has %d loops, want 3 — a word multiply is ONE "+
			"pass and `mul` would nest a second:\n%s", n, out)
	}
}

// AND A PROGRAM WITH NO DECLARATION ABOVE THE WINDOW IS UNTOUCHED, which is the
// containment property for this pass.
func TestNoBigRangeMeansNoLimbs(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (loop ((acc 0) (i 0)) (>= i n) acc else (again (+ acc i) (+ i 1)))))
`
	if out := lowerOn(t, tg, src, "f"); strings.Contains(out, "16777216") {
		t.Errorf("a program with no big range was given limbs:\n%s", out)
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

func lowerOn(t *testing.T, tg *Target, src, name string) string {
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
	nf, err := core.Normalize(prog.Defs[name], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := PromoteBig(tg, prog.Sigs[name], nf, allProgSigs(prog)...)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func allProgSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}
