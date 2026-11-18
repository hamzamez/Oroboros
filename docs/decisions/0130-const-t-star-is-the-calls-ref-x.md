# 0130 — const T* is the call's ref(x), not a mode of the header

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18: "go ahead with ref(x)"). Reverses
[ADR 0129](0129-a-struct-parameter-is-its-prototype.md) item 2, the mode `in`; ADR 0129's items 1
and 3, the bare set as the value and its passing by the triple's ABI, stand. Extends
[ADR 0118](0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md)'s argument terms.
Built ([ref-2026-11-18](../../gauntlet/results/ref-2026-11-18.md)).

## Context

ADR 0129 marked `_In_`'s `const T*` with a mode after the set, `rc ∈ windef.RECT in`. hamza asked
why the language needed `in`, and what it had failed to express.

It had failed to express nothing. `rc ∈ RECT` passed by address and `pt ∈ POINT` passed by value
mean the same thing: a record value passed, which the host reads and keeps nothing of (ADR 0084's
P2, Theorem P). Their difference is how C receives the argument, the copy's address or its bytes.
That is representation, and a set has no representation (ADR 0049). The header's other modes pass
a test that `in` fails: each changes what a program observes.
- `linear` is a uniqueness claim the program must keep.
- `inout` is a value the host writes back, which the program reads after the call.

Remove `in` and flip the passing, and every program means the same; only the machine code changes.

ADR 0118 already holds the place where how a C argument is formed is stated: the call is a linear
term over the declaration's parameters, written in the host clause, `call "F"(t₁, …, tₖ)`, with
terms `x`, `elem(b, lo)` and `null`.

## Decision

1. **`ref(x)` is a C argument term**: &x of a fresh copy of x's encoding, `const T*`, defined where
   x's set is a C struct's (a scalar the host writes is a cell, `cell(T)`). It counts as x's one
   occurrence in the linear term.
2. **A struct parameter's set is its bare record set** wherever C receives it. The header states
   the meaning, a value passed, and the host clause states the representation:
   `call "PtInRect"(ref(rc), pt)` is `&rc` and `pt` by value, as the prototype reads.
3. **A struct parameter not under `ref` is passed by value**, by ADR 0129's item 3. With no argument
   list, ADR 0094's default reads each parameter by its C type, and a bare struct goes by value.
4. **The mode `in` is withdrawn** from both readers, j.md and the inventory.

## Why not

- **The mode `in`** (ADR 0129 item 2): a representation fact in the part of the declaration that
  states meaning, a mode no program can observe.
- **The bare set as `const T*`, a mark for the value** (step 2b's reading, ADR 0129's other
  option): the same fault the other way round.
- **A pointer set, `rc ∈ ptr(RECT)`**: its values would be addresses, while the program passes a
  record. The address is the call's, made from the copy, and never the program's (ADR 0084: an
  address is in the logic, never a value the program holds).
- **Reading the SDK's prototype at compile time**: the compiler does not read the SDK's headers. The
  generator and `TestWin32DeclarationsAreTheirPrototypes` do, and they check the declaration against
  the prototype, which ref(x) makes the same shape as C's `&x`.

## Consequences

- `RegisterClassA(wc ∈ WNDCLASSA)` with `call "RegisterClassA"(ref(wc))`, and
  `PtInRect(rc ∈ windef.RECT, pt ∈ windef.POINT)` with `call "PtInRect"(ref(rc), pt)`.
- `ref` now has two senses, both "a reference to a T": a boxed representation,
  `(repr (ref T) …)`, and this argument term. They are read in different positions.
- The by-value decision moves from the type's unfold to `cPrototypes`, after the argument terms are
  resolved: `Prim.ByValue` is every struct parameter not under `ref`. Its planted fault is
  `PlantRefByValue`.
