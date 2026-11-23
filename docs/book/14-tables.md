# 14. Tables

A tuple holds a few values, and you know how many when you write the program. Real data comes
in quantities nobody knows in advance: the bytes of a file, the lines of a report, a million
measurements. Chapter 13 gave the way to *do* something many times. This chapter gives the
thing to do it *to*.

Every language has this, under the name array, list, slice or vector. And in most of them it
is the single largest source of serious bugs there has ever been: reading or writing a
position that is not there. So the question of this chapter is: **how does a program hold
many values of one kind, and how does it know that a position exists before it looks?**

The answer is three sentences.

> **A table is a function on the positions 0, 1, …, len − 1.** `t[i]` is its value at `i`,
> and `len(t)` is how many positions it has.
>
> **A table is written two ways: by listing its values, or by giving a rule for position
> `i`.** A rule takes no memory until something needs it stored.
>
> **`t[i]` compiles only where `i` is proven to be a position of `t`.** Nothing is checked
> when the program runs.

Along the way: why counting starts at zero, the bug that leaked the internet's passwords in
2014, why a chain of table operations can cost one loop, a search that finds one name among a
billion in thirty steps, and how to build lists and grids when there are no pointers.

The examples are in [code/tables/](code/tables/). `go run docs/book/check.go
docs/book/14-tables.md` re-runs every block. Checked 2026-11-16.

---

## 14.1 Many values of one kind

<!-- file docs/book/code/tables/first.oro -->
```go
use io

export fn main() {
    primes = array { 2, 3, 5, 7, 11 }
    io.print_int(primes[0])
    io.print_int(primes[4])
    io.print_int(len(primes))

    days = array { "mon", "tue", "wed", "thu", "fri", "sat", "sun" }
    io.print_line(days[1])
    io.print_line(days[len(days) - 1])

    io.print_int(len(array { }))
}
```

<!-- run go docs/book/code/tables/first.oro -->
```
2
11
5
tue
sun
0
```

**`array { 2, 3, 5, 7, 11 }` is a table**: five values in a row. (Chapter 10 used one to write
bytes down, with a promise to explain.) **`primes[0]`** is the value at position 0, and
**`len(primes)`** is the number of positions, 5.

The positions are numbered **from 0**. The first value is `primes[0]` and the last is
`primes[4]`, which is `primes[len(primes) - 1]`. With five values, 5 itself is not a
position.

Counting from zero trips everyone once. The reason for it is that a position is a
*distance from the start*: the first value is zero steps in. It makes the arithmetic of
this chapter come out clean: the positions of a table of length n are exactly the numbers
`i` with `0 <= i < n`, there are `n` of them, and a table split at position `k` has `k`
values before the split. With counting from one, each of those sentences needs a "+ 1"
somewhere, and a forgotten "+ 1" is a bug.

All the values of a table are of one kind: integers, or strings, or booleans. That is the
difference from a tuple, which has a fixed number of parts of any kinds. An empty table,
`array { }`, has length 0 and no positions at all.

A table is a value like any other. It never changes (chapter 15 is about the things that
do), so it can be passed around and shared freely.

### A table is a function

`days[1]` looks like a function call with square brackets, and that is what it is. A table
*is* a function: give it a position, it gives you the value there. It differs from an
ordinary function in one respect. An ordinary function is defined for every argument of its
set. A table is defined only on the positions 0 to len − 1.

The classic use is a function too awkward to compute, written out as its list of answers:

<!-- file docs/book/code/tables/month.oro -->
```go
export fn days_in(month ∈ 1..12) (28..31) {
    array { 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31 }[month - 1]
}
```

<!-- gen go docs/book/code/tables/month.oro -->
```go
func TablesDaysIn(v0 int) int {
    v21 := []byte{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
    v23 := (v0 - 1)
    v24 := int(v21[v23])
    return v24
}
```

There is no formula for the days in a month, so the function is a lookup. Three things were
proven here, all while compiling:

- `month - 1` is in 0..11, and the table has 12 positions, so the lookup is inside.
- Every value in the table is between 28 and 31, so the result's claim `(28..31)` holds.
- Every value fits in a byte, so Go was given a `[]byte`: twelve bytes, not twelve words.

---

## 14.2 A position must be proven

Ask for a position that is not there:

<!-- file docs/book/code/tables/no-outside.oro -->
```go
use io

export fn main() {
    primes = array { 2, 3, 5, 7, 11 }
    io.print_int(primes[5])
}
```

<!-- build go docs/book/code/tables/no-outside.oro -->
```
build: main: `array`(2, 3, 5, 7, 11)[5] is an indexing, and 5 < len(`array`(2, 3, 5, 7, 11)) does not follow
  known: nothing
  A table is a function with a finite domain, so 0 <= i < len is the
  condition for the application to be DEFINED — not a check bolted on
  (docs/spec/tables.md §6).
```

The message says it the way this chapter does: a table is a function with a finite domain,
and `primes[5]` is that function applied outside its domain. It is the same kind of mistake
as dividing by zero (chapter 7), and it is treated the same way: the program is refused.

What do other languages do with `primes[5]`?

| | reading past the end |
|---|---|
| Go | stops the program with a panic |
| Java, Python | throw an exception |
| JavaScript | gives `undefined`, and carries on |
| C, C++ | reads whatever is in memory after the table, and carries on |

The last row is the famous one. In 2014 a bug called Heartbleed was found in the software
that secures most of the web. A client could say "here are 5 bytes, send them back, and by
the way their length is 65535". The server copied 65535 bytes starting at those 5, and so
handed over whatever was next to them in its memory: passwords, private keys. One missing
comparison of an index against a length. The first internet worm, in 1988, spread through
the same mistake in the other direction, *writing* past the end of a table.

Go and Java are safe because they check every access as the program runs. That works, and
it costs a comparison on every read. Here the comparison is done once, by the compiler, and
the program that runs contains none.

So how do you prove a position?

<!-- file docs/book/code/tables/no-at.oro -->
```go
export fn at(t ∈ Array(Int), i ∈ Int) (Int) { t[i] }
```

