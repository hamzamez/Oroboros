# Types as static values, step 1: sets as arguments, and declarations over them

Status: **built** ([typevalues-2026-10-13](../../gauntlet/results/typevalues-2026-10-13.md)). Decided by [ADR 0063](../decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md).
Written 2026-10-13 on hamza's *"go ahead with 1, types as static values"*. It is
[settypes-research.md](../settypes-research.md) §8's step 3, reordered first by the Presburger
measurement, and the part of it that `errors.AsType` demands. The semantics is
[ADR 0049](../decisions/0049-types-are-sets-and-the-logic-never-prints.md)'s and
[abstract.md](abstract.md)'s (§2.6: "a static parameter is a ranging binder over a set of sets,
`E ∈ subsets(error)`, and instantiation is app"); the spelling is today's notation's, since the surface
is not chosen (surface-design.md), and a later reader changes only the spelling.

## 0. In brief

1. **A type is a set of values, and a set is a value of the static level** (W9's level 1). A type's
   name is a name like any other: it resolves like one, β substitutes it, and it is erased by staging.
   **No set survives staging**: a residual that still holds one is refused, as a residual that still
   holds a closure is (closures-direction.md).
2. **One namespace.** A module's types and values share its names, so a name denotes one thing.
   Measured, one hand-written name in the corpus is both: `go/strconv.NumError`, the type and its
   struct literal. The generator names every struct constructor after its type. **A struct literal is
   its type's introduction rule, so it moves into the type's theory**, the companion module, as `new`.
3. **A declaration may bind static parameters in its head**, each a name and the set of sets it ranges
   over: `(sig (AsType (E (subsets error))) ((err error)) (option E) …)`. That is
   AsType ∈ Π(E ∈ 𝒫(⟦error⟧)). (⟦error⟧ → 1 + E), prenex and rank 1.
4. **Instantiation is application, reduced at compile time**: `(errors.AsType strconv.NumError)`
   reduces by δ to the instance `AsType#[go/strconv.NumError]`, the declaration with [NumError/E]
   applied to its types and its template. An instance is identified **applicatively**, by the
   declaration and its arguments, so it is a function of its name and is re-derived wherever the name
   is read. The bound is membership, ⟦NumError⟧ ∈ 𝒫(⟦error⟧), decided by the target's subsumption.
5. **A host signals an option's absence by a flag**, Go's comma-ok: `(host … (comma-ok))` makes
   H((option E)) = [E, bool] and r(e, b) = `some e` if b, `none` otherwise.
6. **Built for `errors.AsType`, `Is`, `New` and `Unwrap`** (`targets/go/errors.oro`), demanded by
   `gauntlet/stdlib/acceptance/errors-astype.oro`.

## 1. What a type is, and where it lives

ADR 0049: a type is a set of values. The levels of W9 (abstract.md §3) are:

```
L0   values                                  3, "GET", a table
L1   sets of values: types                   int, (int 0 255), error, go/strconv.NumError
L2   sets of sets of values                  subsets(error) = 𝒫(⟦error⟧)
```

The static level computes at compile time and is erased by staging (ADR 0009, closures-direction.md).
**A set is a static value**: it may be bound, passed and applied to, and it has no run-time
representation. This step makes the names the target declares as types (`tg.Types`: `int`, `error`,
`go/strconv.NumError`, …) static values. A program's own variants, manifest types (`int32`), and the
type formers (`array`, `option`, `tuple`) used as functions on sets come with step 2, the Boolean
algebra of sorts (§10).

**Until then a program cannot name its own set, and is told so where it tries.** A type in a program's
`sig` that names one of the program's definitions, in its own module or through an import, is refused
at that `sig`: `(def Byte (int 0 255)) (sig f ((x Byte)) …)`. The checker has no set for a definition to
denote before step 2. Accepted, `Byte` was an opaque type that every use refused, "x is Byte, but int is
required here", and a table of it printed an unknown host type, `[]/*Byte?*/` (du-2026-10-14). A set a
program states is written where it is used, `(int 0 255)`.

**Law S (staging).** A set has no run-time representation, so ⟦P⟧ does not depend on how a set
argument is written or bound, and erasing every set from a residual changes nothing it computes.
Consequently **a residual that holds a set is refused**: it would have no meaning to print.

## 2. One namespace

A module is a theory (ADR 0021) whose names denote its constants. With types as values, a type and a
value are both constants, so **a name declared twice in one module, once as a type and once as a value,
is refused**, as a name defined twice is. A companion module (`go/strconv/NumError`) is a namespace,
not a value (ADR 0011: modules are resolution), and its path is written with `/`, a qualified name with
`.`, so it collides with nothing.

**Measured** (`go run gauntlet/types/namespace.go`): of 65 type names declared in the target files and
the programs, **one** is also a value in its module, `go/strconv.NumError`: the type and a hand-written
constructor, the struct literal `&strconv.NumError{…}`. 20 types have a companion module, which is no
collision. The Go generator (`gauntlet/stdlib/survey.go`, `structCtors`) names every struct constructor
after its type, so every generated file has one collision per constructor.

**The struct literal is the type's introduction rule.** A struct is Π_{f ∈ L} T_f, a product over its
labels; the literal is the tupling ⟨…⟩ : Π T_f → T and the field reads are its projections. Its methods
and projections already live in the companion module, the type's theory, so the introduction joins
them there under the name `new`:

```lisp
(use go/strconv/NumError as ne)
(ne.new e "ParseInt" "x")            ; was (sc.NumError e "ParseInt" "x")
(ne.Func n)                           ; a projection, beside it
```

`new` is Rust's convention (`Type::new`) and Go's own word for a pointer to a fresh value; it names an
operation, not a type, so it cannot collide with one.

## 3. A set as an argument

A type's name in a term resolves like any name: `strconv.NumError` is `go/strconv.NumError` by the
import alias, and a bare `error` is the root's. It is pure and duplicable, so β substitutes it, and the
scope check accepts it.

**Refused after reduction:**
- **a set in the residual** (Law S): "go/strconv.NumError is a set, a static value, and cannot be a value
  at run time; a set is an argument of a declaration's static parameter";
- **a declaration with static parameters not applied to them**: its instance is not determined, and a
  variable is never defaulted (typevars-research §7: defaulting to `any` would relax every check that
  reads the result).

## 4. Static parameters

```
(sig (f B₁ … Bₙ) params result clause… (host …))      Bᵢ ::= E | (E S)
```

- `E` ranges over every set of values (no claim on it, settypes §10);
- `(E S)` ranges over S, a set of sets. This step has one: **`(subsets T)`**, 𝒫(⟦T⟧), abstract.md's
  Σ₀ constant `subsets`, TLA+'s `SUBSET T`.

The binders are the head's, as a variant's head binds its parameters (`(variant (result T E) …)`), and
they are bound in the parameters, the result and the clauses. Read as the abstract syntax:

```
def(AsType; λ(E ∈ subsets(error); ; ⊤; ⊤; λ(err ∈ error; r ∈ option(E); ⊤; ⊤; host(…))))
AsType ∈ Π(E ∈ 𝒫(⟦error⟧)). (⟦error⟧ → 1 + ⟦E⟧)
```

The static λ is outside, curried. Every quantifier is in front, so this is bounded rank-1 polymorphism
(Hindley 1969, Milner 1978; bounds Cardelli and Wegner 1985), whose instances are decided by a
membership test; full F<: is undecidable (Pierce 1994), and Go's generics chose the same fragment
(Griesemer et al., "Featherweight Go", OOPSLA 2020). E may occur wherever a type may in the
declaration: a parameter, the result, inside a tuple, an option, a table's element, a cell.

A static parameter's name shadows a type of the same name inside its declaration: the declaration's
types resolve lexically (theories.md §3.4) **after** its binders, as a λ's body sees its parameters
first.

## 5. Instantiation is application

```
(f T₁ … Tₙ)  ⟶δ  f#[T₁,…,Tₙ]        each Tᵢ a set
```

**The instance** f#[T̄] is the declaration with the substitution σ = [T̄/Ē] applied:
- to its parameters', results' and clauses' types, each Tᵢ written by its qualified name;
- to its template: the hole `%tᵢ` is Tᵢ's host spelling (`errors.AsType[%t1](%s)` becomes
  `errors.AsType[*strconv.NumError](%s)`);
