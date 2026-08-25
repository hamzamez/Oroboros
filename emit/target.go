package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oroboros/core"
)

// A target, declared as data rather than as Go source.
//
// This exists because requirements 3 and 4 were false: adding a host function
// meant editing three Go files and rebuilding the compiler. What a target
// declaration has to carry was not designed — it was read off what three
// backends turned out to need, and corrected twice along the way (a *kind*
// rather than an expression/statement boolean; types optional, because JS has
// none).
//
// The boundary is deliberate. **Expression and statement primitives are pure
// data** — a template, an arity, types, and an optional import. **Structural
// primitives are named in data and implemented in code**: a loop binds
// variables and emits a header, and no template expresses that. So adding
// `fmt.Println` needs no Go; adding a new control structure does.

type Prim struct {
	Name   string
	Args   []string // our type names; empty when the target is untyped
	Result string
	Kind   string // expr | stmt | loop | loop2 | cond | let
	Form   string // template with %s holes; empty for structural kinds
	Import string
	Pure   bool // declared `pure`; DEFAULTS TO FALSE, deliberately — see below
	Index  bool // declared `index`: argument 0 is a container indexed by argument 1

	// Length is `(length N)`: the result is a container whose length is the
	// VALUE of argument N — `make([]bool, n)` is n long. LengthOf is
	// `(length-of N)`: the result is AS LONG AS argument N — `c[i] = true`
	// returns something as long as c.
	//
	// Both are declared, never inferred, because each is a fact about the HOST
	// call and only the target author knows it. Nothing about the string
	// "make-bool" says the result is n long.
	//
	// They are two attributes rather than one read off the argument's declared
	// type, and that is the second design: the first inferred `int` argument
	// means count, anything else means pass-through. It broke on the first
	// target that tried it. `targets/js/` declares EVERY argument as `any`,
	// because JavaScript has one number type and untyped containers, so
	// `new Array(n)` and a hypothetical pass-through are indistinguishable by
	// type. A declaration that only works on targets with a rich type table is
	// not a declaration.
	//
	// Without either, a program that allocates and then indexes cannot be
	// proven: the sieve's `(let (go.make-bool n) (fn (c) … (go.at-bool c i)))`
	// has `len(c)` as an opaque variable unrelated to `n`. Zero means "not
	// declared"; positions are stored one-based for exactly that reason.
	Length   int
	LengthOf int

	// Jump is a BRANCH form: the host's own condition code for this predicate,
	// so a conditional can test it directly instead of materialising a boolean
	// and comparing that against zero. Empty on every host with expressions —
	// Go writes `if a < b` and its own compiler does this. It exists because
	// assembly has no expressions: `cmp; setl; cmp; je` is two compares where
	// hand-written code has one (docs/spec/windows-target.md §4).
	//
	// It once carried two pseudo-codes, "and" and "or", and ADR 0017 removed
	// them: they made short-circuiting a claim a target author makes, and on
	// the windows target they made ONE name mean the strict instruction as a
	// value and a branch as a guard. The connectives are the language's.
	Jump string
	// JumpForm is the comparison that sets the flags for Jump, when the default
	// — `cmp %1, %2` for integers, `comisd %1, %2` for floats — is not it.
	// `(jump "ne" "cmp byte ptr [%1+%2], 0")` is a predicate over a container
	// and an index, and it is the fused test x86 actually has.
	JumpForm string

	// Checked is the primitive to use when the compiler CANNOT prove this
	// operation's result stays inside the portable window — the representation
	// a declared range selects (sct-2026-08-19, data-model.md §1.5).
	//
	// A target that declares none simply cannot do exact arithmetic for that
	// operation, and the covering check says which targets those are. That is
	// ADR 0002 answering, not a special case.
	Checked string

	// Where is a refinement: a boolean term over the primitive's parameter
	// names, discharged at every call site (docs/spec/refinements.md).
	Where *core.Term
	// Names are the parameter names, which a Where refers to. Empty unless the
	// declaration used the named form.
	Names []string
}

type Target struct {
	Name  string
	Types map[string]string // our type name -> the target's spelling

	// ArrayType is how this target spells an array of something — `[]%s` on Go,
	// `%s[]` on Java. One declaration replaces an entry per element type.
	// Empty means the target has no types to spell (JavaScript, windows).
	ArrayType string
	Prims     map[string]Prim
	Names     []string // every primitive name, for core.Env

	// Narrow is a template `dst = src[:n]` that restricts a container to a
	// known length. A target that declares one gets bounds-check elimination
	// in loops (bce-2026-08-15.md); one that does not simply gets none, which
	// is right for JS (no bounds checks) and Java (fixed-length arrays).
	Narrow string

	// Artifact is the emitted filename that IS the deliverable when the host
	// has no compile step. JavaScript is such a host: `node main.mjs` runs the
	// source, so there is nothing to build and the artifact is a copy.
	Artifact string

	// Data is host storage the target itself owns — a scratch buffer, an
	// out-parameter cell. Every host so far could allocate from within an
	// expression, so no target had ever needed to declare storage; Win32
	// `WriteFile` takes an out-pointer, and there is nowhere in the LANGUAGE to
	// put one. Emitted verbatim into the artifact's data section.
	Data []string

	// Build is the host toolchain command: %s is the artifact path, %s the
	// directory holding the emitted source. A target that declares none can
	// still emit source, which is what cmd/gen does (build.md §4).
	Build string
}

