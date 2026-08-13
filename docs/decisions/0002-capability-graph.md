# 0002 — A capability graph replaces the fixed layer tower

Date: 2026-08-13
Status: Accepted

## Context

Three separate requirements turned out to be the same problem:

1. Exploit each ecosystem maximally ([0001](0001-parasite-model.md)).
2. Use a feature on a target that lacks it — floating point on an integer-only machine —
   by lowering it to something the target does have.
3. Use a feature on a target that implements it *natively in hardware* without lowering it
   at all: "compiling up."

The original design was a rigid tower — L3 → L2 → L1 → L0 — where everything bottoms out at a
universal core. That answers (2) and nothing else.

## Decision

Layers form a **graph, not a tower**, resolved per target.

- A **capability** is a named, typed unit of functionality: `float64`, `map`, `threads`,
  `matmul`.
- A **module** declares which capabilities it requires.
- A **target** declares which it provides natively, plus **shims** implementing capability X
  in terms of Y and Z.
- Building covers the required set from (native ∪ reachable shims); anything uncovered is an
  error naming the gap.

**Emit at the highest layer the target natively provides.** Lower only as far as necessary.
Go's `map` stops at `map`; the same source on C keeps lowering into a real hash table.

Capabilities come in two tiers:

- **Tier 1 — specified.** Portable, conformance-tested, deliberately small.
- **Tier 2 — raw bindings.** Target-specific, unspecified: names, types, import line. Near
  zero cost, bulk-generatable, no portability claim.

## Why not

**Fixed tower with everything lowering to a universal core.** Loses the ecosystem, the
performance, and the binary size. Emitting a hand-rolled hash table into Go when Go has `map`
is strictly worse on every axis.

**Idiom recognition to "compile up."** The obvious implementation of requirement (3) is
pattern-matching low-level IR to recognize a memcpy loop or re-vectorize a scalarized one.
This is notoriously brittle and compilers have fought it for decades. The capability graph
avoids it by construction: never lower the operation in the first place.

**One tier of capabilities, all specified.** Binding Go's standard library would mean
thousands of interchangeability specifications. Does not scale. The two-tier split is what
makes requirement 4 — "a file listing function names" — cost approximately nothing.

## Consequences

- Requirements (2) and (3) collapse into one mechanism: a shim's presence or absence.
- Lowering is a **search over an implementation graph**, not a fixed pipeline. This is the
  main new piece of compiler machinery, and the main source of implementation risk.
- Tier 1 specifications must be tight enough for interchangeability but loose enough that
  native implementations qualify — do not specify map iteration order, since Go deliberately
  randomizes it. Each Tier 1 capability needs a conformance suite.
- Diagnostics matter unusually much: "target X cannot provide capability Y required by module
  Z" is a primary user-facing surface, not an edge case.
