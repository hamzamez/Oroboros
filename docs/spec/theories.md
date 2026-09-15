# Theories: declarations, modules, models and views

**Status: specification, draft, nothing built.** This is build-order step 1 of
[theories-b-or-c.md](../theories-b-or-c.md) §9. It specifies what that research recommends, **C₁₂**:
theories, models, instantiation along the ambient view, and named views reserved but not built.
The research, the literature and the rejected candidates are in [theories.md](../theories.md) and
[theories-b-or-c.md](../theories-b-or-c.md). This document states only the rules. The data forms
the declarations name are in [data.md](data.md).

Sections marked **(to write)** are named so that the gaps are visible rather than discovered.

---

## 1. Declarations

### 1.1 The record

Every declaration form in §8 elaborates to one or more declarations

```
    x : A  [= t]  [↦ ρ]
```

| part | is | example |
|---|---|---|
| `x` | a name, qualified by its module path (§3) | `go/encoding/hex.Encode` |
| `A` | a classifier at one of three levels (§1.2) | a type, for a term |
| `t` | a **definiens**, optional: an equation δ may use | a `def`'s body; an alias's type |
| `ρ` | a **realization**, optional: how a target spells `x` | a `host` clause |

### 1.2 Levels

| level | `x` is | `A` is | forms |
|---|---|---|---|
| **type** | a type or type constructor | a kind: `type`, or `type → … → type` | `type`, `variant`, `record` names |
| **term** | an operation or a value | a type | `sig`, `def`, `const`, a variant's constructors |
| **prop** | a named fact | a proposition | `fact`, and a `sig`'s `where` and `ensures` |

### 1.3 Well-formedness: a well-founded order

The declarations of a program must admit an order in which every declaration's classifier and
definiens mention only declarations before it, or names in `lang` (§5.3).

This one rule is what refuses both a recursive type and a recursive definition
([theories.md](../theories.md) §2.3, ADR 0014). A future class of declaration may be allowed to
break it, with its own termination argument. That would be an addition to this specification, not a
relaxation of it.

---

## 2. The four cells, and resolution

Whether a declaration has a definiens and whether it has a realization on the current target puts
it in one of four cells:

| | no realization on `T` | a realization on `T` |
|---|---|---|
| **no definiens** | **not covered**: an error if the residual mentions it | **host**: reduction halts at `x` |
| **definiens** | **defined**: δ unfolds it | **conditional**: reduction halts; the definiens is the conformance claim |

**R2, generalised.** A declaration with a realization on the current target is *not unfolded*,
whatever its level. modules.md §5's *native wins* is this rule for terms. For types it means a type
with a host spelling is compared by name, not by its definiens. For a constant it means the emitter
writes the host's name and the analysis reads the value (§8.3).

**Covering** is unchanged: a program builds on `T` iff its residual mentions no not-covered
declaration (modules.md §5, target-system.md §4).

---

## 3. Modules

### 3.1 A file is a module

A **library file's** module path is the path it was found at: `lib/text/regex.oro` is module
`text/regex`. It declares no header, because modules.md already requires a library file to declare
exactly the module its path names, and a header would be redundancy the loader checks.

The **entry file** is the anonymous root module, and may contain `(module NAME …)` blocks.

A **target layer's file** belongs to the target its layer is for (target-system.md §7.2). Its
module path is declared by `(module PATH …)` blocks, because a target file realizes many modules.

### 3.2 Nesting and namespaces

`(module NAME decl…)` declares a **child** module whose path is its parent's path, then `/`, then
`NAME`. A module has three namespaces for its members, plus its children:

| namespace | holds |
|---|---|
| types | `type`, `variant`, `record` names |
| terms | `sig`, `def`, `const`, constructors |
| props | `fact` |
| children | `module` blocks |

The same identifier may name a type and a term in one module. That is ordinary: a variant named
`result` and a function named `result` do not collide.

### 3.3 Companions

**A child module with the same name as a type in its parent is that type's companion.** Its term
declarations are the operations on the type, which is how a host type's methods are declared.

**A child module and a type may share a name only in that relation.** A child with the same name as
a parent's *term* is an error. On the JVM a package containing a subpackage and a type of the same
name is already an error (JLS §7.1); here it is checked when the module is loaded.

### 3.4 Names and resolution

- **A qualified name** is a path, then `.`, then a member: `go/io.Writer`, `go/io.Writer.Write`.
  modules.md's R1 holds: targets and libraries name into one qualified namespace.
- **A name inside a module is resolved lexically**: first in the current module, then in each
  enclosing module in turn, then in `lang`. A companion therefore names its type without
  qualification.
- **`(use PATH [as NAME])`** binds a module, as today, and is resolved before reduction (ADR 0011).
- **A type name is resolved like a term name.** This is new: today a type is a key in one flat pool
  per target (`tg.Types`). Resolving it makes `go/io.Writer` and `hex.Writer` different names, which
  is what removes the base-name collisions measured in theories.md §2.2.

### 3.5 Elaboration

