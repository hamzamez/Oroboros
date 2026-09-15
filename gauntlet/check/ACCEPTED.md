# Accepted emission changes

Every change to the baseline in this directory, with its reason. Written by `go run ./cmd/check -accept`; the rule is in [README.md](README.md).

## 2026-09-15 — on bdb92a9, with uncommitted changes

**Reason:** initial baseline: the first run of cmd/check, taken at bdb92a9 plus cmd/check itself; no compiler change

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

The initial baseline.
