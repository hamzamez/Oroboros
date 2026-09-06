# The target system

2026-09-06. A specification of what a target IS, as opposed to
[target-files.md](target-files.md), which specifies the file one is written in.

It exists because four questions turned out to have one answer between them, and
that answer is an algebra the code is already most of the way to implementing —
`loadTargetDir`'s merge is a sheaf gluing, and `unfoldable`'s native-wins clause
is an override. Naming them separates two operations that are currently the same
word.

> **Status.** §1–§4 describe what is BUILT, with one gap named at each point.
> §5–§8 are PROPOSED and nothing in them is implemented. §9's theorems hold of
> the built part. Priced against a real ecosystem in
> [gostdlib-2026-09-06](../../gauntlet/results/gostdlib-2026-09-06.md).

---

## 1. The objects

**Names.** `N` is the set of fully qualified names. There is exactly one such
set: targets and libraries name into it identically, which is
[modules.md](modules.md)'s **R1** and the reason everything below composes.

**Declarations.** `Decl` is what a target says about one name: an arity, argument
and result types, a kind, a template, purity, imports, and contracts.

**Backends.** `𝔅 = {go, js, java, x86-64}` is a **finite closed set**, and it is
compiler code rather than data. A backend is the thing that emits control flow
and binds variables, which is why [target-files.md §8](target-files.md) refuses
new structural kinds: a template cannot express a binder.

**A target** is a pair

```
    T = (B, Δ)        B ∈ 𝔅          Δ : N ⇀ Decl
```

and `P_T = dom Δ` is the capability set —
[ADR 0002](../decisions/0002-capability-graph.md)'s parameter to reduction.

This pair is the first thing the current implementation does not quite have.
`LoadTarget` returns a `*Target` whose backend is chosen later, by name, from the
target's own `Name` field. The pair is implicit, and §5 needs it explicit.

## 2. Two operations that are not the same operation

Everything that composes targets is one of exactly two things, and the current
vocabulary calls both "merge".

### 2.1 Gluing — within a layer

```
    (φ ⊔ ψ)(n) = φ(n)              n ∈ dom φ
               = ψ(n)              n ∈ dom ψ
    defined iff  ∀ n ∈ dom φ ∩ dom ψ .  φ(n) = ψ(n)
```

`(Φ, ⊔, ∅)` is a **partial commutative idempotent monoid**. Partial because
disagreement is an error, not a resolution; commutative and idempotent because
agreement is symmetric.

Equivalently and more usefully: **declarations form a sheaf over `N`.** A fragment
is a section over a subset of `N`, and fragments glue exactly when they agree on
their overlap. `loadTargetDir` is this construction restricted to one directory,
and its comment — *"deterministic diagnostics; merging is order-independent"* — is
the commutativity, already observed and not named.

### 2.2 Overriding — between layers

```
    (φ ▷ ψ)(n) = φ(n)              n ∈ dom φ
               = ψ(n)              otherwise
```

Associative, **not** commutative, and it *permits* disagreement — that is its
entire purpose. `unfoldable`'s `if e.Prim[name] { return false }` is `▷` with the
native on the left, which modules.md calls **R2, native wins**.

> **The distinction is the specification.** Gluing requires agreement and ignores
> order; overriding requires order and licenses disagreement. A system that uses
> one word for both is a system where adding a file can silently change a
> program and nobody can say from the rules whether it should have.

## 3. A declaration splits into an interface and an implementation

```
    Decl  ≅  Σ × I
```

- `Σ` — arity, argument and result types, purity, `where`, `ensures`, `length`,
  `index`. **What the name means.**
- `I` — kind, template, imports, `data`, `jump`, `checked`. **How this host does
  it.**

Write `π : Decl → Σ` for the projection. The language already makes exactly this
split one level up, between `sig` and `def`, and
[modules.md §8](modules.md) already calls the obligation that they agree
*conformance*. §5 is that observation applied to targets.

**Gap.** The format has no syntax for the two halves separately, so a family that
shares `Σ` must repeat it once per member.

## 4. Covering is unchanged

A program `t` builds on `T` iff `Residual_T(nf_T(t)) = ∅`. Everything above
changes where `Δ` comes from and nothing about what covering means.

---

## 5. Question 2 — target families

*"JavaScript is in the browser, the backend, node, bun, vscode extensions. The JS
core is the same, the API is different. Same for windows: x64, x86, ARM — same
API, different assembly."*

