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

	// Ensures is a POSTCONDITION: what the call GUARANTEES about its result,
	// over the parameter names and `result`. It is the only kind of contract a
	// primitive cannot have derived for it, because a primitive has no body —
	// which is why postconditions live here and are redundant on an internal
	// definition (postconditions.md §3).
	//
	// It is ASSUMED at a call site, and only where the primitive's own `Where`
	// was DISCHARGED. A contract is an implication: with the precondition
	// unproven the guarantee says nothing, and assuming it anyway puts a false
	// fact into a conjunctive fragment, from which everything follows.
	Ensures *core.Term

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

	// MapType is how this target spells a map from something to something —
	// `map[%s]%s` on Go, `java.util.HashMap<%s,%s>` on Java. One declaration
	// replaces an entry per (K, V) pair, which is the suffix explosion squared:
	// `targets/java/util.oro` says so in as many words, having had to declare
	// `Map<String,Long>` and nothing else.
	// Empty means the target has no types to spell (JavaScript, windows).
	MapType string

	// Boxed is how this target spells a type when it must be an OBJECT rather
	// than a primitive — `Long` for `long` on the JVM. Declared, not hardcoded:
	// which types a host boxes and what it calls them is a host fact, and host
	// facts live in target files.
	//
	// Only Java declares any. Everywhere else a boxed type is the type, so
	// `boxed` falls through to `ty` and nothing changes.
	Boxed map[string]string

	// BuiltinMap says this host ships no map of its own, so the language
	// supplies one — `emit/winmap.oro`, lowered into buffers and loops before
	// reduction.
	//
	// Declared rather than inferred, because there is nothing to infer it from:
	// an empty `MapType` means "no map" on windows and "no TYPES" on
	// JavaScript, which has a perfectly good map and spells nothing. A host
	// fact belongs in a target file whichever way it points.
	BuiltinMap bool

	// Reprs are the integer representations this target can store, narrowest
	// first, declared as `(int-repr LO HI "spelling")`. A range type selects
	// the first one that CONTAINS it — ADR 0003's "the compiler selects the
	// representation that fits", moved out of Go and into the target file where
	// every other host fact lives.
	//
	// A target that declares none stores every integer the one way it already
	// does, which is the right answer for JavaScript: it has no integers, and a
	// plain packed Array measured FASTER than a Uint8Array
	// (jsontok-2026-08-26).
	// MaxLen is the largest number of elements a table can have on this
	// target, or 0 for "no tighter than the language's own bound".
	//
	// A LENGTH IS BOUNDED WITHOUT ANY DECLARATION, and that is a LANGUAGE fact
	// rather than a host one. `(len t)` returns an `int`, and ADR 0012 says
	// `int` is exact within ±(2^53−1); a table with more elements than that has
	// a length this language cannot count exactly, so it is outside the
	// language and every guarantee about indexing it has already failed. So the
	// analysis may assume `(len t) ≤ 2^53−1` everywhere, assuming nothing ADR
	// 0012 did not already require. See MaxLenOf and docs/spec/tables.md §2.3.
	//
	// A target may say something TIGHTER, and one of them can: a Java array
	// holds at most 2^31−1 elements because `arraylength` returns an `int`.
	// That is the same shape as `int-repr` — the host declaring what it can
	// hold — and it is the fact indextype-2026-08-25 hardcoded in Go.
	MaxLen int64

	Reprs []IntRepr
	Prims map[string]Prim
	Names []string // every primitive name, for core.Env

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
	// THE WRITE SIDE — ADR 0018. Values are immutable; mutation exists only
	// inside `build`, whose buffer is linear and is frozen on the way out.
	//
	//	(alloc t)               a rule, in memory. GATHER — pure, parallel.
	//	(build n (fn (b) …))    a scoped mutable buffer. SCATTER — sequential.
	//	(set b i v)             a store; consumes b, returns b.
	//
	// `(table n f)` is a gather and cannot express a scatter, so the sieve,
	// in-place sorting, histograms, union-find and general dynamic programming
	// are inexpressible portably AT ANY SPEED without this. That is what
	// decided ADR 0018 — expressiveness, not the 2.7x.
	"alloc": true, "build": true, "set": true,
	"map": true, "build-map": true, "insert": true,
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
	// A MAP LITERAL — a table given by its graph, where the index set is not
	// implicit so the graph carries both columns (maps.md §3.1).
	//
	// Pure, for `array`'s reason: it is a value and reading one has no effect.
	// It overloads `map` between the type language and the term language
	// exactly as `array` already does, disambiguated by position — a type only
	// ever appears in a `sig` or a `prim`, which read syntax rather than terms.
	{Name: "map", Kind: "map", Pure: true},
	// IMPURE, all three, and that is the sequencing mechanism rather than an
	// omission. ADR 0010 never substitutes an impure argument, which denies
	// contraction (no duplicated store), weakening (no dropped store) and
	// exchange (no reordered store) — the three properties a mutable buffer
	// needs, and they were built for `print-line`.
	//
	// `alloc` and `build` are impure because they ALLOCATE: duplicating one
	// duplicates the allocation. ADR 0018 calls `alloc` pure in the
	// referential-transparency sense, which is true and is not the property β
	// needs here.
	// The LENGTH attributes are 1-based, matching the reader. `build` makes a
	// buffer as long as its count; `alloc` and `set` pass their argument's
	// length through. Without them a program cannot prove its own index, because
	// nothing relates the buffer to the number it was made from.
	{Name: "alloc", Kind: "table-alloc", LengthOf: 1},
	{Name: "build", Kind: "table-build", Length: 1},
	{Name: "set", Kind: "table-set", LengthOf: 1},
	// A MAP BUFFER and its store (maps.md §3.3). Identical in discipline to
	// `build`/`set` and impure for the same reasons — allocation for the first,
	// sequencing for the second.
	//
	// arrays-revisited.md §6 derives rather than chooses this: the discipline
	// is about ALIASING, and aliasing does not care what the index set is. So a
	// growing map is ADR 0018's linear buffer with `I = S ⊆ K` instead of
	// `I = Fin n`, and `occurrences` is the check, unchanged.
	//
	// `build-map` takes a CAPACITY, and that is the load-bearing decision of
	// maps.md §6 rather than a performance hint: windows ships no map, a
	// growing hash table must rebuild into a larger allocation, and that is not
	// expressible. Letting three hosts grow and windows not is an OBSERVABLE
	// disagreement, which is a Tier 2 construct in the core.
	//
	// No LengthOf on `insert`: `|dom m|` after an insert is `|dom m|` or one
	// more, because whether the key was already present is a fact about the
	// input (growth.md §1.1). Append keeps an equation; insert keeps only an
	// interval, and claiming the equation here would be unsound.
	{Name: "build-map", Kind: "map-build"},
	{Name: "insert", Kind: "map-insert"},
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
	for n, ty := range o.Boxed {
		if tg.Boxed == nil {
			tg.Boxed = map[string]string{}
		}
		if have, dup := tg.Boxed[n]; dup && have != ty {
			return fmt.Errorf("%s: boxed %s is declared as %q and as %q", from, n, have, ty)
		}
		tg.Boxed[n] = ty
	}
	if o.BuiltinMap {
		tg.BuiltinMap = true
	}
	if o.MapType != "" {
		if tg.MapType != "" && tg.MapType != o.MapType {
			return fmt.Errorf("%s: map-type is declared as %q and as %q",
				from, tg.MapType, o.MapType)
		}
		tg.MapType = o.MapType
	}
	if o.MaxLen != 0 {
		if tg.MaxLen != 0 && tg.MaxLen != o.MaxLen {
			return fmt.Errorf("%s: max-len is declared as %d and as %d",
				from, tg.MaxLen, o.MaxLen)
		}
		tg.MaxLen = o.MaxLen
	}
	// Representations are ORDERED, so they append rather than merging by key —
	// narrowest first is the whole selection rule. A target that splits them
	// across files gets them in file order, which is the order it wrote them.
	tg.Reprs = append(tg.Reprs, o.Reprs...)
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
		case "int-repr":
			// `(int-repr LO HI "spelling")`. Signedness is not a concept here
			// and does not need to be: a host that cannot store 0..255 in its
			// byte — the JVM, whose `byte` is signed — simply does not declare
			// that range for it, and the range selects the next one up. The
			// declaration says what the host CAN hold, and nothing else.
			if len(f.Kids) != 4 || f.Kids[1].Kind != core.KInt ||
				f.Kids[2].Kind != core.KInt || f.Kids[3].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (int-repr LO HI \"spelling\"), got %s", path, f)
			}
			lo, hi := f.Kids[1].Int, f.Kids[2].Int
			if lo > hi {
				return nil, fmt.Errorf("%s: int-repr %d..%d is empty", path, lo, hi)
			}
			tg.Reprs = append(tg.Reprs, IntRepr{Lo: lo, Hi: hi, Spell: f.Kids[3].Str})
		case "max-len":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KInt || f.Kids[1].Int < 1 {
				return nil, fmt.Errorf("%s: (max-len N) with N >= 1, got %s", path, f)
			}
			if f.Kids[1].Int > portableMaxLen {
				return nil, fmt.Errorf("%s: max-len %d is outside the portable "+
					"window; a length this target cannot count exactly is not a "+
					"length (ADR 0012)", path, f.Kids[1].Int)
			}
			tg.MaxLen = f.Kids[1].Int
		case "array-type":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (array-type \"[]%%s\"), got %s", path, f)
			}
			tg.ArrayType = f.Kids[1].Str
		case "builtin-map":
			if len(f.Kids) != 1 {
				return nil, fmt.Errorf("%s: (builtin-map) takes nothing, got %s", path, f)
			}
			tg.BuiltinMap = true
		case "boxed":
			if len(f.Kids) != 3 || f.Kids[1].Kind != core.KName || f.Kids[2].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (boxed NAME \"spelling\"), got %s", path, f)
			}
			if tg.Boxed == nil {
				tg.Boxed = map[string]string{}
			}
			tg.Boxed[f.Kids[1].Name] = f.Kids[2].Str
		case "map-type":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (map-type \"map[%%s]%%s\"), got %s", path, f)
			}
			tg.MapType = f.Kids[1].Str
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
			case a.Kind == core.KApp && core.TypeName(a) != "":
				// A compound type, positional: `(array string)`.
				p.Args = append(p.Args, core.TypeName(a))
				p.Names = append(p.Names, "")
			case a.Kind == core.KApp && len(a.Kids) == 2 &&
				a.Kids[0].Kind == core.KName && a.Kids[1].Kind == core.KName:
				// The named form, the same one `sig` uses — because a
				// refinement attaches to a NAME (refinements.md §2).
				p.Names = append(p.Names, a.Kids[0].Name)
				p.Args = append(p.Args, a.Kids[1].Name)
			case a.Kind == core.KApp && len(a.Kids) == 2 &&
				a.Kids[0].Kind == core.KName && core.TypeName(a.Kids[1]) != "":
				// Named, with a compound type: `(ws (array string))`.
				p.Names = append(p.Names, a.Kids[0].Name)
				p.Args = append(p.Args, core.TypeName(a.Kids[1]))
			default:
				return Prim{}, fmt.Errorf("%s: %s has a bad argument: %s", path, p.Name, a)
			}
		}
	} else if k[1].Kind == core.KName && k[1].Name != "none" {
		p.Args = []string{k[1].Name}
	}

	// A result type may be COMPOUND: `(array string)`, the same spelling the
	// signature language uses. Without it a target's array types have to be
	// enumerated — `string-array`, `long-array`, `double-array` — which is the
	// suffix explosion `(array V)` exists to delete (tables.md §10), and it
	// showed up as `final /*unknown*/ w = ws[(int) i]` the first time a native
	// Java program indexed the result of `split`.
	if rt := core.TypeName(k[2]); rt != "" {
		p.Result = rt
	} else {
		return Prim{}, fmt.Errorf("%s: %s has a result type that is neither a name nor "+
			"`(array T)`: %s", path, p.Name, k[2])
	}

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
			rest.Kids[0].Name == "ensures" && len(rest.Kids) == 2:
			p.Ensures = rest.Kids[1]
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
	// A TARGET WITH NO MAP GETS OURS, rewritten into buffers and loops before
	// reduction so that nothing downstream learns maps exist (winmap.go).
	if err := lowerMaps(tg, p); err != nil {
		return nil, err
	}
	e.Defs = p.Defs
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

