# Products: the one empty cell in the algebra

Research, 2026-09-09. No decision and no build. On hamza's *"if we say that we have functions
and sets, in mathematics they use tuples and records (TLA) which are themselves functions as well
… whatever we are going to add to the language should come from well-reasoned mathematics, not
because we want to reflect windows or go. Then the compiler spits out the correct type for the
target."*

> **The answer in one line.** Everything in this language is `Π` over an index set, and exactly one
> cell of that table is empty: the **positive, heterogeneous, statically-indexed** product — the one
> that must EXIST because it escapes. `values` is its negative twin and was built in its place.
> The record is not a new idea: **it is `(array x y)` with the indices static**, which
> [tables.md §5.3](spec/tables.md) already derived and nobody built.
>
> And it is not a new proposal either. [type-algebra.md §8](type-algebra.md) lists four things to
> build in order. Items 2, 3 and 4 — sums, `match`, case-of-case — are built.
> **Item 1 is the product, and it is the only one that is not.**

---

## 1. Tuples and records are functions, and this project has said so twice

TLA+ is literal about it. `<<a, b>>` is a function whose domain is `1..2`; `[x |-> 1, y |-> 2]` is
a function whose domain is `{"x", "y"}`. A tuple and a record differ only in **which finite set
indexes them**, and a record is a tuple up to a bijection on that set.

[data-structures.md](data-structures.md) reached the same place from five directions at once — TLA+,
containers (Abbott/Altenkirch/Ghani), Naperian functors, Dex, and SML's `(a,b) = {1=a,2=b}` — and
recorded the conclusion as *everything is a function from an index set*.

And [tables.md §5.3](spec/tables.md) went further, in a sentence written before the product existed
and never acted on:

> **"A statically-indexed heterogeneous `(array x y)` is a pair, and `(a 0)`/`(a 1)` are its
> projections. The tuple is not a separate feature."**

So the mathematics hamza is pointing at is already the mathematics in the repository. What follows
is what it forces.

## 2. The whole language is `Π`, and one cell is empty

A dependent product `Π_{i∈I} V(i)` is *a thing that yields a V(i) for each i in I*. Three parameters
generate every data construct this language has:

- **the index set `I`** — infinite, dynamically finite, or statically finite;
- **the family `V`** — constant (homogeneous) or varying (heterogeneous);
- **the polarity** — given by how it is *consumed*, or by what it is *made of* (§3).

| construct | `I` | family | polarity | status |
|---|---|---|---|---|
| `(fn (A) B)` | `A`, possibly infinite | constant | negative | built |
| `(array V)` | `Fin n`, `n` dynamic | constant | positive | built |
| `(map K V)` | finite `S ⊆ K`, dynamic | constant | positive | built |
| `(values a b)` | `Fin n`, **n static** | **varying** | **negative** | built |
| `(sum …)` | `Σ`, not `Π` | varying | positive | built |
| **a record** | **`Fin n` static, or labels** | **varying** | **positive** | **absent** |

Six cells, five filled. The empty one is not a gap in a list of features — it is the only
combination of the three parameters that nothing occupies.

`Σ` belongs in the table because it is the dual and it is built: [sums.md](spec/sums.md) says *a sum
is Σ, so its value is a tag and a payload, which is a **product*** — so the sum's payload is already
a product waiting for one to exist. Today an n-ary payload is a special case in the sum machinery
rather than `×` applied to `Σ`.

## 3. Polarity is the whole question, and it has nothing to do with Windows

The distinction that decides everything here is not `⊗` against `&`. It is older and simpler:

> **A type is NEGATIVE when it is given by how it is ELIMINATED, and POSITIVE when it is given by
> how it is INTRODUCED.**

`(values a b)` is reader sugar for `(fn (#k) (#k a b))` — a thing defined entirely by what a
consumer does with it. Under call-by-need a component nobody projects is never computed, which is
precisely why it measured **0.99× with zero allocations** ([values.md](spec/values.md)): a negative
product need not exist.

A record is defined by what it is made of. Both components are there because the value is there. It
has an identity, so it can be an element of a table, the payload of a sum, or a value crossing a
boundary we do not control.

[type-algebra.md §2.2](type-algebra.md) already wrote the criterion without naming it:

| | free when | must exist when |
|---|---|---|
| `A × B` | consumed in the same reduction | **it escapes a boundary** |

**So the positive product is the negative one granted a representation, so that it may escape.** And
the compiler already says exactly this, in the only vocabulary it has:

```lisp
(sig mk ((a int) (b int)) (array int) (where (< 0 a)))
(def mk (fn (a b) (build 4 (fn (t) (set t 0 (values a b))))))
```
```
gen: This is an escaping closure. g6 measured its cost but the emitter
     does not implement closures yet.
```

It is not a closure. It is a pair with nowhere to be written down. values.md knew this and fixed it
at **one** boundary — *"`(sig f (…) (int int))` is what disambiguates a product from an escaping
closure"* — and the general form of that sentence is the whole of this document.

## 4. Measured: the algebra is not closed under `×`

