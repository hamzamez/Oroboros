# The compile-time gate pairs a suspect against the baseline's binary

2026-09-25. The gate of [compiletime-2026-09-21](compiletime-2026-09-21.md) false-alarmed on `freq`,
the largest program, twice in two days:
- hoststring-2026-09-24: `freq`/js at 1.58×, and a rerun passed;
- names-2026-09-25 §5: `freq` at 1.63× on Go and on the JVM, in two consecutive sweeps.

Each time, a serial measurement of the two binaries was identical, and the baseline was
deliberately not moved. Rule: [gauntlet/check/README.md](../check/README.md), "The rule for compile
time".

## 1. What was measured

All figures are CPU time (user + system) of one `gen` process, with wall time in parentheses where it
matters. The machine is an i5-12500H: 4 P-cores (8 threads) and 8 E-cores. A replica of the sweep
(same `gen`, same job order) varied one thing at a time.

**Quiet sweeps do not reproduce it.**

| run | `freq` Go | `freq` JS | `freq` JVM |
|---|---:|---:|---:|
| replica, 16 jobs, 9 sweeps | 17.4–19.9 s | 11.9–13.4 s | 42.9–47.2 s |
| the real check's emission step, 3 runs | 17.8–18.9 s | 12.5–13.5 s | 45.4–48.4 s |
| replica straight after the 40 s compiler tests (heat) | 17.4–19.7 s | 12.2–12.5 s | 42.9–44.1 s |
| replica, 8 jobs / 4 jobs | 21.1 / 19.2 s | 15.0 / 12.4 s | 49.4 / 48.1 s |
| **the flagged run** (names §5) | **27.3 s (18.5)** | **19.6 s (12.9)** | **71.4 s (54.6)** |
| baseline (`compiletime.txt`) | 16.7 s | 13.0 s | 43.4 s |

So neither the parallelism level nor heat drives it. The flagged run's own record
(`.check/compiletime.txt`) shows its **wall** time doubled as well: 18.5 s against 9.6 s. Its emission
step took 1m50s where two sweeps take about 45 s. The whole machine was loaded by something outside
the check.

**Two mechanisms move the CPU time of an unchanged compile:**

| `freq`, serial, one binary | Go | JS | JVM |
|---|---:|---:|---:|
| pinned to P-cores | 11.9–12.1 s | 8.2–8.3 s | 33.4–34.5 s |
| pinned to E-cores | 19.3–19.6 s | 13.5 s | 51.6–51.9 s |
| P-cores, `GOMAXPROCS=1` | 9.6 s | 6.8 s | 23.8 s |
| E-cores, `GOMAXPROCS=1` | 16.5 s | 11.7 s | 42.5 s |

1. **Core type.** An E-core costs 1.6–1.7× the CPU for the same work. Which core a long compile gets
   depends on the rest of the machine's load.
2. **Go's collector.** It runs idle mark workers on every idle P, so a GC-heavy compile's CPU time
   grows with the number of idle cores. In the replica sweep:
   - `GOMAXPROCS=1` gave 12.3 / 9.3 / 28.2 s, against 17–19 / 12–13 / 43–47 s at 16;
   - `GOGC=200` gave 12.0 / 8.0 / 25.7 s;
   - `GOGC=30` gave 35.4 / 24.7 / 87.9 s.

   About 40% of `freq`'s sweep CPU time is collector work, and part of it is soaked-up idle
   capacity.

**External load moves both.** A CPU burner was run beside the replica:

| load | Go | JS | JVM |
|---|---:|---:|---:|
| 8 busy threads | 18.4 s (14.2) | 13.8 s (10.9) | 54.9 s (30.8) |
| 16 busy threads | 17.2 s (32.9) | 12.2 s (24.9) | 41.7 s (47.8) |
| the whole sweep confined to the E-cores | 20.2 s (16.0) | 14.7 s (12.1) | 53.9 s (41.6) |

Load pushes toward E-cores, which raises CPU time. It also removes the idle cores, which lowers it.
No single factor reproduced 1.63×; the flagged afternoon combined them under a neighbour I cannot
identify after the fact. **What matters for the gate is that the sweep's CPU time is a function of
the machine's state, and the baseline was recorded in another state.** Two sweeps in a row sample the
same state, which is why the existing confirmation passed the false alarm through.

## 2. What is built

**A suspect both sweeps keep is paired against the baseline's own binary** (`cmd/check/paired.go`).

1. **The baseline's binary** is `gen` built from the commit that last wrote
   `gauntlet/check/compiletime.txt`, the commit whose compiler those numbers are of. It is built in a
   tree extracted with `git archive`, so it reads its own targets and sources, and nothing in `.git`
   changes. No worktree is involved.
2. **The two binaries alternate on each suspect, serially**, for three rounds, with `GOMAXPROCS=1`.
   gen is single-threaded (no goroutine in `core`, `emit` or `cmd/gen`), so one P changes only how
   its collector is scheduled. CPU time becomes the compile's work.
3. **The same rule is applied to the ratio of the two minima**: 1.5× and 250 ms. Whatever the machine
   does, it does to both.
4. **A cleared suspect is not reported, and it keeps its baseline time.** Otherwise `-accept` would
   record the inflated sweep time and widen the gate by exactly the noise it refused to report. A
   confirmed suspect is listed with both figures.
