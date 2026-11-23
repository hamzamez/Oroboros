# 12. Records

Chapter 11 put several values side by side in a tuple, `(3, 4)`, and took them apart by position:
`(w, h) = size`. That works while there are two parts and everyone remembers which is which. It
stops working the day a size grows a third part, or a function takes a size and a position that
are both `(Int, Int)`, and someone swaps them. Nothing in `(3, 4)` says which number is the width.

So the question of this chapter is: **how do the parts of a value get names?**

The answer is three sentences.

> **A record is a tuple whose positions are names**: `record { x : 2, y : 3 }`. You read a part
> with a dot, `p.x`.
>
> **A record is a function on its labels.** Everything else follows from that: each label once,
> every label given, the order of writing irrelevant, and `with` to make a changed copy.
>
> **Like a tuple, a record does not exist when the program runs.** Each part becomes a value of
> its own, and the names are gone.

Along the way: the four laws that every "update one field" operation in every language obeys or
breaks, why a record has no `==`, what a "zero" value is and why `Bool` has none, and why two
record sets written in different orders are the same set.

The examples are in [code/records/](code/records/). `go run docs/book/check.go docs/book/12-records.md`
re-runs every block. Checked 2026-11-20.

---

## 12.1 A tuple with names

A point on a grid has an `x` and a `y`:

<!-- file docs/book/code/records/point.oro -->
```go
use io

set Point = record { x ∈ Int, y ∈ Int }

fn manhattan(p ∈ Point, q ∈ Point) (Int) {
    dx = if p.x > q.x { p.x - q.x } else { q.x - p.x }
    dy = if p.y > q.y { p.y - q.y } else { q.y - p.y }
    dx + dy
}

export fn main() {
    home = record { x : 2, y : 3 }
    shop = record { y : 7, x : -1 }
    io.print_int(home.x)
    io.print_int(shop.y)
    io.print_int(manhattan(home, shop))
}
```

<!-- run go docs/book/code/records/point.oro -->
```
2
7
7
```

Three new things, all in the first lines.

**`record { x ∈ Int, y ∈ Int }` is a set**: every value with an `x` in `Int` and a `y` in `Int`.
`set Point = …` gives it a name, as `set` has given names to variants since chapter 11.

**`record { x : 2, y : 3 }` is a value**: an `x` that is 2 and a `y` that is 3. The difference
from the set is one character after each label: `∈` says "is a member of", as everywhere in this
language, and `:` says "is".

**`p.x` reads a part.** The distance walks 3 across and 4 up: 7.

`shop` was written `y` first. That made no difference to anything, and §12.4 says why.

---

## 12.2 What a record is

A tuple `(2, 3)` is a function from positions to values: position 0 gives 2, position 1 gives 3.
A record is the same thing with names for positions: a **function from labels to values**. The
label `x` gives 2, the label `y` gives 3. And `p.x` is that function applied to `x`.

That one sentence decides every rule of this chapter, and each is a refusal you can see.

The token after a label says which kind of thing is being written, so a former that mixes them is
neither:

<!-- file docs/book/code/records/no-mixed.oro.txt -->
```go
use io
export fn main() {
    p = record { x : 2, y ∈ Int }
    io.print_int(p.x)
}
```

<!-- gen go docs/book/code/records/no-mixed.oro.txt -->
```
gen: docs/book/code/records/no-mixed.oro.txt: line 3: a record former mixes a set's `y ∈ S` and a value's `x : e` (ADR 0128, X9): `record { l ∈ S }` is a set, a family of claims, and `record { l : e }` a value, its graph
```

"Its graph" is the mathematician's word for a function written out as pairs: `x : 2, y : 3` is
exactly the pairs (x, 2) and (y, 3).

---

## 12.3 Each label once, every label given

**A function has one value at each point**, so a label may not appear twice:

<!-- file docs/book/code/records/no-twice.oro.txt -->
```go
use io
export fn main() {
    p = record { x : 2, x : 3 }
    io.print_int(p.x)
}
```

<!-- gen go docs/book/code/records/no-twice.oro.txt -->
```
gen: docs/book/code/records/no-twice.oro.txt: line 3: the label x appears twice in one record (data.md §4.2, X1): a function has one value at each point
```

**A function is defined at every point of its domain.** A value claimed to be a `Point` must
give every label a `Point` has:

<!-- file docs/book/code/records/no-missing.oro -->
```go
use io
set Point = record { x ∈ Int, y ∈ Int }
export fn main() {
    p ∈ Point = record { x : 2 }
    io.print_int(p.x)
}
```

