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

// prim is a primitive's Go form. Format uses {0}, {1}, … for operands.

// ---------------------------------------------------------------- emitter

type Emitter struct {
	tgt     *Target
	buf     strings.Builder
	imports map[string]bool
	types   map[string]string // variable -> inferred type
	weak    map[string]string // variable -> `any`, used only if nothing else fits
	tmp     int
	indent  int
}

// Func emits a top-level abstraction as a Go function.
func Func(tgt *Target, name string, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &Emitter{tgt: tgt, types: map[string]string{}, weak: map[string]string{},
		imports: map[string]bool{}}

	// Parameter types come from how the body uses them. Local propagation from
	// primitive signatures — not inference, just reading the table.
	e.inferFrom(t.Body())
	e.inferLet(t.Body())
	e.inferFrom(t.Body())

	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		ty := e.types[p]
		if ty == "" {
			ty = e.weak[p]
		}
		if ty == "" {
			return "", fmt.Errorf("cannot determine a Go type for parameter %q; "+
				"it is never passed to a primitive whose signature would fix it", p)
		}
		params[i] = mangle(p) + " " + e.tgt.ty(ty)
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
		e.tgt.ty(e.typeOf(t.Body())))
	out.WriteString(inner)
	fmt.Fprintf(&out, "\treturn %s\n}\n", result)
	for imp := range e.imports {
		Imports[imp] = true
	}
	return out.String(), nil
}

// Imports accumulates what emitted functions need. A package-level sink is
// crude — the real answer is that each binding declares its import and the file
// writer collects them, which is g5's Tier 2 binding format arriving in the
// emitter by need rather than by design.
var Imports = map[string]bool{}

