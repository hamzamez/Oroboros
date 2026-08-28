# A containment harness for the interval analysis — 2026-08-27

`emit/containment_test.go`. Randomly generated programs, run concretely, every claim the analysis
makes checked against what the program actually does. Two halves:

| | programs | checked | claims made |
|---|---|---|---|
| **operations** — `MaxOp`, which decides the index type | 2,000 | 1,877 | — |
| **buffers** — `ElemType`, which decides the element width | 2,000 | 2,000 | 1,490 narrowed |

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

## 1. The two properties

> **γ-soundness (operations).** For every reachable concrete state σ and every integer operation `e`
> evaluated at σ, `⟦e⟧σ ∈ γ(MaxOp)`.

> **γ-soundness (buffers).** For every `build` buffer `b` and every value `v` that can be read out of
> `b`, `v ∈ γ(ElemType(b))`.

`MaxOp` is what index narrowing trusts to hold a counter in the host's own `int`
([indexnarrow](indexnarrow-2026-08-27.md)); `ElemType` is what element narrowing trusts to hold an
element in a byte ([elemwidth](elemwidth-2026-08-27.md)). Neither is checked for **tightness** and
neither should be: the analysis is allowed to be imprecise, and a test demanding precision would
fail on every conservative answer the design exists to give. A claim too wide costs space; one too
narrow is a silent wrong answer.

## 2. Why checking the stores is enough

The buffer property quantifies over *reads*, and the harness checks *writes*. That is not a
shortcut — it is a theorem, and it is the reason the check is one line rather than a shadow heap.

> **Theorem (buffer γ-soundness).** Let `E = ElemType(b) = (int lo hi)`. If `0 ∈ [lo,hi]` and every
> value stored into `b` lies in `[lo,hi]`, then every value ever read out of `b` lies in `[lo,hi]`.

> **Proof.** A slot holds either the zero fill or the value of the most recent `set` into it. There
> is no third source: `build` is the only allocator, `set` the only store, and ADR 0018's linearity
> means no other reference can have written it. Both cases are in `[lo,hi]` by hypothesis. ∎

So the harness checks exactly two things — every stored value, and the zero — and that is
**sufficient**, not merely necessary. Both hypotheses are load-bearing and both are checked
separately, because dropping either is a real bug shape (§4).

## 3. How

Generate a loop in the fragment the analysis actually meets. For operations: one to three variables,
a counter with an exit guard so it terminates, a second clause so refinements have to compose, and
`again` arguments that are pass-throughs, arithmetic, or the running-extremum shape
`(if (> v e) v e)`.

For buffers, the shape every real buffer in this repository has:

```
(build N (fn (b)
   (loop ((b b) (i 0) (a z))
      (if (>= i LIM) exit
          (again (set b idx val) (+ i 1) upd)))))
```

The buffer is a **loop variable**, threaded, because `set` consumes its argument and hands it back
(ADR 0018) — so it is the same buffer at every iteration and linearity holds without the harness
checking anything. Stored values are drawn from both derivations deliberately: literals and
conditionals over literals, which `bufferElem` settles exactly; a value bounded only by the loop
guard (`(* i k)` — the node table's shape), which only `BufferRange` can settle; a **read back out
of the buffer being built**, which must be refused; and a **mixture** of an exact store and an
unanalysable one, which is the shape a rule that forgets one of them gets wrong.

The counter steps by exactly +1 and the buffer is sized `LIM+8`, so every generated index is inside
the domain. Bounds are a **precondition**, not a behaviour (tables.md) — Go panics, Java throws,
JavaScript silently returns `undefined` — so a generator producing out-of-range stores would be
asking the interpreter to invent an answer three hosts disagree on.

The interpreter shares `arithOp` with the analysis, so the two cannot drift about *which* operations
are counted — the only thing they have to agree on.

## 4. The pass condition, and the harness failed it first

Set before the harness was believed: **it must fail when the bug it exists for is put back.** Four
bugs were reintroduced, one at a time.

**The fixpoint bug** — `restore` installing its snapshot by reference — and the first version of the
generator **passed anyway**. That is a harness proving nothing, and the reason is the interesting
part. **Every conditional it produced sat in tail position.** A clause chain is
`(if g₁ exit (if g₂ exit (again …)))`, and the environment after such an `if` is never used again, so
the leak was invisible. The bug bites when an `if` is in **value** position — an operand — where
whatever the analysis believes afterwards is immediately spent on the other operand. With that shape
generated:

```
seed 15: the analysis claims every operation is in -153..765, and one produced 918
```

**The first-store bug** — `bufferElem` reading the width off the first store instead of joining all
of them, which made `tree.oro`'s node table one byte wide and returned `4030140` on windows where
three targets returned `4040171`:

```
seed 6: the analysis claims every element is in "int 0 207", and the program stores 242
  (build 11 (fn (b) (loop (fn (b i a) (if (go.>= i 3) a
    (again (set (set b 1 242) 2 207) (go.+ i 1) a))) b 0 0)))
```

Two stores in one iteration, one 242 and one 207 — the node table's exact shape, found on the sixth
program.

**Dropping the `sawOther` guard**, so a buffer narrows on its exact stores while ignoring the ones
nothing can analyse:

```
seed 10: the analysis claims every element is in "int 0 4", and the program stores 8
```

**Dropping the zero-fill join** in `BufferRange`, which is the theorem's other hypothesis:

```
seed 14: the element range is "int 2 344", which excludes the zero fill
```

Restored, everything passes. Both halves also carry an **anti-rot guard**: the operations half fails
if fewer than 500 of 2,000 programs were genuinely run, and the buffer half additionally fails if
fewer than 200 buffers got a **narrower-than-word** range. That second one matters more than it
looks — refusing to narrow is always sound, so a harness that only ever saw refusals would pass
forever while testing nothing. It has to watch the compiler commit.

## 5. One rule that random search cannot test

A buffer may not narrow on **its own contents**: an element range may be built from a declared range
or from a buffer's *syntactic* one, never from `BufferRange`, which is a fixpoint being fed its own
output.

No random search will produce a counterexample, because a self-reading buffer usually *does* have a
narrow range in fact — the pinning program below stores only 1s. So this is a **policy** test rather
than a soundness one, and it is the kind of rule a refactor that looks like a simplification can
quietly delete. It carries a **control**: the same program storing a literal must narrow to
`int 0 7`, so the refusal is the rule firing rather than the test missing.

## 6. What it does not cover

**Tables read as parameters** — the element range of an `(array (int 0 255))` argument, which is
declared rather than derived, so the check would be against a signature rather than an analysis.

**Non-integer elements** — booleans have their own path in `ElemBytes`.

**Tightness**, and it should not: the analysis is allowed to be imprecise, and a test that demanded
precision would fail on every conservative answer the design is built to give.
