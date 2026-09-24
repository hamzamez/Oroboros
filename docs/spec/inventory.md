# Inventory: every word an `.oro` file can contain

The standard is *no word used in an `.oro` file should go unexplained or unspecified*. This is the audit
against that standard.

> **Retaken 2026-09-17, and it cannot go stale silently any more.**
> `TestTheInventoryIsTheWordsTheCompilerKnows` (`emit/inventory_test.go`) reads the words out of the
> compiler **mechanically**:
> - every literal a form's head is dispatched on in the reader, the module loader and the target
>   loaders;
> - the compiler's word tables: injected names, respelled forms, clause words, type formers, operators,
>   backends and `lang`'s variant.
>
> It then requires the tables below to list **exactly that set**. A word the compiler knows and this
> file omits fails the test, and so does a word listed here that the compiler no longer knows.
> **A row marked `specified` must link a spec that mentions the word in code** — a necessary
> condition, which the test checks, for a sufficient one, which the row's reader judges.
> `TestTheInventoryCheckFails` shows each of those rules failing against a planted mistake.
>
> The two previous audits (2026-08-25, and the portable layer's before it) were taken by hand, and each
> was stale within a week.

**What is not a word.** An `.oro` file also contains a lexical layer this audit cannot reach, because
none of it is dispatched on: the token classes, and **comments**, which are gap rather than tokens.
Comments were unspecified until 2026-09-20 and are now [comments.md](comments.md) and
[ADR 0024](../decisions/0024-comments-are-erased.md). The mechanical check tests words; the lexical
layer is tested by `core/comment_test.go`.

**Result: 136 words.**

| status | count |
|---|---:|
| `specified` — a spec states it | 95 |
| `recorded` — only research, a result document or code comments | 25 |
| `reserved` — refused, and specified as owed | 4 |
| `refused` — an old spelling, refused naming the new one | 12 |
| `undocumented` | 0 |

A row carries one status, so the counts sum to the total. What is left is §6.

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

## 1. Program forms

Everything a *program* may write. From `core/read.go`'s form and special-form dispatch.

### Top-level forms

| word | status | where |
|---|---|---|
| `def` | specified — and its shorthand `(def f (x…) body)` | [def.md](def.md), [program-surface.md](../program-surface.md) |
| `sig` | specified | [types.md](types.md), [refinements.md](refinements.md) |
| `where` | specified — a precondition, on a program sig and a target sig | [refinements.md](refinements.md) |
| `ensures` | specified — a postcondition | [postconditions.md](postconditions.md) |
| `result` | specified — the value, named only inside `ensures` | [postconditions.md](postconditions.md) |
| `variant` | specified — closed, finite, non-recursive, with type arguments | [sums.md](sums.md), [data.md](data.md) |
| `module`, `use`, `export` | specified — in a program, and `module` in a target | [modules.md](modules.md) |
| `as` | specified — `(use PATH as ALIAS)` | [state.md](state.md) |
| `provides` | specified — a target fragment written where a library lives, which may hold `def`s | [target-system.md](target-system.md), [theories.md](theories.md) |
| `target` | specified — a target file's header; a program writing it is refused | [target-files.md](target-files.md) |

### Term forms

**None of the sugar survives the reader**, except `case`, which needs a variant declared in another file
and so expands in `Load` ([state.md](state.md)).

| word | status | where |
|---|---|---|
| `fn`, `λ` | specified — the only non-sugar special form | [core-0.md](core-0.md), [state.md](state.md) |
| `let` | specified — the flat binding form, sugar for nested applications; injected, and a structural kind | [binding.md](binding.md), [def.md](def.md) |
| `seq` | specified — a binding whose name is discarded, `((fn (_) b) a)` | [binding.md §6](binding.md), [effects.md](effects.md) |
| `and`, `or`, `not`, `cond` | specified — sugar for `if`; `cond` is also a structural kind | [booleans.md](booleans.md) |
| `tuple` | specified — sugar for `(fn (k) (k a b …))`, the tuple type, and a `let`'s left-hand side | [data.md](data.md), [binding.md §5](binding.md) |
| `match`, `when`, `else` | specified — sugar for `loop` | [match.md](match.md) |
| `_` | specified — a pattern that binds nothing, and `seq`'s binder | [match.md](match.md) |
| `case` | specified — sugar, expanded in `Load` | [sums.md](sums.md) |
| `loop`, `again` | specified — also the retired layer's structural kind `loop` | [ADR 0015](../decisions/0015-loop-and-again.md), [iteration.md](iteration.md) |
| `true`, `false` | specified — the boolean literals | [booleans.md](booleans.md) |

### The type language

| word | status | where |
|---|---|---|
| `int` | specified — and `(int LO HI)`, a range; also the subject of `(repr (int LO HI) …)` | [integers.md](integers.md) |
| `f64` | specified | [arithmetic.md](arithmetic.md) |
| `bool` | specified | [booleans.md](booleans.md) |
| `string` | specified | [strings.md](strings.md) |
| `array` | specified — a table type, and the injected graph constructor | [tables.md](tables.md) |
| `map` | specified — a type, the injected constructor, and `(repr map …)` | [maps.md](maps.md) |
| `buffer` | specified — a linear parameter type | [ADR 0020](../decisions/0020-uniqueness-on-parameters.md) |
| `record` | reserved — a type former in `core/read.go`, specified in data.md and not built | [data.md](data.md) |
| `prod` | recorded — the internal spelling of a tuple type (`prod(A, B)`), reserved as a type former so no variant can take the name | `core/read.go` `TypeName` |

### Range endpoints

An endpoint is a compile-time integer expression, evaluated and never emitted.

| word | status | where |
|---|---|---|
| `+inf`, `-inf` | specified — an unbounded endpoint | [ADR 0019](../decisions/0019-precision-by-declaration.md), [unbounded-rung.md](../unbounded-rung.md) |
| `pow` | specified | [ADR 0019](../decisions/0019-precision-by-declaration.md), [unbounded-rung.md](../unbounded-rung.md) |

`+`, `-` and `*` are also endpoint operators; their rows are in §1's injected names.

### Names the compiler injects into every target

A target may not declare one, and declaring one is an **error**.

| word | status | where |
|---|---|---|
| `if` | specified | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md), [booleans.md](booleans.md) |
| `=` | specified — integer equality only | [match.md](match.md) |
| `+`, `-`, `*`, `/`, `%`, `<`, `<=`, `>`, `>=` | specified — found per target by spelling | [integers.md](integers.md) |
| `table`, `len` | specified | [tables.md](tables.md) |
| `alloc`, `set` | specified | [tables.md](tables.md), [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) |
| `build` | specified — the scoped buffer; also a target file's `(build "cmd")` and a retired structural kind | [tables.md](tables.md), [build.md](build.md) |
| `build-map`, `insert`, `keys` | specified | [maps.md](maps.md) |
| `concat`, `string-of` | recorded — the free monoid's operation and its generator; derived in research and built, and no spec in `docs/spec/` states them | [string-operations.md](../string-operations.md), [render-2026-09-04](../../gauntlet/results/render-2026-09-04.md) |
| `the` | recorded — a range ascribed to a term, erased at emission; the construct owes a spec | [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md), [inlining-and-declarations.md](../inlining-and-declarations.md) |

