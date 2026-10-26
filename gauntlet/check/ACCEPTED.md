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

## 2026-09-25 — on 3cc5ab5, with uncommitted changes

**Reason:** ADR 0032 step 2: the Go backend is the IR's printer (ir/golang). Every Go output is re-printed from IR_P; the differential suite (44 cases, four targets) and the gauntlet (parity within 2% of the term backend, irstep2-2026-09-25) pass, all 78 Go programs compile, and two refusals keep their outcome with the IR's message.

520 runs: 242 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

45 change(s):

- emitted text changed — `examples/big/fact-limbs.oro go`
- emitted text changed — `examples/big/fact.oro go`
- emitted text changed — `examples/big/fib.oro go`
- emitted text changed — `examples/big/limbs.oro go`
- emitted text changed — `examples/big/pair.oro go`
- emitted text changed — `examples/big/power.oro go`
- emitted text changed — `examples/big/render.oro go`
- emitted text changed — `examples/hello.oro go`
- emitted text changed — `examples/int/digit-sum.oro go`
- emitted text changed — `examples/int/gcd.oro go`
- emitted text changed — `examples/int/sum-range.oro go`
- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/jsonfmt.oro go`
- emitted text changed — `examples/io/wc.oro go`
- emitted text changed — `examples/json/tokenize.oro go`
- emitted text changed — `examples/json/tree.oro go`
- emitted text changed — `examples/kara/workspace.oro go`
- emitted text changed — `examples/map/dynamic.oro go`
- emitted text changed — `examples/match/runs.oro go`
- emitted text changed — `examples/native/centroid-go.oro go`
- emitted text changed — `examples/native/divmod-go.oro go`
- emitted text changed — `examples/native/dot-go.oro go`
- emitted text changed — `examples/native/generic-go.oro go`
- emitted text changed — `examples/native/report-go.oro go`
- emitted text changed — `examples/native/search-go.oro go`
- emitted text changed — `examples/native/shortcircuit-go.oro go`
- emitted text changed — `examples/native/sieve-go-threaded.oro go`
- emitted text changed — `examples/native/sieve-go.oro go`
- emitted text changed — `examples/native/smooth-go.oro go`
- emitted text changed — `examples/native/wordcount-go.oro go`
- emitted text changed — `examples/sum/parse.oro go`
- emitted text changed — `examples/table/alloc.oro go`
- emitted text changed — `examples/table/dot.oro go`
- emitted text changed — `examples/table/sieve.oro go`
- emitted text changed — `examples/u128/factorials.oro go`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro go`
- emitted text changed — `gauntlet/differential/cases/big-negmid.oro go`
- emitted text changed — `gauntlet/differential/cases/big-sign.oro go`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro go`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/shift-div.oro go`
- compiler output changed — `gauntlet/differential/cases/string-escapes.oro go`
- compiler output changed — `gauntlet/differential/cases/utf8-widths.oro go`
- emitted text changed — `lib/num/u128.oro go`

## 2026-09-25 — on ee834bb, with uncommitted changes

**Reason:** alloc of a non-rule table is the table of its contents (tables.md §2): the new differential case alloc-copies on four targets; every existing program byte-identical

524 runs: 246 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

4 change(s):

- new source — `gauntlet/differential/cases/alloc-copies.oro go`
- new source — `gauntlet/differential/cases/alloc-copies.oro java`
- new source — `gauntlet/differential/cases/alloc-copies.oro js`
- new source — `gauntlet/differential/cases/alloc-copies.oro windows`

## 2026-09-25 — on 44dd368, with uncommitted changes

**Reason:** ADR 0032 step 3: the JavaScript backend is the IR's printer (ir/js) on the shared plan (ir/plan). 28 JavaScript outputs re-printed; Go, Java and x86 byte-identical; no outcome changed; differential suite passes; JS gauntlet within 0.94-1.01x of the term backend (irstep3-2026-09-25).

524 runs: 246 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

28 change(s):

- emitted text changed — `examples/big/fact-limbs.oro js`
- emitted text changed — `examples/big/fact.oro js`
- emitted text changed — `examples/big/fib.oro js`
- emitted text changed — `examples/big/limbs.oro js`
- emitted text changed — `examples/big/pair.oro js`
- emitted text changed — `examples/big/power.oro js`
- emitted text changed — `examples/big/render.oro js`
- emitted text changed — `examples/io/freq.oro js`
- emitted text changed — `examples/io/jsonfmt.oro js`
- emitted text changed — `examples/io/wc.oro js`
- emitted text changed — `examples/kara/workspace.oro js`
- emitted text changed — `examples/native/centroid-js.oro js`
- emitted text changed — `examples/native/dot-js.oro js`
- emitted text changed — `examples/native/generic-js.oro js`
- emitted text changed — `examples/native/report-js.oro js`
- emitted text changed — `examples/native/search-js.oro js`
- emitted text changed — `examples/native/sieve-js.oro js`
- emitted text changed — `examples/native/smooth-js.oro js`
- emitted text changed — `examples/native/wordcount-js.oro js`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro js`
- emitted text changed — `gauntlet/differential/cases/big-negmid.oro js`
- emitted text changed — `gauntlet/differential/cases/big-sign.oro js`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro js`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro js`
- emitted text changed — `gauntlet/differential/cases/render.oro js`
- emitted text changed — `gauntlet/differential/cases/shift-div.oro js`
- emitted text changed — `gauntlet/differential/cases/string-escapes.oro js`
- emitted text changed — `gauntlet/differential/cases/utf8-widths.oro js`

## 2026-09-25 — on 5380b77, with uncommitted changes

**Reason:** IR step 3, Java: the Java backend is the IR's printer (ir/java, irstep3java-2026-09-25). Every Java output re-printed; no outcome or proof count changed; two refusals reworded by the IR's untyped-parameter message; narrow-from-wide added (the Java term backend answered 705032704 for 5000000000). Java gauntlet 0.94-1.01x the term backend, JSON tree 0.86x; freq's Java compile 0.41x.

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

33 change(s):

- emitted text changed — `examples/big/fact-limbs.oro java`
- emitted text changed — `examples/big/fact.oro java`
- emitted text changed — `examples/big/fib.oro java`
- emitted text changed — `examples/big/limbs.oro java`
- emitted text changed — `examples/big/pair.oro java`
- emitted text changed — `examples/big/power.oro java`
- emitted text changed — `examples/big/render.oro java`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/jsonfmt.oro java`
- emitted text changed — `examples/io/wc.oro java`
- emitted text changed — `examples/kara/workspace.oro java`
- emitted text changed — `examples/native/centroid-java.oro java`
- emitted text changed — `examples/native/divmod-java.oro java`
- emitted text changed — `examples/native/dot-java.oro java`
- emitted text changed — `examples/native/generic-java.oro java`
- emitted text changed — `examples/native/report-java.oro java`
- emitted text changed — `examples/native/search-java.oro java`
- emitted text changed — `examples/native/sieve-java.oro java`
- emitted text changed — `examples/native/smooth-java.oro java`
- emitted text changed — `examples/native/wordcount-java.oro java`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro java`
- emitted text changed — `gauntlet/differential/cases/big-negmid.oro java`
- emitted text changed — `gauntlet/differential/cases/big-sign.oro java`
- emitted text changed — `gauntlet/differential/cases/big-subdiv.oro java`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro java`
- new source — `gauntlet/differential/cases/narrow-from-wide.oro go`
- new source — `gauntlet/differential/cases/narrow-from-wide.oro java`
- new source — `gauntlet/differential/cases/narrow-from-wide.oro js`
- new source — `gauntlet/differential/cases/narrow-from-wide.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro java`
- emitted text changed — `gauntlet/differential/cases/shift-div.oro java`
- compiler output changed — `gauntlet/differential/cases/string-escapes.oro java`
- compiler output changed — `gauntlet/differential/cases/utf8-widths.oro java`

1 compile(s) recorded FASTER:

- `examples/io/freq.oro java` 43984 → 14531 ms, 0.33x

## 2026-09-26 — on 4bc5a46, with uncommitted changes

