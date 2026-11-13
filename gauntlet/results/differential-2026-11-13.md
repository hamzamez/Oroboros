# The check in 7 minutes: the differential in parallel, and the steps at once

Date: 2026-11-13 (hamza: "go ahead with 1 and 2 now", after asking whether 50 minutes for a full check
was too long). **A full `go run ./cmd/check` took 49m 49s on 2026-11-13 and takes 7m 5s now**, every
verdict the same: the differential's output byte-identical to its serial run, the emission
byte-identical to its baseline. No compiler code changed.

## What was measured first

The check run that accepted ADR 0108 (carry-2026-11-13), step by step, one after another:

| step | time | where it goes |
|---|---|---|
| emission | 19m 13s | the sweep (632 compiles, 187 s of CPU at 16 jobs), a second sweep, then 50 suspects each paired serially, 3 rounds × 2 compiles at GOMAXPROCS=1 |
| differential | 16m 44s | 71 cases, about 300 builds, **one at a time**, each `go run ./cmd/build` (a link), then the run: about 3.3 s each |
| compiler | 8m 19s | `go test` runs packages in parallel; `ir` is the critical path, 242 s of serial tests, 11 of them over 8 s |
| tooling | 5m 13s | |
| vet, gauntlet, ir | 23 s | |

The steps ran in sequence, and only the emission step's measurement needs the machine alone: its
gate is CPU time, which a neighbour's load moves (compiletime-2026-09-25). Every other step is a
verdict that load cannot change.

## What was built

- **The differential in parallel** (`gauntlet/differential/run.go`):
  - `cmd/build` is built once into the work directory;
  - the per-case body is `runCase`, writing its report to a buffer;
  - `-j N` cases run at once, half the cores by default;
  - each report is printed in the cases' order as soon as every case before it is done.

  A case builds only into its own directories, so its verdict depends on nothing but its source.
  `-cases DIR` runs another directory.
- **The steps at once** (`cmd/check/main.go`):
  - emission first and alone;
  - then vet, compiler, gauntlet, ir, differential and tooling concurrently, each reported as it
    finishes;
  - a failing command's log tail is carried in its result, so it prints under its own header;
  - the compiler step's `go test` gets `-timeout 40m`, since a package's default 10 minutes is too
    near `ir`'s 8 minutes under load.
- **The witness** (`cmd/check/differential_test.go`): three cases at `-j 4`:
  - one right;
  - one whose targets all agree on an answer it does not expect;
  - one with no `// expect:`.

  The run must fail, name exactly the two, and report `2 case(s) failed`, in the cases' order. **The
  planted fault**, the wrong-answer path counted as a pass, makes the test fail ("want `2 case(s)
  failed`"). Restored, it passes.

## What failed first

- **The wrong-answer path kept a `continue`** when the loop body became a function. It sat inside two
  `if`s, and the scripted rewrite turned only the top-level ones into `return false`, so `go vet`
  refused it ("continue not in for statement"). That is the path the witness plants, so the witness
  covers the rewrite's riskiest line.

## Measured

The differential alone, after the change (its cases unchanged):

| workers | time |
|---|---|
| 1, `go run` per build (before) | 16m 44s |
| 4 | 112 s |
| 8 (default) | 81 s |
| 16 | 75 s |

The gain is more than the workers'. About 300 links of `cmd/build` are gone, and the go, java and
windows builds overlap their own toolchains' serial phases.

The whole check, after the change:

| step | before (serial) | after |
|---|---|---|
| emission, alone | 19m 13s | 1m 4s |
| compiler | 8m 19s | 5m 57s, at once |
| differential | 16m 44s | 1m 46s, at once |
| tooling | 5m 13s | 2m 37s, at once |
| vet, gauntlet, ir | 23 s | 31 s, at once |
| **total** | **49m 49s** | **7m 5s** |

- **The emission step's 19 → 1 minutes is not this change.** This run's gate paired 2 suspects
  against the last run's 50: the compile-time baseline had just been re-recorded, and the machine was
  quieter (compile time 0.85× the baseline). The step is 1 to 19 minutes by its suspect count, so a
  check with many suspects is still about 25 minutes. That is the next lever, item 4 of the proposal:
  log the emission step's phases first.
- The steps beside it now bound the check by the compiler's 6 minutes, whose critical path is `ir`'s
  serial tests (item 3).
- `fib` and `pair` on windows "compile faster" than the baseline (4,515 → 1,781 ms, 5,625 →
  2,500): that baseline is the slow sweep the last accept recorded, so the note is the machine, not
  the code. It is a note, not a failure.

## What was not built

- **The `ir` tests in parallel** (item 3): many tests set and reset the `Plant*` switches, and a test
  planting a fault must not run beside one trusting the clean rule. Only tests that touch no switch can
  be parallel, a list to derive from the code.
- **Fewer or cheaper pairs in the emission step** (item 4): first its phases timed (sweep, second
  sweep, pairs), so the 1-to-19-minute range is explained by a measurement, not by this estimate.
- **A planted disagreement between targets**: the disagreement path reports through the same
  `return false` the witness's wrong answer does. A case whose targets differ needs a target-specific
  bug to plant, and no fault switch makes one.
