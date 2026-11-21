# 0. A tour

This chapter is the whole language at walking pace, for someone who already programs in Go, Python,
JavaScript, Java, C or Rust. Most of what you will see looks like what you know: braces, `fn`,
`f(x)`, `t[i]`, infix arithmetic, `//` comments, `if` and `else`. The chapter's job is to point at
the places where it only *looks* that way, and at the things you will reach for that are not here.

Each section says what is **like** the languages you know and what is **unlike** them, and shows the
unlike part failing before it shows it working, because the compiler's refusals are most of what is
unfamiliar. Every function is shown three ways: its source, what a program using it prints, and
what the compiler made of it.

**Everything printed below is real.** Each source is a file in [code/tour/](code/tour/); each
emitted function, each refusal and each program's output is what the compiler gave, and
`go run docs/book/check.go docs/book/00-tour.md` re-runs all of it and compares. A line `…` stands
for lines left out. Checked 2026-11-21.

Only what is **decided and built** is here. What is specified and not built is named once, in
§0.14, so that nothing in between is a promise.

---

## 0.1 Hello

<!-- file docs/book/code/tour/hello.oro -->
```go
use io

export fn main() {
    io.print_line("hello, " ++ "world")
}
```

```bash
go run ./cmd/build -target=go -o hello.exe docs/book/code/tour/hello.oro
```

<!-- run go docs/book/code/tour/hello.oro -->
```
hello, world
```

The same file builds for the browser's engine with `-target=js`, and prints the same line under
Node:

<!-- run js docs/book/code/tour/hello.oro -->
```
hello, world
```

**Like:** a file of declarations; `fn`; a call is `f(x)`; a member of a module is `io.print_line`;
strings are in double quotes; a comment is `//` to the end of the line; a newline ends a statement,
and there are no semicolons to forget.

**Unlike:**

- **There is no runtime.** No prelude, no support library, no garbage collector of ours. The
  emitted Go is a `package main` holding your functions, and the binary is what `go build` makes of
  it. The language is a **parasite** ([ADR 0001](../decisions/0001-parasite-model.md)): it lives on
  a host's ecosystem and brings nothing of its own.
- **A file is a module**, named by its path. It has no `package` line and no header.
- **`export` marks what the host can call.** `export fn main()` is the program's entry.
- **`++` joins strings**, and `+` is only ever arithmetic.

## 0.2 One function, four hosts

<!-- file docs/book/code/tour/sum.oro -->
```go
use io

export fn sum_to(n ∈ 0..65536) (Int) {
    loop acc = 0, i = 0 {
        i < n -> again(acc + i, i + 1)
        else  -> acc
    }
}

export fn main() {
    io.print_int(sum_to(10))
    io.print_int(sum_to(65536))
}
```

Read the header aloud: *sum_to takes an n in the set 0 to 65536 and gives an integer.* The body is
one loop with two variables. `main` uses it twice:

<!-- run go docs/book/code/tour/sum.oro -->
```
45
2147450880
```

Now compile it without building, to see what a host receives:

```bash
go run ./cmd/gen -name tour docs/book/code/tour/sum.oro go sum.go
```

