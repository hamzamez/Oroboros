# Accepted emission changes

Every change to the baseline in this directory, with its reason. Written by `go run ./cmd/check -accept`; the rule is in [README.md](README.md).

## 2026-09-15 — on bdb92a9, with uncommitted changes

**Reason:** initial baseline: the first run of cmd/check, taken at bdb92a9 plus cmd/check itself; no compiler change

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

The initial baseline.

## 2026-09-15 — on 9a8a590, with uncommitted changes

**Reason:** loader (target half): 194 emitted files byte-identical, proof counts identical (2307/2413 ops, 345/382 loops); only change is the refusal text for a provides file given to gen as a target, 'expected (target NAME ...)' -> 'a target file begins with (target NAME ...)'. Same refusal, same inputs. Tooling passed in the previous full run of this tree.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

18 change(s):

- compiler output changed — `lib/io/go.oro go`
- compiler output changed — `lib/io/go.oro java`
- compiler output changed — `lib/io/go.oro js`
- compiler output changed — `lib/io/java.oro go`
- compiler output changed — `lib/io/java.oro java`
- compiler output changed — `lib/io/java.oro js`
- compiler output changed — `lib/io/js.oro go`
- compiler output changed — `lib/io/js.oro java`
- compiler output changed — `lib/io/js.oro js`
- compiler output changed — `lib/os/go.oro go`
- compiler output changed — `lib/os/go.oro java`
- compiler output changed — `lib/os/go.oro js`
- compiler output changed — `lib/os/java.oro go`
- compiler output changed — `lib/os/java.oro java`
- compiler output changed — `lib/os/java.oro js`
- compiler output changed — `lib/os/js.oro go`
- compiler output changed — `lib/os/js.oro java`
- compiler output changed — `lib/os/js.oro js`

## 2026-09-15 — on 2bfb4eb, with uncommitted changes

**Reason:** lang facts: an unproven obligation in tally now names the fact it knows, len-nonneg, where it said 'known: nothing'; emitted text and proof counts identical

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

3 change(s):

- compiler output changed — `examples/tally/tally.oro go`
- compiler output changed — `examples/tally/tally.oro java`
- compiler output changed — `examples/tally/tally.oro js`

## 2026-09-15 — on 1f0641e, with uncommitted changes

**Reason:** facts F9: the induced mask transfer is tighter than the hand-written andI (it meets and-left), so a windows limb table in render.oro narrows to one byte; HEAD's andI reproduces the baseline exactly, and the differential case prints 25! and 30! correctly from the byte table

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

2 change(s):

- emitted text changed — `examples/big/render.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`

## 2026-09-16 — on 644b6aa, with uncommitted changes

**Reason:** D_T: tally is ONE program on two hosts (step 5). tally-go.oro and tally-java.oro are gone, replaced by (provides TARGET tally/host ...) fragments beside the program; the emitted Go is byte-identical to the old entry's. And a fragment-only file is refused by what it is rather than by the first name inside it, which is the six lib/{io,os} messages.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

25 change(s):

- new source — `examples/tally/host-go.oro go`
- new source — `examples/tally/host-go.oro java`
- new source — `examples/tally/host-go.oro js`
- new source — `examples/tally/host-go.oro windows`
- new source — `examples/tally/host-java.oro go`
- new source — `examples/tally/host-java.oro java`
- new source — `examples/tally/host-java.oro js`
- new source — `examples/tally/host-java.oro windows`
- source removed — `examples/tally/tally-go.oro go`
- source removed — `examples/tally/tally-go.oro java`
- source removed — `examples/tally/tally-go.oro js`
- source removed — `examples/tally/tally-go.oro windows`
- source removed — `examples/tally/tally-java.oro go`
- source removed — `examples/tally/tally-java.oro java`
- source removed — `examples/tally/tally-java.oro js`
- source removed — `examples/tally/tally-java.oro windows`
- compiler output changed — `examples/tally/tally.oro go`
- compiler output changed — `examples/tally/tally.oro java`
- compiler output changed — `examples/tally/tally.oro js`
- compiler output changed — `lib/io/go.oro windows`
- compiler output changed — `lib/io/java.oro windows`
- compiler output changed — `lib/io/js.oro windows`
- compiler output changed — `lib/os/go.oro windows`
- compiler output changed — `lib/os/java.oro windows`
- compiler output changed — `lib/os/js.oro windows`

