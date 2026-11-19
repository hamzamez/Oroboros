# 7. Bits

[Chapter 6](06-integers.md) said an integer is an integer, and spent its length keeping the
machine out of the picture: the word was a place an answer either fits or does not. But a great
deal of real programming is *about* the machine's picture. A colour is three bytes packed in
one number. A file's permissions are three switches. A checksum is arithmetic that throws the
high digits away on purpose.

So the question of this chapter is: **if an integer is not a row of 64 bits, what do `&`, `|`,
shifts, and arithmetic that wraps mean?**

The answer is two sentences.

> **The bits are the integer's own**: every integer has a binary numeral, and the bit operations
> work on that numeral, however long it is.
>
> **Wrapping is an operation you write**, `mod 2ⁿ`, and never something that happens to you.

Along the way: positional notation, Boole's algebra again (now sixty-four copies at once), how
a machine writes a negative number, sets as integers, the circuit that adds, clock arithmetic,
and a hash function used in real systems, in one line.

The examples are in [code/bits/](code/bits/). `go run docs/book/check.go docs/book/07-bits.md`
re-runs every block. Checked 2026-11-16.

---

## 7.1 An integer, written in twos

You write numbers in base ten: 305 means 3 hundreds, 0 tens and 5 ones, and the places are
powers of ten. The same number can be written with any base. In base two the places are powers
of two and the only digits are 0 and 1:

```
 13  =  8 + 4 + 0 + 1  =  1·2³ + 1·2² + 0·2¹ + 1·2⁰  =  1101 in base two
```

A digit of a base-two numeral is a **bit** (*binary digit*). Bits are numbered from the right,
starting at 0: bit 0 of 13 is 1, bit 1 is 0, bits 2 and 3 are 1.

Nothing here is about computers. Thirteen pebbles are thirteen pebbles whether you write
"13", "XIII" or "1101". Machines use base two because a wire that is on or off is easy to build
and a wire with ten distinguishable levels is not.

The language has no base-two literal. It has base sixteen, **hexadecimal**, which is base two
with the bits read four at a time:

| hex digit | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | A | B | C | D | E | F |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| bits | 0000 | 0001 | 0010 | 0011 | 0100 | 0101 | 0110 | 0111 | 1000 | 1001 | 1010 | 1011 | 1100 | 1101 | 1110 | 1111 |

So `0xD` is 1101, `0xFF` is 1111 1111, eight ones, and `0x3C` is 0011 1100. Reading hex is
reading bits.

<!-- file docs/book/code/bits/binary.oro -->
```go
use io

export fn main() {
    io.print_int(0xD)
    io.print_int(8 + 4 + 1)
    io.print_int(2^3 + 2^2 + 2^0)
    io.print_int(0xFF)
    io.print_int(2^8 - 1)
    io.print_int(0x100)
    io.print_int(0xFFFFFFFF)
    io.print_int(2^32 - 1)
}
```

<!-- run go docs/book/code/bits/binary.oro -->
```
13
13
13
255
255
256
4294967295
4294967295
```

Two facts worth carrying in your head, both visible above:

- **n ones in a row is 2ⁿ − 1.** `0xFF` is 2⁸ − 1, `0xFFFFFFFF` is 2³² − 1. (In base ten: 999 is
  10³ − 1.)
- **A 1 followed by n zeros is 2ⁿ.** `0x100` is 2⁸.

## 7.2 Three operations, one position at a time

Take two numerals and line them up. Three operations combine them **column by column**, each
column on its own, with no carry from one to the next:

| | name | a column of the result is 1 when | |
|---|---|---|---|
| `a & b` | and | both bits are 1 | |
| `a \| b` | or | at least one bit is 1 | |
| `a ⊕ b` | exclusive or, "xor" | exactly one bit is 1: the bits differ | typed `\oplus` |

```
      1100   (12)          1100   (12)          1100   (12)
    & 1010   (10)        | 1010   (10)        ⊕ 1010   (10)
    ------               ------               ------
      1000   (8)           1110   (14)          0110   (6)
```

<!-- file docs/book/code/bits/ops.oro -->
```go
use io

export fn main() {
    io.print_int(12 & 10)
    io.print_int(12 | 10)
    io.print_int(12 ⊕ 10)
    io.print_int(12 \oplus 10)
    io.print_int(0xF0 & 0x3C)
    io.print_int(0xF0 | 0x3C)
    io.print_int(0xF0 ⊕ 0x3C)
}
```

<!-- run go docs/book/code/bits/ops.oro -->
```
8
14
6
6
48
252
204
```

<!-- run js docs/book/code/bits/ops.oro -->
```
8
14
6
6
48
252
204
```

- `⊕` is the mathematician's symbol for this operation. If your keyboard has no `⊕`, type
  `\oplus`; the two are the same token.
- **`^` is not xor.** In C, Java, Go and JavaScript it is. Here `^` is a power (§6.1), because
  that is what it means on paper, and `2^32` appears in programs far more often than an
  exclusive or does.
- Check the last three by reading the hex: `F0` is 1111 0000, `3C` is 0011 1100. And: 0011 0000,
  which is `0x30`, 48. Or: 1111 1100, `0xFC`, 252. Xor: 1100 1100, `0xCC`, 204.

**This is chapter 4's algebra.** A bit is a `Bool` written 0 and 1. `&` on one column is `&&`,
`|` is `||`, and `⊕` is "differs", `!=` on booleans. A 64-bit `&` is sixty-four `&&`s computed
at once, by one machine instruction. George Boole wrote the algebra down in 1854; in 1937
Claude Shannon's master's thesis observed that circuits of switches obey it, and that is the
reason a computer is built out of it.

