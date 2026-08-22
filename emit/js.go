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

type jsEmitter struct {
	tgt    *Target
	buf    strings.Builder
	tmp    int
	indent int

	// bound is every name already emitted in this function — see openFresh.
	bound map[string]bool

	// tail is true while emitting a term whose value IS the function's value.
	// A loop in that position returns directly instead of assigning a result
	// variable and breaking, and `returned` records that it did so the wrapper
	// omits its own `return`.
	//
	// A flag rather than the term itself: `Term.Body()` opens de Bruijn
	// indices into names and ALLOCATES, so two calls to it are different
	// pointers and identity cannot be used to recognise a position.
	//
	// Measured, not tidied. On V8 the same search loop written with an early
	// `return` runs at 36,973 ns and written with a result variable and
	// `break` at 48,383 — 1.31x for the shape alone. On Go the two are
	// indistinguishable, which is why this lives here and not in the shared
	// path: JS is the hostile host and this is the sort of thing it exists to
	// surface (native-js-2026-08-20).
	tail     bool
	returned bool
}

// JSFunc emits a top-level abstraction as a JavaScript function.
//
// Note what is absent compared with Func: there is no type lattice, no
// inference, and no way for a parameter to fail to have a type. Everything
// emit/golang.go does with Ty exists to satisfy Go, not the language.
// The signature is accepted and unused: JavaScript needs no parameter types,
// which is the measurement targets/js.oro records by declaring none.
func JSFunc(tgt *Target, name string, sig *core.Sig, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &jsEmitter{tgt: tgt, indent: 1, bound: map[string]bool{}}
	// The PARAMETERS are already bound, and forgetting them is not a cosmetic
	// bug on this host: `(loop ((n n)) …)` emitted `let n = n;` inside
	// `function f(n)`, which is a SyntaxError — the module does not parse.
	// Go and Java seeded this from the start; JavaScript did not, and nothing
	// noticed until `match` made reusing a parameter name the common case.
	for _, p := range t.Params {
		e.bound[jsMangle(p)] = true
	}

	// SEVERAL RESULTS. This is the ONE place JSFunc reads the signature, and
	// the reason is that JavaScript has no multiple return: the arity has to
	// come from somewhere and the host cannot supply it. Everywhere else JS
	// needs no types, which is the measurement targets/js.oro records by
	// declaring none.
	if sig != nil && len(sig.Results) > 1 {
		return e.multiFunc(name, sig, t)
	}

	e.tail = true
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
	if e.returned {
		// A loop in tail position returned from every exit, so there is no
		// value left to return and no reachable statement after the loop.
		out.WriteString("}\n")
	} else {
		fmt.Fprintf(&out, "\treturn %s;\n}\n", result)
	}
	return out.String(), nil
}

// multiFunc emits a function with several results.
//
// JavaScript has no native multiple return, so the language construct is built
// out of what the host has — which is the compiler's job, not a target author's.
// An OBJECT LITERAL with a fixed shape, measured rather than chosen for how it
// reads (mrshape-2026-08-22):
//
//	                     caller uses p.f0   caller destructures
//	  return [a, b]           8,348 ns            955 ns
//	  return {f0, f1}         5,164 ns            956 ns
//	  no product at all            —              940 ns
//
// The object is 1.62x faster when the caller reads a property and identical
// when it destructures, so it is better or equal in both. The first version
// emitted an array because `const [a, b] = f()` reads well — clarity over
// requirement 5, argued from a measurement of a different shape.
//
// And the larger finding, which belongs with the CALLER rather than here:
// destructuring at the call site costs nothing, because V8 scalar-replaces the
// object entirely and lands on the no-product number. Binding it and reading
// fields keeps the allocation, at 5.4x.
func (e *jsEmitter) multiFunc(name string, sig *core.Sig, t *core.Term) (string, error) {
	vs, ok := multiValue(t.Body(), len(sig.Results))
	if !ok {
		return "", multiResultErr(name, sig, t.Body())
	}
	outs := make([]string, len(vs))
	for i, v := range vs {
		s, err := e.emit(v)
		if err != nil {
			return "", err
		}
		outs[i] = s
	}
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = jsMangle(p)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "export function %s(%s) {\n", jsMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	fields := make([]string, len(outs))
	for i, v := range outs {
		fields[i] = fmt.Sprintf("f%d: %s", i, v)
	}
	fmt.Fprintf(&out, "\treturn {%s};\n}\n", strings.Join(fields, ", "))
	return out.String(), nil
}

