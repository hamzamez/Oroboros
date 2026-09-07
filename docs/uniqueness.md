# Uniqueness on parameters

Research, 2026-09-06. **DECIDED the same day** —
[ADR 0020](decisions/0020-uniqueness-on-parameters.md) takes §10's four points: the smallest form,
the two obligations named at opposite ends, the corrected count, and the ergonomic trigger. It also
supersedes ADR 0013 and amends ADR 0018's consequence 3. This document is kept as the reading the
decision rests on.

The brief was: [ADR 0018](decisions/0018-immutable-values-linear-buffers.md)
says that firing its trigger 2 *"should produce an ADR adopting uniqueness on parameters rather than
a workaround"*, and [arrays-revisited.md](arrays-revisited.md) fired it. This is the reading that
should come before that ADR.

> **Three findings, in order of how much they change the ADR.**
>
> 1. **The demand is smaller than the assessment claimed.** Four independent demands were named;
>    **rule R closed two of them** three days ago. What is left is the *boundary* instance, and
>    exactly one case is measured and unmitigated.
> 2. **Linearity and uniqueness are not the same property, and this repository has used one word
>    for both.** They point in opposite directions, and an in-place parameter needs *both* — which
>    is why the ADR must say which of the two it is adopting at which end.
> 3. **The mechanism is far cheaper here than in any language that has it**, and for a structural
>    reason: ADR 0018 already made uniqueness a distinction between two **types** rather than an
>    **attribute** on every type, and whole-program reduction removes the propagation problem that
>    is most of Clean's implementation. What is missing is a *type name* and a *seed*.

---

## 1. The problem, stated without the word "uniqueness"

A function that fills a workspace must either receive it or allocate it. Today it must allocate it,
because there is no way to *say* in a signature that a parameter is a buffer.

```lisp
(sig mul ((a (array int)) (b (array int))) (array int))   ; workspace built inside, every call
```

Measured on Karatsuba, medians of three, the only difference being where the workspace comes from
([arrays-revisited §2](arrays-revisited.md)):

| | reused | fresh per call | cost | allocs |
|---|---:|---:|---:|---:|
| 1024 limbs, D=5 | 721,064 ns | 1,195,545 | **1.66×** | 10 / 504 KB |
| 1024 limbs, D=3 | 1,101,785 | 1,637,509 | **1.49×** | 10 / 198 KB |
| 256 limbs, D=4 | 107,230 | 142,964 | **1.33×** | 10 / 95 KB |
| 256 limbs, D=2 | 131,587 | 140,586 | **1.07×** | 10 / 30 KB |

The number understates the stake. [bigarith-2026-08-28](../gauntlet/results/bigarith-2026-08-28.md)'s
entire result is that our bignum beats `math/big` **because it allocates nothing** — `math/big`'s
inner loops are hand-written assembly and better than ours; what we avoid is its per-operation
overhead, and *naive* `math/big` is 4–5× worse than careful for exactly that reason. **A bignum
that allocates half a megabyte per multiply becomes the thing it beats.**

### 1.1 And the demand is smaller than it was said to be

[assessment-2026-09-06](assessment-2026-09-06.md) names four independent demands. Checked:

| demand | status |
|---|---|
| Karatsuba's workspace | **live**, 1.07–1.66×, unmitigated |
| ADR 0013's stencil | **live as a portability gap** — reuse is expressible on a native target via `go.set-float64`, at the cost of the portability claim; and the allocating shape costs 2.71× for *hand-written* Go too |
| the mutable bignum | **closed for the back edge** by rule R (bigreuse-2026-09-02): 6.4×/4.1×/1.44× recovered, and we now beat careful hand-written Go |
| the fixed-limb library | **closed for the back edge** by rule R: 200 allocations → 2, 19,270 → 13,850 ns. What remains there is the 24-bit limb and bitwise carry, not the buffer |

> **Rule R closed the BACK-EDGE instance of the gap. What uniqueness on parameters addresses is the
> BOUNDARY instance, and there is one measured live case.**

That is a materially weaker brief than "four demands", and the ADR should open with it rather than
inherit the count. It is not *nothing* — the one case is a library entry point, and libraries are
what general-purpose.md says we want — but a type-system feature bought with one measurement
deserves to be argued rather than assumed.

## 2. The mathematics: where this language already sits

A judgement `Γ ⊢ e : A` may or may not admit three **structural rules**:

```
  Exchange     Γ, x:A, y:B, Δ ⊢ e : C   ⟹   Γ, y:B, x:A, Δ ⊢ e : C
  Weakening    Γ ⊢ e : C                ⟹   Γ, x:A ⊢ e : C            (discard)
  Contraction  Γ, x:A, y:A ⊢ e : C      ⟹   Γ, z:A ⊢ e[z/x, z/y] : C  (duplicate)
```

