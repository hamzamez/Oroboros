# Accepted emission changes

Every change to the baseline in this directory, with its reason. Written by `go run ./cmd/check -accept`; the rule is in [README.md](README.md).

## 2026-11-19 — on 1b73f88e, with uncommitted changes

**Reason:** windows32's initial baseline: cmd/check -tier2, the second tier (hamza 2026-11-19)

177 runs: 118 emitted, 59 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

The initial baseline.

The initial compile-time baseline, the minimum of two sweeps.

## 2026-11-19 — on 5858116c, with uncommitted changes

**Reason:** print_int stops past 20 digits where it printed the low 20 (a wrong answer, print-2026-11-19); num/u128 in ℤ pinned as refused (examples/u128/inz.oro); a closed ≠ decided in ℤ (u128-2026-11-19)

178 runs: 118 emitted, 60 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

8 change(s):

- emitted text changed — `examples/io/cmp.oro windows32`
- emitted text changed — `examples/io/count.oro windows32`
- emitted text changed — `examples/io/roundtrip.oro windows32`
- emitted text changed — `examples/io/wc.oro windows32`
- emitted text changed — `examples/native/shortcircuit-win.oro windows32`
- emitted text changed — `examples/native/wndproc-win.oro windows32`
- emitted text changed — `examples/table/sieve-win.oro windows32`
- new source — `examples/u128/inz.oro windows32`
