# 0086 — A number is an element of ℤ[1/10], and a rounding is its one lossy operation

Date: 2026-11-01
Status: Accepted; not built. Amends ADR 0009 (floats fold exactly, not through `float64`) and ADR 0085
item 3 (an unrounded float fold is exact, so associative).

## Context

Walls 5–7 of the seven [llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md) found
in Σ_LLVM are float formats, relations and the float environment. Until now floats were the targets'
operations (`go.f+`), never the language's. [floats-research.md](../floats-research.md) researched them
together with decimals and literals:
- **Part I** proposed one float sort, binary64's, with narrower formats as sets inside it.
- **Part II** answered hamza's question whether the number could be decoupled from its format, as
  integers were. It could, and Part I became the special case "a rounding at every operation".
- **Part III** wrote the result in J, in programs and target declarations.

hamza (2026-11-01): "I agree to all recommendations": G1–G7 (§17) and S1–S4 (§21).

## Decision

1. **One numeric sort, ℤ[1/10]** (G1). It is the least subring of ℚ containing ℤ, every binary float
   and every decimal numeral (Theorem R).
   - Its +, −, ·, = and < are exact and decidable. The language has one `+` for every number.
   - `Int`, the ranges, F(p, e_min, e_max) for any p, D(k, R) and Q(k, R) are its sets, defined in
     `lib/num` and not in the compiler.
   - `F64 = F(53, -1022, 1023)`, `F32`, `F16`, `BF16`, `E4M3`, `E5M2` and `E2M1` are named sets.
   - A literal (`0.1`, `1e-3`, `0x1.8p3`) is an element, exactly, and nothing rounds it unless the
     program writes a rounding.

2. **A rounding r_S,m is the only lossy operation** (G2). It is spelled `round(S, e)` or
   `round(S, e, mode)` (S1), with S a static set and the mode a tag:
   - `'nearest`, which rounds to even and is the default;
   - `'away`, `'zero`, `'up`, `'down`;
   - `'sat`, total;
   - `'stochastic(bits)`, with its random bits explicit.

   The operations that leave the ring are written only at the root of a rounding, one per rounding,
   which makes each computable (Theorem T: Lindemann–Weierstrass, with Ziv's loop for the
   transcendentals). They are:
   - the real quotient `÷`, ASCII `\div` (S3);
   - `sqrt`, `exp`, `log`, the trigonometric functions and their inverses, `atan2`, `pow`.

   A claim may write them anywhere, because the logic never prints (ADR 0049). Integer `/` and `%`
   keep their meaning, r_ℤ toward zero of a ÷ b, and **`/` requires integer operands**: `3.5 / 2` is
   refused and points to `÷` (S4).

3. **Every answer is a function of the inputs** (Theorem D). Neither ρ, nor the target, nor the
   strength of the proofs enters it, so ADR 0009 and ADR 0049 hold with no exception. Staging folds
   numbers exactly.

4. **The facts choose the representation** (G3), as ADR 0079 has it for integers. A value's facts are
   its interval and its lowest digit, the valuations v₂ and v₅. ρ chooses cheapest first:
   - the word;
   - a double or a float;
   - a pair or expansion of doubles;
   - an exact accumulator;
   - a scaled decimal word;
   - limbs or the host's big number, by consent, or refused (ADR 0019).

