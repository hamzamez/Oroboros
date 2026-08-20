# The gauntlet moves to a native target, 2026-08-20

The one piece of process debt named in every assessment since 2026-08-19: seven gauntlet programs
running on `num/vec`, `num/int`, `num/f64` and `io` — the portable layer the native targets
replaced and that nothing had moved off. **All six have moved.** This is what moving them found.

The library did not change. A delayed vector is three lines written *in the language*, not a
primitive, so moving targets moves only the names it calls: `alen` becomes `go.len`, `aindex`
becomes `go.at-float64`, `fold-range` becomes a `loop`.

## 1. `dot` — the fusion survives

`examples/native/dot-go.oro`. The residual is a single loop and the emitted Go is
`acc + a[i]*b[i]` with no intermediate structure.

| | ns/op |
|---|---|
| hand-written, best form | 485 |
| hand-written `DotRange` | 595 |
| **emitted, native** | **485** |

Parity. Pinned to one core — see §9.

## 2. `search` — early exit, on a target with no portable layer

`examples/native/search-go.oro`. ADR 0015 claimed `loop` buys early exit at parity; that was
measured on the portable layer and never re-checked.

| | early exit (ns) | late exit (µs) |
|---|---|---|
| hand-written `FindFirstRef` | 2.67 | 34.91 |
| **emitted, native** | **2.59** | **34.87** |

**0.97× and 1.00×.** The claim holds off the portable layer.

## 3. The stencil — and this is the one that matters

ADR 0013 is the project's one accepted failure: **1.79× on Go** against a hand-written
buffer-reusing form, because `num/vec.materialize` allocates fresh so nothing can alias.

`examples/native/smooth-go.oro` carries **both forms**, which is the rule for adding to the
gauntlet — the one expected to win and the one expected to lose:

- `smooth` allocates its own destination (`go.make-float64`, then a `loop` of `go.set-float64`);
- `smooth-into` takes the destination as a parameter and writes through it.

Both reduce to the same loop. The delayed vector is gone entirely:

```go
func NativeSmoothInto(dst []float64, a []float64) []float64 {
	d := dst
	var i int = 0
	for {
		if i >= ((len(a)) - 2) { break }
		d[i] = ((((a[i]) + (a[(i + 1)])) + (a[(i + 2)])) / 3.0)
		i = (i + 1)
		continue
	}
	return d
}
```

### Measured, 100,000 elements, pinned, medians of 5 and of 7

**Allocating shape:**

| | ns/op | allocs |
|---|---|---|
| hand-written `SmoothFresh` | 266,169 | 1 |
| **emitted `NativeSmooth`** | **246,900** | 1 |

**0.93× — the emitted form is faster than the hand-written one.**

**Buffer-reusing shape:**

| | ns/op | allocs |
|---|---|---|
| hand-written `Smooth` (naive) | 98,046 | 0 |
| hand-written `SmoothNoAlias` (window in registers) | 98,878 | 0 |
| **emitted `NativeSmoothInto`** | **97,939** | 0 |

**0.999× against the naive form and 0.990× against the register-carrying one.** Parity, and
fastest of the three.

### The finding

**The 1.79× is the price of the SHAPE, not of the compiler — and hand-written code pays it too.**

Within this run, allocating costs **2.71× for hand-written code** (266,169 / 98,046) and **2.52×
for emitted code** (246,900 / 97,939). In each shape separately, emitted matches hand-written.

ADR 0013 already said the compiler is not what loses. What is new is the other half: **on a native
target the programmer can decline to allocate, and the code that declines is at parity.**
`go.set-float64` is Go's own store, carries no portability claim and no aliasing guarantee, and
that is the parasite model working exactly as designed — the portable layer names its price
([construction.md](../../docs/spec/construction.md)) and a program that cannot pay it drops one
layer and writes the store itself.

None of ADR 0013's four reopening triggers fired. This is a fifth thing they did not name, and it
does not reverse the decision: `num/vec.materialize` still costs what it costs. It changes the
**consequence**. "Oroboros is a language in which nothing can alias" is true of the portable layer
and false of the native targets.

### The compiler's decisions, not just the clock

`-gcflags=-d=ssa/check_bce/debug=1`:

| | `IsInBounds` |
|---|---|
| hand-written `Smooth` | 1 |
| hand-written `SmoothNoAlias` | 2 |
| emitted `NativeSmoothInto` | 1 |
| **emitted `NativeSmooth`** | **0** |

The allocating emitted form has **fewer bounds checks than any hand-written form here**, because
`make([]float64, len(a)-2)` and the guard `i < len(a)-2` are the same expression and Go's prover
sees it.

And `SmoothNoAlias`'s register-carrying trick — the thing you would write if you knew the slices
were disjoint — buys **nothing** here: 98,878 against the naive 98,046. Memory-bound, which is
exactly the condition [bce-2026-08-15](bce-2026-08-15.md) attached to its own 1.96×.

