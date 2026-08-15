# Inventory: every word an `.oro` file can contain

The standard is *no word used in an `.oro` file should go unexplained or unspecified*. This is the
audit against it, taken mechanically from `targets/*.oro`, `examples/*.oro`, `core/read.go` and
`emit/target.go` rather than from memory.

**Result: of 41 words, 16 are specified.** The rest are described in code comments, in measurement
documents, or nowhere. Four are outright **wrong** — a declared type that no program obeys, and
three gaps that make a whole class of program inexpressible.

---

## 1. The four findings

### 1.1 `fold-range` declares a type that is false

```lisp
(prim fold-range (f64 int any) f64 loop pure)      ; targets/go.oro
```

```lisp
(fold-range (dict-empty) (slen ws) …)              ; examples/wordcount.oro
```

The accumulator is declared `f64` and word count passes a **dictionary**. This has been wrong in
all four target files since the loop existed, and it is invisible because argument types are used
only as inference *hints* and are never checked. The first thing a type checker does is reject
every target file we have.

`fold-range` is genuinely polymorphic: `z : A`, `n : int`, `f : (A, int) → A`, result `A`. That
cannot be written in a monomorphic table.

**Disposition.** Structural primitives are already *named in data and implemented in code*
([target.go](../../emit/target.go)). Their **types are equally structural**, so they belong in the
checker beside their emission, not in the table. Remove the type columns from `loop`, `loop2`,
`cond`, `let`; the table keeps the name, the kind, and purity.

### 1.2 There is no integer arithmetic, and it blocks a gauntlet program

Every arithmetic primitive is f64:

```lisp
(prim add (f64 f64) f64 expr "%s + %s" pure)
```

`fold-range` binds `i : int` and **nothing can be computed with it.** `(aindex a (sub i 1))` is
inexpressible.

That is not a theoretical gap. [g7](../derivations/g7-aliasing.md)'s stencil is
`src[i-1] + src[i] + src[i+1]`; it exists as hand-written Go in `gauntlet/go/aliasing.go` and has
**no `.oro` file**, because it cannot be written. Of the gauntlet's programs, this is the one the
language cannot express at all.

### 1.3 There is no boolean logic

`bool` is a declared type. The only things that produce one are `lt` and `gt`, both on f64. There
is no `true`, no `false`, no `and`, no `or`, no `not`, and no `=`.

So `if` can branch on exactly one thing: a comparison of two floats. `examples/filter.oro` is the
only program with a conditional and it is `(gt x 0.0)`.

### 1.4 The type names have no owner

`Types` maps our name to the target's spelling — `f64` → `float64` → `double` → `double*`. But
nothing says whether `f64` is a **language** concept or a **target** concept. `targets/js.oro`
declares none at all. `any` and `none` are in the same table and are not types: `none` means *this
primitive takes no arguments*, `any` means *no constraint*.

## 2. The inventory

**Specified** means a document states what it means and what every target must do with it.

### Program forms — the language proper

| word | status | where |
|---|---|---|
| `def` | ✅ | [def.md](def.md) |
| `fn`, `λ` | ✅ | [core-0.md](core-0.md) |
| `let` | ✅ sugar | [def.md](def.md) |
| `seq` | ✅ sugar | [effects.md §5](effects.md) |
| `module`, `use`, `as`, `export` | ✅ | [modules.md §3](modules.md) |

Nine of nine. This half is in good order, which is what happens when the spec is written first.

### Target-file forms

| word | status | where |
|---|---|---|
| `pure` | ✅ | [effects.md §3](effects.md) |
| `import` | ⚠️ code comment | `emit/target.go` |
| `target` | ⚠️ code comment | `emit/target.go` |
| `type` | ⚠️ code comment | `emit/target.go` |
| `prim` | ⚠️ code comment | `emit/target.go` |
| `module` (in a target) | ✅ | [modules.md §4](modules.md) |
| `none` | ❌ | nowhere — a comment says "a nullary primitive writes `(none)`" |

