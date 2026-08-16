# Modules

Written before the code, per [state.md §6](state.md).

> **Status, 2026-08-15.** §3 (names), §4 (targets declaring into modules), and §5 (resolution,
> the four cells, covering) are **built and tested**. The claim held: `core/reduce.go` gained
> **no reduction rule** — resolution runs before normalisation and hands the reducer one flat
> qualified namespace. All seven examples emit byte-identical output on all three targets.
> `cmd/gen` now names emitted functions **after the export they came from** — `GenGeneric0` and
> `GenGeneric1` became `GenSumOf` and `GenWordTally`. A file with no `(export …)` keeps the old
> stem-based naming, so nothing else moved.
>
> **Files, 2026-08-15.** `(use PATH)` now resolves against a search path — `PATH.oro` under the
> entry file's directory, then `-path` (default `lib`). A path with no file is not an error: it is
> a module the *target* provides. `lib/num/vec.oro` is the first library, and `examples/dot.oro`
> and `examples/report.oro` share it instead of duplicating it, with byte-identical output.
> Crucially, `P_T ∩ D` still decides **across the file boundary**.
>
> **One file, one module, 2026-08-16.** A library file declares exactly the module its path names.
> Extras were being loaded along with it, which made their visibility depend on **load order** —
> `(use sub/two)` failed on its own and succeeded once `(use sub/one)` had pulled in the file that
> declared both. Meaning that depends on what else is in scope is precisely what §3's qualified
> imports exist to prevent, so it was the same rule leaking through a different hole. The entry
> file is unaffected: it is not found by a path, so it may declare as many modules as it likes.
>
> Three diagnostics were also pointing at the wrong half of a name, all found by writing
> [chapter 3](../book/03-modules.md):
>
> - a member that is **private** and one that **does not exist** gave the same message;
> - an alias collision in the entry file said `module "" binds …`;
> - a **misspelled path** is indistinguishable from a target-provided module — both find no file —
>   and failed later, on the member. `Program.Unresolved` now carries which imports matched nothing,
>   so the error can say the path is the problem when the target provides no such module either.
>
> Not yet built: a target is still one file rather than a directory (§4).

## 0. Entry file and library file

Two roles, not two kinds — the same `.oro` file can be either, and which one it is in is decided
entirely by how it was reached.

> The **entry file** is the one named on the command line. A **library file** is one reached by
> following a `(use …)`, resolved as `PATH.oro` on the search path.

| | entry | library |
|---|---|---|
| `(module …)` | optional; may declare several | exactly one, matching the path it was found at |
| bare top-level terms | reduced | discarded |
| `(export …)` | the program's entry points | visibility only |
| `(export main)` | what `build` compiles | ignored |

The last row is the one with teeth: a dependency cannot supply a program's `main`, so adding one
cannot move where the program starts. The rest is what makes `(use …)` a dependency rather than an
inclusion — nothing crosses the boundary except names asked for by path.

Implemented as `entryPaths` in `LoadWith`, which is computed before imports are followed.

---

The claim this document has to earn is that **modules add no mechanism to the reducer**. If it
needs a new reduction rule, a new term kind, or a second parameter to normalisation, it is the
wrong design and should be thrown away. What follows is an argument that a module system is
already implied by the two rules we have, and that the work is naming, resolution and covering —
not reduction.

---

**Decision recorded as [ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md).**

## 1. Why now

Three times, the absence of a library mechanism pushed something into the core that did not
belong there:

| | went into | should have gone into |
|---|---|---|
| `seq` | the **reader** ([effects.md §5](effects.md)) | `std/control` |
| `print-line` | **every target file, separately** | `std/io`, once |
| the string literal | the **language** ([strings.md §1](strings.md)) | nowhere; target files needed it |

The pattern is mechanical, not accidental:

> **Without a library mechanism, "put it in a library" is not an available answer, so every
> pressure to grow lands on the language.**

That is the argument for doing this before anything else. A small core is not a discipline that
can be maintained by intention; it needs somewhere else for things to go.

Two smaller reasons. `cmd/gen` named emitted functions **by position** — `GenGeneric0`,
`GenGeneric1` — which is not a naming scheme; it now names them after their export. And the target files cannot hold ten thousand
names in one flat map, which is what the parasite thesis eventually asks of them.

## 2. The shape: signature and structure

Taken from ML, because the problem is the same one and the algebra already exists.

- A **signature** Σ names a set of exports and specifies each one's behaviour, independently of
  any target. `std/words` says what `split-words` *means*.
- A **structure** implements a signature. There may be many.
- A **target** implements *part* of a signature natively — whatever the host already has.
- A **library** supplies definitions for the rest, in terms of names further down.

The reason this is the right import is that ML's signature/structure split exists to answer
exactly the question this project keeps asking: **when may one implementation be substituted for
another without changing what a program means?** §5 answers it.

What we deliberately do **not** take is functors — parameterised modules. Our parameterisation is
the target, and it is already the parameter to reduction ([the-atom](../the-atom.md)). A functor
would be a second, competing parameterisation mechanism for the same job.

## 3. Names

