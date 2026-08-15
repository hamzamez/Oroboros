# Constructing data

Written before the code, per [state.md §6](state.md).

> **Status, 2026-08-15. Built, on all three targets.** `examples/build-vec.oro` constructs an
> array and prints its length, identically from Go, node and java.
>
> Two things fell out. `int → f64` was added, because §7's example needs it and the answer was
> already decided: the conversion is **exactly lossless inside the portable range**, since that
> range is 2⁵³ *precisely so* an f64 holds every integer in it ([arithmetic.md §4](arithmetic.md)).
> The lossy direction stays absent.
>
> And declaring Go's `int` as `int64` turned out to have **never been tested**: `:=` infers Go's
> own `int` everywhere, so the mismatch only appears when the declared type is *used* — in a
> function's return type. `alen` and `slen` now convert, and loop counters are explicitly `int64`.
> Measured: **33,504 ns/op against 33,600 hand-written**, unchanged. Java hides the same bug behind
> implicit widening, which is the second time Go has caught something the other two would not
> have.

The language cannot build an array. Every gauntlet program takes its data as a **parameter**, and
`main` takes no arguments — so `examples/hello.oro` prints `21 + 21` not because it is a hello
world but because that is the most a program can currently do.

This was found by *building*, not by auditing: writing `hello.oro` meant inventing `make3`, and it
did not exist.

---

## 1. What is actually missing

Checking the primitive set: `split-words` produces a `vec-string` from a string, `dict-empty`
produces a dict. **Nothing produces a `vec-f64` at all**, and nothing produces any collection
element-wise from a computation.

So the gap is not "no data structures" — it is:

> **A program can compute over data it is given, and a program given nothing can compute over
> nothing.**

## 2. Why not `make-vec` as the obvious primitive

The instinct is `(make-vec n f)` returning a real array. That is right as the *primitive* and
wrong as the *interface*, and the stencil benchmark says why.

A constructed array is a **runtime value**, so `(sum (make-vec n f))` cannot fuse: the array is
really built and then consumed. That is exactly the `materialised` form measured at
**103,509 ns/op and 512 KB against 7,946 ns/op** — a 13× penalty
([arithmetic.md](arithmetic.md) status) — and making it the natural way to write a program would
hand every user that penalty by default.

## 3. The design is already in the library

`lib/num/vec.oro` already constructs vectors. `(vec n f)` is a length paired with an index
function; it has no runtime existence and reduces away. What is missing is only the other
direction:

> **`materialize` turns a delayed vector into a real host array.**

Everything else stays in the library and keeps fusing. You materialize **only at a boundary** —
handing an array to a host function, or returning one.

Which is [g5 §1](../derivations/g5-bindings.md) for the fourth time: *representation is free in the
interior and fixed at the boundary.*

### The primitive cannot take a delayed vector

A delayed vector reduces to a **λ**. Passing one to a primitive leaves a bare abstraction in the
residual, which all three backends refuse as an escaping closure. So the primitive takes the
*pieces*, exactly as `fold-range` does:

```lisp
(make-vec n (fn (i) element))          ; the primitive: a length and an element function
```

and the library supplies the interface:

```lisp
(def materialize (fn (v) (make-vec (vlen v) (fn (i) (vindex v i)))))
```

`vlen` and `vindex` reduce away because `v`'s constructor is statically known, so the emitted loop
sees the element expression directly.

`make-vec` is declared **unqualified**, beside `alen` and `aindex`, because it is the same layer:
raw operations on the host's array type. `num/vec.materialize` is the library on top.

## 4. A fifth structural kind — and a correction

`make-vec` must allocate and fill, which needs a bound index variable and a loop header. No `%s`
template expresses that, so it is **structural**: named in data, implemented in the backend.

That makes five, and [arithmetic.md §2](arithmetic.md) claimed the four were *exactly* the
eliminators whose scrutinee is dynamic, and that this was why the set was closed. **`make-vec` is a
constructor, not an eliminator.** So that characterisation was **incomplete rather than wrong** —
it explained the four we had and did not describe the space.

The two properties were conflated, and they are separate questions:

| question | answer |
|---|---|
| Why is it **primitive** rather than a library definition? | Reduction cannot produce it — either its eliminator's scrutinee is dynamic, so an encoding does not erase, **or it allocates** |
| Why is it **structural** rather than a template? | Its emission binds a variable or emits control flow |

`cond`, `loop`, `loop2` and `let` are primitive for the first reason. **`make-vec` is primitive for
the second**, which is a reason the earlier statement did not have. The set was never closed; it
was merely complete for the eliminators.

## 5. What it must emit

```go
n   := <length>
dst := make([]float64, n)
for i := 0; i < n; i++ {
    dst[i] = <element>
}
```

- The length is evaluated **once**, before the loop, as for `loop`
  ([target-files.md §4](target-files.md)).
- `dst` is fresh and every element is written exactly once, so **no narrowing is needed** — Go
  already knows `len(dst) == n` from `make`.
- The element function is `(fn (i) …)`, one parameter, of type `int`.

## 6. Fusion: it deliberately does not

`(sum (materialize v))` **builds the array and then folds it**, and costs the 13× of §2. That is
correct and should be documented rather than optimised away:

> **Materializing in the interior is a mistake the language lets you make, and the cost is the
> point.** `materialize` marks a boundary. A program that materializes where it did not need to has
> said something it did not mean.

A later pass could recognise `(consume (make-vec n f))` and fuse it, which is classic
deforestation. It is **not** proposed now: no program needs it, and δ+β already fuse everything
written in the delayed representation, which is the representation the library encourages.

## 7. What this unlocks

- **`main` can build data**, so a program with no arguments stops being a constant-printer.
- **[g7](../derivations/g7-aliasing.md)'s stencil write half becomes expressible.** The read half
  arrived with integer arithmetic; `dst[i] = (a[i-1]+a[i]+a[i+1])/3` becomes
  `(materialize (vec (sub n 2) (fn (j) …)))`.
- **Aliasing is sidestepped, not solved.** `make-vec` allocates fresh and writes each element
  once, so the destination is **unique by construction**. g7's question — the oldest open one —
  does not have to be answered for its program to land.

  > **Measured 2026-08-15, and the price is 1.87×**
  > ([stencil](../../gauntlet/results/stencil-2026-08-15.md)). Generated code is at parity with
  > hand-written *functional* code, and loses to a hand-written stencil that reuses buffers. By
  > the gauntlet's standard that is a **fail**, and the first in the project. The compiler is not
  > what loses — `materialize` is.

## 8. Deliberately absent

- **Mutating an existing array.** `(set! a i x)` is where aliasing becomes real, and §7 is the
  argument for not needing it yet.
- **Constructing anything but `vec-f64`.** A `vec-string` or a dict built element-wise needs either
  another primitive per element type or a type system. One suffices for the programs that exist.
- **Array literals.** `[1.0 2.0 3.0]` is sugar over `make-vec` with a constant function, and no
  program has asked.
- **Deforestation.** §6.
