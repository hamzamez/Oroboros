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

## 2026-09-19 — on ef27603, with uncommitted changes

**Reason:** A literal table's element type is the JOIN of its elements' exact ranges, and at a boundary the DECLARED element decides (docs/literal-elements.md). The two JSON example programs' documents become []byte on Go, with int() on each read, which is what a narrowed buffer already emits; nothing else moves, and windows and JavaScript are untouched by design. Benchmarked: BenchmarkTreeGen 4747 -> 4688 ns median of five at 20000x, 1.3% and inside the noise floor, with TestTree still agreeing with both hand-written implementations. The declared-parameter path was already narrow, which is why the differential twins do not move.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling pass.

2 change(s):

- emitted text changed — `examples/json/tokenize.oro go`
- emitted text changed — `examples/json/tree.oro go`

## 2026-09-20 — on 569cafb, with uncommitted changes

**Reason:** a refusal names the host's own module path: java/java-util-regex/Matcher.find becomes java/java/util/regex/Matcher.find (ADR 0025, modpath-2026-09-20). The text is the only change; no program's code moved.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

1 change(s):

- compiler output changed — `examples/tally/tally.oro java`

## 2026-09-21 — on c1d6a0b, with uncommitted changes

**Reason:** the initial compile-time baseline: CPU time of every gen process in the emission sweep, the minimum of two sweeps (compiletime-2026-09-21). Emission itself is unchanged.

456 runs: 194 emitted, 262 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.


The initial compile-time baseline, the minimum of two sweeps.

## 2026-09-22 — on a9d5df6, with uncommitted changes

**Reason:** ADR 0026: legality is per target. Refusal texts name the target's word instead of the portable window; two new differential cases (word-wide, word-wide-native) witness a result in (2^53, 2^63] that was refused on every target and is now a word on Go, the JVM and windows and a BigInt on JS. No compile changed legality; proof counts unchanged.

464 runs: 201 emitted, 263 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

17 change(s):

- compiler output changed — `examples/big/fact.oro windows`
- compiler output changed — `examples/big/fib.oro windows`
- compiler output changed — `examples/big/pair.oro windows`
- compiler output changed — `examples/big/power.oro windows`
- compiler output changed — `examples/int/collatz.oro go`
- compiler output changed — `examples/int/fib.oro go`
- compiler output changed — `examples/int/power.oro go`
- compiler output changed — `examples/kara/core.oro go`
- compiler output changed — `examples/match/runs.oro go`
- new source — `gauntlet/differential/cases/word-wide-native.oro go`
- new source — `gauntlet/differential/cases/word-wide-native.oro java`
- new source — `gauntlet/differential/cases/word-wide-native.oro js`
- new source — `gauntlet/differential/cases/word-wide-native.oro windows`
- new source — `gauntlet/differential/cases/word-wide.oro go`
- new source — `gauntlet/differential/cases/word-wide.oro java`
- new source — `gauntlet/differential/cases/word-wide.oro js`
- new source — `gauntlet/differential/cases/word-wide.oro windows`

## 2026-09-22 — on 3d5ae83, with uncommitted changes

**Reason:** ADR 0026 (10): Go realizes U = [0, 2^64-1] natively as uint64. Refusal texts on Go name both word realizations; two new differential cases (u64-digits: a declared u64 walked to its digits; u64-square: a product computed into U with nothing declared wide) run natively on Go and are skipped elsewhere until the JVM, windows and JS rungs exist.

472 runs: 209 emitted, 263 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

13 change(s):

- compiler output changed — `examples/int/collatz.oro go`
- compiler output changed — `examples/int/fib.oro go`
- compiler output changed — `examples/int/power.oro go`
- compiler output changed — `examples/kara/core.oro go`
- compiler output changed — `examples/match/runs.oro go`
- new source — `gauntlet/differential/cases/u64-digits.oro go`
- new source — `gauntlet/differential/cases/u64-digits.oro java`
- new source — `gauntlet/differential/cases/u64-digits.oro js`
- new source — `gauntlet/differential/cases/u64-digits.oro windows`
- new source — `gauntlet/differential/cases/u64-square.oro go`
- new source — `gauntlet/differential/cases/u64-square.oro java`
- new source — `gauntlet/differential/cases/u64-square.oro js`
- new source — `gauntlet/differential/cases/u64-square.oro windows`

