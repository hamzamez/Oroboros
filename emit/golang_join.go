package emit

import (
	"fmt"
	"maps"
	"strings"

	"oroboros/core"
)

// A JOIN POINT — tables.md §2.5, ADR 0031 §4, iteration.md §2.
//
// `(P (fn (x₁ … xₘ) body))` with P a `build` or a `loop` whose tails are
// m-tuples (prodresult.go). The eliminator cannot be pushed into P's exits
// without copying `body` into every one of them, so it is bound ONCE instead:
//
//	var x1 T1; var x2 T2           the eliminator's own names, one per component
//	nodes := make([]int16, 2048)   P, emitted as statements; each tail assigns
//	for { … x1, x2 = nodes, nn; break … }
//	…body…                         once, after P, reading x1 and x2
//
// Case-of-case on the exits with the continuation shared, which is what a join
// point is (Maurer, Downen, Ariola & Peyton Jones, PLDI 2017). The result
// variables ARE the eliminator's parameters, so nothing is copied into them
// twice; a component the body never reads is not declared, since Go refuses an
// unused local, and is discarded the way a `let` with an unused binder is.

// joinCtx is one join point being emitted: where each component goes ("" for
// one the body never reads), and each component's type, read off the first
// tail that is reached.
type joinCtx struct {
	dests []string
	tys   []string
	seen  bool
	// narrow: a component the JVM holds in its own `int` (java_join.go).
	narrow []bool
}

// emitJoin emits the join point and returns the body's value.
func (e *Emitter) emitJoin(t *core.Term) (string, bool, error) {
	prod, k, ok := tupleElim(e.tgt, t)
	if !ok {
		return "", false, nil
	}
	body, raw, out := openFresh(k, e.bound, mangle)
	ctx := &joinCtx{dests: make([]string, len(raw)), tys: make([]string, len(raw))}
	for j, nm := range raw {
		if core.Occurs(body, nm) {
			ctx.dests[j] = out[j]
		}
	}
	// THE DECLARATIONS PRECEDE THE PRODUCER, and their types are known only once
	// its first tail has been reached, where its variables are typed. So the
	// producer is emitted first, and its text is put after the declarations.
	pre := e.buf.String()
	e.buf.Reset()
	if err := e.emitInto(prod, ctx); err != nil {
		return "", true, err
	}
	produced := e.buf.String()
	e.buf.Reset()
	e.buf.WriteString(pre)
	for j, d := range ctx.dests {
		if d != "" {
			e.line("var %s %s", d, e.tgt.ty(orAny(ctx.tys[j])))
		}
	}
	e.buf.WriteString(produced)
	for j, nm := range raw {
		e.types[nm] = ctx.tys[j]
	}
	s, err := e.emit(body)
	return s, true, err
}

// emitInto emits a producer whose tails assign the join's variables. The forms
// are the ones projectTail walks: a tuple, `if`, `let`, `build`, a host call's
// continuation, and a loop, whose exits are tails.
func (e *Emitter) emitInto(t *core.Term, ctx *joinCtx) error {
	if m, ok := tupleArity(t); ok && m == len(ctx.dests) {
		comps := t.OpenWith([]*core.Term{core.Name("#k")}).Args()
		var lhs, rhs []string
		for j, c := range comps {
			if !ctx.seen {
				ctx.tys[j] = e.typeOf(c)
			}
			before := maps.Clone(e.bound)
			v, err := e.emit(c)
			if err != nil {
				return err
			}
			if ctx.dests[j] == "" {
				if !emitsStatement(e.tgt, c) && !(atomicValue(v) && !declaredBy(before, e.bound, v)) {
					e.line("_ = %s", v)
				}
				continue
			}
			lhs, rhs = append(lhs, ctx.dests[j]), append(rhs, v)
		}
		ctx.seen = true
		if len(lhs) > 0 {
			e.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
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
				e.line("if %s {", c)
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
				before := maps.Clone(e.bound)
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					if !emitsStatement(e.tgt, args[0]) && !(atomicValue(val) && !declaredBy(before, e.bound, val)) {
						e.line("_ = %s", val)
					}
					return e.emitInto(k.Body(), ctx)
				}
				ty := e.typeOf(args[0])
				body, raw, out := openFresh(k, e.bound, mangle)
				e.types[raw[0]] = ty
				if ty == "int" {
					e.line("var %s %s = %s", out[0], e.tgt.ty(ty), val)
				} else {
					e.line("%s := %s", out[0], val)
				}
				return e.emitInto(body, ctx)
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

// tailTypes are a producer's component types, read at its first tail that is a
// tuple, as exitType reads a loop's type at its first exit. They are what the
// join's names are typed with where the join is not being emitted: the
// enclosing function's result type is asked for before its body is.
func (e *Emitter) tailTypes(t *core.Term, m int) []string {
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
	if p, _, k, ok := multiPrimCall(e.tgt, t); ok {
		body, raw, _ := openFresh(k, map[string]bool{}, func(s string) string { return s })
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
	id := func(s string) string { return s }
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
