package emit

import "oroboros/core"

// ARRAY SMASHING — the interval layer's consumer of a table's contents.
//
// Blanchet, Cousot, Cousot, Feret, Mauborgne, Miné, Monniaux & Rival, "A static
// analyzer for large safety-critical software" (PLDI 2003) §6.1: an array is
// summarised by ONE abstract cell standing for every element, and a store is a
// WEAK update, joining the stored value into the cell rather than replacing it.
//
// THE DOMAIN. For a table-valued name c, E(c) ∈ Itv with the invariant
//
//	∀s. 0 ≤ s < len c  →  c[s] ∈ γ(E(c))
//
// kept in `p.elem`, which already held exactly that for DECLARED and syntactic
// sources. What this file adds is the transfer function for every term a table
// value can be, so the cell is COMPUTED where it used to be refused.
//
// THE TRANSFER FUNCTIONS, each a theorem about the concrete semantics:
//
//	E(build n λx.e) — inside e, E(x) = [0,0]      the zero fill (tables.md §14.3)
//	E(set c i v)    = E(c) ⊔ ⟦v⟧                   slots ⊆ slots(c) ∪ {v}
//	E(if b t u)     = E(t) ⊔ E(u)                  the value is one of the two
//	E(let v λx.e)   = E(e) with E(x) = E(v)        a binder is its value
//	E(again …)      = ⊥                            a jump has no value
//	E(loop …)       = E(body) at the fixpoint      the loop's value is an exit
//	E(x)            = E(x)                          ⊤ — no record — if nothing binds it
//
// A LOOP'S TABLE VARIABLES JOIN THE KLEENE ITERATION the scalars already run:
// E(v) starts at E(init) and absorbs E(arg) at every back edge, with the same
// widening and the same narrowing, so the result is a post-fixpoint of a
// monotone F and γ of it contains every slot of every version the variable
// takes. A read `(c i)` inside the loop reads the CURRENT ITERATE of that
// fixpoint — which is the induction hypothesis of array-facts.md's Theorem S′,
// and why this is sound where frozen-2026-08-28 had to refuse: that refusal was
// for a SEPARATE analysis consulting its own unsettled answer, not for a read
// inside one monotone fixpoint.
//
// LINEARITY IS WHAT MAKES A NAME'S CELL NEVER STALE. ADR 0018: after
// `(set c i v)` consumes c, c is dead and only `len` may still observe it — so a
// name is read only while its cell describes it, and E(x) = [0,0] for a build
// binder is exact at every read of x.
//
// WHAT IT DELIBERATELY DOES NOT DO: choose storage. A buffer's element width is
// decided by `BufferRange` on a separate sub-pass, and that sub-pass runs with
// smashing off (`noSmash`), so "a buffer may not narrow on its own contents"
// stays the policy it is pinned as. Smashing bounds VALUES — what an operation
// on a read can produce — and changes no representation.

// elemOf is E(t) for a term this pass REBUILT, or nothing when t is not known to
// be a table with a computed cell.
func (p *intervalPass) elemOf(t *core.Term) (ival, bool) {
	if p.noSmash || t == nil {
		return ival{}, false
	}
	if t.Kind == core.KName {
		v, ok := p.elem[t.Name]
		return v, ok
	}
	v, ok := p.tabElem[t]
	return v, ok
}

// setElemOf records E for a rebuilt term. Rebuilt terms are fresh pointers, so
// the key cannot collide with another term's.
func (p *intervalPass) setElemOf(t *core.Term, v ival) {
	if p.noSmash || t == nil {
		return
	}
	if p.tabElem == nil {
		p.tabElem = map[*core.Term]ival{}
	}
	p.tabElem[t] = v
}

// smashEdge joins the cell of back-edge argument k into the loop's accumulator,
// or marks the variable as not a smashed table when the argument has no cell.
func (p *intervalPass) smashEdge(k int, arg *core.Term) {
	if k >= len(p.smashTracked) || !p.smashTracked[k] {
		return
	}
	if e, ok := p.elemOf(arg); ok {
		p.smashAcc[k] = joinI(p.smashAcc[k], e)
		if p.dmode && k < len(p.smashDAcc) {
			if d, ok := p.deltaOf(arg); ok {
				p.smashDAcc[k] = joinI(p.smashDAcc[k], d)
			}
		}
		return
	}
	p.smashOK[k] = false
}

