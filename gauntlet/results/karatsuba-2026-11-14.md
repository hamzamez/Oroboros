# Karatsuba in K(ℕ\*): a product as evaluation and interpolation, its levels a loop

Date: 2026-11-14. [ADR 0109](../../docs/decisions/0109-a-product-is-evaluation-and-interpolation-and-its-levels-are-a-tensor-power.md),
[karatsuba-research.md](../../docs/karatsuba-research.md). **Fibonacci of 10⁶ by fast doubling, three
products of numerals of one size a bit, runs in 48 ms on windows where it ran in 119, against Go's
29: from 4.1× Go to 1.67×.** The standing programs are unchanged, and every proof count is unchanged
(5,710 of 5,764, 649 of 673).

## The program

`fib(10⁶)` printed mod 10⁹ + 7 (scratchpad `kara/ffib-w.oro`, `m3/ffib-w.oro`). Its state is
(F(m − 1), F(m)) over the bits of n:
- F(2m − 1) = p² + q²;
- F(2m) = q(2p + q);
- F(2m + 1) = their sum.

These are sums and products of naturals, so `Nat` is proven. The first version wrote F(2m) =
F(m)(2F(m + 1) − F(m)), and its claim `Nat` was refused: nothing proves 2b − a ≥ 0. F(10⁶) has
10,848 digits.

**A first timing of Go's binary read 430 ms**, against 8 ms inside a hand-written Go program. That
was the first run of a new executable, which Windows scans. Interleaved runs give 17–29 ms for
ours and for hand-written Go alike.

## The algebra, and the C model

karatsuba-research.md:
- a numeral is its polynomial at B;
- Karatsuba is μ₂'s bilinear algorithm of rank 3, with interpolation by a W of determinant 1;
- D levels are the D-th tensor power, which factors into one pass a level;
- every slot of a level has one size, s_{L+1} = ⌈s_L/2⌉ + 1, so the combine's last carry is 0.

A C model of exactly that (scratchpad `kara/k.c`) equals schoolbook at n = 10,848 and on all-ones
operands of every size from 1 to 300. One product of 10,848 digits:

| T | 32 | 64 | 96 | 128 | 192 |
|---|---|---|---|---|---|
| ms (schoolbook 106) | 18.4 | 17.3 | 17.3 | 17.0 | 20.1 |

Splitting the combine's domain by which terms are present gains little at large T. At n = 5,424 the
leaves are 68% of a product, the down pass 21% and the up pass 11%.

## What was built

- `emit/natnum.oro`:
  - `mul` dispatches to `kmul` when both sizes are at least `kt` = 64 and within a factor of two;
    otherwise it calls `mulm`, Algorithm M;
  - `kmul` is three loops in sequence over frozen tables: down (`kdown`), the leaves (`kleaves`,
    `kmac`) and up (`kup`);
  - each pass is one loop over its level's nodes × digits;
  - a leaf row bounds its reads and writes once.
- `ir/words.go`: **a rule's body is its table's elements**, received by ρ of the element hull, as a
  store's value is.
- The witnesses:
  - `gauntlet/differential/cases/big-kara.oro`, nine products on four targets: 64 × 64 (one leaf),
    65 × 64, 128², 129 × 100, 200², 190 × 100 (unbalanced), 1000² (depth 4), 777 × 500, and 63 ×
    64 (Algorithm M). Each pair is computed twice, on all-ones digits and on powers of 3, and every
    answer is Python's;
  - `cmd/build/kara_test.go` builds those products on windows. **The planted fault** drops −P₂ from
    the combine, and exactly the seven products with a level go wrong;
  - `cmd/build/rule_test.go`: a rule of U digits under a guard, on windows. **The planted fault**,
    the body received by the table value, brings the refusal back.

## What failed first

1. **"a result is [0, 2⁶⁴ − 1] … held in the UNSIGNED word, not an Int"**: the level-0 tables are
   rules, `array { dg(a, la, i) : … }`, and the word selection received a rule's body by the table
   value, which is no integer, so it defaulted to signed. Stores had the law (ADR 0106: a table's
   elements are ρ of their hull), and rules did not. This was found by dumping the refused value's
   definition: `index` of the parameter numeral, a U digit, yielded by `dg`'s `if`.
