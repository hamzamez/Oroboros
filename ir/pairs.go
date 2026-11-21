package ir

import (
	"fmt"
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// THE UNSIGNED WORD AS A PAIR (ADR 0083; u64repr-2026-09-22).
//
// On a target whose word is narrower than 64 bits (JavaScript's, ±(2⁵³−1)),
// U = [0, 2⁶⁴) is held as two digits base B = 2³²: the representation
// (hi, lo) ∈ [0, B)², d(hi, lo) = hi·B + lo, which is a bijection onto U.
// SelectPairs gives every value whose fact lies in U and outside the word, and
// every operation fused under `mod 2ⁿ` with 32 < n ≤ 64 (mode wrap64, Fuse),
// that representation, and expands each operation on one into the language's
// own operations on its digits. Each expansion is a law of arithmetic in base
// B, with d(·) commuting:
//
//	a + b:  s = aₗ + bₗ;  lo = s mod B;  hi = aₕ + bₕ + ⌊s/B⌋        (mod B, wrapped)
//	a − b:  t = aₗ − bₗ;  lo = t mod B;  hi = aₕ − bₕ + ⌊t/B⌋        (mod B, wrapped)
//	a · b:  aₗ·bₗ exactly by 16-bit digits (each product below 2³², exact in a
//	        double); hi = ⌊aₗbₗ/B⌋ + aₕbₗ + aₗbₕ mod B, the cross terms by the
//	        target's 32-bit wrapping multiply (Math.imul), aₕbₕ·B² vanishing mod 2⁶⁴
//	& | ⊕:  digit by digit;  ~:  B − 1 − each digit
//	<< k, >> k (k a constant): the digits' shifts, the bits crossing the
//	        boundary carried between them
//	/ 2ᵐ, % 2ᵐ, mod 2ᵐ: a right shift and a mask
//	x ⋈ y:  the lexicographic order of (h, l), xₗ ⋈ yₗ where xₕ = yₕ (ADR 0151)
//	a / b, a rem b for a word |a| < b: 0 and a, reading no digit of b
//
// Every operation a pass writes is an operation on integers below 2³⁴ or
// fused under `mod 2³²`, so the decision run after it proves each one inside
// the word, or the program is refused: the expansion is checked by the
// machinery that checks everything else. An exact operation's result is proven
// in U, so its digits are the value's; a wrapped one's are its residue's. A
// result in the word S is read from its residue's digits signed, h − 2³²·[h ≥
// 2³¹], and ascribed its decided fact (toWord, ADR 0151).
//
// A VALUE THE PAIR CANNOT COMPUTE WITH IS NOT A PAIR (ADR 0151, PairsLeaving):
// one divided by a non-power of two or shifted by a variable, one compared with
// a word that may be negative, one whose operation's result leaves U, is
// demanded in ℤ before the rung, and the rung's closure takes what it touches.
// Left refused, saying so: such a use of a residue (an operation fused under
// mod 2⁶⁴, which the rung does not hold), a pair entering or leaving through a
// build's region, and an entry point's parameter held as a pair. A pair
// reaching a host argument of any type is boxed (`pair-box`, a decimal
// string), and an entry point's result is the host's BigInt (`pair-bigint`),
// the representation a caller holds it in; a value of U held exactly leaves
// the same way (`big-box`, the BigInt itself).
func SelectPairs(tg *emit.Target, f *Func, sig *core.Sig) (bool, error) {
	if !tg.Word.Pair {
		return false, nil
	}
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
	}
	p := newPairs(tg, f, fs, sig)
	p.classify()
	if p.err != nil {
		return false, p.err
	}
	if len(p.pair) == 0 && !p.anyResult() {
		return false, nil
	}
	for _, x := range f.Params {
		if p.pair[x] {
			return false, fmt.Errorf("an entry point's parameter held in the unsigned word, %s, is not built on "+
				"target %s, which holds that word as a pair (ir/pairs.go)", termShow(fs[x].v), tg.Name)
		}
	}
	p.rewrite()
	if p.err != nil {
		return false, p.err
	}
	Canonicalize(f)
	f.facts = nil
	return true, nil
}

func newPairs(tg *emit.Target, f *Func, fs []fact, sig *core.Sig) *pairs {
	return &pairs{tg: tg, f: f, fs: fs, pair: map[V]bool{}, halves: map[V][2]V{}, kon: map[V]*big.Int{},
		sub: map[V]V{}, def: map[V]*Stmt{}, nv: f.NV(),
		// a header that states its result: an unstated one claims nothing
		// (ADR 0062 item 8), so its type is typing's and no declared set
		stated: sig != nil && (len(sig.Results) > 0 || sig.Result != "" && sig.Result != "any")}
}

