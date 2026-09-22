package emit

import (
	"strings"

	"oroboros/core"
)

// THE UNSIGNED WORD — ADR 0026 (10), u64repr-2026-09-22.
//
// A target may realize two intervals at word width: its signed word
// S = [−2^63, 2^63−1], which realizes `int`, and U = [0, 2^64−1], which Go
// holds in a `uint64`. Neither contains the other; their join in the lattice
// of realizations is arbitrary precision. A binding is represented by the
// LEAST realization containing its interval, S preferred where both do:
//
//	ρ(I) = int   if I ⊆ S
//	       u64   if I ⊆ U, I ⊄ S
//	       big   otherwise        (not this file: bigrep.go, and only by declaration)
//
// THE THEOREM. The quotient map q : ℤ → ℤ/2^64 is a ring homomorphism, and S
// and U are both sets of representatives of ℤ/2^64 — each residue class meets
// each of them exactly once. So for ⋆ ∈ {+, −, ·}
//
//	q(a ⋆ b) = q(a) ⋆ q(b)
//
// and if the result is PROVEN to lie in a realization R, the residue names
// exactly one member of R, which is the result. So +, − and · may be computed
// with wrapping in either 64-bit type, after converting each operand by the
// residue map (Go's `uint64(x)` and `int(x)` ARE the residue map), and the
// answer is exact. The quotient does NOT preserve order, and does not commute
// with truncating division, so `<`, `=`, `/` and `%` are computed in a
// realization that CONTAINS both operands — where the conversions are the
// identity on the values that occur — or not selected at all.
//
// WHAT THIS PASS DOES is make every term's representation ρ of its interval
// and every binder's ρ of its binder's interval, inserting `u64-of` and
// `int-of-u64` where a value crosses from one to the other. Where the interval
// of the value lies in both realizations, the conversion is the identity on
// it; where it does not, the value is an operand of +, − or ·, and the theorem
// covers it. Nothing else converts.
//
// LEGALITY IS NOT DECIDED HERE. A LANGUAGE operation is proven only inside S
// (interval.go's `record`); this pass turns an operation proven in U into the
// target's `u64` primitive, which is then proven in U. If this pass did not run,
// an operation in U would stay a language operation and be REFUSED — loud, never
// a silent wrap.

// wordOps are the unsigned word's operations. A target that realizes U
// declares them in its own file, the way it declares its bignum's.
var wordOps = []string{
	"u64+", "u64-", "u64*", "u64/", "u64%",
	"u64<", "u64<=", "u64>", "u64>=", "u64=",
	"u64-of", "int-of-u64",
}

func wordOpArity(op string) int {
	if op == "u64-of" || op == "int-of-u64" {
		return 1
	}
	return 2
}

func isWordOp(name string) bool {
	return strings.HasPrefix(name, "u64") && name != "u64-of"
}

// wordLang is the language operator a u64 operation computes, for the
// transfer function: `u64+` is `+` on the integers it is proven to hold.
func wordLang(name string) (string, bool) {
	if name == "u64-of" || name == "int-of-u64" {
		return "", false
	}
	if strings.HasPrefix(name, "u64") {
		return strings.TrimPrefix(name, "u64"), true
	}
	return "", false
}

// inU reports I ⊆ U.
func inU(v ival) bool {
	return v.isBottom() || (v.bounded() && v.lo.sign() >= 0 && v.hi.le(bu(1<<64-1)))
}

// inS reports I ⊆ S, the target's signed word.
func (p *intervalPass) inS(v ival) bool { return v.fitsIn(p.tgt.Word) }

// wantU is ρ(I) = u64: inside U and outside the signed word.
func (p *intervalPass) wantU(v ival) bool {
	return p.tgt.Word.Unsigned && !p.inS(v) && inU(v)
}

