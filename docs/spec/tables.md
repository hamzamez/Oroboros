# Tables — the primary data structure

**Status: specification, not yet built.**

This is the language's data structure. There is one, and it is a **function with a known finite
domain**. Everything below follows from taking that literally — including the type system, where
it turns out to reorganise more than the data structure.

> **Second draft.** The first got three things wrong and hamza caught all three: it proposed a
> named indexing operation instead of application; it treated a target's representation choice as a
> *problem* rather than a measurement; and it refuted compile-time unrolling globally on evidence
> that only supports refuting it *conditionally*. All three are corrected below, and §5 — types are
> functions, and the domain is the whole story — is his and is the largest single idea in this
> document.

---

## 1. One concept, and it is a scale

### 1.1 Types are functions; the domain is what varies

A table is a function. So its type is a function type. The only thing that distinguishes an array
from a map from an ordinary function is **what the domain is**:

| written | means | domain |
|---|---|---|
| `(fn (A) B)` | an ordinary function | **all of A** |
| `(array V)` | an array of V | **`[0, len)`** — a finite prefix of the integers |
| `(map K V)` | a map from K to V | **the keys present** — a finite subset of K |

Three points on one scale, and the scale is the domain. `(array V)` is sugar for `(fn (int) V)`
restricted to `[0, len)`. `(map K V)` is sugar for `(fn (K) V)` restricted to the keys it holds.

Two things fall straight out:

**Bounds checking is a domain condition.** `(a i)` requires `0 ≤ i < (len a)` because that is
`i ∈ dom(a)`. It is not a special rule about arrays; it is what applying a partial function means.

**Go's comma-ok is the same condition.** `(m k)` requires `k ∈ dom(m)`, and Go's map returning a
zero value for an absent key is Go *totalising* the partial function by extending its domain with a
default. That is a design choice we can now name rather than inherit.

### 1.2 Types are sets, and this is the semantics we already had

`int`, `f64`, `string`, `bool` are sets. A function type is a function space. A table is an element
of a function space with a restricted domain. That is set-theoretic semantics — **TLA+ is built on
ZF and has exactly two primitives, sets and functions**, and Z is built on sets with relations and
functions on top.

This project has been writing that semantics without saying so: `aindex` already carries
`(where (and (<= 0 i) (< i (alen v))))`, which *is* the statement `i ∈ dom(v)`. The refinement
system is the domain calculus and nobody named it.

### 1.3 Two presentations of one function

A function can be written two ways, and the difference is memory, not meaning:

| | given by | memory |
|---|---|---|
| `(table n f)` | a **rule** — compute the element from the index | **none** |
| `(array e₀ … eₙ₋₁)` | a **graph** — the answers, written out | its own size |

The isomorphism between them is `tabulate`/`index` — Gibbons' **Naperian** functors, Dex's tables,
the container extension of Abbott/Altenkirch/Ghani. `lib/num/vec.oro` already wrote it:
`(vec n f) = (fn (sel) (sel n f))` is a length paired with an index function, which is
`Σ(n). Fin n → V` Church-encoded.

**The length is what makes a table more than a function.** A lambda has no domain bound; a table
does. That is the `Σ(n)`, it is why `len` is total on tables and undefined on lambdas, and it is
why `DOMAIN` is primitive in TLA+.

---

## 2. The surface

```lisp
(array e₀ e₁ … eₙ₋₁)      ; a graph. n is static.
(table n (fn (i) e))      ; a rule. n may be dynamic. NO MEMORY.
(alloc t)                 ; the same table, in memory. GATHER — pure, parallel by construction.
(build n (fn (b) …))      ; a scoped mutable buffer. SCATTER — sequential. Returns a table.
(set b i v)               ; a store. Consumes b, returns b.
(len t)                   ; the domain bound
(t i)                     ; the element at i — APPLICATION
```

Seven names. Everything else — `zip`, `sum`, `dot`, `reverse`, `take`, the stencil — is a library
written in the language, as it is today.

`build` and `set` are [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) and §9. The
pair is **gather and scatter**, which is the standard vector-programming distinction and is exactly
the boundary: `(table n f)` says element `i` is a function of `i`, and no such rule can express a
write at an index computed from the data.

### 2.1 `table`, not `vec` and not `vector`

`vector` is wrong, and `vec` inherits the same wrongness.

| | what it is |
|---|---|
| C++ `std::vector<T>` | growable, heap-allocated, contiguous, **owns memory**, mutable |
| Rust `Vec<T>` | the same |
| Go `[]T` | a **view** — pointer, len, cap — over a backing array; `append` may reallocate |
| Julia `Vector{T}` | mutable, growable |
| Scheme/Racket `vector` | fixed-length, **mutable** |
| Clojure `vector` | persistent, immutable, O(log₃₂ n) |

**Every mainstream `vector` is a mutable, growable, memory-owning container.** Ours is the exact
opposite: a *rule* with no runtime existence at all, which usually reduces to nothing. Naming it
`vec` tells a human — and an LLM, which has read far more Rust than Repa — to expect allocation,
`push`, and mutation, and every one of those is wrong.

Go's slice is worth a separate note because it is the closest thing and still not it. A slice is a
*view*: `(ptr, len, cap)`. It shares the allocation question we have not settled (§9) and it
carries `cap`, which is a growth policy we do not have. Our `alloc` produces something with a
length and no capacity, because there is no `append`.

