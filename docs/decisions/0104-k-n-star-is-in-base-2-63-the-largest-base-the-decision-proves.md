# 0104 — K(ℕ\*) is in base 2⁶³, the largest base the decision proves

Date: 2026-11-11
Status: Accepted (hamza, 2026-11-11: "go ahead with M4"). **Reverses
[ADR 0099](0099-an-unbounded-integer-is-a-numeral-of-any-length-with-its-signed-size-last.md) item 3**
(B = 2³² over U₆₄), which said the wider digit would be measured against it when built. **Amends
[ADR 0103](0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md) item 4**: a divisor
may be a positive word where the triple has a 2-by-1 division and the decision proves the quotient a
digit. **Item 1 reversed by [ADR 0106](0106-k-n-star-is-in-base-2-64-the-machines-digit.md)**: B = 2⁶⁴, on ADR 0105's wider ends.

## Context

The standing measure (hamza, 2026-11-10) is windows faster than Go. Every K(ℕ\*) program was slower,
and every loop's step count is the digit count, so the digit's width is the first factor of each gap
(llvm-udiv-2026-11-11).

The double word (ADR 0103) holds an intermediate the decision proves in E = (−2¹²⁶, 2¹²⁶). With
digits in [0, B), Algorithm M's step u·v + w + k is at most B² − 1, so the largest base whose digits
are non-negative words and whose step is proven is B = 2⁶³ (B² − 1 = 2¹²⁶ − 1 ∈ E).

## Decision

1. **ℕ\* is in base B = 2⁶³.** A digit is a non-negative word, and the header 2m + s is unchanged. One
   reduction serves A, S, M and D: t mod B = t & (B − 1) and ⌊t/B⌋ = t >> 63, since ℤ's & is 2-adic,
   for every step's t:
   - A: x + y + c < 2⁶⁴, the unsigned word's;
   - S: x − y − b ∈ [−B, B), a word;
   - M and D4: < B², the double word's.
2. **The two-digit lemma is the decision's** (Knuth, TAOCP 2 §4.3.1): for t = u·C + w with
   w ∈ [0, C) and u a π of the guard u < d, ⌊t/d⌋ ∈ [0, C − 1], because
   t ≤ (d − 1)·C + (C − 1) < d·C. It is a transfer for `/` (ir/interval.go), relational where intervals
   are not.
3. **A quotient by a word is the triple's 2-by-1 division** where the lemma, or intervals, prove it a
   digit: x86-64's `divq` (`targets/windows/x64.oro`), declared as it is, its domain hi < d. It is
   Knuth's primitive and Go's `bits.Div64`. Algorithm D's q̂ is one.
4. **The double word reads back three more ways** (ADR 0103 item 1, extended):
   - `&` by a constant mask;
   - a right shift by a word count in [0, 127];
   - a quotient or remainder by a positive word, as in item 3, typed by its proven range for W14.
5. **A host operation states its law where the decision needs it**: `llvm.u_lshr`'s
   `ensure r == a >> b`, so a realized shift keeps its fact.

## Why not

- **B = 2⁶⁴.** Its step reaches 2¹²⁸, past E. Either E widens to 2²⁵⁴, so every end of every
  program's analysis becomes four limbs, or the split of a step into its halves becomes host operations
  (`mulq`'s rdx:rax, `adc`), which the decision does not see. Measured, it is what is left of the
  product's 1.5× and the sum's 1.8×: each 63-bit step's `shld` and `and`. It is the next step on that
  gap, not a reason to stop at 2³².
- **Keep B = 2³²**: half the digits do the same work, measured. The 2,000-quotient program was 1.7×
  Go, and is 0.85×.
- **Möller–Granlund division by a precomputed reciprocal** for D's estimate. It needs a 128-by-64
  division to make the reciprocal, and its intervals do not close without the same relational
  lemma. With the lemma, the hardware's own division is the derivation's primitive, in one
  instruction.
- **`__udivti3`** for a quotient by a word: a library call per digit, and a link MSVC does not make by
  default.

## Consequences

- fib 3.9× → 1.8× `math/big`, fact 5.0× → 2.9×, the 2,000-quotient program 1.7× → 0.85×
  (natword-2026-11-11). The product of a big integer by a word is unchanged at 1.5×: B = 2⁶⁴'s gap.
- The fixed limbs V_n keep base 2³² until ADR 0083 item 3 retires them on LLVM.
- A witness that forced an add-back at 2³² is constructed anew at 2⁶³, since an add-back is about 2/B
  likely.
