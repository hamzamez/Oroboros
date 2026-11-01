# 0087 — A result's set is a claim by its ends inside the word, decided on ℤ's facts

Date: 2026-11-02
Status: Accepted. **Reverses** the rule of [scalarrange-2026-08-31](../../gauntlet/results/scalarrange-2026-08-31.md)
that a result's range past the word is "a REPRESENTATION declaration and not a contract" (`core/read.go`).
**Amends [ADR 0029](0029-above-the-word-one-set-on-every-representation.md)** in where its run-time
enforcement is relied on. Completes [ADR 0072](0072-a-declared-result-is-a-claim-decided-at-its-definition.md)
and [ADR 0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) for results.
Built: [resultclaim-2026-11-02](../../gauntlet/results/resultclaim-2026-11-02.md).

## Context

A declared result is a claim decided at its definition (ADR 0072), and a declared set is a claim whose
representation the facts choose (ADR 0079). Since scalarrange-2026-08-31 the reader has made a result's
range a postcondition only when both of its ends fit the word. Its argument was that the interval
domain reports a bignum as [−∞, +∞], so a guarantee above the word could never be discharged. Such a
range was left to ADR 0029's run-time check.

That rule left two kinds of result decided by nothing:
- a **range with an infinite end**, `Nat` or `10..`: α of it is ℤ (ADR 0029 item 2), "enforced
  nowhere";
- a **range past the word held in the word**: ADR 0029's check runs on the representation above the
  word, and a value the facts keep in the word never reaches it.

So `fn h(y ∈ -5..5) (Nat) { y }` and `fn h(y ∈ -5..5) (0..2^100) { y }` both compiled and returned −5
(grids-2026-11-01). A parameter's range was never affected: at a call the finite side of an open range
was kept as a conjunct, and refused when unproven.

The premise was the term analysis's, not the IR's. The IR's domain is over ℤ̄, and a finite end
survives beside an infinite one, except where three of its rules gave ⊤:
- a product with an unbounded factor;
- a range type with an infinite end;
- a claim decided after the rung above the word, where a bignum's operation is a host call.

## Decision

1. **A result's set S is the meet of two half-spaces**, [a, +∞) ∩ (−∞, b], so `result ∈ S` is the
   conjunction of their claims. **Each end inside the word is a postcondition**, the dual of a
   parameter's, which is assumed. This holds whatever the representation.
2. **It is decided on ℤ's facts, before any representation is chosen.** A representation is an
   abstraction function (ADR 0083), so the value the claim is about is the same on every
   representation. Exports are decided after the first decision; definitions alone, on the body
   decided alone, as before.
3. **The domain keeps a product's sign when one factor is unbounded.** The hull of the corner products
   over ℤ̄, with 0·(±∞) = 0, is sound for intervals of ℤ, whose infinite ends are never attained (Hickey,
   Ju and van Emden 2001). A range type keeps its finite end, and an ascription in a set with an infinite
   end tells nothing, since α of it is ℤ.
4. **An end past the word stays ADR 0029's**, enforced at run time through α. ADR 0029's check also
   stands for an end inside the word, in one case only: the sign end 0 of a bounded range past the word,
   whose α = N_b checks the sign exactly, on a value whose facts leave the word, so it is held where
   the check runs. In every other case an end the facts do not prove is refused, naming it.

## Consequences

- `Nat`, `10..` and `..-1` on a result are claims, decided. The four `Nat` programs of `examples/big`
  are proven on the facts. A definition whose parameter has no set and whose result is `Nat` is refused,
  since its parameter ranges over ℤ (ADR 0072).
- `big-negmid`, whose result 5·10³⁰ is held above the word and whose facts do not see `a − b + b = a`,
  keeps ADR 0029's sign check at run time, as before.
- A separation of ℤ by bounds can be its range, `{ y ∈ Int : y >= 0 }` = `Nat`, since a result in an
  open range is now decided. It is numbers.md §2's normal form, held back until this was true.
- More precise refusals: an unproven product with an unbounded factor reports its sign, `[6, +inf]`
  where it reported `[-inf, +inf]`.

## Why not

- **Keep the rule, and decide `Nat` by run-time checks.** α of a range with an infinite end is ℤ, so
  there is nothing to check at run time, and a check on a value held in the word would be a new
  enforcement on every operation of the word for a claim the facts already decide.
- **Decide every end statically, the sign end above the word included.** This would refuse
  `big-negmid`, a correct program whose claim ADR 0029 enforces exactly, because the interval domain is
  not relational. A refusal standing in front of a check that exists is the shape scalarrange-2026-08-31
  meant to avoid, and it remains right there, and only there.
- **Model a bignum in the domain separately**, its sign as a second fact. The interval over ℤ̄ already
  carries the sign once its rules keep finite ends; a second domain would state the same fact twice.
- **Decide the claim on the realized function**, after the rung. A bignum's operations are host calls
  there, whose facts are ⊤ unless each is given ℤ's transfer, which is the ℤ-level function's
  decision repeated on a different presentation.
