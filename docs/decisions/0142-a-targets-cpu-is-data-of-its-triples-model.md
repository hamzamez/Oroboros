# 0142 — A target's CPU is data of its triple's model, printed as the functions' `target-cpu`

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-15, kernels-research.md §6 Q2: "a target may name its CPU and
features, and that is a feature, not a defect … the choice is the target's data, as the triple is.
Baseline stays the default"). Built 2026-11-19 ([cpu](../../gauntlet/results/cpu-2026-11-19.md)).

## Context

A triple is a model: its data model, its address space in bytes, its ISA and its toolchain (ADR
0122). The ISA is an x86-64 level, and a level is an extension of the ISA's operations: x86-64-v3
adds AVX2, BMI1, BMI2, FMA and MOVBE to the baseline's SSE2 (the x86-64 psABI's levels, 2020). Q2
decided that a target may name more of the machine, trading portability for speed. It waited on a
program a measurement shows it pays for.

ADR 0085's three data-parallel forms already vectorize through LLVM's own vectorizers at the
baseline: a fold by `+`, a rule, and a loop of stores, each two i64 lanes of SSE2. An integer dot
product of 65,536 terms, taken 2,000 times, runs in 67.1 ms at the baseline and in 40.3 ms with the
same IR compiled for x86-64-v3 (cpu-2026-11-19). That is the measurement.

## Decision

1. **A target file may state `cpu "NAME"`**, NAME an LLVM CPU (`x86-64-v3`). It is the triple's
   model's data, as `triple` is: combined by agreement, never a default.
2. **The printer writes it as LLVM's own `"target-cpu"="NAME"` on every function's attributes.**
   The IR's text then says it, and the module is the whole input of the toolchain, as for the
   triple (ADR 0093 item 5). No flag on the build line.
3. **A CPU level is a member of the family** (ADR 0122's fibration): `windows-v3` glues `windows`
   and states `cpu "x86-64-v3"`. Every fact, realization and header of `windows` holds of it,
   since the level only adds operations.
4. **Baseline stays the default.** No existing target states a CPU, so nothing they emit changes.
   A program built for `windows-v3` runs only on a machine with that level. That is the portability
   Q2 trades, and the target's name says it.

## Why not

- **`-march` on the build line**: the `.ll` would not say what it is compiled for. `cmd/check`
  reprints and compares the IR's text, and a property of the code that lives outside it is a path
  nothing checks.
- **The level on the existing member** (`windows` at x86-64-v3): it changes every program's
  emission and every binary's portability, against Q2's "baseline stays the default".
- **A list of features** (`+avx2,+bmi2`) in place of a CPU: a level is the named, documented
  set; a feature list is a subset nothing names, and LLVM's `target-cpu` already implies its
  features. A feature list can be added beside `cpu` when a program needs one feature alone (ADX
  for the adder, kernels-research R2).
- **Explicit vector code in the language** (ADR 0085 item 4): LLVM vectorizes these forms
  itself. The level gives it wider registers, and the language says nothing about either.

## Consequences

- `windows-v3` exists; `cmd/build -target=windows-v3` builds for it.
- The vector width is not the only lever: the tables hold 0..65535 in i64 elements, so narrower
  elements would multiply the lanes again (ADR 0127 item 5's question, waiting on its trigger).
