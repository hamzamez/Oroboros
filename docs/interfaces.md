# Interfaces

Research, 2026-09-09. No decision. On hamza's *"that go has interfaces does not
mean we have to, it just means: can we express the api? can we use it? what are
the semantics in oroboros?"*

That framing is the whole document. The question is never *should this language
have interfaces* — it is **what does an `io.Reader`-typed parameter MEAN to a
program we compile**, and the answer turns out to be three different things with
three different prices, only one of which is expensive.

> **AN INTERFACE IS AN EXISTENTIAL TYPE**, `∃X. X × Πᵢ(X → Tᵢ)` — a hidden
> representation packed with the operations that consume it (Mitchell & Plotkin
> 1988). **PACKING one is manufacturing a closure**, callbacks.md tier 3, and
> refused. **UNPACKING one we were handed is nothing at all**: it is an opaque
> token with methods, which is what a Win32 `HANDLE` has always been.
>
> **AND PASSING ONE IS NEITHER.** `io.Copy(dst, src)` does not ask us to build an
> `io.Reader`; it asks us to hand over a `*os.File`, and **Go's own compiler
> inserts that coercion**. What is missing is not a value, a runtime or a
> representation — it is **one declared edge** telling our type checker that the
> host accepts it. `⟦coerce⟧ = id`: it costs **zero emitted characters**.
>
> **AND THE MEASUREMENT REFUTES THE PLAN.** Of 2,135 declarable-and-unusable Go
> names, **47** are blocked only by an interface we could supply, 87 by one we
> could not, and **1,867 by something that is not an interface at all**.
> Interfaces are not where the gap is. maxlen-2026-08-28's discipline —
> classify every blocked name by the fact that would settle it, before building
> anything — killing a second feature.
>
> **What the measurement found instead is that the survey was wrong twice**, in
> opposite directions, and the larger error was ours in our own favour by
> omission: a result whose type is obtainable **can** be read, and calling it
> unreadable cost **15.2 points**. Usable is **44.5%**, not the 31.7% published
> on 2026-09-06 nor the 27.5% published this morning.

---

## 1. What an interface is, precisely

```go
type Reader interface { Read(p []byte) (n int, err error) }
```

Denotationally,

```
    Reader  ≅  ∃X. X × (X → []byte → (int × error))
```

a **existential type**: a representation `X` nobody outside may name, packed with
the operations that consume it. That is Mitchell & Plotkin, *Abstract Types Have
Existential Type* (TOPLAS 1988), and it is the standard reading.

Generalising to n methods,

```
    I  ≅  ∃X. X × Π_{m ∈ methods(I)} (X → Args(m) → Res(m))
```

— an existential over a **product of functions**, which is a vtable. Our product
is built (products.md); our function values are not (closures-direction.md). So
the shape decomposes into two things this project has already decided about, and
the decisions point in opposite directions.

**Reynolds 1975 and Cook 2009 are the reason the two directions differ.**
Reynolds distinguished *user-defined types* (an abstract type with operations
that take the representation) from *procedural data structures* (a value that
**is** its operations); Cook's *On Understanding Data Abstraction, Revisited*
(OOPSLA 2009) makes the point sharply: **an ADT hides one representation behind
many operations; an OBJECT is a record of closures and hides nothing else.** Go's
interface is the object side — at runtime an `iface` is a two-word pair, a
method-table pointer and a data pointer.

That pairing is exactly where our three directions come from.

## 2. Three directions, and only one of them is expensive

| | what it is | what it costs us |
|---|---|---|
| **(a) hold one** | an interface RESULT — `os.Stat` gives `fs.FileInfo` | **nothing.** An opaque host token with methods, which `HANDLE` has been since the windows target existed |
| **(b) pass one** | an interface ARGUMENT, and we hold a concrete type that satisfies it | **one declared edge.** The host inserts the coercion; we emit nothing |
| **(c) make one** | an interface ARGUMENT, and nothing we can obtain satisfies it | **tier 3**, and refused |

**(a) needs no argument.** An `fs.FileInfo` we cannot inspect except through
`.Size()` and `.Name()` is precisely an opaque type whose methods are prims with
the receiver as argument 0 — the convention win32 used for `HANDLE` and
gomethods-2026-09-09 generalised to every Go method. There is nothing to add and
nothing to decide: it works today.

**(c) is the one that looks like the question and is not.** Manufacturing an
`io.Reader` means shipping `∃X. X × Π(X → …)` — a data pointer and a table of
function pointers — which is a **runtime**: an indirect call and a heap
environment on x86, against requirement 6, and closures-direction.md's ranked
refusal. It also gives Landin's knot, which kills size-change termination. Every
reason callbacks.md gives for refusing tier 3 applies unchanged.

**(b) is the interesting one, and it is not a value question at all.**

## 3. Passing is a coercion, and its denotation is the identity

