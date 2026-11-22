# 12. `loop`

Ten chapters, and not one program has done anything more than a fixed number of times.
Chapter 7 hashed the three letters of `abc` by writing the step out three times. Chapter 9
turned a two-digit number into text and stopped at two digits. Each time the text said "that
needs chapter 12".

The usual ways to repeat are closed here. A function may not call itself: chapter 2 refused
that, because a definition is an abbreviation and an abbreviation of itself has no end. And
there is no variable to count with, because a name is bound once and never changes. So the
question of this chapter is: **how does a program do something an unknown number of times,
when nothing can call itself and nothing can change?**

The answer is three sentences.

> **`loop` names some variables and gives each a starting value.** Its arms are tried in
> order, as in `case`.
>
> **An arm either gives the loop's answer, or says `again(…)` with the variables' next
> values.** Nothing is changed: each time round, the names are bound afresh.
>
> **The compiler tries to prove two things about every loop: that its arithmetic fits, and
> that it stops.** The first is required. The second is reported.

Along the way: the oldest algorithm still in use, why a loop is a function that calls itself
*last*, what Turing proved about questions like "does this stop?", and a loop a child can
understand that no mathematician can prove ends.

The examples are in [code/loop/](code/loop/). `go run docs/book/check.go docs/book/12-loop.md`
re-runs every block. Checked 2026-11-16.

---

## 12.1 Adding up

The sum of the numbers from 1 to n.

<!-- file docs/book/code/loop/sum.oro -->
```go
export fn sum_to(n ∈ 0..1000) (Int) {
    loop i = 1, s = 0 {
        i <= n -> again(i + 1, s + i)
        else -> s
    }
}
```

Read it aloud. *Start with `i` as 1 and `s` as 0. If `i` is at most `n`, go round again with
`i + 1` for `i` and `s + i` for `s`. Otherwise the answer is `s`.*

Follow it for `n` = 4. Each row is one time round: the values the two names have, which arm
is chosen, and what it says.

| `i` | `s` | `i <= 4`? | the arm says |
|---|---|---|---|
| 1 | 0 | yes | `again(2, 1)` |
| 2 | 1 | yes | `again(3, 3)` |
| 3 | 3 | yes | `again(4, 6)` |
| 4 | 6 | yes | `again(5, 10)` |
| 5 | 10 | no | the answer is 10 |

<!-- file docs/book/code/loop/sum-run.oro -->
```go
use io

fn sum_to(n ∈ 0..1000) (Int) {
    loop i = 1, s = 0 {
        i <= n -> again(i + 1, s + i)
        else -> s
    }
}

export fn main() {
    io.print_int(sum_to(4))
    io.print_int(sum_to(100))
    io.print_int(sum_to(0))
}
```

<!-- run go docs/book/code/loop/sum-run.oro -->
```
10
5050
0
```

`sum_to(0)` never goes round at all: 1 is not at most 0, so the first test fails and the
answer is the starting `s`.

The parts:

- **`loop i = 1, s = 0`** introduces the **loop variables** and their first values.
- **The arms** are chapter 4's guarded choice: a condition, `->`, a body, tried from the top,
  and the last arm is `else`.
- **`again(i + 1, s + i)`** means *start the loop over*, with these as the new values, in the
  order the variables were declared.
- **An arm with no `again`** is an exit: its value is the value of the whole `loop`.

A `loop` is an expression. It has a value like `if` and `case` do, and here that value is the
function's result.

What does a machine do with it?

<!-- gen go docs/book/code/loop/sum.oro -->
```go
func LoopSumTo(v0 int) int {
    var v12 int = 1
    var v13 int = 0
    for ; ; v12 = (v12 + 1) {
        v14 := (v12 <= v0)
        if v14 {
            v19 := (v13 + v12)
            v13 = v19
            continue
        }
        break
    }
    return v13
}
```

A `for` loop with two variables that are overwritten each time round, which is what a Go
programmer would have written. `again` became `continue`, and the exit became `break`. At the
machine's level the storage for `i` is reused. At the program's level you never see a name
change its value, because each trip round is a fresh binding of the names.

---

## 12.2 All at once

<!-- file docs/book/code/loop/fib.oro -->
```go
use io

fn fib(n ∈ 0..90) (Int) {
    loop i = 0, a = 0, b = 1 {
        i < n && b <= 4 * 10^18 -> again(i + 1, b, a + b)
        else -> a
    }
}

export fn main() {
    io.print_int(fib(0))
    io.print_int(fib(1))
    io.print_int(fib(2))
    io.print_int(fib(10))
    io.print_int(fib(90))
}
```

<!-- run go docs/book/code/loop/fib.oro -->
```
0
1
1
55
2880067194370816120
```