`symbolChars` currently admits both `.` and `/` as identifier characters, and **neither appears in
any name** in `targets/` or `examples/`. So one of them can be reserved at zero cost.

- **`/` stays an ordinary identifier character.** A module path is one token: `std/words`,
  `go/strings`, `android/view`.
- **`.` becomes reserved** as the qualifier separator, and is no longer an identifier character.
  `words.split-words` is three tokens.

This closes [concerns.md §3.2](concerns.md), which predicted the collision and noted nothing
depended on it yet. Something does now.

```lisp
(module std/words
  (export split-words))

(use go/strings)              ; bound to `strings`, the last path segment
(use go/strings as s)         ; or explicitly

(def split-words (fn (t) (strings.fields t)))
```

**Imports stay qualified.** Every name from another module is written with its qualifier. The
alternative — flat import — makes a program's meaning depend on import order and on which names a
target happens to provide, and both of those change under exactly the conditions this system is
built to make cheap.

## 4. What a target declares

A target stops being one file and becomes a directory, in which each file says which names of one
module the host provides **natively**.

```lisp
; targets/go/std-words.oro
(provides go std/words
  (prim split-words (string) vec-string expr "strings.Fields(%s)" pure (import "strings")))
```

A target may provide **any subset** of a module's exports, including none. Nothing obliges a
target to implement a whole signature, and this is the mechanical form of the claim that porting
is cheap to start: a porter implements what the residual check asks for, and nothing else.

## 5. The algebra

Write:

- `N(M)` — the names exported by module `M`. Fully qualified: `std/words.split-words`.
- `P_T` — the qualified names target `T` provides natively. This is
  [ADR 0002](../decisions/0002-capability-graph.md)'s capability set, and the parameter to
  reduction.
- `D` — the qualified names defined by libraries in scope.
- `⟶_T` — reduction, halting on `P_T`.
- `⟦n⟧_T` — what `n` denotes on `T`. `⟦n⟧_Σ` — what its signature says it should denote.

**Resolution happens before reduction.** `words.split-words` under `(use std/words)` becomes the
qualified name `std/words.split-words`. So by the time the reducer runs, every name is fully
qualified and the maps `P_T` and `D` are keyed the same way.

That last sentence is the whole reason the design works, and it is worth stating as the rule it
is:

> **R1 — one namespace.** Targets and libraries name into the *same* qualified namespace. A
> target provides `std/words.split-words`; a library defines `std/words.split-words`; a program
> refers to `std/words.split-words`.

Without R1 the conditional lowering of §6 silently stops working, because the intersection below
would always be empty.

### The four cells

Every qualified name a program mentions falls into exactly one:

| | meaning | mechanism |
|---|---|---|
| `n ∈ P_T \ D` | host binding | reduction halts on `n` |
| `n ∈ D \ P_T` | portable definition | δ unfolds it |
| **`n ∈ P_T ∩ D`** | **the conditional** | **δ is inhibited; native wins** |
| `n ∉ P_T ∪ D` | not covered | the residual check reports it |

**R2 — native wins.** This is one clause already in `unfoldable`:
`if e.Prim[name] { return false }`. Nothing is added.

### Covering

A program `t` builds on `T` iff `Residual_T(nf_T(t)) = ∅`, where `Residual` reports free names
that are neither primitive nor recursive definitions — the second clause is now vestigial, since
ADR 0014 rejects a recursive definition before reduction. This is unchanged from today; only the keys
become qualified.

The porter's obligation is therefore **computed, not estimated**, and it is demand-driven: it
contains only names the program actually reaches.

## 6. Conditional lowering is already complete

An earlier draft of this argument claimed that N targets need N bodies and therefore a new form.
That was wrong. The four cells give N natives plus one fallback with no new mechanism:

```lisp
; std/words — the portable definition, used by any target with no native
(module std/words (export split-words))
(def split-words (fn (t) (fold-ws t)))       ; in terms of something lower

; targets/go/std-words.oro    — Go has it natively
(provides go std/words
  (prim split-words (string) vec-string expr "strings.Fields(%s)" pure (import "strings")))

; targets/js/std-words.oro    — JS has a DIFFERENT native
(provides js std/words
  (prim split-words (string) vec-string expr "%s.split(/\\s+/)" pure))

; targets/c/…                 — no native; δ unfolds the definition
```

Three targets, three outcomes, from `P_T ∩ D` and δ. This is the same mechanism as
`examples/dot.oro`, which has emitted a BLAS call on one target and a fused loop on another since
the first commit — read in the other direction.

## 7. Theorems

These are the reason to write the specification before the code: they say what may be assumed
later, and each one is short enough to check.

**T1 — reduction is target-independent except at the floor.**
Neither β nor δ mentions `T`. `T` enters normalisation only through `unfoldable`, which consults
`P_T`. Hence for any `t` and any `T₁, T₂`, the two normal forms differ only in *which subterms
were left unreduced*; no reduction performed under one is unavailable under the other.

*Proof.* Induction on the reduction sequence. Every step is β or δ. β's applicability depends only
on the term. δ's depends on `P_T` and on `Rec`. ∎

