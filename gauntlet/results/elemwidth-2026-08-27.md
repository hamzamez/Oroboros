# Element width from the range — 2026-08-27

The first piece of [precision-integers.md](../../docs/precision-integers.md), and the one it called
*"the cheapest real payoff — do it first regardless of what happens to the rest."*

**A range is a type.** `(sig tokens ((src (array (int 0 255)))) int)` says what a source byte *is*;
each target says how wide it stores one. That is [ADR 0003](../../docs/decisions/0003-range-typed-integers.md)'s
*"mathematical semantics, machine representation"* written in the type language, five months after
it was decided.

```lisp
(sig tokens ((src (array (int 0 255)))) int)
```

| target | emitted | why |
|---|---|---|
| Go | `func GenTokens(src []byte) int` | `byte` is unsigned and holds 0..255 |
| Java | `static long GenTokens(short[] src)` | **the JVM's `byte` is SIGNED**, so 0..255 does not fit it |
| JavaScript | `function genTokens(src)` | it has no integers, and declares no representations |
| x86 | unchanged | declares none yet |

---

## 1. No host fact lives in Go

The selection rule is four lines: **the narrowest declared representation that CONTAINS the range
wins.** Everything else is in the target file, where every other host fact already lives.

```lisp
; targets/go/go.oro
(int-repr 0 255 "byte")
(int-repr -128 127 "int8")
(int-repr 0 65535 "uint16")
…

; targets/java/java.oro — no unsigned types, so `byte` is -128..127 and 0..255 is not in it
(int-repr -128 127 "byte")
(int-repr -32768 32767 "short")
(int-repr -2147483648 2147483647 "int")
```

**Java choosing `short[]` for 0..255 is the whole design working.** CLAUDE.md has recorded *"the JVM
has no unsigned types"* since the beginning as a constraint to remember; here it is a line in a data
file, and the consequence falls out with no case in the emitter. A target declares what it can hold
and the range picks.

A target that declares nothing keeps storing integers the one way it already does — which is the
right answer for JavaScript, and a measured one: [jsontok-2026-08-26](jsontok-2026-08-26.md) found a
plain packed `Array` **1.15× faster** than a `Uint8Array`.

## 2. The range says what a value is; the width belongs only to its storage

`ValueType` normalises a range to `int` everywhere except a table's element slot. A local reading a
byte array is an **integer** — otherwise a counter over one would overflow at 255 while the language
says integers do not overflow.

On Go that costs one `int(src[i])` per read, which is a zero-extend and free. On Java `short`
widens implicitly and costs nothing at all.

---

## 3. Go: parity, like-for-like at last

`-benchtime=2s -count=5`, median. The tokeniser now takes the same `[]byte` a person's does.

| | ns/op | |
|---|---|---|
| hand-written `[]byte` | 9,662 | what a person writes |
| hand-written `[]int` | 10,171 | what our element used to force |
| **generated** (`[]byte`) | **9,522** | **0.99×** |

[jsontok-2026-08-26](jsontok-2026-08-26.md) had to compare our `[]int` against a hand-written
`[]int` and note the `[]byte` number separately. That caveat is gone.

The gain is small — **1.02×** against our own previous `[]int` form — and that was predicted: the
loop is branch-bound, so an eight-times-larger input hides. The point on Go is not the speed, it is
that the comparison is now honest.

## 4. Java: it works, and it is not what Java's gap was

This is the result worth keeping.

| | ns/op | |
|---|---|---|
| hand-written `byte[]` | 7,744 | |
| hand-written **`short[]`** | **7,439** | now the fastest form measured |
| hand-written `long[]`, `int` index | 7,686 | |
| hand-written `long[]`, **`long` index** | 9,265 | **the shape we emit** |
| generated, `long[]` (before) | 9,400 | |
| **generated, `short[]` (after)** | **9,269** | **1.00× of our shape, 1.25× of hand-written** |

Narrowing the element moved us **1.4%**. The representation choice was right — hand-written
`short[]` is the fastest of the three hand-written forms — and **we cannot exploit it while the
index is a `long`.**

That is [indextype-2026-08-25](indextype-2026-08-25.md)'s cost, re-confirmed from a new direction:
the emitter is at **1.00×** of the shape it emits, and the entire remaining gap is that `i` is a
64-bit local carrying an `(int)` cast to every access.

> **So "element width from the range" is done and Java's parity gap is untouched by it.** They were
> two costs that looked like one because they were measured together. The element is a
> **declaration** — the programmer says the range. The index is an **inference** — nobody declares a
> loop variable, so the analysis has to bound it.

## 5. And the index needs exactly the fact the whole plan needs

The general form of the index fix is not another platform special case. It is: *the interval
analysis bounds `i` to `[0, len src)`, that fits the host's `int`, so declare it `int`.* Sound,
because the analysis **proved** it rather than the programmer asserting it.

