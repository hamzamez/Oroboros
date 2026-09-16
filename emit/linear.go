package emit

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"oroboros/core"
)

// The decidable fragment of docs/spec/refinements.md §4: linear integer
// arithmetic over difference constraints, which is what every bounds obligation
// actually is.
//
// Deliberately incomplete OVER THE INTEGERS, and not an SMT solver. `entails`
// tries the one- and two-fact certificates first and then `farkas`, which is
// Fourier–Motzkin elimination on the goal's negation: complete for every
// entailment a rational certificate proves, and sound over ℤ because a system with
// no rational solution has no integer one (split-2026-09-16's successor, loopsum).
// What it misses is what only integrality proves, and being incomplete is safe
// because an undischarged obligation is REPORTED rather than assumed.

// linear is a linear expression: coefficients over opaque variables, plus a
// constant. A variable is a parameter name, or a length term like "alen(a)"
// which is opaque and never inspected.
type linear struct {
	coef  map[string]int64
	konst int64
}

func constant(k int64) *linear { return &linear{coef: map[string]int64{}, konst: k} }

func variable(name string) *linear {
	return &linear{coef: map[string]int64{name: 1}}
}

func (l *linear) clone() *linear {
	c := &linear{coef: make(map[string]int64, len(l.coef)), konst: l.konst}
	for k, v := range l.coef {
		c.coef[k] = v
	}
	return c
}

func (l *linear) addScaled(o *linear, s int64) *linear {
	out := l.clone()
	for k, v := range o.coef {
		out.coef[k] += v * s
		if out.coef[k] == 0 {
			delete(out.coef, k)
		}
	}
	out.konst += o.konst * s
	return out
}

// sameVars reports whether two expressions differ only in their constant.
func sameVars(a, b *linear) bool {
	if len(a.coef) != len(b.coef) {
		return false
	}
	for k, v := range a.coef {
		if b.coef[k] != v {
			return false
		}
	}
	return true
}

func (l *linear) String() string {
	parts := make([]string, 0, len(l.coef)+1)
	names := make([]string, 0, len(l.coef))
	for k := range l.coef {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		switch l.coef[k] {
		case 1:
			parts = append(parts, k)
		case -1:
			parts = append(parts, "-"+k)
		default:
			parts = append(parts, fmt.Sprintf("%d*%s", l.coef[k], k))
		}
	}
	if l.konst != 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%+d", l.konst))
	}
	return strings.Join(parts, " + ")
}

// facts is a conjunction of `e <= 0`, plus equalities used for substitution.
type facts struct {
	le  []*linear
	eq  map[string]*linear // name -> the linear expression it equals
	log []string           // human-readable, for the diagnostic

	// opaque are assumptions OUTSIDE the fragment, kept as printed terms and
	// matched by syntactic identity — refinements.md §3's "propagated and
	// matched by name, never decided". They were being dropped, so a `where`
	// the solver could not read left the diagnostic saying `known: nothing`
	// while the program declared something.
	opaque []string

	// pure is the atom signature this fact set reasons in (atoms).
	pure atoms

	// content is F-D₁: for a table name, the φ over `#e` that hold of every slot
	// (emit/content.go). zero marks a `build` binder still holding its zero fill
	// until a loop threading it says otherwise.
	content map[string][]*core.Term
	zero    map[string]bool

	// sub caches le rewritten by eq, which entails needs on every call: sub[i] is
	// substitute(le[i]) under the equations as they stood at version subEq. An
	// equation invalidates it (eqVer moves); a new inequality only extends it.
	sub          []*linear
	eqVer, subEq int
}

func newFacts() *facts {
	return &facts{eq: map[string]*linear{}, content: map[string][]*core.Term{}, zero: map[string]bool{}}
}

// assumeOpaque records an assumption the fragment cannot decide.
func (f *facts) assumeOpaque(printed string) {
	for _, o := range f.opaque {
		if o == printed {
			return
		}
	}
	f.opaque = append(f.opaque, printed)
	f.log = append(f.log, "assumed "+printed)
}

// entailsOpaque discharges an obligation the fragment cannot decide, and only
// by an assumption that is syntactically the same term.
func (f *facts) entailsOpaque(printed string) bool {
	for _, o := range f.opaque {
		if o == printed {
			return true
		}
	}
	return false
}

