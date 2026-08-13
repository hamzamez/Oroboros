# Derivation: gauntlet program 1 — dot product

> **⚠ Corrected by measurement, 2026-08-13.** The bounds check *is* hoisted as §7 describes —
> verified in Go's SSA output — but it buys **nothing measurable**, in L1 or out. The loop is
> bottlenecked on the serial `acc +=` dependency chain. `require` remains justified by
> correctness, not performance. Separately, §8's left-to-right `sum` decision has a measured
> price: **5.2–7.2× on Go**. See
> [baseline R2 and U1](../../gauntlet/results/baseline-2026-08-13.md).

The hard case. Fusion rules are same-layer, so the termination guarantee from
[g4 §10](g4-word-count.md) does not cover them. This is where the rewriting core was most
likely to break.

**Result: it survives, and the termination risk turns out smaller than feared.** But program 1
costs more than program 4 — one new core form, one specification decision that cannot be
deferred, and confirmation of the hygiene defect.

---

## 1. The references to hit

**Go.** The naive version leaves a bounds check on `ys[i]`, because the compiler cannot prove
`len(ys) >= len(xs)`. Fast hand-written Go hoists it:

```go
func dot(xs, ys []float64) float64 {
	ys = ys[:len(xs)]          // hoists the check out of the loop
	acc := 0.0
	for i := 0; i < len(xs); i++ {
		acc += xs[i] * ys[i]
	}
	return acc
}
```

**JavaScript** — on `Float64Array`, not `Array`. The choice of representation is itself a
Parasite decision and belongs in the vocabulary:

```js
function dot(xs, ys) {
  let acc = 0.0;
  for (let i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
  return acc;
}
```

**Java** — `double[]`, with the same shape.

## 2. The source, and why the obvious version is wrong

The obvious source:

```lisp
(fn dot ((xs (slice f64)) (ys (slice f64))) -> f64
  (sum (zip * xs ys)))
```

Nothing here says the two slices have the same length. So `(at ys i)` where `i < (len xs)` is
not provably in bounds, and under [ADR 0003](../decisions/0003-range-typed-integers.md) — where
out-of-range is a compile-time error when provable and a trap otherwise — this either rejects a
legitimate program or emits a per-access check and loses to the reference.

Three ways out:

| | Cost | Result |
|---|---|---|
| (a) Length-indexed arrays, `(array f64 n)` | Dependent types in the core | Full elimination, no runtime check |
| (b) An explicit precondition | One new core form | One check at entry, then everything inside is provable |
| (c) Slices, checked per access | Nothing | Loses to hand-written |

**(b).** It is small, it gives range analysis a fact to work from, and it emits exactly the
idiom fast Go already uses:

```lisp
(fn dot ((xs (slice f64)) (ys (slice f64))) -> f64
  (require (= (len xs) (len ys)))
  (sum (zip * xs ys)))
```

`require` is the one new core form program 1 forces. It is not an assertion — it is a fact
introduced into the range environment, which happens to also emit a guard.

## 3. Ordering is the whole problem

```lisp
(layer vectors
  (rule (dot ?a ?b) => (sum (zip * ?a ?b))))
```

After step 1 the term is `(sum (zip * xs ys))`, and both `sum` and `zip` are outside every
target's vocabulary. If lowering fires on `zip` first, it materializes an intermediate array
and the program allocates *n* floats it never needed. Total parity failure.

> **Fusion must run to fixpoint before lowering descends a layer.**

That fixes the phase structure: at each layer, run optimization rules to fixpoint, then lower
exactly one layer. Optimization rules therefore only need to terminate *within a layer*, which
is a much smaller obligation than global confluence.

## 4. Two kinds of fusion, and only one is risky

The fusion rule for this program is:

```lisp
(rule (sum (zip ?f ?a ?b))
   => (fold-range 0.0 (len ?a)
        (fn (acc i) (+ acc (?f (at ?a i) (at ?b i))))))
```

