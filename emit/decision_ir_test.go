package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// WHAT THE COMPILER PROVES OF A SHAPE, pinned on the IR's decision (decideOn,
// ir.Decide). These tests were written against the term interval analysis and
// moved here when it was deleted (irstep4j): each claim is about a shape the
// language promises to prove or to refuse, not about the analysis that once
// answered it.

// GAP 1 — a read from a table whose elements are RANGED carries that range.
//
// ⟦t[i]⟧ ∈ [lo,hi] because the range over-approximates every stored value and 0
// is in it by `build`'s zero-fill, so a read returns a stored value or the zero
// fill. The source used here is a DECLARATION, which is a premise rather than
// an inference — the analysis never feeds itself.
func TestReadCarriesTheElementRange(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := `(def f (fn (a)
	  (loop ((i 0) (acc 0))
	    (go.>= i (len a))  acc
	    else               (again (go.+ i 1) (go.+ acc (go.* (a i) (a i)))))))`
	for _, c := range []struct {
		name, elem string
		want       int
	}{
		{"declared 0..255", "(array (int 0 255))", 3},
		{"undeclared", "(array int)", 1},
	} {
		src := "(use go)\n(export f)\n(sig f ((a " + c.elem +
			")) int (where (go.< (len a) 1024)))\n" + body
		forms, err := core.Read(src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		prog, _, err := core.LoadWith(forms, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		env, _ := tg.Env(prog)
		nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		rep := decideOn(t, tg, prog.Sigs["f"], nf)
		if rep.Proven != c.want {
			t.Errorf("%s: %d of %d operations proven, want %d — a squared byte is "+
				"at most 65025 and the analysis should say so", c.name, rep.Proven, rep.Ops, c.want)
		}
	}
}

// The whole point, end to end: a running maximum over a guarded variable is
// bounded, where before it widened to infinity.
func TestRunningMaximumIsBounded(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(use go)
(def run (fn (n)
  (loop ((i 0) (sp 0) (mx 0))
    (go.>= sp 32)  mx
    (go.>= i 100)  mx
    else           (again (go.+ i 1)
                          (if (go.> (go.+ sp 1) 4) 0 (go.+ sp 1))
                          (if (go.> (go.+ sp 1) mx) (go.+ sp 1) mx)))))`
	forms, _ := core.Read(src)
	prog, _, err := core.LoadWith(forms, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, _ := tg.Env(prog)
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	rep := decideOn(t, tg, nil, nf)
	if rep.Ops != rep.Proven {
		t.Errorf("a running maximum over a guarded variable must be bounded: "+
			"%d of %d operations proven", rep.Proven, rep.Ops)
	}
}

// The premise half, with a control. A range must be BELIEVED, and the only way
// to know a test of that proves anything is to run the same program without it:
// `(go.* n n)` is 0 of 1 operations bounded with nothing declared and 1 of 1
// with the range, so the passing case and the failing case genuinely differ.
func TestScalarRangeIsAPremise(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")

	for _, c := range []struct {
		what   string
		sig    *core.Sig
		proven int
	}{
		{"nothing declared", sigOf(t, "int"), 0},
		{"a declared range", sigOf(t, "(int 0 1000)"), 1},
	} {
		rep := decideOn(t, tg, c.sig, body)
		if rep.Ops != 1 {
			t.Fatalf("%s: %d operations counted, want 1", c.what, rep.Ops)
		}
		if rep.Proven != c.proven {
			t.Errorf("%s: %d of %d bounded, want %d of 1",
				c.what, rep.Proven, rep.Ops, c.proven)
		}
	}
}

// THE THEOREM, STATED AS A TEST: a range and the `where` it means are the same
// declaration. `(n (int LO HI))` and `(n int) (where (and (<= LO n) (<= n HI)))`
// have the same denotation — γ(int LO HI) = {k | LO ≤ k ≤ HI} is exactly the
// satisfying set of that conjunct — so every analysis must reach the same
// answer, not merely a good enough one.
//
// Checked on a program where the declaration does work in three separate
// places: the multiply is bounded only if the range is believed, the loop
// terminates only if its bound is finite, and the buffer's element range comes
// out of the stores.
func TestScalarRangeAndWhereAgree(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (loop ((i 0) (s 0)) (go.>= i n) s "+
		"else (again (go.+ i 1) (go.+ s (go.* i 3)))))")

	ranged := sigOf(t, "(int 0 1000)")
	whered := sigOf(t, "int")
	whered.Where = mustRead(t, "(and (<= 0 n) (<= n 1000))")

	a := decideOn(t, tg, ranged, body)
	b := decideOn(t, tg, whered, body)

	if a.Ops == 0 || a.Loops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if a.Proven != b.Proven || a.Ops != b.Ops {
		t.Errorf("range says %d of %d bounded, `where` says %d of %d — a range "+
			"and the `where` it means must be the same declaration",
			a.Proven, a.Ops, b.Proven, b.Ops)
	}
	if a.Terminates != b.Terminates {
		t.Errorf("range proves %d of %d loops, `where` proves %d of %d",
			a.Terminates, a.Loops, b.Terminates, b.Loops)
	}
	// The control: without either, the same program must do WORSE. A test whose
	// two sides agree because neither learned anything proves nothing.
	c := decideOn(t, tg, sigOf(t, "int"), body)
	if c.Proven == a.Proven && c.Terminates == a.Terminates {
		t.Errorf("the undeclared program is as provable as the declared one "+
			"(%d of %d, %d of %d loops), so this test is vacuous",
			c.Proven, c.Ops, c.Terminates, c.Loops)
	}
}

// AND A DECLARED RESULT RANGE IS READ BY THE INTERVAL LAYER.
//
// The other half of the same problem, and it was measured on two independent
// ecosystems before it was built: `emit/interval.go` returned ⊤ for an
// application of any primitive it did not structurally recognise, so ADR 0019's
// bounded-by-default refused arithmetic on EVERY host result in EVERY ecosystem
// and `-checked` was the only way through (gostdlib §4b, win32 §5).
//
// A primitive has no body, so a declaration is the only source there can be —
// the same reason `ensures` belongs on a `prim` and is redundant on an internal
// definition.
func TestADeclaredResultRangeIsRead(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// Two primitives, identical but for the declared result.
	wide := emit.Prim{Name: "h.wide", Args: []string{"int"}, Result: "int", Kind: "expr", Form: "h(%s)"}
	narrow := emit.Prim{Name: "h.narrow", Args: []string{"int"}, Result: "int 0 64", Kind: "expr", Form: "h(%s)"}
	tg.Prims[wide.Name], tg.Prims[narrow.Name] = wide, narrow
	tg.Names = append(tg.Names, wide.Name, narrow.Name)

	for _, c := range []struct {
		name  string
		bound bool
	}{{"h.wide", false}, {"h.narrow", true}} {
		terms, err := core.ReadAll(`(fn (n) (go.+ (` + c.name + ` n) 1))`)
		if err != nil {
			t.Fatal(err)
		}
		rep := decideOn(t, tg, nil, terms[0])
		got := rep.Proven == rep.Ops && rep.Ops > 0
		if got != c.bound {
			t.Errorf("%s: %d of %d operations bounded; want bounded=%v.\n"+
				"  A declared result range is the only fact a primitive can offer,\n"+
				"  and without it every host call in every ecosystem is unprovable.",
				c.name, rep.Proven, rep.Ops, c.bound)
		}
	}
}

// THE INTERVAL ANALYSIS SEES THE BACK EDGE. `x` is incremented only inside
// Add64's continuation. Missing that edge, the fixpoint kept x at 0: the product
// was "proven" and a loop that diverges for a negative n was reported to
// terminate — both emitted, before collectAgain walked the continuation.
func TestABackEdgeInAHostContinuationIsSeen(t *testing.T) {
	const hidden = `(use go/math/bits as b)
(export f)
(sig f ((n int)) int)
(def f (n)
  (let r (loop ((x 0)) (= x n) x
          else (let (tuple s c) (b.Add64 5 6 0) (again (+ x 1))))
    (* r 3037000500)))`
	tg, nf, sig := normGo(t, hidden)
	rep := decideOn(t, tg, sig, nf)
	if rep.Proven == rep.Ops {
		t.Errorf("x grows without bound, so nothing about it is proven: %d of %d proven", rep.Proven, rep.Ops)
	}
	if rep.Terminates != 0 {
		t.Errorf("the loop diverges for a negative n and must not be proven to terminate")
	}
	// The same loop with the `again` bare, which every walker always saw, is the
	// answer the continuation must give.
	tg, nf, sig = normGo(t, strings.Replace(hidden,
		"(let (tuple s c) (b.Add64 5 6 0) (again (+ x 1)))", "(again (+ x 1))", 1))
	bare := decideOn(t, tg, sig, nf)
	if bare.Proven != rep.Proven || bare.Ops != rep.Ops || bare.Terminates != rep.Terminates {
		t.Errorf("under a continuation %d/%d ops, %d terminating; bare %d/%d, %d",
			rep.Proven, rep.Ops, rep.Terminates, bare.Proven, bare.Ops, bare.Terminates)
	}
}

// A LOOP VARIABLE THAT SHADOWS A PARAMETER gives the parameter back when its
// scope ends. `(loop ((h h)) …)` keeps the spelling — a parameter is not among
// the names the pass has bound — and the loop's exit DELETED h from the
// environment, taking the parameter's premise with it (u128-2026-09-23). Two
// witnesses: after the loop the parameter still bounds `(* h h)`; and a U
// parameter walked down by a loop of the same name keeps its representation,
// where the next sweep had read it as ⊤ and left `(= h 0)` to be refused by type.
func TestAShadowingLoopGivesTheParameterBack(t *testing.T) {
	tg := goNative(t)
	src := `(export f)
(sig f ((h (int 0 100))) int)
(def f (h) (+ (loop ((h h)) (= h 0) 1 else (again (/ h 10))) (* h h)))`
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
	if rep := decideOn(t, tg, prog.Sigs["f"], nf); rep.Proven != rep.Ops {
		t.Errorf("h ∈ [0, 100] after a loop that shadowed it: %d of %d proven, unproven %v",
			rep.Proven, rep.Ops, rep.Unproven)
	}
	// Through CheckSignatures, where it was met: the claim is checked on the
	// definition after the representation is chosen, and that pass sweeps the
	// loop twice.
	forms, err = core.Read(`(export f)
(sig f ((h (int 0 18446744073709551615))) int)
(def f (h) (loop ((h h)) (= h 0) 1 else (again (/ h 10))))`)
	if err != nil {
		t.Fatal(err)
	}
	if prog, _, err = core.Load(forms); err != nil {
		t.Fatal(err)
	}
	if env, err = tg.Env(prog); err != nil {
		t.Fatal(err)
	}
	if err := emit.CheckSignatures(tg, prog, env, nil); err != nil {
		t.Errorf("a U parameter shadowed by its loop must stay in U: %v", err)
	}
}

// THE COMPONENT LAW, WITNESSED SOUND (ADR 0031 §3). The two components of one
// tuple have different ranges, and each name must get its OWN: `a` ≤ 11, so
// a·10¹⁵ is inside int64; `b` ≤ 1.1·10¹³, so b·10⁶ is not. Exactly that one
// operation is unproven (the `%` keeps it out of the sum). Binding every name to
// the first component's facts proves the overflow; binding none loses a·10¹⁵.
//
// The back edge is a `let` whose body jumps, which is what a clause chain looks
// like after reduction, and the join over the exits must see that it yields no
// value (prodfacts.go, noValue) or it loses the exit it does have.
func TestEachComponentHasItsOwnFacts(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (let (tuple a b) (loop ((i 0) (s 0))
                     (>= i n)  (tuple i (* i 1000000000000))
                     else      (let j (if (< i 5) (+ i 1) (+ i 2))
                                 (again j j)))
    (+ (* a 1000000000000000) (% (* b 1000000) 7))))`
	rep := reportGo(t, src)
	if rep.Proven != 5 || rep.Ops != 6 || len(rep.Unproven) != 1 ||
		!strings.Contains(rep.Unproven[0], "(* b 1000000)") {
		t.Errorf("%d of %d proven, unproven %v; want 5 of 6, only b·10⁶", rep.Proven, rep.Ops, rep.Unproven)
	}
}

// rungOn takes a program's first export to its rung above the word, on the
// IR, without the checks on terms: the refusal a test asks about is the rung's.
func rungOn(t *testing.T, tg *emit.Target, src string) (*ir.Func, error) {
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
	sig := prog.Sigs[q]
	plan, err := emit.PlanBig(tg, sig, nf, allProgSigs(prog)...)
	if err != nil {
		return nil, err
	}
	f, err := ir.Lower(tg, "t", sig, emit.EraseWordAscriptions(tg.Word, nf), ir.Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	ir.Decide(tg, f, false)
	if _, err := ir.SelectRung(tg, f, sig, plan); err != nil {
		return nil, err
	}
	return f, nil
}

func printed(f *ir.Func) string {
	return ir.Print(&ir.Program{Target: "go", Stage: ir.StageA, Funcs: []*ir.Func{f}})
}

const signedProgram = `(export f)
(sig f ((n (int 0 10))) (int (- 0 (pow 2 200)) (pow 2 200)))
(def f (n) (- (- 0 1606938044258990275541962092341162602522202993782792835301376) n))`

const naturalProgram = `(export f)
(sig f ((n (int 0 10))) (int 0 (pow 2 200)))
(def f (n) (+ 1606938044258990275541962092341162602522202993782792835301376 n))`

// THE ENFORCED SET HAS A SIGN (ADR 0029): the host's bignum checks a signed
// program with big-fit-signed and a non-negative one with big-fit, and the
// limb rung, which holds a magnitude, refuses a signed one by name.
func TestTheLimbsHoldOnlyANonNegativeSet(t *testing.T) {
	f, err := rungOn(t, goNative(t), signedProgram)
	if err != nil || !strings.Contains(printed(f), "big-fit-signed") {
		t.Errorf("a signed program is enforced by big-fit-signed: %v", err)
	}
	f, err = rungOn(t, goNative(t), naturalProgram)
	if err != nil || !strings.Contains(printed(f), "(call big-fit ") {
		t.Errorf("a non-negative program is enforced by big-fit: %v", err)
	}
	if _, err := rungOn(t, winTarget(t), signedProgram); err == nil || !strings.Contains(err.Error(), "NEGATIVE") {
		t.Errorf("windows has only limbs, so a signed range above the word is refused; got %v", err)
	}
	if _, err := rungOn(t, winTarget(t), naturalProgram); err != nil {
		t.Errorf("windows takes a non-negative range above the word on limbs: %v", err)
	}
}

// AN OPERATION THE LIMB LIBRARY LACKS IS REFUSED BY NAME on a target with no
// bignum to fall back to, and the refusal says what the library does have.
func TestAnUnsupportedLimbOperationIsRefusedByName(t *testing.T) {
	tg := winTarget(t)
	if tg.HasBig() {
		t.Skip("windows now declares a bignum; this test has done its job")
	}
	for _, c := range []struct{ body, want string }{
		{"(% a b)", "the remainder by another arbitrary-precision value"},
		{"(/ a b)", "division by another arbitrary-precision value"},
	} {
		src := "(export f)\n" +
			"(sig f ((a (int 0 (pow 2 200))) (b (int 1 (pow 2 200)))) (int 0 (pow 2 200)))\n" +
			"(def f (fn (a b) " + c.body + "))\n"
		_, err := rungOn(t, tg, src)
		if err == nil {
			t.Errorf("%s was accepted on windows, which cannot do it", c.body)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: refusal does not name the operation (want %q):\n%s", c.body, c.want, err)
		}
		if !strings.Contains(err.Error(), "remainder by a machine") {
			t.Errorf("%s: refusal does not say what the library does have:\n%s", c.body, err)
		}
	}
}

// WITHOUT THE DECLARATION THE MAGNITUDE IS NOT DERIVABLE: the same accumulator
// that a declared range above the word holds exactly is, undeclared, a word
// multiply no fact bounds, and the decision says so.
func TestWithoutTheDeclarationTheMagnitudeIsNotDerivable(t *testing.T) {
	tg, nf, sig := normGo(t, `(export run)
(sig run ((n (int 0 50))) int)
(def run (fn (n) (% (loop ((acc (+ n 7)) (i 0)) (>= i 6) acc else (again (* acc 999983) (+ i 1))) 100000000)))
`)
	rep := decideOn(t, tg, sig, nf)
	if rep.Proven == rep.Ops {
		t.Fatal("the multiply was proven in the word without any declaration; if the " +
			"analysis can now derive it, this test has done its job")
	}
	if !strings.Contains(strings.Join(rep.Unproven, "\n"), "[-inf, +inf]") {
		t.Errorf("expected the multiply to be unbounded, got %v", rep.Unproven)
	}
}

// A BIGNUM MAY NOT SILENTLY BECOME AN `int`, which is unbounded-rung.md §3: the
// promotion is a WIDENING and not a refinement, so the narrowing direction is
// refused and that refusal is the surface where a programmer finds out a value
// left the machine word.
//
// `int ⊆ big` is the decidable half of subtyping type-algebra.md already keeps,
// run in the other direction.
func TestABignumIsRefusedWhereAWordIsRequired(t *testing.T) {
	src := `(use go)
(export narrow)
(sig narrow ((n (int 0 +inf))) int)
(def narrow (fn (n) (+ n 1)))
`
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, _ := core.Read(src)
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	err = emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, allProgSigs(prog)))
	if err == nil {
		t.Fatal("a value above the portable window was accepted as an `int`")
	}
	if !strings.Contains(err.Error(), "WIDENING") {
		t.Errorf("the refusal does not say what kind of mismatch this is: %v", err)
	}
}
