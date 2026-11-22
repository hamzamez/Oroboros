# 0153 — The double word holds a loop's variable, and a yield into ℤ is its digits' recombination

Date: 2026-11-22
Status: Accepted (hamza, 2026-11-22: "1A then 2A, go ahead always mathematical and algebraic", on
2A: "E for a loop variable"). Amends [ADR 0103](0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md)
(an intermediate never a loop's variable) and [ADR 0149](0149-the-double-word-is-every-hosts.md) (the
double word on every 64-bit host, its intermediates' printing).

## Context

ADR 0103 computes a term whose intermediates the decision proves in E = (−2¹²⁶, 2¹²⁶) in the double
word, "never a table's element, a call's argument, a loop's variable, a π or a result". So a sum the
decision bounds inside E but past both words, carried by a loop, was ℤ's: `upto`'s s ≤ 2⁸⁰, the sum of
i below n ≤ 2⁴⁰, a `*big.Int` each step on Go, where the research note measured an accumulator in ℤ
at 25× a word (sets-are-claims-research §5). ρ(v) is the cheapest representation whose domain holds
v's fact and whose operations cover its uses (ADR 0150, 0151): the exclusion of a join was the
selection's, not the algebra's.

## Decision

1. **A join of the double word.** The residue map r : ℤ → ℤ/2¹²⁸ is injective on E and commutes with
   a join's choice: r(if c then a else b) = if c then r(a) else r(b). So a loop's variable, whose
   value is its initial argument's or an `again`'s, or a loop's result, a break's, is carried as its
   residue where every input is a word or held in E. E's selection (SelectDouble) is the greatest
   fixed point over operations and these joins: a candidate is dropped while one of its inputs is
   neither a word nor a candidate, or one of its uses is not E's (an exit into a join E holds being
   one).
2. **The text says which.** A join has no defining statement, so it is marked by its type: an `int`
   whose range lies outside both 64-bit words is no word's, and ℤ's type is `big`, so it is the
   double word's (`Doubles`, `pastBothWords`). The IR's text is the printers' whole input (ADR 0038).
3. **The verifier's W14** lets a double-word value through an exit, or a loop's argument, only into a
   join the double word holds.
4. **The printers**: an i128 slot on LLVM, a word stored into it extended by its sign; two digits on Go
   (`uint64`) and the JVM (`long`), set from the initial value's halves, both assigned at an `again`
   and a break, never a post-clause step; both digits kept, the flows of DoubleHighs and DoubleLows
   seeded at every join.
5. **A yield into ℤ of a value E can hold is its digits' recombination.** By Euclid's identity in
   mixed radix 2³², for every integer v,

       v = ⌊v/2⁶⁴⌋·2⁶⁴ + (⌊v/2³²⌋ mod 2³²)·2³² + (v mod 2³²),

   each digit a read-back of the double word by a shift or a `mod` of a constant, so v's uses are E's
   and the loop keeps it there; the recombination's operations past the word are the rung's, at the
   exit (RecombineYields). Applied where a clone's selection then holds v in E. Three digits, not
   two: 2⁶⁴ is no literal, and a digit in U does not widen into the rung on every host.

## Why not

- **ℤ, as before** (option B): exact and 1.7× slower on a whole program measured here, 25× on the
  accumulator alone.
- **A counter as a checked word** (option C): a clamp justified by time, not proof (ADR 0145, no
  axioms), and what `-checked` already gives where an author asks.
- **Two digits at the exit**, ⌊v/2⁶⁴⌋·2⁶⁴ + v mod 2⁶⁴: 2⁶⁴ is no int64 literal, and the low digit is in
  U, which Go's rung does not widen.
- **A new IR type for the double word**: the range type already says it, by the same theorem the
  selection uses; a new type would be a second statement of one fact.
- **A π or an `if`'s result in E**: the same law; not needed by a program yet (a π takes its source's
  type already, ADR 0103), owed with its trigger.

## Consequences

- A sum bounded inside E, read back into the word or returned into ℤ, is carried in the double word
  on Go, the JVM and windows; JavaScript has no E and keeps BigInt.
- The differential's `e-loop` runs it on four targets with its inputs unknown; the book's chapter 12
  (§12.7) teaches a sum carried in two words, and chapter 13's table sum is one
  ([eloop-2026-11-22](../../gauntlet/results/eloop-2026-11-22.md)).