<!-- gen go docs/book/code/tour/sum.oro -->
```go
note: tour-sum-to: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
…
func TourSumTo(v0 int) int {
    var v12 int = 0
    var v13 int = 0
    for ; ; v13 = (v13 + 1) {
        v14 := (v13 < v0)
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

That is a Go `for` loop over two `int`s, which is what you would have written. The same source on
the other three targets:

<!-- gen js docs/book/code/tour/sum.oro -->
```javascript
export function tourSumTo(v0) {
    let v12 = 0;
    let v13 = 0;
    for (;; v13 = (v13 + 1)) {
        if ((v13 < v0)) {
            v12 = (v12 + v13);
            continue;
        }
        return v12;
    }
}
```

<!-- gen java docs/book/code/tour/sum.oro -->
```java
    public static long tourSumTo(int v0) {
        long v12 = 0;
        int v13 = 0;
```

<!-- gen windows docs/book/code/tour/sum.oro -->
```llvm
define i64 @tour_sum_to(i64 range(i64 0, 65537) %v0) #0 {
…
  %v17 = add nuw nsw i64 %v12, %v13
  %v19 = add nuw nsw i64 %v13, 1
```

**Like:** the output. Each host gets its own idiom: Go's three-clause `for`, JavaScript's `let`,
Java's primitives, and on Windows LLVM IR that links against `kernel32` alone.

**Unlike**, and these three are the rest of the chapter:

- **`n ∈ 0..65536` is the parameter's type, and it is a set.** Java's signature shows it being
  used: the parameter is an `int` and the sum a `long`, chosen from what the ranges prove, not from
  anything the source said about widths (§0.4).
- **The compiler printed a proof count.** *2 of 2 integer operations bounded* means both additions
  were proven to stay inside every target's machine word, so each is one machine instruction and
  nothing is checked at run time. LLVM's `nuw nsw` flags are that proof, handed to the optimizer
  (§0.5).
- **There is no `for`.** `loop` with `again` is the only iteration, and it was *proven
  terminating* (§0.7).

The targets are Go, Windows, Android and the browser
([ADR 0039](../decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)); Java is how
Android is reached, and JavaScript the browser.

## 0.3 A block's value is its last expression

<!-- file docs/book/code/tour/blocks.oro -->
```go
use io

export fn hyp2(a ∈ 0..1000, b ∈ 0..1000) (Int) {
    aa = a * a
    bb = b * b
    aa + bb
}

export fn grade(score ∈ 0..100) (String) {
    if score >= 90 { "A" } else if score >= 80 { "B" } else { "C" }
}

export fn sign(n ∈ -1000..1000) (-1..1) {
    case {
        n < 0  -> -1
        n == 0 -> 0
        else   -> 1
    }
}

export fn main() {
    io.print_int(hyp2(3, 4))
    io.print_line(grade(95) ++ grade(85) ++ grade(12))
    io.print_int(sign(-7))
    io.print_int(sign(0))
}
```

<!-- run go docs/book/code/tour/blocks.oro -->
```
25
ABC
-1
0
```

<!-- gen go docs/book/code/tour/blocks.oro -->
```go
func TourGrade(v0 int) string {
    v10 := (v0 >= 90)
    if v10 {
        return "A"
    }
    v15 := (v0 >= 80)
    if v15 {
        return "B"
    }
    return "C"
}

func TourHyp2(v0 int, v1 int) int {
    v23 := (v0 * v0)
    v24 := (v1 * v1)
    v25 := (v23 + v24)
    return v25
}
```

**Like:** braces delimit a block; `if`/`else if`/`else`; `==`, `!=`, `<`, `<=`, `&&`, `||`, `!` mean
what they mean in C; arithmetic has the usual precedence.

**Unlike:**

- **`x = e` is a definition**, as in mathematics: *let x be e*. There is no `let`, `const` or `val`
  keyword in front of it, and it is not an assignment. Writing to something is a different
  statement with a different symbol, `:=` (§0.8).
- **There is no `return`.** A block's value is its last expression, as in Rust or Ruby, and a
  function's value is its body's. Nothing leaves a function early. The `return`s above are Go's,
  which the compiler wrote.
- **`if` is an expression**, with a value, as Rust's is and C's `?:` is. `grade` has no temporary.
- **`case { … }` with no subject is a chain of guards**: the first true guard's arm is the value,
  and `else` is the last arm. It is the same thing as the `if` chain, written as a table, and it
  compiles to the same code.
- **Comparisons do not chain.** `a < b < c` is refused: `a < b` is a `Bool`, and `<` wants an
  integer.

Three spellings you will get wrong once each:

| | means |
|---|---|
| `x = e` | *let x be e*, a definition |
| `x := e` | write e into the state `x`, declared with `var` |
| `x == e` | are they equal? |

## 0.4 A type is a set

`n ∈ 0..65536` is not notation for a type. It *is* the type: a set, and the statement that `n` is a
member of it ([ADR 0049](../decisions/0049-types-are-sets-and-the-logic-never-prints.md)). You can
type `∈` as `\in`; every symbol has an ASCII spelling, and the reader takes both:

| symbol | ASCII | | symbol | ASCII |
|---|---|---|---|---|
| `∈` | `\in` | | `∪` | `\cup` |
| `×` | `\X` | | `⊕` (xor) | `\oplus` |
| `∀` | `\A` | | `∃` | `\E` |

The sets you will write:

| set | its members |
|---|---|
| `Int` | the integers, all of them |
| `lo..hi` | the integers from lo to hi, **both ends included**: `0..255` is a byte |
| `0..`, `Nat` | an end left out is unbounded: the naturals |
| `Bool`, `String` | `true` and `false`; text |
| `F64` | the numbers a double holds |
| `Array(V)` | tables of V, indexed from 0 |
| `Map(Int, V)` | finite maps from integers to V |
| `A × B` | pairs |
| `'a ∪ 'b(T)` | a variant: a tag, or a tag carrying a T (§0.9) |

A header is three claims: each parameter's set, the result's set in its own parentheses, and
optionally a `where` relating the parameters and an `ensure` about the result.

```go
fn f(x ∈ S, y ∈ T) (R) where φ ensure ψ { body }
```

**Like:** every statically typed language: parameters and results are annotated, and a mismatch
is a compile error.

**Unlike:** the annotations are **claims, and the compiler proves them or refuses the program.**
A result's set is not a conversion and not a cast.

<!-- file docs/book/code/tour/no-clamp.oro -->
```go
export fn clamp(n ∈ 0..1000) (0..255) { n }
```

<!-- gen go docs/book/code/tour/no-clamp.oro -->
```
gen: clamp: the body does not establish 0 <= result && result <= 255; its result is [0, 1000]
```

In Go, `uint8(n)` would compile and truncate. Here the header said the result is a byte, the body
can return 1000, and that is a false statement. Make it true:

<!-- file docs/book/code/tour/clamp.oro -->
```go
use io

export fn clamp(n ∈ 0..1000) (0..255) { if n > 255 { 255 } else { n } }

export fn abs(n ∈ -1000..1000) (r ∈ Int) ensure r >= 0 {
    if n < 0 { -n } else { n }
}

export fn main() {
    io.print_int(clamp(7))
    io.print_int(clamp(1000))
    io.print_int(abs(-12))
}
```

<!-- run go docs/book/code/tour/clamp.oro -->
```
7
255
12
```

<!-- gen go docs/book/code/tour/clamp.oro -->
```go
func TourAbs(v0 int) int {
    v10 := (v0 < 0)
    if v10 {
        v13 := (0 - v0)
        return v13
    }
    return v0
}

func TourClamp(v0 int) int {
    v10 := (v0 > 255)
    if v10 {
        return 255
    }
    return v0
}
```

No check was emitted, because none is needed: the claims were proven and then erased. `abs` names
its result `r` so that `ensure` can speak about it. Change `r >= 0` to `r > 0` and it is refused
the same way, because `abs(0)` is 0.

Inside a body almost nothing is annotated. `aa = a * a` has no type written on it and needs none;
when you do want to state a set for a local, the binder takes one, `end ∈ 0..n = t + got`, and that
too is proven or refused.

### Claims inside a body

A claim may also stand in the middle of a body, `assert φ`, and φ may quantify over a set:
`∀ j ∈ lo..m : φ` is *φ holds for every j from lo to m*, and `∃` is *for some*. Here is the
midpoint of two numbers, written so that it cannot overflow, with what it promises:

<!-- file docs/book/code/tour/no-mid.oro -->
```go
use io

fn mid(lo ∈ 0..1000, hi ∈ 0..1000) (Int) where lo <= hi {
    m = lo + (hi - lo) / 2
    assert 2 * m == lo + hi
    m
}

export fn main() {
    io.print_int(mid(3, 10))
}
```

<!-- gen go docs/book/code/tour/no-mid.oro -->
```
gen: tour-main: an `assert` is false: 2 * 6 == 3 + 10
```

The claim is false when `lo + hi` is odd, and the compiler found the call where it is. What is
true is weaker:

<!-- file docs/book/code/tour/mid.oro -->
```go
use io

fn mid(lo ∈ 0..1000, hi ∈ 0..1000) (Int) where lo <= hi {
    m = lo + (hi - lo) / 2
    assert lo <= m && m <= hi
    assert ∀ j ∈ lo..m : 2 * j <= lo + hi
    m
}

export fn main() {
    io.print_int(mid(3, 10))
    io.print_int(mid(990, 1000))
}
```

<!-- run go docs/book/code/tour/mid.oro -->
```
6
995
```

<!-- gen go docs/book/code/tour/mid.oro -->
```go
func TourMain() int {
    v1 := (strconv.Itoa(6))
    fmt.Println(v1)
    v5 := (strconv.Itoa(995))
    fmt.Println(v5)
    return 995
}
```

**Unlike** C's `assert` or Python's: **an `assert` never runs.** It is proven when the program is
compiled, for every argument the header admits, or the program is refused; then it is erased. Here
both calls had known arguments, so `main` is its two answers. Chapter 16 is the logic: what it
proves, how, and what it cannot.

## 0.5 An integer is an integer

This is the largest difference from every host, and it is one sentence: **`Int` means ℤ, and an
arithmetic operation computes the integer it means**
([ADR 0026](../decisions/0026-an-int-is-an-integer.md),
[ADR 0147](../decisions/0147-int-is-the-integers-at-an-entry-point-and-decimal-renders-them.md)).

<!-- file docs/book/code/tour/square.oro -->
```go
use io

export fn square(n ∈ Int) (Int) { n * n }

export fn main() {
    io.print_int(square(12))
    io.print_int(square(10000000000))
}
```

<!-- run go docs/book/code/tour/square.oro -->
```
144
100000000000000000000
```

In Go, Java and C the second line wraps to a wrong number; in JavaScript it loses its low digits.
Here it is 10²⁰, exactly, on every host:

<!-- run js docs/book/code/tour/square.oro -->
```
144
100000000000000000000
```

<!-- gen go docs/book/code/tour/square.oro -->
```go
func TourMain() *big.Int {
    v1 := (strconv.Itoa(144))
    fmt.Println(v1)
    v5 := (big.NewInt(int64(10000000000)))
    v7 := (big.NewInt(int64(10000000000)))
    v8 := (new(big.Int).Mul(v5, v7))
    v9 := (v8.String())
    fmt.Println(v9)
    return v8
}

func TourSquare(v0 *big.Int) *big.Int {
    v1 := (new(big.Int).Mul(v0, v0))
    return v1
}
```

**The source names no representation**, and the compiler chooses one for each value from what it
proves about it
([ADR 0079](../decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md)).
`square(12)` is 144, which fits a word, so it is the constant 144. `square(10000000000)` does not
fit, so it is Go's `*big.Int`. And `square` itself, which a host may call with any integer, takes
and returns a `*big.Int`, because nothing bounds its `n`.

### Where the word is written

When you want a machine word, say so, as a set. A word is the integers from −2⁶³ to 2⁶³ − 1, and
`-2^63..2^63-1` is that set (`^` is a power, written in a set's ends; xor is `⊕`, §0.12):

<!-- file docs/book/code/tour/no-word.oro -->
```go
export fn square64(n ∈ Int) (-2^63..2^63-1) { n * n }
```

<!-- gen go docs/book/code/tour/no-word.oro -->
```
gen: square64: the body does not establish -9223372036854775808 <= result && result <= 9223372036854775807; its result is [-inf, +inf]
```

The header claimed a result inside the word, and for most `n` the square is not. That is where an
integer is refused: **where a value must be held in a set it is not proven to be in**, here a
declared range, and later a table's element (§0.8) and a host's word (§0.13). Narrow the
parameter, and the claim is true:

<!-- file docs/book/code/tour/word.oro -->
```go
export fn square64(n ∈ -3037000499..3037000499) (-2^63..2^63-1) { n * n }
```

<!-- gen go docs/book/code/tour/word.oro -->
```go
func TourSquare64(v0 int) int {
    v9 := (v0 * v0)
    return v9
}
```

One multiplication on an `int`, and no check, because 3037000499² is below 2⁶³: the proof is why
the plain machine instruction is correct.

### A value at any size

Squaring 3 seven times is 3¹²⁸, a number of 62 digits:

<!-- file docs/book/code/tour/grow.oro -->
```go
use io

export fn main() {
    p = loop i = 0, p = 3 {
        i < 7 -> again(i + 1, p * p)
        else -> p
    }
    io.print_int(p)
}
```

<!-- run go docs/book/code/tour/grow.oro -->
```
11790184577738583171520872861412518665678211592275841109096961
```

<!-- gen go docs/book/code/tour/grow.oro -->
```go
func TourMain() *big.Int {
    v2 := (big.NewInt(int64(3)))
    var v4 int = 0
    v5 := v2
    for ; ; v4 = (v4 + 1) {
        v7 := (v4 < 7)
        if v7 {
            v11 := (v5.Mul(v5, v5))
            v5 = v11
            continue
        }
        break
    }
    v13 := (v5.String())
    fmt.Println(v13)
    return v5
}
```

The counter `i` is an `int` and the product a `*big.Int`, in one loop. Windows has no big-integer
library to call, and prints the same 62 digits, computed by a table of 64-bit digits written in
this language:

<!-- run windows docs/book/code/tour/grow.oro -->
```
11790184577738583171520872861412518665678211592275841109096961
```

`io.print_int` prints an integer of any size; `decimal(n)` is its text (§0.11).

### Which targets

Whether a program compiles is a fact about the program *and* the target, and it is computed, not
promised:

```bash
go run ./cmd/portable docs/book/code/tour/square.oro
```

<!-- portable docs/book/code/tour/square.oro -->
```
portable to go, js, java — not windows
```

Go, JavaScript and Java each have an integer of any size, `*big.Int`, `BigInt` and `BigInteger`,
so a host there can call `square` with one. Windows' callers are C programs, and C has none:

<!-- gen windows docs/book/code/tour/square.oro -->
```
gen: tour-square: its parameter n ∈ Int is any integer, and an entry point of target windows takes no integer above its word: its callers are C's, and C has none (K(ℕ*) is this program's own). Bound it, `where n <= …`, or give it a range inside the word
```

The program itself runs on Windows, since `main` takes nothing; it is the exported `square` that
no C caller could use. Chapter 18 says what a Windows export can take and give. A bounded range is no trouble anywhere, even one whose square is above every
word:

<!-- file docs/book/code/tour/wide.oro -->
```go
export fn wide(n ∈ 0..2^63-1) (0..2^126) { n * n }
```

<!-- portable docs/book/code/tour/wide.oro -->
```
portable to go, js, java, windows
```

On Go `wide` takes an `int` and returns a `*big.Int`; on JavaScript, whose numbers are exact only
up to 2⁵³, it takes and returns a `BigInt`. **Portability is a property a program has or lacks,
and the compiler tells you which.**

### Numbers with a decimal point

<!-- file docs/book/code/tour/exact.oro -->
```go
use io

