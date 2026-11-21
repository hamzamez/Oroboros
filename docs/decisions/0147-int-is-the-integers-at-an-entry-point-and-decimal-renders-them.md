# 0147 — `Int` is ℤ at an entry point too, and `decimal` renders ℤ

Date: 2026-11-20
Status: Accepted (hamza, 2026-11-20: "1. yes 2. decimal, write the research note", then "go ahead build
it now"), on [integers-boundary-research.md](../integers-boundary-research.md). Amends
[ADR 0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) item 3 (an entry
point's `Int` is no longer the host's word) and
[ADR 0095](0095-text-from-ascii-is-the-inclusion-and-rendering-is-integers-at-every-sort.md) item 3 (its
name: `big_str` is `decimal`).

## Context

`Int` denoted two sets: ℤ inside a program and the host's word at an `export`, so `export fn
square(n ∈ Int) (Int) { n * n }` was refused while `cube` inside a program was accepted (the book's
§0.5). Printing was split by representation: `io.print_int` over the word, `big_str` over ℤ, and
"consent" was the name for a consumer whose set is ℤ. And `wide(n ∈ 0..2^63-1) (0..2^126)` compiled on
Go and not on JavaScript, which has `BigInt`. hamza: printing "is deciding representation, it is no
different than go have + for int and addition for bigint".

## Decision

1. **`Int` denotes ℤ at every place** (ADR 0073: a set's name denotes one set).
2. **An entry point's parameter is ρ of what its header states**, its set and its `where`'s constant
   bounds (ADR 0075): the word where they bound it inside the word, otherwise the host's integer of any
   size (`*big.Int`, `BigInt`, `BigInteger`). On JavaScript a bounded range above the word, whose
   unsigned word is a pair of digits and no host type, is a `BigInt` too.
3. **A result declared `Int` is held as its yields are**, in the word where they are: an export
   accepted before keeps its interface.
4. **A target with no integer of any size at its boundary refuses an unbounded entry `Int`**, saying
   so: windows' callers are C's, and K(ℕ\*) is the program's own (ADR 0099).
5. **The host's word is its range**, `-2^63..2^63-1`; a host declaration's word is its host module's
   `Word` (`go.Word`, `js.Word` = ±(2⁵³−1), `java.Word`, `llvm.Word`), a name denoting one set.
6. **One rendering of ℤ, `decimal(n)`**, realized by the argument's representation: the host's own
   formatting in the word (`decimal-word`: `strconv.Itoa`, `String`, `Long.toString`), its big integer's
   above it, the limbs' render on windows. **`io.print_int(n)` is a definition over ℤ**,
   `print_line(decimal(n))`, and `lib/win/print.print_int` splits at the word (ℤ = S ⊔ (ℤ ∖ S)).
   `num.decimal(x, k)` keeps its name; on ℤ at k = 0 it is `decimal`.

7. **An entry point's `where` bounds a parameter by its constants, by a table's length** (`i <
   len(t)`: a length is at most the target's largest table) **and through another parameter**
   (`i < n && n <= len(t)`, the difference bounds' closure, Fourier–Motzkin's two-variable
   fragment), so an index bounded by its header stays a word; and a `where`'s conjuncts on word
   parameters are assumed when another parameter is above the word.
8. **A table's element and a map's value are words by their storage**, an `Array(Int)`'s element
   in the signed word whatever its set, so it widens where it meets an integer of any size.

## Why not

- **`Int` the host's word at a boundary, with `..` for all of ℤ** (finding 94's fix): two spellings
  of one set with two meanings, which ADR 0073 refuses.
- **A set `Word` of the language meaning each target's word**: it would denote a different set on
  each target. A host module's `Word` denotes one.
- **An export's `Int` result always the host's big integer**: correct, and it would change the
  interface of every export that writes `Int`, 159 of 699, where ρ of the proven hull keeps the word.
- **`string_of(n)` for the rendering**: `string_of(65)` is "A".
- **`big_str` kept beside a word rendering**: a name for each representation is what the language's
  one `+` refuses.
- **Keep `io.print_int` over the word, and refuse a printed value above it** ("bounded by default"
  at the printer): it made `cube` printable through one name and not the other.
- **An intermediate above the word computed exactly wherever a word does not hold it**
  (integers-boundary §7's sentence taken whole): the word midpoint's `lo + hi` would compile, and
  so would an accumulator whose fact is unbounded only because a table's length is, as a big
  integer, silently where it is refused today. Not built; a question for hamza (the book's
  finding 108).

## Consequences

- A printed value is computed exactly at any size: the book's `no-grow`, 3 squared seven times, prints
  3¹²⁸. A value is refused where it must be held in a set it is not proven in: a declared range, a
  table's element, a host's word. An intermediate whose every consumer is bounded is refused too
  (finding 108), and a map read with a key of any size (finding 111): both correct programs, not
  built.
- Every export, test and example that meant the word states it (the gauntlet's harness `run(k ∈
  0..2^31-1)`, divmod's divisor, a host operator's operand).
- The book's findings 27 and 94 close, and windows' 20-digit stop.