These are **two different factorisations**, and conflating them is why one
mechanism will not serve.

### 5.1 The JavaScript family varies Σ and fixes B

```
    node     = (js, js-core ⊔ node-api)
    browser  = (js, js-core ⊔ dom ⊔ fetch)
    bun      = (js, js-core ⊔ bun-api)
    vscode   = (js, js-core ⊔ node-api ⊔ vscode-api)
```

One backend, and the members differ by **gluing** — §2.1, already the right
operation, already implemented, and reachable only within a single directory.
A `js-core` fragment cannot today be shared by four directories.

### 5.2 The windows family fixes Σ and varies B and I

```
    windows-x64    = (x86-64, Σ_win32 ⋉ I_x64)
    windows-arm64  = (arm64,  Σ_win32 ⋉ I_arm64)
```

`VirtualAlloc` has one name, one argument list, one result type, one purity and
one contract on every Windows machine. What differs is the instruction sequence,
because `mov %r, %1` is x86 and not AArch64.

So this family is not a gluing at all. It is a **fibration**: the map
`Δ ↦ π ∘ Δ` has base `Σ_win32`, and each member is a choice of section over it.

### 5.3 The general statement

> **A target family over an interface `σ : N ⇀ Σ` is the set of targets
> `(B_i, Δ_i)` with `π ∘ Δ_i = σ`. The base is the API; the fibre is the
> implementation; and conformance is the assertion that every fibre denotes what
> the base says.**

Both examples are instances. JavaScript's members have different bases and a
shared backend; Windows's have a shared base and different backends. The two axes
are independent, which is why a design that offers only "inheritance" answers
half the question.

### 5.4 What it costs

**For §5.1, nothing but a path.** Gluing already works; fragments merely have to
be findable outside one directory. That is §7.

**For §5.2, one form.** Something that declares `Σ` once:

```lisp
(interface win32
  (prim VirtualAlloc ((n int)) ptr pure (where (< 0 n)) (ensures (<= 0 result))))

; targets/windows-x64/alloc.oro
(implements win32 x86-64
  (VirtualAlloc expr "mov rcx, %1\n…"))
```

The obligation is then checkable rather than promised: two members of a family
that disagree about an arity or a result type are a conflict the loader can
report, where today they are two unrelated files.

**Recommended and not built.** The measurement that should precede it is a second
Windows ISA, because a family with one member proves nothing — the same rule this
project applied to `int-repr` and `big-repr`.

---

## 6. Question 1 — Oroboros inside a target

*"Can we use Oroboros in the target file to express primitives? How much of the
language should we allow in?"*

### 6.1 It is already happening, in Go rather than in data

Three Oroboros libraries ship **inside the compiler**, embedded and spliced by
compiler code:

| file | what it is |
|---|---|
| `emit/bignum.oro` | the fixed-limb bignum — windows' arbitrary precision |
| `emit/bignum-host.oro` | the same operations over a host bignum |
| `emit/winmap.oro` | open addressing over `build` — windows' map |

Each exists because a host lacks a capability and the language can supply it.
Each is reached by a `//go:embed` and a Go pass. That is the exact shape this
project's own rule refuses:

> *"Primitives are declared in `targets/*.oro`, not in Go. If you find yourself
> adding a case to `emit/*.go` for a host function, that is the wrong place."*

So the answer to *can we* is: **we already do, three times, in the wrong place.**

### 6.2 The algebra: a third source

Let `D_T` be the definitions a target contributes, written in Oroboros. Then
implementation selection for a name is

```
    impl  =  P_T  ▷  D_T  ▷  D
```

— the **same `▷`** as §2.2, used once more. Reduction's behaviour follows
mechanically, extending modules.md's four cells to five:

| | meaning | mechanism |
|---|---|---|
| `n ∈ P_T` | host binding | reduction halts on `n` |
| `n ∈ D_T \ P_T` | **the target's own library** | **δ unfolds it** |
| `n ∈ D \ (P_T ∪ D_T)` | portable definition | δ unfolds it |
| `n ∈ P_T ∩ (D_T ∪ D)` | the conditional | δ inhibited; native wins |
| `n ∉ P_T ∪ D_T ∪ D` | not covered | the residual check reports it |

Nothing in the reducer changes: `unfoldable` already consults `P_T`, and `D_T` is
just more entries in the definition environment.

