# `values` — several results, and why it is not a tuple

A function may return more than one value.

```lisp
(sig divmod ((a int) (b int)) (int int) (where (go.!= b 0)))
(def divmod (fn (a b) (values (go./ a b) (go.% a b))))
```

emits, on Go:

```go
func Divmod(a int, b int) (int, int) {
	return (a / b), (a % b)
}
```

**Measured at parity with hand-written Go and zero allocations** — 2.679 ns against 2.665, and
2.685 against 2.661 for the consumed form. Nothing is built in either.

## 1. It is not a data structure, and that is the whole design

`(values a b)` is **reader sugar**:

```
(values e₁ … eₙ)   ⟶   (fn (#k) (#k e₁ … eₙ))
```

A function that answers whichever observation you hand it. In linear logic this is `A & B` — the
**negative** product, introduced by giving a way to answer each projection and eliminated by
choosing one. Its β law *is* β, so **the reducer needs nothing**: three reduction rules before,
three after, seven term kinds before, seven after.

The binder is named `#k`, and `#` is not `isIdentStart`, so no source term can contain a free
occurrence of it and the sugar cannot capture one. (`seq` uses `_`, which a user *can* write.)

Scheme's `values`/`call-with-values` and Common Lisp's `values`/`multiple-value-bind` are
specified the same way and for the same reason: an implementation should return several results in
registers rather than box them so the caller can immediately unbox them. Go's `(int, error)` is
the same idea with types. This is deliberately **not** a tuple —
[data-structures.md §4.5](../data-structures.md) records that all six demands the language has
accumulated are multiple-*return*, not data modelling.

## 2. Two cases, and only the second costs anything

**Consumed in the same reduction — free, and already worked.**

```lisp
(def f (fn (a b) ((divmod a b) (fn (q r) (go.+ q r)))))
```

reduces to `(fn (a b) (go.+ (go./ a b) (go.% a b)))`. The product is gone before any backend sees
it. This is why the shape measured 1.01× with zero allocations
([product-2026-08-19](../../gauntlet/results/product-2026-08-19.md)), and it needed no work at all
— it is what β does.

**Surviving to a boundary — the target's own form.** An exported function is a boundary: the
caller is hand-written host code and reduction cannot see it. Then the selector-taking lambda
reaches the emitter, and the target says what to do with it.

## 3. The signature is what decides

`(fn (k) (k a b))` read one way is a pair; read the other way it is a genuine higher-order
function — "apply my argument to these two things". **They are the same term.** So the
disambiguator is the declaration:

| | |
|---|---|
| `(sig f (…) (int int))` | a product — emit the target's multiple return |
| `(sig f (…) any)` | an escaping closure — **still refused** |

This is the same shape as a declared range selecting an integer's representation
([selection-2026-08-19](../../gauntlet/results/selection-2026-08-19.md)): the source says what it
means, and the target says what that costs.

`(sig f (…) (int))` — one result in a list — normalises to the bare result, so there is exactly
one spelling for one result and nothing ambiguous can reach a backend. `(values x)` with one
operand is refused for the same reason: one value is just the value.

## 4. What each target does — and none of it is declared

**No target declares this.** `values` is a language construct, so it works on every target and the
compiler finds the implementation, exactly like `if`, `let` and `loop`. There is no line to write
and none to forget.

| target | form | measured |
|---|---|---|
| **Go** | `func f(…) (int, int)` — two values in registers | **0.99×**, 0 allocs |
| **Java** | a generated `record`, shared by result *shape* | **0.97×**, and 1.01× against no product at all |
| **JavaScript** | `return {f0, f1}` — an object literal | **1.62× faster than an array** when the caller reads a property, identical when it destructures |
| **windows** | `rax`, `rdx` — our convention, mirroring Win64's argument convention | free by construction |

[multiresult-2026-08-22](../../gauntlet/results/multiresult-2026-08-22.md) has the numbers and the
one bug this build found: x86 needs **two passes**, because placing a result into `rax` as it is
computed is clobbered by the next `idiv`.

**On JavaScript, tell the caller to destructure.** `const {f0, f1} = f(x)` costs nothing — V8
scalar-replaces the object and lands on the no-product number — while `const p = f(x); … p.f0 …`
keeps the allocation, at **5.4×**. That is a property of the *call site*, not of what we emit.

**The first version of this document described a `(multi-return "…" "…")` declaration and said Java
and windows refuse.** That was reverted: a construct in the core that a target can decline is a
library carrying a portability claim, which is the thing
[ADR 0001](../decisions/0001-parasite-model.md) exists to refuse. Java's generated record and
windows' register convention are what avoiding that work was hiding, and both turned out to be
small.

## 5. What this does not do

**It does not give the language a product type.** There is no way to *store* several values, put
them in an array, or pass them as one thing. `values` reaches a boundary and stops. That is the
recommendation in [data-structures.md §7](../data-structures.md) and the restriction is the point:
a stored product is positive, must be built, and costs an allocation wherever `materialize` costs
one.

**It does not let a target primitive return several values.** `strings.Cut` returns three results,
`Fprintf` returns `(int, error)`, and Go's comma-ok forms return two — all still undeclarable, and
[targets/go/strings.oro](../../targets/go/strings.oro) and `fmt.oro` both record it. That is the
*call* half of the same feature and it is the obvious next increment: the elimination form is
`let` with an n-ary continuation, and `let` is already a structural primitive.

**And it does not type-check the results.** The residual checker verifies one result against the
signature; the multi-result path does not yet. The arity is checked; the types are not.
