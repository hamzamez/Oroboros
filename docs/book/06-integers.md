# 6. Integers

Part I was about how a program is reduced: [functions](01-fn.md), [names](02-def.md),
[modules](03-modules.md), [choices](04-choice.md), [effects](05-effects.md). Numbers stood in the
examples the whole time, and nothing was said about them. Part II is about what a program
computes with, and it starts with the oldest thing there is to compute with.

The question of this chapter sounds too simple to ask: **what is `n + 1`?**

In mathematics it is the next integer, always. In almost every programming language it is the
next integer *unless `n` is the largest number the machine's register holds*, in which case it is
something else: a large negative number in C, Java and Go, a number that is quietly wrong in
JavaScript. Programs have been lost to that "unless" for sixty years. This language's answer is
one sentence, and the chapter is its consequences:

> **An integer is an integer. An operation either computes the integer it means, or the program
> does not compile.**

Along the way: Euclid's division, the machine word and why it wraps, intervals (the oldest
program analysis there is), preconditions (Hoare, 1969), and the most famous one-line bug in
computing, which this language cannot express.

The examples are in [code/integers/](code/integers/), compiled for real targets, mostly Go.
`go run docs/book/check.go docs/book/06-integers.md` re-runs every block. Checked 2026-11-15.

---

## 6.1 Writing one

<!-- file docs/book/code/integers/literals.oro -->
```go
use io

export fn main() {
    io.print_int(42)
    io.print_int(-7)
    io.print_int(0xFF)
    io.print_int(2^10)
    io.print_int(10^6)
    io.print_int(2 + 3 * 4)
    io.print_int((2 + 3) * 4)
    io.print_int(10 - 3 - 2)
    io.print_int(100 / 10 / 5)
    io.print_int(2 * 3^2)
}
```

<!-- run go docs/book/code/integers/literals.oro -->
```
42
-7
255
1024
1000000
14
20
5
2
18
```

- A literal is decimal, or hexadecimal after `0x`. A minus sign in front makes it negative, and
  so does one in front of a name or a parenthesis: `-n`, `-(a + b)`.
- **`^` is a power**, as in mathematics, and it binds tighter than `*`: `2 * 3^2` is 18. It is
  for writing constants, `2^10`, `10^6`, `2^63-1`, and both sides must be known when the program
  is compiled. (In C and its descendants `^` is "exclusive or". That operation is `⊕` here, and
  [chapter 7](07-bits.md) is about it.)
- `*`, `/` and `%` bind tighter than `+` and `-`, and operators of the same strength group from
  the left: `10 - 3 - 2` is `(10 - 3) - 2`, and `100 / 10 / 5` is `(100 / 10) / 5`. That is the
  convention of school arithmetic and of nearly every language.
- All of these were computed by the compiler (§1.3). The program that runs prints ten constants.

Two things you might try, and cannot:

<!-- oro tutorial docs/book/code/integers/no-underscore.oro.txt -->
```
oro: docs/book/code/integers/no-underscore.oro.txt: line 1: "_000_000" after a statement; a statement ends at a line break or `;`
```

<!-- file docs/book/code/integers/no-power.oro -->
```go
export fn power(n ∈ 0..10) (Int) { 2^n }
```

<!-- gen go docs/book/code/integers/no-power.oro -->
```
gen: docs/book/code/integers/no-power.oro: in power: `^` is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

There is no digit separator: a million is `1000000` or `10^6`. And `^` with an exponent the
compiler does not know is not an operation any target has; a power of a variable is a loop
([chapter 12](README.md)).

## 6.2 Division, and what is left over

Addition, subtraction and multiplication of integers give integers. Division does not: 7 divided
by 2 is three and a half. So "integer division" needs a decision about the half, and a second
operation for what was thrown away.

<!-- file docs/book/code/integers/divide.oro -->
```go
use io

export fn main() {
    io.print_int(7 / 2)
    io.print_int(-7 / 2)
    io.print_int(7 / -2)
    io.print_int(-7 / -2)
    io.print_line("remainders")
    io.print_int(7 % 2)
    io.print_int(-7 % 2)
    io.print_int(7 % -2)
    io.print_int(-7 % -2)
    io.print_line("mod")
    io.print_int(7 mod 2)
    io.print_int(-7 mod 2)
}
```

<!-- run go docs/book/code/integers/divide.oro -->
```
3
-3
-3
3
remainders
1
-1
1
-1
mod
1
1
```

<!-- run js docs/book/code/integers/divide.oro -->
```
3
-3
-3
3
remainders
1
-1
1
-1
mod
1
1
```

- **`/` rounds toward zero**: the fraction is dropped, so `-7 / 2` is `-3`, not `-4`.
- **`%` is the remainder of that division**, and takes the sign of the left side: `-7 % 2` is
  `-1`.
- **`mod` is the remainder that is never negative**: `-7 mod 2` is `1`. Its right side must be
  positive.

The first two are tied together by the oldest theorem in arithmetic. Euclid, around 300 BC: for
any `a` and any `b` that is not zero, there is one quotient `q` and one remainder `r` with

```
a = q * b + r        and r smaller in size than b
```

Whatever rule `/` follows, `%` must be the `r` that makes this true. Rounding toward zero forces
the remainder to have the sign of `a`. Rounding *down* (the floor) would force it to have the
sign of `b`, and that is what `mod` computes.

Why two? Because both are wanted. A clock wants `mod`: three hours before 1 o'clock is
`(1 - 3) mod 12`, which is 10, where `(1 - 3) % 12` is `-2`, an hour that does not exist.
But every processor's divide instruction rounds toward zero, and so do C, Java, Go and
JavaScript. A language that emits the code you would have written by hand takes the machine's
division as `/` and gives the mathematician's remainder its own name.

<!-- file docs/book/code/integers/euclid.oro -->
```go
export fn back_again(a ∈ -1000..1000, b ∈ 1..1000) (Int) { (a / b) * b + a % b }

