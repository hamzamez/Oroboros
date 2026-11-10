# 0. A tour

This chapter is the whole language at walking pace, for someone who already programs in Go, Python,
JavaScript, Java, C or Rust. Most of what you will see looks like what you know: braces, `fn`,
`f(x)`, `t[i]`, infix arithmetic, `//` comments, `if` and `else`. The chapter's job is to point at
the places where it only *looks* that way, and at the things you will reach for that are not here.

Each section says what is **like** the languages you know and what is **unlike** them, and shows the
unlike part failing before it shows it working, because the compiler's refusals are most of what is
unfamiliar.

**Everything printed below is real.** Each source is a file in [code/tour/](code/tour/); each
emitted function, each refusal and each program's output is what the compiler gave, and
`go run docs/book/check.go docs/book/00-tour.md` re-runs all of it and compares. Checked
2026-11-10.

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
export fn sum_to(n ∈ 0..65536) (Int) {
    loop acc = 0, i = 0 {
        i < n -> again(acc + i, i + 1)
        else  -> acc
    }
}
```

Read the header aloud: *sum_to takes an n in the set 0 to 65536 and gives an integer.* The body is
one loop with two variables. Now compile it, without building, to see what a host receives:

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
define i64 @tour_sum_to(i64 %v0) #0 {
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
  were proven to stay inside every target's machine word, so no overflow check was emitted and none
  is needed. LLVM's `nuw nsw` flags are that proof, handed to the optimizer (§0.5).
- **There is no `for`.** `loop` with `again` is the only iteration, and it was *proven
  terminating* (§0.7).

The targets are Go, Windows, Android and the browser
([ADR 0039](../decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)); Java is how
Android is reached, and JavaScript the browser.

## 0.3 A block's value is its last expression

<!-- file docs/book/code/tour/blocks.oro -->
```go
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
```

**Like:** braces delimit a block; `if`/`else if`/`else`; `==`, `!=`, `<`, `<=`, `&&`, `||`, `!` mean
what they mean in C; arithmetic has the usual precedence.

**Unlike:**

- **`x = e` is a definition**, as in mathematics: *let x be e*. There is no `let`, `const` or `val`
  keyword in front of it, and it is not an assignment. Writing to something is a different
  statement with a different symbol, `:=` (§0.8).
- **There is no `return`.** A block's value is its last expression, as in Rust or Ruby, and a
  function's value is its body's. Nothing leaves a function early.
- **`if` is an expression**, with a value, as Rust's is and C's `?:` is. `grade` has no temporary.
- **`case { … }` with no subject is a chain of guards**: the first true guard's arm is the value,
  and `else` is the last arm. It is the same thing as the `if` chain, written as a table.
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

The sets you will write:

| set | its members |
|---|---|
| `Int` | the integers |
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
export fn clamp(n ∈ 0..1000) (0..255) { if n > 255 { 255 } else { n } }

export fn abs(n ∈ -1000..1000) (r ∈ Int) ensure r >= 0 {
    if n < 0 { 0 - n } else { n }
}
```

<!-- gen go docs/book/code/tour/clamp.oro -->
```go
func TourClamp(v0 int) int {
    v10 := (v0 > 255)
    if v10 {
        return 255
    }
    return v0
}
```

No check was emitted, because none is needed: the claim was proven and then erased. `abs` names its
result `r` so that `ensure` can speak about it. Change `r >= 0` to `r > 0` and it is refused the
same way, *the body does not establish result > 0; its result is [0, 1000]*, because `abs(0)` is 0.

Inside a body almost nothing is annotated. `aa = a * a` has no type written on it and needs none;
when you do want to state a set for a local, the binder takes one, `end ∈ 0..n = t + got`, and that
too is proven or refused.

(A `-` directly before a literal is part of the literal, `-1`. Negating a variable is written
`0 - n` today.)

## 0.5 An integer is an integer

This is the largest difference from every host, and it is one sentence: **`Int` means ℤ, and an
arithmetic operation either computes the integer it means or does not compile**
([ADR 0026](../decisions/0026-an-int-is-an-integer.md)).

<!-- file docs/book/code/tour/no-square.oro -->
```go
export fn square(n ∈ Int) (Int) { n * n }
```

<!-- gen go docs/book/code/tour/no-square.oro -->
```
gen: tour-square: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  * [-inf, +inf] in n * n
```

In Go, Java and C this function compiles and wraps; in JavaScript it compiles and loses precision
past 2⁵³. Each of those is a different wrong integer. Here there are three ways to say what you
meant, and each is visible in the source or on the command line.

