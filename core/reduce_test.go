package core

import (
	"strings"
	"testing"
)

// Every case here is a worked example from the specification. If the spec and
// the implementation disagree, one of them is wrong and the test says which
// pair to look at.
//
//	core-0 §5.1  β
//	core-0 §5.2  δ
//	core-0 §5.3  the same term, two normal forms
//	core-0 §5.4  a residual λ
//	q5   §3      fusion by δ+β, the delayed vector representation
//	q5b  §3      filter by δ+β, the push representation

// testEnv builds a reduction environment from a bare list of primitive names.
// Real programs get theirs from a target file; tests do not need one.
//
// Primitives are PURE here, the opposite of the target-file default, because
// almost every test primitive is arithmetic and the alternative is writing the
// word forty times. A name prefixed with `!` is impure, which is how the
// effect tests declare one.
func testEnv(p *Program, prims ...string) *Env {
	e := &Env{
		Defs: p.Defs,
		Prim: map[string]bool{},
		Pure: map[string]bool{},
		Rec:  map[string]bool{},
	}
	for _, n := range prims {
		pure := !strings.HasPrefix(n, "!")
		n = strings.TrimPrefix(n, "!")
		e.Prim[n] = true
		e.Pure[n] = pure
	}
	e.Prim["let"] = true
	e.Pure["let"] = true
	e.MarkRecursive()
	return e
}

func norm(t *testing.T, src, target string) string {
	t.Helper()
	src, prims := splitPrims(src, target)
	forms, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("expected exactly one term to reduce, got %d", len(terms))
	}
	env := testEnv(prog, prims...)
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return out.String()
}

// splitPrims lifts (prim …) and (target …) lines out of a test program, since
// programs may no longer declare either — primitives come from a target file.
// Tests do not need a target file, only a set of names.
func splitPrims(src, target string) (string, []string) {
	var body, prims []string
	for _, line := range strings.Split(src, "\n") {
		tr := strings.TrimSpace(line)
		isPrim := strings.HasPrefix(tr, "(prim ")
		isTarget := strings.HasPrefix(tr, "(target ")
		if !isPrim && !isTarget {
			body = append(body, line)
			continue
		}
		words := strings.FieldsFunc(tr, isPunct)
		// A test may declare several targets; take only the one being reduced.
		if isTarget && len(words) > 1 && words[1] != target {
			continue
		}
		for _, w := range words {
			switch w {
			case "prim", "target", target:
			default:
				prims = append(prims, w)
			}
		}
	}
	return strings.Join(body, "\n"), prims
}

func isPunct(r rune) bool { return r == '(' || r == ')' || r == ' ' || r == '\t' }

func check(t *testing.T, src, target, want string) {
	t.Helper()
	got := norm(t, src, target)
	if got != want {
		t.Errorf("target %s\n got: %s\nwant: %s", target, got, want)
	}
}

// ---------------------------------------------------------------- core-0 §5.1

func TestBeta(t *testing.T) {
	check(t, `
		(prim add)
		((fn (x) (add x 1)) 4)
	`, "default", "(add 4 1)")
}

// ---------------------------------------------------------------- core-0 §5.2

func TestDelta(t *testing.T) {
	check(t, `
		(prim mul)
		(def double (fn (x) (mul x 2)))
		(def quad   (fn (x) (double (double x))))
		(quad 3)
	`, "default", "(mul (mul 3 2) 2)")
}

// ---------------------------------------------------------------- core-0 §5.3
//
// The thesis, executable: one word of target declaration separates a BLAS call
// from a loop.

const dotProgram = `
	(target go   (prim add mul alen aindex fold-range))
	(target blas (prim add mul alen aindex fold-range dot))

	(def vec     (fn (n f) (fn (sel) (sel n f))))
	(def vlen    (fn (v)   (v (fn (n f) n))))
	(def vindex  (fn (v i) ((v (fn (n f) f)) i)))
	(def of-array (fn (a) (vec (alen a) (fn (i) (aindex a i)))))

	(def zip (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))
	(def sum (fn (v) (fold-range 0.0 (vlen v) (fn (acc i) (add acc (vindex v i))))))
	(def dot (fn (a b) (sum (zip mul (of-array a) (of-array b)))))

	(dot p q)
`

func TestSameTermTwoNormalForms(t *testing.T) {
	// On blas, dot is primitive: reduction halts immediately, zero steps.
	check(t, dotProgram, "blas", "(dot p q)")

	// On go, dot is defined, so it reduces all the way to g1's residual.
	check(t, dotProgram, "go",
		"(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))")
}

