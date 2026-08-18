# Iteration: `loop` and `again`

**Status: proposed, not built.** Written before the code, per [state.md §6](state.md). The research
record is [loops.md](loops.md); the retraction that shrank this question to its present size is
[loop-encoding-2026-08-18](../../gauntlet/results/loop-encoding-2026-08-18.md).

> **The proposal is one structural form:**
>
> ```lisp
> (loop ((acc 0.0) (i 0))
>   (int.lt i n)  (again (f.add acc (aindex a i)) (int.add i 1))
>   true          acc)
> ```
>
> It gives **n loop variables with no product**, **early exit at zero cost**, and **unbounded
> iteration** — and it subsumes `fold-range`, `fold-range2` and the `fold-while` this document used
> to propose. It is Clojure's `loop`/`recur`, Dijkstra's guarded `do`, and an SSA block with
> arguments, which are all the same thing.

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

And it is not only a speed question. The language today computes **exactly the primitive recursive
functions** ([loops.md §3.1](loops.md)) — `fold-range` is Gödel's System T recursor.

> A language that cannot loop without a bound cannot express all computation.

### What it is worth, measured

A linear search over 100,000 elements hitting at index 6, five shapes of the same loop:

| | ns/op | |
|---|---|---|
| **(a)** ideal — `return` from inside the loop | **2.80** | 1× |
| **(b)** the guard in the loop **condition** | 4.50 | 1.61× |
| **(c)** (b) with the bound hoisted out | 4.50 | identical |
| **(d)** the guard as a **`break` in the body** | **2.62** | 0.94× |
| **(e)** **what `loop`/`again` emits** | **2.89** | **1.03×** |
| **(f)** what the language can express today | 54,800 | **19,600×** |

Two things to read off it.

**19,600×** for having no exit. The Java tally measured this at 2× and said it was a floor
([java-toplevel §3](../../gauntlet/results/java-toplevel-2026-08-18.md)); a nearly-full 128-slot
table was the mildest possible case, and a search that exits early is the normal one.

**And the 1.61× in (b) is not what it looks like.** It is not a re-evaluated bound — (c) hoists that
and changes nothing — and there is no *call* to a predicate, because reduction inlines both the test
and the step before emission. It is that **a compound loop condition defeats the host's bounds-check
elimination**: with `i < n && found < 0` the compiler can no longer prove `i < len(a)` cheaply. Move
the identical test into the body as a `break` and the cost vanishes, (d). That is why this document
no longer proposes a guard.

## 2. The form

```lisp
(loop ((x₁ z₁) … (xₙ zₙ))
  c₁  e₁
  c₂  e₂
  …
  cₖ  eₖ)
```

- **`((xᵢ zᵢ) …)`** — the loop variables and their initial values. The same shape `sig` already uses
  for named parameters, so it is not a new bracket convention.
- **`cᵢ eᵢ`** — guarded clauses, tested in order. The first `cᵢ` that holds selects `eᵢ`.
- **`eᵢ`** is either `(again a₁ … aₙ)` — go round again with these values — or **any other term**,
  which is the loop's result.

`again` is legal **only as the whole of a clause body**. Not nested inside an `if`, not an argument
to anything. That is stricter than "tail position" and it buys two things: the check is a syntactic
shape rather than an analysis, and the clause list becomes the *complete* control flow of the loop,
flat and readable. `if` may appear freely inside `again`'s **arguments**.

Semantics:

```
x̄ := z̄
repeat:
    if c₁(x̄) then  (e₁ is `again ā` ?  x̄ := ā(x̄) ; repeat   :  yield e₁(x̄))
    elif c₂(x̄) then …
```

The assignment `x̄ := ā(x̄)` is **simultaneous** — every new value is computed from the old ones.

### What it emits

