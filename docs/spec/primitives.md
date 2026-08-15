# The remaining primitives

Everything a target declares that is not arithmetic ([arithmetic.md](arithmetic.md)), not
`print-line` ([effects.md §6](effects.md)), and not structural
([target-files.md §4](target-files.md)).

Each is put through the three-question test from [state.md §6](state.md):

1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement **observable**? If so it is Tier 2 and carries no
   portability claim.

**Two of them fail question 3**, in different ways, and one of those was shipping a wrong answer.

---

## 1. `alen`, `slen` — length

> The number of elements. An `int`, never negative.

| Go | JS | Java |
|---|---|---|
| `len(v)` | `v.length` | `v.length` |

Agree exactly. **Tier 1.** Note this is *element* count, which is well-defined — unlike
`length` of a **string**, which is 4/2/2 for `"🙂"` and is why no such primitive exists
([strings.md §2](strings.md)).

## 2. `aindex`, `sat` — indexing, and the first real divergence

> The element at position `i`, **for `0 ≤ i < length`.**

| Go | JS | Java |
|---|---|---|
| `v[i]` | `v[i]` | `v[i]` |

In range, all three agree. **Out of range they do not, and the disagreement is severe:**

| | out-of-range read |
|---|---|
| Go | panics — `index out of range [10] with length 3` |
| Java | throws `ArrayIndexOutOfBoundsException` |
| **JS** | **returns `undefined`, silently**, and `undefined + 1` is `NaN` |

Two targets fail loudly, one **silently produces a wrong number that propagates**. So:

> **`aindex` and `sat` are Tier 1 within bounds and unspecified outside.** A program that indexes
> out of range has no defined meaning, and its behaviour differs on every target.

That is the honest statement, and it names exactly where a refinement type would earn its place:
`{ i : int | 0 ≤ i < (alen v) }` turns "unspecified" into "cannot be written"
([types-sketch §2](../types-sketch.md)). It is also the same obligation as arithmetic's portable
range — *stay inside, nothing checks it* — which is now the **second** hole shaped exactly like a
refinement.

## 3. `split-words` — Tier 1, after being wrong for two months

> The maximal runs of non-whitespace, in order. No empty fields. Whitespace is the Unicode
> **`White_Space`** property.

That is Go's `strings.Fields`, adopted as the specification. The other two now implement it:

| | lowering |
|---|---|
| Go | `strings.Fields(s)` |
| JS | `(s.match(/\S+/g) ?? [])` |
| Java | `Pattern.compile("[^\p{IsWhite_Space}]+").matcher(s).results()…` |

**What was there before was wrong.** `s.split(" ")` on JS and Java disagreed with Go on **four of
ten** cases — any tab, any newline, any repeated space, any leading space — so word count returned
different answers on different targets and nothing detected it. The covering check said the name
was *provided*; it cannot say the name is *right*.

Two things were needed and neither was a language feature:

- **`split` cannot express it on Java.** Even `(?U)\s+`, which *does* handle Unicode whitespace
  correctly, still yields a leading empty field for leading whitespace and one empty field for the
  empty string. A matcher over runs of non-whitespace is exact.
- **A suite.** [gauntlet/conformance/](../../gauntlet/conformance/) runs 13 cases through all three
  and requires byte-identical output. It passes. This is what
  [modules.md §8](modules.md) promised and did not have.

**Cost:** measured at 1.19× on JS against the wrong-but-fast `split(" ")`, and Java now compiles a
`Pattern` per call. Both are the price of an answer that is the same everywhere, and both are
measurable rather than assumed.

> **This is the pattern to expect.** A shared name is a *claim*, not a mechanism. Every Tier 1 name
> needs a suite or it is decoration.

## 4. `sqrt`

> The IEEE-754 square root. Correctly rounded, so **exactly one** `f64` is the right answer.

| Go | JS | Java |
|---|---|---|
| `math.Sqrt` | `Math.sqrt` | `Math.sqrt` |

IEEE-754 *requires* `sqrt` to be correctly rounded — unlike `sin`, `exp` or `pow`, which it does
not — so all three agree bit-for-bit. Negative input gives `NaN` on all three. **Tier 1**, and it
is the only transcendental-looking function that can be, which is why no others are declared.

## 5. `dict-empty`, `dict-inc` — Tier 1 in behaviour, and both effectful

> `dict-empty` is a **fresh, empty** map from string to int.
> `dict-inc` increments the count for a key, treating a missing key as zero, and **returns the same
> map**.

| | `dict-empty` | `dict-inc` |
|---|---|---|
| Go | `make(map[string]int)` | `m[k]++` — one `mapassign_faststr` |
| JS | `Object.create(null)` | `m[k] = (m[k] ?? 0) + 1` |
| Java | `new HashMap<String,Integer>()` | `m.put(k, m.getOrDefault(k, 0) + 1)` |

Behaviourally identical. The **idioms are deliberately opposite** — Go's fused increment wins,
Java's fused `merge` loses 2.6× to the unfused form, and JS's `Object` beats `Map` by 3.25×. One
source, three different best answers, all
[measured](../../gauntlet/results/baseline-2026-08-13.md). This is
[ADR 0008](../decisions/0008-measurement-over-principle.md) at its sharpest.

**Neither is pure**, and this is the language's oldest unexamined assumption
([effects.md §1](effects.md)):

- `dict-empty` has a **fresh identity** — two occurrences are two different maps.
- `dict-inc` **mutates in place**, so it is correct only while the pre-mutation map is dead.

Ordering is now guaranteed ([effects.md §4](effects.md)). **Aliasing is not.** It stays
unobservable only because **no primitive reads a dictionary** — there is no `dict-get`, no
`dict-size`, no iteration — which is the same accident that made strings cheap. Adding any reader
makes it real, and that is [g7](../derivations/g7-aliasing.md)'s open question.

## 6. Where every name now stands

| | tier | note |
|---|---|---|
| `alen`, `slen` | **1** | |
| `aindex`, `sat` | **1 in bounds** | unspecified outside; a refinement would close it |
| `split-words` | **1** | with a conformance suite, as of 2026-08-15 |
| `sqrt` | **1** | IEEE-754 requires correct rounding |
| `dict-empty`, `dict-inc` | **1** behaviourally | impure; aliasing unchecked |
| `print-line` | **2** | `1`, `1`, `1.0` for the same value |
| `num/*`, `logic.*` | **1** | within ±(2⁵³−1) for `int` |

## 7. Absent, with reasons

- **Array or dictionary writes.** `dst[i] = x` needs an ownership story, not just a primitive —
  the g7 stencil's *write* half is still unwritable while its read half now works.
- **Dictionary readers.** §5: the first one makes the aliasing hazard observable, so it should
  arrive together with an answer to it.
- **Any other float function.** `sin`, `cos`, `exp`, `pow`, `log` are **not** correctly rounded by
  IEEE-754, so the three hosts may differ in the last bit. They would be Tier 2, and no program
  needs them.
- **String operations beyond `split-words`.** Length, indexing, slicing and concatenation all
  diverge ([strings.md §4](strings.md)). Equality does not and could be Tier 1, but nothing needs
  it yet.
