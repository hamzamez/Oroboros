package ir

import (
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// TYPES (spec §4). Inside the compiler a type is its CANONICAL SPELLING, the
// string every pass already speaks: `int`, `int 0 255`, `array f64`,
// `map int array f64`, `go.bytestring`. In the file it is written as a term:
// `int`, `(int 0 255)`, `(array f64)`, `(map int (array f64))`. The two
// spellings are inverse bijections (TypeText ∘ typeOf = id on terms the printer
// makes, and typeOf ∘ TypeText = id on canonical strings), which the round-trip
// test pins.

// TypeText is a canonical type as the file writes it.
func TypeText(ty string) string {
	switch {
	case ty == "":
		return "any"
	case strings.HasPrefix(ty, "array "), strings.HasPrefix(ty, "buffer "):
		i := strings.IndexByte(ty, ' ')
		return "(" + ty[:i] + " " + TypeText(ty[i+1:]) + ")"
	}
	if k, v, ok := core.MapTypes(ty); ok {
		return "(map " + TypeText(k) + " " + TypeText(v) + ")"
	}
	if strings.Contains(ty, " ") {
		// A range, or any other spelling of several words (`int 0 inf`): its
		// words, as a list.
		return "(" + ty + ")"
	}
	return ty
}

// typeOf reads a type term back to its canonical spelling.
func typeOf(t *core.Term) (string, error) {
	switch t.Kind {
	case core.KName:
		return t.Name, nil
	case core.KApp:
	default:
		return "", fmt.Errorf("%s is not a type", t)
	}
	if len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return "", fmt.Errorf("%s is not a type", t)
	}
	head := t.Kids[0].Name
	switch head {
	case "array", "buffer":
		if len(t.Kids) != 2 {
			return "", fmt.Errorf("(%s τ) takes one type", head)
		}
		e, err := typeOf(t.Kids[1])
		if err != nil {
			return "", err
		}
		return head + " " + e, nil
	case "map":
		if len(t.Kids) != 3 {
			return "", fmt.Errorf("(map κ τ) takes two types")
		}
		k, err := typeOf(t.Kids[1])
		if err != nil {
			return "", err
		}
		v, err := typeOf(t.Kids[2])
		if err != nil {
			return "", err
		}
		return "map " + k + " " + v, nil
	}
	words := []string{head}
	for _, k := range t.Kids[1:] {
		switch k.Kind {
		case core.KInt:
			words = append(words, fmt.Sprint(k.Int))
		case core.KName:
			words = append(words, k.Name)
		default:
			v, ok := core.EvalEndpoint(k)
			if !ok {
				return "", fmt.Errorf("%s is not a type's word", k)
			}
			words = append(words, v.String())
		}
	}
	return strings.Join(words, " "), nil
}

// sortOf erases every range in a type to the representation ρ_T picks for it,
// `int`, `u64` or `big` (ADR 0026): the type's SORT, which is what IR_A's flow
// rule compares (spec §3, W5).
func sortOf(tg *emit.Target, ty string) string {
	switch {
	case strings.HasPrefix(ty, "array "), strings.HasPrefix(ty, "buffer "):
		i := strings.IndexByte(ty, ' ')
		return ty[:i+1] + sortOf(tg, ty[i+1:])
	}
	if k, v, ok := core.MapTypes(ty); ok {
		return "map " + sortOf(tg, k) + " " + sortOf(tg, v)
	}
	if lo, _, ok := core.IntRangeBig(ty); ok {
		if tg.ValueType(ty) == core.BigType && tg.BigRepr == "limbs" && lo.Sign() >= 0 {
			// A FINITE, non-negative range above the word is held as a TABLE OF
			// LIMBS where the target chooses limbs (ADR 0029), so its sort is that
			// table's: ρ_T's kernel, as for a host alias of `(array E)`. The atom
			// `big` is different: an unbounded or signed value takes the host's
			// bignum even then, since limbs hold a magnitude.
			return "array int"
		}
		return tg.ValueType(ty)
	}
	return ty
}

// unbuffer reads a buffer as the table it is (ADR 0020); linearity is W7's.
func unbuffer(ty string) string {
	if strings.HasPrefix(ty, "buffer ") {
		return "array " + unbuffer(ty[len("buffer "):])
	}
	if strings.HasPrefix(ty, "array ") {
		return "array " + unbuffer(ty[len("array "):])
	}
	if k, v, ok := core.MapTypes(ty); ok {
		return "map " + k + " " + unbuffer(v)
	}
	return ty
}

// subtype is τ ≤ σ on final types (spec §4.1): ⟦τ⟧ ⊆ ⟦σ⟧. Ranges by
// containment, `int` as the target's word, and tables and maps covariantly,
// since they are immutable values. A table's element REPRESENTATION must also
// agree (spec §4.3, Theorem D), which the element ranges alone do not say.
func subtype(tg *emit.Target, a, b string) bool {
	a, b = unbuffer(a), unbuffer(b)
	if a == b {
		return true
	}
	rng := func(t string) (lo, hi *big.Int, ok bool) {
		if t == "int" {
			return big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi), true
		}
		return core.IntRangeBig(t)
	}
	if la, ha, ok := rng(a); ok {
		if lb, hb, ok := rng(b); ok {
			return la.Cmp(lb) >= 0 && ha.Cmp(hb) <= 0
		}
		return false
	}
	if strings.HasPrefix(a, "array ") && strings.HasPrefix(b, "array ") {
		ea, eb := a[len("array "):], b[len("array "):]
		return subtype(tg, ea, eb) && tg.HostType(a) == tg.HostType(b)
	}
	if ka, va, ok := core.MapTypes(a); ok {
		if kb, vb, ok := core.MapTypes(b); ok {
			return ka == kb && subtype(tg, va, vb) && tg.HostType(a) == tg.HostType(b)
		}
	}
	return false
}

// ElemOf is the element type of a table type: `(array τ)`, `(buffer τ)`, or a
// host alias the target realizes as one (spec §4.1). "" when ty is no table.
func ElemOf(tg *emit.Target, ty string) string {
	if e := core.ArrayElem(ty); e != "" {
		return e
	}
	if e, ok := newUnifier(tg).aliasOf(ty); ok {
		return e
	}
	return ""
}

// involvesBig reports whether a type is above the word: `big`, or a range the
// target realizes as one.
func involvesBig(tg *emit.Target, ty string) bool {
	if _, _, ok := core.IntRangeBig(ty); ok {
		return tg.ValueType(ty) == core.BigType
	}
	return false
}

// Typed reports whether a target's BACKEND spells a static type for every
// value: Go and Java do; JavaScript and x86 do not, whatever types their
// declarations document (JavaScript's `int` is "number"). On an untyped
// backend `any` is a final type.
func Typed(tg *emit.Target) bool {
	b, err := tg.ResolveBackend()
	return err == nil && (b == "go" || b == "java")
}
