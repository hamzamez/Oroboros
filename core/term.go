// Package core is the atom: lambda calculus in which the normal form is a
// parameter. See docs/the-atom.md and docs/spec/core-0.md.
//
// Nothing here knows about types, grades, rules, collections, or targets beyond
// a set of primitive names. Those are all later layers and are deliberately
// absent.
package core

import (
	"fmt"
	"strconv"
	"strings"
)

type Kind uint8

const (
	KName  Kind = iota // a variable or a global reference
	KInt               // integer literal
	KFloat             // float literal, IEEE-754 binary64 exactly
	KStr               // string literal
	KBool              // boolean literal, `true` or `false` (booleans.md)
	KFn                // (fn (p...) body)  — Params, Kids[0] = body
	KApp               // (f a...)          — Kids[0] = operator, Kids[1:] = operands
	KBound             // a bound variable, by index — never written in source
)

// Term is a pointer tree rather than the flat index-based arena of ADR 0005.
// A reducer allocates new terms on every substitution, so an append-only arena
// would grow without bound; the flat form is right for the IR file format
// (ADR 0006) and wrong here. Recorded as a deliberate deviation.
type Term struct {
	Kind   Kind
	Name   string
	Str    string
	Int    int64
	Float  float64
	Params []string // KFn: naming HINTS. The body refers to them by index.
	Index  int      // KBound: which parameter
	Depth  int      // KBound: how many binders out (0 = nearest)
	Kids   []*Term
}

func Name(s string) *Term   { return &Term{Kind: KName, Name: s} }
func Int(v int64) *Term     { return &Term{Kind: KInt, Int: v} }
func Float(v float64) *Term { return &Term{Kind: KFloat, Float: v} }
func Str(v string) *Term    { return &Term{Kind: KStr, Str: v} }

// Bool is the fourth literal kind. It carries its value in Int rather than in a
// field of its own: Term is allocated on every substitution, and a bool field
// would cost every term in the program a byte for the benefit of one kind.
func Bool(v bool) *Term {
	if v {
		return &Term{Kind: KBool, Int: 1}
	}
	return &Term{Kind: KBool}
}

// IsTrue reports which of the two a boolean literal is. Meaningless on any
// other kind, and callers check Kind first.
func (t *Term) IsTrue() bool { return t.Int != 0 }

// Fn builds an abstraction from an OPEN body, closing it. Adding a binder
// shifts anything already pointing outward.
func Fn(params []string, body *Term) *Term {
	return &Term{Kind: KFn, Params: params,
		Kids: []*Term{closeTerm(shift(body, 0, 1), params, 0)}}
}

// FnClosed builds one from an already-closed body. Reduction uses this: it never
// holds an open body, which is what keeps open/close round-trips out of the
// reducer entirely.
func FnClosed(params []string, body *Term) *Term {
	return &Term{Kind: KFn, Params: params, Kids: []*Term{body}}
}

func App(op *Term, args ...*Term) *Term {
	return &Term{Kind: KApp, Kids: append([]*Term{op}, args...)}
}

// Body OPENS the abstraction. Every existing reader — three backends, the
// checker, the refiner — keeps working unchanged.
func (t *Term) Body() *Term { return openTerm(t.Kids[0], t.Params, 0) }

// Closed is the body with indices intact.
func (t *Term) Closed() *Term { return t.Kids[0] }
func (t *Term) Op() *Term     { return t.Kids[0] }
func (t *Term) Args() []*Term { return t.Kids[1:] }

// String renders a term as an s-expression. Floats print with the shortest
// representation that round-trips, so that printing and reading are inverses
// (ADR 0009 — a value must not change by being written down).
func (t *Term) String() string {
	var sb strings.Builder
	t.write(&sb)
	return sb.String()
}

