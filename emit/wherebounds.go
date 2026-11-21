package emit

import (
	"math/big"
	"strings"

	"oroboros/core"
)

// A BOUND IS THE BINDER'S SET, WHATEVER ITS SPELLING (ADR 0062 item 2: bounds in
// binder sets, relations in `where`). A host declaration's `where` conjunct
// that bounds one parameter by a constant, lo ≤ x or x ≤ hi, says x ∈ [lo, hi],
// which is the claim `x ∈ lo..hi` states. When both ends are given, the pair is
// that parameter's range obligation (Bounds), decided as a ranged parameter's
// is: a literal, then the IR's intervals, then the refinement layer
// (DischargeRequires). The rest of the `where` stays, checked by the refinement
// layer as before. A one-sided bound stays in the `where`, as a definition's
// infinite end does (installContracts).
//
// The host's TYPE is unchanged, so the hand check still holds each parameter to
// the host's: what moves is how the bound is decided, not what the declaration
// claims. Read as a `where` only, the bound was invisible to the IR's intervals,
// which hold a loop's exit value exactly: `gos.Exit(status)` with status a loop
// variable in {0, 1} was refused (examples/os/wc.oro, wc-2026-10-24).
func (tg *Target) whereBounds() {
	for n, p := range tg.Prims {
		if p.Where == nil || len(p.Names) == 0 {
			continue
		}
		lo := make([]*big.Int, len(p.Names))
		hi := make([]*big.Int, len(p.Names))
		cs := whereConjuncts(p.Where)
		for _, c := range cs {
			i, l, h, ok := constantBound(c, p.Names)
			if !ok {
				continue
			}
			if l != nil && (lo[i] == nil || l.Cmp(lo[i]) > 0) {
				lo[i] = l
			}
			if h != nil && (hi[i] == nil || h.Cmp(hi[i]) < 0) {
				hi[i] = h
			}
		}
		moved := false
		bounds := make([]string, len(p.Args))
		for i := range p.Names {
			if i >= len(p.Args) || lo[i] == nil || hi[i] == nil {
				continue
			}
			l, h := lo[i], hi[i]
			if al, ah, ok := core.IntRangeBig(p.Args[i]); ok { // the declared set's own ends
				if al.Cmp(l) > 0 {
					l = al
				}
				if ah.Cmp(h) < 0 {
					h = ah
				}
			}
			bounds[i] = "int " + l.String() + " " + h.String()
			moved = true
		}
		if !moved {
			continue
		}
		// Keep every conjunct that is not one of the moved pairs.
		var kept []*core.Term
		for _, c := range cs {
			if i, _, _, ok := constantBound(c, p.Names); ok && i < len(bounds) && bounds[i] != "" {
				continue
			}
			kept = append(kept, c)
		}
		p.Bounds = bounds
		p.Where = conjoin(kept)
		tg.Prims[n] = p
	}
	tg.whereUnions()
}

// whereUnions reads a conjunct of a host `where` that is a disjunction of
// constant bounds on one parameter, x ≤ 55295 ∨ x ≥ 57344, as what it states:
// x ∈ R₁ ∪ … ∪ Rₖ, a finite union of ranges, each end a disjunct's or the
// declared set's own. It is that parameter's range obligation, decided as one
// (DischargeRequires): a literal in some member; the IR's interval inside some
// member; else the refinement layer, handed the disjunction itself
// (residualRange), so the route is never weaker than the `where` it replaces.
// It extends ADR 0075 from a range to a union of ranges, and a parameter with a
// range already moved there keeps its range and its `where` (ADR 0084's build:
// `string_of` of a byte read from a file, examples/pe).
func (tg *Target) whereUnions() {
	for n, p := range tg.Prims {
		if p.Where == nil || len(p.Names) == 0 {
			continue
		}
		cs := whereConjuncts(p.Where)
		var kept []*core.Term
		moved := false
		for _, c := range cs {
			closers := append([]string(nil), p.Args...)
			for j := range closers {
				if j < len(p.Bounds) && p.Bounds[j] != "" {
					closers[j] = p.Bounds[j] // the range already moved closes an open end
				}
			}
			i, members, ok := unionBound(c, p.Names, closers)
			if ok && i < len(p.Bounds) && p.Bounds[i] != "" {
				// A RANGE ALREADY MOVED IS CONJOINED: [l, h] ∩ (R₁ ∪ … ∪ Rₖ) is
				// (R₁ ∩ [l, h]) ∪ … ∪ (Rₖ ∩ [l, h]), conjunction distributing
				// over the union. A member it empties is dropped; one moved
				// union is not intersected with another (it stays a `where`).
				members, ok = intersectUnion(members, p.Bounds[i])
			}
			if !ok {
				kept = append(kept, c)
				continue
			}
			if len(p.Bounds) < len(p.Args) {
				b := make([]string, len(p.Args))
				copy(b, p.Bounds)
				p.Bounds = b
			}
			p.Bounds[i] = strings.Join(members, "|")
			if p.Unions == nil {
				p.Unions = map[int]*core.Term{}
			}
			p.Unions[i] = c
			moved = true
		}
		if moved {
			p.Where = conjoin(kept)
			tg.Prims[n] = p
		}
	}
}

