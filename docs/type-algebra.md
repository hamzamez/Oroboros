# The type algebra, and `match`

hamza: *"this looks a lot like algebraic types. if we are going to do it, let's do it right. we
already have types as functions, arrays as functions, and datatypes as sets. and just like in
mathematics we add products `float * int` and sums … let's add the whole machinery to express our
types. and pattern matching is on the menu — why just add `case`, make it like `loop` and `cond`,
with `again` being match again."*

**Two proposals. I agree with the second completely and it is cheaper than it looks. I agree with
the first with three exclusions, and the exclusions are where the decidability map bites.**

---

## 1. The answer in one line

> **Take the semiring — `+`, `×`, `0`, `1` — and the exponential. Refuse fixed points, subtyping and
> untagged unions. Make `match` sugar over `loop`, which costs the reducer nothing and gives the
> refinement checker *more* than `if` does.**

The rest is why.

---

## 2. The algebra we would have

### 2.1 It is a semiring, and then a bicartesian closed category

Types as sets, with cardinality as the semantics:

```
|A × B| = |A| · |B|        |A + B| = |A| + |B|        |A → B| = |B|^|A|
|1| = 1                    |0| = 0
```

The laws, all up to isomorphism:

```
A + 0 ≅ A              A × 1 ≅ A               A × 0 ≅ 0
A + B ≅ B + A          A × B ≅ B × A
(A+B)+C ≅ A+(B+C)      (A×B)×C ≅ A×(B×C)
A × (B + C) ≅ (A × B) + (A × C)                          distributivity

A^(B+C) ≅ A^B × A^C    (A×B)^C ≅ A^C × B^C    (A^B)^C ≅ A^(B×C)
A^1 ≅ A                A^0 ≅ 1                1^A ≅ 1
```

`(Types, +, ×, 0, 1)` is a **commutative semiring**; adding exponentials makes it a **bicartesian
closed category**, which is Lambek's and Lawvere's semantics for simply-typed λ-calculus with
products and sums. This is not an analogy — it is the standard model, and "algebraic data type"
means precisely a type built from `+`, `×`, `0` and `1`.

Three of those laws are load-bearing for us and worth naming:

**Associativity and commutativity are up to isomorphism**, so a **flat n-ary product and a flat
n-ary sum are as legitimate as nested binary ones.** No re-association work, and `divmod`'s pair and
a three-field record are the same shape to the reducer.

**Distributivity `A × (B+C) ≅ (A×B) + (A×C)` *is* case-of-case**
([sums-research §1.1](sums-research.md)). The transformation that makes a locally-consumed sum free
is a law of the algebra, not an optimisation bolted on top.

**`A^(B+C) ≅ A^B × A^C`** says a function *from* a sum is a pair of functions — which is exactly
what a `match` clause list is. The elimination form falls out of the algebra.

### 2.2 What each operation costs — and this is the law of the language

| | free when | must exist when |
|---|---|---|
| `A × B` | consumed in the same reduction — measured **1.01×**, zero allocations | it escapes a boundary |
| `A + B` | the tag is **static** — `((inl x) f g) → (f x)` | the tag is **dynamic** |
| `A → B` | it reduces away | it survives — refused as a closure |
| `Π(Fin n). T` (table) | given by a rule | `alloc`, or a dynamic index |
| `1` | always — no representation | never |
| `0` | always — no values | never |

> **Every operation in the algebra is free at the static level and priced at the dynamic level.**

That is the same boundary for the fourth time — product, table, sum, function — and it is the
two-level language ([closures-direction.md](closures-direction.md)) stated in the type system. It is
the reason "add the whole machinery" is affordable: **the machinery is free where it is used to
think, and priced where it is used to run.**

### 2.3 The one thing the algebra makes us honest about

`|A + B| = |A| + |B|` is only true if the two sides are **distinguishable**. That is the whole
argument of §3, and it is why `success | fail` and `float | int` are not the same proposal.

---

## 3. Three exclusions, argued

### 3.1 Fixed points (μ) — refused, and measured

