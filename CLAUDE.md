# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## Project state

**One gauntlet program is not at parity, and that is an accepted, provisional decision** —
[ADR 0013](docs/decisions/0013-accept-the-allocation-price.md). g7's stencil runs at **1.79× on Go
and 2.01× on JS** against a hand-written buffer-reusing form
([measurement](gauntlet/results/stencil-2026-08-15.md)). The emitted code is at parity with
hand-written *functional* code; `materialize` is what costs, because it allocates fresh so nothing
can alias.

**The price is the SHAPE, not the compiler, and it is avoidable one layer down** —
[native-gauntlet-2026-08-20](gauntlet/results/native-gauntlet-2026-08-20.md). The stencil moved to
the native Go target carrying both forms: allocating measures **0.93x** hand-written and
buffer-reusing measures **0.999x**. In each shape emitted matches hand-written, and *allocating*
costs 2.71x for hand-written code too. So the portable layer has no way to express reuse and a
native target does — `go.set-float64` is Go's own store, no portability claim, at parity.

**This price is expected to be paid off, not kept.** The ADR names the triggers that should reopen
it. Note the correction recorded there: the original first trigger, *a type system exists*, **fired
and bought nothing** — uniqueness constrains the *context*, not the value, so it is not a
refinement. The nearest machinery is ADR 0010's substructural discipline plus the reducer's
occurrence counting, and that is a hypothesis rather than a finding. Do not treat 1.8× as the bar;
the bar is still hand-written code.

**Working compiler.** A β/δ reducer with call-by-need and an effect discipline, three backends
(Go, JavaScript, Java), and **all seven gauntlet programs** reaching parity with hand-written
code — two of them producing byte-identical machine code on Go.

Start with [README.md](README.md), then [docs/design-direction.md](docs/design-direction.md),
then the ADRs in [docs/decisions/](docs/decisions/). Measurements are in
[gauntlet/results/](gauntlet/results/), and they are the authority: **every design claim in this
repository that was not measured has been wrong about half the time.**

## What this project is

Oroboros is a small language and build system that **parasitizes** target ecosystems rather
than abstracting over them. Portability is a property a program may or may not have, computed
by the compiler — not a global guarantee.

The single mechanism is a **capability graph**: modules declare required capabilities, targets
declare provided capabilities plus shims, and building means covering the former with the
latter. The governing rule is **emit at the highest layer the target natively provides** —
lower only as far as necessary.

## Decisions already made

Do not relitigate these without reading the ADR first. Each one has a "Why not" section
recording alternatives that were considered and rejected.

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
| Accept the allocation price, provisionally | [0013](docs/decisions/0013-accept-the-allocation-price.md) |
| Recursion is not in the language | [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) |
| `loop`/`again` — guarded clauses over n variables | [0015](docs/decisions/0015-loop-and-again.md) |
| A target need not be an expression language | [0016](docs/decisions/0016-targets-need-not-have-expressions.md) |
| Booleans and control flow are in the language | [0017](docs/decisions/0017-booleans-are-in-the-language.md) |
| Immutable values, one scoped linear buffer | [0018](docs/decisions/0018-immutable-values-linear-buffers.md) |

Design questions still open are listed in section 8 of
[docs/design-direction.md](docs/design-direction.md) — memory model, error model,
concurrency, strings, module format, naming translation.

## How this project is run

**The core is deliberately unspecified.** Do not propose freezing it, and do not treat
[docs/core-candidates.md](docs/core-candidates.md) as settled — candidates there are
disposable and expected to die. The predecessor project stalled on a fixed language that
effort then went into making viable; recreating that is the primary process risk.

**[The gauntlet](docs/gauntlet.md) is the one fixed commitment.** Five programs, three
targets, parity with hand-written code. Candidates are killed by measurement, not by
argument — arguments only select what is worth measuring. When a candidate dies, write an ADR
naming what killed it.

The baselines exist and have been run: [gauntlet/](gauntlet/), results in
[gauntlet/results/](gauntlet/results/). The leading candidate — a rewriting core, which turns
out to be **lambda calculus staged so it terminates at compile time** — has survived
hand-derivation of all five programs plus escaping closures, in
[docs/derivations/](docs/derivations/). Read those before proposing anything about lowering,
generics, closures, or capability granularity; each records what was believed, what measurement
said, and which of the two won.

