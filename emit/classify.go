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