The Fibonacci numbers: 0, 1, 1, 2, 3, 5, 8, … each the sum of the two before. The loop keeps
two neighbours, `a` and `b`, and steps them along. (The `b <= 4 * 10^18` is there for the
compiler, and 12.7 explains it. For `n` up to 90 it is never false.)

Look at `again(i + 1, b, a + b)`. The new `a` is the old `b`. The new `b` is the old `a` plus
the old `b`. **All of `again`'s arguments are computed from the current values, and then all
the variables are rebound together.** In a language with assignment you would write

```
a = b
b = a + b      ← wrong: a has already changed
```

and get it wrong, then fix it with a temporary variable. Here there is no order to get wrong,
because `again` is a call with three arguments, and a call's arguments are all evaluated
before anything is bound. The compiler writes the temporary, where the host needs one:

<!-- file docs/book/code/loop/gcd-gen.oro -->
```go
export fn gcd(a ∈ 0..10^9, b ∈ 0..10^9) (Int) {
    loop x = a, y = b {
        y == 0 -> x
        else -> again(y, x % y)
    }
}
```

<!-- gen go docs/book/code/loop/gcd-gen.oro -->
```go
func LoopGcd(v0 int, v1 int) int {
    var v24 int = v0
    var v25 int = v1
    for {
        v27 := (v25 == 0)
        if v27 {
            break
        }
        v30 := (v24 % v25)
        v24, v25 = v25, v30
        continue
    }
    return v24
}
```

<!-- gen js docs/book/code/loop/gcd-gen.oro -->
```js
export function loopGcd(v0, v1) {
    let v24 = v0;
    let v25 = v1;
    for (;;) {
        if ((v25 === 0)) {
            return v24;
        }
        const u1 = v24;
        v24 = v25;
        v25 = (((u1 % v25) + 0));
        continue;
    }
}
```

Go can assign two variables at once (`v24, v25 = v25, v30`). JavaScript cannot, so there is
`u1`, holding the old value across the overwrite.

---

## 12.3 Euclid

That last function deserves a proper look. It is the oldest algorithm still in daily use:
Euclid wrote it down around 300 BC, and every secure connection your computer opens runs it.

The **greatest common divisor** of two numbers is the largest number that divides both. Of 48
and 18 it is 6. Euclid's observation: anything that divides both `x` and `y` also divides the
remainder of `x` by `y`, and the other way round. So the pair (x, y) has the same common
divisors as the pair (y, x % y), and the second pair is smaller.

| `x` | `y` | the arm says |
|---|---|---|
| 48 | 18 | `again(18, 12)` |
| 18 | 12 | `again(12, 6)` |
| 12 | 6 | `again(6, 0)` |
| 6 | 0 | the answer is 6 |

<!-- file docs/book/code/loop/gcd.oro -->
```go
use io

fn gcd(a ∈ 0..10^9, b ∈ 0..10^9) (Int) {
    loop x = a, y = b {
        y == 0 -> x
        else -> again(y, x % y)
    }
}

export fn main() {
    io.print_int(gcd(48, 18))
    io.print_int(gcd(17, 5))
    io.print_int(gcd(1000000000, 999999937))
    io.print_int(gcd(0, 7))
}
```

<!-- run go docs/book/code/loop/gcd.oro -->
```
6
1
1
7
```

Two things to notice. The exit is the *first* arm here and the `again` is in the `else`:
either order is fine, and you write whichever reads better. And nobody can say in advance
how many times this loop goes round. For 48 and 18 it is three. For two neighbouring
Fibonacci numbers it is as many as it can possibly be. That is the kind of repetition a fixed
number of written-out steps can never do.

`x % y` needs `y` not to be zero (chapter 6). It is proven: the first arm took the case
`y == 0`, so in the `else` arm the compiler knows `y` is not zero. An arm's guard is a fact in
that arm and its opposite is a fact in every arm below it.

---

## 12.4 Doing things

A loop's body can have effects (chapter 5), and then each trip round does them:

<!-- file docs/book/code/loop/count.oro -->
```go
use io

export fn main() {
    loop i = 1 {
        i <= 3 -> {
            io.print_int(i)
            again(i + 1)
        }
        else -> io.print_line("done")
    }
}
```

<!-- run go docs/book/code/loop/count.oro -->
```
1
2
3
done
```

An arm's body may be a block: statements, and then, last, either `again` or a value.

A loop can build text, a piece at a time:

<!-- file docs/book/code/loop/stars.oro -->
```go
use io

fn stars(n ∈ 0..80) (String) {
    loop i = 0, s = "" {
        i < n -> again(i + 1, s ++ "*")
        else -> s
    }
}

export fn main() {
    io.print_line(stars(1))
    io.print_line(stars(2))
    io.print_line(stars(3))
    io.print_line(stars(10))
    io.print_line("[" ++ stars(0) ++ "]")
}
```

