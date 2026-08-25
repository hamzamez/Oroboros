# Java on the native target — 2026-08-25

Java was the host that had not moved. Every Java number in this repository came from the retired
portable layer, and Java was the one most likely to disagree with Go and JavaScript.

Four gauntlet programs are now written against `targets/java/` and against the language's own table
— `(table n f)`, `len`, indexing by application — and measured against hand-written Java on
**JDK 17.0.12**.

Two findings, and the second is a refutation of a measurement this repository cites in nine places.

---

## 1. Everything is at parity, and the one gap is `int` → `long`

| | hand (`int`) | hand (`long`) | generated | vs hand-`long` | vs hand-`int` |
|---|---|---|---|---|---|
| dot, n=65536 | 31,244 | 33,067 | 32,963 | **1.00×** | 1.06× |
| dot, n=1024 | 454.6 | 518.6 | 513.9 | **0.99×** | 1.13× |
| centroid | 31,528 | 32,622 | 32,658 | **1.00×** | 1.04× |
| search, early exit | 3.1 | 4.1 | 4.0 | **0.98×** | 1.29× |
| search, late exit | 19,168 | 27,790 | 29,559 | **1.06×** | 1.54× |

**The emitted code matches hand-written Java in every case — once the counter is a `long`.** The
entire residual gap is that our `int` maps to Java's `long`, so an emitted loop counter is 64-bit
and every array access carries an `(int)` cast:

```java
long i = 0;
…
acc = (acc + (a[(int) i] * b[(int) i]));
```

That costs **1.04× to 1.45×** depending on how much else the loop does — nothing on a
memory-bound centroid, 1.45× on a tight scan. It is
[ADR 0012](../../docs/decisions/0012-portable-integer-range.md)'s portable window meeting the one
host where `int` is not the natural index width, and it is the first time that decision has had a
price attached on Java.

> **FIXED the same day** — [indextype-2026-08-25](indextype-2026-08-25.md). A loop variable the
> target can prove small enough is emitted as the host's own `int`, and every row above is now at
> parity with idiomatic hand-written Java: `search late` went from **1.54× to 1.01×**.
>
> Not via `emit/interval.go` in the end. That pass opens lambdas with different names than the
> emitter does, so keying its results back would have been fragile; the emitter already had the
> analysis it needed in `countedGuard`, built for bounds-check elimination. The justification is a
> **platform** fact rather than an inferred range: a Java array holds at most 2³¹−1 elements, so a
> length *is* an int by the host's own rule.

### What is NOT the cost

**The result-variable shape is free on Java.** `native-js-2026-08-20` found that a loop in tail
position emitting a result variable plus `break`, rather than an early `return`, costs **1.31× on
V8**. Measured here, hand-written, with the counter type held constant: 27,574 against 27,365 —
**nothing**. The JVM flattens it. A finding on one host is not a finding on another, which is the
whole reason each host gets its own measurement.

---

## 2. Baseline R5 does not reproduce, and the sign is reversed

`targets/java/util.oro` declared only the **unfused** `getOrDefault`+`put` idiom, and cited
[baseline-2026-08-13 §R5](baseline-2026-08-13.md):

| R5, 2026-08-13 | ns/op |
|---|---|
| `wordCountMerge` — `merge(w, 1, Integer::sum)` | 9,259,530 |
| `wordCountGetOr` — `put(w, getOrDefault(w,0)+1)` | 3,577,103 |

> "On Java the **unfused** form wins by 2.6×."

**Re-run today, using R5's own functions from `Gauntlet.java`, unchanged:**

| | ns/op | |
|---|---|---|
| `wordCountGetOr` | 3,977,398 | |
| `wordCountMerge` | 3,700,589 | **0.93×** |

The unfused number reproduces (3.58M then, 3.98M now). **The fused number is 2.5× better than
recorded**, and the fused form is now the faster of the two.

With `split` hoisted out of the timed region, so the map work is not diluted, and **one JVM per
form** because JIT state carries across benchmarks in a process the way V8's does:

| | run 1 | run 2 | run 3 |
|---|---|---|---|
| unfused | 1,897,037 | 1,827,331 | 1,861,427 |
| **fused `merge`** | **1,558,888** | **1,536,354** | **1,549,205** |

**0.84× — the fused form is 1.19× faster**, consistently, in separate processes, outside the ~15%
noise floor.

### What this does and does not change

**ADR 0008 is reinforced, not damaged.** Its rule is *"which granularity wins is a per-target
measurement, not a principle"* — and a measurement moving under a five-month-old conclusion is that
rule working, not failing. What is wrong is the **example**, and the example is quoted in nine
places.

**Both forms are now declared** and the program picks, which is what the gauntlet's "carry both
forms" rule asks for and what R5's conclusion prevented:

| | ns/op | |
|---|---|---|
| hand-written, unfused | 3,959,172 | |
| hand-written, fused `merge` | 3,730,637 | |
| generated, unfused | 4,398,860 | 1.11× |
| **generated, fused** | **3,722,239** | **1.00×** |

