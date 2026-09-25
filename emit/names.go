package emit

import (
	"fmt"
	"sort"
	"strings"
)

// A NAME INSIDE A MODULE IS RESOLVED LEXICALLY — theories.md §3.4, decided by
// ADR 0021 (item 2: "names resolve lexically; type names are resolved like term
// names") and built for target files by names-2026-09-25. Until then a type in a
// signature was taken verbatim, so `NumError` inside `go/strconv` had to be
// written `go/strconv.NumError`, and a constant resolved one way and a type
// another.
//
// THE SCOPES ARE THE PATH'S PREFIXES. A name used in module `p` is `p.N`, then
// the same in each enclosing module, then bare. Enclosing is read off the path,
// never off the nesting, so `(module m (module T …))` and `(module m/T …)` —
// one target by concatenation (target-files.md §1a) — resolve alike.

// scopes is a module path followed by each of its enclosing paths, ending at the
// root, "".
func scopes(mod string) []string {
	out := []string{mod}
	for mod != "" {
		if i := strings.LastIndex(mod, "/"); i >= 0 {
			mod = mod[:i]
		} else {
			mod = ""
		}
		out = append(out, mod)
	}
	return out
}

// resolveIn is the one resolution rule: a qualified name as written, a bare one
// in the innermost scope that declares it. It reports whether any scope did.
func resolveIn(mod, n string, has func(string) bool) (string, bool) {
	if strings.Contains(n, ".") {
		return n, has(n)
	}
	for _, s := range scopes(mod) {
		if q := qualify(s, n); has(q) {
			return q, true
		}
	}
	return n, false
}

func (tg *Target) hasType(n string) bool {
	if _, ok := tg.Types[n]; ok {
		return true
	}
	_, ok := tg.Aliases[n]
	return ok
}

// resolveTypeNames resolves every type name a declaration writes, on the GLUED
// target: a type declared in one file or layer is named bare from another, and
// splitting a file changes nothing. It runs after the constants and before the
// aliases unfold, so that pass meets only qualified names.
//
// SHADOWING IS REFUSED FIRST, and naming both. Lexical scoping would let a type
// added to a parent rebind every bare use in its children in silence — the
// hazard nestmod-2026-09-20 named when it deferred this. A module type named
// like one in an enclosing module, like a root-level type or like a language
// type is an error; siblings (go/io.Reader, go/bufio.Reader) are two names.
func (tg *Target) resolveTypeNames() error {
	var owned []string
	for n := range tg.Types {
		owned = append(owned, n)
	}
	for n := range tg.Aliases {
		owned = append(owned, n)
	}
	sort.Strings(owned) // deterministic diagnostics
	for _, n := range owned {
		mod, base := moduleOf(n), baseOf(n)
		if mod == "" {
			continue
		}
		if langTypes[base] {
			return fmt.Errorf("type %s shadows the language's own %s; a module type may not take a "+
				"name its enclosing scopes already give a type (theories.md §3.4)", n, base)
		}
		for _, s := range scopes(mod)[1:] {
			if q := qualify(s, base); tg.hasType(q) {
				return fmt.Errorf("type %s shadows %s: a bare %s below %s would change meaning, so a "+
					"module type may not take a name an enclosing scope already gives a type "+
					"(theories.md §3.4); rename one, or name the other by its path", n, q, base, mod)
			}
		}
	}
	resolve := func(mod string) func(string) string {
		return func(m string) string {
			q, _ := resolveIn(mod, m, tg.hasType)
			return q
		}
	}
	for n, p := range tg.Prims {
		r := resolve(moduleOf(n))
		for i, a := range p.Args {
			p.Args[i] = mapTypeNames(a, r)
		}
		for i, x := range p.Results {
			p.Results[i] = mapTypeNames(x, r)
		}
		if p.Result != "" {
			p.Result = mapTypeNames(p.Result, r)
		}
		tg.Prims[n] = p
	}
	for n, ty := range tg.Aliases {
		tg.Aliases[n] = mapTypeNames(ty, resolve(moduleOf(n)))
	}
	return nil
}