**Current standing** is in [docs/assessment-2026-08-20.md](docs/assessment-2026-08-20.md) — the
analysis round, the one place a demonstration became a decision by accident, and the five things
next. The previous two are [2026-08-19](docs/assessment-2026-08-19.md) — four
targets in, what should go into the language next, and the one place the process has drifted (the
gauntlet still runs on the retired portable layer). The previous one is
[docs/assessment-2026-08-13.md](docs/assessment-2026-08-13.md).
Deliberately *not* an ADR — writing "candidate B is the core" as a decision would recreate the
predecessor's failure. Every falsifier it named has since been tested and none fired, so its
**Revised verdict** section calls for building the vertical slice.

**The atom** — the irreducible unit, what lambda calculus is to a Lisp — is in
[docs/the-atom.md](docs/the-atom.md): **lambda calculus in which the normal form is a
parameter.** A target supplies a partition of names into primitive and defined; reduction runs
until only primitives remain. Layers, both directions of ADR 0002, and staging all collapse into
that, and grading turns out to be an *observation on the normal form* rather than a primitive.
Do not describe the core as "a vocabulary" — that was an earlier and worse answer, and the
vocabulary survives only as the parameter to a reduction relation.

**Beware the minimality trap.** The instinct toward a tiny elegant core — lambda calculus,
objects and messages — minimizes *constructs needed to express all computation*, which is not
the property this project needs and is often opposed to it. Lambda calculus is minimal because
everything is a function, which is exactly why it allocates. That is the Shen wall. The core
should be minimal **subject to lowering natively to every target at zero cost**, the way
WebAssembly is. See section 1 of [docs/core-candidates.md](docs/core-candidates.md).

## Constraints that override normal instincts

These are the non-obvious ones. Violating them produces code that looks fine and undermines
the project.

**Anything promoted to the LANGUAGE works on every target, and the compiler is responsible for
finding the implementation.** `fn`, `def`, `loop`, `if` are the language's. A target does not get
to decline one, and it does not get to *declare* one either — we pick the best way to do it on each
host, and if a host has no native form we build one. The capability graph is for **target-native
names** — `go.map`, `js.at`, `x64.andb` — where *this target cannot do it* is a true and useful
answer that a program can be told. It is **not** for language constructs, where that answer means
the construct is a library carrying a portability claim, which is the thing
[ADR 0001](docs/decisions/0001-parasite-model.md) exists to refuse.

**Applied 2026-08-20**: `let` and `loop` joined `if` as names the compiler injects into every
target, and declaring one is now an **error**. The four native targets declare **no structural
primitives at all**. Eleven target files had carried the same two lines, while `core/read.go`
already desugars `let`, `seq` and `loop` into applications of those precise names — so a target
spelling either differently broke every program, and forgetting one made an ADR 0015 language
construct silently unavailable. What stays declarable is the retired portable layer —
`fold-range`, `fold-range2`, `make-vec` — which is library, not language.

The precedent is already written and was not generalised:
[ADR 0017](docs/decisions/0017-booleans-are-in-the-language.md) put `if` in the language and made
*declaring a boolean name an error* — the target may not even offer an opinion. Every language
construct should be held to that.

The instance that produced this rule: `values` — several results from one function — was built as
reader sugar (so, the language) with a `(multi-return …)` target declaration and a refusal on Java
and windows. That is a construct in the core that two of four targets decline, which is incoherent.
Reverted. If several results go into the language, **Java gets a generated record and windows gets a
register or stack convention**, and finding those is the compiler's job, not the target author's.

**Never lower further than the target requires.** Emitting a hand-rolled hash table into Go
when Go has `map` is wrong on performance, binary size, and ecosystem access simultaneously.
This is the single most common way to get the architecture wrong.

**But never assert which host construct is fastest — measure it.** That rule is a prior, not a
derivation. The first baseline run refuted four inferences from it at once: JS's `Map` is 3.25×
*slower* than a null-prototype `Object`; Java's fused `merge` loses 2.6× to unfused
`getOrDefault`+`put`; Java's `Point[]` costs 1.05× where JS's array-of-objects costs 2.86×; and
all three hosts inline a literal callback we assumed only we would specialize. Every one was a
plausible reading of how the host is documented to work. Treat host compilers as black boxes
with measured behaviour. See [ADR 0008](docs/decisions/0008-measurement-over-principle.md).

