# The state of the language

Read off the code, not from memory. Everything here is checkable by grep, and the words it describes
are pinned by two tests: [inventory.md](inventory.md) lists every word the compiler knows
(`TestTheInventoryIsTheWordsTheCompilerKnows`), and [j.md](j.md) §1.4 every keyword J's reader takes
(`TestJsKeywordsAreTheReadersKeywords`). Each fails when its document and the code disagree.

Rewritten 2026-10-25 in J. The surface is J ([j.md](j.md), [ADR 0068](../decisions/0068-j-is-the-surface.md)),
and the examples here are J's. **Two levels are shown, and kept apart**: what a source file says, in J,
and the core term J's reader makes of it by D ([abstract.md](abstract.md) §8, `core/desugar.go`),
which the reducer and everything after it read. A core term is printed as an s-expression,
`(def f (fn (x) (+ x 1)))`; nothing writes one by hand but the tests of the core and the IR's text.
The version before, in the notation the J switch replaced, is in git at `1e2bdcd`.

---

## 1. The whole language

### The core's terms

**Seven term kinds.** That is the entire grammar of what reduction sees.

```
term ::= name | integer | float | string | true | false
       | (fn (name…) term) | (term term…)
```

- **`KBound` is not an eighth.** `core.Term` has a further kind that no source can write: a bound
  variable stored as *"the parameter of the binder N levels out"*, the locally nameless representation.
  The names in `Params` are hints, kept so emitted code reads well.
- **A residual's hints are apart** (`core/hygiene.go`), by two invariants on what reduction returns:
  no binder's hint equals a name its body refers to past it, so opening a body by its hints captures
  nothing; and **no binder's hint equals an enclosing binder's**, whether or not its body mentions
  that variable. The second is the variable convention (Barendregt 1984, 2.1.13) along the scope
  chain, and it is what a consumer carrying a context keyed by name needs: the refinement layer's
  facts about a parameter `n` were read as facts about a loop variable `n`, and an index of 101 into
  a table of 6 was accepted on every target (bufio-2026-10-01). Sibling binders keep their names
  ([ADR 0036](../decisions/0036-a-residuals-binders-are-named-apart.md)).

### A source file

A file is a theory named by its path ([ADR 0064](../decisions/0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md),
j.md §2.1–2.2). Its declarations:

```go
use go/os                            // an import, its alias the path's last segment
use result as r                      // or one given
module companion { … }               // a child module

cap = 4096                           // a value
fn f(x ∈ 0..255, b ∈ Array(Int) linear) (r ∈ Int) where x < len(b) ensure r >= 0 { … }
set Kind = 'not_found ∪ 'other(error) success 'not_found relevant
pub fn area(w, h) { … }              // in the interface: another module may name it
fn helper(x) { … }                   // private to this module, the default
export fn main() { … }               // an entry point
```

- **A comment is `//` to the end of the line**, and it is **gap**, not a token: the lexer erases it, so
  no pass below the reader can see one and no emitted file carries one. There are no doc comments, no
  pragmas and no block comments ([comments.md](comments.md), [ADR 0024](../decisions/0024-comments-are-erased.md)).
  The IR's text and a printed core term keep `;`.
- **`fn f(…) … { e }` is a definition**, a named λ, and its header carries its claims: each binder's
  set, a mode (`linear`, `inout`, `out`), the result, `where φ` and `ensure ψ` ([def.md](def.md),
  ADR 0051, 0062). D makes two core forms of it, **`def`**, the named λ, and **`sig`**, its claim:
  `(def f (fn (x b) …))` and `(sig f ((x (int 0 255)) (b (buffer int))) int (where …) (ensures …))`.
  `def` and `sig` are the core's words, written by no source file (inventory.md §1).
- **The claims are checked** ([types.md](types.md), [refinements.md](refinements.md)):
  - a parameter's set and `where` are obligations at every call a definition is inlined into, and
    assumed only at an export's own boundary ([ADR 0028](../decisions/0028-a-definitions-contract-is-checked-at-its-calls.md),
    refinements.md §6b); above the word, a range is the set a sign and a bit length decide
    ([ADR 0029](../decisions/0029-above-the-word-one-set-on-every-representation.md));
  - the result's set and `ensure` are a claim decided at the definition, against the body, exported
    or not, called or not ([ADR 0072](../decisions/0072-a-declared-result-is-a-claim-decided-at-its-definition.md),
    [postconditions.md](postconditions.md));
  - every name a header uses as a set must denote one ([ADR 0073](../decisions/0073-a-sets-name-denotes-a-set.md)).
