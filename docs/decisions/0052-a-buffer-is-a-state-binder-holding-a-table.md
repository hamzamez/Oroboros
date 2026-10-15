# 0052 — A buffer is a state binder holding a table, written in program order

Date: 2026-10-11
Status: Accepted (hamza, 2026-10-11: "I agree to the recommendations"), on
[storeorder-research.md](../storeorder-research.md) §6, all four. **Not built**: it is built with the
surface, behind the byte-identical gate. Amends [ADR 0018](0018-immutable-values-linear-buffers.md)'s
check (the guarantee stands), [ADR 0047](0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md)'s
initializer rule and [spec/abstract.md](../spec/abstract.md) §2.2 and §2.4, under
[ADR 0050](0050-the-abstract-syntax-fits-the-semantics.md). Supersedes surface-design.md §2.3's
option (a), `linear` written at 26 binders. **Extended to maps by
[ADR 0058](0058-a-map-buffer-is-a-state-binder-holding-a-map.md)** (not built).

**Built 2026-10-15** behind the `ORO_VIA_J` gate ([storeorder-2026-10-15](../../gauntlet/results/storeorder-2026-10-15.md),
spec/local.md §2.1): J writes every buffer in program order, and S of it is today's residual on the
corpus. Building it measured two errors here: the 4 renames in alloc-copies are no refusals (each renames
a dead version, which is the place), and R0 as written refuses 3 programs it should not, the merge
sorts' two buffers swapping places each pass. Three gaps follow, G5–G7 in spec/j.md; G5 and G6 decided
by hamza (2026-10-15), G7 open: a concurrent assignment `a, b := b, a` for the swap (R0's moves, a permutation of
places), the record store `dt[s][1] := v`, and one borrow rule, under which growth is the statement
`strconv.AppendInt(b, k, 10)` rather than this ADR's `b := f(b, …)`.

## Context

Today a buffer is a value threaded linearly. A store `(set b i v)` returns the buffer, and loop
variables, back edges and host calls' tuples carry it by hand. Linearity is checked by an ordering
automaton over evaluation order, (r | ℓ)* (m ℓ*)?, in `emit/linearity.go`.

surface-design.md §2.4 wrote stores as `c[j] := TRUE`, in program order. storeorder-research.md
researched whether that is one semantics with today's, and measured what it changes:
- the state-passing translation S (local.md §3) maps a program-order program to a threaded one;
- its image is the store-single-threaded programs (Schmidt 1985), exactly what today's check accepts;
- the corpus has 55 loop variables and 51 back-edge arguments that exist only to carry a buffer, and
  27 stores nested in stores;
- 4 sites rename a buffer version, all in `gauntlet/differential/cases/alloc-copies.oro`;
- 30 Go declarations hand a buffer back: 12 write-borrows whose template wraps the call to return it,
  and 18 growth calls (`Append*`).

## Decision

1. **A buffer is written in program order** (option B). `b[i] := v` writes an element, `b := e`
   replaces the whole value, and `b[i]` reads. The meaning is S of the program: S turns each
   element write into today's `(set b i v)` threaded to the buffer's next version, so the residual
   is today's.
2. **A buffer is a state binder whose value is a table.** There is no fourth binder kind.
   - `t == table(n, 0)` is definitional, an immutable table.
   - `b := table(n, 0)` is state, a buffer.
   - `x := 0` is state, a cell, as ADR 0047 has it.

   The operator decides definitional against state. Among state binders, an element write is legal
   only on a **fresh** value (R0 below), which is ADR 0047's initializer rule kept as a legality
   condition, not as the decision of the kind. `linear` is written only on a parameter, a definition's
   or a host's, where it means R2's borrow.
3. **A borrowed buffer parameter is passed by reference** (R2), not copied in and out as ADR 0048's
   cells are. Uniqueness is assumed at an export, as ADR 0020 has it, and linearity through the body
   follows from R0–R4.
4. **The wrapper on host write-borrows stays.** The template keeps handing the buffer back, so
   emission is unchanged. Removing it needs the IR to know that a write-borrow returns the same
   storage. That is a separate step with its own measurement.

**The rules that replace the ordering check.** R1–R4 are Reynolds' syntactic control of interference
(POPL 1978) applied to buffers, storeorder-research §2.4. R0 is the inbound half of uniqueness, which
the research left implicit:

