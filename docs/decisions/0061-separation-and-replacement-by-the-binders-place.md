# 0061 — Inside a former, separation and replacement are told apart by the binder's place; the colon is set-builder's

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §4.10: "agree"). Completes
[ADR 0059](0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md)'s former shape, whose
separator it left open. Amends [spec/abstract.md](../spec/abstract.md): the logic gains the image of a
set. Not built.

## Context

ADR 0059 gave every collection one shape, `former { … }`, and dropped `array(n, f)`. hamza proposed `:`
as the separator inside it. surface-design §4.9 read `binder : body` as one λ in every former, the
binder always first. hamza then pointed to TLA+'s two set-builders: `{x ∈ S : P}` filters and
`{e : x ∈ S}` maps, as in `{2 * x : x ∈ {1, 2, 3, 4}}` = {2, 4, 6, 8} and `{x + y : x ∈ 1..2, y ∈ 1..2}`
= {2, 3, 4}.

They are two operations of set theory, separation (Zermelo 1908) and replacement (Fraenkel 1922). Both
take a λ over S, and TLA+ tells them apart by where the binder stands. Its colon reads "such that" in
both, since {e : x ∈ S} is {y : ∃x ∈ S. y = e}.

## Decision

1. **`set { x ∈ S : P }` is separation**: the elements of S for which P holds.
2. **`set { e : x ∈ S, … }` is replacement**: the values of e as its binders range over their sets, a
   product when there are several.
3. **`array { e : k ∈ 0..n-1 }` is an ordered image**, the replacement of 0..n−1 under λk. e, kept in
   order, which is a table: table(n; λk. e). Its binder is one, ranging over 0..n−1. The zero-filled
   buffer is `array { 0 : _ ∈ 0..n-1 }`.
4. **`map { k : v, … }` is a graph only**; a map has no rule form (maps.md §3.2).
5. **The colon is set-builder's "such that"** in sets and arrays, and a pair's separator in a map.
6. **A binder-first form `x ∈ … : …` is a filter**, so `{x ∈ T : x ∈ S}` is T ∩ S, and the image of a
   membership test is parenthesized, `{(x ∈ T) : x ∈ S}`, as TLA+ reads them.
7. **D writes TLA+'s**: `{x ∈ S : P}`, `{e : x ∈ S}`, and `[k ∈ 0..n-1 ↦ e]` for a table's rule.

## Why not

- **`↦` for "maps to" beside `:` for "such that"** (surface-design §4.8): under 𝒫(S) ≅ Bool^S both bodies
  are a λ's, so J would spell one λ two ways.
- **The binder always first, the image a second former** (§4.9): it puts an array's rule in a filter's
  shape, needs a word for the image that mathematics and TLA+ write in `set`, and makes D and J differ.
- **A rule form for maps**: a map's index set is its data, and a rule over it would still store it.

## Consequences

- abstract.md's logic gains **image(x̄ ∈ S̄; e)**, the replacement {e : x̄ ∈ S̄}, beside compr, the
  separation. It is a static set like every set; nothing below ε sees it.
- surface-design's E3 writes `array { k * k : k ∈ 0..n-1 }`.
