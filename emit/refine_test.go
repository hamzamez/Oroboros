package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/refinements.md. A table's application is defined only WITHIN
// BOUNDS (primitives.md §2); this is the check that makes the condition real.
//
// The programs are on the native Go target and use the language's own table
// operations — application, `len`, `loop` — so what they test is the
// refinement layer, not a target's spelling. Until portable-2026-09-29 they
// were written against the portable layer's `aindex` and `fold-range`.

func refineSrc(t *testing.T, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read("(use go)\n" + src)
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	var term *core.Term
	var sig *core.Sig
	if len(prog.Exports) > 0 {
		q := prog.Exports[0]
		term, sig = prog.Defs[q], prog.Sigs[q]
	} else {
		term = terms[0]
	}
	nf, err := core.Normalize(term, env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Refine(tg, "test", sig, nf)
	return err
}

// refused asserts that err is a refusal of a table's index, not of anything
// else. A refusal test on a target that does not know a name passes for the
// wrong reason: pointed at the native target unchanged, all but one of this
// file's refusal tests still passed.
func refused(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s must be refused", what)
	}
	if !strings.Contains(err.Error(), "is an indexing") || !strings.Contains(err.Error(), "does not follow") {
		t.Errorf("%s must be refused as an index obligation, got %v", what, err)
	}
}

// §5 — a loop index is in bounds for the table that bounds it.
func TestRefineProvesTheLoopIndex(t *testing.T) {
	if err := refineSrc(t, `
		(export f)
		(sig f ((a (array f64))) f64)
		(def f (a)
		  (loop ((acc 0.0) (i 0))
		    (>= i (len a))  acc
		    else            (again (go.f+ acc (a i)) (+ i 1))))
	`); err != nil {
		t.Errorf("the bounding table's own index must be provable: %v", err)
	}
}

// §4 — the stencil: j+2 < len a follows from j < len a - 2, which needs the
// constant offsets to be compared in the right direction.
func TestRefineProvesTheStencilWindow(t *testing.T) {
	if err := refineSrc(t, `
		(export f)
		(sig f ((a (array f64))) f64)
		(def f (a)
		  (loop ((acc 0.0) (j 0))
		    (>= j (- (len a) 2))  acc
		    else                  (again (go.f+ acc (a (+ j 2))) (+ j 1))))
	`); err != nil {
		t.Errorf("the stencil window must be provable: %v", err)
	}
}

// The negative case: one past the end.
func TestRefineRejectsOutOfBounds(t *testing.T) {
	err := refineSrc(t, `
		(export f)
		(sig f ((a (array f64))) f64)
		(def f (a)
		  (loop ((acc 0.0) (i 0))
		    (>= i (len a))  acc
		    else            (again (go.f+ acc (a (+ i 1))) (+ i 1))))
	`)
	refused(t, err, "indexing one past the end")
	if err != nil && !strings.Contains(err.Error(), "known:") {
		t.Errorf("the diagnostic should say what was known, got %v", err)
	}
}

// §5 — a second table is NOT in bounds without a precondition. This is the
// latent bug the checker found in dot.oro and centroid.oro on the day it was
// written.
func TestRefineRejectsASecondArray(t *testing.T) {
	refused(t, refineSrc(t, `
		(export two)
		(sig two ((p (array f64)) (q (array f64))) f64)
		(def two (p q)
		  (loop ((acc 0.0) (i 0))
		    (>= i (len p))  acc
		    else            (again (go.f+ acc (q i)) (+ i 1))))
	`), "a second table with an unrelated length")
}

// …and the precondition discharges it, by becoming a substitution rather than
// two inequalities.
func TestRefineAcceptsASecondArrayGivenAPrecondition(t *testing.T) {
	if err := refineSrc(t, `
		(export two)
		(sig two ((p (array f64)) (q (array f64))) f64
		  (where (= (len p) (len q))))
		(def two (p q)
		  (loop ((acc 0.0) (i 0))
		    (>= i (len p))  acc
		    else            (again (go.f+ acc (q i)) (+ i 1))))
	`); err != nil {
		t.Errorf("the precondition should discharge it: %v", err)
	}
}