## 2026-09-23 — on 4ece268, with uncommitted changes

**Reason:** The refiner's memos (tokenize-compile-2026-09-23): the tokeniser compiles 1.78x faster serially, 1.48x in the sweep, below the rule's 1.5; re-recorded deliberately so a regression back to the old cost is caught. Emission is byte-identical.

472 runs: 209 emitted, 263 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.


The compile-time baseline RE-RECORDED (-retime), the minimum of two sweeps — compile time 1.00x the baseline (median of 12 compiles over 300 ms); 1 compile(s) FASTER than the baseline — -accept records them, so a regression back to the old time is caught.

1 compile(s) recorded FASTER:

- `examples/json/tokenize.oro go` 1593 → 781 ms, 0.49x

## 2026-09-23 — on d37875e, with uncommitted changes

**Reason:** The unsigned word's value check committed as a differential case (u64-values: 16,000 values across the 2^63 boundary against a math/big reference), Go only until the other targets' rungs exist.

476 runs: 213 emitted, 263 refused; 2361 of 2413 integer operations bounded, 345 of 382 loops proven. compiler pass, differential pass, tooling skip.

4 change(s):

- new source — `gauntlet/differential/cases/u64-values.oro go`
- new source — `gauntlet/differential/cases/u64-values.oro java`
- new source — `gauntlet/differential/cases/u64-values.oro js`
- new source — `gauntlet/differential/cases/u64-values.oro windows`

## 2026-09-23 — on 22f6cdc, with uncommitted changes

**Reason:** num/u128, its factorial program and differential case: new sources; the n-ary let changes no existing emission (u128-2026-09-23)

488 runs: 215 emitted, 273 refused; 2367 of 2419 integer operations bounded, 347 of 385 loops proven. compiler pass, differential pass, tooling skip.

12 change(s):

- new source — `examples/u128/factorials.oro go`
- new source — `examples/u128/factorials.oro java`
- new source — `examples/u128/factorials.oro js`
- new source — `examples/u128/factorials.oro windows`
- new source — `gauntlet/differential/cases/u128-fact.oro go`
- new source — `gauntlet/differential/cases/u128-fact.oro java`
- new source — `gauntlet/differential/cases/u128-fact.oro js`
- new source — `gauntlet/differential/cases/u128-fact.oro windows`
- new source — `lib/num/u128.oro go`
- new source — `lib/num/u128.oro java`
- new source — `lib/num/u128.oro js`
- new source — `lib/num/u128.oro windows`

## 2026-09-23 — on 22f6cdc, with uncommitted changes

