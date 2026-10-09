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
}

// CellDeclTerm is the form a target hands the loader, (cells ARITY LIST i…).
func CellDeclTerm(d CellDecl) *Term {
	kids := []*Term{Name("cells"), Int(int64(d.Arity)), Int(int64(d.List))}
	for _, i := range d.Params {
		kids = append(kids, Int(int64(i)))
	}
	return &Term{Kind: KApp, Kids: kids}
}

func readCellDecl(t *Term) (CellDecl, error) {
	if t == nil || t.Kind != KApp || len(t.Kids) < 3 {
		return CellDecl{}, fmt.Errorf("a cells form is (cells ARITY LIST i…), got %s", t)
	}
	var ints []int
	for _, k := range t.Kids[1:] {
		if k.Kind != KInt {
			return CellDecl{}, fmt.Errorf("a cells form is (cells ARITY LIST i…), got %s", t)
		}
		ints = append(ints, int(k.Int))
	}
	return CellDecl{Arity: ints[0], List: ints[1], Params: ints[2:]}, nil
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
	for i, a := range t.Kids[1:] {
		if d.at(i) && !(a.Kind == KName && strings.HasPrefix(a.Name, "#")) {
			return true
		}
	}
	return false
}

func isOut(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == OutName
}

// translateCall is S⟦·⟧ of a call of a declaration with cell parameters, with
// k its continuation when it is eliminated, nil when it stands as a value.
func (c *cellTr) translateCall(call *Term, d CellDecl, k *Term, inLoop bool, carried []string) (*Term, error) {
	head := call.Kids[0]
	kids := []*Term{head}
	var outAt, cellAt []int // argument positions, in order
	var cellNames []string
	for i, a := range call.Kids[1:] {
		if !d.at(i) {
			kids = append(kids, a)
			continue
		}
		switch {
		case a.Kind == KName && c.isCell(a.Name):
			kids = append(kids, App(Name(RefName), a))
			cellAt, cellNames = append(cellAt, i), append(cellNames, a.Name)
		case isOut(a):
			ty := TypeName(a.Kids[1])
			if ty == "" {
				return nil, fmt.Errorf("%s: (out τ) takes a type, got %s", call, a.Kids[1])
			}
			kids = append(kids, App(Name(FreshName), Str(ty)))
			outAt = append(outAt, i)
		default:
			return nil, fmt.Errorf("%s: argument %d of %s is a cell the host writes, and takes a cell in "+
				"scope or (out τ); %s is neither. A host that writes through the address of a value the "+
				"program cannot read again has written into nothing (spec/local.md §5)",
				call, i+1, head.Name, a)
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
				call, len(outAt), d.Arity, len(k.Params))
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
