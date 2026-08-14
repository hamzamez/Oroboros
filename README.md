# Oroboros

A small language and build system that **parasitizes** target ecosystems instead of
abstracting over them.

**Status: design. No code yet.**

---

## The idea

Most portable languages define a core that is the *intersection* of every host, then treat
anything platform-specific as an escape hatch. That guarantees portability and gives up the
ecosystem — and, historically, gives up performance too.

Oroboros inverts it. If the best way to build a Windows app is Win32, add a Win32 target and
use Win32 fully. If it is .NET, add a .NET target and use .NET fully. Android might be best
served by Kotlin, or the JVM, or the NDK — so have all three as separate targets and pick per
program.

Portability is therefore a **property a program may or may not have**, computed and reported
by the compiler, rather than a guarantee the language enforces globally. A program that uses
only portable capabilities is portable. A program that uses Win32 is not, and that is a
first-class thing to write.

## How it works

Everything runs on one mechanism: a **capability graph**.

- A **capability** is a named, typed unit of functionality — `float64`, `map`, `threads`,
  `matmul`.
- A **module** declares what it requires.
- A **target** declares what it provides natively, plus **shims** implementing one capability
  in terms of others.

Building covers the required set from what the target provides plus what its shims can reach.
Anything uncovered is a build error naming the exact gap.

The rule that makes this fast: **emit at the highest layer the target natively provides.**
Lower only as far as necessary. Go has `map`, so emission stops there and the output uses
Go's `map`. C does not, so the same source keeps lowering into a real hash table.

Two consequences fall out for free:

- A feature the target lacks — floating point on an integer-only machine — is just a shim.
- A feature the target has *in hardware* is just the **absence** of a shim. "Compiling up"
  is not a separate mechanism.

That rule is a prior, not a proof. Which host construct is actually fastest is a measurement —
JS's `Map` turns out to be 3.25× slower than a plain object for string keys, and Java's fused
`merge` loses to the unfused form. See [ADR 0008](docs/decisions/0008-measurement-over-principle.md).

## What the core turned out to be

Hand-derivation of all five gauntlet programs against a candidate "everything is a rewrite" core
produced a sharper answer than the question started with. Rewriting **is** lambda calculus —
one rule, beta, plus alpha — generalized. What separates it from the predecessor project is not
the mechanism but the **stage**:

> **Everything is a function, evaluated at compile time. What survives is what the target must
> do at runtime, and the compiler will tell you exactly what that is.**

Lambda calculus at runtime allocates, because a closure's environment must outlive the
abstraction. The same substitution at compile time costs nothing, because it is gone before the
program runs. Escaping closures are exactly where that fails — and there they cost what the host
charges for the same program, which is the standard.

Following the defects those derivations turned up led to the other half. Every one was naive
rewriting losing something the term held implicitly — sharing, capture-freedom, simultaneity,
effect ordering — and four of them are **structural rules**: when may a term be copied, moved,
or deleted? Grading each term by multiplicity answers all four. And grade 0 — *erased before
runtime* — turns out to be the staging annotation itself. So:

> **Terms, rewritten by rules. Every term graded by how many times it may be used and at which
> stage. Grade 0 means it is gone before the program runs — and the compiler will tell you the
> grade of anything you wrote.**

That last clause is the feature the pitch needs: **statically, whether an abstraction was
eliminated or survived** — "this fold is inlined, no call" versus "this handler survives: 1
allocation, 16-byte environment, 1.55ns indirect call." A checkable answer instead of a hope,
and it falls out of the machinery already required for soundness.

## Design goals

| | |
|---|---|
| Small | The language should be easy to implement. |
| Parasitic | Take maximum advantage of each target ecosystem. |
| Open | Adding a target should be low effort, for anyone, out of tree. |
| Declarative bindings | Adding a target's APIs should be close to a file listing names. |
| Fast | Parity with hand-written code in the target language. Enforced in CI. |
| Small output | Small binaries and footprints. |
| Abstractable | Express more in fewer tokens over time. |
| Legible to models | Easy for LLMs to write and reason about. |

