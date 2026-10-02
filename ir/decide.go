package ir

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// THE DECISION (spec §7, item 2 of the step from IR_A to IR_P): every counted
// integer operation is proven inside its representation, or it is not, and
// that decides both whether the program is legal and each operation's mode.
//
// The population is the term analysis's (`record` in emit/interval.go),
// measured equal on every function of the corpus (irstep4b-2026-09-26):
//   - `add`, `sub`, `mul` and `neg` in the language, proven inside the
//     target's signed word W;
//   - `u64+`, `u64-` and `u64*`, proven inside U = [0, 2⁶⁴);
//   - not a host operation lowering promoted, which is the host's where it is
//     not ℤ's, and legal either way; not a value only an `assume` reads.
//
// The rule (ADR 0019, ADR 0026): an operation proven inside its set is exact.
// One that is not is REFUSED, unless the build asked for the trap
// (`-checked`), when it is `trap` where the primitive declares a checked form.
// An unproven operation whose primitive declares none stays exact under
// `-checked`: that is what the term selection did (it renamed only to a
// declared `Checked`), and the IR keeps it rather than change which programs
// compile in the same step that moves the decision.

// Legality is the decision's report.
type Legality struct {
	Ops, Proven int
	Unproven    []string // "prim [lo, hi] in source", the refusal's lines
	// Outside: an unproven operation IS bounded, only not by this target's
	// word, so declaring the range is the answer (ADR 0026).
	Outside bool
	// InU: an unproven operation's interval lies in U, which the unsigned word
	// would hold (wordsel.go): the pipeline selects words and decides again.
	InU     bool
	Trapped int // operations given `trap`
	// Loops and Halts: the loops, and those size-change termination proves
	// (sct.go). A count, not a refusal: termination is reported, as the term
	// analysis reported it.
	Loops, Halts int
	// Sizes and SizeUnproven: the tables a function allocates (`build`,
	// `tabulate`), and those whose size is not proven in [0, max-len_T], the
	// domain of allocation (tables.md §2.3.1): below 0 on either end's side.
	Sizes        int
	SizeUnproven []string
}

// Decide analyses f, writes each counted operation's mode, and reports. With
// checked false an unproven operation keeps `exact`, and the program is to be
// refused (Refusal).
func Decide(tg *emit.Target, f *Func, checked bool) *Legality {
	rep := &Legality{}
	a := newIntervals(tg, f)
	a.region(f.Body)
	fs := a.fs
	f.facts = fs
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OLoop {
				rep.Loops++
				if h, seen := a.halts[s]; !seen || h {
					rep.Halts++ // an unevaluated loop is never entered
				}
			}
		}
	})
	// THE SIZE OF AN ALLOCATION is in [0, max-len_T]: a table's length is
	// exactly its size, and a target realizes lengths up to max-len (Java's
	// `new T[n]` takes an int, so 2³¹ − 1). Measured here, per end.
	maxLen := ei(tg.MaxLenOf())
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if (s.Op != OBuild && s.Op != OTabulate) || len(s.Args) == 0 {
				continue
			}
			rep.Sizes++
			n := fs[s.Args[0]].v
			switch {
			case n.bot || n.within(ep{}, maxLen):
			default:
				ends := ""
				if n.nlo || n.lo.sign() < 0 {
					ends += " below 0"
				}
				if n.phi || maxLen.lt(n.hi) {
					ends += " above max-len"
				}
				rep.SizeUnproven = append(rep.SizeUnproven, fmt.Sprintf("%s size %s:%s", s.Op, termShow(n), ends))
			}
		}
	})
	spec := specOnly(f)
	wLo, wHi := ei(tg.Word.Lo), ei(tg.Word.Hi)
	inOrder(f, func(s *Stmt) {
		if len(s.Res) != 1 || spec[s.Res[0]] {
			return
		}
		arith := (s.Op == OAdd || s.Op == OSub || s.Op == OMul || s.Op == ONeg) && s.Name == ""
		word := s.Op == OCall && (s.Name == "u64+" || s.Name == "u64-" || s.Name == "u64*")
		if !arith && !word {
			return
		}
		v := fs[s.Res[0]].v
		proven := v.within(wLo, wHi)
		if word {
			proven = tg.Word.Unsigned && v.within(ep{}, u64Max)
		}
		rep.Ops++
		if proven {
			rep.Proven++
			if arith {
				s.Mode = MExact
			}
			return
		}
		rep.Unproven = append(rep.Unproven, unprovenLine(s, v))
		if v.finite() {
			rep.Outside = true
		}
		if tg.Word.Unsigned && v.within(ep{}, u64Max) {
			rep.InU = true
		}
		if arith && checked {
			if prim, ok := tg.Prims[srcName(s)]; ok && prim.Checked != "" {
				s.Mode = MTrap
				rep.Trapped++
			}
		}
	})
	return rep
}

