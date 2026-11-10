# A digit's range is its table's element type: the read masks removed (item 2)

Date: 2026-11-10. Item 2 of the standing measure's list (printpass-2026-11-10): every digit read was
masked, `if v < 0 { 0 } else { v % B }`, to give the word proofs d ∈ [0, B).

## What it is

A numeral's table is a function [0, len) → [0, B), and its codomain is a set, the table's element
type (ADR 0049: types are sets). As a type, the invariant is a claim checked once at each store and
known at every read. The IR already derives a table's element range from its stores: a build's zero
fill and each store a weak update, joined and widened through loops. So nothing was declared: once
every value stored into a numeral's table is proven in [0, B), every read has the digit's range, and
the mask is redundant.

Two entries stood outside [0, B):
- **the header**, h = ±m, negative for a negative value. It is now **h = 2m + s**, with s the sign and
  s = 0 when m = 0, so h = 0 is still the one zero, and `eq` still compares headers. h < 2³² iff
  m < 2³¹, a machine bound beside max_len: about 8 GB per numeral. `room` now crashes past it too.
- **plain tables' entries in D**: `addv`'s sign entry 1 − c, whose interval reaches below 0, is now a
  case on c. D's quotient digit q − b₀ − b₁ is ≥ 0, since a correction implies q̂ ≥ 1, but that is a
  relation the intervals cannot hold. It is now the total extension max(·, 0), never taken.

## What failed first

**The header made every digit unbounded.** The size `top` is a loop variable rising with i, so
top ≤ cap ≤ 2³¹ is a relation the intervals widen to +∞. The header 2·top + s then carried +∞ into
the table's element fact, and every digit read inherited it: fib had 13 operations unproven and D
11. The fix is the machine's bound stated where the header is written: `signed` crashes for
m ≥ 2³¹, which `room` already makes impossible. That is one comparison per operation, not one per
digit.

## What was measured

Every operation and loop is proven in fib (111 of 111), fact (81 of 81) and D (21,102 of 21,102), and
each answer equals Go's. Best of five, two rounds:

| program | masked reads | element facts | Go, `math/big` |
|---|---|---|---|
| fib(10⁵) | 222.0 / 222.0 ms | **207.7 / 198.0** | 29.9 / 28.5 |
| 2,000 quotients | 271.7 / 275.5 | **243.2 / 241.4** | 103.9 / 93.7 |
| fact(20,000) | 316.3 / 317.3 | 320.6 / 319.2 | 36.5 / 35.3 |

fib is 8–11% faster and D 11%. fact is unchanged: its cost is the product's `mul_small` loop and
render's per-chunk table and its division by the constant 10⁹, neither of which the masks dominated.

**The check**: only `examples/big/`'s four windows programs change, printing what Go prints; the
count went from 6,062 of 6,116 operations to 6,080 of 6,134, every added one proven, the unproven
54 and 24 unchanged.

## The owed list, restated

Windows against Go: fib 7.0×, D 2.5×, fact 8.9×. In order:
1. ~~the length reloaded~~ (lenonce-2026-11-10);
2. ~~every digit read masked~~ (here); the bounds tests i < 0 and i ≥ n remain, since loops read past
   an operand's size. Owed: **the index domain split**, [0, min) ∪ [min, max) ∪ {max}, each piece's
   reads in bounds by its own guard (GMP's `mpn_add_n` then `mpn_add_1`);
3. the sign case selected per digit: unswitched;
4. the carry by comparisons: M4;
5. the size tracked per digit: a sum's from its carry;
6. render's divide and conquer, after 7;
7. Karatsuba and recursive division.
