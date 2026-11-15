# Printing a numeral: radix conversion as a tree of divisions

Date: 2026-11-15. hamza: "go ahead with printing, always mathematical and algebraic". The step
CLAUDE.md names after Karatsuba: `fact`'s gap is printing. This document is the algebra, a measured
C model of three methods, and the design for K(ℕ\*) (`emit/natnum.oro`).

## 1. The program, and what it measures

`fact(20,000)` printed: 4,015 words of base 2⁶⁴, 77,338 decimal digits. On a quiet machine:
- our `fact` takes 100 ms, of which about 67 are printing (`factm`, the same product without
  printing, takes 33);
- Go's `fact` takes 41 ms, and Go prints the number (`big.Int.String`) in **1.96 ms**.

Printing is the whole gap, about 34× Go's. (A later, quieter sweep put our `fact` at 95 ms, printing
about 62, Go's at 34; radix-2026-11-15.)

## 2. The algebra

A numeral is the image of its digits under ε_B : x ↦ Σ xᵢBⁱ (karatsuba-research.md §2). Printing
is the change of basis to β = 10¹⁹, the largest power of 10 below 2⁶⁴ (10¹⁸ as built, §4): the
unique digits dᵢ ∈ [0, β) with Σ dᵢβⁱ = x, each then written as 19 decimal digits. That is ε_β⁻¹ ∘ ε_B, and **division is
its only operation**, since dᵢ = ⌊x / βⁱ⌋ mod β.