export fn is_even(n ∈ Int) (Bool) { n % 2 == 0 }

export fn last_digit(n ∈ Nat) (0..9) { n % 10 }

export fn clock(hour ∈ -1000..1000) (0..23) { hour mod 24 }
```

<!-- gen go docs/book/code/integers/euclid.oro -->
```go
…
func IntegersBackAgain(v0 int, v1 int) int {
    v23 := (v0 / v1)
    v24 := (v23 * v1)
    v25 := (v0 % v1)
    v26 := (v24 + v25)
    return v26
}

func IntegersClock(v0 int) int {
    v10 := (func(a, m int) int { r := a % m; if r < 0 { r += m }; return r }(v0, 24))
    return v10
}
…
```

`back_again` is Euclid's equation, and always returns `a`. `clock` shows what `mod` costs on a
host that lacks it: the host's remainder, and a correction when it came out negative. The
notation `a ∈ -1000..1000` in those headers has been used since chapter 0 without explanation.
It is next.

### A comparison is not chained

<!-- file docs/book/code/integers/no-chain.oro -->
```go
export fn between(a ∈ 0..9, b ∈ 0..9, c ∈ 0..9) (Bool) { a < b < c }
```

<!-- gen go docs/book/code/integers/no-chain.oro -->
```
gen: between: in argument 1 of `<`: `<` is Bool, but Int is required here
```

`a < b` is a `Bool` (§4.1), and a `Bool` is not less than anything. C accepts this line and
compares `c` with 0 or 1. Mathematics means `a < b` and `b < c`, and that is how to write it:

<!-- file docs/book/code/integers/chain.oro -->
```go
export fn between(a ∈ 0..9, b ∈ 0..9, c ∈ 0..9) (Bool) { a < b && b < c }
```

## 6.3 A range is a set

<!-- file docs/book/code/integers/sets.oro -->
```go
set Digit = 0..9

set Byte = 0..2^8-1

export fn percent(n ∈ 0..100) (Int) { n }

export fn any(n ∈ Int) (Int) { n }

export fn one_point(n ∈ 7..7) (Int) { n + n }

export fn complement(d ∈ Digit) (Digit) { 9 - d }

export fn high_bit(b ∈ Byte) (0..1) { b / 128 }
```

<!-- gen go docs/book/code/integers/sets.oro -->
```go
…
func IntegersAny(v0 int) int {
    return v0
}

func IntegersComplement(v0 int) int {
    v10 := (9 - v0)
    return v10
}

func IntegersHighBit(v0 int) int {
    v10 := (v0 >> 7)
    return v10
}
…
```

`n ∈ 0..100` is read *n is a member of the set of integers from 0 to 100*, both ends included.
The symbol `∈` is the one mathematics uses, and the thing on its right is a **set**:

| written | the set |
|---|---|
| `0..100` | the integers from 0 to 100, ends included |
| `7..7` | the one integer 7 |
| `-2^63..2^63-1` | ends may be any constant expression |
| `Int` | every integer, ℤ |
| `Nat` | the integers from 0 up, ℕ |
| `5..` | from 5 up, with no upper end |
| `..5` | up to 5, with no lower end |
| `Digit`, after `set Digit = 0..9` | a name for a set |

A header says which set each parameter comes from and which set the result is in. Those are not
annotations for a reader. They are **claims**, and the compiler proves each one or refuses the
program. `complement` claims that `9 - d` is a digit when `d` is, and that was checked: 9 − 0 is
9 and 9 − 9 is 0. `high_bit` claims that a byte divided by 128 is 0 or 1. (And the compiler,
knowing `b` is not negative, wrote the division as a shift, which is what you would have done.)

What a set is *not* is a size in memory. In C, `short`, `int` and `long` say how many bytes a
value occupies, and the range follows from that. Here the source says what the values are, and
the compiler chooses how to hold them. All five functions above take a Go `int`.

**A range may not be empty**, since no call could ever supply a member:

<!-- file docs/book/code/integers/no-empty.oro.txt -->
```go
export fn nobody(n ∈ 9..0) (Int) { n }
```

<!-- gen go docs/book/code/integers/no-empty.oro.txt -->
```
gen: docs/book/code/integers/no-empty.oro.txt: nobody: the set 9..0 is empty: no integer is at least 9 and at most 0, so no value inhabits it. Write the smaller end first
```

**An end may be a constant the program named**, and a constant may carry a claim of its own,
written like a parameter's:

<!-- file docs/book/code/integers/limit.oro -->
```go
cap = 100

half ∈ 0..cap = cap / 2

export fn below_cap(n ∈ 0..cap) (0..cap) { cap - n }

export fn near_half(n ∈ 0..cap) (Bool) { half - 5 <= n && n <= half + 5 }
```

<!-- gen go docs/book/code/integers/limit.oro -->
```go
func IntegersBelowCap(v0 int) int {
    v10 := (100 - v0)
    return v10
}

func IntegersNearHalf(v0 int) bool {
    v11 := (50 - 5)
    v12 := (v11 <= v0)
    return (v12 && (v0 <= (50 + 5)))
}
```

`cap` is written once and the three ranges follow it. `half ∈ 0..cap = cap / 2` says what
`half` is and which set it must be in, and the Go shows it was computed: 50. Change the
definition to `cap * 2` and the program is refused at the constant, under its name, whether or
not anything reads it:

<!-- gen go docs/book/code/integers/no-limit.oro -->
```
gen: half: the claim half ∈ 0..100 is false: its value is 200
```

## 6.4 The machine's integer

Here is the function everything in this chapter turns on:

<!-- file docs/book/code/integers/no-square.oro -->
```go
export fn square(n ∈ Int) (Int) { n * n }
```

<!-- gen go docs/book/code/integers/no-square.oro -->
```
note: integers-square: 0 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: integers-square: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  * [-inf, +inf] in n * n
  Outside its word a target's arithmetic does not compute the integer the
  program means — Go, the JVM and x86 wrap, JavaScript loses precision — so
  this is refused rather than noted (ADR 0019, ADR 0026).
