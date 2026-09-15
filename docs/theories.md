# Declarations are theories

Research, **no decision, nothing built**. 2026-09-14, on hamza's *"let's research the literature, do
the math, derive the algebra — they are connected: modules, targets, types, alias, const, maybe
interfaces, algebraic types, and the rules as data we said we should borrow from sequent calculus in
Shen. Sketch candidates, and build a solid foundation, even if we don't implement everything now;
whatever we do should fit what we add in the future."*

It follows [declaration-surface.md](declaration-surface.md), whose §4 found that a module wants to
own its types. This document asks what a module, a target, a type, an alias, a constant, an
interface, a sum and a rule have in common. Its answer is that they are **one kind of object and
one kind of arrow**, and that the operations this repository already performs on them are the
standard operations on those objects.

The one-line answer:

> **A module is a THEORY: a list of declarations `x : A [= t]`, at three levels (types, terms,
> propositions). A target is a MODEL of theories: it realizes what they leave abstract. `use`,
> `provides` and `implements` are MORPHISMS. Gluing is amalgamation, overriding is asymmetric
> concatenation, `export` is a reduct, and an alias is a definitional equation. A sum and an
> interface are the two POLARITIES of one declaration form. Rules are data exactly when each rule
> belongs to a fragment whose decision procedure the compiler owns.**

---

## 0. What exists today, read as a list of unrelated forms

| form | where | what it says |
|---|---|---|
| `(module P …)` | programs, targets | a namespace; a target's may contain only `prim` (`emit/target.go:1488`) |
| `(use P as A)` | programs | bind a module under a name |
| `(export n …)` | programs | visibility |
| `(def n t)` | programs | a name with a body |
| `(sig n ((p T)…) R (where P) (ensures Q))` | programs | a claim about a name |
| `(sum S (c T) …)` | programs | a closed, finite, named coproduct |
| `(prim n (T…) R kind "tmpl" pure (where …) (ensures …) (import …))` | targets | a host operation |
| `(type T "spelling")` | targets | a host type, in one flat pool per target |
| `(implements T I …)` | targets | `T` may be used where `I` is wanted |
| `(provides T M decl…)` | libraries | a declaration fragment for target `T` |
| `(int-repr LO HI "spelling")`, `(big-repr …)`, `(max-len N)`, `(shift-width N)` | targets | representation facts |
| the injected set: `if let loop the = + …` | compiler | the language; a target may not declare them |
| `seedDivAxioms`, `x ≤ x·x`, the Farkas step | **Go code** | axioms the refinement layer assumes |

That is fourteen forms. They were added one at a time for good reasons, and nothing yet says how
they relate. The last row is the one [decidability-map.md §5](decidability-map.md) already called a
patch: rules written in Go, which this project's own rule sends to data.

---

## 1. The literature, and what each piece is for

This design is not new mathematics. Five bodies of work have already solved the parts:

**Institutions and algebraic specification.** Goguen & Burstall, *Institutions* (JACM 1992);
Burstall & Goguen, *Putting theories together to make specifications* (IJCAI 1977); Sannella &
Tarlecki, *Foundations of Algebraic Specification* (2012). A **signature** is sorts plus operation
symbols. A **theory** is a signature plus sentences. A **model** interprets them. Theories are put
together by **colimits**, restricted by **reducts**, and renamed along **signature morphisms**. The
**satisfaction condition** says truth is invariant under a change of notation. The CASL reference
manual (Mosses, ed., 2004) gives `free type`, the initial semantics of a datatype declaration.

**Logical frameworks.** Harper, Honsell & Plotkin, *A framework for defining logics* (JACM 1993): a
signature is a list of **constants**, and inference rules *are* constants whose types are
judgments. Rabe & Kohlhase, *A scalable module system* (Information and Computation 2013), MMT:
everything is a **theory** of declarations `c : A [= t]`, related by **includes**, **structures**
(named nested imports) and **views** (morphisms that must type-check). This is the closest existing
design to what this repository has drifted into.

**ML modules.** MacQueen (1984); Leroy, *Manifest types, modules, and separate compilation* (POPL
1994); Harper & Lillibridge, *translucent sums* (POPL 1994); Rossberg, Russo & Dreyer, *F-ing
modules* (JFP 2014). A structure holds **abstract** types (`type t`) and **manifest** types
(`type t = int`). Modules can be **elaborated away** into a core language, which is ADR 0011's claim
about ours.

