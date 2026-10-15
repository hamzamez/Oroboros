package j

import "oroboros/core"

// WHICH NAMES HOLD DATA (spec/j.md §2.7): J marks indexing, `t[i]`, apart from
// application, `f(x)` (ADR 0050 item 6), and today's notation writes both
// `(t i)`. D reads an index as an application (core/desugar.go), so the choice
// never changes a D-image; it decides only what a reader of J sees. The printer
// decides it from evidence in the file, and counts what it cannot decide:
//   - a parameter whose claim is a table, a map or a buffer holds data;
//   - a name a definition hands to `len`, `keys` or `insert`, stores into, or
//     copies a table from, holds data (prescan);
//   - a binder whose value is a table former or a data name holds data;
//   - a definition's parameter holds data, or a function, when a call in the
//     file passes it one: the least fixpoint over the file's calls.

type evid int8

const (
	unknown evid = iota
	isData
	isFunc
)

// analyze computes, for each λ definition of the file, what each parameter
// receives at the file's own calls.
func (p *printer) analyze(ds []core.Decl) {
	p.argKinds = map[string][]evid{}
	lams := map[string]*core.Lam{}
	for _, d := range ds {
		if def, ok := d.(*core.DefDecl); ok {
			if l, ok := def.Value.(*core.Lam); ok {
				lams[def.Name] = l
				p.argKinds[def.Name] = make([]evid, len(l.Params))
			}
		}
	}
	p.returnsData = map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, l := range lams {
			data, fns := p.evidence(name, l)
			if !p.returnsData[name] && p.producesData(value(l.Body), data) {
				p.returnsData[name], changed = true, true
			}
			visit(l.Body, func(e core.Expr) {
				a, ok := e.(*core.Apply)
				if !ok {
					return
				}
				f, ok := a.Fn.(*core.Var)
				if !ok || lams[f.Name] == nil {
					return
				}
				ks := p.argKinds[f.Name]
				for i, arg := range a.Args {
					if i >= len(ks) || ks[i] != unknown {
						continue
					}
					switch arg := arg.(type) {
					case *core.Lam:
						ks[i], changed = isFunc, true
					case *core.Var:
						switch {
						case data[arg.Name]:
							ks[i], changed = isData, true
						case fns[arg.Name] || lams[arg.Name] != nil:
							ks[i], changed = isFunc, true
						}
					case *core.Apply:
						if p.producesData(arg, data) {
							ks[i], changed = isData, true
						}
					}
				}
			})
		}
	}
}

// evidence is what a definition's body shows of its names: the data and the
// functions among its parameters and binders.
func (p *printer) evidence(name string, l *core.Lam) (data, fns map[string]bool) {
	data, fns = map[string]bool{}, map[string]bool{}
	for _, n := range l.Borrows {
		data[n] = true // a borrowed parameter is a place, a buffer (ADR 0052)
	}
	for i, n := range l.Params {
		switch {
		case p.argKinds[name][i] == isData || claimsData(l.Claim, i):
			data[n] = true
		case p.argKinds[name][i] == isFunc:
			fns[n] = true
		}
	}
	saved := p.data
	p.data = data
	p.prescan(l)
	p.data = saved
	visit(l.Body, func(e core.Expr) {
		switch e := e.(type) {
		case *core.Let:
			for _, b := range e.Binds {
				if v, ok := b.Pat.(*core.PVar); ok {
					if p.producesData(b.Value, data) {
						data[v.Name] = true
					}
					if _, ok := b.Value.(*core.Lam); ok {
						fns[v.Name] = true
					}
				}
			}
		case *core.Local:
			for _, b := range e.Binds {
				if b.Buffer || b.Place {
					data[b.Name] = true
				}
			}
		case *core.BuildMap:
			for _, n := range e.Names {
				data[n] = true
			}
		case *core.Loop:
			for i, v := range e.Vars {
				if p.producesData(e.Inits[i], data) {
					data[v] = true
				}
			}
		}
	})
	return data, fns
}

// claimsData reports whether a claim gives parameter i a table, a map or a
// buffer.
func claimsData(c *core.Claim, i int) bool {
	if c == nil || i >= len(c.Params) {
		return false
	}
	a, ok := c.Params[i].Set.(*core.Apply)
	if !ok {
		return false
	}
	v, ok := a.Fn.(*core.Var)
	return ok && (v.Name == "array" || v.Name == "map" || v.Name == "buffer")
}

// value is the expression a term's value is: past its binders and statements.
func value(t core.Tail) core.Tail {
	for {
		switch e := t.(type) {
		case *core.Let:
			t = e.Body
		case *core.TLet:
			t = e.Body
		case *core.Seq:
			t = e.Xs[len(e.Xs)-1]
		case *core.TSeq:
			t = e.Last
		default:
			return t
		}
	}
}

// producesData reports whether a value is a table or a map: a data name, a table
// or map former, a store, a scope of local state (its value is its frozen
// buffer), or a call of a definition of the file that returns one.
func (p *printer) producesData(t core.Tail, data map[string]bool) bool {
	switch e := t.(type) {
	case *core.Local, *core.BuildMap:
		return true
	case *core.Apply:
		if v, ok := e.Fn.(*core.Var); ok && p.returnsData[v.Name] {
			return true
		}
	}
	if e, ok := t.(core.Expr); ok {
		return producesData(e, data)
	}
	return false
}
