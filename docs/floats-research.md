# Research: floats in the language, their formats, their relations, their environment, and decimals

2026-10-30; Part II 2026-10-31; Part III 2026-11-01. **Decided 2026-11-01** ([ADR 0086](decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md)): hamza agreed to Part II's G1–G7 (§17), which revise F1, F2, F5, F6 and F10 of §11: one numeric sort ℤ[1/10], a rounding the only lossy operation, the representation the facts'; and Part III's surface questions S1–S4 (§21), with programs and target declarations in J (§18, §19) and what the design restricts on a host (§20). Walls 5, 6 and 7 of the seven
[llvmsurvey-2026-10-27](../gauntlet/results/llvmsurvey-2026-10-27.md) found (float formats, relations,
the float environment), the 10-24 plan's 6b steps 5–7, researched as one because they share one
algebra; and the question of decimals. hamza: "until now, floats are still using the target ops. not the
language. it is time to promote floats to the language the same way we did with integers … in ai these
days we use fp4, fp8 .. so include those … should we introduce decimals in the language same as Z. or
keep it a library? should there be a difference between a decimal literal and a float?" The
measurements in §10 were taken on this machine: LLVM 23.1.4, Go 1.27, JDK 17, Node 26.

**The short version:**
- **Today a float is each target's.**
  - A program writes ``go.`f+` ``, a host's operation.
  - `F64` is a host set per target, and `sqrt` is the only float function classified (Tier 1).
  - The hosts disagree in measurable ways (§10): Go fuses `x*y + z` into one rounding on arm64 (Android)
    and amd64 v3; the three print the same number differently; and their `sin` and `pow` differ in the
    last bits. Go's `pow` is off by up to **18,212 ulps** where Java's is correctly rounded.
- **The algebra that makes floats the language's** is IEEE 754's own, read as Kulisch's
  **semimorphism**:
  - a format F is a finite set of dyadic rationals;
  - a rounding ρ_F : ℝ → F is a monotone retraction;
  - each operation is the exact real operation followed by the rounding, x ⊞ y = ρ_F(x + y).

  So **a float is a real number**, as an `Int` is an integer (ADR 0026), and its operations are defined
  once, independently of any target, deterministic and portable.
- **One float sort, binary64's; every narrower format a set within it, with its rounding a map**:
  - F32, F16, BF16, FP8 (E4M3, E5M2), FP6 and FP4 (E2M1) are subsets of F64;
  - `r32(x + y)` is binary32's sum, as `(x + y) mod 2³²` is a 32-bit word's (ADR 0082);
  - a set never changes what `+` computes (ADR 0049).
- **Figueroa's double-rounding theorem (1995) is the floats' fusion**: +, −, ×, ÷ and √ computed in
  binary64 and rounded once give the correctly rounded result of any format with 2p + 2 ≤ 53. So
  `r32(x + y)` is printed as one native binary32 operation, exactly. Measured: **0 disagreements in 20
  million** binary32 triples; fma, which the theorem does not cover, disagrees 22 times. A rounding is
  not a homomorphism, so unlike `mod 2ⁿ` it fuses one operation at a time. Every format is Tier 1 on
  every target: native where the hardware has it, binary64 and a rounding elsewhere. The microscaling block formats (MXFP8, MXFP4, NVFP4) are a representation of a table
  (ADR 0085) with a shared scale.
- **Exceptional values are obligations, as overflow is for integers** (ADR 0019): an overflow to ∞,
  an invalid operation (NaN) and a division by zero are proven absent, checked, or consented to by
  claiming the extended set. Proving it is a float interval analysis on the IR (Miné 2004; Astrée).
- **Wall 6, relations**: the language's float operations are functions. Three kinds of relation
  appear, each with its answer:
  - **a reordered fold**: ADR 0085's, recorded;
  - **a transcendental function**: not correctly rounded by the hosts, so the language gives its own
    correctly rounded ones (CORE-MATH), or a host's is Tier 2 with its accuracy as a claim;
  - **stochastic rounding** (training in FP8): made a function by giving it its random bits as an
    argument.
- **Wall 7, the environment**: no dynamic rounding mode and no observable flags. A rounding mode is a
  static parameter of an operation, ∘_r, realized natively or by error-free transformations.
- **Decimals: in the language, like ℤ.** The decimals are the ring ℤ[1/10] = {m·10⁻ᵏ}: +, − and · are
  exact there, so a decimal is a number, and only division needs a rounding, which is stated. The facts
  choose its representation as they choose an integer's (ADR 0079): a scaled machine word when the scale
  and magnitude are bounded (measured 22× faster than `BigDecimal`), the host's big decimal otherwise.
- **Literals: a literal is an exact number, and the sort it meets decides it.**
  - `0.1` is the decimal 1/10.
  - Where it meets the float sort, it is rounded once, to nearest, at compile time, identically on
    every target.
  - In the exact sort it stays the exact decimal, so `0.1 + 0.2 == 0.3` holds.
  - A decimal or integer *value* becomes a float only by an explicit rounding.

  Go already reads literals this way, as exact constants; §10 shows the consequence.

---

## 1. The issue

### 1.1 What a program writes today

```
// examples/native/dot-go.oro
fn sum(v) {
    loop acc = 0.0, i = 0 {
        go.`>=`(i, vlen(v)) -> acc
        else -> again(go.`f+`(acc, vindex(v, i)), go.`+`(i, 1))
    }
}
```

A float operation is a target's primitive, ``go.`f+` ``, ``java.`f+` ``, and `F64` is declared by each
target (`set F64 = host "float64"`). So a program using floats names a host and is not portable, and
the language says nothing about what the operation computes. primitives.md classifies `sqrt` as Tier 1
("IEEE-754 requires correct rounding") and lists `sin`, `exp`, `pow` and `log` as absent because they are
not. ADR 0009 fixed one thing: compile-time float arithmetic is binary64, as at run time, because Go's
exact constant folding made `0.1 + 0.2` print `0.3` folded and `0.30000000000000004` at run time.

### 1.2 What the targets offer

| | Go | the JVM / Android | the browser (JavaScript) | LLVM (Windows) |
|---|---|---|---|---|
| binary64 | `float64` | `double` | `number`, the only numeric type | `double` |
| binary32 | `float32` | `float` | `Math.fround` after each binary64 operation | `float` |
| binary16 | none | none in JDK 17 (`Float.float16ToFloat` from JDK 20) | `Math.f16round`, `Float16Array` (Node 26: present) | `half` |
| bfloat16 | none | none | none | `bfloat` |
| FP8, FP6, FP4 | none | none | none | **no IR type**; LLVM's APFloat knows 12 formats of 8 bits or fewer (E4M3, E4M3FN, E4M3FNUZ, E4M3B11FNUZ, E5M2, E5M2FNUZ, E3M4, E2M3, E3M2, E2M1, E8M0FNU, TF32); operations only as target intrinsics on bytes (97 for FP8 on AArch64) |
| binary128 | none | none | none | `fp128` (software on x86) |
| fused multiply-add | `math.FMA`; **the compiler may fuse `x*y + z` by itself** | `Math.fma`; never fused implicitly (strict since JDK 17) | none (no fma) | `llvm.fma`; fused only under `contract` |
| rounding modes | none (`math/big.Float` has them) | none (`BigDecimal` has them) | none | constrained intrinsics; x86's MXCSR, AVX-512's embedded rounding |
| decimal | none (`math/big.Rat` is exact rationals) | `BigDecimal` | none (BigInt only) | none |

### 1.3 What the ML formats are

| format | sign / exponent / mantissa bits | precision p | largest finite | ∞, NaN | where |
|---|---|---|---:|---|---|
| binary64 | 1/11/52 | 53 | ≈1.8·10³⁰⁸ | yes | everywhere |
| binary32 | 1/8/23 | 24 | ≈3.4·10³⁸ | yes | everywhere |
| TF32 | 1/8/10 | 11 | ≈3.4·10³⁸ | yes | NVIDIA tensor cores (a computation format) |
| binary16 | 1/5/10 | 11 | 65504 | yes | GPUs, ARM, AVX-512 FP16 |
| bfloat16 | 1/8/7 | 8 | ≈3.4·10³⁸ | yes | training; Google TPUs, AMX, ARM (Kalamkar et al. 2019) |
| FP8 E4M3 (OCP "FN") | 1/4/3 | 4 | 448 | no ∞; one NaN pattern | training and inference (Micikevicius et al. 2022) |
| FP8 E5M2 | 1/5/2 | 3 | 57344 | yes | gradients; it is binary16 with the low byte cut |
| FP6 E2M3, E3M2 | 1/2/3, 1/3/2 | 4, 3 | 7.5, 28 | none | OCP microscaling |
| FP4 E2M1 | 1/2/1 | 2 | 6 | none | inference; its 15 values are 0, ±0.5, ±1, ±1.5, ±2, ±3, ±4, ±6 |
| E8M0 | 0/8/0 | — | 2¹²⁷ | one NaN | a block's shared scale, a power of two |

**Microscaling (MX)** (OCP MX specification v1.0, 2023; Rouhani et al. 2023): a block of 32 elements in
FP8, FP6, FP4 or INT8 shares one E8M0 scale, so element i is 2^s · cᵢ. NVIDIA's **NVFP4** (Blackwell)
uses E2M1 elements with an E4M3 scale per 16 and a binary32 scale per tensor. AMD's and Graphcore's
**FNUZ** variants have no negative zero and use the pattern 0x80 for NaN.

### 1.4 What decimals are asked for

Money, quantities written in decimal (0.1 kg), protocols that carry decimal strings (JSON numbers),
and the literal `0.1` itself, which no binary format holds.

---

## 2. The algebra of a binary format

### 2.1 A format is a finite set of dyadic rationals

A binary format with precision p and exponent range [e_min, e_max] is

```
F(p, e_min, e_max) = { (−1)ˢ · m · 2^(e − p + 1) : s ∈ {0, 1}, m ∈ [0, 2ᵖ), e ∈ [e_min, e_max],
                        m ≥ 2^(p−1) unless e = e_min }                     (the subnormals at e_min)
```

