# 0079 — A declared set is a claim, and the facts choose the representation

Date: 2026-10-25
Status: Accepted (hamza, 2026-10-25: "Yes, a declared set should stop being a representation request.
with the representation chosen from what's proven", on the question ADR 0077 item 5 opened). **Built
2026-10-26** ([claimset-2026-10-26](../../gauntlet/results/claimset-2026-10-26.md)); see Consequences. Amends [ADR 0019](0019-precision-by-declaration.md) items 3–4 and
[ADR 0034](0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md)'s supply rule; unblocks
`Nat` ([ADR 0077](0077-an-unbounded-end-is-omitted.md) item 5).

## Context

A declared range does two jobs today. It is a **claim**, a set the value is in (ADR 0049: types are
sets), and it is a **representation request**: ADR 0019 item 3, "declare a range above the window, which
moves the value to arbitrary precision and pays for it", and ADR 0034's supply, "a parameter declared
above the word". Measured 2026-10-25 (surface-2026-10-25 §1): `fn inc(n ∈ 0..) { n + 1 }`, applied to an
`x ∈ 0..100` inside a definition whose result is `0..200`, is refused, "a range above the word is a
WIDENING", though every value in it is proven inside the word. So the name of the set ↑0 cannot be `Nat`:
a parameter of that set would be a bignum wherever it was declared.

ADR 0049 already says the two languages meet at one bridge only, ρ, **a representation chosen from
proven sets**. ADR 0019's request predates it.

## Decision

1. **A declared set is a claim, and only that.** At a definition it is an obligation on each argument
   (ADR 0028) and a premise for the body; at a result, a claim decided at the definition (ADR 0072). It
   names no representation.
2. **The representation of a value is ρ of its proven fact**, at every binder: the word when the fact is
   inside S, the unsigned word inside U, limbs when it is a finite set above both, the host's bignum
   when it is unbounded (ADR 0033, 0034, 0035). So `inc` above is computed in the word.
3. **The consent to pay for the rung above stays, and it is a claim**: an operation whose result is not
   proven inside the word is refused (ADR 0019 item 2, bounded by default) unless a claim above the word
   demands its value, as ADR 0034's demand already reads one: a declared result, an ascription, a
   binder's claim `b ∈ S = e`. What changes is **supply**: a definition's parameter declared above the
   word supplies nothing once inlined, since its argument's fact is known there. **Supply stays only where
   the claim is the fact**: an entry point's parameter, whose value the host passes and nothing proves
   smaller (ADR 0028: assumed there), a host result declared above the word, and a constant outside S.
4. **A value held above the word may reach a position in the word only where its fact is inside the
   word**: the refusal "a range above the word is a WIDENING" follows the fact, on the IR, and the term
   checker's sort rule for it (`emit.WiderThanWord` on a declared type) goes, as ADR 0033 moved the
   integer sort to ℤ.
5. **One enforced set per program is unchanged** (ADR 0029): where a claim above the word is not proven,
   it is enforced by `big-fit` at run time, as now.
6. **`Nat` is then TLA+'s**: `set Nat = 0..`, a set of the language beside `Int`, by ADR 0077 item 5's
   own condition. It is added with the build.

## Why not

- **Keep the request** (ADR 0019 as it stands). A claim that decides a representation is the misfit
  ADR 0049 removed elsewhere: two declarations of one set (`0..` and `0..2^62` on a value proven small)
  would compile to different code, and one of them would be refused.
- **Supply from every declared parameter, refused only at a word position** (today). It is the measured
  refusal: the fact of an inlined argument is known, and ignoring it is ignoring a proof.
- **Drop the consent too**, making every value whose fact leaves the word a bignum. That is a hidden
  allocation (CLAUDE.md: never introduce boxing or hidden allocation into the core) and undoes bounded by
  default (ADR 0019 item 2). The claim above the word is the visible consent; only its second job goes.

## Consequences

**As built** (claimset-2026-10-26). Measuring where the request lived found it in four places, none of
them ADR 0034's supply rule, which already read a parameter only on the function compiled:
1. **The IR typed an ascription by its claim**: lowering recorded `(the S e)`'s declared type, and typing
   took it over the operand's, so a definition inlined with a `(Nat)` result made its caller's value a
   bignum (`f` computed `x + 1` in the word and widened it with `big-of` to return a `*big.Int`). Above
   the word the value is now its operand's (`ir/lower.go`).
2. **A result that claims nothing was a word position**, so `fn inc(n ∈ Nat) { n + 1 }`, checked at its
   definition (ADR 0072), was refused for a representation nobody claimed. It is now held as its yields
   are (`ir/big.go`, `resultUnclaimed`), item 3's consent having come from the parameter's claim.
3. **The term checker refused a claim above the word against a result in it by the declared type**
   (`WiderThanWord`), a rule whose premise, that the residual's representation was chosen by the term
   promotion, went with irstep4j. Deleted; the IR refuses by the fact (`ir/big.go`'s word position).
4. **An entry point's unbounded parameter printed `/*int 0 +inf?*/`** in Go, which does not build: no
   program had one. A range above both words now spells as the target's big representation
   (`emit/target.go`).
Then `Nat`, read as `0..` and printed by messages; the four `examples/big/` results are `(Nat)`. Emission
byte-identical on every target; four windows refusals print `Nat` for `0..`.

**Planned before building:**

- To build, spec first: ADR 0034's supply rule restated (item 3), the term checker's sort rule replaced
  by the IR's fact rule (item 4), each with a planted fault: `inc` accepted and computed in the word, an
  entry point's `0..` parameter still a bignum, an unproven operation with no claim above the word still
  refused.
- Expected: emission byte-identical on the corpus except where a declared range above the word is a
  parameter of an inlined definition, which no corpus program has today (to be measured); then `Nat` and
  the four `examples/big/` results written `(Nat)`.
