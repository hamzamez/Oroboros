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
// β is call-by-need: it refuses to substitute where doing so would duplicate
// runtime work, and binds instead. The criterion came from measurement, not
// taste — see gauntlet/results/wordcount-2026-08-14.md:
//
//   - Duplicating a *pure* expression costs nothing; the host's CSE hoists it.
//     A duplicated a[i] compiled to byte-identical machine code.
//   - Duplicating an *allocating* expression costs 615x on Go and is quadratic,
//     because no host can hoist an allocation.
//
// Effects are handled by a side condition on β, specified in docs/spec/effects.md:
// an impure argument is never substituted, but normalised and let-bound at the
// application site, in argument order, whatever its occurrence count. The three
// clauses deny contraction, exchange and weakening respectively, which is what
// g5 §5 called the ordering discipline.

type Program struct {
	Defs  map[string]*Term
	Order []string // definition order, for stable diagnostics

	// Exports are the program's entry points, fully qualified, in declaration
	// order. A backend emits one function per export, named after it — which is
	// what replaced naming them by position (modules.md §1).
	Exports []string

	// Sigs are declared signatures, by fully qualified name. A signature is a
	// claim about a name that is checked against BOTH the library's definition
	// and any target's native implementation — which is the one job no host
	// compiler can do, since the two live on different targets.
	Sigs map[string]*Sig

	// Sums are the declared sum types, by name. A sum is Σ over a finite index
	// set, and its VALUES are ordinary products of a tag and a payload — so this
	// table is consulted for exhaustiveness and for signatures, and by nothing
	// in the reducer (sums-research.md, docs/spec/sums.md).
	Sums map[string]*Sum

	// Unresolved are `use` paths that found no file on the search path. That is
	// not an error — it is how a target-provided module looks (modules.md §4) —
	// but it is also how a MISSPELLED path looks, and the two were
	// indistinguishable until the name failed to resolve much later.
	Unresolved []string
}

// Env is a program viewed through one target: which names reduce, and which are
// the irreducible floor.
type Env struct {
	Defs map[string]*Term
	Prim map[string]bool
	Pure map[string]bool // names whose APPLICATION is pure; see pureName
	Rec  map[string]bool // recursive definitions are never δ-reduced

	// unresolvedPaths carries Program.Unresolved through to diagnostics.
	unresolvedPaths map[string]bool
}

// SetUnresolved records the imports that found no file, for importHint.
func (e *Env) SetUnresolved(paths []string) {
	e.unresolvedPaths = map[string]bool{}
	for _, p := range paths {
		e.unresolvedPaths[p] = true
	}
}

func NewProgram() *Program {
	return &Program{Defs: map[string]*Term{}, Sigs: map[string]*Sig{}, Sums: map[string]*Sum{}}
}

// A module is a scope: a path, what it imports, what it exports, and what it
// defines. It is NOT a unit of compilation and NOT a unit of reduction — by the
// time Load returns, every module has been flattened into one qualified
// namespace and the reducer cannot tell modules existed (modules.md §5, R1).
//
// That flattening is the whole design. Resolution happens before reduction, so
// `P_T` and `Defs` are keyed the same way and the four cells still work.
type Module struct {
	Path    string
	Uses    map[string]string // alias -> module path
	Exports map[string]bool   // empty means "everything", pending signatures
	Defs    map[string]*Term  // unqualified name -> body, before resolution
	Order   []string
	Sigs    map[string]*Sig
	Sums    map[string]*Sum
}

// qualify joins a module path to a local name. The root module has no path, so
// a program that never says `(module …)` behaves exactly as before.

// bindName picks the name a call-by-need binding may safely use.
//
// Reduction's invariant is stated at the KFn case of normalize: it works on
// CLOSED bodies and never reopens one, so a bound variable can never be
// mistaken for a global. Call-by-need is the one place that breaks the
// invariant on purpose — when β declines to substitute, it puts the parameter's
// own NAME back into the body and wraps a `let` around it.
//
// That is safe only if δ will not unfold the name it puts back. It usually is,
// because `resolve` qualifies every definition with its module path. It is not
// in the MAIN module, where `qualify("", n)` leaves definitions bare: a
// parameter spelled like a top-level `def` in the program being compiled was
// replaced by that definition, silently and with no error.
//
//	(def d1 (array 7 8 9))
//	(def run (fn (k) ((fn (d1) (go.+ d1 d1)) (go.+ k 100))))
//	→ (let (go.+ k 100) (fn (d1) (go.+ (array 7 8 9) (array 7 8 9))))
//
// One occurrence substituted and compiled correctly; two occurrences reached
// this path and did not. Found by a JSON tree builder with a local `d1` and a
// document named `d1` (json-tree-2026-08-26).
func (e *Env) bindName(p string, params []string, used map[string]bool) string {
	clash := func(n string) bool {
		if used[n] {
			return true
		}
		if _, ok := e.Defs[n]; ok {
			return true
		}
		return false
	}
	if !clash(p) {
		return p
	}
	taken := map[string]bool{}
	for _, q := range params {
		taken[q] = true
	}
	for i := 1; ; i++ {
		n := fmt.Sprintf("%s%d", p, i)
		if !clash(n) && !taken[n] {
			return n
		}
	}
}

