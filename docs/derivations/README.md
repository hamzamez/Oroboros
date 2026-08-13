# Derivations

Hand-derivations of each [gauntlet](../gauntlet.md) program against the leading candidate core,
done on paper before any compiler existed. Each records what was believed, what
[measurement](../../gauntlet/results/) said, and which of the two won.

They are working documents, kept current — unlike the [ADRs](../decisions/), which are decision
history and are never edited.

| # | Program | Outcome |
|---|---|---|
| [g4](g4-word-count.md) | Word frequency count | Survives. Found work duplication under substitution, and that capability granularity decides parity. **Two claims later refuted.** |
| [g1](g1-dot-product.md) | Dot product | Survives. Termination splits into three classes; only permutative rules are dangerous, and they can be excluded. **Bounds-check win refuted; `sum` ordering priced at 5–7×.** |
| [g3](g3-generics.md) | Generic fold, two instantiations | Survives, and generics need no mechanism — a non-recursive definition *is* a rewrite rule. **Its counterexample to ADR 0002 refuted.** |
| [g2](g2-structs.md) | Centroid and bounds over structs | Survives, no boxing. Forced value semantics with no interior pointers. **AoS penalty is JS-only; Go does SROA itself.** |
| [g5](g5-bindings.md) | Formatted output, Tier 2 bindings | Survives; bindings cost nothing. **First correctness defect: rules can change effect count and order.** |
| [g6](g6-escaping-closures.md) | Escaping closures | Survives at hand-written cost. Establishes what the core *is*: staged lambda calculus. **Constrained by staging soundness.** |
| [s1](s1-substructural.md) | Do the five disciplines collapse? | **Four of five do.** Machinery drops 11 → 8, and grade 0 turns out to *be* the staging annotation. **One row later corrected.** |
| [s2](s2-multiplicity-inference.md) | Is multiplicity inferable without annotation? | **Zero annotations in application code.** Grade 0 is observed, not inferred. One unavoidable declaration: extern purity. **Refutes s1's accumulator row.** |
| [g7](g7-aliasing.md) | Program 6 — mutation through an aliased slice | Aliasing is a **correctness** problem, not a performance one — being conservative costs 0%. In-place is **4.6–6.2× slower**. A uniqueness false negative on a dict costs **40–1,540×, unbounded.** |
| [s3](s3-cross-boundary-reuse.md) | Does reuse survive function boundaries? | **Most boundaries do not survive rewriting.** Grade in the signature is the ownership annotation. RC fallback costs **3%** — but **naive RC costs 14×**, so it depends on the static analysis rather than replacing it. |

## What came out of them

**The identity.** Rewriting is lambda calculus generalized — one rule, beta, plus alpha. What
separates it from the predecessor project is stage, not mechanism:

> Everything is a function, evaluated at compile time. What survives is what the target must do
> at runtime, and the compiler will tell you exactly what that is.

**One defect family, and it resolved.** Every defect found is naive rewriting losing a property
the term held implicitly — sharing (g4), capture-freedom (g1, g3), simultaneity (g2), effect
count and order (g5). [s1](s1-substructural.md) took the thread and found four of the five are
structural rules (contraction, weakening, exchange) over two axes, while capture is not
structural at all and is better solved by term representation than by any check.

**The machinery it actually needs**, after s1:

| # | Item |
|---|---|
| 1 | **Graded type system** — multiplicity 0/1/ω × pure/stateful. Absorbs auto let-binding, linearity, effect checking, parallel-assignment temporaries, escape analysis, and binding-time analysis. |
| 2 | Locally-nameless term representation — hygiene by construction |
| 3 | ANF normalization — the ordering axis's normal form |
| 4 | Layer stratification — termination |
| 5 | Deforestation measure check — termination |
| 6 | Range analysis with `require` facts |
| 7 | Monomorphization for recursive generics |
| 8 | SROA, splitting only |

**Grade 0 is the staging annotation.** Multiplicity and binding time are the same column, so the
per-abstraction cost report g6 wanted to show programmers is produced by the machinery already
required for soundness — and [s2](s2-multiplicity-inference.md) found it is *observed* in the
residual rather than predicted, so it costs nothing and never has to hedge.

**Multiplicity answers how many times; uniqueness answers how many references.** Only the first
is a grading. Value-typed accumulators get uniqueness free from g2's value semantics; heap-typed
ones need liveness — and [g7](g7-aliasing.md) prices getting that wrong at **40–1,540×,
unbounded in structure size**, a different severity class from anything else measured here.

**Hazards closed by construction, not by checking** — three times now, and it is the design's
most reliable move:

| Hazard | Made unrepresentable by |
|---|---|
| Variable capture | Locally-nameless term representation ([s1](s1-substructural.md)) |
| Aliasing a value local | Value semantics, no interior pointers ([g2](g2-structs.md)) |
| Aliasing a slice parameter | No mutable parameters; reuse chosen by liveness ([g7](g7-aliasing.md)) |

## The correction record

The first baseline run refuted five beliefs these derivations had reasoned their way into. All
five were plausible readings of how the hosts are documented to work; none would have been
caught by argument. That is why [ADR 0007](../decisions/0007-exploration-over-specification.md)
fixes the test rather than the language, and why
[ADR 0008](../decisions/0008-measurement-over-principle.md) now requires a measurement behind
every parasite decision.

Corrections are folded into the reasoning **and** flagged at the top of each document, so a
future reader gets the current answer without losing the record of how it changed.