`μX. 1 + A × X` is a list. Adding `μ` gives the whole inductive design space and brings back
recursion to consume it, allocation per node, and a heap whose acyclicity
[ADR 0018](decisions/0018-immutable-values-linear-buffers.md) depends on.

**Refused, and the alternative is measured rather than argued.** Recursive data becomes a **flat
table of nodes with integer indices** — simdjson's tape, Zig's AST as a `MultiArrayList` of `u32`
indices — and [indexgraph-2026-08-21](../gauntlet/results/indexgraph-2026-08-21.md) measures that
form at **2.02× faster** than the pointer form on realistic irregular access.

A JSON node is `(sum node null bool (num f64) (str int) (arr int) (obj int))` — **non-recursive**,
with children as indices. Closed, finite, flat.

So the precise statement of what we would have:

> **A bicartesian closed category without fixed points, plus one indexed family — tables — whose
> index set is sized at run time.**

Small, complete, and finite except in the one place finiteness was the thing we needed to give up.

### 3.2 Subtyping — refused, because we already have the decidable half

"Datatypes as sets" invites set *inclusion*, and inclusion is subtyping. This is the most tempting
of the three and the most dangerous.

**Full subtyping with polymorphism is undecidable** — Pierce (1992) proved F<: subtyping
undecidable, and it is one of the named cliffs on [decidability-map.md §1](decidability-map.md).
Even without polymorphism, subtyping plus unions plus intersections is where type systems go to
become research projects.

**And we do not need it, because the refinement layer already is bounded subtyping done decidably.**
`{i | 0 ≤ i < n} ⊆ int` is exactly a subtype relation, and `emit/refine.go` decides it in
quantifier-free linear arithmetic. That is subtyping *within* a type, on a decidable fragment —
which is Dependent ML, which [types-direction §6.5](types-direction.md) already identified as our
lineage.

> **We have the useful half of subtyping already. Generalising it is the cliff.**

**Intersections** follow the same argument and lose worse: intersection-type *inference* is
equivalent to strong normalisation and therefore undecidable.

### 3.3 Untagged unions — refused, and `float | int` is the case that shows why

This is the one place I disagree with the proposal as written, and the disagreement is precise.

`success | fail` is a **coproduct**: two things, distinguishable, `|A| + |B|`. That is a tagged sum
and it is exactly what we want.

`float | int` written as a *union* is a different object. Set union is **idempotent** — `A ∪ A = A`
— and **not** a coproduct: `|A ∪ B| ≠ |A| + |B|` when they overlap. TypeScript, Flow and Ceylon
have untagged unions and they work there for one reason: **JavaScript has runtime types**, so
`typeof x` discriminates.

Three reasons to refuse:

1. **Three of four hosts have no generic runtime type test.** Go's `interface{}` type switch costs
   an allocation to get into; the JVM's `instanceof` needs a boxed object; x86 has nothing at all.
   To discriminate an untagged union we would **add a tag** — at which point it is a tagged sum with
   worse ergonomics and a lost algebra.
2. **It requires subtyping** (`A <: A|B`), which is §3.2.
3. **It breaks the semiring**, which is the machinery the proposal asks for. `A + A ≇ A` is a
   *feature*: two distinguishable copies is what `(result T T)` means.

**But the representation may be untagged, and often is.** This is the important qualification:
[sums-research §2.2](sums-research.md) showed that `(option ptr)` lowers to NULL, `(option int)`
from `strings.Index` lowers to −1, and HRESULT's failure is a sign bit. **Tagged in the semantics,
niche-encoded in the representation** — which is Rust's answer and costs nothing.

So: `(| f64 int)` as a *type* is refused; `(sum number (f f64) (i int))` is the same thing with the
tag named, and it costs the same at run time.

---

## 4. Anonymous or named

**Products: anonymous.** `(× f64 int)` needs no names — the components are positional, `divmod`'s
result is `(× int int)`, and naming it is ceremony. ML makes tuples anonymous for exactly this
reason.

**Sums: named, and this is not arbitrary.** `(inl 3.0)` does not determine its type — it could be
`f64 + int` or `f64 + string`. Anonymous sums therefore need either non-local type inference or an
untagged representation, and §3.3 rules out the second. Every language that kept anonymous sums
(TypeScript) has runtime types; every language that did not (ML, Haskell, Rust, Swift) went
nominal.