`×` is not an object of the type language. It is an arity convention in one slot:

```
(int int)              →  "(int int) is not a type"
(array (int int))      →  "is not a type"
(map int (int int))    →  "is not a type"
(buffer (int int))     →  "is not a type"
result (int int)       →  Results ["int" "int"]        ← a LIST, not a type
```

Every law in [type-algebra.md §2.1](type-algebra.md) that mentions `×` is therefore unavailable
inside `array`, inside `map`, inside a sum's payload, and inside another product. A semiring whose
`×` is not an object is not a semiring; it is a calling convention.

## 5. What the absence costs, in this repository, measured

**Four programs encode `Π_{Fin n}(A × B)` as `Π_{Fin kn} C` by hand.**

| program | stride | what it is | how it is written |
|---|---|---|---|
| `examples/json/tree.oro` | 4 | tag / val / kid / sib | `(def nsl (fn (k o) (+ (* 4 k) o)))` |
| `examples/io/freq.oro` | 2 | start / end of a word | `sbeg`, `send` — 16 strided index expressions |
| `examples/io/freq.oro` | 2 | word / count | `dslot`, `dword`, `dcount` |
| `examples/kara/core.oro` | 3 | `ao` / `bo` / `ln` | a descriptor table over one arena |

`nsl` is not a helper. **It is the address arithmetic of an array of records, written as a
definition** — `base + stride·index + fieldOffset` — which is what a compiler emits for a field
access, hand-written because there is no field.

**And the compiler grew an inference rule to re-prove it.** json-tree-bench-2026-08-26:

> *`entails` matched a fact to a goal by requiring identical coefficients, so a known `sp >= 1`
> could not discharge `2*sp - 1 >= 0`. One Farkas multiplier fixes it, and **a stride is exactly the
> shape it missed** … no program here had a strided index until a node table.*

It took the tree from **110 undischarged obligations to 40** and **1.12× faster**, and the clamps it
did not remove cost **1.35×**.

> The compiler has an inference rule whose only purpose is to re-derive the address arithmetic of a
> record that the programmer encoded by hand. **What the compiler generates, it does not have to
> prove.** That is json-tree-bench's own lesson one level up — it found that *fewer clamps left
> FEWER undischarged obligations, because a clamp hides the fact instead of establishing it* — and
> a generated stride hides nothing and establishes everything: **do not prove what you can
> generate.**

## 6. The theorem that makes it worth having

`Π_I` is a right adjoint, and right adjoints preserve limits. Products are limits. Therefore:

```
    Π_{i∈I} (A × B)  ≅  (Π_{i∈I} A) × (Π_{i∈I} B)
```

That is **array-of-structs ≅ struct-of-arrays**, and it is a theorem rather than a folklore
equivalence. Currying gives the third form:

```
    Π_{(i,j) ∈ I×J} V  ≅  Π_{i∈I} Π_{j∈J} V
```

which is **the stride** — the flat table indexed by `k·i + j` that all four programs above write by
hand.

**All three are the same object up to isomorphism, so which one is emitted is a TARGET decision.**
That is ADR 0003's *"mathematical semantics, machine representation"* arriving at the product, and
the choice is not academic: json-tree-bench-2026-08-26 measured flat against boxed on one program
across three hosts and got **Go 2.52× for flat, JavaScript 1.22× for flat, and the JVM 1.24× for
RECURSIVE — flat loses there**.

> A hand-strided table has already chosen, on every host, and it chose by hand. **A record type is
> what gives the choice back to the compiler**, per target, which is the thesis of the whole
> project.

## 7. Where it needs nothing new

**Projection is application.** `(p 0)` — tables.md's rule, unchanged. No new term kind, no new
eliminator, no accessor primitives.

**The index must be static, and that is forced rather than imposed.** tables.md §5.3:

> Every index a literal: the elements need not share a type — the checker knows *which* element it
> is looking at. Any index dynamic: the elements must share a type.

Read forward it explains the array; read backward it *defines the record*. And since reduction
removes every static index, **the checker never sees a dependent type** — the heterogeneous
telescope is erased by staging, which is Dependent ML again and the reason no dependent typing is
needed anywhere in this language.

**Flat n-ary is legitimate.** Associativity and commutativity hold up to isomorphism
(type-algebra.md §2.1), so a three-field record is not nested pairs and no re-association is needed.

**Three of the four representations already exist**, built for `values` and measured:

| target | what `values` already emits | measured |
|---|---|---|
| Go | several results | 0.99×, 0 allocs |
| Java | a `record` shared by result SHAPE | 1.01×, C2 scalar-replaces across a class boundary |
| JavaScript | an **object**, not an array | 5,164 ns against 8,348; free when destructured |
| windows | `rax`/`rdx` | mirrors the Win64 convention |

## 8. Where it costs, honestly

**1. A table's element stops being a scalar.** `ElemBytes`, `int-repr` and the whole element-width
path assume the element is a number. A record element needs a **layout** — field offsets and a
size — and that is the first genuinely new mechanism, not a generalisation of an old one.

