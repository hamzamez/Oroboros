# Assessment: the analysis round, and what to decide next

Dated snapshot after booleans, the windows target, and the integer-analysis chain.
**Deliberately not an ADR**, for the same reason
[assessment-2026-08-13](assessment-2026-08-13.md) and
[assessment-2026-08-19](assessment-2026-08-19.md) are not: naming a design as a decision before
measurement is the predecessor project's failure in ADR form.

Everything in §1 is read off the code.

---

## 1. Where the language is

**Seven term kinds. Three reduction rules. Two parameters.** `fn`, plus reader sugar that all
erases — `let`, `seq`, `loop`, `and`, `or`, `not`, `cond`. **Two structural primitives a target
declares**: `let` and `loop`. `if` is the language's.

| | |
|---|---|
| ADRs | 17 |
| targets | 4 native, 3 portable, blas, 3 tutorial |
| backends | Go, JavaScript, Java, x86-64 |
| `core/` + `emit/` | 12,196 lines |
| tests | 106 passing |
| examples | 29 |
| measurements | 26 in `gauntlet/results/` |

**New since the last assessment:** booleans in the language
([ADR 0017](decisions/0017-booleans-are-in-the-language.md)); an interval analysis; size-change
termination; representation selection driven by a declared range; and an integer corpus built to be
**refused** as much as proven.

## 2. The thing to decide first, because it happened by accident

**Wiring representation selection into `cmd/gen` and `cmd/build` changed the language's semantics,
and no ADR decided it.**

[ADR 0012](decisions/0012-portable-integer-range.md) says: *outside the portable range the
behaviour is the target's.* On Go that has always meant wrapping. With selection on by default, an
operation the compiler could not bound was rewritten to a checked form, and this program stopped
wrapping and started panicking:

```lisp
(loop ((x 1) (i 0)) (go.>= i 200) (fmt.Println x) else (again (go.* x 3) (go.+ i 1)))
```

Three things were wrong with that, and only the first is obvious.

**It contradicts a written ADR.** Not a gap — a stated decision, silently reversed.

**It violates requirement 5 by default.** [checkcost-2026-08-19](../gauntlet/results/checkcost-2026-08-19.md)
measures the cost at **1.23× to 4.54×**, and a 4.54× regression on an arithmetic loop for a program
that asked for nothing is a flat breach of *as fast or faster than hand-written*.

**And it made cross-target divergence worse, not better.** JavaScript declares no checked form. So
the same program, past 2⁵³, now **traps on Go, Java and windows and silently loses precision on
JavaScript** — a *new* divergence, introduced by a change whose purpose was correctness, and
documented nowhere.

**Fixed in this round:** the rewrite is now behind `-checked`, off by default. The analysis still
runs and still reports what it proved, because that is free and useful:

```
note: 1 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
```

The mechanism did not need to be *on* to be proven, and turning it on should be the **consequence**
of deciding exact integers, not the cause. That is §5's first item.

The general lesson is worth keeping: **a demonstration wired into the default path is a decision,
whether or not anyone made one.**

## 3. What the round established

Five things, each measured rather than argued, and each of which changed what should happen next.

**A range is already writable, and reading it is worth double.**
`(sig f ((n int)) int (where …))` has parsed since refinements were built. Nothing read it for
anything but array bounds. Reading it took integer operations provably inside the portable window
from **39% to 81%** ([intervals](../gauntlet/results/intervals-2026-08-19.md)), and then
size-change termination took it to **54% undeclared and 100% declared**
([sct](../gauntlet/results/sct-2026-08-19.md)) with **96% of loops proven to terminate**.

That last number closes something ADR 0015 asserted and nothing computed — *termination is a
computed program property* — and it closes [concerns.md §2.1](spec/concerns.md)'s admission that
our termination guard was "a mechanism, not a proof".

**The justification for types moved, and the new one is stronger.**
§1–2 of [types-direction.md](types-direction.md) killed the performance case for types correctly:
our proofs do not transfer. A **range** does not need to transfer — *it changes what we emit*. The
host never sees the exact-by-default semantics, only the artifact. That is the one job no host
compiler can do, which is the same argument that already justified the residual type checker.

**Our lineage is Dependent ML, and the measurement rediscovered its design point.**
Xi & Pfenning, 1998: indices from a decidable constraint domain, erased at runtime, annotation only
at **function boundaries**. `emit/refine.go` was a baby DML built here without the pedigree, and
"declare the parameters, infer the rest, and the number doubles" is DML's own claim, found twice.

