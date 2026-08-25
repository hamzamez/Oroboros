# Iteration: `loop` and `again`

> **BUILT, 2026-08-19 — [ADR 0015](../decisions/0015-loop-and-again.md).** All three backends,
> the type checker, and the refinement checker. Measured at parity with the best hand-written
> code on Go and JS; every existing generated file unchanged. §6's trip-count question was
> settled by measurement rather than by choosing: `fold-range` stays, because it is 2.7% ahead on
> compute-bound `dot` — far below the 1.96× predicted, because a loop's guard is itself a proof
> the host can use.

**Status: built.** Written before the code, per [state.md §6](state.md). The research
record is [loops.md](loops.md); the retraction that shrank this question to its present size is
[loop-encoding-2026-08-18](../../gauntlet/results/loop-encoding-2026-08-18.md).

> ```lisp
> (loop ((acc 0.0) (i 0))
>   (int.lt i n)  (again (f.add acc (aindex a i)) (int.add i 1))
>   else          acc)
> ```
>
> **n loop variables with no product**, **early exit at parity with hand-written code**, and
> **unbounded iteration** — subsuming `fold-range`, `fold-range2` and the `fold-while` this document
> used to propose. It is Clojure's `loop`/`recur`, Dijkstra's guarded `do`, and an SSA block with
> arguments, which are the same thing three times.

§9 is the list of holes found while writing this, including two measurement errors of my own.

---

> **2026-08-19.** The clause chain is now shared with `cond`, which is this syntax with `again`
> removed and is ordinary reader sugar over `if`
> ([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)). First match wins and `else` is
> mandatory in both, from one function — `clauseChain` in `core/read.go`.

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

### Measured

A linear search over 100,000 elements hitting at index 6. Every row is the same algorithm:

| | ns/op | |
|---|---|---|
| **(a)** ideal — `return` from inside the loop | 2.81 | 1× |
| **(b)** the guard in the loop **condition** | 4.50 | 1.61× |
| **(c)** (b) with the bound hoisted out | 4.50 | identical to (b) |
| **(d)** the guard as a **`break` in the body** | 2.62 | 0.93× |
| **(e)** `loop`/`again`, exit by `return` | 2.66 | 0.95× |
| **(g)** `loop`/`again`, exit by **`break` to a result temp** | 2.94 | **1.05×** |
| **(f)** what the language can express today | 54,800 | **19,600×** |

**19,600×** for having no exit at all. The Java tally measured this at 2× and said it was a floor
([java-toplevel §3](../../gauntlet/results/java-toplevel-2026-08-18.md)); a nearly-full 128-slot
table was the mildest possible case.

**(b) is not what it looks like.** Not a re-evaluated bound — (c) hoists it and nothing changes —
and not a call to a predicate, because reduction inlines both test and step before emission. It is
that **a compound loop condition defeats the host's bounds-check elimination**: with
`i < n && found < 0` the compiler can no longer cheaply prove `i < len(a)`. The identical test as a
`break` in the body is free, (d). That is why this document no longer proposes a guard.

**(g) is the row that matters** and it was missing from the first draft. A loop in tail position of
its function can exit by `return`, (e); a loop used as a *value* — `(f.add 1.0 (loop …))` — must
assign a result and `break`. That is the general emission, and it costs 1.05×.

Clause count does not degrade it: the same three-outcome search, hand-written idiomatically versus
as a `loop`, measures **4.21 vs 4.18 ns**.

## 2. The form

```lisp
(loop ((x₁ z₁) … (xₙ zₙ))
  c₁    e₁
  c₂    e₂
  …
  else  eₖ)
```

- **`((xᵢ zᵢ) …)`** — the loop variables and their initial values. The shape `sig` already uses for
  named parameters, so it is not a new bracket convention.
- **`cᵢ eᵢ`** — guarded clauses, tested in order; the first `cᵢ` that holds selects `eᵢ`.
- **`else`** — the final clause, and it is **required**, so the form is total by construction.
- **`eᵢ`** is either `(again a₁ … aₙ)` — go round with these values — or any other term, which is
  the loop's result.

### Why `else` and not `true`

Readability was the first reason and there is a stronger one: **`true` is not in the language.**

```lisp
(def g (fn (x) (if true x x)))
```
```
in g: true is not bound — it is not a parameter, not a definition,
      and not a primitive on this target
```

