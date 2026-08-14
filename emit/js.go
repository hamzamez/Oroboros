package emit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"oroboros/core"
)

// JavaScript backend.
//
// Written standalone rather than factored against the Go backend on purpose:
// what the two turn out to share is the finding, and designing the shared
// interface first would have assumed the answer. See
// gauntlet/results/js-2026-08-14.md for what the duplication actually was.
//
// JS is the hostile host of ADR 0004 — no integers, no structs, no int64 — and
// the point of building it second is to find out which parts of emit/golang.go
// were general and which were Go-shaped assumptions.

type jsPrim struct {
	Format string
	Arity  int
	Loop   bool
	Loop2  bool
	Cond   bool
	Stmt   bool // emitted as a statement; the value is argument 0
}

var jsPrims = map[string]jsPrim{
	"add":    {Format: "%s + %s", Arity: 2},
	"mul":    {Format: "%s * %s", Arity: 2},
	"sub":    {Format: "%s - %s", Arity: 2},
	"gt":     {Format: "%s > %s", Arity: 2},
	"lt":     {Format: "%s < %s", Arity: 2},
	"alen":   {Format: "%s.length", Arity: 1},
	"aindex": {Format: "%s[%s]", Arity: 2},

	// The Parasite thesis: JS's dictionary is a null-prototype object, not Map.
	// Baseline R4 measured Map at 3.25x slower for string keys, which refuted
	// g4's original pass condition.
	"split-words": {Format: "%s.split(\" \")", Arity: 1},
	"slen":        {Format: "%s.length", Arity: 1},
	"sat":         {Format: "%s[%s]", Arity: 2},
	"dict-empty":  {Format: "Object.create(null)", Arity: 0},
	"dict-inc":    {Format: "%s[%s] = (%s[%s] ?? 0) + 1", Arity: 2, Stmt: true},

	"if":          {Arity: 3, Cond: true},
	"fold-range":  {Arity: 3, Loop: true},
	"fold-range2": {Arity: 6, Loop2: true},
}

type jsEmitter struct {
	buf    strings.Builder
	tmp    int
	indent int
}

// JSFunc emits a top-level abstraction as a JavaScript function.
//
// Note what is absent compared with Func: there is no type lattice, no
// inference, and no way for a parameter to fail to have a type. Everything
// emit/golang.go does with Ty exists to satisfy Go, not the language.
func JSFunc(name string, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &jsEmitter{indent: 1}
	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = jsMangle(p)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "export function %s(%s) {\n", jsMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	fmt.Fprintf(&out, "\treturn %s;\n}\n", result)
	return out.String(), nil
}

func (e *jsEmitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *jsEmitter) fresh(stem string) string {
	e.tmp++
	return fmt.Sprintf("%s%d", stem, e.tmp)
}

func (e *jsEmitter) emit(t *core.Term) (string, error) {
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10), nil

	case core.KFloat:
		// JS has one number type, so a float literal needs no decorating —
		// unlike Go, where 1 and 1.0 are different tokens.
		return strconv.FormatFloat(t.Float, 'g', -1, 64), nil

	case core.KName:
		return jsMangle(t.Name), nil

	case core.KFn:
		return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  This is an escaping closure. JS has first-class functions and could\n"+
			"  emit one directly, but g6's cost model has not been checked here.", t)

	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return "", fmt.Errorf("application of a non-name: %s", t)
		}
		p, ok := jsPrims[op.Name]
		if !ok {
			return "", fmt.Errorf("no JavaScript form for primitive %q", op.Name)
		}
		if p.Stmt {
			args := t.Args()
			vals := make([]any, 0, 2*len(args))
			for _, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals = append(vals, v)
			}
			// dict-inc names both operands twice; repeat them for the template.
			e.line("%s;", fmt.Sprintf(p.Format, append(vals, vals...)...))
			return vals[0].(string), nil
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
		if len(args) != p.Arity {
			return "", fmt.Errorf("%s takes %d argument(s), given %d", op.Name, p.Arity, len(args))
		}
		vals := make([]any, len(args))
		for i, a := range args {
			v, err := e.emit(a)
			if err != nil {
				return "", err
			}
			vals[i] = v
		}
		return "(" + fmt.Sprintf(p.Format, vals...) + ")", nil
	}
	return "", fmt.Errorf("unhandled term: %s", t)
}

