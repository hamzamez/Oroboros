# What a host call does to a buffer

Research, 2026-09-13. **DECIDED 2026-09-14, and built** —
[handdecl-2026-09-14](../gauntlet/results/handdecl-2026-09-14.md). D1 is done: linearity is seeded by
type, and building it closed a second hole — an immutable array handed to a parameter declared a
buffer. And the question behind D2 was settled more broadly than §7 put it: **host declarations are
written by hand**, carrying what a call does to a buffer, what it requires and what it returns, with
the generator kept as the checker of the mechanical half. `unicode/utf8` is the first. D3 is not
taken. This document is kept as the reading the decision rests on.

The wall
[gostd-utf8-2026-09-13](../gauntlet/results/gostd-utf8-2026-09-13.md) §4 stopped at, on hamza's
*"go ahead with the research, be mathematical and algebraic, and always reach for the literature."*
Experiments are in §2 and are reproducible from the programs quoted there.

> **Five findings, in the order they change what should be decided.**
>
> 1. **There is a soundness hole today, and it is not the one the wall named.** `CheckLinear` is
>    seeded by *binders* — a `build`'s parameter and a declared `(buffer V)` parameter — and a
>    buffer can also come from a **primitive's result**. Such a buffer is never checked: used twice,
>    or read after it was handed on, the program is accepted and prints a wrong answer. The control,
>    the identical shape on a seeded name, is refused. **Linearity is a property of the TYPE and the
>    check is keyed by SYNTAX**; the two agreed only while no primitive returned a buffer.
> 2. **The aliasing is not new and not the generator's.** `targets/go/builtin.oro` has declared
>    `append-int` on an ordinary shared value since August, and two appends to one value alias
>    exactly as `AppendRune` does: `x` prints `[1 2 3 5]` where the language says `[1 2 3 4]`.
> 3. **The write-borrow needs no new construct.** `B ⊸ B ⊗ R` — take the buffer, hand it back, plus
>    a result — is declarable in the target format today, with the identity written in the template,
>    and it runs: `3` and `[230 151 165 0]`, what Go prints. It is Linear Haskell's array API and
>    RustHorn's reading of `&mut`.
> 4. **What a host does to storage is exactly Aspinall and Hofmann's three USAGE ASPECTS** (ESOP
>    2002): destroyed, read and shared with the result, read and not shared. Each has one typing,
>    and the three typings are sound together under one invariant, proved in §4.
> 5. **Which aspect a host function has is not in any manifest**, so a generator that declares it is
>    making a claim. For Go it can be READ from the standard library's source; the census is §6.

---

## 1. The question, stated as algebra

A table value on a host with slices is three things, not two. A Go slice is `(ptr, len, cap)`, and
it gives two regions of memory:

```
    ρ(v) = [ptr, ptr + len)          the EXTENT: what the value denotes
    κ(v) = [ptr + len, ptr + cap)    the CAPACITY: storage it may write without being asked
```

ADR 0018 speaks only of `ρ`. Its invariant, written out, is:

> **(I)** For any two live references `u ≠ w`: if either is a buffer, `ρ(u) ∩ ρ(w) = ∅`; and no
> operation writes into `ρ(a)` for an array `a`.

A host call is a function whose semantics carries a **write set** `W ⊆ memory` and a **result
region**. For a slice parameter `p` there are exactly three possibilities worth distinguishing — and
they are not ours; they are the three **usage aspects** of Aspinall and Hofmann, *"Another type
system for in-place update"* (ESOP 2002), extended with type reconstruction by Aspinall, Hofmann and
Konečný (JFP 18(2), 2008):

