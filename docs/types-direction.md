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

## 4. Order, when the time comes

Nothing here is scheduled. If it were:

1. **Collect the 1.94× with an emitter pattern first**, and measure it on the real gauntlet. It is
   the cheapest large win available and it requires no design.
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
