package emit

import (
	"fmt"

	"oroboros/core"
)

// A JOIN POINT on JavaScript — golang_join.go says what it is. The result
// variables are the eliminator's own names, declared with `let` before the
// producer; JavaScript needs no type for them. The components are assigned one
// at a time, which is simultaneous here: a component is a term of the producer's
// scope and cannot read a name the eliminator binds.

func (e *jsEmitter) emitJoin(t *core.Term) (string, bool, error) {
	prod, k, ok := tupleElim(e.tgt, t)
	if !ok {
		return "", false, nil
	}
	body, raw, out := openFresh(k, e.bound, jsMangle)
	ctx := &joinCtx{dests: make([]string, len(raw)), tys: make([]string, len(raw))}
	var decl []string
	for j, nm := range raw {
		if core.Occurs(body, nm) {
			ctx.dests[j] = out[j]
			decl = append(decl, out[j])
		}
	}
	if len(decl) > 0 {
		e.line("let %s;", joinComma(decl))
	}
	if err := e.emitInto(prod, ctx); err != nil {
		return "", true, err
	}
	s, err := e.emit(body)
	return s, true, err
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

func (e *jsEmitter) emitInto(t *core.Term, ctx *joinCtx) error {
	if m, ok := tupleArity(t); ok && m == len(ctx.dests) {
		for j, c := range t.OpenWith([]*core.Term{core.Name("#k")}).Args() {
			v, err := e.emit(c)
			if err != nil {
				return err
			}
			if ctx.dests[j] == "" {
				if !emitsStatement(e.tgt, c) && !atomicValue(v) {
					e.line("%s;", v)
				}
				continue
			}
			e.line("%s = %s;", ctx.dests[j], v)
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
					if !emitsStatement(e.tgt, args[0]) {
						e.line("%s;", val)
					}
					return e.emitInto(k.Body(), ctx)
				}
				kb, _, kout := openFresh(k, e.bound, jsMangle)
				e.line("const %s = %s;", kout[0], val)
				return e.emitInto(kb, ctx)
			case p.Kind == "table-build":
				body, err := e.emitBuildHead(t)
				if err != nil {
					return err
				}
				return e.emitInto(body, ctx)
			case p.Kind == "iterate":
				_, err := e.emitLoopCtx(t, false, ctx)
				return err
			}
		}
	}
	return fmt.Errorf("a tuple's producer has a tail that is not a tuple: %s", t)
}
