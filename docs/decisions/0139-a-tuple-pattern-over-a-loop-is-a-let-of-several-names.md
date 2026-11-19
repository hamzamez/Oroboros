# 0139 — A tuple pattern over a loop is a `let` of several names

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "since it does not change the surface let's go with the
recommendation, and see where it lands performance wise"). Reverses the representation of
[ADR 0134](0134-an-again-after-a-join-point-is-theorem-js-rewrite.md) (B, Theorem J's copy). Its
decision 1, that the program is accepted in a tail, and its measurement, nested `wc` against
nested Go, stand. **Built 2026-11-19** ([joinlet](../../gauntlet/results/joinlet-2026-11-19.md)):
nested `wc` 1.013× nested Go. Decision 1's rewrite is a reading (`emit.joinLet`): every walker of a
clause chain takes the pattern as the `let` it is, and the term is not rewritten, since the
value-level walkers already handle the pattern and would each have had to learn an m-name `let`.

## Context

`(a, b) = loop … { … }; …; again(…)` is refused: an `again` in a join point's body
(`emit.CheckJoins`). ADR 0134 chose to copy the body into the inner loop's exits (B). In the core,
`again` is the nearest loop's, so the copy would make the body's `again` the inner loop's
(joinjump-2026-11-19). Theorem J holds in F_J, whose jumps name their loop.

The same program with one name, `x = loop { … }; …; again(…)`, is accepted and proven today. It is
`(let P (fn (x) k))`, a `let` whose value is a loop and whose body ends in the enclosing loop's
`again`. Every walker of a clause chain reads it:
- P's own `again`s are P's, since P's body is P's;
- k is a tail of the enclosing loop, since a `let` binds, and binding may wrap an `again`
  (iteration.md §2).

## Decision

1. **A tuple pattern over a producer whose body jumps out is a `let` of m names**,
   `(P (fn (x₁ … xₘ) k))` read as `(let P (fn (x₁ … xₘ) k))`. Its meaning is the eliminator's: P
   runs once, and k runs once with P's m components bound (ADR 0031's join point, ADR 0027's "a
   binding of several names is a binding"). The rewrite is made before any walker reads the
   residual, and only where k jumps out: every other tuple pattern keeps its form, so their
   emission cannot change.
2. **Every walker of a `let` takes m names.** The audit covers each site that matches a `let`
   (12 by kind, and those that match it by name): each one binds m names to P's m components, or
   refuses the m-name form. A site that silently takes one name is the fault. Two sites assume
   one name (`projectTail`, `refine`), and the lowering binds only the first value. Each rule
   changed gets a planted fault that makes it unsound or refuse.
3. **The measurement is ADR 0134's**: `wc` written nested, against its flat form and Go's nested
   form, on Go and windows, with the code's size and the compile-time gate. If the nested form is
   not within the noise of Go's nested form, that is recorded, and the next step is named.

## Why not

- **B, the copy**: not the core's term (joinjump-2026-11-19).
- **A, the fourth tail form widened to a producer's continuation**: the 18 walkers of a host call's
  continuation would learn a head that is a loop, which none of them analyses today. The `let`
  form's walkers already analyse a loop at a `let`'s value, and learn only the arity.
- **A labelled `again`**: every walker of `again` changes (28 sites in 20 files), and the core
  gains a jump to an outer loop, against "never add unstructured control flow".
- **D, keep the refusal**: 16% on `wc` (wc-2026-10-24).

## Consequences

- No surface change. j.md §2.8's tail rule and tables.md §2.5 gain "the body of a tuple pattern
  over a loop, in a tail" when this is built.
- A `let` of several names exists in the residual, which only this rewrite makes.
