# Inventory: every word the compiler knows

The standard is *no word the compiler knows should go unexplained or unspecified*. This is the audit
against that standard, and it has two layers, because a source file and the term below it are written
in different languages:

1. **J's words**, what a source file contains. A `.oro` file is J ([j.md](j.md),
   [ADR 0068](../decisions/0068-j-is-the-surface.md)), and its keywords are j.md §1.4's: `fn`, `set`,
   `var`, `loop`, `case`, `try`, `where`, `ensure`, `linear`, `axiom` and the rest. **§0 below.**
2. **The core's forms**, what J's reader makes. The reader desugars J by D into the core term
   ([abstract.md](abstract.md) §8, `core/desugar.go`), and a target file into the loader's forms
   (`core/jreadtarget.go`); the module loader, the reducer and the target loaders dispatch on their
   heads. Most of these words are spelled by no source file: **`def` and `sig` are D's forms for a
   `fn` header and its claims**, `let` and `tuple` for a block and a pair, `local` for `var`. Each row
   says how J writes it. **§1–§5 below.**

> **Both layers are pinned by tests, and neither can go stale silently.**
> - `TestTheInventoryIsTheWordsTheCompilerKnows` (`emit/inventory_test.go`) reads the core's words out
>   of the compiler **mechanically**: every literal a form's head is dispatched on in the readers, the
>   module loader and the target loaders, and the compiler's word tables (injected names, respelled
>   forms, clause words, type formers, operators, backends, `lang`'s variant). It requires §1–§4's
>   tables to list **exactly that set**, and the counts below to be the rows' counts. **A row marked
>   `specified` must link a spec that mentions the word in code**, a necessary condition the test
>   checks for a sufficient one the row's reader judges.
> - `TestJsKeywordsAreTheReadersKeywords` (the same file) reads J's keywords out of J's readers (every
>   word a token is compared against) and requires j.md §1.4's lists to be **exactly that set**, less
>   the words it names as not built.
> - `TestTheInventoryCheckFails` shows each rule of both failing against a planted mistake.
>
> The audits of 2026-08-25 and before were taken by hand, and each was stale within a week. This one's
> counts were stale from the J switch (2026-10-16) to 2026-10-25, when the test began to read them.

**What is not a word.** A file also contains a lexical layer neither test reaches, because none of it
is dispatched on: the token classes, and **comments**, which are gap rather than tokens
([comments.md](comments.md), [ADR 0024](../decisions/0024-comments-are-erased.md), j.md §1.2). It is
tested by `core/comment_test.go` and the J reader's tests.

**Result: 168 words of the core**, and j.md §1.4's keywords.

| status | count |
|---|---:|
| `specified` — a spec states it | 128 |
| `recorded` — only research, a result document or code comments | 25 |
| `reserved` — refused, and specified as owed | 4 |
| `refused` — an old spelling, refused naming the new one | 11 |
| `undocumented` | 0 |

A row carries one status, and a count is of words, not rows. What is left is §6.

Statuses, as the test reads them:
- **`specified`**: a document under `docs/spec/` or `docs/decisions/` says what the word means, and the
  row links it.
- **`recorded`**: explained only in a result document or a code comment. It is true and findable, but
  it is not a specification.
- **`reserved`**: the compiler refuses the word so it cannot be taken for something else before its
  construct is built.
- **`refused`**: a former spelling, refused with a message naming its replacement.
- **`undocumented`**: nowhere.

---

## 0. J's words

j.md §1.4 is the list, and it is not repeated here: **reserved** words (`fn spec lemma set array map var
loop again case if else try expect assert use module provides pub export axiom true false`),
**contextual** ones, keywords only where the grammar takes them (`where ensure invariant pure index
variadic inout out linear as host expr stmt import lib checked jump comma_ok success relevant`), and
**the model's data words** of a target file (`repr`, `fact`, `backend`, …, j.md §2.12).

Three of them are not built, and are named in j.md's status: `spec`, `assert` and `invariant`. And the
reservation itself is not: J's reader takes a word as a keyword only where its grammar meets it, so
`set` or `map` is accepted as a parameter's name.

A J keyword is a core word when D passes it through (`case`, `try`, `expect`, `loop`, `again`, `use`,
`module`, `where`, `pure`), and its row is below. One that D erases has no row: `var` becomes `local`'s
binder, `axiom` an `implements`, `ensure` an `ensures`, `linear` a `buffer`.

---

## 1. The core's forms, and how J writes each

