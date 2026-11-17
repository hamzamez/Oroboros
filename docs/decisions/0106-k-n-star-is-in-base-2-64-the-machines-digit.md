# 0106 — K(ℕ\*) is in base 2⁶⁴, the machine's digit

Date: 2026-11-12
Status: Accepted (hamza, 2026-11-12: "go ahead", the next step explained). **Reverses
[ADR 0104](0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md) item 1** (B = 2⁶³),
on [ADR 0105](0105-the-decisions-ends-are-192-bit-and-the-double-word-has-two-readings.md)'s wider
ends. **Item 1 amended by [ADR 0124](0124-k-n-stars-base-is-the-triples-native-word.md)**: the base is the triple's native word, 2⁶⁴ on
x86_64 and 2³² on i686.

## Context

At B = 2⁶³, every step's split into ⌊t/B⌋ and t mod B cost a `shld` and an `and` on its carry chain.
A row with 64-bit digits ran 3.0× faster per digit in C, its carry the product's high half
(carrysplit-2026-11-12). ADR 0105 widened the decision's ends to 2¹⁹⁰ and gave the double word an
unsigned reading, [0, 2¹²⁸), which a base-2⁶⁴ step needs.

## Decision

1. **ℕ\* is in base B = 2⁶⁴; a digit is a natural of the unsigned word.** Every step is an intermediate
   of the double word, read back by its halves, t & (2⁶⁴ − 1) and t >> 64: A and M read unsigned, S
   signed.
2. **The double word chooses before the unsigned word**, and claims only values outside both words.
   The unsigned word leaves its operations alone: their operands are extended by `zext` or `sext` by
   their facts, and a reading back into U₆₄ is proven where the target realizes U.
3. **A table's elements are ρ of their hull** (ADR 0083 item 5): the hull of the stored facts, taken
   from the table after each store. A hull in U and outside S holds U₆₄ elements.
4. **The sets a call is decided under are read exactly**, past int64 and `u64` included.

## Why not

- **Stay at B = 2⁶³**: measured 3× slower per digit step, by the machine code's `shld`.
- **The halves as host operations** (`mulq`'s rdx:rax, `adc`), with the decision unwidened. The
  decision would not see the step (ADR 0104's "Why not"). ADR 0105 cost 1.01× compile time instead.
- **Let the unsigned word claim the steps it can** (a sum below 2⁶⁴). At B = 2⁶⁴ no step is below
  2⁶⁴, and two passes claiming one value is two answers to one question.

## Consequences

- The product's row is the machine's: `mulq`, `add`, `adc`. The programs gained less: the 2,000
  quotients 0.78× Go, the product 1.8×, fib 1.9×, fact printed 3.2×. The cost moved to storage across
  calls and to the sum's carry (nat64-2026-11-12).
- The add-back witness is constructed anew for B = 2⁶⁴.
