# Loops: candidates

> **Reordered 2026-08-18 by measurement.** The
> [go/builtin experiment](../../gauntlet/results/go-toplevel-2026-08-18.md) put a number on the
> gap this document only described: a sieve written against Go's top level is **860× slower** than
> hand-written Go, and hand-written Go *restricted to the loop shapes `fold-range` can express* is
> **1117×** slower. Our emitted code is 0.77× of that restricted form — so **none of the gap is the
> emitter and all of it is the loop.** The prize is in the loop's ITERATION SPACE — a start, a step,
> and early exit — not in its accumulator. §6's order is updated; §5's candidates are not yet
> changed, because the measurement sizes the prize without choosing the design.
>
> **Replicated on a second host.** The
> [js/global experiment](../../gauntlet/results/js-toplevel-2026-08-18.md) ports the same sieve to
> JavaScript: **445×** for the loop shapes, and our emitted code is **0.56×** of hand-written code
> written under the same constraints — nearly twice as fast as a person so restricted. Two hosts,
> two compilers, one cause. **This is the language, not a host.**

**Status: open question, not a decision.** Written in the mode
[ADR 0007](../decisions/0007-exploration-over-specification.md) asks for — candidates against a
fixed test, arguments only to select what is worth measuring.

Loops arrived without a discussion. `fold-range` was added because the first gauntlet program needed
it, `fold-range2` because the second one did, `make-vec` because a program could not construct data
([construction.md](construction.md)). Three primitives, no design document — and since
[ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) removed recursion they are **the
only way to iterate at all**. That puts them at the level of `fn`, `def` and `module`, and they have
had none of the scrutiny.

This document says what exists, what cannot be written, what the literature offers, and what
today's measurements decide.

---

## 1. What exists

Three structural primitives — named in target files, implemented in the backends:

```lisp
(fold-range  z n f)                    ; f : (acc, i) -> acc
(fold-range2 x0 y0 n fx fy finish)     ; fx, fy : (ax, ay, i) -> a ;  finish : (ax, ay) -> r
(make-vec    n f)                      ; f : i -> element
```

`fold-range` is the whole of iteration. `fold-range2` is the same loop with **two** accumulators and
a finisher. `make-vec` writes an array of length `n`.

**`fold-range2` is the smell this document starts from.** It exists because the core has no
products: folding a pair would allocate a pair per iteration, so the loop grew an arity-specific
second version. [structs-2026-08-14 §7](../../gauntlet/results/structs-2026-08-14.md) already said
so — *"a third accumulator needs `fold-range3`"* — and named two possible general answers without
deciding between them. §5 decides.

## 2. What cannot be written

| shape | status | why it matters |
|---|---|---|
| `map`, `reduce`, `zip` over a range | **yes** | the gauntlet |
| build an array | **yes** — `make-vec` | |
| `filter` | **yes**, as a push collection ([q5b](q5b-filter.md)) | |
| two accumulators | **yes** — `fold-range2` | does not generalise |
| **three or more accumulators** | **no** | |
| **a loop start** — `for j := i*i` | **no** | **1117×** on Go, **445×** on JS |
| **a loop step** — `j += i` | **no** | same |
| **early exit** — `find`, `any`, `all`, `takeWhile` | **no** | every search program |
| **`while`** — trip count unknown at entry | **no** | convergence, streaming input |
| **`scan`** — prefix sums | **no** | Blelloch's primitive; compaction and sort need it |
| **reverse or strided iteration** | **no** | |

And one property that is easy to miss: **nothing ever unrolls.**

```lisp
(fold-range 7 0 (fn (acc i) (+ acc i)))   ⟶   (fold-range 7 0 (fn (acc i) (+ acc i)))
```

A loop with a literal trip count of **zero** survives into the output. A partial evaluator declines
to evaluate the one construct whose trip count it can see. That is not obviously wrong — unrolling
interacts with [ADR 0009](../decisions/0009-staging-preserves-results.md) and with the
[inlining budget](../../gauntlet/results/size-2026-08-13.md) — but it has never been argued either
way.

## 3. What the literature offers

