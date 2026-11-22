# A call's sort claim, decided at the call

Date: 2026-11-22. hamza, 2026-11-22: "A now, go ahead. always mathematical and algebraic", on the
book's finding 110 (ADR 0154).

## The law

A header `fn f(x ∈ S) (R)` is f : S → R, and an application f(a) is defined iff ⟦a⟧ ∈ S: the domain
condition is the call's obligation (ADR 0028), the program's statement whoever reads x. The
obligations of a call are the conjunction over its parameters, each of one of three kinds:

| a parameter's set | its mark | decided by |
|---|---|---|
| a range, a grid, a record | `(#req f x S a)` on the argument | the IR's intervals, the refinement layer |
| a `where` φ | `(#reqw f φ[ā/x̄] body)` on the result | the same |
| any other (ℤ, a string, a Boolean, a variant, a table's sort) | `(#reqw f (#in "S" a "i") body)` on the result, **new** | the type checker |

The third was missing: the claim lived only in the body's demands, which the inlined body may make
of a weaker set, of the same set somewhere a library's names describe, or not at all. The membership
is a condition on the call as a `where` is, so it is one: every rule that moves a where-mark (the
let, a tuple's and a selection's tails, relevance, the state translation, records, a dead value's
hoisting) moves it with no rule of its own. Its decision is the type checker's, so the first decider
of a residual turns it into the typing mark ADR 0137 built, `(#is "S" a body "f" "i")`, which the
checker reads before the body it guards.

The discard, `ignore` = !_A : A → 1, is a function on values; the obligations of a pure value it
drops are statements of the program, so they stand on the unit, as on an unread binding (the book's
81).

## Measured first

On HEAD (00369f19), Go and windows:

- `io.print_int(0.5 + 0.5)`: *in argument 1 of io.print_line: in argument 1 of decimal: n is N, but
  Int is required here* on Go, *in argument 1 of `llvm/str`.utf8: in argument 1 of decimal: n is N,
  but -2^63..2^63-1 is required here* on windows: `n` is the let β made, and no name in either is
  the program's.
- `fn f(s ∈ String) (Int) { 0 }`, `io.print_int(f(3))`: **built**, a false claim accepted.
- `fn h(b ∈ Bool) (Int) { if b { 1 } else { 2 } }`, `h(3)`: *in a condition: an integer literal is
  Int, but Bool is required here*, the `if` inside h.
- `ignore(g(30))` against `fn g(n ∈ 0..9)`, and against `fn g(n ∈ Int) (Int) where n < 5`: **built**,
  both, where `x = g(30); ()` is refused.

## What was built

1. **The mark** (core/reduce.go): `Env.Sorts`, a definition's parameters' sets that are no range,
   grid or record and no buffer, cell or product, filled with `Requires` and `Wheres` by
   `installContracts` (emit/requires.go); `markSorts` at β beside `markWhere`, the argument a pure
   one's copy reduced and stripped of the marks reducing it made (the body's copy carries those) or
   an impure one's name; a literal of the set's kind decided there, a λ left to W16. Also at a kept
   call folded on literals. A dead call naming one is reduced for its marks (`callsContract`). The
   set is the target's unfolding (windows' `error` is 0..2^32-1, a range, the argument mark's), and a
   variant's set is the eliminator's, no mark's. `PlantSortClaimDropped`.
