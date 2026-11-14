# Karatsuba in K(ℕ\*): a product as evaluation and interpolation, its levels a tensor power

Date: 2026-11-13. hamza: "go ahead with Karatsuba, always mathematical and algebraic". The step
before divide-and-conquer printing (carry-2026-11-13: printing's gap is asymptotic, and every fast
radix conversion is built on fast multiplication). The prior work is
[karatsuba-2026-08-30](../gauntlet/results/karatsuba-2026-08-30.md): Karatsuba with no recursion,
level by level, hand-written in Go, Java, JavaScript and MASM. This document is the algebra of the
same algorithm in the language's own library, K(ℕ\*) (`emit/natnum.oro`, ADR 0099, base B = 2⁶⁴).

## 1. The program, and what it measures

`fib(10⁶)` by fast doubling, printed mod 10⁹ + 7 so that printing costs nothing. Its state is
(p, q) = (F(m − 1), F(m)) over the bits of n from the top:
- F(2m − 1) = p² + q²;
- F(2m) = q(2p + q);
- F(2m + 1) = F(2m − 1) + F(2m).

These are sums and products of naturals only, so its result claim `Nat` is proven. Each bit costs
three products of numerals of one size, and F(10⁶) has 694,241 bits, 10,848 digits. Measured
(scratchpad `kara/`, 21 interleaved rounds, medians, ms):

| windows, schoolbook M | Go (`math/big`, ours or hand-written) |
|---|---|
| 109 | 17 |

**6.4×**. Go's `nat.mul` switches to Karatsuba above 40 words (`karatsubaThreshold`). Ours is
quadratic.

## 2. A numeral is a polynomial evaluated at B

ℕ\* is the image of ε_B : ℤ[x] → ℤ, x ↦ B, restricted to coefficients in [0, B). ε_B is a ring
homomorphism, so a product of numerals is
- the product of their polynomials, a convolution of digit vectors, which is exact over ℤ;
- then the normal form ν: the unique digit vector in [0, B) with the same image. ν is the carry pass.

Every linear map on coefficient vectors commutes with ε_B, so carries may be resolved at any point
of a computation that is linear in the coefficients. That is what lets the algorithm below keep
every intermediate a normalized numeral.

## 3. Karatsuba is a bilinear algorithm of rank 3 for μ₂

Let μ_n : ℤⁿ × ℤⁿ → ℤ²ⁿ⁻¹ be polynomial multiplication of degree < n. Writing A = A₀ + A₁y, the
product C = AB = C₀ + C₁y + C₂y² has three coefficients, and **evaluation–interpolation** computes
them from three products:

    C(0) = A₀B₀,   C(1) = (A₀ + A₁)(B₀ + B₁),   C(∞) = A₁B₁,
    C₀ = C(0),     C₁ = C(1) − C(0) − C(∞),       C₂ = C(∞).

As matrices, μ₂(a, b) = W((Ua) ⊙ (Ub)), with ⊙ the pointwise product:

    U = | 1 0 |      W = |  1 0  0 |
        | 1 1 |          | −1 1 −1 |
        | 0 1 |          |  0 0  1 |

**det W = 1: W is unimodular, so the interpolation is exact over ℤ, with no division.** That is
Karatsuba's distinction among the evaluation–interpolation algorithms. Toom–Cook at three points
(Toom 1963; Cook 1966; Bodrato and Zanoni 2007) needs exact divisions by 2 and 3, and the FFT needs
roots of unity, which ℤ lacks (Schönhage and Strassen 1971 adjoin one in ℤ/(2^N + 1)). Over ℤ,
three is the least rank for μ₂: its tensor rank is 3, by Winograd's bound 2n − 1 for polynomial
multiplication (Winograd 1980; Bürgisser, Clausen and Shokrollahi 1997, ch. 14).

**The subtractive form** evaluates at −1 instead of 1: C(−1) = (A₀ − A₁)(B₀ − B₁) and C₁ = C(0) +
C(∞) − C(−1). It keeps every operand at h digits (|A₀ − A₁| < Bʰ) but carries a sign per node (GMP's
`toom22`; Brent and Zimmermann 2010, §1.3.2). **The additive form is chosen here.** It needs no
signs and no comparison, and its growth is one digit a level, which a uniform slot absorbs (§5).
§9 has the measure that would reverse this.

## 4. k levels are the k-th tensor power, and the tensor power factors into level passes

Let n = 2ᵏ. Read a polynomial of degree < 2ᵏ as multilinear in x₁, …, x_k, by binary digits of the
exponent, with x_j ↦ x^(2^(j−1)) the **Kronecker substitution** κ. It is a ring homomorphism
ℤ[x₁, …, x_k] → ℤ[x], and it is not injective on degree ≤ 2 per variable: overlapping images add,
which is the combine's overlap-add. Multiplication of multilinear polynomials is μ₂ in each variable,
so

    μ_{2ᵏ} = κ ∘ μ₂^{⊗k} = κ ∘ W^{⊗k}((U^{⊗k} a) ⊙ (U^{⊗k} b)).