<!-- run go docs/book/code/loop/stars.oro -->
```
*
**
***
**********
[]
```

`s` starts as the empty string, chapter 9's "value that changes nothing", and each trip joins
one more star to it. This is the shape of nearly every loop you will write: one variable that
counts, and one that **accumulates** an answer, starting from the value that changes nothing
for the operation in use. 0 for a sum. 1 for a product. `""` for a join.

---

## 12.5 Where `again` may stand

`again` is not a function, though it is written like a call. It is a **jump**: go back to the
top with these values. A jump has to be the last thing an arm does, because nothing can
happen after it. So it is allowed in one place, the end of an arm, and each of the following
is refused.

Inside an expression:

<!-- file docs/book/code/loop/no-inside.oro.txt -->
```go
export fn count(n ∈ 0..9) (Int) {
    loop i = 0 {
        i < n -> 1 + again(i + 1)
        else -> 0
    }
}
```

<!-- gen go docs/book/code/loop/no-inside.oro.txt -->
```
gen: docs/book/code/loop/no-inside.oro.txt: line 2: `again` may be a clause body, or sit under a `let`, but not under an `if` or inside an expression — write another clause instead, so the clause list stays the loop's whole control flow
```

`1 + again(…)` asks to go round again *and then* add 1 to what comes back. Nothing comes
back from a jump. (What you meant is an accumulator: carry the count in a second variable.)

Under an `if`:

<!-- file docs/book/code/loop/no-under-if.oro.txt -->
```go
export fn count(n ∈ 0..9) (Int) {
    loop i = 0 {
        else -> if i < n { again(i + 1) } else { i }
    }
}
```

<!-- gen go docs/book/code/loop/no-under-if.oro.txt -->
```
gen: docs/book/code/loop/no-under-if.oro.txt: line 3: `again` under an if (ADR 0015)
```

The message before this one says why: "write another clause instead, so the clause list
stays the loop's whole control flow". The arms of a loop are a complete list of what the loop
can do: every way round and every way out, in one flat column, readable at a glance. A jump
buried in a nested `if` would be a way round that the list does not show.

With the wrong number of values:

<!-- file docs/book/code/loop/no-arity.oro.txt -->
```go
export fn count(n ∈ 0..9) (Int) {
    loop i = 0, s = 0 {
        i < n -> again(i + 1)
        else -> s
    }
}
```

<!-- gen go docs/book/code/loop/no-arity.oro.txt -->
```
gen: docs/book/code/loop/no-arity.oro.txt: line 2: again takes 2 argument(s), one per loop variable, given 1
```

And a loop with no `else` has a case it does not answer:

<!-- file docs/book/code/loop/no-else.oro.txt -->
```go
export fn count(n ∈ 0..9) (Int) {
    loop i = 0 {
        i < n -> again(i + 1)
    }
}
```

<!-- gen go docs/book/code/loop/no-else.oro.txt -->
```
gen: docs/book/code/loop/no-else.oro.txt: line 2: the last arm of loop must be `else`, and only the last, so that every path out of the loop is one of its arms (ADR 0015)
```

### A loop is a function that calls itself last

Here is the program a newcomer from most languages writes first:

<!-- file docs/book/code/loop/no-recursion.oro -->
```go
fn count(n) { if n == 0 { 0 } else { 1 + count(n - 1) } }

export fn run(n ∈ 0..9) (Int) { count(n) }
```

<!-- gen go docs/book/code/loop/no-recursion.oro -->
```
gen: docs/book/code/loop/no-recursion.oro: count is recursive: it is defined in terms of itself, and recursion is not in the language, since δ unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with `again` (ADR 0015).
```

A function that calls itself has to *remember* what to do when the call returns: here, "add
1". Each call that is waiting takes memory, on what is called the stack, and the stack is a
different size on every host: a recursion a million deep runs on one and crashes on another.
That alone is reason to leave it out of a language that promises one meaning everywhere.

But one kind of self-call needs no memory: a call that is the *last* thing the function
does, with nothing left to do afterwards. There is nothing to remember, so the call can be a
plain jump back to the top with new arguments. Guy Steele showed in 1977 that this is all a
loop ever was, in a paper titled *Lambda: The Ultimate GOTO*.

That is what `loop` and `again` are. `loop i = 1, s = 0 { … }` is a function of `i` and `s`,
called at once with 1 and 0. `again(a, b)` is that function calling itself, allowed only in
the last position, where the call costs nothing. The rule of this section, "`again` only at
the end of an arm", is the rule "only the free kind of recursion".

And it explains the first puzzle. Nothing changes its value, yet the loop makes progress,
for the same reason a recursive function does: each call has its own `i`.

---

## 12.6 More than one way out, and any kind of answer

Is 91 prime? Try dividing by 2, 3, 4, … If something divides it, that is its smallest
divisor. If you get past its square root with no luck, it has none, and it is prime.

