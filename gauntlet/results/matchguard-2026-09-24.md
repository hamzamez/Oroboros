# A guard that is a connective narrows, and a trip count holds on every edge

2026-09-24. The last of the three analysis limits [bounds-2026-09-24](bounds-2026-09-24.md) named that
the item asked for: a `match` guard did not narrow. Closing it found a false proof in the trip count,
and a counting error in every corpus figure.

## 1. The guard

`and`, `or` and `not` are reader sugar over `if` (ADR 0017). A `match` clause with a literal pattern
and a `when` becomes `(if (= tag 0) (>= v 10) false)`, and `refine`, which narrows the environment by
a condition, read only binary comparisons. On three arguments it returned at once. So the
clause's `(- v 10)` kept −∞ as its lower bound, the reset clause `(again 0 0 c)` showed no descent,
and `match` was 0 of 9 loops proven terminating, with its counter unbounded.

**Built:** `refine` reads a connective by De Morgan:

| condition | narrowing |
|---|---|
| a ∧ b holds, or a ∨ b fails | by both operands, in the order they evaluate |
| a ∧ b fails | ¬a ∨ (a ∧ ¬b): the **join** of the two paths |
| a ∨ b holds | a ∨ (¬a ∧ b): likewise |
| ¬a | a, the other way |

The join of two environments keeps only the keys both know. A key one path alone narrowed is ⊤ on
the other.

## 2. What failed first: a false proof

With the guard narrowing, the reset clause needed an arc. Adding one exposed how `tripCount` numbered
a loop: it divided the span by the least step of the **last edge examined**. The field's own comment
said so: "per the LAST edge examined".

```lisp
(sig f ((n (int 0 100))) int)
(def f (n)
  (loop ((k n) (c 0))
    (<= k 0)  c
    (< k 50)  (again (- k 1) (+ c 1))
    else      (again (- k 10) (+ c 1))))
(def main (fn () (fmt.Println (+ (f 100) 9223372036854775795))))
```

This loop makes 46 trips from 100. Numbered by the last edge's step of 10, it made 11, and the sum was
proven in the word. It compiled without `-checked` and printed −9223372036854775775, which is
9223372036854775841 wrapped modulo 2⁶⁴. Now it prints 9223372036854775841: the honest bound places
the sum in U, which Go holds as `uint64` (ADR 0026).

**Built:** the descent a trip count uses is the **meet** over every edge:
- the smaller δ of two subtractions;
- the smaller base of two divisions;
- δ = 1 for one of each, since v ≥ 1 and b ≥ 2 give v − ⌊v/b⌋ ≥ 1;
- none, if any descending edge has no measure.

**Which exposed a borrowed measure.** A derived step, an index advanced through a choice of inlined
scanners, returned `descent{}`. Its comment said the measure was withheld "because span / delta + 1
also needs cur[src]". It never was: with last-edge numbering, the other clauses' step stood in for
it, and fixpoint-2026-08-27 had recorded exactly that ("i still gets kind{1,1} from the other
clauses"). Under the meet, withholding really excluded it, and `jsonfmt` was refused on three targets.
The reason for withholding was the fixpoint bug of that week, since fixed: `cur` is the loop-head
invariant, and `tripCount` demands it bounded. So the derived edge now carries its own least step,
c. That reproduces the old count where the other edges step by 1, and is honest where they do not.

## 3. The reset, and the dead edges

- **Separated intervals descend.** If every value an `again` argument can take lies below every value
  the variable holds at that edge, in the oriented measure, the edge descends by at least the gap.
  In `match`'s reset clause the guards leave v ∈ [1, 9], and 0 < 1.
- **A dead back edge is no edge.** `match` on 0 exits at its first clause, so its back edges are never
  taken, but they were recorded (no descent) and joined into the loop state (a counter widened to +∞).
  An edge where some loop variable is ⊥ now contributes neither: a state with an empty component is
  empty.

**Result:** `match` is **27 of 27** operations and **9 of 9** loops on every target, and its
`; checked:` declaration is removed. The harness reported it stale before I removed it.

## 4. What the check found: a counting error

The corpus figures moved:

| | before | after |
|---|---|---|
| operations | 2,387 of 2,434 | **2,007 of 2,054** |
| loops | 349 of 387 | **343 of 381** |

The number unproven is **unchanged**: 47 operations and 38 loops. So the corrected denominators are
the only difference. `cond` evaluates and counts a condition, then `refine` evaluates an operand again
for each branch to narrow the other side, **with counting on**. `(>= i (- n 1))` counted its one
subtraction three times, and a connective six. Loops inside guards were counted again the same way.
`refine` now counts nothing, and `TestAGuardsOperationIsCountedOnce` pins it (3 and 6 without the fix).

Every figure quoted since that code was written was inflated. README and CLAUDE.md now carry the
corrected ones.

## 5. Compile time

The first build was slower: `render` on windows took 3,040 ms against 2,110, and `u128` 295 ms against
225. The profile showed `edgeGraph`. `relate` runs once per **pair** of loop variables, and the
separated-interval arc evaluated the `again` argument every time. `collectAgain` has just computed
each argument's value for the fixpoint, so it now hands those values over, and `relate` evaluates
nothing. Serially, warm, against a baseline worktree: `render` windows 2,103 ms against 2,116, `u128`
198 against 238. The sweep median is 1.09×.

## 6. Checked, and how each check fails

`emit/matchguard_test.go` has eleven tests. Each rule was removed in turn:

| rule removed | caught by |
|---|---|
| a connective narrows (the old behaviour) | `TestAConjunctionNarrowsByBothOperands`, `TestAFailedDisjunctionNarrowsByBothNegations` |
| a failed conjunction is a join, not ¬a ∧ ¬b | `TestAFailedConjunctionIsTheJoinOfTwoPaths` and one more |
| the join forgets a key one path alone knows | `TestAJoinForgetsWhatOnePathAloneKnows` |
| the trip count's meet (the last edge, as before) | `TestATripCountHoldsOnEveryEdge` |
| the separated-interval arc | `TestAResetBelowTheGuardDescends` and one more |
| its step is its gap (planted as 1000) | `TestASeparatedEdgeDescendsByItsGap` |
| the derived step's measure, withheld or overclaimed | `TestADerivedStepDescendsByItsLeastStep`, both ways |
| a dead edge is not recorded | `TestAnUnreachableBackEdgeWithNoDescentIsNoEdge` |
| a dead edge is not joined | `TestAnUnreachableBackEdgeIsNoEdge` and one more |
| `refine` counts nothing | `TestAGuardsOperationIsCountedOnce` |

Three plants were first **missed**. Each exposed a weak test, not a gap in the code:
- A join plant kept only the first path's one-sided keys, while the test's key was on the second
  path.
- The separated arc's step was hidden by another edge's smaller step.
- A dead-edge test was rescued by an ascending counter that the other dead-edge rule had just bounded.

Each got its own witness, and all eleven rules are now caught.

## 7. What it cost

- **Emission:** byte-identical. The changes are the proof-count notes of 21 compiles.
- **Differential suite:** passes. `match` needs no declaration.

## 8. Not built

- Two of bounds-2026-09-24's limits stand: element bounds for `alloc (table …)` and for the cells of a
  map's table (which `map-len` and `map-keys` still declare), and geometric accumulation.
- A single loop with edges of very different steps is still numbered at its smallest. That is
  honest, and loose: a phase-wise count would be tighter, and no program has asked.
