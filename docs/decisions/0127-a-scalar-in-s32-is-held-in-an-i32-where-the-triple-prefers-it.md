# 0127 — A scalar proven in S₃₂ is held in an i32 where the triple prefers 32 bits; ρ₃₂ is read off its range

Date: 2026-11-17
Status: Accepted (hamza, 2026-11-17: "Go with item 4, always mathematical and algebraic", on
assessment-2026-11-17 §5 item 4, ADR 0122's D5). Builds ADR 0083 item 2's scalar rule and item 4's
"a value proven in S₃₂ is computed in `i32`". Built ([d5-2026-11-17](../../gauntlet/results/d5-2026-11-17.md)).

## Context

ADR 0083 item 4 holds the word at S₆₄ on every LLVM triple, i686 included: `i64` is exact there.
But i686's registers are 32 bits, so LLVM legalizes each `i64` into a register pair: an add
becomes an add and an add-with-carry, a comparison two comparisons and a branch.

ADR 0083 item 2 gives the remedy in principle: a scalar takes the target's preferred register
width for its fact, narrower only where measured faster. triple-research.md measured it on one loop,
C's `int` 2.6× C's `long long` on i686, and left the building to D5.

Measured on our own code before deciding (d5 §1), the research's loop over 3·10⁸ steps:

| | i686 | x86-64 |
|---|---|---|
| ours, the word | 805 ms | 476 ms |
| ours, the same IR at i32 by hand | 474 ms | 302 ms |
| C `int` / `long long` | 332 / 872 ms | 336 / 328 ms |

## Decision

1. **A representation of S₃₂, (H, d)**: H is the i32, d the signed reading. A target declares that
   its preferred scalar width is 32 as data, `repr scalar 32`.
   - **windows32 declares it.**
   - **windows does not** (item 5).
2. **ρ₃₂ is a function of the proven range, read off IR_P's text.** A value whose representative's
   IR_P type is a range inside S₃₂ is an `i32`. A π shares its source's register, so its source's
   range decides it. The IR is not changed:
   - IR_P's types already are the decision's facts, which the printer trusts for `nsw`, `nuw`,
     `samesign` and `range(…)`;
   - so the representation is readable from the text alone, as ADR 0038 and ADR 0083 (W6) require.
3. **An operation is printed at 32 bits exactly where that is exact.** The residue map
   r: ℤ → ℤ/2³² is a ring homomorphism, injective on S₃₂. So by induction an operation on operands in
   S₃₂ whose result is proven in S₃₂ computes, at 32 bits, the residue of the result, and its lift in
   S₃₂ is the result. The bitwise operations of ℤ (ADR 0082) and the signed order are closed on S₃₂
   and are i32's. The exceptions are the triple's undefined behaviour, so an operation is printed at
   i32 iff:
   - its integer operands and result are `i32`;
   - its spelling is width-generic: no literal type, no `${r}`;
   - its 32-bit domain holds by the ranges: `sdiv` and `srem` exclude (−2³¹, −1), and a shift's
     count lies in [0, 31]. These are ADR 0088's `where`s on the instruction, one width down.

   Its `nsw` is the result in S₃₂, and its `nuw` the naturals', as at the word.
4. **Every other statement is the word's, in a wide scope.** Each `i32` operand is `sext`ed first,
   and each integer result in S₃₂ is `trunc`ed after. Both are the identity on ℤ: d, and its inverse
   on S₃₂.
   - **At a join**, a value is converted into its slot's type. A `trunc` there is exact by the
     destination's range, which the decision proves of every value arriving on that edge. IR_P types
     some constants `int`.
   - **A table keeps its layout**: its index and size are `sext`ed for the address, and an element
     is read into its result's width.
   - **A function's boundary is the word's**: a parameter in S₃₂ is narrowed at entry, and a result
     widened at `ret`. Callers and the C ABI's maps are unchanged.
   - **Any other narrowing** that is not exact by a range is a printer error, never a silent residue.
5. **x86-64 does not declare it, by measurement.** The loop is 1.58× there, and the sieve 1.15×. But
   the gauntlet, the one fixed commitment, does not gain: the JSON tokenizer and tree are 1.04× and
   1.06× slower, the hand-written controls within 0.3%. Big integers are neutral. On x86-64 a 32-bit
   index pays a `sext` for every address, and a 2⁴⁷-element address space leaves most indices
   outside S₃₂ anyway.

## Why not

- **An IR pass that retypes values `s32`**, with conversions as IR operations, as the unsigned word
  has (ADR 0033). It would lose each value's range, which the printer reads for `nsw`, `nuw`,
  `samesign` and `range(…)`, unless the type grammar grew a representation beside the range. The
  range is already the proof; ρ₃₂ of it is a function, not a choice to record. The unsigned word
  needs its own operations (its order and division differ), and S₃₂'s operations are the word's.
- **A selection by a greatest fixed point**, as the double word's (ADR 0103): a value's
  representation would depend on its uses, narrowing only where no use is wide. That would avoid the
  conversions where a narrow value meets a wide use. But the conversions are the identity on ℤ, and
  LLVM folds them. ρ₃₂ by the value's own fact is the least rule, measured sufficient: the hand form
  and the printed form both run in 472 ms.
- **The C `int`'s semantics** (wrapping, or undefined on overflow): the language's integer is ℤ.
  The i32 holds only what is proven in S₃₂, and nothing wraps.
- **U₃₂ as well**, a natural below 2³² in an i32 read unsigned (K(ℕ\*)'s digit on i686, B = 2³²).
  It needs the unsigned reading everywhere: `zext`, `udiv`, `urem`, `ult`, and the double word's
  digit product. Not built: K(ℕ\*)'s digits stay i64 pairs on i686. It is the next measured
  candidate for windows32's big-integer gap (rangeload §3).
- **Declaring it on x86-64 now.** Not faster on the gauntlet (decision 5). Narrowing a value by its
  uses, never where it is an address, would be its own step, measured.
- **Narrowing a function's parameters and results.** It would change every caller and the C ABI's
  maps for a conversion LLVM inlines away inside a module.

## Consequences

- windows32's emission changes almost everywhere: over the corpus, 14,983 operations are printed at
  i32 and 14,181 at i64. Its runtime, `lib/rt/i686.ll`, changes by its `ctlz` count. The other
  targets' emission is byte-identical.
- On windows32, every program measured is faster, with identical output: 20000! printed 0.77×,
  fib(200000) 0.79×, the sieve 0.73×, the loop 0.59×.
- windows32's emission is still not in `cmd/check`, which is still hamza's call. The step is checked
  by the differential suite on the triple (72 of 74, as before), by `TestAScalarIsHeldWhereItsRangeIs`
  (the text, two planted faults) and `TestAScalarIsHeldWhereItsRangeIsRun` (a run, one planted
  fault), and by all 103 windows32 corpus files assembling.
