// Package ir is the compiler's intermediate representation: structured SSA with
// π-parameters, specified in docs/spec/ir.md and decided in ADR 0032.
//
// It is a presentation of a distributive Freyd category with Elgot iteration
// (spec §1). A function is a region; a region is parameters, π-parameters, a
// sequence of operations and one terminator; an operation may own regions. The
// operation set Σ is closed (spec §1.2): a consumer that switches over Op or
// over Term with no default is told by the compiler when either grows.
package ir

import (
	"fmt"

	"oroboros/core"
)

// Version is the format's version (spec §10.2). It changes only when the
// meaning of an existing operation or the syntax changes.
const Version = 1

// V is a value: an id unique within its function (W1).
type V int32

// Op is an operation of Σ (spec §1.2).
type Op uint8

const (
	OConst Op = iota
	OGlobal
	OAdd
	OSub
	OMul
	ONeg
	ODiv
	ORem
	OEq
	ONe
	OLt
	OLe
	OGt
	OGe
	OCall
	OIndex
	OLen
	OArray
	OMap
	ORead
	OKeys
	OSet
	OInsert
	OIf
	OLoop
	OBuild
	OBuildMap
	OTabulate
	OThe
	ORequire
	numOps
)

var opNames = [numOps]string{
	OConst: "const", OGlobal: "global",
	OAdd: "add", OSub: "sub", OMul: "mul", ONeg: "neg", ODiv: "div", ORem: "rem",
	OEq: "eq", ONe: "ne", OLt: "lt", OLe: "le", OGt: "gt", OGe: "ge",
	OCall: "call", OIndex: "index", OLen: "len", OArray: "array", OMap: "map",
	ORead: "read", OKeys: "keys", OSet: "set", OInsert: "insert",
	OIf: "if", OLoop: "loop", OBuild: "build", OBuildMap: "build-map", OTabulate: "tabulate",
	OThe: "the", ORequire: "require",
}

func (o Op) String() string {
	if o < numOps {
		return opNames[o]
	}
	return fmt.Sprintf("op%d", o)
}

// OpNamed is the operation spelled s, if Σ has one.
func OpNamed(s string) (Op, bool) {
	for i, n := range opNames {
		if n == s {
			return Op(i), true
		}
	}
	return 0, false
}

// IsArith and IsCmp partition the integer operations (spec §1.2).
func (o Op) IsArith() bool { return o >= OAdd && o <= ORem }
func (o Op) IsCmp() bool   { return o >= OEq && o <= OGe }

// Pure reports whether an operation is central (spec §1.1). A call's purity is
// its primitive's declared bit, which the caller supplies.
func (o Op) Pure() bool {
	switch o {
	case OSet, OInsert, OBuild, OBuildMap:
		return false
	}
	return true
}

// Mode is an integer operation's mode (spec §4.4). MNone is IR_A's undecided
// operation; IR_P has none (W9).
type Mode uint8

const (
	MNone Mode = iota
	MExact
	MTrap
)

func (m Mode) String() string {
	switch m {
	case MExact:
		return "exact"
	case MTrap:
		return "trap"
	}
	return ""
}

// Stmt is one operation applied: `(val RES… OP)` or `(do OP)` (spec §2.1).
type Stmt struct {
	Op   Op
	Mode Mode       // integer arithmetic only
	Name string     // OCall: the primitive; OGlobal: the global
	Lit  *core.Term // OConst: the literal
	Type string     // OThe: the ascribed type (canonical spelling)
	Args []V
	Res  []V
	Sub  []*Region
}

// Term is a region's terminator (spec §1.3).
type Term uint8

const (
	TYield Term = iota
	TBreak
	TContinue
	TBranch
)

var termNames = [...]string{TYield: "yield", TBreak: "break", TContinue: "continue", TBranch: "branch"}

func (t Term) String() string { return termNames[t] }

// Pi is a π-parameter (spec §6): V is Of renamed on this arm, where
// `Of Rel Other` holds, or `(len Of) Rel Other` when Len.
type Pi struct {
	V, Of, Other V
	Rel          string
	Len          bool
}

// Region is parameters, π-parameters, statements and a terminator.
type Region struct {
	Params     []V
	Pis        []Pi
	Stmts      []Stmt
	T          Term
	Args       []V // yield, break, continue
	Cond       V   // branch
	Then, Else *Region
}

// Func is one exported definition.
type Func struct {
	Name    string
	Params  []V
	Results []string // canonical types
	Body    *Region
	Types   []string // every value's type, canonical spelling, indexed by V
}

// NV is the number of values the function defines.
func (f *Func) NV() int { return len(f.Types) }

// Global is a closed constant graph named at module level (spec §2.4).
type Global struct {
	Name string
	Type string
	Lit  *core.Term
}

// Stage is IR_A or IR_P (spec §0, §7).
type Stage uint8

const (
	StageA Stage = iota
	StageP
)

func (s Stage) String() string {
	if s == StageP {
		return "P"
	}
	return "A"
}

// Program is one IR file.
type Program struct {
	Target  string
	Stage   Stage
	Header  []string // the `ops` a read file declared; nil for a program built in memory
	Globals []Global
	Funcs   []*Func
}

// Ops is the set of operations and terminators a program uses, in Σ's order:
// the header's `ops` (spec §10.2), which covering reads.
func (p *Program) Ops() []string {
	var used [numOps]bool
	var terms [4]bool
	var walk func(r *Region)
	walk = func(r *Region) {
		for i := range r.Stmts {
			used[r.Stmts[i].Op] = true
			for _, s := range r.Stmts[i].Sub {
				walk(s)
			}
		}
		terms[r.T] = true
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	for _, f := range p.Funcs {
		walk(f.Body)
	}
	var out []string
	for i, u := range used {
		if u {
			out = append(out, opNames[i])
		}
	}
	for i, u := range terms {
		if u {
			out = append(out, termNames[i])
		}
	}
	return out
}

// Walk visits every region of f, outermost first.
func (f *Func) Walk(visit func(r *Region)) {
	var walk func(r *Region)
	walk = func(r *Region) {
		visit(r)
		for i := range r.Stmts {
			for _, s := range r.Stmts[i].Sub {
				walk(s)
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
}