From `core/read.go`'s forms (the loader's and the raw reader's, which reads a core term and the IR's
text), D's output, and the module loader's dispatch.

### Declarations

| word | status | where |
|---|---|---|
| `def` | specified — a definition, def(f; λ), a named λ (ADR 0051); J writes `fn f(x) { e }`, or `f = e` for a value, and D makes the `def` | [def.md](def.md), [program-surface.md](../program-surface.md) |
| `sig` | specified — a definition's claim, its λ's sets, `where` and `ensures`; J writes them in the `fn` header, `fn f(x ∈ S) (R) where φ ensure ψ`, and D makes the `sig` | [types.md](types.md), [refinements.md](refinements.md) |
| `where` | specified — a precondition, on a program's header and a target's; J's `where φ` | [refinements.md](refinements.md) |
| `ensures` | specified — a postcondition; J's `ensure ψ` | [postconditions.md](postconditions.md) |
| `result` | specified — the value, named inside `ensures`; J names the result in the header, `(r ∈ U)` | [postconditions.md](postconditions.md) |
| `inplace` | specified — a host declaration's threaded-result component that is a borrowed buffer handed back as its own storage, `(inplace j i)`, derived by J's target reader from a header naming no `p'` for a `linear` p; never written by hand | [ADR 0070](../decisions/0070-a-header-is-the-callers-view-of-a-borrow.md) |
| `variant` | specified — closed, finite, non-recursive, with type arguments; J writes `set S(T) = 'a ∪ 'c(T)` | [sums.md](sums.md), [data.md](data.md) |
| `success` | specified — a variant's role marker, `(success c)`, J's `success 'c` after the union: the constructor `try` continues on; reserved as a constructor name | [errors.md](errors.md) §2 |
| `relevant` | specified — a variant's role marker, `(relevant)`, J's `relevant`: a value of the type must be used; reserved as a constructor name | [errors.md](errors.md) §2, §7 |
| `module`, `use`, `export` | specified — the same words in J; `export` the entry points (ADR 0078); `module` in a target too | [modules.md](modules.md) |
| `pub` | specified — a module's interface, `(pub n…)`, J's `pub` marker: the names an importer may use; apart from the entry points | [ADR 0078](../decisions/0078-a-modules-interface-is-what-it-marks-pub.md) |
| `as` | specified — `use PATH as ALIAS` | [state.md](state.md) |
| `provides` | specified — a target fragment written where a library lives, which may hold definitions; the same word in J | [target-system.md](target-system.md), [theories.md](theories.md) |
| `target` | specified — a target file's header in the loader's form; J's target file names its module by `module`, and a program writing `target` is refused | [target-files.md](target-files.md) |

### Terms

**Nothing of the core's sugar is J**: J's reader makes each of these, and `case` is expanded in
`Load`, which needs a variant declared in another file ([state.md](state.md)).

| word | status | where |
|---|---|---|
| `fn` | specified — the λ, the only binder of the core; J's `fn(x) { e }` | [core-0.md](core-0.md), [state.md](state.md) |
| `let` | specified — the flat binding form, nested applications; J writes a block, `{ x = e; b }`; injected, and a structural kind | [binding.md](binding.md), [def.md](def.md) |
| `and`, `or`, `not`, `cond` | specified — sugar for `if`; J's `&&`, `\|\|`, `!` and `case { φ -> a; else -> b }`; `cond` is also a structural kind | [booleans.md](booleans.md) |
| `tuple` | specified — `(fn (k) (k a b …))`, the tuple type, and a binding's left-hand side; J's `(a, b)`, `()` the unit, typed `prod()` | [data.md](data.md) §3.6, [binding.md §5](binding.md) |
| `when`, `else` | specified — clause words of `loop`, which `match` was sugar for; J's arms `φ -> e` and `else -> e` | [match.md](match.md) |
| `store` | specified — `store(b, i, v)`, the three-argument `set` on a threaded buffer, a transitional J spelling no corpus file uses and three tests of the core write | [j.md §7](j.md) |
| `×` | specified — the product of sets, TLA+'s, n-ary in a chain and a component when parenthesized (ADR 0065) | [j.md](j.md) |
| `case` | specified — the same word in J; expanded in `Load` | [sums.md](sums.md) |
| `try` | specified — the exception monad's bind, J's `try p = e { body }`, `(try (ok x) e body)`; expands to `case` in `Load`, on a variant marked `success` | [errors.md](errors.md) |
| `expect` | specified — total, J's `expect p = e else why`, `(expect (ok x) e why body)`: continues on the marked success and crashes with `why` on every other constructor, through `abandon`; expands to `case` in `Load` | [errors.md](errors.md) §8 |
| `spread` | specified — `(spread xs)`, J's `f(xs...)`, passes a table as a variadic call's list; the sole argument after the fixed ones; rewritten by the loader, so nothing below it sees one | [variadic.md](variadic.md) |
| `ignore` | specified — !_A : A → 1, `ignore(e)`: runs e's effects and has the unit as its value, the one way to discard a value of a `(relevant)` type; erased by the reducer (the eliminator with a constant body, by the terminal object's universal property), so no target sees it | [errors.md](errors.md) §7.3 |
| `loop`, `again` | specified — the same words in J; also the retired layer's structural kind `loop` | [ADR 0015](../decisions/0015-loop-and-again.md), [iteration.md](iteration.md) |
| `true`, `false` | specified — the boolean literals | [booleans.md](booleans.md) |

