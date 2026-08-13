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

## What came out of them

**The identity.** Rewriting is lambda calculus generalized — one rule, beta, plus alpha. What
separates it from the predecessor project is stage, not mechanism:

> Everything is a function, evaluated at compile time. What survives is what the target must do
> at runtime, and the compiler will tell you exactly what that is.

**One defect family.** Every defect found is naive rewriting losing a property the term held
implicitly — sharing (g4), capture-freedom (g1, g3), simultaneity (g2), effect count and order
(g5). Four independent routes to *when may a term be copied, moved, or deleted?*, which is
substructural and probably wants one answer in the type discipline rather than four analyses.
This is the most valuable unexplored thread.

**The machinery it actually needs**, which is more than "a pattern matcher and a rule engine":

> auto let-binding · layer stratification · linearity analysis · hygiene · range analysis with
> `require` facts · deforestation measure check · ANF normalization · monomorphization for
> recursive generics · polymorphic type checking · SROA with parallel-assignment temporaries ·
> effect-context checking on rules

## The correction record

The first baseline run refuted five beliefs these derivations had reasoned their way into. All
five were plausible readings of how the hosts are documented to work; none would have been
caught by argument. That is why [ADR 0007](../decisions/0007-exploration-over-specification.md)
fixes the test rather than the language, and why
[ADR 0008](../decisions/0008-measurement-over-principle.md) now requires a measurement behind
every parasite decision.

Corrections are folded into the reasoning **and** flagged at the top of each document, so a
future reader gets the current answer without losing the record of how it changed.
