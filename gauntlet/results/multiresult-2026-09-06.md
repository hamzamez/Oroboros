# A program that opens a file

2026-09-06. [assessment-2026-09-06](../../docs/assessment-2026-09-06.md)'s item 2:
the two format changes the ecosystem surveys located, with **a program that opens
a file** as the acceptance test rather than a percentage.

> **Go's callable standard library goes from 70.0% declarable and 19.0% usable to
> 88.0% and 32.0%**, and the host types a program can get hold of go from 108 to
> 198. `os` goes from 18 usable names to 76.
>
> And the language needed **nothing**. Both changes are fields in a data format
> and one branch in an analysis.

---

## 1. Several results in a `prim`

`Prim.Result` was a single Go string. That one field refused **19.8% of Go's
callable standard library** — more than every language-level limitation put
together — and it compounded, because `(T, error)` is Go's **constructor** idiom,
so every name it blocked also blocked every method on the type that name would
have returned ([gostdlib-2026-09-06 §4a](gostdlib-2026-09-06.md)).

**It is why this language had no file I/O, and the reason was never a design
position.**

### 1.1 The language already had the construct

`(values a b)` has been the negative product since
[values.md](../../docs/spec/values.md) — reader sugar for `(fn (#k) (#k a b))`,
so the reducer needed nothing then and needs nothing now — measured at **0.99x on
Go with zero allocations**, and **no target declares it**. The elimination form
has been a differential case since August:

```lisp
((divmod2 n) (fn (a b) (+ a b)))
```

What was missing was only the ability for a **target** to say that a *host* call
has that shape. And the consuming side needs no new machinery either, which is
the part that makes this cheap:

> When the producer is a `def`, β performs that application and the product
> vanishes. When the producer is a **`prim`**, β cannot — the operator of the
> outer application is itself an application — so the redex is **stuck**, and the
> shape arrives at the backend intact.

That is the same shape `emitMapCase` has handled since maps landed: an operator
that is legitimately not a name. A fallible map read *is* a host call with two
results, and [maps.md](../../docs/spec/maps.md) lowered it to Go's comma-ok
before this generalisation existed. This is that special case made general.

### 1.2 What was built

`Prim.Results []string`, populated only when there is more than one — exactly as
`core.Sig` does it, so every existing path is untouched.

**The one subtlety is telling a result LIST from a compound TYPE.** `(int int)`
is two results; `(array int)`, `(int 0 255)` and `(map int int)` are one. Both are
an application of names, and only `core.TypeName` knows which constructors exist —
so `resultList` consults it first rather than counting kids.

On Go the emission is the host's own form, with no product built:

```go
src, err := os.ReadFile("go.mod")
```

**A result the body never reads still has to be received**, because Go's multiple
assignment is positional; and it must be `_`, because an unused variable is a
compile error there. When *every* result is discarded the assignment goes away
entirely, since Go rejects `_, _ := f()`. That is `effects.md §5`'s rule — a `let`
with an unused binder emits a bare expression — arriving at a construct that
cannot drop the slot.

### 1.3 Two things the build found

**`typeOf` did not know the shape**, so the enclosing function's result came out
unknown and `main` was emitted with no result while its body returned one. The
fix is one branch and it is the same one the map read already needed, with the
comment already saying why: *the operator of `((m k) …)` is not a name and every
case below assumes it is.*

**And the type CHECKER skipped the arguments entirely.** `(os.ReadFile 42)` was
caught by the Go compiler, not by us — and
[types.md](../../docs/spec/types.md) says this checker exists precisely because
on JavaScript nothing else would. The non-name operator was returning early with
*"the emitter reports this better than the checker can"*, which is true of an
escaping closure and false of this. Now:

```
build: main: in go/os.ReadFile's argument 1: an integer literal is int,
       but string is required here
```

## 2. A declared result range, read by the interval layer

The other half, and it was measured on **two independent ecosystems before it was
built**. `emit/interval.go` returned ⊤ for an application of any primitive it did
not structurally recognise, so ADR 0019's bounded-by-default refused arithmetic
on **every host result in every ecosystem**, and `-checked` was the only way
through — `(fmt.print-int (GetCurrentProcessId))` on windows and
`bits.OnesCount64(n) + 1` on Go, one cause
([gostdlib §4b](gostdlib-2026-09-06.md), [win32 §5](win32-2026-09-06.md)).

A primitive has **no body**, so a declaration is the only source of the fact
there can be — the same reason `ensures` belongs on a `prim` and is redundant on
an internal definition ([postconditions.md](../../docs/spec/postconditions.md)).

```lisp
(prim ones ((x (int 0 4294967295))) (int 0 64) expr "bits.OnesCount64(uint64(%s))" pure)
```

