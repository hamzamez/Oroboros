# Exploration: cycles

Exploration only. No commitments, no ADR.

[s4](s4-nesting.md) closed the uniqueness story end to end and named one thing that could still
genuinely hurt: **cycles defeat reference counting outright** — the standard RC failure, and why
Koka and Lean restrict what can be cyclic. The design now leans on an RC fallback exactly where
static analysis cannot reach.

Two questions, and only the second is measurable.

**Result: cycles turn out to be unrepresentable by construction, which is what makes the RC
fallback safe rather than merely cheap. The workaround they force — arenas and indices — is
1.88× *faster* than pointers for scattered access, and slower only for graphs local enough that
pointer chasing stays in cache.**

---

## 1. Can a cycle be constructed at all?

Only two things in this design write memory:

- **Functional update**, which produces a new value and never mutates an existing one.
- **Compiler-emitted in-place update**, gated on liveness — reuse requires the old value to be
  dead ([s2](s2-multiplicity-inference.md), [s3](s3-cross-boundary-reuse.md)).

A cycle requires some value to reference itself, transitively. Take the only way that could
arise: storing `V` inside a new value `W`, where `W` reuses `V`'s storage.

> For reuse, `V` must be **dead**. But `V` is stored inside `W`, so `V` is **live**.
> Contradiction.

The liveness check that exists for performance also forbids the aliasing that would close a
cycle. Working the cases:

| Attempt | What happens |
|---|---|
| `(let a2 (with-next a a))` | `a` is stored into `a2`, so it is live; no reuse; `a2.next` is the *old* `a`. A DAG. |
| `(dict-insert d "self" d)` | Same shape. The new dict's entry is the old dict. |
| Two nodes pointing at each other | The second update finds its target referenced by the first, so grade ω, so it copies. |
| A self-recursive function | Recursion lives in the **call graph**, not the heap. Recursive functions are residual ([g3](g3-generics.md)), emitted as target functions, not as cyclic data. |

**Cycles are unrepresentable.** That is the fourth hazard this design has closed by making the
bad state unconstructible rather than by checking for it, after capture, scalar aliasing, and
slice-parameter aliasing.

## 2. Which is exactly what makes the RC fallback safe

The two decisions support each other, and neither was chosen with the other in mind:

- s3 and s4 lean on a reference-count check where static analysis cannot reach — 0–4%.
- RC's one fatal weakness is cycles.
- Cycles cannot be built, because liveness-gated reuse forbids them.

So the fallback is not merely affordable, it is **sound**. Koka and Lean have to restrict
cyclicity deliberately and carry the restriction as a language rule; here it falls out of a
decision made for a different reason entirely.

## 3. What the restriction costs

Real data structures need cycles: doubly-linked lists, graph back-edges, parent pointers,
observer patterns. Without reference cycles these become an **arena plus integer indices** —
which is exactly what Rust programs do, for the same reason.

That is a genuine language commitment. It is also, on measurement, mostly an advantage.

## 4. Measured — and a biased first attempt

131,072 nodes, degree 4, summing every neighbour's value.

**First attempt**, edges at offsets 1, 7, 53, −1:

| | ns/op |
|---|---|
| Pointer graph | 308,652 |
| Index graph | 374,698 |

Pointers won by 1.21×, contradicting the expectation. But those offsets are near-local, and Go
allocates the nodes in order, so every neighbour was already in cache. **The benchmark flattered
the pointer version by construction.**

**Second attempt**, random edges, same node count and degree:

| | ns/op |
|---|---|
| Pointer graph | 895,109 |
| **Index graph** | **476,670** |

**Indices win by 1.88×.** The cause is size: an `int32` index is half a 64-bit pointer, so the
adjacency array is half the cache footprint, and once access is scattered the working-set size
is what decides.

> Second time in this session that test data hid the answer, after the linear ramp in
> [g7](g7-aliasing.md). Both were caught only because a result looked wrong and the input was
> re-examined. Worth treating as a standing hazard rather than two incidents.

**GC cost** with each representation live — 131,072 traced objects with 524,288 outbound
pointers, versus three pointer-free slices:

| | best pause |
|---|---|
| Pointer graph live | 2.53 ms |
| Index graph live | 2.41 ms |

**1.05×.** Essentially nothing, and much less than expected. The "indices spare the collector"
argument is real but too small to lead with, at least at this size on Go's collector.

## 5. What it actually costs

Not performance. Two other things:

**Ergonomics.** `node.next` becomes `arena.nodes[node.next]`. This is the standard complaint
about arena-and-index code in Rust, and it is legitimate. A handle type with syntax support
would help; nothing here solves it.

**Safety.** An index is an unchecked reference. Holding a stale index after an arena shrinks
gives the *wrong node* rather than a crash — memory-safe but semantically wrong, which is
arguably worse than a dangling pointer because it fails silently. Generational indices are the
standard fix and cost an extra comparison. Untested here.

## 6. Findings

1. **Cycles are unrepresentable.** Reuse requires the old value dead; a value stored inside the
   new one is live. The liveness check that exists for performance forbids the aliasing that
   would close a cycle.
2. **That is what makes the RC fallback sound**, not just cheap. RC's one fatal weakness cannot
   occur. Koka and Lean carry this as a deliberate language restriction; here it falls out.
3. **Fourth hazard closed by construction**, after capture, scalar aliasing, and slice-parameter
   aliasing.
4. **Index graphs are 1.88× faster for scattered access** and 1.21× slower for near-local
   access. Net favourable, and the direction depends on locality rather than on representation.
5. **The GC argument is real but small** — 1.05×, not worth leading with.
6. **The first benchmark was biased by its edge distribution**, and the bias pointed the
   opposite way from the truth. Second occurrence of test data hiding the answer.
7. **The real costs are ergonomic and safety-related**, not performance: arena-and-index code is
   less pleasant, and stale indices fail silently.

## 7. Verdict

The last thing that could genuinely hurt does not. The uniqueness story is complete:

| Scope | Mechanism | Cost |
|---|---|---|
| Within a function ([s2](s2-multiplicity-inference.md)) | Occurrence counting plus liveness | free |
| Across boundaries ([s3](s3-cross-boundary-reuse.md)) | Multiplicity in the signature | free; RC fallback 2–4% |
| Through nesting ([s4](s4-nesting.md)) | Grading tracks heap fields | free; RC fallback 0–3% |
| **Cycles (here)** | **Unrepresentable** | 1.88× faster to 1.21× slower, plus ergonomics |

And the RC fallback the last three lean on is sound precisely because of the first column of the
fourth row.

**What remains untested in this whole thread:** generational indices, and whether the ergonomic
cost of arena-and-index code is acceptable in practice — which is not a benchmark question but a
legibility one, and therefore belongs with the **rule legibility** experiment that is still
outstanding in the [assessment](../assessment-2026-08-13.md). Those are now the same question
wearing two hats: *can a person or a model read code written this way?*
