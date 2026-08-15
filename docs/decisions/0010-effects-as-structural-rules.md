# 0010 — Effects are a side condition on β, not a feature

Date: 2026-08-14
Status: Accepted

## Context

Gauntlet program 5 was the first with an effect, and [g5 §5](../derivations/g5-bindings.md)
predicted effects would break the rewriting core in the first *correctness* defect where every
earlier one had been a performance defect.

Writing the specification first found that **effects were already here**: `dict-inc` is declared
`stmt` with the form `%s[%s]++`, so it mutates a dictionary in place and returns it, and
`dict-empty` has a fresh identity. Both since word count. β had been free to copy, drop and move
them the whole time, and got away with it only because nothing could observe the difference.

## Decision

Purity is **one declared bit per primitive**, in the target file, defaulting to **impure**. β
carries one side condition: **an impure argument is never substituted; it is normalised and
let-bound at the application site, in argument order, whatever its occurrence count.**

The three clauses deny exactly the three structural rules — contraction, weakening, exchange. A
pure term lives in the cartesian fragment; an impure term is *ordered*: exactly once, where it was
written.

`seq` is reader sugar for a β-redex with an unused binder, and works **only** because weakening is
denied.

## Why not

**Effect types, monads, or an effect system.** All are a second type-level mechanism for something
one bit and a side condition already decide. Nothing here forecloses them if reads must later be
distinguished from writes.

**Linear types on values.** The substructural question had arrived four times from unrelated
directions ([g5 §9](../derivations/g5-bindings.md)) and the obvious reading was that values need
multiplicities. They do not. What needs the discipline is *effects*, and the fix is to make the
structural rules **conditional on purity**, which costs no type system at all.

**Default `pure`, with an `effect` marker.** Rejected on the asymmetry of failure: a third-party
target author who forgets `pure` gets a slower program; one who forgot `effect` gets a **silent
miscompilation**. *The default must be the one whose failure mode is slow, not wrong.*

**g5's precise rule** — compare each metavariable's execution-context depth on both sides. It is
correct and delicate. The conservative rule (never substitute an impure term, even at one
occurrence) is four lines, and
[measured](../../gauntlet/results/effects-2026-08-14.md) to produce **byte-identical machine code**
on the one program it changed.

**A unit type, and a sequencing construct.** Neither is needed: `stmt` already specifies that a
term's value is argument 0, and `seq` desugars to an application.

## Consequences

Six pure programs kept byte-identical output, so the discipline is free where it is not needed.
Program 5 emits correctly on all three targets and `dot` still fuses through the effect.

**Not fixed, deliberately:** aliasing. Ordering is guaranteed; `dict-inc`'s destructive update is
still unchecked, and stays unobservable only because no primitive reads a dictionary. That is
[g7](../derivations/g7-aliasing.md)'s question and this decision does not answer it.

The purity default was chosen for **hand-written** target files and is unlikely to survive
machine-generated ones, where every name would default impure and fusion would die everywhere. No
host publishes purity as metadata. Recorded in [types-direction §3.6](../types-direction.md).
