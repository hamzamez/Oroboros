# 0124 — K(ℕ\*)'s base is the triple's native word

Date: 2026-11-17
Status: Accepted. ADR 0122's D2, decided by hamza 2026-11-17 ("Go with your recommendations on
D1–D7"): `digit_bits` per triple, 64 on x86_64 and 32 on i686, by ADR 0106's theorem applied to the
native double word. **Amends [ADR 0106](0106-k-n-star-is-in-base-2-64-the-machines-digit.md) item 1**
(B = 2⁶⁴ on every target). Built with [d2-2026-11-17](../../gauntlet/results/d2-2026-11-17.md).

## Context

ADR 0106 chose B = 2⁶⁴ as the largest base whose B² − 1 the double word holds and whose
multiply-accumulate the decision proves, i128 on x86_64. On i686 the native double word is i64:
`mul r/m32` gives EDX:EAX, and `div r/m32` divides EDX:EAX. LLVM expands i128 there inline, but
`emit/natnum.oro` wrote B = 2⁶⁴ in about 40 places (20 carries, 13 products by B, the normalization, the printing's β and its digit counts, the bit length), and the decision's division of the double word by a
word needs the triple's 2-by-1 division, which i686 has only at 32 bits. So four K(ℕ\*) cases were
refused on windows32 ("no realization holds both", triple-research.md §3).

## Decision

1. **B = 2^dbits, dbits the triple's native word**: `emit.Target.DigitBits`, read off the triple's
   ISA: 64 for x86_64, 32 for i686 and i386. A triple whose ISA is not known has no digit, and a
   program needing K(ℕ\*) on it is refused. It is the triple's, as the data model and the address
   space are (ADR 0122); nothing in a target file restates it.
2. **The library reads B through one block**, its parameter:
   - dbits; dm = B − 1; bq = B/4, so B is written bq·4 inside the word; bh1 = B/2 − 1;
   - β = kb = 10^kd, the printing's base, the largest power of 10 below min(B, 2⁶²): 10¹⁸ at 64
     bits, 10⁹ at 32;
   - kdd = ⌊dbits·log₁₀ 2⌋ + 1.

   `natnum.oro` states the 64-bit block, and `loadNatLib` writes the 32-bit one in its place, each
   found exactly once. The one structural difference is that a word is ⌈64/dbits⌉ digits: `of` and
   `word` choose their form on dbits, a constant, and the reducer folds the choice. Every other line
   is the same at both bases, and its proofs are the decision's at each instance (ADR 0101).
3. **The digit's facts follow it**: `NumeralType` (a numeral's table as a program function's
   parameter or result) is [0, B), and a product's one-digit operand is below B.
4. **The two-digit lemma reads the unsigned word** (`ir/interval.go`, `numeralQuotient`). At
   B = 2³², Algorithm D's window t = u·B + w is the word's: `u64/` of `u64+` of `u64*` by B,
   through `u64-of`, which wordsel places only where the integer is in both words. These are ℤ's
   operations on the integers they hold, so the lemma (u < d, w < B ⇒ ⌊t/d⌋ < B) applies to them
   as written.
5. **A table's size is its own**: the two allocations whose sizes relied on room's 2³¹ being far
   below max_len (`kdigits`' 2·kj digits, `kdlevel`'s n + 1 words) are sized through `room`. On
   x86_64 the decision proves the guards dead, and the emission is unchanged.

## Why not

- **B = 2⁶⁴ on i686 through LLVM's i128 expansion** (triple-research.md D2's alternative). Each digit
  product would be a schoolbook of 4×4 limbs, and the decision's division of the double word by a
  word would need a 128-by-64 division the triple does not have, the refusal as it stood.
- **A `fact digit_bits` in each target file**: the base is ADR 0106's theorem applied to the ISA's
  word, a consequence of the triple and not a further choice. Stating it again would let the two
  disagree.
- **The library written over symbolic B with every literal computed**: 2⁶⁴ is no word, and a
  constant outside the word does not fold (ADR 0009). The block's literals fold, so the 64-bit
  emission is byte-identical.
- **Masks on Algorithm D's reads** (`& dm`, as `kq4` states them): tried first, then found
  unnecessary once the lemma read through the conversions. They would have changed every x86_64
  program dividing a numeral.

## Consequences

- windows32 runs every K(ℕ\*) case of the suite (`big-fib`, `big-nat-div`, `big-nat-ops`,
  `big-render` among them), and its instances are functions of the program, as on x86_64.
- At B = 2³², Algorithm D's estimate q̂ divides the word by a digit through the runtime's
  `__aulldiv` (ADR 0123), where one `divl` would do, the lemma proving the quotient below 2³². That
  is the rest of D4's `x86.oro`: the 2-by-1 division at 32 bits, chosen as `divq` is at 64.
- The Karatsuba threshold (kt = 64 digits) was measured at 64-bit digits and is not re-measured at
  32.