**Records and concatenation.** Harper & Pierce, *A record calculus based on symmetric
concatenation* (POPL 1991); Wand (1989). **Symmetric** concatenation requires disjoint labels.
**Asymmetric** concatenation is right-biased override. These are target-system.md §2's `⊔` and `▷`,
under the names they already had.

**Polarity and data abstraction.** Andreoli, focusing (1992); Zeilberger, *On the unity of duality*
(APAL 2008); Abel, Pientka, Thibodeau & Setzer, *Copatterns* (POPL 2013); Cook, *On understanding
data abstraction, revisited* (OOPSLA 2009); Mitchell & Plotkin (TOPLAS 1988). A type is defined
either by its **constructors** (positive; data) or by its **destructors** (negative; codata). An
object is codata.

**Decidable rule sets.** McAllester, *Automatic recognition of tractability in inference relations*
(JACM 1993); Givan & McAllester, *local inference relations* (KR 1992); Sofronie-Stokkermans,
*Hierarchic reasoning in local theory extensions* (CADE 2005); Nelson, Detlefs & Saxe, *Simplify*
(JACM 2005), triggers; Datalog (Ceri, Gottlob & Tanca 1989). User-written rules stay decidable, and
fast, when they are **local**: every inference needs only terms that already occur. Wadler & Blott
(POPL 1989), type classes, is the cautionary case for **evidence** and **coherence**.

And **Shen** (Tarver, *The Book of Shen*): a `datatype` is a set of sequent rules, and a
double-lined rule gives both introduction and elimination. What this repository took from Shen was
*rules as data*. What it refused was *discharged by search*
([types-direction.md §3.5](types-direction.md), [decidability-map.md §5](decidability-map.md)).
§6 is how to keep the first without the second.

---

## 2. The objects

### 2.1 A declaration

Every one of §0's forms elaborates to one or more **declarations**:

```
    d  =  x : A  [= t]  [↦ ρ]
```

- `x` is a name, fully qualified by its module path.
- `A` is its **classifier**, at one of three levels:

  | level | `x` is | `A` is | example |
  |---|---|---|---|
  | **type** | a type (or type constructor) | a kind: `type`, `type → type` | `File : type` |
  | **term** | an operation or value | a type | `Read : (File, buffer) → (int, error)` |
  | **prop** | a named fact | a proposition | `div-hi : x ≤ k·(x/k) + k − 1` |

- `t` is an optional **definiens**, an equation `x ≡ t` that δ may use.
- `ρ` is an optional **realization**: how some backend spells `x` (a template, a host type name).

Nothing else is needed. The rest of this document shows that each of §0's forms, and each future one
named in §9, is a choice of level and of which of `t` and `ρ` are present.

### 2.2 The four cells, at every level

Whether a declaration has a definiens and whether it has a realization gives **four cells**, and
they are [modules.md §5](spec/modules.md)'s four cells, now at every level instead of only for terms:

| | no `ρ` | `ρ` |
|---|---|---|
| **no `t`** | *abstract*: an interface, not yet covered | *host*: reduction halts, the target spells it |
| **`t`** | *defined*: δ unfolds it | *conditional*: native wins, `t` is the conformance spec |

At each level:

| level | abstract | host | defined | conditional |
|---|---|---|---|---|
| **type** | a sort of a portable interface | `(type File "*os.File")` | an **alias**: `int32 ≡ (int −2³¹ 2³¹−1)` | a named range with a fixed host spelling |
| **term** | a `sig` with no body anywhere | `(prim …)` | `(def …)` | a **constant**: `MaxRune ≡ 1114111 ↦ "utf8.MaxRune"`, or `split-words` with a native |
| **prop** | an obligation (`where` on a prim) | a **runtime check** (`-checked`, `big-fit`) | an axiom *derived* from definitions | an assumed fact with a runtime check |

Four entries in that table resolve questions that were open:

- **An alias is a type with a definiens and no realization.** It is δ one level up, which
  declaration-surface.md §3.1 said informally. Leroy calls it a *manifest type*. It is **equal** to
  its definiens and adds no type.
- **A host type is a type with a realization and no definiens.** It is an *abstract* type in ML's
  sense, and the host holds the representation.