func (f *facts) clone() *facts {
	c := &facts{le: append([]*linear(nil), f.le...), eq: make(map[string]*linear, len(f.eq)),
		log: append([]string(nil), f.log...), opaque: append([]string(nil), f.opaque...), pure: f.pure,
		content: make(map[string][]*core.Term, len(f.content)), zero: make(map[string]bool, len(f.zero)),
		sub: f.sub[:len(f.sub):len(f.sub)], eqVer: f.eqVer, subEq: f.subEq}
	for k, v := range f.eq {
		c.eq[k] = v
	}
	for k, v := range f.content {
		c.content[k] = v
	}
	for k, v := range f.zero {
		c.zero[k] = v
	}
	return c
}

// fingerprint is a deterministic rendering of everything a fact set assumes:
// inequalities, equations, opaque atoms and content facts. Two fact sets with one
// fingerprint entail the same things, which is what makes it a sound cache key.
func (f *facts) fingerprint() string {
	var b strings.Builder
	for _, l := range f.le {
		b.WriteString(l.String())
		b.WriteByte(';')
	}
	names := make([]string, 0, len(f.eq))
	for n := range f.eq {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b.WriteString(n + "=" + f.eq[n].String() + ";")
	}
	for _, o := range f.opaque {
		b.WriteString(o + ";")
	}
	b.WriteString(f.contentString())
	zs := make([]string, 0, len(f.zero))
	for n := range f.zero {
		zs = append(zs, n)
	}
	sort.Strings(zs)
	b.WriteString(strings.Join(zs, ","))
	return b.String()
}

// assumeLE records `e <= 0`.
func (f *facts) assumeLE(e *linear, why string) {
	f.le = append(f.le, f.substitute(e))
	f.log = append(f.log, why)
}

// assumeEQ records `name = e`, which is what makes a let-bound loop count
// usable: the stencil's `n1 = alen a - 2` is the fact that discharges
// `i + 2 < alen a`.
func (f *facts) assumeEQ(name string, e *linear) {
	f.eq[name] = f.substitute(e)
	f.eqVer++
}

// substitute replaces known equalities, repeatedly, so a chain of lets resolves.
func (f *facts) substitute(e *linear) *linear {
	// Nothing to rewrite is the common case, and deciding it walks e's handful of
	// variables rather than every equation in scope eight times over.
	if !f.rewrites(e) {
		return e
	}
	out := e
	for pass := 0; pass < 8; pass++ {
		changed := false
		for name, def := range f.eq {
			c, ok := out.coef[name]
			if !ok || c == 0 {
				continue
			}
			out = out.clone()
			delete(out.coef, name)
			out = out.addScaled(def, c)
			changed = true
		}
		if !changed {
			break
		}
	}
	return out
}

// substituted is le rewritten by the equations now in scope, from the cache where
// it is still valid. The slice is shared and must not be modified.
func (f *facts) substituted() []*linear {
	if f.subEq != f.eqVer {
		f.sub, f.subEq = nil, f.eqVer
	}
	for i := len(f.sub); i < len(f.le); i++ {
		f.sub = append(f.sub, f.substitute(f.le[i]))
	}
	return f.sub
}

// rewrites reports whether some equation in scope names a variable of e — exactly
// when substitute's first pass would change anything.
func (f *facts) rewrites(e *linear) bool {
	if len(f.eq) == 0 {
		return false
	}
	for name := range e.coef {
		if _, ok := f.eq[name]; ok {
			return true
		}
	}
	return false
}

// combMatch decides whether ma·a + mb·b implies the goal g, without building the
// sum: with scale false, that the sum has g's variable part and a constant at
// least g's (sameVars); with scale true, that g's variable part is q times the
// sum's for one q in (0, 2^20] and q times the sum's constant is at least g's
// (scaleTo). Coefficients are never stored as zero — every constructor drops
// them — so a zero entry of the sum is simply absent, as addScaled leaves it.
func combMatch(a *linear, ma int64, b *linear, mb int64, g *linear, scale bool) bool {
	n := 0
	var q int64
	check := func(k string, s int64) bool {
		if s == 0 {
			return true
		}
		n++
		d, ok := g.coef[k]
		if !scale {
			return ok && d == s
		}
		if !ok || d%s != 0 {
			return false
		}
		qq := d / s
		if qq <= 0 || qq > 1<<20 {
			return false
		}
		if q == 0 {
			q = qq
		}
		return q == qq
	}
	for k, av := range a.coef {
		if !check(k, ma*av+mb*b.coef[k]) {
			return false
		}
	}
	for k, bv := range b.coef {
		if _, in := a.coef[k]; in {
			continue
		}
		if !check(k, mb*bv) {
			return false
		}
	}
	if n == 0 || n != len(g.coef) {
		return false
	}
	if !scale {
		q = 1
	}
	return (ma*a.konst+mb*b.konst)*q >= g.konst
}

