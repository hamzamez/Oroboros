package emit

import (
	"fmt"
	"sort"

	"oroboros/core"
)

// A CONSTANT'S NAME IS A RANGE ENDPOINT — theories.md §8.3, step 4 of the build
// order in theories-b-or-c.md §9.
//
// `(const MaxRune 1114111 (host "utf8.MaxRune"))` elaborates to a declaration
// with a DEFINIENS: `MaxRune : int [1114111,1114111] = 1114111 ↦ "utf8.MaxRune"`.
// The definiens is a compile-time integer, and an endpoint is a compile-time
// integer expression, so `(int 0 utf8.MaxRune)` is δ at the type level —
// substitute the definiens and evaluate. Nothing else changes: `endpoint` still
// sees literals, `KInt` is still an `int64`, and the canonical type string a
// declaration elaborates to is the same one someone writing the digits gets.
//
// IT IS WORTH HAVING BECAUSE A VALUE WRITTEN TWICE IS TWO CLAIMS THAT CAN
// DISAGREE — constdecl-2026-09-15's own reason for `const`, one level up.
// `unicode/utf8.oro` declared `MaxRune` and then wrote `(int 0 1114111)` in four
// result types, with a comment saying "0..MaxRune" beside the digits.
//
// THE ENVIRONMENT IS THE WHOLE TARGET, WHICH IS WHY RESOLUTION IS DEFERRED. A
// constant may be declared in another file or another layer, and a file is a
// FRAGMENT that is glued — `load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂)` — so resolving
// against the file being read would make splitting a file change a target, which
// is the property loader-2026-09-15 exists to keep. So a declaration whose type
// names a constant is carried UNPARSED through the glue and elaborated once the
// target is whole, next to the other passes that need the finished signature
// (companions, views).
//
// A DEFERRED DECLARATION IS GLUED, NEVER OVERRIDDEN, and that is a stated limit
// rather than an oversight: it is elaborated after the layers have been folded,
// so it no longer knows which layer it came from, and a name it collides with is
// REFUSED naming both rather than resolved by an order that is no longer there.
// Writing the digits is the way out, and nothing in the corpus needs it.
type deferred struct {
	form *core.Term
	mod  string
	file string
}

// namesEndpoint reports whether a declaration's types mention a constant, which
// is the one thing that makes elaboration need the finished target.
func namesEndpoint(f *core.Term) bool {
	for _, ty := range declTypes(f) {
		if namedIn(ty) {
			return true
		}
	}
	return false
}

// declTypes is the TYPE POSITIONS of a declaration — kids 2 and 3 of
// `(sig NAME ARGS RESULT clause… (host …))`, the same two primOf reads.
//
// THE POSITIONS ARE NEEDED BECAUSE THE GRAMMAR IS AMBIGUOUS WITHOUT THEM, which
// is typeargs-2026-09-15's finding in a third place. `(int 0 255)` is a range and
// `(int int string)` is an ARGUMENT LIST of three types — both are applications
// headed by `int` — so a walk that looked for the shape anywhere read a windows
// declaration's arguments as a range over a constant called `string`. Inside a
// type there is no ambiguity, so the scan below is free to walk everything.
func declTypes(f *core.Term) []*core.Term {
	if formWord(f) != "sig" || f.Kind != core.KApp || len(f.Kids) < 5 {
		return nil // const takes a literal; structural carries no types
	}
	out := []*core.Term{f.Kids[3]} // the result
	args := f.Kids[2]
	if args.Kind != core.KApp {
		return out
	}
	for _, a := range args.Kids {
		out = append(out, paramType(a))
	}
	return out
}

// paramType is the type of one argument: `τ`, or the `τ` of `(x τ)`.
//
// A NAMED PARAMETER IS THE ONE WHOSE HEAD IS NOT A TYPE FORMER. `(i int)` is a
// name and a type; `(array int)` and `(int 0 255)` are types — which is exactly
// how primOf reads the same list, so the two cannot disagree about which is
// which.
func paramType(a *core.Term) *core.Term {
	if a.Kind == core.KApp && len(a.Kids) == 2 && a.Kids[0].Kind == core.KName &&
		!core.IsTypeFormer(a.Kids[0].Name) {
		return a.Kids[1]
	}
	return a
}

// namedIn walks a TYPE for an endpoint that is a name. Every kind is walked,
// because `(tuple A B)` arrives as the Church term `(fn (#k) (#k A B))` — the
// reader is context-free and a tuple type is the term a tuple VALUE is
// (data.md §3.4).
func namedIn(t *core.Term) bool {
	if t == nil {
		return false
	}
	if isRange(t) {
		for _, e := range t.Kids[1:] {
			if constName(e) != "" {
				return true
			}
		}
	}
	for _, k := range t.Kids {
		if namedIn(k) {
			return true
		}
	}
	return false
}

