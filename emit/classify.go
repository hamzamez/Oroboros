package emit

// ArithOp and CmpOp expose this package's operator classification: every
// spelling four targets use for add, sub, mul, neg, div, rem and the six
// comparisons. They exist for experiments/irproto (docs/ir-research.md, P3), so
// a prototype analysis counts and narrows exactly as the interval pass does and
// its numbers compare. A copy of the table could disagree; a wrapper cannot.

// ArithOp is the arithmetic `name` performs with n operands, or "".
func ArithOp(name string, n int) string { return arithOp(name, n) }

// CmpOp is the comparison `name` is: "eq", "ne", "lt", "le", "gt", "ge", or "".
func CmpOp(name string) string {
	for _, k := range []string{"eq", "ne", "lt", "le", "gt", "ge"} {
		if isOp(name, k) {
			return k
		}
	}
	return ""
}

// IsLengthName reports whether a primitive named `name` is a length: the
// language's `len`, a primitive of kind "len", or a target's spelling of one
// (`go.len`), exactly as isLength decides it.
func (tg *Target) IsLengthName(name string) bool {
	if name == "len" {
		return true
	}
	if p, ok := tg.Prims[name]; ok && p.Kind == "len" {
		return true
	}
	return isOp(name, "alen") || isOp(name, "slen")
}

// HostType is the host's spelling of a language type on this target: `int64`,
// `[]int16`, `map[int64]string`. It is the one table every printer reads, and it
// is exported for the same prototype.
func (tg *Target) HostType(name string) string { return tg.ty(name) }

// IsCheckedName reports whether a primitive is a checked arithmetic form
// (`add-exact`, `Math.addExact`): the operation, in mode `trap`
// (docs/spec/ir.md §4.4). It is interval.go's uncheckedName, so the IR and the
// analysis agree on what -checked produced.
func IsCheckedName(name string) bool {
	_, ok := uncheckedName(name)
	return ok
}

// Agrees reports whether a value of type got may flow where want is required:
// the type checker's own relation (docs/spec/types.md), which the IR's verifier
// applies to every flow edge (docs/spec/ir.md W5) rather than defining its own.
func (tg *Target) Agrees(got, want string) bool {
	c := &checker{tgt: tg, types: map[string]string{}}
	return c.agree("", got, want) == nil
}
