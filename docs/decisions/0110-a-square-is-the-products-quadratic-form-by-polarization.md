# 0110 — A square is the product's quadratic form, by polarization

Date: 2026-11-14
Status: Accepted (hamza, 2026-11-14: "go ahead with squaring, always mathematical and algebraic").
Extends [ADR 0109](0109-a-product-is-evaluation-and-interpolation-and-its-levels-are-a-tensor-power.md).

## Context

After ADR 0109, fib(10⁶) by fast doubling was 1.67–1.79× Go. Its products are two squares and one
product a bit, and Go squares with half the work (`nat.sqr`, `karatsubaSqr`).

## Decision

1. **x·x of one value is the square**, planned as K(ℕ\*)'s `sqr` (`ir/limbs.go`). A residual's
   binders are named apart (ADR 0036), so one SSA value used twice is one integer. The identity of
   the operands is a fact of the term, decided statically.
2. **The square is μ's quadratic form, computed by polarization** (karatsuba-research.md §12):
   - a² = Σ aᵢ²B²ⁱ + 2Σ_{i<j} aᵢaⱼBⁱ⁺ʲ, which is m(m + 1)/2 digit products against m²;
   - below the threshold, the triangle by rows, then one pass of 2T + the diagonal, each step below 3B
     with the carry in [0, 2];
   - above it, Karatsuba's diagonal: a square's three evaluations are squares, so one operand table
     goes down, the leaves are squares, and the up pass is the product's.
3. **Each pass reads only frozen tables.** A guarded read of a live buffer, followed by a store, makes
   the state translation copy the step into the guard's arms. An arm the facts decide is dead, and
   its values are left untyped, which W14 refused. So the triangle is a table of its own, and the
   diagonal pass builds the square from it.

## Why not

- **A run-time test of operand equality** (GMP's pointer test): the language has no pointer, and
  equal values computed apart are a theorem the analysis would have to prove. The static identity is
  what is decidable.
- **Reading the triangle back from its live buffer**: the dead arms above. The extra table costs one
  allocation a square and no pass, since the diagonal pass is the copy.
- **The interval of a square, [l, h] ↦ x²'s image, now**: it is exact, and the decision lacks it, so
  `fn sq(x) (Nat) { x * x }` is refused for an unbounded x. It waits on a program claiming a square's
  set.

## Consequences

- A square of about 10,400 digits takes 0.54 of a product's time, the algebra's half plus the
  diagonal. fib(10⁶) by fast doubling goes 34 → 30 ms (squaring-2026-11-14), 1.79× → 1.57× Go in
  that sweep.
- `big-sqr`, a differential case on four targets. `PlantSquareFault` drops the factor 2, and
  every square of two digits or more goes wrong.
- **Measured against `math/big` per operation**: our square is 3.1× Go's and our product 4.1×, at
  10,402 words (4.2× and 3.3× re-measured on a quiet machine). The leaf row's own gap is 1.3–1.6×,
  and the rest is the Karatsuba design: larger leaves, copies, the additive form
  (kernels-research.md §7).
