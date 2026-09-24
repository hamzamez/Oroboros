# A definition's contract, checked at its calls

2026-09-24. The build of [ADR 0028](../../docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md),
after the measurement in [requires-2026-09-24](requires-2026-09-24.md). hamza chose option A, with
the bit-length rule for ranges above the word.

## 1. What is built

Every call a signed definition is inlined into carries the definition's declared contract as an
obligation, and the program is refused if one is not proven. Two marks carry it:

| mark | on | carries |
|---|---|---|
| `(#req "def" "param" "type" a)` | the argument | a finite parameter range; its value is `a` |
| `(#reqw "def" cond body)` | the call's result, at β | the `where` as written, plus the finite side of an infinite range; its value is `body` |

`emit.DischargeRequires` decides both immediately after reduction and erases them, so nothing
downstream ever sees one. The order:
1. **A literal**, decided while the call reduces.
2. **The interval analysis**, for ranges:
   - within the word, the set is exact;
   - above the word, the set is `|x| < 2ᵇ` (bit length), the set its enforcement admits;
   - a declared result above the word, `(the T e)`, is read as that fact;
   - a range holding the whole signed word is vacuous and is not marked at all.

   What the interval analysis cannot finish it hands on as a `where` condition naming only the ends it
   did not prove.
3. **The refinement layer**, for every condition left, with the facts in scope and each pure call's
   `ensures`. A closed condition is decided with no facts, and then the full walk is not run.

An obligation none of them proves is refused, naming the definition, the parameter or the condition,
and the call's argument.

In source, the only visible change is to two programs:
- `render.oro`'s exported `run` declares `(sig run ((n (int 0 15))) string)`;
- `lib/win/fmt.oro`'s `print-int` is total (§2).

## 2. What failed first

1. **The measurement's scope.** It counted `where` only on non-exported definitions and found one. The
   build found `win/fmt.print-int`, an export called by every Windows harness print: 22 differential
   cases refused. Its `where` was `0 ≤ n < 2⁵³ − 1`:
   - The upper bound was ADR 0012's window, stale. The digit field holds any non-negative int64.
   - The lower bound was the meaning gap refinements.md §6b named: `print-int -13` printed a blank
     line.

   The fix is the one the file's own comment had put off ("a sign is four more lines"). `print-int`
   prints a sign and the digits of −n. −2⁶³, the one value whose negation is not in the word, is its
   own clause. It is tested as `n < −(2⁶³ − 1)` because an ordering narrows the interval of `n` after
   it and an equality does not, and the negation needs that narrowing. The loop is the one it always
   was, so size-change proves it and its trip count still bounds the buffer index. The parameter is
   declared as the word, which is vacuous at a call and bounds the loop when `print-int` is compiled
   on its own.
2. **A range meets a `where` in the refinement layer.** The loader desugars a range into `where`
   conjuncts, so the written clause is kept apart (`Sig.Written`) and a range is not checked twice.
   A condition built from an argument then carried that argument's own range mark,
   `(<= 0 (#req … a))`, which the linear fragment cannot read. The condition now reads arguments
   without their marks.
3. **Two layers, each proving half.** `digits18 ← Rem64(h, l, 10¹⁸)`: `Rem64`'s declared result gives
   `≥ 0` to the interval analysis, and its `ensures` gives `< 10¹⁸` to the refinement layer. Each
   proved one end, and the whole range was asked of each. Now the interval analysis hands on only the
   ends it did not prove.
4. **A table bound to a name.** `examples/json/tokenize.oro` calls `tokens` with literal tables. The
   table is used twice, so β binds it to a name, and `(len src)` was a length nothing knew. A pure
   argument now goes into the condition as itself, so `(len (array …))` folds to 20. An impure one
   stays the name β bound it to, and nothing is duplicated.
5. **Compile time.** The first build ran an extra interval pass and a full refinement walk on every
   program that had a mark:
   - `sieve-win-bench`: +140 ms serially;
   - `tokenize`: 1.96× in the sweep.

   Both are back to their baseline times, measured serially against a baseline worktree:
   - a range holding the word is not marked;
   - the interval stage runs only when there is a range to decide;
   - a closed condition such as `(go.< 20 1048576)` skips the walk.

## 3. Checked, and how each check fails

`emit/requires_test.go`. Each rule below was removed in turn, and a test failed:

| rule removed | caught by |
|---|---|
| a refusal is reported | `TestADeclaredRangeIsAnObligationAtTheCall` (range, and literal out of range) |
| the `where` collects the `ensures` in its condition | `TestIntervalsAndEnsuresShareARange` (a definition returning its parameter, so no primitive above the mark collects them) |
| the interval analysis hands on only the unproven ends | the same test |
| an ascription is the fact its enforcement admits | `TestARangeAboveTheWordIsItsBitLength` |
| above the word a range is its bit length | the same test, with a result declared to 2²⁰¹ − 1 into a parameter declared to 2²⁰⁰: one set by bit length, not exactly |
| a `where` is marked | `TestAWhereIsAnObligationAtTheCall` |
| a pure argument goes into the condition as itself | the same test (a literal table used twice) |
| an infinite range's finite side | `TestAnInfiniteRangesFiniteSideIsChecked` |
| a where-mark is hoisted from operator position | `TestTheMarksLeaveTheResidualUnchanged` (a tuple-returning definition applied to its eliminator) |
| a where-mark is hoisted through a fold | the same test |

One branch was **deleted, not tested**. The refinement layer's own handling of an in-word range could
not be reached, because the interval analysis turns every such range into a condition. Its plant
changed nothing, which is how that showed. What reaches the refinement layer as a range is above the
word, and there it is refused with the reason stated.

## 4. What it cost and what it changed

- **Emission:** 17 changes, all reviewed.
  - The four Windows programs that print, and `lib/win/fmt.oro` itself, carry the total `print-int`.
    `sieve-win-bench` runs in the same time: median 90 ms against 90 over ten alternating runs.
  - `render.oro` on four targets: `(+ 25 n)` is proven now that `n` is declared, and loses its
    `-checked` wrapper.
  - Every other emitted file is byte-identical.
- **Differential suite and acceptance programs:** pass, all hand declarations agree with the host.
- **Proof counts:** 2,387 of 2,434 operations (from 2,368 of 2,419; the new `print-int`'s 15, all
  proven), and 349 of 387 loops, unchanged.

## 5. Not built, and named

- **A definition passed as a value and applied later** is not a direct call by name, and is not
  marked.
- **Printing the literal −2⁶³ on Windows is refused.** Its `else` clause is unreachable, but the
  interval analysis does not treat it as dead inside the digit loop, so it finds the loop's index
  unbounded there. A computed value prints, and −2⁶³ itself is handled by its own clause.
- **Above the word, only the bit length is an obligation,** as it is the only thing enforced.
  `(int 0 (pow 2 200))` admits a negative value on Go, where `BitLen` reads |x|. JavaScript's check,
  `(x >> k) !== 0n`, refuses one. That difference is in bigrepr-2026-09-03's enforcement, not here,
  and is named for whoever meets it.