// THE PAIRS' SELECTION (ADR 0151): ρ(v) is the cheapest representation whose
// domain holds v's fact AND whose operations cover v's uses. U's pair is the
// cheapest for a value of U past the word, but it has no division by a
// non-power of two, no variable shift, no comparison with a word that may be
// negative, and no operation whose result leaves U; ℤ, the rung, has all of
// them. So a value of U leaves the pair exactly when a use of it, or the
// operation computing it, is one the rung computes and the pair does not, and
// the rung's own closure (big.go's solve) takes what that value touches: an
// operation over an exact operand is exact, an exact result's operands are
// demanded. That is the greatest fixed point of "every use covered" on the pair
// and the least on ℤ, the same lattice read from both ends.
//
// PairsLeaving is that set on the decided f, before the rung; D_A (exact.go)
// demands it. A residue, an operation fused under mod 2⁶⁴ (MWrapPair), never
// leaves: the rung holds no residue, and where one meets ℤ it enters by the
// inclusion ι : U ↪ ℤ, `pair-bigint` at the rung's `big-of`. A use the rung
// refuses as well, a table's index, a host's word, is no reason to leave.
//
// Measured (pairsel-2026-11-21): a digit sum over U in BigInt throughout costs
// 531 ns a value, the pair with each division conjugated through BigInt 1,869:
// a value leaves whole, never one operation at a time.
func PairsLeaving(tg *emit.Target, f *Func, sig *core.Sig) map[V]bool {
	if !tg.Word.Pair || PlantPairsKeepAll || len(f.facts) != f.NV() {
		return nil
	}
	p := newPairs(tg, f, f.facts, sig)
	p.classify() // its refusals are the pass's own, made after the rung
	residue := func(v V) bool { d := p.def[v]; return d != nil && d.Mode == MWrapPair }
	out := map[V]bool{}
	leave := func(vs ...V) {
		for _, v := range vs {
			// a closed constant is a value, which the rung spells as one (its
			// spine), never a demand
			if _, k := p.kon[v]; v >= 0 && p.pair[v] && !residue(v) && !k {
				out[v] = true
			}
		}
	}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Name != "" || len(s.Res) != 1 || !langOp(s) || !(s.Op.IsArith() || s.Op.IsBit() || s.Op.IsCmp()) {
				continue
			}
			touched := p.pair[s.Res[0]]
			for _, a := range s.Args {
				touched = touched || p.pair[a]
			}
			if touched && !p.covers(s) {
				leave(s.Args...)
				leave(s.Res[0])
			}
		}
	})
	// A JOIN A PAIR ENTERS THAT MAY LIE OUTSIDE U holds no pair: it is ℤ's,
	// and the pairs that enter it with it
	for v := range p.pair {
		if !p.inU(v) && !residue(v) {
			out[v] = true
			for _, e := range p.flows {
				if !e.fn && e.dst == v {
					leave(e.src)
				}
			}
		}
	}
	return out
}

// PlantPairsKeepAll leaves every value of U in its pair, as before ADR 0151:
// a division of one by 3 is refused again (TestAPairLeavesForTheIntegers).
var PlantPairsKeepAll bool

// covers reports that the pass expands s, an operation a pair touches: the
// pass's own cases, read off its rewrite (arith, compare).
func (p *pairs) covers(s *Stmt) bool {
	r := s.Res[0]
	if _, ok := p.kon[r]; ok && p.pair[r] {
		return true // a closed constant is its digits
	}
	res := p.pair[r] || s.Mode == MWrapPair
	if s.Op.IsCmp() {
		return len(s.Args) == 2 && p.hasDigits(s.Args[0], false) && p.hasDigits(s.Args[1], false)
	}
	switch s.Op {
	case OAdd, OSub, OMul, OAnd, OOr, OXor:
		return res || p.inS(r)
	case ONeg, ONot:
		return s.Mode == MWrapPair
	case OShl, OShr, ODiv, ORem, OMod:
		if len(s.Args) == 2 && p.smallByLarge(s) {
			return true
		}
		if len(s.Args) != 2 || p.pair[s.Args[1]] {
			return false
		}
		c, ok := p.kon[s.Args[1]]
		if !ok || !c.IsInt64() || c.Sign() < 0 {
			return false
		}
		if s.Op != OShl && s.Op != OShr && (c.Sign() == 0 || c.Int64()&(c.Int64()-1) != 0) {
			return false
		}
		return p.hasDigits(s.Args[0], s.Op == OShl && res) && (res || p.inS(r))
	}
	return false
}

// smallByLarge reports a quotient or remainder of a word by a divisor whose
// fact lies above the word's magnitude: for |a| < b, a / b = 0 and a rem b = a,
// truncating, and a mod b = a for a ≥ 0, so no digit of the divisor is read.
func (p *pairs) smallByLarge(s *Stmt) bool {
	a, b := s.Args[0], s.Args[1]
	if p.pair[a] || !p.pair[b] || int(a) >= p.nv || int(b) >= p.nv || PlantPairSmallByLarge {
		return false
	}
	x, y := p.fs[a].v, p.fs[b].v
	if !x.finite() || y.bot || y.nlo {
		return false
	}
	m := x.hi.big()
	if n := new(big.Int).Neg(x.lo.big()); n.Cmp(m) > 0 {
		m = n
	}
	if y.lo.big().Cmp(m) <= 0 {
		return false
	}
	return s.Op != OMod || x.lo.sign() >= 0
}

// PlantPairSmallByLarge refuses a word divided by a pair above it, as before
// ADR 0151: the Mersenne hash's first step is then refused on JavaScript
// (TestAPairLeavesForTheIntegers).
var PlantPairSmallByLarge bool

// hasDigits reports that toPair gives v's digits, without making them.
func (p *pairs) hasDigits(v V, residue bool) bool {
	if p.pair[v] {
		return true
	}
	if c, ok := p.kon[v]; ok {
		return residue || c.Sign() >= 0 && c.Cmp(pairTop) < 0
	}
	if v < 0 || int(v) >= len(p.fs) {
		return false
	}
	return residue || p.fs[v].v.within(ep{}, ei(p.tg.Word.Hi))
}

// inS reports a value of the pass's input whose fact lies in the signed word.
func (p *pairs) inS(v V) bool {
	return v >= 0 && int(v) < p.nv && p.fs[v].v.within(ei(p.tg.Word.Lo), ei(p.tg.Word.Hi))
}