**Narrow the set**, so the product provably fits a machine word. **Or claim a set above the word**,
which is the consent to exact arithmetic:

<!-- file docs/book/code/tour/square.oro -->
```go
export fn small(n ∈ -1000..1000) (Int) { n * n }

export fn wide(n ∈ 0..2^63-1) (0..2^126) { n * n }
```

<!-- gen go docs/book/code/tour/square.oro -->
```go
func TourSmall(v0 int) int {
    v9 := (v0 * v0)
    return v9
}
…
func TourWide(v0 int) *big.Int {
```

`small` is one multiplication on an `int`. `wide` takes an `int` and returns Go's `*big.Int`,
because its declared result does not fit a word. **The source names no representation.** A set is a
claim, and the facts the compiler proves choose `int`, `uint64`, or the host's big integer for each
value ([ADR 0079](../decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md)).
`^` is a power of constants, written in a set's ends; xor is `⊕` (§0.12).

**Or ask for the trap**: `-checked` emits the host's own checked arithmetic, and the program fails
at run time instead of at compile time.

<!-- gen go docs/book/code/tour/no-square.oro -checked -->
```go
func TourSquare(v0 int) int {
    v1 := (func(a, b int) int { t := a * b; if a != 0 && t/a != b { panic("int overflow") }; return t }(v0, v0))
    return v1
}
```

### Which word, and so which targets

The word is each target's own: 64 bits on Go, the JVM and Windows, and ±(2⁵³ − 1) on JavaScript,
whose numbers are doubles. So whether a program is legal is a fact about the program *and* the
target, and it is computed, not promised:

```bash
go run ./cmd/portable docs/book/code/tour/square.oro
```

<!-- portable docs/book/code/tour/square.oro -->
```
portable to go, java, windows — not js
```

JavaScript refuses `wide`, because its parameter can exceed 2⁵³ before anything is computed. Narrow
the parameter to `0..2^53-1` and the program is portable to all four. **Portability is a property a
program has or lacks, and the compiler tells you which.**

### Numbers with a decimal point

<!-- file docs/book/code/tour/exact.oro -->
```go
export fn exact() (Bool) { 0.1 + 0.2 == 0.3 }

export fn tax(cents ∈ 0..10^9) (Int) { round(Int, cents * 0.0825, 'half_away) }
```

<!-- gen js docs/book/code/tour/exact.oro -->
```javascript
export function tourExact() {
    return ((1 + 2) === 3);
}
```

**Like:** the literals look the same. **Unlike:** `0.1` is one tenth, exactly
([ADR 0086](../decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md)).
A numeral is a decimal fraction, `+`, `-`, `*` and the comparisons on them are exact, and the
compiler holds each as an integer numerator at a known scale: one tenth plus two tenths is three
tenths, even in JavaScript, where `0.1 + 0.2 === 0.3` is `false`.

Precision is lost in exactly one place, and you write it: `round(S, e, mode)` rounds `e` onto the
set `S`. `tax` is 8.25% of an amount in cents, rounded to the cent, a tie going away from zero, and
it compiles to integer arithmetic with no double anywhere. `F64` is a set like the others; a
rounding onto it is a host's floating-point operation. `/` is integer division and needs integer
operands.

## 0.6 Functions vanish

