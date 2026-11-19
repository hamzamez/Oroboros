package ir

import (
	"fmt"
	"sort"

	"oroboros/core"
	"oroboros/emit"
)

// A CLAIMED CONSTANT IS DECIDED AT ITS DEFINITION (ADR 0072, the book's
// finding 45). `half ∈ 0..cap = cap * 2` is a definition with no parameters
// whose result is claimed in 0..cap, so its contract is ⟦e⟧ ∈ S, and it holds
// of the definition, read or not. A read constant carried the claim as a mark
// on its value, (#req "#claim" "half" S e), decided where it was inlined, so a
// constant nothing read was decided by nothing and a false claim compiled.
//
// Each top-level constant holding a claim is normalized alone, a residual of
// no parameters, and its marks decided by the one procedure every residual's
// are (DecideMarks): a literal by evaluation, a range on the IR's intervals,
// the rest in the refinement layer. A claim whose set is no range (a typing
// claim, `s ∈ String = 3`) is its ascription (the S e), decided by the type
// checker on the constant's residual (ADR 0137), read or not.
func DecideConstantClaims(tg *emit.Target, prog *core.Program, env *core.Env, all []*core.Sig) error {
	if PlantConstantClaimUndecided {
		return nil
	}
	names := make([]string, 0, len(prog.Defs))
	for n, body := range prog.Defs {
		if _, signed := prog.Sigs[n]; signed || body == nil || body.Kind == core.KFn || !core.HoldsClaim(body) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names) // the first refusal is the first name
	for _, n := range names {
		nf, err := core.Normalize(prog.Defs[n], env, core.DefaultFuel)
		if err != nil {
			continue // reduction reports this better where the constant is read
		}
		if _, err := DecideMarks(tg, n, nil, core.Fn(nil, nf), all); err != nil {
			return err // DecideMarks names the constant
		}
		if !core.PlantTypingClaimDropped {
			if err := emit.Check(tg, core.ShowName(n), emit.TypingMarksAsLets(nf)); err != nil {
				return fmt.Errorf("%w (a constant's claim, decided where it is written: ADR 0137)", err)
			}
		}
	}
	return nil
}

// PlantConstantClaimUndecided restores finding 45: a claimed constant is
// decided only where it is read (TestAClaimedConstantIsDecidedWhereWritten).
var PlantConstantClaimUndecided bool