// inferFrom walks the term assigning types to variables from the signatures of
// the primitives they are passed to.
func (e *Emitter) inferFrom(t *core.Term) {
	switch t.Kind {
	case core.KApp:
		op := t.Op()
		if op.Kind == core.KName {
			if p, ok := e.tgt.Prims[op.Name]; ok {
				for i, a := range t.Args() {
					if i < len(p.Args) && a.Kind == core.KName {
						// `any` is not a constraint, so it must not occupy the
						// slot a real one would fill. It is remembered weakly
						// instead: if nothing else ever constrains the name,
						// the host's own polymorphism is the honest answer.
						if p.Args[i] == "any" {
							e.weak[a.Name] = "any"
						} else if e.types[a.Name] == "" {
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

// inferLet seeds let-bound names, since their type comes from the bound value
// rather than from a primitive they are passed to.
func (e *Emitter) inferLet(t *core.Term) {
	if t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "let" {
		if k := t.Args()[1]; k.Kind == core.KFn && len(k.Params) == 1 {
			e.types[k.Params[0]] = e.typeOf(t.Args()[0])
		}
	}
	for _, k := range t.Kids {
		e.inferLet(k)
	}
	if t.Kind == core.KFn {
		e.inferLet(t.Body())
	}
}

func (e *Emitter) typeOf(t *core.Term) string {
	switch t.Kind {
	case core.KInt:
		return "int"
	case core.KFloat:
		return "f64"
	case core.KName:
		return e.types[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := e.tgt.Prims[op.Name]; ok {
				// A fold's type is its accumulator's type, not a fixed one,
				// and a let's is its body's.
				if p.Kind == "loop" {
					return e.typeOf(t.Args()[0])
				}
				if p.Kind == "build" {
					return "vec-f64"
				}
				if p.Kind == "let" {
					if k := t.Args()[1]; k.Kind == core.KFn {
						return e.typeOf(k.Body())
					}
				}
				// loop2's result is its FINISHER's type. This was never
				// implemented: it fell through to the declared result type,
				// which happened to say f64 — as false as fold-range's
				// accumulator type was, and only correct by luck. Removing the
				// declaration (target-files.md §4) exposed it.
				if p.Kind == "loop2" && len(t.Args()) == 6 {
					if fin := t.Args()[5]; fin.Kind == core.KFn && len(fin.Params) == 2 {
						e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
						if ty := e.typeOf(fin.Body()); ty != "" {
							return ty
						}
					}
					return e.typeOf(t.Args()[0])
				}
				// A statement's value IS argument 0, which is what every
				// target file has said since dict-inc and what none of them
				// implemented — dict-inc got away with declaring `dict` for
				// both. print-line is the first primitive where they differ.
				if p.Kind == "stmt" && len(t.Args()) > 0 {
					if ty := e.typeOf(t.Args()[0]); ty != "" {
						return ty
					}
				}
				return p.Result
			}
		}
	}
	return ""
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

	case core.KStr:
		// A literal was added to the language for target templates and no
		// backend could emit one, because no program had ever used one
		// (strings.md 1). strconv.Quote escapes non-ASCII to \u, which is
		// valid in all three hosts and sidesteps javac's platform-charset
		// hazard entirely (strings.md 5).
		return strconv.Quote(t.Str), nil

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
				"  The operator must be a primitive; recursive definitions were the other\n"+
				"  case and are rejected before reduction (ADR 0014).", t)
		}
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			return "", fmt.Errorf("no Go form for primitive %q", op.Name)
		}
		if p.Import != "" {
			e.imports[p.Import] = true
		}
		if p.Kind == "let" {
			return e.emitLet(t)
		}
		if p.Kind == "stmt" {
			args := t.Args()
			vals := make([]any, len(args))
			for i, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals[i] = v
			}
			if len(args) > 0 && !atomicValue(vals[0].(string)) && strings.Contains(p.Form, "%s") {
				name := e.fresh("v")
				// The type is explicit when we know it. `v1 := (21 + 21)` gives
				// Go's default `int`, and a function declaring `int64` then
				// fails to compile — the literals were an untyped constant
				// before they were bound.
				if ty := e.tgt.ty(e.typeOf(args[0])); ty != "" {
					e.line("var %s %s = %s", name, ty, vals[0])
				} else {
					e.line("%s := %s", name, vals[0])
				}
				vals[0] = name
			}
			e.line("%s", fill(p.Form, vals))
			return vals[0].(string), nil
		}
		if p.Kind == "build" {
			return e.emitMakeVec(t)
		}
		if p.Kind == "loop" {
			return e.emitFoldRange(t)
		}
		if p.Kind == "loop2" {
			return e.emitFoldRange2(t)
		}
		if p.Kind == "cond" {
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
		return "(" + fmt.Sprintf(p.Form, vals...) + ")", nil
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
	e.types[sx.Params[0]], e.types[sx.Params[1]], e.types[sx.Params[2]] = "f64", "f64", "int"
	e.types[sy.Params[0]], e.types[sy.Params[1]], e.types[sy.Params[2]] = "f64", "f64", "int"
	n := e.fresh("n")

	syBody := core.Rename(sy.Body(), map[string]string{
		sy.Params[0]: sx.Params[0], sy.Params[1]: sx.Params[1], sy.Params[2]: sx.Params[2]})

	e.line("%s, %s := %s, %s", ax, ay, x0, y0)
	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.emitNarrow(sx.Params[2], n, sx.Body(), syBody)
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	bx, err := e.emit(sx.Body())
	if err != nil {
		return "", err
	}
	// The second step names its own parameters; rebind them to the first's.
	by, err := e.emit(syBody)
	if err != nil {
		return "", err
	}
	e.line("%s, %s = %s, %s", ax, ay, bx, by)
	e.indent--
	e.line("}")

	// The finisher consumes both accumulators, so compound loop state never
	// crosses the loop boundary as a compound value.
	e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

// emitMakeVec allocates an array and fills it. It is the one primitive that
// lets a program CONSTRUCT data rather than only compute over data it was given
// (construction.md).
//
// The destination is fresh and every element is written exactly once, so it is
// unique by construction — which is why this lands without answering g7's
// aliasing question — and Go already knows len(dst) == n from make, so no
// narrowing is needed.
func (e *Emitter) emitMakeVec(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("make-vec takes a length and an element function")
	}
	elem := args[1]
	if elem.Kind != core.KFn || len(elem.Params) != 1 {
		return "", fmt.Errorf("make-vec's element function must be (fn (i) …), got %s", elem)
	}
	idxName := elem.Params[0]
	e.types[idxName] = "int"

	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	n := e.fresh("n")
	dst := e.fresh("v")
	idx := mangle(idxName)

	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.line("%s := make(%s, %s)", dst, e.tgt.ty("vec-f64"), n)
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(elem.Body())
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s", dst, idx, body)
	e.indent--
	e.line("}")
	e.types[dst] = "vec-f64"
	return dst, nil
}

// emitNarrow restricts every container the loop indexes by the BARE loop
// variable to the loop's own count, before the loop.
//
// This is the whole of bounds-check elimination, and it is worth 1.96x on Go
// (gauntlet/results/bce-2026-08-15.md). Our own proof buys nothing — Go's BCE
// has never heard of us — so the only thing that works is emitting a shape the
// host re-proves for itself. `q = q[:n]` turns one check per iteration into one
// check before the loop.
//
// It is legal precisely because docs/spec/primitives.md §2 specifies `aindex`
// as *unspecified out of bounds*: narrowing moves the panic earlier, and no
// program with defined meaning can tell.
//
// Conservative on purpose. A container is narrowed only if EVERY occurrence of
// it in the body is an index by the bare loop variable — the stencil indexes
// `a` at `j`, `j+1` and `j+2`, so `a` is left alone and stays correct.
func (e *Emitter) emitNarrow(idxName, n string, bodies ...*core.Term) {
	if e.tgt.Narrow == "" {
		return // this target has no such shape; JS and Java do not
	}
	good, bad := map[string]bool{}, map[string]bool{}
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Index {
				a := t.Args()
				if len(a) == 2 && a[0].Kind == core.KName &&
					a[1].Kind == core.KName && a[1].Name == idxName {
					good[a[0].Name] = true
					return // accounted for; do not mark it used elsewhere
				}
			}
		}
		switch t.Kind {
		case core.KName:
			bad[t.Name] = true
		case core.KFn:
			walk(t.Body())
		case core.KApp:
			for _, k := range t.Kids {
				walk(k)
			}
		}
	}
	for _, b := range bodies {
		walk(b)
	}

	names := make([]string, 0, len(good))
	for name := range good {
		if !bad[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := mangle(name)
		e.line("%s", fmt.Sprintf(e.tgt.Narrow, m, m, n))
	}
}

// emitLet binds a value to a name and continues with the body. The λ here is a
// binder, not a closure — which refines g6's "a surviving λ is an escaping
// closure" to "…unless it is let's continuation".
func (e *Emitter) emitLet(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("let takes a value and a continuation")
	}
	k := args[1]
	if k.Kind != core.KFn || len(k.Params) != 1 {
		return "", fmt.Errorf("let's continuation must be (fn (x) …), got %s", k)
	}
	val, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	// A binder used zero times is a sequencing point, not a binding: β keeps an
	// impure argument whatever its occurrence count (effects.md §5), and that is
	// what `seq` desugars into. Emitting `x := v` here would be rejected by Go
	// as an unused variable.
	if !core.Occurs(k.Body(), k.Params[0]) {
		if !emitsStatement(e.tgt, args[0]) {
			e.line("_ = %s", val) // Go forbids a bare expression statement
		}
		return e.emit(k.Body())
	}
	name := mangle(k.Params[0])
	e.types[k.Params[0]] = e.typeOf(args[0])
	e.line("%s := %s", name, val)
	return e.emit(k.Body())
}

// emitsStatement reports whether emitting this term already wrote a line for its
// effect, in which case a sequencing let needs to add nothing. Shared by the
// three backends, which agree here and differ only in how they spell a discarded
// expression.
func emitsStatement(tgt *Target, t *core.Term) bool {
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return false
	}
	return tgt.Prims[t.Op().Name].Kind == "stmt"
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
	if ty == "" {
		ty = e.typeOf(args[2])
	}
	tmp := e.fresh("t")
	e.line("var %s %s", tmp, e.tgt.ty(ty))
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
	e.types[accName] = e.typeOf(args[0])
	e.types[idxName] = "int"

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
	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.emitNarrow(idxName, n, step.Body())
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(step.Body())
	if err != nil {
		return "", err
	}
	if body != acc { // a statement-primitive already updated it in place
		e.line("%s = %s", acc, body)
	}
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
		// `-` word break, `.` qualifier, `/` module path separator: all three
		// become camel-case boundaries so a qualified name is one host identifier.
		case r == '-' || r == '.' || r == '/':
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
	if len(Imports) > 0 {
		imps := make([]string, 0, len(Imports))
		for i := range Imports {
			imps = append(imps, i)
		}
		sort.Strings(imps)
		for _, i := range imps {
			fmt.Fprintf(&out, "import %q\n", i)
		}
		out.WriteString("\n")
	}
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	return out.String()
}
