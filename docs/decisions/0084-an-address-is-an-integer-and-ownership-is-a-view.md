# 0084 — An address is an integer, and ownership is a view

Date: 2026-10-30
Status: Accepted; not built

## Context

The third of the seven walls [llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md)
found in Σ_LLVM is pointers: `ptr`, the casts between pointers and integers, and the attributes on a
pointer parameter. Win32, the Windows track's host (6c, 6d), passes almost everything through one: strings,
records, out-parameters, buffers, handles, code, and integers carrying pointers. TinyRetroPad imports 67
Win32 functions.

[pointers-research.md](../pointers-research.md) has three parts:
- **Part I** derived Theorem P: a host's pointer parameter that the host does not keep is observably a
  value passed and a result returned. It first proposed that no pointer is ever a value.
- **Part II** took hamza's sketch, "addresses are just integers … the ability to modify the content … is
  just a cell … an address will be owned by the scope that introduces it, passed and borrowed, or returned
  and frozen", and found that it holds once ownership is moved off the integer and onto a view, a claim in
  the logic.
- **Part III** answered four questions: views in the surface, uninitialized memory, the linear result,
  and the bytes library.

hamza (2026-10-30) accepted P1′ and P2–P6, Part II's direction and items 5–6, and Part III's D, U3, the
linear result and the codecs.

## Decision

**Addresses and views** (Part II):

1. **An address is an integer offset into a region.** A region is a name, made fresh by its allocator and
   compared by equality. The addresses of one region form an affine space over ℤ: an address plus an
   integer is an address, and two addresses of the same region differ by an integer. The pair (region,
   address) is never a value. The region is always known statically, as the view's, and only the offset
   is dynamic, so provenance is kept without being carried at run time.

2. **Ownership is a view**: a claim in the logic (ADR 0049), erased, that a scope may access a range of a
   region with a permission.
   - Views form a separation algebra. They split and join along ranges, and disjointness of ranges with
     linear bounds is Presburger arithmetic, decided by the Omega test.
   - The permissions:

     | permission | may | the language today |
     |---|---|---|
     | exclusive | read and write; linear | a `var` buffer, `linear` |
     | fraction | read | an observer |
     | persistent | read; duplicable | a frozen buffer, a constant |
     | device | read and write, each an effect | nothing |

   - A view is consumed by its scope's end, returned when lent, moved, or frozen. It is linear, not affine,
     so a leak cannot be written.

3. **No load or store without a view.** That is the whole restriction, on *how*, not on *what*. An access
   is proven inside its view's range, or checked: a bounds test, a validity test on punned bytes, or a
   count on a shared view. There is no `unsafe`, and the only trusted code is a host declaration
   (ADR 0022).

4. **Theorem V**: under an exclusive view, memory's contents are a value threaded by the state-passing
   translation. So buffers and cells are the special case where the compiler made the region, and every
   existing proof carries over.

5. **The root region**, the whole address space with origin 0, belongs to a target that offers it (an OS,
   bare metal): Tier 2. There `ptrtoint` and `inttoptr` are coordinates. They are not offered elsewhere.

**The surface** (Part III, option D):

6. **The binders that exist are abbreviations of view formulas**, by D:
   - a parameter without a mode is a fraction, returned unchanged;
   - `linear` is an exclusive view, lent and returned;
   - `var` is a fresh exclusive region;
   - a segment `b[lo:hi]` is the splitting law;
   - a linear result is a move out;
   - `out S` is a fresh region the host fills.

   **A formula, with ↦ and ∗, is written in `where` or `ensure` only where no binder says it**: contents
   promised, a protocol across calls, a split kept. Segments become views anywhere, not only at host calls
   (amends ADR 0074). Two segments of one buffer may be lent at once when their ranges are proven
   disjoint.

7. **A definition may return a linear result**: a writable view it owns, moved to the caller (extends
   ADR 0018, 0052 and 0070 from host declarations to definitions). It brings two rules and one existing
   property:
   - **owned only**: a function moves out a view it made or received by move, never one lent to it;
   - **generativity**: each call makes a fresh region, and no transformation merges two calls;
   - **relevance** (ADR 0043): the result must be consumed.

   A segment as a moved result waits for a program.

8. **Uninitialized memory is never visible (U3).** A table is born full: by its rule, `array { e : k ∈ … }`,
   or at a host's boundary.
   - There, `out S` takes a **set of tables** (extending ADR 0048 from a cell to a table), stated by
     separation (ADR 0061), e.g. `out set { t ∈ Array(0..255) : len(t) <= 4096 }`.
   - The allocation is the set's supremum length, and a set with none is refused.
   - The host fills a prefix, and the program receives an element of S.

   No `of` and no new word. Separation's build is needed.

**Hosts** (Part I, P2–P6, as Part II restated them):

9. **P2: a host's pointer parameter is a view lent for one call**, its mode from the host's annotations:
   - `_In_` is a value;
   - `_Out_` is a result (fixed T) or `out S` at the call (the caller's T);
   - `_Inout_` is `inout`, or `linear` on a table;
   - `_In_opt_` is an `Option`, and an unwanted `_Out_opt_` is a NULL in the template.

   On LLVM a mode prints its premises: `captures(none)`, `noalias`, `readonly` or `writeonly`,
   `initializes`, `dereferenceable(n)`, `nonnull`, `align`. These are proofs on the language's functions,
   and claims on a host's, checked against SAL where SAL speaks. The windows target's hidden global cells
   (`__read`, `__written`) become results.

10. **P3: an opaque handle is a host type with equality**, one per kind `STRICT` `windows.h` distinguishes,
    replacing the windows target's `ptr`. Its failure value is a niche: on the type when every function
    shares it, on the declaration's result when it varies (`CreateFileA`'s −1).

