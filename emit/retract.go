package emit

// A FALLIBLE HOST CALL IS THE HOST'S PRODUCT COMPOSED WITH A RETRACTION
// (spec/errors.md §4, ADR 0040).
//
// A host does not give the coproduct. Go's `error` is 1 + E, its absent point
// `nil` inside its own values, a niche the target declares once:
//
//	(repr error (niche (host expr "%s == nil")))
//
// A declaration whose result is a marked variant, an option, or a tuple with
// one of them as a factor, means the host's product composed with a retraction
// r. For each declared result R, H(R) is the list of host values behind it and
// r rebuilds R from them, factor by factor:
//
//	H(T)                  = [T]               r = the value
//	H((tuple))            = []                r = (tuple)
//	H((option E))         = [E]               r(e) = none if niche(e), else (some e)
//	H((V T E))            = H(T) ++ [E]       r(t̄, e) = s (r t̄) if niche(e), else c e
//	H((tuple R₁ … Rₙ))    = H(R₁) ++ … ++ H(Rₙ)    r = (tuple r₁ … rₙ)
//
// where V is a variant marked (success s) with exactly one other constructor
// c, whose payload is E. H is a functor on result shapes and r is natural in
// each factor, so the construction composes: `ReadFile` is a sum, `Read` a
// tuple with an option factor, and both use the one niche.
//
// The loader realizes it by δ, nothing new: the declaration becomes the raw
// host call, #raw:MODULE:NAME, declaring H(R), and a definition NAME in the target's
// library (D_T) that applies r. Where the sum is eliminated in the same
// program, reduction removes it and the host's own `if err != nil` remains.
// A `#` cannot be written in a program, so the raw call, the host's product,
// is out of a program's reach: the product leak (errors-2026-10-04) is gone.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oroboros/core"
)

// needsRetraction reports whether a declared result is a variant, an option, or
// a tuple with one of them as a factor.
func needsRetraction(t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KApp && len(t.Kids) >= 2 && t.Kids[0].Kind == core.KName &&
		!typeFormer(t.Kids[0].Name) {
		return true
	}
	if comps, ok := tupleComponents(t); ok {
		for _, c := range comps {
			if needsRetraction(c) {
				return true
			}
		}
	}
	return false
}

func typeFormer(n string) bool {
	switch n {
	case "tuple", "array", "buffer", "map", "int":
		return true
	}
	return false
}

// tupleComponents reads `(tuple A B …)` as the reader leaves it, the Church
// term `(fn (#k) (#k A B …))`, and the unit `(fn (#k) (#k))`.
func tupleComponents(t *core.Term) ([]*core.Term, bool) {
	if t.Kind != core.KFn || len(t.Params) != 1 || !strings.HasPrefix(t.Params[0], "#k") {
		return nil, false
	}
	b := t.Body()
	if b.Kind != core.KApp || len(b.Kids) == 0 || b.Kids[0].Kind != core.KName || b.Kids[0].Name != t.Params[0] {
		return nil, false
	}
	return b.Kids[1:], true
}

// shape is a declared result as the retraction reads it.
type shape struct {
	ty      string   // a leaf: one host value of this type
	unit    bool     // (tuple): no host value
	elems   []*shape // a tuple
	option  string   // (option E): E's type
	payload *shape   // a variant: its success payload's shape
	succ    string   // a variant: the success constructor, as the definition spells it
	fail    string   // a variant: the other constructor
	err     string   // a variant: E's type
}