```go
acc, i := 0.0, 0                       // Go
for {
    if i < n {
        acc, i = acc+a[i], i+1
        continue
    }
    return acc
}
```
```javascript
let acc = 0.0, i = 0;                  // JavaScript — no parallel assignment,
for (;;) {                             // so simultaneity needs temporaries
    if (i < n) { const t1 = acc+a[i], t2 = i+1; acc = t1; i = t2; continue; }
    return acc;
}
```
```java
double acc = 0.0; long i = 0;          // Java, same
for (;;) {
    if (i < n) { final double t1 = acc+a[i]; final long t2 = i+1; acc = t1; i = t2; continue; }
    return acc;
}
```

The temporaries are the ones `fold-range2` already emits, and they were
[measured free](../../gauntlet/results/loop-encoding-2026-08-18.md) — 581 vs 552 ns on JS typed
arrays, 463 vs 464 on the JVM, both inside the noise floor. No host needs a shim.

## 3. Why this is better than a guard, and better than `fold-range2`

**No product, for n variables.** `(again a b c)` is a multi-argument *application*, not a returned
tuple. Nothing is ever constructed, so nothing can escape, so the problem
§3b is about never arises. That is not a trick — it is the
same reason SSA block arguments need no tuple.

**`fold-range2` dies.** It exists only because the tupling law needed a tuple and the language had
none, so the tuple was burned into a primitive at n = 2. With n loop variables it is an ordinary
`loop`, and `fold-range3` never has to be written.

**Early exit is free**, (e) above, because the exit is a *clause* rather than a value the step has to
smuggle out. 1.03× against an ideal hand-written `return`.

**Unbounded iteration**, because nothing requires a clause to be `again`-free on any particular path.

## 3b. Why "a sum or a product", and why this form needs neither

The phrase recurs throughout this project and deserves to be shown rather than asserted. Everything in
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

**And this is why `loop`/`again` needs neither.** A step that returns the next accumulator has to
smuggle the decision through its return type. `(again a b c)` does not *return* anything — it is a
multi-argument application that becomes a jump with parallel assignment, so no value carrying "and
also, stop" ever exists. The exit is a **clause**, not a value.

That is the deep reason the withdrawn `fold-while` had to pay 1.61× and this form pays 1.03×: the
guard version keeps the decision inside the loop's *condition*, where it defeats bounds-check
elimination, because it had nowhere else to put it.

## 4. What it is, mathematically

`(loop ((x̄ z̄)) c₁ e₁ … cₖ eₖ)` denotes `(fix F)(z̄)` where

```
F : (Sⁿ → R) → (Sⁿ → R)
F(f)(x̄) = if c₁(x̄) then  (e₁ = `again ā`  ?  f(ā(x̄))  :  e₁(x̄))
          elif c₂(x̄) then …
```

Every recursive occurrence of `f` is in **tail position**, which is what makes `fix F` computable by
iteration rather than by a stack: `fix F = ⨆ₙ Fⁿ(⊥)`, and the value at `x̄` is reached by iterating
the state transition `x̄ ↦ ā(x̄)` until a clause exits. This is the classical statement that

> **a tail-recursive function of n arguments is a while-program with n variables** — Steele,
> *Lambda: The Ultimate GOTO* (1977).

Two consequences worth naming:

**`again` is a jump, not a call.** No frame, no stack. This is the precise sense in which the form
reintroduces the *useful* half of recursion while
[ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) stays intact: recursion was
rejected because stack depth differs by orders of magnitude across Go, the JVM and JS with no
specification. A `loop` has no stack on any of them, so a non-terminating one **hangs identically
everywhere** instead of crashing at three different depths.

**Simultaneity is φ.** `x̄ := ā(x̄)` all at once is exactly what an SSA φ-node means, and exactly what
a branch with block arguments means. The temporaries JS and Java need are the φ made explicit.

## 5. The literature

The form is not new, which is the best thing about it.

