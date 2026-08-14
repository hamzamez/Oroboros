# Gauntlet program 4 — word count, on both targets

The program that tests the **Parasite thesis** rather than the reducer. Its pass condition was
never a number: the Go output must use Go's own dictionary, and the JS output must use JS's.

**Result: the thesis passes structurally on both targets. And the duplication problem, whose
justification was withdrawn two days ago when Go's CSE turned out to erase it, comes back at
610× on Go and 1,089× on JS — with a precise criterion for when it matters.**

---

## 1. The thesis passes

Neither backend had a dictionary before this program. One source, two targets:

**Go**

```go
import "strings"

func GenWordCount(text string) map[string]int {
	acc := (make(map[string]int))
	n1 := (len((strings.Fields(text))))
	for i := 0; i < n1; i++ {
		acc[((strings.Fields(text))[i])]++
	}
	return acc
}
```

**JavaScript**

```js
export function genWordCount(text) {
	let acc = (Object.create(null));
	const n1 = ((text.split(" ")).length);
	for (let i = 0; i < n1; i++) {
		acc[((text.split(" "))[i])] = (acc[((text.split(" "))[i])] ?? 0) + 1;
	}
	return acc;
}
```

`map[string]int` with `acc[k]++` on Go — **one** hash lookup, which
[baseline C3](baseline-2026-08-13.md) verified structurally as a single `mapassign_faststr`.
`Object.create(null)` on JS — **not `Map`**, because [baseline R4](baseline-2026-08-13.md)
measured `Map` at 3.25× slower for string keys, refuting
[g4](../../docs/derivations/g4-word-count.md)'s original assumption.

Each target got the dictionary *it* is fastest with, from one source, with the difference living
entirely in the primitive table. Correctness asserted against the hand-written reference on both.

## 2. The duplication, finally costing something

The residual duplicates `(split-words text)` — once for `slen`, and once **inside the loop body**:

```lisp
(fn (text) (fold-range (dict-empty) (slen (split-words text))
             (fn (acc i) (dict-inc acc (sat (split-words text) i)))))
```

`strings.Fields` therefore runs once per iteration. Measured at n=2000 words:

| | Go | JS |
|---|---|---|
| hand-written | 75,751 ns | 197 µs |
| **generated** | **46,587,398 ns** | **214,959 µs** |
| | **615×** | **1,089×** |

And it is **quadratic**, so the ratio grows with input. 65,536 words — the size every other
gauntlet program uses — would not finish.

JS is worse than Go because `dict-inc`'s template names its operands twice, so `text.split(" ")`
appears **twice** per iteration rather than once.

## 3. The criterion, which is the actual finding

[Two days ago](duplicate-read-2026-08-14.md) the duplicated array read `a[i]` turned out to cost
**nothing** — Go's CSE eliminated it, and all three variants compiled to byte-identical machine
code. The justification for call-by-need was withdrawn.

It is now back, and the two results together give something better than either alone:

| duplicated term | cost | why |
|---|---|---|
| `a[i]` — a **pure** expression | **zero** | the host's CSE hoists it; it is referentially transparent |
| `strings.Fields(text)` — **allocates** | **615×, quadratic** | no host can hoist it; allocation is not a pure expression a compiler may move |

> **Duplication is free exactly when the duplicated term is pure, and unbounded when it is not.**

### And the asymmetry gives a simple rule

The two costs are wildly asymmetric:

- **Over**-residualizing a pure term costs **nothing** — the host's CSE undoes it.
- **Under**-residualizing an allocating term costs **615×**, growing with input.

So there is no need to classify which primitives are expensive, and no need for a cost model:

> **Residualize every primitive application.** Conservative, trivially decidable, and correct.
> Where it was unnecessary, the host cleans up for free.

That is a much smaller rule than the grade-directed classification proposed in
[concerns.md §1.1](../../docs/spec/concerns.md), and it is justified by measurement rather than
by taste. It is also [ADR 0008](../../docs/decisions/0008-measurement-over-principle.md) once
more: let the host do what the host does, and only guard what it cannot.

## 4. What the emitter needed that it did not have

Three things, all arriving by need rather than by design — which was the point of building
before specifying the binding format.

**An import.** `strings.Fields` requires `import "strings"`. This is the first emitted code that
needs one, and it is [g5](../../docs/derivations/g5-bindings.md)'s Tier 2 binding format showing
up in the emitter: *name, type, and an import line.* The current implementation is a package-level
sink, which is crude and marked as such in the source.

**A third kind of primitive.** `dict-inc` is a **statement that returns its first argument** —
`acc[k]++` mutates and yields nothing, but the fold needs the accumulator back. So the primitive
kinds are now: expression, loop, conditional, and statement-returning-an-argument. The binding
format's "expression or statement" flag needs to be a small enum, not a boolean.

**A polymorphic fold.** `fold-range`'s accumulator was hardcoded to `float64`. A dictionary
accumulator forced its result type to follow its *initialiser*. Small change, and the sort of
thing that only shows up when a second accumulator type exists.

## 5. What this does not settle

- **Call-by-need is still not implemented.** This measures the hole; it does not fill it.
- **The generated word count is unusable** at realistic sizes. Program 4 does *not* currently
  reach parity, and that is the first gauntlet program to fail.
- **`split-words` semantics still diverge.** g4 §9 found Go's `strings.Fields` trims while JS's
  `split(" ")` does not. This program uses a single-space corpus so the difference does not show,
  which means the conformance problem is deferred rather than solved.

## Reproducing

```bash
go run ./cmd/gen examples/wordcount.oro go gauntlet/go/generated_wc.go gen-word-count
go run ./cmd/gen examples/wordcount.oro js gauntlet/js/generated_wc.mjs gen-word-count
```

```bash
cd gauntlet/go && go test -bench='WCHandWritten|WCGenerated' -benchtime=2s -count=3
cd gauntlet/js && node wc.mjs
```
