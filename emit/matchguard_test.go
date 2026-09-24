package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A GUARD THAT IS A CONNECTIVE NARROWS, AND A TRIP COUNT HOLDS ON EVERY EDGE
// (matchguard-2026-09-24). Each test pins one rule by an operation whose proof
// depends on it, or a loop whose termination does.

func reportGo(t *testing.T, src string) *IntervalReport {
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
	rep, _ := Intervals(tg, prog.Sigs[q], nf, 0)
	return rep
}

// `a ∧ b` HOLDING narrows by both: v ≥ 10 bounds v − 10 from below, and only
// the second conjunct says so. v's declared floor is the word's, so without the
// narrowing v − 10 passes below it.
func TestAConjunctionNarrowsByBothOperands(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((tag int) (v (int -9223372036854775808 1000))) int)
(def f (tag v) (if (and (= tag 0) (>= v 10)) (- v 10) 0))`)
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: under v ≥ 10, v − 10 is in the word", rep.Proven, rep.Ops)
	}
}

// `a ∨ b` FAILING narrows by both negations: 0 ≤ v ≤ 10.
func TestAFailedDisjunctionNarrowsByBothNegations(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((v int)) int)
(def f (v) (if (or (< v 0) (> v 10)) 0 (+ v 9223372036854775797)))`)
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: v ∈ [0, 10] when neither disjunct holds", rep.Proven, rep.Ops)
	}
}

// `a ∧ b` FAILING is ¬a ∨ (a ∧ ¬b), a JOIN: v ∈ [0, 4] ∪ [11, 20], whose hull
// is [0, 20]. Reading it as ¬a ∧ ¬b would make it empty, and prove anything.
func TestAFailedConjunctionIsTheJoinOfTwoPaths(t *testing.T) {
	const src = `(export f)
(sig f ((v (int 0 20))) int)
(def f (v) (if (and (>= v 5) (<= v 10)) 0 (+ v K)))`
	if rep := reportGo(t, strings.Replace(src, "K", "9223372036854775787", 1)); rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: v ≤ 20 on both paths, and 20 + (2^63 − 21) fits", rep.Proven, rep.Ops)
	}
	if rep := reportGo(t, strings.Replace(src, "K", "9223372036854775788", 1)); rep.Proven == rep.Ops {
		t.Errorf("%d of %d proven: v may be 20, and 20 + (2^63 − 20) is past the word", rep.Proven, rep.Ops)
	}
}

// A TRIP COUNT DIVIDES BY THE LEAST STEP OF EVERY EDGE. This loop steps by 10
// above 50 and by 1 below: 46 trips from 100. Numbered by the last edge's 10,
// it was 11, and `c + (2^63 − 20)` was proven while it overflowed at run time.
func TestATripCountHoldsOnEveryEdge(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (+ (loop ((k n) (c 0))
       (<= k 0)  c
       (< k 50)  (again (- k 1) (+ c 1))
       else      (again (- k 10) (+ c 1)))
     9223372036854775787))`)
	if rep.Proven == rep.Ops {
		t.Errorf("%d of %d proven: c reaches 46, and 46 + (2^63 − 21) is past the word", rep.Proven, rep.Ops)
	}
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating: k descends on both edges", rep.Terminates, rep.Loops)
	}
}

// SEPARATED INTERVALS DESCEND. The reset `(again 0 …)` runs only where v ∈
// [1, 9], and 0 < 1: v descends there as on the edge that subtracts 10, so the
// loop terminates and its counter is bounded by the trips.
func TestAResetBelowTheGuardDescends(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (+ (loop ((v n) (c 0))
       (= v 0)   c
       (>= v 10) (again (- v 10) (+ c 1))
       else      (again 0 (+ c 1)))
     1000))`)
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating: v ∈ [1, 9] when it is reset to 0", rep.Terminates, rep.Loops)
	}
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: the counter is bounded by the trips", rep.Proven, rep.Ops)
	}
}

