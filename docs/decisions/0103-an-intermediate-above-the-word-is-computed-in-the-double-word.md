# 0103 — An intermediate above the word is computed in the double word

Date: 2026-11-11
Status: Accepted (hamza, 2026-11-11: "go ahead with M4", the standing measure's 64-bit digits, which **Item 4 amended by [ADR 0104](0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md)**: a divisor may be a positive word, through the triple's 2-by-1 division, where the quotient is proven a digit. **Its domain amended by [ADR 0105](0105-the-decisions-ends-are-192-bit-and-the-double-word-has-two-readings.md)**: (−2¹²⁷, 2¹²⁷) ∪ [0, 2¹²⁸), with E widened to 2¹⁹⁰.
need it). Builds the first part of [ADR 0083](0083-a-representation-is-an-abstraction-function-chosen-by-place.md)
item 3, native iN above the word on LLVM, for intermediates, and extends
[ADR 0082](0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md)'s fusion.

## Context

K(ℕ\*)'s digits are 32 bits, so every product, sum and quotient by a word stays in U₆₄, and every
digit step does half the work a 64-bit digit would. Its printing does about a quarter (llvm-udiv-2026-11-11:
the product is 1.43× Go's, printing 73% of `fact`). A wider digit needs an intermediate wider than the
word: the multiply-accumulate t = u·y + w + k of base-B digits reaches B² − 1.

The decision's finite ends lie in E = (−2¹²⁶, 2¹²⁶) (ir/endpoint.go). So B = 2⁶³ is the largest base
whose digits are non-negative words and whose t, at most 2¹²⁶ − 1, the decision proves exactly.

The language already states t, as ℤ's `u * y + w + k`, then `t / 2^63` and `t % 2^63`. What it lacked
is a representation for t: such a program is refused, since t leaves the word (ADR 0019), and a claim
above the word would hold it in K(ℕ\*) itself, inside K(ℕ\*).

## Decision

1. **An intermediate is computed in the double word**, i128 on a target declaring `repr double 128`:
   - an intermediate is a value whose fact is finite and outside the word, and every use of which
     reads it back inside its term: + − · neg (the result again an intermediate or in the word), a
     quotient, remainder, `mod` or right shift of which it is the dividend by a positive constant, a
     comparison, or a π, whose uses are then its own;
   - **Theorem**: the residue map ℤ → ℤ/2¹²⁸ is a ring homomorphism, injective on (−2¹²⁷, 2¹²⁷) ⊃ E,
     so computing such a term in i128 gives ℤ's value at every step; each reading back is a function
     of that integer.
2. **No value of the program leaves the word.** An intermediate is never a table's element, an
   argument, a loop variable, a branch's condition or an exit. So it needs no claim above the word
   (ADR 0079), as a fused term's intermediates need none (ADR 0082). A program whose operation leaves
   the word and is read back into it is accepted where a double word exists, and refused elsewhere:
   portability is computed (ADR 0026).
3. **W14 checks it**, independently of the pass: every use of a double-word value is one of item 1's,
   and only on a target declaring one. The decision proves each such operation in E.
4. **A divisor is a constant.** LLVM divides an i128 by a variable through `__udivti3`, a library call
   MSVC's link does not take by default. A quotient by a run-time word is the library's own
   computation, by its precomputed reciprocal (Möller and Granlund 2011).
5. **The printer states the proofs**: `nsw` on each operation (its result is in E); `nuw` on `add` and
   `mul` of naturals; and `udiv`, `urem`, `lshr` for a natural (llvm.md §3c). The flag witness reads
   them back.

## Why not

- **Widen E to 2²⁵⁴ and take B = 2⁶⁴.** Every interval operation of every program would pay for four
  limbs, so the compile gate and every analysis would pay, for one bit of density (63/64). It is the
  same construction one width up, and waits for a program that needs it.
- **B = 2⁶⁴ with `mulhi`, `uadd.with.overflow` and `usub.with.overflow` as host operations.** That puts
  LLVM's names in the language's own library and makes the double word's product an LLVM primitive,
  where ℤ already states it. The intermediate would be no less real, only unseen by the decision,
  which would then prove less.
- **A sort for the double word.** A sort is a set a program names. No program names this one: it is
  the representation of a term's intermediate, ρ by place (ADR 0083 item 2), as `mod 2ⁿ`'s residue is.
- **Hold every finite set above the word in iN now** (ADR 0083 item 3 whole, with the limbs retired).
  That changes what programs that already compile hold, which ADR 0083 measures first, against the
  limbs, on the u128 factorial and Karatsuba programs. This decision changes only what was refused.
- **Accept a variable divisor and link compiler-rt.** A run-time division in i128 costs a call on every
  digit, where the reciprocal costs two multiplies; the one consumer is the library, which divides by
  its reciprocal.

## Consequences

- K(ℕ\*) can take B = 2⁶³ (M4b): every digit operation is ℤ's, the decision proves each in E, and clang
  prints `mulq`, `add`, `adc` and `shld`.
- A program computing (a·b) mod m with a constant m below 2⁶² now compiles on windows: the Mersenne
  hash, `gauntlet/differential/cases/mersenne-hash.oro`.
- Go, the JVM and JavaScript declare no double word. Go could realize one by `math/bits`' `Mul64` and
  `Div64`, a step for a program that asks for it there.
