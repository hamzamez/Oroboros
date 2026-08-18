# Iteration: one primitive short of everything

**Status: proposed, not built.** Written before the code, per
[state.md §6](state.md). The research record is [loops.md](loops.md); the retraction that shrank
this question to its present size is
[loop-encoding-2026-08-18](../../gauntlet/results/loop-encoding-2026-08-18.md).

> **The proposal is one new structural primitive, `fold-while`.** It is the smallest addition that
> makes the language Turing-complete, and it is the host's own `for` with a condition on every
> target. Everything else people ask for from loops — `find`, `any?`, `all?`, convergence,
> bounded search — becomes a **library definition over it** that reduces away entirely.

---

## 1. What is actually missing

After the retraction, exactly one thing: **a loop whose trip count does not exist**.

`fold-range`'s bound is an arbitrary expression, so any loop whose length can be *computed before
entry* is already expressible, including arbitrary starts and steps
([loop-encoding §1](../../gauntlet/results/loop-encoding-2026-08-18.md)). What cannot be expressed
is a loop that stops because of something it *found*:

| | why there is no count |
|---|---|
| `find`, `any?`, `all?`, a probe that stops on a hit | the count is the answer |
| convergence — Newton, fixpoint, relaxation | the count depends on the data |
| streaming — read until exhausted | the count is not knowable |

These are the same shape. A loop that stops on a condition covers all of them.

### And it is not only a speed question

The language today computes **exactly the primitive recursive functions**
([loops.md §3.1](loops.md)): `fold-range` is Gödel's System T recursor, and every loop terminates by
construction. That is a real guarantee and it has a real price — Ackermann is not expressible, and
neither is "keep going until you find it".

> A language that cannot loop without a bound cannot express all computation.

So this addition is not an optimisation. It is the difference between a very large class of programs
and all of them.

### What it is worth, measured

A linear search over 100,000 elements that hits at index 6:

| | ns/op |
|---|---|
| hand-written Go, `return` from inside the loop | **2.78** |
| the proposed shape — a guard in the loop condition | 4.62 |
| **what the language can express today** — scan everything, keep the first hit | **57,694** |

**20,700×.** The Java tally measured this at 2× and said it was a floor
([java-toplevel §3](../../gauntlet/results/java-toplevel-2026-08-18.md)); a 128-slot nearly-full
table was the mildest possible case. A search that exits early is the normal case.

## 2. The primitive

```lisp
(fold-while z cont? step)
```

| | |
|---|---|
| `z` | the initial accumulator |
| `cont?` | `(fn (acc i) …)` → `bool` — **keep going while this holds** |
| `step` | `(fn (acc i) …)` → the next accumulator |
| result | the accumulator when `cont?` first fails |

The index `i` counts from 0 and belongs to the **primitive**, exactly as in `fold-range`. That is
the choice that keeps a single accumulator sufficient: without it, every bounded search would need
its state to carry a counter, and the state would have to be a product.

Semantics, and it is short:

```
acc := z
i   := 0
while cont?(acc, i):
    acc := step(acc, i)
    i   := i + 1
yield acc
```

`cont?` is tested **before** each step, so zero iterations is normal and `(fold-while z (fn (a i) false) f)`
is `z`.

### Every target spells it the same way

Which is the test [primitives.md](primitives.md) requires — what does each target do, and do they
agree?

```go
acc := z                                    // Go
for i := int64(0); cont(acc, i); i++ { acc = step(acc, i) }
```
```javascript
let acc = z;                                // JavaScript
for (let i = 0; cont(acc, i); i++) { acc = step(acc, i); }
```
```java
T acc = z;                                  // Java
for (long i = 0; cont(acc, i); i++) { acc = step(acc, i); }
```

No host needs a shim, nothing is emulated, and the loop is the one the host's own optimiser is built
around. **Tier 1**, with one caveat in §6.

## 3. Why `fold-range` stays

`fold-range z n f` is exactly `(fold-while z (fn (acc i) (int.lt i n)) f)`, so it is *derivable*.
It stays a primitive anyway, for a reason that is not taste:

