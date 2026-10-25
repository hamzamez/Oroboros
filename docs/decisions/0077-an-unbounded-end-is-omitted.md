# 0077 — An unbounded end is omitted: `lo..` and `..hi`, and no `+inf`

Date: 2026-10-25
Status: Accepted (hamza, 2026-10-25: "go ahead with item 2", on the question of 2026-10-24: "do we have
to `+inf`? isn't there a better spelling? propose"; the 10-24 plan's item 2, assessment-2026-10-24 §5). Amends [spec/j.md](../spec/j.md) §2.9 and §6.1;
[ADR 0019](0019-precision-by-declaration.md)'s spelling `(int 0 +inf)` stays the core's.

## Context

Four programs in `examples/big/` declare a result `0..`+inf``. The backquotes are σ_J's fallback for a
name that is no identifier (j.md §1.5), so the endpoint reads as a host's name: `` `+inf` `` is spelled
like `` `big+` `` and `` go.`>=` ``, and it is neither a name nor an integer. unbounded-rung.md §4.2
named both faults when the core took `+inf`: ℤ has no infinite element, so "`inf` is an endpoint in
the extended reals rather than an integer", and "`+inf` as a *token* wants a reader rule that exists
nowhere else". J kept the core's spelling by quoting it.

**What the set is.** A range is a convex subset of the chain (ℤ, ≤): a set C with x ≤ y ≤ z and x, z ∈ C
implying y ∈ C. In a chain with no ends, the non-empty convex sets are the intervals [a, b], the up-sets
↑a = {x : a ≤ x}, the down-sets ↓b = {x : x ≤ b}, and ℤ, and they are closed under ∩: ↑a ∩ ↓b = [a, b].
So a range is fixed by **which bounds it has**, each end present or absent, and **an absent bound is not
an element**. ℤ̄'s ±∞ are the ends of an *abstract domain* (the IR's intervals, `[-inf, +inf]` in a
report), which needs a top; a *set* needs none, since ↑a is a set of integers and nothing else.

**What declaring one does today**, measured 2026-10-25. A range above the target's word is a
representation request as well as a set (ADR 0019, ADR 0029): it widens a value into the rung above the
word. `fn inc(n ∈ 0..`+inf`) { n + 1 }`, applied to an `x ∈ 0..100` inside a definition whose result is
`0..200`, is refused: "a range above the word is a WIDENING, not a refinement". So is
`n ∈ 0..2^100`. That decides the name question below.

## Decision

1. **An unbounded end is omitted.** `lo..` is ↑lo and `..hi` is ↓hi; `lo..hi` is unchanged. They read
   to the core's `(int lo +inf)` and `(int -inf hi)`, which keep their spelling, so emission is
   unchanged: the core term was always the same.
2. **When an end is absent.** `..` has no right operand when the next token cannot begin one: `)`, `]`,
   `}`, `,`, `;`, `=`, `:=`, `->`, `:`, a line end or the file's end, or a word of the header's grammar,
   `where ensure pure index variadic linear inout out` (j.md §2.4), which are contextual and so must end
   a range written before them: `n ∈ 0.. inout`. `..` with no left operand is prefix, where an
   expression begins: `x ∈ ..0`. A bound that is such a word is parenthesized.
3. **At most one end is omitted.** `..` alone is refused, naming `Int`: ℤ already has its name, and a
   second spelling would differ from it only by item 5's representation request.
4. **`` `+inf` `` and `` `-inf` `` are refused** as an endpoint, naming the omitted end. The compiler's
   messages write `0..` and `..hi` (j.md §6.1), so a declaration and a diagnostic use the same spelling,
   which was unbounded-rung.md §4.2's reason for `+inf` in the first place.
