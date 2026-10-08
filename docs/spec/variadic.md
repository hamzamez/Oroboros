# Variadic declarations and `spread`

Status: built 2026-10-08 (variadic-2026-10-08, ADR 0046).

## 1. What a variadic is

A host function `f(fixed…, rest ...T)` is a function of a **list**: f : X × T* → B, where T* = ⋃ₙ Tⁿ is
the free monoid on T. Go's spec says so in two sentences ("Passing arguments to ... parameters"):

- within f, the parameter's value is a `[]T`, and a call with arguments passes "a new slice of type
  []T … whose successive elements are the actual arguments";
- "if the final argument is assignable to a slice type []T and is followed by `...`, it is passed
  unchanged".

So a call with arguments a₁ … aₙ is f applied to the **word** a₁ ⋯ aₙ, and a call with `xs...` is f
applied to a given word. Both are applications of the one function. Neither is overloading, and
neither needs a second name.

## 2. The declaration

A declaration marks the host function variadic, and its last parameter is the list, a table:

```lisp
(sig Println ((a (array go.Value))) any variadic (host stmt "fmt.Println(%s...)" (import "fmt")))
(sig NewReplacer ((oldnew (array bytestring))) Replacer pure variadic
  (where (= (% (len oldnew) 2) 0))
  (host expr "strings.NewReplacer(%s...)" (import "strings")))
```

`variadic` is a claim about the operation, so it is a `sig` word, beside `pure`. The loader checks
the shape:
- a `variadic` declaration's last parameter is `(array T)` and its template's last hole is `%s...`,
  Go's spread;
- a template that spreads its last hole is declared `variadic`. A spread is how a host spells a
  variadic call, so one without the other is refused.

The host check compares the declaration with the manifest's own line, `...T` read as `(array T)`
(gauntlet/stdlib/tooling_test.go).

## 3. The call

For a declaration with k fixed parameters:

```
(f x₁ … xₖ a₁ … aₙ)          ⟶  (f x₁ … xₖ (array a₁ … aₙ))     n ≥ 0
(f x₁ … xₖ (spread xs))       ⟶  (f x₁ … xₖ xs)
```

- **The word is a table literal written at the call**, so its element type is the declared T, as for
  any literal at a boundary (literal-elements.md): `(fmt.Println 42 "x")` is Go's `[]any{42, "x"}`,
  each value entering the box (types.md §3.3).
- **`(spread xs)` passes a table you have**, Go's `xs...`. It is the only argument after the fixed
  ones. The marker is needed because a table is itself one value of `...any`: `(fmt.Println xs)`
  prints the table, and `(fmt.Println (spread xs))` prints its elements, exactly as Go's
  `fmt.Println(xs)` and `fmt.Println(xs...)` differ.
- **Refused**: fewer than k arguments; `spread` beside other variadic arguments, or among the fixed
  ones; `spread` at a call that is not variadic, or outside a call; a definition named `spread`,
  since the word is the language's.

**Where it happens: at loading, once, where names resolve.** The target hands the loader one
`variadic` form per declaration (with its fixed count), and after resolution every call to that name
in the program's definitions and terms is rewritten. Nothing below the loader learns that variadics
exist: the reducer, the checker and the IR see a table argument. Loading is the right stage, and
not a step a driver remembers, because forgetting would be a wrong answer rather than a refusal: a
table passed where a single value of `...any` belongs would print its elements.

**Not covered:** a variadic used as a value and applied later, `(let p fmt.Println (p a b))`. The
rewrite reads the call's head as written. Such a call reaches the checker with the wrong arity and is
refused.

## 4. What is emitted

The printer emits the host's own call when the table is written at the call and read only there:
`fmt.Println(v1, v2)`, not `v := []any{v1, v2}; fmt.Println(v...)`. It is the same value (Go's spec
builds the slice from the arguments), and `-gcflags=-m` shows Go treating the two alike. A table
passed with `spread` is passed as `xs...`.

## 5. On each target

| target | what it does |
|---|---|
| Go | the spread `f(xs...)`, and Go's own call for a literal |
| Java | varargs are an array (`Object... a` is an `Object[]`); declared when a program needs one |
| JavaScript | rest parameters are an array, `f(...xs)`; declared when a program needs one |
| Windows | C's `...` (`printf`) has no list value; a declaration would be per arity, the restrictions |
