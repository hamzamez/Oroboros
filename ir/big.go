package ir

import (
	"fmt"
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// ARBITRARY PRECISION, CHOSEN ON THE IR (ADR 0033 extended to the rung above
// the word, ADR 0019's third escape, ADR 0029's one set).
//
// The representation lattice of an integer value is S ⊑ big, with big
// absorbing: a value is held in the signed word or exactly. Which one is a
// representation, not a type (ADR 0033): the checker's integer sort is ℤ, and
// this pass chooses, on facts, the least set B ⊆ V of values held exactly.
//
// ═══ THE SYSTEM
//
// B is the least set closed under two rules.
//
//	(S) SUPPLY. v ∈ B if v is
//	    - a parameter declared above the word, or a primitive's result
//	      declared `big`;
//	    - a language operation + − · / with an operand in B, or a remainder
//	      a % b with b ∈ B (a % b with a ∈ B and b a word is under |b|, a
//	      word: `big%-small`);
//	    - a join (an `if`'s or a loop's result, a loop parameter) with an
//	      incoming value in B;
//	    - a closed constant term whose value lies outside S (a literal past the
//	      word is a Horner spine of words, core/read.go).
//	(D) DEMAND. A position that requires an exact integer demands its value:
//	    the function's result declared above the word, a primitive's `big`
//	    argument, an ascription above the word, every incoming value of a join
//	    in B, and every operand of an operation in B (both operands of a
//	    comparison with one in B). A demanded v is in B iff fact(v) ⊄ S;
//	    otherwise it stays in the word and is widened where it is used, by the
//	    inclusion S ↪ ℤ that `big-of` realizes.
//
// Both rules are monotone in B over a finite lattice, so B is the least fixed
// point, reached by iteration (Tarski; Kleene iteration terminates on a finite
// height).
//
// The term pass (emit/bigrep.go) had a third rule, PRESSURE: a name a big
// operation reads is promoted when its interval leaves the word. On SSA that
// rule is (D) at an operand position, so it is not a third rule. And its
// syntactic distinction between a NAME, which demand promotes, and an
// EXPRESSION, which demand widens, is replaced by the fact that decides it: a
// value proven inside S is computed in the word and widened, and one that is
// not is computed exactly. The second is the only way it has a value at all.
// The first costs one allocation, where an exact computation costs one per
// operation.
//
// ═══ WHY IT IS SOUND
//
// Each operation of B computes the language's operation in ℤ, exactly. Each
// value outside B is either proven inside S, where the word computes the same
// integer, or is refused by the decision taken again on the selected function.
// A widening is the inclusion, the identity on integers. So the selected
// function computes the integer the unselected one denotes, and the
// interpreter test checks exactly that (big_test.go).
//
// A value of B that reaches a position declared in the word is REFUSED, as the
// term checker refused it: a range above the word is a widening, not a
// refinement, so it is never narrowed silently (unbounded-rung.md §3).
//
// ═══ THE ONE SET (ADR 0029)
//
// When the program declares a finite bound, every operation of B that can grow
// a value is wrapped in `big-fit` or `big-fit-signed` with the bound's bit
// length, so the host's exact integer enforces the set the limb rung enforces.
// A widening is not wrapped: a word lies inside any bound above the word.

// SelectBig chooses which of f's integer values are held exactly and rewrites
// f to match. bits and signed are the program's one set (emit.BigHull): bits
// 0 means no finite bound, which enforces nothing. It reports the number of
// operations computed exactly, or the first obligation that fails.
func SelectBig(tg *emit.Target, f *Func, sig *core.Sig, bits int, signed bool) (int, error) {
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
	}
	g := &bigs{tg: tg, f: f, fs: fs, sLo: ei(tg.Word.Lo), sHi: ei(tg.Word.Hi),
		root: map[V]V{}, b: map[V]bool{}, dem: map[V]bool{}, bits: bits}
	if bits > 0 {
		g.fit = "big-fit"
		if signed {
			g.fit = "big-fit-signed"
		}
	}
	g.collect(sig)
	g.solve()
	// Nothing held exactly and nothing demanded: no widening either.
	if len(g.b) == 0 && len(g.dem) == 0 {
		eraseThe(f)
		return 0, nil
	}
	g.uses = useCounts(f)
	g.rewrite(sig)
	if g.err != nil {
		return 0, g.err
	}
	g.stripPis()
	eraseThe(f)
	// Destinations are the host's bignum's: on the fixed-limb rung a value is a
	// table the instances build, and `big*!` has no instance.
	if tg.HasBigDest() && tg.BigRepr != "limbs" {
		reuseBig(tg, f)
	}
	Canonicalize(f)
	f.facts = nil
	return g.ops, nil
}

