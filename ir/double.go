package ir

import (
	"fmt"
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// THE DOUBLE WORD (ADR 0103, llvm.md §3c).
//
// The residue map r: ℤ → ℤ/2¹²⁸ is a ring homomorphism, injective on
// (−2¹²⁷, 2¹²⁷). So a term built from +, −, · and neg whose every
// intermediate the decision proves in E = (−2¹²⁶, 2¹²⁶) is computed exactly
// in i128: by induction each step's residue is its value's, and the lift of a
// residue in E is unique. ADR 0082's fusion reads a term through `mod 2ⁿ`, a
// homomorphism, and so ignores its intermediates; this reads one through an
// injection, and so needs them in E, which the decision proves.
//
// What reads an intermediate back into the word is a quotient, a remainder, a
// `mod` or a right shift by a positive constant, a comparison, or + − · whose
// result the decision proves in the word: each is a function of the
// intermediate's integer, computed on its i128 representative. So no value of
// the program leaves the word: an intermediate is never a table's element, a
// call's argument, a loop's variable, a π or a result, and needs no claim
// above the word (ADR 0079). A divisor is a constant because a variable one
// is a library call on LLVM (__udivti3); a quotient by a run-time word is the
// library's, by its reciprocal (Möller and Granlund 2011).
//
// SelectDouble is the greatest fixed point: the candidates are the unproven
// +, −, · and neg whose facts are finite, and a candidate is dropped while one
// of its uses is not one of the above or one of its operands is neither a word
// nor a candidate. It reports how many operations it marks MDouble; the
// decision is to be taken again, which proves each in E.
func SelectDouble(tg *emit.Target, f *Func) int {
	if tg.DoubleWidth != 128 {
		return 0
	}
	fs := analyse(tg, f)
	wLo, wHi := ei(tg.Word.Lo), ei(tg.Word.Hi)
	inWord := func(v V) bool { return fs[v].v.within(wLo, wHi) }
	// inU is the unsigned word's, which holds its own values (ADR 0033)
	inU := func(v V) bool { return fs[v].v.within(ei(0), u64Max) }
	// back says a reading-back's result is a word's, signed or unsigned
	back := func(v V) bool { return inWord(v) || inU(v) }
	def := map[V]*Stmt{}
	// a use is a statement's operand j, a π (the same integer refined by its
	// arm's guard, whose own uses are then the value's), or anything else
	// or an exit's argument flowing into a join (a loop's variable or result),
	// which is the double word's where the join is (join ≥ 0)
	type use struct {
		s    *Stmt
		j    int
		pi   V
		join V
	}
	uses := map[V][]use{}
	// THE JOINS (ADR 0153): a loop's variable takes its initial argument and
	// each `again`'s, its result each break's. r : ℤ → ℤ/2¹²⁸ commutes with that
	// choice, r(if c then a else b) = if c then r(a) else r(b), so a join whose
	// inputs are all words or held in E is carried as its residue; its inputs
	// are joinIn's. Every other exit (a function's yield, a scope's) is a use
	// that leaves E, as before.
	joinIn := map[V][]V{}
	flow := func(args []V, into []V) {
		for j, a := range args {
			if j < len(into) {
				uses[a] = append(uses[a], use{pi: -1, join: into[j]})
				joinIn[into[j]] = append(joinIn[into[j]], a)
			} else {
				uses[a] = append(uses[a], use{pi: -1, join: -1})
			}
		}
	}
	leave := func(args []V) {
		for _, a := range args {
			uses[a] = append(uses[a], use{pi: -1, join: -1})
		}
	}
	for _, args := range ownExits(f.Body, TYield) {
		leave(args)
	}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch {
			case s.Op == OLoop && len(s.Sub) == 1 && !PlantDoubleNoJoins:
				body := s.Sub[0]
				flow(s.Args, body.Params)
				for _, args := range ownExits(body, TContinue) {
					flow(args, body.Params)
				}
				for _, args := range ownExits(body, TBreak) {
					flow(args, s.Res)
				}
			case s.Op == OLoop && len(s.Sub) == 1:
				for _, args := range ownExits(s.Sub[0], TContinue) {
					leave(args)
				}
				for _, args := range ownExits(s.Sub[0], TBreak) {
					leave(args)
				}
				leave(s.Args)
			default:
				for _, sub := range s.Sub {
					for _, args := range ownExits(sub, TYield) {
						leave(args)
					}
				}
			}
		}
	})
	f.Walk(func(r *Region) {
		if r.T == TBranch {
			uses[r.Cond] = append(uses[r.Cond], use{pi: -1, join: -1})
		}
		for _, pi := range r.Pis {
			// the π's other side is the comparison's, read by no instruction
			uses[pi.Of] = append(uses[pi.Of], use{pi: pi.V, join: -1})
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OLoop { // a loop's arguments are its variables' inputs (flow)
				for j, a := range s.Args {
					uses[a] = append(uses[a], use{s: s, j: j, pi: -1, join: -1})
				}
			}
			for _, v := range s.Res {
				def[v] = s
			}
		}
	})
	ring := func(s *Stmt) bool {
		return (s.Op == OAdd || s.Op == OSub || s.Op == OMul || s.Op == ONeg || s.Op == OShr) && s.Name == "" && len(s.Res) == 1
	}
	cand := map[V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			// unproven is its fact outside the word, whatever placeholder mode it
			// carries (`exact` without -checked, `trap` with it)
			if ring(s) && s.Mode != MWrap && s.Mode != MWrapPair && s.Via == "" &&
				(inDouble(fs[s.Res[0]].v) || PlantDoubleUnbounded) && !inWord(s.Res[0]) && !inU(s.Res[0]) {
				cand[s.Res[0]] = true
			}
		}
	})
	// a join is a candidate where its fact is the double word's and neither
	// word's, as an operation is
	for v := range joinIn {
		if x := fs[v].v; (inDouble(x) || PlantDoubleUnbounded) && !inWord(v) && !inU(v) {
			cand[v] = true
		}
	}
	if len(cand) == 0 {
		return 0
	}
	positive := func(v V, max int64) bool {
		d := def[v]
		return d != nil && d.Op == OConst && d.Lit != nil && d.Lit.Kind == core.KInt && d.Lit.Int > 0 && d.Lit.Int <= max
	}
	// an operand of a double-word computation: a word, signed or unsigned
	// (a natural's zero extension is exact), or a candidate
	operand := func(v V) bool { return cand[v] || inWord(v) || inU(v) }
	// THE 2-BY-1 DIVISION, where the triple has one (targets/windows/x64.oro):
	// a natural by a positive word whose quotient the decision proves in the
	// word (the two-digit lemma, numeralQuotient), its remainder by the same
	// proof. quot holds the (dividend, divisor) pairs so proven.
	twoByOne := HasTwoByOne(tg)
	nat := func(v V) bool { x := fs[v].v; return x.finite() && !x.lo.lt(ei(0)) }
	quot := map[[2]V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == ODiv && len(s.Res) == 1 && inU(s.Res[0]) {
				quot[[2]V{s.Args[0], s.Args[1]}] = true
			}
		}
	})
	byWord := func(s *Stmt) bool {
		d := s.Args[1]
		if !twoByOne || cand[d] || !fs[d].v.within(ei(1), u64Max) || !nat(s.Args[0]) {
			return false
		}
		return quot[[2]V{s.Args[0], d}]
	}
	var allowed func(u use) bool
	allowed = func(u use) bool {
		s := u.s
		if PlantDoubleEscape {
			return true
		}
		if u.join >= 0 { // into a join: the double word's where the join is
			return cand[u.join]
		}
		if u.pi >= 0 { // a π's uses are its source's
			for _, w := range uses[u.pi] {
				if !allowed(w) {
					return false
				}
			}
			return true
		}
		if s == nil || s.Name != "" || s.Via != "" || len(s.Res) != 1 {
			return false
		}
		for _, a := range s.Args {
			if !operand(a) {
				return false
			}
		}
		switch s.Op {
		case OAdd, OSub, OMul, ONeg:
			return cand[s.Res[0]] || back(s.Res[0])
		case ODiv, ORem, OMod:
			if u.j == 0 && (s.Op == ODiv || s.Op == ORem) && !positive(s.Args[1], 1<<62) {
				return byWord(s) && back(s.Res[0])
			}
			// BY A CONSTANT, on every host: a shift by 2ᵏ; any other d LLVM's
			// unsigned division of the magnitude (ir/llvm, divMagnitude), Go's and
			// the JVM's 2-by-1 division with Horner's rule for a remainder
			return u.j == 0 && positive(s.Args[1], 1<<62) && back(s.Res[0])
		case OAnd:
			// t & m for m >= 0, a word or the unsigned word's (2⁶⁴ − 1 is no
			// literal): in [0, m] whatever m, and t mod 2ᵏ where m = 2ᵏ − 1;
			// ℤ's & is 2-adic, the same on its i128 residue
			m := s.Args[1-u.j]
			return !cand[m] && inU(m) && back(s.Res[0])
		case OShr:
			// by a count in [0, 127], a constant or a word: `lshr i128` by a
			// register is an instruction, not a call; the result an intermediate
			// or back in the word
			k := s.Args[1]
			return u.j == 0 && !cand[k] && fs[k].v.within(ei(0), ei(127)) && (cand[s.Res[0]] || back(s.Res[0]))
		case OEq, ONe, OLt, OLe, OGt, OGe:
			// one reading for both sides: naturals compared unsigned, or both
			// inside (−2¹²⁷, 2¹²⁷) compared signed
			a, b := fs[s.Args[0]].v, fs[s.Args[1]].v
			return a.within(ei(0), u128) && b.within(ei(0), u128) || a.within(s128.neg(), s128) && b.within(s128.neg(), s128) ||
				PlantDoubleUnbounded
		}
		return false
	}
	for changed := true; changed; {
		changed = false
		for v := range cand {
			ok := true
			in := joinIn[v]
			if d := def[v]; d != nil {
				in = d.Args
			}
			for _, a := range in {
				if !operand(a) {
					ok = false
				}
			}
			for _, u := range uses[v] {
				if !ok || !allowed(u) {
					ok = false
					break
				}
			}
			if !ok {
				delete(cand, v)
				changed = true
			}
		}
	}
	// each marked, and typed by its proof in E, which the printer reads: `nuw`,
	// and LLVM's unsigned quotient of a natural
	for v := range cand {
		if d := def[v]; d != nil {
			d.Mode = MDouble
		}
		// A JOIN IS MARKED BY ITS TYPE (ADR 0153): an `int` past both words is
		// no word's and no rung's (ℤ's type is `big`), so the text says the
		// double word holds it, and Doubles reads it so
		if r := fs[v].v; r.finite() {
			f.Types[v] = fmt.Sprintf("int %d %d", r.lo, r.hi)
		}
	}
	// AND ITS πs, the same integers refined by their arms' guards: typed as
	// their sources, which W6 asks of a π
	for grew := len(cand) > 0; grew; {
		grew = false
		f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				if (cand[pi.Of] || Doubles(f)[pi.Of]) && f.Types[pi.V] != f.Types[pi.Of] {
					f.Types[pi.V] = f.Types[pi.Of]
					grew = true
				}
			}
		})
	}
	// A QUOTIENT OR REMAINDER BY A WORD reads an intermediate back by the
	// triple's 2-by-1 division, whose domain is its quotient in the word: typed
	// by its proven range, which W14 reads
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if (s.Op == ODiv || s.Op == ORem) && len(s.Res) == 1 && cand[s.Args[0]] && !positive(s.Args[1], 1<<62) {
				if x := fs[s.Res[0]].v; x.finite() {
					f.Types[s.Res[0]] = fmt.Sprintf("int %d %d", x.lo, x.hi)
				}
			}
		}
	})
	if len(cand) > 0 {
		f.facts = nil
	}
	return len(cand)
}