<!-- file docs/book/code/loop/prime.oro -->
```go
use io

fn smallest_divisor(n ∈ 2..10^12) (Int) {
    loop d = 2 {
        d > 10^6 -> n
        d * d > n -> n
        n % d == 0 -> d
        else -> again(d + 1)
    }
}

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

fn prime?(n ∈ 2..10^12) (Bool) { smallest_divisor(n) == n }

export fn main() {
    io.print_int(smallest_divisor(91))
    io.print_int(smallest_divisor(97))
    io.print_int(bit(prime?(97)))
    io.print_int(bit(prime?(1000000007)))
    io.print_int(smallest_divisor(999999999989))
}
```

<!-- run go docs/book/code/loop/prime.oro -->
```
7
97
1
1
999999999989
```

91 is 7 × 13. 97, a billion and seven, and 999999999989 are prime. The last one made the
loop go round a million times, and it printed at once.

Three arms leave this loop and one goes round. In a language with `while`, a loop has one
exit condition at the top, and leaving from the middle takes a `break` or a flag variable.
Here an exit is just an arm that does not say `again`, and there can be as many as the
problem has. (The first arm, `d > 10^6`, never fires for these inputs: 10⁶ squared is 10¹²,
so the second arm always gets there first. It is for the compiler, like `fib`'s extra test,
and 12.7 is about it.)

A loop's answer can be any value. A tuple, a variant:

<!-- file docs/book/code/loop/values.oro -->
```go
use io

fn divmod(a ∈ 0..10^6, b ∈ 1..10^6) {
    loop q = 0, r = a {
        r >= b -> again(q + 1, r - b)
        else -> (q, r)
    }
}

fn root(n ∈ 0..10^6) {
    loop r = 0 {
        r > 1000 -> 'none
        r * r == n -> 'some(r)
        r * r > n -> 'none
        else -> again(r + 1)
    }
}

fn show(found) {
    case found {
        'some(r) -> io.print_int(r)
        'none -> io.print_int(-1)
    }
}

export fn main() {
    (q, r) = divmod(17, 5)
    io.print_int(q)
    io.print_int(r)
    show(root(144))
    show(root(145))
}
```

<!-- run go docs/book/code/loop/values.oro -->
```
3
2
12
-1
```

`divmod` is division the way you first learned it: keep taking `b` away, counting, until what
is left is smaller than `b`. Its exit gives chapter 10's pair. `root` searches for an exact
square root and answers `'some(r)` or `'none`: a search is the natural home of chapter 10's
"something or nothing".

And now chapter 9's promise can be kept. A number of any size, as text:

<!-- file docs/book/code/loop/digits.oro -->
```go
use io

fn digits(n ∈ 0..10^15) (String) {
    loop m = n, s = "" {
        m >= 10 -> again(m / 10, string_of(48 + m % 10) ++ s)
        else -> string_of(48 + m) ++ s
    }
}

export fn main() {
    io.print_line(digits(0))
    io.print_line(digits(7))
    io.print_line(digits(1234567890))
    io.print_line("total: " ++ digits(40 + 2) ++ " items")
}
```

<!-- run go docs/book/code/loop/digits.oro -->
```
0
7
1234567890
total: 42 items
```

`m % 10` is the last digit and `m / 10` is everything before it. Each trip peels the last
digit off `m` and puts its character on the *front* of `s`, so the digits come out in the
right order though they were found backwards. This is how `io.print_int` works inside, on
every computer there is.

<!-- run js docs/book/code/loop/digits.oro -->
```
0
7
1234567890
total: 42 items
```

<!-- run windows docs/book/code/loop/digits.oro -->
```
0
7
1234567890
total: 42 items
```

---

## 12.7 The word, in a loop

Chapter 6's rule has not gone anywhere: an operation computes the integer it means, in the
machine's word where that is proven to hold it, and a value claimed in a set it is not proven to
be in is refused. A loop makes the proof harder, because an operation in a loop is not done once.
Take the first example, loosen its header, and claim the word for its result:

<!-- file docs/book/code/loop/no-bound.oro -->
```go
export fn sum_to(n ∈ Int) (-2^63..2^63-1) {
    loop i = 1, s = 0 {
        i <= n -> again(i + 1, s + i)
        else -> s
    }
}
```

<!-- gen go docs/book/code/loop/no-bound.oro -->
```
gen: sum-to: the body does not establish -9223372036854775808 <= result && result <= 9223372036854775807; its result is [0, +inf]
```

