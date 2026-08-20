# Data structures: the research

**Status: research, not a decision.** No ADR follows from this document by itself. It exists to
say what is known, what the literature settled long ago, what this project has already
rediscovered without naming it, and which candidates are worth measuring.

The question as asked: *do we make the primary data structure an array because we do not want to
pay the price of a list? Is there a better option? What is the algebra of the candidates, and can
we reason about them? Do they fit the language, the reduction, the type system? Do we need
map/dict, tuples, structs?*

---

## 0. Three things this project already knows, and two it rediscovered

Before reaching outward, what is already on the ground here.

**The product is measured affordable, and the measurement came with its own condition**
([product-2026-08-19](../gauntlet/results/product-2026-08-19.md)): Go 1.01× with zero allocations
including the explicit-pointer control, Java's `record` scalar-replaced by C2, V8's object literal
1.11× and its *array* 1.32×, x86 free by construction. And:

> **A product is free where our reducer already makes things free — when it is built and taken
> apart in the same place. It costs an allocation exactly where `materialize` costs one.**

**Polarity says which products those are, in advance**
([types-direction §6.3](types-direction.md)): negative types are eliminated by *projection* and
need never be built; positive types are eliminated by *pattern matching* and must exist.

**And two results in this repository are famous results under other names.**

`lib/num/vec.oro` defines `(vec n f) = (fn (sel) (sel n f))` with `f : Fin n → A`. That is the
**extension of a container** in the sense of Abbott, Altenkirch and Ghani — `⟦S ▷ P⟧ A = Σ(s : S).
P s → A` — Church-encoded. It is also a **pull array**, and `materialize` is Repa's `Manifest`.
The delayed/manifest split, arrived at from cost measurements, is the split Repa arrived at from
fusion.

[q5b-filter.md](spec/q5b-filter.md) derived that pull arrays cannot express `filter`, that a
**push** collection `(fn (step z) …)` can, that push cannot express `zip`, that pull→push is free
and push→pull must materialise, and that stream fusion would unify them at the cost of
case-of-case. That is, line for line, the Obsidian result (Claessen, Sheeran and Svensson, DAMP
2012), and the second half is GHC's own experience with stream fusion versus foldr/build.

Neither was read off a paper. Both being independently derived is evidence the constraints here
are the same constraints that produced them, which is a reason to take the rest of those
literatures seriously rather than to feel clever.

---

## 1. The mathematics: what a data structure *is*

### 1.1 Universal properties, and why they are the right frame

A product `A × B` is not "two things next to each other". It is the object with projections `π₁`,
`π₂` such that any pair of maps `f : C → A`, `g : C → B` factors uniquely as `⟨f,g⟩ : C → A × B`.
A coproduct is the dual. An exponential `B^A` is the object with `eval : B^A × A → B` and the
currying isomorphism.

Why this matters practically: **the universal property tells you the eliminator, and the eliminator
tells you the cost.** A product's eliminator is projection — you can always *not* build the thing
and answer the projection directly. A coproduct's eliminator is case analysis — you cannot answer
"which side?" without the tag existing somewhere. That is the whole of §1.4 in advance.

The laws are worth writing down because they license compiler moves:

```
π₁ ∘ ⟨f,g⟩ = f              ⟨π₁∘h, π₂∘h⟩ = h            (β and η for products)
A × 1 ≅ A     A × B ≅ B × A     (A × B) × C ≅ A × (B × C)
[f,g] ∘ ι₁ = f              A + 0 ≅ A                   (β for sums)
A × (B + C) ≅ (A × B) + (A × C)                         (distributivity)
```

Two consequences for us. First, associativity and commutativity hold only **up to isomorphism**,
so a **flat n-ary product is as legitimate as nested pairs** and avoids the re-association work —
which matters, because a nested pair encoding would make `divmod` and a three-field record
different shapes to the reducer. Second, the β law for products *is* β for lambdas under the
Church encoding, which is why the reducer already implements it.

### 1.2 Initial algebras — and why we cannot have them

An algebraic data type is the **initial algebra** of a polynomial functor (Lambek; Malcolm 1990;
Meijer, Fokkinga and Paterson 1991; Bird and de Moor 1997). A list is `μX. 1 + A × X`. A binary
tree is `μX. 1 + X × A × X`.

The *point* of an initial algebra is its unique homomorphism out — the **catamorphism**, `foldr`.
That is what "list" means mathematically: the free monoid on `A`, whose universal property is that
any monoid homomorphism out of it is determined by where the generators go.

