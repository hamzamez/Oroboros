# 0013 — Accept the allocation price, for now

Date: 2026-08-15
Status: Accepted — **provisional, and expected to be superseded**

> ## Correction, 2026-08-15
>
> **Trigger 1 fired and its reasoning was wrong.** A type system now exists
> ([types.md](../spec/types.md), [refinements.md](../spec/refinements.md)), and it does **not**
> make option (a) cheap. This ADR claimed:
>
> > *"a uniqueness refinement is a short step from a range refinement"*
>
> That is false, and the distinction is not a detail. **A refinement constrains a value**;
> `{i | 0 ≤ i < n}` says something about `i`. **Uniqueness constrains the context** — how many
> other references exist — which no predicate on a value can express. `(set! a i x)` needs `a` to
> be *consumed*, and a refinement system will happily let the program use `a` afterwards and
> observe the mutation. That is why Rust uses ownership rather than predicates, and why ATS
> stratifies linear types away from refinements.
>
> **The decision below is unaffected.** The price is still accepted and still provisional; only one
> supporting claim was wrong.
>
> **Where the mechanism probably does live**, and this is a hypothesis rather than a finding:
> uniqueness is *substructural*, and this project already has a substructural mechanism.
> [ADR 0010](0010-effects-as-structural-rules.md) made the structural rules conditional on purity,
> and the reducer already computes `occurrences`, which decides *used at most once*. That is much
> closer to option (b) than to (a).
>
> It is written as a hypothesis on purpose. The original trigger asserted an enabling relationship
> that had not been tested, and asserting a second one the same way would repeat the mistake.

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

~~It is also premature: it is far cheaper to express once a type system exists, because a
uniqueness refinement is a short step from a range refinement, and it would be foolish to invent a
second mechanism weeks before the first one arrives.~~ **Wrong — see the correction above.** The
original text is kept because the rejected reasoning is the part an ADR exists to preserve.

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

1. ~~**A type system exists.** Uniqueness becomes a refinement rather than a new mechanism.~~
   **Fired 2026-08-15, and the reasoning was wrong** — see the correction. Replaced by:
   **someone decides to spend a substructural analysis.** Uniqueness is not a refinement; the
   nearest existing machinery is ADR 0010's purity-conditioned structural rules together with the
   reducer's occurrence counting. Whether that suffices is **unmeasured**, and this trigger should
   not be treated as fired until it is.
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
