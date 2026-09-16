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

// A CONDITIONAL NESTED UNDER ARITHMETIC IS SPLIT ON: case-of-case in the logic.
// `4·(clamp k) + 2 < 4·512` is outside the fragment as written — the clamp is an
// alien inside a linear skeleton — and no template join foresees 2048. Splitting
// the GOAL on the clamp's branches proves it exactly (refine.go, provedBySplit).
func TestAClampUnderArithmeticIsSplitOn(t *testing.T) {
	tg := tempTarget(t, ``)
	const strided = `(use tgt)
	  (fn (k) (build 2048 (fn (a) (a (+ (* 4 (if (< k 0) 0 (if (>= k 512) 0 k))) 2)))))`
	notes, err := refineWith(t, tg, strided)
	if err != nil {
		t.Fatalf("a strided clamp was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("4·clamp(k)+2 < 2048 must be proven by splitting, got: %s", notes)
	}
	// CONTROL: one slot too far — stride 4 with offset 4 reaches 2048 — must stay
	// propagated, never proven.
	const over = `(use tgt)
	  (fn (k) (build 2048 (fn (a) (a (+ (* 4 (if (< k 0) 0 (if (>= k 512) 0 k))) 4)))))`
	notes, err = refineWith(t, tg, over)
	if err != nil {
		t.Fatalf("an unprovable strided index must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Error("an index reaching past the table was taken as proven")
	}
	// A `let` INSIDE the arithmetic: its binder becomes a fresh name, equal to the
	// value when the value is linear.
	const let = `(use tgt)
	  (fn (k) (build 2048 (fn (a) (a (+ (* 4 (let (+ k 1) (fn (i) (if (< i 1) 0 (if (>= i 512) 0 i))))) 3)))))`
	notes, err = refineWith(t, tg, let)
	if err != nil {
		t.Fatalf("a let-bound strided clamp was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("a let-bound clamp under arithmetic must be proven, got: %s", notes)
	}
}

// A LOOP'S RESULT IS SUMMARISED FROM ITS EXITS, UNDER ITS INVARIANTS.
//
// A count that advances with its index satisfies `n ≤ i` — a difference template
// — and `i ≤ len a` holds of the index, so both exits return a value `≤ len a`.
// With the count bound, `nw ≥ 1` and `nw ≤ len a` give `len a ≥ 1`: three facts
// combined, which is Fourier–Motzkin's job, and what makes `(a 0)` in range.
func TestALoopResultIsBoundedByItsExits(t *testing.T) {
	tg := tempTarget(t, ``)
	const count = `(use tgt)
	  (fn (a)
	    (let (loop ((n 0) (i 0)) (>= i (len a)) n else (again (+ n 1) (+ i 1))) (fn (nw)
	      (if (>= nw 1) (a (if (< nw 0) 0 0)) 0))))`
	notes, err := refineWith(t, tg, count)
	if err != nil {
		t.Fatalf("a count bounded by its loop was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("nw ≤ len a must be summarised and (a 0) proven, got: %s", notes)
	}
	// THREE FACTS, freq's own shape: 0 ≤ u, u < nw and nw ≤ len a give 0 < len a.
	// No single fact and no sum of two reaches it; eliminating u and nw does.
	const three = `(use tgt)
	  (fn (a u)
	    (let (loop ((n 0) (i 0)) (>= i (len a)) n else (again (+ n 1) (+ i 1))) (fn (nw)
	      (if (>= u 0) (if (< u nw) (a (if (< u 0) 0 0)) 0) 0))))`
	notes, err = refineWith(t, tg, three)
	if err != nil {
		t.Fatalf("a three-fact entailment was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("0 < len a from 0 ≤ u < nw ≤ len a must be proven by elimination, got: %s", notes)
	}
	// CONTROL: a count that advances TWICE as fast is not below the index, so no
	// summary relates it to the table, and the read stays propagated.
	const twice = `(use tgt)
	  (fn (a)
	    (let (loop ((n 0) (i 0)) (>= i (len a)) n else (again (+ n 2) (+ i 1))) (fn (nw)
	      (if (>= nw 1) (a (if (< nw 0) 0 0)) 0))))`
	notes, err = refineWith(t, tg, twice)
	if err != nil {
		t.Fatalf("an unprovable read must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Error("a count that can exceed the table's length was summarised as bounded by it")
	}
}