// ---------------------------------------------------------------- core-0 §5.4

func TestResidualLambda(t *testing.T) {
	check(t, `
		(prim mul)
		(def make-scaler (fn (f) (fn (v) (mul v f))))
		(make-scaler 3)
	`, "default", "(fn (v) (mul v 3))")
}

// ---------------------------------------------------------------- q5b §3
//
// filter, via the push representation: a collection is its own fold.
//
// This test spent two days encoding the WRONG answer on purpose. β substituted
// unconditionally, so (aindex a i) appeared twice, and concerns.md §1.1 recorded
// that the spec and the implementation disagreed. Call-by-need closed it: the
// element is now bound once, exactly as q5b §3 derived on paper.

func TestFilterFusesToOneLoop(t *testing.T) {
	check(t, `
		(target go (prim add alen aindex fold-range if pos))

		(def push-of-array (fn (a)   (fn (step z) (fold-range z (alen a) (fn (acc i) (step acc (aindex a i)))))))
		(def push-filter   (fn (p c) (fn (step z) (c (fn (acc x) (if (p x) (step acc x) acc)) z))))
		(def push-sum      (fn (c)   (c (fn (acc x) (add acc x)) 0.0)))

		(push-sum (push-filter pos (push-of-array a)))
	`, "go",
		"(fold-range 0.0 (alen a) (fn (acc i) (let (aindex a i) (fn (x) (if (pos x) (add acc x) acc)))))")
}

// ---------------------------------------------------------------- termination

func TestRecursiveDefinitionIsNotUnfolded(t *testing.T) {
	// core-0 §6: δ on a recursive definition does not terminate, so it is never
	// applied. `count` must survive into the residual as a target function.
	got := norm(t, `
		(target go (prim add lt if))
		(def count (fn (n) (if (lt n 1) 0 (add 1 (count (add n -1))))))
		(count 5)
	`, "go")
	if !strings.Contains(got, "count") {
		t.Errorf("recursive definition was unfolded; got %s", got)
	}
}

func TestMutualRecursionIsNotUnfolded(t *testing.T) {
	got := norm(t, `
		(target go (prim if lt add))
		(def even? (fn (n) (if (lt n 1) 1 (odd? (add n -1)))))
		(def odd?  (fn (n) (if (lt n 1) 0 (even? (add n -1)))))
		(even? 4)
	`, "go")
	if !strings.Contains(got, "even?") && !strings.Contains(got, "odd?") {
		t.Errorf("mutual recursion was unfolded; got %s", got)
	}
}

