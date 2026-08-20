# Is static data free? 2026-08-20

[data-structures.md §8.4](../../docs/data-structures.md) proposed that a literal table
`(array c₀ … cₙ₋₁)` whose elements are all constants lands in **static data** — free at run time
where `materialize` costs an allocation and n stores. §8.5 marked that claim as one *yes*, one
*probably* and two *no*s, **all four guesses**, and named it the load-bearing claim of the whole
proposal.

It is measured now, and it does not hold.

## Method

The same table two ways on each host: written as a **literal**, and **computed** by a loop at
startup. Hand-written artifacts — no compiler work, because the point was to decide whether to do
the compiler work. 65,536 `int32` entries of `(i * 7919) % 65521`, plus a small-table control at
256 and an expensive-element control described below.

## Results

| target | static form | code at startup? | artifact | verdict |
|---|---|---|---|---|
| **x86-64** | `.rodata`, and the target format already has `(data …)` | **none** — the section is mapped from the image | + table size | **free of code** |
| **Go** | package-level `[65536]int32{…}` | **none** — `go tool nm` shows **no `main.init`** | **+261,632 B** for 262,144 B of table | free of code, pays in size |
| **Java** | `static final int[] TABLE = {…}` | **256 `iastore` instructions in `<clinit>`** | + class-file size | **pure loss** |
| **JavaScript** | module-level `const T = [ … ]` | the array is built at module evaluation | **382,124 B vs 144 B** | **pure loss** |

**Go, the exact numbers.** `go tool nm` on the two binaries:

```
static:    D main.table                      (no main.init at all)
computed:  T main.init      D main.table
```

The static build has the table in the data section and generates no initialiser. The computed
build allocates the same space and emits code to fill it. Binary sizes are 2,476,544 against
2,214,912 — a difference of 261,632 bytes for a 262,144-byte table, so the table costs **exactly
itself** and nothing else.

But the startup saving is not there: 30 runs each, **9.75 ms median against 9.93 ms**. Process
creation on Windows is ~9 ms and swamps a 65,536-iteration fill loop entirely.

**JavaScript, the exact numbers.** Timing `await import()`, three runs each:

| | import | source |
|---|---|---|
| `const T = [ …65536 numbers… ]` | 4.77 / 4.15 / 4.28 ms | 382,124 B |
| `new Int32Array([ … ])` | 4.61 / 4.28 / 4.33 ms | 382,140 B |
| built by a loop | **1.28 / 1.18 / 1.20 ms** | **144 B** |

**The literal is 3.5× slower to load and 2,600× larger in source.**

**Java, the exact evidence.** `javap -c -p` on the class:

```
static {};
  Code:
     0: sipush        256
     3: newarray      int
     5: dup
     6: iconst_0
     7: iconst_0
     8: iastore
     9: dup
     ...
```

256 elements, 256 `iastore`. The JLS specifies array initialisers as executed statements, so a
`static final int[]` is built element by element at class load. There is no constant-pool form for
an array.

## The control that mattered: an expensive element

The first measurement used a multiply and a modulo, which is nearly free, so a fair objection is
that the literal wins only when the element is *expensive* to compute. Tested: 4,096 values of
`sqrt(i) · log(i+1)` on JavaScript.

| | import | source |
|---|---|---|
| literal | 1.41 / 1.83 / 1.08 ms | 75,367 B |
| computed | 1.50 / 0.99 / 1.55 ms | **145 B** |

Indistinguishable, at 520× the source size. Four thousand transcendental calls is about 50 µs,
still far under the module overhead, and the literal's parse cost grows with the table at least as
fast as the compute cost it saves.

> **A lookup table is worth having when the element is expensive relative to a memory load. A
> literal table is worth *emitting* only when the element is expensive relative to a parse. The
> second bar is far higher, and on a source-code host the literal must be parsed every time the
> program starts.**

## What this decides

**The `unroll` and `freeze` edges of §8.4's memory algebra are refuted, and should not be built.**
Turning `(vec n f)` with a literal `n` into `(array c₀ … cₙ₋₁)` is free of code on two targets, a
pure loss on the other two, and **never a measurable win** on any of them. It also would have
needed a binary-size budget — a heuristic this project has avoided everywhere else — and the
measurement removes the need for one.

**What survives is the compile-time half.** β-tab — `((array 1 2 3) 1) → 2`, `(+ 1 (a 1)) → 3` —
is about *reduction*, not memory, and nothing here touches it. So does the observation that a
statically-indexed heterogeneous table is a tuple.

**And that collapses the ranking.** The literal table's distinguishing benefit was the memory
edge. Without it, what remains overlaps almost entirely with the negative product (D-B): both are
constructs that reduce away and cost nothing when they do, and D-B additionally covers the
*returned* case, which is where all six recorded demands actually are. So:

> **D-B — multiple return values — is the next build. D-K reduces to a syntax question and can
> wait.**

## The method note worth keeping

This is the fourth time in this project that a plausible reading of how a host is documented to
work was wrong, and the second time in two days that the wrong direction was *in our favour* — the
JavaScript module-namespace penalty inflated hand-written references
([native-js-2026-08-20 §1](native-js-2026-08-20.md)), and here a proposal's headline benefit did
not exist.

The cost of checking was about an hour of hand-written artifacts and no compiler work at all,
against a design that would have added a reduction rule, a size heuristic, and a memory-region
model to the language. **The measurement that kills a candidate is cheapest before the candidate
is built**, which is [ADR 0007](../../docs/decisions/0007-exploration-over-specification.md)'s
whole content, and it is worth noticing that the argument in §8 was *good* — it was internally
consistent, it had the literature behind it, and it was wrong about the only thing that mattered.
