package ir

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"oroboros/presburger"
)

// THE RELATIONS THE INTERVALS FORGET, decided by Presburger arithmetic.
//
// The interval domain is non-relational: a value's fact is a box, and the
// transfer of x + y is fact(x) + fact(y) whatever x and y are. On a term in
// which each atom occurs once and no two atoms are related, that is exact: each
// of +, −, a constant's ·, and ⌊·/k⌋ is monotone in each argument, and
// independent arguments reach every corner of the box (Moore 1966, the single
// use expression; "the natural interval extension of a function in which each
// variable occurs once is its range"). So the box loses precision exactly
// where an atom REPEATS, or where the program has RELATED two atoms, by a
// `where` (assumeDiff) or an arm's guard (a π): Bloch's midpoint
// lo + ⌊(hi − lo)/2⌋ under lo ≤ hi repeats lo, and its box is
// [−2⁶³, 2⁶⁴ − 2] while its range is [lo, hi] ⊆ S₆₄:
//
//	d = hi − lo ≥ 0,  d = 2q + m, 0 ≤ m ≤ 1  ⟹  0 ≤ q ≤ d  ⟹  lo ≤ lo + q ≤ hi.
//
// relate is that step, run where the box leaves the word: the operation's term
// over the function's values, unfolded through + − · ⌊/k⌋ to its atoms, each
// atom bounded by its fact, the related pairs by their difference, each
// division by its definition (truncated, as `/` is: a = k·q + m with the
// remainder's sign the dividend's), and the claim "the value is in the word"
// handed to the Omega test. Valid, the box is met with the word; anything
// else, the box stands. It is sound because every hypothesis holds where the
// operation runs: a fact holds of its value everywhere, an entry assumption
// everywhere (assume), and a π exists only on the arm its guard selects, so a
// π the term reads is in scope, and its guard holds there.
func (a *intervals) relate(s *Stmt, r iv) iv {
	if PlantRelateOff || r.bot || len(s.Res) != 1 {
		return r
	}
	wLo, wHi := ei(a.tg.Word.Lo), ei(a.tg.Word.Hi)
	if r.within(wLo, wHi) {
		return r
	}
	if !a.mayRelate(s.Res[0]) {
		return r // independent atoms: the box is exact
	}
	tr := &relTr{a: a, bound: map[V]iv{}, uses: map[V]int{}}
	l, ok := tr.build(s.Res[0])
	if !ok || !tr.related() {
		return r
	}
	// THE SAME QUESTION IS ASKED ONCE: the fixpoint evaluates a statement on
	// each round, and the answer is a function of the term and its atoms'
	// bounds, which is the key
	key := tr.key(l)
	if m, ok := a.related[s]; ok && m.key == key {
		return meetIV(r, m.out)
	}
	out := tr.decide(l, wLo, wHi)
	if a.related == nil {
		a.related = map[*Stmt]relMemo{}
	}
	a.related[s] = relMemo{key, out}
	return meetIV(r, out)
}

type relMemo struct {
	key string
	out iv
}

// decide is the set the term is proven in: the word, the unsigned word, or ⊤.
// THE COMBINATION FIRST: with the repeated atoms' coefficients summed, the
// term is Σ cᵢ·xᵢ + k over distinct atoms, whose box is exact when no two are
// related and nothing is a division's quotient; lo + (hi − lo) is hi this
// way, at no cost. Only what that does not settle reaches the Omega test.
func (tr *relTr) decide(l presburger.Lin, wLo, wHi ep) (out iv) {
	defer func() {
		if e := recover(); e != nil {
			if !presburger.Overflowed(e) {
				panic(e)
			}
			out = ivTop // a coefficient past int64: unknown, the box stands
		}
	}()
	if len(tr.defs) == 0 && len(tr.pairs()) == 0 {
		if b, ok := tr.box(l); ok {
			switch {
			case b.within(wLo, wHi):
				return rangeE(wLo, wHi)
			case tr.a.tg.Word.Unsigned && b.within(ep{}, u64Max):
				return rangeE(ep{}, u64Max)
			}
		}
		return ivTop // the box of independent atoms is exact: nothing more
	}
	hyps := tr.hyps()
	var hk strings.Builder
	for _, h := range hyps {
		formulaText(&hk, h)
	}
	in := func(lo, hi ep) bool {
		goal := presburger.And(
			presburger.Ge(l.Add(presburger.ConstBig(lo.big()), -1)),
			presburger.Ge(presburger.ConstBig(hi.big()).Add(l, -1)))
		var k strings.Builder
		k.WriteString(hk.String())
		k.WriteString("⊢")
		formulaText(&k, goal)
		if v, ok := validMemo.Load(k.String()); ok && !PlantRelateNoMemo {
			return v.(bool)
		}
		v := presburger.Valid(hyps, goal) == presburger.Unsat
		validMemo.Store(k.String(), v)
		return v
	}
	// ONE QUESTION FIRST, the union of the two words: a term in neither, as
	// K(ℕ*)'s digit arithmetic mostly is, fails once and not twice
	top := wHi
	if tr.a.tg.Word.Unsigned {
		top = u64Max
	}
	if !in(wLo, top) {
		return ivTop
	}
	switch {
	case in(wLo, wHi):
		return rangeE(wLo, wHi)
	case tr.a.tg.Word.Unsigned && in(ep{}, u64Max):
		return rangeE(ep{}, u64Max)
	}
	return ivTop
}

