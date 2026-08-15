package emit

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"oroboros/core"
)

// A target, declared as data rather than as Go source.
//
// This exists because requirements 3 and 4 were false: adding a host function
// meant editing three Go files and rebuilding the compiler. What a target
// declaration has to carry was not designed — it was read off what three
// backends turned out to need, and corrected twice along the way (a *kind*
// rather than an expression/statement boolean; types optional, because JS has
// none).
//
// The boundary is deliberate. **Expression and statement primitives are pure
// data** — a template, an arity, types, and an optional import. **Structural
// primitives are named in data and implemented in code**: a loop binds
// variables and emits a header, and no template expresses that. So adding
// `fmt.Println` needs no Go; adding a new control structure does.

type Prim struct {
	Name   string
	Args   []string // our type names; empty when the target is untyped
	Result string
	Kind   string // expr | stmt | loop | loop2 | cond | let
	Form   string // template with %s holes; empty for structural kinds
	Import string
	Pure   bool // declared `pure`; DEFAULTS TO FALSE, deliberately — see below
}

type Target struct {
	Name  string
	Types map[string]string // our type name -> the target's spelling
	Prims map[string]Prim
	Names []string // every primitive name, for core.Env
}

// Kinds that the emitter implements in code rather than from a template.
var structuralKinds = map[string]bool{
	"loop": true, "loop2": true, "cond": true, "let": true,
}

func LoadTarget(path string) (*Target, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	terms, err := core.ReadAll(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(terms) != 1 {
		return nil, fmt.Errorf("%s: expected one (target …) form, got %d", path, len(terms))
	}
	return parseTarget(terms[0], path)
}

func parseTarget(t *core.Term, path string) (*Target, error) {
	if t.Kind != core.KApp || t.Kids[0].Kind != core.KName || t.Kids[0].Name != "target" {
		return nil, fmt.Errorf("%s: expected (target NAME …)", path)
	}
	if len(t.Kids) < 2 || t.Kids[1].Kind != core.KName {
		return nil, fmt.Errorf("%s: target needs a name", path)
	}
	tg := &Target{Name: t.Kids[1].Name, Types: map[string]string{}, Prims: map[string]Prim{}}

	for _, f := range t.Kids[2:] {
		if f.Kind != core.KApp || f.Kids[0].Kind != core.KName {
			return nil, fmt.Errorf("%s: expected (type …) or (prim …), got %s", path, f)
		}
		switch f.Kids[0].Name {
		case "type":
			if len(f.Kids) != 3 || f.Kids[1].Kind != core.KName || f.Kids[2].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (type NAME \"spelling\"), got %s", path, f)
			}
			tg.Types[f.Kids[1].Name] = f.Kids[2].Str
		case "prim":
			if err := tg.declare(f, "", path); err != nil {
				return nil, err
			}
		case "structural":
			// (structural NAME KIND [pure]) — the four the backend implements.
			// They carry NO TYPES, because fold-range is
			// A x int x ((A,int) -> A) -> A and that cannot be written in a
			// monomorphic table. Writing (f64 int any) f64 was a false
			// statement in every target file (target-files.md §4).
			p, err := parseStructural(f, path)
			if err != nil {
				return nil, err
			}
			if _, dup := tg.Prims[p.Name]; dup {
				return nil, fmt.Errorf("%s: %s is declared twice", path, p.Name)
			}
			tg.Prims[p.Name] = p
			tg.Names = append(tg.Names, p.Name)
		case "module":
			// (module PATH (prim …) …) — the names this target provides
			// NATIVELY from that module. A target may provide any subset,
			// including none, which is what makes porting demand-driven
			// (modules.md §4).
			if len(f.Kids) < 2 || f.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("%s: (module PATH (prim …)…), got %s", path, f)
			}
			for _, inner := range f.Kids[2:] {
				if inner.Kind != core.KApp || inner.Kids[0].Kind != core.KName ||
					inner.Kids[0].Name != "prim" {
					return nil, fmt.Errorf("%s: module %s may only contain (prim …), got %s",
						path, f.Kids[1].Name, inner)
				}
				if err := tg.declare(inner, f.Kids[1].Name, path); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unknown target form %q", path, f.Kids[0].Name)
		}
	}
	sort.Strings(tg.Names)
	return tg, nil
}

// declare records one primitive under a module path. The name stored is the
// FULLY QUALIFIED one, because that is what resolution produces and R1 requires
// targets and libraries to key the same namespace (modules.md §5).
func (tg *Target) declare(f *core.Term, modPath, file string) error {
	p, err := parsePrim(f, file)
	if err != nil {
		return err
	}
	if modPath != "" {
		p.Name = modPath + "." + p.Name
	}
	if _, dup := tg.Prims[p.Name]; dup {
		return fmt.Errorf("%s: %s is declared twice", file, p.Name)
	}
	tg.Prims[p.Name] = p
	tg.Names = append(tg.Names, p.Name)
	return nil
}

