# Derivation: gauntlet program 4 — word frequency count

> **⚠ Corrected by measurement, 2026-08-13.** Three corrections. §7's `m[k]++` advantage is
> **1.23×, not "roughly 2×"**. The pass condition "JS output must contain `Map`" is **wrong** —
> a null-prototype Object is 3.25× faster than `Map` for string keys. And the granularity law
> is **target-dependent, not universal**: on Java the *unfused* `getOrDefault`+`put` beats the
> fused `merge` by 2.6× — **which did not reproduce on JDK 17, where the fused form is 1.19×
> faster** ([native-java-2026-08-25](../../gauntlet/results/native-java-2026-08-25.md)). See
> [baseline C3, R4, R5](../../gauntlet/results/baseline-2026-08-13.md).

Hand-derivation, no compiler. The question: does a plausible rule set take the source to the
hand-written Go, terminate in Go's vocabulary, and keep `map[string]int` in the output?

**Result: it survives, with three disciplines that were not in the design before.** Four
defects were found; all four are fixable and one refines [ADR 0002](../decisions/0002-capability-graph.md).

---

## 1. The references to hit

**Go** — the pass condition is that the output contains `map[string]int`:

```go
func wordCount(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(text) {
		counts[w]++
	}
	return counts
}
```

**JavaScript:**

```js
function wordCount(text) {
  const counts = new Map();
  for (const w of text.trim().split(/\s+/)) {
    counts.set(w, (counts.get(w) ?? 0) + 1);
  }
  return counts;
}
```

**Java:**

```java
static Map<String,Integer> wordCount(String text) {
    Map<String,Integer> counts = new HashMap<>();
    for (String w : text.trim().split("\\s+")) counts.merge(w, 1, Integer::sum);
    return counts;
}
```

## 2. The source

```lisp
(fn word-count ((text string)) -> (dict string (int 0 2^31))
  (tally (split-words text)))
```

`(int 0 2^31)` rather than an unbounded `nat`: 2^31 is the range that survives JavaScript,
where numbers are doubles and integers are exact only to 2^53. Writing `nat` here would be a
portability leak dressed as convenience. ([ADR 0003](../decisions/0003-range-typed-integers.md))

## 3. Vocabularies

```lisp
(target go   (vocab core seqs strings dicts))
(target js   (vocab core seqs strings dicts))
(target java (vocab core seqs strings dicts))
(target c    (vocab core))
```

Neither `tally` nor `for-each-into` is in any target's vocabulary, so both always lower.
`dicts` and `strings` are in all three initial targets, so on those they never lower — which
is the entire Parasite claim, and it is decided by vocabulary membership alone.

## 4. First attempt, and the first defect

```lisp
(layer collections
  (rule (tally ?seq)
     => (for-each-into (dict-empty) ?seq
          (fn (d w) (dict-update d w 0 inc)))))

(layer iteration
  (rule (for-each-into ?init ?seq ?step)
     => (block (var acc ?init) (var i 0)
          (loop (when (= i (len ?seq)) (break))
                (set acc (?step acc (at ?seq i)))
                (set i (+ i 1)))
          acc)))
```

Applying the second rule substitutes `?seq` textually, and `?seq` occurs three times on the
right-hand side:

```lisp
(loop (when (= i (len (split-words text))) (break))
      (set acc (dict-update acc (at (split-words text) i) 0 inc))
      ...)
```

**Defect 1 — naive substitution duplicates work.** The text is re-split on every iteration, and
twice per iteration at that. This is the classic term-rewriting failure and it is quadratic-or-worse,
not a constant factor.

**Fix, and it is mechanical:** a metavariable occurring more than once on a rule's right-hand
side must be let-bound rather than substituted. The rule engine can check this and insert the
binding automatically — no per-rule discipline required from whoever writes the rules.

```lisp
(layer iteration
  (rule (for-each-into ?init ?seq ?step)
     => (block
          (let xs ?seq)                        ; bound once, by construction
          (var acc ?init)
          (var i (int 0 (len xs)) 0)
          (loop (when (= i (len xs)) (break))
                (set acc (?step acc (at xs i)))
                (set i (+ i 1)))
          acc)))
```

## 5. The derivation

**Step 0.**

```lisp
(tally (split-words text))
```

`split-words` ∈ `strings` ⊆ go — halt on that subterm. `tally` ∉ go — rewrite.

**Step 1** — `collections/tally`:

```lisp
(for-each-into (dict-empty) (split-words text)
  (fn (d w) (dict-update d w 0 inc)))
```

`dict-empty`, `dict-update` ∈ `dicts` ⊆ go — halt. `for-each-into` ∉ go — rewrite.

**Step 2** — `iteration/for-each-into`, with the let-binding fix:

```lisp
(block
  (let xs (split-words text))
  (var acc (dict-empty))
  (var i (int 0 (len xs)) 0)
  (loop
    (when (= i (len xs)) (break))
    (set acc ((fn (d w) (dict-update d w 0 inc)) acc (at xs i)))
    (set i (+ i 1)))
  acc)
```

**Step 3** — beta. The operator is a literal lambda at the application site and each parameter
occurs once, so substitution is safe and non-duplicating:

```lisp
(set acc (dict-update acc (at xs i) 0 inc))
```

Worth noting: no closure is ever formed. The "closures are not a core primitive" constraint is
satisfied by a rewrite rule rather than by a closure-conversion pass.

**Step 4.** Every remaining term is in go's vocabulary. Rewriting halts.

## 6. Emission

```go
func wordCount(text string) map[string]int {
	xs := strings.Fields(text)
	acc := make(map[string]int)
	for i := 0; i < len(xs); i++ {
		acc[xs[i]]++
	}
	return acc
}
```

Against the reference: an indexed loop instead of `range`. Go handles both, and `i < len(xs)`
in the loop condition is the shape Go's bounds-check elimination recognizes, so `xs[i]` should
not be checked. Expected parity — **to be confirmed by the gauntlet baseline, not assumed.**

The pass condition holds: the output contains `map[string]int`, not a hash table.

**On JS, the analogous condition was wrong.** This derivation assumed the JS output must contain
`Map`, as the host's native dictionary. Measured at n=65536: `new Map()` costs 20,546,287 ns
against a null-prototype `Object` at 6,320,347 ns — **`Map` is 3.25× slower** for string keys.
The correct emission for `dicts` on JS is `Object.create(null)`, and the pass condition is
"uses the host's own dictionary," with *which* one being a measurement.

## 7. Defect 2 — capability granularity decides parity

`acc[xs[i]]++` is emitted only if the backend produces that form. If `dict-update` lowers the
obvious way — get, then set — the output is:

```go
acc[xs[i]] = acc[xs[i]] + 1        // two hash lookups
```

Go compiles `m[k]++` through a single `mapassign` returning a pointer to the value slot, so the
reference does **one** lookup and the naive lowering does **two**.

**Measured, n=65536:**

| Go | ns/op |
|---|---|
| `m[w]++` | 1,959,729 |
| `m[w] = m[w] + 1` | 2,412,086 |
| explicit get-or / set | 2,404,768 |

**1.23×, not the "roughly 2×" claimed here.** The loop also pays for `strings.Fields` and string
hashing, so the extra lookup is not the whole cost. Still past any reasonable threshold, so the
conclusion holds — `dict-update`, not `dict-get` plus `dict-set`, is the Tier 1 capability, and
the Go backend carries an emission rule recognising `(dict-update ?d ?k 0 inc)` → `?d[?k]++`.

The **generalization**, however, does not survive:

> ~~Capability granularity determines whether parity is reachable. Split a capability finer than
> the target's fused idiom and the fusion is unrecoverable.~~

| Java, n=65536 | ns/op |
|---|---|
| `merge(w, 1, Integer::sum)` — fused | 9,259,530 |
| `put(w, getOrDefault(w,0)+1)` — unfused | 3,577,103 |

On Java the **unfused form wins by 2.6×**; `merge` pays for boxing and a per-entry functional
call. Go's fused idiom wins, Java's fused idiom loses. Restated:

> **Capability granularity determines whether parity is reachable, and which granularity wins
> is a per-target measurement — not a principle.** The fused idiom is not reliably the fast one.

See [ADR 0008](../decisions/0008-measurement-over-principle.md).

On JavaScript the same `dict-update` lowers to `get` then `set`, two lookups, because
hand-written JS also does two. Parity is against the target's own ceiling, which is
[ADR 0004](../decisions/0004-first-targets.md) working as intended.

## 8. Defect 3 — this is idiom recognition, and that is fine here

[ADR 0002](../decisions/0002-capability-graph.md) rejects idiom recognition as brittle. Section 7
just used it. The distinction is real and worth recording:

- Recognizing an idiom in **lowered IR** — spotting a memcpy loop, re-vectorizing a scalarized
  one — is brittle, because lowering has already destroyed the shape being looked for.
- Recognizing an idiom in an **unlowered term** — `(dict-update ?d ?k 0 inc)` — is reliable,
  because the shape is still there, syntactically, and matching it is a pattern match rather
  than an analysis.

ADR 0002's warning is about the first. Section 7 is the second. **Refinement, not
contradiction:** raise before lowering, never after.

## 9. Defect 4 — Tier 1 conformance bites on the first capability

`split-words` is not the same function on all three targets:

| Target | Native | On leading whitespace |
|---|---|---|
| Go | `strings.Fields` | trims; no empty element |
| JS | `split(/\s+/)` | emits a leading `""` |
| Java | `split("\\s+")` | emits a leading `""` |

Specifying `split-words` as Tier 1 forces a choice. Take Go's `Fields` semantics as the spec
and JS/Java need `.trim()` first, which is an extra pass and a small parity cost — and
`"".trim().split(/\s+/)` still yields `[""]`, so the empty-input edge needs handling too.

This is the theoretical concern from ADR 0002 §"Specification tightness" arriving on capability
number one of program number one. It is manageable, but it confirms that **Tier 1 costs real
design work per capability**, and it is the argument for keeping Tier 1 small.

## 10. Termination — better than expected

The confluence risk flagged in [core-candidates.md](../core-candidates.md) §4 turns out to
split in two.

Every rule used here replaces a term with terms from a **strictly lower layer**
(`collections` → `iteration` → `core`). If layers form a DAG and every lowering rule's
right-hand side draws only on strictly lower layers, termination follows by well-founded
induction on layer height. That is a **structural property the rule engine can check**, not a
proof obligation discharged per rule set.

But optimization rules are different. Fusion — `(sum (zip ?f ?a ?b)) => (reduce ...)` — is
*same-layer* by nature, and same-layer rules break the stratification argument.

So there are two rule kinds:

| | Lowering rules | Optimization rules |
|---|---|---|
| Layer movement | Strictly decreasing | Same layer |
| Termination | Free, structurally checkable | Needs a measure, a budget, or e-graphs |
| Required for correctness | Yes | No |

**Correctness never depends on the risky mechanism.** If an optimization rule set fails to
terminate, bound it and the output is still correct, merely slower. That confines the confluence
problem to the optional half of the system, which is the best available outcome.

## 11. Defect 5 — linearity, again

`(set acc (dict-update acc ...))` reads as a functional update returning a new dict, while the
Go emission mutates in place. That is sound only because `acc` is used linearly — threaded
through the loop, never aliased.

So the accumulator needs a **linearity/uniqueness check** before functional update may be
emitted as in-place mutation. Precedent: Clean's uniqueness types, Perceus reference counting.

Note that linearity is now required twice, for unrelated reasons: once on rule right-hand sides
(§4, to avoid duplicating work) and once on accumulators (here, to avoid copying). That
repetition suggests it belongs in the core's type discipline rather than being bolted on as two
separate analyses.

## 12. The contrast case — C

C's vocabulary is `core` alone, so `dicts` and `strings` never halt and the same source keeps
rewriting:

```lisp
(layer dicts-impl
  (rule (dict-empty) => (ht-new 16))
  (rule (dict-update ?d ?k ?z ?f)
     => (block (let s (ht-slot ?d ?k ?z))
               (store s (?f (load s))))))
```

Identical source, hash table emitted. `ht-slot` returning a slot pointer is also what makes the
one-lookup property recoverable on C — the same shape Go gets from `mapassign`.

## 13. Verdict

The rewriting core survives program 4 on paper. Three disciplines are now required that were
not in the design before:

1. **Auto let-binding** of metavariables occurring more than once on a right-hand side.
2. **Layer stratification** for lowering rules, which buys termination for free and confines
   the confluence risk to optimization rules.
3. **Linearity analysis**, needed for both substitution and in-place accumulator update.

And one capability-design law, in its corrected form:

> Capability granularity determines whether parity is reachable — and **which granularity wins
> is a per-target measurement, not a principle.** Go's fused `m[k]++` wins by 1.23×; Java's
> fused `merge` *loses* by 2.6×.

The uncorrected version of that law, and the assumption that JS should emit `Map`, were both
plausible readings of how the hosts are documented to work. Both were wrong, and neither would
have been caught by argument. That is [ADR 0008](../decisions/0008-measurement-over-principle.md)
in one program.
