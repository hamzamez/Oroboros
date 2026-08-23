# The state of the language

Read off the code, not from memory. Everything here is checkable by grep.

---

## 1. The whole language

**Seven term kinds.** That is the entire grammar of what a program can say.

```
term ::= name | integer | float | string | true | false
       | (fn (name…) term) | (term term…)
```

The two boolean literals arrived last, in
[ADR 0017](../decisions/0017-booleans-are-in-the-language.md), and they are the only literal kind
whose value set is finite and on which all four targets agree exactly. They are FORCED rather than
chosen: the reader's desugaring of `and` has to produce a false value, and the reader does not know
which target it is reading for.


`core.Term` has a **seventh** kind, `KBound`, which no program can write and which the grammar
above therefore omits. A bound variable is stored as *"the parameter of the binder N levels out"*
rather than as a name — the locally nameless representation — so two variables cannot be merged by
sharing a spelling. The names in `Params` are hints kept only so emitted code reads well. This is
noted because someone auditing `core/term.go` against this document will count seven and should
know which one is not language.

**A program's entry point is an export named `main` taking no arguments**
([build.md §2](build.md)). `(fn () …)` is legal; `()` alone is still not a term.

**Six top-level forms.** One introduces a definition, one types it, one declares a sum, and three
are module bookkeeping; all but `def` are erased before reduction ([modules.md](modules.md)).

```
(def name term)
(sig name ((param type)…) result [(where pred)])
(sum name (variant type)… )
(module path)  (use path [as alias])  (export name…)
```

`sum` was the sixth and arrived 2026-08-22. It is erased in the strongest sense: it does not
survive as a *concept* either, because what it produces is `def`s (§ below).

A `def` or `fn` name is a **simple** name: `.` qualifies a member of an imported module, so it
cannot appear in a binder ([def.md §11](def.md), [chapter 2 §2.4](../book/02-def.md)). An `export`
or a `sig` must name a definition in the same module; naming nothing is an error, not a no-op.

**Special forms in the reader.** `fn` (also spelled `λ`) is the only one that is not sugar.
`let`, where `(let e k)` reads as `(k e)` ([def.md](def.md)); `seq`, where `(seq a b)` reads as
`((fn (_) b) a)` ([effects.md §5](effects.md)); `loop`, which desugars to `(loop (fn (x…) …) z…)`
([iteration.md](iteration.md)); `and`, `or`, `not` and `cond`, which desugar to `if`
([booleans.md](booleans.md)); `values`, which desugars to `(fn (#k) (#k a b))`
([values.md](values.md)); and `match`, which desugars to a `loop`
([match.md](match.md)). **None of the sugar survives the reader** — a residual contains `fn`,
`let`, `if`, `loop` and nothing else structural.

**`case` is the one exception, and the reason is worth stating.** It is sugar, but it expands in
`Load` rather than in the reader, because the reader sees ONE FILE and a sum may be declared in
another — an imported error type is the ordinary case. By the time `Load` returns, `case` is gone
and what is left is `if` over a tag comparison ([sums.md](sums.md)).

**And those three are the LANGUAGE's, not a target's.** `if`, `let` and `loop` are injected into
every target and **declaring one is an error**; the backend implements them. `=` joined them on
2026-08-22 — integer equality, which each backend resolves to the host's own (`==`, `===`, `sete`)
so nothing is lowered further than the target requires, and which `match` needs because a target
cannot be allowed to spell it differently or forget it. A target's structural
set is normally empty, and the four native targets declare none. This is the general rule that
[ADR 0017](../decisions/0017-booleans-are-in-the-language.md) set for `if` alone: a construct
promoted to the language works on every target and the compiler finds the implementation. The
capability graph is for *target-native* names, where "this target cannot do it" is a true answer a
program can be told.

**A `sum` declaration is DEFINITIONS.** `(sum result (ok int) (err int))` generates constructors
and tag constants as ordinary defs, and `case` expands in `Load` — so the reducer, the module
system and every backend are unchanged by sums ([sums.md](sums.md)). Seven term kinds before,
seven after.

**Four reduction rules**, two of which have two clauses each. It was three until sums landed on
2026-08-22, and the count went up honestly rather than by relabelling.

