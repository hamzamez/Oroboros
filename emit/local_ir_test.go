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
	src := `use go
use go/fmt
use result
fn half(n ∈ 0..1000) (result.Result(Int, Int)) {
    if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) }
}
fn g(n) {
    loop i = 0 {
        i >= n -> half(1)
        else -> { try result.'ok(h) = half(2 * i + 1) { again(i + 1) } }
    }
}
export fn f(n ∈ 0..100) (()) {
    fmt.Println(case g(n) { result.'ok(v) -> v; result.'err(e) -> 0 - e })
}`
	if _, err := entryGo(t, src); err != nil {
		t.Fatal(err)
	}
}

// A TABLE WRITTEN AS ITS GRAPH, INDEXED AT RUN TIME, owes 0 ≤ i < len. The
// shape lowers since local-2026-10-08 (a constant table unfolded at its use),
// so the obligation is what stands between an unbounded index and a read
// outside the literal.
func TestALiteralIndexedAtRunTimeOwesItsBound(t *testing.T) {
	_, err := entryGo(t, `use go
export fn f(n ∈ Int) (Int) { array { 1, 2, 3 }(n) }`)
	if err == nil || !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("an unbounded index into a literal of 3 was accepted: %v", err)
	}
	if _, err := entryGo(t, `use go
export fn f(n ∈ 0..2) (Int) { array { 1, 2, 3 }(n) }`); err != nil {
		t.Errorf("the control, an index in [0, 2], was refused: %v", err)
	}
}

// A TABLE OF A SORT'S ZERO KEEPS ITS SORT. `(table 3 false)` is build's
// zero-filled form, whose element is open, and the buffer is ascribed
// `buffer bool` for the checker alone: a store of an int is refused, as the
// fill loop's store of false made it before the fast path.
func TestAZeroTableKeepsItsSort(t *testing.T) {
	_, err := entryGo(t, `use go
export fn f() (Int) { len({ var b := array { false : _ ∈ 0..2 }; b[0] := 5; b }) }`)
	if err == nil || !strings.Contains(err.Error(), "bool is required here") {
		t.Errorf("an int stored into a table of false was accepted: %v", err)
	}
	if _, err := entryGo(t, `use go
export fn f() (Int) { len({ var b := array { false : _ ∈ 0..2 }; b[0] := true; b }) }`); err != nil {
		t.Errorf("the control, a bool stored, was refused: %v", err)
	}
}
