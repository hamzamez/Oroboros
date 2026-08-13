# Exploration: is multiplicity inferable without annotation?

Exploration only. No commitments, no ADR.

[s1](s1-substructural.md) took on the design's largest assumed risk: substructural systems have
a poor usability record, and the proposed mitigation — **infer and report, do not declare and
check** — was untested.

The test: write the five gauntlet programs with **no** multiplicity annotations and check by
hand whether every grade falls out of use.

**Result: zero annotations are needed in application code across all five programs. One
annotation is unavoidable, and it lands in binding files where the author already knows the
answer. But the test also refutes one of s1's rows — accumulator safety is not a multiplicity
property at all.**

---

## 1. The rules being tested

Grades are `0` (eliminated before runtime), `1` (survives, used once), `ω` (survives, shared),
over the semiring where `1 + 1 = ω`.

| Situation | Grade |
|---|---|
| Occurrence under a loop | ω |
| Occurrences in separate branches of a conditional | least upper bound, not sum — only one arm runs |
| Two occurrences in sequence | sum → ω |
| No occurrence in the residual | 0 |

Prior art for exactly this: GHC's demand and cardinality analysis (Sergey, Vytiniotis, Peyton
Jones), and Perceus (Koka, Lean) for the in-place-update half.

## 2. Grade 0 is observed, not inferred

The first thing the test found is a simplification.

Grade 0 means *eliminated by rewriting*. Whether rewriting eliminates a term is not something
that needs predicting — **rewrite to fixpoint, then look at what is left.** Anything absent from
the residual was grade 0.

So no analysis computes grade 0. It is read off, for free, from a step the compiler performs
anyway.

That matters for the reporting story in [g6 §9](g6-escaping-closures.md): the per-abstraction
cost report is not a prediction, it is an observation, and observations do not need to be
conservative. A prediction would have to say "may survive"; this can say "survived."

The one case needing a *prediction* is a declared constraint — a library asserting a function is
zero-cost. That is checked by comparing observed grade against declared grade, which is the
"declare to constrain" case s1 already carved out.

## 3. Program by program

**G1 — dot product.** `xs`, `ys`, `n`, `i` all occur under the loop → ω, syntactically.
`sum`, `zip`, `dot`, `fold-range`, and the fold's lambda are absent from the residual → 0,
observed. `acc` is discussed in §4.

**G3 — generics.** `step` and `fold` are absent from the residual → 0. `init` occurs once → 1.
And the type parameters `T` and `A` are grade 0 always — which is *literally what QTT introduced
grade 0 for*. The match is not a coincidence; erasure of type arguments is the original case.

**G2 — structs.** `ps` is ω under the loop. The `(point …)` constructor in the fold body is
absent from the residual after SROA → 0. **Scalar replacement of aggregates shows up as a
grade-0 observation**, not as a separate fact to track.

**G4 — word count.** `text` occurs once → 1. `xs` is ω. `acc` holds a dict and is discussed in
§4.

**G6 — closures.** `ops` occurs once → 1. In `make-scaler`, `f` is captured by a lambda that
survives into the residual and is returned; the closure may be called any number of times, so
`f` is ω. Inferable, because the escape is visible in the residual — the same observation that
[g6 §6](g6-escaping-closures.md) called "rewriting is its own escape analysis."

**G5 — output.** `label` occurs once → 1; `xs` is ω after `dot` inlines. The three `print-line`
calls are the ordering axis, and they are §5.

## 4. The correction: accumulators are not a multiplicity property

[s1 §1](s1-substructural.md) mapped "accumulator must not be aliased" onto *contraction
forbidden*. **That row is wrong.**

Contraction forbidden means grade 1 — used exactly once. But an accumulator in a loop is read
and written on every iteration, so it is used *n* times. By counting, `acc` is ω. The two
statements are incompatible, and counting is right.

What the accumulator actually needs is **uniqueness** — that no *other* reference to its storage
exists — which is a different property from linearity, and in a precise sense dual to it.
Linearity constrains the future (this will be used once); uniqueness constrains the past (no
other reference has been made). Clean's uniqueness types and the linear/unique distinction are
the literature here.

So where does uniqueness come from, if not from the grading?

**For value-typed accumulators, it is free.** [g2 §4](g2-structs.md) decided structs are values
with no interior pointers, so nothing *can* alias an `f64` or a `point` local. The property is
guaranteed by a decision already made, exactly as capture-freedom is guaranteed by the term
representation. No analysis.