**The middle ground is parameterised named sums**, and it is what everyone actually uses:

```lisp
(sum (option T)   none (some T))
(sum (result T E) (ok T) (err E))
```

which gives anonymous-like ergonomics for the two cases that matter without the inference problem.

---

## 5. `match`, and it is `loop`

This is the part of the proposal I think is straightforwardly right, and it is cheaper than a `case`
form would have been.

### 5.1 The proposal

```lisp
(match (a b c)
  p1 p2 p3   (again a' b' c')
  q1 q2 q3   body
  else       body)
```

Clauses over n scrutinees, with `again` meaning *match again with new values*.

### 5.2 Why loop-shaped is right

**It is what a function-by-clauses language already is.** Erlang:

```erlang
loop(0, Acc) -> Acc;
loop(N, Acc) -> loop(N-1, Acc+N).
```

Clause heads plus a tail call. The proposal is that shape with `again` instead of the call — which
keeps [ADR 0014](decisions/0014-recursion-is-not-in-the-language.md) intact, because `again` is a
jump. Prolog, ML's `function`, Haskell's equations and Rust's `loop { match … }` are all the same
shape.

**`loop` and `match` are already the same construct.** [ADR 0015](decisions/0015-loop-and-again.md)
gives `loop` *guarded clauses over n variables*; the proposal gives *pattern clauses over n
scrutinees*. A boolean guard is a pattern on a `bool`, and the scrutinee expressions are the loop
variables' initial values:

```
(match (e₁ … eₙ)  pats  body  …  else  body)
     ≡
(loop ((v₁ e₁) … (vₙ eₙ))  guard  body  …  else  body)
```

**And this is the construct a general-purpose language needs.**
[general-purpose.md §2](general-purpose.md) listed parsers, event loops, protocol handlers and
request dispatch. Every one of them is *match on (state, input), transition* — a state machine —
and that is precisely `(match (state input) … (again state' input') …)`. It is also exactly what
[decidability-map.md §7.1](decidability-map.md) said replaces recursion: an iterative walk with an
explicit state, and the state is the loop variables.

### 5.3 It is sugar, and costs the reducer nothing

`match` desugars to `loop`. Each pattern becomes a test and a binding:

```lisp
(match (s)
  (ok v)  (use v)
  (err e) (report e))

;; ⟶
(loop ((x s))
  (== (tag x) 0)  (let (payload x) (fn (v) (use v)))
  else            (let (payload x) (fn (e) (report e))))
```

