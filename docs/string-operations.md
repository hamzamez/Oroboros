# What a string can do

Research, 2026-09-04. No decision, no build.

[string-literals.md](spec/string-literals.md) settled what a string *is* and how
to write one down. This asks what operations it has — derived from the algebra
first, and priced on the hosts second.

**It also corrects [strings.md §2](spec/strings.md)**, which is the same error
string-literals.md was rewritten to remove: a target fact wearing a language
fact's clothes.

---

## 1. The object, and what it gives for free

A string is an element of **Scalar\*** — the free monoid over Unicode scalar
values. "Free" is not decoration; it is a universal property, and the whole of
this section is that property being read off.

> For any monoid `M` and any function `f : Scalar → M`, there is a **unique**
> monoid homomorphism `f* : Scalar* → M` with `f*(⟨c⟩) = f(c)`.

That is a fold, and **every operation that respects concatenation factors through
it**. So the operation set is not chosen; it is enumerated.

| operation | what it is | derived how |
|---|---|---|
| `empty` | the identity | the monoid's unit |
| `concat` | the operation | the monoid's multiplication |
| `length` | `f*` with `f(c) = 1` into (ℕ, +, 0) | the unique homomorphism counting scalars |
| `count p` | `f*` with `f(c) = 1` if `p c` else `0` | the same, filtered |
| `=` | structural equality | `Scalar` has decidable equality, so `Scalar*` does |
| **UTF-8** | `f*` with `f(c)` = that scalar's bytes, into `Byte*` | a homomorphism into another free monoid |

The last row is worth pausing on. **An encoding is a monoid homomorphism** — it
respects concatenation and is determined by what it does to one scalar — so
"convert to UTF-8" is not a special operation bolted on for a host's benefit. It
is an instance of the same universal property that gives `length`.

`length` is the one to watch, because §2 says the repository currently believes
it does not exist.

## 2. The correction: three answers to three different questions

strings.md §2 measures `length` of the same text and concludes:

> There is no portable `string-length`. Not "it is hard" — the three answers are
> different integers for the same text.

**They are different integers because they are answers to different questions.**
Go's `len` asks *how many UTF-8 bytes*; Java's and JavaScript's ask *how many
UTF-16 code units*. Neither asks *how many scalar values*, which is the only
question `Scalar*` poses — and to that question there is one answer:

| | Go | JS | Java | **scalars** |
|---|---|---|---|---|
| `"café"` | 5 | 4 | 4 | **4** |
| `"日本"` | 6 | 2 | 2 | **2** |
| `"🙂"` | 4 | 2 | 2 | **1** |
| `e` + combining acute | 3 | 2 | 2 | **2** |

The scalar column is not a fourth opinion. It is what the language means, and it
is **computable on every target**: `utf8.RuneCountInString`, `codePointCount`, a
spread or a loop, and counting non-continuation bytes on x86. O(n) rather than
O(1) — a difference in **price, not in answer**, which
[maps.md](spec/maps.md) already records as the shape that does *not* make a
construct Tier 2: *"JS's `len` is O(n) against O(1) elsewhere — same answer,
different price, so not Tier 2."*

So `length` is Tier 1, and so is `=`, and so is `concat`. The reason strings have
no operations is not that they cannot; it is that nobody asked the question in
the language's own terms.

## 3. Is a string a table?

`tables.md` says a table is a function with a known finite domain. A string of
length *n* is a function `Fin n → Scalar`, so:

```
Scalar*  ≅  Σ_{n ∈ ℕ} (Fin n → Scalar)
```

A string is a **length-indexed table**, which is exactly what `(array V)` already
is. If that is taken at face value, `(s i)` is application, `len s` is the domain
bound, and **strings need no new construct at all**.

It is the elegant answer and this project's own structure points straight at it.
It is also where the cost lands, so §4 is the one that decides.

## 4. What indexing costs, measured

