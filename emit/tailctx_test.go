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