// unionBound reads c as a disjunction of constant bounds on one parameter, each
// disjunct one bound or a conjunction of two, and gives its members as ranges,
// an open end closed by the parameter's enclosing set: its range already moved
// out of the `where`, or its declared set. A disjunct that is no
// such bound, two parameters, or an end neither gives makes it no union.
func unionBound(c *core.Term, names, args []string) (int, []string, bool) {
	var disjuncts []*core.Term
	var flat func(*core.Term)
	flat = func(t *core.Term) {
		if a, b, ok := erasedOr(t); ok {
			flat(a)
			flat(b)
			return
		}
		disjuncts = append(disjuncts, t)
	}
	flat(c)
	if len(disjuncts) < 2 {
		return 0, nil, false
	}
	at := -1
	var members []string
	for _, d := range disjuncts {
		var lo, hi *big.Int
		for _, b := range whereConjuncts(d) {
			i, l, h, ok := constantBound(b, names)
			if !ok || (at >= 0 && i != at) {
				return 0, nil, false
			}
			at = i
			if l != nil {
				lo = l
			}
			if h != nil {
				hi = h
			}
		}
		if at < 0 || at >= len(args) {
			return 0, nil, false
		}
		dl, dh, ok := core.IntRangeBig(args[at])
		if lo == nil || (ok && dl.Cmp(lo) > 0) {
			lo = dl
		}
		if hi == nil || (ok && dh.Cmp(hi) < 0) {
			hi = dh
		}
		if lo == nil || hi == nil {
			return 0, nil, false
		}
		members = append(members, "int "+lo.String()+" "+hi.String())
	}
	return at, members, true
}

// whereConjuncts flattens a conjunction, (if a b false), into its conjuncts.
func whereConjuncts(t *core.Term) []*core.Term {
	if a, b, ok := erasedAnd(t); ok {
		return append(whereConjuncts(a), whereConjuncts(b)...)
	}
	return []*core.Term{t}
}

// conjoin is the conjunction of ts, nil when there is none.
func conjoin(ts []*core.Term) *core.Term {
	var out *core.Term
	for i := len(ts) - 1; i >= 0; i-- {
		if out == nil {
			out = ts[i]
		} else {
			out = core.App(core.Name("if"), ts[i], out, core.Bool(false))
		}
	}
	return out
}

// constantBound reads c as a bound on parameter names[i] by an integer literal:
// lo ≤ x, x ≥ lo, lo < x, x > lo (a lower end) or x ≤ hi, hi ≥ x, x < hi,
// hi > x (an upper end).
func constantBound(c *core.Term, names []string) (i int, lo, hi *big.Int, ok bool) {
	if c == nil || c.Kind != core.KApp || len(c.Kids) != 3 || c.Kids[0].Kind != core.KName {
		return 0, nil, nil, false
	}
	op, a, b := c.Kids[0].Name, c.Kids[1], c.Kids[2]
	param := func(t *core.Term) int {
		if t.Kind != core.KName {
			return -1
		}
		for j, n := range names {
			if n == t.Name {
				return j
			}
		}
		return -1
	}
	lit := func(t *core.Term) (*big.Int, bool) { return closedInt(t) }
	one := big.NewInt(1)
	if j := param(a); j >= 0 {
		if k, isLit := lit(b); isLit {
			switch {
			case isOp(op, "le"):
				return j, nil, k, true
			case isOp(op, "lt"):
				return j, nil, new(big.Int).Sub(k, one), true
			case isOp(op, "ge"):
				return j, k, nil, true
			case isOp(op, "gt"):
				return j, new(big.Int).Add(k, one), nil, true
			}
		}
	}
	if j := param(b); j >= 0 {
		if k, isLit := lit(a); isLit {
			switch {
			case isOp(op, "le"): // k ≤ x
				return j, k, nil, true
			case isOp(op, "lt"):
				return j, new(big.Int).Add(k, one), nil, true
			case isOp(op, "ge"): // k ≥ x
				return j, nil, k, true
			case isOp(op, "gt"):
				return j, nil, new(big.Int).Sub(k, one), true
			}
		}
	}
	return 0, nil, nil, false
}

