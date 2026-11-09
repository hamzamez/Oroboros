package ir

import "oroboros/core"

// A TABLE BORN FULL NEEDS NO ZERO FILL (ADR 0084's U3, fill-2026-11-09).
//
// A `build` makes the zero table of n elements, which a region realizes by its
// zeroed bytes: region = zero ∘ region_raw. When every element is written
// before any is read, the zero is never observed, and zeroing is a dead store.
// A rule (`tabulate`) and an array literal write every element by
// construction. A `build` is proven by one theorem:
//
// THE WRITTEN PREFIX. Let the build's body run a loop carrying its buffer b
// and a counter i from 0 by +1, writing b[i] on every continuing path without
// reading b, and leaving only by a branch on i ≥ n. Then at each iteration's
// start [0, i) is written (induction: [0, 0) = ∅; an iteration writes i and
// passes i + 1), so at the exit [0, n) is, before any read of b. Reads in the
// exit arm come after. The descending counter, from n − 1 by −1 leaving on
// i < 0, is the same theorem on the suffix (i, n).
//
// Nothing else is proven, and a build the theorem does not cover keeps its
// zeroed region: a fill not proven dead is a cost, never a wrong answer.

// PlantFillDead takes every build as born full: TestAZeroFillIsKeptWhereItIsRead
// must see a table that reads its fill answer wrongly.
var PlantFillDead bool

// fillDead reports a build statement whose zero fill the theorem proves dead.
func fillDead(f *Func, s *Stmt) bool {
	if PlantFillDead {
		return true
	}
	if s.Op != OBuild || len(s.Sub) != 1 || len(s.Args) != 1 || len(s.Sub[0].Params) != 1 {
		return false
	}
	body := s.Sub[0]
	b0 := body.Params[0]
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	konst := func(v V) (int64, bool) {
		if d := def[v]; d != nil && d.Op == OConst && d.Lit != nil && d.Lit.Kind == core.KInt {
			return d.Lit.Int, true
		}
		return 0, false
	}
	// n and m denote the same count: one value, or two equal constants
	same := func(a, b V) bool {
		if a == b {
			return true
		}
		x, ok1 := konst(a)
		y, ok2 := konst(b)
		return ok1 && ok2 && x == y
	}
	n := s.Args[0]
	// the buffer chain: b0, and every `set` result and loop result made from it
	chain := map[V]bool{b0: true}
	reads := func(st *Stmt) bool {
		for _, a := range st.Args {
			if chain[a] && st.Op != OSet && st.Op != OLoop && st.Op != OLen {
				return true
			}
		}
		return false
	}
	// Before the loop: no read of the buffer. The loop must be the body's one
	// statement that carries it.
	for i := range body.Stmts {
		st := &body.Stmts[i]
		if st.Op != OLoop {
			if reads(st) {
				return false
			}
			continue
		}
		k := -1
		for j, a := range st.Args {
			if chain[a] {
				k = j
			}
		}
		if k < 0 {
			continue
		}
		if !loopWritesAll(st, k, n, same, konst, def) {
			return false
		}
		// after the loop every element is written; reads are free
		return true
	}
	return false
}

// loopWritesAll is the written-prefix theorem on loop l, whose k-th parameter
// carries the buffer: some parameter j is a counter from 0 by +1 leaving on
// j ≥ n, or from n − 1 by −1 leaving on j < 0, and every continuing path
// writes the buffer at the counter without reading it.
func loopWritesAll(l *Stmt, k int, n V, same func(a, b V) bool, konst func(V) (int64, bool), def map[V]*Stmt) bool {
	body := l.Sub[0]
	bp := body.Params[k]
	if body.T != TBranch {
		return false
	}
	cond := def[body.Cond]
	if cond == nil || len(cond.Args) != 2 {
		return false
	}
	// the exit: Then breaks on (ge i n) or (lt i 0)
	var ip V = -1
	asc := false
	switch {
	case cond.Op == OGe && same(cond.Args[1], n):
		ip, asc = cond.Args[0], true
	case cond.Op == OLt:
		if z, ok := konst(cond.Args[1]); ok && z == 0 {
			ip = cond.Args[0]
		}
	}
	j := -1
	for x, p := range body.Params {
		if p == ip {
			j = x
		}
	}
	if j < 0 || body.Then.T != TBreak || body.Else.T != TContinue {
		return false
	}
	// the counter starts at 0 ascending, at n − 1 descending
	init := l.Args[j]
	if asc {
		if z, ok := konst(init); !ok || z != 0 {
			return false
		}
	} else {
		d := def[init]
		one := d != nil && d.Op == OSub && len(d.Args) == 2 && same(d.Args[0], n)
		if one {
			c, ok := konst(d.Args[1])
			one = ok && c == 1
		}
		if !one {
			return false
		}
	}
	// the continuing arm: the buffer written at the counter, not read, and the
	// counter stepped by one; a π names the counter within the arm
	arm := body.Else
	alias := map[V]V{}
	for _, pi := range arm.Pis {
		alias[pi.V] = pi.Of
	}
	root := func(v V) V {
		for {
			a, ok := alias[v]
			if !ok {
				return v
			}
			v = a
		}
	}
	chain := map[V]bool{bp: true}
	wrote := false
	for i := range arm.Stmts {
		st := &arm.Stmts[i]
		touches := false
		for _, a := range st.Args {
			touches = touches || chain[a]
		}
		if !touches {
			continue
		}
		switch {
		case st.Op == OSet && chain[st.Args[0]] && !chain[st.Args[2]]:
			if root(st.Args[1]) == ip {
				wrote = true
			}
			chain[st.Res[0]] = true
		case st.Op == OLen:
		default:
			return false // a read of the buffer, or a write we do not model
		}
	}
	if !wrote || len(arm.Args) <= j || !chain[arm.Args[k]] {
		return false
	}
	step := def[arm.Args[j]]
	if step == nil || len(step.Args) != 2 || root(step.Args[0]) != ip {
		return false
	}
	c, ok := konst(step.Args[1])
	if !ok || c != 1 || (asc && step.Op != OAdd) || (!asc && step.Op != OSub) {
		return false
	}
	return true
}