<!-- build go docs/book/code/records/no-missing.oro -->
```
build: a value of record { x ∈ Int, y ∈ Int } gives no y (data.md §4.2, X2): a record is total on its labels; a default is `zero(T) with { … }`
```

`p ∈ Point = …` is chapter 7's claim on a binding, used on a record. Without the claim,
`record { x : 2 }` would be a perfectly good record: a record of one label, in the set
`record { x ∈ Int }`. It is the claim that says which labels there must be.

**Nor may it give a label the set does not have:**

<!-- file docs/book/code/records/no-extra.oro -->
```go
use io
set Point = record { x ∈ Int, y ∈ Int }
export fn main() {
    p ∈ Point = record { x : 2, y : 3, z : 4 }
    io.print_int(p.x)
}
```

<!-- build go docs/book/code/records/no-extra.oro -->
```
build: a value of record { x ∈ Int, y ∈ Int } gives z, which is not among its labels (data.md §4.2, X3)
```

**And a function is not applied outside its domain**, so reading a label a record does not have
is refused, where it is written:

<!-- file docs/book/code/records/no-label.oro -->
```go
use io
export fn main() {
    p = record { x : 2, y : 3 }
    io.print_int(p.z)
}
```

<!-- build go docs/book/code/records/no-label.oro -->
```
build: projection of z from a record whose labels are x, y (data.md §4.2, X3): application outside its domain
```

In a language where a record is a dictionary underneath, `p.z` is a question asked at run time,
and the answer is `undefined`, `None` or a crash. Here the labels of every record are known
before the program runs, so the question has been answered by then.

A claim also reaches inside. Each part keeps its own set, and a part outside it is refused naming
the part:

<!-- file docs/book/code/records/no-claim-part.oro -->
```go
use io
set Pen = record { ink ∈ 0..255, width ∈ 1..9 }
export fn main() {
    p ∈ Pen = record { ink : 300, width : 2 }
    io.print_int(p.width)
}
```

<!-- build go docs/book/code/records/no-claim-part.oro -->
```
build: main: the claim ink ∈ 0..255 is false: 0 <= 300 && 300 <= 255
  A binder's claim, `b ∈ S = e`, is an obligation: proven of e, or the program is refused
  (ADR 0049), whoever reads b.
```

"Whoever reads b": the program reads only `p.width`, and the claim on `p.ink` is refused anyway.
A claim is about the value, not about the parts someone happens to use.

That is the rule for a set of records, in one line: a record is in `record { ink ∈ 0..255, … }`
when each part is in its own set. A claim on a record is a claim on each part.

---

## 12.4 The order is not part of it

A function has no order on its domain: "x gives 2 and y gives 3" is the same function as "y gives
3 and x gives 2". So two record sets with the same labels and the same sets at each label are
**one set**, whatever order they were written in:

<!-- file docs/book/code/records/order.oro -->
```go
use io

set Size = record { width ∈ Int, height ∈ Int }
set Size2 = record { height ∈ Int, width ∈ Int }

fn area(s ∈ Size) (Int) { s.width * s.height }

export fn main() {
    a ∈ Size2 = record { height : 2, width : 5 }
    io.print_int(area(a))
    io.print_int(area(record { width : 3, height : 3 }))
}
```

<!-- run go docs/book/code/records/order.oro -->
```
10
9
```

`a` was claimed to be a `Size2` and handed to a function that asks for a `Size`. That is not a
conversion. They are the same set, under two names.

**Nor is a set's name part of it.** `set Size = …` is an abbreviation, as every `set` is: the
program means exactly what it would mean with the record set written out in place. Two
programmers who each write `record { width ∈ Int, height ∈ Int }` have written the same set, and
neither has to import the other's.

That is called *structural* typing, and its opposite, *nominal* typing, is what C, Go and Java
do: there, two `struct`s with the same fields and different names are different types. §12.11
says what each costs.

A function that never names a set for its argument asks for exactly what it reads:

<!-- file docs/book/code/records/structural.oro -->
```go
use io

fn area(b) { b.width * b.height }

export fn main() {
    io.print_int(area(record { width : 3, height : 4 }))
    io.print_int(area(record { height : 10, width : 2, colour : 7 }))
}
```

<!-- run go docs/book/code/records/structural.oro -->
```
12
20
```

