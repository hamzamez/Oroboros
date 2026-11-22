package ir

import (
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// EVERY INTEGER IS THE INTEGER IT IS (sets-are-claims-research §3, the design
// A; hamza 2026-11-21: "A an intermediate outside E yes", "report is enough").
//
// ρ(v) is the cheapest representation whose domain holds v's proven fact, over
// the chain each target provides: the signed word S, the unsigned word U, the
// double word E (ADR 0149), and ℤ, the rung. A declared set is a claim and
// chooses nothing, so an operation is never refused for its size: what lies
// in none of S, U and E is held exactly, and reported. The demand is
//
//	D_A = { v | v a language operation's result, fact(v) ⊄ S, fact(v) ⊄ U,
//	        v ∉ E's selection }
//
// E's selection being the double word's greatest fixed point (SelectDouble),
// which takes only an intermediate read back into a word: a·b in E whose one
// use is a·b·c past E is not E's, and the rung's own closure takes it with
// its use. The rung then holds the least set of values meeting D_A and every
// other demand (big.go's solve). A refusal is left only where a value must
// enter a bounded set, a sink: a table's index or element, a host's word, an
// entry point's word, a claim; the rung refuses there, as it did.
//
// D_A is empty for every program whose operations the decision proves in a
// word or E, so no program accepted before changes; it is non-empty exactly
// where the decision refused for size.

// WordsOnly is `-words`: no operation is held in ℤ by its size, and one the
// machine's words and E do not hold is refused, as before A. An option for a
// program that must allocate nothing.
var WordsOnly bool

// PlantTaintIgnored demands a value in ℤ though it flows into a sink: under
// -checked a product stored into a table is then refused at the store, not
// trapped where it leaves the word (TestAnIntermediatePastEIsTheIntegersOwn).
var PlantTaintIgnored bool

// PlantTaintThroughRem taints a remainder's dividend as well as its divisor:
// a·b·c % 7 returned as a word is then refused, though the rung computes it
// (TestAnIntermediatePastEIsTheIntegersOwn).
var PlantTaintThroughRem bool

// PlantKeptCallSink reads a kept definition's parameter as a sink, as before
// finding 112's fix: windows' `grow` under -checked keeps p a word and stops
// (TestAKeptParameterIsNoSink).
var PlantKeptCallSink bool

// PlantPairSinks keeps a host's any and a result declared in U sinks on a
// target whose U is a pair: a division of a value of U printed is then refused
// again (TestAPairLeavesForTheIntegers).
var PlantPairSinks bool

// PlantExactDemandOff leaves D_A empty: a product of three words past E is
// then refused again (TestAnIntermediatePastEIsTheIntegersOwn).
var PlantExactDemandOff bool

// exactDemand is D_A of the decided f and its report: the operations past
// every word, and those of U the pair does not compute, each counted with the
// first of them as the program wrote them.
//
// ON A TARGET WHOSE U IS A PAIR (JavaScript's, ADR 0151), U holds a value only
// where the pair covers its uses: D_A takes PairsLeaving too, a value of U the
// pair cannot compute with, and the rung's closure the rest.
func exactDemand(tg *emit.Target, f *Func, sig *core.Sig) (map[V]bool, exactReport) {
	var rep exactReport
	if WordsOnly || PlantExactDemandOff || !tg.HoldsZ() {
		return nil, rep
	}
	fs := f.facts
	if len(fs) != f.NV() {
		return nil, rep
	}
	sLo, sHi := ei(tg.Word.Lo), ei(tg.Word.Hi)
	leaving := PairsLeaving(tg, f, sig)
	inU := func(v V, x iv) bool {
		return (tg.Word.Unsigned || tg.Word.Pair && !leaving[v]) && x.within(ei(0), u64Max)
	}
	var e, taint map[V]bool
	out := map[V]bool{}
	inOrder(f, func(s *Stmt) {
		if len(s.Res) != 1 || s.Name != "" || s.Mode == MWrap || s.Mode == MWrapPair || !langOp(s) {
			return
		}
		if !(s.Op.IsArith() || s.Op.IsBit()) {
			return
		}
		v := s.Res[0]
		x := fs[v].v
		if x.bot || x.within(sLo, sHi) || inU(v, x) {
			return
		}
		ofU := leaving[v] && x.within(ei(0), u64Max)
		if e == nil && !ofU { // E's selection, on a clone: nothing marked
			c := f.Clone()
			c.facts = f.facts
			SelectDouble(tg, c)
			e = Doubles(c)
		}
		if !ofU && e[v] {
			return
		}
		if taint == nil {
			taint = tainted(tg, f, sig)
		}
		if taint[v] && !PlantTaintIgnored {
			return // it would carry ℤ into a sink: a word, refused or trapped
		}
		out[v] = true
		// EACH OPERATION BY ITS SOURCE, as J writes it: the application it lowers,
		// or, derived by a rewrite (Of), its own kind in the application it was
		// derived from, so two from one rounding are told apart
		short := func(t string) string {
			if len(t) > 48 {
				return t[:45] + "..."
			}
			return t
		}
		src := s.Op.String()
		switch {
		case s.Src != nil:
			src = short(core.ShowTerm(s.Src))
		case s.Of != nil:
			src = s.Op.String() + " in " + short(core.ShowTerm(s.Of))
		}
		if ofU {
			rep.pair.add(src)
		} else {
			rep.past.add(src)
		}
	})
	// THE PAIRS' JOINS: a loop's variable of U the pair cannot divide is
	// demanded as a value, not counted as an operation
	for v := range leaving {
		if out[v] || fs[v].v.bot || fs[v].v.within(sLo, sHi) {
			continue
		}
		if taint == nil {
			taint = tainted(tg, f, sig)
		}
		if !taint[v] || PlantTaintIgnored {
			out[v] = true
		}
	}
	if len(out) == 0 {
		return nil, rep
	}
	return out, rep
}

// exactReport is D_A's operations: past every word and the double word, and
// of U where the pair does not compute them.
type exactReport struct{ past, pair opList }

// opList is the operations held exactly, each as written: the whole list, so
// the report is D_A itself and not a sample of it (hamza, "report is enough").
type opList struct {
	n    int
	srcs []string
}

func (l *opList) add(src string) {
	l.n++
	if !PlantReportFirstFour || len(l.srcs) < 4 {
		l.srcs = append(l.srcs, src)
	}
}

// PlantReportFirstFour keeps the first four, as before: a fifth operation is
// then missing from the report (TestTheReportNamesEveryOperation).
var PlantReportFirstFour bool

func (l opList) String() string {
	more := ""
	if l.n > len(l.srcs) {
		more = "; …"
	}
	return strings.Join(l.srcs, "; ") + more
}

// exactNotes is the report of D_A: how many operations the machine's readings
// do not hold, computed as the integers they are, and each of them; on a
// target whose U is a pair, those of U it holds exactly for want of the pair's
// operation, a separate line, since they are not past the words.
func exactNotes(name string, r exactReport) []string {
	var out []string
	if r.past.n > 0 {
		out = append(out, fmt.Sprintf("%s: %d operation(s) past every word and the double word, held exactly: %s",
			name, r.past.n, r.past))
	}
	if r.pair.n > 0 {
		out = append(out, fmt.Sprintf("%s: %d operation(s) of the unsigned word that its pair does not compute, "+
			"held exactly: %s", name, r.pair.n, r.pair))
	}
	return out
}

// tainted is the set of values that must stay words because holding them
// exactly would carry ℤ into a sink (sets-are-claims-research §3): a value is
// tainted when a sink uses it, and when it is an operand of an operation, or
// an input of a join, whose result is tainted, since the rung's closure makes
// an operation over an exact operand exact. The least fixed point. A tainted
// value past the words is refused for its size, or trapped under -checked,
// where the operation leaves the word, as before A.
//
// A use takes ℤ when it is the language's arithmetic, bit operation or
// comparison; a host's parameter declared above the word, or `decimal`; a join
// (an `if`'s result, a loop's variable or result), its own uses followed; or a
// result declared ℤ. Every other use is a sink.
func tainted(tg *emit.Target, f *Func, sig *core.Sig) map[V]bool {
	fs := f.facts
	bigType := func(ty string) bool { return ty == core.BigType || tg.ValueType(ty) == core.BigType }
	// ON A TARGET WHOSE U IS A PAIR, ℤ leaves for two positions as U does (ADR
	// 0151): a host's parameter of any type, where a value of U is its decimal
	// string in either representation (`pair-box`, `big-box`), and a result
	// declared in U, which is the host's BigInt in either (`pair-bigint`)
	inU := func(v V) bool { return tg.Word.Pair && !PlantPairSinks && fs[v].v.within(ei(0), u64Max) }
	anyArg := func(p emit.Prim, j int) bool {
		return j < len(p.Args) && (p.Args[j] == "any" || p.Args[j] == "Any" || p.Kind == "stmt" && p.Args[j] == "")
	}
	// recv is each join's receiver for an input: the value it flows into
	recv := map[V][]V{}
	join := func(in []V, out V) {
		for _, a := range in {
			recv[a] = append(recv[a], out)
		}
	}
	var operands [][2][]V // an operation's operands and its one result
	sink := map[V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch {
			case len(s.Res) == 1 && s.Name == "" && (s.Op.IsArith() || s.Op.IsBit()):
				// THE RUNG'S CLOSURE, as solve has it: a remainder is exact only
				// where its divisor is (an exact dividend's remainder by a word is
				// a word, big%-small), and a bit operation whose result lies in S
				// is read back into the word; every other operation is exact where
				// an operand is
				args := s.Args
				switch {
				case s.Op == ORem && len(s.Args) == 2 && !PlantTaintThroughRem:
					args = s.Args[1:]
				case s.Op.IsBit() && fs[s.Res[0]].v.within(ei(tg.Word.Lo), ei(tg.Word.Hi)):
					args = nil
				}
				operands = append(operands, [2][]V{args, {s.Res[0]}})
			case s.Op.IsCmp() && s.Name == "":
				// a comparison of exact values is exact's; its result a boolean
			case s.Op == OCall && tg.Prims[s.Name].Program && !strings.HasPrefix(s.Name, natPrefix) && !PlantKeptCallSink:
				// A KEPT DEFINITION'S PARAMETER IS NO SINK (finding 112, ADR
				// 0102): keeping is a factoring, which never changes an answer, and
				// its parameter's word is the factoring's (a kept parameter above the
				// word is not built), not the program's. Its argument's use is the
				// body's, which P4 inlines where the argument leaves the parameter's
				// set (CheckCallSets). As a sink, windows' `io.print_int(p)` kept
				// p * p a word, trapped under -checked, where the inlined program
				// holds it in ℤ and prints 3¹²⁸.
			case s.Op == OCall:
				p := tg.Prims[s.Name]
				for j, a := range s.Args {
					if !(s.Name == "decimal" || j < len(p.Args) && bigType(p.Args[j]) || anyArg(p, j) && inU(a)) {
						sink[a] = true
					}
				}
			case s.Op == OIf && len(s.Sub) == 2:
				for j, v := range s.Res {
					for _, sub := range s.Sub {
						for _, args := range ownExits(sub, TYield) {
							if j < len(args) {
								join([]V{args[j]}, v)
							}
						}
					}
				}
				if len(s.Args) > 0 {
					sink[s.Args[0]] = true // a condition is a boolean
				}
			case s.Op == OLoop && len(s.Sub) == 1:
				body := s.Sub[0]
				for j, p := range body.Params {
					if j < len(s.Args) {
						join([]V{s.Args[j]}, p)
					}
					for _, args := range ownExits(body, TContinue) {
						if j < len(args) {
							join([]V{args[j]}, p)
						}
					}
				}
				for j, v := range s.Res {
					for _, args := range ownExits(body, TBreak) {
						if j < len(args) {
							join([]V{args[j]}, v)
						}
					}
				}
			case s.Op == OThe:
				if len(s.Args) == 1 && len(s.Res) == 1 {
					join(s.Args, s.Res[0])
				}
			default:
				for _, a := range s.Args {
					sink[a] = true // a table's index or element, a length, a store, …
				}
			}
		}
		if r.T == TBranch {
			sink[r.Cond] = true
		}
	})
	// THE FUNCTION'S RESULTS: a word's, a sink; ℤ's, not
	for _, args := range ownExits(f.Body, TYield) {
		for j, a := range args {
			ty := ""
			if j < len(f.Results) {
				ty = f.Results[j]
			}
			if !(resultIsZ(sig, j) || bigType(ty) || tg.ValueType(ty) == core.U64Type && inU(a)) {
				sink[a] = true
			}
		}
	}
	// A π IS ITS SOURCE, the same value renamed on an arm: tainted together,
	// inside the fixed point, so a chain of πs and what each source feeds are
	// followed (at(q, k + 1)'s index reads the guard's π of k + 1)
	var pis [][2]V
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			pis = append(pis, [2]V{pi.V, pi.Of})
		}
	})
	// THE LEAST FIXED POINT, backward: a tainted result taints its operands, a
	// tainted receiver its inputs
	t := map[V]bool{}
	for v := range sink {
		t[v] = true
	}
	for changed := true; changed; {
		changed = false
		for _, o := range operands {
			if t[o[1][0]] {
				for _, a := range o[0] {
					if !t[a] {
						t[a], changed = true, true
					}
				}
			}
		}
		for a, outs := range recv {
			if t[a] {
				continue
			}
			for _, out := range outs {
				if t[out] {
					t[a], changed = true, true
					break
				}
			}
		}
		for _, pi := range pis {
			if t[pi[0]] != t[pi[1]] {
				t[pi[0]], t[pi[1]], changed = true, true, true
			}
		}
	}
	return t
}