Resolution produces the flat qualified namespace the reducer receives today. Nothing about nesting,
companions or lexical scope survives into reduction. This is ADR 0011's decision, kept.

---

## 4. Composition

These are target-system.md §2's operations, specified there and restated here for declarations of
every level:

| operation | where | law |
|---|---|---|
| **glue** `⊔` | fragments within one layer | partial, commutative, associative, idempotent; defined iff the fragments agree on every shared name |
| **override** `▷` | layers, nearest first | total, associative, idempotent, not commutative |
| **reduct** | `export` | `(φ ⊔ ψ)\|S = φ\|S ⊔ ψ\|S` when the left is defined |

**One strictness is kept from layers-2026-09-07.** Under glue, a declaration repeated in one layer
is an error even when identical, because within a layer a repeat is a mistake.

---

## 5. Models

### 5.1 A target is a model

A target `T = (B, ρ_T)` pairs a backend from the closed set `{go, js, java, x86-64}` with
realizations for declarations left abstract by theories. A target layer's header:

```lisp
(target go (backend go))
(target windows (backend x86-64) (link "kernel32.lib" "ucrt.lib" "vcruntime.lib"))
```

### 5.2 `host`

A `host` clause is the only form that carries text meant for a host:

```
(host [expr | stmt | jump] "template" hclause…)      ; on a term
(host "spelling")                                     ; on a type
hclause ::= (import "…") | (lib "…") | (checked NAME) | (data …)
```

The target a `host` clause realizes on is **never written in the clause**. It is the target of the
layer the file is in, or the `TARGET` of the enclosing `provides`. So a file with no `host` clause
makes no claim about any host, and "portable" is a property a checker can test.

A `host` clause may be **attached** to the declaration it realizes (`sig`, `type`, `const`) or
**detached** inside `provides` (§5.4). Attachment is sugar for detachment.

### 5.3 `lang`

`lang` is the theory every module includes: `if`, `let`, `loop`, `the`, `quote`, `=`, the integer
operators, `array`, `map`, `tuple`, `record`, `with`, `build`, `set`, `len` and the rest of what the
compiler injects. **`lang` is closed.** No module may add declarations to it and no target may
realize one of its structural names (ADR 0017). The injected arithmetic is realized by spelling, as
`findEq` does today.

### 5.4 `provides`

```lisp
(provides TARGET PATH assign…)
assign ::= (host NAME hclause-or-template…)  |  (def NAME term)  |  (type NAME τ)
```

`provides` is the **ambient view** of the theory at `PATH` on `TARGET`: it realizes that theory's
declarations on that target. An assignment may be a realization, a definition, or a type.

**New here:** a `def` assignment. That is target-system.md §6's `D_T`, the target's own library.
Its free names must lie in `lang`, the target's realizations, and the theories in scope
(target-system.md §6.3). A `provides` may not declare a new name: every assigned name must exist in
the theory at `PATH`. **Not built** (theories-b-or-c.md §9, step 5).

### 5.5 Representation choices and model facts

```lisp
(repr (int LO HI) (host "spelling"))       ; today's (int-repr LO HI "spelling")
(repr big host)  (repr big limbs)          ; today's (big-repr …)
(repr shift N)                             ; today's (shift-width N)
(fact max-len ((a (array A))) (<= (len a) N))   ; today's (max-len N)
```

`repr` chooses a realization for a sub-lattice of types by containment, so it is not a declaration
of a name. `max-len` becomes a `fact` because it *is* one: a proposition true of every array on
this target, which the refinement layer uses.

### 5.6 Realizing `lang`'s type constructors

`lang` owns `array` and `map`, and every target must realize them. A target realizes a *type
constructor* the way it realizes a type: with a `host` spelling whose holes are the arguments.

```lisp
(type (array A)   (host "[]%s"))                  ; today's (array-type "[]%s")
(type (map K V)   (host "map[%s]%s"))             ; today's (map-type "map[%s]%s")
```

A target that spells no types (JavaScript, windows) declares neither. `lang` stays closed in §5.3's
sense: no target declares a new language name or realizes a binder. Realizing `lang`'s types is
exactly what a target is for.

### 5.7 A representation chosen by position: `(repr (ref T) …)`, today's `boxed`

**What it is.** On the JVM an integer has two representations: `long`, a primitive, and `Long`, an
object. A generic type argument must be a reference type (JLS §4.5.1), so `(map int int)` must be
spelled `Map<Long,Long>`. Java also has a `Double` for `f64` and a `Boolean` for `bool`.

```lisp
(repr (ref int)  (host "Long"))                   ; today's (boxed int "Long")
(repr (ref f64)  (host "Double"))
(repr (ref bool) (host "Boolean"))
```

`(ref T)` is **not a language type**. It names *the positions where the host requires a reference*,
and exists only as the subject of `repr`. It is ADR 0003 once more: one type, a representation
chosen by where the value sits. A target that declares no `(ref T)` spells `T` itself in those
positions, which is every target but Java.

**Where it is used today:** exactly two places, both on Java's map path.
- The spelling of a map type ([target.go:1906](../../emit/target.go)).
- A map read, `final Long box = m.get((long) key)` ([java.go:981](../../emit/java.go)), whose
  `null` test is the read's `none`.

