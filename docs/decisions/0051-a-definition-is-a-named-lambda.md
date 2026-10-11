# 0051 — A definition is a named λ, and the λ carries its claims

Date: 2026-10-11
Status: Accepted (hamza, 2026-10-11: "agreed to all recommendations, go ahead with 1b"). Amends
[spec/abstract.md](../spec/abstract.md) §2.4, §2.6 and W3, under
[ADR 0050](0050-the-abstract-syntax-fits-the-semantics.md). Built for programs in step 1b
([abstract1b-2026-10-11](../../gauntlet/results/abstract1b-2026-10-11.md)).

## Context

Step 1b of abstract.md §13 reads declarations into trees, and the trees' shape had to be fixed first.
Four questions were open:
- surface-design.md's Finding 1: `def` is a λ with a name;
- its Finding 2: every claim position is a set;
- how several results are named;
- whether target files belong to the same step.

Step 1a left a fifth: a λ applied by hand to one argument, with a jump in its body.

Measured on the corpus: all 109 program `sig`s have a λ definition in the same file. None has an
unnamed parameter or a different arity. Four bind their parameters under other names than their
definition (`dot`: `p q` against `a b`), and one is written after its definition. 87 `sig`s return a
tuple, 79 in target files and 8 in programs. Target files hold 870 `sig`s.

## Decision

1. **A definition is a name bound to a value, and a definition of a function is a named λ** (D1).
   The λ carries what it claims: each parameter's set, the result's set, a precondition and a
   postcondition. A constant is a name bound to any other value. A module is then a group of
   definitional binders, the binder kind `let` has.
2. **The tree keeps a set per binder, plus φ and ψ; their meaning is two sets** (D2):
   P = {x̄ ∈ S₁ × … × Sₙ : φ} for the arguments, and R = {result ∈ U : ψ} for the value. The structure
   stays because ρ chooses a value's representation from its own set, and ADR 0028 checks a
   parameter's set on its argument and a `where` on the call.
3. **The result is one binder whose pattern may be a tuple** (D3): `tuple(q, r) ∈ int × int` names
   the components of several results without a second form. Today's notation writes the binder as
   `result`.
4. **Programs first** (D4). Step 1b turns program declarations into trees. Step 1c turns target files'
   `sig`s into definitions with a host body, under their own byte-identical gate.
5. **A λ's body is never a tail of a loop around it** (D5): `((fn (x) (again x)) e)` is refused, as
   W3 says, and `let` is how a jump is bound. Today's walker accepted the redex because it could not
   tell it from the one a `let` reads as; a tree can, since `let` is its own operator.

## Why not

- **Two operators, a λ and a definition that carries claims** (abstract.md as first written): two
  forms that differ by a name. That is the misfit fit-research counts as a synonym, and it leaves the
  top level a binder kind of its own.
- **Only the two sets P and R in the tree** (Finding 2 taken literally): ρ and ADR 0028 would have
  to project P onto each parameter. That projection is computable when P is Presburger, and not for
  an arbitrary comprehension.
- **A group of result binders, as Go's named results**: a second form for what a tuple pattern
  already is.
- **Target files in the same step**: 870 declarations with their host clauses (template, import,
  `checked`, `jump`, cells, `variadic`), read by a second reader (`emit/target.go`), in one step with
  one gate. Two steps give two checks that can each fail.
- **Keep accepting the hand-written redex**: it is β-equal to the `let`, but it puts a λ between a
  loop and its jump, and no program writes one.

## Consequences

- A `sig` is the claim of its definition's λ wherever exactly one claim and one λ definition of its
  arity share a name. Otherwise it stays a claim apart, and Load decides it as before.
- Today's notation writes a claim apart from its λ, binding its own names, so the claim keeps the
  names it wrote, α-equivalent to the λ's, and the position it was written at. Both exist only so D
  gives today's forms back, and both go when the notation writes one binder list.
- An anonymous λ may carry a claim in the tree. No notation writes one yet, and the checker obliges
  it at application, as it obliges a definition's at each call it is inlined into (ADR 0028).