…
```

A function that squares an integer, and it does not compile. To see why, look at what the
machine has.

A processor computes with **words**: registers of a fixed number of bits, 64 on the machines
this language targets. A word has 2⁶⁴ possible states, so it can stand for 2⁶⁴ different
integers and no more. Read as a signed number, those are −2⁶³ to 2⁶³ − 1, which is the first
interval in the message: from about minus nine billion billion to plus nine billion billion.

Inside that interval the machine's `+`, `-` and `*` give the true integer. At its edge they
cannot, since the true answer has no state to be stored in. What the hardware does instead is
keep the low 64 bits of the answer and drop the rest, which makes the numbers go round in a
circle, like a car's odometer rolling over from 999999 to 000000:

| in mathematics | in a 64-bit word |
|---|---|
| (2⁶³ − 1) + 1 = 2⁶³ | −2⁶³ |
| 3 000 000 000 × 3 000 000 000 = 9 × 10¹⁸ | 9 × 10¹⁸ (it fits) |
| 4 000 000 000 × 4 000 000 000 = 1.6 × 10¹⁹ | −2 446 744 073 709 551 616 |

This is called **overflow**, and the trouble with it is that nothing happens. No error, no
signal: the program continues with a wrong number. In 1996 the first Ariane 5 rocket was lost
forty seconds after launch because a number too large for the 16 bits it was being stored in
went unnoticed in a piece of code that had worked for years on a slower rocket.

`square` takes any integer. The host can pass it four billion. So `n * n` can leave the word,
the compiler cannot prove otherwise, and it says so. The interval it printed for the product,
`[-inf, +inf]`, is all it knows.

**The word is the target's.** Go, the JVM and Windows have the 64-bit word. JavaScript has no
integers at all: its numbers are floating-point, exact only from −(2⁵³ − 1) to 2⁵³ − 1, and that
is its word (§6.11).

## 6.5 How the compiler knows: intervals

The proof that an operation stays inside the word is arithmetic on **intervals**, and it is
simple enough to do in your head.

<!-- file docs/book/code/integers/intervals.oro -->
```go
export fn sum(a ∈ 0..100, b ∈ 1..50) (1..150) { a + b }

export fn difference(a ∈ 0..100, b ∈ 1..50) (-50..99) { a - b }

export fn product(a ∈ -10..10, b ∈ -3..7) (-70..70) { a * b }

export fn quotient(a ∈ 0..100, b ∈ 1..50) (0..100) { a / b }

export fn remainder(a ∈ 0..100, b ∈ 1..50) (0..49) { a % b }
```

<!-- gen go docs/book/code/integers/intervals.oro -->
```go
note: integers-difference: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-product: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-quotient: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-sum: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func IntegersProduct(v0 int, v1 int) int {
    v23 := (v0 * v1)
    return v23
}
…
```

If `a` is somewhere in 0..100 and `b` somewhere in 1..50, then:

| operation | smallest | largest | so it is in |
|---|---|---|---|
| `a + b` | 0 + 1 | 100 + 50 | 1..150 |
| `a - b` | 0 − 50 | 100 − 1 | −50..99 |
| `a * b`, with −10..10 and −3..7 | −10 × 7 | 10 × 7 | −70..70 |
| `a / b` | 0 / 50 | 100 / 1 | 0..100 |
| `a % b` | 0 | 50 − 1 | 0..49 |

Add the ends, subtract the opposite ends, try all four corners of a product. Each result set in
the source is exactly the interval in the table, and each was accepted: *1 of 1 integer
operations bounded* is the compiler reporting that it did the same sum. Claim one less and it is
refused, with the interval it found:

<!-- file docs/book/code/integers/no-tight.oro -->
```go
export fn too_tight(a ∈ 0..100, b ∈ 1..50) (1..149) { a + b }
```

<!-- gen go docs/book/code/integers/no-tight.oro -->
```
gen: too-tight: the body does not establish 1 <= result && result <= 149; its result is [1, 150]
…
```

And the emitted code is one machine operation each. The proof was done once, by the compiler,
and left nothing behind.

Interval arithmetic is from Ramon Moore's work on numerical error (1966). Using it to prove
facts about a program without running it is the founding example of **abstract interpretation**
(Patrick and Radhia Cousot, 1977): run the program not on numbers but on *descriptions* of
numbers, here "somewhere between lo and hi", and whatever holds of the description holds of
every run. The compiler has finer tools than intervals, and [chapter 16](16-claims.md) is about
them. Most of the time intervals are enough.

## 6.6 A condition is a fact

An interval starts from what a header says. It is narrowed by every condition on the way to an
operation.

<!-- file docs/book/code/integers/facts.oro -->
```go
export fn distance(n ∈ 0..100) (0..50) { if n > 50 { n - 50 } else { 50 - n } }

export fn magnitude(n ∈ -9..9) (0..9) { if n < 0 { -n } else { n } }

export fn both_sides(n ∈ Int) (Int) { if n >= 0 && n < 1000 { n * n } else { 0 } }

export fn the_other_way(n ∈ Int) (Int) { if n < 0 || n >= 1000 { 0 } else { n * n } }