Every finite float is a **dyadic rational**, an element of ℤ[1/2] = {m·2ᵏ}. So F ⊂ ℤ[1/2] ⊂ ℚ ⊂ ℝ,
and **a float is a number**: 0.5 is one half, 0.1₆₄ is 3602879701896397/2⁵⁵.

IEEE 754 adds four special data:
- **±∞**, the overflow's results;
- **NaN**, with a sign and payload bits, the invalid operation's result;
- **−0**, a second zero that is distinguishable by 1/x, `copysign` and `atan2`.

They are not real numbers, and §4 decides what the language does with them.

### 2.2 A rounding is a monotone retraction, and an operation is a semimorphism

A **rounding** ρ_F : ℝ → F̄ (F extended by ±∞) is:
- **monotone**: x ≤ y ⇒ ρ(x) ≤ ρ(y);
- **a projection**: ρ(f) = f for f ∈ F, so ρ ∘ ρ = ρ, a retraction of ℝ onto F;
- for round-to-nearest-even, **odd**: ρ(−x) = −ρ(x).

IEEE 754 §4.3 requires +, −, ×, ÷, √, fma and conversions to be **correctly rounded**:

```
x ⊞ y = ρ_F(x + y)        x ⊟ y = ρ_F(x − y)        x ⊠ y = ρ_F(x · y)
x ⊘ y = ρ_F(x / y)        √̄x = ρ_F(√x)               fma(x, y, z) = ρ_F(x·y + z)
```

This is Kulisch's **semimorphism** (Kulisch and Miranker, *Computer Arithmetic in Theory and Practice*,
1981): the operation on F is the operation on ℝ composed with a monotone, odd projection. **It defines
each operation's result uniquely, independently of any machine.** So binary64 arithmetic is as portable
as ℤ's: the same inputs give the same bits on every conforming target, unless something rounds twice,
fuses, or substitutes a different operation (§5).

### 2.3 What the semimorphism keeps and loses

Kept:
- **commutativity** of ⊞ and ⊠;
- **monotonicity**: x ≤ y ⇒ x ⊞ z ≤ y ⊞ z;
- **exactness where the result is in F**: x ⊞ y = x + y when x + y ∈ F, so sums of small integers in
  F64 are exact below 2⁵³;
- **the error bound** |ρ(x) − x| ≤ u·|x| with u = 2⁻ᵖ, for x in the normal range (Higham, *Accuracy and
  Stability of Numerical Algorithms*, 2002).

Lost:
- **associativity**: (1 ⊞ 2⁻⁵³) ⊞ 2⁻⁵³ = 1, but 1 ⊞ (2⁻⁵³ ⊞ 2⁻⁵³) = 1 + 2⁻⁵²;
- **distributivity**;
- **cancellation**: x ⊞ y = x ⊞ z does not give y = z.

So (F, ⊞) is a **commutative, non-associative magma**. That is exactly why ADR 0085 keeps a float fold's
order.

**Exactness theorems the language can use as facts:**
- **Sterbenz** (1974): if y/2 ≤ x ≤ 2y then x ⊟ y = x − y exactly.
- **Error-free transformations**: s = a ⊞ b, b′ = s ⊟ a, a′ = s ⊟ b′ and e = (a ⊟ a′) ⊞ (b ⊟ b′) give
  a + b = s + e exactly, e ∈ F (Møller 1965; Knuth's TwoSum, TAOCP vol. 2; with |a| ≥ |b|, Dekker's
  Fast2Sum e = b ⊟ (s ⊟ a) in three operations). With fma, p = a ⊠ b and e = fma(a, b, −p) give
  a · b = p + e exactly (Dekker 1971). These make exact and directed computations portable (§7, §8).
  IEEE 754-2019 names them (`augmentedAddition`, `augmentedMultiplication`).

### 2.4 Formats are ordered by inclusion, and narrower formats live inside binary64

F(p, e_min, e_max) ⊆ F(p′, e′_min, e′_max) when p ≤ p′ and the exponent range, subnormals included, lies
inside. So:

```
FP4 E2M1 ⊂ FP6 E2M3 ⊂ FP8 E4M3 ⊂ binary16 ⊂ binary32 ⊂ binary64 ⊂ binary128
FP8 E5M2 ⊂ binary16             bfloat16 ⊂ binary32          (bfloat16 ⊄ binary16, binary16 ⊄ bfloat16)
```

This is a partial order, a lattice under join: bfloat16 ∨ binary16 = binary32, the least format holding
both.

**Figueroa's theorem** ("When is double rounding innocuous?", SIGNUM Newsletter 1995): for x, y in a
format of precision p, computing +, −, ×, ÷ or √ in a format of precision p′ ≥ 2p + 2 and rounding the
result to p gives the correctly rounded p-result:

```
ρ_p(ρ_p′(x ∘ y)) = ρ_p(x ∘ y)        when p′ ≥ 2p + 2,  ∘ ∈ {+, −, ×, ÷, √}
```

For binary64, p′ = 53, so every format with p ≤ 25 qualifies: binary32 (24), binary16 (11), bfloat16
(8), TF32 (11), FP8 (4, 3), FP6, FP4. **So each of them is exactly realizable on any target with
binary64 and a rounding function**: JavaScript's `Math.fround` is ρ₂₄ and `Math.f16round` ρ₁₁. Measured
(§10): 0 disagreements in 20 million binary32 triples. The theorem does **not** cover fma: its exact
product needs 2p bits before the addition, and 22 of the same 20 million triples disagreed. A narrow
fma is computed exactly by an error-free transformation, or natively.

### 2.5 A format is a set and its rounding a map; the operations are binary64's

Take x, y ∈ binary32. There are two readings of `x + y`:

- **(a) A format is a sort.** `x + y` is ⊞₃₂, rounded to binary32, and an operation's format is the join
  of its operands' formats (binary32 ∨ binary64 = binary64). This is C's, Java's and Mojo's reading.
- **(b) A format is a set, and its rounding a function.** F32 ⊂ F64, so x and y are binary64 values
  that happen to lie in F32. `x + y` is ⊞₆₄, and binary32's sum is written `r32(x + y)`, with
  r32 = ρ₂₄ the rounding the program applies.

**Two decisions already made settle it, for (b):**
- **ADR 0049: a claim never changes an answer.** Under (a), declaring the same value x ∈ F32 rather than
  x ∈ F64 changes what `x + y` computes. A set would print.
- **ADR 0082: wrapping is the residue map `mod 2ⁿ`, applied by the program, not a sort**, and
  *fusion* computes it in n-bit arithmetic. Rounding is the floats' residue map: ρ_p, a retraction.
  **Figueroa's theorem is the floats' fusion**: for ∘ ∈ {+, −, ×, ÷, √} on F-operands,
  ρ_F(x ⊞₆₄ y) = ρ_F(x ∘ y). So `r32(x + y)` is printed as one native binary32 addition, and is
  exactly that.

**Where floats differ from integers.** `mod 2ⁿ` is a ring homomorphism ℤ → ℤ/2ⁿ, so ADR 0082 fuses a
whole term. A rounding is no homomorphism, so **fusion is per operation**:
- `r32(a * b + c)` is binary64's two operations rounded once to binary32. That is more accurate than
  binary32's two roundings, and not equal to them.
- A program that wants binary32's arithmetic bit for bit writes the rounding at each operation,
  `r32(r32(a * b) + c)`. That is the cost of (b): a binary32 program's roundings are visible. A
  library's `add32` and `mul32` name them.

The same holds for a literal. **Rounding twice is not innocuous on an arbitrary real**, only on an
operation's result: x = 1 + 2⁻²⁴ + 2⁻⁷⁰ gives ρ₂₄(ρ₅₃(x)) = 1 (ρ₅₃ drops 2⁻⁷⁰, leaving a tie that goes
to even), while ρ₂₄(x) = 1 + 2⁻²³. So r_F is defined on exact numbers too, and `r32(0.1)` rounds the
literal's exact value once.

**So there is one float sort**, whose operations are binary64's semimorphism. It is the widest format
every target has natively, and the only number JavaScript has. Every narrower format is a set within it:
- each has its rounding r_F : ℝ → F (r32, r16, rbf16, re4m3, re5m2, re2m1, …);
- a format without ∞ (E4M3, E2M1) has two: r_F, whose overflow is an obligation (§4), and the saturating
  sat_F, a different, total retraction (OCP's "saturation mode");
- a value stored in `Array(F32)` must be proven in F32, an obligation, like a byte stored in
  `Array(0..255)`. Then ρ (ADR 0083) holds it in 4 bytes, by place.

The one thing that would be a sort is an operation *wider* than binary64: binary128's ⊞ is not F64's
restricted. It would be a second float sort, and it is not offered, since only LLVM has it, in
software.

This confirms the survey's §4.3 recommendation ("F32 a set of F64, round₃₂(F64), and its operations F64's
followed by round₃₂"), now with its reasons.

### 2.6 The microscaling formats are representations of tables

An MX block of 32 elements is the pair (s, c) ∈ E8M0 × E^32 with the abstraction function
d(s, c)ᵢ = 2ˢ · cᵢ. That is ADR 0083's (H, d) for a **table** (ADR 0085: a vector is a representation of a
table). **Quantization**, choosing s and rounding each element, is a rounding map on tables:

```
q(x) = (s, ρ_E(x / 2ˢ))        s = ⌊log₂ max |xᵢ|⌋ − e_max(E)         (OCP MX v1.0 §6.3)
```

