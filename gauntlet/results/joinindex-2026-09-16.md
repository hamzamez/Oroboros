# An index that is a term is proven branch by branch: 298 propagated obligations become 105

2026-09-16. `emit/refine.go` (`provedThroughJoin`), `emit/inductive_test.go`.

inductive-2026-09-16 §6 named the next measurement. The tally obligations still propagated were one class: **an index
that is a `let` or conditional TERM**, outside the linear fragment as written. The reading that proved the loop
invariant's back-edge argument applies to them verbatim.

## 1. The reading

An index term `e` of the form `(let … (fn (x) (if c₁ … ℓ₁ … ℓₘ)))` denotes **one** value. Read it as a fresh name `ι`
and give `ι` what every branch satisfies:

```
if  Pⱼ ⊢ φ(ℓⱼ)  for every leaf j,  then  F ⊢ φ(ι)
```

Exactly one leaf is evaluated, on a path whose facts hold. That is `joinConditional`'s theorem (Sankaranarayanan,
Sipma & Manna's template join), unchanged, now including the equation `x = v` for every `let` whose value is linear.
The domain obligation `0 ≤ e < len t` is then asked of `ι`.

## 2. The first build refused working programs, and that decided the design

Read as a **widening of the fragment**, an index that became readable and could not be proven became a **refusal**.
purecontract-2026-09-15 predicted that gate. Measured over the whole corpus:

| program | before | read as a widening |
|---|---|---|
| `json/tokenize.oro` | 10 propagated | **0 — all proven** |
| `kara/core.oro` | 6 propagated | **0 — all proven** |
| `io/freq.oro` (three hosts) | 80 propagated each | **refused** |
| tally | 58 propagated | **refused** |

freq's and tally's refusals are **true obligations that no fragment here can prove**. A clamp `(if (< i 0) 0 (if (>= i
n) 0 i))` is in range only when the table is non-empty, and each program's table is non-empty only because a loop that
indexes it runs at all: `nd ≥ 1` implies `n ≥ 1` through `nd ≤ n`, a fact about a loop's RESULT. Refusing a correct
program over an incompleteness is not what "an undischarged obligation is reported, never assumed" asks for, because
propagation IS the report.

**So the join is a PROOF ATTEMPT for a term outside the fragment, not a wider fragment.** A success removes the note.
A failure leaves exactly the propagation there was. The proven set only grows and the refused set is unchanged, which
makes the change monotone in the order that matters.

## 3. Measured

- **Propagated obligations across the corpus: 298 → 105.** freq 80 → 21 on each of three hosts, tokenize 10 → 0,
  kara/core 6 → 0, tally (built against generated declarations) 58 → 6.
- **Every emitted file byte-identical** by hash, **no refusal changed**, proof counts unchanged: 2307 of 2413
  operations, 345 of 382 loops.

`TestAConditionalIndexIsProvenBranchByBranch` covers:

- a clamp into a non-empty table, which must be proven;
- a conditional with a branch outside the table, which must stay propagated rather than proven or refused;
- a clamp into a possibly empty table, which must stay propagated.

The first case fails with the attempt removed.

## 4. What is left is ONE class, and it is not relational

Of the 105:

| where | count | index |
|---|---|---|
| `json/tree.oro` | 40 | `(nodes k)`, k read out of the node table itself |
| `io/freq.oro` × 3 | 17 each | `(src …)` at a word offset read out of the span table `sp` |
| `io/freq.oro` × 3 | 4 each | `(dt …)` at a position read out of a table |
| `native/smooth-js.oro` | 2 | JavaScript's `/`, a refinement rather than an index |

Every index obligation left is **an index computed from a value read out of a table**. What bounds it is an invariant
over the table's CONTENTS: every offset stored in `sp` is below `len src`, and every link in `nodes` is below `nn`. That
is a quantified statement over slots, **facts.md's F-D, the array property fragment** (Bradley, Manna & Sipma, VMCAI
2006). frozen-2026-08-28 and maxlen-2026-08-28 reached this class from the other direction, and it is not an octagon.
The corpus now isolates it: **nothing else remains.**

## 5. Cost

- `emit/refine.go` 945 → 972 code lines, **+27**.
- Every check step passes, differential and tooling included.