With `n` any integer the loop may go round any number of times, so `i` and `s` may grow
without limit, and the claim is false. (Claim `Int` instead and it compiles, `i` and `s` held as
Go's `*big.Int`: exact, and an allocation per trip.) With `n ∈ 0..1000` it compiled in the word,
and here is the reasoning the compiler did:

- `i` starts at 1 and goes up by 1 while `i <= n`. So inside the arm `i` is at most 1000, and
  `i + 1` at most 1001.
- The loop therefore goes round at most 1000 times.
- `s` starts at 0 and gains at most 1000 each time. After at most 1000 trips it is at most a
  million.

A bound on the *number of trips* turned into a bound on the *accumulator*. That works for
sums, because adding the same bounded amount a bounded number of times is a multiplication.

The bound can lie past the word and still be a bound. With `n ∈ 0..2^40` and the result
claimed `Int`, `s` is at most n(n + 1)/2, below 2⁸⁰: past the word, inside the double word of
chapter 6, two words side by side. So the loop carries `s` as two words, each trip an addition
with its carry, and only the end makes it the `*big.Int` the claim asks for:

<!-- file docs/book/code/loop/wide-sum.oro -->
```go
export fn sum_to(n ∈ 0..2^40) (Int) {
    loop i = 1, s = 0 {
        i <= n -> again(i + 1, s + i)
        else -> s
    }
}
```

<!-- gen go docs/book/code/loop/wide-sum.oro -->
```go
…
    var v13h, v13l uint64 = uint64(0), uint64(0)
…
            v19l, v19c := bits.Add64(v13l, uint64(v12), 0)
            v19h := v13h + uint64(v12 >> 63) + v19c
            v13h, v13l = v19h, v19l
…
    v35 := (new(big.Int).Add(v34, v29))
    return v35
}
```

The end takes the two words apart into three pieces of 32 bits and puts them together as an
integer, `h·2⁶⁴ + m·2³² + l`, which is the same number written in base 2³².

It does not work for products:

<!-- file docs/book/code/loop/no-fact.oro -->
```go
export fn fact(n ∈ 0..20) (-2^63..2^63-1) {
    loop i = 1, p = 1 {
        i <= n -> again(i + 1, p * i)
        else -> p
    }
}
```

<!-- gen go docs/book/code/loop/no-fact.oro -->
```
gen: fact: the body does not establish -9223372036854775808 <= result && result <= 9223372036854775807; its result is [1, +inf]
```

20 factorial is 2432902008176640000, which fits Go's word with a little to spare, and 21
factorial does not. You know that because you can multiply twenty numbers. The compiler's
method is to reason about one trip round and how values change across it, and "multiplied by
at most 20, at most 20 times" gives 20²⁰, far past the word. It cannot see that the factors
are 1, 2, 3, … and not 20 every time.

There are two honest ways out, and they mean different things.

**Say the bound in the loop.** If you know `p` never needs to pass a certain size, test it:

<!-- file docs/book/code/loop/fact.oro -->
```go
export fn fact(n ∈ 0..20) (Int) {
    loop i = 1, p = 1 {
        i <= n && p <= 4 * 10^17 -> again(i + 1, p * i)
        else -> p
    }
}
```

<!-- gen go docs/book/code/loop/fact.oro -->
```go
note: loop-fact: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
…
func LoopFact(v0 int) int {
    var v12 int = 1
    var v13 int = 1
    for ; ; v12 = (v12 + 1) {
        v15 := (v12 <= v0)
        v16 := (v15 && (v13 <= 400000000000000000))
        if v16 {
            v29 := (v13 * v12)
            v13 = v29
            continue
        }
        break
    }
    return v13
}
```

Now inside the arm `p` is at most 4 × 10¹⁷ and `i` at most 20, so `p * i` is at most 8 × 10¹⁸,
inside the word. The test is real: it is in the Go, and it runs. For `n` up to 20 it is never
false (19 factorial is about 1.2 × 10¹⁷), so the answers are right. It is a guard you are sure
of, written where the compiler can lean on it. That is what `b <= 4 * 10^18` was doing in
`fib`, and `d > 10^6` in `smallest_divisor`.

Be honest with yourself about such a guard. If it *could* be false, the loop stops early and
returns a wrong answer without a word. Writing it is a claim that it cannot, and the claim is
yours.

(Chapter 16 has the other tool: a loop may state an *invariant*, which the compiler proves
and which never runs. It is for statements about sums. This one is about a product, and
stays a guard.)

**Or leave the word.** Chapter 6's other answer: claim `Int`, or a result that does not fit,
and the value is held with as many digits as it needs.

<!-- file docs/book/code/loop/fact-run.oro -->
```go
use io

fn fact(n ∈ 0..20) (Int) {
    loop i = 1, p = 1 {
        i <= n && p <= 4 * 10^17 -> again(i + 1, p * i)
        else -> p
    }
}

fn big_fact(n ∈ 0..30) (Nat) {
    loop i = 1, p = 1 {
        i <= n -> again(i + 1, p * i)
        else -> p
    }
}

export fn main() {
    io.print_int(fact(5))
    io.print_int(fact(20))
    io.print_line(decimal(big_fact(25)))
}
```

<!-- run go docs/book/code/loop/fact-run.oro -->
```
120
2432902008176640000
15511210043330985984000000
```

`big_fact` has no guard and needs none: `Nat` has no upper end, so there is nothing to
overflow. 25 factorial has 26 digits. It costs what big numbers cost, and it is exact.

---

## 12.8 Does it stop?

Every `gen` in this chapter has printed a line you may have skimmed:

```
note: loop-sum-to: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
```

"1 of 1 loop(s) proven terminating." The compiler tries to prove that each loop ends, for
every input the header allows. Here are four it proves, each for a different reason:

<!-- file docs/book/code/loop/stops.oro -->
```go
export fn up(n ∈ 0..1000) (Int) {
    loop i = 0 {
        i < n -> again(i + 1)
        else -> i
    }
}

export fn down(n ∈ 0..1000) (Int) {
    loop i = n, s = 0 {
        i > 0 -> again(i - 1, s + i)
        else -> s
    }
}

export fn halvings(n ∈ 1..10^9) (Int) {
    loop x = n, k = 0 {
        x > 1 -> again(x / 2, k + 1)
        else -> k
    }
}

export fn meet(n ∈ 0..100) (Int) {
    loop i = 0, j = n {
        i < j -> again(i + 1, j - 1)
        else -> i
    }
}
```

<!-- gen go docs/book/code/loop/stops.oro -->
```
note: loop-down: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
note: loop-halvings: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
note: loop-halvings: 1 division(s) became a shift or a mask
note: loop-meet: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
note: loop-up: 1 of 1 integer operations bounded; 1 of 1 loop(s) proven terminating
```

The argument is the same each time, and it is the only argument there is for a loop ending:

> **Find a whole number that gets strictly smaller every time round, and that cannot go below
> some floor.** A whole number cannot go down forever.

- `up`: the distance `n - i` shrinks by 1 each trip and the guard `i < n` keeps it above 0.
- `down`: `i` itself shrinks and the guard keeps it above 0.
- `halvings`: `x / 2` is less than `x` when `x > 1`.
- `meet`: the gap `j - i` shrinks by 2.
- And Euclid's loop from 12.3: `x % y` is always less than `y`, so `y` shrinks, and it cannot
  go below 0. That is Euclid's own proof that his method ends.

Such a number is called a **measure** (or a variant, or a ranking function). When you write a
loop, ask what its measure is. If you cannot name one, you do not yet know that it stops.

Now a loop with none:

<!-- file docs/book/code/loop/stuck.oro -->
```go
export fn stuck(n ∈ 0..9) (Int) {
    loop i = 0 {
        i < n -> again(i)
        else -> i
    }
}
```

<!-- gen go docs/book/code/loop/stuck.oro -->
```go
note: loop-stuck: 0 of 0 integer operations bounded; 0 of 1 loop(s) proven terminating
…
func LoopStuck(v0 int) int {
    var v11 int = 0
    for {
        v12 := (v11 < v0)
        if v12 {
            continue
        }
        break
    }
    return v11
}
```

`again(i)`, not `again(i + 1)`: a one-character slip, and for any `n` above 0 this goes round
forever. The note says **0 of 1**, and the program compiled anyway.

That is deliberate, and it is the one place in this book where the compiler reports a doubt
without refusing. The reason is a theorem. In 1936 Alan Turing proved that no program can
decide, for every program, whether it stops: the **halting problem**. Any checker must
sometimes answer "I cannot tell" about a loop that is in fact fine. If "I cannot tell" meant
"refused", there would be correct programs you could never compile. So an unproven overflow
is refused, because you can always fix it by saying more, and an unproven termination is
counted and shown. Read the count. **A loop the compiler could not prove is a loop to look at
again.**

Here is a loop no one can prove, in any language:

<!-- file docs/book/code/loop/collatz-run.oro -->
```go
use io

fn collatz(n ∈ 1..10^6) (Int) {
    loop x = n, steps = 0 {
        x == 1 -> steps
        x > 10^15 || steps >= 10000 -> -1
        x % 2 == 0 -> again(x / 2, steps + 1)
        else -> again(3 * x + 1, steps + 1)
    }
}

export fn main() {
    io.print_int(collatz(1))
    io.print_int(collatz(6))
    io.print_int(collatz(27))
    io.print_int(collatz(837799))
}
```

<!-- run go docs/book/code/loop/collatz-run.oro -->
```
0
8
111
524
```

Take a number. If it is even, halve it. If it is odd, triple it and add one. Repeat until
you reach 1. Starting from 6: 3, 10, 5, 16, 8, 4, 2, 1, eight steps. Starting from 27 it
climbs as high as 9232 before falling, 111 steps in all.

Does this reach 1 from every starting number? Lothar Collatz asked in 1937. It has been
checked by machine for every start up to about 3 × 10²⁰. Nobody has a proof, and Paul Erdős
said mathematics is not yet ready for such problems. There is no measure anyone can name: `x`
goes up as well as down.

So how did this version get "proven terminating"? Look at its second arm. It gives up, with
−1, if `x` passes 10¹⁵ or after 10000 steps. That arm is the measure: `10000 - steps` shrinks
every trip. It also bounds `x`, which the arithmetic needed. This is the practical answer to
the halting problem. You cannot always prove your loop stops for the reason you believe. You
can always *make* it stop, by counting, and then the proof is trivial and the doubt has
become a visible −1.

And here is a loop whose measure you can name, and the compiler cannot see. Chapter 7's `x & (x
- 1)` clears the lowest 1 of `x`, so counting how often it can be done before `x` is 0 counts the
ones (Peter Wegner, 1960):

<!-- file docs/book/code/loop/ones.oro -->
```go
use io

