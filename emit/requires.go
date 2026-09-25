package emit

import (
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
)

// A DEFINITION'S CONTRACT IS CHECKED AT ITS CALLS (ADR 0028, refinements.md §6b).
//
// Reduction inlines every non-exported call, so a declared parameter range or a
// `where` has no call site left once anything is analysed. The reducer marks
// them instead (core.Env.Requires, core.Env.Wheres):
//
//	(#req  "def" "param" "type" a)    a range, on the argument; its value is a
//	(#reqw "def" cond body)           a `where`, on the call's result; its value is body
//
// DischargeRequires decides every mark immediately after reduction and returns
// the residual with the marks erased — the term reduction gives without them —
// so nothing downstream ever sees one. The order is literal (decided by the
// reducer as it reduces), then the interval analysis, then the refinement layer
// with every fact in scope; an obligation none of them proves is refused.

// RequireSet carries one program's contracts and the reducer's verdicts on the
// literal ones, which arrive while a unit reduces.
type RequireSet struct {
	lits []requireFail
	// enforced is E_P, the one set the program enforces above the word (ADR
	// 0029): what a declared result there is known to be in. ok is false when
	// nothing is enforced — no bound, or an unbounded type.
	enforced   rangeSet
	enforcedOK bool
}

type requireFail struct {
	def, param, ty, arg, got, known string
	literal                         bool
}

// InstallRequires fills env's contract tables from the program's signatures.
// Every signed definition is covered, exported ones included: from outside an
// export's contract is assumed, and a call from inside the program is a call.
func InstallRequires(env *core.Env, prog *core.Program) *RequireSet {
	set := &RequireSet{}
	sigs := make([]*core.Sig, 0, len(prog.Sigs))
	for _, sig := range prog.Sigs {
		sigs = append(sigs, sig)
	}
	if bits, signed, ok := BigHull(env.Word, sigs...); ok {
		set.enforced, set.enforcedOK = rangeSet{bits: bits, signed: signed}, true
	}
	env.Requires = map[string][]string{}
	env.Wheres = map[string]core.WhereContract{}
	env.Prim[core.RequireName], env.Pure[core.RequireName] = true, true
	env.Prim[core.RequireWhereName], env.Pure[core.RequireWhereName] = true, true
	for name, sig := range prog.Sigs {
		if sig == nil {
			continue
		}
		if _, isDef := prog.Defs[name]; !isDef {
			continue
		}
		tys := make([]string, len(sig.Params))
		ranged := false
		for i, p := range sig.Params {
			if _, _, ok := core.IntRangeBig(p.Type); ok {
				tys[i], ranged = p.Type, true
			}
		}
		if ranged {
			env.Requires[name] = tys
		}
		// THE CALL'S OWN OBLIGATION: the `where` as written, and each range the
		// argument mark cannot carry — one with an infinite endpoint, whose finite
		// side is still a conjunct (`(int 0 +inf)` is `0 <= n`).
		cond := sig.Written
		ps := make([]string, len(sig.Params))
		for i, p := range sig.Params {
			ps[i] = p.Name
			if tys[i] != "" || p.Name == "" {
				continue
			}
			lo, hi, haveLo, haveHi := core.RangeBounds(p.Type)
			if haveLo {
				cond = conjTerm(cond, core.App(core.Name("<="), core.Int(lo), core.Name(p.Name)))
			}
			if haveHi {
				cond = conjTerm(cond, core.App(core.Name("<="), core.Name(p.Name), core.Int(hi)))
			}
		}
		if cond != nil {
			env.Wheres[name] = core.WhereContract{Params: ps, Cond: cond}
		}
	}
	env.OnRequire = func(def, param, ty string, arg *core.Term) {
		if param == "where" {
			set.lits = append(set.lits, requireFail{def: def, param: param, arg: arg.String(), literal: true})
			return
		}
		if arg.Kind == core.KInt && !literalIn(env.Word, ty, big.NewInt(arg.Int)) {
			set.lits = append(set.lits, requireFail{def: def, param: param, ty: ty, arg: arg.String(), literal: true})
		}
	}
	return set
}

