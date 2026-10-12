# 0059 — A tag is a quoted symbol; case carries no category; a collection has one former; `Array(V)` and `Map(K, V)`

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §4.8: "1. the tag is 'name, agreed. 2. agreed.
3. yes, but not sold on the use of :"). **Supersedes [ADR 0056](0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md)'s
items 8 (`array(n, f)` for a rule) and 9 (`Seq(S)` for the set).** Amends
[spec/abstract.md](../spec/abstract.md)'s Σ₀ (`Seq` becomes `Array`) and extends
[spec/data.md](../spec/data.md) §2's symbols to tags. The separator inside a former (`:` or `↦`), J's set
operators and the qualification of tags are decided separately (surface-design §4.9). Not built.

## Context

Installment 4 (surface-design §4.1–4.8) measured how the corpus writes its data
(`gauntlet/surface/data.go`): 17 constructors, written 64 times as expressions and 96 times as
patterns, 8 of the patterns nullary; 47 tables by their graph and 77 by a rule, 67 of those
zero-filled; `(array T)` as a type at 38 sites, spelled as a one-element graph.

A constructor applied, `fine(x)`, reads as a call, though Σ's names are of two kinds: constructors,
free and matchable, and defined names, which reduce. In D, `fine(x) == e` has a definition's shape; in J,
`fine(x) = e` reads as one; and a nullary constructor in a pattern, `none`, cannot be told from a binder.

## Decision

1. **A tag is written `'name`**, in patterns and in expressions: `'fine(x)`, `'none`, `'ok(src)`. It is
   data.md §2's symbol, a name used as data, so a variant's tags and a record's labels are one kind of
   thing: the index of a Σ and of a Π over a finite set of names. ADR 0042's tag is the representation
   data.md §2.4 left open for a symbol that survives staging. A pattern is built from tags, tuples,
   literals, names and `_`; a name in a pattern always binds, and a tag never does. TLA+'s prime is
   postfix (`x'`), which data.md §2.2 keeps reserved, so the prefix quote collides with nothing.
2. **Case carries no category.** Identifiers are case-sensitive, as a host's are, but the language reads
   nothing from case: sets are Capitalized by convention only, and D's reserved words are a fixed list.
3. **A collection has one former shape, `former { … }`**, its graph or its rule inside the braces:
   `array { 2, 3, 5 }`, `array { k ∈ 0..n-1 … e }`, `map { k … v, … }`, `set { … }`. **`array(n, f)` goes**,
   and with it the one-bracket hazard ADR 0056 accepted (`array(3, 0)` against `array{3, 0}`): `array(`
   is now a syntax error. An empty map buffer keeps its capacity, `map(cap)` (ADR 0058).
4. **The sets are `Array(V)` and `Map(K, V)`**, in D and J both, as names of Σ₀; abstract.md's `Seq` is
   renamed `Array`. With `array(…)` gone, a mis-cased `array(V)` is refused, not read as a value.

## Why not

- **`#name`** (surface-design §4.4): it worked, but `'` was already reserved for symbols, and a tag is
  one.
- **Capitalized tags** (OCaml, Haskell): case carries no category here, so nothing would enforce it.
- **A leading dot, a colon**: the dot is qualification and Swift resolves `.ok` by the expected type,
  which resolution (ADR 0011) does not know; D uses `:` in `LOOP … :` and in comprehensions.
- **`array(n, f)` beside the former** (ADR 0056, item 8): two shapes for one constructor, and a
  one-bracket hazard against the graph.
- **`Seq(V)`** (ADR 0056, item 9): chosen so a set and a value were never spelled alike, which `Array(V)`
  now also guarantees, and `Array` matches the former's name.

## Consequences

- A zero-filled buffer writes its binder, `array { _ ∈ 0..n-1 … 0 }`, about six tokens more than
  `array(n, 0)` at each of 67 sites.
- data.md's symbols are no longer only compile-time labels: a tag is a symbol applied to its payload,
  represented at run time as ADR 0042's integer tag. Record labels stay compile-time (data.md §2.4).
- abstract.md's `Seq(…)` becomes `Array(…)` throughout; the worked example's `xs ∈ Seq(0..1000)` reads
  `xs ∈ Array(0..1000)`.
