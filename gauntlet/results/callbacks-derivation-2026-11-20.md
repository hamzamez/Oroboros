# callbacks-derivation — a λ bound to a name wrote a cell and lost it; tier 1's decisions derived, 2026-11-20

hamza, on callbacks-sketches.md Part 2: "2.3 you mean a cell from outside the lambda? my first
instinct is no … 2.4 same, this looks like a closure, am I wrong? … for every decision I have to
make on these, show me where the derivation leads."

## What was measured

To answer whether a cell written in a λ exists in the language today, I wrote three probes:

| program | before | after |
|---|---|---|
| `var n := 0; loop i = 0 { i < 5 -> { n := n + i; again(i + 1) }; else -> print(n) }` | 10 | 10 |
| `twice(fn(x) { n := n + x })`, a λ passed to our own definition | refused: a function that mentions a cell | the same |
| `add = fn(x) { n := n + x }; add(1); add(2); print(n)` | **printed 0** | refused, as the one before |
| `get = fn() { n }; n := 5; print(get())` | **printed 0** | refused |
| `put = fn(i) { b[i] := 7 }`, over a buffer | refused, R3 | the same |

**A wrong answer**, on every target, since it comes from the core. local.md §3 refuses a λ that
mentions a cell anywhere but an elimination's continuation or a loop. The state translation checked
that for a λ in an application's argument (`cell.go`, "any other application") and for a λ in value
position. A let's argument it kept as it is, for β to substitute, without the check. The λ's writes
reached no translation and were lost, and its reads saw the cell's first value.

**The fix**: the let case refuses a λ argument that mentions a cell, with the passed λ's message.
`PlantLetLambdaCellUnchecked` lets it through again, and `TestALambdaBoundToANameMayNotMentionACell`
requires both programs refused and, under the fault, both loading. A buffer's case was already
refused by R3.

## The derivation (callbacks-sketches.md Part 3)

Read in the core, where the state translation makes a cell a threaded value:
- **A λ given to a host is a closure in every option**, 2.1's `fn(r) { r == c }` included. What
  the language refuses is a closure that survives; tier 1's is second-class.
- **The markers exist only for the two checks that cannot see through a host's body**: the count
  for usage (𝕌), the order for effects. A pure λ consuming nothing needs neither (Theorem D).
- **A cell written in the λ is the state monad's lift of a traversal** (`mapAccumL`), a pure
  function. It is defined where the host's order is documented, and nowhere else.
- **Option (b) of decision 3, commuting writes under no order, is withdrawn.** It needs the count
  as well as the order, and under no order the count is the host's.
- **A buffer is the same decision as a cell**, its state a table.

## Paid and bought

| | |
|---|---|
| compile time, run time, code size | none: a refusal added; `cmd/check` byte-identical |
| proofs, refusals changed | two programs that printed a wrong answer are refused; no accepted program in the corpus, the book or the gauntlet changed |
| compiler lines | `core/cell.go` +13; `core/local_test.go` +32 |
| documents | callbacks-sketches.md Part 3 (about 230 lines); §2.3's (b) and research §3.4 and §8 corrected; the book's §14.2 gains "A cell and a function" (4 blocks, 1,115 in all) and finding 102 |
| rule made less general | none: the rule is local.md §3's, now checked where it was not |
