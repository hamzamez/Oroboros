# A containment harness for the interval analysis — 2026-08-27

`emit/containment_test.go`. 1,877 randomly generated programs, run concretely, every integer
operation checked against what the analysis claimed.

It exists because [fixpoint-2026-08-27](fixpoint-2026-08-27.md) found a soundness bug that had been
there since the analysis was written and was found **by accident** — a change to something else
happened to expose it. Three things make that worth acting on rather than shrugging at.

**The analysis decides bits.** Element width and index type both read its intervals. A wrong one is
a wrong answer, not a slow program.

**The differential suite is structurally blind to it.** Every target narrows on the same decision,
so they agree and are wrong together. Only a hand-computed `; expect:` can catch it.

**Hand-written adversarial tests are hard to get right.** `TestIntervalsNeverOverclaim` exists, is
hand-written, and **passed for months while the fixpoint was unsound**. Two of the adversarial cases
written for `BufferRange` the same week expected a refusal and got a correct claim — the tests were
wrong, not the analysis.

---

## 1. The property

> **γ-soundness.** For every reachable concrete state σ and every integer operation `e` evaluated at
> σ, `⟦e⟧σ ∈ γ(MaxOp)`.

`MaxOp` is the join of every *checkable* operation's abstract result, and it is exactly what index
narrowing trusts. The test is **containment, never tightness**: a claim that is too wide costs
space, one that is too narrow is a silent wrong answer.

## 2. How

Generate a loop in the fragment the analysis actually meets — one to three variables, a counter with
an exit guard so it terminates, a second clause so refinements have to compose, and `again`
arguments that are pass-throughs, arithmetic, or the running-extremum shape `(if (> v e) v e)`. Run
it with a direct interpreter that records every arithmetic result. Assert containment.

The interpreter shares `arithOp` with the analysis, so the two cannot drift about *which* operations
are counted — which is the only thing they need to agree on.

## 3. The pass condition, and the harness failed it first

Set before the harness was believed: **it must fail when the fixpoint bug is put back.**

Reverting `restore` to install its snapshot by reference — the original bug — and it **passed
anyway**. That is a harness proving nothing, and the reason is the interesting part.

**Every conditional the first generator produced sat in TAIL position.** A clause chain is
`(if g₁ exit (if g₂ exit (again …)))`, and the environment after such an `if` is never used again,
so the leak was invisible. The bug bites when an `if` is in **value** position — an operand — where
whatever the analysis believes afterwards is immediately spent on the other operand.

With conditionals generated in value position too:

```
seed 15: the analysis claims every operation is in -153..765, and one produced 918
```

Restored, 1,877 of 2,000 programs check clean. The 123 skipped are ones that ran out of fuel or left
the fragment, and the harness fails if fewer than 500 are genuinely checked — a generator that
quietly stops generating anything runnable is the other way a test like this rots.

## 4. What it does not cover

**Buffers.** The element range is the other place an interval decides bits, and a wrong one there
truncates stored data rather than merely widening it. Generating `build`/`set` and checking every
stored value against `ElemType`'s answer is the same property one level along, and it is the honest
next step.

**Tables and their reads**, for the same reason.

It also does not check *tightness*, and should not: the analysis is allowed to be imprecise, and a
test that demanded precision would fail on every conservative answer the design is built to give.
