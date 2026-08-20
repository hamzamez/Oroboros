# Tables — the primary data structure

**Status: specification, not yet built.** Written first because
[strings.md](strings.md) exists as the correction for what happens otherwise.

This is the language's data structure. There is one, and it is a **function from a finite index
set**. Everything below follows from taking that literally.

---

## 0. What this settles, and the two things it does not

**Settles.** What a table is; how it is written, indexed and measured; the three ways to make one
and what each costs; why indexing is *application* rather than a named operation; how the element
type reaches four backends without a type constructor in the term language; and where the
allocation is.

**Does not settle.** **Reuse** — writing into a table you already have — which is
[ADR 0013](../decisions/0013-accept-the-allocation-price.md)'s open question and is not smuggled in
here (§7.3). And **`filter`**, which is not a table operation at all
([q5b](q5b-filter.md), and §1.4).

---

## 1. One concept

### 1.1 A table is a function with a length

```
Table n V  ≅  Fin n → V
```

An array *is* a function from `Fin n`; it is not merely *like* one. TLA+ defines a tuple as a
function with domain `1..n` and a record as a function with domain a set of strings. Containers
(Abbott, Altenkirch, Ghani) define the extension of a shape-and-positions pair as `Σ(s : S). P s →
A`. Gibbons' **Naperian** functors are exactly those isomorphic to `Log F → A`. Dex writes a table
as `n => a` and builds one with `for i:n. e`. SML defines `(a, b)` as `{1 = a, 2 = b}`.

Five traditions, one statement, and **this project already wrote it down**:
`lib/num/vec.oro` defines `(vec n f) = (fn (sel) (sel n f))` — a length paired with an index
function, which is the container extension Church-encoded.

The **length is what makes a table more than a function**. `(fn (i) e)` has no domain; a table
does. That is the `Σ(n)` in the container, and it is why `len` is total here and `DOMAIN` is
primitive in TLA+.

### 1.2 Two presentations of the same function

| | given by | memory |
|---|---|---|
| `(vec n f)` | a **rule** — compute the element from the index | **none** |
| `(array e₀ … eₙ₋₁)` | a **graph** — the elements, written down | its own size |

β for the first is ordinary β. β for the second is *selection*. They are the two ways a function
can be presented, and the isomorphism between them is `tabulate`/`index` — the Naperian
isomorphism, at the term level (§4).

### 1.3 What the language gets

**Three constructors, one measure, and application.**

```lisp
(array e₀ e₁ … eₙ₋₁)      ; a graph. n is static.
(vec n (fn (i) e))        ; a rule. n may be dynamic. NO MEMORY.
(materialize t)           ; a table in memory. THE ONE PLACE MEMORY APPEARS.
(len t)                   ; the length
(t i)                     ; the element at i — application
```

That is the whole surface. Everything else — `zip`, `sum`, `dot`, the stencil — is a library
written in the language, as it is today.

### 1.4 What is deliberately absent

**`filter`.** Not an oversight and not a limitation of the encoding: filtering changes the shape as
a function of the *elements*, and a container morphism may only look at the shape. It is not a
table operation, and [q5b](q5b-filter.md) already derived that the answer is a second
representation — the **push** collection — which is a library and stays one.

**A store.** See §7.3.

**Anything inductive.** [data-structures.md §1.2](../data-structures.md): a list's universal
property is `foldr`, [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) removed
recursion, so a cons chain here would be an array with worse constants and no compensating law.

---

## 2. Indexing is application

```lisp
(def a (array 1 2 3 4 5))
(a 0)                        ; 1
(a 2)                        ; 3
(+ 1 (a 1))                  ; → (+ 1 2) → 3   at compile time
```

**Not `(at a i)`.** The earlier draft of this design proposed a named operation and it was wrong.

### 2.1 Why application is right

**It is true.** If a table *is* a function from `Fin n`, a named indexing operation is a second
spelling for something the language already has. Every language that writes `a[i]` does so because
in that language an array is *not* a function; here it is, and the distinction would be a lie.

**It unifies the two presentations at the call site.** Today a program says `(vindex v i)` for a
delayed vector and would say `(at a i)` for a real one — the same operation, twice, and every
`materialize` boundary changes the *shape of the source*. With application, `(v i)` and `(a i)` are
the same text, so **materialising changes where the memory is and nothing else.** That is worth a
great deal: it is the difference between a cost decision and a rewrite.

**It costs no elimination form.** The reducer has application. β-tab is one entry in the constant
folder (§3.2).

**It nests by currying.** `(a i j)` is `((a i) j)`, so a two-dimensional table is a table of
tables and needs no new mechanism — Naperian functors compose, which is the whole reason Gibbons'
paper is about arrays.