func TestFuelStopsSelfApplication(t *testing.T) {
	forms, _ := Read(`((fn (f) (f f)) (fn (f) (f f)))`)
	prog, terms, _ := Load(forms)
	env := testEnv(prog, "x")
	if _, err := Normalize(terms[0], env, 10_000); err == nil {
		t.Fatal("expected the step limit to stop self-application")
	} else if _, ok := err.(*FuelError); !ok {
		t.Fatalf("expected FuelError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------- capture

func TestSubstitutionAvoidsCapture(t *testing.T) {
	// (fn (x) ((fn (y) (fn (x) (f y x))) x))  applied to a
	// The inner binder x must be freshened, or the outer x is captured.
	check(t, `
		(prim f a)
		((fn (x) ((fn (y) (fn (x) (f y x))) x)) a)
	`, "default", "(fn (x') (f a x'))")
}

// ---------------------------------------------------------------- normal form

func TestResidualNamesReported(t *testing.T) {
	forms, _ := Read(`
		(def use-missing (fn (x) (add x (nowhere x))))
		(use-missing 1)
	`)
	prog, terms, _ := Load(forms)
	env := testEnv(prog, "add")
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	got := Residual(out, env)
	if len(got) != 1 || got[0] != "nowhere" {
		t.Errorf("expected residual [nowhere], got %v (term %s)", got, out)
	}
}

// ---------------------------------------------------------------- reader

func TestReaderRejectsBidiControl(t *testing.T) {
	if _, err := Read("(add 1 ‮2)"); err == nil {
		t.Fatal("expected a bidirectional control to be rejected")
	}
}

func TestReaderAcceptsUTF8Identifiers(t *testing.T) {
	check(t, `
		(prim جمع)
		(def مجموع (fn (س ص) (جمع س ص)))
		(مجموع 1 2)
	`, "default", "(جمع 1 2)")
}

// UAX #31 admits letters, not mathematical symbols, so `＋` and `≤` are not
// identifiers. That is the standard's answer and it may not be the one we want
// — see docs/spec/concerns.md, open question on the identifier profile.
func TestReaderRejectsMathSymbolAsIdentifier(t *testing.T) {
	if _, err := ReadTerm("＋"); err == nil {
		t.Fatal("expected U+FF0B to be rejected as an identifier")
	}
}

// Case must never be semantically significant (core-0 §1.1), so that the
// language is writable in scripts that have no case at all.
func TestCaseIsNotSignificant(t *testing.T) {
	check(t, `
		(prim add)
		(def Double (fn (X) (add X X)))
		(def double (fn (x) (add x 1)))
		(Double 5)
	`, "default", "(add 5 5)")
}

func TestFloatsRoundTrip(t *testing.T) {
	// ADR 0009: a value must not change by being written down.
	for _, src := range []string{"1.0", "0.1", "0.30000000000000004", "1e+21", "-0.0"} {
		tm, err := ReadTerm(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		back, err := ReadTerm(tm.String())
		if err != nil {
			t.Fatalf("reread %s -> %s: %v", src, tm, err)
		}
		if back.Float != tm.Float {
			t.Errorf("%s did not round-trip: %v vs %v", src, tm.Float, back.Float)
		}
	}
}

func TestLambdaSpelledEitherWay(t *testing.T) {
	a := norm(t, "(prim add)\n((fn (x) (add x 1)) 2)", "default")
	b := norm(t, "(prim add)\n((λ (x) (add x 1)) 2)", "default")
	if a != b {
		t.Errorf("fn and λ disagree: %s vs %s", a, b)
	}
}

// ---------------------------------------------------------------- fix

// (def f (f)) denotes ⊥. δ must not unfold it, it survives as a target function
// that calls itself, and that is the correct compilation — so the residual check
// must not report it as a failure. See docs/spec/pcf.md §4.
func TestSelfCallIsBottomNotAnError(t *testing.T) {
	forms, err := Read(`
		(def f (f))
		(f)
	`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, _ := Load(forms)
	env := testEnv(prog)
	out, err := Normalize(terms[0], env, DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got := out.String(); got != "(f)" {
		t.Errorf("expected (f) to survive unreduced, got %s", got)
	}
	if left := Residual(out, env); len(left) != 0 {
		t.Errorf("a recursive definition is a legitimate survivor, not a residual failure; got %v", left)
	}
}

// ---------------------------------------------------------------- let
//
// `let` has two roles and one spelling, and the difference between them is the
// whole of the option-1/option-2 question:
//
//   in SOURCE   it is sugar for an application, so it reduces like anything else
//   in RESIDUAL it is the primitive β produced when it declined to substitute
//
// A `let` in a residual can only have come from the reducer, because the reader
// desugars every source-level one — so the two roles never collide.

// A programmer's let is erased when the compiler sees no reason to keep it.
func TestSourceLetIsErasedWhenSharingIsPointless(t *testing.T) {
	check(t, `
		(prim add)
		(let 5 (fn (x) (add x 1)))
	`, "default", "(add 5 1)")
}

// ...and is *not honoured as a knob*: a let around a value that reduces to a λ
// must still be substituted, or fusion would die. This is the case that makes
// option 1 dangerous rather than merely useless.
func TestSourceLetCannotBlockFusion(t *testing.T) {
	check(t, `
		(target go (prim add alen aindex fold-range))
		(def vec      (fn (n f) (fn (sel) (sel n f))))
		(def vlen     (fn (v)   (v (fn (n f) n))))
		(def vindex   (fn (v i) ((v (fn (n f) f)) i)))
		(def of-array (fn (a)   (vec (alen a) (fn (i) (aindex a i)))))
		; The let here is exactly what a programmer might write for clarity.
		(def sum (fn (v) (let v (fn (w)
		           (fold-range 0.0 (vlen w) (fn (acc i) (add acc (vindex w i))))))))
		(fn (a) (sum (of-array a)))
	`, "go", "(fn (a) (fold-range 0.0 (alen a) (fn (acc i) (add acc (aindex a i)))))")
}

// The compiler still introduces one where sharing genuinely pays, which is the
// same shape arrived at by decision rather than by instruction.
func TestCompilerIntroducesLetWhereItPays(t *testing.T) {
	check(t, `
		(target go (prim add aindex))
		((fn (x) (add x x)) (aindex a 0))
	`, "go", "(let (aindex a 0) (fn (x) (add x x)))")
}

// A program may no longer declare its own primitives; that is a target file's
// job. Accepting the form silently would let a program believe it had said
// something.
func TestProgramsCannotDeclarePrimitives(t *testing.T) {
	for _, src := range []string{`(prim add)`, `(target go (prim add))`} {
		forms, err := Read(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if _, _, err := Load(forms); err == nil {
			t.Errorf("%s should be rejected", src)
		}
	}
}

// ------------------------------------------------------- docs/spec/effects.md
//
// The three denials of §4, one test each, plus the two rules that make them
// enough. `!name` declares an impure primitive; everything else is pure.

// §4 — contraction. Two occurrences must not become two effects.
func TestEffectIsNotDuplicated(t *testing.T) {
	check(t, `
		(prim add !read)
		((fn (x) (add x x)) (read))
	`, "default", "(let (read) (fn (x) (add x x)))")
}

// §4 — weakening, and the hazard g5 missed. Zero occurrences must not become
// zero effects. This is the clause that makes seq work at all.
func TestEffectIsNotDeleted(t *testing.T) {
	check(t, `
		(prim !read)
		((fn (x) 1) (read))
	`, "default", "(let (read) (fn (x) 1))")

	// The pure case still deletes, which is the point of having the property.
	check(t, `
		(prim add)
		((fn (x) 1) (add 2 3))
	`, "default", "1")
}

// §4 — exchange, the hazard the reducer could not see. One occurrence, but it
// is inside what becomes a loop body: substituting would turn one effect into n.
func TestEffectIsNotMovedIntoALoop(t *testing.T) {
	check(t, `
		(prim add fold-range !read)
		((fn (x) (fold-range 0.0 10 (fn (acc i) (add acc x)))) (read))
	`, "default",
		"(let (read) (fn (x) (fold-range 0.0 10 (fn (acc i) (add acc x)))))")

	// The same shape with a pure argument still substitutes, so the discipline
	// costs nothing where it is not needed.
	check(t, `
		(prim add mul fold-range)
		((fn (x) (fold-range 0.0 10 (fn (acc i) (add acc x)))) (mul 2.0 3.0))
	`, "default",
		"(fold-range 0.0 10 (fn (acc i) (add acc (mul 2.0 3.0))))")
}

// §5 — seq is sugar, and order survives reduction.
func TestSeqPreservesOrder(t *testing.T) {
	check(t, `
		(prim !print)
		(seq (print 1) (print 2) (print 3))
	`, "default",
		"(let (print 1) (fn (_) (let (print 2) (fn (_) (print 3)))))")
}

// §3 — a λ is a value in argument position, so an effectful callback still
// substitutes into the loop the programmer wrote it for. Judging it impure
// would let-bind a bare λ, which reaches the emitter as an escaping closure.
func TestEffectfulCallbackStillSubstitutes(t *testing.T) {
	check(t, `
		(prim fold-range !print)
		((fn (f) (fold-range 0.0 10 f)) (fn (acc i) (print i)))
	`, "default", "(fold-range 0.0 10 (fn (acc i) (print i)))")
}

// §3 — but a λ argument to a PRIMITIVE is transparent, or the whole loop would
// be judged pure and could be moved into another loop.
func TestALoopContainingAnEffectIsImpure(t *testing.T) {
	check(t, `
		(prim add fold-range !print)
		((fn (x) (add x x)) (fold-range 0.0 10 (fn (acc i) (print i))))
	`, "default",
		"(let (fold-range 0.0 10 (fn (acc i) (print i))) (fn (x) (add x x)))")
}

// §4 — δ is safe because a definition's body must be a value. Without this,
// two occurrences of `x` would print twice.
func TestDefinitionBodyMustBeAValue(t *testing.T) {
	src, prims := splitPrims(`
		(prim !print)
		(def x (print 1))
		(add x x)
	`, "default")
	forms, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, _, err := Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := testEnv(prog, prims...).CheckDefs(); err == nil {
		t.Fatal("expected (def x (print 1)) to be rejected")
	}

	// Wrapping it in a λ makes it a value, and is accepted.
	src, prims = splitPrims(`
		(prim !print)
		(def x (fn (n) (print n)))
		(x 1)
	`, "default")
	forms, _ = Read(src)
	prog, _, _ = Load(forms)
	if err := testEnv(prog, prims...).CheckDefs(); err != nil {
		t.Fatalf("a λ is a value and should be accepted: %v", err)
	}
}
