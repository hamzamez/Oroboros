# Exploration: do the five disciplines collapse into one?

Exploration only. No commitments, no ADR.

> **⚠ One row corrected by [s2](s2-multiplicity-inference.md).** §1 maps "accumulator must not
> be aliased" onto *contraction forbidden*. That is wrong: an accumulator in a loop is used *n*
> times, so it is ω by counting. What it needs is **uniqueness** — no other reference exists —
> which is a different property, dual to linearity. Value-typed accumulators get it free from
> [g2](g2-structs.md)'s value semantics; heap-typed ones need liveness, not counting. The
> collapse survives one row smaller: **multiplicity answers how many times, uniqueness answers
> how many references, and only the first is a grading.**

Six derivations produced defects that all looked like the same shape. The
[assessment](../assessment-2026-08-13.md) named the question they kept arriving at — *when may a
term be copied, moved, or deleted?* — and flagged it as the highest-information thread
remaining, because it directly changes the answer to "is this easy to implement."

**Result: four of five collapse, and they collapse onto something that also hands us the
staging annotation for free. The fifth does not collapse and is solved better anyway. The
machinery list drops from eleven items to eight, and the largest remaining item does five jobs.**

---

## 1. The five, mapped onto structural rules

Substructural logic classifies systems by which structural rules they permit:

- **Contraction** — using something twice (copying)
- **Weakening** — not using it at all (deleting)
- **Exchange** — swapping the order of two things (reordering)

Mapping each defect:

| Derivation | Defect | Structural rule | Verdict |
|---|---|---|---|
| [g4](g4-word-count.md) | Substitution duplicates *work* | Contraction **costs** | fits |
| [g4](g4-word-count.md), [g1](g1-dot-product.md), [g2](g2-structs.md) | Accumulator must not be aliased | ~~Contraction forbidden~~ | **does not fit** — see [s2 §4](s2-multiplicity-inference.md); this is uniqueness, not multiplicity |
| [g5](g5-bindings.md) | Effect duplicated | Contraction **forbidden** | fits |
| [g5](g5-bindings.md) | Effect moved into a dead branch | Weakening **forbidden** | fits |
| [g5](g5-bindings.md) | Effects reordered | Exchange **forbidden** | fits |
| [g2](g2-structs.md) | Simultaneity — the swap case | Exchange **forbidden** (read before write, same cell) | fits |
| [g1](g1-dot-product.md), [g3](g3-generics.md) | Capture | — | **does not fit** |

## 2. Capture does not belong here — and that is good news

Capture is not a structural-rule problem. It is a property of how terms *represent* binding, and
substitution's failure to respect alpha-equivalence.

That matters because it changes the fix from a check to a **representation choice**. Under a
locally-nameless representation (Charguéraud), de Bruijn indices, or nominal sets
(Gabbay–Pitts), capture is not detected — it is **unrepresentable**.

So hygiene leaves the analysis list entirely and becomes a decision about the term type. That is
strictly cheaper than what [g1 §6](g1-dot-product.md) assumed, which was a freshening pass.

**Four of five collapse; the fifth exits by a better door.**

## 3. It is two axes, not one

Contraction and weakening answer *how many times*. Exchange answers *in what order*. Those are
independent, and forcing them into one grading is over-unification.

| Axis | Question | Covers |
|---|---|---|
| **Multiplicity** | How many times may this be used? | Sharing, linearity, effect duplication, effect elision |
| **Ordering** | May this move relative to other operations? | Effect ordering, simultaneity |

The ordering axis is small: not algebraic effects with handlers, just **pure versus stateful**.
Pure terms float freely; stateful terms are pinned to their statement position. Two colours.

And there is a pleasing consequence — **ANF normalization is the ordering axis's normal form**,
not a separate pass. [g3 §6](g3-generics.md) and [g5 §4](g5-bindings.md) each demanded ANF for
unrelated reasons; both are this axis making evaluation order explicit.

## 4. The multiplicity axis already has our staging annotation in it

