package core

import (
	"fmt"
	"strings"
)

// A HOST WRITING A CELL (spec/local.md §5, §6).
//
// An out-parameter is a result: f : X × Ref T → R that writes its cell and keeps
// no address is, observably, X × T → R × T, because a cell the compiler owns is
// unaliased. Inside the call the cell is a REFERENCE, the address of a fresh
// copy of its value, and after it the cell holds what the copy holds:
//
//	S⟦(f a… c …)⟧ = let r = (#ref c) in let y = (f a… r …) in let c = (#deref r) in (y, state)
//
// `#ref` and `#deref` are impure, so the effect discipline (ADR 0010) keeps them
// on either side of the call. The reference is an ordinary argument, so it
// passes unchanged through a fallible declaration's retraction, a variadic
// call's list and Go's box; after the call the cell is a loop variable again.
//
// `(out τ)` at a cell parameter is a cell for that one call, holding τ's zero,
// `(#fresh "τ")`, and the call's value is the cells' values, then the declared
// result's components: the order a buffer's write-borrow returns.

// RefName, FreshName and DerefName are the reference's introductions and its
// elimination: (#ref v) a reference to a fresh copy of v, (#fresh "τ") one to
// τ's zero, and (#deref r) the value it holds. The language's, injected into
// every target, impure, and out of a program's reach.
const (
	RefName   = "#ref"
	FreshName = "#fresh"
	DerefName = "#deref"
	// OutName is the sugar, recognized only at a cell parameter.
	OutName = "out"
)

// CellDecl is what a declaration says about its cell parameters: their
// indices, the index its variadic list of cells starts at (-1 when none), and
// its declared result's arity, which (out τ) spreads after the cells.
type CellDecl struct {
	Params []int
	List   int
	Arity  int
	// CTypes are the cell parameters' C type names where the type has a C
	// layout (llvm.md §6b), and Records their record sets: such a cell holds
	// its record's leaves. Both "" for any other cell.
	CTypes, Records []string
	// Sets are the scalar cell parameters' element sets, a range such as a
	// DWORD's 0..2^32-1, "" for any other cell: a program's cell passed there
	// owes its value's membership (translateCall).
	Sets []string
	// Outs are a C declaration's out-results (ADR 0131): cell parameters after
	// its NArgs − len(Outs) declared ones, each of the set in OutSets, which a
	// caller omits; the translation fills each with `out T`.
	Outs    []int
	OutSets []string
	NArgs   int
}

// CellDeclTerm is the form a target hands the loader, (cells ARITY LIST i…).
func CellDeclTerm(d CellDecl) *Term {
	kids := []*Term{Name("cells"), Int(int64(d.Arity)), Int(int64(d.List))}
	for _, i := range d.Params {
		kids = append(kids, Int(int64(i)))
	}
	for i := range d.CTypes {
		kids = append(kids, Str(d.CTypes[i]), Str(d.Records[i]), Str(d.Sets[i]))
	}
	if len(d.Outs) > 0 {
		outs := []*Term{Name("outs"), Int(int64(d.NArgs))}
		for j, i := range d.Outs {
			outs = append(outs, Int(int64(i)), Str(d.OutSets[j]))
		}
		kids = append(kids, &Term{Kind: KApp, Kids: outs})
	}
	return &Term{Kind: KApp, Kids: kids}
}

func readCellDecl(t *Term) (CellDecl, error) {
	if t == nil || t.Kind != KApp || len(t.Kids) < 3 {
		return CellDecl{}, fmt.Errorf("a cells form is (cells ARITY LIST i…), got %s", t)
	}
	var ints []int
	var strs []string
	var outs *Term
	for _, k := range t.Kids[1:] {
		switch {
		case k.Kind == KApp && len(k.Kids) >= 2 && k.Kids[0].Kind == KName && k.Kids[0].Name == "outs":
			outs = k
		case k.Kind == KStr:
			strs = append(strs, k.Str)
		case k.Kind == KInt && strs == nil:
			ints = append(ints, int(k.Int))
		default:
			return CellDecl{}, fmt.Errorf("a cells form is (cells ARITY LIST i… \"C\" \"R\" \"S\"…), got %s", t)
		}
	}
	d := CellDecl{Arity: ints[0], List: ints[1], Params: ints[2:]}
	for i := 0; i+2 < len(strs); i += 3 {
		d.CTypes, d.Records, d.Sets = append(d.CTypes, strs[i]), append(d.Records, strs[i+1]), append(d.Sets, strs[i+2])
	}
	if outs != nil {
		d.NArgs = int(outs.Kids[1].Int)
		for j := 2; j+1 < len(outs.Kids); j += 2 {
			d.Outs, d.OutSets = append(d.Outs, int(outs.Kids[j].Int)), append(d.OutSets, outs.Kids[j+1].Str)
		}
	}
	return d, nil
}

// at reports whether argument i of a call is a cell parameter.
func (d CellDecl) at(i int) bool {
	if d.List >= 0 && i >= d.List {
		return true
	}
	for _, p := range d.Params {
		if p == i {
			return true
		}
	}
	return false
}

