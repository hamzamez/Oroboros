# 0100 — A dead region serves a request it fits

Date: 2026-11-10
Status: Accepted (hamza, 2026-11-10: "go ahead with R1d"). **Amends
[ADR 0096](0096-a-values-storage-is-chosen-by-size-class-and-released-at-its-death.md) item 6**: a
table of run-time length takes a dead region by fit, in place of the data stack. Built with
[fitreuse-2026-11-10](../../gauntlet/results/fitreuse-2026-11-10.md).

## Context

ADR 0096 chose storage by size class and gave a table of run-time length that dies inside the region
that made it a data stack, popped at the region's exit. R1b's reuse (release ; region ≡ zero) handed
a dead region to the next allocation of *equal* size only, a static size by the facts. K(ℕ\*)
(ADR 0099) made run-time sizes the common case: every numeral's table has its operands' size.

Its loops do not fit a stack. In fib, (a, b) → (b, a + b) carries b out of the iteration that made
it while a + b is born, so the lifetimes interleave and no last-in-first-out order holds them.

## Decision

1. **A dead region serves any request it fits.** A region answered for b bytes is a block of at least
   b bytes, and `region`'s contract is b fresh zero bytes. So for b′ ≤ b in one size class:

       release(p, b) ; q ← region(b′)   ≡   q ← zero(p, b′)

   The class is Σ_OS's seventh operation symbol, `class(b)`, the rule `region` and `release` choose
   their allocator by: 65,536 bytes on windows, VirtualAlloc above and the process heap below.
   Equality of classes is what makes the table's own later `release(p, b′)` give the block back by
   the rule that made it. A reuse keeps the class, so by induction it is the region's.
2. **The token carries its table's element count k**, a lower bound on its region's size (len·w + 8
   of the table it was). `reuse_fit(tok, k, n, w)`, a term of the LLVM layer over Σ_OS, compares
   n ≤ k and the classes at run time. It takes the region when they hold, and otherwise gives it back
   and allocates. A fit that fails costs one comparison and the allocation that would have happened.
3. **Where both sizes are static, R1b's equality stands**, and its emission is unchanged. A size of
   the run's on either side takes the fit form, locally and across a back edge.
4. **W12 re-derives every token by fit**:
   - it is null or the storage of a table not used after it is handed on;
   - the count beside it is that table's own `len`;
   - its width is the table's element width;
   - it is consumed once on every path, a discard giving it back by the same count.

## Why not

- **The data stack ADR 0096 chose.** It holds a value that dies in the region that made it, in LIFO
  order. A loop's carried value outlives its iteration while its successor is made, so the loops
  that make run-time sizes, the numerals', are not LIFO.
- **Equal sizes only, with the size made static.** A K(ℕ\*) value's size is its operands', a run-time
  quantity no fact makes a singleton.
- **Record each region's true capacity** in the table, so slack is reused. It costs a word per table
  and a layout change, for slack the measured loops do not have: a numeral's size grows slowly, so
  its dead predecessor's region almost always fits as it is.
- **Reuse across classes**: the later release would free a VirtualAlloc block with HeapFree, or the
  reverse.

## Consequences

- K(ℕ\*)'s loops reuse their dead tables' regions, and Algorithm D's per-digit remainders reuse
  theirs. The machine steps of a quotient by a big value took 683 tokens.
- Measured, the allocation was not where K(ℕ\*)'s time went (fitreuse-2026-11-10): fib gained about
  6%, the others nothing. The cost is the per-digit computation, and the result names it.
- Strings and containment (R1e) and frame slots for a nested loop's first iteration still wait on
  their triggers.
