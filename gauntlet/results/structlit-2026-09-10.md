# Struct literals are built, and the projection was its own last correction

2026-09-10. `gauntlet/stdlib/survey.go`,
`gauntlet/stdlib/acceptance/struct-literal.oro`. What
[struct-literals.md](../../docs/struct-literals.md) recommended: *"build it, and
expect a correction."*

> **A PROGRAM BUILDS A HOST VALUE NOTHING HANDED IT.**
> `image.Rectangle{Max: image.Point{X:10,Y:5}, Min: image.Point{X:2,Y:1}}` — a
> composite literal whose operands are composite literals — with every
> declaration GENERATED, built and run, printing **8** and **4**. The generator
> writes **462 constructors** and goes **4,311 -> 4,773 primitives**.
>
> **AND THE CORRECTION CAME, AT THE EXACT PLACE THE RESEARCH NAMED IT.** The
> projection was **+486 names, 55.2%**. The measurement is **+229, 45.4% ->
> 50.0%** — and the difference is **not** the corrections found on the way. Run
> the old method on today's data and it reports **+489**, reproducing itself: the
> gap is the METHOD.
>
> **SEEDING A FIXED POINT IS NOT RUNNING IT.** The projection SEEDED every
> constructible type into the obtainable set, which ASSERTS the type is
> buildable. A constructor is an ordinary function, so it belongs INSIDE the
> fixed point, which ASKS whether its FIELDS can be built — and **229 of the 462
> want a field nothing can build**. Declarable, and not callable.
>
> **AND THE LITERAL'S FORM IS DECIDED BY THE METHOD SET**, which the research had
> not seen: `&T{…}` is a `*T` and `T{…}` is a `T`, and our checker compares type
> names, so whichever is generated is the only method set the program reaches.
> Pointer iff the manifest gives the type a pointer-receiver method — worth
> **+78 names**, and what makes `image.Rectangle` usable at all.
>
> **Cost: no compiler change, no format change, no backend change.** Nothing
> under `core/`, `emit/`, `targets/` or `lib/` is touched, so no emitted file can
> move; the differential suite is green on 30 cases and four targets.

---

## 1. The algebra, unchanged by the build

A struct is `Π` over a finite set of LABELS, and the constructor is

```
    mk_T : Π_{f ∈ writable(T)} T_f  →  T          ⟦mk_T⟧ = T{f: …}
```

with `writable(T)` the exported fields, because a composite literal names the
fields it sets and a Go program outside the package sets exactly those. An
unexported or unspellable field is **left zero**, which is not a restriction we
impose — it is what the host permits and what a Go programmer writes.

products.md §7 defers the heterogeneous product for want of a flat LAYOUT, and
that deferral is about a product **we** represent. Here the value is built by the
host, held as an opaque token and handed back to the host; we never index it and
never lay it out. `HANDLE`'s shape a fourth time — and the build CONFIRMED it,
because the nested case is where a layout question would have had to appear and
it does not: the inner value is another token built by its own constructor.

## 2. The correction, and why it is the method rather than the data

| | usable | of 4,932 |
|---|---:|---:|
| before | 2,237 | 45.4% |
| struct-literals.md projected | 2,723 | 55.2% |
| **the same method, on today's data** | **2,726** | **55.3%** |
| **measured** | **2,466** | **50.0%** |

The third row is the one that settles it. Two real corrections were found while
building — a field type carrying the manifest's issue number (`OmitHost bool
#46059`) was unspellable, and 50 more struct types were registered once
`type T struct #12345` parsed at all — and together they move the projection by
**three names**.

**The gap is that a fixed point was SEEDED instead of RUN.**

`obtainable` is the least set of host types a program can get hold of: a type is
obtainable when some declarable function returns it **and every argument that
function needs is a scalar or already obtainable**. A struct constructor is
exactly such a function. Putting the constructibles into the ANSWER asserts they
are all buildable; putting the constructors into the INPUT asks the question, and
the question has a different answer for **229 of 462** of them — `&tls.Config{…}`
wants fields whose types nothing returns, and a declaration nobody can call is a
declaration, not a capability.

