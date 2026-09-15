# A variant type is its qualified declaration, and a pattern resolves in its module

2026-09-15. [spec/data.md §5.5.2](../../docs/spec/data.md),
[spec/theories.md §3.4](../../docs/spec/theories.md). It replaces the interim refusal of two modules declaring different variant types named
`result`, added earlier the same day (commit `bdb92a9`).

## 1. The law

Resolution is a renaming ρ_m from what module `m` can write to the declaration it means. The one law
it has to obey is **injectivity on declarations that differ**: if two declarations are different,
ρ must send them to different names, or which one a use means depends on something other than the
program text.

`Load` kept every module's variant types in one table keyed by the **bare** name. That key is
ρ composed with forgetting the module, `π ∘ ρ`, and `π` is not injective. So `a.result` and
`b.result` collided and the later declaration overwrote the earlier. A signature returning `result`
took its payload from whichever module loaded last, `(int, string)` in one order and `(int, int)` in
the other.

**The fix is the key**: `sumKey(path, s) = path.name`. The language's `option` stays one key,
`option`, because every module holds an identical copy only so that `some` and `none` resolve where
the compiler produces a map read, and a program may not declare its own.

**The property tested is load-order independence**: `Load(a ++ b)` and `Load(b ++ a)` give every
signature the same results.

## 2. What else the table was doing

Before building, the table's other consumer was measured rather than assumed. It was `case`, and it
made patterns **the one kind of name that ignored scope**. A constructor call `(a.ok n)` went through
`Module.resolve` and a `case` pattern did not:

| written in module `b` | before | now |
|---|---|---|
| `ok`, declared by the root module, not imported | **captured**: the tag `ok#tag` resolved by δ to the root's definition, since root names are unqualified | refused, naming the declaration and its module |
| `ok`, declared by `a`, imported as `a` | **accepted with the tag free**: residual `(if (= 0 ok#tag) n 0)`, no refusal from `Load` | refused: write the pattern through its alias |
| `a.ok` | **refused**: *"not a variant of any sum in scope"*, spelled in the old words | resolved; a static case reduces to `n` |

A pattern now resolves by `Module.resolve`'s rule: `c` in `m`'s own declarations, `alias.c` in the
module the alias imports. Both constructors and signature result types go through one function,
`Module.scope`, the half of ρ_m every namespace shares. `c#tag` is exported exactly when `c` is,
because `#` is not an identifier character and the tag exists only as what `case` expands a pattern
into. A `case` mixing two types with one spelling is refused naming both qualified names.

## 3. Witnesses, each failing against the bug put back

| test | planted bug | result |
|---|---|---|
| `TestTwoModulesMayDeclareDifferentSumsWithOneName` | `sumType` resolves against every module, last wins | fails in both orders, each naming the wrong payload |
| `TestAPatternDoesNotReachAnUnimportedModule` | `constructor` falls back to any module's declaration | fails |
| `TestAnImportedConstructorIsWrittenThroughItsAlias` | same | fails |

Plus a qualified pattern reducing to nothing, the export rule, the mixed-type refusal, and a control
that `option` loads in two modules.

## 4. Cost

- **Code**: `core/sum.go` 183 → 198, `core/reduce.go` 1,214 → 1,257, **+58**. It did not get smaller.
  The deleted half (the global table, the interim refusal, `sameSum`, the global constructor check)
  was shorter than resolution that checks scope and export, and says more when it refuses.
- Two refusal messages in unit tests changed wording (*variant type* and *constructor* where they
  said *sum* and *variant*). No corpus program declares a variant in one module and uses it in
  another, so no emitted file can change.
- **`go run ./cmd/check`, every step, passes**: emission 194 of 194 files byte-identical (456 runs,
  262 refused, unchanged), proof counts identical (2,307 of 2,413 integer operations, 345 of 382
  loops), differential on four targets, tooling.

## 5. Not built

- `option` as one declaration in `lang` rather than an identical copy per module (data.md §5.5.1).
- A module's own declaration shadowing `lang`'s, with the two-spelling message (§5.5.2, D4).
- Type arguments on variant types (§5.5.3–§5.5.5), and `type`, companions and `const` in modules.