| | |
|---|---|
| **Clojure `loop`/`recur`** | this design, almost exactly, including that `recur` is legal only in tail position and is **checked** rather than optimised. Clojure's motivation is ours: the JVM has no tail calls, so `recur` must be a loop |
| **Scheme's named `let`** | `(let go ((a 0)) … (go (+ a 1)))` — the same thing with a bound name instead of a keyword, and it relies on Scheme's guaranteed proper tail calls |
| **Dijkstra, guarded commands (1975)** | `do G₁ → S₁ ▯ G₂ → S₂ od`. The clause list **is** guarded alternatives. Dijkstra's repeats while *any* guard holds and is nondeterministic between them; ours is ordered, and exits through a clause that is not `again` |
| **SSA with block arguments** | MLIR, Swift SIL, Cranelift. `(again a b c)` is `br ^header(a, b, c)`; the loop variables are the header block's parameters. Block arguments were introduced precisely to replace φ-nodes, which is why they need no tuple — the same reason this form needs no product |
| **Rust's `loop` + `break value`** | everything is a value; a non-`again` clause is `break value` |
| **Kotlin `tailrec`, Scala `@tailrec`** | the checked-tail-position idea, attached to a named function instead of a loop form |
| **Common Lisp `do`** | `(do ((var init step)…) (end result) …)` — the same binding list, but with a fixed per-variable step, so it cannot express a data-dependent transition |
| **Böhm–Jacopini (1966)** | sequence, selection, iteration suffice. This is the iteration |

Against the form this document used to propose:

| | `fold-while` (withdrawn) | `loop` / `again` |
|---|---|---|
| loop variables | 1 | **n, with no product** |
| early exit | 1.61× — the guard defeats BCE | **1.03×** |
| retires `fold-range2` | no | **yes** |
| unbounded iteration | yes | yes |
| new syntax | none | a binding list, a clause list, `again` |
| new check | none | `again` only as a clause body |

`fold-while` loses on capability *and* on speed. It is withdrawn.

## 6. How it fits the language

**Binders reuse `fn`.** Represented internally as `(loop (z̄) (fn (x̄) c₁ e₁ …))`, the loop variables
are an ordinary abstraction, so `core/term.go` needs no new binding machinery and the locally
nameless representation, capture-avoidance, and the emitter's `openFresh` all work unchanged. That
is the same move `let` already makes.

**Reduction needs no new rule.** `loop` is a structural primitive, so δ does not unfold it and its
arguments are normalised in place. `again` is an application of a name that nothing defines, which
reduction already leaves alone — the same status `alen` has on a target that declares it.

**Effects need no new rule.** A primitive application is not a β-redex, so an effect written inside
a clause stays inside it, exactly as [chapter 4 §4.7](../book/04-effects.md) describes for `if` and
`fold-range`. Nothing is hoisted out of a loop it was written in.

**Types are a check, not an inference.** Every `xᵢ` has the type of `zᵢ`; every `again`'s i-th
argument must have that same type; every non-`again` clause body must have one common type, which is
the loop's. Every `cᵢ` is `bool`. All of it is the walk [types.md §3](types.md) already does.

**Refinements gain, rather than lose.** A clause guarded by `(int.lt i n)` gives `i < n` inside that
clause — ordinary Hoare logic, and *more* than `fold-range` currently offers, because the guard is
explicit rather than implied.

### The one real cost: the trip count

`fold-range`'s bound is evaluated **once**, and that single `n` is what the bounds-check-elimination
pattern narrows against — worth [1.96× on compute-bound loops](../../gauntlet/results/bce-2026-08-15.md).
A `loop` states its bound as a guard, so the count is not handed over.

Two ways out, and this document does not choose:

1. **Keep `fold-range`.** Two loop forms: the counted one, declared and analysable; the general one,
   expressive. That is Meyer & Ritchie's `LOOP` and `WHILE`, and each earns its place. Cost: the
   language has two loops.