### Primitive kinds

| word | status | note |
|---|---|---|
| `expr` | ⚠️ | comment in `targets/go.oro` |
| `stmt` | ⚠️ | its whole contract — *the value of the term is argument 0* — exists only in that comment, and **no backend implemented it** until `print-line` forced the issue |
| `loop`, `loop2`, `cond`, `let` | ❌ | named in data, implemented in code, contract stated nowhere |

### Type names

| word | status |
|---|---|
| `f64`, `int` | ⚠️ [ADR 0003](../decisions/0003-range-typed-integers.md) decides the *principle*; no spec says what `int` is here |
| `bool` | ❌ — see §1.3 |
| `string` | ✅ [strings.md](strings.md) |
| `vec-f64`, `vec-string`, `dict` | ❌ — opaque handles, named nowhere |
| `any` | ❌ — "absence of a constraint", recorded only in a measurement document |

### Primitives

| | status |
|---|---|
| `add sub mul lt gt` | ❌ meaning assumed; f64-only, and nobody wrote that down |
| `alen aindex slen sat` | ❌ |
| `split-words` | ❌ — and **measured to disagree across targets** ([modules.md §8](modules.md)) |
| `dict-empty dict-inc` | ❌ — and both are *effectful*, recorded in [effects.md §1](effects.md) |
| `sqrt` | ❌ |
| `print-line` | ✅ Tier 2 | [effects.md §6](effects.md) |
| `if fold-range fold-range2` | ❌ |
| `dot` | ❌ — demonstration only |

## 3. What must be promoted, and why the type system decides it

The proposed checker needs boolean connectives and linear integer arithmetic to write a `where`
clause. §1.2 and §1.3 say we need exactly those things anyway, for programs. That coincidence is
the useful part:

> **The predicate language should be a decidable *fragment of the term language*, not a second
> language.**

`(< i (alen v))` is an ordinary term of type `bool`. A `where` clause holds a term. There is no
predicate syntax, no separate grammar, nothing new to learn, and nothing that can look out of
place — a refinement is a boolean expression the checker happens to be able to decide.

Decidability is then a **syntactic restriction on which terms may appear in a `where`**: names,
integer literals, `+ - *`-by-constant, comparison, `and or not`, and length. Everything else is
rejected from refinements while remaining legal in programs.

So the promotions are forced twice over, which is the strongest kind of argument this project
accepts:

| promote | needed by a program | needed by the checker |
|---|---|---|
| integer `+ - *` | §1.2 — g7's stencil | linear arithmetic in refinements |
| comparison `< <= = >= >` on int | §1.2 | the entire content of a bounds refinement |
| `true`, `false`, `and`, `or`, `not` | §1.3 — any conditional not on floats | boolean structure of a `where` |
| `=` on f64 and string | today there is no equality at all | `(= (alen a) (alen b))` |

Nothing else is promoted. In particular **no data structures, no error model, and no
polymorphic containers** — those are libraries, and the point of having modules is that they can
be.

## 4. Decisions to take

Each needs an answer before the specification can be written. Stated as questions rather than
answers, because they are the argument, not its conclusion.

1. **Do type names belong to the language or the target?** Proposal: **the language owns the
   names, the target owns the spellings**, and a target must spell every type it uses in a
   declaration. `targets/js.oro` declaring none becomes "JS spells them all as nothing", which is
   already what the code does.
2. **Is `int` ADR 0003's range-typed integer, or a machine word?** ADR 0003 says mathematical
   semantics with machine representation. Nothing implements it. If `int` is refined, this is the
   same question as the type system's, and answering it twice would be a mistake.
3. **Are `any` and `none` types?** Proposal: no. Move them out of the type table — `none` is arity
   zero, `any` is the absence of a constraint. Both are *target-file grammar*, not types.