### 2.2 Why it is unambiguous, which is the objection that has to be answered

The worry: on JavaScript `a(0)` and `a[0]` are different operations, and `targets/js/` declares no
types. So how does the backend know?

**Because in a residual, an application whose operator is a variable can only be a table.**

A function passed as an argument is substituted and its application reduces; a function that
survives is an escaping closure and is *refused* ([g6](../derivations/g6-escaping-closures.md)). So
after reduction, the operator of an application is a primitive, a structural name, or `again` —
and **never a variable**. Checked, not assumed: `emit/golang.go` and `emit/js.go` both error with
*"no Go form for primitive"* on any other operator, and every program in `examples/` emits
successfully. The slot is empty.

So `(a i)` with `a` a variable is **syntactically** an indexing, before any type is consulted. The
backend needs the element *type* to spell the Go declaration, but never to decide what the
operation is.

### 2.3 What it costs

A misapplied name is now an indexing rather than an unbound-name error, so the diagnostic must
change: applying something the signature does not give a table type is *"x is indexed here but is
not a table"*, not *"no form for primitive x"*.

---

## 3. The laws

### 3.1 The isomorphism

```
(β-tab)   ((array e₀ … eₙ₋₁) k)  =  e_k                  k a literal, 0 ≤ k < n
(η-tab)   (vec (len a) (fn (i) (a i)))  =  a
(len-a)   (len (array e₀ … eₙ₋₁))  =  n
(len-v)   (len (vec n f))  =  n
(β-vec)   ((vec n f) i)  =  (f i)
```

β-tab and η-tab say `array` and application are mutually inverse: **`Table n V ≅ Fin n → V`**,
which is the definition of a representable functor and the reason all five traditions in §1.1 agree.

`β-vec` is what makes fusion free and is *already* how the library behaves: indexing a rule-table
calls the rule, and the reducer inlines it.

### 3.2 β-tab is constant folding, not a fourth rule

The language has three reduction rules and that number is load-bearing. β-tab does not add one:

```
(go.+ 1 2)             → 3
((array 1 2 3) 1)      → 2
(len (array 1 2 3))    → 3
```

All three are *a primitive applied to known arguments has a known result*. Constant folding is
already listed as unbuilt ([integers.md §13](integers.md)); β-tab is one entry in its table. **If
folding is built first, tables cost the reducer nothing.** If it is not, β-tab is a fourth rule and
that must be counted honestly.

### 3.3 η-tab is a law we may state and may not apply

`(materialize (of-array a)) = a` is true of *values* and **unsound with mutation**: `materialize`
exists to produce a *fresh* table so nothing can alias it, and a program that materialises to get a
buffer it owns would begin mutating its input. Measured — the compiler emits an allocation and a
full copy loop and is right to.

Recorded as [ADR 0013](../decisions/0013-accept-the-allocation-price.md)'s fifth reopening trigger.
The law becomes applicable exactly when uniqueness becomes provable.

---

## 4. Static and dynamic indices — one condition, two consequences

Let `t` be a table in the residual and consider the indices at which it is applied.

**Every index a literal:**

- the elements need not share a type — `(a 0)` may be an `int` and `(a 1)` a `string`, because the
  checker knows *which* element it is looking at. The type is a telescope `Π(i : Fin n). Tᵢ`;
- and the table **need not exist**: every use folds.

**Any index dynamic:**

- the elements *must* share a type, because the checker cannot know which is read;
- and the table **must exist**.

> **A dynamic index forces homogeneity and forces existence, and it is the same condition doing
> both.**

Three consequences.

**It subsumes the tuple.** A statically-indexed heterogeneous `(array x y)` *is* a pair, and
`(a 0)`/`(a 1)` are its projections. This is why [data-structures.md §4.5](../data-structures.md)
concluded the language wants multiple *return*, not tuples: the stored pair is this, and it costs
nothing new.

**The dependent type is erased by staging.** The telescope exists only while indices are static,
and static indices are exactly what reduction eliminates. **The checker, which runs on the
residual, only ever sees the homogeneous `Fin n → V`.** No dependent types are needed. Same trick
as Low\*/F\* erasure and Cogent.

**And it is polarity again.** A statically-indexed table is `&` — an n-ary negative product whose
projections are chosen at compile time, never built. Dex draws this line in its *types*
(`n => a` is data, `n -> a` is code); here reduction draws it, and the residual is where it lands.

---

## 5. Types

### 5.1 `(array T)` in a signature, and nowhere else

```lisp
(sig dot ((a (array f64)) (b (array f64))) f64
  (where (== (len a) (len b))))

(sig grid ((g (array (array f64)))) f64)     ; nested, by composition
```

