# What F\* and Low\* teach, and where they say we are wrong

Low\* has been cited five times in this repository as an authority and never examined. hamza's
question, at a point he correctly calls a crossroad: what are the lessons?

Nine of them. **Five confirm decisions already made, three challenge them, and one names something
we have been getting for free and had not noticed.**

---

## 0. What they are

**F\*** (Swamy et al., POPL 2016) is a dependently-typed ML with refinement types, an SMT backend,
and a hierarchy of effects. **Low\*** (Protzenko et al., ICFP 2017) is a *subset* of F\* — a
shallow embedding of C — extracted to real C by **KaRaMeL**. **HACL\*** (Zinzindohoué et al.,
CCS 2017) is a verified cryptographic library written in it.

The reason to take them seriously is not the papers. It is that HACL\* code **ships**: Firefox's
NSS, WireGuard's Curve25519 in the Linux kernel, mbedTLS, Microsoft's msquic, Tezos. Verified code
at hand-written-C performance, in production, for years. It is the largest existence proof that the
thing this project is attempting — a restricted language that reaches hand-written performance and
carries a machine-checked claim — is possible at industrial scale.

---

## 1. Confirmed: the restriction IS the mechanism, and it must refuse

Low\* is not "F\* with a C backend". It is **the subset of F\* that has a C meaning**, and a program
outside the subset **does not extract** — KaRaMeL refuses rather than compiling badly. No closures,
no GC, no polymorphism after monomorphisation, memory explicit.

That is our architecture exactly. `emit/*.go`'s *"a bare abstraction reached the emitter"* is
KaRaMeL's *"this construct has no C equivalent"*, and
[ADR 0014](decisions/0014-recursion-is-not-in-the-language.md)'s refusal of recursion is the same
move — `oro` accepted programs `build` refused, and the fix was to refuse earlier.

**What to take:** the refusal is a feature and should read like one.
[closures-direction.md §8](closures-direction.md) proposes saying *"this function would have to
exist at run time"* rather than *"a bare abstraction reached the emitter"*, which is exactly the
register KaRaMeL uses.

## 2. Confirmed: erasure must be total, and checkable

F\* erases proofs, ghost state, refinement predicates and dependent indices. The extracted C
carries **zero** verification overhead. `erased` and the `Ghost` effect make "does this vanish?" a
typing question rather than a hope.

Ours: types, refinements, the delayed table, the Church encoding, and every closure must vanish by
the end of reduction. **The lesson is the second half — make it checkable.** F\* can *tell* you
something is ghost. We find out when the emitter fails.

That is what [closures-direction.md §2](closures-direction.md) names as binding-time analysis, and
it is the cheapest thing on this list to build.

## 3. Confirmed, emphatically: keep the decision procedure decidable

F\*'s best-known practical problem is **proof instability** — an SMT query that succeeds today and
times out tomorrow after an unrelated edit. The Everest team has written about it at length; it is
the tax of "encode to Z3 and hope", and it is paid on every build by every developer.

`emit/refine.go` uses linear integer arithmetic with a **deliberately incomplete** decision
procedure that *reports* an undischarged obligation rather than assuming it
([refinements.md](spec/refinements.md)). That is the opposite trade and F\* is the evidence for
why it is right: a checker that always terminates and sometimes says "I could not prove this" is
worth more than one that usually proves more and occasionally hangs.

**Do not add an SMT backend.** If the fragment is too weak, widen the fragment.

## 4. Confirmed: the poorest host defines the subset

Low\*'s subset is exactly what C can express, and every design decision traces to that. For us the
role is played by `targets/windows/` — no GC, no closures, no `map`. CLAUDE.md already says *never
make the core a superset of one host*; Low\* is the industrial-scale existence proof that
constraining to the poorest host is survivable and produces something fast.

## 5. Confirmed: two levels, and ours is cheaper than theirs

HACL\* is written three times: a **pure executable specification** (a mathematical ChaCha20), a
**Low\* implementation** (the bit-twiddling one), and a **proof they agree**. The spec is erased.

That is the same two-level structure [closures-direction.md](closures-direction.md) just named —
and **ours is cheaper in one specific way**:

> F\* needs a *proof* that the implementation refines the specification. We need none, because
> reduction **produces** the implementation *from* the specification. They are the same program at
> two stages, not two programs with a theorem between them.

`(alloc (table n f))` is the specification and the emitted loop is the implementation, and η-tab is
the refinement theorem — free, by construction.

## 6. Free and unnoticed: linearity is framing done by the type system

This is the lesson we have been collecting without knowing it.

Low\*'s memory model is **HyperStack** — a stack of regions plus a heap — and reasoning about
mutation requires **framing**: a `modifies` clause on every function saying which regions it may
change, plus liveness and disjointness preconditions. It is the single largest source of proof
burden in HACL\*, and whole papers exist about making it tolerable.

**[ADR 0018](decisions/0018-immutable-values-linear-buffers.md) gets framing for free.** A buffer is
linear and lexically scoped, so its `modifies` set is *syntactically* the buffer and nothing else.
There is nothing to state and nothing to prove:

| | Low\* | us |
|---|---|---|
| what may be mutated | a region, declared in a `modifies` clause | the buffer in scope, and nothing else |
| how it is established | a proof obligation per function | the shape of `build` |
| aliasing | disjointness preconditions | not expressible |

That is a genuine advantage of linearity over regions-plus-framing, and it is worth writing down
because it is the strongest argument for ADR 0018 that ADR 0018 does not make.

---

## 7. Challenged: they kept recursion, and they kept real data types

Low\* has recursion, structs, unions, tagged unions and arrays — because C has them. It reaches
hand-written C performance *with* them.

**So "no recursion" and "no sums of products" are our choices, not consequences of wanting speed.**
The justifications are still good and they are about having four hosts rather than one:

- recursion — stack depth differs by orders of magnitude across Go, the JVM and JS, and none
  guarantees TCO ([ADR 0014](decisions/0014-recursion-is-not-in-the-language.md));
- data types — three of four hosts have a garbage collector we do not control, so a tagged union is
  an allocation we cannot price uniformly.

But Low\* is evidence that **our data-structure minimalism may be stricter than the performance
requirement demands**, and that if it is ever relaxed the reason will be that four hosts is what
forbids it, not that speed does.

## 8. Challenged, and this is the sharpest one: we cannot write a fast implementation and prove it

This is the real cost of §5's cheapness, and it took reading Low\* to see it.

HACL\* writes a clean specification, writes a *completely different* bit-twiddling implementation,
and **proves they compute the same function**. The implementation can use any trick — unrolled
rounds, SIMD, precomputed tables, a different algorithm — and the proof carries the claim.

**We cannot do that.** Our implementation is whatever reduction produces from the specification. If
`alloc` of the obvious `table` is not fast enough, there is no way to write a clever version and
show it equivalent. The only escape is to drop to the target and lose the claim entirely.

The seed of an answer is already here and is not being used: `(sig name … (where …))` is
[described](spec/types.md) as *a claim checked in two directions* — against the definition's
residual, **and against any target that provides the name natively**. That is exactly a refinement
proof between two implementations, and `blas` providing `num/vec.dot` as `cblas_ddot` is the one
instance of it in the repository.

**Generalising that is the honest path to §8's gap**: two definitions of one name, one obviously
correct and one fast, with the signature as the contract between them. It is a real feature, it is
not a type system, and Low\* is the argument for wanting it.

## 9. Challenged: know what layer you are

Protzenko et al. are explicit that Low\* targets **the systems layer** — cryptography, parsers,
protocol state machines — and not general application programming. That is why HACL\* is a crypto
library and not a web framework, and the restriction is why it succeeded.

Our gauntlet is dot products, stencils, sieves, word counts.

> **Answered 2026-08-21, and it is not what this section guessed** —
> [general-purpose.md](general-purpose.md). hamza: *"this is general purpose programming. I want
> apps on windows and android, I want website in the browser and I want backend in the cloud."*
>
> The paragraph that stood here read the *benchmarks* and concluded "a systems and numeric
> language". That was a read of what has been measured, not of what is intended, and the four
> targets were application platforms all along.
>
> **The Low\* lesson does not disappear, it inverts.** Low\* succeeded by picking a layer where its
> restrictions are advantages. Choosing general purpose means **the restrictions must be paid for
> rather than enjoyed** — and general-purpose.md is the bill: recursion, sums, strings, growable
> collections and maps all move from *deferred* to *owed*.

[assessment-2026-08-20 §4](assessment-2026-08-20.md) already names the risk in its own words:
*"nothing here is a program anyone would want to run."* Low\* answers that by picking a layer where
the restriction is an advantage and being the best thing there.

---

## 10. What this says about the crossroad

Ranked, and none of it is a language change today.

**Do:**

1. **Say the two levels out loud** — in the documentation, in the diagnostics, and as a checkable
   binding-time property rather than an emitter failure (§1, §2).
2. **Keep the decision procedure decidable and incomplete.** No SMT (§3).
3. **Write down that linearity is framing** — it is ADR 0018's best argument and ADR 0018 does not
   make it (§6).
4. **Generalise the two-directional `sig`** into a way to give one name two definitions, a
   specification and a fast implementation, with the signature as the contract (§8). This is the
   one genuinely new capability on the list.

**Do not:**

5. **Do not add closures, general recursion, or sums of products because Low\* has some of them.**
   Low\* has one target with explicit memory; we have four, three with collectors we do not own.
   The constraint that forbids them is *four hosts*, and it has not changed.

**And decide, explicitly:**

6. ~~**What layer this is.**~~ **Answered: general purpose** — [general-purpose.md](general-purpose.md).
   Which means the restrictions are now costs to be paid rather than advantages to be enjoyed, and
   the gauntlet should grow toward the awkward application shapes — a parser, an event loop, a
   request handler — which is precisely where
   [assessment-2026-08-20 §5](assessment-2026-08-20.md)'s *"write something awkward"* points.
