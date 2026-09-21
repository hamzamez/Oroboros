# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

**This file describes the project's current STATE, not its history.** Update the state when it
changes. Don't append a narrative: each result has its own document in
[gauntlet/results/](gauntlet/results/), and each round has an assessment in [docs/](docs/). Until
2026-09-17 this file was a changelog of about 60,000 words; that version is in git at `73004de`.

## What this project is

Oroboros is a small language and build system that **parasitizes** target ecosystems rather than
abstracting over them. Portability is a property a program may or may not have, and the compiler
computes it; it is not a global guarantee ([ADR 0001](docs/decisions/0001-parasite-model.md)).

The single mechanism is a **capability graph**:
- modules declare the capabilities they require;
- targets declare the capabilities they provide;
- building means covering the first with the second ([ADR 0002](docs/decisions/0002-capability-graph.md)).

The governing rule is **emit at the highest layer the target natively provides**, and lower only as
far as necessary.

It is a **general-purpose language** ([general-purpose.md](docs/general-purpose.md)): apps on Windows
and Android, websites, cloud backends. The four targets are application platforms.

**The atom** ([the-atom.md](docs/the-atom.md)) is **lambda calculus in which the normal form is a
parameter**:
- a target partitions names into primitive and defined;
- reduction runs until only primitives remain.

It is a **two-level language** ([closures-direction.md](docs/closures-direction.md)). The static
level is unrestricted higher-order and is erased by staging. The dynamic level is first-order tables
and loops.

## Where it stands

As of 2026-09-17. The current assessment is [assessment-2026-09-17.md](docs/assessment-2026-09-17.md);
read it before planning.

**The compiler.**
- A β/δ reducer with call-by-need and an effect discipline.
- A type checker on the residual, a refinement layer, an interval analysis and size-change
  termination.
- Four backends: **Go, JavaScript, Java and x86-64** (the `windows` target, MASM).
- `go run ./cmd/check` passes every step: vet, compiler, emission, differential and tooling.

**The gauntlet** ([docs/gauntlet.md](docs/gauntlet.md)) runs seven programs, and all seven are at
parity with hand-written code on Go, JavaScript and Java. Last full benchmark:
[gauntlet-2026-09-07](gauntlet/results/gauntlet-2026-09-07.md), where the largest gap was 1.13× and
the largest win 0.91×. Program 7's tree walk was re-measured in
[compfacts-2026-09-17](gauntlet/results/compfacts-2026-09-17.md): 1.06× of hand-written unclamped Go,
with no clamps.

**Provability.**
- **2,361 of 2,413** integer operations are proven inside the portable window. The 52 left are mostly
  meant to be refused.
- **345 of 382** loops are proven to terminate.
- Both counts, and every emitted file, are pinned by `cmd/check`.

**Host APIs.**
- Surveys, as "declarable / usable":
  - Go 87.8% / 60.4%;
  - the JVM 81.4% / 60.9%;
  - Win32 90.5% declarable, 31.0% callable *and* linkable;
  - JavaScript 100% declarable, a figure the survey itself calls vacuous.
- **Supported, in the sense of [ADR 0022](docs/decisions/0022-host-declarations-are-written-by-hand.md)**
  (declared by hand, checked against the host, exercised by a program): the Go packages
  `unicode/utf8` and `encoding/hex`, plus `io`'s interfaces.

**The standing goal** (hamza) is **the Go standard library, package by package**. When a package hits
a wall that needs language work, stop and research it, then design it.

**Next, from the current assessment:**
1. Resume packages (`encoding/binary`, `strconv`). The analysis layer grows only for a refusal that
   has been named first. **Both hit the integer window first**: their 64-bit results can be printed
   but not computed with, and declaring the true range today makes them bignums.
   [what-an-int-is.md](docs/what-an-int-is.md) is the research — `int` as ℤ, with each target
   realizing what it can natively — and it ends on a decision that is hamza's.
2. ~~Gate compile time in `cmd/check` against the baseline.~~ **Done**
   ([compiletime-2026-09-21](gauntlet/results/compiletime-2026-09-21.md)): a compile is slower at 1.5×
   and +250 ms, measured to flag all six compiles `7e36002` slowed and none across identical sweeps.
   The regression itself is **not** fixed — the tokeniser still compiles in ~700 ms serially, freq in
   8.8 s on Go and 22 s on the JVM — and the baseline records today's costs, not the old ones.