The second record has a `colour`, which `area` never reads. Nothing asked for it, so nothing
minds it. (Chapter 3 said why this costs nothing: `area` is unfolded at each call, and each copy
reads the record it was given.)

---

## 12.5 A changed copy: `with`

A record, like every value in this book, never changes. To get a point one step up, you make a new
point that is the old one except at `y`:

<!-- file docs/book/code/records/with.oro -->
```go
use io

set Point = record { x ∈ Int, y ∈ Int }
set Box = record { corner ∈ Point, width ∈ Int, height ∈ Int }

export fn main() {
    a = record { x : 1, y : 2 }
    b = a with { y : 20 }
    io.print_int(a.y)
    io.print_int(b.y)
    io.print_int(b.x)

    box ∈ Box = record { corner : a, width : 10, height : 5 }
    moved = box with { corner.x : 100, width : 11 }
    io.print_int(moved.corner.x)
    io.print_int(moved.corner.y)
    io.print_int(moved.width)
    io.print_int(box.corner.x)
}
```

<!-- run go docs/book/code/records/with.oro -->
```
2
20
1
100
2
11
1
```

**`a with { y : 20 }` is `a`, except at `y`.** `a.y` is still 2 afterwards: `with` made a new
record and left the old one alone. `b.x` came from `a`.

**A record may hold a record**, as `box` holds its `corner`, and `moved.corner.x` reads through
both. And `with` follows the same path: `corner.x : 100` changes `x` inside `corner` inside
`box`, and leaves `corner.y` and `height` as they were. Several changes go in one `with`, as
`width : 11` does here.

`box.corner.x` is still 1. Nothing was overwritten, anywhere.

Two changes may not touch the same part, because then their order would matter, and a `with` has
no order:

<!-- file docs/book/code/records/no-prefix.oro.txt -->
```go
use io
set Point = record { x ∈ Int, y ∈ Int }
set Box = record { corner ∈ Point, width ∈ Int }
export fn main() {
    b ∈ Box = record { corner : record { x : 1, y : 2 }, width : 3 }
    c = b with { corner : record { x : 0, y : 0 }, corner.x : 5 }
    io.print_int(c.corner.x)
}
```

<!-- gen go docs/book/code/records/no-prefix.oro.txt -->
```
gen: docs/book/code/records/no-prefix.oro.txt: line 6: a `with`'s paths corner and corner.x are not prefix-free (data.md §4.3, X10): one writes a part of the record the other writes, which is put–put; write one update
```

Should `c.corner.x` be 0 or 5? Either answer is defensible, which is the reason to refuse the
question. "Prefix-free" means no path is the beginning of another: `corner` and `corner.x` are
not, `corner.x` and `corner.y` are.

---

## 12.6 Four laws

Every language with records has some way to "update one field", and every one of them either obeys
four laws or has a bug. They are named after what they compose: *get* is reading a part, *put* is
`with`.

| law | in symbols | in words |
|---|---|---|
| put–get | `(r with { l : e }).l` is `e` | you read back what you wrote |
| put–get, elsewhere | `(r with { l : e }).m` is `r.m` | the other parts are untouched |
| get–put | `r with { l : r.l }` is `r` | writing what is there changes nothing |
| put–put | `(r with { l : e }) with { l : f }` is `r with { l : f }` | the last write wins |

Here they are, run:

<!-- file docs/book/code/records/laws.oro -->
```go
use io

fn yes(b ∈ Bool) { if b { "yes" } else { "no" } }

export fn main() {
    r = record { x : 1, y : 2 }
    io.print_line(yes((r with { x : 5 }).x == 5))
    io.print_line(yes((r with { x : 5 }).y == r.y))
    s = r with { x : r.x }
    io.print_line(yes(s.x == r.x && s.y == r.y))
    t = (r with { x : 5 }) with { x : 9 }
    io.print_line(yes(t.x == 9 && t.y == 2))
}
```

<!-- run go docs/book/code/records/laws.oro -->
```
yes
yes
yes
yes
```

They look too obvious to state. Their use is that they are exactly what a compiler may assume
when it rewrites a program: by the first, `(r with { x : 5 }).x` may become `5` without building
the record at all, and this compiler does that. Programming-language researchers call a get and a
put that obey them a *very well-behaved lens* (Foster et al., 2007), and the name is earned: a
`set` that forgets to copy, or a getter with a side effect, breaks one of them.

---

## 12.7 The zero

Often you want a record that is "nothing yet" and then set a few parts. `zero(T)` is the record of
`T` whose every part is its set's zero:

