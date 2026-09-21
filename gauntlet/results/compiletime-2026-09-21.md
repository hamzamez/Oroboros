# Compile time is gated against the baseline

2026-09-21. `cmd/check/compiletime.go`, `cmd/check/emission.go`, `cmd/check/main.go`, the rule in
[gauntlet/check/README.md](../check/README.md). Plan item 2 of
[assessment-2026-09-17](../../docs/assessment-2026-09-17.md), which found the tokeniser compiling
6.6× slower than a round earlier with nothing having noticed.

```
compile time 1.05x the baseline (median of 12 compiles over 300 ms)
```

is now part of every emission verdict, and a compile at **1.5× its baseline and 250 ms slower** is a
change for review.

## 1. The question that started it, answered first

hamza asked whether something was wrong, because the full check had passed ten minutes. Measuring the
noise for this gate turned up what looked like a new regression: the tokeniser at 1.7–2.0 s and freq
at 11.5 s, against the assessment's 777 ms and 7.0 s.

**It was not one.** Measured the assessment's way — serially, warm, the best of two runs — with a
binary built at the assessment's commit and one built today, on the same machine within the hour:

| | `c2cc351` (assessment) | HEAD |
|---|---:|---:|
| tokenize, go | 699 ms | 712 ms |
| jsonfmt, go | 160 ms | 159 ms |
| freq, go | 8.8 s | 8.8 s |
| freq, java | 21.9 s | 22.0 s |

Two things came out of that, and both shaped the gate:

- **the sweep inflates a compile unevenly.** Sixteen compilations at once on a hybrid P/E-core laptop
  put the tokeniser at about 2.5× its serial time. The assessment had already caught
  loopsum-2026-09-16 hiding a 12× jump inside the sweep's *total*; this is the same trap met from the
  other side, and it is now in CLAUDE.md's list of benchmark-method errors;
- **the machine drifts.** The same `c2cc351` binary gave 7.0 s for freq on 2026-09-17 and 8.8 s today:
  26%, with no code changed.

## 2. What is measured, and why it can be gated at all

The **CPU time — user plus system — of each `gen` process in the emission sweep**. Not a serial time,
but a *repeatable* one, because the job order is fixed and so is the contention it meets. Three sweeps
of one compiler:

| compiles by CPU time | n | max/min between sweeps: median | p95 | max |
|---|---:|---:|---:|---:|
| under 100 ms | 441 | 1.00 | 3.07 | 4.13 |
| 100–300 ms | 5 | 1.34 | 1.67 | 1.74 |
| 300 ms – 1 s | 4 | 1.18 | 1.19 | 1.21 |
| over 1 s | 6 | 1.05 | 1.11 | 1.18 |

and the sweep's total moved 0.6% (102.4, 103.0, 102.5 s). The first row is Windows' 15.6 ms CPU-time
tick: a 16 ms compile against a 47 ms one is one tick against three. Nothing that small can be judged
by a ratio.

**Why CPU time and not wall time**, measured rather than argued. The initial baseline was recorded
while the machine was in a fast state — the whole emission step took 28 s, against 51 s for the same
step hours earlier. Scored against that baseline, the three earlier sweeps give a median ratio of
**1.03–1.09 and flag nothing**. The step's wall time moved 1.8×; the per-compile CPU time moved under
10%. Wall time would have made the gate a report on the machine.

## 3. The rule, derived from those numbers

An observation is `t·m·(1+ε)`: `m` the machine that day, `ε ≥ 0` because interference only ever
adds time. Under positive noise the minimum over repeated observations is the estimator (Chen &
Revels, *Robust benchmarking in noisy environments*, 2016). So:

> **Slower** ⇔ `now ≥ 1.5 · base` **and** `now − base ≥ 250 ms`.

- **1.5** is above the worst variation measured for a compile over 300 ms (1.21×) and below the
  smallest regression the gate exists for (freq, 1.67×).
- **250 ms** is above what the tick does to a small program.
- **There is deliberately no floor on the baseline.** It was the obvious design, and it would have
  missed the tokeniser, whose baseline in the sweep was 234 ms when it went to 2,109. The delta is the
  floor, and it is a floor on the *change*.
- **The baseline is the minimum of two sweeps**, because a baseline that happened to be slow hides a
  regression — at 1.21× noise, a single slow baseline would let 1.67× through a 1.5 rule.
- **A suspect explains itself before it is reported**: the sweep runs again, the rule is applied to the
  per-compile minimum. A clean run pays nothing. The second sweep also compares the two sweeps'
  emission — the first place the check runs the emitter twice, which CLAUDE.md has asked for since the
  map-order bug.
- **The median ratio** over compiles above 300 ms is printed on every run. One regression moves a few
  compiles; the machine moves all of them. At 1.3× or more the verdict says the machine is slower than
  when the baseline was taken.

## 4. The witnesses, on real sweeps

The new sweep was run at the commits either side of the regression, in worktrees, and the tables are
in `cmd/check/testdata/`:

| compile | `80386e1` | `7e36002` | ratio | flagged |
|---|---:|---:|---:|:---:|
| tokenize, go | 234 ms | 2,109 ms | 9.01× | **yes** |
| freq, js | 6.9 s | 18.2 s | 2.63× | **yes** |
| jsonfmt, go | 187 ms | 515 ms | 2.75× | **yes** |
| jsonfmt, js | 265 ms | 546 ms | 2.06× | **yes** |
| freq, go | 13.4 s | 25.2 s | 1.89× | **yes** |
| jsonfmt, java | 312 ms | 562 ms | 1.80× | **yes** |
| freq, java | 40.8 s | 52.9 s | 1.30× | no |
| big/render, windows | 250 ms | 312 ms | 1.25× | no |
| tree, go | 1.67 s | 1.94 s | 1.16× | no |

**Six flagged, the day it landed.** And across all six orderings of three sweeps of one compiler,
**none**. `cmd/check/compiletime_test.go` pins both, plus the rule's edges, drift against a single
regression, and the minimum — each test shown to fail against a planted bug:

| planted | caught by |
|---|---|
| ratio **or** delta | the historical witness, the identical sweeps, the edges, drift |
| the delta dropped | the historical witness, the identical sweeps, the edges |
| a floor on the baseline | the historical witness and the edges — this is the plant that matters |
| the minimum taken as a maximum | the per-compile minimum |

## 5. What it cost

| | |
|---|---|
| `compiletime.go` | the rule, the minimum, the median, the confirmation sweep — 260 lines with comments |
| `emission.go` | each compile's CPU and wall time; the verdict names changed outputs and slower compiles together |
| `main.go` | `-accept` records the time baseline and logs every accepted slower compile |
| the check's time | **nothing on a clean run**; one more sweep (~50 s) when a suspect appears, and on every `-accept` |

The last row is deliberate, and it answers the question in §1: the check is already about eleven
minutes, most of it in the two steps that build and run host programs, and a gate that added a minute
to every run would have made that worse to fix something that happens rarely.

## 6. Not fixed, and not built

- **The regression itself.** The baseline records today's costs, so the tokeniser's ~700 ms and freq's
  8.8 s and 22 s are what "no slower" now means. Recovering the 6.6× is separate work, and this gate is
  what will show it when it lands.
- **Slowdowns under 1.5× on one program**: freq on the JVM moved 1.30× at `7e36002` and passes.
- **A uniform slowdown of the compiler under the machine's own drift** is indistinguishable from the
  machine. The median is printed so that it is at least visible.
- **Serial timing in the check.** It would give the number a person waits for, and it costs a minute
  per heavy program per run; §1's comparison is how to get that number when it is wanted.
