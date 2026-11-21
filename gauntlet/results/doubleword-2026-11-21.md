# The double word on every host: Go by math/bits, the JVM by its carries

Date: 2026-11-21. ADR 0149. The book's finding 118. hamza, 2026-11-21: "1. E on Go and the JVM, yes".

## The law

E is ℤ/2¹²⁸, injective on (−2¹²⁷, 2¹²⁷) and on [0, 2¹²⁸), each value read by its fact (ADR 0105). A
host with 64-bit words holds an element as two digits, x = h·2⁶⁴ + l mod 2¹²⁸, and each operation is
the ring's:

    x + y:  l = xₗ + yₗ, carry c = ⌊(xₗ + yₗ)/2⁶⁴⌋;  h = xₕ + yₕ + c
    x − y:  l = xₗ − yₗ, borrow b;                  h = xₕ − yₕ − b
    x · y:  (h₀, l) = xₗ·yₗ in full;                 h = h₀ + xₗ·yₕ + xₕ·yₗ   (xₕyₕ·2¹²⁸ ≡ 0)

A read-back into the word is a function of the integer: + − · & by the low digits alone (ℤ → ℤ/2⁶⁴ a
ring homomorphism), a comparison lexicographically, a quotient by d through the 2-by-1 division on the
magnitude, whose precondition |x|ₕ < d is the quotient's being a word (d·q ≤ |x| < d·(q + 1)), and a
remainder by Horner's rule in base 2⁶⁴,

    x mod d = ((h mod d)·2⁶⁴ + l) mod d,

since a remainder's quotient need not be a word. By 2ᵏ: x mod 2ᵏ = x's low k bits whatever its sign
(the 2-adic residue), and the truncated quotient (x + [x < 0]·(2ᵏ − 1)) >> k.

On the JVM, which has neither a carry nor a 128-by-64 division: the carry of a sum is the majority
function's top bit, ((a ∧ b) ∨ ((a ∨ b) ∧ ¬l)) >>> 63; the unsigned high product is the signed one,
`Math.multiplyHigh`, plus (a >> 63 ∧ b) + (b >> 63 ∧ a), a's unsigned value being a + 2⁶⁴·[a < 0]; the
2-by-1 division Knuth's Algorithm D in base 2³² (Warren's divlu).

## What was built

1. **`ir/golang/double.go`** and **`ir/java/double.go`**, each operation of E and each read-back;
   **`ir/plan/double.go`**, the high digits some use reads, the least fixed point both printers print
   by (Go refuses a variable declared and not used, and no use reads the others).
2. **`repr double 128` and `repr divide host`** on Go and the JVM; `ir.HasTwoByOne`, an ISA's 2-by-1
   division or the printer's own.
3. **A signed quotient of E by a constant d** is the magnitude's: on LLVM its unsigned division by
   LLVM itself (`divMagnitude`; multiplications on x86-64, inline on i686), since LLVM's own
   `sdiv i128` by 7 is `__divti3`, which no windows program links; on Go and the JVM the 2-by-1
   division with Horner's rule for a remainder. A quotient by a word needs a 2-by-1 division
   (`ir.HasTwoByOne`), which i686 lacks.
4. **Tests**: `TestTheDoubleWordOnEveryHost` (cmd/build: the midpoint at S₆₄'s ends, a quotient and a
   remainder by 7, run on Go, the JVM and windows from a table at a loop's index so nothing folds;
   `PlantDoubleCarryDropped` on each host and `PlantRemOneDivision` answer wrongly or panic; windows32
   computes the same); `TestASignedQuotientOfEIsTheMagnitudes` (ir/llvm: no signed i128 division by 7,
   `plantDivTI3` prints it again); the differential case `double-word.oro`, seven programs agreeing on
   Go, the JVM, windows and windows32. Every generated function
   checked against `math/big` and `BigInteger` on three million random and edge inputs.

## What failed first

- **The generated files imported `math/big` unused**: asking a double-word value's Go type named the
  big integer's package, its IR type being a range past the word; its lower end is read from that range.
- **The midpoint was 2.38 ns, Bloch's 1.81**: `Div64` by 2 is a division; by 2ᵏ the shift, 1.52–1.71 ns
  against 1.61–1.79, the same within the noise.
- **windows did not link a quotient by 7**, ADR 0103's own: LLVM's i128 division by a constant not a
  power of two is `__divti3`.
- **The flags test caught my rule for a remainder**: one 2-by-1 division of x·(2⁶³ − 1) by 10, whose
  quotient ≈ 2¹²² is no word, breaks `divq`'s precondition. Horner's two divisions.
- **Then the review run caught the rule itself**: `divq` had replaced popcount's and stein's printing
  divisions by 10¹⁸ on windows, where LLVM's `udiv i128` by a constant is four multiplications by the
  reciprocal and needs no precondition. Measured with clang at -O2: only the signed i128 divisions are
  calls, and only on x86-64; on i686 all four are inline. So LLVM divides the magnitude itself, the
  2-by-1 division and Horner stay Go's and the JVM's, and windows32 computes the quotient by 7, which
  the first rule refused.
- **Go refused a low digit no use read**: a shift by 64 or more reads the high digit alone, so
  `u64-mulhi`'s `v60l` was declared and not used; the dual fixed point, `plan.DoubleLows`. The case's
  skips on Go and the JVM, "where shr is not built", were stale: both compute it now.
- **The witness folded**: `main` with constant arguments computed everything while compiling; the
  arguments are read from a table at a loop's index.

## Paid and bought

| | |
|---|---|
| compile time | 1.07× the baseline (`cmd/check`'s gate, median of 116 compiles over 300 ms), no suspect |
| run time | the midpoint in E on Go 1.52–1.71 ns, Bloch's form 1.61–1.79, a big integer 39; `w * h <= N` in E 0.58 ns, its rewrite 2.34 |
| code size | unchanged: no existing program's emitted text changed on any target |
| emitted text changed | 4 changes, all new: `double-word.oro` on four targets. The first review run's two windows changes (popcount, stein: `divq` for LLVM's multiplications by the reciprocal) are gone with the rule that made them |
| proofs | 12,764 of 12,818 operations, from 12,731 of 12,785: the new case's 33, all proven; loops 1,603 of 1,906, unchanged |
| refusals changed | accepted on Go and the JVM: an intermediate in E read back into the word, the five programs among them; windows: a quotient of E by a d not a power of two links |
| compiler lines | +1,003 −4 outside tests (`ir/golang/double.go`, `ir/java/double.go`, `ir/plan/double.go`, `ir/llvm/llvm.go`, `ir/double.go`, `emit/target.go`); tests +140 −15 and two new files; targets +15 |
| documents | ADR 0149; the book's chapters 6 (§6.9 on the three readings and the five programs) and 8, finding 118; inventory.md (`divide`); CLAUDE.md, README; this |
| rule made less general | none |
| not built | E on JavaScript (no 64-bit word; BigInt, and finding 114); E's timings on windows against the algebraic forms; `Math.multiplyHigh` on ART |