| aspect | host behaviour on `p` | formally | Go instance |
|---|---|---|---|
| **3** read-only, not shared | reads `ρ(p)`, result does not alias it | `W ∩ (ρ(p) ∪ κ(p)) = ∅`, `ρ(r) ∩ ρ(p) = ∅` | `utf8.Valid`, `utf8.RuneCount` |
| **2** read-only, shared with the result | reads, and the result may point into `ρ(p)` | `W ∩ (ρ(p) ∪ κ(p)) = ∅` | `bytes.TrimSpace` |
| **1** destructive | may write `ρ(p)` or `κ(p)` | `W ∩ (ρ(p) ∪ κ(p)) ≠ ∅` possible | `utf8.EncodeRune`, `utf8.AppendRune` |

Aspect 2 was Aspinall and Hofmann's contribution over plain linearity: a linear value may be read
*and even aliased with a result* without being consumed, because nothing writes. Konečný (TLCA 2003)
gives the aspects their reading as heap pre- and post-conditions, which is the reading used here.

**Aspect 1 splits in two by what comes back**, and the split is the difference between the two
witnesses the wall found:

- **write-borrow** — the host writes `ρ(p)` and returns something else: `EncodeRune(p, r) int`.
- **consume-and-return** — the host may write `κ(p)`, possibly allocate, and returns the storage:
  `AppendRune(p, r) []byte`, whose result is `p` extended when `cap` allows and a copy when not.

And there is a fourth shape that no aspect names because it is not about one call: **ownership
transfer**, where the host *keeps* `p` — `bytes.NewBuffer(buf)` documents that the new Buffer takes
ownership of `buf`. It is aspect 1 with no return.

## 2. Measured, before anything was argued

Every program below was built with `cmd/build -checked -target=go` against a scratch target layer and
run. The layer declares two primitives on the buffer type, which the format already accepts:

```lisp
(prim EncodeInto ((p (buffer (int 0 255))) (r (int -2147483648 2147483647)))
      ((buffer (int 0 255)) int)
      expr "func(p []byte, r int32) ([]byte, int) { return p, utf8.EncodeRune(p, r) }(%s, int32(%s))")
(prim AppendInto ((p (buffer (int 0 255))) (r (int -2147483648 2147483647)))
      (buffer (int 0 255))
      expr "utf8.AppendRune(%s, int32(%s))")
```

| | program | predicted | observed |
|---|---|---|---|
| **A** | `((EncodeInto b 26085) (fn (b2 n) (seq (print n) b2)))` in `build 4` | builds, `3`, `[230 151 165 0]` | **as predicted** |
| **D** | the same, reading `(b 0)` after handing `b` on — the CONTROL | refused | **refused**: *"the buffer b is read after it has already been handed on"* |
| **E** | the same one step later: `b2` handed to a second call, then `(b2 0)` read | accepted — wrong | **accepted, prints `195`**: the second write seen through a dead name |
| **B** | `base` from `AppendInto`, appended to twice | accepted — wrong | **accepted, `x` = `[104 195 169 240 159 153]`** where `日` was appended |
| **F** | hand-written `go.append-int` on `slice-int`, base3 appended to twice | aliasing | **`x` = `[1 2 3 5]`**, where immutability says `[1 2 3 4]` |

**A** is the good news: the write-borrow is expressible and the emitted Go is what anyone would write.
**D against E** is the hole: identical shapes, one refused and one accepted, differing only in whether
the buffer's name was bound by `build` or by a continuation receiving a primitive's result.
**B** is the same hole reached through `AppendRune`. **F** says the aliasing predates this week: an
opaque type is shared freely by construction, and the comment beside the declaration — *"append may
write into the argument's backing array, so it is NOT pure"* — names the hazard and addresses it
with the wrong tool. Impurity sequences effects (ADR 0010); it does nothing about aliases.

## 3. The literature, organised by the shape it answers