**Staging must never change an answer.** Compile-time arithmetic must be bit-identical to
runtime — IEEE-754 binary64 for floats, exactly. Go folds `0.1+0.2` to `0.3` at compile time and
`0.30000000000000004` at runtime because its untyped constants are arbitrary-precision. Writing
the compiler's constant folder the natural way in Go reproduces that bug and makes partial
evaluation unsound. Force every compile-time float operation through explicit `float64`. See
[ADR 0009](docs/decisions/0009-staging-preserves-results.md).

**Never make the core a superset of one host.** The core must be expressible on Go,
JavaScript, and the JVM at once. JavaScript has no integers, no structs, and no int64; the JVM
has no unsigned types, no `goto`, and no tail calls; Go restricts `goto` and forbids pointer
arithmetic. If a proposed core feature only works in one of them, it is not a core feature.

**Never introduce boxing or hidden allocation into the core.** This is what killed the
predecessor project — see section 2 of the design direction. Boxing in the substrate sets a
performance ceiling for every target at once, and no host optimizer can undo it.

**A program could not construct data until 2026-08-15** — every gauntlet program took its arrays
as parameters, and `main` takes none, so programs could only print constants. The fix is one
primitive, `make-vec`, wrapped by `num/vec.materialize`: build with the delayed representation,
which fuses, and **materialize only at a boundary**. Materializing in the interior costs the 13×
the stencil benchmark measured, and [that cost is the point](docs/spec/construction.md).

**Recursion is not in the language** — [ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md).
It reduced correctly and no backend emitted it, so `oro` accepted programs `build` refused; it is
now an error, checked per-target before reduction by `Env.CheckProgram`. Emitting it would ship the
first construct that looks Tier 1 and is not — stack depth differs by orders of magnitude across
Go, the JVM and JS, and none of them guarantees tail calls. Iteration is `fold-range`. **TCO is
moot** until a while-shaped loop primitive exists, which is also recursion's own prerequisite.

**Iteration is `fold-range` and `loop`** — [ADR 0015](docs/decisions/0015-loop-and-again.md).
`(loop ((x z)…) c e … else e)` with `(again a…)` gives n loop variables with **no product**, early
exit at parity with hand-written code, and unbounded iteration. `again` may be a clause body or sit
under a `let`, never under an `if` — *let binds, if branches* — so the clause list is the loop's
whole control flow. `again` is a jump, not a call, so ADR 0014 stands. **Termination is now a
program property**, computed like portability, not a language guarantee.

**Never add unstructured control flow.** Structured only: `if`, `loop`, `break n`, `return`.
Recovering structure from `goto` is a hard algorithm and three of the initial targets cannot
express `goto` at all.

**Refusing closures costs FUNCTION VALUES, not host APIs** — [callbacks.md](docs/spec/callbacks.md).
Three tiers, and only the third is a closure. **Tier 1** — a lambda written at the call site, where
the HOST closes over it: `go func(){}`, `defer`, `sort.Slice`, `addEventListener`, `setTimeout`,
`Runnable`. The lambda never becomes a value in our residual; it sits in a structural position the
backend already reads positionally, and Go's/V8's/the JVM's own closure does the capture, at parity
by construction. **Tier 2** — a function pointer with no free variables, which all four hosts have
and which we **refuse today with the wrong message**: `(def twice (fn (x) (go.* x 2)))` returned as
a value says *"this is an escaping closure"* and it is not one. And **the Win32 API is designed for
a language without closures** — `EnumWindows(fn, LPARAM)`, `CreateThread(…, lpParameter)`,
`qsort_s(…, context)` pass the environment explicitly, so the OS API that looks most hostile is the
one best suited. **Tier 3** — a manufactured closure that escapes — stays refused, and ADR 0018
depends on it. What is genuinely lost: dispatch tables and partially-applied callbacks. One hazard
to carry: **a callback body may not capture a buffer**, or two goroutines hold it and ADR 0018's
linearity is gone.

**Closures are not a core primitive.** They belong above the core, lowered by defunctionalization
or explicit environment structs. First-class closures require captured environments, which
require heap allocation.

**Performance claims must be measured, not asserted.** The standard is parity with
hand-written code in the target language, checked by benchmark against a hand-written
equivalent. "Should be fast" is not a result.

When adding to the gauntlet, **carry both forms** — the one expected to win and the one expected
to lose. Five beliefs were refuted in the first run only because the losing form was there to
measure. And check the compiler's own decisions, not just the clock:
`go build -gcflags="-m -m"` and `-gcflags="-d=ssa/check_bce/debug=1"` were each decisive where
timings were ambiguous.

