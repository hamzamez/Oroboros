# Types: a direction, not a decision

**Not an ADR and not a specification.** Nothing here is settled, and the language has no types
today — they live in `emit/`, which is a
[measured result](../gauntlet/results/js-2026-08-14.md) rather than an accident: `targets/js.oro`
declares **zero** types, because JavaScript needs none.

This exists because the question was raised deliberately early, while the interface can still
change, and because one measurement taken while answering it is worth keeping regardless of what
we decide.

---

## 1. The measurement that frames everything

The performance argument for a strong type system is that a proof lets you delete a runtime check.
Bounds checking is the cleanest case, so it was measured. Go, 4096-element dot product, 2s × 4:

| | ns/op | per-iteration check |
|---|---|---|
| indexed loop — **what `cmd/gen` emits today** | 1,005 | `IsInBounds` on the second array |
| same loop, reshaped so Go's own BCE fires | **525** | none; one `IsSliceInBounds` before the loop |

> **Corrected 2026-08-15 by [bce-2026-08-15](../gauntlet/results/bce-2026-08-15.md).** The
> transformation is now built as an emitter pattern and removes the per-iteration check from every
> generated program. But the 1.96× it is worth in isolation **does not appear on the gauntlet's
> large inputs**, where the loop is memory-bound and the removed instruction hides behind the cache
> miss. The number below is real *for compute-bound loops* and was quoted without that condition.

**1.94×.** Larger than every host-idiom difference this project has measured — larger than JS's
`Map` vs null-prototype object (3.25× but on a slower path), larger than Java's fused vs unfused
dictionary (2.6×), and it applies to the innermost loop of every array program in the gauntlet.
`go build -gcflags="-d=ssa/check_bce/debug=1"` confirms `generated_dot.go` and
`generated_centroid.go` both carry the check today.

So the premise is **right**: proving `i < len(a)` is worth roughly a factor of two.

## 2. Three corrections to "a proof removes a check"

### 2.1 Our proofs do not transfer. The host must re-prove them.

We emit host source. Go's bounds-check elimination runs on Go's own analysis and has never heard
of us. A theorem in our type system removes exactly zero instructions unless the code we emit is
**shaped so that the host proves it again**:

```go
b = b[:len(a)]          // one IsSliceInBounds, outside the loop
for i := range a {      // Go now knows i is in range for both
```

> **A type system that does not change what we emit buys nothing.**

That is the parasite thesis arriving at the type system, and it is a much harder constraint than
it sounds. Each host re-proves differently: Go has BCE, the JVM has its own, JavaScript engines
have hidden classes and range analysis that we cannot address at all. A proof we can state and no
host can re-derive is decoration.

### 2.2 The easy 1.94× needs no type system

The residual for `dot` is

```lisp
(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))
```

Everything needed to emit the reshaped loop is **syntactically visible**: the loop bound is
`(alen p)`, and `q` is indexed by the same `i`. An emitter pattern — *narrow every array indexed
by the loop variable to the length that bounds the loop* — collects the whole factor of two with
no types anywhere.

So the type system must not be justified by the cases the emitter can already see. Its value is
the cases where the relationship is **not** syntactically evident: an index computed by
arithmetic, a length carried across a function boundary, a bound established by a caller. Those
are real, and they are a much smaller and more honest claim than "types make it fast."

### 2.3 "The most powerful type system" is the minimality trap wearing a hat

[CLAUDE.md](../CLAUDE.md) already warns against minimising *constructs needed to express all
computation*, because that is not the property this project needs. Maximising *propositions
expressible about a program* is the same error with the sign flipped. The criterion should mirror
the one already adopted for the core:

> **As powerful as necessary subject to (i) every proof being re-provable by some host, and
> (ii) checking terminating.**

Sequent-calculus type systems in the Shen sense — user-supplied inference rules discharged by
proof search — fail (ii) by construction. Shen answers with a depth limit and backtracking, which
means type checking may fail for reasons that are not about the program. This project's core has
one hard-won property, that **reduction terminates at compile time and yields a residual**; adding
a second compile-time process with no termination guarantee fights it directly, and
[ADR 0009](decisions/0009-staging-preserves-results.md) is about exactly that class of hazard.