// entails reports whether the facts prove `goal <= 0`.
//
// The method is single-fact implication after substitution: if a known fact has
// the same variable part and a constant at least as tight, the goal follows.
// That decides every obligation the language currently generates, including the
// stencil's `i + 2 < alen a` from `i < alen a - 2`, and it fails honestly on
// anything needing two facts combined.
// entailsDisjunction proves a goal of the form `A ∨ B` by proving either side.
//
// A DISEQUALITY is the only one we need and the only one that shows up:
// `d ≠ 0` is `d < 0 ∨ d > 0`, and the fragment is conjunctions of linear
// inequalities, so it cannot hold the goal at all. It can hold each DISJUNCT,
// which is a case split — cheap, complete for this shape, and needing no new
// decision procedure (integers.md §5, types-direction.md §6.7).
func (f *facts) entailsEither(a, b *linear) bool { return f.entails(a) || f.entails(b) }

// neKey is a canonical rendering of a disequality, so that a fact and an
// obligation match whatever spelling each was written in.
//
// `(go.ne b 0)` — which is what `negate` produces from a guard — and `(!= b 0)`
// — which is what a target file declares — are the same statement, and matching
// them by printed term did not work. Both sides are normalised through the
// linear form, and the two operands are sorted, because `a ≠ b` and `b ≠ a` are
// one fact.
func neKey(t *core.Term) (string, bool) {
	l, r, ok := disequalityParts(t)
	if !ok {
		return "", false
	}
	a, b := l.String(), r.String()
	if a > b {
		a, b = b, a
	}
	return "ne(" + a + "," + b + ")", true
}

func disequalityParts(t *core.Term) (*linear, *linear, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return nil, nil, false
	}
	if !isOp(t.Op().Name, "ne") {
		return nil, nil, false
	}
	l, ok1 := asLinear(t.Args()[0])
	r, ok2 := asLinear(t.Args()[1])
	if !ok1 || !ok2 {
		return nil, nil, false
	}
	return l, r, true
}

// disequality turns `a != b` into the two goals whose disjunction it is.
// Integers, so `a < b` is `a - b + 1 <= 0` and `a > b` is `b - a + 1 <= 0`.
func disequality(t *core.Term) (lo, hi *linear, ok bool) {
	l, r, ok := disequalityParts(t)
	if !ok {
		return nil, nil, false
	}
	return l.clone().addScaled(r, -1).addScaled(constant(-1), -1),
		r.clone().addScaled(l, -1).addScaled(constant(-1), -1), true
}

// scaleTo returns the positive integer m for which `fact` scaled by m has
// exactly `g`'s coefficients, or reports that no such m exists.
//
// This is one Farkas multiplier, and it is the smallest step that turns
// coefficient-matching into something worth calling a decision procedure.
// Without it `sp >= 1` cannot discharge `0 <= 2*sp - 1`, because the goal's
// coefficient is -2 and the fact's is -1 — a consequence so immediate that the
// gap looked like a missing fact rather than a missing inference.
//
// Found by examples/json/tree.oro, whose addressing is `(go.* 4 k)` and
// `(go.* 2 d)`: EVERY index into a strided table has a coefficient the guard
// that bounds it does not, so a stride is exactly the shape this misses. The
// program was clamping instead, at a measured 1.35x
// (json-tree-bench-2026-08-26).
//
// m is capped because a Farkas multiplier that large means the goal is not the
// kind of consequence this procedure is for, and konst*m must not overflow.
func scaleTo(fact, g *linear) (int64, bool) {
	if len(fact.coef) == 0 || len(fact.coef) != len(g.coef) {
		return 0, false
	}
	var m int64
	for k, c := range fact.coef {
		d, ok := g.coef[k]
		if !ok || c == 0 || d%c != 0 {
			return 0, false
		}
		q := d / c
		if q <= 0 || q > 1<<20 {
			return 0, false
		}
		if m == 0 {
			m = q
		} else if m != q {
			return 0, false
		}
	}
	return m, m > 0
}

