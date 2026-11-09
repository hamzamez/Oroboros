# 0097 — A table born full takes a region with no zero fill

Date: 2026-11-09
Status: Accepted (hamza, 2026-11-09: "Fill elision instead", replacing R1c, frame slots, after the
measurement below). Built with [fill-2026-11-09](../../gauntlet/results/fill-2026-11-09.md). Amends
[ADR 0096](0096-a-values-storage-is-chosen-by-size-class-and-released-at-its-death.md) item 6's order.

## Context

ADR 0096 planned R1c, a table of static length in a frame slot, to remove its allocation. After R1b's
tokens, a loop's steady state allocates nothing: a token is a reused slot. What remained was measured
on 10⁷ limb additions: 183 ms with reuse, 145 ms with the reused regions not zeroed. That zeroing,
about 1.3 ns a table, was 21% of the loop. A frame slot must be zeroed too, since `region`'s contract
(every byte 0) is what a `build`'s zero fill relies on. So frame slots could only have replaced
`RtlZeroMemory` with inline stores.

## Decision

1. **region = zero ∘ region_raw**, and the zero is a dead store when every element of the table is
   written before any is read (ADR 0084's U3: a table is born full). Σ_OS gains `region_raw(b)`, b
   bytes of any contents. windows defines it by `region`'s size rule: `VirtualAlloc` from 64 KiB, whose
   pages are zero anyway, and `HeapAlloc` without `HEAP_ZERO_MEMORY` below. So release ∘ region_raw
   is the identity on the heap too.
2. **Born full, by construction or by one theorem** (`ir/fill.go`):
   - a rule (`tabulate`) and an array literal write every element by construction;
   - a `build` is born full by **the written prefix**. Its body runs a loop carrying the buffer and a
     counter i from 0 by +1, writes b[i] on every continuing path without reading b, and leaves only by
     a branch on i ≥ n, the build's own length. By induction [0, i) is written at each iteration's
     start, so [0, n) is written at the exit, before any read. The descending counter, from n − 1 by
     −1 leaving on i < 0, is the same theorem on the suffix.
3. **`alloc-raw` and `reuse-raw`** are the IR's operations for such a table: `lang.allocate_raw`, and a
   token taken without zeroing. A table the theorem does not cover keeps its zeroed region, so a fill
   not proven dead costs time, never an answer.
4. **Frame slots wait for a measurement**: the first iteration's allocations of a nested loop, if a
   program measures them.

## Why not

- **R1c as planned.** It would remove the steady state's allocations, which R1b had already removed,
  and would still owe the zeroing. Its provenance rule, keeping frame and heap regions out of one
  token, would have been new machinery for at most about 15% on the measured loop.
- **Inline zeroing by `llvm.memset`.** For a constant length LLVM may still call the C runtime's
  `memset`, which this target does not link, and `llvm.memset.inline` needs an immediate length the
  printer does not always spell. Not zeroing beats zeroing faster.
- **A general array must-write analysis** (array dataflow, Feautrier 1991). It decides more, at more
  cost. The written prefix is the shape every limb operation but `mul` has, and a program that needs
  more can name it.

## Consequences

- A fresh table a rule or a literal makes, and a build the theorem proves, is allocated without
  zeroing and reused without it. On the limb loop that removes three of four zeroings an iteration,
  and reaches the time of no zeroing at all.
- Σ_OS has six operation symbols.
