# 0101 — A library instance is a function of the program

Date: 2026-11-10
Status: Accepted (hamza, 2026-11-10: "go ahead with item 1", on the standing measure's list, after
addsub-2026-11-10 measured the cost of inlining twice). **Amends
[ADR 0035](0035-a-compiler-library-is-a-theory-checked-once.md)**: "a use is an inlined instance"
becomes "a use is a call of the instance, a function of the program". Moves "a limb instance emitted
once as a function" off the plan's deliberately-not-next list. Built in steps; see the plan in
CLAUDE.md. **Amended by [ADR 0102](0102-factoring-never-changes-an-answer-or-a-verdict.md)**: a
call whose argument is outside its parameter's set, and an instance its contract does not prove, is
inlined, never refused.

## Context

ADR 0035 checks a library definition once as a theory, then inlines each use, so the site's facts
decide its word arithmetic. Inlining multiplies code by the number of uses:
- `big-sign` emitted 19,399 lines of LLVM IR for one subtraction (signlimbs-2026-11-09);
- after A and S became separate loops, a program of 2,000 quotients reached 29,579 operations and ten
  minutes in clang (addsub-2026-11-10).

Inlining also leaves no call for LLVM's parameter `noalias`, the sound form of the views' separation
(lenonce-2026-11-10).

## Decision

1. **An instance is a function of the program**, and a use is `call f ā`. It is the same value as
   the inlined body, since β is an equality, so the change is in where the arithmetic is decided, not
   in what any program computes. ir.md §2.5 specifies it.
2. **The callee is decided once, under its parameters' sets.** A numeral's parameter and result are
   `Array(0..2³²−1)`, true since its header is 2m + s (digitfacts-2026-11-10). An argument's fact
   lying in its parameter's set is an obligation at each call. The result's element range is a claim
   the callee's stores prove.
3. **Ownership across a call is the frame rule**: arguments borrowed, the result a birth owned by the
   caller. W13 checks the callee's side, every returned table born in it. An instance that would
   return an argument returns a copy, as `rem` must when |a| < |b|.
4. **In steps, each measured**:
   1. K(ℕ\*)'s instances on the LLVM layer, without reuse across the call;
   2. reuse across the call, by destination-passing: the dead table's token a linear argument the
      callee's allocation takes by fit (Lorenzen, Leijen and Swierstra 2023, FP²);
   3. parameter `noalias` on the callee's tables;
   4. the fixed limbs' instances, if a measurement names them.

## Why not

- **Keep inlining.** Code is multiplied by uses, and every improvement to a library loop multiplies
  it again: measured twice.
- **Outline in the printer.** The IR has already decided each inlined copy under its site's facts;
  re-folding copies into one function would need those facts re-proven, which the IR's decision is
  for.
- **Decide the callee per call site** (a call keeps the site's facts). That is inlining's decision
  without inlining's code, but it needs a function per distinct fact set. One function under its
  parameters' sets is the theory ADR 0035 already checks.

## Consequences

- A program's text holds its entry and each instance it calls; every printer reprints from it.
- The decision gains an obligation per call: its arguments in its parameters' sets.
- Step 1 loses reuse across a call (R1d's tokens reach no allocation inside a callee) until step 2.
