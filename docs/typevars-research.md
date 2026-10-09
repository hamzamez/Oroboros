# Type variables in declarations: quantification over sets

Research, 2026-10-09, on hamza's *"write the research. the math, the algebra and the literature.
take into consideration tla+ in your research. and let's try to keep the syntax pleasant and uniform
so programmers don't get confused."* It follows outcells-2026-10-09, whose next step, `errors.As`,
needs a declaration whose result names an argument's type. Nothing here is built. The measurement is
`gauntlet/stdlib/generics.go`.

## 0. The answer, in brief

1. **A type is a set of values** (ADR 0003, ADR 0029, types.md), so **a type variable ranges over
   sets**, and a declaration with one is a family of functions indexed by the sets it ranges over.
   Its honest form is a quantifier that says what the variable ranges over:
   `As : ∀E ∈ ↓error. error → E + 1`.
2. **A bound is a set of types**, and a variable is a member of it, `E ∈ K`. An interface I as a bound
   is the set of types below it, ↓I = {T : T ≤ I}, so for interfaces membership is the subset
   relation on values, ⟦E⟧ ⊆ ⟦I⟧. A union such as Go's `cmp.Ordered` is a set of types and is not a
   subset of anything. One relation, ∈, covers both, and it is Go's own definition: since 1.18 Go
   says a constraint "defines a type set".
3. **This is TLA+'s reading exactly.** TLA+ is untyped set theory: a type is a set, every binder
   carries its set (`\A x \in S : P`, `{x \in S : P}`, `[x \in S |-> e]`), and an operator is
   parametric by taking sets as arguments. We take its uniformity, every binder is a name and the set
   it is drawn from, and decline its one choice that does not fit a compiler: types checked as
   theorems after the fact. We check them statically, on a decidable fragment.
4. **The general form is bounded rank-1 polymorphism**: ∀T₁ ∈ K₁ … Tₙ ∈ Kₙ. params → result, the
   quantifiers in front and instantiated at every call (Hindley–Milner's instantiation, with Cardelli
   and Wegner's bounds). Full System F<: is undecidable (Pierce 1994); the prenex fragment is not.
5. **Measured: Go's standard library has 88 generic functions and methods and 8 generic types**,
   and about 16 more names typed `any` whose type the caller chooses (the out-parameters still open
   after the Scan family). A variable occurs in an ordinary parameter in 57 of the 88, in the result
   in 31, inside a callback's type in 30. **51 need no callback**, so they are declarable once type
   variables exist.
6. **The syntax, uniform with what exists**: a declaration's head binds its type parameters, as a
   variant's head already does; a binder is a name and its set; a use instantiates by application:
   ```lisp
   (variant (result T E) (ok T) (err E))                     ; exists: the head binds T and E
   (sig (AsType (E error)) ((err error)) (option E) …)       ; E drawn from the types below error
   ((errors.AsType strconv.NumError) e)                     ; explicit: the declaration applied to a type
   (slices.Max xs)                                          ; inferred: every variable fixed by an argument
   ```
   No spelling convention: a variable is a variable because a head binds it, so Go's capitalized
   type names (`NumError`, `Writer`) stay types.

## 1. What a type variable is, in sets

The language's types denote sets of values: `(int 0 255)` the integers 0 … 255, `bool` the two
truth values, `go/strconv.NumError` the values Go's `*strconv.NumError` holds, `(array σ)` the finite
functions from an initial segment of ℕ into ⟦σ⟧. A declaration `(sig f ((x A)) B)` claims
f : ⟦A⟧ → ⟦B⟧.

A type variable T is a name for an unknown set, and a declaration over it is a **family**:

```
reverse : ∀T. ⟦array T⟧ → ⟦array T⟧        one function for each set T
```

Instantiating T at a set S picks one member of the family, `reverse_S`. This is System F's type
abstraction and application (Girard 1972, Reynolds 1974), restricted to the case where every
quantifier is in front (prenex, rank 1), which is ML's let-polymorphism (Hindley 1969, Milner 1978,
Damas and Milner 1982): a type scheme ∀T. τ, instantiated at each use by a fresh variable that
unification solves.

