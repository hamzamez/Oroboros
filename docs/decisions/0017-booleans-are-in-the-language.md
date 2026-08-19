# 0017 — Booleans and control flow are in the language

Date: 2026-08-19
Status: Accepted

Revisits two rejected alternatives in [ADR 0012](0012-portable-integer-range.md). That ADR's
decision — an `int` is exact within ±(2⁵³−1) — is untouched and still stands.

## Context

`and`, `or`, `not`, `true` and `false` were declared by each target. `if` was declared by each
target as a structural primitive. [arithmetic.md §3](../spec/arithmetic.md) recorded why, and ADR
0012 rejected the alternatives, both on one premise:

> All three hosts have short-circuiting `&&` natively; lowering to a conditional emits an
> if-statement and a temporary where the host has an operator.

**The fourth host has no `&&` at all.** It has `and` — one strict ALU instruction — and it has
branches. The premise is false, and four things followed from it being false.

**One name meant two functions.** `targets/windows/x64.oro` declared `x64.andb` with
`(jump "and")`, so it short-circuited in a guard and evaluated both operands as a value. The
difference is observable: `(x64.andb (x64.setne d 0) (x64.setl (x64.idiv n d) 20))` with `d = 0`
raises #DE and kills the process.

**`&&` denoted three different functions across four targets** — `bool→bool` short-circuiting on
Go and Java, `any→any` returning an *operand* on JavaScript, strict on windows.

**The reader already hardcoded `if`.** `core/read.go` emits the name when desugaring a `loop`'s
clause chain, so a target that spelled its conditional anything else would have compiled
straight-line code and failed on every loop. `if` was in the language and the specification said
otherwise.

**And there was no boolean literal at all.** All four targets declared `true` and `false` under
exactly those names — the only agreement of that kind outside the structural set — and a program
still had to write `go.true` or `x64.true`.

Six candidates were surveyed against the literature in [booleans.md](../spec/booleans.md).

## Decision

**`bool` is data in the language, `if` is its eliminator, and `and`/`or`/`not`/`cond` are reader
sugar that erases.**

This is Scheme's answer, ML's answer, and — through call-by-push-value's polarity — the same answer
the type theory gives: `bool = 1 ⊕ 1` is positive, and a positive type is eliminated by case
analysis, which is a branch. McCarthy stated the operative half in 1960: **a conditional cannot be
a function**, because a function in a strict language receives every argument evaluated.

**One new term kind**, `KBool`, and two literals.

**Four new reader forms, all definitional, none surviving the reader:**

```
(and)          ⟶ true          (or)           ⟶ false
(and e)        ⟶ e             (or e)         ⟶ e
(and a b …)    ⟶ (if a (and b …) false)
(or  a b …)    ⟶ (if a true (or b …))
(not a)        ⟶ (if a false true)
(cond c₁ e₁ … else e) ⟶ (if c₁ e₁ (cond … else e))
```

`cond` is `loop`'s clause chain with `again` removed — first match wins, `else` mandatory. The two
share `clauseChain` in the reader.

**Two new reduction rules**, `(if true a b) → a` and `(if false a b) → b`.

**`if` is injected into every target** and a target that declares it is an error, as is a target
declaring `and`, `or`, `not` or `cond` unqualified. `(structural if cond pure)` is gone from all
eleven target files; `cond` remains a structural *kind* that each backend implements.

**Each backend puts the host's operator back.** `(if p q false)` with a literal `false` is a
one-node match, and Go, JavaScript and Java emit `&&`, `||` and `!`. The windows backend emits
jumping code: both failures branch to the same label, which is the dragon book's translation and
the commuting conversion where the duplicated continuation is one label rather than two copies.

**The strict, branchless operators stay** as `x64.andb`, `x64.orb`, `x64.notb` — host-named, no
portability claim. Ada kept `and` beside `and then` and C kept `&` beside `&&`; a program that has
measured its predicate as unpredictable can still reach for the instruction.

## Why not

