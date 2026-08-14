# Settling the pull → push conversion

[q5b](q5b-filter.md) established two dual collection representations — pull for `zip` and random
access, push for `filter` and friends — and left one thing open:

> How is the pull → push conversion chosen? Either the two families are named differently and the
> programmer converts explicitly, or the type system coerces.

**Answer: one name per operation, representation inferred over a two-point lattice, and the
coercion is asymmetric — the free direction is silent, the costly direction is an error the
programmer must answer. Resolution happens in the type checker, so the reduction relation is
untouched.**

---

## 1. The asymmetry decides it

Everything follows from one fact already measured:

| Direction | Cost |
|---|---|
| pull → push | **free** — drive the fold from the index |
| push → pull | **allocates** — the length cannot be known without running the producer |

A design that treats these the same is wrong in one direction or the other. Make both implicit
and the allocation is silent. Make both explicit and free conversions clutter every pipeline.

> **Coerce the free direction. Refuse the costly one and make the programmer write it.**

That is the same principle as [ADR 0003](../decisions/0003-range-typed-integers.md)'s
"no mystery about what is emitted" and [g6 §9](../derivations/g6-escaping-closures.md)'s cost
report — silence where there is nothing to report, and a name where there is.

## 2. The options that lose

**Explicit families — the programmer writes `push-of-pull`.** Every pipeline carries plumbing
that is free at runtime and expensive to read. Against requirement 7 (more with fewer tokens) and
requirement 8.

**Both directions implicit.** `(zip f (filter p a) b)` silently allocates an *n*-element array.
This is precisely the class of thing the project has spent twelve documents refusing to hide.

**A single representation with a "do I know my length?" flag.** Either the flag is dynamic —
runtime checks in every operation — or it is static, in which case it is a type index and this is
the two-point lattice with extra steps.

**Pull only for concrete arrays, everything derived is push.** Tempting because it needs no
inference. Too restrictive: `(zip f (map g a) b)` is ordinary code and would materialise for no
reason, since `map` preserves random access perfectly well.

## 3. The answer

**One name per operation.** The programmer writes `map`, `filter`, `zip`, `sum`. The words `pull`
and `push` never appear in user code.

**Representation is a type**, inferred over a two-point lattice with a single directed edge:

```
Pull  ⟶  Push          (free, inserted silently)
Push  ⟶  Pull          (allocates, never inserted)
```

Operations constrain it:

| Operation | Requires |
|---|---|
| `zip`, `index`, `len`, `reverse` | **Pull** |
| `filter`, `concat`, `flat-map` | **Push** |
| `map`, `sum`, `fold`, `any` | either — propagates its neighbour's choice |
| array literal, `of-array` | produces **Pull** |

Inference is unification over two points with one coercion edge. Small, and much smaller than
Hindley–Milner — but it is real machinery and should be counted as such.

**Resolution happens in the type checker, before reduction.** Once `map` has resolved to
`pull-map` or `push-map`, every name is monomorphic and δ is name-directed exactly as
[core-0](core-0.md) specifies. **The atom does not change**, and overloading does not leak into
the reduction relation.

## 4. Worked

```lisp
(sum (filter pos (map sqrt xs)))
```

`filter` requires Push. `map` propagates, so it is `push-map`. `xs` is an array, so Pull, and a
free coercion is inserted at the source. Fuses to one loop, no allocation.

```lisp
(dot a b)   ; = (sum (zip mul a b))
```

`zip` requires Pull. `a` and `b` are arrays. No coercion. Fuses to
[g1](../derivations/g1-dot-product.md)'s residual.

```lisp
(zip f (filter p a) b)
```

`zip` requires Pull; `filter` produces Push. No edge exists. **Error** — and the error is the
useful part:

```
zip needs random access, but `filter` cannot provide it: filtering
destroys the index correspondence, so the length is not known until
the data is scanned.

  (zip f (filter p a) b)
         ^^^^^^^^^^^^

Write `(materialize (filter p a))` to allocate the intermediate
array. Cost: 1 allocation, up to (len a) elements.
```

That is not a compiler being obstructive. Hand-written Go materialises at exactly this point too
— it has no choice either. The compiler is naming a cost the language did not invent.

## 5. The check that makes this safe

**The pull/push distinction has no runtime existence.** Both are λ-terms; both are grade 0; both
reduce away entirely. The residual contains neither a pull array nor a push array — only loops
and primitives.

So the representation choice cannot cost anything at runtime **except** through the one
materialise edge, which is explicit by construction. The inference can be wrong about
ergonomics; it cannot be silently wrong about performance.

That is worth stating as the safety property:

> Every allocation the collection library can cause is named in the source.

## 6. Findings

1. **The asymmetry decides the design.** Free direction coerced, costly direction refused.
2. **One name per operation**; `pull` and `push` never appear in user code.
3. **Inference is unification over two points with one directed edge** — real machinery, and
   small.
4. **Resolution is a type-checking phase, before reduction**, so overloading never enters the
   reduction relation and the atom is unchanged.
5. **The error message is the feature.** It names the cost, the reason, and the fix.
6. **The distinction is entirely compile-time**, so the only reachable allocation is the explicit
   one.

## 7. What this opens

Pull and push are one instance of a general shape: **two representations of the same abstract
thing, each better at different operations, with a directed conversion between them.** Strings
have it (rope versus flat), maps have it (association list versus hash), matrices have it (dense
versus sparse).

If the answer here generalises, "representation selection with asymmetric coercion" is a
mechanism rather than a special case for collections — which would be worth having, since
[ADR 0002](../decisions/0002-capability-graph.md) already says a capability may have several
implementations.

**Not settled, and the next thing to look at in this thread.** The risk is that the two-point
lattice does not survive contact with a third representation, and the whole thing becomes general
subtyping — which is much larger machinery than anything currently in the design.