2′. **The copy** (`claimed`): reduced with claims off, since the calls inside it are the body's copy's
   too, and stripped of the marks reducing it made; reduced again where a `where` would be
   (`reduceCondition`), as a claim. A copy that does not reduce alone, or whose records the eraser
   cannot shape alone (a record read through a table rule, curried at the body's binder), leaves the
   claim to the body's demands, HEAD's behaviour for that argument.
2″. **The claim is no use** (`programOccurrences`, work.go): β's one-use rule and ADR 0071's work
   count read the program's occurrences, outside the claims; a claim reading a name takes its value
   wherever the binding goes (`unread`). `PlantClaimCounted`.
2. **Its decision** (emit/bigrep.go, emit/check.go): `TypingWheresAsMarks` makes each a typing mark,
   at `DischargeRequires`' start and `ir.Entry`'s; the checker reads a call's typing mark as
   `(#claim-at "S" a "f" "i")`, a walked as itself and its type agreeing with S, and refuses it as a
   `ClaimAtError`, *in argument i of f: a is T, but S is required here*. `ir.Entry` reports a claim's
   refusal ahead of the plain check's when it is the first in the program's order, and without the
   residual's frames around it.
3. **The deciders read what a typing mark denotes**: `DischargeRequires` decides ranges and `where`s
   on the residual with its typing marks erased, and returns them kept; `ir.Lower` reads a typing
   mark as its body, so a path that lowers without checking (a printer's `FromResidual`, the interval
   route, a realization) reads what it always read.
4. **The discard** (core/reduce.go): `ignore` of a pure value is the unit carrying its obligations;
   `PlantDeadClaimDropped`, the 81 rule's fault, drops them again.
5. **Tests**: `TestACallsSortClaimIsDecidedAtTheCall` (unread, dead and read claims refused at the
   call on Go and windows, true ones building, the plant restoring the old behaviour on each),
   `TestADiscardedCallOwesItsContract`; `TestTheMarksLeaveTheResidualUnchanged` compares the decided
   residual erased of its typing marks.

## What failed first

- **The copy reduced with its own claims was exponential**: an argument holding a call with a claim
  was reduced once in the body and once in the claim, and k nested calls 2^k times; windows' `os`
  programs passed the step limit, pe compiled 3.8× slower, the first sweep 1.22×. Reduced with claims
  off it costs one reduction more per claimed argument. Reducing the argument once before β and
  sharing it was tried and is wrong in this reducer: some rules read an argument's unreduced shape,
  and pe's segment bound became a tuple.
- **A mark left its binder**: `hoistWheres` lifted every where-mark out of a condition, a λ's closed
  body included, and a condition reading the λ's variable read the next binder's (pe's `name`, a
  segment of a segment in its claim's copy: n read as the pair (n, r), the buffer as n). A pre-existing
  defect, reached first by a condition holding a table rule. A mark now leaves a λ only if its
  condition reads none of its variables (`readsLooseBound`), `PlantWhereLeavesItsBinder`, witnessed
  through `PlantSortClaimReducedAsWhere`.
- **The copy counted as a read**: a pure binding read once and copied by a claim stayed a binding,
  and roundtrip's main on windows grew from 10,320 lines to 12,507; the book's refusals named `half`
  and `primes` where they had shown the substituted terms. The claim is now no occurrence of the
  program's, and the texts are the old ones.
- **The claim met what the checker's ascription knew and `agree` did not**: a live buffer at an
  `Array(V)` (ADR 0069, cmp), a literal table of string literals at `Array(go.bytestring)` (the
  variadic acceptance program), WNDPROC's `UINT` and windows' `error` (a target's set, unfolded now).
  The claim is decided as the ascription, a walked against S, and the message is a's own type against
  S.
- **A claim nested in another inlined call** was reported under the residual's frames, *in argument
  1 of io.print_line: in argument 1 of decimal: in argument 1 of f*: the walk's prefixes describe the
  residual. A call's claim is now reported alone.
- **The refinement layer and the interval route read the new marks**: `#is` has no lowering, and a
  fact behind one was lost (a realization's `lang.release` unproven on windows, ADR 0137's own test
  failing, the token variants). The deciders read the marks' bodies.
- **Ω's test overflowed the stack**: `normalizeStep`'s frame at `MaxDepth` sits near 512 MB, and the
  new locals and calls in it passed the limit. Both moved out of line (`contracted`, `markCall`,
  `discarded`), as `keptCall` had been for the same reason.
- **The heredoc and the Python alias**: two edit scripts lost their backslashes, and the WindowsApps
  `python` stopped running mid-step; the scripts were rewritten and run by the installed interpreter.

## Paid and bought

| | |
|---|---|
| compile time | the sweep's median 1.06–1.10× the baseline's over three sweeps, none confirmed slower paired serially (codecs on windows 1.06× paired, cleared); one reduction more per claimed argument, the copy's |
| run time | none: the marks are erased before the IR |
| code size | none: every emitted file byte-identical |
| emitted text changed | none on any target; one refusal's reason, pe on windows (refused before and after: P4 inlines it whole, and the copies pass the step limit before the table-rule refusal it met), accepted |
| proofs | unchanged, 12,595 of 12,649 operations and 1,588 of 1,889 loops |
| refusals changed | refused now: a call's false claim whose parameter is unread (`f(3)` against `s ∈ String`), and a discarded call's range and `where` (`ignore(g(30))`), no program of the corpus; reported at the call now: a read claim (`io.print_int(0.5 + 0.5)`, chapter 8) |
| compiler lines | +513 −35 (core/reduce.go +311, emit/requires.go +76, emit/bigrep.go +55, emit/check.go +41, ir/pipeline.go +25, core/recerase.go +14, the rest under 6); tests: cmd/build/sortclaim_test.go, 4 tests and 5 planted faults |
| rule made less general | none: the where-mark's binder rule is stricter than before, only where a condition read a binder's variable, which no program met correctly |
| not built | a term a message quotes is still shown reduced (chapter 17's no-print, the half of finding 110 left); a definition over sets marks no instance (its sets are variables until instantiated); a claim whose set is a function's (a callback's arrow) is W16's, a variant's the eliminator's; a copy that does not reduce or shape alone leaves its claim to the body (a record read through a table rule); a claim that reduced to a literal after it was made stays a mark to the checker (439 of `int` in pe's residual); `normalizeStep`'s frame, 0x14b0 bytes of locals, holds Ω's 100,000 levels within 8 bytes of Go's stack limit, at HEAD as now, so any local added to it overflows (moved out of line here); a range by an alias (`c ∈ error` on windows) is no argument mark, so it is the body's, as before |
