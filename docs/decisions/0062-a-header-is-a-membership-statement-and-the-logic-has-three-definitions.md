# 0062 — A header is a membership statement; bounds are binder sets; one equality in formulas; `fn`, `spec` and `lemma` share the header

Date: 2026-10-13
Status: Accepted (hamza, 2026-10-13, on surface-design.md §5.7: "I agree to all 8 recommendations").
Amends [spec/abstract.md](../spec/abstract.md): a binder list is a telescope; a lemma has binders, a
precondition and a conclusion, and is used by `assert`; the logic's ≐ is written with the computation's
equality. Not built: the surface is built when it is chosen.

## Context

surface-design installment 5 wrote the second language of ADR 0049, the logic: a definition's claim,
an assertion, an invariant, a specification function, a lemma, and the formulas they are written in.
`gauntlet/surface/claims.go` measured how the corpus writes claims today:
- every formula is a comparison or a conjunction of comparisons over parameters, `len` and linear
  arithmetic: Presburger arithmetic with lengths;
- of 89 `where` clauses, 66 only bound a parameter, 20 relate two (`len p = len q`), 3 are disjunctions;
- programs have 0 `ensures`, 0 `the`, and the corpus 0 assertions, invariants, specification
  functions, lemmas or quantifiers, because no notation can write them;
- the 18 `ensures` in target files read `result`, a name nobody binds.

## Decision

1. **The header is the membership statement** f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ} (abstract.md
   §2.6). Parameters carry their sets; the result is a binder, `(r ∈ U)` when ψ names it and its set
   alone, `(U)`, when not; a tuple result is a group, `(q ∈ Int, m ∈ 0..b-1)`; then `where φ` and
   `ensure ψ`. **`result` as a keyword goes.** D and J share the header and differ in their keywords and
   the body's delimiter (`{ }` against `==`):
   ```
   fn div(a ∈ Int, b ∈ Int) (r ∈ Int) where b != 0 ensure abs(r) <= abs(a) { a / b }    // J
   div(a ∈ Int, b ∈ Int) (r ∈ Int) WHERE b ≠ 0 ENSURE |r| ≤ |a| == a ÷ b               \* D
   ```
2. **A bound is the binder's set, and a later binder's set may name an earlier binder**: a binder list
   is a telescope (de Bruijn 1991; Martin-Löf's Σ), P = {x̄ : x₁ ∈ S₁ ∧ x₂ ∈ S₂(x₁) ∧ …}:
   `fn get(a ∈ Array(Int), i ∈ 0..len(a)-1)`. **A relation between parameters stays in `where`**:
   `where len(q) == len(p)`.
3. **One equality in a formula, the logic's ≐**, written `==` in J and `=` in D. The computation's
   equality is ≐ restricted to the sorts whose equality it decides (integers, booleans, tags, and what
   the representations compare), and is its characteristic function there. A comparison in the
   computation at a sort it cannot decide is refused, as W6 refuses an undecidable membership test.
4. **A formula is an expression with four more forms**, each legal only in a claim position (W6):
   quantifiers, J `forall x ∈ S : φ` and `exists x ∈ S : φ`, D `∀ x ∈ S : φ` and `∃ x ∈ S : φ`, several
   binders quantifying over their product; implication and equivalence, J `==>` and `<==>` (Dafny's), D
   `⇒` and `≡`; membership in any set; a specification function's call. The quantifier's colon is
   ADR 0061's: Church's quantifier applied to λx. φ.
5. **Three definitions share the header and differ in their body**:
   - `fn` computes, and its claim is proven (Dafny's `method`);
   - `spec` defines a value of the logic, may recur, and never runs (Dafny's `function`, TLA+'s
     `RECURSIVE`);
   - `lemma` has a proof for its body: assertions, cases and uses of lemmas (Dafny's `lemma`, TLAPS's
     `THEOREM … PROOF`). Its header has binders, `where φ` and `ensure ψ`, and no result.

   **A lemma is used by `assert L(ā)`, by the rule of a call**: ā ∈ S̄ ∧ φ[ā/x̄] is an obligation
   (ADR 0028) and ψ[ā/x̄] becomes a fact (postconditions.md's exact swap). **A lemma that uses itself on
   smaller arguments is a proof by induction**, owing the well-foundedness a spec's recursion owes,
   checked by size-change (Lee, Jones and Ben-Amram 2001; under Curry–Howard a well-founded recursive
   proof is an induction).
6. **`assert` proves, `expect` checks.** `assert φ` is static: proven from the path's facts, then a
   fact; it never runs, ε erases it, and an unprovable one refuses the program. `expect` runs and
   abandons on failure (ADR 0040).
7. **`the` leaves the surface.** A binder's set states a claim on an intermediate value
   (`b ∈ 0..255 = src[i]`), and a header's result set a claim on a host's result. abstract.md keeps the
   operator, the image of today's `(the S e)` under μ.
8. **No claim stays distinct from `any`.** A binder without a set claims nothing, its value known by
   what the analyses derive; `any` is a set, every value (settypes §10).

## Why not

- **Keep `result`** as the postcondition's name: ADR 0051 made the result a binder whose pattern may be
  a tuple, and a keyword cannot name a tuple's components.
- **Every bound in `where`**, as today: it names each parameter and its set twice, for 66 of 89 clauses.
- **A relation as a dependent set**, `q ∈ { t ∈ Array(F64) : len(t) == len(p) }`: a comprehension to
  state an equation, where `where` states it.
- **Two equalities**, the computation's and a second glyph for ≐: on every sort the computation
  decides they are one relation, and elsewhere the computation's is undefined, not different. Two glyphs
  would be a synonym where they agree.
- **A lemma as a sentence `lemma(n; φ; steps)`** (abstract.md before this ADR): without binders it cannot
  be instantiated at a use, and induction needs the argument that decreases.
- **`assert` as a run-time check**, as C, Go and Python read it: that is `expect`'s job; the confusion
  fails safe, since an unprovable `assert` is refused and the refusal can name `expect`.
- **`the` in the surface**: no program uses it, and every use in a target file is a result set.

## Consequences

- abstract.md: `lam`'s and `spec`'s binder lists are telescopes; `lemma(n; b̄; φ; ψ; steps)`; 𝒪 of a
  lemma's use; the ≐ note; `the` marked as μ's image.
- surface-design's G1–G5 are the decided spellings, modulo D or J.
- The target files' 18 `ensures` will read the result's binder, and their 9 `the` become result sets,
  when the surface is built.