### The type language

J names the base sets `Int`, `F64`, `Bool`, `String` and the formers `Array(V)` and `Map(K, V)` (j.md
§1.5); the reader is case-blind there and gives the core's names below.

| word | status | where |
|---|---|---|
| `int` | specified — and `(int LO HI)`, a range, J's `LO..HI`; also the subject of `repr LO..HI …` | [integers.md](integers.md) |
| `f64` | specified | [arithmetic.md](arithmetic.md) |
| `bool` | specified | [booleans.md](booleans.md) |
| `string` | specified | [strings.md](strings.md) |
| `array` | specified — a table type, J's `Array(V)`, and the injected graph constructor, J's `array { … }` | [tables.md](tables.md) |
| `map` | specified — a type, J's `Map(K, V)`, the injected constructor, and `repr map …` | [maps.md](maps.md) |
| `buffer` | specified — a linear parameter type, J's `b ∈ Array(V) linear` | [ADR 0020](../decisions/0020-uniqueness-on-parameters.md) |
| `cell` | specified — `(cell T)` in a declaration's parameter, J's `x ∈ T inout`: the address of a T the host may read and write during the call and does not keep; `(cell go.Value)` a cell of any type in Go's box. Also a target's `(type (cell A) (host …))` | [local.md](local.md) §5 |
| `record` | reserved — a type former in `core/read.go`, specified in data.md and not built | [data.md](data.md) |
| `prod` | recorded — the internal spelling of a tuple type (`prod(A, B)`), J's `A × B`, reserved as a type former so no variant can take the name | `core/read.go` `TypeName` |

### Range endpoints

An endpoint is a compile-time integer expression, evaluated and never emitted.

| word | status | where |
|---|---|---|
| `+inf`, `-inf` | specified — an unbounded endpoint, the core's; J omits the end, `lo..` and `..hi`, and refuses `` `+inf` `` (ADR 0077) | [ADR 0019](../decisions/0019-precision-by-declaration.md), [unbounded-rung.md](../unbounded-rung.md) |
| `pow` | specified — J's `^` on constants | [ADR 0019](../decisions/0019-precision-by-declaration.md), [unbounded-rung.md](../unbounded-rung.md) |

`+`, `-` and `*` are also endpoint operators; their rows are in the injected names below.

### Names the compiler injects into every target

A target may not declare one, and declaring one is an **error**.

