# Where K(ℕ\*)'s time goes after M4b: the carry's split

Date: 2026-11-12. After natword-2026-11-11, fact(20,000) printed is 2.9× Go's `math/big`, about half
of it printing. This round asked three questions in turn, each measured before anything was built on
it. **The answer to all three is one instruction: the `shld` that splits a step's t into ⌊t/2⁶³⌋ and
t mod 2⁶³.** A row with 64-bit digits, whose carry is the product's high half, runs **3.0× faster** per
digit (C, the same loop at both bases).

## 1. Does divide-and-conquer printing pay before Karatsuba?

The plan said no: m²/2 steps of D against chunking's 0.53m². The algebra of the two steps says the
counts are not comparable:
- **a chunk's step** is a short division, the serial recurrence r ← (r·B + dᵢ) mod 10¹⁸, with every
  multiplication of its reciprocal on the loop-carried chain;
- **D's multiply-subtract step** has its multiply off the chain.

So Schönhage's conversion, x = q·10^(18·2ᵏ) + r level by level, should beat chunking even with
schoolbook D.

**Measured, one split by hand** (`10^(18·2048)` by squaring, x = fact(20,000), medians):

| | ms |
|---|---|
| fact and the tower (C) | 72–87 |
| + printing x by chunks (A) | 139–157, so printing ≈ 70 |
| + one D split and printing both halves (B) | 132–149, so ≈ 62 |
| + one D split alone (D1) | 98.5, so the split ≈ 18 |

The split's 4.2·10⁶ steps cost 4.2 ns each, against a chunk step's 8. Divide and conquer would spend
about 18·(1 + ½ + ¼ + …) ≈ 36 ms in divisions against 70: **about 1.8×, not several**. That is not
because the recurrence argument is wrong. A D step costs 12–13 cycles where its algebra needs about 4.

## 2. Is D's step its guarded reads?

D4 read `pg(v, …)` and `wg(r, …)`, each a compare and a branch, and clang spilled a pointer it reloaded
every step. **Built**: the index domain [0, n + 2) = {0} ⊔ [1, n) ⊔ {n} ⊔ {n + 1}, by what each index
reads:
- the new digit d at 0;
- r_{i−1} and vᵢ on [1, n);
- r_{n−1} alone at n, since a normalized divisor carries nothing out;
- the sign at n + 1.

The lengths are one guard at entry, true of every call. The loop became 17 instructions with no
branch but its own. **Measured: 84.5 → 81.1 ms** on the 2,000-quotient program, inside the noise. Its
code is the algebra's and no slower, so it is kept.

## 3. What is left on the chain

The tight loop's carry chain is `add`, `adc`, `shld $1`. The borrow's is `add`, `sub`, `sar`. `shld`
splits t ∈ [0, 2¹²⁶) into ⌊t/2⁶³⌋ = 2·rdx + (rax >> 63) and t mod 2⁶³. It is the same split in
`mul_small`'s row, in A's sum and in D4's step: three of the four loops K(ℕ\*) spends its time in.

**Measured, in C** (clang `-O2`, 4,000 digits × 20,000 rows, `u128` products):

| row | ns per digit |
|---|---|
| base 2⁶³: `o = t & (2⁶³ − 1)`, `c = t >> 63` | 1.11 |
| base 2⁶⁴: `o = t mod 2⁶⁴`, `c = t >> 64`, the halves | **0.37** |

Go's `math/big` product runs at about 1 ns per digit on this machine (fact's product, 41 ms over
4·10⁷ steps). **B = 2⁶⁴ is worth up to 3× on the product, the sum and D**, which would put all three
ahead of Go's.

## What it needs (M4c, not built)

1. **E widened** past 2¹²⁸. A step of base 2⁶⁴ reaches B² − 1 = 2¹²⁸ − 1, and E is (−2¹²⁶, 2¹²⁶) today.
   An end becomes 192-bit two's complement, |v| < 2¹⁹⁰. `ir/endpoint.go` is the end's only
   representation (no field is read outside it), so the change is local; its cost is the compile gate's.
2. **The double word read unsigned where its fact is natural.** ℤ → ℤ/2¹²⁸ is injective on any
   interval of length 2¹²⁸, so [0, 2¹²⁸) is exact read unsigned. The printer then writes `nuw` and
   `icmp ult`, and never `nsw` past 2¹²⁷.
3. **Digits in U₆₄**, a table of `0..2^64-1` elements (ADR 0083 item 2), each step's t read back by
   t mod 2⁶⁴, the low half (a `trunc`), and t >> 64.
4. **The library at B = 2⁶⁴**: A's sum is then the double word's too (x + y + c < 2⁶⁵), and render's
   chunk is 10¹⁹.

## Found beside it

**A result claim is not a fact at a call.** `fn tower(k) (1..)` proves its result positive at its
definition (ADR 0072). A caller dividing by it is still asked to prove `t != 0`, and the refinement
layer does not know the claim. A guard cleared it. This is a missing fact, owed when a program meets
it.

## What it cost

About two hours, nothing on the plan's path broken. Two predictions were wrong, the latency
argument's size (§1) and the guarded reads (§2), and each was found by measuring before building more
on it. The third question was answered by reading the machine code and then measuring the
alternative in C, the form expected to lose included.