So every law of chapter 4 holds here, and a few new ones for `⊕`:

<!-- file docs/book/code/bits/laws.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    a = 0xCAFE
    b = 0xBEEF
    io.print_int(bit(a & b == b & a))
    io.print_int(bit(a | a == a))
    io.print_int(bit(a & 0 == 0))
    io.print_int(bit(a | 0 == a))
    io.print_int(bit(a ⊕ 0 == a))
    io.print_int(bit(a ⊕ a == 0))
    io.print_int(bit(a ⊕ b ⊕ b == a))
    io.print_int(bit(a + b == (a ⊕ b) + 2 * (a & b)))
    io.print_int(bit(a | b == (a ⊕ b) + (a & b)))
}
```

<!-- run go docs/book/code/bits/laws.oro -->
```
1
1
1
1
1
1
1
1
1
```

(`bit` turns a `Bool` into 0 or 1 so it can be printed; it is the only helper in the chapter.)

- `a ⊕ a == 0` and `a ⊕ 0 == a`: xor-ing with something twice undoes it. `a ⊕ b ⊕ b` is `a`.
  Every cipher that scrambles data with a key and unscrambles it with the same key leans on
  this line.
- **`a + b == (a ⊕ b) + 2 * (a & b)`** is how a machine adds. In one column, 1 + 1 is 10 in base
  two: write 0, carry 1. The digit written is 1 exactly when the bits differ, which is `⊕`. The
  carry is 1 exactly when both are 1, which is `&`, and it belongs one column to the left,
  which is the `2 *`. An adder circuit is this identity in silicon, applied until no carries
  are left.

### How the operators group

<!-- file docs/book/code/bits/precedence.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(1 | 2 & 3)
    io.print_int(1 + 2 << 3)
    io.print_int(1 + (2 << 3))
    io.print_int(bit(6 & 3 == 2))
}
```

<!-- run go docs/book/code/bits/precedence.oro -->
```
3
17
17
1
```

- `&` and the shifts `<<` `>>` (§7.4) group like `*`: they bind tighter than `|` and `⊕`, which
  group like `+`. So `1 | 2 & 3` is `1 | (2 & 3)`, and `1 + 2 << 3` is `1 + (2 << 3)`. The
  analogy is exact: `&` is the multiplication of single bits, and a shift is a multiplication.
- All of them bind tighter than a comparison, so `6 & 3 == 2` asks whether `6 & 3` is 2.
- **If you come from C, both of the last two lines are traps there.** C reads `1 + 2 << 3` as
  `(1 + 2) << 3`, which is 24, and `x & mask == 0` as `x & (mask == 0)`, a mistake its own
  designers regretted in print. The grouping here is Go's, which fixed both.

## 7.3 Negative numbers have infinitely many ones

What are the bits of −1?

Ask what −1 has to do: adding 1 to it must give 0. Try a numeral that is all ones, as far left
as you care to look:

```
      …1111 1111
    +          1
    ------------
      …0000 0000
```

1 + 1 is 10: write 0, carry 1. The carry meets the next 1: write 0, carry 1. And so on forever;
the carry runs off to the left and every column you can point at is 0. So **−1 is the numeral
that is ones all the way**. The same reasoning gives every negative number: −2 is `…1110`, −13
is `…1111 0011`.

That gives the rule. **Every integer's numeral is eventually constant to the left: zeros
forever for a number that is not negative, ones forever for one that is.** A word in a machine
stores the rightmost 64 columns of it, and the 64th tells you which kind you have. That
encoding is called *two's complement*, and it is usually taught as a trick about 64-bit
registers. It is simpler than that: it is what subtraction does to numerals when you let them
be as long as they like.

The fourth bit operation, `~a`, **not**, flips every column, all infinitely many:

<!-- file docs/book/code/bits/negative.oro -->
```go
use io

export fn main() {
    io.print_int(~0)
    io.print_int(~12)
    io.print_int(~(-13))
    io.print_int(-1 & 0xFF)
    io.print_int(-256 & 0xFF)
    io.print_int(-2 & 7)
    io.print_int(-1 | 12)
    io.print_int(-1 ⊕ 12)
}
```

<!-- run go docs/book/code/bits/negative.oro -->
```
-1
-13
12
255
0
6
-1
-13
```

- `~0` flips zeros-forever into ones-forever: −1.
- **`~a` is `-a - 1`, for every integer.** `~12` is −13 and `~(-13)` is 12. The reason: `a` and
  `~a` have a 1 in opposite columns, so `a + ~a` is all ones, which is −1.
- `-1 & 0xFF` keeps the low eight columns of all-ones: 255. `-256 & 0xFF` is 0, since −256 is
  `…1111 0000 0000`. `&` with a number that is not negative always gives a number that is not
  negative, because the zeros-forever of one side wins.
- `-1 | x` is −1 and `-1 ⊕ x` is `~x`: or-ing with all ones gives all ones, xor-ing flips
  everything.

None of these answers depends on a width. There is no "32-bit not" or "64-bit not" in the
language: `~12` is −13 on every target, as `12 + 1` is 13 on every target. (When you *want* the
32-bit complement, the one that turns `0x0000FFFF` into `0xFFFF0000`, you say so; §7.9.)

## 7.4 Shifts

`a << k` moves the numeral `k` places to the left, filling with zeros. `a >> k` moves it `k`
places to the right, and the bits that fall off the end are gone.

In base ten, moving a numeral one place left multiplies by ten: 305 becomes 3050. In base two it
multiplies by two. So:

> **`a << k` is `a · 2ᵏ`. `a >> k` is `a / 2ᵏ` rounded down.**

