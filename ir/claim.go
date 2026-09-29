package ir

import (
	"oroboros/core"
	"oroboros/emit"
)

// THE CLAIM EDGE (types.md §7, tableelim-2026-09-29). A signature claims
// f : A → B, and the body's yield τ must satisfy τ ≤ B. W5 checks each yield
// against f.Results, but lowering sets a result to its declaration only when
// the declaration is a table, because an integer's representation is chosen
// later (words, big); for every other declaration the edge compared the yield
// with its own type and could not fail. So `(len a)` under a result declared
// `string` printed `func F(a []float64) int`, and nothing below the checker
// said so.
//
// The claim is compared here, where the signature is known, on SORTS: the
// checker's relation (Target.Agrees), which holds the integer sort ℤ across
// realizations (ADR 0033), `any`, host identity and declared subsumption,
// lifted through tables and maps element by element, with a host alias read as
// the table it realizes, as the unifier reads it (aliasOf). It is the second
// line: the checker refuses these first, and in source terms.

// claimSorts reports that a yield of type got meets a declared result want.
func claimSorts(tg *emit.Target, got, want string) bool {
	if got == "" || want == "" || got == "any" || want == "any" || tg.Agrees(got, want) {
		return true
	}
	ge, gt := tableOf(tg, got)
	we, wt := tableOf(tg, want)
	if gt || wt {
		return gt && wt && claimSorts(tg, ge, we)
	}
	gk, gv, gm := core.MapTypes(got)
	wk, wv, wm := core.MapTypes(want)
	if gm || wm {
		return gm && wm && claimSorts(tg, gk, wk) && claimSorts(tg, gv, wv)
	}
	return false
}

// tableOf gives a table type's element: a language table's or buffer's, or a
// host alias's by ρ_T's kernel.
func tableOf(tg *emit.Target, ty string) (string, bool) {
	if e := core.ArrayElem(ty); e != "" {
		return e, true
	}
	return newUnifier(tg).aliasOf(ty)
}