export fn through_a_name(n ∈ Int) (Int) { small = n >= 0 && n < 1000; if small { n * n } else { 0 } }
```

<!-- gen go docs/book/code/integers/facts.oro -->
```go
note: integers-both-sides: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-distance: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-magnitude: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-the-other-way: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-through-a-name: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func IntegersBothSides(v0 int) int {
    v4 := (v0 >= 0)
    v5 := (v4 && (v0 < 1000))
    if v5 {
        v13 := (v0 * v0)
        return v13
    }
    return 0
}
…
```

- `distance`: in the first branch `n > 50` is known, so `n` is in 51..100 and `n - 50` in 1..50.
  In the other, `n` is in 0..50 and `50 - n` in 0..50. The claim `(0..50)` holds on both.
- `magnitude`: the absolute value. Where `n < 0`, `-n` is in 1..9.
- `both_sides`: this is `square` from §6.4, **compiling**. `n` is any integer, and inside the
  branch it is in 0..999, so the product is under a million. A test you would have written
  anyway is the proof.
- `the_other_way`: the `else` of a condition knows its negation. Not (`n < 0` or `n >= 1000`) is
  `n >= 0` and `n < 1000`, by De Morgan's law (§4.5).
- `through_a_name`: the condition may be computed first and tested later. The fact follows the
  value.

A fact also survives a function, since functions are reduced away (§1.2):

<!-- file docs/book/code/integers/clamp.oro -->
```go
fn clamp(n, lo, hi) { case { n < lo -> lo; n > hi -> hi; else -> n } }

export fn squared(n ∈ Int) (0..10^6) { c = clamp(n, 0, 1000); c * c }
```

<!-- gen go docs/book/code/integers/clamp.oro -->
```go
note: integers-squared: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
```

`clamp` has no header and says nothing about its result. The compiler looked at its three arms
after unfolding it: the result is `0`, or `1000`, or an `n` that is neither below 0 nor above
1000. So `c` is in 0..1000 and its square is at most a million, which is what `squared` claims.

## 6.7 Dividing by zero

Overflow is one way an integer operation can fail to mean anything. Division has another.

<!-- file docs/book/code/integers/no-zero.oro -->
```go
export fn share(total ∈ 0..100, people ∈ 0..100) (Int) { total / people }
```

<!-- gen go docs/book/code/integers/no-zero.oro -->
```
gen: integers-share: `/` requires people != 0, which does not follow
  known: assumed -total <= 0, assumed total + -100 <= 0, assumed -people <= 0, assumed people + -100 <= 0
```

There is no integer that is `total / 0`. The hosts disagree about what to do instead: Go and
Java stop the program, and JavaScript produces `Infinity`, which is not an integer, and carries
on. So the language does not pick a behaviour. It states a **precondition**: `/` requires its
right side not to be zero, and a program that cannot show it is refused. The message says what
was required, and under *known* everything the compiler had to work with: `people` is at least 0
and at most 100. Zero is not excluded.

Even a literal zero gets the same treatment, for the same reason, and there the requirement is not
merely unproven but false:

<!-- file docs/book/code/integers/no-zero-literal.oro -->
```go
export fn one_over_nothing() (Int) { 1 / 0 }
```

<!-- gen go docs/book/code/integers/no-zero-literal.oro -->
```
gen: integers-one-over-nothing: `/` requires 0 != 0, which is false at this call
```

Three ways to supply the fact:

<!-- file docs/book/code/integers/zero.oro -->
```go
export fn guarded(total ∈ 0..100, people ∈ 0..100) (Int) {
    if people != 0 { total / people } else { 0 }
}

export fn by_its_set(total ∈ 0..100, people ∈ 1..100) (Int) { total / people }

export fn by_a_where(total ∈ 0..100, people ∈ 0..100) (Int) where people > 0 { total / people }
```

<!-- gen go docs/book/code/integers/zero.oro -->
```go
…
func IntegersByAWhere(v0 int, v1 int) int {
    v30 := (v0 / v1)
    return v30
}

func IntegersByItsSet(v0 int, v1 int) int {
    v23 := (v0 / v1)
    return v23
}

func IntegersGuarded(v0 int, v1 int) int {
    v24 := (v1 == 0)
    if v24 {
        return 0
    }
    v28 := (v0 / v1)
    return v28
}
```

- `guarded` tests, and decides what the answer is when there is nobody to share with. The test
  is in the emitted code because it is in the source.
- `by_its_set` says zero people is not a case: `people ∈ 1..100`.
- `by_a_where` says the same with a `where`, the place in a header for a condition (§6.12).

The second and third emit a bare division. They have moved the obligation to whoever calls
them, and for a function inside the program the compiler will check each call. An `export` is
called by the host, from outside, so there its header is a promise the caller makes.

> **A precondition is what must be true before an operation for it to mean anything.** The word
> and the idea are Tony Hoare's (1969). Most languages check preconditions when the program
> runs, if at all. Here they are checked when it is compiled, and one that does not follow is an
> error with a list of what was known.

## 6.8 Three ways to clear a refusal

Back to `square`. There are three things you can say, and each is visible in the source or on
the command line.

**Narrow the set**, so the operation provably fits. **Or claim a set above the word**, which is
asking for exact arithmetic however large the result:

<!-- file docs/book/code/integers/above.oro -->
```go
export fn small(n ∈ -1000..1000) (Int) { n * n }

export fn wide(n ∈ 0..2^40) (0..2^80) { n * n }
```

<!-- gen go docs/book/code/integers/above.oro -->
```go
note: integers-small: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-wide: 1 operation(s) in arbitrary precision
…
func IntegersSmall(v0 int) int {
    v9 := (v0 * v0)
    return v9
}

