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

**Current standing** is in [docs/assessment-2026-08-13.md](docs/assessment-2026-08-13.md).
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
code rather than from memory. Six term kinds, five top-level forms, two reduction rules, two
parameters.

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
| `emit/` | Go, JavaScript and Java backends. Types live here, **not** in the language. |
| `targets/` | Target declarations — **data, not Go**. `go/` is a **directory**, host-native, no portability claim ([target-native.md](docs/spec/target-native.md)); `portable-go.oro` is the layer it replaced, kept for the gauntlet. |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet's Go package |
| `cmd/build` | follow imports, reduce `main`, emit a program, run the host toolchain |
| `examples/` | twelve programs; `smooth.oro` completes the gauntlet |
| `lib/` | modules a program imports by `(use …)`; resolved on a search path |
| `gauntlet/` | hand-written references and results — the bar |

**Both emitted programs reach parity with hand-written Go.** See
[parity](gauntlet/results/parity-2026-08-14.md).
