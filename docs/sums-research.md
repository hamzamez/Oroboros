# Sum types: the research

Three independent requirements converge here, which is why it is next:

1. **Errors.** Every host API can fail, and a refinement cannot discharge a network error
   ([general-purpose.md §2.2](general-purpose.md)).
2. **Target contracts.** `_Ret_maybenull_` and `_Success_(expr)` are the two things a Win32
   declaration needs that we cannot say ([general-purpose.md §5](general-purpose.md)).
3. **Dispatch.** A tagged union is what replaces the closure — it is what Zig uses and what a
   closure-free language dispatches with ([decidability-map.md §7.2](decidability-map.md)).

**Status: research.** No decision follows from this document by itself.

---

## 0. What already works, and where the boundary is

The same experiment that found the Church list. A Church-encoded sum:

```lisp
(def inl (fn (a) (fn (l r) (l a))))
(def inr (fn (b) (fn (l r) (r b))))
```

**Static tag — it vanishes:**

```lisp
(def f (fn (x) ((inl x) (fn (a) (go.+ a 1)) (fn (b) (go.- b 1)))))
;; f = (fn (x) (go.+ x 1))
```

**Dynamic tag — it survives, and is refused:**

```lisp
(def f (fn (x) ((if (go.> x 0) (inl x) (inr x)) (fn (a) …) (fn (b) …))))
;; f = (fn (x) ((if (go.> x 0) (fn (l r) (l x)) (fn (l r) (r x))) (fn (a) …) (fn (b) …)))
```

**This is the third time this exact boundary has appeared**, and it is the same boundary each time:

| | free | must exist |
|---|---|---|
| product | consumed in the same reduction | escapes a boundary |
| table | index is static | index is **dynamic** |
| **sum** | tag is static | tag is **dynamic** |

The static/dynamic split *is* the cost boundary, everywhere in this language.

### 0.1 And the dynamic case is one rule away from free

Look at what the residual is stuck on:

```
((if c  A  B)  F G)
```

β cannot fire because the operator is not syntactically a lambda. **Case-of-case** —
`((if c A B) args) ⟶ (if c (A args) (B args))` — exposes the redex, and then everything reduces:

```
((if c A B) F G)
  ⟶  (if c (A F G) (B F G))           commuting conversion
  ⟶  (if c ((fn (l r) (l x)) F G) …)   
  ⟶  (if c (F x) (G x))                β
  ⟶  (if c (go.+ x 1) (go.- x 1))      β
```

**Fully reduced. No closure survives, no tag is built, nothing is allocated.**

[q5b §6](spec/q5b-filter.md) identified this rule and rejected it — *"load-bearing for the entire
collection library rather than one corner"* — because two representations were cheaper than
unifying pull and push. **For sums it is load-bearing in the other direction: it is the rule that
makes a locally-consumed sum free**, exactly as β makes a locally-consumed product free.

This is **Prawitz's commuting conversion** (1965) from natural-deduction normalization, and it is
**GHC's case-of-case** (Peyton Jones & Santos 1998), which is the transformation that makes
short-cut deforestation work. Its known hazard is term-size blow-up, and its known answer is **join
points** (Maurer, Downen, Ariola & Peyton Jones, PLDI 2017) — which
[types-direction §6.3](types-direction.md) already observed **we have, because `again` is one**.

---

## 1. The mathematics

### 1.1 The coproduct

`A + B` with injections `ι₁ : A → A+B`, `ι₂ : B → A+B`, universal among objects with a map from
each side: for `f : A → C` and `g : B → C` there is a unique `[f,g] : A+B → C`.

```
[f,g] ∘ ι₁ = f        [f,g] ∘ ι₂ = g          (β for sums)
[ι₁, ι₂]   = id                                (η for sums)
A + 0 ≅ A         A + B ≅ B + A        (A+B)+C ≅ A+(B+C)
A × (B + C) ≅ (A × B) + (A × C)                (distributivity)
```

Two consequences, both practical. Associativity and commutativity hold **up to isomorphism**, so a
**flat n-ary sum is as legitimate as nested binary ones** — the same argument that made a flat
n-ary product right. And **distributivity is case-of-case**: pushing a context into the branches is
exactly `A × (B+C) → (A×B) + (A×C)`, so §0.1's rule is not an optimisation bolted on, it is a law of
the algebra.

