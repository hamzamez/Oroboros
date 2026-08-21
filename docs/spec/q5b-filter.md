# Does `filter` break the δ+β result?

[q5](q5-do-we-need-rules.md) concluded that δ over `def` covers all lowering *and* fusion, given
a delayed vector representation — then flagged its own weak point:

> `filter` is the test to do next — it cannot be a pull array with a static length, and if the
> delayed representation cannot express it then fusion may need rules after all.

**Result: the concern is real and the conclusion survives. `filter` genuinely defeats pull
arrays, and is handled by a second representation that is still pure δ+β. The one composition
that costs an allocation costs one in hand-written code too.**

---

## 1. Confirming the failure

A pull array is `(length, index → element)`. `filter` drops elements, so
`(vlen (filter p xs))` cannot be computed without running `p` across all of `xs` — the length is
data-dependent.

There is no encoding that fixes this. A pull array must answer "how long are you?" before
anything reads it, and a filter cannot answer.

**Pull arrays cannot express `filter`.** Confirmed.

## 2. The other representation

Represent a collection as **its own fold** — a producer that accepts a consumer. This is the
`build` side of foldr/build, and a push array in the Feldspar/Obsidian sense.

```lisp
;; a push collection is (fn (step z) …)
(def push-of-array (fn (a)   (fn (step z) (fold-range z (alen a) (fn (acc i) (step acc (aindex a i)))))))
(def push-filter   (fn (p c) (fn (step z) (c (fn (acc x) (if (p x) (step acc x) acc)) z))))
(def push-map      (fn (f c) (fn (step z) (c (fn (acc x) (step acc (f x))) z))))
(def push-sum      (fn (c)   (c (fn (acc x) (add acc x)) 0.0)))
```

Nothing needs a length, because nothing asks for one.

## 3. Hand-reducing `(push-sum (push-filter pos (push-of-array a)))`

Writing `S` for `(fn (acc x) (add acc x))`:

```
⟶δ  ((push-filter pos (push-of-array a)) S 0.0)
⟶δ  (((fn (p c) (fn (step z) (c (fn (acc x) (if (p x) (step acc x) acc)) z)))
       pos (push-of-array a)) S 0.0)
⟶β  ((fn (step z) ((push-of-array a) (fn (acc x) (if (pos x) (step acc x) acc)) z)) S 0.0)
⟶β  ((push-of-array a) (fn (acc x) (if (pos x) (S acc x) acc)) 0.0)
⟶δβ (fold-range 0.0 (alen a) (fn (acc i) ((fn (acc x) (if (pos x) (S acc x) acc)) acc (aindex a i))))
```

The next β would substitute `(aindex a i)` for `x`, which occurs **twice** — in the predicate and
in the accumulation. `(aindex a i)` is a primitive application, not a literal or a variable, so
[core-0 §3](core-0.md) requires a let-binding rather than a copy:

```
⟶β  (fold-range 0.0 (alen a)
       (fn (acc i) (let ((x (aindex a i))) (if (pos x) (S acc x) acc))))
⟶β  (fold-range 0.0 (alen a)
       (fn (acc i) (let ((x (aindex a i))) (if (pos x) (add acc x) acc))))
```

Normal form, and **exactly the hand-written filtered sum**: one pass, no intermediate array, one
index per element.

Worth noting what just happened: **[g4](../derivations/g4-word-count.md)'s let-binding discipline
was load-bearing here.** Without it the residual would call `aindex` twice per element — a
silent 2× on the hot loop. That rule was derived from a word-count program and it fired correctly
in an unrelated encoding, which is some evidence it is the right rule rather than a patch.

## 4. The dual failure

Push arrays cannot express **`zip`**. Each push collection drives its own iteration, and two
independent folds cannot be stepped in lockstep without one of them being materialised.

So the two representations are exact duals:

| | length & random access | `zip`, `index`, `reverse` | `filter`, `concat`, `flat-map` |
|---|---|---|---|
| **Pull** — `(length, index→elem)` | yes | **yes** | **no** |
| **Push** — `(fn (step z) …)` | no | **no** | **yes** |

