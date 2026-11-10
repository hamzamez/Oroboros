# Printing in one pass per chunk; divide and conquer derived and not built

Date: 2026-11-10. Item 6 of the standing measure's list (lenonce-2026-11-10): render, 62% of
fact(20,000) on windows.

## What it is

Render divides |a| by 10⁹ once per nine decimal digits. It took two passes per chunk, `rem_small` and
then `div_small`, each over the whole remaining numeral. **A short division is a Mealy machine whose
final state is the remainder**, so one fold yields both. The pair cannot be returned as a product
and fed back to the loop, since an `again` after a product's join point is plan item 9, not built. It
is packed into one table instead:

    [r, q_0, q_1, …, q_{m−1}, header]

The division runs from the top digit down, and so does its build, so slot 0 is written last, exactly
when r is known. The writes stay at the counter and the table is born full (ADR 0097). The dividend's
digit j is at a[off + j], with off 0 for a numeral and 1 for a packed quotient. Its size is its
header's either way. The quotient's size is known first, as `qsmall`'s.

## What was built

`qrem(ml, a, off, k)` in `emit/natnum.oro`, its capacity through `room`. `render` carries the packed
table and its offset, one pass per chunk. The theory checks.

## What was measured

Best of five, two rounds; "before" is lenonce-2026-11-10's build:

| program | before | one pass | Go, `math/big` |
|---|---|---|---|
| fact(20,000) | 435.2 / 417.7 ms | **331.5 / 323.3** | 35.8 / 36.3 |
| fib(10⁵) | 236.4 / 234.2 | 230.8 / 227.7 | 28.3 / 29.1 |
| 2,000 quotients | 287.4 / 281.6 | 285.8 / 291.9 | 90.2 / 89.3 |

fact is 23% faster. Printing went from about 255 ms (lenonce's split) to about 170 ms, less than the
halving the pass count predicts: each chunk still makes a table, taken by fit, and reads each digit
through a mask (item 2). fib and D print little, and are unchanged.

**The check** passes with the emission byte-identical: no program in the baseline prints a K(ℕ\*)
value (`examples/big/`'s functions return theirs). The differential's `big-fib`, `big-nat-ops` and
`big-nat-div` run the new render on four targets and agree.

## Divide and conquer, derived first

`math/big` converts by divide and conquer (Brent and Zimmermann 2010, §1.7): x = q·P + r with
P = 10^(9·2^k) splitting the numeral in halves, each half printed recursively. Its cost here, with
schoolbook D underneath:
- the top level divides m digits by m/2, about (m/2)² digit-pair steps;
- the next level does two at m/4, 2·(m/4)²;
- and so on, a geometric series summing to **about m²/2**.

Chunking costs about C·m/2 with C ≈ 32m·log₁₀2 / 9 ≈ 1.07m chunks, **about 0.53m²**. The same order:
**no gain** while multiplication and division are quadratic. Divide and conquer pays only over
subquadratic multiplication and division: Karatsuba (Karatsuba and Ofman 1962), then recursive
division (Burnikel and Ziegler 1998), which `math/big` has above its measured thresholds. So it is
owed after them, and was not built.

## The owed list, restated

Windows against Go, after this step: fact 9.0×, its product 4.4×, fib 7.9×, D 3.2×. In order:
1. ~~the length reloaded~~ (lenonce-2026-11-10);
2. every digit read masked: the invariant d ∈ [0, B) as a fact;
3. the sign case selected per digit: unswitched;
4. the carry by comparisons: M4, 64-bit digits with `uadd.with.overflow` and i128 (native iN);
5. the size tracked per digit: a sum's from its carry;
6. render: ~~two passes per chunk~~ (here); divide and conquer after Karatsuba and recursive division;
7. **Karatsuba and recursive division** themselves, above measured thresholds, for every product
   and quotient of large numerals.
