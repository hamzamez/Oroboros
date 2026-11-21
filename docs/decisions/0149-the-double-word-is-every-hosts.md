# 0149 — The double word is every host's, and a signed quotient of it the magnitude's

Date: 2026-11-21
Status: Accepted (hamza, 2026-11-21: "1. E on Go and the JVM, yes", on
[sets-are-claims-research.md](../sets-are-claims-research.md) §6). Extends
[ADR 0103](0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md) and
[ADR 0105](0105-the-decisions-ends-are-192-bit-and-the-double-word-has-two-readings.md) from the LLVM
layer to every host with a word of 64 bits; corrects ADR 0103's quotient by a constant.

## Context

ADR 0103 computes an intermediate the decision proves in E = (−2¹²⁷, 2¹²⁷), read back into the word, in
the double word: LLVM's i128. Only the LLVM layer declared it, so the same program had two verdicts: the
midpoint `(lo + hi) / 2` over full words, a comparator `a - b`, `x * 3 / 4`, `w * h <= N`,
`a + b > 2^63 - 1` compiled on windows and were refused on Go and the JVM (the book's finding 118).
Measured on Go by hand, the double word costs what the programs' algebraic rewrites cost (the midpoint
1.70 ns against Bloch's 1.79; `w * h <= N` 0.58 ns against its rewrite's 2.34).

And ADR 0103's signed quotient by a constant was wrong on windows for a d not a power of two: it
printed LLVM's own `sdiv i128` by 7, which LLVM lowers to a call of `__divti3` (and `srem` to
`__modti3`) on x86-64, and no windows program links the C runtime. Measured with clang 23 at -O2:
`udiv i128` and `urem i128` by a constant are multiplications by the reciprocal on x86-64 and an
inline expansion on i686, and `sdiv`/`srem i128` are inline on i686 and calls on x86-64.

## Decision

1. **E is ℤ/2¹²⁸ on every host with a 64-bit word**, held as two digits (h, l), x = h·2⁶⁴ + l mod 2¹²⁸,
   each value read by its fact, signed or unsigned (ADR 0105). Go: two `uint64`, `math/bits`'s `Add64`,
   `Sub64`, `Mul64`, `Div64` (`ir/golang/double.go`). The JVM: two `long`s, the carry of a sum the
   majority function's top bit, the borrow likewise, the unsigned high product `Math.multiplyHigh`
   corrected by the operands' signs, the 2-by-1 division Knuth's Algorithm D in base 2³², one static
   method per class (`ir/java/double.go`). `repr double 128` on both.
2. **A high digit is printed where a use reads it**: a read-back by + − · neg & into the word reads only
   the low digits (ℤ → ℤ/2⁶⁴ is a ring homomorphism), so the set of values whose high digit is printed is
   the least fixed point of the uses' flow (`plan.DoubleHighs`, shared by both hosts).
3. **A signed quotient of E by a constant d is the magnitude's**: |x| divided, the sign put back by
   truncation and `mod` lifted into [0, d). On LLVM the magnitude's division is LLVM's own unsigned one
   (`divMagnitude`), multiplications on x86-64, so no call is left; a natural's is LLVM's as it was. On
   Go and the JVM it is the 2-by-1 division (`Div64`, Knuth's), whose precondition, the high digit
   below d, a quotient proven in the word gives (d·q ≤ |x| < d·(q + 1)); **a remainder's quotient need
   not be a word**, so there a remainder is Horner's rule in base 2⁶⁴, x mod d =
   ((h mod d)·2⁶⁴ + l) mod d, two divisions whose high digits are below d. By 2ᵏ every host shifts:
   x mod 2ᵏ is the low k bits whatever x's sign (the 2-adic residue), the signed quotient
   (x + [x < 0]·(2ᵏ − 1)) >> k.
4. **A quotient of E by a word** is the 2-by-1 division, so only where the target has one: an ISA's
   (`ir.TwoByOne`, x86-64's `divq`) or its printer's own (`repr divide host`: Go's `Div64`, the JVM's
   Knuth division); i686 has none (x86's `divl` divides 64 bits by 32).

## Why not

- **The host's big integer for E**: `*big.Int` is 39 ns and three allocations for the midpoint, 22× the
  double word; E is the representation between U and ℤ in ρ's order, and a host with 64-bit words has
  its operations.
- **One expression per value**, a struct `{h, l}` or a `long[2]`: Go's helper functions inline under its
  budget, but a value type of two words is a copy per operation and an escape analysis per use; two
  scalar variables per value are what the measurement's hand-written form is, and what the printers
  emit.
- **LLVM's own signed i128 division by a constant**: `__divti3`/`__modti3` link on no windows
  program.
- **x86-64's `divq` for every quotient by a constant**, built first and taken back: LLVM's unsigned
  division by a constant is multiplication by the reciprocal (Granlund and Montgomery 1994), cheaper
  than a division, and its quotient has no precondition; `divq` replaced popcount's and stein's printing
  divisions by 10¹⁸ with two divisions each.
- **One 2-by-1 division for a remainder** (Go, the JVM): its precondition is the quotient's in the
  word, which a remainder does not give: the remainder of x·(2⁶³ − 1) by 10 would panic `Div64`.
- **`Math.unsignedMultiplyHigh`** (Java 18): `multiplyHigh` (Java 9) with the sign correction is the
  same integer one version earlier; whether ART has either is owed to an Android build.

## Consequences

- The five programs of the research note's §4 compile and agree on Go, the JVM and windows
  (`gauntlet/differential/cases/double-word.oro`), checked against exact arithmetic on three million
  inputs each on Go and the JVM.
- A windows program whose signed quotient of E by a d not a power of two printed `__divti3` links.
- JavaScript, which has no double word, computes over BigInt or meets finding 114.