// Kinds that the emitter implements in code rather than from a template.
var structuralKinds = map[string]bool{
	"loop": true, "loop2": true, "cond": true, "let": true, "build": true,
	// `iterate` is (loop (fn (x…) body) z…) — docs/spec/iteration.md. The kind
	// name differs from the primitive name because `loop` was already taken as
	// the kind of `fold-range`.
	"iterate": true,
}

// LoadTarget reads a target from a FILE or a DIRECTORY.
//
// A directory holds one `(target NAME …)` form per file, merged — which is what
// lets a target's surface be organised the way the host organises itself, one
// file per package, instead of one file that grows without bound. build.md §4
// and modules.md both recorded "a target is still one file rather than a
// directory" as not-yet-built; this is it.
//
// Merging is a union with no precedence: two files declaring the same primitive
// is an error, because silently taking one would make a target's meaning depend
// on filename order.
func LoadTarget(path string) (*Target, error) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return loadTargetDir(path)
	}
	if err != nil {
		// A directory target may be named without its extension.
		if di, dErr := os.Stat(strings.TrimSuffix(path, ".oro")); dErr == nil && di.IsDir() {
			return loadTargetDir(strings.TrimSuffix(path, ".oro"))
		}
		return nil, err
	}
	tg, err := loadTargetFile(path)
	if err != nil {
		return nil, err
	}
	tg.addCore()
	return tg, nil
}

// coreNames are the names the LANGUAGE owns. A target may not declare any of
// them; three are reader sugar that never reaches a target at all, and `if` is
// injected into every target by addCore.
var coreNames = map[string]bool{
	"if": true, "and": true, "or": true, "not": true, "cond": true,
	// `let` and `loop` for the same reason `if` is here, generalised late.
	// A construct promoted to the LANGUAGE works on every target and the
	// compiler finds the implementation; a target neither declines it nor
	// declares it. The capability graph is for target-native names, where
	// "this target cannot do it" is a true answer a program can be told.
	"let": true, "loop": true,
	// `=` is integer equality, and it is here because `match` desugars to it
	// (core/read.go). The language has no GENERAL equality — floats have NaN,
	// which is not an equivalence relation, and strings have no portable
	// comparison — but equality on a FINITE type is portable and total, and a
	// match guard is exactly that: an integer against a literal.
	//
	// It is `=` rather than `==`, and rather than the `tag=` it was first built
	// as.
	//
	// CORRECTION, 2026-08-25. The reason first recorded for rejecting `==` was
	// that JavaScript had already taken the name. That is FALSE and was never
	// checked: `tg.Prims` is keyed by the QUALIFIED name, so `js.==` and a bare
	// `==` are different keys and would coexist exactly as `=` and `go.==` do
	// today. There was no collision to avoid.
	//
	// What survives is a legibility argument, which is weaker and is stated as
	// such: a program holding both `==` (strict, the language's) and `js.==`
	// (loose, the host's) spells two different operations almost identically,
	// and `=` cannot be misread that way. `=` is also what Scheme, Clojure, SQL
	// and mathematics use for equality.
	//
	// Not `tag=`, and this reason is unaffected: a name should say what an
	// operation IS rather than what it is for — `(when (= (go.% v 2) 1))` is not
	// comparing a tag — and the honesty a narrow name was buying is better
	// bought by the REFUSAL, which can explain itself where a name cannot.
	"=": true,
	// TABLES. A table is a function with a known finite domain (tables.md), so
	// `array` and `table` are its two presentations — a graph and a rule — and
	// `len` is its domain bound. Indexing needs no name at all, because it is
	// APPLICATION.
	//
	// These are the language's for the same reason `if` is: a construct promoted
	// to the language works on every target and the compiler finds the
	// implementation. A target may not decline one and may not declare one.
	//
	// There is no collision with a host's own `len`. `tg.Prims` is keyed by the
	// QUALIFIED name, so `go.len` — which works on maps and channels too, and
	// stays reachable — and a bare `len` are different keys.
	"array": true, "table": true, "len": true,
}

// coreStructural is what addCore injects: the language's own constructs, with
// the structural KIND each backend already implements.
var coreStructural = []Prim{
	{Name: "if", Kind: "cond", Pure: true},
	{Name: "let", Kind: "let", Pure: true},
	{Name: "loop", Kind: "iterate", Pure: true},
	// A table's two presentations and its domain bound (tables.md §2). Pure:
	// a table is a value, and reading one has no effect — which is what
	// separates `(array V)` from ADR 0018's `(buffer V)`, whose reads are impure
	// precisely so that stores stay ordered.
	{Name: "array", Kind: "array", Pure: true},
	{Name: "table", Kind: "table", Pure: true},
	{Name: "len", Kind: "len", Pure: true},
}