// conjTerm is a conjunction in the reader's erased spelling, `(if a b false)`,
// which every consumer of a `where` already reads (booleans.md).
func conjTerm(a, b *core.Term) *core.Term {
	if a == nil {
		return b
	}
	return core.App(core.Name("if"), a, b, core.Bool(false))
}

// DischargeRequires decides every contract mark in one unit's residual and
// returns the residual without them, or the first obligation nothing proves.
func DischargeRequires(set *RequireSet, tgt *Target, what string, sig *core.Sig, t *core.Term) (*core.Term, error) {
	fails := set.lits
	set.lits = nil
	t, _ = set.decideAscribed(tgt.Word, t)
	if hasRequireMarks(t) {
		// The interval analysis decides the ranges it can and leaves every mark
		// it cannot — and every `where` — in the term it rebuilds. It runs only
		// when there is a range to decide: it has nothing to say about a `where`.
		rest := t
		if hasRangeMarks(t) {
			_, rest = intervals(tgt, sig, t, 0, nil, false, false, false, false, false, false, true)
		}
		// A CLOSED CONDITION NEEDS NO CONTEXT. `(go.< 20 1048576)` — a length
		// that folded, against a literal — is decided with no facts at all, and
		// when every mark left is one of those, the refinement walk, which reads
		// the facts at every point of the program, is not run.
		if hasRequireMarks(rest) && !closedWheresHold(tgt, rest) {
			r := &refiner{tgt: tgt, pure: pureAtoms(tgt), probe: true, requires: &[]requireFail{},
				memo: &refineMemo{houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
			f := newFacts()
			f.pure = r.pure
			if rest.Kind == core.KFn && sig != nil && sig.Where != nil {
				assume(f, sig.Where)
			}
			seedFacts(f, rest, langFacts)
			_ = r.walk(rest, f) // a dry walk: only the marks decide, in r.requires
			fails = append(fails, *r.requires...)
		}
	}
	// ONE CALL, ONE OBLIGATION: a range mark rides every copy of its argument,
	// so the same failure can be met at each use of the parameter.
	seen := map[string]bool{}
	var distinct []requireFail
	for _, f := range fails {
		k := f.def + "|" + f.param + "|" + f.ty + "|" + f.arg
		if !seen[k] {
			seen[k] = true
			distinct = append(distinct, f)
		}
	}
	if len(distinct) > 0 {
		return nil, fmt.Errorf("%s: %s", what, distinct[0].message(len(distinct)))
	}
	return core.StripRequires(t), nil
}

func (f requireFail) message(n int) string {
	var b strings.Builder
	switch {
	case f.param == "where" && f.literal:
		fmt.Fprintf(&b, "%s's `where` is false at a call: %s", f.def, f.arg)
	case f.param == "where":
		fmt.Fprintf(&b, "%s requires %s at a call, which does not follow\n  known: %s", f.def, f.arg, f.known)
	case f.literal:
		fmt.Fprintf(&b, "%s's parameter %s is declared (%s), and a call passes %s", f.def, f.param, f.ty, f.arg)
	default:
		fmt.Fprintf(&b, "%s's parameter %s is declared (%s), and at a call the argument %s is not proven in it",
			f.def, f.param, f.ty, f.arg)
		if f.got != "" {
			fmt.Fprintf(&b, "\n  its interval: %s", f.got)
		}
		if f.known != "" {
			fmt.Fprintf(&b, "\n  known: %s", f.known)
		}
	}
	b.WriteString("\n  A definition's declared parameter range and `where` are obligations at every call it is\n" +
		"  inlined into (ADR 0028). Prove the argument in range — narrow it, or declare the range of\n" +
		"  what it comes from — or widen the declaration if the definition is right outside it.")
	if n > 1 {
		fmt.Fprintf(&b, "\n  (%d obligations at calls fail; this is the first.)", n)
	}
	return b.String()
}

// hasRangeMarks reports a range mark, the kind the interval analysis decides.
func hasRangeMarks(t *core.Term) bool {
	if t == nil {
		return false
	}
	if core.IsRequire(t) {
		return true
	}
	if t.Kind == core.KFn {
		return hasRangeMarks(t.Kids[0])
	}
	for _, k := range t.Kids {
		if hasRangeMarks(k) {
			return true
		}
	}
	return false
}

// closedWheresHold reports that every mark left is a `where` whose condition
// has no free names and is proven with no facts, so no context could matter.
func closedWheresHold(tgt *Target, t *core.Term) bool {
	ok := true
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if !ok || x == nil {
			return
		}
		switch {
		case core.IsRequire(x):
			ok = false
			return
		case core.IsRequireWhere(x):
			cond := x.Kids[2]
			r := &refiner{tgt: tgt, pure: pureAtoms(tgt), probe: true,
				memo: &refineMemo{houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
			f := newFacts()
			f.pure = r.pure
			if !closedCond(tgt, cond) || !r.proveCond(cond, f) {
				ok = false
				return
			}
			walk(x.Kids[3])
			return
		case x.Kind == core.KFn:
			walk(x.Kids[0])
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return ok
}

// closedCond reports a condition that mentions no variable: no bound index, and
// every name a primitive of the target or one of the language's own operators.
func closedCond(tgt *Target, t *core.Term) bool {
	switch t.Kind {
	case core.KBound, core.KFn:
		return false
	case core.KName:
		if _, ok := tgt.Prims[t.Name]; ok {
			return true
		}
		switch t.Name {
		case "if", "and", "or", "not", "=", "<", "<=", ">", ">=", "+", "-", "*", "/", "%", "len":
			return true
		}
		return false
	case core.KApp:
		for _, k := range t.Kids {
			if !closedCond(tgt, k) {
				return false
			}
		}
	}
	return true
}

func hasRequireMarks(t *core.Term) bool {
	if t == nil {
		return false
	}
	if core.IsRequire(t) || core.IsRequireWhere(t) {
		return true
	}
	if t.Kind == core.KFn {
		return hasRequireMarks(t.Kids[0])
	}
	for _, k := range t.Kids {
		if hasRequireMarks(k) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- the set a range denotes

// rangeSet is the set a declared range denotes on a target: exact within the
// word, and ABOVE the word its least member of H, the sets a sign and a bit
// length decide (ADR 0029): [0, 2^bits) for a range with no negative value,
// (−2^bits, 2^bits) for one with, bits the bit length of max(|lo|, |hi|).
type rangeSet struct {
	lo, hi *big.Int // exact; nil when bitwise
	bits   int      // above the word: |x| < 2^bits
	signed bool     // above the word: negative values admitted
}

func setOf(w core.Word, ty string) (rangeSet, bool) {
	lo, hi, ok := core.IntRangeBig(ty)
	if !ok {
		return rangeSet{}, false
	}
	if !w.Exceeds(ty) {
		return rangeSet{lo: lo, hi: hi}, true
	}
	m := new(big.Int).Abs(lo)
	if a := new(big.Int).Abs(hi); a.Cmp(m) > 0 {
		m = a
	}
	return rangeSet{bits: m.BitLen(), signed: lo.Sign() < 0}, true
}

// has reports v in a set above the word.
func (s rangeSet) has(v *big.Int) bool {
	return (s.signed || v.Sign() >= 0) && new(big.Int).Abs(v).BitLen() <= s.bits
}

// within reports a ⊆ b for two sets above the word, H's order:
// Nⱼ ⊆ Nₖ, Zⱼ ⊆ Zₖ and Nⱼ ⊆ Zₖ exactly when j ≤ k, and Zⱼ ⊄ Nₖ.
func (a rangeSet) within(b rangeSet) bool {
	return a.bits <= b.bits && (!a.signed || b.signed)
}

func literalIn(w core.Word, ty string, v *big.Int) bool {
	s, ok := setOf(w, ty)
	if !ok {
		return true
	}
	if s.lo != nil {
		return v.Cmp(s.lo) >= 0 && v.Cmp(s.hi) <= 0
	}
	return s.has(v)
}

// decideAscribed discharges every range mark whose argument is a declared result
// above the word, `(the T e)`, and returns the term without them and how many.
//
// WHAT THE ASCRIPTION TELLS is not T's set but E_P, the program's, because that
// is all that is enforced: the bound is one per program (BigBound), so a result
// declared (int 0 2^100) in a program whose widest type is 2^200 is checked
// against 2^201 and may be 2^150. ADR 0028 read it as T's own set, a false
// proof ADR 0029 corrects. So the mark is discharged exactly when E_P lies in
// the parameter's set; any other is left for the analyses, which know nothing
// of a bignum's value, and is refused.
func (set *RequireSet) decideAscribed(w core.Word, t *core.Term) (*core.Term, []RequireResult) {
	if !set.enforcedOK || !hasRangeMarks(t) {
		return t, nil
	}
	var proven []RequireResult
	var walk func(*core.Term) *core.Term
	walk = func(x *core.Term) *core.Term {
		if x == nil || (x.Kind != core.KApp && x.Kind != core.KFn) {
			return x
		}
		if x.Kind == core.KFn {
			return core.FnClosed(x.Params, walk(x.Closed()))
		}
		if core.IsRequire(x) && len(x.Kids) == 5 {
			def, param, ty, arg := x.Kids[1].Str, x.Kids[2].Str, x.Kids[3].Str, x.Kids[4]
			if s, ok := setOf(w, ty); ok && s.lo == nil && ascribedAbove(w, arg) && set.enforced.within(s) {
				proven = append(proven, RequireResult{Def: def, Param: param, Type: ty,
					Arg: arg.String(), Got: top, Proven: true})
				return walk(arg)
			}
		}
		kids := make([]*core.Term, len(x.Kids))
		for i, k := range x.Kids {
			kids[i] = walk(k)
		}
		return &core.Term{Kind: core.KApp, Kids: kids}
	}
	out := walk(t)
	if len(proven) == 0 {
		return t, nil
	}
	return out, proven
}

// ascribedAbove reports `(the T e)` with T above the word.
func ascribedAbove(w core.Word, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName || t.Op().Name != core.AscribeName ||
		len(t.Args()) != 2 || t.Args()[0].Kind != core.KStr {
		return false
	}
	s, ok := setOf(w, t.Args()[0].Str)
	return ok && s.lo == nil
}

// provenIn decides a range mark on the interval analysis's evidence, the
// argument's interval.
func provenIn(w core.Word, v ival, ty string) bool {
	if v.isBottom() {
		return true // unreachable
	}
	s, ok := setOf(w, ty)
	if !ok {
		return false
	}
	// A RANGE HOLDING THE WHOLE SIGNED WORD is vacuous for an integer argument:
	// every `int` value on a target is held in its word (ADR 0026), whatever
	// interval the analysis found for it.
	if s.lo != nil && s.lo.IsInt64() && s.hi.IsInt64() && s.lo.Int64() <= w.Lo && s.hi.Int64() >= w.Hi {
		return true
	}
	if s.lo != nil {
		if s.lo.IsInt64() && s.hi.IsInt64() {
			return within(v, rng(s.lo.Int64(), s.hi.Int64()))
		}
		wr, ok := wideRange(ty)
		return ok && within(v, wr)
	}
	// Bitwise: a finite interval is under 2^127, the bound's own headroom. The
	// set has no negative value unless the range declares one (ADR 0029).
	if v.loInf || v.hiInf {
		return false
	}
	if !s.signed && v.lo.sign() < 0 {
		return false
	}
	if s.bits >= 127 {
		return true
	}
	lim := new(big.Int).Lsh(big.NewInt(1), uint(s.bits))
	lim.Sub(lim, big.NewInt(1))
	l, okl := fromBig(new(big.Int).Neg(lim))
	h, okh := fromBig(lim)
	return okl && okh && within(v, ival{lo: l, hi: h})
}

// residualRange is what the interval analysis hands the refinement layer for a
// range it could not prove: within the word, a condition naming only the ends
// it did not establish — each layer proves what it can, and `0 <= Rem64(…)`
// from the interval with `Rem64(…) <= 10^18 − 1` from Rem64's `ensures` is a
// proof neither has alone. The condition rides a where-mark labelled with the
// parameter, so a refusal still names it. Above the word the set is bitwise,
// which the linear fragment cannot state, and the range mark stays as it is.
func residualRange(w core.Word, v ival, def, param, ty string, arg *core.Term) *core.Term {
	s, ok := setOf(w, ty)
	if !ok || s.lo == nil || !s.lo.IsInt64() || !s.hi.IsInt64() {
		return &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name(core.RequireName),
			core.Str(def), core.Str(param), core.Str(ty), arg}}
	}
	lo, hi := s.lo.Int64(), s.hi.Int64()
	var cond *core.Term
	if v.loInf || v.lo.lt(bi(lo)) {
		cond = core.App(core.Name("<="), core.Int(lo), arg)
	}
	if v.hiInf || v.hi.gt(bi(hi)) {
		cond = conjTerm(cond, core.App(core.Name("<="), arg, core.Int(hi)))
	}
	label := fmt.Sprintf("%s's parameter %s, declared (%s),", def, param, ty)
	return &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name(core.RequireWhereName), core.Str(label), cond, arg}}
}

// ---------------------------------------------------------------- the refinement layer's half

// requireMark decides a mark the interval analysis left, with the facts in
// scope, and walks on into what it wraps. Only the top-level walk records.
func (r *refiner) requireMark(t *core.Term, f *facts) error {
	// A CALL'S ARGUMENTS ARE EVALUATED BEFORE IT, so the guarantees of the pure
	// calls inside them are in scope — collected on a copy, as `walk` collects
	// them before a primitive's `where` (refine.go), so they reach this goal and
	// nothing after it.
	if core.IsRequireWhere(t) {
		cond := t.Kids[2]
		g := f.clone()
		r.collectEnsures(cond, g)
		if r.requires != nil && !r.proveCond(cond, g) {
			*r.requires = append(*r.requires, requireFail{def: t.Kids[1].Str, param: "where",
				arg: cond.String(), known: f.known()})
		}
		return r.walk(t.Kids[3], f)
	}
	// A RANGE MARK THAT REACHES HERE IS ABOVE THE WORD. Within the word the
	// interval analysis either proved the range or handed on the ends it could
	// not, as a condition on a where-mark (residualRange); above it the set is
	// bitwise, which the linear fragment cannot state, so what the interval
	// analysis did not prove is not proven.
	a := t.Kids[4]
	if err := r.walk(a, f); err != nil {
		return err
	}
	if r.requires != nil {
		*r.requires = append(*r.requires, requireFail{def: t.Kids[1].Str, param: t.Kids[2].Str,
			ty: t.Kids[3].Str, arg: a.String(), known: f.known()})
	}
	return nil
}

// proveCond is discharge's decision for a condition that is not a primitive's:
// a disequality by either side, a linear conjunction by entailment, an opaque
// atom by an identical assumption. It proves or it does not; it never refuses.
func (r *refiner) proveCond(cond *core.Term, f *facts) bool {
	if lo, hi, isNe := disequality(cond); isNe {
		return f.entailsEither(lo, hi)
	}
	goals, ok := f.oblig(cond)
	if !ok {
		return f.entailsOpaque(opaqueKey(cond))
	}
	for _, g := range goals {
		if !f.entails(g) {
			budget := splitBudget
			return r.provedBySplit(cond, f, &budget)
		}
	}
	return true
}