// structAt is the C type with a layout the cell parameter at argument i
// holds, and its record set, or "".
func (d CellDecl) structAt(i int) (string, string) {
	for j, p := range d.Params {
		if p == i && j < len(d.CTypes) && d.CTypes[j] != "" {
			return d.CTypes[j], d.Records[j]
		}
	}
	return "", ""
}

// setAt is cell argument i's element set where it is a scalar's range, or "".
func (d CellDecl) setAt(i int) string {
	for j, p := range d.Params {
		if p == i && j < len(d.Sets) {
			return d.Sets[j]
		}
	}
	return ""
}

// cellCall reports a call of a declaration with cell parameters.
func (c *cellTr) cellCall(t *Term) (CellDecl, bool) {
	if t == nil || t.Kind != KApp || len(t.Kids) == 0 || t.Kids[0].Kind != KName {
		return CellDecl{}, false
	}
	d, ok := c.decls[t.Kids[0].Name]
	return d, ok
}

// asWritten reports a call some cell parameter of which holds what the
// program wrote, not a reference the translation bound: a compiler name,
// which a program cannot write, is one.
func (c *cellTr) asWritten(t *Term, d CellDecl) bool {
	// a C call whose out-results the caller omits (ADR 0131): the translation
	// fills them, exactly when fillOuts would, so a call it leaves as it is
	// is not translated again and again
	if len(d.fillOuts(t).Kids) != len(t.Kids) {
		return true
	}
	for i, a := range t.Kids[1:] {
		if d.at(i) && !compilerMade(a) {
			return true
		}
	}
	return false
}

// compilerMade reports a name or a form the translation made, which starts with
// `#`: a reference bound to a name, or `(#ref c)` and `(#fresh "τ")` left where
// they stand.
func compilerMade(a *Term) bool {
	if a.Kind == KName {
		return strings.HasPrefix(a.Name, "#")
	}
	return a.Kind == KApp && len(a.Kids) > 0 && a.Kids[0].Kind == KName && strings.HasPrefix(a.Kids[0].Name, "#")
}

func isOut(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == OutName
}

