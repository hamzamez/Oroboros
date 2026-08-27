# Index narrowing from the interval analysis — 2026-08-27

The third piece of [precision-integers.md](../../docs/precision-integers.md), after
[element width](elemwidth-2026-08-27.md) on the read and write sides.

[indextype-2026-08-25](indextype-2026-08-25.md) narrows a loop counter to the host's own `int` when
it is **bounded by a length** and **steps by +1**, because a Java array index is 32-bit and our
`int` is 64. It named the program it could not help:

> the sieve narrows *neither* of its loops because its bound is `i*i >= n` and its step is `+i`.

That is now the general rule instead of a pattern: **the interval analysis bounds the loop, and if
every integer operation in the method stays inside a 32-bit index, its counters are held in one.**

| Java, `examples/table/sieve.oro`, N = 100,000 | ns/op | |
|---|---|---|
| hand-written, `int` index | 161,955 | what a person writes |
| generated, `long` index (before) | 187,628 | **1.16×** |
| **generated, `int` index (after)** | **159,731** | **0.99× — parity** |

`int` locals went from **0 to 4** and `(int)` casts from **4 to 1** — the survivor being
`new boolean[(int) n]`, where `n` is the method's own parameter and the cast is unavoidable.

---

## 1. What it rests on

Holding a value in 32 bits computes the same answer as in 64 **exactly when every intermediate stays
inside 32 bits**. So two things have to hold, and only the first is what the analysis reports.

**`MaxOp` fits.** A new field: the join of every *checkable* operation's interval. That is not the
same question as `fits`, which is the portable window at ±(2⁵³−1) — a value bounded by 2⁵³ does not
fit a 32-bit index, and the two answers diverge on exactly the programs this is for.

**Every value a counter can TAKE fits, and MaxOp does not cover all of those.** A literal is not an
operation. Neither is a read out of a table, whose element range this pass has not got. So each
variable's sources are checked directly by `fitsIndexSource`, and anything unrecognised refuses.

The list of what MaxOp counts had to become one thing rather than two. `transfer` marks add, sub,
mul and neg checkable and leaves **division and remainder bounded but uncounted** — so a rule
trusting MaxOp must not trust a division. That distinction was inside a switch statement; it is now
`arithOp`, which both the transfer function and the narrowing rule read.

Tested by refusal: a literal past 2³¹, a free name, a division, a table read, and a conditional
hiding any of those.

## 2. It has to be a WHOLE-METHOD question, and that is the interesting part

The first version ran the analysis on the loop alone and narrowed nothing. The reason is worth
keeping, because it is the same property that makes
[buffer narrowing](elemwidth-2026-08-27.md#5d) safe, biting from the other side:

**a loop's bound usually comes from the enclosing `where`.** Analysing the loop subterm loses `n`,
so `i*i < n` bounds nothing and every counter stays wide. Less context can only widen — which is
exactly the conservatism that lets `BufferRange` run on a `build` lambda in isolation, and exactly
the wrong trade here.

So the emitter asks once, with the signature in hand, and the answer applies to every loop in the
method. That is coarse — **one unbounded operation anywhere refuses all of them** — and it is the
safe coarseness.

## 3. It changed nothing on the existing gauntlet, and that is the honest result

Regenerating all nine `examples/native/*-java.oro` before and after: **identical output**. The
syntactic rule already covered every loop there, because they are `+1` counters over lengths — which
is what it was written for.

The generalisation earns its place on exactly one shape: **a computed bound**. That is the sieve,
and it needed one more thing before it could show it.

## 4. `sieve-java.oro` declared no range, and a target template hides the cast anyway

Two separate findings, and both are about where a fact lives.

**The Java sieve had no `where`.** `sieve-go.oro` carries `(< n 1048576)` and `sieve-java.oro` did
not, so the analysis could not bound it and neither rule could fire. Adding the clause takes it from
0 `int` locals to 4. The range is the input to all of this, and half the corpus does not declare
one.

**And its casts still do not go away**, because it is written against *target-declared* accessors:

```lisp
(prim at-bool  ((v bool-array) (i int)) bool  expr "%s[(int) %s]" pure index)
(prim set-bool (bool-array int bool) bool-array stmt "%s[(int) %s] = %s" (length-of 0))
```

The cast is **in the template**, so the emitter never gets a say and narrowing cannot reach it. A
target file cannot express *"cast unless the index is already an int"*, and it should not have to.

`examples/table/sieve.oro` is the same program on structural indexing — `(c i)` — where the cast is
the backend's and the narrowing applies. That is §1's benchmark, and it is an argument for
structural indexing that [tables.md](../../docs/spec/tables.md) did not make: **a host detail buried
in a target template is a host detail no analysis can improve.**

## 5. What it still cannot do

Neither parser narrows. `tokenize.oro` and `tree.oro` both report `idx -inf..+inf`, for the reason
[precision-integers.md §2.1](../../docs/precision-integers.md) isolated: the progress variable is
assigned a **scanner's return value**, so there is no size-change witness, no trip count, and no
bound on anything derived from it.

So Java's remaining gap on both programs is unchanged and its cause is unchanged. **The third
independent demand for a postcondition naming a result** — after the precision-integer plan and
after [elemwidth §5](elemwidth-2026-08-27.md) — and the second time this week that a measured cost
has terminated at the same missing feature.