func (f *facts) entails(goal *linear) bool {
	g := f.substitute(goal)
	if len(g.coef) == 0 {
		return g.konst <= 0
	}
	// THE FACTS ARE REWRITTEN BY THE SAME EQUATIONS AS THE GOAL, and at the same
	// moment. A fact is substituted when it is assumed, and an equation can
	// arrive later — the division axioms are seeded at the root, before any
	// `let` — so a fact kept `len(enc)` while the goal, rewritten now, said
	// `2·len(a)`, and they stopped matching (hex-2026-09-14 §3). Equality is a
	// congruence, so rewriting both sides by one set of equations is sound.
	le := f.substituted()
	// A single fact, after substitution.
	for _, fact := range le {
		// `fact` says L + a <= 0, i.e. L <= -a. `g` says L + b <= 0, i.e.
		// L <= -b. The fact implies the goal when -a <= -b, i.e. a >= b.
		// Getting this backwards made the stencil's j+1 < alen(a) unprovable
		// from j < alen(a)-2, which is strictly stronger.
		if sameVars(fact, g) && fact.konst >= g.konst {
			return true
		}
		// The same fact SCALED. `m*L + m*a <= 0` implies `L' + b <= 0` whenever
		// m*L is exactly L' and m*a >= b — one Farkas multiplier, and what a
		// strided index needs.
		if m, ok := scaleTo(fact, g); ok && fact.konst*m >= g.konst {
			return true
		}
	}
	// Or the sum of two. `i < alen p` plus `alen p <= alen q` gives
	// `i < alen q`, which is the shape a two-array loop always produces and
	// which one fact can never reach. Cheap: the fact set is tiny.
	// A PAIR CAN ONLY SUM TO THE GOAL'S VARIABLES if every variable of each that the
	// goal lacks is cancelled, so occurs in the other. That is decided on a short
	// list per fact before any combination is formed, and rejects almost every
	// pair of a large fact set without iterating a map.
	out := make([][]string, len(le))
	for i, fact := range le {
		for v := range fact.coef {
			if _, in := g.coef[v]; !in {
				out[i] = append(out[i], v)
			}
		}
	}
	covers := func(i, j int) bool {
		for _, v := range out[i] {
			if _, in := le[j].coef[v]; !in {
				return false
			}
		}
		return true
	}
	for i, a := range le {
		for j := i + 1; j < len(le); j++ {
			b := le[j]
			if !covers(i, j) || !covers(j, i) {
				continue
			}
			if combMatch(a, 1, b, 1, g, false) {
				return true
			}
		}
	}
	// AND THE SUM OF TWO WITH MULTIPLIERS THAT CANCEL A VARIABLE — one
	// Fourier–Motzkin elimination step, which is the principled form of the
	// single Farkas multiplier above rather than another special case.
	//
	// For `a <= 0` and `b <= 0` sharing a variable v with opposite signs,
	// `|b_v|·a + |a_v|·b <= 0` holds and no longer mentions v. Sound because
	// both multipliers are positive; incomplete over the integers, which is the
	// side this procedure has always taken.
	//
	// It is what a flattened product needs. From `w < (len sp)/2` and the
	// axiom `2·((len sp)/2) <= len sp`, cancelling the quotient gives
	// `2w + 2 <= len sp`, which is the bound on `(sp (+ (* 2 w) 1))` — and
	// neither one fact scaled nor two facts summed can reach it, because the
	// combination needs a different multiplier on each.
	for i, a := range le {
		for j := i + 1; j < len(le); j++ {
			b := le[j]
			if !covers(i, j) || !covers(j, i) {
				continue
			}
			for v, av := range a.coef {
				bv, ok := b.coef[v]
				if !ok || (av > 0) == (bv > 0) {
					continue
				}
				ma, mb := bv, av
				if ma < 0 {
					ma = -ma
				}
				if mb < 0 {
					mb = -mb
				}
				if combMatch(a, ma, b, mb, g, false) || combMatch(a, ma, b, mb, g, true) {
					return true
				}
			}
		}
	}
	return farkas(le, g)
}

