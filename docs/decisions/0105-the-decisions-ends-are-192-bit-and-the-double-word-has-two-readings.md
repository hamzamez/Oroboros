# 0105 — The decision's ends are 192-bit, and the double word has two readings

Date: 2026-11-12
Status: Accepted (hamza, 2026-11-11, "go ahead with M4", and 2026-11-12, "keep going": M4c, B = 2⁶⁴,
step 1). **Amends [ADR 0103](0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md)**:
the double word's domain is (−2¹²⁷, 2¹²⁷) ∪ [0, 2¹²⁸), no longer E.

## Context

carrysplit-2026-11-12 measured a row of 64-bit digits at 0.37 ns per digit against 1.11 for 63-bit
ones, because the carry is the product's high half and needs no `shld`. A base-2⁶⁴ step reaches
B² − 1 = 2¹²⁸ − 1. The decision's ends were E = (−2¹²⁶, 2¹²⁶), so that step was +∞ and could not be
proven.

## Decision

1. **An end is 192-bit two's complement, E = (−2¹⁹⁰, 2¹⁹⁰)** (ir/endpoint.go, its only
   representation). Two bits of headroom keep a sum of two ends and a negation exact in the
   representation, as before; a product of two int64 ends is exact in 128 bits, and the rest go
   through math/big.
2. **The double word has two readings.** The residue map ℤ → ℤ/2¹²⁸ is injective on any interval of
   2¹²⁸ integers, so i128 holds exactly:
   - an intermediate in (−2¹²⁷, 2¹²⁷), read signed;
   - an intermediate in [0, 2¹²⁸), read unsigned.

   Each value is read by its own fact, and `inDouble` is the union. The printer writes `nsw` only on
   a result inside (−2¹²⁷, 2¹²⁷), and `nuw` on naturals. A comparison's operands share a reading:
   both naturals, compared unsigned, or both inside the signed range.

## Why not

- **Four-limb ends (2²⁵⁴)**, as ADR 0104's "Why not" first said. Three limbs hold 2¹²⁸ with headroom,
  and the decision is no slower (the check: 1.01× compile time).
- **Keep E and add a 129th bit only to the double word.** E is what every fact lives in. A double word
  wider than E would hold values the decision cannot state.
- **Read the double word signed only.** A natural step of base 2⁶⁴ passes 2¹²⁷.

## Consequences

- Facts that were +∞ between 2¹²⁶ and 2¹⁹⁰ are now finite. The corpus is byte-identical, refusal
  texts included.
- Steps 2–4 of M4c (CLAUDE.md): digits in U₆₄ as the double word's operands and results, then the
  library at B = 2⁶⁴.