func qualify(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// A Resolver finds the source of a module by path. It returns found=false for a
// path the target provides rather than a library — `go/strings` has no file.
//
// This is the whole of multi-file support: resolution already turns every name
// into a fully qualified one, so loading more modules only means having more of
// them in scope. Nothing downstream can tell whether a module came from the
// entry file or from disk.
type Resolver func(path string) (src string, found bool, err error)

// Load reads one source with no file resolution — every module it uses must be
// declared in the same text, or provided by the target.
func Load(forms []Form) (*Program, []*Term, error) { return LoadWith(forms, nil) }

// LoadWith reads an entry source and follows its imports, transitively.
//
// Only the ENTRY file contributes entry points: a library's bare terms and
// exports are not the program's. That is what makes `(use …)` a dependency
// rather than an inclusion.
func LoadWith(forms []Form, resolve Resolver) (*Program, []*Term, error) {
	mods, entries, err := partition(forms)
	if err != nil {
		return nil, nil, err
	}
	entryPaths := map[string]bool{}
	for _, m := range mods {
		entryPaths[m.Path] = true
	}
	var unresolved []string

	// Fixpoint over imports. A module already in scope is never re-read, which
	// is also what terminates on a cycle.
	if resolve != nil {
		seen := map[string]bool{}
		for _, m := range mods {
			seen[m.Path] = true
		}
		for {
			var want string
			for _, m := range mods {
				for _, path := range m.Uses {
					if !seen[path] {
						want = path
						break
					}
				}
				if want != "" {
					break
				}
			}
			if want == "" {
				break
			}
			seen[want] = true
			src, found, err := resolve(want)
			if err != nil {
				return nil, nil, fmt.Errorf("use %s: %w", want, err)
			}
			if !found {
				unresolved = append(unresolved, want)
				continue // provided by the target, not by a library
			}
			sub, err := Read(src)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", want, err)
			}
			subMods, _, err := partition(sub)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", want, err)
			}
			declared := false
			var extra []string
			for _, m := range subMods {
				if m.Path == "" && len(m.Defs) == len(injected) {
					continue // the empty anonymous scope every file starts with
				}
				if m.Path == "" {
					return nil, nil, fmt.Errorf(
						"%s: a library file must declare (module %s) before its definitions", want, want)
				}
				// A library file declares exactly the module its path names.
				// Adding the extras made their visibility depend on LOAD ORDER:
				// `(use extra/helper)` failed on its own and succeeded after
				// `(use extra/pair)` had pulled in the file that declared both.
				// Meaning that depends on what else is in scope is the thing
				// qualified imports exist to prevent (modules.md §3).
				if m.Path != want {
					extra = append(extra, m.Path)
					continue
				}
				declared = true
				seen[m.Path] = true
				mods = append(mods, m)
			}
			switch {
			case !declared && len(extra) == 1:
				return nil, nil, fmt.Errorf("%s declares (module %s); a library file's module "+
					"must be the path that imports it", want, extra[0])
			case !declared:
				return nil, nil, fmt.Errorf("%s does not declare (module %s)", want, want)
			case len(extra) > 0:
				return nil, nil, fmt.Errorf("%s also declares (module %s); a library file declares "+
					"one module, and it is the one its path names — put %s in its own file, or its "+
					"members are reachable only after something else has imported this one",
					want, strings.Join(extra, ", "), extra[0])
			}
		}
	}
	// An `export` or a `sig` naming nothing was silently dropped, because both
	// were read off m.Order — the list of DEFINITIONS. A misspelled export left
	// a program with no entry points, and build then reported the absence of a
	// `main` it could see two lines above.
	for _, m := range mods {
		var noDef []string
		for n := range m.Exports {
			if _, ok := m.Defs[n]; !ok {
				noDef = append(noDef, n)
			}
		}
		if len(noDef) > 0 {
			sort.Strings(noDef)
			return nil, nil, fmt.Errorf("%s exports %s, which it does not define",
				modLabel(m.Path), strings.Join(noDef, ", "))
		}
		noDef = nil
		for n := range m.Sigs {
			if _, ok := m.Defs[n]; !ok {
				noDef = append(noDef, n)
			}
		}
		if len(noDef) > 0 {
			sort.Strings(noDef)
			return nil, nil, fmt.Errorf("%s declares (sig %s …) but does not define %s",
				modLabel(m.Path), noDef[0], strings.Join(noDef, ", "))
		}
	}

	// `case` expands here, before name resolution, and with EVERY module's sums
	// in scope. That is the reason it is not reader sugar like `match`: the
	// reader sees one file, and an error type is declared in another.
	sums := map[string]*Sum{}
	byVariant := map[string]*Sum{}
	for _, m := range mods {
		for _, sum := range m.Sums {
			sums[sum.Name] = sum
			for _, v := range sum.Variants {
				// The comparison is by NAME, not by pointer. Every module gets
				// its own copy of the language's injected `option` (newModule),
				// so pointer inequality would report `option` as clashing with
				// itself in a two-module program.
				if other, dup := byVariant[v.Name]; dup && other.Name != sum.Name {
					return nil, nil, fmt.Errorf("%s is a variant of both %s and %s; a "+
						"constructor names one sum", v.Name, other.Name, sum.Name)
				}
				byVariant[v.Name] = sum
			}
		}
	}
	for _, m := range mods {
		for n, body := range m.Defs {
			x, err := expandCase(body, sums, byVariant)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", qualify(m.Path, n), err)
			}
			m.Defs[n] = x
		}
	}
	// AND OVER BARE ENTRY TERMS, which this loop did not reach. A `case` inside
	// a `(def …)` expanded and the identical `case` written at top level did
	// not — it stayed an application of an unbound name and reduced to a stuck
	// term rather than an error. A construct that works in one position and
	// silently does not in another is the shape of bug this repository keeps
	// finding; every real program puts its code in a `def`, which is why it was
	// never seen.
	for i := range entries {
		x, err := expandCase(entries[i].term, sums, byVariant)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", modLabel(entries[i].mod.Path), err)
		}
		entries[i].term = x
	}

	// A SUM IN A SIGNATURE is the product of its tag and its payload, which is
	// the whole representation story: `(sig f ((n int)) result)` becomes two
	// results, and two results are already Go's multiple return, Java's record,
	// JavaScript's object and x86's rax/rdx (values.md). Nothing new is emitted
	// on any target, and Go's own `(T, error)` idiom IS this shape.
	for _, m := range mods {
		for n, sig := range m.Sigs {
			sum, ok := sums[sig.Result]
			if !ok || len(sig.Results) > 0 {
				continue
			}
			payload, uniform := sum.uniformPayload()
			if !uniform {
				return nil, nil, fmt.Errorf("%s returns %s, whose variants carry different "+
					"payload types. A sum CROSSING A BOUNDARY is transmitted as its tag and "+
					"its payload, so the payload needs one type; inside a program a mixed sum "+
					"is fine, because reduction removes it", qualify(m.Path, n), sum.Name)
			}
			sig.Result = ""
			sig.Results = []string{"int", payload}
		}
	}

	p := NewProgram()
	sort.Strings(unresolved)
	p.Unresolved = unresolved
	p.Sums = sums
	for _, m := range mods {
		for _, n := range m.Order {
			body, err := m.resolve(m.Defs[n], map[string]bool{}, mods)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", qualify(m.Path, n), err)
			}
			q := qualify(m.Path, n)
			if _, dup := p.Defs[q]; dup {
				return nil, nil, fmt.Errorf("%s is defined twice", q)
			}
			p.Defs[q] = body
			p.Order = append(p.Order, q)
		}
	}
	for _, m := range mods {
		for n, sig := range m.Sigs {
			p.Sigs[qualify(m.Path, n)] = sig
		}
	}
	for _, m := range mods {
		if !entryPaths[m.Path] {
			continue // a library's exports are not the program's entry points
		}
		for _, n := range m.Order {
			if m.Exports[n] {
				p.Exports = append(p.Exports, qualify(m.Path, n))
			}
		}
	}
	terms := make([]*Term, 0, len(entries))
	for _, e := range entries {
		t, err := e.mod.resolve(e.term, map[string]bool{}, mods)
		if err != nil {
			return nil, nil, err
		}
		terms = append(terms, t)
	}
	return p, terms, nil
}