**Reason:** u128 round: the remainder is an atom (Div64 under % now proven; factorials' note gone); a loop that shadows a parameter restores it (examples/match/runs now proven and emitted, its answers checked against Python); num/u128 declares its tuple results; u128-ops is a new case

492 runs: 217 emitted, 275 refused; 2368 of 2419 integer operations bounded, 348 of 385 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- now emits — `examples/match/runs.oro go`
- emitted text changed — `examples/u128/factorials.oro go`
- compiler output changed — `examples/u128/factorials.oro go`
- new source — `gauntlet/differential/cases/u128-ops.oro go`
- new source — `gauntlet/differential/cases/u128-ops.oro java`
- new source — `gauntlet/differential/cases/u128-ops.oro js`
- new source — `gauntlet/differential/cases/u128-ops.oro windows`
- compiler output changed — `lib/num/u128.oro go`

## 2026-09-24 — on d12817c, with uncommitted changes

**Reason:** ADR 0027: a host call's continuation is a tail position. factorials' step binds mulw once (2 Mul64 + 1 Add64, was 6 + 3); lib/num/u128.oro compiles on its own (add/mulw/divw return from inside Add64/Mul64/Div64 continuations); read-loop is a new case on go/js/java. No existing emission moved.

496 runs: 221 emitted, 275 refused; 2368 of 2419 integer operations bounded, 349 of 387 loops proven. compiler pass, differential pass, tooling skip.

7 change(s):

- emitted text changed — `examples/u128/factorials.oro go`
- compiler output changed — `examples/u128/factorials.oro go`
- new source — `gauntlet/differential/cases/read-loop.oro go`
- new source — `gauntlet/differential/cases/read-loop.oro java`
- new source — `gauntlet/differential/cases/read-loop.oro js`
- new source — `gauntlet/differential/cases/read-loop.oro windows`
- now emits — `lib/num/u128.oro go`

## 2026-09-24 — on 0d37fbf, with uncommitted changes

**Reason:** ADR 0028: a definition's parameter ranges and where are obligations at its calls. win/fmt print-int made total (its stale where, 0<=n<2^53, was unprovable at every harness print, and a negative printed a blank line); sieve-win-bench runtime unchanged (median 90 ms vs 90 over 10 runs). render.oro's exported run declares (int 0 15), fact's domain, so (+ 25 n) is proven and loses its checked wrapper.

496 runs: 221 emitted, 275 refused; 2387 of 2434 integer operations bounded, 349 of 387 loops proven. compiler pass, differential pass, tooling skip.

17 change(s):

- emitted text changed — `examples/native/shortcircuit-win.oro windows`
- compiler output changed — `examples/native/shortcircuit-win.oro windows`
- emitted text changed — `examples/native/sieve-win-bench.oro windows`
- compiler output changed — `examples/native/sieve-win-bench.oro windows`
- emitted text changed — `examples/native/sieve-win.oro windows`
- compiler output changed — `examples/native/sieve-win.oro windows`
- emitted text changed — `examples/table/sieve-win.oro windows`
- compiler output changed — `examples/table/sieve-win.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- compiler output changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro java`
- compiler output changed — `gauntlet/differential/cases/render.oro java`
- compiler output changed — `gauntlet/differential/cases/render.oro js`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- compiler output changed — `gauntlet/differential/cases/render.oro windows`
- emitted text changed — `lib/win/fmt.oro windows`
- compiler output changed — `lib/win/fmt.oro windows`

## 2026-09-24 — on 904e958, with uncommitted changes

**Reason:** ADR 0029: above the word the enforced set has a sign — big-fit tests it on Go and Java ([0, 2^k)), big-fit-signed for a signed range; two new differential cases (big-sign, big-negmid). Each change is only the sign test; bounded 200! 1.03x Go, 1.01x Java, within noise

504 runs: 227 emitted, 277 refused; 2387 of 2434 integer operations bounded, 349 of 387 loops proven. compiler pass, differential pass, tooling skip.

18 change(s):

- emitted text changed — `examples/big/fact-limbs.oro go`
- emitted text changed — `examples/big/fact-limbs.oro java`
- emitted text changed — `examples/big/render.oro go`
- emitted text changed — `examples/big/render.oro java`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro go`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro java`
- new source — `gauntlet/differential/cases/big-negmid.oro go`
- new source — `gauntlet/differential/cases/big-negmid.oro java`
- new source — `gauntlet/differential/cases/big-negmid.oro js`
- new source — `gauntlet/differential/cases/big-negmid.oro windows`
- new source — `gauntlet/differential/cases/big-sign.oro go`
- new source — `gauntlet/differential/cases/big-sign.oro java`
- new source — `gauntlet/differential/cases/big-sign.oro js`
- new source — `gauntlet/differential/cases/big-sign.oro windows`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro go`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro java`

## 2026-09-24 — on 9eb6e11, with uncommitted changes

**Reason:** matchguard: a guard's narrowing no longer counts what the condition already counted — 380 operations and 6 loops were counted more than once; unproven unchanged (47 and 38). Emission byte-identical. Also connective narrowing, the trip-count meet (a false proof closed), separated-interval arcs, dead back edges

504 runs: 227 emitted, 277 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

21 change(s):

- compiler output changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro js`
- compiler output changed — `examples/io/jsonfmt.oro go`
- compiler output changed — `examples/io/jsonfmt.oro java`
- compiler output changed — `examples/io/jsonfmt.oro js`
- compiler output changed — `examples/json/tokenize.oro go`
- compiler output changed — `examples/json/tree.oro go`
- compiler output changed — `examples/native/sieve-go-threaded.oro go`
- compiler output changed — `examples/native/sieve-go.oro go`
- compiler output changed — `examples/native/sieve-java.oro java`
- compiler output changed — `examples/native/sieve-win.oro windows`
- compiler output changed — `examples/native/smooth-go.oro go`
- compiler output changed — `examples/native/smooth-java.oro java`
- compiler output changed — `examples/table/sieve.oro go`
- compiler output changed — `gauntlet/differential/cases/big-divmod.oro go`
- compiler output changed — `gauntlet/differential/cases/big-divmod.oro java`
- compiler output changed — `gauntlet/differential/cases/big-divmod.oro js`
- compiler output changed — `gauntlet/differential/cases/render.oro go`
- compiler output changed — `gauntlet/differential/cases/render.oro java`
- compiler output changed — `gauntlet/differential/cases/render.oro js`

## 2026-09-24 — on e2226b6, with uncommitted changes

**Reason:** ADR 0030: the portable os.text-of is d (maximal-subpart substitution) on Go and Java, and os.Args decodes by d on Go; each changed file differs only by the decoder template. New differential case text-of

508 runs: 230 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling pass.

9 change(s):

- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/jsonfmt.oro go`
- emitted text changed — `examples/io/jsonfmt.oro java`
- emitted text changed — `examples/io/wc.oro go`
- new source — `gauntlet/differential/cases/text-of.oro go`
- new source — `gauntlet/differential/cases/text-of.oro java`
- new source — `gauntlet/differential/cases/text-of.oro js`
- new source — `gauntlet/differential/cases/text-of.oro windows`

## 2026-09-25 — on 9b3a94e, with uncommitted changes

**Reason:** ADR 0031 built: tree.oro's parser returns (tuple nodes nn ok) instead of writing count and ok into node 0's slots; proofs identical (99/99, 416/416); TreeGen at 1.04x hand-written, the 3-4% against the old emission shown to be code layout (prodresult-2026-09-25 §5). Three new differential cases (prod-count, prod-loop, prod-two).

520 runs: 242 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling pass.

13 change(s):

- emitted text changed — `examples/json/tree.oro go`
- new source — `gauntlet/differential/cases/prod-count.oro go`
- new source — `gauntlet/differential/cases/prod-count.oro java`
- new source — `gauntlet/differential/cases/prod-count.oro js`
- new source — `gauntlet/differential/cases/prod-count.oro windows`
- new source — `gauntlet/differential/cases/prod-loop.oro go`
- new source — `gauntlet/differential/cases/prod-loop.oro java`
- new source — `gauntlet/differential/cases/prod-loop.oro js`
- new source — `gauntlet/differential/cases/prod-loop.oro windows`
- new source — `gauntlet/differential/cases/prod-two.oro go`
- new source — `gauntlet/differential/cases/prod-two.oro java`
- new source — `gauntlet/differential/cases/prod-two.oro js`
- new source — `gauntlet/differential/cases/prod-two.oro windows`

## 2026-09-25 — on f208c97, with uncommitted changes

**Reason:** refinements.md §3a: an undischarged obligation refuses the program. smooth-js on js loses its two 'refinement propagated' notes (js./ with 3.0 is decided by evaluating the closed comparison); emission otherwise byte-identical. Differential map cases on windows use a guarded read (wm-at); math-bits' Div64 guard matched by relation (opaqueKey).

520 runs: 242 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling pass.

1 change(s):

- compiler output changed — `examples/native/smooth-js.oro js`
