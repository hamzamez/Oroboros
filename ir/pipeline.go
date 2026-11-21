package ir

import (
	"errors"
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// THE PIPELINE EVERY DRIVER RUNS, from an entry point's residual to its decided
// IR: `gen`, `build`, `cmd/intervals` and the tests take a program through the
// same steps in the same order, so what one of them proves or refuses is what
// the others do.
//
//  0. the contract marks decided (DecideMarks, ADR 0037): a driver has done it
//     already, to report, and a caller that has not has it done here;
//  1. products flattened (emit/product.go);
//  2. the plan for the rung above the word (ADR 0034, 0035); the term keeps
//     only the ascriptions above the word, which are its demands;
//  3. the checks on terms: types, linearity, the refinement layer;
//  4. lowering, and the decision (spec §7);
//  5. the rung above the word, and the decision again;
//  6. the unsigned word (ADR 0033), and the decision again;
//  7. a postcondition, an obligation on an export (postconditions.md §2).
//
// The refusal of an unproven operation, the size refusal and the shift
// selection follow in the driver, which knows whether `-checked` was asked for
// and how it reports.

// DecideMarks decides every contract mark in a unit's residual (ADR 0028) and
// returns the residual without them, or the first obligation nothing proves:
// a literal by evaluation, a range on the IR's intervals, and what is left in
// the refinement layer. A residual with no mark is returned as it is.
//
// It is the one place the three routes are put together. Every path from a
// residual to emitted text goes through it: ir.Entry, each backend's
// FromResidual, and the drivers, which call emit.DischargeRequires themselves
// to report on the way. all is the program's signatures, for the one set it
// enforces above the word; nil enforces none, which proves less.
func DecideMarks(tg *emit.Target, name string, sig *core.Sig, nf *core.Term, all []*core.Sig) (*core.Term, error) {
	if !emit.HasRequireMarks(nf) {
		return nf, nil
	}
	return emit.DischargeRequires(emit.RequiresOf(tg.Word, all...), tg, name, sig, nf,
		func(x *core.Term) (*core.Term, error) { return DischargeRanges(tg, sig, x) })
}

// NoteKind is which step a note comes from, so a driver can phrase it.
type NoteKind int

const (
	NoteFlattened NoteKind = iota // N product accesses flattened
	NoteRefine                    // a refinement-layer note, Text
	NoteBig                       // N operations in arbitrary precision
	NoteWord                      // N operations or conversions in the unsigned word
)

// Note is one note, in the order the steps make them.
type Note struct {
	Kind NoteKind
	N    int
	Text string
}

// Entry takes one entry point through the pipeline. name is the unit's name,
// for messages and the checks; irName is the lowered function's. note receives
// each note as it is made, so a refusal later still follows the notes before it.
//
// WHICH DEFINITIONS ARE KEPT NEVER CHANGES A VERDICT (ADR 0102, P4): a call of
// a program function whose arguments are not in its parameters' sets inlines
// that instance, and the entry is taken through the pipeline again, at most
// once per instance, so the program is accepted wherever inlining accepts it.
func Entry(tg *emit.Target, name, irName string, sig *core.Sig, nf *core.Term, all []*core.Sig,
	checked bool, note func(Note)) (*Func, *Legality, error) {
	if note == nil {
		note = func(Note) {}
	}
	// AN ENTRY POINT'S HEADER IS THE HOST'S INTERFACE (integers-boundary §2):
	// an `Int` its header does not bound inside the word is the host's integer
	// of any size, and a target with none (windows: K(ℕ*) is the program's own
	// numeral, no C type a caller passes) cannot take it, a true answer
	if es := emit.EntrySig(tg, sig); es != sig {
		for i, p := range es.Params {
			if p.Type != sig.Params[i].Type && !tg.HasBig() {
				return nil, nil, fmt.Errorf("%s: its parameter %s ∈ Int is any integer, and an entry point of target %s "+
					"takes no integer above its word: its callers are C's, and C has none (K(ℕ*) is this "+
					"program's own). Bound it, `where %s <= …`, or give it a range inside the word",
					name, core.ShowName(p.Name), tg.Name, core.ShowName(p.Name))
			}
		}
		sig = es
	}
	inline := map[string]bool{}
	for {
		f, leg, err := entry(tg, name, irName, sig, nf, all, checked, note, inline)
		var cs *CallSetError
		if err == nil || PlantNoInlineFallback || !errors.As(err, &cs) || inline[cs.instance()] {
			return f, leg, err
		}
		inline[cs.instance()] = true
	}
}

func entry(tg *emit.Target, name, irName string, sig *core.Sig, nf *core.Term, all []*core.Sig,
	checked bool, note func(Note), inline map[string]bool) (*Func, *Legality, error) {
	// A TYPING MARK (ADR 0137) is the type checker's, and no other pass reads
	// it: the residual with its marks is kept for the checker below, and every
	// pass reads it erased.
	var typed *core.Term
	tsig := sig // the header before products are flattened, as typed is
	if emit.HasTypingMarks(nf) {
		typed, nf = nf, emit.EraseTypingMarks(nf)
	}
	// AN OBLIGATION IS DECIDED BEFORE ANYTHING ELSE READS THE RESIDUAL. A driver
	// that reports on the marks decides them itself, first, and then there are
	// none here; any other caller's are decided now, by the same routes.
	nf, err := DecideMarks(tg, name, sig, nf, all)
	if err != nil {
		return nil, nil, err
	}
	if nfl, fsig, k, err := emit.FlattenProducts(tg, sig, nf); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	} else if k > 0 {
		nf, sig = nfl, fsig
		note(Note{Kind: NoteFlattened, N: k})
	}
	plan, err := emit.PlanBig(tg, sig, nf, all...)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	plan.Inline = inline
	checking := emit.KeepSortAscriptions(tg.Word, plan.Holds(), nf)
	if plan.Holds() {
		nf = emit.EraseWordAscriptions(tg.Word, nf)
	} else {
		nf = emit.EraseAscriptionsButClaims(tg.Word, nf)
	}
	// Check the residual before emitting it (docs/spec/types.md). On Go and
	// Java the host would catch most of this; on JavaScript nothing would. It
	// reads the residual with a buffer's declared sort still on it.
	if err := emit.Check(tg, name, checking); err != nil {
		return nil, nil, err
	}
	// THE TYPING MARKS, each a binding of its claim nothing reads, checked on
	// the residual as it was before the passes above took it apart.
	if typed != nil {
		if err := emit.Check(tg, name, emit.TypingMarksAsLets(emit.AscribeParams(tsig, emit.KeepSortAscriptions(tg.Word, plan.Holds(), typed)))); err != nil {
			return nil, nil, fmt.Errorf("%w (the claim of a binding nothing reads, decided where it is written: ADR 0137)", err)
		}
	}
	// ADR 0018's linearity, checked on the residual rather than by a type.
	if err := emit.CheckLinear(nf, tg, sig); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	// The refinement layer: every index and divisor obligation
	// (docs/spec/refinements.md).
	notes, err := emit.Refine(tg, name, sig, nf)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range notes {
		note(Note{Kind: NoteRefine, Text: n})
	}
	// THE DECISION (ADR 0032 step 4, spec §7).
	f, err := Lower(tg, irName, sig, nf, Options{Decided: true})
	if err != nil {
		return nil, nil, err
	}
	// THE NUMBERS OF N (ADR 0086, numbers.go): each value of a decimal grid
	// held as its numerator, and each operation of N and each rounding
	// rewritten into integer operations, which the decision below proves.
	if _, err := SelectNumbers(tg, f); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	// FUSION (ADR 0082 item 3): a term under `mod 2ⁿ` computed wrapping,
	// before the decision, which would count its intermediates.
	Fuse(tg, f)
	leg := Decide(tg, f, checked)
	// A POSTCONDITION on an exported definition is an obligation: the caller
	// is outside the program, so the body is the only evidence there is. It is
	// decided HERE, on ℤ's facts, before any representation is chosen: a
	// representation is an abstraction function (ADR 0083), so the value the
	// claim is about is the same on every one, and past this point a value
	// held above the word is a host call's, whose fact is ⊤
	// (resultclaim-2026-11-02).
	// A CLAIM THE INTERVALS DO NOT STATE, a formula or arithmetic relating the
	// result to the parameters, is decided on the residual by the logic (ADR
	// 0145), and the intervals decide the result's range alone. CheckEnsures
	// itself refuses what it cannot decide, so no caller that skips this step
	// accepts a claim unchecked.
	ens := sig
	if emit.FormulaClaim(tg, sig) {
		if err := emit.DecideClaim(tg, name, sig, nf); err != nil {
			return nil, nil, err
		}
		s := *sig
		s.Ensures = sig.ResultRange
		ens = &s
	}
	if err := CheckEnsures(tg, f, ens); err != nil {
		// THE INTERVALS FORGET RELATIONS: a claim they do not establish is
		// decided by the logic before it is refused (emit.DecideRangeClaim)
		if emit.DecideRangeClaim(tg, name, ens, nf) != nil {
			return nil, nil, fmt.Errorf("%s: %v", name, err)
		}
	}
	// A λ GIVEN TO A HOST ANSWERS IN ITS CLAIM'S RESULT (ADR 0143), decided
	// here on ℤ's facts, as a postcondition is.
	if err := CheckCallbackResults(tg, f); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	if err := CheckCallbackEffects(tg, f); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	// D_A (exact.go): an operation the machine's readings and the double word
	// do not hold is held exactly, on the target's own rung where the program
	// asked for none, and reported; `-words` refuses it instead
	if d, srcs := exactDemand(tg, f, sig); len(d) > 0 {
		f.exactDemand = d
		if !plan.Holds() {
			// THE REPORT where A decided: the program declared nothing above the
			// word, and its operations past the readings are named; one that did
			// is reported by the rung's count, as before
			note(Note{Kind: NoteRefine, Text: exactNote(name, d, srcs)})
			plan = emit.BigPlan{Host: tg.HasBig(), Var: !tg.HasBig(), Signed: !tg.HasBig(), Inline: plan.Inline}
		}
	}
	// THE RUNG ABOVE THE WORD: the least set of values held exactly, on the
	// decided function's facts; decided again after, where no operation held
	// exactly is counted.
	if plan.Holds() {
		n, err := SelectRung(tg, f, sig, plan)
		f.exactDemand = nil
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", name, err)
		}
		if n > 0 {
			note(Note{Kind: NoteBig, N: n})
		}
		// decided again even where nothing is held exactly: the rung erases
		// the ascriptions and renames what they named, and every pass after
		// reads the facts (a results-only plan skipping this gave SelectShifts
		// stale facts, TestTheLoopRulesAreSound's case 6)
		leg = Decide(tg, f, checked)
		// A NUMERAL'S RESULT IS A CLAIM ITS BODY PROVES (ADR 0072, on a
		// table): every returned table's digits in [0, B), by its stores
		for _, r := range f.Results {
			if r == NumeralType(tg) {
				if err := proveResultElems(tg, f, ei(0), digitMax(tg)); err != nil {
					return nil, nil, fmt.Errorf("%s: a result held as a numeral: %v", name, err)
				}
				break
			}
		}
	}
	leg, err = afterRung(tg, f, name, sig, checked, leg, note)
	if err != nil {
		return nil, nil, err
	}
	return f, leg, nil
}