```go
f, _ := os.Open(path)   // *os.File
io.Copy(dst, f)         // io.Reader wanted
```

Go inserts the conversion. The emitted text is `io.Copy(dst, f)` — **no syntax at
all**. The `itab` is built by the Go compiler (statically where both types are
known; Cox, *Go Data Structures: Interfaces*, 2009), and nothing about it reaches
us.

So the missing thing is a fact our **type checker** needs and our **backend**
does not:

```
    ⟦coerce_{T→I}⟧ = id                    at emission
    T ≤ I                                  in the checker
```

`compatible` compares declared type names and refuses `ptr-os-File` where
`io-Reader` is wanted. That refusal is the entire cost.

### 3.1 And the order is the one subtyping we can afford

type-algebra.md refuses subtyping and keeps *"the decidable half"* —
`{i | 0 ≤ i < n} ⊆ int`, bounded subtyping decided in QF-LIA — and refuses to
generalise because that is **Pierce's undecidable F<:** (*Bounded Quantification
is Undecidable*, 1994).

**F<:'s undecidability is about BOUNDED QUANTIFICATION**, `∀X <: T. …`. After
staging there is nothing quantified left: decidability-map.md's own observation
is that the residual is *monomorphic, first-order and closed*, which is why we
never pay Hindley–Milner's DEXPTIME either. What is wanted here is a relation on
**ground type names**, and it has the shape of a lattice:

```
    T ≤ I   iff   methods(I) ⊆ methods(T)
```

Interfaces ordered by **method-set inclusion**, which is the powerset lattice
under reverse inclusion, with `interface{}` at the top and Go's embedding as set
union. Concrete types sit below every interface their method set covers. It is
Cardelli's record subtyping (*A Semantics of Multiple Inheritance*, 1984) with
the records being method sets.

**Two ways to have it, and they differ in exactly one property.**

- **DERIVE it** — compute method sets from our declarations and compare. Faithful
  and free of data, but method signatures can mention interfaces, so the relation
  is recursive and needs coinduction (Amadio & Cardelli, *Subtyping Recursive
  Types*, TOPLAS 1993). It is also fragile against us: our `error` is opaque and
  our `int` is a range, so two signatures that Go calls equal need not be equal
  after `spell`.
- **DECLARE it** — `(implements ptr-os-File io-Reader)` in a target file, a
  finite set of edges closed transitively at load, checked by lookup. **O(1), no
  inference, no variance, no fixed point.**

Declaring is this project's own shape — a target says what its host does, and the
compiler learns nothing — and it makes the recursion question disappear rather
than solving it.

### 3.2 The claim is checkable BY THE HOST, which is unusual and worth taking

A `prim` template is a claim nobody can check but the host compiler, and only
when a program happens to call it. An `implements` edge is different:

```go
var _ io.Reader = (*os.File)(nil)
```

**One line per edge, and `go build` decides.** A generator can emit the whole
relation as a Go file and the host will refuse it if a single edge is wrong.
That is a stronger conformance story than anything in `targets/` has today, and
it costs a file.

## 4. The measurement, which refutes the plan

maxlen-2026-08-28 classified every unproven operation in the corpus by the fact
that would settle it, found that **not one needed an octagon**, and killed the
feature before it was built. Same discipline, and the classification is part of what
`gauntlet/stdlib/survey.go` now prints:

**189 interface types in Go's manifest; 67 of them are satisfied by a concrete
type a program can already obtain.**

Of the declarable names that are not usable:

| | |
|---:|---|
| **47** | every blocker is an interface **we hold an implementation of** — reachable by a declared edge alone |
| **87** | every blocker is an interface **we hold nothing for** — genuinely tier 3 |
| **1,867** | at least one blocker **is not an interface** |
| **134** | nothing blocks an argument; the result is unread |

**The interface question is 134 names of 2,135 — about 6%, and 47 of them are the
part a coercion would fix.** Against 1,867 blocked by something else entirely:
struct arguments built by literal, and types no declarable function returns.

The interfaces most asked for in the remaining blocked positions:

| interface | positions | what we could hand it |
|---|---:|---|
| `types.Type` | 98 | nothing |
| `context.Context` | 39 | nothing |
| `reflect.Type` | 38 | nothing |
| `http.ResponseWriter` | 38 | 1 type |
| `color.Color` | 28 | nothing |
| `io.Reader` | 19 | 16 types |
| `io.ReaderAt` | 16 | 3 types |
| `io.Writer` | 14 | 18 types |

**The head of that list is not an I/O interface.** `types.Type`, `reflect.Type`
and `constant.Value` are the compiler-introspection packages, where the interface
is the *whole* API and no concrete implementation is exported. `context.Context`
has one canonical constructor we cannot reach. Those are not coercion problems.