// validMemo is Valid's answer by its sequent's text: a function of the text,
// so a process-wide table changes when an answer is computed and never what
// it is. A function is decided several times (before and after the rung, the
// words, the pairs), each asking the same questions of its library's digits.
var validMemo sync.Map

// PlantRelateNoMemo asks the Omega test every time (a measurement's switch).
var PlantRelateNoMemo bool

// formulaText writes f canonically: its former, its term, its children.
func formulaText(b *strings.Builder, f *presburger.Formula) {
	fmt.Fprintf(b, "(%d %s %d %s %s", f.Kind, f.T.String(), f.D, f.Name, f.Var)
	for _, x := range f.Xs {
		formulaText(b, x)
	}
	b.WriteString(")")
}

// box is the interval of Σ cᵢ·xᵢ + k over the atoms' bounds.
func (tr *relTr) box(l presburger.Lin) (iv, bool) {
	k, ok := fromBig(presburger.KOf(l))
	if !ok {
		return iv{}, false
	}
	out := rangeE(k, k)
	for x, c := range l.Coef {
		var p V
		if _, err := fmt.Sscanf(x, "v%d", &p); err != nil {
			return iv{}, false
		}
		out = addIV(out, mulIV(exactIV(c), tr.bound[p]))
	}
	return out, true
}

// key is the question: the term, its atoms' bounds and the definitions.
func (tr *relTr) key(l presburger.Lin) string {
	ps := make([]V, 0, len(tr.bound))
	for p := range tr.bound {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i] < ps[j] })
	var b strings.Builder
	b.WriteString(l.String())
	for _, p := range ps {
		fmt.Fprintf(&b, ";%d%s", p, show(tr.bound[p]))
	}
	fmt.Fprintf(&b, ";%d", len(tr.defs))
	return b.String()
}

// mayRelate is related's structural premise, read off the term's shape alone
// before anything is built: an atom's place reached twice, or a place the
// program compared with another (an entry assumption, or a guard on a π
// read). Most of K(ℕ*)'s digit arithmetic is neither, and is turned away here.
func (a *intervals) mayRelate(v V) bool {
	var places [relateNodes + 1]V
	n, nodes := 0, 0
	related := false
	var walk func(v V)
	walk = func(v V) {
		if related || nodes > relateNodes {
			return
		}
		nodes++
		if d := a.def[v]; d != nil && len(d.Res) == 1 && d.Mode != MWrap && d.Mode != MWrapPair {
			switch {
			case d.Op == OConst:
				return
			case d.Name == "" && (d.Op == OAdd || d.Op == OSub || d.Op == OMul || d.Op == ONeg || d.Op == ODiv),
				d.Op == OCall && relWord[d.Name]:
				for _, x := range d.Args {
					walk(x)
				}
				return
			}
		}
		p := a.pl(v)
		if _, ok := a.piRel[v]; ok && v != p {
			related = true // a guard on the chain: pairs() reads it
			return
		}
		for _, q := range places[:n] {
			if q == p {
				related = true
				return
			}
			if _, ok := a.assumed[[2]V{p, q}]; ok {
				related = true
				return
			}
			if _, ok := a.assumed[[2]V{q, p}]; ok {
				related = true
				return
			}
		}
		if n < len(places) {
			places[n] = p
			n++
		}
	}
	walk(v)
	return related
}

// relWord is the unsigned word's and the rung's operations lin unfolds.
var relWord = map[string]bool{"u64-of": true, "int-of-u64": true, "big-of": true, "big-word": true,
	"u64+": true, "u64-": true, "u64/": true}

// PlantRelateOff skips relate: Bloch's midpoint over full words is then
// refused on every target (TestBlochsMidpointIsProven).
var PlantRelateOff bool

// PlantRelateGuardAnyArm reads a π's guard as holding of its source on every
// arm, so the ELSE arm of `if lo <= hi` is believed lo ≤ hi too
// (TestRelateReadsAGuardOnItsArm).
var PlantRelateGuardAnyArm bool