type bigs struct {
	tg       *emit.Target
	f        *Func
	fs       []fact
	sLo, sHi ep
	// root identifies a value with the one it renames: a π with its source,
	// a `the` with its operand. They are one object, so one representation.
	root   map[V]V
	b, dem map[V]bool // B, and the demanded values; keyed by root
	bits   int
	fit    string
	// the constraints, collected once
	ariths []*Stmt
	cmps   []*Stmt
	joins  []bigJoin
	err    error
	ops    int
	// the rewrite
	stmts      []Stmt
	loopParams [][]V
	uses       map[V]int
}

// bigJoin is a value that receives several: an `if`'s or a loop's result, a
// loop parameter.
type bigJoin struct {
	v  V
	in []V
}

func (g *bigs) r(v V) V {
	for {
		w, ok := g.root[v]
		if !ok {
			return v
		}
		v = w
	}
}

func (g *bigs) isB(v V) bool { return v >= 0 && g.b[g.r(v)] }

func (g *bigs) bigType(ty string) bool {
	return ty == core.BigType || g.tg.ValueType(ty) == core.BigType
}

// integer reports a value of the integer sort.
func (g *bigs) integer(v V) bool {
	if v < 0 || int(v) >= len(g.f.Types) {
		return false
	}
	ty := g.f.Types[v]
	switch g.tg.ValueType(ty) {
	case "int", core.U64Type, core.BigType:
		return true
	}
	return ty == "u64"
}

func (g *bigs) fits(v V) bool { return g.fs[v].v.within(g.sLo, g.sHi) }

// langOp reports whether an integer operation is the LANGUAGE's. A target's own
// name (`go.+`) is the host's operator with the host's semantics and no
// portability claim, and turning it into a bignum would change what the
// target's name means (emit/bigrep.go, langArith).
func langOp(s *Stmt) bool {
	if s.Src == nil || s.Src.Kind != core.KApp {
		return true
	}
	op := s.Src.Op()
	if op.Kind != core.KName {
		return true
	}
	switch op.Name {
	case "+", "-", "*", "/", "%", "<", "<=", ">", ">=", "=":
		return true
	}
	return false
}