// A DEAD BACK EDGE IS NO EDGE. From 0 the loop exits at once; its back edge,
// which doubles v and so shows no descent, is never taken. It neither widens
// the counter nor blocks termination.
func TestAnUnreachableBackEdgeIsNoEdge(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (+ (loop ((v 0) (c 0))
       (= v 0) c
       else    (again (* v 2) (+ c 1)))
     9223372036854775807))`)
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating: the only back edge is dead", rep.Terminates, rep.Loops)
	}
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: c is 0, and 0 + (2^63 − 1) fits", rep.Proven, rep.Ops)
	}
}

// THE JOIN KEEPS ONLY WHAT BOTH PATHS KNOW. `len t` is narrowed on the second
// path alone (x ≥ 5 and len t < 10); on the first (x < 5) nothing is known of
// it, so after the join it is anything — and len t + (2^63 − 10) may overflow.
func TestAJoinForgetsWhatOnePathAloneKnows(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((x (int 0 20)) (t (array int))) int)
(def f (x t) (if (and (>= x 5) (>= (len t) 10)) 0 (+ (len t) 9223372036854775798)))`)
	if rep.Proven == rep.Ops {
		t.Errorf("%d of %d proven: when x < 5, len t may be anything", rep.Proven, rep.Ops)
	}
}

// A SEPARATED EDGE DESCENDS BY ITS GAP, NOT BY MORE. Every edge here jumps to
// just below its band — 100 → 59 → 39 → 19 → 0 — a gap of 1 each time, so the
// counter is bounded by 101 trips and no fewer can be claimed: c + (2^63 − 2)
// may overflow, since c reaches 4.
func TestASeparatedEdgeDescendsByItsGap(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (+ (loop ((v n) (c 0))
       (<= v 0)  c
       (>= v 60) (again 59 (+ c 1))
       (>= v 40) (again 39 (+ c 1))
       (>= v 20) (again 19 (+ c 1))
       else      (again 0 (+ c 1)))
     9223372036854775806))`)
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating: every edge jumps below its band", rep.Terminates, rep.Loops)
	}
	if rep.Proven == rep.Ops {
		t.Errorf("%d of %d proven: c reaches 4, and 4 + (2^63 − 2) is past the word", rep.Proven, rep.Ops)
	}
}

// AND A DEAD EDGE THAT NOTHING ELSE RESCUES: it doubles v and leaves c alone,
// so no variable descends on it. Recorded, it is a cycle with no descent.
func TestAnUnreachableBackEdgeWithNoDescentIsNoEdge(t *testing.T) {
	rep := reportGo(t, `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (loop ((v 0) (c 0))
    (= v 0) c
    else    (again (* v 2) c)))`)
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating: the only back edge is dead", rep.Terminates, rep.Loops)
	}
}

// A DERIVED STEP DESCENDS BY ITS OWN LEAST STEP. The index advances through a
// choice of two inlined scanners, each returning at least i + 1: a step of 1,
// no more, so up to n + 1 trips and c + (2^63 − 2) may overflow. (A step of 1
// is what the other clauses' step used to lend it, when the measure was said to
// be withheld and was not.)
func TestADerivedStepDescendsByItsLeastStep(t *testing.T) {
	const src = `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (+ (loop ((i 0) (c 0))
       (>= i n) c
       else     (again (if (= (% i 2) 0)
                           (loop ((j (+ i 1))) (>= j n) j (= (% j 7) 0) j else (again (+ j 1)))
                           (loop ((j (+ i 1))) (>= j n) j (= (% j 5) 0) j else (again (+ j 1))))
                       (+ c 1)))
     K))`
	rep := reportGo(t, strings.Replace(src, "K", "9223372036854775806", 1))
	if rep.Proven == rep.Ops {
		t.Errorf("%d of %d proven: c reaches n, and n + (2^63 − 2) is past the word", rep.Proven, rep.Ops)
	}
	if rep.Terminates != rep.Loops {
		t.Errorf("%d of %d loops proven terminating", rep.Terminates, rep.Loops)
	}
	rep = reportGo(t, strings.Replace(src, "K", "9223372036854775706", 1))
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d proven: c ≤ 101 by the derived step's own 1, and 101 + (2^63 − 102) fits", rep.Proven, rep.Ops)
	}
}

// AN OPERATION IS COUNTED ONCE. The guard's subtraction is evaluated by the
// condition and again by the narrowing of i against it; only the first is the
// program's. A connective narrowed its first operand on two paths, three counts.
func TestAGuardsOperationIsCountedOnce(t *testing.T) {
	for _, g := range []string{"(>= i (- n 1))", "(and (>= i (- n 1)) (< i 5))"} {
		rep := reportGo(t, `(export f)
(sig f ((i (int 0 10)) (n (int 0 10))) int)
(def f (i n) (if `+g+` 0 1))`)
		if rep.Ops != 1 {
			t.Errorf("%s: %d operations counted, and the program has one", g, rep.Ops)
		}
	}
}
