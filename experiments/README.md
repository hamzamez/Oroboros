# Experiments

Work done to answer a question, kept whether or not it became a feature. Nothing here is a
deliverable; several of these will be deleted, and when they are, what they found moves into
`docs/` or `gauntlet/results/` first.

| | question | verdict |
|---|---|---|
| [go-toplevel](go-toplevel/) | can Oroboros express Go's predeclared identifiers? | mostly yes — and it found the **1117×** |
| [js-toplevel](js-toplevel/) | everything Node and the browser both provide with no import | yes, and more easily than Go |
| [java-toplevel](java-toplevel/) | `java.lang` | yes, but `java.lang` has **no collections** |
| [legibility](legibility/) | (earlier) | see its own notes |

## The three top-level experiments

Run together, deliberately, because a target model that only ever meets *portable* programs has not
been tested. Each writes non-trivial, deliberately **non-portable** programs against one host's top
level, per [ADR 0001](../docs/decisions/0001-parasite-model.md): portability is a property a program
may or may not have.

### What they agreed on

**The loop is the problem, and it is the language's.** The same sieve, three hosts:

| host | hand-written | restricted to our loop shapes | generated |
|---|---|---|---|
| Go | 17.4 µs | 19,437 µs — **1117×** | 14,974 µs (0.77× of restricted) |
| JavaScript | 49.9 µs | 22,165 µs — **445×** | 12,415 µs (0.56×) |
| Java | 20.2 µs | 21,864 µs — **1083×** | 9,573 µs (0.44×) |

`fold-range` has no start, no step and no early exit, so `for j := i*i; j < n; j += i` cannot be
said and the algorithm degrades from O(n log log n) to O(n²). **On all three hosts our emitted code
is faster than a human writing under the same constraints**, so none of the gap is the backend.

This is the largest number in the repository. It reordered
[docs/spec/loops.md](../docs/spec/loops.md).

**A product type is wanted, from three directions.** `v, ok := m[k]` on Go, "found?" alongside a
value on JS, and `fold-range2`'s two accumulators. Three independent demands for one feature.

**Mutation needed no new mechanism.** A `stmt` primitive's value is its first argument, so an
indexed write yields its container and threads through a fold like an accumulator, with the effect
discipline pinning the order.

### What they disagreed on — which is the thesis

| | Go | JavaScript | Java |
|---|---|---|---|
| types the target needs | one per element type | **none at all** | one per element type |
| `if` inside a loop | `var t; if … else …` | `(c ? a : b)` | `T t; if … else …` |
| top-level collections | map + slice | array + object | **none** |
| host's higher-order API | little | most of it — **measured 3.6×–133× *worse* than a loop** | streams, not top level |

Three hosts, three different walls, one emitter. That is
[ADR 0001](../docs/decisions/0001-parasite-model.md) doing what it claimed.

### What they found

- **A silent wrong answer.** Nested folds sharing a name hint emitted `for i :=` inside `for i :=`
  and `acc := acc`; the outer accumulator was never written. Found by the first `go/builtin`
  program, fixed in all three backends.
- **An integer accumulator that did not compile** — `acc := 0` is Go's `int`.
- **A false claim in `targets/js.oro`**: argument types said to be "never consulted" are consulted by
  the checker on every target — and caught a real bug on the host with no type layer.

Three portable-only years of gauntlet programs had exercised none of this.
