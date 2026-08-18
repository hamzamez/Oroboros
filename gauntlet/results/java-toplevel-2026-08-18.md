# Java's top level: no collections at all, and the loop finding holds at three hosts

> **⚠ SUPERSEDED HEADLINE — see [loop-encoding-2026-08-18](loop-encoding-2026-08-18.md).**
> The measurement below is accurate; the conclusion drawn from it is not. `fold-range`'s bound is an
> arbitrary expression, so a loop with a start and a step needs only its **trip count**, which is
> arithmetic. Written that way the same sieve runs **at parity with idiomatic hand-written code**,
> with no new primitive. What the numbers below measure is the cost of the *naive encoding*, not of
> the loop primitive.


**Result: `java.lang` has no map, no list, and nothing growable except `StringBuilder`** — every
collection is in `java.util` and needs an import, so Java's top level is the poorest of the three by
a wide margin. The sieve replicates a third time: **1083×** for the loop shapes, with our emitter at
**0.44×** of hand-written code written under the same constraints — the best of the three hosts.

Completes [go-toplevel](go-toplevel-2026-08-18.md) and [js-toplevel](js-toplevel-2026-08-18.md).

---

## 1. What Java's top level is

Exactly `java.lang`, the one package auto-imported into every compilation unit. Declared as eight
modules: `java/math`, `java/int`, `java/string`, `java/char`, `java/strbuf`, `java/array`,
`java/system`, `java/bool`.

Everything landed — `Math.*`, `String`'s methods, `Character`'s predicates, `Long`'s statics,
`StringBuilder`, `System.out`, and raw arrays.

Note what that list does **not** contain. Before this experiment, the whole Java target file had
exactly one `(import …)` in it, for `java.util.HashMap` behind the `dict` primitive. That single
line was already the evidence: **the dictionary the gauntlet depends on is not part of Java's top
level**, and neither is anything else with a variable size.

| | Go | JavaScript | **Java** |
|---|---|---|---|
| growable sequence | `append` | `Array.push` | **none** — `StringBuilder`, text only |
| dictionary | `map[K]V` | object / `Map` | **none** |
| fixed array | `[n]T` | `Array`, typed arrays | `T[]` |

Arrays are *language*-level in Java rather than library, so they need no import — which is the same
statement seen from the other side: they are there precisely because they are fixed-length.

## 2. The sieve, at three hosts

[experiments/java-toplevel/sieve.oro](../../experiments/java-toplevel/sieve.oro), n = 20000, all
returning 2262:

| | µs | vs idiomatic |
|---|---|---|
| **hand-written Java** — `for (long j = i*i; j < n; j += i)`, with `continue` | **20.2** | 1× |
| hand-written Java **restricted to our loop shapes** | 21,864 | **1083×** |
| **generated** | 9,573 | 474× |

And the set:

| host | loop shape costs | emitter | overall |
|---|---|---|---|
| Go | **1117×** | 0.77× | 860× |
| JavaScript | **445×** | 0.56× | 249× |
| **Java** | **1083×** | **0.44×** | 474× |

**Three hosts, three compilers, one cause.** The loop's missing start, step and early exit turn
O(n log log n) into O(n²) on every one of them. The ratio varies with how fast the host's idiomatic
form is, not with anything about the language.

**And on all three, our emitted code is *faster* than a human writing under the same constraints** —
0.77×, 0.56×, 0.44×. The margin grows as the host's own optimiser does less with the naive shape.
Whatever is wrong here, it is not the backend, and that is now established three times rather than
inferred once.

## 3. What no dictionary costs

[tally.oro](../../experiments/java-toplevel/tally.oro) counts distinct values with two fixed arrays
and a linear probe, which is what `java.lang` forces. n = 20000, capacity 128, 97 distinct keys:

| | µs | |
|---|---|---|
| `HashMap.merge` — **needs `import java.util`** | 144.6 | the idiomatic answer |
| linear scan **with `break`** — `java.lang` only | 283.7 | **2×** — the cost of no dictionary |
| **generated** — no `break` available | 601.6 | **a further 2×** — the cost of no early exit |

**4× overall**, cleanly decomposed into "no dictionary" and "no early exit".

This is a much *milder* number than the sieve, and the reason is worth stating rather than hiding:
the table is nearly full — 97 keys in 128 slots — so a `break` saves little on average, and a
128-element scan is cache-resident. On a large sparse table both penalties would grow with capacity.
**The 2× is a floor, not a typical case.**

It is also the third independent appearance of early exit as the missing feature, after the sieve on
Go and on JS.

## 4. Where Java is worse than JavaScript for us, and why

| | Go | JavaScript | Java |
|---|---|---|---|
| types needed | one per element type | **none** | one per element type |
| `if` as an expression | `var t; if … else …` | `(c ? a : b)` | `T t; if … else …` |
| top-level collections | map + slice | array + object | **none** |
| higher-order host API | little | most of it, **and not worth having** | streams, needs `java.util.stream` |

Java repeats Go's two walls — a type per element type, because the type language has no
constructors; and statement-only `if`, so a conditional inside a loop is three times the size a
human would write — and adds one of its own.

There is a small extra cost: `int` versus `long`. Our `int` is Java's `long`
([arithmetic.md §4](../../docs/spec/arithmetic.md): Java's `int` wraps at 2³¹, *inside* the portable
range), but array indices and `String.charAt` take `int`. So every array access emits a cast:
`%s[(int) %s]`. Correct, free at runtime, and noise in the output.

## 5. Method

Best of five over enough repetitions to clear the timer, after 30–50 warm-up iterations so the JIT
has compiled. No dispatch inside the timed loop — the hazard
[loops.md §4](../../docs/spec/loops.md) records, which cost three wrong measurements before it was
noticed. The usual laptop, ~15% noise floor, irrelevant at these ratios.