The pattern *bindings* become `let`s, and **`again` under a `let` already works** —
[ADR 0015](decisions/0015-loop-and-again.md) permits exactly that (*"`again` may be a clause body
or sit under a `let`, never under an `if`"*), a rule written for a different reason that turns out
to be the one this needs. Verified: a `loop` whose clause body is a `let` containing `again` reduces
and emits correctly today.

> **So `match` costs zero reduction rules and zero term kinds.** It joins `let`, `seq`, `and`, `or`,
> `not`, `cond` and `loop` as reader sugar that erases — which is the same answer that made
> booleans cheap ([ADR 0017](decisions/0017-booleans-are-in-the-language.md)).

`if` stays primitive. It is the eliminator of `bool = 1 + 1`, it is the third reduction rule, and it
is what `match` desugars *into*. Making it sugar over `match` would move a reduction rule for no
gain.

### 5.4 Flat patterns, and our data model wants them exactly

Nested patterns — `(ok (some (pair x y)))` — need real compilation: Maranget's decision trees (2008)
or Augustsson's backtracking automata (1985), plus Maranget's exhaustiveness algorithm (JFP 2007).
Real algorithms, well understood, and not free.

**Start flat, and the reason is not laziness.** The main use of deep patterns is deep *data*, and
§3.1 refused recursive data in favour of flat tables plus indices. **Our data is flat, so our
patterns can be flat**, and a flat match over a closed sum is a `switch`.

Nesting can arrive later without changing anything else, because the desugaring composes: a nested
pattern is a match inside a match.

### 5.5 It makes the refinement checker *stronger*, which is the argument I did not expect

`emit/refine.go`'s `clauses` walks an if-chain assuming each guard in its own branch **and the
negation of the earlier guards in the later ones** — *"the other half of Hoare logic"*, and the
thing that gives `i < len(a)` to a search's second clause.

A `match` clause carries **strictly more information** than a boolean guard:

- it tells you the **tag**, so the payload's type is known in that branch;
- and reaching a later clause means it is **none of the earlier tags** — which for a *closed* sum
  narrows to a finite remaining set, where a boolean chain only gives you a negated predicate.

So `_Success_(expr)`-style contracts ([general-purpose.md §4](general-purpose.md)) are discharged by
machinery that exists, and they are discharged *better* under `match` than under `if`.

### 5.6 What must be pinned down

- **What is a pattern**: a variable (binds, always matches), a literal (equality), a constructor with
  sub-patterns, `_` (wildcard). Guards — `(when c)` — probably, later.
- **Exhaustiveness**: `else` is mandatory unless the clauses cover the sum, which for a closed
  finite sum is a set-cover check on a small set.
- **Linearity**: a `(buffer V)` inside a matched value must be used at most once *per branch*, since
  branches are alternatives rather than sequence. `occurrences` computes it per branch already,
  which is the same walk `clauses` does.

---

## 6. What it looks like

```lisp
;; declarations
(sum (option T)   none (some T))
(sum (result T E) (ok T) (err E))
(sum token        lparen rparen (num f64) (ident int))

;; a product, anonymous
(sig divmod ((a int) (b int)) (× int int) (where (!= b 0)))

;; a state machine — the shape general purpose needs
(def tokenize (fn (src)
  (match ((build (len src) …) 0 top)
    b i (in-string)  (if (== (src i) 34) (again (emit b) (+ i 1) top)
                                         (again b (+ i 1) (in-string)))
    b i (top)        (again b (+ i 1) (classify (src i)))
    else             b)))

;; and a target contract, which is why this started
(prim VirtualAlloc ((size int)) (option ptr) expr "…"
  (where   (< 0 size))
  (ensures (=> (some? r) (writable r size)))
  (none-is 0))
```

---

## 7. Honest accounting

**Added:** two type formers (`×`, `+`) with `0` and `1`; a `sum` declaration; `match` as sugar;
tag/payload accessors as primitives the backends implement.

**Removed or subsumed:** `values`/multiple return becomes the product rather than a special case;
`case` never exists as a separate form; the negative-product story becomes "a product consumed
locally", which is a fact rather than a feature.

**Reducer cost: zero.** `match` is sugar; the product's β is β; the sum's β is case-of-case, which
is the one genuinely new rule and is
[sums-research §5.1](sums-research.md)'s open question rather than this document's.

**Type-checker cost: real but bounded.** Two type formers, exhaustiveness over closed finite sums,
and the tag-narrowing of §5.5. No inference, no unification, no subtyping — because staging
monomorphises.

**The risk, named:** this is the largest single addition to the language since `loop`, and it
arrives as *design* rather than as a response to a measured failure, which is the pattern
[ADR 0007](decisions/0007-exploration-over-specification.md) exists to distrust. The mitigation is
that all three motivating requirements are already documented and one is measured, and that the
first thing to build is the *smallest* piece — §8.

---

## 8. What I would build, in order

1. **The product**, redone properly as `×` rather than as `values` — and this time implemented on
   all four targets, because the last attempt was reverted precisely for declaring it optional.
   Java gets a generated record, windows gets a register convention.
2. **`(sum …)` closed, finite, non-recursive**, with the niche declaration, because that is what
   three requirements need.
3. **`match` as sugar over `loop`**, because it is free and it is what makes the sum usable.
4. **Case-of-case**, last, and only after measuring whether it changes any existing residual —
   [sums-research §9](sums-research.md).

And one thing not to do: **do not add `μ`, subtyping, intersections or untagged unions**, and when
one of them is next demanded, the reason it is refused is written down here.