- **A constant is the conditional cell for a term.** Its definiens, the value, is what the analysis
  reads, and what an endpoint like `(int 0 utf8.MaxRune)` needs when the declaration is loaded. Its
  realization, the host's name, is what the emitter writes. Native wins, as R2 says. The obligation
  that the two agree, `utf8.MaxRune == 1114111`, is **checkable by the host**, like an `implements`
  edge. That is a derivation of declaration-surface.md §2 option A (sugar in the target file) over
  option B (a bare name at the use site): option A is the only one that puts a constant in the cell
  where its value is usable.
- **The prop row is ADR 0019's three escapes, found again.** An unproven integer fact is *obligated*
  (refused if not discharged), *realized* (trap at run time, `-checked`), or *assumed* at a boundary
  (an exported `where`). Those escapes were designed separately; here they are three cells of one
  table.

**The measured instance.** Go's API manifest has **1,270** exported type names across the standard
library, and exactly **three** collide when a type is named by its package's last segment, as the
flat pool does:

| name | packages | what they are |
|---|---|---|
| `template.Template` | `html/template`, `text/template` | two distinct structs |
| `scanner.Scanner` | `go/scanner`, `text/scanner` | two distinct structs |
| `template.FuncMap` | `html/template`, `text/template` | **one type**: `html/template` declares `type FuncMap = template.FuncMap` |

The first two are abstract types that the flat pool wrongly identifies. The third is an alias, a
defined-cell type, that the pool identifies correctly and **by accident**. A theory states the
difference instead of relying on the accident: `html/template.FuncMap ≡ text/template.FuncMap` is a
declaration with a definiens, and `html/template.Template` has a realization and no definiens.

### 2.3 A module is a theory

```
    Th  =  (path, [d₁, …, dₙ])        well-formed iff each dᵢ's classifier and definiens
                                       mention only names declared earlier, or in an
                                       included theory
```

The "only earlier names" condition is a **well-founded order on declarations**. It is what
[type-algebra.md §3.1](type-algebra.md) refuses when it refuses μ: a type whose definiens mentions
itself breaks the order. So *no fixed points* is not a separate rule about types. It is the
well-formedness of a theory, and the same condition gives ADR 0014 at the term level: a `def` whose
definiens mentions itself is recursion. **One order, two exclusions.**

A theory has **three namespaces**, types, terms and props, plus its **child theories** — the
nested modules of declaration-surface.md §1.6. OCaml keeps types, values and modules in separate
namespaces for the same reason.

### 2.4 A target is a model

```
    T  =  (B, ρ_T)        B ∈ 𝔅,    ρ_T : ⋃ abstract declarations in scope  ⇀  realizations
```

A realization is backend-specific: a template means something on `x86-64` and nothing on `go`.
[target-system.md §3](spec/target-system.md)'s split `Decl ≅ Σ × I` becomes structural:

- `Σ` is the declaration: its classifier and its contracts, which are prop-level declarations about
  it. It lives in a **theory**.
- `I` is `ρ`: template, imports, host spelling. It lives in a **target**.

For a host-named theory like `go/os`, the two are written together in one form because there is
exactly one model, so `(prim …)` is sugar for a declaration plus its realization. For a portable
theory like `os` or `io`, they live apart, and `(provides T M …)` is the realization. Target
families (target-system.md §5) are then **several models of one theory**, and T4, *a family shares
covering*, holds because covering depends only on which declarations have a realization.

**The language is the root theory.** `if`, `let`, `loop`, `the`, `=` and `+` are declarations of a
theory `lang` that every module includes. Their realizations are compiler code for the structural
names, and target spellings found by `findEq` for the arithmetic. ADR 0017's rule, that a target may
not declare `if`, becomes a statement about this theory: **`lang` is closed**. No theory may add
declarations to it, and no target may give a structural name a realization. The rule stays a
decision; the framework gives it a location instead of a list.

### 2.5 The arrows

A **morphism** `σ : Th₁ → Th₂` maps each declaration of `Th₁` to one of `Th₂`, preserving levels and
classifiers. Every arrow in this system is one:

| form | morphism | what must hold |
|---|---|---|
| `(use P as A)` | **inclusion with renaming** `x ↦ A.x` | nothing; always well-formed |
| `(export …)` | a **reduct**: restriction to a sub-theory | nothing; the rest is hidden |
| `(provides T M …)` | a **model** of `M` on `T` | conformance: each realization denotes what `M` says |
| `(implements T I)` | a **view** `I → T`: `I`'s sort to `T`, each method to `T`'s | method-set inclusion; §4.3 |
| a `sig` on a `def` | a view from a one-declaration theory into the module | the two-direction check `sig` already performs |
| a family's shared `Σ` | a **view** from the interface theory into each member | target-system.md §5.4's obligation |