// inDouble says an interval is the double word's: inside one of i128's two
// readings, (−2¹²⁷, 2¹²⁷) signed or [0, 2¹²⁸) unsigned. The residue map
// ℤ → ℤ/2¹²⁸ is injective on any interval of 2¹²⁸ integers, so either is exact
// (ADR 0105); each value is read by its own fact.
func inDouble(x iv) bool {
	return x.finite() && (x.within(s128.neg(), s128) || x.within(ei(0), u128))
}

// RecombineYields rewrites a yield that must be ℤ's, of a value the double word
// can hold, as that value's recombination from three digits each in the signed
// word (ADR 0153): by Euclid's identity in mixed radix 2³², for every integer v,
//
//	v = ⌊v/2⁶⁴⌋·2⁶⁴ + (⌊v/2³²⌋ mod 2³²)·2³² + (v mod 2³²)
//
// (let q = ⌊v/2³²⌋: v = q·2³² + (v mod 2³²), q = ⌊q/2³²⌋·2³² + (q mod 2³²), and
// ⌊q/2³²⌋ = ⌊v/2⁶⁴⌋, floors nesting), `>>` the floor and `mod` the remainder in
// [0, m). Each digit is a read-back of the double word, by a shift or a `mod`
// of a constant, so v's uses are E's and a loop carrying it keeps it there; the
// recombination's four operations, past the word into a result in ℤ, are the
// rung's. Before, the result's demand reached back through the join and held
// the loop's sum in ℤ: `upto`'s s ≤ 2⁸⁰, a *big.Int each step. Applied only
// where it pays, a clone's selection holding v in E with it. Three digits and
// not two: 2⁶⁴ is no literal, and a digit in U does not widen into the rung on
// every host.
// PlantRecombinePoints recombines a yield whose fact is a point, as before
// finding 120's fix: `export fn lit() (Int) { 2^64 }` is refused on Go, the JVM
// and windows (TestAPointIsItsConstant).
var PlantRecombinePoints bool