// §3 — an assumption OUTSIDE the fragment must be kept, not dropped. It cannot
// decide a linear obligation, and the diagnostic must still say it was there:
// `known: nothing` was a lie whenever a program declared a `where` the solver
// could not read. A product of two variables is outside the linear fragment.
func TestOpaqueAssumptionIsKept(t *testing.T) {
	err := refineSrc(t, `
		(export f)
		(sig f ((a (array f64)) (k (int 0 1000))) f64
		  (where (< (len a) (* k k))))
		(def f (a k) (a k))
	`)
	refused(t, err, "an index under an opaque assumption")
	if err != nil && !strings.Contains(err.Error(), "assumed (lt (len a) (* k k))") {
		t.Errorf("the diagnostic must report what was assumed; got %v", err)
	}
}

// And one before the start: `0 <= i - 1` does not follow from `0 <= i`.
func TestRefineRejectsBeforeTheStart(t *testing.T) {
	refused(t, refineSrc(t, `
		(export f)
		(sig f ((a (array f64))) f64)
		(def f (a)
		  (loop ((acc 0.0) (i 0))
		    (>= i (len a))  acc
		    else            (again (go.f+ acc (a (- i 1))) (+ i 1))))
	`), "indexing one before the start")
}

// iteration.md §6: a loop's guard is written down, so the refinement checker
// gets `0 <= i` and `i < len a` from the clauses themselves.
func TestLoopGuardsDischargeBounds(t *testing.T) {
	if err := refineSrc(t, `
		(export find)
		(sig find ((a (array f64)) (k f64)) int)
		(def find (a k)
		  (loop ((i 0))
		    (>= i (len a))       -1
		    (go.f> (a i) k)      i
		    else                 (again (+ i 1))))
	`); err != nil {
		t.Errorf("a loop's own guards should prove its index in bounds: %v", err)
	}
	// And without the range guard it must NOT be discharged.
	refused(t, refineSrc(t, `
		(export find)
		(sig find ((a (array f64)) (k f64)) int)
		(def find (a k)
		  (loop ((i 0))
		    (go.f> (a i) k)      i
		    else                 (again (+ i 1))))
	`), "an index with no range guard")
}

// A ZERO DIVISOR IS A PRECONDITION (integers.md §5), and `d ≠ 0` is a
// DISJUNCTION — `d < 0 ∨ d > 0` — where the fragment is conjunctions of linear
// inequalities. It is discharged by case split: prove either side.
func TestDisequalityDischargedByCaseSplit(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// Guarded: the else-branch of `(== b 0)` says exactly what is needed.
	ok := reduce(t, `(use go) (fn (a b) (if (go.== b 0) 0 (go./ a b)))`, "go")
	if _, err := Refine(tg, "guarded", nil, ok); err != nil {
		t.Errorf("a guarded divisor should discharge: %v", err)
	}
	// A positive lower bound is the other way to prove it.
	pos := reduce(t, `(use go) (fn (a b) (if (go.> b 0) (go./ a b) 0))`, "go")
	if _, err := Refine(tg, "positive", nil, pos); err != nil {
		t.Errorf("a positive divisor should discharge: %v", err)
	}
	// And nothing at all must NOT discharge, or the check is decoration.
	bad := reduce(t, `(use go) (fn (a b) (go./ a b))`, "go")
	if _, err := Refine(tg, "bare", nil, bad); err == nil {
		t.Error("an unbounded divisor must be refused")
	}
}

