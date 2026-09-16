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
