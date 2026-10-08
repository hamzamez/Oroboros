package emit_test

import (
	"strings"
	"testing"
)

// A SLOT NO TAIL SELECTS HAS EVERY TYPE (local-2026-10-08). A loop yields a
// sum whose success slot is the unconstrained value at every exit, since every
// exit here fails; its binder in the case is typed bottom, so the conditional
// joining it with the failure arm's int is an int, which then enters
// fmt.Println's box. Typed by its demand instead, the slot was go.Value, and
// the conditional's branches go.Value and int did not join: try-loop's harness
// stopped building once the residual let's β folded the one success away.
func TestASlotNoTailSelectsHasEveryType(t *testing.T) {
	src := `(use go) (use go/fmt as fmt) (use result) (export f)
(sig half ((n (int 0 1000))) (result.result int int))
(def half (n) (if (= (% n 2) 0) (result.ok (/ n 2)) (result.err n)))
(def g (n)
  (loop ((i 0)) (>= i n) (half 1)
    else (try (result.ok h) (half (+ (* 2 i) 1)) (again (+ i 1)))))
(sig f ((n (int 0 100))) (tuple))
(def f (n) (fmt.Println (case (g n) (result.ok v) v (result.err e) (- 0 e))))`
	if _, err := entryGo(t, src); err != nil {
		t.Fatal(err)
	}
}

// A TABLE WRITTEN AS ITS GRAPH, INDEXED AT RUN TIME, owes 0 ≤ i < len. The
// shape lowers since local-2026-10-08 (a constant table unfolded at its use),
// so the obligation is what stands between an unbounded index and a read
// outside the literal.
func TestALiteralIndexedAtRunTimeOwesItsBound(t *testing.T) {
	_, err := entryGo(t, `(use go) (export f) (sig f ((n int)) int) (def f (n) ((array 1 2 3) n))`)
	if err == nil || !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("an unbounded index into a literal of 3 was accepted: %v", err)
	}
	if _, err := entryGo(t, `(use go) (export f) (sig f ((n (int 0 2))) int) (def f (n) ((array 1 2 3) n))`); err != nil {
		t.Errorf("the control, an index in [0, 2], was refused: %v", err)
	}
}