**The second use is not a representation of `int`, and it is worth stating why.** `Long` has one
more value than `long`, `null`, so `Long ≅ long + 1 ≅ (option int)`. That map read is a **niche
encoding** of the option: `none` stored as `null`. sums-research.md §2.2 described that design
(`(option ptr)` with `none` as 0) and deferred it, and this is its first working instance. When the
niche encoding is specified, it should be specified from this instance:

```lisp
(repr (option int) (host "Long") (niche none "null"))    ; proposed, not specified
```

That also names the JVM hazard: unboxing a `null` throws, which is the extra point escaping into a
position that has no room for it.

### 5.8 A language type realized by a library: `(repr map library)`, today's `builtin-map`

**What it is.** A target may not decline `map` (ADR 0017's rule for language constructs). Go, Java
and JavaScript each realize it with the host's hash map. **windows has none**, so the language
supplies one: `emit/winmap.oro`, open addressing over an ordinary `build` buffer. It is lowered by
`emit/winmap.go` before reduction, so nothing downstream learns maps exist.

```lisp
(repr map library)                                ; today's (builtin-map)
(repr map host)                                   ; the default: the target's own map
(repr big limbs)                                  ; today's (big-repr limbs): the same kind of choice
(repr big host)
```

`builtin-map` and `(big-repr limbs)` are one kind of declaration: *realize this language type by the
language's own library rather than by the host*. They were spelled two ways only because they
arrived separately.

**Why it is declared, not inferred** (the reason in `winmap.go`, kept): an empty map spelling means
*no map* on windows and *no types at all* on JavaScript, which has a good map. Inferring it would
have given JavaScript our hash table, which measured 3.67× slower than its own.

**A composition bug the new loader fixes.** Today two fragments combine as
`BuiltinMap = a || b` ([target.go:1244](../../emit/target.go)), whose comment calls it monotone. That
is a join on `false < true`, not §4's override: a nearer layer can turn the built-in map on and
**can never turn it off**. `repr` composes by override like every other declaration, so a project
layer's `(repr map host)` wins. No program depends on the old behaviour.

**Where it is used today:** one declaration ([windows.oro:49](../../targets/windows/windows.oro)),
one check (`NeedsMapImpl`, read by `lowerMaps`), one test (`emit/winmap_test.go`).

**Where it goes after the loader:** into `targets/windows/` as the target's own library, a `provides`
holding `def`s (§5.4), with the `//go:embed` deleted. target-system.md §6.1 already lists `winmap.oro`
among three libraries *"in the wrong place"*. That move waits on one question: a map read is written
as application and has no name for δ to unfold. Today `winmap.go`'s rewrite gives it one, so the
library route needs map operations to have names before reduction.

### 5.9 `narrow`

```lisp
(repr narrow (host "%s := %s[:%s]"))              ; today's (narrow "%s := %s[:%s]")
```

A template, not a type: how this host restricts a container to a known length before a loop, which
hands the host's bounds-check elimination a proof it accepts (target-files.md, `index` and
`narrow`; bce-2026-08-15). Declared only by Go. A target that declares none gets no transformation.

---

## 6. Views

### 6.1 `implements`

```lisp
(implements T I…)
```

A view from `I`'s companion to `T`'s. **Checked** (not built): for every term declaration `m` in
`I`'s companion, `T`'s companion has a declaration `m` whose type is at least as specific:
- `self : I` translated to `self : T`;
- the other arguments contravariant;
- the results covariant.

The realization is the identity: the host coerces, `⟦coerce⟧ = id` (target-files.md §2a). By
theories.md §4.4's T2, views that carry no run-time evidence cannot be incoherent, wherever they
are declared.

### 6.2 `include`

```lisp
(module WriteCloser (include Writer Closer))
```

Inside a companion, `include` is a theory inclusion: every declaration of the included companions,
with `self` translated to the including type. **It derives the view:** `T ≤ I` holds for every
companion `I` that `T`'s companion includes, directly or transitively, with no `implements`
written.

### 6.3 Reserved

- `(use PATH with VIEW)`: instantiation along a named view.
- `(view NAME FROM TO assign…)`: a named view.

Both are accepted by the reader and refused by the loader with *"named views are specified in
theories-b-or-c.md and not built"*, so neither word can acquire another meaning first.
theories-b-or-c.md §7 names the trigger: a program that needs a view other than the ambient one.

---

## 7. Facts

**Decided 2026-09-15: F-B**, local boundedness facts, from [facts.md](../facts.md). **F-C, F-D and
F-E are kept open**, by construction rather than by intention: each is a *reserved fragment* that
the admission check recognises and refuses by name (§7.6). Building one later adds a fragment; it
never loosens F-B's rules.

### 7.1 What a fact is

A fact is a prop-level declaration (§1.2): an **axiom schema**

```
    ∀x̄.  G₁(x̄) ∧ … ∧ Gₘ(x̄)  →  C(x̄)
```

