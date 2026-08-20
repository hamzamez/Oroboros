package emit

import "testing"

// The size-change algebra, against the cases Lee, Jones & Ben-Amram use to
// motivate it — including the one that makes the principle worth having.
func TestSizeChangeAlgebra(t *testing.T) {
	yes := func(int) bool { return true }

	// One variable, strictly down: terminates.
	g := newGraph(1)
	g.set(0, 0, down)
	if ok, _ := SizeChangeTerminates([]scGraph{g}, yes); !ok {
		t.Error("a strictly descending counter should terminate")
	}

	// One variable, only non-increasing: NOT enough. `x' <= x` forever is a
	// loop that never moves.
	h := newGraph(1)
	h.set(0, 0, downEq)
	if ok, _ := SizeChangeTerminates([]scGraph{h}, yes); ok {
		t.Error("non-increase alone must not prove termination")
	}

	// THE CASE THE PRINCIPLE EXISTS FOR. Two variables, two edges, and NEITHER
	// variable descends on both — but every repeatable cycle descends
	// somewhere. A "find a variable that always decreases" check fails here and
	// size change does not.
	//
	//   edge A:  x' < y,  y' <= x
	//   edge B:  x' <= y,  y' < x
	a := newGraph(2)
	a.set(1, 0, down)   // y → x, strict
	a.set(0, 1, downEq) // x → y
	b := newGraph(2)
	b.set(1, 0, downEq)
	b.set(0, 1, down)
	if ok, w := SizeChangeTerminates([]scGraph{a, b}, yes); !ok {
		t.Errorf("the alternating case should terminate; witness %s", w.Render([]string{"x", "y"}))
	}
	// And a naive diagonal check would have said no.
	if _, dA := a.descends(yes); dA {
		t.Error("neither edge descends on its own diagonal; the test is not testing what it claims")
	}

	// A genuine non-terminator: x' <= x and nothing else.
	c := newGraph(2)
	c.set(0, 0, downEq)
	c.set(1, 1, downEq)
	if ok, _ := SizeChangeTerminates([]scGraph{c}, yes); ok {
		t.Error("a cycle with no strict descent must not be proven")
	}

	// Well-foundedness is demanded of the WITNESS. The same descending graph is
	// refused when its variable has no floor.
	none := func(int) bool { return false }
	if ok, _ := SizeChangeTerminates([]scGraph{g}, none); ok {
		t.Error("descent without a floor is not an argument over the integers")
	}
}

// Composition is where the principle lives; a wrong `min`/`max` here would make
// every answer plausible and none of them right.
func TestSizeChangeComposition(t *testing.T) {
	a := newGraph(2)
	a.set(0, 1, down) // x → y strict
	b := newGraph(2)
	b.set(1, 0, downEq) // y → x
	c := compose(a, b)
	if c.at(0, 0) != down {
		t.Errorf("strict on either leg should stay strict, got %v", c.at(0, 0))
	}
	if c.at(1, 1) != noArc {
		t.Errorf("no path from y to y, got %v", c.at(1, 1))
	}
	// Idempotence is what selects the repeatable cycles.
	i := newGraph(1)
	i.set(0, 0, down)
	if !i.idempotent() {
		t.Error("a single strict self-loop is idempotent")
	}
}