**Indexing has no word at all.** `(a i)` is an application, because a table *is* a function with a
known finite domain ([tables.md](tables.md)).

### `lang`'s declarations

| word | status | where |
|---|---|---|
| `option`, `some` | specified — one declaration, in `lang` | [maps.md](maps.md), [data.md](data.md) |
| `none` | specified — `option`'s other variant, and a nullary sig's argument list in a target file | [maps.md](maps.md), [target-files.md](target-files.md) |

---

## 2. Target-file forms

From `emit/target.go`, `emit/fact.go` and `emit/constend.go`. The whole grammar is
[target-files.md](target-files.md), the file a third party writes; its algebra is
[theories.md](theories.md).

### Forms

| word | status | where |
|---|---|---|
| `type` | specified — a host type, a manifest type, or a constructor's realization | [target-files.md](target-files.md), [theories.md](theories.md) |
| `const` | specified — sugar for a pure zero-argument sig with a singleton range | [target-files.md](target-files.md) |
| `host` | specified — the only clause carrying host text; also `(repr big host)`, `(repr map host)` | [target-files.md](target-files.md) |
| `repr` | specified | [target-files.md](target-files.md), [theories.md](theories.md) |
| `fact` | specified — `(fact max-len …)` in a target, and `lang`'s facts | [target-files.md](target-files.md), [theories.md](theories.md) |
| `implements` | specified — checked as a view | [target-files.md](target-files.md), [theories.md](theories.md) |
| `include` | specified — theory inclusion between companions | [theories.md](theories.md) |
| `structural` | specified | [target-files.md](target-files.md) |
| `backend` | specified | [target-files.md](target-files.md) |
| `artifact` | specified | [build.md](build.md) |
| `data` | specified | [windows-target.md](windows-target.md) |
| `link` | specified | [target-files.md](target-files.md) |

