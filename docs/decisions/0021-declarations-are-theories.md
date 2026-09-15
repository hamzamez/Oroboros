# 0021 — Declarations are theories, and the surface that follows

Date: 2026-09-15
Status: Accepted — **specified, not built**. Amends the reasoning of
[0011](0011-modules-add-nothing-to-the-reducer.md) (its *Why not — Functors*), not its decision.

Research: [declaration-surface.md](../declaration-surface.md), [theories.md](../theories.md),
[theories-b-or-c.md](../theories-b-or-c.md), [facts.md](../facts.md).
Specifications: [spec/theories.md](../spec/theories.md), [spec/data.md](../spec/data.md).

## Context

Writing Go's standard library by hand (ADR 0022) made the declaration surface the thing every later
package would be written in, so its problems stopped being cosmetic. Measured or found in code:

- **Fourteen declaration forms with nothing relating them**: `prim`, `type`, `sig`, `def`, `sum`,
  `implements`, `provides`, `int-repr`, `big-repr`, `max-len`, `shift-width`, the injected set, and
  axioms written in Go. Each was added for a good reason, and each new one was argued from scratch.
- **A flat type pool per target.** Hand files prefix types by package (`io-Reader`,
  `hex-InvalidByteError`), and the generator does the same by a package's *last* path segment. **3 of
  Go's 1,270 exported type names collide that way**: `template.Template` and `scanner.Scanner` are each
  two distinct structs merged into one, and `template.FuncMap` is a real alias merged correctly by
  accident.
- **`/` meant two things**: package nesting, and a type's method set. Only capitalisation separated
  them.
- **A constant was a primitive of no arguments**, so its value was unusable as a range endpoint.
- **`tally` built a functor argument by hand**: an untyped tuple of six host operations, one per host,
  in two binding files.
- **`boxed` and `builtin-map` were parsed and specified nowhere**, and `builtin-map` composed across
  layers by *join* where layers need *override*.
- **Sums lived in one global table by bare name.** Two modules declaring `result` with different
  payloads loaded, and a signature's payload type depended on load order: `(int, string)` in one
  order, `(int, int)` in the other, accepted by our checker both times (core/sumclash_test.go, fixed in
  the interim).
- **`(array int)` could not tell a table of ints from a product of one int.** products.md's
  `(array int string)` in a result position was read as three results.
- **The compiler's axioms were scattered.** Of the 41 rules the analysis layer uses, 12 are facts, and
  the remainder bound alone is written three times in three encodings: the shape of every recorded
  *"two layers, and only one was ever told"* (facts.md §1).
- **ADR 0011 refused functors** because *"our parameterisation is the target, and it is already the
  parameter to reduction. A functor would be a second, competing mechanism."*

## Decision

**A module is a theory, a target is a model of theories, and every declaration form is one record.**
The rules are the two specifications; this ADR records what they decide.

1. **One declaration record.** Every form elaborates to `x : A [= t] [↦ ρ]` — a name, a classifier, an
   optional definition, an optional host realization — at the **type**, **term** or **proposition**
   level. Definition × realization gives modules.md's four cells at every level:
   - an alias is a type with a definition;
   - a host type is a type with a realization;
   - a constant has both;
   - native-wins (R2) applies to all of them.
2. **Modules are theories.**
   - A library file's path is its module, with no header.
   - Modules nest, with types, terms and propositions in separate namespaces.
   - A child module named like a type in its parent is that type's **companion**, which holds its
     methods.
   - Names resolve lexically.
   - **Type names are resolved like term names**, so `go/io.Writer` and a different `Writer` are
     different types.