---

## 3. The operations and their laws

Four operations compose theories. Three are already implemented, under other names.

**Amalgamation, `⊔`.** Two fragments glue when they agree on their overlap. This is a pushout over
the shared names (Burstall & Goguen 1977), and it is target-system.md §2.1:

```
    ⊔  is partial, commutative, associative and idempotent;  defined iff the overlap agrees
```

**Asymmetric concatenation, `▷`.** The left side wins at a collision (Harper & Pierce 1991). This is
target-system.md §2.2 and R2:

```
    ▷  is total and associative;  idempotent;  NOT commutative
```

**Reduct, `|S`.** Restriction to a set of names, which is `export`.

```
    (φ ⊔ ψ)|S = φ|S ⊔ ψ|S   whenever the left side is defined      (reduct preserves amalgamation)
    (φ ▷ ψ)|S = φ|S ▷ ψ|S                                          (and override)
```

*Proof.* Both operations act name by name, and restriction selects names. ∎ The converse of the
first law fails: two fragments may disagree only on a hidden name. So **hiding a name cannot make a
gluing legal**, and the loader is right to glue before restricting.

**Renaming along a morphism, `σ*`.** This is `use … as`.

> **T1 — renaming preserves meaning (the satisfaction condition).** For an injective renaming `σ`,
> a program is well-formed and has meaning `m` over `Th` iff its renaming is well-formed and has
> meaning `m` over `σ*(Th)`.

This is Goguen & Burstall's satisfaction condition, restricted to the syntactic case, and it is the
formal content of "an alias cannot change a program". **The hypothesis that bites is injectivity.**
A non-injective renaming identifies distinct declarations. That is precisely what naming a host type
by its last path segment does to `template.Template` and `scanner.Scanner` (§2.2): it is a
non-injective morphism out of Go's own namespace, and the pushout it builds identifies two sorts Go
keeps distinct. Naming by the full path makes the morphism injective, and T1 then applies.

---

## 4. Types

### 4.1 The type level, stated once

Every type in this repository is one of five things:

| type declaration | cell | literature | today |
|---|---|---|---|
| **abstract** | host | ML abstract type; an opaque host token | `(type File "*os.File")` |
| **manifest** | defined | Leroy's manifest type | none (declaration-surface.md §3 option B) |
| **refinement** | defined, to a subset | Dependent ML; Freeman & Pfenning | `(int lo hi)`, desugared to a `where` |
| **variant** (positive) | defined, *freely generated* | CASL `free type`; initial algebra | `(sum …)` |
| **record** (negative) | defined by *destructors* | copatterns; Cook's objects | products `(array A B)`, `values`; a host interface |

Plus the built-in formers of [type-algebra.md](type-algebra.md): `×`, `+`, `→`, `0`, `1`, and
tables. The first three rows are covered above. The last two are where the connection to sequent
calculus pays for itself.

### 4.2 One declaration form, two polarities

A type in a sequent calculus is given by rules. **Right rules** say how to make a value (introduce
it). **Left rules** say how to use one (eliminate it). Focusing (Andreoli; Zeilberger) shows that
for each type only one side needs to be *given*. The other side is determined by an **inversion
principle**:

- a **positive** type is given by its right rules, its **constructors**. Its left rule, `case`,
  is derived: to use a value, handle every constructor.
- a **negative** type is given by its left rules, its **destructors**. Its right rule is derived:
  to make a value, say what every destructor returns.

Shen's `datatype` with a double line gives both sides by hand. Focusing says that giving both is
redundant, and that giving them independently lets the user write an inconsistent pair, which is
one reason Shen needs search to use them.

So one idea, parameterised by polarity, covers algebraic types and interfaces alike.

> **Respelled 2026-09-15 — [spec/data.md](spec/data.md).** This section first used the literature's
> keywords, `data` and `codata`. They were dropped: `codata` means *infinite* (Hagino, Charity,
> Idris), and this language has no recursive types. The positive form is **`variant`** and the
> negative form is **`record`**, the textbook pair. A host interface turned out not to be a record
> at all: it is an abstract type with a companion module, and `include` derives its subtyping
> ([spec/theories.md](spec/theories.md) §6). The polarity argument below is unchanged.