**2. AoS against SoA is a real per-target decision and owes a measurement.** §6 says they are
isomorphic; ADR 0008 says never assert which host construct is faster. So the choice is a target
declaration backed by a benchmark, exactly as `int-repr` and `big-repr` are, and picking it by
argument would be the mistake this project has made and caught most often.

**3. ADR 0020 rule 6 extends.** A buffer may not be a record field, for the reason it may not be an
array element: an observation must not be able to extract an alias, which is what keeps the
read-borrow free with no `let!` and no observers.

**4. Equality follows, and should be named rather than allowed to fall out.** `=` on a product is
componentwise and is decidable exactly when each component's is. The language's `=` is integer
equality, so **a record of ints would have a decidable `=` — which makes `(map (int int) V)`
well-formed**, since maps.md's rule is *`(map K V)` is well formed exactly where `=` is defined*.
The lexicographic order on a product is canonical too, so `keys` still has its `≤_K`. That is a real
extension of the map's key type and it should be a decision, not a consequence nobody noticed.

It does **not** reopen freq.oro's finding: a word is a byte sequence of *dynamic* length, not a
fixed product, so it is still not a key.

**5. The residual gains a non-scalar type in a table slot for the first time**, and every
width/representation path in `emit/` was written on the assumption that it never would.

## 9. What the mathematics does not license

**Untagged unions.** hamza's *"union of sets"* is the one place the algebra says no. `|A ∪ B| ≠
|A| + |B|` when they overlap, so a union is idempotent and is **not a coproduct**; the disjoint
union is, and that is the sum we already have. type-algebra.md §3 refuses it, and the refusal
stands unchanged.

Win32's `union` — `LARGE_INTEGER` and its kind — is **reinterpretation**, reading one layout as
another, which precision-integers.md refuses on a stronger ground: *types choose representation* is
wanted, *operations determine meaning* would make every bound, range and termination proof unsound.

**μ.** A recursive record is refused, and the alternative is measured rather than asserted: recursive
data is a flat table plus indices.

**Subtyping.** Width or depth subtyping on records is F<:, undecidable in general; we keep the
decidable half we already have.

**Labels as a distinct construct.** A record is a tuple up to a bijection on its index set, so labels
are *surface*. They buy legibility and a self-describing type, and they buy nothing algebraic —
which is consistent with type-algebra.md's *products anonymous, sums named*, arrived at from the
other direction.

## 10. The status this leaves

This is not a new proposal. It is the oldest unbuilt item in the language's own plan, and
type-algebra.md §8 already said what would be hard about it:

> *"The product, redone properly as `×` rather than as `values` — and this time implemented on all
> four targets, **because the last attempt was reverted precisely for declaring it optional**."*

And the Win32 survey is a **second, independent witness arriving from outside**: 1,610 names —
13.9% of the flat API — refused as *struct by value*, of which 675 are enums the tool misread and the
rest are aggregates the ABI passes in a register or by a pointer to a layout. The project treats two
independent demands for one feature as its strongest evidence — it is what settled the postcondition
naming a result — and this is that pattern, with the internal demand (four strided programs and a
Farkas rule) older than the external one.

## 11. What would decide it

Three measurements, in order, and the first is the falsifiable one:

**1. Does the record recover the clamps?** The two strided programs pay the same cost in
different currencies, and both were re-measured today rather than quoted:

| | undischarged index obligations | explicit clamps written | strided index expressions |
|---|---:|---:|---:|
| `tree.oro` | **8** (105 of 111 operations bounded) | `nsl` 4, `nslc` 6, `cn` 1 | stride 4 |
| `freq.oro` | **0** — it clamps instead | `cli` 25, `cl` 2, `pat` 7 | **16** |
| `kara/core.oro` | **6** (9 of 12) | `c4` | stride 3 |

`freq.oro` is at 951 of 951 with nothing propagated, and that is not the absence of the cost — it is
the cost **paid in advance**: thirty-four clamp call sites, most of them clamping an index the
program computed from a stride it also wrote. Rewrite either table as an array of records and count
again. **If neither number falls, §5's argument is wrong** and the feature is ergonomics alone.

(The historical figures are json-tree-bench's — 110 undischarged obligations to 40, and clamping at
1.35× — and they are a fortnight and several analysis improvements old. They motivate the question;
they are not the baseline.)

**2. Can the compiler produce today's code from the better source?** Emit the record form and diff
against the strided form, on all four targets. Byte-identical is the result that says the feature
costs nothing to adopt; anything else is a number to explain before proceeding.

**3. AoS against SoA, per host, on the node table.** json-tree-bench's shape re-run with the
representation as the variable. This is the one that decides whether §6's isomorphism is worth
anything in practice, and it is the only part that could show the whole idea to be free of value —
if every host prefers the same layout, the compiler's freedom to choose buys nothing and the record
is legibility alone.

**And one thing not to do:** do not build the layout machinery first. §8's item 1 is the real cost,
and item 2 is a decision that has to be measured; starting with the surface would repeat the
`values` reversion, which happened because a construct shipped before every target could carry it.
