# Inventory: every word an `.oro` file can contain

The standard is *no word used in an `.oro` file should go unexplained or unspecified*. This is the
audit against it, taken mechanically from `core/read.go`, `emit/target.go` and `targets/*/*.oro`
rather than from memory.

> **Retaken 2026-08-25.** The previous audit was of the **retired portable layer** and all four of
> its findings are closed — see §5, which keeps them because what closed each one is the useful
> part. The language has roughly doubled since: tables, sums, `match`, several results, and the
> write side of the memory model.
>
> **Result: of 62 words, 57 are specified.** Four are described only in code comments and one is
> genuinely undocumented (§6). **None is wrong**, which is the first time that has been true — the
> previous audit found four outright errors.

---

## 1. Program forms — the language proper

Everything a *program* may write. Taken from `core/read.go`'s form kinds and its special-form
switch.

### Top-level forms — six

| word | status | where |
|---|---|---|
| `def` | ✅ | [def.md](def.md) |
| `sig` | ✅ | [types.md](types.md), [refinements.md](refinements.md) |
| `sum` | ✅ | [sums.md](sums.md) |
| `module`, `use`, `as`, `export` | ✅ | [modules.md §3](modules.md) |

`prim` and `target` also parse, and a program writing either is an **error** — primitives come from
target files and nowhere else ([state.md §2](state.md)).

### Term forms — one primitive, the rest sugar

| word | status | where |
|---|---|---|
| `fn`, `λ` | ✅ the only non-sugar special form | [core-0.md](core-0.md) |
| `let` | ✅ sugar → `(k e)` | [def.md](def.md) |
| `seq` | ✅ sugar → `((fn (_) b) a)` | [effects.md §5](effects.md) |
| `and`, `or`, `not`, `cond` | ✅ sugar → `if` | [booleans.md](booleans.md) |
| `values` | ✅ sugar → `(fn (#k) (#k a b))` | [values.md](values.md) |
| `match`, `when` | ✅ sugar → `loop` | [match.md](match.md) |
| `case` | ✅ sugar, expanded in `Load` | [sums.md](sums.md) |
| `loop`, `again`, `else` | ✅ | [ADR 0015](../decisions/0015-loop-and-again.md), [iteration.md](iteration.md) |
| `where` | ✅ | [refinements.md](refinements.md) |
| `true`, `false` | ✅ literals of the language | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md) |

**None of the sugar survives the reader**, except `case`, which needs a sum declared in another
file and so expands in `Load` ([state.md §1](state.md)).

### Names the compiler injects into every target — ten

A target may not declare one, and declaring one is an **error**.

| word | status | where |
|---|---|---|
| `if` | ✅ | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md) |
| `let`, `loop` | ✅ | [ADR 0015](../decisions/0015-loop-and-again.md), [state.md §1](state.md) |
| `=` | ✅ integer equality only, and the refusal explains itself | [match.md §6](match.md) |
| `array`, `table`, `len` | ✅ | [tables.md](tables.md) |
| `alloc`, `build`, `set` | ✅ | [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md), [tables.md §9](tables.md) |

**Indexing has no word at all** — `(a i)` is an application, because a table *is* a function with a
known finite domain ([tables.md §3](tables.md)).

**Thirty-three of thirty-three specified.** This half is in good order, which is what happens when
the specification is written first.

---

## 2. Target-file forms

Taken from `emit/target.go`'s parser. The whole grammar is
[target-files.md](target-files.md), which is the file a third party writes and therefore the one
that most needs to be a specification.

| word | status | where |
|---|---|---|
| `target`, `module` | ✅ | [target-files.md](target-files.md), [modules.md §4](modules.md) |
| `prim` | ✅ | [target-files.md §2](target-files.md) |
| `structural` | ✅ | [target-files.md §4](target-files.md) |
| `type` | ✅ | [target-files.md](target-files.md) |
| `data` | ✅ | [windows-target.md](windows-target.md) |
| `artifact`, `build` | ✅ | [build.md](build.md) |
| `narrow` | ✅ | [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) |
| **`array-type`** | ⚠️ **code comment** | `emit/target.go` — how a target spells an array of something, one declaration replacing an entry per element type ([tables.md §10](tables.md) states the *intent*, not the form) |

### Primitive attributes

| word | status | where |
|---|---|---|
| `pure` | ✅ | [effects.md §3](effects.md) |
| `import` | ✅ | [target-files.md §2](target-files.md) |
| `where` | ✅ | [refinements.md](refinements.md) |
| `checked` | ✅ | [selection-2026-08-19](../../gauntlet/results/selection-2026-08-19.md) |
| `length`, `length-of` | ✅ | [native-gauntlet-2026-08-20](../../gauntlet/results/native-gauntlet-2026-08-20.md) |
| `index` | ✅ | [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) |
| `jump` | ✅ | [ADR 0016](../decisions/0016-targets-need-not-have-expressions.md), [windows-target.md](windows-target.md) |
| `none` | ✅ a nullary primitive's argument list | [target-files.md §2](target-files.md) |

### Primitive kinds