// inOrder visits f's statements in PROGRAM ORDER, depth first: a statement's
// nested regions before the statement after it, a branch's two arms after its
// region's statements. A refusal lists its first lines, so they are the first
// in the source, as the term analysis listed them.
func inOrder(f *Func, visit func(s *Stmt)) {
	var walk func(r *Region)
	walk = func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, sub := range s.Sub {
				walk(sub)
			}
			visit(s)
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
}

// Refusal is the error that refuses the program, naming it `what`, or nil
// when every counted operation is proven (ADR 0019).
func (l *Legality) Refusal(what string, tg *emit.Target) error {
	if l.Proven == l.Ops {
		return nil
	}
	return fmt.Errorf("%s", emit.UnboundedText(what, l.Ops, l.Proven, tg, l.Outside, l.Unproven))
}

// srcName is the primitive a statement lowers, from its provenance.
func srcName(s *Stmt) string {
	if s.Name != "" {
		return s.Name
	}
	if s.Src != nil && len(s.Src.Kids) > 0 {
		return s.Src.Kids[0].Name
	}
	return s.Op.String()
}

// unprovenLine is a refusal's line: the primitive, its interval, and the
// source application, as the term analysis wrote it.
func unprovenLine(s *Stmt, v iv) string {
	src := s.Op.String()
	if s.Src != nil {
		src = s.Src.String()
	}
	if len(src) > 64 {
		src = src[:61] + "..."
	}
	return fmt.Sprintf("%s %s in %s", srcName(s), termShow(v), src)
}

// termShow spells an interval as the term analysis does: [lo, hi], -inf and
// +inf.
func termShow(a iv) string {
	if a.bot {
		return "[1, 0]"
	}
	lo, hi := a.lo.String(), a.hi.String()
	if a.nlo {
		lo = "-inf"
	}
	if a.phi {
		hi = "+inf"
	}
	return "[" + lo + ", " + hi + "]"
}

// ToP is the step from IR_A to IR_P for one decided function: Finalize, then
// the verifier (W1–W10).
func ToP(tg *emit.Target, f *Func) error {
	// NOTHING IS PRINTED FROM A RESIDUAL THAT STILL CARRIES AN OBLIGATION. Every
	// printer comes through here, so this is where "decided" is required and
	// not assumed (hazard-2026-10-02).
	if f.undecided > 0 {
		return fmt.Errorf("%s: %d contract obligation(s) reached the printer undecided: the residual was "+
			"lowered with its marks, so nothing proved them. Decide them first (ir.DecideMarks, which "+
			"ir.Entry and every FromResidual call)", f.Name, f.undecided)
	}
	p := &Program{Target: tg.Name, Funcs: []*Func{f}}
	if err := Finalize(tg, p); err != nil {
		return err
	}
	if err := Verify(tg, p); err != nil {
		return fmt.Errorf("%s: IR_P does not verify:\n%v", f.Name, err)
	}
	return nil
}

// Clone is a deep copy of f: its regions, statements and types. Provenance is
// shared, since nothing writes a term.
func (f *Func) Clone() *Func {
	out := *f
	out.Params = append([]V(nil), f.Params...)
	out.Results = append([]string(nil), f.Results...)
	out.Types = append([]string(nil), f.Types...)
	out.Body = cloneRegion(f.Body)
	return &out
}