Benchmarks in [gauntlet/results/](gauntlet/results/) were taken on a hybrid P/E-core laptop with
a ~15% noise floor. Do not rest a decision on a smaller margin than that.

## Adding to the language

**Nothing goes in without a specification saying how it behaves on every target.** String
literals were added without one and [docs/spec/strings.md](docs/spec/strings.md) is the
correction — write the spec first.

The test for a proposed addition is not "is it useful":

1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement **observable**? If so it is Tier 2 and carries no
   portability claim.

Every primitive is classified in [docs/spec/primitives.md](docs/spec/primitives.md). Two are
Tier 1 only *within bounds* — `aindex` and `sat`, because an out-of-range read panics on Go, throws
on Java, and **silently returns `undefined`** on JS. A Tier 1 name without a conformance suite is
decoration: `split-words` passed every check for two months while returning different answers on
different targets. The suite is [gauntlet/conformance/](gauntlet/conformance/).

`length` fails (3) — `"🙂"` is 4 on Go, 2 on JS and Java, 1 counting characters — which is why it
is not in the core. Strings pass only by having almost no operations.

**The current state of the language is [docs/spec/state.md](docs/spec/state.md)**, read off the
code rather than from memory. Seven term kinds, five top-level forms, three reduction rules, two
parameters.

**Booleans are in the language and the connectives are sugar** —
[ADR 0017](docs/decisions/0017-booleans-are-in-the-language.md),
[booleans.md](docs/spec/booleans.md). `bool` is data, `if` is its eliminator, and
`and`/`or`/`not`/`cond` erase in the reader — Scheme's answer, ML's answer, and McCarthy's reason
from 1960: **a conditional cannot be a function**. Each backend puts the host's `&&` back, so
nothing is lowered further than the target requires, and that was
[measured](gauntlet/results/and-form-2026-08-19.md) rather than argued. A target declares zero
boolean names now, and declaring one is an error. Two consequences to carry: `(if true a b) → a` is
the **only** evaluation reduction performs, which gives conditional compilation with no
preprocessor; and the strict branchless operators survive as host names — `x64.andb` — because Ada
kept `and` beside `and then` for a measurable reason.

**Effects are a side condition on β, not a feature** — [docs/spec/effects.md](docs/spec/effects.md).
Purity is one declared bit per primitive, defaulting to *impure* so that a target author's omission
costs speed rather than correctness. An impure argument is never substituted; it is let-bound at
the application site, whatever its occurrence count, which denies contraction, weakening and
exchange in that order. There are no effect types, no monads, and no linear types on values, and
adding any of them should be argued against this first. `seq` is sugar for a β-redex with an unused
binder and works *only* because weakening is denied.

**There is now a type checker, and it is not in the language** —
[docs/spec/types.md](docs/spec/types.md). It runs on the **residual**, before emission, which is
cheap because reduction has already made the term monomorphic, first-order and closed. One checker
serves all three targets, including JavaScript, which previously compiled
`(f64.add "hello" 1.0)` into a program that printed `hello1`. `(sig name ((p type)…) result)` on a module export is a **claim checked in two directions**:
against the definition's residual, and against any target that provides the name *natively*. The
second is the job no host compiler can do, because the two implementations live on different
targets and no single compiler sees both. Parameters are **named** because a refinement attaches to a name.

**Refinements are built** — [docs/spec/refinements.md](docs/spec/refinements.md). `aindex` carries
`(where (and (<= 0 i) (< i (alen v))))`, and the obligation is discharged at every call site from
facts collected out of loop bounds. The fragment is linear integer arithmetic with a deliberately
incomplete decision procedure; **an undischarged obligation is reported, never assumed**. This
closed the first of the two holes shaped like a refinement, and **found a real latent bug in `dot`
and `centroid`**, which index two arrays under one loop bound. Still open: the integer range hole
([arithmetic.md §4](docs/spec/arithmetic.md)).

