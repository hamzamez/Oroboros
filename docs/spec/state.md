# The state of the language

Read off the code, not from memory. Everything here is checkable by grep.

---

## 1. The whole language

**Six term kinds.** That is the entire grammar of what a program can say.

```
term ::= name | integer | float | string | (fn (name…) term) | (term term…)
```

**Four top-level forms.** One introduces a definition; three are module bookkeeping and are
erased before reduction ([modules.md](modules.md)).

```
(def name term)
(module path)  (use path [as alias])  (export name…)
```

**Three special forms in the reader**, two of which are sugar. `fn` (also spelled `λ`); `let`,
where `(let e k)` reads as `(k e)` ([def.md](def.md)); and `seq`, where `(seq a b)` reads as
`((fn (_) b) a)` ([effects.md §5](effects.md)). Neither sugar survives the reader.

**Two reduction rules.** β with call-by-need, and δ over definitions. β carries one side
condition: an impure argument is let-bound rather than substituted ([effects.md §4](effects.md)).

**Two parameters.** Which names are primitive, and which of those are pure — both supplied by a
target file. The first is [ADR 0002](../decisions/0002-capability-graph.md)'s capability set. The
second decides whether β may move a term, and defaults to *impure*, so that a target author's
omission costs speed rather than correctness.

That is all of it. 1,058 lines in `core/`, 28 tests.

Arithmetic, booleans, comparison and equality live in the modules `num/f64`, `num/int` and
`logic` ([arithmetic.md](arithmetic.md)) — **not** in the language. An `int` is a mathematical
integer whose portable range is ±(2⁵³−1), which is JavaScript's limit and the only range on which
all three targets agree exactly.

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
| Arithmetic evaluation | `(num/int.add 1 2)` does not fold. No primitive is ever evaluated |
| Pattern matching | none — ι of Coq's βδιζη |
| Extensionality | none — η |
| Effect *types* | none. Purity is one declared bit per primitive; g5's ordering discipline is a side condition on β ([effects.md](effects.md)) |
| Modules | **scopes, resolution, imports, exports** ([modules.md](modules.md)); emitted functions are named after their export. Not yet: file-per-module |
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

## 4b. Effects, which were specified before they were added

The correct order, and the first time this project has used it. [effects.md](effects.md) was
written before any code.

The finding that reordered the whole question: **effects were already here.** `dict-inc` mutates a
dictionary in place and `dict-empty` has a fresh identity, both since word count. Program 5 did not
introduce effects, it made them observable.

> **Purity is the licence to use the structural rules.** A pure term may be copied, dropped and
> moved. An impure term is *ordered*: exactly once, where it was written.

Denying contraction, weakening and exchange to impure terms is the whole discipline, and it needs
no effect types, no monads, and no linear types on values. It cost the six existing programs
nothing [measurable](../../gauntlet/results/effects-2026-08-14.md).

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
- **Sequencing with no sequencing construct.** `seq` is a β-redex whose binder is unused, and it
  works because β refuses to drop an impure argument — the ordering discipline and the ability to
  write a statement sequence are the same mechanism.

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
