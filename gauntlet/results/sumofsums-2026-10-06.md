# A sum of sums reduces, and a table's element is its least solution

2026-10-06. The error model needs a failure kind inside a result, `(err not-found)`: a sum in another
sum's payload, A + (K₁ + K₂). Written that way, it did not compile. sums.md §4 claims nested sums
reduce, and its test nests them **in sequence**: a sum, eliminated, producing a sum, each payload an
integer. A sum inside a payload had never been written. Closing it took four fixes, two in the reducer
and two in the type checkers. Each was found by the one before it, and each has a planted fault.

## 1. The reducer, arms without effects: the recount

```lisp
(variant (result T E) (ok T) (err E))
(variant kind small big)
(def g (n) (if (< n 5) (ok n) (err (if (< n 10) small big))))
(def f (n) (case (g n) (ok v) v (err k) (case k small 1000 big 2000)))
```

reduced to

```lisp
(fn (n) (if (< n 5) n
  (let (if (< n 10) (fn (#x) (#x 0 0)) (fn (#x) (#x 1 0)))
       (fn (#p) (#p (fn (#t #p1) (if (= #t 0) 1000 2000)))))))
```

The eliminator names its payload in **every** arm, `ok`'s `v` and `err`'s `k`. So β counted two
occurrences of the conditional payload, found it not duplicable, and let-bound it. Folding the known
tag, `(= 1 0)`, then removed the `ok` arm. That left a binding whose variable occurs once, holding an
eliminator applied to a **variable**, which neither commuting conversion reaches.

**The fix: recount after normalising.** A binding β made only because a pure value occurred more than
once is counted again in the normalised body. With one occurrence left, it is substituted and the body
normalised again, so case-of-case sees the `if` in operator position. With none left, it is dropped.
That is β's own rule ("one occurrence or none: substituting cannot duplicate anything") applied after
the fact, and sound for the same reasons: the value is pure, and one occurrence duplicates nothing
(ADR 0010). Impure bindings and table-read bindings are never recounted.

## 2. The reducer, arms with effects: a read under a λ

With `Println` in the arms, the same program stayed stuck after the recount. The binding came from the
**table-read** path: a read through a bound variable may not move into a body with effects, which is
what keeps a buffer swap from becoming a copy (bufread_test.go). But a constructor is
`(fn (#x) (#x tag payload))`, and `readsBoundTable` descended into it and read `(#x 0 0)`, the λ's
application of **its own parameter**, as a table read.

**The fix: the check stops at a λ.** A read inside a λ runs where the λ is applied, and substituting the
term moves no application. That is the reason β already exempts an argument that **is** a λ, applied to
a λ nested inside one.

## 3. The term checker: a loop's exits are typed after its stores

With both reducer fixes, `tally`'s acceptance test failed:
`(cs …) is go.bytestring, but string is required here`. The residual had changed order, which exposed
an order-dependence in typing:
- A `build` filled its buffer in a loop whose clause chain lists the exits first,
  `(>= i (len ls)) cs`. So the loop's value was typed before the store that solves the buffer's open
  element: the build froze to `array ?`.
- The frozen table was then solved by whichever **read** the checker met first. Before, that was
  `concat`, wanting `string`. Now it was `strings.Compare`, wanting `bytestring`.

A read constrains an element **from above** (σ ≤ ω); a store **from below** (τ ≤ σ). The principal
solution is the least, the join of what is stored. **The fix:** a loop variable's type is one thing
across the loop, so when the first walk of a clause chain leaves the loop's value with an open element,
the chain is walked again with the variables' solved types in scope. Typing is idempotent, so the
second walk only reads what the first learned, and it runs only when the value is still open.

## 4. The IR's typing: a parameter's type is an upper bound

A test of exactly that shape then passed the term checker and failed the verifier:
`W5: %32 is go.bytestring and flows into concat's argument 1, which is string`. The IR types by
unification, and it unified an argument with its parameter's type **where the call was walked**: an
equality, for what is an upper bound. **The fix:** parameter demands are collected and applied after
every defining constraint (constants, call results, stores, a build's buffer) has given its lower bound.
They fill only what is still unknown, and a known type meets its demand at W5, which judges the flow by
subsumption.

## 5. Witnesses

| rule | test | planted fault | caught |
|---|---|---|---|
| the recount | `TestASumOfSumsReducesWithPureArms` | no recount | the residual keeps `(let (if …) …)` |
| a read under a λ is not a table read | `TestASumOfSumsReducesWithArmsThatHaveEffects` | the check descends into λs | the same, with effects |
| the recount moves only a pure value | `TestTheRecountDoesNotMoveAnImpureValue` | impure bindings recounted too | the impure call leaves its binding |
| a loop's exits typed after its stores | `TestABuildCarriesTheElementItsStoresSolved` | no second walk | `(cs …) is go.bytestring, but string is required` |
| a parameter demand is an upper bound | the same test | demands applied where met | `W5: … go.bytestring … flows into concat's argument 1` |

The sum of sums reduces to the flat sum's residual,
`(fn (n) (if (< n 5) n (if (< n 10) 1000 2000)))`. By associativity of the coproduct,
A + (K₁ + K₂) ≅ A + K₁ + K₂, so it should. The error model's file reporter, written with
`(variant (result T E) (ok T) (err E))` and a kinds variant, now compiles to Go identical to the flat
version's.

## 6. Cost

| | |
|---|---|
| compiler | `core/reduce.go` (the recount; the λ stop), `emit/check.go` (the second walk), `ir/typing.go` (deferred demands) |
| emission | byte-identical on all 492 runs; proof counts unchanged, 1,970 of 2,015 and 347 of 365. No program in the corpus had a sum of sums |
| compile time | 1.26× the baseline in the gate's parallel sweep, under its rule; no compile flagged |

## 7. Not built

- **A sum computed by an impure term, or used twice, inside a program** stays bound, and a `case` on it
  stays stuck (the emitter reports it). Reducing it needs a run-time tag and payload inside the program,
  as at an export boundary. Nothing needs it yet.
- **The term checker's read-then-store order on one table.** A table whose first constraint is a read and
  second a store is still solved by the read. No program has that shape; the loop-exit case was the one
  `tally` had.