**What an unproven operation costs is 1.23× to 4.54×, and the shape decides** —
[checkcost-2026-08-19](gauntlet/results/checkcost-2026-08-19.md), the same source compiled twice
differing only in the declared range. Arithmetic-bound: **Go 4.54×, Java 1.52×** — the JVM has
`Math.addExact` as an intrinsic and Go has nothing, which is the §0 spread rule biting at a factor
of three. Memory-bound: **Go 1.23×, windows 1.46×**, because the branch hides behind the cache
miss. **The isolated microbenchmark was wrong in BOTH directions** — the same lesson as
bce-2026-08-15, where a 1.96× win in isolation vanished on memory-bound loops. A cost behaves the
way a saving does, and neither survives being quoted without its condition.

**The eleven integer questions are settled** — [integers.md](docs/spec/integers.md), each measured
on all four targets rather than read off four specifications. They **agree** on everything inside
the window: division truncates toward zero, the remainder takes the dividend's sign, the identity
`(a/b)*b + a%b == a` holds, and `int → f64` is exact — because the portable window IS the binary64
exact-integer range, so the constraint that looked like a concession to JavaScript makes the one
conversion everybody needs free. They **disagree** in exactly three places, and each is refused
rather than emulated: outside the window (no claim), **division by zero** (a precondition — and on
JavaScript `1/0` is `Infinity` and it keeps going), and `f64 → int` out of domain (three hosts,
three answers). Not settled deliberately: a bignum, which needs the product first.

**Representation selection is OPT-IN, behind `-checked`, and that is deliberate** — wiring it into
the default path reversed [ADR 0012](docs/decisions/0012-portable-integer-range.md) without an ADR,
breached requirement 5 by up to 4.54×, and made cross-target divergence *worse* (three targets trap,
JavaScript silently loses precision). Turning it on should be the consequence of deciding exact
integers, not the cause — [assessment-2026-08-20 §2](docs/assessment-2026-08-20.md). **A
demonstration wired into the default path is a decision, whether or not anyone made one.**

**A declared range selects the representation** —
[selection-2026-08-19](gauntlet/results/selection-2026-08-19.md). An integer operation the compiler
can bound keeps the host's own operator; one it cannot is rewritten to the `checked` primitive the
target declares. On `examples/native/sieve-go.oro`, adding one `(where …)` to the existing `sig`
takes it from **10 of 10 operations checked and 0 of 3 loops proven terminating** to **0 checked and
3 of 3**. Nothing else in the program changes. Java uses `Math.addExact`, x86 uses `jno`/`ud2`, Go
uses an immediately-called func literal, and **JavaScript declares none** — which is the price list
of [overflow-2026-08-19](gauntlet/results/overflow-2026-08-19.md) showing up as a capability.

**The corpus is `examples/int/`, and three of its six programs are meant to be REFUSED** — Collatz
(the conjecture is open), Fibonacci past 2⁵³, and exponentiation whose accumulator genuinely
overflows. A proof system is worth what it declines to prove. Growing it found four gaps invisible
on the sieves, including that `go./` was not recognised as division at all.

**Termination is computed now, not guarded by fuel** —
[sct-2026-08-19](gauntlet/results/sct-2026-08-19.md). Size-change termination (Lee, Jones &
Ben-Amram 2001) plus a trip count closed both open holes at once: **every integer operation in
every program is provably inside the portable window** once one range is declared, and **96% of
loops are proven to terminate**. The single refusal is a true negative — Newton's method on a
float. Two additions were needed that the paper does not have: integers are not well-founded, so
the **floor comes from the interval analysis** and is demanded of the *witness*; and an ascending
counter is handled by **orientation**, μ = −v, which is a ranking function as a change of variable.
Three bugs found on the way all made the numbers *worse* than the truth, which is the only reason
they were not mistaken for results.

**The type-system question is reopened, and the literature answers it** —
[types-direction.md §6](docs/types-direction.md). Three findings to carry. The **justification
moved**: a range does not need to transfer to the host, it changes what we *emit*, which is
categorically unlike the bounds-check case §1–2 refuted. A range is **already writable** —
`(sig f ((n int)) int (where …))` parses today — and reading it takes the sieve from **45% to 90%**
provable ([intervals-2026-08-19](gauntlet/results/intervals-2026-08-19.md)) with no language
change. And on sequent calculus specifically: GHC built Sequent Core, measured it, and shipped
**join points in direct-style Core** instead — and `again` already *is* a join point. What sequent
calculus does give us is **polarity**: a product eliminated by projection need never be built,
which is exactly why the product measured free. Take the classification, not the core.

