# The state of the language

Read off the code, not from memory. Everything here is checkable by grep, and the set of words it
describes is pinned by a test: [inventory.md](inventory.md) lists every word the compiler knows, and
`TestTheInventoryIsTheWordsTheCompilerKnows` fails when the two disagree.

Rewritten 2026-09-17. The previous version dated from 2026-08-27 and predated maps, the integer
operators, buffer parameters, `tuple` and `variant`, facts, and declarations as theories. It is in git
history.

---

## 1. The whole language

### Terms

**Seven term kinds.** That is the entire grammar of what a program can say.

```
term ::= name | integer | float | string | true | false
       | (fn (name…) term) | (term term…)
```

- **`KBound` is not an eighth.** `core.Term` has a further kind that no program can write: a bound
  variable stored as *"the parameter of the binder N levels out"*, the locally nameless representation.
  The names in `Params` are hints, kept so emitted code reads well.
- **The entry point** is an export named `main` taking no arguments ([build.md §2](build.md)).
- **`fn` is also spelled `λ`.**

### Top-level forms of a program

Six, all erased before reduction except `def`:

```
(def name term)
(sig name ((param type)…) result [(where pred)] [(ensures pred)])
(variant name (constructor payload…)…)      ; or (variant (name T…) …) with type arguments
(module path)  (use path [as alias])  (export name…)
```

- **`def`** introduces a definition. Its name is simple, because `.` qualifies an imported member
  ([def.md](def.md)).
- **`sig`** is a claim checked in two directions: against the definition's residual, and against a
  target that provides the name natively ([types.md](types.md)). In it:
  - a range in a parameter is a premise;
  - `where` has three meanings ([refinements.md §6b](refinements.md));
  - `ensures` is its exact swap ([postconditions.md](postconditions.md)).
- **`variant`** is closed, finite and non-recursive ([sums.md](sums.md), [data.md §5](data.md)). Its
  declaration generates ordinary definitions: constructors and tag constants. A variant is its
  QUALIFIED declaration, and a pattern resolves in its module. `option` is one declaration, in `lang`.
- **`module`, `use` and `export`** are resolution, not reduction ([modules.md](modules.md),
  [ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md)).

A library directory may also hold `(provides TARGET PATH …)` fragments. These are the target's own
declarations and definitions for that module, which is `D_T` ([target-system.md](target-system.md),
[theories.md §5.4](theories.md)).

### Reader sugar

**None survives the reader**, except `case`.

| written | reads as | spec |
|---|---|---|
| `(let e (fn (x) b))` | `((fn (x) b) e)` | [def.md](def.md) |
| `(seq a b)` | `((fn (_) b) a)` | [effects.md §5](effects.md) |
| `(and a b)`, `(or a b)`, `(not a)`, `cond` | `if` | [booleans.md](booleans.md) |
| `(tuple a b …)` | `(fn (k) (k a b …))` | [data.md](data.md), [values.md](values.md) |
| `(match (e…) pats body … else body)`, with `when` guards and `_` | a `loop` | [match.md](match.md) |
| `(loop ((x z)…) clauses… else e)` with `again` | `(loop (fn (x…) …) z…)` | [iteration.md](iteration.md), [ADR 0015](../decisions/0015-loop-and-again.md) |
| `(case e (ctor x…) body …)` | `if` over a tag comparison, **in `Load`**, because the variant may be declared in another file | [sums.md](sums.md) |

**A `loop` with no `again` is not a loop.** The name is dropped, and what remains is a β-redex
([match.md §5b](match.md)).

**Old spellings are refused naming the new one:** `sum` → `variant`, `values` → `tuple`
([data.md §10](data.md)).

### Names the language owns

These are injected into every target. **Declaring one is an error**, in a target file as much as in a
program; the backend implements each on each host.