It is lossy, a retraction of F^32 onto the block's set, and so it is explicit, a function the program
calls. A matrix multiply over MX blocks (the hardware's purpose) is a fold whose partial sums the
hardware keeps in binary32: ADR 0085's fold, with its order fixed by the hardware's specification.

---

## 3. Floats in the language

The construction mirrors ADR 0026, an `Int` is an integer:

| | integers (built) | floats (proposed) |
|---|---|---|
| a value is | an element of ℤ | an element of ℤ[1/2], a real number |
| the language's operations | ℤ's own, exact | binary64's: ρ₆₄ ∘ ℝ's operation, correctly rounded (the semimorphism) |
| a narrower set | a range, a width's `0..2ⁿ-1`; wrapping the residue map `mod 2ⁿ` (ADR 0082) | a format F ⊂ F64 (F32, F16, BF16, E4M3, …), or a range in one; rounding the retraction r_F |
| fusion | a term under `mod 2ⁿ` computed in n-bit arithmetic (ADR 0082) | an operation under r_F computed in F, one at a time (Figueroa) |
| the default | `Int`, unbounded, held in the word when proven | `F64`, binary64 |
| an operation leaving the set | an obligation: proven inside the word, refused, or `-checked` (ADR 0019) | an obligation: proven finite and valid, refused, or `-checked` (§4) |
| a representation | chosen by place, invisible (ADR 0083) | chosen by place, invisible: a value proven in F16 held in 2 bytes, in a typed array or a `half` |
| staging | folds only inside the word (ADR 0009) | folds in binary64, and through each written rounding (ADR 0009, unchanged) |

What the language then owns: `+ − * /`, `sqrt`, `fma`, the comparisons, `min`/`max`, the conversions
(`round`, `floor`, `ceil`, `trunc` to `Int`; an `Int` or a decimal rounded to a format), `abs`, `neg`.
It owns them as Σ_L names on every target, as `& | ⊕` became ℤ's (ADR 0082), and each target realizes
them by spelling. `go.\`f+\`` goes, and `a + b` on two F64s is the language's.

---

## 4. Exceptional values are obligations (wall 5's and 7's half)

Overflow, invalid operation and division by zero are IEEE's three exceptions that produce non-numbers.
The language treats them as it treats integer overflow:

| exception | when | the language |
|---|---|---|
| overflow | \|ρ(x ∘ y)\| exceeds the format's largest finite | an obligation: the result's interval proven inside the format's finite range |
| invalid | ∞ − ∞, 0 × ∞, 0/0, √(negative), a NaN operand | an obligation on the operands: x ≥ 0 for √, the divisor ≠ 0; no ∞ or NaN exists to produce it |
| division by zero | x/0, x ≠ 0 | an obligation: divisor ≠ 0, as for integers |
| underflow | a result below the normal range | **not an exception**: subnormals are values, gradual underflow is IEEE's, and no target flushes to zero |
| inexact | almost every operation | not observable: rounding is the operation's meaning |

**Deciding them** is an interval analysis over floats with outward rounding: the IR's intervals over ℤ̄,
extended to F̄. This is the established technique: Miné, "Relational abstract domains for the detection
of floating-point run-time errors", ESOP 2004; Astrée (Cousot et al., ESOP 2005), which proves the
absence of float overflow and invalid operations in flight software. A program that cannot be proven is
refused on that target, or traps under `-checked`, exactly as for integers.

**The consent to IEEE's specials** is a claim of the extended set, `F̄64` (as a claim above the word
consents to the rung above, ADR 0079): ±∞ and NaN become values, and the program owns their algebra.
NaN ≠ NaN under IEEE comparison, so the extended set separates **identity** (=, reflexive, NaN equals
itself) from **IEEE comparison** (`ieee_eq`, which a NaN fails). In F64 without the extension NaN never
exists, so `==` is the real numbers' equality, reflexive and exact.

**Signed zero**: in ℝ there is one zero. −0 arises in F64 (−1 × 0), and only 1/x, `copysign` and
`atan2` see it. With division by zero an obligation and the other two not offered on the quotient, F64
without the extension is the **quotient** that identifies ±0, and printing prints `0`. Measured: Go
prints `-0`, Java `-0.0`, JavaScript `0`. On `F̄64`, −0 is a value.

---

## 5. Each operation rounds once: what the printers must prevent

The semimorphism holds only if every operation rounds exactly once, to its own format. Three host
behaviours break it, and each printer must prevent them:

| hazard | where | prevented by |
|---|---|---|
| **contraction**: x*y + z fused into one rounding | **Go**: the spec permits it, and the compiler does it on arm64 and on amd64 with GOAMD64=v3 (measured); C with `-ffp-contract=on`; GPUs | Go: `float64(x*y) + z`, the conversion the spec says forces rounding; LLVM: never the `contract` flag; fma only when the program writes `fma` |
| **excess precision**: intermediates kept wider | x87 (80-bit), C's FLT_EVAL_METHOD 2 | not on our targets; the JVM is strict since JDK 17 (JEP 306); LLVM on x86-64 uses SSE |
| **flush-to-zero**: subnormals read or written as zero | SSE's FTZ/DAZ bits, GPU defaults (`-ftz`) | never enabled; CUDA's `-ftz=false` when a GPU target exists |
| **a narrower format through a wider one twice** | JavaScript's binary32 | correct for + − × ÷ √ by Figueroa; fma by an error-free transformation |

Monniaux, "The pitfalls of verifying floating-point computations" (TOPLAS 2008) catalogues these.

---

## 6. Relations (wall 6)

A **relation** R ⊆ X × Y lets an operation give any of several results. IEEE's basic operations are
functions. The relations that reach programs are three:

**6.1 A reordered fold.** A float sum computed in any bracketing is the set of all bracketings' results.
ADR 0085 item 3 keeps the program's order. A program that wants speed and accepts any order is asking for
this relation. The algebraic alternative is exact summation, a function again.

**6.2 A transcendental function.** IEEE 754 *recommends* correct rounding for `exp`, `log`, `sin`,
`pow` and the rest (§9.2), and no common libm delivers it. So a host's `sin(x)` is one element of the
set {y : |y − sin x| ≤ k ulp}, and different hosts pick different elements. Measured (§10):

| | sin | pow |
|---|---|---|
| Go vs JavaScript, results that differ | 50,420 of 200,000 | 199,993 of 200,000 |
| largest error against the true value | Go, Java, StrictMath and JavaScript ≤ 1 ulp | **Go 18,212 ulps**; Java's `Math` 0, correctly rounded on these; `StrictMath` and JavaScript ≤ 1 |

The answer is to make the function a function again: **correctly rounded implementations**. They exist
and are fast now: CRlibm (Daramy-Loirat et al. 2006) and the CORE-MATH project (Sibidanov, Zimmermann and
Glondu, ARITH 2022), correctly rounded binary32 and binary64 functions within a small factor of glibc's
speed. Written in the language or ported as a library, they give one answer on every target. A host's
own function stays available as Tier 2, declared with its accuracy as a claim
(`ensure abs(r − sin(x)) <= ulp(r)`), and a program choosing it is not portable.

**6.3 Stochastic rounding.** Low-precision training rounds up with probability equal to the fraction
(Gupta et al., "Deep learning with limited numerical precision", ICML 2015): a relation, random. It
becomes a function by **taking its randomness as an argument**: SR(x, u) = ⌊x⌋_F if u ≥ frac(x), else
⌈x⌉_F, with u ∈ [0, 1) supplied. That is the standard construction of a relation as a function of an
oracle. The program passes the random bits, and the result is reproducible from them.

So **only deterministic operations are offered**, the survey's recommendation for this wall. Each
relation a program meets has a function form: an exact fold, a correctly rounded function, or an
explicit random input.

---

## 7. The float environment (wall 7)

C's `<fenv.h>` is global mutable state: the rounding mode, the exception flags and the traps. Each read
or write is an effect on every float operation after it, which would make every float operation impure
(ADR 0010). **The algebra replaces the state with a parameter.** Each rounding mode r ∈ {nearest-even,
toward zero, toward +∞, toward −∞, nearest-away} is a different rounding map ρ_r, and so a different
operation, ∘_r = ρ_r ∘ ∘. A static mode is part of the operation's name:

```
lo = add_down(x, y)        hi = add_up(x, y)        // interval arithmetic's outward rounding
```

Realized natively where the hardware takes a mode per instruction: LLVM's constrained intrinsics with a
static rounding argument, AVX-512's embedded rounding. Elsewhere it is realized **exactly by error-free
transformations**. With s = x ⊞ y and e the exact error (TwoSum), the upward sum is s if e ≤ 0 and
nextUp(s) if e > 0, and the other directions likewise (Rump, Ogita and Oishi, "Accurate floating-point
summation", SIAM J. Sci. Comput. 2008, and Rump's interval libraries). So every mode is portable, at a few
operations each.

**The flags** are replaced by §4's obligations: an overflow or invalid operation is proven absent or
refused, so there is nothing to flag. Inexact is the meaning of rounding. **Traps** are never enabled.
**FTZ and DAZ** are never set (§5).

---

## 8. Decimals

### 8.1 The algebra: ℤ[1/10], a ring

The decimals are the numbers with a finite decimal expansion:

```
Dec = ℤ[1/10] = { m · 10⁻ᵏ : m ∈ ℤ, k ∈ ℕ } = ℤ[1/2, 1/5]
```

the localization of ℤ at 10. It is a **ring**: closed under +, − and ·, with exact results:
- **sum**: m·10⁻ᵏ + n·10⁻ʲ = (m·10^(j−k) + n)·10⁻ʲ for k ≤ j;
- **product**: (m·10⁻ᵏ)(n·10⁻ʲ) = mn·10^−(k+j).

**Division is not closed**: 1/3 ∉ Dec. So a decimal division states its result's scale and rounding,
`div(a, b, scale, mode)`, or leaves Dec for ℚ. And ℤ ⊂ Dec ⊂ ℚ, while Dec ∩ F64 holds only the dyadic
decimals (0.5, 0.25, 0.375): 0.1 ∈ Dec, 0.1 ∉ F64.

So **a decimal is a number in the same sense an integer is**, and its arithmetic is exact. That is the
criterion for a language construct (CLAUDE.md, "spec first"): it means one thing independently of any
target, and every target can realize it. **Decimals belong in the language, like ℤ**, not in a library.

**Not a new sort: a larger set of the same one.** ℤ's +, − and · are Dec's restricted, since ℤ is a
subring. So the integer sort becomes the exact sort ℤ[1/10]:
- `Int` stays the set it is, a subset, and `Dec` is the whole sort;
- every integer program means what it meant, since no operation on integers changes;
- **`/` and `mod` keep ℤ's meaning**, truncating division and the floor remainder, with "both operands
  are integers" one more obligation on them;
- a decimal's division is a different function and is written so, `div(a, b, k, r)`: the quotient
  rounded by r to k places.

What it costs: the IR's facts gain a **scale**, the least k with v·10ᵏ ∈ ℤ, next to an interval. An
interval over ℤ̄ becomes one over Dec̄, and representation chooses a word when both the scaled
interval and k are bounded. A fact of the same kind as a bit length (ADR 0029), computed by the same
lattice: scale(a ± b) = max, scale(a · b) = sum.

### 8.2 The facts choose its representation, as for integers

A decimal is a pair (m, k), an integer and a scale. When the program's facts bound the scale (money to
the cent) and the magnitude, it is **an integer in disguise**: m in the machine word, k a compile-time
constant. Its sum is a word addition and its product a word product with the scales added, each
proven inside the word as ADR 0019 requires. Unbounded, it is the host's big decimal:
- Java's `BigDecimal`;
- Go's `math/big.Int` with a scale;
- JavaScript's `BigInt` with a scale.

