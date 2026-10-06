# 0042 — A variant's value is its tag and its slots, everywhere it must exist as data

Date: 2026-10-06
Status: Accepted (hamza: "go ahead and continue the work on errors", on the proposal in
unit-2026-10-06 §4). Realizes data.md §5.5.5 (Theorem R) at every join point, not only at a
signature, with the slots taken from the declaration.

## Context

Inside a program reduction removes every sum. Where a sum must exist as a value, it was the pair
(tag, payload), one slot, and three legal programs were refused:
- a loop over a fallible step, whose exits are `(ok acc)` and `(err e)` with `acc : int` and
  `e : error`: *a loop's exits are int and string*. The error model's read loop is this shape;
- `(variant msg (text string) quiet)` returned at a boundary: a nullary constructor's payload was the
  literal 0, which fits only an `int` slot (W5);
- `(result (tuple) E)` at a signature, the unit's payload counted as a second type.

data.md §5.5.5 had specified the representation for a signature, `(tag, S₁ … Sₖ)`, one slot per
distinct payload type, with an unselected slot unconstrained. Its Theorem R: decoding reads only the
slot the tag selects, so `dec ∘ enc = id` whatever the other slots hold. It was "not built for k > 1".

## Decision

1. **A variant's value is (tag, s₁ … sₙ)**: one slot per distinct payload type of its **declaration**,
   in declaration order, and at least one. `(variant (result T E) (ok T) (err E))` has two, T and E.
   The slots are the declaration's because a constructor's term is built at declaration time, before
   any instance is known, and a term must have one shape in every instance.
2. **A constructor writes its payload into its own slot and `#any` into every other.** A nullary
   constructor writes `#any` everywhere. `case` binds, in each arm, the slot its constructor writes,
   and nothing else.
3. **`#any` is the zero of its slot's type**, a language constant no source can write. It agrees with
   every type in the checkers. On the IR it is a value whose type the slot decides: a scalar zero
   becomes a constant (0, `false`, `0.0`, `""`), and a non-scalar one is printed as its host's zero
   (`nil` or `*new(T)` on Go, `null` on Java and JavaScript, 0 on x86).
4. **The unit's one value is represented by 0**, the singleton {0} ⊂ int (data.md §3.6), so a unit
   slot is an `int` at a boundary and the constant 0 on the IR.
5. **At a signature** the results are the tag and the declaration's slots under the substitution.
   `(result int int)` is `(int, int, int)`: two slots stay two when an instance makes their types
   equal.
6. **What the analyses know of a slot is what its selecting exits wrote.** An `#any` tail is ⊥ in the
   join of a slot's tails. A name whose tails are all constants ranges over exactly that set, so a
   guard naming a value outside it opens an infeasible path, on which the facts entail anything (ex
   falso, in `entails`, before any other rule).

## Why not

- **Slots per distinct instantiated type, in canonical order** (data.md §5.5.5 as first written). It
  needs the instance to build a constructor's term, and a constructor is a definition δ unfolds before
  types exist. Merging equal slots per instance is Theorem R's sharing corollary, an optimisation of
  this representation, and is kept for later: no existing program crosses a boundary with two slots of
  one type.
- **One slot, typed as the join of the payloads.** A join of `int` and `error` does not exist in any
  host, and an untagged union is not a coproduct (type-algebra.md §3).
- **The literal 0 kept as the nullary payload, with `#any` only for the unit.** A nullary constructor
  is a constructor with payload 1, `quiet ≅ (quiet (tuple))`, so there is one case, and the literal fit
  only an `int` slot.
- **An unconstrained component typed as the unknown on plain tuples.** Unsound: a tuple's component is
  read without a tag, so a filler would be observed. The filler is sound only under the tag discipline,
  which only a sum has. That is why only a constructor writes `#any` and only `case` reads a slot.
- **A boxed or tagged-union host type** (an interface on Go, a sealed class on Java). It allocates
  where a product of registers does not, against "never introduce boxing" (design-direction.md §2).

## Consequences

- **Every existing emitted file was byte-identical** when it landed: `#any` at an `int` slot settles to
  the literal 0 the nullary payload always was, and an enum keeps its one slot.
- **A sum leaves a loop, a scope or a function with payloads of any types.** `try` in a loop over two
  payload types is built (`cases/sum-slots.oro`, four targets).
- **Three refinement rules came with it**, each a gap the sum exposed rather than a rule for sums: a
  table literal's length, a finite value set beside the interval hull, and ex falso first in
  `entails`. A fourth fix is the Go printer's conversion of a literal's variable elements to a narrow
  storage type, which a refusal had hidden.
- **Two slots of one type at a boundary are not merged**, as named above.