var (
	pairB    = new(big.Int).Lsh(big.NewInt(1), 32)
	pairMask = int64(1<<32 - 1)
	pairTop  = new(big.Int).Lsh(big.NewInt(1), 64)
)

type pairs struct {
	tg     *emit.Target
	f      *Func
	fs     []fact
	pair   map[V]bool
	halves map[V][2]V
	kon    map[V]*big.Int // closed constants, exact
	sub    map[V]V        // a value replaced by another
	def    map[V]*Stmt
	resPar []bool // the function's results held as pairs
	flows  []flow // classify's joins, each input to its receiver
	stated bool   // the header states the results
	nv     int    // the values before the pass, whose facts are the decision's
	out    []Stmt
	err    error
}

func (p *pairs) fail(format string, a ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, a...)
	}
}

func (p *pairs) anyResult() bool {
	for _, b := range p.resPar {
		if b {
			return true
		}
	}
	return false
}

// inU reports a fact inside U; above reports one outside the word.
func (p *pairs) inU(v V) bool { return v >= 0 && int(v) < len(p.fs) && p.fs[v].v.within(ep{}, u64Max) }
func (p *pairs) above(v V) bool {
	return v >= 0 && int(v) < len(p.fs) && !p.fs[v].v.within(ei(p.tg.Word.Lo), ei(p.tg.Word.Hi))
}

// ---------------------------------------------------------------- classify

type recv struct {
	vs []V
	fn bool // the function's results
}

type flow struct {
	src V
	dst V   // or −1 with fn: the function's result j
	j   int // fn's index
	fn  bool
}

func (p *pairs) classify() {
	f := p.f
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				p.def[v] = s
			}
		}
	})
	// the closed constants, exactly (a literal past int64 is a spine of words)
	var konst func(v V) (*big.Int, bool)
	konst = func(v V) (*big.Int, bool) {
		if c, ok := p.kon[v]; ok {
			return c, true
		}
		d := p.def[v]
		if d == nil || d.Name != "" {
			return nil, false
		}
		switch {
		case d.Op == OConst && d.Lit != nil && d.Lit.Kind == core.KInt:
			c := big.NewInt(d.Lit.Int)
			p.kon[v] = c
			return c, true
		case (d.Op == OAdd || d.Op == OSub || d.Op == OMul) && len(d.Args) == 2 && d.Mode != MWrap:
			a, ok1 := konst(d.Args[0])
			b, ok2 := konst(d.Args[1])
			if !ok1 || !ok2 {
				return nil, false
			}
			c := new(big.Int)
			switch d.Op {
			case OAdd:
				c.Add(a, b)
			case OSub:
				c.Sub(a, b)
			default:
				c.Mul(a, b)
			}
			if d.Mode == MWrapPair {
				c.Mod(c, pairTop)
			}
			p.kon[v] = c
			return c, true
		}
		return nil, false
	}
	var flows []flow
	var walk func(r *Region, yield, brk, cont recv)
	edge := func(args []V, to recv) {
		for j, a := range args {
			if to.fn {
				flows = append(flows, flow{src: a, j: j, fn: true})
			} else if j < len(to.vs) {
				flows = append(flows, flow{src: a, dst: to.vs[j]})
			}
		}
	}
	walk = func(r *Region, yield, brk, cont recv) {
		for _, pi := range r.Pis {
			flows = append(flows, flow{src: pi.Of, dst: pi.V})
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				konst(v)
				if s.Mode == MWrapPair || p.inU(v) && p.above(v) {
					p.pair[v] = true
				}
			}
			switch s.Op {
			case OIf:
				for _, sub := range s.Sub {
					walk(sub, recv{vs: s.Res}, brk, cont)
				}
			case OLoop:
				body := s.Sub[0]
				edge(s.Args, recv{vs: body.Params})
				for _, x := range body.Params {
					if p.inU(x) && p.above(x) {
						p.pair[x] = true
					}
				}
				walk(body, recv{vs: s.Res}, recv{vs: s.Res}, recv{vs: body.Params})
			default:
				for _, sub := range s.Sub {
					walk(sub, recv{vs: s.Res}, recv{}, recv{})
				}
			}
		}
		switch r.T {
		case TYield:
			edge(r.Args, yield)
		case TBreak:
			edge(r.Args, brk)
		case TContinue:
			edge(r.Args, cont)
		case TBranch:
			walk(r.Then, yield, brk, cont)
			walk(r.Else, yield, brk, cont)
		}
	}
	p.resPar = make([]bool, len(f.Results))
	walk(f.Body, recv{fn: true}, recv{}, recv{})
	p.flows = flows
	// A JOIN WITH A PAIR COMING IN IS A PAIR: the least fixed point.
	for changed := true; changed; {
		changed = false
		for _, e := range flows {
			if !p.pair[e.src] {
				continue
			}
			if e.fn {
				if e.j < len(p.resPar) && !p.resPar[e.j] {
					p.resPar[e.j], changed = true, true
				}
			} else if !p.pair[e.dst] {
				p.pair[e.dst], changed = true, true
			}
		}
	}
	// A RESULT HELD AS A PAIR LEAVES AS U ONLY WHERE IT IS DECLARED IN U (ADR
	// 0033, 0079 item 3): an entry point's result takes its representation from
	// its declared set, so a declared Int is the word on every target, and a
	// value of U is refused there, as Go's, the JVM's and windows' are. Taken
	// as the pair's BigInt, an export declared Int returned a BigInt or a
	// number by its facts (book finding 27).
	for j, held := range p.resPar {
		if held && p.stated && j < len(f.Results) && p.tg.ValueType(f.Results[j]) != core.U64Type && !PlantPairResultAnySet {
			p.fail("a result is held in the unsigned word, which target %s holds as a pair, and is "+
				"declared %s, not in the unsigned word: an entry point's result takes its representation "+
				"from its declared set, so it is refused here, as a value of the unsigned word is at a "+
				"declared Int on every target (ADR 0033, ADR 0079). Prove it inside the word, or declare "+
				"the result's range, 0..N, to return it in the unsigned word",
				p.tg.Name, core.ShowSet(f.Results[j]))
		}
	}
	for v := range p.pair {
		if !p.inU(v) && p.def[v] != nil && p.def[v].Mode != MWrapPair {
			p.fail("%s joins a value held in the unsigned word as a pair, and may lie outside it, %s; "+
				"no representation holds both on target %s (ir/pairs.go)", termShow(p.fs[v].v), termShow(p.fs[v].v), p.tg.Name)
		}
	}
}