> **The tool caught itself the only way it could: by being made to GENERATE the
> thing it had measured.** Sixth correction to a number this survey published,
> and the first that the survey found rather than the next question finding it.

## 3. The form of the literal is decided by the method set

This the research did not have, and it is worth **78 names**.

```
    &T{…} : *T      method set = value methods ∪ pointer methods
     T{…} :  T      method set = value methods
```

In Go the pointer form is strictly the more capable value, because a value method
is callable on an addressable pointer. **Our checker compares type names and gets
no auto-dereference**, so whichever form the generator writes is the only method
set the program can reach: `&image.Rectangle{…}` is a `ptr-image-Rectangle`, `Dx`
declares its receiver `image-Rectangle`, and the call is refused.

So: **`&T{…}` exactly when the manifest gives T a pointer-receiver method, and
`T{…}` otherwise.** `image.Point`, `image.Rectangle` and the `color` types have
value methods only and become usable; `http.MaxBytesError` and `os.PathError`
need the pointer and get it.

**The residue is counted rather than hidden: 52 types have BOTH kinds of receiver
and get the pointer form, so their value methods are out of reach.** The general
fix is an auto-dereference rule for a RECEIVER position in the checker, and that
is a compiler question rather than a generator one — a blanket `*T ≤ T`
subsumption edge would be UNSOUND, because an ordinary parameter of type `T` does
not accept a `*T` in Go either.

## 4. What it buys

```
STRUCT LITERALS: 702 struct types; 462 get a generated constructor
  393 have every exported field spellable; 69 have one we cannot spell,
  which a composite literal LEAVES ZERO -- what a Go program does.
  238 have no exported field and get NOTHING.
  2 are skipped: two packages share a base name and both define the type.
  233 of the 462 constructors are themselves USABLE.
  USABLE GOES 2237 -> 2466 (+229), 45.4% -> 50.0% of the callable surface.
  52 types have BOTH a value and a pointer method and get the pointer form.
  199 host types are buildable that nothing RETURNS.
```

**199 host types are buildable that no declarable function returns**, and that is
what the percentage hangs off: `os.PathError`, `image.Point`, `color.RGBA`,
`elf.Header64`, `dsa.PrivateKey`, `httptrace.GotConnInfo`. Every method on each of
them was unreachable yesterday.

**And 238 struct types still get NOTHING, deliberately.** `&bytes.Buffer{}` is the
documented idiom and `&os.File{}` is a broken file; the manifest — the exported
API — cannot tell them apart, because the difference is a sentence in a doc
comment. **A generator does not make a claim it cannot justify**, which is the
rule `pure` was removed under. That stays a hand declaration, where somebody can
be answerable for it.

## 4a. The named risk fired, and what caught it was a diff

struct-literals.md's first named risk was that **field ORDER must be sorted or the
emitter stops being a function of its input**, *"backend-2026-09-06, where it
would be easiest to miss"*. Sorting was written first, the acceptance program
passed, and the risk fired anyway.

**Two independent emits of the generator differed in five files.**

```
< (prim FileHeader (… (compressedSize (int 0 4294967295)) (compressedSize zip---deprecated) …)
> (prim FileHeader (… (compressedSize zip---deprecated) (compressedSize (int 0 4294967295)) …)
```

**`CompressedSize //deprecated` is not a field of type `//deprecated`.** The
manifest records HISTORY — the same reason `parseLine` keys symbols by identity
and takes the last — and a later file marks an existing field deprecated by
re-listing it with an ANNOTATION where the type goes. Reading that as a type gave
`archive/zip.FileHeader` **two `CompressedSize` fields**, which is two separate
defects at once:

- the emitted composite literal has a **duplicate key**, which Go refuses, so
  those five packages could never have compiled;
- a repeated name makes **sort-by-name a PARTIAL order**, and `sort.Slice` is not
  stable, so the coin was flipped on every run.

