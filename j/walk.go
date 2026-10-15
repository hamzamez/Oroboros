package j

import "oroboros/core"

// visit calls f on every expression of a tree, outermost first.
func visit(t core.Tail, f func(core.Expr)) {
	var tail func(t core.Tail)
	var expr func(e core.Expr)
	exprs := func(es []core.Expr) {
		for _, e := range es {
			if e != nil {
				expr(e)
			}
		}
	}
	binds := func(bs []core.LetBind) {
		for _, b := range bs {
			expr(b.Value)
		}
	}
	expr = func(e core.Expr) {
		if e == nil {
			return
		}
		f(e)
		switch e := e.(type) {
		case *core.Lam:
			expr(e.Body)
		case *core.Apply:
			expr(e.Fn)
			exprs(e.Args)
		case *core.Index:
			expr(e.Table)
			expr(e.At)
		case *core.Let:
			binds(e.Binds)
			expr(e.Body)
		case *core.Case:
			expr(e.Scrut)
			for _, a := range e.Arms {
				expr(a.Pat)
				expr(a.Body)
			}
		case *core.Loop:
			exprs(e.Inits)
			for _, a := range e.Arms {
				if a.Guard != nil {
					expr(a.Guard)
				}
				if a.When != nil {
					expr(a.When)
				}
				tail(a.Body)
			}
		case *core.Local:
			for _, b := range e.Binds {
				expr(b.Init)
			}
			expr(e.Body)
		case *core.BuildMap:
			exprs(e.Caps)
			expr(e.Body)
		case *core.The:
			expr(e.Set)
			expr(e.E)
		case *core.If:
			expr(e.C)
			expr(e.A)
			expr(e.B)
		case *core.And:
			exprs(e.Xs)
		case *core.Or:
			exprs(e.Xs)
		case *core.Not:
			expr(e.X)
		case *core.Cond:
			for _, a := range e.Arms {
				if a.Guard != nil {
					expr(a.Guard)
				}
				expr(a.Body)
			}
		case *core.Seq:
			exprs(e.Xs)
		case *core.Monad:
			exprs(e.Items)
			expr(e.Body)
		}
	}
	tail = func(t core.Tail) {
		switch t := t.(type) {
		case *core.Again:
			exprs(t.Args)
		case *core.TLet:
			binds(t.Binds)
			tail(t.Body)
		case *core.TSeq:
			exprs(t.Xs)
			tail(t.Last)
		case *core.TMonad:
			exprs(t.Items)
			tail(t.Body)
		case core.Expr:
			expr(t)
		}
	}
	tail(t)
}

// renameExpr and renameTail rename free occurrences of names, as D's renameFree
// does for a match arm (core/desugar.go): a binder that rebinds a name stops it.
func renameExpr(e core.Expr, m map[string]string) core.Expr {
	if len(m) == 0 || e == nil {
		return e
	}
	return rename(e, m).(core.Expr)
}

func renameTail(t core.Tail, m map[string]string) core.Tail {
	if len(m) == 0 {
		return t
	}
	return rename(t, m)
}

func without(m map[string]string, names ...string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for _, n := range names {
		delete(out, n)
	}
	return out
}

func rename(t core.Tail, m map[string]string) core.Tail {
	re := func(e core.Expr) core.Expr { return renameExpr(e, m) }
	res := func(es []core.Expr) []core.Expr {
		out := make([]core.Expr, len(es))
		for i, e := range es {
			out[i] = renameExpr(e, m)
		}
		return out
	}
	switch t := t.(type) {
	case *core.Var:
		if n, ok := m[t.Name]; ok {
			return &core.Var{Name: n}
		}
		return t
	case *core.Lam:
		return &core.Lam{Params: t.Params, Body: renameExpr(t.Body, without(m, t.Params...)), Claim: t.Claim}
	case *core.Apply:
		return &core.Apply{Fn: re(t.Fn), Args: res(t.Args)}
	case *core.Index:
		return &core.Index{Table: re(t.Table), At: re(t.At)}
	case *core.If:
		return &core.If{C: re(t.C), A: re(t.A), B: re(t.B)}
	case *core.And:
		return &core.And{Xs: res(t.Xs)}
	case *core.Or:
		return &core.Or{Xs: res(t.Xs)}
	case *core.Not:
		return &core.Not{X: re(t.X)}
	case *core.Seq:
		return &core.Seq{Xs: res(t.Xs)}
	case *core.Again:
		return &core.Again{Args: res(t.Args)}
	case *core.Let:
		binds, inner := renameBinds(t.Binds, m)
		return &core.Let{Binds: binds, Body: renameExpr(t.Body, inner)}
	case *core.TLet:
		binds, inner := renameBinds(t.Binds, m)
		return &core.TLet{Binds: binds, Body: renameTail(t.Body, inner)}
	case *core.TSeq:
		return &core.TSeq{Xs: res(t.Xs), Last: renameTail(t.Last, m)}
	case *core.Cond:
		arms := make([]core.CondArm, len(t.Arms))
		for i, a := range t.Arms {
			arms[i] = core.CondArm{Guard: renameExpr(a.Guard, m), Body: re(a.Body)}
		}
		return &core.Cond{Arms: arms}
	}
	// A form with binders of its own (case, loop, local, monad) is returned as
	// it is: no match arm in the corpus renames into one, and a rename that
	// stopped short would be refused by the round trip of step 3, not hidden.
	return t
}

func renameBinds(bs []core.LetBind, m map[string]string) ([]core.LetBind, map[string]string) {
	out := make([]core.LetBind, len(bs))
	for i, b := range bs {
		out[i] = core.LetBind{Pat: b.Pat, Value: renameExpr(b.Value, m)}
		switch p := b.Pat.(type) {
		case *core.PVar:
			m = without(m, p.Name)
		case *core.PTuple:
			m = without(m, p.Names...)
		}
	}
	return out, m
}