// ---------------------------------------------------------------- rewrite

func (p *pairs) fresh() V {
	v := V(len(p.f.Types))
	p.f.Types = append(p.f.Types, "int")
	p.fs = append(p.fs, fact{v: ivTop})
	return v
}

func (p *pairs) emit(op Op, mode Mode, args ...V) V {
	v := p.fresh()
	p.out = append(p.out, Stmt{Op: op, Mode: mode, Args: args, Res: []V{v}})
	return v
}

func (p *pairs) k(c int64) V {
	v := p.fresh()
	p.out = append(p.out, Stmt{Op: OConst, Lit: core.Int(c), Res: []V{v}})
	p.kon[v] = big.NewInt(c)
	return v
}

func (p *pairs) get(v V) V {
	for {
		w, ok := p.sub[v]
		if !ok {
			return v
		}
		v = w
	}
}

func (p *pairs) isK(v V, c int64) bool {
	x, ok := p.kon[p.get(v)]
	return ok && x.IsInt64() && x.Int64() == c
}

// The digits' operations, folding the constants they meet.
// konOf is a value's constant, if known and in int64.
func (p *pairs) konOf(v V) (int64, bool) {
	c, ok := p.kon[p.get(v)]
	if !ok || !c.IsInt64() {
		return 0, false
	}
	return c.Int64(), true
}

func (p *pairs) add(a, b V) V {
	if x, ok := p.konOf(a); ok {
		if y, ok := p.konOf(b); ok && x+y >= x == (y >= 0) {
			return p.k(x + y)
		}
	}
	switch {
	case p.isK(a, 0):
		return b
	case p.isK(b, 0):
		return a
	}
	return p.emit(OAdd, MExact, a, b)
}
func (p *pairs) subt(a, b V) V {
	if p.isK(b, 0) {
		return a
	}
	return p.emit(OSub, MExact, a, b)
}
func (p *pairs) mul(a, b V) V {
	if x, ok := p.konOf(a); ok {
		if y, ok := p.konOf(b); ok && x >= 0 && y >= 0 && x <= 1<<31 && y <= 1<<31 {
			return p.k(x * y)
		}
	}
	switch {
	case p.isK(a, 0) || p.isK(b, 0):
		return p.k(0)
	case p.isK(a, 1):
		return b
	case p.isK(b, 1):
		return a
	}
	return p.emit(OMul, MExact, a, b)
}
func (p *pairs) and(a V, c int64) V {
	if p.isK(a, 0) || c == 0 {
		return p.k(0)
	}
	if x, ok := p.konOf(a); ok && x >= 0 {
		return p.k(x & c)
	}
	return p.emit(OAnd, MExact, a, p.k(c))
}
func (p *pairs) shr(a V, k int64) V {
	if k == 0 || p.isK(a, 0) {
		return a
	}
	if x, ok := p.konOf(a); ok && x >= 0 {
		return p.k(x >> k)
	}
	return p.emit(OShr, MExact, a, p.k(k))
}

// shl32 is (a·2ʲ) mod B, by the target's 32-bit wrapping shift and a mask.
func (p *pairs) shl32(a V, j int64) V {
	if j == 0 || p.isK(a, 0) {
		return a
	}
	return p.emit(OAnd, MExact, p.emit(OShl, MWrap, a, p.k(j)), p.k(pairMask))
}

