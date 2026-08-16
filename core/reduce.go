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
	return &Program{Defs: map[string]*Term{}, Sigs: map[string]*Sig{}}
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
}

// qualify joins a module path to a local name. The root module has no path, so
// a program that never says `(module …)` behaves exactly as before.
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
				if m.Path == "" && len(m.Defs) == 0 {
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

	p := NewProgram()
	sort.Strings(unresolved)
	p.Unresolved = unresolved
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

// partition splits a form list into module scopes. A file with no `(module …)`
// is one anonymous root module, which is why every existing program still loads.
func partition(forms []Form) ([]*Module, []entry, error) {
	root := &Module{Uses: map[string]string{}, Exports: map[string]bool{},
		Defs: map[string]*Term{}, Sigs: map[string]*Sig{}}
	mods := []*Module{root}
	byPath := map[string]*Module{"": root}
	cur := root
	var entries []entry

	for _, f := range forms {
		switch f.Kind {
		case "module":
			m, ok := byPath[f.Name]
			if !ok {
				m = &Module{Path: f.Name, Uses: map[string]string{},
					Exports: map[string]bool{}, Defs: map[string]*Term{}, Sigs: map[string]*Sig{}}
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
	case KInt, KFloat, KStr, KBound:
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
		case KInt, KFloat, KStr:
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
func (e *Env) pureTerm(t *Term, seen map[string]bool) bool {
	switch t.Kind {
	case KName, KInt, KFloat, KStr, KFn, KBound:
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
	case KInt, KFloat, KStr, KBound:
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
		if op.Kind == KFn { // β
			if len(op.Params) != len(args) {
				return nil, &ArityError{Fn: op, Want: len(op.Params), Given: len(args)}
			}
			subs := make([]*Term, len(args))
			var bound []struct {
				name string
				val  *Term
			}
			for i, p := range op.Params {
				// Impure: never substituted. Bound here, at the application
				// site, which is where the programmer wrote it — at its
				// original loop depth and under its original guards. Binding
				// at the USE site instead would be the bug. (effects.md §4)
				if !e.pureTerm(args[i], map[string]bool{}) {
					na, err := normalize(args[i], e, fuel)
					if err != nil {
						return nil, err
					}
					bound = append(bound, struct {
						name string
						val  *Term
					}{p, na})
					subs[i] = Name(p)
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
				if duplicable(na) {
					subs[i] = na
					continue
				}
				bound = append(bound, struct {
					name string
					val  *Term
				}{p, na})
				subs[i] = Name(p)
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
	case KInt, KFloat, KStr, KName, KFn, KBound:
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
	case KName, KInt, KFloat, KStr:
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
	case KInt, KFloat, KStr, KBound:
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
	case KInt, KFloat, KStr, KBound:
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
		case KInt, KFloat, KStr, KBound:
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
	case KInt, KFloat, KStr, KBound:
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