added to the base theory the refinement layer decides. It is **not** a proof rule, an induction
principle or a law of `lang`'s data. Those are the compiler's (facts.md §1.1), and no form here can
state one.

### 7.2 Surface

```lisp
(fact NAME ((x τ) …)
  [(when φ …)]            ; guards: linear, conjoined
  φ)                      ; the conclusion: a linear inequality, an equation, or (and …) of them
```

```lisp
; F1 and F2 — both halves of Euclidean division by a positive literal
(fact div-floor ((x int) (k int))
  (when (<= 0 x) (<= 1 k))
  (and (<= (* k (/ x k)) x)
       (<= x (+ (* k (/ x k)) (- k 1)))))

; F6, the positive-divisor half — the remainder is under the divisor, sign of the dividend
(fact rem-below ((a int) (b int))
  (when (<= 1 b))
  (and (<= (- 1 b) (% a b)) (<= (% a b) (- b 1))))

(fact rem-nonneg ((a int) (b int))
  (when (<= 0 a) (<= 1 b))
  (<= 0 (% a b)))

; a host law — encoding/hex doubles length
(fact encode-length ((src (array (int 0 255))))
  (= (len (hex.EncodeToString src)) (* 2 (len src))))
```

The reader's connectives are erased (ADR 0017), so `and` in a conclusion arrives as
`(if a b false)`. `assume` already reads that form, and a conclusion is read the same way.

### 7.3 Base terms, extension terms, the trigger

- A **base term** is a parameter, an integer literal, or `+`, `−` or `*`-by-a-literal of base terms:
  what the linear fragment interprets.
- An **extension term** is any other integer-valued application whose arguments are base terms:
  - `(/ x k)`, `(% a b)` and `(* x y)` with neither factor a literal;
  - `(len t)`;
  - a host shift or mask;
  - an application of a **pure** declared name.

  It is exactly what `asLinear` turns into an atom named by its printed form.
- The **trigger** of a fact is the set of extension terms occurring in its guards and conclusion.

### 7.4 Admission: the F-B fragment

A fact is admitted as **F-B** when all of these hold. Each condition is one clause of locality
(facts.md §2.2):

1. **One trigger term.** Exactly one distinct extension term occurs, possibly several times:
   `(% a b)` twice in `rem-below` is one term.
2. **Flat.** The trigger's arguments are base terms, so no extension term occurs inside another.
3. **Covering.** Every parameter occurs in the trigger, so an instance is determined entirely by a
   term already present.
4. **Linear elsewhere.** Once the trigger is read as an atom, every guard and every conjunct of the
   conclusion is linear.
5. **Integer, and no floats.** A float fact is refused (facts.md §8: IEEE arithmetic is not an ordered
   field, and ADR 0009).

A fact that fails is refused with the condition it failed, unless it matches a reserved fragment
(§7.6), in which case it is refused with that fragment's name.

### 7.5 Meaning: one declaration, three consumers

**The linear layer — instantiation on present terms.** For a query with facts `F` and goal `g`, and
for every occurrence of a fact's trigger shape in `F` or `g` with matching arguments `ā`: if
`F ⊢ G(ā)`, the instance `C(ā)` is available to that query.

- **The guard must be entailed, not merely consistent** (Lemma 1, postconditions.md §4).
- **An instance creates no term that triggers another**, so instantiation terminates (facts.md §2.2).
  This is `seedDivAxioms`, generalised and moved into data.

**The interval layer — the induced transfer (facts.md, Theorem T).** For an application `f(ā)` with
argument intervals `X̄`, every fact about `f` whose guards hold on all of `γ(X̄)` (decided by
interval entailment) gives the interval `[lo(s#(X̄)), hi(t#(X̄))]` from its bounds. The transfer is
the **meet** of those intervals, and of the compiler's own transfer where one exists.

**Element narrowing** is the interval layer's rule on exact ranges, point intervals.

**The compiler's own transfers for `+`, `−` and `·`** remain. They are the best abstraction of
those operators, not facts (facts.md §1.2).

### 7.6 Reserved fragments — the open doors

| fragment | what it would admit | recognised by | refused with |
|---|---|---|---|
| **F-C** | a relation between several extension terms, e.g. monotonicity `(when (<= x y)) (<= (f x) (f y))` | two or more distinct extension terms | *"a fact relating two terms is F-C, reserved (facts.md §5)"* |
| **F-D** | a quantified fact over a table's contents: the array property fragment | a `(forall ((i int)) …)` in a conclusion | *"a quantified fact is F-D, reserved (facts.md §5)"* |
| **F-E** | a law about a `def`, **proved by the compiler** by unfolding | the form `(lemma NAME …)` | *"`lemma` is F-E, reserved (facts.md §5)"* |

`forall` and `lemma` are **reserved words** from this specification on, so neither can be taken for
something else before its fragment is built. `fact` is the **assumed** form, whatever the evidence
behind it (§7.8). `lemma` is the **proved** form, following Why3's split.

**Also reserved:** a bound written as a compile-time function of literals, such as `isqrt`
(facts.md §5.1). F4 stays a compiler transfer (`narrowSquare`) until a second fact needs such a
function.

