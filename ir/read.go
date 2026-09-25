package ir

import (
	"fmt"
	"strconv"
	"strings"

	"oroboros/core"
)

// THE READER (spec §2.1, §10). The file is raw s-expressions in the language's
// lexical syntax, read by core.ReadRaw, and every list is dispatched on its
// head. It checks the GRAMMAR only; whether a program is well formed is the
// verifier's question (spec §3), so a planted fault reads and is then refused
// with the rule it breaks.

// Read parses one IR file.
func Read(src string) (*Program, error) {
	ts, err := core.ReadRaw(src)
	if err != nil {
		return nil, err
	}
	if len(ts) != 1 {
		return nil, fmt.Errorf("an IR file is one (ir …) form, found %d forms", len(ts))
	}
	rd := &reader{}
	p, err := rd.program(ts[0])
	if err != nil {
		return nil, err
	}
	return p, nil
}

type reader struct {
	f *Func
}

func head(t *core.Term) string {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		return t.Kids[0].Name
	}
	return ""
}

func (rd *reader) program(t *core.Term) (*Program, error) {
	if head(t) != "ir" || len(t.Kids) < 2 || t.Kids[1].Kind != core.KInt {
		return nil, fmt.Errorf("an IR file begins (ir VERSION …)")
	}
	if v := t.Kids[1].Int; v != Version {
		return nil, fmt.Errorf("IR version %d; this reader reads version %d", v, Version)
	}
	p := &Program{}
	for _, k := range t.Kids[2:] {
		switch head(k) {
		case "target":
			if len(k.Kids) != 2 || k.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("(target NAME)")
			}
			p.Target = k.Kids[1].Name
		case "stage":
			if len(k.Kids) != 2 || k.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("(stage A|P)")
			}
			switch k.Kids[1].Name {
			case "A":
				p.Stage = StageA
			case "P":
				p.Stage = StageP
			default:
				return nil, fmt.Errorf("stage %s: A or P", k.Kids[1].Name)
			}
		case "ops":
			p.Header = []string{}
			for _, o := range k.Kids[1:] {
				if o.Kind != core.KName {
					return nil, fmt.Errorf("(ops …) lists names")
				}
				p.Header = append(p.Header, o.Name)
			}
		case "global":
			if len(k.Kids) != 4 || k.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("(global NAME τ LITERAL)")
			}
			ty, err := typeOf(k.Kids[2])
			if err != nil {
				return nil, err
			}
			p.Globals = append(p.Globals, Global{Name: k.Kids[1].Name, Type: ty, Lit: k.Kids[3]})
		case "func":
			f, err := rd.function(k)
			if err != nil {
				return nil, err
			}
			p.Funcs = append(p.Funcs, f)
		default:
			return nil, fmt.Errorf("unknown form (%s …) in an IR file", head(k))
		}
	}
	return p, nil
}

func (rd *reader) function(t *core.Term) (*Func, error) {
	if len(t.Kids) != 5 || t.Kids[1].Kind != core.KName || head(t.Kids[2]) != "params" || head(t.Kids[3]) != "results" {
		return nil, fmt.Errorf("(func NAME (params …) (results …) (region …))")
	}
	f := &Func{Name: t.Kids[1].Name}
	rd.f = f
	for _, pt := range t.Kids[2].Kids[1:] {
		v, err := rd.param(pt)
		if err != nil {
			return nil, err
		}
		f.Params = append(f.Params, v)
	}
	for _, rt := range t.Kids[3].Kids[1:] {
		ty, err := typeOf(rt)
		if err != nil {
			return nil, err
		}
		f.Results = append(f.Results, ty)
	}
	body, err := rd.region(t.Kids[4])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	f.Body = body
	return f, nil
}

// value reads `%N`.
func (rd *reader) value(t *core.Term) (V, error) {
	if t.Kind != core.KName || !strings.HasPrefix(t.Name, "%") {
		return 0, fmt.Errorf("%s is not a value (%%N)", t)
	}
	n, err := strconv.Atoi(t.Name[1:])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s is not a value (%%N)", t.Name)
	}
	for len(rd.f.Types) <= n {
		rd.f.Types = append(rd.f.Types, "")
	}
	return V(n), nil
}

