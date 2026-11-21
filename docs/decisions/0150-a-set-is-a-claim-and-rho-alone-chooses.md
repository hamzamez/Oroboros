# 0150 — A set is a claim, and ρ alone chooses the representation

Date: 2026-11-21
Status: Accepted (hamza, 2026-11-21: "2. A an intermediate outside E yes 3. report is enough, a flag
(-words is just a bonus option for the compiler)", on
[sets-are-claims-research.md](../sets-are-claims-research.md)). Reverses
[ADR 0019](0019-precision-by-declaration.md) item 2 (bounded by default) for an intermediate; amends
[ADR 0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) item 3 (a claim
above the word is no longer the consent to the rung) and
[ADR 0148](0148-where-the-box-forgets-a-relation-presburger-arithmetic-decides.md) item 1 (a binder's
`∈ Int` is a claim, and asks nothing the program did not already have). **Amended** by
[ADR 0151](0151-a-representation-holds-a-value-only-where-it-computes-its-uses.md) item 1: ρ the
cheapest representation whose domain holds the fact and whose operations cover the value's uses.

## Context

A declared set had two jobs: a claim, decided or refused, and the consent to hold a value above the
word. The second broke four laws the language already had (the research note §2): the one bridge
between the logic and the computation is ρ of *proven* sets (ADR 0049); inlining changes no verdict
(ADR 0051: a helper's `(Int)` asked nothing while a binder's `∈ Int` asked); a weaker claim is never
refused where a stronger one is accepted (`−2⁶⁴..2⁶⁴ ⊆ ℤ`, and only the smaller asked); and one rule on
every target (whether `Int` asked depended on how a target spelled it). The double word on every host
(ADR 0149) left the consent one question: a value outside E = (−2¹²⁷, 2¹²⁷), refused or computed.

## Decision

1. **A declared set is a claim and chooses nothing.** ρ(v) is the cheapest representation whose domain
   holds v's proven fact, over the chain each target provides: S, U, E, ℤ (the host's integer of any
   size, or K(ℕ\*)). Measured, never asserted (ADR 0008): S and U an instruction, E a few, ℤ an
   allocation.
2. **An operation is never refused for its size.** Its result, outside S, U and E's selection (the
   double word's greatest fixed point, which takes only an intermediate read back into a word), is
   demanded in ℤ (`ir/exact.go`, D_A), the rung's least fixed point holding it and what it needs; the
   program asked for nothing, and the target's own rung is used.
3. **It is reported**: `f: N operation(s) past every word and the double word, held exactly: a * b; …`,
   beside the rung's count. hamza: the report is enough.
4. **A refusal is left only where a value must enter a bounded set**, a sink: a table's index, length
   or element, a map's key or value, a host's word, an entry point's word, a claim (decided at its
   definition, its ends past the word enforced at run time as ADR 0029 enforces them), and `-checked`'s
   trap where a word is required.
5. **`-words`** (`ir.WordsOnly`) holds nothing exactly for its size: an operation past the words and E
   is refused, as before. An option for a program that must allocate nothing.

## Why not

- **B, every set above the word asks** (the research note §3): upward-closed and β-invariant once its
  demand prefers U, and still a second bridge from the logic to the computation, and a claim weakened
  for its side effect.
- **Bounded by default, kept** (ADR 0019 item 2): a refusal guarding cost, not correctness, and the
  verdicts it gave were not one rule on every target (the midpoint compiled on windows and JavaScript
  and not on Go before ADR 0149).
- **A refusal when the allocation is large**: the size is the fact's, and an unbounded fact is the
  case it would refuse; the allocation is the meaning (ℤ), reported, and `-words` refuses it where that
  is wanted.
- **A warning instead of a note**: hamza, "report is enough".

## Consequences

- No program accepted before changes: D_A is empty wherever the decision proved every operation in a
  word or E. A program refused before for an intermediate's size compiles, its operation held exactly.
- The book's chapter 6 teaches S, U, E and ℤ as ρ's chain, and "the ways to clear a refusal" lose
  their first: an intermediate needs nothing.
- A loop counter whose bound the decision does not find is ℤ, reported; the program that wants a word
  states the bound, or builds with `-words`.
