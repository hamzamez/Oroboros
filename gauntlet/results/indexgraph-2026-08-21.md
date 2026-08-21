# Pointers against indices, 2026-08-21

`gauntlet/go/cycles.go` has carried `SumPointerGraph` against `SumIndexGraph` since the derivation
rounds and **no result file ever cited it**. It was named as a gap in
[data-structures.md §3.1](../../docs/data-structures.md) — *"the cost argument against cons cells is,
here, still an argument"* — and it is the number behind hamza's observation that the real cost of
refusing recursion is the choice of data structure, not the choice of control flow.

Measured, pinned, medians of 5.

| | pointer graph | index graph | |
|---|---|---|---|
| **Z1 — neighbours allocated in order** | 301,588 ns | 430,249 ns | index **1.43× slower** |
| **Z2 — random edges** | 996,299 ns | 494,065 ns | index **2.02× faster** |

**Both directions, and the condition is the access pattern.**

`cycles.go` anticipated it in a comment before anyone ran it: *"the neighbours land in cache. A
graph with random edges is the honest test."* Z1 builds nodes in a loop, so Go's allocator lays
them out contiguously and "pointer chasing" is really sequential memory access — while the index
form does **two dependent loads** (`g.Vals[g.Adj[base+d]]`) where the pointer form does one. Z2
scatters the edges, every chase becomes a cache miss, and the flat form's contiguous adjacency array
wins by 2×.

## What this decides

**Flat-and-indexed is not universally faster, and neither is pointer-chasing.** The honest claim is
the conditional one:

> An arena plus integer indices wins when the traversal is **irregular** — a real graph, a parsed
> document, a scattered object web — and loses when the allocator has already laid the structure out
> in traversal order.

Z2 is the realistic case for the shapes that motivate it: a JSON document, a DOM, a compiler's AST,
an entity-component world. Those are built once and traversed in orders the allocation order does
not predict.

This is the same shape as [bce-2026-08-15](bce-2026-08-15.md) (a 1.96× win that vanished on
memory-bound loops) and [checkcost-2026-08-19](checkcost-2026-08-19.md) (a cost that was wrong in
both directions). **Neither a saving nor a price survives being quoted without its condition**, and
this project has now been caught by that three times.

## Why it matters here

It prices the alternative to recursive data. A JSON value is `Null | Bool | Num | Str | Array of
JSON | Object of (String × JSON)` — an inductive type, which
[data-structures.md §1.2](../../docs/data-structures.md) says this language cannot have. The
industry answer is not recursion: it is **a flat table of nodes with integer indices**, which is
what simdjson's tape, Zig's AST (`MultiArrayList` plus `u32` indices), an ECS world, and a column
store all are.

So the structure this language *already chose* — tables indexed by integers — is the one that makes
recursive data unnecessary, and on the realistic access pattern it is **2× faster than the pointer
form it replaces.**