And in the tokeniser it cannot, for the reason
[precision-integers.md §2.1](../../docs/precision-integers.md) isolated: `i` is assigned a
**scanner's return value**, so there is no size-change witness, no trip count, and no bound.

**Java's index cost and the precision-integer plan are blocked on the same missing fact** — a
postcondition saying `scan-string` returns more than `i` and at most `len src`. Two costs, measured
independently, one cause. That is the strongest argument yet for building result postconditions
next, and it is also the reason not to patch `indextype`'s condition again.

---

## 5b. The write side: a buffer's range is INFERRED, because a buffer is a local

ADR 0003 says ranges are declared at boundaries and **inferred for locals**. A `build` buffer is a
local, so unlike an array parameter nothing declares its width and it has to be derived from what is
stored.

```lisp
(set stk sp (if (= (src i) 123) 125 93))    →   stk : []byte
```

| target | buffer |
|---|---|
| Go | `stk := make([]byte, 32)`, stored as `byte(v)` |
| Java | `final byte[] stk = new byte[32]`, stored as `(byte) v` — 93..125 fits a **signed** byte here |
| x86 | one byte per element, the same `movzx` path a boolean table takes |
| JavaScript | unchanged |

**The inference is deliberately syntactic and weaker than the interval analysis.** A literal is its
own exact range, a conditional is the join of its branches, and a value read from an
already-narrowed table carries one. Everything else keeps the host's word.

That is a soundness choice, not laziness: **a range too narrow truncates on store and is a silent
wrong answer**, so only facts that are exact by construction are used. Widening this to the interval
domain is a real extension and owes its own soundness argument.

**Zero is always an element.** `build` zero-fills ([tables.md §14.3](../../docs/spec/tables.md)) and
a slot that is never written reads 0, so the range has to hold 0 whatever the program stores. The
tests pin that: `(set b 0 40)` gives `int 0 40`, not `int 40 40`.

### What narrowed, and what correctly did not

`tokenize.oro`'s stack becomes a byte buffer. **`tree.oro`'s node table correctly stays 64-bit**,
because it stores node indices that no syntactic fact bounds — the range is `nn < 512`, which is the
interval analysis's to know, not a literal's. Getting that answer right is the feature working.

| | ns/op | |
|---|---|---|
| Go, generated | 8,988 | **0.95× of hand-written `[]byte`** in the same run |
| Java, generated | 9,073 | from 9,269 — **2%**, and still 1.21× of hand-written `short[]` |

Java's residue is the index, again and unchanged.

---

## 5c. Two bugs, and the suite earned its keep

**`BufferElemBytes` took the FIRST store.** x86 has no type system, so element width is read off the
program — and it read one store. `tree.oro`'s node table writes a tag of 1..5 into slot 0 and a node
index of up to 511 into slots 2 and 3, so the first store said *one byte* and every link was
truncated. windows returned **4030140** where the other three returned **4040171**.

It compiled, it ran, and it returned a number. The differential suite is the only thing that saw it,
which is the second time this week a silent wrong answer has been caught by running the same program
on four hosts and demanding they agree.

**Java's value cast was on one of two store paths.** The index has its own `(int)` cast and the
emitter branches on whether that is needed; the value cast went into one branch. The branch it
missed is the one every program takes until the index can be narrowed — so it would have failed on
the first real program and passed every test that had an index-narrowed loop.

**And a third, from §7 below**: two live buffers in one body could not be told apart, because the
walk ignored which buffer a `set` targeted. It did not matter while every buffer was 64-bit. It
matters now, and where the two genuinely cannot be distinguished — one caller passes a term whose
binders are not opened — the stores **merge**, which can only widen a range and never truncate one.

## 6. What is not built

**A buffer whose range only the INTERVAL ANALYSIS knows.** `tree.oro`'s node table wants
`(int 0 511)` — two bytes, half the memory — and no syntactic fact says so. This is where
[json-tree-bench](json-tree-bench-2026-08-26.md) measured the JVM's **1.19×** element cost, and it is
still owed. Two routes: teach the inference to use the interval domain, with the soundness argument
that needs; or give a `build` a declared element type, which is a language change.

**The index.** Unchanged, and it is now the whole of Java's remaining gap in both programs.

**A differential case.** Narrowing cannot be exercised there, and the reason is itself the finding:
reduction inlines every non-exported call, so a narrowed parameter only survives at an **export**.
The suite's cases all reach `run` through inlining, so the declaration is gone before emission.
[precision-integers.md §5](../../docs/precision-integers.md) asked where declarations live once
staging removes the boundaries; this is the answer, arriving as a limitation.

## 7. One bug on the way

`LoadTarget` on a target **directory** merges each file's declarations, and the merge dropped
`Reprs` — so `targets/go/go.oro` loaded alone selected `[]byte` and the real `targets/go/` selected
`[]int`, silently. Representations are ordered, so they append rather than merging by key.

Every native target is a directory ([target-native.md](../../docs/spec/target-native.md)), so this
would have made the whole feature a no-op in every real build while passing any test that loaded a
single file.
