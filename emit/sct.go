package emit

import (
	"fmt"
	"sort"
	"strings"
)

// Size-change termination — Lee, Jones & Ben-Amram, *The size-change principle
// for program termination* (POPL 2001).
//
// THE PRINCIPLE. Build, for each back edge, a graph relating the loop's
// variables before to the same variables after, with arcs labelled *strictly
// decreases* or *does not increase*. Compose those graphs along every possible
// sequence of back edges and close the set under composition. The program
// terminates if **every idempotent graph in the closure has an arc x → x
// labelled strict** — because such a graph describes a repeatable cycle, and a
// repeatable cycle in which some value strictly descends forever is impossible
// over a well-founded order.
//
// The power of the principle over "find a variable that always decreases" is
// that different variables may descend on different paths. A loop that
// alternately shrinks x and shrinks y terminates, and no single variable
// witnesses it; the idempotent composition does.
//
// WHAT WE ADD, AND WHY WE MUST. Classical SCT assumes a well-founded order —
// structural descent on data. Our loop variables are INTEGERS, and `x → x-1` is
// not well-founded without a floor. So the descent argument is only sound here
// when the interval analysis also supplies a lower bound, and the two passes are
// deliberately joined: SCT provides the descent, intervals provide the floor.
//
// This closes two holes at once. concerns.md §2.1 records that our termination
// guard is "a *mechanism*, not a proof, and the fuel limit is an admission that
// the mechanism is incomplete". And intervals-2026-08-19 found that the entire
// unproven residue is a variable bounded by the TRIP COUNT rather than by a
// guard — and a trip count is a termination argument with a number attached.

// arc is the label on a size-change edge. The order matters: composition takes
// the minimum along a path and the maximum across paths, so `down` must be the
// largest.
type arc uint8

const (
	noArc  arc = iota // nothing is known relating these two
	downEq            // x' ≤ x
	down              // x' < x
)

func (a arc) String() string {
	switch a {
	case down:
		return "v"
	case downEq:
		return "="
	}
	return "."
}

// scGraph relates n variables before a back edge to the same n after.
// g[i][j] is what is known about variable j AFTER against variable i BEFORE.
type scGraph struct {
	n int
	a []arc // n*n, row-major
}

func newGraph(n int) scGraph { return scGraph{n: n, a: make([]arc, n*n)} }

func (g scGraph) at(i, j int) arc { return g.a[i*g.n+j] }
func (g *scGraph) set(i, j int, v arc) {
	if v > g.a[i*g.n+j] {
		g.a[i*g.n+j] = v
	}
}

func (g scGraph) key() string {
	var b strings.Builder
	for _, v := range g.a {
		b.WriteString(v.String())
	}
	return b.String()
}

// compose is graph concatenation: an arc exists from i to k if some j carries
// arcs both ways, and it is strict if EITHER leg is strict.
func compose(g, h scGraph) scGraph {
	out := newGraph(g.n)
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			x := g.at(i, j)
			if x == noArc {
				continue
			}
			for k := 0; k < g.n; k++ {
				y := h.at(j, k)
				if y == noArc {
					continue
				}
				v := downEq
				if x == down || y == down {
					v = down
				}
				out.set(i, k, v)
			}
		}
	}
	return out
}

// idempotent graphs are exactly the ones describing a cycle that can repeat.
func (g scGraph) idempotent() bool { return compose(g, g).key() == g.key() }

// descends reports whether some variable strictly decreases along this cycle
// AND is measured over a well-founded order.
//
// The second half is ours rather than Lee-Jones-Ben-Amram's, and it is not
// optional: integers descend forever unless something stops them. It is checked
// per WITNESS rather than per variable, which matters — a loop that counts
// primes into an unbounded accumulator still terminates, because the accumulator
// is not what carries the argument.
func (g scGraph) descends(wf func(int) bool) (int, bool) {
	for i := 0; i < g.n; i++ {
		if g.at(i, i) == down && (wf == nil || wf(i)) {
			return i, true
		}
	}
	return 0, false
}

// SizeChangeTerminates applies the criterion to one loop's back edges.
//
// Returns the verdict and, when it fails, the idempotent graph that witnesses
// the failure — which is the useful half of a negative answer, because it names
// the cycle nothing was shown to shrink.
func SizeChangeTerminates(edges []scGraph, wf func(int) bool) (bool, scGraph) {
	if len(edges) == 0 {
		return true, scGraph{} // no back edge is a loop that runs once
	}
	seen := map[string]scGraph{}
	var work []scGraph
	add := func(g scGraph) {
		if _, dup := seen[g.key()]; !dup {
			seen[g.key()] = g
			work = append(work, g)
		}
	}
	for _, e := range edges {
		add(e)
	}
	// Closure under composition. The set of n×n graphs over three labels is
	// finite, so this terminates; the bound is generous rather than tight.
	for len(work) > 0 && len(seen) < 4096 {
		g := work[len(work)-1]
		work = work[:len(work)-1]
		for _, e := range edges {
			add(compose(g, e))
		}
	}
	// Deterministic order, so a failure witness does not depend on map
	// iteration.
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g := seen[k]
		if g.idempotent() {
			if _, ok := g.descends(wf); !ok {
				return false, g
			}
		}
	}
	return true, scGraph{}
}

// Render prints a graph against variable names, for diagnostics.
func (g scGraph) Render(names []string) string {
	var b strings.Builder
	for i := 0; i < g.n && i < len(names); i++ {
		for j := 0; j < g.n && j < len(names); j++ {
			switch g.at(i, j) {
			case down:
				fmt.Fprintf(&b, "%s>%s ", names[i], names[j])
			case downEq:
				fmt.Fprintf(&b, "%s>=%s ", names[i], names[j])
			}
		}
	}
	if b.Len() == 0 {
		return "(nothing known)"
	}
	return strings.TrimSpace(b.String())
}
