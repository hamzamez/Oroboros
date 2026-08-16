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
	tg, err := LoadTarget("../targets/go.oro")
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
