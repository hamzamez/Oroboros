# The state of the language

Read off the code, not from memory. Everything here is checkable by grep.

---

## 1. The whole language

**Six term kinds.** That is the entire grammar of what a program can say.

```
term ::= name | integer | float | string | (fn (name…) term) | (term term…)
```


`core.Term` has a **seventh** kind, `KBound`, which no program can write and which the grammar
above therefore omits. A bound variable is stored as *"the parameter of the binder N levels out"*
rather than as a name — the locally nameless representation — so two variables cannot be merged by
sharing a spelling. The names in `Params` are hints kept only so emitted code reads well. This is
noted because someone auditing `core/term.go` against this document will count seven and should
know which one is not language.

**A program's entry point is an export named `main` taking no arguments**
([build.md §2](build.md)). `(fn () …)` is legal; `()` alone is still not a term.

**Five top-level forms.** One introduces a definition, one types it, and three are module
bookkeeping; all but `def` are erased before reduction ([modules.md](modules.md)).

```
(def name term)
(sig name ((param type)…) result [(where pred)])
(module path)  (use path [as alias])  (export name…)
```

A `def` or `fn` name is a **simple** name: `.` qualifies a member of an imported module, so it
cannot appear in a binder ([def.md §11](def.md), [chapter 2 §2.4](../book/02-def.md)). An `export`
or a `sig` must name a definition in the same module; naming nothing is an error, not a no-op.

**Three special forms in the reader**, two of which are sugar. `fn` (also spelled `λ`); `let`,
where `(let e k)` reads as `(k e)` ([def.md](def.md)); and `seq`, where `(seq a b)` reads as
`((fn (_) b) a)` ([effects.md §5](effects.md)). Neither sugar survives the reader.

**Two reduction rules.** β with call-by-need, and δ over definitions. β carries one side
condition: an impure argument is let-bound rather than substituted ([effects.md §4](effects.md)).

**No recursion.** A definition defined in terms of itself is an error, checked per-target before
reduction ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). δ still declines to
unfold a cycle, so the reducer stays correct on a term the front-end rejects. Iteration is
`fold-range`. Removing recursion did not make the language terminating — self-application still
diverges and is still guarded by fuel ([pcf.md §9](pcf.md)).

**Two parameters.** Which names are primitive, and which of those are pure — both supplied by a
target file. The first is [ADR 0002](../decisions/0002-capability-graph.md)'s capability set. The
second decides whether β may move a term, and defaults to *impure*, so that a target author's
omission costs speed rather than correctness.

That is all of it. 1,887 lines in `core/`, 47 tests there and 20 in `emit/`.

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
| Types in the *language* | none — no annotations. `(sig …)` is a **claim about a definition**, checked against the residual and against any target providing the name natively ([types.md §7](types.md)); it is not a type on a term |
| Type **checking** | **yes**, on the residual before emission ([types.md](types.md)). One checker, three targets, including the one with no type layer |
| Data structures | **none.** `string`, `vec-f64`, `dict` are opaque handles only primitives touch |
| Arithmetic evaluation | `(num/int.add 1 2)` does not fold. No primitive is ever evaluated |
| Pattern matching | none — ι of Coq's βδιζη |
| Extensionality | none — η |
| Effect *types* | none. Purity is one declared bit per primitive; g5's ordering discipline is a side condition on β ([effects.md](effects.md)) |
| Modules | **scopes, resolution, imports, exports, and files** ([modules.md](modules.md)); emitted functions are named after their export. Not yet: a target as a directory |
| Unbounded iteration | **absent** — the language is exactly primitive recursive. [iteration.md](iteration.md) proposes `fold-while`, one primitive, to close it |
| Recursion | **rejected**, per-target, before reduction ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). The `def`/`rec` split is withdrawn with it — nothing is left to opt into ([def.md §3](def.md)) |
| Tail-call optimisation | not guaranteed, and moot: no target provides it and there is no recursion to optimise ([def.md §10](def.md)) |
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