### 7.7 Scope and composition

- **A fact is in scope** where its module is, and **active** where its trigger occurs.
- **`lang`'s facts** are declarations in a theory shipped with the compiler, read by the loader like
  any other. Facts F1–F12 of facts.md move there, and their Go encodings are deleted.
- **A target layer may declare facts** whose trigger is `len` (a model's limit on tables, e.g. Java's
  `max-len`) or a name that target realizes. **It may not declare a fact whose trigger is one of
  `lang`'s arithmetic operators**, or a program's provability would change between hosts for a reason
  that is not about the host.
- **Glue and override** apply as to every declaration (§4). Overriding a fact is safe: it can only
  remove proofs, and no proof outlives a build (facts.md §4).
- **A proof that used a target's fact is not portable**, like a program using that target's
  primitive. Reporting it is owed.

### 7.8 Evidence: every fact has a check that can fail

| fact about | evidence required |
|---|---|
| `lang` | a proof in this repository **and** a named test that fails when the fact is deleted or weakened |
| a target's model | a host probe |
| a host function | an acceptance program against the host |
| a user's `def` | none admitted as `fact` in a program's own module until F-E: the compiler cannot check it |

### 7.9 Contracts are facts

A primitive's `ensures`, its result range and its `(length N)` are facts whose trigger is the
application `(f x̄)` (facts.md §6). When a contract is of F-B's form, it is **consumed exactly as a
`fact` is**. That extends contracts to **pure** calls, which today carry them only as an opaque atom.
It is gated on the measurement facts.md §6 names: naming more atoms can turn *propagated, not
proven* into a refusal.

### 7.10 Acceptance

1. **Consolidation with no change.** F1–F12 written as `lang` facts and the Go encodings deleted. Pass
   requires:
   - every emitted file byte-identical on four targets;
   - **every per-program proof count identical** (integer operations bounded, loops proven).
2. **The evidence can fail.** Deleting `div-floor`'s upper half fails the hex Decode witness in
   `emit/multiwhere_test.go`, and weakening `rem-below` fails `TestDivisionAndRemainderContain`.
3. **One new capability.** An `ensures` on a pure primitive discharges an obligation that HEAD only
   propagates. The witness is written first, and it fails against HEAD.

**What would refute this section:** an induced transfer proving less than the hand-written one on some
program, with no set of facts that recovers it. Then Theorem T's single source would need a second
encoding after all.

---

## 8. The forms

### 8.1 Grammar

```
decl  ::= (use PATH [as NAME] [with VIEW])
        | (export NAME…)
        | (module NAME decl…)
        | (include NAME…)                                  ; inside a companion only
        | (type NAME [τ] [(host "spelling")])
        | (variant TNAME ctor…)                            ; data.md §5
        | (sig NAME ((x τ)…) τ [pure] clause…)
        | (def NAME term)
        | (const NAME term [(host …)])
        | (fact NAME ((x τ)…) [(when φ…)] φ)               ; §7; F-B admitted
        | (lemma NAME …)                                   ; reserved: F-E (§7.6)
        | (implements T I…)
        | (provides TARGET PATH assign…)
        | (view NAME PATH PATH assign…)                    ; reserved
        | (target NAME (backend B) clause…)                ; target layers only
        | (repr τ-or-word clause…)                         ; target layers only
clause ::= (where φ) | (ensures φ) | (host …)
ctor   ::= NAME | (NAME τ…)
TNAME  ::= NAME | (NAME A…)
τ      ::= int | f64 | bool | string | (int e e) | (array τ) | (map τ τ) | (buffer τ)
         | (tuple τ τ…) | (record ('l τ)…) | NAME | (NAME τ…) | (fn (τ…) τ)
```

### 8.2 `sig` and `def`

Unchanged in meaning. A `sig` is a term declaration's classifier with its contracts, and a `def`
its definiens. They may appear separately, in the same module. A `sig` with a `host` clause and no
`def` is what `prim` is today.

### 8.3 `const`

```lisp
(const MaxRune 1114111 (host "utf8.MaxRune"))
```

Elaborates to:
- `(sig MaxRune (int 1114111 1114111) pure)`;
- `(def MaxRune 1114111)`;
- the host realization.

It is the conditional cell of §2. The emitter writes `utf8.MaxRune`, and `(int 0 utf8.MaxRune)` is a
legal range endpoint because the definiens is available when the endpoint is evaluated. `term` must
be a literal, since it is a compile-time value.

### 8.4 From today's forms

