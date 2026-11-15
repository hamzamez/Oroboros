# 6. Integers

> **Not written yet.** This is the place of the sixth brick, integers: ℤ, a range as a set, the
> machine word, an operation proven inside it or refused, and what lies above it. Until it is
> written, [chapter 0 §0.4 and §0.5](00-tour.md) are the account of those.
>
> One section is here already, moved from chapter 1 when the book was reordered (2026-11-15),
> because it uses sets and ranges, which chapter 1 had not introduced. It is checked like any
> other: `go run docs/book/check.go docs/book/06-integers.md`.

---

## A named function carries claims

Everything so far was a function alone. Given a name at a file's top level, a function may also
carry a **header**: what its parameters are, what its result is, and what must hold between them.
These examples compile for a real target, Go, since the claims are about real integers.

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
place it was called, `n <= 9` must be provable, and all that is known is `0 <= n <= 100`. Make it
provable, and the claim costs nothing:

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

**An entry point's parameters need their sets.** A function used only inside the program takes its
arguments' sets from each use. An `export` is called from outside, where nothing is known:

<!-- file docs/book/code/integers/no-untyped.oro -->
```go
export fn inc(n) { n + 1 }
```

<!-- gen go docs/book/code/integers/no-untyped.oro -->
```
  + [-inf, +inf] in n + 1
```

With no set, `n` is any integer, and `n + 1` cannot be shown to fit a machine word (chapter 0, §0.5).

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

How those claims are proven is [chapter 15](15-claims.md). Where they live: **on the function, in its header, and nowhere else.** An expression carries no type
annotation.