// retractions replaces every declaration with a Retract result (retract.go's
// header) by its raw host call and a definition in its module's library.
func (tg *Target) retractions() error {
	var names []string
	for n, p := range tg.Prims {
		if p.Retract != nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, q := range names {
		if err := tg.retract(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	return nil
}

func (tg *Target) retract(q string) error {
	p := tg.Prims[q]
	mod, local := q, q
	if i := strings.LastIndex(q, "."); i >= 0 {
		mod, local = q[:i], q[i+1:]
	}
	// The result's variant resolves where the declaration was written; an
	// included copy lives in another module, which then needs that module's
	// `use` for the definition core will resolve.
	home := p.RetractIn
	if home == "" {
		home = mod
	}
	sh, err := tg.shapeOf(p.Retract, home)
	if err != nil {
		return err
	}
	if home != mod {
		for _, f := range tg.Defs[home] {
			if f.Kind == "use" && !hasUse(tg.Defs[mod], f) {
				if tg.Defs == nil {
					tg.Defs = map[string][]core.Form{}
				}
				tg.Defs[mod] = append(tg.Defs[mod], f)
			}
		}
	}
	// The types derived from the result as written resolve in its module, as
	// every declaration's do (names.go); the declaration's own were resolved
	// before this pass.
	in := func(n string) string {
		r, _ := resolveIn(mod, n, tg.hasType)
		return r
	}
	sh.resolve(func(ty string) string { return mapTypeNames(ty, in) })
	host := sh.host()
	if len(host) == 0 {
		return fmt.Errorf("the declared result carries no host value")
	}
	// THE RAW CALL, the host's own product, under a name no program can write.
	raw := p
	raw.Name, raw.Retract = RawName(q), nil
	raw.Result, raw.Results = "", nil
	if len(host) == 1 {
		raw.Result = host[0]
	} else {
		raw.Results = host
	}
	delete(tg.Prims, q)
	tg.Prims[raw.Name] = raw
	tg.Names = replaceName(tg.Names, q, raw.Name)
	// The niche tests it needs, one primitive per type.
	for _, e := range sh.niches() {
		tmpl, ok := tg.Niches[e]
		if !ok {
			tmpl, ok = tg.Niches[baseOf(e)]
		}
		if !ok {
			return fmt.Errorf("its failure is a %s, which this target declares no niche for: "+
				"write (repr %s (niche (host expr \"…\"))) (spec/errors.md §4.1)", e, e)
		}
		n := nicheName(e)
		if _, have := tg.Prims[n]; !have {
			tg.Prims[n] = Prim{Name: n, Args: []string{e}, Result: "bool", Kind: "expr", Form: tmpl, Pure: true}
			tg.Names = append(tg.Names, n)
		}
	}
	sort.Strings(tg.Names)
	// THE DEFINITION: NAME applies r to the raw call's values.
	params := make([]string, len(p.Args))
	for i := range params {
		params[i] = fmt.Sprintf("#a%d", i+1)
	}
	args := make([]*core.Term, len(params))
	for i, a := range params {
		args[i] = core.Name(a)
	}
	call := &core.Term{Kind: core.KApp, Kids: append([]*core.Term{core.Name(raw.Name)}, args...)}
	hs := make([]string, len(host))
	for i := range hs {
		hs[i] = fmt.Sprintf("#h%d", i+1)
	}
	next := 0
	body := sh.build(hs, &next)
	var def *core.Term
	if len(hs) == 1 {
		def = core.App(core.Fn(hs, body), call)
	} else {
		def = core.App(call, core.Fn(hs, body))
	}
	if tg.Defs == nil {
		tg.Defs = map[string][]core.Form{}
	}
	tg.Defs[mod] = append(tg.Defs[mod], core.Form{Kind: "def", Name: local, Term: core.Fn(params, def)})
	return nil
}

// RawName and nicheName are the loader's own primitives. A `#` keeps a program
// from writing them, and they contain no `.`, because core reads a dotted name
// as an import's alias and any other name a module does not define as a
// primitive (core/reduce.go, resolve).
func RawName(q string) string { return "#raw:" + strings.ReplaceAll(q, ".", ":") }
func hasUse(fs []core.Form, u core.Form) bool {
	for _, f := range fs {
		if f.Kind == "use" && f.Name == u.Name && f.Alias == u.Alias {
			return true
		}
	}
	return false
}

func nicheName(ty string) string { return "#niche:" + strings.ReplaceAll(ty, ".", ":") }

// DeclaredName inverts RawName: the name a raw call was declared under, which is
// the host's own declaration and is checked against the host under that name
// (gauntlet/stdlib, TestHandDeclarationsAgreeWithTheHost). A module path has no
// `.`, so every `:` after the prefix was one.
func DeclaredName(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "#raw:") {
		return "", false
	}
	return strings.ReplaceAll(strings.TrimPrefix(raw, "#raw:"), ":", "."), true
}

// spelled is how a diagnostic names a primitive: a raw call by the declaration
// the program wrote, since `#raw:…` is the compiler's own name for it.
func spelled(name string) string {
	if d, ok := DeclaredName(name); ok {
		return d
	}
	return name
}

func replaceName(names []string, from, to string) []string {
	out := names[:0]
	for _, n := range names {
		if n != from {
			out = append(out, n)
		}
	}
	return append(out, to)
}

// resolve maps every type in the shape.
func (s *shape) resolve(f func(string) string) {
	if s.ty != "" {
		s.ty = f(s.ty)
	}
	if s.option != "" {
		s.option = f(s.option)
	}
	if s.err != "" {
		s.err = f(s.err)
	}
	for _, e := range s.elems {
		e.resolve(f)
	}
	if s.payload != nil {
		s.payload.resolve(f)
	}
}

// host is H(R): the host's values behind a declared result, in order.
func (s *shape) host() []string {
	switch {
	case s.unit:
		return nil
	case s.ty != "":
		return []string{s.ty}
	case s.option != "":
		return []string{s.option}
	case s.elems != nil:
		var out []string
		for _, e := range s.elems {
			out = append(out, e.host()...)
		}
		return out
	}
	return append(s.payload.host(), s.err)
}

func (s *shape) niches() []string {
	switch {
	case s.option != "":
		return []string{s.option}
	case s.elems != nil:
		var out []string
		for _, e := range s.elems {
			out = append(out, e.niches()...)
		}
		return out
	case s.payload != nil:
		return append(s.payload.niches(), s.err)
	}
	return nil
}

// build is r: the term that rebuilds the declared result from the host values
// hs[*next:], consuming them in H's order.
func (s *shape) build(hs []string, next *int) *core.Term {
	take := func() *core.Term {
		n := core.Name(hs[*next])
		*next++
		return n
	}
	niche := func(ty string, v *core.Term) *core.Term {
		return core.App(core.Name(nicheName(ty)), v)
	}
	switch {
	case s.unit:
		return core.Fn([]string{"#k"}, &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name("#k")}})
	case s.ty != "":
		return take()
	case s.option != "":
		e := take()
		return core.App(core.Name("if"), niche(s.option, e), core.Name("none"), core.App(core.Name("some"), e))
	case s.elems != nil:
		kids := []*core.Term{core.Name("#k")}
		for _, e := range s.elems {
			kids = append(kids, e.build(hs, next))
		}
		return core.Fn([]string{"#k"}, &core.Term{Kind: core.KApp, Kids: kids})
	}
	ok := s.payload.build(hs, next)
	e := take()
	return core.App(core.Name("if"), niche(s.err, e),
		core.App(core.Name(s.succ), ok), core.App(core.Name(s.fail), e))
}