## 5. Conversion, and where the cost is real

```lisp
(def push-of-pull (fn (v) (fn (step z) (fold-range z (vlen v) (fn (acc i) (step acc (vindex v i)))))))
```

**pull → push is free** — drive the fold from the index. So `filter` applied to a `zip` is fine:
convert at the filter, and everything downstream is push.

**push → pull must materialise.** There is no way to answer "how long are you?" without running
the producer, so the conversion allocates an array. That is not a defect of the encoding — it is
the operation.

And it is the honest comparison, because **hand-written code materialises at exactly the same
point.** To zip a filtered slice with another in Go:

```go
filtered := make([]T, 0, len(a))
for _, x := range a { if p(x) { filtered = append(filtered, x) } }
// only now can it be zipped with b
```

The index correspondence is destroyed by filtering, so the array has to exist. **Parity holds at
the point where the cost appears.**

## 6. The elegant unification costs more, not less

Stream fusion (Coutts, Leshchinskiy, Stewart) handles both with one representation, by adding a
`Skip` constructor: `Step = Done | Yield elem state | Skip state`. Filtering yields `Skip` for
rejected elements, and `zip` works because the consumer controls stepping.

It does not help here. Church-encoding `Step` makes matching into application, and β eliminates
it **only when the constructor is statically known at the match site**. In `filter`'s step
function the result is `(if (p x) (Yield …) (Skip …))` — a conditional, so the constructor is not
known, and β cannot fire through it.

Making it fire needs `((if c A B) args) ⟶ (if c (A args) (B args))` — **case-of-case**, which is
shape-directed and therefore a rule, not δ. And unlike SROA it would be load-bearing for the
entire collection library rather than one corner.

> **The unified representation is more elegant and needs strictly more machinery. Two
> representations need none.**

> **Revisited 2026-08-21 — [sums-research.md §0.1](../sums-research.md).** The rule this section
> rejected is load-bearing somewhere else. For a **sum**, `((if c A B) F G)` is exactly what a
> dynamically-tagged Church encoding gets stuck on, and case-of-case unsticks it **completely** —
> no closure survives, no tag is built, nothing is allocated. So the same rule that was too much
> machinery for the collection library is what would make a locally-consumed `Result` free, exactly
> as β makes a locally-consumed product free.
>
> The rejection here still stands on its own terms — two representations remain cheaper than
> unifying pull and push. What changes is that the rule may arrive anyway, for a different reason,
> and if it does this section's cost/benefit should be re-run rather than assumed.

That is the same shape as GHC's experience, where stream fusion is more general than foldr/build
and correspondingly more fragile.

## 7. Findings

1. **Pull arrays genuinely cannot express `filter`.** The flagged concern was correct.
2. **Push arrays can**, and hand-reduction produces exactly the hand-written loop — one pass, no
   intermediate, one index per element.
3. **Push arrays cannot express `zip`.** Exact dual.
4. **pull → push is free; push → pull materialises**, and hand-written code materialises at the
   same point, so parity holds.
5. **g4's let-binding discipline was load-bearing** in an encoding it was not derived from.
6. **Stream fusion would unify them and would require case-of-case**, a shape-directed rule
   needed by the whole collection library. The two-representation answer is cheaper.
7. **q5's conclusion survives.** Two representations are more *library*, not more *core*. δ+β+`P`
   is unchanged, and rules remain scoped to residual-to-residual transformation.

## 8. What this opens

~~**How is the pull → push conversion chosen?**~~ **Settled** —
[q5c-representation-choice.md](q5c-representation-choice.md). One name per operation, with
representation inferred over a two-point lattice. The asymmetry decides the design: **pull → push
is free and coerced silently; push → pull allocates and is refused**, so the programmer writes
`materialize` and the compiler names the cost at the exact site. Resolution happens in the type
checker, before reduction, so the atom is untouched.

The safety property that falls out: both representations are grade 0 and reduce away entirely, so
**every allocation the collection library can cause is named in the source.**

Which is [g6 §9](../derivations/g6-escaping-closures.md)'s cost report arriving as a type error
instead of a profile.