| names | what | spec |
|---|---|---|
| `if`, `let`, `loop` | control and binding | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md), [ADR 0015](../decisions/0015-loop-and-again.md) |
| `=` | integer equality | [match.md](match.md) |
| `+ - * / % < <= > >=` | integer arithmetic and order, found per target by spelling | [integers.md §0a](integers.md) |
| `array`, `table`, `len` | a table's graph, its rule and its domain bound; indexing is APPLICATION | [tables.md](tables.md) |
| `alloc`, `build`, `set` | allocation and the scoped linear buffer | [tables.md](tables.md), [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) |
| `map`, `build-map`, `insert`, `keys` | maps over `int` keys | [maps.md](maps.md) |
| `concat`, `string-of` | the free monoid over scalars, and its generator | [string-operations.md](../string-operations.md) |
| `the` | a range ascribed to a term, erased at emission | [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md) (no spec yet) |

**Indexing has no word at all.** `(a i)` is an application, because a table is a function with a known
finite domain, and a named indexing operation would be a second spelling. The bounds obligation is
generated from the form.

### The type language

- **Base types:** `int`, `f64`, `bool`, `string`.
- **A range** `(int LO HI)` is a type. Its endpoints are compile-time expressions over literals, unary
  `-`, `+ - *`, `pow` and `+inf`/`-inf`, and in a target file its constants.
- **Tables:** `(array V)` and `(map K V)`.
- **`(buffer V)`**, a linear parameter type ([ADR 0020](../decisions/0020-uniqueness-on-parameters.md)).
  A buffer may not be an element type.
- **`(tuple A B …)`**, and a variant applied to its type arguments.
- **`record` is reserved**, specified in data.md and not built.

A range means three things, kept apart:
- an `int` for typing;
- a premise for the analyses;
- a representation for storage.

The representation is chosen by the target: `(repr (int LO HI) …)` below the word and `(repr big …)` above it
([ADR 0003](../decisions/0003-range-typed-integers.md), [ADR 0019](../decisions/0019-precision-by-declaration.md)).

### Reduction

**Four rules**, two of them with two clauses.

| | |
|---|---|
| **β**, call-by-need | An impure argument is let-bound rather than substituted ([effects.md §4](effects.md)); a table read through a bound variable is not substituted into an impure body (§7c). **β-tab** is its second clause: a table or map written as a graph, applied to a literal, is looked up |
| **δ** | unfolding a definition, declining a cycle; a target's native name wins over a library's (`▷`) |
| **evaluation on literals** | `(if true a b) → a`; the language's integer operators and `=` on two integer literals, **only inside the portable window and never dividing by zero** ([ADR 0009](../decisions/0009-staging-preserves-results.md)). No float folds, and no primitive of a target is ever evaluated |
| **commuting conversion** | push an eliminator through `if` and `let` (case-of-case), only when every argument is pure |

**No recursion** ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). A definition in
terms of itself is an error, checked per target before reduction. δ still declines to unfold a cycle,
so the reducer stays correct on a term the front end rejects. Self-application still diverges and is
guarded by fuel ([pcf.md §9](pcf.md)).

**Two parameters,** both supplied by a target:
- which names are primitive ([ADR 0002](../decisions/0002-capability-graph.md));
- which of those are pure. This defaults to *impure*, so an omission costs speed rather than
  correctness.

**Size:** 5,163 lines in `core/`, with 146 tests there and 309 in `emit/`.

### What runs after reduction

These are not language, but they decide what is legal.
- **The type checker** runs on the residual, which is monomorphic, first-order and closed
  ([types.md](types.md)).
- **Linearity** is checked by type (`CheckLinear`, ADR 0018 and ADR 0020).
- **The refinement layer** discharges obligations in linear integer arithmetic by Fourier–Motzkin
  entailment, with facts instantiated from `lang` and the target
  ([refinements.md](refinements.md), [theories.md §7](theories.md)).
- **The interval analysis** decides that every integer operation stays inside ±(2⁵³−1) **or the
  program is refused** (ADR 0019).
- **Size-change termination** reports which loops are proven to terminate.

---

## 2. What a program may *not* say

- **`(prim …)` and `(target …)` are errors.** Primitives come from `targets/NAME/*.oro` and from
  `provides` fragments, and nowhere else. They were once silently accepted, which let a program believe
  it had declared something.
