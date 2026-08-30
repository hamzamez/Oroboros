# How a value grows

Research. **No decision, no specification** — this document exists to say what the question actually
is, what is already settled, what each answer would cost, and what has to be measured before anything
is written down.

[general-purpose.md §2.4](general-purpose.md) owes two things and calls them *"growable collections,
and maps"*. **The order is wrong, and that is this document's main finding.** A growable array has a
measured, parity-preserving workaround that every array language uses; a map does not. The map is the
one that cannot be worked around, and it is the one that reaches further into the language than
anyone had noticed.

---

## 0. What is already settled, so it is not relitigated

| | where |
|---|---|
| A table is a function with a known finite domain; indexing is application | [tables.md §2](spec/tables.md) |
| `(map K V)` is `(fn (K) V)` restricted to the keys present | tables.md §2 |
| Values immutable; mutation only inside `build`; the buffer is **linear** and frozen on exit | [ADR 0018](decisions/0018-immutable-values-linear-buffers.md) |
| `filter` needs a push collection, and push→pull materialises — at the same point hand-written code does | [q5b](spec/q5b-filter.md) |
| `filter` as **count, then build** — the array-language answer, two passes, exact size, no growth | tables.md §14.3 |
| Growth is *reachable today* target-natively: Go `append`, JS `push`, Java `ArrayList` are declared | tables.md §14.3 |
| There are **two buffer disciplines**: zero-filled scatter, and append-only accumulation | tables.md §14.3 |
| Sums are built, closed/finite/non-recursive, **1.00× against hand-written Go with 0 allocs** | [sums.md](spec/sums.md) |

What is *not* settled, and what this document is about: **whether a growable form enters the
language, and what a map is when it does.**

One fact worth having up front: **`(map K V)` is not a parseable type today.** `core.TypeName`
accepts a bare name, `(array V)` and `(int LO HI)`, and nothing else — tables.md §5.4 *designs* `map`
as a type operator, and the implementation has never seen one. What exists is monomorphic and
target-native: Go declares `map-string-int`, Java `java/Map`, JavaScript a bare object.

One housekeeping note: tables.md has **two sections numbered 14.3** — the zero-fill guarantee and
`append`. Worth fixing whenever that file is next touched.

---

## 1. The algebra: growth is a change of index set

A table is a dependent product over a finite index set:

```
    T  =  Π_{i ∈ I} V
```

An array has `I = Fin n`. A map has `I = S`, a finite subset of `K`. A total function has `I = A`.
Everything tables.md says follows from what `I` is.

**Growth is exactly the operation that changes `I`.** Nothing else about the algebra moves: `V` is
unchanged, the product is unchanged, reading is still application. So the question "how does a value
grow" is the question "how does a program change an index set", and there are precisely two ways to
change a finite index set — extend it by a *position*, or extend it by a *point*.

### 1.1 Two growths, and they are not the same operation

```
    append :  Buf V → V → Buf V              dom' = Fin (n+1)          n' = n + 1
    insert :  Map K V → K → V → Map K V      dom' = dom ∪ {k}         |dom'| ∈ {|dom|, |dom|+1}
```

> **Observation (equation versus inequality).** `append` extends the domain by exactly one and the
> new index is **known** — it is the old length. `insert` extends the domain by one **or zero**, and
> which depends on data.

That difference is the whole cost story, and it is sharper than it looks.

**Proof that appending keeps an equation.** Let a loop carry `(b, i)` with `b` a buffer and `i` a
counter, and let every `again` append exactly once and step `i` by one. Then `len b = len b₀ + i` is
an invariant: it holds initially, and if it holds before an iteration then after one `append` and one
`+1` both sides have grown by one. ∎

That is a *linear* fact in the fragment `emit/linear.go` already decides, and it is derived by
induction on iterations — the same induction `emit/monotone.go` already performs for
[loop monotonicity](../gauntlet/results/monotone-2026-08-27.md). **So the machinery to bound a
growable array's length exists and was built for something else.**

**Why insertion keeps only an interval.** After `k` insertions, `|dom m| ∈ [min(1,k), k]` and nothing
narrower is derivable, because whether the *j*-th key was already present is a fact about the input.
An interval is the best any non-relational domain can do, and no relational domain helps either — the
missing fact is about set membership, not about arithmetic between variables. (That is the same
reason octagons were refuted in [maxlen-2026-08-28](../gauntlet/results/maxlen-2026-08-28.md); the
gap keeps not being a linear one.)

