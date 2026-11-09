# Signed limbs: Z(V_n), the group completion over the magnitudes (R2, ADR 0098)

Date: 2026-11-09. Step R2 of [integers-research.md](../../docs/integers-research.md), decided by hamza
with R1–R4 the same day ("Defer R1d, R1e; do R2"). Decision:
[ADR 0098](../../docs/decisions/0098-fixed-limbs-realize-a-signed-set-by-the-group-completion.md);
spec: [integers.md §1b](../../docs/spec/integers.md).

## What it is

ℤ is the group completion of (ℕ, +). Each class of pairs (a, b) ~ a − b has one normal form, a sign and
a magnitude, so a sign over any representation M of magnitudes, **Z(M)**, represents the signed
integers of M's bound. Every operation of Z(M) is a case split on the signs over M's operations
(integers-research §4.2):

| operation | on Z(M) |
|---|---|
| a + b, equal signs | the sign, \|a\| + \|b\| |
| a + b, different signs | the larger magnitude's sign, larger − smaller |
| a − b | a + (−b) |
| a · b | the product of the signs, \|a\| · \|b\| |
| a / k, a % k (truncated), k a word | the dividend's sign, k's magnitude |
| a < b | by sign, then by magnitude, reversed when both are negative |
| render | "−" when negative, then the magnitude's numeral |

On the fixed limbs, M = V_n = [0, Bⁿ), B = 2²⁴: a signed value is a table of n + 1 entries, the n
limbs and the sign (0 or 1) in entry n, with sign 0 when the magnitude is 0, the class's one
representative. The magnitude operations read entries [0, n) only, so they apply to a signed table
as they are. Z_k = (−2ᵏ, 2ᵏ) is |x| < 2ᵏ, the magnitude's bound, so each guard is N_k's.

## What was built

- **`emit/bignum.oro`**, the signed layer over the magnitudes:
  - `with_sign` (the normal form) and `sgn`;
  - `sof`, a word of either sign, its digits taken by truncated division without negating it, so
    −2⁶³ is exact;
  - `sadd`, `ssub`, `smul`, `smul_small`, `sdiv_small`, `srem_small`;
  - `slt`, `sle`, `sgt`, `sge`, `seq`;
  - `srender`.

  Each is checked once per width as a theory (ADR 0035, `emit/limbfunc.go`), like the magnitudes'.
- **`emit/bigrep.go`**: `PlanBig` no longer refuses a signed set on limbs, and plans it signed.
- **`ir/limbs.go`**: `LowerLimbs` takes the plan's sign, and maps each magnitude instance to its signed
  form (`signedLimb`), or reports an internal error naming the one it lacks.
- **`big-sign` runs on windows**: its skip, "fixed limbs hold a magnitude", is gone.
- **`big-signed-ops`**, a new differential case on four targets, takes one derived operation per input
  against the hosts' bignums and Python's values:
  - mixed-sign sum;
  - an all-negative sum;
  - a product by a negative word;
  - truncated / and %;
  - the order of two negatives;
  - −2⁶³ · 3⁴⁰.

  The −2⁶³ is computed as `0 - 2^63`, since the literal is outside JavaScript's word.

## Witnesses

- **`TestASignedSumIsTheGroupCompletions`** (`cmd/build`), run on windows, where Z(V_9) is the only
  representation: 3¹⁰⁰ + (−5⁶⁰) and (−5⁶⁰) + (−5⁶⁰) print Python's numerals. **The planted fault**,
  `ir.PlantSignFault`, takes the magnitude's `add` for `sadd`, ignoring the signs. It must print another
  numeral, and it does.
- **`TestTheLimbsHoldASignedSetByTheGroupCompletion`** (`emit`), restated from the test that pinned
  ADR 0029 decision 4's refusal: windows now takes a signed set above the word on a limb table.
- `big-sign` and `big-signed-ops` agree on go, js, java and windows.

## What was measured

**Correctness**: every case agrees on every target. **Proofs**: the check's counts went from 5,120 of
5,174 operations and 522 of 546 loops to 5,943 of 5,997 and 866 of 890. Every added operation and loop
is proven; the unproven 54 and 24 are unchanged.

**Run time**: 10⁷ additions of 10²¹ on windows, best of five, two rounds:

