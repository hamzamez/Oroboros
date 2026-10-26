# Architecture Decision Records

One decision per file, numbered, never deleted. When a decision is reversed, write a new ADR
that supersedes the old one and mark the old one `Superseded by NNNN` — do not edit history.

This exists because this project gets put down and picked up again. The value of an ADR is
almost entirely in the **Why not** section: six months from now, the decision itself will be
visible in the code, but the rejected alternatives and their reasons will not.

## Template

```markdown
# NNNN — Title

Date: YYYY-MM-DD
Status: Accepted | Superseded by NNNN | Reversed

## Context
What forced a decision.

## Decision
What was decided, stated flatly.

## Why not
The alternatives, and what specifically ruled each one out.

## Consequences
What this makes easy, what it makes hard, and what it commits us to.
```

## Index

| # | Decision |
|---|---|
| [0001](0001-parasite-model.md) | Targets are ecosystems; portability is a program property |
| [0002](0002-capability-graph.md) | A capability graph replaces the fixed layer tower |
| [0003](0003-range-typed-integers.md) | Range-typed integers with mathematical semantics |
| [0004](0004-first-targets.md) | Go, JavaScript, and Java/Android first; C deferred |
| [0005](0005-implementation-language.md) | The compiler is written in Go |
| [0006](0006-ir-file-format.md) | The backend interface is a file format |
| [0007](0007-exploration-over-specification.md) | Explore candidates against a fixed test, don't specify the core first |
| [0008](0008-measurement-over-principle.md) | Parasite decisions are per-target measurements, not principles |
| [0009](0009-staging-preserves-results.md) | Staging must not change results |
| [0010](0010-effects-as-structural-rules.md) | Effects are a side condition on β, not a feature |
| [0011](0011-modules-add-nothing-to-the-reducer.md) | Modules are resolution, not reduction |
| [0012](0012-portable-integer-range.md) | `int` is exact within ±(2⁵³−1) — partly superseded by 0026 |
| [0013](0013-accept-the-allocation-price.md) | Accept the allocation price, provisionally |
| [0014](0014-recursion-is-not-in-the-language.md) | Recursion is not in the language |
| [0015](0015-loop-and-again.md) | `loop`/`again` — guarded clauses over n variables |
| [0016](0016-targets-need-not-have-expressions.md) | A target need not be an expression language |
| [0017](0017-booleans-are-in-the-language.md) | Booleans and control flow are in the language |
| [0018](0018-immutable-values-linear-buffers.md) | Immutable values, one scoped linear buffer |
| [0019](0019-precision-by-declaration.md) | Precision by declaration |
| [0020](0020-uniqueness-on-parameters.md) | A buffer is a nameable type: uniqueness on parameters |
| [0021](0021-declarations-are-theories.md) | Declarations are theories, and the surface that follows |
| [0022](0022-host-declarations-are-written-by-hand.md) | Host declarations are written by hand; the generator is their checker |
| [0023](0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) | A generator does not make a claim it cannot justify |
| [0024](0024-comments-are-erased.md) | A comment never carries meaning; documentation is a term |
| [0025](0025-a-module-path-is-the-hosts.md) | A module path is the host's path, and the file is the path |
| [0026](0026-an-int-is-an-integer.md) | An `int` is an integer, and each target realizes what it can |
| [0027](0027-a-host-calls-continuation-is-a-tail.md) | A host call's continuation is a tail position; `again` may sit under a tuple binding |
| [0028](0028-a-definitions-contract-is-checked-at-its-calls.md) | A definition's declared parameter range and `where` are obligations at its calls |
| [0029](0029-above-the-word-one-set-on-every-representation.md) | Above the word, a type denotes a set decided by sign and bit length, and every representation enforces the program's one set |
| [0030](0030-a-hosts-string-is-the-hosts.md) | At a host boundary a string is the host's; ours is Σ*, entered by one total decode |
| [0031](0031-a-builds-result-is-a-product.md) | A `build`'s result is a product, and each buffer in it is frozen |
| [0032](0032-the-ir-is-structured-ssa.md) | The IR is structured SSA with π-parameters, and representation is a type |
| [0033](0033-an-integers-representation-is-chosen-on-the-ir.md) | An integer's representation is chosen on the IR; the checker's integer sort is ℤ |
| [0034](0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md) | The rung above the word is a least fixed point on the IR, and a bignum destination needs ownership |
| [0035](0035-a-compiler-library-is-a-theory-checked-once.md) | A compiler library is a theory checked once; a use is an instance, inlined on the IR |
| [0036](0036-a-residuals-binders-are-named-apart.md) | A residual's binders are named apart along the scope chain |
| [0037](0037-a-programs-obligations-are-in-its-residual.md) | A program's obligations are in its residual, and nothing reads one undecided |
| [0038](0038-the-irs-text-is-read-not-written.md) | The IR's text is the printers' whole input (IR_P, checked on every program); others may read it, and nobody else writes it until the IR proves index obligations |
| [0039](0039-the-targets-are-go-windows-android-and-the-browser.md) | The targets are Go, Windows, Android and the browser; Node is a third-party layer; supersedes 0004's list |
| [0040](0040-a-failure-is-a-value-of-a-marked-sum.md) | A failure is a value of a marked sum: the compiler knows `success` and `relevant`, `try` is bind, the host's encoding is a niche, `expect` is the one way to give up |
| [0041](0041-a-success-arm-is-a-tail.md) | A success arm is a tail: `again` may sit under `try` and `expect`; a constructor is the tuple (tag, payload) — extends 0015 |
| [0042](0042-a-variants-value-is-its-tag-and-its-slots.md) | A variant's value is its tag and one slot per payload type of its declaration, everywhere it must exist as data; an unselected slot holds `#any`, the zero of its type |
| [0043](0043-relevance-is-checked-where-a-binder-meets-its-value.md) | Relevance is checked at β, where a binder meets its value, on every path (the eliminators are additive), with usage inference through definitions; `ignore` is !_A : A → 1, and η extends to the unit |
| [0044](0044-a-partial-successs-error-is-relevant.md) | A partial success's error is the relevant 1 + E, `(result (tuple) E)`; `(option error)` stays only where the host makes the error ignorable (the builder's writes, the sticky writer's), with the reason stated |
| [0045](0045-a-hosts-empty-interface-is-a-type.md) | A host's empty interface is a type every one-value type is below, derived from its empty method set; a variadic over it is a table of boxes, spread; `fmt` declares one value under Go's name and any number as NAME-All |
| [0046](0046-a-variadic-call-is-its-declaration-applied-to-a-word.md) | A variadic is one declaration under the host's name, f : X × T* → B; a call's trailing arguments are the list, `(spread xs)` passes a table; rewritten by the loader from the target's forms; a variadic statement's value is the unit; the printer writes the host's own call; supersedes 0045's NAME-All |
| [0049](0049-types-are-sets-and-the-logic-never-prints.md) | Types are sets of values and the logic that reasons about the computation never prints; a representation chosen from proven sets is the one bridge; the decided fragment (Presburger, regular strings, semantic subtyping, the array property fragment, BAPA, congruence closure) is the ceiling of automation; own complete procedures, no SMT; extension by definitions and proven lemmas only; types are static values |
| [0048](0048-a-host-writes-a-cell-through-a-reference.md) | A host writes a cell through a reference to a fresh copy, `#ref`/`#deref` around the call, an ordinary argument through the retraction, the variadic list and the box; `(cell T)` in declarations, `(cell go.Value)` a cell of any type in Go's box; `(out τ)` a cell for one call, its value the cells' then the result's; the IR's `cell T`; extends 0047 |
| [0050](0050-the-abstract-syntax-fits-the-semantics.md) | The abstract syntax is the initial algebra of the semantics (spec/abstract.md): usage is a mode on a binder, one definition carries its claims, sets are formed explicitly, pairs are pairs with one guarded choice, data application is marked; the meanings are homomorphisms and claims are inert by construction; the surface stays open |
| [0051](0051-a-definition-is-a-named-lambda.md) | A definition is a named λ, and the λ carries its claims: a set per binder with φ and ψ, meaning P and R; one result binder whose pattern may be a tuple; program declarations first, target files in step 1c; a λ applied by hand is no binding for a jump |
| [0052](0052-a-buffer-is-a-state-binder-holding-a-table.md) | A buffer is a state binder holding a table, written in program order (`b[i] := v`); the ordering check becomes R0–R4 (freshness, then syntactic control of interference); a borrowed buffer parameter is passed by reference; the write-borrow wrapper stays; not built |
| [0053](0053-the-arrow-is-a-claim-on-a-host-parameter.md) | The arrow is a claim used as a set, on a host's callback parameter or as a named claim: a function space Π(P).R with markers, not an operation of the type algebra; a returned host function is a host type with application; not built |
| [0054](0054-let-and-local-are-one-scope.md) | `let` and `local` are one scope, the free monoid of binder words acting on expressions, whose binders state their kind; a definitional name may be rebound, a state name is bound once in its region; the head is `let`; not built |
| [0055](0055-a-loop-variable-is-bound-by-application.md) | A loop variable is bound by application, not assigned: one iteration is a `let` (the unfolding law), so a loop's binders are definitional, written `=` and `==`; `:=` is only a place taking a value; `again` is an application; not built |
| [0056](0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md) | The source notation is D or J (B′ and K dropped); one arm grammar, `cond` and `match` retired; `again` positional; `;` tighter than `[]` in D; `var` in J; J's collection formers, `array(n, f)`, `Seq(S)`; not built |
| [0057](0057-the-handled-bind-and-its-tail.md) | The handled bind, bind(p; e; ā; b) = case(e; p → b, ā), is a derived operator of which try and expect are instances; J `try p = e [else ā], … { body } [else ā]`, D `TRY … IN … [ELSE …]`; its success arm is a tail in a loop and its handler leaves (extends 0041); not built |
| [0058](0058-a-map-buffer-is-a-state-binder-holding-a-map.md) | A map buffer is a state binder holding a map, written in program order (`m[k] := v`); R0–R4 as for a table; extends 0052; not built |
| [0059](0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md) | A tag is a quoted symbol, `'name`; case carries no category; a collection has one former, `former { … }`, and `array(n, f)` goes; the sets are `Array(V)` and `Map(K, V)`; supersedes 0056's items 8 and 9; not built |
| [0060](0060-j-set-operators-and-qualified-tags.md) | J's membership is `∈` and its set operations Python's (`\| & - ^ <= < *`), D's TLA+'s; an imported tag is qualified, `os.'not_found`, a module's own and the language's unqualified, no `exposing`; not built |
| [0061](0061-separation-and-replacement-by-the-binders-place.md) | Inside a former, separation (`set { x ∈ S : P }`) and replacement (`set { e : x ∈ S }`, `array { e : k ∈ 0..n-1 }`) are told apart by the binder's place, as in TLA+; a map takes a graph only; the logic gains the image of a set; not built |
| [0062](0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md) | A header is the membership statement f ∈ Π(P).R, its result a binder; a bound is the binder's set (a telescope), a relation stays in `where`; one equality in formulas, the logic's; quantifiers and implication only in claim positions; `fn`, `spec` and `lemma` share the header, a lemma used by `assert L(…)`, a recursive one an induction; `assert` proves, `expect` checks; `the` leaves the surface; no claim is not `any`; not built |
| [0063](0063-a-type-is-a-static-value-and-instantiation-is-application.md) | A type is a static value: one namespace per module, a struct literal its companion's `new`; a declaration's head binds static parameters `(E (subsets T))`; instantiation is application reduced by δ, the bound membership by subsumption, an instance identified by its name; `(comma-ok)`; built for `errors.AsType` |
| [0064](0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md) | A file is a theory named by its path, a target file a set of fragments, the target its directory; an import carries its alias; visibility public, `private` hiding, `export` an entry point only; a declaration is a header and one body, an expression or a host; `let` off the top level, `set S = e` an optional claim; a variant a union of tagged sets; an inclusion an axiom of a model; a companion a nested module; not built |
| [0065](0065-j-writes-sets-in-tla-symbols.md) | J writes sets in TLA+'s symbols `∈ ∉ ∪ ∩ ∖ ⊆ ⊂ ×`, TLA+'s names their ASCII, the file storing the symbol; short input is the editor's, never a string escape; a set former keeps its keyword and a headless brace is a block; supersedes 0060's item 2; not built |
| [0066](0066-a-definition-over-sets-is-a-lambda-returning-a-lambda.md) | A definition over sets is a λ returning a λ, `E ⊆ S` binding a set; static arguments inferred by matching sorts; `~[]E` collapses; the arrow a λ's header, its domain a telescope; a site λ checked under the parameter's claim; no receiver form; `x'` in `ensure`; a host template quoted code with named splices `${x}`, a value of Code_T = text × requirements; not built |
| [0067](0067-the-lexical-level.md) | The lexical level: ASCII words with a final `?`, `-` as `_`, backquoted host names; few reserved words, clause words contextual; case meaningless, sets UpperCamelCase by convention; `^` on constants; line comments carried; Scala 3's line rule; J's logic in TLA+'s symbols; one precedence table; prime and tag by adjacency; not built |
| [0068](0068-j-is-the-surface.md) | J is the surface, one rendering; supersedes 0056's "D or J"; the experiment not run; not built |
| [0069](0069-an-observer-at-a-host-parameter.md) | An observer at a host parameter: a pure declaration with scalar results reads a live buffer for the length of the call, derived not declared; §7c counts it; a segment is a function composed with a restriction; a binder's claim `b ∈ S = e` is an obligation, and `the` is not J; built |
| [0070](0070-a-header-is-the-callers-view-of-a-borrow.md) | A host declaration's header is the caller's view of a borrow: the threaded result derived; a primed result `p'` names storage handed back, its absence a write-borrow in place; the template the host's own call, the wrapper gone; amends 0052 item 4; built |
| [0071](0071-work-stays-where-it-was-written.md) | Work stays where it was written: β counts evaluations in {0, 1, ω}, not occurrences; let-floating exposes a value; a residual let takes β's rule; the IR reads a measured length as the length; built |
| [0072](0072-a-declared-result-is-a-claim-decided-at-its-definition.md) | A declared result is a claim decided at its definition, exported or not, called or not; a product's set factor by factor; the words only; a definition over a function refused with its reason; built |
| [0073](0073-a-sets-name-denotes-a-set.md) | A set's name denotes a set: a header's atoms are scope-checked against the language's sets and the target's types; an alias resolves as a term's does; built |
| [0074](0074-a-segment-is-its-function-restricted-wherever-its-table-is.md) | A segment is its function restricted wherever its table is, a writer's suffix included; one check for every segment; built |
| [0075](0075-a-wheres-constant-bounds-are-the-parameters-range.md) | A host `where`'s constant bounds on one parameter are its range obligation, decided on the IR's intervals too; built |
| [0076](0076-a-case-in-a-tail-is-a-tail-when-its-arms-are.md) | A case in a tail is a tail when each arm is: the tail sort closed under a sum's eliminator; built |
| [0077](0077-an-unbounded-end-is-omitted.md) | An unbounded end is omitted, `lo..` and `..hi`; `` `+inf` `` refused; `Nat` waits until a declared set stops being a representation request; built |
| [0078](0078-a-modules-interface-is-what-it-marks-pub.md) | A module's interface is what it marks `pub`, private the default; `export` an entry point only, a second relation; supersedes 0064 item 4; built |
| [0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) | A declared set is a claim, and the facts choose the representation; the consent to the rung above stays a claim's demand; supply only at a boundary; `Nat` follows; amends 0019 and 0034; built |
| [0080](0080-the-native-targets-print-llvm-ir.md) | The native targets print LLVM IR, stating the IR's proofs (`nsw`, `nuw`, `inbounds`, `select`); MASM retired at parity; C not chosen; 32-bit a triple; not built |
| [0047](0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md) | `local` is a scope of local state: the initializer decides buffer (a table) or cell (anything else); cells are loop variables by the state-passing translation in the loader; an elimination it assumes is marked and decided by the reducer on a normal form; a projection of a loop moves into its exits; generalizes 0018's scope and respells `build`'s binder |