### 3.1 `fold-range` already has a name, and it is a strong one

`fold-range` is **Gödel's System T recursor**:

```
R : A -> (N -> A -> A) -> N -> A
```

So the current language is, at the value level, **exactly primitive recursive**. The same boundary
in imperative dress is Meyer & Ritchie's **LOOP language** (1967): `LOOP n DO … END` computes exactly
the primitive recursive functions, and `WHILE` is what adds the partial ones.

Worth stating as a *guarantee* rather than a limitation:

> **Every loop in an Oroboros program terminates.** Not by analysis — by construction. The trip
> count is evaluated before entry and the body cannot change it.

Ackermann is not expressible. Nobody has asked. What *is* missing is search, and §3.3 is the cheap
way to get it without giving the guarantee up.

### 3.2 The recursion schemes, and what `fold-range2` really is

Bird–Meertens formalism; Meijer, Fokkinga & Paterson, *Functional Programming with Bananas, Lenses,
Envelopes and Barbed Wire* (1991). The relevant vocabulary:

| scheme | what it is | here |
|---|---|---|
| **cata** | fold | `fold-range`, over the range functor |
| **ana** | unfold | absent; a generator |
| **hylo** | unfold then fold, no intermediate structure | **what our fusion produces** |
| **para** | fold with access to the original substructure | roughly what the stencil wants |
| **banana-split** | `⟨cata f, cata g⟩ = cata ⟨f, g⟩` — two folds in one pass | **`fold-range2`, at n = 2** |

The last row is the find. `fold-range2` is not an ad-hoc primitive; it is the **tupling law**,
instantiated once. The law's general form is a fold whose accumulator is a *product* — exactly the
thing we lack — and the reason two separate step functions appear, rather than one step returning a
pair, is that a step returning a pair would need multi-value return or allocation.

The hylo row explains something we already observed: `q5`'s "fusion by δ+β with no fusion rules" is
**hylo-fusion**, and deforestation is its corollary. We did not avoid needing fusion rules by being
clever; we picked a scheme whose fusion law is an identity.

### 3.3 Bounded loop *with* early exit — the cheap extension

Clojure's `reduced`, Rust's `try_fold` / `ControlFlow`, Scala's `foldLeft` over `Either`, Haskell's
lazy `foldr`. All the same idea: the step may say *stop*.

The critical property: **a bounded loop with early exit is still bounded.** It still terminates by
construction, it is still primitive recursive, and it buys `find`, `any`, `all`, `takeWhile` and
linear search. All three hosts have `break`, and [CLAUDE.md](../../CLAUDE.md) already anticipated
`break n` in its list of permitted control flow.

This is the highest ratio of programs-gained to guarantees-lost in the document, because nothing is
lost.

### 3.4 `while` — the expensive one

Full partial recursion. `(while state cond step)` emits `for cond {}` on all three hosts, and:

- the termination guarantee goes;
- the reducer cannot unroll it, bound it, or fuse across it;
- it is [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)'s named prerequisite for
  tail-call optimisation, and for recursion ever returning.

Wanted eventually. Not wanted before §3.3, because most of what people reach for `while` for is a
bounded search.

### 3.5 Array combinators — the library layer, not the core

Futhark's **SOACs** (second-order array combinators), Blelloch's NESL, Accelerate, APL: `map`,
`reduce`, `scan`, `filter`, `iota`, `stencil`, `histogram`, with algebraic fusion rules.

Almost all are definable from `fold-range` + `make-vec` today. The exception is **`scan`**, and it is
the interesting one: a prefix sum writes an output *while* folding, so it needs a loop carrying both
an accumulator and an array — the product-accumulator problem again.

Blelloch's point stands: `scan` is the primitive that unlocks compaction, sorting and sparse
operations. It belongs in the *library*; what it needs from the core is §5.

### 3.6 Stream fusion, staged — the closest existing work

Coutts, Leshchinskiy & Stewart, *Stream Fusion* (2007). And much more directly: Kiselyov, Biboudis,
Palladinos & Smaragdakis, ***Stream Fusion, to Completeness*** (POPL 2017), which builds fully-fused
stream pipelines **using staging** — MetaOCaml — with no residual closures.