3. A Windows application.

**The balance risk, named in every assessment.** The compiler grows much faster than the code written
in the language.
- Last round: +7,892 lines of compiler against +3 lines in `examples/` + `lib/`.
- `emit : core` is 4.42.
- The analysis layer (`interval`, `refine`, `linear`, `monotone`, plus `fact`, `content`, `smash`,
  `component`) is 8,727 lines.

The part that decides what is legal is the hardest to check. Write programs, and let them demand the
analysis.

Previous assessments:
[09-13](docs/assessment-2026-09-13.md), [09-11](docs/assessment-2026-09-11.md),
[09-09](docs/assessment-2026-09-09.md), [09-06](docs/assessment-2026-09-06.md),
[08-20](docs/assessment-2026-08-20.md), [08-19](docs/assessment-2026-08-19.md),
[08-13](docs/assessment-2026-08-13.md).

## Read first

1. [README.md](README.md)
2. [docs/design-direction.md](docs/design-direction.md). Its section 8 lists the open questions.
3. The ADRs in [docs/decisions/](docs/decisions/).
4. [docs/spec/state.md](docs/spec/state.md), which is the language read off the code.
5. The current assessment.

Measurements in [gauntlet/results/](gauntlet/results/) are the authority. **Every design claim here
that was not measured has been wrong about half the time.**

## Decisions already made

Do not relitigate these without reading the ADR first. Each has a "Why not" section recording
rejected alternatives.

| Decision | ADR |
|---|---|
| Targets are ecosystems; portability is a program property | [0001](docs/decisions/0001-parasite-model.md) |
| Capability graph, not a fixed layer tower | [0002](docs/decisions/0002-capability-graph.md) |
| Range-typed integers, mathematical semantics, machine representation | [0003](docs/decisions/0003-range-typed-integers.md) |
| Go, JavaScript, Java/Android first; C deferred | [0004](docs/decisions/0004-first-targets.md) |
| Compiler written in Go | [0005](docs/decisions/0005-implementation-language.md) |
| Backend interface is a file format, not a Go interface | [0006](docs/decisions/0006-ir-file-format.md) |
| Explore candidates against a fixed test; do not specify the core first | [0007](docs/decisions/0007-exploration-over-specification.md) |
| Parasite decisions are per-target measurements, not principles | [0008](docs/decisions/0008-measurement-over-principle.md) |
| Staging must not change results | [0009](docs/decisions/0009-staging-preserves-results.md) |
| Effects are a side condition on β, not a feature | [0010](docs/decisions/0010-effects-as-structural-rules.md) |
| Modules are resolution, not reduction | [0011](docs/decisions/0011-modules-add-nothing-to-the-reducer.md) |
| `int` is exact within ±(2⁵³−1) | [0012](docs/decisions/0012-portable-integer-range.md) |
| ~~Accept the allocation price~~ — superseded by 0020 | [0013](docs/decisions/0013-accept-the-allocation-price.md) |
| Recursion is not in the language | [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) |
| `loop`/`again` — guarded clauses over n variables | [0015](docs/decisions/0015-loop-and-again.md) |
| A target need not be an expression language | [0016](docs/decisions/0016-targets-need-not-have-expressions.md) |
| Booleans and control flow are in the language | [0017](docs/decisions/0017-booleans-are-in-the-language.md) |
| Immutable values, one scoped linear buffer | [0018](docs/decisions/0018-immutable-values-linear-buffers.md) |
| Precision by declaration — provisionally | [0019](docs/decisions/0019-precision-by-declaration.md) |
| A buffer is a nameable type: uniqueness on parameters | [0020](docs/decisions/0020-uniqueness-on-parameters.md) |
| Declarations are theories, and the surface that follows — amends 0011's *reasoning* | [0021](docs/decisions/0021-declarations-are-theories.md) |
| Host declarations are written by hand; the generator is their checker | [0022](docs/decisions/0022-host-declarations-are-written-by-hand.md) |
| A generator does not make a claim it cannot justify | [0023](docs/decisions/0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) |
| A comment never carries meaning; documentation is a term | [0024](docs/decisions/0024-comments-are-erased.md) |
| A module path is the host's path, and the file is the path | [0025](docs/decisions/0025-a-module-path-is-the-hosts.md) |