// farkas decides `facts ⊢ g <= 0` over the integers by refuting its negation.
//
// THE THEOREM. Every variable here is an integer, so g > 0 is g ≥ 1, and
//
//	{ aᵢ ≤ 0 } ⊢ g ≤ 0     if     { aᵢ ≤ 0 } ∪ { 1 − g ≤ 0 } has no RATIONAL solution,
//
// because a system with no rational solution has no integer one. Rational
// infeasibility of a conjunction of linear inequalities is exactly what
// Fourier–Motzkin elimination decides (Fourier 1826, Motzkin 1936): eliminating
// a variable v combines every pair with opposite signs on v by positive
// multipliers, and the eliminated system is feasible iff the original is. When
// no variable is left, the system is infeasible iff some row reads c ≤ 0 with
// c > 0. The combination that reaches that row is a Farkas certificate — the
// non-negative multipliers whose sum is a contradiction (Farkas 1902) — which is
// what the one- and two-fact checks above were each a special case of.
//
// TWO INTEGER STRENGTHENINGS, both sound over ℤ: every row Σaⱼxⱼ + c ≤ 0 is
// divided by d = gcd(aⱼ) with the constant rounded UP, Σ(aⱼ/d)xⱼ + ⌈c/d⌉ ≤ 0
// (the normalisation step of Pugh's Omega test, 1991); and the goal is negated to
// g ≥ 1 rather than g > 0. Neither is complete for ℤ, which this procedure has
// never claimed to be — it is sound, and complete for everything a rational
// certificate proves.
//
// IT MAY GIVE UP, and giving up is "not proven", never a guess: Fourier–Motzkin
// is doubly exponential in the worst case, so only facts connected to the goal's
// variables take part, and the elimination stops past fixed sizes. A coefficient
// that would overflow drops that combination, which only weakens the system.
func farkas(le []*linear, g *linear) bool {
	// Relevance: the connected component of the goal's variables.
	vars := map[string]bool{}
	for v := range g.coef {
		vars[v] = true
	}
	used := make([]bool, len(le))
	var rows []*linear
	for changed := true; changed; {
		changed = false
		for i, a := range le {
			if used[i] {
				continue
			}
			for v := range a.coef {
				if vars[v] {
					used[i] = true
					rows = append(rows, a)
					for w := range a.coef {
						vars[w] = true
					}
					changed = true
					break
				}
			}
		}
	}
	if len(rows) == 0 || len(rows) > farkasMaxFacts {
		return false
	}
	sys := make([]*linear, 0, len(rows)+1)
	for _, r := range rows {
		if n, ok := normalizeRow(r); ok {
			sys = append(sys, n)
		}
	}
	if neg, ok := normalizeRow(constant(1).addScaled(g, -1)); ok {
		sys = append(sys, neg)
	} else {
		return false
	}
	for {
		for _, r := range sys {
			if len(r.coef) == 0 && r.konst > 0 {
				return true
			}
		}
		// Eliminate the variable whose elimination creates the fewest rows.
		best, bestCost := "", -1
		count := map[string][2]int{}
		for _, r := range sys {
			for v, c := range r.coef {
				pn := count[v]
				if c > 0 {
					pn[0]++
				} else {
					pn[1]++
				}
				count[v] = pn
			}
		}
		for v, pn := range count {
			cost := pn[0] * pn[1]
			if bestCost < 0 || cost < bestCost || (cost == bestCost && v < best) {
				best, bestCost = v, cost
			}
		}
		if best == "" {
			return false // no variables and no contradiction: feasible
		}
		var pos, neg, next []*linear
		for _, r := range sys {
			switch c := r.coef[best]; {
			case c > 0:
				pos = append(pos, r)
			case c < 0:
				neg = append(neg, r)
			default:
				next = append(next, r)
			}
		}
		seen := map[string]bool{}
		for _, r := range next {
			seen[r.String()] = true
		}
		for _, p := range pos {
			for _, n := range neg {
				mp, mn := -n.coef[best], p.coef[best]
				row, ok := combine(p, mp, n, mn)
				if !ok {
					continue // an overflow drops a row, which only weakens the system
				}
				delete(row.coef, best)
				row, ok = normalizeRow(row)
				if !ok {
					continue
				}
				if k := row.String(); !seen[k] {
					seen[k] = true
					next = append(next, row)
				}
			}
		}
		if len(next) > farkasMaxRows {
			return false
		}
		sys = next
	}
}

// The sizes past which farkas gives up. They are MEASURED, not guessed: at 48
// connected facts freq.oro lost four proofs the moment guards started naming
// their loops (more true facts in scope, and the procedure gave up rather than
// using them). 160 facts and 4,096 rows recover every one with no measurable
// change in the sweep's time.
const (
	farkasMaxFacts = 160
	farkasMaxRows  = 4096
)

// combine is ma·a + mb·b with overflow reported rather than wrapped.
func combine(a *linear, ma int64, b *linear, mb int64) (*linear, bool) {
	out := constant(0)
	add := func(x, y int64) (int64, bool) {
		s := x + y
		if (s > x) != (y > 0) {
			return 0, false
		}
		return s, true
	}
	mul := func(x, y int64) (int64, bool) {
		if x == 0 || y == 0 {
			return 0, true
		}
		p := x * y
		if p/y != x || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64) {
			return 0, false
		}
		return p, true
	}
	for _, t := range []struct {
		l *linear
		m int64
	}{{a, ma}, {b, mb}} {
		for v, c := range t.l.coef {
			p, ok := mul(c, t.m)
			if !ok {
				return nil, false
			}
			s, ok := add(out.coef[v], p)
			if !ok {
				return nil, false
			}
			if s == 0 {
				delete(out.coef, v)
			} else {
				out.coef[v] = s
			}
		}
		p, ok := mul(t.l.konst, t.m)
		if !ok {
			return nil, false
		}
		if out.konst, ok = add(out.konst, p); !ok {
			return nil, false
		}
	}
	return out, true
}