Denying them gives the substructural family — Girard's linear logic (1987) is the case where
weakening and contraction are available only under the exponential `!A`:

| discipline | W | C | E | a variable is used |
|---|:-:|:-:|:-:|---|
| structural (ordinary λ) | ✓ | ✓ | ✓ | freely |
| **affine** | ✓ | ✗ | ✓ | at most once |
| **relevant** | ✗ | ✓ | ✓ | at least once |
| **linear** | ✗ | ✗ | ✓ | exactly once |
| **ordered** | ✗ | ✗ | ✗ | exactly once, in order |

**This language already denies all three, and it did so for a different reason.**
[ADR 0010](decisions/0010-effects-as-structural-rules.md) never substitutes an impure argument; it
let-binds it at the application site whatever its occurrence count, *"which denies contraction,
weakening and exchange in that order"*. That discipline was built for `print-line`, and ADR 0018
then observed that a buffer needs exactly those three properties and took them for free.

> So the substructural machinery is **already in the reducer**. What is being proposed is not a new
> discipline — it is letting an existing one be *named at a boundary*.

## 3. Linearity is not uniqueness, and a parameter needs both

This is the distinction the repository has been eliding, and it is load-bearing for the ADR.

**Linearity is a restriction on the FUTURE.** *This value will be used exactly once.* It is a
promise the **consumer** makes to the context. Wadler (1990) and Linear Haskell (Bernardy,
Boespflug, Newton, Peyton Jones, Spiwack, POPL 2018) put the multiplicity on the **arrow** —
`a %1 -> b` is a function that consumes its argument once — precisely because it is a property of
*how the function uses it*, not of the value.

**Uniqueness is a guarantee about the PAST.** *No other reference to this value exists.* It is a
promise the **producer**, or the caller, makes. Clean (Barendsen & Smetsers, 1993) defines it on the
graph: a node is unique when its reference count is one, and the attribute is on the **type** —
`*[*Int]` — with a subtyping coercion `unique ⊑ non-unique` that may always be taken and never
reversed.

Marshall, Vollmer and Orchard put the two side by side in *"Linearity and Uniqueness: An Entente
Cordiale"* (ESOP 2022): linearity is a guarantee the function *gives* the context; uniqueness is one
the context *gives* the function. They are duals, not synonyms, and a system with both needs two
modalities and two coercions.

**An in-place parameter needs one of each, at opposite ends:**

```
        caller ────────── uniqueness ─────────▶ the call     "nobody else holds this"
        the body ───────── linearity ──────────▶ the result  "I thread it and hand it back"
```

Neither alone suffices. Uniqueness without linearity lets the body drop the buffer or read it after
a store. Linearity without uniqueness lets the caller keep an alias and observe the write.

**This is the correction ADR 0013 already half-recorded** and CLAUDE.md preserves: *"uniqueness
constrains the CONTEXT, not the value, so it is not a refinement."* That sentence is exactly
Marshall/Orchard's point, arrived at from a different direction and never connected to the design.

## 4. What the language has today, and none of it is a type

Three mechanisms, worth separating because the ADR will build on the third.

**(a) ADR 0010's ordered discipline, in the reducer.** An impure argument is never substituted. This
is what sequences stores.

**(b) `emit/effects`' read/store ordering.** [effects.md §7c](spec/effects.md) — a term that reads a
table through a bound variable is not substituted into an impure body, because a read moving across
a store is a silent wrong answer (a swap became a copy).

**(c) `CheckLinear`, in `emit/linearity.go`.** This is the real one, and it is *not a type system*:

- it runs on the **residual**, after reduction has flattened the program;
- it is an **ordering** check, not a counting one — it tracks one bit, `moved`, per buffer;
- **reads do not consume**, so `(b i)` before a store is free;
- it walks in **evaluation order**, forks at an `if` and rejoins conservatively;
- the buffer's name comes from `build`'s lambda parameter.

Two things about it are worth naming, because they are results rather than conveniences.

**It is Wadler's `let!` and Odersky's observers, without the machinery.** Wadler (1990) needed a
`let!` construct and Odersky (*"Observers for linear types"*, 1992) a whole calculus to allow
reading a linear value without consuming it. We allow it with one line — and the reason is
structural:

> **No element type is unique.** A buffer's elements are scalars or frozen arrays, and
> `(array (buffer V))` is not a type. So an observation `(b i)` produces something that **cannot
> alias the buffer**, and there is nothing for a borrow checker to check. The read-borrow is free
> because ADR 0018 made every value immutable.

**And it is a move check, which is Rust's ownership without borrowing.** `moved` is exactly Rust's
moved-out-of state; what is absent is `&`/`&mut` and lifetimes, because (per above) there is nothing
to borrow.

