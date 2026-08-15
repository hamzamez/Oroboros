# 0013 — Accept the allocation price, for now

Date: 2026-08-15
Status: Accepted — **provisional, and expected to be superseded**

## Context

`examples/smooth.oro` completed the gauntlet and failed it
([stencil-2026-08-15](../../gauntlet/results/stencil-2026-08-15.md)).

Generated code is at parity with hand-written **functional** code — 123,485 ns/op against
124,188, 0.6 % apart. It loses to a hand-written stencil that **reuses buffers**:

| | Go | JavaScript |
|---|---|---|
| hand-written, caller owns the destination | 544,610 ns/op, **0 allocs** | 663.8 µs/op |
| generated | 975,568 ns/op, 8 × 512 KB | 1,335.9 µs/op |
| | **1.79×** | **2.01×** |

The compiler is not what loses. `materialize` is: it allocates fresh so that the destination is
**unique by construction**, which is how g7's *program* landed without g7's *question* — aliasing —
being answered.

## Decision

**Accept the price. Add no ownership property and no reuse analysis.**

A program that applies a stencil repeatedly allocates once per pass and runs at roughly **1.8× to
2.0×** the hand-written buffer-reusing form, on every target.

This is **provisional**. It is written down as a decision rather than left implicit precisely so
that it can be found and reversed, and it is expected to be.

## Why not

**(a) In-place write with an ownership discipline.** `(set! a i x)` plus a proof that `a` is
uniquely held. It is the only option that reaches the hand-written 544,610, and it is the honest
answer to g7's original question.

Rejected *now* because it costs a **uniqueness or linearity property on values** —
the thing [ADR 0010](0010-effects-as-structural-rules.md) declined for effects, on the grounds that
what needed the discipline was effects rather than values, and that the structural rules made
conditional on purity were enough. Adding it here would reverse that reasoning, and it would grow
the *interface*: uniqueness is visible to the programmer in a way `pure` is not.

It is also premature: it is far cheaper to express once a type system exists, because a uniqueness
refinement is a short step from a range refinement, and it would be foolish to invent a second
mechanism weeks before the first one arrives.

**(b) Liveness-based reuse.** `v = smooth(v)` with the old `v` dead afterwards could reuse its
storage rather than allocating — Perceus-style, explored in
[s3](../derivations/s3-cross-boundary-reuse.md).

Rejected *now* because it is the cheapest option in **interface** terms — nothing user-visible at
all — and the most expensive in **analysis** terms. Reduction is whole-program, so the analysis is
tractable, but it is a real optimisation pass and this project has never had one. Every performance
result so far has come from *emitting a better shape*, never from analysis. Starting that habit
deserves its own argument, and its own measurement.

It also has the failure mode the type-system discussion named from the other side: when it does not
fire, the programmer cannot see why. An optimisation that silently does not happen is the thing
[bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already required a diagnostic for.

**(c) Denying the problem** — arguing 1.8× is within tolerance. It is not. Requirement 5 is *no
compromise on performance*, measured against hand-written code, and a hand-written stencil reuses
buffers. Calling this a pass would mean redefining the bar to whatever we can currently hit, which
is the failure mode the gauntlet exists to prevent.

## Consequences

Oroboros is, today, a language in which **nothing can alias and array-producing code costs about
twice hand-written**. That is a coherent product and it should be described that way rather than
quietly.

It is the project's **first failed gauntlet program**, and it stays failed. `CLAUDE.md` says so at
the top.

### What should reopen this

Named now, because the reason to write a provisional decision down is so the trigger is not left
to memory:

1. **A type system exists.** Uniqueness becomes a refinement rather than a new mechanism, which
   collapses most of (a)'s cost. This is the expected trigger.
2. **A second program shows a worse ratio.** 1.8–2.0× is measured on one kernel. A program
   dominated by allocation rather than arithmetic would be worse, and would move this from a
   tolerable tax to a wall.
3. **A memory-constrained target.** Android is [ADR 0004](0004-first-targets.md)'s reason for the
   JVM, and allocating 512 KB per pass is a different proposition on a phone than on a laptop.
   Requirement 6 is about footprint, and this decision spends it.
4. **The gap grows on a host we add.** It is already worse on JavaScript (2.01×) than on Go
   (1.79×), which is what GC pressure predicts. A host with a weaker allocator would be worse
   still.

Any one of those should produce an ADR superseding this.