**Its bound is evaluated once.** That single evaluated `n` is what the bounds-check-elimination
pattern narrows against — `p = p[:n1]`, worth
[1.96× on compute-bound loops](../../gauntlet/results/bce-2026-08-15.md) — and what the refinement
checker reads as `0 ≤ i < n` at every `aindex`. A condition re-evaluated per iteration offers
neither without an analysis that recovers what the counted form states outright.

So the split is the one every fast language has, and the one Meyer & Ritchie drew in 1967:

| | analysable | expressive |
|---|---|---|
| `fold-range` | trip count, bounds facts, narrowing, guaranteed termination | bounded only |
| `fold-while` | the guard, and nothing else | everything |

Two primitives, and each earns its place. Not a compromise — `LOOP` and `WHILE`.

## 4. The beauty is in the library, not the primitive

`fold-while` on its own is honest and a little bare:

```lisp
(fold-while -1
  (fn (found i) (logic.and (int.lt i (alen a)) (int.lt found 0)))
  (fn (found i) (if (p (aindex a i)) i found)))
```

Nobody should write that twice. Write it once, in a library, and every use reads like the thing it
means — and **reduces to exactly the loop above**, because δ+β consume the definition
([chapter 2 §1.8](../book/02-def.md)):

```lisp
(module seq)
(export find-first any? all? count-while iterate-until)

; The index of the first element satisfying p, or -1.
(def find-first (fn (a p)
  (fold-while -1
    (fn (found i) (logic.and (int.lt i (alen a)) (int.lt found 0)))
    (fn (found i) (if (p (aindex a i)) i found)))))

(def any? (fn (a p) (int.ge (find-first a p) 0)))
(def all? (fn (a p) (int.lt (find-first a (fn (x) (logic.not (p x)))) 0)))

; How many leading elements satisfy p.
(def count-while (fn (a p)
  (fold-while 0
    (fn (n i) (logic.and (int.lt i (alen a)) (int.eq n i)))
    (fn (n i) (if (p (aindex a i)) (int.add n 1) n)))))

; Iterate until a fixpoint test passes. The index is unused, which is what an
; unbounded loop looks like.
(def iterate-until (fn (z done? step)
  (fold-while z (fn (s i) (logic.not (done? s))) (fn (s i) (step s)))))
```

And then programs read like this:

```lisp
(use seq)
(use num/f64 as f)

(def first-big (fn (xs) (seq.find-first xs (fn (x) (f.gt x 10.0)))))

(def sqrt-newton (fn (x)
  (seq.iterate-until x
    (fn (g) (f.lt (f.abs (f.sub (f.mul g g) x)) 1e-12))
    (fn (g) (f.div (f.add g (f.div x g)) 2.0)))))
```

That is the answer to "beautiful syntax". **The core gets the smallest honest construct; the library
gets the readable names; reduction makes the library free.** Adding `find-first` to the *language*
would buy nothing that a `def` does not, and would cost a keyword — which is
[chapter 3 §3.11](../book/03-modules.md)'s functor argument again, in a new place.

## 5. Termination becomes a program property

This is the interesting consequence, and it fits the project rather than fighting it.

Today every loop terminates. After `fold-while`, a program that uses it might not. The instinct is
to call that a loss. But the same instinct was already answered by
[ADR 0001](../decisions/0001-parasite-model.md):

> Portability is a property a program may or may not have, **computed by the compiler** — not a
> global guarantee.

Termination should be the same thing:

> **A program that uses only `fold-range`, `fold-range2` and `make-vec` provably terminates.** One
> that uses `fold-while` does not, and the compiler can say which — by exactly the walk that
> `Env.CheckProgram` already does for recursion.

That is a *better* state than today's blanket guarantee, because today the guarantee is bought by
refusing to express half of computing. A computed property that a program can be checked against
beats an unconditional promise that costs the language its completeness.

It also composes with [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md). Recursion
was rejected because its stack depth differs per target with no specification. `fold-while` has no
stack at all — it is a loop on every host — so a non-terminating `fold-while` **hangs identically
everywhere** rather than crashing at three different depths. Unbounded iteration is exactly the part
of recursion that *is* portable.