## How this project is run

**The core is deliberately unspecified.** Do not propose freezing it, and do not treat
[core-candidates.md](docs/core-candidates.md) as settled. The predecessor project stalled on a fixed
language that effort then went into making viable, and recreating that is the primary process risk.

**[The gauntlet](docs/gauntlet.md) is the one fixed commitment.** It is seven programs on three
targets, held to parity with hand-written code. Candidates are killed by measurement, not argument;
when one dies, write an ADR naming what killed it. The derivations in
[docs/derivations/](docs/derivations/) record what was believed, what measurement said and which won.
Read them before proposing anything about lowering, generics, closures or capability granularity.

**An assessment is written when a plan finishes**, not on a schedule. It:
- scores the last plan;
- measures the balance with the same counting method every time;
- names process failures;
- sets the next plan, with a "deliberately not next" list.

It is deliberately **not** an ADR.

**Beware the minimality trap.** A tiny elegant core minimizes the constructs needed to express all
computation. That is not the property this project needs; lambda calculus allocates precisely
because everything is a function. The core should be minimal **subject to lowering natively to every
target at zero cost** ([core-candidates.md §1](docs/core-candidates.md)).

## The language today

Grounded in [docs/spec/state.md](docs/spec/state.md), which is checkable by grep. Each item names the
spec to read before touching it.

### Core

- **Terms.** Seven term kinds: name, integer, float, string, `true`/`false`, `fn` and application.
  `core.Term`'s `KBound` is internal.
- **Comments.** `;` to the end of the line or of the input, and **gap rather than a token**: erased by
  the lexer, so nothing below the reader sees one and no emitted file carries one. No doc comments, no
  pragmas, no block comments, no `#;` — which would let a comment change which grammar a form takes,
  since `def`, `let` and every clause chain are discriminated by arity or parity
  ([comments.md](docs/spec/comments.md), ADR 0024). **A tool that rewrites source preserves every gap
  or refuses the edit**; two rewriters have lost comments the compiler could never have missed.
- **Definitions.** `(def f (x…) body)` is sugar for `(def f (fn (x…) body))`: a parameter list is a
  list of NAMES, there is one body, and `(fn …)` stays legal
  ([def.md §4](docs/spec/def.md), [program-surface.md](docs/program-surface.md)). **A constant is a
  value** — `(def cap 65536)`; the `(fn () …)` wrapper is for a computation, whose effects unfolding
  would repeat.
- **Binding.** `(let x e … body)` is flat, sequential and n-ary, with a name or a `(tuple a b)`
  pattern on the left ([binding.md](docs/spec/binding.md)). It erases to applications, so nothing
  below the reader knows it; the RESIDUAL's `let` — `(let e (fn (x) b))`, what β leaves when it
  declines to substitute — keeps its own spelling, and the old source spelling is refused.
  `seq` is a binding whose name is discarded, and works only because ADR 0010 denies weakening.
- **Reduction.** Four rules, two with two clauses ([state.md](docs/spec/state.md)). `if`, `let`, `loop`, `=`, the integer operators, `the` and the table
  operations are **injected** into every target; declaring one is an error.
- **Effects.** A declared purity bit per primitive, defaulting to impure. An impure argument is
  let-bound, never substituted, which denies contraction, weakening and exchange
  ([effects.md](docs/spec/effects.md), ADR 0010). A table read through a bound variable is not
  substituted into an impure body (§7c).
- **Booleans and control.** `bool` is data and `if` is its eliminator; `and`/`or`/`not`/`cond` erase
  in the reader (ADR 0017, [booleans.md](docs/spec/booleans.md)). `(if true a b) → a` is the only
  evaluation reduction performs. The reader also builds `(if (not c) a b)` as `(if c b a)` — case-of-case
  plus that evaluation, done eagerly — **unless a branch is a boolean literal**, where the term is a
  connective a backend emits as an operator. That is what makes `cond` cost nothing
  ([cond-2026-09-19](gauntlet/results/cond-2026-09-19.md)).
