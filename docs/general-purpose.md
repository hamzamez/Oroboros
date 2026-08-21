# General purpose, and reasoning about the target

hamza, answering [lowstar-lessons.md §9](lowstar-lessons.md)'s question:

> *"this is general purpose programming. I want apps on windows and android, I want website in the
> browser and I want backend in the cloud."*

and adding a requirement:

> *"because we have, and we want, unrestricted access to the target … our type system or proof
> system should be able to express and reason about the target. I should be able to express a
> windows api call so I can check my program in oroboros."*

**The first answer overturns something I wrote yesterday, and the second is a new capability.** This
document works out what each costs.

---

## 1. The correction

[lowstar-lessons.md §9](lowstar-lessons.md) read the gauntlet — dot products, stencils, sieves,
word counts — and concluded *"the honest read is that this is a systems and numeric language"*,
citing Low\*'s success as evidence for picking a narrow layer.

**That was a read of the benchmarks, not of the intent.** The intent is four deployment targets that
are all application platforms: a Windows desktop app, an Android app, a browser page, a cloud
backend. Corrected in that document.

The Low\* lesson does not disappear, it *inverts*: Low\* succeeded by picking a layer where the
restrictions are advantages. Choosing general purpose means **the restrictions must be paid for
rather than enjoyed**, and this document is the bill.

---

## 2. What general purpose pressures, ranked by how much it hurts

Six things. Several are the same decision seen from different sides.

### 2.1 Recursion — the hardest, and it is [ADR 0014](decisions/0014-recursion-is-not-in-the-language.md)

JSON. XML. A DOM walk. A recursive-descent parser. A directory tree. An expression evaluator. A UI
widget hierarchy. **These are the bread and butter of application programming and every one of them
recurses to a depth the input decides.**

[closures-direction.md §8](closures-direction.md) proposed relaxing ADR 0014 to *refuse recursion
that survives* rather than recursion that is written. General purpose needs the opposite: recursion
that **does** survive, because the depth is a runtime value.

Three ways out, and one of them matches a decision this project has already made:

**(a) Emit host recursion, and name the portable depth.** All four hosts have recursive calls. ADR
0014's objection is that stack depth differs by orders of magnitude and none guarantees TCO — but
that is *exactly* the shape of [ADR 0012](decisions/0012-portable-integer-range.md), which answered
"the hosts disagree past a limit" by **naming the portable window** (`int` is exact within
±(2⁵³−1)) and saying that beyond it the behaviour is the target's. A portable recursion depth is
the same answer to the same shape of problem, and the size-change machinery already computes
descent.

**(b) Defunctionalise to an explicit stack.** Turn recursion into a `loop` over a stack we allocate.
Portable, bounded, no host stack involved — and it needs the **growable buffer**
([tables.md §14.3](spec/tables.md)), which is already the natural extension of ADR 0018.

**(c) Make the programmer write the stack.** What we have. Honest, and a hard sell for a
general-purpose language.

**(a) is the one that fits the existing pattern**, and (b) is what the compiler could do underneath
it. This should get its own ADR superseding 0014, and it is now the largest open question in the
language.

### 2.2 Errors — every API can fail, and we have no sum

`VirtualAlloc` returns NULL. `ReadFile` returns a BOOL. `fetch` rejects. A database query errors.
**Error handling is not a corner of application programming; it is most of it.**

[data-structures.md §4.7](data-structures.md) deferred sums on the grounds that refinements cover
the `Option` cases where a precondition can be discharged. That is right for `aindex` and division
by zero and **wrong for a network call**, whose failure is genuinely dynamic and cannot be
discharged by any proof.

So a **closed, finite, non-recursive sum** — `Result`, `Option` — moves from *deferred* to
*required*, and §5 shows it is also what a target contract needs to say.

### 2.3 Strings

[strings.md](spec/strings.md) records that strings pass the portability test *only by having almost
no operations*, because `length` disagrees on `"🙂"` — 4 on Go, 2 on JS and Java, 1 counting
characters. An application does string handling constantly.

This is the place where "portability is a computed property" has to earn its keep: a portable
subset that names its price, plus target-native surfaces like `targets/go/strings.oro` for the rest.

### 2.4 Growable collections, and maps

[tables.md §14.3](spec/tables.md) and [data-structures.md §4.4](data-structures.md). Both were
deferred with target-native answers, which is the right call for a numeric kernel and probably the
wrong one when every program reads input of unknown length and keys things by string.

### 2.5 Concurrency, and async

All four platforms are event-driven: a Windows message loop, Android's main thread and coroutines,
the browser's event loop and promises, a cloud backend's request concurrency.
[ADR 0018](decisions/0018-immutable-values-linear-buffers.md) was written so this would be *safe*
when it arrives — tables shareable, buffers linear — and [callbacks.md](spec/callbacks.md) shows
the entry points are reachable. What does not exist yet is any notion of a task, a promise or a
happens-before.

### 2.6 Tier 3 closures

