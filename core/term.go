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
	KFn                // (fn (p...) body)  — Params, Kids[0] = body
	KApp               // (f a...)          — Kids[0] = operator, Kids[1:] = operands
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
	Params []string
	Kids   []*Term
}

func Name(s string) *Term   { return &Term{Kind: KName, Name: s} }
func Int(v int64) *Term     { return &Term{Kind: KInt, Int: v} }
func Float(v float64) *Term { return &Term{Kind: KFloat, Float: v} }
func Str(v string) *Term    { return &Term{Kind: KStr, Str: v} }

func Fn(params []string, body *Term) *Term {
	return &Term{Kind: KFn, Params: params, Kids: []*Term{body}}
}

func App(op *Term, args ...*Term) *Term {
	return &Term{Kind: KApp, Kids: append([]*Term{op}, args...)}
}

func (t *Term) Body() *Term   { return t.Kids[0] }
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
