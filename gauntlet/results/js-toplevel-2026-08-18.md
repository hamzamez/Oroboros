# JavaScript's top level: the loop finding replicates, and the idioms lose

**Result: three things.** The loop penalty is not a Go artifact — the same sieve is **445× slower**
on JS, and our emitted code is **0.56×** of hand-written code written under the same constraints,
i.e. nearly twice as fast as a human so restricted. **JavaScript's higher-order array API is 3.6× to
133× slower than a loop**, so the one thing this language cannot reach on JS is the one thing not
worth reaching. And the **type checker caught a real bug on the target that declares no types.**

Companion to [go-toplevel-2026-08-18](go-toplevel-2026-08-18.md). Same experiment, different host,
deliberately different walls.

---

## 1. What was declared

JavaScript's top level is not shaped like Go's. Go's is a list of predeclared **functions**; JS's is
mostly **methods on values** and **statics on namespace objects**. So the modules mirror how
JavaScript organises itself:

`js/global` (parseInt, Number, String, NaN, true/false), `js/math`, `js/int` (integer `/` and `%`),
`js/array`, `js/string`, `js/object`, `js/json`, `js/console`.

Everything reachable in **both Node and the browser** with no import and no require.

A template puts the receiver in the first hole, so `"%s.map(%s)"` needed **no new mechanism** —
method syntax was free. And unlike Go, **no types had to be added at all**: `targets/js.oro`
declares none, and the type-constructor wall that forced `make-int` and `make-bool` on Go simply
does not exist here.

## 2. The higher-order API is unreachable — and that costs nothing

`map`, `filter`, `reduce`, `some`, `findIndex`, `sort`, `forEach`, `flatMap` all take a **function**.
Every one of them is refused:

```
gen: a bare abstraction reached the emitter: (fn (x) (num/f64.mul x 2.0))
  This is an escaping closure. JS has first-class functions and could
  emit one directly, but g6's cost model has not been checked here.
```

So the idiomatic half of JavaScript's array API cannot be called. Before deciding that is a problem,
measure whether it is worth calling. n = 1024, one variant per process, one array kind, one
monomorphic call site:

| | loop | higher-order | penalty |
|---|---|---|---|
| sum, `Float64Array` | 494 | 8861 (`reduce`) | **17.9×** |
| map + sum, `Float64Array` | 3730 | 15726 | **4.2×** |
| filter + sum, `Float64Array` | 719 | 95888 | **133×** |
| sum, `Array` | 491 | 481 (`reduce`) | **0.98× — equal** |
| map + sum, `Array` | 1633 | 5890 | 3.6× |
| filter + sum, `Array` | 676 | 3396 | 5.0× |

**Refusing closures is not a limitation on this host; it is the correct answer, and it is now
measured rather than assumed.** Fusing a pipeline into one loop beats JavaScript's own idioms by
between 3.6× and 133×, worst on typed arrays — which are the *fast* representation, so the penalty
is largest exactly where a performance-minded programmer would be.

Two things in that table are worth keeping separately:

- **`filter().reduce()` on a typed array is 133×.** `filter` on a `Float64Array` allocates a new
  typed array; the loop allocates nothing. This is [ADR 0013](../../docs/decisions/0013-accept-the-allocation-price.md)'s
  allocation price seen from the other side.
- **`reduce` on a plain `Array` matches the loop exactly** (481 vs 491). V8 optimises it well there
  and not at all on typed arrays. Another entry for [ADR 0008](../../docs/decisions/0008-measurement-over-principle.md)'s
  list: the same method, the same engine, two representations, a 18× spread.

## 3. The sieve, replicated

[experiments/js-toplevel/sieve.oro](../../experiments/js-toplevel/sieve.oro), n = 20000, all three
returning 2262:

| | µs | vs idiomatic |
|---|---|---|
| **hand-written JS** — `for (let j = i*i; j < n; j += i)`, with `continue` | **49.9** | 1× |
| hand-written JS **restricted to our loop shapes** | 22,165 | **445×** |
| **generated** | 12,415 | 249× |

Compare Go — 17.4 → 19,437 → 14,974, a **1117×** loop penalty at 0.77× — and
[Java](java-toplevel-2026-08-18.md), **1083×** at **0.44×**.

Two conclusions:

**The loop penalty is a property of the language, not of a host.** Two hosts, two compilers, the
same missing start, step and early exit, the same collapse from O(n log log n) to O(n²). The exact
ratio differs — 1117× on Go, 445× on JS — because idiomatic JS is slower to begin with, not because
the language does better there.

**The emitter is ahead of a human on both hosts.** 0.77× on Go, **0.56× on JS**. Given the same loop
shapes, our generated code is faster than what a person writes, because the residual hoists work the
shaped version repeats. Whatever is wrong here, it is not the backend.

## 4. Where JS is *better* than Go for us

Worth recording, because the parasite thesis predicts hosts differ and the differences ran both
ways.

| | Go | JS |
|---|---|---|
| types needed | `[]int` and `[]bool` are unrelated atoms; a `make` per element type | **none at all** |
| `if` as an expression | `var t2 T; if c { t2 = … } else { t2 = … }` | `(c ? a : b)` — idiomatic |
| method syntax | n/a | free, the receiver is hole 1 |
| higher-order host API | little of it | most of it, and **not worth having** |

The `if` row is the visible one. Go's statement-only `if` makes every conditional inside a loop
three times the size a human would write; JavaScript's conditional expression maps one-to-one. The
same residual, the same emitter, two very different-looking outputs — which is the thesis.

## 5. The checker earned its keep on the untyped target

`targets/js.oro` declares **zero** `(type …)` forms, and its header said argument types "are never
consulted". That is now false, and the correction is worth more than the comment.

The first draft of [wordstats.oro](../../experiments/js-toplevel/wordstats.oro) started a
longest-word fold at `0` instead of `""`:

```
gen: gen-main: … in a loop body: in a condition: in argument 2 of num/int.gt:
     in argument 1 of js/string.len: best is int, but string is required here
```

A real bug, caught on the host that has no type layer to catch it, using types the *emitter* never
reads. That is [types.md §1](../../docs/spec/types.md)'s whole argument arriving in a new place: JS
would have run this and produced nonsense.

## 6. Method

Each JS variant ran in **its own process**, with one array kind and one monomorphic call site. This
matters: an earlier harness that dispatched several variants through one site and mixed `Array` with
`Float64Array` produced a confident 1.88× that was entirely V8's type feedback
([loops.md §4](../../docs/spec/loops.md)). Timings are best-of-five over enough repetitions to clear
the timer, on the usual laptop with a ~15% noise floor — irrelevant at these ratios except for the
0.98× row, which should be read as "the same".
