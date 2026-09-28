package ir

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// THE PIPELINE EVERY DRIVER RUNS, from an entry point's residual (its contracts
// discharged) to its decided IR: `gen`, `build`, `cmd/intervals` and the tests
// take a program through the same steps in the same order, so what one of them
// proves or refuses is what the others do.
//
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
func Entry(tg *emit.Target, name, irName string, sig *core.Sig, nf *core.Term, all []*core.Sig,
	checked bool, note func(Note)) (*Func, *Legality, error) {
	if note == nil {
		note = func(Note) {}
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
	if plan.Host || plan.Limbs {
		nf = emit.EraseWordAscriptions(tg.Word, nf)
	} else {
		nf = emit.EraseAscriptions(nf)
	}
	// Check the residual before emitting it (docs/spec/types.md). On Go and
	// Java the host would catch most of this; on JavaScript nothing would.
	if err := emit.Check(tg, name, nf); err != nil {
		return nil, nil, err
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
	leg := Decide(tg, f, checked)
	// THE RUNG ABOVE THE WORD: the least set of values held exactly, on the
	// decided function's facts; decided again after, where no operation held
	// exactly is counted.
	if plan.Host || plan.Limbs {
		n, err := SelectRung(tg, f, sig, plan)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", name, err)
		}
		if n > 0 {
			note(Note{Kind: NoteBig, N: n})
		}
		leg = Decide(tg, f, checked)
	}
	// THE UNSIGNED WORD: each integer value's realization is ρ of its fact;
	// the decision is taken again on the selected function.
	if changed, err := SelectWords(tg, f); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	} else if changed {
		note(Note{Kind: NoteWord, N: WordOps(f)})
		leg = Decide(tg, f, checked)
	}
	// A POSTCONDITION on an exported definition is an obligation: the caller
	// is outside the program, so the body is the only evidence there is.
	if err := CheckEnsures(tg, f, sig); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	return f, leg, nil
}
