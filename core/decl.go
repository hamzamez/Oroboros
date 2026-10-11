package core

import (
	"fmt"
	"sort"
)

// DECLARATIONS AS TREES — docs/spec/abstract.md §2.6, ADR 0051 (step 1b).
//
// μ reads each top-level form into a Decl. Every form is first read as today's
// reader read it, toForm on its image, so a refusal and its message are what
// they were; the tree is then built from the form as written. After the whole
// file, a `sig` becomes the Claim of its definition's λ: a definition is a
// named λ, and a λ carries what it claims (ADR 0051, D1). D gives the forms back
// in the order they were written, the claim where the `sig` stood, so every
// stage after the reader reads what it always has.

// muDecl reads one top-level form, `f` being what toForm made of its image.
func (r *reader) muDecl(datum *Term, f Form, at int) (Decl, error) {
	kids := datum.Kids
	switch f.Kind {
	case "def":
		if len(kids) == 4 { // the equational shorthand, (def f (x…) body)
			b := r.expr(kids[3])
			return &DefDecl{Name: f.Name, Value: r.mark(&Lam{Params: f.Term.Params, Body: b}, b), At: at}, nil
		}
		return &DefDecl{Name: f.Name, Value: r.expr(kids[2]), At: at}, nil
	case "sig":
		c, err := r.claim(kids, f.Sig, at)
		if err != nil {
			return nil, err
		}
		return &ClaimDecl{Name: f.Name, Claim: c}, nil
	case "sum":
		v := &VariantDecl{Name: f.Sum.Name, Params: f.Sum.Params, At: at}
		for _, k := range kids[2:] {
			img := r.image(k)
			switch {
			case img.Kind == KName:
				v.Items = append(v.Items, VariantItem{Ctor: img.Name})
			case img.Kids[0].Name == "success":
				v.Items = append(v.Items, VariantItem{Success: img.Kids[1].Name})
			case img.Kids[0].Name == "relevant":
				v.Items = append(v.Items, VariantItem{Relevant: true})
			default:
				v.Items = append(v.Items, VariantItem{Ctor: img.Kids[0].Name, Payload: r.expr(k.Kids[1])})
			}
		}
		return v, nil
	case "module":
		return &ModuleDecl{Path: f.Name, At: at}, nil
	case "use":
		u := &UseDecl{Path: f.Name, At: at}
		if len(kids) == 4 {
			u.As = f.Alias
		}
		return u, nil
	case "export":
		return &ExportDecl{Names: f.Names, At: at}, nil
	case "term":
		return &TermDecl{E: r.expr(datum), At: at}, nil
	}
	return &ForeignDecl{E: r.expr(datum), At: at}, nil // provides, prim, target
}

// claim reads a `sig`'s claim: each parameter's set, the result's set, and the
// `where` and `ensures` it states (ADR 0051, D2). A parameter list is read as a
// list (mu.go): its elements are the parameters the program wrote.
func (r *reader) claim(kids []*Term, sig *Sig, at int) (*Claim, error) {
	ps := kids[2].Kids
	if len(ps) != len(sig.Params) {
		return nil, fmt.Errorf("sig %s: a parameter list is a list of parameters, each TYPE or "+
			"(name TYPE); got %s", kids[1].Name, r.image(kids[2]))
	}
	c := &Claim{Result: r.expr(kids[3]), At: at}
	for i, p := range ps {
		if sig.Params[i].Name == "" {
			c.Params = append(c.Params, ClaimParam{Set: r.expr(p)})
			continue
		}
		c.Params = append(c.Params, ClaimParam{Name: sig.Params[i].Name, Set: r.expr(p.Kids[1])})
	}
	for _, k := range kids[4:] {
		switch r.image(k).Kids[0].Name {
		case "where":
			c.Pre = r.expr(k.Kids[1])
		case "ensures":
			c.Post = r.expr(k.Kids[1])
		}
	}
	return c, nil
}

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
	sort.Slice(ts, func(i, j int) bool { return ts[i].at < ts[j].at })
	out := make([]Form, len(ts))
	for i, p := range ts {
		f, err := toForm(p.t)
		if err != nil {
			return nil, err
		}
		out[i] = f
	}
	return out, nil
}

// sig is D of a claim: today's `sig`, which binds its own names.
func (d *desugarer) sig(name string, c *Claim) *Term {
	params := &Term{Kind: KApp}
	for _, p := range c.Params {
		if p.Name == "" {
			params.Kids = append(params.Kids, d.term(p.Set))
			continue
		}
		params.Kids = append(params.Kids, &Term{Kind: KApp, Kids: []*Term{Name(p.Name), d.term(p.Set)}})
	}
	t := &Term{Kind: KApp, Kids: []*Term{Name("sig"), Name(name), params, d.term(c.Result)}}
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
