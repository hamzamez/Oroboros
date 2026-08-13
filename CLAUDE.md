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

None yet — there is no `go.mod` and no Go source. The module path is an open decision tied to
where this is hosted; pick it when creating `go.mod`, while there are zero imports to update.

Once the compiler exists, this section should carry the actual build, test, and
single-test-run commands.