A string of *n* scalars of mixed width (`a`, `é`, `日`, `🙂` in rotation), summed
three ways. Times per pass; the JVM's are noisy because C2 optimises the loops
differently at each size.

**n = 16,000 scalars**

| | Go | JavaScript | Java |
|---|---:|---:|---:|
| index by **scalar position** | **262 ms** | **383 ms** | **97 ms** |
| iterate natively | ~30 µs | 78 µs | ~10 µs |
| `alloc` to an array of scalars | 41 µs | 74 µs | 60 µs |
| index **after** `alloc` | ~0 | 6.9 µs | 4.5 µs |
| index by the host's own storage unit | O(1) | 21 µs | 4.7 µs |

And the growth, which is the part that matters:

| n | Go | JavaScript | Java |
|---|---:|---:|---:|
| 1,000 | 1.0 ms | 1.7 ms | 0.38 ms |
| 4,000 | 15.9 ms | 25.6 ms | 6.1 ms |
| 16,000 | 262 ms | 383 ms | 97 ms |

**Four times the length is fifteen to sixteen times the work.** Indexing a string
by scalar position is quadratic on every host, and it is quadratic for one reason
that is not a host's fault: every host stores text in a **variable-width**
encoding, because that is the right trade for text. There is no host to move to.

Against that, `alloc` costs about **one native iteration** — 41, 74 and 60 µs —
and indexing afterwards is 6.9 µs, 4.5 µs, and free. **`alloc` + index is 81 µs on
V8 against 383 ms for indexing in place: 4,700×.**

> **Every host offers O(1) indexing into its storage unit, and none offers O(1)
> indexing by scalar.** A byte on Go and windows, a UTF-16 code unit on Java and
> JavaScript — and neither is a scalar.

## 5. So the resolution is `alloc`, and it is already the project's pattern

§3 says a string *is* a table. §4 says you cannot index it where it lies. Both are
true, and the construct that reconciles them already exists:

> **`(alloc s)` : string → (array scalar)** — pay O(n) once, then index in O(1).

`tables.md` calls `alloc` "the one construct that allocates", turning a rule into
memory. A string is a function given extensionally, so `alloc` on one is that
same operation with nothing added. And the inverse — a table of scalars back to a
string — is the very isomorphism `tables.md` names between a rule and its graph,
with **η-tab** as its law.

Three things fall out.

**The cost is visible in the source.** A program that indexes pays an `alloc` and
can see it. That is this repository's repeated preference — `tree.oro`'s node cap,
the trip bound, ADR 0018's `build` — and it is stated there as *a capacity does
not create the limit, it makes it VISIBLE*.

**It is already the measured advice.** [jsontok-2026-08-26](../gauntlet/results/jsontok-2026-08-26.md)
found a string-based tokeniser **1.89× slower than an array-based one on V8** and
concluded that a JSON API handed a string should convert once rather than index
it. §4 is that finding generalised and given a number.

**And `length` becomes an optimisation rather than a primitive.** `length s` is
`len (alloc s)` by definition — but that allocates, and every host has an O(n)
scalar count that does not. So `length` is declared per target as a *faster way to
compute a derived operation*, which is exactly what `fold-map` turned out to be
in [maps.md §7](spec/maps.md): a corollary that survives as a licence to optimise.

## 6. What is NOT derived, and should not be invented

**Graphemes.** `"e" + combining acute` is two scalars and one thing a person would
call a character. A grapheme cluster is a **quotient** of `Scalar*` by the
segmentation algorithm of UAX #29 — which is table-driven, large, and *versioned
with Unicode*, so the same string has a different grapheme count under different
Unicode releases. Nothing in the free monoid produces it. It is a real notion and
it is a library over an operation set, not part of one.

**Collation, case mapping, normalisation as an operation.** All three are locale-
and version-dependent (`"i".toUpperCase()` is `"İ"` in Turkish). None is a
homomorphism out of `Scalar*` in any useful sense. They belong to the host or to a
library that names its Unicode version.