// relTr is one term's translation: its atoms by place, each one's bound the
// meet of the facts of the values naming it, and the divisions' definitions.
type relTr struct {
	a      *intervals
	bound  map[V]iv // an atom's place: the meet of its names' facts
	uses   map[V]int
	seen   []V // every name of an atom read, π-parameters among them
	defs   []*presburger.Formula
	fresh  int
	nodes  int
	failed bool
}

const relateNodes = 24 // a term's size, past which the box stands

func (tr *relTr) build(v V) (l presburger.Lin, ok bool) {
	defer func() {
		if e := recover(); e != nil {
			if !presburger.Overflowed(e) {
				panic(e)
			}
			l, ok = presburger.Lin{}, false // a coefficient past int64
		}
	}()
	l = tr.lin(v, true)
	return l, !tr.failed
}

// lin is v's term. The operation itself is unfolded (top); below it, a value
// is unfolded only through the operations whose box is exact on independent
// arguments, so that every atom is a value the term cannot see into.
func (tr *relTr) lin(v V, top bool) presburger.Lin {
	a := tr.a
	if tr.nodes++; tr.nodes > relateNodes {
		tr.failed = true
		return presburger.Const(0)
	}
	d := a.def[v]
	if d != nil && d.Op == OCall && d.Mode != MWrap && d.Mode != MWrapPair && len(d.Res) == 1 && !PlantRelateWordsOpaque {
		// THE UNSIGNED WORD AND THE RUNG ARE ℤ's on what they hold. The words
		// pass's invariant (ADR 0026, 0033; the transfers in interval.go read
		// it too, u64Arith taking ℤ's operation): an operation of U is chosen
		// only where ℤ's result is in U, so the ring homomorphism ℤ → ℤ/2⁶⁴
		// agrees with ℤ there, and its operands' residues r_U(x) give ℤ's
		// result; a conversion back is placed only where the integer is in S,
		// where r_S is the identity. So each is ℤ's operation on its
		// arguments' ℤ terms, the conversions read through, the result in
		// [0, 2⁶⁴ − 1], the premise the choice was made on: this reads the
		// words pass's output by its meaning, and trusts nothing the decision
		// after it does not already.
		inU := func(l presburger.Lin) presburger.Lin {
			tr.defs = append(tr.defs, boundF(l, rangeE(ep{}, u64Max))...)
			return l
		}
		switch d.Name {
		case "u64-of", "int-of-u64", "big-of", "big-word":
			if len(d.Args) == 1 {
				return tr.lin(d.Args[0], false)
			}
		case "u64+":
			return inU(tr.lin(d.Args[0], false).Add(tr.lin(d.Args[1], false), 1))
		case "u64-":
			return inU(tr.lin(d.Args[0], false).Add(tr.lin(d.Args[1], false), -1))
		case "u64/":
			if k, ok := tr.constThrough(d.Args[1]); ok && k >= 1 {
				return inU(tr.div(tr.lin(d.Args[0], false), k))
			}
		}
	}
	if d != nil && d.Mode != MWrap && d.Mode != MWrapPair && d.Name == "" && len(d.Res) == 1 {
		switch d.Op {
		case OConst:
			if k, ok := a.constOf(v); ok {
				return presburger.Const(k)
			}
		case OAdd:
			return tr.lin(d.Args[0], false).Add(tr.lin(d.Args[1], false), 1)
		case OSub:
			return tr.lin(d.Args[0], false).Add(tr.lin(d.Args[1], false), -1)
		case ONeg:
			return tr.lin(d.Args[0], false).Scale(-1)
		case OMul:
			if k, ok := a.constOf(d.Args[1]); ok {
				return tr.lin(d.Args[0], false).Scale(k)
			}
			if k, ok := a.constOf(d.Args[0]); ok {
				return tr.lin(d.Args[1], false).Scale(k)
			}
		case ODiv:
			if k, ok := a.constOf(d.Args[1]); ok && k >= 1 {
				return tr.div(tr.lin(d.Args[0], false), k)
			}
		}
	}
	// The operation itself, where it cannot be unfolded (a product of two
	// variables), is the term's one atom, read once: related() turns that away,
	// so its fact, the one being computed, is never a hypothesis
	_ = top
	return tr.atom(v)
}

// div is x / k, k ≥ 1, by its definition: a fresh q with x = k·q + m,
// 0 ≤ m < k when x ≥ 0 and −k < m ≤ 0 when x < 0, the truncated division `/`
// is (and on U, where x ≥ 0, the floor).
func (tr *relTr) div(x presburger.Lin, k int64) presburger.Lin {
	tr.fresh++
	Q := presburger.Var(fmt.Sprintf("q·%d", tr.fresh))
	M := presburger.Var(fmt.Sprintf("m·%d", tr.fresh))
	tr.defs = append(tr.defs,
		presburger.EqF(x.Add(Q, -k).Add(M, -1)),
		presburger.Or(
			presburger.And(presburger.Ge(x), presburger.Ge(M), presburger.Ge(presburger.Const(k-1).Add(M, -1))),
			presburger.And(presburger.Ge(x.Scale(-1).Add(presburger.Const(-1), 1)),
				presburger.Ge(M.Scale(-1)), presburger.Ge(M.Add(presburger.Const(k-1), 1)))))
	return Q
}

