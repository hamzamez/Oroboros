package emit

import (
	"fmt"

	"oroboros/core"
)

// A HOST DECLARATION IS A DEFINITION WITH A HOST BODY — docs/spec/abstract.md
// §2.6, ADR 0051 (step 1c).
//
// A target file's `sig` claims each parameter's set, the result's set, a
// precondition and a postcondition, and is realized by a template: it is a
// definition whose λ carries those claims and whose body is the host's. The
// claim is an axiom, checked against the host (ADR 0022). hostDef reads the
// declaration into that tree, and core.HostSig (D) writes today's `sig` back,
// which parseSig reads: every declaration a target holds has passed through
// its tree, and the primitive it becomes is the one it always was (Theorem D3,
// checked on every target file by TestHostDeclarationsAreDefinitions).
//
// USAGE IS A MODE (ADR 0050, decision 2): `(buffer V)` is a table bound
// linearly, `(cell T)` a parameter the host writes by value-result, inout
// (ADR 0048), and `(array (cell T))` on a variadic declaration's list is inout
// on each trailing argument (ADR 0046). A former anywhere deeper than a
// parameter, the result, or a tuple result's component stays in its set.

// hostDef reads one declaration, `p` being what parseSig made of it.
func hostDef(f *core.Term, p Prim, path string) (*core.DefDecl, error) {
	if formWord(f) == "const" {
		s, err := constSig(f, path)
		if err != nil {
			return nil, err
		}
		f = s
	}
	var ps []*core.Term // the parameters as written, `none` dropped
	switch a := f.Kids[2]; {
	case a.Kind == core.KApp:
		for _, k := range a.Kids {
			if k.Kind != core.KName || k.Name != "none" {
				ps = append(ps, k)
			}
		}
	case a.Kind == core.KName && a.Name != "none":
		ps = []*core.Term{a}
	}
	if len(ps) != len(p.Args) {
		return nil, fmt.Errorf("%s: sig %s: %d parameters read as %d", path, p.Name, len(ps), len(p.Args))
	}
	var h core.HostBody
	var c core.Claim
	for _, k := range f.Kids[4:] {
		switch w := formWord(k); {
		case k.Kind == core.KName:
			h.Markers = append(h.Markers, w)
		case w == "where" || w == "ensures":
			e, err := core.TermTree(k.Kids[1])
			if err != nil {
				return nil, fmt.Errorf("%s: sig %s: %w", path, p.Name, err)
			}
			if w == "where" {
				c.Pre = e
			} else {
				c.Post = e
			}
		case w == "host":
			h.Kind, h.Template = k.Kids[1].Name, k.Kids[2].Str
			for _, hc := range k.Kids[3:] {
				e, err := core.TermTree(hc)
				if err != nil {
					return nil, fmt.Errorf("%s: sig %s: %w", path, p.Name, err)
				}
				h.Clauses = append(h.Clauses, e)
			}
		}
	}
	lam := &core.Lam{Claim: &c}
	for i, k := range ps {
		name, ty := "", k
		if len(p.Names) == len(p.Args) && p.Names[i] != "" {
			name, ty = p.Names[i], k.Kids[1]
		}
		e, err := core.TermTree(ty)
		if err != nil {
			return nil, fmt.Errorf("%s: sig %s: %w", path, p.Name, err)
		}
		set, mode := paramMode(e, p.Variadic && i == len(ps)-1)
		c.Params = append(c.Params, core.ClaimParam{Name: name, Set: set, Mode: mode})
		if name == "" {
			name = "_"
		}
		lam.Params = append(lam.Params, name)
	}
	e, err := core.TermTree(f.Kids[3])
	if err != nil {
		return nil, fmt.Errorf("%s: sig %s: %w", path, p.Name, err)
	}
	c.Result, c.ResultModes = resultModes(e)
	return &core.DefDecl{Name: f.Kids[1].Name, Value: lam, Host: &h}, nil
}

// paramMode separates a parameter's usage from its set.
func paramMode(ty core.Expr, variadicList bool) (core.Expr, core.Mode) {
	switch {
	case former(ty, "buffer"):
		return applied("array", arg(ty)), core.Linear
	case former(ty, "cell"):
		return arg(ty), core.InOut
	case variadicList && former(ty, "array") && former(arg(ty), "cell"):
		return applied("array", arg(arg(ty))), core.InOut
	}
	return ty, core.NoMode
}

// resultModes separates the result binder's usage from its set: a buffer handed
// back, alone or as a component of a tuple result.
func resultModes(rt core.Expr) (core.Expr, []core.Mode) {
	if former(rt, "buffer") {
		return applied("array", arg(rt)), []core.Mode{core.Linear}
	}
	t, ok := rt.(*core.Apply)
	if !ok || len(t.Args) == 0 {
		return rt, nil
	}
	if v, isVar := t.Fn.(*core.Var); !isVar || v.Name != "tuple" {
		return rt, nil
	}
	modes := make([]core.Mode, len(t.Args))
	comps := append([]core.Expr(nil), t.Args...)
	linear := false
	for i, k := range t.Args {
		if former(k, "buffer") {
			comps[i], modes[i], linear = applied("array", arg(k)), core.Linear, true
		}
	}
	if !linear {
		return rt, nil
	}
	return &core.Apply{Fn: t.Fn, Args: comps}, modes
}

// former reports a type former applied to one argument, `(buffer V)`.
func former(t core.Expr, name string) bool {
	a, ok := t.(*core.Apply)
	if !ok || len(a.Args) != 1 {
		return false
	}
	v, ok := a.Fn.(*core.Var)
	return ok && v.Name == name
}

func arg(t core.Expr) core.Expr { return t.(*core.Apply).Args[0] }

func applied(name string, x core.Expr) core.Expr {
	return &core.Apply{Fn: &core.Var{Name: name}, Args: []core.Expr{x}}
}

// declaredPrim is the primitive a declaration becomes: today's reading of it,
// which reports any refusal where it always was, then the declaration read into
// its tree, and the primitive read off the `sig` D writes back.
func declaredPrim(f *core.Term, path string) (Prim, error) {
	p, err := parseSig(f, path)
	if err != nil {
		return Prim{}, err
	}
	def, err := hostDef(f, p, path)
	if err != nil {
		return Prim{}, err
	}
	return parseSig(core.HostSig(def), path)
}
