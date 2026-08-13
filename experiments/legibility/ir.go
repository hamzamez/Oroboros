// Package legibility holds the rule-versus-pass comparison.
//
// This is not part of the gauntlet — the gauntlet is the fixed performance test
// (ADR 0007). This is a one-off experiment answering the last outstanding
// falsifier: is a layer written as rewrite rules harder to reason about than the
// same layer written as a compiler pass? If it is, requirement 8 is lost.
//
// The honest way to answer that is to write both, run them on the same input,
// and check they agree. Everything here is real and executable.
package legibility

import (
	"fmt"
	"strings"
)

// Node is the flat tagged representation ADR 0005 settles on: a kind enum, a
// small payload, and children by index rather than by pointer.
type Kind uint8

const (
	KLit   Kind = iota // F64
	KRef               // Str = name
	KVar               // Str = name, Kids[0] = init
	KSet               // Str = name, Kids[0] = value
	KAdd               // Kids[0] + Kids[1]
	KStruct            // Str = type name, Kids = fields
	KField             // I = field index, Kids[0] = value
	KSeq               // Kids in order
	KPar               // Kids simultaneously — see g2 §6
	KLoop              // Kids[0] = body
	KWhen              // Kids[0] = cond, Kids[1] = body
	KBreak
	KLT // Kids[0] < Kids[1]
)

var kindName = map[Kind]string{
	KLit: "lit", KRef: "ref", KVar: "var", KSet: "set", KAdd: "add",
	KStruct: "struct", KField: "field", KSeq: "seq", KPar: "par",
	KLoop: "loop", KWhen: "when", KBreak: "break", KLT: "lt",
}

type Node struct {
	Kind Kind
	Str  string
	F64  float64
	I    int
	Kids []int
}

// Fn is an arena of nodes plus a root index. Cycles are not constructible here
// for the same structural reason as in s5: children are indices into a slice
// that only ever grows, and a node is written once at construction.
type Fn struct {
	Nodes []Node
	Root  int
}

func (f *Fn) add(n Node) int { f.Nodes = append(f.Nodes, n); return len(f.Nodes) - 1 }

func (f *Fn) Lit(v float64) int      { return f.add(Node{Kind: KLit, F64: v}) }
func (f *Fn) Ref(s string) int       { return f.add(Node{Kind: KRef, Str: s}) }
func (f *Fn) Var(s string, k int) int { return f.add(Node{Kind: KVar, Str: s, Kids: []int{k}}) }
func (f *Fn) Set(s string, k int) int { return f.add(Node{Kind: KSet, Str: s, Kids: []int{k}}) }
func (f *Fn) Add(a, b int) int        { return f.add(Node{Kind: KAdd, Kids: []int{a, b}}) }
func (f *Fn) Struct(t string, ks ...int) int {
	return f.add(Node{Kind: KStruct, Str: t, Kids: ks})
}
func (f *Fn) Field(i, k int) int  { return f.add(Node{Kind: KField, I: i, Kids: []int{k}}) }
func (f *Fn) Seq(ks ...int) int   { return f.add(Node{Kind: KSeq, Kids: ks}) }
func (f *Fn) Par(ks ...int) int   { return f.add(Node{Kind: KPar, Kids: ks}) }
func (f *Fn) Loop(b int) int      { return f.add(Node{Kind: KLoop, Kids: []int{b}}) }
func (f *Fn) When(c, b int) int   { return f.add(Node{Kind: KWhen, Kids: []int{c, b}}) }
func (f *Fn) Break() int          { return f.add(Node{Kind: KBreak}) }
func (f *Fn) LT(a, b int) int     { return f.add(Node{Kind: KLT, Kids: []int{a, b}}) }

func (f *Fn) String() string {
	var sb strings.Builder
	f.print(&sb, f.Root, 0)
	return sb.String()
}

func (f *Fn) print(sb *strings.Builder, i, depth int) {
	n := f.Nodes[i]
	pad := strings.Repeat("  ", depth)
	switch n.Kind {
	case KLit:
		fmt.Fprintf(sb, "%s%g\n", pad, n.F64)
	case KRef:
		fmt.Fprintf(sb, "%s%s\n", pad, n.Str)
	case KVar, KSet:
		fmt.Fprintf(sb, "%s(%s %s\n", pad, kindName[n.Kind], n.Str)
		f.print(sb, n.Kids[0], depth+1)
		fmt.Fprintf(sb, "%s)\n", pad)
	case KField:
		fmt.Fprintf(sb, "%s(field %d\n", pad, n.I)
		f.print(sb, n.Kids[0], depth+1)
		fmt.Fprintf(sb, "%s)\n", pad)
	default:
		fmt.Fprintf(sb, "%s(%s", pad, kindName[n.Kind])
		if n.Str != "" {
			fmt.Fprintf(sb, " %s", n.Str)
		}
		sb.WriteString("\n")
		for _, k := range n.Kids {
			f.print(sb, k, depth+1)
		}
		fmt.Fprintf(sb, "%s)\n", pad)
	}
}

// Centroid builds the g2 accumulator: a struct-typed local updated every
// iteration. This is the input both implementations must scalarize.
//
//	(var acc (struct point 0 0))
//	(loop
//	  (when (lt i n) (break))
//	  (set acc (struct point (add (field 0 acc) x)
//	                        (add (field 1 acc) y))))
func Centroid() *Fn {
	f := &Fn{}
	init := f.Struct("point", f.Lit(0), f.Lit(0))
	declare := f.Var("acc", init)

	newX := f.Add(f.Field(0, f.Ref("acc")), f.Ref("x"))
	newY := f.Add(f.Field(1, f.Ref("acc")), f.Ref("y"))
	update := f.Set("acc", f.Struct("point", newX, newY))

	body := f.Seq(f.When(f.LT(f.Ref("i"), f.Ref("n")), f.Break()), update)
	f.Root = f.Seq(declare, f.Loop(body))
	return f
}

// Swap is the g2 §6 hazard: the right-hand side reads fields of the very
// variable being assigned, so splitting the assignment into a sequence
// miscompiles it. Both implementations must emit KPar, not KSeq.
//
//	(set acc (struct point (field 1 acc) (field 0 acc)))
func Swap() *Fn {
	f := &Fn{}
	declare := f.Var("acc", f.Struct("point", f.Lit(1), f.Lit(2)))
	swap := f.Set("acc", f.Struct("point",
		f.Field(1, f.Ref("acc")),
		f.Field(0, f.Ref("acc"))))
	f.Root = f.Seq(declare, swap)
	return f
}

// Triple is a three-field struct, to check that a lowering is arity-general
// rather than hard-coded to the two fields `point` happens to have.
func Triple() *Fn {
	f := &Fn{}
	declare := f.Var("t", f.Struct("triple", f.Lit(1), f.Lit(2), f.Lit(3)))
	update := f.Set("t", f.Struct("triple",
		f.Add(f.Field(0, f.Ref("t")), f.Lit(10)),
		f.Add(f.Field(1, f.Ref("t")), f.Lit(20)),
		f.Add(f.Field(2, f.Ref("t")), f.Lit(30))))
	f.Root = f.Seq(declare, update)
	return f
}