<!-- file docs/book/code/records/zero.oro -->
```go
use io

set Point = record { x ∈ Int, y ∈ Int }
set Pen = record { at ∈ Point, ink ∈ 0..255, width ∈ 1..9 }

export fn main() {
    p = zero(Point)
    io.print_int(p.x + p.y)
    pen = zero(Point) with { y : 4 }
    io.print_int(pen.y)
}
```

<!-- run go docs/book/code/records/zero.oro -->
```
0
4
```

`zero(Point) with { y : 4 }` is the idiom: everything zero, except what you say. It is C's
`Point p = {0}; p.y = 4;`, written as one value.

But a zero must *be* in the set, and not every set has one:

<!-- file docs/book/code/records/no-zero-range.oro -->
```go
use io

set Pen = record { ink ∈ 0..255, width ∈ 1..9 }

export fn main() {
    pen = zero(Pen)
    io.print_int(pen.ink)
}
```

<!-- build go docs/book/code/records/no-zero-range.oro -->
```
build: zero(record { ink ∈ 0..255, width ∈ 1..9 }): its component width ∈ 1..9 has no zero (data.md §4.6, X11): 0 is not in 1..9
```

A pen of width 0 is not a `Pen`, so there is no zero `Pen`. In C, `Pen p = {0};` would compile and
give you one anyway.

A `String` has no zero, and neither has a `Bool`:

<!-- file docs/book/code/records/no-zero.oro -->
```go
use io
set Named = record { name ∈ String, age ∈ 0..150 }
export fn main() {
    n = zero(Named)
    io.print_int(n.age)
}
```

<!-- build go docs/book/code/records/no-zero.oro -->
```
build: zero(record { age ∈ 0..150, name ∈ String }): its component name ∈ String has no zero (data.md §4.6, X11): only an integer set holding 0, an Option and a record have one
```

<!-- file docs/book/code/records/no-zero-bool.oro -->
```go
use io
set Lamp = record { on ∈ Bool, watts ∈ 0..100 }
export fn main() {
    l = zero(Lamp)
    io.print_int(l.watts)
}
```

<!-- build go docs/book/code/records/no-zero-bool.oro -->
```
build: zero(record { on ∈ Bool, watts ∈ 0..100 }): its component on ∈ Bool has no zero (data.md §4.6, X11): only an integer set holding 0, an Option and a record have one
```

Why not `false`, and `""`? Because a zero is a choice somebody made, and the language makes none it
can avoid. Is a lamp's "nothing yet" off? Is an unknown name the empty string, or is it missing?
An integer has a 0 and an `Option` a `'none`, because those *mean* nothing. `false` and `""` are
real answers, and a default that is a real answer is how a program comes to believe a lamp is off
when nobody looked. Say which you mean: `zero(Lamp)` is refused, and
`record { on : false, watts : 0 }` is three more words.

---

## 12.8 Records with tuples, tuples with records

A record goes wherever a tuple goes, and holds what a tuple holds. A record with tuple parts:

<!-- file docs/book/code/records/inside.oro -->
```go
use io

set Line = record { from ∈ Int × Int, to ∈ Int × Int, label ∈ String }

fn length2(l ∈ Line) (Int) {
    (x0, y0) = l.from
    (x1, y1) = l.to
    (x1 - x0) * (x1 - x0) + (y1 - y0) * (y1 - y0)
}

export fn main() {
    l = record { from : (0, 0), to : (3, 4), label : "diagonal" }
    io.print_int(length2(l))
    io.print_line(l.label)
}
```

<!-- run go docs/book/code/records/inside.oro -->
```
25
diagonal
```

`Int × Int` is the set of pairs, chapter 11's product. `l.from` is a pair, and chapter 11's pattern
takes it apart.

And a function that returns two records returns a tuple of them:

<!-- file docs/book/code/records/pair.oro -->
```go
use io

set Point = record { x ∈ -1000..1000, y ∈ -1000..1000 }

fn bounds(a ∈ Point, b ∈ Point) (Point, Point) {
    lo = record { x : if a.x < b.x { a.x } else { b.x }, y : if a.y < b.y { a.y } else { b.y } }
    hi = record { x : if a.x > b.x { a.x } else { b.x }, y : if a.y > b.y { a.y } else { b.y } }
    (lo, hi)
}

export fn main() {
    (lo, hi) = bounds(record { x : 5, y : -2 }, record { x : 1, y : 7 })
    io.print_int(lo.x)
    io.print_int(lo.y)
    io.print_int(hi.x)
    io.print_int(hi.y)
}
```