A type constructor in the **signature language**, not in the term language. It is read by the
checker and by `cmd/gen`, and erased before anything else. Nothing in the reducer, the residual, or
`core/` knows it exists.

### 5.2 No target declares it

Under the rule that a construct promoted to the language works on every target and the compiler
finds the implementation, **`array`, `vec`, `materialize`, `len` and indexing are implemented in
the backends**, exactly like `if`, `let` and `loop`. A target neither declines them nor declares
them, and there is no `(array-of "[]%s")` line to forget.

The element type's *spelling* still comes from the target's own `(type f64 "float64")`, because
that is the target's identity rather than a capability. The array construction around it is the
backend's:

| target | `(array f64)` | `(materialize (vec n f))` |
|---|---|---|
| Go | `[]float64` | `make([]float64, n)` + a fill loop |
| Java | `double[]` | `new double[n]` + a fill loop |
| JavaScript | *(untyped)* | `new Float64Array(n)` or `new Array(n)` + a fill loop |
| windows | *(untyped)* | an allocation and a fill loop |

**This deletes surface rather than adding it.** Go currently names seven array types and declares
`at-int`, `at-float64`, `at-string`, `at-bool`, five `make-*` and five `set-*`. Java names four and
declares its own set. Those exist because our type table has no constructors, and the suffix
explosion is that table showing through. One `(array T)` replaces all of it.

### 5.3 Element types on an untyped target

`targets/js/` declares every argument `any` and needs no element type at all — which is the point
of §5.2 rather than a problem with it. The Go and Java backends need the element type; they get it
from the signature at a boundary and from the residual in the interior, where reduction has already
made every value monomorphic. `emit/golang.go` already has the lattice that does this; it is
currently hardcoded to `vec-f64` for `make-vec`.

---

## 6. Bounds are a precondition, not a behaviour

Out of range: Go **panics**, Java **throws**, JavaScript **silently returns `undefined`**, and x86
reads whatever is there. Four hosts, four answers, and one of them is silent.

So indexing carries an obligation, discharged at every call site:

```
(t i)   requires   (and (<= 0 i) (< i (len t)))
```

exactly as `aindex` does today ([refinements.md](refinements.md)), and the machinery already
exists: the interval analysis, the linear-arithmetic decision procedure, `(length N)` /
`(length-of N)`, and the loop-bound facts. On the sieve this is *proven*, not propagated.

**In-bounds indexing is Tier 1. Out-of-bounds is not a behaviour the language has** — it is the
same shape as division by zero ([integers.md §5](integers.md)), where three hosts trap and
JavaScript keeps going with `Infinity`. An undischarged obligation is **reported, never assumed**.

This is a strictly stronger position than today's, because today `aindex`'s obligation is a
declared `where` on a target primitive and can be omitted by a target author. As a language
construct it cannot.

---

## 7. Memory

### 7.1 Three forms, three costs

| form | where it lives | creation | selection |
|---|---|---|---|
| `(vec n f)`, fully reduced | nowhere | 0 | the rule, inlined |
| `(array c₀ … cₙ₋₁)` surviving | the artifact | 0 at run time; its size in the binary | one load |
| `(materialize t)` | the heap | 1 allocation + n stores | one load |

**`materialize` is the only construct in the language that allocates.** That is the property worth
having and it is why the name is long: [construction.md](construction.md) says *materialize at a
boundary*, doing it in the interior costs the 13× the stencil measured, and **that cost is the
point**. A short name would hide it. The name is as long as its price.

### 7.2 What was refuted, so it is not re-proposed

Automatically turning `(vec n f)` with a literal `n` into `(array c₀ … cₙ₋₁)` — compile-time
materialisation into static data — is **measured and dead**
([staticdata-2026-08-20](../../gauntlet/results/staticdata-2026-08-20.md)): free of code on x86 and
Go, a pure loss on Java (256 `iastore` in `<clinit>`) and JavaScript (3.5× slower to load, 2,600×
larger source), and never a measurable win. It also would have needed a binary-size budget, which
is a heuristic this project has avoided everywhere else.

`(array …)` stays as a **source** form — a way to write data down — and nothing unrolls into one.

### 7.3 Reuse is not here, and that is ADR 0013 unchanged

A portable program can allocate and fill. It cannot write into a table it already has. So a
portable stencil pays the allocating shape, which
[native-gauntlet-2026-08-20](../../gauntlet/results/native-gauntlet-2026-08-20.md) measured at
**2.71× for hand-written code and 2.52× for emitted code** — the price of the *shape*, which
hand-written code pays too.