| today | here |
|---|---|
| `(prim n args r kind "t" pure (where …) (import …))` | `(sig n args r pure (where …) (host kind "t" (import …)))` |
| `(type NAME "spelling")` in a flat pool | `(type NAME (host "spelling"))` in the module that owns it |
| `(target T (module P prim…))` | a target layer file, with `(module P …)` blocks |
| `(provides T M prim…)` | `(provides T M assign…)` |
| `(length N)`, `(length-of N)` | `(ensures (= (len result) …))`, **gated** on pure-call postconditions |
| `(int-repr …)`, `(big-repr …)`, `(shift-width …)`, `(max-len …)` | §5.5, §5.8 |
| `(array-type "…")`, `(map-type "…")` | `(type (array A) (host "…"))`, `(type (map K V) (host "…"))` — §5.6 |
| `(boxed T "s")` | `(repr (ref T) (host "s"))` — §5.7 |
| `(builtin-map)` | `(repr map library)`, composed by override — §5.8 |
| `(narrow "…")` | `(repr narrow (host "…"))` — §5.9 |
| `(sum …)`, `(values …)`, `(A B)` result lists | [data.md](data.md) §10 |

---

## 9. Acceptance

The first build of this specification is an **elaborating loader**: the new forms are read and
turned into today's `emit.Target`, `emit.Prim` and `core.Module` structures, and nothing downstream
changes. It passes when:

1. every target file and program in the repository is translated mechanically;
2. **every emitted file is byte-identical** on all four targets;
3. the differential suite and the tooling suite are green, including
   `TestHandDeclarationsAgreeWithTheHost` catching all of its planted mistakes.

4. every refusal in §10.3 has a test that builds the failing declaration and checks §10.7's three
   properties.

Item 2 is also the test of theories-b-or-c.md §2.1's central claim, that resolution against a target
and instantiation along the ambient view give the same names in every case. If they differ anywhere,
that is where it shows.

---

## 10. Diagnostics

Every rule in this specification can fail. This section decides, for each failure, whether it is an
**error**, a **note**, or **legitimate**, and what the message must say. It exists because the
easiest implementation of a rule is to ignore the declarations that break it. Here, ignoring a
declaration has repeatedly meant a program that compiles and gives a wrong answer:
- `split-words` passed every check for two months while giving different answers on different
  targets (modules.md §8);
- `CheckEnsures` returned success with a note when a claim was outside its fragment
  (scalarrange-2026-08-31);
- `CheckSigs` swallowed the honest refusal about the limb representation and printed a misleading
  one (subdiv-2026-09-03).

### 10.1 Principles

**D1 — No silent acceptance.** A declaration that breaks a rule is refused, or its case is listed in
§10.4 as legitimate. An implementation that drops such a declaration without doing either does not
conform to this specification.

**D2 — A declaration error is reported when the declaration is loaded.** Anything decidable from
declarations alone is reported before any program is reduced, whether or not a program uses the
name. This follows from *declarations are data* (theories-b-or-c.md §6.2). Today a template is
checked only when some program happens to call it (coercion-2026-09-09), and a declaration error
that waits for a program is one that some program will find. **Covering is the exception**: it is
defined relative to a program (§2).

**D3 — Provenance.** Every declaration carries its **origin** (§10.2). A message about one
declaration names its origin; a message about two names both.

**D4 — Source spelling.** Names are printed as they are spelled at the reported site, with the
qualified name in parentheses where the two differ:

```
Writer (go/io.Writer)
```

This is always possible, and for a structural reason: resolution is an **injective** renaming
(§3.4; theories.md §3, T1), and an injective map has an inverse, so every elaborated name has exactly
one source spelling in a given scope. The flat type pool this specification replaces could not
promise that. `template-Template` named two types (theories.md §2.2).

**D5 — Say what to write.** Every error names the rule it breaks and the correction, to the standard
products.md §3 set: *"every refusal … by name, checked, and has a message that says what to write
instead."*

**D6 — A failed proof names its premises.** The `known:` list shows every premise with its source:
- a `where`, with its declaration's origin;
- a guard;
- a fact, **by name and instance**: `div-floor at (/ (len src) 2)`;
- an interval rule derived from a fact (§7.5).

A premise that came from a **target layer's** fact is marked, because the proof is then not portable
(§7.7).

**D7 — Deterministic output.** Diagnostics are ordered by file, then line, then name, which is a total
order. A ranked report with ties once swapped between identical runs (tooling-2026-09-11).

**D8 — A note never stands in for an error.** A note is printed and changes nothing. If a rule's
failure can change what a program means, it is an error.

### 10.2 The origin record

```
origin ::= (file, line, layer, module, via)
via    ::= written
         | sugar(form, origin)        ; e.g. the const, attached host or include that produced it
         | generated(tool)            ; e.g. gauntlet/stdlib/survey.go
```

- `layer` is the declaration's position in §4's chain, nearest first: the program's directory, then
  each `-targets` entry, then the built-ins, then a library's `provides`.
- **Sugar records what it expanded from.** An error in `(sig MaxRune (int 1114111 1114111) pure)` is
  reported as the `const` that produced it, at the `const`'s line.
- **Generated declarations record the tool that wrote them**, so a message can say that a
  declaration was generated rather than hand-written. The distinction is handdecl-2026-09-14's
  decision: hand declarations are checked *against* generated ones.

**Terms do not carry origins in this specification.** The reader tracks lines while parsing
(`core/read.go`) and discards them, so positions on terms would be a compiler-wide change. It would
touch the same term-rebuilding code (`Body()`, `openFresh`) that five recorded bugs came from. That is
a decision of its own (§10.6). Declarations are data the loader already holds, and giving them
origins costs nothing downstream.

