# The memory model: research and decision

The question [tables.md §9](spec/tables.md) left open, and the one behind
[ADR 0013](decisions/0013-accept-the-allocation-price.md). Decided in
[ADR 0018](decisions/0018-immutable-values-linear-buffers.md); this is the reasoning.

---

## 0. The question is really four, and two of them are not language questions

| | question | whose |
|---|---|---|
| **Q1** | Can a program **observe** a change to a table it holds? | **language** |
| **Q2** | Is there a portable way to **write into** storage? | **language** |
| **Q3** | Can two names denote the same storage — **aliasing**? | **language** |
| **Q4** | **Who frees**, and when? | **target** |

Q4 is a target concern and separating it dissolves half the difficulty. Go, the JVM and V8 each
bring a garbage collector we neither control nor pay for twice. `targets/windows/` brings
`VirtualAlloc` and `HeapAlloc` and **no collector at all**, so x86 needs a reclamation discipline
that the other three must not be burdened with. That is exactly the parasite model: the *semantics*
are the language's and must be uniform; the *reclamation* is the target's and may differ.

So this document decides Q1–Q3 and treats Q4 as an emission strategy (§8).

---

## 1. Five things about this language that change the answer

Before the literature, because they eliminate most of it.

**1. There is no recursion** ([ADR 0014](decisions/0014-recursion-is-not-in-the-language.md)) **and
no recursive type** ([data-structures.md §1.2](data-structures.md)). Therefore **the heap is
acyclic by construction.** No cycle collector is ever needed, and reference counting — if we wanted
it — would be complete. That is Koka's and Lean 4's precondition, and we get it from a decision
already made for another reason.

**2. Closures are refused** ([g6](derivations/g6-escaping-closures.md)). A function cannot capture a
table and outlive it. This kills the single largest source of pain in every uniqueness-typed
language: Clean's and Futhark's uniqueness has to survive higher-order code, and ours does not have
any.

**3. Reduction is whole-program.** `cmd/build` reduces to a nullary `main`; every non-exported
function is inlined. So an analysis that would need a modular type system elsewhere is a **local
analysis on one term** here. Only an *exported* function has a boundary.

**4. The reducer already counts occurrences.** `core/reduce.go:809` — `occurrences(t, name)`. The
substructural machinery ADR 0013 hypothesised already exists and is already load-bearing for
call-by-need.

**5. The effect discipline already sequences impure terms**
([ADR 0010](decisions/0010-effects-as-structural-rules.md)). An impure argument is *never
substituted* — it is normalised and let-bound whatever its occurrence count, which denies
contraction, weakening and exchange. **A store is impure. The mechanism that keeps two stores from
being reordered, dropped or duplicated is already built and already tested.**

Points 4 and 5 together are ADR 0013's own correction, which said the nearest machinery is
"ADR 0010's purity-conditioned structural rules together with the reducer's occurrence counting"
and called it *a hypothesis rather than a finding*. This document is the argument that the
hypothesis is right.

---

## 2. The literature

### 2.1 The aggregate update problem — named in 1985

Hudak and Bloss, *The aggregate update problem in functional programming systems* (POPL 1985), is
this exact question forty years early: a pure language must copy an array to update one element,
unless the compiler can prove the old value is dead. They proposed abstract interpretation to find
out.

**SISAL** is the existence proof and the cautionary tale. A pure dataflow language for numerics
with *update-in-place analysis* and *copy elimination*; Cann's *Retire Fortran?* (1992) reported it
matching and sometimes beating Fortran on Livermore loops. It worked. It also required heroic
compiler work, gave the programmer **no source-level guarantee**, and died for funding reasons
rather than technical ones.

The lesson to take: **the analysis works, and an optimisation with no source-level guarantee is a
performance cliff you cannot see.** That is precisely the failure mode
[bce-2026-08-15](../gauntlet/results/bce-2026-08-15.md) already required a diagnostic for.

**Haskell** is the negative empirical result. `Data.Array`'s immutable API is famously slow, and
essentially every performance-sensitive Haskell program uses `Data.Vector` with `ST` and
`unsafeFreeze`. Twenty-five years of a large community converging on *build mutably in a scope,
then freeze* is data.

### 2.2 Uniqueness and linearity

**Wadler**, *Linear types can change the world!* (1990) — the founding paper, and its motivating
example is in-place array update. Girard's linear logic (1987) underneath.

