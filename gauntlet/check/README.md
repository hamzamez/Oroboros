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
| `compiler` | `go test ./core/ ./emit/ ./cmd/...` |
| `emission` | every `.oro` under `examples/`, `lib/` and `gauntlet/differential/cases/`, on every target, compared with the baseline in this directory |
| `differential` | `gauntlet/differential/run.go`: build, **run** and agree on four targets |
| `tooling` | `go test ./gauntlet/stdlib/`: the surveys twice, the pins, the acceptance programs, the hand declarations against the host |

**Every step runs**, whatever the ones before it did, so one run reports everything. Exit codes:
`0` all pass and emission is byte-identical, `1` a step failed, `2` emission changed and needs review.
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

## What this does not do, and the gap it found

**It does not benchmark.** Performance is the second half of the rule and stays a person's measurement.

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
| `ACCEPTED.md` | every accepted change, with its reason |
| `../../.gitattributes` | keeps git from converting this directory's line endings |