The generated fused form is at parity; the generated unfused form pays the `long` counter of §1
plus a `let` binding for the word.

**Why the original was wrong is not established.** The candidates are the JVM version (unknown for
R5, 17.0.12 here), warm-up — a `merge` call site goes through `invokedynamic` and a method
reference, which need more warm-up than a plain `put` before C2 inlines them — and the harness. The
honest statement is that the number does not reproduce, not that a particular thing caused it.

---

## 3. The remaining three: generic, report, stencil

All three are at parity, and none needed a new idea.

### generic — one definition, two element types

`reduce-over` is instantiated at `double` with `java.f+` and at `Map<String,Long>` with a map
update, from one definition, with no type annotations, no monomorphization pass, no type parameters
and no dictionary. The backend works out `double` for one and `java.util.Map<String,Long>` for the
other on its own.

| | ns/op |
|---|---|
| `sum-of` hand-written | 30,699 |
| **`sum-of` generated** | **30,749 — 1.00×** |

No `long`-counter penalty here, and that is §1's finding behaving the way
[checkcost-2026-08-19](checkcost-2026-08-19.md) said a cost behaves: this loop is memory-bound over
65,536 doubles, so the cast hides behind the cache miss. The same reason centroid pays 1.04× and a
tight scan pays 1.45×.

### stencil — ADR 0013's program, on a third host

Three forms carried, because the difference between them *is* the question:

| | ns/op | |
|---|---|---|
| allocating, hand-written | 94,096 | |
| **allocating, generated `(alloc (table …))`** | **92,762** | **0.99×** |
| **allocating, generated `build`/`set`** | **93,688** | **1.00×** |
| reusing, hand-written | 61,011 | |
| **reusing, generated `java.set-double`** | **60,945** | **1.00×** |

**And allocating costs 1.54× for HAND-WRITTEN Java too** (94,096 against 61,011). That is
[ADR 0013](../../docs/decisions/0013-accept-the-allocation-price.md)'s price confirmed on a third
host, and confirmed as *the shape rather than the compiler* — which is what
[native-gauntlet-2026-08-20](native-gauntlet-2026-08-20.md) established on Go and what the ADR's
correction says.

The gather and the scatter measure the same here, which is worth noting: `(alloc (table …))` is
pure and parallel by construction and `build`/`set` is sequential, and on this program the JVM does
not care.

The reusing form is deliberately **target-native**. ADR 0018 scopes a buffer to `build`, so writing
into an array the *caller* owns is `java.set-double` — Java's own store, carrying no portability
claim, exactly as `go.set-float64` does on Go.

### report — the program whose pass condition is not a number

Built and run. All three native targets agree on the value and differ only in how the host prints a
float, which is target-native and carries no portability claim:

| | output |
|---|---|
| Go | `report` / `1000` / `3.328335e+08` |
| JavaScript | `report` / `1000` / `332833500` |
| Java | `report` / `1000` / `3.328335E8` |

`Σ i²` for i < 1000 is 332,833,500.

---

## 3b. One compiler gap the stencil found

`(alloc (table n (fn (j) …)))` was **refused**: the rule indexes the array it is built from, and
nothing told the refinement layer that `j` is in `[0, n)`.

```
(a j) is an indexing, and (<= 0 j) does not follow
  known: assumed -len(a) + +3 <= 0
```

**A rule's parameter IS its domain.** `(table n (fn (j) …))` says element `j` is a function of `j`
for `j` in `[0, n)`, so the body may assume exactly that. It is
[tables.md §6](../../docs/spec/tables.md) once more — *bounds are the domain* — and it is the third
time the same sentence has been the fix: the read side needed it for indexing, `build` needed it
from the other side as `len(b) = n`, and a rule needs it for its own parameter.

Without it, no stencil could be written as a rule at all, on any target. It was invisible until a
program indexed an array *inside* a `table`.

---

## 3c. What is covered, and what is not

Built and measured on the native Java target: **dot, search, centroid, wordcount, generic, report,
stencil** — all seven. Written on the language's table, so this migration lands on the current
architecture rather than on the interim surface. `sieve-java.oro` and `divmod-java.oro` predate
this and already work.

One thing the migration deleted on the way: `targets/java/lang.oro`'s `split` returned
`string-array`, one of the enumerated legacy type names `(array V)` exists to replace. Prim
declarations accept a compound result type now, so it returns `(array string)` — and the first
native Java program to index a split result stopped emitting `final /*unknown*/ w = …`.

---

## 4. Method

Correctness first: every generated function is checked against its hand-written reference before
any timing, because a benchmark of a wrong program is not a result.

The harness warms up by **time**, not by a fixed iteration count. A fixed 50,000 iterations suits a
450 ns `dot` and takes three minutes for a 3.4 ms wordcount — which is how the first version of
`NativeBench` ran for seventeen minutes before being killed.

The §2 comparison uses **one JVM per form**. That is the JS lesson applied to a different runtime,
and it is why the numbers there are lower and tighter than the in-process ones.