It is a range in the **result position** rather than an `ensures`, because
[scalarrange-2026-08-31](scalarrange-2026-08-31.md) established that a range in a
result position *is* an `ensures` — the same claim written in the type language —
and because `ensures` feeds the **refinement** layer while being in-window is
decided by the **interval** layer. Two layers, and only one of them was ever
told. All three spellings had been tried and none worked; this is why.

**The first attempt died on a guard**, and it is worth recording. `transfer`
begins `if prim.Result != "int" && prim.Result != "" { return top, false }` —
which keeps `bool` and `f64` out of the arithmetic and also kept `int 0 64` out.
That is scalarrange's *three effects of a range* arriving in a fourth place: a
range is an `int` **for typing** and a distinct string in the type language.

## 3. The acceptance test

[examples/io/wc.oro](../../examples/io/wc.oro) — the first Oroboros program that
opens a file. It reads `go.mod`, counts the newlines, and prints **5**, which is
the truth.

```lisp
((os.ReadFile "go.mod") (fn (src err)
  (if (os.err-nil err)
      (loop ((i 0) (lines 0))
        (>= i (len src)) (fmt.Println lines)
        else (again (+ i 1) (if (= (src i) 10) (+ lines 1) lines)))
      (fmt.Println 0))))
```

```go
src, err := os.ReadFile("go.mod")
if (err == nil) { … if (int(src[i]) == 10) { … } … }
```

`src` is a `[]byte` because the target declares the element range, and it builds
**without `-checked`** because both facts the analysis needed are now declarable.

**`error` is opaque and that is honest.** A program can hold one, test it, and
hand it back — `(prim err-nil ((e error)) bool expr "%s == nil")` — which is what
most Go code does at the point of the call. Reading it needs an interface, which
is [callbacks.md](../../docs/spec/callbacks.md) tier 3 and stays refused.

## 4. What it moved

| | before | after |
|---|---:|---:|
| declarable | 70.0% | **88.0%** |
| usable | 19.0% | **32.0%** |
| — of those, error-only | — | 454 |
| declarable methods | 75.9% | **92.5%** |
| usable methods | 13.7% | **28.3%** |
| obtainable host types | 108 | **198** |

Per package, the ones a program reaches for:

| | total | decl (was) | usable (was) |
|---|---:|---:|---:|
| `os` | 136 | 130 (85) | **76** (18) |
| `math` | 67 | 67 (63) | 67 (63) |
| `math/bits` | 49 | 49 (37) | 49 (37) |
| `bytes` | 95 | 82 (62) | 76 (51) |
| `regexp` | 48 | 46 (38) | 37 (1) |
| `net/http` | 136 | 130 (101) | 35 (3) |
| `strconv` | 36 | 34 (26) | 32 (24) |
| `io` | 30 | 28 (10) | 6 (0) |
| `fmt` | 23 | 1 (1) | 0 (0) |

**Two of those numbers are the survey's own classification changing, not the
compiler**, and both are stated rather than folded in. `obtainable` now counts a
multi-result constructor, which is the point of the change rather than a
concession — `os.Open` returning `(*File, error)` is how you get a `*File`. And
an `error` beside a usable result no longer counts as a blocker, because
`err-nil` exists and `wc.oro` uses it; the 454 are reported separately, because
that count is the size of what sums.md would buy.

**`fmt` is still 1 of 23** — it is variadic `...any` all the way down, which is
now the largest single format refusal at 3.3%.

## 5. Cost

**Every pre-existing emitted file is byte-identical** — 63 of 63 across four
targets — and the one new file is the new program. Unit tests, `go vet` and the
27-case differential suite are green.

Four tests, each verified to fail against its bug: a `prim` declares several
results and a compound type is still one; the emission is Go's own multiple
assignment with no product built; an unread result becomes `_`; and a declared
result range is read where an undeclared one is not.

**There is no differential case**, and that is honest rather than an omission:
`os.ReadFile` is a *target-native* binding with no portability claim, and the
suite's question is whether four targets agree. JavaScript and Java signal
failure by throwing rather than by returning an error, so the same program is a
different shape there. What a differential case would test is the multi-result
*shape*, and `values-product.oro` already tests that on all four.

## 6. What is next, and what this did not touch

The gap is now **`cannot build the argument` at 43.6%** — a host type no
declarable function returns — and **`cannot read the result` at 20.1%**. Both are
the same underlying thing: an opaque value with no eliminator. That is
interfaces and structs, which this project refuses on arguments made elsewhere,
and the surveys now **price** those refusals rather than reopening them.

Nothing here touched the core, the reducer, ADR 0018 or the structural set. Seven
term kinds before and after.