---

## 2. Who discharges the domain condition

tables.md says a bounds check *is* a domain condition, and puts three constructs on one scale. Read
that scale again and something is visible that the original framing does not say:

| | domain | the condition | **who discharges it** |
|---|---|---|---|
| `(fn (A) B)` | all of `A` | trivial | the **type**, statically |
| `(array V)` | `[0, len)` | `0 ≤ i < len a` | the **refinement layer**, statically, in QF-LIA |
| `(map K V)` | the keys present | `k ∈ dom m` | **nobody, at compile time** |

> **The three points on tables.md's scale are three answers to the question "who discharges the
> domain condition", and the map is the first one whose answer is *the program, at run time*.**

**Why it cannot be discharged.** `dom m` is a function of the program's input. For a `sig`-exported
function the input is universally quantified, so `k ∈ dom m` is a statement about all inputs and is
not in any decidable fragment we have — it is not linear arithmetic at all. Inside a program with a
literal input, reduction *does* decide it, because staging evaluates the insertions; that is the
two-level language working, and it is exactly why the static level can have a rich map library for
free while the dynamic level cannot.

**And this is the first language-internal argument for sums.** [sums-research.md](sums-research.md)
justified sums by errors, Win32 contracts and dispatch — all of them *outside* the language. A map
read is a construct **in** the language whose obligation is undischargeable by construction, and
sums are the mechanism that already exists for exactly that shape. Go says the same thing with
comma-ok; tables.md noticed the coincidence and did not draw the conclusion.

### 2.1 The design fork this opens

- **F1 — `(m k) : V`, obligation always reported.** Uniform with arrays, and every map read in every
  program becomes a permanent *propagated, not proven* note. A diagnostic that always fires is a
  diagnostic nobody reads.
- **F2 — `(m k) : (option V)`.** Application still, result type decided by the domain kind. This is
  the one that fits §2's table: each point on the scale pays for its domain condition in its own
  coin. Costs a sum per read — measured free when consumed locally (sums.md), **unmeasured** when
  not.
- **F3 — `(get m k default)`.** What `wordcount` does today via Go's zero-value read. Simple, no
  sums, and it **conflates absent with zero**, which is the bug class Go's own comma-ok exists to
  avoid.

F2 is the one the algebra points at. It is not free of risk: it makes `(a i)` and `(m k)` differ in
result type, which is a wrinkle in "indexing is application" — though arguably an honest one, since
their domains differ and the result type is *where* that difference should show.

---

## 3. What growth costs the analyses, item by item

Five things now depend on a buffer's shape. Growth touches them very unevenly, and three of the five
cost nothing.

| | growable array | map |
|---|---|---|
| **Linearity** (ADR 0018) | unchanged — `append` consumes and returns, exactly like `set` | unchanged |
| **Refinement** (`i < len b`) | the equation `len b = len b₀ + i` survives, by §1.1 | **no positional index exists**, so nothing to discharge |
| **Element range** (`ElemType`) | unchanged, and *tighter*: no unwritten slots, so the zero-fill join can be dropped | same mechanism gives the **value range** — which is `wordcount`'s last unproven operation |
| **Intervals** (`exactLen`) | `len b` stops being a constant; bounded instead by the loop's trip count | `|dom m|` is only an interval, by §1.1 |
| **Termination** | a loop guarded by a growing length is a worklist | same |

Two of those deserve a sentence each.

**The element range is the one that pays immediately.** `(+ (at-map m w) 1)` is the only unproven
operation left in the native corpus outside `tree.oro`, and it is unproven because nothing declares a
map's value type. `(map string (int 0 65535))` gives it by declaration, through exactly the mechanism
[elemwidth](../gauntlet/results/elemwidth-2026-08-27.md) built for `(array (int 0 255))`. **The map's
value range is free.**

**Termination is precedented rather than novel.** `tree.oro`'s walk already loops over a structure
whose progress it cannot prove and carries an explicit trip bound. A loop over a growing collection
is the same shape and gets the same answer, and CLAUDE.md's argument applies unchanged: the bound
does not create the limit, it makes it visible.

---

## 4. The hosts, and which measurements are stale