2. **A rule must be allocated** to be a loop's initial value or a result: `alloc(array { … })`.
3. **The compile never finished**: more than 16 minutes, with a stack dump showing the interval
   analysis five loops deep.
   - The up pass sat in the down loop's exit arm. Sequential loops over frozen tables fixed that: the
     build finished in 191 s, refused (item 1).
   - The rest was nesting: each pass was a node loop around a digit loop around the level loop. The
     decision re-solves an inner loop on every round of the loop around it, and it runs again after
     each selection pass. One loop over nodes × digits, with the carry reset at a node's start, took
     deciding `nat.mul` from 53 s to 1.5 s, and the program's build from 72 s to 8 s.
4. **43 of 243 operations unproven**: every offset product, j·s, 3·nodes·s₂ and so on. A numeral's
   size is bounded by its table's length, about 2⁴⁷, so the products reached 2⁶⁵. s₀ is now taken
   through `room`, the machine's 2³¹-entry tables. With 3¹² nodes at most, every offset is below 2⁵³,
   and all 242 are proven.
5. **`kslot`'s guarded reads in the leaf row** cost two tests a digit and two stack reloads (read
   off the assembly). The row now bounds itself once, and the program went from 46 to 37 ms.
6. **The witness's premise**: 64 × 64 is at T, a single leaf with no combine, so the planted fault
   leaves it right. The test now states which products have a level.
7. **A rule of U elements on Go** does not compile: no table is retyped by its element hull for Go
   (Finalize narrows a table's type inside the signed word only), so `[]int` holds `uint64`. This is
   outside this step and not built. The rule witness runs on windows, where `i64` serves both
   readings.

## Measured

Serial, interleaved, 21 rounds, whole processes from Python; medians, minima in brackets, ms. The
baseline was built at HEAD (`93e2461`) in a worktree, and every output equals the baseline's and
Go's:

| program | before | **Karatsuba** | Go `math/big` | ratio to Go |
|---|---|---|---|---|
| fib(10⁶), fast doubling | 119 (111) | **48 (39)** | 29 (22) | 4.1 → **1.67** |
| the product by a word, `factm` | 44 (36) | 48 (34) | 50 (36) | 0.97 |
| fact(20,000), printed | 114 (102) | 114 (105) | 55 (42) | 2.06 |
| fib(10⁵) | 49 (41) | 49 (41) | 46 (34) | 1.06 |
| 2,000 quotients | 93 (83) | 93 (86) | 120 (96) | 0.78 |

- The machine was in its slow state for this sweep. fib(10⁵) reads 1.06× here and read 1.23× in
  carry-2026-11-13, so the ratios are this sweep's, comparable within it.
- The threshold, on fast doubling: 52, 50, 51, 48 and 49 ms at T = 32, 48, 64, 96 and 128, inside
  the noise. 64 is kept.
- **What is left against Go** is mostly the leaves, and fast doubling's products are two squares
  and one product. Go squares by polarization (`nat.sqr`, `karatsubaSqr`), which halves a leaf, and
  we do not yet.

**The compile.** `nat.mul` is decided in 1.5 s (the probe, alone). Every windows program with a
product of two numerals pays it, and `power.oro`'s paired compile grew by 1.9 s: `power.oro` 296 → 2,218 ms paired, `pair.oro` 2.4×,
both refused by `gen`'s older W5 either way. Accepted, and logged in ACCEPTED.md. Analysing a loop's
exit arm once, rather than on every round, would win part of it back; that is the decision's
change, not done here.

## What was not built

- **Squaring by polarization**, a² = Σ aᵢ²B²ⁱ + 2Σ_{i<j} aᵢaⱼBⁱ⁺ʲ, which halves the leaf products.
  Karatsuba's three children of a square are squares. It is the next step.
- **Offsets for lo and hi** in place of copies: the down pass is 21% in the C model.
- **The subtractive form**, **Toom-3**, **FFT**: karatsuba-research.md §9–§10.
- **An unbalanced product** by chunks: past the factor of two, M. No program asks.
- **A rule of U elements on Go, the JVM and JavaScript**: item 7 above.
