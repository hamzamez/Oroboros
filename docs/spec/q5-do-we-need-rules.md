# Settling open question 5: does δ over `def` remove the need for rules?

[core-0 §7](core-0.md) asks whether the language needs a separate `rule` construct, or whether
δ-reduction over `def` already covers every lowering in the derivations. If δ suffices, a whole
layer of machinery disappears and the `?x` metavariable sigil is unnecessary.

**Settled on paper. Answer: δ covers more than expected — including fusion, which was not
obvious — but there is a counterexample, and it is sharp enough to define exactly what the extra
construct is for.**

---

## 1. Why this one settles without building

Five predictions in this project have been refuted by measurement: Go's inliner, JS's `Map`,
Java's allocator, HotSpot's object layout, the bounds-check win. **Every one was a fact about
someone else's compiler.** No amount of reasoning reaches those.

*"Can this transformation be expressed in that grammar"* is a different kind of question. It is
formal: either the encoding can be exhibited, or a term can be exhibited that defies it. Paper is
the correct instrument, and building would not make the answer more reliable — only faster to
check exhaustively (see §6).

**Method:** enumerate every lowering in the six derivations, attempt each as a `def`, and
characterise the failures.

## 2. What δ handles trivially

| From | Rule | As a definition |
|---|---|---|
| [g4](../derivations/g4-word-count.md) | `(tally ?seq) => (for-each-into …)` | `(def tally (fn (seq) (for-each-into …)))` |
| [g1](../derivations/g1-dot-product.md) | `(dot ?a ?b) => (sum (zip mul ?a ?b))` | `(def dot (fn (a b) (sum (zip mul a b))))` |
| [g4](../derivations/g4-word-count.md) | `(for-each-into …) => (block …)` | `(def for-each-into (fn (init seq step) …))` |
| [g2](../derivations/g2-structs.md) | dict/string layers on C | ordinary definitions |

Every rule whose left-hand side is **a name applied to arguments** is a definition. That is the
entire capability graph — all of [ADR 0002](../decisions/0002-capability-graph.md)'s lowering,
and both of its directions, with no pattern matching anywhere.

## 3. Fusion: δ handles it too, which was not obvious

The apparent counterexample is [g1](../derivations/g1-dot-product.md)'s fusion rule:

```lisp
(rule (sum (zip ?f ?a ?b)) => (fold-range …))
```

The head is `sum`, but it fires only when the **argument has a particular shape**. A definition
of `sum` fires on any argument. So this looks like it needs pattern matching.

It does not — if a vector is represented as **a length paired with an index function** rather
than as data. This is the delayed or pull-array representation (Repa, Accelerate, Halide), and
the fusion technique is foldr/build (Gill, Launchbury, Peyton Jones).

```lisp
(def vec     (fn (n f) (fn (sel) (sel n f))))
(def vlen    (fn (v)   (v (fn (n f) n))))
(def vindex  (fn (v i) ((v (fn (n f) f)) i)))

(def of-array (fn (a) (vec (alen a) (fn (i) (aindex a i)))))
(def zip      (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))
(def sum      (fn (v) (fold-range 0.0 (vlen v) (fn (acc i) (add acc (vindex v i))))))
```

Reducing `(sum (zip mul (of-array p) (of-array q)))` — the length first:

```
(vlen (of-array p))
⟶δ  (vlen (vec (alen p) (fn (i) (aindex p i))))
⟶δ  ((vec (alen p) (fn (i) (aindex p i))) (fn (n f) n))
⟶δ  (((fn (n f) (fn (sel) (sel n f))) (alen p) (fn (i) (aindex p i))) (fn (n f) n))
⟶β  ((fn (sel) (sel (alen p) (fn (i) (aindex p i)))) (fn (n f) n))
⟶β  ((fn (n f) n) (alen p) (fn (i) (aindex p i)))
⟶β  (alen p)
```