| | growable sequence | map | notes |
|---|---|---|---|
| Go | `append`, amortised doubling | `map[K]V`, comma-ok | declared today |
| JavaScript | `Array.push` | **null-prototype `Object`**, not `Map` | `Map` measured **3.25× slower** — first baseline, months old |
| Java | `ArrayList` | `HashMap` | `merge` vs `getOrDefault`+`put`: 2.6× *slower*, then **1.19× faster** on JDK 17 — did not reproduce |
| windows | `HeapReAlloc` available | **nothing** | we own the allocator |

Two of those four numbers are exactly the kind [ADR 0008](decisions/0008-measurement-over-principle.md)
says must not be quoted without re-running. The Java one has already failed to reproduce once. Every
surprising JavaScript result in this repository has been a benchmark-method error at least once
([jsontok](../gauntlet/results/jsontok-2026-08-26.md) §: a closure per byte worth 1.5×), and the JS
map number has never been re-taken.

**So: three of the four measurements this design would rest on are stale, and one is known-unstable.**
Re-measuring them is cheap and comes first.

---

## 5. windows, and the staging it forces

Three hosts bring a hash map. windows brings none, and CLAUDE.md's rule is explicit: *a target does
not get to decline a language construct; if a host has no native form we build one.* So a map in the
language means **writing a hash table** for windows.

The interesting part is where it should be written. Not in Go, in `emit/` — that is the "adding a case
to `emit/*.go` for a host function" mistake. **In Oroboros**, compiled to windows, using `build`,
`loop` and sums. That would be the first library the language writes for itself, and it is a real
test of whether the language is expressive enough to be general-purpose — which is the question
[general-purpose.md](general-purpose.md) exists to answer.

But hashing a key needs the key's **bytes**, and for string keys that is the string question, which
[strings.md](spec/strings.md) says is unsettled (`length` of `"🙂"` is 4 / 2 / 1). Which gives a
staging that nobody has to argue about:

> **`(map int V)` first.** No string question, trivial hashing, expressible on all four hosts,
> and it answers every structural question in this document — growth, the undischargeable domain
> condition, the value range, the windows implementation. `(map string V)` follows strings.

`(map int V)` is not a toy: a sparse array, a graph's adjacency, a memo table, and the interning
table a string map would itself need are all keyed by integers.

---

## 6. The real program tables.md is waiting for

tables.md §14.3 declines to specify a growable form until *"a real program"* needs it. Four
candidates, and the result is not what the section assumed.

- **`filter`** — solved without growth. Count, then build: two passes, exact size, and it is how
  Futhark, ISPC and every GPU library do it. Not a witness.
- **`tree.oro`'s node table** — capped at 512, and CLAUDE.md *celebrates* the cap: the explicit limit
  is the thing the program is showing off. Growth would delete a feature. Not a witness.
- **A tokeniser producing a token table** — genuinely unbounded in the input. A witness, but a
  capped-with-count version already exists and is at parity.
- **`wordcount`** — needs a map, is written **four times** in this repository (portable, Go, JS,
  Java), and **cannot run on windows at all**.

> **The array's growth has a workaround that matches hand-written code. The map's does not.** So
> general-purpose.md §2.4's *"growable collections, and maps"* has it backwards: the map is primary,
> and a growable array may well never be needed.

This disagrees with a prior document and should be read as a disagreement rather than a refinement.
tables.md §14.3 calls the growable buffer *"the natural extension"* of ADR 0018 and treats the map as
the smaller question. On the evidence here it is the other way round, and the disagreement is
settled by measurement 3 rather than by either document's argument.

That also explains why `build` has survived two parsers, a sieve, a stencil and a tree without
anyone missing growth: **every one of those is positional**, and positional accumulation always has a
count-then-build form.

---

## 7. Design space, with what each costs

**D1 — Nothing changes.** Maps stay target-native. *Cost:* `wordcount` stays four programs and
windows stays unable to run it; a language construct is declined by a target, which is what
[ADR 0001](decisions/0001-parasite-model.md) exists to refuse. This is the status quo and it is not
tenable for a general-purpose language.

**D2 — A linear map buffer, ADR 0018 with the domain unfixed.**

```lisp
(collect (fn (m) …))        ; m : (mbuf K V), linear, frozen on exit to (map K V)
(put m k v)                 ; consumes m, returns m
(m k)                       ; (option V) — see §2.1
```

Everything ADR 0018 relies on survives untouched: linearity is the same ordering property, the freeze
still copies nothing, stores are still sequenced by [ADR 0010](decisions/0010-effects-as-structural-rules.md),
and the buffer still cannot escape because closures are refused. **What is new is only that the
domain is not fixed before the body runs.** That is a small delta on a decision already made, which
is the strongest evidence available that the mechanism is right — the same argument ADR 0018 made
about `print-line`'s effect discipline.

