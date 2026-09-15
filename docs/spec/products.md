# Products

> **Respelling specified, not built — [data.md](data.md) §3, §4, §6.** The product type `(array A B)`
> and the heterogeneous literal `(array a b)` become `(tuple …)`, because `(array int)` could not
> tell a one-component product from a table of ints. Labelled products are `(record ('x A) …)`,
> lowered as the tuple of their canonical label order. That is §1's *"a tuple up to a bijection on
> the index set"* made precise. The positions, representation and flattening below are unchanged.

The specification CLAUDE.md's first rule requires and
[product-2026-09-09](../../gauntlet/results/product-2026-09-09.md) shipped without:
*"Nothing goes in without a specification saying how it behaves on every target."* The research is
[products.md](../products.md); the measurement is the result; this is the third thing, and it
answers the three questions that decide whether an addition may exist.

> **1. What does it mean, independently of any target?** A product is `Π` over a statically finite
> index set. A tuple is a function whose domain is a finite set, which is what TLA+ says literally
> and what this language's tables already are.
>
> **2. What does each target do with it, and do they agree?** Nothing, and yes — **by
> construction**. A product is flattened into a table before the type checker runs, so no backend
> has an opinion to disagree with. The differential case `product` checks it on all four anyway,
> because "by construction" is an argument and this project's rule is that a Tier 1 name without a
> conformance case is decoration.
>
> **3. Is any disagreement observable?** There is none to be observable. **Tier 1.**

---

## 1. What a product is

`Π_{i∈I} V(i)` for a **statically finite** `I` — a family of values indexed by a set the compiler
knows entirely. Denotationally:

```
    γ(prod(A, B))  =  γ(A) × γ(B)              |A × B| = |A| · |B|
```

It is the semiring's `×` of [type-algebra.md](../type-algebra.md), and the third of that document's
four build items to exist — after the sum, `match` and case-of-case.

**A record is a tuple up to a bijection on the index set**, so labels are surface and are not in the
language. That is type-algebra.md's *products anonymous, sums named*, arrived at from the other
side.

## 2. Surface

**The type is spelled the way the term already is**, and that is not a convenience:
[tables.md §5.3](tables.md) wrote the consequence before there was a type — *"a statically-indexed
heterogeneous `(array x y)` is a pair, and `(a 0)`/`(a 1)` are its projections; the tuple is not a
separate feature."*

| | |
|---|---|
| type | `(array T1 … Tn)`, n ≥ 2. Canonically `prod(T1, …, Tn)` |
| introduction | `(array e1 … en)` — the same form, which already existed |
| elimination | **application**: `((t i) j)`, `j` a literal |
| field update | `(set (b i) j v)` — `set` applied to a projection |
| length | `(len t)` counts ELEMENTS, not slots |
| capacity | `(build n f)` allocates n ELEMENTS |

**ARITY DISAMBIGUATES, AND IT IS NOT A TRICK.** `(array V)` names `Fin n` for an `n` nobody has
stated, so it is homogeneous and its length is dynamic. `(array A B)` names `Fin 2` exactly, so the
family may vary. One argument or many is precisely the distinction tables.md §5.3 draws — *a dynamic
index forces homogeneity and forces existence, and it is the same condition doing both* — written in
the type rather than discovered in the term.

**The canonical form is delimited** where every other one is space-separated, because a component
may itself contain spaces (`int 0 255` does) and `MapTypes` gets away with splitting on the first
space only because a map's key is always one token. A type never contains a comma, so `prod(A, B)`
parses back.

## 3. Where a product may appear, and where it may not

**A PRODUCT IS AN ELEMENT TYPE.** It may be the element of an `(array …)` or a `(buffer …)`, and
nowhere else. Every refusal below is by name, checked, and has a message that says what to write
instead.

| | |
|---|---|
| `(array (array A B))` | **yes** — a table of pairs, the case that has a representation |
| `(buffer (array A B))` | **yes** — the workspace that builds one |
| `(sig f ((p (array A B))) …)` | **no** — a bare product has no width the caller and callee could agree on |
| a result of product type | **no** — `values` is the product at a function boundary, declared `(A B)` |
| `(map K (array A B))` | **no** — §4's representation is *flattening an array*, and a map is not one |
| `(array (buffer V) B)` | **no** — ADR 0020 rule 6: an observation must not extract an alias |
| `(array (array A B) C)` | **no** — a nested product needs the layout §7 defers |

Each of the first three was *accepted* before this spec was written, and writing it is what found
them: a bare product parameter reached the emitter and a map of products emitted
`map[int]/*prod(int, int)?*/` — **a Go type that does not exist**, and one that a host typing
nothing would have taken silently. That is precisely why the rule exists.

**A product as a result emitted `[]int{n, n}`**, which happens to be the flat form and happens to be
right. "Happens to" is not a specification, and it is refused: several results are `values`, the
NEGATIVE product, which need not exist at all
([values.md](values.md), [products.md §3](../products.md)).

## 4. Representation: one isomorphism, and the target's choice