### 6.3 How much of the language — all of it, and the constraint is elsewhere

The question feels like it should be answered with a subset. It should not.

> **A target library is a library. The restriction is on its FREE NAMES, not on
> its constructs.**

Well-formedness, for every `d ∈ D_T`:

```
    FV(d)  ⊆  Lang  ∪  P_T  ∪  dom D_T  ∪  dom D
```

where `Lang` is what the compiler injects into every target — `if`, `let`,
`loop`, `the`, `=`, and the arithmetic promoted in
[integers.md §0a](integers.md). Three consequences, and each is already enforced
by something that exists:

- **It may not name another target's primitives.** That is the free-name
  condition, and it is what makes a target library target-*private*.
- **It may not be recursive** — [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md),
  checked by `Env.CheckProgram` before reduction.
- **It must reduce to primitives of `T`** — which is covering, already computed,
  demand-driven, and reported by name.

There is no need for a syntactic subset because the *semantic* condition is
already checked, and it is checked on the residual, where it is a fact rather
than a promise. `emit/bignum.oro` uses `loop`, `build`, `let`, `if`, `again` and
declared ranges — the whole language — and is correct because it covers.

**A target library may not DECLARE.** `def` and `sig`, never `prim`, `type` or
`structural`; otherwise `P_T` would depend on reduction, and `P_T` is reduction's
parameter. That is the one genuine restriction and it is a stratification, not a
taste.

### 6.4 What it costs and what it risks

**Cost: a form and a load step.** A target file gains `(library PATH)` naming an
Oroboros module the target ships, loaded into `D_T` when the target is loaded.
The three embedded files move from `//go:embed` into the target directories that
need them, and `emit/biglimb.go`'s splice becomes the ordinary δ.

**Risk, stated plainly: a target library can be slow or wrong, and only
conformance can tell.** A `prim` is one line and a lie in it is bounded; a
library is a program and a lie in it is a program. The mechanism that answers
this already exists and is already the answer for `split-words`: a signature is a
claim checked in two directions, and the conformance suite
([modules.md §8](modules.md)) is what checks it. **A target library must carry a
`sig` and must be in the conformance suite** — that is the price of admission,
and it is the same price a portable library pays.

---

## 7. Question 3 — where a target lives

*"When a user creates a new target for his use, does it have to be in the target
folder, or can it be in the project folder?"*

### 7.1 Today: one directory, and it is an asymmetry

```go
dir := flag.String("targets", "targets", "directory holding target declarations")
tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
```

versus modules, which get a genuine search path:

```go
func libDirs(entry, extra string) []string {
    dirs := []string{filepath.Dir(entry)}          // the source's own directory
    for _, d := range filepath.SplitList(extra) { … }
}
```

So a library may live beside the program and a target may not. Pointing
`-targets` at a project directory works — it is how the Go-standard-library
survey built and ran a generated target — but it **replaces** the built-ins
rather than adding to them. A user target and `targets/go` cannot both exist.

That is not a design decision anywhere; it is one call to `filepath.Join` that
was never generalised.

### 7.2 Proposed: a target is the glue of the fragments on a path

```
    Δ_T  =  L₁ ▷ L₂ ▷ … ▷ Lₖ           each  Lᵢ = ⨆ { fragments for T in layer i }
```

Layers ordered nearest-first: the program's own directory, then `-targets`
entries, then the built-ins. **Within a layer, glue** — order-free, agreement
required, so two files in one directory cannot silently disagree. **Between
layers, override** — so a project can replace a built-in declaration deliberately
and the rule says which wins.

This is §2's two operations used exactly once each, and it makes the target path
and the module path the same shape, which is what R1 says they should be.

---

## 8. Question 4 — a portable library with native fast paths

*"If the user creates a portable library, does he have to put it in the target
folder for each target he supports, or is there another way?"*

**No, and the mechanism is specified and unbuilt.** modules.md §6 already writes
it:

```lisp
; std/words — the portable definition, used by any target with no native
(module std/words (export split-words))
(def split-words (fn (t) (fold-ws t)))

; targets/go/std-words.oro — Go has it natively
(provides go std/words
  (prim split-words (string) (array string) expr "strings.Fields(%s)" pure (import "strings")))
```

Nothing in `emit/target.go` parses `provides`. The four cells are implemented;
the syntax that lets a *library* fill the third one is not.

### 8.1 The only change needed is where fragments are found

