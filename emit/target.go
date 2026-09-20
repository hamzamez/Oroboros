package emit

import (
	"fmt"
	"io/fs"
	"math"
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
	Name string
	Args []string // our type names; empty when the target is untyped

	// Results is populated ONLY when the primitive gives back more than one —
	// `(prim Open ((p string)) (ptr error) …)`. One result stays in `Result`,
	// exactly as `core.Sig` does it, so every existing path is untouched.
	// See parsePrim for why this field is worth 19.8% of an ecosystem.
	Results []string
	Result  string
	Kind    string // expr | stmt | loop | loop2 | cond | let
	Form    string // template with %s holes; empty for structural kinds
	Import  string
	// Lib is the import library that RESOLVES Import on a host that links —
	// `(lib "user32")`. Collected exactly as Import is, from primitives the
	// program uses, so the link line is computed rather than a constant that
	// silently reached 666 of 4,093 callable Win32 names (target-files.md §6a).
	Lib   string
	Pure  bool // declared `pure`; DEFAULTS TO FALSE, deliberately — see below
	Index bool // declared `index`: argument 0 is a container indexed by argument 1

	// A CONTAINER'S LENGTH IS A POSTCONDITION, `(ensures (= (len result) n))` for
	// a count and `(ensures (= (len result) (len c)))` for a pass-through
	// (theories.md §7.9, §8.4). It was two positional attributes, `(length N)` and
	// `(length-of N)`, beside the one clause that already says what a call
	// guarantees. refine.go's `lengthContract` reads it back; the reasons it is
	// declared rather than read off an argument's type — `targets/js/` types every
	// argument `any` — are unchanged.

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

	// Implements is the SUBSUMPTION relation this host declares: which concrete
	// types it accepts where an interface is wanted. Keyed by the concrete type,
	// the value being every interface it satisfies, closed transitively at load.
	//
	// AN INTERFACE IS AN EXISTENTIAL TYPE and PACKING one is manufacturing a
	// closure, which callbacks.md tier 3 refuses. But `io.Copy(dst, f)` does not
	// ask us to pack one -- it asks us to hand over a `*os.File`, and the HOST
	// inserts the coercion. So what is missing is not a value, a runtime or a
	// representation: it is a fact the type CHECKER needs and the backend does not.
	// The denotation of the coercion is the IDENTITY and it costs zero emitted
	// characters (docs/interfaces.md).
	//
	// DECLARED rather than derived. Derived, the relation is `methods(I) subset of
	// methods(T)` -- Cardelli's record subtyping over method sets -- and a method
	// signature may mention an interface, so it is recursive and wants coinduction
	// (Amadio & Cardelli 1993); it is also fragile against us, since our `error` is
	// opaque and our `int` is a range, so two signatures Go calls equal need not
	// survive `spell`. Declared, it is a preorder on GROUND names decided by
	// lookup. Pierce's undecidable F<: is about BOUNDED QUANTIFICATION and after
	// staging nothing is quantified, so none of that arises.
	//
	// And unusually the claim is checkable BY THE HOST: `var _ io.Reader =
	// (*os.File)(nil)` is one line per edge and `go build` decides.
	Implements map[string][]string

	// Includes is theory inclusion between COMPANIONS (theories.md §6.2), by
	// companion module path: `go/io/WriteCloser` includes `go/io/Writer` and
	// `go/io/Closer`, which is how an interface defined as `interface { Writer;
	// Closer }` says so. Resolved once on the merged target, because an included
	// companion may live in another file or layer (emit/companion.go).
	Includes map[string][]string

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

	// MapRepr is how this model realizes `map`: "library" when this host ships
	// no map of its own, so the language supplies one — `emit/winmap.oro`,
	// lowered into buffers and loops before reduction — and "host" or "" for the
	// host's own. `(repr map library)`, theories.md §5.8.
	//
	// Declared rather than inferred, because there is nothing to infer it from:
	// an empty `MapType` means "no map" on windows and "no TYPES" on
	// JavaScript, which has a perfectly good map and spells nothing. A host
	// fact belongs in a target file whichever way it points.
	//
	// A WORD AND NOT A FLAG, because it composes by override. It was a bool
	// joined with `||`, so a nearer layer could turn the library map on and never
	// off; "" is "not declared here", which is what lets `▷` see a choice.
	MapRepr string

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

	// BigRepr is how this target stores a value whose declared range is above
	// the portable window but FINITE — "limbs" or "host". Empty means the
	// obvious default: the host's own bignum where it has one, fixed limbs
	// where it does not.
	//
	// IT IS A REPRESENTATION AND NOT A MEANING, which is the whole reason it is
	// declared here rather than read off the range. `(int 0 (pow 2 1300))` says
	// the value is a mathematical integer in that interval — a fact about the
	// program, true on every target — and ADR 0003 has said since the beginning
	// that mathematical semantics and machine representation are two different
	// things. `int-repr` already works this way one rung down: the programmer
	// writes `(int 0 255)` and Go picks `[]byte`, the JVM picks `short[]`
	// (its byte is signed) and JavaScript picks nothing at all.
	//
	// The reason this exists is that the same rule was NOT followed at the top
	// of the ladder. A finite range selected limbs and `+inf` selected the
	// host's bignum, so the SHAPE of a declaration chose a representation —
	// which cost 100x on V8, where BigInt is C++ with 64-bit limbs and ours is
	// portable Oroboros with 24-bit ones (biglimb-2026-09-02).
	//
	// There is no total order to select from the way `int-repr` has one, so
	// this cannot be derived: bigarith-2026-08-28 measured ours winning where
	// the operation is LINEAR and the host winning where it is QUADRATIC. A
	// target declares what somebody measured, and when the limb library gets
	// faster the thing that changes is a target file and no program moves.
	BigRepr string

	// ShiftWidth is the largest N for which this host's `>>` and `&` are exact
	// on every value in [0, 2^N). Zero means the target declares nothing and
	// the compiler will not rewrite a division into a shift there.
	//
	// It exists because `x / 2^k` on a SIGNED value is not a shift: truncation
	// toward zero needs a rounding correction, and Go, the JVM and x86 all emit
	// one. Measured on our own fixed-limb factorial, replacing `/` and `%` by a
	// constant power of two with a shift and a mask is worth **2.39x** — the
	// dominant term in that program by a distance, larger than the clamp, the
	// element mask and the buffer clear put together, which are together inside
	// the noise floor (limbcost, gauntlet/results/shiftdiv-2026-09-03.md).
	//
	// The rewrite is licensed by a PROOF — the interval analysis showing the
	// dividend non-negative and inside 2^N — rather than by a declaration, so
	// it needs no new operator in the language and it applies to any program,
	// not only a bignum. What the target declares is the host fact the proof is
	// checked against, and the four disagree: Go, the JVM and x86 shift 64-bit
	// values, and **V8 coerces both operands of `>>` and `&` to int32**, which
	// is the same divergence integers.md §0a keeps bitwise operators out of the
	// language for. So JavaScript declares 31 and gets the rewrite on values
	// that provably fit, rather than being excluded.
	ShiftWidth int64

	Reprs []IntRepr
	Prims map[string]Prim
	Names []string // every primitive name, for core.Env

	// Narrow is a template `dst = src[:n]` that restricts a container to a
	// known length. A target that declares one gets bounds-check elimination
	// in loops (bce-2026-08-15.md); one that does not simply gets none, which
	// is right for JS (no bounds checks) and Java (fixed-length arrays).
	Narrow string

	// Backend names the CODE GENERATOR that compiles this target — the `B` of
	// target-system.md's `T = (B, Δ)`, made explicit.
	//
	// It is a finite closed set, because a backend is compiler code: it emits
	// control flow and binds variables, which no template can do. `Δ` is data
	// and anyone may write it; `B` is not.
	//
	// Empty means the target did not say, and `ResolveBackend` then falls back
	// to the target's own NAME when that names a backend — so `(target go …)`
	// need not also write `(backend go)`. When neither resolves, that is an
	// ERROR rather than a default, which is the whole point of this field:
	// `cmd/build` used to switch on the -target FLAG and fall through to the Go
	// backend for any name it did not recognise, so a target directory called
	// anything of its own was compiled by the wrong backend, silently, and the
	// first sign of it was MASM refusing a file full of Go control flow
	// (win32-2026-09-06 §6).
	Backend string

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

	// Link is the libraries EVERY program is linked against on a host that
	// resolves imports at link time — what the backend's own code needs and
	// what the target's hand-written runtime assumes. A primitive's `lib` adds
	// to it only when the program uses that primitive (target-files.md §6a).
	// It was a constant in asmBuildBat, which is a host fact living in Go.
	Link []string

	// Build is the host toolchain command: %s is the artifact path, %s the
	// directory holding the emitted source. A target that declares none can
	// still emit source, which is what cmd/gen does (build.md §4).
	Build string

	// Defs are `D_T` — the definitions this target contributes, in Oroboros,
	// keyed by module path (target-system.md §6.2, theories.md §5.4). A target
	// has always been able to say how a host SPELLS a name; this lets it say
	// that the host DEFINES one. Handed to core.LoadWithDefs, where δ unfolds
	// them exactly as it unfolds a library's.
	//
	// A TARGET LIBRARY MAY NOT DECLARE — `def` and `use`, never `sig`, `type` or
	// `structural` — and that is a stratification rather than a taste: `P_T` is
	// reduction's parameter, so it may not depend on reduction.
	Defs map[string][]core.Form

	// Aliases are the MANIFEST types — `(type NAME τ)`, a type with a definiens
	// and no realization (theories.md §2 at the type level). Unfolded on the
	// glued target, after which no alias name occurs anywhere: emit/alias.go.
	Aliases map[string]string

	// Deferred are the declarations whose types name a CONSTANT as a range
	// endpoint, carried unparsed through the glue and elaborated once the target
	// is whole — emit/constend.go. Empty after a successful load.
	Deferred []deferred
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
	tg, err := loadFragment(path)
	if err != nil {
		return nil, err
	}
	if err := tg.resolveDeferred(); err != nil {
		return nil, err
	}
	if err := tg.unfoldAliases(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	tg.addCore()
	if err := tg.expandCompanions(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	tg.closeImplements()
	if err := tg.checkViews(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return tg, nil
}

// loadFragment is ONE LAYER: a directory whose `.oro` files are GLUED, or a
// single file. It does not inject the core, because `addCore` resolves the
// language's names by spelling and must therefore see the WHOLE target — run it
// per layer and a name would be resolved against a fragment.
func loadFragment(path string) (*Target, error) {
	// ONE RULE, both spellings: a layer's contribution for target T is the
	// DIRECTORY `L/T` or the FILE `L/T.oro`, and the caller may write either
	// with or without the extension. `targets/go` is a directory and
	// `targets/blas.oro` is a file, and nothing above here should have to know
	// which.
	base := strings.TrimSuffix(path, ".oro")
	for _, d := range []string{path, base} {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return loadTargetDir(d)
		}
	}
	for _, f := range []string{path, base + ".oro"} {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			return loadTargetFile(f)
		}
	}
	return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
}

// loadProvides is the LIBRARY layer — target-system.md §8, and modules.md §6's
// `(provides …)`, specified since August and never parsed.
//
// ALGEBRAICALLY IT IS NOT A NEW THING. `(provides T M decl…)` is exactly
// `(target T (module M decl…))` written in a library file, so it needs no new
// operation, no new precedence and no new cell in the four-cell table: it is a
// target FRAGMENT, and §7.2 already says a target is the glue of its fragments.
// The only thing that changes is WHERE fragments are found.
//
// That is what makes a portable library with native fast paths a one-file job:
// `mylib/mylib.oro` holds the portable definitions, `mylib/go.oro` holds
// `(provides go …)`, and the library author never edits `targets/`.
//
// It is the LOWEST layer. A `provides` is a library's opinion about a host; the
// target's own files are the authority on that host, so if both name the same
// thing the target wins — which is `▷` with the library on the right.
func loadProvides(name string, libDirs []string) (*Target, []string, error) {
	out := &Target{Name: name, Types: map[string]string{}, Prims: map[string]Prim{}}
	var from []string
	for _, d := range libDirs {
		if d == "" {
			continue
		}
		err := filepath.Walk(d, func(fp string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(fp, ".oro") {
				return nil
			}
			src, err := os.ReadFile(fp)
			if err != nil {
				return nil
			}
			// Cheap reject before parsing: most library files have none.
			if !strings.Contains(string(src), "(provides") {
				return nil
			}
			terms, err := core.ReadAll(string(src))
			if err != nil {
				return nil // a library that does not parse is the loader's problem, not ours
			}
			for _, t := range terms {
				if t.Kind != core.KApp || len(t.Kids) < 3 || t.Kids[0].Kind != core.KName ||
					t.Kids[0].Name != "provides" {
					continue
				}
				if t.Kids[1].Kind != core.KName || t.Kids[1].Name != name ||
					t.Kids[2].Kind != core.KName {
					continue
				}
				mod := t.Kids[2].Name
				// A DECLARATION THAT DOES NOT LOAD IS AN ERROR HERE TOO. Both used to
				// vanish: anything but a `prim` was skipped and a declaration that
				// failed ended the file's walk in silence, so a library's native fast
				// path could disappear with no diagnostic (theories.md §10.1, D1).
				for _, inner := range t.Kids[3:] {
					if err := out.declare(inner, mod, fp); err != nil {
						return err
					}
				}
				from = append(from, fp)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	if len(out.Prims) == 0 && len(out.Defs) == 0 {
		return nil, nil, nil
	}
	return out, from, nil
}

// LoadTargetLayers builds `Δ_T = L₁ ▷ L₂ ▷ … ▷ Lₖ` — target-system.md §7.2.
//
// A target used to be ONE directory: `filepath.Join(dir, name)`, while modules
// got a genuine search path that already included the source's own directory.
// So a library could live beside the program and a target could not, and
// pointing `-targets` elsewhere REPLACED the built-ins rather than adding to
// them. That was not a decision anywhere; it was one call to `filepath.Join`
// that was never generalised.
//
// The rule is the two operations used exactly once each: **glue within a layer,
// override between layers**. Within a directory the files must agree and their
// order cannot matter; between directories the nearer one wins and is allowed to
// disagree, which is what lets a project replace a built-in binding deliberately
// and say so.
//
// `dirs` is nearest first. A layer that does not have this target contributes
// nothing — an absent layer is the empty fragment, which is `⊔`'s identity, so
// there is no special case for it.
func LoadTargetLayers(name string, dirs []string, libDirs ...[]string) (*Target, error) {
	var out *Target
	var found []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		p := filepath.Join(d, name)
		frag, err := loadFragment(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // this layer does not have it
			}
			return nil, err
		}
		found = append(found, p)
		if out == nil {
			out = frag
			continue
		}
		if frag.Name != out.Name {
			return nil, fmt.Errorf("%s declares target %q, but %s declares %q — the layers of "+
				"one target must agree on its name", p, frag.Name, found[0], out.Name)
		}
		if err := out.combine(frag, p, override); err != nil {
			return nil, err
		}
	}
	// THE LIBRARY LAYER, LAST. See loadProvides: a `(provides T M …)` block is a
	// target fragment that happens to live in a library file, so it joins the
	// same chain — at the bottom, because the target's own files are the
	// authority on the target.
	for _, lds := range libDirs {
		lib, _, err := loadProvides(name, lds)
		if err != nil {
			return nil, err
		}
		if lib == nil {
			continue
		}
		if out == nil {
			out = lib
			continue
		}
		if err := out.combine(lib, "a library's (provides "+name+" …)", override); err != nil {
			return nil, err
		}
	}
	if out == nil {
		return nil, fmt.Errorf("no target %q on the search path: looked in %s",
			name, strings.Join(dirs, string(filepath.ListSeparator)))
	}
	sort.Strings(out.Names)
	if err := out.resolveDeferred(); err != nil {
		return nil, err
	}
	if err := out.unfoldAliases(); err != nil {
		return nil, err
	}
	out.addCore()
	if err := out.expandCompanions(); err != nil {
		return nil, err
	}
	out.closeImplements()
	if err := out.checkViews(); err != nil {
		return nil, err
	}
	return out, nil
}

// coreNames are the names the LANGUAGE owns. A target may not declare any of
// them; three are reader sugar that never reaches a target at all, and `if` is
// injected into every target by addCore.
var coreNames = map[string]bool{
	"if": true, "and": true, "or": true, "not": true, "cond": true,
	// A range on a TERM. A target may not declare it for the same reason it may
	// not declare `if`: it is the language's, and the compiler finds what it
	// means on each host — which here is nothing, since it is erased.
	core.AscribeName: true,
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
	// CONCATENATION, which is the free monoid's operation and therefore the
	// only string operation that is not derived from something else
	// (string-operations.md §1). `empty` needs no name at all: it is `""`, the
	// identity, and a literal already denotes it.
	//
	// A target may not declare it under this name for the reason it may not
	// declare `if`: the language owns it, and the compiler finds each host's own
	// spelling — which on three of the four is `+` on their string type.
	"concat": true,
	// AND THE GENERATOR INJECTION, η : Scalar → Scalar*, which the free monoid's
	// universal property is stated in terms of — `f*(⟨c⟩) = f(c)` mentions the
	// one-scalar string, so it is part of the structure rather than an extra
	// (string-operations.md §1).
	//
	// With `concat` and `""` it is enough to BUILD any string, which is what a
	// renderer does. Going the other way — a whole table of scalars at once — is
	// `alloc`'s inverse and is not built, because nothing has needed it.
	"string-of": true,
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
	"map": true, "build-map": true, "insert": true, "keys": true,
}

// coreStructural is what addCore injects: the language's own constructs, with
// the structural KIND each backend already implements.
var coreStructural = []Prim{
	{Name: "if", Kind: "cond", Pure: true},
	{Name: "let", Kind: "let", Pure: true},
	{Name: "loop", Kind: "iterate", Pure: true},
	// A RANGE ON A TERM (core.AscribeName). `(the "int 0 …" e)` says what set
	// the value of `e` is in, and it exists because a declaration on a signature
	// is a boundary that whole-program reduction removes: a range the analysis
	// cannot re-derive has nowhere else to live.
	//
	// It is ERASED at emission — it carries a range and nothing else, and the
	// runtime enforcement of a declared bound is `big-fit` and the limb rung's
	// `guard`, both of which already exist. Pure, because it is the identity on
	// every value it accepts.
	{Name: core.AscribeName, Kind: "ascribe", Args: []string{"string", "any"}, Pure: true},
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
	// THEIR LENGTHS ARE POSTCONDITIONS, stated the way a target states one
	// (theories.md §8.4). `build` makes a buffer as long as its count; `alloc` and
	// `set` pass their argument's length through. Without them a program cannot
	// prove its own index, because nothing relates the buffer to the number it
	// was made from.
	{Name: "alloc", Kind: "table-alloc", Names: []string{"t"}, Ensures: lenEquals("t", false)},
	{Name: "build", Kind: "table-build", Names: []string{"n", "f"}, Ensures: lenEquals("n", true)},
	{Name: "set", Kind: "table-set", Names: []string{"c", "i", "x"}, Ensures: lenEquals("c", false)},
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
	// `keys : Map K V → Array K`, IN ASCENDING KEY ORDER, and both halves are
	// derived rather than chosen (maps.md §7).
	//
	// The result is an `Array`, whose index set is `Fin n` and therefore
	// ORDERED, so producing one from an unordered index set requires supplying
	// an order — and the only order available canonically is the one on K
	// itself. A host's iteration order is not canonical: Go randomises on
	// purpose, JavaScript specifies its own, Java leaves it unspecified. So
	// sorting by ≤_K is what makes `keys` the same on four hosts, precisely
	// because it ignores all four.
	//
	// Insertion order is the other candidate and the algebra rules it out: a
	// map is a SET, it has no insertion order, and keeping one would mean
	// storing it.
	//
	// PURE — it is a value, like `array` and `len`.
	{Name: "keys", Kind: "map-keys", Pure: true},
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
	// CONCATENATION, found the way `=` is found.
	//
	// It is the free monoid's operation (string-operations.md §1) and the only
	// string operation that does not factor through something else — `length`,
	// `=` and every encoding are folds, and `empty` is the literal `""`.
	//
	// A target that has no string type declares none, and a program using it
	// there is refused by name — the same answer JavaScript gives for the
	// `checked` primitive it does not declare.
	if _, have := tg.Prims["string-of"]; !have {
		if c, ok := tg.findBySpelling("string-of", 1); ok {
			c.Name = "string-of"
			c.Args = []string{"int"}
			c.Result = "string"
			c.Pure = true
			tg.Prims["string-of"] = c
			tg.Names = append(tg.Names, "string-of")
		}
	}
	if _, have := tg.Prims["concat"]; !have {
		if c, ok := tg.findBySpelling("concat", 2); ok {
			c.Name = "concat"
			c.Args = []string{"string", "string"}
			c.Result = "string"
			c.Pure = true
			tg.Prims["concat"] = c
			tg.Names = append(tg.Names, "concat")
		}
	}
	// AND THE REST OF INTEGER ARITHMETIC, on exactly `=`'s argument.
	//
	// `=` was promoted because equality on a finite type is portable and total.
	// So is addition inside ADR 0012's window, and so is every operator below —
	// [integers.md](../docs/spec/integers.md) asked the eleven questions on all
	// four hosts and found them to AGREE on everything inside it: division
	// truncates toward zero, the remainder takes the dividend's sign, and
	// `(a/b)*b + a%b == a`.
	//
	// Until this, `=` was the only integer operator the language owned, and
	// every "portable" claim in the repository was really a claim about `go.+`.
	// A program could not add two numbers without naming a host.
	//
	// Found rather than written twice, so the emission, the `where` on
	// division, and the `checked` variant ADR 0019 selects all come from the
	// target's own declaration.
	for _, op := range langOps {
		if _, have := tg.Prims[op.name]; have {
			continue
		}
		if p, ok := tg.findOpBySpelling(op.spellings, op.result); ok {
			p.Name = op.name
			p.Args = []string{"int", "int"}
			if op.result != "" {
				p.Result = op.result
			}
			p.Pure = true
			tg.Prims[op.name] = p
			tg.Names = append(tg.Names, op.name)
		}
	}
	// ARBITRARY PRECISION, THE RUNG ABOVE THE HOST'S WORD.
	//
	// ADR 0019's third escape: a range declared ABOVE the portable window
	// promotes that value to arbitrary precision. Found by spelling exactly the
	// way `=` and `+` are, so a target says how it does it in its own file and
	// nothing here learns that Go allocates a receiver or that Java's is
	// immutable.
	//
	// A TARGET MAY DECLINE THIS ONE, and that is not the exception to CLAUDE.md's
	// rule about language constructs. `int` is ADR 0012's window and every target
	// provides it; ABOVE the window there is no portability claim to keep, so a
	// missing bignum is the capability model answering — the same answer the
	// `checked` primitive already gives on JavaScript, which declares none.
	// windows declares none here, and ADR 0019 already says what it owes: a
	// bignum written in Oroboros, the way win/map is.
	for _, op := range bigOps {
		if _, have := tg.Prims[op.name]; have {
			continue
		}
		if p, ok := tg.findBySpelling(op.name, op.arity); ok {
			p.Name = op.name
			// PURITY IS NOT UNIFORM HERE, and forcing it was a bug the moment
			// the destination forms existed: `big+` allocates a fresh result and
			// reads nothing it can change, but `big+!` WRITES INTO its first
			// argument. Declaring that pure would let the reducer substitute one
			// call into two places, and both would write the same object — which
			// is exactly what ADR 0010's effect discipline exists to prevent.
			p.Pure = op.pure
			tg.Prims[op.name] = p
			tg.Names = append(tg.Names, op.name)
		}
	}
	sort.Strings(tg.Names)
}

// bigOps is the arbitrary-precision surface a target may declare. The names are
// the language's; each target spells the operation its own way.
//
// It is the SAME set as langOps plus `=`, plus two conversions — and the
// conversions go one way only. `big-of` widens a word to a bignum; there is no
// narrowing, because unbounded-rung.md §3 is that the promotion is a WIDENING
// and not a refinement, so a value that may leave the machine word cannot
// silently be used where an `int` is required. `big-str` exists because a value
// past 2^53 cannot be printed as an `int` on any of the four hosts.
var bigOps = []struct {
	name  string
	arity int
	pure  bool
}{
	{"big+", 2, true}, {"big-", 2, true}, {"big*", 2, true}, {"big/", 2, true}, {"big%", 2, true},
	{"big<", 2, true}, {"big<=", 2, true}, {"big>", 2, true}, {"big>=", 2, true}, {"big=", 2, true},
	{"big-of", 1, true}, {"big-str", 1, true},

	// `big-of-small` is deliberately NOT here. No target declares it: it is a
	// marker the representation pass writes and the fixed-limb lowering removes
	// before anything else looks, saying that a widened word is small enough to
	// be a limb multiplier (emit/bigrep.go's widenTo).

	// THE DESTINATION FORMS, where the first argument is the object written
	// into. A target that cannot mutate its bignum declares none of these, and
	// that is a fact about the host: `java.math.BigInteger` is immutable and
	// JavaScript's `BigInt` is a primitive, so on two of the three hosts that
	// HAVE a bignum the careful hand-written form does not exist either.
	{"big+!", 3, false}, {"big-!", 3, false}, {"big*!", 3, false},
	{"big/!", 3, false}, {"big%!", 3, false}, {"big-of!", 2, false},

	// THE REMAINDER BY A MACHINE WORD, WHOSE RESULT IS A WORD. `a % k` with
	// 0 <= a and 0 < k <= 2^53 is under k, so the answer fits the portable
	// window — that is a fact about the ARITHMETIC and not about anyone's
	// representation, so it must hold on all of them.
	//
	// Without it the two rungs disagreed about which programs type-check:
	// `(sig digit ((a (int 0 (pow 2 200)))) int)` with body `(% a 100000000)`
	// was accepted on the fixed-limb rung, where the loop naturally returns a
	// word, and refused on the host's bignum, where `big%` yields a bignum
	// against a declared `int`. Selecting a representation would then change
	// which programs are LEGAL, which is ADR 0009 at the representation
	// boundary — the same thing `big-fit` exists to prevent for the bound.
	//
	// It is not a new kind of thing: `big<` is already a big operation with a
	// non-big result. And it is what DECIMAL RENDERING is made of, since
	// printing needs each digit group as a word.
	{"big%-small", 2, true},

	// THE DECLARED BOUND, ENFORCED ON THE HOST'S OWN BIGNUM. `(big-fit x k)` is
	// x when x needs at most k bits and a trap otherwise.
	//
	// It exists because a bound is SEMANTICS and the representation is the
	// target's choice, so the two representations have to enforce the same
	// thing: the limb rung traps on its carry, and without this the host rung
	// would silently accept what the limb rung refuses. Then `(big-repr host)`
	// would not be a change of storage but a change of ANSWER, which is ADR
	// 0009's rule at the representation boundary.
	//
	// Bit length rather than the endpoint itself, because that is O(1) on all
	// three hosts that have a bignum where a full comparison costs what the
	// operation costs. See emit/biglimb.go's BigBound.
	{"big-fit", 2, false},

	// THE FIXED-LIMB RUNG'S CARRY CHECK (emit/bignum.oro). Every target has it,
	// because every target can fail: `panic`, `throw`, `throw`, `ud2`. It is
	// IMPURE — an operation that can end the program is not one to duplicate or
	// elide, which is exactly what ADR 0010's discipline is for.
	{"trap-if", 1, false},
}

// findBySpelling returns the target's own primitive whose UNQUALIFIED name is
// `want`, at the given arity. It is findOpBySpelling without the result-kind
// discrimination, which the big operators do not need: their names already say
// which is a comparison.
// spelled returns every primitive whose UNQUALIFIED name is `want`, in a
// DETERMINISTIC and principled order.
//
// THE EMITTED FILE MUST BE A FUNCTION OF ITS INPUT, and it was not. Five
// lookups here iterated `tg.Prims` — a Go map, whose range order is randomised
// — and returned the first match, so when a target declared one operation twice
// the winner was decided per run. `targets/js/` declares `concat` three times
// (the language's `%s + %s`, `js/Array.concat`, `js/String.concat`), and six
// identical `cmd/gen` runs over `examples/big/render.oro` produced TWO different
// programs. Every "byte-identical across N programs" claim in this repository
// rests on this not happening.
//
// The order is LEAST-QUALIFIED FIRST, then by name. That is not merely stable,
// it is the right preference: a name in the target's own core module is the
// operation, and the same name in a sub-module is a HOST API binding that
// happens to share it — overloading.md §3's distinction between a concept name
// and a method. `js.concat` beats `js/String.concat` for that reason and not
// because `.` sorts before `/`.
//
// A target declaring one operation twice is arguably ambiguous and could be
// refused. It is not, because both spellings here are correct and a refusal
// would make `targets/js/` illegal for declaring the String method it binds.
func (tg *Target) spelled(want string) []Prim {
	type cand struct {
		name string
		p    Prim
	}
	var cs []cand
	for name, p := range tg.Prims {
		seg := name
		if i := strings.LastIndex(seg, "."); i >= 0 {
			seg = seg[i+1:]
		}
		if seg == want {
			cs = append(cs, cand{name, p})
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		qi, qj := strings.Count(cs[i].name, "/"), strings.Count(cs[j].name, "/")
		if qi != qj {
			return qi < qj
		}
		return cs[i].name < cs[j].name
	})
	out := make([]Prim, len(cs))
	for i, c := range cs {
		out[i] = c.p
	}
	return out
}

func (tg *Target) findBySpelling(want string, arity int) (Prim, bool) {
	for _, p := range tg.spelled(want) {
		if len(p.Args) == arity && p.Kind == "expr" {
			return p, true
		}
	}
	return Prim{}, false
}

// HasBig reports whether this target declares arbitrary-precision integers.
func (tg *Target) HasBig() bool {
	_, ok := tg.Prims["big+"]
	return ok
}

// BigOp is the target's arbitrary-precision form of a language operator, if it
// has one. `+` becomes `big+`.
func BigOpName(op string) string { return "big" + op }

// langOps are the integer operators the LANGUAGE owns, with how a host may
// spell each — most preferred first, so a target that has both keeps the one
// its own programmers would write.
//
// BITWISE AND SHIFTS ARE DELIBERATELY ABSENT, and the reason is measured rather
// than cautious: JavaScript coerces both operands to **int32** for `& | ^ << >>`
// (and to uint32 for `>>>`), so `(2^32) & -1` is **0** on V8 and 4294967296 on
// Go, Java and x86 — an OBSERVABLE disagreement INSIDE ADR 0012's window.
// `targets/js/builtin.oro` says so already. They stay target-native, where a
// program using one has chosen its host. Promoting them conditionally, when a
// declared range fits int32, is a real design and is not made here.
//
// `/` and `%` carry whatever precondition the target declared — division by
// zero is a precondition, not a behaviour (integers.md §5) — because the host
// prim is copied wholesale rather than rebuilt.
// THE RESULT IS THE LANGUAGE'S AND NOT THE HOST'S, and that is a correction
// rather than a tidy-up. `targets/js` declares everything `any` on purpose —
// JavaScript has one number type — so the language's `+` inherited `any` there,
// and `emit/interval.go`'s transfer function ignores any operation whose result
// is not `int`. The consequence was that the analysis counted ZERO integer
// operations on JavaScript, and `Unbounded` returns success when there is
// nothing to prove.
//
// So ADR 0019's bounded-by-default was enforced on three targets and VACUOUS on
// the fourth — the one whose failure mode is SILENT PRECISION LOSS, which is the
// case the ADR's own headline example is about: fib(100) wraps on Go and the JVM
// and comes back 354224848179262000000 on V8. The refusal fired everywhere
// except where it mattered most.
//
// It is sound to say `int` here because these names are the LANGUAGE's:
// `addCore` already declares their arguments `(int int)`, and integers.md
// measured all four hosts agreeing on their meaning inside the window. Only the
// result had been left to whatever the host happened to say.
var langOps = []struct {
	name      string
	spellings []string
	result    string // the LANGUAGE's result type, not the host's
}{
	{"+", []string{"+", "add"}, "int"},
	{"-", []string{"-", "sub"}, "int"},
	{"*", []string{"*", "imul", "mul"}, "int"},
	{"/", []string{"idiv", "/"}, "int"},
	{"%", []string{"irem", "%", "rem"}, "int"},
	{"<", []string{"<", "setl"}, "bool"},
	{"<=", []string{"<=", "setle"}, "bool"},
	{">", []string{">", "setg"}, "bool"},
	{">=", []string{">=", "setge"}, "bool"},
}

// findOpBySpelling returns the target's own two-argument integer operator.
//
// It demands an INTEGER operator specifically: a host that declares `+` for
// both ints and floats (JavaScript does, as `any`) is fine, but Go's `f+` must
// never be picked for `+`, and it is not, because the segment compared is the
// whole unqualified name.
func (tg *Target) findOpBySpelling(spellings []string, result string) (Prim, bool) {
	for _, want := range spellings {
		for _, p := range tg.spelled(want) {
			if len(p.Args) != 2 || p.Kind != "expr" {
				continue
			}
			// The host's own result must agree with what the language expects:
			// a comparison yields a boolean and arithmetic does not. A host that
			// types everything `any` (JavaScript, on purpose) says nothing and
			// is accepted.
			if result == "bool" && p.Result != "bool" && p.Result != "any" {
				continue
			}
			if result != "bool" && p.Result == "bool" {
				continue
			}
			return p, true
		}
	}
	return Prim{}, false
}

// findEq returns the target's own integer equality, by spelling.
func (tg *Target) findEq() (Prim, bool) {
	for _, want := range eqSpellings {
		for _, p := range tg.spelled(want) {
			if len(p.Args) == 2 && p.Kind == "expr" {
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
	if len(terms) == 0 || formWord(terms[0]) != "target" {
		return nil, fmt.Errorf("%s: a target file begins with (target NAME …)", path)
	}
	// THE HEADER NAMES THE MODEL AND EVERY FORM AFTER IT IS GLUED IN. Whether a
	// form sits inside the header's parentheses or after them is the same fragment
	// by the homomorphism parseTarget folds with, so both load; the header-only
	// spelling is theories.md §5.1's.
	head := *terms[0]
	head.Kids = append(append([]*core.Term(nil), head.Kids...), terms[1:]...)
	return parseTarget(&head, path)
}

// Backends is the finite closed set of code generators — target-system.md §1's
// `𝔅`. A backend emits control flow and binds variables, which is why a target
// file may not add one and why `(structural …)` is closed for the same reason.
//
// `x86-64` currently implies MASM and the Win64 ABI; splitting those out is
// target-system.md §5.2's family question and wants a second ISA first.
var Backends = []string{"go", "js", "java", "x86-64"}

func isBackend(n string) bool {
	for _, b := range Backends {
		if b == n {
			return true
		}
	}
	return false
}

// ResolveBackend answers which code generator compiles this target.
//
// Declared wins. Otherwise the target's own name, when that names a backend —
// `(target go …)` writing `(backend go)` would be noise. Otherwise an ERROR,
// which is the fix for the bug this field exists for: there is no default,
// because a wrong default here is a silent miscompilation rather than a
// missing feature.
func (tg *Target) ResolveBackend() (string, error) {
	if tg.Backend != "" {
		return tg.Backend, nil
	}
	if isBackend(tg.Name) {
		return tg.Name, nil
	}
	return "", fmt.Errorf("target %q does not say which backend compiles it, and its name is "+
		"not one, so it cannot emit code.\n"+
		"  Add (backend NAME) to the target file, where NAME is one of: %s\n"+
		"  A backend is compiler code rather than data, so the set is closed.\n"+
		"\n"+
		"  Declaring none is legitimate: a target is a capability set first, and one\n"+
		"  that only parameterises the NORMAL FORM (ADR 0002) works with cmd/oro and"+
		"\n  has nothing to emit with. `blas` and the tutorial targets are exactly\n"+
		"  that. What is refused is emitting from such a target, which used to fall\n"+
		"  through to whichever generator the -target flag resembled — and is how a\n"+
		"  windows target once emitted Go control flow with x86 templates spliced\n"+
		"  into it (win32-2026-09-06 §6).",
		tg.Name, strings.Join(Backends, ", "))
}

// loadTargetDir glues every `.oro` file under `dir`, AT ANY DEPTH.
//
// It walks rather than reading one level because a module path is a word in the
// free monoid over segments and the directory tree is that monoid's trie: the
// declaration of `go/encoding/hex` belongs at `targets/go/encoding/hex.oro`, and
// with a single-level read there was nowhere for it to live
// (modpath-2026-09-20, ADR 0025). Nothing depends on the file NAME — a target
// file declares its own module path inside — so the layout is a picture of the
// trie rather than a mechanism, and the one mechanism this adds is that a
// deeper file is found at all.
//
// `loadProvides` has always walked the library layer, so this also makes the two
// halves of target loading agree.
func loadTargetDir(dir string) (*Target, error) {
	var files []string
	err := filepath.Walk(dir, func(fp string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(fp, ".oro") {
			files = append(files, fp)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	// NO `addCore` HERE. A directory is one LAYER, and the core's names are
	// resolved by SPELLING against the whole target — inject them per layer and
	// `concat` would be resolved against a fragment. The callers that have the
	// finished target do it.
	return out, nil
}

// THE TWO OPERATIONS OF target-system.md §2, AS ONE FOLD.
//
// Composing targets is always one of exactly two things, and they differ ONLY in
// what happens when two fragments declare the same name:
//
//	⊔  GLUE, within a layer.     They must agree; a collision is an error.
//	                             Commutative, associative, idempotent — so the
//	                             order files are discovered in cannot matter.
//	▷  OVERRIDE, between layers. The nearer layer wins, silently. Ordered, and
//	                             licensing disagreement is its whole purpose.
//
// Writing them as one walk with a `combiner` is not tidiness: it is the reason
// the two can be reasoned about together. Everything else — which fields exist,
// how a map is folded, that representations append in order — is shared, and the
// single point of difference is `how`.
type combiner int

const (
	glue     combiner = iota // ⊔ — agreement required
	override                 // ▷ — the value already present wins
)

// combineOne folds a single-valued field. `zero` means "not declared here",
// which is what makes gluing a partial map rather than a total one.
func combineOne[T comparable](dst *T, src T, what, from string, how combiner) error {
	var zero T
	if src == zero || *dst == src {
		return nil
	}
	if *dst == zero {
		*dst = src
		return nil
	}
	if how == glue {
		return fmt.Errorf("%s: %s is declared as %v and as %v", from, what, *dst, src)
	}
	return nil // ▷ — the nearer layer already said, and it wins
}

// combineMap folds a keyed field. The sheaf condition is per name: two fragments
// may both mention a name so long as they say the same thing about it.
func combineMap[T comparable](dst, src map[string]T, what, from string, how combiner) error {
	for n, v := range src {
		have, dup := dst[n]
		if !dup {
			dst[n] = v
			continue
		}
		if have == v {
			continue
		}
		if how == glue {
			return fmt.Errorf("%s: %s %s is declared as %v and as %v", from, what, n, have, v)
		}
	}
	return nil
}

// merge is ⊔ — the operation `loadTargetDir` performs over the files of one
// directory, and the one whose commutativity its comment already observed.
func (tg *Target) merge(o *Target, from string) error {
	return tg.combine(o, from, glue)
}

func (tg *Target) combine(o *Target, from string, how combiner) error {
	if tg.Boxed == nil && len(o.Boxed) > 0 {
		tg.Boxed = map[string]string{}
	}
	if err := combineMap(tg.Types, o.Types, "type", from, how); err != nil {
		return err
	}
	if tg.Aliases == nil && len(o.Aliases) > 0 {
		tg.Aliases = map[string]string{}
	}
	if err := combineMap(tg.Aliases, o.Aliases, "manifest type", from, how); err != nil {
		return err
	}
	if err := combineMap(tg.Boxed, o.Boxed, "boxed", from, how); err != nil {
		return err
	}
	for _, f := range []struct {
		dst  *string
		src  string
		what string
	}{
		{&tg.MapRepr, o.MapRepr, "map representation"},
		{&tg.ArrayType, o.ArrayType, "array-type"},
		{&tg.MapType, o.MapType, "map-type"},
		{&tg.Backend, o.Backend, "backend"},
		{&tg.BigRepr, o.BigRepr, "big-repr"},
		{&tg.Narrow, o.Narrow, "narrow"},
		{&tg.Artifact, o.Artifact, "artifact"},
		{&tg.Build, o.Build, "build"},
	} {
		if err := combineOne(f.dst, f.src, f.what, from, how); err != nil {
			return err
		}
	}
	for _, f := range []struct {
		dst  *int64
		src  int64
		what string
	}{
		{&tg.ShiftWidth, o.ShiftWidth, "shift-width"},
		{&tg.MaxLen, o.MaxLen, "max-len"},
	} {
		if err := combineOne(f.dst, f.src, f.what, from, how); err != nil {
			return err
		}
	}
	// ORDERED, so they append rather than folding by key — narrowest first is
	// the whole selection rule for `int-repr`. Nearest layer first, so a nearer
	// declaration is found before a built-in one.
	// A RELATION IS A SET, so glue and override are the same operation here and
	// a repeat is not a mistake: two layers may both know that `*os.File` reads.
	// That is why this is an append rather than a `combineMap` -- there is no
	// collision to detect, because there is no disagreement expressible.
	for mod, incs := range o.Includes {
		if tg.Includes == nil {
			tg.Includes = map[string][]string{}
		}
		for _, i := range incs {
			if !contains(tg.Includes[mod], i) {
				tg.Includes[mod] = append(tg.Includes[mod], i)
			}
		}
	}
	for sub, ifs := range o.Implements {
		if tg.Implements == nil {
			tg.Implements = map[string][]string{}
		}
		tg.Implements[sub] = append(tg.Implements[sub], ifs...)
	}
	tg.Reprs = append(tg.Reprs, o.Reprs...)
	tg.Data = append(tg.Data, o.Data...)
	// A deferred declaration is a declaration that has not been read yet, so it
	// travels with the fragment under BOTH operators and is elaborated once.
	tg.Deferred = append(tg.Deferred, o.Deferred...)
	// `D_T` composes per NAME like everything else: a repeat within a layer is a
	// mistake, and between layers the nearer one — already present, since layers
	// fold nearest first — wins.
	for path, fs := range o.Defs {
		if tg.Defs == nil {
			tg.Defs = map[string][]core.Form{}
		}
		have := map[string]bool{}
		for _, g := range tg.Defs[path] {
			if g.Kind == "def" {
				have[g.Name] = true
			}
		}
		for _, f := range fs {
			if f.Kind == "def" && have[f.Name] {
				if how == glue {
					return fmt.Errorf("%s: %s is defined twice in this target", from,
						qualify(path, f.Name))
				}
				continue
			}
			tg.Defs[path] = append(tg.Defs[path], f)
		}
	}
	// A library is a set member, so two layers naming one is not a collision.
	tg.Link = append(tg.Link, o.Link...)
	// A PRIMITIVE IS STRICTER THAN THE SHEAF CONDITION UNDER GLUE, deliberately:
	// two identical declarations of one name in a single layer are a mistake
	// rather than a coincidence, and saying so has caught real ones. Between
	// layers the nearer wins, which is what makes a project able to replace a
	// built-in binding.
	for n, p := range o.Prims {
		if _, dup := tg.Prims[n]; dup {
			if how == glue {
				return fmt.Errorf("%s: %s is declared twice in this target", from, n)
			}
			continue
		}
		tg.Prims[n] = p
		tg.Names = append(tg.Names, n)
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
	tg := newTarget(t.Kids[1].Name)

	// A FILE IS THE GLUE OF ITS FORMS, exactly as a directory is the glue of its
	// files: each form elaborates to a fragment and `merge` folds them. That is the
	// homomorphism load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂), so splitting a file in two
	// cannot change a target. Assigning fields as they were read did: a second
	// `(array-type …)` replaced the first in one file and was refused across two
	// (TestSplittingAFileChangesNothing).
	for _, f := range t.Kids[2:] {
		if f.Kind != core.KApp || f.Kids[0].Kind != core.KName {
			return nil, fmt.Errorf("%s: expected a declaration — (sig …), (type …), (repr …), (module …) — got %s",
				path, f)
		}
		if spelled, old := respelled[f.Kids[0].Name]; old {
			return nil, fmt.Errorf("%s: (%s …) is spelled %s (theories.md §8.4)", path, f.Kids[0].Name, spelled)
		}
		frag := newTarget(tg.Name)
		switch f.Kids[0].Name {
		case "implements":
			// (implements T I ...) -- T is accepted where any I is wanted.
			//
			// SEVERAL INTERFACES ON ONE LINE because the relation is a SET and a
			// concrete type usually satisfies a family: `*os.File` is a Reader, a
			// Writer, a Closer and a ReaderAt, and writing four lines would suggest
			// four independent facts.
			if len(f.Kids) < 3 {
				return nil, fmt.Errorf("%s: (implements TYPE IFACE ...), got %s", path, f)
			}
			for _, k := range f.Kids[1:] {
				if k.Kind != core.KName {
					return nil, fmt.Errorf("%s: (implements TYPE IFACE ...) takes names, got %s",
						path, f)
				}
			}
			if frag.Implements == nil {
				frag.Implements = map[string][]string{}
			}
			sub := f.Kids[1].Name
			for _, k := range f.Kids[2:] {
				frag.Implements[sub] = append(frag.Implements[sub], k.Name)
			}
		case "repr":
			if err := parseRepr(f, frag, path); err != nil {
				return nil, err
			}
		case "fact":
			if err := parseFact(f, frag, path); err != nil {
				return nil, err
			}
		case "sig", "const", "def", "use":
			// `def` and `use` reach `declare` only to be refused with the reason:
			// they are `D_T`, and `D_T` is keyed by module.
			if err := frag.declare(f, "", path); err != nil {
				return nil, err
			}
		case "type":
			if err := parseType(f, frag, "", path); err != nil {
				return nil, err
			}
		case "backend":
			// (backend NAME) — which code generator compiles this target.
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("%s: (backend NAME), one of %s; got %s",
					path, strings.Join(Backends, ", "), f)
			}
			if !isBackend(f.Kids[1].Name) {
				return nil, fmt.Errorf("%s: %q is not a backend. A backend is compiler code, "+
					"not data, so the set is closed: %s",
					path, f.Kids[1].Name, strings.Join(Backends, ", "))
			}
			frag.Backend = f.Kids[1].Name
		case "artifact":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (artifact \"name\"), got %s", path, f)
			}
			frag.Artifact = f.Kids[1].Str
		case "build":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (build \"cmd %%s %%s\"), got %s", path, f)
			}
			frag.Build = f.Kids[1].Str
		case "data":
			if len(f.Kids) != 2 || f.Kids[1].Kind != core.KStr {
				return nil, fmt.Errorf("%s: (data \"label ...\"), got %s", path, f)
			}
			frag.Data = append(frag.Data, f.Kids[1].Str)
		case "link":
			if len(f.Kids) < 2 {
				return nil, fmt.Errorf("%s: (link \"library\"…) names at least one library, got %s", path, f)
			}
			for _, k := range f.Kids[1:] {
				if k.Kind != core.KStr {
					return nil, fmt.Errorf("%s: (link \"library\"…) takes strings, got %s", path, f)
				}
				frag.Link = append(frag.Link, k.Str)
			}
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
			frag.Prims[p.Name] = p
			frag.Names = append(frag.Names, p.Name)
		case "module":
			// (module PATH (sig …) …) — the names this target provides
			// NATIVELY from that module. A target may provide any subset,
			// including none, which is what makes porting demand-driven
			// (modules.md §4).
			if len(f.Kids) < 2 || f.Kids[1].Kind != core.KName {
				return nil, fmt.Errorf("%s: (module PATH (sig …)…), got %s", path, f)
			}
			for _, inner := range f.Kids[2:] {
				// A TYPE IS A MEMBER OF ITS MODULE, named by the whole path
				// (theories.md §3.2, §3.4). A module in a target is a signature
				// Σ = (S, Ω), and its sorts belong to it: `go/io.Writer` has one
				// owner, and a base name shared by two packages is two types.
				if formWord(inner) == "include" {
					if len(inner.Kids) < 2 {
						return nil, fmt.Errorf("%s: (include COMPANION …), got %s", path, inner)
					}
					for _, k := range inner.Kids[1:] {
						if k.Kind != core.KName {
							return nil, fmt.Errorf("%s: (include …) takes module names, got %s", path, k)
						}
						if frag.Includes == nil {
							frag.Includes = map[string][]string{}
						}
						frag.Includes[f.Kids[1].Name] = append(frag.Includes[f.Kids[1].Name], k.Name)
					}
					continue
				}
				if formWord(inner) == "type" {
					if err := parseType(inner, frag, f.Kids[1].Name, path); err != nil {
						return nil, err
					}
					continue
				}
				if err := frag.declare(inner, f.Kids[1].Name, path); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("%s: unknown target form %q", path, f.Kids[0].Name)
		}
		if err := tg.merge(frag, path); err != nil {
			return nil, err
		}
	}
	sort.Strings(tg.Names)
	return tg, nil
}

func newTarget(name string) *Target {
	return &Target{Name: name, Types: map[string]string{}, Prims: map[string]Prim{}}
}

func lastKid(t *core.Term) *core.Term {
	if t.Kind != core.KApp || len(t.Kids) == 0 {
		return t
	}
	return t.Kids[len(t.Kids)-1]
}

// parseRepr reads a representation choice (theories.md §5.5–§5.9). It declares
// no name: it chooses how this model realizes something the language already
// has, so it composes by override like every other declaration.
func parseRepr(f *core.Term, frag *Target, path string) error {
	bad := func(want string) error { return fmt.Errorf("%s: %s, got %s", path, want, f) }
	if len(f.Kids) != 3 {
		return bad("(repr SUBJECT CHOICE)")
	}
	subj, choice := f.Kids[1], f.Kids[2]
	spelling, hosted := hostSpelling(choice)
	word := func(options ...string) (string, bool) {
		for _, o := range options {
			if subj.Kind == core.KName && choice.Kind == core.KName && choice.Name == o {
				return o, true
			}
		}
		return "", false
	}
	switch formWord(subj) {
	case "int":
		// Signedness is not a concept here and does not need to be: a host that
		// cannot store 0..255 in its byte — the JVM, whose `byte` is signed —
		// simply does not declare that range for it, and the range selects the
		// next one up. The declaration says what the host CAN hold, and nothing else.
		if subj.Kind != core.KApp || len(subj.Kids) != 3 || subj.Kids[1].Kind != core.KInt ||
			subj.Kids[2].Kind != core.KInt || !hosted {
			return bad(`(repr (int LO HI) (host "spelling"))`)
		}
		lo, hi := subj.Kids[1].Int, subj.Kids[2].Int
		if lo > hi {
			return fmt.Errorf("%s: (int %d %d) is empty", path, lo, hi)
		}
		frag.Reprs = append(frag.Reprs, IntRepr{Lo: lo, Hi: hi, Spell: spelling})
	case "ref":
		if subj.Kind != core.KApp || len(subj.Kids) != 2 || subj.Kids[1].Kind != core.KName || !hosted {
			return bad(`(repr (ref T) (host "spelling"))`)
		}
		frag.Boxed = map[string]string{subj.Kids[1].Name: spelling}
	case "big":
		w, ok := word("host", "limbs")
		if !ok {
			return bad("(repr big host) or (repr big limbs)")
		}
		frag.BigRepr = w
	case "map":
		w, ok := word("host", "library")
		if !ok {
			return bad("(repr map host) or (repr map library)")
		}
		frag.MapRepr = w
	case "shift":
		// N is a number of bits, and the upper limit is the portable window's own:
		// a host that claimed to shift values it cannot represent exactly would be
		// claiming something ADR 0012 already denies.
		if subj.Kind != core.KName || choice.Kind != core.KInt || choice.Int < 1 || choice.Int > 63 {
			return bad("(repr shift N) with 1 <= N <= 63")
		}
		frag.ShiftWidth = choice.Int
	case "narrow":
		if subj.Kind != core.KName || !hosted {
			return bad(`(repr narrow (host "dst = src[:n]"))`)
		}
		frag.Narrow = spelling
	default:
		return bad("(repr (int LO HI) | (ref T) | big | map | shift | narrow  CHOICE)")
	}
	return nil
}

// parseFact reads the one fact a target may state before facts are built
// (theories.md §7): every array on this model is at most N long.
//
//	(fact NAME ((a (array A))) (<= (len a) N))
//
// It is a fact because it IS one — a proposition true of every array here,
// which the refinement layer assumes — and any other fact is refused rather
// than read as this one.
// parseType reads a sort declaration: `(type NAME (host "spelling"))`, and the
// two constructors `lang` owns which every typed target must realize,
// `(type (array A) …)` and `(type (map K V) …)` (theories.md §5.6).
//
// A MODULE IN A TARGET IS A SIGNATURE Σ = (S, Ω), AND ITS SORTS BELONG TO IT.
// Declared inside `(module PATH …)` a type is named `PATH.NAME`, so it has one
// owner and a base name two packages share is two types — measured: 3 of Go's
// 1,270 exported type names collide by base name, two of them distinct structs
// (theories.md §2.2, declaration-surface.md §4). A type constructor is the
// TARGET's, not a module's: `array` and `map` are `lang`'s own, realized once
// per target, so declaring one inside a module is refused.
func parseType(f *core.Term, frag *Target, modPath, path string) error {
	// `(type NAME τ)` — A DEFINIENS RATHER THAN A REALIZATION, which is the
	// MANIFEST type of theories.md §2's four cells at the type level. It is
	// unfolded by δ on the glued target and the name then does not exist, which
	// is why it may not also carry a host spelling: emit/alias.go.
	if len(f.Kids) == 3 && f.Kids[1].Kind == core.KName && formWord(f.Kids[2]) != "host" {
		ty, err := aliasOf(f, path)
		if err != nil {
			return err
		}
		n := f.Kids[1].Name
		if langTypes[n] || core.IsTypeFormer(n) {
			return fmt.Errorf("%s: (type %s …): %s is the language's own type and a target does not "+
				"get to say what it means (docs/spec/booleans.md's rule, one level up)", path, n, n)
		}
		if frag.Aliases == nil {
			frag.Aliases = map[string]string{}
		}
		frag.Aliases[qualify(modPath, n)] = ty
		return nil
	}
	s, ok := hostSpelling(lastKid(f))
	if !ok || len(f.Kids) != 3 {
		return fmt.Errorf("%s: (type NAME (host \"spelling\")) or (type NAME τ), got %s", path, f)
	}
	switch n := f.Kids[1]; {
	case n.Kind == core.KName:
		frag.Types[qualify(modPath, n.Name)] = s
	case modPath != "":
		return fmt.Errorf("%s: (type %s …) in module %s: `array` and `map` are the language's own "+
			"type constructors and a target realizes each once, outside any module (theories.md §5.6)",
			path, n, modPath)
	case formWord(n) == "array" && len(n.Kids) == 2 && n.Kids[1].Kind == core.KName:
		frag.ArrayType = s
	case formWord(n) == "map" && len(n.Kids) == 3 &&
		n.Kids[1].Kind == core.KName && n.Kids[2].Kind == core.KName:
		frag.MapType = s
	default:
		return fmt.Errorf("%s: (type NAME (host …)), (type (array A) (host …)) or "+
			"(type (map K V) (host …)), got %s", path, f)
	}
	return nil
}

// qualify joins a module path to a member name, as resolution does.
func qualify(modPath, name string) string {
	if modPath == "" {
		return name
	}
	return modPath + "." + name
}

func parseFact(f *core.Term, frag *Target, path string) error {
	var n int64
	ok := len(f.Kids) == 4 && f.Kids[1].Kind == core.KName
	if ok {
		ps, body := f.Kids[2], f.Kids[3]
		ok = ps.Kind == core.KApp && len(ps.Kids) == 1 && ps.Kids[0].Kind == core.KApp &&
			len(ps.Kids[0].Kids) == 2 && ps.Kids[0].Kids[0].Kind == core.KName &&
			strings.HasPrefix(core.TypeName(ps.Kids[0].Kids[1]), "array ")
		ok = ok && formWord(body) == "<=" && len(body.Kids) == 3 && body.Kids[2].Kind == core.KInt
		if ok {
			l := body.Kids[1]
			ok = formWord(l) == "len" && len(l.Kids) == 2 && l.Kids[1].Kind == core.KName &&
				l.Kids[1].Name == ps.Kids[0].Kids[0].Name
			n = body.Kids[2].Int
		}
	}
	switch {
	case !ok:
		return fmt.Errorf("%s: facts are specified (theories.md §7) and not built; the one fact a "+
			"target may state today is (fact NAME ((a (array A))) (<= (len a) N)), got %s", path, f)
	case n < 1:
		return fmt.Errorf("%s: an array bound must be at least 1, got %d", path, n)
	case n > portableMaxLen:
		return fmt.Errorf("%s: an array bound of %d is outside the portable window; a length this "+
			"target cannot count exactly is not a length (ADR 0012)", path, n)
	}
	frag.MaxLen = n
	return nil
}

// declare records one primitive under a module path. The name stored is the
// FULLY QUALIFIED one, because that is what resolution produces and R1 requires
// targets and libraries to key the same namespace (modules.md §5).
func (tg *Target) declare(f *core.Term, modPath, file string) error {
	// `(def NAME term)` and `(use PATH [as A])` are `D_T`, not declarations:
	// they are the target's own library, and they go to core rather than to the
	// primitive table (target-system.md §6).
	if w := formWord(f); w == "def" || w == "use" {
		if modPath == "" {
			return fmt.Errorf("%s: (%s …) is the target's own library and belongs to a module — "+
				"write it inside (module PATH …) or (provides TARGET PATH …) (theories.md §5.4)",
				file, w)
		}
		fm, err := core.ToForm(f)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if tg.Defs == nil {
			tg.Defs = map[string][]core.Form{}
		}
		for _, g := range tg.Defs[modPath] {
			if g.Kind == "def" && fm.Kind == "def" && g.Name == fm.Name {
				return fmt.Errorf("%s: %s is defined twice in this target", file,
					qualify(modPath, fm.Name))
			}
		}
		tg.Defs[modPath] = append(tg.Defs[modPath], fm)
		return nil
	}
	// A CONSTANT'S NAME AS A RANGE ENDPOINT needs the definiens, which may be
	// declared in another file or another layer, so the declaration waits for
	// the finished target (emit/constend.go).
	if namesEndpoint(f) {
		tg.Deferred = append(tg.Deferred, deferred{form: f, mod: modPath, file: file})
		return nil
	}
	p, err := parseSig(f, file)
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

// lenEquals is a length postcondition built as a term: `len(result) = arg` for
// a count, `len(result) = len(arg)` for a pass-through — the two shapes
// refine.go's lengthContract reads, for the compiler's own structural tables.
func lenEquals(arg string, count bool) *core.Term {
	rhs := core.Name(arg)
	if !count {
		rhs = core.App(core.Name("len"), core.Name(arg))
	}
	return core.App(core.Name("="), core.App(core.Name("len"), core.Name(core.ResultName)), rhs)
}

// respelled are the target forms theories.md §8.4 respelled. They are refused
// rather than kept as aliases, for data.md §10's reason: two spellings of one
// declaration is the shape `merge` had before glue and override were separated.
var respelled = map[string]string{
	"prim":        `(sig NAME ((x τ)…) τ clause… (host KIND "template" hclause…))`,
	"int-repr":    `(repr (int LO HI) (host "spelling"))`,
	"big-repr":    `(repr big host) or (repr big limbs)`,
	"shift-width": `(repr shift N)`,
	"max-len":     `(fact max-len ((a (array A))) (<= (len a) N))`,
	"array-type":  `(type (array A) (host "spelling"))`,
	"map-type":    `(type (map K V) (host "spelling"))`,
	"boxed":       `(repr (ref T) (host "spelling"))`,
	"builtin-map": `(repr map library)`,
	"narrow":      `(repr narrow (host "spelling"))`,
}

// The words a `sig` may carry, split by WHO they are a claim about. A sig clause
// is a fact about the operation's meaning; a host clause is text for, or a fact
// about, one host (theories.md §5.2). Keeping the second set inside `(host …)` is
// what makes "no host clause" mean "no claim about any host".
var (
	sigWords  = map[string]bool{"pure": true, "index": true, "where": true, "ensures": true, "length": true, "length-of": true}
	hostWords = map[string]bool{"import": true, "lib": true, "checked": true, "jump": true}
)

// constSig elaborates a constant to the declaration it means (theories.md §8.3):
//
//	(const NAME v (host "spelling" hclause…))
//	  =  (sig NAME () (int v v) pure (host expr "spelling" hclause…))
//
// It is the CONDITIONAL cell of §2: a definiens, which the analysis reads as the
// exact range [v, v], and a realization, which the emitter writes. The spelled-out
// sig states v twice — once in the range and once, implicitly, as whatever the
// host's name holds — and a value written twice is two claims that can disagree.
//
// ONLY AN INTEGER IS A CONSTANT HERE, and that is the definition, not a gap. A
// constant is a declaration with a definiens the compiler may USE. An integer's
// value is used: it proves arithmetic on the name. A float's may not be folded
// (ADR 0009), and nothing analyses a string or a bool, so their definiens is
// never read. For them a zero-argument sig already says everything true.
func constSig(f *core.Term, path string) (*core.Term, error) {
	if len(f.Kids) != 4 || f.Kids[1].Kind != core.KName || formWord(f.Kids[3]) != "host" {
		return nil, fmt.Errorf("%s: (const NAME INTEGER (host \"spelling\" hclause…)), got %s", path, f)
	}
	name, v, host := f.Kids[1], f.Kids[2], f.Kids[3]
	if v.Kind != core.KInt {
		return nil, fmt.Errorf("%s: const %s: %s is not an integer literal. A constant's value is a fact "+
			"the analysis uses, and only an integer's is one; for any other value write "+
			"(sig %s () TYPE pure (host expr \"spelling\"))", path, name.Name, v, name.Name)
	}
	if len(host.Kids) < 2 || host.Kids[1].Kind != core.KStr {
		return nil, fmt.Errorf("%s: const %s: (host \"spelling\" hclause…) — a constant is a value, so "+
			"its host clause has no kind; got %s", path, name.Name, host)
	}
	app := func(kids ...*core.Term) *core.Term { return &core.Term{Kind: core.KApp, Kids: kids} }
	realize := app(append([]*core.Term{core.Name("host"), core.Name("expr")}, host.Kids[1:]...)...)
	return app(core.Name("sig"), name, app(), app(core.Name("int"), v, v), core.Name("pure"), realize), nil
}

// parseSig reads the declaration a target realizes:
//
//	(sig NAME ((x τ)…) τ clause… (host KIND "template" hclause…))
//
// It is `prim` with its two kinds of claim separated, so it elaborates to the
// same Prim by the same reader: the host clause's kind, template and clauses
// are handed to primOf beside the sig's own.
func parseSig(f *core.Term, path string) (Prim, error) {
	if formWord(f) == "const" && f.Kind == core.KApp {
		s, err := constSig(f, path)
		if err != nil {
			return Prim{}, err
		}
		f = s
	}
	if w := formWord(f); w != "sig" || f.Kind != core.KApp {
		if spelled, old := respelled[w]; old {
			return Prim{}, fmt.Errorf("%s: (%s …) is spelled %s (theories.md §8.4)", path, w, spelled)
		}
		return Prim{}, fmt.Errorf("%s: expected (sig …), got %s", path, f)
	}
	if len(f.Kids) < 4 {
		return Prim{}, fmt.Errorf("%s: (sig NAME ((x τ)…) τ clause… (host KIND \"template\" …)), got %s", path, f)
	}
	name := f.Kids[1]
	var host *core.Term
	var rest []*core.Term
	for _, c := range f.Kids[4:] {
		w := formWord(c)
		switch {
		case w == "host":
			if host != nil {
				return Prim{}, fmt.Errorf("%s: sig %s gives (host …) twice", path, name)
			}
			host = c
		case sigWords[w]:
			rest = append(rest, c)
		case hostWords[w]:
			return Prim{}, fmt.Errorf("%s: sig %s: (%s …) is a claim about the host, and goes inside (host …)",
				path, name, w)
		default:
			return Prim{}, fmt.Errorf("%s: sig %s: unexpected %s; a sig takes pure, index, (where …), "+
				"(ensures …) and one (host …)", path, name, c)
		}
	}
	if host == nil {
		return Prim{}, fmt.Errorf("%s: sig %s has no (host …) clause. A target REALIZES a declaration, "+
			"and one with no realization belongs in a module (theories.md §5.2)", path, name)
	}
	if len(host.Kids) < 3 || host.Kids[2].Kind != core.KStr {
		return Prim{}, fmt.Errorf("%s: sig %s: (host KIND \"template\" hclause…), got %s", path, name, host)
	}
	for _, c := range host.Kids[3:] {
		if !hostWords[formWord(c)] {
			return Prim{}, fmt.Errorf("%s: sig %s: (host …) takes (import …), (lib …), (checked …) and "+
				"(jump …), got %s", path, name, c)
		}
	}
	return primOf(name, f.Kids[2], f.Kids[3], host.Kids[1], append(rest, host.Kids[2:]...), path)
}

// formWord is the head of `(word …)`, or a bare word itself, or "".
func formWord(t *core.Term) string {
	switch {
	case t.Kind == core.KName:
		return t.Name
	case t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName:
		return t.Kids[0].Name
	}
	return ""
}

// hostSpelling reads `(host "spelling")`.
func hostSpelling(t *core.Term) (string, bool) {
	if formWord(t) != "host" || len(t.Kids) != 2 || t.Kids[1].Kind != core.KStr {
		return "", false
	}
	return t.Kids[1].Str, true
}

func primOf(nameT, argsT, resultT, kindT *core.Term, rest []*core.Term, path string) (Prim, error) {
	k := []*core.Term{nameT, argsT, resultT, kindT}
	if k[0].Kind == core.KBool {
		return Prim{}, fmt.Errorf("%s: `%s` is a literal of the language, not a name a target "+
			"declares (docs/spec/booleans.md)", path, k[0])
	}
	if k[0].Kind != core.KName {
		return Prim{}, fmt.Errorf("%s: a sig needs a name, got %s", path, k[0])
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

	// SEVERAL RESULTS — `(prim Open ((p string)) (ptr error) expr …)`.
	//
	// `Prim.Result` was a single string, and that one field was the largest
	// single obstacle to the whole parasite thesis: it refused **19.8% of Go's
	// callable standard library**, more than every language-level limitation
	// combined, and it compounds, because `(T, error)` is Go's CONSTRUCTOR
	// idiom — so every name it blocked also blocked every method on the type
	// that name would have returned (gostdlib-2026-09-06 §4a).
	//
	// The LANGUAGE has had several results since values.md: `(values a b)` is
	// reader sugar for `(fn (#k) (#k a b))`, the negative product, measured at
	// 0.99x on Go with zero allocations, and NO TARGET DECLARES IT. What was
	// missing was only the ability for a target to say a HOST call has that
	// shape — and the consuming form needs nothing new either, because
	// `((f x) (fn (a b) …))` is how a product is eliminated already.
	//
	// A result type may be COMPOUND: `(array string)`, the same spelling the
	// signature language uses. Without it a target's array types have to be
	// enumerated — `string-array`, `long-array`, `double-array` — which is the
	// suffix explosion `(array V)` exists to delete (tables.md §10), and it
	// showed up as `final /*unknown*/ w = ws[(int) i]` the first time a native
	// Java program indexed the result of `split`. And several results are a
	// TUPLE, `(tuple ptr error)`, exactly as in a program's sig (spec/data.md §3.3).
	switch rt := core.TypeName(k[2]); {
	case core.IsProd(rt):
		p.Results = core.ProdTypes(rt)
	case rt != "":
		p.Result = rt
	default:
		return Prim{}, fmt.Errorf("%s: %s has a result that is not a type; several results are "+
			"(tuple A B …): %s", path, p.Name, k[2])
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

	for _, rest := range rest {
		switch {
		case rest.Kind == core.KStr:
			p.Form = rest.Str
		case rest.Kind == core.KName && rest.Name == "pure":
			p.Pure = true
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "where" && len(rest.Kids) == 2:
			// A second clause used to REPLACE the first, so a precondition the
			// author wrote vanished without a word. A conjunction is one clause.
			if p.Where != nil {
				return Prim{}, fmt.Errorf("%s: %s gives (where …) twice, and one would be dropped; "+
					"write one (where (and …))", path, p.Name)
			}
			p.Where = rest.Kids[1]
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "ensures" && len(rest.Kids) == 2:
			if p.Ensures != nil {
				return Prim{}, fmt.Errorf("%s: %s gives (ensures …) twice, and one would be dropped; "+
					"write one (ensures (and …))", path, p.Name)
			}
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
			(rest.Kids[0].Name == "length" || rest.Kids[0].Name == "length-of"):
			// RESPELLED as the postcondition it always was (theories.md §8.4), and
			// refused naming the new spelling, as every respelled form is.
			if rest.Kids[0].Name == "length" {
				return Prim{}, fmt.Errorf("%s: %s: (length N) is spelled "+
					"(ensures (= (len result) n)), naming argument N as n", path, p.Name)
			}
			return Prim{}, fmt.Errorf("%s: %s: (length-of N) is spelled "+
				"(ensures (= (len result) (len c))), naming argument N as c", path, p.Name)
		case rest.Kind == core.KName && rest.Name == "index":
			if len(p.Args) != 2 {
				return Prim{}, fmt.Errorf("%s: %s is marked index but does not take "+
					"a container and an index", path, p.Name)
			}
			p.Index = true
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "import" && len(rest.Kids) == 2 && rest.Kids[1].Kind == core.KStr:
			p.Import = rest.Kids[1].Str
		case rest.Kind == core.KApp && rest.Kids[0].Kind == core.KName &&
			rest.Kids[0].Name == "lib" && len(rest.Kids) == 2 && rest.Kids[1].Kind == core.KStr:
			p.Lib = rest.Kids[1].Str
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
		// A MODULE'S IMPORTS COME FIRST, AND THIS BACKEND HAD NONE.
		//
		// `(import ...)` is one field of the shared target format: Go collects it,
		// Java collects it, x86 turns it into an extern - and JavaScript DROPPED it,
		// silently, because no target file for this host had ever declared one. A
		// path nothing runs is a path nothing checks, and here that was a whole
		// backend: the first primitive on this target needing a module could not
		// have had one.
		//
		// Sorted, because backend-2026-09-06 found the emitter was not a function of
		// its input and the fix is an order rather than a hope.
		for _, imp := range sortedSet(JSImports) {
			fmt.Fprintf(&b, "import * as %s from %q;\n", jsImportAlias(imp), imp)
		}
		if len(JSImports) > 0 {
			b.WriteString("\n")
		}
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
		// THE COMMAND LINE IS THE ONE THING A JVM PROGRAM CANNOT ASK FOR.
		//
		// Go has `os.Args` and Node has `process.argv`; the JVM hands its
		// arguments to `main` and offers no global, so `os.Args` on this target is
		// the only declaration in any target file that needs the program LAYOUT to
		// cooperate. It is emitted only when something reads it, which keeps every
		// program that does not byte-identical - and the test is the honest one,
		// since the name appearing in the code is exactly what makes the field
		// necessary.
		//
		// The empty string at index 0 is the parasite model rather than padding:
		// `os.Args` and `process.argv.slice(1)` both start with the program, so
		// prepending one here makes `(av 1)` the first argument on all three hosts.
		wantArgs := strings.Contains(code, "oroArgs")
		if wantArgs {
			b.WriteString("\tstatic String[] oroArgs = new String[0];\n")
		}
		b.WriteString(code)
		b.WriteString("\n\tpublic static void main(String[] args) {\n")
		if wantArgs {
			b.WriteString("\t\toroArgs = new String[args.length + 1];\n")
			b.WriteString("\t\toroArgs[0] = \"\";\n")
			b.WriteString("\t\tSystem.arraycopy(args, 0, oroArgs, 1, args.length);\n")
		}
		fmt.Fprintf(&b, "\t\t%s();\n\t}\n}\n", javaMangle(entry))
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
		return os.WriteFile(filepath.Join(dir, "build.bat"), []byte(asmBuildScript(tg.Link, AsmLibs)), 0o644)
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
link -nologo -subsystem:console -entry:main main.obj {{LIBS}} -out:main.exe || exit /b 1
`

// asmBuildScript fills the link line. It is `link` in declared order, then each
// library a USED primitive names that is not already there, sorted — so the
// script is a function of the program and the target, and a program that calls
// nothing new gets exactly the line it always had.
//
// Windows library names are case-insensitive, so presence is decided on the
// lowercase spelling; the declared spelling is what is written.
func asmBuildScript(link []string, used map[string]bool) string {
	var libs []string
	have := map[string]bool{}
	for _, l := range link {
		if k := strings.ToLower(l); !have[k] {
			have[k] = true
			libs = append(libs, l+".lib")
		}
	}
	var extra []string
	for l := range used {
		if k := strings.ToLower(l); !have[k] {
			have[k] = true
			extra = append(extra, l)
		}
	}
	sort.Strings(extra)
	for _, l := range extra {
		libs = append(libs, l+".lib")
	}
	return strings.Replace(asmBuildBat, "{{LIBS}}", strings.Join(libs, " "), 1)
}

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
// `self` is the buffer being filled, and a read of it answers NO at any depth.
// A BUFFER MAY NOT NARROW ON ITS OWN CONTENTS — the rule already stated below
// for a bare read, threaded through the recursion because arithmetic can bury
// one: `(+ (b i) 1)` is an increment of a slot, and deciding `b`'s element
// range from it would be reasoning in a circle. Pass "" where there is no
// buffer in question.
func storedRange(v *core.Term, typeOf func(*core.Term) string, self string) (int64, int64, bool) {
	if v == nil {
		return 0, 0, false
	}
	if v.Kind == core.KInt {
		return v.Int, v.Int, true
	}
	if self != "" && v.Kind == core.KApp && len(v.Kids) == 2 &&
		v.Kids[0].Kind == core.KName && v.Kids[0].Name == self {
		return 0, 0, false
	}
	// `if` is injected into every target and declaring one is an error
	// (ADR 0017), so there is exactly one spelling to match.
	if v.Kind == core.KApp && len(v.Kids) == 4 && v.Kids[0].Kind == core.KName &&
		v.Kids[0].Name == "if" {
		alo, ahi, aok := storedRange(v.Kids[2], typeOf, self)
		blo, bhi, bok := storedRange(v.Kids[3], typeOf, self)
		if !aok || !bok {
			return 0, 0, false
		}
		return min64(alo, blo), max64(ahi, bhi), true
	}
	// A COPY FROM AN ALREADY-NARROWED TABLE carries that table's element range.
	// elemwidth-2026-08-27 states this rule — *"a read from an already-narrowed
	// table carries one"* — and this path never implemented it, because nothing
	// had copied bytes out of one table into another until a program formatted
	// a file (examples/io/jsonfmt.oro). The tokeniser's stack stores literals;
	// the tree's node table stores indices.
	//
	// It has to ask for the table's type rather than the READ's, because
	// `typeOf` normalises a range to `int` — correct for the value, since a
	// local reading a byte array is an `int` and cannot overflow at 255, and
	// exactly what destroys the fact here. That is scalarrange-2026-08-31's
	// three effects of a range: `typeOf` gives the TYPING answer and this wants
	// the REPRESENTATION one.
	if v.Kind == core.KApp && len(v.Kids) == 2 && v.Kids[0].Kind == core.KName {
		if elem := core.ArrayElem(typeOf(v.Kids[0])); elem != "" {
			if lo, hi, ok := core.IntRange(elem); ok {
				return lo, hi, true
			}
		}
	}
	// ARITHMETIC WHOSE OPERANDS ARE THEMSELVES EXACT.
	//
	// The rules above are closed under nothing, which is what a program that
	// COMPUTES a byte runs into: `(+ 48 (% d 10))` is a digit, every operand is
	// exact, and the composite was refused — so a text program could copy bytes
	// into a byte buffer and not produce one. examples/io/freq.oro is the first
	// program to write a number into text it is building.
	//
	// This is still not the interval analysis and the distinction is the one
	// this function's header makes. Interval ARITHMETIC is exact on exact
	// endpoints; what fixpoint-2026-08-27 withdrew is the interval FIXPOINT,
	// whose endpoints come from widening a loop. There is no fixpoint here and
	// no loop variable can enter, because a loop variable is a bare name and a
	// bare name is decided by `typeOf` alone.
	if v.Kind == core.KApp && len(v.Kids) == 3 && v.Kids[0].Kind == core.KName {
		switch arithOp(v.Kids[0].Name, 2) {
		case "add", "sub", "mul":
			alo, ahi, aok := storedRange(v.Kids[1], typeOf, self)
			blo, bhi, bok := storedRange(v.Kids[2], typeOf, self)
			if aok && bok {
				if lo, hi, ok := exactArith(arithOp(v.Kids[0].Name, 2),
					alo, ahi, blo, bhi); ok {
					return lo, hi, true
				}
			}
		case "rem":
			// |a %% d| < |d| WHATEVER a IS. The remainder takes the dividend's
			// sign and its magnitude is under the divisor's, on all four
			// targets inside the portable window (integers.md §5), so a literal
			// divisor bounds the result with no fact about the dividend at all.
			// That is what makes a digit exact: nothing bounds `x / p` and
			// `(%% (/ x p) 10)` is still in -9..9.
			//
			// F7, now read from the same facts as the interval layer's `%`: an
			// exact range is Theorem T on a point interval, with the dividend ⊤.
			if d := v.Kids[2]; d.Kind == core.KInt && d.Int != 0 && d.Int != math.MinInt64 {
				if r := remI(top, exact(d.Int)); r.bounded() {
					return r.lo, r.hi, true
				}
			}
		}
	}
	return core.IntRange(typeOf(v))
}

// exactArith is interval arithmetic on exact endpoints, refusing on overflow.
// Refusing is the safe direction: the buffer keeps the host's own width.
func exactArith(op string, alo, ahi, blo, bhi int64) (int64, int64, bool) {
	switch op {
	case "add":
		return addChk(alo, blo), addChk(ahi, bhi), !ovf(alo, blo) && !ovf(ahi, bhi)
	case "sub":
		return addChk(alo, -bhi), addChk(ahi, -blo),
			bhi != math.MinInt64 && blo != math.MinInt64 &&
				!ovf(alo, -bhi) && !ovf(ahi, -blo)
	case "mul":
		lo, hi, ok := int64(0), int64(0), true
		for i, x := range [4]int64{alo, alo, ahi, ahi} {
			y := [4]int64{blo, bhi, blo, bhi}[i]
			p, good := mulChk(x, y)
			if !good {
				ok = false
				break
			}
			if i == 0 || p < lo {
				lo = p
			}
			if i == 0 || p > hi {
				hi = p
			}
		}
		return lo, hi, ok
	}
	return 0, 0, false
}

func ovf(a, b int64) bool {
	c := a + b
	return (c > a) != (b > 0)
}

func addChk(a, b int64) int64 { return a + b }

func mulChk(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	c := a * b
	if c/b != a {
		return 0, false
	}
	return c, true
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

// LoopOneJoin gives every `build` that feeds one loop variable THE SAME element
// type: the join over all of that variable's sources.
//
// A LOOP-CARRIED BUFFER HAS SEVERAL SOURCES AND ONE VARIABLE, and until the
// interval analysis got precise enough about division nothing in the corpus had
// two that disagreed. A bignum accumulator does: it starts as a buffer whose
// stores are provably 0..1 and is replaced each iteration by one whose stores
// are 0..2^24-1. Each was narrowed on its own and Go refused the file —
// `cannot use []int32 as []byte`.
//
// A COMPILE ERROR IS THE LUCKY CASE, and only two of the four hosts give one.
// Go and Java type their slices; JavaScript narrows nothing; and on x86 the
// element size travels BY NAME (wintables-2026-08-25), so reading a byte table
// as qwords is a wrong answer rather than a slow one. One shape, three
// outcomes, one of them silent.
//
// It is per-LOOP and takes the body already OPEN, which is not a convenience:
// every binder the emitter opens rebuilds its subtree, so a map keyed by a
// pointer under a `let` is read against a copy — four identical sequenced calls
// gave the first one the joined width and the other three their own. Carrying
// the fact to the subterm is the answer `Refine` and `intervalsAssuming`
// already use, and this is the third time the alternative has been tried here.
//
// Refusing is the fallback: a source this cannot read takes the whole variable
// back to the host's word, because a narrowing that is wrong for one source is
// wrong for the buffer.
func LoopOneJoin(tgt *Target, inits []*core.Term, openBody *core.Term,
	raw []string, typeOf func(*core.Term) string, sig *core.Sig,
	params []string) map[*core.Term]string {
	out := map[*core.Term]string{}
	joinSources(tgt, inits, againTerms(tgt, openBody), typeOf, sig, params, out)
	return out
}

func joinSources(tgt *Target, inits, agains []*core.Term,
	typeOf func(*core.Term) string, sig *core.Sig, params []string,
	out map[*core.Term]string) {
	for k, init := range inits {
		lams, tys, ok := []*core.Term{}, []string{}, true
		add := func(src *core.Term) {
			if src == nil {
				return
			}
			lam, isBuild := buildLambda(tgt, src)
			if !isBuild {
				// A PASS-THROUGH IS NOT A SOURCE — it hands the same buffer
				// back, so it adds nothing and must not veto. Anything else is
				// a source this cannot read, and does.
				if src.Kind != core.KBound && src.Kind != core.KName {
					ok = false
				}
				return
			}
			b, r, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
			lams = append(lams, lam)
			tys = append(tys, ElemType(tgt, lam, b, r[0], typeOf, sig, params))
		}
		add(init)
		for _, ag := range agains {
			if as := ag.Args(); k < len(as) {
				add(as[k])
			}
		}
		if !ok || len(lams) < 2 {
			continue // one source cannot disagree with itself
		}
		j := joinElemTypes(tys)
		same := true
		for _, ty := range tys {
			if ty != j {
				same = false
			}
		}
		if same {
			continue
		}
		for _, lam := range lams {
			out[lam] = j
		}
	}
}

// joinElemTypes is the join in the element lattice: ranges join to their hull,
// and anything that is not a range takes the whole variable to the host's word.
func joinElemTypes(tys []string) string {
	lo, hi, seen := int64(0), int64(0), false
	for _, ty := range tys {
		l, h, ok := core.IntRange(ty)
		if !ok {
			return "int"
		}
		if !seen {
			lo, hi, seen = l, h, true
			continue
		}
		lo, hi = min64(lo, l), max64(hi, h)
	}
	if !seen {
		return "int"
	}
	return fmt.Sprintf("int %d %d", lo, hi)
}

// buildLambda finds the `build` a source term yields, looking through the two
// things that can stand between them.
//
// A `let` is what call-by-need leaves when a buffer is used more than once, and
// a `set` hands the buffer back (ADR 0018). Neither changes which allocation the
// loop variable ends up holding, so neither may hide it: the fixed-limb library
// produces both shapes, and with them opaque a loop-carried buffer's sources
// disagreed on width again — `byte[]` against `int[]` on Java.
func buildLambda(tgt *Target, t *core.Term) (*core.Term, bool) {
	for i := 0; i < 8 && t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName; i++ {
		pr, ok := tgt.Prims[t.Op().Name]
		if !ok {
			return nil, false
		}
		args := t.Args()
		switch {
		case pr.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn:
			t = args[1].Closed()
		case pr.Kind == "table-set" && len(args) == 3:
			t = args[0]
		default:
			i = 8
		}
		if i == 8 {
			break
		}
	}
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return nil, false
	}
	if pr, ok := tgt.Prims[t.Op().Name]; !ok || pr.Kind != "table-build" {
		return nil, false
	}
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return nil, false
	}
	return args[1], true
}

// againTerms are the back edges of ONE loop: a nested loop's are its own, and
// two loops whose arities happened to match would be read as one.
func againTerms(tgt *Target, body *core.Term) []*core.Term {
	var out []*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KApp && x.Op().Kind == core.KName {
			if x.Op().Name == "again" {
				out = append(out, x)
				return
			}
			if pr, ok := tgt.Prims[x.Op().Name]; ok && pr.Kind == "iterate" {
				return
			}
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(body)
	return out
}

func ElemType(tgt *Target, lam, body *core.Term, name string,
	typeOf func(*core.Term) string, sig *core.Sig, params []string) string {
	return elemTypeFixed(tgt, lam, body, name, typeOf, sig, params, nil)
}

// elemTypeFixed is ElemType with LoopOneJoin's answer, when there is one. The
// join OVERRIDES, because a buffer's own stores are the right answer only when
// nothing else is assigned to the same variable.
func elemTypeFixed(tgt *Target, lam, body *core.Term, name string,
	typeOf func(*core.Term) string, sig *core.Sig, params []string,
	fix map[*core.Term]string) string {
	if ty, ok := fix[lam]; ok {
		return ty
	}
	if ty := bufferElem(body, name, typeOf); ty != "int" {
		return ty
	}
	// A BUFFER NOBODY WRITES TO TAKES ITS ELEMENT TYPE FROM THE HOST CALL IT IS
	// PASSED TO, because the host is the thing that writes it.
	//
	// `(build 64 (fn (b) (File.Read f b) ...))` is a scratch buffer handed to
	// `(*os.File).Read`, whose declared parameter is `(array (int 0 255))` -- and
	// with no `set` anywhere the syntactic inference correctly has nothing to say,
	// so the buffer came out `[]int` and Go refused the file. Found on the first
	// program to call a GENERATED `os` method.
	//
	// It is tried only where the stores decide nothing, and that is soundness
	// rather than an order of preference: a declaration is the most exact source
	// there is, but a buffer the program ALSO writes to must satisfy its own
	// stores, and narrowing it to what the host expects would truncate them
	// silently. Where the two disagree the host compiler says so, which is the
	// loud direction.
	if ty := declaredElem(tgt, body, name); ty != "" {
		return ty
	}
	// The analysis is asked WITH the enclosing precondition, because what
	// bounds a buffer's stores is usually something the signature says.
	if r, ok := BufferRange(tgt, lam, sig, params); ok {
		return r
	}
	return "int"
}

// declaredElem is the element type a PRIMITIVE declares for the position this
// buffer is passed in. See elemTypeFixed for when it is consulted and why not
// sooner.
//
// Both call shapes are walked, because a host call with several results is an
// application whose operator is an application -- `((File.Read f b) (fn (n e)
// ...))` -- which is the shape `multiPrimCall` recognises and the shape a
// fallible read has on every host.
func declaredElem(tgt *Target, body *core.Term, name string) string {
	found := ""
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t == nil || found != "" {
			return
		}
		if t.Kind == core.KApp {
			op, args := t.Op(), t.Args()
			if op.Kind == core.KApp && op.Op().Kind == core.KName {
				op, args = op.Op(), op.Args()
			}
			if op.Kind == core.KName {
				if p, ok := tgt.Prims[op.Name]; ok {
					for i, a := range args {
						if i >= len(p.Args) || BufferRoot(a) != name {
							continue
						}
						if elem := core.ArrayElem(p.Args[i]); elem != "" && elem != "int" {
							found = elem
							return
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
	return found
}

// closeImplements makes the declared edges a PREORDER: reflexive by
// `compatible`'s own equality, and transitive here.
//
// Transitivity is not decoration. `io.ReadCloser` embeds `io.Reader`, so a type
// declared to satisfy the first satisfies the second -- and a target file that
// had to spell out every consequence would be stating a closure by hand and
// getting it wrong. The relation is finite and small, so the closure is the
// obvious fixed point rather than anything clever.
//
// ANTISYMMETRIC RATHER THAN SYMMETRIC, which is the whole difference from
// `compatible`: a `*os.File` goes where an `io.Reader` is wanted and an
// `io.Reader` does not go where a `*os.File` is wanted. Subsumption FORGETS
// (every method but the interface's own), and forgetting has a direction.
func (tg *Target) closeImplements() {
	if len(tg.Implements) == 0 {
		return
	}
	for {
		grew := false
		for sub, ifs := range tg.Implements {
			have := map[string]bool{}
			for _, i := range ifs {
				have[i] = true
			}
			for _, i := range ifs {
				for _, up := range tg.Implements[i] {
					if up == sub || have[up] {
						continue
					}
					have[up] = true
					tg.Implements[sub] = append(tg.Implements[sub], up)
					grew = true
				}
			}
		}
		if !grew {
			break
		}
	}
	for sub, ifs := range tg.Implements {
		seen := map[string]bool{}
		out := ifs[:0]
		for _, i := range ifs {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
		sort.Strings(out)
		tg.Implements[sub] = out
	}
}

// Subsumes reports whether a value of type `got` may stand where `want` is
// declared. It is a LOOKUP: the relation is ground, finite and declared, so
// there is no inference, no variance and no fixed point to run here.
//
// The emitted text is unchanged either way -- see the Implements field. This
// answers a question the type checker asks and nothing else.
func (tg *Target) Subsumes(got, want string) bool {
	if tg == nil || got == "" || want == "" || got == want {
		return got == want && got != ""
	}
	// BY REALIZATION ON BOTH ENDS, because a type owned by its module means one
	// host type may have two keys — a hand file's `go/io.Writer` and a generated
	// `io-Writer` (SameHostType).
	for subj, ifs := range tg.Implements {
		if subj != got && !tg.SameHostType(subj, got) {
			continue
		}
		for _, i := range ifs {
			if i == want || tg.SameHostType(i, want) {
				return true
			}
		}
	}
	return false
}

// SameHostType reports whether two of OUR names denote ONE host type: they
// realize the same spelling.
//
// A TYPE'S IDENTITY IS ITS REALIZATION. The pool maps a name to the host's own
// spelling, and for an opaque host type that spelling IS the type — so two names
// realizing "io.Writer" name one thing however each is keyed. Nothing had to ask
// while one flat pool per target gave every host type exactly one name; a type
// owned by its module (theories.md §3.2) means a hand-written file and a
// generated one may key it differently, and a program that uses both must still
// pass a value from one to the other (ownedtypes-2026-09-16).
func (tg *Target) SameHostType(a, b string) bool {
	if tg == nil || a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	sa, oka := tg.Types[a]
	sb, okb := tg.Types[b]
	return oka && okb && sa != "" && sa == sb
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
	// A `let` IS TRANSPARENT HERE, and without this the rule above can never
	// fire. `(set b i (src k))` does not survive as written: effects.md §7c
	// refuses to substitute a table read into an impure body — a read moving
	// across a store is a silent wrong answer — so a COPIED BYTE is ALWAYS
	// let-bound, and what this walk sees is a bare name whose `typeOf` has
	// already normalised the range away.
	//
	// So the two rules are exactly complementary: §7c guarantees the binding
	// exists, and this resolves through it.
	binds := map[string]*core.Term{}
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t == nil {
			return
		}
		if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName &&
			t.Kids[0].Name == "let" && t.Kids[2].Kind == core.KFn &&
			len(t.Kids[2].Params) == 1 {
			walk(t.Kids[1])
			binds[t.Kids[2].Params[0]] = t.Kids[1]
			walk(t.Kids[2].Body())
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
			v := t.Kids[3]
			if v.Kind == core.KName {
				if b, bound := binds[v.Name]; bound {
					v = b
				}
			}
			// The circularity guard is `storedRange`'s `self` argument now, and
			// it had to move there: it was written for a BARE read, and
			// arithmetic can bury one — `(+ (b i) 1)` is an increment of a
			// slot, which this saw as an opaque application and now recurses
			// into. `buffer-swap` is the case that found the first version
			// (elemwidth-2026-08-27, pinned there with a control), and
			// examples/io/freq.oro's run-length counter is the buried one.
			if l, h, ok := storedRange(v, typeOf, name); ok {
				sawRange = true
				if l < lo {
					lo = l
				}
				if h > hi {
					hi = h
				}
			} else {
				sawOther = true
				if ty := typeOf(v); other == "" && ty != "" {
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
func PostVars(body *core.Term, raw []string, scope map[string]bool) map[int]*core.Term {
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
		// AND EVERY NAME IT MENTIONS MUST BE A LOOP VARIABLE.
		//
		// The post clause sits OUTSIDE every binder the body opens, so an update
		// that mentions anything bound inside it is undefined there. ADR 0015
		// permits `again` under a `let`, and decimal rendering uses that:
		//
		//	(let (/ v 100000000) (fn (nv) (again … nv …)))
		//
		// so `v`'s update is the name `nv`. It reads no other LOOP variable,
		// which is what the sibling guard tests, so it was hoisted and Go
		// refused the file with `undefined: nv44`.
		//
		// The rule is stated as a WHITELIST rather than as a search for inner
		// binders, because the search has to know which names are bound where
		// and that is exactly what a rebuilt term makes unreliable — the term
		// reaching here can carry a binder whose body was opened by an earlier
		// pass. Naming what is allowed needs no such knowledge: a loop variable
		// is in scope at the post clause and nothing else is guaranteed to be.
		// It costs the hoist for an update mentioning an outer constant, which
		// measured as no emitted file across the whole corpus.
		if readsOtherLoopVar(want, raw, i) || mentionsInnerBinder(want) ||
			mentionsOutOfScope(want, raw, scope) {
			continue
		}
		out[i] = want
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mentionsOutOfScope reports whether a term names anything that is not in scope
// at the post clause: this loop's own variables, or a name the emitter has
// already bound outside it.
//
// `bound` is the same set `soleExit` takes, and it is what "in scope" means
// here — an enclosing loop's variable is in it, and a name a binder INSIDE the
// body introduces is not.
func mentionsOutOfScope(t *core.Term, raw []string, scope map[string]bool) bool {
	ok := make(map[string]bool, len(scope)+len(raw))
	for n := range scope {
		ok[n] = true
	}
	for _, v := range raw {
		ok[v] = true
	}
	found := false
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil || found {
			return
		}
		if x.Kind == core.KName && !ok[x.Name] {
			found = true
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	// The OPERATOR of an application is a primitive's name, not a value, so it
	// is skipped: `(+ i 1)` mentions `+`, which is in scope everywhere.
	if t != nil && t.Kind == core.KApp {
		for _, a := range t.Args() {
			walk(a)
		}
		return found
	}
	walk(t)
	return found
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
		for _, p := range tg.spelled(want) {
			if len(p.Args) == 1 && p.Kind == "expr" {
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
	if lo, hi, ok := storedRange(t, func(*core.Term) string { return "" }, ""); ok {
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
	sig *core.Sig, params []string, fix map[*core.Term]string) int {
	ty := elemTypeFixed(tgt, lam, body, name, func(t *core.Term) string {
		if IsBoolTerm(tgt, t) {
			return "bool"
		}
		return ""
	}, sig, params, fix)
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

// LOOP-CARRIED BUFFER REUSE: a `build` on a back edge writes into storage the
// loop already owns, instead of allocating a fresh table every iteration.
//
// This is the fourth place the same gap has been named — Karatsuba's workspace,
// ADR 0013's stencil, the mutable bignum, and the fixed-limb rung, where 199
// iterations of a factorial cost 199 allocations and that is most of what
// biglimb-2026-09-02 measured. The first three are about a boundary; this one is
// about a back edge, and a back edge is where ADR 0015 already gives the answer.
//
// THE CONDITIONS ARE RULE R's, ONE LEVEL UP. `emit/bigreuse.go` asks when a
// bignum operation may write into a loop variable's object; this asks the same
// of a table:
//
//	(1) the `again` argument for vⱼ is a `build` of constant length;
//	(2) vⱼ's initialiser is a `build` of the SAME length, so the two are
//	    interchangeable storage — and it allocates in this function, so nothing
//	    outside owns it;
//	(3) EVERY occurrence of vⱼ in the whole `again` is inside that argument;
//	(4) the loop has exactly one back edge, so there is one story about which
//	    buffer is live.
//
// TWO BUFFERS AND A SWAP, NOT ONE BUFFER IN PLACE. Writing the new value into
// the old one would alias the reads the body makes of vⱼ, and whether that is
// safe depends on the ALGORITHM: a limb multiply reads `a[i]` after writing
// `o[i+j]`, and in place that is a wrong answer. Alternating two buffers needs
// no aliasing argument at all — the body always reads one and writes the other —
// and the buffer it writes was last read two iterations ago and is dead.
//
// The spare is CLEARED on entry, because `build` zero-fills (tables.md §14.3)
// and a program may rely on it — `mul` accumulates into slots it has not
// written. A clear is a memset where an allocation is a heap operation and a
// collection.
func LoopBufferReuse(tgt *Target, inits []*core.Term, openBody *core.Term,
	raw []string) map[int]*core.Term {
	agains := againTerms(tgt, openBody)
	if len(agains) != 1 {
		return nil // (4)
	}
	as := agains[0].Args()
	out := map[int]*core.Term{}
	for j := range raw {
		if j >= len(as) || j >= len(inits) {
			continue
		}
		lam, n, ok := constBuild(tgt, as[j]) // (1)
		if !ok {
			continue
		}
		if _, m, ok := constBuild(tgt, inits[j]); !ok || m != n { // (2)
			continue
		}
		total := 0
		for _, a := range as {
			total += countName(a, raw[j])
		}
		if total != countName(as[j], raw[j]) || total == 0 { // (3)
			continue
		}
		out[j] = lam
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// constBuild reports the lambda and length of a `build` whose length is a
// literal, looking through the `let` call-by-need leaves and the `set` that
// hands a buffer back — the same two shapes buildLambda sees through.
func constBuild(tgt *Target, t *core.Term) (*core.Term, int64, bool) {
	lam, ok := buildLambda(tgt, t)
	if !ok {
		return nil, 0, false
	}
	// buildLambda followed the wrappers; the length lives on the build itself,
	// so find it the same way.
	for i := 0; i < 8 && t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName; i++ {
		pr, known := tgt.Prims[t.Op().Name]
		if !known {
			return nil, 0, false
		}
		args := t.Args()
		if pr.Kind == "table-build" && len(args) == 2 {
			if args[0].Kind != core.KInt {
				return nil, 0, false
			}
			return lam, args[0].Int, true
		}
		switch {
		case pr.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn:
			t = args[1].Closed()
		case pr.Kind == "table-set" && len(args) == 3:
			t = args[0]
		default:
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// buildLen is a `build`'s literal length, or 0.
func buildLen(tgt *Target, t *core.Term) int64 {
	if _, n, ok := constBuild(tgt, t); ok {
		return n
	}
	return 0
}

// ShiftNames are this target's own spellings of a right shift and a bitwise
// and, found the way `=` and `+` are found — by spelling, at the right arity,
// so a target says how it does it in its own file.
//
// They are NOT promoted to the language and this does not make them portable:
// integers.md §0a keeps bitwise operators out for a measured reason, and that
// reason stands. What this does is let the COMPILER use a host's own shift
// where it has proved the rewrite changes no answer, which is the parasite rule
// (`emit at the highest layer the target natively provides`) applied to an
// operation a program may not write.
func (tg *Target) ShiftNames() (shr, and string, ok bool) {
	find := func(spellings ...string) (string, bool) {
		for _, want := range spellings {
			// `shr` before `sar` on x86: both are correct on a non-negative
			// value and the logical one says so.
			for _, p := range tg.spelled(want) {
				if len(p.Args) == 2 && p.Kind == "expr" &&
					p.Args[0] != "bool" && p.Args[1] != "bool" {
					return p.Name, true
				}
			}
		}
		return "", false
	}
	shr, okS := find(">>", "shr", "sar")
	and, okA := find("&", "and")
	return shr, and, okS && okA
}

// LiteralElem is the element type of a table written as its GRAPH —
// `(array e₁ … eₙ)` — and it is the JOIN of the elements' exact ranges
// (docs/literal-elements.md).
//
//	V  =  ⨆_{i<n} [eᵢ, eᵢ]  =  [min eᵢ, max eᵢ]
//
// SOUND BY CONSTRUCTION, and for the reason elemwidth-2026-08-27 already gives:
// a literal is its own exact range, so every element is inside the hull. That is
// the opposite of a buffer's hazard, where the danger is a later `set` the
// inference did not see — a literal table is an immutable value and has no later
// store at all (ADR 0018).
//
// Unlike a buffer's, the hull does NOT start at zero: `build` zero-fills, so 0 is
// always one of a buffer's elements, and a graph holds exactly what is written.
//
// Anything that is not an exact integer — a float, a string, a computed element —
// falls back to the first element's type, which is what this was before.
func LiteralElem(t *core.Term, typeOf func(*core.Term) string) string {
	elems := t.Args()
	if len(elems) == 0 {
		return "int"
	}
	lo, hi, ok := int64(0), int64(0), true
	for i, x := range elems {
		l, h, exact := storedRange(x, typeOf, "")
		if !exact {
			ok = false
			break
		}
		if i == 0 || l < lo {
			lo = l
		}
		if i == 0 || h > hi {
			hi = h
		}
	}
	if !ok {
		return typeOf(elems[0])
	}
	return fmt.Sprintf("int %d %d", lo, hi)
}

// LiteralFits reports whether a literal table's elements all lie inside a
// DECLARED element type, which is the checking half of docs/literal-elements.md
// §3: at a boundary the declaration decides the representation, and the literal
// must fit it. A literal that does not fit is ours to refuse, naming the element
// — before this, the Go compiler refused it with a type it never wrote.
func LiteralFits(elem string, t *core.Term, typeOf func(*core.Term) string) (string, bool) {
	lo, hi, ok := core.IntRange(elem)
	if !ok {
		return "", true
	}
	for _, x := range t.Args() {
		l, h, exact := storedRange(x, typeOf, "")
		if !exact {
			return "", true // nothing exact to check against
		}
		if l < lo || h > hi {
			return fmt.Sprintf("%d", l), false
		}
	}
	return "", true
}