func isRange(t *core.Term) bool {
	return t != nil && t.Kind == core.KApp && len(t.Kids) == 3 &&
		t.Kids[0].Kind == core.KName && t.Kids[0].Name == "int"
}

// constName is the name an endpoint stands for, or "" when it is not one.
// `+inf` and `-inf` are endpoints of the grammar itself (unbounded-rung.md §1),
// not constants, and they keep their meaning.
func constName(t *core.Term) string {
	if t == nil || t.Kind != core.KName || t.Name == "+inf" || t.Name == "-inf" {
		return ""
	}
	return t.Name
}

// substEndpoints rewrites a declaration, replacing every constant name in a
// range endpoint of one of its TYPES by the definiens. It is δ at the type
// level, and it is the whole of the feature: `endpoint` still sees literals.
func (tg *Target) substEndpoints(f *core.Term, mod, file string) (*core.Term, error) {
	if formWord(f) != "sig" || f.Kind != core.KApp || len(f.Kids) < 5 {
		return f, nil
	}
	var err error
	sub := func(t *core.Term) *core.Term {
		if err != nil {
			return t
		}
		var s *core.Term
		s, err = tg.substIn(t, mod, file)
		return s
	}
	out := *f
	out.Kids = append([]*core.Term(nil), f.Kids...)
	out.Kids[3] = sub(f.Kids[3])
	if args := f.Kids[2]; args.Kind == core.KApp {
		na := *args
		na.Kids = append([]*core.Term(nil), args.Kids...)
		for i, a := range args.Kids {
			if ty := paramType(a); ty != a {
				np := *a
				np.Kids = []*core.Term{a.Kids[0], sub(ty)}
				na.Kids[i] = &np
				continue
			}
			na.Kids[i] = sub(a)
		}
		out.Kids[2] = &na
	}
	return &out, err
}

// substIn is the walk inside a type, where `(int A B)` is unambiguously a range.
func (tg *Target) substIn(t *core.Term, mod, file string) (*core.Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	out := *t
	out.Kids = append([]*core.Term(nil), t.Kids...)
	if isRange(t) {
		for i := 1; i < 3; i++ {
			n := constName(out.Kids[i])
			if n == "" {
				continue
			}
			v, err := tg.constValue(mod, n, file)
			if err != nil {
				return nil, err
			}
			out.Kids[i] = core.Int(v)
		}
	}
	for i, k := range out.Kids {
		s, err := tg.substIn(k, mod, file)
		if err != nil {
			return nil, err
		}
		out.Kids[i] = s
	}
	return &out, nil
}

// constValue reads a constant's definiens off its declaration.
//
// THE SINGLETON RANGE IS THE DEFINIENS, which is why nothing has to record that
// a declaration came from `const`: a pure operation of no arguments whose result
// is `(int v v)` denotes v and can denote nothing else, so the type already says
// what the value is. `const` is the sugar that writes it once instead of twice.
//
// Resolution is a term's: the module's own name first, then the bare one — the
// rule theories.md §3.4 states for types, and the same one `Module.resolve`
// applies inside a program.
func (tg *Target) constValue(mod, name, file string) (int64, error) {
	for _, q := range []string{qualify(mod, name), name} {
		p, ok := tg.Prims[q]
		if !ok {
			continue
		}
		lo, hi, ok := core.IntRange(p.Result)
		if !ok || lo != hi || len(p.Args) > 0 || len(p.Results) > 0 || !p.Pure {
			return 0, fmt.Errorf("%s: %s is a range endpoint, so it must be a CONSTANT — a pure "+
				"declaration of no arguments whose result is an exact range, which is what "+
				"(const %s v (host …)) elaborates to; %s is declared %s", file, name, name, q, describePrim(p))
		}
		return lo, nil
	}
	return 0, fmt.Errorf("%s: %s is not declared in this target, and a range endpoint that is a name "+
		"must be a constant — (const %s v (host \"spelling\")) — because an endpoint is evaluated at "+
		"compile time (theories.md §8.3)", file, name, name)
}

func describePrim(p Prim) string {
	r := p.Result
	if len(p.Results) > 0 {
		r = "tuple"
	}
	s := fmt.Sprintf("with %d argument(s) returning %s", len(p.Args), r)
	if !p.Pure {
		s += ", and it is not pure"
	}
	return s
}

// resolveDeferred elaborates every declaration that was waiting for a constant,
// against the target as it now stands. It runs before `addCore` and before the
// companion expansion, so a deferred declaration is an ordinary member of its
// module by the time anything reads one.
func (tg *Target) resolveDeferred() error {
	d := tg.Deferred
	tg.Deferred = nil
	for _, dd := range d {
		f, err := tg.substEndpoints(dd.form, dd.mod, dd.file)
		if err != nil {
			return err
		}
		if err := tg.declare(f, dd.mod, dd.file); err != nil {
			return err
		}
	}
	if len(d) > 0 {
		sort.Strings(tg.Names)
	}
	return nil
}
