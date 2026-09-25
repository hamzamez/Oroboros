# A `build` returns a product: ADR 0031, built, parser first

2026-09-25. ADR 0031 said a `build`'s result is a product of its frozen buffers and buffer-free
values, taken apart by a tuple pattern. hamza asked for it built, spec first, parser first. Spec:
[tables.md §2.5](../../docs/spec/tables.md), [binding.md §5](../../docs/spec/binding.md),
[iteration.md §2](../../docs/spec/iteration.md), [maps.md §3.3](../../docs/spec/maps.md). The ADR
carries four corrections the build produced.

## 1. The parser

`examples/json/tree.oro` returned one table and wrote its node count and ok flag into slots 0 and 1
of it. It now returns `(tuple nodes nn ok)` from its four exits:

```lisp
(let (tuple nodes nn ok) (parse src)
  (go.+ (go.* (go.- nn 1) 1000000)
        (go.+ (go.* (walk nodes) 10) ok)))
```

**The proofs are exactly the old program's:** `measure` 99 of 99 operations and 5 of 5 loops, `run`
416 of 416 and 20 of 20. That includes the walk over `nodes`, whose node indices are proven in range
from content facts about the frozen table. So those facts crossed the tuple.

`gauntlet/differential/cases/json-tree.oro`, its twin, runs on all four targets and gives the answers
computed by hand: 3030081 3030111 4040171 7070331.

## 2. What was built

| piece | where |
|---|---|
| the shape, once for every pass: `tupleElim`, the projection Pⱼ, the skeleton | `emit/prodresult.go` |
| the component law in the interval pass: each exit's components recorded, joined over the exits, bound to the names; element range and length from Pⱼ | `emit/prodfacts.go`, hooks in `interval.go` |
| the type checker walks Pⱼ; refinement binds each name as a `let` binds Pⱼ | `check.go`, `refine.go` |
| a loop's head facts keyed by its back-edge skeleton | `content.go`, `bodyKey` |
| a join point per backend | `golang_join.go`, `js_join.go`, `java_join.go`, `asm_join.go` |
| S: a store into a frozen table or map is refused, and a map buffer is linear | `linearity.go` |
| R1 (a variant out of a scope) and a jump out of a join point, refused by name | `CheckJoins`, before any pass |
| a nested loop's `again` is its own, in the reader | `core/read.go`, `noAgain` |

**Why a projection is sound.** ⟦Pⱼ⟧ = πⱼ⟦P⟧: Pⱼ is case-of-case with the eliminator `(fn (x̄) xⱼ)`,
which is pure, so copying it into every tail changes nothing observable. A fact about Pⱼ's value is
therefore a fact about `xⱼ`. Pⱼ is analysed and never emitted, because emitting m of them would run
the producer's effects m times.

**Why the skeleton key is sound.** The facts cached at a loop's head describe the states the loop
reaches there, and those come from its entry and its back edges. The producer and each Pⱼ are the same
loop with different exit values, so they reach the same states.

## 3. What failed first

1. **The theorem's hypothesis was false in the compiler.** ADR 0031's no-copy freeze assumes that no
   store can name a frozen value. Probing it showed three wrong answers:
   - `(set t 1 n)` on a frozen `t` changed what `(t 1)` read;
   - `(insert m 1 …)` on a frozen map did the same;
   - a map buffer used twice leaked a key into a map that never received it.

   No check looked at a map buffer at all. maps.md said one did. The corpus is byte-identical under the
   new refusals, so no program relied on any of this.
2. **Two passes silently checked nothing inside the new shape.** For `((build …) K)`, the interval
   pass returned ⊤ without evaluating either side, and the type checker returned "unknown" without
   walking it. That is the host-call continuation's hazard a second time, now a CLAUDE.md lesson.
3. **The ADR's commute into the scope was wrong for the analyses.** `K` inside the scope reads the
   table inside the scope that fills it, where reads are unknown by design, so the walk's proofs would
   be lost. The join point spans the whole scope instead.
4. **Content facts did not cross the tuple.** The refiner caches a loop's head facts by the printed
   loop. The producer's loop and Pⱼ's differ only at the exits, so the lookup missed. The fix was to
   key by the skeleton.
5. **The exits' join missed a clause.** A clause that binds and then jumps (`let` … `again`) was not
   recognised as yielding no value, so the join lost the exits that do yield one, and `nn` was
   unbounded in `run`.
6. **The `build` hook sat after an early return** in the interval pass's generic path, so the record
   never reached the scope's rebuilt term.
7. **Go's function result type came out `/*unknown*/`,** because `typeOf` did not see through the join.
   `tailTypes` now reads it at the first tail.
