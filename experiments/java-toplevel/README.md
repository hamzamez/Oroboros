# Java's top level

An experiment. **The measurement is
[gauntlet/results/java-toplevel-2026-08-18.md](../../gauntlet/results/java-toplevel-2026-08-18.md)**.
Third of three, after [go-toplevel](../go-toplevel/) and [js-toplevel](../js-toplevel/).

## The question

Java's top level is exactly **`java.lang`** — the one package auto-imported into every compilation
unit. Eight modules in `targets/java.oro`:

```lisp
(use java/math as m)     ; Math.*
(use java/int as ji)     ; Long statics, integer / and %
(use java/string as s)   ; java.lang.String
(use java/char as ch)    ; Character predicates
(use java/strbuf as b)   ; StringBuilder — the only growable thing here
(use java/array as a)    ; arrays, which are language-level
(use java/system as sys) ; System.out, System.nanoTime
(use java/bool as bo)    ; true / false
```

## Programs

| | what it exercises |
|---|---|
| [sieve.oro](sieve.oro) | `boolean[]`, indexed write, nested loops, integer `%` |
| [tally.oro](tally.oro) | counting distinct values **with no dictionary** |

`*.txt` files are benchmark harnesses, kept as text so they are not compiled.

```bash
go run ./cmd/gen experiments/java-toplevel/sieve.oro java /tmp/JSieve.java
```

## The finding: `java.lang` has no collections

No map. No list. Nothing growable except `StringBuilder`, and that only holds text. Every collection
is in `java.util`.

The evidence was already in the repository and nobody had read it: before this experiment, the whole
Java target file contained **exactly one `(import …)`** — `java.util.HashMap`, behind the `dict`
primitive the gauntlet's word-count depends on.

| | Go | JavaScript | Java |
|---|---|---|---|
| growable sequence | `append` | `Array.push` | **none** |
| dictionary | `map[K]V` | object / `Map` | **none** |
| fixed array | `[n]T` | `Array` | `T[]` |

Arrays are *language*-level here rather than library, so they need no import — the same fact from the
other side: they are there because they are fixed-length.

[tally.oro](tally.oro) is what that forces: two parallel fixed arrays and a linear probe. It costs
**2×** against `HashMap`, and a further **2×** because the scan cannot `break`. Both are floors
rather than typical cases — the table is nearly full, so a `break` saves little.

## The walls

### 1. The loop's iteration space — **1083×**

Third replication. Go 1117×, JavaScript 445×, Java 1083×. Same missing start, step and early exit;
same collapse to O(n²).

On all three hosts our emitted code is **faster** than hand-written code written under the same loop
constraints — 0.77×, 0.56×, **0.44×** here, the best of the three.

### 2. Java repeats Go's walls, not JavaScript's

**A type per element type**, because the type language has no constructors: `long[]` and `boolean[]`
are unrelated atoms, each needing its own `make`, read and write. JavaScript needed none of this.

**Statement-only `if`**, so a conditional inside a loop becomes `T t; if (c) { t = … } else { t = … }`
where JavaScript emits a ternary. The same residual, three quite different-looking outputs.

### 3. `int` versus `long`

Our `int` is Java's `long`, because Java's `int` wraps at 2³¹ — *inside* the portable range, so
declaring it would be a silent miscompilation
([arithmetic.md §4](../../docs/spec/arithmetic.md)). But array indices, `charAt` and `substring` all
take `int`, so every one emits a cast: `%s[(int) %s]`. Correct and free at runtime; noise in the
output.

### 4. Not attempted

`java.util` in any form, generics, classes and interfaces, lambdas and method references, streams,
`try`/`catch` and checked exceptions, `Optional`, records, `Thread`, autoboxing beyond what `dict`
already does. Streams are the closest analogue to JavaScript's higher-order API and would be the
obvious next measurement — but they need `java.util.stream`, so they are not top level at all.

## What the three experiments cost, and returned

Go's found two emitter bugs, one of them a silent wrong answer. JavaScript's found a false claim in
a target file's documentation. Java's found no bugs — which is its own small result, since it was
written after the other two had already forced the fixes.

Together they produced the largest number in the repository, three times over, and established that
it belongs to the language rather than to any host.
