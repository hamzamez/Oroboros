# 0045 — A host's empty interface is a type, and a variadic over it is a table of boxes

Date: 2026-10-08
Status: Accepted (hamza: "go with c, it is more honest"; the language's `any` kept as it is). Extends
ADR 0021's interfaces with the one derived subsumption edge, and closes gocoverage-2026-10-06's H.
Decision 3's NAME-All names are superseded by ADR 0046: one declaration under Go's name.

## Context

Go's `fmt.Append(b []byte, a ...any) []byte`, and every `fmt.Print*`, take `...any`. The language's
`any` is not a type: it is the absence of a claim (target-files.md §2), consistent with every type in
both directions, so `(a any)` is filled in at each call by the argument's own type. A declaration over
it is a template with a fixed number of holes, so `fmt.oro` declared each function three times
(`Println`, `Println2`, `Println3`) with a wall at four operands. A table of `any` has no element type:
the IR refused `(array any)` as *not final*.

Go's `any` is a different thing, the **empty interface**: ∃X. X, a box holding a value and its type.
A value of it has one type, so `...any` is (∃X. X)*, which Go represents as `[]any`. G had just shown
that a variadic over one type is a function of a table, spread into the call (variadic-2026-10-08).

## Decision

1. **Go's empty interface is a host type**, `go.Value`, realized as Go's `any`. The language's `any`
   keeps its meaning and its name.
2. **It is the top of the host's interface order, derived.** An interface with no methods is
   satisfied by every type vacuously, methods(I) = ∅ ⊆ methods(T), so the derivation ADR 0021 declines
   for interfaces in general is trivial here. A target declares which type its empty interface is,
   `(implements any go.Value)`, and the edge is then derived for **every type held as one host
   value**: an integer, a float, a boolean, a string, a host type, or a frozen table of such. Not a
   tuple or a sum (several values), a live buffer (the box would let the host alias it, ADR 0018), a
   function (it may not escape staging), or `any` (no type). As for every interface, the coercion is
   the identity in meaning and the host inserts the box.
3. **A variadic over `any` is one declaration over `(array go.Value)`**, spread into Go's call.
   `fmt.oro` declares each function twice:
   - for one value under Go's name, `(fmt.Println x)`, the restriction f|Value¹;
   - for any number as NAME-All, `(fmt.PrintlnAll (array a b c d))`.

   The two- and three-operand declarations (20) are deleted. H's `Append`, `Appendf` and `Appendln`
   are declared the same way.
4. **A position that holds one value refuses several.** Lowering took the first value of a term
   that yields several, in 19 places. A table of tuples handed to a host as one table was emitted as
   the table of first components, a wrong answer that predates this work:
   `(strings.Join (array (tuple "a" "1") …) "-")` printed `a-b`. It is refused now (`lowerer.single`).

## Consequences

- `-gcflags=-m` and a benchmark show Go treating `[]any{a, b}` spread into `fmt.Append` as its own
  call's table: the table does not escape, each boxed value escapes in both, 8 B and 0 allocations
  per call, the same time within noise.
- 293 of 590 Go names declared (49.7%); H is gone from the causes.
- Eight call sites moved to the NAME-All forms (`lines.oro`, `u128/factorials.oro`, the acceptance
  program). One `factorials.oro` exit had relied on a `Print` statement's value being its first
  argument, and now returns `n` explicitly; the program's output is identical.
- An emit test had compared a `result` with `io.EOF` since ADR 0044, emitting Go that did not compile;
  `single` found it, and it now reads the error through the result.
- Formatting through a box sees the host value, so a verb that tells representations apart tells
  ours apart (`%x` on a table whose element width the compiler chose). Tier 2, as it always was
  through `(a any)`.

## Why not

- **Rename the language's `any`** so that `any` could name Go's box, as Go's documentation does:
  measured at 388 uses in 19 files and 35 compiler sites, no change to emitted code. hamza kept the
  name.
- **Make the language's `any` the existential**: Windows' x86 has no box without one in the core,
  which CLAUDE.md forbids, and a language construct must work on every target. A box is a host type,
  where the host has one (Go's `any`, Java's `Object`, any JavaScript value).
- **Restrictions per arity for H** (`Append2`, `Append3`): no language work, and the wall at three
  operands stays. The algebra says the operands are one list.
- **A spread for every print, `(fmt.Println (array x))`**: 220 of the corpus's 231 print calls pass
  one value, and each would gain a table. The one-value restriction is exact, so it keeps Go's name.
- **A tuple of any length as the operand list**: a new type former, and not Go's representation. A
  table of boxes is exactly Go's `[]any`.
- **Declare `int ≤ go.Value`, `string ≤ go.Value`, … one edge per type**: a list stating a
  closure by hand, which is how closures get stated wrong. The edge is derived from the empty method
  set.