**Sequent calculus gives a classification, not a core.** GHC built Sequent Core, measured it, and
shipped join points in direct-style Core instead — and `again` already *is* a join point. What is
worth taking is **polarity**: a product eliminated by projection need never be built, which is
exactly why the product measured 1.01× with zero allocations and why `(value, error)` will not.

**And the corpus is what finds bugs.** Six integer programs, three of them written to be *refused*
— Collatz, Fibonacci past 2⁵³, exponentiation that genuinely overflows. Growing it found four gaps
invisible on the sieves, including that **`go./` was not recognised as division at all**.

## 4. Are we on the right track

**Yes, and the method is now the strongest thing here.** Every significant result this round came
from building something and looking at *why* each case failed rather than at the headline number:

- the first interval run said 10–20%, and that was a missing narrowing phase, not a property of the
  programs;
- three bugs in the size-change work all made the numbers **worse** than the truth, which is the
  only reason none was mistaken for a result;
- the isolated cost measurement was wrong in **both directions** against the real-program one.

That last one is the round's most transferable finding. [bce-2026-08-15](../gauntlet/results/bce-2026-08-15.md)
already recorded that a 1.96× *win* in isolation vanishes on memory-bound loops. Now a *cost*
behaves the same way. **Neither a saving nor a price survives being quoted without its condition**,
and this project has now been caught by that twice.

### Three risks, named

**The compiler is growing faster than the language.** `emit/` is now interval analysis, size-change
termination, a decision procedure, a type checker and four backends. That is not a language
problem — but the ratio deserves watching, because a language whose claims depend on a clever
compiler is a language whose claims are hard to check.

**The annotation dissolves on inlining.** A library's `sig … (where …)` is checked at the definition
and lost when the definition is reduced into a caller, because the analysis runs on the exported
unit. `cmd/build`'s unit is always a nullary `main`, so for a whole program the ranges come from
**literals** — which reduction supplies for free — and the annotation only bites for library
functions compiled by `cmd/gen`. Both directions of one mechanism, and neither is designed.

**And the honest risk is unchanged from the last assessment.** The corpus is ten integer programs
and seven gauntlet shapes. **Nothing here is a program anyone would want to run.** The measurements
are real; they are measurements of a language that has still not been asked to do anything awkward.

## 5. What is next

**1. Decide the integer semantics, then turn selection on as its consequence.**
[data-model.md §7](spec/data-model.md) lists eleven questions that must be settled before any
integer is implemented — representation, overflow, division rounding, remainder sign, division by
zero, int/float comparison, the lossy `int → float`, `float → int`, equality, ordering, constant
folding. Every one has hosts that disagree. They should be answered **in a specification first**,
which is the order booleans followed and the order [strings.md](spec/strings.md) exists to enforce.

The evidence for that specification now exists and is unusually complete: what the checks cost in
isolation *and* in real programs, how much is provable with and without a declaration, what each
host can do natively, and what a bignum costs when the check fails.

**2. The product.** Six independent demands, measured affordable — 1.01× on Go with zero
allocations, scalar-replaced on Java, 1.11× on V8 — and now with a *rule* for which ones are free:
**negative first**, because a product eliminated by projection need never be built. This is the
single most-demanded missing thing in the language and it is the one item with both a measurement
and a theory behind it.

**3. `bytes` and scalar bitwise.** Blocked on nothing, no language change, the `vec-f64` pattern.
They are what make binaries, hashes and bit sets writable, and without an integer container the
interesting half of [bitwise.md](spec/bitwise.md) stays theoretical.

**4. Move the gauntlet onto the native targets.** Still the one piece of process debt, still the
cheapest available surprise, and now overdue: seven programs on a layer we declared retired, while
every result this round came from programs outside it.

**5. Write something awkward.** Not a feature. The least-tested claim in the project is that this
language is pleasant to write real programs in, and no amount of analysis substitutes for finding
out.

### Deliberately not next

**Exact integers as the default**, until item 1. **A sequent-calculus core** — GHC's precedent.
**Full dependent types** — Low\*'s lesson is that the restriction is the mechanism. **Liquid
inference**, until the annotation burden is measured to be too heavy rather than argued to be.