// eqSpellings are how a target may spell integer equality, most preferred
// first. `addCore` finds one and gives `=` its emission — so `=` is the
// LANGUAGE's name for the equality the target already has, rather than a second
// implementation of it. JavaScript is why the list is ordered: it declares both
// `===` and `==`, and a tag test wants the strict one.
var eqSpellings = []string{"===", "==", "sete"}

// addCore gives every target the conditional.
//
// `if` was declared by each of eleven target files, identically, while
// `core/read.go` already emitted the name when desugaring a loop — so a target
// that spelled it anything else would have compiled straight-line code and
// failed on every loop. It is the language's (ADR 0017), and the backends still
// implement it: `cond` remains a structural KIND, it is just no longer a
// structural DECLARATION.
//
// `let` and `loop` are here for exactly that argument, generalised late. The
// reader desugars `let`, `seq` and `loop` into applications of those precise
// names, so a target spelling either differently breaks every program — the
// declaration could only ever be written one way and was a fiction. It was 22
// identical lines across eleven files that a third-party author could forget,
// and forgetting one made an ADR 0015 language construct silently unavailable.
//
// The general rule: a construct promoted to the language works on EVERY target
// and the compiler finds the implementation. A target may not decline one, and
// may not declare one either.
func (tg *Target) addCore() {
	for _, p := range coreStructural {
		if _, have := tg.Prims[p.Name]; have {
			continue
		}
		tg.Prims[p.Name] = p
		tg.Names = append(tg.Names, p.Name)
	}
	// `=` is integer equality, and it is injected because `match` desugars to
	// it (core/read.go). The language has no GENERAL equality — `==` is
	// target-native on all four and disagrees on floats and strings — but
	// equality on a FINITE type is portable and total, and a match guard is
	// exactly that: a tag against a literal.
	//
	// Its emission is the target's own, found rather than written twice.
	if _, have := tg.Prims["="]; !have {
		if eq, ok := tg.findEq(); ok {
			eq.Name = "="
			eq.Args = []string{"int", "int"}
			eq.Result = "bool"
			eq.Pure = true
			tg.Prims["="] = eq
			tg.Names = append(tg.Names, "=")
		}
	}
	sort.Strings(tg.Names)
}

// findEq returns the target's own integer equality, by spelling.
func (tg *Target) findEq() (Prim, bool) {
	for _, want := range eqSpellings {
		for name, p := range tg.Prims {
			seg := name
			if i := strings.LastIndex(seg, "."); i >= 0 {
				seg = seg[i+1:]
			}
			if seg == want && len(p.Args) == 2 && p.Kind == "expr" {
				return p, true
			}
		}
	}
	return Prim{}, false
}

func loadTargetFile(path string) (*Target, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	terms, err := core.ReadAll(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(terms) != 1 {
		return nil, fmt.Errorf("%s: expected one (target …) form, got %d", path, len(terms))
	}
	return parseTarget(terms[0], path)
}

func loadTargetDir(dir string) (*Target, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".oro") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files) // deterministic diagnostics; merging is order-independent
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: a target directory needs at least one .oro file", dir)
	}
	var out *Target
	for _, f := range files {
		part, err := loadTargetFile(f)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = part
			continue
		}
		if part.Name != out.Name {
			return nil, fmt.Errorf("%s declares target %q, but %s declares %q — a directory is "+
				"one target", f, part.Name, dir, out.Name)
		}
		if err := out.merge(part, f); err != nil {
			return nil, err
		}
	}
	sort.Strings(out.Names)
	out.addCore()
	return out, nil
}

// merge folds one file's declarations into the target. Union, no precedence.
func (tg *Target) merge(o *Target, from string) error {
	for n, ty := range o.Types {
		if have, dup := tg.Types[n]; dup && have != ty {
			return fmt.Errorf("%s: type %s is declared as %q and as %q", from, n, have, ty)
		}
		tg.Types[n] = ty
	}
	if o.ArrayType != "" {
		if tg.ArrayType != "" && tg.ArrayType != o.ArrayType {
			return fmt.Errorf("%s: array-type is declared as %q and as %q",
				from, tg.ArrayType, o.ArrayType)
		}
		tg.ArrayType = o.ArrayType
	}
	for n, p := range o.Prims {
		if _, dup := tg.Prims[n]; dup {
			return fmt.Errorf("%s: %s is declared twice in this target", from, n)
		}
		tg.Prims[n] = p
		tg.Names = append(tg.Names, n)
	}
	tg.Data = append(tg.Data, o.Data...)
	for _, pair := range []struct {
		dst  *string
		src  string
		what string
	}{
		{&tg.Narrow, o.Narrow, "narrow"},
		{&tg.Artifact, o.Artifact, "artifact"},
		{&tg.Build, o.Build, "build"},
	} {
		if pair.src == "" {
			continue
		}
		if *pair.dst != "" && *pair.dst != pair.src {
			return fmt.Errorf("%s: %s is declared twice in this target", from, pair.what)
		}
		*pair.dst = pair.src
	}
	return nil
}