func cloneRegion(r *Region) *Region {
	if r == nil {
		return nil
	}
	out := *r
	out.Params = append([]V(nil), r.Params...)
	out.Pis = append([]Pi(nil), r.Pis...)
	out.Args = append([]V(nil), r.Args...)
	out.Stmts = append([]Stmt(nil), r.Stmts...)
	for i, s := range out.Stmts {
		s.Args = append([]V(nil), s.Args...)
		s.Res = append([]V(nil), s.Res...)
		if s.Sub != nil {
			sub := make([]*Region, len(s.Sub))
			for k, x := range s.Sub {
				sub[k] = cloneRegion(x)
			}
			s.Sub = sub
		}
		out.Stmts[i] = s
	}
	out.Then, out.Else = cloneRegion(r.Then), cloneRegion(r.Else)
	return &out
}

// CheckEnsures decides an exported definition's POSTCONDITION against its body
// (postconditions.md §2): the caller is outside the program, so the body is
// the only evidence. The postcondition's constant bounds denote a set S
// (emit.EnsuresBounds); the body's result fact R is the join of the values the
// function yields, from the decision's facts (Decide runs first). The
// obligation is discharged iff R ⊆ S.
//
// A postcondition outside that fragment is REFUSED, not noted: an obligation
// is discharged or the program is refused (refinements.md §3a). The term
// analysis passed such a claim with "propagated and not proven", which is the
// shape that rule removed everywhere else.
func CheckEnsures(tg *emit.Target, f *Func, sig *core.Sig) error {
	if sig == nil || sig.Ensures == nil {
		return nil
	}
	// ABOVE THE WORD a result's range is a representation declaration, not a
	// contract (ADR 0026): only what the author wrote is checked.
	ens := sig.Ensures
	if tg.ValueType(sig.Result) == core.BigType {
		if ens = sig.Stated(); ens == nil {
			return nil
		}
	}
	lo, hi, decided := emit.EnsuresBounds(tg, ens)
	if !decided {
		return fmt.Errorf("the postcondition %s is outside what the compiler decides "+
			"(constant bounds on the result, and their conjunctions); an obligation is "+
			"discharged or the program is refused (refinements.md §3a)", ens)
	}
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
	}
	r := ivBot
	for _, y := range exitsOf(f.Body, TYield) {
		if len(y) > 0 {
			r = joinIV(r, fs[y[0]].v)
		}
	}
	// S's ends, exactly. A bound K past E on its own side is met by every
	// finite end, so it is E's edge (which still asks R's end to be finite);
	// one past E on the far side admits no end at all, only R = ⊥.
	s, empty := ivTop, false
	if lo != nil {
		if e, ok := fromBig(lo); ok {
			s.lo, s.nlo = e, false
		} else if lo.Sign() < 0 {
			s.lo, s.nlo = eMax.neg(), false
		} else {
			empty = true
		}
	}
	if hi != nil {
		if e, ok := fromBig(hi); ok {
			s.hi, s.phi = e, false
		} else if hi.Sign() > 0 {
			s.hi, s.phi = eMax, false
		} else {
			empty = true
		}
	}
	if !r.bot && (empty || meetIV(r, s) != r) {
		return fmt.Errorf("the body does not establish %s; its result is %s", ens, termShow(r))
	}
	return nil
}

// SizeRefusal refuses an allocation whose size is not proven in
// [0, max-len_T]: `build` and `tabulate` make a table whose length is exactly
// the size, and a target realizes lengths up to max-len (tables.md §2.3.1).
// It is the allocation's domain condition, so `-checked` does not clear it,
// as it does not clear an index (refinements.md §3a). Java realized a size
// past 2³¹ − 1 as `new T[(int) n]`, of length n mod 2³² (irstep3java §3).
func (l *Legality) SizeRefusal(what string, tg *emit.Target) error {
	if len(l.SizeUnproven) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %d table allocation(s) cannot be proven to have a size in [0, %d], the lengths target %s holds\n  %s\n"+
		"  A table's length is exactly its size (tables.md §2.3.1), and outside that range the host allocates\n"+
		"  a different length or traps. Narrow the size, or declare the range it is computed from.",
		what, len(l.SizeUnproven), tg.MaxLenOf(), tg.Name, l.SizeUnproven[0])
}
