package ir

import (
	"fmt"
	"math/big"
	"strings"

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
//	    - a parameter of the function compiled declared above the word, where
//	      the claim is the fact: an entry point's, whose value the host
//	      passes, or a definition's checked alone at its claim (ADR 0072). A
//	      definition inlined has no parameter left, only its argument's fact,
//	      and its declared sets are claims (ADR 0079);
//	    - a primitive's result declared `big`;
//	    - a language operation + − · / with an operand in B, or a remainder
//	      a % b with b ∈ B (a % b with a ∈ B and b a word is under |b|, a
//	      word: `big%-small`);
//	    - a Boolean operation, a shift or `mod` (ADR 0082) with an operand in
//	      B (a shift's count is a word), unless its fact lies in S: `y & 1`
//	      with y exact is in [0, 1], `x mod m` with m a word in [0, m), each a
//	      word computed exactly and brought back by r (`big-word`), as
//	      `big%-small` is for `%`;
//	    - a join (an `if`'s or a loop's result, a loop parameter) with an
//	      incoming value in B;
//	    - a closed constant term whose value lies outside S (a literal past the
//	      word is a Horner spine of words, core/read.go).
//	(D) DEMAND. A position that requires an exact integer demands its value:
//	    the function's result declared above the word, a primitive's `big`
//	    argument, an ascription above the word (a claim inlined, whose value is
//	    its operand's: ADR 0079), every incoming value of a join
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
	// Nothing held exactly and nothing demanded: no widening either. A demand
	// only a result declared `Int` made, with nothing held exactly, is met by
	// the word: every yield fits.
	if len(g.b) == 0 && (len(g.dem) == 0 || g.onlyResultDemands()) {
		eraseThe(f)
		return 0, nil
	}
	g.uses = useCounts(f)
	g.rewrite(sig)
	if g.err != nil {
		return 0, g.err
	}
	g.stripPis()
	g.fitClaims()
	eraseThe(f)
	if len(f.exactDemand) > 0 && tg.BigRepr == "host" && !PlantRungFactsForgotten {
		g.ascribe()
	}
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
	// resDem are the yields a result declared `Int` demanded, and nothing else
	resDem []V
	// exact are the values that can be held exactly (anyExactable), once
	exact map[V]bool
	bits  int
	fit   string
	// the constraints, collected once
	ariths []*Stmt
	bitops []*Stmt // ADR 0082's operations
	cmps   []*Stmt
	joins  []bigJoin
	err    error
	ops    int
	// the rewrite
	stmts      []Stmt
	loopParams [][]V
	uses       map[V]int
}

// ascribe states, after the rung, the facts its representation hides from the
// decision taken again (ADR 0151). A representation is an abstraction function
// (ADR 0083), so what the first decision proved of a value on ℤ holds of it in
// any representation, as a realization's site keeps its fact (realize.go). A
// value held exactly is a host's, ⊤ to the decision, and a word computed from
// one loses the bound it had: a digit sum's accumulator, bounded by the trip
// count of the loop dividing v by 10, is unbounded once v is a BigInt. So each
// word variable of a loop with a variable held exactly is ascribed its decided
// fact, each retraction r : ℤ ⇀ S its operand's (word's g.fs), and each
// remainder of an exact value by a word its own (v % 10 ∈ [0, 9]). Placed only
// where A held values exactly (f.exactDemand), so no program accepted before
// changes, and only on the host's integer (`repr big host`), where a value
// held exactly is ⊤ to the decision. On K(ℕ*) the rung's values are numerals
// whose library instances the decision reads under their parameters' sets, and
// there the ascriptions are not placed: stein's gcd over Nat on windows, its
// D_A non-empty, compiled in 40 s without them and did not end in an hour with
// them (pairsel-2026-11-21). Everywhere else the rung runs, they are owed.
func (g *bigs) ascribe() {
	the := func(v V) (Stmt, bool) {
		x := g.fs[v].v
		if !x.finite() || x.bot {
			return Stmt{}, false
		}
		return Stmt{Op: OThe, Type: factType(g.tg, x), Res: []V{v}}, true
	}
	g.f.Walk(func(r *Region) {
		for i := 0; i < len(r.Stmts); i++ { // a statement inserted is passed over
			s := &r.Stmts[i]
			switch {
			case s.Op == OLoop && len(s.Sub) == 1:
				body := s.Sub[0]
				exact := false
				for _, p := range body.Params {
					exact = exact || g.f.Types[p] == core.BigType && g.leftPair(p)
				}
				if !exact {
					continue
				}
				var pre []Stmt
				for j, p := range body.Params {
					if int(p) >= len(g.fs) || g.f.Types[p] == core.BigType || !g.integer(p) {
						continue
					}
					a, ok := the(p)
					if !ok {
						continue
					}
					q := g.fresh(g.f.Types[p])
					body.Params[j] = q
					a.Args = []V{q}
					pre = append(pre, a)
					for k := range body.Pis {
						if body.Pis[k].Of == p {
							body.Pis[k].Of = q
						}
						if body.Pis[k].Other == p {
							body.Pis[k].Other = q
						}
					}
				}
				body.Stmts = append(pre, body.Stmts...)
			case s.Op == OCall && (s.Name == "big-word" || s.Name == "big%-small") && len(s.Res) == 1 &&
				int(s.Res[0]) < len(g.fs) && len(s.Args) > 0 && g.leftPair(s.Args[0]):
				a, ok := the(s.Res[0])
				if !ok {
					continue
				}
				w := g.fresh(g.f.Types[s.Res[0]])
				a.Args = []V{w}
				s.Res = []V{w}
				r.Stmts = append(r.Stmts[:i+1], append([]Stmt{a}, r.Stmts[i+1:]...)...)
				i++
			}
		}
	})
}