[arithmetic.md §3](arithmetic.md) decided against a boolean literal, and the `true` in the earlier
draft only worked because the `go/builtin` experiment declared one. Writing `true` there would have
made the last clause of every portable loop non-portable. `else` is a reserved word in the condition
position, which costs nothing and cannot be got wrong.

### Where `again` may appear

**As the whole of a clause body, or under a `let`.** Not under an `if`, not as an argument to
anything else.

The rule is not arbitrary — it is one sentence:

> **`let` binds; `if` branches. Binding may wrap an `again`; branching may not.**

so that **the clause list is the loop's complete control flow**, flat and readable, which is exactly
the property Dijkstra's guarded commands have.

`let` has to be allowed, and §9 explains why: it is already how a loop body shares a computation,
and it already emits well. Today's `fold-range` body

```lisp
(fn (acc i) (let (aindex a i) (fn (x) (f.add acc (f.mul x x)))))
```
```go
x := (a[i])
acc = (acc + (x * x))
```

Forbidding `again` under `let` would mean an `again` could not share a subexpression between its
arguments — and duplicating an allocating one costs
[615×](../../gauntlet/results/wordcount-2026-08-14.md).

### Semantics

```
x̄ := z̄
repeat:
    if c₁(x̄) then  (e₁ = `again ā` ?  x̄ := ā(x̄) ; repeat   :  yield e₁(x̄))
    elif … else    (eₖ = `again ā` ?  x̄ := ā(x̄) ; repeat   :  yield eₖ(x̄))
```

`x̄ := ā(x̄)` is **simultaneous** — every new value is computed from the old ones.

### What it emits

```go
var result T                              // Go
acc, i := 0.0, int64(0)
for {
    if i < n { acc, i = acc+a[i], i+1; continue }
    result = acc; break
}
```
```javascript
let acc = 0.0, i = 0;                     // JavaScript — no parallel assignment,
let result;                               // so simultaneity needs temporaries
for (;;) {
    if (i < n) { const t1 = acc+a[i], t2 = i+1; acc = t1; i = t2; continue; }
    result = acc; break;
}
```

Java is JavaScript's shape with types. The temporaries are the ones `fold-range2` already emits, and
they were [measured free](../../gauntlet/results/loop-encoding-2026-08-18.md) — 581 vs 552 ns on JS
typed arrays, 463 vs 464 on the JVM, both inside the noise floor. When the loop *is* the function's
result, `break` collapses to `return` and the temp disappears.

## 3. Why this beats a guard, and beats `fold-range2`

**No product, for n variables.** `(again a b c)` is a multi-argument *application*, not a returned
tuple. Nothing is constructed, so nothing can escape, so §3b's problem never arises — the same
reason SSA block arguments need no tuple.

**`fold-range2` dies.** It exists only because the tupling law needed a tuple and the language had
none, so the tuple was burned into a primitive at n = 2. With n loop variables it is an ordinary
`loop`, and `fold-range3` never has to be written.

**Early exit costs 1.05×** rather than 1.61×, because the exit is a *clause* and not a value the step
must smuggle out through its own type.

**Unbounded iteration**, because nothing requires any path to reach a non-`again` clause.

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

`(loop ((x̄ z̄)) c₁ e₁ … else eₖ)` denotes `(fix F)(z̄)` where

```
F : (Sⁿ → R) → (Sⁿ → R)
F(f)(x̄) = if c₁(x̄) then  (e₁ = `again ā`  ?  f(ā(x̄))  :  e₁(x̄))
          elif … else    (eₖ = `again ā`  ?  f(ā(x̄))  :  eₖ(x̄))
```

Every recursive occurrence of `f` is in **tail position**, which is what makes `fix F` computable by
iteration rather than by a stack: `fix F = ⨆ₙ Fⁿ(⊥)`, and the value at `x̄` is reached by iterating
`x̄ ↦ ā(x̄)` until a clause exits. Classically,

> **a tail-recursive function of n arguments is a while-program with n variables** — Steele,
> *Lambda: The Ultimate GOTO* (1977).

Two consequences worth naming.

**`again` is a jump, not a call.** No frame, no stack. This is the precise sense in which the form
recovers the *useful* half of recursion while
[ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) stands: recursion was rejected
because stack depth differs by orders of magnitude across Go, the JVM and JS with no specification.
A `loop` has no stack on any of them, so a non-terminating one **hangs identically everywhere**
rather than crashing at three different depths.