- **A program may not declare a language name** (§1's owned names), nor a variant colliding with
  `lang`'s in the main module.
- **An integer operation not proven inside the window is a compile error.** It is cleared by
  narrowing a range, declaring one above the window, or `-checked`
  ([ADR 0019](../decisions/0019-precision-by-declaration.md)).
- **A buffer may not be used after it is consumed**, and an immutable array may not reach a parameter
  declared a buffer.
- **A closure may not survive staging**, and all four backends refuse one
  ([callbacks.md](callbacks.md)).
- **`lemma` and `forall` are reserved**: facts beyond F-B are specified and not built
  ([theories.md §7](theories.md)).

## 3. What is absent, and deliberately

| | Status |
|---|---|
| Type annotations on terms | none. `sig` is a claim about a definition, and a range on a term is `the`, erased |
| Recursion and recursive data | **rejected** (ADR 0014); recursive data is a flat table plus indices |
| Unbounded iteration | **built**, `loop`/`again`; termination is a computed program property |
| Tail-call optimisation | not guaranteed, and moot |
| Escaping closures | refused; three tiers of what that costs are in [callbacks.md](callbacks.md) |
| Mutation | only inside `build`, on a linear buffer, or on a `(buffer V)` parameter (ADR 0018, ADR 0020) |
| Effect types, monads | none. Purity is one declared bit per primitive ([effects.md](effects.md)) |
| General equality | none. `=` is integer equality; floats have NaN, and strings have no portable comparison |
| Bitwise operators | not promoted to the language: V8 truncates them to int32 inside the window ([integers.md](integers.md)) |
| String indexing, `length` | not in the language: three hosts give three answers, and three text programs needed neither ([strings.md](strings.md)) |
| Records, symbols, `with`, views | specified ([data.md](data.md), [theories.md](theories.md)) and not built. def.md §5's refusal of *runtime* symbols stands; data.md's `'x` labels never survive staging |
| Extensionality | none; η |

## 4. Effects

[effects.md](effects.md) was written before any code, the first time this project used that order.

> **Purity is the licence to use the structural rules.** A pure term may be copied, dropped and moved.
> An impure term is *ordered*: exactly once, where it was written.

Denying contraction, weakening and exchange to impure terms is the whole discipline. It needs no effect
types, no monads, and no linear types on values. Buffers are the one linear thing, and they are a type
because a host call can hand one back ([host-buffers.md](../host-buffers.md)).

## 5. What the language has that is unusual

- **Generics with no generics.** One definition instantiates at any number of types, with no type
  parameters, no monomorphization pass and no dictionary.
- **Fusion with no fusion rules.** β and δ alone; the intermediate structure never exists.
- **The normal form is a parameter.** One source, four targets, and each target's choice of host
  construct comes from what was measured faster there.
- **Sequencing with no sequencing construct.** `seq` is a β-redex whose binder is unused, and it works
  because β refuses to drop an impure argument.
- **A variant that costs nothing at either level.**
  - A tag known at compile time reduces away by β.
  - A tag decided at run time reduces to the `if` that decided it, by the commuting conversion.
- **Exhaustiveness that REMOVES a branch.** A variant is closed, so the last clause needs no test.
- **A product that is currying.** A table of tuples is flattened before the checker runs
  ([products.md](products.md)), so no backend knows products exist.
- **Proofs instead of checks.**
  - An index, an integer operation or a table's contents that the compiler proves costs nothing at
    run time.
  - One it cannot prove is a compile error, never a silent trap.

## 6. The rule this document exists to enforce

**Nothing goes into the language without a specification that says how it behaves on every target.**
String literals were added without one, and [strings.md](strings.md) is the correction.

The test for a proposed addition is not "is it useful" but:
1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement observable? If so, the feature is Tier 2 and carries no
   portability claim.

The words that are not yet specified are listed in [inventory.md §6](inventory.md), and the test keeps
that list honest.
