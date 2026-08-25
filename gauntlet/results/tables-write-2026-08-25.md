# Tables, the write side — 2026-08-25

`(alloc t)`, `(build n (fn (b) …))` and `(set b i v)` — [ADR 0018](../../docs/decisions/0018-immutable-values-linear-buffers.md)
built, on the reducer and on three of four backends.

The pass condition was set before building: **the sieve, written portably.** ADR 0018 was decided
on exactly that program — a gather cannot express a scatter, so the sieve, in-place sorting,
histograms, union-find and general dynamic programming were inexpressible portably *at any speed*.

It is written now, in [examples/table/sieve.oro](../../examples/table/sieve.oro), and names no
target primitive for memory at all. Correct on Go, JavaScript and Java, checked against a
hand-written reference across 2000–3000 sizes on each.

---

## 1. What it costs, decomposed

Timings are minimums of 5–7 runs, **one process per benchmark, pinned to one core**. This machine
is bimodal and unpinned numbers on it are not measurements
([native-gauntlet §9](native-gauntlet-2026-08-20.md)); the first pass here gave the *same*
benchmark 385,000 and 1,149,000 ns.

| | ns/op | |
|---|---|---|
| hand-written Go | 315 – 345 k | |
| **our loop shape only** — hand-written, no tables | **457 – 500 k** | **the cost is here** |
| emitted from `examples/native/sieve-go.oro` — `go.make-bool`, `go.set-bool` | 481 – 486 k | |
| **emitted from `examples/table/sieve.oro`** — `build`, `set`, indexing | **447 – 484 k** | |

**Tables cost nothing.** The portable sieve matches the one written against Go's own primitives,
and is if anything slightly ahead.

**But neither reaches hand-written, and that gap is about 1.4x.** It is not caused by this build; it
was there before and nobody had measured it, because the sieve had only ever been compared against
*our own previous output*.

### Where the 1.4x is, isolated

Two probes, kept in `gauntlet/go/sieve_shape_test.go` because the conclusion is a live constraint:

- **Aliasing is not the cost.** The table version threads the buffer as a loop variable, so the
  output says `c2 := c`, `c3 := c2`, `c2 = c3`. Hand-written with exactly that shape: **326 k** —
  inside hand-written. Go coalesces all of it.
- **The loop shape is the cost.** `for { if guard { break }; …; continue }` against
  `for init; cond; post`, hand-written, no tables anywhere: **457 k**. Narrowed further, it is the
  **outer** loop — our form duplicates the increment into each clause and gives the loop several
  back edges, so Go cannot see a counted loop. Keeping the outer loop idiomatic and using our shape
  only on the inner one measures **327 k**, at hand-written.

So: **the emitted code matches hand-written code written in our shape, and the shape is what
costs.** That is [ADR 0013](../../docs/decisions/0013-accept-the-allocation-price.md)'s finding
arriving again from a different direction — *the price is the shape, not the compiler* — and it is
program-dependent in the same way [bce-2026-08-15](bce-2026-08-15.md) was: `dot` reaches **1.00x**
with the identical loop shape ([tables-read](tables-read-2026-08-25.md)).

**This is a new open item, not a table problem.** Emitting a counted loop where the clause structure
allows one is the obvious next move, and it would benefit every program rather than this one.

---

## 2. Linearity, which was missing and is the ADR's own safety property

ADR 0018 says the buffer is **linear**: `(set b i v)` consumes `b` and returns it, and that is what
lets `build` freeze on the way out without copying — nothing else can be holding it.

It was not checked. This was accepted:

```lisp
(build n (fn (c)
  (let (set c 0 1) (fn (c2)
    (seq (set c 1 (c 0))          ; `c` READ after it was consumed
         c2)))))
```

The check is **`occurrences` on the residual, not a type**, exactly as the ADR specifies — and
building it showed the ADR's phrasing needs one refinement: it is an **ordering** property, not a
counting one. Reads do not consume. The sieve tests a cell and then carries the same buffer
forward, so a checker that counted occurrences would refuse the one program this ADR exists for.
So the walk goes in **evaluation order** — a `let`'s value before its body, an `if`'s condition
before its branches, a store's index and value before the store — and branches fork and rejoin.

### And it found the same shadowing bug `match` did, from the other direction

The first version refused the sieve. `core.Term.Body()` **opens** a lambda using its parameter-name
*hints*, so the sieve's nested `(fn (c i) …)` — which threads the buffer under the same name —
turned its own bound occurrences into free `c`s, and every one looked like a use of the outer
buffer. `Closed()` leaves inner binders as indices. Two builds in a row have now been bitten by a
name being reused for a shadowing binder.

---

## 3. Per target

| | `build` | `set` | `alloc` |
|---|---|---|---|
| **Go** | `make([]bool, n)` | `c[i] = true` | `make` + fill loop |
| **JavaScript** | `new Array(n).fill(0)` | `c[i] = true;` | `new Array` + fill loop |
| **Java** | `new boolean[(int) n]` | `c[(int) i] = true;` | `new` + fill loop |
| **windows** | **refused, deliberately** | — | — |

**JavaScript fills rather than leaving the array sparse**, and that is not cosmetic: a sparse array
on V8 is a dictionary, so every store into one is a **map insert** rather than an element write.
That is `js.set`'s existing refusal — *a JavaScript array store is a map insert* — arriving from the
allocation side.

**Java needed a `KBool` case in `typeOf`**, which had never existed. The element type of a buffer is
read off the value a `set` stores, the sieve stores `true`, and without it every boolean buffer was
a `long[]` and `javac` refused the file. Found by running `javac`, not by reading the output.

**windows refuses, and the refusal names two decisions rather than claiming the target cannot.** It
needs the array representation `len` is already waiting on, *and* an allocator: the other three
hosts bring a collector and this one brings `VirtualAlloc` and nothing. ADR 0018 says reclamation
here is a lexical arena or Perceus-style refcounting — a decision to make deliberately rather than
by writing whichever is easiest first. `x64.buf` and `x64.mov-store` are target-native and work
today.

---

## 4. What the build confirmed about ADR 0018's cost claim

The ADR argued this "costs almost nothing to build because every mechanism it needs already
exists". That held, with one addition and one correction:

- **Stores were already sequenced.** `set` is impure, and ADR 0010 never substitutes an impure
  argument — denying contraction, weakening and exchange. Nothing was written for this.
- **The buffer could already not escape.** Closures are refused as values.
- **`occurrences` was already in the reducer** — though the ordering walk above is new, and the
  ADR's "at most once at each point" understated what was needed.
- **What the ADR did not mention**: `build` has to record `len(b) = n` for the refinement layer, or
  a program cannot prove its own index. The sieve knows `i < n` from its guard and needs the
  equation to connect that to `(c i)`. One `(length 1)` attribute and one case in `refine.go`.
  With it: **10 of 10 integer operations bounded, 3 of 3 loops proven terminating** — the same
  numbers the native sieve gets.

---

## 5. Method

**Zero of 188 residuals changed** — every example on all four targets, before and after.

Correctness was checked by running the emitted code against a hand-written reference: 3000 sizes on
Go, 2000 on JavaScript and Java, plus n = 200000. All three agree, and all three give 2262 primes
below 20000.