// normalizeRow divides Σaⱼxⱼ + c ≤ 0 by the gcd of its coefficients, rounding the
// constant up — valid over the integers, and the reason a parity fact can close
// a gap a rational certificate cannot.
func normalizeRow(r *linear) (*linear, bool) {
	out := constant(r.konst)
	var d int64
	for v, c := range r.coef {
		if c == 0 {
			continue
		}
		out.coef[v] = c
		a := c
		if a < 0 {
			a = -a
		}
		if d == 0 {
			d = a
		} else {
			for a != 0 {
				d, a = a, d%a
			}
		}
	}
	if d > 1 {
		for v := range out.coef {
			out.coef[v] /= d
		}
		// ⌈c/d⌉ for either sign of c.
		q := out.konst / d
		if out.konst%d != 0 && out.konst > 0 {
			q++
		}
		out.konst = q
	}
	return out, true
}

// isVar reports whether e is exactly one variable with coefficient 1 and no
// constant — the form an equality can be turned into a substitution for.
func isVar(e *linear) (string, bool) {
	if len(e.coef) != 1 || e.konst != 0 {
		return "", false
	}
	for k, v := range e.coef {
		if v == 1 {
			return k, true
		}
	}
	return "", false
}

func (f *facts) known() string {
	if len(f.log) == 0 {
		return "nothing"
	}
	return strings.Join(f.log, ", ")
}

// ---------------------------------------------------------------- reading terms

// asLinear interprets a term as a linear expression, or reports that it is
// outside the fragment. A length term is opaque: `(alen a)` becomes the variable
// "alen(a)", which is exactly what lets bounds reasoning work without the
// checker knowing anything about arrays.
// atoms is the ATOM SIGNATURE Σ of the linear fragment: which applications,
// beyond the ones asLinear interprets, may be read as variables. The fragment is
// a theory over a signature, and its atoms are the signature's uninterpreted
// terms.
//
// A PURE HOST CALL is a sound atom by referential transparency: two occurrences
// of one printed application in a closed residual denote one value. That is not
// true of a bare application in general — a buffer read `(b i)` is headed by a
// name too, is impure (ADR 0018), and two occurrences across a store differ — so
// Σ cannot be decided from the term alone, and is supplied by the target
// (refine.go, pureAtoms). nil is the fragment with no extra atoms.
type atoms func(op string) bool

// appVar names a pure application as a variable, by its printed form — exactly
// as divVar names a quotient and lengthVar a length.
func appVar(t *core.Term) string { return "app" + t.String() }

// lin and oblig read a term in the signature this fact set reasons in, so a
// fact and a goal about the same pure call name the same atom.
func (f *facts) lin(t *core.Term) (*linear, bool)     { return asLinearIn(f.pure, t) }
func (f *facts) oblig(t *core.Term) ([]*linear, bool) { return obligationIn(f.pure, t) }

func asLinear(t *core.Term) (*linear, bool) { return asLinearIn(nil, t) }

func asLinearIn(pure atoms, t *core.Term) (*linear, bool) {
	switch t.Kind {
	case core.KInt:
		return constant(t.Int), true
	case core.KName:
		return variable(t.Name), true
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return nil, false
		}
		args := t.Args()
		switch {
		case isOp(op.Name, "add") && len(args) == 2:
			a, ok1 := asLinearIn(pure, args[0])
			b, ok2 := asLinearIn(pure, args[1])
			if ok1 && ok2 {
				return a.addScaled(b, 1), true
			}
		case isOp(op.Name, "sub") && len(args) == 2:
			a, ok1 := asLinearIn(pure, args[0])
			b, ok2 := asLinearIn(pure, args[1])
			if ok1 && ok2 {
				return a.addScaled(b, -1), true
			}
		case isOp(op.Name, "mul") && len(args) == 2:
			// Linear only when one side is a literal.
			if args[0].Kind == core.KInt {
				if b, ok := asLinearIn(pure, args[1]); ok {
					return constant(0).addScaled(b, args[0].Int), true
				}
			}
			if args[1].Kind == core.KInt {
				if a, ok := asLinearIn(pure, args[0]); ok {
					return constant(0).addScaled(a, args[1].Int), true
				}
			}
		case isLenOp(op.Name) && len(args) == 1:
			return variable(lengthVar(op.Name, args[0])), true

		// A DIVISION BY A POSITIVE LITERAL IS AN ATOM, and it was outside the
		// fragment entirely — not opaque, ABSENT — so a guard mentioning one
		// bounded nothing at all. `(< w (/ (len sp) 2))` is a perfectly linear
		// fact about the unknown `(len sp)/2`; what is nonlinear is the
		// RELATION between that unknown and `(len sp)`, and that relation is a
		// declared axiom rather than a search (decidability-map.md).
		//
		// Naming the atom by the whole term is what makes two occurrences of
		// one quotient the same variable, exactly as `lengthVar` does for a
		// length — and for the same reason: two spellings of one quantity must
		// key alike or nothing composes.
		case isOp(op.Name, "div") && len(args) == 2 &&
			args[1].Kind == core.KInt && args[1].Int > 0:
			if _, ok := asLinearIn(pure, args[0]); ok {
				return variable(divVar(t)), true
			}
		}
		// AN APPLICATION IN Σ IS AN ATOM — a pure host call, so its contract can
		// be a fact about it (theories.md §7.9) rather than an opaque string that
		// discharges only an identical one.
		if pure != nil && pure(op.Name) {
			return variable(appVar(t)), true
		}
	}
	return nil, false
}

