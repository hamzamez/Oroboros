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

Full reasoning, including rejected designs and the diagnosis of why the predecessor project
hit a performance wall, is in [docs/design-direction.md](docs/design-direction.md).

## Next steps

1. Freeze the core on paper — primitives, types, control flow, capability declarations.
2. Specify the IR file format. This is the backend interface and the hardest thing to change
   later.
3. Reader, printer, and canonical formatter for s-expressions.
4. Front end: functions, locals, structs, range-typed scalars, structured control flow.
5. Go backend, running a non-trivial program.
6. JS backend, **before** adding any front-end features.
7. Binding format, validated against `fmt` on Go and the DOM on JS.
8. Benchmark harness against hand-written Go and JS, wired into CI as a gate.

Steps 6 and 8 are the ones most likely to be skipped, and the two that determine whether the
architecture holds.

## Name

The predecessor was called **Parasite**, after the strategy. **Oroboros** — the serpent eating
its own tail — for the intended endpoint: a language whose compiler is eventually written in
itself.