And it is worth saying plainly: that type system belongs to the predecessor that stalled. The wall
was the boxing in the portability layer, not the types — but adopting the stalled project's most
distinctive component into its successor deserves an argument, not an inheritance.

## 3. What looks right instead

**Refinement types over the range-typed integers already decided in
[ADR 0003](decisions/0003-range-typed-integers.md).**

ADR 0003 committed to integers carrying ranges with mathematical semantics and machine
representation. A range *is* a refinement — `{ i : int | 0 ≤ i < n }` — so half of this is already
a decision, and the type system would be its generalisation rather than a new idea.

Constrain the refinement language to a **decidable fragment**: linear arithmetic over integers,
which is Presburger, which is what Liquid Types uses and what every array-bounds obligation
actually needs. Checking is then a small solver, terminating, with no user-visible search
behaviour.

That buys, in order of how well it is evidenced:

1. **Array bounds** — measured above, and re-provable by Go and the JVM.
2. **ADR 0003's ranges** — overflow and representation selection, which is a *correctness*
   obligation we have already taken on and are not currently discharging.
3. **Signatures that mean something.** This is the argument I did not expect to be the strongest.

### 3.1 The first job is modules, not correctness or speed

[modules.md §2](spec/modules.md) says a signature "names a set of exports and specifies each one's
behaviour". Today a signature is a **list of names** — `(export split-words)` — and nothing more.
The conformance suite exists precisely because the signature cannot state anything a checker could
verify.

A type system is what turns a signature from a list into a claim. And that gives the first
increment a bounded, checkable requirement that has nothing to do with dependent types:

> A signature should be able to say what a name's arguments and result are, so that a target's
> native implementation and a library's fallback can be **checked against the same statement**.

That is [modules.md T2](spec/modules.md)'s substitution soundness becoming machine-checked instead
of asserted, and it is the one job no backend can do for us — because the two implementations
being compared live on *different targets* and no single host compiler ever sees both.

## 3.2 Why not Hindley-Milner and algebraic data types

Skipping past them was a gap in §3, because **refinement types are not an alternative to HM —
they are a layer on top of it.** Liquid Types is HM *plus* refinements. So §3 named the upper
storey and omitted the ground floor.

The reason for skipping is still real: this architecture already has HM's two headline features
by other means.

**Parametric polymorphism, by specialisation.** `examples/generic.oro` uses one definition,
`reduce-over`, at f64→f64 and at string→dictionary. The residual is two monomorphic loops — no
type parameters, no dictionary, no monomorphisation pass, and
[measured](../gauntlet/results/generics-2026-08-14.md) as byte-identical machine code to
hand-written monomorphic Go. HM gives polymorphism and then has to *erase it again*, by
dictionaries (which allocate) or by a monomorphisation pass (which we would be re-implementing).
δ+β do it as a side effect, and the result is guaranteed monomorphic.

**Algebraic data types, by encoding.** `(def vec (fn (n f) (fn (sel) (sel n f))))` is a product
type and `vlen`/`vindex` are its projections — a Scott encoding that reduces away entirely. q5b's
push representation is the same trick for another shape. We already have data types; what we lack
is *checking* them, not representing them.

A native sum type would be actively dangerous here: a tagged union allocates on the JVM and on JS,
which is the boxing CLAUDE.md forbids in the core. Encoded-and-reduced is strictly better for this
project than ADTs are for ML.

## 3.3 The residual is monomorphic, which makes the checker smaller than HM

Source is polymorphic. Reduction specialises it away. The residual is **monomorphic, first-order**
(escaping closures are refused by all three backends) **and closed**. Checking *that* needs no type
schemes, no generalisation, no unification beyond the trivial.

Which suggests two checkers with different jobs rather than one:

| | checks | for whom |
|---|---|---|
| **signature level** | module exports, before reduction | the programmer; what [modules.md](spec/modules.md) needs |
| **residual level** | the specialised normal form, before emission | a soundness net |

The second is cheap precisely because reduction has already done the hard part.

## 3.4 The bigger performance case is representation, not check deletion

§1 framed the argument as deleting bounds checks. That undersold it.

