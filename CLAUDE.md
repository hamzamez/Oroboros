# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## Project state

Design phase. **There is no code yet.** The repository currently contains only design
documents. Do not infer architecture from the file tree — read the documents.

Start with [README.md](README.md), then [docs/design-direction.md](docs/design-direction.md),
then the ADRs in [docs/decisions/](docs/decisions/).

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
go run ./cmd/oro -target=go examples/dot.oro     # reduce to normal form
go run ./cmd/oro -target=blas -steps examples/dot.oro
```

```bash
# emit Go into the gauntlet, then benchmark generated against hand-written
go run ./cmd/gen examples/dot.oro go gauntlet/go/generated_dot.go gen-dot
cd gauntlet/go && go test -bench='SmallDot|SmallGenDot' -benchtime=3s -count=5
```

The gauntlet (`gauntlet/go`, `gauntlet/js`, `gauntlet/java`) and `experiments/legibility` are
**separate modules** — `cd` into them before running their tests.

`gauntlet/fmt/*.go` carry `//go:build ignore`; they are standalone scripts run with `go run`.

## What exists

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer. The atom of [core-0](docs/spec/core-0.md). |
| `emit/` | Go backend. Types live here, **not** in the language. |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet's Go package |
| `examples/` | `dot.oro`, `filter.oro` |
| `gauntlet/` | hand-written references and results — the bar |

**Both emitted programs reach parity with hand-written Go.** See
[parity](gauntlet/results/parity-2026-08-14.md).
