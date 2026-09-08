# 0020 — A buffer is a nameable type: uniqueness on parameters

Date: 2026-09-06
Status: Accepted — **built 2026-09-07**,
[uniqueness-2026-09-07](../../gauntlet/results/uniqueness-2026-09-07.md): 51 non-comment lines,
zero backend changes, and 18,973 ns with **0 allocations** against 34,927 ns and 128 KB per call
for the same body building its own workspace — **1.84x**, corrected 2026-09-08 from a first
measurement taken at four times the workspace size
([freq-2026-09-08](../../gauntlet/results/freq-2026-09-08.md) §8). Supersedes [0013](0013-accept-the-allocation-price.md);
amends consequence 3 of [0018](0018-immutable-values-linear-buffers.md), on ADR 0018's own trigger.

Research: [uniqueness.md](../uniqueness.md).
Build: [uniqueness-2026-09-07](../../gauntlet/results/uniqueness-2026-09-07.md).

## Context

[ADR 0018](0018-immutable-values-linear-buffers.md) made values immutable and put mutation inside
one scoped linear buffer, and deferred one thing with a named trigger: *"uniqueness types on
parameters … reduction removes every non-exported boundary so buffer reuse already works inside a
program"*. [arrays-revisited.md](../arrays-revisited.md) fired that trigger.

**The demand is one measured case, and the ADR opens with the corrected count rather than the one
the assessment inherited.** Four demands were named; **rule R closed two of them**
([bigreuse-2026-09-02](../../gauntlet/results/bigreuse-2026-09-02.md)) — the mutable bignum and the
fixed-limb library were the *back-edge* instance, where a loop variable's `build` writes into
storage the loop already owns. What is left is the **boundary** instance:

| | status |
|---|---|
| **Karatsuba's workspace** | **live and unmitigated** — 1.07×–1.66×, 10 allocations and 504 KB per multiply |
| ADR 0013's stencil | live as a *portability* gap: reuse is expressible on a native target via `go.set-float64`, and the allocating shape costs 2.71× for hand-written Go too |
| the mutable bignum | closed for the back edge |
| the fixed-limb library | closed for the back edge |

One case, and it understates the stake: [bigarith](../../gauntlet/results/bigarith-2026-08-28.md)'s
whole result is that our bignum beats `math/big` **because it allocates nothing**, and naive
`math/big` is 4–5× worse than careful for exactly this reason. A bignum that allocates half a
megabyte per multiply becomes the thing it beats. **The case is a library entry point, and
[general-purpose.md](../general-purpose.md) says libraries are the point.**

**ADR 0013's trigger 1 is what makes this decidable now.** It was rewritten in August to say
*"someone decides to spend a substructural analysis … whether ADR 0010's structural rules plus the
reducer's occurrence counting suffice is **unmeasured**, and this trigger should not be treated as
fired until it is."* They suffice. `emit/linearity.go` already implements the discipline; it is
seeded from `build`'s binder and nothing else.

## Decision

**A buffer is a nameable type. `(buffer V)` may appear in a signature, and a parameter of that type
is unique on entry and linear through the body.**

```lisp
(sig mul ((a (array int)) (b (array int)) (w (buffer int))) (buffer int))
```

Six rules, stated so they can be checked:

1. **`(buffer V)` is a type in the signature language**, in parameter and result position. It is
   refused today with *"(buffer int) is not a type"*, and that single absence was the whole of
   trigger 2.

2. **Two obligations, at opposite ends, and they are different properties.**
   **Uniqueness in** — the caller holds no other reference. **Linearity through** — the body moves
   it exactly once and may read it freely before. Neither alone suffices: uniqueness without
   linearity lets the body read after a store; linearity without uniqueness lets the caller observe
   the write.

3. **The linearity obligation is checked by `CheckLinear`, seeded from the signature** instead of
   from `build`'s binder. No new analysis.

4. **The uniqueness obligation is discharged by the residual internally and ASSUMED at an export.**
   δ inlines every non-exported call before any check runs, so a non-export parameter survives no
   boundary and its occurrences are all in the residual. At an export the caller is outside the
   program by construction, so the annotation is a published contract — exactly
   [refinements.md §6b](../spec/refinements.md)'s middle row, the trust model an exported `where`
   already has.

5. **No uniqueness ATTRIBUTE is added.** There is no `*T`, no attribute variable, no inferred
   `unique ⊑ non-unique` coercion. The distinction is between two type *constructors*, which
   ADR 0018 already made, and the one coercion Clean infers we already write — the **freeze**.

6. **A buffer may not be an element type.** `(array (buffer V))` and `(map K (buffer V))` remain
   not types. This is load-bearing, not tidiness: see below.

## Why this is small

The same section ADR 0018 needed, and for the same reason — the decision is affordable because
every mechanism exists.

**The check exists.** `emit/linearity.go` walks the residual in evaluation order tracking one bit
per buffer, forks at an `if`, and rejoins conservatively. Seeding it from a signature parameter is
a different argument to the same function.

**The propagation problem does not exist.** Clean and Futhark must propagate and infer uniqueness
through a whole program because they compile modularly; most of Clean's implementation is that
algebra. Whole-program reduction removes it: there is nothing between a definition and its call
sites, because there are no call sites.