**The write-borrow is the pair.** Linear Haskell (Bernardy, Boespflug, Newton, Peyton Jones,
Spiwack, POPL 2018) gives mutable arrays an API in which every operation takes the array linearly and
hands it back: in the `linear-base` library, `unsafeGet :: Int -> Array a %1-> (Ur a, Array a)`,
the extracted value wrapped in `Ur` (the `!` of linear logic) because it may be duplicated while the
array may not. That is `B ⊸ Ur R ⊗ B` — the write-borrow's type, with Wadler's reason (*"Linear types
can change the world!"*, 1990): the world is threaded, so an update in place is indistinguishable
from a functional one.

**And a mutable borrow reduces to the pair.** Matsushita, Tsukada and Kobayashi's RustHorn (ESOP
2020; TOPLAS 2021) models a Rust `&mut T` as a pair of its *current* and *final* values, the final
one a prophecy resolved when the borrow ends. For a borrow that ends at the call's return — which is
every borrow in a first-order residual — the prophecy is resolved immediately and the pair *is*
`B ⊸ B ⊗ R`. Rust's own standard library says so in a signature:
`char::encode_utf8(self, dst: &mut [u8]) -> &mut str` hands back the part of the buffer it wrote.
Matsushita and Ishii's *Pure Borrow* (PLDI 2026) realises Rust-style borrowing inside Linear Haskell,
which is the same equivalence built rather than modelled. **So borrowing is not a second mechanism
here**: ADR 0020 already refused lifetimes (Why-not (e)), and nothing in this document reopens that,
because a borrow that cannot outlive its call needs no lifetime.

**The alias rule for results is Futhark's.** Futhark (Henriksen, Serup, Elsman, Henglein, Oancea,
PLDI 2017) states it in its language reference: a returned value declared alias-free has no aliases;
*otherwise it aliases all arguments passed for non-consumed parameters*. That is aspect 2 made
conservative: a non-unique result is assumed to share with every observed argument, which is sound
precisely because observed arguments are not written.

**Uniqueness is the caller's half.** Barendsen and Smetsers (*"Uniqueness typing for functional
languages with graph rewriting semantics"*, MSCS 6, 1996) define it on the reference count, and
Marshall, Vollmer and Orchard (ESOP 2022) separate it from linearity; uniqueness.md §3 already
carries both. What is new here is that a **host** result can be a buffer, so uniqueness is also a
promise the *target declaration* makes about what the host returns — `AppendRune`'s result is unique
exactly because its argument was consumed.

**The capacity is a capability.** L3 (Ahmed, Fluet, Morrisett; TLCA 2005, Fundamenta Informaticae
2007) separates a pointer from the linear *capability* to write through it, which is what lets it
allow aliased pointers and strong updates together. A Go slice carries an unstated capability over
`κ(v)`, and two values made from one base share it — which is the entire mechanism of witness B. The
host's own tool for revoking it is the **full slice expression**: *"`a[low : high : max]` … sets the
resulting slice's capacity to `max - low`"* (Go specification), and with `cap = len` the specification
of `append` leaves only one branch: *"If the capacity of s is not large enough … append allocates a
new, sufficiently large underlying array … Otherwise, append re-uses the underlying array."*

**Framing is what makes any of this local.** Reynolds' separation logic (LICS 2002) and Boyland's
fractional permissions (SAS 2003) are the general theories of *who may write where*. lowstar-lessons
already recorded that a linear buffer's `modifies` set is syntactically the buffer; §4 below is that
fact extended to host calls, and nothing needs fractions because nothing here is shared for writing.

**Deciding the aspect of existing code is reference-immutability inference.** ReIm and ReImInfer
(Huang, Milanova, Dietl, Ernst, OOPSLA 2012) infer, for Java, which references are never used to
modify what they point to and which methods are pure; Sălcianu and Rinard (VMCAI 2005) do purity and
side-effect analysis for Java. §6 is the same question asked of Go's standard library, answered with
far less machinery because the question is one slice parameter at a time.

## 4. The typing, and the theorem

Write `A` for `(array V)` — an immutable value, freely duplicated — and `B` for `(buffer V)` — linear.
A host parameter's declared type follows from its aspect:

| aspect | declared as | read |
|---|---|---|
| 3 | `A → R`, or a read of `B` that does not consume it | the observer; free by ADR 0020 rule 6 |
| 2 | `A → A` | **never `B`**: a result aliasing a live buffer would see its later writes |
| 1, write-borrow | `B ⊸ B ⊗ R` | the template returns the argument it was given |
| 1, consume-and-return | `B ⊸ B` | the result may be the same storage or new storage |
| ownership transfer | `B ⊸ H` | the buffer is gone; `H` is the host's handle |

Three facts about the machine are needed, each checkable against the specification rather than
assumed:

- **(M1)** `build n` emits `make([]T, n)`, whose capacity is `n`, so a buffer is born with `κ = ∅`.
- **(M2)** The freeze copies nothing (ADR 0018), so a frozen array may carry the capacity of the
  buffer it was.
- **(M3)** Every table the program did not build arrives through a declaration, whose type is the
  claim.

> **Theorem.** Suppose every host parameter that may be written (aspect 1) is declared at `B`, every
> aspect-2 parameter at `A`, and every term of type `B` is used linearly. Then invariant (I) holds
> after every host call.
>
> *Proof.* Take a call writing `W`. By the first hypothesis `W ⊆ ρ(p) ∪ κ(p)` for some buffer
> argument `p`. **`ρ(p)`**: by (I) before the call no other live reference overlaps it, and `p`
> itself is consumed by linearity, so the only reference that can observe the write is the one the
> call returns. **`κ(p)`**: a buffer's capacity is empty at birth (M1); a consume-and-return call may
> give its result non-empty capacity, but its argument is dead, so that capacity is reachable only
> through the result, which is itself a linear buffer. The only way capacity reaches an array is the
> freeze (M2), and an array is never an aspect-1 argument by the first hypothesis, so an array's
> capacity is never written. **Results**: an aspect-2 result aliases only arrays, which nothing
> writes; a buffer result is unaliased because its argument was consumed. ∎

**Every hypothesis is load-bearing, and each has a witness in §2.** Drop *every term of type `B` is
linear* — which is what the checker does today for a buffer a primitive returns — and E and B are
counterexamples. Drop *aspect 1 is declared at `B`* and F is one. Drop *aspect 2 is never `B`* and a
`TrimSpace` result would observe the buffer's next store — predicted, not yet run. The freeze
hypothesis (M2) is why the capacity argument has to go through the first hypothesis rather than be
avoided: if freezing clipped capacity it would be unnecessary, and it does not.

## 5. The hole in `CheckLinear`, precisely

`emit/linearity.go` walks the residual once per **seed**: each `build`'s lambda parameter, and each
signature parameter whose type is `(buffer V)`. A name is checked if and only if it is a seed. The
theorem's third hypothesis is about *every term of type `B`*, and there are now three producers of
such terms:

| producer | seeded today |
|---|---|
| `build`'s binder | yes |
| a declared `(buffer V)` parameter (ADR 0020) | yes |
| a binder receiving a primitive's `(buffer V)` result | **no** |

**The gap is a syntactic approximation of a semantic property that was exact until a primitive
could return a buffer.** Nothing in the corpus did, which is why it was invisible, and every target
file in the repository still declares none. The natural repair is to seed by type: the residual is
monomorphic and `emit.Check` runs before `CheckLinear` in `cmd/build`, so the type of every binder is
already known when the linearity walk starts. That is a statement about where the information is,
not a design; the design question is §7.

## 6. The census: which aspect does Go's standard library have?

The manifest says `func EncodeRune(p []byte, r rune) int` and nothing about writes. A generator
that declares an aspect must therefore get it from somewhere else, and for Go it can: the source ships
with the toolchain, exactly as `api/go1*.txt` does.

`hostbuf` (scratchpad, not committed) reads every exported function and method body in `GOROOT/src`
outside `internal`, `vendor`, `cmd` and tests, and classifies each slice parameter — interprocedurally
within a package, as a least fixed point over same-package function calls, and conservatively
everywhere else: a parameter reaching a method call, a function value, another package, an escape
into a variable or an assembly body is **unknown**, never read.