// u64Term reports whether a REBUILT term is represented in U. It observes: a
// term is u64 because this pass made it so, because a primitive returns one, or
// because it names a binder that is one.
func (p *intervalPass) u64Term(t *core.Term) bool {
	if t == nil || p.u64 == nil {
		return false
	}
	if p.u64Val[t] {
		return true
	}
	switch t.Kind {
	case core.KName:
		return p.u64[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if pr, ok := p.tgt.Prims[op.Name]; ok {
				return p.tgt.ValueType(pr.Result) == core.U64Type
			}
		}
	}
	return false
}

// toRep converts a rebuilt term to the representation wanted, when it is not
// already in it. A jump has no value and is never converted.
func (p *intervalPass) toRep(t *core.Term, want bool) *core.Term {
	if p.u64Term(t) == want || isJumpTerm(t) {
		return t
	}
	// A CONVERSION BACK CANCELS: `u64-of (int-of-u64 x)` is x on every value
	// that reaches it, since both are the identity there.
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Op().Kind == core.KName {
		if n := t.Op().Name; (want && n == "int-of-u64") || (!want && n == "u64-of") {
			return t.Kids[1]
		}
	}
	p.rep.Worded++
	var out *core.Term
	if want {
		out = core.App(core.Name("u64-of"), t)
	} else {
		out = core.App(core.Name("int-of-u64"), t)
	}
	if want {
		p.u64Val[out] = true
	}
	return out
}

// peelWord strips the unsigned word's conversions, which are the identity on
// every value they convert.
func peelWord(t *core.Term) *core.Term {
	for t != nil && t.Kind == core.KApp && len(t.Kids) == 2 && t.Op().Kind == core.KName &&
		(t.Op().Name == "u64-of" || t.Op().Name == "int-of-u64") {
		t = t.Kids[1]
	}
	return t
}

// wordOpsIn counts the unsigned word's operations and conversions in a term —
// what the pass left, as opposed to how often its fixpoint sweeps rewrote.
func wordOpsIn(t *core.Term) int {
	if t == nil {
		return 0
	}
	n := 0
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		for _, w := range wordOps {
			if t.Op().Name == w {
				n++
				break
			}
		}
	}
	if t.Kind == core.KFn {
		return n + wordOpsIn(t.Closed())
	}
	for _, k := range t.Kids {
		n += wordOpsIn(k)
	}
	return n
}

// selectWord rewrites one language operation whose operands or result live in
// U. `kids` are the rebuilt operator and operands, `vals` the operands'
// intervals and `out` the result's. It returns the rebuilt term, or nil when
// the operation is not this pass's.
func (p *intervalPass) selectWord(op string, kids []*core.Term, vals []ival, out ival) *core.Term {
	if !p.tgt.Word.Unsigned || len(kids) != 3 || len(vals) != 2 {
		return nil
	}
	arith, cmp := langArith(op), langCompare(op)
	if !arith && !cmp {
		return nil
	}
	a, b := kids[1], kids[2]
	ua, ub := p.u64Term(a), p.u64Term(b)
	app := func(name string, x, y *core.Term) *core.Term { return core.App(core.Name(name), x, y) }
	switch {
	case op == "+" || op == "-" || op == "*":
		// ANY 64-BIT REALIZATION CONTAINING THE RESULT, by the homomorphism:
		// the operands are converted by the residue map whatever their sign.
		switch {
		case p.inS(out):
			if !ua && !ub {
				return nil // the ordinary case, and every program before U existed
			}
			return app(op, p.toRep(a, false), p.toRep(b, false))
		case inU(out):
			p.rep.Worded++
			t := app("u64"+op, p.toRep(a, true), p.toRep(b, true))
			p.u64Val[t] = true
			return t
		}
		return nil // not proven in either: the language operation stays, and is refused
	default:
		// ORDER AND DIVISION NEED A REALIZATION HOLDING BOTH OPERANDS, where the
		// conversions are the identity on every value that occurs.
		switch {
		case p.inS(vals[0]) && p.inS(vals[1]):
			if !ua && !ub {
				return nil
			}
			return app(op, p.toRep(a, false), p.toRep(b, false))
		case inU(vals[0]) && inU(vals[1]):
			p.rep.Worded++
			t := app("u64"+op, p.toRep(a, true), p.toRep(b, true))
			if cmp {
				return t
			}
			// THE QUOTIENT AND REMAINDER LIE IN U, and are ρ'd like any value:
			// back to `int` where the result is inside the signed word.
			p.u64Val[t] = true
			return p.toRep(t, p.wantU(out))
		}
		// A POSSIBLY-NEGATIVE `int` AGAINST A VALUE PAST 2^63: no realization
		// holds both. Left as the language operation, whose operand types then
		// disagree and are refused by name — correct and loud, and a two-part
		// comparison is the day a program needs one.
		return nil
	}
}