func IntegersWide(v0 int) *big.Int {
…
```

`small` is one multiplication. `wide` takes an `int` and returns Go's `*big.Int`, an integer of
any size, because its declared result does not fit a word. §6.10 is about that.

Without the wider claim, the same function is refused, and the message is different from
`square`'s. The compiler knows how large the product can be; it is the word that is too small:

<!-- file docs/book/code/integers/no-above.oro -->
```go
export fn wide(n ∈ 0..2^40) (Int) { n * n }
```

<!-- gen go docs/book/code/integers/no-above.oro -->
```
note: integers-wide: 0 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: integers-wide: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  * [0, 1208925819614629174706176] in n * n
  An operation above is BOUNDED, only not by this target's word: targets whose
  word holds that interval accept the program, and this one refuses it. To compile it
  here too, DECLARE THE RANGE — e.g. a result of `0..N` — which is arbitrary
  precision on this target and a machine word where one holds it. `go run ./cmd/portable`
  reports which targets accept the program.
…
```

**Or ask for the trap.** `-checked` emits the host's own checked arithmetic for every operation
that was not proven, and the program stops at run time if one overflows:

<!-- gen go docs/book/code/integers/no-square.oro -checked -->
```go
…
func IntegersSquare(v0 int) int {
    v1 := (func(a, b int) int { t := a * b; if a != 0 && t/a != b { panic("int overflow") }; return t }(v0, v0))
    return v1
}
```

That is a division after every multiplication, which is why it is not the default: the proven
operations cost nothing, and the trap is what you pay for the ones you chose not to prove.

| you say | you get | it costs |
|---|---|---|
| a narrower set, or a test | the machine's operation, proven | nothing at run time |
| a result set above the word | exact arithmetic of any size | an allocation, where the value leaves the word |
| `-checked` | the machine's operation, and a stop on overflow | a test per unproven operation |

What you cannot get is the fourth thing, the one every host gives by default: the machine's
operation, unproven and unchecked.

## 6.9 The other half of the word, and a famous bug

The refusals above printed *two* intervals for Go's word: `[-9223372036854775808,
9223372036854775807]` and `[0, 18446744073709551615]`. The same 64 bits can be read as a signed
number, −2⁶³ to 2⁶³ − 1, or as an unsigned one, 0 to 2⁶⁴ − 1. Go calls the two readings `int`
and `uint64`. You never write either. The compiler picks the reading that holds the value:

<!-- file docs/book/code/integers/unsigned.oro -->
```go
export fn square(n ∈ 0..2^32-1) (0..2^64-1) { n * n }
```

<!-- gen go docs/book/code/integers/unsigned.oro -->
```go
note: integers-square: 3 operation(s) or conversion(s) in the unsigned word
note: integers-square: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func IntegersSquare(v0 int) uint64 {
    v9 := (uint64(v0))
    v10 := (uint64(v0))
    v11 := ((v9 * v10))
    return v11
}
```

The square of a number under 2³² is under 2⁶⁴: too big for the signed reading, fine for the
unsigned one. The result's set says so, and the function returns a `uint64`.

Now the bug. To search a sorted list you look at the middle, and the middle of `lo` and `hi` is
their average:

<!-- file docs/book/code/integers/midpoint.oro -->
```go
export fn mid(lo ∈ 0..2^63-1, hi ∈ 0..2^63-1) (0..2^63-1) { (lo + hi) / 2 }
```

In 2006 Joshua Bloch reported that this line, in the binary search of Java's standard library,
had been wrong for nine years, and that the same line was wrong in nearly every binary search
ever published, including the one in the textbook he had learned it from. `lo + hi` overflows
when the two are large. The average would have fitted; the *sum on the way to it* does not. In
Java the sum wraps negative, the "middle" is negative, and the program fails with an index out
of range, but only on lists of more than a billion elements, which in 1997 nobody had.

Here is what this compiler does with the line:

<!-- gen go docs/book/code/integers/midpoint.oro -->
```go
note: integers-mid: 6 operation(s) or conversion(s) in the unsigned word
note: integers-mid: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func IntegersMid(v0 int, v1 int) int {
    v23 := (uint64(v0))
    v24 := (uint64(v1))
    v25 := ((v23 + v24))
    v27 := (uint64(2))
    v28 := ((v25 / v27))
    v29 := (int(v28))
    return v29
}
```

The sum of two numbers below 2⁶³ is below 2⁶⁴. It does not fit the signed reading and does fit
the unsigned one, so the compiler adds there, halves there, and reads the result back as signed,
where the claim `(0..2^63-1)` says it belongs. That is one of the two fixes Bloch gave, found
from the sets alone. And with no sets to go on, the line does not compile at all:

<!-- file docs/book/code/integers/no-midpoint.oro -->
```go
export fn mid(lo ∈ Int, hi ∈ Int) (Int) { (lo + hi) / 2 }
```

<!-- gen go docs/book/code/integers/no-midpoint.oro -->
```
note: integers-mid: 0 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: integers-mid: 2 of 2 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  + [-inf, +inf] in lo + hi
  / [-inf, +inf] in (lo + hi) / 2
…
```

The bug is not caught by a clever check for averages. It is the ordinary rule: `+` must be
proven inside a word, and here it is not.

**The two readings do not mix silently.** A value held unsigned is not an `Int` for the host,
and saying it is one is refused, here at the very edge of the word, where `n + 1` can be 2⁶³:

<!-- file docs/book/code/integers/no-edge.oro -->
```go
export fn next(n ∈ 0..2^63-1) (Int) { n + 1 }
```

<!-- gen go docs/book/code/integers/no-edge.oro -->
```
gen: integers-next: a result is [1, 9223372036854775808], which is outside target go's signed word [-9223372036854775808, 9223372036854775807] and inside [0, 18446744073709551615], so it is held in the UNSIGNED word, not an `Int` (ADR 0026, ADR 0033).
…
```

<!-- file docs/book/code/integers/edge.oro -->
```go
export fn next(n ∈ 0..2^63-2) (Int) { n + 1 }
```

<!-- gen go docs/book/code/integers/edge.oro -->
```go
note: integers-next: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
```

One integer of difference in the parameter's set, the largest one, and the answer to this
chapter's opening question changes from "refused" to "proven". That is the "unless" of every
other language, stated where you can see it.

### The one place division overflows

<!-- file docs/book/code/integers/corner.oro -->
```go
export fn positive_divisor(a ∈ -2^63..2^63-1, b ∈ 1..9) (Int) { a / b }

export fn one_less(a ∈ -2^63+1..2^63-1, b ∈ -9..-1) (Int) { a / b }

export fn remainder(a ∈ -2^63..2^63-1, b ∈ -9..-1) (Int) { a % b }
```

<!-- gen go docs/book/code/integers/corner.oro -->
```go
note: integers-one-less: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
note: integers-positive-divisor: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
```

<!-- file docs/book/code/integers/no-corner.oro -->
```go
export fn negative_divisor(a ∈ -2^63..2^63-1, b ∈ -9..-1) (Int) { a / b }
```

<!-- gen go docs/book/code/integers/no-corner.oro -->
```
note: integers-negative-divisor: 0 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
gen: integers-negative-divisor: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  / [-9223372036854775807, 9223372036854775808] in a / b
…
```

A quotient is never larger in size than what was divided, so division looks safe. It has one
exception. The signed word is lopsided: it holds −2⁶³ and not +2⁶³. So −2⁶³ divided by −1, which
is +2⁶³, does not fit. On an Intel processor that division stops the program; in Go and Java it
quietly returns −2⁶³. The compiler found it: the interval printed ends at 9223372036854775808,
one past the word. Exclude either the one dividend or the one divisor and the division is
proven. The remainder never leaves the word and needs nothing.

## 6.10 Above the word

Some integers do not fit in 64 bits, and some programs need them: a factorial, a cryptographic
key, the national debt in cents. A set whose members can exceed the word is the request to
compute with them exactly.

<!-- file docs/book/code/integers/huge.oro -->
```go
use io

fn square(n ∈ Nat) (Nat) { n * n }

export fn main() {
    io.print_line(big_str(2^64))
    io.print_line(big_str(2^200))
    io.print_line(big_str(square(10^20) + 1))
    io.print_line(big_str(2^200 / 10^50))
    io.print_line(big_str(2^200 % 10^9))
    io.print_line(big_str(2^64 - 2^65))
}
```

<!-- run go docs/book/code/integers/huge.oro -->
```
18446744073709551616
1606938044258990275541962092341162602522202993782792835301376
10000000000000000000000000000000000000001
16069380442
835301376
-18446744073709551616
```

<!-- run js docs/book/code/integers/huge.oro -->
```
18446744073709551616
1606938044258990275541962092341162602522202993782792835301376
10000000000000000000000000000000000000001
16069380442
835301376
-18446744073709551616
```

<!-- run windows docs/book/code/integers/huge.oro -->
```
18446744073709551616
1606938044258990275541962092341162602522202993782792835301376
10000000000000000000000000000000000000001
16069380442
835301376
-18446744073709551616
```

`big_str` is the language's own function from an integer of any size to its decimal text, and
`io.print_line` prints text. `square` claims `Nat`, the naturals with no upper end, so its
product is computed exactly, whatever its size.

The three targets hold these numbers three different ways, and the source names none of them:

| target | an integer above the word is |
|---|---|
| Go | `*big.Int`, from Go's `math/big` |
| JavaScript | a `BigInt` |
| Java | a `java.math.BigInteger` |
| Windows | a table of 64-bit digits, written in this language, since there is no library to call |

**Exactness is paid for only where it is needed.** The same `square`, used on small numbers, is
the machine's multiplication: a function inside the program is reduced into each caller (§1.2),
and what decides how a value is held is what is *proven* about it there, not what was claimed
for the function in general. The claim `Nat` gave permission to leave the word. It did not
oblige anyone to.

**The open sets are above the word at an entry point.** An `export` taking a `Nat` can be
handed any natural number, so on Go it takes a `*big.Int`:

<!-- file docs/book/code/integers/open.oro -->
```go
export fn natural(n ∈ Nat) (Nat) { n + 1 }

export fn from_five(n ∈ 5..) (5..) { n }

export fn up_to_five(n ∈ ..5) (..5) { n }
```

<!-- gen go docs/book/code/integers/open.oro -->
```go
note: integers-natural: 1 operation(s) in arbitrary precision
…
func IntegersNatural(v0 *big.Int) *big.Int {
    v2 := (big.NewInt(int64(1)))
    v3 := (new(big.Int).Add(v0, v2))
    return v3
}
…
```

Here, at last, `n + 1` is the next integer with no "unless". (`Int` at an entry point is the
exception: it is taken as the host's word, as §6.3's `any` showed, with nothing known about its
size. That is why arithmetic on a bare `Int` parameter is refused and not made exact.)

**A bounded claim above the word is checked when the program runs**, where the compiler could
not prove it. `wide` of §6.8, in full:

<!-- gen go docs/book/code/integers/above.oro -->
```go
…
func IntegersWide(v0 int) *big.Int {
    v9 := (big.NewInt(int64(v0)))
    v10 := (big.NewInt(int64(v0)))
    v11 := (new(big.Int).Mul(v9, v10))
    v13 := (func(oroZ *big.Int, oroK int) *big.Int { if oroZ.Sign() < 0 || oroZ.BitLen() > oroK { panic("bignum overflow: the declared range is too small") }; return oroZ }(v11, 81))
    return v13
}
```

**A value above the word comes back down where it is proven to fit.** Squaring and then
dividing gives a number in [0, 2⁴⁰], and the compiler brings it back into the word by the
inclusion's inverse, `Int64`, which loses nothing on a value proven inside the word
([ADR 0079](../decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md)
item 4):

<!-- file docs/book/code/integers/down.oro -->
```go
fn square(n ∈ Nat) (Nat) { n * n }

export fn scaled(n ∈ 0..2^40) (0..2^40) { square(n) / 2^40 }
```

<!-- gen go docs/book/code/integers/down.oro -->
```go
…
    v14 := (new(big.Int).Quo(v11, v13))
    v15 := (int(v14.Int64()))
    return v15
}
```

### The compiler's own arithmetic obeys the rule

§1.3 said the compiler computes what it can. It computes integers, not words, so the question
of this chapter applies to it too:

<!-- file docs/book/code/integers/no-fold.oro -->
```go
export fn doubled() (Int) { 2^62 + 2^62 }
```

<!-- gen go docs/book/code/integers/no-fold.oro -->
```
gen: integers-doubled: a result is [9223372036854775808, 9223372036854775808], which is outside target go's signed word [-9223372036854775808, 9223372036854775807] and inside [0, 18446744073709551615], so it is held in the UNSIGNED word, not an `Int` (ADR 0026, ADR 0033).
…
```

2⁶² + 2⁶² is 2⁶³, one past the signed word. A compiler that folded this in 64-bit arithmetic
would write −2⁶³ into the program; one that folded it exactly and then stored it in an `int`
would do the same a step later. This one notices that the constant it computed does not fit
where the header says it goes. **Computing at compile time never changes an answer**
([ADR 0009](../decisions/0009-staging-preserves-results.md)).

## 6.11 Which word, and so which targets

The word is the target's, so whether a program is legal depends on the program *and* the target.
That is computed, and there is a tool to ask:

<!-- file docs/book/code/integers/words.oro -->
```go
export fn small(n ∈ 0..2^26) (Int) { n * n }

