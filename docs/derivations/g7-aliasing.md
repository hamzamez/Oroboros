# Derivation: gauntlet program 6 — mutation through an aliased slice

Exploration only. No commitments, no ADR.

[s2](s2-multiplicity-inference.md) found multiplicity inference works with zero annotations in
application code, then named its own hole: **no gauntlet program mutates a shared structure**,
and `dot(v, v)` shows slices *can* alias. That is where uniqueness stops being free and where
Rust's difficulty actually begins. Per
[ADR 0008](../decisions/0008-measurement-over-principle.md) it was built rather than reasoned
about.

**Result: three measured surprises, all of which point the same way, plus an asymmetry between
slices and heap structures that would be wrong to treat uniformly.**

---

## 1. What it tests

Two sub-cases, deliberately different in kind.

**6a — a stencil, where aliasing changes the *answer*.** This is the hazard C's `restrict`
exists for, and **none of Go, JS, or Java can express that two slices are disjoint.**

```go
func Smooth(dst, src []float64) {                 // naive
	for i := 1; i < len(src)-1; i++ {
		dst[i] = (src[i-1] + src[i] + src[i+1]) / 3
	}
}

func SmoothNoAlias(dst, src []float64) {          // window carried in registers
	a, b := src[0], src[1]
	for i := 1; i < len(src)-1; i++ {
		c := src[i+1]
		dst[i] = (a + b + c) / 3
		a, b = b, c
	}
}
```

**6b — the price of failing to prove uniqueness**: a defensive copy of a dict, versus updating
in place.

## 2. The hazard, demonstrated

`gauntlet/go/aliasing_test.go` asserts the two forms agree when disjoint and **disagree when
aliased** — if they agreed, the test would not be exercising anything.

```
disjoint (both):  [_, 4.667, 9.667,  16.667, 25.667, 36.667, 49.667, _]
aliased naive:    [1, 4.667, 9.889,  16.963, 25.988, 36.996, 49.999, 64]
aliased register: [1, 4.667, 9.667,  16.667, 25.667, 36.667, 49.667, 64]
```

The naive form drifts, because at *i* it reads `src[i-1]`, which it already overwrote. The
register form silently computes **the disjoint answer** even though the slices alias.

So a legitimate, standard optimization changes program results depending on a caller-side fact
the language cannot see.

> Aside worth keeping: the test failed on first run because the input was `1..8`, and a 3-point
> mean is the *identity* on a linear ramp — both forms agreed for the wrong reason. Test data
> can hide the thing the test exists to find.

## 3. Three surprises

Medians, n=65536, all three targets:

| | Go | JS | Java |
|---|---|---|---|
| disjoint, naive | 63,692 ns | 94,578 ns | 64,107 ns |
| disjoint, registers | 63,768 ns (**0%**) | 84,542 ns (−11%) | 63,501 ns (−1%) |
| **in-place (aliased)** | **378,810 ns (5.9×)** | **435,155 ns (4.6×)** | **395,926 ns (6.2×)** |
| fresh allocation | 107,772 ns (1.7×) | 249,757 ns (2.6×) | 132,782 ns (2.1×) |

**Surprise 1 — carrying the window in registers buys nothing.** 0% on Go, 1% on Java, 11% on
JS. The assumption going in was that aliasing-conservative codegen costs real performance, which
is the entire premise of `restrict`. Here it does not: the loop is bottlenecked elsewhere.

**So the aliasing hazard is a correctness problem, not a performance problem.** Being
conservative is free.

**Surprise 2 — in-place is 4.6–6.2× *slower* than out-of-place.** On every target. This is a
store-to-load forwarding stall: writing `dst[i]` and reading it back as `src[i-1]` on the next
iteration serializes the memory pipeline, and the CPU cannot run ahead.

It appears identically on three unrelated runtimes, so it is a **hardware effect, not a language
effect** — and it inverts the usual intuition that mutating in place saves time.

Not to be over-generalized: this is about a *window*, where the write at *i* is read at *i+1*.
For elementwise work (`xs[i] = f(xs[i])`) there is no cross-iteration dependency and in-place is
fine.

**Surprise 3 — allocating a fresh destination costs only 1.7–2.6×** versus reusing a
pre-allocated buffer. Real, but bounded, and smaller than the in-place penalty it avoids.

## 4. The cost of a uniqueness false negative

What the compiler must emit when it *cannot* prove a structure is uniquely referenced:

| Go | ns/op | B/op | allocs |
|---|---|---|---|
| dict insert, in place | 8.0 | 0 | 0 |
| dict insert, copying (8 entries) | 319.8 | 504 | 4 |
| dict insert, copying (~500 entries) | 12,365 | 27,352 | 4 |
| slice copy (65,536 × f64) | 33,393 | 524,288 | 1 |

