# How often can the compiler prove an integer stays in a machine word?

2026-08-19. The experiment [data-model.md §8](../../docs/spec/data-model.md) named as the gate on
the integer design.

## Why this number decides something

hamza's direction is **exact integers by default, with declared ranges and inferred intervals
choosing the representation** — correctness and performance at once, and the burden of declaring
ranges on the programmer.

Its viability rests entirely on one unknown. Every operation the compiler *cannot* bound carries an
overflow check, and [product-2026-08-19](product-2026-08-19.md) priced those: **1.19×–1.81× on
addition, 1.87× on multiplication where the hardware is reachable and 7.40× where it is not**, with
`math/big` at 38.9× when a value actually grows. If ranges are usually provable the design is what
it claims to be. If they are not, the checks are everywhere and it is a tax.

## What was built

`emit/interval.go` and `cmd/intervals` — the classical interval domain (Cousot & Cousot, POPL 1977)
over the **residual**, which is where this project's analyses live because reduction has already
made the term monomorphic, first-order and closed.

- guard refinement, so `(< i n)` narrows `i` inside the branch that tests it;
- a loop fixpoint with **widening then narrowing**;
- inversion of `x*x < e`, because `while (i*i < n)` is not an exotic pattern;
- `-assume N`, the experimental knob: give every otherwise-unbounded parameter and array length the
  range `[0, N]`, simulating a programmer who declared ranges without rewriting seven programs.

Counted: applications of `+`, `−`, `×` and negation producing an `int` — the operations that can
leave the window. Division cannot grow a value and comparison produces no integer.

**Soundness is tested, not assumed** (`emit/interval_test.go`). The analysis must *fail* to prove a
product of two 2³⁰-bounded values, an accumulator over an unbounded loop, and anything about an
undeclared parameter. A change that raises the headline number by breaking one of those has
measured nothing.

## The result

| program | target | nothing declared | one range declared |
|---|---|---|---|
| `sieve-go.oro` | go | 9/20 — 45% | **18/20 — 90%** |
| `sieve-java.oro` | java | 0/10 — 0% | **9/10 — 90%** |
| `sieve-win.oro` | windows | 10/25 — 40% | **19/25 — 76%** |
| `sieve-win-bench.oro` | windows | 11/17 — 65% | 11/17 — 65% |
| `shortcircuit-go.oro` | go | 1/1 — 100% | 1/1 — 100% |
| `smooth.oro` | portable-go | 0/3 — 0% | **3/3 — 100%** |
| `stencil.oro` | portable-go | 0/3 — 0% | **3/3 — 100%** |
| `search.oro` | portable-go | 0/1 — 0% | **1/1 — 100%** |
| **total** | | **31/80 — 39%** | **65/80 — 81%** |

## Three findings

### 1. Declaring one range on the inputs is worth roughly double

39% → 81%, from giving parameters and array lengths a bound. That is exactly the trade hamza
proposed — *the burden falls on the programmer to decide which range to use* — and it is the
difference between most arithmetic being checked and most of it not being.

The unit of declaration is small: a program's *parameters*, not its every variable. Everything
downstream follows from propagation and the loop guards the program already contains.

> **Closed the same day by [sct-2026-08-19](sct-2026-08-19.md).** Size-change termination plus a
> trip count took these numbers to **54% undeclared and 100% declared**, and the residue described
> below is gone. Two of the three bugs found while building it had been making *these* numbers too
> low as well.

### 2. The entire residue is ONE class

Not a scatter. Every unproven operation left at 81% is a loop variable **whose bound comes from the
trip count rather than from a guard on itself**:

```
count-primes: go.+ [1, +inf] in (go.+ acc 1)      ; counts primes; bounded by the trip count
win/fmt:      x64.sub [-inf, 24] in (x64.sub i 1) ; a digit index; the guard is on the QUOTIENT
```

`acc` is bounded by `n` because the loop runs at most `n` times. The digit index is bounded by 19
because dividing by ten reaches zero. Neither bound is visible to an analysis that only reads
guards, and **both are standard**: a trip-count analysis derives the first from a counter with a
bounded range and a fixed step, and the second from a decreasing measure.

So 81% is a **lower bound on a lower bound**. The obvious next increment is well understood and
unimplemented.

### 3. Where a call site is concrete, everything is provable

`sieve-go.oro` reaches 45% with nothing declared, where `sieve-java.oro` reaches 0%. The difference
is not the language — it is that Go's version exports `main`, which calls `count-primes` with the
literal `20000`, and **reduction substitutes it**. Inside `main` every bound is a constant.

That is partial evaluation feeding the interval analysis for free, and it is the same mechanism
that gives this project generics with no generics and fusion with no fusion rules. Whole-program
reduction is an interval analysis's best friend, because it turns parameters into literals wherever
a caller is concrete.

## What the experiment cost, and one mistake worth recording

Two rounds, and the first was wrong in a way worth naming.

The first run reported **10–20%**, and the cause was entirely inside the analysis: widening threw
each growing counter to infinity *before* the loop's own guard could cap it, and there was no
descending phase to recover the bound. Adding the classical narrowing iterations took the same
programs from 20% to 90%.

> **The first numbers measured the absence of a standard technique, not a property of the
> programs.** Recorded because it is the same shape as the retracted "1117× loop encoding": an
> implementation's weakness read as a language's limit.

## What this does not settle

It measures **provability**, not the end-to-end cost of the design. Still open, and in order:

1. What an unproven operation actually costs *in context* — the checks were priced in isolation, in
   a tight loop, not inside a real program where the branch may be free.
2. Whether trip-count analysis lifts the residue, and at what implementation cost.
3. What the annotation actually looks like. `-assume N` is a knob, not syntax; nothing here decides
   how a range is written, which is [data-model.md §7](../../docs/spec/data-model.md)'s question 1.
4. Floats. Every gauntlet program above is float-heavy and contributes almost no integer
   operations — the corpus for this question is the sieves, and it is small.