- **Iteration.** `(loop ((x z)…) c e … else e)` with `(again a…)` (ADR 0015). `again` is a jump; it may
  be a clause body or sit under a `let`, never under an `if`. `match` is reader sugar over `loop`
  ([match.md](docs/spec/match.md)).
- **No recursion** (ADR 0014). A balanced, data-independent recursion is a loop over levels, as
  Karatsuba showed ([karatsuba-2026-08-30](gauntlet/results/karatsuba-2026-08-30.md)).
- **Closures.** A closure may not survive staging ([closures-direction.md](docs/closures-direction.md)).
  Host callbacks come in three tiers, and only tier 3, a manufactured closure that escapes, is refused
  ([callbacks.md](docs/spec/callbacks.md)).

### Data

Every data form is a function whose domain differs ([data.md](docs/spec/data.md)).

- **Tables.** `(array V)` is a function on `[0, len)` and indexing is application
  ([tables.md](docs/spec/tables.md)). `build` zero-fills (§14.3). A length is bounded by the window
  or by a target's `max-len` (§2.3.1).
- **A table written as its GRAPH** — `(array 104 105 33)` — takes the JOIN of its elements' exact
  ranges, and at a boundary the DECLARED element decides and the literal must fit it
  ([literal-elements.md](docs/literal-elements.md)). windows keeps qwords by choice.
- **Buffers and mutation.** Values are immutable. Mutation happens only inside
  `(build n (fn (b) …))`, whose buffer is linear, checked by `occurrences` on the residual
  (ADR 0018). `(buffer V)` is a nameable parameter type (ADR 0020):
  - uniqueness is *assumed* at an export;
  - linearity is *checked* through the body;
  - a buffer may not be an element type.
- **Tuples and products.** `tuple` is one object for products and several results. A table of tuples
  is flattened by currying before the checker runs, so no backend knows products exist
  ([products.md](docs/spec/products.md), [values.md](docs/spec/values.md)).
- **Variants.** `variant` is closed, finite and non-recursive, and takes type arguments. At a boundary
  it lowers to tag plus payload. `case`-of-case makes both levels free
  ([sums.md](docs/spec/sums.md), data.md §5.5). `option` is one declaration, in `lang`.
- **Maps.** `(map int V)`. A read gives `(option V)`, every map has a declared capacity, and iteration
  is `keys` in ascending order ([maps.md](docs/spec/maps.md)).
- **Strings.** An element of the free monoid over Unicode scalar values. The operations are `concat`,
  `""` and `string-of`; three text programs needed nothing more
  ([string-literals.md](docs/spec/string-literals.md), [strings.md](docs/spec/strings.md),
  [string-operations.md](docs/string-operations.md)).
- **Not built:** records, symbols, `with`.

### Integers

- **What an `int` is.** Exact within ±(2⁵³−1) (ADR 0012). A range is a type: `(int LO HI)` means an
  `int` for typing, a premise for the analyses, and a representation for storage (ADR 0003,
  [integers.md](docs/spec/integers.md)). All four hosts agree inside the window. Bitwise operators are
  deliberately not promoted to the language.
- **Bounded by default** (ADR 0019). An operation not proven inside the window is a compile error,
  cleared by one of:
  - narrowing the range;
  - declaring a range above the window, which promotes the value to arbitrary precision;
  - `-checked`, which asks for the trap.
- **Above the window.** A target chooses `(big-repr host)` or `(big-repr limbs)`. The bound is
  enforced on both, so representation never changes which programs are legal
  ([bigrepr-2026-09-03](gauntlet/results/bigrepr-2026-09-03.md)).
- **Element width follows the range.** `(int-repr …)` picks the narrowest host type containing the
  range ([elemwidth-2026-08-27](gauntlet/results/elemwidth-2026-08-27.md)).

### Types, contracts and facts

- **The type checker** runs on the residual, which is monomorphic, first-order and closed
  ([types.md](docs/spec/types.md)). `sig` is a claim checked in two directions.
- **`where` has three meanings** ([refinements.md §6b](docs/spec/refinements.md)):
  - on a `prim`, an obligation;
  - on an export, assumed;
  - on an internal definition, dropped, because inlining is stronger.