| | |
|---|---|
| **β**, call-by-need | with one side condition: an impure argument is let-bound rather than substituted ([effects.md §4](effects.md)) |
| **δ** | unfolding a definition, declining a cycle |
| **evaluation on literals** | `(if true a b) → a`, and `(= i j) → true/false` on two *integer* literals |
| **commuting conversion** | push an eliminator through `if` and through `let` |

**The "only evaluation" claim is dead, and this is where it died.** Until sums,
`(if true a b) → a` was the single evaluation step reduction performed. A sum's tag is a literal
after reduction, so `(case (ok n) …)` reduced to `(if (= 0 0) …)` — the sum had vanished and left
a **tautological test** behind, which is a static cost the two-level language says should not
exist. Folding `=` on two integer literals removes it.

It is narrow deliberately: integers only, `=` only.
[ADR 0009](../decisions/0009-staging-preserves-results.md) permits it because integer equality
inside the portable window is bit-identical on every target — which is exactly what is *not* true
of float arithmetic, and why nothing here folds a float. It is the first entry in the
constant-folding table [tables.md §8](tables.md) predicted, where `((array 1 2 3) 1) → 2` and
`(go.+ 1 2) → 3` are the same kind of step rather than new rules.

The **commuting conversion** is Prawitz's, and GHC's case-of-case:

```
((if c A B) k…)          ⟶  (if c (A k…) (B k…))
((let v (fn (x) B)) k…)  ⟶  (let v (fn (x) (B k…)))
```

It is what makes a *dynamic* sum cost nothing. A sum whose tag is known reduces away by β alone;
one whose tag depends on runtime data gets stuck as `((if c A B) F G)`, with the constructor under
the branch and the eliminator outside it, so neither can see the other. Pushing the eliminator in
reunites them and β finishes the job. The `let` clause is not a second rule so much as the same
one: β itself puts a `let` between a constructor and its eliminator whenever a shared subterm is
not duplicable, so the honest statement is **push an eliminator through anything β can leave in
operator position**, which in this language is exactly `if` and `let`.

It applies only when every argument is **pure**, because it duplicates them into both branches and
[ADR 0010](../decisions/0010-effects-as-structural-rules.md) denies contraction for an impure term.
Left stuck, an impure eliminator is reported by the emitter rather than silently mis-ordered. The
known hazard is code growth — `k` appears twice, so nested cases multiply; GHC's answer is join
points and `again` is one.

**Measured before it shipped**, as the build order demanded: across **184 residuals** — every
example on all four targets — the commuting conversion changes **nothing**, because it fires only
where a sum is eliminated.

Conditional compilation still falls out of a definition — `(def debug? false)` erases the branch.
Neither literal rule contradicts "no primitive is ever evaluated" below: `if` and `=` are the
language's, not a target's, and the literals are not primitive applications. Dropping the untaken
branch is sound even when it is impure, for a different reason than β's — β may not drop an impure
argument because the argument would have run, and here the branch does not.

**No recursion.** A definition defined in terms of itself is an error, checked per-target before
reduction ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). δ still declines to
unfold a cycle, so the reducer stays correct on a term the front-end rejects. Iteration is
`fold-range`. Removing recursion did not make the language terminating — self-application still
diverges and is still guarded by fuel ([pcf.md §9](pcf.md)).

**Two parameters.** Which names are primitive, and which of those are pure — both supplied by a
target file. The first is [ADR 0002](../decisions/0002-capability-graph.md)'s capability set. The
second decides whether β may move a term, and defaults to *impure*, so that a target author's
omission costs speed rather than correctness.

That is all of it. 3,124 lines in `core/`, 89 tests there and 73 in `emit/`.

Arithmetic and comparison live in target files — **not** in the language. **Equality no longer
does**: `=` moved in on 2026-08-22 and is now injected like `if`, `let` and `loop`, resolving to
each host's own (`==`, `===`, `sete`). So booleans
([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)) are no longer the only thing that
has moved in from a target — they were the precedent, and `=` is the second case. An `int` is a
mathematical integer whose portable range is ±(2⁵³−1), which is JavaScript's limit and the only
range on which all four targets agree exactly ([integers.md](integers.md)).

## 2. What a program may *not* say

Removed 2026-08-14 after the addition of target files made it dead:

- `(prim …)` and `(target …)` in a program are now **errors**. They were silently accepted and
  ignored, which would let a program believe it had declared something. Primitives come from
  `targets/NAME.oro` — or `targets/NAME/*.oro`, since a target is a directory now — and nowhere
  else. A program may not declare a **language** construct either: `if`, `let`, `loop` and `=` are
  injected into every target and declaring one is an error, in a target file as much as in a
  program.