Enumerate what our three hosts actually check at runtime: **bounds** (yes — §1, 1.94×); **null**
(we have no nulls); **division by zero** (we have no division); **overflow** (both wrap, no check);
**casts** (we already emit monomorphic code). On these targets, check *deletion* really is mostly
bounds.

But types drive a second mechanism that is not check deletion at all: **choosing the
representation** — unboxed versus boxed, `i32` versus `i64`, struct-of-arrays versus
array-of-structs. That is [ADR 0003](decisions/0003-range-typed-integers.md)'s entire subject and
[g2](derivations/g2-structs.md)'s finding, it is plausibly larger than bounds checking, and **no
emitter pattern can reach it** — it needs the range and kind of values, not their syntactic
position.

So the performance case for types is stronger than §2.2 allowed, for a reason §1 did not name.

## 3.5 Two layers, because codegen needs types whose meaning is fixed

Sequent calculus can certainly **express** refinements; it is a presentation of a logic, not a type
system. Two objections survive that, and the second is decisive.

**Expressible is not decidable.** Refinement checking gets its power from a *decision procedure*
over arithmetic. `0 ≤ i < n ∧ n = len(a) ⊢ i < len(a)` is discharged by Presburger in
microseconds; a search re-deriving arithmetic from user-written rules is where it becomes
exponential. Shen bounds the search with a depth limit, which makes *"your program is ill-typed"*
and *"the search gave up"* indistinguishable to the programmer.

**A compiler cannot optimise against types it does not understand.** If the type system is
user-defined, the compiler knows only that *a proof exists in somebody's rule set* — not what
`{0..n-1}` **means**. It can delete no bounds check from that and choose no representation. Shen's
type system is built for correctness and expressiveness, and it is structurally unable to drive
codegen.

That is a real tension between the two things wanted — maximum power, and performance — and the
resolution is to stop asking one mechanism for both:

> **Layer 1 — types the compiler exploits.** Small, fixed, decidable. Base types plus refinements
> over ADR 0003's ranges, in linear integer arithmetic. Drives representation selection and bounds
> elimination. The compiler knows what every one of these means.
>
> **Layer 2 — propositions the programmer proves.** Extensible, sequent-calculus-shaped if that is
> the best notation, discharged by search. **Not load-bearing for codegen**, so undecidability
> costs the programmer's patience and never a miscompilation or a lost optimisation.

Layer 2 has no ceiling, which is what the ambition wants, and it is safe *because* the emitter
never consults it. The precedent is broad: Rust's types drive codegen while trait obligations do
not; Liquid Haskell and F\* erase refinements before emission; ATS separates the proof language
from the value language for exactly this reason.

Consequence for ordering: **layer 1 must be designed first**, and designing it well is mostly
keeping it small enough that every construct has a meaning the emitter can act on.

## 3.6 The boundary is the whole job — declare there, infer everywhere else

The framing that makes the rest fall into place: **the language's boundary is the core plus the
exposed surface of the target API**, and the checker's job is to prove that what crosses that
boundary is safe to hand over.

That unifies two mechanisms we already have or want, which turn out to be the same shape:

| selection | by | mechanism |
|---|---|---|
| native implementation vs portable fallback | **availability** | `P_T ∩ D` ([modules.md §5](spec/modules.md)) |
| which native, among several | **provability** | types |

Choosing `strings.Fields` over a hand loop because the argument is known ASCII, or an unchecked
accessor because the index is known in range, is the same act as choosing BLAS over a loop because
the target has BLAS. **Types select which host facility to parasitize.** That is the parasite
thesis reaching the type system, and it is a better argument for types than either speed or
correctness alone.

### The push-back: the performance half needs no language surface

Bounds and ranges can be recovered by **abstract interpretation over the residual** — interval
analysis, which is what every optimising compiler already does — with *zero* new tokens in the
language. The residual is monomorphic, first-order and closed (§3.3), which is the easiest
possible input for it.

So the honest division is not "types for speed, proofs for correctness". It is:

> **Declare at boundaries. Infer in the interior.**

Which is [g5 §1](derivations/g5-bindings.md) again, exactly: *representation is free in the
interior and fixed at the boundary*. That was derived for data layout and it turns out to be the
shape of the type system too — the second application of the same principle, which is the sort of
coincidence worth trusting.