**For heap-typed accumulators it is not free.** G4's `acc` holds a dict. `(set acc (dict-update
acc …))` reads as a functional update and is emitted as in-place mutation, which is sound only
while no other reference exists. That is inferable — `acc` is created by `dict-empty`, threaded
read-modify-write through the loop, and returned once — but **inferring it requires liveness and
threading, not occurrence counting.** Textbook, and more than the semiring gives.

Two things follow. s1's collapse is slightly smaller than claimed: the accumulator row leaves
the multiplicity axis, splitting into "free from value semantics" and "needs liveness." And the
grading is genuinely about *how many times*, with *how many references* handled elsewhere — a
cleaner separation than s1 drew.

## 5. The one annotation that cannot be inferred

The ordering axis needs to know whether a term is pure or stateful. For our own code that is
inferable bottom-up from the leaves. **For an extern it is not** — there is no body to analyze.
`fmt.Println` is stateful; `math.Sqrt` is pure; nothing in the signature distinguishes them.

So **every extern declares its purity**, and this is unavoidable.

It lands well, though:

- It goes in the **binding file**, which already lists names, types, and an import line
  ([g5 §2](g5-bindings.md)). One more column.
- The binding author is the person who knows.
- Application programmers never write it.
- **The safe default is `stateful`**, so bulk-generated Tier 2 bindings are correct by
  construction and merely forfeit some optimization until someone marks a function pure.

One genuine hazard: a binding that wrongly declares a stateful function pure permits the
compiler to reorder or delete its effects. **That is an unsoundness sourced from data rather
than from code**, and it will not be caught by any check on our side. Bulk-generated bindings
defaulting to stateful is the mitigation, and marking something pure should be treated as a
claim requiring the same care as an FFI signature.

## 6. Scorecard

| Fact needed | Where it comes from | Cost |
|---|---|---|
| Grade 0 — eliminated | **Observed** in the residual | free |
| Type parameters are grade 0 | Always | free |
| 1 vs ω | Occurrence counting | standard usage analysis |
| Loop → ω, branches → lub | Syntactic | free |
| Value accumulator uniqueness | **Guaranteed** by value semantics (g2) | free |
| Heap accumulator uniqueness | Liveness + threading | textbook, beyond counting |
| Escaping capture → ω | **Observed** in the residual | free |
| Purity of our own code | Bottom-up from leaves | small |
| **Purity of an extern** | **Declared** | one column in binding files |
| Slice aliasing under mutation | — | **untested** |

**Annotations required in application code across all five gauntlet programs: zero.**

## 7. Findings

1. **Zero annotations in application code**, across all five programs. The mitigation s1
   proposed for its own largest risk holds, on this evidence.
2. **Grade 0 is observed, not inferred** — read off the residual after a step the compiler
   already performs. The cost report is an observation, not a conservative prediction.
3. **Type parameters are grade 0**, which is QTT's original motivating case.
4. **s1's accumulator row is wrong.** Accumulators are ω by counting; what they need is
   uniqueness, which is a different property. Value-typed accumulators get it free from
   [g2](g2-structs.md)'s value semantics; heap-typed ones need liveness.
5. **One unavoidable annotation: extern purity.** It belongs in binding files, defaults safely
   to stateful, and application programmers never see it.
6. **A wrongly-declared-pure extern is an unsoundness sourced from data.** No check on our side
   catches it.
7. **Slice mutation is untested.** No gauntlet program mutates through a slice, and slices *can*
   alias — `dot(v, v)` is legal. This is where uniqueness would actually be needed and is not
   yet known to be free.

## 8. Verdict

The risk s1 flagged as the design's largest is materially reduced. Rust makes you write
lifetimes; Linear Haskell makes you write multiplicities; on this evidence Oroboros makes you
write nothing, because grade 0 is observed rather than predicted and the rest falls out of
counting and value semantics.

Two honest qualifications:

- **Five small programs is weak evidence.** None of them mutates a shared data structure,
  aliases a slice, or builds a graph — the cases where uniqueness stops being free and Rust's
  difficulty actually begins.
- **The untested hole is specific and namable:** mutation through a slice. That is the next
  thing to derive, and it should be a *new* gauntlet program rather than a thought experiment,
  since [ADR 0008](../decisions/0008-measurement-over-principle.md) applies — a sixth program
  exercising in-place mutation of a shared structure.

The collapse from s1 survives, one row smaller and better separated: **multiplicity answers how
many times, uniqueness answers how many references, and only the first is a grading.**