4. **Do structural primitives carry types at all?** Proposal: no (§1.1). Their types live in the
   checker with their emission.
5. **What is `vec-f64`?** Today an opaque handle. Under a type system it is presumably
   `(vec f64)` — which introduces type *constructors*, and that is a real addition. Deferrable: it
   can stay an opaque name until a program needs `(vec int)`.
6. **Does `=` on f64 mean IEEE equality?** It must, and then `(= NaN NaN)` is false and `=` is not
   an equivalence relation — which matters the moment it appears in a refinement.

## 4a. Answers so far

### Q1 — can we construct new types? **Yes, and we already do. `fn` is the type constructor.**

Three needs were conflated in the question, and they have different answers.

**Naming a target's type.** `(type dict "map[string]int")` is already "adding a type the way we add
an API function" — a line of data, unlimited, and the language never looks inside. This scales to
ten thousand types exactly as primitives do. **We do not *support* a target's types; we *name*
them.**

**Constructing a type in the language.** Already happening, and it costs nothing:

```lisp
(def vec (fn (n f) (fn (sel) (sel n f))))     ; a product type
(def vlen   (fn (v) (v (fn (n f) n))))        ; its first projection
(def vindex (fn (v i) ((v (fn (n f) f)) i)))  ; its second
```

That is a Scott encoding, and β/δ **erase it entirely** — `dot`'s residual contains no trace of
`vec`. So the core needs no type-construction form: **λ is the data constructor and the type
constructor at once**, and specialisation is what makes it free. What is missing is a *name* for
such a type and a way to *check* it, which is the checker's job, not the core's.

**A type that must exist at runtime and is not the host's.** This is the one we should refuse.
An encoding is free exactly while reduction erases it; when it survives it becomes a closure, which
all three backends reject and which would allocate. So:

> **User-defined types are free precisely when they are erased, and impossible when they are not.**

Which is [g5 §1](../derivations/g5-bindings.md) yet again — free in the interior, fixed at the
boundary.

And the apparent gap closes without new mechanism: a *portable record* is a **module** whose
members each target provides natively. `(module data/point)` with `make-point`, `point-x`,
`point-y` — Go supplies a struct, JS an object, Java a class, and any target without one gets a
fallback. `P_T ∩ D`, again.

### Q2 — should the principal type be a byte? **The insight is right; the conclusion inverts the project.**

The insight — *what determines the type of data is the operations we perform on it* — is not only
right, **it is already implemented**. `emit/golang.go`'s `inferFrom` "walks the term assigning
types to variables from the signatures of the primitives they are passed to". A value is `vec-f64`
because `alen` was applied to it. That is exactly the stated principle, and it is why
`targets/js.oro` can declare no types at all.

But *operations determine the type* does not entail *the representation is bytes*. Three reasons
the second half fails here, and the first is fatal:

1. **Two of three targets have no bytes.** JavaScript has no integer and no byte; a `Number` is a
   float64, and `Uint8Array` is an object rather than a value. CLAUDE.md's rule — *never make the
   core a superset of one host* — rules this out directly. A byte-oriented core is expressible on
   Go and C and **not on JS**.
2. **Bytes expose exactly what we survive by hiding.** [strings.md §3](strings.md)'s whole
   strategy is that representation is never observable. Bytes make endianness, width and alignment
   observable everywhere, on hosts that disagree about all three.
3. **It is the Shen wall by another road.** If the substrate is bytes, `add` on floats means
   *reinterpret eight bytes, add, write back*. On JS that requires a typed array — an allocation
   and a memory model — which is a performance ceiling on every target at once, set in the
   substrate where no host optimiser can undo it.

The sharpest way to put it:

> **A byte-oriented core picks the *lowest* representation every target shares.
> [ADR 0002](../decisions/0002-capability-graph.md) says emit at the *highest* layer the target
> natively provides.** They are exact opposites.

So: keep the insight as the *inference rule* it already is, and let it choose the host's **best**
representation rather than its lowest.