## Decisions made so far

- Targets are ecosystems, not machines — [ADR 0001](docs/decisions/0001-parasite-model.md)
- A capability graph, not a layer tower — [ADR 0002](docs/decisions/0002-capability-graph.md)
- Range-typed integers, mathematical semantics — [ADR 0003](docs/decisions/0003-range-typed-integers.md)
- Go, JavaScript, Java/Android first; C deferred — [ADR 0004](docs/decisions/0004-first-targets.md)
- The compiler is written in Go — [ADR 0005](docs/decisions/0005-implementation-language.md)
- The backend interface is a file format — [ADR 0006](docs/decisions/0006-ir-file-format.md)
- Explore candidates against a fixed test — [ADR 0007](docs/decisions/0007-exploration-over-specification.md)
- Parasite decisions are measurements, not principles — [ADR 0008](docs/decisions/0008-measurement-over-principle.md)
- Staging must not change results — [ADR 0009](docs/decisions/0009-staging-preserves-results.md)

Full reasoning, including rejected designs and the diagnosis of why the predecessor project
hit a performance wall, is in [docs/design-direction.md](docs/design-direction.md).

## How this project is run

The core is **not** specified up front. The predecessor stalled on a fixed language that work
then went into making viable, and committing to a specification now would recreate that.

Instead, one thing is fixed: **[the gauntlet](docs/gauntlet.md)** — five programs that must
reach parity with hand-written code on Go, JavaScript, and Java. Candidate cores are
disposable and expected to die. Every one that dies gets an ADR naming what killed it, so a
dropped direction becomes an accumulating result rather than lost time.

Candidates currently on the table are in [docs/core-candidates.md](docs/core-candidates.md),
with actual syntax. Hand-derivations of each gauntlet program against the leading candidate are
in [docs/derivations/](docs/derivations/), and the measured baselines are in
[gauntlet/](gauntlet/).

The first baseline run **refuted five beliefs** the derivations had reasoned their way into.
That is the process working, and it is why nothing is frozen.

**Where this stands** — after six derivations and one baseline run — is assessed in
[docs/assessment-2026-08-13.md](docs/assessment-2026-08-13.md): the candidate survived both
tests designed to kill it, its costs are known and bounded rather than open-ended, and the three
things that could still kill it are all cheap to test before writing a compiler.

## Where this is

The atom is built and both emitted programs reach parity with hand-written Go.

| | |
|---|---|
| Reducer | `core/` — β and δ, normal form parameterised by the target's primitive set |
| Backends | `emit/` — **Go and JavaScript**. Types live here, not in the language. |
| Measured | Both programs at parity on **both targets** — [Go](gauntlet/results/parity-2026-08-14.md), [JS](gauntlet/results/js-2026-08-14.md) |

```bash
go run ./cmd/oro -target=blas examples/dot.oro   # (fn (p q) (dot p q))
go run ./cmd/oro -target=go   examples/dot.oro   # a loop
```

## Next

**Gauntlet program 2 — structs.** Neither backend has a struct primitive, and
[g2](docs/derivations/g2-structs.md) measured that JS needs a *different representation* from Go
(array-of-objects loses 2.86× there and 1.05× on Java). It is the first program where the two
targets must genuinely diverge, and the first real test of the capability graph rather than of
the reducer.

Open, and deliberately not yet built: call-by-need — which lost its performance justification
when [Go's CSE turned out to do the work](gauntlet/results/duplicate-read-2026-08-14.md) —
compile-time evaluation of primitives, splitting `def` from `rec` ([def.md](docs/spec/def.md)),
Java, and the four unemitted gauntlet programs.

## Name

The predecessor was called **Parasite**, after the strategy. **Oroboros** — the serpent eating
its own tail — for the intended endpoint: a language whose compiler is eventually written in
itself.
