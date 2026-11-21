# 0148 — Where the box forgets a relation, Presburger arithmetic decides the value's word

Date: 2026-11-21
Status: Accepted (hamza, 2026-11-21: "yes, I agree. if this is not in the book, include it, it is a
good example and a good explainer. always mathematical and algebraic"), on the book's finding 108.
Extends [ADR 0019](0019-precision-by-declaration.md)'s decision and
[ADR 0147](0147-int-is-the-integers-at-an-entry-point-and-decimal-renders-them.md) item 1 (`Int` is ℤ as a
claim at a binder too, the consent there).

## Context

The midpoint of two signed words, `(lo + hi) / 2`, is refused: the sum lies in [−2⁶⁴, 2⁶⁴ − 2], in
neither word, and nothing asked for exactness (finding 108). Two ways out failed:

- **The consent was target-dependent.** `sum ∈ Int = lo + hi` and a helper `fn add(a ∈ Int, b ∈ Int)
  (Int)` compiled on windows and were refused on Go and the JVM: `Int` canonicalizes to `"int"`, which a
  target holding a bignum reads as its word in a value's type, and an ascription of it was erased as a
  word's. ADR 0147 says `Int` denotes ℤ at every place.
- **Bloch's form was refused although every value is in a word.** `lo + (hi - lo) / 2` under
  `where lo <= hi`: d = hi − lo ∈ [0, 2⁶⁴ − 1], q = ⌊d/2⌋ with 2q ≤ d ≤ 2q + 1, so 0 ≤ q ≤ d and
  lo ≤ lo + q ≤ hi ⊆ S₆₄. The interval domain gave hi − lo [−2⁶⁴ + 1, 2⁶⁴ − 1], since an assumed
  comparison narrowed each side's interval and recorded no difference, and lo + q [−2⁶³, 2⁶⁴ − 2],
  since a box forgets that q was computed from hi − lo.

The second is the general fact about non-relational domains: **the natural interval extension of an
expression in which each variable occurs once is its exact range** (R. E. Moore, *Interval Analysis*,
1966; each of +, −, a constant's ·, ⌊·/k⌋ is monotone in each argument, and independent arguments
reach every corner). The box loses precision exactly where an atom repeats or two atoms are related.

## Decision

1. **A claim of `Int` at a binder is ℤ**, written as the range with both ends open (`core.ZRange`,
   `core.AscribedSet`), the consent above the word on every target. A definition's result `Int` is
   not: inlined, it is a claim every value satisfies, and a helper asks by a set above the word
   (finding 116).
2. **An entry assumption x rel y is a difference fact**, x − y in rel's half-line, the zone domain's
   entry (Miné 2001) that the assumption states, kept beside the intervals and read where a π's guard
   is (`assumeDiff`, `guardDiff`). An `assume` sits at the function's entry and values are immutable,
   so it holds wherever the two values are used.
3. **Where an arithmetic operation's box leaves the word and its term repeats an atom or relates two**
   (an assumption, a guard on a π the term reads), the term is unfolded through +, −, a constant's ·,
   `/` by a positive constant and the unsigned word's ring operations (ℤ's on their preimages, which the
   words pass chose only where ℤ's result is in U), each atom bounded by its fact, each related pair by
   its difference, each division by its definition (truncated: x = k·q + m with m's sign x's), and
   "the value is in S₆₄" (then "in U") is decided by the Omega test. Valid, the box is met with that
   word; otherwise it stands (`ir/relate.go`). The answer is memoized per statement and its atoms'
   bounds; a term of independent atoms, the box of its combined coefficients.
4. **A range claim the intervals do not establish is decided by the logic** before it is refused
   (`emit.DecideRangeClaim`): a claim tighter than the word, the midpoint of 0..2⁶² claimed in it, is
   [0, 3·2⁶¹] by the box and lo ≤ m ≤ hi by Presburger arithmetic.
5. **Presburger's constants are integers of any size** (`presburger.Lin.K`, `Constraint.K` a
   `*big.Int`; coefficients stay int64): an elimination's constants are the facts' ends combined, and
   those are the word's own. An arithmetic obligation the linear fragment, whose constants are int64,
   fails goes to the logic before the program is refused.
6. **A constant past the word, widened into the rung, is its Horner form over 10¹⁵**, each digit in
   the word, as the reader writes a literal past int64: S₆₄'s ends claimed `∈ Int` on JavaScript.

## Why not

- **A relational domain everywhere** (octagons, Miné 2006; convex polyhedra, Cousot and Halbwachs
  1978): every value would carry its relations at every program point, a cost on every program for a
  precision Moore's theorem says is needed only where an atom repeats. Polyhedra are also rational:
  ⌊d/2⌋ ≤ d needs the integers, which the Omega test decides and Fourier–Motzkin does not.
- **Recognizing the midpoint**: a pattern `lo + (hi - lo) / 2` is a list of cases where the derivation
  is general; `relate` proves `a + (b − a)/k` for any k ≥ 1, `x − x + y`, and whatever else a repeated
  atom makes exact, and nothing it does not prove.
- **The logic on every operation**: deciding each operation's word in Presburger arithmetic is
  complete for this fragment and costs a decision per operation on every program; the box is exact on
  independent atoms, so the step runs only where it can win (measured: 1.75% of the CPU of K(ℕ\*)'s
  heaviest test, where it was 8% before the structural scan; 4.4% of `fnv`'s windows compile with
  one question asked first and the answers kept, where it was 25%).
- **Leaving it to the author**: an `assert lo <= lo + (hi - lo) / 2 <= hi` would let the logic carry
  it, but the derivation exists, and a fact the compiler can derive is not the author's to state
  (prove, do not clamp).
- **The silent rule**, integers-boundary-research §7 whole, every intermediate exact where its consumer
  is bounded: an accumulator unbounded only through a table's length would become a big integer
  silently (finding 108). Not taken; a program asks, by a claim or by an algebra that stays in the word.
- **Reading `Int` at a binder as the word, and only ranges above it as consent**: that is the old
  reading ADR 0147 removed, `Int` denoting two sets.
- **A helper's `(Int)` as the consent too**: built and taken back. The rung's demand puts a value
  whose fact leaves S into the big integer ahead of U, so every library helper returning `Int` sent
  its unsigned values there (`strconv.FormatUint`'s argument, an index, a tuple's components),
  refused programs the IR could not lower, and slowed five windows compiles 1.5–1.7×. It waits on a
  demand that prefers U (finding 116).

## Consequences

- Bloch's midpoint compiles to Go's `uint64(hi) - uint64(lo)`, halved, added back signed, and LLVM's
  `sub`, `udiv`, `add nsw`, with no big integer, on Go, the JVM and both Windows triples
  (`gauntlet/differential/cases/bloch-midpoint.oro`), and over BigInt on JavaScript.
- The decision is relational only where a term repeats an atom or a relation is stated; every other
  verdict is the box's, unchanged.
- An obligation, a claim or an operation whose constants pass int64 is decided exactly.
- On JavaScript a pair of digits added to a negative word is still refused (finding 114): the pairs
  pass has no signed reading.
