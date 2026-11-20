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

## 2026-11-19 — on 895863b7, with uncommitted changes

**Reason:** ADR 0138: a remainder or quotient by a constant of a proven natural is srem/sdiv (point(b)), measured on the corpus first (srem-2026-11-19); lib/rt/i686.ll's one udiv by 2^31 now sdiv, its i686 assembly identical; ADR 0137's typing marks change no outcome

178 runs: 118 emitted, 60 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

34 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows32`
- emitted text changed — `examples/big/fact.oro windows32`
- emitted text changed — `examples/big/fib.oro windows32`
- emitted text changed — `examples/big/limbs.oro windows32`
- emitted text changed — `examples/big/pair.oro windows32`
- emitted text changed — `examples/big/power.oro windows32`
- emitted text changed — `examples/errors/expect.oro windows32`
- emitted text changed — `examples/errors/slots.oro windows32`
- emitted text changed — `examples/hash/crc32.oro windows32`
- emitted text changed — `examples/hash/fnv.oro windows32`
- emitted text changed — `examples/io/cmp.oro windows32`
- emitted text changed — `examples/io/count.oro windows32`
- emitted text changed — `examples/io/freq.oro windows32`
- emitted text changed — `examples/io/jsonfmt.oro windows32`
- emitted text changed — `examples/io/roundtrip.oro windows32`
- emitted text changed — `examples/io/wc.oro windows32`
- emitted text changed — `examples/native/shortcircuit-win.oro windows32`
- emitted text changed — `examples/native/wndproc-win.oro windows32`
- emitted text changed — `examples/num/ledger.oro windows32`
- emitted text changed — `examples/table/sieve-win.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-negmid.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-sign.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-signed-ops.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro windows32`
- emitted text changed — `gauntlet/differential/cases/bytes-table.oro windows32`
- emitted text changed — `gauntlet/differential/cases/codecs.oro windows32`
- emitted text changed — `gauntlet/differential/cases/fnv32.oro windows32`
- emitted text changed — `gauntlet/differential/cases/fnv64-fold.oro windows32`
- emitted text changed — `gauntlet/differential/cases/fnv64.oro windows32`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro windows32`
- emitted text changed — `gauntlet/differential/cases/quotient-corner.oro windows32`
- emitted text changed — `gauntlet/differential/cases/render.oro windows32`
- emitted text changed — `lib/rt/i686.oro windows32`

## 2026-11-19 — on 895863b7, with uncommitted changes

**Reason:** ADR 0138 by the corpus's measurement: a power-of-two divisor keeps urem/udiv (srem by 2^k was 1.13x on the limbs' factorial, srem-2026-11-19); a constant's typing claim decided (ADR 0137)

178 runs: 118 emitted, 60 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

