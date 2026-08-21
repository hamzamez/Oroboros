# 0018 — Immutable values, and one scoped linear buffer

Date: 2026-08-21
Status: Accepted

Research: [memory-model.md](../memory-model.md). Specification it completes:
[tables.md §9](../spec/tables.md).

## Context

[tables.md](../spec/tables.md) specified the primary data structure and deliberately left the
memory model undecided. [ADR 0013](0013-accept-the-allocation-price.md) accepted a 1.8–2.0×
allocation price provisionally and named four triggers for reopening it; a fifth was added when
η-tab turned out to be a rewrite the compiler is entitled to and cannot take.

The performance question — buffer reuse, measured at **2.5–2.7×** and paid by hand-written code
too ([native-gauntlet-2026-08-20](../../gauntlet/results/native-gauntlet-2026-08-20.md)) — is not
the one that decides this.

**The one that decides it is expressiveness.** Without a way to write into storage, scatter is
inexpressible: the Sieve of Eratosthenes, in-place sorting, histograms, union-find, breadth-first
search, and dynamic programming whose dependencies are not a prefix. `(table n f)` is a *gather* —
element `i` is a function of `i` — and gather cannot express scatter, which is the same
container-morphism boundary that makes `filter` impossible on a pull table.

`examples/native/sieve-go.oro` is in this repository, is one of the two programs the interval
analysis and the size-change termination checker were built against, and **cannot be written
portably today.**

## Decision

**Values are immutable. Mutation exists only inside one scoped construct, whose buffer is used
linearly and is frozen on the way out.**

```lisp
(alloc t)                    ; a table in memory, from a rule. Pure. Parallel by construction.
(build n (fn (b) body))      ; a mutable buffer, scoped. Sequential. Returns an immutable table.
(set b i v)                  ; consumes b, returns b
```

Four consequences, stated so they can be checked:

1. **A table is immutable and freely shareable.** `(array V)` reads are pure.
2. **A buffer is linear and scoped.** `(buffer V)` reads are *impure*; it is introduced and
   eliminated only by `build`; it may be used at most once at each point.
3. **The linearity check is `occurrences` on the residual**, not a type. Uniqueness does not enter
   the type language and does not appear in any signature.
4. **The freeze at `build`'s boundary copies nothing**, because linearity guarantees nothing else
   holds the buffer.

**Reclamation is a target decision, not a language one.** Go, the JVM and V8 bring collectors;
`targets/windows/` brings `VirtualAlloc` and none, and gets a lexically-scoped arena or
Perceus-style refcounting — an implementation choice invisible in the language, and available there
precisely because we own that allocator.

## Why this costs almost nothing to build

Every mechanism it needs already exists, for other reasons:

- **The heap is acyclic** because [ADR 0014](0014-recursion-is-not-in-the-language.md) removed
  recursion and there are no recursive types. No cycle collector is ever needed.
- **A buffer cannot escape** because closures are refused ([g6](../derivations/g6-escaping-closures.md)) —
  which is the *only* thing Haskell's rank-2 `runST` type exists to prevent.
- **A buffer is lexically local in the residual** because reduction is whole-program and inlines
  every non-exported function.
- **`occurrences(t, name)` is already in the reducer** (`core/reduce.go:809`), load-bearing for
  call-by-need.
- **Stores are already sequenced correctly.** A store is impure, and
  [ADR 0010](0010-effects-as-structural-rules.md) never substitutes an impure argument — denying
  contraction (no duplicated store), weakening (no dropped store) and exchange (no reordered
  store). The three properties a mutable buffer needs are the three the effect discipline already
  provides, built for `print-line`.

ADR 0013's correction hypothesised exactly this — *"the nearest machinery is ADR 0010's
purity-conditioned structural rules together with the reducer's occurrence counting"* — and
explicitly declined to treat it as fired until it was tested. **This ADR is that hypothesis
argued; it is not yet that hypothesis measured**, and §"Consequences" says what would falsify it.

## Why not

