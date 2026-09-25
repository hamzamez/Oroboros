package emit

import "oroboros/core"

// THE COMPONENT LAW IN THE INTERVAL PASS — ADR 0031 §3, tables.md §2.5.
//
// A tuple's eliminator binds xⱼ to the j-th component of whatever the producer
// yields, and what the pass knows of that component it must know of xⱼ. Two
// sources, one per kind of fact:
//
//   - WHAT THE EXITS SAW. Each tail tuple is evaluated once, where it stands,
//     under the facts its path established, and its components' intervals,
//     cells and lengths are recorded against the rebuilt term. They are joined
//     at every `if` and carried through `let`, `loop`, `build` and a host call's
//     continuation, so the producer's rebuilt term holds the join over its
//     exits: the loop's value, componentwise. An `again` arm contributes
//     nothing, because a jump has no value.
//   - WHAT THE PROJECTION IS. Pⱼ (prodresult.go) is the producer with its tails
//     projected, so ⟦Pⱼ⟧ = πⱼ⟦P⟧, and the queries a `let` asks of its value —
//     its element range, its length — are asked of Pⱼ. They are queries: no
//     operation is counted and no fixpoint is rerun.
//
// Both are sound, so they are met.

// compFact is what the pass knows of one component of a tuple value.
type compFact struct {
	iv            ival
	elem, len     ival
	hasElem, hasL bool
	u64           bool
}

func joinComp(a, b compFact) compFact {
	out := compFact{iv: joinI(a.iv, b.iv), u64: a.u64 && b.u64}
	if a.hasElem && b.hasElem {
		out.elem, out.hasElem = joinI(a.elem, b.elem), true
	}
	if a.hasL && b.hasL {
		out.len, out.hasL = joinI(a.len, b.len), true
	}
	return out
}

func (p *intervalPass) setTupOf(t *core.Term, c []compFact) {
	if t == nil || c == nil {
		return
	}
	if p.tupOf == nil {
		p.tupOf = map[*core.Term][]compFact{}
	}
	p.tupOf[t] = c
}

// carryTup records on `to` what `from` yields, when it yields a tuple.
func (p *intervalPass) carryTup(to, from *core.Term) {
	if c, ok := p.tupOf[from]; ok {
		p.setTupOf(to, c)
	}
}

// joinTup is a conditional's: the componentwise join of the arms that yield a
// value. A jump yields none, so an arm that is one is left out.
func (p *intervalPass) joinTup(to, a, b *core.Term) {
	ca, okA := p.tupOf[a]
	cb, okB := p.tupOf[b]
	switch {
	case okA && okB && len(ca) == len(cb):
		out := make([]compFact, len(ca))
		for j := range ca {
			out[j] = joinComp(ca[j], cb[j])
		}
		p.setTupOf(to, out)
	case okA && p.noValue(b):
		p.setTupOf(to, ca)
	case okB && p.noValue(a):
		p.setTupOf(to, cb)
	}
}

// noValue reports a term every tail of which is a jump: `again` under any chain
// of `if`, `let` and host-call continuations, the four forms of a clause chain.
// It yields no value, so a join leaves it out.
func (p *intervalPass) noValue(t *core.Term) bool {
	if isJumpTerm(t) {
		return true
	}
	if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 {
		return false
	}
	if _, _, k, ok := multiPrimCall(p.tgt, t); ok {
		return p.noValue(k.Closed())
	}
	if t.Kids[0].Kind != core.KName {
		return false
	}
	pr, known := p.tgt.Prims[t.Kids[0].Name]
	switch {
	case known && pr.Kind == "cond" && len(t.Kids) == 4:
		return p.noValue(t.Kids[2]) && p.noValue(t.Kids[3])
	case known && pr.Kind == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn:
		return p.noValue(t.Kids[2].Closed())
	}
	return false
}

// tupleValue evaluates a tuple term, `(fn (#k) (#k c₁ … cₘ))`: each component
// once, as the generic path did, and records what each is.
func (p *intervalPass) tupleValue(t *core.Term) (ival, *core.Term) {
	body := t.Body() // `#k` is the tuple's own binder; no component mentions it
	kids := []*core.Term{body.Kids[0]}
	comps := make([]compFact, len(body.Kids)-1)
	for j, c := range body.Kids[1:] {
		v, nc := p.evalR(c)
		comps[j].iv = v
		comps[j].elem, comps[j].hasElem = p.elemOf(nc)
		comps[j].u64 = p.words && p.u64Term(nc)
		if nc.Kind == core.KName {
			comps[j].len, comps[j].hasL = p.env["len("+nc.Name+")"]
		}
		kids = append(kids, nc)
	}
	rebuilt := core.Fn(t.Params, &core.Term{Kind: core.KApp, Kids: kids})
	p.setTupOf(rebuilt, comps)
	return top, rebuilt
}

// tupleJoin evaluates `(P (fn (x₁ … xₘ) body))` for a producer P that β cannot
// reach (prodresult.go). P is evaluated once, which counts and checks everything
// in it; then each xⱼ is bound as a `let` would bind Pⱼ's value, and the body is
// evaluated. Before this, the whole application was ⊤ and NEITHER side was
// evaluated: every operation in the producer and the body went unchecked.
func (p *intervalPass) tupleJoin(t *core.Term) (ival, *core.Term, bool) {
	prod, k, ok := tupleElim(p.tgt, t)
	if !ok {
		return top, t, false
	}
	m := len(k.Params)
	_, nprod := p.evalR(prod)
	comps := p.tupOf[nprod]
	projs := projections(p.tgt, prod, m)
	body, raw, _ := openFresh(k, p.bound, asmIdent)
	type saved struct {
		v, e, l            ival
		had, hadE, hadL, u bool
		hadU               bool
	}
	old := make([]saved, m)
	for j, x := range raw {
		s := &old[j]
		lk := "len(" + x + ")"
		s.v, s.had = p.env[x]
		s.l, s.hadL = p.env[lk]
		if p.words {
			s.u, s.hadU = p.u64[x]
		}
		var c compFact
		if j < len(comps) {
			c = comps[j]
		} else {
			c.iv = top
		}
		p.env[x] = c.iv
		s.e, s.hadE = p.bindElem(x, projs[j])
		if c.hasElem {
			if have, ok := p.elem[x]; ok {
				p.elem[x] = intersect(have, c.elem)
			} else {
				p.elem[x] = c.elem
			}
		}
		switch L, ok := p.lenOf(projs[j]); {
		case ok && c.hasL:
			p.env[lk] = intersect(L, c.len)
		case ok:
			p.env[lk] = L
		case c.hasL:
			p.env[lk] = c.len
		default:
			delete(p.env, lk)
		}
		if p.words {
			if c.u64 {
				p.u64[x] = true
			} else {
				delete(p.u64, x)
			}
		}
	}
	out, nb := p.evalR(body)
	for j, x := range raw {
		s := old[j]
		restoreVar(p.env, x, s.v, s.had)
		restoreVar(p.env, "len("+x+")", s.l, s.hadL)
		p.unbindElem(x, s.e, s.hadE)
		if p.words {
			if s.hadU {
				p.u64[x] = s.u
			} else {
				delete(p.u64, x)
			}
		}
	}
	p.releaseBound(raw)
	rebuilt := core.App(nprod, core.Fn(raw, nb))
	if e, ok := p.elemOf(nb); ok {
		p.setElemOf(rebuilt, e)
	}
	p.carryTup(rebuilt, nb)
	return out, rebuilt, true
}