And the element, by the same chain: `(vindex Z i) ⟶* (mul (aindex p i) (aindex q i))`.

Assembling:

```lisp
(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))
```

**That is exactly g1's residual**, and no intermediate vector appears anywhere in it. Fusion, by
β and δ alone.

Note `Z` occurs twice and is duplicated freely rather than let-bound — sound because `Z` is grade
0, so duplicating it costs compiler time and nothing else. [g4](../derivations/g4-word-count.md)'s
let-binding discipline applies to runtime terms.

### This is stronger here than in Haskell

GHC's foldr/build fusion is famously fragile: it depends on the inliner firing, and it silently
fails to fire. **Here β at compile time is unconditional** — reduction to normal form is the
definition of compilation, not an optimisation that may or may not happen. The technique that is
best-effort in Haskell is total here.

### And it collapses g1's termination taxonomy

[g1 §5](../derivations/g1-dot-product.md) established three termination classes and identified
same-layer *deforestation* rules as the ones needing a structural measure. **If fusion is δ+β,
those rules do not exist.** Termination reduces to: δ terminates because non-recursive definitions
form a DAG, and β terminates because no λ is self-applying. The measure check leaves the
machinery list.

## 4. The counterexample

Scalar replacement of a **loop-carried** accumulator, from
[g2](../derivations/g2-structs.md).

```lisp
(fold-range (struct 0.0 0.0) n
  (fn (acc i) (struct (add (field 0 acc) x) (add (field 1 acc) y))))
```

`fold-range` is **primitive** — it is the loop, it is in `P`, and reduction stops at it. Its
function argument becomes the loop body, so `acc` is a *bound variable of a surviving
abstraction*, not an application of `struct`.

Therefore `(field 0 acc)` has no β-redex. There is nothing to unfold and nothing to substitute.

Church-encoding the struct does not help: `(field 0 acc)` becomes `(acc (fn (a b) a))`, and `acc`
is still opaque. Defining `fold-range` rather than making it primitive would help only if `n`
were known at compile time, which in general it is not.

The transformation needed — split the loop-carried variable into one variable per field — is a
transformation **on the residual**: it rewrites terms that are already in `P`, and it dispatches
on the accumulator's *type* rather than on a name.

> **δ is name-directed and reduces source toward the residual. It cannot transform the residual
> itself.**

## 5. The answer

Two constructs, with a clean split:

| | δ over `def` | Rules |
|---|---|---|
| Direction | source → residual | residual → residual |
| Dispatch | on a **name** | on a **shape or type** |
| Covers | all layers, the whole capability graph, fusion | SROA on loop-carried values, and cleanups on primitive terms |
| Needs `?x` | no | yes |

So core-0 §1.3 stays, but its scope shrinks sharply: **metavariables exist only for
residual-to-residual transformation**, not for lowering. Every rule in every derivation except
g2's SROA becomes a `def`.

That is a substantial simplification. The capability graph — the project's central mechanism —
needs no pattern matching at all.

## 6. What paper did not settle

Honest limits, since the method was hand-reduction:

1. **One fusion example was verified, not all of them.** `sum`/`zip` reduces correctly. `filter`,
   `scan`, and nested `zip` were not checked, and the delayed representation is known to have
   awkward cases — `filter` cannot be a pull array with a static length.
2. **Termination of the delayed encoding was not proved.** §3 claims δ terminates on a DAG of
   definitions; the encoding introduces higher-order definitions (`vec` returns a function) and
   the argument should be redone against those.
3. **The residual was checked for shape, not for cost.** It matches g1's residual textually.
   Whether the emitter produces the same Go from it is a
   [parity](../decisions/0008-measurement-over-principle.md) question, and paper cannot answer
   that one.

Item 1 is the one that could still overturn this. **`filter` is the test to do next** — if a
pull array cannot express it, the delayed representation is not general and fusion may need rules
after all.
