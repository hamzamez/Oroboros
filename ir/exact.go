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

// PlantExactDemandOff leaves D_A empty: a product of three words past E is
// then refused again (TestAnIntermediatePastEIsTheIntegersOwn).
var PlantExactDemandOff bool

// exactDemand is D_A of the decided f, and its operations as the program wrote
// them, for the report.
func exactDemand(tg *emit.Target, f *Func, sig *core.Sig) (map[V]bool, []string) {
	if WordsOnly || PlantExactDemandOff || !tg.HoldsZ() {
		return nil, nil
	}
	fs := f.facts
	if len(fs) != f.NV() {
		return nil, nil
	}
	sLo, sHi := ei(tg.Word.Lo), ei(tg.Word.Hi)
	inU := func(x iv) bool { return (tg.Word.Unsigned || tg.Word.Pair) && x.within(ei(0), u64Max) }
	var e, taint map[V]bool
	out := map[V]bool{}
	var srcs []string
	inOrder(f, func(s *Stmt) {
		if len(s.Res) != 1 || s.Name != "" || s.Mode == MWrap || s.Mode == MWrapPair || !langOp(s) {
			return
		}
		if !(s.Op.IsArith() || s.Op.IsBit()) {
			return
		}
		v := s.Res[0]
		x := fs[v].v
		if x.bot || x.within(sLo, sHi) || inU(x) {
			return
		}
		if e == nil { // E's selection, on a clone: nothing marked
			c := f.Clone()
			c.facts = f.facts
			SelectDouble(tg, c)
			e = Doubles(c)
		}
		if e[v] {
			return
		}
		if taint == nil {
			taint = tainted(tg, f, sig)
		}
		if taint[v] && !PlantTaintIgnored {
			return // it would carry ℤ into a sink: a word, refused or trapped
		}
		out[v] = true
		if len(srcs) < 4 {
			src := s.Op.String()
			if s.Src != nil {
				src = core.ShowTerm(s.Src) // as J writes it
			}
			if len(src) > 48 {
				src = src[:45] + "..."
			}
			srcs = append(srcs, src)
		}
	})
	if len(out) == 0 {
		return nil, nil
	}
	return out, srcs
}

// exactNote is the report of D_A: how many operations the machine's readings
// do not hold, computed as the integers they are, and the first of them.
func exactNote(name string, d map[V]bool, srcs []string) string {
	more := ""
	if len(d) > len(srcs) {
		more = "; …"
	}
	return fmt.Sprintf("%s: %d operation(s) past every word and the double word, held exactly: %s%s",
		name, len(d), strings.Join(srcs, "; "), more)
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
			case s.Op == OCall:
				p := tg.Prims[s.Name]
				for j, a := range s.Args {
					if !(s.Name == "decimal" || j < len(p.Args) && bigType(p.Args[j])) {
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
			if !(resultIsZ(sig, j) || bigType(ty)) {
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