| word | status | where |
|---|---|---|
| `if` | specified — the same word in J | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md), [booleans.md](booleans.md) |
| `=` | specified — integer equality only; J's `==` | [match.md](match.md) |
| `+`, `-`, `*`, `/`, `%`, `<`, `<=`, `>`, `>=` | specified — found per target by spelling | [integers.md](integers.md) |
| `&`, `\|`, `⊕`, `~`, `<<`, `>>`, `mod` | specified — found per target by spelling; on JavaScript per result window (ADR 0082) | [integers.md](integers.md) §0b |
| `table`, `len` | specified — `table`, a table by its rule, J's `array { e : k ∈ 0..n-1 }`; `len` the same in J | [tables.md](tables.md) |
| `alloc`, `set` | specified — `set`, a store into a live buffer, J's `b[i] := v` | [tables.md](tables.md), [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) |
| `local` | specified — a scope of local state, J's `var x := e` in a block: a binder initialized by a table is a buffer; by anything else, a cell, written in program order and translated into loop variables at load, so nothing below the loader sees one | [local.md](local.md) |
| `out` | specified — `(out τ)`, J's `out T` as an argument, at a cell parameter: a cell for that one call, holding τ's zero; the call's value is the cells' values, then the declared result's components. Anywhere else an ordinary application | [local.md](local.md) §6 |
| `#ref`, `#fresh`, `#deref` | specified — the cell translation's own: a reference to a fresh copy of a value, one to a type's zero, and the value a reference holds; impure, injected into every target, and out of a program's reach | [local.md](local.md) §5 |
| `build` | specified — the scoped buffer's core form `(build n (fn (b) …))`, made from J's `var`; also a target file's `build "cmd"` and a retired structural kind | [tables.md](tables.md), [build.md](build.md) |
| `build-map`, `insert`, `keys` | specified — `build-map` made from J's `var m := map(cap)`, `insert` from `m[k] := v` | [maps.md](maps.md) |
| `concat`, `string-of` | recorded — the free monoid's operation and its generator, J's `++` and `string_of`; derived in research and built, and no spec in `docs/spec/` states them | [string-operations.md](../string-operations.md), [render-2026-09-04](../../gauntlet/results/render-2026-09-04.md) |
| `the` | recorded — a range ascribed to a term, erased at emission; J refuses `the`, and writes a binder's claim `b ∈ S = e` (ADR 0069); the construct owes a spec | [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md), [inlining-and-declarations.md](../inlining-and-declarations.md) |

**Indexing has no word at all.** `t[i]` is an application, because a table *is* a function with a
known finite domain ([tables.md](tables.md)).

### `lang`'s declarations

| word | status | where |
|---|---|---|
| `option`, `some` | specified — one declaration, in `lang`; J's tags `'some(v)` and `'none` | [maps.md](maps.md), [data.md](data.md) |
| `none` | specified — `option`'s other variant, and a nullary sig's argument list in a target file | [maps.md](maps.md), [target-files.md](target-files.md) |

---

## 2. Target-file forms

From `core/jreadtarget.go`, which reads a target file's J into these forms, and `emit/target.go`,
`emit/fact.go` and `emit/constend.go`, which load them. The whole grammar is
[target-files.md](target-files.md), the file a third party writes; its algebra is
[theories.md](theories.md).

### Forms

| word | status | where |
|---|---|---|
| `type` | specified — a host type, a manifest type, or a constructor's realization; J's `set N = host "s"` or `set N = S` | [target-files.md](target-files.md), [theories.md](theories.md) |
| `const` | specified — a pure zero-argument sig with a singleton range; J's `N ∈ set { v } = host "s"` | [target-files.md](target-files.md) |
| `host` | specified — the only clause carrying host text, J's `{ host expr "t" }`; also `repr big host`, `repr map host` | [target-files.md](target-files.md) |
| `repr` | specified | [target-files.md](target-files.md), [theories.md](theories.md) |
| `fact` | specified — `fact max_len …` in a target, and `lang`'s facts | [target-files.md](target-files.md), [theories.md](theories.md) |
| `implements` | specified — checked as a view; J's `axiom T ⊆ I` | [target-files.md](target-files.md), [theories.md](theories.md) |
| `underlying` | specified — a host fact the hand-declaration checker reads; the compiler ignores it | [target-files.md §2a](target-files.md) |
| `include` | specified — theory inclusion between companions | [theories.md](theories.md) |
| `structural` | specified | [target-files.md](target-files.md) |
| `backend` | specified | [target-files.md](target-files.md) |
| `artifact` | specified | [build.md](build.md) |
| `data` | specified | [windows-target.md](windows-target.md) |
| `link` | specified | [target-files.md](target-files.md) |

### Header and host clauses

