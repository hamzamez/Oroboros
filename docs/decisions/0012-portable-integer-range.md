# 0012 — An `int` is exact within ±(2⁵³−1), and unspecified outside

Date: 2026-08-15
Status: Accepted

## Context

[ADR 0003](0003-range-typed-integers.md) chose *mathematical semantics, machine representation*
and nothing implemented it. Meanwhile the language had **no integer arithmetic at all** — every
primitive was f64, so the index `fold-range` binds was a value nothing could compute with, and
[g7](../derivations/g7-aliasing.md)'s stencil existed as hand-written Go with no `.oro` file
because `(aindex a (sub i 1))` was inexpressible.

Measured, rather than assumed:

| | Go `int64` | Java `long` | JS `Number` |
|---|---|---|---|
| `4503599627370496 + 2` | …498 | …498 | …498 |
| `9007199254740992 + 1` | …993 | …993 | **…992** |
| `3037000499 × 3037000499` | …249001 | …249001 | **…249000** |
| `2⁶³−1 + 1` | wraps negative | wraps negative | **rounds up** |

Three hosts, **three different failure modes at two different thresholds** — and exact agreement
below 2⁵³.

## Decision

An `int` is a **mathematical integer**; arithmetic on it is exact. The **portable range is
−(2⁵³−1) … 2⁵³−1**: a program whose intermediate *and* final integer values all lie inside it
computes the same answer on every target. Outside it the behaviour is the target's and carries no
portability claim.

Spelled `int64` on Go, `long` on Java, nothing on JavaScript. Arithmetic, comparison and equality
live in the modules `num/int`, `num/f64` and `logic` — the root module exports none.

## Why not

**Wrapping at 64 bits.** Would oblige JavaScript to emulate, via `BigInt` or a split
representation, at a cost nobody has measured and no program needs.

**Java's 32-bit `int`.** It wraps at 2³¹, **inside** the portable range. Declaring it would have
been a silent miscompilation on every integer above two billion. The specification caught this
before a line of code was written.

**Go's plain `int`.** 64-bit on our platforms but not guaranteed to be.

**Leaving overflow unspecified entirely, with no stated range.** Then nothing is portable and the
three-question test fails outright. Naming the range is what makes the claim checkable — the same
move [strings.md](../spec/strings.md) makes: *portable because it refuses to claim the region where
the targets diverge.*

**One overloaded `add` resolved by argument type.** The right end state, and it needs a checker
that does not exist. Qualified names cost nothing and unify later without touching a target file.

**Church-encoded booleans.** An encoding erases only when the eliminator's scrutinee is statically
known; a boolean's constructor is a *runtime value*, which is what makes it a conditional. `(c a b)`
puts a variable in operator position, no redex exists, and reduction sticks on a term every emitter
rejects — and Church booleans evaluate **both** branches, which is wrong for effects. Fixing that
needs thunks, which are closures, which allocate.

**Boolean literals, and a seventh term kind.** *Overturned by
[ADR 0017](0017-booleans-are-in-the-language.md) — a reader-level desugaring of `and` has to
PRODUCE a false value and cannot name a target's.* Not needed: `and`/`or`/`not` as expression
primitives need no literal, `if` takes its condition from a comparison, and no refinement needs a
constant.

**`(if a b false)` for `and`.** *Overturned by [ADR 0017](0017-booleans-are-in-the-language.md) —
a fourth host has no `&&` at all, and the cost was measured at zero
([and-form-2026-08-19](../../gauntlet/results/and-form-2026-08-19.md)).* All three hosts have
short-circuiting `&&` natively; lowering to a conditional emits an if-statement and a temporary
where the host has an operator.

**Division, bitwise operations and shifts.** Division's rounding disagrees on negatives and its
zero case traps on two hosts and yields `Infinity` on the third. JavaScript's bitwise operators
coerce to **32 bits**, which would silently contradict the range.

## Consequences

`examples/stencil.oro` became writable, and reaches parity with hand-written Go.

Float equality is **IEEE**, because every host's `==` is. So `(eq NaN NaN)` is false, float equality
is not reflexive, not an equivalence relation, and any refinement mentioning it falls **outside**
the decided fragment and is propagated as an opaque atom rather than used as an equality.

"Stay inside the range" is an obligation on the programmer that **nothing checks** — and it is
precisely a refinement, `{ int | -(2⁵³-1) ≤ n ≤ 2⁵³-1 }`. The specification has a hole exactly
where a type system would later plug it, which is the right shape for a hole to have. It is now the
second such hole, alongside `aindex` being unspecified out of bounds
([primitives.md §2](../spec/primitives.md)).