// PlantNoWordsOnImage leaves the unsigned word unchosen on ι's image, as
// before ADR 0126: TestTheWordsAreChosenOnTheRealizationsImage must see W5.
var PlantNoWordsOnImage bool

// afterRung is the pipeline after the rung above the word, shared by an entry
// and a program function (ADR 0101): the unsigned word, its pair, storage,
// release, reuse and the target's interpretation, each decided again.
func afterRung(tg *emit.Target, f *Func, name string, sig *core.Sig, checked bool, leg *Legality, note func(Note)) (*Legality, error) {
	// THE DOUBLE WORD (ADR 0103, 0105, ir/double.go), FIRST: an intermediate
	// outside both words that its term reads back into one, computed in i128
	// where the decision proves it in one of i128's readings; decided again.
	// The unsigned word then holds what lies in U, the double word's naturals
	// among them, and leaves the double word's operations alone.
	if SelectDouble(tg, f) > 0 {
		leg = Decide(tg, f, checked)
	}
	// THE UNSIGNED WORD: each integer value's realization is ρ of its fact;
	// the decision is taken again on the selected function.
	if changed, err := SelectWords(tg, f); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	} else if changed {
		note(Note{Kind: NoteWord, N: WordOps(f)})
		leg = Decide(tg, f, checked)
	}
	// THE UNSIGNED WORD AS A PAIR (ADR 0083, ir/pairs.go), on a target whose word
	// is narrower: every value held there, and every operation on one, expanded
	// into the halves' operations, which the decision then proves.
	if changed, err := SelectPairs(tg, f, sig); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	} else if changed {
		leg = Decide(tg, f, checked)
	}
	// THE MAP, REALIZED AFTER THE DECISION (ADR 0120): on a target with no host
	// map, each map operation becomes its term over the table of its cells, the
	// decision having seen the map; before storage, so a build-map's table is
	// allocated and released as every table is.
	if n, err := RealizeMaps(tg, f); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	} else if n > 0 {
		leg = Decide(tg, f, checked)
	}
	// THE TARGET'S INTERPRETATION OF THE LANGUAGE (ADR 0089), on the decided
	// function: each operation by the narrowest stated realization its facts
	// prove, a term's instance inlined; the decision taken again decides the
	// instances' word arithmetic under the site's facts.
	// A TABLE'S STORAGE AS AN OPERATION (ADR 0092), before ι substitutes the
	// allocator's term into it; and again for the tables a substituted term
	// makes (a crash's newline), until none is left: the allocator's own term
	// makes no table, so this ends.
	for round := 0; ; round++ {
		tables := Storage(tg, f)
		// A TOKEN VARIANT'S RESULT IS STORED IN ITS TOKEN, by fit (ADR 0125),
		// placed once the storage is and before ι substitutes the allocator
		if round == 0 && len(f.ResultTok) == 2 {
			if err := TakeResultToken(tg, f); err != nil {
				return nil, fmt.Errorf("%s: %v", name, err)
			}
			if err := CheckReuse(tg, f); err != nil {
				return nil, fmt.Errorf("%s: %v", name, err)
			}
		}
		// A TABLE'S RELEASE AT ITS DEATH (ADR 0096), placed once the program's
		// tables have their storage and before ι substitutes the terms, and
		// checked there by W11, independently of the pass.
		if round == 0 && Release(tg, f) > 0 {
			if err := CheckReleases(tg, f); err != nil {
				return nil, fmt.Errorf("%s: %v", name, err)
			}
			// AND A DEAD TABLE'S STORAGE TAKEN BY THE NEXT TABLE OF ITS SIZE
			// (R1b), checked by W12.
			if Reuse(tg, f) > 0 {
				if err := CheckReuse(tg, f); err != nil {
					return nil, fmt.Errorf("%s: %v", name, err)
				}
			}
		}
		n, err := RealizeOps(tg, f)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		if n > 0 {
			leg = Decide(tg, f, checked)
			// ι's IMAGE IS AN IR LIKE ANY OTHER: a substituted term's host call
			// may take the unsigned word (a 64-bit unsigned C type, ADR 0126:
			// region's HeapAlloc and VirtualAlloc, whose SIZE_T is 0..2^64-1 on
			// LLP64), so the realization of each value is chosen on it too.
			if !PlantNoWordsOnImage {
				if changed, err := SelectWordsOnImage(tg, f); err != nil {
					return nil, fmt.Errorf("%s: %v", name, err)
				} else if changed {
					leg = Decide(tg, f, checked)
				}
			}
		}
		if tables == 0 && round > 0 || n == 0 {
			break
		}
		if round == 8 {
			return nil, fmt.Errorf("%s: a table's storage and ι reach no fixed point in 8 rounds (ADR 0092)", name)
		}
	}
	// THE OBLIGATION AT EACH CALL OF A PROGRAM FUNCTION (ADR 0101): its
	// arguments in its parameters' sets, on the function's final facts.
	if err := CheckCallSets(tg, f); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return leg, nil
}
