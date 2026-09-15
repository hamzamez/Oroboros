# Data: functions on a domain

**Status: specification, not built under these names.** Four of the six forms below exist today
under older spellings. This document fixes the spelling, and says what each form is and what it
lowers to. The design came from [theories.md](../theories.md) §4 and
[theories-b-or-c.md](../theories-b-or-c.md) §8, and the vocabulary was agreed with hamza on
2026-09-15. The declaration forms that *name* these types are specified in [theories.md](theories.md).

> **1. What does it mean, independently of any target?** Every data form is a function. The six
> forms differ only in their **domain** and in **when that domain is known** (§1).
>
> **2. What does each target do with it, and do they agree?** Nothing new. Every form here lowers to
> a mechanism that already exists on all four targets and already has a conformance case:
> - a tuple to values.md's multiple results and products.md's flattening;
> - a record to the tuple of its canonical order;
> - a variant to sums.md;
> - a symbol to nothing, because it does not survive staging (§7).
>
> **3. Is any disagreement observable?** None is possible: no backend receives anything it did not
> receive before. **Tier 1.** §9's conformance cases check it anyway, and each one can fail.

---

## 0. The vocabulary

| form | is | today |
|---|---|---|
| `(fn (x…) e)` | a function on the whole of a type | built |
| `(array V)` | a function on `[0, len)`, homogeneous, length known at run time | built ([tables.md](tables.md)) |
| `(map K V)` | a function on a finite subset of `K`, known at run time | built ([maps.md](maps.md)) |
| `(tuple A B …)` | a function on `{0, …, n−1}`, known statically | built as `values` and as `(array A B)` |
| `(record ('x A) …)` | a function on a static set of labels | **not built** |
| `(variant NAME (c A) …)` | a function defined at **exactly one** label | built as `sum` ([sums.md](sums.md)) |
| `'x` | a label: a name used as data | **not built** |

**Retired when this is built:** `sum`, `values`, the type `(array A B)`, and the heterogeneous
literal `(array a b)`. Each retired spelling is refused with a message naming its replacement (§10).

---

## 1. The semantics: everything is a function

### 1.1 Six forms, one question

A value of every form here is a function, and what distinguishes the forms is the domain:

| form | domain | domain known | application `(v k)` is defined when | discharged by |
|---|---|---|---|---|
| `fn` | all of `A` | statically | always | the type |
| `tuple` | `{0 … n−1}` | statically | `k` is a literal in range | the type |
| `record` | a label set `L` | statically | `k` is a label in `L` | the type |
| `array` | `[0, len)` | at run time | `0 ≤ k < len` | a proof (the refinement layer) |
| `map` | a finite `S ⊆ K` | at run time | `k ∈ S` | a **value**: the result is an `option` |
| `variant` | one label of `L` | at run time | `k` is *that* label | a **branch**: `case` |

This extends [tables.md](tables.md) §1.1's three points (function, array, map) to six. It is the
mathematics of TLA+, where a record `[x ↦ 1, y ↦ 2]` is a function on `{"x", "y"}` and a tuple
`⟨a, b⟩` is a function on `1..2` (Lamport, *Specifying Systems*, 2002).

### 1.2 A variant is a function defined at one point

The sum `Σ_{l ∈ L} T_l` is ordinarily defined as a set of pairs `(l, t)` with `t ∈ T_l`.

**Theorem V.** The map `(l, t) ↦ {l ↦ t}` is a bijection from `Σ_{l∈L} T_l` to the set of
dependent functions `f` with `dom f ⊆ L`, `|dom f| = 1` and `f(l) ∈ T_l`.

*Proof.* The map is injective: the singleton function `{l ↦ t}` determines both `l` (its only
domain point) and `t` (its value there). It is surjective: a function `f` with `dom f = {l}`
is the image of `(l, f(l))`. ∎

So a record and a variant are the same kind of object. **A record is defined at every label, and
a variant at exactly one.** Eliminating a record needs no decision, because every label is known to
be in the domain. Eliminating a variant must first find *which* label is in the domain, which is a
branch on a fact the compiler does not have — sums-research.md's *"the tag is information the caller
does not have"*. That is why the surface has projection for records and `case` for variants, and
no application form for a variant (§5.4).

### 1.3 What is not here, and why

- **Recursive types.** A type whose definition mentions itself breaks the well-founded order of
  declarations ([theories.md](theories.md) §2). Refused, as in type-algebra.md §3.1.