func parseTarget(t *core.Term, path string) (*Target, error) {
	if t.Kind != core.KApp || t.Kids[0].Kind != core.KName || t.Kids[0].Name != "target" {
		return nil, fmt.Errorf("%s: expected (target NAME …)", path)
	}
	if len(t.Kids) < 2 || t.Kids[1].Kind != core.KName {
		return nil, fmt.Errorf("%s: target needs a name", path)
	}
	tg := &Target{Name: t.Kids[1].Name, Types: map[string]string{}, Prims: map[string]Prim{}}

	for _, f := range t.Kids[2:] {
		if f.Kind != core.KApp || f.Kids[0].Kind != core.KName {
			return nil, fmt.Errorf("%s: expected (type …) or (prim …), got %s", path, f)
		}
		switch f.Kids[0].Name {
		case "array-type":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (array-type \"[]%%s\"), got %s", path, f)
			}
			tg.ArrayType = f.Kids[1].Str
		case "type":
			if len(f.Kids) != 3 || f.Kids[1].Kind != core.KName || f.Kids[2].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (type NAME \"spelling\"), got %s", path, f)
			}
			tg.Types[f.Kids[1].Name] = f.Kids[2].Str
		case "prim":
			if err := tg.declare(f, "", path); err != nil {
				return nil, err
			}
		case "artifact":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (artifact \"name\"), got %s", path, f)
			}
			tg.Artifact = f.Kids[1].Str
		case "build":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (build \"cmd %%s %%s\"), got %s", path, f)
			}
			tg.Build = f.Kids[1].Str
		case "data":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (data \"label ...\"), got %s", path, f)
			}
			tg.Data = append(tg.Data, f.Kids[1].Str)
		case "narrow":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (narrow \"dst = src[:n]\"), got %s", path, f)
			}
			tg.Narrow = f.Kids[1].Str
		case "structural":
			// (structural NAME KIND [pure]) — the four the backend implements.
			// They carry NO TYPES, because fold-range is
			// A x int x ((A,int) -> A) -> A and that cannot be written in a
			// monomorphic table. Writing (f64 int any) f64 was a false
			// statement in every target file (target-files.md §4).
			p, err := parseStructural(f, path)
			if err != nil {
				return nil, err
			}
			if _, dup := tg.Prims[p.Name]; dup {
				return nil, fmt.Errorf("%s: %s is declared twice", path, p.Name)
			}
			tg.Prims[p.Name] = p
			tg.Names = append(tg.Names, p.Name)
		case "module":
			// (module PATH (prim …) …) — the names this target provides
			// NATIVELY from that module. A target may provide any subset,
			// including none, which is what makes porting demand-driven
			// (modules.md §4).
			if len(f.Kids) < 2 || f.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("%s: (module PATH (prim …)…), got %s", path, f)
			}
			for _, inner := range f.Kids[2:] {
				if inner.Kind != core.KApp || inner.Kids[0].Kind != core.KName ||
					inner.Kids[0].Name != "prim" {
					return nil, fmt.Errorf("%s: module %s may only contain (prim …), got %s",
						path, f.Kids[1].Name, inner)
				}
				if err := tg.declare(inner, f.Kids[1].Name, path); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unknown target form %q", path, f.Kids[0].Name)
		}
	}
	sort.Strings(tg.Names)
	return tg, nil
}

// declare records one primitive under a module path. The name stored is the
// FULLY QUALIFIED one, because that is what resolution produces and R1 requires
// targets and libraries to key the same namespace (modules.md §5).
func (tg *Target) declare(f *core.Term, modPath, file string) error {
	p, err := parsePrim(f, file)
	if err != nil {
		return err
	}
	// A MODULE may declare `and` — that is `logic.and`, a qualified name like
	// any other. Only an unqualified declaration collides with the language.
	if modPath == "" && coreNames[p.Name] {
		return fmt.Errorf("%s: %s belongs to the language and cannot be declared by a target "+
			"(docs/spec/booleans.md)", file, p.Name)
	}
	if modPath != "" {
		p.Name = modPath + "." + p.Name
		if p.Checked != "" {
			p.Checked = modPath + "." + p.Checked
		}
	}
	if _, dup := tg.Prims[p.Name]; dup {
		return fmt.Errorf("%s: %s is declared twice", file, p.Name)
	}
	tg.Prims[p.Name] = p
	tg.Names = append(tg.Names, p.Name)
	return nil
}