fn ones(v ∈ Nat) (Nat) {
    loop x = v, c = 0 { x == 0 -> c; else -> again(x & (x - 1), c + 1) }
}

export fn main() {
    io.print_line(decimal(ones(255)))
    io.print_line(decimal(ones(3^1000)))
}
```

<!-- run go docs/book/code/loop/ones.oro -->
```
8
758
```

<!-- gen go docs/book/code/loop/ones.oro -->
```
note: loop-main: 66 operation(s) in arbitrary precision
note: loop-main: 2 of 2 integer operations bounded; 0 of 2 loop(s) proven terminating
…
```

3¹⁰⁰⁰ is a number of 1585 bits, held exactly (chapter 6), and 758 of them are ones. The measure
is the number of ones in `x`: each trip removes one, so the loop runs exactly that many times and
stops. The compiler counts the loop as not proven, because its check compares sizes of values,
and the number of ones in a 1585-bit number is not a size it knows how to read. The loop is
right, and the count is honest about who proved it: you did.

---

## 12.9 Loops inside loops

A loop is an expression, so it can stand anywhere a value can, including inside another
loop's arm:

<!-- file docs/book/code/loop/nested.oro -->
```go
export fn pairs(n ∈ 0..100) (Int) {
    loop i = 0, total = 0 {
        i < n -> {
            row = loop j = 0, s = 0 {
                j < i -> again(j + 1, s + j)
                else -> s
            }
            again(i + 1, total + row)
        }
        else -> total
    }
}
```

<!-- gen go docs/book/code/loop/nested.oro -->
```go
note: loop-pairs: 4 of 4 integer operations bounded; 2 of 2 loop(s) proven terminating
…
func LoopPairs(v0 int) int {
    var v12 int = 0
    var v13 int = 0
    for ; ; v12 = (v12 + 1) {
        v14 := (v12 < v0)
        if v14 {
            var v22 int = 0
            var v23 int = 0
            for ; ; v22 = (v22 + 1) {
                v24 := (v22 < v12)
                if v24 {
                    v29 := (v23 + v22)
                    v23 = v29
                    continue
                }
                break
            }
            v32 := (v13 + v23)
            v13 = v32
            continue
        }
        break
    }
    return v13
}
```

The inner loop runs to completion, its value is named `row`, and the outer arm carries on.
Each `again` belongs to the nearest loop around it: the inner one restarts the inner loop,
the outer one the outer. Both loops are proven to stop, and all four additions to fit, the
inner bound depending on the outer variable.

When a loop body grows, the cleaner habit is 12.4's: give the inner loop a name by making it
a function (`stars`), and call it.

An inner loop may give several values, which a tuple pattern takes apart before the outer loop
goes on:

<!-- file docs/book/code/loop/digitsums.oro -->
```go
use io

