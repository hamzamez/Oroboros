# The check in 4½ minutes: the differential in parallel, the steps at once, the ir tests in parallel

Date: 2026-11-13 (hamza: "go ahead with 1 and 2 now", after asking whether 50 minutes for a full check
was too long; then "go ahead with 3 and 4 too"). **A full `go run ./cmd/check` took 49m 49s on
2026-11-13 and takes 4m 25s now**, every verdict the same: the differential's output byte-identical
to its serial run, the emission byte-identical to its baseline. No compiler code changed.

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

## Items 3 and 4

**3. The `ir` tests in parallel.** Go runs a test marked `t.Parallel()` only beside other parallel
tests, after every serial test's body has returned. So a test may be parallel exactly when it writes
no state another test reads, and the serial tests that plant faults never overlap one. The 14 tests
over a second (203 of the package's 242 serial seconds) were checked for that, since the race
detector needs cgo and this machine has no C compiler:
- by a script over the test files, transitively through their helpers, for any write to a
  package-level variable, the `Plant*` switches included;
- by hand, for writes through a pointer (`*p = true` over `&PlantX`), which the script cannot see:
  these appear only in `localtab`, `release` and `reuse` tests, which stay serial;
- on the compiler's side, the package-level variables written at run time: `callees` and `calleeEr`
  (under `calleeMu`), three `sync.Map`s, and the printers' import sets, which no `ir` test reaches.
  Every test loads its own target, and `emit` caches none.

The package then took 56 s alone (about 250 s before), and passed three times running (`-count=3`,
235 s). Under the full check's load it is 199 s, and the compiler step 3m 24s (5m 57s before).
`ir/interval.go`'s comment alignment, which `gofmt -l` listed at HEAD, was fixed in passing.

**4. The emission step's phases**, now printed in its note every run ("the step's phases: …"):

| run | sweep | second sweep | baseline `gen` built | pairs |
|---|---|---|---|---|
| this check | 21 s | 22 s | 8 s | 2 pairs, 14 s |
| against e173ef0's times (a throwaway branch), quiet | 29 s | 18 s | 8 s | 4 pairs, 27 s |
| the 19-minute run (carry-2026-11-13), by estimate | about 1 min | about 1 min | | 50 pairs, about 15 min |

A pair costs about 7 s on a quiet machine. So the emission step is about one minute plus 7 s a
suspect. The past accept runs in ACCEPTED.md paired 1, 1, 3, 3 and 5 suspects; the 50 is the outlier.
Against the same stale baseline, a quiet machine finds 4 suspects, so the outlier needed a noisy
machine as well as a stale baseline. The stale baseline was this session's: at class-2026-11-12,
`compiletime.txt` was restored rather than recorded from a slow sweep. That avoided writing noise
into the baseline, but left it a compiler behind. Re-recording it with `-accept "why" -retime` on a quiet machine
is the remedy, and is now said in gauntlet/check/README.md.

**No sound way to cut the pairs was found.** Each pair's verdict is min(now)/min(base) over its
rounds. Under the gate's noise model (an observation is t·m·(1 + ε) with ε ≥ 0, unbounded), a later
round can lower either minimum by any amount. So no round can be skipped in either direction: a
suspect cleared after one round could still be confirmed after three. Pairs running at once, each
pinned to one of the four P-cores, would share caches and turbo frequency. That is the contention
compiletime-2026-09-25 found moves a compile by 1.6×, and would need its own validation against
identical binaries. Deliberately not built: the cost it removes appears only on a noisy day.

## The check, step by step, at the end

| step | before (serial) | 1 and 2 | **1–4** |
|---|---|---|---|
| emission, alone | 19m 13s | 1m 4s | 57 s |
| compiler | 8m 19s | 5m 57s | **3m 24s** |
| differential | 16m 44s | 1m 46s | 2m 16s |
| tooling | 5m 13s | 2m 37s | 3m 9s |
| vet, gauntlet, ir | 23 s | 31 s | 32 s |
| **total** | **49m 49s** | **7m 5s** | **4m 25s** |

The steps beside the emission step now finish within 80 s of one another, all CPU-bound on 16
threads. A shorter check now needs less work, not more parallelism. `cmd/build`'s tests (192 s) and
`emit`'s (188 s) are the next critical paths. `cmd/build`'s build programs in-process through the
printers' global import sets (`emit.LLVMImports`, which the adder's witness reads), so they cannot be
parallel until that state is per build.

## What was not built

- **Concurrent pairs** in the emission step, above.
- **`cmd/build`'s and `emit`'s tests in parallel**: the printers' import sets are global, and the
  tests that plant faults would need the same analysis as `ir`'s.
- **A planted disagreement between targets**: the disagreement path reports through the same
  `return false` the witness's wrong answer does. A case whose targets differ needs a target-specific
  bug to plant, and no fault switch makes one.