**Why not leave it alone and just delete `(jump "and")`?** That fixes the one defect and leaves
four: `if` hardcoded in the reader but declared in eleven files, no boolean literal, JavaScript's
`any`, and no static folding. It is the do-nothing option and it was named as such.

**Why not Church-encode?** Already refuted in this repository, before this ADR
([arithmetic.md §2](../spec/arithmetic.md)): an encoding is free exactly when the eliminator's
scrutinee is statically known, and a boolean's constructor is a runtime value — which is what makes
it a conditional. Plus both branches would be evaluated, and fixing that needs thunks, which are
closures, which allocate.

**Why not keep `and` a primitive, since lowering it to a conditional lowers further than the target
requires?** This is ADR 0012's objection and it deserved a measurement rather than an answer. It
got one: [and-form-2026-08-19](../../gauntlet/results/and-form-2026-08-19.md). On Go the nested form
is never slower and is 1.11× ahead on a three-term predicate; on V8 the two are indistinguishable;
and `-d=ssa/check_bce/debug=1` shows every bounds check eliminated in both. **The objection does not
reproduce.** It is also answered structurally: the rule is about the emitted artifact, and the
emitted artifact is `p && q`.

**Why not ship both operators in the language, as Ada does?** Because the strict one is a *target*
concern and this project already has the mechanism for target concerns. `x64.andb` sits exactly
where `go.+` sits — host-named, unportable, available. Putting it in the language doubles the
vocabulary to say something a target file already says.

**Why not require every target to declare both translations — the value form and the branch form?**
That is what `(jump "and")` was, and it makes short-circuiting **a claim a target author makes**
that the format cannot check and that is observable only through a trapping or effectful operand.
It is `split-words` again: a name that passed every check for two months while returning different
answers on different targets.

**Why not do sum types now, with `bool = 1 ⊕ 1`?** That is the right destination and the wrong
order — it is blocked on the product/sum allocation measurement, which is exactly where the
predecessor project died. The design here is the n = 2 case of a sum eliminator, so it is not work
that has to be undone.

**Why a seventh term kind, when ADR 0012 said one was not needed?** Because it is now *forced*
rather than chosen. The reader's desugaring of `and` has to produce a false value, and the reader
does not know which target it is reading for — it cannot emit `go.false` or `x64.false`. A
reader-level sugar requires a reader-level literal. Nothing needed a boolean constant until a
desugaring did.

## Consequences

**Conditional compilation, with no preprocessor.** A condition known at compile time now compiles
to nothing:

```lisp
(def debug? false)
(def f (fn (x) (if debug? (seq (log x) x) x)))     ⟶  (fn (x) x)
```

This is the only evaluation reduction performs. "No primitive is ever evaluated" still holds — `if`
is not a primitive and the literals are not primitive applications. Dropping the untaken branch is
sound even when it is impure, and for a different reason than β's: β may not drop an impure
argument because the argument would have run, and here the branch does not run.

**Targets got smaller.** Each declared five boolean names and a structural conditional; they now
declare none.

**Two latent bugs surfaced.** Neither backend's `typeOf` had a case for `cond`, so an emitted
function whose body was a conditional returned `/*unknown*/` on Go and Java. No program had one
until `and` became a conditional.

**The refinement fragment had to learn the desugaring.** `emit/linear.go` recognised `and`; a
refinement written with the language's `and` now arrives as `(if a b false)` and is recognised as
the conjunction it is. Only conjunction — `or` and `not` desugar to a disjunction and a negation,
and the fragment is conjunctions of linear inequalities, which is the same wall `d ≠ 0` hits.

**Conformance is a program that runs**, not a claim:
`examples/native/shortcircuit-go.oro` and `shortcircuit-win.oro` put an `idiv` by a zero divisor
behind the guard that must short-circuit. Both print `222` then `111`, on the two hosts where the
failure modes are a panic and a #DE respectively.

| | before | after |
|---|---|---|
| term kinds | 6 | 7 |
| reduction rules | β, δ | β, δ, `if` on a literal |
| structural primitives a target declares | `let`, `if`, `loop` | `let`, `loop` |
| boolean names a target declares | 5 | 0 |
