# Index-type selection — 2026-08-25

[native-java-2026-08-25](native-java-2026-08-25.md) found the one place this project missed its own
bar with a number attached: our `int` is 64-bit, a Java array index is not, so an emitted loop
counter was a `long` and every access carried an `(int)` cast — **1.04× to 1.54×** against
hand-written Java depending on the loop.

Fixed. **Every measured program is now at parity with idiomatic hand-written Java.**

---

## 1. The result

One JVM per benchmark, minimum of nine timed rounds after a two-second warm-up. The in-process
harness is not trustworthy on a loaded machine — a run taken while nineteen other benchmarks shared
the process put `centroid` hand-written at 88,041 ns against its own 30,076 in a quiet one.

| | hand (`int`) | hand (`long`) | generated | before | **after** |
|---|---|---|---|---|---|
| dot, n=65536 | 30,573 | 32,496 | 30,025 | 1.06× | **0.98×** |
| dot, n=1024 | 440.2 | — | 445.5 | 1.13× | **1.01×** |
| centroid | 30,076 | — | 30,190 | 1.04× | **1.00×** |
| search, late exit | 16,950 | 28,527 | 17,051 | **1.54×** | **1.01×** |
| stencil, allocating | 93,083 | — | 92,010 | 0.99× | **0.99×** |

The `hand (long)` column is kept because it is the evidence: the penalty is real — 28,527 against
16,950 on the same program — and we no longer pay it.

```java
// before                                   // after
long i = 0;                                 int i = 0;
…                                           …
acc = (acc + (a[(int) i] * b[(int) i]));    acc = (acc + (a[i] * b[i]));
```

---

## 2. The rule, and why each clause is there

A loop variable is emitted as the host's own `int` when **all four** hold:

1. a clause guard is `(>= v B)` or `(> v B)` with `B` not mentioning a loop variable — so `B` is
   the loop's upper bound;
2. `B` is a **length**, or a length minus a non-negative literal;
3. every `again` steps `v` by exactly **+1**;
4. `v` starts at a non-negative integer literal.

(2) is where the platform's own guarantee enters: a Java array holds at most 2³¹−1 elements, so a
length *is* an int by the host's rule rather than by our analysis. A length **minus** a literal
cannot grow past the length it came from, which is the stencil's `(- (len a) 2)`. A length **plus**
a literal is refused, because `len + k` at a length near 2³¹ is exactly the overflow this exists to
avoid.

(3) is the same hazard from the other side. Together (1) and (3) give `v ∈ [init, B]`: the guard
exits when `v >= B`, so with a step of one `v` reaches `B` and stops — precisely the range Java's
own `for (int i = 0; i < a.length; i++)` occupies. A larger step could land past `B`.

**It is a representation selection, not a semantic change**, in the sense
[selection-2026-08-19](selection-2026-08-19.md) established: what is emitted changes, what the
program means does not. [ADR 0012](../../docs/decisions/0012-portable-integer-range.md) still says
`int` is exact within ±(2⁵³−1); this decides how to *store* one the compiler can bound.

### The `alloc` fill loop is narrow by construction

`(alloc t)` emits `new T[(int) n]` followed by a fill loop. If `n` did not fit in a host int the
**allocation itself** would have failed, so `j < n` is a bound an int can hold with no analysis at
all. That one is taken without going through the rule.

---

## 3. What it refuses, and the sieve is where both refusals matter

`examples/native/sieve-java.oro` narrows **neither** of its loops, and both refusals are correct:

```java
long i = 2;
for (;; i = (i + 1)) {
    if (((i * i) >= n)) break;       // bound is i*i >= n — NOT a length
    …
    long j = (i * i);
    for (;; j = (j + i)) {           // step is +i — NOT +1
```

- The outer loop exits on `i*i >= n`, and nothing says `n` fits in a host int.
- The inner loop advances by `i`, so at a length near 2³¹ it could pass the end.

The sieve still produces 2262 primes below 20000 and agrees with a hand-written reference over 2000
sizes. Being conservative costs it the `(int)` casts and nothing else.

---

## 4. Per target

**Java** is the only target that changes. **Go** does not: its `int` is the platform word and a Go
slice index takes it, so there was never a cast — verified, the emitted `dot` is byte-for-byte what
it was. **JavaScript** has no integer types at all. **x86-64** indexes with a 64-bit register.

That is the whole point of the parasite model showing up in a compiler pass: the cost existed on
exactly one host, for a reason particular to that host, and the fix is in that host's backend.

---

## 5. Method

Correctness first, on every program: the sieve against a reference over 2000 sizes, and
`NativeBench`/`NativeBench2`'s agreement checks, all before any timing.

The first attempt to measure this used the shared in-process harness while the machine was loaded
from a day of benchmarking, and produced `dot` generated at 94,714 ns against hand-written 30,706 —
a 3× "regression" that contradicted every other row in the same table. It was thrown away rather
than explained. One JVM per benchmark is what the numbers above use, which is the same discipline
[native-js-2026-08-20](native-js-2026-08-20.md) arrived at for V8 and §2 of
[native-java-2026-08-25](native-java-2026-08-25.md) for the `merge` question.
