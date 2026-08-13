# The gauntlet

The one fixed thing in an otherwise exploratory project.

Candidate cores are disposable. This test is not. Every candidate must express all five
programs and reach parity with hand-written code on all three initial targets.

## Why this exists

Exploration without a falsification test is how a project discovers its wall three years in —
which is what happened with Shen. Fixing the *language* early recreates that failure from the
other direction: a specification nobody can abandon.

So fix the test instead. The gauntlet is cheap (roughly a day to write the hand-written
reference implementations), it is reusable across every candidate forever, and it kills bad
cores in days rather than years.

## The programs

Each one exists to kill a specific class of core.

| # | Program | Kills |
|---|---|---|
| 1 | Dot product over `f64` slices | Boxing, hidden allocation, abstraction overhead. Shen fails here. |
| 2 | Centroid and bounding box over an array of structs | Bad value semantics, bad layout, per-element allocation |
| 3 | The same generic operation instantiated at two element types | Abstraction that is not actually free — tests monomorphization |
| 4 | Word frequency count over a text | **The parasite test.** Must emit Go's `map` and JS's `Map`, not a hand-rolled hash table |
| 5 | Formatted output to stdout | The Tier 2 binding story — `fmt` on Go, `console` on JS, `System.out` on Java |

Programs 1 and 4 are the important ones. 1 is where elegant cores die. 4 is where the whole
Parasite thesis is either true or false.

## Method

1. **Write the reference implementations first**, by hand, idiomatically, in Go, JavaScript,
   and Java. Before any compiler exists. These are the numbers to beat.
2. Record timings and output size on fixed inputs, with the toolchain versions.
3. For each candidate core, express all five programs and generate target code.
4. Compare against the reference on: wall time, allocation count, and output size.
5. A candidate **passes** a program on a target when it is within threshold on all three.

## Thresholds

- **Wall time:** within 5% of the hand-written reference. Faster is a legitimate result —
  program 1 should beat naive Go, since range-typed indices remove the bounds check.
- **Allocations:** must not exceed the reference. For program 1, the correct number is zero.
- **Output size:** within 20% of the reference, excluding toolchain-fixed overhead.

Program 4 has an additional pass condition that is not a number: **the generated Go must
contain `map[string]int`.** If it contains a hash table implementation, the candidate has
failed the Parasite model regardless of its timing.

## Recording results

Every candidate that dies gets an ADR naming what killed it. That is the mechanism that makes
dropping a direction an accumulating result rather than a loss — the point is to never
re-explore the same dead end after picking the project back up.

## Status

**Built and measured, 2026-08-13.** Code in [`gauntlet/`](../gauntlet/), results in
[`gauntlet/results/baseline-2026-08-13.md`](../gauntlet/results/baseline-2026-08-13.md).

- [x] Reference implementations in Go
- [x] Reference implementations in JavaScript
- [x] Reference implementations in Java
- [x] Harness: fixed inputs, timing, allocation counting
- [x] Baseline numbers recorded
- [ ] Output-size measurement
- [ ] CI wiring

The first run checked six claims the derivations had made about host compilers. **Five were
confirmed, five were refuted, and five findings nobody predicted appeared** — including one
that constrains the core's identity: Go's arbitrary-precision constant folding means
compile-time and runtime arithmetic can disagree, which would make partial evaluation unsound
over floats.

Corrections have been applied as notices at the top of the affected derivations rather than by
editing their reasoning, so the record of what was believed and what measurement said stays
readable.