func RecombineYields(tg *emit.Target, f *Func, sig *core.Sig) bool {
	// under -words a value past the words is refused, not held in ℤ: the
	// recombination's operations would be refused at the exit as its value is
	if tg.DoubleWidth != 128 || PlantNoRecombine || WordsOnly || len(f.facts) != f.NV() {
		return false
	}
	wLo, wHi := ei(tg.Word.Lo), ei(tg.Word.Hi)
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	// the yields' positions whose result is ℤ's, and the value each carries:
	// a yield itself, or the argument of the ascription it is
	type site struct {
		r *Region
		j int
		s *Stmt // the ascription, or nil
	}
	var sites []site
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == TYield {
			for j, a := range r.Args {
				ty := ""
				if j < len(f.Results) {
					ty = f.Results[j]
				}
				if !resultIsZ(sig, j) && tg.ValueType(ty) != core.BigType && ty != core.BigType {
					continue
				}
				if d := def[a]; d != nil && d.Op == OThe && len(d.Args) == 1 {
					sites = append(sites, site{r, j, d})
				} else {
					sites = append(sites, site{r, j, nil})
				}
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
	val := func(x site) V {
		if x.s != nil {
			return x.s.Args[0]
		}
		return x.r.Args[x.j]
	}
	cands := sites[:0]
	for _, x := range sites {
		v := val(x)
		// A POINT IS ITS CONSTANT (finding 120): a value whose fact is one
		// integer is computed by nothing, so Euclid's recombination, which reads
		// back a value E computes, has nothing to read; the rung holds the
		// closed term exactly as it is. Recombined, the literal 2⁶⁴ returned as
		// `Int` became word operations over the rung's value, unbounded, and Go,
		// the JVM and windows refused the program.
		if fv := f.facts[v].v; inDouble(fv) && !fv.within(wLo, wHi) && !fv.within(ei(0), u64Max) &&
			(fv.lo != fv.hi || PlantRecombinePoints) {
			cands = append(cands, x)
		}
	}
	if len(cands) == 0 {
		return false
	}
	apply := func(g *Func, x site) {
		fresh := func() V { v := V(len(g.Types)); g.Types = append(g.Types, "int"); return v }
		var out []Stmt
		k := func(c int64) V {
			v := fresh()
			out = append(out, Stmt{Op: OConst, Lit: core.Int(c), Res: []V{v}})
			return v
		}
		v := val(x)
		src := sourceOf(g, v)
		op := func(o Op, a, b V) V {
			w := fresh()
			out = append(out, Stmt{Op: o, Args: []V{a, b}, Res: []V{w}, Mode: MExact, Of: src})
			return w
		}
		c32, c64, m := k(32), k(64), k(1<<32)
		h := op(OShr, v, c64)
		l2 := op(OMod, op(OShr, v, c32), m)
		l1 := op(OMod, v, m)
		w := op(OAdd, op(OMul, op(OAdd, op(OMul, h, m), l2), m), l1)
		if x.s != nil {
			x.s.Args[0] = w
		} else {
			x.r.Args[x.j] = w
		}
		x.r.Stmts = append(x.r.Stmts, out...)
	}
	// on a clone first: the rewrite pays only where the selection then holds v
	// in E (its other uses being E's too)
	c := f.Clone()
	cs := make([]site, len(cands))
	cmap := map[*Region]*Region{}
	var pair func(a, b *Region)
	pair = func(a, b *Region) {
		cmap[a] = b
		for i := range a.Stmts {
			for k := range a.Stmts[i].Sub {
				pair(a.Stmts[i].Sub[k], b.Stmts[i].Sub[k])
			}
		}
		if a.T == TBranch {
			pair(a.Then, b.Then)
			pair(a.Else, b.Else)
		}
	}
	pair(f.Body, c.Body)
	vs := make([]V, len(cands)) // each value, before the rewrite replaces it
	for i, x := range cands {
		vs[i] = val(x)
		y := site{cmap[x.r], x.j, nil}
		if x.s != nil {
			y.s = clonedDef(c, x.s.Res[0])
		}
		cs[i] = y
		apply(c, y)
	}
	c.facts = nil
	SelectDouble(tg, c)
	held := Doubles(c)
	done := false
	for i, x := range cands {
		if held[vs[i]] {
			apply(f, x)
			done = true
		}
	}
	if done {
		f.facts = nil
	}
	return done
}

// sourceOf is the source application a value comes from, for a diagnostic: its
// defining statement's provenance, or, for a loop's variable or result, the
// first provenance found back through the joins' inputs (a recombination of
// upto's sum is named `s + i`, not `mul; add`).
func sourceOf(g *Func, v V) *core.Term {
	def := map[V]*Stmt{}
	in := map[V][]V{}
	g.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, w := range s.Res {
				def[w] = s
			}
			if s.Op == OLoop && len(s.Sub) == 1 {
				body := s.Sub[0]
				for j, p := range body.Params {
					if j < len(s.Args) {
						in[p] = append(in[p], s.Args[j])
					}
					for _, args := range ownExits(body, TContinue) {
						if j < len(args) {
							in[p] = append(in[p], args[j])
						}
					}
				}
				for j, rv := range s.Res {
					for _, args := range ownExits(body, TBreak) {
						if j < len(args) {
							in[rv] = append(in[rv], args[j])
						}
					}
				}
			}
		}
	})
	seen := map[V]bool{}
	queue := []V{v}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if seen[w] {
			continue
		}
		seen[w] = true
		if d := def[w]; d != nil {
			if t := provenance(d); t != nil {
				return t
			}
		}
		queue = append(queue, in[w]...)
	}
	return nil
}