```lisp
(variant result (ok int) (err int))             ; positive: constructors given
(type point (record ('x f64) ('y f64)))         ; negative: projections given
(type Reader (host "io.Reader"))                ; negative, realized by the host:
(module Reader                                  ;   its companion's declarations are
  (sig Read ((self Reader) (p (buffer (int 0 255)))) (tuple int error)))   ; the destructors
```

`sum`, spelled `variant`, is already built. It generates definitions for the constructors and a `case` checked
for exhaustiveness, the derived left rule (sums.md §3).

### 4.3 What each polarity costs, and the law of the language again

[type-algebra.md §2.2](type-algebra.md)'s law, *every operation is free at the static level and
priced at the dynamic level*, splits exactly by polarity:

| | derived rule | static level | dynamic level |
|---|---|---|---|
| **data** | elimination (`case`) | free: case-of-case | a tag, carried as a product (sums.md) |
| **negative, first-order destructors** (a `record`) | introduction (tupling) | free | a flat product; values.md, products.md |
| **negative, a function-valued destructor** (an object) | introduction (a closure per method) | free | **a closure: refused**, callbacks.md tier 3 |
| **negative, realized by the host** (an interface: a type and its companion) | introduction is the host's | — | **free**: holding, passing and calling a host object |

That table is interfaces.md §2's three prices, *packing* (refused), *holding* (free) and *passing*
(`⟦coerce⟧ = id`), now derived from polarity rather than listed. **An interface is a negative type.
Packing one means building codata at the dynamic level, which needs a closure. Holding one means
using its left rules, which a host object supports for free.** The same fact explains why the
negative product was the free one (types-direction.md §6.3): its destructors are projections, not
functions.

### 4.4 Subtyping is a view, and a view carries no evidence

`(implements T I)`, with `I` a negative type (an interface), is a **view** `I → T`. It maps `I`'s sort to `T`, and each of
`I`'s destructors to the like-named method in `T`'s companion theory
(declaration-surface.md §1.6). The view is well-formed when each of `I`'s methods has a counterpart
in `T` whose type is at least as specific: contravariant in arguments, covariant in results. That is
Cardelli's record subtyping, and it is **checkable from declarations**. Today the Go generator
checks it by asking `go build`, and a hand-written edge is not checked at all
(hex-2026-09-14 §6).

**T2 — views without evidence are coherent.** If a view's realization is the identity (the host
coerces) or selection by a monomorphic type (the emitter chooses), then any set of views, declared
anywhere, determines the same program.

*Proof.* A view contributes a pair to a relation, and the program's meaning depends on the relation
only through membership. Membership is unchanged by repeats (`R ∪ R = R`) and by where a pair was
declared. ∎

This is why this system does not have Haskell's **orphan instance** problem. An instance carries
evidence, a dictionary, so two instances for one class and type are two different programs, and
coherence needs global uniqueness (Wadler & Blott 1989). A view here carries nothing. Where a view
is declared is therefore a question of **ownership and checking**, not of meaning:
declaration-surface.md §4.4's *the subject's module* is a convention, not a soundness requirement.

**And it gives the refusal for type classes a reason.** A concept name that carried evidence, a
user type made an instance of `len` with a dictionary passed at run time, would make views
evidence-carrying and lose T2. [overloading.md §3(b)](overloading.md)'s concept names are safe
because their instance set is **closed** and resolved at emission. In these terms, a concept name is
a theory of destructors on one sort, the views into it may only be declared by `lang`, and each view's
realization is static selection. **An interface and a concept name are the same object with
different rules about who may declare a view**: the host may declare views into an interface, and
only the language may declare them into a concept.

### 4.5 A refinement is a pair of rules, and the swap is polarity

A range `{x : int | lo ≤ x ≤ hi}` has two rules:
- **introduction**: to produce a value of the range, prove the predicate. This is an *obligation*.
- **elimination**: from a value of the range, obtain the predicate. This is an *assumption*.

The **swap** of [postconditions.md](spec/postconditions.md) (`where` is an obligation at a call and an
assumption in the body; `ensures` the reverse) is these two rules applied at the two ends of an
arrow. An argument is introduced by the caller and eliminated by the callee; a result the other way
round. It is the sequent calculus's left/right symmetry on a single refinement type, which is why
the implementation was two cases rather than six.

