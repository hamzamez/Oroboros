# The one-command check

```bash
go run ./cmd/check                    # every step
go run ./cmd/check -skip tooling      # every step but the host surveys, which need every toolchain
go run ./cmd/check -only emission     # one step, or a comma-separated list
go run ./cmd/check -accept "reason"   # keep this run's emission as the baseline, and log why
```

| step | what it runs |
|---|---|
| `vet` | `go vet ./...` |
| `compiler` | `go test ./core/ ./emit/ ./ir/ ./cmd/...` |
| `emission` | every `.oro` under `examples/`, `lib/` and `gauntlet/differential/cases/`, on every target, compared with the baseline in this directory — its output byte for byte, and what each compile cost |
| `ir` | the IR ([docs/spec/ir.md](../../docs/spec/ir.md) §11): the sweep runs `gen -ir`, so every program that emits is also lowered, verified (W1–W10) and printed to `.check/ir/`. This step fails if a program that emitted has no IR, a refused one has one, or `print ∘ read ∘ print ≠ print`. It keeps no baseline of its own until a printer reads the IR |
| `differential` | `gauntlet/differential/run.go`: build, **run** and agree on four targets |
| `tooling` | `go test ./gauntlet/stdlib/`: the surveys twice, the pins, the acceptance programs, the hand declarations against the host |

**Every step runs**, whatever the ones before it did, so one run reports everything. Exit codes:
`0` all pass and emission is byte-identical, `1` a step failed, `2` emission changed or a compile got slower, and needs review.
**Under `go run` a non-zero code shows as 1**, with the real one on the last line (`exit status 2`).
The summary's `REVIEW` or `FAIL` is what to read; `go build ./cmd/check` gives a binary whose exit code
is the real one.
Every step's full output is in `.check/logs/`, which git ignores.

## The rule for emission

**Byte-identical is the default.** Emitted code is compared byte for byte with `testdata/emitted/`, and every
compiler outcome with `outcomes.txt`:
- whether each source emitted or was refused on each target;
- the hash of what it emitted;
- every line the compiler printed, including the per-program **proof counts** and any refusal.

So a weaker proof shows as a change before it shows as a refusal, and a program that stops emitting
cannot vanish from the comparison. The sweep this replaces did both of those wrong, and it skipped
`examples/tally`.

**A difference is a change, not a failure and not a pass.** It is reported for review (exit code `2`),
with a `git diff --no-index` command for each changed file. Then:

- **Keep it** when the new result is **correct and no slower**:
  1. the compiler, differential and tooling suites pass;
  2. where the program is one the gauntlet benchmarks, the benchmark does not regress beyond the 15%
     noise floor (CLAUDE.md), measured against hand-written code as the gauntlet requires.

  Then `-accept "reason"` makes it the baseline and appends the reason to [ACCEPTED.md](ACCEPTED.md).
  The reason should say what changed and what evidence shows it is an improvement.
- **Otherwise it is a regression**, and the code is fixed, not the baseline.

**`-accept` is refused unless the compiler and differential steps ran and passed in the same run.** The
reason is written by a person; the correctness evidence is not. If the tooling step was skipped, the log
records that.

Two conventions the sweep reads rather than guesses (ADR 0023):
- **The differential cases are compiled with `-checked`**, because `run.go` builds them that way.
- **An example compiled with `-checked`** says so on its first line: `; BUILT WITH \`-checked\``.

## The rule for compile time

**A compile is SLOWER than the baseline when it takes at least 1.5× the time and at least 250 ms
more.** Either half alone is wrong, and both were measured rather than chosen
([compiletime-2026-09-21](../results/compiletime-2026-09-21.md)):

- the ratio **1.5** sits above the largest variation seen between three sweeps of one compiler for a
  compile over 300 ms (1.21×), and below the smallest regression this gate exists for (freq, 1.67×);
- the delta **250 ms** sits above what Windows' 15.6 ms CPU-time tick does to a small program, where
  16 ms against 47 ms is one tick against three and a ratio of 2.9 means nothing.

The delta is also what lets the rule catch a *small* program that became slow. A floor on the
baseline would have been the obvious design, and it would have missed the tokeniser, whose baseline
was 234 ms when it went to 2,109.

**What is measured** is the CPU time of each `gen` process in the emission sweep — user plus system.
It is not the time a person waits: the sweep runs 16 compilations at once, and the tokeniser costs
about 2.5× its serial time there. It is **repeatable on a quiet machine**, because the job order and
so the contention are fixed. It is **not repeatable against a neighbour's load**
([compiletime-2026-09-25](../results/compiletime-2026-09-25.md)):
- the same compile of freq costs 1.6× the CPU on an E-core as on a P-core;
- Go's collector runs idle mark workers on every idle core, so a GC-heavy compile's CPU time also
  grows with how many cores happen to be idle.

