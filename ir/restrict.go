package ir

import "oroboros/core"

// BOUNDS-CHECK RE-SLICING AS A REWRITE BY LAW L13 (spec §9.4).
//
//	L13:  for i < n ≤ len t,   index (restrict t n) i  =  index t i
//
// A loop whose body compares a parameter p against `len X` bounds p by X's
// length. A table Y the loop reads only at p is replaced, inside the loop, by
// `restrict Y (len X)`: every read is at an index below len X, so L13 makes the
// replacement meaning-preserving — PROVIDED `restrict`'s own obligation,
// len X ≤ len Y, holds at the loop's entry. That premise is what today's Go
// backend never checked (the witness in spec §9.4 panics on a legal program).
//
// Here the premise is discharged by refinements.md §3a's second route, AN
// ASSUMPTION THAT IS THE SAME TERM: an `assume` at the function's entry (an
// export's `where`, ADR 0028) that says len X = len Y, len X ≤ len Y or
// len Y ≥ len X of the same two values. Values are immutable, so a length
// assumed at entry holds at every point. No assumption, no rewrite.
//
// What a host gains is ITS business (ADR 0008): Go's prover sees one bound for
// both tables, which measured 1.96× on compute-bound loops (bce-2026-08-15).

// restrictLoops applies the rewrite everywhere it is justified, and reports
// how many tables it restricted.
func restrictLoops(f *Func) int {
	alias := map[V]V{}
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			alias[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	root := func(v V) V {
		for {
			w, ok := alias[v]
			if !ok {
				return v
			}
			v = w
		}
	}
	le := assumedLengths(f, def, root)
	n := 0
	var walk func(r *Region)
	walk = func(r *Region) {
		for i := 0; i < len(r.Stmts); i++ {
			s := &r.Stmts[i]
			for _, sub := range s.Sub {
				walk(sub)
			}
			if s.Op != OLoop {
				continue
			}
			ins := restrictOne(f, s, le, root)
			if len(ins) > 0 {
				r.Stmts = append(r.Stmts[:i], append(ins, r.Stmts[i:]...)...)
				i += len(ins)
				n += len(ins) - 1 // one `len`, then one `restrict` per table
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
	if n > 0 {
		Canonicalize(f)
	}
	return n
}

// assumedLengths is the set of pairs (X, Y) with len X ≤ len Y assumed at the
// function's entry: the conjuncts of its `assume`s that compare two lengths.
func assumedLengths(f *Func, def map[V]*Stmt, root func(V) V) map[[2]V]bool {
	out := map[[2]V]bool{}
	lenOf := func(v V) (V, bool) {
		if d := def[v]; d != nil && d.Op == OLen {
			return root(d.Args[0]), true
		}
		return 0, false
	}
	var conj func(c V)
	conj = func(c V) {
		d := def[c]
		if d == nil {
			return
		}
		switch d.Op {
		case OIf: // (if a b false) is a ∧ b (L10)
			if y := firstYieldOf(d.Sub[1]); len(y) == 1 && isConst(def, y[0], false) {
				conj(d.Args[0])
				if y0 := firstYieldOf(d.Sub[0]); len(y0) == 1 {
					conj(y0[0])
				}
			}
		case OEq, OLe, OGe:
			a, okA := lenOf(d.Args[0])
			b, okB := lenOf(d.Args[1])
			if !okA || !okB {
				return
			}
			switch d.Op {
			case OEq:
				out[[2]V{a, b}], out[[2]V{b, a}] = true, true
			case OLe:
				out[[2]V{a, b}] = true
			case OGe:
				out[[2]V{b, a}] = true
			}
		}
	}
	for i := range f.Body.Stmts {
		if s := &f.Body.Stmts[i]; s.Op == OAssume {
			conj(s.Args[0])
		}
	}
	return out
}

func firstYieldOf(r *Region) []V {
	switch r.T {
	case TYield:
		return r.Args
	case TBranch:
		if y := firstYieldOf(r.Then); y != nil {
			return y
		}
		return firstYieldOf(r.Else)
	}
	return nil
}

func isConst(def map[V]*Stmt, v V, b bool) bool {
	d := def[v]
	return d != nil && d.Op == OConst && d.Lit.Kind == core.KBool && d.Lit.IsTrue() == b
}

// restrictOne finds, for one loop, the guard `p ? len X` at the top of its
// body and the tables read only at p, and returns the statements to insert
// before the loop: `n = len X` and `Y' = restrict Y n` for each Y whose premise
// is assumed. It rewrites the loop's reads of Y to Y'.
func restrictOne(f *Func, loop *Stmt, le map[[2]V]bool, root func(V) V) []Stmt {
	body := loop.Sub[0]
	inside := map[V]bool{}
	var defs func(r *Region)
	defs = func(r *Region) {
		for _, p := range r.Params {
			inside[p] = true
		}
		for _, pi := range r.Pis {
			inside[pi.V] = true
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				inside[v] = true
			}
			for _, s := range r.Stmts[i].Sub {
				defs(s)
			}
		}
		if r.T == TBranch {
			defs(r.Then)
			defs(r.Else)
		}
	}
	defs(body)
	isParam := map[V]bool{}
	for _, p := range body.Params {
		isParam[p] = true
	}
	// The guard: a comparison, at the body's top level, of a parameter and the
	// length of an invariant table.
	lenOf := map[V]V{}
	param, bound := V(-1), V(-1)
	for i := range body.Stmts {
		s := &body.Stmts[i]
		if s.Op == OLen && !inside[root(s.Args[0])] {
			lenOf[s.Res[0]] = root(s.Args[0])
			continue
		}
		if !s.Op.IsCmp() {
			continue
		}
		if x, ok := lenOf[s.Args[1]]; ok && isParam[root(s.Args[0])] {
			param, bound = root(s.Args[0]), x
			break
		}
		if x, ok := lenOf[s.Args[0]]; ok && isParam[root(s.Args[1])] {
			param, bound = root(s.Args[1]), x
			break
		}
	}
	if param < 0 {
		return nil
	}
	// Candidates: invariant tables read at the parameter and used nowhere else
	// in the loop.
	good, bad := map[V]bool{}, map[V]bool{}
	var scan func(r *Region)
	scan = func(r *Region) {
		for _, pi := range r.Pis {
			bad[root(pi.Of)] = true
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OIndex {
				t := root(s.Args[0])
				if !inside[t] && root(s.Args[1]) == param {
					good[t] = true
				} else {
					bad[t] = true
				}
			} else {
				for _, a := range s.Args {
					bad[root(a)] = true
				}
			}
			for _, sub := range s.Sub {
				scan(sub)
			}
		}
		for _, a := range r.Args {
			bad[root(a)] = true
		}
		if r.T == TBranch {
			scan(r.Then)
			scan(r.Else)
		}
	}
	scan(body)
	var ys []V
	for y := range good {
		if !bad[y] && y != bound && le[[2]V{bound, y}] {
			ys = append(ys, y)
		}
	}
	if len(ys) == 0 {
		return nil
	}
	sortV(ys)
	fresh := func(ty string) V {
		f.Types = append(f.Types, ty)
		return V(len(f.Types) - 1)
	}
	nx := fresh("int")
	ins := []Stmt{{Op: OLen, Args: []V{bound}, Res: []V{nx}}}
	with := map[V]V{}
	for _, y := range ys {
		y2 := fresh(f.Types[y])
		ins = append(ins, Stmt{Op: ORestrict, Args: []V{y, nx}, Res: []V{y2}})
		with[y] = y2
	}
	var rewrite func(r *Region)
	rewrite = func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OIndex {
				if y2, ok := with[root(s.Args[0])]; ok {
					s.Args = []V{y2, s.Args[1]}
				}
			}
			for _, sub := range s.Sub {
				rewrite(sub)
			}
		}
		if r.T == TBranch {
			rewrite(r.Then)
			rewrite(r.Else)
		}
	}
	rewrite(body)
	return ins
}

func sortV(vs []V) {
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0 && vs[j] < vs[j-1]; j-- {
			vs[j], vs[j-1] = vs[j-1], vs[j]
		}
	}
}