### Q4 — restated, since the question was unclear

There are two kinds of primitive. `expr` and `stmt` are pure data — a name, argument types, a
result type, a template. `loop`, `loop2`, `cond` and `let` are **named in data but implemented in
Go**, because a loop binds variables and emits a header and no template expresses that.

The question is whether a structural primitive's **type** can be written in that table. It cannot:
`fold-range` is `A × int × ((A, int) → A) → A`, which needs type variables and function types — a
whole type language in a target file, for four primitives, none of which a target author may add
anyway.

**Proposal:** since these four are implemented in code, their *types* live in code too. The table
keeps name, kind and purity. The checker knows `fold-range`'s typing rule the same way the emitter
knows its emission rule. This changes nothing about who can do what — target authors already
cannot add structural primitives — and removes a false statement from four lines in every target
file.

## 4b. What may appear in a `where` — classify, do not restrict

"A syntactic restriction on which terms may appear" was the wrong framing, and too crude. The
criterion is not syntax; it is **which theory the solver decides**, and the syntax is downstream of
that.

### The options, weighed

| theory | adds | decidable | cost | buys |
|---|---|---|---|---|
| QF-LIA | `+ -`, `*` by a literal, `< =`, `and or not` | yes | microseconds | every bounds obligation |
| **QF-UFLIA** | **+ uninterpreted functions and predicates** | **yes** | **microseconds** | **`alen`, and unlimited opaque vocabulary — `sorted?`, `ascii?`** |
| QF-AUFLIA | + the theory of arrays | yes | more | facts about *elements*, not just indices |
| + nonlinear | `x*y`, both variables | **no** over the integers (Hilbert's 10th) | — | little we need |
| + quantifiers | `∀i. 0≤i<n ⇒ …` | incomplete in practice | unpredictable | whole-array properties |

**QF-UFLIA** is the recommendation: decidable, fast, covers every bounds obligation, and its
uninterpreted half is precisely the mechanism that lets the predicate vocabulary grow without
limit ([types-sketch §3](../types-sketch.md)).

### The rule that removes the restrictiveness

The concern is right, and the answer is to stop restricting *what may be written* and instead
classify *what can be decided*:

> **Any boolean term may appear in a `where`. A term inside the decided fragment is **proven**. A
> term outside it is treated as an **opaque atom** — propagated, matched by name, never decided.**

Consequences, and they are all good:

- **Nothing is rejected for being too expressive.** You may write anything of type `bool`.
- **It is always sound.** An undecided term is not assumed true; it is an obligation that can only
  be discharged by a matching `ensures`, or by a runtime check at a boundary.
- **It degrades gracefully.** More solver power means more terms move from *propagated* to
  *proven*, and **no program has to change**.
- **The fragment can grow later.** Adding the array theory, or a proof layer, strictly increases
  what is proven and invalidates nothing already written — the same migration property established
  for the proof layer in [types-sketch §8](../types-sketch.md).

The one thing this requires is a **diagnostic**, not a rule: the compiler must be able to say
*"`(< i (alen q))` was propagated, not proven, so the checked form was selected"*. Without that,
a refinement silently doing nothing is indistinguishable from one doing its job.

## 5. Order

1. **Decide §4.** Nothing below is safe to write until §4.1–§4.4 are settled.
2. **Specify and promote §3** — integers, booleans, comparison, equality. One document, and it
   closes §1.2 and §1.3 and unblocks g7.
3. **Fix §1.1** — remove the false types from structural primitives.
4. **Specify the target-file grammar** — every ⚠️ and ❌ in §2's second and third tables. This is
   the file format third parties write, so it is the one that most needs to be a specification
   rather than a comment.
5. **Specify the existing primitives.** Each one needs the three-question test from
   [state.md §6](state.md). `split-words` will fail it, which is already known.
6. Only then, syntax for `sig` and `where`.