**40× at eight entries. 1,540× at five hundred.** The penalty is O(n) in the structure while the
operation it replaces is O(1), so it is **unbounded in the size of the data.**

Compare with the other costs this project has measured — an escaping closure at 16 bytes, a
strict reduction at 5–7×. Those are constant factors. **This one is not bounded at all**, which
puts it in a different severity class from anything found so far.

## 5. The asymmetry

The two sub-cases pull in opposite directions, and treating them uniformly would be a mistake:

| | Slices / arrays | Heap structures (dict) |
|---|---|---|
| Cost of being conservative | ~0% (surprise 1), plus 1.7–2.6× if a fresh buffer is needed | **40×–1,540×, unbounded** |
| Is reuse desirable? | **No** for windowed access — 4.6–6.2× slower *and* changes the answer | **Yes, always** |
| Where uniqueness must be proven | Rarely | **Reliably** |

The cause is a ratio: copying a slice is a contiguous scan at ~0.5 ns/element, while copying a
dict rehashes every entry at ~25 ns/entry — fifty times more per element — and it replaces an
8ns insert rather than a full pass.

> **The cost of a uniqueness false negative is the size of the structure divided by the work
> being done.** For a full pass over a slice it amortizes to a constant factor. For a single
> dict insert it is catastrophic.

## 6. What this says about the design

**Forbid mutable slice parameters.** Mutation is expressed as producing a value, and the
compiler chooses reuse by liveness. Then parameter aliasing is *impossible by construction* —
the same move that made capture impossible (representation) and value aliasing impossible
(value semantics). Program 6 is the third time this design has closed a hazard by making the
bad state unrepresentable rather than by checking for it.

Surprise 1 is what makes this affordable: being unable to assume disjointness costs nothing, so
the safety is free. Surprise 2 makes it *better than* the alternative for windowed code, since
the in-place version the programmer might have hand-written is 5–6× slower.

**Uniqueness inference for heap structures must be reliable, not best-effort**, and where it
fails the diagnostic must be loud. [g6 §9](g6-escaping-closures.md) proposed surfacing
per-abstraction cost; program 6 says one line of that report is in a different class from the
others:

```
counts    could not prove unique — copying on each update, O(n)
```

That is not a footnote next to "16-byte environment." It is the difference between a program
that works and one that does not.

## 7. Does s2 survive?

Yes, and its mechanism is confirmed. Liveness does decide the case correctly:

```lisp
(let d (dict-empty))
(let e d)                      ; d's value is live after the update below
(let d2 (dict-insert d "x" 1)) ; must copy — e still refers to the old value
(lookup e "x")
```

At the update, `d`'s old value is still live (via `e`), so in-place is unsound and the copy is
forced. In [g4](g4-word-count.md)'s accumulator the old value is dead immediately, so reuse is
taken. Standard last-use analysis, exactly as Perceus does it.

What s2 got wrong was not the mechanism but the **risk assessment**: it recorded liveness as
"textbook, beyond counting" without pricing a failure. The price is unbounded.

## 8. Findings

1. **The aliasing hazard is a correctness problem, not a performance one.** Aliasing-conservative
   codegen costs 0% on Go, 1% on Java, 11% on JS — refuting the premise behind `restrict`, at
   least for this shape of loop on this hardware.
2. **In-place mutation is 4.6–6.2× slower** than out-of-place for windowed access, on all three
   targets. A hardware store-to-load stall, not a language artifact. Inverts the usual intuition.
3. **A legitimate optimization silently changes the answer under aliasing**, and no target
   language can express disjointness.
4. **Fresh allocation costs only 1.7–2.6×**, less than the in-place penalty it avoids.
5. **A uniqueness false negative on a heap structure costs 40×–1,540×**, unbounded in structure
   size — a different severity class from every other cost measured in this project.
6. **Slices and heap structures pull opposite ways.** Conservative is free for slices and
   catastrophic for dicts.
7. **Forbidding mutable slice parameters closes the hazard by construction**, and surprise 1
   makes it free while surprise 2 makes it faster than what a programmer would hand-write.
8. **s2's mechanism survives; its risk assessment did not.**

## 9. Verdict

The hole s2 named is filled, and the answer is better than expected in one direction and worse
in the other.

Better: for slices — the case that looked most like Rust's hard problem — the language can
simply remove the hazard, and measurement says the removal is free. There is no borrow checker
here because there is nothing to borrow.

Worse: for heap structures, uniqueness is not an optimization but a correctness-adjacent
requirement with an unbounded penalty for failure. That is the one place this design genuinely
needs to be *good* rather than merely sound, and it is where the next real risk lives.

**Untested and now the obvious next thing:** whether liveness-based reuse stays reliable across
function boundaries. Everything derived so far is intraprocedural. A dict threaded through
several calls, or stored in a struct, is where Perceus-style reuse gets hard — and by finding 5,
getting it wrong there is 1,540× rather than 2×.
