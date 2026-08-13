# The gauntlet

Hand-written reference implementations in Go, JavaScript, and Java. These are the numbers a
candidate core must reach parity with — see [docs/gauntlet.md](../docs/gauntlet.md) for what
each program exists to kill, and [ADR 0007](../docs/decisions/0007-exploration-over-specification.md)
for why this is the project's one fixed commitment.

Nothing here is Oroboros code. It is the bar, written by hand, in each target language.

## Running

```bash
cd gauntlet/go && go test -bench=. -benchmem -benchtime=2s -count=3
```

```bash
cd gauntlet/js && node bench.mjs
```

```bash
cd gauntlet/java && javac -d out Gauntlet.java Bench.java && java -cp out Bench
```

The L1-resident variants matter more than the full-size ones for anything compiler-related —
at n=65536 the loops are memory-bound and hide everything:

```bash
cd gauntlet/go && go test -bench=Small -benchtime=3s -count=5
```

## Checking claims, not just timings

Timings alone cannot tell you *why*. Go will report its own decisions:

```bash
go build -gcflags="-d=ssa/check_bce/debug=1" ./...
```

```bash
go build -gcflags="-m -m" ./...
```

The first lists every surviving bounds check; the second lists inlining and escape decisions
with their cost against the budget. Both were decisive in the first run — the inlining output
refuted a claim that the timings alone would only have made ambiguous.

## Layout

```
go/     gauntlet.go     reference implementations, with both forms wherever a
                        derivation made a claim about what Go's compiler does
        unordered.go    multi-accumulator variants, to price strict left-to-right
        input.go        deterministic inputs
        *_test.go       benchmarks; small_test.go is the L1-resident set
js/     gauntlet.mjs    references, AoS and SoA both
        bench.mjs       median-of-runs harness with a V8 warmup
java/   Gauntlet.java   references
        Bench.java      median-of-runs harness with a C2 warmup
fmt/                    float formatting and constant-folding divergence checks
results/                recorded baselines, dated
```

Inputs use the same Mulberry32 generator in JS and Java so the two see identical data. Go uses
its own PRNG — sizes and distributions match, exact values do not.

## Adding a program

Keep both forms whenever a derivation claims a host compiler does or does not do something. The
first run refuted five claims, and it could only do that because the losing form was there to
measure. A benchmark that only contains what you expect to win teaches nothing.

## Caveats

Benchmarks were taken on a hybrid P/E-core laptop, which produced occasional ~3× fast outliers
across unrelated cases. Medians are reported. **Treat differences under ~15% as noise** on this
class of hardware, and re-measure on a pinned machine before anything depends on a small margin.
