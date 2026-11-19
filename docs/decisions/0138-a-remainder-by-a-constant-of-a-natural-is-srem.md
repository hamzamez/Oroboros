# 0138 — A remainder by a constant of a proven natural is the signed instruction

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "agree"). Amends
[ADR 0088](0088-a-target-states-its-realizations-and-a-host-operation-is-declared-as-it-is.md)'s choice
of `udiv`/`urem` on ℕ ∩ S₆₄, per instance. **Built 2026-11-19** ([srem](../../gauntlet/results/srem-2026-11-19.md)):
measured on the corpus first. The corpus decided the rule: a constant **not a power of two**, since
by 2ᵏ `srem` cost the limbs' factorial 1.13× where `urem` is one `and`. The loop measured 0.71×, the
rest within noise. `point(b) && !pow2(b)` is the realization's `where`.

## Context

phi-range-2026-11-17 §4: a loop testing `x % 7 == 0`, its values proven natural, runs at 1.42× C on
x86_64. C writes `srem`, which LLVM folds to a divisibility test, a multiply by 7's inverse and a
compare. We write `urem`, chosen on ℕ ∩ S₆₄ by measurement (K(ℕ\*)'s product, a variable divisor,
0.55× its time). LLVM's loop strength reduction rewrites `urem … == 0` before instruction selection,
and the divisibility fold no longer applies: a full 64×64 `mulq`. With `srem` in its place the
module runs 314 ms against 453, C's 319.

## Decision

1. **On ℕ ∩ S₆₄, `%` and `/` by a constant divisor are realized as `srem` and `sdiv`**: the same
   function there, the dividend proven non-negative and the divisor a positive constant. A variable
   divisor keeps `urem` and `udiv`, ADR 0088's measured choice.
2. **Measured on the corpus before it is accepted**: every constant divisor's site, both ways, so
   the rule is the corpus's and not one loop's (phi-range §4 asks this).

## Why not

- **The divisibility test emitted by us** (multiply by the inverse, compare): independent of LLVM's
  pass order, and more work; the signed instruction already gives C's code.
- **Leave it**: 1.42× on the loop measured.
