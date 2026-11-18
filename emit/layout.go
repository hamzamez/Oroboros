package emit

import (
	"fmt"

	"oroboros/core"
)

// A C TYPE'S LAYOUT (llvm.md §6b, ADR 0128 items 8–9; records-research §3.2).
//
// A C struct is two things, and a header module states them apart:
//
//	set RECT = record { left ∈ winnt.LONG, top ∈ winnt.LONG, … }    the set, the language's record
//	repr RECT c(left, top, right, bottom)                          its representation at a C boundary
//
// The record set is a function on its labels and has no order (data.md §4.4);
// the order and the packing are the representation's, chosen by place and
// stated by the declaration, never chosen (ADR 0083). The name RECT is the C
// type's, its typedef (ADR 0126), so it denotes the set and selects the
// layout: a parameter `cell(windef.RECT)` holds a record of RECT's set laid
// out as RECT's C definition lays it.
//
// The layout is derived, the fold L_{M,p} over the fields in the C order, M
// the triple's data model:
//
//	off₁ = 0,  offᵢ₊₁ = ⌈offᵢ + kᵢ⌉ at min(aᵢ₊₁, p),  A = max min(aᵢ, p),  N = ⌈end⌉ at A
//
// THEOREM L: in the declared order this is the least layout, every field at an
// offset its alignment divides and no two overlapping; and N's rounding is
// forced by a table of the struct, whose element j puts field l at j·N + off_l,
// aligned for every j iff A | N. Checked against clang's on both triples,
// never trusted (emit/layout_test.go).

// CLayout is a C type's layout as its header states it: the record's labels
// in the C definition's order, and the packing, 0 for none.
type CLayout struct {
	Fields []string
	Pack   int
}

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

// CStruct resolves a set's name to a C type with a layout: its qualified name
// and its record set, unfolded. ok is false for any other set.
func (tg *Target) CStruct(name string) (string, string, bool) {
	k := name
	if _, ok := tg.Layouts[k]; !ok {
		kk, _, ok := tg.alias("", name)
		if !ok {
			return "", "", false
		}
		if _, ok := tg.Layouts[kk]; !ok {
			return "", "", false
		}
		k = kk
	}
	rec := tg.UnfoldType(moduleOf(k), tg.Aliases[k])
	if !core.IsRecord(rec) {
		return "", "", false
	}
	return k, rec, true
}

// Layout is the C type's layout on this target, or why it has none.
func (tg *Target) Layout(name string) (StructLayout, error) {
	k, _, ok := tg.CStruct(name)
	if !ok {
		return StructLayout{}, fmt.Errorf("%s has no C layout (llvm.md §6b): its header states none", core.ShowSet(name))
	}
	f, err := tg.fold(k)
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

// PlantNaturalLayout ignores a layout's packing: richedit's pack(4) structs on
// x86_64 are laid out naturally, FORMATRANGE 8-aligned where it is 4.
var PlantNaturalLayout bool

// fold is the layout of the C type k, a qualified name with a layout. A
// field's type is read off the set's definition as written, so a field whose
// type is another C type with a layout (FORMATRANGE's rc, a RECT) is laid out
// as that type, and any other field is a C scalar.
func (tg *Target) fold(k string) (folded, error) {
	lay := tg.Layouts[k]
	mod := moduleOf(k)
	raw := tg.Aliases[k]
	labels, types := core.RecordFields(raw)
	if labels == nil {
		return folded{}, fmt.Errorf("%s's layout names a set that is no record: %s", k, core.ShowSet(raw))
	}
	typeOf := map[string]string{}
	for i, l := range labels {
		typeOf[l] = types[i]
	}
	if len(lay.Fields) != len(labels) {
		return folded{}, fmt.Errorf("%s's layout lists %d fields and its record has %d labels: a layout "+
			"orders the record's labels, each once (llvm.md §6b)", k, len(lay.Fields), len(labels))
	}
	pack := lay.Pack
	if PlantNaturalLayout {
		pack = 0
	}
	var f folded
	off := int64(0)
	f.align = 1
	for _, l := range lay.Fields {
		ct, ok := typeOf[l]
		if !ok {
			return folded{}, fmt.Errorf("%s's layout names %s, which its record does not have", k, core.Spell(l))
		}
		var fl foldedField
		if n, _, ok := tg.CStruct(tg.qualifyName(mod, ct)); ok {
			sub, err := tg.fold(n)
			if err != nil {
				return folded{}, err
			}
			fl.sub, fl.size, fl.align = &sub, sub.size, sub.align
		} else {
			u := tg.UnfoldType(mod, ct)
			cll, cross, signed, ok := tg.CType(u)
			if !ok || cll == "void" || cross == "cstr" || cross == "elems" {
				return folded{}, fmt.Errorf("%s's field %s ∈ %s is no C scalar type this layout holds "+
					"(llvm.md §6b)", k, core.Spell(l), core.ShowSet(u))
			}
			fl.ll, fl.trunc, fl.signed = cll, cross == "trunc", signed
			sz, err := tg.cSize(cll)
			if err != nil {
				return folded{}, err
			}
			fl.size, fl.align = sz, sz
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
	// the record view's order, the labels sorted by spelling, as RecordFields
	for _, l := range labels {
		for i, m := range lay.Fields {
			if m == l {
				f.canonical = append(f.canonical, i)
			}
		}
	}
	return f, nil
}

// qualifyName resolves a set name written in module mod to its qualified key,
// or returns it as written.
func (tg *Target) qualifyName(mod, n string) string {
	if k, _, ok := tg.alias(mod, n); ok {
		return k
	}
	return n
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

// FieldLayout is a C type's fields in their C order with each one's offset,
// and its size and alignment: what clang's layout is compared with.
func (tg *Target) FieldLayout(name string) ([]string, []int64, int64, int64, error) {
	k, _, ok := tg.CStruct(name)
	if !ok {
		return nil, nil, 0, 0, fmt.Errorf("%s has no C layout", name)
	}
	f, err := tg.fold(k)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	offs := make([]int64, len(f.fields))
	for i, fl := range f.fields {
		offs[i] = fl.off
	}
	return tg.Layouts[k].Fields, offs, f.size, f.align, nil
}