## 4. `wordcount` — the target could not SAY it, and then it could

Gauntlet program 4's pass condition was never a number: the Go output must use
Go's own `map[string]int` and Go's own splitting. On the portable layer that took three names —
`dict-empty`, `dict-inc`, `split-words` — whose whole job was to hide which host construct was
chosen.

**The native Go target had no string surface at all.** `builtin.oro` declared `s<` and
`string-of-bytes` and nothing else, so a program that wanted to split text could not move.
`targets/go/strings.oro` is the fix — 25 primitives covering Go's `strings`, written as data with
no compiler change. What it *cannot* express is the finding, and it is recorded in the file:
`Cut` returns three results (the fourth demand for a product), `Builder` is a struct with methods,
and every `…Func` variant takes a callback, which is an escaping closure.

The emitted code is what you would write:

```go
func NativeTally(text string) map[string]int {
	ws := (strings.Fields(text))
	m := (make(map[string]int))
	var i int = 0
	for {
		if (i >= (len(ws))) { break }
		w := (ws[i])
		m[w] = ((m[w]) + 1)
		i = (i + 1)
		continue
	}
	return m
}
```

| | ns/op | |
|---|---|---|
| hand-written `counts[w]++` | 2,578,680 | |
| hand-written `counts[w] = counts[w] + 1` | 3,135,585 | |
| hand-written get-then-set | 3,149,581 | |
| emitted, unfused | 3,079,156 | **1.19×** against `++` |
| **emitted, fused** | **2,564,185** | **0.995×** |

**Parity with the identical hand-written form, and a 1.19× gap to Go's fused `m[k]++` —
closed by one declared primitive.**

`m[k]++` is a single `mapassign` returning a value pointer; `m[k] = m[k] + 1` is a `mapaccess`
*and* a `mapassign`, hashing the same key twice. The g4 derivation claimed exactly that and never
measured it. Declaring `(prim inc-map (map-string-int string) map-string-int stmt "%s[%s]++")` is
the governing rule doing its job — **emit at the highest layer the target natively provides** — and
it needed no compiler change, because primitives are declared in `targets/*.oro`.

### And the first draft of the program asserted the answer

`examples/native/wordcount-go.oro` originally carried one form and a comment reasoning that the
host's clever API would be *slower*, citing the baseline where Java's fused `merge` loses **2.6×**
to unfused `getOrDefault`+`put`.

That reasoning is right about Java and wrong about Go, and it violated the rule it cited:
**never assert which host construct is fastest — measure it**
([ADR 0008](../../docs/decisions/0008-measurement-over-principle.md)). The same fusion is worth
+1.19× on one host and −2.6× on another, which is precisely why this is a per-target declaration
rather than a principle. The program now carries both forms, which is what the rule for adding to
the gauntlet says to do in the first place.

## 5. `generic` — instantiation with no monomorphization pass

`examples/native/generic-go.oro`. g3's claim is that a non-recursive definition **is** a rewrite
rule (and since [ADR 0014](../../docs/decisions/0014-recursion-is-not-in-the-language.md) there is
no other kind), so instantiation is a side effect of matching and there is no monomorphization
pass.

The portable form could not fully test it: `aindex` and `sat` are different names for the same
shape, so matching could have been keying on the name. Here the two instantiations reach
`go.at-float64` and `go.at-string` — genuinely different host functions over `[]float64` and
`[]string` — and the two `combine`s are not even the same **shape**: one is an expression
(`acc + a[i]`) and one is a statement (`acc[ws[i]]++`).

| | ns/op | |
|---|---|---|
| hand-written `SumF64` | 33,273 | |
| **emitted `NativeSumOf`** | **32,678** | **0.98×** |
| hand-written `WordCountIncr` | 2,555,386 | |
| **emitted `NativeWordTally`** | **2,547,762** | **0.997×** |

One definition, two instantiations, both at parity, no pass.

## 6. `centroid` — and `fold-range2` disappears

`examples/native/centroid-go.oro`. The Church-encoded point — a function handing its two fields to
a selector — reduces away completely, and the residual is two scalar accumulators:

```go
ax, ay, i = (ax + (xs[i])), (ay + (ys[i])), (i + 1)
```

33,650 ns against hand-written `CentroidSumRef` at 33,316 — **1.01×**, inside the noise floor.
Every form measured here lands between 32.2 and 34.4 µs.

**What moving this program removed is `fold-range2`.** It existed for exactly one reason — two
accumulators and no product to pair them with — and its *finisher* existed so that compound loop
state never escaped the loop as a compound value. `loop` has n variables and no product at all
([ADR 0015](../../docs/decisions/0015-loop-and-again.md)), so the finisher is just the last clause
and the whole construct is gone. One fewer structural primitive a target author has to implement,
and the clearest thing the migration has *removed* from the language surface.

## 7. `report` — the only program whose pass condition is not a number

`examples/native/report-go.oro`, built and run with `cmd/build`. Three claims at once, and all
three hold:

```go
func main() {
	dst := (make([]float64, 1000))
	d := dst
	var i int = 0
	for { if (i >= 1000) { break }; d[i] = (float64(i)); i = (i + 1); continue }
	xs := d
	fmt.Println("report")
	var v1 int = (len(xs))
	fmt.Println(v1)
	acc := 0.0
	var i2 int = 0
	for {
		if (i2 >= (len(xs))) { break }
		acc, i2 = (acc + ((xs[i2]) * (xs[i2]))), (i2 + 1)
		continue
	}
	fmt.Println(acc)
}
```

1. **A host binding costs nothing** — `fmt.Println` is one line in `targets/go/fmt.oro` and no Go.
2. **Order survives reduction** — the three lines print in the order written, once each. `seq` is
   sugar for a β-redex with an unused binder and survives only because weakening is denied for an
   impure argument ([effects.md §5](../../docs/spec/effects.md)); `fmt.Println` declares no `pure`,
   so its result — used zero times — cannot be dropped.
3. **A pure computation feeds an effectful call and neither moves the other** — `dot` still fuses
   into one loop with no intermediate vector, *inside* the `seq`.

It prints `report`, `1000`, `3.328335e+08`, which is Σi² for i < 1000.

The portable form takes its array as a parameter, and `main` takes none. Construction is why: a
program could not build data at all until 2026-08-15, and on a native target it is
`go.make-float64` plus a fill loop rather than `make-vec`
([construction.md](../../docs/spec/construction.md)).

## 8. Five gaps found, and the fifth cost the most

Moving one program — `dot` — found five. None was visible from inside the portable layer.

1. **`asLinear` knew only `alen`.** The fragment was keyed to the retired layer's names, so
   `(where (== (go.len p) (go.len q)))` — the gauntlet's oldest refinement — degraded to an opaque
   atom the moment it moved.
2. **A `sig`'s `where` did not survive parameter renaming.** A length is an opaque variable string
   that substitution cannot reach inside.
3. **Lengths were not keyed in the interval environment.**
4. **`loop` lost `fold-range`'s bounds-check elimination.** Confirmed with
   `-d=ssa/check_bce/debug=1`: per-iteration `IsInBounds` before, `IsSliceInBounds` after.
5. **Fixing (1) turned a silence into a failure.** The sieves' bounds obligation had been sitting
   *outside* the fragment, reported as "propagated, not proven", because `go.len` was
   unrecognisable. Recognising it made the goal real, and the goal did not hold.

Closing (5) needed three things, and they are worth more than the migration:

- **`nonDecreasing` descended into nested loops** and read the inner `again` as the outer's, so the
  sieve's counter looked like it might decrease. Same class as the three size-change bugs, and like
  those it made the answer *worse* than the truth.
- **A square now yields a linear fact.** `x <= x*x` holds for **every** integer — the square
  dominates above 1 and is non-negative below 0 — so `x*x < n` gives `x < n`. `narrowSquare`'s
  insight arriving in the decision procedure, and the only route to a sieve's bounds proof.
- **`(length N)` is a declarable primitive attribute**, and with it a length abstraction. The
  argument's declared *type* says how to read it: an `int` argument is a count
  (`make([]bool, n)`), a container argument passes its own length through (`c[i] = true`).
  Lengths propagate through lets, loops and conditionals, and a threaded array's length is
  established as a **loop invariant** — bound from the initial value, verified against every back
  edge.

The sieve's `c[i]` is now **proven** rather than propagated, on Go, Java and windows. The migration
paid for that.

One emitter bug was introduced and caught the same way: hoisting the loop bound for narrowing
emitted `var n1 int` even when nothing was narrowed, which is `declared and not used` — a Go
**compile error**. A `loop` over a single array narrows nothing, because the guard already bounds
it. Collection is now separate from emission so the caller can ask first.

## 9. Method note: this machine is bimodal

Timings here differ by up to **3×** between consecutive passes unless the process is pinned. The
same benchmark gave 462 ns and 1403 ns on successive runs. Every number above was taken with
`ProcessorAffinity = 1`. The `StencilIntoRef` row in the first pass shows the failure mode
directly: 91,751 / 273,155 / 338,606 / 97,366 / 101,602 for five runs of one benchmark.

Unpinned numbers on a hybrid P/E-core laptop are not measurements.

## 10. What is left

**None of the six.** `dot`, `centroid`, `generic`, `wordcount`, `report` and the stencil all run on
the native Go target, all at parity with hand-written code.

Still portable-only, and none of them a gauntlet program: `norm`, `converge`, `filter`,
`build-vec`, `modules`.

**JavaScript followed the same day** — [native-js-2026-08-20](native-js-2026-08-20.md). All six at
parity there too, and it found what a hostile host is for: a `loop` in tail position was lowering to
a result variable plus `break`, worth **1.31x on V8** and **nothing on Go**. **Java has not moved.**
