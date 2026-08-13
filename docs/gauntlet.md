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
| 6 | Stencil over a slice that may alias itself, plus dict update | Uniqueness. Where in-place mutation is unsound, and what failing to prove it costs |

Programs 1 and 4 are the important ones. 1 is where elegant cores die. 4 is where the whole
Parasite thesis is either true or false.

Program 6 was added after [s2](derivations/s2-multiplicity-inference.md) found that the first
five never mutate a shared structure, leaving uniqueness untested. It is the only program where
a **correctness** hazard is the point: the same optimization gives different answers depending on
whether two slices alias, and no target language can express that they do not.

## Method

1. **Write the reference implementations first**, by hand, idiomatically, in Go, JavaScript,
   and Java. Before any compiler exists. These are the numbers to beat.
2. Record timings and output size on fixed inputs, with the toolchain versions.
3. For each candidate core, express all five programs and generate target code.
4. Compare against the reference on: wall time, allocation count, and output size.
5. A candidate **passes** a program on a target when it is within threshold on all three.

## Thresholds

- **Wall time:** within 5% of the hand-written reference — **but only on hardware that can
  resolve 5%.** The first run was taken on a hybrid P/E-core laptop that produced ~3× outliers
  across unrelated benchmarks; on that machine nothing under ~15% is a real difference. Report
  medians, state the noise floor, and do not let a decision rest on a margin smaller than it.
- **Allocations:** must not exceed the reference. For program 1, the correct number is zero.
- **Output size:** within 20% of the reference, excluding toolchain-fixed overhead. **Measured**
  — [size baseline](../gauntlet/results/size-2026-08-13.md). Note that "excluding toolchain-fixed
  overhead" is doing nearly all the work on Go, whose floor is **1.43 MB** against which 200
  specialized call sites add 1.0%. On JS, measure **gzipped** size for transfer and **raw** size
  for parse time; they differ by 24× on specialized output.

Program 4 has an additional pass condition that is not a number: **the generated code must use
the host's own dictionary, not one of ours.** Which dictionary that is, is a measurement — the
first run found a null-prototype `Object` beats `Map` by 3.25× on JS, so the earlier form of
this condition ("must contain `Map`") was itself an unmeasured assumption. See
[ADR 0008](decisions/0008-measurement-over-principle.md).

**Faster than the reference is a legitimate result, but do not assume where it will come from.**
Program 1 was expected to beat hand-written Go via bounds-check elimination. The check is
verifiably removed and the gain is zero — the loop is bottlenecked on the serial `acc +=`
dependency chain.

## Carry both forms

Whenever a design argument claims a host compiler does or does not do something, the gauntlet
must contain **both** forms — the one expected to win and the one expected to lose.

This is not thoroughness for its own sake. The first run refuted five beliefs, and it could only
do that because the losing form was present to measure. A benchmark containing only what you
expect to win teaches nothing and quietly confirms whatever you already thought.

Timings alone are also not enough. Go will state its own decisions directly:

```bash
go build -gcflags="-d=ssa/check_bce/debug=1" ./...
```

```bash
go build -gcflags="-m -m" ./...
```

The inlining output refuted a claim that the timings alone left ambiguous.

## Recording results

Every candidate that dies gets an ADR naming what killed it. That is the mechanism that makes
dropping a direction an accumulating result rather than a loss — the point is to never
re-explore the same dead end after picking the project back up.

The same applies to *beliefs* that die. Results are dated and version-stamped in
[`gauntlet/results/`](../gauntlet/results/), because a parasite decision can be invalidated by
someone else's compiler release ([ADR 0008](decisions/0008-measurement-over-principle.md)).

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