func (e *jsEmitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *jsEmitter) fresh(stem string) string {
	for {
		e.tmp++
		n := fmt.Sprintf("%s%d", stem, e.tmp)
		if !e.bound[n] {
			e.bound[n] = true
			return n
		}
	}
}

func (e *jsEmitter) emit(t *core.Term) (string, error) {
	// Tail position is consumed here, so no nested emit inherits it by
	// accident. The two constructs that genuinely preserve it — a `let`
	// continuation and the loop itself — put it back explicitly.
	tail := e.tail
	e.tail = false
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10), nil

	case core.KFloat:
		// JS has one number type, so a float literal needs no decorating —
		// unlike Go, where 1 and 1.0 are different tokens.
		return strconv.FormatFloat(t.Float, 'g', -1, 64), nil

	case core.KBool:
		if t.IsTrue() {
			return "true", nil
		}
		return "false", nil

	case core.KStr:
		// A literal was added to the language for target templates and no
		// backend could emit one, because no program had ever used one
		// (strings.md 1). strconv.Quote escapes non-ASCII to \u, which is
		// valid in all three hosts and sidesteps javac's platform-charset
		// hazard entirely (strings.md 5).
		return strconv.Quote(t.Str), nil

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
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			return "", fmt.Errorf("no JavaScript form for primitive %q", op.Name)
		}
		if p.Kind == "let" {
			args := t.Args()
			k := args[1]
			if k.Kind != core.KFn || len(k.Params) != 1 {
				return "", fmt.Errorf("let's continuation must be (fn (x) …), got %s", k)
			}
			val, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			// A binder used zero times is a sequencing point rather than a
			// binding — see effects.md §5 and the Go backend's emitLet.
			if !core.Occurs(k.Body(), k.Params[0]) {
				if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
					e.line("%s;", val)
				}
				e.tail = tail
				return e.emit(k.Body())
			}
			kBody, _, kOut := openFresh(k, e.bound, jsMangle)
			e.line("const %s = %s;", kOut[0], val)
			e.tail = tail
			return e.emit(kBody)
		}
		if p.Kind == "stmt" {
			args := t.Args()
			vals := make([]any, 0, 2*len(args))
			for _, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals = append(vals, v)
			}
			if len(args) > 0 && !atomicValue(vals[0].(string)) && strings.Contains(p.Form, "%s") {
				name := e.fresh("v")
				e.line("const %s = %s;", name, vals[0])
				vals[0] = name
			}
			e.line("%s;", fill(p.Form, vals))
			return vals[0].(string), nil
		}
		if p.Kind == "build" {
			return e.emitMakeVec(t)
		}
		if p.Kind == "iterate" {
			return e.emitLoop(t, tail)
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
		return "(" + fmt.Sprintf(p.Form, vals...) + ")", nil
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
// emitConnective emits the host's own operator for a conditional that is one
// of the three boolean connectives (booleans.md §4.4).
func (e *jsEmitter) emitConnective(c Connective) (string, error) {
	vals := make([]string, len(c.Args))
	for i, a := range c.Args {
		v, err := e.emit(a)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}
	switch c.Op {
	case "not":
		return "(!" + vals[0] + ")", nil
	case "and":
		return "(" + vals[0] + " && " + vals[1] + ")", nil
	}
	return "(" + vals[0] + " || " + vals[1] + ")", nil
}

func (e *jsEmitter) emitIf(t *core.Term) (string, error) {
	if c, ok := connective(e.tgt, t); ok {
		return e.emitConnective(c)
	}
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
	body, _, out := openFresh(step, e.bound, jsMangle)
	acc, idx := out[0], out[1]
	n := e.fresh("n")

	e.line("let %s = %s;", acc, init)
	e.line("const %s = %s;", n, count)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	got, err := e.emit(body)
	if err != nil {
		return "", err
	}
	if got != acc { // a statement-primitive already updated it in place
		e.line("%s = %s;", acc, got)
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
		// `#` starts a name the READER generates and a program cannot write —
		// `match`'s loop variables, `values`' selector. It becomes `_` rather
		// than an escape so the emitted code is readable; a collision with a
		// user's `_m0` is resolved by openFresh like any other.
		case r == '#':
			b.WriteString("_")
		// `-` word break, `.` qualifier, `/` module path separator: all three
		// become camel-case boundaries so a qualified name is one host identifier.
		case r == '-' || r == '.' || r == '/':
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

func (e *jsEmitter) emitMakeVec(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("make-vec takes a length and an element function")
	}
	elem := args[1]
	if elem.Kind != core.KFn || len(elem.Params) != 1 {
		return "", fmt.Errorf("make-vec's element function must be (fn (i) ...), got %s", elem)
	}
	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	n := e.fresh("n")
	dst := e.fresh("v")
	elemBody, _, eOut := openFresh(elem, e.bound, jsMangle)
	idx := eOut[0]
	e.line("const %s = %s;", n, count)
	e.line("const %s = new Array(%s);", dst, n)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(elemBody)
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s;", dst, idx, body)
	e.indent--
	e.line("}")
	return dst, nil
}

// ---------------------------------------------------------------- loop
//
// (loop (fn (x…) body) z…) — docs/spec/iteration.md. Identical in shape to the
// Go emitter, minus the types, plus temporaries: JavaScript has no parallel
// assignment, so a simultaneous update needs them — the same ones fold-range2
// already emits, and measured free.

func (e *jsEmitter) emitLoop(t *core.Term, tail bool) (string, error) {
	args := t.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return "", fmt.Errorf("loop takes (fn (x…) body) and one initial value per variable")
	}
	lam := args[0]
	inits := args[1:]
	if len(lam.Params) != len(inits) {
		return "", fmt.Errorf("loop has %d variable(s) and %d initial value(s)",
			len(lam.Params), len(inits))
	}
	vals := make([]string, len(inits))
	for i, z := range inits {
		v, err := e.emit(z)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}
	body, raw, names := openFresh(lam, e.bound, jsMangle)
	for i := range names {
		e.line("let %s = %s;", names[i], vals[i])
	}
	result := ""
	if !tail {
		result = soleExit(e.tgt.Prims, body, raw, names, e.bound, jsMangle)
		if result == "" {
			result = e.fresh("r")
			e.line("let %s;", result)
		}
	}
	e.line("for (;;) {")
	e.indent++
	if err := e.emitLoopBody(body, raw, names, result); err != nil {
		return "", err
	}
	e.indent--
	e.line("}")
	if tail {
		e.returned = true
	}
	return result, nil
}

func (e *jsEmitter) emitLoopBody(t *core.Term, raw, names []string, result string) error {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			cond, err := e.emit(t.Args()[0])
			if err != nil {
				return err
			}
			e.line("if (%s) {", cond)
			e.indent++
			if err := e.emitLoopBody(t.Args()[1], raw, names, result); err != nil {
				return err
			}
			e.indent--
			e.line("}")
			return e.emitLoopBody(t.Args()[2], raw, names, result)
		}
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "let" && len(t.Args()) == 2 {
			args := t.Args()
			k := args[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					if !emitsStatement(e.tgt, args[0]) {
						e.line("%s;", val)
					}
					return e.emitLoopBody(k.Body(), raw, names, result)
				}
				kb, _, kout := openFresh(k, e.bound, jsMangle)
				e.line("const %s = %s;", kout[0], val)
				return e.emitLoopBody(kb, raw, names, result)
			}
		}
	}
	if isAgain(t) {
		return e.emitAgain(t, raw, names)
	}
	v, err := e.emit(t)
	if err != nil {
		return err
	}
	// An empty result means this loop is the function's value: return from the
	// loop rather than assigning and breaking. Worth 1.31x on V8 for a loop
	// with two exits, and nothing at all on Go.
	if result == "" {
		e.line("return %s;", v)
		return nil
	}
	if v != result {
		e.line("%s = %s;", result, v)
	}
	e.line("break;")
	return nil
}

// emitAgain: skip unchanged variables, and use temporaries only when a changed
// one is read by another changed one — otherwise sequential assignment is
// already simultaneous.
func (e *jsEmitter) emitAgain(t *core.Term, raw, names []string) error {
	as := t.Args()
	if len(as) != len(names) {
		return fmt.Errorf("again takes %d argument(s), given %d", len(names), len(as))
	}
	changed := changedArgs(as, raw)
	vals := make(map[int]string, len(changed))
	var real []int
	for _, i := range changed {
		v, err := e.emit(as[i])
		if err != nil {
			return err
		}
		if v == names[i] {
			continue // a statement primitive handed the variable back
		}
		vals[i] = v
		real = append(real, i)
	}
	changed = real
	if needTemps(as, raw, changed) {
		tmp := make(map[int]string, len(changed))
		for _, i := range changed {
			tmp[i] = e.fresh("u")
			e.line("const %s = %s;", tmp[i], vals[i])
		}
		for _, i := range changed {
			e.line("%s = %s;", names[i], tmp[i])
		}
	} else {
		for _, i := range changed {
			e.line("%s = %s;", names[i], vals[i])
		}
	}
	e.line("continue;")
	return nil
}