`table` is right: it is what TLA+ means by a function with a domain, what Dex calls the values of
`n => a`, and what the maths in §1.3 describes. It carries no promise of allocation, growth or
mutation. `(table n f)` reads as *"a table of length n whose element i is …"*, which is exactly
what it is.

The literature's own name is **`tabulate`** — `tabulate : (Rep f → a) → f a` is half of the
Naperian isomorphism, and `index` is the other half. `tab` would be technically perfect and
opaque to everyone who has not read that paper. `table` says the same thing in a word people
already have.

### 2.2 `alloc`, not `materialize`

The first draft argued the name should be *long because it costs*. That is a real principle and it
is outweighed here.

**A name should say where the money goes, and be a word the reader already has.** `alloc` does
both in five characters. Every programmer and every model knows `alloc` means *this is where memory
happens*, which is the entire content of [construction.md](construction.md): materialize at a
boundary, doing it in the interior costs the 13× the stencil measured, **and that cost is the
point**.

`materialize` is a term of art — Repa's `computeS`/`Manifest`, a materialised view in a database,
Spark. It is precise for people who have that background and opaque otherwise, and it is eleven
characters that do not say *memory*.

Against `alloc`: it can suggest uninitialised storage. In a pure language there is no such thing —
`(alloc t)` obviously holds the elements of `t` — so the objection does not bite.