- **`ensures` is the exact swap** ([postconditions.md](docs/spec/postconditions.md)). A pure call is
  an atom of the linear fragment.
- **Facts** are guarded boundedness axioms of a local theory extension, instantiated on present terms
  ([facts.md](docs/facts.md), [theories.md §7](docs/spec/theories.md), `emit/lang-facts.oro`). Only F-B
  is admitted; F-C, F-D and F-E are reserved. **Content facts (F-D₁) are derived, never declared**:
  - Houdini invariants;
  - induction on a buffer's store chain;
  - components of a strided table.

  See [array-facts.md](docs/array-facts.md).

### Declarations and targets

- **Declarations are theories** (ADR 0021, [spec/theories.md](docs/spec/theories.md)):
  - a module is a theory;
  - a target is a model;
  - `use`/`export`/`provides` are morphisms;
  - a file is the glue of its forms.
- **What is built:**
  - target files in the `(sig … (host …))` spelling ([target-files.md](docs/spec/target-files.md));
  - types owned by their module;
  - companions and `include`, from which subtyping is derived, written as child modules inside their package;
  - `const`, and constants as range endpoints;
  - manifest types unfolded by δ;
  - `D_T`, definitions inside `provides`.
- **A target is a chain of layers** `Δ_T = L₁ ▷ … ▷ Lₖ`: glue within a layer, override between layers.
  The source's own directory is the nearest layer ([target-system.md](docs/spec/target-system.md)).
  The backend is `(backend NAME)`, from a closed set.
- **A host module's path is the HOST'S path**, prefixed by the host — `go/encoding/hex`,
  `java/java/util/regex` — because `ι(p) = HOST ++ p` is injective and commutes with `lastSegment`,
  so an import's default alias is the host's own name and needs no `as` (ADR 0025,
  [modules.md §3.1](docs/spec/modules.md)). A type's companion is a child of its package. **The file
  is the path**: `targets/go/encoding/hex.oro`, and the target loader walks. **A module may contain a
  module** — a child's path is its parent's followed by its own, so a companion is written inside its
  package ([target-files.md §1a](docs/spec/target-files.md),
  [nestmod-2026-09-20](gauntlet/results/nestmod-2026-09-20.md)). The absolute spelling stays legal,
  because a fragment in another layer must be able to add to a module it does not enclose.
- **Host declarations are written by hand**, by someone who read the host's source. The generator
  checks them (ADR 0022) and never claims what it cannot justify (ADR 0023).
- **What a host call does to a buffer** is declared as a write-borrow or a consume, following
  [host-buffers.md](docs/host-buffers.md).

## Constraints that override normal instincts

Violating these produces code that looks fine and undermines the project.

**Anything promoted to the LANGUAGE works on every target, and the compiler finds the
implementation.**
- A target may neither decline nor declare a language construct (`fn`, `def`, `loop`, `if`, `let`,
  `=`, integer operators, tuples, variants).
- The capability graph is for **target-native names** (`go.map`, `x64.andb`), where "this target
  cannot do it" is a true answer.
- The precedent is `values`. It was reverted when it arrived as a construct two targets declined; it
  came back with Java getting a record and windows getting `rax`/`rdx`.

**Never lower further than the target requires.** Emitting a hand-rolled hash table into Go, which
has `map`, is wrong on performance, size and ecosystem access at once.

**But never assert which host construct is fastest: measure it** (ADR 0008). The first baseline
refuted four such inferences, and the refutations did not all reproduce either. Treat host compilers
as black boxes with measured behaviour.

**Staging must never change an answer** (ADR 0009). Compile-time arithmetic must be bit-identical to
runtime.
- Force every compile-time float operation through explicit `float64`; Go's untyped constants fold
  `0.1+0.2` differently.
- Integers fold only inside the window.
- Division by zero never folds.

**Never make the core a superset of one host.**
- JavaScript has no integers, structs or int64.
- The JVM has no unsigned types, `goto` or tail calls.
- Go restricts `goto` and forbids pointer arithmetic.

**Never introduce boxing or hidden allocation into the core.** That is what killed the predecessor
([design-direction.md §2](docs/design-direction.md)). The one allocating construct is visible in the
source.

