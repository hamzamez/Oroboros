# 0049 — Types are sets, and the logic that reasons about the computation never prints

Date: 2026-10-10
Status: Accepted (hamza: "I love it, and I agree to all the recommendations"). Decides
[settypes-research.md](../settypes-research.md), which stays a primary document until everything in
it is built. Supersedes [typevars-research.md](../typevars-research.md)'s framing (its surface
survives as a special case, §6.2 of the research), and raises the ceiling
[types-direction.md](../types-direction.md) §3.5 set on its Layer 2.

## Context

typevars-research proposed type variables in declarations on Go's model. hamza asked to rethink the
type system from first principles in the direction of TLA+, Shen and Coq: types as sets with set
operations, a type system that reasons about code and does not translate to it, reaching as far as
decidability allows. settypes-research did that and drew the decidability map.

## Decision

1. **Two languages.** The computation reduces and prints host code. The logic's types are sets of
   values and its claims are formulas about the computation, and it never prints. The one bridge is
   ρ: a value's representation is chosen on the IR from a set it is proven to lie in (ADR 0033,
   generalised to every sort).
2. **The decided fragment is the ceiling of automation**: the Boolean algebra of sorts (the partition
   theorem), semilinear integer sets (Presburger), regular strings with lengths, float intervals,
   products and tagged sums (semantic subtyping), the array property fragment, BAPA and congruence
   closure, combined by Nelson–Oppen. A claim beyond it is refused, or proven by a programmer's
   assertions, invariants and lemmas, each decided within it.
3. **Our own complete decision procedures**, starting with Presburger. No external SMT solver as the
   checker, only possibly as a cross-check in tests (lowstar-lessons.md).
4. **The logic is extended only by definitions and proven lemmas**, never by user inference rules or
   unproven axioms. The host declarations remain the only axioms, checked against the hosts
   (ADR 0022).
5. **Types are static values**: the type operators (union, intersection, difference, comprehension,
   the constructors) are static functions on sets, a named type is a definition, a literal in a type
   position is its singleton, and polymorphism is a static λ over sets instantiated by β at compile
   time.
6. **The order of settypes-research §8**, beginning with a measurement: every obligation the corpus
   makes, run through a Presburger procedure beside today's entailment.

## Why not

- **Type variables on Go's model alone** (typevars-research): the right instance, the wrong frame.
  It gave quantification without the Boolean algebra, so "int or f64 or string" and "a nonzero
  integer" stayed inexpressible.
- **A user-programmable type system (Shen)**: a user rule is an unproven axiom, and its search may not
  terminate. A compiler that chooses representations from types would miscompile on an unsound rule.
- **Proof terms (Coq)**: correct and slow by extraction, and a proof-to-code ratio no program here has
  asked for. Decided steps reach most of the same claims (Dafny, TLAPS).
- **An SMT backend**: heuristic quantifier instantiation over undecidable theories is what makes F\*'s
  proofs unstable. Complete procedures on decidable fragments give the same answer on every build.
- **Keeping the two provers** (the term refinement layer and the IR's intervals): one logic with a
  stack of procedures answers the assessment's question "one prover or two".

## Consequences

- The checking half (the checker and table typing, 1,327 lines; the refinement layer, 5,561) is
  rewritten around inclusion in the algebra and a Presburger core. The IR's interval analysis stays,
  as the fast domain in a reduced product.
- Unions across sorts are represented as ADR 0042's variants; a refinement within one sort costs
  nothing at run time.
- Linearity, relevance and purity stay separate judgements: usage and effects are not membership.
- The surface for all of this, and whether the rest of the language's notation changes with it, is
  the next research (notation-research.md).