> **[ADR 0014](decisions/0014-recursion-is-not-in-the-language.md) removed recursion. Therefore
> `foldr` is not writable. Therefore the list's universal property is inaccessible, and what is
> left of a list is an array with worse constants.**

This is the real answer to the question that opened this document, and it is stronger than the
cost argument. A cons list in this language would not be a slow list. It would not be a list at
all — it would be a linked structure with no fold, which is a data structure with no algebra.

The same argument kills trees, ropes, finger trees, and every other inductive type, and it kills
them for a reason that has nothing to do with allocation.

Dually, **final coalgebras** — streams, infinite structures — need corecursion, and `loop` is
bounded and now provably terminating ([sct-2026-08-19](../gauntlet/results/sct-2026-08-19.md)). So
codata is out on the same grounds.

**What is left is exactly:** finite products, finite sums, exponentials, and one family of
structures indexed by a runtime-sized finite set. In categorical terms, a **bicartesian closed
category plus a Π over `Fin n`**. That is a small and completely respectable universe, and it is
worth stating positively rather than as a list of absences.

### 1.3 Containers and Naperian functors: the algebra of the array

A **container** (Abbott, Altenkirch, Ghani, TCS 2005) is a pair of a shape set `S` and a position
family `P : S → Set`, with extension

```
⟦S ▷ P⟧ A  =  Σ(s : S). P s → A
```

Containers are closed under product, coproduct, composition and (for the strictly positive ones)
fixed points. Their morphisms are exactly the natural transformations between the extensions, so
"the operations you can write generically on a container" is a *theorem*, not a design choice.

A functor is **Naperian** (Gibbons, ESOP 2017) — equivalently *representable* — when
`F A ≅ Log F → A` for some fixed `Log F`. Arrays of known shape are Naperian with `Log = Fin n`.
Naperian functors are closed under product and composition, which is what makes multi-dimensional
arrays fall out of nesting, and they come with `zipWith` and `transpose` for free.

Our delayed vector is `Σ(n : ℕ). Fin n → A`: a container whose shape is a natural number. Its
`zip` works because for a fixed shape the functor is Naperian, and its `filter` does not because
filtering is not a container morphism — it changes the shape as a function of the *elements*, and
a container morphism may only look at the shape.

> **q5b's "pull arrays genuinely cannot express filter" is the container-morphism theorem.** It was
> not a limitation of the encoding, and no amount of cleverness in the encoding would have removed
> it.

That is worth knowing, because it means the push/pull pair is not a workaround to be replaced
later by a smarter representation. It is the shape of the problem.

