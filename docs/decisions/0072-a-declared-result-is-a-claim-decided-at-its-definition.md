# 0072 — A declared result is a claim, decided at its definition

Date: 2026-10-23
Status: Accepted (hamza, 2026-10-23: "go ahead with item 2. always mathematical and algebraic", the 10-21
plan's second item, assessment-2026-10-21 §5). Amends [postconditions.md](../spec/postconditions.md)
§2's third row, which classed an internal definition's postcondition as "redundant". The first piece of
[ADR 0062](0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md)'s logic that
is built.

## Context

`fn bad(x ∈ 0..100) (0..5) { x + 10 }` compiled and printed 11 when called (observer-2026-10-21 §5),
and compiled without a word when it was not. Exported, the same header was refused: "the body does not
establish 0 <= result && result <= 5; its result is [10, 110]". The reader already makes a result's
range the conjunct `lo <= result && result <= hi` of the postcondition, and postconditions.md §2 decided
a postcondition at an export and called it redundant on an internal definition. Its argument (§3) is
that inlining re-derives at each site whatever the body gives, so no site needs the claim **assumed**.
That is true. But it does not make a false claim harmless. ADR 0062 makes the header the membership
statement f ∈ Π(x ∈ P). R, ADR 0049 makes a claim a formula about the computation, and refinements.md
§3a says an obligation is discharged or the program is refused.

## Decision

1. **The obligation is the contract.** For a definition f with body B, parameter sets P, `where` φ and
   result set R with postcondition ψ, it is C(f) ≜ ∀x ∈ P. φ(x) ⟹ ⟦B⟧(x) ∈ R ∧ ψ(x, ⟦B⟧(x))
   (postconditions.md §1). It holds of the definition, so it is decided **at the definition**, exported
   or not, called or not.
2. **Where it is decided.** `emit.CheckSignatures`, which `gen` and `build` run once per program and
   which already normalizes every definition with a header, calls `ir.CheckClaim`, and that now decides
   the claim first (`checkResultClaim`). The body is lowered alone, each parameter at its declared set
   and the `where` assumed at entry (`assumeWhere`, as at an export), and decided. Then the join of what
   its exits yield must lie in R, and the postcondition's constant bounds must hold (`CheckEnsures`, its
   comparison now `resultWithin`). A contract mark inside the body is its value there: the obligations
   of the calls the body makes stay where the definition is inlined, decided with its arguments.
3. **A product's set is decided factor by factor.** A tuple is in R₁ × … × Rₙ iff each component is in
   its factor. The reader keeps a tuple's factors as `sig.Results` and makes no postcondition of them,
   so each factor is decided here: `(0..2^64-1 × 0..2^64-1 × 0..1)` is three claims.
4. **The words, and above them.** A set held in a word on the target is decided: the signed word, and
   the unsigned word U on Go, whose values the IR's interval domain holds exactly. (The reader makes a
   postcondition only of int64 ends, since it reads before any target is known.) Above every word a
   result's set is representation, enforced on every representation (ADR 0029). There only a stated
   postcondition is decided, as at an export.
5. **A definition over a function is refused, with its reason.** Its contract quantifies over the
   function, and a function has no set to decide it over (the arrow, ADR 0053, is not built). The
   decision it needs is per instance, at each site where staging knows the function, and that is not
   built either. A parameter applied to a λ or to several arguments is surely a function, and is refused
   up front. An unclaimed parameter applied to one value is a table read or an application, one term by
   design (tables.md §3.1), so a failing claim's refusal names both readings.

## Why not

- **A mark at each inlined site** (the plan's sketch, "the mechanism binder claims have"). It decides
  only the instances C(f)[a] the program forms: a false claim whose counterexample no call reaches is
  accepted, and an uncalled definition's claim is decided by nothing, a path nothing runs. It is the
  right mechanism for P, the caller's duty (ADR 0028). Q is the callee's, and the callee is in hand.
- **Keeping postconditions.md's "redundant".** It is redundant as an assumption (§3's theorem, kept).
  As a claim it is the program's own statement, and a statement nobody checks is worse than none: it
  reads as documentation and can be false.
- **Assuming the proven claim at each site** (feeding R to the IR as a fact on the call's value).
  Inlining re-derives it, and an assumption is a new premise to witness. No program names a refusal that
  it would clear (rule 5 of the plan).
- **Deciding above the word.** The IR's domain stops at 2¹²⁶, and the bignum rung's values reach it.
  So a claim there would be the refusal-in-front-of-nothing shape `core/read.go` already declines, and
  ADR 0029 enforces the set on every representation instead.
- **Accepting a definition over a function unchecked.** That is the hole this decision closes, kept for
  one shape of program. No program in the corpus writes one.

## Consequences

- `bad` is refused called, uncalled and exported, with the interval that disproves it. A refusal names
  the decision and the three remedies: narrow a parameter's set, state a `where`, or widen the result.
- In the corpus, 22 claims are now decided at their definitions, on Go: 10 definitions, the three
  `num/u128` helpers in four programs each, `sat` in three, and `near_top`'s unsigned result. All are
  true. Emission is byte-identical
  ([resultclaim-2026-10-23](../../gauntlet/results/resultclaim-2026-10-23.md)).
- Each claimed definition is lowered once more, in the program-level check. It is small, and the
  compile-time gate did not move.
- `cmd/intervals`, a reporting tool, does not run `CheckSignatures`. Every driver that emits code does.
