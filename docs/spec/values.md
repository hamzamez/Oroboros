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

## 4. What each target declares

```lisp
(multi-return "RESULT-TYPES-FORM" "RETURN-FORM")
```

Two templates, each taking the comma-joined list.

| target | declaration | emits |
|---|---|---|
| **Go** | `(multi-return "(%s)" "return %s")` | `func F(…) (int, int) { return a, b }` — in registers |
| **JavaScript** | `(multi-return "" "return [%s]")` | `return [a, b];` |
| **Java** | *none* | **refuses** |
| **windows** | *none* | **refuses** |

**JavaScript has no multiple return**, so the declaration picks an idiom and names its price rather
than hiding it: product-2026-08-19 measured an array literal at **1.32×** against an object literal
at **1.11×** for a create-and-consume pair. The array is what `const [a, b] = f()` reads; a target
author who would rather pay 1.11× for `{_0, _1}` changes one string and no Go.

**Java and windows refuse, and that is a capability answer rather than a gap in the compiler.**
Java's only form is a generated record type — a real design question about naming and about who
owns the type — and the Win64 ABI returns one value in `rax`, so a two-register convention would be
ours and not the platform's. A program that needs several results does not cover on those targets
and the error says so:

```
divmod declares 2 results and target "java" has no multiple-return form.
  The program does not cover here — a capability answer, not a syntax error.
  Declare (multi-return "types" "return") in the target, or return one value.
```

This is exactly JavaScript declaring no `checked` primitive, arriving from the other direction.

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
