# 0050 — The abstract syntax is the initial algebra of the semantics: modes on binders, one definition, explicit sets, pairs, one choice, marked indexing

Date: 2026-10-10
Status: Accepted (hamza, 2026-10-10: decision 1 by "specify", decisions 2–6 by "decided. agreed to
recommendation"). Decides [fit-research.md](../fit-research.md) §7 decisions 1–6. Decision 7, the
surface notation, stays with [syntax-research.md](../syntax-research.md). Specified by
[spec/abstract.md](../spec/abstract.md). **Not built.** Amends the surface of ADR 0020 and ADR 0048 and
the spelling of ADR 0049 §5, not their meaning.


> **Item 4 amended** by ADR 0064 item 8: in a set position a value denotes its singleton
> ([spec/j.md](../spec/j.md) §0.2, C1).

## Context

notation-research and syntax-research asked which notation reads and writes best. hamza then asked
which syntax *maps* best to the semantics, the type side and the computation side. fit-research
answered with compositionality: a syntax and a semantics are algebras over one signature, and a
meaning is a homomorphism (Montague 1970; Goguen, Thatcher, Wagner and Wright 1977). A syntax fits
when its abstract syntax is the initial algebra of the semantic signature. Misfits are countable:
synonyms, homonyms, hidden binders, missing or spurious distinctions, and invariants checked rather
than written.

Measured and derived, fit-research found:
- **most of the fit is in the abstract syntax.** Prefix against infix is decided by the semantics in
  one place only, claims read as mathematics. β's textual substitution, the expected argument for
  prefix, failed in 6 of 1,590 parameter occurrences in the corpus;
- **every notation written so far, today's included, misfits in the same places**: usage written as
  a set, a value punned with its singleton, a claim split across five forms;
- **today's notation is the furthest from the semantics**: eight binder shapes for three kinds of
  binder, and five forms whose structure comes from parity.

## Decision

1. **The language is specified first as abstract syntax**: the initial algebra of a many-sorted
   binding signature, with the meanings (erasure, desugaring, obligations, usage, effects) as
   homomorphisms out of it, and every notation a rendering of it (spec/abstract.md). The tail rule
   (ADR 0015) and the logic's boundary (ADR 0049) are sorts, not checks.
2. **Usage is a mode on a binder, not a set.** `linear` (ADR 0018, 0020), `inout` and `out`
   (ADR 0048's value-result: Algol W, Ada) are written on a binder. The type formers `(buffer V)` and
   `(cell T)` go. `relevant` stays on a variant's declaration as the default mode of binders ranging
   over it (ADR 0043), and `pure` stays a marker of a declaration. Quantitative type theory is the
   precedent (McBride 2016; Atkey 2018).
3. **One definition carries its claims**: parameters with their sets, a precondition (a comprehension
   over the parameter tuple), a result set and a postcondition (a comprehension over the result).
   That is f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}, a Z schema. It replaces `sig`, `where`, `requires`
   and `ensures` beside `def`. A host declaration is the same form with a host body.
4. **A set is formed explicitly.** A singleton or enumeration is written with a set former, and a
   complement with an explicit difference, S ∖ T. A literal is never its own singleton by position,
   and `or`, `and` and `not` stay connectives on formulas. This amends the spelling of settypes-research
   §7.2 and leaves ADR 0049's decision as it is.
5. **A pair is a pair, and there is one guarded choice.** A binding group is a sequence of (binder,
   value) pairs, and a loop's clause and a choice's arm are each (pattern, guard, body), in every
   notation. `case` on constructors, on literals, on sorts and on guards is one operator. `if`, `cond`,
   `try`, `expect`, `seq` and the connectives are derived operators, each defined by an equation and
   kept because it names a law. `match` is a loop whose arms have patterns.
6. **Data application is marked.** Indexing a table or map is its own operator, written `t[i]` or its
   prefix spelling, distinct from applying an operator `f(x)`. The line it draws is ADR 0049's: tables
   and maps are values in V, and functions are not. data.md's "indexing is application" stays true of
   the meaning.

## Why not

- **Choose the surface notation first** (syntax-research's question): five of the six decisions
  above hold in every notation compared. Deciding prefix or mathematics first would leave them open,
  or settle them by accident in whichever notation won.
- **Usage as type formers** (`(buffer V)`, `Buffer<T>`, Rust's `&mut T`): a frozen buffer *is* a
  table, so `(buffer V)` and `(array V)` have the same values. Writing usage as a set says some values
  are buffers, and makes `(array (buffer V))` look meaningful, which ADR 0020 then refuses by a special
  rule. Linearity is a substructural judgement (Girard 1987), not membership (settypes §2.5).
- **`sig` beside `def`**: two forms name one object, so the checker checks the claim "in two
  directions" and the two can disagree. Typed Racket's `(: f T)`, F\*'s `val` and Shen's `{A --> B}` are
  the same split.
- **A literal as its singleton by position** (TypeScript, Typed Racket): consistent only if a set is
  identified with its characteristic predicate and literals are coerced by position. That coercion is
  the context-dependence a homonym is, and it diverges exactly where a static value is both a term
  and a set (ADR 0049 §5). The same holds for `(not 0)`, whose complement is relative to a sort the
  form does not name.
- **Structure by parity** (today's `let`, `local`, `loop`, `case`, `cond`; Clojure's vectors): the
  structure is not in the tree. ADR 0024 had to refuse the datum comment `#;` because a comment could
  change which construct a form is.
- **Six eliminators**: `cond` and `match` used as a choice are spellings of guarded choice with no law
  of their own, which makes them synonyms. `try`, `expect` and `ignore` each name an algebraic
  operation, so they stay (Felleisen 1991: derived forms add no power; keep those that name a law).
- **Uniform application for tables and functions**: it fits the untyped reading, but the corpus never
  uses it. Of 119 applied parameters, none receives both a λ literal and a table literal, and the one
  program that could pass a table as a function writes the λ (`(fn (i) (a i))`). It hides the line
  between data that lives at run time and functions that staging removes.

## Consequences

- **No meaning changes.** spec/abstract.md defines ⟦P⟧ := ⟦D(ε(P))⟧, so claims are inert by
  construction, and its Theorem D3 is the build check: today's notation, mapped in by μ and desugared,
  must reproduce today's reader, with `cmd/check` byte-identical.
- **The built specs describe today's notation until it changes**: def.md ("a parameter list is a
  list of NAMES"), types.md (`sig` in two directions), local.md and target-files.md (`(cell T)`,
  `(buffer V)`), tables.md (indexing written as application). Each will be rewritten when the
  surface is, and not before. The decision is about the abstract syntax, and the surface waits for
  syntax-research's decision.
- **Target files change with the surface**: every `(buffer V)` and `(cell T)` parameter becomes a
  mode, and every `sig` becomes a definition with a host body. That is a mechanical translation, μ.
- **`#;` becomes admissible.** Once no form reads its structure from parity, ADR 0024's reason for
  refusing it is gone. Admitting it would be a successor to ADR 0024, and nothing here decides it.
- **Several walkers become redundant**: the ones enforcing the tail rule, and the claim-only checks.
  They are deleted only after a planted fault shows the sort catches what they caught.
- **The usage checks get a specification**: CheckLinear and relevance at β implement spec/abstract.md
  §8's requirements on an interval semiring. A disagreement between them and the specification is a
  bug in one or the other.