**(a) Immutable, with reuse recovered by a compiler analysis.** SISAL did this and matched Fortran
on Livermore loops (Cann, *Retire Fortran?*, 1992). *Rejected* because it gives the programmer no
source-level guarantee — an optimisation that silently does not fire is the failure mode
[bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already required a diagnostic for — and
because **it does not express scatter at any speed**, which is the decisive point above.

**(b) Uniqueness types on parameters**, Clean/Futhark/Cogent style. *Deferred, not rejected.* It is
the only option that reaches buffer reuse across an **exported** boundary, and two languages with
our exact constraints — no recursion, arrays primary, must reach C performance — both chose it.
It is deferred because no measured case in this repository needs it, because it puts a
substructural property in every interface, which [ADR 0010](0010-effects-as-structural-rules.md)
declined once on the grounds that it is visible in a way `pure` is not, and because the decision
here gets the same power inside a program for free. **Its trigger is named below.**

**(c) Reference counting with reuse** — Perceus (Reinking, Xie, de Moura, Leijen, PLDI 2021), as
shipped in Koka and Lean 4. *Rejected as a language mechanism, kept as a windows-target option.*
It is right and it is unavailable: three of four targets bring their own collector, and inserting
our refcounts on top means paying twice and defeating the host escape analysis that
[product-2026-08-19](../../gauntlet/results/product-2026-08-19.md) measured *removing* an
allocation on Go and scalar-replacing a record on Java.

**(d) Copy-on-write everywhere**, Swift-style. *Rejected* — it needs the refcounts of (c), and its
silent copy when the count is unexpectedly two is a performance cliff with no diagnostic, which
requirement 5 does not tolerate.

**(e) Regions**, Tofte-Talpin. *Rejected as a language mechanism, adopted as the likely windows
implementation.* Regions buy nothing where a host collector already exists, and sub-allocating out
of a large buffer defeats it.

**(f) Unchecked portable mutation.** *Rejected without argument.* β stops being sound because
substitution would change meaning, η-tab dies, parallelism dies, and data races become expressible.
That is not a memory model; it is the absence of one.

## Consequences

**η-tab becomes sound on tables.** `(alloc (table (len a) (fn (i) (a i)))) = a` is a rewrite the
compiler may now take. That was ADR 0013's fifth reopening trigger, and this decision fires it.

**ADR 0013 is not superseded, and its price is now bounded rather than accepted.** A portable
program still allocates where a caller-supplied buffer would not, at an *exported* boundary. Inside
a program it need not, because reduction removes the boundary. The remaining gap is (b)'s.

**Parallelism is preserved and the distinction is in the source.** `(alloc (table n f))` is
embarrassingly parallel by construction; `(build …)` is sequential by construction. That is
Futhark's design and the reason its parallel programs are deterministic.

**Data races are not expressible.** Tables are immutable and shareable — Pony's `val`. Buffers are
linear and scoped — Pony's `iso`. `Send`/`Sync` without writing them down, and the Java, Go and
C++11 memory models never come into it.

**Two table-shaped things** now exist, `(array V)` and `(buffer V)`, and a programmer must know
which they hold. That is Haskell's `Array`/`STArray` split, which is real friction and which
twenty-five years of practice suggests is the right friction.

### What should reopen this

1. **Occurrence counting rejects programs that are obviously fine.** If the threading discipline is
   usably linear only in the shapes we happen to have written, then uniqueness types are the answer
   after all and Futhark and Cogent were right to pay for them. **This is the one to watch, and it
   can only be found by writing awkward programs.**
2. **A measured case needing buffer reuse across an exported API.** That is (b), and it should
   produce an ADR adopting uniqueness on parameters rather than a workaround.
3. **The diagnostic proves untranslatable.** A linearity error is found on the residual; if source
   positions cannot be carried back well enough for the message to be usable, the check belongs
   somewhere earlier and that is a different design.
4. **A target appears whose allocator we own and whose footprint matters** — Android is
   [ADR 0004](0004-first-targets.md)'s reason for the JVM, and requirement 6 is about size.
   Perceus becomes available exactly there.
