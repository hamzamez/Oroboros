# 0109 — A product is evaluation and interpolation, and its levels are a tensor power

Date: 2026-11-14
Status: Accepted (hamza, 2026-11-13: "go ahead with Karatsuba, always mathematical and algebraic";
the step [karatsuba-research.md](../karatsuba-research.md) sets out). Extends
[ADR 0099](0099-an-unbounded-integer-is-a-numeral-of-any-length-with-its-signed-size-last.md)'s
Algorithm M with Karatsuba's product. Builds [ADR 0014](0014-recursion-is-not-in-the-language.md)'s
"a balanced recursion is a loop over levels" as a factorization.

## Context

After ADR 0108, K(ℕ\*)'s product of two numerals was Algorithm M, quadratic. `fib(10⁶)` by fast
doubling, three products of numerals of one size per bit, ran 119 ms on windows against Go's 29:
4.1× (the result's sweep; 110 against 21 in the research's). Go's `math/big` switches to Karatsuba above 40 words. `fact`'s printing gap (2.7×) is
asymptotic too, and every fast radix conversion is built on fast multiplication.

## Decision

1. **A product of two numerals, each at least T = 64 digits and within a factor of two of each
   other, is Karatsuba's** (karatsuba-research.md §3–§5):
   - a numeral is its polynomial at B = 2⁶⁴, and μ₂ = W((Ua) ⊙ (Ub)) is evaluation at 0, 1 and ∞;
   - interpolation is by W, with det W = 1, so it is exact over ℤ with no division;
   - D levels are μ₂'s D-th tensor power, which factors into one pass a level (I ⊗ U ⊗ I down,
     I ⊗ W ⊗ I up) with the Kronecker substitution's overlap-add.

   So the product is three loops over levels with no recursion: down, leaves by Algorithm M, up. The
   additive form needs no signs. A level is one table of 3^L slots of one size, s_{L+1} = ⌈s_L/2⌉ +
   1, so the combine's last carry is 0 by the slot sizes.
2. **A pass over a level is one loop over the level's nodes × digits.** (j, i) is its state and the
   carry is reset at each node's start. The product of two index domains is one domain, so the pass
   is one fold, not a fold of folds. The interval analysis re-solves an inner loop on every round of
   the loop around it, so a nested loop multiplies its cost by the rounds. Deciding `nat.mul` took
   53 s nested and takes 1.5 s flat.
3. **Every offset is a word by the machine's bounds**:
   - s₀ is taken through `room`: a table holds at most 2³¹ entries;
   - a level's node count stays below 3¹², and past it the leaves only grow longer, which is exact;
   - a leaf row bounds what it reads and writes once, the library's idiom, the identity on every slot
     by §5, so every access is proven by its loop's guard.
4. **A rule's body is its table's elements**, received by ρ of the element hull as a store's value is
   (ADR 0083 item 5, ADR 0106; `ir/words.go`). Karatsuba's level-0 tables are rules of a numeral's
   digits, which are in U.

## Why not

- **The subtractive form** (evaluation at −1, GMP's `toom22`): each operand stays one digit
  shorter a level, at the price of a sign per node and a comparison. The leaves' saving is about 8%
  at s_D = 24. Built if the leaves measure above 90% of the time (karatsuba-research.md §9).
- **Recursion**: not in the language, and the tensor power's factorization needs none.
- **One function per level** (kmul₀ … kmul_K): a finite unrolling of the same tensor power, and an
  allocation per node.
- **Nested node and digit loops**: measured. The decision of `nat.mul` took 53 s, and 191 s for the
  whole program when its instance fell back to inlining (ADR 0102).
- **Copying the lo and hi children** is what the down pass does. Offsets into the parent (the prior
  Go port's 1.40×, karatsuba-2026-08-30) wait on the down pass's measured share: 21% of a product in
  the C model.
- **Toom-3 and FFT multiplication**: the next members of the family, each after this is measured
  (karatsuba-research.md §10).
- **Squaring by polarization now**: a² = Σ aᵢ²B²ⁱ + 2Σ_{i<j} aᵢaⱼBⁱ⁺ʲ halves the leaves, and
  Karatsuba's squares stay squares. It is the next step, since fast doubling is two squares and one
  product a bit.

## Consequences

- fib(10⁶) by fast doubling: 119 → 48 ms against Go's 29, 4.1× → 1.67× (karatsuba-2026-11-14);
  the standing programs unchanged; `nat.mul` decided in 1.5 s, which every windows program with a
  product of two numerals pays.
- `big-kara`, a differential case on four targets, pairs Karatsuba's products with Python's answers.
  `PlantKaratsubaFault` drops −P₂, and the witness sees every product with a level go wrong.
- `PlantRuleElemSigned` brings back the refusal a rule of U digits met.
