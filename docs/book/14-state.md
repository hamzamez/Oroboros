# 14. State

For twelve chapters nothing has changed. A name is bound once. A string is never altered. A
table is what it was when it was made. Chapter 12 got repetition without change, by binding
the loop's names afresh each time round.

That discipline bought a great deal: any part of a program can hold any value and be sure of
it. But some programs are naturally told as a sequence of changes. "Start the count at zero;
for each word, add one." "Make a row of a thousand zeros; for each number that arrives, mark
its slot." Chapter 13's rule fills a table by saying what each position *is*. It cannot say
"for each input, go and change position such-and-such", and a great many algorithms are
that.

So the question of this chapter is: **how does a program build something a piece at a time,
without giving up the guarantee that a value you hold never changes?**

The answer is three sentences.

> **`var x := e` declares a name that may be written again**, with `x := e`.
>
> **A `var` that holds a table is a buffer: it may be written one position at a time**,
> `b[i] := v`. When its scope ends it becomes an ordinary table and never changes again.
>
> **While a buffer can be written it has exactly one name**, so nobody else can be holding it
> when it changes.

Along the way: why "the same variable, later" is a harder idea than it looks, the bug that
has two names for one thing, a way of finding primes that is twenty-two centuries old, and
what a compiler really does with your variables.

The examples are in [code/state/](code/state/). `go run docs/book/check.go
docs/book/14-state.md` re-runs every block. Checked 2026-11-16.

---

## 14.1 A name that can be written again

<!-- file docs/book/code/state/cell.oro -->
```go
use io

export fn main() {
    var x := 1
    io.print_int(x)
    x := x + 10
    io.print_int(x)
    x := x * 2
    io.print_int(x)

    var name := "a"
    name := name ++ "b"
    name := name ++ "c"
    io.print_line(name)

    var on := false
    on := !on
    if on { io.print_line("on") }
}
```

<!-- run go docs/book/code/state/cell.oro -->
```
1
11
22
abc
on
```

**`var x := 1`** declares `x` and gives it its first value. **`x := x + 10`** writes a new
value: the right side is computed with the old `x`, and from that line on `x` is the result.
Such a name is called a **cell** here (most languages just say "variable").

There are now two symbols, and they mean different things:

| | | may the name be given another value later? |
|---|---|---|
| `x = e` | *x is e.* A definition, as in mathematics | no |
| `var x := e`, then `x := e` | *put e in x.* An instruction | yes |

In C, Java and Python one sign, `=`, does both jobs, and "is `x = x + 1` an equation?" is
the first thing that confuses every beginner. (As an equation it is false for every number.)
Here `=` is always an equation and `:=` is always an instruction. Most of a program's names
never change, and now you can see at a glance which few do: they say `var`.

Writing to a name that was not declared a `var` is refused:

<!-- file docs/book/code/state/no-write.oro -->
```go
export fn f(n ∈ 0..100) (Int) {
    x = 1
    x := n
    x
}
```

<!-- gen go docs/book/code/state/no-write.oro -->
```
gen: state-f: `:=` writes a `var`, and the place written here, 1, is none: a name bound with `=` is a value, which never changes (ADR 0047). Declare it with `var x := …` where it is bound, or bind the new value to a new name
```

---

## 14.2 What a cell really is

Here is the surprise. A cell does not add anything to the language. It is a way of
*writing*, and what it stands for is something you already have.

<!-- file docs/book/code/state/cells.oro -->
```go
export fn bonus(n ∈ 0..100) (Int) {
    var total := 0
    if n > 50 { total := total + 100 }
    total := total + n
    total
}

export fn snapshot(n ∈ 0..100) (Int) {
    var x := 1
    y = x
    x := n
    y + x
}
```

<!-- gen go docs/book/code/state/cells.oro -->
```go
func StateBonus(v0 int) int {
    v10 := (v0 > 50)
    var v11 int
    if v10 {
        v11 = 100
    } else {
        v11 = 0
    }
    v16 := (v11 + v0)
    return v16
}

func StateSnapshot(v0 int) int {
    v10 := (1 + v0)
    return v10
}
```

There is no variable called `total` being updated in `bonus`. There are *three values*: the
total at the start (0), the total after the `if` (100 or 0, depending), and the total at the
end. Each is computed once and never altered. The compiler gave each one its own name.

That is the whole meaning of a cell: **each `:=` starts a new name, and every later use of
the cell means the latest one.** A write under an `if` means "the new value on that path, the
old value on the other", and the two are joined after it. `x := x + 10` does not change a
value. It retires a name.

`snapshot` shows why it matters. `y = x` took the *value* `x` had at that moment, 1. The
later `x := n` gave `x` a new value and could not reach back and alter `y`: the answer is
`1 + n`. A value you have taken stays the value you took. That is the guarantee of the first
twelve chapters, and cells do not weaken it.

A cell written inside a loop becomes one of the loop's variables. Chapter 12's sum, written
with a cell:

<!-- file docs/book/code/state/sum.oro -->
```go
export fn sum_to(n ∈ 0..1000) (Int) {
    var s := 0
    loop i = 1 {
        i <= n -> { s := s + i; again(i + 1) }
        else -> s
    }
}
```

<!-- gen go docs/book/code/state/sum.oro -->
```go
func StateSumTo(v0 int) int {
    var v12 int = 0
    var v13 int = 1
    for ; ; v13 = (v13 + 1) {
        v14 := (v13 <= v0)
        if v14 {
            v17 := (v12 + v13)
            v12 = v17
            continue
        }
        break
    }
    return v12
}
```