The tensor product of bilinear algorithms is a bilinear algorithm for the tensor product map, of
rank at most the product of the ranks (Bürgisser, Clausen and Shokrollahi 1997, ch. 14). So the
algorithm has 3ᵏ pointwise products, which is n^(log₂ 3) = n^1.585.

**The tensor power factors into k passes, one per variable**, the Kronecker factorization that
makes the FFT iterative (Van Loan 1992, ch. 1, for the Cooley–Tukey case):

    U^{⊗k} = Π_{L=0}^{k−1} (I_{3^L} ⊗ U ⊗ I_{2^{k−L−1}}),   and W^{⊗k} likewise, in reverse.

Pass L applies U along variable L. So the k-level algorithm is three phases:
- **down**: k passes, each taking level L's 3^L operand pairs to level L + 1's 3^{L+1};
- **leaves**: 3ᵏ independent products;
- **up**: k passes, each taking level L + 1's products to level L's, by W and κ along one variable.

**No pass needs recursion**, since each is a loop over the nodes of one level, and the levels are a
loop. This is ADR 0014's "a balanced recursion is a loop over levels", now as a theorem rather than
an observation: the recursion tree is the index set of a tensor power, which is the same for every
operand.

**Stopping at depth D < k**: the leaves are then not digits but numerals of s_D digits, multiplied
by Algorithm M. The leaves' ring is ℤ[x]_{<s_D} under ε_B, and the passes above are unchanged.

## 5. Sizes, and why every slot of a level is one size

Integers are not polynomials of degree a power of two, and a sum A₀ + A₁ can carry. Let s₀ =
max(m_a, m_b), the operands zero-extended, and at each level:

    h_L = ⌈s_L / 2⌉,   s_{L+1} = h_L + 1.

- A node of level L holds operands < B^{s_L} in s_L digits. Its children are:
  - lo, digits [0, h_L);
  - hi, digits [h_L, s_L), at most h_L of them;
  - lo + hi < 2B^{h_L} ≤ B^{h_L + 1}.

  Each fits in s_{L+1} digits, **so every node of a level has one slot size**, and a level is a
  table of 3^L slots. Zero extension is exact: the extra coefficients are 0, and 0 rides through
  U, ⊙ and W.
- A node's product is < B^{2s_L}, so it fits its slot of 2s_L digits, and **the combine's last carry
  is 0**. That is a theorem of the slot sizes, not of the analysis. The proof: the product of two
  operands < B^{s_L}. The witness is the products checked against Algorithm M and Go's `math/big`.
- s converges to its fixed point 2, so a threshold T ≥ 4 is reached: D = the least L with s_L ≤ T,
  about log₂(s₀/T).

## 6. Each pass's arithmetic, inside the double word

Every step is an intermediate the decision proves in i128 (ADR 0103, 0105), each read back by its
halves:

| pass | step | range | carry |
|---|---|---|---|
| down, lo + hi | x + y + c | [0, 2B) | {0, 1}: **the full adder** (ADR 0108) |
| leaves, M | u·v + w + k | [0, B²) | [0, B) (nat64-2026-11-12) |
| up, the combine | P₀[i] + P₂[i − 2h] + P₁[i − h] − P₀[i − h] − P₂[i − h] + c | [−2B, 3B) | [−2, 2] |

**The combine is one signed pass per node.** Digit i of the parent's product is the coefficient of
Bⁱ in P₀ + (P₁ − P₀ − P₂)Bʰ + P₂B²ʰ. Three of its terms add and two subtract, so with digits in
[0, B) and the carry c ∈ [−2, 2], the step t lies in [−2B, 3B). Its carry ⌊t / B⌋, an arithmetic
shift, is again in [−2, 2]: the interval [−2, 2] is a fixed point of the carry's transfer. Each digit
written is t & (B − 1), in [0, B).

The middle coefficient C₁ = A₀B₁ + A₁B₀ is never formed on its own. It is the linear combination
inside the step, and ν, the carry pass, resolves it with the rest (§2).

## 7. Cost, and the threshold

The leaf count is 3^D, each leaf ≈ s_D² steps, against s₀² for M:
- with s_D ≈ s₀/2^D + 2, the leaves cost (3/4)^D·s₀² plus lower terms;
- the passes cost O(s₀·(3/2)^D) digit steps each.

So the gain at depth D is about (4/3)^D, and the threshold T balances a leaf's s² against the
passes' per-node overhead. GMP measures it per machine (`MUL_TOOM22_THRESHOLD`, 20–30 limbs on
x86-64); Go fixes 40. T is measured here (§9), not assumed. For fib(10⁶)'s 10,848 digits with
T = 32: D = 9, 3⁹ = 19,683 leaves of 22 digits, and a model gain of about 6×. The prior MASM port
reached 3.6× at 1,024 limbs (karatsuba-2026-08-30 §3c).

