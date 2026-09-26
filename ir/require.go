package ir

import (
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// RANGE OBLIGATIONS, DISCHARGED BY THE IR'S FACTS (ADR 0028, refinements.md
// §6b, the interval route of emit.DischargeRequires).
//
// A range mark `(#req d p τ a)` is the value ⟦a⟧, with the obligation
// ⟦a⟧ ∈ ⟦τ⟧: an argument inlined into d's parameter p, declared τ. Lowering is
// transparent to the mark and records the values a became. The interval
// domain's facts are sound over-approximations of those values, so
//
//	fact(v) ⊆ [lo, hi] for every v the argument became   ⇒   ⟦a⟧ ∈ ⟦τ⟧
//
// Checking against [lo, hi] is sound above the word too: ADR 0029's ⟦τ⟧ there
// is the least set a sign and a bit length decide, which contains [lo, hi].
// A mark with no recorded value, or one numbering dropped, decides nothing.
//
// DischargeRanges returns t with every discharged range mark replaced by its
// argument, and every other by what the refinement route still has to show:
// an obligation on [lo, hi] is the two half-lines lo ≤ a and a ≤ hi, and an end
// the facts establish is dropped from the residual `where`, so the two routes
// prove it together (0 ≤ Rem64(…) from the facts, Rem64(…) ≤ 10¹⁸ − 1 from
// Rem64's `ensures`). The sets are emit.DecideRange's, one definition for
// every route. A residual that cannot
// be lowered yet (a table of products, before FlattenProducts) has nothing
// decided: that is sound, the refinement route still sees every mark.
func DischargeRanges(tg *emit.Target, sig *core.Sig, t *core.Term) *core.Term {
	out, _ := decideMarks(tg, sig, t)
	if len(out) == 0 {
		return t
	}
	return replaceMarks(t, out)
}

// RangeVerdict is one range mark's verdict, for gen's -report-requires.
type RangeVerdict struct {
	Def, Param, Type, Arg, Got string
	Proven                     bool
}

// MeasureRanges reports the verdict DischargeRanges reaches on each range mark,
// and changes nothing.
func MeasureRanges(tg *emit.Target, sig *core.Sig, t *core.Term) []RangeVerdict {
	_, vs := decideMarks(tg, sig, t)
	return vs
}

// decideMarks is the decision of DischargeRanges: each decided mark's
// replacement, and each mark's verdict.
func decideMarks(tg *emit.Target, sig *core.Sig, t *core.Term) (map[*core.Term]*core.Term, []RangeVerdict) {
	f, err := Lower(tg, "require", sig, t, Options{})
	if err != nil || len(f.marks) == 0 {
		return nil, nil
	}
	fs := analyse(tg, f)
	out := map[*core.Term]*core.Term{}
	var verdicts []RangeVerdict
	for m, vs := range f.marks {
		if len(vs) == 0 || m.Kids[3].Kind != core.KStr {
			continue
		}
		x, known := argumentFact(fs, vs)
		if !known {
			continue
		}
		proven, residual := decideMark(tg, m, x)
		if proven {
			out[m] = m.Kids[4]
		} else {
			out[m] = residual
		}
		verdicts = append(verdicts, RangeVerdict{Def: m.Kids[1].Str, Param: m.Kids[2].Str, Type: m.Kids[3].Str,
			Arg: m.Kids[4].String(), Got: termShow(x), Proven: proven})
	}
	return out, verdicts
}

// replaceMarks replaces each decided mark: a proven one by its argument, and
// one proven only in part by the residual the refinement route must show.
// Marks inside a replacement are decided too.
func replaceMarks(t *core.Term, out map[*core.Term]*core.Term) *core.Term {
	if t == nil {
		return nil
	}
	if r, ok := out[t]; ok {
		if r == t.Kids[4] {
			return replaceMarks(r, out)
		}
		// the residual carries the argument as its last kid: decide inside it
		c := *r
		c.Kids = append([]*core.Term(nil), r.Kids...)
		c.Kids[len(c.Kids)-1] = replaceMarks(r.Kids[len(r.Kids)-1], out)
		return &c
	}
	if len(t.Kids) == 0 {
		return t
	}
	var kids []*core.Term
	for i, k := range t.Kids {
		nk := replaceMarks(k, out)
		if nk != k && kids == nil {
			kids = append([]*core.Term(nil), t.Kids...)
		}
		if kids != nil {
			kids[i] = nk
		}
	}
	if kids == nil {
		return t
	}
	c := *t
	c.Kids = kids
	return &c
}

// argumentFact is what the facts say of a mark's argument: the join over every
// value it became, since the obligation is on each. A value numbering dropped
// makes it unknown.
func argumentFact(fs []fact, vs []V) (iv, bool) {
	x := ivBot
	for _, v := range vs {
		if v < 0 || int(v) >= len(fs) {
			return iv{}, false
		}
		x = joinIV(x, fs[v].v)
	}
	return x, true
}

// decideMark is the obligation ⟦a⟧ ∈ ⟦τ⟧ on the argument's fact x, by the
// shared definition of the sets (emit.DecideRange). An infinite end of x is
// passed as open, never as the number canonical form leaves under it.
func decideMark(tg *emit.Target, m *core.Term, x iv) (bool, *core.Term) {
	var lo, hi *big.Int
	if !x.bot && !x.nlo {
		lo = x.lo.big()
	}
	if !x.bot && !x.phi {
		hi = x.hi.big()
	}
	return emit.DecideRange(tg.Word, m.Kids[3].Str, lo, hi, x.bot, m.Kids[1].Str, m.Kids[2].Str, m.Kids[4])
}
