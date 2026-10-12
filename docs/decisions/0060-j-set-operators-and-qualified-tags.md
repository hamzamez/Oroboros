# 0060 — J's membership is `∈` and its set operations are Python's; an imported tag is qualified, `os.'not_found`

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §4.9: "2. sure. 3. but I thought os.'not_found
the tag ' is part of the symbol why does it move to the module name. no to exposing"). Not built: the
surface is built when it is chosen.

## Context

surface-design §4.8–4.9 asked how J writes the operations of the sets' Boolean algebra, and whether a
variant's tags are qualified by their module. Python's operators are the Boolean algebra's own (`|` and
`&` join and meet, `-` the relative complement, `<=` the order) and are free in J, which has no bitwise
operators. But Python's `in` would sit beside the modes `inout` and `out`, named after Ada's, in which
`in` is itself a mode: `fn Sscan(str in String, a in Value inout, …)`.

Today's T qualifies every imported constructor (`result.ok` 53 times, `result.err` 46).

## Decision

1. **J writes membership `∈`** (ASCII input `\in`, which a formatter prints as `∈`): it is the binder's
   own relation, in every parameter and claim, and it keeps its glyph.
2. **J writes the Boolean algebra's operations as Python does**: `|` (∪), `&` (∩), `-` (∖), `^` (symmetric
   difference), `<=` (⊆), `<` (⊂), and `*` (×, which Python lacks). `-`, `<=`, `<` and `*` also act on
   integers, overloaded by sort as `+` and `*` already are between `int` and `f64`. **D writes TLA+'s**,
   `∈ ∪ ∩ \ ⊆ ×`.
3. **An imported tag is qualified, `alias.'tag`**: `os.'not_found`, `result.'ok(src)`. The quote belongs to
   the symbol (data.md §2: `'` is followed directly by an identifier), and the module qualifies it as it
   qualifies any name, `os.ReadFile`. A module's own tags are written unqualified inside it, as its own
   functions are, and the language's, `'some` and `'none`, unqualified, as its owned names (`len`,
   `concat`) are.
4. **No `exposing`**: a module cannot bring another's tags into scope unqualified.

## Why not

- **Python's `in` for membership**: it reads as Ada's mode beside `inout` and `out`. Renaming the modes
  would not help: `inout` is Swift's word for exactly ADR 0048's copy-in, copy-out.
- **TLA+'s ASCII in J** (`\cup`, `\subseteq`): made for typesetting, not for reading in a source file.
- **`'os.not_found`** (surface-design §4.8): it put the module inside the symbol.
- **Tags unqualified when they come into scope with their variant** (§4.8), or **by `exposing`** (§4.9):
  both trade the provenance every other imported name shows for brevity. Type-directed resolution, as
  OCaml and Swift do, is closed: resolution precedes types (ADR 0011).

## Consequences

- `result`'s tags are written `result.'ok` and `result.'err`, as today's T writes `result.ok`.
- surface-design's installment 4 examples use `os.'not_found` and the local `'fine`, `'some`, `'none`.