## 6. What it costs, honestly

**One comparison per iteration.** The measured 4.62 ns against a hand-written 2.78 ns is the guard
being re-tested where a `return` would have left immediately. A `break` from inside the body would
avoid it, but the step would then have to *signal* stopping — `(done v)` versus `v` — which needs a
sum type, or a second returned value, which needs a product. The language has neither.

So the shape is: **1.66× against an ideal `break`, against 20,700× for having no exit at all.** Take
the guard.

**One value out.** `fold-while` yields the accumulator, not the index, so a program that needs *both*
the result and the number of steps — Collatz stopping time is the honest example — cannot have both
without a product accumulator. That is the **fourth** independent demand for products, after
`v, ok := m[k]`, `fold-range2`, and JS's "was the key present". It is a separate question and this
proposal does not need it answered.

**No facts for the refinement checker**, unless the guard is read. Inside the body `cont?(acc,i)`
holds, which is ordinary Hoare logic, so a guard written `(logic.and (int.lt i n) …)` could yield
`i < n` by inspection. Worth doing and not required for a first version — the honest default is that
`fold-while` gives `0 ≤ i` and nothing else, and an `aindex` inside one reports an undischarged
obligation, which is [refinements.md §3](refinements.md)'s classify-don't-restrict working as
designed.

**No narrowing**, so no bounds-check elimination inside a `fold-while`. That is 1.96× on
compute-bound loops, and it is the price of not knowing the count — which is precisely why
`fold-range` stays.

## 6b. Why "a sum or a product", concretely

That phrase appears four times above and deserves to be shown rather than asserted. Everything in
this section is real output.

**A product** is a value carrying several values at once — "A *and* B". A tuple, a struct, a record.
**A sum** is a value that is one of several alternatives, tagged so you can tell which — "A *or* B".
An enum, a variant, `Option`, `Either`.

The `step` function has type `(acc, i) → acc`. It returns **one** value, of the accumulator's type.
For it to say *stop*, it must return something the loop primitive can tell apart from an ordinary
accumulator:

| | the step returns | the primitive reads |
|---|---|---|
| with a **sum** | `Done(acc)` or `More(acc)` | the tag — Clojure's `reduced`, Rust's `ControlFlow`, Haskell's `Either` |
| with a **product** | `(acc, keep-going?)` | the second component |

Either way the return type is strictly richer than `A`. That is the whole of the claim.

### But we have encodings, and they are free — except here

[Chapter 2 §2.11](../book/02-def.md) showed a Church pair costing nothing. It really does:

```lisp
(def pair (fn (a b) (fn (sel) (sel a b))))
(def fst  (fn (p) (p (fn (a b) a))))
(def snd  (fn (p) (p (fn (a b) b))))
(def g (fn (x y) (f.add (fst (pair x y)) (snd (pair x y)))))
```
```go
func GenG(x float64, y float64) float64 {
	return (x + y)
}
```

Now carry the *same pair* across a loop iteration — a two-accumulator fold, the obvious thing:

```lisp
(def g (fn (v)
  (fst (fold-range (pair 0.0 0.0) (alen v) (fn (acc i)
    (pair (f.add (fst acc) (aindex v i)) (snd acc)))))))
```
```
gen: application of a non-name: ((fold-range (fn (sel) (sel 0.0 0.0)) (alen v) …) (fn (a b) a))
```

And a Church-encoded **sum** — `done`/`more`, the exact shape early exit wants — fails identically:

```lisp
(def done (fn (x) (fn (on-done on-more) (on-done x))))
(def more (fn (x) (fn (on-done on-more) (on-more x))))
```
```
gen: application of a non-name: ((fold-range (fn (on-done on-more) (on-more -1)) (alen v) …) …)
```

**An encoding erases only when reduction can see the eliminator.** Outside a loop it can: the
constructor and the destructor are adjacent in the term, and β brings them together. Across a loop
iteration it cannot — the value is *produced* in iteration k and *consumed* in iteration k+1, and
those are the same code executed at different times. Reduction happens once, at compile time; the
loop runs many times, at runtime. **Nothing reduces across the back-edge.**