Dispatch tables, plugin registries, a handler stored and invoked later.
[closures-direction.md §9](closures-direction.md) argued the gain is small because almost every
host API is called from the place that knows what to do. **That argument is weaker for application
code than for kernels**, and it should be revisited when a real app is written rather than
re-argued now.

---

## 3. The new requirement, stated precisely

Here is everything a Windows declaration says today:

```lisp
(prim VirtualAlloc (int) ptr expr
  "xor ecx, ecx\nmov rdx, %1\nmov r8, 3000h\nmov r9, 4\ncall VirtualAlloc\nmov %r, rax"
  (import "VirtualAlloc"))
```

A name, argument types, a result type, an emission template. What it does **not** say:

| | |
|---|---|
| the size must be positive | a **precondition** |
| the result may be **NULL** on failure | a **sum**, and the reason §2.2 is required |
| on success it points to `size` writable bytes | a **postcondition** relating result to argument |
| it must eventually be passed to `VirtualFree` | a **resource protocol** |
| the pointer is invalid after that | **typestate** |
| it allocates and may fail | an **effect**, beyond the one `pure` bit |

`HeapAlloc (ptr int) ptr` is worse: its first argument must be a handle from `GetProcessHeap`, and
nothing says so — passing any `ptr` type-checks.

**The request is that these become expressible where the declaration lives**, so the checker that
already discharges `aindex`'s bounds can discharge a Win32 contract.

---

## 4. The literature, and one source that is unreasonably relevant

### 4.1 SAL — Microsoft already annotated Win32 for a static checker

The **Source-code Annotation Language** is what shipped in the Windows SDK so that
`/analyze` could check callers. Every Win32 header carries it. It is *field-tested* evidence of
what a Win32 contract actually needs:

| SAL | says |
|---|---|
| `_In_`, `_Out_`, `_Inout_` | direction — who reads, who writes |
| `_In_opt_`, `_Outptr_opt_` | **may be NULL** |
| `_Ret_maybenull_` | the **result** may be NULL |
| `_Success_(expr)` | which results mean success |
| `_Out_writes_(n)`, `_In_reads_bytes_(n)` | **buffer size, related to another argument** |
| `_When_(cond, ann)` | conditional contracts |
| `_Acquires_lock_`, `_Releases_lock_` | **resource protocol** |
| `_Check_return_`, `_Must_inspect_result_` | the caller must look at it |

Two things jump out. **Most of SAL is about buffers and sizes** — which our refinement system
already does, and does with a decidable procedure rather than SAL's heuristics. And the rest is
**nullability, success, and acquire/release**, which is a sum plus linearity.

### 4.2 The verification tradition

**F\*/Low\*** — preconditions, postconditions, `modifies` clauses, liveness and disjointness
([lowstar-lessons.md](lowstar-lessons.md)). **ACSL/Frama-C** and **JML** are the same for C and
Java. **Eiffel's design by contract** (Meyer) is the ancestor: `require`, `ensure`, `invariant`.