// lohi is a digit pair's value, a big constant's digits included.
func (p *pairs) toPair(v V, residue bool, what string) (V, V) {
	orig := v
	v = p.get(v)
	if h, ok := p.halves[v]; ok {
		return h[0], h[1]
	}
	if c, ok := p.kon[v]; ok {
		x := new(big.Int).Set(c)
		if residue {
			x.Mod(x, pairTop)
		}
		if x.Sign() < 0 || x.Cmp(pairTop) >= 0 {
			p.fail("%s, %s, is outside the unsigned word it would be held in (ir/pairs.go)", what, x)
			return v, v
		}
		hi := new(big.Int).Rsh(x, 32)
		lo := new(big.Int).And(x, big.NewInt(pairMask))
		return p.k(hi.Int64()), p.k(lo.Int64())
	}
	// THE ORIGINAL VALUE'S FACT: a value rebuilt from a pair's digits is a fresh
	// one, which nothing has bounded yet, and denotes the same integer.
	iv := ivTop
	if int(orig) < p.nv {
		iv = p.fs[orig].v
	} else if int(v) < p.nv {
		iv = p.fs[v].v
	}
	switch {
	case iv.within(ep{}, ei(pairMask)):
		return p.k(0), v
	case iv.within(ep{}, ei(p.tg.Word.Hi)):
		return p.shr(v, 32), p.and(v, pairMask)
	case residue:
		// x mod 2⁶⁴ for an x that may be negative: the 2-adic digits, ⌊x/B⌋
		// and x masked, each reduced mod B
		return p.and(p.shr(v, 32), pairMask), p.and(v, pairMask)
	}
	// THE TRUE REASON, by the interval (book finding 26): a value that may be
	// negative has no pair; a non-negative one above the word was computed by
	// no pair's operations, which is how a value enters the pair, and a
	// parameter there is not built on this target
	if !iv.nlo && iv.lo.sign() >= 0 {
		p.fail("%s, %s, lies above the word target %s holds exactly, [%d, %d], and no operation of the "+
			"unsigned word's pair computed it: a value enters the pair from operations proven in "+
			"[0, 2^64), and a parameter in the unsigned word is not built on %s (ir/pairs.go). Narrow "+
			"the parameter to the word", what, termShow(iv), p.tg.Name, p.tg.Word.Lo, p.tg.Word.Hi, p.tg.Name)
		return v, v
	}
	p.fail("%s, %s, may be negative, and would be held in the unsigned word as a pair; a value there must be "+
		"proven non-negative (ir/pairs.go)", what, termShow(iv))
	return v, v
}

// toS is a pair's value in the word, hi·B + lo, into res: the decision run
// after proves it inside the word or refuses the program.
func (p *pairs) toS(hi, lo V) V {
	if p.isK(hi, 0) {
		return lo
	}
	return p.add(p.mul(hi, p.k(1<<32)), lo)
}

func (p *pairs) arg(v V) V {
	v = p.get(v)
	if h, ok := p.halves[v]; ok {
		return p.toS(h[0], h[1])
	}
	return v
}

func (p *pairs) setResult(r V, hi, lo V) {
	if p.pair[r] {
		p.halves[r] = [2]V{hi, lo}
		return
	}
	p.sub[r] = p.toWord(hi, lo, r)
}

// toWord is the value in the signed word of an operation whose digits are
// its result's residue mod 2⁶⁴, the result r proven in S, [L, H].
//
// THE SIGNED READING (ADR 0151; book finding 114). For v ∈ S₆₄ with residue
// digits (h, l), v = ⌊v/B⌋·B + (v mod B), l = v mod B, and ⌊v/B⌋ ∈ [−2³¹, 2³¹)
// is h read as a 32-bit two's complement: h − B·[h ≥ 2³¹] = h − B·⌊h/2³¹⌋. On
// a non-negative v the correction is 0. Each step is an operation the decision
// counts, on integers below 2³³; what it cannot see is the theorem, that the
// high digit so read is ⌊v/B⌋ ∈ [⌊L/B⌋, ⌊H/B⌋] and the value is v ∈ [L, H],
// so both are ascribed, as a realization ascribes its site's fact (realize.go):
// the expansion is a homomorphism, and d(h, l) = v.
func (p *pairs) toWord(hi, lo, r V) V {
	iv := ivTop
	if int(r) < p.nv {
		iv = p.fs[r].v
	}
	if p.isK(hi, 0) || iv.bot || !iv.within(ei(p.tg.Word.Lo), ei(p.tg.Word.Hi)) || PlantPairWordUnascribed {
		return p.toS(hi, lo)
	}
	L, H := iv.lo.big(), iv.hi.big()
	hs := hi
	if L.Sign() < 0 && !PlantPairUnsignedReading {
		hs = p.subt(hi, p.mul(p.shr(hi, 31), p.k(1<<32)))
	}
	q := func(x *big.Int) *big.Int { return new(big.Int).Div(x, pairB) } // Euclid's, the floor for B > 0
	hs = p.the(hs, q(L), q(H))
	return p.the(p.add(p.mul(hs, p.k(1<<32)), lo), L, H)
}

// the is v ascribed [lo, hi], in the word.
func (p *pairs) the(v V, lo, hi *big.Int) V {
	t := p.fresh()
	p.out = append(p.out, Stmt{Op: OThe, Type: "int " + lo.String() + " " + hi.String(), Args: []V{v}, Res: []V{t}})
	return t
}

// PlantPairUnsignedReading reads a result in the word from its residue's
// digits unsigned: x − y = −3 is then 2⁶⁴ − 3, ascribed −3..−3, a wrong answer
// (TestAPairLeavesForTheIntegers).
var PlantPairUnsignedReading bool

// PlantPairWordUnascribed rebuilds a result in the word with no ascription, as
// before ADR 0151: the decision then refuses hi·B + lo, which it sees reach
// 2⁶⁴ (TestAPairLeavesForTheIntegers).
var PlantPairWordUnascribed bool