## 2026-09-16 — on bffdcf9, with uncommitted changes

**Reason:** tally.oro spelled with h.* directly now that the comparator's index is proven: the only change is which unbound name the js refusal reports first.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

1 change(s):

- compiler output changed — `examples/tally/tally.oro js`

## 2026-09-16 — on 99c0549, with uncommitted changes

**Reason:** A let or conditional index is now PROVEN through the branch join where every branch is in range: propagated notes 298 -> 105 (freq 80->21 on three hosts, tokenize 10->0, kara/core 6->0). Emitted code byte-identical, no refusal changed; a failed attempt leaves the note exactly as before.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

5 change(s):

- compiler output changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro js`
- compiler output changed — `examples/json/tokenize.oro go`
- compiler output changed — `examples/kara/core.oro go`

## 2026-09-16 — on 4bdfb41, with uncommitted changes

**Reason:** Goals split on nested conditionals (case-of-case in the logic): propagated notes 105 -> 53, tree.oro 40 -> 0, freq dt 4 -> 0 on three hosts. Emitted code byte-identical, no refusal changed.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- compiler output changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro js`
- compiler output changed — `examples/json/tree.oro go`

## 2026-09-16 — on 80386e1, with uncommitted changes

**Reason:** Loop invariants over templates, result summaries, loops and conditionals named inside goals and guards, and Fourier-Motzkin entailment: propagated notes 53 -> 2 (freq 17 -> 0 on three hosts). Emitted code byte-identical, no refusal changed; farkas checked against enumeration.

456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

3 change(s):

- compiler output changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro js`

## 2026-09-16 — on 87d5d37, with uncommitted changes

**Reason:** Array smashing (smashfd-2026-09-16): a table's contents are an abstract cell, and the copy-closed joint invariant and bounded-increment theorem narrow it. Limb library: a limb is proven non-negative, so the carry split is a shift and a mask (2.2x faster, identical limbs). freq/tally: value clamps deleted, output identical on go/js/java and against tally's hand-written oracle. tree: 525/525 operations proven, so its checked additions become plain ones. Other diffs are fresh-name renumbering only.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

29 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows`
- compiler output changed — `examples/big/fact-limbs.oro windows`
- emitted text changed — `examples/big/limbs.oro go`
- compiler output changed — `examples/big/limbs.oro go`
- emitted text changed — `examples/big/limbs.oro java`
- compiler output changed — `examples/big/limbs.oro java`
- compiler output changed — `examples/big/limbs.oro js`
- emitted text changed — `examples/big/limbs.oro windows`
- compiler output changed — `examples/big/limbs.oro windows`
- compiler output changed — `examples/big/render.oro windows`
- emitted text changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`
- emitted text changed — `examples/json/tree.oro go`
- compiler output changed — `examples/json/tree.oro go`
- compiler output changed — `examples/kara/core.oro go`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-divmod.oro windows`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro go`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro go`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro java`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro java`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro js`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro windows`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- compiler output changed — `gauntlet/differential/cases/render.oro windows`

## 2026-09-17 — on 87d5d37, with uncommitted changes

**Reason:** tree.oro builds without -checked: all 525 operations proven by array smashing, so no checked arithmetic was being selected; the diff is fresh-name renumbering only.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

1 change(s):

- emitted text changed — `examples/json/tree.oro go`

## 2026-09-17 — on 947761c, with uncommitted changes

**Reason:** tree.oro: its six node-index clamps are deleted, proven by facts about one component of a strided table (compfacts-2026-09-17); answers identical to the clamped form on every document size, 1.06x of hand-written unclamped Go and faster than the clamped emission it replaces.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

1 change(s):

- emitted text changed — `examples/json/tree.oro go`