5. **No `Nat` yet.** The set ↑0 is ℕ, and TLA+ names it `Nat`. But in this language today, declaring
   `0..` is a request for the rung above the word (Context), so `Nat` would name the bignum while
   reading as "a natural number": a homonym (fit-research.md's misfit). Our own documents show the
   misreading: abstract.md writes `i ∈ Nat` for an index, where `0..` would widen the index out of the
   word. **`Nat` becomes TLA+'s exactly when a declared set stops being a representation request**,
   which is ADR 0049's line (ρ chosen from proven sets) applied at a declaration: a question of its own,
   put to hamza with this ADR. **Answered 2026-10-25, yes** ([ADR 0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md)):
   `Nat` is added when that is built.

## Why not

- **A word `inf`, +∞ on the right and −∞ on the left: `0..inf`, `inf..0`** (hamza, 2026-10-25, weighed
  against this decision and rejected):
  - **`lo..hi` is closed**, {x : lo ≤ x ≤ hi}, so `0..inf` says inf is in the set, which is false of ℤ.
    Mathematics writes [0, ∞) with an open bracket for exactly this reason, and J's `..` has no open
    form. An omitted end says "no bound", which is what ↑0 is.
  - **One word, two elements, by position**: a homonym (fit-research.md). `inf..0`, read as it is
    written, is the range from +∞ down to 0, the empty set; only the position rule saves it.
  - **`-inf` would be written anyway**: every interval in the literature, and our own IR's reports
    (`[-inf, +inf]`), write the sign. `-inf..0` would be either a synonym of `inf..0` (two spellings of
    one set) or refused (a surprise).
  - **It is a word**: reserved, it takes a name from programs (`inf` the infimum); and when float sets are
    built (settypes-research §5's float intervals), F64 has an element +∞, IEEE's, so `0.0..inf` would
    name an element where `0..inf` names none, a homonym across sorts.
  - **An endpoint is an expression** (`2^62 - 1`), so `inf - 1`, `2^inf` and `-(inf)` must each be
    refused, which is what `TestInfiniteEndpointsMustPointOutward` existed for under `+inf`.
  - **Its one advantage** is visibility: a bound forgotten from `0..n` reads `0..`, a weaker claim that
    may still compile. The common case gets a name instead, `Nat` (ADR 0079), and the omitted end is
    Rust's, Python's slices' and Haskell's `[lo..]` choice.
- **Keep `` `+inf` ``.** A quoted name that is no name and an endpoint that is no integer: two misfits in
  one token, and a reader rule only for it.
- **`0..∞`, or `∞` as a symbol with an ASCII name.** It writes the element ℤ lacks, the imprecision
  unbounded-rung.md §4.2 named, and it adds a symbol to ADR 0065's set to say what an absent bound says.
- **`Nat` now** (proposed in assessment-2026-10-24 §5). Item 5. It would be the common case (all four
  corpus sites are `0..`), and the name is right for the set; it is wrong for what the declaration does.
- **A set former, `set { x ∈ Int : x >= 0 }`.** Separation (ADR 0061) is not built, and when it is, `0..`
  is that set's normal form, not a rival to it.
- **A type name (`bigint`, `exact`).** ADR 0019's option B: it names a representation, and loses the
  bound a finite range carries.
- **`..` for ℤ too, the full product {present, absent}².** Item 3: ℤ with the license and ℤ without it
  would be one set spelled twice.

## Consequences

- The four sites in `examples/big/` become `(0..)`; emission and counts are byte-identical, and the four
  refusals on windows that quote the range now say `0..`.
- spec/j.md §2.9's `range` takes an optional end on either side, and §1.6's spread `...` stays one token,
  so `xs...` and `0..` never meet. §6.1's messages print `0..`.
- `+inf` and `-inf` stay the core's words (inventory.md), spelled only in a core term and the IR's text.
- **Opened: whether a declared set above the word should be a representation request at all**, or only
  a set, with ρ chosen from proven facts as everywhere else (ADR 0034's least fixed point). `Nat` waits
  on it. **Decided 2026-10-25, a claim only**: [ADR 0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md), not built.
