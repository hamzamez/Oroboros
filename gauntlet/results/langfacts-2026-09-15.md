# `lang`'s facts: two laws of the integers become declarations, and one algorithm instantiates them

2026-09-15. [spec/theories.md §7](../../docs/spec/theories.md), [facts.md](../../docs/facts.md).

## 1. What moved, and why it is one algorithm

`seedDivAxioms` was F1 and F2 of facts.md's inventory, written as Go: for every quotient of a length by
a positive literal, assume both halves of Euclidean division. It is now two declarations in
`emit/lang-facts.oro`:

```lisp
(fact len-nonneg ((t (array A)))
  (<= 0 (len t)))

(fact div-floor ((x int) (k int))
  (when (<= 0 x) (<= 1 k))
  (and (<= (* k (/ x k)) x)
       (<= x (+ (* k (/ x k)) (- k 1)))))
```

**What a fact is.** An axiom schema `∀x̄. G(x̄) → C(x̄)` over the linear fragment extended by one
uninterpreted trigger term: a local theory extension (Sofronie-Stokkermans, CADE 2005).

**The algorithm.** `emit/fact.go` has one algorithm for every such schema: instantiation on present
terms, to a fixpoint.
- For each subterm matching a trigger by σ, if the facts already assumed entail σG, assume σC. That is
  complete for a local extension (Ihlemann, Jacobs & Sofronie-Stokkermans, TACAS 2008).
- **It terminates.** An instance adds inequalities and never a term, so the candidates are the finite
  set of (fact, subterm) pairs, each used once.
- **It is a fixpoint and not one pass**, because a guard can become entailed only after another
  instance: `div-floor`'s `0 ≤ x` for a length needs `len-nonneg` first.

**The guard is entailed, not assumed** (postconditions.md §4, Lemma 1). `seedDivAxioms` had checked
`x ≥ 0` syntactically, by asking whether `x` is a length. That is now the theorem `len-nonneg`, and the
syntactic test is gone.

## 2. One reading of the spec made precise

theories.md §7.3 classifies `(* x y)` with neither factor a literal as an extension term, and the
spec's own `div-floor` writes `(* k (/ x k))`. Read literally, the spec's example fails its own
admission rule: it has two extension terms.

**Resolution.** `(/ x k)` is an atom of the linear fragment only when `k` is a positive literal. That is
what `asLinear` requires. So matching the trigger can only bind `k` to a literal: `k` is a **constant
parameter**, and `(* k …)` is multiplication by a constant.

Admission therefore treats a parameter in a literal-only position of the trigger as a literal. At
instantiation, the conclusion must still read as linear (`obligation`), or nothing is assumed. That
closes the case of a match binding the divisor to a non-literal.

## 3. Admission refuses by the condition that failed

| fact | refused with |
|---|---|
| `(<= (% x 3) (% y 3))` | a fact relating two terms is **F-C**, reserved |
| `(<= 0 (len x))` over parameters `x`, `y` | condition 3, `y` does not occur in the trigger |
| `(<= 0 (% (% x 3) 5))` | condition 2, not flat |
| a parameter of type `f64` | condition 5, a float fact |
| `(forall …)` | **F-D**, reserved |
| `(lemma …)` | **F-E**, reserved |
| `(<= 0 x)` | condition 1, no trigger term |

## 4. The evidence can fail (§7.10, item 2)

- **The control.** `lang`'s full theory proves hex.Decode's precondition `len src ≤ 2·len dst + 1`,
  for a buffer sized `⌊len a / 2⌋` — the witness `TestBothHalvesOfDivisionByALiteralAreKnown` already
  pinned.
- **Delete `div-floor`'s upper half**, and the same query is no longer proven.
- **Delete `len-nonneg`**, and neither half is instantiated. The guard is taken from entailment, so it
  is not taken at all.

## 4b. What changed: one diagnostic names what it knows

The emission check reported **three changes, all the same line**, in `examples/tally/tally.oro` on Go,
JavaScript and Java. That program is the host-independent core of the tally application, and it is
refused alone by design: `(compile p)` is its host interface, and without a binding file it reads as an
indexing.

The refusal's premises were:

```
   gen: gen-tally: (compile p) is an indexing, and (<= 0 p) does not follow
-    known: nothing
+    known: assumed -len(ls) <= 0 (fact len-nonneg)
```

The refinement layer now knows a length is non-negative, and says so by the fact's name. That is
theories.md §10's principle, that *a failed proof names its premises, facts by name*.

**Nothing was proven or lost.** Every emitted file is byte-identical and the totals are unchanged. The
new instances of `len-nonneg` are true facts that happen to decide nothing new in this corpus. Accepted
into the baseline with `-accept`.

## 5. Cost

- **Code**: `emit/refine.go` 824 → 788, `emit/fact.go` +263, `lang-facts.oro` two declarations. It
  did not get smaller. One schema's worth of Go became a general instantiation procedure and its
  admission, which is what every later fact reuses.
- **`go run ./cmd/check -accept`, every step, passes**: emission 194 of 194 files byte-identical, proof
  counts identical (2,307 of 2,413 integer operations, 345 of 382 loops), differential on four targets,
  tooling; three changes accepted, all §4b's one diagnostic line.

## 6. Not built

- The interval layer's facts F5–F10 and F12 (`divI`, `remI`, `storedRange`, `andI`, `shrI`, `mulI`),
  through Theorem T's induced transfer. That is the half where one declaration serves three consumers.
- F3 and F4 (`x ≤ x·x`, `narrowSquare`): F4's bound is a compile-time `isqrt`, reserved (§7.6).
- Facts in target layers beyond `max-len`, and §7.9's contracts consumed as facts.