That is our mechanism, not an analogy. Their findings that bear on us:

- **push** streams handle `map`/`filter`/`flatMap` trivially — exactly what [q5b](q5b-filter.md)
  found by hand;
- **pull** streams are required for `zip`, because `zip` must advance two sources independently;
- supporting both, plus early termination, is where the complexity lives.

We currently have push (`examples/filter.oro`) and a delayed pull vector (`lib/num/vec.oro`) as
*library encodings*, discovered rather than designed. This paper is the map of the territory we have
been walking around in, and it says `zip` and `takeWhile` are where the encoding starts to bite.

### 3.7 Named, not proposed

**Polyhedral / schedule languages** — Halide, TVM: separate the algorithm from the loop schedule
(tiling, fusion, vectorisation). Where a stencil goes to get fast, and a research programme rather
than a candidate.

**`goto` / irreducible control flow** — already refused; three of the initial targets cannot express
it.

## 4. Measured today

The question §5 turns on is whether a loop may carry a **product** accumulator. That is answerable by
measurement, and had not been answered: *"the measurement does not decide between them"*
([structs-2026-08-14 §7](../../gauntlet/results/structs-2026-08-14.md)).

One pass, two accumulators, n = 1024, medians. Fresh product per iteration versus two scalars:

| host | two scalars | fresh product | penalty |
|---|---|---|---|
| **Go** | 97.5 ns, 0 allocs | 97.6 ns, **0 allocs** | **1.00×** |
| **JVM** | 464 ns | 2981 ns | **6.4×** |
| **JS** (`Float64Array`) | 552 ns | 7611 ns | **13.8×** |
| **JS** (`Array`) | 488 ns | 7413 ns | **15.2×** |

**Go does SROA for us and the other two do not.** Go's SSA scalarizes the struct accumulator
completely — zero allocations, timing inside the noise floor. The JVM's escape analysis does not save
a record carried across iterations. V8 does not come close.

Two side results.

**The simultaneous-update temporaries are free.** Our emitted JS and Java write
`const u = ax + …, v = ay + …; ax = u; ay = v;`, because `fold-range2`'s step functions may each read
both accumulators. Against the compound form a human would write: 581 vs 552 on JS typed arrays,
463 vs 464 on the JVM. Inside the noise floor on both. The parity claim stands.

**A methodology correction, recorded because it nearly became a finding.** The first three
measurements said those temporaries cost 1.88× on JS. They were wrong: the harness dispatched every
variant through one shared call site and mixed `Array` with `Float64Array`, so it measured V8's type
feedback rather than the loop. Isolating each variant in its own process — one array kind, one
monomorphic site — removed the effect entirely. **A benchmark that dispatches through a shared call
site measures the dispatch.** The same hazard applied to the JVM, where a `switch` inside the timed
loop had to be removed as well.

## 5. Candidates

### C1 — grow the family: `fold-range3`, `fold-range4`, …

Honest generalisation of what exists. No allocation; emits to n locals.

**Against.** The arity leaks into library code — a programmer writing `centroid` must choose
`fold-range2` — which structs-2026-08-14 §7 already called *"the first place requirement 7 has been
dented"*. Each arity is a new structural primitive in three backends and the checker. And it never
reaches `scan`, whose state is an accumulator *and* an array.

### C2 — one accumulator of a product type, scalarized by us

`fold-range` alone, with a product in the core, and a residual→residual **SROA** rule splitting a
product-typed loop accumulator into one local per field. `fold-range2` dies; `scan` becomes
expressible. It is banana-split's general form rather than one instance of it.

**§4 decides this one.** SROA must exist anyway for two of the three hosts — 6.4× and 13.8× are not
prices this project pays — and once it exists, C1's family is redundant. Go needing none of it is not
an argument against: it is the parasite model working, and the emitter can decline to bother on Go if
that ever measures better.

This is also [q5](q5-do-we-need-rules.md)'s prediction arriving — the one thing δ+β cannot do is a
residual→residual rewrite, and this is one.