5. **When no pair can be made, the suspect stands, and the note says why**: the baseline file has
   uncommitted changes, or the baseline commit does not build.

**Why a pair, and not the other fixes the task named:**
- **Serial re-timing alone is biased.** `freq` costs 0.68× its sweep time serially, so a serial
  time held against the sweep's baseline would let a 1.47× regression through the 1.5 rule. Serial
  has to be compared with serial, taken in the same minutes.
- **Medians over more sweeps** sample the same afternoon: names-2026-09-25 flagged in two
  consecutive sweeps.
- **`GOMAXPROCS=1` in the sweep** removes the collector term but not the core type. On P-cores it
  gives 9.6 s and on E-cores 16.5 s, which is 1.72×, above the rule. So it cannot make the screen
  decisive. It would also change what the baseline measures and add about 9 s to every check (the
  sweep went 21 → 30 s). It is used where it is free: inside the pair.
- **Pinning to one core class** needs masks for each machine and halves the sweep's throughput.

## 3. Checked, and how each check fails

**Unit tests** (`cmd/check/paired_test.go`) run on a fake machine whose slowness the test controls:

- an unchanged compiler on a machine 1.6× slower all afternoon is cleared, at exactly 1.00×;
- one doubled sample is cleared too;
- a 1.6× regression is kept on a quiet machine, a slow one, and one that alternates P and E call by
  call;
- a machine slowing steadily through the measurement is cleared, because the pair is interleaved;
- an unpaired suspect stands;
- the pair obeys the 250 ms half of the rule.

| planted in `paired.go` | caught by |
|---|---|
| baseline timed first, then this binary (not interleaved) | the drift test |
| each side's maximum, not its minimum | the slow-machine test (its spike case) and the drift test |
| the verdict always clears | the regression test |
| the ratio only, no delta | the same-rule test |
| an unpaired suspect cleared | the unpaired test |

**Live, through `go run ./cmd/check -only emission`:**

- **A slower environment with an unchanged compiler: cleared.** `GOGC=30` was inherited by every
  `gen`, a stand-in for "the machine is slower today" that reaches both binaries equally. Both sweeps
  kept six suspects: `freq` on Go, JS and the JVM, `jsonfmt`/java, `kara/core`/go and
  `render`/windows. The pair cleared all six, at 0.92–1.01×. Result: pass, byte-identical.
- **A genuine regression: confirmed.** The planted fault was a quadratic loop over the source in
  `gen`'s loader, for files over 15 KB (only `freq`, at 19.9 KB), adding about 19 s. On a quiet
  machine:

  | compile | sweep | paired |
  |---|---|---|
  | `freq` go | 16,687 → 37,953 ms, 2.27× | 10,296 → 28,578 ms, 2.78× |
  | `freq` js | 12,953 → 33,078 ms, 2.55× | 7,437 → 25,796 ms, 3.47× |
  | `freq` windows | 0 → 20,843 ms | — |

  Result: REVIEW. `freq`/java read about 1.47× in the sweep and was never a suspect. That is the
  gate's stated resolution (a slowdown under 1.5× on one program passes), not the pair's doing.
- **The full check** `go run ./cmd/check -skip tooling` passes: emission byte-identical, compile time
  1.05× the baseline.

## 4. What failed first

- **The first hypothesis was contention.** The task's framing, and mine, was that 16 parallel
  compiles make `freq` bimodal. Nine quiet sweeps, three real emission steps, 4, 8 and 16 jobs, and a
  hot machine all gave the low state. The bimodality belongs to the machine's load, not to the sweep.
- **The planted regression was too small.** At 60 rounds it added 2.8 s, under 1.5× everywhere, and
  needed 400 rounds.
- **The note contradicted itself.** It read "a second sweep confirmed it … 0 confirmed". It now says
  how many suspects each stage kept.
- **A zero-millisecond baseline printed no paired figure**, because the text was guarded on the
  baseline side only.

## 5. What it cost

- **A clean run: nothing.** The pair runs only on a suspect that both sweeps keep.
- **A run the sweep flags twice** now costs minutes before the verdict: 6m8s for the planted
  regression's three suspects, and 8m8s for the six under `GOGC=30`, whose paired compiles are
  slowed too. That is about `freq`'s three targets × 2 binaries × 3 rounds at single-threaded speed.
  The hand investigation it replaces took longer each time it was done.
- **Code:** `paired.go`, about 250 lines, and its test.
- **Emission and baseline:** both unchanged. `compiletime.txt` did not move.

## 6. Not built

- **`GOMAXPROCS=1` for the sweep itself.** Measured in §1, and deferred for the reasons in §2. It
  becomes worth doing if the screen's false suspects start costing more pair time than 9 s per check.
- **A paired `faster`.** A faster compile is still believed from the sweeps alone. Positive noise
  cannot make a compile quicker than it is, but a baseline recorded on a slow afternoon can make an
  ordinary one look faster. `-accept` records it, and nothing yet pairs it.
- **Pairing during `-accept`'s own sweeps**, so that the recorded baseline is itself immune to a
  loaded machine. Today it is the minimum of two sweeps, as before.
