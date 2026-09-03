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
func TestAFiniteRangeGivesABoundAndInfinityDoesNot(t *testing.T) {
	for _, c := range []struct {
		result string
		want   int
	}{
		{"(int 0 (pow 2 300))", 301},
		{"(int 0 (pow 2 1300))", 1301},
		{"(int 0 +inf)", 0},
		{"(int -inf +inf)", 0},
		{"int", 0},
	} {
		bits, on := BigBound(resultSig(t, c.result))
		if c.want == 0 {
			if on {
				t.Errorf("%s gave a bound of %d bits; ℤ is not an interval",
					c.result, bits)
			}
			continue
		}
		if !on || bits != c.want {
			t.Errorf("%s gave %d bits (on=%v), want %d", c.result, bits, on, c.want)
		}
	}
}

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
		{"../targets/go", false},   // math/big: 2,186 ns against 18,150 in limbs
		{"../targets/js", false},   // BigInt: 5,290 against 528,334 — a factor of 100
		{"../targets/java", false}, // BigInteger: 2,905 against 7,948
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

// AND THE BOUND IS THE MAXIMUM OVER THE WHOLE PROGRAM, which is not laziness.
//
// Reduction inlines every non-exported call, so by the time the representation
// is selected a helper's declared range is gone — refinements.md §6b's limit
// for a fourth time — and `main` has no signature at all. A per-function width
// would mean no whole program could ever take this rung, which is exactly what
// happened: the first version compiled `main` with the host's bignum and the
// trap never fired.
func TestTheBoundComesFromTheWholeProgram(t *testing.T) {
	small := resultSig(t, "(int 0 (pow 2 100))")
	wide := resultSig(t, "(int 0 (pow 2 1300))")
	bits, on := BigBound(nil, small, wide)
	if !on || bits != 1301 {
		t.Errorf("%d bits (on=%v) over two signatures, want the maximum, 1301", bits, on)
	}
	// One unbounded declaration anywhere takes the program to the host's bignum,
	// because a program holds ONE representation and ℤ cannot be one of the
	// fixed ones.
	if _, on := BigBound(small, resultSig(t, "(int 0 +inf)")); on {
		t.Error("an unbounded declaration did not veto the bound")
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
	// GO'S OWN ANSWER IS `math/big` (targets/go/bigint.oro), so the rung under
	// test is asked for rather than assumed. That is the separation working:
	// the program is the same either way.
	tg.BigRepr = "limbs"
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
	tg.BigRepr = "limbs"
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

// LOOP-CARRIED BUFFER REUSE (LoopBufferReuse), the back-edge instance of the
// gap named in four places.
//
// A `build` on a back edge writes into storage the loop already owns. The
// conditions are rule R's one level up, and the two that carry the weight are
// (3) — every occurrence of the loop variable is inside that argument — and the
// requirement that both lengths be the same CONSTANT, since two buffers of
// different sizes are not interchangeable storage.
func TestABackEdgeBuildReusesTheLoopsStorage(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// THE LIMB RUNG IS ASKED FOR: Go's own answer for a bounded big value is
	// `math/big` (targets/go/bigint.oro), where the reuse this test is about is
	// `bigreuse.go`'s and not this one's. Same program, different storage.
	tg.BigRepr = "limbs"
	src := `(export fact)
(sig fact ((n (int 0 50))) (int 0 (pow 2 300)))
(def fact (fn (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1)))))
`
	code := emitGo(t, tg, src, "fact")
	// One `make` for the initial buffer and one for the spare, both OUTSIDE the
	// loop; the allocating form has one inside it.
	if n := strings.Count(code, "make("); n != 2 {
		t.Errorf("%d allocations, want 2 — the spare is hoisted and the back edge "+
			"swaps into it:\n%s", n, code)
	}
	if !strings.Contains(code, "clear(") {
		t.Errorf("the spare is not cleared; `build` zero-fills (tables.md §14.3) "+
			"and a program may rely on it:\n%s", code)
	}
	// AND THE SWAP, not a plain assignment: `acc = o` alone would make the
	// accumulator and the spare the same buffer, and `clear` would wipe it
	// before every read. That is what the first version did, and it returned 0.
	if !strings.Contains(code, ", sp") || !strings.Contains(code, "= o") {
		t.Errorf("no swap at the back edge:\n%s", code)
	}
}

// AND A LOOP WHOSE BUFFER IS READ SOMEWHERE ELSE KEEPS ITS ALLOCATION, which is
// condition (3): the old buffer has to be dead after the back edge, and another
// reader is the proof that it is not.
func TestABufferReadElsewhereIsNotReused(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int 0 20))) int)
(def at (fn (t i) (if (< i 0) 0 (if (>= i (len t)) 0 (t i)))))
(def f (fn (n)
  (loop ((b (build 8 (fn (x) x))) (s 0) (i 0))
    (>= i n)  s
    else      (again (build 8 (fn (o) (set o 0 (at b 0)))) (+ s (at b 1)) (+ i 1)))))
`
	code := emitGo(t, tg, src, "f")
	if strings.Count(code, "make(") != 2 || strings.Contains(code, "clear(") {
		t.Errorf("a buffer read outside its own back-edge argument was reused; "+
			"its old contents are still needed:\n%s", code)
	}
}

func emitGo(t *testing.T, tg *Target, src, name string) string {
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
	nf, _, err = PromoteBig(tg, prog.Sigs[name], nf, allProgSigs(prog)...)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Func(tg, name, prog.Sigs[name], nf)
	if err != nil {
		t.Fatal(err)
	}
	return code
}