// shapeOf reads a declared result in module mod.
func (tg *Target) shapeOf(t *core.Term, mod string) (*shape, error) {
	if comps, ok := tupleComponents(t); ok {
		if len(comps) == 0 {
			return &shape{unit: true}, nil
		}
		s := &shape{elems: []*shape{}}
		for _, c := range comps {
			e, err := tg.shapeOf(c, mod)
			if err != nil {
				return nil, err
			}
			s.elems = append(s.elems, e)
		}
		return s, nil
	}
	if t.Kind == core.KApp && len(t.Kids) >= 2 && t.Kids[0].Kind == core.KName && !typeFormer(t.Kids[0].Name) {
		head := t.Kids[0].Name
		if head == "option" {
			if len(t.Kids) != 2 {
				return nil, fmt.Errorf("(option E) takes one type")
			}
			return &shape{option: core.TypeName(t.Kids[1])}, nil
		}
		return tg.variantShape(head, t.Kids[1:], mod)
	}
	ty := core.TypeName(t)
	if ty == "" {
		return nil, fmt.Errorf("%s is not a type", t)
	}
	return &shape{ty: ty}, nil
}

// variantShape reads `(A.V T E)`: the variant V of the library module the
// module's `(use … as A)` names, marked (success s), with one other
// constructor c whose payload is E.
func (tg *Target) variantShape(head string, args []*core.Term, mod string) (*shape, error) {
	alias, vname := "", head
	if i := strings.LastIndex(head, "."); i >= 0 {
		alias, vname = head[:i], head[i+1:]
	}
	path := ""
	for _, f := range tg.Defs[mod] {
		last := f.Name
		if i := strings.LastIndex(last, "/"); i >= 0 {
			last = last[i+1:]
		}
		if f.Kind == "use" && (f.Alias == alias || (f.Alias == "" && last == alias)) {
			path = f.Name
		}
	}
	if path == "" {
		return nil, fmt.Errorf("its result names %s, and module %s has no (use …) for %s", head, mod, alias)
	}
	sum, err := tg.readVariant(path, vname)
	if err != nil {
		return nil, err
	}
	if sum.Success == "" {
		return nil, fmt.Errorf("variant %s declares no (success …) constructor, so it is not a "+
			"model of the exception monad (spec/errors.md §2)", head)
	}
	if len(args) != len(sum.Params) {
		return nil, fmt.Errorf("(%s …) takes %d type arguments, not %d", head, len(sum.Params), len(args))
	}
	sigma := map[string]*core.Term{}
	for i, p := range sum.Params {
		sigma[p] = args[i]
	}
	instance := func(payload string) (*core.Term, bool) {
		if a, ok := sigma[payload]; ok {
			return a, true
		}
		if payload == "" {
			return nil, false
		}
		return core.Name(payload), true
	}
	s := &shape{succ: alias + "." + sum.Success}
	var others []core.Variant
	for _, v := range sum.Variants {
		if v.Name == sum.Success {
			pt, ok := instance(v.Payload)
			if !ok {
				s.payload = &shape{unit: true}
				continue
			}
			ps, err := tg.shapeOf(pt, mod)
			if err != nil {
				return nil, err
			}
			s.payload = ps
			continue
		}
		others = append(others, v)
	}
	if len(others) != 1 {
		return nil, fmt.Errorf("variant %s has %d constructors besides its success; a host's "+
			"failure is one value, so a retraction needs exactly one (spec/errors.md §4.1)", head, len(others))
	}
	et, ok := instance(others[0].Payload)
	if !ok {
		return nil, fmt.Errorf("variant %s's failure constructor %s carries nothing, so it cannot "+
			"carry the host's error", head, others[0].Name)
	}
	s.fail = alias + "." + others[0].Name
	s.err = core.TypeName(et)
	return s, nil
}

// readVariant finds variant name in the library module at path.
func (tg *Target) readVariant(path, name string) (*core.Sum, error) {
	for _, d := range tg.LibDirs {
		b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(path)+".oro"))
		if err != nil {
			continue
		}
		forms, err := core.Read(string(b))
		if err != nil {
			return nil, err
		}
		for _, f := range forms {
			if f.Kind == "sum" && f.Sum != nil && f.Sum.Name == name {
				return f.Sum, nil
			}
		}
		return nil, fmt.Errorf("library module %s declares no variant %s", path, name)
	}
	return nil, fmt.Errorf("library module %s is not on the library path %v", path, tg.LibDirs)
}
