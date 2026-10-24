package core

// visit calls f on every expression of a tree, outermost first.
func visit(t Tail, f func(Expr)) {
	var tail func(t Tail)
	var expr func(e Expr)
	exprs := func(es []Expr) {
		for _, e := range es {
			if e != nil {
				expr(e)
			}
		}
	}
	binds := func(bs []LetBind) {
		for _, b := range bs {
			expr(b.Value)
		}
	}
	expr = func(e Expr) {
		if e == nil {
			return
		}
		f(e)
		switch e := e.(type) {
		case *Lam:
			expr(e.Body)
		case *Apply:
			expr(e.Fn)
			exprs(e.Args)
		case *Index:
			expr(e.Table)
			expr(e.At)
		case *Let:
			binds(e.Binds)
			expr(e.Body)
		case *Case:
			expr(e.Scrut)
			for _, a := range e.Arms {
				expr(a.Pat)
				expr(a.Body)
			}
		case *Loop:
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
		case *Local:
			for _, b := range e.Binds {
				expr(b.Init)
			}
			expr(e.Body)
		case *BuildMap:
			exprs(e.Caps)
			expr(e.Body)
		case *The:
			expr(e.Set)
			expr(e.E)
		case *If:
			expr(e.C)
			expr(e.A)
			expr(e.B)
		case *And:
			exprs(e.Xs)
		case *Or:
			exprs(e.Xs)
		case *Not:
			expr(e.X)
		case *Cond:
			for _, a := range e.Arms {
				if a.Guard != nil {
					expr(a.Guard)
				}
				expr(a.Body)
			}
		case *Seq:
			exprs(e.Xs)
		case *Monad:
			exprs(e.Items)
			expr(e.Body)
		case *Write:
			exprs(e.Values)
			for _, pl := range e.Places {
				exprs(pl.At)
			}
		}
	}
	tail = func(t Tail) {
		switch t := t.(type) {
		case *Again:
			exprs(t.Args)
		case *TLet:
			binds(t.Binds)
			tail(t.Body)
		case *TSeq:
			exprs(t.Xs)
			tail(t.Last)
		case *TMonad:
			exprs(t.Items)
			tail(t.Body)
		case *TCase:
			expr(t.Scrut)
			for _, a := range t.Arms {
				expr(a.Pat)
				tail(a.Body)
			}
		case Expr:
			expr(t)
		}
	}
	tail(t)
}

// renameExpr and renameTail rename free occurrences of names, as D's renameFree
// does for a match arm (core/desugar.go): a binder that rebinds a name stops it.
func renameExpr(e Expr, m map[string]string) Expr {
	if len(m) == 0 || e == nil {
		return e
	}
	return rename(e, m).(Expr)
}

func renameTail(t Tail, m map[string]string) Tail {
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

func rename(t Tail, m map[string]string) Tail {
	re := func(e Expr) Expr { return renameExpr(e, m) }
	res := func(es []Expr) []Expr {
		out := make([]Expr, len(es))
		for i, e := range es {
			out[i] = renameExpr(e, m)
		}
		return out
	}
	switch t := t.(type) {
	case *Var:
		if n, ok := m[t.Name]; ok {
			return &Var{Name: n}
		}
		return t
	case *Lam:
		return &Lam{Params: t.Params, Body: renameExpr(t.Body, without(m, t.Params...)), Claim: t.Claim}
	case *Apply:
		return &Apply{Fn: re(t.Fn), Args: res(t.Args)}
	case *Index:
		return &Index{Table: re(t.Table), At: re(t.At)}
	case *If:
		return &If{C: re(t.C), A: re(t.A), B: re(t.B)}
	case *And:
		return &And{Xs: res(t.Xs)}
	case *Or:
		return &Or{Xs: res(t.Xs)}
	case *Not:
		return &Not{X: re(t.X)}
	case *Seq:
		return &Seq{Xs: res(t.Xs)}
	case *Again:
		return &Again{Args: res(t.Args)}
	case *Write:
		places := make([]Place, len(t.Places))
		for i, pl := range t.Places {
			name := pl.Name
			if n, ok := m[name]; ok {
				name = n
			}
			places[i] = Place{Name: name, At: res(pl.At)}
		}
		return &Write{Places: places, Values: res(t.Values)}
	case *Let:
		binds, inner := renameBinds(t.Binds, m)
		return &Let{Binds: binds, Body: renameExpr(t.Body, inner)}
	case *TLet:
		binds, inner := renameBinds(t.Binds, m)
		return &TLet{Binds: binds, Body: renameTail(t.Body, inner)}
	case *TSeq:
		return &TSeq{Xs: res(t.Xs), Last: renameTail(t.Last, m)}
	case *Cond:
		arms := make([]CondArm, len(t.Arms))
		for i, a := range t.Arms {
			arms[i] = CondArm{Guard: renameExpr(a.Guard, m), Body: re(a.Body)}
		}
		return &Cond{Arms: arms}
	}
	// A form with binders of its own (case, loop, local, monad) is returned as
	// it is: no match arm in the corpus renames into one, and a rename that
	// stopped short would be refused by the round trip of step 3, not hidden.
	return t
}

func renameBinds(bs []LetBind, m map[string]string) ([]LetBind, map[string]string) {
	out := make([]LetBind, len(bs))
	for i, b := range bs {
		out[i] = LetBind{Pat: b.Pat, Value: renameExpr(b.Value, m)}
		switch p := b.Pat.(type) {
		case *PVar:
			m = without(m, p.Name)
		case *PTuple:
			m = without(m, p.Names...)
		}
	}
	return out, m
}
