package core

import (
	"fmt"
	"sort"
	"strings"
)

// A CLAIMED λ IS A DEFINITION WITH A NAME OF ITS OWN (the book's finding 121,
// ADR 0158). ADR 0051 made a λ carry what it claims, lam(x̄ ∈ S̄; r ∈ U; φ; ψ;
// e), and a definition the name bound to one: the two differ by a name, and its
// "Why not" refuses a second operator for the difference. So an anonymous λ
// that claims is given the name, and its claim is a definition's, meant the
// same way:
//
//   - its domain, P = {x̄ ∈ S̄ : φ}, an obligation at each application (ADR
//     0028, 0154), where the definition is inlined;
//   - its codomain, f ∈ Π(x̄ ∈ P). R with R = {r ∈ U : ψ}, decided at the λ for
//     every x̄ ∈ P (ADR 0072), not at each application: a claim proven only at
//     its arguments is not the claim (`fn(n ∈ Int) (0..9) { n }(1)` is false).
//
// A λ closes over its context, so the name is given by λ-lifting (Johnsson
// 1985): with ȳ the λ's free variables bound around it,
//
//	fn(x̄ ∈ S̄) (R) where φ ensure ψ { e }  =  fn(x̄) { f#λ(ȳ, x̄) },
//	fn f#λ(ȳ, x̄ ∈ S̄) (R) where φ ensure ψ { e },
//
// an equation by β, so staging changes no answer (ADR 0009). The codomain is
// then decided in the part of the λ's context Γ that is declared: a yᵢ that is
// the enclosing definition's parameter keeps its set there, Γ's assumption, and
// the call of f#λ obliges it, a fact of the body; any other yᵢ is claimed by
// nothing (`any`), so the codomain is decided for every value of it, which
// implies it for the one the λ sees and refuses a claim that holds only by a
// fact the context derives. The η-expanded form keeps a λ where one
// was written, so a host's callback parameter still meets a λ (ADR 0143), whose
// parameters take the host's claim and whose call of f#λ obliges them to the
// λ's own: the arrow's subsumption, dom(host) ⊆ dom(λ), by composition.
//
// A free variable that is a place (a `var` cell, a buffer, a map buffer, a
// borrowed buffer) is state, which a value parameter cannot carry: the λ that
// captures one is refused, the threading through a named definition's
// `inout` (ADR 0135, 0144) not built for it.

// ClaimedLamName marks an anonymous λ that carries a claim, (#claimed-λ k fn),
// k its index among the desugarer's claimed λs, until placed lifts it.
const ClaimedLamName = "#claimed-λ"

// lambdaMark is the infix a lifted λ's name carries: `run#λ3`, the λ written on
// line 3 inside run. `#` starts no identifier, so no program's name is taken.
const lambdaMark = "#λ"

// PlantLambdaClaimDropped lifts a claimed λ without its claim: a false claim is
// then accepted (TestALambdaCarriesItsClaim).
var PlantLambdaClaimDropped bool

