# 0115 — A rule at an export's boundary is its extension

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "go ahead with item 4", the book's finding 6).

## Context

A table given by a rule, `array { e : i ∈ 0..n-1 }`, is a function on 0..n − 1 given by its term.
In the interior it fuses: an index is β, `len` is n, and `alloc` writes it to memory where the
author asks (tables.md §2). An export that returned one was refused, "a table rule that was never
allocated", although `alloc(…)` of the same rule built. construction.md says one materializes
"only at a boundary — handing an array to a host function, or returning one", and g5 §1 says
"representation is free in the interior and fixed at the boundary".

## Decision

**A rule in an export's result position is its extension, `alloc(rule)`.**
- At an export's boundary a table has one representation, the host's array (g5 §1). The rule's
  only meaning there is its extension.
- So the compiler writes the `alloc` the boundary already fixes (`emit.MaterializeResult`). The
  rewrite applies at each value tail of the export's body: an `if`'s arms, a `let`'s body, a
  continuation's body, and a tuple's components.

Nowhere else: in the interior, a rule that is neither fused nor `alloc`ed is still refused, since
there materializing is a choice whose "cost is the point" (construction.md).

## Why not

- **Keep the refusal, the author writes `alloc`**: the program has one meaning, and the refusal
  asks the author to write what the boundary fixes. tables.md's case for a visible `alloc` is the
  interior, where it is a choice.
- **Materialize every rule that reaches lowering**: a rule bound and read twice in the interior
  would be allocated with nothing written. That is the hidden allocation the core never introduces.
- **Materialize at a kept function's boundary too** (ADR 0113): that boundary is the compiler's,
  not the program's. An allocation there would be paid for a choice no one wrote, and P4 inlines
  the definition instead. A segment passed to a kept function (`pe`) stays owed.

## Consequences

- `export fn sq(n ∈ 0..100) (Array(Int)) { array { i * i : i ∈ 0..n - 1 } }` builds on four
  targets: `TestARuleAtTheBoundaryIsItsExtension`, with `PlantRuleAtBoundary`.
- The corpus's emission is unchanged: no export returned a rule, since none could.