11. **P4: a pointer the host keeps is a protocol of views.** `FindTextA` moves the struct's view to the
    host, the window procedure is lent it back for each message, and the dialog's end returns it. It is
    declared with 6d's window procedure.

12. **P5: a family indexed by a constant is one declaration per member** (`SendMessage`'s messages, as
    `windowsx.h`'s crackers); the integer carrying a pointer is the template's.

13. **P6: memory the host returns is a view kind**: owned (`VirtualAlloc`), lent for an epoch
    (`GlobalLock`, borrow-research's `lent`), or persistent (`GetCommandLineA`, decoded at the boundary,
    ADR 0030).

**The bytes library** (Part III, §22):

14. **A codec is a partial isomorphism between a set and k bytes**: e total and injective, d partial,
    d ∘ e = id. It is ADR 0083's (H, d) with H = Bᵏ, and a static value erased by staging.
    - Codecs are closed under product, power, byte order and padding.
    - `lib/bytes` offers `codec(S, order)`, whose width is the narrowest that holds S, plus `f64`, `pair`,
      `times`, `pad` and `width`.
    - It offers `get` (total codecs only), `check` (an `option`), `put`, and `table`, a typed view of a
      byte region.
    - A `put` then `get` round-trips as a fact, without a check.
    - The byte order is always explicit.
    - **A record's memory layout is a codec**, which 6d uses.

15. **Shared and atomic views are concurrency's** (ADR 0081). Page tables wait for an OS target.

## Why not

- **Pointers as a sort, the casts offered to every program** (Part I §7A). An address's integer is the
  allocator's choice and changes every run. JavaScript and the JVM have none. A load from any integer
  breaks the no-aliasing property every proof stands on, in every module and not only the one that uses
  it.
- **References as values** (Part I §7B). Identity and sharing would need full separation logic in the
  claims, for 63 of TinyRetroPad's 67 functions that need none of it.
- **Ownership on the integer** (the sketch's first form, Part II §11.1). An integer can be copied,
  recomputed and read from input, so it cannot be owned. Ownership needs an object the program cannot
  forge.
- **Rust's fused reference** (`&mut T`, §13.1). A reference that is its own permission cannot be stored
  in data, so a doubly linked list needs run-time counts or `unsafe`. Separating the address from the view
  makes links plain integers in an arena.
- **Run-time capabilities as the default** (CHERI, §13.2). They make every pointer fat and check every
  access. Erased views cost nothing where proven, and on a CHERI target a view may print as one.
- **Affine views** (§13.3): a silent leak.
- **Absolute addresses outside the root region** (§13.4): nondeterministic, not relocatable, not portable.
- **An `unsafe` escape** (§13.5): it suspends the proof. A check takes its place.
- **Full separation logic as the claims' logic** (§13.7): undecidable with inductive predicates. Views on
  ranges decide in Presburger arithmetic, and shapes are content facts.
- **C's `restrict` and strict aliasing** (§13.8): promises the compiler assumes and no tool checks. Here
  the same facts are theorems.
- **Shen's globals** (§13.9): a view everyone holds is aliasing everywhere.
- **Region inference** (§13.10): the regions are the program's scopes and buffers, written where they are.
- **Ownership types on objects** (§13.11): they presume a heap of objects, and this language has regions
  of elements.
- **Surface A**, inferred only (§19.2): it cannot promise contents or state a protocol. **B**, views as
  threaded values (§19.3): it doubles the code and puts the logic in the computation. **C**, a formula in
  every header (§19.4): heavy where a mode says it all, and it refuses a swap of an element with itself
  unless a disjunction is written.
- **Uninit as a typestate in the surface (U1)**: a typestate on every buffer, to save a memset on one
  target of four; it waits on a measured trigger. **Target-only (U2)**: a portable meaning made
  non-portable. **Always zeroed (U0)** stays inside the compiler, wherever no rule or host write covers
  a table.
- **`out Array(V) of n`** (§20.4): the set operations say it already, and the supremum form also states
  the host's result.
- **A native byte order in `lib/bytes`** (§22.4): not portable. A host struct's order is its declaration's
  codec.

## Consequences

- **Amends**: ADR 0048 (`out` takes a set of tables), ADR 0074 (a segment is a view anywhere), ADR 0018,
  0052 and 0070 (a definition's linear result), ADR 0083 (a codec is a representation over bytes).
  pointers-research.md's P1 is replaced by P1′.
- **Built on every target, independent of `ir/llvm`**:
  - separation (ADR 0061), with `len` in its formula;
  - `out` on a set of tables;
  - the linear result with its two rules;
  - segments as views, with disjoint segments lent together;
  - `lib/bytes`.
- **Built with `ir/llvm`**: the attributes of item 9; the handles of item 10, replacing `ptr`; the
  allocation by place (`alloca` or the heap).
- **With 6d**: P4's protocol, P5's message declarations, record layout as codecs.
- **Each rule's planted fault, before it is trusted**:
  - a borrowed view moved out;
  - two calls merged (generativity);
  - a moved result used twice;
  - two segments with overlapping ranges lent together;
  - an `out` set with no supremum length;
  - a `get` on a partial codec;
  - a `dereferenceable` larger than the proven length;
  - `noalias` on a buffer passed twice;
  - a host declaration claiming `readonly` where SAL says `_Inout_`.
- **Commits the language** to every C and C++ pointer idiom having a form (Part II §14), with the classic
  memory bugs excluded by rule (§15). Where a program's correctness is beyond the static rules, the price
  is a run-time check, never undefined behaviour.
