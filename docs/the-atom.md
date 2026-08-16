# The atom

What is the irreducible unit of this language — the thing lambda calculus is to a Lisp?

This supersedes the framing in [first-implementation.md](first-implementation.md), which
answered a different question ("what should be typed first") and gave **"a vocabulary"** as the
core. That was wrong. A vocabulary is a *list*. Lists have no algebra, no reduction relation, and
nothing to prove about them. It is the answer WebAssembly gives, and it is a refusal of the
question rather than an answer to it.

---

> **⚠ Corrected by [spec/pcf.md](spec/pcf.md).** This document says the atom is "as small as
> lambda calculus" and lists departures from it. There are none — it is **PCF** (λ + constants +
> ~~`fix`~~ — see [pcf.md §9](spec/pcf.md), recursion was removed by ADR 0014), reduced to normal
> form at compile time. Nothing in the mathematics is new; the
> contribution is that Σ is a *per-target* parameter. The framing below survives, the novelty
> claim does not, and pcf.md explains why saying it smaller is worth more.

## The atom

> **Lambda calculus in which the normal form is a parameter.**

In lambda calculus, "normal form" is fixed: a term is normal when no β-redex remains. That is
baked into the calculus.

Make it a parameter and this language falls out. A target does not supply a list of instructions
— it supplies a **partition of names into primitive and defined**. Reduction runs until only
primitives remain.

```
term ::= x | λx.t | (t t)

reduce until no defined name remains
```

Go declares `map` primitive, so reduction stops there and the output uses Go's `map`. C declares
`map` defined — by the hash-table rules — so reduction continues into a real hash table. **Same
term, same rules, different normal form, because normality is a parameter.**

Three constructs and one parameter. Exactly as small as lambda calculus, and unlike a vocabulary
it has an algebra.

## What collapses into it

**Layers and rules.** `(dot ?a ?b) => (sum (zip * ?a ?b))` looked like separate machinery. It is
a definition, so applying it is δ-reduction — and δ over named definitions *is* β once the names
are let-bound. A layer is a set of definitions. Nothing new is introduced.

**Both directions of [ADR 0002](decisions/0002-capability-graph.md).** Previously these were two
mechanisms:

| | Previously | Now |
|---|---|---|
| Lowering a feature the target lacks | apply the shim's rules | the name is **defined** |
| "Compiling up" to hardware that has it natively | *absence* of a shim | the name is **primitive** |

One word: which side of the partition a name is on.

**Staging.** Reduce at compile time; the residual *is* the normal form. There is no separate
staging mechanism to specify.

**And grading is *not* in the atom.** Grade 0 means "reduced away" — absent from the normal form.
Grades 1 and ω are a counting property *of the residual*. So the entire substructural apparatus
from [s1](derivations/s1-substructural.md)–[s5](derivations/s5-cycles.md) is an **observation on
the normal form**, not a primitive of the calculus.

That retro-explains a result which was found by measurement and could not be explained at the
time: [s2 §2](derivations/s2-multiplicity-inference.md) found that **grade 0 is observed, not
inferred** — read off the residual rather than computed. The atom says why it had to be.

## The specification, and its tests

| Property | Kind |
|---|---|
| Reduction relation: β, plus δ over defined names | definition |
| Normal form: no defined name remains | definition |
| **Confluence** — reduction order does not change the result | theorem; what layer stratification buys |
| **Termination** — reduction reaches a normal form | theorem; the layer DAG |
| **Stage soundness** — the normal form computes what the unreduced term computes | theorem |
| **Parity** — the emitted normal form matches hand-written code | **not provable** |

The third theorem deserves attention. **[ADR 0009](decisions/0009-staging-preserves-results.md)
is the soundness theorem of a two-level calculus.** It was written as a bug report about Go
folding `0.1+0.2` to `0.3` at compile time and `0.30000000000000004` at runtime. It is the
coherence property of the atom, and it was derivable from the algebra rather than from a
benchmark.

**Prior art**: two-level λ-calculus (Nielson & Nielson); Davies & Pfenning on staged computation
as modal logic, where `□A` is a closed term available now — which is grade 0.

## Where mathematics stops

Four properties, three of them theorems. The fourth — **parity** — is not provable, because it
depends on measured behaviour of Go's inliner, V8's array representation, and HotSpot's
allocator. [ADR 0008](decisions/0008-measurement-over-principle.md) exists precisely because that
boundary is real.

This is the honest limit of a specify-then-test methodology here, and it is a boundary Lisp does
not have. **In a Lisp, correctness is the whole game. Here it is three-quarters of it**, and the
missing quarter is the quarter that stopped the predecessor.

## What this changes about the first implementation

[first-implementation.md](first-implementation.md) argued for the Go emitter first, on the
grounds that the calculus could not be tested without a backend. **With the atom correctly
identified, that is false.**

A **β/δ reducer parameterized by a primitive set** is roughly 200 lines, and confluence,
termination, and stage soundness are all checkable on terms with no backend at all. So the
specify-then-test methodology does apply — the earlier document denied it only because it had
mis-identified the atom.

Revised order:

| | Step | Test |
|---|---|---|
| 1 | **β/δ reducer parameterized by a primitive set** | confluence, termination, stage soundness — all algebraic |
| 2 | **Go emitter over the normal form** | hand-written residual reaches the baseline: 1,389 ns at n=1024, zero allocations |
| 3 | Reader, printer, formatter | round-trips |
| 4 | One layer of definitions ([g4](derivations/g4-word-count.md)'s `collections`) | reaches baseline; output contains `map[string]int` |
| 5 | **JS backend before any front-end features** | the atom survives the hostile host |

Step 2 is not optional and is not deferrable past step 2. **A verified calculus proves the core
is coherent; it proves nothing about the thesis.** Every unmeasured prediction in this project
has been wrong roughly half the time, and parity is the prediction that matters.

## The identity, restated

[g6](derivations/g6-escaping-closures.md) gave the semantic identity. The atom sharpens it from
one side:

> **Everything is a function. Reduction runs until only what the target already knows remains —
> and what the target knows is a parameter, not a fact about the language.**