**Against.** It needs a product in the core, the largest single addition anyone has proposed, and it
interacts with [ADR 0013](../decisions/0013-accept-the-allocation-price.md)'s open question. It
deserves its own ADR.

### C3 — bounded loop with early exit

Orthogonal to C1/C2, and cheap:

```lisp
(fold-until z n f)      ; f : (acc, i) -> acc, plus a way to say done
```

The open question is *how* the step says stop without a sum type in the core — a second predicate
argument, a sentinel, or a `(done v)` marker the primitive recognises. All three hosts have `break`.

Buys `find`, `any`, `all`, `takeWhile`, linear search. Costs **nothing**: still bounded, still
terminating, still primitive recursive.

### C4 — `while`

Everything C3 has, plus non-termination. Enables convergence and streaming; disables the termination
guarantee, unrolling and bounding. Prerequisite for TCO and for recursion's return.

**Defer until C3 exists**, because C3 covers most `while` uses and is free.

### C5 — unrolling a statically-bounded loop

Not a new primitive; a reduction question. Should `(fold-range 7 0 f)` reduce to `7`?

**For:** it is what a partial evaluator is for; a zero-trip loop in the output is embarrassing; a
small literal count unrolled would fuse with its neighbours.

**Against:** [ADR 0009](../decisions/0009-staging-preserves-results.md) — compile-time arithmetic
must be bit-identical to runtime, and unrolling *moves arithmetic across the stage boundary*, which
is exactly where that rule was written to bite. And the
[inlining budget](../../gauntlet/results/size-2026-08-13.md) found a sharp discontinuity: above the
host's cost budget, specialisation costs up to 14.4× in size for a win the host was declining.

**A cheap first step with none of the risk:** reduce a loop whose trip count is a literal `<= 0` to
its initial accumulator. No arithmetic crosses the boundary, and no code grows.

## 6. Recommendation

In this order, because each step is cheap and informs the next:

> **Updated 2026-08-18.** The iteration space moved to the front, because it is worth 1117× and
> everything else on this list is worth between 1.8× and 14×.

1. **The iteration space: a start, a step, and early exit.** Not listed as a candidate above,
   because the document was written before the number existed. `fold-range` always begins at 0,
   always steps by 1, and always runs to its bound; a sieve therefore does O(n²) work where Go does
   O(n log log n). Whether that is a richer `fold-range`, a `loop`/`break` expression, or `while` is
   exactly what §5 has to decide — but it is the thing to decide *first*.
2. **C5's cheap half.** A provably-zero-trip loop reduces to its initial value. One rule, no risk,
   and it marks the staging question without answering it.
3. **C2 — one accumulator, product type, SROA.** §4 makes this necessary rather than optional. Its
   own ADR, argued against ADR 0013; it retires `fold-range2` and unlocks `scan`. The
   [go/builtin experiment](../../experiments/go-toplevel/README.md) wants the same product for a
   different reason — `v, ok := m[k]` — and two independent demands for one feature is the
   strongest evidence it is real.
4. **C4 — `while`.** After the above, with a specification of what it means for the termination
   guarantee to be gone.

**C1 is rejected**: §4 shows the machinery C2 needs must be built anyway, so growing the family buys
time at the cost of three backends' worth of code that C2 then deletes.

## 7. What would kill each of these

Per [ADR 0008](../decisions/0008-measurement-over-principle.md), each candidate needs the measurement
that would refute it:

| candidate | killed by |
|---|---|
| C2 (SROA) | an emitted shape the hosts *already* scalarize — i.e. if §4's penalties can be removed by emitting differently rather than by splitting the accumulator |
| C3 (early exit) | a `break` in a loop costing measurably more than a bounded loop on any host |
| C5 (unrolling) | any program where unrolling changes an answer — an ADR 0009 violation, not a performance result |
| C4 (`while`) | nothing yet: no program needs it, so there is nothing to measure |

And the gauntlet needs an eighth program that **searches**, because none of the seven does, and a
loop design chosen against tests that never exit early would be chosen blind. The sieve is a
candidate: it is small, it is famous, its fast form needs all three missing pieces at once, and
there is now a hand-written reference and a measured number for it.