Cardinality gives the name: `|A + B| = |A| + |B|`. So `bool = 1 + 1`, `option A = 1 + A`,
`result A E = A + E`.

### 1.2 A sum is Σ over a finite index set — the dual of a table

[tables.md §1.1](spec/tables.md) says a table is a function with a known finite domain. The precise
dual:

| | | |
|---|---|---|
| `Π(i : Fin n). T i` | **all** of them | table, array, record |
| `Σ(i : Fin n). T i` | **one** of them, and which | sum, variant, tagged union |

Same index set, dual quantifier. And that immediately explains the cost asymmetry:

> **A Π can be given by a rule — compute the element on demand, store nothing. A Σ must carry
> *which*, and no rule can answer that. The tag is the irreducible content of a sum.**

That is why `(table n f)` is free and a sum is not, stated as algebra rather than as an
implementation observation.

### 1.3 Polarity: there is no negative sum

Linear logic separates four connectives, and the pairing is the point:

| | positive | negative |
|---|---|---|
| conjunction | `A ⊗ B` — build it, match on it | `A & B` — projections, never built |
| disjunction | `A ⊕ B` — build it, case on it | `A ⅋ B` — *not a coproduct* |

**A product has a negative form and we took it for free** ([ADR-less, but measured at
1.01×](../gauntlet/results/product-2026-08-19.md)). **A coproduct does not.** `⅋` is the negative
disjunction of linear logic and it is not the categorical coproduct — you cannot observe "which
side" without the information existing.

So the honest statement is not *"sums are expensive because of allocation"*. It is:

> **The tag is information the caller does not have and the callee does. It has to be transmitted.
> Everything else is representation.**

§0.1's case-of-case does not contradict this: when the tag is *statically* known, the caller does
have it, so there is nothing to transmit.

### 1.4 Church and Scott encodings

`A + B ≅ ∀r. (A → r) → (B → r) → r` — Böhm & Berarducci (1985) for the general result; for a
non-recursive sum the Church and Scott encodings coincide. This is a **function**, hence negative,
hence free when it reduces away — which is §0 restated in type theory.

The encoding needs rank-1 polymorphism (`∀r`), which the residual checker does not have. It does
not need to: reduction erases the encoding, and the case that *survives* gets a nominal type. Same
argument as [tables.md §5.3](spec/tables.md), third time.

---

## 2. Representation, and the one that costs nothing

### 2.1 The menu

1. **Tag plus payload** — a struct with a discriminant. Size `max(|A|,|B|) + tag`. No allocation if
   it is a value type.
2. **Niche / null-pointer optimisation** — use an unused value of the payload to mean the other
   variant. Rust's `Option<&T>` and `Option<NonNull<T>>` are **one word**, no tag.
3. **Pointer tagging** — spare low bits (OCaml tags integers) or **NaN boxing** (V8, LuaJIT).
4. **Boxed variants** — each variant a heap object. Java's sealed interface + records.
5. **Church/Scott encoding** — a function, free when reduced (§1.4).
6. **Struct-of-arrays** — for a *table of sums*, store tags contiguously and payloads separately.
   This is simdjson's tape and Zig's `MultiArrayList`, and it composes with
   [tables.md](spec/tables.md).

### 2.2 The niche optimisation is what host APIs already do — so a contract costs nothing

This is the most useful finding in the document.

```
VirtualAlloc  →  NULL or a pointer          =  (option ptr)
strings.Index →  -1 or an index             =  (option int)
map[k]        →  zero value or the value    =  (option V)   (Go totalises it)
ReadFile      →  0 or nonzero               =  (result … )
HRESULT       →  sign bit is failure        =  (result … )
```

**Every one of these is already a sum, encoded in the payload's own value space.** So declaring
`(prim VirtualAlloc ((size int)) (option ptr) …)` does not *add* a representation — it **names the
one the API already uses**, and lowers to exactly the NULL test a C programmer writes.

That is the parasite model applied to sums, and it means requirement 2 (target contracts) is nearly
free: no tag, no allocation, no wrapper. The compiler needs to know *which* value is the niche,
which is one declaration: `(none-is 0)` or `(fail-when (< r 0))` — and SAL spells the second one
`_Success_(expr)`.

### 2.3 Per target