**Clean** (Barendsen & Smetsers, 1993/1996) — *uniqueness types*, and the distinction from linear
types matters: linearity constrains **future** use, uniqueness constrains the reference count
**now**. Clean's `*World` and `*Array` work, and the standard complaints are **uniqueness
propagation** (a unique field forces its container unique) and the interaction with higher-order
code. Both complaints are about structures we do not have (§1.2, §1.1).

**Mercury** has unique modes. **ATS** (Xi) combines linear types with DML refinements and compiles
to C with no runtime — the closest thing to "our type system plus linearity" that exists.

**Rust** (2014) — ownership, borrowing, lifetimes. It works spectacularly and costs a learning
curve and a region system. We do not want lifetimes: our lifetimes are lexical because there is no
recursion and no escape.

**Linear Haskell** (Bernardy, Boespflug, Newton, Peyton Jones, Spiwack, POPL 2018) — multiplicity
arrows `a %1 -> b`, retrofitted. Shows linearity can be added to a language that did not have it,
at the cost of multiplicity polymorphism everywhere.

**Quantitative Type Theory** (McBride 2016; Atkey 2018), shipped in **Idris 2** — quantities 0, 1, ω
in a dependent setting. The `0` quantity is *erasure*, which is what our staging already does.

**Granule** (Orchard, Liepelt, Eades 2019) — graded modal types; linearity as one point in a lattice
of grades. The general form of what ADR 0010 does with one bit.

