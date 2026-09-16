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

// AN INDEX THAT IS A CONDITIONAL TERM IS PROVEN WHEN EVERY BRANCH IS IN RANGE,
// and stays PROPAGATED — never refused, never proven — when one is not.
// refine.go, provedThroughJoin: a proof attempt for a term outside the fragment,
// so the proven set grows and nothing that built stops building.
func TestAConditionalIndexIsProvenBranchByBranch(t *testing.T) {
	tg := tempTarget(t, ``)
	// A clamp, under a guard that makes the table non-empty: both bounds hold on
	// every branch, so both are PROVEN and no note remains.
	notes, err := refineWith(t, tg, `(use tgt)
	  (fn (a i) (if (>= (len a) 1) (a (if (< i 0) 0 (if (>= i (len a)) 0 i))) 0))`)
	if err != nil {
		t.Fatalf("a clamped index was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("a clamp into a non-empty table must be proven, got: %s", notes)
	}
	// CONTROL: one branch is 5 and the table may be shorter, so the upper bound
	// does not follow — and must be PROPAGATED, exactly as before, not proven.
	notes, err = refineWith(t, tg, `(use tgt)
	  (fn (a i) (if (>= (len a) 1) (a (if (< i 0) 0 5)) 0))`)
	if err != nil {
		t.Fatalf("an unprovable conditional index must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Errorf("a branch outside the table was taken as proven")
	}
	// AND WITHOUT THE GUARD the clamp's zero branch is out of range for an empty
	// table, so it is not proven either — the case freq.oro and tally leave as a note.
	notes, _ = refineWith(t, tg, `(use tgt) (fn (a i) (a (if (< i 0) 0 (if (>= i (len a)) 0 i))))`)
	if !propagated(notes) {
		t.Errorf("a clamp into a possibly empty table was taken as proven")
	}
}