This is ADR 0079's mechanism exactly: a declared set is a claim, and the facts choose the
representation. Measured (§10): a million two-place amounts summed as scaled `long`s take 0.39 ms, as
`BigDecimal`s 8.61 ms (22×), both exact; as `double`, 0.52 ms and **wrong in the cents**.

**Decimal floating point** (IEEE 754-2008's decimal64 and decimal128, Cowlishaw's General Decimal
Arithmetic) is a different thing: a *rounded* decimal format, a semimorphism onto a finite set of
decimals. It has its uses (fixed-width storage, Java's `MathContext.DECIMAL64`), and it is a format
like §2's, available later as one. The exact ring is what decimals are, and it comes first.

---

## 9. Literals

**A literal is an exact number.** `3` is an integer, and `0.1`, `1.5e-3` and `6.02e23` are decimals:
elements of ℤ[1/10], since a decimal numeral with an exponent denotes m·10ᵏ exactly. What it becomes
depends on the **sort** it meets: the exact numbers (§8.1) or the floats. A sort is the type checker's
(ADR 0049). A *set* within a sort never changes the value, only what must be proven of it:

| where the literal is | `0.1` means |
|---|---|
| the exact sort: no float near it, or a binder `∈ Dec` | 1/10 exactly |
| the float sort: an operand of a float operation, or a binder `∈ F64` | ρ₆₄(1/10), rounded to nearest-even **once, at compile time**, the same bits on every target |
| a narrower format's rounding, `r32(0.1)` | ρ₂₄(1/10), rounded once from the exact value (§2.5: never through binary64) |
| a claim in the float sort, `x ∈ F32 = 0.1` | ρ₆₄(1/10), and the claim is refused: that value is not in F32. Write `r32(0.1)` |
| a claim in the exact sort, `x ∈ Int = 0.1` | refused, as `x ∈ 0..9 = 12` is; `2.0` is the integer 2 |

So **there is a difference between a decimal literal and a float, and it lies in the context, not the
spelling**:
- `x ∈ F64 = 0.1` is the binary64 nearest 1/10.
- `d = 0.1` is one tenth.
- `0.1 + 0.2 == 0.3` holds, because the arithmetic is the decimals' and exact.
- `a + b` with a, b ∈ F64 bound to 0.1 and 0.2 is 0.30000000000000004 on every target and at every
  stage. ADR 0009 is unchanged, because the rounding happens where the literal meets F64, once,
  identically at compile time and run time.

**A value crosses into a float only by an explicit rounding**, `to_f64(d)`, because rounding loses
information, and a decimal or an integer above 2⁵³ is not a float. A literal is the exception: its
rounding is what writing a decimal where a float is expected means in every language, and the
compiler does it exactly (Clinger, "How to read floating point numbers accurately", PLDI 1990).

**Why not a separate spelling** (C#'s `0.1m`, a suffix `d`): the number is the same, and the context
already says which set it lies in. A suffix would be a second spelling of one literal, which
fit-research counts as a misfit.

**Why not Go's exact constant expressions for floats** (Go folds `0.1 + 0.2` exactly, then rounds once):
that is the folding ADR 0009 rejected. Here the exactness belongs to the decimals, where the
arithmetic is exact at run time too, so folding cannot change an answer.

---

## 10. Measured

| what | result |
|---|---|
| **Figueroa**: binary32's + − × ÷ √ through binary64, 20,000,000 random triples (finite operands) | **0** disagreements; fma through binary64: **22** |
| **Contraction**: Go's `x*y + z` | amd64 v1: `MULSD`, `ADDSD`; **amd64 v3: `VFMADD231SD`**; **arm64: `FMADDD`**; `float64(x*y) + z`: never fused |
| **Printing** the same nine doubles | 1e23: Go `1e+23`, JDK 17 `9.999999999999999E22`, JavaScript `1e+23`; −0: `-0`, `-0.0`, `0`; 2.82879384806159e17: `2.82879384806159e+17`, `2.82879384806159008E17`, `282879384806159000`; ∞: `+Inf`, `Infinity`, `Infinity`; `0.1 + 0.2` as written: Go `0.3` (folded exactly), the others `0.30000000000000004` |
| **sin, exp, log, pow** on 200,000 arguments: results that differ | Go vs JavaScript: 50,420 / 3,069 / 186 / 199,993; JavaScript vs `StrictMath`: 0 / 0 / 0 / 19,266; Java's `Math` vs `StrictMath`: 6,778 / 1,822 / 1,985 / 19,201 |
| **Accuracy against a 60-digit reference** (2,062 samples) | sin: every host ≤ 1 ulp (Go not correctly rounded in 518, Java's `Math` in 3); pow(1.0001, y): **Go up to 18,212 ulps** (relative error 2·10⁻¹² at y = 97000), Java's `Math` correctly rounded in all, `StrictMath` and JavaScript ≤ 1 ulp |
| **Decimals**: 10⁶ two-place amounts summed on the JVM | scaled `long` 0.39 ms exact; `BigDecimal` 8.61 ms exact; `double` 0.52 ms, **4998789092.6099790** for 4998789092.61 |
| **Formats in the hosts** | Node 26: `Float16Array`, `Math.f16round`; JDK 17: no binary16; Go 1.27: no binary16 or bfloat16; LLVM 23.1.4: IR types `half bfloat float double fp128 x86_fp80`, no 8-bit type, 12 small formats in APFloat, 97 FP8 intrinsics on AArch64 |

---

## 11. Decisions for hamza

**F1. Floats are the language's.** A float is a real number, an element of a format's finite set
F ⊂ ℤ[1/2]. The language owns +, −, ×, ÷, √, fma, the comparisons, min, max, abs and the conversions,
each defined as IEEE 754's correctly rounded operation, round-to-nearest-even (Kulisch's semimorphism):
one meaning, every target, as Σ_L names. The targets realize them by spelling, and `go.\`f+\``
and its siblings retire. **Recommended.**

**F2. One float sort, binary64's; a narrower format is a set within it, its rounding a function**
(§2.5; ADR 0049 and ADR 0082 decide it).
- The sets are binary32, binary16 and bfloat16, and the ML formats: FP8 E4M3 and E5M2 (OCP), FP6 E2M3
  and E3M2, FP4 E2M1, each with its rounding r_F.
- A format without ∞ also gets the saturating sat_F.
- A value stored in a table of a format is proven in it.
- FNUZ variants are added when a program needs one. Binary128, a wider operation and so a second sort,
  is not offered.
- **Why not a format as a sort** (C, Java, Mojo): a claim would change an answer.

**Recommended.**

**F3. Every format is Tier 1, and `r_F(x ∘ y)` is fused into F's native operation** (Figueroa:
measured, 0 disagreements in 20 million). The rule is ADR 0082's fusion for one operation at a time,
with a planted fault for a compound term fused.
- Native where the host has the format: Go's `float32`, Java's `float`, JavaScript's `Math.fround` and
  `Math.f16round`, LLVM's types and its FP8 intrinsics.
- Elsewhere: binary64's operation and the rounding function, written once in the language.
- A narrow fma is computed by an error-free transformation.

**Recommended.**

**F4. The microscaling block formats are representations of tables** (ADR 0083, 0085): a block of 32 (or
16) elements and a shared scale. Quantization is an explicit rounding of a table, and a block's
arithmetic is a fold whose order the format's specification fixes. Built when a program needs one.
**Recommended.**

**F5. Overflow, invalid operations and division by zero are obligations** (ADR 0019 for floats):
- each is proven absent by a float interval analysis on the IR, refused, or trapped under `-checked`;
- ±∞ and NaN exist only in the extended set `F̄64`, claimed when a program wants IEEE's specials, with
  identity (reflexive `=`) separate from IEEE comparison;
- in F64, ±0 are one zero, and print `0`;
- subnormals are values.

**Recommended.**

**F6. Each operation rounds once.** No contraction: Go prints `float64(x*y)`, and LLVM never gets
`contract`. No flush-to-zero and no excess precision. `fma` only where the program writes it.
**Recommended.**

**F7. Only deterministic operations are offered (wall 6).**
- A reordered fold stays ADR 0085's, and is offered later as exact summation.
- Transcendental functions are the language's own and correctly rounded (CORE-MATH, ported), so
  identical everywhere. A host's is Tier 2, with its accuracy as a claim (measured: Go's `pow` misses
  by 18,212 ulps).
- Stochastic rounding takes its random bits as an argument.

**Recommended.**

**F8. No float environment (wall 7).** A rounding mode is a static parameter of an operation
(`add_up`, `div_toward_zero`): native where the hardware takes one, by error-free transformations
elsewhere. No observable flags, no traps, no dynamic mode. **Recommended.**

**F9. Decimals are the language's, the ring ℤ[1/10], like ℤ.**
- Not a new sort: the integer sort becomes the exact sort ℤ[1/10], with `Int` a subset of it.
- +, − and · are exact. `/` and `mod` keep ℤ's meaning, with integer operands obligated; a decimal's
  division states its places and rounding, `div(a, b, k, r)`.
- The IR's facts gain a scale beside the interval.
- Its representation is chosen by the facts: a scaled machine word when scale and magnitude are
  bounded, the host's big decimal otherwise (ADR 0079's mechanism; measured 22×).
- Decimal floating point (decimal64 and decimal128) is a format for later.

**Recommended.**

