package emit

import (
	"strings"
	"testing"
)

// A SUM AT A JOIN POINT IS ITS TAG AND ITS SLOTS (data.md §5.5.5, Theorem R),
// and three refinement rules make a slot provable where its tag selects it
// (sumrep-2026-10-06). Each row is a program that needs exactly one of them.
func TestASlotIsProvableWhereItsTagSelectsIt(t *testing.T) {
	for _, c := range []struct{ why, src string }{
		{"a table written as its graph has its number of elements as its length", `
(export f)
(sig f ((n (int 0 9))) int)
(def f (n)
  (let (tuple k t) (loop ((i 0)) (>= i n) (tuple 1 (array n 100 7)) else (again (+ i 1)))
    (+ k (t 2))))`},
		{"an #any tail is ⊥ in the join of a slot's tails", `
(variant (found R) (rows R) nothing)
(export f)
(sig f ((n (int 0 9))) int)
(def f (n)
  (case (loop ((i 0)) (>= i n) (rows (array n 100 7)) (= i 5) nothing else (again (+ i 1)))
    (rows t) (t 2)
    nothing 0))`},
		{"an arm whose tag no exit carries is dead: the tag's exact value set, and ex falso", `
(variant (found R) (at int) (rows R) nothing)
(export f)
(sig f ((n (int 0 9))) int)
(def f (n)
  (case (loop ((i 0)) (> i 9) nothing (= i n) (at n) else (again (+ i 1)))
    (at v)   v
    (rows t) (t 2)
    nothing  0))`},
	} {
		if err := refineGo(t, c.src); err != nil {
			t.Errorf("%s: %v", c.why, err)
		}
	}
}

// EX FALSO IS NOT A LICENCE: a guard whose constant IS among the tag's values
// opens a live arm, and an index there must still be proven.
func TestALiveArmIsStillChecked(t *testing.T) {
	err := refineGo(t, `
(variant (found R) (at int) (rows R) nothing)
(export f)
(sig f ((n (int 0 9))) int)
(def f (n)
  (case (loop ((i 0)) (> i 9) nothing (= i n) (rows (array n 7)) else (again (+ i 1)))
    (at v)   v
    (rows t) (t 2)
    nothing  0))`)
	if err == nil || !strings.Contains(err.Error(), "indexing") {
		t.Errorf("an index past a two-element table in a live arm must be refused, got %v", err)
	}
}
