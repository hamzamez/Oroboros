# The map of the woods

hamza: *"you want decidable, I would like that. but it should not look like we are patching the
language. reach for the literature, what is the algebra, reason about it mathematically. give us a
map of the possibilities, where is the limit of decidable, the best thing we can get. give us a map
of the woods, the limits, if we cross them what we get and we lose."*

This is that map. It is not a decision document — it is the territory, so that every future decision
can be located on it instead of argued from scratch.

---

## 0. Four questions that get conflated

"Is it decidable?" is four different questions and they have four different answers.

| | question | ours |
|---|---|---|
| **Q1** | does *reduction* terminate? | **undecidable** — the static level is untyped λ |
| **Q2** | does the residual *type check*? | **trivially decidable** — a syntax walk |
| **Q3** | are the *obligations* discharged? | QF-LIA, **decidable, NP-complete** |
| **Q4** | do the *loops* terminate? | size-change, **decidable, PSPACE-complete** |

The single most important structural fact about this language is how those four are separated, and
it is §3.

---

## 1. The territory

Concentric, by cost. Everything here is a published result.

```
╔═══════════════════════════════════════════════════════════════════════╗
║  UNDECIDABLE                                                          ║
║    halting · program equivalence · extensional function equality      ║
║    System F type inference (Wells 1994) · F<: subtyping (Pierce '92)  ║
║    nonlinear INTEGER arithmetic — Hilbert's 10th (Matiyasevich 1970)  ║
║    unrestricted ∀ over arrays · intersection-type inference           ║
║  ┌─────────────────────────────────────────────────────────────────┐  ║
║  │  DECIDABLE, INFEASIBLE                                          │  ║
║  │    Presburger with quantifiers — 2-EXP lower (Fischer&Rabin '74)│  ║
║  │    nonlinear REAL arithmetic — Tarski 1951, 2-EXP (Collins '75) │  ║
║  │  ┌───────────────────────────────────────────────────────────┐  │  ║
║  │  │  DECIDABLE, EXPONENTIAL                                   │  │  ║
║  │  │    QF-LIA — NP-complete          bit-vectors — NEXP-ish   │  │  ║
║  │  │    size-change termination — PSPACE (Lee/Jones/Ben-Amram) │  │  ║
║  │  │    polyhedra domain (Cousot&Halbwachs '78)                │  │  ║
║  │  │    Hindley–Milner inference — DEXPTIME (Mairson 1990)     │  │  ║
║  │  │  ┌─────────────────────────────────────────────────────┐  │  │  ║
║  │  │  │  DECIDABLE, POLYNOMIAL                              │  │  │  ║
║  │  │  │    difference logic (x−y≤c) · UTVPI (±x±y≤c)        │  │  │  ║
║  │  │  │    octagons — O(n³) (Miné 2006)                     │  │  │  ║
║  │  │  │    intervals — O(n) (Cousot&Cousot 1977)            │  │  │  ║
║  │  │  │    EUF — congruence closure, O(n log n)             │  │  │  ║
║  │  │  │    linear REAL arithmetic · STLC checking           │  │  │  ║
║  │  │  │                                                     │  │  │  ║
║  │  │  │    ★ WE ARE HERE, plus an NP goal solved by an      │  │  │  ║
║  │  │  │      incomplete polynomial procedure                │  │  │  ║
║  │  │  └─────────────────────────────────────────────────────┘  │  │  ║
║  │  └───────────────────────────────────────────────────────────┘  │  ║
║  └─────────────────────────────────────────────────────────────────┘  ║
╚═══════════════════════════════════════════════════════════════════════╝
```

Two facts on that map are worth pausing on because they are counterintuitive.

**Nonlinear arithmetic is undecidable over the integers and decidable over the reals.** Hilbert's
tenth problem is about ℤ; Tarski's quantifier elimination for real closed fields is about ℝ. So
`x*x < n` is *harder* over the integers than over the reals, which is the opposite of the intuition
that integers are simpler.

**Adding quantifiers to linear integer arithmetic keeps it decidable and makes it useless.**
Presburger arithmetic is decidable (Presburger 1929) with a doubly-exponential *lower* bound
(Fischer & Rabin 1974). Decidability is not the frontier; **feasibility** is.

---

## 2. The two lattices, and how they meet

The algebra has two halves and conflating them is the usual confusion.