| | representation | cost |
|---|---|---|
| **Go** | struct with a discriminant — a **value type** | no allocation. `interface{}` would allocate; do not use it. **Go has no sum type**, which makes it the hardest host here, not x86 |
| **JavaScript** | `{tag, v}` object literal | **1.11×** measured ([product-2026-08-19](../gauntlet/results/product-2026-08-19.md)); an array would be 1.32×. For `option`, `undefined`/`null` is the niche and is free |
| **Java** | sealed interface + records (JEP 409/441), or a class with a tag field | allocates, but **C2 scalar-replaces it** when it does not escape — measured |
| **x86-64** | two registers, or the niche | free by construction |

Go being the hardest is worth noting: it has no sum type and a long-standing rejected proposal, so
the emitted form is a struct with a tag and a `switch`. That is what Go programmers write by hand,
so it is at parity by construction.

---

## 3. The error model

The sum is the mechanism; the *error model* is the design question on top of it.

| | | verdict |
|---|---|---|
| **Exceptions** — Java, C++, JS, Python | non-local control flow, stack unwinding | **out**. We would have to implement unwinding on x86; it breaks "no runtime"; and `loop`/`again` is our only control flow |
| **Result + explicit propagation** — Rust `?`, Haskell `Either` | a value, checked by the type system | **the fit**. No runtime, works on four hosts, matches Go's and Rust's practice |
| **Multiple return** — Go `(v, err)` | convention, unchecked | Go's own form; we should *emit* this on Go and check it ourselves |
| **Error codes** — C `errno`, Win32 `HRESULT` + `GetLastError` | what the OS actually does | what we must *consume*; §2.2 says it is a niche-encoded sum |
| **Algebraic effects** — Koka, Eff, OCaml 5 | handlers, resumption | more general, needs a runtime for resumption. **Out**, and [ADR 0010](decisions/0010-effects-as-structural-rules.md) already argues against effect machinery |
| **Checked exceptions** — Java | widely regretted | out |

### 3.1 `try` is bind, and it can be reader sugar

Rust's `?` and Haskell's `>>=` for `Either` are the same operation:

```lisp
(try e (fn (v) rest))   ⟶   (case e  (ok v)  rest
                                     (err x) (err x))
```

Monadic bind for the error monad, written as a **case whose error branch is the identity**. It is
reader sugar in exactly the way `seq`, `and`, `cond` and `loop` are — no monads in the language, no
new reduction rule, and the residual contains only `case`.

The one thing to check is that it composes without nesting deeper than a reader can follow, which is
what Rust's postfix `?` solves syntactically. That is a surface question, not a semantic one.

---

## 4. Recursive sums, and why we do not need them

`List`, `Tree` and `JSON` are μ-types, and they would bring back everything
[data-structures.md §1.2](data-structures.md) removed.

**We do not need them**, and [decidability-map.md §7.1](decidability-map.md) is why, now with a
measurement: recursive *data* becomes **a flat table of nodes with integer indices** — simdjson's
tape, Zig's AST as a `MultiArrayList` of `u32` indices, an ECS world — and
[indexgraph-2026-08-21](../gauntlet/results/indexgraph-2026-08-21.md) measures that form at **2.02×
faster** than the pointer form on realistic irregular access.

A JSON node is then `(sum null bool (num f64) (str int) (arr int) (obj int))` — **non-recursive**,
with children as indices into a table. That is a closed finite sum plus the data structure we
already chose.

So: **closed, finite, non-recursive sums.** No μ, no cycles, no change to ADR 0014, and the acyclic
heap that ADR 0018 depends on survives untouched.

---

## 5. What it costs the compiler

### 5.1 Case-of-case — the fourth-rule question

§0.1 says it is what makes a locally-consumed sum free. Honestly accounted:

**It is a new normalization step.** β says *apply a function to an argument*; `(if c A B)` is not a
function, it *evaluates to* one. Pushing the application inside is a **commuting conversion** —
standard in Prawitz's normalization alongside β, but a fourth rule in our count of three.

**Its hazard is term-size blow-up**, and its answer is join points, which
[types-direction §6.3](types-direction.md) already noted `again` provides.

**And ADR 0010 already protects the effects.** An impure argument is let-bound at the application
site before any of this, so commuting the conditional cannot duplicate an effect — only static code.

### 5.2 Exhaustiveness

Maranget, *Warnings for pattern matching* (JFP 2007) is the standard algorithm; Sestoft (1996) and
Maranget (2008) cover compiling matches to decision trees; Augustsson (1985) is the backtracking
alternative.