| loop | round 1 | round 2 |
|---|---|---|
| unsigned, `0..2^200` (V_9) | 138.7 ms | 144.1 ms |
| signed, equal signs (Z(V_9)) | 1,219.0 ms | 1,219.9 ms |
| signed, mixed signs: −10²⁸ climbing to 0 | 1,298.1 ms | 1,306.4 ms |

**The signed loop is 8.5× the unsigned one.** ADR 0098's expected cost was the magnitude operation, a
comparison, and a copy into the normal form. The measured cost is mostly something else: **storage the
reuse pass never reaches**. I read the IR right after `ir.Reuse`:

- the unsigned loop's two allocations each take a token (`reuse-raw`), and none are fresh;
- the signed loop's body has 12 allocation sites and 2 reuses.

The cause is where the allocations sit. `sof`'s `case v >= 0` and `sadd`'s `case` on the signs put
each allocation inside a branch of a statement-level `if`. R1b hands a token only to an allocation at
the top level of a terminal arm (`ir/reuse.go`), and W12 counts a token's uses only along the
terminator tree. So each iteration makes four tables on the heap and frees four, which accounts for
the ~1.08 s. The mixed-sign loop adds the comparison and the subtraction branch: 1.07× the equal-sign
one.

**Compile time**: `big-sign` on windows compiled in 687 ms against 15 ms in the baseline, where it was
refused. The profile has no hot spot: the passes over the IR (intervals, the LLVM printer, the text)
take it, in proportion to the code. The emitted code is the cost:

| case on windows | emitted lines | compile time |
|---|---|---|
| `big-subdiv` | 3,532 | 280 ms |
| `big-sign` | 19,399 | 630 ms |
| `big-signed-ops` | 45,189 | 1,200 ms |

Each limb instance is inlined at its use, and each signed instance inlines up to four magnitude ones.
"A limb instance emitted once as a function" is on the plan's deliberately-not-next list.

## What failed first

- **`with_sign` as a rule**, `array { e : i ∈ 0..n }`, was never allocated: the lowering's gap for a
  rule whose length is a parameter's expression. It is a build over all n + 1 entries instead, the
  written-prefix shape, so it is also born full (ADR 0097).
- **`srender`'s field**: ascii's `lo = top − 1 ≥ 0` was not provable from `top ≥ 0`. Rather than clamp
  it, the field is laid out on render's own `top`, the sign written at `top` and the digits after it.
- **`big-signed-ops` was refused on JavaScript**: 9223372036854775807 is outside JavaScript's word.
  `0 - 2^63` computes the same value exactly.
- **A test pinned ADR 0029 decision 4's refusal**, and failed when the refusal went. It is restated for
  the amended decision. The IR inlines the instances, so the test checks for a limb table rather than
  for an instance's name.

## What it cost

The library's signed layer is about 120 lines of the language, in `bignum.oro`. The compiler gained
about 40 lines (`limbs.go`, `bigrep.go`, `limbfunc.go`), and the tests about 60.

## What was not built

- **A token through a branch**, in R1b's pass and in W12: a token is linear, so it may enter an `if`
  and be used once in every branch, given back by `discard` in a branch that does not allocate. Two
  branches that each release a table of one size can also yield its token from the `if`.
- **`with_sign ∘ add` fused** into one build over n + 1 entries, with the normal form's zero test an
  OR-fold carried in the same loop: no temporary and no copy (deforestation, Wadler 1988).

  Together these remove the 8.5×. Both wait for R3: its magnitudes have run-time lengths, which
  R1b's static-size tokens do not reach, so storage for limbs is redesigned there. They are in the
  plan as R3's first measurement.
- **A division by a negative word.** As for the magnitudes, the one-pass forms (`smul_small`,
  `sdiv_small`) are taken only where the facts prove k ∈ [0, 2²⁸) (`widened` in `ir/limbs.go`). A
  product by a negative word goes through `sof` and `smul`, which is input 2's path. A quotient by a
  negative word is refused by name, "division by another arbitrary-precision value": the library has
  no division by limbs. Its form is the quotient by |k| with the sign of a · k, and it waits for a
  program dividing by one. The remainder needs nothing: cur % k on a non-negative cur is cur mod |k|.
- **Two's complement limbs**: ADR 0098's "Why not". Worth building only if a program measures the sign
  branch, and the sign branch is 1.07× here.
