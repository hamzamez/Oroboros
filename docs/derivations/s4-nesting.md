# Exploration: a heap structure stored inside another structure

Exploration only. No commitments, no ADR.

The last known gap in the uniqueness story, named by
[s3](s3-cross-boundary-reuse.md): everything derived so far threads values through calls, but a
dict held in a *struct field* is reachability rather than liveness.

**Result: it collides with [g2](g2-structs.md)'s value semantics and forces a narrowing of that
decision. But the case s3 predicted would be hardest — a dynamic index — turns out to cost
nothing to handle at runtime. The hardest case for the analysis is the cheapest case for the
fallback.**

---

## 1. The collision

[g2 §4](g2-structs.md) decided: **structs are values, always copied, never referenced, no
interior pointers.** That was justified for `point` — two `f64`s — and it is what makes SROA
unconditional and makes aliasing a struct local impossible.

Once a field is a heap reference, "always copied" has two possible meanings that differ by
orders of magnitude:

```go
type Cache struct {
	Entries *Boxed   // heap
	Hits    int      // scalar
}
```

| Copying `Cache` | ns/op | allocs |
|---|---|---|
| **Shallow** — copy the reference | **0.72** | 0 |
| **Deep** — copy the dict too (8 entries) | 202.7 | 3 |
| **Deep** — copy the dict too (~500 entries) | 11,771 | 5 |

**281× at eight entries. 16,300× at five hundred.** Value semantics taken literally is
untenable the moment a struct holds a heap reference.

## 2. The resolution

Copying a struct copies its fields' **references**, and the grading tracks the referenced
structure. A struct copy is shallow — 0.72ns — and it counts as a *use* of every heap-typed
field, so the field's multiplicity rises to ω.

That is not a new mechanism. It is what the grading already does:

```lisp
(let c2 c)                      ; a use of c, hence of c.entries
(dict-insert (.entries c) "x" 1) ; second use -> entries is ω -> must copy
(lookup (.entries c2) "x")       ; correctly does not see the insert
```

And when the original is dead afterwards, the grade stays 1 and the copy becomes a move — free.
So rebinding costs nothing, and a copy happens only when the program genuinely needs two
independent values. Rust reaches the same place with `Clone` versus move; here it is inferred
rather than declared.

## 3. g2's claim, narrowed

> ~~Value semantics with no interior pointers makes SROA unconditional and aliasing
> impossible.~~

**True for scalar fields. False for heap-typed fields**, where two struct copies share the
referenced structure and the grading has to carry the weight instead.

SROA survives intact — splitting a struct into per-field locals works whether a field holds a
`f64` or a pointer. What does not survive is the alias-freedom guarantee, which was the reason
[s2 §4](s2-multiplicity-inference.md) could say value-typed accumulators get uniqueness *free*.
That freeness is now scalar-only.

This is the third narrowing of g2 — after the AoS penalty turning out to be JS-only, and Go
performing SROA itself. The derivation's core result (no boxing, scalarization by ordinary
layer-decreasing rules) is untouched each time; its *generalizations* keep being too broad.

## 4. The hard case is free

s3 predicted the failure point: a struct in a slice at a runtime index, where no static analysis
can distinguish `cs[i]` from `cs[j]` because `cs[i]` is not a variable and liveness works over
variables.

| Go | ns/op |
|---|---|
| `cs[i].Entries.m[k] = v` — direct | 12.4 |
| same, guarded by a sharing check | **10.1** |

**The check costs nothing measurable** — it landed inside noise, and on the faster side of it.
Same at one level up:

| | ns/op | overhead |
|---|---|---|
| Mutate through a field, direct | 12.0 | — |
| Mutate through a field, copy-on-write check | 12.4 | **+3.4%** |

So the case that defeats static reasoning is handled by a runtime check that is, at this
granularity, free. **The design does not need separation logic, access paths, or a borrow
checker for nested structures. It degrades to a check nobody can measure.**

## 5. But the outcome is not free

The *check* is free; the *copy it may trigger* is not.

| | ns/op | vs direct |
|---|---|---|
| `grid.Rows[r][c] = v` direct | 1.95 | — |
| copy the row and the row-vector | 573 | **294×** |

So a spurious sharing verdict still costs 294×–16,300×. The static analysis remains worth doing
— not to make the check cheap, which it already is, but to keep it **answering *unshared***.

That is [s3](s3-cross-boundary-reuse.md)'s negative result arriving from a second direction:
runtime machinery makes uniqueness *decidable* cheaply; it does not make it *true*.

## 6. A parity problem worth naming

[g7 §6](g7-aliasing.md) recorded that all three hosts are garbage collected, never reuse for a
functional update, and that **hand-written Go mutates — so that is the bar.**

Nested structures create a case where our semantics costs something the bar does not pay. A Go
programmer holding two references to a cache shares one map and mutates it; there is no copy
because Go does not offer value semantics over it. A program that asks for two *independent*
caches pays 281× in our language and would pay the same in Go if written to mean the same thing
— but the Go programmer would simply not write it.

So the honest statement is that we lose to hand-written Go on programs whose *semantics* differ,
not on programs that do the same work. That is defensible, and it is the first place in the
project where the gauntlet's "parity with hand-written" standard needs the qualifier **at equal
semantics**.

Whether that qualifier is acceptable is a real question and is not settled here.

## 7. Findings

1. **Value semantics is untenable for heap-typed fields.** Deep copy costs 281×–16,300×;
   shallow costs 0.72ns.
2. **The resolution needs no new mechanism** — a struct copy is a use of its heap fields, so the
   grading raises them to ω, and a copy happens only when the program means two independent
   values. Dead-original rebinding stays free.
3. **g2's alias-freedom guarantee is scalar-only.** SROA survives; the free uniqueness that
   [s2](s2-multiplicity-inference.md) leaned on does not extend to heap fields.
4. **The dynamic-index case costs nothing to check** — within noise at one level, +3.4% at
   another. No separation logic, access paths, or borrow checker required.
5. **A spurious sharing verdict still costs 294×–16,300%.** Cheap checks do not remove the need
   for a good analysis.
6. **The gauntlet's parity standard needs the qualifier "at equal semantics."** Hand-written Go
   would not write the program that costs us 281×.
7. **Third narrowing of g2.** Its core result keeps holding; its generalizations keep being too
   broad. Worth treating as a pattern rather than three separate corrections.

## 8. Verdict

The uniqueness story is now closed end to end: intraprocedural
([s2](s2-multiplicity-inference.md)), across boundaries
([s3](s3-cross-boundary-reuse.md)), and through nesting (here). The shape that emerged is
consistent across all three:

> **Static grades decide the statically-nameable cases. A runtime check decides the rest, and
> costs 0–4%. Neither replaces the other — the check is cheap, but only the analysis keeps its
> answer *unshared*.**

The cost recorded against the design: g2's value semantics narrows to scalars, and the parity
standard gains a qualifier. Neither is fatal; both are the kind of thing that would have been
discovered much later and much more expensively if the structure had been built first.

**What is not tested:** cycles. A structure reachable from itself defeats reference counting
outright — the standard failure of RC, and the reason Koka and Lean restrict what can be cyclic.
Nothing in the gauntlet builds a graph. Given that the design now leans on an RC fallback in
exactly the cases static analysis cannot reach, this is the next thing that could genuinely
hurt.