**Simultaneity is φ.** `x̄ := ā(x̄)` all at once is what an SSA φ-node means and what a branch with
block arguments means. The temporaries JS and Java need are the φ written out.

## 5. The literature

The form is not new, which is the best thing about it.

| | |
|---|---|
| **Clojure `loop`/`recur`** | this design almost exactly, including that `recur` is legal only in tail position and is **checked** rather than optimised. Clojure's motivation is ours: the JVM has no tail calls, so `recur` must be a loop |
| **Scheme's named `let`** | `(let go ((a 0)) … (go (+ a 1)))` — the same with a bound name instead of a keyword, relying on Scheme's guaranteed proper tail calls |
| **Dijkstra, guarded commands (1975)** | `do G₁ → S₁ ▯ G₂ → S₂ od`. The clause list **is** guarded alternatives. Dijkstra's repeats while *any* guard holds and is nondeterministic between them; ours is ordered and exits through a non-`again` clause |
| **SSA with block arguments** | MLIR, Swift SIL, Cranelift. `(again a b c)` is `br ^header(a, b, c)`; the loop variables are the header block's parameters. Block arguments replaced φ-nodes, which is why they need no tuple — the same reason this form needs no product |
| **Rust's `loop` + `break value`** | everything is a value; a non-`again` clause is `break value` |
| **Kotlin `tailrec`, Scala `@tailrec`** | the checked-tail-position idea attached to a named function instead of a loop form |
| **Common Lisp `do`** | the same binding list, but with a fixed per-variable step, so it cannot express a data-dependent transition |
| **Böhm–Jacopini (1966)** | sequence, selection, iteration suffice. This is the iteration |

### Alternatives seriously considered

**Decompose it: `loop` + `cond`.** Make `loop` bind variables and take one body, and add `cond` as
reader sugar for nested `if`:

```lisp
(loop ((acc 0.0) (i 0))
  (cond (int.lt i n)  (again …)
        else          acc))
```

More orthogonal — `cond` is independently useful, and `loop` gains no bespoke clause syntax. It is
the more *principled* decomposition and the project's aesthetic normally favours it (`let` is `fn`,
`seq` is sugar).

**Rejected, for one reason:** it forces `again` to be legal under `if`, because `cond` *is* nested
`if`. That loses the property §2 is built on — the clause list being the loop's complete control
flow — and turns the check from a shape into a tail-position walk. The fused form buys a real
guarantee with one bespoke bracket. If `cond` is wanted later it can be added independently, and
`loop`'s clause list will read the same way.

**Name the loop instead of reserving `again`.** `(loop go ((i 0)) … (go (int.add i 1)))`, Scheme's
named `let`. Costs no keyword and, because the name is bound, an inner loop could target an outer
one — a labelled `continue`, which Go, JS and Java all have.

**Not rejected, deferred.** It is strictly more expressive and no program has asked for the extra
expressiveness. `again` reads better for the common case and the anonymity is a feature: there is
exactly one thing it can mean. If nested-loop targeting is ever needed, the named form is the
answer and it is a compatible extension.

**A `tailrec`-marked `def`.** Allow a definition to call itself when every self-call is in tail
position, and compile it to a loop — ADR 0014 §10's shape, Kotlin's and Scala's.

**Rejected.** It needs no new syntax at all, which is genuinely attractive, but the loop then needs a
*global name* and must take every free variable of its context as a parameter — defunctionalisation
by hand, at the source level. Scheme solved this with named `let` precisely because top-level
recursion is the wrong scope for a loop. It also reopens ADR 0014 for a reader, whose rule becomes
"recursion is forbidden unless it is tail recursion", and the diagnostic when you get it wrong is
famously confusing.

**A fold with a `reduced` marker** — Clojure's, Rust's `ControlFlow`. §3b is why: it needs a sum.

## 6. How it fits the language

**Binders reuse `fn`.** Represented internally as `(loop (z̄) (fn (x̄) c₁ e₁ …))`, the loop variables
are an ordinary abstraction, so `core/term.go` needs no new binding machinery and the locally
nameless representation, capture-avoidance and the emitter's `openFresh` work unchanged. `let`
already makes this move.

**Reduction needs no new rule.** `loop` is a structural primitive: δ does not unfold it, and its
arguments are normalised in place.

**Effects need no new rule, and this was checked rather than assumed.** A primitive application is
not a β-redex, so an effect written in a clause stays in it:

```lisp
(fold-range 0 3 (fn (acc i) (+ acc (shout i))))
⟶ (fold-range 0 3 (fn (acc i) (+ acc (shout i))))
```

Nothing is hoisted out of the loop it was written in, so `(again (io.print-line x) …)` needs no
special case.

**Types are a check, not an inference.** Every `xᵢ` has the type of `zᵢ`; every `again`'s i-th
argument must agree; every non-`again` clause body must share one type, which is the loop's; every
`cᵢ` is `bool`. That is the walk [types.md §3](types.md) already does.

**Refinements gain.** A clause guarded by `(int.lt i n)` gives `i < n` inside that clause — ordinary
Hoare logic, and *more* than `fold-range` offers today, because the guard is explicit rather than
implied by the primitive.

### The one real cost: the trip count

`fold-range`'s bound is evaluated **once**, and that single `n` is what the bounds-check-elimination
pattern narrows against — worth [1.96×](../../gauntlet/results/bce-2026-08-15.md). A `loop` states
its bound as a guard, so the count is not handed over.

Two ways out, and this document does not choose:

1. **Keep `fold-range`.** Two loop forms: counted and analysable, general and expressive. Meyer &
   Ritchie's `LOOP` and `WHILE`. Cost: the language has two loops.
2. **Recognise the counted shape.** A variable initialised to `0`, incremented by exactly `1` in
   every `again`, guarded by `i < e` with `e` loop-invariant, is an induction variable with trip
   count `e`. A small standard analysis, and the emitter already does one of the same class —
   `narrow` fires only when *every* occurrence of a container is indexed by the bare loop variable.
   Cost: an analysis, and the risk it silently does not fire, which
   [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) already warns about.

**The measurement that decides it**: whether the recognised form emits the same code as `fold-range`
for all seven gauntlet programs on all three targets.

## 7. Worked examples

**A fold** — what `fold-range` says today:

```lisp
(loop ((acc 0.0) (i 0))
  (int.lt i (alen a))  (again (f.add acc (aindex a i)) (int.add i 1))
  else                 acc)
```

**Two accumulators** — `centroid`, without `fold-range2`:

```lisp
(loop ((ax 0.0) (ay 0.0) (i 0))
  (int.lt i (alen xs))  (again (f.add ax (aindex xs i)) (f.add ay (aindex ys i)) (int.add i 1))
  else                  (f.add ax ay))
```

**Early exit** — the index of the first element over `k`, or −1:

```lisp
(loop ((i 0))
  (int.ge i (alen a))    -1
  (f.gt (aindex a i) k)  i
  else                   (again (int.add i 1)))
```

Read top to bottom and it is the algorithm: *out of range, give up; found it, that's the answer;
otherwise keep going.* That is what the guarded-command form buys.

**Unbounded** — Newton's method, which has no trip count at all:

```lisp
(loop ((g x))
  (f.lt (f.abs (f.sub (f.mul g g) x)) 1e-12)  g
  else                                        (again (f.div (f.add g (f.div x g)) 2.0)))
```

**Sharing, under a `let`** — the pattern `again` must not forbid:

```lisp
(loop ((acc 0.0) (i 0))
  (int.lt i (alen a))  (let (aindex a i) (fn (x) (again (f.add acc (f.mul x x)) (int.add i 1))))
  else                 acc)
```

**The sieve's inner loop**, which started all of this:

```lisp
(loop ((c composite) (j (int.mul i i)))
  (int.lt j n)  (again (g.bset c j (g.true)) (int.add j i))
  else          c)
```

A start, a step and a threaded mutation, with no arithmetic reconstructing the index — the legibility
cost [loop-encoding §3](../../gauntlet/results/loop-encoding-2026-08-18.md) named, paid off for free
as a side effect of having variables.

## 8. Termination becomes a program property

Today every loop terminates. After `loop`, a program that uses one might not. The instinct is to
call that a loss, but [ADR 0001](../decisions/0001-parasite-model.md) already answered the same
instinct about portability:

> Portability is a property a program may or may not have, **computed by the compiler** — not a
> global guarantee.

Termination should be the same:

> **A program that uses only `fold-range`, `make-vec` and `loop`s the compiler can bound provably
> terminates.** One that does not, does not — and the compiler can say which, by the walk
> `Env.CheckProgram` already does for recursion.