5. **A rounding is realized as cheaply as its term allows** (G4):
   - Figueroa's fusion where the term is one host operation, `r64(a·b + c)` as `fma`;
   - round to odd;
   - error-free transformations;
   - the accumulator;
   - a correctly rounded transcendental written in the language (CORE-MATH's algorithms).

   **A target declaration realizes a rounding by its `ensure`**, `r == round(S, t)`, checked against
   the host (ADR 0022) and by a conformance case. A host operation without one is the host's,
   Tier 2.

6. **Accuracy is a claim, proven or refused, never a chooser of precision** (G5). A tool may propose
   roundings, and the program contains them.

7. **IEEE's per-operation arithmetic is a static scope, `round(F64) { … }`** (G6, S2). D rewrites every
   operation and literal inside it to its rounding, and nothing about it is dynamic.

8. **No exceptional values** (G7). The ring has no ∞, NaN or −0.
   - A rounding's overflow is its obligation; `'sat` is the total alternative.
   - ÷'s divisor and √'s operand have domains.
   - A host's double is its own type (`go.float64`), with `F64 ⊆ go.float64`, and is entered by one
     partial decode, as ADR 0030 enters a host string.

## Why not

- **One float sort, binary64's, with narrower formats as sets inside it** (Part I's F2):
  - Its `+` is ⊞₆₄, a second operation beside ℤ's, so the integers and the floats are two algebras.
  - Every format above binary64's precision, and every decimal, needs another sort.
  - Its `0.1` must be rounded where it is read.
- **Exact reals** (Boehm's constructive reals, iRRAM, RealPCF): by the Grzegorczyk–Ceitin theorem a
  computable function on ℝ is continuous, so `<`, `=` and rounding are not computable on it.
  ℤ[1/10] is the largest ring we need on which they are.
- **ℚ as the sort**: it is closed under ÷, but a rational's denominator grows without a bound the facts
  can give. Every float and every numeral is already in ℤ[1/10], and ÷ under a rounding is decided
  exactly (Theorem T).
- **Precision chosen from an error bound** (Rosa, Daisy, FPTuner as a compiler): the printed answer
  would depend on the analysis, which ADR 0049 forbids. Kept as a tool that proposes roundings.
- **Ada's declared `digits` and model numbers, Fortran's `selected_real_kind`**: the first is a
  relation, a set of permitted results; the second names a format, which is Part I.
- **NaN, ±∞ and −0 as values of the language's numbers**: they break the ring's laws (x = x, the order,
  x − x = 0) for every program, to serve those that want them. Those programs keep the host's type, or
  state an `Option`.
- **Fast-math, `fmuladd`, unordered reductions and the dynamic rounding mode as the language's**: each
  is a relation, or state every operation would read (survey §4.4–4.5). They stay callable by name,
  Tier 2. An exact fold is associative, so it gets the vectorization `reassoc` buys, with one answer.
- **The rounding spelled as an infix (`e round F64`) or as a set applied (`F64(e)`)**: the second is
  refused today, since a set has no representation in a term, and both hide that this is an operation
  (S1).
- **`/` extended to the ring as the integer quotient**: sound, but a reader of `3.5 / 2` expects 1.75
  (S4).

## Consequences

- **The type checker's numeric sort becomes ℤ[1/10].** Integer programs are unchanged, because ℤ is a
  subring and `/` on integers keeps its meaning. Linear arithmetic over ℚ is the decidable theory
  there, where Fourier–Motzkin is complete, and the Omega test stays the procedure for the integer
  subset.
- **The core's float term kind becomes a ring literal**, m·10ᵏ exactly. The IR's text carries it so.
- **Every float program states bounds**, as integer programs do (ADR 0019). The exits are a narrower
  set, `'sat` or `-checked`. The decode at an input is where a range is usually stated.
- **`go.f+` and its kin become Tier 2**, and the gauntlet's float programs (`dot`, `centroid`), each
  written three times today, become one source in the language's numbers.
- **The cost of exact interiors is a measurement owed**: a sum costs 2.96× scalar, while a dot
  product, the accumulator vectorized against `reassoc`, and JavaScript's `fma` emulation are not
  measured yet.
- **Theorem T's cost for an arbitrary p is bounded in practice, not in proof**. For binary32 and
  binary64 the worst cases are known (Lefèvre and Muller).
- **Planted faults, when built**:
  - a compound term printed as per-operation native code;
  - a double rounding through binary64 for p > 25 without round to odd;
  - `r64(r64(a·b) + c)` contracted into `fma`;
  - Go's fusion of `x*y + z` across a missing conversion;
  - a decode that lets a NaN in;
  - two non-closed operations under one rounding, accepted;
  - a realization whose `ensure` is false (Go's `math.Exp` claimed as `round(F64, exp(x))`).