`(provides T M …)` is a **declaration fragment for target `T`**, and §7.2 already
says a target is the glue of its fragments. So the answer is one sentence:

> **A library's own directory is a fragment source.** `mylib/mylib.oro` holds the
> portable definitions; `mylib/go.oro` holds `(provides go …)`; both are found on
> the module path, and the target picks up the fragment when the library is in
> scope.

The library author never edits `targets/`. Adding a target means adding one file
to the library. Adding a library means editing nothing.

### 8.2 The theorem that comes with it, and it has teeth

Adding a fragment enlarges `P_T`, so by §9 T5 it can only turn *not covered* into
*covered*. **It is not monotone on emitted code**: a name in `P_T ∩ D` takes the
native, so shipping a `provides` fragment *changes what an existing program
compiles to*. That is conditional lowering working as designed — and it is also
exactly how a library could silently change a program's answer.

**This is what the conformance obligation is for**, and it should be stated as a
rule rather than left to the theorem: a `provides` fragment claims
`⟦n⟧_T = ⟦n⟧_Σ`, nothing checks it, and `split-words` passed every mechanical
check for two months while returning different answers on different targets.

---

## 9. Theorems

**T3 — gluing is order-independent.** `⊔` is commutative, associative and
idempotent where defined, so the order in which fragments are discovered cannot
change `Δ`. *Proof:* immediate from §2.1, since the value at every name in an
overlap is required to be equal. ∎

**T4 — a family shares covering.** If `π ∘ Δ₁ = π ∘ Δ₂` then `dom Δ₁ = dom Δ₂`,
so `P_{T₁} = P_{T₂}`, and covering depends only on `P_T`. **A program that builds
for one member of a family builds for all of them.** *Proof:* covering is
`Residual_T(nf_T(t)) = ∅`, and both `nf_T` and `Residual_T` mention `T` only
through `P_T`. ∎

This is the payoff of §5 and the reason the split is worth making: *port to
ARM64 and every program that built for x64 still builds*, by construction rather
than by testing.

**T5 — adding a fragment is monotone on coverage.** `P_T ⊆ P_T ∪ dom φ`, and
`Residual` is antitone in `P_T`. So a fragment can never un-cover a program. ∎

**T5′ — and it is NOT monotone on emission.** For `n ∈ P_T ∩ D`, R2 makes δ stop
where it previously unfolded, so the residual, the emitted code, and — absent
conformance — the answer may all change. ∎

**T6 — a target library adds no reduction power.** `D_T` enters normalisation
only through the definition environment, exactly as `D` does; `unfoldable` is
unchanged. So modules.md's **T1** — reduction is target-independent except at the
floor — survives §6 verbatim. ∎

---

## 10. What this does not change

- **The structural set stays closed.** §6 lets a target ship *definitions*, never
  a new binder. A target library is written in the language; it does not extend
  it.
- **A target still may not decline a language construct.**
  [ADR 0017](../decisions/0017-booleans-are-in-the-language.md)'s rule is
  untouched: `if`, `let`, `loop`, `the`, `=` and the promoted arithmetic are
  injected, and declaring one is an error.
- **`P_T` is still data and still reduction's parameter.** §6.3's refusal to let
  a target library declare anything is what keeps that true.

## 11. Build order

Ordered by measured value, with the Go survey's numbers where they apply.

1. **Several results in a `prim`** — not from this document, but it dominates
   everything: 19.8% of the Go standard library, and the language already has
   `values` at parity ([gostdlib-2026-09-06 §4a](../../gauntlet/results/gostdlib-2026-09-06.md)).
2. **A declared result range that the interval layer reads** — §4b of the same
   result. Without it, ADR 0019 refuses arithmetic on every host call in every
   ecosystem, and `-checked` is the only way through.
3. **§7.2, the target search path.** Smallest change here, unblocks the user
   question directly, and is the prerequisite for 4 and 5.
4. **§8, `provides` from a library's own directory.** Specified since modules.md
   and never built; makes a portable library with native fast paths a
   one-file-per-target job.
5. **§6, `(library PATH)`.** Moves three files out of `//go:embed`. Should wait
   for a target that needs a fourth — windows' `concat` and `string-of` over
   `build` is exactly that, and is next in the strings work.
6. **§5.4, the interface/implementation split.** Wait for a second Windows ISA.
   A family with one member proves nothing.