**Reason:** IR step 3, x86: the x86 backend is the IR's printer (ir/x86, irstep3x86-2026-09-26); places are a colouring of exact live sets (spec 9.6). 16 windows outputs re-printed; no outcome or proof count changed; Go, JS and Java byte-identical. Windows sieve 0.98x hand-written, 1.00x the term backend; hello-win saves two callee-saved pushes.

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

16 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows`
- emitted text changed — `examples/big/limbs.oro windows`
- emitted text changed — `examples/big/render.oro windows`
- emitted text changed — `examples/kara/workspace.oro windows`
- emitted text changed — `examples/native/hello-win.oro windows`
- emitted text changed — `examples/native/shortcircuit-win.oro windows`
- emitted text changed — `examples/native/sieve-win-bench.oro windows`
- emitted text changed — `examples/native/sieve-win.oro windows`
- emitted text changed — `examples/table/sieve-win.oro windows`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- emitted text changed — `gauntlet/differential/cases/shift-div.oro windows`
- emitted text changed — `gauntlet/differential/cases/string-escapes.oro windows`
- emitted text changed — `gauntlet/differential/cases/utf8-widths.oro windows`
- emitted text changed — `lib/win/fmt.oro windows`

## 2026-09-26 — on b768da0, with uncommitted changes

**Reason:** IR step 4a: the term backends are retired (irstep4a-2026-09-26). 20 outputs re-printed, no outcome or proof count changed: negation (if c false true) is !c on Go, JS and Java (freq, render); x86 orders back-edge updates in place and threads jumps (13 windows outputs). Windows sieve 1.01x hand-written, 1.03x the previous printer, same minimum.

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

20 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows`
- emitted text changed — `examples/big/limbs.oro windows`
- emitted text changed — `examples/big/render.oro go`
- emitted text changed — `examples/big/render.oro java`
- emitted text changed — `examples/big/render.oro js`
- emitted text changed — `examples/big/render.oro windows`
- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/kara/workspace.oro windows`
- emitted text changed — `examples/native/shortcircuit-win.oro windows`
- emitted text changed — `examples/native/sieve-win-bench.oro windows`
- emitted text changed — `examples/native/sieve-win.oro windows`
- emitted text changed — `examples/table/sieve-win.oro windows`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro js`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- emitted text changed — `lib/win/fmt.oro windows`

## 2026-09-26 — on f8c07f2, with uncommitted changes

**Reason:** IR interval domain at parity with the term analysis (irstep4b): Theorem 3 and trip bounds narrow freq's count tables (uint16/short) and tree's worklist (int16), and Java scalars proven in 32 bits become int; differential passes; tree on Go 1.02x hand-written

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- emitted text changed — `examples/big/render.oro java`
- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/wc.oro java`
- emitted text changed — `examples/json/tree.oro go`
- emitted text changed — `examples/native/sieve-java.oro java`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro java`

## 2026-09-26 — on 4ab3f4e, with uncommitted changes

**Reason:** IR step 4c: the IR decides legality and modes; its mask/shift laws narrow Java scalars (limbs, power, limb-subdiv) to int, limb-subdiv's limbs to uint32 on Go and render's digits to bytes on windows; refusal lines in source order with source names; no benchmark covers the changed outputs

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 343 of 381 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- emitted text changed — `examples/big/limbs.oro java`
- emitted text changed — `examples/big/power.oro java`
- compiler output changed — `examples/int/collatz.oro go`
- emitted text changed — `examples/io/freq.oro java`
- compiler output changed — `examples/kara/core.oro go`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro go`
- emitted text changed — `gauntlet/differential/cases/limb-subdiv.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`

1 compile(s) recorded FASTER:

- `examples/io/jsonfmt.oro go` 625 → 375 ms, 0.60x

## 2026-09-26 — on f4248d7, with uncommitted changes

**Reason:** IR step 4d: termination is proven on the IR (359 of 381 loops, was 343): notes only, no emitted code changed; the JS native loops over promoted js.+ and power.oro's halving exponent are proven; gen no longer runs the term interval analysis

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 359 of 381 loops proven. compiler pass, differential pass, tooling skip.

11 change(s):

- compiler output changed — `examples/big/power.oro go`
- compiler output changed — `examples/big/power.oro java`
- compiler output changed — `examples/big/power.oro js`
- compiler output changed — `examples/native/centroid-js.oro js`
- compiler output changed — `examples/native/dot-js.oro js`
- compiler output changed — `examples/native/generic-js.oro js`
- compiler output changed — `examples/native/report-js.oro js`
- compiler output changed — `examples/native/search-js.oro js`
- compiler output changed — `examples/native/sieve-js.oro js`
- compiler output changed — `examples/native/smooth-js.oro js`
- compiler output changed — `examples/native/wordcount-js.oro js`

1 compile(s) recorded FASTER:

- `examples/native/sieve-win-bench.oro windows` 781 → 453 ms, 0.58x

## 2026-09-26 — on b13cded, with uncommitted changes

**Reason:** IR step 4e: postconditions, buffer ranges and shifts on the IR's facts; shift-count notes are now the true count of rewritten operations (the term pass counted per fixpoint evaluation); freq on JS takes len >> 1 where the IR proves the length below 2^31; compile time 0.56x

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 359 of 381 loops proven. compiler pass, differential pass, tooling skip.

17 change(s):

- compiler output changed — `examples/big/fact-limbs.oro windows`
- compiler output changed — `examples/big/limbs.oro go`
- compiler output changed — `examples/big/limbs.oro java`
- compiler output changed — `examples/big/limbs.oro js`
- compiler output changed — `examples/big/limbs.oro windows`
- compiler output changed — `examples/big/render.oro windows`
- compiler output changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`
- compiler output changed — `examples/io/freq.oro js`
- compiler output changed — `examples/match/runs.oro go`
- compiler output changed — `gauntlet/differential/cases/big-divmod.oro windows`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro go`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro java`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro js`
- compiler output changed — `gauntlet/differential/cases/limb-subdiv.oro windows`
- compiler output changed — `gauntlet/differential/cases/render.oro windows`

7 compile(s) recorded FASTER:

- `examples/io/freq.oro go` 24406 → 12937 ms, 0.53x
- `examples/io/freq.oro js` 23000 → 12468 ms, 0.54x
- `examples/io/freq.oro java` 24453 → 13171 ms, 0.54x
- `examples/json/tree.oro go` 2218 → 1000 ms, 0.45x
- `examples/kara/core.oro go` 312 → 46 ms, 0.15x
- `examples/native/sieve-win-bench.oro windows` 406 → 93 ms, 0.23x
- `gauntlet/differential/cases/render.oro windows` 2734 → 703 ms, 0.26x

## 2026-09-26 — on a05b3b7, with uncommitted changes

**Reason:** IR step 4f: the relational difference and Theorem 1 on a difference narrow jsonfmt's output counter to int on Java and tree's node table to uint16 on Go (1.02x hand-written, no slower than before); allocation sizes are now an obligation, proven everywhere in the corpus

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 359 of 381 loops proven. compiler pass, differential pass, tooling skip.

2 change(s):

- emitted text changed — `examples/io/jsonfmt.oro java`
- emitted text changed — `examples/json/tree.oro go`

## 2026-09-26 — on 7a41137, with uncommitted changes

**Reason:** IR step 4g: the unsigned word is chosen on the IR (ADR 0033); u128's decimal now declares its halves in U, so its loop runs in uint64 with fewer conversions and a negative half is refused rather than converted by the residue map

528 runs: 250 emitted, 278 refused; 2007 of 2054 integer operations bounded, 359 of 381 loops proven. compiler pass, differential pass, tooling skip.

3 change(s):

- emitted text changed — `examples/u128/factorials.oro go`
- compiler output changed — `examples/u128/factorials.oro go`
- emitted text changed — `lib/num/u128.oro go`

## 2026-09-26 — on 101c6b6, with uncommitted changes

**Reason:** irstep4h: the rung above the word is chosen on the IR (ADR 0034). pair and power take one more bignum destination each on Go (0.92x and 0.89x, allocations 82->45 and 25->15); render's factorial counter is a word, proven by the IR's fact (+3 operations, +3 loops proven); power reports its shifts on JS and Java, emission identical

528 runs: 250 emitted, 278 refused; 2010 of 2057 integer operations bounded, 362 of 381 loops proven. compiler pass, differential pass, tooling skip.

11 change(s):

- emitted text changed — `examples/big/pair.oro go`
- emitted text changed — `examples/big/power.oro go`
- compiler output changed — `examples/big/power.oro go`
- compiler output changed — `examples/big/power.oro java`
- compiler output changed — `examples/big/power.oro js`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- compiler output changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro java`
- compiler output changed — `gauntlet/differential/cases/render.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro js`
- compiler output changed — `gauntlet/differential/cases/render.oro js`

