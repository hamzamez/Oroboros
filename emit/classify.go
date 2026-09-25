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