| word | status | where |
|---|---|---|
| `pure` | specified | [effects.md](effects.md) |
| `variadic` | specified — the host function takes a list as its last parameter, f : X × T* → B: the parameter a table, the template spreading it (`%s...`); a call's trailing arguments are the list | [variadic.md](variadic.md) |
| `cells` | specified — the loader's form for a declaration's cell parameters, `(cells ARITY LIST i…)`, handed from the target as `variadic` is | [local.md](local.md) §5 |
| `borrows` | specified — the loader's form for a declaration that borrows a buffer, `(borrows RESULTS i…)`: its `(buffer V)` parameters, handed back first, and its result's other components, handed from the target as `cells` is | [local.md](local.md) §2.1 |
| `index` | specified | [target-files.md](target-files.md) |
| `import` | specified | [target-files.md](target-files.md) |
| `lib` | specified | [target-files.md](target-files.md) |
| `checked` | specified | [target-files.md](target-files.md) |
| `jump` | specified | [windows-target.md](windows-target.md) |
| `comma-ok` | specified — J's `comma_ok`: the host gives a declared `(option E)` as Go's pair (v, ok): H = [E, bool], r(e, b) = `some e` if b, else `none` | [typevalues.md](typevalues.md) §7 |
| `subsets` | specified — a static parameter's set in a declaration's head, `(E (subsets T))`, J's `E ⊆ T`, ranging over 𝒫(⟦T⟧); abstract.md's Σ₀ constant | [typevalues.md](typevalues.md) §4 |

### Kinds

What a declaration's `host KIND …` or a `structural NAME KIND` may say.

| word | status | where |
|---|---|---|
| `expr`, `stmt` | specified — a statement's value is its first argument | [target-files.md](target-files.md) |
| `loop2` | specified — the retired portable layer's two-accumulator fold, kept for old benchmarks | [target-files.md](target-files.md) |
| `iterate` | recorded — the structural kind of `loop`; named only in `emit/target.go`, because the kind `loop` was already taken by `fold-range` | `emit/target.go` `structuralKinds` |

`let`, `cond`, `loop` and `build` are kinds as well; their rows are in §1.

### `repr` subjects and choices

| word | status | where |
|---|---|---|
| `ref` | specified — `(repr (ref T) (host …))`, a boxed representation | [target-files.md](target-files.md), [theories.md](theories.md) |
| `big` | specified — `repr big host` or `repr big limbs` | [target-files.md](target-files.md) |
| `limbs` | specified | [target-files.md](target-files.md) |
| `library` | specified — `(repr map library)`: a map written in Oroboros | [target-files.md](target-files.md) |
| `shift` | specified — `(repr shift N)` | [theories.md](theories.md) |
| `narrow` | specified — `(repr narrow (host …))` | [theories.md](theories.md) |
| `read`, `write` | specified — `repr LO..HI host "T" read "…" write "…"`, a host type's second reading (ADR 0083) | [integers.md](integers.md) §0b |
| `pair` | specified — `repr 0..2^64-1 pair`, the unsigned word as two 32-bit digits of a narrower word (ADR 0083, `ir/pairs.go`) | [integers.md](integers.md) §0b |
| `wrap` | specified — `(repr wrap W)`, the width a target's operators wrap at (ADR 0082) | [integers.md](integers.md) §0b |
| `niche` | specified — `(repr TYPE (niche (host expr "…")))`: the host type is 1 + E, its absent point inside its own values; a fallible declaration's failure is read through it | [errors.md](errors.md) §4.1 |
| `word` | specified — `repr LO..HI word`, the target's word | [ADR 0026](../decisions/0026-an-int-is-an-integer.md) |

### Backends

| word | status | where |
|---|---|---|
| `go`, `js`, `java`, `x86-64` | specified — the closed set of backends | [target-files.md](target-files.md) |
| `windows` | recorded — **a finding, §6**: `WriteProgram` lays out a program's build tree by the target's NAME (`go`, `js`, `java`, `windows`), not by its backend | `emit/target.go` `WriteProgram` |

### Names a target declares for the compiler to find

The integer operators and `=` above are found by spelling in the same way. These are `abandon`, and the arbitrary-
precision names the host rung needs: the target declares them, and the compiler selects and emits them.

| word | status | where |
|---|---|---|
| `abandon` | specified — crash : E → 0, which `expect` expands into: a statement taking the reason, injected unqualified and impure; each target's own crash (panic, throw, ExitProcess after the reason on standard error). Its value is in 0: it absorbs every strict context, and the IR ends its region with `unreachable` | [errors.md](errors.md) §8–9 |
| `big+` | specified | [target-files.md](target-files.md) |
| `big-fit` | specified — the bound enforced on a host bignum, [0, 2ᵏ) | [target-files.md](target-files.md), [ADR 0029](../decisions/0029-above-the-word-one-set-on-every-representation.md) |
| `big-fit-signed` | specified — the same for a signed range, (−2ᵏ, 2ᵏ) | [ADR 0029](../decisions/0029-above-the-word-one-set-on-every-representation.md) |
| `big-`, `big*`, `big/`, `big%`, `big<`, `big<=`, `big>`, `big>=`, `big=`, `big-of`, `big-str` | recorded — "`big+` and the rest" in target-files.md §3 names the family, not these words | [bigrep-2026-09-02](../../gauntlet/results/bigrep-2026-09-02.md) |
| `big+!`, `big-!`, `big*!`, `big/!`, `big%!`, `big-of!` | recorded — the in-place forms | [bigreuse-2026-09-02](../../gauntlet/results/bigreuse-2026-09-02.md) |
| `big%-small` | recorded — a remainder by a machine word, whose result is a word | [subdiv-2026-09-03](../../gauntlet/results/subdiv-2026-09-03.md) |
| `trap-if` | recorded — the fixed-width rung's overflow trap | [bigrepr-2026-09-03](../../gauntlet/results/bigrepr-2026-09-03.md) |

