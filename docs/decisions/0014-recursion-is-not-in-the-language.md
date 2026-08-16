# 0014 — Recursion is not in the language

Date: 2026-08-16
Status: Accepted

## Context

Recursion **reduced correctly and could not be compiled**, and had done since the reducer was
written. δ declines to unfold a cycle — the standard partial-evaluation answer, and the right one,
since `Y f` has no normal form — so the residual keeps the name. No backend emits one. For a day
the gap was reported at the emitter ([def.md §9](../spec/def.md)), which left the language in an
incoherent state:

```
oro   examples/countdown.oro   →  (countdown 3)          accepted
build examples/countdown.oro   →  no backend emits it    refused
```

**If it is not allowed it should not compile; if it is allowed it should build.** A construct the
reducer accepts and the compiler refuses is a promise the language does not keep.

There are exactly two ways to close it. Emit recursion, or reject it.

## Decision

**Reject it.** A definition that is defined in terms of itself is an error, reported before
reduction, by every command.

```
countdown is recursive: it is defined in terms of itself, and recursion is not in the
language — iteration is fold-range (ADR 0014).
  If the self-reference was not deliberate, this definition is shadowing the countdown you meant.
```

Three properties of where the check lives:

1. **Before reduction, not at the emitter.** So `oro`, `gen` and `build` agree, which was the whole
   complaint.
2. **After the target is known.** Whether a name is recursive is a *per-target* question. A target
   that provides the name natively never unfolds the definition, so the cycle is unreachable —
   ADR 0002's "compiling up". `markRecursive` now skips names in `P_T`, and a library may define
   `sort` recursively for targets that lack it without breaking targets that have it. Rejecting at
   read time would have broken exactly the mechanism the project exists for.
3. **Over every definition, not only reachable ones**, matching the scope check. That is what turns
   a typo'd self-reference — `(def size (fn (v) (size v)))`, meaning some *other* `size` — from a
   term that silently reduces to itself into an error at the line it is written on.

The check is one method, `Env.CheckProgram`, holding both scope and recursion, because the
two-call-site version of the recursion check drifted from the scope check within a day of being
written.

## Why not emit it

Emitting recursion is not hard — collect the recursive definitions a residual reaches, reduce each
body, emit one host function per definition. It is *wrong now*, for a reason that is already a rule
here:

> **Nothing goes in without a specification saying how it behaves on every target**, and if the
> targets disagree observably it is Tier 2 and carries no portability claim
> ([primitives.md](../spec/primitives.md)).

Recursion's observable behaviour includes **how deep it can go before it fails**, and the three
initial targets disagree by orders of magnitude: Go grows stacks dynamically to a 1 GB default,
the JVM overflows at a few thousand frames depending on `-Xss`, and JavaScript engines at a few
thousand more or fewer with no specified limit at all. None of them guarantees tail calls
([def.md §10](../spec/def.md)), so nothing recovers the difference.

Emitting recursion today would therefore ship the first construct that looks Tier 1 and is not: the
same source, the same input, terminating on one target and crashing on another. Rejecting it is not
the cheap answer that happens to be available — under the project's own admission rule it is the
only available answer.

**The cost of rejecting is small**, which is what makes this affordable. Iteration here is
`fold-range`, a primitive that lowers to a `for`. No gauntlet program, no example and no library
function is recursive. Nothing was using it.

## Why not leave it reducing-but-not-building

That is where it was. It is defensible only if the boundary is principled — `oro` is a reducer,
`build` is a compiler, and a reducer may legitimately show you a normal form. But the boundary
here was not principled: it was the accident of no backend having been written. A language whose
demonstration tool accepts more than its compiler teaches the wrong thing about itself.

## What reopens this

Recursion is wanted eventually, for the shapes a counted fold cannot express — while-loops, tree
walks, variable trip counts. The order is fixed by the reasoning above:

1. **A while-shaped loop primitive.** `fold-range` is counted; this is also
   [def.md §10](../spec/def.md)'s prerequisite for tail-call optimisation, and it is the cheaper
   half of what recursion is wanted for.
2. **A specification of depth**, or an opt-in `tailrec`-style marker that is *checked* and rewritten
   to that loop — which makes depth a non-question, because there are no frames. Kotlin and Scala
   do exactly this, and for exactly this reason: the VM will not do it for them.
3. Then, if anything is left over, general recursion as a Tier 2 construct with its depth
   difference written down.

A superseding ADR, not an edit to this one.

## Consequences

- `core.RecursiveNames` and its two call sites in `cmd/gen` and `cmd/build` are gone; the check
  happens once, earlier, for everyone.
- `Env.CheckScope` became `Env.CheckProgram` and does both checks.
- `Env.Rec` stays. δ still declines to unfold a cycle, because reduction must remain correct on a
  term the check has not run over — tests reduce such terms directly.
- [Chapter 2 §2.8](../book/02-def.md) rewritten: it taught the stuck residual, and now teaches the
  error and `fold-range`.
- [core-0 §6](../spec/core-0.md)'s "stays in the residual as a target function" is superseded by
  this. It was written as a prediction and never run.
