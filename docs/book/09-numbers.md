# 9. Numbers with a point

[Chapter 7](07-integers.md) said an integer is an integer, and [chapter 8](08-bits.md) said
wrapping is something you write. Both chapters were about whole numbers. Prices, rates,
averages and measurements are not whole.

Nearly every language you have used answers the next question the same way, and the answer is
famous for being wrong:

```
>>> 0.1 + 0.2
0.30000000000000004
```

That is Python, and it is also C, Java, JavaScript, Ruby and Swift. So the question of this
chapter is: **what is `0.1 + 0.2`, and where is a program allowed to lose precision?**

The answer is two sentences again.

> **A number written with a point is exactly the number written.** `+`, `-`, `*` and the
> comparisons are exact.
>
> **Precision is lost in one place, `round`**, which you write, saying onto which set and in
> which direction.

Along the way: why one third has no decimal numeral, how a shop and a bank round differently
and why, how a machine holds 12.50 without any "floating point", what a `double` really is,
and why 0.1 is not one.

The examples are in [code/numbers/](code/numbers/). `go run docs/book/check.go
docs/book/09-numbers.md` re-runs every block. Checked 2026-11-16.

---

## 9.1 The sum

<!-- file docs/book/code/numbers/sum.oro -->
```go
use io
use num

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(0.1 + 0.2 == 0.3))
    io.print_line(num.decimal(0.1 + 0.2, 1))
    io.print_line(num.decimal(0.1 + 0.2, 18))
}
```

<!-- run go docs/book/code/numbers/sum.oro -->
```
1
0.3
0.300000000000000000
```

One tenth and two tenths are three tenths. The first line is the comparison (`bit` is chapter
8's helper: 1 for true, 0 for false). The other two print the sum with one digit after the
point, then with eighteen, and there is no 4 hiding at the end.

Two things here are new.

**`0.1` is a literal for the number one tenth.** Not for "the nearest thing to one tenth the
machine has". The number.

**`num.decimal(x, k)`** gives `x` as text with exactly `k` digits after the point. `num` is a
library module, [lib/num.oro](../../lib/num.oro), forty lines written in the language itself.
`io.print_int` will not print a number with a point, so this chapter prints with
`num.decimal`:

<!-- file docs/book/code/numbers/show.oro -->
```go
use io
use num

export fn main() {
    io.print_line(num.decimal(12.5, 1))
    io.print_line(num.decimal(12.5, 3))
    io.print_line(num.decimal(12.5, 0))
    io.print_line(num.decimal(13.5, 0))
    io.print_line(num.decimal(-0.25, 2))
    io.print_line(num.decimal(7, 2))
}
```

<!-- run go docs/book/code/numbers/show.oro -->
```
12.5
12.500
12
14
-0.25
7.00
```

Asking for more places than the number has pads with zeros. Asking for fewer has to drop
something: 12.5 with no places printed 12, and 13.5 printed 14. Hold on to that pair. One went
down and one went up, and section 8.6 says why.

---

## 9.2 Which numbers these are

A numeral with a point is a whole number and a place for the point: 12.5 is 125 with the point
moved one place left, 0.0825 is 825 with the point moved four.

```
 12.5    =   125 / 10
 0.0825  =   825 / 10000
 7       =     7 / 1
```

So every such number is **an integer divided by a power of ten**. Call the integer the
**numerator** and the count of places the **scale**. These are the numbers of this chapter:
all of them, and nothing else. The compiler's messages call the set **N**, so this chapter
will too.

A literal can be written several ways, and each way names one number:

<!-- file docs/book/code/numbers/literals.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(2.50 == 2.5))
    io.print_int(bit(1e-3 == 0.001))
    io.print_int(bit(12e2 == 1200))
    io.print_int(bit(2.0 == 2))
    io.print_int(bit(0.1 < 0.11))
    io.print_int(bit(-0.5 < 0))
}
```

<!-- run go docs/book/code/numbers/literals.oro -->
```
1
1
1
1
1
1
```

`1e-3` is scientific notation: 1 with the point moved three places left. `12e2` moves it two
places right. `2.50` and `2.5` are the same number, because a trailing zero changes the
numeral and not the number. And `2.0 == 2`: the integers are among these numbers, with
scale 0.

### Adding, subtracting and multiplying stay inside

Add two of these numbers and you get another. Write both over the larger power of ten and add
the numerators, which is what you did in school when you lined up the points:

```
   1.5   =  150 / 100
 + 2.25  =  225 / 100
 ------------------
   3.75  =  375 / 100
```

Multiply two and you get another: multiply the numerators, add the scales.

```
 0.25 × 0.25  =  (25 / 100) × (25 / 100)  =  625 / 10000  =  0.0625
```

<!-- file docs/book/code/numbers/ring.oro -->
```go
use io
use num

export fn main() {
    io.print_line(num.decimal(1.5 + 2.25, 2))
    io.print_line(num.decimal(1 - 0.9, 18))
    io.print_line(num.decimal(1.5 * 1.5, 2))
    io.print_line(num.decimal(0.25 * 0.25, 4))
    io.print_line(num.decimal(0.1 * 0.1 * 0.1, 3))
    io.print_line(num.decimal(1000 * 1.05 * 1.05 * 1.05, 3))
}
```

<!-- run go docs/book/code/numbers/ring.oro -->
```
3.75
0.100000000000000000
2.25
0.0625
0.001
1157.625
```

`1 - 0.9` is 0.1. In Python it prints `0.09999999999999998`. A thousand at 5% a year for three
years is 1157.625, to the last digit.

Because the answers are exact, the laws you learned for arithmetic hold. That sounds like
nothing. It is not: every line below prints 0 in at least one mainstream language.

<!-- file docs/book/code/numbers/laws.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit((0.1 + 0.2) + 0.3 == 0.1 + (0.2 + 0.3)))
    io.print_int(bit(0.1 * (0.2 + 0.3) == 0.1 * 0.2 + 0.1 * 0.3))
    io.print_int(bit(0.1 + 0.1 + 0.1 + 0.1 + 0.1 + 0.1 + 0.1 + 0.1 + 0.1 + 0.1 == 1))
    io.print_int(bit(1.1 * 1.1 == 1.21))
    io.print_int(bit(4.35 * 100 == 435))
}
```

