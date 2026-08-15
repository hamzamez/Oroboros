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
