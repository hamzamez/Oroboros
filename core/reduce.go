package core

import (
	"fmt"
	"sort"
	"strings"
)

// Reduction per core-0 §3: β on application of an abstraction, δ on unfolding a
// global definition, and a normal form parameterised by the target's primitive
// set P.
//
// Two deliberate omissions, both documented in docs/spec/concerns.md:
//
//   - β always substitutes. core-0 requires a let-binding when a variable occurs
//     more than once and the argument is non-trivial (g4's Defect 1). Deciding
//     that needs a grade or a cost model, and the atom has neither. In a pure
//     calculus with no effects the omission costs duplicated work, not wrong
//     answers — but it is a real gap and the filter reduction in the spec
//     depends on it being closed.
//   - There is no notion of effect, so g5's ordering discipline is absent.

type Program struct {
	Defs    map[string]*Term
	Order   []string        // definition order, for stable diagnostics
	Targets map[string][]string
}

// Env is a program viewed through one target: which names reduce, and which are
// the irreducible floor.
type Env struct {
	Defs map[string]*Term
	Prim map[string]bool
	Rec  map[string]bool // recursive definitions are never δ-reduced
}

func NewProgram() *Program {
	return &Program{Defs: map[string]*Term{}, Targets: map[string][]string{}}
}

func Load(forms []Form) (*Program, []*Term, error) {
	p := NewProgram()
	var terms []*Term
	var globalPrim []string
	for _, f := range forms {
		switch f.Kind {
		case "def":
			if _, dup := p.Defs[f.Name]; dup {
				return nil, nil, fmt.Errorf("%s is defined twice", f.Name)
			}
			p.Defs[f.Name] = f.Term
			p.Order = append(p.Order, f.Name)
		case "prim":
			globalPrim = append(globalPrim, f.Names...)
		case "target":
			p.Targets[f.Name] = f.Names
		case "term":
			terms = append(terms, f.Term)
		}
	}
	if len(globalPrim) > 0 {
		p.Targets["default"] = append(p.Targets["default"], globalPrim...)
	}
	return p, terms, nil
}

// Env builds the reduction environment for one target. This is the parameter
// that makes the normal form a parameter: nothing else differs between targets.
func (p *Program) Env(target string) (*Env, error) {
	prims, ok := p.Targets[target]
	if !ok {
		names := make([]string, 0, len(p.Targets))
		for n := range p.Targets {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("no target %q (have: %s)", target, strings.Join(names, ", "))
	}
	e := &Env{Defs: p.Defs, Prim: map[string]bool{}, Rec: map[string]bool{}}
	for _, n := range prims {
		e.Prim[n] = true
	}
	e.markRecursive()
	return e, nil
}

// markRecursive finds definitions reachable from their own bodies. core-0 §6
// makes this the side condition on termination: δ on a recursive definition does
// not terminate, so recursive definitions stay in the residual as target
// functions. This is g3's "recursive functions cannot be rules", arriving as a
// proof obligation.
func (e *Env) markRecursive() {
	for name := range e.Defs {
		seen := map[string]bool{}
		if e.reaches(name, name, seen) {
			e.Rec[name] = true
		}
	}
}

func (e *Env) reaches(from, target string, seen map[string]bool) bool {
	body, ok := e.Defs[from]
	if !ok {
		return false
	}
	var walk func(t *Term) bool
	walk = func(t *Term) bool {
		switch t.Kind {
		case KName:
			if t.Name == target {
				return true
			}
			if e.Prim[t.Name] || seen[t.Name] {
				return false
			}
			seen[t.Name] = true
			return e.reaches(t.Name, target, seen)
		case KInt, KFloat:
			return false
		}
		for _, k := range t.Kids {
			if walk(k) {
				return true
			}
		}
		return false
	}
	return walk(body)
}

// unfoldable reports whether δ applies to this name under this environment.
func (e *Env) unfoldable(name string) bool {
	if e.Prim[name] || e.Rec[name] {
		return false
	}
	_, ok := e.Defs[name]
	return ok
}

const DefaultFuel = 1_000_000

type FuelError struct{ Term *Term }

func (f *FuelError) Error() string {
	return fmt.Sprintf("reduction did not terminate within the step limit; last term head: %s",
		head(f.Term))
}

func head(t *Term) string {
	for t.Kind == KApp {
		t = t.Op()
	}
	return t.String()
}

// Normalize reduces t to normal form: no reducible β-redex and no name outside
// P. Reduction is normal order — leftmost-outermost, and under abstractions —
// which is what a partial evaluator needs: it must not get stuck on an argument
// it cannot evaluate, and it must reduce inside the function bodies that become
// loop bodies.
func Normalize(t *Term, e *Env, fuel int) (*Term, error) {
	f := &fuel
	return normalize(t, e, f)
}

func normalize(t *Term, e *Env, fuel *int) (*Term, error) {
	if *fuel <= 0 {
		return nil, &FuelError{Term: t}
	}
	*fuel--

	switch t.Kind {
	case KInt, KFloat:
		return t, nil

	case KName:
		if e.unfoldable(t.Name) { // δ
			return normalize(e.Defs[t.Name], e, fuel)
		}
		return t, nil

	case KFn:
		b, err := normalize(t.Body(), e, fuel)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, b), nil

	case KApp:
		op, err := normalize(t.Op(), e, fuel)
		if err != nil {
			return nil, err
		}
		args := t.Args()
		if op.Kind == KFn { // β
			if len(op.Params) != len(args) {
				return nil, fmt.Errorf("arity: %s expects %d argument(s), given %d",
					op, len(op.Params), len(args))
			}
			m := make(map[string]*Term, len(args))
			for i, p := range op.Params {
				m[p] = args[i]
			}
			return normalize(subst(op.Body(), m), e, fuel)
		}
		out := make([]*Term, 0, len(t.Kids))
		out = append(out, op)
		for _, a := range args {
			na, err := normalize(a, e, fuel)
			if err != nil {
				return nil, err
			}
			out = append(out, na)
		}
		return &Term{Kind: KApp, Kids: out}, nil
	}
	return nil, fmt.Errorf("unknown term kind %d", t.Kind)
}