Quantitative Type Theory (Atkey; implemented in Idris 2) grades every variable **0, 1, or ω**:

| Grade | QTT meaning |
|---|---|
| **0** | erased — not present at runtime |
| **1** | used exactly once at runtime |
| **ω** | unrestricted |

Now compare what [g6](g6-escaping-closures.md) established about this core:

| Grade | What it means here |
|---|---|
| **0** | eliminated by rewriting — does not appear in the residual |
| **1** | survives, used once — an accumulator, updatable in place |
| **ω** | survives, used many times — needs sharing, and possibly allocation |

**These are the same three grades.** Grade 0 *is* compile-time-only. Which means:

> **The multiplicity annotation and the binding-time annotation are one thing.**

g6 argued that binding-time analysis should be surfaced to the programmer as a per-abstraction
guarantee — "this fold is eliminated" versus "this handler survives: 16 bytes, 1.55 ns." That
report is not a separate analysis bolted onto the type system. **It is the type system's
multiplicity column, printed.**

One caveat on the borrowing: QTT's grade 0 means *irrelevant to computation*, while ours means
*computed at compile time, value possibly inlined into the residual*. Same shape, not identical
semantics. Steal the structure, not the metatheory.

## 5. Checking it against the derivations

A unification is only worth anything if it reproduces the answers that were derived
independently.

**g4's auto let-binding.** Current rule: bind a repeated metavariable if the matched term is
"non-trivial." That is a heuristic. Under grading it becomes principled:

> Contraction of a **grade-0** term is free — duplicating compile-time work costs only compiler
> time. Contraction of a **grade-1 or ω** term duplicates runtime work and requires a binding.

That gives the same answers as the heuristic — literals and variable references are free,
computations are not — but from a rule rather than a guess. The [g1](g1-dot-product.md)
refinement ("only bind non-trivial terms") falls out rather than being patched on.

**g2's swap.** `(set acc (point (.y acc) (.x acc)))` — reads and writes on the same cell are
stateful, so the ordering axis forbids sequencing them naively. Detected, and the temporaries
follow.

**g5's effect in a loop.** Moving a term into a loop multiplies its multiplicity by *n*. If it
is stateful or grade 1, that is a violation. If it is pure and grade ω, it is fine. Exactly the
execution-context-depth check g5 proposed, derived instead of stipulated.

**g6's escape analysis.** A closure whose environment is used at unknown future times is grade
ω, so it cannot be stack-allocated and must go to the heap. A closure fully eliminated by
rewriting is grade 0. g6 observed that *"rewriting is its own escape analysis."* Sharper:
**the grading is the escape analysis, and it is the same annotation as binding time.**

Four independent derivations, four correct reproductions, and in three of them the unified rule
is better than what was derived by hand.

## 6. What the machinery list becomes

Before — eleven items:

> auto let-binding · layer stratification · linearity analysis · hygiene · range analysis with
> `require` facts · deforestation measure check · ANF normalization · monomorphization for
> recursive generics · polymorphic type checking · SROA with parallel-assignment temporaries ·
> effect-context checking on rules

After — eight:

| # | Item | Absorbed |
|---|---|---|
| 1 | **Graded type system** (multiplicity 0/1/ω × pure/stateful) | auto let-binding, linearity, effect-context checking, parallel-assignment temporaries, escape analysis, binding-time analysis |
| 2 | Locally-nameless term representation | hygiene — by construction, not by check |
| 3 | ANF normalization | *is* the ordering axis's normal form |
| 4 | Layer stratification | — termination, unrelated |
| 5 | Deforestation measure check | — termination, unrelated |
| 6 | Range analysis with `require` facts | — arithmetic, unrelated |
| 7 | Monomorphization for recursive generics | — unrelated |
| 8 | SROA (splitting only) | temporaries now come from #1 |

Eleven to eight is the smaller part of the result. The larger part: **item 1 does six jobs, and
one of them is a feature the pitch needs rather than a tax it pays.**

## 7. The cost, stated honestly

