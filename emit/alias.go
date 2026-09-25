package emit

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
)

// A MANIFEST TYPE IS A TYPE WITH A DEFINIENS — theories.md §2's four cells at the
// TYPE level, and the other half of step 4 of the build order.
//
//	(type rune (int -2147483648 2147483647))
//
// §2 classifies a declaration by whether it has a definiens `t` and whether it
// has a realization `ρ`. At the term level the four cells are a `sig`, a `def`, a
// `prim` and a constant. At the TYPE level they are the same four, and only two
// were built: `(type NAME (host "spelling"))` is the ABSTRACT type — a name this
// language can hold and cannot look inside — and `(type NAME τ)` is the MANIFEST
// one, a name that MEANS something already writable.
//
// UNFOLDING IS δ, exactly as it is for a definition, and it erases the name: a
// declaration written `((r rune))` is the declaration written
// `((r (int -2147483648 2147483647)))`, so no backend, no analysis and no
// generator learns that aliases exist. That is why an alias may not also carry a
// realization — after δ the name is gone, so a host spelling attached to it could
// never be reached, and a claim nothing can use is a claim nothing checks.
//
// IT IS UNFOLDED AFTER THE GLUE, for constend.go's reason: the alias may be
// declared in another file or another layer. Here it costs no deferral, because
// a canonical type is a string with a small grammar and the rewrite is a walk
// over it — which is also what makes the invariant checkable: after the pass no
// alias name occurs in any declaration, and the pass says so by refusing a cycle.

// aliasOf reads `(type NAME τ)` — the definiens form — returning the canonical
// spelling of τ.
func aliasOf(f *core.Term, path string) (string, error) {
	ty := core.TypeTerm(f.Kids[2])
	switch {
	case f.Kids[2].Kind == core.KStr:
		// A HOST SPELLING GOES INSIDE (host …), theories.md §5.2 — the one clause
		// that carries host text. Without that rule "no host clause" would stop
		// meaning "no claim about any host".
		return "", fmt.Errorf("%s: (type %s \"…\"): a host spelling goes inside (host …) — write "+
			"(type NAME (host \"spelling\")); a bare τ is a manifest type", path, f.Kids[1])
	case ty == "":
		return "", fmt.Errorf("%s: (type %s τ) — τ must be a type, got %s", path, f.Kids[1], f.Kids[2])
	case core.IsProd(ty):
		// SEVERAL RESULTS ARE DECIDED AT THE DECLARATION, not after it: primOf
		// splits a `tuple` result into Prim.Results while reading the sig, and an
		// alias is unfolded later, so a name standing for a tuple would be one
		// result where the tuple it means is two. Writing the tuple says it.
		return "", fmt.Errorf("%s: (type %s %s): a manifest type may not stand for a tuple, because "+
			"several results are read from the declaration itself; write the tuple", path, f.Kids[1], ty)
	}
	return ty, nil
}

// mapTypeNames rewrites every NAME occurring in a canonical type, leaving the
// structure alone. The grammar is the one TypeName produces.
func mapTypeNames(ty string, f func(string) string) string {
	switch {
	case strings.HasPrefix(ty, "array "):
		return "array " + mapTypeNames(ty[len("array "):], f)
	case strings.HasPrefix(ty, "buffer "):
		return "buffer " + mapTypeNames(ty[len("buffer "):], f)
	case strings.HasPrefix(ty, "map "):
		if k, v, ok := core.MapTypes(ty); ok {
			return "map " + mapTypeNames(k, f) + " " + mapTypeNames(v, f)
		}
	case strings.HasPrefix(ty, "int "):
		return ty // a range: two endpoints, no names
	case core.IsProd(ty):
		parts := core.ProdTypes(ty)
		for i, p := range parts {
			parts[i] = mapTypeNames(p, f)
		}
		return "prod(" + strings.Join(parts, ", ") + ")"
	}
	if ctor, args, ok := core.Applied(ty); ok {
		for i, a := range args {
			args[i] = mapTypeNames(a, f)
		}
		return ctor + "(" + strings.Join(args, ", ") + ")"
	}
	return f(ty)
}

// unfoldAliases replaces every manifest type by what it means, everywhere a
// declaration states a type. It runs on the glued target, before the core is
// injected and before companions are expanded, so every later pass sees a target
// in which no alias name occurs.
func (tg *Target) unfoldAliases() error {
	if len(tg.Aliases) == 0 {
		return nil
	}
	for n := range tg.Aliases {
		if _, host := tg.Types[n]; host {
			return fmt.Errorf("%s is declared both as a manifest type and as a host type; "+
				"a type has one definition", n)
		}
	}
	names := make([]string, 0, len(tg.Aliases))
	for n := range tg.Aliases {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic diagnostics
	// δ TO A FIXPOINT, because an alias may be written in terms of another. It
	// terminates unless the definitions are circular, and a cycle is REFUSED
	// naming the chain — the same well-founded order theories.md §2 keeps for
	// definitions, one level up.
	var expand func(mod, n string, path []string) (string, error)
	expand = func(mod, n string, path []string) (string, error) {
		key, ty, ok := tg.alias(mod, n)
		if !ok {
			return n, nil
		}
		for _, p := range path {
			if p == key {
				return "", fmt.Errorf("manifest type %s is defined in terms of itself: %s",
					key, strings.Join(append(path, key), " -> "))
			}
		}
		var err error
		out := mapTypeNames(ty, func(m string) string {
			if err != nil {
				return m
			}
			var e string
			e, err = expand(moduleOf(key), m, append(path, key))
			return e
		})
		return out, err
	}
	for _, n := range names {
		if _, err := expand(moduleOf(n), baseOf(n), nil); err != nil {
			return err
		}
	}
	for n, p := range tg.Prims {
		mod := moduleOf(n)
		var err error
		sub := func(ty string) string {
			return mapTypeNames(ty, func(m string) string {
				if err != nil {
					return m
				}
				var e string
				e, err = expand(mod, m, nil)
				return e
			})
		}
		for i, a := range p.Args {
			p.Args[i] = sub(a)
		}
		for i, r := range p.Results {
			p.Results[i] = sub(r)
		}
		if p.Result != "" {
			p.Result = sub(p.Result)
		}
		if err != nil {
			return err
		}
		tg.Prims[n] = p
	}
	return nil
}

// alias resolves a type name the way a term name resolves, by the one rule
// (names.go): the module's own, each enclosing module's, then the bare one
// (theories.md §3.4).
func (tg *Target) alias(mod, n string) (string, string, bool) {
	k, ok := resolveIn(mod, n, func(k string) bool { _, ok := tg.Aliases[k]; return ok })
	if !ok {
		return "", "", false
	}
	return k, tg.Aliases[k], true
}

// moduleOf and baseOf split a qualified name. A module path may contain `/` and
// a member name may not contain `.`, so the last dot is the split.
func moduleOf(n string) string {
	if i := strings.LastIndex(n, "."); i > 0 {
		return n[:i]
	}
	return ""
}

func baseOf(n string) string {
	if i := strings.LastIndex(n, "."); i > 0 {
		return n[i+1:]
	}
	return n
}

// langTypes are the type words the LANGUAGE owns. A target may not give one a
// definiens, for booleans.md's reason one level up: a host does not get to say
// what `int` means.
var langTypes = map[string]bool{"int": true, "f64": true, "bool": true, "string": true}