---

## 5. Modules, nesting and names

### 5.1 Paths

A path is a word in `Seg*`. The set of paths in scope is prefix-closed, so it is a tree, and a theory
has members in three namespaces plus children. Resolution inside a theory is **lexical**. A name is
looked up in the current theory, then its parent, and so on up to `lang`. A companion theory
therefore sees its sort without qualification:

```lisp
(module go/encoding/hex
  (use go/io)
  (type InvalidByteError "hex.InvalidByteError")
  (prim NewEncoder ((w io.Writer)) io.Writer expr "hex.NewEncoder(%s)" (import "encoding/hex"))
  (module InvalidByteError                     ; the companion of the sort above
    (prim Error ((self InvalidByteError)) string expr "%s.Error()" pure)))
```

The companion rule of declaration-surface.md §1.6 is the one condition. A child theory whose name
equals a sort of its parent is that sort's companion. A child theory and a sort may share a name
only in that relation, and any other clash is refused (JLS §7.1 already refuses one on the JVM).

### 5.2 Elaboration, so ADR 0011 holds

Nesting, lexical resolution and companions all happen during **resolution**, before reduction. The
output is the flat qualified namespace the reducer sees today, with names like
`go/encoding/hex.InvalidByteError.Error`. This is the *F-ing modules* strategy (Rossberg, Russo &
Dreyer): the module language is elaborated into the core, and nothing downstream learns it existed.
**The reducer gains nothing, and neither does any backend**, which is modules.md's founding claim
kept.

### 5.3 What is still absent

**Functors**, meaning theories parameterised by theories, stay absent for modules.md §9's reason: the
target is already the parameter to reduction. **Type constructors with parameters are present**:
`(variant (option T) …)` is a type-level declaration of kind `type → type`. That is a parameter at the
*type* level, which staging makes monomorphic. It is not a parameterised theory.

---

## 6. Rules as data

### 6.1 What a rule is here

A rule is a **prop-level declaration**, a sequent

```
    x₁ : A₁, …, xₙ : Aₙ ;  P₁, …, Pₖ   ⊢   Q
```

with typed schematic variables `xᵢ`, premises `Pⱼ` and a conclusion `Q`. Every rule-like thing in
§0 has this shape:

| existing thing | as a sequent |
|---|---|
| a prim's type | `a : A ⊢ f a : B` |
| `where P` on a prim | `a : A ; ⊢ P(a)` is required before `f a` (a premise the caller discharges) |
| `ensures Q` on a prim | `a : A ; P(a) ⊢ Q(a, f a)` |
| `(implements T I)` | `e : T ⊢ e : I` |
| a range | the pair of §4.5 |
| `seedDivAxioms` (Go today) | `x : int, k : lit ; k > 0 ⊢ k·(x/k) ≤ x ∧ x ≤ k·(x/k) + k − 1` |
| `x ≤ x·x` (Go today) | `x : int ⊢ x ≤ x·x` |
| purity | the argument admits **contraction** and **weakening** (ADR 0010) |
| a usage aspect (host-buffers.md) | a structural rule on one parameter: linear, borrowed, or shared |

That the table closes is the point. **Every fact the compiler uses about a name is a sequent, and
most are already written as data.** The Go-code rows are the exceptions, and the framework says
where they go: prop-level declarations in `lang`, or in the theory whose operator they mention.

### 6.2 Why Shen needs search, and what avoids it

Shen gives the user the whole sequent calculus as a rule language, so using the rules is proof
search. The search is undecidable in general, and Shen bounds it with a depth limit. That makes
*"your program is ill-typed"* and *"the search gave up"* indistinguishable
([types-direction.md §3.5](types-direction.md)).

The literature's answer is **not** to forbid user rules. It is to admit them only in **fragments
with decision procedures the compiler owns**, and to check which fragment a rule belongs to when it
is declared, so the author gets an error rather than the user getting a timeout.
[types-direction.md](types-direction.md)'s *"extend the predicates, not the rules"* becomes more
precise: **extend the rules, never the fragments.**

### 6.3 The fragments