// THE DELTA OF A TABLE TERM, and why a weak update needs one.
//
// E(set c i v) = E(c) ⊔ ⟦v⟧ ⊒ E(c), so the loop transfer on a cell is EXTENSIVE:
// whatever the cell was, the back edge hands back at least that. Widening sends
// a growing cell to +inf and narrowing, which accepts only a value inside the
// current one, can never take it back — the running extremum of monotone.go,
// arrived at a table. A sort adds the second form: it COPIES slots between two
// buffers, so each cell also contains the other's.
//
// THEOREM (copy-closed joint invariant). Let G be a loop's table variables and J
// any sound joint invariant — every slot of every v ∈ G lies in γ(J) at every
// iteration. Let U join, over every back edge: every stored value, except that a
// value which IS a read of a G-table contributes ⊥; and every table handed to a
// G position from outside G. Evaluate U with reads interpreted in J. Then
//
//	J′ = ⨆ init(G) ⊔ U
//
// is a sound joint invariant too.
//
// PROOF, by induction on iterations. At iteration 0 every slot is an initial
// one. At a back edge a new slot is an old slot of some G-table — in J by
// hypothesis and in J′ by the induction — or a copy of a G-slot, the same; or a
// stored value, in U; or a slot of a table from outside G, in U. ∎
//
// So from the widened post-fixpoint E, `E ← E ⊓ J′` is sound and does shrink.
// The DELTA of a term is what it contributes to U: ⊥ for a name in G, the stored
// value for a store, a copy of a G-slot as that slot's delta, and structurally
// through `if`, `let`, λ and a nested loop, whose own table variables get their
// own delta fixpoint (they start from a G-table and add their stores). Anything
// not recorded falls back to its CELL, which contains its delta, so a missing
// record costs precision and never soundness.

// deltaOf is the delta of a rebuilt term while a delta round is running, and its
// cell otherwise.
func (p *intervalPass) deltaOf(t *core.Term) (ival, bool) {
	if !p.dmode || t == nil {
		return p.elemOf(t)
	}
	if t.Kind == core.KName {
		if v, ok := p.dcell[t.Name]; ok {
			return v, true
		}
		if p.dgroup[t.Name] {
			return bottom, true
		}
		return p.elemOf(t)
	}
	if v, ok := p.tabDelta[t]; ok {
		return v, true
	}
	return p.elemOf(t)
}

func (p *intervalPass) setDeltaOf(t *core.Term, v ival) {
	if !p.dmode || t == nil {
		return
	}
	p.tabDelta[t] = v
}

// storedDelta is a stored value's contribution to U: a read of a table whose
// delta is known — a G-table or one derived from it — contributes that delta,
// and anything else its interval.
func (p *intervalPass) storedDelta(val *core.Term, v ival) ival {
	if !p.dmode || val == nil {
		return v
	}
	// A NAME bound to a read is that read.
	if val.Kind == core.KName {
		if r, ok := p.dReadOf[val.Name]; ok {
			val = r
		}
	}
	if val.Kind != core.KApp || val.Op().Kind != core.KName {
		return v
	}
	n := val.Op().Name
	if _, isPrim := p.tgt.Prims[n]; !isPrim && len(val.Kids) == 2 {
		if _, derived := p.dcell[n]; derived || p.dgroup[n] {
			if d, ok := p.deltaOf(val.Op()); ok {
				return d
			}
		}
		return v
	}
	// AN INCREMENT, `(+ R d)` with R a read of a G-derived table and d a literal:
	// its origin is R's, so it contributes R's delta, and the literal is COUNTED
	// for the bounded-increment theorem rather than joined. Inside a nested loop
	// the count per iteration is unbounded, so the round is marked.
	if len(val.Args()) == 2 && arithOp(n, 2) == "add" {
		args := val.Args()
		for i := range args {
			r, lit := args[i], args[1-i]
			if r.Kind == core.KName {
				if rd, ok := p.dReadOf[r.Name]; ok {
					r = rd
				}
			}
			if lit.Kind != core.KInt || r.Kind != core.KApp || len(r.Kids) != 2 || r.Op().Kind != core.KName {
				continue
			}
			if _, isPrim := p.tgt.Prims[r.Op().Name]; isPrim {
				continue
			}
			rn := r.Op().Name
			if _, derived := p.dcell[rn]; !derived && !p.dgroup[rn] {
				continue
			}
			d, ok := p.deltaOf(r.Op())
			if !ok {
				continue
			}
			p.dIncSeen = true
			p.dInc = joinI(p.dInc, exact(lit.Int))
			p.dIncEdge++
			if p.dDepth > 0 {
				p.dIncBad = true
			}
			return d
		}
	}
	return v
}

// shadowDelta binds a name that may SHADOW a G-table — a λ or build parameter
// keeps its source spelling — to a delta, returning what to restore.
func (p *intervalPass) shadowDelta(name string, v ival) (ival, bool, bool) {
	if !p.dmode {
		return ival{}, false, false
	}
	old, had := p.dcell[name]
	p.dcell[name] = v
	return old, had, true
}

func (p *intervalPass) unshadowDelta(name string, old ival, had, did bool) {
	if did {
		restoreVar(p.dcell, name, old, had)
	}
}

// derivedRead reports whether t is a read of a G-derived table — a name in the
// round's group or with a recorded delta, applied to one index.
func (p *intervalPass) derivedRead(t *core.Term) bool {
	if !p.dmode || t == nil || t.Kind != core.KApp || len(t.Kids) != 2 || t.Op().Kind != core.KName {
		return false
	}
	n := t.Op().Name
	if _, isPrim := p.tgt.Prims[n]; isPrim {
		return false
	}
	_, derived := p.dcell[n]
	return derived || p.dgroup[n]
}