// portableMaxLen is ADR 0012's window, and it is the bound on every table's
// length that needs no declaration at all. See Target.MaxLen.
const portableMaxLen = 1<<53 - 1

// MaxLenOf is the largest length a table can have on this target: what the
// target declared, or the language's own bound when it declared nothing.
//
// Failure is not a case here, which is the point. Before this, `(len a)` was
// `[0, +inf)` and every counter under a `len` guard was unbounded — 32 of the
// corpus's unproven operations, and every one of them is `(+ i 1)` under
// `(>= i (len a))`.
func (tg *Target) MaxLenOf() int64 {
	if tg.MaxLen != 0 {
		return tg.MaxLen
	}
	return portableMaxLen
}

// IntRepr is one integer representation a target can store.
type IntRepr struct {
	Lo, Hi int64
	Spell  string
}

// reprFor picks the narrowest declared representation that holds [lo,hi], or ""
// if the target declared none that does — in which case the caller falls back
// to the target's plain `int`, which is what it would have used anyway.
//
// Narrowest wins because the declarations are searched in order and a target
// lists them narrowest first. That is a convention rather than a sort, so a
// target author can put a wider one first deliberately and be believed.
func (tg *Target) reprFor(lo, hi int64) string {
	for _, r := range tg.Reprs {
		if r.Lo <= lo && hi <= r.Hi {
			return r.Spell
		}
	}
	return ""
}

