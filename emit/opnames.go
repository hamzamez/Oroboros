package emit

import "strings"

// WHICH OPERATION A NAME IS. A target spells the language's integer operations
// its own way (`go.+`, `Math.addExact`, `x64.imul-checked`), and several passes
// ask the same question of a name: the IR's lowering, the refinement layer's
// linear fragment, the postconditions. They ask it here.

// arithOp is the integer operation a primitive named `name` applied to n
// arguments computes: "add", "sub", "mul", "div", "rem", "neg", the mask
// "and" and the shift "shr" (which SelectShifts writes), or "".
//
// A CHECKED FORM IS STILL THE OPERATION: under `-checked` a target's spelling
// is `add-exact` or `Math.addExact`, and it computes the same integer or traps.
// The mask and the shift are matched here and not in `opAlias`, which the
// linear fragment reads: `x & m` is not a linear term.
func arithOp(name string, n int) string {
	if base, ok := uncheckedName(name); ok {
		name = base
	}
	switch {
	case n == 2 && (isOp(name, "add") || name == "+" || strings.HasSuffix(name, ".add")):
		return "add"
	case n == 2 && (isOp(name, "sub") || name == "-" || strings.HasSuffix(name, ".sub")):
		return "sub"
	case n == 2 && (isOp(name, "mul") || name == "*" || strings.HasSuffix(name, ".imul")):
		return "mul"
	case n == 2 && (name == "/" || strings.HasSuffix(name, ".idiv") || isOp(name, "div")):
		return "div"
	case n == 2 && (name == "%" || strings.HasSuffix(name, ".irem") || isOp(name, "rem")):
		return "rem"
	case n == 1 && (isOp(name, "neg") || strings.HasSuffix(name, ".neg")):
		return "neg"
	case n == 2 && (name == "&" || strings.HasSuffix(name, ".&") || isOp(name, "and")):
		return "and"
	case n == 2 && (name == ">>" || strings.HasSuffix(name, ".>>") ||
		isOp(name, "shr") || isOp(name, "sar")):
		return "shr"
	}
	return ""
}

// uncheckedName maps a target's checked spelling back to the operation it
// checks. The spellings are the targets' own: `add-exact` and `mul-exact` on
// Go, `addExact` and `multiplyExact` on the JVM, `add-checked` and
// `imul-checked` on x86, matched by their last segment.
func uncheckedName(name string) (string, bool) {
	seg := name
	if i := strings.LastIndex(seg, "."); i >= 0 {
		seg = seg[i+1:]
	}
	switch seg {
	case "add-exact", "addExact", "add-checked":
		return "add", true
	case "sub-exact", "subtractExact", "sub-checked":
		return "sub", true
	case "mul-exact", "multiplyExact", "imul-checked":
		return "mul", true
	}
	return "", false
}

// wordOps are the unsigned word's operations (ADR 0026 (10), ADR 0033). A
// target that realizes U declares them in its own file, the way it declares
// its bignum's, and the loader finds them by spelling.
var wordOps = []string{
	"u64+", "u64-", "u64*", "u64/", "u64%",
	"u64<", "u64<=", "u64>", "u64>=", "u64=",
	"u64-of", "int-of-u64",
}

func wordOpArity(op string) int {
	if op == "u64-of" || op == "int-of-u64" {
		return 1
	}
	return 2
}

// HasBigDest reports whether this target's bignum can be written into, which
// is the destination rule's premise (ir/bigreuse.go). Go's can; Java's
// BigInteger and JavaScript's BigInt are immutable, a fact about those hosts
// rather than a gap in their target files.
func (tg *Target) HasBigDest() bool {
	_, ok := tg.Prims["big+!"]
	return ok
}