Its left side is at `vectors` and its right side is at `iteration` — **strictly
layer-decreasing.** So it terminates under the g4 §10 argument. It is a *lowering* rule that
happens to fuse.

Genuinely same-layer fusion looks different:

```lisp
(rule (zip ?f (map ?g ?a) ?b)          ; vectors -> vectors
   => (zip (fn (x y) (?f (?g x) y)) ?a ?b))
```

Dot product needs none of these. But without them, every composite shape needs its own
fused-lowering rule, and the rule count goes combinatorial. **Same-layer rules are what keep
the lowering set linear rather than exponential**, so they cannot simply be banned.

## 5. Termination: three classes, not two

| Class | Example | Terminates because | Risk |
|---|---|---|---|
| **Layer-decreasing** | `(sum (zip ...)) => (fold-range ...)` | Layer DAG height decreases | None |
| **Measure-decreasing** | `(map ?f (map ?g ?a)) => (map (compose ?f ?g) ?a)` | Collection-producing nodes: 2 → 1 | None, and mechanically checkable |
| **Permutative** | `(+ ?a ?b) => (+ ?b ?a)` | Nothing decreases | Diverges |

Deforestation rules all fall in class 2, and the measure — *count the collection-producing
constructors* — is structural, so the engine can verify each rule decreases it. That is far
better than the "needs e-graphs" verdict recorded in
[core-candidates.md](../core-candidates.md) §4.

Only class 3 is genuinely dangerous. And class 3 can be excluded outright:

> **Only implement optimizations the host cannot do.**
>
> Go's compiler, V8, and HotSpot all reassociate arithmetic and simplify algebra. None of them
> can undo a materialized intermediate array, because by emission time the allocation is real.
> So implement deforestation and nothing else.

This is the Parasite argument turned on the optimizer itself, and it deletes class 3 from v1
entirely. The confluence risk does not need to be managed — it needs to not be taken.

## 6. The derivation

**Step 0.** `(dot xs ys)`

Worth pausing here. On a target whose vocabulary includes `dot` — a BLAS binding, or hardware
with a dot-product instruction — rewriting halts *immediately* and emits the native call. That
is [ADR 0002](../decisions/0002-capability-graph.md)'s "compiling up," and it is one vocabulary
entry, not a mechanism.

For plain Go, `dot ∉ vocab` — rewrite.

**Step 1** — `vectors/dot`:

```lisp
(sum (zip * xs ys))
```

**Step 2** — fusion, before any lowering:

```lisp
(fold-range 0.0 (len xs)
  (fn (acc i) (+ acc (* (at xs i) (at ys i)))))
```

`?a` occurs twice on the right-hand side, which by [g4 §4](g4-word-count.md) should be
let-bound. But `?a` matched the bare variable `xs`, and binding a variable to a variable
produces let-chains everywhere for no benefit. **Refinement to the g4 fix: auto-bind only when
the matched term is non-trivial** — not already a variable or a literal.

**Step 3** — `iteration/fold-range` lowers. `?n` matched `(len xs)`, which is non-trivial and
occurs twice, so it *is* bound:

```lisp
(block
  (let n (len xs))
  (var acc 0.0)
  (var i (int 0 n) 0)
  (loop (when (= i n) (break))
        (set acc ((fn (acc i) (+ acc (* (at xs i) (at ys i)))) acc i))
        (set i (+ i 1)))
  acc)
```

**Step 4** — beta, and **the hygiene defect made concrete.**

The rule's right-hand side introduces binders named `acc` and `i`. The lambda being applied
also binds `acc` and `i`. Here they coincide and the result is accidentally correct. Had the
user written `(fn (a j) ...)` over a body referencing an outer `i`, the rule's `i` would have
captured it silently.

This is the alpha-conversion problem, and it is **not** the same as g4's Defect 1 — that one
was the *sharing* problem, why graph reduction and call-by-need exist. Both classical, both
solved: sharing by let-binding, capture by hygiene. Rule-introduced binders must be fresh by
construction, exactly as in hygienic macro expansion.

