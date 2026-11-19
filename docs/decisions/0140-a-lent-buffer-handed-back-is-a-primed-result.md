# 0140 — A lent buffer handed back is a primed result, `(b' ∈ Array(V) linear)`

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "a lent buffer handed back (dst' ∈ Array(V) linear) looks
better and more consistent"). Completes [ADR 0135](0135-a-cell-parameter-is-the-mode-inout-not-the-set-cell.md)
at a result. **Built 2026-11-19**: `smooth_into` and `kara/workspace` respelled, their emission
identical; `TestALentBufferHandedBackIsAPrimedResult`.

## Context

ADR 0135 made a binder `x ∈ S mode` and refused the core's formers, `cell(T)` and `buffer(V)`, at a
parameter. A program definition's result has two linear relations to the caller (inout-2026-11-19):
- `(r ∈ Array(V) linear)`, a region the definition made and moves out (ADR 0084);
- the buffer the caller lent, handed back written (ADR 0070), still spelled with the core's former,
  `(buffer(V))`, in `smooth_into` and `kara/workspace`.

A host declaration already writes the second as a primed name: `b' ∈ Array(V) linear` is the
borrow of parameter b handed back (`core/jreadtarget.go`).

## Decision

1. **A result `(b' ∈ Array(V) linear)`, with b a `linear` parameter of the same definition, is that
   borrow handed back**: D's `(buffer V)` result, the caller's buffer, written. The prime names the
   parameter whose region it is, as in a host declaration.
2. **An unprimed linear result, `(r ∈ Array(V) linear)`, stays ADR 0084's moved-out region.**
3. **`buffer(V)` at a result is refused**, naming `b' ∈ Array(V) linear`, as at a parameter.
4. A primed result naming no `linear` parameter of the definition is refused, saying which name it
   lacks.

## Why not

- **No result, the borrow ending at the call** (inout-2026-11-19's (b)): changes the two programs
  and every caller of them, where the prime changes only the spelling.
- **Keep `(buffer(V))` at a result**: the core's former in J, which ADR 0135 removed at a
  parameter.

## Consequences

- Emission-neutral: both spellings are one core term. A full check showing no emission change is
  the build's witness.
- A result's prime is the host's notation, now the program's as well.