- **`set S = …` with tags is a variant**, closed, finite and non-recursive, with type arguments,
  `set Result(T, E) = 'ok(T) ∪ 'err(E)` ([sums.md](sums.md), [data.md §5](data.md)). D makes the core's
  `variant`, which generates ordinary definitions: constructors and tag constants. A variant is its
  qualified declaration, and a pattern resolves in its module. `option` is one declaration, in `lang`.
- **`use`, `module`, `pub` and `export`** are resolution, not reduction ([modules.md](modules.md),
  [ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md)). A declaration is private to its
  module unless marked `pub`, which puts it in the interface an importer sees; `export` marks an entry
  point, the core's `(pub …)` and `(export …)`, two relations ([ADR 0078](../decisions/0078-a-modules-interface-is-what-it-marks-pub.md)).
  The build's entry is `export fn main()`, taking nothing ([build.md §2](build.md)).

A library directory may also hold `provides TARGET PATH { … }` fragments: the target's own
declarations and definitions for that module, `D_T` ([target-system.md](target-system.md),
[theories.md §5.4](theories.md)).

### From J to the core term

**Nothing of the surface survives the reader except `case`**, and the two forms that expand into it.
J's reader builds abstract Oroboros's trees (`core/abs.go`, `core/decl.go`; [abstract.md](abstract.md),
ADR 0050), and D makes each core term below. ADR 0015's tail rule is a sort of the trees, not a walk
over terms (abstract1-2026-10-11). A target file's declaration is a definition with a host body whose
claim carries usage as modes (`core/jreadtarget.go`, `emit/hostdef.go`).

| J | the core term | spec |
|---|---|---|
| `fn f(x, y) { e }` | `(def f (fn (x y) e))`, and the header's claims a `sig` | [def.md §4](def.md) |
| `{ x = e; (a, b) = p; body }` | nested applications: `((fn (x) …) e)`, and `(p (fn (a b) body))` for a tuple pattern | [binding.md](binding.md), ADR 0054 |
| `{ e; body }`, a statement | `((fn (_) body) e)`, a binding whose name is discarded | [binding.md §6](binding.md), [effects.md §5](effects.md) |
| `{ var b := array { 0 : _ ∈ 0..n-1 }; b[i] := v; … }` | per binder, by its initializer: a fresh table makes a **buffer**, the core's `(build n (fn (b) …))`, its stores in program order threaded by S, the state translation (`core/place.go`, R0–R4); `map(cap)` a **map buffer**, `(build-map cap (fn (m) …))`; anything else a **cell**, translated into loop variables after `case` expands, so nothing below the loader learns cells exist, each elimination it assumes marked `((#elim op) λ)` and decided on op's normal form | [local.md](local.md), [ADR 0047](../decisions/0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md), [ADR 0052](../decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), [ADR 0058](../decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md) |
| `a && b`, `a \|\| b`, `!a`, `a != b`, `case { φ -> a; else -> b }` | `if`; and `(if (not c) a b)` is built as `(if c b a)`, unless a branch is a boolean literal and the term is a connective | [booleans.md §4.2](booleans.md) |
| `(a, b)`, `()` | `(fn (k) (k a b))`, `(tuple)` | [data.md](data.md), [values.md](values.md) |
| `loop x = z { φ -> again(e); else -> r }` | `(loop (fn (x…) …) z…)` | [iteration.md](iteration.md), [ADR 0015](../decisions/0015-loop-and-again.md); `again` may sit under a binding ([ADR 0027](../decisions/0027-a-host-calls-continuation-is-a-tail.md)), in a `try` or `expect` success arm ([ADR 0041](../decisions/0041-a-success-arm-is-a-tail.md)), and in each arm of a `case` in a tail ([ADR 0076](../decisions/0076-a-case-in-a-tail-is-a-tail-when-its-arms-are.md)) |
| `case e { 'c(x) -> a; else -> b }` | `if` over a tag comparison, **in `Load`**, because the variant may be declared in another file | [sums.md](sums.md) |
| `try p = e { body }`, `expect p = e else why` | a `case` in `Load`, on a variant marked `success`: `try` returns every other constructor unchanged, `expect` crashes with `why` through `abandon` | [errors.md](errors.md) §5, §8 |
| `b ∈ S = e` | the binder's claim: an obligation, proven or refused, then the core's `(the S e)`, which the IR reads | [ADR 0069](../decisions/0069-an-observer-at-a-host-parameter.md) |
| `f(x, ys...)`, `f(out S)` | `(f x (spread ys))`, `(f (out S))`, rewritten by the loader | [variadic.md](variadic.md), [local.md §6](local.md) |