The consequence is a much smaller language change than "add a type system": annotations exist at
module signatures and at target API declarations, and **nowhere else**. The interior stays
untyped, and stays inferred.

### What is better than sequent calculus, in one line

> **Let the programmer extend the *predicates*, not the *rules*.**

Shen's power comes from user-written inference rules discharged by search: undecidable, and opaque
to the compiler (§3.5). Refinement systems get comparable reach by fixing the rules and letting the
*predicate language* grow over a decidable theory. The compiler keeps a meaning for every type, the
checker keeps terminating, and the programmer keeps the expressiveness — because in practice what
people want to say is `0 ≤ i < len(a)` and `s is ascii`, which are predicates, not rules.

### The literature that actually applies

Not proof search. **Multi-stage programming**: MetaML and MetaOCaml (Taha & Sheard), and the
Davies–Pfenning modal account already cited in this project's staging work. Our compiler *is* a
staged evaluator, and the theorem we want is precisely theirs:

> A well-typed source program generates a well-typed residual.

That is what makes checking the source worth anything, given that the thing which actually runs is
the residual. It is also the type-level twin of
[ADR 0009](decisions/0009-staging-preserves-results.md), which says staging must not change an
*answer*; this says staging must not break a *proof*.

### Two things this would buy that §1–§3 did not name

**Termination as a theorem instead of a fuel limit.** [concerns.md §2.1](spec/concerns.md) records
that our termination guard — `markRecursive` plus a step limit — is "a *mechanism*, not a proof,
and the fuel limit is an admission that the mechanism is incomplete". Sized or structural types
would discharge it. That is the one place the core currently cheats, and no other proposal on the
table removes it.

**Checking on the target that has no checker.** Go and Java reject an ill-typed residual for us.
JavaScript accepts anything. So today the effective type safety of an emitted program depends on
which target it was emitted for, which is the opposite of what this project claims.

### Good news for the boundary, unlike purity

Every one of ten thousand generated target names will need a type. That sounds like the purity
problem from [effects.md §3](spec/effects.md), and it is the opposite: **no host publishes purity,
and every host publishes types.** Go has `go/types`, Java has reflection and its class files,
TypeScript ships `.d.ts` for the entire DOM. The layer that most needs machine generation is the
one the hosts already machine-generate.

### A picture of it

[types-sketch.md](types-sketch.md) works the whole thing through in concrete syntax — the `dot`
example end to end, how a programmer extends the predicates, and a comparison table. A sketch, not
a specification.

> **Reopened 2026-08-19 — see §6.** Items 2 and 3 below are no longer speculative: the annotation
> they describe **already parses**, and reading it doubles what the compiler can prove
> ([intervals-2026-08-19](../gauntlet/results/intervals-2026-08-19.md)). §6 also does what §3's
> one-line dismissal of sequent calculus did not — takes it seriously, and finds that its main
> practical payoff is something this language already has.

## 4. Order, when the time comes

Nothing here is scheduled. If it were:

1. ~~**Collect the 1.94× with an emitter pattern first**~~ — **done**, 2026-08-15. It required no
   design and no types, exactly as §2.2 predicted, and measuring it on the real gauntlet is what
   revealed the win is conditional on the loop being compute-bound.
2. **Signature types** — argument and result types on module exports, checked across targets.
   Small, and it makes something we already built stronger.
3. **Refinements on integers**, decidable fragment, discharging ADR 0003.
4. Anything beyond that only when a gauntlet program demands it, per
   [ADR 0007](decisions/0007-exploration-over-specification.md).

## 5. What would change this document

- A measurement showing the emitter pattern of §2.2 does **not** collect most of §1's factor —
  which would mean the interesting bounds cases are the non-syntactic ones, and move refinements
  up the list.
- A gauntlet program whose correctness cannot be stated without dependent types.
- Evidence that JavaScript engines can be addressed by emitted shape at all. Right now the entire
  performance argument is Go and JVM only, and JS is the target that most needs it.

---

# 6. Reopened, 2026-08-19: what the literature actually offers