// clonedDef is the statement of g defining v.
func clonedDef(g *Func, v V) *Stmt {
	var out *Stmt
	g.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, w := range r.Stmts[i].Res {
				if w == v {
					out = &r.Stmts[i]
				}
			}
		}
	})
	return out
}

// PlantNoRecombine leaves a yield into ℤ of a value E can hold as it is: the
// result's demand then holds the loop's sum in ℤ (TestALoopVariableInEIsTheDoubleWords).
var PlantNoRecombine bool

// pastBothWords says a type is an `int` range outside the signed 64-bit word
// and outside the unsigned one: the double word's, where a join has it.
func pastBothWords(ty string) bool {
	lo, hi, ok := core.IntRangeBig(ty)
	if !ok {
		return false
	}
	inS := lo.Cmp(s64Lo) >= 0 && hi.Cmp(s64Hi) <= 0
	inU := lo.Sign() >= 0 && hi.Cmp(u64Hi) <= 0
	return !inS && !inU
}

var (
	s64Lo = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 63))
	s64Hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(1))
	u64Hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))
)

// TwoByOne is the triple's 2-by-1 division, x86-64's `divq`.
const TwoByOne = "llvm/isa.udiv-2by1"

// HasTwoByOne says the target divides a double word by a word whose quotient
// is a word: an ISA's instruction (TwoByOne), or its printer's own (`repr
// divide host`: Go's bits.Div64, the JVM's Knuth division).
func HasTwoByOne(tg *emit.Target) bool {
	_, ok := tg.Prims[TwoByOne]
	return ok || tg.DoubleDivide == "host"
}