<!-- file docs/book/code/bits/shift.oro -->
```go
use io

export fn main() {
    io.print_int(1 << 10)
    io.print_int(3 << 4)
    io.print_int(-3 << 4)
    io.print_int(1000 >> 3)
    io.print_int(1000 / 8)
    io.print_int(-7 >> 1)
    io.print_int(-7 / 2)
    io.print_int(5 >> 100)
    io.print_int(-5 >> 100)
}
```

<!-- run go docs/book/code/bits/shift.oro -->
```
1024
48
-48
125
125
-4
-3
0
-1
```

- `1 << 10` is 2¹⁰. `1 << k` is the number whose only 1 is bit `k`, and you will write it often.
- **`-7 >> 1` is −4, and `-7 / 2` is −3.** Both divide by two; they round differently. `/`
  rounds toward zero (§6.2). `>>` rounds *down*, toward minus infinity, because dropping the
  last bit of `…1001` (−7) leaves `…100` (−4). For numbers that are not negative the two agree,
  as `1000 >> 3` and `1000 / 8` do.
- **A shift count has no upper limit.** `5 >> 100` is 0: move a numeral a hundred places right
  and nothing is left but its zeros-forever. `-5 >> 100` is −1, its ones-forever. In C a shift
  by 64 or more is undefined; in Java and on x86 the count is quietly reduced modulo 64, so
  `5 >> 100` is `5 >> 36`. Here it is the integer the definition gives, and the compiler emits
  what each machine needs to get it.

A shift count that might be negative has no meaning, so it is a precondition, like a divisor
that might be zero (§6.7):

<!-- file docs/book/code/bits/no-count.oro -->
```go
export fn scale(n ∈ 0..255, k ∈ -1..8) (Int) { n << k }
```

<!-- gen go docs/book/code/bits/no-count.oro -->
```
gen: bits-scale: `<<` requires -k <= 0, which does not follow
  known: assumed -n <= 0, assumed n + -255 <= 0, assumed -k + -1 <= 0, assumed k + -8 <= 0
```

"`-k <= 0`" is `k >= 0`, in the form the prover keeps it. And since `<<` is a multiplication,
it can leave the word like one:

<!-- file docs/book/code/bits/no-shift.oro -->
```go
export fn scale(n ∈ 0..255, k ∈ 0..64) (Int) { n << k }
```

<!-- gen go docs/book/code/bits/no-shift.oro -->
```
note: bits-scale: 0 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: bits-scale: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  << [0, 4703919738795935662080] in n << k
…
```

255 · 2⁶⁴ is the number printed, and it does not fit. This is the bug C calls "shifting into
the sign bit", caught the way chapter 6 caught `n * n`. Say what the count can be, and it
compiles to the machine's shift and nothing else:

<!-- file docs/book/code/bits/scale.oro -->
```go
export fn scale(n ∈ 0..255, k ∈ 0..8) (0..65280) { n << k }

export fn shrink(n ∈ -2^63..2^63-1, k ∈ 0..1000) (Int) { n >> k }
```

<!-- gen go docs/book/code/bits/scale.oro -->
```go
note: bits-scale: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-shrink: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func BitsScale(v0 int, v1 int) int {
    v23 := (v0 << v1)
    return v23
}

func BitsShrink(v0 int, v1 int) int {
    v23 := (v0 >> v1)
    return v23
}
```

- The result's set `0..65280` (255 · 2⁸) was *proven*: the compiler multiplied the interval of
  `n` by the interval of 2ᵏ.
- `shrink` takes any word and any count up to a thousand, with nothing to prove: a right shift
  only makes a number smaller in size. Go's `>>` happens to give the mathematical answer for
  large counts, so that is all that is emitted.

## 7.5 A mask is a proof

Chapter 6's method was: every value has an interval, and every operation is checked against the
word. The bit operations fit that method unusually well, because several of them *bound* their
result no matter what goes in.

<!-- file docs/book/code/bits/facts.oro -->
```go
set Word = -2^63..2^63-1

export fn low(n ∈ Word) (0..255) { n & 255 }

export fn low_mod(n ∈ Word) (0..255) { n mod 256 }

export fn top(n ∈ 0..2^32-1) (0..255) { n >> 24 }

export fn join(a ∈ 0..255, b ∈ 0..255) (0..255) { a | b }

export fn differ(a ∈ 0..255, b ∈ 0..255) (0..255) { a ⊕ b }
```

<!-- gen go docs/book/code/bits/facts.oro -->
```go
note: bits-differ: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-join: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-low: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-low-mod: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-top: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func BitsLow(v0 int) int {
    v10 := (v0 & 255)
    return v10
}

func BitsLowMod(v0 int) int {
    v10 := (v0 & 255)
    return v10
}
…
```

Every declared result is a claim (§6.12), and all five were proven. The rules the compiler
used, each one something you can check on your fingers:

| operation | what is known about the result |
|---|---|
| `a & m`, with `m` not negative | between 0 and `m`, whatever `a` is |
| `a \| b`, `a ⊕ b`, neither negative | not negative, and below the next power of two above both |
| `~a` | `-a - 1`, so the interval is flipped and moved by one |
| `a >> k` | `a` divided by the interval of 2ᵏ, rounded down |
| `a << k` | `a` times the interval of 2ᵏ |
| `a mod m` | between 0 and `m − 1` |

- `low` takes **any** word, negative ones included, and its result is in `0..255` because `&`
  with 255 cannot produce a 1 above bit 7. An operation that keeps chosen bits and clears the
  rest is called a **mask**, and a mask is the cheapest proof in the language: one instruction
  that makes a value small.
