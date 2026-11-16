# 0121 — A map's capacity is a claim on its `len`

Date: 2026-11-16
Status: Accepted (hamza, 2026-11-16: "Go ahead with B′, always mathematical and algebraic", of the
four options A, B, B′ and C shown in the surface). Built with
[capacity-2026-11-16](../../gauntlet/results/capacity-2026-11-16.md), on ADR 0120.

## Context

maps.md §6 decided that a map has a declared capacity, so that all four targets agree, and that
"inserting beyond it is a condition the program must handle". No target enforced it:
- Go, JavaScript and the JVM grew the map;
- windows dropped a key once its 2c slots were full.

A differential case had inserted nine keys into a `map(8)` since it was written.

## Decision

1. **A map of capacity c is in Map_c = {m ∈ Map(K, V) : len(m) ≤ c}**, with len(m) = |dom m|.
   `map(c)` is in it, and so is any map a binder declares in a separation,
   `set { m ∈ Map(K, V) : len(m) <= c }` (ADR 0061). So the capacity is a claim like any declared
   set (ADR 0079), and the surface gains no new parameter.
2. **An insert is partial on Map_c.** m[k ↦ v] ∈ Map_c iff k ∈ dom m or |dom m| < c, and its
   condition is decided as a store's domain is (ADR 0119). The state translation S marks each
   insert into a map place with its binder's claim, ADR 0028's mark
   `(#reqw "a map's capacity" (len(m) < c) (insert m k v))`. The condition len(m) < c is
   sufficient; presence is a fact the refinement layer does not have (Why not).
3. **The capacity is bound once, where the program wrote it.** D writes
   `(fn (#cap-m) (build-map #cap-m (fn (m) (#mapplace m (fn (m) body) #cap-m)))) c`. The binder is
   apart along the scope chain, since a `var` may not be shadowed in its own scope. A term copied
   into the marks could be captured by an inner binder.
4. **The laws that decide it**, each with a planted fault:
   - **a fresh map is empty**: |dom m| = 0 at `build-map`'s binder (`PlantMapNotEmpty`);
   - **an insert adds at most one key**: dom(m[k ↦ v]) = dom m ∪ {k}, so
     len(m) ≤ len(m′) ≤ len(m) + 1 (`PlantInsertUnbounded`). It is instantiated on every insert
     term present, a local extension's ground instance, and is `insert`'s postcondition at a binder.
     It is not a fact of lang's, because `insert` inside `len` is not flat (F-B, condition 2);
   - **a loop that inserts once a step**: Houdini's template |dom t| ≤ v for a loop variable t that
     threads a map (`PlantNoMapTemplate`);
   - **a length is an atom of its subject**: the split does not descend into `len`'s argument
     (`PlantSplitInsideLen`), except where the subject is a conditional, len(if c a b) =
     if c len(a) len(b).
5. **A mark's condition is logic.** Linearity and the length of a term read a mark as its body: the
   condition is decided and never run (ADR 0049). The theorem that S is the state-passing
   translation (storeorder-research §2.3) holds after erasure ε of the marks (ADR 0050).

## Why not

- **(A) The capacity tracked from `map(c)` with no syntax.** A helper that inserts could not state
  its capacity, and the tracking is the binder-tracking class that produced finding 71.
- **(B) A third type parameter, `Map(K, V, c)`.** It says what B′ says with a new parameter through
  the checker, the IR's types and four printers. B′ uses ADR 0061's separation and ADR 0079's claim.
- **(C) Enforce at run time**, giving up on a full map on all four targets. That moves the
  program's condition to a crash, and ADR 0119 rejected the same for stores.
- **The exact domain, k ∈ dom m ∨ |dom m| < c**, rather than its sufficient half. It needs a
  presence fact, the `'some` arm of a read of k, which the refinement layer does not carry. Until it
  does, a program that fills a map exactly to its capacity and then updates existing keys is
  refused. No program in the corpus does.
- **A count cell on windows for an O(1) `len`**, so that a program can guard an insert cheaply.
  No program guards one: every insert in the corpus is proven.

## Consequences

- `map-dynamic`, which inserted nine keys into a `map(8)`, now declares 16 and states its input,
  `run(n ∈ 0..16)`.
- Windows' "full: keep the map" arm in `map_put` is unreachable in every accepted program.