// isLenOp recognises every spelling of "how long is this".
//
// It was `alen` and `slen` only — the names the RETIRED portable layer used —
// so tables.md's structural `len`, which is what every program written since
// has said, was opaque to the whole refinement and interval layer. A guard of
// `(go.>= i (len src))` therefore bounded nothing, and a `where` on `(len src)`
// propagated nothing.
//
// Found while re-measuring after the fixpoint fix: `-assume`, which bounds
// lengths directly, took the JSON tokeniser to 100%, and a real declared bound
// on `(len src)` only reached 46.7% (fixpoint-2026-08-27 §5).
func isLenOp(name string) bool {
	return isOp(name, "alen") || isOp(name, "slen") ||
		name == "len" || strings.HasSuffix(name, ".len")
}

// isLenTerm reports whether a term IS a length, which is the cheapest source of
// "this is non-negative" the fragment has.
func isLenTerm(t *core.Term) bool {
	if t == nil || t.Kind != core.KApp {
		return false
	}
	op := t.Op()
	return op.Kind == core.KName && isLenOp(op.Name) && len(t.Args()) == 1
}

// divVar names a quotient opaquely, keyed by the whole division term so that
// two occurrences of one quotient are one variable.
func divVar(t *core.Term) string { return "div" + t.String() }

// lengthVar names a length term opaquely. Two occurrences of `(alen a)` must
// produce the same variable or nothing is provable.
func lengthVar(op string, arg *core.Term) string {
	// NORMALISED, so `alen(a)` and `len(a)` are the same variable. Two
	// spellings of one quantity must key alike or nothing composes.
	return "len(" + arg.String() + ")"
}