**Never add unstructured control flow.** Structured control only.

**Performance claims are measured against hand-written code in the target language**, and a
decision never rests on less than the ~15% noise floor of the hybrid P/E-core laptop the benchmarks
run on.
- When adding to the gauntlet, carry both the form expected to win and the form expected to lose.
- Check the host compiler's own decisions as well: `go build -gcflags="-m -m"` and
  `-gcflags="-d=ssa/check_bce/debug=1"` have each been decisive where timings were ambiguous.

**Primitives are declared in `targets/*.oro`, not in Go.** Adding a case to `emit/*.go` for a host
function is the wrong place. Only structural constructs live in code.

**Prefer deriving a fact over clamping around it or declaring it.**
- A clamp hides the fact instead of establishing it.
- An internal `where` is weaker than what inlining gives.

A clamp in a program is a missing proof.

## Lessons that recur

Each of these has bitten more than once. The instances are in the results they name.

- **A path nothing runs is a path nothing checks.** A `; skip:`, a rebuilt term discarded unless
  `-checked` is on, a backend no target used: each hid a real bug for weeks.
- **A harness that cannot fail proves nothing.**
  - Every soundness test must be shown to fail against a planted bug, including the bug that shipped.
  - Refusal-shaped properties need an anti-vacuity guard.
  - Random search cannot test what over-approximation makes sound; pin those with exact witnesses.
- **"Not refused" is not "proven".** A path that returns success with a note has checked nothing.
- **`Body()` and `openFresh` rebuild terms.**
  - A map keyed by a term pointer answers about a copy.
  - `FnClosed` does not close.
  - Printing hides an unbound binder.
  - The structural test is: *a lambda's closed body never contains a free name equal to its own
    parameter.*
- **The emitter must be a function of its input.** Iterating a Go map without a total order made
  output vary between runs. Test it by running twice.
- **A survey's first number describes the measurer.** Seven corrections so far, in both directions.
  Read ten members of a figure one at a time before it becomes a plan item.
- **Benchmark-method errors have many species:**
  - a closure inside the reference;
  - a fixed iteration count;
  - benchmarks sharing one process, including allocating ones;
  - V8 eliminating unused work;
  - two things changed at once;
  - a silently modified input;
  - comparing against HEAD after a regression has landed;
  - timing one job among many run in parallel, which inflates it unevenly (the tokeniser ~2.5×).

  Make a suspicious result explain itself before recording it.
- **A refusal can hide a wrong answer.** Removing the refusal is often what finds it.
- **A program finds what a construct suite cannot.** `tally`, `jsonfmt`, `freq` and `tree` each found
  bugs in constructs the differential suite already covered; size and combination were what was new.

## Adding to the language

**Nothing goes in without a specification saying how it behaves on every target.** Write the spec
first. The first time a construct shipped without one, writing it immediately found three silently
accepted shapes.

The test is not "is it useful":
1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement **observable**? If so, it is Tier 2 and carries no
   portability claim.

Every primitive is classified in [primitives.md](docs/spec/primitives.md). A Tier 1 name without a
conformance case ([gauntlet/conformance/](gauntlet/conformance/)) is decoration.

## Working conventions

**Always be mathematical and algebraic, and reach for the literature.** This is hamza's standing
rule. Before building or declaring anything, say what it IS:
- the set;
- the operation;
- the law it obeys;
- the theorem that makes the design sound;
- where the literature already answered it.

A host package is an algebra before it is a list of functions. A declaration is a claim about that
algebra: state the law, make the declaration say as much of it as the language can check, and name
the part it cannot. Deriving what a construct is and measuring whether it is good are different jobs.
Do the first before the second, and never substitute a list of cases for a derivation.

**Every significant decision gets an ADR** in `docs/decisions/`, using the template in its README.
The "Why not" section is the point. Reversing a decision means a new ADR that supersedes the old one.
Do not edit decision history.

**A result gets its own document** in `gauntlet/results/NAME-YYYY-MM-DD.md`. It records what was
measured, what failed first, what it cost, and what was not built. Update this file only where the
**state** changed.

**Keep documents current rather than accumulating drafts.** A stale design document is worse than
none. Status lines ("nothing built", "no decision") go stale fastest; fix them when the thing ships.