The same Go as chapter 12's `loop i = 1, s = 0 { … again(i + 1, s + i) … }`. The two
spellings are one program. Use whichever reads better: a second loop variable when a value
changes on every trip, a cell when it changes on some trips and not others.

<!-- file docs/book/code/state/counters.oro -->
```go
export fn summary(n ∈ 0..100) (Int) {
    var count := 0
    var total := 0
    loop i = 0 {
        i >= n -> total * 1000 + count
        else -> {
            total := total + i
            if i % 3 == 0 { count := count + 1 }
            again(i + 1)
        }
    }
}
```

<!-- gen js docs/book/code/state/counters.oro -->
```js
export function stateSummary(v0) {
    let v13 = 0;
    let v14 = 0;
    let v15 = 0;
    for (;; v15 = (v15 + 1)) {
        if ((v15 >= v0)) {
            return ((v14 * 1000) + v13);
        }
        v13 = (((((v15 % 3) + 0)) === 0) ? (v13 + 1) : v13);
        v14 = (v14 + v15);
        continue;
    }
}
```

With loop variables this would need `again(i + 1, …, …)` spelled out in two arms, one that
counts and one that does not. With cells, `count` is written where it changes and nowhere
else, and `again(i + 1)` mentions only the counter. Look at the JavaScript: `count := count +
1` under an `if` became `v13 = cond ? v13 + 1 : v13`, a new value on one path and the old one
on the other, exactly as described.

Every optimizing compiler for every language does this rewriting to your variables before
it does anything else. It is called *static single assignment*, and it was invented (in
1988) because code in which each name has one value is so much easier to reason about. Here
it is not an internal trick. It is what the notation means.

### A record in a cell

A cell may hold a record, and then each part can be written on its own:

<!-- file docs/book/code/state/pen.oro -->
```go
use io

set Point = record { x ∈ -100..100, y ∈ -100..100 }

export fn main() {
    var p := zero(Point)
    p.x := 3
    p.y := p.x * 2
    p.x := p.x - 5
    io.print_int(p.x)
    io.print_int(p.y)
}
```

<!-- run go docs/book/code/state/pen.oro -->
```
-2
6
```

`p.x := 3` is chapter 11's `with`, done in program order: it means `p := p with { x : 3 }`, and
`p.y := p.x * 2` reads the `x` just written. Nothing is changed that anyone else could see,
because only `p`'s own block can name `p`. Every value here is known, so the compiler computed
the program: `main` prints −2 and 6.

### A cell and a function

A function written inside the cell's block may not mention the cell:

<!-- file docs/book/code/state/no-closure.oro -->
```go
use io

export fn main() {
    var n := 0
    add = fn(x) { n := n + x }
    add(1)
    add(2)
    io.print_int(n)
}
```

<!-- gen go docs/book/code/state/no-closure.oro -->
```
gen: docs/book/code/state/no-closure.oro: main: a function that mentions a cell (n): fn(x) { n := n + x } may run any number of times, or later, so the cell's order would not be the program's. A cell may be read and written in the scope, a case arm, a loop and a host call's continuation; a λ at a callback's field may capture one it passes to a step's cell parameter (ADR 0133), and callbacks applied during a call are tier 1, not built (spec/local.md §3)
```

The reading above says why. Each `:=` starts a new name, and a later use means the latest one,
*in the order the program is written*. `add` is a value: it could be applied twice, or never, or
handed to something that applies it later. Where `add` is written, "the latest `n`" has no
answer. A loop's body is the one function that may mention a cell, because `loop` applies it in
an order the program states, and `again` passes the cell's next value as it passes the counter's.

The working form says what `add` meant: take the value in, give the new one back, and let the
block write it.

<!-- file docs/book/code/state/add.oro -->
```go
use io

fn add(n, x) { n + x }

export fn main() {
    var n := 0
    n := add(n, 1)
    n := add(n, 2)
    io.print_int(n)
}
```

<!-- run go docs/book/code/state/add.oro -->
```
3
```