- **`n mod 256` and `n & 255` are the same function**, on every integer, negative ones
  included: both keep the low eight bits. The compiler knows it. Look at `BitsLowMod`: you wrote
  a remainder and the machine does an and. This holds for every power of two, `n mod 2ᵏ` is
  `n & (2ᵏ − 1)`, and it is the hinge of §7.8.
- `top` shifts a 32-bit number right by 24: what is left is below 2⁸.

"Below the next power of two" is sharper than it may look. An or can exceed both of its
operands:

<!-- file docs/book/code/bits/no-join.oro -->
```go
export fn join(a ∈ 0..200, b ∈ 0..200) (0..200) { a | b }
```

<!-- gen go docs/book/code/bits/no-join.oro -->
```
gen: join: the body does not establish 0 <= result && result <= 200; its result is [0, 255]
  A definition's declared result is a claim about every argument its parameters' sets admit (ADR 0072): narrow a parameter's set, state a `where`, or widen the result's set if the definition is right outside it
```

The refusal is right: `128 | 100` is 228. Two numbers up to 200 can light any of the low eight
bits between them, so 255 is the honest bound.

## 7.6 A set of small numbers is an integer

Here is the first place the bit operations earn their keep. Take a small universe, say three
things a user may do to a file: read it, write it, run it. Give each thing a bit. A *set* of
permissions is then one number, with a 1 in the column of every member:

```
    run write read
     0    1    1      =  3   the set {read, write}
     1    0    1      =  5   the set {read, run}
```

and the set operations of school are the bit operations of §7.2:

| sets | bits |
|---|---|
| union, A ∪ B | `a \| b` |
| intersection, A ∩ B | `a & b` |
| everything not in A | `~a` |
| A without B | `a & ~b` |
| in exactly one of the two | `a ⊕ b` |
| the empty set | `0` |
| is x a member? | `a & x != 0` |

<!-- file docs/book/code/bits/flags.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

set Rights = 0..7

read = 1
write = 2
run = 4

fn may?(r ∈ Rights, what ∈ Rights) (Bool) { r & what != 0 }

fn grant(r ∈ Rights, what ∈ Rights) (Rights) { r | what }

fn revoke(r ∈ Rights, what ∈ Rights) (Rights) { r & ~what }

fn flip(r ∈ Rights, what ∈ Rights) (Rights) { r ⊕ what }

export fn main() {
    r = grant(read, write)
    io.print_int(r)
    io.print_int(bit(may?(r, write)))
    io.print_int(bit(may?(r, run)))
    io.print_int(grant(r, run))
    io.print_int(revoke(r, write))
    io.print_int(revoke(r, run))
    io.print_int(flip(r, read | run))
}
```

<!-- run go docs/book/code/bits/flags.oro -->
```
3
1
0
7
1
3
6
```

- The three constants are 1, 2 and 4: `1 << 0`, `1 << 1`, `1 << 2`. One bit each.
- `grant` is idempotent, `a | a == a`: granting a right twice is granting it once. `revoke` of
  a right not held changes nothing (the sixth line). Those are §7.2's laws doing work.
- **`revoke` is `r & ~what`**, "and with everything except". `~what` is a negative number, ones
  forever to the left with holes where `what` has ones. That is fine: `r` has zeros forever, so
  the and is back in `0..7`, and the compiler proved the declared `Rights` from exactly that
  reasoning (§7.5's first row, read the other way round).
- `flip(r, read | run)` toggles two rights at once: 011 ⊕ 101 is 110, which is 6.

This is how Unix stores file permissions (the 7 in `chmod 755` is `read | write | run`), how
a network packet stores its options, and how a chess program stores which of 64 squares hold a
pawn: one word, 64 switches, and a set operation on all of them in one instruction.

A set written this way is also a claim you can state. `Rights` is `0..7`: three bits and no
more. A function that returns a `Rights` has been proven never to set a fourth.

## 7.7 Packing several numbers into one

The other everyday use: a number made of fields. A colour on a screen is three intensities,
red, green and blue, each from 0 to 255: each a byte, eight bits. Put the three side by side
and you have one 24-bit number, which is the `#3399FF` of a web page:

```
    bits 23..16    bits 15..8    bits 7..0
       red           green          blue
       0x33          0x99           0xFF
```

To build it, shift each field to its place and or them together. To take one out, shift it
down to the bottom and mask off what is above it.

<!-- file docs/book/code/bits/colour.oro -->
```go
use io

set Byte = 0..255
set Colour = 0..2^24-1

fn rgb(r ∈ Byte, g ∈ Byte, b ∈ Byte) (Colour) { r << 16 | g << 8 | b }

fn red(c ∈ Colour) (Byte) { c >> 16 }

fn green(c ∈ Colour) (Byte) { c >> 8 & 255 }

fn blue(c ∈ Colour) (Byte) { c & 255 }

export fn main() {
    c = rgb(0x33, 0x99, 0xFF)
    io.print_int(c)
    io.print_int(red(c))
    io.print_int(green(c))
    io.print_int(blue(c))
}

export fn pack(r ∈ Byte, g ∈ Byte, b ∈ Byte) (Colour) { rgb(r, g, b) }

export fn middle(c ∈ Colour) (Byte) { green(c) }
```

<!-- run go docs/book/code/bits/colour.oro -->
```
3381759
51
153
255
```

<!-- gen go docs/book/code/bits/colour.oro -->
```go
…
func BitsMiddle(v0 int) int {
    v10 := (v0 >> 8)
    v12 := (v10 & 255)
    return v12
}

func BitsPack(v0 int, v1 int, v2 int) int {
    v42 := (v0 << 16)
    v44 := (v1 << 8)
    v45 := (v42 | v44)
    v46 := (v45 | v2)
    return v46
}
```