- **Untagged unions.** A set union is idempotent, `A ∪ A = A`, so it is not a coproduct. TLA+ can
  use one because it is untyped. Refused, as in type-algebra.md §3.3.
- **Width subtyping on records.** A record with more labels is not usable where fewer are expected.
  Subtyping is refused in general (type-algebra.md §3.2). The host-interface case is a *view*,
  specified in [theories.md](theories.md), not a record rule.

---

## 2. Symbols

### 2.1 What a symbol is

```
symbol ::= ' identifier
```

A **symbol** is a name used as data rather than as a reference: `'x` means *the name x*, while `x`
means *the value bound to x*. That is the use–mention distinction, and it is exactly what a label
is: `(p 'x)` asks for the component named `x`, not for a component whose index is the value of a
variable `x`.

The spelling of a symbol is its identifier, a non-empty sequence of identifier characters with no
whitespace. **A symbol is not a string value.** Strings are elements of `Scalar*` that exist at run
time ([string-literals.md](string-literals.md)). A symbol exists only at compile time (§2.4).

### 2.2 Why `'` and not `:`

The two candidates cost the same: neither `'` nor `:` is an identifier character today
(`core/read.go`, `symbolChars`). So the choice turns on what the character means to a reader:

- **`'` is quotation.** In Lisp and Scheme `'x` is `(quote x)`, *the symbol x itself*. That is the
  use–mention reading of §2.1, in the syntax family this language already belongs to.
- **`:x` is a keyword in Clojure, Elixir and Ruby**, where it is a *run-time value*: interned,
  comparable, storable. A label here never exists at run time, so that reading would be the wrong
  one.
- **`:` stays free** for anything a later design wants, and ascription is already spelled
  `(the T e)`.

**`'` is restricted to a symbol.** In Scheme `'(1 2)` quotes a list. Here `'` must be followed
directly by an identifier; `'(…)`, `'3` and `'"s"` are errors that say so. Quoting a general datum
would be staging, and staging in this language is reduction, not quotation (the-atom.md).

**`'` inside an identifier is reserved.** `x'` is not legal today and stays illegal, so a later
decision to allow mathematical primes is not blocked.

### 2.3 How it reads

`'x` and `(quote x)` read to the same thing. Internally the reader produces an application of the
injected name `quote` to the string literal of the spelling. So:
- **the term language keeps seven kinds** (state.md §1);
- the spelling is never resolved as a variable, because it is a string inside the reader's output
  rather than a name.

`quote` joins `if`, `let` and `loop` as a name the compiler injects into every target, which a
target may not declare. A `quote` whose argument is not a literal identifier is refused by `Load`.

### 2.4 A symbol does not survive staging

A symbol may appear as:
- a record label in a type, a literal, a projection or an update (§4);
- nothing else.

A symbol that reaches the residual in any other position is refused, with the same diagnostic
shape as a closure that survives. That is callbacks.md's rule — *free at the static level, refused at
the dynamic one* — applied to the smallest possible value. A later design may give a surviving
symbol a representation, such as a string or an integer tag. None is given here, because no program
has needed one.

### 2.5 Order

Symbols are totally ordered by comparing their spellings as sequences of Unicode scalar values,
lexicographically. §4.4 needs a total order; the lesson of structlit-2026-09-10 was that a *partial*
order in the emitter makes its output depend on something other than its input.

---

## 3. Tuples

### 3.1 Surface

| | |
|---|---|
| type | `(tuple T₀ … Tₙ₋₁)`, n ≥ 2 |
| introduction | `(tuple e₀ … eₙ₋₁)` |
| projection | `(t k)`, `k` an integer **literal** with `0 ≤ k < n` |
| destructuring | `(t (fn (x₀ … xₙ₋₁) body))` |
| update | `(with t (k e) …)` (§4.3) |

`n = 1` is refused because `(tuple T)` ≅ `T` and the form would mean nothing. `n = 0` is refused
because `bool`, `true` and `false` already give the language its finite types and nothing has asked
for a unit value.

### 3.2 Two eliminators, both of them the product's

A product `A × B` has two equivalent elimination principles:
- **the projections** `π₀ : A × B → A` and `π₁ : A × B → B`;
- **the universal property**, currying: `(A × B → C) ≅ (A → B → C)`.