### 2.1 Inference — abstract interpretation

Cousot & Cousot 1977. A Galois connection `α ⊣ γ` between the concrete powerset and an abstract
domain, with the domains ordered by precision:

```
⊤  ⊑  intervals  ⊑  zones (x−y ≤ c)  ⊑  octagons (±x±y ≤ c)  ⊑  polyhedra  ⊑  concrete
      non-relational    O(n³)              O(n³), O(n²) space    exponential
```

Termination on an infinite-height lattice comes from **widening ∇**, and precision is recovered by
**narrowing Δ**. `emit/interval.go` is intervals with both — and the first run reported 10–20%
because the narrowing phase was missing, which is the lattice theory biting in practice.

### 2.2 Proof — a logical theory with a decision procedure

A set of formulas closed under entailment. Ours is quantifier-free linear integer arithmetic.
Nelson–Oppen (1979) says disjoint stably-infinite theories combine, which is how a real SMT solver
is built out of these pieces.

### 2.3 Where they meet, and our one real mismatch

The domain produces **facts**; the theory **discharges obligations** from them. Intervals produce
`0 ≤ i ∧ i ≤ n−1`; QF-LIA discharges `i < len(a)`.

> **Intervals are non-relational and every interesting obligation is relational.** `i < len(a)` is a
> statement about two variables, and no interval can express it.

`emit/refine.go` papers over that with `assumeLE` on symbolic *linear expressions* — a hand-rolled
weak relational layer bolted onto a non-relational domain. **Octagons are the principled version**,
they are O(n³), they are inside the polynomial ring of the map, and they express exactly the
`±x ± y ≤ c` shape that bounds checking and loop invariants live in.

**That is the single highest-value move available on the inference side, and it is not a patch — it
is moving one step up a lattice that has been studied since 1978.**

---

## 3. Why we are where we are, and it is structural rather than lucky

The reason the expensive parts of the map never get paid is **staging**, and this is the answer to
*"it should not look like we are patching the language."*

| level | question | why it is cheap |
|---|---|---|
| **static** | does it reduce? | Turing-complete, so **undecidable** — bounded by **fuel**, and divergence is a *compile error*, which is honest |
| **dynamic** | does it type check? | reduction has made the residual **monomorphic, first-order and closed**, so checking is a syntax walk |
| **refinements** | are obligations discharged? | QF-LIA, NP-complete, solved by an **incomplete but total** procedure |
| **termination** | do loops terminate? | size-change, PSPACE, 96% in practice |

**We never pay Hindley–Milner's DEXPTIME, and we never meet System F's undecidable inference,
because there is nothing polymorphic left to infer.** Reduction monomorphises. That is not a
restriction we imposed on the type system; it is a *consequence* of the language being two-level,
and it is why the type checker is a few hundred lines rather than a research project.

And the undecidable question — Q1 — is **quarantined into the level where an honest answer exists**.
At compile time, "I gave up after N steps" is a diagnostic. At run time it would be a hang. Zig
makes the same trade with its branch quota; Shen makes it with a search limit on type checking,
which is the *worse* place to make it (§5).

---

## 4. The five cliffs

For each: what is on the other side, what crossing buys, what it costs, and whether there is a way
to get some of it without crossing.

### Cliff 1 — variable × variable

**Beyond it:** Hilbert's tenth problem. **Undecidable**, not merely expensive.

**What you want from over there:** `x*x < n ⟹ x < n` (the sieve, already needed); overflow bounds
on products; and — the one that matters most — **`i*width + j`, which is every flattened
two-dimensional array**.

**Three ways to get most of it without crossing:**

1. **Sound axioms, declared.** `x ≤ x*x` for all integers; `0 ≤ a ∧ 0 ≤ b ⟹ 0 ≤ a*b`; monotonicity
   `a ≤ b ∧ 0 ≤ c ⟹ a*c ≤ b*c`. This is what we already did for squares. **Adding a sound
   incomplete rule is not patching — it is choosing axioms over search**, and it is what every
   practical prover does at this boundary.
2. **Linearise on a pinned factor.** `i*width` is *linear* whenever the interval analysis has pinned
   `width` to a constant, which covers most real 2-D indexing. This costs nothing new.