export fn third(n ∈ 0..2^60) (Int) { n / 3 }
```

```bash
go run ./cmd/portable docs/book/code/integers/words.oro
```

<!-- portable docs/book/code/integers/words.oro -->
```
…
portable to go, java, windows — not js
W(go, java, windows) = [-9223372036854775808, 9223372036854775807]   (the meet of their words: derived, not assumed — ADR 0026)
```

`small` is fine everywhere: 2²⁶ squared is 2⁵², inside JavaScript's 2⁵³. `third` takes a number
up to 2⁶⁰, which a JavaScript number cannot hold exactly, and JavaScript is refused for it.
Narrow the parameter to `0..2^53-1` and the file is portable to all four.

> **Portability is a property a program has or lacks, and the compiler tells you which.** Most
> languages promise it for every program and then define an `int` that differs from machine to
> machine. Here nothing is promised, and everything is checked.

The same source on JavaScript, where it fits:

<!-- gen js docs/book/code/integers/above.oro -->
```javascript
…
export function integersSmall(v0) {
    return (v0 * v0);
}
…
```

## 6.12 A header carries claims

Every header in this chapter has been making claims. Here is the form in full:

```go
fn name(x ∈ S, y ∈ T) (R) where φ ensure ψ { body }
```

**A parameter's set is an obligation on every caller**
([ADR 0028](../decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)):

<!-- file docs/book/code/integers/no-digit.oro -->
```go
fn digit(d ∈ 0..9) (Int) { d * 2 }

