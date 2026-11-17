package emit

import (
	"fmt"

	"oroboros/core"
)

// A HOST STRUCT'S LAYOUT (llvm.md §6b, ADR 0128 item 9; records-research §3.2).
//
// The layout is derived from the struct's C definition, never chosen: the fold
// L_{M,p,d} over its fields in their declared order, with M the triple's data
// model (each C type's size and alignment), p the header's packing:
//
//	off₁ = 0,  offᵢ₊₁ = ⌈offᵢ + kᵢ⌉ at min(aᵢ₊₁, p),  A = max min(aᵢ, p),  N = ⌈end⌉ at A
//
// THEOREM L: in the declared order this is the least layout, every field at an
// offset its alignment divides and no two overlapping; and N's rounding is
// forced by a table of the struct, whose element j puts field l at j·N + off_l,
// aligned for every j iff A | N. A C type's size and alignment are equal here
// (every integer type and a pointer on both Windows triples; a nested struct
// carries its own A), and the packing is the meet in the divisibility lattice
// of powers of two. Checked against clang's layout on both triples, never
// trusted (gauntlet/stdlib/win32hand_test.go).

// LeafLayout is one leaf of a struct, in the record view's canonical order: its
// absolute offset and its C type.
type LeafLayout struct {
	Offset int64
	LL     string // the LLVM type
	Trunc  bool   // narrower than the word: trunc in, sext or zext out
	Signed bool
}

// StructLayout is a struct's size N, alignment A, and leaves.
type StructLayout struct {
	Size, Align int64
	Leaves      []LeafLayout
}

// Layout is the struct type's layout on this target, or why it has none.
func (tg *Target) Layout(ty string) (StructLayout, error) {
	if !core.IsStruct(ty) {
		return StructLayout{}, fmt.Errorf("%s is no host struct", core.ShowSet(ty))
	}
	f, err := tg.fold(ty)
	if err != nil {
		return StructLayout{}, err
	}
	var leaves []LeafLayout
	var walk func(f folded, base int64)
	walk = func(f folded, base int64) {
		for _, i := range f.canonical {
			fl := f.fields[i]
			if fl.sub != nil {
				walk(*fl.sub, base+fl.off)
				continue
			}
			leaves = append(leaves, LeafLayout{Offset: base + fl.off, LL: fl.ll, Trunc: fl.trunc, Signed: fl.signed})
		}
	}
	walk(f, 0)
	return StructLayout{Size: f.size, Align: f.align, Leaves: leaves}, nil
}

type foldedField struct {
	off, size, align int64
	ll               string
	trunc, signed    bool
	sub              *folded
}

type folded struct {
	fields      []foldedField // in C order
	canonical   []int         // the C indices in the record view's order
	size, align int64
}

// PlantNaturalLayout ignores a struct's packing: commdlg's structs on i686 are
// laid out naturally, PRINTDLGA 68 bytes where it is 66 (records-research §2.1).
var PlantNaturalLayout bool

func (tg *Target) fold(ty string) (folded, error) {
	labels, types, pack := core.StructFields(ty)
	if PlantNaturalLayout {
		pack = 0
	}
	var f folded
	off := int64(0)
	f.align = 1
	for i, ct := range types {
		var fl foldedField
		ct = tg.UnfoldType("", ct)
		switch {
		case core.IsStruct(ct):
			sub, err := tg.fold(ct)
			if err != nil {
				return folded{}, err
			}
			fl.sub, fl.size, fl.align = &sub, sub.size, sub.align
		default:
			ll, cross, signed, ok := tg.CType(ct)
			if !ok || ll == "void" || cross == "cstr" || cross == "elems" {
				return folded{}, fmt.Errorf("the struct's field %s ∈ %s is no C scalar type this layout holds "+
					"(llvm.md §6b)", core.Spell(labels[i]), core.ShowSet(ct))
			}
			fl.ll, fl.trunc, fl.signed = ll, cross == "trunc", signed
			k, err := tg.cSize(ll)
			if err != nil {
				return folded{}, err
			}
			fl.size, fl.align = k, k
		}
		a := fl.align
		if pack > 0 && int64(pack) < a {
			a = int64(pack)
		}
		off = (off + a - 1) / a * a
		fl.off = off
		off += fl.size
		if a > f.align {
			f.align = a
		}
		f.fields = append(f.fields, fl)
	}
	f.size = (off + f.align - 1) / f.align * f.align
	// the record view's order: the labels sorted by spelling, as RecordFields
	view, _ := core.RecordFields(ty)
	for _, l := range view {
		for i, m := range labels {
			if m == l {
				f.canonical = append(f.canonical, i)
			}
		}
	}
	return f, nil
}

// cSize is an LLVM scalar's size in bytes on this triple: iw is w/8, and a
// pointer the triple's ISA word, 8 bytes on x86_64 and 4 on i686, which is the
// data model's pointer width (LLP64 and ILP32, ADR 0122 item 2, 0124).
func (tg *Target) cSize(ll string) (int64, error) {
	switch ll {
	case "i8":
		return 1, nil
	case "i16":
		return 2, nil
	case "i32":
		return 4, nil
	case "i64":
		return 8, nil
	case "ptr":
		bits, err := tg.DigitBits()
		if err != nil {
			return 0, fmt.Errorf("a struct holding a pointer has no layout: %v", err)
		}
		return int64(bits / 8), nil
	}
	return 0, fmt.Errorf("an LLVM %s has no size this layout states", ll)
}

// FieldLayout is a struct's top-level fields in their C order with each one's
// offset, and its size and alignment: what clang's layout is compared with.
func (tg *Target) FieldLayout(ty string) ([]string, []int64, int64, int64, error) {
	f, err := tg.fold(ty)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	labels, _, _ := core.StructFields(ty)
	offs := make([]int64, len(f.fields))
	for i, fl := range f.fields {
		offs[i] = fl.off
	}
	return labels, offs, f.size, f.align, nil
}
