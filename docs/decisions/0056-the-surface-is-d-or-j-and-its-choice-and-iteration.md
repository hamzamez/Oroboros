# 0056 — The surface is D or J; one arm grammar, positional `again`, and J's collection formers

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §3.9 and §3.10: "I agree to one arm grammar,
retire cond and match … agree to again positional … agree ; inside arms in D and K … agree VAR/var …
agree, one-armed if in J … sure drop B′ … in fact moving forward it is D or J"; and §3.10's four, all
agreed). §3.9's items 2 (the handled bind) and 7 (a loop's invariant slot) are not decided here. Not
built: the surface is built when it is chosen. **Amended 2026-10-12**: item 7 decided (hamza: "I agree
to the invariant slot"): a loop's invariant has an optional slot in D and J, `LOOP i == 0 INVARIANT I :`
and `loop i = 0 invariant I { … }`, absent meaning ⊤. **Items 8 and 9 superseded by
[ADR 0059](0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md)**: the rule moves inside the
former, `array(n, f)` goes, and the set is `Array(V)`.

> **Item 1 superseded by [ADR 0068](0068-j-is-the-surface.md)**: J is the surface.

## Context

surface-design.md designs the surface one construct at a time over abstract.md's signature, which is
decided (ADR 0050). Installments 1–3 carried four candidate notations: B′ (fitted S-expressions), D
(mathematical, TLA+'s lexicon), K (D's lexicon as statements without brackets) and J (Go-inspired, braced
blocks). Installment 3 measured choice and iteration over the corpus (`gauntlet/surface/control.go`) and
eight programs in each notation (`tokens.go -section 3`); §3.10 argued hamza's proposal for array
literals (`gauntlet/surface/data.go`).

## Decision

1. **The source notation is D or J.** B′ and K are dropped as candidates.
   - B′ measured 1.30–1.47× today's size and deeper than today in every installment that measured it;
     writing the semantics faithfully (arms as pairs, indexing marked) is what made it so.
     S-expressions stay as abstract.md's T′, the rendering for tools and interchange.
   - K and D were within a few tokens in installment 3 and differed only where a scope introduces
     state. J carries K's statement lists; D keeps `LET … IN` groups.
2. **One arm grammar**, `p ⟨if φ⟩ → body`, for `case` and `loop`. A `case` with no scrutinee is the
   guarded choice, and a loop's arms are its state machine, so `cond` and `match` are not keywords.
   `if` stays, derived, naming 𝔹's eliminator (ADR 0017). This is ADR 0050's decision 5 made concrete,
   and it closes two holes: a guard on a case arm, and patterns and guards in one form.
3. **`again` is positional**: an application of the loop's λ, its arity checked by W3. ADR 0055's
   derivation (a loop variable is bound by application) gives the same answer.
4. **In D, `;` binds tighter than `[]`**: an arm's body may be a sequence, a chain extends to the end
   of what encloses it, and a chain or `IF` that something follows, or a chain nested in an arm, is
   parenthesized. That is the guarded commands' own precedence, the cheapest by the corpus (31 + 22
   sites, against 88 + 22 for `;` loosest, or 393 closing words).
5. **In J, a state binder's introduction is marked**, `var x := e`, and a write is `x := e`. Σ's binder
   and its write are two operations, and spelling them alike would let a misspelled write create a
   cell. D marks the introduction by its place in a `LET` group.
6. **A one-armed `if` in J**, `if c { a }`, the statement if(c; a; ()). Not in D, where it would bring
   back the dangling else.
7. **In J, a finite collection given by its elements is `former { e, … }`**: `array { … }`, `set { … }`
   (ADR 0050's set former), and `map { k ↦ v, … }` when installment 4 reaches maps. J's braces are
   blocks, so `{x}` alone could not also be a set. A bare `{ … }` is a block.
8. **In J, a table by a rule is `array(n, f)`**, and the name `table` goes. The hazard is accepted:
   `array(3, 0)` and `array{3, 0}` differ by one bracket and are both well-typed, as Rust's `[0; 3]` and
   `[0, 3]` are.
9. **The set of sequences is `Seq(S)`**, never `array(S)`, so a value and its set are never spelled
   alike. Today `(array T)` is the type at 38 sites and spells like a one-element table.
10. **The comprehension `[k ∈ D ↦ e]` stays** until installment 4 decides it with tables and maps as one
    finite function.

## Why not

- **Keep B′.** Its one merit, a failure the reader catches, D and J now have by their own rules (item
  4, braces), at today's size.
- **Keep K.** It differs from D only in how a scope introduces state, and J already has statement
  lists. Two notations that close are a choice the remaining installments cannot separate.
- **`cond` and `match` as keywords.** Each is a spelling of the one arm grammar (fit-research's F1).
- **Named `again`.** It is Dijkstra's concurrent assignment, which goes with loop variables as state;
  ADR 0055 made them parameters.
- **`;` loosest in D** (§2.9's first claim): every arm whose body is a sequence would be parenthesized,
  88 sites, and installment 2's examples were written under two rules because of it.
- **Bare braces for a set in J**, a homonym with the block; **a second name for the rule**, the
  `table` this decision drops.

## Consequences

- surface-design.md's examples in D and J are the live ones; B′'s and K's stay as the record.
- abstract.md is unchanged: `cond` stays a derived operator of Σ, and `match` was never one. Only the
  surface stops writing them.
- Open from installment 3: the handled bind (§3.9, item 2, with 2b in a loop's arm) and a loop's
  invariant slot (item 7). Then installment 4, data and sets, and the comprehension.