## 2026-09-27 — on 70085f5, with uncommitted changes

**Reason:** irstep4i: the fixed-limb rung is chosen on the IR and the limb library is inlined as checked instances (ADR 0035). fact-limbs, render and big-limbs are byte-identical on windows; big-divmod moves one prologue add; render's factorial counter is a word, not limbs (-16 operations and -6 loops, all 13 remaining loops proven, 1322 -> 830 lines)

528 runs: 250 emitted, 278 refused; 1994 of 2041 integer operations bounded, 357 of 375 loops proven. compiler pass, differential pass, tooling skip.

3 change(s):

- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- compiler output changed — `gauntlet/differential/cases/render.oro windows`

1 compile(s) recorded FASTER:

- `gauntlet/differential/cases/render.oro windows` 578 → 281 ms, 0.49x

## 2026-09-28 — on d8d7f16, with uncommitted changes

**Reason:** irstep4j: the term interval analysis is deleted; the IR's domain gains a map's cell fact (a read's payload is the hull of the inserted values), which proves map/dynamic's two operations on Go, so its -checked traps become plain operations

528 runs: 250 emitted, 278 refused; 1996 of 2041 integer operations bounded, 357 of 375 loops proven. compiler pass, differential pass, tooling skip.

2 change(s):

- emitted text changed — `examples/map/dynamic.oro go`
- compiler output changed — `examples/map/dynamic.oro go`

## 2026-09-29 — on 870de46, with uncommitted changes

**Reason:** Retire the portable layer (portable-2026-09-29): 10 portable-layer examples deleted, refused on every real target; examples/native/filter-go.oro added, the push-stream pipeline ported

492 runs: 251 emitted, 241 refused; 1997 of 2042 integer operations bounded, 358 of 376 loops proven. compiler pass, differential pass, tooling skip.

44 change(s):

- source removed — `examples/build-vec.oro go`
- source removed — `examples/build-vec.oro java`
- source removed — `examples/build-vec.oro js`
- source removed — `examples/build-vec.oro windows`
- source removed — `examples/centroid.oro go`
- source removed — `examples/centroid.oro java`
- source removed — `examples/centroid.oro js`
- source removed — `examples/centroid.oro windows`
- source removed — `examples/converge.oro go`
- source removed — `examples/converge.oro java`
- source removed — `examples/converge.oro js`
- source removed — `examples/converge.oro windows`
- source removed — `examples/filter.oro go`
- source removed — `examples/filter.oro java`
- source removed — `examples/filter.oro js`
- source removed — `examples/filter.oro windows`
- source removed — `examples/generic.oro go`
- source removed — `examples/generic.oro java`
- source removed — `examples/generic.oro js`
- source removed — `examples/generic.oro windows`
- new source — `examples/native/filter-go.oro go`
- new source — `examples/native/filter-go.oro java`
- new source — `examples/native/filter-go.oro js`
- new source — `examples/native/filter-go.oro windows`
- source removed — `examples/norm.oro go`
- source removed — `examples/norm.oro java`
- source removed — `examples/norm.oro js`
- source removed — `examples/norm.oro windows`
- source removed — `examples/report.oro go`
- source removed — `examples/report.oro java`
- source removed — `examples/report.oro js`
- source removed — `examples/report.oro windows`
- source removed — `examples/search.oro go`
- source removed — `examples/search.oro java`
- source removed — `examples/search.oro js`
- source removed — `examples/search.oro windows`
- source removed — `examples/stencil.oro go`
- source removed — `examples/stencil.oro java`
- source removed — `examples/stencil.oro js`
- source removed — `examples/stencil.oro windows`
- source removed — `examples/wordcount.oro go`
- source removed — `examples/wordcount.oro java`
- source removed — `examples/wordcount.oro js`
- source removed — `examples/wordcount.oro windows`

## 2026-09-30 — on c6fc635, with uncommitted changes

**Reason:** Go target, builtin step (gotarget-2026-09-30 13): per-type table names retired onto the language's table (their pure reads beside stores were reordered by reduction); report and smooth now use build, measured unchanged (new/old 1.04, 0.99, 0.94; drift 0.95-1.05); the two sieve-go variants deleted, examples/table/sieve.oro is their program

484 runs: 249 emitted, 235 refused; 1965 of 2010 integer operations bounded, 346 of 364 loops proven. compiler pass, differential pass, tooling pass.

10 change(s):

- emitted text changed — `examples/native/report-go.oro go`
- source removed — `examples/native/sieve-go-threaded.oro go`
- source removed — `examples/native/sieve-go-threaded.oro java`
- source removed — `examples/native/sieve-go-threaded.oro js`
- source removed — `examples/native/sieve-go-threaded.oro windows`
- source removed — `examples/native/sieve-go.oro go`
- source removed — `examples/native/sieve-go.oro java`
- source removed — `examples/native/sieve-go.oro js`
- source removed — `examples/native/sieve-go.oro windows`
- emitted text changed — `examples/native/smooth-go.oro go`

## 2026-09-30 — on d5db79f, with uncommitted changes

**Reason:** libos: examples/io/roundtrip.oro, the witness that every portable os/io name builds and runs on Go, JavaScript and Java (refused on windows, which has no os cell)

488 runs: 252 emitted, 236 refused; 1965 of 2010 integer operations bounded, 346 of 364 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `examples/io/roundtrip.oro go`
- new source — `examples/io/roundtrip.oro java`
- new source — `examples/io/roundtrip.oro js`
- new source — `examples/io/roundtrip.oro windows`

1 compile(s) recorded FASTER:

- `examples/io/jsonfmt.oro js` 578 → 281 ms, 0.49x

## 2026-10-01 — on cc48e96, with uncommitted changes

**Reason:** bufio: examples/lines/lines.oro, the line tool over bufio (new, four targets); kara's refusal prints a binder as out3 because a residual's binders are now named apart along the scope chain. Every emitted file is byte-identical, so no benchmark is affected; paired compile time freq 0.99x, tree 1.04x, jsonfmt 1.05x

492 runs: 253 emitted, 239 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

5 change(s):

- compiler output changed — `examples/kara/core.oro go`
- new source — `examples/lines/lines.oro go`
- new source — `examples/lines/lines.oro java`
- new source — `examples/lines/lines.oro js`
- new source — `examples/lines/lines.oro windows`

## 2026-10-04 — on 137e9c6, with uncommitted changes

**Reason:** irtext-2026-10-04: gen prints a program's functions in name order, the order of its IR text (ADR 0038), so x86's file-wide label and literal numbers are a function of the IR: lib/win/fmt and kara/workspace on windows renumber labels only; six programs' notes reorder; kara/core names gen-main before gen-run in an identical refusal

492 runs: 253 emitted, 239 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

10 change(s):

- compiler output changed — `examples/json/tokenize.oro go`
- compiler output changed — `examples/kara/core.oro go`
- compiler output changed — `examples/kara/workspace.oro go`
- compiler output changed — `examples/kara/workspace.oro java`
- compiler output changed — `examples/kara/workspace.oro js`
- emitted text changed — `examples/kara/workspace.oro windows`
- compiler output changed — `examples/kara/workspace.oro windows`
- compiler output changed — `examples/native/sieve-js.oro js`
- compiler output changed — `lib/num/u128.oro go`
- emitted text changed — `lib/win/fmt.oro windows`

## 2026-10-06 — on be89946, with uncommitted changes

**Reason:** lib/result.oro, the result variant and err-map (spec/errors.md §3, ADR 0040): a new library source, refused on every target because the sweep compiles it with lib/ as its own nearest layer, so lib/io's and lib/os's provides fragments load twice ("io.print-line is declared twice"); a loader quirk no file directly under lib/ had met (corrected after the run)

496 runs: 253 emitted, 243 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `lib/result.oro go`
- new source — `lib/result.oro java`
- new source — `lib/result.oro js`
- new source — `lib/result.oro windows`

## 2026-10-06 — on 472825e, with uncommitted changes

**Reason:** expect and abandon (errors.md §12 step 3): two new sources, examples/errors/expect.oro and the differential case expect.oro, on four targets; every existing file byte-identical

504 runs: 261 emitted, 243 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- new source — `examples/errors/expect.oro go`
- new source — `examples/errors/expect.oro java`
- new source — `examples/errors/expect.oro js`
- new source — `examples/errors/expect.oro windows`
- new source — `gauntlet/differential/cases/expect.oro go`
- new source — `gauntlet/differential/cases/expect.oro java`
- new source — `gauntlet/differential/cases/expect.oro js`
- new source — `gauntlet/differential/cases/expect.oro windows`

## 2026-10-06 — on 0946092, with uncommitted changes

**Reason:** constructor spelled as the tuple it is, and eta for products: lib/result.oro's windows refusal prints #k; new differential case try-loop.oro on four targets; every other file byte-identical

508 runs: 265 emitted, 243 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

5 change(s):

- new source — `gauntlet/differential/cases/try-loop.oro go`
- new source — `gauntlet/differential/cases/try-loop.oro java`
- new source — `gauntlet/differential/cases/try-loop.oro js`
- new source — `gauntlet/differential/cases/try-loop.oro windows`
- compiler output changed — `lib/result.oro windows`

## 2026-10-06 — on f4993f4, with uncommitted changes

**Reason:** the unit (tuple) and seq's unwritable binder: new differential case unit.oro on four targets; every other file byte-identical

512 runs: 269 emitted, 243 refused; 1970 of 2015 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `gauntlet/differential/cases/unit.oro go`
- new source — `gauntlet/differential/cases/unit.oro java`
- new source — `gauntlet/differential/cases/unit.oro js`
- new source — `gauntlet/differential/cases/unit.oro windows`

## 2026-10-06 — on 8568c14, with uncommitted changes

**Reason:** ADR 0042, a variant's value is its tag and its slots: new sources examples/errors/slots.oro and cases/sum-slots.oro on four targets; lib/result.oro's windows refusal prints the slots; every other file byte-identical

520 runs: 277 emitted, 243 refused; 1974 of 2019 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

9 change(s):

- new source — `examples/errors/slots.oro go`
- new source — `examples/errors/slots.oro java`
- new source — `examples/errors/slots.oro js`
- new source — `examples/errors/slots.oro windows`
- new source — `gauntlet/differential/cases/sum-slots.oro go`
- new source — `gauntlet/differential/cases/sum-slots.oro java`
- new source — `gauntlet/differential/cases/sum-slots.oro js`
- new source — `gauntlet/differential/cases/sum-slots.oro windows`
- compiler output changed — `lib/result.oro windows`

## 2026-10-06 — on 9605545, with uncommitted changes

**Reason:** the error model's niche (errors.md §4, step 4): tally's refusal on go and java gains an accurate import hint, because a target library's use is now closed over; every emitted file byte-identical

520 runs: 277 emitted, 243 refused; 1974 of 2019 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

2 change(s):

- compiler output changed — `examples/tally/tally.oro go`
- compiler output changed — `examples/tally/tally.oro java`

## 2026-10-06 — on 5783c8f, with uncommitted changes

**Reason:** step 5a, the portable os returns kinds (oskinds-2026-10-06): roundtrip rewritten in the model with the kinds' cross-host line; windows refusals name os/host; lib/os.oro a new library module; wc, jsonfmt and freq byte-identical

524 runs: 277 emitted, 247 refused; 1974 of 2019 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

10 change(s):

- emitted text changed — `examples/io/roundtrip.oro go`
- emitted text changed — `examples/io/roundtrip.oro java`
- emitted text changed — `examples/io/roundtrip.oro js`
- compiler output changed — `examples/tally/tally.oro windows`
- compiler output changed — `gauntlet/differential/cases/read-loop.oro windows`
- compiler output changed — `gauntlet/differential/cases/text-of.oro windows`
- new source — `lib/os.oro go`
- new source — `lib/os.oro java`
- new source — `lib/os.oro js`
- new source — `lib/os.oro windows`

## 2026-10-06 — on 535c11d, with uncommitted changes

**Reason:** step 5b, the fallible Go declarations in the error model (gomigrate-2026-10-06): lines.oro migrated, the niche test shared and summed's range test at each use; one more operation bounded

524 runs: 277 emitted, 247 refused; 1975 of 2020 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

2 change(s):

- emitted text changed — `examples/lines/lines.oro go`
- compiler output changed — `examples/lines/lines.oro go`

## 2026-10-07 — on 859df81, with uncommitted changes

**Reason:** step 6, relevance: lib/result.oro alone is refused with the same reason, its term now printing a relevant constructor's binder #k!; examples/errors/discard.oro is refused on every host by the relevance rule and ignore.oro emits what discard.oro emitted before

532 runs: 280 emitted, 252 refused; 1975 of 2020 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

9 change(s):

- new source — `examples/errors/discard.oro go`
- new source — `examples/errors/discard.oro java`
- new source — `examples/errors/discard.oro js`
- new source — `examples/errors/discard.oro windows`
- new source — `examples/errors/ignore.oro go`
- new source — `examples/errors/ignore.oro java`
- new source — `examples/errors/ignore.oro js`
- new source — `examples/errors/ignore.oro windows`
- compiler output changed — `lib/result.oro windows`

## 2026-10-08 — on 4d2fcea, with uncommitted changes

**Reason:** ADR 0045: fmt's ...any as a table of Go's boxes; lines.oro and u128/factorials.oro spread []any into Fprintf and Println (measured equal to Go's own call); factorials' refusal text on other targets names PrintlnAll

532 runs: 280 emitted, 252 refused; 1975 of 2020 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

5 change(s):

- emitted text changed — `examples/lines/lines.oro go`
- emitted text changed — `examples/u128/factorials.oro go`
- compiler output changed — `examples/u128/factorials.oro java`
- compiler output changed — `examples/u128/factorials.oro js`
- compiler output changed — `examples/u128/factorials.oro windows`

## 2026-10-08 — on 26679e0, with uncommitted changes

**Reason:** ADR 0046: a variadic is one declaration under Go's name, a call's arguments its list; Go's own call printed where the []any spread was; a variadic print's value is the unit (return 0 where a function's value was a print); IR value names shifted by the list and the unit

532 runs: 280 emitted, 252 refused; 1975 of 2020 integer operations bounded, 347 of 365 loops proven. compiler pass, differential pass, tooling pass.

8 change(s):

- emitted text changed — `examples/hello.oro go`
- emitted text changed — `examples/lines/lines.oro go`
- emitted text changed — `examples/map/dynamic.oro go`
- emitted text changed — `examples/native/report-go.oro go`
- emitted text changed — `examples/u128/factorials.oro go`
- compiler output changed — `examples/u128/factorials.oro java`
- compiler output changed — `examples/u128/factorials.oro js`
- compiler output changed — `examples/u128/factorials.oro windows`

## 2026-10-08 — on f5fe4bb, with uncommitted changes

**Reason:** local (ADR 0047): the differential cases local-cells, local-tables, local-case and examples/io/count.oro are new; fact-limbs, big-divmod and render on windows lose a dead (< literal 0) test, folded once a residual let's duplicable value is substituted

548 runs: 295 emitted, 253 refused; 2023 of 2068 integer operations bounded, 350 of 368 loops proven. compiler pass, differential pass, tooling pass.

19 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows`
- new source — `examples/io/count.oro go`
- new source — `examples/io/count.oro java`
- new source — `examples/io/count.oro js`
- new source — `examples/io/count.oro windows`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- new source — `gauntlet/differential/cases/local-case.oro go`
- new source — `gauntlet/differential/cases/local-case.oro java`
- new source — `gauntlet/differential/cases/local-case.oro js`
- new source — `gauntlet/differential/cases/local-case.oro windows`
- new source — `gauntlet/differential/cases/local-cells.oro go`
- new source — `gauntlet/differential/cases/local-cells.oro java`
- new source — `gauntlet/differential/cases/local-cells.oro js`
- new source — `gauntlet/differential/cases/local-cells.oro windows`
- new source — `gauntlet/differential/cases/local-tables.oro go`
- new source — `gauntlet/differential/cases/local-tables.oro java`
- new source — `gauntlet/differential/cases/local-tables.oro js`
- new source — `gauntlet/differential/cases/local-tables.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`

## 2026-10-08 — on 387da9f, with uncommitted changes

**Reason:** local-zero: a table of a sort's zero is build's zero-filled form, its sort ascribed for the checker alone

552 runs: 299 emitted, 253 refused; 2023 of 2068 integer operations bounded, 350 of 368 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `gauntlet/differential/cases/local-zero.oro go`
- new source — `gauntlet/differential/cases/local-zero.oro java`
- new source — `gauntlet/differential/cases/local-zero.oro js`
- new source — `gauntlet/differential/cases/local-zero.oro windows`

1 compile(s) recorded FASTER:

- `examples/io/freq.oro go` 16468 → 10734 ms, 0.65x

## 2026-10-09 — on 4fdf21a, with uncommitted changes

**Reason:** outcells (ADR 0048): examples/lines/ages.oro is new; nothing else changed through the cell translation's move, the unifier's check and the purity fix

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `examples/lines/ages.oro go`
- new source — `examples/lines/ages.oro java`
- new source — `examples/lines/ages.oro js`
- new source — `examples/lines/ages.oro windows`

## 2026-10-13 — on 32d5134, with uncommitted changes

**Reason:** stale clamps deleted (claimsdemand-2026-10-13): freq's cli in iota, putnum, putword and the line layout, and limbs' at with (k (int 0 63)) on its export; the analysis proves every index they guarded. Same output on six inputs (freq) and the big-limbs case on four targets; run time indistinguishable

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling skip.

7 change(s):

- emitted text changed — `examples/big/limbs.oro go`
- emitted text changed — `examples/big/limbs.oro java`
- emitted text changed — `examples/big/limbs.oro js`
- emitted text changed — `examples/big/limbs.oro windows`
- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`

## 2026-10-13 — on 5f020fd, with uncommitted changes

**Reason:** freq's pat reads without a clamp: k >= 0 and the guard not (k = 0) give k >= 1, now that a disequality beside an order is strict (refinements.md 3c). Same output on six inputs

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling skip.

3 change(s):

- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`

## 2026-10-13 — on 0f09b09, with uncommitted changes

**Reason:** freq's merge reads without clamps: i + j - k = mid, a translation invariant (refinements.md 5c), bounds j by hi once i reaches mid. Same output on six inputs; merge-sort's case without its clamp agrees on four targets

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling skip.

3 change(s):

- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`

## 2026-10-13 — on 1c23ea3, with uncommitted changes

**Reason:** freq's dists stores and reads without clamps: the first iteration peeled gives d >= 1 from the second on, a failed or is the conjunction of its failed disjuncts, and a failed and resolved by a later guard gives d < nd (refinements.md 5d). Same output on six inputs; freq compiles 4% slower in a serial pair (5.87 against 5.65 s), unchanged sources flat

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling skip.

3 change(s):

- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`

## 2026-10-14 — on be8bd22, with uncommitted changes

**Reason:** freq's cl gone at all three uses (wbyte, putword, dword): reads are atoms and a record fact beg <= end over the spans' pairs bounds beg + k by end <= len src (refinements.md 5e). The four shifts no longer counted were len(sp) >> 1 inside the deleted clamps. Same output on seven inputs; tree compiles 0.76 against 0.82 s and freq about 7.0 against 6.7 s in interleaved serial runs

556 runs: 300 emitted, 256 refused; 2039 of 2084 integer operations bounded, 351 of 369 loops proven. compiler pass, differential pass, tooling skip.

6 change(s):

- emitted text changed — `examples/io/freq.oro go`
- compiler output changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- compiler output changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`
- compiler output changed — `examples/io/freq.oro js`

## 2026-10-14 — on 52e6e12, with uncommitted changes

**Reason:** examples/os/du.oro, a directory walk program first for os.ReadDir, io/fs and path/filepath: 19 of 19 operations proven, its output checked against its definition by the acceptance harness and against Go's filepath.WalkDir on two trees; the other targets refuse it for want of go/os

560 runs: 301 emitted, 259 refused; 2058 of 2103 integer operations bounded, 352 of 371 loops proven. compiler pass, differential pass, tooling skip.

4 change(s):

- new source — `examples/os/du.oro go`
- new source — `examples/os/du.oro java`
- new source — `examples/os/du.oro js`
- new source — `examples/os/du.oro windows`

## 2026-10-15 — on 22623d8, with uncommitted changes

**Reason:** State in program order (ADR 0052, spec/local.md §2.1), applied to cells as to buffers: a loop carries the state it writes, first, in binding order (49 of the 53 hand-threaded loops' order, gauntlet/surface/stores.go), and a statement if writing one state binder is a φ on its value. examples/io/count.oro and examples/lines/ages.oro, the two cell programs, are the same programs with their cells first and one join where duplicated branches were: 19 fewer operations, the same 45 unproven, compile time 0.98x; differential and cmd/build's ages run pass (storeorder-2026-10-15)

560 runs: 301 emitted, 259 refused; 2039 of 2084 integer operations bounded, 352 of 371 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- emitted text changed — `examples/io/count.oro go`
- compiler output changed — `examples/io/count.oro go`
- emitted text changed — `examples/io/count.oro java`
- compiler output changed — `examples/io/count.oro java`
- emitted text changed — `examples/io/count.oro js`
- compiler output changed — `examples/io/count.oro js`
- emitted text changed — `examples/lines/ages.oro go`
- compiler output changed — `examples/lines/ages.oro go`

## 2026-10-16 — on 0c6470f, with uncommitted changes

**Reason:** Step 5a, the corpus in J (ADR 0068, jswitch-2026-10-16): every source and target file is J, read by J's reader. Proof counts unchanged (2039 of 2084, 352 of 371), differential passes, compile time 1.07x. Changes, each reviewed: jsonfmt computes one addition before a store, where today's program named a version with a let (storeorder-2026-10-15); the limb library's guard borrows its buffer and its dead (if c o o) branch is gone, on windows in fact-limbs, big-divmod and render, the same trap check in fewer instructions; refusal texts: a fragment's (provides …) reworded for J, a fragment found as a target layer file named as one (J target files have no header), kara/core's version binder #vout2 for out3, tally an entry module (in tally for in tally.tally)

560 runs: 301 emitted, 259 refused; 2039 of 2084 integer operations bounded, 352 of 371 loops proven. compiler pass, differential pass, tooling skip.

40 change(s):

- emitted text changed — `examples/big/fact-limbs.oro windows`
- emitted text changed — `examples/io/jsonfmt.oro go`
- emitted text changed — `examples/io/jsonfmt.oro java`
- emitted text changed — `examples/io/jsonfmt.oro js`
- compiler output changed — `examples/kara/core.oro go`
- compiler output changed — `examples/tally/host-go.oro go`
- compiler output changed — `examples/tally/host-go.oro java`
- compiler output changed — `examples/tally/host-go.oro js`
- compiler output changed — `examples/tally/host-go.oro windows`
- compiler output changed — `examples/tally/host-java.oro go`
- compiler output changed — `examples/tally/host-java.oro java`
- compiler output changed — `examples/tally/host-java.oro js`
- compiler output changed — `examples/tally/host-java.oro windows`
- compiler output changed — `examples/tally/tally.oro js`
- emitted text changed — `gauntlet/differential/cases/big-divmod.oro windows`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- compiler output changed — `lib/io/go.oro go`
- compiler output changed — `lib/io/go.oro java`
- compiler output changed — `lib/io/go.oro js`
- compiler output changed — `lib/io/go.oro windows`
- compiler output changed — `lib/io/java.oro go`
- compiler output changed — `lib/io/java.oro java`
- compiler output changed — `lib/io/java.oro js`
- compiler output changed — `lib/io/java.oro windows`
- compiler output changed — `lib/io/js.oro go`
- compiler output changed — `lib/io/js.oro java`
- compiler output changed — `lib/io/js.oro js`
- compiler output changed — `lib/io/js.oro windows`
- compiler output changed — `lib/os/go.oro go`
- compiler output changed — `lib/os/go.oro java`
- compiler output changed — `lib/os/go.oro js`
- compiler output changed — `lib/os/go.oro windows`
- compiler output changed — `lib/os/java.oro go`
- compiler output changed — `lib/os/java.oro java`
- compiler output changed — `lib/os/java.oro js`
- compiler output changed — `lib/os/java.oro windows`
- compiler output changed — `lib/os/js.oro go`
- compiler output changed — `lib/os/js.oro java`
- compiler output changed — `lib/os/js.oro js`
- compiler output changed — `lib/os/js.oro windows`

## 2026-10-20 — on 7214e38, with uncommitted changes

**Reason:** Messages in J (jmsg-2026-10-20): a refusal quotes names by σ_J, sets by J's formers and terms by J's expressions (core/jshow.go); emission unchanged

560 runs: 301 emitted, 259 refused; 2039 of 2084 integer operations bounded, 352 of 371 loops proven. compiler pass, differential pass, tooling skip.

235 change(s):

- compiler output changed — `examples/big/fact.oro windows`
- compiler output changed — `examples/big/fib.oro windows`
- compiler output changed — `examples/big/pair.oro windows`
- compiler output changed — `examples/big/power.oro windows`
- compiler output changed — `examples/dot.oro go`
- compiler output changed — `examples/dot.oro java`
- compiler output changed — `examples/dot.oro js`
- compiler output changed — `examples/dot.oro windows`
- compiler output changed — `examples/errors/discard.oro go`
- compiler output changed — `examples/errors/discard.oro java`
- compiler output changed — `examples/errors/discard.oro js`
- compiler output changed — `examples/errors/discard.oro windows`
- compiler output changed — `examples/errors/ignore.oro windows`
- compiler output changed — `examples/hello.oro java`
- compiler output changed — `examples/hello.oro js`
- compiler output changed — `examples/hello.oro windows`
- compiler output changed — `examples/int/collatz.oro java`
- compiler output changed — `examples/int/collatz.oro js`
- compiler output changed — `examples/int/collatz.oro windows`
- compiler output changed — `examples/int/digit-sum.oro java`
- compiler output changed — `examples/int/digit-sum.oro js`
- compiler output changed — `examples/int/digit-sum.oro windows`
- compiler output changed — `examples/int/fib.oro java`
- compiler output changed — `examples/int/fib.oro js`
- compiler output changed — `examples/int/fib.oro windows`
- compiler output changed — `examples/int/gcd.oro java`
- compiler output changed — `examples/int/gcd.oro js`
- compiler output changed — `examples/int/gcd.oro windows`
- compiler output changed — `examples/int/power.oro java`
- compiler output changed — `examples/int/power.oro js`
- compiler output changed — `examples/int/power.oro windows`
- compiler output changed — `examples/int/sum-range.oro java`
- compiler output changed — `examples/int/sum-range.oro js`
- compiler output changed — `examples/int/sum-range.oro windows`
- compiler output changed — `examples/io/count.oro windows`
- compiler output changed — `examples/io/freq.oro windows`
- compiler output changed — `examples/io/jsonfmt.oro windows`
- compiler output changed — `examples/io/roundtrip.oro windows`
- compiler output changed — `examples/io/wc.oro windows`
- compiler output changed — `examples/json/tokenize.oro java`
- compiler output changed — `examples/json/tokenize.oro js`
- compiler output changed — `examples/json/tokenize.oro windows`
- compiler output changed — `examples/json/tree.oro java`
- compiler output changed — `examples/json/tree.oro js`
- compiler output changed — `examples/json/tree.oro windows`
- compiler output changed — `examples/kara/core.oro java`
- compiler output changed — `examples/kara/core.oro js`
- compiler output changed — `examples/kara/core.oro windows`
- compiler output changed — `examples/lines/ages.oro java`
- compiler output changed — `examples/lines/ages.oro js`
- compiler output changed — `examples/lines/ages.oro windows`
- compiler output changed — `examples/lines/lines.oro java`
- compiler output changed — `examples/lines/lines.oro js`
- compiler output changed — `examples/lines/lines.oro windows`
- compiler output changed — `examples/map/dynamic.oro java`
- compiler output changed — `examples/map/dynamic.oro js`
- compiler output changed — `examples/map/dynamic.oro windows`
- compiler output changed — `examples/match/runs.oro java`
- compiler output changed — `examples/match/runs.oro js`
- compiler output changed — `examples/match/runs.oro windows`
- compiler output changed — `examples/modules.oro go`
- compiler output changed — `examples/modules.oro java`
- compiler output changed — `examples/modules.oro js`
- compiler output changed — `examples/modules.oro windows`
- compiler output changed — `examples/native/centroid-go.oro java`
- compiler output changed — `examples/native/centroid-go.oro js`
- compiler output changed — `examples/native/centroid-go.oro windows`
- compiler output changed — `examples/native/centroid-java.oro go`
- compiler output changed — `examples/native/centroid-java.oro js`
- compiler output changed — `examples/native/centroid-java.oro windows`
- compiler output changed — `examples/native/centroid-js.oro go`
- compiler output changed — `examples/native/centroid-js.oro java`
- compiler output changed — `examples/native/centroid-js.oro windows`
- compiler output changed — `examples/native/divmod-go.oro java`
- compiler output changed — `examples/native/divmod-go.oro js`
- compiler output changed — `examples/native/divmod-go.oro windows`
- compiler output changed — `examples/native/divmod-java.oro go`
- compiler output changed — `examples/native/divmod-java.oro js`
- compiler output changed — `examples/native/divmod-java.oro windows`
- compiler output changed — `examples/native/dot-go.oro java`
- compiler output changed — `examples/native/dot-go.oro js`
- compiler output changed — `examples/native/dot-go.oro windows`
- compiler output changed — `examples/native/dot-java.oro go`
- compiler output changed — `examples/native/dot-java.oro js`
- compiler output changed — `examples/native/dot-java.oro windows`
- compiler output changed — `examples/native/dot-js.oro go`
- compiler output changed — `examples/native/dot-js.oro java`
- compiler output changed — `examples/native/dot-js.oro windows`
- compiler output changed — `examples/native/filter-go.oro java`
- compiler output changed — `examples/native/filter-go.oro js`
- compiler output changed — `examples/native/filter-go.oro windows`
- compiler output changed — `examples/native/generic-go.oro java`
- compiler output changed — `examples/native/generic-go.oro js`
- compiler output changed — `examples/native/generic-go.oro windows`
- compiler output changed — `examples/native/generic-java.oro go`
- compiler output changed — `examples/native/generic-java.oro js`
- compiler output changed — `examples/native/generic-java.oro windows`
- compiler output changed — `examples/native/generic-js.oro go`
- compiler output changed — `examples/native/generic-js.oro java`
- compiler output changed — `examples/native/generic-js.oro windows`
- compiler output changed — `examples/native/hello-win.oro go`
- compiler output changed — `examples/native/hello-win.oro java`
- compiler output changed — `examples/native/hello-win.oro js`
- compiler output changed — `examples/native/report-go.oro java`
- compiler output changed — `examples/native/report-go.oro js`
- compiler output changed — `examples/native/report-go.oro windows`
- compiler output changed — `examples/native/report-java.oro go`
- compiler output changed — `examples/native/report-java.oro js`
- compiler output changed — `examples/native/report-java.oro windows`
- compiler output changed — `examples/native/report-js.oro go`
- compiler output changed — `examples/native/report-js.oro java`
- compiler output changed — `examples/native/report-js.oro windows`
- compiler output changed — `examples/native/search-go.oro java`
- compiler output changed — `examples/native/search-go.oro js`
- compiler output changed — `examples/native/search-go.oro windows`
- compiler output changed — `examples/native/search-java.oro go`
- compiler output changed — `examples/native/search-java.oro js`
- compiler output changed — `examples/native/search-java.oro windows`
- compiler output changed — `examples/native/search-js.oro go`
- compiler output changed — `examples/native/search-js.oro java`
- compiler output changed — `examples/native/search-js.oro windows`
- compiler output changed — `examples/native/shortcircuit-go.oro java`
- compiler output changed — `examples/native/shortcircuit-go.oro js`
- compiler output changed — `examples/native/shortcircuit-go.oro windows`
- compiler output changed — `examples/native/shortcircuit-win.oro go`
- compiler output changed — `examples/native/shortcircuit-win.oro java`
- compiler output changed — `examples/native/shortcircuit-win.oro js`
- compiler output changed — `examples/native/sieve-java.oro go`
- compiler output changed — `examples/native/sieve-java.oro js`
- compiler output changed — `examples/native/sieve-java.oro windows`
- compiler output changed — `examples/native/sieve-js.oro go`
- compiler output changed — `examples/native/sieve-js.oro java`
- compiler output changed — `examples/native/sieve-js.oro windows`
- compiler output changed — `examples/native/sieve-win-bench.oro go`
- compiler output changed — `examples/native/sieve-win-bench.oro java`
- compiler output changed — `examples/native/sieve-win-bench.oro js`
- compiler output changed — `examples/native/sieve-win.oro go`
- compiler output changed — `examples/native/sieve-win.oro java`
- compiler output changed — `examples/native/sieve-win.oro js`
- compiler output changed — `examples/native/smooth-go.oro java`
- compiler output changed — `examples/native/smooth-go.oro js`
- compiler output changed — `examples/native/smooth-go.oro windows`
- compiler output changed — `examples/native/smooth-java.oro go`
- compiler output changed — `examples/native/smooth-java.oro js`
- compiler output changed — `examples/native/smooth-java.oro windows`
- compiler output changed — `examples/native/smooth-js.oro go`
- compiler output changed — `examples/native/smooth-js.oro java`
- compiler output changed — `examples/native/smooth-js.oro windows`
- compiler output changed — `examples/native/wordcount-go.oro java`
- compiler output changed — `examples/native/wordcount-go.oro js`
- compiler output changed — `examples/native/wordcount-go.oro windows`
- compiler output changed — `examples/native/wordcount-java.oro go`
- compiler output changed — `examples/native/wordcount-java.oro js`
- compiler output changed — `examples/native/wordcount-java.oro windows`
- compiler output changed — `examples/native/wordcount-js.oro go`
- compiler output changed — `examples/native/wordcount-js.oro java`
- compiler output changed — `examples/native/wordcount-js.oro windows`
- compiler output changed — `examples/os/du.oro java`
- compiler output changed — `examples/os/du.oro js`
- compiler output changed — `examples/os/du.oro windows`
- compiler output changed — `examples/smooth.oro go`
- compiler output changed — `examples/smooth.oro java`
- compiler output changed — `examples/smooth.oro js`
- compiler output changed — `examples/smooth.oro windows`
- compiler output changed — `examples/sum/parse.oro java`
- compiler output changed — `examples/sum/parse.oro js`
- compiler output changed — `examples/sum/parse.oro windows`
- compiler output changed — `examples/table/alloc.oro java`
- compiler output changed — `examples/table/alloc.oro js`
- compiler output changed — `examples/table/alloc.oro windows`
- compiler output changed — `examples/table/dot.oro java`
- compiler output changed — `examples/table/dot.oro js`
- compiler output changed — `examples/table/dot.oro windows`
- compiler output changed — `examples/table/sieve-win.oro go`
- compiler output changed — `examples/table/sieve-win.oro java`
- compiler output changed — `examples/table/sieve-win.oro js`
- compiler output changed — `examples/table/sieve.oro java`
- compiler output changed — `examples/table/sieve.oro js`
- compiler output changed — `examples/table/sieve.oro windows`
- compiler output changed — `examples/tally/tally.oro go`
- compiler output changed — `examples/tally/tally.oro java`
- compiler output changed — `examples/tally/tally.oro js`
- compiler output changed — `examples/tally/tally.oro windows`
- compiler output changed — `examples/u128/factorials.oro java`
- compiler output changed — `examples/u128/factorials.oro js`
- compiler output changed — `examples/u128/factorials.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-fib.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-mul.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-negmid.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-select.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-sign.oro windows`
- compiler output changed — `gauntlet/differential/cases/big-subdiv.oro windows`
- compiler output changed — `gauntlet/differential/cases/read-loop.oro windows`
- compiler output changed — `gauntlet/differential/cases/string-escapes.oro go`
- compiler output changed — `gauntlet/differential/cases/string-escapes.oro java`
- compiler output changed — `gauntlet/differential/cases/text-of.oro windows`
- compiler output changed — `gauntlet/differential/cases/u128-fact.oro java`
- compiler output changed — `gauntlet/differential/cases/u128-fact.oro js`
- compiler output changed — `gauntlet/differential/cases/u128-fact.oro windows`
- compiler output changed — `gauntlet/differential/cases/u128-ops.oro java`
- compiler output changed — `gauntlet/differential/cases/u128-ops.oro js`
- compiler output changed — `gauntlet/differential/cases/u128-ops.oro windows`
- compiler output changed — `gauntlet/differential/cases/utf8-widths.oro go`
- compiler output changed — `gauntlet/differential/cases/utf8-widths.oro java`
- compiler output changed — `gauntlet/differential/cases/word-wide.oro windows`
- compiler output changed — `lib/io/go.oro go`
- compiler output changed — `lib/io/go.oro java`
- compiler output changed — `lib/io/go.oro js`
- compiler output changed — `lib/io/java.oro go`
- compiler output changed — `lib/io/java.oro java`
- compiler output changed — `lib/io/java.oro js`
- compiler output changed — `lib/io/js.oro go`
- compiler output changed — `lib/io/js.oro java`
- compiler output changed — `lib/io/js.oro js`
- compiler output changed — `lib/num/u128.oro java`
- compiler output changed — `lib/num/u128.oro js`
- compiler output changed — `lib/num/u128.oro windows`
- compiler output changed — `lib/num/vec.oro go`
- compiler output changed — `lib/num/vec.oro java`
- compiler output changed — `lib/num/vec.oro js`
- compiler output changed — `lib/num/vec.oro windows`
- compiler output changed — `lib/os.oro windows`
- compiler output changed — `lib/os/go.oro go`
- compiler output changed — `lib/os/go.oro java`
- compiler output changed — `lib/os/go.oro js`
- compiler output changed — `lib/os/java.oro go`
- compiler output changed — `lib/os/java.oro java`
- compiler output changed — `lib/os/java.oro js`
- compiler output changed — `lib/os/js.oro go`
- compiler output changed — `lib/os/js.oro java`
- compiler output changed — `lib/os/js.oro js`
- compiler output changed — `lib/result.oro windows`
- compiler output changed — `lib/win/fmt.oro go`
- compiler output changed — `lib/win/fmt.oro java`
- compiler output changed — `lib/win/fmt.oro js`

## 2026-10-20 — on 981e9b4, with uncommitted changes

**Reason:** lib/result.oro and lib/os.oro compiled alone are refused for their own reasons: a library file reached by two roots is read once (no 'declared twice'), a λ argument is no index, an undeclared parameter's reading as a table is named, lowering's messages render in J

560 runs: 301 emitted, 259 refused; 2039 of 2084 integer operations bounded, 352 of 371 loops proven. compiler pass, differential pass, tooling skip.

7 change(s):

- compiler output changed — `lib/os.oro go`
- compiler output changed — `lib/os.oro java`
- compiler output changed — `lib/os.oro js`
- compiler output changed — `lib/result.oro go`
- compiler output changed — `lib/result.oro java`
- compiler output changed — `lib/result.oro js`
- compiler output changed — `lib/result.oro windows`

## 2026-10-21 — on 8fdc434, with uncommitted changes

**Reason:** ls-2026-10-20: examples/os/ls.oro joins (13 of 13 operations, 1 of 2 loops; Go only, as du), and refusal texts quote J: unproven lines by show_J, the NARROW advice in J's header syntax

564 runs: 302 emitted, 262 refused; 2052 of 2097 integer operations bounded, 353 of 373 loops proven. compiler pass, differential pass, tooling skip.

8 change(s):

- compiler output changed — `examples/int/collatz.oro go`
- compiler output changed — `examples/int/fib.oro go`
- compiler output changed — `examples/int/power.oro go`
- compiler output changed — `examples/kara/core.oro go`
- new source — `examples/os/ls.oro go`
- new source — `examples/os/ls.oro java`
- new source — `examples/os/ls.oro js`
- new source — `examples/os/ls.oro windows`

## 2026-10-21 — on 6a83467, with uncommitted changes

**Reason:** shape-2026-10-21: examples/lines/shape.oro joins (5 of 5 operations, 0 of 1 loops; Go only, as lines); J's != is now !(a == b), which no corpus file used

568 runs: 303 emitted, 265 refused; 2057 of 2102 integer operations bounded, 353 of 374 loops proven. compiler pass, differential pass, tooling skip.

4 change(s):

- new source — `examples/lines/shape.oro go`
- new source — `examples/lines/shape.oro java`
- new source — `examples/lines/shape.oro js`
- new source — `examples/lines/shape.oro windows`

## 2026-10-21 — on 6be56d5, with uncommitted changes

**Reason:** observer-2026-10-21: examples/lines/shape.oro reads its own buffer with os.File.Read and bytes.oro's segments (ADR 0069): 13 of 13 operations, parity with the hand-written program; its compile is 1.2 s serial, Houdini over a 12-variable loop. No other emission changed.

568 runs: 303 emitted, 265 refused; 2065 of 2110 integer operations bounded, 353 of 374 loops proven. compiler pass, differential pass, tooling skip.

2 change(s):

- emitted text changed — `examples/lines/shape.oro go`
- compiler output changed — `examples/lines/shape.oro go`

1 compile(s) accepted SLOWER — compile time 0.99x the baseline (median of 10 compiles over 300 ms); a second sweep kept 1 of 1 suspect(s); paired serially against the baseline's binary (6942a00), 1 confirmed:

- `examples/lines/shape.oro go` 125 → 937 ms, 7.50x; paired 78 → 468 ms, 6.00x

## 2026-10-22 — on 24e1e53, with uncommitted changes

**Reason:** inplace-2026-10-22: ADR 0070, a host header is the caller's view of a borrow; a write-borrow is the host's own call, no wrapper, and shape's read loop loses the closure and the buffer's back-edge copy; counts unchanged

568 runs: 303 emitted, 265 refused; 2065 of 2110 integer operations bounded, 353 of 374 loops proven. compiler pass, differential pass, tooling pass.

1 change(s):

- emitted text changed — `examples/lines/shape.oro go`

## 2026-10-22 — on 5f73417, with uncommitted changes

**Reason:** work-2026-10-22: ADR 0071, beta counts evaluations not occurrences, so a pure value bound outside a loop is computed there (freq loses a nested loop; smooth computes len-2 once, 2065->2064 operations); let-floating; a residual let takes beta's rule; the IR reads a measured length as the length; report-go returns its dot product again

568 runs: 303 emitted, 265 refused; 2064 of 2109 integer operations bounded, 353 of 374 loops proven. compiler pass, differential pass, tooling pass.

31 change(s):

- emitted text changed — `examples/big/render.oro go`
- emitted text changed — `examples/big/render.oro java`
- emitted text changed — `examples/big/render.oro js`
- emitted text changed — `examples/big/render.oro windows`
- emitted text changed — `examples/errors/slots.oro go`
- emitted text changed — `examples/errors/slots.oro java`
- emitted text changed — `examples/io/freq.oro go`
- emitted text changed — `examples/io/freq.oro java`
- emitted text changed — `examples/io/freq.oro js`
- emitted text changed — `examples/io/jsonfmt.oro go`
- emitted text changed — `examples/io/jsonfmt.oro java`
- emitted text changed — `examples/io/jsonfmt.oro js`
- emitted text changed — `examples/lines/shape.oro go`
- emitted text changed — `examples/native/centroid-go.oro go`
- emitted text changed — `examples/native/dot-go.oro go`
- emitted text changed — `examples/native/dot-js.oro js`
- emitted text changed — `examples/native/generic-go.oro go`
- emitted text changed — `examples/native/generic-java.oro java`
- emitted text changed — `examples/native/generic-js.oro js`
- emitted text changed — `examples/native/report-go.oro go`
- emitted text changed — `examples/native/smooth-go.oro go`
- compiler output changed — `examples/native/smooth-go.oro go`
- emitted text changed — `examples/native/smooth-js.oro js`
- emitted text changed — `examples/sum/parse.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro go`
- emitted text changed — `gauntlet/differential/cases/render.oro java`
- emitted text changed — `gauntlet/differential/cases/render.oro js`
- emitted text changed — `gauntlet/differential/cases/render.oro windows`
- compiler output changed — `lib/os.oro go`
- compiler output changed — `lib/os.oro java`
- compiler output changed — `lib/os.oro js`

## 2026-10-24 — on 90e1475, with uncommitted changes

**Reason:** wc-2026-10-24: examples/os/wc.oro, a word count over files (Go): unicode.IsSpace, utf8 and File.Read segments (ADR 0074); a where's constant bounds decided as the parameter's range; a length keyed by what it denotes; +36 operations, +2 loops

572 runs: 304 emitted, 268 refused; 2100 of 2145 integer operations bounded, 353 of 376 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- new source — `examples/os/wc.oro go`
- new source — `examples/os/wc.oro java`
- new source — `examples/os/wc.oro js`
- new source — `examples/os/wc.oro windows`

## 2026-10-24 — on 4f401ec, with uncommitted changes

**Reason:** ADR 0076: a case in a tail is a tail when each arm is; wc names and skips an unopenable file in the case's own arm, its junk slot gone (36 -> 39 operations, all proven)

572 runs: 304 emitted, 268 refused; 2103 of 2148 integer operations bounded, 353 of 376 loops proven. compiler pass, differential pass, tooling pass.

2 change(s):

- emitted text changed — `examples/os/wc.oro go`
- compiler output changed — `examples/os/wc.oro go`

## 2026-10-25 — on f40f531, with uncommitted changes

**Reason:** ADR 0077: an unbounded end is omitted; the four windows refusals in examples/big quote the range as 0.. instead of 0..`+inf`

572 runs: 304 emitted, 268 refused; 2103 of 2148 integer operations bounded, 353 of 376 loops proven. compiler pass, differential pass, tooling skip.

4 change(s):

- compiler output changed — `examples/big/fact.oro windows`
- compiler output changed — `examples/big/fib.oro windows`
- compiler output changed — `examples/big/pair.oro windows`
- compiler output changed — `examples/big/power.oro windows`

## 2026-10-25 — on 9bda359, with uncommitted changes

**Reason:** ADR 0078: a library's interface is no longer its entry points; lib/os and lib/result compiled alone have none and emit an empty file, where their whole interface was refused as entry points (err_map's function parameter, err's untyped one)

572 runs: 311 emitted, 261 refused; 2103 of 2148 integer operations bounded, 353 of 376 loops proven. compiler pass, differential pass, tooling skip.

7 change(s):

- now emits — `lib/os.oro go`
- now emits — `lib/os.oro java`
- now emits — `lib/os.oro js`
- now emits — `lib/result.oro go`
- now emits — `lib/result.oro java`
- now emits — `lib/result.oro js`
- now emits — `lib/result.oro windows`

## 2026-10-26 — on d4b3799, with uncommitted changes

**Reason:** ADR 0079: a declared set is a claim, the facts choose the representation; examples/big's results are (Nat), and the four windows refusals print Nat for 0..

572 runs: 311 emitted, 261 refused; 2103 of 2148 integer operations bounded, 353 of 376 loops proven. compiler pass, differential pass, tooling pass.

4 change(s):

- compiler output changed — `examples/big/fact.oro windows`
- compiler output changed — `examples/big/fib.oro windows`
- compiler output changed — `examples/big/pair.oro windows`
- compiler output changed — `examples/big/power.oro windows`