func (p *pairs) rewrite() {
	f := p.f
	var region func(r *Region, yield, brk, cont []bool, fn bool)
	region = func(r *Region, yield, brk, cont []bool, fn bool) {
		var params []V
		for _, x := range r.Params {
			if p.pair[x] {
				hi, lo := p.fresh(), p.fresh()
				p.halves[x] = [2]V{hi, lo}
				params = append(params, hi, lo)
			} else {
				params = append(params, x)
			}
		}
		r.Params = params
		var pis []Pi
		for _, pi := range r.Pis {
			if p.pair[pi.V] || p.pair[pi.Of] || p.pair[pi.Other] {
				if h, ok := p.halves[p.get(pi.Of)]; ok && p.pair[pi.V] {
					p.halves[pi.V] = h
				} else if !p.pair[pi.V] {
					p.sub[pi.V] = p.arg(pi.Of)
				}
				continue // a π on a pair carries no fact the digits keep
			}
			pi.Of, pi.Other = p.get(pi.Of), p.get(pi.Other)
			pis = append(pis, pi)
		}
		r.Pis = pis
		outer := p.out
		p.out = nil
		for i := range r.Stmts {
			s := r.Stmts[i]
			p.stmt(&s, region, brk, cont)
		}
		terms := func(args []V, flags []bool, fnRes bool) []V {
			var out []V
			for j, a := range args {
				switch {
				case fnRes && j < len(flags) && flags[j]:
					hi, lo := p.toPair(a, false, "a result")
					v := p.fresh()
					f.Types[v] = core.BigType
					p.out = append(p.out, Stmt{Op: OCall, Name: "pair-bigint", Args: []V{hi, lo}, Res: []V{v}})
					out = append(out, v)
				case j < len(flags) && flags[j]:
					hi, lo := p.toPair(a, false, "a joined value")
					out = append(out, hi, lo)
				default:
					out = append(out, p.arg(a))
				}
			}
			return out
		}
		switch r.T {
		case TYield:
			r.Args = terms(r.Args, yield, fn)
		case TBreak:
			r.Args = terms(r.Args, brk, false)
		case TContinue:
			r.Args = terms(r.Args, cont, false)
		case TBranch:
			r.Cond = p.get(r.Cond)
		}
		r.Stmts = p.out
		p.out = outer
		if r.T == TBranch {
			region(r.Then, yield, brk, cont, fn)
			region(r.Else, yield, brk, cont, fn)
		}
	}
	region(f.Body, p.resPar, nil, nil, true)
	// THE RESULTS' FLAGS ARE APPLIED ON THE WAY OUT: the function body's yields
	// were written with the pass's own flags (yield above is the results').
	for j, b := range p.resPar {
		if b {
			f.Results[j] = core.BigType
		}
	}
}

// flagsOf is, for each value, whether it is held as a pair.
func (p *pairs) flagsOf(vs []V) []bool {
	out := make([]bool, len(vs))
	for i, v := range vs {
		out[i] = p.pair[v]
	}
	return out
}

func (p *pairs) expand(vs []V) []V {
	var out []V
	for _, v := range vs {
		if p.pair[v] {
			hi, lo := p.fresh(), p.fresh()
			p.halves[v] = [2]V{hi, lo}
			out = append(out, hi, lo)
		} else {
			out = append(out, v)
		}
	}
	return out
}

func (p *pairs) stmt(s *Stmt, region func(r *Region, yield, brk, cont []bool, fn bool), brk, cont []bool) {
	any := false
	for _, a := range s.Args {
		if p.pair[p.get(a)] {
			any = true
		}
	}
	for _, v := range s.Res {
		if p.pair[v] {
			any = true
		}
	}
	switch s.Op {
	case OIf:
		for j := range s.Args {
			s.Args[j] = p.get(s.Args[j])
		}
		flags := p.flagsOf(s.Res)
		s.Res = p.expand(s.Res)
		p.out = append(p.out, *s)
		for _, sub := range s.Sub {
			region(sub, flags, brk, cont, false)
		}
		return
	case OLoop:
		body := s.Sub[0]
		cflags := p.flagsOf(body.Params)
		var inits []V
		for j, a := range s.Args {
			if j < len(cflags) && cflags[j] {
				hi, lo := p.toPair(a, false, "a loop's initial value")
				inits = append(inits, hi, lo)
			} else {
				inits = append(inits, p.arg(a))
			}
		}
		s.Args = inits
		rflags := p.flagsOf(s.Res)
		s.Res = p.expand(s.Res)
		p.out = append(p.out, *s)
		region(body, rflags, rflags, cflags, false)
		return
	case OBuild, OTabulate, OBuildMap:
		if any {
			p.fail("a value held as a pair enters or leaves a build's region, which is not built (ir/pairs.go)")
		}
		for j := range s.Args {
			s.Args[j] = p.arg(s.Args[j])
		}
		p.out = append(p.out, *s)
		for _, sub := range s.Sub {
			region(sub, p.flagsOf(s.Res), nil, nil, false)
		}
		return
	}
	if !any {
		for j := range s.Args {
			s.Args[j] = p.get(s.Args[j])
		}
		p.out = append(p.out, *s)
		return
	}
	if s.Op == OCall {
		p.call(s)
		return
	}
	if s.Op.IsCmp() && len(s.Args) == 2 && len(s.Res) == 1 {
		p.compare(s)
		return
	}
	if !s.Op.IsArith() || len(s.Res) != 1 {
		for j := range s.Args {
			s.Args[j] = p.arg(s.Args[j])
		}
		p.out = append(p.out, *s)
		return
	}
	p.arith(s)
}

