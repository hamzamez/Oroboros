# 0001 — Targets are ecosystems; portability is a program property

Date: 2026-08-13
Status: Accepted

## Context

The predecessor project was named **Parasite**, and the name was the thesis: the language
should take maximum advantage of whatever the host ecosystem already offers. If the best way
to build a Windows app is Win32, use Win32. If it is .NET, use .NET. If Android is best served
by Kotlin, or by the JVM, or by the NDK, the answer is whichever is actually best — and
possibly all three, as separate targets.

The first draft of the design direction assumed the opposite: a core that is the *intersection*
of all hosts, with everything lowering to it and portability guaranteed by construction. That is
the Java/Shen/WASM model.

## Decision

Targets are **ecosystems to exploit**, not machines to abstract over.

Portability is a **property a program may or may not have**, computed and reported by the
compiler — not a guarantee the language enforces globally.

A program using only portable capabilities is portable. A program using Win32 is not, and that
is a legitimate, supported, first-class thing to write. When a program requires something a
target lacks, that is a build error naming the specific gap, resolved by adding a binding,
writing a shim, or accepting that the program does not target that platform.

## Why not

**Portability-first (intersection core).** The intersection of Go, JS, the JVM, Win32, and
embedded C is nearly empty, and anything reached through an FFI escape hatch is treated as
second-class. It also loses on performance: lowering a hash table through to Go's core is
slower and larger than emitting Go's `map`. The model forces exactly the compromise the
project exists to avoid.

**Per-target dialects with no shared core.** Gives up requirement 7 (abstraction accumulating
over time) and makes shared libraries impossible. Portability should be optional, not absent.

## Consequences

- The fixed layer tower does not survive. See [0002](0002-capability-graph.md).
- A capability/requirement system becomes mandatory infrastructure, not a nicety — it is the
  only thing that makes "portable or not" checkable rather than a matter of hope.
- Adding two targets for one ecosystem (Java *and* Kotlin) is normal, not contradictory.
- **Risk:** Haxe, the closest existing system, suffers real per-target standard library
  divergence — N dialects under a thin shared syntax. Mitigated by allowing a library to be
  declared portable, after which the compiler rejects non-portable capabilities within it.