// (prim NAME (ARGTYPES…) RESULT KIND ["form"] [(import "x")])
func parsePrim(f *core.Term, path string) (Prim, error) {
	k := f.Kids[1:]
	if len(k) < 4 {
		return Prim{}, fmt.Errorf("%s: (prim NAME (args) result kind [form] [(import x)]), got %s", path, f)
	}
	if k[0].Kind == core.KBool {
		return Prim{}, fmt.Errorf("%s: `%s` is a literal of the language, not a name a target "+
			"declares (docs/spec/booleans.md)", path, k[0])
	}
	if k[0].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: prim needs a name, got %s", path, k[0])
	}
	p := Prim{Name: k[0].Name}

	// Argument types. `()` reads as an empty list, which the reader rejects, so
	// a nullary primitive writes `(none)`.
	if k[1].Kind == core.KApp {
		for _, a := range k[1].Kids {
			switch {
			case a.Kind == core.KName:
				if a.Name != "none" {
					p.Args = append(p.Args, a.Name)
					p.Names = append(p.Names, "")
				}
			case a.Kind == core.KApp && len(a.Kids) == 2 &&
				a.Kids[0].Kind == core.KName && a.Kids[1].Kind == core.KName:
				// The named form, the same one `sig` uses — because a
				// refinement attaches to a NAME (refinements.md §2).
				p.Names = append(p.Names, a.Kids[0].Name)
				p.Args = append(p.Args, a.Kids[1].Name)
			default:
				return Prim{}, fmt.Errorf("%s: %s has a bad argument: %s", path, p.Name, a)
			}
		}
	} else if k[1].Kind == core.KName && k[1].Name != "none" {
		p.Args = []string{k[1].Name}
	}

	if k[2].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: %s has a non-name result type", path, p.Name)
	}
	p.Result = k[2].Name

	if k[3].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: %s has a non-name kind", path, p.Name)
	}
	p.Kind = k[3].Name
	switch p.Kind {
	case "expr", "stmt":
	case "loop", "loop2", "cond", "let":
		return Prim{}, fmt.Errorf("%s: %s is %s, which is structural and carries no types; "+
			"write (structural %s %s [pure])", path, p.Name, p.Kind, p.Name, p.Kind)
	default:
		return Prim{}, fmt.Errorf("%s: %s has unknown kind %q (expr, stmt)", path, p.Name, p.Kind)
	}

	for _, rest := range k[4:] {
		switch {
		case rest.Kind == core.KStr:
			p.Form = rest.Str
		case rest.Kind == core.KName && rest.Name == "pure":
			p.Pure = true
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "where" && len(rest.Kids) == 2:
			p.Where = rest.Kids[1]
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "checked" && len(rest.Kids) == 2 &&
			rest.Kids[1].Kind == core.KName:
			p.Checked = rest.Kids[1].Name
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "jump" && len(rest.Kids) >= 2 && rest.Kids[1].Kind == core.KStr:
			p.Jump = rest.Kids[1].Str
			if len(rest.Kids) == 3 && rest.Kids[2].Kind == core.KStr {
				p.JumpForm = rest.Kids[2].Str
			} else if len(rest.Kids) != 2 {
				return Prim{}, fmt.Errorf("%s: %s: (jump \"cc\" [\"compare form\"]), got %s",
					path, p.Name, rest)
			}
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			(rest.Kids[0].Name == "length" || rest.Kids[0].Name == "length-of") &&
			len(rest.Kids) == 2 && rest.Kids[1].Kind == core.KInt:
			n := int(rest.Kids[1].Int)
			if n < 0 || n >= len(p.Args) {
				return Prim{}, fmt.Errorf("%s: %s: (%s %d) names argument %d, "+
					"which it does not have", path, p.Name, rest.Kids[0].Name, n, n)
			}
			if rest.Kids[0].Name == "length" {
				p.Length = n + 1
			} else {
				p.LengthOf = n + 1
			}
		case rest.Kind == core.KName && rest.Name == "index":
			if len(p.Args) != 2 {
				return Prim{}, fmt.Errorf("%s: %s is marked index but does not take "+
					"a container and an index", path, p.Name)
			}
			p.Index = true
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "import" && len(rest.Kids) == 2 && rest.Kids[1].Kind == core.KStr:
			p.Import = rest.Kids[1].Str
		default:
			return Prim{}, fmt.Errorf("%s: %s has an unexpected trailing form %s", path, p.Name, rest)
		}
	}
	if !structuralKinds[p.Kind] && p.Form == "" {
		return Prim{}, fmt.Errorf("%s: %s is %s and needs an emission template", path, p.Name, p.Kind)
	}
	return p, nil
}

// parseStructural reads (structural NAME KIND [pure]). No argument types, no
// result type, no template — a structural primitive's types and its emission
// both live in the backend, which is the only place either can be expressed.
func parseStructural(f *core.Term, path string) (Prim, error) {
	k := f.Kids[1:]
	if len(k) < 2 || k[0].Kind != core.KName || k[1].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: (structural NAME KIND [pure]), got %s", path, f)
	}
	p := Prim{Name: k[0].Name, Kind: k[1].Name}
	if coreNames[p.Name] {
		return Prim{}, fmt.Errorf("%s: %s belongs to the language and cannot be declared by a "+
			"target. Delete the line; `if`, `let` and `loop` are injected into every target, "+
			"and the backend implements them (docs/spec/core-0.md)", path, p.Name)
	}
	if !structuralKinds[p.Kind] {
		return Prim{}, fmt.Errorf("%s: %s has kind %q, which is not structural "+
			"(let, cond, loop, loop2, build, iterate)", path, p.Name, p.Kind)
	}
	for _, rest := range k[2:] {
		if rest.Kind == core.KName && rest.Name == "pure" {
			p.Pure = true
			continue
		}
		return Prim{}, fmt.Errorf("%s: %s has an unexpected trailing form %s", path, p.Name, rest)
	}
	return p, nil
}

