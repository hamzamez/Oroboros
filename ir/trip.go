package ir

import (
	"math/big"

	"oroboros/emit"
)

// TRIP-BOUNDED PARAMETERS (docs/spec/ir.md §7). The interval of a counter a
// loop only ever advances is bounded by how many times the loop goes round,
// which intervals alone cannot see: the tokeniser's token count rises by at
// most one per iteration, and nothing else bounds it.
//
// THEOREM 1. If every execution of a loop takes at most B back edges, and
// every continue passes for parameter p a value whose difference from p lies
// in Δ (the join over the continues), then at every iteration
//
//	p ∈ [z.lo + B·min(Δ.lo, 0),  z.hi + B·max(Δ.hi, 0)]
//
// where z is p's initial interval. Proof: by induction on the number k ≤ B of
// back edges taken, p = z + a sum of k values in Δ, and a sum of k ≤ B values
// in Δ lies in [B·min(Δ.lo,0), B·max(Δ.hi,0)]. ∎
//
// THEOREM 2. B is bounded by any RANKING parameter r:
//   - increasing: Δ_r.lo ≥ 1 and r ≤ H at every continue (its fact there,
//     under the arm's guards) give B ≤ H − z.lo + 1, since before the k-th
//     back edge r ≥ z.lo + k − 1;
//   - decreasing: Δ_r.hi ≤ −1 and r ≥ L at every continue give
//     B ≤ z.hi − L + 1;
//   - geometric: every continue passes r / c for a literal c ≥ 2, and r ≥ 1 at
//     every continue, give B ≤ ⌊log_c z.hi⌋ + 1, since r_k ≤ z.hi / cᵏ and
//     r_k ≥ 1 is needed to go round again. ∎
//
// The step Δ is computed symbolically (delta). This is the IR's form of the
// term analysis's trip counts and loop monotonicity (emit/bound.go,
// emit/monotone.go).

// rankBound is Theorem 2's bound from one parameter, increasing or
// decreasing: z its initial interval, pre its fact at the continues, step its
// Δ. ok is false when the parameter does not rank the loop.
func rankBound(z, pre, step iv) (*big.Int, bool) {
	if z.bot || pre.bot || step.bot {
		return nil, false
	}
	one := big.NewInt(1)
	switch {
	case !step.nlo && step.lo.sign() > 0 && !pre.phi && !z.nlo:
		b := new(big.Int).Sub(pre.hi.big(), z.lo.big())
		return nonNeg(b.Add(b, one)), true
	case !step.phi && step.hi.sign() < 0 && !pre.nlo && !z.phi:
		b := new(big.Int).Sub(z.hi.big(), pre.lo.big())
		return nonNeg(b.Add(b, one)), true
	}
	return nil, false
}

// geoBound is Theorem 2's geometric bound: r divided by c ≥ 2 at every
// continue, r ≥ 1 there, and r starting in z.
func geoBound(z, pre iv, c int64) (*big.Int, bool) {
	if c < 2 || z.bot || pre.bot || pre.nlo || pre.lo.sign() <= 0 || z.phi || z.hi.sign() <= 0 {
		return nil, false
	}
	return big.NewInt(floorLog(z.hi.big(), c) + 1), true
}

func nonNeg(b *big.Int) *big.Int {
	if b.Sign() < 0 {
		return new(big.Int)
	}
	return b
}

// tripInterval is Theorem 1's interval for a parameter starting in z and
// stepping by Δ = step on each of at most b back edges. An end it cannot
// bound, or that leaves E, is infinite.
//
// b = nil is B = ∞, which is still an argument: a side whose step is zero
// (Δ.lo ≥ 0 below, Δ.hi ≤ 0 above) stays at its initial end on every
// iteration, because the induction step p_{k+1} ≥ p_k (or ≤) needs no count.
// It is what bounds a buffer that is only ever given fresh values, in a loop
// no parameter ranks.
func tripInterval(z, step iv, b *big.Int) iv {
	out := iv{nlo: true, phi: true}
	if z.bot || step.bot {
		return out
	}
	if b == nil {
		if !step.nlo && step.lo.sign() >= 0 && !z.nlo {
			out.lo, out.nlo = z.lo, false
		}
		if !step.phi && step.hi.sign() <= 0 && !z.phi {
			out.hi, out.phi = z.hi, false
		}
		return out.norm()
	}
	if !step.nlo && !z.nlo {
		lo := new(big.Int).Mul(b, minE(step.lo, ep{}).big())
		if v, ok := fromBig(lo.Add(lo, z.lo.big())); ok {
			out.lo, out.nlo = v, false
		}
	}
	if !step.phi && !z.phi {
		hi := new(big.Int).Mul(b, maxE(step.hi, ep{}).big())
		if v, ok := fromBig(hi.Add(hi, z.hi.big())); ok {
			out.hi, out.phi = v, false
		}
	}
	return out.norm()
}