// wideRange reads a range whose endpoints no int64 holds — [0, 2^64−1] is the
// one that occurs — as an interval, for the two places a range is a FACT the
// analysis must see and a term cannot state it: a parameter's premise and a
// primitive's declared result. Ranges an int64 does hold are left to the paths
// that already read them, so nothing existing changes.
func wideRange(ty string) (ival, bool) {
	lo, hi, ok := core.IntRangeBig(ty)
	if !ok || (lo.IsInt64() && hi.IsInt64()) {
		return ival{}, false
	}
	l, okl := fromBig(lo)
	h, okh := fromBig(hi)
	if !okl || !okh {
		return ival{}, false
	}
	return ival{lo: l, hi: h}, true
}

// DeclaresWord reports whether U reaches a program BY DECLARATION: a signature
// names a range in U, or the term calls a primitive with a parameter or a
// result in U. It is
// syntactic and costs nothing, which is the point — the selection is a whole
// interval pass, and running it on every program doubled the compile time of
// one that has nothing in U (json-tree, 95 → 235 ms serially).
//
// The other way into U is COMPUTED — arithmetic on signed-word values proven
// past 2^63 − 1 — and that is found by the legality pass that runs anyway: it
// reports an unproven operation whose interval lies in U (IntervalReport.InU),
// and the driver then selects and checks again. So an accepted program pays
// nothing, and a refused one pays one more pass on the way to being accepted.
func DeclaresWord(tgt *Target, sig *core.Sig, t *core.Term) bool {
	if !tgt.Word.Unsigned {
		return false
	}
	if sig != nil {
		if tgt.ValueType(sig.Result) == core.U64Type {
			return true
		}
		for _, r := range sig.Results {
			if tgt.ValueType(r) == core.U64Type {
				return true
			}
		}
		for _, sp := range sig.Params {
			if tgt.ValueType(sp.Type) == core.U64Type {
				return true
			}
		}
	}
	var walk func(*core.Term) bool
	walk = func(t *core.Term) bool {
		if t == nil {
			return false
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if pr, ok := tgt.Prims[t.Op().Name]; ok {
				if tgt.ValueType(pr.Result) == core.U64Type {
					return true
				}
				for _, r := range pr.Results {
					if tgt.ValueType(r) == core.U64Type {
						return true
					}
				}
				for _, a := range pr.Args {
					if tgt.ValueType(a) == core.U64Type {
						return true // an argument must be converted into U
					}
				}
			}
		}
		if t.Kind == core.KFn {
			return walk(t.Closed())
		}
		for _, k := range t.Kids {
			if walk(k) {
				return true
			}
		}
		return false
	}
	return walk(t)
}

// SelectWords is the unsigned word's representation pass, run where
// `PromoteBig` runs and before the type checker, because the representation is
// part of what the program means to a host: `(* a b)` with a result in U must
// reach the Go backend as `uint64(a) * uint64(b)`, and the checker must see the
// `u64` it produces.
//
// A target that does not realize U gets its term back unchanged, and so does a
// program in which nothing lives in U — the same pointer, because a rebuilt
// term is a different key in every map keyed on one (SelectShifts' note).
func SelectWords(tgt *Target, sig *core.Sig, t *core.Term) (*core.Term, int) {
	if !tgt.Word.Unsigned {
		return t, 0
	}
	rep, out := intervals(tgt, sig, t, 0, nil, true, true, false, true, false, true)
	if rep.Worded == 0 {
		return t, 0
	}
	return out, wordOpsIn(out)
}