<!-- run go docs/book/code/numbers/laws.oro -->
```
1
1
1
1
1
```

Grouping does not matter. Multiplication distributes over addition. Ten tenths are one. The
last line is a classic payroll bug: in JavaScript `4.35 * 100` is `434.99999999999994`, and a
program that then chops off the fraction pays 434 cents for 4.35.

A set with `+`, `-` and `*` obeying those laws is called a **ring**. The integers are one.
These numbers are another, a bigger one with the integers inside it.

---

## 9.3 Integers are numbers; numbers are not all integers

Every integer is one of these numbers, so an integer may go anywhere a number may. The other
direction needs care:

<!-- file docs/book/code/numbers/no-int.oro -->
```go
use io

export fn main() {
    io.print_int(0.5 + 0.5)
}
```

<!-- build go docs/book/code/numbers/no-int.oro -->
```
build: main: in argument 1 of io.print_int: 0.5 + 0.5 is N, but Int is required here
```

`io.print_int`'s header says its parameter is in `Int`, and that is a claim about every call: the
argument must be an integer, and the compiler checks it where the call is written. A half and a
half *is* 1, and the compiler still refuses. The reason is that this check
happens by looking at the program, not by running it. `+` of two numbers is a number, and
whether it happens to land on an integer depends on the values. For these two literals you
can see it does; for two prices read from a file nobody can.

The way from a number back to an integer is the one operation that may lose something, and
you write it:

<!-- file docs/book/code/numbers/back.oro -->
```go
use io

export fn main() {
    io.print_int(round(Int, 0.5 + 0.5))
    io.print_int(round(Int, 4.35 * 100))
    io.print_int(round(Int, 2.0) << 3)
}
```

<!-- run go docs/book/code/numbers/back.oro -->
```
1
435
16
```

`round(Int, e)` is the integer nearest to `e`. When `e` is already an integer nothing is lost,
as in all three lines here. The third line needs it because `2.0`, although it equals 2, was
written with a point: a numeral with a point is a number, and `<<` wants an integer.

The operations of chapters 7 and 8 that only make sense on integers stay on integers: `/`,
`%`, `mod`, and all the bit operations.

<!-- file docs/book/code/numbers/no-slash.oro -->
```go
use io

export fn main() {
    half = 7 * 0.5
    io.print_int(half / 2)
}
```

<!-- build go docs/book/code/numbers/no-slash.oro -->
```
build: main: in argument 1 of `/`: `*` is N, but Int is required here
```

`/` is the integer quotient of chapter 7, the one where `7 / 2` is 3. It has no meaning on
3.5. What you wanted is the next section.

---

## 9.4 The operation that leaves

Share 1 among 3. Each gets one third. Write one third with a point:

```
 0.3        too small, by 0.0333…
 0.33       too small, by 0.00333…
 0.333      too small, by 0.000333…
```

It never ends, and that is a fact about the number and not about patience. Suppose one third
were an integer over a power of ten, `m / 10ᵏ`. Then `3 × m = 10ᵏ`. But 10ᵏ is 1 followed by
zeros, its digits add up to 1, and a number is divisible by 3 only when its digits add up to
a multiple of 3. So there is no such `m`. **One third is not in N.**

The general rule: a fraction in lowest terms has a finite decimal numeral exactly when its
denominator is made of 2s and 5s only, because those are the factors of 10. So 1/4 = 0.25 and
1/40 = 0.025 are fine, and 1/3, 1/6, 1/7 and 1/9 are not.

So real division is different in kind from `+`, `-` and `*`. Those three take two of our
numbers and give one of our numbers. Division takes two and gives, most of the time,
something *outside*. The language writes it `÷`, typed `\div`, and it is refused on its own:

<!-- file docs/book/code/numbers/no-quotient.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    third = 1 ÷ 3
    io.print_int(bit(third < 0.34))
}
```

<!-- build go docs/book/code/numbers/no-quotient.oro -->
```
build: oro-main: lowering: 1 ÷ 3 is the real quotient, which leaves N: it is written only at the root of a rounding, round(S, a ÷ b) (ADR 0086, Theorem T); a / b is the integer quotient
```

"Which leaves N": the quotient is a perfectly good number, it is only not one of the numbers
a program can hold. To hold it you must say which nearby number you will accept in its place.
That is a rounding, and `÷` may be written only directly inside one:

<!-- file docs/book/code/numbers/split.oro -->
```go
use io
use num

