# 0119 — A store owes its table's domain

Date: 2026-11-16
Status: Accepted (hamza, 2026-11-16: "do what you recommend", option 1 of three). Built with
[store-2026-11-16](../../gauntlet/results/store-2026-11-16.md).

## Context

A table is a function on [0, len) (tables.md §2). A read t(i) is defined only on that domain, so it
owes 0 ≤ i < len(t), and an application not known to be defined is refused (tables.md §6,
refinements.md §3a).

A store owed nothing. `b[n] := 1` one past the end, `b[i] := 1` with `i ∈ Int`, and the off-by-one
loop `i <= n` each compiled on Go, JavaScript and windows (the book's finding 70):
- Go panicked at run time;
- JavaScript's array grew;
- windows wrote past the region.

tables.md §6 had said the obligation was needed "in three places", `build` among them, and nothing
enforced it. Clamps removed as "stale" from freq's writer (claimsdemand-2026-10-13) had been stale
only because a store owed nothing: "not refused" read as "proven".

## Decision

1. **A store is the update of a function at a point of its domain.** For f : [0, n) → V, set(f, i, x)
   = f[i ↦ x] is a function on [0, n) only for i ∈ [0, n). So a store owes 0 ≤ i < len(c), the
   obligation a read owes, decided by the same procedure (`indexObligation`). It is refused where
   not proven, saying *a store, defined only on its table's domain*. Planted fault:
   `PlantStoreUnchecked`.
2. **Lang's guarded facts are instantiated where an obligation is.** A fact G(x̄) → C(x̄) was
   instantiated once, at the root, and only where its guard held there. So div_floor's instance at
   (hi − lo) / 2, whose guard 0 ≤ hi − lo holds inside the loop's guard, was never made. σ(G → C)
   holds everywhere, so assuming C where G is entailed is sound; a clone keeps it in that scope.
   - It is done only after a goal fails without it, and at the top of a split, not at each level,
     which keeps `pe`'s compile at its baseline.
   - Unguarded facts need nothing: the root's walk visits every subterm.
   - Planted fault: `PlantNoLocalInstances`. It proves the book's binary-search midpoint
     (finding 67) and windows' `argv`.
3. **A square is an atom, and F12 is its law**: 0 ≤ x·x and x ≤ x·x over ℤ. The sieve's crossing
   loop starts at i·i. Planted fault: `PlantSquareOpaque`. Each half has a deletion test.
4. **Where the bound lies outside the fragment, the program guards the store** (hamza's option 1).
   The guard is a missing proof, said so where it stands:
   - jsonfmt's and freq's writers: count, then build, so op < len(out) relates two loops;
   - K(ℕ\*)'s product row and its size store: a numeral's len ≥ 1, and `size`'s hull at a join;
   - windows' map keys: a rank below a count of occupied slots.

## Why not

- **Trap an unproven store at run time** (option 3), counted as debt like an unproven loop.
  Programs would keep building, but a read and a store would follow two rules for one domain
  condition, and the algebra gives them one.
- **Leave stores unchecked.** That is the hole: windows wrote past the region.
- **Build the proofs first** (option 2), the builder and the numeral's carrier. That is the
  derivation's answer and owed (below), but it is a design, and the hole is closed now.
- **Give up in the guard's arm (`abandon`)** rather than skip the store. That is what a missing
  proof should do, but it does not compile yet: an arm into ∅ in a borrowed buffer's code freezes the
  buffer, or makes a join point a loop cannot leave. ADR 0090 says such an arm leaves the threading
  as it is (k ∘ h = h), and the state translation does not use that. That is a defect, owed.
- **A new Houdini template for each constant a proof needs** (print_int's 1 ≤ top). The program was
  restated so that its invariant is one the templates give: the sign's slot p = top − 1, with
  0 ≤ p ≤ 19.

## Consequences

**Owed, each guard's proof:**
- **count-then-build is a builder.** With the output a fold of pieces in the free monoid,
  outlen = len ∘ render, since len : (B\*, ·) → (ℕ, +) is a homomorphism, and every store lies
  below the whole by construction. `cat` (ADR 0117) is the operation, and a fold that writes in
  place is the missing piece;
- **K(ℕ\*)'s carrier as a set**, {t : len t ≥ 1}, and the join keeping `size`'s hull;
- **the arm into ∅ in the state translation.**