// constThrough is v's constant, through the unsigned word's inclusion.
func (tr *relTr) constThrough(v V) (int64, bool) {
	if d := tr.a.def[v]; d != nil && d.Op == OCall && d.Name == "u64-of" && len(d.Args) == 1 {
		v = d.Args[0]
	}
	return tr.a.constOf(v)
}

// PlantRelateWordsOpaque reads the unsigned word's operations as atoms: the
// decision after the words pass then loses the relation the first one proved,
// and Bloch's midpoint is refused there (TestBlochsMidpointIsProven).
var PlantRelateWordsOpaque bool

// atom is v as a variable, named by its place: a π-parameter is its source,
// renamed on an arm, one value.
func (tr *relTr) atom(v V) presburger.Lin {
	a := tr.a
	p := a.pl(v)
	f := a.fs[v].v
	if b, ok := tr.bound[p]; ok {
		f = meetIV(b, f)
	}
	tr.bound[p] = f
	tr.uses[p]++
	tr.seen = append(tr.seen, v)
	return presburger.Var(atomName(p))
}

func atomName(p V) string { return fmt.Sprintf("v%d", p) }

// related reports what makes the box inexact: an atom read twice, or two
// atoms the program related.
func (tr *relTr) related() bool {
	for _, n := range tr.uses {
		if n > 1 {
			return true
		}
	}
	return len(tr.pairs()) > 0
}

// pairs is every difference the program states between two atoms: an entry
// assumption's (assumeDiff), keyed by place, and the guard of each π a name
// read lies under, on that name's chain (guardDiff's reading).
func (tr *relTr) pairs() []*presburger.Formula {
	a := tr.a
	var out []*presburger.Formula
	diff := func(x, y V, d iv) {
		l := presburger.Var(atomName(x)).Add(presburger.Var(atomName(y)), -1)
		out = append(out, boundF(l, d)...)
	}
	for x := range tr.bound {
		for y := range tr.bound {
			if d, ok := a.assumed[[2]V{x, y}]; ok && x != y {
				diff(x, y, d)
			}
		}
	}
	for _, v := range tr.seen {
		for u, n := v, 0; n < 8; n++ {
			if pi, ok := a.piRel[u]; ok && !pi.Len {
				if _, in := tr.bound[a.pl(pi.Other)]; in && a.pl(pi.Other) != a.pl(v) {
					diff(a.pl(v), a.pl(pi.Other), diffOf(pi.Rel))
				}
			}
			src, ok := a.piOf[u]
			if !ok {
				break
			}
			u = src
		}
	}
	if PlantRelateGuardAnyArm {
		out = append(out, tr.anyArm()...)
	}
	return out
}

// anyArm is the planted fault's reading: every guard on a source, wherever the
// π naming it is, as if the source were under it.
func (tr *relTr) anyArm() []*presburger.Formula {
	a := tr.a
	var out []*presburger.Formula
	for _, pi := range a.piRel {
		x, y := a.pl(pi.Of), a.pl(pi.Other)
		_, inx := tr.bound[x]
		_, iny := tr.bound[y]
		if inx && iny && x != y && !pi.Len {
			l := presburger.Var(atomName(x)).Add(presburger.Var(atomName(y)), -1)
			out = append(out, boundF(l, diffOf(pi.Rel))...)
		}
	}
	return out
}

// hyps is the term's premises: each atom's bound, the related pairs, and the
// divisions' definitions.
func (tr *relTr) hyps() []*presburger.Formula {
	out := append([]*presburger.Formula(nil), tr.defs...)
	for p, b := range tr.bound {
		out = append(out, boundF(presburger.Var(atomName(p)), b)...)
	}
	return append(out, tr.pairs()...)
}

// boundF is l ∈ b, a finite end each an inequality.
func boundF(l presburger.Lin, b iv) []*presburger.Formula {
	if b.bot {
		return []*presburger.Formula{presburger.Ge(presburger.Const(-1))} // false
	}
	var out []*presburger.Formula
	if !b.nlo {
		out = append(out, presburger.Ge(l.Add(presburger.ConstBig(b.lo.big()), -1)))
	}
	if !b.phi {
		out = append(out, presburger.Ge(presburger.ConstBig(b.hi.big()).Add(l, -1)))
	}
	return out
}