func (t *Term) write(sb *strings.Builder) {
	switch t.Kind {
	case KName:
		sb.WriteString(t.Name)
	case KInt:
		sb.WriteString(strconv.FormatInt(t.Int, 10))
	case KStr:
		sb.WriteString(strconv.Quote(t.Str))
	case KBool:
		if t.IsTrue() {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case KFloat:
		s := strconv.FormatFloat(t.Float, 'g', -1, 64)
		// A float literal must read back as a float, so 1 prints as 1.0.
		if !strings.ContainsAny(s, ".eEni") {
			s += ".0"
		}
		sb.WriteString(s)
	case KFn:
		sb.WriteString("(fn (")
		sb.WriteString(strings.Join(t.Params, " "))
		sb.WriteString(") ")
		t.Body().write(sb)
		sb.WriteString(")")
	case KBound:
		// Never reaches a printed residual: printing goes through Body, which
		// opens. Rendered so a bug is visible rather than silent.
		fmt.Fprintf(sb, "#%d.%d", t.Depth, t.Index)
	case KApp:
		sb.WriteString("(")
		for i, k := range t.Kids {
			if i > 0 {
				sb.WriteString(" ")
			}
			k.write(sb)
		}
		sb.WriteString(")")
	default:
		fmt.Fprintf(sb, "#<bad kind %d>", t.Kind)
	}
}

// Equal is structural equality up to nothing at all — not up to alpha. Tests
// compare printed forms, which is stricter and catches accidental renaming.
func (t *Term) Equal(u *Term) bool {
	if t.Kind != u.Kind {
		return false
	}
	switch t.Kind {
	case KName:
		return t.Name == u.Name
	case KInt:
		return t.Int == u.Int
	case KFloat:
		return t.Float == u.Float
	case KStr:
		return t.Str == u.Str
	case KBool:
		return t.Int == u.Int
	}
	if len(t.Params) != len(u.Params) || len(t.Kids) != len(u.Kids) {
		return false
	}
	for i := range t.Params {
		if t.Params[i] != u.Params[i] {
			return false
		}
	}
	for i := range t.Kids {
		if !t.Kids[i].Equal(u.Kids[i]) {
			return false
		}
	}
	return true
}

// Rename alpha-renames free occurrences. Backends need it when two abstractions
// that must share variables were written with different parameter names.
func Rename(t *Term, m map[string]string) *Term {
	sub := make(map[string]*Term, len(m))
	for from, to := range m {
		sub[from] = Name(to)
	}
	return substPublic(t, sub)
}

// Rename2 substitutes TERMS for names, capture-avoiding.
//
// It used to reimplement substitution, justified by a comment saying its input
// "contains no binders". That comment was true and the code was still wrong:
// it neither dropped shadowed names when descending under a λ nor freshened a
// binder that would capture. It was a second substitution written two hours
// after the first, which is exactly the fragility a locally nameless
// representation removes (concerns.md §1.3).
//
// The fix is to have one substitution, not two.
func Rename2(t *Term, m map[string]*Term) *Term {
	if t == nil {
		return nil
	}
	return substPublic(t, m)
}

// ---------------------------------------------------------------- locally nameless
//
// A FREE variable is a name; a BOUND variable is an index (concerns.md §1.3, s1).
// `Fn` closes its body and `Body` opens it, so substitution replaces a name while
// binders hold indices — capture is not avoided, it is unrepresentable.
//
// `Params` survives as a naming HINT, so the emitters keep producing `acc` and
// `i` rather than gensyms. A hint cannot cause a wrong answer: meaning is in the
// indices.
//
// Depth counts binders outward from the use; Index selects a parameter of that
// binder. Two levels, because an abstraction here takes several parameters at
// once.

func Bound(depth, index int) *Term { return &Term{Kind: KBound, Depth: depth, Index: index} }

// shift adds `by` to every bound variable pointing at or beyond `cutoff`.
//
// This is the part I omitted on the first attempt, and it is not an edge case:
// `duplicable` deliberately admits abstractions, because a duplicated λ must be
// substituted or fusion dies. Moving λs across binder depths is the mechanism
// this project runs on, and every such move needs a shift.
func shift(t *Term, cutoff, by int) *Term {
	if by == 0 {
		return t
	}
	switch t.Kind {
	case KBound:
		if t.Depth >= cutoff {
			return Bound(t.Depth+by, t.Index)
		}
		return t
	case KName, KInt, KFloat, KStr, KBool:
		return t
	case KFn:
		return &Term{Kind: KFn, Params: t.Params, Kids: []*Term{shift(t.Kids[0], cutoff+1, by)}}
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = shift(k, cutoff, by)
	}
	return &Term{Kind: KApp, Kids: kids}
}

// closeTerm replaces free occurrences of `params` with bound indices.
func closeTerm(t *Term, params []string, depth int) *Term {
	switch t.Kind {
	case KName:
		for i, p := range params {
			if t.Name == p {
				return Bound(depth, i)
			}
		}
		return t
	case KInt, KFloat, KStr, KBool, KBound:
		return t
	case KFn:
		return &Term{Kind: KFn, Params: t.Params, Kids: []*Term{closeTerm(t.Kids[0], params, depth+1)}}
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = closeTerm(k, params, depth)
	}
	return &Term{Kind: KApp, Kids: kids}
}

// openTerm replaces the bound variables of one binder level with names.
func openTerm(t *Term, names []string, depth int) *Term {
	switch t.Kind {
	case KBound:
		if t.Depth == depth && t.Index < len(names) {
			return Name(names[t.Index])
		}
		return t
	case KName, KInt, KFloat, KStr, KBool:
		return t
	case KFn:
		return &Term{Kind: KFn, Params: t.Params, Kids: []*Term{openTerm(t.Kids[0], names, depth+1)}}
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = openTerm(k, names, depth)
	}
	return &Term{Kind: KApp, Kids: kids}
}

// OpenWith is β: replace this abstraction's bound variables with terms and drop
// the binder level. A substituted term is shifted to its new depth; anything
// that pointed *through* the removed binder shifts down.
//
// No freshening, no free-variable computation, no capture avoidance.
func (t *Term) OpenWith(args []*Term) *Term { return openWith(t.Kids[0], args, 0) }

func openWith(t *Term, args []*Term, depth int) *Term {
	switch t.Kind {
	case KBound:
		if t.Depth == depth {
			if t.Index < len(args) {
				return shift(args[t.Index], 0, depth)
			}
			return t
		}
		if t.Depth > depth {
			return Bound(t.Depth-1, t.Index) // the binder it looked through is gone
		}
		return t
	case KName, KInt, KFloat, KStr, KBool:
		return t
	case KFn:
		return &Term{Kind: KFn, Params: t.Params, Kids: []*Term{openWith(t.Kids[0], args, depth+1)}}
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = openWith(k, args, depth)
	}
	return &Term{Kind: KApp, Kids: kids}
}
