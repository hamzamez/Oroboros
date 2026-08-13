# What to implement first

Follows the [revised verdict](assessment-2026-08-13.md): build the vertical slice. This says
where the slice starts and why, and it is a response to a specific proposal — the one that would
normally be right.

---

## The proposal, and what is right about it

> Start with lambda calculus. Its specification is mathematical. The tests are the functions
> people have defined over it for decades. Then add naming, specified the same way, with its own
> tests. Then packages. The language grows slowly, each part fits, nothing looks out of place,
> and all of it is specified algebraically.

This is how the ML, Scheme, and Haskell families were built, and almost everything in it should
be kept:

- one feature at a time
- specified before implemented
- tests written alongside the specification
- each addition must compose with what is already there

None of that is in question. **Only the starting point is.**

## Why lambda calculus first fails here

Not because it is slow. Because **the tests would pass and teach nothing.**

Church numerals, the Y combinator, the classical derived functions — these test β-reduction's
*correctness*. They say nothing about whether the language can add two `int32`s at machine speed.
You would end up with a verified core, a green suite, and no information about the property that
stopped the predecessor.

Worth stating plainly: **Shen's core is correct.** Correctness was never what failed.

## The deeper reason, specific to a parasite language

In a Lisp, meaning is **intrinsic**. `(λx.x) y` means `y` because β says so, and the whole core
can be specified equationally without mentioning a machine.

Here, meaning is **extrinsic**. `(+ a b)` means nothing until you say *which Go expression it
becomes*, and its cost is part of its meaning, because
[ADR 0008](decisions/0008-measurement-over-principle.md) makes every parasite decision a
measurement. This core cannot be specified algebraically and declared done.

> **The first specification must be denotational into a target, not equational over terms.**

So the first thing to specify is not a calculus. It is a **vocabulary and its denotation**.

## What to build first

**The core vocabulary plus the Go emitter, validated by hand-written IR against the existing
baseline.**

No parser. No rewriter. No type system. No grading.

### The argument

1. **It tests the only claim nothing has tested.** Every number in
   [gauntlet/results/](../gauntlet/results/) measures hand-written host code — the bar. None
   measures output this project produced. This is the smallest artifact that changes that.
2. **It requires the fewest decisions.** Hand-write the IR as Go data structures;
   [experiments/legibility](../experiments/legibility/) already shows that is comfortable. The
   only question to answer is: *what are the core terms, and what Go does each emit?*
3. **It forces the real L0 decision with immediate feedback.** Every derivation has quietly
   assumed a vocabulary — `var`, `set`, `loop`, `when`, `break`, `at`, `len`, `+`, `*` — and none
   of it is written down or checked.
4. **The proposed methodology survives intact.** Specify first, one feature at a time, tests
   alongside, each addition must fit. Only *what gets specified first* changes.
5. **The test suite already exists.** Hand-write the residual that
   [g1](derivations/g1-dot-product.md) specifies exactly, emit it, compile it, and check for
   **1,389 ns at n=1024** and **zero allocations**.
6. **It fails fast and cheap.** If hand-written residual cannot reach parity, no parser or type
   checker would have helped — and the alternative is discovering that after building both.

### It can genuinely fail

This is not self-fulfilling. Real ways it goes wrong:

- Bounds checks not eliminated, because the emitted shape differs from the reference's.
- The emitter producing Go that the compiler treats differently than the hand-written form —
  which the [baseline](../gauntlet/results/baseline-2026-08-13.md) showed happens, repeatedly.
- The assumed vocabulary turning out to be wrong or incomplete.

Any of those is worth knowing in week one.

## Then grow, one feature at a time

Each step gated on the gauntlet, and nothing moves until the previous step holds.

| | Step | The test |
|---|---|---|
| 1 | **Core vocabulary + Go emitter** | Hand-written residual reaches the baseline |
| 2 | Reader, printer, canonical formatter | Round-trips; the IR becomes text rather than Go |
| 3 | **β-reduction — the rewriter** | Rewritten output *still* reaches the baseline |
| 4 | One rule layer (`collections`, from [g4](derivations/g4-word-count.md)) | g4's program reaches baseline, and the output contains `map[string]int` |
| 5 | **JS backend, before any front-end features** | The core survives the most hostile host |
| 6 | Grading and multiplicity | The cost report matches [g6](derivations/g6-escaping-closures.md)'s measured numbers |
| 7 | Bindings | `fmt` on Go, `console` on JS |

Lambda calculus is **step 3, not step 1**. Not demoted — [g6](derivations/g6-escaping-closures.md)
established the core *is* staged lambda calculus, and it remains the heart. But it becomes
*testable* only once a residual can be emitted and measured. Built first, it can only be tested
against Church numerals, which prove something nobody doubted.

## The one thing to hold on to

The gauntlet has never once agreed with an unmeasured prediction made in this project. Step 1
exists so that the first thing built is the first thing measured, and so that every step after it
inherits a test rather than an assumption.