## 5. The literature, and the two languages with our exact constraints

| system | mechanism | what it teaches us |
|---|---|---|
| **Clean** (Barendsen & Smetsers 1993) | uniqueness *attributes* on every type, with attribute variables and `u ⊑ n` subtyping | the design we are told to copy — and its cost is the attribute algebra, which infects every signature |
| **Futhark** (Henriksen et al., PLDI 2017) | `*[]f64` parameter = consumed; whole-program; aliasing analysis | **closest match**: array-oriented, no general recursion, whole-program, data-parallel. Its parallel/sequential distinction is ADR 0018's |
| **Cogent** (O'Connor et al., ICFP 2016) | linear types for verified systems code, no GC | linearity gives a *frame rule* for free — which is [lowstar-lessons](lowstar-lessons.md)'s point that `modifies` is syntactically the buffer |
| **Linear Haskell** (POPL 2018) | multiplicity on the **arrow**, not the type | existing code does not change. The opposite of Clean's choice and much cheaper to adopt |
| **Rust** | ownership + borrowing + lifetimes | the complete answer, and the one with the largest surface. RustBelt (Jung et al. 2018) is its semantic model |
| **Haskell `ST`** (Launchbury & Peyton Jones 1994) | rank-2 `runST` stops the reference escaping | ADR 0018 already cites this: closures are refused, so *we get the only thing rank-2 buys* |
| **Koka / Perceus** (Reinking et al., PLDI 2021) | runtime reference counting with reuse | rejected in ADR 0018 (c): three hosts bring a collector, so we would pay twice |
| **FP²** (Lorenzen, Leijen, Swierstra, ICFP 2023) | a *static* discipline guaranteeing in-place execution | the newest and most relevant: it is rule R generalised into a type discipline |
| **Granule** (Orchard et al., ICFP 2019) / QTT (Atkey 2018), Idris 2 | graded modalities, multiplicities 0/1/ω | the general theory. Far more than is needed here |

**Futhark and Cogent are the two with our exact constraints, and both chose uniqueness on
parameters.** ADR 0018 already says so. What the reading adds is *why the cost differed for them*:
both compile whole programs, and both still needed a real aliasing analysis, because both allow a
unique value to flow through data structures we do not have.

## 6. What it would look like here, and why it is smaller

### 6.1 ADR 0018 already made uniqueness a TYPE, not an attribute

This is the finding that changes the cost estimate. Clean needs `*` on every type because *any*
type can be unique, so the attribute must be inferred, propagated, generalised over, and coerced —
and that algebra is most of Clean's implementation and most of its reputation for unreadable
signatures.

Here there are exactly two table types and the distinction is already made:

| | reads | aliasing | ADR 0018 |
|---|---|---|---|
| `(array V)` | pure | freely shared | an immutable value — Pony's `val` |
| a **buffer** | impure | exactly one reference | linear and scoped — Pony's `iso` |

There is no attribute lattice to build, because there is nothing to attach an attribute *to*: the
uniqueness is the type constructor. And the one coercion Clean infers — `unique ⊑ non-unique` — we
already write explicitly, as the **freeze** at `build`'s boundary.

> **So the surface is one missing type name.** `(buffer V)` is not a type today —
> `(sig f ((b (buffer int))) int)` is refused with *"(buffer int) is not a type"* — and that single
> absence is the whole of trigger 2.

### 6.2 Whole-program reduction removes the propagation problem

**Claim.** With whole-program reduction, uniqueness checking reduces to (i) an *assumption* at each
export and (ii) the *existing* residual move check, seeded from the signature instead of from
`build`.

*Argument.* δ inlines every non-exported call before any check runs, so a parameter that is not an
export's survives no boundary — its occurrences are all in the residual, where `CheckLinear` already
decides them. At an export the caller is outside the program by construction, so no check is
possible and the annotation is a **published contract, assumed** — which is precisely
[refinements.md §6b](spec/refinements.md)'s middle row, the same trust model an exported `where`
already has. ∎

That is the reason this costs less here than in Clean or Futhark, and it is worth stating as the
ADR's central argument. It also predicts the implementation: **`CheckLinear` with a different seed.**

### 6.3 The shape of the declaration

Following Futhark, the workspace is taken and returned:

```lisp
(sig mul ((a (array int)) (b (array int)) (w (buffer int))) (buffer int))
```

and the obligations are:

| where | obligation | who discharges it |
|---|---|---|
| the body | `w` is threaded linearly: moved once, read freely before | `CheckLinear`, seeded from the signature |
| an internal call | no alias survives | the residual — there is no call site left |
| an **export** | the caller holds no other reference | **assumed** — the caller is the host |

## 7. What it buys, and what it does not

**Buys.** The one measured case: 1.07–1.66× on Karatsuba, and the removal of 10 allocations and
504 KB per multiply — which is the difference between a bignum that beats `math/big` and one that
does not. Structurally, it makes a *library* expressible: today every exported entry point that
needs scratch space must allocate it.

**Does not buy** — and this list matters, because each is a thing someone might expect:

- **Nothing inside a program.** Reduction already removes the boundary; arrays-revisited measured
  two multiplies threaded through one workspace emitting **one** `make`.
- **Nothing on the back edge.** Rule R closed that three days ago.
- **No new expressiveness.** Every program that can be written now can still be written; this is
  entirely about where the storage comes from.
- **No checking of the host.** At the export, the guarantee is a promise.
- **Not the stencil's portability gap.** That is a *portable-layer* problem: reuse is expressible
  natively, and what is missing there is a portable name for the store.

## 8. What it costs

**Ergonomics, which is the honest one and is unmeasured.** Threading a workspace through every
`again` and every helper is the cost Futhark users report, and arrays-revisited already names it:
*"unmeasured, a different complaint from trigger 1, and probably wants sugar rather than a memory
model."* It is the same complaint `tree.oro` raises about the explicit stack.

**A type name in the signature language**, which is small, plus its interaction with `array` — a
frozen buffer *is* an array, so the freeze must be visible in the types.

**One genuine soundness risk.** A buffer parameter that is *not* threaded but merely read would be
tempting to allow (`_In_` rather than `_Inout_`, in SAL's terms). That is Odersky's observer at a
boundary, and it is sound only while §4's structural reason holds — no element type is unique. If a
table of buffers ever becomes expressible, the read-borrow needs real machinery.

**And the cost this project should weigh most:**
[assessment-2026-09-06 §3.3](assessment-2026-09-06.md) records the compiler growing at 41 lines per
line of Oroboros, with the compiler-to-core ratio at 3.62:1 and rising. A feature justified by one
measurement, in a compiler already flagged for growing faster than the language, is exactly the
trade the assessment said to stop making — **unless it is as small as §6 says.** The ADR should
therefore be explicit that adopting it means the *small* version and not Clean's.

## 9. The alternatives, taken seriously

**(a) Do nothing.** The interior is free, the back edge is closed, and one library entry point pays
1.07–1.66×. Cheap, honest, and it leaves a bignum library that becomes what it beats.

**(b) Infer it — destination-passing at the boundary.** The compiler adds the workspace parameter
itself, which is rule R one level up (Shaikhha et al., FHPC 2017; and FP², ICFP 2023, as a
discipline). Attractive because it is already half built. **Refused on the project's own precedent
twice over**: an optimisation that silently does not fire is requirement 5's failure mode
(arrays-revisited §7), and adding a parameter to an *exported* function changes the published API,
so the information has to be in the signature anyway — at which point declaring beats inferring, for
[ADR 0019](decisions/0019-precision-by-declaration.md)'s reason.

**(c) Linear types on the arrow**, Linear Haskell style, rather than uniqueness on the type. More
general and a better fit for a language with higher-order functions — which at the dynamic level we
do not have. Multiplicities buy nothing when every arrow that survives reduction is first-order.

**(d) Ownership and borrowing**, Rust style. Strictly more powerful and strictly larger. The thing
it buys over (uniqueness + our free read-borrow) is aliased *shared* references, which we do not
need because values are immutable.

**(e) Free mutation.** Refuted in arrays-revisited §4: strictly dominated, and it would cost the
seven properties §3 of that document lists, four of which are measured results that postdate
ADR 0018.

## 10. What the ADR should decide

Not a recommendation to build, but the shape of the decision the reading supports:

1. **Adopt uniqueness on parameters in the smallest form**: make the buffer a nameable type, seed
   `CheckLinear` from the signature, assume the aliasing guarantee at an export. Reject Clean's
   attribute algebra explicitly, and record §6.1 as the reason it is not needed.
2. **State which property holds at which end** — uniqueness in, linearity through — and cite
   Marshall/Orchard for why one word will not do.
3. **Correct the count.** Two of the four demands were closed by rule R; the live case is
   Karatsuba's workspace, and the ADR should say so rather than inherit "four".
4. **Name the trigger for the next step.** The unmeasured cost is ergonomics. The honest trigger is
   *a program in which threading the workspace is what makes the program unpleasant* — which is
   [assessment-2026-09-06](assessment-2026-09-06.md)'s item 4 arriving from a different direction,
   and one more reason to write an application before adding surface.

**The one measurement that would change this reading** is a second live boundary case. One is a
finding; two would make it a pattern, and the corpus is about to acquire more library-shaped code
now that the ecosystem is reachable.