3. **Relax to the reals.** A proof over ℝ implies the goal over ℤ, so Tarski's procedure is a
   **sound, incomplete** method for integer goals. Decidable, doubly exponential. *Not recommended*
   — it buys the same cases (1) and (2) buy, at 2-EXP.

**Recommendation: (1) and (2). Never a search.**

### Cliff 2 — quantifiers

**Beyond it:** Presburger with quantifiers is decidable and 2-EXP; unrestricted ∀ over arrays is
**undecidable**.

**What you want:** "every element is initialised", "this array is sorted", "every key is non-empty"
— which is `_Out_writes_all_` and most of what an API contract wants to say beyond sizes.

**The decidable slice:** the **array property fragment** (Bradley, Manna & Sipma, VMCAI 2006) — ∀
over index variables, guards restricted to comparisons among indices and index terms, and no
comparison between two array reads inside the body. Initialisation is in it. General sortedness is
not, because it compares `a[i]` with `a[j]`.

**Recommendation:** adopt the array property fragment *if and when* an API contract demands it. It
is a real, bounded, published fragment — not an open-ended step into quantified logic.

### Cliff 3 — polymorphic inference

**Beyond rank-2:** undecidable (Wells 1994). Rank-2 inference is decidable (Kfoury & Wells).

**What crossing buys us: nothing.** Staging monomorphises, so there is no polymorphism at the level
that gets checked. This cliff is one we are structurally on the safe side of and can stop thinking
about.

### Cliff 4 — dependent types

**Beyond it:** type checking is decidable *only if* the language is strongly normalising, which is
why Coq and Agda restrict recursion. Our static level is untyped λ and is not.

**What crossing buys:** the ability to say `Vec n` with `n` a term. **We do not need it** —
[tables.md §5.3](spec/tables.md) showed a dynamic index forces homogeneity and reduction erases
every static one, so the checker only ever sees `Fin n → V` with `n` opaque.

**Cost of crossing:** a totality checker for the static level and a conversion algorithm — Agda's
machinery, for a problem staging already solved.

### Cliff 5 — program equivalence

**Beyond it:** undecidable, full stop. η-tab — `(alloc (table (len a) (fn (i) (a i)))) = a` — is a
law we **state and apply as a rewrite**, never one we *decide*. Crossing means being a proof
assistant, and [lowstar-lessons.md §3](lowstar-lessons.md) is the argument against: F\*'s proof
instability is what "encode to Z3 and hope" costs on every build.

This is also [lowstar-lessons.md §8](lowstar-lessons.md)'s gap — HACL\* writes a fast implementation
and *proves* it refines a clean spec, and we cannot. The bounded version of that capability is
`sig`'s existing *claim checked in two directions*, which is a contract rather than a proof search.

---

## 5. Elegance: Shen, sequent calculus, and what to take

**Shen** (Tarver) is small and big at once for three reasons: a tiny kernel (KLambda) that
everything ports through; a type system whose rules are **written by the user** in sequent style;
and self-hosting.

**Take: rules as data.** Shen's `datatype` declarations *are* sequent rules the programmer writes.
We already do this for primitives — `targets/*.oro` is rules-as-data, and `(where …)` is a rule. The
natural extension is that **the nonlinear axioms of Cliff 1 should be declared, not hardcoded in
Go** — `(lemma (<= x (* x x)))` in a file, not a Go function called `squareBound`. That is exactly
the project's own rule that *primitives are declared in `targets/*.oro`, not in Go*, applied one
level up, and it is the difference between a language and a patch.

**Take: a tiny kernel everything reduces through.** We have it — [the atom](the-atom.md).

**Reject: a Turing-complete type checker.** Tarver's position is that decidable type systems are too
weak, so Shen's is undecidable and controlled by a search limit. **That puts the undecidability in
the wrong place.** A type error should be a *fact*; a timeout is not an answer. Our fuel is in the
*evaluator*, where "I gave up" is honest because the alternative was running the program at compile
time anyway.

**Sequent calculus**: [types-direction §6.3](types-direction.md) already concluded — take the
*classification* (polarity), not the core, on GHC's precedent. For decidability specifically its
gift is **focusing** (Andreoli 1992), which makes proof *search* deterministic in phases. We do not
search; we decide. So focusing is a tool for a road we chose not to take.

---

## 6. The frontier — the best thing we can get

Stated as one paragraph, because it should be quotable:

> **Quantifier-free linear integer arithmetic, discharged by a total but incomplete procedure;
> facts inferred by a relational abstract domain — octagons — with widening and narrowing;
> nonlinearity handled by declared sound axioms rather than by search; size-change termination; and
> the array-property fragment when ∀-contracts are demanded. All of it checked on a residual that
> staging has already made monomorphic, first-order and closed, so the type-checking half is a
> syntax walk and the undecidable half is quarantined into the evaluator where fuel is an honest
> answer.**

Everything past that line is undecidable, doubly exponential, or a proof assistant.

**And the four things we are giving up, named so nobody has to rediscover them:**

1. We cannot **decide program equivalence** — so no "write it fast and prove it matches".
2. We cannot **prove sortedness** in general — it is outside the array property fragment.
3. We cannot **bound a product of two unknowns** — Hilbert's tenth.
4. We cannot **prove a loop terminates when the measure is not a size-change** — Newton's method on
   a float is already the one true refusal in the corpus.

Each of those is a *published* limit, not a limitation of our effort, and that is the point of
drawing the map.

---

## 7. Two corrections hamza made, and one of them is now measured

### 7.1 Recursion is much less urgent than §2.1 of [general-purpose.md](general-purpose.md) claimed

> *"nothing prevents me from expressing json, walk a dom, using a loop. in fact if you put lisp/FP
> programmers aside, nobody uses recursion. the real impact maybe the choice of the data
> structure."*

Correct, and the second sentence is the sharp one.

**Recursion as control flow is genuinely optional and industry practice already avoids it.**
Production JSON parsers are iterative or depth-limited precisely because recursion on adversarial
input is a stack-overflow vulnerability; browsers walk the DOM with an explicit work-list; a
compiler's tree walk is a loop over an arena.

**What is not optional is recursive DATA** — a JSON value *is* `Null | Bool | Num | Str | Array of
JSON | Object of (String × JSON)`, a μ-type. And the industry answer to that is not recursion
either: it is **a flat table of nodes with integer indices**. simdjson's tape. Zig's own AST, which
is a `MultiArrayList` with `u32` node indices. An ECS world. A column store. An arena-allocated AST
in any serious compiler.

> **So the data structure this language already chose — tables indexed by integers — is precisely
> the one that makes recursive data unnecessary.**

**And it is now measured** —
[indexgraph-2026-08-21](../gauntlet/results/indexgraph-2026-08-21.md), from a benchmark that has sat
in `gauntlet/go/cycles.go` since the derivation rounds with no result file citing it:

| | pointer graph | index graph | |
|---|---|---|---|
| neighbours allocated in order | 301,588 ns | 430,249 ns | index **1.43× slower** |
| **random edges — the honest case** | 996,299 ns | 494,065 ns | index **2.02× faster** |

Both directions, and the condition is the access pattern — the third time this project has been
caught quoting a ratio without its condition. On the realistic shape, flat-and-indexed wins by 2×.

**Consequence:** ADR 0014 is no longer "the largest open question in the language". It is a
*style* question about control flow, with a measured answer for the data. The item that should be
promoted in its place is the **sum type**, which §7.2 explains is also the answer to something else.

### 7.2 Closures: C and Zig, and what the sum type quietly closes

> *"zig does not have them, does that mean we can't build apps with zig? c does not have them, does
> that mean we can't build apps with c, the world is built on it?"*

Plainly: no, and the world is built on C. C has function pointers plus a `void*` context — which is
[callbacks.md §2.2](spec/callbacks.md)'s Tier 2, and is why the Win32 API is *already shaped* for a
language without closures. Zig has function pointers, `comptime`, and **tagged unions for
dispatch**, and people write compilers, games and GUI toolkits in it.

That last one is the connection worth making:

> **The sum type is what replaces the closure for dispatch.** A dispatch table of closures becomes a
> table of tags plus a `case`. So the sum — already required for error handling
> ([general-purpose.md §2.2](general-purpose.md)) and already required for a Win32 contract's
> `_Ret_maybenull_` — **also closes the one gap the closure refusal leaves open.**

Three requirements, one mechanism. That is the strongest argument yet for building sums first, and
it makes the closure trade a much smaller one than §2.6 of general-purpose.md implied: what you
lose is *manufactured* callbacks; what you keep is no hidden allocation, a first-order residual, and
every analysis keeping its call graph.
