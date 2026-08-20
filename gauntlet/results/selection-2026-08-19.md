# Ranges select the representation, 2026-08-19

The payoff the last four rounds were for. A declared range now changes the **emitted code**.

## The rule

An integer operation whose result the compiler can prove stays inside the portable window keeps the
host's own operator. One it cannot prove is rewritten to the **checked** primitive the target
declares. A target that declares none gets its term back unchanged, and covering reports that it
cannot do exact arithmetic — the capability model answering rather than a special case.

The declaration is one line, and it is syntax the language already had:

```lisp
(sig count-primes ((n int)) int (where (and (<= 0 n) (< n 1048576))))
```

## The demonstration

`examples/native/sieve-go.oro`, the same program, changing only that line:

| | integer operations checked | loops proven terminating |
|---|---|---|
| as written, no range declared | **10 of 10** | 0 of 3 |
| one line added to the `sig` | **0 of 10** | 3 of 3 |

Nothing else about the program changes. The emitted Go goes from seven overflow-checking call sites
to none, and the artifact is what it always was.

## What it looks like, on each host

Fibonacci, whose `i + 1` is bounded by the loop guard and whose `a + b` is not — `fib(79)` is
past 2⁵³, so refusing to bound it is correct.

**Go** — no overflow intrinsic, so the check is a func literal called immediately. Each operand
appears once and Go's inliner sees straight-line code. No runtime, no prelude:

```go
a, b, i = b,
    (func(a, b int) int { t := a + b; if (t > a) != (b > 0) { panic("int overflow") }; return t }(a, b)),
    (i + 1)
```

**Java** — the JVM has the intrinsic:

```java
final var u2 = (Math.addExact(a, b));
final var u3 = (i + 1);
```

**windows** — the hardware has carried an overflow flag since 1978, so it is one instruction and a
perfectly-predicted branch. `ud2` raises #UD, ending the process the way an unrecoverable
arithmetic fault should:

```asm
add r12, 1          ; bounded: plain
...
add r14, rdi        ; not bounded: checked
jno Lok4
ud2
Lok4:
```

**JavaScript** declares no checked form. There is no fixnum/bignum unification to hook into, and
`Number.isSafeInteger` measured worst of the four
([overflow-2026-08-19](overflow-2026-08-19.md)). So JS programs are unchanged and the covering
check is what tells a program it cannot have exact arithmetic there.

**The price list from that measurement predicted this exactly**: Java 1.31×, x86 ~1×, Go 2.61×,
JavaScript 3.74×. The two hosts that can do it natively are the JVM and bare metal; the two that
cannot are Go and JavaScript.

## The corpus, grown

`examples/int/` — six integer programs, chosen so that **three of them should be refused**. An
analysis that proves all six has a bug.

| program | ops proven | terminates | and that is right because |
|---|---|---|---|
| `sum-range` | **2/2** | yes | the accumulator is bounded by the trip count |
| `digit-sum` | **1/1** | yes | `m / 10` descends geometrically; `s` grows by ≤ 9 per step |
| `gcd` | — | **yes** | `x mod y < y` — an arc from y to y through an expression y does not head |
| `fib` | 1/2 | yes | it terminates, and `a`/`b` genuinely leave the window at n = 79 |
| `power` | 0/3 | yes | `k` halves, so it stops; `acc` and `x` multiply, so they do not stay |
| `collatz` | 0/4 | **no** | the Collatz conjecture is open |

`gcd` is the case Lee, Jones & Ben-Amram use to motivate the principle: neither variable descends on
its own — x becomes y, and y becomes x mod y.

`collatz` and the two partial refusals are the point of the corpus. A proof system is only worth
what it declines to prove.

## Four gaps the corpus found

None of these were visible on the sieves, which is the argument for growing a corpus rather than
polishing one program.

1. **`go./` was not recognised as division at all.** `opAlias` mapped `+`, `-`, `*` and the
   comparisons but not `/` or `%`, so every geometric descent — the digit loop, exponentiation by
   squaring — looked like an unknown operation.
2. **`x mod y < y` was not an arc.** Euclid's algorithm terminates for exactly this reason, and the
   analysis could not see it until a program contained it.
3. **A guard `(== y 0)` gave nothing in the branch where it is false.** Its negation is a
   disequality, and a disequality against an *endpoint* narrows an interval perfectly well — `y ≠ 0`
   on `[0, n]` is `[1, n]`. Without it neither `gcd` nor `power` could be shown to terminate. This
   is the cheap half of the `d ≠ 0` problem
   ([types-direction.md §6.7](../../docs/types-direction.md)) arriving from a different direction.
4. **The remainder's sign was thrown away**, so `s + (m % 10)` grew in both directions.

## What is guaranteed

`emit/interval_test.go` holds two tests that matter more than the numbers:

- **The rewrite is the identity** on a target that declares no checked forms. The pass rebuilds the
  entire residual, so a target that opts out must get back exactly what it gave.
- **The selection goes both ways.** An operation the compiler can bound keeps the host operator; one
  it cannot gets the checked form. A pass that rewrote everything would be no more useful than one
  that rewrote nothing.

Plus the soundness cases: the analysis must still fail to bound a product of two 2³⁰ values, an
accumulator over an unbounded loop, and an undeclared parameter.

> **Opt-in since 2026-08-20.** The rewrite is behind `-checked` and off by default. Wiring it into
> the default path reversed ADR 0012 without an ADR, breached requirement 5 by up to 4.54×, and made
> the same program trap on three targets and silently lose precision on the fourth
> ([assessment-2026-08-20 §2](../../docs/assessment-2026-08-20.md)). The analysis still runs and
> still reports what it proved.

## What this does not do

**It does not make `int` exact.** The checked form *traps*; it does not promote to a bignum. This is
the representation-selection machinery with two representations, and the third — arbitrary precision
— is still [data-model.md §7](../../docs/spec/data-model.md)'s eleven unanswered questions.

~~**It is not measured for speed.**~~ **Measured** —
[checkcost-2026-08-19](checkcost-2026-08-19.md). Same source compiled twice, differing only in the
declared range: **4.54× on Go and 1.52× on Java for an arithmetic-bound loop, 1.23× on Go and 1.46×
on windows for a memory-bound one.** The isolated numbers were wrong in *both* directions, and the
spread within one design is wider than the spread across hosts.

**And the corpus is still small.** Six integer programs and four sieves. Better than one program,
not yet a bar.
