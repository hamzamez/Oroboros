# Types, layer 1

Written before the code, per [state.md §6](state.md). The argument is in
[types-direction.md](../types-direction.md); the picture is in [types-sketch.md](../types-sketch.md).
This is the part being **built**.

> **Status, 2026-08-15. Built.** `emit/check.go` runs on the residual before emission, in
> `cmd/build` and `cmd/gen`. The §1 program is now rejected **identically on all three targets**:
>
> ```
> main: in argument 1 of io.print-line: in argument 1 of num/f64.add:
>       a string literal is string, but f64 is required here
> ```
>
> And §6 held: **every example, on every target, passes unchanged**, and every generated file is
> byte-identical. So no target file was lying this time — which was a live possibility given that
> `fold-range`'s declared type, `stmt`'s result type and `loop2`'s were each wrong at some point.
>
> One bug in the checker itself is worth recording: the first version skipped `KFn`, reasoning that
> a bare abstraction is an escaping closure the emitter rejects anyway. But **the top-level term of
> every residual is an abstraction**, so it checked nothing at all and passed the §1 program. A
> checker that accepts everything is indistinguishable from a correct one until you have a case it
> must reject.

---

## 1. The gap, measured

```lisp
(def main (fn () (io.print-line (f64.add "hello" 1.0))))
```

| | |
|---|---|
| reduction | happy — β and δ are untyped |
| Go | build fails, with the message pointing at *generated* code |
| Java | build fails |
| **JavaScript** | **builds and prints `hello1`** |

So the effective type safety of an Oroboros program depends on which target it was emitted for,
which is the opposite of what this project claims. That is the whole of the case for a checker of
our own, and it is a measurement rather than a principle.

## 2. What is checked, and where

> **The residual, before emission.**

Not the source. The residual is **monomorphic, first-order and closed**
([types-direction §3.3](../types-direction.md)) — reduction has already specialised every generic
definition and every backend refuses an escaping closure — so checking it needs no type schemes, no
generalisation, and no unification beyond the trivial.

This is the cheap half, and it is where the measured bug lives.

**Not in a backend.** `targets/js.oro` declares argument types it never uses, and the file says why:
*"they document the primitive and a future checker could use them regardless of target."* That
prediction is what makes one checker serve three targets, including the one with no type layer at
all.

## 3. The judgement

A type is one of the target's declared type names, or **unknown**.

```
type(integer literal)      = int
type(float literal)        = f64
type(string literal)       = string
type(name)                 = whatever has been demanded of it, or unknown
type((p a…))               = p's declared result type
```

Checking is a walk. At every primitive application, each argument is **demanded** to have the
primitive's declared argument type:

| the argument's type | what happens |
|---|---|
| equal to the demand | fine |
| **unknown**, and the argument is a name | the name is **bound** to the demanded type — this is the inference half |
| unknown, otherwise | fine; nothing is learned |
| a **different** concrete type | **error** |

`any` demands nothing ([target-files.md §2](target-files.md)), so it never conflicts and never
binds.

**Inference and checking are the same walk.** The emitters already infer this way and keep the
first answer they find; the only change is that a second, different answer is now an error instead
of being discarded.

## 4. The structural primitives

Their types live in the backend rather than the target file
([target-files.md §4](target-files.md)), so the checker knows them the same way the emitter does:

| | |
|---|---|
| `loop` — `(fold-range z n f)` | `n : int`; `f`'s parameters are the accumulator's type and `int`; the body must match the accumulator |
| `loop2` | both accumulators `f64`, index `int` |
| `cond` — `(if c t e)` | `c : bool`; the branches must agree |
| `let` | the binder takes the value's type |
| `build` — `(make-vec n f)` | `n : int`; `f`'s parameter is `int`; the body must be `f64`; the result is `vec-f64` |

## 5. What this deliberately does not do yet

- **No `sig` on exports.** Declaring argument and result types on a module's exports, and checking
  a *target's native implementation* against the *library's* signature, is the job no backend can
  do — [types-direction §3.1](../types-direction.md) argued it is the strongest reason for types at
  all. It is the next increment, not this one, because the measured bug in §1 does not need it.
- **No refinements.** `{int | 0 ≤ i < n}` is layer 1's other half and needs a solver.
- **No proofs.** Layer 2 ([types-sketch §7](../types-sketch.md)).
- **No polymorphism.** The residual has none left; the *source* does, and checking source would
  need it.

## 6. What must not happen

The checker must **reject no program that is currently correct.** Every example, every gauntlet
program, and every generated file must pass unchanged. If one does not, either the checker is wrong
or a target file is — and given the record, the target file is a live possibility: `fold-range`'s
declared type was false for months, `stmt`'s result type was never implemented, and `loop2`'s was
correct only by luck.

That is the acceptance test, and it is more interesting than the negative one.