<!-- file docs/book/code/tour/twice.oro -->
```go
fn twice(f, x) { f(f(x)) }

export fn add6(n ∈ 0..100) (Int) { twice(fn(x) { x + 3 }, n) }

export fn shout(s ∈ String) (String) { twice(fn(x) { x ++ "!" }, s) }
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

- **Only `export`ed functions exist in the output.** `twice` is gone, and so is the anonymous
  function. The compiler works by *reduction*: it substitutes definitions and applies functions
  until only the target's own operations remain. A function you define is a way of writing a
  program, not a thing that is called at run time.
- **`twice` has no types, and was used at two.** There are no generics, no type parameters and no
  `<T>`: a definition with unannotated parameters is specialised wherever it is used, and what
  reaches the type checker is already first-order and monomorphic. Annotate the functions you
  `export`, since a host must know their sets, and the rest as far as you want their claims checked.
- **Higher-order code costs nothing**, for the same reason: there is no closure to allocate and no
  indirect call. The price is the next refusal.

<!-- file docs/book/code/tour/no-closure.oro -->
```go
export fn adder(n ∈ 0..10) { fn(x) { x + n } }
```

<!-- gen go docs/book/code/tour/no-closure.oro -->
```
gen: tour-adder: lowering: a λ in value position is a closure (callbacks.md): fn(x) { x + #1.0 }
```

**A function may not survive to run time.** Returning one to the host, or storing one in a table,
would need a closure, a hidden allocation, and hidden allocation is what this language exists to
avoid. (The `#1.0` in
that message is the compiler's internal name for `n`, a wart in the diagnostic.) Handing a function
to a host API that takes a callback is specified and not built.

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

export fn gcd(a ∈ 0..10^6, b ∈ 0..10^6) (Int) {
    loop x = a, y = b {
        y == 0 -> x
        else   -> again(y, x % y)
    }
}

export fn main() {
    io.print_int(gcd(1071, 462))
    io.print_line(big_str(fact(30)))
}
```

<!-- run go docs/book/code/tour/fact.oro -->
```
21
265252859812191058636308480000000
```

Read a `loop` as a table. `loop i = 1, acc = 1` names the variables and their first values. Each
arm is `guard -> what happens`, tried in order:

- **`again(e₁, e₂)`** goes round, with the variables' next values, by position. All of them are
  given at once, so there is no half-updated state: `again(y, x % y)` is Euclid's step and needs no
  temporary.
- **An arm without `again` leaves the loop, and its value is the loop's value.** That is `break`,
  with a result.

**Unlike** a `while` loop, the variables are not assigned to; they are *rebound* each time round,
as a recursive function's parameters would be. A `loop` is the tail-recursive function you were
going to write, with the recursion spelled `again`. Two consequences:

- **The compiler proves termination** where it can (*1 of 1 loop(s) proven terminating*, by
  size-change analysis), and reports the loops it cannot.
- **`again` stands only where the loop's turn ends**: as an arm's value, or last in an arm's block.
  It may not sit under an `if`:

<!-- file docs/book/code/tour/no-again.oro -->
```go
export fn skip(n ∈ 0..10) (Int) {
    loop i = 0 {
        i < n -> if i == 5 { again(i + 2) } else { again(i + 1) }
        else  -> i
    }
}
```

<!-- gen go docs/book/code/tour/no-again.oro -->
```
gen: docs/book/code/tour/no-again.oro: line 3: `again` under an if (ADR 0015)
```

The condition belongs in the guards: `i < n && i == 5 -> again(i + 2)`, then `i < n ->
again(i + 1)`. A loop's arms are where its decisions are written.

And look at `fact`'s result, `Nat`. Thirty factorial does not fit a word; the claim asked for the
natural number, and the program printed it. `big_str` is the language's rendering of an integer of
any size as text.

## 0.8 State, and tables

A definition never changes. When something must change, it is declared as state, with `var`, and
written with `:=`:

<!-- file docs/book/code/tour/cells.oro -->
```go
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
```

**Like:** a mutable local, updated in a loop, in program order. `t[i]` indexes, `len(t)` is the
length, indices start at 0. An `if` with no `else` is a statement.

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
primes = array { 2, 3, 5, 7, 11 }

export fn third_prime() (Int) { primes[2] }

export fn sum_squares(n ∈ 0..1000) (Int) {
    sq = array { k * k : k ∈ 0..n-1 }
    loop i = 0, acc = 0 {
        i < n -> again(i + 1, acc + sq[i])
        else  -> acc
    }
}
```

- **By listing it**: `array { 2, 3, 5, 7, 11 }`.
- **By a rule**: `array { k * k : k ∈ 0..n-1 }` is the table whose entry k is k², for k from 0 to
  n − 1. It is set-builder notation, and it is Python's `[k * k for k in range(n)]`.
- **By filling a buffer**: `var s := array { … }` makes a table you may write, element by element,
  until the block ends; then it is a value like any other.

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
read `sq[i]` applied it, and no array was ever allocated. Likewise `third_prime` compiles to
`return 5`. Writing the intermediate structure is free when reduction can see through it.

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
export fn third(t ∈ Array(Int)) (Int) where len(t) >= 3 { t[2] }

export fn third_or_zero(t ∈ Array(Int)) (Int) {
    if len(t) > 2 { t[2] } else { 0 }
}
```

A loop's guard is such a test, which is why `t[i]` under `i < len(t)` in `count_even` needed
nothing said. In practice most indexing is like that, and proves itself.

The same proof covers arithmetic on what a table holds. This is refused,

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
  + [0, 2351959869397967831040] in acc + t(i)
```

because a table may be as long as memory allows, and that many bytes sum past 2⁶³. The compiler
computed the bound and showed it. Adding `where len(t) <= 2^20` to the header
([prefix.oro](code/tour/prefix.oro)) makes it *3 of 3 integer operations bounded*: the limit you
were assuming is now written down, and every caller is held to it.

Maps follow the same idea. `map { 200 : 1, 404 : 2 }` is a map written out, `var m := map(cap)` one
you fill, and a read `m[k]` cannot be assumed to find its key, so its value is `'some(v)` or
`'none` and you must say what happens in each case ([maps.oro](code/tour/maps.oro)).

## 0.9 Variants and `case`

<!-- file docs/book/code/tour/shapes.oro -->
```go
pub set Shape = 'circle(Int) ∪ 'square(Int) ∪ 'point

pub fn area(s) {
    case s {
        'circle(r) -> 3 * r * r
        'square(w) -> w * w
        'point     -> 0
    }
}

fn helper(n) { n + 1 }
```

**Like:** Rust's `enum` with `match`, Swift's enums, TypeScript's tagged unions, a sealed class
hierarchy. A value of `Shape` is one of three alternatives, and `case` takes it apart, binding the
payload.

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
use shapes

export fn square_area(side ∈ 0..1000) (Int) { shapes.area(shapes.'square(side)) }

export fn pick(n ∈ 0..1000) (Int) {
    shapes.area(if n < 10 { shapes.'point } else { shapes.'circle(n) })
}
```

<!-- gen go docs/book/code/tour/pick.oro -->
```go
func TourPick(v0 int) int {
    v10 := (v0 < 10)
    if v10 {
        return 0
    }
    v15 := (3 * v0)
    v16 := (v15 * v0)
    return v16
}
…
func TourSquareArea(v0 int) int {
    v9 := (v0 * v0)
    return v9
}
```

No tag, no `switch`, no allocation. In `square_area` the tag was known at compile time and the
`case` reduced away. In `pick` the tag was decided by an `if`, and the `case` was pushed into its
branches, so what is left is the `if` that decided it.

## 0.10 A failure is a value

There are no exceptions and there is no `null`. A computation that may fail returns a variant that
says so. The library's is `Result`, and it is the whole of
[lib/result.oro](../../lib/result.oro):

```go
pub set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant
```

<!-- file docs/book/code/tour/half.oro -->
```go
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
```

**Like:** Rust's `Result`, Go's `(value, err)`, Swift's `throws`. The three ways to meet a failure
are the three you know:

| here | elsewhere | what it does |
|---|---|---|
| `case r { 'ok(v) -> …; 'err(e) -> … }` | `match`, `if err != nil` | handle both |
| `try 'ok(v) = e { body }` | Rust's `?` | go on with `v`; hand any failure back unchanged |
| `expect 'ok(v) = e else "why"` | `unwrap`, `panic` | go on with `v`; crash with the reason |

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

## 0.11 Tuples

<!-- file docs/book/code/tour/tuples.oro -->
```go
fn divmod(a, b) { (a / b, a % b) }

export fn clock(seconds ∈ 0..86399) (String) {
    (m, s) = divmod(seconds, 60)
    (h, min) = divmod(m, 60)
    string_of(48 + h / 10) ++ string_of(48 + h % 10) ++ ":" ++
        string_of(48 + min / 10) ++ string_of(48 + min % 10)
}
```

**Like:** Python's and Rust's tuples, Go's multiple results: `(a, b)` makes one, `(m, s) = e` takes
one apart, `()` is the empty one, which a block ending in a statement has as its value.

**Unlike:** a tuple is not allocated. In what `clock` compiles to the pairs are gone, and `s`,
which it never uses, is never computed. A line that ends in an operator continues on the next.

Text is small on purpose. A `String` is a sequence of Unicode characters, `++` joins two, and
`string_of(n)` is the one-character string of code point n. Everything richer is the host's, under
the host's name (§0.13).

## 0.12 Bits

<!-- file docs/book/code/tour/bits.oro -->
```go
export fn clear?(flags ∈ 0..255, mask ∈ 0..255) (Bool) { flags & mask == 0 }

export fn fnv32(src ∈ Array(0..255)) (0..2^32-1) {
    loop i = 0, h = 0x811C9DC5 {
        i < len(src) -> again(i + 1, ((h ⊕ src[i]) * 16777619) mod 2^32)
        else -> h
    }
}
```

**Like:** `&`, `|`, `~`, `<<`, `>>`, hexadecimal literals.

**Unlike:**

- **`flags & mask == 0` means `(flags & mask) == 0`.** In C, Java, JavaScript and Go's ancestors
  `==` binds tighter than `&`, and that line tests the wrong thing. Here every arithmetic and bit
  operator binds tighter than every comparison.
- **Xor is `⊕`**, typed `\oplus`. `^` is the power, as in mathematics, and takes constants.
- **Nothing wraps unless you say so.** There is no `uint32` whose multiplication silently drops
  bits. FNV-1a's step is *xor, multiply, keep the low 32 bits*, and the last part is written:
  `mod 2^32`. The compiler then computes the whole step in the host's wrapping arithmetic, because
  under `mod 2ⁿ` that is correct
  ([ADR 0082](../decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md)):

<!-- gen go docs/book/code/tour/bits.oro -->
```go
            v13 := (v5 ^ v12)
            v15 := (v13 * 16777619)
            v17 := (v15 & 4294967295)
```

This function is from a real program, [examples/hash/fnv.oro](../../examples/hash/fnv.oro), whose
output equals Go's `hash/fnv` on three hosts.

## 0.13 Modules, and the host

`shapes.oro` in §0.9 is a module because it is a file. Another file uses it by its path:

<!-- file docs/book/code/tour/room.oro -->
```go
use shapes
use io

export fn floor(side ∈ 0..1000) (Int) { shapes.area(shapes.'square(side)) }

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

- **`pub`: may another module name this?** Private is the default. `shapes.helper` has no `pub`:

<!-- gen go docs/book/code/tour/no-private.oro -->
```
gen: docs/book/code/tour/no-private.oro: f: module shapes defines helper and does not make it `pub`: a declaration is private to its module unless marked `pub` (ADR 0078)
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

## 0.14 What is not here

| you may look for | |
|---|---|
| `return`, `break`, `continue`, `for`, `while` | a block's last expression; `loop` and `again` (§0.3, §0.7) |
| recursion | refused; a `loop` (§0.7) |
| `null`, `nil`, `undefined` | a variant: `'some(v)` or `'none` (§0.8) |
| exceptions, `try`/`catch`/`finally` | a failure is a value (§0.10) |
| integer overflow, `int32`, `uint8`, casts | a set, proven (§0.4, §0.5) |
| implicit conversions, truthiness | none: an `if` takes a `Bool` |
| `NaN`, `Infinity`, `-0` | none: a number is a number (§0.5) |
| generics, `<T>`, interfaces, traits | a definition is specialised where it is used (§0.6) |
| classes, methods, inheritance, `this` | functions in modules; data is tables, tuples and variants |
| closures at run time | refused (§0.6) |
| mutable arrays, shared mutable state | `var`, local to a block (§0.8) |
| macros, annotations, pragmas, doc comments | none; a comment is erased and means nothing |
| a string library (split, upper, format) | the host's, by name (§0.13); the language's own are `++` and `string_of` |
| operator overloading | none |
| a garbage collector or runtime of its own | none (§0.1) |

**Specified, and not built yet**, so not in this chapter: records (a `struct`), the logic's own
definitions (`spec`, `lemma`, `assert`, `∀` and `∃` in a claim), a guard on a `case` arm, handlers
on `try`, callbacks into a host, and concurrency. [spec/j.md](../spec/j.md)'s status table is the
exact list.

## 0.15 From what you write to what you write here

| elsewhere | here |
|---|---|
| `let x = e`, `const x = e`, `x := e` (Go) | `x = e` |
| `x = e`, an assignment | `x := e`, with `x` declared `var x := …` |
| `return e` | `e`, last in the block |
| `fn f(x: T) -> R`, `func f(x T) R` | `fn f(x ∈ T) (R)` |
| `x: u8`, `x uint8` | `x ∈ 0..255` |
| `assert(n > 0)` at the top of a function | `where n > 0` in its header, proven at each call |
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
- **A type is a set**, `x ∈ 0..255`, and a header's sets, `where` and `ensure` are claims the
  compiler proves or refuses. Nothing is cast.
- **An integer is an integer.** An operation that might leave the machine word does not compile
  until you narrow a set, claim one above the word, or ask for `-checked`. `0.1` is one tenth.
- **Functions, variants, tuples, `Result`s and tables written by a rule are how you write the
  program, not what runs.** Reduction removes them; a closure that would survive is refused.
- **`loop` and `again` are the only iteration**, there is no recursion, and termination is proven.
- **State is `var`, local to a block.** A table is a value, and an index is proven in range.
- **A failure is a value**, and a `relevant` one cannot be dropped silently.
- **`pub` is for other modules, `export` is for the host.** A host's library is used under its own
  names, and how portable that leaves a program is computed.

Next: [`fn`](01-fn.md), and how reduction does all of this. Chapters 1 to 5 are still written in
the notation this surface replaced, and are being brought up to date in order; until each is, read
its examples for their content and this chapter for the spelling.