```lisp
(set acc (+ acc (* (at xs i) (at ys i))))
```

**Step 5.** All terms are in Go's vocabulary. Halt.

## 7. Emission and parity

```go
func dot(xs []float64, ys []float64) float64 {
	if len(xs) != len(ys) {
		panic("dot: length mismatch")
	}
	ys = ys[:len(xs)]
	n := len(xs)
	acc := 0.0
	for i := 0; i < n; i++ {
		acc += xs[i] * ys[i]
	}
	return acc
}
```

`ys = ys[:len(xs)]` comes from the `require` fact, and is the form that lets Go's bounds-check
elimination clear `ys[i]`. Both indexed accesses should end up unchecked, matching *optimized*
hand-written Go and beating the naive version.

The exact form that triggers BCE is a Go-compiler-version detail. That yields a small finding
in its own right: **BCE idioms are target- and version-specific, so they belong in backend
emission rules, not in the core** — and the gauntlet is what keeps them honest as Go changes.

Allocations: zero. No intermediate array survives fusion, and no closure is ever formed.

## 8. The specification decision that cannot be deferred

`sum` was fused into a **left-to-right** fold. But mathematically `sum` is unordered, and
**floating point addition is not associative.** So:

- If `sum` means the mathematical sum, the compiler may reassociate, and results differ between
  targets and between optimization levels. That contradicts strict-IEEE-by-default.
- If `sum` means a left-to-right fold, results are deterministic everywhere, but it can never
  be vectorized or parallelized without an explicit opt-in.

There is no third option; this is the `-ffast-math` tension and it cannot be papered over.

**Decision: `sum` is specified left-to-right. `sum-unordered` is a separate capability that
permits reassociation.**

Consequence, and it ties back to §6: a target with a native `dot` can only be used from
`sum-unordered`, because a BLAS call makes no ordering promise. The strict version halts one
layer lower. That is correct behaviour, and it means the "compile up" case is gated on the
program having asked for it.

## 9. Findings

1. **Fusion must reach fixpoint before lowering descends.** Otherwise `zip` materializes.
   Fixes the phase structure: optimize to fixpoint, lower one layer, repeat.
2. **Fusion splits in two.** Fused-lowering is layer-decreasing and free; deforestation is
   same-layer and needed to keep the lowering set from going combinatorial.
3. **Optimization rules fall in three termination classes**, not two. Deforestation is
   measure-decreasing on a structural measure and mechanically checkable.
4. **Only implement optimizations the host cannot do.** Hosts do algebra; hosts cannot undo a
   materialized intermediate. This deletes the permutative class and with it the confluence
   risk.
5. **Auto let-binding refined:** bind only non-trivial matched terms.
6. **Hygiene is a live defect.** Rule-introduced binders must be fresh by construction.
7. **`require` is a new core form**, forced by bounds elimination. Preconditions as facts beat
   both dependent types (too large) and per-access checks (loses parity).
8. **`sum` must be specified left-to-right**, with `sum-unordered` separate. Not deferrable.
9. **BCE idioms belong in backend emission rules**, being target- and version-specific.

## 10. Verdict

Program 1 survives, and the thing most likely to kill the rewriting core — same-layer fusion —
turned out to be tractable, largely because most of it is not same-layer at all, and the
remainder is measure-decreasing.

The cost is that program 1 buys less for more than program 4 did: a new core form, a
non-deferrable specification decision, and confirmation that hygiene is required.

**Running tally of required machinery:** auto let-binding, layer stratification, linearity
analysis, hygiene, range analysis with `require` facts, and a deforestation measure check.
That is more than "a pattern matcher and a rule engine," and the claim in
[core-candidates.md](../core-candidates.md) §4 that the implementation is "a few hundred lines"
should be treated as false.

**Next:** program 3 (the same generic operation at two element types) is the remaining
structural risk, since monomorphization interacts with rule matching in a way neither
derivation has touched.