**The primary data structure is SPECIFIED and not yet built** —
[tables.md](docs/spec/tables.md). One concept: **a table is a function with a known finite domain**,
and **types are functions; the domain is what varies**: `(fn (A) B)` is total on A, `(array V)` is
`(fn (int) V)` on `[0, len)`, `(map K V)` is `(fn (K) V)` on the keys present. Three points on one
scale. So **a bounds check is a domain condition** and Go's comma-ok is the same condition — and
the dependency rides in the **refinement** layer that already exists, which is simple types plus a
decidable constraint domain, which is **Dependent ML**, which types-direction §6.5 had already
named as our lineage from a measurement. Surface: `(array e…)` a graph, `(table n f)` a rule with
no memory, `(alloc t)` the one construct that allocates, `(len t)`, and **indexing is APPLICATION**.
`vec`/`vector` were rejected — every mainstream `vector` is a growable mutable memory-owning
container and ours is the opposite; `materialize` lost to `alloc`, which says where the money goes
in a word everyone has. **β-tab is a second CLAUSE of β**, not a fourth rule — a rule is an
intensional presentation of a function and a graph an extensional one — so it needs no constant
folder and comes first. **Mullin's Psi Correspondence Theorem is β**: MoA needs an index calculus
because APL arrays are data, ours are functions, so composing index maps is composing functions.
The leading-axis rule is currying, and MoA's DNF/ONF split is our residual/emission split. And
**automatic unrolling is deferred, NOT refuted** — the measurement only covered cheap elements, the
win scales as compile-cost over artifact-cost, and ADR 0009 bites exactly where the win would be
because transcendentals are not bit-reproducible. **The memory model is DECIDED** —
[ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md), research in
[memory-model.md](docs/memory-model.md). Values are immutable; mutation exists only inside
`(build n (fn (b) …))`, whose buffer is **linear** and frozen on the way out; `(array V)` reads are
pure and `(buffer V)` reads are impure; and **the linearity check is `occurrences` on the residual,
not a type** — uniqueness never enters a signature. What decided it was **expressiveness, not the
2.7x**: `(table n f)` is a *gather* and cannot express a *scatter*, so the sieve, sorting,
histograms, union-find and general DP are inexpressible portably **at any speed** —
`examples/native/sieve-go.oro` is in this repo and could not be written portably. It costs almost
nothing because every mechanism exists already: the heap is acyclic (ADR 0014), a buffer cannot
escape (closures are refused — the only thing Haskell's rank-2 `runST` prevents), it is lexically
local in the residual (whole-program reduction), `occurrences` is in the reducer, and **ADR 0010
already sequences stores** by never substituting an impure argument, denying contraction, weakening
and exchange — the three properties a buffer needs, built for `print-line`. This **fires ADR 0013's
fifth trigger**: η-tab is now sound. Uniqueness types on *parameters* (Futhark, Cogent — the two
languages with our exact constraints, which both chose them) are **deferred with a named trigger**,
because reduction removes every non-exported boundary so buffer reuse already works inside a
program. Reclamation is a **target** decision: three hosts bring collectors, and windows gets a
lexical arena or Perceus, which is available there precisely because we own that allocator.

**The older framing, kept because the reasoning is what changed**: a table is a function from a
finite index set,
and the length is what makes it more than a function. Three constructors — `(array e…)` a graph,
`(vec n f)` a rule with no memory, `(materialize t)` the one construct that allocates — plus
`(len t)`, and **indexing is APPLICATION**: `(a i)`, not `(at a i)`. That is unambiguous by a
checked invariant — in a residual the operator of an application is never a variable, because a
surviving lambda is a refused closure — so the slot is empty and `(a i)` is an indexing before any
type is consulted. `(array T)` exists only in the **signature** language and is erased by staging,
because a dynamic index forces homogeneity and reduction removes every static one, so the checker
only ever sees `Fin n → V` and **no dependent types are needed**. No target declares any of it:
the backends implement it like `if`/`let`/`loop`. It **deletes** surface — Go's 7 array types and
14 `at-`/`make-`/`set-` names, Java's set, and the portable layer's `alen`/`aindex`/`slen`/`sat`,
which never covered int or bool because it *enumerated* element types instead of having a
constructor. Bounds are a **precondition, not a behaviour** (JS returns `undefined` silently).
Reuse is deliberately absent and stays ADR 0013's question.

**The data-structure question is researched and NOT decided** —
[docs/data-structures.md](docs/data-structures.md). Three things to carry. The array is primary
**because a list's algebra needs recursion**, not because a list is slow: `foldr` *is* the list's
universal property, ADR 0014 removed recursion, so a cons chain here would be an array with worse
constants and no compensating law — *a language's iteration construct selects its data structure*.
The product is **not a new kind of thing**: `(vec n f)` and `(fn (sel) (sel x y))` are the same
construct at different index sets, so `materialize` and reifying an escaped product are one
operation, and TLA+, containers, Naperian functors, Dex and SML's `(a,b) = {1=a,2=b}` all say the
same thing — *everything is a function from an index set*. And the recommendation is **multiple
return values, not tuples**: all six measured demands are multiple-return, Scheme and CL made that
exact distinction on purpose, and three of our four targets have a native form. Two results here
were rediscovered independently before being read: the delayed vector is a **container**
(Abbott/Altenkirch/Ghani) and Repa's `Delayed`, and [q5b](docs/spec/q5b-filter.md)'s pull/push
duality is Obsidian's.

**§8 is the literal table, and it is the missing dual** — `(array 1 2 3)` with application as
indexing. `tab`/`idx` are mutually inverse, which IS the Naperian isomorphism written at the term
level, and β-tab is **the extensional counterpart of β**: a function given by its graph rather than
by a rule. It costs no new term kind and no new reduction rule *if constant folding is built*,
since `((array 1 2 3) 1) → 2` is one entry in the same table as `(go.+ 1 2) → 3`. A **dynamic index
forces homogeneity and forces existence, and it is the same condition doing both** — so the
dependent type is erased by staging and the checker only ever sees `Fin n → V`. **The memory half is REFUTED** —
[staticdata-2026-08-20](gauntlet/results/staticdata-2026-08-20.md). Compile-time materialisation
into static data is free of *code* on x86 and Go, a **pure loss** on Java (256 `iastore` in
`<clinit>`) and JavaScript (**3.5x slower to load, 2,600x larger source**), and never a measurable
win — even on Go a 65,536-entry table saves 0.2 ms against 9 ms of process creation and costs
exactly its own size in the binary. So `unroll` should not be built, D-K is demoted to a syntax
question, and **the next build is multiple return values.** And **η-tab is a law we can state and are not
allowed to apply**: `materialize (of-array a) = a` is true of values and unsound with mutation,
which is ADR 0013's fifth trigger.

**Types are not in the language and that is measured, not assumed** — `targets/js.oro` declares
zero types because JS needs none. A type system is *wanted eventually*, and
[docs/types-direction.md](docs/types-direction.md) records the direction and the one measurement
that constrains it: **our proofs do not transfer** — a proof buys nothing unless the emitted code is shaped so the
host re-proves it. That win has been **collected as an emitter pattern**, needing no types at all
([bce-2026-08-15](gauntlet/results/bce-2026-08-15.md)): 1.96× on compute-bound loops, and
**nothing** on memory-bound ones, which is the condition the earlier "1.94×" left off.

## Working conventions

**Every significant decision gets an ADR.** Numbered, in `docs/decisions/`, using the template
in that directory's README. The "Why not" section is the point — this project is deliberately
put down at dead ends and picked up later, and the rejected alternatives are what will not be
recoverable from the code.

**Reversing a decision means a new ADR that supersedes the old one.** Mark the old one
`Superseded by NNNN`. Do not edit decision history.

**Keep documents current rather than accumulating drafts.** A stale design document is worse
than no design document. `docs/design-direction.md` was rewritten, not appended to, when the
Parasite reframing invalidated part of it.

**A target need not have expressions** — [ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md).
`targets/windows/` emits x86-64 assembly under MASM and reaches **parity with hand-written
assembly** ([windows-2026-08-19](gauntlet/results/windows-2026-08-19.md)), with the structural set
still three. The format gained `%r`, `%u`, `(jump …)` and `(data …)` and lost nothing. Two things
to carry forward: a template may not touch r10, r11, xmm4 or xmm5, and **the optimisations you
were parasitizing only become visible on a host that has none** — the first three hosts were doing
common-subexpression elimination for us and nothing noticed until one did not.

**The gauntlet runs on the native Go target now** —
[native-gauntlet-2026-08-20](gauntlet/results/native-gauntlet-2026-08-20.md). All six programs
moved off `num/vec`/`num/int`/`num/f64`/`io`, all at parity. Two things the move deleted and one it
added: **`fold-range2` is gone** — it existed only because two accumulators had no product to pair
them with, and `loop` has n variables and no product at all — and the native Go target **had no
string surface at all** until `wordcount` asked for one (`targets/go/strings.oro`, 25 primitives,
no compiler change). What it added is `(length N)`, a declared primitive attribute saying which
argument decides the result's length; the sieve's `c[i]` is **proven** now rather than propagated.
It is **two** attributes — `(length N)` for a count and `(length-of N)` for a pass-through — because
reading it off the argument's declared type broke on the first target that tried it: `targets/js/`
declares everything `any`. And a **JavaScript array store is a map insert** — `a[10] = x` on a
three-element array extends it — so `js.set` declares neither, which is `go.set-map`'s refusal
arriving from the opposite direction.
**JavaScript is done too** — [native-js-2026-08-20](gauntlet/results/native-js-2026-08-20.md).
All six at parity there as well, and the hostile host earned its reputation: a `loop` in **tail
position** was lowering to a result variable plus `break`, which costs **1.31x on V8** against an
early `return` and **nothing on Go** — five months of Go measurements could not see it. It also
found two benchmark-method errors that had been inflating every JS number here: reaching a function
through a module namespace object (`g.dotTyped`) is **1.66x** slower than a named import, and the
old harness did that on the hand-written side only; and V8 carries optimization state across
benchmarks, so each comparison now gets its own process. **Java has not moved.**

**Two backends before front-end features.** Once the Go backend works, build the JavaScript
backend before adding anything to the language. JS is the most hostile host in the set and
surfaces core flaws while they are still cheap to fix.

## Build commands

Module path is `oroboros` (local). Change it when the repository gets a home.

```bash
go test ./core/ ./emit/          # the compiler
go test ./core/ -run TestBeta    # one test
go vet ./...
```

```bash
go run ./cmd/build -target=portable-go -o hello examples/hello.oro   # a real binary
go run ./cmd/oro -target=portable-go examples/dot.oro   # reduce to normal form
go run ./cmd/oro -target=blas -steps examples/dot.oro
```

```bash
# emit into the gauntlet, then benchmark generated against hand-written
go run ./cmd/gen examples/dot.oro portable-go gauntlet/go/generated_dot.go
go run ./cmd/gen examples/dot.oro js   gauntlet/js/generated_dot.mjs
go run ./cmd/gen examples/dot.oro java gauntlet/java/gen/GenDot.java
cd gauntlet/go && go test -bench='SmallDot|SmallGenDot' -benchtime=3s -count=5
```

The target file format is specified in [docs/spec/target-files.md](docs/spec/target-files.md) —
it is the file a third party writes, so it is the one that most needs to be a specification rather
than a comment.

A **doctor** — reporting which toolchains a target needs, which are installed, and what is
missing — is wanted and deliberately not built yet
([build.md §6](docs/spec/build.md)). It can only diagnose requirements that exist, and what a
target must declare about its toolchain should be read off what builds turn out to need.

**Primitives are declared in `targets/*.oro`, not in Go.** If you find yourself adding a case to
`emit/*.go` for a host function, that is the wrong place — only *structural* primitives (loops,
conditionals, bindings) live in code.

The gauntlet (`gauntlet/go`, `gauntlet/js`, `gauntlet/java`) and `experiments/legibility` are
**separate modules** — `cd` into them before running their tests.

`gauntlet/fmt/*.go` carry `//go:build ignore`; they are standalone scripts run with `go run`.

## What exists

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer. The atom of [core-0](docs/spec/core-0.md). |
| `emit/` | Go, JavaScript, Java and x86-64 backends. Types live here, **not** in the language. |
| `targets/` | Target declarations — **data, not Go**. `go/`, `js/`, `java/` and `windows/` are **directories**, host-native, no portability claim ([target-native.md](docs/spec/target-native.md), [windows-target.md](docs/spec/windows-target.md)); the `portable-*.oro` files are the layer they replaced, kept for the gauntlet. |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet's Go package |
| `cmd/build` | follow imports, reduce `main`, emit a program, run the host toolchain |
| `examples/` | twelve programs; `smooth.oro` completes the gauntlet |
| `lib/` | modules a program imports by `(use …)`; resolved on a search path |
| `gauntlet/` | hand-written references and results — the bar |

**Both emitted programs reach parity with hand-written Go.** See
[parity](gauntlet/results/parity-2026-08-14.md).