- 3381759 is `0x3399FF`. The three fields come back out as 51, 153 and 255, which are `0x33`,
  `0x99` and `0xFF`.
- `r << 16 | g << 8 | b` needs no parentheses: shifts bind tighter than `|` (§7.2).
- **Every set in the file was proven.** `rgb` returns a `Colour` because a byte shifted left by
  16 is below 2²⁴ and an or of numbers below 2²⁴ is below 2²⁴. `red` returns a `Byte` because a
  `Colour` shifted right by 16 is below 2⁸. `green` because of its mask.
- `red` has no mask and `green` has one. Delete `green`'s and the compiler refuses it: `c >> 8`
  of a 24-bit number is 16 bits, not 8. In C the unmasked version compiles and returns red and
  green glued together. You do not have to remember which fields need the mask, since the set
  on the result asks the question for you.
- Here `|` could have been `+`: the three fields share no column, so nothing carries. That is
  §7.2's identity `a | b == (a ⊕ b) + (a & b)` with `a & b` equal to 0. Writing `|` says what
  you mean.

## 7.8 Wrapping, on purpose

Now the second sentence of the chapter. Some arithmetic is *supposed* to throw the high bits
away. A **hash function** takes some bytes and stirs them into one number, such that different
inputs nearly always give different numbers; hash tables, checksums and version-control systems
all stand on one. A widely used one, FNV-1a (Fowler, Noll and Vo, 1991), is a single line per
byte:

> start with h = 2166136261; for each byte b: h ← (h ⊕ b) · 16777619, **keeping the low 32 bits**.

Write the step without the last four words and the compiler says what chapter 6 taught it to
say:

<!-- file docs/book/code/bits/no-wrap.oro -->
```go
set U32 = 0..2^32-1

export fn step(h ∈ U32, b ∈ 0..255) (U32) { (h ⊕ b) * 16777619 }
```

<!-- gen go docs/book/code/bits/no-wrap.oro -->
```
gen: step: the body does not establish 0 <= result && result <= 4294967295; its result is [0, 72059324892970605]
  A definition's declared result is a claim about every argument its parameters' sets admit (ADR 0072): narrow a parameter's set, state a `where`, or widen the result's set if the definition is right outside it
```

A 32-bit number times 16777619 is a 56-bit number; it is not in `U32`. In C, with `uint32_t`,
this line is the whole hash: the multiplication wraps and nobody writes anything. The wrapping
is real, the algorithm depends on it, and the source does not mention it.

Here you mention it. "Keeping the low 32 bits" is `mod 2^32` (§7.5):

<!-- file docs/book/code/bits/wrap.oro -->
```go
set U32 = 0..2^32-1

export fn step(h ∈ U32, b ∈ 0..255) (U32) { ((h ⊕ b) * 16777619) mod 2^32 }

export fn add32(a ∈ U32, b ∈ U32) (U32) { (a + b) mod 2^32 }

export fn sub32(a ∈ U32, b ∈ U32) (U32) { (a - b) mod 2^32 }
```

<!-- gen go docs/book/code/bits/wrap.oro -->
```go
note: bits-add32: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-step: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: bits-sub32: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func BitsAdd32(v0 int, v1 int) int {
    v23 := (v0 + v1)
    v25 := (v23 & 4294967295)
    return v25
}

func BitsStep(v0 int, v1 int) int {
    v23 := (v0 ^ v1)
    v25 := (v23 * 16777619)
    v27 := (v25 & 4294967295)
    return v27
}

func BitsSub32(v0 int, v1 int) int {
    v23 := (v0 - v1)
    v25 := (v23 & 4294967295)
    return v25
}
```

One xor, one multiply, one and. `mod 2^32` became `& 4294967295`, and there is no division
anywhere. `sub32` is worth a look: `a - b` may be negative, and `mod` of a negative number is
never negative (§6.2), so `sub32(0, 1)` is 4294967295, which is what a 32-bit register would
hold. The mathematics and the machine agree because `mod` was defined to make them agree.

Hash the three letters `abc`, whose bytes are 97, 98 and 99. (Looping over the bytes of a real
string needs [chapter 11](README.md); three steps can be written out.)

<!-- file docs/book/code/bits/hash.oro -->
```go
use io

set U32 = 0..2^32-1

fn step(h ∈ U32, b ∈ 0..255) (U32) { ((h ⊕ b) * 16777619) mod 2^32 }

export fn main() {
    io.print_int(step(step(step(0x811C9DC5, 97), 98), 99))
}
```

<!-- run go docs/book/code/bits/hash.oro -->
```
440920331
```

<!-- run js docs/book/code/bits/hash.oro -->
```
440920331
```

<!-- run windows docs/book/code/bits/hash.oro -->
```
440920331
```

440920331 is `0x1A47E90B`, the value every FNV-1a implementation in the world gives for `abc`.

### Why the machine may wrap early

On Go the product above fitted in the 64-bit word, so the compiler multiplied honestly and
masked. JavaScript's word stops at 2⁵³, and a 56-bit product does not fit. Chapter 6 would
refuse. Here is what happens instead:

<!-- gen js docs/book/code/bits/wrap.oro -->
```javascript
…
export function bitsStep(v0, v1) {
    return ((((Math.imul(((v0 ^ v1)), 16777619)) & 4294967295) >>> 0));
}
…
```

`Math.imul` is JavaScript's multiplication *that wraps at 32 bits*. The compiler never computes
the 56-bit product. It computes a wrapped one, and the answer is the same. Why is that
allowed?