**With flat, non-nested patterns we need almost none of it** — a `case` over a closed finite tag set
is a completeness check on a small set and a `switch` in emission. Nested patterns would need
Maranget, and are not required by any of the three motivating requirements.

### 5.3 Refinements: the tag is a case-split we already do

`emit/refine.go`'s `clauses` already walks an if-chain assuming each guard in its own branch, and
assuming the negation in the others — *"the other half of Hoare logic"*. **A `case` on a sum is the
same operation**: assume `tag = k` inside branch k.

So `_Success_(expr)` — a postcondition that holds only on the success branch — costs the refinement
system **nothing new**:

```lisp
(ensures (=> (ok? r) (writable (value r) size)))
```

is discharged by the case-split machinery that already exists. That is a strong argument that sums
and refinements were designed for each other, arrived at from opposite ends.

### 5.4 ADR 0018: a buffer inside a sum

`(result (buffer V) E)` puts a linear value inside a sum. Linearity must flow through the case: each
branch uses it at most once, and the branches are alternatives rather than sequential, so
"at most once per branch" is the rule. `occurrences` computes it per branch already — the same walk
`clauses` does.

Worth naming because it is the one place the two new mechanisms meet.

---

## 6. What the surface might look like

Sketch, not a proposal:

```lisp
;; a closed finite sum
(sum result (ok V) (err E))
(sum shape  circle square triangle)          ; no payloads — an enum

;; construction and elimination
(ok 42)
(case r
  (ok v)  (use v)
  (err e) (report e))

;; the niche, declared where the target declares the primitive
(prim VirtualAlloc ((size int)) (option ptr) expr "…"
  (none-is 0))

;; sugar
(try (VirtualAlloc n) (fn (p) rest))
```

Design questions inside that sketch, deliberately unanswered: whether `case` is a distinct form or
reader sugar over nested `if` plus a tag test; whether payload-less variants are a separate concept
(an enum) or the degenerate case; whether `option` and `result` are built in or library
declarations; and whether the niche declaration belongs on the *primitive* or on the *type*.

---

## 7. Candidates

**S-A — Church/Scott encoding as a library, plus case-of-case.** No language change at all; sums
become three definitions and one new reduction rule. *Buys* the free static case immediately.
*Costs* the fourth rule, and gives no story for the sum that survives — it is still a refused
closure.

**S-B — a real sum in the language, closed and non-recursive**, with `case` and per-target
lowering. *Buys* all three requirements. *Costs* a term form, an emission per target, exhaustiveness
checking, and a type-language addition.

**S-C — S-B plus case-of-case.** The complete story: free when local, built when it crosses a
boundary, exactly like the product and the table.

**S-D — recursive sums.** *Rejected*, §4.

**S-E — exceptions or effect handlers.** *Rejected*, §3.

**S-F — nothing; keep using refinements and target-native conventions.** The status quo. *Rejected
by requirement*: it cannot express `_Ret_maybenull_`, and it cannot represent a dynamic failure.

---

## 8. Where this points

**S-C**, and the order inside it matters: the *representation* is the cheap part (§2.2 — the niche is
already what the host uses), the *checking* is nearly free (§5.3 — the case-split exists), and the
expensive part is **case-of-case**, which is one well-understood rule with one well-understood
hazard and an answer we already have.

The thing to be careful about is exactly what q5b warned: case-of-case is shape-directed, and
adding it changes reduction for *every* program, not just the ones with sums. That is the
measurement to take before committing — **does adding it change any existing residual?** Six
gauntlet programs and a corpus of sieves say whether it is free.

## 9. What to measure

1. **Does case-of-case change any existing residual?** Run the whole corpus with and without. If
   nothing changes except sums getting shorter, the rule is safe. If residuals grow, the join-point
   answer has to come with it rather than after.
2. **The niche, per target.** `(option ptr)` as a NULL test against a tagged struct, on all four.
   §2.2 predicts the niche is free and the tag is not; predictions in this repository have been
   wrong about half the time.
3. **Go's struct-with-tag against Go's interface**, because the difference is *allocation* and
   getting it backwards would cost everything.
4. **A JSON node as a flat non-recursive sum**, which is §4's claim and is also the first program in
   this project that would look like an application rather than a kernel — which
   [assessment-2026-08-20 §5](assessment-2026-08-20.md) has wanted since before any of this.
