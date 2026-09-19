# What a literal table's elements are

Research, 2026-09-19, on hamza's question about the README's fourth example: why is the three-byte
input written as a `build` with nested `set`s rather than `(array 104 105 33)`, and *"this is a
question about style, that might turn out to be fundamental to the language."*

> **Built the same day** — [literalelem-2026-09-19](../gauntlet/results/literalelem-2026-09-19.md).
> That result also **corrects §4's prediction about the JVM**: `examples/json/*.oro` are Go-only
> sources, so no corpus program exercises the Java path at all, and it is covered by a unit test
> rather than by a measurement.

It is fundamental, and the style is right: a literal table is the **graph** presentation of a table
([tables.md §5](spec/tables.md)) and is the way to write one down. What stops it is a gap in the
compiler, measured below.

---

## 1. The symptom

```lisp
(let src (array 104 105 33)
  (build (* 2 (len src)) (fn (dst) (let (tuple dst n) (hex.Encode dst src) dst))))
```

```
.\main.go:13:91: cannot use src (variable of type []int) as []byte value in
                 argument to func(dst, src []byte) ([]byte, int){…}
```

**Our checker accepted it and the Go compiler refused it** — a refusal from the wrong place. Inside
the language the same literal is fine: read in a loop it prints its sum and emits
`d := []int{104, 105, 33}`.

## 2. Where an element width comes from today

Three paths decide it, and a literal has none of them.

| the table | how its element is decided | where |
|---|---|---|
| a declared parameter, `(array (int 0 255))` | the **declaration**; the target's narrowest containing representation | ADR 0003, [elemwidth-2026-08-27](../gauntlet/results/elemwidth-2026-08-27.md) |
| a `build` buffer | **inferred from its stores**, each literal being its own exact range | `storedRange`, `elemTypeFixed` |
| a buffer **nobody writes to** | from the **host call it is passed to** | `declaredElem`, [gomethods-2026-09-09](../gauntlet/results/gomethods-2026-09-09.md) |
| **an array literal** | `"array " + typeOf(first element)`, and `typeOf` of an integer literal is `int` | `emit/golang.go` `typeOf`, `arrayLit` |

So the same three bytes are a `[]byte` when stored into a buffer and a `[]int` when written as a
graph. And the fourth row is the **first-element** rule — the exact shape of the `BufferElemBytes`
bug elemwidth-2026-08-27 fixed for buffers, where taking the FIRST store made `tree.oro`'s node table
one byte and truncated every link. It is not a wrong answer here, because `int` is the widest thing a
literal can be; it is the same mistake in the safe direction.

## 3. What it should be, derived

**A table is `Π_{i<n} V`** ([tables.md](spec/tables.md)), so writing one down as a graph fixes both
`n` and every element. The least `V` that contains them is the **join in the interval lattice**:

```
V  =  ⨆_{i<n} [eᵢ, eᵢ]   =  [min eᵢ, max eᵢ]
```

**Sound by construction**, and the argument is the one elemwidth already makes: *a literal is its own
exact range*. Every element is in `V` because `V` is their hull, so narrowing a literal table can
never truncate. That is the opposite of the hazard for a buffer, where the danger is a later `set` of
something the inference did not see — and a literal has no later store at all, because it is an
immutable value (ADR 0018).