export fn exact() (Bool) { 0.1 + 0.2 == 0.3 }

export fn tax(cents ∈ 0..10^9) (Int) { round(Int, cents * 0.0825, 'half_away) }

export fn main() {
    io.print_line(if exact() { "exact" } else { "not exact" })
    io.print_int(tax(1999))
    io.print_int(tax(1000))
}
```

<!-- run js docs/book/code/tour/exact.oro -->
```
exact
165
83
```

**Like:** the literals look the same. **Unlike:** `0.1` is one tenth, exactly
([ADR 0086](../decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md)).
A numeral is a decimal fraction, `+`, `-`, `*` and the comparisons on them are exact, and one tenth
plus two tenths is three tenths, even in JavaScript, where `0.1 + 0.2 === 0.3` is `false`.

Precision is lost in exactly one place, and you write it: `round(S, e, mode)` rounds `e` onto the
set `S`. `tax` is 8.25% of an amount in cents, rounded to the cent, a tie going away from zero:
1999 × 0.0825 is 164.9175, which rounds to 165, and 1000 × 0.0825 is 82.5, a tie, which goes to
83. Here is what JavaScript received:

<!-- gen js docs/book/code/tour/exact.oro -->
```javascript
export function tourExact() {
    return ((1 + 2) === 3);
}
…
export function tourTax(v0) {
    const v10 = (v0 * 825);
    const v12 = (((a, m) => { const r = a % m; return r < 0 ? r + m : r + 0; })(v10, 10000));
    const v14 = ((Math.trunc((v10 - v12) / 10000) + 0));
    const v17 = (v12 * 2);
    const v19 = (v17 === 10000);
    return ((v17 > 10000) ? (v14 + 1) : (v19 ? ((v10 >= 0) ? (v14 + 1) : v14) : v14));
}
```

No double anywhere. The compiler holds each numeral as an integer at a known scale: `exact`
compares tenths, `1 + 2` with `3`, and `tax` multiplies by 825 ten-thousandths, then divides by
10000 and looks at twice the remainder to round. `F64` is a set like the others; a rounding onto
it is a host's floating-point operation. `/` is integer division and needs integer operands.

## 0.6 Functions vanish, unless the compiler keeps one

<!-- file docs/book/code/tour/twice.oro -->
```go
use io

fn twice(f, x) { f(f(x)) }

export fn add6(n ∈ 0..100) (Int) { twice(fn(x) { x + 3 }, n) }

export fn shout(s ∈ String) (String) { twice(fn(x) { x ++ "!" }, s) }

export fn main() {
    io.print_int(add6(4))
    io.print_line(shout("hey"))
}
```

<!-- run go docs/book/code/tour/twice.oro -->
```
10
hey!!
```

<!-- gen go docs/book/code/tour/twice.oro -->
```go
func TourAdd6(v0 int) int {
    v10 := (v0 + 3)
    v12 := (v10 + 3)
    return v12
}
…
func TourShout(v0 string) string {
    v2 := (v0 + "!")
    v4 := (v2 + "!")
    return v4
}
```

**Like:** functions are values; `fn(x) { … }` is an anonymous one; you can pass a function to a
function.

**Unlike:**

- **A function you define is a way of writing a program, not a promise of a call at run time.**
  `twice` is gone, and so is the anonymous function. The compiler works by *reduction*: it
  substitutes definitions and applies functions until only the target's own operations remain.
  What a host can call is what you `export`.
- **`twice` has no types, and was used at two.** There are no generics, no type parameters and no
  `<T>`: a definition with unannotated parameters is specialised wherever it is used, and what
  reaches the type checker is already first-order and monomorphic. Annotate the functions you
  `export`, since a host must know their sets, and the rest as far as you want their claims checked.
- **Higher-order code costs nothing**, for the same reason: there is no closure to allocate and no
  indirect call.

Reduction copies a definition into every place that uses it. For a small one that is exactly
right. For a large one used many times it is the same code many times over, so **the compiler
may keep a definition as one real function and call it**
([ADR 0113](../decisions/0113-a-programs-definitions-are-kept-as-functions-where-the-target-prints-them.md)).
It decides; the source has no word for it:

<!-- file docs/book/code/tour/kept.oro -->
```go
use io

fn triangle(n ∈ 0..1000) (Int) {
    loop acc = 0, i = 0 {
        i < n -> again(acc + i, i + 1)
        else  -> acc
    }
}

fn twice(f, x) { f(f(x)) }

export fn main() {
    io.print_int(triangle(10))
    io.print_int(triangle(100))
    io.print_int(twice(fn(x) { x + 3 }, 4))
}
```

<!-- run go docs/book/code/tour/kept.oro -->
```
45
4950
10
```

<!-- gen go docs/book/code/tour/kept.oro -->
```go
func TourMain() int {
    var v3 int = 0
    var v4 int = 0
    for ; ; v4 = (v4 + 1) {
        v6 := (v4 < 10)
        if v6 {
            v8 := (v3 + v4)
            v3 = v8
            continue
        }
        break
    }
    v12 := (strconv.Itoa(v3))
    fmt.Println(v12)
    var v17 int = 0
    var v18 int = 0
    for ; ; v18 = (v18 + 1) {
        v20 := (v18 < 100)
        if v20 {
            v22 := (v17 + v18)
            v17 = v22
            continue
        }
        break
    }
    v26 := (strconv.Itoa(v17))
    fmt.Println(v26)
    v29 := (strconv.Itoa(10))
    fmt.Println(v29)
    return 10
}
```

On Go everything vanished. `triangle` was used twice and there are two loops, each with its own
bound, 10 and 100, written into it. `twice` and its anonymous function are the constant 10.
`io.print_int` is Go's own `strconv.Itoa` and `fmt.Println`.

Windows has neither. There, `io.print_int` is a function written in this language, which turns an
integer into digits and hands them to the system, and this program uses it three times:

<!-- gen windows docs/book/code/tour/kept.oro -->
```llvm
note: 3 definition(s) kept as functions, 0 inlined by their contracts
…
  %v12 = call i1 @"win/print.print_int"(i64 %v2)
…
  %v25 = call i1 @"win/print.print_int"(i64 %v15)
  %v27 = call i1 @"win/print.print_int"(i64 10)
…
define i1 @"win/print.print_int"(i64 %v0) #0 {
```

One function, three calls. `triangle` is still two loops and `twice` is still 10: they are
small, and copying them is cheaper than calling them. The rules, in short:

- **A function that takes or returns a function is never kept.** It is always reduced away,
  which is why higher-order code is free.
- **A first-order definition is kept where it pays**: it is large, and copying it would cost more
  than a call.
- **Only where the target prints functions.** That is the Windows target today. On Go,
  JavaScript and Java every definition is still reduced away, and the exports are the only
  functions in the output.
- **No answer changes and no proof is lost.** A kept definition is proven once, under its
  header's sets, and each call proves its arguments are in them. If that proof fails where the
  copies' proofs would succeed, the compiler goes back to copying.

So write functions for the reader. Whether one is a call is not yours to decide, and you cannot
get it wrong.

### A function may not outlive its call

<!-- file docs/book/code/tour/no-closure.oro -->
```go
export fn adder(n ∈ 0..10) { fn(x) { x + n } }
```

<!-- gen go docs/book/code/tour/no-closure.oro -->
```
gen: tour-adder: lowering: a λ in value position is a closure (callbacks.md): fn(x) { x + n }
```

Returning a function to the host, or storing one in a table, would need a closure, a hidden
allocation, and hidden allocation is what this language exists to avoid. What you may do is hand a
function to a host that applies it **during** the call, as Go's `strings.Map` applies its first
argument to each character of its second:

<!-- file docs/book/code/tour/rot13.oro -->
```go
use io
use go/strings

fn rot(r ∈ -2^31..2^31-1) (-2^31..2^31-1) {
    case {
        97 <= r && r <= 122 -> 97 + (r - 97 + 13) % 26
        65 <= r && r <= 90 -> 65 + (r - 65 + 13) % 26
        else -> r
    }
}

export fn main() {
    io.print_line(strings.Map(fn(r) { rot(r) }, "Hello, World"))
}
```

<!-- run go docs/book/code/tour/rot13.oro -->
```
Uryyb, Jbeyq
```

<!-- gen go docs/book/code/tour/rot13.oro -->
```go
func TourMain() string {
    v0 := func(v1h int32) int32 {
        v1 := int(v1h)
…
    v43 := (strings.Map(v0, "Hello, World"))
    fmt.Println(v43)
    return v43
}
```

The λ became a Go function literal over `int32`, the set Go's declaration of `Map` gives its
argument, and its body was proven under that set. Go holds nothing after `Map` returns, so nothing
outlives the call. Chapter 17 is functions a host calls: what the host promises about the order
and number of its calls, and what a λ may do under each promise. Chapter 18 is the one function
Windows keeps, a window procedure.

## 0.7 One loop, and no recursion

**Like:** nothing, quite. Here is what you would write first:

<!-- file docs/book/code/tour/no-fact.oro -->
```go
export fn fact(n ∈ 0..20) (Int) {
    if n == 0 { 1 } else { n * fact(n - 1) }
}
```

<!-- gen go docs/book/code/tour/no-fact.oro -->
```
gen: docs/book/code/tour/no-fact.oro: fact is recursive: it is defined in terms of itself, and recursion is not in the language, since δ unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with `again` (ADR 0015).
```

§0.6 is why: a definition is unfolded into its uses, and a definition that mentions itself has no
end to its unfolding. So **there is no recursion**, and **there is one iteration form**, which
replaces `for`, `while`, `do`, `break` and `continue`:

<!-- file docs/book/code/tour/fact.oro -->
```go
use io

fn fact(n ∈ 0..1000) (Nat) {
    loop i = 1, acc = 1 {
        i <= n -> again(i + 1, acc * i)
        else   -> acc
    }
}

fn gcd(a ∈ 0..10^6, b ∈ 0..10^6) (Int) {
    loop x = a, y = b {
        y == 0 -> x
        else   -> again(y, x % y)
    }
}

export fn main() {
    io.print_int(gcd(1071, 462))
    io.print_int(fact(30))
}
```

<!-- run go docs/book/code/tour/fact.oro -->
```
21
265252859812191058636308480000000
```

<!-- gen go docs/book/code/tour/fact.oro -->
```go
func TourMain() *big.Int {
    var v3 int = 1071
    var v4 int = 462
    for {
        v6 := (v4 == 0)
        if v6 {
            break
        }
        v9 := (v3 % v4)
        v3, v4 = v4, v9
        continue
    }
    v10 := (strconv.Itoa(v3))
    fmt.Println(v10)
    v14 := (big.NewInt(int64(1)))
    var v16 int = 1
    v17 := v14
    for ; ; v16 = (v16 + 1) {
        v19 := (v16 <= 30)
        if v19 {
            v23 := (big.NewInt(int64(v16)))
            v24 := (v17.Mul(v17, v23))
            v17 = v24
            continue
        }
        break
    }
    v26 := (v17.String())
    fmt.Println(v26)
    return v17
}
```

Read a `loop` as a table. `loop i = 1, acc = 1` names the variables and their first values. Each
arm is `guard -> what happens`, tried in order:

- **`again(e₁, e₂)`** goes round, with the variables' next values, by position. All of them are
  given at once, so there is no half-updated state: `again(y, x % y)` is Euclid's step and needs no
  temporary. Go got it as `v3, v4 = v4, v9`.
- **An arm without `again` leaves the loop, and its value is the loop's value.** That is `break`,
  with a result.

**Unlike** a `while` loop, the variables are not assigned to; they are *rebound* each time round,
as a recursive function's parameters would be. A `loop` is the tail-recursive function you were
going to write, with the recursion spelled `again`. Two consequences:

- **The compiler proves termination** where it can, by size-change analysis, and reports the loops
  it cannot.
- **`again` stands only where the loop's turn ends**: as an arm's value, or last in an arm's block.
  It may not sit under an `if`:

<!-- file docs/book/code/tour/no-again.oro.txt -->
```go
export fn skip(n ∈ 0..10) (Int) {
    loop i = 0 {
        i < n -> if i == 5 { again(i + 2) } else { again(i + 1) }
        else  -> i
    }
}
```

<!-- gen go docs/book/code/tour/no-again.oro.txt -->
```
gen: docs/book/code/tour/no-again.oro.txt: line 3: `again` under an if (ADR 0015)
```

The condition belongs in the guards: `i < n && i == 5 -> again(i + 2)`, then `i < n ->
again(i + 1)`. A loop's arms are where its decisions are written.

And look at `fact`'s result, `Nat`, and 30! in the output: 33 digits, past every word. `gcd` is a
loop over two `int`s and `fact` a loop whose counter is an `int` and whose product is a
`*big.Int`, chosen as in §0.5, value by value.

## 0.8 State, and tables

A definition never changes. When something must change, it is declared as state, with `var`, and
written with `:=`:

<!-- file docs/book/code/tour/cells.oro -->
```go
use io

export fn count_even(t ∈ Array(0..255)) (Int) {
    var n := 0
    loop i = 0 {
        i < len(t) -> {
            if t[i] % 2 == 0 { n := n + 1 }
            again(i + 1)
        }
        else -> n
    }
}

export fn main() {
    io.print_int(count_even(array { 3, 8, 12, 7, 0 }))
}
```

<!-- run go docs/book/code/tour/cells.oro -->
```
3
```

<!-- gen go docs/book/code/tour/cells.oro -->
```go
func TourCountEven(v0 []byte) int {
    var v4 int = 0
    var v5 int = 0
    for ; ; v5 = (v5 + 1) {
        v6 := len(v0)
        v7 := (v5 < v6)
        if v7 {
            v10 := int(v0[v5])
            v12 := (v10 & 1)
            v14 := (v12 == 0)
            var v15 int
            if v14 {
                v17 := (v4 + 1)
                v15 = v17
            } else {
                v15 = v4
            }
            v4 = v15
            continue
        }
        break
    }
    return v4
}
```

**Like:** a mutable local, updated in a loop, in program order. `t[i]` indexes, `len(t)` is the
length, indices start at 0. An `if` with no `else` is a statement. A table of `0..255` reached Go
as a `[]byte`, and `% 2` as `& 1`.

**Unlike:** `var` is the *only* mutation, and it is local. There are no mutable globals and no
mutable fields, and **a table is a value**: it cannot be written to.

<!-- file docs/book/code/tour/no-store.oro -->
```go
export fn first_one(t ∈ Array(Int)) (Int) where len(t) > 0 {
    t[0] := 1
    t[0]
}
```

<!-- gen go docs/book/code/tour/no-store.oro -->
```
gen: tour-first-one: a store writes into t, which is a frozen table, not a buffer.
```

A parameter of set `Array(Int)` is somebody's table, and nobody who holds a table ever sees it
change. This is what Python's lists, Go's slices and JavaScript's arrays do not give you.

### Making a table

Three ways, all written with the word `array` and braces:

<!-- file docs/book/code/tour/squares.oro -->
```go
use io

primes = array { 2, 3, 5, 7, 11 }

export fn sum_squares(n ∈ 0..1000) (Int) {
    sq = array { k * k : k ∈ 0..n-1 }
    loop i = 0, acc = 0 {
        i < n -> again(i + 1, acc + sq[i])
        else  -> acc
    }
}

export fn main() {
    io.print_int(primes[2])
    io.print_int(sum_squares(10))
}
```

- **By listing it**: `array { 2, 3, 5, 7, 11 }`.
- **By a rule**: `array { k * k : k ∈ 0..n-1 }` is the table whose entry k is k², for k from 0 to
  n − 1. It is set-builder notation, and it is Python's `[k * k for k in range(n)]`.
- **By filling a buffer**: `var s := array { … }` makes a table you may write, element by element,
  until the block ends; then it is a value like any other.

<!-- run go docs/book/code/tour/squares.oro -->
```
5
285
```

<!-- gen go docs/book/code/tour/squares.oro -->
```go
func TourSumSquares(v0 int) int {
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
```

Look for the table of squares in that output. It is not there: a table by a rule is a function, the
read `sq[i]` applied it, and no array was ever allocated. Likewise `primes[2]` is the constant 5.
Writing the intermediate structure is free when reduction can see through it.

### An index is proven

<!-- file docs/book/code/tour/no-index.oro -->
```go
export fn third(t ∈ Array(Int)) (Int) { t[2] }
```

<!-- gen go docs/book/code/tour/no-index.oro -->
```
gen: tour-third: t[2] is an indexing, and 2 < len(t) does not follow
  known: nothing
```

There is no index-out-of-range at run time, because a read that might be one does not compile. A
table is a function on `0..len-1`, and applying it outside its domain means nothing. Say why the
index is good, either as a precondition your callers must meet or as a test:

<!-- file docs/book/code/tour/index.oro -->
```go
use io

export fn third(t ∈ Array(Int)) (Int) where len(t) >= 3 { t[2] }

export fn third_or_zero(t ∈ Array(Int)) (Int) {
    if len(t) > 2 { t[2] } else { 0 }
}

export fn main() {
    io.print_int(third(array { 5, 6, 7, 8 }))
    io.print_int(third_or_zero(array { 5, 6 }))
}
```

<!-- run go docs/book/code/tour/index.oro -->
```
7
0
```

<!-- gen go docs/book/code/tour/index.oro -->
```go
func TourThird(v0 []int) int {
    v5 := v0[2]
    return v5
}

func TourThirdOrZero(v0 []int) int {
    v1 := len(v0)
    v3 := (v1 > 2)
    if v3 {
        v6 := v0[2]
        return v6
    }
    return 0
}
```

`third` is one read: its callers proved `len(t) >= 3`, and `main`'s call did, by its literal. A
loop's guard is such a test too, which is why `t[i]` under `i < len(t)` in `count_even` needed
nothing said. In practice most indexing is like that, and proves itself.

### A table holds words

An element of a table is held in a machine word, so a value stored into one is the §0.5 case: it
must be proven to fit. This is refused,

<!-- part docs/book/code/tour/no-prefix.oro -->
```go
export fn prefix_sums(t ∈ Array(0..255)) (Array(Int)) {
    var s := array { 0 : _ ∈ 0..len(t)-1 }
    loop i = 0, acc = 0 {
        i < len(t) -> { s[i] := acc + t[i]; again(i + 1, acc + t[i]) }
        else -> s
    }
}
```

<!-- gen go docs/book/code/tour/no-prefix.oro -->
```
  + [0, 2351959869397967831040] in acc + t[i]
```

because a table may be as long as memory allows, and that many bytes sum past 2⁶³. The compiler
computed the bound and showed it. Adding `where len(t) <= 2^20` to the header
([prefix.oro](code/tour/prefix.oro)) makes it *3 of 3 integer operations bounded*: the limit you
were assuming is now written down, and every caller is held to it.

When you cannot prove it, you can **ask for the trap**. `-checked` is a flag of the compiler, on
`gen` and on `build`, and no word of the source. It emits the host's own checked arithmetic for
each operation that was not proven, and the program stops when it runs if one of them leaves the
word. This one stores 3, 3², 3⁴, … into a table of eight:

<!-- file docs/book/code/tour/trap.oro -->
```go
use io

export fn main() {
    var t := array { 0 : _ ∈ 0..7 }
    loop i = 0, p = 3 {
        i < 8 -> { t[i] := p; again(i + 1, p * p) }
        else  -> { ignore(p); io.print_int(t[4]) }
    }
}
```

<!-- gen go docs/book/code/tour/trap.oro -->
```
gen: tour-main: 1 of 2 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  * [9, +inf] in p * p
```

```bash
go run ./cmd/build -checked -target=go -o trap.exe docs/book/code/tour/trap.oro
```

<!-- gen go docs/book/code/tour/trap.oro -checked -->
```go
            v6[v7] = v8
            v15 := (func(a, b int) int { t := a * b; if a != 0 && t/a != b { panic("int overflow") }; return t }(v8, v8))
```

<!-- run go docs/book/code/tour/trap.oro -checked -->
```
panic: int overflow
…
```

The sixth squaring, 3³² × 3³², left the word, and the program stopped there and printed nothing,
which is the point: in Go without the test it stores a wrong number and goes on. The `i + 1` was
proven and costs nothing; the one test is on the one multiplication nobody proved. On JavaScript
the trap is a `RangeError`, and on Windows the processor's own, an illegal instruction.

### Maps

<!-- file docs/book/code/tour/maps.oro -->
```go
use io

codes = map { 200 : 1, 404 : 2, 500 : 3 }

export fn known(code ∈ 0..999) (Int) {
    case codes[code] {
        'some(v) -> v
        'none    -> 0
    }
}

export fn main() {
    io.print_int(known(404))
    io.print_int(known(302))
}
```

<!-- run go docs/book/code/tour/maps.oro -->
```
2
0
```

<!-- gen go docs/book/code/tour/maps.oro -->
```go
func TourKnown(v0 int) int {
    v15 := map[int]int{200: 1, 404: 2, 500: 3}
    v17, ok16 := v15[v0]
…
    return 0
}
```

`map { 200 : 1, 404 : 2, 500 : 3 }` is a map written out, and `var m := map(cap)` one you fill. A
read `m[k]` cannot be assumed to find its key, so its value is `'some(v)` or `'none`, and you say
what happens in each case. Go's own `v, ok := m[k]` is what it became.

## 0.9 Variants and `case`

<!-- file docs/book/code/tour/shapes.oro -->
```go
pub set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

fn sq(n) { n * n }

pub fn area(s) {
    case s {
        'circle(r) -> 3 * sq(r)
        'square(w) -> sq(w)
        'point     -> 0
    }
}
```

**Like:** Rust's `enum` with `match`, Swift's enums, TypeScript's tagged unions, a sealed class
hierarchy. A value of `Shape` is one of three alternatives, and `case` takes it apart, binding the
payload. (`sq` is a helper of the module's own, used twice; it has no `pub`, which matters in
§0.13.)

**Unlike:**

- **A tag is written with a leading quote**, `'circle`, so a tag can never be mistaken for a
  variable. In a pattern a bare name always binds and a tag never does. This is the one use of `'`.
- **The declaration is a set equation**: `Shape` is the union of the circles, the squares and the
  point. `set` declares it, as `fn` declares a function. A tag carries at most one payload.
- **`case` must be exhaustive**, and says which alternative you forgot:

<!-- file docs/book/code/tour/no-case.oro -->
```go
use shapes

export fn round?(s ∈ shapes.Shape) (Bool) {
    case s {
        shapes.'circle(r) -> true
        shapes.'point     -> false
    }
}
```

<!-- gen go docs/book/code/tour/no-case.oro -->
```
gen: docs/book/code/tour/no-case.oro: round?: case on shape is not exhaustive: square is unmatched. Add it a clause, or an `else`
```

(A name may end in one `?`, the convention for a predicate. A tag from another module is qualified
like any other name, `shapes.'circle`.)

- **A variant costs nothing where the tag is known.**

<!-- file docs/book/code/tour/pick.oro -->
```go
use io
use shapes

export fn pick(n ∈ 0..1000) (Int) {
    shapes.area(if n < 10 { shapes.'point } else { shapes.'circle(n) })
}

export fn main() {
    io.print_int(shapes.area(shapes.'square(12)))
    io.print_int(pick(5))
    io.print_int(pick(20))
}
```

<!-- run go docs/book/code/tour/pick.oro -->
```
144
0
1200
```

<!-- gen go docs/book/code/tour/pick.oro -->
```go
func TourMain() int {
    v1 := (strconv.Itoa(144))
    fmt.Println(v1)
    v5 := (strconv.Itoa(0))
    fmt.Println(v5)
    v9 := (strconv.Itoa(1200))
    fmt.Println(v9)
    return 1200
}

func TourPick(v0 int) int {
    v10 := (v0 < 10)
    if v10 {
        return 0
    }
    v15 := (v0 * v0)
    v16 := (3 * v15)
    return v16
}
```

No tag, no `switch`, no allocation. In `main` every tag was known at compile time, and the three
`case`s reduced to their answers. In `pick` the tag was decided by an `if`, and the `case` was
pushed into its branches, so what is left is the `if` that decided it.

## 0.10 A failure is a value

There are no exceptions and there is no `null`. A computation that may fail returns a variant that
says so. The library's is `Result`, and it is the whole of
[lib/result.oro](../../lib/result.oro):

```go
pub set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant
```

<!-- file docs/book/code/tour/half.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) (result.Result(Int, Int)) {
    if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) }
}

fn quarter(n ∈ 0..1000000) (result.Result(Int, Int)) {
    try result.'ok(h) = half(n) { half(h) }
}

export fn quarter_or_zero(n ∈ 0..1000000) (Int) {
    case quarter(n) {
        result.'ok(q)  -> q
        result.'err(e) -> { ignore(e); 0 }
    }
}

export fn quarter_or_crash(n ∈ 0..1000000) (Int) {
    expect result.'ok(q) = quarter(n) else "a multiple of four is expected"
    q
}

export fn main() {
    io.print_int(quarter_or_zero(20))
    io.print_int(quarter_or_zero(6))
    io.print_int(quarter_or_crash(40))
    io.print_int(quarter_or_crash(6))
}
```

<!-- run go docs/book/code/tour/half.oro -->
```
5
0
10
panic: a multiple of four is expected
…
```

**Like:** Rust's `Result`, Go's `(value, err)`, Swift's `throws`. The three ways to meet a failure
are the three you know:

| here | elsewhere | what it does |
|---|---|---|
| `case r { 'ok(v) -> …; 'err(e) -> … }` | `match`, `if err != nil` | handle both |
| `try 'ok(v) = e { body }` | Rust's `?` | go on with `v`; hand any failure back unchanged |
| `expect 'ok(v) = e else "why"` | `unwrap`, `panic` | go on with `v`; crash with the reason |

6 is even and its half, 3, is not, so `quarter_or_zero(6)` is 0 and `quarter_or_crash(6)` stops
the program with its reason.

**Unlike:**

- **`Result` is a library, not the language.** The two words after the declaration are what the
  compiler knows: `success 'ok` names the alternative `try` and `expect` continue on, and any
  variant of yours may say the same.
- **`relevant` means a value may not be dropped.** This is Go's unchecked `err` and Rust's
  `#[must_use]`, as a rule and not a lint. A program that writes a file and ignores whether it
  worked ([examples/errors/discard.oro](../../examples/errors/discard.oro)) is refused:

<!-- gen go examples/errors/discard.oro -->
```
gen: the result of os.WriteFile is a value of a relevant type (spec/errors.md §7), and it is discarded: use it, or discard it with ignore(…)
```

  Dropping it on purpose is written down, `ignore(e)`, where a reader will see it.
- **It costs what the host's own error handling costs.** `quarter_or_crash`, compiled:

<!-- gen go docs/book/code/tour/half.oro -->
```go
func TourQuarterOrCrash(v0 int) int {
    v10 := (v0 & 1)
    v12 := (v10 == 0)
    if v12 {
        v14 := (v0 >> 1)
        v16 := (v14 & 1)
        v18 := (v16 == 0)
        if v18 {
            v20 := (v14 >> 1)
            return v20
        }
        panic("a multiple of four is expected")
    }
    panic("a multiple of four is expected")
}
```

Two tests and a `panic`. `half`, `quarter`, the `try` and every `Result` are gone, and the
divisions by two became shifts. A host function that fails, such as Go's `os.ReadFile`, arrives as
a `Result` too, and compiles back to Go's own `if err != nil`.

## 0.11 Tuples, text and records

<!-- file docs/book/code/tour/tuples.oro -->
```go
use io

fn divmod(a, b) { (a / b, a % b) }

export fn clock(seconds ∈ 0..86399) (String) {
    (m, s) = divmod(seconds, 60)
    (h, min) = divmod(m, 60)
    string_of(48 + h / 10) ++ string_of(48 + h % 10) ++ ":" ++
        string_of(48 + min / 10) ++ string_of(48 + min % 10)
}

export fn main() {
    io.print_line(clock(3725))
    io.print_line(clock(86399))
}
```

<!-- run go docs/book/code/tour/tuples.oro -->
```
01:02
23:59
```

<!-- gen go docs/book/code/tour/tuples.oro -->
```go
func TourClock(v0 int) string {
    v10 := (v0 / 60)
    v12 := (v10 / 60)
    v14 := (v10 % 60)
    v17 := (v12 / 10)
    v18 := (48 + v17)
    v19 := (string(rune(v18)))
…
    return v39
}
```

**Like:** Python's and Rust's tuples, Go's multiple results: `(a, b)` makes one, `(m, s) = e` takes
one apart, `()` is the empty one, which a block ending in a statement has as its value.

**Unlike:** a tuple is not allocated. In what `clock` compiles to the pairs are gone, and `s`, the
seconds, which `clock` never uses, is never computed: there is no `v0 % 60`. A line that ends in an
operator continues on the next.

Text is small on purpose. A `String` is a sequence of Unicode characters, `++` joins two,
`string_of(n)` is the one-character string of code point n (48 is `0`), and `decimal(n)` is the
decimal numeral of an integer of any size, the text `io.print_int` prints. Everything richer is the
host's, under the host's name (§0.13).

A record is a tuple whose parts have names:

<!-- file docs/book/code/tour/records.oro -->
```go
use io

set Point = record { x ∈ Int, y ∈ Int }

fn step(p ∈ Point, dx ∈ -1..1, dy ∈ -1..1) (Point) { p with { x : p.x + dx, y : p.y + dy } }

export fn main() {
    start = record { x : 0, y : 0 }
    p = step(step(start, 1, 0), 1, 1)
    io.print_int(p.x)
    io.print_int(p.y)
    io.print_int(start.x)
}
```

<!-- run go docs/book/code/tour/records.oro -->
```
2
1
0
```

<!-- gen go docs/book/code/tour/records.oro -->
```go
func TourMain() int {
    v1 := (strconv.Itoa(2))
    fmt.Println(v1)
    v5 := (strconv.Itoa(1))
    fmt.Println(v5)
    v9 := (strconv.Itoa(0))
    fmt.Println(v9)
    return 0
}
```

**Like:** a `struct` read by its fields, and OCaml's and F#'s `{ r with x = … }` for a changed copy.
**Unlike:** `with` never changes `start`, two record sets with the same labels are one set whatever
they are called, and here every value was known, so the compiler computed the whole program: the
records, the steps and the copies are gone, and `main` prints three numbers. Chapter 11.

## 0.12 Bits

<!-- file docs/book/code/tour/bits.oro -->
```go
use io

export fn clear?(flags ∈ 0..255, mask ∈ 0..255) (Bool) { flags & mask == 0 }

export fn fnv32(src ∈ Array(0..255)) (0..2^32-1) {
    loop i = 0, h = 0x811C9DC5 {
        i < len(src) -> again(i + 1, ((h ⊕ src[i]) * 16777619) mod 2^32)
        else -> h
    }
}

export fn main() {
    io.print_line(if clear?(12, 3) { "clear" } else { "set" })
    io.print_int(fnv32(array { 104, 101, 108, 108, 111 }))
}
```

<!-- run go docs/book/code/tour/bits.oro -->
```
clear
1335831723
```

**Like:** `&`, `|`, `~`, `<<`, `>>`, hexadecimal literals.

**Unlike:**

- **`flags & mask == 0` means `(flags & mask) == 0`.** In C, Java, JavaScript and Go's ancestors
  `==` binds tighter than `&`, and that line tests the wrong thing. Here every arithmetic and bit
  operator binds tighter than every comparison: 12 is `1100` and 3 is `0011`, so they share no bit.
- **Xor is `⊕`**, typed `\oplus`. `^` is the power, as in mathematics, and takes constants.
- **Nothing wraps unless you say so.** There is no `uint32` whose multiplication silently drops
  bits. FNV-1a's step is *xor, multiply, keep the low 32 bits*, and the last part is written:
  `mod 2^32`. The compiler then computes the whole step in the host's wrapping arithmetic, because
  under `mod 2ⁿ` that is correct
  ([ADR 0082](../decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md)):

<!-- gen go docs/book/code/tour/bits.oro -->
```go
func TourFnv32(v0 []byte) int {
    var v4 int = 0
    var v5 int = 2166136261
    for ; ; v4 = (v4 + 1) {
        v6 := len(v0)
        v7 := (v4 < v6)
        if v7 {
            v12 := int(v0[v4])
            v13 := (v5 ^ v12)
            v15 := (v13 * 16777619)
            v17 := (v15 & 4294967295)
            v5 = v17
            continue
        }
        break
    }
    return v5
}
```

1335831723 is the 32-bit FNV-1a hash of `hello`, as Go's `hash/fnv` computes it. This function is
from a real program, [examples/hash/fnv.oro](../../examples/hash/fnv.oro), whose output equals Go's
on three hosts.

## 0.13 Modules, and the host

`shapes.oro` in §0.9 is a module because it is a file. Another file uses it by its path:

<!-- file docs/book/code/tour/room.oro -->
```go
use shapes
use io

fn floor(side ∈ 0..1000) (Int) { shapes.area(shapes.'square(side)) }

export fn main() {
    io.print_int(floor(12))
}
```

<!-- run go docs/book/code/tour/room.oro -->
```
144
```

**Like:** `import`. `use shapes` binds the module's name, `use go/os as gos` binds another, and a
member is `module.name`.

**Unlike:** there are two markers, and they answer different questions
([ADR 0078](../decisions/0078-a-modules-interface-is-what-it-marks-pub.md)).

- **`pub`: may another module name this?** Private is the default. `shapes.sq` has no `pub`:

<!-- file docs/book/code/tour/no-private.oro -->
```go
use shapes

export fn f(n ∈ 0..1000) (Int) { shapes.sq(n) }
```

<!-- gen go docs/book/code/tour/no-private.oro -->
```
gen: docs/book/code/tour/no-private.oro: f: module shapes defines sq and does not make it `pub`: a declaration is private to its module unless marked `pub` (ADR 0078)
```

- **`export`: does the host get a function for this?** Only an `export` of the file being compiled
  is emitted. Everything else, `pub` or not, exists to be reduced away (§0.6).

Case carries no meaning anywhere: `Shape` is capitalised by convention, as sets are, and nothing
reads it.

### The host's own library

<!-- file docs/book/code/tour/upper.oro -->
```go
use go/strings
use go/fmt

export fn main() {
    fmt.Println(strings.ToUpper("quiet"), 6 * 7)
}
```

<!-- run go docs/book/code/tour/upper.oro -->
```
QUIET 42
```

A module whose path begins with a host's name is that host's own package, under the host's own
names, called with no wrapper: the emitted Go says `strings.ToUpper("quiet")`. The language does not
abstract over its hosts, it uses them. And so:

<!-- portable docs/book/code/tour/upper.oro -->
```
portable to go — not js, java, windows
```

A program that names Go is a Go program, and the compiler says so. `io` and `os`, with no host in
their path, are libraries written once over each host's own calls; `hello.oro` is portable to Go,
JavaScript and Java. **You choose, per program, how much of one host to use**, and `cmd/portable`
reports what the choice cost.

A host's function is declared with the host's sets. Go's `strings.Repeat` takes a Go `int`, so its
count is a word, and a count the program cannot prove is one is refused as in §0.5; `strings.Map`
in §0.6 gave its λ an `int32`. Inside the program `Int` is ℤ; at a host's door it is what the host
takes.

## 0.14 What is not here

| you may look for | |
|---|---|
| `return`, `break`, `continue`, `for`, `while` | a block's last expression; `loop` and `again` (§0.3, §0.7) |
| recursion | refused; a `loop` (§0.7) |
| `null`, `nil`, `undefined` | a variant: `'some(v)` or `'none` (§0.8) |
| exceptions, `try`/`catch`/`finally` | a failure is a value (§0.10) |
| integer overflow, `int32`, `uint8`, `BigInt`, casts | `Int` is ℤ; a range is a set, proven (§0.4, §0.5) |
| implicit conversions, truthiness | none: an `if` takes a `Bool` |
| `NaN`, `Infinity`, `-0` | none: a number is a number (§0.5) |
| an `assert` that runs | a claim, proven when compiled (§0.4) |
| generics, `<T>`, interfaces, traits | a definition is specialised where it is used (§0.6) |
| classes, methods, inheritance, `this` | functions in modules; data is tables, tuples, records and variants |
| closures at run time | refused; a λ a host applies during its call is allowed (§0.6) |
| mutable arrays, shared mutable state | `var`, local to a block (§0.8) |
| macros, annotations, pragmas, doc comments | none; a comment is erased and means nothing |
| a string library (split, upper, format) | the host's, by name (§0.13); the language's own are `++`, `string_of` and `decimal` |
| operator overloading | none |
| a garbage collector or runtime of its own | none (§0.1) |

**Specified, and not built yet**, so not in this chapter: the logic's own definitions (`spec`, and
`lemma` proven by induction), a guard on a `case` arm, handlers on `try`, a function a host
returns, and concurrency. [spec/j.md](../spec/j.md)'s status table is the exact list.