**Derivatives** (Huet's zipper, 1997; McBride 2001) are the other half of the container story: `∂F`
is the type of one-hole contexts, and `∂(Fin n → A) ≅ Fin n × (Fin n → A)` — an index plus the
array — which is the "update at position i" operation. This is where in-place update *would* come
from as a type, and it is one route to ADR 0013's open question, though not the cheapest.

### 1.4 Polarity: which structures must exist at runtime

Girard's linear logic already separates the two products that classical logic conflates:

| | connective | introduction | elimination | must the value exist? |
|---|---|---|---|---|
| **positive** | `A ⊗ B`, `A ⊕ B`, inductive data | build it | pattern match | **yes** |
| **negative** | `A & B`, `A → B`, records, codata | give a way to answer each observation | project | **no** |

Andreoli's focusing (1992), Zeilberger (2008), Levy's **call-by-push-value** (1999) and
Munch-Maccagnoni all reach the same classification from different directions.

`A & B` is the negative product: introduced by a pair of derivations *sharing* a context,
eliminated by `π₁` or `π₂`. It is implemented by waiting to see which projection is demanded — so
it never needs to be built. `A ⊗ B` splits its context and is taken apart by matching, so both
halves are demanded and the pair must exist.

**Our Church-encoded pair is `&`.** `(fn (sel) (sel x y))` is precisely "give me the observation
and I will answer it".

The 1.01× measurement is this fact showing up on a clock, and the caveat attached to that
measurement — *a product that escapes will allocate* — is also predicted:

> **A negative product is free exactly where reduction can see both the introduction and the
> elimination. Where it crosses a runtime boundary — a loop back edge, a call the host declines to
> inline, a target ABI — it must be reified, and from that point you are paying for a positive
> product.**

The project has hit this boundary three times already and named it three different things:
`fold-range2`'s **finisher** existed so that compound loop state never escaped the loop
([structs-2026-08-14](../gauntlet/results/structs-2026-08-14.md)); `materialize` is the same
boundary for arrays ([construction.md §6](spec/construction.md)); and ADR 0013's accepted price is
that boundary for the stencil. **They are one boundary.**

---

## 2. Specification languages: where "one structure" was already tried

Specification languages are worth reading here precisely because they are not constrained by cost.
They show what people choose when only the *reasoning* matters — which is the upper bound on what
an algebra can give us.

### TLA+ — everything is a function

Lamport's *Specifying Systems* has two primitives: **sets** and **functions**. Everything else is
notation.

```
[S -> T]                    the set of functions from S to T
DOMAIN f                    the domain
[f EXCEPT ![x] = e]         functional update
<<a, b, c>>                 a tuple: a function with domain 1..3
[x |-> 1, y |-> 2]          a record: a function with domain {"x", "y"}
Seq(S), Len(s), s \o t      sequences: functions with domain 1..Len
```

A record *is* a function from a finite set of strings. A tuple *is* a function from `1..n`. A
sequence *is* a tuple. There is no separate tuple type, record type, array type or list type — one
concept, four notations.

This is the same statement as §1.3 from a different tradition, and it is 25 years older than
containers as a design rather than a theorem. It also tells us what the reasoning looks like:
**extensionality** (two functions are equal iff same domain and pointwise equal) and `EXCEPT` are
the entire calculus.

### Alloy — everything is a relation

Jackson's *Software Abstractions* goes one step further: every value is a **relation** of some
arity, a scalar is a singleton unary relation, and the dot operator is relational join —
generalising field access, function application, and image-of-a-set into one operator. Field
access is `p.x`, function application is `f[i]`, and both are join.

The lesson for us is the *cost* of that unification: Alloy is decidable only within finite scopes,
and analysis is by SAT over bounded universes. Total unification of data structures buys enormous
uniformity in the logic and pays for it in the decision procedure.

### Z, VDM, B, Dafny

Z (Spivey) builds everything from sets: relations are sets of pairs, functions are functional
relations, and the schema calculus is the composition mechanism. VDM-SL keeps `set`, `seq`, `map`
and record as separate primitives with separate laws — the pragmatic opposite. B and Event-B
(Abrial) follow Z.

**Dafny is the most instructive**, because it draws exactly the line this project cares about:
`seq<T>`, `set<T>`, `map<K,V>` are *mathematical* values with clean laws for the verifier, and
`array<T>` is a separate, mutable, heap-allocated thing that requires framing and modifies-clauses
to reason about. **The mathematical sequence and the machine array are different types on
purpose.** That is our delayed/manifest split appearing in a verifier, and for the same reason: the
laws hold of the first and the performance belongs to the second.

Lean 4 is the counter-example worth knowing: it presents `Array` as a functional value and uses
**reference counting with in-place reuse** (Perceus — Reinking, Xie, de Moura, Leijen, PLDI 2021)
to make functional update run in place when the refcount is 1. That is ADR 0013's option (b) with
an implementation and a shipping compiler behind it.

---

## 3. Programming languages, with the price attached

### 3.1 The cons cell

McCarthy's pair (1960) is two words and a pointer chase per element, and it is the reason "list" is
the default word for "sequence" in three generations of functional languages. Its costs are not
controversial: no random access, poor locality, an allocation per element, and — on a modern
machine — a dependent load chain that defeats prefetch and out-of-order execution.

This repository has the *benchmark* in a related shape and has never recorded the number:
`gauntlet/go/cycles.go` carries `SumPointerGraph` against `SumIndexGraph` — pointer chasing versus
indexing over the same data — and no file in `gauntlet/results/` cites it. So the cost argument
against cons cells is, here, still an argument. It is cheap to close and it is on the list in §7.

But per §1.2, cost is the *second* argument. The first is that the list's algebra is unavailable.

### 3.2 The array languages

**APL** (Iverson 1962; *Notation as a Tool of Thought*, 1980) made the array the only structure and
got an algebra out of it: rank, frames and cells, the leading-axis rule, and scan/reduce/outer
product as first-class combinators. **J** and **K** continue it; kdb+ makes a table a dictionary of
columns, i.e. **struct-of-arrays as the data model**, which is exactly what g2 measured JavaScript
needing.

**A Mathematics of Arrays** (Mullin 1988) is the formal version: the ψ-calculus gives an algebra of
shapes and index maps in which `reshape`, `transpose`, `take`, `drop` and indexing all compose, and
the Psi Correspondence Theorem says a composition of these can be rewritten into a single index
computation — **fusion, as an algebraic identity on index maps rather than as a compiler pass**.

**Remora** (Slepak, Shivers, Manolios, ESOP 2014) gives rank polymorphism a static type system.
**SaC** compiles shape-generic array code to C. **Futhark** (Henriksen et al., PLDI 2017) is the
closest existing language to this project's constraints and deserves its own line:

> Futhark has **no recursion**, arrays as the primary structure, **size types** so that lengths
> appear in signatures, and **uniqueness types** for in-place update — and it compiles to GPU code
> that competes with hand-written kernels.

Every one of those four is a decision this project has made or is circling. Futhark chose
uniqueness types where ADR 0013 declined to, and that is the single most useful comparison
available.

**Dex** (Maclaurin, Radul, Johnson, Duvenaud) makes the Naperian view the surface syntax: `n => a`
is a *table*, a function from an index set, and `for i:n. e` builds one. Index sets are types.
Records and arrays are the same construct at different index sets — the unification of §1.3 and §2
as a working language.

### 3.3 Pull, push, and where to materialise

**Repa** (Keller, Chakravarty, Leshchinskiy, Peyton Jones, Lippmeier, ICFP 2010) distinguishes
`Delayed` (a function from index to element) from `Manifest` (a real array), with an explicit
`computeS`/`force` between them. **Obsidian** (Claessen, Sheeran, Svensson, DAMP 2012) adds the
**push** array and shows the duality that q5b rederived. **Accelerate** does delayed/manifest with
a fusion pass. **Stream fusion** (Coutts, Leshchinskiy, Stewart, ICFP 2007) unifies them with a
`Skip` step, and **foldr/build** (Gill, Launchbury, Peyton Jones, FPCA 1993) is the older, less
general, more predictable answer.

**Halide** (Ragan-Kelley et al., PLDI 2013) is the one to take seriously for a different reason: it
separates the *algorithm* from the *schedule*, and `compute_at` / `store_at` make **where to
materialise** an explicit, first-class decision the programmer writes down. That is precisely what
`materialize` is, and Halide is the existence proof that making it explicit is a usable interface
rather than a wart — with the caveat that Halide schedules are famously hard to write, which is an
argument for having exactly one knob rather than Halide's dozen.

### 3.4 Records and rows

SML defines a **tuple as a record with numeric labels** — `(a, b)` is literally `{1 = a, 2 = b}`.
The unification of §2 shows up in the definition of a 1990 standard.

Row polymorphism is the theory of extensible records: Wand (1987), Rémy (1989), Gaster and Jones
(1996), Leijen's *Extensible records with scoped labels* (2005). It gives `{x : Int | r}` — "a
record with an `x` field and whatever else" — and is what makes structural record types compose.

Haskell's record system is the cautionary tale: field selectors are partial functions when a type
has several constructors, names live in the module namespace, and the whole thing has been patched
for thirty years. The lesson is small and concrete: **a record is a product with named
projections, and the names must be scoped to the record, not to the module.**

### 3.5 Multiple values are not a data structure — and that is deliberate

Common Lisp (1984) and Scheme (R7RS) both have `values` and `multiple-value-bind` /
`call-with-values`, and both specify them as **not** constructing an object. The reason given at
the time is the reason that still holds: an implementation should be able to return several results
in registers, and wrapping them in a heap object to immediately take it apart is pure loss. Lua
does the same. Go's `(int, error)` is the same idea with types.

In CPS the reason is obvious: **returning two values is calling the continuation with two
arguments.** `f : A → (B → C → R) → R`. That is the negative product of §1.4, and it is
`(fn (sel) (sel x y))`.

What each of our four targets provides natively:

| target | multiple return | measured |
|---|---|---|
| **Go** | two return values, in registers | 1.01×, 0 allocs |
| **Java** | none; a `record` is the idiom | 0.96×, C2 scalar-replaces |
| **JavaScript** | none; object literal or array | object **1.11×**, array **1.32×** |
| **x86-64** | `rax`:`rdx`, and the SysV/Win64 ABIs say so | free by construction |

Three of the four have a native form. That is a capability-graph situation, not a language-design
situation.

### 3.6 Persistent structures, and why they are out

Okasaki's *Purely Functional Data Structures* (1998) gives numerical representations, random-access
lists, and the amortisation arguments that make them work. Bagwell's hash-array-mapped tries became
Clojure's persistent vector; Bagwell and Rompf's RRB-trees (2011) add concatenation.

These are excellent and they are **out on the parasite rule**. None of Go, JavaScript, the JVM or
x86-64 provides one, so shipping one means emitting a hand-rolled tree into a host that has a
native array — which
[CLAUDE.md](../CLAUDE.md) names as *the single most common way to get the architecture wrong*.
They also allocate per node, which is the ceiling ADR 0013 is already unhappy about.

### 3.7 Erlang binaries: what was actually invented

The user's instinct that Erlang's binary is an invention is right, and it is worth being precise
about *which part*.

The representation is not the invention: a binary is a refcounted byte buffer, and a sub-binary is
`(offset, length, buffer)` sharing it. That is Go's slice, JavaScript's `subarray`, and a Java
`ByteBuffer` view. Three of our four hosts have it natively.

**The bit syntax is the invention** (Gustafsson and Sagonas, 2005–2007): pattern matching at
*bit* granularity with declared sizes, signedness and endianness, in both directions —

```erlang
<<Version:4, IHL:4, TOS:8, TotalLen:16, Rest/binary>> = Packet
```

— which turns protocol parsing into a declaration. That is a **matching notation over a positive
type**, and per §1.4 it is the expensive kind. [data-model.md §3](spec/data-model.md) already
records what our hosts can and cannot do here.

---

## 4. The questions, answered

### 4.1 Is the array primary because a list is too expensive?

**No — because a list's algebra needs recursion and this language does not have it.** ADR 0014
already decided this question without appearing to. `foldr` is the list's universal property; with
no recursion there is no `foldr`; without `foldr` a cons chain is an array with worse constants and
no compensating law.

The array's universal property is **representability**: `Fin n → A`, eliminated by *indexing*, and
indexing needs only bounded iteration, which is `loop`.

Stated as a principle worth testing:

> **A language's iteration construct selects its data structure.** Fold over a structure ⟹
> inductive types. Bounded loop over an index ⟹ arrays. This project chose `loop`
> ([ADR 0015](decisions/0015-loop-and-again.md)) and thereby chose arrays, six weeks before
> asking the question.

Cost is a second and independent argument, and it agrees.

### 4.2 Is there a better option?

Ranked by what the parasite rule allows:

**Better, and nearly free: the array plus an affine index map.** A view `(offset, stride, length)`
over a buffer gives `take`, `drop`, `reverse`, `stride` and the leading-axis operations with no
allocation, and the composition law is just composition of affine maps — Mullin's ψ-calculus in
miniature. In the *delayed* form we already have this for nothing, because an index map is a
lambda. What is missing is that the **reified** form does not carry offset and stride, so
materialising a slice copies where Go's `a[i:j]` would not. Go and JavaScript have this natively;
Java does not for arrays; x86 has it by construction.

**Better, and cheap: shaped/nested arrays.** `Fin n → Fin m → A` is currying, and Naperian
functors compose, so multi-dimensional arrays need no new mechanism — and row-major versus
column-major becomes a *choice of index map* rather than a representation. Worth having when a
program demands it, not before.

**Not better here: ropes, finger trees, RRB-vectors, HAMTs.** Out on the parasite rule (§3.6).

**Not available: anything inductive.** Out on §1.2.

So: **the array is the right primary structure, and the honest improvement is the index map, not a
different container.**

### 4.3 What is the algebra, and can we reason about it?

There are three algebras in play and this project already has parts of all three.

**The product algebra is β.** The projection laws of §1.1 *are* β-reduction under the Church
encoding, so the reducer implements the product's universal property already and needs nothing
added. This is the strongest single argument for the negative product: its algebra costs zero
machinery.

**The shape algebra is what `(length N)` started.**
[target-files.md §3](spec/target-files.md) now lets a target declare that a primitive's result is
`n` long, or as long as its argument. That is the first two entries in a table that MoA and
Iverson wrote out in full:

```
alen (zip f a b)     = min (alen a) (alen b)      -- or a precondition that they agree
alen (map f a)       = alen a
alen (take k a)      = min k (alen a)
alen (materialize v) = vlen v
shape (transpose a)  = reverse (shape a)
```

The machinery to *use* these already exists: the interval analysis, the linear-arithmetic decision
procedure, and the size-change termination checker all consume exactly this kind of fact. Extending
the shape algebra is compiler work with no language change, and it is the highest-value item in
this document that requires no decision.

**The reasoning discipline is extensionality.** TLA+'s entire calculus over functions is
`DOMAIN`, `EXCEPT` and extensionality. Ours would be `alen`, a functional update, and:

> two delayed vectors are equal iff they have the same length and agree pointwise.

We **cannot state that today**, because equality on functions is not decidable and the reducer has
no rule for it. It is worth naming as a known gap rather than pretending the algebra is complete:
the laws above are usable as *rewrite rules the compiler applies*, not as *propositions a program
can assert*.

### 4.4 Do we need map/dict?

**We already have one, and it should stay where it is: target-native.**

Gauntlet program 4 forces a dictionary, `targets/go/` declares `map[string]int`, `targets/js/`
declares `Object.create(null)` (measured 3.25× better than `Map`), and both are at parity. The
question is only whether a *portable* `dict` should exist, and there are two reasons to say no.

**Iteration order is an observable disagreement.** Go randomises map iteration deliberately and the
specification says so. JavaScript specifies object key order — integer-like keys ascending, then
string keys in insertion order. Java's `HashMap` is unspecified and `LinkedHashMap` is insertion.
By the test in [CLAUDE.md](../CLAUDE.md) — *if they disagree, is the disagreement observable?* —
this is Tier 2 and carries no portability claim. A portable dict could offer
`empty/get/set/has/delete/size` as Tier 1 and would have to either omit iteration or pay for a
sorted order.

**And one of our four targets has no dictionary at all.** `targets/windows/` emits assembly.
Making `dict` portable means *implementing* a hash table for one target out of four, which is
lowering further than three of them require to serve the fourth — precisely the inversion the
parasite model exists to prevent. The right answer is that a program using a dictionary does not
cover on that target, and **covering reports it**. That is the capability graph doing its job,
which is a better outcome than a portable dict that is secretly a hand-rolled hash table.

Note also that a **set is a dict to unit**, so it is not a separate question.

### 4.5 Do we need tuples?

**We need multiple return values, which is not the same thing, and we already have the term-level
form.**

All six recorded demands are multiple-return, not data-modelling: `fold-range2`'s pair (now gone),
Go's `(int, error)`, `Fprintf`'s `(int, error)`, `strings.Cut`'s three results, one `idiv`
producing quotient and remainder, and a bignum's inline fast path. None of them wants a tuple that
is *stored*; every one wants a call that answers two questions.

Scheme and Common Lisp made exactly this distinction on purpose, and for exactly our reason (§3.5).
Three of our four targets have a native form for it.

`(fn (k) (k a b))` already expresses it and already reduces away. What is missing is not a term
form. It is (a) a name, (b) a backend lowering for the case that survives to the residual, and (c)
a type for it — see §5.

### 4.6 Do we need structs?

**Eventually, and the thing to get right is that a struct must not fix its layout.**

A struct is a product with *named* projections, and names earn their keep for one reason this
project already relies on: [types.md](spec/types.md) makes parameters named "because a refinement
attaches to a name". The same argument applies to fields.

But g2 measured that the layout question has different answers per host — array-of-structs costs
JavaScript **2.86×** and Go **1.05×** — so a record type that fixes array-of-structs is a
portability hazard of exactly the same kind as an integer type that fixes a width. kdb+, Jai's
`SOA`, Zig's `MultiArrayList` and ISPC's `soa<N>` all exist because that choice has to be made
somewhere.

> **Layout is a representation selection, and this project already has representation selection**
> ([selection-2026-08-19](../gauntlet/results/selection-2026-08-19.md)): a declaration in the
> source, a decision per target, and the price named. A record declaration should name the fields
> and let the target choose array-of-structs or struct-of-arrays, the way `(checked …)` lets it
> choose an overflow form.

That is a genuinely reusable piece of machinery and an argument for doing records *after* the
integer representation question settles rather than before.

### 4.7 What about sums?

Worth separating out, because `(value, error)` is usually miscategorised.

**`Result` is a sum, not a product.** Go spells it as a product by convention and then relies on a
discipline — "if err != nil" — that the type system does not enforce. A real `Result` is `A + E`,
which is **positive**: it must be built, it needs a tag, and there is no negative version to fall
back on. Costs: a discriminant plus payload on Go and x86, `{tag, …}` on JavaScript, a sealed
interface plus records on the JVM. Sums are strictly more expensive than products and always will
be.

Two things make this less urgent than it looks.

**`bool` is `1 + 1` and `if` is its eliminator**, so the degenerate case is already in the language
([ADR 0017](decisions/0017-booleans-are-in-the-language.md)), and it is the case that covers most
uses.

**Refinements are this project's answer to `Option`.** `aindex` does not return `Maybe A`; it
carries a precondition, discharged at the call site, and the runtime cost is zero. Division by zero
is handled the same way ([integers.md §5](spec/integers.md)). Where a precondition can be
discharged, a sum is a runtime tag standing in for a proof we already have. That is not a
universal substitute — a parse failure is genuinely dynamic — but it covers the cases where the
cost would hurt most.

And when sums do arrive: **closed, finite and non-recursive only**, because recursive sums are
initial algebras (§1.2).

---

## 5. Does it fit the language?

Walking the recommended shape through every stage, because "does it fit" is answerable precisely.

### The reader

Nothing. `(fn (k) (k a b))` parses today. A surface form — `(pack a b)` / a binding form for the
consumer — is reader sugar that erases, the way `let`, `seq`, `loop`, `and`, `or`, `cond` already
do ([booleans.md](spec/booleans.md)).

### The reducer

**Nothing.** This is the decisive property. The negative product is a lambda; its β law is β. The
atom ([the-atom.md](the-atom.md)) says the normal form is a parameter and reduction runs until only
primitives remain — so a *reified* product is another primitive a target declares, exactly like
`make-vec` and `materialize`.

Contrast with what would break it: a **positive** product with pattern matching needs a `case`
form, and a **sum** needs case-of-case to fuse — which q5b already identified as a shape-directed
rule, not δ. So:

> **The boundary between "free" and "needs a new reduction rule" is exactly the boundary between
> negative and positive.** The project found this twice independently — once as q5b's stream-fusion
> finding, once as the polarity classification — and they are the same finding.

### The residual and the type checker

The Church encoding of a pair is `∀r. (A → B → r) → r`, which needs rank-2 polymorphism, which the
residual checker does not have and should not grow.

It does not have to. **Reduction erases the encoding before the checker runs — unless it
escapes.** So:

> The type system only ever has to type the products that *survive* reduction, and those are
> exactly the ones that get reified. The checker needs **nominal record types generated at
> reification points**, not polymorphic encodings.

The hard typing problem is deleted by staging. This is the same trick as Low\*/F\* erasure and as
Cogent, both of which CLAUDE.md already cites, and it is a strong argument that the negative
product costs the type system almost nothing.

For arrays, note that **`Fin n` already exists** in this language, spelled as a refinement:
`aindex`'s `(where (and (<= 0 i) (< i (alen v))))` *is* the judgement `i : Fin (alen v)`. Dex's
design point is closer than it looks.

### The backends

| target | negative product, reified | array view |
|---|---|---|
| Go | two return values, or a value struct — both 0 allocs | `a[i:j]` native |
| JavaScript | object literal (1.11×), **not** an array (1.32×) | `subarray` native |
| Java | a `record`; C2 scalar-replaces it | no array view |
| x86-64 | `rax`:`rdx` per the ABI | native by construction |

Three of four have a native multiple-return. All four have a workable reified form and it is
already measured. This is a `targets/*.oro` change plus a backend lowering, which is the shape of
change this project handles well.

### What it does to `emit/`

The honest cost: a fourth structural concern in the backends, alongside `let`, `loop` and `if`.
[assessment-2026-08-20 §4](assessment-2026-08-20.md) already flags that the compiler is growing
faster than the language. A reified product is a real addition to that, and it should be counted.

---

## 6. Candidates

**D-A — nothing new.** Keep the Church encoding, add no reification. *Rejected*: it is the status
quo, and the status quo is that a product cannot cross a function boundary, which is why
`fold-range2` needed a finisher and why `(value, error)` cannot be written.

**D-B — multiple return values, lowered per target.** Name the negative product, type it at
reification points, lower it to Go's multiple return / a JS object / a Java record / `rax:rdx`.
*Recommended first.* Zero core change, zero reducer change, covers all six measured demands, free
on three hosts and 1.11× on the fourth.

**D-C — D-B plus an explicit reifier**, symmetric with `materialize`: a name the programmer writes
when a product must escape, so the allocation is visible in the source. *Recommended second.* It is
the same interface decision `materialize` already made, and
[q5b §8](spec/q5b-filter.md)'s safety property generalises: every allocation the data library can
cause is named in the source.

**D-D — named records with target-chosen layout.** `(record point (x f64) (y f64))` where the
target picks array-of-structs or struct-of-arrays. *Recommended third, and not yet.* It reuses
representation selection, but it should wait until the integer representation question settles, so
that one mechanism serves both.

**D-E — a portable dict.** *Rejected*, on iteration order being an observable disagreement and on
one of four targets having no dictionary at all (§4.4). Dictionaries stay target-native, and
covering reports the gap.

**D-F — finite closed sums.** *Deferred.* Positive, so always allocated or tagged; and refinements
already cover the `Option` cases where the cost would hurt most. Revisit when a program needs a
genuinely dynamic alternative that a precondition cannot discharge.

**D-G — inductive types (lists, trees).** *Rejected*, and the reason is ADR 0014 rather than cost
(§1.2, §4.1). **This is the one to reopen if a bounded recursion scheme ever arrives** — a
catamorphism whose depth is statically bounded would make the list's universal property writable,
and then the cost argument would have to be made on its own.

**D-H — persistent vectors, ropes, RRB-trees.** *Rejected* on the parasite rule (§3.6).

**D-I — array views: offset, stride, length.** *Recommended, and cheapest of all.* Free in the
delayed form already; the work is carrying it into the reified form where the host has one. Buys
`take`/`drop`/`reverse`/`stride` with no allocation, and it is the honest answer to "is there a
better structure than an array" (§4.2).

**D-J — shaped/nested arrays, Naperian style.** *Deferred, cheap when wanted.* Currying plus
composition; no new mechanism.

---

## 7. Recommendation

**Add the negative product as multiple return values (D-B), with an explicit reifier (D-C), and
carry array views into the reified form (D-I). Everything else waits.**

The reasoning in one paragraph: the negative product costs the core nothing because β already is
its algebra; it is measured free on three hosts and 1.11× on the fourth; it covers all six
independent demands; the type system escapes the polymorphism problem because reduction erases the
encoding except exactly where reification gives it a nominal type; and the one place it costs — a
product crossing a runtime boundary — is a boundary the language already has, already names, and
already charges for.

What this document changes about the earlier framing: **the product is not a new kind of thing to
add to the language. It is the same thing as the array, at a different index set, with the same
delayed/reified split and the same boundary.** `materialize` and reification are one operation.
That is worth an ADR when one is written, and it is a better organising principle than "add a
product type".

### What would falsify this

- **A program that needs a product to be *stored*, not returned.** Every demand so far is
  multiple-return. A container of pairs would make the positive product unavoidable and change the
  ranking.
- **Reification turning out to be common rather than rare.** If most products escape, the negative
  form is a complication rather than a saving, and a plain struct is the better design.
- **A measured case where JavaScript's 1.11× compounds.** It was measured on a non-escaping
  product in a hot loop. A program that returns products across a loop boundary on V8 could be much
  worse, and that is the cheapest useful measurement to take next.
- **A bounded recursion scheme.** It reopens D-G, and with it the entire inductive-type family.

### What to measure next, in order

1. **A product crossing a loop back edge, on all four hosts.** This is the one number the existing
   measurement explicitly does not have, and it is what decides whether D-C's reifier is a rare
   escape hatch or the common case.
2. **`(value, error)` as a product versus as two calls versus as a sum**, on Go and JavaScript. It
   is the most-demanded shape and the one where the product/sum confusion is most costly.
3. **Array views: does carrying offset and stride into the reified form cost anything on Go?** Go's
   slice header is three words and already exists, so the expected answer is no — which would make
   D-I free.
4. **Struct-of-arrays versus array-of-structs, emitted from one source**, to confirm that layout
   selection is worth building before D-D is designed.
5. **`SumPointerGraph` against `SumIndexGraph`**, which is already written and has never been run
   into a result file (§3.1). It is the pointer-chasing cost this document leaned on and did not
   have — cheap to close, and the kind of unmeasured assertion this project has been wrong about
   half the time.

### Deliberately not next

**A unified collection type.** q5b already established that unifying pull and push needs strictly
more machinery than keeping both, and §1.3 explains why: filtering is not a container morphism, and
no representation makes it one.

**A dict in the language** (§4.4). **Inductive types** (§1.2). **Full dependent types for shapes** —
the refinement system already provides `Fin n` where it is needed, and Low\*'s lesson is that the
restriction is the mechanism.
