# The double word: an intermediate above the word, in i128 (M4a)

Date: 2026-11-11. M4's first step: [ADR 0103](../../docs/decisions/0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md),
[llvm.md §3c](../../docs/spec/llvm.md). The 64-bit digits need an intermediate wider than the word,
the multiply-accumulate of two digits, and the language had no representation for one.

## What it is

The residue map ℤ → ℤ/2¹²⁸ is a ring homomorphism, injective on (−2¹²⁷, 2¹²⁷). That interval contains
E = (−2¹²⁶, 2¹²⁶), where the decision's finite ends lie. So a term of + − · neg whose every
intermediate the decision proves in E is computed exactly in i128. A term reads its intermediates
back into the word by:
- a quotient, remainder, `mod` or right shift by a positive constant;
- a comparison;
- + − · whose result is proven in the word.

No value of the program leaves the word. ADR 0082's fusion reads a term through `mod 2ⁿ`, a
homomorphism, and so ignores its intermediates; this reads one through an injection, and so needs
them in E.

So B = 2⁶³ is the largest base whose digits are non-negative words and whose multiply-accumulate
u·y + w + k ≤ B² − 1 = 2¹²⁶ − 1 the decision proves exactly, with no change to E.

## What was built

- **`repr double 128`**, target data, declared on the LLVM layer (`targets/llvm/llvm.oro`).
- **`ir.SelectDouble`** (`ir/double.go`), in `afterRung`, so program functions get it too. It is a
  greatest fixed point:
  - the candidates are the unproven + − · neg with a finite fact;
  - a candidate is dropped while a use is not one of the above, or an operand is neither a word nor a
    candidate;
  - a π of one is one of them, the same integer refined, and its uses are its own;
  - each is marked `double` in IR_P and typed by its proven range.
- **The decision** proves a `double` operation where its fact is finite (every finite end is in E).
- **W14**, in the verifier: a double-word value is read only by those operations, and only on a
  target declaring one.
- **The LLVM printer** computes them in i128:
  - a word operand is `sext`'d in;
  - `nsw` always, `nuw` on `add` and `mul` of naturals;
  - `udiv`, `urem`, `lshr` of a natural;
  - a result back in the word is `trunc`'d.

  The flag witness now reads i128 lines too.
- **A divisor is a constant**: LLVM divides an i128 by a variable through `__udivti3`.

## What failed first

- **Nothing was selected without `-checked`.** The decision gives an unproven operation the mode
  `exact` as a placeholder, and the filter skipped every `exact` operation. Unproven is a fact
  outside the word, not a mode.
- **`Finalize` runs before the pass**, so its double-word values had no range, and the printer chose
  `srem` and no `nuw`. The pass now types what it marks.
- **W5 read every arithmetic operand as a word.** The double word's operands are W14's.
- **A comparison that is a branch's condition gives its operands πs**, which the first closure counted
  as escapes. Algebraically a π is the same integer, so it joins the term.
- **The planted fault `PlantDoubleUnbounded` had its sign reversed in the decision** (`finite &&
  !plant`), so it refused instead of admitting. Its witness's anti-vacuity check ("the witness cannot
  fail") found it.

## Witnesses

- `TestADoubleWordIntermediateIsComputedThere`: the Mersenne hash's product and sum are the double
  word's, and the program verifies.
- `TestADoubleWordNeverLeavesItsTerm`: a product that is the function's result is refused. **The
  planted fault** `PlantDoubleEscape` lets it in, and W14 refuses the program for the reason,
  independently of the pass.
- `TestAnIntermediateOutsideEIsRefused`: a product of three words reaches 2¹⁸⁶ and is refused. **The
  planted fault** `PlantDoubleUnbounded` takes any fact in, and the refusal goes.
- `TestAFlagIsTheIRsProof` reads `mul nuw nsw i128` and `urem i128` back against the IR's facts.
- **Program first**: `gauntlet/differential/cases/mersenne-hash.oro`, the polynomial hash modulo
  2⁶¹ − 1, (h·x + c) mod p with h·x < 2¹²², agrees with Python on windows. It is refused on Go, the JVM
  and JavaScript, which declare no double word.

## Measured

10⁸ steps of the hash, serial, 21 rounds, median:

| program | time | ratio |
|---|---|---|
| Go, `bits.Mul64` and `bits.Rem64` (idiomatic) | 869 ms | 1.00 |
| Go, `bits.Mul64` and the Mersenne fold, by hand | 385 | 0.44 |
| **the language on windows** | **425** | **0.49** |

clang multiplies by the reciprocal of 2⁶¹ − 1 where Go's `Rem64` divides in hardware. The hand-folded
form, which exploits 2⁶¹ ≡ 1, is 1.10× faster than ours, inside the noise floor. Folding a Mersenne
modulus is a further theorem the compiler does not apply.

**The check**: every program that compiled before compiles to the same bytes. The only change is the
new case, emitted on windows and refused on the other three. Compile time is 0.96× the baseline.

## What was not built

- **K(ℕ\*) over B = 2⁶³ (M4b)**, the step this one is for.
- **The Mersenne fold**: x mod (2ᵏ − 1) by (x & m) + (x >> k).
- **Go's double word** by `math/bits`.
- **ADR 0083 item 3 for values** (iN for every finite set above the word, the limbs retired). It is
  measured first, as the ADR says.