## 0.15 From what you write to what you write here

| elsewhere | here |
|---|---|
| `let x = e`, `const x = e`, `x := e` (Go) | `x = e` |
| `x = e`, an assignment | `x := e`, with `x` declared `var x := …` |
| `return e` | `e`, last in the block |
| `fn f(x: T) -> R`, `func f(x T) R` | `fn f(x ∈ T) (R)` |
| `x: u8`, `x uint8` | `x ∈ 0..255` |
| `x: i64`, `x int64` | `x ∈ -2^63..2^63-1` |
| `big.Int`, `BigInt`, `BigInteger` | `Int`: nothing to write |
| `assert(n > 0)` at the top of a function | `where n > 0` in its header, proven at each call |
| `assert(φ)` in a body | `assert φ`, proven when compiled |
| `c ? a : b` | `if c { a } else { b }` |
| `if … else if … else …` | the same, or `case { φ -> a; else -> b }` |
| `for (i = 0; i < n; i++) acc += i` | `loop i = 0, acc = 0 { i < n -> again(i + 1, acc + i); else -> acc }` |
| `while (c) { … }` | `loop … { c -> { …; again(…) }; else -> … }` |
| `[1, 2, 3]` | `array { 1, 2, 3 }` |
| `[f(k) for k in range(n)]` | `array { f(k) : k ∈ 0..n-1 }` |
| `Vec<T>`, `[]T`, `T[]` | `Array(T)` |
| `a[i]`, `len(a)` | `a[i]`, `len(a)`, the index proven |
| `enum E { A, B(T) }` | `set E = 'a ∪ 'b(T)` |
| `match e { … }`, `switch` | `case e { 'a -> …; 'b(x) -> … }` |
| `Option<T>`, `T?` | `'some(v)`, `'none` |
| `e?` (Rust) | `try 'ok(x) = e { … }` |
| `e.unwrap()`, `panic("why")` | `expect 'ok(x) = e else "why"` |
| `s + t` on strings | `s ++ t` |
| `strconv.Itoa(n)`, `str(n)`, `n.toString()` | `decimal(n)` |
| `a ^ b`, xor | `a ⊕ b`; `2^10` is 1024 |
| `(a * b) & 0xFFFFFFFF`, or a `uint32` | `(a * b) mod 2^32` |
| `import m`, `import m as a` | `use m`, `use m as a` |
| `pub`, `public`, a capital letter (Go) | `pub` |
| `main`, `#[no_mangle]`, `module.exports` | `export` |
| `x.method(y)` | `module.method(x, y)` |
| `// comment` | `// comment` |

