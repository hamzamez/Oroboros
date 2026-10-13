# 0063 — A type is a static value: one namespace, static parameters, instantiation by application

Date: 2026-10-13
Status: Accepted (hamza, 2026-10-13: "go ahead with 1, types as static values", the first of two build
items proposed after surface-design installment 5). Builds the first step of
[ADR 0049](0049-types-are-sets-and-the-logic-never-prints.md)'s "types are static values", as
[abstract.md](../spec/abstract.md) §2.6 states it. Specified in [spec/typevalues.md](../spec/typevalues.md); built, [typevalues-2026-10-13](../../gauntlet/results/typevalues-2026-10-13.md).

## Context

settypes-research §8 put "types as static values and the Boolean algebra of sorts" first after the
Presburger measurement, demanded by `errors.AsType`, Go 1.26's `AsType[E error](err error) (E, bool)`.
Its type parameter occurs only in the result, so a call must name it: the declaration is a function
from a set, and a call applies it to one. typevars-research worked the example through; abstract.md
decided the reading (a static parameter is a binder over a set of sets, instantiation is app).

Making a type a value puts types and values in one namespace. `gauntlet/types/namespace.go` measured
the collisions: of 65 declared type names, one is also a value in its module, `go/strconv.NumError`,
the type and its struct literal; the Go generator names every struct constructor after its type.

## Decision

1. **A set is a static value.** A type's name resolves like any name, β substitutes it, and staging
   erases it. A residual that holds a set, or a declaration with static parameters left unapplied, is
   refused.
2. **One namespace per module**: a name declared as a type and as a value in one module is refused.
   **A struct literal is its type's introduction rule and lives in the companion module as `new`**,
   beside the projections and methods: `(ne.new e "ParseInt" "x")`. The generator does the same.
3. **A declaration's head may bind static parameters**, `E` or `(E (subsets T))`, prenex and rank 1:
   `(sig (AsType (E (subsets error))) ((err error)) (option E) …)`. `subsets` is abstract.md's Σ₀
   constant, 𝒫(⟦T⟧).
4. **Instantiation is static application, reduced by δ**: `(f T̄)` ⟶ `f#[T̄]`, the declaration with
   [T̄/Ē] applied to its types and its template (`%tᵢ` the host spelling of Tᵢ), its imports gaining
   the Tᵢ's packages. **The bound is membership**, ⟦Tᵢ⟧ ⊆ ⟦B⟧ by the target's subsumption, refused
   otherwise. **The instance is identified applicatively by its name**, so every reader re-derives it,
   the printers from IR text included.
5. **A retraction's definition binds the static parameters first**: polymorphism is a static λ, and β
   instantiates through it with no new rule.
6. **`(comma-ok)`**: a host clause by which a declared `(option E)` is the host's (v, ok),
   H = [E, bool], r(e, b) = `some e` if b, else `none`.
7. **No inference yet.** A static argument is written; inferring one from a value argument needs the
   checker's unifier and waits for a program that calls such a declaration.

## Why not

- **Two namespaces, a type argument read by position** (typevars-research §6: "the loader knows AsType
  takes one type parameter"). A name would denote a type in one position and a value in another, a
  homonym (fit-research's misfits), and ADR 0049 makes types values. The measured cost of one namespace
  is one hand-written name.
- **A set applied to arguments as its constructor** (`(sc.NumError e …)` kept, the set's application
  defined as the literal). It defines application of a set to mean one particular function of it,
  which is the homonym again, and fails for a type with several introduction forms.
- **`(E error)` for a bounded parameter** (typevars-research decision 1). As a binder it reads E ∈ error,
  E a value; the parameter ranges over subsets of error, a set of sets (W9's level 2), so its set is
  written `(subsets error)`.
- **The niche for AsType's absence.** Go's `err.(E)` holds of a typed nil, so AsType may return
  (nil, true); only the flag separates a found nil from none.
- **Instances as new primitives recorded in the IR's text with their types**: the text would carry a
  second copy of the declaration; a name that determines its instance needs nothing more.
- **Inference now**: no program calls a declaration whose static argument an argument fixes, and the
  unifier is on the residual, after reduction has chosen the instance.

## Consequences

- `targets/go/errors.oro` (`New`, `Is`, `Unwrap`, `AsType`), NumError's `implements error`, field
  reads and `ErrSyntax`/`ErrRange`, and `acceptance/errors-astype.oro`.
- The hand `NumError` constructor and every generated struct constructor move to the companion's `new`;
  `acceptance/strconv.oro` and `struct-literal.oro` follow.
- Next in this line, step 2: the Boolean algebra of sorts, demanded by the first program that states a
  set it cannot write today.