// tripRefine meets each parameter's post-fixpoint fact with Theorem 1's
// interval, using the least bound any parameter gives by Theorem 2. It reports
// whether anything tightened.
func (a *intervals) tripRefine(s *Stmt, init, cur []fact, lc *exitFacts) bool {
	body := s.Sub[0]
	conts := exitsOf(body, TContinue)
	if len(conts) == 0 || !lc.hasPre || len(init) != len(body.Params) || len(lc.pre) != len(body.Params) {
		return false
	}
	steps := make([]iv, len(body.Params))
	for j := range steps {
		steps[j] = ivBot
	}
	for _, cr := range exitRegions(body, TContinue) {
		rec, taken := lc.at[cr]
		if !taken {
			continue // a dead arm's continue is never taken: it adds no step
		}
		for j, q := range body.Params {
			// Two sound bounds on v − p at this continue, met: the symbolic
			// step, and the difference of the two facts there, each under
			// this arm's guards. The second is what a reset to a literal has:
			// `(again 0 …)` under v ∈ [1, 9] steps by [−9, −1].
			d := meetIV(a.delta(cr.Args[j], q, 4), subIV(rec[0][j].v, rec[1][j].v))
			steps[j] = joinIV(steps[j], d)
		}
	}
	var best *big.Int
	consider := func(b *big.Int, ok bool) {
		if ok && (best == nil || b.Cmp(best) < 0) {
			best = b
		}
	}
	for j, q := range body.Params {
		consider(rankBound(init[j].v, lc.pre[j].v, steps[j]))
		if c, ok := a.geometric(conts, j, q); ok {
			consider(geoBound(init[j].v, lc.pre[j].v, c))
		}
	}
	// best stays nil when no parameter ranks the loop: B = ∞ (tripInterval).
	refined := false
	for j, q := range body.Params {
		if m := meetIV(cur[j].v, tripInterval(init[j].v, steps[j], best)); m != cur[j].v {
			cur[j].v = m
			refined = true
		}
		if cur[j].hasEl && init[j].hasEl {
			if m := meetIV(cur[j].el, a.elemInterval(conts, j, q, init[j].el, best)); m != cur[j].el {
				cur[j].el = m
				refined = true
			}
		}
	}
	return refined
}

// THEOREM 3 (bounded increments; smashfd-2026-09-16's, on the IR). Let a
// loop take at most B back edges, and let buffer parameter p's cells start in
// z. Suppose every continue passes for p a chain of stores into p itself, and
// each stored value w satisfies, on each side, either
//   - w ≤ Zhi (a FRESH value), or
//   - w − e ≤ Δhi for some read e of p (an INCREMENT of a cell),
//
// and symmetrically below. Then every cell of p lies in
//
//	[min(z.lo, Zlo) + B·min(Δlo, 0),  max(z.hi, Zhi) + B·max(Δhi, 0)].
//
// Proof. Let M_k bound p's cells after k back edges. A read e of p is a cell
// of p at the iteration's start, so e ≤ M_k, and every store of the
// iteration writes at most max(M_k + Δhi, Zhi): a cell after it is untouched
// (≤ M_k) or stored. So M_{k+1} ≤ max(M_k + max(Δhi,0), Zhi), and by
// induction M_k ≤ max(z.hi, Zhi) + k·max(Δhi,0) with k ≤ B. Below is the
// mirror. ∎
//
// The reads are of p, the iteration's start, and never of a buffer the chain
// has already stored into; so the bound is the iteration's MAX increment,
// not a sum of them, which is where it is sharper than the term theorem's
// T·s. A chain that passes anything but stores into p (a host call, an inner
// loop's result) gives no bound.
func (a *intervals) elemInterval(conts [][]V, j int, p V, z iv, b *big.Int) iv {
	pp := a.pl(p)
	readOfP := func(x V) bool {
		d := a.def[x]
		return d != nil && d.Op == OIndex && a.pl(d.Args[0]) == pp
	}
	var ws []V
	for _, c := range conts {
		if !a.storesInto(c[j], pp, 8, &ws) {
			return ivTop
		}
	}
	lo, hi := z, z // the fresh ends, starting from the initial cells
	step := exactIV(0)
	for _, w := range ws {
		f, d := a.fs[w].v, a.deltaFrom(w, readOfP, 4)
		if f.bot {
			continue
		}
		switch {
		case !f.phi:
			hi = joinIV(hi, iv{lo: f.hi, hi: f.hi})
		case !d.phi && !d.bot:
			step.hi = maxE(step.hi, d.hi)
		default:
			return ivTop
		}
		switch {
		case !f.nlo:
			lo = joinIV(lo, iv{lo: f.lo, hi: f.lo})
		case !d.nlo && !d.bot:
			step.lo = minE(step.lo, d.lo)
		default:
			return ivTop
		}
	}
	return tripInterval(iv{lo: lo.lo, nlo: lo.nlo, hi: hi.hi, phi: hi.phi}, step, b)
}