export fn run(n ∈ 0..100) (Int) { digit(n) }
```

<!-- gen go docs/book/code/integers/no-digit.oro -->
```
gen: integers-run: digit's parameter d, declared in 0..9, requires n <= 9 at a call, which does not follow
  known: assumed -n <= 0, assumed n + -100 <= 0
```

`digit` is still reduced into its caller, as every function is. Its claim goes with it: at the
place it was called, `n <= 9` must be provable, and all that is known is `0 <= n <= 100`. This
is §6.7's division by zero again. `/` is a function with a precondition on its right side;
`digit` is a function with one you wrote. Make it provable, and the claim costs nothing:

<!-- file docs/book/code/integers/claims.oro -->
```go
fn digit(d ∈ 0..9) (Int) { d * 2 }

export fn run(n ∈ 0..100) (Int) {
    if n < 10 { digit(n) } else { 0 }
}
```

<!-- gen go docs/book/code/integers/claims.oro -->
```go
func IntegersRun(v0 int) int {
    v10 := (v0 < 10)
    if v10 {
        v13 := (v0 * 2)
        return v13
    }
    return 0
}
```

**`where` relates parameters to each other**, and is an obligation the same way:

<!-- file docs/book/code/integers/no-where.oro -->
```go
fn span(lo ∈ Int, hi ∈ Int) (Int) where lo <= hi { hi - lo }

export fn run(a ∈ 0..10, b ∈ 0..10) (Int) { span(a, b) }
```

<!-- gen go docs/book/code/integers/no-where.oro -->
```
gen: integers-run: span requires a <= b at a call, which does not follow
```

**The result has four forms**, and each is a claim checked against the body:

<!-- file docs/book/code/integers/results.oro -->
```go
export fn inferred(n ∈ 0..9) { n + 1 }

export fn a_set(n ∈ 0..9) (0..10) { n + 1 }

export fn named(n ∈ 0..9) (r ∈ Int) ensure r >= 0 { n * n }

