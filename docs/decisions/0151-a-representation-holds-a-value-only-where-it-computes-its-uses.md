# 0151 — A representation holds a value only where it computes the value's uses

Date: 2026-11-21
Status: Accepted (hamza, 2026-11-21, on the three options for JavaScript's open refusals: "I agree.
record 2 and 3 as optimizations for another time": option 1, the pairs as a selection, with option
2's two cheap operations, comparison and the signed reading). Amends
[ADR 0083](0083-a-representation-is-an-abstraction-function-chosen-by-place.md) item 2 (U on
JavaScript, two 32-bit digits, for every value of U) and
[ADR 0150](0150-a-set-is-a-claim-and-rho-alone-chooses.md) item 1 (ρ the cheapest representation whose
domain holds the fact). Closes the book's finding 114.

## Context

ADR 0150 made ρ(v) the cheapest representation whose domain holds v's proven fact, over S, U, E, ℤ.
On JavaScript U is a pair of 32-bit digits (ADR 0083), and the pair computes addition, subtraction,
multiplication, the bit operations, shifts by constants and division by powers of two. A value of U
whose program divides it by 3, compares it, or adds it to a negative word was refused, saying "not
built": a refusal of a program every other target compiles, for an operation ℤ has. Five differential
cases skipped JavaScript for it (`u64-digits`, `u64-square`, `u64-values`, `mersenne-hash`,
`double-word`), and finding 114's midpoint.

## Decision

1. **ρ(v) is the cheapest representation whose domain holds v's fact and whose operations cover v's
   uses.** A domain is not enough: a representation is an algebra, and a value is held in one only if
   that algebra has every operation the program applies to it. On every target but JavaScript the two
   conditions coincide (S, U and E each have the ring, the order and the divisions); on JavaScript a
   value of U leaves the pair exactly when a use of it, or the operation computing it, is one the
   rung computes and the pair does not: a division by a non-power of two, a shift by a variable, a
   comparison with a word that may be negative, an operation whose result leaves U.
2. **The selection is a fixed point, and the rung computes it.** `PairsLeaving` (ir/pairs.go) is the
   set of such values on the decided program; D_A demands it, and the rung's closure takes what it
   touches (an operation over an exact operand is exact, an exact result's operands demanded). The
   greatest fixed point of "every use covered" on the pair is the least on ℤ: one lattice from both
   ends. A residue (an operation fused under mod 2⁶⁴) never leaves; where one meets ℤ it enters by the
   inclusion ι : U ↪ ℤ, `pair-bigint` at the rung's `big-of`.
3. **A value leaves whole, never one operation at a time.** Measured (pairsel-2026-11-21): a digit sum
   over U in BigInt throughout 531 ns a value, the pair with each division conjugated through BigInt
   (r ∘ op_ℤ ∘ ι) 1,869 ns.
4. **The pair compares.** The order of U is the lexicographic order of (h, l), since d(h, l) = h·2³² + l
   with l < 2³²: x ⋈ y is xₗ ⋈ yₗ where xₕ = yₕ, and xₕ ⋈′ yₕ elsewhere, ⋈′ the strict order ⋈ refines.
5. **The pair reads a word signed.** A ring or bit operation's operands are read by their residues mod
   2⁶⁴ (ℤ → ℤ/2⁶⁴ is a ring homomorphism, and a residue's bits are the integer's low bits); a result in
   S is ⌊v/2³²⌋·2³² + l with ⌊v/2³²⌋ = h − 2³²·[h ≥ 2³¹]. The rebuilt value and its high digit are
   ascribed their decided facts, as a realization's site keeps its fact (ir/realize.go): the expansion
   is a homomorphism, and the decision run after cannot see the theorem.
6. **A word divided by a pair above it reads no digit of it**: for |a| < b, a / b = 0 and a rem b = a,
   truncating (the Mersenne hash's first step, a word by 2⁶¹ − 1).
7. **A value of U held exactly leaves as U does.** At a host's parameter of any type a value of U is
   its decimal string (`pair-box`), and held exactly it is the same string (`big-box`), since a
   representation changes no value; at a result declared in U it is the host's BigInt in either
   (`pair-bigint`). Neither position is a sink for the taint (ir/exact.go).
8. **What the rung hides from the decision taken again is ascribed.** A value held exactly is a host's,
   ⊤ to the decision, and a word computed from it loses its bound (a digit sum's accumulator, bounded by
   the trip count of the loop dividing v by 10). Each word variable of a loop with a variable held
   exactly, and each retraction `big-word` and remainder `big%-small`, is ascribed the fact the first
   decision proved on ℤ. Placed only where a value of U left the pair (its fact in U, held exactly
   on a target whose U is a pair), so no program accepted before changes: on K(ℕ\*) stein's gcd on
   windows did not finish compiling with them, and on `Nat` loops elsewhere they cost termination
   proofs (pairsel-2026-11-21). Size-change termination reads an ascription as the identity, the(x) =
   x, so a variable renamed by one is the same variable for its steps.
9. **The report says which.** An operation of U the pair does not compute is reported apart from one
    past every word: `f: N operation(s) of the unsigned word that its pair does not compute, held
    exactly: a * a; a * a / 3`.

## Why not

- **Conjugation, each operation the pair lacks computed through ℤ** (r ∘ op_ℤ ∘ ι, the value kept a
  pair): local and simpler, and 3.5× slower than the value in ℤ throughout on a loop (§3's
  measurement), every iteration paying both conversions.
- **Every value of U in ℤ on JavaScript**: the pair is 4.8× faster than BigInt on a decode's
  multiply-add (u64repr-2026-09-22) where it covers the uses, and a hash is the program it was built
  for.
- **The pair's missing operations, now** (option 2 whole): short division by a constant d < 2²¹ on
  the digits, two exact float divisions, measured 4.3× faster than BigInt on u64repr-2026-09-22's
  kernel and 1.6× on this digit sum; a variable shift; a build's region. hamza: recorded as
  optimizations for another time, each its trigger a program the selection sends to ℤ and a
  measurement that asks. (An entry point's parameter as a pair, listed here first, was built after:
  the BigInt its caller holds split once by pair_bigint's inverse, pairsel-2026-11-21's gap 1.)
- **The program's one set (ADR 0029) widened to what A holds exactly**, the width raised to every
  proven fact past the word: built first, it made `x * 3` under a declared S₆₄ compile on JavaScript,
  and it lost `big-sign`'s trap, a yield past its declared −2²⁰⁰..2²⁰⁰ printed where it must stop (a
  wrong answer, on every host). The width is both the representation's room and the declared sets'
  enforcement, one number for two things.
- **No fit at an operation of D_A**, the fit kept only where a declared result claims (its yields
  followed back through joins): sound for an export's own result, and wrong once a definition is
  inlined, since nothing in the IR then marks its result as claimed (`big-sign`'s `calc` inside `main`,
  printed again). Both are the same question, and it is the next step's: a claim above the word
  carried through inlining as a mark, so the one set is enforced where a set claims and nowhere else.
- **The double word on JavaScript as four 32-bit digits** (option 3): recorded for another time with
  the same trigger; BigInt is E there today.
- **A refusal kept where the pair cannot compute** (ADR 0083's original rule): a refusal for a
  representation's missing operation, of a program every other target compiles: L4 broken by what one
  representation lacks rather than what the target lacks, since the target has BigInt.

## Consequences

- Four differential cases (`u64-digits`, `u64-square`, `u64-values`, `mersenne-hash`) and finding
  114's midpoint run on JavaScript, one answer on four targets; `double-word` reaches the one set's
  stop below instead of a refusal.
- A residue, a hash fused under mod 2⁶⁴, divided by a non-power of two is still refused: the rung holds
  no residue. Owed, with its trigger.
- The ascriptions of item 8 are placed only on the host's integer; on K(ℕ\*) a program meeting the
  same lost bound is refused as before (owed, with why the ascriptions there did not end).
- **Under a declared set above the word, an intermediate past it stops the program** with its value
  proven, as before this ADR: `x * 3 / 4` over S₆₄ on JavaScript, where S₆₄ is above the word, and
  `double-word` skips JavaScript for it (a checked skip, `output: trap`). The other three targets hold
  it in E. Not a wrong answer, a run-time stop; its fix is the claim carried through inlining (Why not).