// storesInto walks a continue's argument back to buffer p through stores,
// the arms of an `if` and `restrict`, appending every stored value. It reports
// false for anything else.
func (a *intervals) storesInto(v, pp V, depth int, ws *[]V) bool {
	if a.pl(v) == pp {
		return true
	}
	d := a.def[v]
	if d == nil || depth < 0 {
		return false
	}
	switch d.Op {
	case OSet:
		*ws = append(*ws, d.Args[2])
		return a.storesInto(d.Args[0], pp, depth-1, ws)
	case ORestrict:
		return a.storesInto(d.Args[0], pp, depth-1, ws)
	case OIf:
		k := resIndex(d, v)
		for _, arm := range d.Sub {
			for _, y := range exitsOf(arm, TYield) {
				if k >= len(y) || !a.storesInto(y[k], pp, depth-1, ws) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// delta is an interval containing ⟦v⟧ − ⟦p⟧ wherever v is evaluated, for a
// loop parameter p: Theorem 1's step. It reads v's definition:
//
//	p (or a π of it)       [0, 0]
//	x + y                  Δ(x) + Y,  or Δ(y) + X
//	x − y                  Δ(x) − Y
//	an `if`'s result       the join of the arms' yields' Δ
//	an inner loop's result [Δ(q₀).lo, +∞) when every break yields the inner
//	                       parameter q, q never decreases there, and q
//	                       starts at q₀ (loop monotonicity, emit/monotone.go)
//
// and ⊤ otherwise. Each clause is ℤ's arithmetic on the values, so the result
// contains every difference that occurs.
func (a *intervals) delta(v, p V, depth int) iv {
	pp := a.pl(p)
	return a.deltaFrom(v, func(x V) bool { return a.pl(x) == pp }, depth)
}

// deltaFrom is delta against a BASE: an interval containing ⟦v⟧ − e for some
// value e the predicate accepts, wherever v is evaluated. For a parameter the
// base is the parameter; for a buffer's element (elemRefine) it is any read
// of the buffer.
func (a *intervals) deltaFrom(v V, base func(V) bool, depth int) iv {
	if depth < 0 || v < 0 {
		return ivTop
	}
	// A VALUE NO EXECUTION DEFINES has no differences: its step is ⊥, the
	// identity of every join and minimum below. It is a dead arm's yield or a
	// dead exit's argument (the dead-arm rule, interval.go), and without this
	// it read as ⊤ and spoiled the join it sat in (tree.oro's scanner).
	if int(v) < len(a.fs) && a.fs[v].v.bot {
		return ivBot
	}
	if base(v) {
		return exactIV(0)
	}
	d := a.def[v]
	if d == nil {
		return ivTop
	}
	switch d.Op {
	case OAdd:
		if x := a.deltaFrom(d.Args[0], base, depth-1); x != ivTop {
			return addIV(x, a.fs[d.Args[1]].v)
		}
		if y := a.deltaFrom(d.Args[1], base, depth-1); y != ivTop {
			return addIV(y, a.fs[d.Args[0]].v)
		}
	case OSub:
		if x := a.deltaFrom(d.Args[0], base, depth-1); x != ivTop {
			return subIV(x, a.fs[d.Args[1]].v)
		}
	case OIf:
		k := resIndex(d, v)
		out, has := ivTop, false
		for _, arm := range d.Sub {
			for _, y := range exitsOf(arm, TYield) {
				if k >= len(y) {
					return ivTop
				}
				dy := a.deltaFrom(y[k], base, depth-1)
				if has {
					out = joinIV(out, dy)
				} else {
					out, has = dy, true
				}
			}
		}
		if has {
			return out
		}
	case OLoop:
		// An inner loop's result r, against an inner parameter q that never
		// decreases and starts at q₀: at a break yielding b,
		// r − p = (b − q) + (q − q₀) + (q₀ − p) ≥ Δ(b, q).lo + 0 + Δ(q₀, p).lo.
		// So r − p ≥ min over the breaks of Δ(b, q).lo, plus Δ(q₀, p).lo. The
		// scanner that returns `j + 1` at the closing quote and `j` elsewhere
		// is one; the best q is taken.
		k := resIndex(d, v)
		inner := d.Sub[0]
		breaks := exitsOf(inner, TBreak)
		best, found := ep{}, false
		for qi, q := range inner.Params {
			if qi >= len(d.Args) || len(breaks) == 0 {
				continue
			}
			ok := true
			for _, c := range exitsOf(inner, TContinue) {
				// A ⊥ step is a continue no execution takes: it constrains nothing.
				if st := a.delta(c[qi], q, depth-1); !st.bot && (st.nlo || st.lo.sign() < 0) {
					ok = false
				}
			}
			d0 := a.deltaFrom(d.Args[qi], base, depth-1)
			if !ok || d0.bot || d0.nlo {
				continue
			}
			m, have := ep{}, false
			for _, b := range breaks {
				if k >= len(b) {
					ok = false
					break
				}
				db := a.delta(b[k], q, depth-1)
				if db.bot {
					continue // a break no execution takes: the minimum's identity
				}
				if db.nlo {
					ok = false
					break
				}
				if !have || db.lo.lt(m) {
					m, have = db.lo, true
				}
			}
			if !ok || !have {
				continue
			}
			lo, fit := m.add(d0.lo)
			if fit && (!found || best.lt(lo)) {
				best, found = lo, true
			}
		}
		if found {
			return iv{lo: best, phi: true}
		}
	}
	return ivTop
}

// geometric reports whether every continue passes parameter j divided by a
// literal c ≥ 2, and the least such c.
func (a *intervals) geometric(conts [][]V, j int, p V) (int64, bool) {
	c := int64(0)
	for _, args := range conts {
		d := a.def[args[j]]
		if d == nil || len(d.Args) != 2 || a.pl(d.Args[0]) != a.pl(p) {
			return 0, false
		}
		k, ok := a.constOf(d.Args[1])
		switch {
		case d.Op == ODiv:
		case d.Op == OCall && d.Name == "u64/":
			// ℤ's truncating division on U ⊆ [0, ∞): ⌊r / c⌋, the same law.
		case d.Op == OCall && emit.ArithOp(d.Name, 2) == "shr" && ok && k >= 1 && k <= 62:
			// r >> k is ⌊r / 2ᵏ⌋ where r ≥ 0, which Theorem 2's geometric case
			// already requires (r ≥ 1 at every continue): SelectShifts' form of
			// the same division.
			k = 1 << k
		default:
			return 0, false
		}
		if !ok || k < 2 {
			return 0, false
		}
		if c == 0 || k < c {
			c = k
		}
	}
	return c, c >= 2
}

// floorLog is ⌊log_c n⌋ for n ≥ 1, c ≥ 2, in integers.
func floorLog(n *big.Int, c int64) int64 {
	k, m, bc := int64(0), new(big.Int).Set(n), big.NewInt(c)
	for m.Cmp(bc) >= 0 {
		m.Quo(m, bc)
		k++
	}
	return k
}

func resIndex(s *Stmt, v V) int {
	for i, r := range s.Res {
		if r == v {
			return i
		}
	}
	return 0
}

// exitRegions is the regions of r's branch chain ending in terminator k: the
// continues of a loop body, by region (exitsOf gives their arguments).
func exitRegions(r *Region, k Term) []*Region {
	var out []*Region
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == k {
			out = append(out, r)
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}
