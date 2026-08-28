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

## 5d. And the interval analysis decides the rest

> **WITHDRAWN 2026-08-27, the same day.** This section's gains rested on an interval fixpoint that
> was not monotone ([fixpoint-2026-08-27](fixpoint-2026-08-27.md)). With that fixed, `BufferRange`
> correctly **declines** to bound `tree.oro`'s node table — the pass runs on the `build` lambda alone,
> where `src` is free — so the table is `[]int` again, not `[]uint16`, and the timings below are
> withdrawn with it. The old answer `int 0 512` was a *sound range reached unsoundly*: it contained
> 511, so no wrong answer shipped, but that was luck rather than the analysis working.
>
> **§5b's syntactic narrowing is untouched.** A literal, a conditional over literals, and a read from
> an already-narrowed table never needed the fixpoint, so the tokeniser's stack is still `[]byte`.
> That is the part of this result that stands.

The syntactic inference gets a buffer of literals. It cannot get
[`tree.oro`](../../examples/json/tree.oro)'s node table, whose stores are node indices bounded by a
loop guard — `nn < 512` — and by nothing a literal can show. That is the interval analysis's fact,
and it is now used.

```
tree.oro on Go:   make([]uint16, 4*512)   ← the node table, was []int
                  make([]uint16, 2*32)    ← the parse stack
                  make([]int,    2*512)   ← the worklist, correctly NOT narrowed
```

The worklist stays wide because it stores a depth read back out of itself, which nothing bounds.
Getting that one right is as much the feature as the other two.

### The soundness argument, because this is where an analysis starts deciding bits

A wrong bound here is a **silent wrong answer**, not a slow program, so it is worth stating what
this rests on.

1. **The pass runs on the `build` lambda alone**, not the enclosing function. Less context can only
   widen an interval, never narrow one — so a subterm analysis is conservative with respect to the
   whole-program one, and anything free in the lambda is unbounded.
2. **Exact facts are used first.** A literal, a conditional over literals, and a read from an
   already-narrowed table are decided syntactically; the analysis is asked only when none of those
   settles it. So this argument carries the residue, not the whole feature.
3. **Failure is the safe direction and is the default.** An infinite endpoint answers no and the
   buffer keeps the host's word.
4. **The differential suite cannot catch a bad narrowing** — every target narrows on the same
   decision, so they agree and are wrong together. Only the `; expect:` answers can, which is the
   second time this week that half of the suite has been the load-bearing half.

So the checks are direct. `TestBufferRangeContainsEveryStore` runs five programs whose true extremes
are computed by hand and requires the claimed range to **contain** them — containment, not tightness,
because over-approximating costs space and under-approximating corrupts. Two of those cases were
written expecting a refusal and got a claim, and **the claims were right**: `0 * 3` stays 0 forever,
and `i*j` for `i < 10` with `j` stepping by 10⁹ really is under 9.9×10¹⁰. The tests were wrong, which
is the correct way round for that to go.

`TestBufferRangeRefusesWhatItCannotBound` requires a refusal for a value read out of the buffer
itself, a free variable, and a parameter of the enclosing function.

And empirically: the tree's agreement test compares the generated parser against two hand-written
implementations on documents of up to 443 nodes, where a truncated link breaks the walk immediately.

### Measured

| | before | after | |
|---|---|---|---|
| Go, tree | 6,053 ns | **5,524** | **1.10×**, and now faster than hand-written *clamped* (6,068) |
| Java, tree | 7,756 ns | **6,795** | **1.14×** |
| node table | 16 KB | **4 KB** | `4×512` slots at 2 bytes |

### A correction to json-tree-bench-2026-08-26

That result explained part of the JVM's preference for recursive descent by size: *"our 64-bit `int`
makes an emitted node 32 bytes against a `Node`'s 24 — the flat form is larger than the boxed one."*

A node is **8 bytes** now, a third of a `Node`, and **recursive still wins on the JVM** — 4,265 ns
against our 6,795 and against a hand-written `int[]` flat table's 5,330. So element width was **not**
the driver. What remains is what that result also named and should have leaned on alone: TLAB
bump-allocation, a young collector that pays for survivors when every node here dies, and C2 scalar
replacement. The headline — *flat beats pointers is a Go fact* — is unchanged; one of its three
explanations was too generous and is withdrawn.

## 6. What is not built

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