Try it in base ten, where "keep the low two digits" is `mod 100`:

```
    1234 × 5678 = 7006652          the last two digits are 52
      34 ×   78 =    2652          the last two digits are 52
```

The low digits of a product depend only on the low digits of the factors. Long multiplication
shows why: every digit of the answer is made from digits at or to the right of it, and carries
only ever move left. The same is true of a sum and a difference, in any base. In symbols:

> (a + b) mod m = ((a mod m) + (b mod m)) mod m, and the same for − and ×.

This is **clock arithmetic**: on a 12-hour clock, 9 o'clock plus 5 hours is 2 o'clock, and it
does not matter how many times the hand went round before. Gauss built the theory in 1801 and
called it arithmetic *modulo* m; `mod` is his word. A mathematician says that taking the
remainder *is a homomorphism*, a map that commutes with the operations: reduce and then compute, or
compute and then reduce, and you arrive at the same place.

The bit operations `&`, `|`, `⊕`, `~` and a left shift by a constant have the same property
for `mod 2ⁿ`: a column of the result depends on nothing to its left. So the rule the compiler
applies is:

> **Under a `mod 2ⁿ`, a calculation made of `+ - * & | ⊕ ~` and `<<` may be done in the machine's
> wrapping arithmetic from the start.** Its intermediate values are never checked against the
> word, because they are never held.

The clearest case is 64 bits, where the product does not fit in *any* machine word:

<!-- file docs/book/code/bits/wrap64.oro -->
```go
set U64 = 0..2^64-1

export fn step(h ∈ U64, b ∈ 0..255) (U64) { ((h ⊕ b) * 1099511628211) mod 2^64 }
```

<!-- gen go docs/book/code/bits/wrap64.oro -->
```go
note: bits-step: 2 operation(s) or conversion(s) in the unsigned word
…
func BitsStep(v0 uint64, v1 int) uint64 {
    v2 := (int(v0))
    v3 := (v2 ^ v1)
    v5 := (v3 * 1099511628211)
    v6 := (uint64(v5))
    return v6
}
```

The true product reaches 2¹⁰⁴. The emitted code is an xor and a multiply, and no mask at all:
`mod 2^64` on a 64-bit machine is what the multiply instruction already does. This is the same
code a C programmer gets from `uint64_t`. The difference is in the source. There, the wrapping
is a property of a type and applies to every operation whether you thought about it or not.
Here it is three words at the place you meant it.

On Windows the same function is two instructions of LLVM's:

<!-- gen windows docs/book/code/bits/wrap64.oro -->
```llvm
…
  %v3 = xor i64 %v2, %v1
  %v5 = mul i64 %v3, 1099511628211
…
```

And the 64-bit hash of `abc`, printed as its two 32-bit halves:

<!-- file docs/book/code/bits/hash64.oro -->
```go
use io

set U64 = 0..2^64-1

fn step(h ∈ U64, b ∈ 0..255) (U64) { ((h ⊕ b) * 1099511628211) mod 2^64 }

export fn main() {
    h = step(step(step(0xCBF29CE484222325, 97), 98), 99)
    io.print_int(h >> 32)
    io.print_int(h & 0xFFFFFFFF)
}
```

<!-- run go docs/book/code/bits/hash64.oro -->
```
3877610009
88168267
```

<!-- run js docs/book/code/bits/hash64.oro -->
```
3877610009
88168267
```

<!-- run windows docs/book/code/bits/hash64.oro -->
```
3877610009
88168267
```

Those are `0xE71FA219` and `0x0541574B`; the hash is `0xE71FA2190541574B`, the published value.
JavaScript has no 64-bit integer at all, and prints the same two lines: there the compiler
holds the value as two 32-bit halves and expands each operation into arithmetic on the halves,
the way you multiply two-digit numbers knowing only the one-digit table.

### What may not wrap early

Division does not have the property. In base ten: 100 / 2 is 50, but the low two digits of 100
are 00, and 00 / 2 is 0. A quotient's low digits depend on the high digits of what was divided.
Neither does a comparison, nor a right shift. So a calculation that divides before it reduces
must really hold the big number, and chapter 6's rule is back:

<!-- file docs/book/code/bits/product.oro -->
```go
set U32 = 0..2^32-1

export fn low(a ∈ 0..2^40, b ∈ 0..2^40) (U32) { (a * b) mod 2^32 }
```

<!-- file docs/book/code/bits/no-divide.oro -->
```go
set U32 = 0..2^32-1

export fn third(a ∈ 0..2^40, b ∈ 0..2^40) (U32) { (a * b / 3) mod 2^32 }
```

<!-- gen go docs/book/code/bits/product.oro -->
```go
note: bits-low: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func BitsLow(v0 int, v1 int) int {
    v23 := (v0 * v1)
    v25 := (v23 & 4294967295)
    return v25
}
```

<!-- gen go docs/book/code/bits/no-divide.oro -->
```
note: bits-third: 1 of 3 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: bits-third: 2 of 3 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  * [0, 1208925819614629174706176] in a * b
  / [0, 402975273204876391568725] in a * b / 3
…
```

The first product reaches 2⁸⁰ and compiles to one multiply, because nothing but the `mod` reads
it. The second is the same product, read by a division, and is refused. A C compiler accepts
both, and the second quietly computes a third of the wrong number.

## 7.9 The complement with a width, and a rotation

Two idioms that every C programmer writes without thinking, and that need one more word here.

**The 32-bit complement.** `~a` is `-a - 1` (§7.3): negative whenever `a` is not. A checksum
that says "invert all 32 bits" wants `0x0000FFFF` to become `0xFFFF0000`, a positive number.