3. **Targets are models.**
   - A target is a backend plus realizations.
   - **`host` is the only clause that carries host text**, so a file with no `host` clause is portable
     by a check rather than by convention.
   - `provides` may hold **definitions** (a target's own library).
   - `repr` states representation choices: integer ranges, `(ref T)` (formerly `boxed`),
     `(repr map library)` (formerly `builtin-map`, now composed by override), `big`, `shift` and
     `narrow`.
   - `lang` is closed.
4. **Composition** is target-system.md's glue, override and reduct, for declarations of every level.
5. **Views.**
   - `implements` is checked from declarations, by method-set inclusion with variance.
   - `include` inside a companion **derives** subtyping.
   - **Instantiation along a named view (C1) and views as named objects (C2) are specified and
     reserved**: `(use M with V)` and `(view …)` are refused by name until built. Instantiation along
     the target's ambient view is what a build already does.
6. **Data is functions on domains.**
   - Six forms: `fn`, `array`, `map`, `tuple`, `record`, `variant`.
   - **`sum` is spelled `variant`, and `values` and the product `(array A B)` are spelled `tuple`.** The
     old spellings are refused with the new one in the message.
   - Records are **structural**, lowered in canonical label order, and updated with `with`, which obeys
     the lens laws.
   - Labels are **symbols**, `'x`.
   - Variants are nominal in their declaration and applicative in their arguments, and **type arguments
     are never inferred**.
   - A variant at a boundary is **one slot per distinct payload type** (Theorem R), with no width
     bounded by any target.
7. **Facts are F-B**: local boundedness facts, instantiated on terms already present, one declaration
   feeding the linear layer, the interval layer and element narrowing. **F-C, F-D and F-E are reserved
   fragments**, and `forall` and `lemma` are reserved words, so building one later adds a fragment and
   never loosens F-B. Every fact about `lang` has a test that fails when the fact is deleted.
8. **Diagnostics are part of the specification.**
   - No silent acceptance.
   - A declaration error fires at load.
   - Every declaration carries its origin.
   - Names are printed in the user's spelling, which is always possible because resolution is an
     injective renaming.
   - A failed proof names its premises.

   A table of 43 rows says what each refusal must name, and seven legitimate cases must be accepted.
9. **The first build is an elaborating loader.** It turns the new forms into today's structures, and it
   passes only if **every emitted file is byte-identical**, **every per-program proof count is
   identical**, and every diagnostics row has its test.

### The amendment to ADR 0011

**0011's decision stands**: modules are resolution, not reduction, and elaboration still hands the
reducer one flat qualified namespace.

**Its reason for refusing functors was wrong.** A build already *is* the instantiation of a
parameterised theory: the program is parameterised by the theory its target realizes, and building is
parameter passing along the target's view (Ehrig & Mahr 1985; Goguen 1984). The target was never a
competitor to functors; it was the one functor argument the language had, unnamed. `tally` built a
second argument by hand. **Functors are deferred for lack of demand** (C1's explicit form, beyond the
ambient view) **and for what C3–C6 cost** (below), not because they compete with the target. 0011 is not
edited; this ADR is where the correction is recorded.

## Why not

**Candidate A — types in modules and nesting, with the forms otherwise kept.** It fixes the measured
problems (prefixes, merged types, re-declared types) and leaves fourteen unrelated forms, so every future
form — `const`, a record, an interface, a fact — is designed from nothing. The rewrite A saves is
mechanical; the design work it defers is not.

**Full C — structured specification with every feature.** Priced one feature at a time
(theories-b-or-c.md §1):
- **C3, sharing constraints:** no theory takes two parameters, and host types named by path are already
  equal without a constraint;
- **C4, higher-order functors:** no demand, and typing them is where module systems become research
  projects;
- **C5, opaque sealing of a user type:** needs a **typed static level**, because representation
  independence is a theorem about a typed language, and reduction erases the abstraction before the
  residual checker runs. Host types need no sealing: they have no definition to leak;
- **C6, first-class modules:** a module chosen at run time is a dictionary, which is a closure (callbacks.md
  tier 3). Refused permanently.

**Candidate Z — modules as static values** (1ML, Rossberg 2015; Zig). It deletes the module language,
because the static level is already an untyped λ-calculus. It is refused on the principle kept throughout
this ADR: **declarations are data, definitions are terms.** Every host check, every glue and override,
every survey and `TestHandDeclarationsAgreeWithTheHost` reads declarations without running a program. In
Z, knowing what a module declares means reducing it, and that reverses ADR 0006 as well as 0011.

**The capitalisation convention, or a second separator,** to tell a package from a type's methods. The
first is a host convention a future host may lack. The second is a new token for a distinction the
companion rule makes checkable: a child named like a type is its companion, and any other clash is an
error.

**Keywords from the literature: `data` and `codata`.** `codata` means *infinite* (Hagino, Charity, Idris),
and this language has no recursive types. **`sum`** reads as arithmetic, **`union`** is untagged in C and
TypeScript, **`enum`** means integer constants in C, Java, Go and every Win32 header, and **`choice` or
`oneof`** belong to F# and protobuf. `record` and `variant` are the textbook pair (Pierce, *TAPL* §11).

**Nominal records, as in F#.** F# needs them so type inference can tell which record a field belongs to.
Our residual is monomorphic, so the question never arises, and structural records are what TLA+ has.

**`:x` for labels.** `:x` is a *run-time* value in Clojure, Elixir and Ruby, and a label never survives
staging. `'x` is quotation, the use–mention distinction a label is. Neither character was legal in
identifiers, so meaning decided it.

**Keeping `values` beside a tuple type.** values.md's *"not a tuple"* meant *not a heap-allocated
tuple*, and products.md later settled that representation is chosen per position, not by syntax. Two
names for one product is the shape `merge` had before target-system.md separated its two operations.

**One slot per constructor at a boundary.** Sound, and wasteful. Two constructors with one payload type
can share a slot because the tag tells them apart, so `(result int int)` is `(tag, int)`.

**Letting a target bound a variant's or tuple's width.** It would make a language construct declinable,
which ADR 0017 and values.md both refuse: the compiler finds each target's convention, and a backend
without one reports a compiler limitation.

**Inferring type arguments.** Never needed. Inside a program they are erased; at a boundary the signature
writes them.

**Keeping the axioms in Go, consolidated (facts candidate F-A).** It ends the scattering and keeps
language facts out of data, against this project's own rule, and leaves host laws nowhere to go but
`ensures`. **User-supplied proof or rewrite rules (F-F)** extend the logic, not the theory, and bring
Shen's search into the type checker.

**Positions on every term now.** Declaration origins are cheap, since declarations are data. Term
positions touch the same term-rebuilding code five recorded bugs came from, so they are their own
decision.

## Consequences

**Easy.**
- Every future declaration form is a choice of level, cell and polarity, not a design.
- The laws of composition and renaming cover every form at once.
- `tally` becomes one program when `provides` holds definitions.
- The Win32 family (target-system.md §5.4) is two models of one theory.
- A misspelled `provides`, which today silently selects the portable definition, is a load-time error.

**Hard.**
- **Type names enter name resolution**, a reader and loader change.
- **`'` becomes a reader token** and `quote` an injected name.
- **The rewrite is mechanical and wide**: 931 primitive declarations in 37 files, 6,015 lines in 119
  programs, and the generators' output, which is regenerated rather than rewritten.
- **Structural records put canonical order on every lowering**, and a record-order conformance case must
  be able to catch an emitter that uses written order.

**Commits us to:**
- the elaborating loader as the first build, with byte-identical emission **and** identical proof
  counts as the bar. That is also the test of the central claim, that resolution against a target and
  instantiation along its view name everything the same way;
- **removing the interim sum rule** (two different sums with one name are refused) once type names are
  resolved;
- **reserved words**: `forall`, `lemma`, `view`, `quote`, and `with` outside `use`;
- a **test per diagnostics row**, and per legitimate case.

**Triggers to revisit:**
- a program that needs a view other than the ambient one: build C1 and C2;
- a portable library whose invariants its clients can break: sealing, and a typed static level;
- a demand for relational facts (F-C), quantified facts over arrays (F-D, which is tree.oro's `d` gap),
  or laws proved by unfolding (F-E);
- recursive data, which would be a **new class of declaration** allowed to break the well-founded order
  refusing μ and recursion today — an addition, not a relaxation;
- the niche encoding, to be specified from `(repr (ref int) (host "Long"))`, its first working instance.
