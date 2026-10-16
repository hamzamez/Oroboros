// Package jdecl prints host declarations in J (spec/j.md §2.11, ADR 0064): the
// one printer of the surveys' generators, which write the declarable subset of a
// host as target files. A declaration is a header and one host body; a set is a
// language set, a host carrier, a range or a former over them.
//
// A name is an atom of abstract.md's 𝒩, kebab-case as the generators make it;
// σ_J (core.Spell) spells it here, so a host name outside J's identifier alphabet,
// a generic instance `typedArshalers[jsontext-Decoder]`, is quoted.
package jdecl

import (
	"strings"

	"oroboros/core"
)

// langSets are the language's sets, spelled by the convention (ADR 0067 item 3).
var langSets = map[string]string{"int": "Int", "bool": "Bool", "string": "String", "f64": "F64",
	"any": "Any"}

// Name spells a set's name: a language set by the convention, anything else by σ_J.
func Name(n string) string {
	if s, ok := langSets[n]; ok {
		return s
	}
	for _, s := range langSets {
		if s == n {
			return s // already spelled
		}
	}
	return core.Spell(n)
}

// Ident spells a declared name or a parameter by σ_J.
func Ident(n string) string { return core.Spell(n) }

// Range is the set lo..hi, each end spelled as the bound it is (ADR 0067 item 4).
func Range(lo, hi string) string { return core.Integer(lo) + ".." + core.Integer(hi) }

// Array is the set of tables over e.
func Array(e string) string { return "Array(" + e + ")" }

// Map is the set of maps from k to v.
func Map(k, v string) string { return "Map(" + k + ", " + v + ")" }

// Product is the n-ary product, TLA+'s × (ADR 0065); the empty one is the unit.
func Product(ts ...string) string {
	if len(ts) == 0 {
		return "()"
	}
	return strings.Join(ts, " × ")
}

// Hole is a template's named splice of a parameter (ADR 0066 item 9).
func Hole(param string) string { return "${" + core.Spell(param) + "}" }

// Quote writes a string literal with string-literals.md's escapes; a `${` the
// host's own text holds is escaped as `$${`.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Template quotes a template whose splices are already named, escaping
// nothing the generator wrote as a splice.
func Template(t string) string { return Quote(t) }

// A Param is a binder and its set, already spelled.
type Param struct{ Name, Set string }

// A Fn is a host declaration: `fn NAME(x ∈ S, …) (R) [pure] { host KIND "t" … }`.
type Fn struct {
	Name     string
	Params   []Param
	Result   string // a set, spelled
	Pure     bool
	Kind     string // expr or stmt
	Template string // its splices named, Hole
	Import   string
	Lib      string
}

// String is the declaration on one line, indented for a module's body.
func (f Fn) String() string {
	var ps []string
	for _, p := range f.Params {
		ps = append(ps, Ident(p.Name)+" ∈ "+p.Set)
	}
	var b strings.Builder
	b.WriteString("    fn " + Ident(f.Name) + "(" + strings.Join(ps, ", ") + ") (" + f.Result + ")")
	if f.Pure {
		b.WriteString(" pure")
	}
	b.WriteString(" { host " + f.Kind + " " + Template(f.Template))
	if f.Import != "" {
		b.WriteString(" import " + Quote(f.Import))
	}
	if f.Lib != "" {
		b.WriteString(" lib " + Quote(f.Lib))
	}
	b.WriteString(" }\n")
	return b.String()
}

// Const is an integer constant, its one value the exact set: `NAME ∈ set { v } =
// host "h" import "p"`, indented for a module's body.
func Const(name, value, host, imp string) string {
	s := "    " + Ident(name) + " ∈ set { " + core.Integer(value) + " } = host " + Quote(host)
	if imp != "" {
		s += " import " + Quote(imp)
	}
	return s + "\n"
}

// Set is a set with a host carrier, `set NAME = host "h"`.
func Set(name, host string) string { return "set " + Name(name) + " = host " + Quote(host) + "\n" }

// Underlying is a named host type's integer set, `underlying NAME S`.
func Underlying(name, set string) string { return "underlying " + Name(name) + " " + set + "\n" }

// Axiom is an inclusion of a model (ADR 0064): `axiom T ⊆ I₁ ∩ … ∩ Iₙ`.
func Axiom(sub string, sups []string) string {
	var ns []string
	for _, s := range sups {
		ns = append(ns, Name(s))
	}
	return "axiom " + Name(sub) + " ⊆ " + strings.Join(ns, " ∩ ") + "\n"
}

// Word is the target's word, `repr lo..hi word` (ADR 0026).
func Word(lo, hi string) string { return "repr " + Range(lo, hi) + " word\n" }

// Module is a module and its body, `module PATH { … }`.
// A segment J's path does not read as written, `windows.ui.interop`, is quoted.
func Module(path, body string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if !pathWord(s) {
			segs[i] = "`" + s + "`"
		}
	}
	return "module " + strings.Join(segs, "/") + " {\n" + body + "}\n"
}

// pathWord reports a segment J reads as written: letters, digits, `_` and `-`.
func pathWord(s string) bool {
	for _, r := range s {
		if !(r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return s != ""
}

// Comment is a line comment for each line.
func Comment(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		if l == "" {
			b.WriteString("//\n")
			continue
		}
		b.WriteString("// " + l + "\n")
	}
	return b.String()
}

// PrimLine matches one declaration a generated file holds, a host function or
// a constant: what the surveys count as emitted.
const PrimLine = `(?m)^\s*(fn \S|\S+ ∈ set \{)`