export fn a_group(a ∈ 0..1000, b ∈ 1..1000) (q ∈ Int, m ∈ 0..999) { (a / b, a % b) }
```

<!-- gen go docs/book/code/integers/results.oro -->
```go
func IntegersAGroup(v0 int, v1 int) (int, int) {
    v23 := (v0 / v1)
    v24 := (v0 % v1)
    return v23, v24
}
```

| written | means |
|---|---|
| nothing | no claim; the result is whatever the body gives |
| `(0..10)` | the result is in this set |
| `(r ∈ Int) ensure r >= 0` | the result, named so that `ensure` can speak of it |
| `(q ∈ Int, m ∈ 0..999)` | a pair of results; on Go, two return values |

A result's claim is about *every* argument the parameters' sets allow, and one that fails for
some is refused, with what the body can actually give:

<!-- file docs/book/code/integers/no-result.oro -->
```go
export fn result(n ∈ 0..9) (0..5) { n }
```

<!-- gen go docs/book/code/integers/no-result.oro -->
```
gen: result: the body does not establish 0 <= result && result <= 5; its result is [0, 9]
…
```

`where` is the precondition and `ensure` the **postcondition**: what the function promises in
return. Today `ensure` decides bounds of the result against constants. A promise relating the
result to a parameter is refused as beyond what is decided, never accepted unproven:

<!-- file docs/book/code/integers/no-ensure.oro -->
```go
export fn grows(a ∈ 0..9, b ∈ 0..9) (r ∈ Int) ensure r >= a { a + b }
```

<!-- gen go docs/book/code/integers/no-ensure.oro -->
```
gen: grows: the postcondition result >= a is outside what the compiler decides (constant bounds on the result, and their conjunctions); an obligation is discharged or the program is refused (refinements.md §3a)
…
```

**An entry point's parameters need their sets.** A function used only inside the program takes
its arguments' sets from each use. An `export` is called from outside, where nothing is known:

<!-- file docs/book/code/integers/no-untyped.oro -->
```go
export fn inc(n) { n + 1 }
```

<!-- gen go docs/book/code/integers/no-untyped.oro -->
```
  + [-inf, +inf] in n + 1
```

**A function with no header is used at whatever fits its body**, each use on its own:

<!-- file docs/book/code/integers/generic.oro -->
```go
fn first(a, b) { a }

export fn of_ints(n ∈ 0..9) (Int) { first(n, 0) }

export fn of_strings(s ∈ String) (String) { first(s, "") }
```

<!-- gen go docs/book/code/integers/generic.oro -->
```go
func IntegersOfInts(v0 int) int {
    return v0
}
…
func IntegersOfStrings(v0 string) string {
    return v0
}
```

and what does not fit is refused at the use, in terms of the operation that did not fit:

<!-- file docs/book/code/integers/no-mixed.oro -->
```go
fn double(n) { n + n }

export fn of_strings(s ∈ String) (String) { double(s) }
```

<!-- gen go docs/book/code/integers/no-mixed.oro -->
```
gen: of-strings: in argument 1 of `+`: s is String, but Int is required here
```

How harder claims are proven is [chapter 16](16-claims.md). Where they live: **on the function,
in its header, and nowhere else.** An expression carries no annotation.

## 6.13 What other languages do

| | `n + 1` at the top of the word | to compute it exactly |
|---|---|---|
| C | undefined: the compiler may assume it never happens | a library |
| Java, Go, C# | wraps to the most negative number | a library: `BigInteger`, `math/big` |
| JavaScript | a nearby number that is not `n + 1` | `BigInt`, a separate type that does not mix with numbers |
| Rust | stops the program in a debug build, wraps in a release build | a library |
| Swift | stops the program | a library |
| Python, Lisp, Haskell's `Integer` | exact, always | nothing: every integer may be big, and every operation checks |
| Ada, SPARK | a range on the type, checked at run time, or proven by a separate tool | a library |
| this language | proven to fit, or refused | a set above the word, and exact where the value leaves it |

Three positions, really. *Fast and sometimes wrong* (the first three rows). *Right and always
paying for it*, a test on every operation (Swift, Python). And the one taken here, which Ada
and SPARK came closest to: say what the numbers are, and let a proof decide, operation by
operation, whether the machine's own instruction is the right answer. Where the proof succeeds,
which in the programs written so far is nearly always, the cost is the same as C's and the
answer is Python's.

The price is the one you have seen all chapter. You have to say what your numbers are.

---

## What to remember

- **An integer is an integer.** `Int` is ℤ. An operation computes the integer it means, or the
  program is refused.
- **Literals** are decimal or `0x` hexadecimal; `^` is a power of constants. `* / %` bind
  tighter than `+ -`, and equal strengths group from the left.
- **`/` rounds toward zero, `%` has the sign of its left side, and `(a / b) * b + a % b` is
  `a`.** `mod` is the remainder that is never negative.
- **A range `lo..hi` is a set**, ends included. `Int` is all integers, `Nat` those from 0,
  `lo..` and `..hi` are open at one end, and `set Name = …` names one. A set says what the
  values are, never how they are stored.
- **The machine's word** holds −2⁶³ to 2⁶³ − 1, or 0 to 2⁶⁴ − 1 read unsigned; JavaScript's
  holds ±(2⁵³ − 1). Outside it the machine's arithmetic is not arithmetic.
- **The compiler proves each operation inside the word by intervals**: add the ends, subtract
  the opposite ends, try the corners of a product. **A condition narrows an interval** in the
  branch it guards, and its negation narrows the `else`.
- **Division requires a divisor that is not zero**, a precondition. Supply the fact by a test,
  a set, or a `where`.
- **To clear a refusal**: narrow a set or test; claim a result above the word, for exact
  arithmetic; or build with `-checked`, for a trap.
- **The compiler chooses the representation**: signed word, unsigned word, or the target's big
  integer. `(lo + hi) / 2` is computed in the unsigned word when the sets allow it, and refused
  when nothing is known.
- **Above the word** is the host's big integer, or the language's own digits where the host has
  none. `big_str` gives its text. It does not come back down silently.
- **Portability is computed**: `cmd/portable` says which targets accept a program.
- **A header's sets, `where` and `ensure` are claims**: obligations on callers, promises about
  the result, proven or refused.

Next: chapter 7, Bits. The same integers, looked at as the machine holds them: `&`, `|`, `⊕`,
shifts, and arithmetic that wraps on purpose.