**F10. A literal is an exact number, and the sort it meets decides it.**
- `0.1` is 1/10.
- In the float sort it is rounded once, to nearest-even, at compile time, identically everywhere.
- Under a narrower rounding (`r32(0.1)`) it is rounded once from its exact value.
- In the exact sort it is the exact decimal, so `0.1 + 0.2 == 0.3`.
- A claim never rounds it: `x ∈ F32 = 0.1` is refused.
- A decimal or integer *value* becomes a float only by an explicit rounding.
- No separate decimal spelling.

ADR 0009 is unchanged. **Recommended.**

**F11. One rendering and one reading per format**: the shortest string that reads back to the same
value (Steele and White, PLDI 1990; Ryu, Adams, PLDI 2018; Schubfach, Giulietti 2020), and reading
correctly rounded (Clinger 1990). The same on every target: measured, the three hosts print the same
values three ways. It joins printing's P3 (printing-research.md), which owns rendering. **Recommended.**

## 12. Not here

- **ℚ**, exact rationals: a set the decimals sit inside. It is unneeded until a program divides
  exactly.
- **Interval arithmetic as a type**: §7's directed operations make it a library.
- **Posits** (Gustafson and Yonemoto 2017): no target has them.
- **Complex numbers**: a product of two floats, a library.
- **x87's 80-bit and IBM's double-double**: Tier 2 if ever.
- **Error analysis as claims**: bounds on a result's error, as Fluctuat, FPTaylor (Solovyev et al.,
  FM 2015) and Herbie (Panchekha et al., PLDI 2015) compute them. That is the logic's later work, once
  floats are the language's.

---

# Part II: the number without its format

2026-10-31. hamza, on Part I: "by choosing one sort binary64 we are kind of choosing the representation
as a sort … could we decouple them, and use mathematical numbers just like we did with integers. in this
case, what we care about is a range and a precision, resolution, uncertainty, error … let's reach for
the literature, and see if anyone did it before. if not, let's follow the math and see where we land."

**Where it lands: decoupling is possible, and it is the better design.**
- **One numeric sort, the ring ℤ[1/10]**, holds every integer, every decimal and every binary float.
  `+`, `−`, `·`, `=` and `<` are exact on it: one `+` for every number in the language, ℤ's.
- **A rounding is the only operation that loses information.** It is a retraction r_S onto a set S, and
  S is mathematical: a range and a resolution. S may be F(p, e_min, e_max) for *any* p, a decimal grid
  D(k), a fixed-point grid, or ℤ itself.
- **Division, √ and the transcendentals leave the ring**, and so they are only written under a rounding.
  Integer `/` turns out to be one such rounding: r_ℤ toward zero of a ÷ b.
- **The facts choose the representation**, as for integers. A value proven in F(53, …) is held in a
  double; an exact a·b + c in a pair; an unrounded float sum in an exact accumulator.
- **Every answer is a function of the inputs**, independent of the representation and of how strong the
  compiler's proofs are, so ADR 0009 and ADR 0049 hold with no exception.
- **Part I's design is this one with a rounding written at every operation.** That is IEEE's arithmetic,
  kept as a special case.

Nobody we found has done the whole of it. Pieces exist, in Kulisch's exact dot product and
PASCAL-XSC's exact expressions, in Shewchuk's exact predicates, and in Scheme's exact numbers. Two other
lines of work decoupled precision and paid with determinism: Ada's model numbers, and Rosa and Daisy's
compilation of reals.

## 13. What the literature did

| line of work | what it decouples | what it costs, or why it stops |
|---|---|---|
| **Exact real arithmetic**: Boehm's constructive reals (1986; the Android calculator, Boehm, PLDI 2020); iRRAM (Müller 2001); RealPCF (Escardó 1996); Type-2 computability (Weihrauch 2000) | everything: a real is a program producing approximations on demand | **Theorem C** (Grzegorczyk 1957, Ceitin 1959): every computable function on ℝ is continuous. So `x < y` and `x == y` do not terminate when x = y, rounding and `floor` are not computable, and printing is multivalued. iRRAM's comparisons are partial, and Boehm's calculator shows approximations. Values are lazy closures on the heap, orders of magnitude slower |
| **Declared precision**: Ada 83, `type T is digits 6 range -1.0 .. 1.0`, with W. S. Brown's model numbers (TOMS 1981); Fortran 90's `selected_real_kind(p, r)`; PL/I's `FLOAT DECIMAL(p)`; COBOL's `PICTURE` | the program states a precision and a range, and the compiler picks the format | **Ada**: an operation's result is any number in the *model interval*, a relation, so two conforming compilers may print different answers. In practice every compiler picks IEEE single or double, and programs rely on that. **Fortran**: the kind chosen is a format, and from then on the semantics is that format's, so (p, r) only names it |
| **Compiling reals to an error bound**: Rosa (Darulova and Kuncak, POPL 2014), "Towards a compiler for reals" (Darulova and Kuncak, TOPLAS 2017), Daisy (Darulova et al., TACAS 2018), FPTuner (Chiang et al., POPL 2017), Precimonious (Rubio-González et al., SC 2013), Herbie (Panchekha et al., PLDI 2015) | the program is written over ℝ with an error bound; the tool chooses each operation's precision, or rewrites it | the semantics is a relation, any answer within ε. **The printed answer depends on the tuner**: a better analysis chooses cheaper precisions and prints different bits, the opposite of ADR 0009 |
| **Exact expressions, rounded once**: Kulisch's exact dot product (Kulisch and Miranker 1981; *Computer Arithmetic and Validity*, 2013); PASCAL-XSC's `#( … )` expressions (Klatte et al. 1991) and ACRITH-XSC; IEEE 1788-2015's exact dot product; Neal's superaccumulators (2015); ExBLAS (Iakymchuk et al. 2015); ReproBLAS | inside the expression nothing is rounded; the one rounding is at its end | **Restricted to dot products and sums.** Deterministic, reproducible, and near memory speed for long sums |
| **Exact predicates**: Shewchuk, "Adaptive precision floating-point arithmetic and fast robust geometric predicates" (1997) | a sign of a polynomial in doubles, decided exactly: a fast path with an error bound, exact expansions when it fails | only signs, in a library |
| **Exact and inexact numbers**: Scheme (R7RS), Common Lisp's rationals | exactness is a property of the value | the inexact numbers are IEEE's, chosen by the value, dynamically |
| **Run-time uncertainty**: ball arithmetic, Arb (Johansson 2017); MPFI; Mathematica's significance arithmetic (Sofroniou and Spaletta 2005) | a value carries its error | the value *is* a set, decided at run time; the algebra is a library's, not the language's |
| **Round to odd** (Boldo and Melquiond, IEEE TC 2008) | any precision p from a wider one | the tool that makes any F(p, …) with p ≤ 51 exact through binary64 |

Each line meets one of the three theorems below. Exact reals meet Theorem C. Declared precision and
compiling to an error bound give up determinism. The exact expressions stop at dot products, but they
show where a construction can stand.

## 14. The algebra

### 14.1 Theorem R: the sort is ℤ[1/10]

**ℤ[1/10] is the least subring of ℚ that contains ℤ, every binary float and every decimal numeral.**
*Proof.* It contains 1/2 (a float) and 1/10 (a numeral), hence 1/5 = 2 · (1/10), and the ring generated
by ℤ, 1/2 and 1/5 is ℤ[1/2, 1/5] = ℤ[1/10]. Conversely, every finite float is m·2ᵉ = m·5⁻ᵉ·10ᵉ for
e < 0, and every numeral is m·10ᵏ. ∎

It is closed under +, − and ·, and **=** and **<** are decidable on it: an element is a pair (m, k)
compared by cross-multiplying. So it is to the numbers what ℤ was to the integers. The sets a program
names are its subsets:

| set | elements | range | resolution |
|---|---|---|---|
| `Int`, `lo..hi` | m | lo..hi | 1 |
| F(p, e_min, e_max), any p ≥ 1: binary64 is F(53, −1022, 1023), FP4 E2M1 is F(2, 0, 2) | ±m·2^(e−p+1), m < 2ᵖ | ±(2 − 2^(1−p))·2^e_max | relative, 2^(1−p); absolute 2^(e_min−p+1) at the bottom |
| D(k, lo..hi), decimal places: cents are D(2, …) | m·10⁻ᵏ | lo..hi | 10⁻ᵏ |
| Q(k, lo..hi), binary fixed point (DSP, Q15) | m·2⁻ᵏ | lo..hi | 2⁻ᵏ |

hamza's "a range and a precision, resolution" is exactly this: **a set is its range and its resolution**,
and binary64 is one such set, as `0..255` is one range.

### 14.2 Theorem C: what leaves the ring must be rounded back

÷ leaves the ring for ℚ; √, exp, log, sin and pow leave it for ℝ. By Theorem C, a real-valued result
cannot in general be compared, printed or branched on. The way back into the ring is a **rounding**: a
monotone retraction r_S,m : ℝ → S onto a discrete closed set S, with a mode m (nearest-even, toward 0,
toward ±∞, away, saturating; stochastic with its random bits, §6.3). A rounding is discontinuous at S's
breakpoints:
- the midpoints, for nearest;
- the points of S themselves, for the directed modes.

That is the only place a computation can fail to decide.

**Integer division is already one**: today's `a / b` is r_ℤ,toward 0(a ÷ b), and `mod` is
a − b·r_ℤ,floor(a ÷ b). They keep their meaning, extended to the whole ring.

### 14.3 Theorem T: one non-closed operation per rounding terminates

Let t, t′ be terms of the ring (+, −, ·, literals, variables), so their values are exact, and let f be
one non-closed operation. Then **r_S(f(t)) is computable, and so is r_S(t ÷ t′)**:
- **÷**: t / t′ is rational, and its position against S's breakpoints is decided exactly.
- **√**: √t is either rational, which is decided by an integer square root, or irrational and so never a
  breakpoint.
- **exp, log, sin, cos, tan and their inverses**: by Lindemann–Weierstrass, e^a is transcendental for
  algebraic a ≠ 0. So exp(t), log(t) (t ≠ 1), sin(t) (t ≠ 0) and the rest are transcendental at every
  point of the ring other than the trivial ones, and never a breakpoint, which is rational. Ziv's
  iteration (1991), evaluating at increasing precision until the rounding is determined, terminates.
- **pow(x, y)** with y = m/2ʲ rational: x^y is algebraic, and whether it is a breakpoint is decided by
  extracting an integer root. That is the exact-case test CORE-MATH runs.