**Separation logic** (Reynolds 2002; O'Hearn) and its tools — **VeriFast**, **Viper**, **Iris** —
are the heavy artillery for heap reasoning, and are how you would verify a driver. *Too heavy for
us*, and F\*'s proof-instability lesson applies double.

**Typestate** (Strom & Yemini, TSE 1986; later Plaid, and Rust's typestate before it was removed)
is the right frame for open→read→close: an object's type changes with its state, and a wrong call
is a type error. `Acquires_lock`/`Releases_lock` is typestate with two states.

### 4.3 What other languages do at the FFI boundary

**Rust** — `Result` for failure, `Drop` for release, and `unsafe` marking the boundary where the
programmer takes responsibility. **Cogent** — uniqueness types for resources, with the C layer
verified separately. **ATS** — linear types plus DML refinements over a C FFI, which is closest to
our mechanism list. **Idris 2** — QTT, with the `0` quantity as erasure.

---

## 5. The map: what we have, what is missing

| what a target contract must say | our mechanism | status |
|---|---|---|
| precondition on arguments | `(where …)` on a prim | **have** — `aindex`, `go./` |
| buffer size related to another argument | refinements + `(length N)`/`(length-of N)` | **have** |
| purity / effects | the `pure` bit | **have, one bit** |
| **postcondition on the result** | — | **missing** |
| **the result may fail / be NULL** | — | **missing — needs §2.2's sum** |
| **acquire/release, use-after-free** | ADR 0018's linear buffer | **have the mechanism, not the surface** |
| **the caller must inspect the result** | — | **missing** |
| a handle's provenance (`HeapAlloc`'s heap) | — | **missing — a refinement over an opaque type** |

**Three of the eight are missing and one is a surface away.** That is a much smaller gap than it
looked, and the reason is that the refinement system was built for array bounds and array bounds
are most of what a systems API contract is about.

---

## 6. Candidates

**C-A — postconditions.** `(sig f ((a T)) (r R) (where P) (ensures Q))`, where `Q` may mention `r`.
The result needs a **name**, exactly as
[types.md](spec/types.md) already argues parameters do because *a refinement attaches to a name*.
Cheap: the same decidable fragment, the same decision procedure, a new clause.

**C-B — a nullable/fallible result.** The minimum is `(option T)` and `(result T E)` as closed
non-recursive sums, with `if`-like elimination. It is what §2.2 requires anyway, and it is what
`_Ret_maybenull_` and `_Success_` need. **This is the load-bearing one.**

**C-C — linear handles.** Generalise ADR 0018's buffer from "a scoped table" to "a scoped
resource": a handle introduced by `VirtualAlloc`, consumed by `VirtualFree`, checked by the same
`occurrences` machinery. Gets `_Acquires_lock_`/`_Releases_lock_` and use-after-free for free, and
is the natural second use of a mechanism built for one thing.

**C-D — an effect system.** More than one bit: allocates, reads memory, writes memory, does I/O,
may block, may fail. *Pressures* [ADR 0010](decisions/0010-effects-as-structural-rules.md), which
says explicitly that there are no effect types and that adding any should be argued against it
first. **Not obviously needed**: `pure`/impure plus C-C's linearity plus C-B's failure covers most
of what SAL says, and the ADR's argument — that what needed the discipline was effects, and
purity-conditioned structural rules were enough — has not been refuted by anything here.

**C-E — separation logic.** *Rejected.* The right tool for a driver, the wrong tool for us; F\*'s
proof instability is the warning and we have no proof engineers.

**C-F — typestate proper**, with named states and transitions. *Deferred*: C-C's linearity is
typestate with two states, and two states is what `open`/`close` and `alloc`/`free` need. A third
state can wait for a program that has one.

---

## 7. What it would look like

```lisp
(target windows
  (module win/kernel32
    ;; A contract, where the declaration already lives.
    (prim VirtualAlloc ((size int)) (r (option ptr))
      expr "…" 
      (where   (< 0 size))                       ; a precondition
      (ensures (=> (some? r) (writable r size))) ; result relates to argument
      (acquires r))                              ; r is linear until released

    (prim VirtualFree ((p ptr)) int
      expr "…"
      (releases p))))
```

and a program that uses it:

```lisp
(def with-page (fn (n k)
  (match (win.VirtualAlloc n)
    (none)   (fail "out of memory")
    (some p) (let (k p) (fn (r) (seq (win.VirtualFree p) r))))))
```

The checker's job, all of it from machinery that exists:

- `(< 0 size)` at the call site — the linear-arithmetic procedure, today.
- the `none` branch must exist — sum exhaustiveness, C-B.
- `p` must reach exactly one `VirtualFree` — `occurrences`, today, via C-C.
- `(writable p size)` feeds a later bounds obligation — refinements, today.

**And a program that forgets the `none` branch does not compile**, which is what
*"check my program in oroboros"* asks for.

---

## 8. What this changes about decisions already taken

| | |
|---|---|
| [ADR 0014](decisions/0014-recursion-is-not-in-the-language.md) — no recursion | **Under pressure.** §2.1. Needs a superseding ADR; the ADR 0012 pattern — name the portable window — is the shape of the answer. |
| Sums deferred ([data-structures.md §4.7](data-structures.md)) | **Reversed by need.** §2.2 and C-B. |
| Maps target-native ([data-structures.md §4.4](data-structures.md)) | **Weakened.** Right for kernels, thin for applications. |
| [ADR 0010](decisions/0010-effects-as-structural-rules.md) — effects as one bit | **Holds, for now.** C-D is not obviously needed. |
| [ADR 0018](decisions/0018-immutable-values-linear-buffers.md) — linear buffer | **Strengthened.** C-C is a second, independent use of the same mechanism, which is the best evidence a mechanism is right. |
| [ADR 0001](decisions/0001-parasite-model.md) — parasite model | **Strengthened, and this is the point.** A target contract is the capability graph *saying what it means*, not just what it is called. |
| Closures refused | **Unchanged today, revisit with a real app.** §2.6. |

---

## 9. Recommendation

**The target contract (C-A + C-B + C-C) is the right next design, and it is smaller than it looks
because five of its eight requirements already exist.** It also forces the sum, which general
purpose needs anyway — so one piece of work discharges two requirements.

Order:

1. **`(option T)` / `(result T E)`** — closed, finite, non-recursive sums with an eliminator. Load
   bearing for §2.2, C-B, and every host API that can fail.
2. **`(ensures …)`** — postconditions naming the result. Same fragment, same procedure.
3. **`(acquires …)` / `(releases …)`** — linear handles, reusing `occurrences`.
4. **Then recursion** (§2.1), which deserves its own ADR and its own measurement, and is the
   largest remaining question in the language.

And one thing to hold onto, because it is the strategic answer to §1: **choosing general purpose
means the restrictions are no longer free.** Every one of them — no recursion, no sums, no growth,
no closures — was justified by four hosts and by parity with hand-written code. Those
justifications are still true. What has changed is that the language now has to be *worth* them,
and the way to find out is [assessment-2026-08-20 §5](assessment-2026-08-20.md)'s last item, which
has been on the list since before any of this: **write something awkward.**