func (rd *reader) values(ts []*core.Term) ([]V, error) {
	out := make([]V, len(ts))
	for i, t := range ts {
		v, err := rd.value(t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// param reads `(%N τ)`, a definition with its type.
func (rd *reader) param(t *core.Term) (V, error) {
	if t.Kind != core.KApp || len(t.Kids) != 2 {
		return 0, fmt.Errorf("%s is not (%%N τ)", t)
	}
	v, err := rd.value(t.Kids[0])
	if err != nil {
		return 0, err
	}
	ty, err := typeOf(t.Kids[1])
	if err != nil {
		return 0, err
	}
	rd.f.Types[v] = ty
	return v, nil
}

func (rd *reader) region(t *core.Term) (*Region, error) {
	if head(t) != "region" || len(t.Kids) < 2 {
		return nil, fmt.Errorf("%s is not (region … TERMINATOR)", t)
	}
	r := &Region{}
	kids := t.Kids[1:]
	if head(kids[0]) == "params" {
		for _, pt := range kids[0].Kids[1:] {
			v, err := rd.param(pt)
			if err != nil {
				return nil, err
			}
			r.Params = append(r.Params, v)
		}
		kids = kids[1:]
	}
	if len(kids) == 0 {
		return nil, fmt.Errorf("a region ends in a terminator")
	}
	for _, k := range kids[:len(kids)-1] {
		switch head(k) {
		case "pi":
			pi, err := rd.pi(k)
			if err != nil {
				return nil, err
			}
			r.Pis = append(r.Pis, pi)
		case "val", "do":
			s, err := rd.stmt(k)
			if err != nil {
				return nil, err
			}
			r.Stmts = append(r.Stmts, s)
		default:
			return nil, fmt.Errorf("(%s …) is not a region's statement", head(k))
		}
	}
	last := kids[len(kids)-1]
	switch head(last) {
	case "yield", "break", "continue":
		vs, err := rd.values(last.Kids[1:])
		if err != nil {
			return nil, err
		}
		r.Args = vs
		r.T = map[string]Term{"yield": TYield, "break": TBreak, "continue": TContinue}[head(last)]
	case "branch":
		if len(last.Kids) != 4 {
			return nil, fmt.Errorf("(branch %%c REGION REGION)")
		}
		c, err := rd.value(last.Kids[1])
		if err != nil {
			return nil, err
		}
		th, err := rd.region(last.Kids[2])
		if err != nil {
			return nil, err
		}
		el, err := rd.region(last.Kids[3])
		if err != nil {
			return nil, err
		}
		r.T, r.Cond, r.Then, r.Else = TBranch, c, th, el
	default:
		return nil, fmt.Errorf("a region ends in yield, break, continue or branch, not (%s …)", head(last))
	}
	return r, nil
}

// pi reads `(pi %N τ (%M REL %K))` or `(pi %N τ ((len %M) REL %K))`.
func (rd *reader) pi(t *core.Term) (Pi, error) {
	bad := fmt.Errorf("(pi %%N τ (%%M REL %%K)), got %s", t)
	if len(t.Kids) != 4 || t.Kids[3].Kind != core.KApp || len(t.Kids[3].Kids) != 3 {
		return Pi{}, bad
	}
	v, err := rd.value(t.Kids[1])
	if err != nil {
		return Pi{}, err
	}
	ty, err := typeOf(t.Kids[2])
	if err != nil {
		return Pi{}, err
	}
	rd.f.Types[v] = ty
	g := t.Kids[3]
	pi := Pi{V: v}
	of := g.Kids[0]
	if head(of) == "len" && len(of.Kids) == 2 {
		pi.Len, of = true, of.Kids[1]
	}
	if pi.Of, err = rd.value(of); err != nil {
		return Pi{}, err
	}
	if g.Kids[1].Kind != core.KName {
		return Pi{}, bad
	}
	pi.Rel = g.Kids[1].Name
	switch pi.Rel {
	case "eq", "ne", "lt", "le", "gt", "ge":
	default:
		return Pi{}, fmt.Errorf("%s is not a relation", pi.Rel)
	}
	if pi.Other, err = rd.value(g.Kids[2]); err != nil {
		return Pi{}, err
	}
	return pi, nil
}

// stmt reads `(val (%N τ)… OP)` or `(do OP)`.
func (rd *reader) stmt(t *core.Term) (Stmt, error) {
	if len(t.Kids) < 2 {
		return Stmt{}, fmt.Errorf("an empty statement")
	}
	var res []V
	if head(t) == "val" {
		for _, pt := range t.Kids[1 : len(t.Kids)-1] {
			v, err := rd.param(pt)
			if err != nil {
				return Stmt{}, err
			}
			res = append(res, v)
		}
		if len(res) == 0 {
			return Stmt{}, fmt.Errorf("(val …) with no result is (do …)")
		}
	} else if len(t.Kids) != 2 {
		return Stmt{}, fmt.Errorf("(do OP)")
	}
	s, err := rd.op(t.Kids[len(t.Kids)-1])
	if err != nil {
		return Stmt{}, err
	}
	s.Res = res
	return s, nil
}

func (rd *reader) op(t *core.Term) (Stmt, error) {
	h := head(t)
	o, ok := OpNamed(h)
	if !ok {
		return Stmt{}, fmt.Errorf("(%s …) is not an operation of Σ", h)
	}
	s := Stmt{Op: o}
	args := t.Kids[1:]
	var err error
	switch o {
	case OConst:
		if len(args) != 1 {
			return s, fmt.Errorf("(const LITERAL)")
		}
		switch args[0].Kind {
		case core.KInt, core.KFloat, core.KBool, core.KStr:
			s.Lit = args[0]
		default:
			return s, fmt.Errorf("%s is not a literal", args[0])
		}
		return s, nil
	case OGlobal, OCall:
		if len(args) < 1 || args[0].Kind != core.KName {
			return s, fmt.Errorf("(%s NAME …)", h)
		}
		s.Name = args[0].Name
		if o == OGlobal {
			return s, nil
		}
		s.Args, err = rd.values(args[1:])
		return s, err
	case OAdd, OSub, OMul, ONeg, ODiv, ORem:
		if len(args) > 0 && args[0].Kind == core.KName {
			switch args[0].Name {
			case "exact":
				s.Mode, args = MExact, args[1:]
			case "trap":
				s.Mode, args = MTrap, args[1:]
			}
		}
		s.Args, err = rd.values(args)
		return s, err
	case OMap:
		for _, row := range args {
			if row.Kind != core.KApp || len(row.Kids) != 2 {
				return s, fmt.Errorf("a map row is (%%k %%v)")
			}
			kv, err := rd.values(row.Kids)
			if err != nil {
				return s, err
			}
			s.Args = append(s.Args, kv...)
		}
		return s, nil
	case OThe:
		if len(args) != 2 {
			return s, fmt.Errorf("(the τ %%a)")
		}
		if s.Type, err = typeOf(args[0]); err != nil {
			return s, err
		}
		s.Args, err = rd.values(args[1:])
		return s, err
	case OIf:
		if len(args) != 3 {
			return s, fmt.Errorf("(if %%c REGION REGION)")
		}
		if s.Args, err = rd.values(args[:1]); err != nil {
			return s, err
		}
		for _, a := range args[1:] {
			r, err := rd.region(a)
			if err != nil {
				return s, err
			}
			s.Sub = append(s.Sub, r)
		}
		return s, nil
	case OLoop:
		if len(args) != 2 || head(args[0]) != "init" {
			return s, fmt.Errorf("(loop (init …) REGION)")
		}
		if s.Args, err = rd.values(args[0].Kids[1:]); err != nil {
			return s, err
		}
		r, err := rd.region(args[1])
		if err != nil {
			return s, err
		}
		s.Sub = []*Region{r}
		return s, nil
	case OBuild, OBuildMap, OTabulate:
		if len(args) != 2 {
			return s, fmt.Errorf("(%s %%n REGION)", h)
		}
		if s.Args, err = rd.values(args[:1]); err != nil {
			return s, err
		}
		r, err := rd.region(args[1])
		if err != nil {
			return s, err
		}
		s.Sub = []*Region{r}
		return s, nil
	}
	s.Args, err = rd.values(args)
	return s, err
}