`(t 0)` is the first. `(t (fn (a b) body))` is the second, and it is values.md's elimination,
`((values a b) (fn (x y) …))`, kept as it was. Which one an application means is decided by its
argument, and no argument can be both: a projection's argument is an integer literal and a
destructuring's is a function of arity `n`.

**A dynamic index is refused.** `(t i)` with `i` a variable would need every component to have
the same type, and a homogeneous family indexed at run time is an `array`
(tables.md §5.3: *a dynamic index forces homogeneity*). The diagnostic says to write an array.

### 3.3 Where a tuple may appear

This is products.md §3, unchanged, and the reasons are unchanged:

| position | allowed | lowered as |
|---|---|---|
| consumed within one reduction | yes | nothing: it reduces away (β, β-tab) |
| a function's result | yes | several results (values.md): Go's multiple returns, a generated Java `record`, a JS object, `rax`/`rdx` |
| an element of `(array …)` or `(buffer …)` | yes | the flat stride (products.md §4) |
| a parameter at an export | **no** | a bare product has no width caller and callee could agree on |
| a value in a `(map K …)` | **no** | a map is not an array, so the stride does not apply |
| a component that is itself a product, at a representation position | **no** | the nested layout products.md §7 defers |

### 3.4 Reduction

- **Projection** is β-tab (tables.md §4): `((tuple e₀ … eₙ₋₁) k) → eₖ` for a literal `k`. It is
  exactly the clause that folds `((array a b) 0)` today.
- **Destructuring** is `((tuple e₀ … eₙ₋₁) (fn (x₀ … xₙ₋₁) b)) → ((fn (x₀ … xₙ₋₁) b) e₀ … eₙ₋₁)`.
  Today values.md obtains it for free, because `(values …)` reads as `(fn (#k) (#k e…))` and the
  rule is β.

**Decided, by the acceptance test: the Church term** ([respell-2026-09-15](../../gauntlet/results/respell-2026-09-15.md)).
One spelling has to serve both eliminators. The reader keeps `(tuple e…)` as `(fn (#k) (#k e…))`,
the term `values` always read as, so destructuring is β, every backend's multiple return is
unchanged, and the product pass recognises a stored tuple with the same matcher the backends use.
The other choice, a table literal, would have needed destructuring as a clause beside β-tab and every
backend's several-results path taught a second shape. With the Church term every emitted file is
byte-identical. **Projection of a literal tuple by an integer, `((tuple a b) 0)`, is not reduced yet**:
no program writes it, and it is one clause when one does.

A type is built from a term, so `TypeName` inverts the reading: `(fn (#k) (#k A B))` in a type
position is the type `(tuple A B)`. `#k` cannot be written in source, so nothing else has that shape.

### 3.5 Laws

For a tuple `t = (tuple e₀ … eₙ₋₁)`:

```
(t k)                                   = eₖ
(t (fn (x₀ … xₙ₋₁) b))                  = b[x₀ := (t 0), …, xₙ₋₁ := (t n−1)]      (for pure eᵢ)
(tuple (t 0) … (t n−1))                 = t                                        η
```

The second law holds only for pure components, for effects.md's reason. An impure component is
evaluated once at the tuple's construction, and duplicating it into several projections would
evaluate it several times.

---

## 4. Records

### 4.1 Surface

| | |
|---|---|
| type | `(record ('l₁ T₁) … ('lₙ Tₙ))`, n ≥ 1, labels distinct symbols |
| introduction | `(record ('l₁ e₁) … ('lₙ eₙ))`, every label of the type given once |
| projection | `(r 'l)` |
| update | `(with r ('l e) …)` |
| a name for a record type | `(type point (record ('x f64) ('y f64)))`, an abbreviation (theories.md) |

`(p 'x)` is application because a record is a function on its labels. It is the same form as array
indexing and map lookup, and it needs no name of its own (tables.md §3).

### 4.2 Well-formedness, derived rather than listed

A record is a function on its label set. Each rule below is what that sentence means:

- **A label may not appear twice** in a type or a literal. A function has one value at each point.
- **A literal gives every label.** A record is *total* on its labels; a partial one would be a map.
- **Projection of a label not in the type is a type error.** It is application outside the domain,
  and the domain is known statically, so it is caught statically.
- **A label is a literal symbol.** A label computed at run time would be a dynamic index, which
  forces homogeneity (§3.2); that is `(map K V)`.

### 4.3 Update is TLA+'s `EXCEPT`

