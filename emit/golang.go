// Package emit turns a normal form into target source.
//
// This is the first code that produces output measurable against the gauntlet.
// Everything before it measured hand-written host code — the bar — and never
// what we produce.
//
// Deliberately narrow: it handles the residual we actually have, and the shape
// of what it needs is the finding. In particular the primitive table below is
// hardcoded, and what a *general* binding format must carry should be read off
// from what this file turned out to need, rather than designed in advance.
package emit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"oroboros/core"
)

// ---------------------------------------------------------------- types
//
// A four-point lattice, not a type system. The language stays untyped; Go needs
// types, so the backend works them out. Nothing here is visible in the source
// language, which is the point — the emitter must not push types up into the
// core before we know we need them.

type Ty uint8

const (
	TUnknown Ty = iota
	TF64
	TInt
	TBool
	TVecF64
)

func (t Ty) Go() string {
	switch t {
	case TF64:
		return "float64"
	case TInt:
		return "int"
	case TBool:
		return "bool"
	case TVecF64:
		return "[]float64"
	}
	return "interface{}" // will not compile — deliberately loud
}

// prim is a primitive's Go form. Format uses {0}, {1}, … for operands.
type prim struct {
	Format string
	Args   []Ty
	Result Ty
	Loop   bool // emitted as a loop statement
	Loop2  bool // loop carrying two accumulators
	Cond   bool // emitted as an if statement
}