// NarrowedElem reports the host spelling for a table whose element type is a
// range NARROWER than the target's own integer, and whether there is one.
//
// This is the only question the width is ever asked. A range never narrows a
// local: `(a i)` is an integer wherever it is used, and the storage is the only
// place a target gets an opinion.
func (tg *Target) NarrowedElem(ty string) (string, bool) {
	lo, hi, ok := core.IntRange(core.ArrayElem(ty))
	if !ok {
		return "", false
	}
	spell := tg.reprFor(lo, hi)
	if spell == "" || spell == tg.ty("int") {
		return "", false
	}
	return spell, true
}

// ty spells one of our type names in the target's own language. An untyped
// target declares no types and this is never consulted.
// boxed spells a type as an object where the target says one is needed, and as
// itself everywhere else.
func (tg *Target) boxed(name string) string {
	if s, ok := tg.Boxed[core.ValueType(name)]; ok {
		return s
	}
	return tg.ty(name)
}

func (tg *Target) ty(name string) string {
	if s, ok := tg.Types[name]; ok {
		return s
	}
	// A RANGE spells itself as the representation that holds it, and falls back
	// to the target's own integer when nothing narrower was declared.
	if lo, hi, ok := core.IntRange(name); ok {
		if spell := tg.reprFor(lo, hi); spell != "" {
			return spell
		}
		return tg.ty("int")
	}
	// `(array V)` resolves through ONE declaration — `(array-type "[]%s")` on
	// Go, `"%s[]"` on Java — instead of enumerating an entry per element type.
	//
	// That enumeration is what tables.md §10 called "the suffix explosion", and
	// it is the surface this construct deletes: Go declared seven `slice-*`
	// types and nineteen `at-*`/`make-*`/`set-*`/`len` primitives, and the four
	// targets together declared 54. They existed because the type language had
	// no constructor.
	// `(map K V)` resolves through ONE declaration, for `array`'s reason: the
	// alternative is an entry per (K, V) pair, and Java's collections showed
	// what that costs — `targets/java/util.oro` says the suffix explosion is
	// "the same limitation squared" there, one name per (container, K, V).
	if k, v, ok := core.MapTypes(name); ok {
		if tg.MapType != "" {
			// BOXED type arguments. A JVM generic cannot be instantiated at a
			// primitive, so `map int int` is `Map<Long,Long>` and not
			// `Map<long,long>`. On a host that boxes nothing this is `ty`.
			return Fill(tg.MapType, tg.boxed(k), tg.boxed(v))
		}
		// A target with no types — JavaScript, windows — spells a map nothing
		// at all, which is why neither declares one.
		return ""
	}
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
		// THROUGH ValueType, because this seeds a SCALAR's type and a range is
		// not a width. `(sig sq ((n (int 0 1000))) int)` otherwise emits
		// `func GenSq(n uint16)`, and `n * n` at uint16 wraps at 65536: 1000*1000
		// returns 16960. A range says what the value IS; only a table's element
		// slot consults how wide it is stored.
		//
		// The bug was LATENT until a scalar range type-checked at all — the
		// checker refused every use of the parameter, so nothing reached this
		// line. A refusal was standing in front of a silent wrong answer.
		//
		// `(array (int 0 255))` is untouched: ValueType strips only a top-level
		// range, so an element width still reaches the emitter, which is what
		// elemwidth built and what must keep working.
		if ty := core.ValueType(sig.Params[i].Type); ty != "" && ty != "any" {
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
// `post` names the variables the `for` statement's own post clause updates
// (PostVars); assigning one here as well would advance it TWICE. That is not
// hypothetical — the first version of the post hoist patched each backend's
// `emitAgain` separately, JavaScript's routes through this helper instead, and
// the emitted sieve incremented `i` twice per iteration and got 1984 of 2000
// answers wrong. The skip belongs in the one place all three share.
func changedArgs(as []*core.Term, raw []string, post map[int]*core.Term) []int {
	var out []int
	for i, a := range as {
		if _, hoisted := post[i]; hoisted {
			continue
		}
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

// isTableRule reports whether a term is `(table n f)`.
func isTableRule(tgt *Target, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp || len(t.Kids) != 3 {
		return false
	}
	op := t.Kids[0]
	if op.Kind != core.KName || op.Name != "table" {
		return false
	}
	p, ok := tgt.Prims["table"]
	return ok && p.Kind == "table"
}

// bufferElem works out what a `build` buffer holds, by finding a store into it.
//
// There is no element type written anywhere: `(build n (fn (b) …))` says a
// length and a body, and what the buffer holds is whatever `set` puts there.
// Reading it off the first store is exact, because a table is homogeneous — a
// dynamic index forces that (tables.md §5).
// storedRange is the range of a value being stored, where it can be had
// EXACTLY. A literal is its own range, a read from an already-narrowed table
// carries one, and a conditional is the join of its branches — which is the
// shape a tag or a sentinel actually takes: `(if (= c 123) 125 93)`.
//
// Everything else answers no, and the buffer keeps the host's own width. This
// is deliberately not the interval analysis: a range that is too narrow
// truncates on store and is a silent wrong answer, so only facts that are exact
// by construction are used.
func storedRange(v *core.Term, typeOf func(*core.Term) string) (int64, int64, bool) {
	if v == nil {
		return 0, 0, false
	}
	if v.Kind == core.KInt {
		return v.Int, v.Int, true
	}
	// `if` is injected into every target and declaring one is an error
	// (ADR 0017), so there is exactly one spelling to match.
	if v.Kind == core.KApp && len(v.Kids) == 4 && v.Kids[0].Kind == core.KName &&
		v.Kids[0].Name == "if" {
		alo, ahi, aok := storedRange(v.Kids[2], typeOf)
		blo, bhi, bok := storedRange(v.Kids[3], typeOf)
		if !aok || !bok {
			return 0, 0, false
		}
		return min64(alo, blo), max64(ahi, bhi), true
	}
	return core.IntRange(typeOf(v))
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// ElemType is a `build` buffer's element type: the exact syntactic answer where
// there is one, and the interval analysis's where there is not.
//
// The order matters and is not arbitrary. A literal, a conditional over
// literals, and a read from an already-narrowed table are exact by
// construction. Only when none of those decides does the analysis get asked —
// so the cases that can be settled without trusting a fixpoint are, and
// BufferRange's soundness argument carries only the rest.
// MapElemTypes decides a map buffer's key and value types from its stores.
//
// The KEY is `int` and there is no inference to do: `(map K V)` is well-formed
// exactly where the language's `=` is defined, and `=` is integer equality only
// (maps.md §2). A range narrows the key's REPRESENTATION and not its type, so
// this returns `int` and lets `ty` spell it.
//
// The VALUE is read off the `(insert m k v)` calls, the way ElemType reads a
// buffer's element off its `set`s — and for the same soundness reason: a slot
// holds either nothing or the most recent insert, there being no third source,
// because `build-map` is the only allocator and `insert` the only store and
// ADR 0018's linearity means nothing else can have written it.
//
// Where the stores disagree or say nothing, the host's own integer. Widening is
// the safe direction: a value type too narrow truncates on store and is a
// silent wrong answer, which is exactly what elemwidth-2026-08-27 recorded.
func MapElemTypes(tgt *Target, lam, body *core.Term, name string,
	typeOf func(*core.Term) string, sig *core.Sig, params []string) (string, string) {

	val := ""
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil {
			return
		}
		if t.Kind == core.KApp {
			if op := t.Op(); op.Kind == core.KName && op.Name == "insert" {
				if a := t.Args(); len(a) == 3 && a[0].Kind == core.KName && a[0].Name == name {
					if ty := core.ValueType(typeOf(a[2])); ty != "" && ty != "any" {
						if val == "" {
							val = ty
						} else if val != ty {
							val = "int" // disagreement widens, it never narrows
						}
					}
				}
			}
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(body)
	if val == "" {
		val = "int"
	}
	return "int", val
}

func ElemType(tgt *Target, lam, body *core.Term, name string,
	typeOf func(*core.Term) string, sig *core.Sig, params []string) string {
	if ty := bufferElem(body, name, typeOf); ty != "int" {
		return ty
	}
	// The analysis is asked WITH the enclosing precondition, because what
	// bounds a buffer's stores is usually something the signature says.
	if r, ok := BufferRange(tgt, lam, sig, params); ok {
		return r
	}
	return "int"
}

// BufferRoot follows a threaded buffer back to the name it came from.
// `(set (set b i v) j w)` writes to `b`, and a program with two live buffers —
// examples/json/tree.oro is the first — needs them told apart.
func BufferRoot(t *core.Term) string {
	for t != nil {
		if t.Kind == core.KName {
			return t.Name
		}
		if t.Kind == core.KApp && len(t.Kids) == 4 && t.Kids[0].Kind == core.KName &&
			(t.Kids[0].Name == "set" || strings.HasSuffix(t.Kids[0].Name, ".set")) {
			t = t.Kids[1]
			continue
		}
		return ""
	}
	return ""
}

// bufferElem is a `build` buffer's element type, read off everything STORED
// into it.
//
// ADR 0003 says ranges are declared at boundaries and inferred for locals, and
// a buffer is a local — so unlike an array parameter, nothing declares its
// width and it has to be derived. The derivation is deliberately syntactic:
// a literal is its own exact range, and a value read out of an already-narrowed
// table carries one. Anything else is a plain integer and the buffer stays the
// host's own width.
//
// That is weaker than the interval analysis and is meant to be. A wrong range
// here is a SILENT WRONG ANSWER — a value stored into a byte slot and read back
// truncated — so the inference only draws on facts that are exact by
// construction rather than on a fixpoint. Widening it to the interval domain is
// a real extension and needs the soundness argument made separately.
//
// ZERO IS ALWAYS AN ELEMENT. `build` zero-fills (tables.md §14.3) and a leaf
// slot is never written, so the range has to hold 0 whatever the program
// stores.
func bufferElem(body *core.Term, name string, typeOf func(*core.Term) string) string {
	lo, hi := int64(0), int64(0)
	sawRange, sawOther, other := false, false, ""
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t == nil {
			return
		}
		if t.Kind == core.KApp && len(t.Kids) == 4 &&
			t.Kids[0].Kind == core.KName && t.Kids[0].Name == "set" &&
			// An UNIDENTIFIED root counts, which is what this did before there
			// was a root at all. One caller passes `Closed()`, where the buffer
			// is a KBound and no name is recoverable; merging two buffers'
			// stores there only ever WIDENS the range, and a range too wide
			// costs space while a range too narrow is a silent wrong answer.
			(BufferRoot(t) == "" || BufferRoot(t) == name) {
			if l, h, ok := storedRange(t.Kids[3], typeOf); ok {
				sawRange = true
				if l < lo {
					lo = l
				}
				if h > hi {
					hi = h
				}
			} else {
				sawOther = true
				if ty := typeOf(t.Kids[3]); other == "" && ty != "" {
					other = ty
				}
			}
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(body)
	switch {
	case sawRange && !sawOther:
		return fmt.Sprintf("int %d %d", lo, hi)
	case other != "":
		return other
	}
	// A buffer nobody writes to. Its elements are whatever the host zeroes
	// to; `int` is the one every target has.
	return "int"
}

// collectAgains gathers every `again` in a clause chain.
func collectAgains(t *core.Term) []*core.Term {
	var out []*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KApp && len(x.Kids) > 0 &&
			x.Kids[0].Kind == core.KName && x.Kids[0].Name == "again" {
			out = append(out, x)
			return // an `again` is a tail; nothing nests under it
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return out
}

// PostVars picks the loop variables whose update can move into the `for`
// statement's post clause, turning several back edges into one.
//
// This is what the sieve cost 1.4x against hand-written Go
// (tables-write-2026-08-25). Our loops emit
//
//	for { if guard { break }; …; i = i + 1; continue }
//
// where a person writes `for i := 2; i*i < n; i++`. The increment is duplicated
// into every clause, so the loop has several back edges and Go's SSA does not
// see a counted loop. Measured: hoisting the increment ALONE takes the sieve
// from 470k to 348k, at hand-written — and so does hoisting the condition
// alone, so Go needs only one of the two to recognise the shape.
//
// A variable qualifies when:
//
//  1. every `again` passes the SAME term for it — otherwise there is no single
//     update to hoist;
//  2. that term is not the variable itself, which `changedArgs` already skips;
//  3. the term reads no OTHER loop variable;
//  4. the term mentions nothing bound BETWEEN the loop header and the `again`.
//
// (3) is the soundness condition and it is easy to get wrong. `again`'s
// arguments are evaluated simultaneously, with every variable's OLD value. A
// post clause runs after the body, so if the hoisted update read another
// variable that the body had already assigned, it would see the new value.
// `i = i + 1` reads only itself and is safe; `i = i + j` alongside a changing
// `j` is not, and stays in the body.
//
// (4) is the scope condition, and it exists because ADR 0015 permits `again`
// under a `let`. The post clause is written on the `for` statement, OUTSIDE
// every binder the body opened, so an update like `(if (go.> dp mx) dp mx)`
// whose `dp` came from an enclosing `let` cannot go there. `collectAgains`
// walks the CLOSED body, so such a name is a `KBound` and this is exactly the
// test for it. Nothing had hit it because no program before had a non-trivial
// update under a `let` — a JSON tree walk did (json-tree-2026-08-26).
func PostVars(body *core.Term, raw []string) map[int]*core.Term {
	agains := collectAgains(body)
	if len(agains) == 0 {
		return nil
	}
	out := map[int]*core.Term{}
	for i := range raw {
		var want *core.Term
		ok := true
		for _, a := range agains {
			as := a.Args()
			if i >= len(as) {
				ok = false
				break
			}
			if want == nil {
				want = as[i]
				continue
			}
			if want.String() != as[i].String() {
				ok = false
				break
			}
		}
		if !ok || want == nil {
			continue
		}
		if want.Kind == core.KName && want.Name == raw[i] {
			continue // unchanged; nothing to hoist
		}
		if readsOtherLoopVar(want, raw, i) || mentionsInnerBinder(want) {
			continue
		}
		out[i] = want
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mentionsInnerBinder reports whether a term refers to a binder opened between
// the loop header and this `again` — a `let`'s name, in practice. See
// PostVars (4). In a closed body such a reference is a `KBound`.
func mentionsInnerBinder(t *core.Term) bool {
	found := false
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil || found {
			return
		}
		if x.Kind == core.KBound {
			found = true
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return found
}

// readsOtherLoopVar reports whether a term mentions a loop variable that is not
// the one being updated. See PostVars (3).
func readsOtherLoopVar(t *core.Term, raw []string, self int) bool {
	found := false
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil || found {
			return
		}
		if x.Kind == core.KName {
			for j, n := range raw {
				if j != self && x.Name == n {
					found = true
					return
				}
			}
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return found
}

// NarrowIndex reports which loop variables are provably small enough to hold in
// a HOST INT rather than in the language's `int`.
//
// This exists because our `int` is 64-bit and Java's array index is not. An
// emitted counter is a `long`, so every access carries an `(int)` cast, and
// that measured **1.04x to 1.45x** against hand-written Java depending on how
// much else the loop does (native-java-2026-08-25). It is the only place the
// project currently misses its own bar with a number attached.
//
// The rule is deliberately narrow, and each clause is the reason the next one
// is safe:
//
//  1. a clause guard is `(>= v B)` or `(> v B)` with B not mentioning any loop
//     variable — so B is the loop's upper bound;
//  2. B is a LENGTH, which on a host whose arrays are int-indexed is at most
//     2³¹−1 by the platform's own rule;
//  3. every `again` steps v by exactly +1;
//  4. v starts at a non-negative integer literal.
//
// Together those give `v ∈ [init, B]`: the guard exits when `v >= B`, so with a
// step of one v reaches B and stops. That is exactly the range Java's own
// `for (int i = 0; i < a.length; i++)` occupies, and the reason the step must
// be one is that a larger step could overshoot B — which only matters at
// B near 2³¹, and is refused rather than reasoned about.
//
// It is a REPRESENTATION selection in the sense selection-2026-08-19
// established: what is emitted changes, what the program means does not.
// NarrowByInterval reports the loop variables this target may hold in its own
// index type, decided by the INTERVAL ANALYSIS rather than by a syntactic
// pattern.
//
// indextype-2026-08-25 narrows a counter "bounded by a length and stepping by
// +1", and named the sieve as a program it cannot help: its bound is `i*i >= n`
// and its step is `+i`. The analysis bounds that sieve at 1..20164, so the
// general rule reaches what the pattern cannot.
//
// SOUNDNESS. Holding a value in 32 bits computes the same answer as 64 exactly
// when every intermediate stays inside 32 bits, so two things must hold:
//
//  1. `MaxOp` — the join of every checkable operation in the loop — fits.
//  2. Every value a narrowed variable can TAKE fits, and MaxOp does not cover
//     all of those. A literal is not an operation, and neither is a read out of
//     a table, whose element range this pass does not know. So each variable's
//     sources are checked directly, and anything not recognised refuses.
//
// Refusing is always safe: the variable keeps the host's widest integer, which
// is what every program emitted before this existed.
func NarrowByInterval(tgt *Target, fitsIdx bool, body *core.Term,
	raw []string, inits []*core.Term) map[string]bool {
	// `fitsIdx` is the WHOLE FUNCTION's answer, computed once by the emitter
	// with the signature in hand. Running the analysis on the loop alone does
	// not work here and the reason is worth keeping: a loop's bound usually
	// comes from the enclosing `where`, so a subterm loses exactly the fact it
	// needs. That is the same conservatism that makes BufferRange safe, biting
	// in the other direction.
	//
	// Whole-function is coarse — one unbounded operation anywhere refuses every
	// loop in it — and it is the safe coarseness.
	if !fitsIdx || len(raw) != len(inits) {
		return nil
	}
	out := map[string]bool{}
	for i, n := range raw {
		out[n] = fitsIndexSource(tgt, inits[i], raw)
	}
	for _, a := range collectAgains(body) {
		as := a.Args()
		for i, n := range raw {
			if i >= len(as) || !fitsIndexSource(tgt, as[i], raw) {
				out[n] = false
			}
		}
	}
	for n, v := range out {
		if !v {
			delete(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// fitsIndexSource reports whether a value a loop variable takes is one MaxOp has
// already bounded, or is otherwise known to fit.
//
// A literal is checked against the range directly. Another loop variable is
// fine inductively. An operation the target declares is covered by MaxOp — but
// a CONDITIONAL is not, because it is a structural primitive rather than an
// arithmetic one, so its branches are checked instead. Everything else, table
// reads included, refuses.
func fitsIndexSource(tgt *Target, t *core.Term, raw []string) bool {
	switch {
	case t == nil:
		return false
	case t.Kind == core.KInt:
		return t.Int >= -2147483648 && t.Int <= 2147483647
	case t.Kind == core.KName:
		return contains(raw, t.Name)
	case t.Kind == core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return false
		}
		p, known := tgt.Prims[op.Name]
		if !known {
			return false // a table read, whose element range this pass has not got
		}
		// AN INLINED LOOP fits when every value it can produce fits, which is
		// every one of its EXITS. Without this an inner loop narrows while the
		// outer one does not, and the inner's initial value — computed from the
		// outer's variable — is a `long` assigned to an `int`. javac catches it;
		// nothing else would (monotone-2026-08-27 §7).
		if p.Kind == "iterate" {
			return loopExitsFit(tgt, t, raw)
		}
		if p.Kind == "cond" && len(t.Args()) == 3 {
			return fitsIndexSource(tgt, t.Args()[1], raw) &&
				fitsIndexSource(tgt, t.Args()[2], raw)
		}
		// Only what MaxOp actually counted. Division is bounded by the analysis
		// and NOT joined into MaxOp, so trusting it here would trust a number
		// that was never checked.
		return CountedOp(tgt, t)
	}
	return false
}

func NarrowIndex(tgt *Target, fitsIdx bool, body *core.Term,
	raw []string, inits []*core.Term) map[string]bool {
	// The analysis first: it subsumes the pattern wherever it can bound the
	// loop, and reaches programs the pattern was written to exclude.
	if nw := NarrowByInterval(tgt, fitsIdx, body, raw, inits); nw != nil {
		return nw
	}
	idx, bound, ok := countedBy(tgt, body, raw)
	if !ok || !isIntBound(tgt, bound) {
		return nil
	}
	pos := -1
	for i, n := range raw {
		if n == idx {
			pos = i
		}
	}
	if pos < 0 || inits[pos].Kind != core.KInt || inits[pos].Int < 0 {
		return nil
	}
	// Every `again` must step it by exactly one.
	agains := collectAgains(body)
	if len(agains) == 0 {
		return nil
	}
	for _, a := range agains {
		as := a.Args()
		if pos >= len(as) || !isPlusOne(tgt, as[pos], idx) {
			return nil
		}
	}
	return map[string]bool{idx: true}
}

// countedBy is countedGuard without the Emitter, so every backend can ask.
func countedBy(tgt *Target, t *core.Term, raw []string) (string, *core.Term, bool) {
	for t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName {
		p, ok := tgt.Prims[t.Op().Name]
		if !ok || p.Kind != "cond" || len(t.Args()) != 3 {
			return "", nil, false
		}
		c := t.Args()[0]
		if c.Kind == core.KApp && c.Op().Kind == core.KName && len(c.Args()) == 2 {
			if n := c.Op().Name; isOp(n, "ge") || isOp(n, "gt") {
				lhs, rhs := c.Args()[0], c.Args()[1]
				if lhs.Kind == core.KName && contains(raw, lhs.Name) && !mentions(rhs, raw) {
					return lhs.Name, rhs, true
				}
			}
		}
		t = t.Args()[2]
	}
	return "", nil, false
}

// isIntBound reports whether a bound is small enough for a host int.
//
// A LENGTH is, by the platform's own rule. So is a length MINUS a non-negative
// literal, which is what a stencil's bound looks like — `(- (len a) 2)` — and
// which cannot grow past the length it came from.
//
// Adding to a length is NOT accepted, because `len + k` at a length near 2³¹
// is exactly the overflow this rule exists to avoid.
func isIntBound(tgt *Target, t *core.Term) bool {
	if isLength(tgt, t) {
		return true
	}
	if t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName && len(t.Args()) == 2 {
		if isOp(t.Op().Name, "sub") {
			a, b := t.Args()[0], t.Args()[1]
			return isIntBound(tgt, a) && b.Kind == core.KInt && b.Int >= 0
		}
	}
	return false
}

// isLength reports whether a term is a length — the language's `len` or any
// spelling a target declares for one. A length is what carries the platform's
// own guarantee that it fits in a host int.
func isLength(tgt *Target, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 1 {
		return false
	}
	n := t.Op().Name
	if n == "len" {
		return true
	}
	if p, ok := tgt.Prims[n]; ok && p.Kind == "len" {
		return true
	}
	return isOp(n, "alen") || isOp(n, "slen")
}

// isPlusOne reports whether a term is `(+ v 1)` for this variable.
func isPlusOne(tgt *Target, t *core.Term, v string) bool {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return false
	}
	if !isOp(t.Op().Name, "add") {
		return false
	}
	a, b := t.Args()[0], t.Args()[1]
	return a.Kind == core.KName && a.Name == v && b.Kind == core.KInt && b.Int == 1
}

// allocSpellings are how a target may spell "give me n bytes", most preferred
// first. Found the way findEq finds equality — by the LAST SEGMENT of a
// declared name — so a target that already declares an allocator declares
// nothing new to get tables.
//
// This is the shape of the answer ADR 0002 gives generally: `alloc` and `build`
// are the LANGUAGE's, and where the memory comes from on this host is the
// TARGET's. Go, JavaScript and Java have allocation as syntax and their
// backends emit it directly; x86 has a call, and the target says which.
var allocSpellings = []string{"VirtualAlloc", "malloc", "HeapAlloc"}

// findAlloc returns the target's own allocator.
func (tg *Target) findAlloc() (Prim, bool) {
	for _, want := range allocSpellings {
		for name, p := range tg.Prims {
			seg := name
			if i := strings.LastIndex(seg, "."); i >= 0 {
				seg = seg[i+1:]
			}
			if seg == want && len(p.Args) == 1 && p.Kind == "expr" {
				return p, true
			}
		}
	}
	return Prim{}, false
}

// ElemBytes is how wide one element of a table is on a target that has to
// choose — which is x86, the only host with no types of its own.
//
// wintables-2026-08-25 measured the cost of NOT choosing: a boolean sieve with
// eight bytes per element runs **3x** slower than the same program using one,
// because the marking loop moves eight times the memory. Go never showed it
// because Go has a `bool` and `[]bool` is one byte; three hosts were sizing our
// elements for us through their own type systems.
//
// One byte for a bool, eight for everything else. `int` and `f64` are both
// 64-bit here, so there is no third case to get wrong.
func ElemBytes(tgt *Target, t *core.Term) int {
	if IsBoolTerm(tgt, t) {
		return 1
	}
	// A RANGE narrows the storage on a host with no type system too — this is
	// the same question `(array (int 0 255))` asks Go, asked in the one unit
	// x86 has, which is bytes. The target still decides: a width is used only
	// if the target DECLARED a representation covering it, so a target that
	// declares none keeps its machine word.
	if lo, hi, ok := storedRange(t, func(*core.Term) string { return "" }); ok {
		if n := tgt.reprBytes(lo, hi); n != 0 {
			return n
		}
	}
	return 8
}

// reprBytes is how many bytes the declared representation for [lo,hi] occupies,
// or 0 if this target declared none that holds it.
//
// The width comes from the DECLARED range rather than from the spelling,
// because the spelling is the host's word and the width is arithmetic. A target
// that says it can hold -128..127 has said one byte, whatever it calls it.
func (tg *Target) reprBytes(lo, hi int64) int {
	for _, r := range tg.Reprs {
		if r.Lo <= lo && hi <= r.Hi {
			switch {
			case r.Lo >= -128 && r.Hi <= 255:
				return 1
			case r.Lo >= -32768 && r.Hi <= 65535:
				return 2
			case r.Lo >= -2147483648 && r.Hi <= 4294967295:
				return 4
			}
			return 8
		}
	}
	return 0
}

// IsBoolTerm reports whether a term's value is a boolean — a literal, an `if`
// whose branches are, or an application of a primitive declared to return one.
func IsBoolTerm(tgt *Target, t *core.Term) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case core.KBool:
		return true
	case core.KApp:
		if t.Op().Kind != core.KName {
			return false
		}
		p, ok := tgt.Prims[t.Op().Name]
		if !ok {
			return false
		}
		if p.Kind == "cond" && len(t.Args()) == 3 {
			return IsBoolTerm(tgt, t.Args()[1]) || IsBoolTerm(tgt, t.Args()[2])
		}
		return p.Result == "bool"
	}
	return false
}

// BufferElemBytes reads a buffer's element width off the value a `set` stores
// into it — the same trick bufferElem uses for Go and Java, without needing a
// type environment, because a table is homogeneous.
// BufferElemBytes is how wide one element of a `build` buffer is, on a host
// with no type system to read it off.
//
// IT MUST CONSIDER EVERY STORE, not the first one. It took the first, and that
// is a silent wrong answer waiting: examples/json/tree.oro's node table stores a
// tag of 1..5 into slot 0 and a node index of up to 511 into slots 2 and 3, so
// the first store said one byte and the rest were truncated. windows returned
// 4030140 where the other three returned 4040171, and the differential suite is
// what noticed (elemwidth-2026-08-27 §4).
//
// So it defers to bufferElem, which joins. The `typeOf` it passes recognises
// booleans and nothing else, because that is all this host can know.
func BufferElemBytes(tgt *Target, lam, body *core.Term, name string,
	sig *core.Sig, params []string) int {
	ty := ElemType(tgt, lam, body, name, func(t *core.Term) string {
		if IsBoolTerm(tgt, t) {
			return "bool"
		}
		return ""
	}, sig, params)
	if ty == "bool" {
		return 1
	}
	if lo, hi, ok := core.IntRange(ty); ok {
		if n := tgt.reprBytes(lo, hi); n != 0 {
			return n
		}
	}
	return 8
}

// LoopInvariant reports which loop variables never actually change.
//
// A buffer threaded through a loop looks like it changes — `(again (set c j v) …)`
// — but `set` CONSUMES ITS ARGUMENT AND RETURNS IT, so the value handed back is
// the one that went in. ADR 0018's linearity is what makes that true rather
// than merely usual: nothing else can be holding the buffer, so nothing else
// can have replaced it.
//
// It matters on a host with a fixed register file. The sieve threads its buffer
// through both loops, and giving it a place of its own cost a register plus a
// copy in and out — which pushed the inner loop's INDEX to a spill slot, where
// it was reloaded three times per iteration:
//
//	mov r10, qword ptr [rsp+48]
//	cmp r10, 200000
//	…
//	mov r10, qword ptr [rsp+48]
//	mov byte ptr [r15+r10+8], 1
//
// Five memory accesses per element for a loop that needs none.
func LoopInvariant(tgt *Target, body *core.Term, raw []string) map[int]bool {
	agains := collectAgains(body)
	if len(agains) == 0 {
		return nil
	}
	out := map[int]bool{}
	for i, n := range raw {
		ok := true
		for _, a := range agains {
			as := a.Args()
			if i >= len(as) || !handsBack(tgt, as[i], n) {
				ok = false
				break
			}
		}
		if ok {
			out[i] = true
		}
	}
	return out
}

// handsBack reports whether a term evaluates to the variable it was given —
// the variable itself, or any chain of stores into it.
func handsBack(tgt *Target, t *core.Term, name string) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName {
		return t.Name == name
	}
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if p, ok := tgt.Prims[t.Kids[0].Name]; ok && p.Kind == "table-set" && len(t.Args()) == 3 {
			return handsBack(tgt, t.Args()[0], name)
		}
	}
	return false
}

// KindOf is a primitive's structural kind, for tests and tools.
func (tg *Target) KindOf(name string) string { return tg.Prims[name].Kind }