// compare is a comparison of two values of U, each its digits: the order of U
// is the lexicographic order of (h, l), since d(h, l) = h·B + l with l < B, so
// x ⋈ y is xₗ ⋈ yₗ where xₕ = yₕ and xₕ ⋈′ yₕ elsewhere, ⋈′ the strict order ⋈
// refines (= and ≠ answer false and true there). Each digit comparison is the
// word's, on integers below 2³².
func (p *pairs) compare(s *Stmt) {
	xh, xl := p.toPair(s.Args[0], false, "an operand of "+s.Op.String())
	yh, yl := p.toPair(s.Args[1], false, "an operand of "+s.Op.String())
	if p.err != nil {
		return
	}
	boolean := func() V {
		v := p.fresh()
		p.f.Types[v] = "bool"
		return v
	}
	eq, lo := boolean(), boolean()
	p.out = append(p.out, Stmt{Op: OEq, Args: []V{xh, yh}, Res: []V{eq}})
	then := &Region{T: TYield, Stmts: []Stmt{{Op: s.Op, Args: []V{xl, yl}, Res: []V{lo}}}, Args: []V{lo}}
	hi := boolean()
	var at Stmt
	switch s.Op {
	case OEq, ONe:
		at = Stmt{Op: OConst, Lit: core.Bool(s.Op == ONe), Res: []V{hi}}
	case OLt, OLe:
		at = Stmt{Op: OLt, Args: []V{xh, yh}, Res: []V{hi}}
	default:
		at = Stmt{Op: OGt, Args: []V{xh, yh}, Res: []V{hi}}
	}
	if PlantPairCompareLowOnly {
		at = Stmt{Op: s.Op, Args: []V{xl, yl}, Res: []V{hi}}
	}
	els := &Region{T: TYield, Stmts: []Stmt{at}, Args: []V{hi}}
	p.out = append(p.out, Stmt{Op: OIf, Args: []V{eq}, Sub: []*Region{then, els}, Res: s.Res})
}

// PlantPairCompareLowOnly compares two pairs by their low digits alone, where
// the high ones differ too: 2³² < 1 is then true (TestAPairLeavesForTheIntegers).
var PlantPairCompareLowOnly bool

// call: the residue map into U is the identity on a pair; a pair at a host's
// argument is boxed where the argument takes any value, and its value in the
// word elsewhere.
func (p *pairs) call(s *Stmt) {
	if s.Name == "u64-of" && len(s.Args) == 1 && len(s.Res) == 1 {
		hi, lo := p.toPair(s.Args[0], true, "a residue")
		p.setResult(s.Res[0], hi, lo)
		return
	}
	// THE INCLUSION ι : U ↪ ℤ (ADR 0151): the rung widens a value of U it meets,
	// a residue or a value the rung did not take, by `big-of`, whose premise S
	// the pair does not meet; on its digits it is d(h, l) = h·B + l, held exactly
	if s.Name == "big-of" && len(s.Args) == 1 && len(s.Res) == 1 {
		if h, ok := p.halves[p.get(s.Args[0])]; ok {
			p.out = append(p.out, Stmt{Op: OCall, Name: "pair-bigint", Args: []V{h[0], h[1]}, Res: s.Res})
			return
		}
	}
	q := p.tg.Prims[s.Name]
	// A STATEMENT'S VALUE IS ITS FIRST ARGUMENT (ir.md): where that is a pair,
	// so is the statement's value, the same digits, and the statement itself
	// gets a fresh value for the boxed argument it now takes.
	if q.Kind == "stmt" && len(s.Res) == 1 && len(s.Args) > 0 && p.pair[s.Res[0]] {
		if h, ok := p.halves[p.get(s.Args[0])]; ok {
			p.halves[s.Res[0]] = h
			s.Res = []V{p.fresh()}
		}
	}
	for j, a := range s.Args {
		a = p.get(a)
		h, ok := p.halves[a]
		if !ok {
			s.Args[j] = a
			continue
		}
		if j < len(q.Args) && (q.Args[j] == "any" || q.Args[j] == "Any" || q.Kind == "stmt" && q.Args[j] == "") {
			box, found := p.tg.Prims["pair-box"]
			if !found {
				p.fail("target %s declares no pair-box for a pair at a host's argument", p.tg.Name)
				return
			}
			v := p.fresh()
			p.f.Types[v] = box.Result
			p.out = append(p.out, Stmt{Op: OCall, Name: "pair-box", Args: []V{h[0], h[1]}, Res: []V{v}})
			s.Args[j] = v
			continue
		}
		s.Args[j] = p.toS(h[0], h[1])
	}
	p.out = append(p.out, *s)
}