var prims = map[string]prim{
	"add":    {Format: "%s + %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"mul":    {Format: "%s * %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"sub":    {Format: "%s - %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"alen":   {Format: "len(%s)", Args: []Ty{TVecF64}, Result: TInt},
	"aindex": {Format: "%s[%s]", Args: []Ty{TVecF64, TInt}, Result: TF64},
	"gt":     {Format: "%s > %s", Args: []Ty{TF64, TF64}, Result: TBool},
	"lt":     {Format: "%s < %s", Args: []Ty{TF64, TF64}, Result: TBool},

	// if(cond, then, else) — the SECOND statement-primitive. Go has no
	// conditional expression, so this is ANF arriving for a fourth time.
	"if": {Args: []Ty{TBool, TUnknown, TUnknown}, Result: TUnknown, Cond: true},

	// fold-range(init, count, (fn (acc i) body)) — the one primitive that is a
	// statement rather than an expression. How this generalises is the main
	// open question the emitter raises.
	"fold-range": {Args: []Ty{TF64, TInt, TUnknown}, Result: TF64, Loop: true},

	// fold-range2(x0, y0, n, stepX, stepY) — a loop carrying TWO accumulators.
	// Needed because compile-time reduction cannot cross a runtime loop
	// boundary, so loop-carried state must be primitive-shaped. See
	// gauntlet/results/structs-2026-08-14.md.
	"fold-range2": {Args: []Ty{TF64, TF64, TInt, TUnknown, TUnknown, TUnknown}, Result: TF64, Loop2: true},
}

// ---------------------------------------------------------------- emitter

type Emitter struct {
	buf    strings.Builder
	types  map[string]Ty // variable -> inferred type
	tmp    int
	indent int
}

// Func emits a top-level abstraction as a Go function.
func Func(name string, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &Emitter{types: map[string]Ty{}}

	// Parameter types come from how the body uses them. Local propagation from
	// primitive signatures — not inference, just reading the table.
	e.inferFrom(t.Body())

	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		ty := e.types[p]
		if ty == TUnknown {
			return "", fmt.Errorf("cannot determine a Go type for parameter %q; "+
				"it is never passed to a primitive whose signature would fix it", p)
		}
		params[i] = mangle(p) + " " + ty.Go()
	}

	var body strings.Builder
	e.buf = body
	e.indent = 1
	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	inner := e.buf.String()

	var out strings.Builder
	fmt.Fprintf(&out, "func %s(%s) %s {\n", export(name), strings.Join(params, ", "),
		e.typeOf(t.Body()).Go())
	out.WriteString(inner)
	fmt.Fprintf(&out, "\treturn %s\n}\n", result)
	return out.String(), nil
}

// inferFrom walks the term assigning types to variables from the signatures of
// the primitives they are passed to.
func (e *Emitter) inferFrom(t *core.Term) {
	switch t.Kind {
	case core.KApp:
		op := t.Op()
		if op.Kind == core.KName {
			if p, ok := prims[op.Name]; ok {
				for i, a := range t.Args() {
					if i < len(p.Args) && a.Kind == core.KName {
						if e.types[a.Name] == TUnknown {
							e.types[a.Name] = p.Args[i]
						}
					}
				}
			}
		}
		for _, k := range t.Kids {
			e.inferFrom(k)
		}
	case core.KFn:
		e.inferFrom(t.Body())
	}
}

func (e *Emitter) typeOf(t *core.Term) Ty {
	switch t.Kind {
	case core.KInt:
		return TInt
	case core.KFloat:
		return TF64
	case core.KName:
		return e.types[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := prims[op.Name]; ok {
				return p.Result
			}
		}
	}
	return TUnknown
}

func (e *Emitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *Emitter) fresh(stem string) string {
	e.tmp++
	return fmt.Sprintf("%s%d", stem, e.tmp)
}

// emit writes any statements the term needs and returns a Go expression for its
// value. This is the expression/statement split that g3 §6 and g5 §4 both
// demanded under the name ANF — here it is, arriving for a third time.
func (e *Emitter) emit(t *core.Term) (string, error) {
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10), nil

	case core.KFloat:
		s := strconv.FormatFloat(t.Float, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s, nil

	case core.KName:
		return mangle(t.Name), nil

	case core.KFn:
		return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  This is an escaping closure. g6 measured its cost but the emitter\n"+
			"  does not implement closures yet.", t)

	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return "", fmt.Errorf("application of a non-name: %s\n"+
				"  The operator must be a primitive or a recursive definition.", t)
		}
		p, ok := prims[op.Name]
		if !ok {
			return "", fmt.Errorf("no Go form for primitive %q", op.Name)
		}
		if p.Loop {
			return e.emitFoldRange(t)
		}
		if p.Loop2 {
			return e.emitFoldRange2(t)
		}
		if p.Cond {
			return e.emitIf(t)
		}
		args := t.Args()
		if len(args) != len(p.Args) {
			return "", fmt.Errorf("%s takes %d argument(s), given %d", op.Name, len(p.Args), len(args))
		}
		vals := make([]any, len(args))
		for i, a := range args {
			v, err := e.emit(a)
			if err != nil {
				return "", err
			}
			vals[i] = v
		}
		// Parenthesised throughout rather than tracking precedence. Go's parser
		// does not care and neither does its optimiser; gofmt would strip them.
		return "(" + fmt.Sprintf(p.Format, vals...) + ")", nil
	}
	return "", fmt.Errorf("unhandled term: %s", t)
}