**A `loop` with no `again` is not a loop.** The name is dropped, and what remains is a β-redex
([match.md §5b](match.md)).

What J does not build yet (`spec`, `lemma`, `assert`, quantifiers, a case arm's guard, `try`'s handlers,
a λ's header) is j.md's status table.

### Names the language owns

These are injected into every target, and the backend implements each on each host. They are Σ_L, the
language's signature (`core.LanguageSignature`): the names J's reader and `Load`'s expansions emit bare.
**A target file declaring one is an error, and so is a module of a program defining one** (§2).

| the core's names | J writes | what | spec |
|---|---|---|---|
| `if`, `let`, `loop` | `if c { a } else { b }`, a block, `loop` | control and binding | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md), [binding.md](binding.md), [ADR 0015](../decisions/0015-loop-and-again.md) |
| `=` | `==` | integer equality | [match.md](match.md) |
| `+ - * / % < <= > >=` | the same | integer arithmetic and order, found per target by spelling | [integers.md §0a](integers.md) |
| `array`, `table`, `len` | `array { 2, 3 }`, `array { e : k ∈ 0..n-1 }`, `len(t)` | a table's graph, its rule and its domain bound; indexing `t[i]` is APPLICATION | [tables.md](tables.md) |
| `alloc`, `build`, `set`, `local` | `var b := …`, `b[i] := v`, `x := v` | allocation, the scoped linear buffer, and a scope of local state | [tables.md](tables.md), [local.md](local.md), [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md), [ADR 0047](../decisions/0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md) |
| `map`, `build-map`, `insert`, `keys` | `map { 1 : 10 }`, `var m := map(cap)`, `m[k] := v`, `keys(m)` | maps over `Int` keys | [maps.md](maps.md) |
| `concat`, `string-of` | `a ++ b`, `string_of(n)` | the free monoid over scalars, and its generator | [string-operations.md](../string-operations.md) |
| a host's string | `go.bytestring` | its own type, with `String` a subtype of it; it enters `String` only through d, the maximal-subpart decode (`go.text`, and the portable `os.text_of` on every host) | [ADR 0030](../decisions/0030-a-hosts-string-is-the-hosts.md), [strings.md §8](strings.md) |
| `abandon` | `expect`'s failure | crash : E → 0, a statement taking the reason; found per target by spelling, and impure. Its value is in 0 | [errors.md](errors.md) §8–9 |
| `the` | `b ∈ S = e` | a range ascribed to a term, erased at emission; J refuses `the` itself | [ascribe-2026-09-03](../../gauntlet/results/ascribe-2026-09-03.md) (no spec yet) |

**Indexing has no word at all.** `t[i]` is an application, because a table is a function with a known
finite domain, and a named indexing operation would be a second spelling. The bounds obligation is
generated from the form.

### The sets

- **Base sets:** `Int`, `F64`, `Bool`, `String`; the core spells them `int`, `f64`, `bool`, `string`.
  **`Nat`** is ↑0, read as `0..` (ADR 0079).
- **A range** `LO..HI` is a set, the core's `(int LO HI)`. Its endpoints are compile-time expressions
  over literals, unary `-`, `+ - *` and `^`, and in a target file its constants. **An end may be omitted**:
  `lo..` is ↑lo and `..hi` is ↓hi, the core's `(int lo +inf)` and `(int -inf hi)` (ADR 0077).
- **Tables:** `Array(V)` and `Map(K, V)`.
- **A buffer parameter** is `b ∈ Array(V) linear`, the core's `(buffer V)`
  ([ADR 0020](../decisions/0020-uniqueness-on-parameters.md), [ADR 0052](../decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md));
  a buffer may not be an element type. A cell a host writes is `x ∈ T inout`, the core's `(cell T)`.
- **Products** `A × B` and the unit `()`; a variant applied to its arguments, `result.Result(T, E)`;
  a static parameter `E ⊆ S` in a host declaration ([ADR 0063](../decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md)).
- **`record` is reserved**, specified in data.md and not built.

A range means three things, kept apart:
- an `Int` for typing;
- a premise for the analyses;
- a representation for storage.

The representation is chosen on the IR from the proven fact, by what the target declares, `repr LO..HI
…` below the word and `repr big …` above it ([ADR 0003](../decisions/0003-range-typed-integers.md),
[ADR 0033](../decisions/0033-an-integers-representation-is-chosen-on-the-ir.md)). **A declared set is a
claim and names no representation** ([ADR 0079](../decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md),
[integers.md §1a](integers.md)): a claim above the word is the consent to the rung above, which holds a
value exactly only where its fact leaves the word, and is the representation itself only where the claim
is the fact, an entry point's parameter or result.

