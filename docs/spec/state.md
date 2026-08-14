# The state of the language

Read off the code, not from memory. Everything here is checkable by grep.

---

## 1. The whole language

**Six term kinds.** That is the entire grammar of what a program can say.

```
term ::= name | integer | float | string | (fn (name…) term) | (term term…)
```

**One top-level form.**

```
(def name term)
```

**Two special forms in the reader.** `fn` (also spelled `λ`), and `let`, which is sugar —
`(let e k)` reads as `(k e)` and reduces like any other application
([def.md](def.md)).

**Two reduction rules.** β with call-by-need, and δ over definitions.

**One parameter.** Which names are primitive, supplied by a target file. That is
[ADR 0002](../decisions/0002-capability-graph.md)'s capability set, and it is now literally a
separate file.

That is all of it. 987 lines in `core/`, 21 tests.

## 2. What a program may *not* say

Removed 2026-08-14 after the addition of target files made it dead:

- `(prim …)` and `(target …)` in a program are now **errors**. They were silently accepted and
  ignored, which would let a program believe it had declared something. Primitives come from
  `targets/NAME.oro` and nowhere else.

## 3. What is absent, and deliberately

| | Status |
|---|---|
| Types | none in the language. Two of three backends need them; they live there ([js](../../gauntlet/results/js-2026-08-14.md)) |
| Data structures | **none.** `string`, `vec-f64`, `dict` are opaque handles only primitives touch |
| Arithmetic evaluation | `(add 1 2)` does not fold. No primitive is ever evaluated |
| Pattern matching | none — ι of Coq's βδιζη |
| Extensionality | none — η |
| Effects | none, so [g5](../derivations/g5-bindings.md)'s ordering discipline has no implementation |
| Modules | none. `cmd/gen` names emitted functions by position |
| `rec` | not implemented; `markRecursive` decides silently ([def.md §3](def.md)) |
| Escaping closures | all three backends refuse them |
| Symbols | **refused**, and that is a decision rather than a gap ([def.md §5](def.md)) |

## 4. Strings, which were added out of order

A term kind and a literal syntax were added to the reader so that **target files** could carry
emission templates. That was a compiler need, not a language need, and the language gained a
literal that **no program uses** — every quote in `examples/` is prose in a comment.

Specified retroactively in [strings.md](strings.md). The short version:

> A string is an immutable, opaque sequence of Unicode scalar values. Its representation is the
> target's and is never specified, because it is never exposed. The core provides a literal and
> nothing else.

That is portable only because it refuses to expose what diverges — `length` of `"🙂"` is **4** on
Go, **2** on JS and Java, and **1** if you count characters.

**Known smell:** `core.Term` is doing double duty as the language's term type *and* as the
s-expression parse tree for target files. A target file is not a program and should not have to
be one. If target files ever need something the language does not have, the split becomes forced.

## 5. What the language has that is unusual

Worth stating, because the absences above make it look smaller than it is.

- **Generics with no generics.** One definition instantiates at any number of types with no type
  parameters, no monomorphization pass, and no dictionary — [measured](../../gauntlet/results/generics-2026-08-14.md)
  as byte-identical machine code to hand-written monomorphic Go.
- **Fusion with no fusion rules.** β and δ alone; the intermediate structure never exists.
- **The normal form is a parameter.** One source, three targets, and the dictionary comes out
  *fused on Go and unfused on Java* because that is what each measured faster.
- **Call-by-need with no cost model.** Occurrence counting plus a four-case syntactic test, which
  the [measurements](../../gauntlet/results/callbyneed-2026-08-14.md) showed is enough.

## 6. The rule this document exists to enforce

**Nothing goes into the language without a specification that says how it behaves on every
target.** Strings were added without one and this is the correction.

The test for a proposed addition is not "is it useful" but:

1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement observable? If it is, the feature is Tier 2 and carries
   no portability claim.

Strings pass (3) only by having almost no operations. That is the honest reason they were cheap,
and the reason `length` is not in the core.