**Parametricity** says what such a family may do. A function defined uniformly in T cannot inspect
values of T, so it commutes with every relation between instances (Reynolds 1983, "Types,
abstraction and parametric polymorphism"; Wadler 1989, "Theorems for free!"): `reverse` can only
rearrange, never invent an element. A declaration with a bound is parametric up to the bound's
operations: a ∀T ∈ ↓error may call `Error()` and nothing else.

## 2. The bound: a set of types

An unbounded variable ranges over every type. A bounded one ranges over a set of types K, and is a
member of it, T ∈ K. Three kinds of K occur in Go's library (§4):

| bound | the set of types | example |
|---|---|---|
| none (`any` in Go's spelling) | every type | `slices.Reverse[S ~[]E, E any]` |
| an interface I | ↓I = {T : T ≤ I}, the types below I | `errors.AsType[E error]` |
| a union, a type set | the listed types, or those whose underlying type is listed (`~int`) | `cmp.Ordered`, `comparable` |

For an interface, T ∈ ↓I says ⟦T⟧ ⊆ ⟦I⟧: every value of T is a value of I. That is the subset
relation on values, and it is the target's subsumption, which `implements` declares and types.md
§3.2 reads. **So "T ≤ error" does mean T is a subset of error**, read through ↓. A union is a set of
types whose values do not form a type: Go's `cmp.Ordered` is the integers, the floats and the strings,
a set of sets with no common value set the program could hold. Membership covers both; subset covers
only the first. So the bound's relation is ∈, and an interface used as a bound denotes ↓I.

**Go defines it the same way.** Since Go 1.18 the spec reads an interface as a type set: "an
interface type defines a type set", a type satisfies a constraint when it is in that set, and a
constraint may be a union (`~int | ~float64`) that is no ordinary interface. Featherweight Generic
Go formalizes this and proves the translation to monomorphic Go correct (Griesemer, Hu, Kokke, Lange,
Taylor, Toninho, Wadler and Yoshida, "Featherweight Go", OOPSLA 2020). Type classes are the same idea
in Haskell, a bound as a predicate on types (Wadler and Blott 1989); bounded quantification
∀t ≤ τ. σ is Cardelli and Wegner's ("On understanding types, data abstraction, and polymorphism",
1985).

**Decidability chooses the fragment.** Bounded quantification in full, F<:, has an undecidable
subtyping relation (Pierce, "Bounded quantification is undecidable", 1994), because a quantifier may
appear anywhere and its bound may mention other quantified types. With every quantifier in front and
each bound a fixed set the target names, checking an instance is a membership test, and solving one
is first-order unification. That is the fragment Go chose, and the one this language needs.

## 3. TLA+

TLA+ (Lamport, *Specifying Systems*, 2002) builds on untyped Zermelo–Fraenkel set theory with
choice, and its treatment of types is the most direct statement of the view this language already
holds.

**A type is a set, and typing is a theorem.** TLA+ has no type system. A specification states its
"type" as an invariant, `TypeOK == x \in Nat /\ f \in [Proc -> Nat]`, which TLC checks by
exploring states and TLAPS proves; a value outside it is not ill-typed, the invariant is false.
Lamport and Paulson argue the trade-off ("Should your specification language be typed?", ACM TOPLAS
21(3), 1999): untyped set theory is more expressive and closer to ordinary mathematics, and types buy
mechanical checking at the cost of expressiveness. Their argument is about specification; for a
compiler, whose hosts need the types to run, the static half is not optional.

**Polymorphism is free, and it is a set argument.** Because nothing is typed, every operator is
polymorphic. When an operator needs to know a set, it takes the set as an argument:
`Max(S) == CHOOSE x \in S : \A y \in S : y <= x`, `Seq(S)`, `[S -> T]`, `SUBSET S`. A module is
parametric in its `CONSTANTS`, and `INSTANCE M WITH Proc <- {p1, p2}` instantiates it by
substitution. That is our instantiation of a declaration's type parameters, at a finer grain: a
module's constants are a theory's parameters (ADR 0021 reads a module as a theory and a target as a
model), and a call's type arguments are the same substitution applied to one declaration.

**Every binder carries its set.** TLA+ writes one shape for every binding: `\A x \in S : P`,
`\E x \in S : P`, `{x \in S : P}`, `{e : x \in S}`, `[x \in S |-> e]`, `CHOOSE x \in S : P`. A
reader never wonders where a variable ranges. Our language already has the shape for values,
`(x int)` in a signature meaning x ∈ int, and §6 extends it to types: `(E error)` in a head meaning E
is drawn from the types below error. The relation is the same membership; what changes is the set,
types rather than values.

**Apalache adds types to TLA+, with variables.** Apalache's type checker (Konnov, Kukovec and Tran,
"TLA+ model checking made symbolic", OOPSLA 2019; its type system, "Snowcat", 2020) annotates
operators, `\* @type: (Set(a), a) => Bool;`, with lowercase type variables solved by Hindley–Milner
inference. It writes variables lowercase because TLA+ convention spells constants and sets in
capitals: a spelling convention chosen so as not to collide with the host language's names. We have
the same collision, with Go's capitalized types, and avoid it the way §6 says: by a binder, not by
spelling.

