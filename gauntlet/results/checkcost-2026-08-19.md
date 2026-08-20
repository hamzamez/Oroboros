# What an unproven operation costs in a real program, 2026-08-19

[overflow-2026-08-19](overflow-2026-08-19.md) and [product-2026-08-19](product-2026-08-19.md)
priced the overflow check **in isolation** — a tight loop doing nothing but the operation. That is
the number a microbenchmark gives, and [selection-2026-08-19](selection-2026-08-19.md) ended by
saying it is not the number a program pays. This is the number a program pays.

## Method

The **same source, compiled twice**, differing only in whether the signature declares a range. So
the checked and unchecked forms are the compiler's own output rather than hand-written
approximations of it, and the two builds do identical work on identical values.

```lisp
(sig sum-range ((n int)) int (where (and (<= 0 n) (< n 65536))))   ; 0 operations checked
(sig sum-range ((n int)) int)                                      ; 2 of 2 checked
```

Two shapes, deliberately at opposite ends:

- **arithmetic-bound** — `sum-range`: two adds and a compare per iteration, and in the undeclared
  build *both* adds are checked.
- **memory-bound** — the sieve: the inner loop is a byte store, and the checked operations are the
  index arithmetic around it.

On windows the comparison is the whole 100-round benchmark, with the bound arriving through
`GetTickCount64 & 0` — zero at run time, opaque to the analysis. **Identical work, identical
numbers printed, differing only in what the compiler could prove.** Both print `1798400`.

## Results

| shape | host | operations checked | plain | checked | |
|---|---|---|---|---|---|
| arithmetic-bound | **Go** | 2 of 2 | 19,430 ns | 88,194 ns | **4.54×** |
| arithmetic-bound | **Java** | 2 of 2 | 13,441 ns | 20,497 ns | **1.52×** |
| memory-bound | Go | 7 sites | 1,084,908 ns | 1,331,681 ns | **1.23×** |
| memory-bound | windows | 16 of 18 | 102 ms | 149 ms | **1.46×** |

For comparison, the isolated numbers: **1.65×–1.81×** for a windowed add, **2.61×–3.74×** for a
fixnum-style add, **1.87×–7.40×** for a multiply.

## Three findings

### 1. The isolated number is wrong in BOTH directions

On Go it understates the arithmetic-bound case — **4.54× against an isolated 2.61×** — because the
real loop checks *two* additions where the microbenchmark checked one, and because two dependent
branches in a loop whose plain form runs at about one cycle per iteration destroy the instruction
level parallelism the plain form had.

And it overstates the memory-bound case — **1.23× against an isolated 1.65×** — because the branch
hides behind the cache miss.

That is the same shape as [bce-2026-08-15](bce-2026-08-15.md), where a 1.96× *win* in isolation
disappeared on memory-bound loops. **A cost behaves the same way a saving does**, and neither can be
quoted without the condition attached.

### 2. The spread within one design is wider than the spread across hosts

1.23× to 4.54×, on one host, for the same feature. Which *shape* your loop has matters more than
which host you are on.

This is the practical form of a thing this project keeps rediscovering: a single number for the
cost of a language feature is almost always a number for the cost of one benchmark.

### 3. The host's intrinsic is worth 3×, and that is the §0 rule biting

Same source, same annotation, same loop:

> **Go 4.54×. Java 1.52×.**

The JVM has `Math.addExact` as an intrinsic that compiles to an add and a jump-on-overflow. Go has
no equivalent, so the emitted check is a comparison pair in the dependency chain. The gap is not
tuning — it is a capability the two hosts do or do not have, and it was **predicted** by
[overflow-2026-08-19](overflow-2026-08-19.md)'s 1.31× against 2.61×.

[data-model.md §0](../../docs/spec/data-model.md) says that if a portable name's price differs
across targets by more than the noise floor, **the spread is part of the name's meaning**. Here it
is a factor of three, and it is now measured rather than asserted.

## What this means for the design

**Declaring a range is worth the most exactly where you would want it to be** — in tight arithmetic
loops, where it is a factor of 4.5 on Go — and worth least where nothing was going to be fast
anyway.

**And the cost of *not* declaring is bearable in the shapes most programs are.** 1.23× on a
memory-bound loop is a price a correct-by-default integer could plausibly ask. 4.54× on a tight
accumulator is not, which is why the annotation exists.

Neither of those was knowable from the microbenchmark.

## What is still not measured

- **The bignum path.** Everything here traps; nothing promotes. `math/big` was 38.9× in isolation
  and has not been measured in a program, because there is no bignum representation to measure.
- **JavaScript**, which declares no checked form at all, so there is nothing to compare.
- **Whether a better emitted check closes Go's gap.** The func literal *is* inlined —
  `-gcflags=-m` confirms it — so 4.54× is real for this formulation, but it is not proof that no
  formulation does better.
