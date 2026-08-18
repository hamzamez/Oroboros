# Go's top level, and the 1117× the loop primitive costs

> **⚠ SUPERSEDED HEADLINE — see [loop-encoding-2026-08-18](loop-encoding-2026-08-18.md).**
> The measurement below is accurate; the conclusion drawn from it is not. `fold-range`'s bound is an
> arbitrary expression, so a loop with a start and a step needs only its **trip count**, which is
> arithmetic. Written that way the same sieve runs **at parity with idiomatic hand-written code**,
> with no new primitive. What the numbers below measure is the cost of the *naive encoding*, not of
> the loop primitive.


**Result: the emitter is at parity and the *loop* is three orders of magnitude off.** A Sieve of
Eratosthenes written against Go's predeclared identifiers produces a correct answer 860× slower than
hand-written Go — and hand-written Go **restricted to the loop shapes Oroboros can express** is
1117× slower than idiomatic Go. Our generated code is 0.77× of that restricted form, i.e. slightly
*faster* than a human writing under the same constraints.

Nothing in the gap is the emitter's. All of it is `fold-range`.

---

## 1. What the experiment was

Declare a `go/builtin` module — Go's **predeclared identifiers**, the `builtin` pseudo-package,
which is exactly "everything usable with no import" — plus `go/fmt` for output. Then write
non-trivial, deliberately **non-portable** programs against it.

This is [ADR 0001](../../docs/decisions/0001-parasite-model.md) taken at its word: portability is a
property a program may or may not have, and these programs deliberately do not have it. A program
that says `(use go/builtin)` will not build on JS or Java, and the covering check says so.

What landed, all emitting idiomatic Go and running correctly:

| | |
|---|---|
| `len`, `cap`, `min`, `max`, `panic`, `println` | one declaration each |
| `true`, `false` | the core has no boolean literal, so the target supplies Go's |
| integer `/` and `%` | **not** in `num/int` — the hosts disagree, so this is Go's operator with Go's semantics and no portability claim |
| `make([]int64, n)`, `make([]bool, n)`, `make(map[string]int64)` | one per element type |
| indexed read, and **indexed write** | `iset` is a `stmt` whose value is its container |
| `append` | declared **impure**, correctly — it may write into the argument's backing array |
| map read, write, delete | reading a missing key yields Go's zero value |
| slicing `s[lo:hi]` | |

Mutation is the surprise. A statement primitive's value is its first argument, so
`(g.iset s i v)` yields `s` back and threads through a fold exactly as an accumulator does, and the
effect discipline pins the order for free. This emits:

```go
var v1 []int64 = (make([]int64, n))
v1[0] = 42
v1[1] = 7
return (v1[0])
```

which is what a Go programmer writes. **Imperative array programming needed no new mechanism.**

## 2. The sieve

[experiments/go-toplevel/sieve.oro](../../experiments/go-toplevel/sieve.oro), n = 20000, best of five,
each timed over enough repetitions to clear the timer:

| | µs | vs idiomatic |
|---|---|---|
| **hand-written Go** — `for j := i*i; j < n; j += i`, with `continue` | **17.4** | 1× |
| hand-written Go **restricted to our loop shapes** | 19,437 | **1117×** |
| **generated** | 14,974 | 860× |

All three return 2262, the correct count of primes below 20000.

The middle row is the whole finding. It is hand-written Go, by a human, with every loop starting at
0, stepping by 1, running to its bound, and unable to exit early — the shapes `fold-range` can
express. It is 1117× slower than the same algorithm written normally.

**Our generated code is 0.77× of it.** The emitter is not merely at parity with what a human can
write under these constraints; it is slightly ahead, because the residual hoists work the shaped
version repeats.

## 3. What is missing, precisely

The sieve's inner loop is `for j := i*i; j < n; j += i`. Three things in that line are
inexpressible:

1. **A start.** `fold-range` always begins at 0. We iterate from 0 and test `j >= i*i`.
2. **A step.** `j += i` cannot be said. We visit every `j` and test `j % i == 0`.
3. **Early exit.** `if composite[i] { continue }` becomes a conditional over the entire range, and
   the outer loop cannot stop at `i*i >= n`.

So the algorithm degrades from O(n log log n) to O(n²) — and it is the *source* that cannot say the
fast thing, not the emitter that fails to produce it.

A second program, [histogram.oro](../../experiments/go-toplevel/histogram.oro), shows the same hole
from the other side. Collatz iteration has no bound known in advance, so it must be written as a
**fixed budget with an idempotent tail**: run 200 steps, and once the value reaches 1 keep returning
it. Correct, wasteful, and awkward — the shape every convergence loop is forced into.

## 4. What this changes

[docs/spec/loops.md](../../docs/spec/loops.md) ranked its candidates before this measurement
existed, and this reorders them.

**The iteration space — start, step, and early exit — is worth 1117× on a program anyone would
write.** For comparison, everything else this project has weighed:

| | |
|---|---|
| loop start/step/exit, this measurement | **1117×** |
| duplicating an allocating expression ([wordcount](wordcount-2026-08-14.md)) | 615× |
| product accumulator, JS ([loops.md §4](../../docs/spec/loops.md)) | 13.8× |
| product accumulator, JVM | 6.4× |
| bounds-check elimination ([bce](bce-2026-08-15.md)) | 1.96× |
| the accepted allocation price ([ADR 0013](../../docs/decisions/0013-accept-the-allocation-price.md)) | 1.79× |

It is the largest single number in the repository, and it is not close.

**Replicated on both other hosts.** [js-toplevel](js-toplevel-2026-08-18.md) measures 445× with the
emitter at 0.56×; [java-toplevel](java-toplevel-2026-08-18.md) measures 1083× at 0.44×. Three hosts,
three compilers, one cause — so this is a property of the language rather than of Go.

**It does not decide the design**, and should not be read as doing so. `fold-range` with a start and
a step is one answer; a general `while` is another; a `loop`/`break` expression is a third. What the
measurement establishes is the *size of the prize*, and that the prize is in the loop's iteration
space rather than in its accumulator.

## 5. Two bugs, both found by the first program

Recorded here because the experiment is what found them, and both are committed.

**Nested binders shadowed each other, silently.** Two folds whose step functions used the obvious
names — `acc`, `i` — emitted `for i := …` inside `for i := …` and `acc := acc`. The outer
accumulator was never written and the function returned its initial value. `f(3)` gave 0 where the
answer is 12. The reducer was right throughout; parameter names are *hints*, and the emitters had
assumed they were unique. Fixed by opening abstractions through `OpenWith` against a scope of
already-emitted names.

**An integer accumulator did not compile.** `acc := 0` is Go's `int`, and everything else emitted is
`int64`. No gauntlet program folds over an integer, so it had never fired.

Every generated file in the gauntlet is byte-identical after both fixes, on all three targets.

## 6. Method

Timings on the usual hybrid P/E-core laptop with a ~15% noise floor — irrelevant at these ratios,
but the sub-2× comparison in row 3 sits inside it and should be read as "the same".

The comparison that matters is row 2 against row 1: both are hand-written Go, compiled by the same
compiler, differing only in the loop shapes allowed. That isolates the language from the emitter,
which is the only way this number means anything.