<!-- file docs/book/code/bits/no-not.oro -->
```go
export fn flip(a ∈ 0..2^32-1) (0..2^32-1) { ~a }
```

<!-- gen go docs/book/code/bits/no-not.oro -->
```
gen: flip: the body does not establish 0 <= result && result <= 4294967295; its result is [-4294967296, -1]
  A definition's declared result is a claim about every argument its parameters' sets admit (ADR 0072): narrow a parameter's set, state a `where`, or widen the result's set if the definition is right outside it
```

There are two ways to say "of 32 bits", and they are the same function:

<!-- file docs/book/code/bits/not32.oro -->
```go
use io

set U32 = 0..2^32-1

fn flip(a ∈ U32) (U32) { a ⊕ 0xFFFFFFFF }

fn flip_wrapped(a ∈ U32) (U32) { ~a mod 2^32 }

export fn main() {
    io.print_int(~0x0000FFFF)
    io.print_int(flip(0x0000FFFF))
    io.print_int(flip_wrapped(0x0000FFFF))
}
```

<!-- run go docs/book/code/bits/not32.oro -->
```
-65536
4294901760
4294901760
```

Xor with thirty-two ones flips exactly thirty-two bits (§7.2: xor with 1 flips, with 0 keeps).
Or flip all of them and keep the low thirty-two. 4294901760 is `0xFFFF0000`.

**A rotation.** A shift drops the bits that fall off the end. A *rotation* brings them round to
the other end; ciphers and hash functions use it to stir bits without losing any. "The other
end" only means something once you fix a width, and the width is a `mod`:

<!-- file docs/book/code/bits/rotate.oro -->
```go
use io

set U32 = 0..2^32-1

fn rotl8(x ∈ U32) (U32) { (x << 8 | x >> 24) mod 2^32 }

export fn main() {
    io.print_int(rotl8(0x12345678))
    io.print_int(0x34567812)
}

export fn turn(x ∈ U32) (U32) { rotl8(x) }
```

<!-- run go docs/book/code/bits/rotate.oro -->
```
878082066
878082066
```

<!-- gen go docs/book/code/bits/rotate.oro -->
```go
…
func BitsTurn(v0 int) int {
    v10 := (v0 << 8)
    v12 := (v0 >> 24)
    v13 := (v10 | v12)
    v15 := (v13 & 4294967295)
    return v15
}
```

`0x12345678` rotated left by two hex digits is `0x34567812`: the `12` left the top and came
back at the bottom. `x << 8` pushes everything up (and the top byte past bit 31), `x >> 24`
brings the top byte down to the bottom, the or joins them, and `mod 2^32` discards what went
past bit 31.

## 7.10 Three small tricks, and why they work

Bit tricks have a reputation as magic. Each is a short consequence of how carrying and
borrowing work, and each is worth understanding once.

<!-- file docs/book/code/bits/tricks.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

fn even?(n ∈ 0..2^62) (Bool) { n & 1 == 0 }

fn power_of_two?(n ∈ 1..2^62) (Bool) { n & (n - 1) == 0 }

fn lowest(n ∈ 1..2^62) (Int) { n & -n }

fn without_lowest(n ∈ 1..2^62) (Int) { n & (n - 1) }

export fn main() {
    io.print_int(bit(even?(10)))
    io.print_int(bit(even?(7)))
    io.print_int(bit(power_of_two?(64)))
    io.print_int(bit(power_of_two?(96)))
    io.print_int(lowest(96))
    io.print_int(without_lowest(96))
}
```

<!-- run go docs/book/code/bits/tricks.oro -->
```
1
0
1
0
32
64
```

- **`n & 1`** is bit 0, the ones place. A number is even when its ones place is 0, as a decimal
  number is a multiple of ten when it ends in 0.
- **`n - 1`** borrows. Subtracting 1 turns the lowest 1 of `n` into 0 and every 0 below it into
  1, and leaves everything above alone: 96 is `110 0000`, 95 is `101 1111`. So `n & (n - 1)` is
  `n` **with its lowest 1 removed**: `100 0000`, 64.
- A power of two has exactly one 1. Remove it and nothing is left: **`n & (n - 1) == 0`**.
- **`-n`** is `~n + 1` (§7.3, rearranged). Flipping makes the zeros below the lowest 1 into
  ones; adding 1 carries through them and stops at that lowest position, restoring it. Above
  it everything is flipped, below it everything is 0. So `n & -n` is **the lowest 1 alone**:
  32.

## 7.11 JavaScript's bit operators see 32 bits

Chapter 6 ended on portability: a program is legal on a target or it is not, and the compiler
says which. The bit operations add one line to that story.

JavaScript has no integers. Its `&`, `|`, `^`, `<<` and `>>` convert each operand to a 32-bit
integer, operate, and convert back. `2**40 & 2**40` is 0 there. So on that target the compiler
uses JavaScript's own operator only where the result is proven to fit in 32 bits, and otherwise
finds another way to get the integer the program means:

<!-- file docs/book/code/bits/narrow.oro -->
```go
export fn low(n ∈ 0..2^53-1) (0..255) { n & 255 }

export fn differ(a ∈ 0..2^32-1, b ∈ 0..2^32-1) (0..2^32-1) { a ⊕ b }

export fn top(n ∈ 0..2^32-1) (0..255) { n >> 24 }

export fn scale(n ∈ 0..255, k ∈ 0..8) (0..65280) { n << k }

export fn invert(n ∈ -2^31..2^31-1) (-2^31..2^31-1) { ~n }
```

<!-- gen js docs/book/code/bits/narrow.oro -->
```javascript
…
export function bitsDiffer(v0, v1) {
    return (((v0 ^ v1) >>> 0));
}

