package ir

import (
	"fmt"

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
	def := map[V]*Stmt{}
	// a use is a statement's operand j, a π (the same integer refined by its
	// arm's guard, whose own uses are then the value's), or anything else
	type use struct {
		s  *Stmt
		j  int
		pi V
	}
	uses := map[V][]use{}
	f.Walk(func(r *Region) {
		for _, a := range r.Args {
			uses[a] = append(uses[a], use{pi: -1})
		}
		if r.T == TBranch {
			uses[r.Cond] = append(uses[r.Cond], use{pi: -1})
		}
		for _, pi := range r.Pis {
			// the π's other side is the comparison's, read by no instruction
			uses[pi.Of] = append(uses[pi.Of], use{pi: pi.V})
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for j, a := range s.Args {
				uses[a] = append(uses[a], use{s: s, j: j, pi: -1})
			}
			for _, v := range s.Res {
				def[v] = s
			}
		}
	})
	ring := func(s *Stmt) bool {
		return (s.Op == OAdd || s.Op == OSub || s.Op == OMul || s.Op == ONeg) && s.Name == "" && len(s.Res) == 1
	}
	cand := map[V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			// unproven is its fact outside the word, whatever placeholder mode it
			// carries (`exact` without -checked, `trap` with it)
			if ring(s) && s.Mode != MWrap && s.Mode != MWrapPair && s.Via == "" &&
				(fs[s.Res[0]].v.finite() || PlantDoubleUnbounded) && !inWord(s.Res[0]) {
				cand[s.Res[0]] = true
			}
		}
	})
	if len(cand) == 0 {
		return 0
	}
	positive := func(v V, max int64) bool {
		d := def[v]
		return d != nil && d.Op == OConst && d.Lit != nil && d.Lit.Kind == core.KInt && d.Lit.Int > 0 && d.Lit.Int <= max
	}
	// an operand of a double-word computation: a word, or a candidate
	operand := func(v V) bool { return cand[v] || inWord(v) }
	var allowed func(u use) bool
	allowed = func(u use) bool {
		s := u.s
		if PlantDoubleEscape {
			return true
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
			return cand[s.Res[0]] || inWord(s.Res[0])
		case ODiv, ORem, OMod:
			return u.j == 0 && positive(s.Args[1], 1<<62) && inWord(s.Res[0])
		case OShr:
			return u.j == 0 && positive(s.Args[1], 127) && inWord(s.Res[0])
		case OEq, ONe, OLt, OLe, OGt, OGe:
			return true
		}
		return false
	}
	for changed := true; changed; {
		changed = false
		for v := range cand {
			ok := true
			for _, a := range def[v].Args {
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
		def[v].Mode = MDouble
		if r := fs[v].v; r.finite() {
			f.Types[v] = fmt.Sprintf("int %d %d", r.lo, r.hi)
		}
	}
	if len(cand) > 0 {
		f.facts = nil
	}
	return len(cand)
}

// PlantDoubleEscape lets a double-word intermediate reach any use, a table or
// a result included: TestADoubleWordNeverLeavesItsTerm must see it refused
// without, and accepted with, the fault.
var PlantDoubleEscape bool

// PlantDoubleUnbounded takes an intermediate into the double word, and counts
// it proven, whatever its fact: TestAnIntermediateOutsideEIsRefused must see
// the refusal go.
var PlantDoubleUnbounded bool

// Doubles are the values f holds in the double word: those its MDouble
// operations define, and their πs, the same integers refined.
func Doubles(f *Func) map[V]bool {
	out := map[V]bool{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Mode == MDouble {
				for _, v := range s.Res {
					out[v] = true
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
