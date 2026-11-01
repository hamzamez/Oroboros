# resultclaim-2026-11-02: a result in an open range is a claim (ADR 0087)

The hole [grids-2026-11-01](grids-2026-11-01.md) found and brought to hamza: a definition's declared
result in a range with an infinite end was decided by nothing, so `fn h(y ∈ -5..5) (Nat) { y }`
compiled and returned −5. The same was true of a range past the word whose value the facts keep in the
word: `(0..2^100)` returned −5 too. Both reproduced at HEAD before this step. Repaired by
[ADR 0087](../../docs/decisions/0087-a-results-set-is-a-claim-by-its-ends-inside-the-word.md).

## The algebra

A result's set S = [a, b] is the meet of two half-spaces, [a, +∞) ∩ (−∞, b], so `result ∈ S` is the
conjunction of two half-space claims. A parameter's range was already read so: at a call the finite
side of an open range is a conjunct, refused when unproven. The result is the dual: each end inside the
word is a postcondition, decided at the definition (ADR 0072). An end past the word is ADR 0029's,
enforced at run time through α, the least set of sign and bit length containing S.

The rule that shipped (scalarrange-2026-08-31, `core/read.go`) made a postcondition only of a range
whose two ends fit the word. Its premise was that the interval domain reports a bignum as [−∞, +∞], so
no guarantee above the word could be discharged. That was the term analysis's domain. The IR's is over
ℤ̄ and keeps a finite end beside an infinite one, except where three of its rules gave ⊤. Each is
repaired by its algebra:

1. **A product with an unbounded factor** (`mulIV`) was ⊤. Now it is the hull of the four corner
   products over ℤ̄, with (±∞)·c = ±sign(c)·∞ and 0·(±∞) = 0 (`mulInf`). The last is sound because an
   interval of ℤ never attains its infinite end, and a factor 0 makes every product 0 (Hickey, Ju and van
   Emden 2001, interval arithmetic over the extended reals). So [1, +∞)·[2, 30] = [2, +∞), a factorial's
   sign. A finite product whose every corner leaves E now keeps the side its signs agree on, where it
   took both ends ±E.
2. **A range type with an infinite end** (`rangeOf`) was ⊤, since `IntRangeBig` does not read `+inf`.
   Now `Nat` is [0, +∞] (`core.RangeEnds`).
3. **The export's claim was decided after the rung above the word**, where a bignum's multiplication is
   a host call with a ⊤ fact. A representation is an abstraction function (ADR 0083), so the claim's
   value is one on every representation, and it is now decided on ℤ's facts, right after the first
   decision. A definition alone already was.

And one rule followed from ADR 0029 rather than being lost: **an ascription in a set with an infinite end
tells nothing**, since α of it is ℤ. With `rangeOf` exact, the export's own `Nat` result ascription
otherwise proved its own claim. That was right before only by accident, while `rangeOf` gave ⊤.

**Where ADR 0029's run time still decides an end inside the word**: only the sign end 0 of a bounded
range past the word, whose α = N_b checks the sign exactly, on a value whose facts leave the word, so it
is held where the check runs (`signAtRunTime`). Every other end the facts do not prove is refused.

## What was measured

- **The hole, closed**: `fn h(y ∈ -5..5) (Nat) { y }` is refused, "the body does not establish
  0 <= result; its result is [-5, 5]". So are `(r ∈ 10..)` and `(..-1)` on a body that leaves them, and
  `(0..2^100)` held in the word. A definition alone and an export are both covered.
- **The programs above the word stay proven on the facts**: `examples/big`'s fact, fib, pair and power,
  all declared `Nat`, now establish 0 ≤ result statically, which nothing established before.
- **The differential suite**: `big-mul` is proven statically after the all-overflow corner rule.
  `big-negmid` keeps ADR 0029's sign check at run time: its result is 5·10³⁰, held above the word,
  and the facts do not see a − b + b = a. Both agree on every target, as before.
- **Tests and planted faults**:
  - `TestTheProductOfUnboundedIntervalsIsSound`: every product of elements sampled up to 2²⁰⁰ past
    each finite end lies in the computed interval, over 11 shapes with an infinite end. Planted: an
    infinite corner dropped, which the sampling sees.
  - `TestAnOpenRangeOnAResultIsAClaim`: the refusals above, each naming its end, and the proofs.
    Planted: the old skip, which accepts −5 as `Nat`; and the run time taken for a word-held value,
    which accepts `(0..2^100)` returning −5.
  - `TestAResultClaimIsAnObligation`'s new cases cover a definition alone.
- **Stale tests**, each pinning the rule that shipped, rewritten to the corrected one:
  - `TestABigResultRangeSynthesisesNoGuarantee`, now `…GuaranteesItsFiniteEnd`;
  - `TestABigResultRangeIsNotAnObligation`, now `…IsDecidedOnTheFacts`;
  - `TestADeclaredSetChoosesNoRepresentation`, which accepted `fn inc(n) (Nat) { n + 1 }`, a claim
    its unconstrained parameter refutes (inc(−5) = −4);
  - `TestWithoutTheDeclarationTheMagnitudeIsNotDerivable`, which required [−∞, +∞] where the product
    is [6999881, +∞).
- **ℤ's separation normal form** (numbers.md §2), held back by grids-2026-11-01 because of this hole:
  `{ y ∈ Int : 0 <= y && y < 10 }` is `0..9`, `{ y ∈ Int : y >= 0 }` is `Nat`, and ADR 0084's test of
  separations now uses a formula that is no bound for its nested and computed cases.
- **The corpus** (`cmd/check`, accepted): every program's emission and proof counts unchanged
  (4,801 of 4,853 operations, 502 of 525 loops), since no program in it declared a false open range. Two
  refusals report more: collatz's `3·x` is `[6, +inf]` and kara's `acc·100` is `[0, +inf]`, where both
  were `[-inf, +inf]`. Compile time 1.04×.

## What failed first, what it cost

- **The first repair decided every end inside the word statically**, which refused `big-negmid`, a
  correct program whose sign ADR 0029 enforces exactly. A bounded range's sign end on a value held above
  the word is the one case the run time decides, and the derivation had to say which case that is
  before the rule was right.
- **Reading open ranges exactly in `rangeOf` made a claim prove itself**: the export's result
  ascription narrowed the value to `Nat`. ADR 0029's α says why that is false, and the ascription rule
  now follows it.
- **A unsigned word's result lost its upper end**, `0..2^64-101` on Go. The factor that decides a range
  the reader cannot express was keyed on the reader having made no conjunct. It is keyed now on both
  ends not being int64.
- **The first probes were accepted though the reader was fixed**: `checkResultClaim` carried the same
  premise in its own condition, and the export's check ran after the rung. Three places held one rule.

## What was not built

- **A relational fact for the sign**: `big-negmid`'s a − b + b is [−10³⁰, 1.1·10³¹] by intervals. Its
  claim stays ADR 0029's at run time, as it was.
- **ℤ's transfer for a bignum's host operations** on the realized function: not needed, since the
  claim is decided before the rung, and every later consumer reads the ℤ-level decision.