`(with r ('l e))` is the record equal to `r` at every label except `l`, where it is `e`. It is TLA+'s
`[r EXCEPT !.l = e]` and F#'s `{ r with l = e }`. Several updates in one `with` apply to distinct
labels, so their order is irrelevant; a repeated label is an error.

The result has **the type of `r`**. `e` must be compatible with the label's component type, and a
component declared with a range makes that an obligation, like any other value flowing into a
declared range.

`with` is not `set`. `set` stores into a **linear buffer** and consumes it (ADR 0018). `with`
builds a **new immutable value** and consumes nothing. The two are the same mathematics (a function
equal to another except at one point), and the distinction is ADR 0018's between a buffer and a
value. products.md's `(set (b i) j v)`, a store into a table of products, is unchanged.

**Laws.** Projection and update form a *very well-behaved lens* (Foster, Greenwald, Moore, Pierce &
Schmitt, *Combinators for bidirectional tree transformations*, TOPLAS 2007):

```
((with r ('l e)) 'l)          = e                          put–get
((with r ('l e)) 'm)          = (r 'm)          for m ≠ l
(with r ('l (r 'l)))          = r                          get–put
(with (with r ('l e)) ('l f)) = (with r ('l f))            put–put
```

### 4.4 Record types are structural, and their order is canonical

Two record types are **equal** when they have the same label set and equal component types at each
label. The order in which labels are written is not part of the type:
`(record ('x int) ('y f64))` and `(record ('y f64) ('x int))` are one type. A record is a function,
and a function does not have an order on its domain.

**Why structural, and not nominal as in F#.** F# makes records nominal so that type inference can
tell which record a field name belongs to. This compiler never needs to know that before reduction.
The residual is monomorphic, so every record's type is known exactly by the time anything checks it
(decidability-map.md). Structural records cost no inference here, and they are what TLA+ has.
`(type point …)` names a record type as an **abbreviation** and creates no new type.

**Representation needs an order, and the order is canonical.** A record is lowered as the tuple of
its components **in ascending label order** (§2.5). That order is a bijection from the label set to
`{0 … n−1}`, which is products.md §1's *"a record is a tuple up to a bijection on the index set"*.
Once the bijection is fixed, the tuple's positions and lowerings (§3.3) apply unchanged.

**The hazard, stated because a conformance case must catch it:** an emitter that used *written*
order would store a record literal written `('x 1) ('y 2)` and read one written `('y …) ('x …)` at
the wrong slots. That is a silent wrong answer on every target at once, which the differential
suite would not see as a disagreement. §9 names the case that catches it.

### 4.5 Where a record may appear

Exactly where a tuple may (§3.3), because it *is* one after the bijection.

---

## 5. Variants

### 5.1 Surface

```lisp
(variant result (ok int) (err int))             ; a named variant type with two constructors
(variant colour red green blue)                 ; no payloads
(variant (option T) none (some T))              ; a type parameter — §5.3

(ok 42)                                          ; construct: a constructor is a function
(case r
  (ok v)  (+ v 1)
  (err e) (- 0 e))                               ; eliminate
```

This is [sums.md](sums.md) with `sum` renamed to `variant`. Constructors are generated definitions,
`case` expands in `Load`, a missing case is an error, and the last case needs no test. Nothing about
the reduction, the representation or case-of-case changes.

### 5.2 Why the name is `variant`

`sum` is correct mathematics and reads as arithmetic to a programmer. The replacement had to name a
*tagged* choice and pair with `record`:

| word | why not |
|---|---|
| `union` | untagged in C and TypeScript, which is what §1.3 refuses |
| `enum` | integer constants in C, Java, Go and every Win32 header this repository declares |
| `data` | Haskell's recursive sum of products |
| `choice`, `oneof` | tied to F#'s `Choice` and protobuf's `oneof` |

`variant` is tagged everywhere it is used (C++'s `std::variant`, OCaml, Pascal's variant records,
COM's `VARIANT`), and **record and variant are the textbook pair** (Pierce, *Types and Programming
Languages*, §11.8 and §11.10; Cardelli & Wegner 1985).

### 5.3 Variants are nominal, and may take type parameters

A variant type is **named** by its declaration, and two declarations with the same constructors are
different types. The reason is sums.md §2's and ML's: `(ok 3)` alone does not determine its type,
and structural variants need row polymorphism to infer (Garrigue, *Programming with polymorphic
variants*, 1998), which this language does not have.