Go 1.27, 718 exported functions and methods with a slice parameter, 824 slice parameters:

| aspect, as read | parameters | of which `[]byte` | a result has the same type | functions whose worst parameter is this |
|---|---:|---:|---:|---:|
| **read** (aspect 3 or 2) | 85 (10.3%) | 57 | 13 | 61 |
| **write** (aspect 1, write-borrow) | 48 (5.8%) | 32 | 0 | 48 |
| **append** (aspect 1, consume-and-return) | 33 (4.0%) | 31 | 33 | 33 |
| **unknown** | 658 (79.9%) | 459 | 122 | 576 |

**Of the 166 parameters the reading decides, 81 — 49% — are destructive.** So aspect 1 is not an
edge case the wall happened to meet in `unicode/utf8`: among Go parameters whose behaviour can be
read at all, half write. And every `append` parameter has a result of its own type, which is
consume-and-return's signature; no `write` parameter does, which is the write-borrow's.

**Spot-checked against source outside the witness, and right wherever it decides**:
`hex.Encode(dst, src)` is `write, read`; `binary.PutUvarint` and `binary.Encode` write;
`strconv.AppendQuote` and `binary.AppendUvarint` append; `binary.Uvarint`, `bytes.Equal`,
`bytes.HasPrefix` read; `(*bytes.Buffer).Read` writes and `(*bytes.Buffer).Write` reads; and
`bytes.NewBuffer` — ownership transfer — is unknown, which is the conservative answer.

**The 79.9% is the reading's limit, not the library's**, and it has named causes. The walk skips
`internal/`, so everything `bytes` delegates to `internal/bytealg` is unknown (`bytes.Index`,
`bytes.TrimSpace`); a method call is not followed; and many of those end in assembly, which has no
Go body to read. **The column "a result has the same type" is a SHAPE, not an alias**: `bytes.Clone`
has it and copies. Deciding aspect 2 against aspect 3 needs the result's provenance, which this
reading does not track — so the 85 are "does not write", not yet split.

**The classifier was checked against answers read off the source by hand before its numbers were
used, and it failed three times on the way**, each time for a reason worth keeping:

1. **Its parent chain was built with `append`**, so siblings shared spare capacity and a node's
   ancestors were overwritten — `DecodeRune` and `Valid` were misclassified by *the exact bug this
   document studies*. Fixed with `parents[:len:len]`, the full slice expression.
2. **Its fixed point did not terminate**: two build-tagged variants of one function with different
   arities reset each other's summary on every pass.
3. **Its fixed point was not least**: a callee met before its own summary existed read as unknown,
   and since summaries only rise the answer stuck — `DecodeLastRune` was misclassified by it. Every
   summary now starts at bottom before any is computed.

A witness that cannot fail proves nothing; this one failed three times.

## 7. What is open, for decision

Not recommendations to build — the shape of the choices, each located.

**D1. Seed linearity by type.** Closes §5's hole whatever else is decided, and it is the only item
here that is a correctness repair to shipped mechanisms (ADR 0018, ADR 0020) rather than a new
capability. Witnesses B and E fail against the current tree and are the pass condition. It changes no
emitted file in the corpus, predicted from the fact that no target declares a buffer result — to be
measured, not assumed.

**D2. The generator declares aspects.** Aspect 1 at `B` with a pass-through result, aspect 2 and 3 at
`A`. The open part is **unknown**, and it is ADR 0010's question again: purity defaults to impure so
an omission costs speed rather than correctness. The corresponding default here is *unknown ⇒ B*,
which is sound and costs ergonomics: an unknown parameter cannot be handed a frozen array without a
copy. The alternative — refuse the name — costs coverage instead. §6's unknown share is the price of
either, and the census should be sharpened (following method calls on known receivers, for one)
before that price is quoted.