**What we take, and what we decline.** We take the reading (a type is a set; a variable ranges over a
set of sets; a bound is a membership), the uniformity (every binder a name and its set), and
instantiation as substitution. We decline typing as an after-the-fact theorem, for the reason above,
and we keep the fragment decidable (§2), where TLA+ needs none, since it checks by exploration and
proof.

## 4. Measured: what Go's library needs

`go run gauntlet/stdlib/generics.go` reads every line of Go's API manifest with a type parameter
(Go 1.27):

**88 generic functions and methods, and 8 generic types.** By package: `slices` 40, `maps` 10,
`encoding/json/v2` 5, `hash/maphash` 4, `sync/atomic` 4, `cmp` 3, `crypto/hkdf` 3,
`encoding/json/jsontext` 3, and one or two each in `iter`, `database/sql`, `unique`, `weak`,
`reflect`, `sync`, `math/rand/v2`, `runtime`, `crypto/pbkdf2` and `errors`. The types:
`atomic.Pointer`, `iter.Seq`, `iter.Seq2`, `sql.Null`, `maphash.Hasher`, `maphash.ComparableHasher`,
`unique.Handle`, `weak.Pointer`.

**Where a variable occurs** (a name counts once for each position it uses):

| position | names | example |
|---|---:|---|
| an ordinary parameter | 57 | `cmp.Compare[T Ordered](x, y T) int` |
| the result | 31 | `slices.Max[S ~[]E, E Ordered](x S) E` |
| inside a callback's type | 30 | `slices.SortFunc(x S, cmp func(a, b E) int)` |
| a host type's argument | 9 | `atomic.Pointer[T].Load() *T` |
| the result, as an iterator | 7 | `maps.Keys(m) iter.Seq[K]` |
| a pointer the host writes | 5 | `atomic.Pointer[T].Store(val *T)` |
| a variadic list's element | 4 | `slices.Concat(slices ...S)` |

**51 of the 88 need no callback and no iterator**; 37 need one (callbacks are tiers 1 and 2,
callbacks.md, not built). In 2, the variable occurs only in the result, so no argument can fix it and
a call must name it: `errors.AsType[E error](error) (E, bool)` and `reflect.TypeAssert[T](Value) (T,
bool)`. In `reflect.TypeFor[T]() Type` it occurs in no value at all: a pure type argument.

**What bounds a function's type parameter**: none (`any`) 58, an approximation type set `~[]E` or
`~map[K]V` 53, `comparable` 19, `cmp.Ordered` 9, an interface (`hash.Hash` 4, `error` 1), and one
private union.