2. **Recognise the counted shape.** A variable initialised to `0`, incremented by exactly `1` in
   every `again`, guarded by `i < e` with `e` loop-invariant, is an induction variable with trip
   count `e`. This is a small standard analysis, and the emitter already does one of the same class —
   `narrow` fires only when *every* occurrence of a container is indexed by the bare loop variable.
   Cost: an analysis, and the risk that it silently does not fire, which
   [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already warns about.

**The measurement that decides it** is whether the recognised form emits the same code as
`fold-range` on all three targets, for all seven gauntlet programs. If it does, `fold-range` retires
and the language has exactly one loop.

## 7. Worked examples

**A fold.** What `fold-range` says today:

```lisp
(loop ((acc 0.0) (i 0))
  (int.lt i (alen a))  (again (f.add acc (aindex a i)) (int.add i 1))
  true                 acc)
```

**Two accumulators** — `centroid`, without `fold-range2`:

```lisp
(loop ((ax 0.0) (ay 0.0) (i 0))
  (int.lt i (alen xs))  (again (f.add ax (aindex xs i)) (f.add ay (aindex ys i)) (int.add i 1))
  true                  (f.add ax ay))
```

**Early exit** — the index of the first element over `k`, or −1:

```lisp
(loop ((i 0))
  (int.ge i (alen a))        -1
  (f.gt (aindex a i) k)      i
  true                       (again (int.add i 1)))
```

Read the clause list top to bottom and it is the algorithm: *out of range, give up; found it, that's
the answer; otherwise keep going.* That is the readability the guarded-command form buys.

**Unbounded** — Newton's method, which has no trip count at all:

```lisp
(loop ((g x))
  (f.lt (f.abs (f.sub (f.mul g g) x)) 1e-12)  g
  true                                        (again (f.div (f.add g (f.div x g)) 2.0)))
```

**The sieve's inner loop**, which started all of this:

```lisp
(loop ((c composite) (j (int.mul i i)))
  (int.lt j n)  (again (g.bset c j (g.true)) (int.add j i))
  true          c)
```

A start, a step, and a mutation threaded through the state — with no arithmetic to reconstruct the
index, which is what [loop-encoding §3](../../gauntlet/results/loop-encoding-2026-08-18.md) called
the legibility cost of the counted encoding. It is paid here for free, as a side effect of having
variables.

## 8. What is still deliberately absent

- **Products and SROA.** `loop` removes three of the four demands for a product — n accumulators,
  early exit, and multi-value loop state. The fourth remains: `v, ok := m[k]` needs a *value* that
  carries two things, and nothing here provides one.
- **`scan`.** Now expressible: a `loop` carrying both an accumulator and an output array.
- **Breaking out of nested loops.** `again` and the exit clauses belong to their own `loop`. No
  program has asked for more.
- **Labelled loops.** Same.

## 9. What would kill it

| | refuted by |
|---|---|
| "early exit is free" | a host where the clause form does not match a hand-written `return`; (e) is Go only so far, and JS and Java must be measured |
| "n variables need no product" | an emitted form that allocates on any target |
| "`fold-range2` can retire" | `centroid` losing parity when written as a `loop` |
| "the counted shape can be recognised" | any of the seven gauntlet programs emitting different code than today |
| the whole form | a program that reads *worse* as a clause list than as a fold, which is a legibility judgement and should be made on real programs rather than on these examples |

And the acceptance test, as always: **every existing program unchanged**, byte-for-byte, on all
three targets.

## 10. The order to build it in

1. **A gauntlet program that searches**, and one that converges. Neither exists, and a loop chosen
   against seven programs that never exit early would be chosen blind.
2. **`loop` in the Go backend**, checked against hand-written references for both.
3. **JS and Java.**
4. **Rewrite `centroid` as a `loop`** and check parity; if it holds, retire `fold-range2`.
5. **Decide the trip-count question** by measuring §6's two ways out.
6. **An ADR**, recording what it cost and what it retired.