**D3. Clip capacity at the boundary instead.** Pass `a[:len(a):len(a)]` to an aspect-1 host call
taking an array, so `append` must copy. Sound for consume-and-return on every value, with no linear
threading — and it moves an allocation into every such call, which is the hidden-allocation shape the
project refuses in the core. It does nothing for a write-borrow, which writes `ρ`, not `κ`.

**D4. The hand-written targets.** `append-int`, `append-float64` and `append-string` on Go, and
`push` on JavaScript, are declared on freely shared types. F measures the Go instances. JavaScript
is **predicted** to be worse: an array there is a reference, so `push` writes into `ρ` itself and
every holder sees it — not measured.

**D5. The other hosts have the write-borrow too**, so this is not a Go idiom: JavaScript's
`TextEncoder.encodeInto(string, Uint8Array)` writes the array and returns `{read, written}`; the JVM's
`InputStream.read(byte[])` writes and returns a count; Win32's `WideCharToMultiByte` writes a caller's
buffer and returns a length. The consume-and-return shape through a hidden capacity is, among these
four, Go's.

**Deliberately not proposed**: lifetimes and borrow regions (a borrow that ends at the call needs no
lifetime, §3); fractional permissions (nothing is written through a shared reference); Clean-style
uniqueness attributes (ADR 0020 Why-not (c) stands unchanged).

## Sources

- D. Aspinall, M. Hofmann. *Another type system for in-place update.* ESOP 2002, LNCS 2305, 36–52.
- D. Aspinall, M. Hofmann, M. Konečný. *A type system with usage aspects.* JFP 18(2), 2008, 141–178.
- M. Konečný. *Functional in-place update with layered datatype sharing.* TLCA 2003.
- P. Wadler. *Linear types can change the world!* Programming Concepts and Methods, 1990.
- M. Odersky. *Observers for linear types.* ESOP 1992.
- E. Barendsen, S. Smetsers. *Uniqueness typing for functional languages with graph rewriting
  semantics.* MSCS 6, 1996, 579–612.
- D. Marshall, M. Vollmer, D. Orchard. *Linearity and uniqueness: an entente cordiale.* ESOP 2022.
- J.-P. Bernardy, M. Boespflug, R. Newton, S. Peyton Jones, A. Spiwack. *Linear Haskell: practical
  linearity in a higher-order polymorphic language.* POPL 2018; the `linear-base` library,
  `Data.Array.Mutable.Linear`.
- T. Henriksen, N. Serup, M. Elsman, F. Henglein, C. Oancea. *Futhark: purely functional
  GPU-programming with nested parallelism and in-place array updates.* PLDI 2017, 556–571; and the
  Futhark language reference on in-place updates and alias-free return types.
- Y. Matsushita, T. Tsukada, N. Kobayashi. *RustHorn: CHC-based verification for Rust programs.*
  ESOP 2020; TOPLAS 2021.
- Y. Matsushita, H. Ishii. *Pure Borrow: Linear Haskell meets Rust-style borrowing.* PLDI 2026
  (arXiv 2604.15290).
- A. Ahmed, M. Fluet, G. Morrisett. *L3: a linear language with locations.* TLCA 2005; Fundamenta
  Informaticae 77(4), 2007.
- J. C. Reynolds. *Separation logic: a logic for shared mutable data structures.* LICS 2002.
- J. Boyland. *Checking interference with fractional permissions.* SAS 2003.
- W. Huang, A. Milanova, W. Dietl, M. D. Ernst. *ReIm & ReImInfer: checking and inference of
  reference immutability and method purity.* OOPSLA 2012.
- A. Sălcianu, M. Rinard. *Purity and side effect analysis for Java programs.* VMCAI 2005.
- The Go Programming Language Specification: *Appending to and copying slices*; *Full slice
  expressions*.
- The Rust standard library: `char::encode_utf8`. MDN: `TextEncoder.encodeInto()`.
