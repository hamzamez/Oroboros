package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/refinements.md. `aindex` is Tier 1 only WITHIN BOUNDS
// (primitives.md §2); this is the check that makes the condition real.

func refineSrc(t *testing.T, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/portable-go.oro")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
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

// §5 — a loop index is in bounds for the array that bounds it.
func TestRefineProvesTheLoopIndex(t *testing.T) {
	if err := refineSrc(t, `
		(use num/f64)
		(fn (a) (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (aindex a i)))))
	`); err != nil {
		t.Errorf("the bounding array's own index must be provable: %v", err)
	}
}

// §4 — the stencil: j+2 < alen a follows from j < alen a - 2, which needs the
// constant offsets to be compared in the right direction.
func TestRefineProvesTheStencilWindow(t *testing.T) {
	if err := refineSrc(t, `
		(use num/f64) (use num/int)
		(fn (a) (fold-range 0.0 (int.sub (alen a) 2)
		          (fn (acc j) (f64.add acc (aindex a (int.add j 2))))))
	`); err != nil {
		t.Errorf("the stencil window must be provable: %v", err)
	}
}

// The negative case: one past the end.
func TestRefineRejectsOutOfBounds(t *testing.T) {
	err := refineSrc(t, `
		(use num/f64) (use num/int)
		(fn (a) (fold-range 0.0 (alen a)
		          (fn (acc i) (f64.add acc (aindex a (int.add i 1))))))
	`)
	if err == nil {
		t.Fatal("indexing one past the end must be rejected")
	}
	if !strings.Contains(err.Error(), "known:") {
		t.Errorf("the diagnostic should say what was known, got %v", err)
	}
}

// §5 — a second array is NOT in bounds without a precondition. This is the
// latent bug the checker found in dot.oro and centroid.oro on the day it was
// written.
func TestRefineRejectsASecondArray(t *testing.T) {
	if err := refineSrc(t, `
		(use num/f64)
		(fn (p q) (fold-range 0.0 (alen p) (fn (acc i) (f64.add acc (aindex q i)))))
	`); err == nil {
		t.Fatal("a second array with an unrelated length must not be assumed in bounds")
	}
}

// …and the precondition discharges it, by becoming a substitution rather than
// two inequalities.
func TestRefineAcceptsASecondArrayGivenAPrecondition(t *testing.T) {
	if err := refineSrc(t, `
		(use num/f64) (use num/int)
		(export two)
		(sig two ((p vec-f64) (q vec-f64)) f64
		  (where (int.eq (alen p) (alen q))))
		(def two (fn (p q) (fold-range 0.0 (alen p) (fn (acc i) (f64.add acc (aindex q i))))))
	`); err != nil {
		t.Errorf("the precondition should discharge it: %v", err)
	}
}

// §3 — an assumption OUTSIDE the fragment must be kept, not dropped. It cannot
// decide a linear obligation, and the diagnostic must still say it was there:
// `known: nothing` was a lie whenever a program declared a `where` the solver
// could not read.
func TestOpaqueAssumptionIsKept(t *testing.T) {
	err := refineSrc(t, `
		(use num/f64)
		(use num/int)
		(export f)
		(sig f ((a vec-f64) (k int)) f64
		  (where (ascii? k)))
		(def f (fn (a k) (aindex a k)))
	`)
	if err == nil {
		t.Fatal("an opaque assumption must not discharge a bounds obligation")
	}
	if !strings.Contains(err.Error(), "assumed (ascii? k)") {
		t.Errorf("the diagnostic must report what was assumed; got %v", err)
	}
}

// iteration.md §6: a loop's guard is written down, so the refinement checker
// gets `0 <= i` and `i < alen a` from the clauses themselves — MORE than
// fold-range offers, where the bound is implied by the primitive.
func TestLoopGuardsDischargeBounds(t *testing.T) {
	if err := refineSrc(t, `
		(use num/f64 as f)
		(use num/int as int)
		(export find)
		(sig find ((a vec-f64) (k f64)) int)
		(def find (fn (a k)
		  (loop ((i 0))
		    (int.ge i (alen a))    -1
		    (f.gt (aindex a i) k)  i
		    else                   (again (int.add i 1)))))
	`); err != nil {
		t.Errorf("a loop's own guards should prove its index in bounds: %v", err)
	}
	// And without the range guard it must NOT be discharged.
	err := refineSrc(t, `
		(use num/f64 as f)
		(use num/int as int)
		(export find)
		(sig find ((a vec-f64) (k f64)) int)
		(def find (fn (a k)
		  (loop ((i 0))
		    (f.gt (aindex a i) k)  i
		    else                   (again (int.add i 1)))))
	`)
	if err == nil {
		t.Error("with no range guard the index is unproven and must be reported")
	}
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
			(use num/vec)
			(use num/int)
			(fn (a)
			  (loop ((k 0) (acc 0))
			    (int.ge (int.mul 4 k) (vec.alen a))  acc
			    else (again (int.add k 1) (int.add acc (vec.aindex a (int.mul 4 k))))))`},
		{"stride 2, offset 1", `
			(use num/vec)
			(use num/int)
			(fn (a)
			  (loop ((k 1) (acc 0))
			    (int.ge (int.add (int.mul 2 k) 1) (vec.alen a))  acc
			    else (again (int.add k 1)
			                (int.add acc (vec.aindex a (int.add (int.mul 2 k) 1))))))`},
	} {
		if err := refineSrc(t, e.src); err != nil {
			t.Errorf("%s: a strided index bounded by its own guard must be provable: %v",
				e.name, err)
		}
	}
}

// The multiplier must be POSITIVE and must divide exactly: scaling by a
// negative flips the inequality, and a fractional one is not this procedure's
// business. `k >= 0` says nothing about `3*k - 1 >= 0` when k is 0.
func TestScaleToRejectsUnsound(t *testing.T) {
	fact := &linear{coef: map[string]int64{"k": -1}, konst: 0}   // -k <= 0, k >= 0
	goal := &linear{coef: map[string]int64{"k": 3}, konst: 0}    // 3k <= 0
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