**Type parameters**, `(variant (option T) none (some T))`, make a family of variant types, one per
argument. Staging makes every use monomorphic, so no parameter survives to the checker.
**Not built:** today `option` exists as maps.md's result type, not as a user declaration.

### 5.4 Payloads

- **No payload:** `red`, a constructor of arity zero.
- **One payload:** `(ok int)`.
- **Several:** `(point int int)` means the payload is `(tuple int int)`. The constructor takes two
  arguments, and the pattern `(point x y)` destructures. **Not built:** sums.md's constructors take
  one payload today.

**No application form.** `(v 'ok)` is not a surface form. By §1.2 it would be application at a
single point whose identity is a run-time fact, so its domain condition is a branch, and the branch
is `case`.

### 5.5 Across modules and at boundaries

The questions here were spec/theories.md §11's gap. Answering them turned up a bug that was live at
HEAD (§5.5.2).

#### 5.5.1 What a variant type is

A variant **declaration** introduces a type constructor, of kind `type` or `type → … → type`. A
variant **type** is that constructor applied to its arguments, and it is identified by **the
qualified name of its declaration together with its arguments**:

```
    a/result(int, string)          b/result(int, string)          lang.option(int)
```

- **Nominal in the declaration.** `a/result` and `b/result` are different types even when their
  constructors are spelled alike. That is §5.3's reason: `(ok 3)` alone does not determine a type.
- **Applicative in the arguments.** Two occurrences of `(option int)` resolving to one declaration
  are the **same type**, because instantiation is resolution and names the instance by its diagram
  (theories-b-or-c.md §3.2, I3).
- **`option` is one declaration, in `lang`.** Today every module holds its own copy
  (`core/reduce.go`, `newModule`), and sums are compared by name so the copies do not clash. Under
  §5.5.1 there are no copies to reconcile.