**The approximation is Go's, not a new idea for us.** 53 of those bounds are the pattern
`[S ~[]E, E any]`: S is any type whose underlying type is a slice of E, which exists so a named slice
type is accepted. The language has no named slice types, so S is always `(array E)` here and the
pattern collapses to one variable, E, in a table's element: `(sig (Max (E cmp.Ordered)) ((x (array
E))) E)`.

**Beside the generic API**, the caller-typed `any` parameters: outparams-research.md counted about 25
in 8 packages, and the Scan family closed 9 (outcells-2026-10-09). The rest, `errors.As`,
`json.Unmarshal`, `binary.Read`, `sql.Rows.Scan` and their kin, have a variable in a cell parameter,
which §5 counts as a sixth position.

## 5. Where a variable may appear: the general form

```
(sig (f p₁ … pₖ) params result clauses…)        each pᵢ a name, or (name bound)
```

means f : ∀p₁ ∈ K₁ … pₖ ∈ Kₖ. params → result, where a bare name's K is every type. A variable may
occur in any type position of the params and the result, and the positions differ only in what the
rest of the compiler must already have:

| position | what it needs | Go names | status |
|---|---|---|---|
| an ordinary parameter, a table's element | the checker's and the IR's unification (both have variables already) | `cmp.Compare`, `slices.Contains`, `slices.Index`, `slices.Max` | this research |
| the result | the same, plus explicit instantiation when no argument fixes it | `errors.AsType`, `slices.Max` | this research |
| a cell parameter | ADR 0048's cells | `errors.As`, `binary.Read` | this research |
| a variadic list's element | ADR 0046's lists | `slices.Concat` | this research |
| a host type's argument | host type constructors with parameters, `(type (Pointer T) (host "atomic.Pointer[%s]"))`, generalizing the array, map and cell spellings | `atomic.Pointer`, `unique.Handle`, `sql.Null` | when a program needs one |
| inside a callback's type | callbacks, tiers 1 and 2 | `slices.SortFunc`, `maps.DeleteFunc`, iterators | with callbacks |

**A buffer stays a buffer.** `slices.Sort(x S)` sorts in place: on our tables it is a write-borrow,
B ⊸ B, as `utf8.EncodeRune`'s (host-buffers.md): `(sig (Sort (E cmp.Ordered)) ((x (buffer E)))
(buffer E) …)`. `slices.Insert` may reallocate: consume and return. The variable changes nothing about
what a host does to storage; the declaration says both.

**A program's own `sig` takes the same form.** `(sig (swap A B) ((p (tuple A B))) (tuple B A))` is a
claim about a definition, for every A and B. §7 says how it is checked.

## 6. The syntax

**Three principles**, each already true of the language somewhere:

1. **A head binds.** `(variant (result T E) …)` declares T and E in its head. A declaration's head
   does the same: `(sig (AsType (E error)) …)`. A variable is a variable because a head binds it,
   never because of how it is spelled.
2. **A binder is a name and the set it is drawn from.** A parameter is `(err error)`, err ∈ error. A
   type parameter is `(E error)`, E drawn from the types below error. The shape is TLA+'s
   `\A x \in S`, and Go's own order: `AsType[E error]`.
3. **Application instantiates.** A variant type is applied to its arguments, `(option int)`,
   `(result.result int error)`. A polymorphic declaration is applied to its type arguments the same
   way: `(errors.AsType strconv.NumError)` is the instance, a function of an error. This is System F's
   type application, written as the language writes every application.

**The forms:**

```lisp
; declaring: the head binds; a bare name is unbounded, (name bound) is bounded
(sig (Reverse E) ((s (buffer E))) (buffer E) …)
(sig (Max (E cmp.Ordered)) ((x (array E))) E …)
(sig (AsType (E error)) ((err error)) (option E) …)
(sig (As (T error)) ((err error) (target (cell T))) (option T) …)

; using: inferred when every variable is fixed by an argument
(slices.Max xs)
(errors.As e (out strconv.NumError))       ; the cell's type fixes T

; using: explicit, the declaration applied to its type arguments first
((errors.AsType strconv.NumError) e)
```

**Reading a call.** `((errors.AsType strconv.NumError) e)` is read as Go reads
`errors.AsType[*strconv.NumError](e)`: the instance, then its argument. The loader knows `AsType`
takes one type parameter, so the first application's argument is a type, as a variant's head's
arguments are; there is no second spelling of names for types.

**Alternatives weighed.**

- **A capital letter makes a variable** (ML's `'a`, Haskell's lowercase, Apalache's `a`). Every such
  convention collides with some host: Go's exported types are capitalized (`NumError`, `Writer`,
  `Value`), and a target file names them constantly. A binder cannot collide.
- **A separate quantifier clause**, `(sig As ((err error) (target (cell T))) (option T) (forall (T
  error)))`. It states the same thing, but after the variable is used, and in a different place from
  the variant's head. A head that binds what the body then uses is the order a reader expects and
  the order Go writes.