// liftClaimed lifts each claimed λ in a definition's term to a definition of
// its own, returning the term and the lifted definitions with their `sig`s.
func (d *desugarer) liftClaimed(owner string, root *Lam, t *Term) (*Term, []*Term, error) {
	if !termMentions(t, ClaimedLamName) {
		return t, nil, nil
	}
	var lifted []*Term
	taken := map[string]bool{}
	with := func(m map[string]bool, add []string, on bool) map[string]bool {
		out := make(map[string]bool, len(m)+len(add))
		for k, v := range m {
			out[k] = v
		}
		for _, a := range add {
			if on {
				out[a] = true
			} else {
				delete(out, a)
			}
		}
		return out
	}
	// sets are the enclosing definition's parameters' sets, where not shadowed
	sets := map[string]Expr{}
	var walk func(t *Term, bound, places map[string]bool) (*Term, error)
	// fn walks a λ's body, its parameters bound, and places when they are
	fn := func(t *Term, bound, places map[string]bool, place bool) (*Term, error) {
		saved := map[string]Expr{}
		for _, p := range t.Params {
			if s, ok := sets[p]; ok {
				saved[p] = s
				delete(sets, p)
			}
		}
		defer func() {
			for p, s := range saved {
				sets[p] = s
			}
		}()
		body, err := walk(t.Body(), with(bound, t.Params, true), with(places, t.Params, place))
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, body), nil
	}
	walk = func(t *Term, bound, places map[string]bool) (*Term, error) {
		if !termMentions(t, ClaimedLamName) {
			return t, nil
		}
		switch t.Kind {
		case KFn:
			return fn(t, bound, places, false)
		case KApp:
			if headIs(t, ClaimedLamName) && len(t.Kids) == 3 && t.Kids[1].Kind == KInt && t.Kids[2].Kind == KFn {
				lam := d.claimed[t.Kids[1].Int]
				f, err := fn(t.Kids[2], bound, places, false)
				if err != nil {
					return nil, err
				}
				var ys []string
				for n := range freeVars(f) {
					if bound[n] {
						ys = append(ys, n)
					}
				}
				sort.Strings(ys)
				for _, y := range ys {
					if places[y] {
						return nil, fmt.Errorf("line %d: the λ claims a set and captures %s, which is state (a `var`, a buffer or a map buffer): a λ that claims is a definition, and state reaches a definition only as its parameter, `fn f(%s ∈ S inout, …)` for a cell, which an unclaimed λ calls, `fn(…) { f(%s, …) }`", lam.Line, Spell(y), Spell(y), Spell(y))
					}
				}
				name := fmt.Sprintf("%s%s%d", owner, lambdaMark, lam.Line)
				for k := 2; taken[name]; k++ {
					name = fmt.Sprintf("%s%s%d_%d", owner, lambdaMark, lam.Line, k)
				}
				taken[name] = true
				params := append(append([]string(nil), ys...), f.Params...)
				lifted = append(lifted, App(Name("def"), Name(name), Fn(params, f.Body())))
				if !PlantLambdaClaimDropped {
					c := *lam.Claim
					c.Params = nil
					for _, y := range ys {
						c.Params = append(c.Params, ClaimParam{Name: y, Set: sets[y]})
					}
					c.Params = append(c.Params, lam.Claim.Params...)
					lifted = append(lifted, d.sig(name, &c))
				}
				args := make([]*Term, len(params))
				for i, p := range params {
					args[i] = Name(p)
				}
				return Fn(f.Params, App(Name(name), args...)), nil
			}
			// A PLACE'S BINDER: (#cell e (fn (x) …)), (#place x (fn (x) …)),
			// (#mapplace m (fn (m) …) cap), (#borrow b… (fn (b…) …)) bind state
			place := headIs(t, CellName) || headIs(t, PlaceName) || headIs(t, MapPlaceName) || headIs(t, BorrowName)
			kids := make([]*Term, len(t.Kids))
			for i, k := range t.Kids {
				var err error
				if place && k.Kind == KFn {
					kids[i], err = fn(k, bound, places, true)
				} else {
					kids[i], err = walk(k, bound, places)
				}
				if err != nil {
					return nil, err
				}
			}
			return &Term{Kind: KApp, Kids: kids}, nil
		}
		return t, nil
	}
	var out *Term
	var err error
	if root != nil && t.Kind == KFn && len(t.Params) == len(root.Params) {
		// THE DEFINITION'S OWN PARAMETERS are bound with their claimed sets
		// (a mode's set is a place's, and stays unclaimed: a place is refused)
		if root.Claim != nil {
			for _, cp := range root.Claim.Params {
				if cp.Mode == NoMode && cp.Set != nil && !containsString(root.Borrows, cp.Name) {
					sets[cp.Name] = cp.Set
				}
			}
		}
		body, e := walk(t.Body(), with(nil, t.Params, true), with(nil, root.Borrows, true))
		if e == nil {
			out = Fn(t.Params, body)
		}
		err = e
	} else {
		out, err = walk(t, map[string]bool{}, map[string]bool{})
	}
	if err != nil {
		return nil, nil, err
	}
	return out, lifted, nil
}

// mentions says whether a name occurs in t.
func termMentions(t *Term, name string) bool {
	if t.Kind == KName {
		return t.Name == name
	}
	for _, k := range t.Kids {
		if termMentions(k, name) {
			return true
		}
	}
	return false
}

// showLifted is a lifted λ's name as a message writes it: `the λ at line 3`.
func showLifted(n string) (string, bool) {
	i := strings.Index(n, lambdaMark)
	if i < 0 {
		return "", false
	}
	line, _, _ := strings.Cut(n[i+len(lambdaMark):], "_")
	return "the λ at line " + line, true
}

// ShowLifted is a definition's name as a refusal writes it: a lifted λ's as
// `the λ at line 3`, any other as it is.
func ShowLifted(n string) string {
	if s, ok := showLifted(n); ok {
		return s
	}
	return n
}
