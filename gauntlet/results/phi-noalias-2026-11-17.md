# φ's fourth kind: a lent buffer as `noalias`

2026-11-17. Item 5 of [assessment-2026-11-16](../../docs/assessment-2026-11-16.md) §5, after
[phi-ptr](phi-ptr-2026-11-17.md).

**A kept definition's lent buffer is printed `noalias`.** It is R2's theorem, and R2 has a planted
fault now. Neutral where measured, with identical code size.

## 1. The algebra

- **noalias(p)** (LangRef): during the call, memory accessed through p is not accessed through any
  pointer not derived from p, if either access writes.
- **A lent buffer satisfies it.**
  - R2 (core/place.go) hands a buffer to one call once, so no other argument names its region.
  - A table frozen from the buffer is no longer a buffer (R3): it is either a copy, another region,
    or the buffer itself, which then cannot be lent.
  - The callee's other pointers are its own allocations, distinct regions, and string literals,
    read-only globals.

  So during the call the buffer's region is reached only through the parameter. ∎
- **Not claimed of a read-only table**: two such parameters may be one table. noalias would still
  hold there, since neither writes, but the claim would rest on the callee writing nothing through
  it, a fact no check states. So it is not made.

## 2. The witness

`TestALentBufferIsNoalias` (cmd/build, windows):
- **the premise, twice**:
  - R2 refuses `shift(b, b)`, under its name;
  - with R2 planted off (`PlantR2Off`, new), linearity refuses the same call, because the second
    position reads b as a frozen table and the callee stores into it.
- **the attribute's place**: with `shift(b, c)` on two regions and `shift` kept (five calls make
  it pay, P8), the printed `shift~lent` takes `noalias` on its lent buffer alone.

**The first witness** was to run `shift(b, b)` with R2 off and see LLVM's vectorized answer differ.
It met the second refusal, which no longer makes it a witness of the attribute. That two
independent checks both refuse aliasing is the stronger statement.

## 3. What failed first

0. **noalias was printed on results too, which is unsound, and I accepted it.** The pointer
   attributes are one function, `tableAttr`, called for parameters and for the single result. On a
   result, noalias says the pointer is fresh, aliasing nothing the caller holds, as malloc's is.
   The gauntlet's `smooth_into` writes its lent `dst` and returns it, so the result *is* the
   caller's pointer. Two files carried it: `examples/native/smooth-go` and `examples/kara/workspace`.
   My check of the accepted emission asked only that every changed line differ by the attribute
   alone, not where the attribute went. That check was too weak. The one-line diff of
   `gen_smooth_native.ll`, which the gauntlet step asked me to regenerate and time, showed it.
   noalias is now a parameter's only. `PlantNoaliasResult` restores the bug, and
   `TestNoaliasIsAParametersNeverAResults` prints `smooth_into` itself and must see it. The baseline
   was re-accepted with the results clean before anything was committed.
1. **The kept function's name** is `@"shift~lent"`, quoted, so a search for `@shift` missed it.
2. **Two calls of `shift` do not pay** (P8's |f| · max(π − 1, 1) ≥ 64), and it was inlined. The
   witness calls it five times.
3. **A timing loop over `shift` meets the 10-24 plan's item 9**: an `again` after a nested loop's
   join point is not built. The timing calls it eight times in sequence instead.

## Paid and bought

- **Compile time**: 1.00× the baseline in the first accepting run, and 1.01× in the corrected one,
  whose five suspects (the same five every run this round has kept, `freq` and `big-negmid` among
  them, each cleared at 1.00× when paired) could not be paired serially. The first run's
  `compiletime.txt` was uncommitted, so the gate re-recorded them.
- **Run time**: `shift` over 4 MiB, eight calls, against HEAD in a worktree, 9 interleaved
  rounds, medians (minima): 41.8 (40.9) ms before, 41.8 (40.6) with noalias, ratio 1.000. The
  executables are byte-for-byte the same size: inlined into `main`, the two regions are separate
  allocations LLVM already knows apart.
- **Emitted text**: 10 windows files change against the committed baseline, each by `noalias` on
  a lent buffer parameter alone, and no result anywhere carries it. One of the gauntlet's 33 files,
  `gen_smooth_native.ll`, changes. On it, g6 into (the variant writing a lent buffer) is 0.999 and
  g6 alloc 0.962, seven interleaved rounds, all 22 cases agreeing with Go. Go, JavaScript and the
  JVM are byte-identical.
- **Proofs and refusals**: unchanged.
- **Compiler lines**: `ir/llvm/llvm.go` +24 (in `tableAttr`, a parameter's only), `core/place.go`
  +6 (the plant); tests `cmd/build/noalias_test.go` 85 and `TestNoaliasIsAParametersNeverAResults`
  in `ir/llvm/tableattr_test.go`; llvm.md §8.
- **Rules made less general**: none.

## What was not built

- **noalias on a read-only table that is the program's only reference to its region**, a fact
  ownership (ADR 0096) has. It needs that fact at the boundary.
- **A program where it pays**: none measured. A kept function reading one table and writing a
  lent buffer, called from many places so it is not inlined, is where LLVM's alias analysis would
  need the attribute. The corpus's kept functions so far are the os cell's and K(ℕ\*)'s.