8. **javac refused twice.** First, `int` result variables were fed from loop variables the method had
   kept wide. Then wide result variables were added into a narrowed loop result. The fix is an `int`,
   assigned through a cast, when `fitsIdx` holds. The cast is exact, because every integer operation
   in such a method is proven inside 32 bits.
9. **The reader refused a literal loop bound by a clause that then jumps.** `noAgain` walked into the
   nested loop and found its own `again`. It now checks only the loop's initial values, since an
   `again` belongs to its nearest loop (ADR 0015).
10. **Three test witnesses were wrong before they were right.**
    - A sum absorbed the one unproven operation.
    - `%` is not counted as an operation.
    - A content fact about `(% (+ i n) 10)` is not derivable even in the one-result program, so that
      test would have tested nothing about the tuple.

    Each was checked against the one-result program before it was kept.

## 4. Checked, and how each check fails

`emit/prodresult_test.go` has eight tests, and core has one:
- **component law:** each component has its own facts, a *soundness* witness, where one of two
  components overflows and the other does not;
- **length and content:** a frozen component keeps its length and its content facts, each with an
  anti-vacuity half;
- **refusals:** S; a map buffer is linear; R1 and a jump out of a join point; R2, the same buffer
  twice;
- **reader:** a nested loop's `again` is its own.

| planted | caught by |
|---|---|
| every name gets the first component's facts | the component-law witness |
| the exits record nothing | the component-law witness |
| a `let` ending in a jump counts as a value | the component-law witness |
| refinement binds nothing to the names | length, content |
| head facts keyed by the whole loop, not the skeleton | content |
| the projection takes the next component | content |
| a store into a frozen value allowed | S |
| a map buffer not checked | S, map linearity |
| a jump out of a join point allowed | the join-point test |
| a variant out of a scope allowed | the join-point test (R1 half) |
| a nested loop's `again` treated as the outer's | the core test |

**Differential cases, all four targets, answers computed by hand:**
- `prod-count`: build to a bound and return the count, which tables.md §14.3 had named as
  inexpressible; 6307 8408 10809;
- `prod-loop`: a plain loop's tuple, with a component nobody reads; 0 106 201402 1309906;
- `prod-two`: two buffers from one scope, of different element widths on windows; 784 924 1064;
- `json-tree`.

**`go run ./cmd/check`**:
- **emission:** 13 changes, all expected: tree's emitted Go, and the three new cases (empty packages,
  since a case exports nothing);
- **proof totals** unchanged: 2007 of 2054 operations, 343 of 381 loops;
- **differential and tooling:** pass.

## 5. What it cost

**`TreeGen`** (`BenchmarkTreeGen`, 20,000×, five runs, alternated in two rounds):

| | ns/op |
|---|---:|
| before (header stores) | 4,402–4,496 |
| after | 4,576–4,747 |
| after, with the two header stores put back into the exits | 4,356–4,388 |
| hand-written `TreeFlat` | 4,441–4,491 |

The new code is 3–4% slower, and the controlled row explains it. The new code does strictly less work;
putting back two stores that run once per call makes it *faster* than before, and they cannot account
for 200 ns. What they change is the size of the exits at the top of the parse loop, and so where the
hot part of the loop lands. That is layout inside the function. It survives three different paddings
of the function's start, so it is not the function's alignment. It is below the 15% noise floor no
decision rests on, and the generated parser is at **1.04×** hand-written (1.06× before). Allocations
are zero in both, and Go's bounds checks are the same 14.

- **Compile time:** 1.09× the baseline (median).
- **Balance:** +1,642 lines of compiler (+224 of tests) against +21 −15 in `examples/` and +105 in
  differential cases. The compiler grew most, as the assessments keep warning, and the one program
  that asked for it is smaller.

## 6. Not built, and found

- **An `again` inside a join point's body**, and **a tuple-valued loop or scope as an export's
  result**. Both are refused by name (tables.md §2.5).
- **A float component on windows** gets an XMM place by `isFloat` at the first tail. No case has one,
  so that path is untested.
- **Found, not fixed: an unproven index can be accepted.** An index outside the linear fragment, such
  as a table read, that the refiner cannot prove is "propagated" with a note, and the program is
  emitted. `(t (b 3))` with `b`'s cells up to 9 and `t` of length 6 compiles, and Go's own bounds check
  is all that stands between it and a wrong read (refine.go, `indexObligation`). That contradicts
  "what cannot be proven is refused" and wants its own session.
- **Found, not fixed:** a dynamic index into a literal table carries no bounds obligation in the
  refiner. The emitter refuses the shape anyway, so nothing wrong is emitted.