## 3. What is absent, and deliberately

| | Status |
|---|---|
| Types in the *language* | none — no annotations. `(sig …)` is a **claim about a definition**, checked against the residual and against any target providing the name natively ([types.md §7](types.md)); it is not a type on a term |
| Type **checking** | **yes**, on the residual before emission ([types.md](types.md)). One checker, **four** targets, including the one with no type layer |
| Data structures | **the algebra's two, and nothing else.** A **product** — `(values a b)`, the negative one, consumed by a continuation ([values.md](values.md)) — and a **sum**, closed, finite and non-recursive ([sums.md](sums.md)). Both are *sugar*: they add no term kind and no target declares either. `string`, `vec-f64` and `dict` remain opaque handles only primitives touch; `bool` is not one of these — it is a literal of the language |
| Boolean connectives | **sugar**, erased by the reader; each backend puts the host's own operators back ([booleans.md](booleans.md)) |
| Arithmetic evaluation | `(go.+ 1 2)` does not fold — **no primitive is ever evaluated**, still true and checkable. The language's own `=` DOES fold on two integer literals (§1), and the distinction is the point: `=` is the language's, so folding it decides nothing about a target |
| Pattern matching | **built**, and it cost the reducer nothing. `match` is reader sugar over `loop` ([match.md](match.md)); `case` eliminates a sum and expands in `Load` ([sums.md](sums.md)). Zero term kinds and zero reduction rules for either. This is **not** Coq's ι, which analyses an *inductive* type — ours are non-recursive, so there is no fixed point to eliminate |
| Extensionality | none — η |
| Effect *types* | none. Purity is one declared bit per primitive; g5's ordering discipline is a side condition on β ([effects.md](effects.md)) |
| Modules | **scopes, resolution, imports, exports, and files** ([modules.md](modules.md)); emitted functions are named after their export. A target **is** a directory now — `targets/go/`, `js/`, `java/`, `windows/` ([target-native.md](target-native.md)) |
| Unbounded iteration | **built** — `loop`/`again` ([ADR 0015](../decisions/0015-loop-and-again.md)). The language is no longer primitive recursive, and termination is a computed program property |
| Recursion | **rejected**, per-target, before reduction ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)). The `def`/`rec` split is withdrawn with it — nothing is left to opt into ([def.md §3](def.md)) |
| Tail-call optimisation | not guaranteed, and moot: no target provides it and there is no recursion to optimise ([def.md §10](def.md)) |
| Escaping closures | all **four** backends refuse them. A closure may not survive staging; the three tiers of what that does and does not cost are in [callbacks.md](callbacks.md) |
| Symbols | **refused**, and that is a decision rather than a gap ([def.md §5](def.md)) |

## 4. Strings, which were added out of order

A term kind and a literal syntax were added to the reader so that **target files** could carry
emission templates. That was a compiler need, not a language need, and when this was written the
language had gained a literal that *no program used*.

**That is no longer true**, and nothing was decided to make it so: `hello`, `report`, `build-vec`
and the sieves all print, so a string literal is now an ordinary thing a program writes. The
retroactive specification is what made that safe rather than accidental.

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
- **The normal form is a parameter.** One source, four targets, and the dictionary comes out
  *fused on Go and unfused on Java* because that is what each measured faster.
- **Call-by-need with no cost model.** Occurrence counting plus a four-case syntactic test, which
  the [measurements](../../gauntlet/results/callbyneed-2026-08-14.md) showed is enough.
- **Sequencing with no sequencing construct.** `seq` is a β-redex whose binder is unused, and it
  works because β refuses to drop an impure argument — the ordering discipline and the ability to
  write a statement sequence are the same mechanism.
- **A sum that costs nothing at either level.** A tag known at compile time reduces away by β; a
  tag decided at runtime reduces to the `if` that decided it, by the commuting conversion. Neither
  leaves a tag, a closure, an allocation, or a dispatch the `if` was not already doing
  ([sums.md](sums.md)).
- **Exhaustiveness that REMOVES a branch.** A sum is closed and finite, so once the other clauses
  are excluded the last one needs no test — a better argument for checking exhaustiveness than the
  one that motivated it.

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