export fn main() {
    // the digit sums and the digit counts of 1 to 1000, all at once
    (sum, count) = loop i = 1, sum = 0, count = 0 {
        i <= 1000 -> {
            (s, d) = loop m = i, s = 0, d = 0 {
                m > 0 -> again(m / 10, s + m % 10, d + 1)
                else -> (s, d)
            }
            again(i + 1, sum + s, count + d)
        }
        else -> (sum, count)
    }
    io.print_int(sum)
    io.print_int(count)
}
```

<!-- run go docs/book/code/loop/digitsums.oro -->
```
13501
2893
```

<!-- gen go docs/book/code/loop/digitsums.oro -->
```
note: loop-main: 6 of 6 integer operations bounded; 2 of 2 loop(s) proven terminating
…
```

`(s, d) = loop …` is chapter 10's pattern over a loop's value, and `again` after it belongs to the
outer loop. Both loops stop, `m / 10` shrinking `m` and `i` climbing to 1000.

---

## 12.10 A loop is where reduction stops

Chapters 1 to 4 had a refrain: whatever the compiler knows, it computes. A call of a known
function is unfolded, an `if` on a known condition picks its branch. A loop is the exception.

<!-- file docs/book/code/loop/static.oro -->
```go
export fn three() (Int) {
    loop i = 0, s = 0 {
        i < 3 -> again(i + 1, s + i)
        else -> s
    }
}

