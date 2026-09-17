package emit

import "testing"

// FACTS ABOUT A TABLE'S CONTENTS ARE ESTABLISHED BY INDUCTION ON THE STORE CHAIN
// AND USED AT A PROVEN READ — F-D₁, emit/content.go, array-facts.md §4.
//
// A buffer filled by a loop storing i, under i < n, holds `0 ≤ #e < n` in every
// slot: the zero fill satisfies it when the table is non-empty (and an empty
// table satisfies everything), and every store preserves it (Theorem S′). With
// `n ≤ len a`, a value read out of the frozen table indexes `a` in range
// (Theorem D) — the value clamp freq.oro and tally write by hand.
func TestAStoredBoundHoldsOfEveryRead(t *testing.T) {
	tg := tempTarget(t, ``)
	fill := func(store string) string {
		return `(use tgt)
	  (fn (a n)
	    (if (<= n (len a))
	      (let (build n (fn (b) (loop ((c b) (i 0)) (>= i n) c else (again (set c i ` + store + `) (+ i 1)))))
	        (fn (t) (loop ((k 0) (s 0)) (>= k (len t)) s else (again (+ k 1) (+ s (a (t k)))))))
	      0))`
	}
	notes, err := refineWith(t, tg, fill(`i`))
	if err != nil {
		t.Fatalf("a read of a filled table was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("0 ≤ (t k) < n ≤ len a must be proven from the stores, got: %s", notes)
	}
	// CONTROL, THE STORE OFF BY ONE: `i + 1` reaches n, so `#e < n` is not
	// preserved and the read must stay propagated — never proven.
	notes, err = refineWith(t, tg, fill(`(+ i 1)`))
	if err != nil {
		t.Fatalf("an unprovable read must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Error("a store reaching n was taken as preserving #e < n")
	}
	// CONTROL, THE ZERO FILL: every store is `m - 1`, taken only when `m ≥ 1`, so
	// every store satisfies `#e < m`. A slot never written holds 0, which does not
	// when `m ≤ 0` — and then the loop exits at once, every slot is 0, and
	// `(a 0)` with `len a ≥ m` may be out of range. So the read must stay
	// propagated: Theorem S's base case is a proof obligation, not a given.
	const zero = `(use tgt)
	  (fn (a n m)
	    (if (<= m (len a))
	      (let (build n (fn (b) (loop ((c b) (i 0)) (>= i n) c (< m 1) c else (again (set c i (- m 1)) (+ i 1)))))
	        (fn (t) (loop ((k 0) (s 0)) (>= k (len t)) s else (again (+ k 1) (+ s (a (t k)))))))
	      0))`
	notes, err = refineWith(t, tg, zero)
	if err != nil {
		t.Fatalf("an unprovable read must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Error("the zero fill was taken as satisfying #e < m")
	}
}

// TWO BUFFERS SWAPPED ON THE BACK EDGE: neither `0 ≤ #e < n` is inductive for one
// buffer alone — each receives the other's contents — and both are for the pair,
// which is Theorem S′'s joint induction and what freq.oro's merge sort does.
func TestSwappedBuffersKeepTheirContentJointly(t *testing.T) {
	tg := tempTarget(t, ``)
	swap := func(store string) string {
		return `(use tgt)
	  (fn (a n)
	    (if (<= n (len a))
	      (let (build n (fn (p) (build n (fn (q)
	             (loop ((x p) (y q) (i 0)) (>= i n) x else
	               (again (set y i ` + store + `) x (+ i 1)))))))
	        (fn (t) (loop ((k 0) (s 0)) (>= k (len t)) s else (again (+ k 1) (+ s (a (t k)))))))
	      0))`
	}
	notes, err := refineWith(t, tg, swap(`i`))
	if err != nil {
		t.Fatalf("a read of a swapped table was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("the pair's content must be proven jointly, got: %s", notes)
	}
	notes, err = refineWith(t, tg, swap(`n`))
	if err != nil {
		t.Fatalf("an unprovable read must be propagated, not refused: %v", err)
	}
	if !propagated(notes) {
		t.Error("a store of n into one buffer was taken as preserving #e < n for the pair")
	}
}

// A FACT ABOUT ONE COMPONENT OF A STRIDED TABLE (component.go): slot 2j holds an
// index below n, slot 2j+1 holds anything. A fact about the whole table cannot
// state the bound, because slot 1's values exceed it; the component fact can, and
// a read at `2k + 0` — residue 0 mod 2, decided by the index — instantiates it.
func TestAComponentFactHoldsOfItsResidueClass(t *testing.T) {
	tg := tempTarget(t, ``)
	prog := func(read string) string {
		return `(use tgt)
	  (fn (a n)
	    (if (<= n (len a))
	      (let (build (* 2 n) (fn (b) (loop ((c b) (i 0)) (>= i n) c else
	             (again (set (set c (+ (* 2 i) 0) i) (+ (* 2 i) 1) 1000000) (+ i 1)))))
	        (fn (t) (loop ((k 0) (s 0)) (>= k n) s else
	          (again (+ k 1) (+ s (a ` + read + `))))))
	      0))`
	}
	notes, err := refineWith(t, tg, prog(`(t (+ (* 2 k) 0))`))
	if err != nil {
		t.Fatalf("a read of component 0 was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("component 0 holds indices below n and its read must be proven, got: %s", notes)
	}
	// CONTROL, THE OTHER COMPONENT: slot 2k+1 holds 1000000, so the index is not
	// in range — it must not be proven, whichever way it is reported.
	notes, err = refineWith(t, tg, prog(`(t (+ (* 2 k) 1))`))
	if err == nil && !propagated(notes) {
		t.Error("component 1 was taken to hold indices below n")
	}
	// CONTROL, AN UNDECIDED RESIDUE: `k` alone is not known mod 2, so no component
	// fact applies to `(t k)`.
	notes, err = refineWith(t, tg, prog(`(t k)`))
	if err == nil && !propagated(notes) {
		t.Error("a component fact was applied at an index whose residue is unknown")
	}
	// CONTROL, A CONDITIONAL INDEX OVER TWO RESIDUES: `(if … 0 1)` lands in both
	// components, so the join of its branches' residues knows nothing mod 2.
	notes, err = refineWith(t, tg, prog(`(t (if (< k 1) 0 1))`))
	if err == nil && !propagated(notes) {
		t.Error("a conditional index over residues 0 and 1 was taken to lie in component 0")
	}
}

// A STORE WHOSE RESIDUE IS UNKNOWN touches every component, so it must be checked
// against every component fact — skipping it would keep `#e < n` on component 0
// after a store of 1000000 at an index that may be even.
func TestAStoreOfUnknownResidueTouchesEveryComponent(t *testing.T) {
	tg := tempTarget(t, ``)
	const src = `(use tgt)
	  (fn (a n j)
	    (if (<= n (len a))
	      (let (build (* 2 n) (fn (b) (loop ((c b) (i 0)) (>= i n) c else
	             (again (set (set (set c (+ (* 2 i) 0) i) (+ (* 2 i) 1) 1000000)
	                         (if (< j 0) 0 (if (>= j (* 2 n)) 0 j)) 1000000) (+ i 1)))))
	        (fn (t) (loop ((k 0) (s 0)) (>= k n) s else
	          (again (+ k 1) (+ s (a (t (+ (* 2 k) 0))))))))
	      0))`
	notes, err := refineWith(t, tg, src)
	if err == nil && !propagated(notes) {
		t.Error("a store at an index of unknown residue was taken to leave component 0 alone")
	}
}
