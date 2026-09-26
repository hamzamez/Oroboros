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

// ExportName is the Go backend's spelling of an exported function's name
// (`native-dot` → `NativeDot`), which the gauntlet's callers are written
// against. The IR's Go printer spells names with it so the two agree.
func ExportName(s string) string { return export(s) }

// JSMangle is the JavaScript backend's spelling of a name, which the IR's
// JavaScript printer uses so callers see the same exports.
func JSMangle(s string) string { return jsMangle(s) }

// MultiPrimDests reports whether a several-result template assigns into
// destinations (a host that signals failure out of band) rather than being the
// tuple itself; FillDests writes those destinations in.
func MultiPrimDests(form string, n int) bool       { return multiPrimDests(form, n) }
func FillDests(form string, dests []string) string { return fillDests(form, dests) }

// JavaMangle and JavaRecordName are the Java backend's spellings of a name
// and of a several-results record, which the IR's Java printer uses so callers
// see the same class.
func JavaMangle(s string) string         { return javaMangle(s) }
func JavaRecordName(tys []string) string { return javaRecordName(tys) }

// Boxed spells a type as an object where the target says one is needed (a
// JVM generic cannot take a primitive), and as itself elsewhere.
func (tg *Target) BoxedType(name string) string { return tg.boxed(name) }

// The x86 backend's shared pieces, which the IR's x86 printer uses so the two
// agree on templates, literals, labels and the frame (docs/spec/ir.md §9.6).

// FillAsm expands one emission template with its destination and operands
// spelled as text (a register, an immediate or a memory operand).
func FillAsm(form, dst string, ops []string, u int) (string, error) {
	ps := make([]place, len(ops))
	for i, o := range ops {
		ps[i] = place{text: o}
	}
	return fillAsm(form, place{text: dst}, ps, u)
}

// AsmUniq is the next number for a template's `%u` and a printer's labels,
// shared with the term backend so two procedures in one file never collide.
func AsmUniq() int { asmUniq++; return asmUniq }

func AsmPeephole(src string) string         { return asmPeephole(src) }
func AsmFloatLit(v float64) string          { return asmFloatLit(v) }
func AsmStringLit(s string) string          { return asmStringLit(s) }
func AsmNegate(cc string) string            { return asmNegate[cc] }
func AsmByte(r string) string               { return asmByte(r) }
func AsmElemAddr(b, i string, w int) string { return asmElemAddr(b, i, w) }

// AsmShadowFloor is the outgoing-argument area every procedure reserves at
// least (asmShadow), and AsmTableHeader the bytes before a table's elements.
const (
	AsmShadowFloor = asmShadow
	AsmTableHeader = asmTableHeader
)

// FindAlloc is the target's allocator, which `build` asks for bytes.
func (tg *Target) FindAlloc() (Prim, bool) { return tg.findAlloc() }

// ReprBytes is how many bytes the declared representation holding [lo, hi]
// occupies, or 0 if the target declares none.
func (tg *Target) ReprBytes(lo, hi int64) int { return tg.reprBytes(lo, hi) }