type entry struct {
	mod  *Module
	term *Term
}

// OptionSum is the language's own `option`, and it is INJECTED into every
// module rather than declared by one.
//
// A map read is `(option V)` (maps.md §4), and the compiler is what produces
// it — from β-tab on a literal, or from the host's own fallible read at a
// boundary. So `some` and `none` have to resolve in whatever module the read
// occurs in, and a program cannot be asked to import them any more than it can
// be asked to import `if`.
//
// It is an ORDINARY sum, so nothing downstream learns that maps exist: the
// constructors are definitions, and qualification, imports, δ, the occurrence
// counter and `case`'s reader desugaring all apply unchanged (sums.md).
//
// The Church encoding is polymorphic in the payload — `some = λp.λk. k 0 p` —
// so one declaration serves every V, and reduction has already made the term
// monomorphic by the time anything asks.
//
// The payload type is written `any` because a sum's variant carries a type NAME
// and the language has no type variables. Nothing checks it: reduction inlines
// the constructor and the checker sees the payload's own type.
func OptionSum() *Sum {
	return &Sum{Name: "option", Variants: []Variant{
		{Name: "some", Payload: "any"},
		{Name: "none"},
	}}
}

// newModule makes a module with the language's own declarations already in it.
// injected is what every module gets for free: the option sum's constructors
// and their tags. Named so that "this scope is empty" can mean "empty apart
// from the language's own declarations" at the one place that asks.
var injected = func() []string { o, _ := OptionSum().Defs(); return o }()

func newModule(path string) *Module {
	m := &Module{Path: path, Uses: map[string]string{},
		Exports: map[string]bool{}, Defs: map[string]*Term{},
		Sigs: map[string]*Sig{}, Sums: map[string]*Sum{}}
	opt := OptionSum()
	m.Sums[opt.Name] = opt
	order, defs := opt.Defs()
	for _, n := range order {
		m.Defs[n] = defs[n]
		m.Order = append(m.Order, n)
	}
	return m
}

// partition splits a form list into module scopes. A file with no `(module …)`
// is one anonymous root module, which is why every existing program still loads.
func partition(forms []Form) ([]*Module, []entry, error) {
	root := newModule("")
	mods := []*Module{root}
	byPath := map[string]*Module{"": root}
	cur := root
	var entries []entry

	for _, f := range forms {
		switch f.Kind {
		case "module":
			m, ok := byPath[f.Name]
			if !ok {
				m = newModule(f.Name)
				mods = append(mods, m)
				byPath[f.Name] = m
			}
			cur = m
		case "use":
			if prev, ok := cur.Uses[f.Alias]; ok && prev != f.Name {
				return nil, nil, fmt.Errorf("%s binds %s to both %s and %s — "+
					"give one of them a different (use … as …) alias",
					modLabel(cur.Path), f.Alias, prev, f.Name)
			}
			cur.Uses[f.Alias] = f.Name
		case "sig":
			cur.Sigs[f.Name] = f.Sig
		case "export":
			for _, n := range f.Names {
				cur.Exports[n] = true
			}
		case "def":
			if _, dup := cur.Defs[f.Name]; dup {
				return nil, nil, fmt.Errorf("%s is defined twice", qualify(cur.Path, f.Name))
			}
			cur.Defs[f.Name] = f.Term
			cur.Order = append(cur.Order, f.Name)
		case "sum":
			// A sum declaration is DEFINITIONS. The constructors become ordinary
			// defs, so qualification, imports, delta and the occurrence counter
			// all apply without anything downstream learning that sums exist.
			if _, dup := cur.Sums[f.Name]; dup {
				return nil, nil, fmt.Errorf("sum %s is declared twice", qualify(cur.Path, f.Name))
			}
			cur.Sums[f.Name] = f.Sum
			order, defs := f.Sum.Defs()
			for _, n := range order {
				if _, dup := cur.Defs[n]; dup {
					return nil, nil, fmt.Errorf("%s is defined twice — sum %s declares a "+
						"variant of that name", qualify(cur.Path, n), f.Name)
				}
				cur.Defs[n] = defs[n]
				cur.Order = append(cur.Order, n)
			}
		case "prim", "target":
			return nil, nil, fmt.Errorf(
				"a program cannot declare %s; primitives are declared in targets/NAME.oro", f.Kind)
		case "term":
			entries = append(entries, entry{mod: cur, term: f.Term})
		}
	}
	return mods, entries, nil
}

// resolve rewrites every name in a term to its fully qualified form.
//
// Three cases, and the third is the one that keeps λ-bound variables safe: a
// name bound by an enclosing abstraction is never qualified, however it is
// spelled.
func (m *Module) resolve(t *Term, bound map[string]bool, mods []*Module) (*Term, error) {
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KBound:
		return t, nil

	case KName:
		if bound[t.Name] {
			return t, nil
		}
		if i := strings.Index(t.Name, "."); i >= 0 {
			alias, rest := t.Name[:i], t.Name[i+1:]
			path, ok := m.Uses[alias]
			if !ok {
				// A qualified name in SOURCE always names an import. Letting
				// this fall through silently made `(use …)` a no-op whenever
				// the alias happened to equal the module path — `io.print-line`
				// resolved to itself and worked with no import at all, which
				// is precisely the "meaning depends on what is in scope"
				// that qualified imports exist to prevent (modules.md §3).
				return nil, fmt.Errorf("%s is not imported: add (use %s) or (use PATH as %s)",
					alias, alias, alias)
			}
			if err := checkExported(mods, path, rest); err != nil {
				return nil, err
			}
			return Name(qualify(path, rest)), nil
		}
		if _, ok := m.Defs[t.Name]; ok {
			return Name(qualify(m.Path, t.Name)), nil
		}
		return t, nil // a primitive, or free

	case KFn:
		inner := make(map[string]bool, len(bound)+len(t.Params))
		for k := range bound {
			inner[k] = true
		}
		for _, p := range t.Params {
			inner[p] = true
		}
		b, err := m.resolve(t.Body(), inner, mods)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, b), nil
	}

	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		r, err := m.resolve(k, bound, mods)
		if err != nil {
			return nil, err
		}
		kids[i] = r
	}
	return &Term{Kind: KApp, Kids: kids}, nil
}