- **Type arguments marked as types at the call**, `(errors.AsType (type strconv.NumError) e)`. A
  second syntax for one application, and a form a programmer must learn only here.
- **Go's brackets**, `errors.AsType[strconv.NumError]`. The language has no brackets anywhere else.

## 7. What it means

**Instantiation is unification at each call.** The checker types a call of a polymorphic declaration
by giving each variable a fresh unification variable, as it already gives a table's open element one
(types.md §3.1), and the IR's typing does the same with its own unifier. An explicit type argument is
an equation given in advance. When the call has been typed, each variable is a set, the call is the
instance, and **nothing after typing learns the declaration was polymorphic**: not the analyses, not
the IR, not a printer. The language stays monomorphic after staging (ADR 0009), and instantiating at
compile time is monomorphization, which Featherweight Generic Go proves correct for Go.

**An unsolved variable is refused, never defaulted.** A variable that no argument fixes and no type
argument gives is a program that has not said which function it calls. Defaulting it to the
language's `any`, the absence of a claim, would be ADR 0045's gradual type standing in for a
choice, and would relax every check that reads the result (the CLAUDE.md lesson on relaxed
relations). The refusal names the variable and suggests the explicit form.

**A bound is checked as membership.** At each instance, T ∈ K: for an interface bound, T ≤ I by the
target's subsumption; for a type set, T among its members. `(errors.As e (out int))` is refused at
compile time, where Go panics at run time: "As panics if target is not a non-nil pointer to either a
type that implements error, or to any interface type" (its documentation, which also says an
interface target may be one `error` is not below; §8).

**A program's polymorphic `sig` is checked parametrically.** Definitions are inlined, so every call
is checked at its instance anyway; what `sig`'s "for all A and B" adds is a claim about the body, and
the body is checked once with each variable an opaque atom that has only its bound's operations. That
is the claim's meaning (Reynolds' parametricity), and it is the check C++ templates lack and Go's and
Rust's generics make. Per-instance checking alone would accept a body correct for every type the
program happens to use and wrong for the next.

**The printers spell a type argument only where the host cannot infer it.** Go infers T from
`slices.Max(xs)`, so the call is printed as written; for `errors.AsType`, where T occurs only in the
result, the template has a hole for the type argument, `errors.AsType[%t1](%s)`, which the printer
fills with the instance's host spelling. The IR's call statement carries its type arguments, printed
in IR_P (ADR 0038: the text is the printers' whole input).

## 8. `errors.As` and `errors.AsType`, worked through

Go has both. `errors.As(err error, target any) bool` takes a pointer (Go 1.13); `errors.AsType[E
error](err error) (E, bool)` returns the value (Go 1.26). Both are As_E : error → E + 1, the
membership test of ⟦E⟧ inside ⟦error⟧, applied to the errors an error wraps. Those form a **tree**,
not a chain: since Go 1.20 an error may wrap several (`Unwrap() []error`), and As searches the tree
depth-first, in pre-order:

```
As_E(e) = some x   for the first x, in pre-order of e's tree, that matches E
          none     otherwise
```

**Matching is membership, or a claim of it.** Go's documentation: x matches E "if the type
assertion err.(E) holds, or if the error has a method As(any) bool such that err.As(target) returns
true". The first is x ∈ ⟦E⟧; the second lets an error declare itself a member of a type it is not,
with a value it supplies. So As_E is a membership test over a relation the errors themselves may
extend, which is why the declaration can state its result's type, `(option E)`, and nothing about
which errors land in `some`.

The two forms of target Go allows are the two kinds of bound (§2): a type below `error`, E ∈ ↓error,
and an interface, where As tests membership in ⟦E⟧ ∩ ⟦error⟧. The declaration's bound is therefore
"below `error`, or an interface", which the target can state as a type set (decision 2).

Declared:

```lisp
(sig (AsType (E error)) ((err error)) (option E)
     (host expr "errors.AsType[%t1](%s)" (import "errors")))
(sig (As (T error)) ((err error) (target (cell T))) (option T)
     (host expr "errors.As(%s, %s)" (import "errors")))
```

The error model's retraction (errors.md §4.2) already builds an `option` from Go's comma-ok pair,
`(E, bool)`: `some` the value when the bool is true. For `As` the pair is the bool and the cell, which
is ADR 0048's cell with the retraction over it. Either way Go's nil, the zero a failed test leaves,
never reaches the program: "no nil" (mutscope decision 6) held by the type.

Used:

```lisp
(case ((errors.AsType strconv.NumError) e)
  (some ne) (fmt.Println "cannot parse" (NumError.Num ne) "with" (NumError.Func ne))
  none      (fmt.Println "another error:" e))
```

and the Go printed:

```go
if ne, ok := errors.AsType[*strconv.NumError](err); ok {
	fmt.Println("cannot parse", ne.Num, "with", ne.Func)
} else {
	fmt.Println("another error:", err)
}
```

`AsType` is the better primitive here: no cell, no allocation, and Go's own newer form. `As` stays
for code that already holds a cell, and as the cell position's first witness. The first program needs
`targets/go/errors.oro` (`New`, `Is`, `As`, `AsType`, `Unwrap`, `Join`), `NumError`'s fields as
accessors, and `strconv.ErrSyntax` and `ErrRange`.

## 9. On each target

| target | what a declaration's variable becomes |
|---|---|
| Go | Go's generics: the call printed as Go writes it, with a type argument where Go cannot infer one |
| Android (Java) | Java's generics, erased by the JVM: a variable instantiated at the call and printed as Java's generic call or its erasure; the JVM survey's generic APIs (`java.util`) are declarable the same way |
| browser (JavaScript) | nothing: JavaScript has no types, a variable is solved for the checker and erases from the output |
| Windows | no Win32 API is generic; a variable never reaches the x86 printer |

The variable is typing, and every target accepts typing; what differs is only whether the host
wants the instance spelled.

## 10. What it would touch

- **The reader**: a `sig` head may be an application, `(f p…)`, each p a name or `(name bound)`,
  shaped like a variant's head.
- **The target loader**: a declaration's variables and bounds; type sets as target data (§11,
  decision 2); the template hole `%t1`.
