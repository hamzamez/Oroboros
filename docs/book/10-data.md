# 10. Tuples and variants

So far a value has been one thing: an integer, a number, a boolean, a string. Real programs
deal in things with parts and things with alternatives. A division has a quotient *and* a
remainder. A traffic light is red *or* green *or* amber. A search finds something *or* finds
nothing.

"And" and "or". Every way of structuring data that has ever been invented is built from those
two words, and most languages give you a good tool for the first and a poor one for the
second. So the question of this chapter is: **how does a program say "this and that", and how
does it say "this or that"?**

The answer is three sentences.

> **A tuple is several values at once**, written `(a, b)`. You take it apart by naming its
> parts.
>
> **A variant is exactly one of several named cases**, written `'red` or `'circle(5)`. You
> take it apart with `case`, which must answer for every case.
>
> **Neither exists when the program runs**, except where a value crosses the program's edge.

Along the way: why a function with two results needs no new idea, the most expensive mistake
in the history of programming languages (its inventor's words), why these are called products
and sums, and what a compiler can check for you once it knows all the cases.

The examples are in [code/data/](code/data/). `go run docs/book/check.go docs/book/10-data.md`
re-runs every block. Checked 2026-11-16.

---

## 10.1 Two answers at once

Chapter 6 divided 17 by 5 twice: `17 / 5` for the quotient and `17 % 5` for the remainder. They
are two halves of one answer, and a function should be able to give both.

<!-- file docs/book/code/data/divmod.oro -->
```go
use io

fn divmod(a ∈ 0..1000, b ∈ 1..1000) { (a / b, a % b) }

fn hours_minutes(minutes ∈ 0..1439) { divmod(minutes, 60) }

export fn main() {
    (q, r) = divmod(17, 5)
    io.print_int(q)
    io.print_int(r)

    (h, m) = hours_minutes(135)
    io.print_int(h)
    io.print_int(m)

    (n, word, yes) = (1, "two", true)
    io.print_line(word)
}
```

<!-- run go docs/book/code/data/divmod.oro -->
```
3
2
2
15
two
```

**`(a / b, a % b)` is a tuple**: two values side by side, in parentheses, with a comma. It is
one value made of two.

**`(q, r) = divmod(17, 5)` takes it apart.** On the left of `=`, where one name would stand,
there is a *pattern*: the shape of a tuple with a name in each place. `q` gets the first part
and `r` the second. From then on `q` and `r` are ordinary names.

`hours_minutes` returns whatever `divmod` returned, without looking inside: 135 minutes is 2
hours and 15 minutes. A tuple can be passed along whole like any value.

The last tuple has three parts of three different kinds: an integer, a string and a boolean.
The parts of a tuple need not be alike, and there may be any number of them. What is fixed is
*how many* and *in which order*: a tuple of three is never a tuple of two.

You have been using tuples since chapter 1 without the word. A function's parameter list
`(a, b)` is a tuple pattern, and its argument list `(17, 5)` is a tuple. And chapter 4's `()`,
"nothing", is the tuple with no parts.

---

## 10.2 A tuple in a header

A header says which set a result is in. For a tuple it says so part by part:

<!-- file docs/book/code/data/header.oro -->
```go
export fn divmod(a ∈ 0..1000, b ∈ 1..1000) (Int, Int) { (a / b, a % b) }

export fn swap(a ∈ Int, b ∈ Int) (Int, Int) { (b, a) }

export fn three(a ∈ 0..9) (Int, Bool, String) { (a, a > 4, "x") }
```

<!-- gen go docs/book/code/data/header.oro -->
```go
func DataDivmod(v0 int, v1 int) (int, int) {
    v23 := (v0 / v1)
    v24 := (v0 % v1)
    return v23, v24
}

func DataSwap(v0 int, v1 int) (int, int) {
    return v1, v0
}

func DataThree(v0 int) (int, bool, string) {
    v10 := (v0 > 4)
    return v0, v10, "x"
}
```

Go can return several values from a function, so a tuple result *is* Go's several results.
Nothing was built to hold them. JavaScript cannot, so there the compiler returns a small
object:

<!-- gen js docs/book/code/data/header.oro -->
```js
export function dataDivmod(v0, v1) {
    return {f0: ((Math.trunc(v0 / v1) + 0)), f1: (((v0 % v1) + 0))};
}

export function dataSwap(v0, v1) {
    return {f0: v1, f1: v0};
}

export function dataThree(v0) {
    return {f0: v0, f1: (v0 > 4), f2: "x"};
}
```

Each host gets what a programmer on that host would have written by hand. (An object with
fixed field names was measured against an array there, and was faster.)

The parts can be named and given their own sets, exactly as parameters are:

<!-- file docs/book/code/data/named.oro -->
```go
export fn divmod(a ∈ 0..1000, b ∈ 1..1000) (q ∈ 0..1000, r ∈ 0..999) {
    (a / b, a % b)
}
```

<!-- gen go docs/book/code/data/named.oro -->
```go
func DataDivmod(v0 int, v1 int) (int, int) {
    v23 := (v0 / v1)
    v24 := (v0 % v1)
    return v23, v24
}
```

Each part's set is a claim (chapter 6), and each is proven separately: the quotient of a number
up to 1000 by a number from 1 up is in 0..1000, and a remainder by a number up to 1000 is in
0..999. Claim too much about one part and that part is named:

<!-- file docs/book/code/data/no-named.oro -->
```go
export fn divmod(a ∈ 0..1000, b ∈ 1..1000) (q ∈ 0..10, r ∈ 0..999) {
    (a / b, a % b)
}
```

<!-- gen go docs/book/code/data/no-named.oro -->
```
gen: divmod: the body does not establish that component 1 of its result is in 0..10; it is [0, 1000]
```

`1000 / 1` is 1000, which is not at most 10.

---

## 10.3 A tuple is not a thing

In many languages a pair is an object: memory is set aside for it, the two values are copied
in, and later they are read back out. Here a tuple is a way of *writing* a program. Watch what
reduction (chapters 1 and 2) does to it. The target is the tutorial one, where `h`, `x` and
`y` are unknown operations that stay as written.

<!-- file docs/book/code/data/gone.oro -->
```go
fn pair(a, b) { (a, b) }

export fn first(u, v) { (a, b) = pair(u, v); a }

export fn swap(u, v) { (a, b) = (u, v); (b, a) }

export fn chosen(c, u, v) { (a, b) = if c { (u, v) } else { (v, u) }; h(a, b) }

export fn once(u) { (a, b) = (x(u), y(u)); h(a, a) }
```

<!-- oro tutorial docs/book/code/data/gone.oro -->
```go
first =
fn(u, v) { u }
swap =
fn(u, v) { (v, u) }
chosen =
fn(c, u, v) { if c { h(u, v) } else { h(v, u) } }
once =
fn(u) { a = x(u); h(a, a) }
```

- `first` built a pair and took its first part. What is left is `u`. No pair.
- `swap` took a pair apart and built another. One pair is left, the result, because that is
  what the function returns.
- `chosen` picked one of two pairs with an `if`. The pattern moved *into* both branches (chapter
  4, "a choice moves to where it is needed"), and each branch then had a known pair to take
  apart. No pair at all is left, only the `if`.
- `once` built a pair of two computations and used the first part twice. `x(u)` is computed
  once and named. `y(u)` was never used and is gone (the tutorial target says `y` is pure).

Taking a tuple apart is the same act as calling a function: put each value where its name is.
That is why nothing had to be added to the language to reduce them.

The same on a real target. `back` puts a division back together:

<!-- file docs/book/code/data/back.oro -->
```go
fn divmod(a ∈ 0..1000, b ∈ 1..1000) { (a / b, a % b) }

export fn back(a ∈ 0..1000, b ∈ 1..1000) (Int) {
    (q, r) = divmod(a, b)
    q * b + r
}
```

<!-- gen go docs/book/code/data/back.oro -->
```go
func DataBack(v0 int, v1 int) int {
    v23 := (v0 / v1)
    v24 := (v23 * v1)
    v25 := (v0 % v1)
    v26 := (v24 + v25)
    return v26
}
```

Four arithmetic operations. `divmod` is gone, its tuple is gone, the pattern is gone.

**What a tuple cannot do.** It has no operations of its own: there is no `t[0]`, no
comparison of two tuples, no "length". A pattern is the only way in. And a tuple cannot be
the set of a *parameter*; a function that wants two values takes two parameters, which is the
same thing. Tuples of tuples, and tuples stored in tables, come with chapter 13.

---

## 10.4 One of several

A traffic light is red, green or amber. Never two, never none, never a fourth thing.

How would you write that with what you have? Chapter 7's answer is an integer: 0 for red, 1
for green, 2 for amber. It works, and it has every problem a convention has. Nothing stops 3.
Nothing stops adding a light to a temperature. Nothing tells the reader that 1 means green.
And when a fourth state is added one day, nothing finds the places that assumed three.

A **variant** says it directly:

<!-- file docs/book/code/data/light.oro -->
```go
use io

set Light = 'red ∪ 'green ∪ 'amber

fn next(l) { case l { 'red -> 'green; 'green -> 'amber; 'amber -> 'red } }

fn name(l) { case l { 'red -> "red"; 'green -> "green"; 'amber -> "amber" } }

fn seconds(l) { case l { 'red -> 30; 'green -> 25; 'amber -> 5 } }

export fn main() {
    io.print_line(name('red))
    io.print_line(name(next('red)))
    io.print_line(name(next(next('red))))
    io.print_line(name(next(next(next('red)))))
    io.print_int(seconds('red) + seconds('green) + seconds('amber))
}
```

<!-- run go docs/book/code/data/light.oro -->
```
red
green
amber
red
60
```

Three things are new.

**`'red` is a tag.** The quote mark in front says: this is not a name to look up, it is a
piece of data that *is* its own name. (The mark and the idea are from Lisp, 1958.) A tag is a
value. It can be passed, returned and stored like 7 or `"x"`.

**`set Light = 'red ∪ 'green ∪ 'amber` declares the variant.** `∪` is the union sign of
chapter 6's set notation, typed `\cup`: the set `Light` has exactly these three members. The
declaration is what makes the tags exist.

**`case l { 'red -> …; 'green -> …; 'amber -> … }` asks which one.** Chapter 4's `case` chose
by testing conditions in order. This form is given a value after the word `case`, and each arm
starts with a tag instead of a condition. The arm whose tag matches is the answer.

`next` is the whole behaviour of the light as a table: after red, green; after green, amber;
after amber, red. Three steps come back to red, and the output shows it. A thing with a fixed
set of states and a rule for moving between them is a **state machine**, and a variant with a
`case` is the most direct way there is to write one.

---

## 10.5 Every case, or the program is refused

The compiler knows all three members of `Light`, so it can check that a `case` answers for
each. Forget one:

<!-- file docs/book/code/data/no-missing.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

export fn stop(n ∈ 0..2) (Bool) {
    l = case { n == 0 -> 'red; n == 1 -> 'green; else -> 'amber }
    case l { 'red -> true; 'green -> false }
}
```

<!-- gen go docs/book/code/data/no-missing.oro -->
```
gen: docs/book/code/data/no-missing.oro: stop: case on light is not exhaustive: amber is unmatched. Add it a clause, or an `else`
```

This is the property that makes variants worth having. Add `'flashing` to `Light` next year
and every `case` in the program that has not been taught about it is refused, by name. In C
the same change compiles cleanly, and each forgotten `switch` falls through to whatever its
author did not think about.

A tag that is not in the set is caught the same way:

<!-- file docs/book/code/data/no-tag.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

export fn stop(n ∈ 0..2) (Bool) {
    l = case { n == 0 -> 'red; n == 1 -> 'green; else -> 'amber }
    case l { 'red -> true; 'green -> false; 'yellow -> false }
}
```

<!-- gen go docs/book/code/data/no-tag.oro -->
```
gen: docs/book/code/data/no-tag.oro: stop: case: 'yellow is not a constructor of any variant type in scope. A variant type is declared `set S = 'c(T) ∪ …` (ADR 0064 item 7)
```

When you really do mean "everything else", say `else`:

<!-- file docs/book/code/data/partial.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

export fn stop(n ∈ 0..2) (Bool) {
    l = case { n == 0 -> 'red; n == 1 -> 'green; else -> 'amber }
    case l { 'red -> true; else -> false }
}
```

<!-- gen go docs/book/code/data/partial.oro -->
```go
func DataStop(v0 int) bool {
    v10 := (v0 == 0)
    return v10
}
```

An `else` is a decision not to be told about new cases, so use it when that is what you want.
And an `else` that can never be reached is refused too, because it is either a mistake or a
leftover:

<!-- file docs/book/code/data/no-dead.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

export fn stop(n ∈ 0..2) (Bool) {
    l = case { n == 0 -> 'red; n == 1 -> 'green; else -> 'amber }
    case l { 'red -> true; 'green -> false; 'amber -> false; else -> true }
}
```

<!-- gen go docs/book/code/data/no-dead.oro -->
```
gen: docs/book/code/data/no-dead.oro: stop: case on light: every variant is matched, so `else` is dead code — remove it
```

Look again at the Go for `partial.oro`: `v0 == 0`. There is no light in it. That is 10.8.

---

## 10.6 A case that carries a value

A shape is a circle with a radius, or a square with a side, or a point with nothing. The
alternatives do not hold the same information, so each tag says what comes with it:

<!-- file docs/book/code/data/shape.oro -->
```go
use io

set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

fn area(s) { case s { 'circle(r) -> 3 * r * r; 'square(w) -> w * w; 'point -> 0 } }

fn name(s) { case s { 'circle(r) -> "circle"; 'square(w) -> "square"; 'point -> "point" } }

export fn main() {
    io.print_int(area('circle(10)))
    io.print_int(area('square(4)))
    io.print_int(area('point))
    io.print_line(name('square(4)))
}
```

<!-- run go docs/book/code/data/shape.oro -->
```
300
16
0
square
```

`'circle(Int)` in the declaration says a circle carries an integer. `'circle(10)` makes one.
And in a `case`, **`'circle(r) ->` is a pattern**: if the value is a circle, call what it
carries `r` and evaluate this arm. `r` exists only in that arm. In the `'square` arm there is
no `r` to misuse, and in the `'point` arm there is nothing at all.

That is the difference from "a struct with a kind field and a radius field and a side field",
which is how this is done without variants. There, every shape has a radius, and reading a
square's radius gives you garbage. Here a radius can be *reached* only where the value is
known to be a circle. The check and the access are one construct, so they cannot be separated
or forgotten.

A tag carries one value or none:

<!-- file docs/book/code/data/no-payload.oro.txt -->
```go
set Shape = 'circle(Int) ∪ 'rect(Int, Int) ∪ 'point

export fn zero(n ∈ 0..9) (Int) { 0 }
```

<!-- gen go docs/book/code/data/no-payload.oro.txt -->
```
gen: docs/book/code/data/no-payload.oro.txt: line 1: "," where ")" was expected
```

and what it carries is written as a set's name: `Int`, `String`, `Bool`, another variant. A
rectangle, with a width *and* a height, wants an "and" inside an "or". That combination is
built in chapter 13, where a variant's value can be a position in a table that holds the
parts.

---

## 10.7 Something or nothing

The most useful variant has two cases, and almost every program needs it. Is this character a
digit, and if so which?

<!-- file docs/book/code/data/maybe.oro -->
```go
use io

set Maybe(T) = 'just(T) ∪ 'nothing

fn digit_of(c ∈ 0..127) { if c >= 48 && c <= 57 { 'just(c - 48) } else { 'nothing } }

fn show(m) {
    case m {
        'just(d) -> io.print_int(d)
        'nothing -> io.print_line("not a digit")
    }
}

export fn main() {
    show(digit_of(55))
    show(digit_of(65))
    show(digit_of(48))
}
```

<!-- run go docs/book/code/data/maybe.oro -->
```
7
not a digit
0
```

Character 55 is `7` (chapter 9: the digit `d` is 48 + d). Character 65 is `A`. Character 48
is `0`, and that third line is the reason this section exists. The digit zero and "not a
digit" are different answers. A function that returned a bare integer would have to pick some
number to mean "no", and whichever it picks (0, −1, 999) is a number some caller will
eventually mistake for an answer.

**`Maybe(T)`** has a parameter: `T` stands for whatever set the `'just` case carries. It is
one declaration for "an integer or nothing", "a string or nothing", and so on. (A set with a
parameter is a function from sets to sets; chapter 2's "a definition is an abbreviation"
covers it.)

Most languages answer this question with a special value that every reference may secretly
be: `null`, `nil`, `None`. Its inventor, Tony Hoare, put it into ALGOL W in 1965 "because it
was so easy to implement", and in 2009 called it his billion-dollar mistake. The trouble is
not that absence exists. It is that with `null`, *every* value might be absent and nothing in
the program says which ones, so every use is a possible crash. With `Maybe`, only the values
declared so can be absent, and the only way to reach the inside is a `case` that has a
`'nothing` arm. Forgetting to check is not a bug you find in production. It does not compile.

You do not have to declare this one. The language has it ready-made as **`'some(v)`** and
**`'none`**:

<!-- file docs/book/code/data/option.oro -->
```go
export fn or_zero(n ∈ 0..9) (Int) {
    found = if n > 4 { 'some(n) } else { 'none }
    case found { 'some(v) -> v; 'none -> 0 }
}
```

<!-- gen go docs/book/code/data/option.oro -->
```go
func DataOrZero(v0 int) int {
    v10 := (v0 > 4)
    if v10 {
        return v0
    }
    return 0
}
```

Looking something up in a map (chapter 14) answers with it, and chapter 15's failures are the
same shape with a reason attached to the "nothing".

---

## 10.8 Where did the variant go?

Three times now the emitted code has had no trace of a variant in it. Here it is on purpose.

<!-- file docs/book/code/data/known.oro -->
```go
set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

fn area(s) { case s { 'circle(r) -> 3 * r * r; 'square(w) -> w * w; 'point -> 0 } }

fn make(n) { case { n == 0 -> 'point; n < 50 -> 'circle(n); else -> 'square(n) } }

export fn fixed() (Int) { area('square(4)) }

export fn through(n ∈ 0..100) (Int) { area(make(n)) }
```

<!-- gen go docs/book/code/data/known.oro -->
```go
func DataFixed() int {
    return 16
}

func DataThrough(v0 int) int {
    v10 := (v0 == 0)
    if v10 {
        return 0
    }
    v15 := (v0 < 50)
    if v15 {
        v18 := (3 * v0)
        v19 := (v18 * v0)
        return v19
    }
    v21 := (v0 * v0)
    return v21
}
```

`fixed` is chapter 1's rule: a `case` on a value that is known picks its arm while compiling.
`area('square(4))` is `4 * 4` is 16.

`through` is the interesting one. `make` builds a shape depending on `n`, and `area` takes it
straight apart. Nobody knows while compiling which shape it will be. And still no shape is
ever built: the Go tests `n` and goes directly to the right formula.

The reason is one law, the same one that moved a tuple pattern into the branches of an `if`
in 10.3:

> **A `case` applied to a choice is a choice between `case`s.**

`area(if c { 'point } else { 'circle(n) })` is `if c { area('point) } else {
area('circle(n)) }`, and inside each branch the tag is now known, so each `case` picks its arm
and disappears. Whatever tested the condition has already done the work of telling the cases
apart. Testing a tag afterwards would be asking the same question twice.

So a function can return "a circle, a square or a point" as its way of *explaining* what it
found, and a caller can `case` on it, and the cost is exactly the `if` that was needed
anyway. It nests:

<!-- file docs/book/code/data/nested.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

set Maybe(T) = 'just(T) ∪ 'nothing

fn signal(n) { case { n == 0 -> 'nothing; n == 1 -> 'just('red); else -> 'just('green) } }

export fn code(n ∈ 0..2) (Int) {
    case signal(n) {
        'nothing -> 0
        'just(l) -> case l { 'red -> 1; 'green -> 2; 'amber -> 3 }
    }
}
```

<!-- gen go docs/book/code/data/nested.oro -->
```go
func DataCode(v0 int) int {
    v10 := (v0 == 0)
    if v10 {
        return 0
    }
    v15 := (v0 == 1)
    if v15 {
        return 1
    }
    return 2
}
```

A light inside a maybe, two `case`s deep, and the residue is two comparisons of `n`. The
`'amber` arm had to be written (10.5) and costs nothing: `signal` never makes an amber, so no
path reaches it.

`digit_of` from 10.7 compiles the same way:

<!-- file docs/book/code/data/value.oro -->
```go
set Maybe(T) = 'just(T) ∪ 'nothing

fn digit_of(c ∈ 0..127) { if c >= 48 && c <= 57 { 'just(c - 48) } else { 'nothing } }

export fn value(c ∈ 0..127) (0..9) {
    case digit_of(c) { 'just(d) -> d; 'nothing -> 0 }
}
```

<!-- gen go docs/book/code/data/value.oro -->
```go
func DataValue(v0 int) int {
    v12 := (v0 >= 48)
    v13 := (v12 && (v0 <= 57))
    if v13 {
        v22 := (v0 - 48)
        return v22
    }
    return 0
}
```

And the claim `(0..9)` was proven through the variant: in the `'just` arm, `d` is `c - 48`
where `c` is between 48 and 57.

---

## 10.9 At the program's edge

Inside a program a variant dissolves. It has to become something real in one place: when an
exported function *returns* one, because the caller is not our program and cannot be reduced.

<!-- file docs/book/code/data/boundary.oro -->
```go
set Light = 'red ∪ 'green ∪ 'amber

set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

set Msg = 'text(String) ∪ 'count(Int) ∪ 'quiet

export fn pick(n ∈ 0..2) (Light) { case { n == 0 -> 'red; n == 1 -> 'green; else -> 'amber } }

export fn make(n ∈ 0..100) (Shape) { case { n == 0 -> 'point; n < 50 -> 'circle(n); else -> 'square(n) } }

export fn label(n ∈ 0..10) (Msg) { case { n > 5 -> 'text("big"); n > 2 -> 'count(n); else -> 'quiet } }
```

<!-- gen go docs/book/code/data/boundary.oro -->
```go
func DataLabel(v0 int) (int, string, int) {
    v10 := (v0 > 5)
    if v10 {
        return 0, "big", 0
    }
    v17 := (v0 > 2)
    if v17 {
        return 1, "", v0
    }
    return 2, "", 0
}

func DataMake(v0 int) (int, int) {
    v10 := (v0 == 0)
    if v10 {
        return 2, 0
    }
    v16 := (v0 < 50)
    if v16 {
        return 0, v0
    }
    return 1, v0
}
```

**A variant that crosses the edge is a tag number and a slot for each kind of thing it can
carry.** The tags are numbered in the order they were declared: `'circle` is 0, `'square` is
1, `'point` is 2.

- `make` returns two integers: which shape, and its one measurement. A circle and a square
  both carry an `Int`, so they share the slot. A point carries nothing and puts 0 there.
- `label` returns three things: the tag, a string slot and an integer slot. `'text("big")`
  fills the string and leaves 0 in the integer; `'count(n)` does the opposite; `'quiet`
  leaves both empty.

So the question "which one is it?" has to be carried: one small integer. The question "what
is inside?" needs room for the largest possibility. That is all a variant ever costs, and
only at the edge.

If you have written Go, `(int, string, int)` with one meaningful slot at a time will look
familiar. Go's own convention for a function that can fail is to return `(value, error)` and
have the caller look at the error first. That is a variant spelled as a tuple: two slots,
where the rule "exactly one of them means something" lives in the programmer's head. Here the
rule is in the declaration, and the tuple is what the compiler emits to honour it.

On JavaScript the same three functions return `{f0: …, f1: …}`, as 10.2's tuples did.

---

## 10.10 Why they are called products and sums

Count values.

`Bool` has 2. `Light` has 3. How many different pairs `(Bool, Light)` are there? Each of 2
booleans with each of 3 lights: 2 × 3 = 6. A tuple's count is the **product** of its parts'
counts, and that is why a tuple set is written with a multiplication sign, `Bool × Light`
(typed `\times`), and called a product.

How many values does `Maybe(Light)` have? The three lights, each wrapped in `'just`, and
`'nothing`: 3 + 1 = 4. A variant's count is the **sum** of its cases' counts. Variants are
called sums.

| the set | how many values | why |
|---|---|---|
| `()` | 1 | the empty tuple: one way to say nothing |
| `Bool` | 2 | |
| `Light` | 3 | 1 + 1 + 1 |
| `Bool × Light` | 6 | 2 × 3 |
| `Maybe(Light)` | 4 | 3 + 1 |
| `Maybe(Bool × Light)` | 7 | 2 × 3 + 1 |

This is arithmetic, and it obeys arithmetic's laws. `()` is the 1 of the product: a tuple
with an extra `()` in it holds no more information. A pair of an `A` and a "`B` or `C`"
carries the same information as "a pair `(A, B)` or a pair `(A, C)`", which is
a × (b + c) = a × b + a × c. Data built from products and sums is called **algebraic** for
this reason, and the idea dates from the language Hope in 1980.

The counting is a design tool. If a value in your program can be in 5 real states and its
representation has 12, there are 7 states that mean nothing, and somewhere a bug will produce
one. A struct with a `kind` field and a slot for every case's data has the *product* of the
counts where the truth is their *sum*. Choosing a variant is choosing the representation with
no impossible states.

`Bool` itself is a variant in everything but spelling: two tags, `true` and `false`, and `if`
is its `case`. Chapter 4 was this chapter with two cases and no payloads.

---

## 10.11 What is not here

**A variant cannot contain itself.**

<!-- file docs/book/code/data/no-recursive.oro.txt -->
```go
set List = 'empty ∪ 'more(List)

export fn zero(n ∈ 0..9) (Int) { 0 }
```

<!-- gen go docs/book/code/data/no-recursive.oro.txt -->
```
gen: docs/book/code/data/no-recursive.oro.txt: variant list: constructor more carries a list, so the type contains itself. That is a recursive type, which is refused; recursive data is a flat table plus indices (type-algebra.md §3.1)
```

A list, a tree, a document: anything of unbounded size is a *table* here, and a "pointer" to
another node is a position in that table. Chapter 13 builds one. It is the layout a careful C
programmer chooses for speed anyway (everything in one block of memory, no chasing pointers),
and it keeps the promise that a variant is a tag and a few slots.

**A variant has at least two cases.** One case is just what it carries:

<!-- file docs/book/code/data/no-single.oro.txt -->
```go
set Wrapped = 'only(Int)

export fn zero(n ∈ 0..9) (Int) { 0 }
```

<!-- gen go docs/book/code/data/no-single.oro.txt -->
```
gen: docs/book/code/data/no-single.oro.txt: variant wrapped: a variant type has two or more constructors; one is just the payload
```

**No names for a tuple's parts.** A tuple with named parts is a *record* (a `struct`), and it is
the next chapter.

**A variant's name goes on a result, and not yet on a parameter.** `(Light)` as a result's set
was used above. `fn name(l ∈ Light)` is refused today (README item 54), which is why this
chapter's functions take `l` and `s` with no set written. The `case` inside them says what
they accept.

### Across modules

A variant declared in another module is marked `pub` like anything else, and its tags are
written with the module's name in front:

<!-- file docs/book/code/data/shapes.oro -->
```go
pub set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

pub fn area(s) { case s { 'circle(r) -> 3 * r * r; 'square(w) -> w * w; 'point -> 0 } }
```

<!-- file docs/book/code/data/uses-shapes.oro -->
```go
use io
use shapes

fn corners(s) { case s { shapes.'circle(r) -> 0; shapes.'square(w) -> 4; shapes.'point -> 1 } }

export fn main() {
    io.print_int(shapes.area(shapes.'square(5)))
    io.print_int(corners(shapes.'square(5)))
    io.print_int(corners(shapes.'point))
}
```

<!-- run go docs/book/code/data/uses-shapes.oro -->
```
25
4
1
```

`shapes.'square` is the `'square` that belongs to `shapes`. Two modules may each have an `'ok`
or a `'none` of their own, and the qualified tag says which is meant.

---

## 10.12 What other languages do

| | "and" | "or" | must a choice cover every case? | "nothing" |
|---|---|---|---|---|
| Oroboros | tuple | variant | yes, or `else` | `'none`, a case you must handle |
| C | `struct` | `enum` plus `union`, tag kept by hand | no | `NULL`, any pointer |
| Go | `struct`, multiple results | constants, or an interface | no | `nil`, any pointer or interface |
| Java | class, `record` | `enum`; `sealed` types since 17 | in a `switch` over a sealed type | `null`, any reference |
| TypeScript | object | union of object types with a `kind` field | if you ask for it | `null` and `undefined`, declared |
| Rust | tuple, `struct` | `enum` | yes | `Option`, a case you must handle |
| Haskell, ML | tuple, record | `data` | a warning | `Maybe` |

Rust is the nearest again: its `enum` and `match` are this chapter, and `Option<T>` is
`'some`/`'none`. The difference is in what remains afterwards. Rust lays an `enum` out in
memory, a tag and a union of the payloads. Here it has no layout inside the program at all
and is reduced away, at the price of the restrictions in 10.11: no recursion, one payload,
and nothing stored except in tables.

C has every ingredient and none of the guarantees: an `enum` for the tag, a `union` for the
payloads, and a comment asking you to check the tag before reading the union.

---

## What to remember

- **A tuple is several values at once**: `(a, b)`, any number of parts, any kinds. `()` is
  the tuple of none.
- **A pattern takes it apart**: `(q, r) = e`. There is no other way in.
- **A tuple result is written part by part**, `(Int, Int)` or `(q ∈ 0..1000, r ∈ 0..999)`,
  and each part's set is its own claim. It is the host's several results.
- **A variant is exactly one of several tagged cases**: `set Light = 'red ∪ 'green ∪ 'amber`.
  A tag starts with a quote. `∪` is typed `\cup`.
- **A tag may carry one value**: `'circle(Int)`. `'circle(10)` makes one.
- **`case v { 'circle(r) -> …; … }` asks which case it is** and names what it carries, in that
  arm only.
- **A `case` must cover every tag**, or say `else`. An unreachable `else` is refused too.
- **`Maybe(T)`-shaped variants replace `null`.** The ready-made one is `'some(v)` and `'none`.
- **A `case` applied to a choice is a choice between `case`s**, so a variant made and taken
  apart inside the program leaves only the test that chose it.
- **At an export's result a variant is a tag number and one slot per kind of payload.**
- **Tuples multiply and variants add** the number of values. Pick the representation with no
  impossible states.
- **No recursive variants and no one-case variants.** Unbounded data is a table.

Next: [chapter 11](11-records.md), Records. A tuple whose parts have names.
