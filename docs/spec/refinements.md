# Refinements

Written before the code, per [state.md §6](state.md). Layer 1's second half; the first half is
[types.md](types.md).

> **Status, 2026-08-15. Built.** `emit/linear.go` and `emit/refine.go`. `aindex` carries its
> bounds refinement on Go, and every obligation in every example is discharged.
>
> **It found a real latent bug in two of our own gauntlet programs on the day it was written.**
> `dot` and `centroid` each index *two* arrays with one loop bounded by the *first* — nothing
> related the lengths, so `(aindex q i)` was genuinely unproven. Both now declare
> `(where (int.eq (alen p) (alen q)))`, which is the precondition moving to the caller exactly as
> [types-sketch §2](../types-sketch.md) described.
>
> Two things the implementation taught. An **equality must be a substitution, not two
> inequalities** — that is what lets a fact about `p` discharge an obligation about `q`. And
> entailment needed **sums of two facts**: `i < alen p` plus `alen p ≤ alen q` gives `i < alen q`,
> a shape one fact can never reach and every two-array loop produces.

---

## 1. What this is for

Two holes in the specification are **shaped exactly like a refinement**, and both currently say
*stay inside, nothing checks it*:

| | the obligation | what happens outside it |
|---|---|---|
| [primitives.md §2](primitives.md) | `0 ≤ i < alen v` for `aindex` | Go panics, Java throws, **JS silently returns `undefined`** |
| [arithmetic.md §4](arithmetic.md) | `-(2⁵³−1) ≤ n ≤ 2⁵³−1` | Go and the JVM wrap, **JS silently rounds** |

The first is the one to close. It is the only place in the language where a *Tier 1* primitive is
Tier 1 only conditionally, and the condition is unchecked.

**This is a correctness deliverable, not a speed one.** The bounds-check *performance* win was
already collected as an emitter pattern with no types at all
([bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md)), and types-direction §2.2 was explicit
that the type system must not be justified by cases the emitter can already see.

## 2. Syntax

A `where` clause on a `sig` or a `prim`, holding an ordinary **boolean term** over the parameter
names:

```lisp
(sig dot ((a vec-f64) (b vec-f64)) f64
  (where (int.eq (alen a) (alen b))))

(prim aindex ((v vec-f64) (i int)) f64 expr "%s[%s]" pure index
  (where (logic.and (int.le 0 i) (int.lt i (alen v)))))
```

On a `prim`, `where` is a **trailing attribute** beside `pure` and `import`, because that is where
the grammar already puts them ([target-files.md §1](target-files.md)). On a `sig` it follows the
result type. The named argument form `((v vec-f64) (i int))` is now accepted by `prim` as well as
`sig`, so the two have not diverged.

There is **no predicate syntax**. A refinement is a term of type `bool` — the predicate language
is a *fragment of the term language*, which is why parameters were named in
[types.md §7](types.md) before anything read them.

## 3. Classify, do not restrict

> **Any boolean term may appear in a `where`. A term inside the decided fragment is *proven*. A
> term outside it is an *opaque atom* — propagated and matched by name, never decided.**

Consequences, all of them wanted:

- Nothing is rejected for being too expressive.
- It is **always sound**: an undecided obligation is not assumed true. It can only be discharged by
  an assumption that matches it, or by a runtime check at a boundary.
- It **degrades gracefully**, and the fragment can grow later without any program changing.

## 4. The fragment

**Linear integer arithmetic over difference constraints**, which is what every bounds obligation
actually is.

A term is *linear* if it is built from integer literals, names, `int.add`, `int.sub`,
multiplication by a literal, and **opaque length terms** like `(alen v)`, which are treated as
variables. Comparisons `int.lt`, `int.le`, `int.gt`, `int.ge`, `int.eq` over linear terms are
atoms; `logic.and` joins them.

Everything else — `f64` comparison, `ascii?`, a call to a defined function — is an **opaque atom**,
matched by syntactic identity and nothing more. `num/f64.eq` is opaque *by necessity*: IEEE
equality is not reflexive, so it is not an equivalence relation and a solver must not treat it as
one ([arithmetic.md §6](arithmetic.md)).

### Entailment

Facts are normalised to `e ≤ 0` where `e` is a linear expression. An obligation is discharged if,
after substituting known equalities, it is **implied by a single fact**: same variable part, and a
constant offset in the right direction.

Deliberately **incomplete**. It is not Fourier–Motzkin and it is not an SMT solver. It is the
smallest thing that decides the obligations this language actually generates, and being incomplete
is safe — an undischarged obligation is *reported*, never assumed.

## 5. Where facts come from

| | fact |
|---|---|
| `(fold-range z n f)` binding `i` | `0 ≤ i` and `i < n` |
| `(make-vec n f)` binding `i` | `0 ≤ i` and `i < n` |
| a `let` binding `x` to a linear term `e` | `x = e` |
| the enclosing `sig`'s own `where` | assumed |
| `alen`, `slen` | `0 ≤ alen v` |

That last one is free and worth having: a length is never negative on any target.

## 6. The diagnostic

An obligation that cannot be discharged is an **error**, naming the obligation and what was known:

```
smooth: (aindex a (int.add i 2)) requires i + 2 < alen a
  known: 0 <= i, i < alen a - 2
```

And the softer case — a refinement that was *propagated* rather than *proven* — must be reportable
too, because [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already established that a
transformation which silently does not fire is indistinguishable from one that does.

## 7. What this does not do

- **No solver for non-linear arithmetic.** Undecidable over the integers (Hilbert's tenth), and no
  obligation needs it.
- **No quantifiers.** `∀i. a[i] > 0` is a whole-array property; the fragment is per-index.
- **No proofs.** Layer 2 ([types-sketch §7](../types-sketch.md)).
- **No refinement inference.** A `where` is declared, never guessed. Liquid Types infers them by
  Horn-clause fixpoint; that is a larger machine and no program has asked.
- **Not applied to the integer range yet.** §1's second hole needs `int` literals to carry ranges,
  which touches every arithmetic primitive. One hole at a time.