Fixed in the two places it should be: an annotation is not a type, and fields are
**deduped by name, last wins** — the rule `parseLine` already applies, for exactly
the same reason. Deduping is what makes sort-by-name a **total** order, and a
total order is what the risk was really about; sorting alone was necessary and not
sufficient.

> **The check is `diff -r` between two emits, and it is worth keeping as one.**
> backend-2026-09-06 found its non-determinism by running `cmd/gen` six times and
> noticing two answers; this found its own by generating twice into two
> directories. **A generator that is a function of its input can be tested by
> running it twice**, and nothing subtler is needed.

**Neither defect moves the headline**: 462 constructors, +229 names, 50.0%. What
it moves is the constructor-usable count, 229 → **233**, because four constructors
had a field whose type was an annotation and therefore unbuildable.

## 5. Acceptance, because a percentage that does not build is a claim

`gauntlet/stdlib/acceptance/struct-literal.oro`:

```lisp
(let (img.Rectangle (img.Point 10 5) (img.Point 2 1)) (fn (r)
  (seq (io.print-int (Rect.Dx r))
       (io.print-int (Rect.Dy r)))))
```

emits

```go
r := ((image.Rectangle{Max: ((image.Point{X: 10, Y: 5})), Min: ((image.Point{X: 2, Y: 1}))}))
var v1 int = (r.Dx())
var v2 int = (r.Dy())
```

and prints **8** then **4**. The pointer path is checked too:
`(&http.MaxBytesError{Limit: 5}).Error()` prints `http: request body too large`.

Three things the program is shaped to check.

**THE LITERAL NESTS**, which is struct-literals.md section 5's second named
risk. It does not reopen the layout question, for the reason in section 1.

**FIELDS ARE SORTED BY NAME**, which is that section's first risk: an unsorted map
iteration would make the emitter stop being a function of its input, and
backend-2026-09-06 is what happens when it does. `Rectangle` takes **Max then
Min** — which is why a generated parameter is named for its FIELD rather than
`a0`, `a1`: at a call site with eleven of them, the declaration is the only thing
that says which slot a value fills.

**THE LITERAL IS PARENTHESISED.** `&T{...}.M()` parses as `&(T{...}.M())`, which
is not addressable, and a bare composite literal in a `for` header or an `if`
condition is ambiguous in Go's own grammar. One pair of parentheses in the
template answers both.

### 5.1 And it prints two numbers rather than their product

`(* (Rect.Dx r) (Rect.Dy r))` is **refused**:

```
* [-inf, +inf] in (* (go/image/Rectangle.Dx r) (go/image/Rectangle.Dy r))
```

A host call's result has no declared range and ADR 0019 is bounded by default.
That is the standing gap win32-2026-09-06 and gostdlib-2026-09-06 both name, and
here it is CORRECT rather than a limitation: Go's `int` really is the host's word,
so the generator writing it as `int` is the honest reading and the refusal lands
in the right place. The program says what it can prove.

## 6. Cost

| | |
|---|---|
| compiler code | **none** — `core/`, `emit/`, `targets/`, `lib/` untouched |
| emitted files | **cannot move**, and did not: nothing they depend on changed |
| differential suite | 30 cases, four targets, green |
| unit tests, `go vet` | green |
| generated primitives | **4,311 -> 4,773** (+462 constructors) |
| new term kinds, reduction rules, format fields, backends | **none** |

The whole change is in one tool, which is what struct-literals.md predicted: *the
format needs nothing*.

## 7. What it leaves

**Usable is 50.0% and the residue has a shape now.** Of what stays declarable and
unusable: an interface we hold nothing for (75 names, tier 3, refused for the
reasons callbacks.md gives); a type no function returns and no literal
constructs; and **229 constructors whose fields need a type nothing can build** —
which is the same fixed point one turn further out, and would move again if any of
those fields became reachable.

**52 types owe an auto-dereference rule** in a receiver position — small,
compiler-side, and the first thing here that is not a generator question.

**And two of four targets have still never been priced.** No survey exists for
JavaScript or the JVM. The JDK is reflectable, which makes it the cheapest of the
four to write, and this week wrote file I/O on both by hand without knowing what
fraction of either that was.