// collect reads the constraints off f once: the operations, the joins, the
// fixed demands and the seeds.
func (g *bigs) collect(sig *core.Sig) {
	f := g.f
	for i, p := range f.Params {
		if g.bigType(f.Types[p]) || (sig != nil && i < len(sig.Params) && g.bigType(sig.Params[i].Type)) {
			g.b[p] = true
		}
	}
	consts := map[V]*big.Int{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			g.root[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch {
			case s.Op == OConst && s.Lit != nil && s.Lit.Kind == core.KInt && len(s.Res) == 1:
				consts[s.Res[0]] = big.NewInt(s.Lit.Int)
			case s.Op.IsArith() && s.Name == "" && len(s.Res) == 1:
				g.ariths = append(g.ariths, s)
				// A CLOSED CONSTANT TERM outside S can only denote an exact
				// value: bounded-by-default refuses it as a word.
				if len(s.Args) == 2 && (s.Op == OAdd || s.Op == OSub || s.Op == OMul) {
					a, aok := consts[s.Args[0]]
					b, bok := consts[s.Args[1]]
					if aok && bok {
						v := new(big.Int)
						switch s.Op {
						case OAdd:
							v.Add(a, b)
						case OSub:
							v.Sub(a, b)
						case OMul:
							v.Mul(a, b)
						}
						consts[s.Res[0]] = v
						if e, ok := fromBig(v); (!ok || !(iv{lo: e, hi: e}).within(g.sLo, g.sHi)) && langOp(s) {
							g.b[s.Res[0]] = true
						}
					}
				}
			case s.Op.IsCmp() && s.Name == "" && len(s.Args) == 2:
				g.cmps = append(g.cmps, s)
			case s.Op == OCall:
				p := g.tg.Prims[s.Name]
				for j, a := range s.Args {
					if j < len(p.Args) && p.Kind != "stmt" && g.bigType(p.Args[j]) {
						g.dem[a] = true
					}
				}
				for j, v := range s.Res {
					ty := p.Result
					if len(p.Results) >= 2 && j < len(p.Results) {
						ty = p.Results[j]
					}
					if g.bigType(ty) {
						g.b[v] = true
					}
				}
			case s.Op == OThe && len(s.Res) == 1 && len(s.Args) == 1:
				g.root[s.Res[0]] = s.Args[0]
				if g.bigType(s.Type) {
					g.dem[s.Args[0]] = true
				}
			case s.Op == OIf && len(s.Sub) == 2:
				for j, v := range s.Res {
					var in []V
					for _, sub := range s.Sub {
						for _, args := range ownExits(sub, TYield) {
							if j < len(args) {
								in = append(in, args[j])
							}
						}
					}
					g.joins = append(g.joins, bigJoin{v, in})
				}
			case s.Op == OLoop && len(s.Sub) == 1:
				body := s.Sub[0]
				conts := ownExits(body, TContinue)
				for j, p := range body.Params {
					in := []V{}
					if j < len(s.Args) {
						in = append(in, s.Args[j])
					}
					for _, args := range conts {
						if j < len(args) {
							in = append(in, args[j])
						}
					}
					g.joins = append(g.joins, bigJoin{p, in})
				}
				breaks := ownExits(body, TBreak)
				for j, v := range s.Res {
					var in []V
					for _, args := range breaks {
						if j < len(args) {
							in = append(in, args[j])
						}
					}
					g.joins = append(g.joins, bigJoin{v, in})
				}
			}
		}
	})
	for j, ty := range f.Results {
		big := g.bigType(ty)
		if sig != nil {
			switch {
			case len(sig.Results) > j:
				big = big || g.bigType(sig.Results[j])
			case j == 0 && len(sig.Results) == 0:
				big = big || g.bigType(sig.Result)
			}
		}
		if big {
			for _, args := range ownExits(f.Body, TYield) {
				if j < len(args) {
					g.dem[args[j]] = true
				}
			}
		}
	}
	// keyed by root from here on
	for _, m := range []map[V]bool{g.b, g.dem} {
		for v := range m {
			if r := g.r(v); r != v {
				m[r] = true
				delete(m, v)
			}
		}
	}
}