// emitIf uses a conditional *expression* when both branches are pure, and falls
// back to a statement otherwise.
//
// This is the first real divergence from the Go backend: Go has no conditional
// expression, so emit/golang.go must always introduce a temporary. JS does, so
// the ANF that g3 §6 and g5 §4 both derived as necessary is **target-dependent**,
// not a property of the language.
func (e *jsEmitter) emitIf(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 3 {
		return "", fmt.Errorf("if takes a condition and two branches")
	}
	cond, err := e.emit(args[0])
	if err != nil {
		return "", err
	}

	// Emit both branches into a scratch buffer to see whether either needs
	// statements. If neither does, a ternary suffices.
	saved, savedIndent := e.buf.String(), e.indent
	e.buf.Reset()
	thenV, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	elseV, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	branchStmts := e.buf.String()
	e.buf.Reset()
	e.buf.WriteString(saved)
	e.indent = savedIndent

	if branchStmts == "" {
		return fmt.Sprintf("(%s ? %s : %s)", cond, thenV, elseV), nil
	}

	tmp := e.fresh("t")
	e.line("let %s;", tmp)
	e.line("if (%s) {", cond)
	e.indent++
	tv, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	e.line("%s = %s;", tmp, tv)
	e.indent--
	e.line("} else {")
	e.indent++
	ev, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	e.line("%s = %s;", tmp, ev)
	e.indent--
	e.line("}")
	return tmp, nil
}

// emitFoldRange2 carries two accumulators. Go has tuple assignment so it can
// write `ax, ay = …, …` directly; JS destructuring allocates an array, so the
// simultaneous update needs temporaries. That is g2 §6's parallel-assignment
// discipline, and the two targets need different code for it.
func (e *jsEmitter) emitFoldRange2(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 6 {
		return "", fmt.Errorf("fold-range2 takes x0, y0, count, stepX, stepY, finish")
	}
	sx, sy, fin := args[3], args[4], args[5]
	if fin.Kind != core.KFn || len(fin.Params) != 2 {
		return "", fmt.Errorf("fold-range2's finisher must be (fn (ax ay) …), got %s", fin)
	}
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
	ax, ay, idx := jsMangle(sx.Params[0]), jsMangle(sx.Params[1]), jsMangle(sx.Params[2])
	n := e.fresh("n")

	e.line("let %s = %s, %s = %s;", ax, x0, ay, y0)
	e.line("const %s = %s;", n, count)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	bx, err := e.emit(sx.Body())
	if err != nil {
		return "", err
	}
	by, err := e.emit(core.Rename(sy.Body(), map[string]string{
		sy.Params[0]: sx.Params[0], sy.Params[1]: sx.Params[1], sy.Params[2]: sx.Params[2]}))
	if err != nil {
		return "", err
	}
	tx, ty := e.fresh("u"), e.fresh("u")
	e.line("const %s = %s, %s = %s;", tx, bx, ty, by)
	e.line("%s = %s; %s = %s;", ax, tx, ay, ty)
	e.indent--
	e.line("}")
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

func (e *jsEmitter) emitFoldRange(t *core.Term) (string, error) {
	args := t.Args()
	step := args[2]
	if step.Kind != core.KFn || len(step.Params) != 2 {
		return "", fmt.Errorf("fold-range's third argument must be (fn (acc i) …), got %s", step)
	}
	init, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	acc := jsMangle(step.Params[0])
	idx := jsMangle(step.Params[1])
	n := e.fresh("n")

	e.line("let %s = %s;", acc, init)
	e.line("const %s = %s;", n, count)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(step.Body())
	if err != nil {
		return "", err
	}
	if body != acc { // a statement-primitive already updated it in place
		e.line("%s = %s;", acc, body)
	}
	e.indent--
	e.line("}")
	return acc, nil
}

// jsMangle differs from mangle only in its keyword set and in permitting `$`.
// The transformation itself — hyphen to camelCase, ? and ! to letters — is
// identical, which is evidence it belongs to the *language* rather than to
// either target.
func jsMangle(s string) string {
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
		case unicode.IsLetter(r) || r == '_' || r == '$':
			if upper {
				b.WriteString(strings.ToUpper(string(r)))
				upper = false
			} else {
				b.WriteRune(r)
			}
		case unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "u%04X", r)
		}
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "x" + out
	}
	if jsKeywords[out] {
		out += "_"
	}
	return out
}

var jsKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`await break case catch class const continue debugger
		default delete do else enum export extends false finally for function if implements
		import in instanceof interface let new null package private protected public return
		static super switch this throw true try typeof var void while with yield`) {
		jsKeywords[k] = true
	}
}

// JSFile wraps emitted functions in an ES module.
func JSFile(funcs map[string]string) string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	return out.String()
}
