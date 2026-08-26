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

## 6b. A `where` on a DEFINITION is a different thing from one on a `prim`

Found by the differential suite, 2026-08-26
([differential-2026-08-26 §4](../../gauntlet/results/differential-2026-08-26.md)), and the answer
is more interesting than the finding.

**A primitive's `where` is discharged at every call site.** That is §5, it is what `aindex` and
`go./` rest on, and it is unchanged.

**A definition's is not — because there is no call site left.** Reduction inlines every
non-exported call, so by the time the residual reaches `Refine` there is no `safe`, no `n`, and
nothing to attach an obligation to:

```lisp
(sig safe ((n int)) int (where (<= 0 n)))
(def safe (fn (n) (go.+ n 1)))
(def f (fn (x) (safe (go.- 0 5))))        ; accepted
```

### The `where` is DROPPED, not assumed

This is the difference between a missing check and an unsound one, and it is the first thing to
establish. Nothing assumes `0 ≤ n` on the strength of the declaration; the clause simply does not
participate. So a program is never *told* something false.

### Inlining is the enforcement mechanism, and it is STRONGER than the declaration

What actually protects the program is that the obligations *inside* the body land at the call site
with the caller's own values:

```lisp
(sig safe ((a (array f64)) (n int)) f64 (where (and (<= 0 n) (< n (len a)))))
(def safe (fn (a n) (a n)))
(def f (fn (a) (safe a (go.- 0 5))))
```

```
(a (go.- 0 5)) is an indexing, and (<= 0 (go.- 0 5)) does not follow
```

Refused — not because of the `where`, but because `(a -5)` is in the residual.

And the declared clause is only ever a **summary**, so checking it instead would be *less* precise.
A `where` of `(< n 100)` on a body that really needs `n < len a` rejects a legal `(get a 400)`
against a 500-element array; the propagated obligation accepts it, because it is the truth rather
than a conservative restatement of it.

> **So a naive fix is a regression.** Enforcing a definition's `where` at call sites would reject
> programs that are correct and currently compile. The declaration is documentation *plus* a
> conservative summary; the check is the propagated obligation.

### Except where the precondition states MEANING rather than guarding an obligation

The one case inlining cannot reach is a body that is **total** and merely *wrong* outside its
domain, because nothing fires. `lib/win/fmt.oro` is the instance:

```lisp
(sig print-int ((n int)) any (where (and (<= 0 n) (< n 9007199254740991))))
```

The digit loop exits immediately on `(x64.setg m 0)` for a negative `n` and writes the one byte it
had already stored — so `print-int -13` prints a blank line. That is not a bug in the
implementation: the declaration says it makes no claim there. It is a precondition with **no
enforcement anywhere**, and the differential suite found it by printing a negative number.

This is a real gap and a named one. It is the same shape as SAL's `_Success_` and
`_Ret_maybenull_` — a contract about what a call *means*, not about what it may touch — which is
the territory [general-purpose.md](../general-purpose.md) is already heading into for Win32. The
difference is that SAL contracts sit on *primitives*, where `where` is enforced, and this one sits
on a definition.

### And an EXPORTED definition's `where` means a third thing

For an exported function the caller is **outside the program**, so nothing in the program could
check it. There it is *assumed*, and correctly: it is a published contract, exactly what SAL is
for a C header. `Refine` assumes it so the body may rely on it.

So one syntax carries three meanings, which nothing said until now:

| on | meaning |
|---|---|
| a `prim` | an obligation, discharged at every call site |
| an **exported** definition | a published contract, assumed; the caller is outside the program |
| an **internal** definition | a summary — dropped, with the body's own obligations doing the work |

The third is sound for everything that guards an obligation and empty for everything that states a
meaning, and telling those apart is the open question.

## 7. What this does not do

- **No solver for non-linear arithmetic.** Undecidable over the integers (Hilbert's tenth), and no
  obligation needs it.
- **No quantifiers.** `∀i. a[i] > 0` is a whole-array property; the fragment is per-index.
- **No proofs.** Layer 2 ([types-sketch §7](../types-sketch.md)).
- **No refinement inference.** A `where` is declared, never guessed. Liquid Types infers them by
  Horn-clause fixpoint; that is a larger machine and no program has asked.
- **No enforcement of a precondition that states MEANING.** §6b: a definition whose body is total
  and merely wrong outside its domain has nothing to catch. `win/fmt.print-int` is the instance.
- **Not applied to the integer range yet.** §1's second hole needs `int` literals to carry ranges,
  which touches every arithmetic primitive. One hole at a time.