Under load, freq read 1.63× in two consecutive sweeps with the compiler unchanged. So **the sweep
screens, and a pair decides.** This run's times, with wall time beside them, are in
`.check/compiletime.txt`.

**Noise only adds time** — an E-core, a cache miss, a neighbour's GC — so the minimum over repeated
observations is the estimate (Chen & Revels, *Robust benchmarking in noisy environments*, 2016).
Three consequences:

- **the baseline is the minimum of two sweeps**, because one slow baseline would hide a regression;
- **a suspect explains itself before it is reported**: the sweep runs again and the rule is applied
  to the per-compile minimum. A clean run pays nothing for this. The second sweep also compares the
  two sweeps' emission, which is the only place the check runs the emitter twice;
- **a suspect both sweeps keep is PAIRED against the baseline's own binary.**
  - That binary is `gen` built from the commit that last wrote `compiletime.txt`, in a tree extracted
    with `git archive`, so it reads its own targets and sources.
  - It and this run's `gen` compile the suspect alternately and serially, three pairs each, with
    `GOMAXPROCS=1`. gen is single-threaded, so one P changes only how its collector is scheduled,
    and CPU time becomes the compile's work.
  - The rule is applied to the ratio of the two minima. Whatever the machine is doing, it does to
    both.

  A suspect the pair clears is **not reported** and **keeps its baseline time**, so `-accept` never
  records the inflated one. A suspect it confirms is listed with both figures. When no pair can be
  made, the suspect stands and the note says why: `compiletime.txt` has uncommitted changes, or the
  baseline commit does not build.

  **Serial alone would not do.** freq costs 0.68× its sweep time serially, so a serial time held
  against the sweep's baseline would let a 1.47× regression through. The pair costs minutes (6 to 8
  with freq's three targets), and only on a run the sweep flagged twice.

**The machine moves too.** The same binary measured 7.0 s on 2026-09-17 and 8.8 s on 2026-09-21. So
the verdict carries the **median ratio** over the compiles above 300 ms: one regression moves a few
of them, the machine moves all of them. A median of 1.3× or more is reported as the machine being
slower than when the baseline was taken, with the advice to re-run idle and on mains power.

**A slower compile is a change for review**, like a changed output: exit code `2`, the compiles
listed with their ratios, and kept only with `-accept "reason"`, which logs every accepted slower
compile in [ACCEPTED.md](ACCEPTED.md).

**A FASTER compile is the mirror**: at most 1/1.5 of the baseline and at least 250 ms quicker. It is
reported, not a failure, and `-accept` records it — the baseline used to move only when something
else was accepted, so a buy-back could never be locked in and a regression all the way back would
have passed ([tokenize-compile-2026-09-23](../results/tokenize-compile-2026-09-23.md)). A real gain
below the rule's resolution — the memos' 1.78× serial measured 1.48× in the sweep — is recorded
deliberately with `-accept "reason" -retime`, which re-records the time baseline from two sweeps.

**What this does not catch**: a slowdown under 1.5× on one program (freq on the JVM moved 1.30× at
7e36002 and would pass), and a uniform slowdown of the whole compiler under the machine's own drift.
The median is printed on every run so that the second is at least visible.

## What this does not do, and the gap it found

**It does not benchmark the emitted code.** That is the second half of the emission rule and stays a
person's measurement against hand-written code. It does gate what the COMPILER costs, above.

**It cannot say which changed programs a benchmark covers.** The generated files the gauntlet benchmarks
— `gauntlet/go/gen_*.go`, `gauntlet/js/gen_*.mjs`, `gauntlet/java/gen/*.java` — record nowhere which
source, target and flags produced them. A few result documents show the command, and most files have
none. So a changed example cannot be traced to a benchmark mechanically, and a benchmark file can be
stale against the compiler without anything noticing. gauntlet-2026-09-07 found three of them still on
the retired portable layer for exactly this reason. **Owed:** a generated benchmark file names its
source, target and flags in its header, so the check can regenerate it and report a change there too.

## Files

| file | what |
|---|---|
| `testdata/emitted/` | the baseline's emitted code, one file per source and target — under `testdata/` so the go tool never compiles it as a package |
| `outcomes.txt` | every source × target outcome, hash and compiler output |
| `compiletime.txt` | every source × target compile time in the sweep, the minimum of two sweeps, in milliseconds |
| `ACCEPTED.md` | every accepted change, with its reason |
| `../../.gitattributes` | keeps git from converting this directory's line endings |