// intersectUnion is (R₁ ∪ … ∪ Rₖ) ∩ r for r one range: each member cut to r,
// an empty one dropped. r a union itself is not intersected (ok false).
func intersectUnion(members []string, r string) ([]string, bool) {
	l, h, ok := core.IntRangeBig(r)
	if !ok || strings.Contains(r, "|") {
		return nil, false
	}
	var out []string
	for _, m := range members {
		ml, mh, ok := core.IntRangeBig(m)
		if !ok {
			return nil, false
		}
		if l.Cmp(ml) > 0 {
			ml = l
		}
		if h.Cmp(mh) < 0 {
			mh = h
		}
		if ml.Cmp(mh) <= 0 {
			out = append(out, "int "+ml.String()+" "+mh.String())
		}
	}
	return out, len(out) > 0
}

// EntrySig is an entry point's header as the host's interface: each parameter
// declared `Int`, ℤ, takes the range its `where`'s constant bounds give it (ADR
// 0075), the word's representation where they bound it inside the word, and
// otherwise an end left open, so the host hands over its integer of any size
// (integers-boundary §2). Any other header is returned as it is.
func EntrySig(tg *Target, sig *core.Sig) *core.Sig {
	if sig == nil || !sig.Entry || PlantEntryIntIsTheWord {
		return sig
	}
	names := make([]string, len(sig.Params))
	for i, p := range sig.Params {
		names[i] = p.Name
	}
	lo := make([]*big.Int, len(sig.Params))
	hi := make([]*big.Int, len(sig.Params))
	type diff struct {
		x, y int
		c    *big.Int // x ≤ y + c
	}
	var diffs []diff
	if sig.Where != nil {
		for _, c := range whereConjuncts(sig.Where) {
			i, l, h, ok := constantBound(c, names)
			if !ok {
				i, h, ok = lenBound(tg, c, names)
			}
			if ok {
				if l != nil && (lo[i] == nil || l.Cmp(lo[i]) > 0) {
					lo[i] = l
				}
				if h != nil && (hi[i] == nil || h.Cmp(hi[i]) < 0) {
					hi[i] = h
				}
				continue
			}
			if x, y, k, ok := paramDiff(c, names); ok && !PlantNoDiffBound {
				diffs = append(diffs, diff{x, y, k})
			}
		}
	}
	// THE DIFFERENCE BOUNDS' CLOSURE: x ≤ y + c carries y's upper end to x and
	// x's lower end to y, the two-variable fragment of Fourier–Motzkin, and a
	// chain of n parameters settles in n rounds (Bellman–Ford). `chain(i, n, t)
	// where 0 <= i && i < n && n <= len(t)` bounds i by len(t) through n
	// (book chapter 16), as the index t[i] needs.
	for round := 0; round <= len(sig.Params); round++ {
		changed := false
		for _, d := range diffs {
			if hi[d.y] != nil {
				h := new(big.Int).Add(hi[d.y], d.c)
				if hi[d.x] == nil || h.Cmp(hi[d.x]) < 0 {
					hi[d.x], changed = h, true
				}
			}
			if lo[d.x] != nil {
				l := new(big.Int).Sub(lo[d.x], d.c)
				if lo[d.y] == nil || l.Cmp(lo[d.y]) > 0 {
					lo[d.y], changed = l, true
				}
			}
		}
		if !changed {
			break
		}
	}
	out := *sig
	out.Params = append([]core.SigParam(nil), sig.Params...)
	wlo, whi := big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi)
	for i, p := range out.Params {
		if p.Type != "int" {
			continue
		}
		if lo[i] != nil && hi[i] != nil && lo[i].Cmp(wlo) >= 0 && hi[i].Cmp(whi) <= 0 {
			continue // inside the word: the word, its bounds the where's
		}
		l, h := "-inf", "+inf"
		if lo[i] != nil {
			l = lo[i].String()
		}
		if hi[i] != nil {
			h = hi[i].String()
		}
		out.Params[i].Type = "int " + l + " " + h
	}
	return &out
}