That is better than today's blanket guarantee, which is bought by refusing to express half of
computing.

## 9. Holes found, including two of mine

Written down because the point of hunting is to record what was found, not to report that nothing
was.

**`true` is not in the language.** The first draft's final clause was `true`, which does not resolve
— [arithmetic.md §3](arithmetic.md) decided against a boolean literal, and the `true` used while
drafting existed only because the `go/builtin` experiment declared one. A correctness hole, not a
readability preference. Fixed by `else`.

**`again` had to be allowed under `let`.** The first draft required `again` to be the *whole* clause
body. But `let` is how a loop body shares a computation, it is used that way today, and it already
emits well. Forbidding it would mean an `again` could not share a subexpression between its
arguments, and duplicating an allocating one costs 615×. Fixed by the `let` binds / `if` branches
rule.

**A loop used as a value cannot `return`.** The first draft measured only a loop in tail position of
its function. The general emission assigns a result and `break`s; measured at 1.05×, so the hole is
closed, but it was measured only after it was noticed.

**A three-clause loop looked 1.49× worse and was not.** I compared a three-clause `loop` against a
*two*-condition hand-written loop, so I measured the extra condition. Like for like: 4.18 vs 4.21 ns.
Second measurement error in the same document, both the same shape — comparing things that differ in
more than one way.

**`again` and `else` must be reserved.** A parameter or definition named either would shadow the
form. The reader must reject them as binder names, as it already rejects qualified names. Two
reserved words is the whole syntactic cost of the proposal.

**`again` refers to the nearest enclosing `loop`.** With no name there is no way to reach an outer
one — consistent with having no `break n`, and the named variant in §5 is the answer if that changes.

**An infinite loop has no result type.** `(loop ((i 0)) else (again (int.add i 1)))` is legal and
never yields. Its type is ⊥, which must unify with anything. Legal, useful for a server loop, and it
needs saying.

**`again` arity must be checked** against the binding list, and an inner `loop`'s `again` must not be
confused with an outer's.

**`loop` does not subsume `make-vec`.** Building an array needs a write, and a portable indexed write
does not exist — `make-vec` is the construction primitive
([construction.md](construction.md)) and stays.

**Still open, and not this proposal's to close:** `v, ok := m[k]` still wants a product. `loop`
removes three of the four demands for one; the fourth is a *value* that carries two things, and
nothing here provides that.

## 10. What would kill it

| | refuted by |
|---|---|
| "early exit is at parity" | JS or Java not matching Go's 1.05× — only Go is measured |
| "n variables need no product" | an emitted form that allocates on any target |
| "`fold-range2` can retire" | `centroid` losing parity when written as a `loop` |
| "the counted shape can be recognised" | any of the seven gauntlet programs emitting different code |
| the clause list | a real program that reads *worse* as clauses than as a fold — a legibility judgement, to be made on real programs rather than on §7 |

And the acceptance test, as always: **every existing program unchanged**, byte-for-byte, on all three
targets.

## 11. The order to build it in

1. **A gauntlet program that searches**, and one that converges. Neither exists, and a loop chosen
   against seven programs that never exit early would be chosen blind.
2. **`loop` in the Go backend**, against hand-written references for both.
3. **JS and Java** — and the §10 measurement that Go's parity holds there.
4. **Rewrite `centroid` as a `loop`**; if parity holds, retire `fold-range2`.
5. **Decide the trip-count question** by measuring §6's two ways out.
6. **An ADR**, recording what it cost and what it retired.


---

## The emitted shape, and why it is not `for { … }`

A loop variable that **every `again` updates identically** moves into the host `for` statement's
post clause:

```go
for ; ; i = (i + 1) {
	if (i * i) >= n { break }
	…
}
```

This is not cosmetic. Emitting `for { … i = i + 1; continue … }` duplicates the update into every
clause, so the loop has several back edges and Go's SSA does not see a counted loop — measured at
**1.4x** on the sieve ([loopshape-2026-08-25](../../gauntlet/results/loopshape-2026-08-25.md)).

The condition is soundness, not tidiness: `again`'s arguments are evaluated **simultaneously** with
every variable's old value, and a post clause runs **after** the body. So an update that reads
another loop variable is left in the body, because by then the body may have assigned it.

`dot` is unaffected — its loop has one back edge, and its machine code is byte-identical to
hand-written either way. The hoist removes a cost that appears only with several back edges.
