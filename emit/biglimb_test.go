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
	// THE SAME ORDER THE DRIVERS USE (cmd/build, cmd/gen): the shift rewrite
	// runs last, after the fixed-limb library has been spliced in, because the
	// library's own carry splits are what it is most for. A helper that stopped
	// before it would be testing a pipeline nothing runs.
	out, _ = SelectShifts(tg, prog.Sigs[name], out)
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
	nf, _ = SelectShifts(tg, prog.Sigs[name], nf)
	code, err := Func(tg, name, prog.Sigs[name], nf)
	if err != nil {
		t.Fatal(err)
	}
	return code
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

func TestADividendProvedNonNegativeBecomesAShift(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (+ (/ n 8) (% n 8))))
`
	code := emitGo(t, tg, src, "f")
	if !strings.Contains(code, ">> 3") || !strings.Contains(code, "& 7") {
		t.Errorf("a non-negative dividend did not become a shift and a mask:\n%s", code)
	}
}

// AND A DIVIDEND THAT MAY BE NEGATIVE DOES NOT, which is the rule this could
// most easily get wrong — and the differential suite could not catch it, because
// Go, the JVM and x86 would all shift and all be wrong together.
//
// An arithmetic shift is FLOOR division; our `/` truncates toward zero. They
// differ by one on every negative dividend: `-1 / 2` is 0 and `-1 >> 1` is −1.
func TestAPossiblyNegativeDividendStaysADivision(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int -1000 1000))) int)
(def f (fn (n) (/ n 8)))
`
	code := emitGo(t, tg, src, "f")
	if strings.Contains(code, ">>") {
		t.Errorf("a dividend that can be negative was rewritten to a shift, which "+
			"is floor division where ours truncates:\n%s", code)
	}
	if !strings.Contains(code, "/ 8") {
		t.Errorf("the division disappeared without becoming a shift:\n%s", code)
	}
}

// AND THE WIDTH IS THE TARGET'S. V8 coerces both operands of `>>` and `&` to
// int32, so `targets/js` declares 31 — and a value that provably fits gets the
// rewrite there while one that does not keeps its division. Declaring the width
// rather than excluding the host is what buys the first half.
func TestTheShiftWidthIsTheTargets(t *testing.T) {
	for _, c := range []struct {
		dir   string
		param string
		want  bool
	}{
		{"../targets/js", "(int 0 1000)", true},
		{"../targets/js", "(int 0 4000000000)", false}, // past 2^31
		{"../targets/go", "(int 0 4000000000)", true},  // Go shifts 64-bit values
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		src := "(export f)\n(sig f ((n " + c.param + ")) int)\n(def f (fn (n) (/ n 8)))\n"
		nf := lowerOn(t, tg, src, "f")
		got := strings.Contains(nf, ">>")
		if got != c.want {
			t.Errorf("%s with %s: rewritten=%v, want %v\n%s", c.dir, c.param, got, c.want, nf)
		}
	}
}

