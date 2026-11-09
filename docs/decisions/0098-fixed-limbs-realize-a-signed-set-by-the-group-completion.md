# 0098 — Fixed limbs realize a signed set by the group completion

Date: 2026-11-09
Status: Accepted (hamza, 2026-11-09: R2 of [integers-research.md](../integers-research.md), agreed
with R1–R4; "Defer R1d, R1e; do R2"). Built with
[signlimbs-2026-11-09](../../gauntlet/results/signlimbs-2026-11-09.md). **Amends
[ADR 0029](0029-above-the-word-one-set-on-every-representation.md) decision 4**, "fixed limbs realize
N_k only".

## Context

ADR 0029 made the sets above the word the lattice H = {N_k} ∪ {Z_k} ∪ {ℤ}, decided by a sign and a bit
length, and every host bignum sign-magnitude. Its decision 4 refused Z_k on fixed limbs, which hold a
magnitude, so on a target with no bignum a signed range above the word was refused (`big-sign`'s
skip on windows).

## Decision

1. **ℤ is the group completion of (ℕ, +)**: pairs (a, b) standing for a − b, with (a, b) ~ (c, d) iff
   a + d = b + c. Every class has one normal form, (m, 0) or (0, m) with m > 0, which is a sign and a
   magnitude (Bourbaki's symmetrization; integers-research §4). So **Z(M)**, a sign over any
   representation M of magnitudes, represents the signed integers of M's bound, with every operation
   derived from M's (§4.2):
   - **+** with equal signs adds the magnitudes; with different signs, the larger magnitude's sign
     and the larger minus the smaller;
   - **−** is + of the negation;
   - **·** multiplies the magnitudes, with the product of the signs;
   - **truncated / and %** by a word k: the remainder's sign is the dividend's, and the quotient's is
     the product of the dividend's and k's. On limbs, the one-pass forms take a k the facts prove in
     [0, 2²⁸), as for the magnitudes, so there the quotient's sign is the dividend's;
   - **order** is by sign, then by magnitude, reversed when both are negative;
   - **render** writes "−" when negative, then the magnitude's numeral.
2. **Z(V_n) on the fixed limbs**: a signed value is a table of n + 1 entries, the magnitude's n limbs
   and the sign, 0 or 1, in entry n. The normal form is sign 0 when the magnitude is 0, the
   completion's one representative. The magnitude operations read entries [0, n) only, so they apply
   to a signed table unchanged. `emit/bignum.oro` gains the signed layer: `sadd`, `ssub`, `smul`,
   `smul_small`, `sdiv_small`, `srem_small`, the five comparisons, `sof` (a word of either sign, its
   digits taken without negating it, so −2⁶³ is exact) and `srender`.
3. **Z_k is enforced on the magnitude**: |x| < 2ᵏ is the magnitude's bound, so each operation's
   `guard` is the one N_k has. ADR 0029 decision 3's two observables are the representation's two
   components.
4. **A signed program on a target with no bignum takes Z(V_n).** A target with a bignum still takes the
   bignum for a signed program, as ADR 0029 decided.

## Why not

- **Two's complement limbs**: residues mod Bⁿ read signed, as S₆₄ is `i64` read signed. Their +, −
  and · are the existing code without a sign branch, by the residue homomorphism. But order, division,
  render and the bound need signed forms, and the construction does not extend to ℤ (R3), which is not
  a residue ring. It is a second mechanism, worth building only if a program measures the sign branch
  of mixed-sign addition (integers-research §4.4).
- **A sign beside the table, a product.** The IR carries products, but the limb rung's every operation
  is a call answering one table, and its lowering, release and reuse all reason about one value. A sign
  entry keeps the value one table.
- **Keep the refusal.** "A target may neither decline nor declare a language construct": a signed range
  is the language's, and its realization is a construction from what the target has.

## Consequences

- `big-sign` runs on windows, and every program whose one set is Z_k there.
- A signed table is one entry longer than an unsigned one of the same bound, and each signed operation
  is its magnitude operation, a comparison when the signs differ, and a copy into the normal form.