**Memory**: level L's operand table has 3^L·s_L digits, about s₀(3/2)^L, and its product table
twice that. A pass's input dies when its output is born, so the live storage is two adjacent levels,
at most about 4s₀(3/2)^D digits: 1.5 MB for fib(10⁶) at D = 9. Each level is one table born full.
Each dead level returns to the heap's free list of its class (ADR 0107), so the allocator is asked
2D + 1 times a product.

## 8. The design in K(ℕ\*), within the language

- `kmul(ml, a, b)`, called from `mul` when both sizes are at least T and the larger is at most
  twice the smaller. Otherwise M; an unbalanced product by chunks waits on a program.
- **down**: `loop L, xa, xb { L ≥ D -> leaves; else -> again(L + 1, down(xa, L), down(xb, L)) }`,
  each `down` a fresh table of level L + 1, built by a loop over its nodes, each node's three
  children written by one helper that borrows the buffer.
- **leaves**: one table of 3^D products, each Algorithm M's rows at the node's offsets.
- **up**: `loop L = D − 1 down to 0, p { … again(L − 1, up(p, L)) }`, each `up` the combine of §6,
  one signed pass per node.
- **the result**: the top product's digits and the header 2m + s, s = s_a ⊕ s_b.

Every index is an offset j·w + i into a level's table. Each node's helper bounds what it reads and
writes once, against its table's length (the library's idiom, "the sizes against what the loop
reads and writes, once"). So every access is proven by its loop's guard, and the theorem of §5 makes
the bounds identities on every valid input.

## 9. What is measured, and what would change the design

- fib(10⁶) against Go, and the product alone against M, at T ∈ {16, 24, 32, 48, 64}.
- **Additive against subtractive**: the subtractive form's leaves are one digit shorter a level,
  (s_D − 1)²/s_D² ≈ 0.92 at s_D = 24, at the price of a comparison and a sign per node. Built only if
  the leaves measure above 90% of the time.
- **Copied children against offsets into the parent**: the prior work measured 1.40× for offsets on
  Go (karatsuba-2026-08-30 §3a). Here the copies are the down pass, so its share of the time decides.

## 10. Why not

- **Recursion**: not in the language (ADR 0014), and §4 shows none is needed. The tensor power is a
  loop over levels by its own factorization.
- **One function per level** (kmul₀ … kmul_K, each calling the one below three times): a finite
  unrolling of the same tensor power, written K times, with each node's tables allocated apart (3^D
  allocations a product). The level loop is the factorization itself.
- **Toom-3 now**: rank 5 for μ₃, n^1.465, with exact divisions, worth it above about 100 limbs in
  GMP. It is the next member of the same family, and so is built after Karatsuba is measured.
- **FFT multiplication** (Schönhage–Strassen 1971; Harvey and van der Hoeven 2021, O(n log n)):
  GMP's threshold is thousands of limbs. Waits on a program that large.
- **The subtractive form**: §3, §9.

## 11. What follows

Divide-and-conquer radix conversion (Brent and Zimmermann 2010, §1.7) and Burnikel–Ziegler
recursive division (1998) are both products of large numerals, each a balanced recursion with the
same level structure. `fact`'s printing (2.7× Go) waits on them.

## References

- Karatsuba, A. and Ofman, Yu. "Multiplication of multidigit numbers on automata", Soviet Physics
  Doklady 7 (1963), 595–596 (Russian 1962).
- Toom, A. L. "The complexity of a scheme of functional elements realizing the multiplication of
  integers", Soviet Math. Doklady 3 (1963); Cook, S. A. *On the minimum computation time of
  functions*, thesis, Harvard 1966.
- Knuth, D. E. *The Art of Computer Programming* 2, 3rd ed., §4.3.3.
- Winograd, S. *Arithmetic Complexity of Computations*, SIAM 1980.
- Bürgisser, P., Clausen, M. and Shokrollahi, M. A. *Algebraic Complexity Theory*, Springer 1997,
  ch. 14–15.
- Van Loan, C. *Computational Frameworks for the Fast Fourier Transform*, SIAM 1992, ch. 1.
- Bernstein, D. J. "Multidigit multiplication for mathematicians", 2001.
- Bodrato, M. and Zanoni, A. "Integer and polynomial multiplication: towards optimal Toom–Cook
  matrices", ISSAC 2007.
- Brent, R. P. and Zimmermann, P. *Modern Computer Arithmetic*, Cambridge 2010, §1.3, §1.7.
- Schönhage, A. and Strassen, V. "Schnelle Multiplikation großer Zahlen", Computing 7 (1971).
- Harvey, D. and van der Hoeven, J. "Integer multiplication in time O(n log n)", Annals of Math. 193
  (2021).
- Burnikel, C. and Ziegler, J. "Fast recursive division", MPI-I-98-1-022, 1998.
- The GMP manual, §15.1 (Multiplication); Go's `math/big`, `nat.go` (`karatsubaThreshold`).