export fn once(n ∈ 0..9) (Int) {
    loop i = n {
        else -> i + 1
    }
}
```

<!-- gen go docs/book/code/loop/static.oro -->
```go
func LoopOnce(v0 int) int {
    v10 := (v0 + 1)
    return v10
}

func LoopThree() int {
    var v3 int = 0
    var v4 int = 0
    for ; ; v3 = (v3 + 1) {
        v6 := (v3 < 3)
        if v6 {
            v10 := (v4 + v3)
            v4 = v10
            continue
        }
        break
    }
    return v4
}
```

Everything about `three` is known: it goes round three times and its answer is 3. The
emitted Go is a loop all the same. **The compiler does not run loops.** A loop is the
construct that says "this part is for the machine". If reduction ran loops, compiling a
program could take as long as running it, or never finish (think of `stuck`).

`once` shows the other edge. A `loop` in which nothing says `again` is not a loop: it is a
block that binds `i` and gives an answer, and it reduces away like one.

On the tutorial target, where reduction is all there is, a loop comes back as it went in:

<!-- file docs/book/code/loop/residual.oro -->
```go
export fn fold(u) {
    loop i = 0, s = u {
        i < 3 -> again(i + 1, h(s, i))
        else -> s
    }
}
```

<!-- oro tutorial docs/book/code/loop/residual.oro -->
```go
fold =
fn(u) { loop i = 0, s = u { i < 3 -> again(i + 1, h(s, i)); else -> s } }
```

This gives the language its two levels, which chapter 1 hinted at. Functions, tuples,
variants and `case` on known things belong to the level that is gone by the time the program
runs. A `loop` over values that arrive at run time is the level that remains.

---

## 12.11 What other languages do

| | repetition | what changes | may leave from the middle? | is stopping checked? |
|---|---|---|---|---|
| Oroboros | `loop` / `again` | nothing; variables are rebound together | yes: any arm without `again` | proven where possible, and counted |
| C, Java, Go, JavaScript | `while`, `for` | variables are assigned, one at a time | `break`, `return` | no |
| Python | `while`, `for … in` | assignment | `break` | no |
| Scheme, Haskell, ML | a function calling itself | nothing | it is a function's result | no (Haskell, ML); proper tail calls are guaranteed in Scheme |
| Clojure | `loop` / `recur` | nothing; rebound together | yes | no |
| Rust | `loop`, `while`, `for` | assignment, checked by the borrow rules | `break` with a value | no |
| Coq, Agda, Lean | recursion on a shrinking argument | nothing | — | **required**: a function that is not proven to stop is refused |

Clojure's `loop` and `recur` are the same construct as this chapter's, for the same reason:
its host, the JVM, cannot make a self-call free, so the language gives the free kind its own
keyword and refuses it anywhere but last. Scheme's "named `let`" is the ancestor of both.

The proof assistants at the bottom take the other road on Turing's theorem: every function
*must* come with a proof that it stops, and the price is that some ordinary programs need a
real mathematical argument before they compile. This language sits between them and the
rest: it proves what it can, refuses nothing on that ground, and tells you the score.

---

## What to remember

- **`loop x = a, y = b { … }`** binds loop variables to starting values and tries its arms in
  order. The last arm is `else`.
- **`again(a', b')`** goes round with new values, one per variable, in order. All are computed
  from the current values, then bound together.
- **An arm without `again` is an exit**, and its value is the loop's value. There may be
  several exits. The value may be a number, text, a tuple, a variant.
- **`again` is a jump and stands only at the end of an arm**: not inside an expression, not
  under an `if`.
- **Nothing changes.** A loop is a function that calls itself last, which costs no memory.
  Ordinary recursion is refused.
- **Most loops count with one variable and accumulate with another**, starting from the value
  that changes nothing: 0, 1, `""`.
- **Arithmetic in a loop must still be proven to fit.** A bound on the number of trips bounds
  a sum. A product needs a bound you state as a guard, or a result above the word.
- **Every loop is checked for stopping**: a whole number that shrinks each trip and has a
  floor. The count "n of m loop(s) proven terminating" is printed. An unproven loop still
  compiles: look at it again.
- **When no measure exists, make one**: count the steps and give up at a limit.
- **The compiler does not run loops.** They are what is left for the machine.

Next: [chapter 13](13-tables.md), Tables. Many values of one kind, and how an index is proven to be inside.
