# Correction: the loop primitive was never the problem

**The 1117× was real and the conclusion drawn from it was wrong.** It measured the cost of writing
a loop the obvious way, not the cost of `fold-range`. Written the other way — computing the trip
count and deriving the index — the *same* sieve, with *no new primitive*, runs at **parity with
idiomatic hand-written Go**.

| | Go | JavaScript |
|---|---|---|
| hand-written, idiomatic | 15.4 µs | 53.6 µs |
| generated, naive encoding | 15,580 µs — **904×** | 16,398 µs — **249×** |
| **generated, counted encoding** | **18.5 µs — 1.2×** | **92.6 µs — 1.73×** |

Supersedes the headline of [go-toplevel](go-toplevel-2026-08-18.md),
[js-toplevel](js-toplevel-2026-08-18.md) and [java-toplevel](java-toplevel-2026-08-18.md). Their
measurements stand; their inference does not.

---

## 1. What was missed

`fold-range`'s bound has always been an **arbitrary expression** — the stencil already uses
`(int.sub (alen a) 2)`. So a loop with a start and a step does not need a start and a step: it needs
its **trip count**, which is arithmetic.

```
for j := i*i; j < n; j += i        becomes        count = (n + i - (i*i + 1)) / i
                                                  j     = i*i + k*i
```

and the outer `for i := 2; i*i < n; i++` becomes a count of `⌊√n⌋ − 1` with `i = k + 2`.

That is [experiments/go-toplevel/sieve_counted.oro](../../experiments/go-toplevel/sieve_counted.oro),
and it emits:

```go
var n1 int64 = ((int64((math.Sqrt((float64(20000)))))) - 1)
for k := int64(0); k < n1; k++ {
    var i int64 = (k + 2)
    …
    for k2 := int64(0); k2 < n4; k2++ {
        c2[((i * i) + (k2 * i))] = (true)
    }
}
```

O(n log log n), like the hand-written one. The earlier version was O(n²) because **I wrote it that
way**, not because the language forced it.

## 2. Two further surprises in the decomposition

Measured on Go, n = 20000, alongside:

| | µs |
|---|---|
| hand-written, `for j := i*i; j < n; j += i` | 15.6 |
| hand-written, counted — `start + k*i` | 59.5 |
| hand-written, counted **with an explicit step variable** | 57.5 |
| **generated, counted** | **15.6** |

**An explicit `step` buys nothing.** 57.5 against 59.5 is inside the noise floor. Go's strength
reduction already turns `start + k*i` into an induction variable, so a `step` parameter on the
primitive would be a legibility feature with no measurable performance content. That kills one of
the three things §3 of the earlier results called missing.

**The generated code is faster than the hand-written counted version**, 15.6 against 59.5. Both are
counted; ours bounds the outer loop by √n and the hand-written one scans to n with a `continue`.
Which is to say the remaining difference between encodings is not about loops at all.

## 3. What is actually missing, after the correction

Two things, and they are the two that have no trip count by construction:

**Early exit.** A loop that stops on a data-dependent condition — `find`, `any`, `all`,
`takeWhile`, a linear probe that stops when it hits the key. There is no count to compute, because
the count is what you are looking for. [java-toplevel §3](java-toplevel-2026-08-18.md) measures this
at **2×** on a linear probe, and says why that is a floor rather than a typical case.

**Unbounded iteration.** A loop with no bound at all: convergence, streaming input, `while (!done)`.
The language currently computes exactly the primitive recursive functions
([loops.md §3.1](../../docs/spec/loops.md)), so this is not a performance question — it is the
difference between "a very large class of programs" and "all computation".

And one thing that is not a capability at all:

**Legibility.** The counted encoding is *ugly*. `(multiples i n)`, `(int.add start (int.mul k i))`,
an `isqrt` defined in terms of a float square root. It is what a compiler emits, written by hand. A
`(loop-range z start stop step f)` primitive would be **sugar over what already works**, and the
honest case for it is that programs should be readable — not that it is worth 1117×.

That is a much weaker case than the one the earlier results made, and it should be argued on its own
terms.

## 4. Why the mistake happened, which is the useful part

The naive sieve is what anybody writes first, and it *is* what the language makes easy. The
measurement of it was accurate. The failure was in the sentence that followed:

> "It is the *source* that cannot say the fast thing, not the emitter that fails to produce it."

The source could say it. I had not tried.

Two guards would have caught this and neither was applied:

1. **Carry both forms.** [CLAUDE.md](../../CLAUDE.md) says it explicitly — *"when adding to the
   gauntlet, carry both forms: the one expected to win and the one expected to lose"*. Three
   encodings of the sieve were measured; the one expected to win was never written, because it was
   assumed inexpressible.
2. **A negative result about expressiveness needs a proof, not a failure to think of one.** "This
   cannot be written" is a much stronger claim than "this is slow", and it was made on the evidence
   for the weaker one.

The rule this earns:

> **Before recording that the language cannot express something, write the thing you believe it
> cannot express and watch it fail.** A measurement of the obvious encoding measures the obvious
> encoding.

## 5. What still stands from the earlier results

Everything that was actually measured:

- The **emitter is at or ahead of parity** on all three hosts, 0.77× / 0.56× / 0.44× against
  hand-written code in the same encoding — and now also 15.6 vs 59.5 in the counted encoding.
- **JavaScript's higher-order array API is 3.6×–133× slower than a loop**, so refusing closures
  costs nothing there.
- **`java.lang` has no collections**, and the linear-probe replacement costs 2× plus 2×.
- **A product type is wanted from three directions**, and a fresh product accumulator costs 6.4× on
  the JVM and 13.8× on JS.
- Two emitter bugs, one a silent wrong answer.

None of those depended on the retracted inference.
