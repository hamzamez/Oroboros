package emit

import (
	"fmt"
	"strings"

	"oroboros/core"
)

// A JOIN POINT on the JVM — golang_join.go says what it is. Two things differ
// from Go. A local needs a TYPE and javac requires it DEFINITELY ASSIGNED before
// a read, which it cannot see through `for (;;)` and its breaks, so each result
// variable is declared with its zero (zeroOf), as a loop's own result is. And an
// integer result follows the method's narrowing (indexnarrow-2026-08-27 §2): when
// every integer operation in the method is proven inside 32 bits (fitsIdx), the
// method's integers are the host's `int`, and a component is declared one and
// assigned through a cast — exact, since its value is proven to fit, and needed,
// since the loop variable it comes from may have been kept wide. Otherwise it
// takes the host's full width, which every component widens into.

func (e *javaEmitter) emitJoin(t *core.Term) (string, bool, error) {
	prod, k, ok := tupleElim(e.tgt, t)
	if !ok {
		return "", false, nil
	}
	body, raw, out := openFresh(k, e.bound, javaMangle)
	ctx := &joinCtx{dests: make([]string, len(raw)), tys: make([]string, len(raw)),
		narrow: make([]bool, len(raw))}
	for j, nm := range raw {
		if core.Occurs(body, nm) {
			ctx.dests[j] = out[j]
		}
	}
	// The declarations precede the producer, and their types are read at its
	// first tail: the producer is emitted first and put after them.
	pre := e.buf.String()
	e.buf.Reset()
	if err := e.emitInto(prod, ctx); err != nil {
		return "", true, err
	}
	produced := e.buf.String()
	e.buf.Reset()
	e.buf.WriteString(pre)
	for j, d := range ctx.dests {
		if d == "" {
			continue
		}
		rt := e.tgt.ty(orAny(ctx.tys[j]))
		if ctx.narrow[j] {
			rt = "int"
		}
		e.line("%s %s = %s;", rt, d, zeroOf(rt))
	}
	e.buf.WriteString(produced)
	for j, nm := range raw {
		e.types[nm] = ctx.tys[j]
		if ctx.narrow[j] {
			if e.narrow == nil {
				e.narrow = map[string]bool{}
			}
			e.narrow[nm] = true
		}
	}
	s, err := e.emit(body)
	return s, true, err
}

func (e *javaEmitter) emitInto(t *core.Term, ctx *joinCtx) error {
	if m, ok := tupleArity(t); ok && m == len(ctx.dests) {
		comps := t.OpenWith([]*core.Term{core.Name("#k")}).Args()
		var assigns []string
		for j, c := range comps {
			if !ctx.seen {
				ctx.tys[j] = e.typeOf(c)
				ctx.narrow[j] = ctx.tys[j] == "int" && e.fitsIdx
			}
			v, err := e.emit(c)
			if err != nil {
				return err
			}
			if ctx.dests[j] == "" {
				if !emitsStatement(e.tgt, c) && !atomicValue(v) {
					e.line("final var %s = %s;", javaMangle(e.fresh("discard")), v)
				}
				continue
			}
			if ctx.narrow[j] {
				v = "(int) (" + v + ")"
			}
			assigns = append(assigns, fmt.Sprintf("%s = %s;", ctx.dests[j], v))
		}
		ctx.seen = true
		if len(assigns) > 0 {
			e.line("%s", strings.Join(assigns, " "))
		}
		return nil
	}
	if body, ok, err := e.emitMultiPrimHead(t); ok {
		if err != nil {
			return err
		}
		return e.emitInto(body, ctx)
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, known := e.tgt.Prims[t.Op().Name]; known {
			args := t.Args()
			switch {
			case p.Kind == "cond" && len(args) == 3:
				c, err := e.emit(args[0])
				if err != nil {
					return err
				}
				e.line("if (%s) {", c)
				e.indent++
				if err := e.emitInto(args[1], ctx); err != nil {
					return err
				}
				e.indent--
				e.line("} else {")
				e.indent++
				if err := e.emitInto(args[2], ctx); err != nil {
					return err
				}
				e.indent--
				e.line("}")
				return nil
			case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
				k := args[1]
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
						e.line("final var %s = %s;", javaMangle(e.fresh("discard")), val)
					}
					return e.emitInto(k.Body(), ctx)
				}
				ty := e.typeOf(args[0])
				kb, kraw, kout := openFresh(k, e.bound, javaMangle)
				e.types[kraw[0]] = ty
				e.line("final %s %s = %s;", e.tgt.ty(ty), kout[0], val)
				return e.emitInto(kb, ctx)
			case p.Kind == "table-build":
				body, err := e.emitBuildHead(t)
				if err != nil {
					return err
				}
				return e.emitInto(body, ctx)
			case p.Kind == "iterate":
				_, err := e.emitLoopCtx(t, ctx)
				return err
			}
		}
	}
	return fmt.Errorf("a tuple's producer has a tail that is not a tuple: %s", t)
}

// tailTypes: golang_join.go's, on this emitter's types.
func (e *javaEmitter) tailTypes(t *core.Term, m int) []string {
	if n, ok := tupleArity(t); ok {
		if n != m {
			return nil
		}
		out := make([]string, m)
		for j, c := range t.OpenWith([]*core.Term{core.Name("#k")}).Args() {
			out[j] = e.typeOf(c)
		}
		return out
	}
	id := func(s string) string { return s }
	if p, _, k, ok := multiPrimCall(e.tgt, t); ok {
		body, raw, _ := openFresh(k, map[string]bool{}, id)
		for i := range raw {
			e.types[raw[i]] = e.tgt.ValueType(p.Results[i])
		}
		return e.tailTypes(body, m)
	}
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return nil
	}
	p, known := e.tgt.Prims[t.Op().Name]
	if !known {
		return nil
	}
	args := t.Args()
	switch {
	case p.Kind == "cond" && len(args) == 3:
		if tys := e.tailTypes(args[1], m); tys != nil {
			return tys
		}
		return e.tailTypes(args[2], m)
	case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
		body, raw, _ := openFresh(args[1], map[string]bool{}, id)
		e.types[raw[0]] = e.typeOf(args[0])
		return e.tailTypes(body, m)
	case p.Kind == "table-build" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
		body, raw, _ := openFresh(args[1], map[string]bool{}, id)
		e.types[raw[0]] = e.buildType(args[1])
		return e.tailTypes(body, m)
	case p.Kind == "iterate" && len(args) >= 2 && args[0].Kind == core.KFn:
		body, raw, _ := openFresh(args[0], map[string]bool{}, id)
		for i, z := range args[1:] {
			if i < len(raw) {
				e.types[raw[i]] = e.typeOf(z)
			}
		}
		return e.tailTypes(body, m)
	}
	return nil
}
