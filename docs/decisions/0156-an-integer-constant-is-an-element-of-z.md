# 0156 — An integer constant is an element of ℤ

Date: 2026-11-22
Status: Accepted (hamza, 2026-11-22: "since Integers are Z, why not fold them at compiler time? … what
does the algebra say", on chapter 1 §1.3 and the book's finding 120). Amends
[ADR 0009](0009-staging-preserves-results.md) (integers folded only inside the word) by
[ADR 0114](0114-a-fold-is-an-equation-between-pure-terms.md), and
[ADR 0153](0153-the-double-word-holds-a-loops-variable.md) (a yield's recombination). **Built
2026-11-22** ([intconst](../../gauntlet/results/intconst-2026-11-22.md)).

## Context

Chapter 1 shows `2^62 * 4` left unreduced, "computing it here would give a different integer from the
one the target would". That was ADR 0009's reason when an `Int` was the machine's word (ADR 0012,
0019). Since ADR 0026, 0147 and 0150 an `Int` is ℤ, a set is a claim and ρ alone chooses its
representation: every target computes 2⁶⁴ there, `io.print_int(2^62 * 4)` printing
18446744073709551616 on all four. What held the fold was the reducer's integer term, an `int64`: the
compiler's carrier, a representation, taken for the set. Measuring it found the book's finding 120:
an export returning a constant past the word as `Int`, `18446744073709551616` itself, was refused on
Go, the JVM and windows.

## Decision

1. **An integer literal denotes an element of ℤ**, the initial ring, and its constants are its
   nullary operations. A fold of the language's `+`, `−`, `·`, `/` and `%` on closed integers is an
   equation of ℤ (ADR 0114) and holds at every size, so the reducer folds them exactly, the quotient
   truncated and the remainder of the dividend's sign as before; the comparisons already were.
   Staging changes no answer (ADR 0009) because the two sides denote one integer.
2. **A division by zero is not folded**: it has no value, and its precondition is the decision's,
   which refuses the program (integers.md §5); the reducer's residual shows the term, and the program
   does not run.
3. **A constant past `int64` is written as its numeral in normal form**: the base-10¹⁵ term the
   reader writes for a literal (bigLiteral), each of its applications folded where the word holds
   it, as reduction always left a literal (`Numeral`), so each of its subterms is again in normal
   form and reducing it changes nothing; J shows it as the decimal. No new term kind: a reader of
   `KInt` reads an `int64`, as before.
4. **A point is its constant.** A value whose proven fact is one integer is computed by nothing, so
   no representation recombines it (ADR 0153's recombination reads back a value E computes): the rung
   holds the closed term exactly as it is.
5. **A constant outside S is supply** (integers.md §1a, which said so and was not built for a
   constant): held exactly where the target's signed word does not hold it, demanded by D_A
   whatever reads it, written as its spine; a sink refuses it, since no word holds it. Only a word
   narrower than `int64` meets one, JavaScript's ±(2⁵³ − 1): there `io.print_int(4611686018427387904)`
   and `console.log` of it printed 4611686018427388000 at HEAD, a wrong answer older than this
   decision, which item 1 made reachable by arithmetic (`quotient-corner`'s 2⁶² folded).

## Why not

- **Fold only inside the word** (ADR 0009 as written): its premise, that run time computes a
  different integer, is false since ADR 0026 and 0150; the restriction kept the compiler's carrier
  in the language's meaning.
- **A term kind of its own for an integer of any size** (or a big field on `KInt`): every reader of
  `KInt`, about 170 of them and 500 reads of `.Int`, would have to know it or read a big value as an
  `int64` silently; the numeral is a term every reader already handles.
- **A target's own big constant at once** (`123n`, `new BigInteger("…")`, a numeral in read-only data
  on windows) in place of the spine of the rung: a representation of the constant, cheaper at run
  time, owed with its trigger (the BigInt literal's spelling is on the list); this decision is what a
  constant *is*.

## Consequences

- `2^62 * 4` is `18446744073709551616` in every residual, and chapter 1 says why.
- A constant past the word returned as `Int` from an export compiles on every target.
- On JavaScript a constant between its word and the unsigned word is a `BigInt`, where the pair once
  took it at an operation; a pair's comparison with such a constant is now the BigInt's.
- A constant flowing into a sink (a table's word, a host's word) is refused there as its value is, at
  compile time, where its operation was refused or trapped before; under `-checked` such a store of a
  known constant is now a refusal, not a trap at run time.
