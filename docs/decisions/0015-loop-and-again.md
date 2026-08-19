# 0015 — `loop` and `again`

Date: 2026-08-19
Status: Accepted

## Context

`fold-range` is Gödel's System T recursor, so the language computed **exactly the primitive
recursive functions**: every loop terminated by construction, and no loop could stop because of
something it found. That ruled out `find`, `any?`, `all?`, every convergence method, and anything
read until exhausted — and it meant the language could not express all computation.

The measurement that made it urgent is a linear search over 100,000 elements hitting at index 6:
hand-written Go **2.81 ns**, and what the language could express **54,800 ns**. Nearly 20,000×.

An earlier proposal, `fold-while`, put a keep-going predicate in the loop's *condition*. It was
withdrawn: a compound loop condition defeats the host's bounds-check elimination, costing 1.61×,
and it carried one accumulator, so multiple loop variables still needed a product the language does
not have. It was hamza who proposed the form that replaced it.

## Decision

One structural form:

```lisp
(loop ((x₁ z₁) … (xₙ zₙ))
  c₁    e₁
  …
  else  eₖ)
```

Guarded clauses over **n loop variables**. A clause body is `(again a₁ … aₙ)` — go round with these
values — or any other term, which is the loop's result. `else` is required, so every way out is
written down.

`again` may be a clause body or sit under a `let`, **never under an `if`**: *let binds, if branches*.
The clause list is therefore the loop's complete control flow, which is what makes it readable and
what makes the check a syntactic shape rather than an analysis.

Internally it desugars to `(loop (fn (x…) if-chain) z…)`, so the binders are an ordinary `fn` and the
locally nameless representation, capture-avoidance and `openFresh` work unchanged; the clause chain
is ordinary `if`s, so reduction needs no new rule.

### Measured, against the best hand-written code on each host

Not against code shaped like ours — against what a person would actually write.

| | hand-written | generated | |
|---|---|---|---|
| Go, search hitting at index 6 | 2.68 ns | 2.87 ns | 1.07× |
| **Go, search hitting at 99,998** | 37,900 ns | 38,300 ns | **1.01×** |
| Go, Newton convergence | 7.44 ns | 7.84 ns | 1.05× |
| JS, search at index 6 | 6.8 ns | 7.0 ns | 1.03× |
| **JS, search at 99,998** | 66,995 ns | 64,208 ns | **0.96×** |

And every existing generated file is **unchanged**, on all three targets.

### What it retires, and what it does not

`fold-range2` is subsumed: `centroid` as a `loop` measures 31,492 ns against 31,414 for
`fold-range2` and 31,399 hand-written — the same, inside the noise floor. It is **kept for now** only
because retiring it means rewriting `examples/centroid.oro`, which is a separate change.

`fold-range` is **kept**, and the reason is measured rather than assumed. On compute-bound `dot` at
n = 1024: `fold-range` **451 ns**, `loop` **463 ns**, hand-written **465 ns**. The loop is at parity
with hand-written and 2.7% behind the fold, because the fold's bound is evaluated once and can be
narrowed for bounds-check elimination. 2.7% is inside the noise floor, and far below the 1.96× that
[bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) measured against an unnarrowed baseline —
because a `loop`'s guard `i < len(p)` is *itself* a proof the host can use.

## Consequences

**Termination becomes a program property**, computed rather than guaranteed — the shape
[ADR 0001](0001-parasite-model.md) already gives portability. A program using only `fold-range`,
`fold-range2` and `make-vec` provably terminates; one using `loop` may not.

**[ADR 0014](0014-recursion-is-not-in-the-language.md) stands.** `again` is a jump, not a call: no
frame, no stack. Recursion was rejected because stack depth differs by orders of magnitude across
the three hosts with no specification; a non-terminating `loop` hangs *identically* everywhere.
Unbounded iteration is the part of recursion that is portable.

**Refinements gained.** A `loop` states its bound as a guard, so the checker reads `i < alen a`
directly, and the negation of an earlier guard in a later clause. That is *more* than `fold-range`
offers, where the bound is implied by the primitive. `emit/refine.go` learned guard assumption and
guard negation, and `find-first` discharges its own `aindex` obligation with no `where` clause.

**Two reserved words**, `again` and `else`, rejected as binder names. That is the whole syntactic
cost.

**A latent Java bug surfaced.** `aindex` was `%s[%s]`, which compiled only because `fold-range`
declares its index variable as `int`; a `loop` variable is our `int`, which is Java's `long`, and the
same template then fails. Now `%s[(int) %s]`, with the refinement it should always have carried.
Eight generated Java files gained a cast — and their **bytecode is identical**, checked with `javap`.

**hamza's optimisation is in**: an `again` argument that *is* the loop variable emits no assignment.
It also shrinks the simultaneity problem, since an unchanged variable cannot be clobbered — so JS and
Java need temporaries only when a changed argument reads a changed variable, and Go, which has
parallel assignment, never needs them.

## Why not

**A guard in the loop condition** (`fold-while`) — 1.61× from defeated bounds-check elimination, and
one accumulator.

**A fold with a `done` marker** — Clojure's `reduced`, Rust's `ControlFlow`. Needs a sum type; see
[iteration.md §3b](../spec/iteration.md).

**`loop` + a general `cond`** — more orthogonal, and `cond` would be independently useful. Rejected
because `cond` is nested `if`, which would force `again` under `if` and lose the flat control flow.
If `cond` is wanted later it can be added without touching this.

**A named loop** — Scheme's named `let`. Costs no reserved word and would allow an inner loop to
target an outer one. **Deferred, not rejected**: it is a compatible extension and no program has
asked.

**A `tailrec`-marked `def`** — no new syntax, but the loop then needs a global name and every free
variable as a parameter. That is defunctionalisation by hand, which is why Scheme invented named
`let`.

## Still open

A **product type**. `loop` removed three of the four demands for one — n accumulators, early exit,
and multi-value loop state. The fourth remains: `v, ok := m[k]` needs a *value* carrying two things,
and nothing here provides that.