| fragment | rules in it | decision procedure | complexity | status |
|---|---|---|---|---|
| **R1 typing** | the type of every term constant | syntax-directed checking of the monomorphic residual | linear | built (types.md) |
| **R2 linear arithmetic** | `where`, `ensures`, ranges, conclusions over `+ − ·lit ≤ =` | the fixed incomplete QF-LIA procedure (`emit/linear.go`) | polynomial in practice; total | built |
| **R3 Horn over names** | `implements`, companions, the closure of subtyping | Datalog: least fixed point computed at load | polynomial | built as ad-hoc closure (`Implements`) |
| **R4 local schemata** | axioms with nonlinear or uninterpreted atoms: `x ≤ x·x`, division, a host function's law | instantiate only on **ground terms that occur** (triggers), then R2 | polynomial for a fixed rule set (McAllester 1993) | **in Go today; the fragment to build** |
| **R5 structural** | purity, linearity, usage aspects | occurrence counting on the residual | linear | built (effects.md, `CheckLinear`) |
| ~~**R6 arbitrary sequents**~~ | ~~Shen `datatype` rules~~ | ~~proof search~~ | ~~undecidable~~ | **refused** |

**R4 is the new idea, and it is small.** A rule schema is admitted only if every schematic variable
occurs in a **trigger**, a term pattern in its conclusion or premises, such as `(/ x k)`. At a query,
a schema is instantiated once per matching ground term *already present* in the facts or the goal,
and the instances join the linear facts. Nothing is instantiated from a term the rules themselves
create, so the set of instances is bounded by the size of the query. The procedure terminates, and
it gives the same answer every time: an instance is either present or absent, with no search order
to be unstable. This is Simplify's trigger discipline, and Sofronie-Stokkermans's local theory
extensions give the condition under which it is also **complete** relative to R2. Where that
condition fails, the fragment is still sound and terminating, and is incomplete **in a way the
author can see**: a missing instance means a missing trigger term, which a diagnostic can name.

**The test R4 already has:** `seedDivAxioms` (`emit/refine.go:99`) and the square bound are R4
rules hard-coded in Go. Moving them into a declaration, with the refinement layer's behaviour
byte-identical across the corpus, is the acceptance test for the fragment. That is the same
standard as moving `if` into the injected set, or the link line into `windows.oro`.

### 6.4 The hazard, stated

A false rule proves anything. postconditions.md's Lemma 1 already records that one false fact makes
a conjunctive fragment derive everything. A rule is a **claim**, exactly as a `prim` is, so it gets a
prim's discipline:
- it lives in a named theory, and is active only where its trigger terms occur, which requires its
  names to be in scope;
- a rule about a host operation is checked against the host where possible, by running it, as
  `hexReference` does for declarations;
- a rule in `lang` needs the containment harness to fail when the rule is wrong, which is what
  `TestDivisionAndRemainderContain` is for the division rules today.

---

## 7. Candidates

**A. Keep the forms; add types to modules and nesting.** declaration-surface.md §4's option B with
§1.6's companions, and nothing else.
- Buys: the smallest change that fixes the measured problems (hand prefixes, the two conflated
  sorts, re-declared sorts).
- Costs: fourteen forms stay unrelated, so the next form (`const`, `record`, `interface`, `lemma`)
  is again argued from scratch, and the `implements` name collision below stays unnoticed.

**B. Theories: one declaration record, forms as sugar over it.** Everything in §2–§6. Every existing
form elaborates to declarations `x : A [= t] [↦ ρ]` in a theory, and the loader works on
declarations.
- Buys:
  - each future form becomes a choice of level, cell and polarity, instead of a design;
  - the laws of §3 and theorems T1 and T2 cover all forms at once;
  - Σ and I separate structurally, which target-system.md §5.4 wants for families;
  - R4 has a home.