// isOp matches a qualified primitive name by its last segment, so `num/int.add`
// and a target that declares `add` unqualified are both recognised.
// opAlias maps a host's own operator spelling to the fragment's name for it.
//
// The decidable fragment was keyed to the portable layer — `int.le`, `logic.and`
// — so a target declaring Go's own `<=` and `&&` degraded every refinement to an
// opaque atom. The fragment is about the OPERATION, not about who named it.
var opAlias = map[string]string{
	"+": "add", "-": "sub", "*": "mul",
	// Division and remainder, in every spelling four hosts use. Their absence
	// here meant `go./` was not recognised as division AT ALL — so the digit
	// loop, exponentiation by squaring and every other geometric descent looked
	// like an unknown operation. Found by growing the corpus.
	"/": "div", "%": "rem", "idiv": "div", "irem": "rem",
	// x86 spells multiplication `imul`, and its absence here is the same bug
	// the division line above records: the sieve's `(setl (imul i i) n)` guard
	// was an OPAQUE ATOM, so the `x <= x*x` rule could not fire and a windows
	// program could not prove the index its own guard bounds. Found the moment
	// indexing became application on this target.
	"imul": "mul",
	// A LENGTH, in every spelling a target uses for one. Without this the
	// fragment knew `alen` — the retired portable layer's name — and nothing
	// else, so `(where (== (go.len p) (go.len q)))` became an opaque atom and
	// the gauntlet's oldest refinement stopped discharging the moment `dot`
	// moved to a native target. Found by moving it.
	"len": "alen", "strlen": "slen",
	"<": "lt", "<=": "le", ">": "gt", ">=": "ge", "==": "eq",
	// `=` is the LANGUAGE's equality, which every target now has injected
	// (docs/spec/match.md §6). Sums are what surfaced its absence here: an
	// error check is `(if (= b 0) (err …) (ok …))`, so the fact the else-branch
	// needs — `b != 0` — comes from negating the language's `=` and nothing
	// else. Without this line the division inside the ok-branch could not be
	// discharged even though the guard above it says exactly what is required.
	"=":   "eq",
	"===": "eq", "sete": "eq",
	"&&": "and", "||": "or", "!": "not",
	"!=": "ne", "ne": "ne", "setne": "ne", "!==": "ne",
	// x86's ORDERING comparisons, which were missing entirely. Only `sete` and
	// `setne` were here, and nothing noticed because this target's own indexing
	// — `x64.mov` — declares no bounds precondition, so no obligation had ever
	// needed to read an x86 guard. Making indexing APPLICATION generates the
	// obligation structurally, and then a windows program could not prove a
	// single one of its own indices.
	//
	// The SIGNED forms only. `setb`, `setbe`, `seta` and `setae` are unsigned
	// comparisons, which are a different relation on a signed value, and
	// mapping them here would let the fragment prove something false.
	"setl": "lt", "setle": "le", "setg": "gt", "setge": "ge",
	// The strict branchless connectives a target may declare under its own
	// name (ADR 0017 kept `x64.andb` for the Ada reason). As a PRECONDITION
	// they are conjunction and disjunction like any other, and a `where`
	// written with them should not degrade to an opaque atom.
	"andb": "and", "orb": "or", "notb": "not",
	"f<": "flt", "f<=": "fle", "f>": "fgt", "f>=": "fge",
}

func isOp(name, want string) bool {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if a, ok := opAlias[name]; ok {
		name = a
	}
	return name == want
}

// obligation turns a boolean term into goals of the form `e <= 0`, or reports
// that it is outside the fragment and must be treated as an opaque atom.
func obligation(t *core.Term) ([]*linear, bool) { return obligationIn(nil, t) }

func obligationIn(pure atoms, t *core.Term) ([]*linear, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return nil, false
	}
	op := t.Op().Name
	args := t.Args()

	// Conjunction, in both spellings it can arrive in.
	//
	// A target that declares the host's own `&&` gives `(&& a b)`. The
	// language's `and` is sugar for a conditional (ADR 0017), so it arrives as
	// `(if a b false)` — and the fragment has to see through the desugaring or
	// a refinement written with `and` silently degrades to an opaque atom.
	//
	// Only conjunction needs this. `(if a true b)` is a DISJUNCTION and
	// `(if a false true)` a negation, and neither is in a fragment that is
	// conjunctions of linear inequalities — which is the same wall `d ≠ 0`
	// hits (assessment-2026-08-19 §3.2).
	conj := isOp(op, "and") && len(args) == 2
	if !conj && isOp(op, "if") && len(args) == 3 &&
		args[2].Kind == core.KBool && !args[2].IsTrue() {
		conj = true
		args = args[:2]
	}
	if conj {
		a, ok1 := obligationIn(pure, args[0])
		b, ok2 := obligationIn(pure, args[1])
		if ok1 && ok2 {
			return append(a, b...), true
		}
		return nil, false
	}
	if len(args) != 2 {
		return nil, false
	}
	l, ok1 := asLinearIn(pure, args[0])
	r, ok2 := asLinearIn(pure, args[1])
	if !ok1 || !ok2 {
		return nil, false
	}
	// Integers, so strict `<` is `<= -1`.
	switch {
	case isOp(op, "lt"): // l < r   ⟶  l - r + 1 <= 0
		return []*linear{l.addScaled(r, -1).addScaled(constant(1), 1)}, true
	case isOp(op, "le"): // l <= r  ⟶  l - r <= 0
		return []*linear{l.addScaled(r, -1)}, true
	case isOp(op, "gt"): // l > r   ⟶  r - l + 1 <= 0
		return []*linear{r.addScaled(l, -1).addScaled(constant(1), 1)}, true
	case isOp(op, "ge"): // l >= r  ⟶  r - l <= 0
		return []*linear{r.addScaled(l, -1)}, true
	case isOp(op, "eq"): // both directions
		return []*linear{l.addScaled(r, -1), r.addScaled(l, -1)}, true
	}
	return nil, false
}

// constantValue reports the value of a linear expression with no variables.
func (l *linear) constantValue() (int64, bool) {
	for _, c := range l.coef {
		if c != 0 {
			return 0, false
		}
	}
	return l.konst, true
}