What survives is a λ that must exist while the program runs — an escaping closure — and every
backend refuses one. This is [structs-2026-08-14](../../gauntlet/results/structs-2026-08-14.md)'s
sentence with a demonstration attached: *compile-time reduction cannot cross a runtime loop
boundary*.

> The trick that makes products free everywhere else in this language fails at **exactly one
> place**: a loop's carried state.

And that is why `fold-range2` exists. It is the tupling law with the tuple **burned into the
primitive**, so that no tuple value ever exists to escape.

### What is available instead: a sentinel

If the accumulator's type has a spare value, a sum can be encoded in its *range* rather than in a
tag:

```lisp
(def g (fn (v k)
  (fold-range -1 (alen v) (fn (found i)
    (if (int.ge found 0) found
      (if (f.gt (aindex v i) k) i found))))))
```
```go
var found int64 = -1
for i := int64(0); i < n1; i++ { … }
return found
```

Free, and clean. But not general: it needs a value of the accumulator's own type that cannot
otherwise occur. `int` has `-1`. `f64` has NaN, which is not equal to itself and so is awkward to
test. A `vec-f64` accumulator has nothing spare at all.

### The four options, and why the proposal takes none of them

| | general | cost |
|---|---|---|
| sentinel | **no** — needs a spare value in the type | free |
| Church encoding | yes | **fails across a loop** — escaping closure |
| heap product or sum | yes | 6.4× on the JVM, 13.8× on JS; free on Go |
| product + SROA | yes | free — but it is a real feature, and its own ADR |

So: every *general* way for a step to say "stop" requires it to return something richer than the
accumulator, and the one mechanism that would be free is precisely the one a loop boundary defeats.

**Which is why `fold-while` uses a guard.** `cont?` is a **separate function returning `bool`**. The
decision never has to be smuggled through the accumulator's type, so no sum and no product is
needed. That is the whole trick of §2, and the reason it is one primitive rather than a feature:

> Move the stopping decision out of the step's *return value* and into its own *predicate*.

The price is the one §6 already states — one comparison per iteration, 1.66× against an ideal
`break`. The alternative is the third row of that table.

## 7. What is deliberately not in this proposal

- **A start and a step.** Expressible today, and an explicit step measured at **no benefit**
  because Go's strength reduction already performs it. If they arrive it will be as *sugar over
  `fold-range`*, argued on legibility, and it should be argued separately.
- **Products and SROA.** Wanted from four directions now, worth 6.4× on the JVM and 13.8× on JS,
  and big enough to need its own ADR against
  [ADR 0013](../decisions/0013-accept-the-allocation-price.md).
- **`scan`.** Needs products.
- **`break n` out of nested loops.** `fold-while`'s guard exits one loop. Multi-level exit has no
  program asking for it.
- **Retiring `fold-range2`.** It dies when products arrive, not here.

## 8. What would kill it

Per [ADR 0008](../decisions/0008-measurement-over-principle.md):

| | refuted by |
|---|---|
| the primitive | a host where `for i := 0; cond; i++` is not the fastest available loop — measure against `while` and against `break`-from-body on each target |
| the guard, versus a `(done v)` marker | a program where one comparison per iteration is a material cost, which would make the sum type worth its price |
| "the library makes it beautiful" | a `find-first` that does **not** reduce away — check the residual, not the source |
| "termination as a program property" | a program that needs the guarantee and cannot get it from the check |

And the acceptance test, from [types.md §6](types.md)'s pattern: **every existing program must be
unchanged.** `fold-while` adds a primitive and touches nothing else, so every generated file in the
gauntlet must be byte-identical on all three targets afterwards.

## 9. The order to build it in

1. **A gauntlet program that searches.** None of the seven does, and a loop primitive chosen against
   tests that never exit early would be chosen blind. `find-first` over a large array, plus a
   convergence program, with hand-written references on all three hosts.
2. **`fold-while` in the Go backend**, checked against those references.
3. **JS and Java**, which should be the same three lines each.
4. **The `seq` library**, and a check that it reduces away.
5. **The termination property**, reported the way `Shadowed()` is.
6. **An ADR**, recording what it cost and what it killed.
