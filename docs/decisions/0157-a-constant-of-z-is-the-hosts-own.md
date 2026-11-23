# 0157 — A constant of ℤ is the host's own

Date: 2026-11-22
Status: Accepted (hamza, 2026-11-22: "yes, go with a for windows and the hosts. always mathematical and
algebraic", on the measurement of a constant rebuilt in a loop). Extends
[ADR 0156](0156-an-integer-constant-is-an-element-of-z.md) (a constant is an element of ℤ) by
[ADR 0089](0089-a-realization-is-a-term-over-the-hosts-operations.md) (a realization is a term over the
host's operations) and the founding rule (emit at the highest layer the target provides).
**Built 2026-11-23** ([hostconst](../../gauntlet/results/hostconst-2026-11-23.md)).

## Context

A constant held exactly was realized by its spine: Horner over 10¹⁵, `big-of` of each digit with
`big*` and `big+`, computed where it is read. In a loop it was rebuilt at every iteration: three
allocations, a product and a sum to make a value that never changes, beside the one product the
program asked for. Measured over 100,000 iterations of `acc + i · 2¹⁰⁰`: 2.4× the constant made once
on Go (7.8–9.0 ms against 3.3–4.6), 3.5× on windows (115 ms against 33, 2,000,000 iterations,
K(ℕ\*)'s numeral rebuilt each time, the function 1,701 lines of LLVM IR).

## Decision

1. **A constant is a nullary operation of ℤ**, and its realization on a target is the closed term
   over the host's signature at the highest layer the host provides: Go's `*big.Int`, a package-level
   variable set once from its decimal (`oroBigN`); the JVM's `private static final BigInteger`; a
   BigInt literal, `cn`. The spine denotes c too, and lowers further than the host requires.
2. **On K(ℕ\*) a constant's numeral is read-only data**: K(ℕ\*)'s table (ADR 0099), digits in base
   2^dbits then the header 2m + s, a global `{ i64 len, [len x i64] }` laid out as a table is. It
   exists before the program runs, so no birth makes it and no death releases it: release is the
   identity on it (ir/release.go owns births only), and no region is reused from it.
3. **The frame rule bounds it**: a program function's result is a birth its caller owns and releases
   (ADR 0101), so a numeral that leaves its function, by a yield, a join or a store, is fresh storage,
   its spine. A numeral is static where every use borrows it, the rung's operations, a comparison,
   `decimal`, or where nothing reads it. On Go, whose `*big.Int` is mutable, a constant a terminator
   moves (a yield, a `continue`, a `break`) is a copy, `new(big.Int).Set(oroBigN)`, and an operation
   borrows the shared constant; the JVM's `BigInteger` and a BigInt are immutable, shared freely.
4. **A closed term held exactly is its constant** where the representation gives a constant a form
   of its own, the host's bignum and K(ℕ\*): an operation whose fact is one integer is that constant,
   not an operation computed where it is read. A point of U on a target with an unsigned word or a
   pair is that word's constant (ADR 0033, 0151), and a point past the decision's 192-bit ends (ADR
   0105) is no point the facts state, so stays an operation.
5. **On fixed limbs**, whose layout is the instance's own (the program's one width), a constant is its
   spine and a closed term its own structure, which shares its operands: one constant per closed term
   was one spine per term and per chunk of its numeral.

## Why not

- **(b) The spine once at the function's entry**: rebuilt at every call, an allocation and ~41 ns
  per call to a small function in a hot loop, and its code kept; a restricted hoist where the
  constant's own storage is the image's.
- **(c) The spine, until a program measures it**: measured, 2.4× and 3.5×.
- **Hoisting or CSE on the IR**: deliberately not next (CLAUDE.md), and it would leave the work the
  host need not do at all.
- **A static numeral everywhere**: a caller releases a program function's result, and would free
  read-only data (`TestAStaticNumeralIsNeverReleased`'s planted fault).

## Consequences

- The IR's text writes a constant past `int64` as its decimal, which the reader reads back as the
  integer; the IR's interpreter, IR_P's element classes and the interval analysis read it.
- Found on the way, a wrong answer older than this decision: on JavaScript a literal table
  `[9223372036854775807, 9223372036854775806]` was emitted as Numbers and read 2⁶³ twice. A table's
  element is a word by storage, the Number there, so a constant past it in a table is refused, as a
  computed one is (ADR 0156 item 5's sink).
- A constant past the word costs nothing at run time where it is read: windows' loop 111 → 32 ms,
  its function 1,701 → 665 lines; Go, the JVM and JavaScript a field or a literal.
- Two file writers (`emit.File`, `JavaFile` and `WriteProgram`) declare the constants, from one
  registry each (`GoConstDecls`, `JavaConstDecls`); LLVM's module writes its numerals beside its
  strings.