**Two non-closed operations under one rounding** (r(√a + √b), r(exp(a) − exp(b))) need a zero test on an
expression. It is decidable for the algebraic numbers, at a cost growing with the degree; for exp and
log, decidable only if Schanuel's conjecture holds (Richardson 1997); and undecidable once sin and the
whole line are allowed (Richardson 1968). **So the rule is: one non-closed operation at the root of each
rounding**, with inner ones rounded first. The cost is bounded for the formats whose worst cases are
known: binary32 and binary64, by Lefèvre and Muller's searches (2001), which CORE-MATH uses. For an
arbitrary p it is bounded in practice, by Ziv's loop, but not proved.

### 14.4 Theorem D: every answer is a function of the inputs

Every operation of the computation is now one of three things:
- **exact**, because it is the ring's;
- **a rounding of an exact ring term**, which is a function of that term's value;
- **a rounding of one non-closed operation**, which is a function by Theorem T.

So the program's answer is a function of its inputs. Neither ρ, nor the target, nor the strength of
the compiler's proofs enters it. **ADR 0009 holds with no float exception**: staging folds exactly, and
a rounding folded at compile time is the same function. **ADR 0049 holds**: no set changes an answer,
since a set only states a claim, and a rounding is an operation the program wrote. This is what
Part I's F2 could not give: there, `+` on floats was a second operation, ⊞₆₄.

### 14.5 Representation is the facts', as for integers

An exact value v = m·2ᵃ·5ᵇ has two facts: its interval and its **lowest digit**, the valuations
v₂(v) and v₅(v). They give the float's ulp and the decimal's scale, and are computed by the same lattice
as bit lengths:
- v(a ± b) ≥ min(v(a), v(b));
- v(a · b) = v(a) + v(b);
- the interval as today.

ρ then chooses, cheapest first:

| proven fact | representation |
|---|---|
| an integer inside the word | the word (today) |
| in F(53, −1022, 1023) | a double |
| in F(p, …), p ≤ 24, exponent inside binary32's | a float, or a double holding it (JavaScript) |
| an exact a·b, or a·b + c, of doubles | a pair or an expansion of doubles: TwoProduct, TwoSum (Dekker 1971; Shewchuk 1997) |
| an unrounded sum of n doubles | an exact accumulator, fixed size for any n below 2⁶⁴ (Kulisch; Neal 2015) |
| a decimal with bounded k and range | a scaled word (Part I §8.2) |
| beyond | limbs or the host's big number, by consent (ADR 0079), or refused (ADR 0019) |

Each rounding is realized as cheaply as its term allows:
- **Fusion** (Part I §2.5, Figueroa): r64(a + b) on doubles is the double `+`; r32(x · y) on floats is
  binary32's; **r64(a · b + c) is `fma`**, one rounding of an exact term, native on amd64 v3 and arm64.
- **Round to odd** (Boldo and Melquiond): any p ≤ 51 within binary64's range, exactly.
- **Error-free transformations** for a short term (Boldo and Melquiond's correctly rounded sum of three).
- **The accumulator** for a fold.
- **CORE-MATH** for a transcendental.

### 14.6 Consequences that come free

- **A float fold becomes associative.** An unrounded sum is exact, so ADR 0085's homomorphism lemma
  applies: it vectorizes and parallelizes, and its answer does not depend on the order. That settles
  ADR 0085's item 3 without a relation. **Measured** (`experiments/floats/exactsum`, 2²⁴ doubles across
  60 binary orders of magnitude):

  | sum | time | answer |
  |---|---:|---|
  | naive, the written order | 0.65 ns per element | 264327992223.14578 |
  | naive, sorted | | 264327992231.0256 |
  | four partial sums | 0.51 ns per element | 264327992223.1665 |
  | exact accumulator, rounded once | 1.92 ns per element (**2.96×**, scalar, not vectorized) | 264327992223.16898, the same in every order, equal to a 2400-bit `big.Float` |

- **Comparisons are exact**: `if a*b - c*d > 0` decides the true sign. Robust geometric predicates are
  what ρ realizes, with Shewchuk's fast path when its error bound excludes zero.