- **The checker**: a fresh variable per call for each of a declaration's variables, solved with the
  arguments, checked against its bound, refused if unsolved; a `sig`'s body checked with each
  variable opaque.
- **The IR**: a call carries its type arguments, printed and read in IR_P; its typing solves them with
  the unifier it has.
- **The printers**: Go fills `%t1`; the others need nothing.
- **Not touched**: the reducer, the analyses, the refinement layer. A variable never reaches them.

## 11. Decisions for hamza

1. **The head binds, a binder is a name and its set**: `(sig (AsType (E error)) …)`, a bare name
   unbounded, `(name bound)` bounded, uniform with `(variant (result T E) …)`, with TLA+'s `\A x \in S`
   and with Go's `AsType[E error]`. *Recommended.*
2. **A bound is a set of types**: an interface I denotes ↓I, the types below it; a union is declared
   as target data, `(typeset cmp.Ordered int f64 string)`, with `comparable` derived by the target as
   one host value is. *Recommended.* (Go's `comparable` is the host's `==`, tier 2 beside the language's
   integer `=`.)
3. **Application instantiates**, `((errors.AsType strconv.NumError) e)`, inference where every
   variable is fixed by an argument, and **an unsolved variable is refused**, never defaulted.
   *Recommended.*
4. **A program's polymorphic `sig` is checked parametrically**, each variable opaque in the body,
   rather than only at the instances the program uses. *Recommended.*
5. **Build now** the positions that need nothing else: ordinary parameters, tables' elements,
   results, cells and variadic lists, and with them `errors.AsType`, `errors.As` and `cmp`/`slices`'
   callback-free half (51 names). Host generic types when a program needs one; callbacks with tier 1.
   *Recommended.*

## 12. Not here

- **Higher-rank types** (a quantifier inside a parameter's type), F<:'s full form: undecidable, and
  nothing in Go's library needs it.
- **Type-level computation**: no type families, no dependent types; a bound is a fixed set.
- **Inference across definitions**: a definition's own type is not inferred as a scheme; a `sig` states
  it, and the static level stays untyped until staging erases it (closures-direction.md).
- **Reflection** (`reflect.TypeFor`, `encoding/json/v2`'s `any` bounds): a type argument a function
  inspects is no longer parametric, and needs records and type values the language does not have.