Until 2026-11-20 the first program compiled and printed 0, the writes inside `add` lost (the
book's finding 102). A function *read* the cell no better: `get = fn() { n }` gave the cell's
first value, whatever had been written since.

---

## 14.3 Building a table in place

A cell holds a number or a string. When a `var` is given a table, it becomes something
stronger: a **buffer**, which can be written one position at a time.

<!-- file docs/book/code/state/squares.oro -->
```go
export fn squares(n ∈ 0..1000) (Array(Int)) {
    var b := array { 0 : _ ∈ 0..n-1 }
    loop i = 0 {
        i < n -> { b[i] := i * i; again(i + 1) }
        else -> b
    }
}
```

<!-- gen go docs/book/code/state/squares.oro -->
```go
func StateSquares(v0 int) []int {
    v10 := make([]int, v0)
    var v9 []int
    v13 := v10
    var v14 int = 0
    for ; ; v14 = (v14 + 1) {
        v15 := (v14 < v0)
        if v15 {
            v18 := (v14 * v14)
            v13[v14] = v18
            continue
        }
        break
    }
    v9 = v13
    return v9
}
```

- **`var b := array { 0 : _ ∈ 0..n-1 }`** makes a buffer of `n` zeros. It is chapter 13's
  rule, "the value at every position is 0". The `_` is a name you write when you do not need
  the name: the rule does not depend on the position.
- **`b[i] := i * i`** is a **store**: put this value at position `i`.
- **`b`**, as the loop's answer, hands the finished table out.

One block of memory was made (`make`), and each store wrote into it directly (`v13[v14] =
v18`). Nothing was copied.

For squares a rule would have done, and better. A buffer earns its keep when *the data
decides where to write*. Count how often each digit occurs:

<!-- file docs/book/code/state/histogram.oro -->
```go
use io

fn histogram(t) {
    var h := array { 0 : _ ∈ 0..9 }
    loop i = 0 {
        i >= len(t) -> h
        t[i] < 0 || t[i] > 9 -> again(i + 1)
        else -> { h[t[i]] := h[t[i]] + 1; again(i + 1) }
    }
}

export fn main() {
    h = histogram(array { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5 })
    io.print_int(h[1])
    io.print_int(h[5])
    io.print_int(h[7])
    io.print_int(len(h))
}
```

<!-- run go docs/book/code/state/histogram.oro -->
```
2
3
0
10
```

Two 1s, three 5s, no 7s. `h[t[i]] := h[t[i]] + 1` reads the count for the digit `t[i]` and
stores it back one higher. Which position is written depends on the input, and one position
may be written many times. A rule cannot say that: a rule computes position `k` from `k`.
The two have names. A rule is a **gather**: each output position goes and fetches what it
needs. A buffer does **scatter**: each input goes and writes where it belongs.

The second arm matters. `h` has positions 0 to 9, so `h[t[i]]` needs `t[i]` to be in 0..9,
and the arm before it sends everything else round again untouched. Below it, `t[i]` is
proven a digit.

<!-- run js docs/book/code/state/histogram.oro -->
```
2
3
0
10
```

<!-- run windows docs/book/code/state/histogram.oro -->
```
2
3
0
10
```

### A store's position must be inside, too

Chapter 13's rule for reading, "`t[i]` only where `i` is proven a position", is the rule for
writing as well, with more at stake: reading past the end leaks a secret, and writing past
the end corrupts whatever is there.

So the compiler proves it, the same way. A buffer of length `n` has positions 0 to `n - 1`,
and this writes one past the last:

<!-- file docs/book/code/state/no-past.oro -->
```go
export fn seven(n ∈ 1..100) (Int) {
    t = {
        var b := array { 0 : _ ∈ 0..n-1 }
        b[n] := 7
        b
    }
    t[0]
}
```

<!-- gen go docs/book/code/state/no-past.oro -->
```
gen: state-seven: b[n] := … is a store, defined only on its table's domain, and n < len(b) does not follow
  known: assumed -n + +1 <= 0, assumed n + -100 <= 0
```

"A store, defined only on its table's domain": a table is a function on its positions, and
a store that names a position outside them does not mean anything. In C this line compiles,
runs, and overwrites whatever happens to sit after the buffer. That one mistake, the
**buffer overflow**, has been the most exploited class of defect in software for forty
years. Here it is not a program.

That is why `histogram` has its second arm, and why every store in this chapter stands
under a test that puts its position inside.

---

## 14.4 Freezing

A buffer lives in a **scope**: the block of braces in which its `var` was declared. When the
scope ends and the buffer is given as its value, the buffer becomes a table. It is **frozen**.

<!-- file docs/book/code/state/frozen.oro -->
```go
export fn seven(n ∈ 1..100) (Int) {
    t = {
        var b := array { 0 : _ ∈ 0..n-1 }
        b[0] := 7
        b
    }
    t[0]
}
```

<!-- gen go docs/book/code/state/frozen.oro -->
```go
func StateSeven(v0 int) int {
    v10 := make([]byte, v0)
    v10[0] = byte(7)
    v15 := int(v10[0])
    return v15
}
```

Inside the braces `b` is a buffer and may be stored into. The braces end with `b`, and
`t = { … }` names the result. `t` is a table, exactly like one written `array { … }`.
Freezing costs nothing: the Go has one `make`, and `t` is that same memory under a new name.
What changed is only what the program is allowed to do with it:

<!-- file docs/book/code/state/no-frozen.oro -->
```go
export fn seven(n ∈ 1..100) (Int) {
    t = {
        var b := array { 0 : _ ∈ 0..n-1 }
        b[0] := 7
        b
    }
    t[0] := 8
    t[0]
}
```

<!-- gen go docs/book/code/state/no-frozen.oro -->
```
gen: state-seven: a store writes into t, which is a frozen table, not a buffer.
  A store writes a LIVE buffer, one a scope such as `var b := array { 0 : _ ∈ 0..n-1 }` is still filling. A frozen
  table is immutable (ADR 0018), and every other name that shares it would see the
  write (tables.md §2.5, S). For a changed copy, copy the table into a new buffer and
  store there: a buffer initialized from a table, `var c := array { t[k] : k ∈ 0..len(t)-1 }`
  (spec/local.md §1).
```

"Every other name that shares it would see the write." A table can be passed to ten
functions and stored in ten places, and all of them rely on it staying as it is. So a table
that anyone else might hold cannot be written. A function's table parameter is such a
table:

<!-- file docs/book/code/state/no-param.oro -->
```go
export fn zero_first(t ∈ Array(Int)) (Array(Int)) where len(t) > 0 {
    t[0] := 0
    t
}
```

<!-- gen go docs/book/code/state/no-param.oro -->
```
gen: state-zero-first: a store writes into t, which is a frozen table, not a buffer.
```

The caller still has `t`, and did not ask for it to be altered. The message says what to do
instead: make your own.

<!-- file docs/book/code/state/copy.oro -->
```go
export fn zero_first(t ∈ Array(Int)) (Array(Int)) where len(t) > 0 {
    var b := array { t[i] : i ∈ 0..len(t)-1 }
    b[0] := 0
    b
}
```

<!-- gen js docs/book/code/state/copy.oro -->
```js
export function stateZeroFirst(v0) {
    const v4 = v0.length;
    const v6 = new Array(v4).fill(0);
    let v5;
    let v9 = 0;
    let v10 = v6;
    for (;; v9 = (v9 + 1)) {
        if ((v9 >= v4)) {
            break;
        }
        v10[v9] = v0[v9];
        continue;
    }
    v10[0] = 0;
    v5 = v10;
    return v5;
}
```

`var b := array { t[i] : i ∈ 0..len(t)-1 }` is a buffer whose first contents are a copy of
`t`. The copy is in the source, where you can see what it costs. The caller's `t` is
untouched, and the function returns a new table that differs in one place.

So the life of every table has two parts. **First it is a buffer**: one name, writable, not
yet visible to anyone else. **Then it is frozen**: any number of names, never written again.
The order is fixed, and there is no way back.

---

## 14.5 One name only

Why exactly one name? Here is the program the rule forbids:

<!-- file docs/book/code/state/no-alias.oro -->
```go
export fn two_names(n ∈ 1..100) (Int) {
    var b := array { 0 : _ ∈ 0..n-1 }
    d = b
    b[0] := 7
    d[0]
}
```

<!-- gen go docs/book/code/state/no-alias.oro -->
```
gen: docs/book/code/state/no-alias.oro: two-names: R1: the buffer b is bound to another name (a buffer is named only by its binder; spec/local.md §2.1, ADR 0052): { d = b; b := store(b, 0, 7); d(0) }
```

What should `d[0]` be? If `d = b` means "`d` is the table `b` held at that moment", it is 0.
If `d` and `b` are two names for one block of memory, it is 7. In Python, JavaScript and Java
it is 7, and this is the most common way for one part of a program to break another without
either being wrong on its own:

```
 a = [1, 2, 3]          # Python
 b = a
 b[0] = 99
 print(a[0])            # 99
```

Two names for one changeable thing is called **aliasing**. Every "who changed my list?" bug
is this. Section 14.2 said `y = x` takes a value that then stays put, and that has to hold
for tables as well. With a number it is free: the number is copied. With a million-element
buffer there are only three choices. Copy it (slow, and silently so). Let the two names share
it (the bug above). Or refuse the second name while writing is possible. This language takes
the third.

That is the full answer to the chapter's question. A value you hold never changes, because
the only things that change are buffers, and nobody but the one name that is filling a
buffer can hold it.

*Reading* a buffer while it is being built is fine, and so is handing it to a function that
only reads:

<!-- file docs/book/code/state/observe.oro -->
```go
use io

fn total(t) { loop i = 0, s = 0 { i < len(t) -> again(i + 1, s + t[i]); else -> s } }

export fn main() {
    var b := array { 0 : _ ∈ 0..3 }
    b[0] := 5
    io.print_int(total(b))
    b[1] := 6
    io.print_int(total(b))
    b[0] := 0
    io.print_int(total(b))
}
```

<!-- run go docs/book/code/state/observe.oro -->
```
5
11
6
```

`total` looks, answers and is finished before the next store happens. It keeps nothing. Each
call sees the buffer as it is at that moment: 5, then 5 + 6, then 0 + 6.

---

## 14.6 Lending a buffer

A function that only reads a buffer takes it like any table. A function that *writes* one
has to say so in its header:

<!-- file docs/book/code/state/sort.oro -->
```go
use io

fn sink(b ∈ Array(Int) linear, start) {
    loop j = start {
        j <= 0 || j >= len(b) -> ()
        b[j - 1] <= b[j] -> ()
        else -> {
            tmp = b[j]
            b[j] := b[j - 1]
            b[j - 1] := tmp
            again(j - 1)
        }
    }
}

fn sort(b ∈ Array(Int) linear) {
    loop i = 1 {
        i >= len(b) -> ()
        else -> { sink(b, i); again(i + 1) }
    }
}

export fn main() {
    var b := array { 3, 1, 4, 1, 5, 9, 2, 6 }
    sort(b)
    loop i = 0 {
        i < len(b) -> { io.print_int(b[i]); again(i + 1) }
        else -> io.print_line("sorted")
    }
}
```

<!-- run go docs/book/code/state/sort.oro -->
```
1
1
2
3
4
5
6
9
sorted
```

**`b ∈ Array(Int) linear`** says: this function is *lent* a buffer and may store into it.
The word `linear` means "exactly one": for the length of the call the function holds the one
name, and when it returns the caller has it back, with whatever was written. No copy is made
in either direction. `sort(b)` in `main` is followed by reads of `b`, and they see it sorted.

This is **insertion sort**, the way most people sort a hand of cards. `sort` walks up the
buffer. For each position `i` it calls `sink`, which takes the value there and swaps it
downward past every larger neighbour until it meets one that is not larger, or reaches the
bottom. Everything below `i` is already in order when `sink` starts, so one value sinking
into place keeps it in order.

The swap is three lines and a name, `tmp`. `tmp = b[j]` takes the *value* at `j`, a number,
which stays what it was when `b[j]` is overwritten on the next line: 14.2 again.

`sink`'s first arm is its guard and its proof: below it `0 < j` and `j < len(b)`, so both
`b[j]` and `b[j - 1]` are positions.

Lending obeys the one-name rule. The same buffer cannot be lent twice in one call:

<!-- file docs/book/code/state/no-twice.oro -->
```go
fn mix(a ∈ Array(Int) linear, c ∈ Array(Int) linear) { 0 }

export fn both(n ∈ 0..100) (Int) {
    var b := array { 0 : _ ∈ 0..n }
    mix(b, b)
}
```

<!-- gen go docs/book/code/state/no-twice.oro -->
```
gen: docs/book/code/state/no-twice.oro: both: R2: the buffer b is handed to one call twice (a call borrows a buffer once; spec/local.md §2.1, ADR 0052): mix(b, b)
```

Inside `mix`, `a` and `c` would be two names for one buffer: 14.5's alias, arriving through
the parameter list.

<!-- run js docs/book/code/state/sort.oro -->
```
1
1
2
3
4
5
6
9
sorted
```

<!-- run windows docs/book/code/state/sort.oro -->
```
1
1
2
3
4
5
6
9
sorted
```

### A cell lent: `inout`

A buffer is lent with `linear`. A cell is lent with `inout`: the function may read it and write
it, and the caller sees what was written.

<!-- file docs/book/code/state/bump.oro -->
```go
use io

fn bump(c ∈ 0..1000 inout, by ∈ 0..10) { c := c + by }

export fn main() {
    var n := 7
    bump(n, 3)
    bump(n, 3)
    io.print_int(n)
}
```

<!-- run go docs/book/code/state/bump.oro -->
```
13
```

`c ∈ 0..1000 inout` is a claim about the cell's value, as `∈` always is, and `inout` says how it
is passed: in, and back out. It is a *mode*, a word about the relation between `c` and the caller's
`n`, and not a set: there is no set of cells, only of values. Each call is a value in and a value
out, so `bump(n, 3)` is `n := n + 3`, and after two calls `n` is 13.

### At the program's edge

An exported function that writes a buffer its caller lends says so in its result, with the name
primed:

<!-- file docs/book/code/state/handback.oro -->
```go
export fn squares(dst ∈ Array(0..10000) linear) (dst' ∈ Array(0..10000) linear) where len(dst) <= 100 {
    loop i = 0 {
        i < len(dst) -> { dst[i] := i * i; again(i + 1) }
        else -> ()
    }
}
```

<!-- gen go docs/book/code/state/handback.oro -->
```go
func StateSquares(v0 []uint16) []uint16 {
    v6 := v0
    var v7 int = 0
    for ; ; v7 = (v7 + 1) {
        v8 := len(v6)
        v9 := (v7 < v8)
        if v9 {
            v12 := (v7 * v7)
            v6[v7] = uint16(v12)
            continue
        }
        break
    }
    return v6
}
```

`dst'` is read "dst after": the same buffer, handed back written. Go receives its own slice and
gets it back, written in place, with no copy. (`uint16`: every element is proven at most 10000,
so Go stores two bytes each.) Inside a program the handing back goes without saying, as `sort`
showed; at an export the caller is a host, and the header is what it reads.

---

## 14.7 The sieve

Chapter 12 tested one number for primality by trial division. To find *all* the primes up to
a limit there is a far better way, published by Eratosthenes of Cyrene around 240 BC, and it
is the classic scatter.

Write down the numbers from 2 to n. Take the first one not crossed out, 2: it is prime. Cross
out every multiple of it. Take the next one not crossed out, 3: prime. Cross out its
multiples. Go on. Whatever is never crossed out is prime.

<!-- file docs/book/code/state/sieve.oro -->
```go
use io

fn cross(c ∈ Array(Bool) linear, i ∈ 2..10000, n ∈ 2..10000) {
    loop j = i * i {
        j > n || j >= len(c) -> ()
        else -> { c[j] := true; again(j + i) }
    }
}

fn sieve(n ∈ 2..10000) {
    var composite := array { false : _ ∈ 0..n }
    loop i = 2 {
        i * i > n -> composite
        composite[i] -> again(i + 1)
        else -> { cross(composite, i, n); again(i + 1) }
    }
}

fn count(c) {
    loop k = 2, n = 0 {
        k >= len(c) -> n
        c[k] -> again(k + 1, n)
        else -> again(k + 1, n + 1)
    }
}

export fn main() {
    io.print_int(count(sieve(100)))
    io.print_int(count(sieve(10000)))
}
```

<!-- run go docs/book/code/state/sieve.oro -->
```
25
1229
```

There are 25 primes below 100 and 1229 below 10000.

- `composite` is a buffer of booleans, one for each number from 0 to n, all `false` to begin
  with: nothing crossed out. (A number that is not prime is called *composite*.)
- `sieve` walks `i` upward. If `composite[i]` is already true, `i` is a multiple of something
  smaller: skip it. Otherwise `i` is prime, and `cross` is lent the buffer to cross out its
  multiples.
- `cross` starts at `i * i`, because the smaller multiples (2i, 3i, …) were crossed out when
  2, 3, … had their turn. For the same reason `sieve` stops once `i * i` passes `n`.
- When `sieve` is done it gives `composite` as its answer, frozen, and `count` reads it.

Not one division is performed. Finding the primes below ten thousand this way takes a few
thousand stores, where testing each number by trial division takes hundreds of thousands of
remainders. It only works because a store can land anywhere: crossing out the multiples of 7
writes positions 49, 56, 63, …, scattered across the table.

---

## 14.8 A buffer and a count

Often you do not know in advance how many values you will produce. Make the buffer as big as
it could need to be, and keep a cell saying how much of it is used:

<!-- file docs/book/code/state/evens.oro -->
```go
use io

fn evens(t) {
    var b := array { 0 : _ ∈ 0..9 }
    var n := 0
    loop i = 0 {
        i >= len(t) || n >= 10 -> (b, n)
        t[i] % 2 == 0 -> { b[n] := t[i]; n := n + 1; again(i + 1) }
        else -> again(i + 1)
    }
}

export fn main() {
    (found, n) = evens(array { 3, 1, 4, 1, 5, 9, 2, 6 })
    io.print_int(n)
    io.print_int(found[0])
    io.print_int(found[1])
    io.print_int(found[2])
}
```

<!-- run go docs/book/code/state/evens.oro -->
```
3
4
2
6
```

A buffer `b` and a cell `n`, side by side. Each even value is stored at position `n`, and
`n` goes up. The exit gives both, as chapter 10's tuple: the table and how much of it means
anything. The first arm stops when the input is used up *or the buffer is full* (`n >= 10`),
and that second half is what proves `b[n]` is a position.

That "or the buffer is full" is worth a moment. The buffer's size is a number in the program,
and the program has to say what happens when it is reached: here, stop. A language where
lists grow by themselves never asks, and the answer it gives silently is "use more memory
until there is none". Neither is wrong. This one makes you write the limit down.

---

## 14.9 Maps

A table's positions are 0, 1, 2, … in a row. Often the thing you want to look up *by* is not
like that: a customer number, an error code, a year. A **map** is a table whose positions,
called **keys**, are whatever integers you put in.

<!-- file docs/book/code/state/map.oro -->
```go
use io

fn look(m, k) { case m[k] { 'some(v) -> v; 'none -> -1 } }

export fn main() {
    m = {
        var m := map(8)
        m[42] := 1
        m[7] := 2
        m[42] := 3
        m
    }
    io.print_int(look(m, 42))
    io.print_int(look(m, 7))
    io.print_int(look(m, 5))
    io.print_int(len(m))

    codes = map { 200 : 1, 404 : 2, 500 : 3 }
    io.print_int(look(codes, 404))
    io.print_int(look(codes, 418))
}
```

<!-- run go docs/book/code/state/map.oro -->
```
3
2
-1
2
2
-1
```

- **`var m := map(8)`** is an empty map buffer with room declared for 8 keys.
- **`m[42] := 1`** stores under the key 42. A second store under the same key replaces the
  first, so `m[42]` ends as 3 and the map has two keys, not three.
- **`map { 200 : 1, 404 : 2, 500 : 3 }`** writes a map out, as `array { … }` writes a table.
- **`m[k]`** looks a key up, and here is the difference from a table.

A table's positions are known from its length, so the compiler can *prove* `t[i]` is there.
A map's keys are whatever was stored, and nothing can know while compiling whether 418 is
among them. So `m[k]` does not give a value. It gives chapter 10's "something or nothing":
**`'some(v)`** if the key is there, **`'none`** if it is not. You cannot use the answer
without a `case`, and the `case` must say what to do about `'none`. `look` says "−1".

Chapter 13 called a table a function on its positions. A map is a function on its keys, and
each kind of function pays for "is this argument allowed?" in its own way:

| | defined on | how you know the argument is allowed |
|---|---|---|
| a function | every member of its parameter's set | the caller proves it, while compiling |
| a table | positions 0 to len − 1 | you prove `0 <= i < len`, while compiling |
| a map | the keys stored | you cannot know: the answer is `'some` or `'none`, at run time |

<!-- file docs/book/code/state/lookup.oro -->
```go
export fn lookup(k ∈ Int) (Int) {
    m = map { 1 : 10, 2 : 20, 5 : 50 }
    case m[k] { 'some(v) -> v; 'none -> 0 }
}
```

<!-- gen go docs/book/code/state/lookup.oro -->
```go
func StateLookup(v0 int) int {
    v7 := map[int]int{1: 10, 2: 20, 5: 50}
    v9, ok8 := v7[v0]
    v8 := 1
    if ok8 {
        v8 = 0
    }
    v11 := (v8 == 0)
    if v11 {
        return v9
    }
    return 0
}
```

<!-- gen js docs/book/code/state/lookup.oro -->
```js
export function stateLookup(v0) {
    const v7 = {1: 10, 2: 20, 5: 50};
    const v9 = v7[v0];
    const v8 = v9 === undefined ? 1 : 0;
    if ((v8 === 0)) {
        return v9;
    }
    return 0;
}
```

On Go a map is Go's own `map`, and the lookup is Go's two-result form, `v, ok := m[k]`, which
is the same "something or nothing" with the flag carried beside the value. JavaScript has
nothing of the kind: a missing key gives `undefined`, and the compiler turns that into the
flag.

A map is filled the way 14.3's histogram was, and it removes the histogram's restriction to
digits. Count how often each value occurs, whatever the values are:

<!-- file docs/book/code/state/tally.oro -->
```go
use io

fn look(m, k) { case m[k] { 'some(v) -> v; 'none -> 0 } }

fn tally(t) {
    var m := map(16)
    loop i = 0 {
        i >= len(t) -> m
        else -> {
            c = look(m, t[i])
            m[t[i]] := if 0 <= c && c < 1000000 { c + 1 } else { c }
            again(i + 1)
        }
    }
}

export fn main() {
    m = tally(array { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5 })
    io.print_int(len(m))
    io.print_int(look(m, 5))
    io.print_int(look(m, 7))
    ks = keys(m)
    loop j = 0 {
        j < len(ks) -> { io.print_int(ks[j]); again(j + 1) }
        else -> io.print_line("end")
    }
}
```

<!-- run go docs/book/code/state/tally.oro -->
```
7
3
0
1
2
3
4
5
6
9
end
```

Seven different values, 5 seen three times, 7 never. `look(m, t[i])` reads the count so far
(0 if the key is new), and the store writes it back one higher. The `if` around `c + 1` is
chapter 12's stated bound: a value that comes out of a map could be anything, so the program
says how high a count may go before it stops counting.

**`keys(m)`** is a table of the map's keys, **in ascending order**. That is a decision worth
knowing. Most languages hand back a map's keys in whatever order their hash table happens to
hold them, and it differs from run to run and from one machine to another: Go even shuffles
it on purpose so that nobody relies on it. A program whose output depended on that order
would not mean one thing. Sorted keys cost a sort, and the same program prints the same lines
everywhere.

On windows there is no map to borrow: the target is LLVM, and LLVM has integers and memory.
The compiler builds the map from a table and the program does not change:

<!-- run windows docs/book/code/state/tally.oro -->
```
7
3
0
1
2
3
4
5
6
9
end
```

### The capacity is a promise

The 16 in `map(16)` is the most keys the map will ever hold. Go's maps and JavaScript's
grow without limit, so why say it? Because a map that grows is an allocation nobody wrote,
somewhere in the middle of a loop, and because on a target with no map of its own the
compiler builds one from a table, and a table has a length. One declared number makes the
map the same thing everywhere.

And a promise in this language is proven. Three keys do not go into a map of two:

<!-- file docs/book/code/state/no-full.oro -->
```go
use io

export fn main() {
    var m := map(2)
    m[1] := 10
    m[2] := 20
    m[3] := 30
    io.print_int(len(m))
}
```

<!-- gen go docs/book/code/state/no-full.oro -->
```
gen: state-main: a map's capacity requires len(#vm1) < 2 at a call, which does not follow
```

(`#vm1` is the compiler's name for the map as it stands after the second store.) Before each
store the compiler must know that there is room: that the map holds fewer keys than its
capacity. It counts. An empty map has none, and a store adds at most one: at most, because
storing to a key already present adds nothing. After two stores there may be two, and two
is not less than two.

In a loop the count is the loop's:

<!-- file docs/book/code/state/distinct.oro -->
```go
export fn distinct(t ∈ Array(Int)) (Int) where len(t) <= 16 {
    var m := map(16)
    loop i = 0 {
        i >= len(t) -> len(m)
        else -> { m[t[i]] := 1; again(i + 1) }
    }
}
```

<!-- gen go docs/book/code/state/distinct.oro -->
```go
note: state-distinct: 1 of 1 integer operations bounded; 1 of 1 loop(s) proven terminating
…
func StateDistinct(v0 []int) int {
    v6 := make(map[int]int, 16)
    var v5 int
    v9 := v6
    var v10 int = 0
    var v8 int
    for ; ; v10 = (v10 + 1) {
        v11 := len(v0)
        v12 := (v10 >= v11)
        if v12 {
            v15 := len(v9)
            v8 = v15
            break
        }
        v18 := v0[v10]
        v9[v18] = 1
        continue
    }
    v5 = v8
    return v5
}
```

How many different values are in a table? Store each as a key and count the keys. The
header says the table has at most 16 values. After `i` trips the map has at most `i` keys
(the compiler finds that for itself: it is true at the start and each trip keeps it), `i`
is less than `len(t)` inside the arm, and `len(t)` is at most 16. So there is room, every
time, and the Go has no test for it. `make(map[int]int, 16)` is Go being told the size in
advance.

Let the table be one longer and the same loop is refused:

<!-- file docs/book/code/state/no-distinct.oro -->
```go
export fn distinct(t ∈ Array(Int)) (Int) where len(t) <= 17 {
    var m := map(16)
    loop i = 0 {
        i >= len(t) -> len(m)
        else -> { m[t[i]] := 1; again(i + 1) }
    }
}
```

<!-- gen go docs/book/code/state/no-distinct.oro -->
```
gen: state-distinct: a map's capacity requires len(#vm) < 16 at a call, which does not follow
```

Seventeen different values would be seventeen keys. When nothing bounds the input, ask the
map itself, and decide what the program does when it is full:

<!-- file docs/book/code/state/distinct-guard.oro -->
```go
export fn distinct(t ∈ Array(Int)) (Int) {
    var m := map(16)
    loop i = 0 {
        i >= len(t) -> len(m)
        len(m) < 16 -> { m[t[i]] := 1; again(i + 1) }
        else -> -1
    }
}
```

<!-- gen go docs/book/code/state/distinct-guard.oro -->
```go
note: state-distinct: 1 of 1 integer operations bounded; 1 of 1 loop(s) proven terminating
```

`len(m) < 16` is the fact the store needs, tested where it is needed, and the `else` arm is
the program's answer to "too many": here −1. That is not a way round the rule. It is what
the rule is for: the question "what if there are more than I planned for?" has to be
answered by somebody, and the only choice is whether it is answered in the source or
discovered in production.

`tally` above needed neither a header nor a test, because it is called on a table written
out in the program, eleven values long, and the compiler can count to eleven.

A map's values may be records, read with chapter 11's dot after the `case` has found one:

<!-- file docs/book/code/state/mapped.oro -->
```go
use io

set Stock = record { count ∈ 0..1000, price ∈ 0..100 }

fn value(m, k) { case m[k] { 'some(s) -> s.count * s.price; 'none -> -1 } }

export fn main() {
    m = {
        var m := map(4)
        m[7] := record { count : 3, price : 20 }
        m[9] := record { count : 10, price : 5 }
        m[7] := record { count : 4, price : 20 }
        m
    }
    io.print_int(value(m, 7))
    io.print_int(value(m, 9))
    io.print_int(value(m, 8))
}
```

<!-- run go docs/book/code/state/mapped.oro -->
```
80
50
-1
```

<!-- run windows docs/book/code/state/mapped.oro -->
```
80
50
-1
```

Item 7 was written twice, and the second record replaced the first: 4 at 20 is 80. Go keeps the
records as `struct`s in its own `map`; Windows, which has no map of its own, keeps each record's
two parts side by side in the slots of a table the program's own code manages.

What maps do not do today, plainly:

- **Keys are integers.** A map needs `==` on its keys, and chapter 9 said strings have none
  yet. A program that counts words keeps them in a sorted table instead.
- **A value looked up has no known range**, which is why `tally` needs its guard.

---

## 14.10 What is not here

- **A function does not capture a cell or a buffer.** A `fn` written inside a block cannot
  write the block's `var`. State is passed: a buffer is lent with `linear`, and a cell with
  `inout`. (One exception is built, for Windows: a window procedure, a function Windows calls
  back with each message, keeps its state in a cell of the block around it. Chapter 18.)
- **A map is read once it is built.** `m[k]` on a `var m` that is still being filled is
  refused; build it in a block and read what the block gives back, as above. The refusal is in
  the compiler's internal words today (the book's finding 100).
- **No growing.** A buffer's length is fixed where it is made (14.8). Appending to a table
  that grows is not in the language.
- **One loop fills a buffer, or functions are lent it.** Today a buffer written by a loop
  that answers `()` cannot be written again after that loop (README item 73). `sort` and
  `sieve` show the shape that works: the loop gives the buffer as its answer, or the writing
  is done by functions that borrow it.
- **No global variables.** There is nowhere outside a block to put a `var`. Everything a
  function can change is in its parameter list.

That last one is the rule that makes a function's header honest. In a language with global
state, a call `f(x)` may change anything, and you have to read `f`, and everything `f` calls,
to know what. Here a function changes what it is lent and nothing else, and what it is lent
says `linear`.

---

## 14.11 What other languages do

| | a changeable variable | two names for one changeable table | may a function alter its caller's table? | when does a table stop changing? |
|---|---|---|---|---|
| Oroboros | `var`, and it is a notation for new names | refused | only one lent `linear` | when its scope ends |
| C, Go | every variable | allowed | yes, through a pointer or slice | never |
| Java, Python, JavaScript | every variable | allowed, and the default | yes, always | never (`final`/`const` protect the name, not the contents) |
| Rust | `let mut` | refused while one can write | only through `&mut`, which is exclusive | when no `mut` name is left |
| Haskell | none; state is threaded or put in `ST` | cannot arise | no | at once |
| Clojure | none; "transients" for building | refused for transients | no | when made `persistent!` |

Rust is the nearest relative for the fourth chapter running. Its rule is "any number of
readers, or exactly one writer, never both", checked by the *borrow checker*; `&mut` is this
chapter's `linear`, and the thing it refuses is 14.5's alias. Rust tracks this for every
value through the whole program, with lifetimes. Here the rule is narrower and needs no
annotations beyond the one word: a buffer has one name, in one scope, and then it is frozen.

Clojure's transients are the closest match for 14.4: build in place under one owner, then
freeze, and the freeze is free.

The idea that a buffer may be updated in place *because* nothing else can see it is from the
language Clean (1987), under the name *uniqueness*, and the theory behind the word `linear`
is the logic chapter 5 met: a thing that must be used exactly once.

---

## What to remember

- **`x = e` says what `x` is. `var x := e` makes a cell, and `x := e` writes it.** Only a
  `var` may be written.
- **A cell is a notation.** Each write starts a new name and later uses mean the latest. A
  value taken from a cell, `y = x`, stays what it was.
- **A cell written in a loop is a loop variable.** Use a cell when a value changes on some
  trips and not others.
- **A `var` holding a table is a buffer**: `var b := array { 0 : _ ∈ 0..n-1 }`, then
  `b[i] := v`. Stores go into one block of memory, in place.
- **A rule gathers, a buffer scatters.** Use a buffer when the data decides where to write.
- **A store's position must be inside the buffer**, proven as a read's is. One past the end
  is refused.
- **When its scope ends, a buffer is frozen into a table.** Freezing is free and final. A
  table, a parameter included, cannot be stored into. To change one, copy it into a new
  buffer.
- **A buffer has exactly one name while it can be written.** `d = b` is refused. That is why
  a value you hold never changes.
- **A function that writes a buffer takes it `linear`**: lent for the call, written in place,
  and returned. One buffer is not lent twice in one call.
- **For an unknown number of results, keep a buffer and a count**, and say what happens when
  it is full.
- **A map is a table with integer keys you choose.** `m[k] := v` stores, `m[k]` gives
  `'some(v)` or `'none`, `keys(m)` is the keys in ascending order.
- **`map(c)` holds at most `c` keys, and each store is proven to have room**: from a bound on
  the input, or under `len(m) < c`.

Next: [chapter 15](15-failures.md), Failures. What a function says when it cannot do what it was asked.