- **Printing is exact and finite**, since every element of ℤ[1/10] has a finite decimal expansion.
  Shortest round-trip printing (Part I's F11) is a function render_F of a value and the set it was
  rounded to, and stays a choice for output.
- **Literals need no rule**: `0.1` is an element of the ring, and nothing rounds it unless the program
  writes a rounding. Part I's §9 table collapses to one line.
- **No exceptional values**: no ∞, no NaN and no −0 exist in the ring. A rounding's overflow is its
  obligation (or the program uses sat_F), and ÷'s divisor ≠ 0 and √'s operand ≥ 0 are obligations. A
  host's double enters the ring by a partial decode, with NaN and ±∞ its failure, as `go.text` decodes a
  host string.

### 14.7 Error and uncertainty are claims, never choosers

hamza's "uncertainty, error": the accuracy of a result against the real expression the program means is
a claim in the logic, `ensure abs(r − e) <= ε`, with e an expression over ℝ that never prints. It is
proven by an error analysis (Higham's bounds, FPTaylor's symbolic Taylor forms, Daisy's) or refused.
**It never chooses a precision**, because then the answer would depend on the analysis: Rosa's and
Daisy's semantics, rejected in §13. A tool may *propose* the cheapest roundings that satisfy the claim,
as FPTuner does. The program then contains the roundings, and the compiler checks them. Uncertainty
carried at run time, a value with its radius, is ball arithmetic, a library over the ring.

### 14.8 IEEE's arithmetic is a special case, written as a scope

Bit-for-bit compatibility with C, Go or Java float code is a rounding at every operation:
`r64(r64(a * b) + c)`. A static, lexical scope says it once: `round(F64) { … }`. D rewrites each
arithmetic operation and literal inside it to its rounding, so nothing is dynamic, and wall 7 stays
answered. The spelling is a surface question for hamza. Without the scope, a program's interior is
exact and its roundings are where it wrote them.

## 15. Part I's decisions, revised

| | Part I | Part II |
|---|---|---|
| F1 the operations | the language's, IEEE's correctly rounded binary64 | the ring's exact +, −, ·; IEEE's operations are roundings of them |
| F2 the sort | one float sort, binary64's; narrower formats sets in it | **one numeric sort, ℤ[1/10]**; every format, decimal grid and fixed point a set, F(p, …) for any p |
| F3 realization | Figueroa's fusion, one operation at a time | the same, plus round to odd, error-free transformations and the accumulator |
| F4 microscaling | representations of tables | unchanged |
| F5 exceptional values | obligations; F̄64 for IEEE's specials | none exist; the rounding's overflow and the domains of ÷ and √ are obligations; host specials enter by a partial decode |
| F6 contraction | forbidden | a different term: r64(a·b + c) *is* fma, r64(r64(a·b) + c) is not, and the printer keeps each |
| F7 relations | deterministic only | the same, now a theorem (D); transcendentals are non-closed operations under a rounding (T) |
| F8 rounding modes | a static parameter | a rounding's mode |
| F9 decimals | ℤ[1/10], the integer sort enlarged | the same sort, which now holds the floats too |
| F10 literals | exact, rounded where they meet the float sort | exact; rounded only by a written rounding |
| F11 rendering | one per format | exact printing, plus render_F |

## 16. What it costs, and what is not known

- **Writing IEEE code is longer** without the scope: every rounding is visible. With the scope it is as
  short as today's.
- **An exact interior can be slower than a rounded one**. An unrounded sum costs 2.96×, measured
  unvectorized; an expansion costs a few operations per term; JavaScript has no fma, so an exact a·b
  needs Veltkamp's splitting, about 17 operations. Not measured: the cost on a real program, and the
  accumulator vectorized.
- **An unrounded value in a loop must be bounded at both ends**, as an integer must. The magnitude needs
  a trip bound, and the lowest digit is a new fact. A product carried around a loop unrounded
  (xₙ₊₁ = xₙ²) doubles its bits each time and is refused, as an integer squaring would be.
- **Theorem T's cost for an arbitrary p is bounded in practice, not in proof.** For binary32 and binary64
  the worst cases are known.
- **The type checker's numeric sort becomes ℤ[1/10]**. Integer programs are unchanged, since ℤ is a
  subring and `/` keeps its meaning. Every rule over ℤ (intervals, Fourier–Motzkin, the Omega test)
  stays sound for the integer subset; over the ring, linear arithmetic over ℚ is the decidable theory
  (Fourier–Motzkin is complete there, Tarski for the reals).
- **A host's floats enter through a decode** (NaN, ±∞ and −0 are its failures or its quotient), and every
  host signature with a `float64` names a set, F64, not a sort.

## 17. Decisions for hamza (Part II), recommended over F1, F2, F5, F6 and F10

**G1. One numeric sort, ℤ[1/10].** Integers, decimals and binary floats are its elements, and `Int`, the
ranges, F(p, e_min, e_max) for any p, D(k, …) and Q(k, …) are sets in it. +, −, ·, = and < are exact.
One `+` for every number. **Recommended** (Theorem R, Theorem D).

**G2. A rounding r_S,m is the only lossy operation, a function onto a set with a mode.** ÷, √ and the
transcendentals are written under one, one non-closed operation per rounding (Theorem T). Integer `/` and
`mod` are roundings and keep their meaning. **Recommended.**

**G3. The facts choose the representation**: an interval and a lowest digit, the ladder of §14.5,
unbounded values refused or consented to. **Recommended.**

**G4. A rounding is realized as cheaply as its term allows**: native fusion where the term is one host
operation (fma included), round to odd, error-free transformations, the accumulator, CORE-MATH. The
planted faults:
- a compound term printed as per-operation native code;
- a double rounding through binary64 for p > 25 without round to odd;
- r64(r64(a·b) + c) contracted into fma.

**Recommended.**

**G5. Accuracy is a claim, proven or refused, never a chooser of precision**; tuning is a tool's.
**Recommended.**

**G6. IEEE's per-operation arithmetic is a static scope**, `round(F64) { … }` or a spelling hamza
chooses. **For hamza: the spelling.**

**G7. No exceptional values in the ring.** Overflow is a rounding's obligation, ÷ and √ have domains, and
a host's double is decoded. **Recommended.**

**So: decoupling does not confirm Part I.** It improves it, and Part I survives as the case "a rounding
at every operation".


# Part III: the surface, sketched

2026-11-01. hamza, on Part II: "show me the surface, how would it look like in programs? examples. and
how would we declare target primitives examples of llvm, go? does it put any restriction on exploiting
targets fully when someone reach for them?" This part writes Part II in J, in programs (§18) and in target
declarations (§19), and answers the third question from the declarations (§20). Nothing in it is built.
Its spellings are proposals, and the four that are open are in §21.

## 18. Programs

### 18.1 What a program writes

| written | meaning | where it comes from |
|---|---|---|
| `3`, `0.1`, `1e-3`, `0x1.8p3` | an element of ℤ[1/10], exactly | Theorem R; a literal is never rounded |
| `+ - * == < <=` | the ring's, exact | G1 |
| `a / b`, `a % b` | the integer quotient and remainder, as today: r_ℤ,zero(a ÷ b) | §14.2 |
| `a ÷ b` (ASCII `\div`), `sqrt(x)`, `exp`, `log`, `sin`, `cos`, `atan2`, `pow` | the real value, which never prints. A computation may write one only at the root of a rounding; a claim may write it anywhere | Theorem C, Theorem T, ADR 0049 |
| `round(S, e)`, `round(S, e, mode)` | r_S,m(e), the one lossy operation; S a set, static, and the mode a tag: `'nearest` (even, the default), `'away`, `'zero`, `'up`, `'down`, `'sat`, `'stochastic(bits)` | G2 |
| `round(S) { … }` | every operation and literal inside rounded to S, IEEE's arithmetic | G6 |
| `F64`, `F32`, `BF16`, `E4M3`, `D(2, R)`, `Q(15, R)` | sets, defined in `lib/num` (§18.2) | §14.1 |
| `string_of(x)` | x's exact decimal expansion, finite | §14.6 |
| `render(S, x)` | the shortest decimal that rounds back to x in S (Ryu, Schubfach) | F11 |

The sets are ordinary set definitions (ADR 0066), written once in a library and not in the compiler:

```
// lib/num.oro
pub set F(p ∈ 1.., lo ∈ Int, hi ∈ Int) = { m * 2^(e - p + 1) : m ∈ -(2^p - 1)..2^p - 1, e ∈ lo..hi }
pub set F64  = F(53, -1022, 1023)
pub set F32  = F(24, -126, 127)
pub set F16  = F(11, -14, 15)
pub set BF16 = F(8, -126, 127)
pub set E4M3 = F(4, -6, 8) ∩ -448..448     // OCP FP8: the top code is NaN, so 448 and not 480
pub set E5M2 = F(3, -14, 15)
pub set E2M1 = F(2, 0, 2)                  // FP4: ±{0, 0.5, 1, 1.5, 2, 3, 4, 6}
pub set D(k ∈ Nat, R) = { x ∈ R : x * 10^k ∈ Int }   // k decimal places
pub set Q(k ∈ Nat, R) = { x ∈ R : x * 2^k ∈ Int }    // binary fixed point
```

F(p, lo, hi) holds the subnormals too: they are the elements with e = lo and a small m.

### 18.2 Money: decimals, with no float in sight

```
use num
set Price = num.D(2, 0..10^9)
set Total = num.D(2, 0..10^16)

fn total(p ∈ Array(Price), q ∈ Array(0..1000)) (Total) where len(q) == len(p) && len(p) <= 10000 {
    loop i = 0, t = 0 {
        i < len(p) -> again(i + 1, t + p[i] * q[i])
        else -> t
    }
}

fn with_tax(t ∈ Total) (Total) { round(Total, t * 1.0825, 'away) }   // to the cent, half away from zero
```

- `t + p[i] * q[i]` is exact, and its facts are an interval and a lowest digit of 10⁻². ρ holds it as
  a word counting cents, which is §10's scaled `long`, 0.39 ms against `BigDecimal`'s 8.61.
- `t * 1.0825` has a lowest digit of 10⁻⁶. The rounding is the program's, and so is the mode.
- `render(Total, x)` prints `1234.50`. `string_of(x)` prints `1234.5`.

### 18.3 A dot product, exact inside and rounded once

```
fn dot(a ∈ Array(F64), b ∈ Array(F64)) (F64) where len(a) == len(b) {
    round(F64, loop i = 0, s = 0 {
        i < len(a) -> again(i + 1, s + a[i] * b[i])
        else -> s
    })
}
```

- `s` is never rounded, so the answer is the correctly rounded dot product: the same on every target,
  in every order, vectorized or not.
- ρ holds `s` in an exact accumulator. That is Kulisch's long accumulator: each product's two halves come
  from TwoProduct by `fma` and go into the accumulator's bins.
- **Not measured**: the cost against a naive loop. §14.6 measured a sum at 2.96×. A dot product adds the
  TwoProduct.
- The obligation that remains: the rounding's argument must lie inside F64's range. That holds when
  each |aᵢ·bᵢ| is bounded and n is a trip bound, as for an integer sum. Otherwise, as today: narrow the
  set (`Array(F64 ∩ -1e100..1e100)`), saturate (`'sat`), or `-checked`.

### 18.4 The same dot product, as Go and C compute it

```
fn dot_ieee(a ∈ Array(F64 ∩ -1e100..1e100), b ∈ Array(F64 ∩ -1e100..1e100)) (F64) where len(a) == len(b) {
    round(F64) {
        loop i = 0, s = 0 {
            i < len(a) -> again(i + 1, s + a[i] * b[i])
            else -> s
        }
    }
}
```

- D rewrites the body to `round(F64, s + round(F64, a[i] * b[i]))`. Each rounding is one host
  operation (fusion, Figueroa), so the printed Go is `s += float64(a[i] * b[i])`. The conversion is
  there because Go's specification allows the compiler to fuse `x*y + z` across statements unless an
  explicit conversion rounds. That is §10's measurement of arm64.
- The narrowed sets make each rounding's overflow obligation provable:
  - each product is below 10²⁰⁰;
  - the sum is below 2⁶³·10²⁰⁰ < 2¹⁰²⁴.
- Fused, `round(F64, s + a[i] * b[i])` inside no scope is one `fma` per step: a different program, and
  written so.

### 18.5 What exactness buys without writing a rounding

**A geometric predicate** (Shewchuk's orientation test):

```
set P = F64 ∩ -1e100..1e100
fn orient(ax ∈ P, ay ∈ P, bx ∈ P, by ∈ P, cx ∈ P, cy ∈ P) (-1..1) {
    d = (bx - ax) * (cy - ay) - (by - ay) * (cx - ax)
    case { d > 0 -> 1; d < 0 -> -1; else -> 0 }
}
```

- The sign of `d` is the true one. No rounding is written, because the result is an integer.
- ρ is Shewchuk's adaptive scheme: the double computation, with its error bound, decides most inputs,
  and an expansion decides the rest.

**A hypotenuse**:

```
fn hypot(x ∈ F64 ∩ -1e300..1e300, y ∈ F64 ∩ -1e300..1e300) (F64) {
    round(F64, sqrt(x * x + y * y))
}
```

- `x * x + y * y` is exact, so there is no intermediate overflow and no scaling trick. One non-closed
  operation sits at the root, so the result is correctly rounded (Theorem T).
- C's `hypot` and Go's `math.Hypot` scale to avoid the overflow, and neither promises correct rounding.

### 18.6 FP8 and FP4 in a mixed-precision kernel

```
fn quantize(v ∈ F32) (E4M3) { round(E4M3, v, 'sat) }       // OCP's satfinite: total, no obligation

fn qdot(w ∈ Array(E4M3), x ∈ Array(BF16), scale ∈ F32) (F32) where len(w) == len(x) {
    round(F32, scale * loop i = 0, s = 0 {
        i < len(w) -> again(i + 1, s + w[i] * x[i])
        else -> s
    })
}
```

- An E4M3 times a BF16 has a 4 + 8 = 12-bit significand, and its exponent is inside binary32's. So each
  product is exact in a `float`, and ρ multiplies in `float`.
- The sum is the accumulator, and the one rounding is at the end. A GPU's tensor core accumulates in an
  order and a width its vendor does not fully specify. This answer is the same on every target.
- Stochastic rounding is `round(BF16, g, 'stochastic(r))`, with `r` random bits the program passes in
  (§6.3). It is a function of g and r.

### 18.7 What is refused, and what the refusal says

| written | refusal |
|---|---|
| `x = a ÷ b` | `a ÷ b is not in the ring; write round(S, a ÷ b), or a / b for the integer quotient` |
| `round(F64, exp(a) - exp(b))` | `two operations leave the ring under one rounding (exp, exp): round each first` |
| `round(F64, sqrt(x))`, x unbounded below | `sqrt needs x >= 0, not proven` |
| `round(F64, a * b)`, a, b ∈ F64 | `round(F64, a * b) may overflow F64: |a * b| < 2^1024 - 2^970 not proven` |
| `loop … again(x * x)` unrounded | `x * x doubles x's bits at every step: no bound`, as for an integer |
| `string_of(round(F64, 0.1))` | not refused. It prints `0.1000000000000000055511151231257827021181583404541015625`, the exact value. `render(F64, …)` prints `0.1` |

The fourth row is the cost that matters most. A float program meets the integers' discipline (ADR 0019):
an operation not proven inside its set is refused. The exits are the same: a narrower set, `'sat`, or
`-checked`. The decode at the input is where a range is usually stated.

## 19. Target primitives

### 19.1 The rule: a declaration realizes a rounding by its `ensure`

A target declaration is Σ × I (target-system.md §3): what it means and how the host does it. A host
operation whose `ensure` states `r == round(S, t)` for a ring term t **realizes** that rounding. Wherever
a program's rounding matches t, the printer may emit the host operation, and nothing else may realize it
by that name. This needs no new mechanism. The `ensure` is a claim the generator checks against the host
(ADR 0022, 0023) and a conformance case tests (primitives.md). A declaration with no such `ensure` is the
host's own operation, Tier 2, as `go./` is today.

### 19.2 Go

```
// targets/go/go.oro: which sets this host holds natively, as for 0..255 and the word
repr F(53, -1022, 1023) host "float64"
repr F(24, -126, 127) host "float32"

module go {
    // The host's double, IEEE's: NaN, ±Inf and -0 are its values, not ours (ADR 0030's pattern).
    set float64 = host "float64"
    axiom F64 ⊆ go.float64
    fn num(x ∈ float64) (result(F64, 'nan ∪ 'inf)) pure { … }     // the one decode, partial

    // ---- realizations: the ensure is the language term computed.
    fn fadd(a ∈ F64, b ∈ F64) (r ∈ F64) pure
        where abs(a + b) < 2^1024 - 2^970 ensure r == round(F64, a + b) { host expr "${a} + ${b}" }
    fn fmul(a ∈ F64, b ∈ F64) (r ∈ F64) pure
        where abs(a * b) < 2^1024 - 2^970 ensure r == round(F64, a * b) { host expr "float64(${a} * ${b})" }
    fn fdiv(a ∈ F64, b ∈ F64) (r ∈ F64) pure
        where b != 0 && abs(a ÷ b) < 2^1024 - 2^970 ensure r == round(F64, a ÷ b) { host expr "${a} / ${b}" }
    fn f32(x ∈ F64) (r ∈ F32) pure
        where abs(x) < 2^128 - 2^103 ensure r == round(F32, x) { host expr "float32(${x})" }
}
module go/math {
    fn FMA(a ∈ F64, b ∈ F64, c ∈ F64) (r ∈ F64) pure
        where abs(a * b + c) < 2^1024 - 2^970 ensure r == round(F64, a * b + c) {
        host expr "math.FMA(${a}, ${b}, ${c})" import "math"
    }
    fn Sqrt(x ∈ F64) (r ∈ F64) pure where x >= 0 ensure r == round(F64, sqrt(x)) {
        host expr "math.Sqrt(${x})" import "math"
    }
    // NOT a realization. Go's exp.go claims an error below 1 ulp for its Go code, and amd64, arm64,
    // loong64, riscv64 and s390x replace it with assembly. So the result is Go's, a function of x and
    // GOARCH, Tier 2. The claim is the one the source justifies, a relation in the logic.
    fn Exp(x ∈ F64) (r ∈ float64) pure ensure abs(r - exp(x)) < ulp(F64, exp(x)) {
        host expr "math.Exp(${x})" import "math"
    }
    // Neither a realization nor a claim: §10 measured Pow 18,212 ulps off, so nothing is promised.
    fn Pow(x ∈ F64, y ∈ F64) (r ∈ float64) pure { host expr "math.Pow(${x}, ${y})" import "math" }
}
```

`round(F64, exp(x))` is never printed as `math.Exp`. The compiler realizes it with a correctly rounded
algorithm (CORE-MATH's, ported) written in the language. A program that wants Go's answer calls
`go.math.Exp` by name and decodes the result. That is a Go program, as `go./` makes one today.

### 19.3 LLVM (the windows target's Σ_LLVM, ADR 0080 item 7)

```
repr F(53, -1022, 1023) host "double"
repr F(24, -126, 127) host "float"
repr F(11, -14, 15) host "half"
repr F(8, -126, 127) host "bfloat"

module llvm {
    fn fadd(a ∈ F64, b ∈ F64) (r ∈ F64) pure
        where abs(a + b) < 2^1024 - 2^970 ensure r == round(F64, a + b) { host expr "fadd double ${a}, ${b}" }
    fn fma(a ∈ F64, b ∈ F64, c ∈ F64) (r ∈ F64) pure
        where abs(a * b + c) < 2^1024 - 2^970 ensure r == round(F64, a * b + c) {
        host expr "call double @llvm.fma.f64(double ${a}, double ${b}, double ${c})"
    }
    fn sqrt(x ∈ F64) (r ∈ F64) pure where x >= 0 ensure r == round(F64, sqrt(x)) {
        host expr "call double @llvm.sqrt.f64(double ${x})"
    }
    fn fptrunc(x ∈ F64) (r ∈ F32) pure
        where abs(x) < 2^128 - 2^103 ensure r == round(F32, x) { host expr "fptrunc double ${x} to float" }
    fn fptrunc_bf(x ∈ F32) (r ∈ BF16) pure
        where abs(x) < 2^128 - 2^119 ensure r == round(BF16, x) { host expr "fptrunc float ${x} to bfloat" }
    fn sitofp(n ∈ -2^53..2^53) (r ∈ F64) pure ensure r == n { host expr "sitofp i64 ${n} to double" }
    // The ordered reduction: a rounding after every add, in index order, which is the scope's sum.
    // Each partial sum's overflow bound is a `where` like fadd's, omitted here.
    fn reduce_fadd4(s ∈ F64, v ∈ Array(F64)) (r ∈ F64) pure where len(v) == 4
        ensure r == round(F64, round(F64, round(F64, round(F64, s + v[0]) + v[1]) + v[2]) + v[3]) {
        host expr "call double @llvm.vector.reduce.fadd.v4f64(double ${s}, <4 x double> ${v})"
    }
}
```

The template form of `ir/llvm` is not fixed. These show only what each declaration states. The
`where` clauses follow the survey's §3: each is a domain D that excludes the host's behaviour outside the
ring, so the flags and the poison never become visible. `sitofp` states an exact conversion,
`ensure r == n`, on the range where it is exact.

**Not offered as realizations**:
- **The fast-math flags** `reassoc`, `contract`, `afn` and `fast`, and `fmuladd` ("fusion is not
  guaranteed"). Each is a relation (survey §4.4), and its answer depends on LLVM's choices.
- **The constrained intrinsics with a static rounding mode.** Their `ensure` would be true only while
  the dynamic mode matches. LLVM states that a mismatch is undefined behaviour (survey §4.5). The
  directed roundings are realized by error-free transformations instead (§7).

### 19.4 JavaScript, where the host lacks a primitive

```
repr F(53, -1022, 1023) host "number"
repr F(24, -126, 127) host "Float32Array"      // a table of F32, as Uint8Array is of 0..255
module js {
    fn fround(x ∈ F64) (r ∈ F32) pure where abs(x) < 2^128 - 2^103 ensure r == round(F32, x) {
        host expr "Math.fround(${x})"
    }
}
```

JavaScript has no `fma`, so no declaration realizes `round(F64, a * b + c)`. The compiler realizes it
in the language: Veltkamp's split and Dekker's product make `a * b` exact as a pair, about 17 operations,
then the correctly rounded sum of three (Boldo and Melquiond). The program is unchanged and so is the
answer. Only the cost differs, and `round(F64) { … }` avoids it where per-operation IEEE is what the
program means.

## 20. Does it restrict reaching for the target?

**What is not restricted: calling.** Every host operation stays declarable under the host's name with
the host's semantics, over the host's own type: `go.float64`, LLVM's `double` with NaN and the flags.
That is Tier 2 with no portability claim, as `go./` and `go.f+` are today. The parasite model loses
nothing it can call.

**What is restricted: which host operations may count as the language's.** An operation counts only when
its `ensure` is true, which takes two things:
- the generator's check against the host (ADR 0022);
- a conformance case.

Measured against §10 and the survey:

| host behaviour | the language's? | reached how |
|---|---|---|
| IEEE `+ - * ÷`, `sqrt`, `fma`, conversions, `fptrunc` and `sitofp`: correctly rounded by IEEE 754 on every host | **yes**, by fusion, at the host's speed | automatically |
| libm and `java.lang.Math`: `exp`, `sin`, `pow`, not correctly rounded | no: their answer is the host's | by name, Tier 2; `round(F64, exp(x))` costs a correctly rounded algorithm, within a small factor of glibc's speed (CORE-MATH's report; not measured here) |
| NaN, ±∞ and −0 as values (a NaN as a missing-data marker) | no: not in the ring | the host's type, then a decode; or an `Option`, a variant the program states |
| `reassoc` and `fast`: a float sum vectorized by LLVM | no: a relation | the exact sum is associative, so it vectorizes with a deterministic answer. **Not measured**: its speed against `reassoc`'s |
| an FP8 GEMM on a tensor core, accumulation order unspecified | no: a relation | Tier 2 on a future GPU layer (ADR 0085), with its hull as the claim |
| the dynamic rounding mode and the status flags | no: state every operation would read | a static mode, by EFTs (§7) |

So the restriction is exactly the line Theorem D draws: **a host behaviour is the language's when it is
a function of the operands, stated and checked**. Everything else is still callable, under its own name,
and marked as the host's. Two costs fall on the programs that want the host's speed with per-operation
rounding:
- they write `round(F64) { … }`, and get the host's instruction, never a slower one;
- their inputs must be bounded so that the overflow obligations are proven, as integer programs' already
  are.

One cost may matter for speed, and it is unmeasured: a fast-math vectorized sum against the exact
accumulator vectorized. If the gap is large, a program that accepts any order has two choices. One is
the scope with an explicit reduction tree (pairwise sums, a function of the inputs and so the
language's). The other is the relation, as Tier 2.

## 21. Surface questions for hamza

- **S1. The rounding's spelling.** `round(S, e, mode)`, with S static and the mode a tag. The
  alternatives: an infix form like `mod`, `e round F64`; or a set applied as a function, `F64(e)`,
  which reads well but is refused today, since a set has no representation in a term. **Recommended:
  `round(S, e)`.** It is an operation, and its name says so.
- **S2. The scope (G6).** `round(F64) { … }` shares the name with S1's operation. The alternatives:
  `ieee(F64) { … }`, or `each round(F64) { … }`. **Recommended: `round(F64) { … }`**, since it means
  "round here, everywhere".
- **S3. The real quotient's symbol.** `÷`, ASCII `\div`, as `⊕` is `\oplus` (ADR 0082). `/` keeps the
  integer quotient, so no program changes meaning. **Recommended.**
- **S4. `/` on a non-integer.** Part II extends `/` to the ring as the integer quotient, so `3.5 / 2` is
  1. The alternative is to require integer operands, so that `3.5 / 2` is refused and points to
  `round(S, 3.5 ÷ 2)`. **Recommended: refuse.** The extension is sound, but a reader of `3.5 / 2` would
  expect 1.75, and a refusal that names `÷` costs nothing.
