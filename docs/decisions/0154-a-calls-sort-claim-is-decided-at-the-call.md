# 0154 — A call's sort claim is decided at the call

Date: 2026-11-22
Status: Accepted (hamza, 2026-11-22: "A now, go ahead. always mathematical and algebraic", on the
book's finding 110, option A: the typing mark at the call). Completes
[ADR 0028](0028-a-definitions-contract-is-checked-at-its-calls.md) (a header's sets are obligations at
every call) for a set that is no range, with [ADR 0137](0137-an-unread-binders-typing-claim-is-checked.md)'s
mark. **Built 2026-11-22** ([sortclaim](../../gauntlet/results/sortclaim-2026-11-22.md)).

## Context

A header `fn f(x ∈ S) (R)` says f : S → R, and f applied to a owes ⟦a⟧ ∈ S (ADR 0028), whoever reads
x. ADR 0028 realized this for two kinds of claim: a range is a mark on the argument, `(#req f x S a)`,
and a `where` a mark on the call's result, `(#reqw f φ[a/x] body)`. A set that is no range (ℤ, a
string, a Boolean, a variant, a table's sort) had no mark. Reduction inlines the call, so the claim
was checked only where the body's uses happened to demand a sort:

- **read**, it was reported inside the body, in its library's names: `io.print_int(0.5 + 0.5)` was
  *in argument 1 of io.print_line: in argument 1 of decimal: n is N, but Int is required here* on Go,
  *in argument 1 of `llvm/str`.utf8: …* on windows (the book's finding 110);
- **unread**, it was accepted: `fn f(s ∈ String) (Int) { 0 }` and `f(3)` compiled, a false claim
  (ADR 0137's case for a parameter where 0137 took a binder's);
- **discarded**, every obligation went: `ignore(g(30))` against `fn g(n ∈ 0..9)` compiled, a range
  and a `where` as well, where `x = g(30); ()` was refused (the book's 81 rule), since `ignore` of a
  pure value was the unit with nothing kept.

## Decision

1. **The claim is a membership on the call's result.** At β of a call of a definition whose
   parameter i's set S is no range, grid or record (each its own mark) and no buffer, cell or product
   (a mode or a flattening, not a value's set), the result is marked
   `(#reqw f (#in "S" a "i") body)`: a, the argument as `markWhere` passes it, pure as itself and
   impure as the name β bound it to, so the condition runs nothing and reads nothing twice. It is a
   where-mark because it is one, a condition on the call: every rule that moves a `where` (the let,
   the tuple's and the selection's tails, relevance, the state translation, the records) moves it, and
   a dead value's is hoisted with the rest.
2. **Decided by its kind where the reducer can, by the type checker otherwise.** A literal of the
   set's own kind (an integer in ℤ or the word, a string literal, a Boolean literal) is the claim
   decided; a λ is a callback's, decided by W16 and ADR 0143. The rest is the type checker's: the first
   decider of a residual (`DischargeRequires`, `ir.Entry`) turns the mark into a typing mark
   `(#is "S" a body "f" "i")`, which the checker reads as `(#claim-at "S" a "f" "i")` before the body
   it guards, and refuses as *in argument i of f: a is T, but S is required here*, the claim alone,
   without the residual's frames around it.
3. **The claim is logic, not a use.** Its copy is reduced with no claim of its own (the calls inside
   it are the body's copy's too) and counted by no rule of β's (the one-use rule and ADR 0071's work
   count read the program's occurrences); a claim reading a name takes its value wherever the binding
   goes. A copy that does not reduce, or shape its records, alone leaves the claim to the body.
4. **A where-mark leaves a λ only if its condition reads none of its variables**: a λ's body is
   closed, and a condition reading one, hoisted, read the next binder's.
5. **The discard keeps its obligations.** `ignore(e)` of a pure e is the unit carrying e's
   obligations (`hoistObligations`), as an unread binding is: !_A : A → 1 discards a value, not the
   program's statements about it.

## Why not

- **Name the inlined definition in the message (provenance)**: relabels the report and decides
  nothing; the unread claim stays accepted, and the read one is still found only where the body
  demands it, which a body may not.
- **A mark on the argument, as a range's**: an argument's shape is what reduction reads (β of a λ, a
  table's length and its reads, a record's projection, a literal's folding); a mark there would stand
  in each rule's way, and a range's mark is looked through only because its argument is an integer.
- **A mark of its own beside the where-mark**: the where-mark's conversions are 61 sites of the
  reducer and more in the state translation and the records; a second mark would repeat each, and
  one missed is a claim lost or a reduction stuck.
- **Type-check the call before reduction**: ADR 0137's reason, a second checker on terms not yet
  specialized.
- **Decide every claim in the reducer**: the reducer has no types; the kinds of literals are the part
  it knows.

## Consequences

- Every claim of a header is decided at every call, read, unread or discarded, and a refusal names
  the call the program wrote.
- A call's argument is copied into the condition, reduced there and stripped of the marks reducing it
  made (the body's copy carries them): one reduction more per claimed argument, 1.06–1.10× the
  sweep's median compile time, none confirmed slower; a program inlined whole on windows (pe) meets
  the step limit sooner.
- Emission is byte-identical: the marks are erased before the IR and change no reduction.
- A definition over sets (ADR 0063) is marked at no instance: its parameters' sets are variables until
  instantiation, and an instance's claims are left to the body as before.
