package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A HOST CALL'S CONTINUATION IS A TAIL POSITION (ADR 0027): `again` may sit
// under a tuple binding, whose body is a host call's continuation after
// reduction. Every walker of a clause chain has to walk that continuation as it
// walks a one-name `let`, and a walker that enumerates back edges and misses one
// is UNSOUND, not merely imprecise. Each test below pins one walker with a
// program whose answer changes when the walker's case is removed.

// normGo reads, loads and reduces a program's first export against the Go
// target, returning the residual and its signature.
func normGo(t *testing.T, src string) (*Target, *core.Term, *core.Sig) {
	t.Helper()
	tg := goNative(t)
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
	return tg, nf, prog.Sigs[q]
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
	rep, _ := Intervals(tg, sig, nf, 0)
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
	bare, _ := Intervals(tg, sig, nf, 0)
	if bare.Proven != rep.Proven || bare.Ops != rep.Ops || bare.Terminates != rep.Terminates {
		t.Errorf("under a continuation %d/%d ops, %d terminating; bare %d/%d, %d",
			rep.Proven, rep.Ops, rep.Terminates, bare.Proven, bare.Ops, bare.Terminates)
	}
}

// THE MONOTONICITY THEOREM CHECKS THE BACK EDGE. `v` falls inside the
// continuation, so `0 <= v` does not hold and the index must be refused.
// monotoneStep passed over the continuation and certified v non-decreasing, and
// the index was accepted. The rising control must still be proven.
func TestMonotonicityChecksAnEdgeInAHostContinuation(t *testing.T) {
	const falling = `(use go/math/bits as b)
(export f)
(sig f ((t (array int))) int)
(def f (t)
  (loop ((v 0) (more true))
    (not more) (if (< v (len t)) (t v) 0)
    else (let (tuple s c) (b.Add64 5 6 0) (again (- v 1) (= c 0)))))`
	if err := refineGo(t, falling); err == nil || !strings.Contains(err.Error(), "(<= 0 v)") {
		t.Errorf("v falls, so the index must be refused on 0 <= v; got %v", err)
	}
	rising := strings.Replace(falling, "(- v 1)", "(+ v 1)", 1)
	if err := refineGo(t, rising); err != nil {
		t.Errorf("v rises from 0, so 0 <= v is the theorem's and the index is proven: %v", err)
	}
}

// THE TYPE CHECKER CHECKS `again` UNDER EVERY BINDING. Inside a continuation
// the loop was not reachable at all before ADR 0027; under a one-name `let` the
// arguments went to `walk`, where `again` is no primitive, and were never
// checked.
func TestAgainIsCheckedUnderEveryBinding(t *testing.T) {
	for _, body := range []string{
		`(let (tuple v err) (sc.ParseInt "5" 10 64) (again "x"))`,
		// y is used twice, so call-by-need keeps the `let` rather than substituting.
		`(let y (* i 3) (again (if (> y y) "x" "z")))`,
	} {
		src := `(use go/strconv as sc)
(export f)
(sig f ((n (int 0 10))) int)
(def f (n) (loop ((i 0)) (>= i n) i else ` + body + `))`
		tg, nf, _ := normGo(t, src)
		if err := Check(tg, "f", nf); err == nil || !strings.Contains(err.Error(), "again's argument") {
			t.Errorf("%s: a string handed to an int loop variable must be refused; got %v", body, err)
		}
	}
}

// A SEVERAL-RESULTS RETURN IN EACH RESULT'S VALUE TYPE. `(int 0 1)` is stored
// as a byte but computed as an `int` (a `long` on the JVM), so the signature and
// the record field that receive it must say so: `byte` there was a type error on
// both hosts, which the return from inside a host continuation first reached.
func TestASeveralResultsReturnUsesTheValueType(t *testing.T) {
	const src = `(export f)
(sig f ((a (int -1000 1000))) (tuple (int 0 1) int))
(def f (a) (tuple (if (> a 0) 1 0) a))`
	tg, nf, sig := normGo(t, src)
	goSrc, err := Func(tg, "f", sig, nf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(goSrc, ") (int, int) {") {
		t.Errorf("the Go signature must return (int, int):\n%s", goSrc)
	}
}
