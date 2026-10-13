# typevalues-2026-10-13 — types as static values, step 1: `errors.AsType`

hamza, 2026-10-13: *"go ahead with 1, types as static values"*. Specified in
[spec/typevalues.md](../../docs/spec/typevalues.md), decided by
[ADR 0063](../../docs/decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md).
settypes-research §8's step 3, reordered first by the Presburger measurement, built as far as the
program that demands it, `gauntlet/stdlib/acceptance/errors-astype.oro`.

## 1. What was built

| | the algebra | where |
|---|---|---|
| a set is a static value | a type's name resolves like any name; β substitutes it; Law S: no set in a residual | `core/reduce.go` (`Env.Sets`, `staged`) |
| one namespace | a module's types and values are one set of names; a struct literal is its companion's `new` | `emit/instance.go` (`oneNamespace`), `targets/go/strconv.oro`, `gauntlet/stdlib/survey.go` |
| static parameters | `(sig (f (E (subsets T))) …)`: f ∈ Π(E ∈ 𝒫(⟦T⟧)). …, an outer λ in the declaration's tree | `emit/target.go` (`staticParams`), `emit/hostdef.go`, `core/decl.go` (`HostLam`) |
| instantiation | (f T̄) ⟶δ f#[T̄], σ = [T̄/Ē] on types and `%tᵢ`; the bound ⟦Tᵢ⟧ ⊆ ⟦B⟧ by subsumption | `core/reduce.go` (`instantiate`), `emit/instance.go` (`Instantiate`) |
| applicative identity | an instance is a function of its name, re-derived by any reader of the IR's text | `emit/instance.go` (`Instances`), `ir.Program.Calls`, `cmd/check/ir.go` |
| polymorphism as a static λ | a retraction's definition binds the static parameters first; β instantiates through it | `emit/retract.go` |
| comma-ok | H((option E)) = [E, bool], r(e, b) = some e if b, else none | `emit/retract.go` |
| `errors` | `New`, `Is`, `Unwrap`, `AsType`; NumError ⊆ error, its field reads, `ErrSyntax`, `ErrRange` | `targets/go/errors.oro`, `targets/go/strconv.oro` |

Go prints the instance as Go writes it:

```go
v10, v11 := errors.AsType[*strconv.NumError](v4)
if v11 {
	v13 := (v10.Func)
```

## 2. Measured first

`go run gauntlet/types/namespace.go`: of 65 type names in the target files and programs, one is also a
value in its module, `go/strconv.NumError`, its hand-written struct literal. 20 types have a companion
module, a namespace and no collision. The Go generator named every struct constructor after its type,
so every generated file would have collided once per constructor; it now writes the literal as the
companion's `new`, and `acceptance/struct-literal.oro` follows.

## 3. Witnesses, each with a planted fault

`emit/typevalues_test.go`. Each test was run against its rule disabled in the code, and failed:

| rule | witness | planted |
|---|---|---|
| the bound is membership | AsType at NumError made, at string refused | in the target: the `implements` edge removed refuses NumError; a false `string ⊆ error` admits string. In the code: the check skipped, and the string case passes wrongly |
| Law S | a set, an unapplied declaration over sets, the root's `error`, each refused; a program holding none passes | the staging check skipped |
| comma-ok | the instance's flag is bound and no `== nil` reads the option | the flag dropped: the declaration is refused at load, since E has no niche |
| one namespace | a type and a value of one name in a module refused | the check skipped |
| applicative identity | a fresh target re-derives the reduced primitive exactly; the IR's text, read back on a fresh target, prints what memory printed; the text does not verify before the instances are re-derived (anti-vacuity) | — |
| a static λ | `is-a` takes a set and β passes it to AsType | — |

The acceptance program's output is computed by the real `errors` and `strconv` in `tooling_test.go`
(`errorsAsTypeReference`): fields, causes by `Is`, `Unwrap` one step, `none` on `errors.New`'s error.

## 4. What it cost

- **`cmd/check`**: vet, compiler, emission, ir and differential pass. Emission is **byte-identical on
  all 556 runs** (300 emitted, 256 refused; 2,039 of 2,084 operations, 351 of 369 loops, unchanged): no
  existing program names a set, and the one that named NumError's literal is an acceptance program,
  outside the sweep. The IR step's round trip is exact with instances re-derived from their names.
- **Compile time**: the sweep's median is 1.21× its baseline, inside the rule (1.5× and 250 ms). Not
  measured by a pair; the added work per compile is a map of the target's types, one walk of the
  residual, and one more target file.
- **Tooling**: the acceptance programs, `errors-astype` among them, and every hand declaration
  (`go/errors` new, `go/strconv` with its exemptions) agree with the host. The step fails on one test
  unrelated to this change: `TestPublishedCountsAreTheCounts/js` refuses Node v26.8.1, since the JS
  survey's counts are pinned to v26.7.0 and this machine's Node was upgraded. Re-measuring the JS
  survey is its own task.
- **Coverage** (`gauntlet/stdlib/coverage.go`): 304 of 590 names in the ten packages it counts,
  51.5% (+2, strconv's `ErrSyntax` and `ErrRange`). `errors` is not among its ten.
- **Size**: +519 production lines in `core`, `emit`, `ir` and `cmd` (instance.go 188 of them), +228 of
  tests; +121 lines of `.oro` (errors.oro, the acceptance program, strconv's theory of NumError).
- **Found on the way**: the hand-agreement check applied exemptions only to partial files, so a
  complete file could not state a name the generator cannot produce (a field read, a package variable);
  an exemption now applies to either.

## 5. Not built

- Step 2, the Boolean algebra of sorts: ∪ ∩ ∖, singletons, a program's variants and the formers as
  static functions, Theorem P, unions across sorts.
- Inference of a static argument from a value argument (`slices.Max`), and a program's own polymorphic
  `sig` checked parametrically.
- Type sets (`cmp.Ordered`), host generic types, `errors.As` with its cell.
- `TypeImports` is read by the Go printer only; no other target declares a generic function.