<!-- run go docs/book/code/records/pair.oro -->
```
1
-2
5
7
```

The smallest box around two points: its low corner takes the smaller of each coordinate, its high
corner the larger.

---

## 12.9 Where did the record go?

Here is what a host receives for two functions over points:

<!-- file docs/book/code/records/header.oro -->
```go
set Point = record { x ∈ -1000..1000, y ∈ -1000..1000 }

export fn mirror(p ∈ Point) (Point) { record { x : p.y, y : p.x } }

export fn norm1(p ∈ Point) (0..2000) {
    ax = if p.x < 0 { 0 - p.x } else { p.x }
    ay = if p.y < 0 { 0 - p.y } else { p.y }
    ax + ay
}
```

<!-- gen go docs/book/code/records/header.oro -->
```go
func RecordsMirror(v0 int, v1 int) (int, int) {
    return v1, v0
}
…
```

<!-- gen js docs/book/code/records/header.oro -->
```js
export function recordsMirror(v0, v1) {
    return {f0: v1, f1: v0};
}

export function recordsNorm1(v0, v1) {
    return (((v0 < 0) ? (0 - v0) : v0) + ((v1 < 0) ? (0 - v1) : v1));
}
```

**The record is gone.** A `Point` parameter became two integer parameters, `v0` for `x` and `v1`
for `y`, and a `Point` result became two results on Go. The labels decided which position each
part takes (alphabetical order, so the same on every host, whatever order the program wrote), and
then the labels were no longer needed.

This is chapter 11's tuple story again. A tuple did not exist at run time either. A record is a
tuple with names, and the names are for the program, not for the machine. `mirror` builds no
object and reads no field: it returns its two arguments the other way round, which is what
mirroring a point is.

(Where a record *is* stored, in a table of records or in a map, it is laid out as its parts side
by side. Chapters 14 and 15 show it.)

---

## 12.10 What a record cannot do

**A record has no `==`:**

<!-- file docs/book/code/records/no-equal.oro -->
```go
use io
export fn main() {
    a = record { x : 1, y : 2 }
    b = record { x : 1, y : 2 }
    io.print_line(if a == b { "same" } else { "different" })
}
```

<!-- build go docs/book/code/records/no-equal.oro -->
```
build: main: in argument 1 of io.print_line: in a condition: in argument 1 of `==`: (1, 2) is a tuple or a record, several values, and `==` is the language's integer equality only: compare the parts, a.x == b.x && a.y == b.y
```

Chapter 5 said why `==` is only for integers: it is the one equality every target agrees on. A
record of integers could have one, comparing part by part. But a record of `F64`s could not
(`NaN` is not equal to itself), nor a record holding a string. And an equality that works for
some records and not others, depending on what is inside, is a rule you would have to look up.
So: compare the parts you mean.

The refusal says `(1, 2)`: by the time `==` is checked, the record has already become its parts,
as §12.9 showed.

**Two branches must give records of the same labels:**

<!-- file docs/book/code/records/no-arms.oro -->
```go
fn pick(c ∈ Bool) { if c { record { x : 1 } } else { record { y : 2 } } }
export fn first(c ∈ Bool) (Int) { pick(c).x }
```

<!-- gen go docs/book/code/records/no-arms.oro -->
```
gen: the projection x reads a term whose record labels are not known where it is a value (ADR 0128, data.md §4.7): if c { record { x : 1 } } else { record { y : 2 } }
```

Which labels does `pick(c)` have? It depends on `c`, which is known only when the program runs, and
a record's labels are known before. If `c` were a constant, chapter 5's rule would keep one branch
and the question would go away.

**A record cannot contain itself:**

<!-- file docs/book/code/records/no-self.oro.txt -->
```go
use io
set Node = record { value ∈ Int, next ∈ Node }
export fn main() { io.print_int(1) }
```

<!-- gen go docs/book/code/records/no-self.oro.txt -->
```
gen: docs/book/code/records/no-self.oro.txt: line 2: set Node mentions Node itself, so the set contains itself. That is a recursive type, which is refused; recursive data is a flat table plus indices (type-algebra.md §3.1)
```

Chapter 11 refused a variant that contains itself, for the same reason: a `Node` would be infinitely
large. A linked list, a tree, a graph: chapter 14 builds each from a table and positions in it.

**A variant cannot carry a record, yet:**