The canonical spelling of an applied type uses products.md §2's delimited form, `a/result(int,
string)`, because an argument may itself contain spaces: `(int 0 255)`.

#### 5.5.2 Resolution, and the bug it closes

**Constructors are names**, resolved lexically like every other name (spec/theories.md §3.4). A
`case` clause's pattern resolves to a constructor, and all of a `case`'s clauses must resolve to
constructors of **one declaration**, compared by identity rather than by spelling.

**A module's own declaration shadows `lang`'s**, as any inner name shadows an outer one. Passing a
map read (a `lang.option`) where a module's own `option` is wanted is a type error, reported in both
spellings (spec/theories.md §10, D4): *"option (my/opt.option) is not option (lang.option)"*.

**The bug.** `Load` kept every module's sums in one table keyed by the **bare** name, and checked only
that no constructor belonged to two differently named sums. Two modules each declaring `result` passed
that check, and the later declaration **overwrote** the earlier. A signature returning `result` then
took its payload type from whichever module loaded last. Measured 2026-09-15:

| order | emitted on Go | our checker | Go |
|---|---|---|---|
| module `a` (`ok int`) before `b` (`ok string`) | `func GenPick(n int) (int, string)` | accepted | refused |
| `b` before `a` | `func GenPick(n int) (int, int)` | accepted | accepted |

JavaScript emitted the first without complaint, since it declares no types. Inside a program the
confusion is invisible, because reduction erases a payload's type; that is why no program in the
corpus ever showed it.

**Built 2026-09-15, replacing the interim rule**
([qualvariant-2026-09-15](../../gauntlet/results/qualvariant-2026-09-15.md)). A variant type is keyed
by its qualified declaration name, so the two `result`s are two types and both load in either order.
The fix is one law: resolution must be **injective on declarations that differ**, and the bare-name
key was resolution composed with forgetting the module. Measuring what else that table did found that
`case` patterns were the one kind of name that ignored scope:

| written in module `b` | before | now |
|---|---|---|
| `ok`, declared by the root module, not imported | captured: the tag resolved by δ to the root's definition | refused, naming the declaration and its module |
| `ok`, declared by `a`, imported as `a` | accepted, residual `(if (= 0 ok#tag) n 0)` with the tag free | refused: *"write the pattern through its alias"* |
| `a.ok` | refused as *"not a variant of any sum"* | resolved; a static case reduces to nothing |

A pattern now resolves by `Module.resolve`'s own rule, and the tag `c#tag` is exported exactly when
`c` is. **Not yet built from §5.5.1**: `option` is still a copy per module, identified by key rather
than removed; a module's own declaration shadowing `lang`'s; and type arguments.

#### 5.5.3 Type arguments are never inferred

- **Inside a program they are not needed.** A constructor is a Church term polymorphic in its payload,
  `some = λp.λk. k tag p`, and reduction erases every variant that does not cross a boundary. By the
  time anything checks a type, the term is monomorphic (decidability-map.md).
- **At a boundary the signature writes them.** `(sig lookup ((k int)) (option int))`. So the only
  place an argument is needed is the only place one is written, and no inference, unification or
  Hindley–Milner is involved.
- **Errors:**
  - a parameterised variant named without its arguments at a boundary, as in `(sig f … option)`, is
    refused with *"write (option T) with its argument"*;
  - the wrong number of arguments is refused, naming the declaration's arity.

#### 5.5.4 Well-formed parameters

Each rule is the consequence of one stated elsewhere:

- **A parameter has kind `type`.** A parameter that is itself applied, `(variant (wrap F) (w (F int)))`,
  is higher-kinded, and is refused: there is no demand, and it is where module systems become research
  projects (theories-b-or-c.md §6.1).
- **A declaration may not mention itself**, applied or not. `(variant (list T) nil (cons T (list T)))`
  is μ, which the well-founded order of declarations refuses (spec/theories.md §1.3, type-algebra.md
  §3.1). A parameter does not smuggle recursion back in.
- **A payload may not be a buffer.** A payload is an element position: `case` binds it, and binding is
  an observation. ADR 0020 rule 6, *a buffer may not be an element type*, is what keeps an observation
  from extracting an alias, and the same reasoning applies here.
- **Every parameter occurs in some payload.** A phantom parameter is refused until a program needs one.

#### 5.5.5 At a boundary: one slot per distinct payload type

This is the representation question, and it is answered from what a variant **is**, not from what
any target can hold.

**The encoding.** Let a variant type have constructors `c₀ … cₘ₋₁` in declaration order. After
instantiation, let `S₁ … Sₖ` be the **distinct** payload types, in canonical order (the total order on
canonical type spellings; §4.4's reason). Then a value crossing a boundary is

```
    (tuple tag S₁ … Sₖ)          tag = the constructor's index, as sums.md's tags already are
```

A value built by `cᵢ` with payload type `Sⱼ` stores its payload in slot `j`. Its other slots are
**unconstrained**.

> **Theorem R (faithfulness).** Let `enc(cᵢ v)` put `i` in the tag and `v` in slot `j(i)`, and let
> `dec` read the tag, then read slot `j(tag)` and apply `c_tag`. Then `dec ∘ enc = id`, whatever the
> unused slots hold.
>
> *Proof.* Tags are distinct, so the tag determines `i`. `i` determines `j(i)`. Slot `j(i)` holds `v`
> by construction, and `dec` reads no other slot. ∎

**Consequences:**
- **The unused slots may hold anything**, so a backend may leave each at its host's zero or default
  value. Nothing reads it, because `case` reads only the slot its tag selects.
- **`k = 1` is exactly today's representation**, sums.md's `(tag, payload)`. The existing refusal,
  *"variants carry different payload types"*, becomes the special case `k > 1` not yet built. **Every
  emitted file for an existing program is unchanged.**
- **`k = 0`**, a variant with no payloads, is the tag alone.
- **Sharing a slot is sound.** Two constructors with the same payload type use one slot, and the tag
  still tells them apart. That is why slots are per *type*, not per *constructor*: `(result int int)`
  is `(tag, int)`, not `(tag, int, int)`.
- **A niche encoding is an optimisation of this representation, never a replacement for it.** A
  target may declare one where the host already represents the extra point, as `boxed`'s `Long` does
  with `null` (spec/theories.md §5.7). Without a declaration, Theorem R's encoding is used.

**Lowering is the tuple's** (§3.3): a variant result is a tuple result, and a variant table element is
a flat stride of `1 + k` slots, under products.md §4's rule that a component with no flat
representation is refused.

**A target does not get to bound `k`.** A tuple of any width is the language's (values.md: *"if several
results go into the language, Java gets a generated record and windows gets a register or stack
convention, and finding those is the compiler's job, not the target author's"*). A backend that has
not yet built a convention for a given width reports a **compiler limitation**, named as one. It is
never a rule of this specification, and never a reason to narrow what a variant may carry.

#### 5.5.6 Conformance

| case | what it checks | fails against |
|---|---|---|
| `variant-two-payloads` | an exported function returning `(result int string)` along both constructors, called by a host driver on every target that can print both | slot assignment by constructor rather than by type, or slots in written order rather than canonical order |
| `variant-shared-slot` | `(result int int)` returned along both constructors | a shared slot overwritten, or the tag ignored |
| `variant-two-modules` | two modules each declaring a different `result`, used at a boundary | today's global table; after the loader, qualified names |

---

## 6. Arrays and maps

Unchanged ([tables.md](tables.md), [maps.md](maps.md)), except that **the array literal becomes
homogeneous only**. `(array 1 2 3)` is an array. `(array 1 "two")` is refused with *"a heterogeneous
literal is a tuple; write (tuple 1 "two")"*, and the type `(array A B)` is refused the same way.

The collision this removes is real. `(array int)` has meant both *a table of ints* and *a product
with one component*, and products.md's `(array int string)` in a result position was read as three
results. Arity was asked to carry a meaning that a name should carry.

---

## 7. Across targets

**No backend learns anything.** After reduction:
- a tuple is what `values` or a flattened product already is;
- a record is a tuple under §4.4's bijection, done before the type checker, exactly where
  products.md's flattening pass runs;
- a variant is what `sum` already generates;
- a symbol has been consumed or refused (§2.4).

The emitted code for a program written in the old spelling and the same program respelled must be
**byte-identical**. That is the acceptance test for the whole document.

---

## 8. Laws the implementation must keep

Each is checkable, and §9 names a case for each one that could break:

1. **Order independence.** A record value is the same whatever order its literal was written in, at
   every position where it can be observed: consumed statically, returned, stored in a table.
2. **The lens laws** of §4.3.
3. **Tuple η and destructuring** of §3.5, for pure components.
4. **Respelling preserves emission** (§7).
5. **A symbol never reaches a backend** (§2.4).

---

## 9. Conformance

Every case is a differential case on all four targets, and each is written so that it **fails**
against the bug it exists for:

| case | what it prints | the bug it fails against |
|---|---|---|
| `record-order` | a table of records filled with literals written in one label order and read through literals and projections written in the other | an emitter using written order: wrong slots on every target |
| `record-lens` | each of the four lens laws evaluated on a record whose fields have *different* values | an update that writes the wrong label, or all of them |
| `tuple-destructure` | a function returning a tuple, consumed once by projection and once by destructuring, both printed | the two eliminators disagreeing about position |
| `variant-payloads` | a constructor with two payloads, matched and printed in both orders | payload order swapped |
| `respell` | not a program: every existing example respelled, emitted, and compared byte for byte | any change in emission |

---

## 10. Migration

> **Built 2026-09-15** ([respell-2026-09-15](../../gauntlet/results/respell-2026-09-15.md)): `variant`,
> `tuple` as term and type, and `(tuple A B)` for several results in a program's `sig` and a target's.
> The old spellings are refused with the message below. Records, symbols, `with` and the heterogeneous
> `(array a b)` *literal* refusal (which needs types, so it belongs to the checker) are not built.

| old | new | refused with |
|---|---|---|
| `(sum S …)` | `(variant S …)` | *"`sum` is spelled `variant` (spec/data.md §5.2)"* |
| `(values a b)` | `(tuple a b)` | *"`values` is spelled `tuple` (spec/data.md §3)"* |
| type `(array A B)` | `(tuple A B)` | §6 |
| literal `(array a b)` with components of different types | `(tuple a b)` | §6 |
| result list `(A B)` in a `sig` or host declaration | `(tuple A B)` | *"a result list is a tuple type"* |

The old spellings are removed rather than kept as aliases. The rewrite is mechanical, and two
spellings for one construct is the shape `merge` had before target-system.md separated its two
operations.

---

## 11. Open

- **§3.4's representation of `(tuple …)` in the reader**: a Church term or a table literal. Decided by
  the byte-identical test.
- **Nested products at representation positions** (products.md §7): unchanged, and still waiting for
  a program that wants one.
- **A symbol that survives staging**: no representation until a program needs one.
- **A `with` that changes several levels at once** (`(with r ('a ('b e)))`): not specified. Nested
  updates are written as nested `with`s.