### 10.3 The refusals

"Fires" is **L** (load, by D2) or **P** (program, after resolution or reduction). "Names" lists what
the message must contain beyond the rule and the correction.

#### Reading

| # | condition | fires | names |
|---|---|---|---|
| R1 | `'` not followed by an identifier: `'(…)`, `'3`, `'"s"`, or `'` inside an identifier (data.md §2.2) | L | the token and its line |
| R2 | a retired spelling: `sum`, `values`, the type `(array A B)`, a result list `(A B)` (data.md §10) | L | the replacement |
| R3 | a reserved word used as a name: `forall`, `lemma`, `view`, `quote`, or `with` outside `use` | L | the fragment or section it is reserved for |
| R4 | a `(module …)` header in a library file | L | the module name the file's path already gives |

#### Modules and resolution

| # | condition | fires | names |
|---|---|---|---|
| M1 | an unresolved name | P | every module searched, innermost first, ending in `lang`; the nearest declared spellings (§10.5) |
| M2 | a `use` path with no file, and no target layer provides the module | P | the search path and the target layers searched (modules.md's `Program.Unresolved`, kept) |
| M3 | **a child module with the same name as a *term* in its parent** (§3.3) | L | both declarations' origins |
| M4 | **an ambiguous qualifier**: `Writer.m` where `Writer` is both a `use` alias and a companion in scope | P | both, and *"rename the alias"* |
| M5 | two `use` bindings of one alias to different paths | L | both paths (today's error, with origins added) |
| M6 | an `export` or a `sig` naming nothing in its module | L | the module, and the nearest names (today's rule, kept) |

#### Composition

| # | condition | fires | names |
|---|---|---|---|
| C1 | two declarations of one name within a layer disagree | L | **both** origins and the parts that differ |
| C2 | one declaration repeated identically within a layer (§4) | L | both origins |
| C3 | a target layer declares a fact whose trigger is `lang` arithmetic (§7.7) | L | the fact and its origin |
| C4 | a nearer layer **overrides** a declaration and changes its classifier or its realization | L | **a note**: both origins. Legitimate by §4, but it can change emission for an existing program (target-system.md T5′), so it is shown |

**C1 changes today's behaviour.** `combineMap` reports *"`from`: type X is declared as A and as B"*
([target.go:1222](../../emit/target.go)), where `from` is the file being merged in. The file that
declared the first version is not named. Both must be.

#### Models

| # | condition | fires | names |
|---|---|---|---|
| H1 | **a `host` clause with no target context**: not in a target layer and not in a `provides` (§5.2) | L | the clause, and *"move it into `targets/T/…` or a `(provides T …)`"* |
| H2 | a template hole beyond the declaration's parameters | L | the hole and the arity. A hole may be *omitted*: on x86-64 a declared arity is a lower bound on what a template writes (win32-2026-09-08) |
| H3 | `(provides T M …)` where no theory `M` exists | L | the search path |
| H4 | **`provides` assigns a name `M` does not declare** | L | the name, and the nearest names `M` declares (§10.5) |
| H5 | a `def` in a `provides` whose free names leave `lang`, `T`'s realizations and the theories in scope (§5.4) | L | each free name, and where it was looked for |
| H6 | a `provides` assignment whose type disagrees with the theory's declaration | P | both origins |
| H7 | a target realizes a structural name of `lang` (ADR 0017) | L | today's error, with the origin |
| H8 | contradictory representation choices, such as `(repr map library)` beside a map type spelling | L | both origins |
| H9 | `(ref T)` anywhere but the subject of a `repr` (§5.7) | L | *"`ref` is not a type"* |
| H10 | a target with no `backend` is used for emission | P | today's refusal, kept (backend-2026-09-06) |

**H4 is the case this section most exists for.** Without it, a misspelled assignment realizes nothing:
the native implementation is never selected, δ unfolds the portable definition instead, and the
emitted program changes without a word.

#### Views

| # | condition | fires | names |
|---|---|---|---|
| V1 | `implements T I` and `I`'s companion declares a method `T`'s does not | L | the method, and both companions' origins |
| V2 | `implements T I` and a method's type does not satisfy the view's variance (§6.1) | L | the method, the parameter position, and both types |
| V3 | `include X` where `X` is not a module in scope | L | the name |
| V4 | a cycle of `include`s | L | the whole cycle, in order (§1.3's well-founded order) |
| V5 | `(use M with V)` or `(view …)` | L | *"named views are specified in theories-b-or-c.md and not built"* (§6.3) |

#### Facts

| # | condition | fires | names |
|---|---|---|---|
| F1 | a fact fails F-B admission (§7.4) | L | **which condition failed**, and the offending subterm: e.g. *"`(/ (% a b) 2)` nests one extension term inside another (condition 2)"* |
| F2 | a fact matches a reserved fragment (§7.6) | L | the fragment: F-C, F-D or F-E |
| F3 | a proof used a premise from a target layer's fact | P | **a note** (D6): the fact and its layer, and *"this proof does not hold on other targets"* |
| F4 | a `lang` fact with no named evidence test (§7.8) | test time | the fact. This is checked by a repository test, not by the loader, because evidence is a test that must exist |

#### Data forms (data.md)

| # | condition | fires | names |
|---|---|---|---|
| X1 | a label repeated in a record type, literal or `with` | L/P | the label, both positions |
| X2 | a record literal missing a label, or giving one the type lacks | P | the missing and the extra labels |
| X3 | projection of a label the record's type does not have | P | the type's labels |
| X4 | a dynamic index into a tuple, or a computed label | P | *"write an `array`"* or *"write a `map`"* (data.md §3.2, §4.2) |
| X5 | `(tuple)` or `(tuple T)` | L | data.md §3.1's reason |
| X6 | a heterogeneous `(array …)` literal | P | *"write `(tuple …)`"* |
| X7 | a symbol reaches the residual (data.md §2.4) | P | the symbol, and the export being built |
| X8 | `(t (fn (x…) …))` whose arity is not the tuple's | P | both arities |
| X9 | a `case` that misses a constructor | P | the missing constructors (sums.md's rule, kept) |

#### Covering

| # | condition | fires | names |
|---|---|---|---|
| K1 | the residual mentions a name no layer realizes on `T` and no definition gives (§2) | P | where the name is declared (origin), **every layer searched for `T`**, and whether another target realizes it |

### 10.4 Legitimate — must not be refused

Each of these looks like a mistake and is not. A conforming implementation must accept them without an
error. Refusing one would break a rule elsewhere in this specification.

| case | why it is legitimate |
|---|---|
| a `provides` that realizes only some of a theory's declarations | partial realization is how porting works (modules.md §4); covering reports what a program actually needs |
| a child module with the same name as a *type* in its parent | that is a companion (§3.3) |
| a type and a term with one name in one module | separate namespaces (§3.2) |
| a nearer layer overriding a declaration | that is what `▷` is for (§4); C4 shows it as a note |
| a fact that no program ever triggers | an unused fact licenses nothing and costs nothing |
| a template that omits a parameter's hole | a statement's template may write fewer, and x86-64 may write more (H2) |
| the same realization reached through two layers | an `implements` edge is a set, so a repeat is not a disagreement (coercion-2026-09-09) |

### 10.5 Suggestions

A message may suggest the declared names nearest to a misspelling: M1 and H4. A suggestion must be
**deterministic**, so that two runs print the same text (D7):
- distance is Damerau–Levenshtein at most 2;
- candidates are drawn from the scope searched;
- ties break by name.

A suggestion is never applied, only printed.

### 10.6 Messages about residual terms

Checks that run after reduction — refinement, intervals, termination, linearity — see a term that
reduction and the lowering passes have rewritten. Until terms carry origins (§10.2), such a message
must:
- name **the export being built** and **the `def`s inlined into the reported subterm**, where
  reduction's δ steps recorded them;
- print names in source spelling (D4);
- **print a term a lowering pass rewrote as the term it was rewritten from**, where the pass can
  record it.

The witness is the tally failure the refinement-walk fix produced (hex-2026-09-14 §4):

```
build: main: (dt (+ (* 2 s) 1)) is an indexing, and (< (+ (* 2 s) 1) (len dt)) does not follow
  known: …, s joined over its branches, s joined over its branches
```

The program wrote `((dt s) 1)`. The message describes the stride the product-flattening pass
generated and a join the refinement layer derived, so neither the index nor the premise is
recognisable. The flattening pass is the first place to record a rewrite's original.

**Positions on terms are owed**, as a decision in their own right, with that rebuilding risk weighed
first.

### 10.7 Acceptance

Every row of §10.3 has a test that builds the failing declaration and checks three things:

1. **It is refused**, or noted where the row says note. For H1, H4 and M3, which today are accepted
   silently or cannot yet be written, the test is written first and must fail against HEAD.
2. **The message contains every item in the row's "names" column**, including both origins where a
   row names two.
3. **The message contains no internal spelling.** Pinned by a pattern that fails on any of:
   - a generated binder hint such as `#tag` or `#k`;
   - a quotient atom `div(`;
   - a flat-pool type prefix such as `ptr-os-`;
   - a strided index `(+ (* k i) j)` for an index the source wrote as a projection.

And every row of §10.4 has a test that the case is **accepted**. A refusal of a legitimate case is a
bug of the same size as an acceptance of a wrong one.

---

## 11. Not yet specified

- **Reserved** fact fragments F-C, F-D and F-E (§7.6), and literal-only functions in a fact's bounds.
- ~~How a type parameter of a variant is resolved across modules~~ — **specified** in
  [data.md §5.5](data.md): identity is the qualified declaration plus its arguments; type arguments
  are never inferred; the boundary representation is one slot per distinct payload type
  (Theorem R). The load-order bug found on the way is fixed in the interim (`core/sumclash_test.go`).
- **Named views** — reserved (§6.3).
- **Positions on terms** — owed as its own decision (§10.6).