---

## 3. Reserved words

| word | status | where |
|---|---|---|
| `lemma` | reserved — F-E, a proved law: read in a target file and refused there; a program's `lemma` (ADR 0062) is not built | [theories.md](theories.md) |
| `forall` | reserved — a quantified fact (F-D); refused inside a `fact` | [theories.md](theories.md) |
| `prim` | reserved — a program may not declare a primitive; in a target it is `refused`, respelled `sig` (§4) | [state.md](state.md), [target-files.md](target-files.md) |

`record` (§1) is the fourth.

---

## 4. Refused spellings

Each is refused with a message naming the spelling that replaced it, in J. A refused word stays in the
compiler so the message can be given. Three are words J's target reader still takes as the model's
forms, so a target file can write them and meet the message: `int_repr`, `big_repr` and `max_len`.

| word | status | where |
|---|---|---|
| `sum` | refused — `variant`, J's `set S = 'a ∪ 'c(T)` | [data.md](data.md) |
| `int-repr` | refused — `repr LO..HI host "spelling"` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `big-repr` | refused — `repr big host` or `repr big limbs` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `shift-width` | refused — `repr shift N` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `max-len` | refused as a form — `fact max_len (a ∈ Array(A)) len(a) <= N` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `array-type` | refused — `set Array(A) = host "spelling"` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `map-type` | refused — `set Map(K, V) = host "spelling"` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `boxed` | refused — `repr ref(T) host "spelling"` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `builtin-map` | refused — `repr map library` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `length`, `length-of` | refused — `ensure len(r) == n` and `ensure len(r) == len(c)` | [lengthensures-2026-09-15](../../gauntlet/results/lengthensures-2026-09-15.md) |

---

## 5. Type names a target declares

Each target declares its own, and nothing here is portable, which is the point
([target-native.md](target-native.md)). They are **not** words of the language, so the test does not
read them:
- Go spells its integer types as manifest ranges (`int32` is `set int32 = -2^31..2^31-1`) and owns
  host types by module (`go/io.Writer`).
- The JVM and windows declare host spellings.
- JavaScript declares everything `any`.

`ptr`, which the last audit found undocumented, is windows' address type; it is a target type name
like the others and is declared in `targets/windows/`.

---

## 6. What is left

1. **The program's build tree is chosen by the target's NAME.** `WriteProgram` switches on
   `go`/`js`/`java`/`windows`, so a user target called anything else, with a perfectly good
   `backend go`, emits its code and is then refused by `cmd/build` with `target "mygo" has no
   program layout`. It is loud rather than wrong, and it is still the shape backend-2026-09-06 fixed
   for emission, left behind in the layout step: the layout belongs to the backend. Found by the
   2026-09-17 audit, not fixed.
2. **`the` has no specification.** It is a construct of the core, injected into every target, and
   only [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md) says what it means. J does
   not spell it (a binder's claim is the surface's, ADR 0069), so it is now the core's alone.
3. **The big-integer names are specified as a family, not as words.** target-files.md §3 says "`big+`
   and the rest". A target author declaring the host rung has to read three result documents to learn
   the other twenty names and their arities.
4. **`iterate` and `prod` are explained only in code.** Neither can be misused (`prod` is reserved,
   `iterate` is a kind), but a reader of a target file meets `iterate` and finds nothing.
5. **Specified and not in the compiler**, named so they are not mistaken for gaps in this audit:
   `quote`, `with` and `view` (theories.md), and records.
6. **`concat` and `string-of` are derived and built, and not specified.** string-operations.md derives
   them from the free monoid's universal property, and render-2026-09-04 built them on four targets;
   strings.md defers to that research rather than stating them.