**T2 — substitution soundness.**
If every primitive appearing in `nf_{T₁}(t)` and `nf_{T₂}(t)` conforms — `⟦n⟧_T = ⟦n⟧_Σ` — then
`⟦nf_{T₁}(t)⟧ = ⟦nf_{T₂}(t)⟧`.

*Proof.* By T1 the two normal forms are reducts of the same term, so they are βδ-convertible modulo
the names each stopped at. Denotation is compositional over the six term kinds, so it is
determined by the denotations at the leaves. Conformance makes those equal. ∎

This is the theorem that says a standard library **is** portable rather than **is expected to
be**, and note precisely what it is conditional on. Not on the hosts agreeing — hosts never agree
— but on **our lowerings conforming to our own signature**. Nothing prevents `std/words` from
choosing a behaviour and implementing it on a host whose native idiom disagrees; that is what §6's
JS row does.

**T3 — portability is decidable.**
`t` is portable across a set of targets iff, for each `T`, every free name of `nf_T(t)` belongs to
a module carrying a signature. Computable by `Residual` plus a per-module lookup, in time linear
in the normal form.

This is [ADR 0001](../decisions/0001-parasite-model.md)'s claim — *portability is a property a
program may or may not have, computed by the compiler* — becoming an algorithm rather than a
position.

**T4 — native adoption is meaning-preserving.**
If `T` moves a name from `D` to `P_T` — the host gained it, or someone wrote a binding — and the
native conforms, then no program's meaning changes.

*Proof.* The only change is that δ is inhibited at that name (R2). By T2, done. ∎

T4 is the property that lets `P_T` grow to ten thousand names without re-auditing a single
program, and it is what makes the parasite thesis safe to scale. Its one precondition is
conformance. That is the whole argument for §8.

## 8. Conformance, and what it is not

A signature is a claim. Conformance is evidence for it.

> A module with a signature ships a **test suite**. It tests **our lowerings**, not the host.

A failure never means "this target cannot have this module" — it means our implementation for that
target is wrong. `strings.Fields` and `.split(" ")` disagreeing is not JavaScript's problem; it is
a bug in our JS lowering, and the fix is to write a different one, at a
[measured](../../gauntlet/results/) cost.

The reason the suite is not optional:

> **The covering check proves a name is *provided*. It cannot prove the name is *right*.**

A porter can satisfy `Residual = ∅` completely and still be wrong. We did: `split-words` was
declared on all three targets, passed every check, and gave different answers on Go and JS for any
text containing a tab, a newline, or a double space — **four of ten cases**. Fixed 2026-08-15, and
the suite now exists: [gauntlet/conformance/](../../gauntlet/conformance/). Covering is a type-level property;
conformance is a semantic one; T2 depends on the second.

### What a signature costs

Specifying `split-words` means answering *what is whitespace*, and the hosts disagree on U+00A0:

| | NBSP | U+2028 |
|---|---|---|
| Go `unicode.IsSpace` | splits | splits |
| JS `/\s+/` | splits | splits |
| Java `\s+` | **does not** | **does not** |
| Java `(?U)\s+` | splits | splits |

> **Correction, 2026-08-15.** An earlier version of this table said `(?U)\s+` also fails on these.
> It does not — `(?U)` handles Unicode whitespace correctly, and the claim came from one careless
> measurement. What `split` genuinely cannot do is suppress the empty field it produces for leading
> whitespace and for the empty string, which is why the lowering is a matcher over runs of
> non-whitespace rather than a split at all. See [primitives.md §3](primitives.md).

So adopting Go's answer obliges Java to carry a Unicode `White_Space` table. That is affordable —
and it is the reason the standard library should grow slowly. Not because implementations are
expensive: because

> **a signature is the only object in this system that cannot be revised per target.**

Every target file is data someone can rewrite. A signature is a promise to every program already
written.

## 9. Deliberately absent

- **Functors.** §2. The target is our parameterisation and there should be only one.
- **Separate compilation.** Reduction is whole-program by construction; fusion crosses every
  boundary a module would draw. Whether that is a scaling problem is a *measurement*, not a
  design question, and it has not been taken.
- **Versioning, cyclic imports, visibility beyond export/not-export.** No program needs them yet
  ([ADR 0007](../decisions/0007-exploration-over-specification.md)).
- **Overload resolution.** `print-line` needed `any` because primitive names are unique keys
  ([effects-2026-08-14 §6](../../gauntlet/results/effects-2026-08-14.md)). Qualification does not
  fix this — `java/io.println` still has ten signatures. It is the first thing generated target
  files will break on, and it is not solved here.

## 10. Open questions this raises

1. **Purity defaults for generated target files.** [effects.md §3](effects.md) chose *impure by
   default* so that a human's omission costs speed rather than correctness. A machine-generated
   ten-thousand-name target file is then entirely impure, and fusion dies everywhere. No host
   publishes purity as metadata. Unresolved, and it does not bite until §9's last bullet does.
2. **Does whole-program reduction scale?** See §9. Measure before deciding.
3. **What is the first signature?** It should be `std/words`, because that is the name we already
   got wrong, and specifying it will exercise every part of this document.