// ownExits is the argument lists of a region tree's exits of kind t that
// belong to it: yields stop at every operation owning regions (their yields
// are theirs), and breaks and continues stop at a nested loop.
func ownExits(r *Region, t Term) [][]V {
	var out [][]V
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == t {
			out = append(out, r.Args)
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
		if t == TYield {
			return
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OLoop {
				continue
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
	}
	walk(r)
	return out
}

// promotable is a value that CAN be computed exactly: a language operation or
// a join. Any other integer value is a word by its definition (a word-typed
// parameter or primitive result, a constant, a length) and is widened.
func (g *bigs) solve() {
	promotable := map[V]bool{}
	for _, s := range g.ariths {
		if langOp(s) {
			promotable[g.r(s.Res[0])] = true
		}
	}
	for _, j := range g.joins {
		promotable[g.r(j.v)] = true
	}
	changed := true
	setB := func(v V) {
		v = g.r(v)
		if !g.b[v] {
			g.b[v], changed = true, true
		}
	}
	setD := func(v V) {
		v = g.r(v)
		if !g.dem[v] {
			g.dem[v], changed = true, true
		}
	}
	demanded := func(v V) bool {
		v = g.r(v)
		return g.dem[v] && promotable[v] && !g.fits(v)
	}
	for changed {
		changed = false
		for _, s := range g.ariths {
			if !langOp(s) {
				continue
			}
			v := s.Res[0]
			switch {
			case s.Op == ORem && len(s.Args) == 2:
				if g.isB(s.Args[1]) {
					setB(v)
				}
			default:
				for _, a := range s.Args {
					if g.isB(a) {
						setB(v)
					}
				}
			}
			if demanded(v) {
				setB(v)
			}
			if g.isB(v) {
				for _, a := range s.Args {
					setD(a)
				}
			}
		}
		for _, s := range g.cmps {
			if langOp(s) && (g.isB(s.Args[0]) || g.isB(s.Args[1])) {
				setD(s.Args[0])
				setD(s.Args[1])
			}
		}
		for _, j := range g.joins {
			for _, u := range j.in {
				if g.isB(u) {
					setB(j.v)
				}
			}
			if demanded(j.v) {
				setB(j.v)
			}
			if g.isB(j.v) {
				for _, u := range j.in {
					setD(u)
				}
			}
		}
	}
}

// fresh is a new value of the given type.
func (g *bigs) fresh(ty string) V {
	v := V(len(g.f.Types))
	g.f.Types = append(g.f.Types, ty)
	g.fs = append(g.fs, fact{v: ivTop})
	return v
}

// widen returns v held exactly: v itself when it is in B, otherwise
// `big-of v`, the inclusion S ↪ ℤ, whose premise is that v lies in S.
func (g *bigs) widen(v V, where string) V {
	if v < 0 || g.isB(v) || !g.integer(v) {
		return v
	}
	if !g.fits(v) && g.err == nil {
		g.err = fmt.Errorf("%s is %s, which is not inside target %s's signed word [%d, %d], and only a "+
			"value in the signed word widens into arbitrary precision (`big-of`); compute it exactly, "+
			"or prove it inside the word", where, termShow(g.fs[v].v), g.tg.Name, g.tg.Word.Lo, g.tg.Word.Hi)
	}
	out := g.fresh(core.BigType)
	w := Stmt{Op: OCall, Name: "big-of", Args: []V{v}, Res: []V{out}}
	// A WIDENING OF A VALUE'S ONLY USE sits immediately after the value, where
	// the term's `(big-of e)` put it. It is pure, so any point between the
	// definition and the use is the same program; this one keeps the emitted
	// order the term pass had.
	if g.uses[v] == 1 {
		for k := len(g.stmts) - 1; k >= 0; k-- {
			if len(g.stmts[k].Res) == 1 && g.stmts[k].Res[0] == v && len(g.stmts[k].Sub) == 0 {
				g.stmts = append(g.stmts[:k+1], append([]Stmt{w}, g.stmts[k+1:]...)...)
				return out
			}
		}
	}
	g.stmts = append(g.stmts, w)
	return out
}

// useCounts is how many times each value is read: as an operand, an exit's
// argument, a branch's condition, or a π's source or bound.
func useCounts(f *Func) map[V]int {
	n := map[V]int{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			n[pi.Of]++
			if pi.Other >= 0 {
				n[pi.Other]++
			}
		}
		for _, s := range r.Stmts {
			for _, a := range s.Args {
				n[a]++
			}
		}
		for _, a := range r.Args {
			n[a]++
		}
		if r.T == TBranch {
			n[r.Cond]++
		}
	})
	return n
}

// word is the obligation at a position declared in the word: a value of B
// may not reach it, because a range above the word is a widening and not a
// refinement (unbounded-rung.md §3, and the checker's message before ADR 0033).
func (g *bigs) word(v V, where string) {
	if v < 0 || !g.isB(v) || g.err != nil {
		return
	}
	g.err = fmt.Errorf("%s is held in arbitrary precision, which is WIDER than the word of target %s, %s, "+
		"so on this target it is not an `int` — it is a rung above the host's word (ADR 0026, "+
		"docs/unbounded-rung.md).\n"+
		"  A range above the word is a WIDENING, not a refinement: a value that may leave the machine "+
		"word cannot silently be used where an `int` is required, and this refusal is where that is "+
		"said. Widen the destination, or take the value to a string with `big-str`.",
		where, g.tg.Name, g.tg.Word)
}

var bigArith = map[Op]string{OAdd: "big+", OSub: "big-", OMul: "big*", ODiv: "big/", ORem: "big%", ONeg: "big-"}
var bigCmp = map[Op]string{OEq: "big=", OLt: "big<", OLe: "big<=", OGt: "big>", OGe: "big>="}