**The read-borrow is already free, and rule 6 is why.** Allowing `(b i)` without consuming `b` is
Wadler's `let!` (1990) and Odersky's observers (1992), each of which needed a calculus. Here it
needs nothing, because **no element type is unique**: an observation produces a scalar or a frozen
array and therefore *cannot alias the buffer*. Rule 6 keeps that true. If a table of buffers ever
becomes expressible, the read-borrow needs real machinery and this ADR must be revisited.

**The effect ordering already covers it.** [effects.md §7c](../spec/effects.md) refuses to
substitute a table read through a bound variable into an impure body — deliberately conservative,
written after a swap became a copy — so a declared buffer parameter needs no new rule to keep its
reads from moving across a store.

**And the move check is Rust's ownership without borrowing**, because immutability means there is
nothing to borrow. What Rust adds over this is shared aliased references, which values already give.

## Why not

**(a) Do nothing.** The interior is free, the back edge is closed by rule R, and one library entry
point pays 1.07×–1.66×. *Rejected*, but it is the closest alternative and the reason is narrow: a
bignum that allocates per multiply becomes the thing bigarith measured it beating, and a library is
exactly the case reduction cannot reach.

**(b) Infer it — destination-passing at the boundary.** The compiler adds the workspace parameter
itself; rule R one level up (Shaikhha et al., FHPC 2017; FP², ICFP 2023). Attractive because it is
half built. *Rejected twice over on this project's own precedent*: an optimisation that silently
does not fire is requirement 5's failure mode, **and** adding a parameter to an exported function
changes the published API, so the information must be in the signature anyway — at which point
declaring beats inferring, for [ADR 0019](0019-precision-by-declaration.md)'s reason.

**(c) Clean's uniqueness attributes.** `*T`, attribute variables, `u ⊑ n` subtyping and inferred
coercions. *Rejected explicitly*, because it solves a problem we do not have: any type can be unique
in Clean, so the attribute must be inferred and propagated. Here exactly one type constructor is
linear and the rest are values. Adopting the attribute algebra would buy nothing and cost Clean's
reputation for unreadable signatures.

**(d) Multiplicities on the arrow**, Linear Haskell (POPL 2018). More general, and the right choice
for a higher-order language. *Rejected*: it buys nothing when every arrow surviving reduction is
first-order, and it puts the property on the function where the thing we need to constrain is the
*caller's aliasing*.

**(e) Ownership and borrowing**, Rust. *Rejected*: strictly larger, and what it adds over
uniqueness-plus-free-read-borrow is shared mutable aliasing, which immutability makes unnecessary.

**(f) Runtime reference counting with reuse** — Perceus (Reinking et al., PLDI 2021), Koka, Lean 4.
*Rejected in ADR 0018 (c) and unchanged*: three of four hosts bring a collector, so we would pay
twice and defeat the host escape analysis that
[product-2026-08-19](../../gauntlet/results/product-2026-08-19.md) measured removing an allocation.

**(g) Free mutation.** *Refuted in [arrays-revisited §4](../arrays-revisited.md)*: strictly
dominated, and it would cost the seven properties that now depend on linearity, four of which are
measured results postdating ADR 0018.

## Consequences

**Makes easy.** A library entry point that owns its scratch space. `(alloc (of-array a)) = a`
becomes applicable where the input is declared unique — ADR 0013's fifth trigger, finally reachable
rather than merely sound. And a portable stencil can express reuse, which the retired portable layer
could not, so ADR 0013's price becomes **avoidable by declaration** rather than accepted.

> That is ADR 0019's shape applied to memory: **an accepted default replaced by a declaration.**
> ADR 0019 made precision a thing you ask for; this makes reuse a thing you ask for.

**Makes hard, and this is the honest cost.** Threading a workspace through every `again` and every
helper. It is the complaint Futhark users report, it is unmeasured here, and it is the same
complaint `tree.oro` raises about its explicit stack. It probably wants sugar rather than a
different memory model, and sugar is not decided here.

**Commits us to.** A guarantee at an export that we cannot check — the host may alias. That is not
new (an exported `where` is assumed too) but it is now a *memory-safety* assumption rather than an
arithmetic one, and the ADR should not pretend otherwise. A caller that passes the same buffer twice
gets a wrong answer, silently.

**Does not change.** The core, the reducer, the structural set, `build`, the freeze, or any of the
seven properties in arrays-revisited §3. Seven term kinds before and after.

## What should reopen this

1. **A table of buffers becomes expressible.** Rule 6 is what makes the read-borrow free; without it
   an observation can extract an alias and this needs Odersky's machinery.
2. **The ergonomic cost is measured and is bad.** The trigger is a program in which threading the
   workspace is what makes the program unpleasant — which is
   [assessment-2026-09-06](../assessment-2026-09-06.md)'s *write an application* arriving from
   another direction. Sugar, not a new model, is the expected answer.
3. **A second live boundary case does not appear.** One case justified this. If the corpus grows and
   no second case arrives, the feature is carrying its weight on a single measurement and the
   decision should be re-examined rather than assumed.
4. **Closures ever survive staging.** Uniqueness plus escaping closures gives Landin's knot, which
   is why [closures-direction.md](../closures-direction.md) refuses them; this ADR now depends on
   that refusal as well.
