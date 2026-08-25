# 0008 — Parasite decisions are per-target measurements, not principles

Date: 2026-08-13
Status: Accepted — refines [0002](0002-capability-graph.md)

## Context

[ADR 0002](0002-capability-graph.md) established the governing rule: **emit at the highest layer
the target natively provides.** Five hand-derivations then built principles on top of it —
which construct to emit, which capability granularity to choose, where the host would and would
not optimize.

The [first baseline run](../../gauntlet/results/baseline-2026-08-13.md) refuted five of them:

| Belief | Measured |
|---|---|
| Emit `Map` on JS, as the native dictionary | A null-prototype `Object` is **3.25× faster** for string keys |
| Never split a capability finer than the host's fused idiom | On Java the *unfused* `getOrDefault`+`put` beat fused `merge` by **2.6×** — and on JDK 17 the fused form is **1.19× faster**, so the measurement itself moved ([native-java-2026-08-25](../../gauntlet/results/native-java-2026-08-25.md)). The rule this ADR states is what survives; its example is not |
| Java's `Point[]` fails like JS's array-of-objects | Java **1.05×**, JS **2.86×** — HotSpot lays the objects out contiguously |
| Go won't inline a generic fold through a func-value parameter | All three hosts inline it, at **identical** speed |
| Range types beat hand-written Go by removing a bounds check | Check verifiably removed, **zero** measurable gain |

Every one was a plausible inference from how the host is documented to work. Every one was
wrong, and none would have been caught by argument.

## Decision

ADR 0002's rule stands as the **default and the prior**, not as a derivation.

Every specific parasite decision — which host construct implements a given capability on a
given target — is a **recorded measurement**, and the capability's target mapping carries a
reference to the benchmark that justifies it.

Corollaries:

- The gauntlet is **permanent infrastructure**, not a one-time acceptance gate.
- Adding a target means *measuring* its constructs, not merely mapping onto them.
- A parasite decision with no measurement behind it is provisional and must be marked so.
- Host compilers are to be treated as **black boxes with measured behaviour**, never as
  specifications. Documented inlining rules, elimination passes, and cost models are evidence
  about intent, not about output.

[g3](../derivations/g3-generics.md)'s principle — never parasitize the host's abstraction
mechanisms — survives only in its measured form: **all three hosts perform the same
specialization we do, under the same condition, that the callee be literal at the call site.**
We win only above the host's inlining budget, or where the callee is not statically known.

## Why not

**Keep deriving from principle.** Refuted five times in the first hour of measurement. The
failure rate is the argument.

**Abandon the principle and measure everything.** The combinatorial space of capability ×
target × construct is too large to measure exhaustively, and "emit at the highest layer" remains
the correct prior — it was right about Go's `map`, about `strings.Fields`, and about escaping
closures. A prior that is usually right and cheaply falsifiable is worth keeping.

**Trust host documentation instead of benchmarks.** Go's inliner cost budget is documented and
its behaviour here was still the opposite of what the documentation implied to us.

## Consequences

- Capability definitions grow an evidence field. This is real ongoing cost, and it is the price
  of the performance claim being true rather than asserted.
- Baselines must be re-run when a host toolchain updates, since a parasite decision can be
  invalidated by someone else's compiler release. Version-stamp every recorded result.
- Benchmark hardware quality becomes a project concern. The first run was taken on a hybrid
  P/E-core laptop with ~15% noise, which is too coarse to resolve several of these decisions.
- **This makes the project slower and its claims defensible.** That trade is the whole point of
  [ADR 0007](0007-exploration-over-specification.md).
