# Struct literals

Research, 2026-09-09. No decision. The residue
[interfaces.md §7](interfaces.md) pointed at: **1,867 declarable-and-unusable Go
names are blocked by something that is not an interface**, and the something is
mostly an argument built by literal.

> **THE FORMAT NEEDS NOTHING, AND IT IS BUILT AND RUN TO PROVE IT.**
>
> ```lisp
> (prim URL ((scheme string) (host string) (path string) (rawquery string))
>   ptr-url-URL expr "&url.URL{Scheme: %s, Host: %s, Path: %s, RawQuery: %s}")
> ```
>
> declared by hand today, called from a program, and it prints
> `https://example.com/a/b?x=1`. **A struct is a PRODUCT WITH LABELS and
> products.md §7 defers it for want of a LAYOUT — on a host that has structs
> there is no layout to invent**, because the host builds it. So this is a
> GENERATOR question, exactly as methods, several results and voids all turned
> out to be.
>
> **PROJECTED: +486 names, usable 45.4% → 55.2%** of the whole callable surface.
> Nearly ten points, and the largest lever left by a distance.
>
> **BUILT 2026-09-10, AND THAT NUMBER WAS WRONG BY 260 —
> [structlit-2026-09-10](../gauntlet/results/structlit-2026-09-10.md).** The
> delivery is **+229, 45.4% → 50.0%**, and the difference is the METHOD rather
> than the data: §2 below SEEDED the constructibles into the obtainable set,
> which asserts they are buildable, where a constructor is an ordinary function
> and belongs INSIDE the fixed point, which asks whether its FIELDS are. **229 of
> the 462 want a field nothing can build.** Everything else in this document
> survived, including both risks §5 names.
>
> **AND CONSTRUCTIBLE IS NOT USEFUL, so the rule is narrower than the number
> could have been.** A constructor is generated only for a struct with at least
> one spellable exported field. `&bytes.Buffer{}` is the documented idiom and
> `&os.File{}` is a broken file; the manifest cannot tell them apart, so a bare
> zero literal is a hand declaration — the `pure` rule again.

---

## 1. The algebra, and why the deferral does not apply

A struct is `Π_{f ∈ fields} T_f` — a product indexed by a **finite set of
labels** rather than by `Fin n`. products.md §2 says labels are surface and a
record is a tuple up to a bijection on the index set, and §7 defers the
heterogeneous product because **flattening is what gives a product a
representation** and a heterogeneous one has no flat form.

**That deferral is about a product WE represent.** Here the value lives on the
host, is built by the host, and is passed back to the host; we never index it and
never lay it out. What we hold is an opaque token — which is what a Win32
`HANDLE` has been since that target existed, and what
[gomethods-2026-09-09](../gauntlet/results/gomethods-2026-09-09.md) generalised
to every Go method.

So the construction is an ordinary primitive whose template happens to be a
composite literal:

```
    mk_T : Π_{f ∈ writable(T)} T_f  →  *T          ⟦mk_T⟧ = &T{f: …}
```

and `writable(T)` is the exported fields, because a composite literal names the
fields it sets and **a Go program outside the package can set exactly those**.
Unexported fields are left at their zero value, which is not a limitation we
impose: it is what the host permits and what Go programmers write.

## 2. Projected, before anything is generated

> **SUPERSEDED by structlit-2026-09-10, and kept because the error is the
> finding.** This section's number is reproducible — run this method on the
> corrected data and it still says +489 — so the 260-name gap is not the two bugs
> the build found. It is the seeding, called out in this section's own last
> paragraph and done anyway.

`gauntlet/stdlib/survey.go` now reports it:

```
STRUCT LITERALS: 652 struct types; 432 get a generated constructor
  344 have every exported field spellable; 88 have one we cannot spell,
  which a composite literal LEAVES ZERO -- what a Go program does.
  220 have no exported field and get NOTHING.
  USABLE WOULD GO 2237 -> 2723 (+486), 45.4% -> 55.2% of the callable surface.
```

**The fixed point is re-run with the constructibles seeded**, not merely added to
the obtainable set: a constructed `*http.Request` is an argument to functions
returning things nothing else reaches, so folding it in without re-running would
have been a lower bound.

**And re-running it with them SEEDED is still the wrong operation** — the
correction, written here rather than only in the result, because this paragraph
is where the mistake was made. Seeding puts the answer in; a constructor is a
function with arguments, and the fixed point exists to decide whether those
arguments can be built. Half of them cannot.

For comparison, every lever measured this week:

| | names | usable |
|---|---:|---|
| several results (2026-09-06) | — | 19.0% → 31.7%¹ |
| methods generated | — | — |
| a readable result | +751 | 29.3% → 44.5% |
| **the interface coercion** | **+41** | 44.5% → 45.4% |
| **struct literals**, projected | +486 | 45.4% → 55.2% |
| **struct literals**, built | **+229** | **45.4% → 50.0%** |

¹ later corrected; see gomethods-2026-09-09 §1 and interfaces.md §5.

## 3. Constructible is not useful, and the rule says so

This is the one place the number could have been dishonest.

`&os.File{}` is legal Go and a broken file. `&bytes.Buffer{}` is legal Go and the
**documented** idiom. The manifest — the exported API — cannot tell them apart,
because the difference is a sentence in a doc comment.

> **A generator does not make a claim it cannot justify.** That is the rule
> `pure` was removed under, on the same tool, on the same day.

So the restriction is: **a constructor is generated only for a struct with at
least one spellable exported field.** If a program is setting `Timeout`, it meant
to build a `Client`; if there is nothing to set, the zero value's usefulness is a
per-type judgment and belongs in a hand-written target file, where somebody can
be answerable for it.

That costs **220 struct types** and it is the difference between +1,012 and +486.
The larger number is the one a survey written by the same hands as the language
would have reported.

## 4. Feasibility, checked rather than projected

Hand-written, built with `cmd/build`, run:

```lisp
(use go/url as u)
(use go/url/URL as URL)
(def main (fn ()
  (io.print-line (URL.String (u.URL "https" "example.com" "/a/b" "x=1")))))
```

prints `https://example.com/a/b?x=1`. Emitted Go:

```go
(&url.URL{Scheme: "https", Host: "example.com", Path: "/a/b", RawQuery: "x=1"}).String()
```

**No compiler change, no format change, no new term kind.** The whole of what is
missing is that `survey.go` does not write those lines.

## 5. What building it would need, and the risks

**The generator**: read `pkg P, type T struct, Field Type` (12,230 lines in the
manifest), keep the exported fields whose type `spell` can name, and write one
`(prim T ((f …)) ptr-P-T expr "&P.T{F: %s, …}")` per type.

**Three risks, named.**

**Field ORDER must be stable**, or the same declaration regenerates differently
and every "byte-identical" claim in this repository stops meaning anything —
backend-2026-09-06's non-deterministic emitter, arriving where it would be
easiest to miss. Sorted by field name.

> **THIS ONE FIRED, WITH THE SORTING ALREADY WRITTEN.** The manifest lists a
> deprecated field a second time with an ANNOTATION where the type goes
> (`CompressedSize //deprecated`), so five packages had a struct with the same
> field twice — a duplicate key in the emitted literal, and a PARTIAL order for
> `sort.Slice`, which is not stable. Sorting was necessary and not sufficient;
> what the risk is really about is a TOTAL order, so fields are deduped by name.
> Caught by `diff -r` between two emits, which is the whole test a generator that
> claims to be a function of its input needs.

**A field whose type is a struct we also construct** makes the composite literal
nest, and nesting is where a layout question could sneak back in. It does not:
the inner value is another opaque token, built by its own constructor and passed
by name.

**And the number will not survive contact unchanged.** 55.2% counts a name usable
when its arguments can be built; it does not count whether the resulting call
does anything. `&http.Client{Timeout: d}` is a real client and
`&tls.Config{ServerName: s}` is a real config, but some of the 486 will be
structs nobody would build that way. The acceptance test for this is a *program*,
as it was for methods — not a percentage.

## 6. Recommendation

**Build it, and expect a correction.** It is the largest measured lever left, it
needs no language change, no format change and no backend change, and the
feasibility is checked rather than argued.

> **DONE 2026-09-10, and the correction was this document's own number** —
> [structlit-2026-09-10](../gauntlet/results/structlit-2026-09-10.md). +229 rather
> than +486, **45.4% → 50.0%**, no compiler change, and one thing §5 did not
> foresee: **the FORM of the literal is decided by the method set**. `&T{…}` is a
> `*T` and `T{…}` is a `T`, and our checker compares type names and gets no
> auto-dereference — so the pointer form is generated exactly when the manifest
> gives the type a pointer-receiver method, which is worth 78 names and is what
> makes `image.Rectangle` usable at all. 52 types have both and lose their value
> methods; the fix is a receiver-position auto-dereference in the checker, which
> is the first thing here that is a compiler question.

Then the residue is what it has been all along: **interfaces we can supply
nothing for** (87 names, tier 3), **types no declarable function returns and no
literal constructs**, and — the part nobody has priced — **JavaScript and the
JVM, which have never been surveyed at all**.
