package emit

import (
	"strings"

	"oroboros/core"
)

// AN OBSERVER is a host call that may READ a live buffer at a table parameter,
// for the length of the call, without consuming it (ADR 0069).
//
// THE ALGEBRA. ADR 0020 gave the program's own buffers Wadler's `let!` (1990)
// and Odersky's observers (1992): `(b i)` reads a buffer without consuming it,
// and that needs no calculus here because an observation produces a scalar or a
// frozen array, so it cannot ALIAS the buffer. A host call is the same
// observation when two things hold of its declaration:
//
//   - it is PURE: it writes nothing (effects.md §1), so it cannot write the
//     buffer, which host-buffers.md §4 types as aspect 3, "a read of B that does
//     not consume it";
//   - every result is a SCALAR (an integer, a boolean, a float, the unit): a
//     value with no extent, so it cannot share the buffer's (borrow-research.md
//     §3). A declaration with a table or a host-object result may alias its
//     argument, host-buffers.md's aspect 2 (`bytes.TrimSpace`), and an aspect-2
//     parameter is "never B".
//
// Then the call's value is the same whether the argument is the live buffer or
// a frozen copy of it taken at the call (Theorem L, borrow-research.md, with the
// epoch the call itself: the call is pure, so nothing writes the buffer while it
// runs, and its result holds no part of it after it returns). So the observer is
// DERIVED from the declaration, never declared: nothing new is claimed about the
// host. A write-borrow (`linear`), a consume-and-return (`strconv.AppendInt`,
// G7) and an impure call keep their rules exactly: none of them is pure with
// scalar results.
func (tg *Target) Observer(p Prim) bool {
	if !p.Pure || p.Kind != "expr" || p.Form == "" {
		return false
	}
	table := false
	for _, a := range p.Args {
		table = table || strings.HasPrefix(a, "array ")
	}
	if !table {
		return false
	}
	results := p.Results
	if len(results) == 0 {
		results = []string{p.Result}
	}
	for _, r := range results {
		if !tg.scalar(r) {
			return false
		}
	}
	return true
}

// scalar reports a type whose values have no extent: the integer sort in any
// realization, the booleans, the floats and the unit.
func (tg *Target) scalar(ty string) bool {
	switch tg.ValueType(ty) {
	case "int", core.U64Type, core.BigType, "bool", "f64", core.UnitType:
		return true
	}
	return false
}

// observes reports whether argument i of p is a table parameter p observes:
// where a live buffer may stand for a frozen array.
func (tg *Target) observes(p Prim, i int) bool {
	return i < len(p.Args) && strings.HasPrefix(p.Args[i], "array ") && tg.Observer(p)
}
