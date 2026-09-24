package emit

import "oroboros/core"

// A BUFFER KEEPS ITS LENGTH. `(set b i v)` hands back b with one cell changed,
// so len(set b i v) = len b: a store acts on a table's cells and never on its
// domain [0, len). Along any chain of stores, then, the length is an invariant,
// and a loop that threads a buffer through its back edges carries its
// initialiser's length on every iteration.
//
// The interval pass knew the first half — `exactLen` follows a `set` chain to
// the `build` that made it — and not the second, so a loop-carried buffer had no
// length at all. emit/winmap.oro is where it cost: the slot count `len/3` was
// not known to be ≥ 1, so the mask `n − 1` might be −1, and `k & −1` is k —
// every probe index unbounded, 52 of 260 operations unproven on windows for a
// map whose buffer has exactly 48 cells (bounds-2026-09-24).
//
// keepsLen is the relation, and it is deliberately syntactic and conservative:
// anything it does not recognise is NOT the same length, which only costs
// precision. It is decided against a set of names already known to share the
// length in question, because a nested loop's variable joins that set exactly
// when its initialiser keeps the length and every back edge does too — the
// Houdini step, a greatest fixpoint over the loop's buffer variables.
//
// It walks the four forms of a clause chain (ADR 0027): `if`, a one-name `let`,
// a host call's continuation, and `again`, which is an exit only when `tail` is
// false. A nested loop's `again` belongs to that loop, and ownAgains keeps the
// two apart.
func (p *intervalPass) keepsLen(t *core.Term, xs map[string]bool, tail bool, depth int) bool {
	if t == nil || depth > 64 {
		return false
	}
	switch t.Kind {
	case core.KName:
		return xs[t.Name]
	case core.KApp:
	default:
		return false
	}
	if v, lam, ok := asLet(p.tgt, t); ok {
		body, raw, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
		inner := withoutNames(xs, raw)
		if p.keepsLen(v, xs, false, depth+1) {
			inner[raw[0]] = true
		}
		return p.keepsLen(body, inner, tail, depth+1)
	}
	if _, _, k, ok := multiPrimCall(p.tgt, t); ok {
		// A host call's results are the host's values: none of them is the buffer.
		body, raw, _ := openFresh(k, map[string]bool{}, func(x string) string { return x })
		return p.keepsLen(body, withoutNames(xs, raw), tail, depth+1)
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	if op.Name == "again" {
		return tail
	}
	pr, known := p.tgt.Prims[op.Name]
	if !known {
		return false
	}
	args := t.Args()
	switch pr.Kind {
	case "table-set":
		return len(args) == 3 && p.keepsLen(args[0], xs, false, depth+1)
	case "cond":
		return len(args) == 3 && p.keepsLen(args[1], xs, tail, depth+1) && p.keepsLen(args[2], xs, tail, depth+1)
	case "iterate":
		return p.loopKeepsLen(t, xs, depth+1)
	}
	return false
}

// loopKeepsLen: a loop's VALUE keeps the length when every exit does, and an
// exit may name any of the loop's variables that keep it — those whose
// initialiser keeps it and whose every back edge hands back one that does.
func (p *intervalPass) loopKeepsLen(t *core.Term, xs map[string]bool, depth int) bool {
	args := t.Args()
	if len(args) < 1 || args[0].Kind != core.KFn {
		return false
	}
	lam, inits := args[0], args[1:]
	body, raw, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
	in := map[int]bool{}
	for k := range raw {
		if k < len(inits) && p.keepsLen(inits[k], xs, false, depth+1) {
			in[k] = true
		}
	}
	outer := withoutNames(xs, raw)
	ys := func() map[string]bool {
		m := withoutNames(outer, nil)
		for k := range in {
			m[raw[k]] = true
		}
		return m
	}
	agains := p.ownAgains(body)
	for changed := true; changed; {
		changed = false
		cur := ys()
		for _, ag := range agains {
			as := ag.Args()
			for k := range in {
				if k >= len(as) || !p.keepsLen(as[k], cur, false, depth+1) {
					delete(in, k)
					changed = true
				}
			}
		}
	}
	return p.keepsLen(body, ys(), true, depth+1)
}

// lenOf is the length of a table term as the environment knows it: exactly, from
// its constructor, or through a store chain from a name whose length is known.
func (p *intervalPass) lenOf(t *core.Term) (ival, bool) {
	if n, ok := exactLen(p.tgt, t); ok {
		return exact(n), true
	}
	for i := 0; i < 8 && t != nil; i++ {
		if t.Kind == core.KName {
			v, ok := p.env["len("+t.Name+")"]
			return v, ok
		}
		if t.Kind != core.KApp || t.Op().Kind != core.KName {
			return ival{}, false
		}
		pr, known := p.tgt.Prims[t.Op().Name]
		if !known || pr.Kind != "table-set" || len(t.Args()) != 3 {
			return ival{}, false
		}
		t = t.Args()[0]
	}
	return ival{}, false
}

// seedLoopLens gives each loop variable that keeps its initialiser's length on
// every back edge that length, for the loop's duration, and returns what to put
// back. Every initialiser is read before any variable is seeded, because a
// variable may keep the spelling of the name it is initialised from.
//
// EVERY OTHER VARIABLE IS SHADOWED. `openFresh` renames a binder only against
// the names it has itself handed out, and a `build`'s buffer is opened by the
// lambda case, which hands out none — so a loop variable can share the buffer's
// spelling while holding a different table, and must not read its length.
func (p *intervalPass) seedLoopLens(body *core.Term, raw []string, inits []*core.Term) func() {
	type seed struct {
		key string
		v   ival
	}
	var seeds []seed
	agains := p.ownAgains(body)
	for k := range raw {
		if k >= len(inits) {
			break
		}
		L, ok := p.lenOf(inits[k])
		if !ok {
			continue
		}
		self := map[string]bool{raw[k]: true}
		keeps := true
		for _, ag := range agains {
			as := ag.Args()
			if k >= len(as) || !p.keepsLen(as[k], self, false, 0) {
				keeps = false
				break
			}
		}
		if keeps {
			seeds = append(seeds, seed{"len(" + raw[k] + ")", L})
		}
	}
	seeded := map[string]ival{}
	for _, s := range seeds {
		seeded[s.key] = s.v
	}
	keys := make([]string, len(raw))
	for k, n := range raw {
		keys[k] = "len(" + n + ")"
	}
	return p.bindLens(keys, seeded)
}

// bindLens sets each key to its seed, or removes it, and returns what puts the
// environment back.
func (p *intervalPass) bindLens(keys []string, seeded map[string]ival) func() {
	type old struct {
		v   ival
		had bool
	}
	olds := make([]old, len(keys))
	for i, k := range keys {
		olds[i].v, olds[i].had = p.env[k]
	}
	for _, k := range keys {
		if v, ok := seeded[k]; ok {
			p.env[k] = v
		} else {
			delete(p.env, k)
		}
	}
	return func() {
		for i := len(keys) - 1; i >= 0; i-- {
			restoreVar(p.env, keys[i], olds[i].v, olds[i].had)
		}
	}
}

// shadowLens removes the lengths of the names a binder introduces, for the
// binder's scope: a host call's results are the host's tables, not the buffer.
func (p *intervalPass) shadowLens(raw []string) func() {
	keys := make([]string, len(raw))
	for k, n := range raw {
		keys[k] = "len(" + n + ")"
	}
	return p.bindLens(keys, nil)
}

func withoutNames(xs map[string]bool, drop []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for k, v := range xs {
		m[k] = v
	}
	for _, d := range drop {
		delete(m, d)
	}
	return m
}