| word | status | where |
|---|---|---|
| `expr`, `stmt` | ✅ — a statement's value is argument 0 | [target-files.md §3](target-files.md) |
| `cond`, `let`, `iterate` | ✅ structural; named in data, implemented in the backend | [target-files.md §4](target-files.md) |
| `array`, `table`, `len` | ✅ | [tables.md](tables.md) |
| `table-alloc`, `table-build`, `table-set` | ⚠️ **code comment** | `emit/target.go`, `emit/asm.go` — the *constructs* are specified in [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md); the KIND names a target-file reader would meet are not |
| `loop`, `loop2`, `build` | ✅ retired portable layer, kept for the gauntlet | [iteration.md](iteration.md) |

---

## 3. Type names

Each target declares its own; nothing here is portable, and that is the point
([target-native.md](target-native.md)).

| | declared by | status |
|---|---|---|
| `int`, `f64`, `bool`, `string`, `any` | all four | ✅ [integers.md](integers.md), [arithmetic.md](arithmetic.md), [booleans.md](booleans.md), [strings.md](strings.md) |
| `(array V)` | Go, Java via `array-type`; JavaScript and windows have no types to spell | ✅ [tables.md §5](tables.md) |
| `int64`, `byte`, `rune`, `error`, `slice-*`, `map-*` | Go | ⚠️ target-native, no portability claim, and the `slice-*` family is what `(array V)` replaces |
| `long-array`, `double-array`, `bool-array`, `string-array`, `list-*`, `map-string-long`, `strbuf`, `char`, `jint` | Java | ⚠️ as above |
| `ptr` | windows | ❌ **undocumented** — the only word in a target file that names a machine concept, and [windows-target.md](windows-target.md) does not define it |

**The enumerated array types are the surface `(array V)` exists to delete.** Go still declares
seven `slice-*`, Java four `*-array`; both are reachable and both are what a program written before
tables used.

---

## 4. Primitives

There is no longer a portable primitive layer to audit. **Every primitive is target-native and
qualified** — `go.+`, `js.===`, `java.merge`, `x64.imul` — carrying no portability claim by
construction ([target-native.md](target-native.md)). Each is classified in
[primitives.md](primitives.md).

What replaced the old portable names:

| was | is now |
|---|---|
| `add sub mul lt gt` (f64 only) | each target's own, at every width it has |
| `alen aindex slen sat` | `len` and **application** — the language's, on every target |
| `fold-range`, `fold-range2` | `loop`/`again` — n variables, no product |
| `dict-empty`, `dict-inc` | each host's own map, fused or unfused as **measured** |
| `if` | the language's, injected, undeclarable |
| `split-words` | `go.Fields`, `java.split`, … — and the conformance suite that caught it |

The retired `portable-*.oro` files still exist and still declare the old names, because the
gauntlet's older results were taken against them and deleting them would delete the ability to
reproduce those numbers.

### Still true, and worth keeping visible

**A Tier 1 name without a conformance suite is decoration.** `split-words` passed every check for
two months while returning different answers on different targets, which is why
[gauntlet/conformance/](../../gauntlet/conformance/) exists.

It covers `split-words` and nothing else — so
[`gauntlet/differential/`](../../gauntlet/differential/) was written the next day for the
LANGUAGE's constructs: `table`, `array`, `len`, indexing, `alloc`, `build`, `set`, `match`, `case`,
`values` and `loop`, each built on four targets, **run**, and required to agree *and* to give the
right answer ([differential-2026-08-26](../../gauntlet/results/differential-2026-08-26.md)). Its
pass condition was reproducing the two silent wrong-answer bugs this audit was written next to.

---

## 5. The previous audit's four findings, and what closed each

Kept because what closed them is the useful part.

**§1.1 — `fold-range` declared a type that was false.** The accumulator was `f64` and word count
passed a dictionary. Closed twice over: structural primitives carry no types in the table
([target-files.md §4](target-files.md)), and `fold-range` itself is **gone** — `loop` has n
variables and no product, so the construct that needed a polymorphic accumulator does not exist
([native-gauntlet-2026-08-20](../../gauntlet/results/native-gauntlet-2026-08-20.md) §6).

**§1.2 — there was no integer arithmetic.** Closed by the native targets, and specified far past
what the finding asked for: [integers.md](integers.md) settles eleven questions by measuring all
four hosts, and [ADR 0012](../decisions/0012-portable-integer-range.md) fixes the portable window.

**§1.3 — there was no boolean logic.** Closed by
[ADR 0017](../decisions/0017-booleans-are-in-the-language.md): `bool` is data, `if` is its
eliminator, the connectives are sugar, and **declaring a boolean name is an error**.

**§1.4 — the type names had no owner.** Closed by making targets directories
([target-native.md](target-native.md)): every type name is declared by exactly one target and
qualified by it. `ptr` in §3 is the one word this did not reach.

---

## 6. What is left

Three words undocumented and two in code comments only:

1. **`array-type`** — a target-file form, in a comment. [target-files.md](target-files.md) is the
   document it belongs in.
2. **`table-alloc` / `table-build` / `table-set`** — primitive kinds, in comments. A target author
   never writes them today, which is why they were missed; that stops being true the moment a
   target wants to provide one natively.
3. **`ptr`** — a windows type name, nowhere.

And the one that is not a word at all: **the conformance suite covers one primitive and none of
the language's constructs** (§4).