**Futhark** (Henriksen et al., PLDI 2017) and **Cogent** (O'Connor et al., ICFP 2016) are the two
closest languages that exist, and both chose uniqueness:

| | Futhark | Cogent | us |
|---|---|---|---|
| recursion | **no** | **no** | **no** |
| primary structure | arrays | records + arrays | tables |
| in-place update | **uniqueness types** | **uniqueness types** | ? |
| compiles to | GPU/C | C | 4 targets |
| verified | no | **yes** | no |

Two independent languages with our exact constraint — *no recursion, arrays primary, must reach C
performance* — reached uniqueness. That is the strongest single piece of evidence in this document.

### 2.3 Reference counting and reuse

**Perceus** (Reinking, Xie, de Moura, Leijen, PLDI 2021) — precise reference counting with reuse
analysis; if the count is 1, update in place. **Koka** and **Lean 4** ship it, and Lean's array
update is in-place exactly when uniquely referenced. Their term for the style is **FBIP**:
functional but in-place.

It requires: an acyclic heap (we have it, §1.1) and **control of the allocator** (we do not, on
three of four targets). Inserting our own refcounts on top of Go's, HotSpot's and V8's collectors
means paying twice and defeating each host's own escape analysis — which
[product-2026-08-19](../gauntlet/results/product-2026-08-19.md) measured *removing* an allocation
on Go and scalar-replacing a record on Java. **Perceus is right and unavailable.**

**Swift's** ARC plus copy-on-write collections is the same idea in an industrial language, and it
carries the well-known hazard: a silent copy when the refcount is unexpectedly 2, invisible at the
call site. A performance cliff with no diagnostic.

### 2.4 Regions

**Tofte & Talpin** (1997), region-based memory management in the MLKit — static region inference,
stack-like discipline, no GC. **Cyclone** (Jim, Morrisett et al. 2002) put regions in a C-like
language. Rust's lifetimes are region inference made explicit.

Regions buy nothing on Go/JVM/JS — the host collector is already there and sub-allocating out of a
big buffer defeats it. On **x86 they are close to mandatory**, which is §8.

### 2.5 Mutable value semantics

**Val / Hylo** (Racordon, Abrahams et al.) and **Swift's `inout`** — values are independent,
mutation happens through `inout` parameters with *exclusive access*, and no reference escapes.
Swift's "law of exclusivity" is checked statically where possible and dynamically otherwise.

This is genuinely attractive and it is a different framing of the same discipline: instead of
"this value is unique", it is "this parameter is exclusively borrowed for the duration of this
call". It is worth knowing that the two are interconvertible, and that Val's claim is that the
`inout` framing is *much* easier to teach than Rust's.

### 2.6 Scoped mutation — the ST pattern

**Launchbury & Peyton Jones**, *Lazy functional state threads* (PLDI 1994) — `runST :: (forall s.
ST s a) -> a`. A rank-2 type prevents the mutable state escaping the scope, so mutation is
*unobservable* from outside and the whole thing is pure. Then `unsafeFreeze` converts the mutable
array to an immutable one with **no copy**, safe because nothing else holds it.

This is what Haskell actually does in practice, and it is the pattern §2.1 says a community
converged on. Its cost in Haskell is the rank-2 type, which exists solely to stop the buffer
escaping in a **closure**.

> **We refuse closures. The rank-2 type has nothing to prevent.**

### 2.7 Concurrency, parallelism, and data races

**The memory models**: the Java Memory Model (Manson, Pugh, Adve, POPL 2005) and the C++11 model
(Boehm & Adve, PLDI 2008) both make a data race undefined or nearly so; Go's model is
happens-before with races explicitly not defined, which is why Go ships a race detector.
JavaScript is single-threaded per agent, with `SharedArrayBuffer` + `Atomics` as the escape.

**Data-race freedom by construction** is achievable and there are three shapes of it:

- **Rust's `Send`/`Sync`** — from ownership. A `&mut` is exclusive, so no race is expressible.
- **Pony's reference capabilities** (`iso`, `val`, `ref`, `box`, `trn`, `tag`) — a lattice of
  aliasing rights; `iso` is uniquely owned and sendable, `val` is immutable and freely shareable.
- **Immutability** — Erlang, Clojure. Nothing to race on.

**Deterministic parallelism**: DPJ (Deterministic Parallel Java), **LVars** (Kuper & Newton),
Concurrent Revisions, and — most relevantly — **Futhark**, whose parallel constructs are
side-effect free so every parallel program has one answer.

The point that matters for us: **`(alloc t)` where `t` is `(table n f)` is embarrassingly parallel
by construction**, because `f` is a pure function of the index and the elements are independent.
That property is worth a great deal and it is destroyed by unchecked mutation. Any memory model we
pick must preserve it.

---

## 3. What works and what does not — the record

| approach | shipped in | verdict for us |
|---|---|---|
| Pure + update-in-place analysis | SISAL | **works, no guarantee** — a cliff you cannot see |
| Pure + no escape hatch | early Haskell arrays | **does not work** — the community routed around it |
| Uniqueness types | Clean, Mercury, **Futhark**, **Cogent**, ATS | **works**, and twice under our exact constraints |
| Ownership + borrowing + lifetimes | Rust | **works**, costs a region system we do not need |
| RC with reuse | **Koka**, **Lean 4**, Swift | **works, unavailable** — we do not own the allocator |
| COW everywhere | Swift collections | **works, unpredictable** — the silent copy |
| Regions | MLKit, Cyclone | **works**, and is a *target* answer for x86 |
| Mutable value semantics | Val/Hylo, Swift `inout` | **works**, and is uniqueness in nicer clothes |
| Scoped mutation, then freeze | **Haskell `ST`** | **works**, and its one cost is closures |
| Unchecked mutation | Go, Java, JS, C | works and forfeits parallelism, races, and η-tab |

---

## 4. The candidates

### M-A — Immutable, reuse recovered by the compiler

Every table is a value; `alloc` always allocates; a liveness or occurrence analysis removes the
allocations it can.

**Buys:** the simplest semantics, η-tab sound, parallelism free, nothing new in the surface.
**Costs:** the 2.5–2.7× when the analysis does not fire, silently, and SISAL's lesson says it will
sometimes not fire. And it **cannot express a scatter at all** — see §5.

### M-B — Uniqueness types on parameters

`(sig smooth-into ((dst *(array f64)) (a (array f64))) *(array f64))`, Clean/Futhark style.

**Buys:** buffer reuse across an *exported* boundary; the full 2.7×; deterministic free on x86.
**Costs:** uniqueness in the type language and therefore in every interface;
[ADR 0010](decisions/0010-effects-as-structural-rules.md) declined a value-level substructural
property once already, on the grounds that it is visible to the programmer in a way `pure` is not.

### M-C — Scoped mutable buffer, frozen at the boundary

`(build n (fn (b) …))` gives a mutable buffer inside a scope and an immutable table outside it.
Haskell's `ST`, without the rank-2 type because closures are refused.

**Buys:** **scatter** — sieve, sort, histogram, union-find; buffer reuse *inside* a program,
because reduction inlines every non-exported function; parallelism preserved, because a buffer is
never shared; η-tab sound outside the scope.
**Costs:** a second thing that is table-shaped (`buffer` versus `table`); a linearity check on the
buffer; and it does **not** cross an exported boundary.

### M-D — Unchecked mutation, portably

`(set t i v)` on any table.

**Buys:** everything, immediately.
**Costs:** β is no longer sound (substitution changes meaning), η-tab dies, parallelism dies, data
races become expressible, and the reducer's entire model breaks. **Rejected without further
argument** — this is not a memory model, it is the absence of one.

### M-E — Copy-on-write

**Rejected**: needs refcounts we cannot afford on three targets (§2.3), and the silent copy is a
performance cliff with no diagnostic, which requirement 5 does not tolerate.

---

## 5. The argument that decides it: scatter is about expressiveness, not speed

The stencil's 2.7× is a *performance* argument and it is the one this project has been staring at
since ADR 0013. It is not the important one.

**Without a way to write into storage, a whole class of algorithms is not expressible portably at
any speed.**

- **Sieve of Eratosthenes.** Element `i` is determined by writes made while processing `j < i`.
  `(table n (fn (i) (prime? i)))` with trial division is O(n√n) against the sieve's O(n log log n).
  Not a constant factor — a different algorithm.
- **Sorting in place**, **histogram**, **union-find**, **BFS/DFS**, **general dynamic programming**
  where a cell depends on more than a prefix.

Every one needs **scatter**: a write at an index computed from the data. `(table n f)` is a
*gather* — element `i` is a function of `i`. Gather cannot express scatter; that is the same
container-morphism boundary that made `filter` impossible.

`examples/native/sieve-go.oro` is already in the repository and it uses `go.set-bool` — the
target-native store. **It is one of the two sieves the interval analysis and the size-change
termination checker were built against, and it cannot be written portably today.**

So M-A is not merely slow, it is incomplete, and M-C is not a performance feature.

---

## 6. The recommendation

> **Immutable values, and one scoped, linear, mutable buffer — M-C.**
> Uniqueness stays out of the type language. The linearity check is occurrence counting on the
> residual, which already exists. Crossing an *exported* boundary with a buffer (M-B) is deferred,
> named, and not needed for any measured case.

### 6.1 Why the check costs nothing new

A buffer is created by `build`, threaded, and frozen when `build` returns. It cannot escape:

- not in a closure — closures are refused;
- not through a function — reduction inlines every non-exported function, so in the residual the
  buffer is **lexically local to its `build`**;
- not into a table — `(table n f)` capturing a buffer would be a closure.

So the property to check is: **within the `build` scope, the buffer is used at most once at each
point**, which is `occurrences(t, b) ≤ 1` per continuation. `core/reduce.go:809` already computes
it.

**And the sequencing is already correct.** A store is impure, so
[ADR 0010](decisions/0010-effects-as-structural-rules.md) let-binds it rather than substituting it,
whatever its occurrence count — which denies contraction (no duplicated store), weakening (no
dropped store) and exchange (no reordered store), in that order. The three properties a mutable
buffer needs are the three the effect discipline already provides, and they were built for
`print-line`.

### 6.2 Why a buffer is a different type from a table

Reading a **table** is pure: `(a i)` always gives the same answer. Reading a **buffer** is not — it
depends on the stores that happened before. So they are different types and pretending otherwise
would be a lie:

```
(array V)    reads are pure       shareable, parallelisable, η-tab holds
(buffer V)   reads are impure     linear, scoped, never shared
```

`build` is the only introduction of a buffer and the only elimination of one, and the freeze at its
boundary **costs nothing** — no copy, because linearity guarantees nobody else holds it. That is
`unsafeFreeze`, made safe by construction rather than by a rank-2 type.

### 6.3 Where the boundary is, and it is the same boundary as everything else

A buffer may cross any function boundary that reduction removes — which is every one except an
exported signature. `cmd/build` reduces a whole program to `main`, so **inside a program, buffer
reuse works**: a caller can allocate once and thread the buffer through as many transformations as
it likes, and they all inline.

What fails is exported `smooth-into(dst, src)` compiled by `cmd/gen` for a hand-written caller.
That is M-B, and it is deferred: no measured case in this repository needs it, and it is the same
export boundary that already limits the negative product, the refinement annotation and the
interval analysis.

---

## 7. What it looks like

```lisp
;; GATHER — element i is a function of i. Pure, parallel, no buffer.
(def smooth (fn (a)
  (alloc (table (- (len a) 2)
                (fn (j) (f/ (f+ (f+ (a j) (a (+ j 1))) (a (+ j 2))) 3.0))))))

;; SCATTER — the sieve. `b` is a buffer, threaded linearly, frozen on the way out.
(def sieve (fn (n)
  (build n (fn (b)
    (loop ((b b) (i 2))
      (>= (* i i) n)  b
      (b i)           (again b (+ i 1))          ; already composite
      else            (again (cross b i n) (+ i 1)))))))

(def cross (fn (b i n)
  (loop ((b b) (j (* i i)))
    (>= j n)  b
    else      (again (set b j true) (+ j i)))))
```

`(set b j true)` **consumes** `b` and returns it, so each occurrence is linear and the loop
variable carries the thread. That is the shape `examples/native/sieve-go.oro` already has with
`go.set-bool`, and it emits the same code — measured at parity, with the sieve's `c[i]` *proven*
in bounds rather than propagated.

The non-linear mistake, and its diagnostic:

```lisp
(seq (set b 0 1.0) (set b 1 2.0))
```
```
in sieve: `b` is used twice here, but a buffer may be used once.
  A store consumes the buffer and returns it, so thread it:
      (let (set b 0 1.0) (fn (b) (set b 1 2.0)))
  or carry it as a loop variable.
```

---

## 8. Reclamation, which is a target decision

| target | who frees | how |
|---|---|---|
| Go | the host collector | nothing to do |
| JVM | the host collector | nothing to do |
| JavaScript | the host collector | nothing to do |
| **windows/x86** | **us** | see below |

x86 has `VirtualAlloc` and `HeapAlloc` and no collector, and this is where §1.1 pays: **the heap is
acyclic, allocation happens only at `alloc` and `build`, and there is no recursion**, so a
lexically-scoped discipline is sufficient. Three implementable options, in order of increasing
cleverness: an **arena per program** (trivially correct, unbounded footprint), an **arena per
`build`/`alloc` scope** freed at the last use the occurrence analysis already computes
(Tofte-Talpin's insight with none of its inference, because our regions are lexical), or
**Perceus-style refcounting**, which is *available here* precisely because we own this allocator.

None of that is visible in the language, and choosing it is a windows-target measurement rather
than a decision this document makes.

---

## 9. Concurrency, parallelism, safety, correctness

**Parallelism is preserved and it is free where it matters.** `(alloc (table n f))` is
embarrassingly parallel by construction — `f` is a pure function of the index, elements are
independent, and no analysis is required to know it. `(build …)` is sequential by construction.
**The distinction is in the source**, which is Futhark's design and the reason its parallel
programs are deterministic.

**Data races are not expressible.** Immutable tables are freely shareable — Pony's `val`, and the
reason Erlang and Clojure need no locks. A buffer is linear and scoped, so it is never shared —
Pony's `iso`, Rust's `&mut`. We get `Send`/`Sync` for free without writing them down, and the
Java, Go and C++11 memory models never come into it because there is nothing to synchronise.

**Correctness.** β stays sound, because a store is impure and impure terms are never substituted.
η-tab — `(alloc (table (len a) (fn (i) (a i)))) = a` — becomes **sound on tables**, which was
ADR 0013's fifth reopening trigger and is now a rewrite the compiler may actually take.

**Safety.** Out-of-bounds is a precondition, discharged by the refinement system
([tables.md §6](spec/tables.md)); use-after-free is not expressible, because there is no free in
the language; and aliasing is not expressible, because a buffer is linear and a table is immutable.

**If threads are ever added**, the shape is already fixed: tables are `val` and sendable, buffers
are `iso` and move rather than copy. That is a decision this model makes cheap rather than one it
forecloses.

---

## 10. The costs, honestly

**Two table-shaped things.** `(array V)` and `(buffer V)`, and a programmer must know which they
have. That is Haskell's `Array`/`STArray` and `Vector`/`MVector` split, which is real friction and
which twenty-five years of practice suggest is the right friction.

**A linearity error is reported on the residual**, so the diagnostic must carry source positions
back through reduction. The refinement checker has the same problem and has not solved it well.

**Threading is syntactically heavier than assignment.** `(let (set b 0 v) (fn (b) …))` against
`b[0] = v`. `loop` absorbs most of it, and a `do`-style sugar that threads implicitly would absorb
the rest — deliberately not specified here, because sugar should follow a real program's shape
rather than precede it.

**And the export boundary still cannot take a buffer**, so an exported `smooth-into` remains
target-native. That is M-B, deferred with its trigger written down: **a measured case that needs
buffer reuse across an exported API.**

**What could falsify the whole recommendation:** if occurrence counting on the residual turns out
to reject programs that are obviously fine — if the threading discipline is *usably* linear only
in the shapes we happen to have written — then the answer is uniqueness types after all, and
Futhark and Cogent were right to pay for them.