- and its imports gain the packages the Tᵢ's spellings name, each the host path of Tᵢ's module
  (ADR 0025: ι(p) = HOST ++ p, so `go/strconv.NumError`'s package is `strconv`).

**The bound is membership.** Tᵢ ∈ Sᵢ; for Sᵢ = subsets(B) that is ⟦Tᵢ⟧ ⊆ ⟦B⟧, the target's subsumption
(`Subsumes`, its declared `implements` closed reflexively and transitively). A violation is refused at
compile time, naming the bound: `(errors.AsType string)` is refused, where Go's `errors.As` panics at run
time.

**Applicative identity.** Two applications of f to the same sets are the same function, so the instance
is named by f and its arguments, `f#[T₁,…]`, as a variant's instance is (`result(int, string)`,
data.md §5.5.3). The `#` keeps a program from writing it (emit/retract.go's convention). Since the name
determines the instance, **any reader of the name re-derives it**: the IR's text names the instance and
the printers re-instantiate from the target's declarations (ADR 0038: the text and the declarations are
the printers' whole input).

**Laws.**
1. **Substitution commutes with instantiation**: β then δ-instantiate = δ-instantiate then β, since σ
   acts on types and β on values; a set bound by a λ is instantiated once β has given it.
2. **Nothing after reduction learns of polymorphism.** The residual names instances, each an ordinary
   primitive; the checker, the analyses, the IR and the printers are unchanged. This is
   monomorphization, which Featherweight Go proves correct for Go's generics.
3. **Staging preserves results** (ADR 0009, Law S): an instance computes what the host's generic call
   computes at that type.

**Not this step: inference.** `(slices.Max xs)`, with E fixed by xs's type, needs the checker's
unifier, since types are known only on the residual. It comes with the first program that calls such a
declaration (typevars-research §4: 51 of Go's 88 generic functions and methods need no callback).

## 6. Polymorphism as a static λ

A definition may take a set like any value, and β does the rest: no rule is added.

```lisp
(def as-num-error (e) ((errors.AsType strconv.NumError) e))      ; an instance used
(def first (E) (fn (e) ((errors.AsType E) e)))                   ; a static λ over a set
((first strconv.NumError) e)                                     ; ⟶β ((errors.AsType strconv.NumError) e)
```

**A retraction's definition is such a λ.** A fallible or optional declaration becomes its raw call and
a target definition applying r (errors.md §4.2). With static parameters the definition binds them first
and passes them to the raw call:

```lisp
(def AsType (E) (fn (#a1) (((#raw:go/errors:AsType E) #a1) (fn (#h1 #h2) (if #h2 (some #h1) none)))))
```

A program's own `sig` with static parameters, checked parametrically (typevars-research decision 4), is
not this step.

## 7. A flag for an option's absence: comma-ok

Go returns an optional value as a pair (v, ok), "the comma ok idiom" (Effective Go; the Go
specification's special forms of index, type assertion and receive), with v the zero value when ok is
false. As a set the pair is {(v, true) : v ∈ E} ∪ {(0, false)} ≅ E + 1. The niche of errors.md §4.1 reads
absence off the value itself; here it is a second host value, so the declaration says so, as §4.4's
sentinel says a per-declaration encoding:

```
(host expr "…" (comma-ok))        H((option E)) = [E, bool]        r(e, b) = some e if b, else none
```

`(comma-ok)` requires the declared result to be `(option E)`. It is not the niche even where E has one:
`err.(E)` holds of a typed nil inside a non-nil interface, so `AsType` may return (nil, true), and only
the flag tells a found nil from none. Go's zero value never reaches the program, "no nil"
(mutscope-research decision 6), held by the type.

## 8. `errors`, and the program

`targets/go/errors.oro`, read off `src/errors/{errors,wrap,join}.go`:
- `New : string → error`, never nil;
- `Is : error × error → bool`, a search of the error's tree for an equal error or one whose `Is`
  says so;
- `Unwrap : error → 1 + error`, the niche: nil when the error wraps nothing or many;
- `AsType : Π(E ∈ 𝒫(error)). error → 1 + E`, the pre-order search of the tree for the first error in
  E, or one whose `As` claims membership (typevars-research §8).

`go/strconv.NumError` gains `(implements go/strconv.NumError error)`, its field reads `Func`, `Num` and
`Err` in its companion, and `strconv.ErrSyntax` and `ErrRange`.

`gauntlet/stdlib/acceptance/errors-astype.oro` parses bad numerals, takes the `*strconv.NumError` out of
each error with AsType, reads its fields, tests its cause with `Is` against `ErrSyntax` and `ErrRange`,
and shows AsType's `none` on an error that is not one, `errors.New`'s. Go prints:

```go
if ne, ok := errors.AsType[*strconv.NumError](err); ok { … }
```

## 9. On each target

Go prints the instance's type argument where its template has a hole. The other targets declare no
generic function today: JavaScript has no types, so a variable would erase; Java's generics would print
the instance or its erasure; Win32 has no generic API (typevars-research §9).

## 10. Not here

- **Step 2, the Boolean algebra of sorts**: ∪ ∩ ∖ on sets, singletons, a program's variants and the type
  formers as static functions, `(def nonzero (not 0))`, Theorem P and unions across sorts as ADR 0042's
  slots. The first program that states a set it cannot write today demands it.
- **Inference of static arguments** (§5), and a program's own polymorphic `sig` (§6).
- **Type sets** (`cmp.Ordered`, `comparable`), host generic types (`atomic.Pointer[T]`), static
  parameters inside a callback's type: each with the program that needs it.
- **`errors.As`**, the cell form: `AsType` is Go's newer and allocation-free form of the same function.