<!-- gen go docs/book/code/tables/no-at.oro -->
```
gen: tables-at: t[i] is an indexing, and 0 <= i does not follow
```

`Array(Int)` is the set of tables of integers, of any length. With `t` any table and `i` any
integer, nothing is known, and the function is refused. There are three ways to supply what
is missing, and they are chapter 7's three ways.

<!-- file docs/book/code/tables/at.oro -->
```go
export fn promised(t ∈ Array(Int), i ∈ Int) (Int) where 0 <= i && i < len(t) { t[i] }

export fn tested(t ∈ Array(Int), i ∈ Int) (Int) {
    if 0 <= i && i < len(t) { t[i] } else { 0 }
}

export fn maybe(t ∈ Array(Int), i ∈ Int) (Int) {
    found = if 0 <= i && i < len(t) { 'some(t[i]) } else { 'none }
    case found { 'some(v) -> v; 'none -> -1 }
}

export fn first(t ∈ Array(Int)) (Int) where len(t) > 0 { t[0] }

export fn last(t ∈ Array(Int)) (Int) where len(t) > 0 { t[len(t) - 1] }
```

<!-- gen go docs/book/code/tables/at.oro -->
```go
func TablesFirst(v0 []int) int {
    v5 := v0[0]
    return v5
}

func TablesLast(v0 []int) int {
    v4 := len(v0)
    v6 := (v4 - 1)
    v7 := v0[v6]
    return v7
}

func TablesMaybe(v0 []int, v1 *big.Int) int {
    v4 := (big.NewInt(int64(0)))
    v5 := (v4.Cmp(v1) <= 0)
    v6 := (v5 && (v1.Cmp((big.NewInt(int64(len(v0))))) < 0))
    if v6 {
        v11 := (int(v1.Int64()))
        v12 := v0[v11]
        return v12
    }
    return -1
}

func TablesPromised(v0 []int, v1 int) int {
    v10 := v0[v1]
    return v10
}
```

- **`promised` puts the condition in its header**, with `where`. The caller must prove it,
  and the body may then assume it. The Go is one line, and `i` is an `int`: a length is at most
  the largest table the target holds, so `i < len(t)` keeps `i` inside the word.
- **`tested` tests.** Inside the `if`, the condition is a fact. The test is in the Go,
  because you wrote it and it does something: it decides what to return.
- **`maybe` answers "something or nothing"** (chapter 11), which is the honest result of a
  lookup that can fail. The variant is gone from the Go, as chapter 11 said it would be. Its
  `i ∈ Int` is any integer, since nothing in the header bounds it, so Go hands over a
  `*big.Int`: the test compares one, and inside it `i` is proven a valid position and comes back
  into an `int` to index. `tested` is the same. An index is a word; `i ∈ -2^63..2^63-1` would
  have said so.
- **`first` and `last`** show that even position 0 is not free: an empty table has no first
  value.

<!-- file docs/book/code/tables/no-first.oro -->
```go
export fn first(t ∈ Array(Int)) (Int) { t[0] }
```

<!-- gen go docs/book/code/tables/no-first.oro -->
```
gen: tables-first: t[0] is an indexing, and 0 < len(t) does not follow
```

Half of the "impossible" crashes in everyday software are this one: taking the first element
of a list that turned out to be empty.

---

## 14.3 Walking a table

Chapter 13's loop, with a table in it:

<!-- file docs/book/code/tables/total.oro -->
```go
export fn total(t ∈ Array(0..1000)) (Int) where len(t) <= 10^6 {
    loop i = 0, s = 0 {
        i < len(t) -> again(i + 1, s + t[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/total.oro -->
```go
func TablesTotal(v0 []uint16) int {
    var v7 int = 0
    var v8 int = 0
    for ; ; v7 = (v7 + 1) {
        v9 := len(v0)
        v10 := (v7 < v9)
        if v10 {
            v15 := int(v0[v7])
            v16 := (v8 + v15)
            v8 = v16
            continue
        }
        break
    }
    return v8
}
```

`i` starts at 0 and steps by 1 while `i < len(t)`. Inside the arm, then, `0 <= i` (it started
at 0 and only went up) and `i < len(t)` (the guard just said so). That is exactly the
condition for `t[i]`. **The loop's own test is the proof**, and no other test is needed or
emitted. This is why the idiom "for i from 0 while i < len" is worth learning by heart.

Get the test wrong by one character:

<!-- file docs/book/code/tables/no-off-by-one.oro -->
```go
export fn total(t ∈ Array(0..1000)) (Int) where len(t) <= 10^6 {
    loop i = 0, s = 0 {
        i <= len(t) -> again(i + 1, s + t[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/no-off-by-one.oro -->
```
gen: tables-total: t[i] is an indexing, and i < len(t) does not follow
```

`<=` lets `i` reach `len(t)`, one past the end. This is the **off-by-one error**, the most
common bug there is in loops over tables, and Heartbleed's whole family. Here it is a
message at compile time that names the access.

Two more things in the header are doing work.

**`Array(0..1000)`** says every value in the table is between 0 and 1000. A table's set
includes the set of its elements, and `t[i]` is known to be in it. It also chose the storage:
Go got `[]uint16`, two bytes per value. (You did not ask for 16 bits. You said what the
values are, and the compiler picked the smallest thing that holds them, as in chapter 7.)

**`where len(t) <= 10^6`** is chapter 13's lesson about sums. Leave it out, and see what it
bought:

<!-- file docs/book/code/tables/no-long.oro -->
```go
export fn total(t ∈ Array(0..1000)) (Int) {
    loop i = 0, s = 0 {
        i < len(t) -> again(i + 1, s + t[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/no-long.oro -->
```go
note: tables-total: 2 operation(s) in arbitrary precision
…
    var v5h, v5l uint64 = uint64(0), uint64(0)
…
        if v7 {
            v12 := int(v0[v4])
            v13l, v13c := bits.Add64(v5l, uint64(v12), 0)
            v13h := v5h + uint64(v12 >> 63) + v13c
            v5h, v5l = v13h, v13l
            continue
        }
…
    v28 := (new(big.Int).Mul(v26, v27))
    v29 := (new(big.Int).Add(v28, v23))
    return v29
}
```

A table could in principle have 2⁶³ − 1 positions, and that many values of up to 1000 do
not fit in a word: the sum can reach 1000 · (2⁶³ − 1), about 2⁷³. It does fit in two words, the
double word of chapter 7, so the loop carries `s` as two of them, `v5h` and `v5l`, and each
step is an addition with its carry. The result is claimed `Int`, every integer, so at the end
the sum becomes Go's `*big.Int` once, from its digits: the *2 operations in arbitrary precision*
the note counts are that last multiplication and addition. `where len(t) <= 10^6` makes the
sum one word and each step one machine addition. Nobody has a table of 2⁶³ − 1 positions, so
say how long yours can be.

The same shape of loop finds the largest value, a position, a count:

<!-- file docs/book/code/tables/walks.oro -->
```go
export fn biggest(t ∈ Array(0..1000)) (0..1000) {
    loop i = 0, m = 0 {
        i >= len(t) -> m
        t[i] > m -> again(i + 1, t[i])
        else -> again(i + 1, m)
    }
}

export fn position(t ∈ Array(Int), x ∈ Int) (Int) {
    loop i = 0 {
        i >= len(t) -> -1
        t[i] == x -> i
        else -> again(i + 1)
    }
}

export fn how_many(t ∈ Array(Bool)) (Int) where len(t) <= 10^6 {
    loop i = 0, n = 0 {
        i >= len(t) -> n
        t[i] -> again(i + 1, n + 1)
        else -> again(i + 1, n)
    }
}
```

<!-- gen js docs/book/code/tables/walks.oro -->
```js
export function tablesBiggest(v0) {
    let v4 = 0;
    let v5 = 0;
    for (;; v4 = (v4 + 1)) {
        if ((v4 >= v0.length)) {
            return v5;
        }
        if ((v0[v4] > v5)) {
            v5 = v0[v4];
            continue;
        }
        continue;
    }
}
```

In these the exit is the first arm, `i >= len(t)`. Chapter 13 said a guard's opposite is a
fact in the arms below it: below that arm, `i < len(t)`, so `t[i]` is proven in both. The
claim `(0..1000)` on `biggest` is proven too: `m` starts at 0 and is only ever replaced by an
element, and elements are in 0..1000.

---

## 14.4 A table by a rule

Listing values is fine for twelve months. For the first thousand squares you want to say
*how to get* the value at each position:

<!-- file docs/book/code/tables/rule.oro -->
```go
export fn squares(n ∈ 0..1000) (Array(Int)) { array { i * i : i ∈ 0..n-1 } }
```

Read `array { i * i : i ∈ 0..n-1 }` as mathematics writes a set: *the table whose value at
`i` is `i * i`, for `i` from 0 to n − 1*. The part after the colon names the position and
says how many there are. The part before it is any expression in that name.

<!-- gen go docs/book/code/tables/rule.oro -->
```go
func TablesSquares(v0 int) []int {
    var n9 int = v0
    v9 := make([]int, n9)
    for v10 := 0; v10 < n9; v10++ {
        var e9 int
        v11 := (v10 * v10)
        e9 = v11
        v9[v10] = e9
    }
    return v9
}
```

Storage for `n` integers, and a loop that fills it. You wrote neither. A rule says *what*
each position holds, and the order in which the positions are filled is nobody's business:
each depends only on its own `i`. (That independence is what lets a rule be computed in
parallel, or a few positions at a time in one wide instruction.)

A rule can read other tables:

<!-- file docs/book/code/tables/rules.oro -->
```go
export fn doubled(t ∈ Array(0..1000)) (Array(Int)) { array { 2 * t[i] : i ∈ 0..len(t)-1 } }

export fn reversed(t ∈ Array(Int)) (Array(Int)) { array { t[len(t) - 1 - i] : i ∈ 0..len(t)-1 } }

export fn gaps(t ∈ Array(0..1000)) (Array(Int)) where len(t) >= 2 {
    array { t[i + 1] - t[i] : i ∈ 0..len(t)-2 }
}

export fn evens(n ∈ 0..1000) (Array(Bool)) { array { i % 2 == 0 : i ∈ 0..n-1 } }
```

<!-- gen js docs/book/code/tables/rules.oro -->
```js
export function tablesGaps(v0) {
    const n1 = ((v0.length - 2) + 1);
    const v9 = new Array(n1).fill(0);
    for (let v10 = 0; v10 < n1; v10++) {
        let e2;
        e2 = (v0[(v10 + 1)] - v0[v10]);
        v9[v10] = e2;
    }
    return v9;
}

export function tablesReversed(v0) {
    const n1 = v0.length;
    const v2 = new Array(n1).fill(0);
    for (let v3 = 0; v3 < n1; v3++) {
        let e2;
        e2 = v0[((v0.length - 1) - v3)];
        v2[v3] = e2;
    }
    return v2;
}
```

Inside a rule, the position's name carries its range as a fact: `i` is in `0..len(t)-1`. So
`t[i]` in `doubled` is proven with no test. In `reversed`, `len(t) - 1 - i` runs from
len − 1 down to 0, all inside. `gaps` reads `t[i + 1]`, so its positions stop one short, at
`len(t)-2`. Let them run to the end and the last one reaches past it:

<!-- file docs/book/code/tables/no-gaps.oro -->
```go
export fn gaps(t ∈ Array(0..1000)) (Array(Int)) {
    array { t[i + 1] - t[i] : i ∈ 0..len(t)-1 }
}
```

<!-- gen go docs/book/code/tables/no-gaps.oro -->
```
gen: tables-gaps: t[i + 1] is an indexing, and i + 1 < len(t) does not follow
```

The positions of a table start at 0, so a rule's range does too:

<!-- file docs/book/code/tables/no-from-one.oro.txt -->
```go
export fn count_up(n ∈ 0..100) (Array(Int)) { array { k : k ∈ 1..n } }
```

<!-- gen go docs/book/code/tables/no-from-one.oro.txt -->
```
gen: docs/book/code/tables/no-from-one.oro.txt: line 1: a table's index ranges over 0..n-1, so its binder's set starts at 0
```

(The numbers 1 to n are `array { k + 1 : k ∈ 0..n-1 }`.)

---

## 14.5 A rule takes no memory

Here is the sentence from the top of the chapter that sounds too good. Add up the first `n`
squares, by making the table of squares and then walking it:

<!-- file docs/book/code/tables/fused.oro -->
```go
export fn sum_of_squares(n ∈ 0..1000) (Int) {
    sq = array { i * i : i ∈ 0..n-1 }
    loop i = 0, s = 0 {
        i < len(sq) -> again(i + 1, s + sq[i])
        else -> s
    }
}

export fn two_of_them(n ∈ 0..99) (Int) {
    sq = array { i * i : i ∈ 0..99 }
    sq[n] + sq[3]
}
```

<!-- gen go docs/book/code/tables/fused.oro -->
```go
func TablesSumOfSquares(v0 int) int {
    var v12 int = 0
    var v13 int = 0
    for ; ; v12 = (v12 + 1) {
        v14 := (v12 < v0)
        if v14 {
            v19 := (v12 * v12)
            v20 := (v13 + v19)
            v13 = v20
            continue
        }
        break
    }
    return v13
}

func TablesTwoOfThem(v0 int) int {
    v9 := (v0 * v0)
    v11 := (v9 + 9)
    return v11
}
```

There is no table in either function. No `make`, no storage, nothing filled in.

It follows from 13.1: a table is a function. A table given by a rule is a function given by
a formula, and **looking up position `k` in it is calling that function**: put `k` where the
rule has `i`. So `sq[i]` is `i * i`, by chapter 2's substitution. `len(sq)` is `n`, because
the rule says so. `sq[3]` is `3 * 3`, which is 9, computed while compiling. The name `sq` was
an abbreviation (chapter 3), and nothing is left of it.

It composes. Double every value, add one to each, and sum:

<!-- file docs/book/code/tables/chain.oro -->
```go
export fn run(t ∈ Array(0..1000)) (Int) where len(t) <= 10^6 {
    doubled = array { 2 * t[i] : i ∈ 0..len(t)-1 }
    plus_one = array { doubled[i] + 1 : i ∈ 0..len(doubled)-1 }
    loop i = 0, s = 0 {
        i < len(plus_one) -> again(i + 1, s + plus_one[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/chain.oro -->
```go
func TablesRun(v0 []uint16) int {
    var v7 int = 0
    var v8 int = 0
    for ; ; v7 = (v7 + 1) {
        v9 := len(v0)
        v10 := (v7 < v9)
        if v10 {
            v16 := int(v0[v7])
            v17 := (2 * v16)
            v19 := (v17 + 1)
            v20 := (v8 + v19)
            v8 = v20
            continue
        }
        break
    }
    return v8
}
```

Three passes in the source: make `doubled`, make `plus_one`, add up. One loop in the Go, which
reads each input value once and computes `2 * x + 1` on the way to the sum. In a language
where each step builds a real list, this program allocates two lists of a million numbers and
throws them away. That saving has a name, **fusion** (or deforestation), and compilers for
other languages work hard to find it. Here it is not an optimization that may or may not
happen. It is what the words mean.

So when *is* a rule stored? When it has to exist as a block of memory: above all, when a
table is the result of an exported function, as in 13.4. The caller is outside the program
and cannot be handed a formula. Inside a program a rule is a formula until something
forces the issue.

A rule is recomputed at each lookup. That is the price: `sq[i]` costs a multiplication each
time, where a stored table costs a memory read. For a cheap rule that is a good trade. For a
rule that is expensive and looked up many times, you want it stored, and chapter 15 has the
way to build a table in memory.

---

## 14.6 Functions over tables

Tables are values and functions take values, so the walks of this chapter can be written
once and named:

<!-- file docs/book/code/tables/library.oro -->
```go
use io

fn total(t) { loop i = 0, s = 0 { i < len(t) -> again(i + 1, s + t[i]); else -> s } }

fn each(f, t) { array { f(t[i]) : i ∈ 0..len(t)-1 } }

fn reversed(t) { array { t[len(t) - 1 - i] : i ∈ 0..len(t)-1 } }

export fn main() {
    t = array { 3, 1, 4, 1, 5, 9, 2, 6 }
    io.print_int(total(t))
    io.print_int(total(each(fn(x) { x * x }, t)))
    io.print_int(total(each(fn(x) { if x % 2 == 0 { x } else { 0 } }, t)))
    r = reversed(t)
    io.print_int(r[0])
    io.print_int(r[7])
}
```

<!-- run go docs/book/code/tables/library.oro -->
```
31
173
12
6
3
```

`each(f, t)` is the table you get by applying `f` at every position of `t`: other languages
call it `map`. It takes a *function* as an argument (chapter 2), and it is three lines of
ordinary code, not something built into the language. The sum of the squares is 173. The
third line sums only the even values, by turning the odd ones into 0.

All of it fuses. `total(each(fn(x) { x * x }, t))` runs as one loop with a multiplication in
it, and no function is called at run time: `each`, the little function passed to it, and
`total` were all unfolded while compiling.

<!-- run js docs/book/code/tables/library.oro -->
```
31
173
12
6
3
```

<!-- run windows docs/book/code/tables/library.oro -->
```
31
173
12
6
3
```

---

## 14.7 Two tables at once

The dot product: multiply two tables position by position and add up.

<!-- file docs/book/code/tables/dot.oro -->
```go
export fn dot(a ∈ Array(0..1000), b ∈ Array(0..1000)) (Int) where len(a) == len(b) && len(a) <= 10^6 {
    loop i = 0, s = 0 {
        i < len(a) -> again(i + 1, s + a[i] * b[i])
        else -> s
    }
}
```

The loop's guard is about `a`. It proves `a[i]`. What proves `b[i]`? Only the header's
`len(a) == len(b)`. Take that out:

<!-- file docs/book/code/tables/no-dot.oro -->
```go
export fn dot(a ∈ Array(0..1000), b ∈ Array(0..1000)) (Int) where len(a) <= 10^6 {
    loop i = 0, s = 0 {
        i < len(a) -> again(i + 1, s + a[i] * b[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/no-dot.oro -->
```
gen: tables-dot: b[i] is an indexing, and i < len(b) does not follow
```

A function that walks two tables in step is making an assumption about their lengths. In
most languages that assumption is a comment, if it is written at all. Here it is part of the
header, and every caller must prove it.

<!-- gen go docs/book/code/tables/dot.oro -->
```go
func TablesDot(v0 []uint16, v1 []uint16) int {
    v16 := len(v0)
    v17 := v1[:v16]
    var v19 int = 0
    var v20 int = 0
    for ; ; v19 = (v19 + 1) {
        v21 := len(v0)
        v22 := (v19 < v21)
        if v22 {
            v27 := int(v0[v19])
            v28 := int(v17[v19])
            v29 := (v27 * v28)
            v30 := (v20 + v29)
            v20 = v30
            continue
        }
        break
    }
    return v20
}
```

One odd line: `v17 := v1[:v16]`. Go checks every access at run time, and its compiler removes
a check only when it can see for itself that the index is in range. It can see that for
`v0[v19]`, from the loop's test. It cannot see it for `v1`, because Go knows nothing of our
`where`. That line re-cuts `v1` to the length of `v0`, which changes nothing and lets Go's
compiler see what ours already proved. It is what an expert Go programmer writes by hand to
make such a loop fast.

---

## 14.8 Part of a table

**`t[lo:hi]`** is the part of `t` from position `lo` up to, not including, `hi`. Leave `hi`
out, `t[lo:]`, for "to the end". It is called a **segment** (or a slice).

<!-- file docs/book/code/tables/segment.oro -->
```go
export fn all_but_first(t ∈ Array(0..1000)) (Int) where 0 < len(t) && len(t) <= 10^6 {
    rest = t[1:]
    loop i = 0, s = 0 {
        i < len(rest) -> again(i + 1, s + rest[i])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/segment.oro -->
```go
func TablesAllButFirst(v0 []uint16) int {
    var v14 int = 0
    var v15 int = 0
    for ; ; v14 = (v14 + 1) {
        v16 := len(v0)
        v18 := (v16 - 1)
        v19 := (v14 < v18)
        if v19 {
            v24 := (1 + v14)
            v25 := int(v0[v24])
            v26 := (v15 + v25)
            v15 = v26
            continue
        }
        break
    }
    return v15
}
```

`rest` is a table in its own right: its positions start at 0 again, and its length is
`len(t) - 1`. And nothing was copied. A segment is a rule, "position `i` of `rest` is
position `1 + i` of `t`", and it fused like any rule: the Go reads `v0[1 + v14]` directly.

Half-open ranges, "from `lo` up to but not including `hi`", are the convention for the same
reason counting starts at zero. `t[lo:hi]` has `hi - lo` values, `t[0:k]` and `t[k:]` split
a table with nothing lost and nothing doubled, and an empty segment is `t[k:k]`.

---

## 14.9 Bytes are a table

Chapter 10 said: text a program must examine is handled as bytes, and becomes a `String` at
the end. Bytes are a table of numbers from 0 to 255, and everything in this chapter applies.

<!-- file docs/book/code/tables/bytes.oro -->
```go
use io

fn count(b, x) {
    loop i = 0, n = 0 {
        i >= len(b) -> n
        b[i] == x -> again(i + 1, n + 1)
        else -> again(i + 1, n)
    }
}

export fn main() {
    text = array { 104, 101, 108, 108, 111 }
    io.print_line(utf8_text(text))
    io.print_int(len(text))
    io.print_int(count(text, 108))
    io.print_line(ascii(text, 1, 3))
}
```

<!-- run go docs/book/code/tables/bytes.oro -->
```
hello
5
2
ell
```

There are the things a `String` would not do. A length: five bytes. A count: two `l`s (108).
A piece: `ascii(text, 1, 3)` is three bytes from position 1. Comparison is `==` on numbers.
They are all definite, because a byte table has no question of encoding left in it: its
length is its number of bytes, on every target.

<!-- file docs/book/code/tables/word.oro -->
```go
export fn first_space(b ∈ Array(0..255)) (Int) {
    loop i = 0 {
        i >= len(b) -> i
        b[i] == 32 -> i
        else -> again(i + 1)
    }
}
```

<!-- gen go docs/book/code/tables/word.oro -->
```go
func TablesFirstSpace(v0 []byte) int {
    var v3 int = 0
    for ; ; v3 = (v3 + 1) {
        v4 := len(v0)
        v5 := (v3 >= v4)
        if v5 {
            break
        }
        v10 := int(v0[v3])
        v12 := (v10 == 32)
        if v12 {
            break
        }
        continue
    }
    return v3
}
```

`Array(0..255)` is Go's `[]byte`, and this loop is how every tokenizer, parser and word
counter begins: walk the bytes until one is a space (32).

---

## 14.10 Thirty steps to search a billion

To find a name in a phone book you do not start at page one. You open in the middle, see
whether your name is before or after, and throw away half the book. Then again with the half
that is left. A book of a billion names takes thirty openings, because halving a billion
thirty times leaves one.

That is **binary search**, and it needs the table to be *sorted*.

<!-- file docs/book/code/tables/search.oro -->
```go
use io

fn half(lo, hi) { lo + (hi - lo) / 2 }

fn search(t, x) {
    loop lo = 0, hi = len(t), m = len(t) / 2 {
        lo >= hi -> -1
        lo < 0 || m < lo || m >= hi || hi > len(t) -> -1
        t[m] < x -> again(m + 1, hi, half(m + 1, hi))
        t[m] > x -> again(lo, m, half(lo, m))
        else -> m
    }
}

export fn main() {
    primes = array { 2, 3, 5, 7, 11, 13, 17, 19, 23, 29 }
    io.print_int(search(primes, 17))
    io.print_int(search(primes, 2))
    io.print_int(search(primes, 29))
    io.print_int(search(primes, 18))
    io.print_int(search(array { }, 5))
}
```

<!-- run go docs/book/code/tables/search.oro -->
```
6
0
9
-1
-1
```

17 is at position 6. 18 is not in the table, and neither is anything in an empty one.

The loop keeps a segment still worth searching, positions `lo` up to (not including) `hi`,
and `m`, a position in the middle of it. If `t[m]` is too small, the answer can only be
after `m`: search `m + 1` to `hi`. If too big, only before: search `lo` to `m`. Otherwise
`m` is it. When `lo` meets `hi`, nothing is left and the answer is "not there".

`half(lo, hi)` is `lo + (hi - lo) / 2` and not `(lo + hi) / 2`. They are the same number, and
the second overflows when the table is huge. That was a bug in Java's own library for nine
years, found in 2006, and chapter 7 told the story.

Now the second arm, which is this language talking. It is a guard of chapter 13's kind: it
says what is true of `lo`, `m` and `hi` every time round (`0 <= lo <= m < hi <= len(t)`), and
it is never false. It hands the compiler, as facts, what it needs to prove `t[m]`. It is not
needed any more. Written the obvious way, without `m` and with only the first guard, the
search compiles and answers the same:

<!-- file docs/book/code/tables/search-plain.oro -->
```go
use io

fn search(t, x) {
    loop lo = 0, hi = len(t) {
        lo < 0 || hi > len(t) || lo >= hi -> -1
        t[lo + (hi - lo) / 2] < x -> again(lo + (hi - lo) / 2 + 1, hi)
        t[lo + (hi - lo) / 2] > x -> again(lo, lo + (hi - lo) / 2)
        else -> lo + (hi - lo) / 2
    }
}

export fn main() {
    primes = array { 2, 3, 5, 7, 11, 13, 17, 19, 23, 29 }
    io.print_int(search(primes, 17))
    io.print_int(search(primes, 2))
    io.print_int(search(primes, 29))
    io.print_int(search(primes, 18))
    io.print_int(search(array { }, 5))
}
```

<!-- run go docs/book/code/tables/search-plain.oro -->
```
6
0
9
-1
-1
```

Past the first arm, `0 <= lo`, `lo < hi` and `hi <= len(t)`. The index is lo + q with
d = hi − lo ≥ 1 and q = ⌊d/2⌋, so 2q ≤ d, and q ≤ d − 1 because d ≥ 1. Then
lo ≤ lo + q ≤ lo + d − 1 = hi − 1 < len(t): the index is in the table. Intervals cannot see it,
because `lo` occurs twice, in lo + q and inside q, and the box of each forgets the other.
Chapter 7 §7.9 showed what the compiler does there: it writes the index as a formula of
Presburger arithmetic and decides it. Until 2026-11-21 it did not, this search was refused,
and the chapter carried `m` and the guard to hand it the facts. The guarded form above is kept
because it shows how to supply a fact the compiler cannot find, which is the next loop's
problem when it is not this one.

And one more honest line. Here is what it says about the version that compiles:

<!-- file docs/book/code/tables/search-gen.oro -->
```go
fn half(lo, hi) { lo + (hi - lo) / 2 }

export fn search(t ∈ Array(Int), x ∈ Int) (Int) where len(t) <= 10^9 {
    loop lo = 0, hi = len(t), m = len(t) / 2 {
        lo >= hi -> -1
        lo < 0 || m < lo || m >= hi || hi > len(t) -> -1
        t[m] < x -> again(m + 1, hi, half(m + 1, hi))
        t[m] > x -> again(lo, m, half(lo, m))
        else -> m
    }
}
```

<!-- gen go docs/book/code/tables/search-gen.oro -->
```
note: tables-search: 2 operation(s) in arbitrary precision
note: tables-search: 9 of 9 integer operations bounded; 0 of 1 loop(s) proven terminating
```

Every access proven, every operation on positions in the word, and **0 of 1 loops proven
terminating**. (The two operations in arbitrary precision are `t[m] < x` and `t[m] > x`: `x ∈
Int` is any integer, so each compares a word with a `*big.Int`. Give `x` the elements' set and
they are machine comparisons.)
The loop does stop: the measure is `hi - lo`, which at least halves each trip. Both `again`s
shrink it, one by raising `lo` past `m` and one by lowering `hi` to `m`. The compiler's
termination check looks for one variable going one way, and here two variables close in from
both sides. Chapter 13 said to read that count and look again at any loop it does not vouch
for. This is such a loop, and the argument in this paragraph is the looking.

Binary search is famous for being easy to describe and hard to get right. A study in the
1980s found that most professional programmers, given hours, wrote it with a bug. The usual
ones are an access one past the end and a loop that never ends. The first of those cannot
happen here. The second is still yours to check.

---

## 14.11 Lists, records and grids, from tables

Chapter 11 left two debts, and chapter 12 a third: a rectangle needs *two* numbers inside one
variant case, a variant may not contain itself, and neither may a record. All three are paid the
same way.

First, records and tables together. A table's elements may be records, given by a rule like any
table's:

<!-- file docs/book/code/tables/squares.oro -->
```go
use io

set Square = record { side ∈ 0..9, area ∈ 0..81 }

fn total(t) {
    loop i = 0, s = 0 {
        i < len(t) -> again(i + 1, s + t[i].area)
        else -> s
    }
}

export fn main() {
    squares = array { record { side : k, area : k * k } : k ∈ 0..9 }
    io.print_int(squares[7].area)
    io.print_int(total(squares))
}
```

<!-- run go docs/book/code/tables/squares.oro -->
```
49
285
```

<!-- gen go docs/book/code/tables/squares.oro -->
```go
…
func TablesMain() int {
    v1 := (strconv.Itoa(49))
    fmt.Println(v1)
    var v7 int = 0
    var v8 int = 0
    for ; ; v7 = (v7 + 1) {
        v10 := (v7 < 10)
        if v10 {
            v14 := (v7 * v7)
            v15 := (v8 + v14)
            v8 = v15
            continue
        }
        break
    }
    v17 := (strconv.Itoa(v8))
    fmt.Println(v17)
    return v8
}
```

`t[i].area` reads a record out of a table and a part out of the record. And look at what ran:
no table at all, and no `side`. §14.5's rule took no memory, the loop asked only for `area`, and
`k * k` was computed where it was read. A table of records written as a list of values,
`array { record { … }, record { … } }`, is refused today where its index is not known (13.12),
as a table of tuples is.

Then the three debts.

> **Where another language would hold a pointer to a thing, hold the thing's position in a
> table.**

A rectangle with a width and a height: keep all the widths in one table and all the heights
in another, and let the variant carry which one.

<!-- file docs/book/code/tables/shapes.oro -->
```go
use io

set Shape = 'circle(Int) ∪ 'rect(Int) ∪ 'point

fn area(s, widths, heights) {
    case s {
        'circle(r) -> 3 * r * r
        'rect(k) -> if 0 <= k && k < len(widths) && k < len(heights) { widths[k] * heights[k] } else { 0 }
        'point -> 0
    }
}

export fn main() {
    widths = array { 3, 10 }
    heights = array { 4, 2 }
    io.print_int(area('rect(0), widths, heights))
    io.print_int(area('rect(1), widths, heights))
    io.print_int(area('circle(2), widths, heights))
    io.print_int(area('rect(7), widths, heights))
}
```

<!-- run go docs/book/code/tables/shapes.oro -->
```
12
20
12
0
```

`'rect(0)` is "rectangle number 0": 3 by 4. A record with fields *width* and *height* has
become two tables named `widths` and `heights`, and one rectangle is one position across
both. Programmers call this layout "a struct of arrays", and the ones who care about speed
choose it on purpose: a loop over all the widths touches only widths, packed side by side in
memory.

`'rect(7)` names a rectangle that does not exist, and the `if` says what to do about it. A
position is only a number, so unlike a pointer it can be checked, and here it has to be.

A linked list: each item says which item comes next.

<!-- file docs/book/code/tables/list.oro -->
```go
use io

fn walk(value, next, start) {
    loop at = start, s = 0, steps = 0 {
        at < 0 || at >= len(next) || at >= len(value) || steps >= 1000 -> s
        else -> again(next[at], s + value[at], steps + 1)
    }
}

export fn main() {
    value = array { 30, 10, 20 }
    next = array { -1, 2, 0 }
    io.print_int(walk(value, next, 1))
    io.print_int(walk(value, next, 0))
    io.print_int(walk(value, next, 7))
}
```

<!-- run go docs/book/code/tables/list.oro -->
```
60
30
0
```

```
 position    0     1     2
 value      30    10    20
 next       -1     2     0

 start at 1:   10  →  (next is 2)  20  →  (next is 0)  30  →  (next is -1)  stop
```

Starting at position 1 the walk visits 10, 20, 30 and adds up 60. −1 is not a position, and
that is how the list says it has ended. The items sit in the table in one order and are
*linked* in another, which is the whole point of a linked list: reorder by changing `next`,
without moving anything.

The same picture with two `next` tables, `left` and `right`, is a tree. This is how a
variant "contains itself" here: it holds the position of another node.

Look at `steps >= 1000`. A list whose `next` values go round in a circle would be walked
forever. In a language with pointers that is a hang nobody can prevent. Here the walk counts
its steps, chapter 13's way of making a loop stop when there is no other measure, and so it
is proven to end.

A grid of rows and columns is one table, with the rows laid end to end:

<!-- file docs/book/code/tables/grid.oro -->
```go
export fn cell(grid ∈ Array(Int), width ∈ 1..1000, row ∈ 0..999, col ∈ 0..999) (Int) {
    k = row * width + col
    if 0 <= k && k < len(grid) { grid[k] } else { 0 }
}
```

<!-- gen go docs/book/code/tables/grid.oro -->
```go
func TablesCell(v0 []int, v1 int, v2 int, v3 int) int {
    v42 := (v2 * v1)
    v43 := (v42 + v3)
    v46 := (0 <= v43)
    v47 := (v46 && (v43 < len(v0)))
    if v47 {
        v54 := v0[v43]
        return v54
    }
    return 0
}
```

Row 2, column 1 of a grid 3 wide is position 2 × 3 + 1 = 7. Every image, every spreadsheet
and every matrix in every program is stored this way underneath. The position is tested
against the length here because it is a product of two unknowns, and the compiler's
arithmetic reasoning is about sums.

### Pairs in a table

A table's elements may be tuples. Such a table arrives from outside or is built in memory
(chapter 15), and its elements are read with a second index for the part:

<!-- file docs/book/code/tables/pairs.oro -->
```go
export fn area(sides ∈ Array(0..100 × 0..100)) (Int) where len(sides) <= 1000 {
    loop i = 0, s = 0 {
        i < len(sides) -> again(i + 1, s + sides[i][0] * sides[i][1])
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/pairs.oro -->
```go
func TablesArea(v0 []byte) int {
    var v7 int = 0
    var v8 int = 0
    for ; ; v7 = (v7 + 1) {
        v9 := len(v0)
        v11 := (v9 >> 1)
        v12 := (v7 < v11)
        if v12 {
            v17 := (2 * v7)
            v18 := int(v0[v17])
            v20 := (2 * v7)
            v22 := (v20 + 1)
            v23 := int(v0[v22])
            v24 := (v18 * v23)
            v25 := (v8 + v24)
            v8 = v25
            continue
        }
        break
    }
    return v8
}
```

`sides[i][0]` is the first part of pair `i`, and `sides[i][1]` the second. `0..100 × 0..100`
is chapter 11's product set. Look at what Go received: one flat `[]byte`, the pairs laid end
to end. Pair `i` is at positions `2 * i` and `2 * i + 1`, and the number of pairs is the
byte count halved (`>> 1`). It is 13.11's grid, two wide, worked out for you, with every
index proven from the one guard `i < len(sides)`.

Chapter 11's pattern takes a pair apart by name, here as anywhere:

<!-- file docs/book/code/tables/pairs-named.oro -->
```go
export fn area(sides ∈ Array(0..100 × 0..100)) (Int) where len(sides) <= 1000 {
    loop i = 0, s = 0 {
        i < len(sides) -> { (w, h) = sides[i]; again(i + 1, s + w * h) }
        else -> s
    }
}
```

<!-- gen go docs/book/code/tables/pairs-named.oro -->
```go
note: tables-area: 3 product access(es) flattened
note: tables-area: 7 of 7 integer operations bounded; 1 of 1 loop(s) proven terminating
note: tables-area: 1 division(s) became a shift or a mask
…
func TablesArea(v0 []byte) int {
    var v7 int = 0
    var v8 int = 0
    for ; ; v7 = (v7 + 1) {
        v9 := len(v0)
        v11 := (v9 >> 1)
        v12 := (v7 < v11)
        if v12 {
            v15 := (2 * v7)
            v16 := int(v0[v15])
            v18 := (2 * v7)
            v20 := (v18 + 1)
            v21 := int(v0[v20])
            v24 := (v16 * v21)
            v25 := (v8 + v24)
            v8 = v25
            continue
        }
        break
    }
    return v8
}
```

`(w, h) = sides[i]` names the two parts, and the Go is the same two reads. No pair was
built in order to be taken apart: a pattern on a table's element is its two positions.

A table of pairs may also be written out as a list, and read at any position. The shoelace
formula gives twice a polygon's signed area from its corners in order,
2A = Σ (x_i · y_{i+1} − x_{i+1} · y_i), the last corner followed by the first:

<!-- file docs/book/code/tables/shoelace.oro -->
```go
use io

fn area(p) {
    loop i = 0, s = 0 {
        i < len(p) && i < 1000 -> {
            j = if i + 1 < len(p) { i + 1 } else { 0 }
            (x, y) = p[i]
            (u, v) = p[j]
            again(i + 1, s + x * v - u * y)
        }
        else -> s
    }
}

export fn main() {
    io.print_int(area(array { (0, 0), (4, 0), (0, 3) }))
    io.print_int(area(array { (0, 0), (0, 5), (5, 5), (5, 0) }))
}
```

<!-- run go docs/book/code/tables/shoelace.oro -->
```
12
-50
```

The 3-4-5 triangle has area 6, so 12; the square of side 5 walked clockwise has −50. The list is
one table of coordinates, x then y for each corner, as if the program had built it.

And a table of tables may be written out the same way, a square as its rows. Lo Shu, the magic
square of order 3, has every row, column and diagonal summing to 15:

<!-- file docs/book/code/tables/loshu.oro -->
```go
use io

export fn main() {
    sq = array { array { 4, 9, 2 }, array { 3, 5, 7 }, array { 8, 1, 6 } }
    lines = loop i = 0, n = 0 {
        i < 3 -> {
            row = sq[i][0] + sq[i][1] + sq[i][2]
            col = sq[0][i] + sq[1][i] + sq[2][i]
            again(i + 1, n + (if row == 15 { 1 } else { 0 }) + (if col == 15 { 1 } else { 0 }))
        }
        else -> n
    }
    io.print_int(lines)
}
```

<!-- run go docs/book/code/tables/loshu.oro -->
```
6
```

`sq[i]` at an `i` the loop chooses is one of three rows, and the compiler writes it that way: if
`i` is 0 the first row, if 1 the second, else the third. Each row is a table written out, so each
read of it is checked against its own length. `i` must still be proven in 0..2, as any index is.

---

## 14.12 What is not here

- **A table cannot be changed.** Building one a piece at a time, or updating a position, is
  chapter 15.
- **A table of tables is written out, never made by a rule, received or built.** A grid that is
  computed is one table (13.11).
- **No `==` on tables.** Compare them position by position, in a loop.

---

## 14.13 What other languages do

| | a position outside the table | where the check happens | a chain of maps | the first position |
|---|---|---|---|---|
| Oroboros | the program is refused | when compiling | one loop, always | 0 |
| C, C++ | reads or writes other memory | never | what you write by hand | 0 |
| Go | panic | at run time, each access | what you write by hand | 0 |
| Java, C# | exception | at run time, each access | streams: objects per stage | 0 |
| Python | exception; negative counts from the end | at run time | a new list per stage, or generators | 0 |
| JavaScript | `undefined` | none | a new array per `.map` | 0 |
| Rust | panic, or `get` returns an `Option` | at run time, removed where the compiler can | iterators: usually one loop | 0 |
| Haskell | exception | at run time | one loop where the rules fire | 0 |
| Fortran, Lua, MATLAB | varies | varies | varies | 1 |

Rust is the nearest again: `t.get(i)` returning `Option` is this chapter's `maybe`, and its
iterators are fused by construction. The difference is that `t[i]` in Rust still compiles
with an unproven index and checks at run time.

The table-as-function idea is from the mathematical specification languages (Z, TLA+), and
the rule that takes no memory until needed is from APL (1962) and its descendants.

---

## What to remember

- **A table is many values of one kind in a row**: `array { 2, 3, 5 }`. It never changes.
- **`t[i]` is the value at position `i`; `len(t)` is the number of positions.** Positions are
  0 to len − 1. A table is a function on them.
- **`t[i]` must be proven inside**: by a `where` in the header, by a test, or by a loop's own
  guard. Nothing is checked at run time.
- **Walk a table with `loop i = 0 { i < len(t) -> … t[i] … again(i + 1) }`.** The guard is the
  proof. `<=` there is refused.
- **`Array(S)` is the set of tables with elements in `S`**, and `S` picks the storage: a table
  of `0..255` is bytes.
- **Say how long a table can be** when you sum over it: `where len(t) <= 10^6`.
- **A rule, `array { e : i ∈ 0..n-1 }`, gives the value at each position.** Looking it up is
  calling it, so a rule used inside a program takes no memory, and chains of rules become one
  loop.
- **Two tables walked together need `where len(a) == len(b)`.**
- **`t[lo:hi]` is a segment**: positions `lo` up to, not including, `hi`. Nothing is copied.
- **Bytes are `Array(0..255)`**, and text a program examines is handled as bytes.
- **Binary search**: keep a segment and its midpoint, halve each time. Carry the facts the
  compiler cannot find as a guard, and check termination yourself where it says "0 of 1".
- **For records, lists, trees and grids, hold positions where others hold pointers.**

Next: [chapter 15](15-state.md), State. Building a table a piece at a time, and why a table still never
changes under you.
