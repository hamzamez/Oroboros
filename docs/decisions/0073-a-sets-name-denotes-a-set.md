# 0073 — A set's name denotes a set: a header's sets are scope-checked

Date: 2026-10-23
Status: Accepted (hamza, 2026-10-23: "go ahead with item 7, always mathematical and algebraic", the 10-21
plan's item 7, found in setdef-2026-10-15, "Not fixed"). Extends
[ADR 0063](0063-a-type-is-a-static-value-and-instantiation-is-application.md) (a type is a static value)
to a program's headers, beside [ADR 0072](0072-a-declared-result-is-a-claim-decided-at-its-definition.md).

## Context

A program's header could name a set that nothing declares, and the name was taken for an opaque type:

| header | what happened |
|---|---|
| `export fn f(x ∈ Array(Nonesuch)) (Int)` | compiled to Go's `func GenF(v0 []/*Nonesuch?*/) int`, which does not build |
| `export fn g(x ∈ Nonesuch) (Int) { x + 1 }` | refused at the `+`, "x is Nonesuch, but Int is required here", as though it were a set |
| `fn k(x ∈ Nonesuch) (Int) { 0 }`, never called | compiled |
| `export fn keep(r ∈ strings.Replacer) (strings.Replacer)`, after `use go/strings` | Go's `/*strings.Replacer?*/`: a real host set, written through its import, never resolved |

The last row is the same defect from the other side: a name that does denote a set, unresolved.

## Decision

1. **⊢ S set is a scope check.** A header's set is a term of the set algebra, and ⟦·⟧ is defined on it
   by structural recursion, a homomorphism from the free algebra of set expressions into sets. It is
   total exactly when every atom is bound in Γ_T = the language's sets ∪ dom(T's types). The language's
   sets are `?`, `?int`, the ranges, and the formers `Array`, `buffer`, `cell`, `Map` and products, each
   well formed when its arguments are. T's types are what its files declare, the base sets `Int`,
   `Bool`, `F64`, `String` and `Any` among them. A name outside Γ_T is refused, the twin of a term's "is
   not bound", naming the header, the position and the name (`emit/setname.go`).
2. **An alias resolves as a term's does.** A set atom `alias.X` is X of the module the alias imports,
   so `strings.Replacer` is `go/strings.Replacer`, rewritten at load (`core.Load`, ρ_m). It runs after
   the step that turns a variant crossing a boundary into its tag and slots.
3. **Decided on every header, where its claims are** (ADR 0072): in `emit.CheckSignatures`, which
   `gen` and `build` run once per program. A definition whose body does not normalize, which is how an
   unbound term name shows itself (it is reported lazily, where reduction reaches it), is refused for
   its body first, as before. So a Java program loaded on Go still says which Java name is unbound.
4. **No variant is in Γ_T**, because none reaches a header by then: a variant result is its tag and
   slots (ADR 0042) and a variant parameter is refused at load (sums.md §6). The branches that admitted
   one were dead, as a planted fault showed, and are removed. A variant that reached a header would be
   refused loudly.

## Why not

- **Lazily, where a definition is reached**, as a term's unbound name is. An unbound term name is lazy
  so that a library shared across targets may name another target's functions in code this target
  never reaches. A header is a claim, decided whether or not the definition is called (ADR 0072). A
  header naming another target's set on this target says nothing here, and its body names that
  target's functions anyway, which keeps the old refusal first.
- **At load, in core.** The target's sets are known only once the target is: the same reason ADR 0026
  puts the word's line in the target's Env.
- **Keeping the opaque type.** It is the shape refinements.md §3a removed elsewhere, a claim accepted
  because nothing could check it. It printed Go that does not build and refused programs with a message
  that treated the name as a set.
- **Γ_T with the program's variants**, as the plan's description had it. No variant reaches the check,
  so the branch checked nothing (a path nothing runs). It comes back with step 2's Boolean algebra of
  sorts, when a program names its own sets.

## Consequences

- The four headers above: the first three are refused with the name and its position, and the fourth
  compiles to `func GenKeep(v0 *strings.Replacer) *strings.Replacer`.
- In the corpus, emission is unchanged ([setname-2026-10-23](../../gauntlet/results/setname-2026-10-23.md)).
  Every header names a set, and the corpus writes its one aliased host set with the full path.
- `emit/target.go`'s `/*name?*/` placeholder can no longer be reached by an unbound name in a program's
  header. It stays for what else might reach it.
