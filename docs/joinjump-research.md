# Research: an `again` after a nested loop's join point

2026-11-19. The 10-24 plan's open item ([assessment-2026-10-24](assessment-2026-10-24.md) §5): "an
`again` after a nested loop's join point (tables.md §2.5): 16% of `wc` against idiomatic Go; research
it as a join point that may jump". **Research, no decision, nothing built.** The decisions are §6's,
hamza's.

## 0. The question and its price

`(a, b) = loop … { … }; …; again(…)`: an enclosing loop's step taken after a nested loop whose value
is a tuple. tables.md §2.5 refuses it ("a jump out of a join point", `emit.CheckJoins`). So a program
with a loop per chunk inside a loop over chunks, which is how Go writes `wc`, is written as one flat
loop carrying every variable instead.

**Measured** ([wc-2026-10-24](../gauntlet/results/wc-2026-10-24.md) §4, ns per call on the same
input):

| form | ns | ratio |
|---|---:|---|
| generated `wc`, one flat loop | 607 | |
| hand-written Go, the same flat shape | 601 | generated / it = 1.01 |
| hand-written Go, a rune loop nested in the read loop | 519 | flat / nested = **1.16** |

The printer is at parity, and the 16% is the shape: the flat loop carries 16 variables, where the
nested inner loop keeps 5 in registers. That is the price of the refusal on the one program measured.

## 1. What the term is

A tuple pattern over a producer, `(P (fn (a b) k))` with P a loop or a scope that yields a product,
is lowered as a **join point** (ADR 0031 §4; Maurer, Downen, Ariola and Peyton Jones, "Compiling
without continuations", PLDI 2017). Each exit of P assigns the pattern's names, and k runs once,
after P, instead of being copied into every exit. The question is whether k may end in `again`, a
jump to the loop enclosing P.

**In System F_J it may.** Maurer et al. type a jump by a label context Δ:
- a jump to a label in Δ is well-typed only in a tail context;
- Δ is kept through the tail positions of a term and emptied at every non-tail position;
- a recursive join point, `joinrec`, is a loop, and its jumps are its back edges.

The enclosing loop is a `joinrec` L, and `again` is `jump L`. k is the body of a join point j bound
at the elimination. If the elimination stands in a tail position of L's body, then k's tails are tail
positions of L's body too, and L stays in Δ throughout k. So `jump L` inside k is well-typed exactly
when the elimination is a tail of the enclosing loop's body.

## 2. Theorem J: what it means

**Theorem J.** Let `E = (P (fn x̄ k))` stand in a tail of a loop L's body. Then E has the meaning
of E′, P with k substituted at each of its exits, its exits' components for x̄. So every `again` in
k is a tail jump to L.

*Proof.* The commuting conversion of the product's eliminator with P's tails, case-of-case, as in
ADR 0076 item 2:
- P runs one exit, once, on each path, and the eliminator applies k once to that exit's components.
  That is k[ēᵢ/x̄] in place of exit i.
- E′ is a clause chain over the four forms: P's leaves are its exits and its `again`s, and each exit
  is now k's body. An `again` of L in k stands in a tail of E′, and E′ stands in a tail of L's body.
- An `again` of P's own loop stays P's: k is placed only at P's exits, never at P's back edges.

The join point is E′ with k shared rather than copied, which is what Maurer et al.'s join points
are for: sharing a continuation without changing what it computes. ∎

So the meaning is already decided, by ADR 0031's join point and ADR 0076's commuting conversion.
What is not built is the representation, and that is what the walkers see.

## 3. Why it was refused: the walkers

"Every walker of a clause chain walks all four forms, or it is unsound" (CLAUDE.md). The four:
- `again`;
- `if`, a conditional;
- `let`;
- a host call's continuation.

An `again` inside a join point's body would be a **fifth**: a leaf of the enclosing loop's clause
chain reached through a product elimination of a producer. A walker that does not know it misses a
back edge, and what each walker then gets wrong differs:
- **size-change termination** would miss a call of the loop, so a non-terminating loop could be
  proven to terminate;
- **the intervals at a loop's head** would miss the state the `again` passes back, so a head fact
  would be too narrow: an unsound proof of an operation;
- **the refiner's back-edge skeleton** (`content.go`) would miss a back edge;
- **the exits' projections** (`projectExits`) would read the `again` as an exit.

Each of those is unsound, not imprecise. A census by name finds **28 sites in 20 files** that test
for `again` (`core`: desugar, the J reader and printer, place, read, recerase, record, reduce,
scope; `emit`: check, linear, linearity, monotone, numnames, post, prodresult, refine, translation;
`ir/lower`). Not all of them walk clause chains, and each would be read before it is trusted.

**The IR is not in the way.** It is structured SSA (ADR 0032): the inner loop's region yields its
tuple, the join point's statements follow it in the enclosing loop's body region, and a `continue`
there is the outer loop's own structured edge. A single-valued nested loop followed by `again`, the
`let` form, is built and printed today. The gap is above the IR: the core's and emit's walkers, and
`CheckJoins`.

## 4. The candidates

- **A. The fifth form in every walker.** Each walker learns the join point's body as tails of the
  enclosing loop, with a planted fault per walker that misses it.
  - *For*: no code is copied, and the lowering is the join point it already is.
  - *Against*: about 20 walkers to read and extend, and one missed is unsound. That is why §2.5
    refused it until a program asked.
- **B. Theorem J as a rewrite, before the walkers run.** The residual's `(P (fn x̄ k))` with an
  `again` in k is replaced by E′, k copied into P's exits, so the walkers see the four forms they
  know. The copy is semantically E (Theorem J).
  - *For*: no walker changes, and soundness follows from the theorem and the four forms.
  - *Against*: code size multiplies k by P's number of exits. That is the copy ADR 0031 chose the
    join point to avoid, and for `wc` the nested loop has a few exits and k is small. The printers
    would print the copy, so the copy may cost what the join saved. Re-sharing it on the IR, where
    a join is a region, is a pass of its own.
- **C. B on the core, A on the IR.** The rewrite feeds the core's walkers, which only need its
  meaning, and lowering recognizes the copied tails and shares them again as one join region, which
  the IR's verifier checks. This takes the soundness argument from B and the code from A. It costs
  the recognition: the copies are equal terms, which the reducer's hash-consing does not promise.
- **D. Keep the refusal.** `wc` stays flat at 1.16×, and every program with a chunked inner loop
  carries its outer state in one loop.

## 5. Planted faults, before any witness is trusted

| rule | the fault | what must fail |
|---|---|---|
| Theorem J's placement | k placed at P's back edges too (B, C) | a nested loop whose inner `again` would also run k: a wrong count |
| the fifth form in SCT (A) | the join body's `again` not a call | a loop that never ends proven to terminate |
| the fifth form at a loop's head (A) | the join body's state not joined into the head | an index proven in range that is not |
| the tail condition (all) | an elimination in a non-tail position accepted | refused, saying the jump would leave an expression |

## 6. For hamza

1. **Whether to build it now**: the only program measured is `wc`, at 16%, and the Go library's
   programs under the standing goal (`cmp` among them) are the next to meet the shape.
2. **Which candidate**: B is the least code and the strongest argument, and costs code size; C adds
   back the sharing; A is the most invasive.
3. **The trigger, if not now**: the first program after `wc` whose flat form a measurement shows
   slower than its nested Go form.