// negate resolves the target's own spelling. Without that it worked on the
// portable layer and silently did nothing on every native target, so the second
// half of Hoare logic never fired where programs actually live.
func TestNegateHandlesOperatorSpellings(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(go.< a b)`, `(go.ge a b)`},
		{`(go.>= a b)`, `(go.lt a b)`},
		{`(go.== a b)`, `(go.ne a b)`},
		{`(num/int.lt a b)`, `(num/int.ge a b)`},
	} {
		in, err := core.ReadTerm(c.src)
		if err != nil {
			t.Fatal(err)
		}
		got := negate(in)
		if got == nil || got.String() != c.want {
			t.Errorf("negate %s gave %v, want %s", c.src, got, c.want)
		}
	}
}

// A STRIDED INDEX needs one Farkas multiplier, and until it had one the only
// way to write a flat node table was to clamp every access
// (json-tree-bench-2026-08-26).
//
// `entails` matched a fact against a goal by requiring identical coefficients,
// so `k < 512` could not discharge `4*k < 2048` — a consequence immediate
// enough that the gap read as a missing fact rather than a missing inference.
// Scaling a fact by a positive integer is the fix, and it is what a stride
// always needs, because `(go.* 4 k)` has a coefficient the guard bounding `k`
// does not.
func TestStridedIndexIsProvable(t *testing.T) {
	for _, e := range []struct{ name, src string }{
		{"stride 4", `
			(export f)
			(sig f ((a (array int))) int)
			(def f (a)
			  (loop ((k 0) (acc 0))
			    (>= (* 4 k) (len a))  acc
			    else (again (+ k 1) (+ acc (a (* 4 k))))))`},
		// The guard bounds k, not 4k: `k < n` gives `4k < 4n = len a` by the
		// multiplier 4. Two routes find that certificate, scaleTo and the
		// Farkas elimination, and each proves every case here alone; with both
		// disabled `0 <= 4k` fails (portable-2026-09-29 §3).
		{"stride 4, guard on k", `
			(export f)
			(sig f ((a (array int)) (n (int 0 1000))) int
			  (where (= (len a) (* 4 n))))
			(def f (a n)
			  (loop ((k 0) (acc 0))
			    (>= k n)  acc
			    else (again (+ k 1) (+ acc (a (* 4 k))))))`},
		{"stride 2, offset 1", `
			(export f)
			(sig f ((a (array int))) int)
			(def f (a)
			  (loop ((k 1) (acc 0))
			    (>= (+ (* 2 k) 1) (len a))  acc
			    else (again (+ k 1) (+ acc (a (+ (* 2 k) 1))))))`},
	} {
		if err := refineSrc(t, e.src); err != nil {
			t.Errorf("%s: a strided index bounded by its own guard must be provable: %v",
				e.name, err)
		}
	}
}

// And a stride its guard does not bound is refused: `4k < len a` says nothing
// about `4k + 4`.
func TestStridedIndexPastItsGuardIsRefused(t *testing.T) {
	refused(t, refineSrc(t, `
		(export f)
		(sig f ((a (array int))) int)
		(def f (a)
		  (loop ((k 0) (acc 0))
		    (>= (* 4 k) (len a))  acc
		    else (again (+ k 1) (+ acc (a (+ (* 4 k) 4))))))
	`), "a strided index one stride past its guard")
}

// The multiplier must be POSITIVE and must divide exactly: scaling by a
// negative flips the inequality, and a fractional one is not this procedure's
// business. `k >= 0` says nothing about `3*k - 1 >= 0` when k is 0.
func TestScaleToRejectsUnsound(t *testing.T) {
	fact := &linear{coef: map[string]int64{"k": -1}, konst: 0} // -k <= 0, k >= 0
	goal := &linear{coef: map[string]int64{"k": 3}, konst: 0}  // 3k <= 0
	if _, ok := scaleTo(fact, goal); ok {
		t.Error("a negative multiplier must be refused: it reverses the inequality")
	}
	frac := &linear{coef: map[string]int64{"k": 2}, konst: 0}
	odd := &linear{coef: map[string]int64{"k": 3}, konst: 0}
	if _, ok := scaleTo(frac, odd); ok {
		t.Error("a fractional multiplier must be refused")
	}
	two := &linear{coef: map[string]int64{"k": 2, "j": 4}, konst: 0}
	one := &linear{coef: map[string]int64{"k": 1, "j": 2}, konst: 0}
	if m, ok := scaleTo(one, two); !ok || m != 2 {
		t.Errorf("a uniform multiplier must be found: got %d %v", m, ok)
	}
}
