package emit

// INSTANTIATION IS APPLICATION — spec/typevalues.md §5, ADR 0063.
//
// A declaration with static parameters is a function from sets,
//
//	(sig (AsType (E (subsets error))) ((err error)) (option E) …)
//	AsType ∈ Π(E ∈ 𝒫(⟦error⟧)). (⟦error⟧ → 1 + ⟦E⟧)
//
// and `(errors.AsType strconv.NumError)` is its application to a set, which the
// reducer reduces at compile time (δ) to the INSTANCE: the declaration with the
// substitution σ = [T̄/Ē] applied to its types and to its template's `%tᵢ`
// holes. An instance is identified applicatively — the declaration and its
// arguments — so its name, f#[T₁,…,Tₙ], determines it, and any reader of the
// name re-derives it (Instances): the IR's text names instances, and the
// printers rebuild them from the target's declarations alone (ADR 0038).
//
// The bound is membership: Tᵢ ∈ subsets(B) is ⟦Tᵢ⟧ ⊆ ⟦B⟧, the target's
// subsumption. Nothing after reduction learns that a declaration was
// polymorphic: an instance is an ordinary primitive (monomorphization).

import (
	"fmt"
	"sort"
	"strings"
)

// InstanceName is f#[T₁,…,Tₙ]. The `#` keeps a program from writing it, as it
// does the raw call's name (retract.go), and makes the IR's text quote it.
func InstanceName(f string, args []string) string {
	return f + "#[" + strings.Join(args, ",") + "]"
}

// splitInstance inverts InstanceName.
func splitInstance(n string) (string, []string, bool) {
	i := strings.Index(n, "#[")
	if i <= 0 || !strings.HasSuffix(n, "]") {
		return "", nil, false
	}
	return n[:i], strings.Split(n[i+2:len(n)-1], ","), true
}

// IsSet reports whether n denotes a set that step 1 makes a static value: a
// type the target declares (spec/typevalues.md §1).
func (tg *Target) IsSet(n string) bool {
	_, ok := tg.Types[n]
	return ok
}

// oneNamespace refuses a name a module declares both as a type and as a value
// (spec/typevalues.md §2): with types as static values a name denotes one thing.
// A companion module is no value; its path is written with `/`.
func (tg *Target) oneNamespace() error {
	var both []string
	for q := range tg.Types {
		_, prim := tg.Prims[q]
		def := false
		for _, f := range tg.Defs[moduleOf(q)] {
			def = def || (f.Kind == "def" && f.Name == baseOf(q))
		}
		if prim || def {
			both = append(both, q)
		}
	}
	if len(both) == 0 {
		return nil
	}
	sort.Strings(both)
	return fmt.Errorf("%s is declared as a type and as a value in one module; a type is a value of the static "+
		"level, so a name denotes one thing. A struct literal is its type's introduction rule and is "+
		"declared in the type's companion module as `new` (spec/typevalues.md §2)", strings.Join(both, ", "))
}

// Instantiate makes f's instance at args, registers it, and returns its name.
func (tg *Target) Instantiate(f string, args []string) (string, error) {
	p, ok := tg.Prims[f]
	if !ok || len(p.Static) == 0 {
		return "", fmt.Errorf("%s takes no static parameters, so it is not applied to a set", spelled(f))
	}
	if len(args) != len(p.Static) {
		return "", fmt.Errorf("%s takes %d static argument%s, a set for each of %s; it is applied to %d",
			spelled(f), len(p.Static), plural(len(p.Static)), staticNames(p.Static), len(args))
	}
	name := InstanceName(f, args)
	if _, have := tg.Prims[name]; have {
		return name, nil
	}
	sigma := map[string]string{}
	for i, sp := range p.Static {
		a := args[i]
		if !tg.IsSet(a) {
			return "", fmt.Errorf("%s's static parameter %s is given %s, which is not a set this target "+
				"declares (spec/typevalues.md §1)", spelled(f), sp.Name, a)
		}
		// THE BOUND IS MEMBERSHIP: ⟦a⟧ ∈ 𝒫(⟦B⟧) iff ⟦a⟧ ⊆ ⟦B⟧.
		if sp.SubsetOf != "" && !tg.Subsumes(a, sp.SubsetOf) {
			return "", fmt.Errorf("%s applied to %s: its static parameter %s ranges over (subsets %s), and "+
				"%s is not a subset of %s — the target declares no (implements %s %s) (spec/typevalues.md §5)",
				spelled(f), a, sp.Name, sp.SubsetOf, a, sp.SubsetOf, a, sp.SubsetOf)
		}
		sigma[sp.Name] = a
	}
	sub := func(ty string) string {
		return mapTypeNames(ty, func(n string) string {
			if a, ok := sigma[n]; ok {
				return a
			}
			return n
		})
	}
	q := p
	q.Name, q.Static = name, nil
	q.TypeArgs = append([]string(nil), args...)
	q.Args = mapEach(p.Args, sub)
	q.Results = mapEach(p.Results, sub)
	if p.Result != "" {
		q.Result = sub(p.Result)
	}
	q.TypeImports = nil
	for i, a := range args {
		q.Form = strings.ReplaceAll(q.Form, fmt.Sprintf("%%t%d", i+1), tg.ty(a))
		if imp := hostPackage(a); imp != "" && imp != p.Import && !contains(q.TypeImports, imp) {
			q.TypeImports = append(q.TypeImports, imp)
		}
	}
	if strings.Contains(q.Form, "%t") {
		return "", fmt.Errorf("%s's template %q has a type hole beyond its %d static parameter%s",
			spelled(f), p.Form, len(p.Static), plural(len(p.Static)))
	}
	tg.Prims[name] = q
	tg.Names = append(tg.Names, name)
	sort.Strings(tg.Names)
	return name, nil
}

// Instances re-derives every instance the names name (ADR 0038: a printer reads
// the IR's text and the target's declarations, and an instance's name is all of
// its declaration that the text needs).
func (tg *Target) Instances(names []string) error {
	for _, n := range names {
		if _, have := tg.Prims[n]; have {
			continue
		}
		if f, args, ok := splitInstance(n); ok {
			if _, err := tg.Instantiate(f, args); err != nil {
				return err
			}
		}
	}
	return nil
}

// hostPackage is the host package a type's module is: ι(p) = HOST ++ p
// (ADR 0025), so `go/strconv.NumError` is in `strconv`, and a root type or one
// of module `go` itself is in none.
func hostPackage(q string) string {
	mod := moduleOf(q)
	if i := strings.Index(mod, "/"); i >= 0 {
		return mod[i+1:]
	}
	return ""
}

func mapEach(xs []string, f func(string) string) []string {
	if xs == nil {
		return nil
	}
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func staticNames(ps []StaticParam) string {
	var ns []string
	for _, p := range ps {
		ns = append(ns, p.Name)
	}
	return strings.Join(ns, ", ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