func (p *pairs) arith(s *Stmt) {
	r := s.Res[0]
	// A CLOSED CONSTANT is its digits (a literal past int64 is a spine of words).
	if c, ok := p.kon[r]; ok && p.pair[r] {
		hi, lo := p.toPair(r, s.Mode == MWrapPair, "a constant")
		_ = c
		p.halves[r] = [2]V{hi, lo}
		return
	}
	wrap := s.Mode == MWrapPair
	res := p.pair[r] || wrap
	a := s.Args[0]
	// A RING OR BIT OPERATION'S OPERANDS BY THEIR RESIDUES: ℤ → ℤ/2⁶⁴ is a ring
	// homomorphism, and the bits of a residue are the integer's low bits, so
	// the result's digits are its residue's; read in U where it is a pair, in
	// S by the signed reading where it is not (toWord)
	operand := func(v V) (V, V) {
		return p.toPair(v, true, "an operand of "+s.Op.String())
	}
	constArg := func(v V) (int64, bool) {
		c, ok := p.kon[p.get(v)]
		if !ok || !c.IsInt64() {
			return 0, false // not a constant: the caller refuses, it never reads c
		}
		return c.Int64(), true
	}
	switch s.Op {
	case OAdd, OSub:
		aH, aL := operand(a)
		bH, bL := operand(s.Args[1])
		var t, h V
		if s.Op == OAdd {
			t = p.add(aL, bL)
			h = p.add(p.add(aH, bH), p.shr(t, 32))
		} else {
			t = p.subt(aL, bL)
			h = p.add(p.subt(aH, bH), p.shr(t, 32))
		}
		// the high digit masked in either mode: wrapped, it is the residue; exact,
		// the result is proven in U, so the mask is the identity on it, and gives
		// the analysis the digit's bound
		p.setResult(r, p.and(h, pairMask), p.and(t, pairMask))
		return
	case ONeg:
		if !wrap {
			break
		}
		aH, aL := operand(a)
		t := p.subt(p.k(0), aL)
		h := p.and(p.add(p.subt(p.k(0), aH), p.shr(t, 32)), pairMask)
		p.setResult(r, h, p.and(t, pairMask))
		return
	case OMul:
		aH, aL := operand(a)
		bH, bL := operand(s.Args[1])
		a0, a1 := p.and(aL, 0xFFFF), p.shr(aL, 16)
		b0, b1 := p.and(bL, 0xFFFF), p.shr(bL, 16)
		mid := p.add(p.mul(a0, b1), p.mul(a1, b0))
		t := p.add(p.mul(a0, b0), p.mul(p.and(mid, 0xFFFF), p.k(1<<16)))
		lo := p.and(t, pairMask)
		hiLL := p.add(p.add(p.mul(a1, b1), p.shr(mid, 16)), p.shr(t, 32))
		// the cross terms mod B, by the target's 32-bit wrapping multiply
		cross := hiLL
		for _, xy := range [][2]V{{aH, bL}, {aL, bH}} {
			if p.isK(xy[0], 0) || p.isK(xy[1], 0) {
				continue
			}
			m := p.emit(OMul, MWrap, xy[0], xy[1])
			cross = p.emit(OAdd, MWrap, cross, m)
		}
		p.setResult(r, p.and(cross, pairMask), lo)
		return
	case OAnd, OOr, OXor:
		aH, aL := operand(a)
		bH, bL := operand(s.Args[1])
		dig := func(x, y V) V {
			switch {
			case s.Op == OAnd && (p.isK(x, 0) || p.isK(y, 0)):
				return p.k(0)
			case s.Op != OAnd && p.isK(x, 0):
				return y
			case s.Op != OAnd && p.isK(y, 0):
				return x
			}
			return p.emit(s.Op, MExact, x, y)
		}
		p.setResult(r, dig(aH, bH), dig(aL, bL))
		return
	case ONot:
		if !wrap {
			break
		}
		aH, aL := operand(a)
		p.setResult(r, p.subt(p.k(pairMask), aH), p.subt(p.k(pairMask), aL))
		return
	case ODiv, ORem, OMod:
		if p.smallByLarge(s) {
			if s.Op == ODiv {
				p.sub[r] = p.k(0)
			} else {
				p.sub[r] = p.get(a)
			}
			return
		}
	}
	switch s.Op {
	case OShl, OShr, ODiv, ORem, OMod:
		c, ok := constArg(s.Args[1])
		if !ok || c < 0 {
			p.fail("%s of a value held in the unsigned word as a pair, by %s, not a constant: not built (ir/pairs.go)",
				s.Op, termShow(p.fs[p.get(s.Args[1])].v))
			return
		}
		aH, aL := p.toPair(a, s.Op == OShl && (wrap || res), "an operand of "+s.Op.String())
		k := c
		op := s.Op
		if op == ODiv || op == ORem || op == OMod {
			if c == 0 || c&(c-1) != 0 {
				p.fail("%s of a value held in the unsigned word as a pair, by %d, not a power of two: not built "+
					"(ir/pairs.go)", s.Op, c)
				return
			}
			k = 0
			for x := c; x > 1; x >>= 1 {
				k++
			}
		}
		switch op {
		case OShl:
			switch {
			case k >= 64:
				p.setResult(r, p.k(0), p.k(0))
			case k >= 32:
				p.setResult(r, p.shl32(aL, k-32), p.k(0))
			default:
				hi := aH
				if k > 0 {
					hi = p.and(p.emit(OOr, MExact, p.shl32(aH, k), p.shr(aL, 32-k)), pairMask)
				}
				p.setResult(r, hi, p.shl32(aL, k))
			}
		case OShr, ODiv:
			switch {
			case k >= 64:
				p.setResult(r, p.k(0), p.k(0))
			case k >= 32:
				p.setResult(r, p.k(0), p.shr(aH, k-32))
			default:
				lo := aL
				if k > 0 {
					lo = p.and(p.emit(OOr, MExact, p.shr(aL, k), p.shl32(aH, 32-k)), pairMask)
				}
				p.setResult(r, p.shr(aH, k), lo)
			}
		default: // the remainder by 2ᵏ, on a non-negative value Euclidean and truncating alike
			switch {
			case k >= 64:
				p.setResult(r, aH, aL)
			case k >= 32:
				p.setResult(r, p.and(aH, 1<<(k-32)-1), aL)
			default:
				p.setResult(r, p.k(0), p.and(aL, 1<<k-1))
			}
		}
		return
	}
	// AN OPERATION WHOSE RESULT IS IN THE WORD takes its operands' values there.
	if p.pair[r] {
		p.fail("%s with a result in the unsigned word, held as a pair, is not built (ir/pairs.go)", s.Op)
		return
	}
	for j := range s.Args {
		s.Args[j] = p.arg(s.Args[j])
	}
	p.out = append(p.out, *s)
}

// PlantPairResultAnySet returns a result held as a pair whatever its declared
// set, as before: an export declared Int then returns a BigInt on JavaScript
// (TestAnEntryPointsResultIsItsDeclaredSets).
var PlantPairResultAnySet bool