<!-- file docs/book/code/records/no-payload.oro.txt -->
```go
use io

set Rect = record { width ∈ Int, height ∈ Int }
set Shape = 'circle(Int) ∪ 'rect(Rect) ∪ 'point

fn area(s) {
    case s {
        'circle(r) -> 3 * r * r
        'rect(b) -> b.width * b.height
        'point -> 0
    }
}

export fn main() {
    io.print_int(area('rect(record { width : 3, height : 4 })))
    io.print_int(area('circle(2)))
    io.print_int(area('point))
}
```

<!-- gen go docs/book/code/records/no-payload.oro.txt -->
```
gen: docs/book/code/records/no-payload.oro.txt: set Shape: 'rect(record { height ∈ Int, width ∈ Int }) carries a record, and a variant's value is its tag and one slot per payload type (ADR 0042): a payload of several parts is not built. Carry one value, as 'rect(Int), and keep the parts where the value can name them
```

Chapter 11 showed why: at the program's edge a variant is its tag and one slot for each payload
set, and a record of two parts would need its slots worked out, which nothing does yet. A tuple
payload, `'rect(Int × Int)`, is refused the same way. Chapter 14 carries a rectangle's number and
keeps its width and height in tables.

**Labels are names, never values.** There is no `p[l]` with `l` computed: a label decided while
the program runs would be a dictionary, and chapter 15's maps are that.

---

## 12.11 What other languages do

| | a record is | a field the type lacks | update | two types, same fields |
|---|---|---|---|---|
| C | a `struct`, laid out in declaration order | refused | assignment, in place | different types |
| Go | a `struct` | refused | assignment, in place; a copy if passed by value | different types |
| Java | a `record` class (since 16) or a class | refused | none for `record`: a new one, by hand | different types |
| JavaScript | an object, a dictionary | `undefined`, at run time | in place, or `{ ...r, x: 5 }` | the same, nobody checks |
| TypeScript | an object, typed by its shape | refused | `{ ...r, x: 5 }` | the same type |
| OCaml, F# | a record | refused | `{ r with x = 5 }` | different types, and the field name picks one |
| Elm | a record | refused | `{ r \| x = 5 }` | the same type |
| TLA+ | a function on its labels | refused | `[r EXCEPT !.x = 5]` | the same set |
| here | a function on its labels, erased into its parts | refused, before it runs | `r with { x : 5 }` | the same set |

The shared word is *with*: OCaml and F# have the same `{ r with x = 5 }`, and TLA+'s `EXCEPT` is
the same operation, written as the mathematics writes it.

**Nominal or structural.** C, Go, Java, OCaml and F# make two record types with the same fields
different, by name. That catches a `Celsius` passed where a `Fahrenheit` was meant, if you wrote
both as types. TypeScript, Elm and this language make them the same, which means a program never
converts between two identical definitions, and a function asks only for what it reads (§12.4).
The `Celsius` mistake is caught here another way: by a claim on the range, chapter 7, or by keeping
different quantities in different labels.

**In place, or a copy.** C and Go update a field in place. That is fast, and it is also how one part
of a program changes a value another part is still looking at. Here `with` always means a new
value, and the compiler is the one who decides, by the laws of §12.6, when nothing needs copying.
Chapter 15 shows a record that *is* changed in place, in a `var`, where the language can see
nobody else is looking.

---

## What to remember

- **`record { x ∈ Int, y ∈ Int }` is a set, and `record { x : 2, y : 3 }` a value.** `∈` is
  membership, `:` is "is". `p.x` reads a part.
- **A record is a function on its labels.** So: each label once, every label of its set given, no
  label the set lacks, and reading a missing label is refused before the program runs.
- **The order labels are written in is not part of anything**, and neither is a set's name: two
  record sets with the same labels and sets are one set.
- **`r with { l : e }` is a new record**, `r` except at `l`. Paths reach inside, `corner.x : 100`,
  and no two may touch the same part. Four laws hold, and the compiler relies on them.
- **`zero(T)` is every part's zero**, and exists only where every part has one: an integer set with
  0 in it, an `Option`, or a record. `Bool` and `String` have none, on purpose.
- **A claim on a record is a claim on each part**, refused naming the part.
- **A record goes where a tuple goes**, and at run time it is its parts, in the same positions on
  every host.
- **No `==` on records**, no records whose labels depend on a run-time choice, no record that
  contains itself, and no record carried by a variant yet.

Next: [chapter 13](13-loop.md), `loop`. Doing something more than once, and knowing that it stops.