Rejected on the way: `freeze` (means *make immutable* in JS and Clojure, and ours already are),
`force` (a laziness term, opaque outside FP), `manifest` (Repa's word, and a noun).

**`build` is the scatter form** (§9), not a rejected name for this one. It reads as what it does —
build a table by writing into it — and the target file's `(build "go build %s")` is a *target
declaration* in a different grammar that no program can write, so the two never meet.

### 2.3 `len`

`len` on Go and Python, `.length` on JS, Java and Haskell, `size` on C++. `len` is the shortest
form of the most common word, it matches the existing `(length N)`/`(length-of N)` prim attributes
and the retired layer's `alen`/`slen`, and no target spells it anything surprising.

It is also the right *concept* rather than a convenience: `len` is the domain bound, so
`(len a)` and `dom(a) = [0, len a)` are one fact.

For a nested table `len` is the **leading-axis** length — `(len a)` is the number of rows and
`(len (a 0))` is the number of columns. That is Iverson's and More's leading-axis rule expressed by
currying, and it is why we need no separate `shape` primitive (§8.2).

---

## 3. Indexing is application

```lisp
(def a (array 1 2 3 4 5))
(a 0)                        ; 1
(a 2)                        ; 3
(+ 1 (a 1))                  ; → (+ 1 2) → 3   at compile time
```

**Not `(at a i)`.** The first draft proposed a named operation and it was wrong.

### 3.1 Why

**It is true.** If a table *is* a function from a finite domain, a named indexing operation is a
second spelling for something the language already has. Languages write `a[i]` because in those
languages an array is not a function. Here it is, and the distinction would be a lie.

**It unifies the two presentations at the call site.** With a named operation, a program says
`(vindex v i)` for a rule and `(at a i)` for a graph — the same operation twice, and every `alloc`
boundary rewrites the source. With application, `(v i)` and `(a i)` are the same text, so
**allocating changes where the memory is and nothing else.** That turns a refactor into a cost
decision, and it is the strongest single argument in this document.

**It costs no elimination form**, and it nests by currying: `(a i j)` is `((a i) j)`, so a
two-dimensional table needs no new mechanism.

### 3.2 The residual invariant makes it unambiguous

**In a residual, an application whose operator is a variable can only be a table.** A function
passed as an argument is substituted and its application reduces; a function that survives is an
escaping closure and is *refused* ([g6](../derivations/g6-escaping-closures.md)).

Checked rather than assumed: `emit/golang.go` and `emit/js.go` both error with *"no form for
primitive"* on any operator that is not a known primitive, and every program in `examples/` emits.
The slot is empty.

So `(a i)` with `a` a variable is an indexing **syntactically**, before any type is consulted.

### 3.3 The target's representation is a measurement, not a worry

The first draft called JavaScript's `a(0)` / `a[0]` distinction a *worry*. It is not, and the
framing was backwards.

Once the semantics are settled at the language level, **what to emit is a per-target measurement
governed by requirement 5**: as fast or faster than hand-written. If a JavaScript table of floats
is fastest as a `Float64Array`, emit that. If a table of strings is fastest as an `Array`, emit
that. If a map is fastest as a null-prototype object — and baseline R4 measured exactly that,
**3.25× faster than `Map`** — emit that. If some future measurement says an object beats an array
for a small numeric table, emit an object.

The language does not care, and the representation may differ *per element type and per size on
the same target*. That is the parasite model, and it is the opposite of a problem: it is the reason
the language does not name `[]float64`.

### 3.4 The diagnostic

`(x i)` where `x` is neither a function nor a table needs a message that tells the coder what they
did. Not *"no form for primitive x"* and not the first draft's *"x is not a table"* either:

```
in dot: `a` is applied to one argument here, but `a` is an f64.
  Applying a name means calling a function or indexing a table, and an f64 is neither.
```

and when the arity is wrong:

```
in grid: `g` is applied to three arguments, but `g` is a table of depth 2.
  `(g i j)` indexes it; the third argument has nothing to index.
```

---

## 4. Reduction: β gains a clause, and that is the easy path

### 4.1 What β-tab is

A function can be presented two ways, so application has two cases.

**β — a function given by a rule.** Substitute the argument into the body:

```
((fn (i) BODY) 2)   →   BODY[i := 2]
```

**β-tab — a function given by its graph.** Look the argument up:

```
((array 10 20 30) 2)   →   30
```

Same judgement — *apply this function to this argument* — for the two ways a function can be
written down. A rule is an **intensional** presentation (how to compute it); a graph is an
**extensional** one (what the answers are). Ordinary β handles rules. β-tab handles graphs. That is
all it is, and calling it "the extensional counterpart of β" is not a flourish: extensionality is
precisely *a function is determined by its input/output pairs*.

### 4.2 It is not a fourth rule

**β is the application rule.** It has one clause today because there is one way to write a
function down. Giving the language a second presentation gives β a second clause. The system still
has three rules: β (two clauses), δ, and structural evaluation on a literal.

`(len (array 1 2 3)) → 3` and `(len (table n f)) → n` join `if` under the third: a construct
evaluated when its argument is a literal it can decide. `if` was already exactly that, so the
generalisation from *"a conditional on a boolean literal"* to *"a construct decided by a literal"*
is a widening of a rule that existed, not a new one. **The count is unchanged and the granularity
change is stated rather than hidden.**

### 4.3 Constant folding is independent, and comes later

Building a general folder means a `(fold …)` declaration, per-target evaluators, and the
[ADR 0009](../decisions/0009-staging-preserves-results.md) hazard head-on: `(go.+ 1 2)` must fold
with *Go's* semantics, bit-identically. That is real work and it is independently wanted
([integers.md §13](integers.md)).

β-tab needs none of it. Looking up element `k` of a literal table is not arithmetic and cannot
disagree with anything at runtime. **Do β-tab now as a clause of β; build folding when integers
need it.** That is the easy one and it is also the correct one.

---

## 5. The type system

This is the part that reorganises more than the data structure.

### 5.1 One constructor: the function space

If a table is a function, and a map is a function, and a function is a function, then the type
language needs **one** constructor and a way to say what the domain is.

```lisp
(fn (A) B)        ; total on A
(array V)         ; ≡ (fn (int) V), domain [0, len)
(map K V)         ; ≡ (fn (K) V),   domain = the keys present
```

`array` and `map` are **sugar for a function type plus a domain**. They exist because the
restricted case is what you always mean, not because they are different kinds of thing.

The nested case is composition, with no new syntax:

```lisp
(array (array f64))          ; a table of tables — (fn (int) (fn (int) f64))
(map string (array f64))     ; a map to tables
```

### 5.2 The dependency lives in refinements — and that is Dependent ML

The obvious objection: `(array V)`'s domain is `[0, len a)`, and `len a` is a **term**. A type
mentioning a term is a dependent type, and full dependent types are rejected
([types-direction.md §6.8](../types-direction.md), on Low\*'s precedent).

They are not needed, because **the dependency is already somewhere else**. `aindex` today carries

```lisp
(where (and (<= 0 i) (< i (alen v))))
```

which is a *refinement*, not a type — a proposition in a decidable fragment, checked by
`emit/refine.go`, discharged at the call site, and erased. The type stays simple; the domain
condition rides in the refinement layer that already exists, already has an interval analysis, a
linear-arithmetic decision procedure and size-change termination behind it.

> **Simple types plus refinements carrying the arithmetic is exactly Dependent ML** (Xi & Pfenning,
> *Eliminating Array Bound Checking Through Dependent Types*, PLDI 1998) — indices from a decidable
> constraint domain, erased at run time, annotation only at function boundaries.
> [types-direction.md §6.5](../types-direction.md) already concluded DML is our lineage, from a
> measurement rather than from reading. This is that conclusion arriving at the data structure and
> fitting without adjustment.

### 5.3 What the checker actually sees

**A dynamic index forces homogeneity and forces existence, and it is the same condition doing
both.**

- Every index a literal: the elements need not share a type — `(a 0)` may be an `int` and `(a 1)` a
  `string`, because the checker knows *which* element it is looking at. And the table need not
  exist: every use folds.
- Any index dynamic: the elements must share a type, and the table must exist.

Reduction removes every static index. So **the checker, which runs on the residual, only ever sees
the homogeneous case.** The heterogeneous telescope is erased by staging, the same way the
polymorphism of a Church-encoded product is.

Consequence worth stating on its own: **a statically-indexed heterogeneous `(array x y)` is a
pair**, and `(a 0)`/`(a 1)` are its projections. The tuple is not a separate feature.

### 5.4 Type operators are cheap and should exist

hamza's sketch:

```lisp
(type map (fn (a b) (fn (a) b)))
```

A function at the type level. That is a **type operator** — ML's `type ('a,'b) map = …`, Haskell's
parameterised synonyms — and it is cheap because types are erased before the residual, so it is all
compile-time substitution. It is what lets `array` and `map` be *defined* rather than built in.

One guard rail: keep type operators **non-recursive**. A recursive type operator is a fixed point,
which is §1.2 of [data-structures.md](../data-structures.md)'s initial algebra, which
[ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) already removed. The restriction
is free — we already do not have the thing it would enable.

### 5.5 The domain type selects the host representation

This is the practical payoff, and it is representation selection of exactly the kind
[selection-2026-08-19](../../gauntlet/results/selection-2026-08-19.md) already built for integers.

| domain | host form |
|---|---|
| `int`, finite prefix, dense | the host's array — `[]float64`, `double[]`, `Float64Array` |
| an arbitrary type, finite subset | the host's map — `map[string]int`, `Object.create(null)` |

**The type says what it is; the target says what that costs.** And the element type's *spelling*
still comes from the target's own `(type f64 "float64")`, which is the target's identity rather
than a capability.

### 5.6 What this does not decide

**Dense versus sparse.** An integer-domain function whose domain is `{0, 1000000}` is better as a
map than an array, and the type does not say which. Today that is not a question any program asks;
when one does, the answer is probably a refinement on the domain, which is the same machinery
again.

**Mutable versus immutable.** §9.

---

## 6. Bounds are the domain

Out of range: Go **panics**, Java **throws**, JavaScript **silently returns `undefined`**, and x86
reads whatever is there. Four hosts, four answers, one of them silent.

So `(t i)` carries `i ∈ dom(t)`, discharged at every call site — which is not a special rule about
arrays but what applying a partial function means (§1.1). **In-bounds indexing is Tier 1.
Out-of-bounds is not a behaviour the language has**, the same shape as division by zero
([integers.md §5](integers.md)) where three hosts trap and JavaScript keeps going with `Infinity`.
An undischarged obligation is reported, never assumed.

This is strictly stronger than today, where `aindex`'s obligation is a `where` on a *target*
primitive and a target author can omit it. On a language construct it cannot be omitted.

---

## 7. The array literal, and unrolling — un-refuted and conditioned

### 7.1 What the measurement actually supports

[staticdata-2026-08-20](../../gauntlet/results/staticdata-2026-08-20.md) measured a 65,536-entry
table of `(i * 7919) % 65521` written as a literal against the same table built by a loop:

| target | literal | |
|---|---|---|
| x86-64 | `.rodata`, mapped from the image | free of code |
| Go | **no `main.init` at all**; +261,632 B for 262,144 B of table | free of code |
| Java | 256 `iastore` in `<clinit>` | pure loss |
| JavaScript | 4.28 ms vs 1.20 ms to import; 382 KB vs 144 B | pure loss |

and Go's startup saving was **0.2 ms against 9 ms of process creation** — not there.

hamza's objection is right, and the first draft's framing invited it. *"Bad on Java and JS"* is not
a reason to refuse anything: the rule is per-target, and if a literal is best on x86 and Go then it
should be a literal there and something else elsewhere.

**What the measurement supports is narrower and it is not about Java or JavaScript.** For a *cheap*
element, unrolling is not a win on any of the four, including Go. That is the honest claim, and the
first draft over-generalised it to *all* elements:

> *"the literal's parse cost grows with the table at least as fast as the compute cost it saves"*

**That is false.** If an element costs 1 ms to compute and there are 100 of them, the literal is
~1 KB and saves 100 ms. The win scales as **(compile-time cost per element) ÷ (artifact cost per
element)**, and the test only covered elements costing nanoseconds.

So unrolling is **not refuted. It is conditioned**, and the condition is a cost model the compiler
does not have.

### 7.2 And ADR 0009 bites exactly where the win would be

There is a second constraint, and it is sharper than the cost model.

To unroll, the compiler must **evaluate the elements at compile time**, and
[ADR 0009](../decisions/0009-staging-preserves-results.md) requires that to be bit-identical to
what the target would compute at run time.

- `+`, `-`, `*`, `/` and `sqrt` on binary64 are **exactly specified** by IEEE-754. Foldable.
- `sin`, `cos`, `log`, `exp`, `pow` are **not**. Every host's libm is free to differ in the last
  ulp, and they do.

**The expensive elements are mostly transcendental, and the transcendental ones are exactly the
ones that cannot be folded soundly.** The case where unrolling would pay is largely the case where
staging would change the answer.

Not entirely — an expensive *integer* computation (a CRC table, a permutation, a primality sieve)
is exactly specified and expensive enough to matter. So the door stays open, on that shape.

### 7.3 Where that leaves it

`(array e₀ … eₙ₋₁)` stays as a **source** form: the way a programmer writes data down, β-tab folds
its static indices, and it subsumes the tuple.

**Automatic unrolling is deferred, not refused**, with three conditions written down so the next
attempt does not re-derive them: the element must be foldable under ADR 0009; the compile-time
saving must exceed the artifact cost, which needs a cost model per target; and the emission is a
per-target choice, so x86 and Go may unroll where Java and JavaScript do not.

---

## 8. The array languages, and what they give us

### 8.1 Mullin's ψ-correspondence theorem is β

**A Mathematics of Arrays** (Mullin, 1988) gives `ρ` (shape), `ψ` (index), and a calculus of index
maps over `reshape`, `transpose`, `take`, `drop`, `rotate`, `catenate`. Its central result — the
**Psi Correspondence Theorem** — is that any composition of those operations reduces to a *single*
index computation. That is fusion, as an algebraic identity rather than a compiler pass.

In MoA you need that calculus to prove

```
(take k (reverse a)) ψ i  =  a ψ (n - 1 - i)
```

Here, `reverse` is

```lisp
(def reverse (fn (a) (table (len a) (fn (i) (a (- (- (len a) 1) i))))))
```

and `((reverse a) i)` **β-reduces** to `(a (- (- (len a) 1) i))`.

> **The Psi Correspondence Theorem is β.** Mullin needed a calculus because APL arrays are *data*
> and index maps have to be composed by hand. Ours are *functions*, so composing index maps is
> composing functions, and normalising the composition is normalising a term. The theorem is not
> something we implement; it is something our reducer already does.

That is the strongest reason to believe the design is right, and it is a third independent
rediscovery after containers and the pull/push duality.

### 8.2 The leading-axis rule is currying

Iverson's and More's leading-axis rule — an operation applies along the first axis and extends to
higher rank by mapping — falls out of `a : Fin n → Fin m → V` and `(a i j) = ((a i) j)`. An
operation on `(a i)` is a row operation, applied per row automatically.

So **`len` needs no companion `shape` primitive**: `(len a)` is the leading-axis length,
`(len (a 0))` the next. MoA's `ρ` returns the whole vector at once because its arrays are flat with
a separate shape; ours are nested, so the shape is read by indexing.

### 8.3 MoA's DNF/ONF split is our residual/emission split

MoA reduces an expression to a **denotational normal form** — a shape and an index function — and
then to an **operational normal form**: actual loops and memory operations. That is exactly
`cmd/oro`'s residual and `emit/`'s output, arrived at from a different direction. Worth knowing
because MoA's ONF stage is where its cost model lives, which is where ours lives too.

### 8.4 What the array languages give the library, and what we do not take

**Take:** `reverse`, `take`, `drop`, `rotate`, `stride`, `transpose`, outer product — every one is
an index map, every one is a library function returning a `table`, and every one is free because
the composition β-reduces. `reduce` is `loop`. Rank polymorphism for the leading-axis cases is
currying (§8.2).

**Do not take:** APL's implicit scalar extension and rank conformability rules, which are the part
of APL that people find unpredictable, and which require the shape algebra to be built into the
*semantics* rather than the library. **Remora** (Slepak, Shivers, Manolios, ESOP 2014) gives that a
static type system, and it is a much bigger language than this one.

**Where we differ from MoA on purpose:** MoA's world is **rectangular** — every row of a matrix has
the same length. Nested tables are not, so `(array (array 1 2) (array 3 4 5))` is expressible here
and is not an MoA array. That means **`transpose` is not total** and rectangularity is a refinement
if we ever want it, not an invariant. Ragged tables are free; the price is that shape-polymorphic
operations need a proof they do not need in MoA.

### 8.5 A correction to q5b

[q5b-filter.md §4](q5b-filter.md) tabulates `concat` as pull-impossible. It is not:

```lisp
(def concat (fn (a b)
  (table (+ (len a) (len b))
         (fn (i) (if (< i (len a)) (a i) (b (- i (len a))))))))
```

The length is computable from the two lengths, so the container morphism condition is satisfied and
the definition type-checks. What is true is the **cost**: this puts a branch in the inner loop,
where the push form emits two loops and no branch. So the row should read *expressible, with a
branch per element* rather than *no*.

`filter` and `flat-map` are genuinely impossible on a pull table, and for the stated reason: their
length depends on the *elements*, and a container morphism may only look at the shape. That half of
q5b stands, and it is the container-morphism theorem.

---

## 9. The memory model — decided

> **[ADR 0018](../decisions/0018-immutable-values-linear-buffers.md), 2026-08-21.** Values are
> immutable; mutation exists only inside `(build n (fn (b) …))`, whose buffer is **linear** and is
> frozen on the way out. `(array V)` reads are pure; `(buffer V)` reads are impure. The linearity
> check is `occurrences` on the residual, **not a type** — uniqueness never enters a signature.
>
> What decided it was **expressiveness, not the 2.7×**: `(table n f)` is a *gather* and cannot
> express a *scatter*, so the sieve, in-place sorting, histograms, union-find and general dynamic
> programming are inexpressible portably at any speed. `examples/native/sieve-go.oro` is in this
> repository and could not be written portably.
>
> It costs almost nothing to build because every mechanism already exists: the heap is acyclic
> (ADR 0014), a buffer cannot escape (closures are refused — the only thing Haskell's rank-2
> `runST` prevents), it is lexically local in the residual (whole-program reduction),
> `occurrences` is in the reducer, and **stores are already sequenced by ADR 0010** — an impure
> argument is never substituted, denying contraction, weakening and exchange, which are exactly the
> three properties a mutable buffer needs.
>
> The research, the candidates and the literature are in [memory-model.md](../memory-model.md).
> The rest of this section is the framing that led there and is kept because the reasoning is what
> changed.

## 9b. The three positions, as they were weighed

Whether tables are **immutable** is undecided, and it should stay undecided until it is decided
deliberately. It is the question behind reuse, and reuse is worth **2.5×–2.7×**
([native-gauntlet-2026-08-20](../../gauntlet/results/native-gauntlet-2026-08-20.md)) — a price
hand-written code pays too, so this is a semantics choice with a number attached, not a compiler
gap.

Three coherent positions, with the literature that shows each works:

**(a) Immutable, reuse recovered by the compiler.** Clean semantics; `alloc` always allocates and
an optimisation removes the ones it can. **Perceus** (Reinking, Xie, de Moura, Leijen, PLDI 2021)
is the existence proof — Lean 4 ships it, and functional array update runs in place when the
refcount is 1. Cost: an optimisation that silently does not fire, which is the failure mode
[bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already required a diagnostic for.

**(b) Mutable with uniqueness.** `(set t i v)` plus a proof that `t` is uniquely held. **Futhark**
is the closest existing language to this one — no recursion, arrays primary, size types — and it
chose exactly this. Clean and Mercury too. Cost: uniqueness is visible to the programmer in a way
`pure` is not, and [ADR 0013](../decisions/0013-accept-the-allocation-price.md) declined it once.

**(c) Mutable, target-native only** — where we are today. `go.set-float64` is at 0.999×, carries no
portability claim, and a portable program simply pays the allocating shape.

Three things that should feed the decision when it is made, all already measured or written:

- η-tab — `(alloc (table (len a) (fn (i) (a i)))) = a` — is **true of values and unsound with
  mutation**. It is a rewrite the compiler is entitled to and cannot take, and it is ADR 0013's
  fifth reopening trigger. **Choosing (a) makes it sound.**
- The 2.5–2.7× is the price of the *shape*, and hand-written code pays it. So (a) without a working
  Perceus is a 2.7× standing cost on array-producing code.
- [ADR 0010](../decisions/0010-effects-as-structural-rules.md) declined a substructural property on
  *values* for effects and got away with it because what needed the discipline was effects. (b)
  reverses that reasoning, which is why it needs its own ADR.

~~**Nothing in this document assumes an answer.**~~ **Answered above** — (a) with a scoped escape
hatch, which is (c)'s power made portable and checked. `alloc` still allocates; what changed is
that a *scatter* is now writable, and that η-tab became sound.

---

## 10. What each backend emits

No target declares any of this. `array`, `table`, `alloc`, `len` and indexing are implemented in
the backends, exactly like `if`, `let` and `loop` — the rule that a construct promoted to the
language works on every target and the compiler finds the implementation.

| | Go | Java | JavaScript | windows |
|---|---|---|---|---|
| `(array f64)` | `[]float64` | `double[]` | *(untyped)* | *(untyped)* |
| `(alloc t)` | `make([]float64, n)` + fill | `new double[n]` + fill | measured per element type | allocation + fill |
| `(a i)` | `a[i]` | `a[i]` | `a[i]` | `mov rax, [rbx+rcx*8]` |
| `(len a)` | `len(a)` | `a.length` | `a.length` | a stored length |
| `(map string int)` | `map[string]int` | `HashMap` | `Object.create(null)` — 3.25× over `Map` | *(no native form)* |

**This deletes surface.** Go currently names seven array types and declares four `at-*`, five
`make-*` and five `set-*`; Java names four and declares its own set; the retired portable layer had
`alen`/`aindex` for f64 only and `slen`/`sat` for strings, and never covered `int` or `bool` at all.
Those exist because the type language has no constructor, and the suffix explosion is that absence
showing through. One `(array V)` replaces all of it.

windows having no native map is the honest case: a program using one does not cover there, and
covering reports it. That is the capability graph doing its job for a *library* type, which `map`
is — `array` is the language's and windows gets memory and a length.

---

## 11. What a program looks like

```lisp
(export dot)
(sig dot ((a (array f64)) (b (array f64))) f64
  (where (== (len a) (len b))))

(def zip (fn (g x y) (table (len x) (fn (i) (g (x i) (y i))))))

(def sum (fn (v)
  (loop ((acc 0.0) (i 0))
    (>= i (len v))  acc
    else            (again (f+ acc (v i)) (+ i 1)))))

(def dot (fn (a b) (sum (zip f* a b))))
```

Against `examples/native/dot-go.oro`, the four-line `vec`/`vlen`/`vindex`/`of-array` preamble is
gone — `len` and application work on both presentations, so there is nothing to convert.

The stencil, with its one allocation where the source says so:

```lisp
(sig smooth ((a (array f64))) (array f64))
(def smooth (fn (a)
  (alloc (table (- (len a) 2)
                (fn (j) (f/ (f+ (f+ (a j) (a (+ j 1))) (a (+ j 2))) 3.0))))))
```

A table written down, its indices folded at compile time:

```lisp
(def shifts (array 0 3 6 9 12))
(def rotate (fn (x k) (<< x (shifts k))))     ; (shifts 2) → 6 where k is known
```

And a map, which is the same construct at a different domain:

```lisp
(sig tally ((text string)) (map string int))
```

---

## 12. Open questions

**Does `map` go in the language or stay target-native?** §5.1 makes it *expressible* as a function
type; that is not the same as deciding the language provides one. windows has no native map, so a
portable `map` means implementing a hash table for one target out of four — lowering further than
three of them require. [data-structures.md §4.4](../data-structures.md) argued it stays
target-native and iteration order (Go randomises deliberately, JS specifies, Java does not) makes
iteration Tier 2 regardless. **The type story does not change that answer; it just gives it a
notation.**

**Does the checker need to distinguish a rule-table from a graph-table?** Dex and Repa give them
different types, so you cannot accidentally index a rule in a loop and recompute it n times. Here
they share a type and `alloc` is explicit in the source. That is simpler and it is a deliberate
divergence; the failure mode is a rule-table indexed repeatedly with an expensive rule, which is
visible in the residual and could be a diagnostic.

**Dense versus sparse**, §5.6.

**And the memory model**, §9, which is the one that should be decided next and deliberately.

## 13. What to measure before building

1. **A portable `dot` and stencil against the current native ones**, on Go and JS. The bar is
   already recorded: 485 ns; 246,900 ns allocating; 97,939 ns reusing. Anything slower is a cost of
   the portable layer and must be named ([data-model.md §0](data-model.md)).
2. **`(a i)` against `(at a i)` on emitted output** — they should be identical, and if the type
   lattice cannot always place a local table, that is the finding.
3. **JavaScript's best table representation per element type and size** — `Array` against
   `Float64Array` against an object, which §3.3 says is a measurement and nobody has taken it for
   the small-table case.
4. **Whether `len` on a `table` ever survives to the residual.** It should always fold; if it does
   not, there is a case where the rule form is not free.

---

## 14. Five questions the design has to answer

Asked by hamza after ADR 0018. Three of them expose things the earlier sections did not say, and
one of them found a hole.

### 14.1 What is a closure, why do we not have one, and how much rides on it?

A **closure** is a function value that carries an environment: it has free variables from an
enclosing scope and it outlives that scope, so the captured values must be kept somewhere — a
heap-allocated environment record. `(fn (x) (+ x n))` is a closure exactly when it escapes the
scope that bound `n`.

**We do not have them by REFUSAL, not by theorem.** That distinction matters and the earlier
sections blurred it.

Reduction removes lambdas wherever it can: β substitutes, every non-exported function is inlined,
and higher-order code disappears. What is left is checked, and `emit/golang.go:281` (and the other
three backends) is the check:

```
case core.KFn:
    return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
        "  This is an escaping closure. …")
```

So the precise rule is:

> **A lambda is fine in a position a backend structurally consumes** — a `let` continuation, a
> `loop`'s body, a `table`'s rule, a `build`'s scope. **It is refused as a VALUE.**

The backends only ever read a `KFn` positionally: `t.Args()[1]` of a `let`, `t.Args()[0]` of a
`loop`. Anywhere else it is an error. Nothing is proven about programs in general; programs that
would need a closure are rejected.

**What it costs.** Higher-order *programming* works — `examples/native/generic-go.oro` passes `get`
and `combine` as parameters and it all reduces away. Higher-order *values* do not:

- an **exported** function cannot take or return a function, because its caller is hand-written
  host code that reduction cannot see;
- a function cannot be **stored** in a table, so there are no dispatch tables, no vtables, no
  strategy-pattern-as-data;
- a **host callback** cannot be passed, which is why `targets/js/methods.oro` declares `map`,
  `filter` and `reduce` and records that they cannot be called. That was measured and is the
  cheapest thing to lose: js-toplevel-2026-08-18 puts those methods at **3.6× to 133× slower** than
  a loop.

**What it buys.** No hidden heap allocation — the thing that killed the predecessor project. A
first-order residual, which is why the type checker is simple, why the interval analysis is
tractable, and why `(a i)` is unambiguous (§3.2). And ADR 0018's escape argument.

**And the refusal is currently BROADER than the design needs** —
[callbacks.md](callbacks.md). A top-level function with no free variables is a *function pointer*,
not a closure, and every one of the four targets can express one; it is refused today with the same
message. Most host callback APIs need only that, or need a lambda written at the call site where
the **host** does the capture. The refusal costs us function *values*, not host APIs.

**How much rides on it: enough that it must be a cross-reference.** ADR 0018 is sound *because* a
buffer cannot be captured. If closures are ever added, ADR 0018 must be revisited and the answer
would be Haskell's rank-2 `runST` type, which exists for exactly this and nothing else. That is
written into the ADR's triggers.

**And there is a second half to the argument that §6.1 did not state.** A lambda in a *structural*
position is allowed, so what stops this?

```lisp
(build n (fn (b) (table m (fn (i) (b i)))))     ; a rule capturing the buffer
```

The rule is a lambda in a structural position, and the table it makes would outlive the buffer.
Two things stop it, and both are needed:

1. **`build`'s continuation must return the buffer** — its type is
   `int → (buffer V → buffer V) → array V` — so a body whose value is a `table` is a type error.
2. **A lambda capturing the buffer cannot be stored or returned anywhere else**, because closures
   are refused as values.

Reading the buffer inside the scope is fine and useful — `(alloc (table m (fn (i) (b i))))` forces
the read *while `b` is alive*, and the read is impure so ADR 0010 sequences it against the stores.
That is the sieve checking whether `i` is already crossed off.

### 14.2 A function receives a table and returns one with a single element changed

This is the aggregate update problem (Hudak & Bloss 1985), and the honest answer is that it is
**O(n) in the language and O(1) only inside a `build`**.

**As a rule — free, and it is not a copy:**

```lisp
(def with (fn (a i v) (table (len a) (fn (j) (if (== j i) v (a j))))))
```

No memory at all. Reading it costs one compare per access. If it is consumed by a loop, the compare
fuses in and there is no table. **This is the right answer when the result is scanned once**, which
is most of the time.

**Forced — O(n):**

```lisp
(alloc (with a i v))     ; a full copy with one element different
```

**O(1), inside a scope:**

```lisp
(build (len a) (fn (b) (set (copy-from b a) i v)))
```

still O(n) because of the copy-in — but *repeated* updates batch:

```lisp
(build (len a) (fn (b)
  (loop ((b (copy-from b a)) (k 0))
    (>= k (len updates))  b
    else                  (again (set b (index-of updates k) (value-of updates k)) (+ k 1)))))
```

one copy, m stores. That is exactly Haskell: `arr // [(i,v)]` on `Data.Array` is O(n), `writeArray`
in `ST` is O(1), and you batch.

**What is genuinely missing** is O(1) update of a table a *caller* owns, which is M-B — uniqueness
on parameters — deferred in ADR 0018 with its trigger. Inside a program it is not missing, because
reduction removes the boundary and the table becomes a local buffer.

A compiler could also recover in-place when `a` is provably dead — that is M-A's analysis, still
available as an *optimisation*. ADR 0018 declines to **rely** on it, because SISAL's lesson is that
an optimisation with no source-level guarantee is a cliff you cannot see.

### 14.3 Growing a table — `append`

**There is no `append`, and a table does not grow.** `(len a)` is fixed, and `alloc` produces
something with a length and no capacity (§2.1).

`append` as a rule is free and O(1) to write:

```lisp
(def append (fn (a x) (table (+ (len a) 1) (fn (i) (if (< i (len a)) (a i) x)))))
```

but **repeated appends are O(n²)** if each is forced, and each nested `if` makes reads slower. That
is not a growable array; it is a linked list wearing one.

**The answer is the array-language answer: compute the size, then build.**

```lisp
;; filter, in two passes — count, then scatter. The parallel-array idiom.
(def filter (fn (p a)
  (let (count-matching p a) (fn (n)
    (build n (fn (b)
      (loop ((b b) (i 0) (k 0))
        (>= i (len a))  b
        (p (a i))       (again (set b k (a i)) (+ i 1) (+ k 1))
        else            (again b (+ i 1) k))))))))
```

Two passes over the input, one allocation of the exact size, no growth. This is how Futhark, ISPC
and every GPU library implement `filter` — count, prefix-sum, scatter — and it is the same shape
[q5b](q5b-filter.md) reached from the push/pull duality.

**What is not served: unbounded accumulation where the size cannot be computed** — reading input
until EOF, for instance. Three answers, none adopted here:

- **two passes**, when the source can be re-read;
- **build to a bound and slice** — `(build max …)` then `(slice b 0 k)`, which is free (§14.5);
- **a growable buffer**: `(push b x)` consuming and returning a possibly-larger buffer. This is
  **fully compatible with ADR 0018** — the buffer stays linear and scoped, and amortised doubling
  is exactly Go's `append`. All four targets can do it. It costs the buffer a *capacity* alongside
  its length.

The third is the natural extension and is deliberately not specified, because it should follow a
real program that needs it rather than precede one.

### 14.4 Copying a table without changing anything

```lisp
(table (len a) (fn (i) (a i)))
```

**That is η-tab, and it *is* `a`.** As a rule it is free and denotes the same function. Forced:

```lisp
(alloc (table (len a) (fn (i) (a i))))  =  a
```

is the η law of §3.1 — and **ADR 0018 is what made it sound**, because tables are immutable so
nothing can observe the difference between the copy and the original. The compiler may elide it.
Before ADR 0018 it emitted an allocation and a full copy loop, measured, and was right to.

So question 4's answer is literally the law the memory model was chosen to license.

**And the defensive copy does not exist here.** In Go or Java you copy a slice before handing it
out because the receiver might write to it. Nobody can write to a table, so there is nothing to
defend against — `DictCopy` in `gauntlet/go/aliasing.go`, which prices exactly that, has no
counterpart in this language.

### 14.5 Slicing

**Free, as an index map:**

```lisp
(def slice (fn (a lo hi) (table (- hi lo) (fn (i) (a (+ i lo))))))
```

and slices **compose by β**: `(slice (slice a 2 20) 3 7)` reduces to one addition, which is
Mullin's ψ-correspondence theorem doing its job (§8.1). Strides and reversal are the same shape:

```lisp
(def stride  (fn (a k) (table (/ (len a) k) (fn (i) (a (* i k))))))
(def reverse (fn (a)   (table (len a) (fn (i) (a (- (- (len a) 1) i))))))
```

**And immutability makes an O(1) *forced* slice safe.** `(alloc (slice a lo hi))` is a copy by the
general rule — but a slice of an already-allocated table can emit the host's own view:

| target | view | |
|---|---|---|
| Go | `a[lo:hi]` | **O(1)**, shares the backing array |
| JavaScript | `a.subarray(lo, hi)` on a typed array | **O(1)** |
| windows | pointer plus length | **O(1)** |
| Java | none for arrays | O(n) copy |

Sharing is safe **only because tables are immutable** — a view and its parent can never disagree,
and a buffer is never shared. That is a concrete performance property the memory model buys, and it
is a per-target emission choice of exactly the kind §3.3 describes.

A *strided* slice is not a view on any of the four, so it forces a copy. That is honest and it is
the same distinction Go draws.