func (g *bigs) rewrite(sig *core.Sig) {
	f := g.f
	for _, p := range f.Params {
		if g.isB(p) && !g.bigType(f.Types[p]) {
			f.Types[p] = core.BigType
		}
	}
	recv := make([]bool, len(f.Results))
	for j := range f.Results {
		big := g.bigType(f.Results[j])
		if sig != nil {
			switch {
			case len(sig.Results) > j:
				big = big || g.bigType(sig.Results[j])
			case j == 0 && len(sig.Results) == 0:
				big = big || g.bigType(sig.Result)
			}
		}
		if big {
			f.Results[j] = core.BigType
		}
		recv[j] = big
	}
	g.region(f.Body, recv)
}

// region rebuilds a region; recv is, for each value its yields or breaks
// hand over, whether the receiver holds it exactly (nil: a word receiver).
func (g *bigs) region(r *Region, recv []bool) {
	for _, pi := range r.Pis {
		if g.isB(pi.V) {
			g.f.Types[pi.V] = core.BigType
		}
	}
	outer := g.stmts
	g.stmts = nil
	for i := range r.Stmts {
		s := r.Stmts[i]
		g.stmt(&s)
	}
	exit := func(args []V, recv []bool, what string) {
		for j, a := range args {
			if j < len(recv) && recv[j] {
				args[j] = g.widen(a, what)
			} else {
				g.word(a, what)
			}
		}
	}
	switch r.T {
	case TYield, TBreak:
		exit(r.Args, recv, "a result")
	case TContinue:
		if n := len(g.loopParams); n > 0 {
			ps := g.loopParams[n-1]
			pr := make([]bool, len(ps))
			for j, p := range ps {
				pr[j] = g.isB(p)
			}
			exit(r.Args, pr, "a loop variable")
		}
	}
	r.Stmts = g.stmts
	g.stmts = outer
	if r.T == TBranch {
		g.region(r.Then, recv)
		g.region(r.Else, recv)
	}
}

func (g *bigs) recvOf(res []V) []bool {
	out := make([]bool, len(res))
	for j, v := range res {
		out[j] = g.isB(v)
		if out[j] {
			g.f.Types[v] = core.BigType
		}
	}
	return out
}