*Cost:* a third table-shaped thing beside `array` and `buffer`; `|dom|` is an interval; the windows
implementation.

**D3 — D2 plus a growable array**, `(push b x)`, the append-only discipline tables.md §14.3
describes. *Cost:* a fourth table-shaped thing, for a case §6 says has a parity-preserving
workaround. **Deferrable, and probably should be deferred**, with the trigger being a measured program
where count-then-build loses.

**D4 — Persistent maps** (HAMT, Clojure-style). *Rejected on sight,* and the reason is the parasite
rule rather than taste: no host provides one, so we would be lowering far below what every target
offers natively — which CLAUDE.md names as the single most common way to get the architecture wrong.

**D5 — Maps at the static level only.** The two-level language already permits an unrestricted
higher-order static level ([closures-direction.md](closures-direction.md)); a compile-time map costs
nothing and reduces away. *This is already true and worth writing down*, but it does not serve
`wordcount`, whose keys come from input.

---

## 8. What to measure, and the predictions recorded first

> **MEASURED 2026-08-30 — [maps-2026-08-30](../gauntlet/results/maps-2026-08-30.md).** All three of
> the gating numbers moved.
>
> **1. JS `Map` vs object: 1.56x**, not 3.25x — direction survives, magnitude halved. And a plain `{}`
> beats `Object.create(null)`. **Integer keys are 3.67x**, more than twice the string gap, which makes
> `(map int V)`-first the case where the host choice matters *most* rather than a way of dodging
> strings.
>
> **2. Java `merge` is 1.22x FASTER**, not 2.59x slower. Second independent re-take to disagree with
> R5; `targets/java/util.oro` declares the losing idiom.
>
> **3. Count-then-build BEATS append** — 2.95x on Go against growing `append`, 1.44x against
> preallocated, and 1.06x on JavaScript. **So D3, the growable array, is withdrawn**: the workaround
> is not merely parity-preserving, it is faster. §6's conclusion that the map is primary is
> strengthened rather than merely confirmed.
>
> And prediction 3 below was **wrong**, for a reason worth keeping: the first run compared a plain
> `Array` against a `Float64Array` and so measured array kind rather than growth.



The repository's rule is that a prediction written after the run is not a prediction. These are
written before.

1. **Re-take JS `Map` versus null-prototype `Object`.** *Predicted:* the object still wins, but by
   less than 3.25× — V8 has had years of `Map` work since that baseline.
2. **Re-take Java `merge` versus `getOrDefault`+`put`.** *Predicted:* no stable answer, which is
   itself the finding, and the target should declare both as it does now.
3. **Count-then-build versus append**, one filter program, four hosts. *Predicted:* two passes win on
   Go and windows for small elements (the counting pass is a tight scan) and lose on JavaScript.
   **This measurement decides whether D3 is ever needed.**
4. **Go `append` with and without a capacity hint.** *Predicted:* the hint is worth less than the
   15% noise floor for a few thousand elements, and materially more beyond.
5. **A hash table written in Oroboros versus the host's own**, on Go. *Predicted:* the host wins
   comfortably; if it does not, that is a warning that the parasite rule is being applied to
   something the hosts are not actually good at, and it should be recorded loudly.
6. **`(option V)` on a map read, consumed by an `if`.** *Predicted:* 1.00× and 0 allocations, because
   that is what sums measured for the analogous shape — and if it is not, F2 is in trouble and F3
   comes back.

---

## 9. What this document does not decide

- Whether maps enter the language at all. §6 argues they must for a general-purpose language; that
  is an argument, and ADR 0008 says arguments only select what is worth measuring.
- Which of F1/F2/F3 a map read is. §2.1 leans F2; measurement 6 can refute it.
- Whether a growable array is ever needed. Measurement 3 decides it.
- Anything about deletion, iteration order, or equality of maps. **Iteration order is the trap**:
  Go randomises it deliberately, the JVM's `HashMap` order is unspecified and has changed between
  releases, and V8's objects have a specified integer-key ordering. That is `split-words`'s shape
  exactly — four hosts agreeing by accident until they do not — and it needs its own section before
  any spec is written.
- Anything about strings, which §5 stages after `(map int V)`.
