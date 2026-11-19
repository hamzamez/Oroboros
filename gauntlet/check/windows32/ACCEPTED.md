# Accepted emission changes

Every change to the baseline in this directory, with its reason. Written by `go run ./cmd/check -accept`; the rule is in [README.md](README.md).

## 2026-11-19 — on 1b73f88e, with uncommitted changes

**Reason:** windows32's initial baseline: cmd/check -tier2, the second tier (hamza 2026-11-19)

177 runs: 118 emitted, 59 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

The initial baseline.

The initial compile-time baseline, the minimum of two sweeps.