**Emission is byte-identical by default** (hamza, 2026-09-15). `cmd/check` compares every source ×
target's emitted code, outcome, proof counts and refusal text against the baseline in
`gauntlet/check/`. A difference is a change for review:
- **keep it** when it is correct and no slower, with `-accept "reason"`. That requires the compiler
  and differential steps passing in the same run, and logs the reason in `gauntlet/check/ACCEPTED.md`;
- **otherwise fix the code.**

**Compile time is gated too** (gauntlet/check/README.md, "The rule for compile time"): a compile at
1.5× its baseline time **and** 250 ms slower is a change for review, confirmed by a second sweep
before it is reported, and kept only with `-accept`. The sweep's times are not serial times — the
tokeniser costs ~2.5× its serial time among 16 parallel compiles — so **to say how long one program
compiles, time its binary serially and warm, and compare against a binary built at the baseline's
commit, not against HEAD**.

## Build commands

Module path is `oroboros` (local).

```bash
go run ./cmd/check               # every check — gauntlet/check/README.md
go run ./cmd/check -skip tooling # everything but the host surveys
go test ./core/ ./emit/          # the compiler
go test ./core/ -run TestBeta    # one test
go vet ./...
```

```bash
go run ./cmd/build -target=go -o hello examples/hello.oro   # a real binary
go run ./cmd/oro -target=portable-go examples/dot.oro       # reduce to normal form
go run ./cmd/gen -name tree examples/json/tree.oro go gauntlet/go/gen_jsontree.go   # emit into the gauntlet
cd gauntlet/go && go test -bench='TreeGen|TreeFlat$' -benchtime=20000x -count=5   # generated vs hand-written
go run ./cmd/intervals examples/native/sieve-go.oro go   # what the interval analysis proves, per exported definition (a main-only program reports 0/0)
```

- `gauntlet/go`, `gauntlet/js`, `gauntlet/java` and `experiments/legibility` are **separate modules**.
  `cd` into them before running their tests.
- `gauntlet/fmt/*.go` carry `//go:build ignore`; they are standalone scripts run with `go run`.
- On Windows, Git Bash's `/tmp` is not visible to native tools; use the session scratchpad.
- **Line endings are LF**, pinned by `.gitattributes` (`* text=auto eol=lf`), whatever a machine's
  `core.autocrlf` says. The four Windows scripts are CRLF and the emission baseline is `-text`. So
  `gofmt -l` means what it says; if it ever lists files nobody edited, check the endings first.
- A **doctor**, which would report missing toolchains, is wanted and deliberately not built yet
  ([build.md §6](docs/spec/build.md)).

## What exists

| | |
|---|---|
| `core/` | Reader, terms, β/δ reducer, module loading, variants, hygiene |
| `emit/` | The four backends, type checker, refinement layer (`refine`, `linear`, `fact`, `content`, `component`), interval analysis (`interval`, `smash`, `monotone`), termination, target loader (`target`, `companion`, `alias`, `constend`), linearity, big-integer representation (`bigrep`, `biglimb`, `bigreuse`), products |
| `targets/` | Target declarations: **data, not Go**. `go/`, `js/`, `java/` and `windows/` are host-native directories. The `portable-*.oro` files are the retired portable layer, kept for the old benchmarks |
| `lib/` | Modules a program imports with `(use …)`: `io` and `os`, which are portable names over each host (`provides` cells), plus `num` and `win` |
| `cmd/` | `check` (every check), `build` (a program), `gen` (emit one file), `oro` (reduce), `intervals` |
| `examples/` | Small programs plus: `int/` (meant to be refused), `big/` (arbitrary precision, including `render.oro`), `io/` (`wc`, `jsonfmt`, and `freq`, the largest program), `json/` (tokeniser and tree), `kara/`, `tally/` (one application on Go and the JVM), `native/` (the gauntlet's native sources) |
| `gauntlet/` | Hand-written references (the bar), `results/`, `check/` (the baseline), `differential/` (cases on all four targets), `conformance/` |
| `gauntlet/stdlib/` | The four host surveys; `acceptance/`, thirteen programs, two of them whole packages declared by hand; `tooling_test.go`, which checks every hand declaration against the host |