- **Repeated short division** (Knuth, TAOCP 2 §4.4, method (1a); today's `render`, with 10¹⁸):
  x ↦ (⌊x/β⌋, x mod β), once per output digit. It is a fold whose state is the shrinking quotient,
  n²/2 two-by-one divisions for n words, and each division waits on the previous remainder: the
  `divq` chain, 12 ns a step, latency-bound (carry-2026-11-13).
- **Divide and conquer** (Knuth §4.4 exercise 14; Brent and Zimmermann 2010, §1.7.2):
  - with P_j = β^(2^j), x = q·P_j + r splits the digits in two: r's are the low 2^j, q's the high;
  - recursing on both gives a balanced tree whose shape is a function of x's size alone, so it is a
    loop over levels (ADR 0014), level j's nodes all of one slot size;
  - the powers come by squaring, P_{j+1} = P_j², which is ADR 0110's polarization.

  The cost is the divisions by the P_j.
- **The division at a node**:
  - by Algorithm D (Knuth §4.3.1): about k² steps for a quotient of k words by a divisor of k
    words. Summed over the levels, about n²/2 steps, the same count as repeated short division.
    But D's step is a multiply-subtract row, a throughput operation, not a `divq` on a latency
    chain;
  - by Newton's reciprocal and Barrett's reduction (Barrett 1986; Brent and Zimmermann §2.4.1): q̂
    from one product by a precomputed reciprocal μ_j = ⌊B^{2k}/P_j⌋, q − 2 ≤ q̂ ≤ q, two corrections
    at most. Two products a node, so O(M(n) log n) with M(n) the product's cost;
  - by the scaled remainder tree (Bernstein 2004): fractions x/P in place of quotients, one product
    a node.

## 3. Measured, a C model (scratchpad `radix/r.c`)

Every method prints Go's 77,338 digits, MD5 `caeb53a0…`:

| method | ms |
|---|---|
| repeated short division by 10¹⁹ | 33 |
| **divide and conquer, Algorithm D at each node** | **9.8** |
| Go `big.Int.String` (divide and conquer, recursive division, Karatsuba) | 2.0–2.4 |

**Newton and Barrett do not pay yet.** With our products (Karatsuba at 3× Go's per operation,
kernels-research.md §7), two products a node cost about 7 ms across the tree, and the reciprocals
add more, which is no better than D's 9.8. Subquadratic conversion pays once multiplication is faster,
and that is the postponed design work. **So D's tree is the step now**: about 3.4× on the C model,
exact, built from the library's own Algorithm D.

## 4. The design in K(ℕ\*) (built, ADR 0111)

- **β = 10¹⁸, not 10¹⁹.** The last level's 2-by-1 division is the double word's by a constant, and
  a constant divisor is at most 2⁶² (ADR 0103); 10¹⁹ is a word, whose 2-by-1 division needs the
  two-digit lemma's premise, which a rule's body does not carry. So a base-β digit is 18 characters,
  and x has at most m + m/14 + 1 of them (log_β B = 64/59.79 < 1 + 1/14).
- **A level is one table of slots.** The language has no table of tables, so a level's 2^L nodes
  are one table of 2^L slots, as Karatsuba's levels are (ADR 0109). The top slot is x itself, and
  every node at level L is below P_{J−L}², so it fits 2k words, k = size(P_{J−L}).
- **Each level is one pass of Algorithm D in place.** For the normalized divisor v = P_j·2ˢ and a
  node u < P_j² ≤ P_j·Bᵏ:
  - **Theorem (the slot is the state).** u′ = u·2ˢ < v·Bᵏ, so u′'s word 2k is 0 and the quotient
    has k digits. After the step at position jq the window U[jq .. jq + k] is below v < Bᵏ, so its
    top word is 0: the digit is stored there. The node ends as remainder·2ˢ (words 0 .. k − 1) and
    quotient (words k .. 2k − 1), and no step allocates.
  - **The sign is the top word.** D4 leaves the window in (−2v, v); in k + 1 words of complement it
    is negative exactly when its top word is not 0, so D6 adds v back while it is (Theorem B).
  - **q̂ lives in word 2k**, counted down by D6 as (q̂ − 1) mod B, since a digit in [0, 2⁶⁴) is no
    `Int` and cannot be a loop variable.
  - **A node starts at its first nonzero window.** With L significant words, q < B^(L−k+2), so the
    steps above jq₀ = min(k − 1, L + 1 − k) are of a zero window. The tree is sized by the power of
    two above x's length, and without this the top node and the quotients below it ran k steps on
    zeros: 29 M row steps for `fact` against the 8 M of the algebra.
  - **Every index is linear in the slot's base** sb = j·w₂, so the reads are proven, not clamped
    (a guarded read of the live buffer copies its continuation into each branch).
- **The last level** splits nodes below β² by β, one 2-by-1 division a node, and the text is ι of
  the ASCII bytes (ADR 0095).
- **The powers**: P_0 = β, P_{j+1} = sqr(P_j) (ADR 0110), up to the least J with x < P_{J+1}.

## 5. Why not

- **The library's own Algorithm D at each node** (`qhat`, `msub`, `fix1`): built first, correct,
  and slower than chunking (155 ms against 95 for `fact`), its window a table born per quotient
  digit.
- **Repeated short division**: the measured 3.4×. Its step is a latency chain no rewrite cheapens
  (carry-2026-11-13 §printing's leaf).
- **Newton and Barrett now**: §3. They wait on faster multiplication, after which they are the
  subquadratic step, with the scaled remainder tree a further 2× beyond them.
- **A table of numerals for a level**: not in the language (a table inside a table, R1e). The slot
  layout is the representation GMP and Karatsuba's levels use.
- **Recursion**: ADR 0014. The tree is balanced and data-independent.

## 6. What it should measure

- `fact(20,000)` on windows, against Go: printing from about 67 ms toward the C model's 10.
- Every output against Go's text: the differential's `big-*` cases, and a numeral of every size
  across the levels' edges.
- A planted fault in the slot division (a dropped add-back), so the witness can fail.

## References

- Knuth, D. E. *The Art of Computer Programming* 2, 3rd ed., §4.3.1 (Algorithm D, Theorem B) and
  §4.4 (radix conversion).
- Brent, R. P. and Zimmermann, P. *Modern Computer Arithmetic*, Cambridge 2010, §1.7 (radix
  conversion), §2.4 (Barrett).
- Barrett, P. "Implementing the Rivest Shamir and Adleman public key encryption algorithm on a
  standard digital signal processor", CRYPTO 1986.
- Bernstein, D. J. "Scaled remainder trees", 2004.
- Burnikel, C. and Ziegler, J. "Fast recursive division", MPI-I-98-1-022, 1998.
