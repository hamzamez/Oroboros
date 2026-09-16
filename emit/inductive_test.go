package emit

import "testing"

// 0 <= v IS AN INDUCTIVE INVARIANT, AND IS DECIDED AS ONE — refine.go,
// inductiveLowerBounds; Houdini (Flanagan & Leino, FME 2001).
//
// The merge sort's pass loop advances `lo` to `hi = min(n, lo + w)` and doubles
// `w`. Neither step is "self plus a literal", so the syntactic rule gave neither
// variable a lower bound, and an index at `lo` could only be refused. The step
// for `lo` is non-negative given the clause guard `lo < n` and `0 <= lo, 0 <= w`;
// the step for `w` given `0 <= w`. So neither is inductive ALONE and both are
// inductive TOGETHER — which is what the simultaneous induction proves.
func TestALowerBoundIsProvedByInduction(t *testing.T) {
	tg := tempTarget(t, `(sig at ((k int)) int (where (<= 0 k)) (host expr "at(%s)"))`)
	const pass = `(use tgt)
	  (fn (n)
	    (loop ((lo 0) (w 1))
	      (>= lo n) 0
	      else (let (+ lo w) (fn (b)
	           (let (if (< n b) n b) (fn (hi)
	           (let (tgt.at lo) (fn (u) (again hi (* 2 w))))))))))`
	if _, err := refineWith(t, tg, pass); err != nil {
		t.Errorf("0 <= lo is inductive jointly with 0 <= w and must be derived: %v", err)
	}
	// THE SAME STEP IN THE TWO SHAPES REDUCTION LEAVES. β substitutes a clamp
	// used once straight into the `again`, so the back-edge argument is the
	// CONDITIONAL, not a name — and when the clamp is `min` inlined, a `let`
	// binds its operand first, so the join has to know `b = lo + w`.
	for _, step := range []string{
		`(if (< n (+ lo w)) n (+ lo w))`,
		`(let (+ lo w) (fn (b) (if (< n b) n b)))`,
	} {
		src := `(use tgt) (fn (n) (loop ((lo 0) (w 1)) (>= lo n) 0 else
		  (let (tgt.at lo) (fn (u) (again ` + step + ` (* 2 w))))))`
		if _, err := refineWith(t, tg, src); err != nil {
			t.Errorf("step %s: 0 <= lo must be derived: %v", step, err)
		}
	}
	// CONTROL: a step that genuinely can go negative must stay refused. `w`
	// starts at 1 and becomes `w - 3`, so it is not an invariant, and without it
	// `lo + w` bounds nothing below — Houdini drops `w` and then `lo`.
	const falls = `(use tgt)
	  (fn (n)
	    (loop ((lo 0) (w 1))
	      (>= lo n) 0
	      else (let (+ lo w) (fn (b)
	           (let (if (< n b) n b) (fn (hi)
	           (let (tgt.at lo) (fn (u) (again hi (- w 3))))))))))`
	if _, err := refineWith(t, tg, falls); err == nil {
		t.Error("a lower bound was assumed for a variable whose step can go negative")
	}
	// AND THE CANDIDATE SET MUST SHRINK, not merely be tested once: here `w` is
	// not inductive but `lo` does not depend on it (its step is `lo + 1`, which
	// the syntactic rule already gives), while a third variable `m` steps to
	// `m + w`. The greatest inductive subset keeps `lo`, drops `w` and `m`, and
	// an index at `m` is refused.
	const shrink = `(use tgt)
	  (fn (n)
	    (loop ((lo 0) (w 1) (m 0))
	      (>= lo n) 0
	      else (let (tgt.at m) (fn (u) (again (+ lo 1) (- w 3) (+ m w))))))`
	if _, err := refineWith(t, tg, shrink); err == nil {
		t.Error("m depends on a non-inductive w and must not be given 0 <= m")
	}
}