// checkExported enforces the export list of a module we can see. A module we
// cannot see is one the TARGET provides, and its surface is the target file's
// business rather than ours.
func checkExported(mods []*Module, path, name string) error {
	for _, m := range mods {
		if m.Path != path {
			continue
		}
		if len(m.Exports) == 0 {
			return nil // no signature yet; everything is visible
		}
		if !m.Exports[name] {
			if _, defined := m.Defs[name]; defined {
				return fmt.Errorf("module %s defines %s but does not export it; "+
					"add it to that module's (export …)", path, name)
			}
			return fmt.Errorf("module %s has no member %s", path, name)
		}
		return nil
	}
	return nil
}

// markRecursive finds definitions reachable from their own bodies. core-0 §6
// makes this the side condition on termination: δ on a recursive definition does
// not terminate, so recursive definitions stay in the residual as target
// functions. This is g3's "recursive functions cannot be rules", arriving as a
// proof obligation.
// MarkRecursive is exported so a Target can build an Env.
func (e *Env) MarkRecursive() { e.markRecursive() }

func (e *Env) markRecursive() {
	for name := range e.Defs {
		if e.Prim[name] {
			// The target provides this name natively, so δ never unfolds the
			// definition and its cycle is unreachable — ADR 0002's "compiling
			// up". Marking it recursive would reject a program that builds.
			continue
		}
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
		case KInt, KFloat, KStr, KBool:
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

// CheckDefs enforces the one restriction that makes δ safe without a side
// condition of its own (effects.md §4). Unfolding copies a definition's body to
// every occurrence, which is contraction; it is sound exactly when the body is a
// value. A λ is a value whatever its body does, so this rejects only a name
// bound to a computation — `(def x (print-line "a"))`, whose two occurrences
// would print twice. Every call-by-value language makes the same decision.
func (e *Env) CheckDefs() error {
	for _, name := range sortedKeys(e.Defs) {
		if e.pureTerm(e.Defs[name], map[string]bool{}) {
			continue
		}
		return fmt.Errorf("the body of %s is a computation, not a value, "+
			"so unfolding it would repeat its effects\n"+
			"  Wrap it in (fn () …) and apply it, or bind it with let at the point of use.", name)
	}
	return nil
}

func sortedKeys(m map[string]*Term) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pureTerm is the judgement of effects.md §3: may this term be copied, dropped,
// or moved without changing what the program does.
//
// The subtle case is the λ, and it is subtle in two directions at once.
//
// As an argument to a β-redex a λ is a VALUE — writing it does nothing, and its
// body's effects fire at each application, which the body already contains. So
// it stays opaque, and it must: `(fn (acc i) (print-line i))` judged impure
// would be let-bound as a bare λ and reach the emitter as an escaping closure.
//
// As an argument to a PRIMITIVE it is transparent, because the structural
// primitives — loop, loop2, cond, let — apply the λ they are given. Treating
// `(fold-range z n (fn (acc i) (print-line i)))` as pure because its third
// argument is a λ would license moving the whole loop into another loop.
// readsBoundTable reports whether a term reads a table through a BOUND
// variable — `(b i)` with `b` a parameter rather than a definition.
//
// That is the shape a buffer read has, and also the shape an array read has:
// nothing in the term distinguishes them, which is why the caller tests the
// destination rather than this. Recognising it at all is what keeps the rule
// narrow — an ordinary arithmetic argument is still substituted into an impure
// body, as it always was.
func readsBoundTable(t *Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KBound {
		return true
	}
	for _, k := range t.Kids {
		if readsBoundTable(k) {
			return true
		}
	}
	return false
}

func (e *Env) pureTerm(t *Term, seen map[string]bool) bool {
	switch t.Kind {
	case KName, KInt, KFloat, KStr, KBool, KFn, KBound:
		return true // values
	case KApp:
		op := t.Op()
		for _, a := range t.Args() {
			b := a
			if op.Kind != KFn {
				for b.Kind == KFn { // applied by the primitive; look through
					b = b.Body()
				}
			}
			if !e.pureTerm(b, seen) {
				return false
			}
		}
		switch op.Kind {
		case KFn:
			return e.pureTerm(op.Body(), seen) // applying a λ runs its body
		case KName:
			return e.pureName(op.Name, seen)
		}
		return e.pureTerm(op, seen)
	}
	return false
}

// pureName answers "does calling this thing do something", which is a different
// question from "is this name safe to move" — a name is always a value. Keeping
// the two apart is what makes the λ rule above sound.
func (e *Env) pureName(n string, seen map[string]bool) bool {
	if e.Prim[n] {
		return e.Pure[n]
	}
	body, ok := e.Defs[n]
	if !ok {
		return true // a free variable is a value, not a call
	}
	if seen[n] {
		return true // recursion contributes nothing new to a least fixed point
	}
	seen[n] = true
	for body.Kind == KFn {
		body = body.Body()
	}
	return e.pureTerm(body, seen)
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

func (f *FuelError) reclose(params []string) error {
	return &FuelError{Term: FnClosed(params, f.Term)}
}

// ArityError is application to the wrong number of arguments. It carries the
// term rather than a formatted string so that reclose can put names back.
type ArityError struct {
	Fn          *Term
	Want, Given int
}

func (a *ArityError) Error() string {
	// The term is printed LAST and introduced as context, because reclose may
	// have wrapped the offending function in its enclosing binders — which is
	// what supplies the names. "%s expects N" would then name the wrong λ.
	return fmt.Sprintf("arity: expects %d argument(s), given %d, in: %s", a.Want, a.Given, a.Fn)
}

func (a *ArityError) reclose(params []string) error {
	return &ArityError{Fn: FnClosed(params, a.Fn), Want: a.Want, Given: a.Given}
}

// A recloser carries a term captured mid-reduction. Under a binder, that term's
// bound variables have no name yet and print as `#1.0`; each enclosing binder
// recloses as the stack unwinds, so the message that reaches the user is
// spelled with the names the source used.
type recloser interface{ reclose(params []string) error }

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
	case KInt, KFloat, KStr, KBool, KBound:
		return t, nil

	case KName:
		if e.unfoldable(t.Name) { // δ
			return normalize(e.Defs[t.Name], e, fuel)
		}
		return t, nil

	case KFn:
		// The CLOSED body. Reduction never opens, so it never re-closes, so a
		// colliding hint can never merge two variables. That is what makes the
		// representation buy anything.
		b, err := normalize(t.Closed(), e, fuel)
		if err != nil {
			if rc, ok := err.(recloser); ok {
				return nil, rc.reclose(t.Params)
			}
			return nil, err
		}
		return FnClosed(t.Params, b), nil

	case KApp:
		op, err := normalize(t.Op(), e, fuel)
		if err != nil {
			return nil, err
		}
		args := t.Args()

		// β-tab — THE SECOND CLAUSE OF β, not a fourth rule.
		//
		//	((array 10 20 30) 2)  ⟶  30      a function given by its GRAPH
		//	((table n f) i)       ⟶  (f i)   a function given by a RULE
		//
		// A function can be written down two ways, so application has two cases.
		// Ordinary β handles the intensional presentation — substitute into the
		// body. β-tab handles the extensional one — look the argument up. Same
		// judgement, *apply this function to this argument*, and calling it "the
		// extensional counterpart of β" is not a flourish: extensionality is
		// precisely that a function is determined by its input/output pairs
		// (tables.md §4.1).
		//
		// It needs NO constant folder. Looking up element k of a literal table
		// is not arithmetic and cannot disagree with anything at runtime, so
		// ADR 0009 has nothing to say about it — which is why this is the easy
		// path and folding waits for integers to need it (tables.md §4.3).
		if idx, ok := e.betaTab(op, args, fuel); ok {
			return normalize(idx, e, fuel)
		}

		// CASE-OF-CASE, and it is what makes a DYNAMIC sum cost nothing.
		//
		//	((if c A B) k…)  ⟶  (if c (A k…) (B k…))
		//
		// Prawitz's commuting conversion; GHC's case-of-case; and for us the
		// rule sums-research.md §0.1 said was one step away from free. A sum
		// whose tag is known reduces away by β alone, but one whose tag depends
		// on runtime data gets STUCK as `((if c A B) F G)` — the constructor is
		// under the branch and the eliminator is outside it, so neither can see
		// the other. Pushing the eliminator INTO both branches reunites each
		// constructor with it, and then β finishes the job: no tag, no closure,
		// no allocation, and no runtime dispatch that the `if` was not already
		// doing.
		//
		// Only when every argument is PURE. The rule duplicates the arguments
		// into both branches, and ADR 0010 denies contraction for an impure
		// term — duplicating one would run an effect twice. Left stuck, an
		// impure eliminator is reported by the emitter rather than silently
		// mis-ordered.
		//
		// The known hazard is code growth: `k` appears twice, so nested cases
		// multiply. GHC's answer is join points, and `again` IS one
		// (types-direction.md §6) — which is the direction if this ever bites.
		// The LET companion, and the nested test is what demanded it:
		//
		//	((let v (fn (x) B)) k…)  ⟶  (let v (fn (x) (B k…)))
		//
		// The same commuting conversion, one constructor over. It is needed
		// because β itself puts a `let` between a constructor and its
		// eliminator: a shared subterm that is not duplicable gets let-bound
		// (ADR 0010), and the eliminator then sits outside a binder rather than
		// outside an `if`. Handling only `if` reduced two levels of a
		// three-level nest and stopped.
		//
		// So the rule is not "case-of-case" so much as: PUSH AN ELIMINATOR
		// THROUGH ANYTHING β CAN LEAVE IN OPERATOR POSITION, which in this
		// language is exactly `if` and `let`.
		if isLetApp(op, e) && allPure(args, e) {
			lam := op.Kids[2]
			inner := lam.OpenWith([]*Term{Name("#c")})
			app := append([]*Term{inner}, args...)
			out := &Term{Kind: KApp, Kids: []*Term{op.Kids[0], op.Kids[1],
				Fn([]string{"#c"}, &Term{Kind: KApp, Kids: app})}}
			return normalize(out, e, fuel)
		}

		if isIfApp(op, e) && allPure(args, e) {
			br := func(b *Term) *Term {
				kids := make([]*Term, 0, len(args)+1)
				return normalizeLater(append(append(kids, b), args...))
			}
			out := &Term{Kind: KApp, Kids: []*Term{
				op.Kids[0], op.Kids[1], br(op.Kids[2]), br(op.Kids[3])}}
			return normalize(out, e, fuel)
		}

		if op.Kind == KFn { // β
			if len(op.Params) != len(args) {
				return nil, &ArityError{Fn: op, Want: len(op.Params), Given: len(args)}
			}
			subs := make([]*Term, len(args))
			var bound []struct {
				name string
				val  *Term
			}
			// Names already chosen for bindings of THIS application, so two
			// parameters cannot be renamed onto each other.
			used := map[string]bool{}
			for i, p := range op.Params {
				// Impure: never substituted. Bound here, at the application
				// site, which is where the programmer wrote it — at its
				// original loop depth and under its original guards. Binding
				// at the USE site instead would be the bug. (effects.md §4)
				// A TABLE READ IS NOT SUBSTITUTED INTO AN IMPURE BODY.
				//
				// `pureTerm` answers "value" for every bound variable, so
				// `(b 0)` — a buffer read — is judged pure and may be moved.
				// ADR 0018 says a buffer read is IMPURE and ADR 0010 says an
				// impure argument is never substituted, precisely so that
				// exchange is denied; without this a swap
				//
				//	(let (b 0) (fn (vx) (let (b 1) (fn (vy)
				//	  (set (set b 0 vy) 1 vx)))))
				//
				// emits as `b[0] = b[1]; b[1] = b[0]` and silently copies.
				//
				// The term cannot say which bound variables are buffers — an
				// ARRAY read has the identical shape and is genuinely pure — so
				// the test is on the DESTINATION instead: a read may move freely
				// into a body that has no effects to be reordered against, and
				// not into one that has. That is exactly the property at stake,
				// and it is decidable here because the body is in hand.
				//
				// Testing the OPERAND instead — "any application of a bound
				// variable is impure" — was tried and is wrong: a rule-table's
				// rule reads its parameter table, so `(table n f)` becomes
				// impure, is no longer substituted, and reaches the backend
				// unfused. `dot` and `smooth` on Java stop compiling.
				if readsBoundTable(args[i]) && !e.pureTerm(op.Body(), map[string]bool{}) {
					na, err := normalize(args[i], e, fuel)
					if err != nil {
						return nil, err
					}
					nm := e.bindName(p, op.Params, used)
					used[nm] = true
					bound = append(bound, struct {
						name string
						val  *Term
					}{nm, na})
					subs[i] = Name(nm)
					continue
				}
				if !e.pureTerm(args[i], map[string]bool{}) {
					na, err := normalize(args[i], e, fuel)
					if err != nil {
						return nil, err
					}
					nm := e.bindName(p, op.Params, used)
					used[nm] = true
					bound = append(bound, struct {
						name string
						val  *Term
					}{nm, na})
					subs[i] = Name(nm)
					continue
				}
				// One occurrence or none: substituting cannot duplicate anything.
				if boundOccurrences(op.Closed(), 0, i) <= 1 {
					subs[i] = args[i]
					continue
				}
				// More than one. Normalise the argument to find out what it is —
				// this is the only place reduction is not lazy, and it is what
				// makes the classification possible at all.
				na, err := normalize(args[i], e, fuel)
				if err != nil {
					return nil, err
				}
				if duplicable(na) || e.duplicableTable(na) {
					subs[i] = na
					continue
				}
				nm := e.bindName(p, op.Params, used)
				used[nm] = true
				bound = append(bound, struct {
					name string
					val  *Term
				}{nm, na})
				subs[i] = Name(nm)
			}
			body, err := normalize(op.OpenWith(subs), e, fuel)
			if err != nil {
				return nil, err
			}
			// Wrap innermost-last so the bindings nest in source order.
			for i := len(bound) - 1; i >= 0; i-- {
				body = App(Name("let"), bound[i].val, Fn([]string{bound[i].name}, body))
			}
			return body, nil
		}
		// The conditional on a known condition. It was the ONLY evaluation
		// reduction performed until `=` on two integer literals joined it above
		// (2026-08-22, for sums) — so the pair of them is now the whole of it,
		// and it is worth being exact about why neither violates "no primitive
		// is ever evaluated" (state.md §3): `if` and `=` are the LANGUAGE's
		// names, injected into every target, and `true`/`false` are not
		// primitive applications. Nothing about the target is being decided
		// here (booleans.md §4.3). `(go.+ 1 2)` still does not fold.
		//
		// Sound for an IMPURE untaken branch, and for a different reason than
		// β's. β may not drop an impure argument because the argument would
		// have run; here the branch genuinely does not run. The structural
		// rules are not involved.
		//
		// ADR 0009 is satisfied trivially: boolean algebra is exact, so there
		// is no compile-time/runtime discrepancy to preserve.
		// `=` ON TWO INTEGER LITERALS folds, and this is the first entry in the
		// constant-folding table tables.md predicted — where `((array 1 2 3) 1)
		// → 2` and `(go.+ 1 2) → 3` are the same kind of step, not new rules.
		//
		// It is here because SUMS need it. A sum's tag is a literal after
		// reduction, so `(case (ok n) …)` reduced to `(if (= 0 0) … …)` — the
		// sum had vanished and left a tautological test behind, which is a
		// static cost the two-level language says should not exist.
		//
		// Narrow deliberately: integers only, `=` only. ADR 0009 permits it
		// because integer equality inside the portable window is bit-identical
		// on every target — the thing that is NOT true of float arithmetic,
		// which is why Go folds `0.1+0.2` two different ways and why nothing
		// here folds a float.
		if op.Kind == KName && op.Name == "=" && e.Prim["="] && len(args) == 2 {
			a, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			b, err := normalize(args[1], e, fuel)
			if err != nil {
				return nil, err
			}
			if a.Kind == KInt && b.Kind == KInt {
				return Bool(a.Int == b.Int), nil
			}
			return &Term{Kind: KApp, Kids: []*Term{op, a, b}}, nil
		}
		// `len` on a table whose length is known. See tableLen.
		if op.Kind == KName && op.Name == "len" && e.Prim["len"] && len(args) == 1 {
			// The ARGUMENT has to be normalised first: `(len a)` where `a` is a
			// definition arrives as a name, and δ is what turns it into the
			// `(array …)` whose length is visible.
			na, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if n, ok := e.tableLen([]*Term{na}); ok {
				return normalize(n, e, fuel)
			}
			return &Term{Kind: KApp, Kids: []*Term{op, na}}, nil
		}

		var cond *Term
		if op.Kind == KName && op.Name == "if" && e.Prim["if"] && len(args) == 3 {
			c, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if c.Kind == KBool {
				if c.IsTrue() {
					return normalize(args[1], e, fuel)
				}
				return normalize(args[2], e, fuel)
			}
			cond = c
		}
		out := make([]*Term, 0, len(t.Kids))
		out = append(out, op)
		for i, a := range args {
			if i == 0 && cond != nil { // already normalised above
				out = append(out, cond)
				continue
			}
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

// duplicable reports whether a normalised term may be copied freely.
//
// Literals and variables are obviously free. Abstractions are the interesting
// case: a duplicated λ MUST be substituted or fusion dies — in the dot product
// the two copies of the zip term reduce to *different* small things, (alen p)
// and a multiply, and that is the entire mechanism. Duplicating a λ that does
// not reduce away costs code size, which is the measured specialize-versus-
// outline tradeoff, not a correctness problem.
//
// Everything else is an application of a primitive, which may allocate, and
// allocation is what no host can hoist.
func duplicable(t *Term) bool {
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KName, KFn, KBound:
		return true
	}
	return false
}

// boundOccurrences counts uses of ONE bound variable, saturating at 2. No
// shadowing check: an inner binder simply sits at a different depth.
func boundOccurrences(t *Term, depth, index int) int {
	switch t.Kind {
	case KBound:
		if t.Depth == depth && t.Index == index {
			return 1
		}
		return 0
	case KName, KInt, KFloat, KStr, KBool:
		return 0
	case KFn:
		return boundOccurrences(t.Kids[0], depth+1, index)
	}
	n := 0
	for _, k := range t.Kids {
		n += boundOccurrences(k, depth, index)
		if n >= 2 {
			return 2
		}
	}
	return n
}

// occurrences counts free occurrences of name in t, saturating at 2 — the
// decision only needs to distinguish "at most once" from "more than once".
func occurrences(t *Term, name string) int {
	switch t.Kind {
	case KName:
		if t.Name == name {
			return 1
		}
		return 0
	case KInt, KFloat, KStr, KBool, KBound:
		return 0
	case KFn:
		for _, p := range t.Params {
			if p == name {
				return 0 // shadowed
			}
		}
		return occurrences(t.Body(), name)
	}
	n := 0
	for _, k := range t.Kids {
		n += occurrences(k, name)
		if n >= 2 {
			return 2
		}
	}
	return n
}

// Occurs reports whether name is used in t. Backends need this to recognise a
// residual `let` whose binder is used zero times: that is a sequencing point
// (effects.md §5), the binding has no reader, and Go rejects the program if one
// is emitted anyway.
func Occurs(t *Term, name string) bool { return occurrences(t, name) > 0 }

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
	case KInt, KFloat, KStr, KBool, KBound:
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
		case KInt, KFloat, KStr, KBool, KBound:
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
	// `again` is bound by the enclosing loop and by nothing else, so it is not
	// residual there — the same rule scope() applies, arriving here late.
	//
	// It arrived late because it was never reached: ADR 0015 was built and
	// benchmarked entirely through `gen`, which emits a function, and this
	// check lives in `build`, which makes a binary. Every loop program was
	// therefore refused by the one command that produces an artifact, and no
	// test noticed, because no test built one. Found writing the fourth target.
	var walk func(t *Term, bound map[string]bool, inLoop bool)
	walk = func(t *Term, bound map[string]bool, inLoop bool) {
		switch t.Kind {
		case KName:
			if bound[t.Name] || (t.Name == "again" && inLoop) {
				return
			}
			if !e.Prim[t.Name] && !e.Rec[t.Name] {
				found[t.Name] = true
			}
		case KInt, KFloat, KStr, KBool, KBound:
		case KFn:
			inner := make(map[string]bool, len(bound)+len(t.Params))
			for k := range bound {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body(), inner, inLoop)
		default:
			// (loop (fn (x…) body) z…): only the abstraction is under the
			// binder. The initial values are evaluated outside it.
			isLoop := t.Kind == KApp && t.Op().Kind == KName && t.Op().Name == "loop"
			for i, k := range t.Kids {
				walk(k, bound, inLoop || (isLoop && i == 1))
			}
		}
	}
	walk(t, map[string]bool{}, false)
	out := make([]string, 0, len(found))
	for n := range found {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// substPublic exposes capture-avoiding substitution for backends.
func substPublic(t *Term, m map[string]*Term) *Term { return subst(t, m) }

// CheckScope reports names that are bound nowhere.
//
// This is **name resolution**, and it is a separate question from the covering
// check even though both look at free names:
//
//	scope    — is this name bound anywhere at all?   A PROGRAM error.
//	covering — can THIS target provide it?           ADR 0001's portability property.
//
// Conflating them left three holes. `oro` printed a warning and exited 0; `gen`
// never checked at all; and a name that appears only in a definition the program
// never reaches was never looked at, so a typo in unused code was invisible.
// That last one is the classic reason name resolution walks EVERYTHING rather
// than only what reduction happens to visit.
//
// It runs before reduction, so the report names the definition the mistake is
// in rather than wherever substitution later carried it.
//
// It also rejects recursion, for the reason in ADR 0014. The two live in one
// method because they are the same question — can this target run this program
// — and because the two-call-site version of the recursion check drifted from
// the scope check within a day of being written.
func (e *Env) CheckProgram(terms []*Term) error {
	if err := e.checkScope(terms); err != nil {
		return err
	}
	return e.checkRecursion()
}

func (e *Env) checkScope(terms []*Term) error {
	for _, name := range sortedKeys(e.Defs) {
		if err := e.scope(e.Defs[name], map[string]bool{}, "in "+name); err != nil {
			return err
		}
	}
	for _, t := range terms {
		if err := e.scope(t, map[string]bool{}, "at the top level"); err != nil {
			return err
		}
	}
	return nil
}

func (e *Env) scope(t *Term, bound map[string]bool, where string) error {
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KBound:
		return nil
	case KName:
		if bound[t.Name] || e.Prim[t.Name] {
			return nil
		}
		if _, ok := e.Defs[t.Name]; ok {
			return nil
		}
		return fmt.Errorf("%s: %s is not bound — it is not a parameter, not a definition, "+
			"and not a primitive on this target%s", where, t.Name, e.importHint(t.Name))
	case KFn:
		inner := make(map[string]bool, len(bound)+len(t.Params))
		for k := range bound {
			inner[k] = true
		}
		for _, p := range t.Params {
			inner[p] = true
		}
		return e.scope(t.Body(), inner, where)
	}
	// `again` is bound by the enclosing loop, and only there. The reader has
	// already checked its arity and position (iteration.md §2); this is the
	// scope half, so an `again` outside any loop is reported like any other
	// unbound name rather than reaching the emitter.
	if t.Kind == KApp && t.Op().Kind == KName && t.Op().Name == "loop" && len(t.Args()) >= 1 {
		if lam := t.Args()[0]; lam.Kind == KFn {
			inner := make(map[string]bool, len(bound)+len(lam.Params)+1)
			for k := range bound {
				inner[k] = true
			}
			for _, p := range lam.Params {
				inner[p] = true
			}
			inner["again"] = true
			if err := e.scope(lam.Body(), inner, where); err != nil {
				return err
			}
			for _, a := range t.Args()[1:] {
				if err := e.scope(a, bound, where); err != nil {
					return err
				}
			}
			return nil
		}
	}
	for _, k := range t.Kids {
		if err := e.scope(k, bound, where); err != nil {
			return err
		}
	}
	return nil
}

// checkRecursion rejects a definition that is defined in terms of itself.
//
// Recursion REDUCES correctly — δ declines to unfold a cycle, which is the
// standard partial-evaluation answer — and no backend emits one. That gap was
// reported at the emitter for one day, which meant `oro` accepted a program
// that `build` refused. ADR 0014 closes it the other way: recursion is not in
// the language, so it must fail at the earliest honest point.
//
// That point is here rather than in the reader, because whether a name is
// recursive depends on the TARGET: a target that provides the name natively
// never unfolds the definition, and markRecursive already skips those.
func (e *Env) checkRecursion() error {
	names := make([]string, 0, len(e.Rec))
	for n := range e.Rec {
		names = append(names, n)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if len(names) == 1 {
		return fmt.Errorf("%s is recursive: it is defined in terms of itself, and recursion "+
			"is not in the language — iteration is fold-range (ADR 0014).\n"+
			"  If the self-reference was not deliberate, this definition is shadowing "+
			"the %s you meant.", names[0], names[0])
	}
	return fmt.Errorf("%s are mutually recursive, and recursion is not in the language — "+
		"iteration is fold-range (ADR 0014)", strings.Join(names, ", "))
}

// Shadowed reports definitions the target provides natively.
//
// This is ADR 0002 working — δ declines to unfold a name in P_T, so the target's
// own implementation is used and the definition is a fallback for targets that
// lack it. It is the single most consequential thing the compiler decides about
// a program, it is decided by a file the program does not name, and it was
// silent. So it is reported as a note rather than left to be discovered.
//
// It is deliberately NOT an error and deliberately not extended to a general
// shadowing report — see def.md §11 for what is not reported and why.
func (e *Env) Shadowed() []string {
	var out []string
	for n := range e.Defs {
		if e.Prim[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// importHint explains an unresolved qualified name when the cause is an import
// that matched nothing at all.
//
// `(use geometrie)` for a module spelled `geometry` is silent at the import —
// a path with no file is how a TARGET-provided module looks — and then fails on
// the first member with a message about the member. Which is the wrong half of
// the name to look at.
func (e *Env) importHint(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return ""
	}
	path := name[:i]
	if !e.unresolvedPaths[path] {
		return ""
	}
	for p := range e.Prim {
		if strings.HasPrefix(p, path+".") {
			return "" // the target does provide this module, just not this member
		}
	}
	return fmt.Sprintf("\n  (use %s) matched no file on the search path and this target "+
		"provides no module %s either, so every name from it is unbound. Check the path.",
		path, path)
}

// modLabel names a module in a diagnostic. The entry file's anonymous scope has
// no path, and calling it "" reads as a bug.
func modLabel(path string) string {
	if path == "" {
		return "the program"
	}
	return "module " + path
}

// isIfApp reports whether t is a residual `if` — a conditional whose branches
// survived because the condition is not known. That is the only shape
// case-of-case applies to.
func isIfApp(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 4 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "if" && e.Prim["if"]
}

// isLetApp reports whether t is a residual `let` — a binding β introduced
// because its value was shared and not duplicable.
func isLetApp(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 3 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "let" && e.Prim["let"] &&
		t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1
}

// allPure reports whether every term is safe to DUPLICATE. Case-of-case copies
// its arguments into both branches, and ADR 0010 denies contraction for an
// impure term, so an impure argument leaves the redex stuck rather than running
// an effect twice.
func allPure(ts []*Term, e *Env) bool {
	for _, t := range ts {
		if !e.pureTerm(t, map[string]bool{}) {
			return false
		}
	}
	return true
}

// normalizeLater builds the application case-of-case pushes into one branch.
func normalizeLater(kids []*Term) *Term { return &Term{Kind: KApp, Kids: kids} }

// isTableForm reports whether t is `(array e…)` or `(table n f)` — the two ways
// a table is written down.
func isForm(t *Term, e *Env, name string) bool {
	return t != nil && t.Kind == KApp && len(t.Kids) > 0 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == name && e.Prim[name]
}

// betaTab is β's second clause: application of a table to an index.
//
// The graph form needs the index to be a LITERAL, because looking up element k
// of `(array a b c)` is only possible when k is known. A dynamic index leaves
// the application alone and the backend emits the host's own indexing — which
// is the whole point of indexing being application: `(a i)` is the same text
// whether it reduces here or is emitted (tables.md §3.1).
//
// The rule form needs nothing: `((table n f) i)` is `(f i)` for any i, which is
// what makes a table-of-a-rule FUSE. That is the mechanism `dot` runs on.
func (e *Env) betaTab(op *Term, args []*Term, fuel *int) (*Term, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if isForm(op, e, "table") && len(op.Kids) == 3 {
		return &Term{Kind: KApp, Kids: []*Term{op.Kids[2], args[0]}}, true
	}
	// A MAP LITERAL, which is β-tab with a SUM in the result position.
	//
	// `((map (1 10) (2 20)) 1)` is `(some 10)` and `((map (1 10)) 5)` is
	// `(none)`. Both need the index AND every literal key to be integer
	// literals, because the domain condition `k ∈ dom m` is decided by equality
	// and this is the only place equality can be decided by inspection.
	//
	// Absence is a RESULT here, not a stuck term — unlike an array, where an
	// out-of-range index is left alone for the refinement layer to report with
	// the bound and the call site. That difference IS the difference between
	// the two constructs: an array's domain condition is discharged statically
	// and a map's is discharged by the program (maps.md §1.1), so `none` is the
	// answer rather than an error.
	if isForm(op, e, "map") {
		k, err := normalize(args[0], e, fuel)
		if err != nil || k == nil || k.Kind != KInt {
			return nil, false
		}
		for _, row := range op.Kids[1:] {
			if row.Kind != KApp || len(row.Kids) != 2 {
				return nil, false
			}
			key, err := normalize(row.Kids[0], e, fuel)
			if err != nil || key == nil || key.Kind != KInt {
				return nil, false
			}
			if key.Int == k.Int {
				return &Term{Kind: KApp, Kids: []*Term{Name("some"), row.Kids[1]}}, true
			}
		}
		// A payload-less variant IS the constructor — its definition is the
		// encoded value, not a function — so absence is the NAME `none`, never
		// an application of it.
		return Name("none"), true
	}
	if isForm(op, e, "array") {
		i, err := normalize(args[0], e, fuel)
		if err != nil || i == nil || i.Kind != KInt {
			return nil, false
		}
		elems := op.Kids[1:]
		if i.Int < 0 || i.Int >= int64(len(elems)) {
			// Out of the domain. NOT an error here — the refinement layer is
			// what reports it, with the bound and the call site. Reduction
			// leaves it alone so the diagnostic comes from the place that can
			// explain it (tables.md §6).
			return nil, false
		}
		return elems[i.Int], true
	}
	return nil, false
}

// tableLen folds `len` when the table's length is known: `(len (array a b c))`
// is 3 and `(len (table n f))` is n.
//
// This joins `if` on a boolean literal and `=` on two integers under the same
// rule — a construct decided by a literal it can see. `if` was already exactly
// that, so this is a widening of a rule that existed rather than a new one
// (tables.md §4.2), and the granularity change is stated rather than hidden.
func (e *Env) tableLen(args []*Term) (*Term, bool) {
	if len(args) != 1 {
		return nil, false
	}
	t := args[0]
	if isForm(t, e, "array") || isForm(t, e, "map") {
		return &Term{Kind: KInt, Int: int64(len(t.Kids) - 1)}, true
	}
	if isForm(t, e, "table") && len(t.Kids) == 3 {
		return t.Kids[1], true
	}
	return nil, false
}

// duplicableTable reports whether a term is a table given by a RULE, which is
// free to copy.
//
// `(table n f)` has no runtime existence: it is a length and a function, and
// copying it copies a description rather than data. That is the whole reason
// the rule form is free, and it is what makes fusion work — `sum` mentions its
// argument twice, as `(len v)` and as `(v i)`, so without this β let-binds the
// table and the intermediate survives to the residual instead of vanishing.
//
// It is exactly the argument that makes `KFn` duplicable, one constructor over:
// a rule-table is a lambda with a length attached. `(array e…)` is deliberately
// NOT included — a graph is data, and duplicating it duplicates the elements.
// `(alloc t)` is where memory happens, and it is not duplicable either.
//
// The condition is PURITY, not duplicability, and the reason is that copying a
// rule-table does not actually duplicate anything: substituting it puts
// `(len (table n f))` where `(len v)` was — which folds to `n` — and
// `((table n f) i)` where `(v i)` was — which is `(f i)`. The table is gone on
// both sides, so `n` still appears once. What looked like duplication is the
// step that ERASES the intermediate.
//
// Purity is still required, because β may not move an impure term at all
// (ADR 0010).
func (e *Env) duplicableTable(t *Term) bool {
	return isForm(t, e, "table") && len(t.Kids) == 3 &&
		e.pureTerm(t, map[string]bool{})
}