On a native target a program can decline to pay: `go.set-float64` is Go's own store, no portability
claim, measured at **0.999×** against hand-written. That is the parasite model working as designed
— the portable layer names its price and a program that cannot pay drops one layer.

**Putting a store in the language means putting uniqueness in the language**, which ADR 0013
declined and which [ADR 0010](../decisions/0010-effects-as-structural-rules.md) declined for
effects on related grounds. It is a real decision with its own ADR, and specifying it inside a
data-structure document would be exactly the accident
[assessment-2026-08-20 §2](../assessment-2026-08-20.md) warns about.

---

## 8. What a program looks like

`dot`, portable, one file, replacing two target-specific ones:

```lisp
(export dot)
(sig dot ((a (array f64)) (b (array f64))) f64
  (where (== (len a) (len b))))

(def zip (fn (g x y) (vec (len x) (fn (i) (g (x i) (y i))))))

(def sum (fn (v)
  (loop ((acc 0.0) (i 0))
    (>= i (len v))  acc
    else            (again (f+ acc (v i)) (+ i 1)))))

(def dot (fn (a b) (sum (zip f* a b))))
```

Note what is gone against `examples/native/dot-go.oro`: `vec`/`vlen`/`vindex`/`of-array` are no
longer a four-line preamble, because `len` and application work on both presentations. `of-array`
in particular disappears entirely — there is nothing to convert.

The stencil:

```lisp
(sig smooth ((a (array f64))) (array f64))
(def smooth (fn (a)
  (materialize
    (vec (- (len a) 2)
         (fn (j) (f/ (f+ (f+ (a j) (a (+ j 1))) (a (+ j 2))) 3.0))))))
```

One allocation, at the one place the source says so.

And a table written down, with its indices folded at compile time:

```lisp
(def shifts (array 0 3 6 9 12))
(def rotate (fn (x k) (<< x (shifts k))))     ; (shifts 2) → 6 when k is known
```

---

## 9. What this replaces

| | today | after |
|---|---|---|
| Go target | 7 array types, 4 `at-*`, 5 `make-*`, 5 `set-*` | the element types it already names |
| Java target | 4 array types plus its own set | likewise |
| JS target | `at`, `set`, `Array`, `Float64Array`, `Int32Array` | likewise |
| portable layer | `alen`/`aindex` (f64 only), `slen`/`sat` (string only), `make-vec` | — |
| library | `vec`, `vlen`, `vindex`, `of-array`, `materialize` | `zip` and friends only |

The portable layer never solved this: it **enumerated**, giving two element types two different
names for the same operation, and never covering int or bool at all. `aindex` and `sat` being
different names for the same shape is not a quirk — it is the missing type constructor showing
through.

---

## 10. Open questions

Named rather than decided, because each would change the design.

**Does `vec` need to be in the language, or does it stay a library?** It reduces away completely,
so nothing at run time depends on it. Putting it in buys **one `len` and one application syntax**
across both presentations, which is §2.1's main argument. Leaving it out means the library keeps
`vlen`/`vindex` and a program's shape changes at every `materialize` boundary. *Recommendation: in
the language, and this is the least certain call in the document.*

**Is `(array …)` heterogeneous, or is that a separate form?** §4 says a static index makes
heterogeneity free, which makes `array` the tuple as well. That is elegant and it means one form
carries two ideas — and if the tuple case ever wants labels, it wants a different index set and
that is a different form anyway.

**Constant folding first, or β-tab as a fourth rule?** §3.2. Folding is independently wanted and
makes this free; building it first is probably right, and it is the one ordering question here.

**Does `materialize` keep its name?** It is long, and §7.1 argues the length is the point. The
alternative is `freeze`, which says what happens rather than what it costs.

**And what does `len` do to the shape algebra?** `(length N)`/`(length-of N)` are the first two
entries in a table Mullin's *A Mathematics of Arrays* writes out in full — `len (zip f a b)`,
`len (vec n f)`, `len (materialize t)`. Making `len` a language construct means those become
compiler knowledge rather than target declarations, and the interval analysis already consumes
exactly that kind of fact.

## 11. What to measure before building

1. **`(a i)` against `(at a i)` on emitted output.** They should be identical; if the type lattice
   cannot always place a local table, that is the finding and it decides §2.2.
2. **A portable `dot` and stencil against the current native ones**, on Go and JS. The bar is the
   numbers already recorded: 485 ns, 246,900 ns allocating, 97,939 ns reusing. Anything slower
   means the portable layer costs something, and naming what is [§0 of
   data-model.md](data-model.md).
3. **Whether `len` on a `vec` survives to the residual anywhere.** It should always fold; if it
   does not, there is a case where the delayed form is not free.
