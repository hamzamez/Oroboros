# 0134 — An `again` after a join point is accepted, by Theorem J's rewrite, measured against sharing

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19, on [joinjump-research.md](../joinjump-research.md): "the choice
should be based on measurements, pick the most likely best choices, even if we have to change it
later if the measurements say so"). Amends [tables.md](../spec/tables.md) §2.5's not-built list and
[ADR 0031](0031-a-builds-result-is-a-product.md) §4's join point. **Not built: B's premise is false in the
core** ([joinjump-2026-11-19](../../gauntlet/results/joinjump-2026-11-19.md)). The core's `again` is the
nearest loop's, so k copied into a loop P's exits jumps to P, not to L. Theorem J holds in F_J, whose
jump names its label. The derivation lands on A read as iteration.md §2's fourth tail form widened
to a producer's continuation, a reversal for hamza.

## Context

`(a, b) = loop … { … }; …; again(…)` is valid J, and `emit.CheckJoins` refuses it on the residual:
an `again` in a join point's body. The program meant is decided already. Theorem J
(joinjump-research §2) gives it the meaning of the join point's body copied into each of the inner
loop's exits, the commuting conversion ADR 0076 used. Only the representation was open, and the
refusal costs `wc` 16% against nested Go (wc-2026-10-24 §4). No candidate changes the surface: each
accepts the same program text, and differs in the compiler and the emitted code.

## Decision

1. **The program is accepted** where the tuple pattern stands in a tail of the enclosing loop's body.
   In any other position, an argument, a binding's value or an operand, it stays refused: the jump
   would leave an expression (ADR 0076 item 3).
2. **Its representation is B, Theorem J as a rewrite**, before any walker of a clause chain reads the
   residual. `(P (fn x̄ k))`, with an `again` of an enclosing loop in k, becomes P with k placed at
   each of its exits, the exits' components for x̄, and never at P's own back edges. The walkers then
   see the four forms they know, and soundness follows from the theorem.
3. **The measurement that keeps it**, taken when it is built:
   - `wc` written nested, against its flat form and against hand-written Go's nested form, on Go
     and windows;
   - the emitted code's size, and the compile-time gate.

   B stays if the nested `wc` is within the ~15% noise floor of Go's nested form and the copy is not
   measured slower than a single body. If B's copy costs a measured difference, the representation
   moves to **C**: B on the core, the copies shared again as one join region on the IR. That move is
   a new ADR.
4. **The planted faults** are joinjump-research §5's: k placed at P's back edges too, and an
   elimination in a non-tail position accepted. Each must fail before the rewrite is trusted.

## Why not

- **A, the fifth form in every walker**: about 20 walkers to read and extend, and one missed is
  unsound, not imprecise. It is also the form most likely to stay wrong unnoticed.
- **C first**: it pays for recognizing and re-sharing equal copies before any measurement says the
  copy costs anything. LLVM's and Go's compilers merge identical tails themselves, so the copy may
  be free after them.
- **D, keep the refusal**: 16% measured on `wc`. Every program with a loop per chunk inside a loop
  over chunks carries all its state in one loop instead.

## Consequences

- No word and no grammar changes. tables.md §2.5, j.md §2.8's tail rule and CLAUDE.md's list of
  `again`'s positions gain "the body of a tuple pattern over a loop, in a tail" when it is built.
- `wc` can be written nested, as Go writes it. Rewriting it so is the measurement's first program.