Graded and substructural type systems have a bad usability record, and pretending otherwise
would be exactly the kind of unmeasured optimism this project keeps catching itself in:

- **Rust** — affine ownership, the mainstream success, and famously hard to learn.
- **Linear Haskell** — multiplicity-polymorphic, widely described as awkward in practice.
- **Idris 2 / QTT** — quantities are a real usability complaint.
- **Clean's uniqueness types** — decades old, never escaped a niche.

Against requirement 8 (legible to models) and general usability, this is the biggest risk the
design has taken on so far.

**The design move that answers it: infer and report, do not declare and check.**

Rust makes you *write* the discipline. Here the programmer writes ordinary code and the compiler
*tells* them the answer:

```
fold        eliminated at compile time
handler     survives: 1 allocation, 16-byte environment, indirect call
acc         linear — updated in place
```

Annotation is required only where you want to **constrain** rather than observe — a library
declared portable, or a function declared allocation-free. That inverts the burden: the
discipline becomes an *output*, which is what [g6 §9](g6-escaping-closures.md) wanted, rather
than an input tax.

Whether inference is strong enough to keep annotations rare is **untested and is the thing to
test next.** Perceus (Koka, Lean) is the closest prior art for inferring exactly this without
programmer annotation, and is worth reading before committing.

## 8. Does this change what the core is?

[g6](g6-escaping-closures.md) landed on:

> Everything is a function, evaluated at compile time. What survives is what the target must do
> at runtime, and the compiler will tell you exactly what that is.

The grading does not replace that — it is what makes the second sentence *checkable* rather than
aspirational. Combining them:

> **Terms, rewritten by rules. Every term graded by how many times it may be used and at which
> stage. Grade 0 means it is gone before the program runs — and the compiler will tell you the
> grade of anything you wrote.**

That is small enough to state in three lines and it now covers the whole design: the rewriting
(mechanism), the staging (why abstraction is free), and the grading (why it is sound, and how
the programmer finds out).

## 9. Findings

1. **Four of five collapse** onto structural rules; capture does not, and exits into a
   representation choice, which is cheaper than the analysis it replaces.
2. **Two axes, not one** — multiplicity for copy/delete, ordering for reorder. Forcing them
   together would be over-unification.
3. **The ordering axis is two colours**, pure versus stateful, not an effect system with
   handlers.
4. **ANF is the ordering axis's normal form**, not a separate pass — which explains why g3 and
   g5 demanded it for unrelated reasons.
5. **Grade 0 is the staging annotation.** Multiplicity and binding time are the same column, so
   g6's user-facing cost report comes out of the type system rather than beside it.
6. **The grading is also the escape analysis** (g6), and the parallel-assignment temporaries
   (g2), and the auto let-binding rule (g4) — with g4's heuristic becoming a consequence.
7. **Machinery drops from eleven to eight**, and the survivor does six jobs.
8. **The usability record of substructural systems is bad**, and this is the design's largest
   assumed risk.
9. **Infer and report, do not declare and check** — the move that could make (8) survivable, and
   it is untested.

## 10. Verdict

The thread was worth pulling. It did what the [assessment](../assessment-2026-08-13.md) hoped:
materially changed the answer to "is this easy to implement," and did so downward.

It also did something unhoped-for — the staging annotation and the substructural annotation
turn out to be the same thing, which means the feature g6 wanted to surface to programmers is
produced by the machinery that was already needed for soundness. Two requirements, one
mechanism, which is the pattern this design keeps hitting when it is on the right track.

**What would falsify it:** if inference turns out to be too weak to keep annotations rare, the
usability risk in §7 becomes real and requirement 8 is in danger. That is the next experiment,
and it is cheap — take the five gauntlet programs, write them without any multiplicity
annotation, and check by hand whether every grade is inferable from use.

**Still open:** compile-time computation (L2). Grade 0 says a term is evaluated at compile time,
but all recursion is currently residual, so compile-time *recursion* — building a table, a
parser, a specialized routine — has no story. That remains the largest unexplored area of the
design, and the grading has just given it a natural place to live.
