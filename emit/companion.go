package emit

import (
	"fmt"
	"sort"
	"strings"
)

// COMPANIONS, `include`, AND WHY AN EDGE IS DERIVED RATHER THAN DECLARED
// (spec/theories.md §3.3, §6.1, §6.2).
//
// A TYPE'S COMPANION is the child module sharing its name: the operations on
// `go/io.Writer` are declared in module `go/io/Writer`. That is how a host type's
// methods are written, and it is the only way an INTERFACE gets any — which is
// the wall `encoding/hex` recorded: `Close` on an `io.WriteCloser` was
// unreachable, so the acceptance program's dump was missing exactly what `Close`
// would write (hex-2026-09-14 §6).
//
// `include` IS THEORY INCLUSION between companions. `io.WriteCloser` is DEFINED
// as `interface { Writer; Closer }`, so its companion includes theirs, and every
// declaration of the included companion becomes one of the includer's with its
// receiver retyped.
//
// AND THEN THE SUBTYPING IS A THEOREM, not a claim. `T ≤ I` iff
// `methods(I) ⊆ methods(T)` — the powerset lattice under reverse inclusion,
// Cardelli's record subtyping with method sets (interfaces.md §2) — and an
// inclusion is exactly that containment. So the edge is DERIVED from the
// declarations, where `(implements io-WriteCloser io-Writer)` had been written by
// hand beside them.

// companionOf is the module that holds a type's operations: `go/io.Writer` has
// companion `go/io/Writer`. The last `.` separates a module path from a member,
// so the companion is that member as a child module.
func companionOf(ty string) (string, bool) {
	i := strings.LastIndex(ty, ".")
	if i <= 0 || i == len(ty)-1 {
		return "", false
	}
	return ty[:i] + "/" + ty[i+1:], true
}

// typeOfCompanion inverts it: `go/io/Writer` is the companion of `go/io.Writer`.
func typeOfCompanion(mod string) (string, bool) {
	i := strings.LastIndex(mod, "/")
	if i <= 0 || i == len(mod)-1 {
		return "", false
	}
	return mod[:i] + "." + mod[i+1:], true
}

// expandCompanions resolves every `include` and derives the subtyping edges it
// entails. It runs once on the merged target, because an included companion may
// be declared in another file or another layer — the same reason `addCore` runs
// at the end (layers-2026-09-07).
func (tg *Target) expandCompanions() error {
	if len(tg.Includes) == 0 {
		return nil
	}
	mods := make([]string, 0, len(tg.Includes))
	for m := range tg.Includes {
		mods = append(mods, m)
	}
	sort.Strings(mods) // a total order: the emitter is a function of its input
	done := map[string]bool{}
	var expand func(mod string, path []string) error
	expand = func(mod string, path []string) error {
		if done[mod] {
			return nil
		}
		for _, p := range path {
			if p == mod {
				return fmt.Errorf("companion %s includes itself, through %s; the declarations of a "+
					"program admit a well-founded order (theories.md §1.3)", mod, strings.Join(path, " → "))
			}
		}
		ty, ok := typeOfCompanion(mod)
		if !ok || tg.Types[ty] == "" {
			return fmt.Errorf("(include …) in module %s, which is not a companion: it names no type %s "+
				"declared by its parent (theories.md §3.3)", mod, ty)
		}
		for _, inc := range tg.Includes[mod] {
			incTy, ok := typeOfCompanion(inc)
			if !ok || tg.Types[incTy] == "" {
				return fmt.Errorf("%s includes %s, which is not a companion: no type %s is declared",
					mod, inc, incTy)
			}
			if err := expand(inc, append(path, mod)); err != nil {
				return err
			}
			// THE DECLARATIONS, with the receiver retyped. A method of `Writer`
			// applies to a `WriteCloser` because a WriteCloser IS one, and the
			// host's own call syntax is the template either way.
			for _, name := range namesUnder(tg, inc) {
				local := name[len(inc)+1:]
				into := mod + "." + local
				if _, have := tg.Prims[into]; have {
					continue // the includer's own declaration wins: override, not glue
				}
				p := tg.Prims[name]
				p.Name = into
				p.Args = append([]string(nil), p.Args...)
				for i, a := range p.Args {
					if a == incTy {
						p.Args[i] = ty
					}
				}
				tg.Prims[into] = p
				tg.Names = append(tg.Names, into)
			}
			// AND THE EDGE, derived: methods(I) ⊆ methods(T) is what `T ≤ I` says.
			if tg.Implements == nil {
				tg.Implements = map[string][]string{}
			}
			if !contains(tg.Implements[ty], incTy) {
				tg.Implements[ty] = append(tg.Implements[ty], incTy)
			}
		}
		done[mod] = true
		return nil
	}
	for _, m := range mods {
		if err := expand(m, nil); err != nil {
			return err
		}
	}
	return nil
}

// namesUnder is every declaration of one module, in a total order.
func namesUnder(tg *Target, mod string) []string {
	var out []string
	for n := range tg.Prims {
		if strings.HasPrefix(n, mod+".") && !strings.Contains(n[len(mod)+1:], ".") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// checkViews is §6.1: an `implements` edge is a VIEW from the interface's
// companion to the subject's, and a view is CHECKED rather than believed. For
// every term `m` the interface's companion declares, the subject's companion must
// declare `m` at a type at least as specific.
//
// An interface with no companion checks nothing, and that is not a gap: a
// generated survey declares thousands of edges and no interface methods, so there
// is nothing to compare. What the check refuses is a claim made where the
// declarations are present to contradict it.
func (tg *Target) checkViews() error {
	var subs []string
	for s := range tg.Implements {
		subs = append(subs, s)
	}
	sort.Strings(subs)
	for _, sub := range subs {
		subComp, ok := companionOf(sub)
		if !ok {
			continue
		}
		for _, iface := range tg.Implements[sub] {
			ifComp, ok := companionOf(iface)
			if !ok {
				continue
			}
			for _, m := range namesUnder(tg, ifComp) {
				local := m[len(ifComp)+1:]
				got, have := tg.Prims[subComp+"."+local]
				if !have {
					return fmt.Errorf("(implements %s %s) is false: %s declares %s and %s does not "+
						"(theories.md §6.1). An edge is a view, and a view is checked",
						sub, iface, iface, local, sub)
				}
				if err := tg.viewAgrees(sub, iface, local, tg.Prims[m], got); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// viewAgrees compares one method of the interface with the subject's, ignoring
// the receiver, which is the one argument the two are REQUIRED to differ in.
func (tg *Target) viewAgrees(sub, iface, local string, want, got Prim) error {
	if len(want.Args) != len(got.Args) {
		return fmt.Errorf("(implements %s %s): %s takes %d argument(s) on %s and %d on %s",
			sub, iface, local, len(want.Args), iface, len(got.Args), sub)
	}
	for i := range want.Args {
		if i == 0 {
			continue // the receiver
		}
		if !compatible(want.Args[i], got.Args[i]) && !tg.SameHostType(want.Args[i], got.Args[i]) {
			return fmt.Errorf("(implements %s %s): %s's argument %d is %q on %s and %q on %s",
				sub, iface, local, i+1, want.Args[i], iface, got.Args[i], sub)
		}
	}
	if !compatible(want.Result, got.Result) && !tg.SameHostType(want.Result, got.Result) {
		return fmt.Errorf("(implements %s %s): %s returns %q on %s and %q on %s",
			sub, iface, local, want.Result, iface, got.Result, sub)
	}
	return nil
}