> **So: build the coercion because it is nearly free, not because it moves the
> number.** 47 names is about 1% of the declarable surface. Anyone proposing this
> as the way to raise usability should be shown this table.

## 5. What the measurement found instead — the survey was wrong twice

Both errors were in `qual`, the function that keys the obtainable-type fixed
point, and neither would have been found without asking a different question.

**First, and the smaller one, already recorded**: keyed by the bare type name,
`*File` in `os` and `*File` in `archive/zip` were one entry
(gomethods-2026-09-09). That **inflated**.

**Second: a predeclared type belongs to no package.** `[]uint8` qualified against
`os` is `[]os.uint8`, and `error` became `os.error`, `io.error`, one per package —
so an `error` obtained in one package stopped satisfying an `error` argument in
another. That **deflated**, and it was mine, published this morning.

**Third, and much the largest: a result whose type is OBTAINABLE can be read.**
`judge` called any opaque result unreadable, so `os.Open` scored **unusable** —
while `gauntlet/stdlib/acceptance/os-methods.oro` opens a file with it, reads
through that type's methods and prints 64. *Having methods is the whole of what
reading an opaque value means here.* The circularity is well-founded: the fixed
point decides obtainability without consulting usability.

| | usable | note |
|---|---:|---|
| published 2026-09-06 | 31.7% | inflated — cross-package name collisions |
| published this morning | 27.5% | over-corrected — `error` split per package |
| `qual` fixed | 29.3% | |
| **an obtainable result is readable** | **44.5%** | and `cannot read the result` falls 18.7% → **3.1%** |

Per package, against the numbers this repository has been quoting:

| package | declarable | usable (was) |
|---|---:|---:|
| `os` | 130 | **96** (76) |
| `io` | 28 | **17** (6) |
| `time` | 81 | **70** (43) |
| `net/http` | 130 | **68** (32) |
| `context` | 10 | **10** (1) |
| `encoding/json` | 46 | **26** (7) |

> **A survey written by the same hands as the language describes the measurer,
> and this is the fifth instance** — after methods-at-0%, the Win32 typedef
> residue, Win32's void-returning functions, and this morning's collisions. Four
> deflated and one inflated. **The one that mattered most was an omission in our
> own favour's opposite direction**: we had been reporting the language as less
> capable than a running program in this repository proves it to be.

## 6. The semantics, stated

If the coercion is built, this is what a program may assume — and it is
deliberately thin.

**An interface value is an opaque host token.** It has no Oroboros semantics
beyond being a value of a declared type. It cannot be constructed, compared,
inspected, or asked what it is. The only operations are:

1. **obtain** one — from a host call that returns it, or by coercion from a
   concrete type;
2. **pass** one — to a host call that wants it, at the same type or a supertype;
3. **call a method on it** — an ordinary prim with the receiver as argument 0.

**`T ≤ I` is declared, ground, and closed transitively.** Reflexive by
`compatible`'s existing equality. It is not inferred, it is not variant, and it
does not appear in any program's source.

**Coercion is the identity at emission.** Nothing is allocated by us; where a
host builds an `iface` header, that is the host's business, exactly as boxing a
`Long` is on the JVM.

**Refused, and each for a reason already written down:**

- **manufacturing** an interface — tier 3, and it is a runtime;
- **a type assertion** `x.(*T)` — that is a *sum with a runtime tag*, which is
  sums.md's niche-encoding question and needs a discriminator no host exposes to
  us portably;
- **comparing** two interface values — Go's `==` on interfaces panics on
  uncomparable dynamic types, which is a divergence inside the portable window;
- **deriving** the relation from our own declarations — §3.1.

**On other targets it is not a new idea either.** A Java parameter typed
`CharSequence` accepting a `String`, and a TypeScript structural type, are the
same edge; the JVM's is nominal and declared by the host, which is if anything
easier. So `implements` is not a Go-shaped wart — but nothing should be built for
Java before the JVM has been surveyed at all.

## 7. Recommendation

**Build the coercion, and expect it to buy 47 names.** It is one declaration
form, one lookup in `compatible`, zero backend change, and a conformance story
the host compiler checks. That is cheap enough to be worth 1%, and the
`os.Open` → `io.Copy` shape is one a real program hits immediately.

**Do not build anything for (c).** 87 names, and every argument against tier 3
still holds.

**And the next thing is not interfaces at all.** 1,867 blocked names have a
non-interface blocker, and that is where *cannot build the argument* actually
lives: **struct arguments built by literal**, and types no declarable function
returns. Whether a struct literal is expressible — `(array …)` is a product and
products.md §7 defers the layout that a `GUID` and an `http.Client{}` both need —
is the question this measurement points at, and it is the same layout question
Win32's struct-by-value 13.9% points at from the other side.

**Two questions convergng on one missing thing is the best evidence this
repository has ever used** that the thing is the right one to build.