| | rule | forbids |
|---|---|---|
| **R0** | **a buffer's value is fresh.** A state binder's initializer and each whole write `b := e` is a table former (`table`, `array`, a comprehension) or a call that takes `b` at a `linear` parameter and returns a buffer (growth, write-borrow). An element write is legal only on a binder that satisfies R0 | `x := xs ; x[0] := 1`, which would write into the immutable `xs`. The copy is explicit: `x := table(len(xs), xs)` |
| **R1** | **a buffer is named only by its binder** | `d == c ; d[j] := 1`, an alias |
| **R2** | **a call borrows a buffer for its duration**, and no call is handed one buffer twice | `f(c, c)` |
| **R3** | **no closure over a buffer survives its scope** | a write after the region ends |
| **R4** | **the scope's value freezes the buffer** (ADR 0031) | a store into a frozen table |

R0 and R1 together are uniqueness: nothing else names the table when the buffer gets it (R0), and
nothing else names it while the buffer holds it (R1). That is the precondition of in-place update in
the aggregate update problem (Hudak and Bloss 1985), and Clean's uniqueness at creation (Barendsen and
Smetsers 1993). Today R0 is ADR 0047's initializer rule, so it refuses nothing the corpus writes:
the 71 buffer binders all have a table former as their initializer.

## Why not

- **A, keep threading.** It keeps a checked invariant, read-after-move, as an error a program can
  commit. Program order makes it unwritable (fit-research's F5), and it removes 55 loop variables
  and 51 back-edge arguments that the semantics does not need.
- **C, both spellings.** Two spellings of one operation, a synonym (fit-research's F1).
- **`linear` written at 26 binders** (surface-design §2.3, option (a)). With program order, `:=`
  already says the binder may be written, so the marker would say it twice.
- **A buffer as a definitional binder with a mode.** That keeps a binder kind that only exists in
  practice. ADR 0047's "a cell is a buffer of one" read in reverse says a buffer is a cell written
  element by element: State(Seq(A)) with point update.
- **By value-result for a borrowed buffer**, as cells are (ADR 0048). Copying a table in and out is
  the hidden allocation CLAUDE.md forbids. By reference is sound exactly when the callee's parameter
  is the only path to the place, which is R0–R2. Ada's RM 6.2 states the same contract for composite
  `in out` parameters, and Rust enforces it with `&mut`.
- **Remove the write-borrow wrapper in the same decision.** notation-research §4.5 and
  surface-design §2.12 said it goes away. It does not follow from the surface: the translation
  rebinds the buffer to the returned version, so the residual still needs one. That is an IR
  question with its own measurement.
- **R0 by type rather than by form**: a call returning a fresh table, `mk(n)`, would satisfy a
  type-and-uniqueness rule after inlining. On the tree it is not visible. The form rule reproduces
  today's behaviour exactly, and a later rule may widen it when a program asks.

## Consequences

- **What it refuses**: the 4 version renames in `alloc-copies.oro`, rewritten with an explicit
  copy. Nothing else in the corpus.
- **What it removes from the page**: 55 loop variables, 51 back-edge arguments, 27 nested stores, and
  the threading of 5 host-call tuples (`n := f(b, …)`, with the buffer not rebound).
- **What it should not change**: the residual, so emission, proofs and the IR. That is a prediction,
  as cells' was before `TestACellReducesToTheLoopVariableItMeans` verified it. The gate is
  byte-identical emission.
- **Two definitions that thread a buffer** (`mac-into`, `smooth-into`, both exports) take it at a
  `linear` parameter and write in place.
- **Today's notation** keeps `(set b i v)` until the surface replaces it. Mapping it into the new
  signature is the converse half of the theorem, unthreading: replace each store threaded to the
  buffer's next version by an element write. The translator the surface needs (notation-research §8)
  does this once, and refuses the 4 renames.
- **Built when the surface is**, in this order:
  1. abstract.md's Σ gains the element write, `assign(x[i]; e)`. Σ₀'s `store` leaves the source
     signature and stays in Λ⁺ as D's image;
  2. local.md's translation gains the element write;
  3. R0–R4 are checked on the trees, each with a planted fault;
  4. `emit/linearity.go` (493 lines) is reduced to what R0–R4 do not cover.