func (g *bigs) stmt(s *Stmt) {
	switch {
	case s.Op.IsArith() && s.Name == "" && len(s.Res) == 1:
		v := s.Res[0]
		switch {
		case g.isB(v):
			args := make([]V, 0, 2)
			if s.Op == ONeg {
				zero := g.fresh("int")
				g.stmts = append(g.stmts, Stmt{Op: OConst, Lit: core.Int(0), Res: []V{zero}})
				args = append(args, g.widen(zero, "an operand"))
			}
			for _, a := range s.Args {
				args = append(args, g.widen(a, "an operand"))
			}
			g.ops++
			g.f.Types[v] = core.BigType
			op := Stmt{Op: OCall, Name: bigArith[s.Op], Args: args, Res: []V{v}, Src: s.Src}
			if g.fit == "" {
				g.stmts = append(g.stmts, op)
				return
			}
			t := g.fresh(core.BigType)
			op.Res = []V{t}
			k := g.fresh("int")
			g.stmts = append(g.stmts, op,
				Stmt{Op: OConst, Lit: core.Int(int64(g.bits)), Res: []V{k}},
				Stmt{Op: OCall, Name: g.fit, Args: []V{t, k}, Res: []V{v}})
		case s.Op == ORem && len(s.Args) == 2 && g.isB(s.Args[0]) && langOp(s):
			// THE REMAINDER BY A WORD IS A WORD: |a mod b| < |b|.
			g.ops++
			g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big%-small", Args: s.Args, Res: s.Res, Src: s.Src})
		default:
			for _, a := range s.Args {
				g.word(a, "an operand of a target's own operator")
			}
			g.stmts = append(g.stmts, *s)
		}
	case s.Op.IsCmp() && s.Name == "" && len(s.Args) == 2:
		if !g.isB(s.Args[0]) && !g.isB(s.Args[1]) {
			g.stmts = append(g.stmts, *s)
			return
		}
		name, ok := bigCmp[s.Op]
		if !ok || !langOp(s) {
			g.word(s.Args[0], "an operand of "+s.Op.String())
			g.word(s.Args[1], "an operand of "+s.Op.String())
			g.stmts = append(g.stmts, *s)
			return
		}
		g.ops++
		a := g.widen(s.Args[0], "an operand")
		b := g.widen(s.Args[1], "an operand")
		g.stmts = append(g.stmts, Stmt{Op: OCall, Name: name, Args: []V{a, b}, Res: s.Res, Src: s.Src})
	case s.Op == OCall:
		p := g.tg.Prims[s.Name]
		for j, a := range s.Args {
			if j >= len(p.Args) || p.Kind == "stmt" {
				continue
			}
			what := fmt.Sprintf("argument %d of %s", j+1, s.Name)
			switch ty := p.Args[j]; {
			case g.bigType(ty):
				s.Args[j] = g.widen(a, what)
			case ty == "int" || g.tg.ValueType(ty) == "int" || g.tg.ValueType(ty) == core.U64Type:
				g.word(a, what)
			}
		}
		for _, v := range s.Res {
			if g.isB(v) {
				g.f.Types[v] = core.BigType
			}
		}
		g.stmts = append(g.stmts, *s)
	case s.Op == OThe:
		if len(s.Res) == 1 && g.isB(s.Res[0]) {
			g.f.Types[s.Res[0]] = core.BigType
		}
		g.stmts = append(g.stmts, *s)
	case s.Op == OIf:
		recv := g.recvOf(s.Res)
		g.stmts = append(g.stmts, *s)
		for _, sub := range s.Sub {
			g.region(sub, recv)
		}
	case s.Op == OLoop:
		body := s.Sub[0]
		for j, a := range s.Args {
			if j < len(body.Params) {
				if g.isB(body.Params[j]) {
					s.Args[j] = g.widen(a, "a loop's initial value")
				} else {
					g.word(a, "a loop's initial value")
				}
			}
		}
		for _, p := range body.Params {
			if g.isB(p) {
				g.f.Types[p] = core.BigType
			}
		}
		recv := g.recvOf(s.Res)
		g.stmts = append(g.stmts, *s)
		g.loopParams = append(g.loopParams, body.Params)
		g.region(body, recv)
		g.loopParams = g.loopParams[:len(g.loopParams)-1]
	default:
		// Every other position is the word's: an index, a stored value, a size,
		// a key, a table's element.
		for _, a := range s.Args {
			if g.integer(a) {
				g.word(a, "an operand of "+s.Op.String())
			}
		}
		g.stmts = append(g.stmts, *s)
		for _, sub := range s.Sub {
			g.region(sub, nil)
		}
	}
}

// stripPis removes the π-parameters of a comparison computed exactly. A π
// names an operand of an IR comparison on one arm (spec §6), and a comparison
// that became a call to `big<` is not one. Each is renamed to its source,
// which is the same integer.
func (g *bigs) stripPis() {
	ren := map[V]V{}
	g.f.Walk(func(r *Region) {
		out := r.Pis[:0]
		for _, pi := range r.Pis {
			if g.isB(pi.Of) || g.isB(pi.Other) {
				ren[pi.V] = pi.Of
				continue
			}
			out = append(out, pi)
		}
		r.Pis = out
	})
	renameAway(g.f, ren)
}

// renameAway replaces every use of a key of ren by its value, transitively.
func renameAway(f *Func, ren map[V]V) {
	if len(ren) == 0 {
		return
	}
	m := func(v V) V {
		for {
			w, ok := ren[v]
			if !ok {
				return v
			}
			v = w
		}
	}
	f.Walk(func(r *Region) {
		for i := range r.Pis {
			r.Pis[i].Of, r.Pis[i].Other = m(r.Pis[i].Of), m(r.Pis[i].Other)
		}
		for i := range r.Stmts {
			for j, a := range r.Stmts[i].Args {
				r.Stmts[i].Args[j] = m(a)
			}
		}
		for j, a := range r.Args {
			r.Args[j] = m(a)
		}
		if r.T == TBranch {
			r.Cond = m(r.Cond)
		}
	})
}

// BigOps counts the operations f computes exactly, as the note reports them.
func BigOps(f *Func) int {
	n := 0
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op != OCall {
				continue
			}
			for _, name := range bigArith {
				if s.Name == name || s.Name == name+"!" {
					n++
					goto next
				}
			}
			for _, name := range bigCmp {
				if s.Name == name {
					n++
					goto next
				}
			}
			if s.Name == "big%-small" {
				n++
			}
		next:
		}
	})
	return n
}
