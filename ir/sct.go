package ir

import (
	"sort"
	"strings"
)

// TERMINATION ON THE IR: size-change termination over integer measures
// (Lee, Jones and Ben-Amram, "The size-change principle for program
// termination", POPL 2001), with the floor the integers lack supplied by the
// interval domain. It is the term analysis's argument (emit/sct.go), read off
// the IR's steps.
//
// THE GRAPHS. For a loop with parameters p₁…pₙ, each back edge e (a continue
// that some execution takes) gives a graph G_e over {1…n}. With an
// ORIENTATION σᵢ ∈ {+1, −1} per parameter, the measure μᵢ = σᵢ·pᵢ, an arc
// i → j is
//   - STRICT (↓) when μⱼ after e < μᵢ before e, on every execution of e;
//   - WEAK (↓=) when μⱼ after ≤ μᵢ before.
//
// Each label is read from one quantity, the step Δ ∋ aⱼ − pᵢ at e (delta
// against base pᵢ, met with the difference of the two facts there): for
// σ = +1 the arc is strict when Δ.hi ≤ −1 and weak when Δ.hi ≤ 0; for σ = −1,
// strict when Δ.lo ≥ 1 and weak when Δ.lo ≥ 0. Only σᵢ = σⱼ is related.
//
// THEOREM (size-change termination, integer form). Let C be the closure of
// {G_e} under composition, where (G;H)(i,k) is the best arc over j of
// G(i,j) then H(j,k), strict if either is. If every IDEMPOTENT G ∈ C (G;G = G)
// has an arc j → j that is strict and whose measure μⱼ has a lower bound at the
// loop head, the loop terminates.
//
// Proof. An infinite run is an infinite sequence of back edges. By Ramsey's
// theorem it splits, after a prefix, into segments whose composed graphs are
// one idempotent G. G's strict arc j → j says μⱼ falls by at least 1 across
// each segment, so it falls without bound; but μⱼ is bounded below at every
// loop head (the interval domain's post-fixpoint is an invariant there). ∎
//
// The floor is demanded of the WITNESS only: a loop that counts into an
// unbounded accumulator still terminates, since the accumulator is not what
// carries the argument. The closure is finite (3^(n²) graphs at most); it is
// cut at 4096, and a cut closure proves nothing.

type scArc uint8

const (
	scNone scArc = iota
	scWeak
	scStrict
)

type scGraph struct {
	n int
	a []scArc
}

func newSCGraph(n int) scGraph { return scGraph{n: n, a: make([]scArc, n*n)} }

func (g scGraph) at(i, j int) scArc { return g.a[i*g.n+j] }

func (g *scGraph) set(i, j int, v scArc) {
	if v > g.a[i*g.n+j] {
		g.a[i*g.n+j] = v
	}
}

func (g scGraph) key() string {
	var b strings.Builder
	for _, v := range g.a {
		b.WriteByte('0' + byte(v))
	}
	return b.String()
}

func composeSC(g, h scGraph) scGraph {
	out := newSCGraph(g.n)
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			x := g.at(i, j)
			if x == scNone {
				continue
			}
			for k := 0; k < g.n; k++ {
				y := h.at(j, k)
				if y == scNone {
					continue
				}
				v := scWeak
				if x == scStrict || y == scStrict {
					v = scStrict
				}
				out.set(i, k, v)
			}
		}
	}
	return out
}

// sizeChangeTerminates decides the theorem's premise over the back edges'
// graphs, with floored(j) saying μⱼ is bounded below at the loop head.
func sizeChangeTerminates(edges []scGraph, floored func(int) bool) bool {
	if len(edges) == 0 {
		return true // no back edge is taken: the loop runs its body once
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
	for len(work) > 0 {
		if len(seen) >= 4096 {
			return false // a cut closure proves nothing
		}
		g := work[len(work)-1]
		work = work[:len(work)-1]
		for _, e := range edges {
			add(composeSC(g, e))
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g := seen[k]
		if composeSC(g, g).key() != k {
			continue // not idempotent
		}
		ok := false
		for j := 0; j < g.n && !ok; j++ {
			ok = g.at(j, j) == scStrict && floored(j)
		}
		if !ok {
			return false
		}
	}
	return true
}

// terminates is the theorem applied to one loop, after its last evaluation:
// s the loop, cur its parameters' facts at the head, lc its exits' records.
func (a *intervals) terminates(s *Stmt, cur []fact, lc *exitFacts) bool {
	body := s.Sub[0]
	n := len(body.Params)
	var taken []*Region
	for _, cr := range exitRegions(body, TContinue) {
		if _, ok := lc.at[cr]; ok {
			taken = append(taken, cr)
		}
	}
	if len(taken) == 0 {
		return true
	}
	// Each step aⱼ − pᵢ at each taken continue: the symbolic step against
	// base pᵢ, met with the facts' difference there (tripRefine's reading).
	step := func(cr *Region, i, j int) iv {
		rec := lc.at[cr]
		pi := a.pl(body.Params[i])
		undo := a.atContinue(body.Params, rec[1])
		d := a.deltaFrom(cr.Args[j], func(x V) bool { return a.pl(x) == pi }, 4)
		undo()
		return meetIV(d, subIV(rec[0][j].v, rec[1][i].v))
	}
	// ORIENTATION: +1 where a parameter never rises across a back edge, −1
	// where it never falls, and +1 as the guess otherwise (a division's step
	// is read by the descent laws, which ask for +1).
	orient := make([]int, n)
	for j := range orient {
		joined := ivBot
		for _, cr := range taken {
			joined = joinIV(joined, step(cr, j, j))
		}
		switch {
		case !joined.bot && !joined.nlo && joined.lo.sign() >= 0 && (joined.phi || joined.hi.sign() > 0):
			orient[j] = -1
		default:
			orient[j] = +1
		}
	}
	var edges []scGraph
	for _, cr := range taken {
		g := newSCGraph(n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if orient[i] != orient[j] {
					continue
				}
				d := step(cr, i, j)
				if d.bot {
					continue
				}
				if orient[j] > 0 {
					switch {
					case !d.phi && d.hi.lt(ep{}):
						g.set(i, j, scStrict)
					case !d.phi && d.hi.sign() <= 0:
						g.set(i, j, scWeak)
					}
				} else {
					switch {
					case !d.nlo && d.lo.sign() > 0:
						g.set(i, j, scStrict)
					case !d.nlo && d.lo.sign() >= 0:
						g.set(i, j, scWeak)
					}
				}
			}
		}
		edges = append(edges, g)
	}
	floored := func(j int) bool {
		if orient[j] > 0 {
			return !cur[j].v.nlo && !cur[j].v.bot
		}
		return !cur[j].v.phi && !cur[j].v.bot
	}
	return sizeChangeTerminates(edges, floored)
}
