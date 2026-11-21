# The check, shortened: the compiler step a schedule, an accept a reused verdict

Date: 2026-11-21. CLAUDE.md plan item 0 (hamza, 2026-11-20: "agree, put it in the plan"; 2026-11-21:
"before that, I think we have an item in the plan to improve the time of the full check. so that
first").

## The law

**The compiler step is makespan scheduling, P‖Cmax.** Its 926 top-level tests are jobs of measured
weight wᵢ (1,941 s in all, run alone, the heaviest `TestPrintIntIsTotalOnTheIntegers` at 144 s); m
processes run at once. A test is indivisible and the work is conserved, so no schedule finishes before

    τ = max(maxᵢ wᵢ, Σᵢ wᵢ / m)        (m = 8: max(144, 243) = 243 s)

P‖Cmax is NP-hard; Graham's LPT rule (1969), the heaviest job first, each to the machine then lightest,
is within (4/3 − 1/(3m)) of the optimum. A machine is a `go test -run '^(…)$' PKG` process, so a group
holds one package's tests: a package of weight Wₚ is split into ⌈Wₚ/τ⌉ groups by LPT (cmd/build's
1,227 s into 6, ir's 440 s into 2), and the lighter packages share one `go test`, which runs them in
parallel itself. Ten processes; each its own fault switches, so no planted fault leaks between groups.

**An accept is a function of the tree.** The check reads the tree's files and the toolchain; the
emitter is a function of its input, which the check verifies; every other step's verdict no rerun on
the same input changes. So a run's verdicts are every run's on that input, recorded under the hash of
what it read (SHA-256 over every file git would track, `go version`, `-tier2`), and an `-accept` of the
same hash reruns only the emission step, whose output it writes, requiring that output to be the
recorded one, outcome for outcome.

## What was built

1. **`cmd/check/groups.go`**: `go test -list` per package, the plan by τ and LPT from
   `gauntlet/check/testtimes.txt` (a test the table does not know weighs its median), every group at
   once with `-json`, the log reassembled as `-v` prints it, each group's wall time, and the partition
   checked: every listed test must report pass, fail or skip, or the step fails naming it. `-testprocs`
   is m (half the cores); `-accept -retime` re-records the weights from the run.
2. **`cmd/check/verdict.go`**: the input's hash, taken when a run begins and when it ends (a tree edited
   during the run gets no record), `.check/verdict.json` with every step's status and each emission's
   digest; an `-accept` whose hash is the record's, its compiler and differential passing and its
   tooling not failed, runs the emission step alone and takes every other verdict from the record.
3. **Tests**, each with its planted fault failing first: `TestLPTIsAPartitionWithinGrahamsBound` (a
   partition, and the makespan within (4/3 − 1/3m) of the lower bound, m = 1…5);
   `TestTheSplitCompilerStepRunsEveryTest`, end to end on a module of three tests weighed to need three
   processes (`PlantGroupDropsATest` leaves one out, and the step fails naming it);
   `TestAVerdictIsReusedOnlyForItsTree` (the hash changes with a tracked file, an untracked one and
   `-tier2`, not with an ignored one; `PlantVerdictAnyTree` reuses another tree's record;
   `PlantEmissionDigestUnchecked` takes any emission).

## Measured

| | before | after |
|---|---|---|
| the full check, wall | 62 min (2026-11-20); 30–37 min in this session's runs, the compiler step its pole | **13.8–15.4 min** (827, 895, 926 s over three runs) |
| the compiler step | 23–32 min, one process per package | 11m43s–14m13s, 10 processes, the longest group cmd/build's 14 tests at 11–12 min |
| an accept after a review | a second full run | **165 s**: the emission step alone (two sweeps, its compile-time gate), every other verdict reused |

A step that changes emission paid two full runs, about two hours on 2026-11-20; it now pays a run and
an accept, about 16½ minutes.

With the compiler step split, the steps finish together: compiler 14m13s, book 14m9s, differential
9m30s, tooling 8m24s. The planned 243 s became 14 minutes because every step runs at once on 16 logical
cores: the bound is now the machine's total CPU, Σ of every step's work over the cores, and no schedule
of one step moves it. Run alone the compiler packages took 20½ minutes of wall (cmd/build 1,227 s).

## What failed first

- **The plant could not fail where every group held one test**: it dropped a test only from a group of
  two or more, and the end-to-end module's three tests on three processes were one each. It now drops
  from the first non-empty group, removing a group left empty.
- **`verdict` was taken**: `paired_test.go` has a helper of that name; the record is `runRecord`.
- **The first reuse trial reran every step**, correctly: I edited this document while the review run
  ran, so the tree it ended on was not the tree it began on, and no record was written. Run again on an
  untouched tree, the accept reused it.
- **Heredocs ate `\n` three times** in Go string literals written through Python (the memory's rule:
  write such text with the Edit tool).

## Paid and bought

| | |
|---|---|
| check time | 62 → 13.8–15.4 min for a full run; an accept after a passing review 165 s, where it was a second full run |
| compile time, run time, code size | none: no compiler or emitted text changed |
| proofs, refusals | none changed |
| check lines | +646 −6 in `cmd/check` (`groups.go` 414, `verdict.go` 172, `main.go` +60 −6), tests +167 in `split_test.go` |
| documents | `gauntlet/check/README.md` (the schedule, the reuse), `testtimes.txt` (926 weights), CLAUDE.md, this |
| rule made less general | none: every test runs, and an accept still requires its compiler and differential steps to have passed on this tree |
| not built | the other steps' own splitting (book 14 min and differential 9½ under load are the next poles); a weight table that adapts without `-retime`; scheduling across steps, which share one machine |