**But the hull is not the whole answer, and this is the fundamental part.** At a host boundary the
element width is part of the *host's type*: `hex.Encode` wants `[]byte`, and on the JVM a declared
`(array (int 0 255))` is `short[]` because that host's `byte` is signed. The hull of `104 105 33` is
`33..105`, which on the JVM picks a **signed byte**, so synthesis alone would disagree with the
declaration. Two directions are needed, which is ordinary **bidirectional typing** — synthesis where
there is nothing to check against, checking where there is (Pierce & Turner, *Local Type Inference*,
TOPLAS 2000; Dunfield & Krishnaswami's survey). Go's own untyped constants are the same idea: a
constant takes the type its context demands and falls back to a default.

And that split is **already this project's rule**, ADR 0003: *ranges are declared at boundaries and
inferred for locals.* So:

> **Synthesis.** A literal table's element type is the hull of its elements' exact ranges.
>
> **Checking.** Where the literal is an argument to a call whose parameter declares an element type,
> the DECLARATION decides, and the literal must fit it. If it does not, that is our refusal to make,
> naming the element that does not fit — not Go's.

`declaredElem` is precisely this rule already written for buffers; the literal needs the same
treatment, reached the same way (walk the continuation for the use).

**Why the checker cannot catch the mismatch on its own:** `ValueType` normalises a range to `int`
everywhere but a table's element slot, so `array int` and `array (int 0 255)` compare equal and
`compatible` sees nothing wrong. That is deliberate — a range says what a value *is* — and it is
exactly why the width has to be decided where the representation is chosen, in the emitter, rather
than being expected to fall out of type equality.

## 4. What changes, measured

**Literals in the corpus: 18.** Most vanish: a static index into a graph is β-tab, so
`((array 1 2 3) 1)` folds to `2` and the table never exists. What survives is a literal read at a
*runtime* index, and the emission baseline says exactly which:

| emitted file | literals | what they are |
|---|---:|---|
| `examples_json_tokenize.go.go` | 4 | the four JSON documents, `[]int{123, 34, 97, …}` |
| `examples_json_tree.go.go` | 4 | the same documents |

Both are gauntlet program 7's inputs, so this is **measurable rather than theoretical**: those
documents are byte strings currently stored eight bytes per byte.

**Per target, because the representation is the target's:**
- **Go**: `[]int` → `[]byte` (hull 33..125 for these documents).
- **JVM**: `long[]` → `byte[]`, and json-tree-bench measured element width as worth about **1.19×**
  there, against nothing on Go. That is the prediction this change tests.
- **JavaScript**: no types; nothing moves.
- **windows**: **deliberately unchanged.** `emitArrayLit` builds the graph with `mov qword ptr` and
  an element size of 8, and on that target the element size travels **by name** through `e.elem` —
  wintables-2026-08-25's hazard, where losing it at one binder read a byte array as qwords and gave a
  wrong answer. A per-target representation choice is allowed to decline, so it declines.

## 5. Acceptance, written before the build

1. The README's fourth example compiles and runs as `(array 104 105 33)`, printing `686921`.
2. A literal whose element does **not** fit a declared parameter is refused **by us**, naming the
   element and the declaration — with a witness that fails against today's compiler, where Go catches
   it instead.
3. A literal that is not all exact literals, or whose hull exceeds a byte, stays `int` — a control,
   because a test where the narrow case and the wide case look alike proves nothing.
4. Emission changes **only** the two JSON programs on Go and the JVM; windows and JavaScript are
   byte-identical, and so is every other program.
5. The differential suite passes on four targets, and the tooling suite passes.
6. **Benchmarks before accepting**: tree and tokenize, Go and the JVM, generated against
   hand-written. json-tree-bench's 1.19× is the number to confirm or refute on the JVM; Go is
   expected to be flat.

## 6. What would refute it

- A host where the narrower literal is **slower** — the same shape as elemwidth's own findings, where
  a narrowing was withdrawn twice for resting on an unsound analysis and for not being a measurable
  gain on that host.
- A literal passed to **two** boundaries that declare different element types. None occurs in the
  corpus today; the rule then has to refuse rather than pick, and the refusal is the honest answer.

## 7. Deliberately not in scope

- **Static data.** staticdata-2026-08-20 measured compile-time materialisation as never a win, and
  nothing here changes that: a literal table is still built at run time.
- **windows**, per §4.
- **A literal of a wider type than `int`** — floats and strings keep `typeOf`'s answer.
