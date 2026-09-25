package emit

import (
	"fmt"

	"oroboros/core"
)

// A JOIN POINT on x86-64 — golang_join.go says what it is. Here a result is a
// PLACE, a register or a frame slot, one per component the body reads. Each is
// allocated at the first tail that reaches it, where the component's names are
// bound and its register file is known; allocating is bookkeeping and emits
// nothing, so the place is the same one on every path.
//
// A table's element width travels BY NAME on this host (wintables-2026-08-25),
// so the join's name for a table component takes the width of the table it was
// handed, and every tail must hand it the same one: two widths would read a
// table of shorts as qwords, a wrong answer rather than a slow one.

type asmJoin struct {
	used, have []bool
	dst        []place
	elem       []int
}

func (e *asmEmitter) emitJoin(t *core.Term) (place, bool, error) {
	prod, k, ok := tupleElim(e.tgt, t)
	if !ok {
		return place{}, false, nil
	}
	body, raw, _ := openFresh(k, e.bound, asmIdent)
	m := len(raw)
	ctx := &asmJoin{used: make([]bool, m), have: make([]bool, m), dst: make([]place, m), elem: make([]int, m)}
	for j, nm := range raw {
		ctx.used[j] = core.Occurs(body, nm)
	}
	if err := e.emitIntoJoin(prod, ctx); err != nil {
		return place{}, true, err
	}
	for j, nm := range raw {
		if !ctx.have[j] {
			continue
		}
		e.where[nm] = hold(ctx.dst[j])
		if ctx.elem[j] != 8 {
			e.elem[nm] = ctx.elem[j]
		}
	}
	res, err := e.emit(body)
	if err != nil {
		return place{}, true, err
	}
	out := res
	for j, nm := range raw {
		delete(e.where, nm)
		if !ctx.have[j] {
			continue
		}
		if res.text == ctx.dst[j].text {
			out = ctx.dst[j] // the body IS this component: ownership passes on, as in emitLet
			continue
		}
		e.release(ctx.dst[j])
	}
	return out, true, nil
}

// emitIntoJoin emits a producer whose tails fill the join's places.
func (e *asmEmitter) emitIntoJoin(t *core.Term, ctx *asmJoin) error {
	if m, ok := tupleArity(t); ok && m == len(ctx.used) {
		for j, c := range t.OpenWith([]*core.Term{core.Name("#k")}).Args() {
			if !ctx.used[j] {
				v, err := e.emit(c)
				if err != nil {
					return err
				}
				e.release(v)
				continue
			}
			w := 8
			if c.Kind == core.KName {
				if n, ok := e.elem[c.Name]; ok {
					w = n
				}
			}
			if !ctx.have[j] {
				ctx.dst[j], ctx.have[j], ctx.elem[j] = e.alloc(e.isFloat(c)), true, w
			} else if ctx.elem[j] != w {
				return fmt.Errorf("component %d of a tuple is a table of %d-byte elements on one path "+
					"and %d-byte on another; on this target a table's width travels with its name, "+
					"so both must be one", j+1, ctx.elem[j], w)
			}
			if err := e.assign(ctx.dst[j], c); err != nil {
				return err
			}
		}
		return nil
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, known := e.tgt.Prims[t.Op().Name]; known {
			args := t.Args()
			switch {
			case p.Kind == "cond" && len(args) == 3:
				u := e.uniq()
				els, end := fmt.Sprintf("Lelse%d", u), fmt.Sprintf("Lend%d", u)
				if err := e.branchUnless(args[0], els); err != nil {
					return err
				}
				if err := e.emitIntoJoin(args[1], ctx); err != nil {
					return err
				}
				e.line("jmp %s", end)
				e.label(els)
				if err := e.emitIntoJoin(args[2], ctx); err != nil {
					return err
				}
				e.label(end)
				return nil
			case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn && len(args[1].Params) == 1:
				k := args[1]
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					e.release(val)
					return e.emitIntoJoin(k.Body(), ctx)
				}
				kbody, kraw, _ := openFresh(k, e.bound, asmIdent)
				e.carryElem(kraw[0], args[0])
				q := val
				if !val.owned && !val.imm {
					q = e.alloc(val.xmm)
					e.move(q, val)
				}
				e.where[kraw[0]] = hold(q)
				err = e.emitIntoJoin(kbody, ctx)
				delete(e.where, kraw[0])
				e.release(q)
				return err
			case p.Kind == "table-build":
				_, err := e.emitBuildWith(t, func(body *core.Term) (place, error) {
					return place{}, e.emitIntoJoin(body, ctx)
				})
				return err
			case p.Kind == "iterate":
				_, err := e.emitLoopCtx(t, ctx)
				return err
			}
		}
	}
	return fmt.Errorf("a tuple's producer has a tail that is not a tuple: %s", t)
}
