# 0003 — Range-typed integers with mathematical semantics

Date: 2026-08-13
Status: Accepted

## Context

Should the type system mirror **machine types** (`int32`, `float64`) or **mathematical types**
(unbounded integers, exact decimals) with the compiler choosing representation?

The requirement is no performance compromise, on targets that disagree profoundly about
numbers: the JVM has no unsigned types, and JavaScript has no integers at all.

## Decision

**Mathematical semantics, machine representation, range declared in the type.**

The type carries a range — `(int 0 255)`, `(int -2^31 2^31-1)` — and the compiler selects the
representation that fits. Semantics are exact: no wrapping, no undefined behavior. Overflow is
a compile-time error where provable, a trap otherwise. Wrapping operations exist under
separate names.

`i32`, `u8`, `nat` and friends are sugar for common ranges. Ranges are inferred for locals;
explicit types are required at function boundaries.

Floats are IEEE-754 binary32/binary64 with strict semantics by default, fast-math opt-in.
Exact decimals are a library type. Unbounded integers remain available as a Tier 1 capability
in a package — not as the default.

Precedent: Ada/SPARK subrange types; Zig's `u7`/`i33` with `+%`.

## Why not

**Unbounded integers as the default.** This repeats Shen exactly. They cannot be unboxed in
general — every operation becomes a branch plus a possible heap allocation, or it depends on
whole-program range analysis, which is fragile and makes performance *unpredictable*.
Unpredictable performance is itself a compromise on performance, and a bignum implementation
in every binary violates the footprint requirement.

**Machine types as the default.** Breaks portability at the first target: `u32` has no JVM
representation and `int64` has no JS representation. Also inherits C's undefined overflow.

## Consequences

- Representation is a machine word chosen from a declared range — zero cost.
- The range is the portability contract. `(int 0 255)` becomes `uint8_t` in C and a plain
  `int` on the JVM; the program means the same thing in both.
- `(int 0 2^31)` maps exactly onto a JS double, since doubles represent integers exactly up to
  2^53. An untyped `int64` on JS would be a catastrophe. The type decision and the target
  decision ([0004](0004-first-targets.md)) reinforce each other.
- Requires range analysis in the compiler from early on. This is the main implementation cost,
  and it is simpler than the whole-program analysis the unbounded-default would have needed.
- Tier 2 bindings gain precision: a Go `int` parameter is declared as its actual range.