// AND A TARGET THAT DECLARES NOTHING GETS NOTHING, which is the containment
// property for this pass and the reason a third-party target is safe by default.
func TestNoShiftWidthMeansNoRewrite(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.ShiftWidth = 0
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (/ n 8)))
`
	if code := emitGo(t, tg, src, "f"); strings.Contains(code, ">>") {
		t.Errorf("a target declaring no shift width was rewritten anyway:\n%s", code)
	}
}

// ═══ THE LIMB LIBRARY'S SURFACE (subdiv-2026-09-03)
//
// windows is the only target that stores a bounded big value as limbs, so what
// the library implements is exactly what arbitrary precision means there. It
// had addition and multiplication and nothing else, which is ADR 0019 item 4
// half delivered: a host that could add two bignums and not subtract them.

func TestWindowsCanSubtractCompareAndDivideByAWord(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	if tg.HasBig() {
		t.Skip("windows now declares a bignum; this test has done its job")
	}
	for _, c := range []struct {
		body, result string
		build        bool
	}{
		// An arithmetic result is a new limb table. A comparison yields a bool
		// and `(% a k)` yields a machine WORD — `a % k` is under k — so neither
		// allocates one, and saying which is which is the point of `big%-small`.
		{"(- a b)", "(int 0 (pow 2 200))", true},
		{"(/ a 4)", "(int 0 (pow 2 200))", true},
		{"(% a 4)", "int", false},
		{"(if (< a b) a b)", "(int 0 (pow 2 200))", false},
	} {
		src := "(export f)\n" +
			"(sig f ((a (int 0 (pow 2 200))) (b (int 0 (pow 2 200)))) " + c.result + ")\n" +
			"(def f (fn (a b) " + c.body + "))\n"
		out := lowerOn(t, tg, src, "f")
		for _, leftover := range []string{"big-", "big/", "big%", "big<"} {
			if strings.Contains(out, leftover) {
				t.Errorf("%s kept %s on windows, which declares no bignum:\n%s",
					c.body, leftover, out)
			}
		}
		if got := strings.Contains(out, "build"); got != c.build {
			t.Errorf("%s: allocates a limb table = %v, want %v:\n%s",
				c.body, got, c.build, out)
		}
	}
}

// AND WHAT IS STILL MISSING IS REFUSED BY NAME, which is the whole of what a
// programmer is told on a target with no bignum to fall back to.
//
// What is missing is now exactly the two operations that need a QUOTIENT
// ESTIMATE — division and remainder by another arbitrary-precision value.
// Dividing by a machine word is one pass and is implemented, and so is the
// remainder, whose result is a word.
//
// The message used to be a fixed list — "subtraction, division or a comparison"
// — which was wrong about subtraction the moment subtraction existed and named
// none of the three for the program in hand. Worse, it was UNREACHABLE: the
// signature checker dropped `PromoteBig`'s error and then type-checked the
// un-promoted body against the limb signature, so `(- a b)` came back as
// "a is array int, but int is required here" — a type error naming an internal
// representation.
func TestAnUnsupportedLimbOperationIsRefusedByName(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	if tg.HasBig() {
		t.Skip("windows now declares a bignum; this test has done its job")
	}
	for _, c := range []struct{ body, want string }{
		{"(% a b)", "the remainder by another arbitrary-precision value"},
		{"(/ a b)", "division by another arbitrary-precision value"},
	} {
		src := "(export f)\n" +
			"(sig f ((a (int 0 (pow 2 200))) (b (int 0 (pow 2 200)))) (int 0 (pow 2 200)))\n" +
			"(def f (fn (a b) " + c.body + "))\n"
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
		nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = PromoteBig(tg, prog.Sigs["f"], nf, allProgSigs(prog)...)
		if err == nil {
			t.Errorf("%s was accepted on windows, which cannot do it", c.body)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: refusal does not name the operation (want %q):\n%s",
				c.body, c.want, err)
		}
		// AND IT SAYS WHAT IS AVAILABLE, so the answer to "then what can I do"
		// is in the same message.
		if !strings.Contains(err.Error(), "division by a machine word") {
			t.Errorf("%s: refusal does not say what the library does have:\n%s", c.body, err)
		}
	}
}

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

func TestADeclaredWideningSurvivesInlining(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// `scaled` is not exported and `run` returns an ordinary `int`, so after
	// inlining nothing in the residual mentions a big type — except the
	// ascription.
	src := `(export run)
(sig scaled ((n (int 0 50))) (int 0 (pow 2 200)))
(def scaled (fn (n) (loop ((acc (+ n 7)) (i 0)) (>= i 6) acc else (again (* acc 999983) (+ i 1)))))
(sig run ((n (int 0 50))) int)
(def run (fn (n) (% (scaled n) 100000000)))
`
	code := emitGo(t, tg, src, "run")
	if !strings.Contains(code, "big.NewInt") || !strings.Contains(code, "Mul") {
		t.Errorf("the inlined accumulator was not promoted:\n%s", code)
	}
	// AND THE MARKER IS GONE. It carries a range and nothing else; the runtime
	// enforcement of the bound is `big-fit`, which is a different mechanism.
	if strings.Contains(code, core.AscribeName+"(") || strings.Contains(code, "int 0 16069") {
		t.Errorf("the ascription reached the backend:\n%s", code)
	}
}

// AND THE ANALYSIS CANNOT SUPPLY WHAT THE DECLARATION DOES, which is why this
// is intent rather than a precision gap. The same loop with the declaration
// removed is refused, and the interval reported is ⊤ — not a large finite bound
// the compiler could have used, even though the trip count is a constant 6.
func TestWithoutTheDeclarationTheMagnitudeIsNotDerivable(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export run)
(sig run ((n (int 0 50))) int)
(def run (fn (n) (% (loop ((acc (+ n 7)) (i 0)) (>= i 6) acc else (again (* acc 999983) (+ i 1))) 100000000)))
`
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
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	nf, _, err = PromoteBig(tg, prog.Sigs["run"], nf, allProgSigs(prog)...)
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := Intervals(tg, prog.Sigs["run"], nf, 0)
	if err := Unbounded("run", rep); err == nil {
		t.Fatal("the multiply was proven in-window without any declaration; " +
			"if the analysis can now derive it, this test has done its job and " +
			"the ascription may be unnecessary for this shape")
	} else if !strings.Contains(err.Error(), "[-inf, +inf]") {
		t.Errorf("expected the multiply to be unbounded, got:\n%s", err)
	}
}

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