**Ordering.** `Scalar` has a total order, so `Scalar*` has a lexicographic one —
that *is* derived, and it is not the order anyone wants for human-facing sorting,
which is collation. Worth having and worth labelling as code-point order.

## 6a. Would storing the length settle it?

It fixes the cheaper half, and this document should have kept the two O(n)s
further apart:

| | why it is O(n) | does a stored length fix it |
|---|---|---|
| `length s` | the scalar count must be computed by scanning | **yes** |
| `(s i)` | the *i*-th scalar's position depends on every width before it | **no** |

The second is the expensive one and it is quadratic because the encoding is
variable-width — the question is not *how many* but *where*, so a count cannot
answer it. And `alloc` already gives the length in O(1) **plus** the indexing, so
a stored length is dominated by it. Fully worked in
[overloading.md §2](overloading.md), along with the one honest residue: a
`length` inside a loop guard is O(n²) unless a loop-invariant pure call is
hoisted, which this compiler does not do.

## 7. The one type problem

`Scalar` is `[0, D7FF] ∪ [E000, 10FFFF]` — **not an interval**, because of the
surrogate hole. The range language has one interval per type, so
`(array (int 0 1114111))` is a strict superset of `(array scalar)`: it admits
D800, which is not a scalar and has no UTF-8.

That matters at exactly one place — building a string from a table — and there are
three candidate answers, none obviously right:

- **a `scalar` base type** in the type language, which is honest and adds a type;
- **`(int 0 1114111)` plus a refinement**, which is a disjunction and the linear
  fragment decides conjunctions;
- **a check at the boundary**, refusing or trapping where a table becomes a
  string — which is `big-fit`'s shape exactly, and the precedent is two days old.

The third is cheapest and localises the problem to one operation. It is named
here rather than settled.

## 8. What this proposes

Not a build — a shape for one, in the order the measurements justify.

1. **`length`, `=` and `concat` are Tier 1**, because the language's question has
   one answer on four hosts and §2's contrary conclusion was about the hosts' own
   questions. Each is declared per target so the host's own fast path is used.
2. **`alloc` and its inverse** are the bridge to indexing, and they are existing
   constructs rather than new ones.
3. **Nothing else in the core.** Substring, search, split, trim, case, collation
   and graphemes are all expressible over (1) and (2) or belong to a host.

The measurement that would change this is the one §4 did not take: **what a real
text program actually does.** If it indexes, §5 is right. If it only folds —
which is what a tokeniser, a renderer and a formatter all do — then `alloc` is
rarely reached and the monoid alone is enough, and that would be worth knowing
before declaring an operation nobody calls.

**The first program to ask is decimal rendering of a bignum**, which windows needs
and which is a fold: repeated divmod produces digits, and digits concatenate. It
uses `concat` and never indexes. That is one data point in the direction of "the
monoid is enough", and it is available now.

## 9. TAKEN — and it folds

2026-09-04, [render-2026-09-04](../gauntlet/results/render-2026-09-04.md),
[examples/big/render.oro](../examples/big/render.oro).

Decimal rendering is written in **three operations, and all three are §1's**:
`concat`, `""`, and η as `string-of`. It uses **no `(s i)`, no `length` and no
string `=`** — so the first real text program in this language needed nothing
from §5 and nothing from
[overloading.md §3](overloading.md)'s concept-name machinery.

`concat` and `string-of` are **two declarations per target and no compiler code**,
found by spelling. windows declares neither and is skipped for a capability
reason: it has no string type, so those two are a library over `build` there.

**What this settles and what it does not.** It settles that the monoid alone
carries a text-PRODUCING program, so §8's items 1 and 2 should be deferred rather
than built ahead of a caller. It does not settle §5: a text-CONSUMING program —
a tokeniser over text, a formatter reading a template — would index, and
jsontok-2026-08-26's 1.89× is that case pointing the other way. **One program is
one data point**, and the honest reading is that the pressure to make `string` a
table has not arrived yet, not that it will not.
