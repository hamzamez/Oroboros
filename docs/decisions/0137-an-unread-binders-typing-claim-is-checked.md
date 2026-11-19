# 0137 — An unread binder's typing claim is checked

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "it should check it, how I will leave it to measurements, you
can go either way, with possibility of change on the future"). Completes
[ADR 0049](0049-types-are-sets-and-the-logic-never-prints.md)'s "a claim is an obligation" for a
claim whose set is no range (claims-2026-11-16, deadclaims-2026-11-18's owed item). **Not built.**

## Context

`fn f(n ∈ Int) { s ∈ String = n; 0 }` compiles: the claim is false, but s is never read, so reduction
drops the binding and the type checker, which runs on the residual, never meets it. A range claim
survives as a mark (`#req`) and is decided unread; a set that is no range has no mark.

## Decision

1. **Every claim is decided, read or not.** An unread binder's typing claim is checked.
2. **The method is a typing mark, first**: the claim kept in the residual as a mark the type checker
   decides, as a range's `#req` is kept, `(#is "S" e)`, erased once decided. No surface change.
3. **Measured, and changed if the measurement says so**: compile time on the corpus, and the
   residuals unchanged where no such claim is written. If the mark costs more than the gate allows,
   the alternative is to type-check the claim on the trees before reduction.

## Why not

- **Type-check before reduction, first**: a second type checker, on terms the reducer has not
  specialized, for a claim that one mark covers.
- **Refuse a claim on a binder nothing reads**: changes what a claim means, where the claim itself
  is the thing to check.
