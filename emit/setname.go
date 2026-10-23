package emit

import (
	"fmt"
	"strings"

	"oroboros/core"
)

// A SET'S NAME DENOTES A SET (spec/typevalues.md §1a, ADR 0073). A header's set
// is a term of the set algebra, and its denotation ⟦·⟧ is defined by structural
// recursion: a homomorphism from the free algebra of set expressions into sets,
// total exactly when every atom is bound in the environment of named sets,
//
//	Γ_T = the language's sets ∪ dom(T's types),
//
// the language's sets being the open elements `?` and `?int`, the ranges, and
// the formers (`array`, `buffer`, `cell`, `map`, products), and the target's
// types the ones its files declare, the language's base sets among them (`int`,
// `bool`, `f64`, `string`, `any`). So ⊢ S set is a scope check, the twin of a
// term's "is not bound": a name nothing binds has no denotation, and a claim
// about a set that is not one says nothing.
//
// A program's variants are not in Γ_T, because none reaches a header by the
// time it is checked: a variant crossing a boundary is its tag and its slots
// (ADR 0042, core.Load), and a variant parameter is refused at load (sums.md §6).
// A variant that did reach one would be refused here, loudly.
//
// Until then an unbound name was an opaque type: `fn f(x ∈ Array(Nonesuch))`
// printed Go's `[]/*Nonesuch?*/`, which does not build, and `fn g(x ∈ Nonesuch)`
// was refused at its first use as though Nonesuch were a set.

// unboundSet returns the first atom of the canonical set ty that Γ_T does not
// bind, or "" when ⊢ ty set.
func (tg *Target) unboundSet(ty string) string {
	switch {
	case ty == "" || ty == "?" || ty == "?int":
		return "" // no claim, or an element the first store solves (types.md §3.1)
	}
	if _, ok := tg.Types[ty]; ok {
		return ""
	}
	if _, _, ok := core.IntRangeBig(ty); ok || core.UnboundedRange(ty) {
		return ""
	}
	for _, elem := range []string{core.ArrayElem(ty), core.BufferElem(ty), core.CellElem(ty)} {
		if elem != "" {
			return tg.unboundSet(elem)
		}
	}
	if k, v, ok := core.MapTypes(ty); ok {
		if u := tg.unboundSet(k); u != "" {
			return u
		}
		return tg.unboundSet(v)
	}
	if core.IsProd(ty) {
		for _, f := range core.ProdTypes(ty) {
			if u := tg.unboundSet(f); u != "" {
				return u
			}
		}
		return ""
	}
	if head, _, ok := core.Applied(ty); ok {
		return head // a type constructor applied: no program declares one (above)
	}
	// A former the checks above do not take apart (`fn`, `record`: not built)
	// is not this judgement's to refuse.
	if f := strings.Fields(ty); len(f) > 1 && core.IsTypeFormer(f[0]) {
		return ""
	}
	return ty
}

// headerSets refuses a header that names a set Γ_T does not bind, saying which
// position names it.
func (tg *Target) headerSets(name string, sig *core.Sig) error {
	refuse := func(where, ty, atom string) error {
		return fmt.Errorf("%s: its header's %s names %s, which is no set: not one of the language's "+
			"and not declared by target %s (spec/typevalues.md §1a, "+
			"ADR 0073). Declare it, import the module that does, or correct the name",
			core.ShowName(name), where, core.ShowName(atom), tg.Name)
	}
	for i, p := range sig.Params {
		if u := tg.unboundSet(p.Type); u != "" {
			what := "parameter " + core.ShowName(p.Name)
			if p.Name == "" {
				what = fmt.Sprintf("parameter %d", i+1)
			}
			return refuse(what, p.Type, u)
		}
	}
	if u := tg.unboundSet(sig.Result); u != "" {
		return refuse("result", sig.Result, u)
	}
	for j, r := range sig.Results {
		if u := tg.unboundSet(r); u != "" {
			return refuse(fmt.Sprintf("result's component %d", j+1), r, u)
		}
	}
	return nil
}