export function bitsInvert(v0) {
    return ((-(v0) - 1));
}

export function bitsLow(v0) {
    return ((v0 & 255));
}

export function bitsScale(v0, v1) {
    return ((v0 * 2 ** v1));
}

export function bitsTop(v0) {
    return ((v0 >>> 24));
}
```

Five functions, five different decisions, none of them yours:

- `low`: a 53-bit number and-ed with 255. JavaScript's `&` looks only at the low 32 bits, and
  the low 8 are among them, so its own operator is right.
- `differ`: JavaScript's `^` returns a *signed* 32-bit number, so `0x80000000 ^ 0` would come
  back negative. The `>>> 0` reads the same 32 bits as unsigned. A hand-written JavaScript
  program that forgets it has a bug that shows only when bit 31 is set.
- `top`: `>>>` and not `>>`, for the same reason.
- `scale`: no `<<` at all. A left shift is a multiplication, and a multiplication is exact up
  to 2⁵³.
- `invert`: `-n - 1`, which is what `~` means.

Past the word there is a way too, the rung above it. A parameter declared wider than
JavaScript's word arrives as a `BigInt`, and `&` on a `BigInt` is the 2-adic and itself. The
result is in 0..255, so it comes back to a number:

<!-- file docs/book/code/bits/no-js.oro -->
```go
export fn low(n ∈ -2^63..2^63-1) (0..255) { n & 255 }
```

<!-- portable docs/book/code/bits/no-js.oro -->
```
…
portable to go, js, java, windows
W(go, js, java, windows) = [-9007199254740991, 9007199254740991]   (the meet of their words: derived, not assumed — ADR 0026)
```

The cost differs by target and the answer does not: a `BigInt` `&` per call on JavaScript, an
`and` instruction elsewhere ([bigbits-2026-11-19](../../gauntlet/results/bigbits-2026-11-19.md)).

## 7.12 What other languages do

| | xor is written | `~12` | `-7 >> 1` | `1 << 64` | `5 >> 100` | wrapping |
|---|---|---|---|---|---|---|
| C, C++ | `^` | −13 | −4 or 2147483644, the compiler's choice until C++20 | undefined | undefined | unsigned types always wrap; signed overflow is undefined |
| Java | `^` | −13 | −4 (`>>`), a huge positive (`>>>`) | 1: the count is taken mod 64 | 0 only by luck: it is `5 >> 36` | always, silently |
| Go | `^` | −13, written `^12` | −4 | 0 | 0 | always, silently |
| JavaScript | `^` | −13 | −4 | 1: the count is taken mod 32 | 0 only by luck: it is `5 >> 4` | everything is cut to 32 bits |
| Python | `^` | −13 | −4 | 18446744073709551616 | 0 | never; you write `& 0xFFFFFFFF` |
| this language | `⊕` | −13 | −4 | refused: 2⁶⁴ does not fit the word | 0 | where you write `mod 2ⁿ` |

Python is the nearest relative: its integers are unbounded, its bit operations work on the
infinite numeral, and a Python programmer who wants 32-bit wrapping writes `& 0xFFFFFFFF`
exactly as you write `mod 2^32`. The difference is the cost. Python really does compute the
104-bit product and then cut it down. Here the same source compiles to the single machine
multiply, because the compiler can prove the high bits are never looked at.

So the position is the one chapter 6 took, carried one step further. The meaning is the
mathematical one, with no width in it. The width is something you state, once, where it
matters. And the machine's fast, narrow, wrapping instruction is what runs, wherever a proof
says it computes that meaning.

---

## What to remember

- **Every integer has a binary numeral**, zeros forever to the left if it is not negative, ones
  forever if it is. Hexadecimal is that numeral read four bits at a time. n ones is 2ⁿ − 1.
- **`&`, `|`, `⊕` work column by column**: both, either, differ. `⊕` is typed `\oplus`; `^` is
  a power. They are chapter 4's `&&`, `||`, `!=`, on every bit at once.
- **`~a` is `-a - 1`**, with no width. −1 is all ones.
- **`a << k` is `a · 2ᵏ`; `a >> k` is `a / 2ᵏ` rounded down** (where `/` rounds toward zero).
  The count must be proven not negative; it has no upper limit. `<<` can leave the word, like
  any multiplication.
- **`& << >>` group like `*`; `| ⊕` like `+`**; all tighter than comparisons.
- **A mask is a proof**: `x & m` is in `0..m`. `x mod 2ᵏ` is `x & (2ᵏ − 1)`, and compiles to it.
- **A set of small numbers is an integer**: union `|`, intersection `&`, without `& ~`, toggle
  `⊕`, member `& x != 0`.
- **Fields are packed with `<<` and `|`, and unpacked with `>>` and `&`**; the result's set
  tells you when a mask is missing.
- **Wrapping is `mod 2ⁿ`, written where you mean it.** Under it, `+ - * & | ⊕ ~ <<` run in the
  machine's wrapping arithmetic and their intermediates are never held, because remainders
  commute with those operations. Division, comparison and `>>` do not commute, and stop it.
- **A 32-bit complement is `a ⊕ 0xFFFFFFFF`**; a 32-bit rotation is `(x << k | x >> (32 - k))
  mod 2^32`.
- **`n & (n - 1)` removes the lowest 1; `n & -n` isolates it.**
- **On JavaScript** the compiler uses the host's 32-bit operators only where the result is
  proven to fit, and something exact elsewhere.

Next: [chapter 8](08-numbers.md), Numbers with a point. What 0.1 + 0.2 is, and the one operation that is
allowed to lose anything.