// emitFoldRange2 emits a loop with two accumulators. The pair is updated
// SIMULTANEOUSLY — g2 §6's parallel-assignment hazard, arriving in the emitter.
// Go has tuple assignment; other targets need temporaries.
func (e *Emitter) emitFoldRange2(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 6 {
		return "", fmt.Errorf("fold-range2 takes x0, y0, count, stepX, stepY, finish")
	}
	fin := args[5]
	if fin.Kind != core.KFn || len(fin.Params) != 2 {
		return "", fmt.Errorf("fold-range2's finisher must be (fn (ax ay) …), got %s", fin)
	}
	sx, sy := args[3], args[4]
	for _, s := range []*core.Term{sx, sy} {
		if s.Kind != core.KFn || len(s.Params) != 3 {
			return "", fmt.Errorf("fold-range2 steps must be (fn (ax ay i) …), got %s", s)
		}
	}
	x0, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	y0, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	ax, ay, idx := mangle(sx.Params[0]), mangle(sx.Params[1]), mangle(sx.Params[2])
	e.types[sx.Params[0]], e.types[sx.Params[1]], e.types[sx.Params[2]] = TF64, TF64, TInt
	e.types[sy.Params[0]], e.types[sy.Params[1]], e.types[sy.Params[2]] = TF64, TF64, TInt
	n := e.fresh("n")

	e.line("%s, %s := %s, %s", ax, ay, x0, y0)
	e.line("%s := %s", n, count)
	e.line("for %s := 0; %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	bx, err := e.emit(sx.Body())
	if err != nil {
		return "", err
	}
	// The second step names its own parameters; rebind them to the first's.
	by, err := e.emit(core.Rename(sy.Body(), map[string]string{
		sy.Params[0]: sx.Params[0], sy.Params[1]: sx.Params[1], sy.Params[2]: sx.Params[2]}))
	if err != nil {
		return "", err
	}
	e.line("%s, %s = %s, %s", ax, ay, bx, by)
	e.indent--
	e.line("}")

	// The finisher consumes both accumulators, so compound loop state never
	// crosses the loop boundary as a compound value.
	e.types[fin.Params[0]], e.types[fin.Params[1]] = TF64, TF64
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

// emitIf turns (if c then else) into a Go if statement assigning to a temporary,
// because Go has no conditional expression. The branches may themselves emit
// statements, which is why this cannot be a format string.
func (e *Emitter) emitIf(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 3 {
		return "", fmt.Errorf("if takes a condition and two branches")
	}
	cond, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	ty := e.typeOf(args[1])
	if ty == TUnknown {
		ty = e.typeOf(args[2])
	}
	tmp := e.fresh("t")
	e.line("var %s %s", tmp, ty.Go())
	e.line("if %s {", cond)
	e.indent++
	thenV, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	e.line("%s = %s", tmp, thenV)
	e.indent--
	e.line("} else {")
	e.indent++
	elseV, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	e.line("%s = %s", tmp, elseV)
	e.indent--
	e.line("}")
	return tmp, nil
}

// emitFoldRange turns (fold-range init count (fn (acc i) body)) into a loop.
func (e *Emitter) emitFoldRange(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 3 {
		return "", fmt.Errorf("fold-range takes init, count and a step function")
	}
	step := args[2]
	if step.Kind != core.KFn || len(step.Params) != 2 {
		return "", fmt.Errorf("fold-range's third argument must be (fn (acc i) …), got %s", step)
	}
	accName, idxName := step.Params[0], step.Params[1]
	e.types[accName] = TF64
	e.types[idxName] = TInt

	init, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[1])
	if err != nil {
		return "", err
	}

	acc := mangle(accName)
	idx := mangle(idxName)
	n := e.fresh("n")

	e.line("%s := %s", acc, init)
	e.line("%s := %s", n, count)
	e.line("for %s := 0; %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(step.Body())
	if err != nil {
		return "", err
	}
	e.line("%s = %s", acc, body)
	e.indent--
	e.line("}")
	return acc, nil
}

// ---------------------------------------------------------------- names
//
// Our identifiers admit -, ?, !, and any Unicode letter. Go's do not. Mangling
// is therefore forced, and it is the first place the language and the target
// disagree about what a name is.
//
// Two jobs, and they must not be confused: making a name *legal*, and making it
// *exported*. mangle does only the first — capitalising every identifier turned
// locals into exported-looking names, which is wrong and was caught by the very
// first emission.

func mangle(s string) string {
	var b strings.Builder
	upper := false
	for _, r := range s {
		switch {
		case r == '-' || r == '.':
			upper = true
		case r == '?':
			b.WriteString("P")
		case r == '!':
			b.WriteString("B")
		case unicode.IsLetter(r) || r == '_':
			if upper {
				b.WriteString(strings.ToUpper(string(r)))
				upper = false
			} else {
				b.WriteRune(r)
			}
		case unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "U%04X", r)
		}
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "X" + out
	}
	if goKeywords[out] {
		out += "_"
	}
	return out
}

// export makes a legal name exported. Separate from mangle by design.
func export(s string) string {
	m := mangle(s)
	if m == "" {
		return m
	}
	return strings.ToUpper(m[:1]) + m[1:]
}

var goKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`break case chan const continue default defer else
		fallthrough for func go goto if import interface map package range return select
		struct switch type var`) {
		goKeywords[k] = true
	}
}

// File wraps emitted functions in a compilable Go file.
func File(pkg string, funcs map[string]string) string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\n", pkg)
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	return out.String()
}