export fn main() {
    share = round(D(2, 0..100), 100 ÷ 3)
    io.print_line(num.decimal(share, 2))
    io.print_line(num.decimal(100 - 3 * share, 2))

    generous = round(D(2, 0..100), 100 ÷ 3, 'up)
    io.print_line(num.decimal(generous, 2))
    io.print_line(num.decimal(100 - 3 * generous, 2))
}
```

<!-- run js docs/book/code/numbers/split.oro -->
```
33.33
0.01
33.34
-0.02
```

A bill of 100.00 split three ways. `D(2, 0..100)` is the set of numbers with at most two
digits after the point, between 0 and 100: amounts in cents. Rounded to the nearest cent each
pays 33.33, and `100 - 3 * share` is one cent that nobody paid. Rounded up each pays 33.34 and
the three have overpaid by two cents.

Notice what the language did and did not do. It did not make the missing cent disappear; no
arithmetic can. It made you write the place where the cent goes missing, and then every
operation after that place is exact again, so the program can *compute* the missing cent and
decide who pays it. In a language where `100 / 3` silently gives 33.333333333333336 the cent
is still missing, and nothing in the program says where.

(This block runs on JavaScript. Today `÷` on exact numbers compiles for JavaScript and the JVM
and is refused on Go and windows with an internal message: README item 37.)

---

## 9.5 Rounding: a grid and a direction

A rounding needs two things said.

**A grid**: the set of numbers you will accept. Picture marks on a ruler.

```
 Int             ──┼─────────────────┼─────────────────┼──
                   2                 3                 4

 D(1, 0..10)     ──┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼─┼──
                   2  .2  .4  .6  .8 3  .2  .4  .6  .8 4
```

`Int` has a mark at every integer. `D(k, lo..hi)` has a mark at every multiple of 10⁻ᵏ from
`lo` to `hi`: `D(1, …)` every tenth, `D(2, …)` every hundredth. A range such as `0..255` is
the integer grid cut to an interval.

**A direction**: when the number falls between two marks, which one. That is the third
argument, a **mode**, written as a quoted name. There are six:

<!-- file docs/book/code/numbers/modes.oro -->
```go
use io

export fn main() {
    io.print_int(round(Int, 2.5, 'down))
    io.print_int(round(Int, 2.5, 'up))
    io.print_int(round(Int, 2.5, 'zero))
    io.print_int(round(Int, 2.5, 'away))
    io.print_int(round(Int, 2.5, 'nearest))
    io.print_int(round(Int, 2.5, 'half_away))
    io.print_line("--")
    io.print_int(round(Int, -2.5, 'down))
    io.print_int(round(Int, -2.5, 'up))
    io.print_int(round(Int, -2.5, 'zero))
    io.print_int(round(Int, -2.5, 'away))
    io.print_int(round(Int, -2.5, 'nearest))
    io.print_int(round(Int, -2.5, 'half_away))
}
```

<!-- run go docs/book/code/numbers/modes.oro -->
```
2
3
2
3
2
3
--
-3
-2
-2
-3
-2
-3
```

| mode | which mark | 2.5 | −2.5 | other names |
|---|---|---|---|---|
| `'down` | the one below | 2 | −3 | floor |
| `'up` | the one above | 3 | −2 | ceiling |
| `'zero` | the one nearer zero | 2 | −2 | truncation |
| `'away` | the one farther from zero | 3 | −3 | |
| `'nearest` | the nearer; exactly halfway, the even one | 2 | −2 | banker's rounding |
| `'half_away` | the nearer; exactly halfway, away from zero | 3 | −3 | what school taught |

The first four ignore how far the number is from each mark. The last two pick the nearer mark
and differ only when the number is exactly halfway. Leave the mode out and you get
`'nearest`:

<!-- file docs/book/code/numbers/near.oro -->
```go
use io

export fn main() {
    io.print_int(round(Int, 2.4))
    io.print_int(round(Int, 2.6))
    io.print_int(round(Int, 2.5))
    io.print_int(round(Int, 3.5))
    io.print_int(round(Int, 2.4, 'away))
    io.print_int(round(Int, 2.4, 'half_away))
    io.print_int(round(Int, 7))
    io.print_int(round(Int, round(Int, 2.6)))
}
```

<!-- run go docs/book/code/numbers/near.oro -->
```
2
3
2
4
3
2
7
3
```

2.4 goes to 2 and 2.6 to 3, as you would expect. 2.5 and 3.5 are both halfway, and go to 2 and
4, the even neighbours. `'away` sends 2.4 to 3, because it does not care that 2 is nearer;
`'half_away` sends it to 2, because 2.4 is not halfway.

The last two lines show two properties every mode has. **A number already on a mark does not
move**: 7 rounds to 7. So rounding twice is rounding once. And **rounding never reorders**: if
x ≤ y then round(x) ≤ round(y). A rounding can make two different numbers equal, and it can
never swap them.

The same six modes work on any grid:

<!-- file docs/book/code/numbers/grid.oro -->
```go
use io
use num

export fn main() {
    io.print_line(num.decimal(round(D(2, 0..10), 3.14159), 2))
    io.print_line(num.decimal(round(D(4, 0..10), 3.14159), 4))
    io.print_line(num.decimal(round(D(1, 0..10), 3.14159, 'up), 1))
    io.print_line(num.decimal(round(D(2, 0..10), 2.675), 2))
    io.print_line(num.decimal(round(D(2, 0..10), 2.665), 2))
    io.print_line(num.decimal(round(D(2, 0..10), 2.665, 'half_away), 2))
}
```

<!-- run go docs/book/code/numbers/grid.oro -->
```
3.14
3.1416
3.2
2.68
2.66
2.67
```

2.675 is halfway between 2.67 and 2.68, and goes to 2.68, whose last digit is even. 2.665 is
halfway between 2.66 and 2.67, and goes to 2.66. In Python `round(2.675, 2)` gives 2.67, and
the documentation has a paragraph explaining that this is not a bug: Python's 2.675 was never
2.675, but a number a hair below it. Here it is 2.675, so the tie is a real tie and the rule
for ties applies.

That also explains 8.1's pair: `num.decimal(x, 0)` rounds to the nearest with ties to even, so
12.5 printed 12 and 13.5 printed 14.

---

## 9.6 Why would anyone round a half to the even side?

School teaches "a half rounds up". It is simple, and it has a flaw you can measure.

Take four amounts that are each exactly halfway: 0.5, 1.5, 2.5, 3.5. Their sum is 8. Round
each first, then add:

<!-- file docs/book/code/numbers/even.oro -->
```go
use io

export fn main() {
    io.print_int(round(Int, 0.5 + 1.5 + 2.5 + 3.5))
    io.print_int(round(Int, 0.5, 'half_away) + round(Int, 1.5, 'half_away)
        + round(Int, 2.5, 'half_away) + round(Int, 3.5, 'half_away))
    io.print_int(round(Int, 0.5) + round(Int, 1.5) + round(Int, 2.5) + round(Int, 3.5))
}
```

<!-- run go docs/book/code/numbers/even.oro -->
```
8
10
8
```

School's rule gives 1 + 2 + 3 + 4 = 10: every half went up, so every error had the same sign
and they piled up. Ties-to-even gives 0 + 2 + 2 + 4 = 8: half the ties went down and half went
up, and the errors cancelled. Over a million transactions the first rule drifts in one
direction and the second does not. That is why banks, statisticians and the IEEE 754 standard
that every processor follows all use ties-to-even, and why it is the default here.

`'half_away` is still there because it is what commerce and most tax law say: a customer
expects 2.665 to become 2.67. Which rule is right is a question about your contract, not about
arithmetic, and that is why it is an argument and not a setting buried in a library.

Direction is not a detail. In 1982 the Vancouver Stock Exchange started an index at 1000 and
recomputed it after every trade, cutting the result to three places instead of rounding it.
Each cut lost a fraction of a thousandth, always downward, thousands of times a day. Twenty-two
months later the index read about 520 while the stocks in it had risen; recomputed properly it
was about 1098. The whole fall was `'zero` where `'nearest` was meant.

---

## 9.7 A rounding must land in its set

The set you round onto has ends, and the result has to be between them:

<!-- file docs/book/code/numbers/no-fit.oro -->
```go
use io
use num

export fn main() {
    io.print_line(num.decimal(round(D(2, 0..1), 5.5), 2))
}
```

<!-- build go docs/book/code/numbers/no-fit.oro -->
```
build: main: oro-main: round(D(2, 0..1), …)'s value inside its set: not proven (the numerator is [550, 550], its set's ends 0..100) (ADR 0086, numbers.md §3)
```

5.5 is on the grid of hundredths and is not between 0 and 1. This is chapter 7's rule again: a
set is a claim, and a claim that is not proven is refused. The message even shows how the
compiler holds the value: as the numerator 550 at scale 2, against a set whose numerators run
from 0 to 100.

Sometimes "outside" should mean "the nearest end". A sound sample, a colour channel, a
percentage: all want values past the end pinned to the end. That is a seventh mode, `'sat`
(for *saturate*), which rounds to the nearest and then clamps:

<!-- file docs/book/code/numbers/sat.oro -->
```go
use io
use num

export fn main() {
    io.print_line(num.decimal(round(D(2, 0..1), 5.5, 'sat), 2))
    io.print_line(num.decimal(round(D(2, 0..1), 0.555, 'sat), 2))
    io.print_int(round(0..255, 300.7, 'sat))
    io.print_int(round(0..255, -4.2, 'sat))
    io.print_int(round(0..255, 127.5, 'sat))
}
```

<!-- run go docs/book/code/numbers/sat.oro -->
```
1.00
0.56
255
0
128
```

`'sat` can never fail, so it has nothing to prove. The other six can, so they do.

---

## 9.8 Integer division was a rounding all along

Chapter 7 said `7 / 2` is 3 and called it "the quotient, rounded toward zero". Now that
sentence can be written in the language:

<!-- file docs/book/code/numbers/quotient.oro -->
```go
use io

export fn main() {
    io.print_int(7 / 2)
    io.print_int(round(Int, 7 ÷ 2, 'zero))
    io.print_int(round(Int, 7 ÷ 2, 'down))
    io.print_int(round(Int, 7 ÷ 2, 'up))
    io.print_int(round(Int, 7 ÷ 2))
    io.print_int(round(Int, 9 ÷ 2))
}
```

<!-- run js docs/book/code/numbers/quotient.oro -->
```
3
3
3
4
4
4
```

**`a / b` is `round(Int, a ÷ b, 'zero)`.** The real quotient 3.5, rounded onto the integers,
toward zero. The other lines are the divisions `/` cannot do: how many boxes of 2 hold 7
things (`'up`, 4), and the nearest whole number of halves, where 3.5 and 4.5 both go to the
even 4.

Chapter 8 met the same thing from another side: `a >> 1` halves and rounds *down*, so
`-7 >> 1` is −4 where `-7 / 2` is −3. Two operations, one real quotient, two modes. Languages
disagree about integer division of negative numbers (Python floors, C truncates) because each
picked one mode and gave it the only symbol.

---

## 9.9 Where you round is part of the program

A shop sells three items at 0.99 each, and the sales tax is 8.25%.

<!-- file docs/book/code/numbers/tax.oro -->
```go
use io
use num

fn tax(amount) { round(D(2, 0..10^6), amount * 0.0825, 'half_away) }

export fn main() {
    price = 0.99
    io.print_line(num.decimal(price * 0.0825, 6))
    io.print_line(num.decimal(tax(price), 2))
    io.print_line(num.decimal(tax(price) + tax(price) + tax(price), 2))
    io.print_line(num.decimal(tax(price + price + price), 2))
}
```

<!-- run go docs/book/code/numbers/tax.oro -->
```
0.081675
0.08
0.24
0.25
```

The exact tax on one item is 0.081675, and nobody can pay that, so it is rounded to the cent:
0.08. Now there are two honest ways to tax three items. Tax each line and add: 0.24. Add the
lines and tax the total: 0.25. Both are "8.25%, rounded to the cent". They differ by a cent,
and real jurisdictions legislate which one is meant.

In a language where every operation rounds a little, this difference is buried under sixteen
digits of noise and you find it in an audit. Here the two programs are visibly two programs:
the `round` is in a different place.

The same with interest. Section 8.2 computed 1000 at 5% for three years exactly. A bank
credits interest once a year, in whole cents:

<!-- file docs/book/code/numbers/interest.oro -->
```go
use io
use num

fn year(balance) { round(D(2, 0..10^6), balance * 1.05) }
fn year_up(balance) { round(D(2, 0..10^6), balance * 1.05, 'half_away) }

export fn main() {
    io.print_line(num.decimal(1000 * 1.05 * 1.05 * 1.05, 6))
    io.print_line(num.decimal(year(year(year(1000))), 2))
    io.print_line(num.decimal(year_up(year_up(year_up(1000))), 2))
}
```

<!-- run go docs/book/code/numbers/interest.oro -->
```
1157.625000
1157.62
1157.63
```

After three years the exact balance is 1157.625, halfway between two cents, and the two rules
for ties give the two neighbours.

---

## 9.10 How a machine holds 12.50

There is no "decimal chip" in your computer. So what did those programs run on? Look at one.

<!-- file docs/book/code/numbers/dearer.oro -->
```go
fn money(cents ∈ 0..10^9) { cents * 0.01 }

export fn dearer(a ∈ 0..10^9, b ∈ 0..10^9) (Bool) {
    money(a) + 0.005 < money(b) * 1.5
}
```

Two amounts arrive as whole cents. `money` turns cents into the number they mean (1250 becomes
12.50). The question is whether `a` plus half a cent is below one and a half times `b`.

<!-- gen go docs/book/code/numbers/dearer.oro -->
```go
func NumbersDearer(v0 int, v1 int) bool {
    v24 := (v0 * 1)
    v27 := (v24 * 10)
    v28 := (v27 + 5)
    v30 := (v1 * 1)
    v32 := (v30 * 15)
    v33 := (v28 < v32)
    return v33
}
```

Integers, and nothing else. No point anywhere, no library, no allocation. Read it as
8.2's school arithmetic:

- `money(a)` is `a` hundredths. The numerator is `a` itself, at scale 2.
- `0.005` is 5 thousandths, scale 3. To add, both go over a thousand: `a * 10` thousandths
  plus 5.
- `money(b) * 1.5` multiplies numerators and adds scales: `b * 15` at scale 3.
- Both sides are now thousandths, so comparing the numbers is comparing the numerators.

**A number is held as its numerator, an ordinary integer, and its scale is known when
compiling.** The scale never exists at run time. This is the oldest trick in business
computing: COBOL has done it since 1959, SQL's `NUMERIC(12, 2)` does it, and every programmer
who was told "store money in cents" did it by hand. The difference is that here the compiler
keeps the scales, and you write `0.005` instead of remembering that this variable is in
thousandths and that one in hundredths.

A rounding becomes an integer division. Eight percent of an amount in cents, rounded down to
the cent:

<!-- file docs/book/code/numbers/cents.oro -->
```go
export fn tax(cents ∈ 0..10^9) (0..10^9) {
    round(Int, cents * 0.08, 'down)
}
```

<!-- gen go docs/book/code/numbers/cents.oro -->
```go
func NumbersTax(v0 int) int {
    v10 := (v0 * 8)
    v12 := (func(a, m int) int { r := a % m; if r < 0 { r += m }; return r }(v10, 100))
    v13 := (v10 - v12)
    v14 := (v13 / 100)
    return v14
}
```

`cents * 0.08` is the numerator `cents * 8` at scale 2. Rounding it down onto the integers is
dividing by 100 and keeping the floor: take off the remainder, then divide.

### The numerators and the machine's readings

A numerator is an integer, so everything chapter 7 said about integers applies to it. Each
operation on numerators is held in the first of the machine's readings that holds it, the word, the
unsigned word or the double word, and past them as the integer itself (chapter 7, §7.9):

<!-- file docs/book/code/numbers/two-scales.oro -->
```go
export fn small(n ∈ 0..10^18) (Bool) {
    n * 0.0825 * 0.0825 < 3
}
```

<!-- portable docs/book/code/numbers/two-scales.oro -->
```
portable to go, js, java, windows
```

`n * 0.0825` has the numerator `n * 825`, up to 8.25 × 10²⁰, and Go's word stops near
9.2 × 10¹⁸; times 0.0825 again, 6.8 × 10²³. Both lie in the double word, which holds integers below
2¹²⁷ ≈ 1.7 × 10³⁸, and only the comparison is kept, so Go, the JVM and Windows compute them there.
JavaScript has no 64-bit integer, and computes them over `BigInt`. Two numbers below 10¹⁸
multiplied pass even the double word:

<!-- file docs/book/code/numbers/past-e.oro -->
```go
export fn small(n ∈ 0..10^18, m ∈ 0..10^18) (Bool) {
    n * m * 0.0825 < 3
}
```

<!-- gen go docs/book/code/numbers/past-e.oro -->
```go
note: numbers-small: 2 operation(s) past every word and the double word, held exactly: n * m; n * m * 0.0825
note: numbers-small: 3 operation(s) in arbitrary precision
…
```

The numerator of `n * m * 0.0825` reaches 8.25 × 10³⁸, past every reading, so it is held as the
integer it is, and the note names it; the comparison is a boolean again. The note calls them
"operations" on numerators because that is what they are.

So a long chain of multiplications grows the scale and the numerator together, and sooner or
later you must round to bring the scale back down. That is not a quirk of this language. It
is what "exact" costs: the digits have to be somewhere.

### A set of numbers is a claim

`D(2, 0..10^9)` can be given a name and used wherever a set can: on a parameter, on a result.

<!-- file docs/book/code/numbers/money.oro -->
```go
set Money = D(2, 0..10^9)

fn add(a ∈ Money, b ∈ Money) (D(2, 0..2 * 10^9)) { a + b }

export fn over(cents ∈ 0..1000) (Bool) {
    add(cents * 0.01, 0.5) > 3
}
```

<!-- gen go docs/book/code/numbers/money.oro -->
```go
func NumbersOver(v0 int) bool {
    v10 := (v0 * 1)
    v13 := (v10 + 50)
    v16 := (v13 > 300)
    return v16
}
```

`cents * 0.01` has two places and is at most 10, so it is in `Money`; 0.5 is in `Money`; the
sum has two places and is at most 2 × 10⁹. All of it is checked while compiling and none of it
is in the Go. Pass something with three places and the claim fails:

<!-- file docs/book/code/numbers/no-places.oro -->
```go
set Money = D(2, 0..10^9)

fn twice(a ∈ Money) { a + a }

export fn over(mills ∈ 0..1000) (Bool) {
    twice(mills * 0.001) > 3
}
```

<!-- gen go docs/book/code/numbers/no-places.oro -->
```
gen: numbers-over: twice's parameter a is declared in D(2, 0..1000000000), and at a call the argument mills * 0.001 is not proven in it
```

A thousandth is not an amount of money. The fix is to say how the thousandths become cents:
`round(Money, mills * 0.001, 'half_away)`, or whichever mode the contract says.

---

## 9.11 Doubles

Exact decimals are right for money, quantities, rates: anything a person wrote down. They are
the wrong tool for a physics simulation, a sound filter or a neural network. Those multiply
millions of measured values, the scale would grow without end, and the inputs were only
accurate to a few digits anyway. For that work every processor has a second kind of
arithmetic built in, standardized in 1985 as IEEE 754. Its common format is the **double**.

Here is what a double is, in this chapter's terms. **A double is a number of the form
m × 2ᵉ, where m is an integer of at most 53 bits** and e is between about −1074 and 971. It
is chapter 8's binary numeral with a binary point that may sit anywhere. Compare:

```
 a decimal of scale k    m / 10ᵏ      the point is fixed when compiling
 a double                m × 2ᵉ       m has 53 bits; e travels with the value
```

Because e travels with the value, the point *floats*: that is "floating point". It lets one
format hold the mass of an electron and the mass of the sun, each to about sixteen significant
digits.

Now the fact that makes this chapter hang together: **every double is one of our numbers.**
A half is 0.5, a quarter is 0.25, an eighth is 0.125: dividing by 2 always terminates in
decimal, because 2 divides 10. So the doubles are not a different kind of thing. They are a
**set** of numbers, a grid like `Int` and `D(2, …)`, with marks that are close together near
zero and far apart for large values. The language calls the set `F64`.

And a grid is something you round onto:

<!-- file docs/book/code/numbers/double.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(round(F64, 0.5) == 0.5))
    io.print_int(bit(round(F64, 0.1) == 0.1))
    io.print_int(bit(round(F64, 0.1) == 0.1000000000000000055511151231257827021181583404541015625))
    io.print_int(bit(round(F64, 2^53) == 2^53))
    io.print_int(bit(round(F64, 2^53 + 1) == 2^53))
}
```

<!-- run go docs/book/code/numbers/double.oro -->
```
1
0
1
1
1
```

Line by line:

1. 0.5 is a double (1 × 2⁻¹), so rounding it changes nothing.
2. **0.1 is not a double.** One tenth is 1/(2 × 5), and no power of two has a factor of 5 to
   cancel. It is the same argument that showed one third is not a decimal, one level down: in
   base two, one tenth is 0.000110011001100… forever.
3. The double nearest to 0.1 is that 55-digit number, exactly. It is a little more than a
   tenth. Every language that prints `0.1` for it is rounding again while printing.
4. 2⁵³ is a double.
5. 2⁵³ + 1 is not: it needs 54 bits. From 2⁵³ upward the doubles are two apart, and the tie
   goes to the even one. This is the 2⁵³ of chapter 7, where JavaScript's word ended.

### IEEE arithmetic is "round after every operation"

When a C program computes `0.1 + 0.2` with doubles it does three roundings: 0.1 to the nearest
double, 0.2 to the nearest double, and their exact sum to the nearest double. Written out,
that is `round(F64, round(F64, 0.1) + round(F64, 0.2))`. Writing a rounding around every
operation would be unbearable, so there is a block that does it:

> `round(F64) { body }` rounds onto `F64` every `+`, `-`, `*` and `÷` written inside `body`,
> and every literal that is not a double.

<!-- file docs/book/code/numbers/ieee.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(round(F64) { 0.1 + 0.2 } == 0.3))
    io.print_int(bit(round(F64) { 0.1 + 0.2 } == 0.3000000000000000444089209850062616169452667236328125))
    io.print_int(bit(round(F64) { 0.1 + 0.2 } == round(F64, round(F64, 0.1) + round(F64, 0.2))))
    io.print_int(bit(round(F64) { (0.1 + 0.2) + 0.3 } == round(F64) { 0.1 + (0.2 + 0.3) }))
    io.print_int(bit(round(F64) { 0.5 + 0.25 } == 0.75))
}
```

<!-- run go docs/book/code/numbers/ieee.oro -->
```
0
1
1
0
1
```

There is the famous answer, reproduced on purpose. Inside the block, 0.1 + 0.2 is not 0.3; it
is exactly 0.3000000000000000444…, which Python abbreviates to `0.30000000000000004`. The
third line confirms the block means the three roundings. The fourth shows that with a rounding
after every step, grouping matters: (a + b) + c and a + (b + c) round at different places and
land on different doubles. The fifth shows when doubles are exact: a half plus a quarter needs
no rounding at all.

So the famous wrong answer is not wrong. It is the right answer to a different question, "what
do three roundings give?", and the trouble in other languages is only that the roundings are
invisible. Outside the block the same literals are exact (8.2's `laws.oro` printed 1 for the
same grouping test). The block's braces show you exactly which operations are approximate.

In 1991 a Patriot missile battery in Dhahran failed to intercept an incoming missile, and 28
soldiers died. Its clock counted tenths of a second and multiplied the count by a binary
approximation of 0.1. The error per tick was about a ten-millionth of a second. After 100
hours of continuous running it had grown to a third of a second, in which the target moved
more than half a kilometre. One tenth is not a binary number.

### Doubles at run time

Everything above was folded while compiling, exactly. Here is a function whose doubles arrive
at run time.

<!-- file docs/book/code/numbers/no-overflow.oro -->
```go
export fn add(x ∈ F64, y ∈ F64) (F64) {
    round(F64, x + y)
}
```

<!-- gen go docs/book/code/numbers/no-overflow.oro -->
```
gen: numbers-add: numbers-add: round(F64, #0.0 + #0.1) may overflow F64: its term within ±(2^1024 - 2^970) is not proven (it is in [-Inf, +Inf]); narrow the operands' sets, F64 ∩ [lo, hi] as set { x ∈ F64 : lo <= x && x <= hi }, or bound a loop's trip count (ADR 0086, numbers.md §3, §5)
```

Refused, and for 8.7's reason: a rounding must land in its set. `F64` has a largest element,
about 1.8 × 10³⁰⁸. Add two doubles near it and the sum is past the end. IEEE's answer is a
special value called infinity. This language has no infinity: `F64` is a set of numbers, and
infinity is not a number. So the sum must be proven to fit, and the message says how (`#0.0`
and `#0.1` are the compiler's names for the two parameters).

Say how large the inputs really are, with chapter 7's set-builder notation:

<!-- file docs/book/code/numbers/mean.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn mean(x ∈ Small, y ∈ Small) (F64) {
    round(F64) { (x + y) ÷ 2 }
}
```

<!-- gen go docs/book/code/numbers/mean.oro -->
```go
func NumbersMean(v0 float64, v1 float64) float64 {
    v2 := (v0 + v1)
    v4 := (v2 / 2.0)
    return v4
}
```

<!-- gen js docs/book/code/numbers/mean.oro -->
```js
export function numbersMean(v0, v1) {
    return ((v0 + v1) / 2);
}
```

<!-- gen windows docs/book/code/numbers/mean.oro -->
```llvm
define double @numbers_mean(double %v0, double %v1) #0 {
entry:
  %v2 = fadd double %v0, %v1
  %v4 = fdiv double %v2, 0x4000000000000000
  ret double %v4
}
```

`Small` is the doubles between −10¹⁰⁰ and 10¹⁰⁰. Two of them add to at most 2 × 10¹⁰⁰, far
inside `F64`, so the overflow is proven impossible and nothing checks it at run time. What is
left is the host's own `+` and `/` on its own doubles: one machine instruction each. Each
target declares that its `+` on doubles *is* "the exact sum, rounded to the nearest double",
and the compiler matches your `round(F64, x + y)` against that declaration. A rounding in the
source is one instruction in the output, or it is refused.

Here `÷` runs on every target: the processor divides and rounds in one step.

### Comparing a double with a decimal is exact

<!-- file docs/book/code/numbers/below.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn below(x ∈ Small) (Bool) { x < 0.1 }
```

<!-- gen go docs/book/code/numbers/below.oro -->
```go
func NumbersBelow(v0 float64) bool {
    v2 := (v0 < 0.1)
    return v2
}
```

The `0.1` in this program is one tenth, the real one, and `x` is a double. The host cannot
compare a double with one tenth, because it cannot hold one tenth. One tenth lies between two
doubles, `0.09999999999999999` and the double written `0.1`, which is a little above it, and no
double lies between them. So "x is below one tenth" is the same as "x is below the double
`0.1`", and the compiler writes that, the host's own `<`. It is `<=` where a C programmer's
reading and this one part: C's `x <= 0.1` is true for the double nearest to 0.1, which lies
above one tenth, and `x <= 0.1` here is false for it, and is written `x < 0.1`. Every
comparison with a decimal the host cannot hold becomes a comparison with the one double just
above it, so a comparison and its negation use the same constant.

### An integer becomes a double when it is one

<!-- file docs/book/code/numbers/half.oro -->
```go
export fn half(n ∈ 0..1000) (F64) {
    round(F64, n ÷ 2)
}
```

<!-- gen go docs/book/code/numbers/half.oro -->
```go
func NumbersHalf(v0 int) float64 {
    v9 := (float64(v0))
    v11 := (v9 / 2.0)
    return v11
}
```

An integer up to 1000 is a double, so `float64(v0)` loses nothing. Past 2⁵³ that stops being
true (remember 2⁵³ + 1), and the compiler stops with it:

<!-- file docs/book/code/numbers/no-exact.oro -->
```go
export fn half(n ∈ 0..2^60) (F64) {
    round(F64, n ÷ 2)
}
```

<!-- gen go docs/book/code/numbers/no-exact.oro -->
```
gen: numbers-half: numbers-half: an integer made a double in round(F64, #0.0 ÷ 2) must be within ±2^53, where it is one: not proven (it is in [0, 1.152921504606847e+18]) (ADR 0086, numbers.md §3, §5)
```

### What doubles may not do

**Arithmetic on doubles must be rounded.** The sum of two doubles is usually not a double, and
nothing at run time can hold it:

<!-- file docs/book/code/numbers/no-interior.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn positive(x ∈ Small, y ∈ Small) (Bool) { x + y > 0 }
```

<!-- gen go docs/book/code/numbers/no-interior.oro -->
```
gen: numbers-positive: numbers-positive: x + y is a number of N made from doubles and not rounded: no representation holds it yet, an exact interior is step 2 (numbers.md §5, §9); round it, round(F64, …), or write round(F64) { … }
```

With decimals the compiler could hold the exact sum, because it knew the scale. A double's
exponent is only known at run time, so the exact sum has no fixed size. Write the rounding, or
the block.

**Only to the nearest.** The directed modes exist on decimals and integers. On doubles the
processor keeps the mode in a hidden global switch, and no target here flips it:

<!-- file docs/book/code/numbers/no-down.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn floor_sum(x ∈ Small, y ∈ Small) (F64) {
    round(F64, x + y, 'down)
}
```

<!-- gen go docs/book/code/numbers/no-down.oro -->
```
gen: numbers-floor-sum: numbers-floor-sum: round(F64, #0.0 + #0.1), 'down: no target realizes a rounding onto F64 but to the nearest: IEEE 754's directed modes are the host's dynamic state (ADR 0086, numbers.md §6.2)
```

**One operation per rounding.** `a * b + c` rounded once is a different number from `a * b`
rounded, plus `c`, rounded again. Some processors have an instruction for the first, a *fused
multiply-add*, and some compilers use it without telling you, so the same C program can print
different digits on two machines. Here the two are two different programs, and the one-rounding
form is refused until a target declares the instruction:

<!-- file docs/book/code/numbers/no-fused.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn fused(a ∈ Small, b ∈ Small, c ∈ Small) (F64) {
    round(F64, a * b + c)
}
```

<!-- gen go docs/book/code/numbers/no-fused.oro -->
```
gen: numbers-fused: numbers-fused: round(F64, #0.0 * #0.1 + #0.2): no declaration of target go realizes it (numbers.md §6.2): a target realizes round(F64, t) by a declaration whose ensure is r == round(F64, t), each operand a double, an integer or a constant that is a double; a rounding inside it is its own operation, and an exact interior (a term of doubles rounded once) is step 2
```

The block form rounds each operation, and it compiles:

<!-- file docs/book/code/numbers/dot.oro -->
```go
set Small = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }

export fn dot2(a ∈ Small, b ∈ Small, c ∈ Small, d ∈ Small) (F64) {
    round(F64) { a * b + c * d }
}
```

<!-- gen go docs/book/code/numbers/dot.oro -->
```go
func NumbersDot2(v0 float64, v1 float64, v2 float64, v3 float64) float64 {
    v4 := (float64(v0 * v1))
    v5 := (float64(v2 * v3))
    v6 := (v4 + v5)
    return v6
}
```

Look at `float64(v0 * v1)`. It converts a `float64` to `float64`, which looks pointless. Go's
specification allows the compiler to fuse `x*y + z` into one instruction *unless* the product
is explicitly converted. The conversion is there to forbid the fusion, so that three roundings
in the source are three roundings on every machine.

**No NaN, no infinity, no negative zero.** IEEE doubles carry three kinds of value that are
not numbers: two infinities, a second zero with a minus sign, and "not a number", NaN, which
is what 0 ÷ 0 gives and which is not equal even to itself. `F64` has none of them. An
operation that would produce one has an obligation instead: a sum that might overflow, a
divisor that might be zero. Equality on `F64` is equality of numbers, so `x == x` is always
true and the laws of chapter 5 survive.

One more limit, which is only a gap: `num.decimal` prints exact decimals, and nothing in the
library prints a double yet. This section compared doubles with exact numerals instead, which
tells you more: `== 0.3000000000000000444089209850062616169452667236328125` is the whole
truth, and a printed `0.30000000000000004` is a summary of it.

---

## 9.12 Which one to use

| | decimals, `D(k, lo..hi)` | doubles, `F64` |
|---|---|---|
| held as | an integer numerator; the scale is known when compiling | 53 bits and an exponent, in one machine register |
| `+ - *` | exact | written under a rounding |
| `0.1` | is one tenth | must be rounded; the nearest double is not one tenth |
| grows when multiplied | yes: scales add, until you round | no: always 53 bits |
| range | what the word holds | from about 10⁻³⁰⁸ to 10³⁰⁸ |
| rounding modes | seven | to the nearest |
| use for | money, counts with fractions, rates, anything a person typed | measurements, geometry, signal processing, statistics |

The rule of thumb: if the values were *written by someone* (a price list, a tax table), they
are decimals and people will notice a wrong cent. If they were *measured* (a sensor, a
position), they were never exact, and doubles give sixteen digits where the sensor gave four.

---

## 9.13 What other languages do

| | Oroboros | C, Java, JavaScript, Python | Go |
|---|---|---|---|
| the literal `0.1` | one tenth | the double nearest to one tenth | one tenth as a constant; a double once stored in a variable |
| `0.1 + 0.2 == 0.3` | true | false | true for constants, false for variables |
| mixing an integer and a fraction | allowed; integers are numbers | converted silently to a double | a compile error without a conversion |
| `1 / 3` on non-integers | written `÷`, inside a rounding | 0.3333333333333333, silently | 0.3333333333333333, silently |
| exact decimals | the default | a library: `BigDecimal`, `decimal.Decimal`; allocated objects, slower | a library |
| rounding a half | you say; ties-to-even by default | differs by language and by function | away from zero (`math.Round`) |
| overflow of a double | must be proven impossible | becomes infinity | becomes infinity |
| 0 ÷ 0 | the divisor must be proven nonzero | NaN, which is not equal to itself | NaN |

Three notes on that table.

Go's constants are the nearest relative: Go computes constant expressions exactly, so
`0.1 + 0.2 == 0.3` is true there too, until the values are stored in variables. This language
extends that idea from constants to every value whose scale it can track.

"Differs by function" is literal. Python's `round(2.5)` is 2 (ties to even) and JavaScript's
`Math.round(2.5)` is 3; `Math.round(-2.5)` is −2 (a half goes *up*, toward +∞), where C's
`round(-2.5)` is −3 (away from zero). Four functions called "round", three rules.

Java's `BigDecimal` agrees with this chapter on division: `new BigDecimal(1).divide(new
BigDecimal(3))` throws an exception, "non-terminating decimal expansion", unless you pass a
rounding mode. It is the same rule, found at run time instead of when compiling.

---

## What to remember

- **A numeral with a point is exactly that number.** The numbers are the integers divided by
  powers of ten; the compiler calls the set N. The integers are among them.
- **`+`, `-`, `*`, `==`, `<` are exact.** Grouping does not matter, ten tenths are one.
- **`/`, `%`, `mod` and the bit operations are for integers only.** `round(Int, e)` goes from
  a number to an integer.
- **Real division is `÷`** (typed `\div`). It leaves the set (one third has no decimal
  numeral), so it is written only directly inside a `round`.
- **`round(S, e, mode)` is the one operation that loses anything.** `S` is a grid cut to an
  interval: `Int`, a range, `D(k, lo..hi)` (k places), or `F64`.
- **Six modes**: `'down`, `'up`, `'zero`, `'away`, `'nearest` (the default; a tie goes to the
  even mark), `'half_away` (a tie goes away from zero). A seventh, `'sat`, clamps to the set's
  ends.
- **The result must be proven inside `S`**, except with `'sat`.
- **`a / b` is `round(Int, a ÷ b, 'zero)`.**
- **Where the `round` stands is part of the program**: tax per line and tax on the total are
  different programs.
- **A decimal is held as an integer numerator** at a scale fixed when compiling. The numerator
  must fit the word like any integer.
- **A double is m × 2ᵉ with 53 bits of m; `F64` is the set of them.** 0.1 is not in it.
- **`round(F64) { … }` is IEEE arithmetic**: every operation inside is rounded to the nearest
  double. Outside the block nothing is.
- **No NaN, no infinity, no −0.** Overflow and a zero divisor are things to prove.

Next: [chapter 10](10-text.md), Text. What a string is made of, and why its length is a harder question than
it looks.