// (prim NAME (ARGTYPES…) RESULT KIND ["form"] [(import "x")])
func parsePrim(f *core.Term, path string) (Prim, error) {
	k := f.Kids[1:]
	if len(k) < 4 {
		return Prim{}, fmt.Errorf("%s: (prim NAME (args) result kind [form] [(import x)]), got %s", path, f)
	}
	if k[0].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: prim needs a name, got %s", path, k[0])
	}
	p := Prim{Name: k[0].Name}

	// Argument types. `()` reads as an empty list, which the reader rejects, so
	// a nullary primitive writes `(none)`.
	if k[1].Kind == core.KApp {
		for _, a := range k[1].Kids {
			if a.Kind != core.KName {
				return Prim{}, fmt.Errorf("%s: %s has a non-name argument type", path, p.Name)
			}
			if a.Name != "none" {
				p.Args = append(p.Args, a.Name)
			}
		}
	} else if k[1].Kind == core.KName && k[1].Name != "none" {
		p.Args = []string{k[1].Name}
	}

	if k[2].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: %s has a non-name result type", path, p.Name)
	}
	p.Result = k[2].Name

	if k[3].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: %s has a non-name kind", path, p.Name)
	}
	p.Kind = k[3].Name
	switch p.Kind {
	case "expr", "stmt":
	case "loop", "loop2", "cond", "let":
		return Prim{}, fmt.Errorf("%s: %s is %s, which is structural and carries no types; "+
			"write (structural %s %s [pure])", path, p.Name, p.Kind, p.Name, p.Kind)
	default:
		return Prim{}, fmt.Errorf("%s: %s has unknown kind %q (expr, stmt)", path, p.Name, p.Kind)
	}

	for _, rest := range k[4:] {
		switch {
		case rest.Kind == core.KStr:
			p.Form = rest.Str
		case rest.Kind == core.KName && rest.Name == "pure":
			p.Pure = true
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "import" && len(rest.Kids) == 2 && rest.Kids[1].Kind == core.KStr:
			p.Import = rest.Kids[1].Str
		default:
			return Prim{}, fmt.Errorf("%s: %s has an unexpected trailing form %s", path, p.Name, rest)
		}
	}
	if !structuralKinds[p.Kind] && p.Form == "" {
		return Prim{}, fmt.Errorf("%s: %s is %s and needs an emission template", path, p.Name, p.Kind)
	}
	return p, nil
}

// parseStructural reads (structural NAME KIND [pure]). No argument types, no
// result type, no template — a structural primitive's types and its emission
// both live in the backend, which is the only place either can be expressed.
func parseStructural(f *core.Term, path string) (Prim, error) {
	k := f.Kids[1:]
	if len(k) < 2 || k[0].Kind != core.KName || k[1].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: (structural NAME KIND [pure]), got %s", path, f)
	}
	p := Prim{Name: k[0].Name, Kind: k[1].Name}
	if !structuralKinds[p.Kind] {
		return Prim{}, fmt.Errorf("%s: %s has kind %q, which is not structural "+
			"(let, cond, loop, loop2)", path, p.Name, p.Kind)
	}
	for _, rest := range k[2:] {
		if rest.Kind == core.KName && rest.Name == "pure" {
			p.Pure = true
			continue
		}
		return Prim{}, fmt.Errorf("%s: %s has an unexpected trailing form %s", path, p.Name, rest)
	}
	return p, nil
}

// Env builds the reduction environment. Which names are primitive is exactly
// what the target file declares, so this is the whole of ADR 0002's parameter.
//
// Purity travels with it, and its default is the point (effects.md §3): a target
// author who forgets `pure` gets a slower program, where one who forgot an
// `effect` marker under the opposite default would get a silent miscompilation.
// The default must be the one whose failure mode is slow, not wrong.
func (tg *Target) Env(p *core.Program) (*core.Env, error) {
	e := &core.Env{
		Defs: p.Defs,
		Prim: map[string]bool{},
		Pure: map[string]bool{},
		Rec:  map[string]bool{},
	}
	for _, n := range tg.Names {
		e.Prim[n] = true
		e.Pure[n] = tg.Prims[n].Pure
	}
	// `let` reaches the reducer only in a residual it produced itself, so it is
	// primitive here without being declared. Its own application does nothing —
	// whether a let is pure is decided by its value and its body, which the
	// judgement reads through.
	e.Prim["let"] = true
	e.Pure["let"] = true
	e.MarkRecursive()
	return e, e.CheckDefs()
}

// fill applies a template to operands, cycling them to cover however many holes
// the template has. `%s[%s]++` names two operands once each; JS's dictionary
// increment names the same two twice; `fmt.Println(%s)` names its one operand
// once. Repeating a fixed number of times worked only while every stmt
// primitive was a dictionary update.
func fill(form string, vals []any) string {
	holes := strings.Count(form, "%s")
	if len(vals) == 0 || holes <= len(vals) {
		return fmt.Sprintf(form, vals[:min(holes, len(vals))]...)
	}
	out := make([]any, holes)
	for i := range out {
		out[i] = vals[i%len(vals)]
	}
	return fmt.Sprintf(form, out...)
}

// ty spells one of our type names in the target's own language. An untyped
// target declares no types and this is never consulted.
func (tg *Target) ty(name string) string {
	if s, ok := tg.Types[name]; ok {
		return s
	}
	if name == "" {
		return "/*unknown*/"
	}
	return "/*" + name + "?*/"
}