// leftPair is a value of U held exactly on a target whose U is a pair: one
// the selection sent from its two digits to ℤ (ADR 0151), whose bound the
// rung hides from the decision taken again. Only its loops and remainders are
// ascribed: a program's `Nat`, unbounded, was ℤ's before, and ascribed its
// loops' variables renamed lost three loops' termination on popcount and
// render (pairsel-2026-11-21).
func (g *bigs) leftPair(v V) bool {
	return g.tg.Word.Pair && v >= 0 && int(v) < len(g.fs) && g.fs[v].v.within(ep{}, u64Max)
}

// PlantRungFactsForgotten places no ascription after the rung: a digit sum over
// a value of U held exactly on JavaScript is then refused, its accumulator
// unbounded (TestAPairLeavesForTheIntegers).
var PlantRungFactsForgotten bool

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

	// D_A (exact.go): an operation the machine's readings do not hold is held
	// exactly, demanded like `decimal`'s argument
	for v := range f.exactDemand {
		g.dem[v] = true
	}
	for i, p := range f.Params {
		if g.bigType(f.Types[p]) || (sig != nil && i < len(sig.Params) && g.bigType(sig.Params[i].Type)) {
			g.b[p] = true
		}
		// AN ENTRY POINT'S PARAMETER IS WHAT THE HOST PASSES (integers-boundary
		// §2, §4): outside the signed word, on a target whose unsigned word is
		// no host type but a pair of its digits (JavaScript's), the host hands
		// over its own integer of any size, a BigInt. `wide(n ∈ 0..2^63-1)`
		// was refused there for want of this case.
		if g.tg.Word.Pair && !PlantBoundedAboveTheWordIsTheWord && g.tg.ValueType(f.Types[p]) == core.U64Type {
			g.b[p] = true
		}
	}
	consts := map[V]*big.Int{}
	constB := map[V]*Stmt{} // the closed constant terms the rule below holds exactly
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			g.root[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				def[v] = s
			}
			switch {
			case s.Op == OConst && s.Lit != nil && s.Lit.Kind == core.KInt && len(s.Res) == 1:
				consts[s.Res[0]] = big.NewInt(s.Lit.Int)
				// A CONSTANT OUTSIDE S IS SUPPLY (integers.md §1a, ADR 0156): a
				// point is held as ρ of itself, exactly where S does not hold it.
				// Only a word narrower than int64 meets one (JavaScript's ±(2⁵³−1));
				// left a word there, 2⁶² reached `console.log` and `String` as a
				// Number and printed 4611686018427388000.
				if !g.fits(s.Res[0]) && f.exactDemand[s.Res[0]] && !PlantConstantInTheWord {
					g.b[s.Res[0]] = true
				}
			case s.Op.IsArith() && !s.Op.IsBit() && s.Name == "" && len(s.Res) == 1:
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
							constB[s.Res[0]] = s
						}
					}
				}
			case s.Op.IsBit() && s.Name == "" && len(s.Res) == 1:
				g.bitops = append(g.bitops, s)
			case s.Op.IsCmp() && s.Name == "" && len(s.Args) == 2:
				g.cmps = append(g.cmps, s)
			case s.Op == OCall:
				p := g.tg.Prims[s.Name]
				for j, a := range s.Args {
					if j < len(p.Args) && p.Kind != "stmt" && g.bigType(p.Args[j]) {
						g.dem[a] = true
					}
				}
				// RENDER IS ℤ's, render : ℤ → Σ*, so its argument is demanded on
				// every target, whatever a target declares its parameter to be
				// (ADR 0099): in B iff its fact leaves S. Only the widening of a
				// word value is the target's question, and on a target with no
				// bignum the word renders itself (below, ADR 0095).
				// keyed by root, so an argument whose own fact keeps it in S,
				// a π under its arm's guard, is no demand: a dead arm's
				// `decimal(n)` would take an unsigned hash with it
				// On a target with no bignum a value in U is rendered by the
				// word (windows' print_int, its field), so it is no demand
				// either: an unsigned hash in print_int's dead arm above the
				// words would need U₆₄ ↪ ℤ, not built
				if s.Name == "decimal" && len(s.Args) == 1 && !g.fits(s.Args[0]) &&
					!(!g.tg.HasBig() && (g.tg.Word.Unsigned || g.tg.Word.Pair) &&
						g.fs[s.Args[0]].v.within(ep{}, u64Max) && !PlantDecimalDemandsU) {
					g.dem[s.Args[0]] = true
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
		// A RESULT DECLARED `Int` IS ℤ (ADR 0147 item 3): its yields are demanded
		// as `decimal`'s argument is, in B exactly where a fact leaves S, so a
		// yield proven only in the unsigned word is the host's integer of any
		// size and not a word the claim never named (the book's finding 27:
		// `medium(n ∈ 0..2^31) (Int) { n * n }` was refused on JavaScript, its
		// product a pair of digits at a declared `Int`)
		if big || resultIsZ(sig, j) && !PlantResultIntIsTheWord {
			for _, args := range ownExits(f.Body, TYield) {
				if j < len(args) {
					if !big {
						// A YIELD ITS OWN FACT KEEPS IN S is a word, whatever
						// its root's: demand is keyed by root, and a π-narrowed
						// loop variable (binary search's m, under its guard)
						// would take the loop's widened join with it
						if g.fits(args[j]) {
							continue
						}
						if !g.dem[args[j]] {
							g.resDem = append(g.resDem, args[j])
						}
					}
					g.dem[args[j]] = true
				}
			}
		}
	}
	// A CONSTANT JOINED WITH A RESIDUE IS THE RESIDUE'S LITERAL: a join one of
	// whose inputs is a residue mod 2⁶⁴, `u64-of` or a fused operation (ADR
	// 0082), holds the unsigned word, and a closed constant in U entering it
	// (FNV-1a's offset basis, the loop's first value) is that word's literal,
	// not an exact value that would make the join exact and refuse the residue
	// (windows32's fnv64, printed: `decimal` made the rung run)
	if !PlantDecimalDemandsU {
		var unmark func(v V)
		unmark = func(v V) {
			st, ok := constB[v]
			if !ok {
				return
			}
			delete(g.b, v)
			delete(constB, v)
			for _, a := range st.Args {
				unmark(a)
			}
		}
		for _, j := range g.joins {
			residue := false
			for _, u := range j.in {
				if d := def[u]; d != nil && (d.Op == OCall && d.Name == "u64-of" || d.Op.IsArith() && !langOp(d)) {
					residue = true
				}
			}
			if !residue {
				continue
			}
			for _, u := range j.in {
				if c, ok := consts[u]; ok && c.Sign() >= 0 && c.BitLen() <= 64 {
					unmark(u)
				}
			}
		}
	}
	// A RESIDUE'S SPINE IS A LITERAL, NOT AN EXACT VALUE: Fuse writes `t mod 2ⁿ`
	// of a closed term as its residue, and one past int64 as hi·10¹⁵ + lo, the
	// core's spelling of a literal (ir/fuse.go), whose `+` is the residue's, a
	// target's own operator (its source the `mod`). The product hi·10¹⁵ leaves
	// S and is a closed constant term, so the rule above took it as exact, and
	// the target's `+` then refused it, wherever a print made the rung run
	// (book chapter 7's hash64, `decimal` of a 64-bit FNV-1a): ρ of a literal in
	// U is the unsigned word's, which the words pass chooses
	if !PlantResidueSpineExact {
		for _, s := range g.ariths {
			if langOp(s) {
				continue
			}
			for _, a := range s.Args {
				if c, ok := consts[a]; ok && c.Sign() >= 0 && c.BitLen() <= 64 {
					delete(g.b, a)
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

// onlyResultDemands reports every demand to be one a result declared `Int`
// made (collect's resDem), and none an operation's or a host's.
func (g *bigs) onlyResultDemands() bool {
	mine := map[V]bool{}
	for _, v := range g.resDem {
		mine[g.r(v)] = true
	}
	for v := range g.dem {
		if !mine[v] {
			return false
		}
	}
	return true
}

// OwnExits is ownExits, for the printers' plan (ir/plan).
func OwnExits(r *Region, t Term) [][]V { return ownExits(r, t) }

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
	for _, s := range g.bitops {
		promotable[g.r(s.Res[0])] = true
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
		// THE BIT OPERATIONS (ADR 0082): exact when an operand is, and held
		// exactly unless the fact of the result lies in S, where r brings it
		// back (rewrite's bit). A shift's count is a word, never demanded.
		for _, s := range g.bitops {
			v := s.Res[0]
			for _, a := range bitOperands(s) {
				if g.isB(a) && !g.fits(v) {
					setB(v)
				}
			}
			if demanded(v) {
				setB(v)
			}
			if g.isB(v) {
				for _, a := range bitOperands(s) {
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
			// A JOIN IS EXACT BY DEMAND only where some input can be: a
			// language operation, another join, or a value already exact. One
			// of host results alone (JavaScript's `js.+` on `Any`, sieve-js's
			// count) holds what the host computed, in its word, and taking it
			// exact refused the host's own operator
			if demanded(j.v) && (g.anyExactable(j.in, promotable) || PlantJoinOfHostsExact) {
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

// anyExactable reports some input that can be held exactly: one already in B,
// a language operation's result, or a join one of whose inputs is, the least
// fixed point over the joins.
func (g *bigs) anyExactable(in []V, promotable map[V]bool) bool {
	if g.exact == nil {
		g.exact = map[V]bool{}
		for _, s := range g.ariths {
			if langOp(s) {
				g.exact[g.r(s.Res[0])] = true
			}
		}
		for _, s := range g.bitops {
			g.exact[g.r(s.Res[0])] = true
		}
		for grew := true; grew; {
			grew = false
			for _, j := range g.joins {
				if g.exact[g.r(j.v)] {
					continue
				}
				for _, u := range j.in {
					if g.exact[g.r(u)] || g.isB(u) {
						g.exact[g.r(j.v)], grew = true, true
						break
					}
				}
			}
		}
	}
	for _, u := range in {
		if g.isB(u) || g.exact[g.r(u)] {
			return true
		}
	}
	return false
}

// PlantJoinOfHostsExact takes a demanded join of host results exact, as
// before: examples/native/sieve-js is then refused on JavaScript
// (TestAJoinOfHostResultsIsNotExact).
var PlantJoinOfHostsExact bool

// bitExact reports a bit operation computed exactly: its result or an
// operand that may be is held so.
func (g *bigs) bitExact(s *Stmt) bool {
	if g.isB(s.Res[0]) {
		return true
	}
	for _, a := range bitOperands(s) {
		if g.isB(a) {
			return true
		}
	}
	return false
}

// bit is a bit operation on the rung (ADR 0082 item 1: the host operator
// where the host computes the function natively, here its bignum's): the
// operands widened, a shift's count a word, and a result outside B, whose
// fact lies in S, brought back by r, the identity on S.
func (g *bigs) bit(s *Stmt) {
	v := s.Res[0]
	ops := bitOperands(s)
	args := make([]V, 0, len(s.Args))
	for _, a := range ops {
		args = append(args, g.widen(a, "an operand of "+s.Op.String()))
	}
	for _, a := range s.Args[len(ops):] {
		args = append(args, g.word(a, "the count of "+s.Op.String()))
	}
	g.ops++
	op := Stmt{Op: OCall, Name: bigBit[s.Op], Args: args, Src: s.Src}
	if !g.isB(v) {
		t := g.fresh(core.BigType)
		g.fs[t] = g.fs[v]
		op.Res = []V{t}
		g.stmts = append(g.stmts, op, Stmt{Op: OCall, Name: "big-word", Args: []V{t}, Res: []V{v}})
		return
	}
	g.f.Types[v] = core.BigType
	// only the left shift and not leave a bounded set's bit length (ADR
	// 0029): the others' results are no longer than their operands
	if g.fit == "" || g.proven(v) || (s.Op != OShl && s.Op != ONot) {
		op.Res = []V{v}
		g.stmts = append(g.stmts, op)
		return
	}
	t := g.fresh(core.BigType)
	op.Res = []V{t}
	k := g.fresh("int")
	g.stmts = append(g.stmts, op,
		Stmt{Op: OConst, Lit: core.Int(int64(g.bits)), Res: []V{k}},
		Stmt{Op: OCall, Name: g.fit, Args: []V{t, k}, Res: []V{v}})
}

// THE ONE SET IS CHECKED WHERE IT CLAIMS (ADR 0152, amending ADR 0029). A
// declared set above the word is the program's one set, by sign and bit
// length, and it was checked at every operation of the rung; under ADR 0150
// an intermediate no set claims is ℤ's, and checked so it stopped the program
// with its value proven (`x * 3` under a declared S₆₄ on JavaScript). So the
// fit is placed where a value is claimed, and where nothing but the declared
// set bounds one:
//
//  1. at a claim, a value ascribed a declared set above the word (`the`, which
//     a definition's result keeps through inlining, core/reduce.go's
//     ascribeResult): fitClaims;
//  2. at an operation whose result the decision does not bound, its fact not
//     finite, the declared set the only consent to its room: proven is false.
//
// An operation whose result has a finite fact is not fitted: the fact is a
// theorem, it is ℤ's where no set claims it, and where one does the claim's
// fit checks it. The width stays the program's one set (g.bits): ADR 0029's
// set, its places changed.
func (g *bigs) proven(v V) bool {
	if PlantFitEveryOp || int(v) >= len(g.fs) {
		return false
	}
	x := g.fs[v].v
	return !x.bot && x.finite()
}

// fitClaims turns each claim above the word on a value held exactly into its
// fit, before the ascriptions are erased: the(x) with x on the rung, its set
// past the word, becomes g.fit(x, bits).
func (g *bigs) fitClaims() {
	if g.fit == "" || PlantNoClaimFit {
		return
	}
	g.f.Walk(func(r *Region) {
		var out []Stmt
		for _, s := range r.Stmts {
			if s.Op != OThe || len(s.Args) != 1 || len(s.Res) != 1 || g.f.Types[s.Args[0]] != core.BigType {
				out = append(out, s)
				continue
			}
			lo, hi, ok := core.IntRangeBig(s.Type)
			if !ok || lo.Cmp(big.NewInt(g.tg.Word.Lo)) >= 0 && hi.Cmp(big.NewInt(g.tg.Word.Hi)) <= 0 {
				out = append(out, s)
				continue
			}
			k := g.fresh("int")
			g.f.Types[s.Res[0]] = core.BigType
			out = append(out, Stmt{Op: OConst, Lit: core.Int(int64(g.bits)), Res: []V{k}},
				Stmt{Op: OCall, Name: g.fit, Args: []V{s.Args[0], k}, Res: s.Res, Src: s.Src})
		}
		r.Stmts = out
	})
}

// PlantFitEveryOp fits every operation of the rung again, as before ADR 0152:
// `x * 3 / 4` under a declared S₆₄ then stops on JavaScript
// (TestTheOneSetIsCheckedWhereItClaims).
var PlantFitEveryOp bool

// PlantNoClaimFit places no fit at a claim: big-sign's yield past its declared
// −2^200..2^200 then answers where it must stop.
var PlantNoClaimFit bool

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
	// a value whose type is already the host's big integer (a host result
	// declared `big`) needs no inclusion
	if v < 0 || g.isB(v) || !g.integer(v) || g.bigType(g.f.Types[v]) {
		return v
	}
	if !g.fits(v) && g.err == nil && !PlantConstantNotSpined {
		// A CONSTANT PAST THE WORD is a value, not an operation: c, exact in
		// every execution (its fact the point [c, c]), is Horner's form over
		// 10¹⁵ held exactly, (hi·10¹⁵ + lo) with each digit in the word, the
		// spelling the reader gives a literal past int64 (core/read.go,
		// bigLiteral). JavaScript's word is ±(2⁵³ − 1), so S₆₄'s own ends,
		// claimed `∈ Int`, reach here
		if c := g.fs[v].v; c.finite() && c.lo == c.hi {
			return g.spine(c.lo.big())
		}
	}
	// A VALUE OF U ON A TARGET WHOSE U IS A PAIR (ADR 0151) is widened by the
	// same name, which the pairs pass, running after, reads on the value's
	// digits as ι : U ↪ ℤ, d(h, l) = h·B + l (`pair-bigint`); a closed constant
	// is not one, a value its spine spells above
	if c := g.fs[v].v; !g.fits(v) && g.tg.Word.Pair && c.within(ep{}, u64Max) && !(c.finite() && c.lo == c.hi) &&
		!PlantPairsKeepAll {
		out := g.fresh(core.BigType)
		g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big-of", Args: []V{v}, Res: []V{out}})
		return out
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

// word is the obligation at a position declared in the word, answering the
// value the position receives. A value of B reaches it only where its fact is
// inside the signed word S (ADR 0079 item 4): there it is r(v), the retraction
// ℤ ⇀ S the target realizes as `big-word`, the identity on S, so nothing is
// lost. A value whose fact may leave S is refused: a range above the word is a
// widening, not a refinement (unbounded-rung.md §3), and no narrowing may lose
// an integer. Before the fact was read, `square(n) / 2^40`, proven in
// [0, 2^40], was refused (book finding 28).
func (g *bigs) word(v V, where string) V {
	if v < 0 || !g.isB(v) || g.err != nil {
		return v
	}
	if g.fits(v) && !PlantNarrowRefused {
		if _, ok := g.tg.Prims["big-word"]; ok || !g.tg.HasBig() {
			out := g.fresh("int")
			g.fs[out] = fact{v: g.fs[v].v} // r is the identity on S
			g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big-word", Args: []V{v}, Res: []V{out}})
			return out
		}
	}
	g.refuseWord(v, where)
	return v
}

// PlantNarrowRefused refuses every value of B at a word position, its fact
// unread, as before (TestAValueProvenInTheWordComesBackIntoIt).
var PlantNarrowRefused bool

// refuseWord is word's refusal: a value of B at a word position, its fact
// not inside the word.
func (g *bigs) refuseWord(v V, where string) {
	if v < 0 || !g.isB(v) || g.err != nil {
		return
	}
	g.err = fmt.Errorf("%s is held in arbitrary precision, which is WIDER than the word of target %s, %s, "+
		"so on this target it is not an `int` — it is a rung above the host's word (ADR 0026, "+
		"docs/unbounded-rung.md).\n"+
		"  A range above the word is a WIDENING, not a refinement: a value that may leave the machine "+
		"word cannot silently be used where an `int` is required, and this refusal is where that is "+
		"said. Widen the destination, or take the value to a string with `decimal`.",
		where, g.tg.Name, g.tg.Word)
}

// resultUnclaimed reports the j-th result declaring no set: no signature, or
// one whose result is unstated.
func resultUnclaimed(sig *core.Sig, j int) bool {
	if sig == nil {
		return true
	}
	if len(sig.Results) > 0 {
		return j < len(sig.Results) && (sig.Results[j] == "" || sig.Results[j] == "any")
	}
	return j == 0 && (sig.Result == "" || sig.Result == "any")
}

// bigBit is each of ADR 0082's operations on the rung: ℤ's 2-adic and, or,
// xor and not, the shifts a·2ᵏ and ⌊a/2ᵏ⌋ with k a word, and the Euclidean
// remainder.
var bigBit = map[Op]string{OAnd: "big&", OOr: "big|", OXor: "big⊕", ONot: "big~", OShl: "big<<", OShr: "big>>", OMod: "big-mod"}

// bitOperands is a bit operation's operands that may be held exactly: a
// shift's count is a word.
func bitOperands(s *Stmt) []V {
	if (s.Op == OShl || s.Op == OShr) && len(s.Args) == 2 {
		return s.Args[:1]
	}
	return s.Args
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
		// A RESULT THAT CLAIMS NOTHING (ADR 0062 item 8) is held as its yields
		// are, their join in S ⊑ big (ADR 0079 item 2): exactly when a yield is
		// in B, which a claim above the word upstream consented to. Until then
		// it was a position in the word, and a definition whose result was never
		// claimed was refused for a representation nobody had asked for.
		// A RESULT DECLARED IN U ON A TARGET WHOSE U IS A PAIR is the host's
		// BigInt whatever holds it inside (`pair-bigint`, ADR 0083): a yield
		// held exactly leaves as itself (ADR 0151)
		inU := g.tg.Word.Pair && g.tg.ValueType(f.Results[j]) == core.U64Type && !PlantPairSinks
		if !big && (resultUnclaimed(sig, j) || resultIsZ(sig, j) || inU) {
			for _, args := range ownExits(f.Body, TYield) {
				if j < len(args) && g.isB(args[j]) {
					big = true
				}
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
				args[j] = g.word(a, what)
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
	case s.Op == OConst && s.Lit != nil && s.Lit.Kind == core.KInt && len(s.Res) == 1 && g.isB(s.Res[0]) && !g.fits(s.Res[0]):
		// the constant held exactly is its spine, defining its own value
		v := s.Res[0]
		sp := g.spine(big.NewInt(s.Lit.Int))
		last := &g.stmts[len(g.stmts)-1]
		if len(last.Res) == 1 && last.Res[0] == sp {
			last.Res = []V{v}
			g.f.Types[v] = core.BigType
			g.fs[v] = g.fs[sp]
			return
		}
		g.err = fmt.Errorf("a constant held exactly, %d, has no spine defining it (ir/big.go)", s.Lit.Int)
	case s.Op.IsBit() && s.Name == "" && len(s.Res) == 1 && g.bitExact(s):
		g.bit(s)
	case s.Op.IsBit():
		for k, a := range s.Args {
			s.Args[k] = g.word(a, "an operand of "+s.Op.String())
		}
		for _, v := range s.Res {
			if g.isB(v) {
				g.refuseWord(v, "the result of "+s.Op.String())
			}
		}
		g.stmts = append(g.stmts, *s)
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
			if g.fit == "" || g.proven(v) {
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
			for k, a := range s.Args {
				s.Args[k] = g.word(a, "an operand of a target's own operator")
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
			s.Args[0] = g.word(s.Args[0], "an operand of "+s.Op.String())
			s.Args[1] = g.word(s.Args[1], "an operand of "+s.Op.String())
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
			// A STATEMENT'S TEMPLATE READS ITS ARGUMENTS as an expression's
			// does, so its word parameter demands the word too: skipped, a big
			// value reached Go's `fmt.Println(${n})` and was refused by W5 in
			// the verifier's words (book finding 3)
			if j >= len(p.Args) || (p.Kind == "stmt" && PlantStmtArgsUnasked) {
				continue
			}
			// RENDERING IS ℤ's AT EVERY SORT (ADR 0095): on a target with no
			// bignum, `decimal` is realized per representation, the limb
			// library's render on a value held exactly (LowerLimbs) and the
			// word's digit loop elsewhere (its stated term), so its argument
			// keeps the representation its facts gave it.
			if s.Name == "decimal" && !g.tg.HasBig() {
				continue
			}
			// AND ON A HOST WITH ONE, a value its facts keep in the word is
			// rendered by the host's own integer formatting, `decimal-word`
			// (integers-boundary §3): no widening paid to print a word
			if s.Name == "decimal" && !g.isB(a) && g.integer(a) && g.tg.ValueType(g.f.Types[a]) == "int" && !PlantDecimalAlwaysBig {
				if n, ok := decimalWord(g.tg); ok {
					s.Name = n
					s.Args[j] = g.word(a, "the argument of decimal")
					continue
				}
			}
			what := fmt.Sprintf("argument %d of %s", j+1, core.ShowName(s.Name))
			switch ty := p.Args[j]; {
			// A VALUE OF U AT A HOST'S PARAMETER OF ANY TYPE, on a target whose U
			// is a pair, is its decimal string, `pair-box` on the digits (ADR
			// 0083): held exactly it is the same string, `big-box`, since a
			// representation changes no value (ADR 0083's L1; ADR 0151)
			case g.isB(a) && g.tg.Word.Pair && !PlantPairSinks && g.fs[a].v.within(ep{}, u64Max) &&
				(ty == "any" || ty == "Any" || p.Kind == "stmt" && ty == ""):
				box := g.fresh(g.tg.Prims["big-box"].Result)
				g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big-box", Args: []V{a}, Res: []V{box}})
				s.Args[j] = box
			case g.bigType(ty):
				s.Args[j] = g.widen(a, what)
			case ty == "int" || g.tg.ValueType(ty) == "int" || g.tg.ValueType(ty) == core.U64Type:
				s.Args[j] = g.word(a, what)
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
					s.Args[j] = g.word(a, "a loop's initial value")
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
		for k, a := range s.Args {
			if g.integer(a) {
				s.Args[k] = g.word(a, "an operand of "+s.Op.String())
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
			for j, a := range r.Stmts[i].Store {
				r.Stmts[i].Store[j] = m(a)
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

// PlantStmtArgsUnasked skips a statement primitive's arguments, as before: a
// big value then reaches a word parameter, refused only by W5
// (TestABigValueAtAWordParameterIsRefusedInTheProgramsTerms).
var PlantStmtArgsUnasked bool

// PlantBoundedAboveTheWordIsTheWord holds an entry point's parameter in the
// unsigned word's pair on JavaScript, as before: `wide(n ∈ 0..2^63-1)` is then
// refused there again (TestIntIsTheIntegersAtTheBoundary).
var PlantBoundedAboveTheWordIsTheWord bool

// resultIsZ reports the j-th result declared `Int`, ℤ: no bound, so held as
// its yields are, in the word where they are (integers-boundary §2), as an
// unclaimed result is.
func resultIsZ(sig *core.Sig, j int) bool {
	if sig == nil || emit.PlantEntryIntIsTheWord {
		return false
	}
	if len(sig.Results) > 0 {
		return j < len(sig.Results) && sig.Results[j] == "int"
	}
	return j == 0 && sig.Result == "int"
}

// PlantDecimalDemandsU demands a value in U at windows' `decimal`, and takes a
// closed constant in U joined with a residue as exact, as before: differential
// case fnv64's printed hash is then refused at its loop variable
// (TestPrintIntIsTotalOnTheIntegers).
var PlantDecimalDemandsU bool

// PlantResidueSpineExact takes a residue's spine hi·10¹⁵ as an exact value, as
// before: chapter 7's hash64 is then refused at the target's `+`
// (TestAResiduesSpineIsALiteral).
var PlantResidueSpineExact bool

// PlantResultIntIsTheWord leaves a result declared `Int` undemanded, as before
// ADR 0147: finding 27's product, in the unsigned word, is then refused at its
// declared `Int` on JavaScript (TestIntIsTheIntegersAtTheBoundary).
var PlantResultIntIsTheWord bool

// PlantDecimalAlwaysBig renders every value through the host's big integer, a
// word widened first, as before integers-boundary: correct, and a widening per
// print (TestIntIsTheIntegersAtTheBoundary's emitted text).
var PlantDecimalAlwaysBig bool

// decimalWord is the target's rendering of a word, `decimal-word` under its
// host module's name (go.decimal-word), found by its spelling as the language's
// names are on a target that states no realization.
func decimalWord(tg *emit.Target) (string, bool) {
	if _, ok := tg.Prims["decimal-word"]; ok {
		return "decimal-word", true
	}
	for n := range tg.Prims {
		if strings.HasSuffix(n, ".decimal-word") {
			return n, true
		}
	}
	return "", false
}

// spine is the constant c held exactly, Horner's form over 10¹⁵: c's digits
// d_k … d_0 in that base (truncated, so each has c's sign), each a word
// constant widened by big-of, folded as ((d_k·B + d_{k−1})·B + …) + d_0.
func (g *bigs) spine(c *big.Int) V {
	base := big.NewInt(1000000000000000)
	var digits []int64
	for v := new(big.Int).Set(c); v.Sign() != 0; {
		q, r := new(big.Int).QuoRem(v, base, new(big.Int))
		digits = append(digits, r.Int64())
		v = q
	}
	if len(digits) == 0 {
		digits = []int64{0}
	}
	konst := func(k int64) V {
		w := g.fresh(fmt.Sprintf("int %d %d", k, k))
		g.fs[w] = fact{v: exactIV(k)}
		g.stmts = append(g.stmts, Stmt{Op: OConst, Lit: core.Int(k), Res: []V{w}})
		out := g.fresh(core.BigType)
		g.fs[out] = fact{v: exactIV(k)}
		g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big-of", Args: []V{w}, Res: []V{out}})
		return out
	}
	acc := konst(digits[len(digits)-1])
	for i := len(digits) - 2; i >= 0; i-- {
		b := konst(base.Int64())
		p := g.fresh(core.BigType)
		g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big*", Args: []V{acc, b}, Res: []V{p}})
		d := konst(digits[i])
		s := g.fresh(core.BigType)
		g.stmts = append(g.stmts, Stmt{Op: OCall, Name: "big+", Args: []V{p, d}, Res: []V{s}})
		acc = s
	}
	if e, ok := fromBig(c); ok {
		g.fs[acc] = fact{v: rangeE(e, e)}
	}
	return acc
}

// PlantConstantInTheWord leaves a constant outside S a word, as before ADR
// 0156: on JavaScript `console.log(4611686018427387904)` prints
// 4611686018427388000 (TestAConstantOutsideTheWordIsHeldExactly).
var PlantConstantInTheWord bool

// PlantConstantNotSpined refuses a constant past the word where it is widened,
// as before: S₆₄'s ends claimed `∈ Int` are then refused on JavaScript
// (TestAConstantPastTheWordIsHeldExactly).
var PlantConstantNotSpined bool