hamza reopened this naming three routes — *a better type system, sequent calculus, or a type system
like Coq* — and asked that we reach for the literature rather than rediscover it. Three things have
changed since §1–§5 were written, and they change what the answer should be.

## 6.1 The justification moved, and this is the whole point

§1–§2 killed the performance argument for types, and killed it correctly: **our proofs do not
transfer**, the host re-proves them or does not, and the 1.94× was collected by an emitter pattern
with no types at all.

That argument does not apply to what is now on the table. If `int` becomes exact by default
([data-model.md §1.5](spec/data-model.md)), the compiler must choose between a machine word and a
bignum at every operation, and **only a range can decide it**. The host cannot make that choice for
us — it never sees the exact-by-default semantics, only whatever we emitted. So:

> **A range does not need to transfer. It changes what we emit.**

That is categorically different from bounds checking, and it is the same argument that already
justified the residual type checker: *the one job no host compiler can do*, because the two
implementations live on different targets and no single compiler sees both
([types.md](spec/types.md)). §3.4 anticipated this — *the bigger performance case is representation,
not check deletion* — and it is now the case in front of us rather than a guess.

## 6.2 The measurement arrived, and it says the boundary really is the whole job

§3.6 guessed: **declare at the boundary, infer everywhere else.**
[intervals-2026-08-19](../gauntlet/results/intervals-2026-08-19.md) measured it. **39% of integer
operations provably stay in a machine word with nothing declared; 81% with one range declared on a
program's parameters.**

And the annotation needs **no new syntax**. This parses today, and `Refine` has always assumed it
for array bounds:

```lisp
(sig count-primes ((n int)) int (where (go.&& (go.<= 0 n) (go.< n 1048576))))
```

Adding that one line to `examples/native/sieve-go.oro` and changing nothing else takes it from
**45% to 90%**. The language already lets a programmer state a range; until this week nothing read
it for anything but array bounds.

That is a much smaller step than "a type system", and it is the step the measurement supports.

## 6.3 Sequent calculus: what it buys, precisely

§3 dismissed sequent calculus in one line, and that line was really about Shen's proof search. The
question deserves better, because the answer is interesting and mostly good news.

**First, a distinction that matters.** Sequent calculus is a *proof-theoretic presentation*, not a
type system. Gentzen's LK and LJ (1935) prove exactly what natural deduction proves; the Hauptsatz
is about **normalising proofs**, not about proving more. So sequent calculus **does not** solve the
interval residue, the range problem, or `d ≠ 0`. It offers three other things, and two of them we
already have.

**λμμ̃ — evaluation order becomes structural.** Curien & Herbelin, *The duality of computation*
(ICFP 2000), building on Herbelin's λ̄ (1994): terms meet **co-terms** (continuations) in a command
`⟨t ∥ e⟩`, and call-by-value and call-by-name are literally dual — the same command reduced from
opposite sides.

For us that is a *reframing* of something already built. [effects.md](spec/effects.md)'s discipline
— an impure argument is never substituted, denying contraction, weakening and exchange in that
order — is a **substructural** statement, and substructural logic is native to the sequent calculus.
So there is a real unification available: our effect side condition on β would become a structural
rule rather than a condition. **Worth knowing if we ever need to prove the discipline sound. Not
worth a core rewrite to restate what already works.**

**Join points — and GHC already ran this experiment for us.** Downen, Maurer, Ariola & Peyton Jones,
*Sequent Calculus as a Compiler Intermediate Language* (ICFP 2016), built **Sequent Core** as a
drop-in alternative to GHC Core. The headline benefit is that a continuation becomes a first-class
name, so **case-of-case names the continuation instead of duplicating it** — exactly the blow-up
[booleans.md §2.7](spec/booleans.md) had to route around.

And then GHC did not adopt it. Maurer, Downen, Ariola & Peyton Jones, *Compiling without
Continuations* (PLDI 2017), added **join points to direct-style Core** instead, on the grounds that
it was a far smaller change for most of the benefit. That is the strongest available precedent for
our situation, and the conclusion is sharper still:

> **We already have join points.** `(again a₁ … aₙ)` is a jump to a labelled continuation with
> arguments — [ADR 0015](decisions/0015-loop-and-again.md) reached it from SSA block arguments
> (MLIR, SIL, Cranelift), which is the same construct under a different name. The main practical
> payoff of a sequent-calculus core is a thing this language got by accident two weeks ago.

**Polarity — and this one we do NOT have, and should take.** Andreoli's focusing (1992),
Zeilberger's *On the unity of duality* (2008), Munch-Maccagnoni, and Levy's **call-by-push-value**
(1999) all classify types the same way:

| | eliminated by | must the value exist? |
|---|---|---|
| **positive** — sums `⊕`, tensor `⊗`, inductive data | **pattern matching** | yes |
| **negative** — functions `→`, `&`, records, coinductive | **projection** | **no** |

Apply that to the product question and it stops being a guess:

- `divmod` returning a quotient and a remainder that the caller **projects** is a **negative**
  product. It need never be built — and
  [product-2026-08-19](../gauntlet/results/product-2026-08-19.md) measured exactly that: 1.01× on
  Go with **zero allocations**, and C2 scalar-replacing Java's record.
- `(value, error)` that the caller **matches** on is **positive**. It must exist, and it allocates
  unless the host removes it.

> **The measurement said products are free when they do not escape. Polarity says which ones those
> are, in advance, from the shape of the eliminator.** That is a rule we can state and check rather
> than a benchmark we have to re-run per case — and it says which product to add first: the
> negative one, which covers `idiv`, `fold-range2` and multiple returns, and is free on all four
> hosts.

That is the sequent calculus's real gift here: **not a core, a classification.**

## 6.4 Coq-like dependent types, and the system that matters most

**The extraction problem is the direct analogue of §2.1.** Coq's CIC lets types mention terms;
checking is decidable, inference is not. Extraction (Letouzey, 2002) erases proofs faithfully — and
the OCaml or Haskell that comes out is *correct and not fast*, because the functional data
structures survive. A theorem removed the obligation; it did not change the representation. That is
"our proofs do not transfer" in another language's mouth.

**And then there is the counter-example, which is the most relevant system in this literature.**

**Low\*** — Protzenko et al., *Verified Low-Level Programming Embedded in F\** (ICFP 2017) — is a
**subset** of F\* with a C-like memory model, extracted to C by KaRaMeL. It is not a toy:
**HACL\*** (Zinzindohoué et al., CCS 2017) is verified cryptography written in it and shipping in
Firefox, the Linux kernel and WireGuard, **at parity with hand-written C**.

The lesson is not "dependent types are fast". It is the opposite, and it is this project's own
thesis in someone else's hands:

> **Full dependence plus extraction gives correct-and-slow. A restricted subset with a predictable
> memory model, and every proof erased, gives correct-and-fast.** The restriction is not a
> concession — it is the mechanism.

Two more in the same family are worth naming because they are closer to us than Coq is:

- **ATS** (Xi) — DML-style dependent types *and* linear types, compiling to C with no runtime
  overhead. The closest existing language to what this project appears to want.
- **Cogent** (Amani et al., ASPLOS 2016) — linear types, compiles to C, emits Isabelle proofs;
  used for verified file systems. And **Ivory** (Galois), an EDSL for embedded C.

## 6.5 Our actual lineage, which the measurement confirmed

**Dependent ML.** Xi & Pfenning, *Eliminating Array Bound Checking Through Dependent Types* (PLDI
1998) and *Dependent Types in Practical Programming* (POPL 1999). Types indexed by terms from a
**constraint domain** — linear integer arithmetic — with three properties we need and one we have:

1. checking is **decidable**, because the index language is fixed;
2. indices are **erased at runtime**, so the proof costs nothing;
3. annotation is required **only at function boundaries**;
4. and `emit/refine.go` is a baby version of it, built here without the pedigree.

Point 3 is the one worth pausing on. **Our measurement independently rediscovered DML's design
point**: declare the parameters, infer the rest, and the number roughly doubles. That is not a
coincidence — it is the same fact about programs, found twice.