// lenBound reads c as an upper bound on parameter names[i] by a table's
// length, x < len(t), x <= len(t), len(t) > x or len(t) >= x: a length is at
// most the target's largest table, MaxLenOf (tables.md §2.3), so the bound is
// that constant's, minus one where strict. `promised(t, i ∈ Int) where 0 <= i
// && i < len(t)` takes i in the word by it (book chapter 13), as its read t[i]
// needs: an index is a word position.
func lenBound(tg *Target, c *core.Term, names []string) (int, *big.Int, bool) {
	if c == nil || c.Kind != core.KApp || len(c.Kids) != 3 || c.Kids[0].Kind != core.KName || PlantNoLenBound {
		return 0, nil, false
	}
	op, a, b := c.Kids[0].Name, c.Kids[1], c.Kids[2]
	param := func(t *core.Term) int {
		if t.Kind != core.KName {
			return -1
		}
		for j, n := range names {
			if n == t.Name {
				return j
			}
		}
		return -1
	}
	isLen := func(t *core.Term) bool {
		return t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName &&
			isOp(t.Kids[0].Name, "alen") && param(t.Kids[1]) >= 0
	}
	max := big.NewInt(tg.MaxLenOf())
	one := big.NewInt(1)
	if j := param(a); j >= 0 && isLen(b) {
		switch {
		case isOp(op, "lt"):
			return j, new(big.Int).Sub(max, one), true
		case isOp(op, "le"):
			return j, max, true
		}
	}
	if j := param(b); j >= 0 && isLen(a) {
		switch {
		case isOp(op, "gt"):
			return j, new(big.Int).Sub(max, one), true
		case isOp(op, "ge"):
			return j, max, true
		}
	}
	return 0, nil, false
}

// paramDiff reads c as x ≤ y + k between two parameters, k ∈ {0, −1}: x <= y,
// x < y (x ≤ y − 1), y >= x, y > x.
func paramDiff(c *core.Term, names []string) (int, int, *big.Int, bool) {
	if c == nil || c.Kind != core.KApp || len(c.Kids) != 3 || c.Kids[0].Kind != core.KName {
		return 0, 0, nil, false
	}
	op, a, b := c.Kids[0].Name, c.Kids[1], c.Kids[2]
	param := func(t *core.Term) int {
		if t.Kind != core.KName {
			return -1
		}
		for j, n := range names {
			if n == t.Name {
				return j
			}
		}
		return -1
	}
	i, j := param(a), param(b)
	if i < 0 || j < 0 {
		return 0, 0, nil, false
	}
	switch {
	case isOp(op, "le"):
		return i, j, big.NewInt(0), true
	case isOp(op, "lt"):
		return i, j, big.NewInt(-1), true
	case isOp(op, "ge"):
		return j, i, big.NewInt(0), true
	case isOp(op, "gt"):
		return j, i, big.NewInt(-1), true
	}
	return 0, 0, nil, false
}

// PlantNoDiffBound reads no bound between two parameters at an entry point:
// chapter 16's chain is then refused at its index (TestATablesElementIsAWord).
var PlantNoDiffBound bool

// PlantNoLenBound reads no bound by a table's length at an entry point:
// chapter 13's `promised` is then refused at its index again
// (TestIntIsTheIntegersAtTheBoundary).
var PlantNoLenBound bool

// PlantEntryIntIsTheWord reads an entry point's `Int` as the host's word, as
// before integers-boundary: `square(n ∈ Int) (Int)` is then refused again
// (TestIntIsTheIntegersAtTheBoundary).
var PlantEntryIntIsTheWord bool