// PlantDoubleEscape lets a double-word intermediate reach any use, a table or
// a result included: TestADoubleWordNeverLeavesItsTerm must see it refused
// without, and accepted with, the fault.
var PlantDoubleEscape bool

// PlantDoubleNoJoins keeps the double word off every join, as before ADR 0153:
// a sum bounded inside E carried by a loop is then held in ℤ
// (TestALoopVariableInEIsTheDoubleWords).
var PlantDoubleNoJoins bool

// PlantDoubleUnbounded takes an intermediate into the double word, and counts
// it proven, whatever its fact: TestAnIntermediateOutsideEIsRefused must see
// the refusal go.
var PlantDoubleUnbounded bool

// Doubles are the values f holds in the double word: those its MDouble
// operations define, the loops' variables and results typed past both words
// (ADR 0153: an `int` there is no word's, and ℤ's type is `big`), and their πs,
// the same integers refined.
func Doubles(f *Func) map[V]bool {
	out := map[V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Mode == MDouble {
				for _, v := range s.Res {
					out[v] = true
				}
			}
			if s.Op == OLoop && len(s.Sub) == 1 {
				for _, v := range append(append([]V(nil), s.Sub[0].Params...), s.Res...) {
					if pastBothWords(f.Types[v]) {
						out[v] = true
					}
				}
			}
		}
	})
	for grew := len(out) > 0; grew; {
		grew = false
		f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				if out[pi.Of] && !out[pi.V] {
					out[pi.V] = true
					grew = true
				}
			}
		})
	}
	return out
}
