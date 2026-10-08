# 0046 — A variadic is one declaration under the host's name, and a call's arguments are its list

Date: 2026-10-08
Status: Accepted (hamza: "go ahead with (spread xs), always mathematical and algebraic"). Supersedes
ADR 0045's decision 3, the NAME-All names, and the per-arity restrictions before it.

## Context

ADR 0045 declared each `fmt` function twice: for one value under Go's name, and for any number as
`PrintlnAll`, a table of boxes. hamza asked whether that was right: a user reads Go's documentation and
should find the same name in our target. It was not. `fmt.oro`'s first line says "under Go's names",
and ADR 0025 makes a module's path Go's so that Go's documentation is ours. `PrintlnAll`, like
`Println2` before it, is a name Go does not have.

Go's spec defines a variadic call in two sentences. Within f, the final parameter `...T` has type `[]T`.
A call with arguments passes "a new slice of type []T … whose successive elements are the actual
arguments", and `xs...` passes xs unchanged. So a variadic is f : X × T* → B, a function of a list
(T* the free monoid on T), and a call is f applied to the word its arguments spell. G and H had
declared the right function. What was missing was the call Go has, not a second name.

## Decision

1. **One declaration per host function, under the host's name, marked `variadic`**: its last
   parameter is the list, `(array T)`, and its template spreads it, `%s...`. The loader refuses a
   `variadic` declaration of another shape, and a spreading template not marked `variadic`.
2. **A call's trailing arguments are the list**, as Go's spec builds it:
   `(f x₁ … xₖ a₁ … aₙ)` ⟶ `(f x₁ … xₖ (array a₁ … aₙ))`, a literal at the call, so the declared element
   decides (`[]any{…}`, each value entering Go's box). `(spread xs)` passes a table one has, Go's
   `xs...`; it is the only argument after the fixed ones. It is needed because a table is itself one
   value of `...any`: `(fmt.Println xs)` prints the table, and `(fmt.Println (spread xs))` its elements.
3. **The loader rewrites, once, from the target's forms**: the target hands one `variadic` form per
   declaration to the loader through `Target.Defs`, which every driver passes, and the rewrite runs
   after resolution over every definition and entry term. Nothing below learns that variadics exist.
   **`Target.Env` refuses a program that mentions a variadic declaration the loader did not read**,
   because a program loaded without the target's forms would take `(fmt.Println xs)`'s table for the
   list: a wrong answer, made a refusal.
4. **A variadic statement's value is the unit.** A `stmt` declaration forgets the host's results, and
   its convention, "its value is argument 0", would make `(fmt.Println x)` the list. What remains of a
   computation whose results are forgotten is 1.
5. **The printer writes the host's own call** for a list written at the call and read only there:
   `fmt.Println(a, b)`, not `v := []any{a, b}; fmt.Println(v...)`. They are one value by Go's spec, and
   `-gcflags=-m` shows Go treating the two alike.
6. **An unknown name meeting a demand takes the demanded type only where nothing else may stand
   there.** The checker's inference half bound a name to a demanded box, so `(fmt.Println n)` made n a
   `go.Value` and `(+ n 1)` was refused. A demand with a type below it (an interface, the empty
   interface) teaches nothing about the name.

## Consequences

- `fmt.oro` is 13 declarations, one per Go function, where it had 30 and then 26. `NewReplacer`,
  `MultiReader` and `MultiWriter` are called as in Go's documentation:
  `(strings.NewReplacer "a" "1" "b" "2")`.
- Emission: eight changes, each Go's own call where the `[]any` spread was, `return 0` where a function's
  value was a print (the unit; the program wrapper never reads it), and IR value names shifted by the
  list and the unit. `report-go.oro` prints the same three lines.
- Seven planted faults, one per rule, each caught (variadic-2026-10-08 Part 3).
- A table passed with `spread` must already have the list's element type: Go's slices are not
  covariant, so a table of strings is no list of boxes, and a table whose evenness is unknown does not
  meet `NewReplacer`'s precondition. A definition that hands its table on declares the precondition,
  and its callers discharge it.

## Why not

- **`PrintlnAll` beside `Println`** (ADR 0045): a name Go does not have. A reader of Go's docs does not
  find it.
- **`Println` as the table form only, `(fmt.Println (array x))`**: Go's name, no language work, and
  the common call worse than Go's at 220 sites in the corpus.
- **Keying primitives by (name, arity)** (overloading.md §3): a family per arity again, with a wall at
  the last one declared. The algebra says the arguments are one list.
- **Go's spelling `xs...`**: `...` would have to become a name, and a suffix marker on an argument is
  a new kind of syntax. `(spread xs)` is an application like every other form.
- **The rewrite at reduction**: the reducer normalises a term more than once, so the rewrite would have
  to mark what it rewrote; and a variadic applied through a variable would need a type the reducer does
  not have. At loading the call's head is the name as written, once.
- **A variadic statement's value as its list**: it means nothing, and it made a function whose value is
  a print fail the IR's table classes.
