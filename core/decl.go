package core

import (
	"fmt"
	"sort"
)

// DECLARATIONS AS TREES — docs/spec/abstract.md §2.6, ADR 0051 (step 1b).
//
// J's reader (jparse.go) reads each top-level declaration into a Decl. A
// definition is a named λ, and a λ carries what it claims (ADR 0051, D1); a
// claim read apart from its definition becomes its Claim (carryClaims). D gives
// the forms back in the order they were written, so every stage after the
// reader reads what it always has.

// carryClaims makes each claim its definition's: a `sig` and the one definition
// of its name, whose value is a λ of the claim's arity. Anything else — no
// definition, two of either, a constant, an arity that disagrees — stays a
// ClaimDecl, so that Load decides it as it always has.
func carryClaims(decls []Decl) []Decl {
	defs := map[string][]*DefDecl{}
	claims := map[string][]*ClaimDecl{}
	for _, d := range decls {
		switch d := d.(type) {
		case *DefDecl:
			defs[d.Name] = append(defs[d.Name], d)
		case *ClaimDecl:
			claims[d.Name] = append(claims[d.Name], d)
		}
	}
	var out []Decl
	for _, d := range decls {
		if c, ok := d.(*ClaimDecl); ok && len(claims[c.Name]) == 1 && len(defs[c.Name]) == 1 {
			if l, ok := defs[c.Name][0].Value.(*Lam); ok && l.Claim == nil && len(l.Params) == len(c.Claim.Params) {
				l.Claim = c.Claim
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

// forms is D of a module's declarations: today's forms, in the order written.
// Each is the declaration term D makes of its tree, read by toForm, today's
// reader of declaration terms.
func (d *desugarer) forms(decls []Decl) ([]Form, error) {
	terms, _ := d.placed(decls)
	out := make([]Form, len(terms))
	for i, t := range terms {
		f, err := toForm(t)
		if err != nil {
			return nil, err
		}
		out[i] = f
	}
	return out, nil
}

// placed is D of each declaration, a term, in the order written.
func (d *desugarer) placed(decls []Decl) ([]*Term, error) {
	type placed struct {
		at int
		t  *Term
	}
	var ts []placed
	for _, dc := range decls {
		switch dc := dc.(type) {
		case *DefDecl:
			ts = append(ts, placed{dc.At, &Term{Kind: KApp, Kids: []*Term{Name("def"), Name(dc.Name), d.term(dc.Value)}}})
			if l, ok := dc.Value.(*Lam); ok && l.Claim != nil {
				ts = append(ts, placed{l.Claim.At, d.sig(dc.Name, l.Claim)})
			}
		case *ClaimDecl:
			ts = append(ts, placed{dc.Claim.At, d.sig(dc.Name, dc.Claim)})
		case *VariantDecl:
			ts = append(ts, placed{dc.At, d.variant(dc)})
		case *ModuleDecl:
			ts = append(ts, placed{dc.At, &Term{Kind: KApp, Kids: []*Term{Name("module"), Name(dc.Path)}}})
		case *UseDecl:
			u := &Term{Kind: KApp, Kids: []*Term{Name("use"), Name(dc.Path)}}
			if dc.As != "" {
				u.Kids = append(u.Kids, Name("as"), Name(dc.As))
			}
			ts = append(ts, placed{dc.At, u})
		case *ExportDecl:
			e := &Term{Kind: KApp, Kids: []*Term{Name("export")}}
			for _, n := range dc.Names {
				e.Kids = append(e.Kids, Name(n))
			}
			ts = append(ts, placed{dc.At, e})
		case *TermDecl:
			ts = append(ts, placed{dc.At, d.term(dc.E)})
		case *ForeignDecl:
			ts = append(ts, placed{dc.At, d.term(dc.E)})
		}
	}
	if len(ts) == 0 {
		return nil, nil
	}
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].at < ts[j].at })
	out := make([]*Term, len(ts))
	for i, p := range ts {
		out[i] = p.t
	}
	return out, nil
}

// sig is D of a claim: today's `sig`, which binds its own names. A binder or a
// result the header states no set for claims nothing (ADR 0062 item 8), which
// the core spells `any`, the absence of a type (types.md §3.3): a header that
// claimed one parameter and left another, or left its result, reached D with no
// set and crashed it (shape-2026-10-21).
func (d *desugarer) sig(name string, c *Claim) *Term {
	set := func(e Expr) *Term {
		if e == nil {
			return Name("any")
		}
		return d.term(e)
	}
	params := &Term{Kind: KApp}
	for _, p := range c.Params {
		if p.Name == "" {
			params.Kids = append(params.Kids, set(p.Set))
			continue
		}
		params.Kids = append(params.Kids, &Term{Kind: KApp, Kids: []*Term{Name(p.Name), set(p.Set)}})
	}
	t := &Term{Kind: KApp, Kids: []*Term{Name("sig"), Name(name), params, set(c.Result)}}
	if c.Pre != nil {
		t.Kids = append(t.Kids, &Term{Kind: KApp, Kids: []*Term{Name("where"), d.term(c.Pre)}})
	}
	if c.Post != nil {
		t.Kids = append(t.Kids, &Term{Kind: KApp, Kids: []*Term{Name("ensures"), d.term(c.Post)}})
	}
	return t
}

// variant is D of a variant declaration.
func (d *desugarer) variant(v *VariantDecl) *Term {
	var head *Term = Name(v.Name)
	if len(v.Params) > 0 {
		head = &Term{Kind: KApp, Kids: []*Term{Name(v.Name)}}
		for _, p := range v.Params {
			head.Kids = append(head.Kids, Name(p))
		}
	}
	t := &Term{Kind: KApp, Kids: []*Term{Name("variant"), head}}
	for _, it := range v.Items {
		switch {
		case it.Success != "":
			t.Kids = append(t.Kids, &Term{Kind: KApp, Kids: []*Term{Name("success"), Name(it.Success)}})
		case it.Relevant:
			t.Kids = append(t.Kids, &Term{Kind: KApp, Kids: []*Term{Name("relevant")}})
		case it.Payload != nil:
			t.Kids = append(t.Kids, &Term{Kind: KApp, Kids: []*Term{Name(it.Ctor), d.term(it.Payload)}})
		default:
			t.Kids = append(t.Kids, Name(it.Ctor))
		}
	}
	return t
}

// ---- Host declarations (step 1c, ADR 0051 D4) ----

// TermTree is the tree a target file's term reads as: a name, a literal, an
// application of them, or a product `(tuple …)`, which the reader has made a
// Church tuple. A target file's set expressions, claims and host clauses are
// only these, so D(TermTree(t)) is t. Any other λ is refused: no declaration's
// claim holds one, and reading one back would need its body opened.
func TermTree(t *Term) (Expr, error) {
	switch t.Kind {
	case KName:
		return &Var{Name: t.Name}, nil
	case KInt, KFloat, KStr, KBool:
		return &Lit{T: t}, nil
	case KApp:
		if len(t.Kids) == 0 {
			return &EmptyList{}, nil
		}
		fn, err := TermTree(t.Kids[0])
		if err != nil {
			return nil, err
		}
		a := &Apply{Fn: fn}
		for _, k := range t.Kids[1:] {
			e, err := TermTree(k)
			if err != nil {
				return nil, err
			}
			a.Args = append(a.Args, e)
		}
		return a, nil
	case KFn:
		// THE PRODUCT FORMER, which the reader has already made a Church tuple:
		// `(tuple A B)` reads as `(fn (#k) (#k A B))`, and D's clause for Σ₀'s
		// `tuple` builds exactly that term back.
		if b := t.Closed(); len(t.Params) == 1 && t.Params[0] == TupleBinder && b.Kind == KApp &&
			len(b.Kids) > 0 && b.Kids[0].Kind == KBound && b.Kids[0].Depth == 0 && b.Kids[0].Index == 0 {
			a := &Apply{Fn: &Var{Name: "tuple"}}
			for _, k := range b.Kids[1:] {
				e, err := TermTree(k)
				if err != nil {
					return nil, err
				}
				a.Args = append(a.Args, e)
			}
			return a, nil
		}
	}
	return nil, fmt.Errorf("%s is not a set expression, a claim or a host clause", t)
}

// HostSig is D of a host definition: today's target-file `sig`, a usage mode
// written back as the type former it was spelled with — a linear table as
// `(buffer V)`, an inout parameter as `(cell T)`, and a variadic list of them as
// `(array (cell T))` (ADR 0050, decision 2).
func HostSig(def *DefDecl) *Term {
	d := newDesugarer()
	l := HostLam(def)
	c, h := l.Claim, def.Host
	variadic := false
	for _, m := range h.Markers {
		variadic = variadic || m == "variadic"
	}
	params := &Term{Kind: KApp}
	for i, p := range c.Params {
		set := p.Set
		switch {
		case p.Mode == Linear:
			set = formed("buffer", set.(*Apply).Args[0])
		case p.Mode == InOut && variadic && i == len(c.Params)-1:
			set = formed("array", formed("cell", set.(*Apply).Args[0]))
		case p.Mode == InOut:
			set = formed("cell", set)
		}
		if p.Name == "" {
			params.Kids = append(params.Kids, d.term(set))
		} else {
			params.Kids = append(params.Kids, App(Name(p.Name), d.term(set)))
		}
	}
	res := c.Result
	switch {
	case len(c.ResultModes) == 1 && c.ResultModes[0] == Linear:
		res = formed("buffer", res.(*Apply).Args[0])
	case len(c.ResultModes) > 1:
		t := res.(*Apply)
		comps := append([]Expr(nil), t.Args...)
		for i, m := range c.ResultModes {
			if m == Linear {
				comps[i] = formed("buffer", comps[i].(*Apply).Args[0])
			}
		}
		res = &Apply{Fn: t.Fn, Args: comps}
	}
	result := d.term(res)
	// A head that binds its static parameters, the outer λ's (spec/typevalues.md §4).
	head := Name(def.Name)
	if outer := def.Value.(*Lam); outer != l {
		head = App(Name(def.Name))
		for _, p := range outer.Claim.Params {
			if p.Set == nil {
				head.Kids = append(head.Kids, Name(p.Name))
			} else {
				head.Kids = append(head.Kids, App(Name(p.Name), d.term(p.Set)))
			}
		}
	}
	t := &Term{Kind: KApp, Kids: []*Term{Name("sig"), head, params, result}}
	for _, m := range h.Markers {
		t.Kids = append(t.Kids, Name(m))
	}
	if c.Pre != nil {
		t.Kids = append(t.Kids, App(Name("where"), d.term(c.Pre)))
	}
	if c.Post != nil {
		t.Kids = append(t.Kids, App(Name("ensures"), d.term(c.Post)))
	}
	host := App(Name("host"), Name(h.Kind), Str(h.Template))
	for _, k := range h.Clauses {
		host.Kids = append(host.Kids, d.term(k))
	}
	t.Kids = append(t.Kids, host)
	return t
}

// HostLam is a host definition's λ, the one whose body is the host's: the
// definition's value, or, under static parameters, the body of the static λ
// that binds them (spec/typevalues.md §4).
func HostLam(def *DefDecl) *Lam {
	l := def.Value.(*Lam)
	if inner, ok := l.Body.(*Lam); ok {
		return inner
	}
	return l
}

// formed is a type former applied to one set, `(buffer V)`.
func formed(name string, x Expr) Expr { return &Apply{Fn: &Var{Name: name}, Args: []Expr{x}} }
