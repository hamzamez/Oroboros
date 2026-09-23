# Why the tokeniser compiles slowly, and half of it bought back

2026-09-23. Plan item 1 of [assessment-2026-09-23](../../docs/assessment-2026-09-23.md): profile the
tokeniser's compile, carried from 09-17, and decide. `emit/refine.go` (the memos), `emit/memo_test.go`,
`cmd/gen -cpuprofile`, and two changes to the compile-time gate (`cmd/check`).

**Before:** the tokeniser compiled in about 720 ms serially, against 108 ms at the 09-13 assessment.
The regression landed at `7e36002` (loop summaries, Fourier–Motzkin entailment), and the last
assessment guessed at Houdini rounds or Fourier–Motzkin sizes. **After:** about 405 ms, with emission
byte-identical on all 472 compiles.

## 1. Where the time goes, measured

`cmd/gen -cpuprofile FILE` is new. Eight profiles of the tokeniser's compile were merged, 6.3 s of
samples:

| cumulative | |
|---:|---|
| 4.59 s | the compile (`main.run`); the rest of the total is GC on other threads |
| 4.20 s | `Refine`, the refinement layer |
| 4.12 s | …`loopInvariants`, the Houdini fixpoint |
| 3.66 s | …`provedBySplit`, one candidate at one back edge |
| 3.54 s | …`facts.entails` |
| 2.00 s | …`farkas`, Fourier–Motzkin elimination |
| 0.65 s | `linear.String`, building cache keys |

So the guess named the right place. But a profile says *where*, not *why*, so the calls were counted
with temporary instrumentation, not committed:

| | depth 0 | depth 1 |
|---|---:|---:|
| Houdini calls | 20 | **285** |
| Houdini rounds | 40 | 390 |
| `provedBySplit` | 1,090 | **11,355** |
| loop-result summaries | 10 | **405** |

There were 20,695 entailments and 5,290 eliminations, averaging 34 facts each.

**The cost is at depth 1: nested loops re-analysed inside every round of the enclosing loop's
Houdini probe.** Each round walks the loop body with a fresh dry refiner, and a fresh refiner has an
empty `bodyFacts`. So every inner loop's fixpoint, and every summary of an inner loop's result,
starts from nothing, once per outer round.

**And most of it is the same computation.** Keyed by loop, initial values and the facts in scope:

| | calls | distinct |
|---|---:|---:|
| loops, depth 0 | 20 | **1** loop, **5** (loop, facts) |
| loops, depth 1 | 285 | **3** loops, **60** (loop, facts) |

## 2. The fix: two memos, each licensed by a theorem

`refineMemo` is shared by a root refiner and every dry walk under it.

**Houdini.** `loopInvariants(lam, inits, f, g)` builds a finite candidate set from those four inputs,
then keeps the greatest subset closed under every back edge. That is a function of the four inputs
and the probe depth, up to renaming of the dry walks' fresh binders. The candidates mention only
`lam`'s parameters, and the key contains `lam`. **So a hit returns exactly what recomputing would.**
The key is `loopKey` (the loop, its starts, f's fingerprint), plus g's fingerprint, plus the depth.

**Result summaries.** `summarizeLoop`'s candidates are 0 ≤ x, x ≤ s and s ≤ x over the loop's frames.
Each is kept iff it is proven at every exit *with the exit's value substituted for x*. So x occurs in
the answer only as a name. The memo stores the answer over the name it was computed for, and a hit
renames it.

| | tokeniser, serial | code, notes |
|---|---:|---|
| before | 716 · 718 · 717 · 713 ms | |
| + Houdini memo | 538 · 531 · 531 · 530 ms | identical |
| + summary memo | **409 · 405 · 402 · 413 ms** | identical |

1.78× on the tokeniser. Across the heavy compiles, same session, alternating binaries:

| | before | after | |
|---|---:|---:|---:|
| jsonfmt, go | 208 · 198 ms | 200 · 207 ms | 1.00× |
| tree, go | 897 · 878 ms | 898 · 876 ms | 1.00× |
| freq, go | 7.35 · 7.37 s | 6.71 · 6.74 s | 1.09× |
| freq, java | 18.4 · 18.3 s | 17.9 · 17.7 s | 1.03× |
| freq, js | 5.40 · 5.39 s | 4.69 · 4.70 s | 1.15× |

## 3. What the check could not see, and the test that can

A memo keyed too coarsely is unsound, so the check was asked whether it could tell. **It could not.**
Two planted keys left all 472 compiles byte-identical:
- the key without g, which turns out to be harmless: in `iterate`, g is built from f, the loop and
  its starts deterministically;
- the key without the starts or f, which is a real bug.

No program in the corpus reaches the same loop under the same facts with different answers. The
memos' soundness rested on the theorem alone. That is *a harness that cannot fail*.

So `emit/memo_test.go` builds the witness where it can be built: one loop at the starts 0, −5, 3, 0,
−5 and 3, under one fact set. Each answer is compared with a refiner that has no memo, and the starts
are chosen so the right answers differ (0 ≤ i holds from 0 and not from −5). A second test asks a
loop's summary for the names a, b, a and compares each with recomputation. **Both tests fail against
their plants**: the key without the starts, and a summary hit without the rename.

## 4. The gate, found wanting from the other side

The compile-time gate only flagged a compile as *slower*. Its baseline moved only when something else
was accepted, so a buy-back could never be locked in, and a regression all the way back would have
passed. Two changes:
- **`faster`, the exact mirror** of `slower`: base ≥ 1.5·now and base − now ≥ 250 ms. It is reported,
  not a failure, and `-accept` records it. `TestTheGateSeesAFixAsWellAsARegression` runs the
  historical sweeps backwards: the same six compiles are called faster, and identical sweeps nothing.
  It fails against the rule with *or* in place of *and*.
- **`-retime`**, because the tokeniser's 1.78× measured **1.48×** in the sweep (1,593 → 1,078 ms)
  on one run. That is below the rule's 1.5, and the rule is symmetric by design. With `-accept
  "reason" -retime` the time baseline is re-recorded from two sweeps and the reason is logged. On
  the run that recorded it, the tokeniser's minimum was 781 ms against 1,593: faster by the rule.

## 5. What is left, and the decision

**Bought back: about half.** The tokeniser is at ~405 ms against ~720 ms, where 108 ms was the
pre-regression cost. **The rest is not redundancy.** The remaining Houdini and summary calls run under
genuinely different facts, one per outer round as candidates are dropped, and there a memo must miss
because the answers can differ. Recovering more would mean changing the algorithm. Two directions
the literature has:
- **incremental Houdini:** an outer round's facts are a subset of the last, so an inner fixpoint could
  restart from the last answer rather than from nothing;
- **not re-walking nested loops in a probe at all:** summarize them once under the weakest facts and
  reuse the result soundly, by monotonicity.

Both are analysis-layer growth, and the plan says that layer grows for a named refusal, not for
speed. **Decision: stop here.** A 405 ms compile is not a refusal.

**freq is a different mechanism**, and it is recorded, not chased. Its 7 s on Go is dominated by the
**interval pass**:
- the buffer-element-width decisions (`elemTypeFixed` → `BufferRange`) run a full interval pass per
  buffer;
- `intervalPass.restore` copies the whole environment at every conditional, 1.24 s of samples, so the
  cost grows with program size.

That is the largest program's cost, and an undo log instead of a copy is the obvious fix. It is
worth doing when a larger program makes it matter. It is not item 1.