**Liquid types are the lever on the annotation burden.** Rondon, Kawaguchi & Jhala (PLDI 2008)
*infer* refinements by predicate abstraction over a fixed set of qualifiers, so most signatures need
nothing written. LiquidHaskell (Vazou et al., 2014) and F\*'s SMT-backed refinements are the same
idea at scale. If "the burden falls on the programmer" turns out to be too much burden, **this is
the published answer**, and it does not change the language — only the inference.

## 6.6 The residue is termination, and the literature has that too

[intervals-2026-08-19](../gauntlet/results/intervals-2026-08-19.md) found the whole unproven residue
is one class: **a loop variable bounded by the trip count rather than by a guard on itself.** An
accumulator counting primes; a digit index whose guard is on the quotient.

A trip count is a termination argument. And [concerns.md §2.1](spec/concerns.md) already records
that our termination guard is "a *mechanism*, not a proof". **These are the same hole seen from two
sides**, and closing it closes both:

- **Well-founded recursion with an explicit measure** — Coq's `Program Fixpoint`, Isabelle's
  `function` package. One optional annotation on `loop`, and it yields the trip count directly.
- **Sized types** — Hughes, Pareto & Sabry (1996); Abel. Types carry a size index that must
  decrease.
- **Size-change termination** — Lee, Jones & Ben-Amram (POPL 2001). **Automatic**, no annotation,
  and it handles exactly the shapes our loops have.

The last is the cheapest and should be tried first: it needs no syntax at all.

## 6.7 And `d ≠ 0` needs no new theory

[assessment §3.2](assessment-2026-08-19.md) found that division's precondition falls outside our
conjunctive linear fragment, because `d ≠ 0` is a disjunction. That framing made it look like a
research problem. It is not:

- **Case split.** `d < 0 ∨ d > 0` is two conjunctive queries against the fragment we already have.
  Cheap, complete for this shape, and needs nothing new.
- Or adopt full **Presburger arithmetic** (Presburger 1929), decidable with disjunction and
  quantifiers — the Omega test (Pugh, 1991) or any QF_LIA solver. High worst-case complexity, fast
  in practice, and a dependency we do not need yet.

## 6.8 Candidates

**T-A — analysis only, no types in the language.** What we have. 39% without annotation, which is
not enough to select a representation. *Rejected as a destination, kept as the floor.*

**T-B — DML-style ranges in `sig`, decidable and erased.** Read the `where` the language already
parses; use it for representation selection as well as bounds. **45% → 90% on the sieve, measured,
with no language change at all.** *Recommended, and it is available now.*

**T-C — T-B plus liquid inference.** Infer refinements over a qualifier set so most signatures need
nothing. *The answer if T-B's annotation burden proves too heavy — and it is a compiler change, not
a language one.*

**T-D — a termination measure on `loop`, or size-change termination.** Closes the interval residue
and the fuel-limit cheat with one mechanism. *Recommended second; try the automatic version first.*

**T-E — polarity, taken as a classification.** Add the **negative** product first, because
projection-eliminated products need never be built and that is why they measured free. *Recommended
third, and it costs no theory — only the discipline of asking which eliminator a product has.*

**T-F — a sequent-calculus core.** *Rejected*, on GHC's own precedent: they built Sequent Core,
measured it, and shipped join points in direct-style Core instead. And we already have join points.

**T-G — full dependent types, Coq-style.** *Rejected as the default.* Low\* is the existence proof
that verified code can reach parity with hand-written C, and it reaches it by being a **restricted
subset with everything erased** — which is T-B plus T-D, not CIC.

## 6.9 What this section changes

The recommendation is no longer "types, someday". It is:

1. **Read the range that is already writable** (T-B). One pass, no language change, measured at
   45% → 90%.
2. **Try size-change termination** (T-D, automatic form), which closes the residue *and* the one
   place [concerns.md](spec/concerns.md) says the core cheats.
3. **Classify products by polarity before adding one** (T-E) — negative first.
4. Hold liquid inference (T-C) in reserve for the annotation burden, which is measurable rather
   than arguable.

And the honest summary of the three routes hamza named: **the better type system is DML, and we are
already most of the way to it. Sequent calculus gives a classification we should take and a core we
should not. Coq-like dependence gives an existence proof — Low\* — whose lesson is that the
restriction is the mechanism.**