// translateCall is S⟦·⟧ of a call of a declaration with cell parameters, with
// k its continuation when it is eliminated, nil when it stands as a value.
func (c *cellTr) translateCall(call *Term, d CellDecl, k *Term, inLoop bool, carried []string) (*Term, error) {
	// A C CALL'S OUT-RESULTS ARE THE CALL'S OWN (ADR 0131): a caller writes the
	// declared parameters, and each out-result is `out T` at its position, so
	// the call's value is the cells' values, then the C function's result.
	call = d.fillOuts(call)
	head := call.Kids[0]
	kids := []*Term{head}
	var outAt, cellAt []int // argument positions, in order
	var cellNames []string
	for i, a := range call.Kids[1:] {
		if !d.at(i) {
			kids = append(kids, a)
			continue
		}
		st, rec := d.structAt(i)
		switch {
		case a.Kind == KName && c.isCell(a.Name) && st != "":
			// A STRUCT'S CELL IS A REFERENCE TO ITS LEAVES (llvm.md §6b): the
			// region the printer lays out, written with the record's leaves in
			// canonical order, and read back leaf by leaf.
			ref := []*Term{Name(RefName), Str(st)}
			for _, lf := range recordLeaves("", rec, nil) {
				ref = append(ref, lf.read(a))
			}
			kids = append(kids, &Term{Kind: KApp, Kids: ref})
			cellAt, cellNames = append(cellAt, i), append(cellNames, a.Name)
		case a.Kind == KName && c.isCell(a.Name):
			// A SCALAR CELL AT A HOST'S CELL OF SET T (ADR 0048, 0126): the host
			// reads the value and writes one back, so the cell fits iff its value
			// is in T at the call, a claim decided as any is, and T is within the
			// cell's own set, which a cell's ℤ always holds. The reference is to a
			// fresh copy of the claimed value, a cell of T.
			// Written (#ref "T" v), as a struct's cell names its set: a word's
			// ascription is erased before lowering, and the reference's set is
			// what a read after the call is known to hold.
			ref := App(Name(RefName), a)
			if ty := d.setAt(i); ty != "" {
				var v *Term = &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), Str(a.Name), Str(ty), a}}
				if PlantCellSetUnclaimed {
					v = a
				}
				ref = App(Name(RefName), Str(ty), v)
			}
			kids = append(kids, ref)
			cellAt, cellNames = append(cellAt, i), append(cellNames, a.Name)
		case isOut(a):
			ty := TypeName(a.Kids[1])
			if a.Kids[1].Kind == KStr {
				ty = a.Kids[1].Str // an out-result's set, filled by the translation
			}
			if ty == "" {
				return nil, fmt.Errorf("%s: `out S` takes a set, got %s", ShowTerm(call), ShowTerm(a.Kids[1]))
			}
			if st != "" {
				ty = st // the declaration's struct: its layout, whatever name `out` used
			}
			kids = append(kids, App(Name(FreshName), Str(ty)))
			outAt = append(outAt, i)
		default:
			return nil, fmt.Errorf("%s: argument %d of %s is a cell the host writes, and takes a cell in "+
				"scope or `out S`; %s is neither. A host that writes through the address of a value the "+
				"program cannot read again has written into nothing (spec/local.md §5)",
				ShowTerm(call), i+1, ShowName(head.Name), ShowTerm(a))
		}
	}
	// The continuation's parameters: the out cells' values, then the declared
	// result's components.
	var outParams, resParams []string
	var body *Term
	if k != nil {
		if len(k.Params) != len(outAt)+d.Arity {
			return nil, fmt.Errorf("%s gives %d cell value(s) and then its declared result's %d "+
				"component(s), and the λ it is applied to takes %d (spec/local.md §6)",
				ShowTerm(call), len(outAt), d.Arity, len(k.Params))
		}
		outParams, resParams = k.Params[:len(outAt)], k.Params[len(outAt):]
		for _, p := range k.Params {
			c.locals[p] = true
		}
		b, err := c.stLoop(k.Body(), inLoop, carried)
		if err != nil {
			return nil, err
		}
		body = b
	}
	// The head stays where it is: the loader's variadic rewrite reads the
	// call by its declaration's name, so only the arguments are threaded.
	return c.thread(kids[1:], inLoop, carried, true, func(vs []*Term) *Term {
		call := App(head, vs...)
		deref := func(at []int) []*Term {
			out := make([]*Term, len(at))
			for j, i := range at {
				if st, rec := d.structAt(i); st != "" {
					out[j] = derefStruct(vs[i], rec)
					continue
				}
				out[j] = App(Name(DerefName), vs[i])
			}
			return out
		}
		outs, cells := deref(outAt), deref(cellAt)
		// AFTER THE CALL: the out cells' values, then each cell rebound to what
		// its reference holds, in argument order.
		after := func(rest *Term, names []string, vals []*Term) *Term {
			if len(names) == 0 {
				return rest
			}
			return App(Fn(names, rest), vals...)
		}
		if k != nil {
			inner := after(body, append(append([]string(nil), outParams...), cellNames...), append(outs, cells...))
			if d.Arity == 1 {
				// ONE RESULT IS NO PRODUCT, so its elimination is a let: the call,
				// impure, is bound first (ADR 0010), and the cells are read after it
				// (ADR 0112; every Go cell call so far answered a product).
				return App(Fn(resParams, inner), call)
			}
			return App(App(Name(ElimName), call), Fn(resParams, inner))
		}
		// AS A VALUE: the call's own value, or the out cells' values then the
		// declared result's components.
		y := c.fresh()
		value := Name(y)
		if len(outAt) > 0 {
			if d.Arity == 1 {
				value = Fn([]string{"#k"}, App(Name("#k"), append(outs, Name(y))...))
			} else {
				ys := make([]string, d.Arity)
				yv := make([]*Term, d.Arity)
				for j := range ys {
					ys[j] = c.fresh()
					yv[j] = Name(ys[j])
				}
				value = App(Name(y), Fn(ys, Fn([]string{"#k"}, App(Name("#k"), append(outs, yv...)...))))
			}
		}
		rest := after(c.tup(value), cellNames, cells)
		return App(Fn([]string{y}, rest), call)
	})
}

// derefStruct is what a struct's cell holds after the call, the record of its
// leaves each read back, (#deref r j) for leaf j in canonical order: the
// isomorphism's inverse, recordOfLeaves, over the reads.
func derefStruct(r *Term, ty string) *Term {
	j := 0
	var build func(ty string) *Term
	build = func(ty string) *Term {
		labels, types := RecordFields(ty)
		kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
		for _, ct := range types {
			if IsRecord(ct) {
				kids = append(kids, build(ct))
				continue
			}
			kids = append(kids, App(Name(DerefName), r, Int(int64(j))))
			j++
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return build(ty)
}

// PlantCellSetUnclaimed passes a scalar cell at a host's cell of T with no
// claim that its value is in T: a value outside T then reaches the host
// (TestAScalarCellIsTheHostsCell).
var PlantCellSetUnclaimed bool

// fillOuts is a C call with each out-result the caller omits (ADR 0131)
// written `out T` at its position, the trailing parameters: a call of the
// declared arity is left as it is.
func (d CellDecl) fillOuts(call *Term) *Term {
	if len(d.Outs) == 0 || len(call.Kids)-1 != d.NArgs-len(d.Outs) || PlantOutsUnfilled {
		return call
	}
	kids := append([]*Term(nil), call.Kids...)
	for j := range d.Outs {
		kids = append(kids, App(Name(OutName), Str(d.OutSets[j])))
	}
	return &Term{Kind: KApp, Kids: kids}
}

// PlantOutsUnfilled leaves a C call's out-results for the caller to write, as
// before ADR 0131: GetClientRect(w) is then a call of too few arguments
// (TestAnOutParameterIsAResult).
var PlantOutsUnfilled bool