### Reduction

On the core's terms. **Four rules**, two of them with two clauses, and η for products, the
extensionality law that lets β see a bound product (sumloop-2026-10-06).

| | |
|---|---|
| **β**, call-by-need | An impure argument is let-bound rather than substituted ([effects.md §4](effects.md)); a table read through a bound variable is not substituted into an impure body (§7c); a pure argument is substituted when its hole is evaluated at most once, counted in {0, 1, ω} with a loop's body and a table's rule ω, and otherwise only if it is a value, a let around a value floated out first (§7d, ADR 0071). **β-tab** is its second clause: a table or map written as a graph, applied to a literal, is looked up |
| **δ** | unfolding a definition, declining a cycle; a target's native name wins over a library's (`▷`) |
| **evaluation on literals** | `(if true a b) → a`; the language's integer operators and `=` on two integer literals, **only inside the target's word (ADR 0026), checked against int64 overflow, and never dividing by zero** ([ADR 0009](../decisions/0009-staging-preserves-results.md)). No float folds, and no primitive of a target is ever evaluated |
| **η for products** | a binding whose value has every tail an n-tuple is taken apart where it stands, `(let V (fn (r) B)) → (V (fn (c̄) B[r := (tuple c̄)]))`, so each `(r k)` is a β-redex; tails through `if`, `let`, a host call's continuation, a scope and a loop's exits, and anything else is left bound ([ADR 0041](../decisions/0041-a-success-arm-is-a-tail.md), sumloop-2026-10-06). **n = 0** is η for the terminal object, r = () for every r : 1, so a unit's binding is taken apart into no components ([errors.md §7.3](errors.md)) |
| **`ignore`** | !_A : A → 1, the language's discard, by the terminal object's universal property: `ignore(e)` is `(e (fn (x̄) (tuple)))` when e's tails are n-tuples, the unit when e is pure, and `(let e (fn (_) (tuple)))` otherwise; it never reaches a target ([errors.md §7.3](errors.md)) |
| **variadic calls**, at loading | a call of a declaration marked `variadic` is the declaration applied to the word its trailing arguments spell, `f(x, a₁, …, aₙ)` ⟶ `(f x (array a₁ … aₙ))`, and `f(x, xs...)` ⟶ `(f x xs)`; rewritten once, from the target's forms, so nothing below the loader sees one ([variadic.md](variadic.md), ADR 0046) |
| **relevance**, a side condition on β | a binder the program wrote, given a value with a relevant constructor's tuple (`#k!`) among its tails, must be used on every path of the λ as written: `if` additively, `abandon` vacuously, `again` vacuously for a variable bound outside its loop and strictly for one bound inside the iteration (abstract2-2026-10-11), a hand-off as the receiver's parameter is used. Otherwise the program is refused ([errors.md §7](errors.md), ADR 0043) |
| **commuting conversion** | push an eliminator through `if`, `let` and a multi-result host call's continuation — the n-ary let, `(((p a…) (fn (x̄) M)) k…) → ((p a…) (fn (x̄) (M k…)))` — only when every argument is pure (u128-2026-09-23). **At arity 0** it is the crash's absorption: `abandon w` is a term of the empty sum, whose eliminator is the empty copairing, and K ∘ [] = [], so a primitive's application with a crash in a strict position (an argument not a λ, an `if`'s condition) is the crash, after the impure arguments before it, kept in order ([errors.md §8](errors.md)) |

**No recursion** ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). A definition in
terms of itself is an error, checked per target before reduction. δ still declines to unfold a cycle,
so the reducer stays correct on a term the front end rejects. Self-application still diverges and is
guarded by fuel ([pcf.md §9](pcf.md)).

**Two parameters,** both supplied by a target:
- which names are primitive ([ADR 0002](../decisions/0002-capability-graph.md));
- which of those are pure. This defaults to *impure*, so an omission costs speed rather than
  correctness.

**Size:** 14,316 lines of Go in `core/` outside its tests, 19,773 with them; 205 test functions there,
394 in `emit/`.

### What runs after reduction

These are not language, but they decide what is legal.
- **The type checker** runs on the residual, which is monomorphic, first-order and closed
  ([types.md](types.md)).
- **Linearity** is checked on the residual (`emit.CheckLinear`, ADR 0018 and ADR 0020), after R0–R4
  on each place's scope in the reader (ADR 0052).
- **The refinement layer** discharges obligations in linear integer arithmetic by Fourier–Motzkin
  entailment, with facts instantiated from `lang` and the target
  ([refinements.md](refinements.md), [theories.md §7](theories.md)).