// Env builds the reduction environment. Which names are primitive is exactly
// what the target file declares, so this is the whole of ADR 0002's parameter.
//
// Purity travels with it, and its default is the point (effects.md §3): a target
// author who forgets `pure` gets a slower program, where one who forgot an
// `effect` marker under the opposite default would get a silent miscompilation.
// The default must be the one whose failure mode is slow, not wrong.
func (tg *Target) Env(p *core.Program) (*core.Env, error) {
	e := &core.Env{
		Defs: p.Defs,
		Prim: map[string]bool{},
		Pure: map[string]bool{},
		Rec:  map[string]bool{},
	}
	e.SetUnresolved(p.Unresolved)
	for _, n := range tg.Names {
		e.Prim[n] = true
		e.Pure[n] = tg.Prims[n].Pure
	}
	// `let` reaches the reducer only in a residual it produced itself, so it is
	// primitive here without being declared. Its own application does nothing —
	// whether a let is pure is decided by its value and its body, which the
	// judgement reads through.
	e.Prim["let"] = true
	e.Pure["let"] = true
	e.MarkRecursive()
	return e, e.CheckDefs()
}

// fill applies a template to operands, cycling them to cover however many holes
// the template has. `%s[%s]++` names two operands once each; JS's dictionary
// increment names the same two twice; `fmt.Println(%s)` names its one operand
// once. Repeating a fixed number of times worked only while every stmt
// primitive was a dictionary update.
// Fill applies a template to string operands, ignoring any it does not use.
// `go build -o %s %s` wants both; `node --check %s` wants one.
func Fill(form string, args ...string) string {
	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = a
	}
	return fill(form, vals)
}

func fill(form string, vals []any) string {
	holes := strings.Count(form, "%s")
	if len(vals) == 0 || holes <= len(vals) {
		return fmt.Sprintf(form, vals[:min(holes, len(vals))]...)
	}
	out := make([]any, holes)
	for i := range out {
		out[i] = vals[i%len(vals)]
	}
	return fmt.Sprintf(form, out...)
}

// ty spells one of our type names in the target's own language. An untyped
// target declares no types and this is never consulted.
func (tg *Target) ty(name string) string {
	if s, ok := tg.Types[name]; ok {
		return s
	}
	// `(array V)` resolves through ONE declaration — `(array-type "[]%s")` on
	// Go, `"%s[]"` on Java — instead of enumerating an entry per element type.
	//
	// That enumeration is what tables.md §10 called "the suffix explosion", and
	// it is the surface this construct deletes: Go declared seven `slice-*`
	// types and nineteen `at-*`/`make-*`/`set-*`/`len` primitives, and the four
	// targets together declared 54. They existed because the type language had
	// no constructor.
	if elem := core.ArrayElem(name); elem != "" {
		if tg.ArrayType != "" {
			return Fill(tg.ArrayType, tg.ty(elem))
		}
		// A target with no types — JavaScript, windows — spells an array
		// nothing at all, which is why neither declares one.
		return ""
	}
	if name == "" {
		return "/*unknown*/"
	}
	return "/*" + name + "?*/"
}

// WriteProgram lays out a complete, buildable source tree for one entry point.
//
// Module structure does not survive into the artifact: reduction is
// whole-program and fusion crosses every boundary, so there is nothing smaller
// than the program to emit separately (build.md §3). One artifact, shaped by the
// target.
func (tg *Target) WriteProgram(dir, code, entry string) error {
	switch tg.Name {
	case "go":
		var b strings.Builder
		b.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\npackage main\n\n")
		if len(Imports) > 0 {
			b.WriteString("import (\n")
			for _, imp := range sortedSet(Imports) {
				fmt.Fprintf(&b, "\t%q\n", imp)
			}
			b.WriteString(")\n\n")
		}
		b.WriteString(code)
		// Go's entry point takes no arguments and returns nothing, so the
		// emitted function's value is discarded here rather than in the
		// language, which has no notion of discarding one.
		fmt.Fprintf(&b, "\nfunc main() {\n\t%s()\n}\n", export(entry))
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(b.String()), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("module oroprog\n\ngo 1.21\n"), 0o644)

	case "js":
		// One ES module. The entry is called at load, which is what running a
		// module means on this host: there is no separate entry convention to
		// honour, so there is nothing to wrap.
		var b strings.Builder
		b.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
		b.WriteString(code)
		fmt.Fprintf(&b, "\n%s();\n", jsMangle(entry))
		return os.WriteFile(filepath.Join(dir, "main.mjs"), []byte(b.String()), 0o644)

	case "java":
		// One class. Java's entry is fixed by the JVM -- public static void
		// main(String[]) -- so the emitted function is called from it and its
		// value discarded, exactly as on Go.
		var b strings.Builder
		b.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
		for _, imp := range sortedSet(JavaImports) {
			fmt.Fprintf(&b, "import %s;\n", imp)
		}
		if len(JavaImports) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("public final class Main {\n")
		b.WriteString(code)
		fmt.Fprintf(&b, "\n\tpublic static void main(String[] args) {\n\t\t%s();\n\t}\n}\n",
			javaMangle(entry))
		return os.WriteFile(filepath.Join(dir, "Main.java"), []byte(b.String()), 0o644)

	case "windows":
		// One translation unit and one batch file. The batch file exists
		// because of a limitation this target was the first to hit: Build is
		// split on whitespace and run without a shell, and every Windows
		// toolchain lives under `C:\Program Files\`. `go`, `node` and `javac`
		// are all bare words on PATH, so no target had ever needed a path with
		// a space in it. Discovery moves into the batch file, which is a
		// workaround and is recorded as one (windows-target.md 6).
		if err := os.WriteFile(filepath.Join(dir, "main.asm"), []byte(code), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "build.bat"), []byte(asmBuildBat), 0o644)
	}
	return fmt.Errorf("target %q has no program layout", tg.Name)
}

