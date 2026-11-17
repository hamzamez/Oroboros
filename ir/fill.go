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
// AND WITH A TAIL (alloc-2026-11-17). The loop may leave on i ≥ k with the
// build's count n = k + c, c a constant, when the exit arm stores b[k + j] for
// every j < c, with values that do not read b, before any read of b. Then the
// written set is W = [0, k) ∪ {k + j : j < c} = [0, k + c) = [0, n): for k ≥ 0
// the prefix and the tail meet, and for k < 0 the loop writes nothing and the
// tail alone covers [k, n) ⊇ [0, n). This is mul_small's shape, whose loop
// writes the digits and whose exit writes the carry and the header. Counts
// and indices are compared as affine forms, base + offset, the base by
// congruence.
//
// Nothing else is proven, and a build the theorem does not cover keeps its
// zeroed region: a fill not proven dead is a cost, never a wrong answer.

// PlantFillTailOne checks only the tail's first store: TestTheWrittenPrefixTheorem
// must see a tail missing a slot taken as born full.
var PlantFillTailOne bool

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
	// n and m denote the same count: one value, two equal constants, or by
	// CONGRUENCE one pure arithmetic operation on the same counts (in SSA an
	// operation is a function, so equal arguments give equal results): the
	// count n + 1 and a guard's own n + 1
	var same func(a, b V) bool
	same = func(a, b V) bool {
		if a == b {
			return true
		}
		x, ok1 := konst(a)
		y, ok2 := konst(b)
		if ok1 || ok2 {
			return ok1 && ok2 && x == y
		}
		da, db := def[a], def[b]
		if da == nil || db == nil || da.Op != db.Op || len(da.Args) != 2 || len(db.Args) != 2 {
			return false
		}
		switch da.Op {
		case OAdd, OSub, OMul:
			return same(da.Args[0], db.Args[0]) && same(da.Args[1], db.Args[1])
		}
		return false
	}
	// affine is v as base + offset, through additions and subtractions of
	// constants: `ma + 1 + 1` is (ma, 2)
	affine := func(v V) (V, int64) {
		var c int64
		for {
			d := def[v]
			if d == nil || len(d.Args) != 2 {
				return v, c
			}
			if x, ok := konst(d.Args[1]); ok && (d.Op == OAdd || d.Op == OSub) {
				if d.Op == OAdd {
					c += x
				} else {
					c -= x
				}
				v = d.Args[0]
				continue
			}
			if x, ok := konst(d.Args[0]); ok && d.Op == OAdd {
				c += x
				v = d.Args[1]
				continue
			}
			return v, c
		}
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
		if !loopWritesAll(st, k, n, same, konst, def, affine) {
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
func loopWritesAll(l *Stmt, k int, n V, same func(a, b V) bool, konst func(V) (int64, bool), def map[V]*Stmt,
	affine func(V) (V, int64)) bool {
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
	tail := int64(0) // c, the stores the exit arm owes past the loop's bound
	switch {
	case cond.Op == OGe && same(cond.Args[1], n):
		ip, asc = cond.Args[0], true
	case cond.Op == OGe:
		bk, ck := affine(cond.Args[1])
		bn, cn := affine(n)
		if same(bk, bn) && cn > ck && cn-ck <= 8 {
			ip, asc, tail = cond.Args[0], true, cn-ck
		}
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
	if j < 0 || body.Then.T != TBreak {
		return false
	}
	if tail > 0 && !exitWritesTail(body.Then, bp, cond.Args[1], tail, same, affine) {
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
	// THE CONTINUING SIDE IS A TERMINATOR TREE, and the induction step holds
	// on each of its paths: every leaf continues (or is unreachable), and on
	// the path to it the buffer is written at the counter, not read, and the
	// counter stepped by one; a π names the counter within the path.
	var path []*Region
	var walk func(r *Region) bool
	walk = func(r *Region) bool {
		path = append(path, r)
		defer func() { path = path[:len(path)-1] }()
		switch r.T {
		case TBranch:
			return walk(r.Then) && walk(r.Else)
		case TUnreachable:
			return true
		case TContinue:
			return pathWritesAt(path, bp, k, j, ip, asc, konst, def)
		}
		return false // a second exit could leave before [0, n) is written
	}
	return walk(body.Else)
}

// pathWritesAt is the induction step on one path through the continuing
// side, ending in a continue: the buffer (parameter bp, the k-th) written at
// the counter ip without being read, and the counter (the j-th) stepped by one.
func pathWritesAt(path []*Region, bp V, k, j int, ip V, asc bool, konst func(V) (int64, bool), def map[V]*Stmt) bool {
	alias := map[V]V{}
	for _, r := range path {
		for _, pi := range r.Pis {
			alias[pi.V] = pi.Of
		}
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
	for _, r := range path {
		for i := range r.Stmts {
			st := &r.Stmts[i]
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
	}
	arm := path[len(path)-1]
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

// exitWritesTail is the tail's half of the theorem: the exit arm r stores the
// buffer (parameter bp) at k + j for every j < c, each value not read from the
// buffer, before any read of it. A π of the arm names a value within it.
func exitWritesTail(r *Region, bp, k V, c int64, same func(a, b V) bool, affine func(V) (V, int64)) bool {
	alias := map[V]V{}
	for _, pi := range r.Pis {
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
	bk, ck := affine(root(k))
	need := map[int64]bool{}
	for j := int64(0); j < c; j++ {
		need[j] = true
		if PlantFillTailOne {
			break
		}
	}
	chain := map[V]bool{bp: true}
	for i := range r.Stmts {
		if len(need) == 0 {
			return true // full; reads of it are free
		}
		st := &r.Stmts[i]
		touches := false
		for _, a := range st.Args {
			touches = touches || chain[a]
		}
		if !touches {
			continue
		}
		switch {
		case st.Op == OSet && chain[st.Args[0]] && !chain[st.Args[2]]:
			bi, ci := affine(root(st.Args[1]))
			if same(root(bi), bk) {
				delete(need, ci-ck)
			}
			chain[st.Res[0]] = true
		case st.Op == OLen:
		default:
			return false // a read of the buffer before the tail is written
		}
	}
	return len(need) == 0
}