- **The IR** ([ir.md](ir.md), ADR 0032) decides that every integer operation stays inside its target's
  word **or the program is refused** (ADR 0019, 0026), chooses each value's representation, and proves
  termination by size-change termination, reporting the loops it cannot.

---

## 2. What a program may *not* say

- **A host body, an `axiom` and the model's data are a target file's** (j.md J5): a program writing one
  is refused by the reader. Primitives come from `targets/NAME/*.oro` and from `provides` fragments,
  and nowhere else.
- **No module defines a language name, and no target file declares one** (§1's Σ_L), nor a program a
  variant colliding with `lang`'s in the main module. A definition introduces a fresh name (a
  definitional extension); one of Σ_L's would redefine a symbol J's syntax is read as. Until
  defects-2026-10-26 it was accepted: in a nested module `fn if(c, a, b) { 7 }` captured every `if` in
  the module, and a function answered 7 whatever its argument; in the root it was ignored beside the
  target's, with a note.
- **A set has no representation** (ADR 0049): a set operator (`∈ ∉ .. ∪ ∩ ∖ × ⊆ ⊂`) in a computation or a
  claim's formula is refused saying what it is; a set stands in a binder, a binder's claim, a former or
  a `set` declaration. Membership as a test, χ_S(e), is not built; for a range the message gives the
  derived `lo <= e && e <= hi`.
- **An integer operation not proven inside its target's word is a compile error on that target**
  (ADR 0026: the word is data, `repr LO..HI word`; Go also realizes 0..2^64-1 as `uint64`,
  ir/words.go). It is cleared by narrowing a range, declaring one above the word, or `-checked`.
  Portability across targets is computed and reported (`cmd/portable`), not assumed
  ([ADR 0019](../decisions/0019-precision-by-declaration.md)).
- **A buffer is used in program order and at one place** (ADR 0052's R0–R4), and an immutable table
  may not reach a parameter declared `linear`.
- **A closure may not survive staging**, and all four backends refuse one
  ([callbacks.md](callbacks.md)).
- **`lemma` and `forall` are reserved** in a target file's facts: facts beyond F-B are specified and
  not built ([theories.md §7](theories.md)). A program's `spec`, `lemma` and `assert` (ADR 0062) are not
  built.

## 3. What is absent, and deliberately

| | Status |
|---|---|
| Type annotations on terms | none. A header's claims are about a definition, and a value's set is a binder's claim, `b ∈ S = e`, proven |
| Recursion and recursive data | **rejected** (ADR 0014); recursive data is a flat table plus indices |
| Unbounded iteration | **built**, `loop`/`again`; termination is a computed program property |
| Tail-call optimisation | not guaranteed, and moot |
| Escaping closures | refused; three tiers of what that costs are in [callbacks.md](callbacks.md) |
| Mutation | only of a state binder, `var`, in a block: a buffer, a map buffer or a cell, or of a `linear` parameter (ADR 0018, 0020, 0047, 0052). A cell is a loop variable after translation; a closure over one is refused |
| Effect types, monads | none. Purity is one declared bit per primitive ([effects.md](effects.md)) |
| General equality | none. `==` is integer equality; floats have NaN, and strings have no portable comparison |
| Bitwise operators | not promoted to the language: V8 truncates them to int32, inside its own 2⁵³ word ([integers.md](integers.md)) |
| String indexing, `length` | not in the language: three hosts give three answers, and three text programs needed neither ([strings.md](strings.md)) |
| Records, symbols, `with`, views | specified ([data.md](data.md), [theories.md](theories.md)) and not built. def.md §5's refusal of *runtime* symbols stands; a tag `'x` never survives staging |
| Extensionality | none; η |

## 4. Effects

[effects.md](effects.md) was written before any code, the first time this project used that order.

> **Purity is the licence to use the structural rules.** A pure term may be copied, dropped and moved.
> An impure term is *ordered*: exactly once, where it was written.

Denying contraction, weakening and exchange to impure terms is the whole discipline. It needs no effect
types, no monads, and no linear types on values. Buffers are the one linear thing, and they are a mode
because a host call can hand one back ([host-buffers.md](../host-buffers.md), ADR 0070).

## 5. What the language has that is unusual

- **Generics with no generics.** One definition instantiates at any number of types, with no type
  parameters, no monomorphization pass and no dictionary.
- **Fusion with no fusion rules.** β and δ alone; the intermediate structure never exists.
- **The normal form is a parameter.** One source, four targets, and each target's choice of host
  construct comes from what was measured faster there.
- **Sequencing with no sequencing construct.** A statement is a β-redex whose binder is unused, and it
  works because β refuses to drop an impure argument.
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