### Sig and host clauses

| word | status | where |
|---|---|---|
| `pure` | specified | [effects.md](effects.md) |
| `index` | specified | [target-files.md](target-files.md) |
| `import` | specified | [target-files.md](target-files.md) |
| `lib` | specified | [target-files.md](target-files.md) |
| `checked` | specified | [target-files.md](target-files.md) |
| `jump` | specified | [windows-target.md](windows-target.md) |

### Kinds

What a sig's `(host KIND …)` or a `(structural NAME KIND)` may say.

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
| `big` | specified — `(repr big host)` or `(repr big limbs)` | [target-files.md](target-files.md) |
| `limbs` | specified | [target-files.md](target-files.md) |
| `library` | specified — `(repr map library)`: a map written in Oroboros | [target-files.md](target-files.md) |
| `shift` | specified — `(repr shift N)` | [theories.md](theories.md) |
| `narrow` | specified — `(repr narrow (host …))` | [theories.md](theories.md) |
| `word` | specified — `(repr (int LO HI) word)`, the target's word | [ADR 0026](../decisions/0026-an-int-is-an-integer.md) |

### Backends

| word | status | where |
|---|---|---|
| `go`, `js`, `java`, `x86-64` | specified — the closed set of backends | [target-files.md](target-files.md) |
| `windows` | recorded — **a finding, §6**: `WriteProgram` lays out a program's build tree by the target's NAME (`go`, `js`, `java`, `windows`), not by its backend | `emit/target.go` `WriteProgram` |

### Names a target declares for the compiler to find

The integer operators and `=` above are found by spelling in the same way. These are the arbitrary-
precision names the host rung needs: the target declares them, and the compiler selects and emits them.

| word | status | where |
|---|---|---|
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
| `lemma` | reserved — F-E, a proved law | [theories.md](theories.md) |
| `forall` | reserved — a quantified fact (F-D); refused inside a `fact` | [theories.md](theories.md) |
| `prim` | reserved — a program may not declare a primitive; in a target it is `refused`, respelled `sig` (§4) | [state.md](state.md), [target-files.md](target-files.md) |

`record` (§1) is the fourth.

---

## 4. Refused spellings

Each is refused with a message naming the spelling that replaced it. A refused word stays in the
compiler so the message can be given.

| word | status | where |
|---|---|---|
| `sum` | refused — `variant` | [data.md](data.md) |
| `values` | refused — `tuple` | [data.md](data.md) |
| `int-repr` | refused — `(repr (int LO HI) (host …))` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `big-repr` | refused — `(repr big host)` or `(repr big limbs)` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `shift-width` | refused — `(repr shift N)` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `max-len` | refused as a form — `(fact max-len ((a (array A))) (<= (len a) N))` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `array-type` | refused — `(type (array A) (host …))` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `map-type` | refused — `(type (map K V) (host …))` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `boxed` | refused — `(repr (ref T) (host …))` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `builtin-map` | refused — `(repr map library)` | [loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md) |
| `length`, `length-of` | refused — `(ensures (= (len result) n))` and `(ensures (= (len result) (len c)))` | [lengthensures-2026-09-15](../../gauntlet/results/lengthensures-2026-09-15.md) |

---

## 5. Type names a target declares

Each target declares its own, and nothing here is portable, which is the point
([target-native.md](target-native.md)). They are **not** words of the language, so the test does not
read them:
- Go spells its integer types as manifest ranges (`int32` is `(int -2147483648 2147483647)`) and owns
  host types by module (`go/io.Writer`).
- The JVM and windows declare host spellings.
- JavaScript declares everything `any`.

`ptr`, which the last audit found undocumented, is windows' address type; it is a target type name
like the others and is declared in `targets/windows/`.

---

## 6. What is left

1. **The program's build tree is chosen by the target's NAME.** `WriteProgram` switches on
   `go`/`js`/`java`/`windows`, so a user target called anything else, with a perfectly good
   `(backend go)`, emits its code and is then refused by `cmd/build` with `target "mygo" has no
   program layout`. It is loud rather than wrong, and it is still the shape backend-2026-09-06 fixed
   for emission, left behind in the layout step: the layout belongs to the backend. Found by this
   audit, not fixed here.
2. **`the` has no specification.** It is a construct of the language, injected into every target, and
   only [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md) says what it means. Exposing
   `(the TYPE e)` to programs was recorded there as owing a spec, and the spec was never written.
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