// asmBuildBat finds MASM and the linker, then runs them.
//
// It exists because Build is split on whitespace and run without a shell, and
// every Windows toolchain lives under a path with a space in it. `go`, `node`
// and `javac` are bare words on PATH, so no target had ever needed more than
// one command or a quoted path. Discovery therefore moves into a script the
// target writes — which works, and is a workaround (windows-target.md 6).
const asmBuildBat = `@echo off
setlocal enabledelayedexpansion
set "VCV="
for %%p in ("%ProgramFiles%\Microsoft Visual Studio" "%ProgramFiles(x86)%\Microsoft Visual Studio") do (
  for /d %%v in ("%%~p\*") do (
    for /d %%e in ("%%~v\*") do (
      if exist "%%~e\VC\Auxiliary\Build\vcvars64.bat" set "VCV=%%~e\VC\Auxiliary\Build\vcvars64.bat"
    )
  )
)
if not defined VCV (echo build.bat: no MSVC toolchain with ml64 was found & exit /b 1)
call "!VCV!" >nul || exit /b 1
ml64 -nologo -c -Fomain.obj main.asm || exit /b 1
link -nologo -subsystem:console -entry:main main.obj kernel32.lib msvcrt.lib legacy_stdio_definitions.lib -out:main.exe || exit /b 1
`

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// atomicValue reports whether an EMITTED expression is free to repeat: an
// identifier or a literal. It is the emitter's half of core's `duplicable`,
// and it exists for one reason — the VALUE of a `stmt` primitive is its first
// argument, so returning that argument's expression writes it twice.
//
//	fmt.Println((strings.Fields(s)))
//	return (strings.Fields(s))
//
// Two allocations where the source asked for one, in a compiler whose whole
// call-by-need discipline exists to prevent exactly that. Found writing
// chapter 4.
//
// The test is on the emitted STRING rather than the term, because a term that
// is not atomic often emits to one that is: a fold-range emits its loop and
// yields the accumulator's name.
func atomicValue(v string) bool {
	if v == "" {
		return false
	}
	if v[0] == '"' || (v[0] >= '0' && v[0] <= '9') || v[0] == '-' {
		return true // a literal
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		ok := c == '_' || c == '$' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(i > 0 && c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// seedFromSig gives the parameters the types their signature declares.
//
// The signature was already CHECKED against this residual (types.md §7) and
// then thrown away, so `(sig f ((n int)) f64)` over
// `(fn (n) (fold-range 0.0 n …))` still failed to emit: the emitter re-infers
// from primitive argument types alone, and a loop bound is a structural
// primitive with no table entry. The program was correct, the claim about it
// was verified, and the compiler refused it anyway.
//
// Seeded by POSITION, not by name — the residual's parameter names are hints
// (chapter 1 §1.6) and a signature's names exist for refinements to attach to.
// Arity is already checked by CheckSignatures.
func seedFromSig(types map[string]string, params []string, sig *core.Sig) {
	if sig == nil {
		return
	}
	for i, p := range params {
		if i >= len(sig.Params) {
			return
		}
		if ty := sig.Params[i].Type; ty != "" && ty != "any" {
			types[p] = ty
		}
	}
}

// openFresh opens an abstraction with names that do not collide with anything
// already emitted in the enclosing function, and reports both the raw names
// (which key the type maps) and the mangled ones (which appear in the output).
//
// The emitters had been using each parameter's NAME HINT directly. Hints are
// not unique — chapter 1 §1.6 — so two nested folds whose steps both say `acc`
// and `i`, which are the obvious names, emitted `for i := …` inside
// `for i := …` and `acc := acc`. The outer accumulator was then never written
// and the function returned its initial value: a **silent wrong answer**, found
// by the first program written against `go/builtin`.
//
// Opening through OpenWith rather than Body() is what makes the repair safe.
// The substitution happens on the CLOSED representation, where the two binders
// are genuinely distinct, so renaming one cannot capture the other. This is the
// locally nameless representation earning its keep a second time, in a pass
// that had quietly assumed names were unique.
func openFresh(t *core.Term, taken map[string]bool, mangle func(string) string) (
	body *core.Term, raw []string, out []string) {

	raw = make([]string, len(t.Params))
	out = make([]string, len(t.Params))
	args := make([]*core.Term, len(t.Params))
	for i, p := range t.Params {
		cand := p
		for k := 2; taken[mangle(cand)]; k++ {
			cand = fmt.Sprintf("%s%d", p, k)
		}
		taken[mangle(cand)] = true
		raw[i], out[i] = cand, mangle(cand)
		args[i] = core.Name(cand)
	}
	return t.OpenWith(args), raw, out
}

// changedArgs reports which `again` arguments are not the loop variable itself.
//
// hamza's optimisation: an unchanged variable needs no assignment at all, which
// removes noise from the output AND shrinks the simultaneity problem, since an
// unchanged variable cannot be clobbered by another.
func changedArgs(as []*core.Term, raw []string) []int {
	var out []int
	for i, a := range as {
		if a.Kind == core.KName && a.Name == raw[i] {
			continue
		}
		out = append(out, i)
	}
	return out
}

// needTemps reports whether the simultaneous update needs temporaries: only if
// some changed argument READS a variable that is itself being changed. Go has
// parallel assignment and never asks; JS and Java do.
func needTemps(as []*core.Term, raw []string, changed []int) bool {
	if len(changed) < 2 {
		return false
	}
	for _, i := range changed {
		// Reading your OWN old value is safe: `i = i + 1` needs no temporary.
		// Only reading a DIFFERENT variable that is also being changed does.
		others := map[string]bool{}
		for _, j := range changed {
			if j != i {
				others[raw[j]] = true
			}
		}
		if readsAny(as[i], others) {
			return true
		}
	}
	return false
}

func readsAny(t *core.Term, names map[string]bool) bool {
	switch t.Kind {
	case core.KName:
		return names[t.Name]
	case core.KFn:
		return readsAny(t.Body(), names)
	case core.KApp:
		for _, k := range t.Kids {
			if readsAny(k, names) {
				return true
			}
		}
	}
	return false
}

// soleExit reports the name every exit clause of a loop yields, when they all
// yield the SAME name already in scope. "" means a result temporary is needed.
//
// Not only tidiness: on Go the extra `var r1 []bool` defeated escape analysis,
// and on JS it leaves a bare `r2;` expression statement in the output.
//
// Called BEFORE the body is emitted, so `bound` holds exactly the enclosing
// scope plus the loop's own variables. A name bound later, inside the body, is
// not in scope after the loop and is correctly refused.
func soleExit(prims map[string]Prim, t *core.Term, raw, names []string,
	bound map[string]bool, mangle func(string) string) string {

	inScope := make(map[string]bool, len(bound))
	for n := range bound {
		inScope[n] = true
	}
	seen := map[string]bool{}
	var walk func(*core.Term) bool
	walk = func(t *core.Term) bool {
		if isAgain(t) {
			return true
		}
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, ok := prims[t.Op().Name]; ok {
				if p.Kind == "cond" && len(t.Args()) == 3 {
					return walk(t.Args()[1]) && walk(t.Args()[2])
				}
				if p.Kind == "let" && len(t.Args()) == 2 {
					if k := t.Args()[1]; k.Kind == core.KFn && len(k.Params) == 1 {
						return walk(k.Body())
					}
				}
			}
		}
		if t.Kind != core.KName {
			return false
		}
		for i, r := range raw {
			if r == t.Name {
				seen[names[i]] = true
				return true
			}
		}
		if m := mangle(t.Name); inScope[m] {
			seen[m] = true
			return true
		}
		return false
	}
	if !walk(t) || len(seen) != 1 {
		return ""
	}
	for n := range seen {
		return n
	}
	return ""
}

// IndexingErr is the diagnostic for `(x i)` where x is neither a primitive nor
// a table.
//
// tables.md §3.4 asks for a message that says what the coder did, not "no form
// for primitive x". The name is in operator position, so they indexed it; the
// question is what it is.
func IndexingErr(host, name string) error {
	return fmt.Errorf("%s is applied to an argument, so it is being used as a table or a "+
		"function — and it is neither.\n"+
		"  Indexing IS application here: `(a i)` is the element of `a` at `i` (docs/spec/tables.md).\n"+
		"  A table is `(array e…)`, `(table n f)`, or a parameter declared `(array V)`.\n"+
		"  If %s was meant to be a function, it did not survive reduction — a function that\n"+
		"  escapes is a closure, and closures are refused (docs/spec/callbacks.md).\n"+
		"  [%s backend]", name, name, host)
}

// IsTableOperand reports whether the operator of an application is a LOCAL
// NAME, which in a residual can only be a table.
//
// The invariant is tables.md §3.2 and it is what makes `(a i)` unambiguous
// before any type is consulted: a function passed as an argument is substituted
// and its application reduces; a function that survives is an escaping closure
// and is refused. So the slot is empty, and a variable in operator position is
// an indexing.
func IsTableOperand(name string, bound map[string]bool) bool {
	return bound[name]
}

// UnallocatedTableErr is the refusal for a `(table n f)` that reached a backend.
//
// A rule-table has NO MEMORY — it is a length and a function, and its whole
// purpose is to fuse away. One that survives to emission is a table nobody
// asked to exist at runtime, and the fix is to say where the memory goes.
func UnallocatedTableErr() error {
	return fmt.Errorf("a `(table n f)` reached the backend, and a rule-table has no memory.\n" +
		"  It is a length and a function; its purpose is to FUSE, and this one did not.\n" +
		"  Wrap it in `(alloc …)` to say that the elements should exist in memory —\n" +
		"  and note that materialising in the interior of a computation is what costs\n" +
		"  (docs/spec/construction.md); at a boundary it is what you want.")
}
