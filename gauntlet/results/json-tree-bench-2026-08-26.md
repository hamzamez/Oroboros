# The tree against hand-written code — 2026-08-26

[`examples/json/tree.oro`](../../examples/json/tree.oro) against hand-written Go, JavaScript and
Java. References in [`gauntlet/go/jsontree.go`](../go/jsontree.go),
[`gauntlet/js/jsontree.mjs`](../js/jsontree.mjs),
[`gauntlet/java/JsonTreeBench.java`](../java/JsonTreeBench.java).

Two questions, and they are different:

- **the representation** — a flat node table plus indices against recursive descent into linked
  objects, which is what ADR 0014 and [data-structures.md](../../docs/data-structures.md) rest on;
- **our code generation** — the emitted flat table against a hand-written one, which is
  requirement 5.

Both hand-written forms are carried on every host, per the rule that has refuted five beliefs so
far. **It refuted another one.**

---

## 1. The headline: the flat table is a GO fact

| | recursive, boxed | flat, indexed | flat vs recursive |
|---|---|---|---|
| **Go** | 11,370 ns · 443 allocs | 4,512 ns · 0 allocs | **flat wins 2.52×** |
| **JavaScript** | 10,940 ns | 8,957 ns | flat wins **1.22×** |
| **Java** | **4,317 ns** | 5,343 ns (`int[]`) | **flat LOSES 1.24×** |

data-structures.md says *"recursive data is a flat table plus indices, 2.02× faster on irregular
access"*, and ADR 0014 leans on it. **On the JVM that is false for this program**, and not
marginally: recursive descent into `Node` objects beats the best hand-written flat table by 1.24×,
and beats our emitted one by 1.80×.

The reasons are all things the JVM does and Go does not: allocation is a pointer bump in a
thread-local buffer, the young collector pays for *survivors* rather than for garbage and every node
here dies, and C2 can scalar-replace what does not escape. On top of that our `int` is 64-bit
(ADR 0012), so an emitted node is **32 bytes against a `Node`'s 24** with compressed oops — the flat
form is *larger* than the boxed one, which is not true on Go.

This is [ADR 0008](../../docs/decisions/0008-measurement-over-principle.md) landing on a decision
rather than on a primitive. *Flat beats pointers* was a measurement on one host that had become a
principle.

---

## 2. Our code generation, with the representation held fixed

### Go — `-benchtime=3s -count=7`, median

| | ns/op | allocs | |
|---|---|---|---|
| hand-written recursive | 11,370 | 443 | |
| hand-written flat, no clamps | 4,512 | 0 | traps on a bad index |
| hand-written flat + clamps | 6,067 | 0 | **our shape** |
| **generated** | **6,053** | **0** | **1.00× of our shape** |

**Parity with the shape we emit**, and **1.88× faster than idiomatic recursive descent** with zero
allocations against 443.

### JavaScript — one `node` process per case, median of 3

| | ns/op |
|---|---|
| hand-written recursive | 10,940 |
| hand-written flat | 8,957 |
| **generated** | **9,477** — **1.06×** |

### Java — best-of-9, median of 3 processes

| | ns/op | |
|---|---|---|
| hand-written recursive | 4,317 | |
| hand-written flat, `int[]` | 5,343 | what a person writes |
| hand-written flat, `long[]` | 6,357 | our element width — **1.19×** |
| **generated** (`long[]`) | **7,773** | **1.22× of our shape, 1.80× of recursive** |

Java's miss decomposes into three separable costs, none of which is "the emitter is bad": the
representation loses on this host (1.24×), our 64-bit element costs 1.19×, and code generation plus
the remaining clamps and [indextype-2026-08-25](indextype-2026-08-25.md)'s `(int)` casts cost 1.22×.

---

## 3. The clamps cost 1.35×, so the compiler learned to prove instead

The first version of `tree.oro` clamped **every** index — `(go.* 4 (cn k))` is in range by
construction — because the refinement layer would not discharge the bound otherwise. Priced by
hand-writing the same program with and without the clamps:

| Go | ns/op |
|---|---|
| hand-written flat, no clamps | 4,512 |
| hand-written flat, all clamps | 6,067 |

**1.35×**, which is a bigger number than it looks: the tokeniser's three extra compares
([jsontok-2026-08-26](jsontok-2026-08-26.md)) cost nothing measurable, because a never-taken branch
is what a predictor is best at. A clamp is not a branch — it is a **data dependency in the address
computation**, and the load cannot start until it resolves.

So the clamps were worth removing, and the reason they existed turned out to be a **missing
inference, not a missing fact**:

```
(stk (go.+ (go.* 2 (go.- sp 1)) 1)) is an indexing, and
(<= 0 (go.+ (go.* 2 (go.- sp 1)) 1)) does not follow
  known: … assumed sp + -31 <= 0, assumed -sp + +1 <= 0
```

It knows `sp >= 1`. The goal is `2*sp - 1 >= 0`. `entails` matched a fact against a goal by
requiring **identical coefficients**, so a fact with `sp` could not discharge a goal with `2*sp`.

**One Farkas multiplier** fixes it: scale a fact by a positive integer when that makes the
coefficient vectors match. Sound, ten lines, capped so `konst*m` cannot overflow.

**A stride is exactly the shape this missed**, and that is why nothing found it before: `(go.* 4 k)`
has a coefficient the guard bounding `k` does not, and no program in this repository had a strided
index until a node table.

What it bought:

| `examples/json/tree.oro` | undischarged obligations | Go ns/op |
|---|---|---|
| clamp everywhere | 110 | 6,331 |
| **prove where possible, clamp what is opaque** | **40** | **5,668**¹ |

¹ measured in one session against 4,277 for the unclamped hand-written form; §2's table is a later,
cleaner run of the same comparison. Both agree on the ratio.

**Fewer clamps left FEWER undischarged obligations, not more** — a clamp hides the fact instead of
establishing it. And the clamps that remain are real: a node index read *out of the table* is data,
so no guard would help.

It changes nothing on the sieve, the tokeniser, `dot` or the stencil. That is the honest scope: it
is what a stride needs and nothing else in the corpus has one.

---

## 4. The allocating version's variance is itself a result

Measured alone, the recursive Go version is stable at 11,370 ns. Measured in the same process as the
other three, it ran **19,168 to 36,495** across seven runs — a 1.9× spread on identical work,
because 443 allocations per call put it at the mercy of whatever else is on the heap.

The flat versions do not move, because there is nothing to collect. **0 allocations means the number
is the number**, and that is a property worth more than a ratio in a program that is one part of
something larger. It is also the strongest remaining argument for the flat table on a host where the
timing alone does not make one.

---

## 5. Method

Hybrid P/E-core laptop, ~15% noise floor. Absolute times are not comparable across sessions — one
run of §2's Go table came in 2.7× slower across every row after the differential suite had been
building for a few minutes — so every comparison here is between rows measured back to back, and
ratios are what is reported.

All three implementations are checked against each other on five document sizes and eight edge cases
before anything is timed, on every host. The document is 20 records, ~2.6 KB, about 443 nodes.

The JavaScript and Java sources are derived from the one Go-flavoured `.oro` by substituting the
host prefix, as the differential suite does.

x86-64 is not measured. Its correctness is covered by the differential suite.
