package ir

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TERMINATION, CHECKED IN THE DIRECTION THAT MATTERS. A loop proven to halt that
// does not is a false proof; one not proven is a lost count. So the tests are
// (1) the closure on graph sets whose answer is known, and (2) programs each of
// which has an input that runs FOREVER, one per law's premise, which must not be
// proven. A planted fault in a law turns one of them into a false proof.

func graph(n int, arcs ...[3]int) scGraph {
	g := newSCGraph(n)
	for _, a := range arcs {
		g.set(a[0], a[1], scArc(a[2]))
	}
	return g
}

func TestTheClosureDecidesTheTheorem(t *testing.T) {
	all := func(int) bool { return true }
	none := func(int) bool { return false }
	cases := []struct {
		name  string
		edges []scGraph
		floor func(int) bool
		halts bool
	}{
		{"a strict self-arc", []scGraph{graph(1, [3]int{0, 0, 2})}, all, true},
		{"a weak self-arc only", []scGraph{graph(1, [3]int{0, 0, 1})}, all, false},
		{"strict but no floor", []scGraph{graph(1, [3]int{0, 0, 2})}, none, false},
		// Lee, Jones and Ben-Amram's point: x shrinks on one edge, y on the
		// other, each keeping the other; no single variable descends on both.
		{"alternating descent", []scGraph{
			graph(2, [3]int{0, 0, 2}, [3]int{1, 1, 1}),
			graph(2, [3]int{0, 0, 1}, [3]int{1, 1, 2}),
		}, all, true},
		// Euclid: x' = y, y' = x mod y < y.
		{"Euclid", []scGraph{graph(2, [3]int{1, 0, 1}, [3]int{1, 1, 2})}, all, true},
		// A swap with no descent: x' = y, y' = x.
		{"a swap", []scGraph{graph(2, [3]int{1, 0, 1}, [3]int{0, 1, 1})}, all, false},
		// One edge shrinks x, the other grows it back (no arc): not proven.
		{"undone descent", []scGraph{graph(1, [3]int{0, 0, 2}), graph(1)}, all, false},
		{"no back edge", nil, none, true},
	}
	for _, c := range cases {
		if got := sizeChangeTerminates(c.edges, c.floor); got != c.halts {
			t.Errorf("%s: halts = %v, want %v", c.name, got, c.halts)
		}
	}
}

// divergent programs: each has an input on which its loop never ends, and names
// the premise that input violates.
var divergent = []struct{ why, body string }{
	{"a descent with no floor: i falls forever below 100", `(loop ((i n)) (>= i 100) i else (again (- i 1)))`},
	{"a quotient at 0: 0 / 2 is 0", `(loop ((x n) (k 0)) (> x 100) k else (again (/ x 2) (+ k 1)))`},
	{"a remainder by a divisor that is not the variable: n = 0 keeps y' = x mod (y+1) = 0",
		`(loop ((x n) (y n)) (= y 3) x else (again y (% x (+ y 1))))`},
	{"a remainder does not fall below its dividend: x mod 13 = x for x < 13",
		`(loop ((x n)) (= x 7) x else (again (% x 13)))`},
	{"a reset not below the guard: 5 stays 5", `(loop ((v n) (k 0)) (= v 0) k (>= v 10) (again (- v 10) k) else (again 5 k))`},
	{"a parameter that does not move on one path", `(loop ((i 0)) (>= i n) i (= (% i 2) 0) (again i) else (again (+ i 1)))`},
	{"an ascent with no ceiling", `(loop ((i n) (s 0)) (< i 0) s else (again (+ i 1) (+ s 1)))`},
}

func TestADivergentLoopIsNotProven(t *testing.T) {
	dir := t.TempDir()
	for k, c := range divergent {
		src := fmt.Sprintf("(export run)\n(sig run ((n (int 0 12))) int)\n(def run (n)\n  %s)\n", c.body)
		path := filepath.Join(dir, fmt.Sprintf("d%d.oro", k))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, p, err := compilePath(path, "go", Options{Decided: true})
		if err != nil {
			t.Fatalf("%s: %v", c.why, err)
		}
		f := p.Funcs[0]
		if leg := Decide(tg, f, true); leg.Loops == 0 || leg.Halts == leg.Loops {
			t.Errorf("%s: %d of %d loops proven to halt, and one does not\n%s", c.why, leg.Halts, leg.Loops, src)
		}
	}
}

// TestTheLawsProveWhatTheyShould: the same shapes with the premise restored
// halt, and are proven, so the divergent cases above are refused for the
// premise and not for want of a rule.
func TestTheLawsProveWhatTheyShould(t *testing.T) {
	proven := []string{
		`(loop ((i n)) (< i 0) i else (again (- i 1)))`,
		`(loop ((x (+ n 1)) (k 0)) (< x 1) k else (again (/ x 2) (+ k 1)))`,
		`(loop ((x n) (y (+ n 1))) (< y 1) x else (again y (% x y)))`,
		`(loop ((v n) (k 0)) (= v 0) k (>= v 10) (again (- v 10) k) else (again 0 k))`,
		`(loop ((i 0)) (>= i n) i else (again (+ i 1)))`,
	}
	dir := t.TempDir()
	for k, body := range proven {
		src := fmt.Sprintf("(export run)\n(sig run ((n (int 0 12))) int)\n(def run (n)\n  %s)\n", body)
		path := filepath.Join(dir, fmt.Sprintf("p%d.oro", k))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, p, err := compilePath(path, "go", Options{Decided: true})
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		if leg := Decide(tg, p.Funcs[0], true); leg.Halts != leg.Loops {
			t.Errorf("%d of %d loops proven to halt\n%s", leg.Halts, leg.Loops, src)
		}
	}
}