- Costs:
  - a rewrite of the target loader (`emit/target.go`'s form switch) into elaborate-then-load;
  - resolution of type names, the same cost as declaration-surface.md §4.3;
  - a spec, before any of it.

**C. Full structured specification (CASL, MMT) with first-class views and parameterised theories.**
- Buys: everything in B, plus theories parameterised by theories, and views as values the user can
  name and compose.
- Costs: functors, which modules.md §9 refuses for a reason that still holds, and a surface no
  program has asked for.

**D. Shen: user-written sequent rules discharged by search.**
- Refused, for decidability-map.md §5's reason: the undecidability ends up in the type checker
  instead of the evaluator. §6.2 keeps what was worth taking.

**E. Type classes for interfaces and concept names**, with dictionaries.
- Refused. Evidence at run time is a closure, which callbacks.md tier 3 refuses. Evidence also
  needs global coherence, which T2 gets for free only because views carry none.

### Leaning

> **Revisited the same day — [theories-b-or-c.md](theories-b-or-c.md).** C's rejection above repeats
> ADR 0011's reason, and that reason is wrong: a build already *is* an instantiation, with the target
> as the argument. C decomposes into six features, and the first two (instantiation along views, and
> views as named objects) are a conservative extension of B, so the leaning there is **specify C
> restricted to those two, build B's part first**. The same document redesigns the declaration
> syntax.

**B as the foundation, built as A.** The model is B. What gets built next is A's piece of it: types
as members of theories, nesting, and companions, implemented **as declarations** so the next forms
slot in without a second loader. B's other pieces then arrive one at a time, each with its own
acceptance test, in the order §8 gives.

---

## 8. What B commits to, and the build order it implies

Each step is independent and has an acceptance test that could fail. The order is by demand already
measured.

1. **Types in theories, nesting, companions** (A). *Test*: `encoding-hex.oro` and
   `unicode-utf8.oro` rewritten with no hand prefixes; the checker still agrees with the host; a
   theory naming `text/template.Template` and `html/template.Template` keeps them distinct; every
   emitted file byte-identical.
2. **`const`**, the conditional cell for terms. *Test*: the 509 generated constants and utf8's four
   written as `const`; `(int 0 utf8.MaxRune)` accepted as an endpoint; a host check that fails when a
   value is written wrong.
3. **Manifest types (aliases), host-qualified.** *Test*: `go.int32` defined once and used by utf8;
   diagnostics print the definiens.
4. **R4, rules as data.** *Test*: `seedDivAxioms` and the square bound moved into declarations with
   byte-identical output on every program, and the containment harness failing when a moved rule is
   weakened.
5. **`implements` checked as a view.** *Test*: a hand-written false edge refused from declarations
   alone, without asking the host.
6. **`record`**, and companion `include` for interfaces (spec/data.md, spec/theories.md). Records
   are specified now; build them when a program asks for one.

**One naming collision to fix whichever candidate is chosen.** target-system.md §5.4 sketches
`(implements win32 x86-64 …)` for a *realization*, and target-files.md §2a uses `(implements T I)`
for *subtyping*. By §2.5 these are different arrows: a model and a view. The realization should be
spelled `provides`, which already means a model, and `implements` should keep meaning a view.

---

## 9. Does it fit what is coming

The future additions this repository has named, and where each lands:

| future addition | lands as | new machinery |
|---|---|---|
| string operations, `len` on a string (overloading.md) | a view from `string` into the concept `Finite`, declared by `lang` | none |
| maps with more key types | a type constructor `type → type → type` whose instances must satisfy `Eq` (a concept) | the `Eq` concept |
| Win32 SAL: nullability, `_Success_` | `(option ptr)` with a niche realization; `ensures` in R2 | the niche, already researched |
| usage aspects on host buffers (host-buffers.md) | R5 structural rules per parameter | none |
| a second Windows ISA (target families) | a second model of `win32` | `provides` per backend |
| portable records | `record` with structural labels, lowered as the tuple of its canonical order (spec/data.md §4) | the form only |
| portable interfaces | an abstract type with a companion theory; packing still refused | the form only |
| declared lemmas about a library | R4 rules in that library's theory | the fragment |
| recursion (if ADR 0014 is ever superseded) | a declaration class that is allowed to break §2.3's well-founded order | **a real extension**: a new declaration class, not a relaxed rule |
| μ, recursive types | the same order, at the type level | the same extension |

The last two rows are the honest limit. Everything else is a choice within the framework. Recursion
and fixed points are the one place the framework itself would change, and the change is **additive**:
a new class of declaration that states its own termination argument. It is not a weakening of the
order everything else relies on. That is the property asked for: what is added later **extends**
the foundation rather than **revising** it.

---

## 10. What is not claimed

- **Nothing here is measured except §2.2's collision count.** This is a derivation and a map, and
  ADR 0007 says a design is killed by measurement. §8's tests are where it can be.
- **B is not a decision to rewrite the loader.** It is a model under which the next forms get
  designed. The leaning is to build A inside it and let demand pull the rest.
- **The prop row of §2.2 and the R4 completeness condition are the least-tested parts.** The first
  may turn out to be a coincidence of vocabulary. The second is the literature's result, and whether
  our rules meet its conditions has not been checked for a single rule.