```
    Π_{(i,j) ∈ I×J} V  ≅  Π_{i∈I} Π_{j∈J} V                 currying
    Π_{i∈I} (A × B)    ≅  (Π_{i∈I} A) × (Π_{i∈I} B)         Π preserves products
```

The first is **the stride**; the second is **array-of-structs ≅ struct-of-arrays**, a theorem rather
than folklore, because `Π_I` is a right adjoint and right adjoints preserve limits. All three forms
are one object, so **which is emitted is a target decision** — ADR 0003 at the product.

**Today every target emits the flat one**, and that is not a choice made by argument: it is what
`tree.oro`, `freq.oro` and `kara/core.oro` already wrote by hand, so adopting the type cannot cost
speed on any host. The other two are a target declaration and a benchmark, in that order, and
[products.md §11](../products.md) records that measurement as untaken.

**A component that is not an integer has no flat representation.** `(array int string)` is a type,
canonicalises, and type-checks; it is refused where it tries to exist rather than flattened into a
slot that cannot hold both. §7.

**The flat element is the join of the components' ranges**, through `IntRange` and not `ValueType` —
scalarrange-2026-08-31's distinction, because `ValueType` gives the typing answer and this wants the
representation one. So `(array (int 0 255) (int 0 255))` is a `[]byte` on Go and a `short[]` on the
JVM, for the reason `(array (int 0 255))` always was.

## 5. Reduction and lowering

**Nothing is added to the reducer.** `((array a b) 0) → a` is β-tab, which tables.md specified and
which folds a statically indexed literal table already:
`((array 10 "two") 0)` and `… 1` emit `fmt.Println(10, "two")` with nothing allocated.

What is added is **one lowering pass**, `emit/product.go`, run before the type checker:

```
((t i) j)              ->  (t (+ (* k i) j))
(set b i (array v…))   ->  k nested stores
(set (b i) j v)        ->  (set b (+ (* k i) j) v)
(len t)                ->  (/ (len t) k)
(build n f)            ->  (build (* k n) f)
```

**After it the term is exactly what a hand-strided program is**, so the checker, the refinement
layer, the interval analysis and the four backends never learn products exist. That is what makes
question 2 answerable by construction — and it is checked: 69 of 70 emitted files were byte-identical
when the pass landed, the one exception being the program whose source was rewritten to use it.

**The arity travels with the buffer**, through `loop` variables, `let`s, and a `build` that fills
itself with products — because ADR 0018's threading means a buffer's name changes at every binder,
and detection must follow the same aliasing the rewrite does.

## 6. What the compiler must prove, and the three facts it needed

A projection becomes `(t (+ (* k i) j))`, whose bounds obligation is `k·i + j < len t` — and the
guard the program wrote is on ELEMENTS, `i < (len t)/k`. Discharging that needed three additions to
the linear fragment, each verified load-bearing:

- **a quotient is an atom.** A division was outside the fragment *entirely* — not opaque, absent —
  so a guard mentioning one bounded nothing.
- **`k·(x/k) ≤ x`** for a non-negative `x` and a positive literal `k`: a **declared sound axiom,
  never a search** (decidability-map.md), seeded once at the root because a quotient in a loop's
  guard never reaches the walk.
- **one Fourier–Motzkin elimination step**, which is the principled form of the single Farkas
  multiplier json-tree-bench added for a *hand-written* stride.

> **What the compiler generates, it discharges.** That is the point of the type rather than a side
> effect of it: a hand-strided program writes the multiplication and must then be believed about it.

## 7. Not specified, and refused for a reason

**The heterogeneous product has no representation.** `(array int string)` is a well-formed type and
is refused where it would need to exist. Giving it one is a **layout** — offsets and a size — which
is the first thing in this area that is not a generalisation of something already present, and it is
what Win32's `GUID` needs.

**Nested products.** Same reason: flattening is what gives a product a representation, and a product
of products needs the layout.

**Labels.** A record is a tuple up to a bijection on its index set; labels buy legibility and nothing
algebraic.

**Equality and ordering.** `=` on a product would be componentwise and decidable exactly where each
component's is — which would make a product of integers a legal map key, since
[maps.md](maps.md)'s rule is *`(map K V)` is well formed exactly where `=` is defined*. **Not
built**, and named here rather than left to fall out silently, because it is an extension of the key
type and deserves to be a decision.

**Untagged unions.** Not a product question, and refused on the algebra: `|A ∪ B| ≠ |A| + |B|` when
they overlap, so a union is idempotent and is not a coproduct. type-algebra.md §3.

## 8. Conformance

`gauntlet/differential/cases/product.oro` — twelve pairs built with `(array a b)`, read with
`((t i) j)`, reduced to a position-weighted checksum whose expected values were computed by hand
before the program ran. **1079 1001 845 on go, js, java and windows.** Verified to fail against two
bugs: the projections swapped, and the fields stored in the other order.

There is **no clamp in that program**: every index is a loop variable the guard already bounds, and
the stride the compiler generates from it is proven from that same guard.

`examples/io/freq.oro` is the program-scale check — both of its tables are arrays of pairs, and its
output is byte-identical to `sort | uniq -c | sort -rn` on eight real files.