---

## What to remember

- **It looks like what you know**, and compiles to what you would have written by hand, on each of
  four hosts, with no runtime of its own.
- **`=` defines, `:=` writes, `==` compares.** A block's value is its last expression; there is no
  `return`.
- **A type is a set**, `x ∈ 0..255`, and a header's sets, `where`, `ensure` and a body's `assert`
  are claims the compiler proves or refuses. Nothing is cast, and no claim runs.
- **An integer is an integer.** `Int` is ℤ, computed exactly at any size, and the compiler chooses
  a word or the host's big integer value by value. A value is refused where it must be held in a
  set it is not proven in: a result's range, a table's element, a host's word; `-checked` traps
  there instead. `0.1` is one tenth.
- **Functions, variants, tuples, `Result`s and tables written by a rule are how you write the
  program, not what runs.** Reduction removes them; a function that would outlive its call is
  refused. A large first-order definition may be kept as one function, where the compiler finds
  that cheaper, and that changes no answer.
- **`loop` and `again` are the only iteration**, there is no recursion, and termination is proven.
- **State is `var`, local to a block.** A table is a value, and an index is proven in range.
- **A failure is a value**, and a `relevant` one cannot be dropped silently.
- **`pub` is for other modules, `export` is for the host.** A host's library is used under its own
  names, and how portable that leaves a program is computed.

Next: [`fn`](01-fn.md), and how reduction does all of this. From there the book goes one brick at a
time, each chapter using only what the ones before it introduced; [the book's README](README.md)
lists the chapters and says which are written.