26 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows32`
- emitted text changed — `examples/big/fact.oro windows32`
- emitted text changed — `examples/big/fib.oro windows32`
- emitted text changed — `examples/big/limbs.oro windows32`
- emitted text changed — `examples/big/pair.oro windows32`
- emitted text changed — `examples/big/power.oro windows32`
- emitted text changed — `examples/errors/expect.oro windows32`
- emitted text changed — `examples/errors/slots.oro windows32`
- emitted text changed — `examples/hash/crc32.oro windows32`
- emitted text changed — `examples/hash/fnv.oro windows32`
- emitted text changed — `examples/io/cmp.oro windows32`
- emitted text changed — `examples/io/count.oro windows32`
- emitted text changed — `examples/io/freq.oro windows32`
- emitted text changed — `examples/io/jsonfmt.oro windows32`
- emitted text changed — `examples/io/roundtrip.oro windows32`
- emitted text changed — `examples/io/wc.oro windows32`
- emitted text changed — `examples/num/ledger.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-negmid.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-sign.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-signed-ops.oro windows32`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro windows32`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro windows32`
- emitted text changed — `gauntlet/differential/cases/quotient-corner.oro windows32`
- emitted text changed — `gauntlet/differential/cases/render.oro windows32`
- emitted text changed — `lib/rt/i686.oro windows32`

## 2026-11-19 — on da1c4134, with uncommitted changes

**Reason:** ADR 0139: an again after a join point accepted, read as a let of several names (joinlet-2026-11-19), examples/os/wc-nested.oro new; ADR 0140's primed result, emission identical

179 runs: 118 emitted, 61 refused; 3057 of 3067 integer operations bounded, 417 of 433 loops proven. compiler pass, differential-windows32 pass, tooling pass.

1 change(s):

- new source — `examples/os/wc-nested.oro windows32`

## 2026-11-19 — on 8f8dcbe9, with uncommitted changes

**Reason:** ADR 0141 (ADR 0084's P5): SendMessageA's members, a C call's constant; examples/native/status-win.oro new (family-2026-11-19)

180 runs: 119 emitted, 61 refused; 3086 of 3096 integer operations bounded, 426 of 442 loops proven. compiler pass, differential-windows32 pass, tooling pass.

1 change(s):

- new source — `examples/native/status-win.oro windows32`

## 2026-11-19 — on 376400a1, with uncommitted changes

**Reason:** ADR 0082 above the word (bigbits-2026-11-19): stein, popcount and three cases new; power and status-win gain nuw/samesign from guardDiff, more proven

185 runs: 124 emitted, 61 refused; 6663 of 6673 integer operations bounded, 735 of 814 loops proven. compiler pass, differential-windows32 pass, tooling pass.

8 change(s):

- new source — `examples/big/popcount.oro windows32`
- emitted text changed — `examples/big/power.oro windows32`
- new source — `examples/big/stein.oro windows32`
- emitted text changed — `examples/native/status-win.oro windows32`
- compiler output changed — `examples/native/status-win.oro windows32`
- new source — `gauntlet/differential/cases/big-bits.oro windows32`
- new source — `gauntlet/differential/cases/big-popcount.oro windows32`
- new source — `gauntlet/differential/cases/big-stein.oro windows32`

## 2026-11-19 — on 51abeda8, with uncommitted changes

**Reason:** ADR 0142 (cpu-2026-11-19): examples/table/idot.oro new, the integer dot product; no existing target states a CPU

186 runs: 125 emitted, 61 refused; 6672 of 6682 integer operations bounded, 739 of 818 loops proven. compiler pass, differential-windows32 pass, tooling pass.

1 change(s):

- new source — `examples/table/idot.oro windows32`

1 compile(s) recorded FASTER:

- `examples/big/stein.oro windows32` 115578 → 73906 ms, 0.64x

## 2026-11-20 — on 047c5a7e, with uncommitted changes

**Reason:** callbacks tier 1 on windows (ADR 0143): examples/callbacks/allwindows.oro new, emitted on windows and windows32, refused on go, js and java; the callbacks programs new on windows32

189 runs: 126 emitted, 63 refused; 6685 of 6695 integer operations bounded, 741 of 820 loops proven. compiler pass, differential-windows32 pass, tooling pass.

3 change(s):

- new source — `examples/callbacks/allwindows.oro windows32`
- new source — `examples/callbacks/caesar.oro windows32`
- new source — `examples/callbacks/prefix.oro windows32`

## 2026-11-20 — on 7e01ed51, with uncommitted changes

**Reason:** callbacks tier 1 on JavaScript and the JVM (ADR 0143): examples/callbacks/quantifiers.oro (js) and semilattice.oro (java) new

191 runs: 126 emitted, 65 refused; 6685 of 6695 integer operations bounded, 741 of 820 loops proven. compiler pass, differential-windows32 pass, tooling pass.

2 change(s):

- new source — `examples/callbacks/quantifiers.oro windows32`
- new source — `examples/callbacks/semilattice.oro windows32`
