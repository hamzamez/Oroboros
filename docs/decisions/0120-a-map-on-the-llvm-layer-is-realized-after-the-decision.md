# 0120 — A map on the LLVM layer is realized after the decision

Date: 2026-11-16
Status: Accepted (hamza, 2026-11-16: "Go ahead, always mathematical and algebraic", on the proposal
to take the windows map's refactor before its capacity). Supersedes maps.md §8.4's lowering before
reduction. Built with [maprealize-2026-11-16](../../gauntlet/results/maprealize-2026-11-16.md).

## Context

Windows' host has no map, so the language supplies one: open addressing over a table, written in J
(`emit/winmap.oro`). It was lowered **before reduction** (`emit/winmap.go`):
- every `insert` became `wm_put`;
- every `len(m)` became `wm_len`, decided by a syntactic tracker of which binders hold maps.

Everything after reduction therefore saw tables, never maps, on windows. Two consequences:
- **The tracker lost a map** that the state translation hands on through a tuple, so `len(m)` was
  the backing table's length, 12 for three keys (the book's finding 71, bookfaults-2026-11-16).
- **No statement about a map can be decided on windows.** A map's capacity (maps.md §6, hamza's
  (B′)) is the claim len(m) ≤ c on its domain. On windows the refinement layer would read `len(m)`
  as the backing table's 3n cells and refuse every map program.

ADR 0089 had already fixed the order for every other operation: a realization is a term over the
host's operations, substituted **after** the decision. ADR 0117 moved the string operations so. The
map's rewrite predates both.

## Decision

1. **A map on the LLVM layer is the table of its cells.** ρ_M : Map(K, V) → Table(ℤ) maps m with
   capacity c to the table t of 3n cells, n = slots(c) = min{2ʲ ≥ 2c}: slot s holds the used flag
   t[3s], the key t[3s + 1] and the value t[3s + 2]. Its abstraction is
   d(t) = {(t[3s + 1], t[3s + 2]) : t[3s] = 1}, with open addressing and linear probing, hash
   k & (n − 1), as before.
2. **Each map operation is realized by a term, a homomorphism h on the IR** with
   d(h(op(m))) = op(d(m)) on op's domain. The terms are definitions in J on the LLVM layer
   (`targets/llvm/lang.oro`), each stated by a `realize` line:

   | operation | realized by |
   |---|---|
   | `build-map(c){m. B}` | `build(cells(c)){t. h(B)}`, cells(c) = 3·slots(c) |
   | `insert(m, k, v)` | `map_put(t, k, v)`, the table lent and written in place |
   | `read(m, k)` | `map_get(t, k)`, the option's (tag, payload): (1, 0) absent, (0, v) present |
   | `keys(m)` | `map_keys(t)`, ascending by rank |
   | `len(m)` | `map_size(t)`, the used flags counted |
   | a literal `map { k : v, … }` | `map_of(ks, vs)` |

3. **Substituted after the decision, on the IR** (`ir/maps.go`), before storage, so the table a
   `build-map` becomes is allocated and released as every table is. Each instance is decided against
   its own header and refined, as ADR 0089 and 0117 have it, and the decision is taken again at the
   site. The decision and the refinement layer see the abstract map on all four targets.
4. **`emit/winmap.go` and `emit/winmap.oro` are deleted**, the tracker with them, and `repr map
   library` with them.

## Why not

- **Decide on a second reduction with the maps kept abstract, then rewrite as before.** It gives
  the order ADR 0089 asks, but by reducing twice. It also keeps the pre-reduction rewrite and its
  tracker, the source of finding 71.
- **Give the refinement layer the windows representation's own facts** (a count cell, read over
  write). The statement about a map would then be decided about its realization. The logic never
  prints (ADR 0049): a claim is about the value, and a representation's facts are the realization's
  business.
- **A host map from a C library.** Windows' C runtime has none, and a library is a dependency the
  target does not have.

## Consequences

- A map's capacity (the next step, hamza's (B′)) is decided once, on the abstract map, the same on
  every target.
- The windows map is otherwise as before: the same table and probe, now instances inlined at their
  sites.