// subst is capture-avoiding. core-0 specifies a locally nameless representation,
// which makes capture unrepresentable; this uses names with freshening, which
// makes it merely impossible. The stronger version belongs with the IR format.
func subst(t *Term, m map[string]*Term) *Term {
	if len(m) == 0 {
		return t
	}
	switch t.Kind {
	case KName:
		if r, ok := m[t.Name]; ok {
			return r
		}
		return t
	case KInt, KFloat:
		return t
	case KFn:
		inner := make(map[string]*Term, len(m))
		for k, v := range m {
			inner[k] = v
		}
		for _, p := range t.Params {
			delete(inner, p) // shadowed
		}
		if len(inner) == 0 {
			return t
		}
		// Freshen any parameter that would capture a free variable of a
		// substituend.
		danger := map[string]bool{}
		for _, v := range inner {
			for n := range freeVars(v) {
				danger[n] = true
			}
		}
		params := t.Params
		body := t.Body()
		for i, p := range t.Params {
			if !danger[p] {
				continue
			}
			fresh := p + "'"
			for occupied(fresh, danger, params) {
				fresh += "'"
			}
			if &params[0] == &t.Params[0] {
				params = append([]string(nil), t.Params...)
			}
			params[i] = fresh
			body = subst(body, map[string]*Term{p: Name(fresh)})
		}
		return Fn(params, subst(body, inner))
	case KApp:
		kids := make([]*Term, len(t.Kids))
		for i, k := range t.Kids {
			kids[i] = subst(k, m)
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return t
}

func occupied(name string, danger map[string]bool, params []string) bool {
	if danger[name] {
		return true
	}
	for _, p := range params {
		if p == name {
			return true
		}
	}
	return false
}

func freeVars(t *Term) map[string]bool {
	out := map[string]bool{}
	var walk func(t *Term, bound map[string]bool)
	walk = func(t *Term, bound map[string]bool) {
		switch t.Kind {
		case KName:
			if !bound[t.Name] {
				out[t.Name] = true
			}
		case KInt, KFloat:
		case KFn:
			inner := make(map[string]bool, len(bound)+len(t.Params))
			for k := range bound {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body(), inner)
		default:
			for _, k := range t.Kids {
				walk(k, bound)
			}
		}
	}
	walk(t, map[string]bool{})
	return out
}

// Residual reports names in a normalised term that the target cannot express —
// the ways in which it failed to reach normal form. An empty result means the
// term is fully in the target's vocabulary.
//
// A recursive definition is NOT a failure. δ deliberately does not unfold it
// (core-0 §6), so it survives as a target function, which is correct: `(def f
// (f))` denotes ⊥ and compiles to a function that calls itself. Reporting it
// here would flag the correct compilation of a well-defined term as an error.
func Residual(t *Term, e *Env) []string {
	found := map[string]bool{}
	for n := range freeVars(t) {
		if !e.Prim[n] && !e.Rec[n] {
			found[n] = true
		}
	}
	out := make([]string, 0, len(found))
	for n := range found {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
