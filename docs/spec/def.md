# The semantics of `def`

Go has `var`, `func`, `const`, `type`. Scheme's `define` does several jobs at once. This settles
what ours does, what the literature already calls it, and what we should refuse to inherit.

---

## 1. `def` is not a term

First distinction, and everything follows from it.

| | Form | Lives in |
|---|---|---|
| **Binding** | `let x = e in b` | a **term** — scoped, nested, has a value |
| **Definition** | `f := t` | the **context** — global, extends the environment, is not part of any term |

Ours is the second. In type theory this is written as a context extension:

```
Γ ::=  ·  |  Γ, x : A        (an assumption)
       |  Γ, f := t : A      (a definition)
```

`def` is a **judgment about the environment**, not a term former. That is why it cannot appear
inside a term in [core-0](core-0.md)'s grammar, and it is why nested `def`
([pcf.md §4](pcf.md)) has to be *sugar* — nesting would make it a term former, and a term former
that binds recursively is `letrec`.

## 2. What `def` means: definitional equality

The literature is type theory, and the word is precise.

- **Propositional equality** `a = b` is a *type* you construct a proof of.
- **Definitional equality** `a ≡ b` (also *judgmental*, also *convertibility*) holds **by
  computation**. No proof object; the checker just reduces.

A definition extends the context with a definitional equality:

```
────────────────────  (δ)
Γ, f := t  ⊢  f ≡ t
```

**Unfolding it is δ-reduction**, and Coq's conversion is literally named **βδιζη** — β for
application, **δ for unfolding definitions**, ι for pattern matching, ζ for `let`, η for
extensionality. Our reducer implements β and δ; the other three are exactly the features we have
not added.

So `def` has a name, a rule, and forty years of metatheory. It is not a new construct.

## 3. Abbreviation and recursion are two different things

`def` is currently doing two jobs, and they have different mathematics.

```lisp
(def pi 3.14159)                                    ; abbreviation
(def count (fn (n) (if (lt n 1) 0 (count ...))))    ; fixpoint
```

| | Meaning | δ-unfoldable? | Grade |
|---|---|---|---|
| `(def pi 3.14159)` | `pi ≡ 3.14159` | **always** | 0 — vanishes |
| `(def count …)` | `count = fix(λcount. …)` | **never** — `fix f` has no normal form | 1/ω — survives |

The implementation already separates them (`markRecursive`), but **the syntax does not**, and
that is a problem for three reasons.

**It hides the grade.** [g6 §9](../derivations/g6-escaping-closures.md) promised the compiler
would tell you whether an abstraction is eliminated or survives. Here the answer is determined by
whether a definition is recursive — and a reader cannot tell without computing reachability over
the whole program by hand. Requirement 8 says a model should be able to reason locally; it
cannot.

**It makes an infinite loop silent.** `(def f (f))` is well-defined — it denotes ⊥ — but so is
`(def size (fn (x) (size x)))` written when a *different* `size` was meant. The second is a typo
that compiles to a hang, with no diagnostic.

**It contradicts the semantics.** One is δ. The other is `fix`. Giving two rules one syntax
means the syntax says less than the calculus does.

### What the literature does

| System | Non-recursive | Recursive |
|---|---|---|
| **Coq** | `Definition` | `Fixpoint` (+ a decreasing-argument check) |
| **ML / OCaml** | `let` | `let rec … and …` |
| **Haskell** | — | everything is `letrec` |
| **Scheme** | `define` does both | — |
| **Rust, Go** | `fn` does both | — |

The split is real and the systems closest to ours — Coq, which is the one that actually
distinguishes δ-unfoldable from not — make it **syntactic**.

### Decision — **and it is blocked on something bigger**

> **Status, 2026-08-15.** The split below is still the right answer and is still **unimplemented**.
> Implementing it now would be decorating, because of §9: **recursion cannot be emitted at all.**
> A `rec` form that distinguishes "unfolded" from "survives as a target function" is only
> meaningful once a target function is something a backend produces. Sequence: emission first,
> then the syntax that talks about it.

**Two forms.**

```lisp
(def  f t)                    ; f ≡ t.  δ-unfoldable.  Must not be recursive.
(rec (f t) (g u) …)           ; a mutually recursive group.  Never unfolded.
```

`rec` takes a *group* because mutual recursion cannot be marked locally — `even?` and `odd?`
mention each other and neither mentions itself. Mathematically the group is one fixpoint over a
tuple, which is why ML writes `let rec … and …` and why the group is the honest unit.

Costs, stated plainly: one more form, and a program that grows a cycle must be regrouped. The
gain is that **the unfold/no-unfold decision — hence the grade, hence what is emitted — is
readable at the definition site.**

Recursion in a `def` becomes an error naming the cycle:

```
`size` is recursive: size → size
A definition marked `def` is unfolded at compile time, and unfolding a
recursive definition does not terminate. Write it as (rec (size …)) if
the recursion is intended.
```

## 4. `(def name "hamza")` then `(name)`

Trace it exactly.

```
(name)              ; an application, zero arguments, operator = name
⟶δ  ("hamza")       ; δ unfolds the definition
```

β cannot fire — the operator is a string, not a λ. So `("hamza")` is **stuck**: a normal form
containing no redex and meaning nothing.

Three things this settles.

**`def` binds a term, not a function.** `(def name "hamza")` makes `name ≡ "hamza"`, so `name` is
the string and `(name)` is applying it. Functions come only from `fn`:
`(def greet (fn () "hamza"))`. There is no implicit "definitions are functions" rule, which is
where Scheme's `define` earns part of its complexity.

**Scheme's `(define (f x) body)` shorthand is sugar** for `(def f (fn (x) body))` and should be
recognised as such if we adopt it — not as a second meaning for `def`.

**Stuck terms need a well-formedness check.** ~~and we do not have one~~ — the emitters now
refuse: *"application of a non-name: the operator must be a primitive or a recursive definition."*
So the error is ours rather than the host's, which was the point. A type checker
([types.md](types.md)) sits in front of it and catches most of this earlier.

## 5. Literals: which ones, and one refusal

**Numbers — yes.** Integers and floats, distinct, already implemented. Ranges come with
[ADR 0003](../decisions/0003-range-typed-integers.md).

**Strings — yes, but they are not simple.** Every target has a native string, so the Parasite
rule says use it; but [g5](../derivations/g5-bindings.md) measured that the three targets
disagree on float→string formatting, which means a *portable* string library is not free. Strings
as a primitive type: fine. String *operations* as Tier 1 capabilities: a real design cost, and
[open decision 4](../design-direction.md).

**Symbols — no.** This is the one to refuse, and it is exactly the "do not pay the price of a
Lisp" line.

Lisp has symbols because it has a **runtime reader and `eval`**: a symbol is a name that survives
into execution, interned, comparable, and reflectable. We have neither. Everything reduces at
compile time; names are already in the term language and none of them survives.

What symbols are usually used for, and what we use instead:

| Use | Ours |
|---|---|
| Enum tags | range-typed integers ([ADR 0003](../decisions/0003-range-typed-integers.md)) |
| Map keys | strings |
| Reflection | we have no runtime to reflect on |

Adding symbols would mean adding an interning table and an identity notion to every target, for a
feature whose motivation does not apply. **Refused.**

## 6. Do we need `let`?

Asked directly, and the answer is **not as a term former** — but the reason is more interesting
than the answer.

`let x = e in b` is `((fn (x) b) e)`, so it is derivable and adds nothing. The pressure for it
comes from elsewhere: **call-by-need** needs a way to *stop* β from firing where firing would
duplicate work. If the result is written as an unreduced application, then "normal form = no
β-redex" is false and the specification has to be weakened.

There is a way out that costs no new syntax and no weakening.

> **`let` is a primitive, of arity two, taking a value and a continuation:**
> `(let e (fn (x) b))`

The binding structure is the λ's — there is no new binder. `let` is a *name in Σ*, so β does not
apply to it and the term is in normal form by the existing definition. Every target already
provides it: Go's `:=`, JS's `const`, Java's assignment. It belongs in Σ for all of them, and on
a target that somehow lacked it, it would be δ-definable back into an application.

And the reducer's job becomes precisely the one partial evaluation already names: when
substitution would duplicate runtime work, **residualize** rather than reduce — emit
`(let e (fn (x) b))` instead of `b[x := e]`. That decision *is* the binding-time decision, so
this is not a special case bolted on; it is the same mechanism seen from the reduction side.

Note this is also ζ, the fourth letter of Coq's βδιζη. We now have β, δ, and a plan for ζ, and
none of them is ours.

### Implemented, with one correction — 2026-08-14

`let` has **two roles and one spelling**, and conflating them was a real bug:

| | |
|---|---|
| in **source** | sugar for an application — it reduces like anything else |
| in a **residual** | the primitive β produced when it declined to substitute |

The first implementation made `let` primitive everywhere, which silently gave the *knob* design
we had rejected: a programmer writing `(let 5 (fn (x) (add x 1)))` got it back unreduced.

Worse than useless — **dangerous**. A `let` written for readability around a value that later
reduces to a λ would prevent that λ being substituted, and fusion would die. The programmer would
have made their program allocate per element by adding a binding for clarity.

The fix is one rule in the reader: `(let e k)` desugars to `(k e)`. The programmer's `let` states
intent and is erased; the compiler re-introduces sharing wherever β declines
([callbyneed](../../gauntlet/results/callbyneed-2026-08-14.md)). A `let` in a residual can only
have come from the reducer, so the two roles never collide.

Three tests pin it: a source `let` is erased when sharing is pointless, a source `let` **cannot**
block fusion, and the compiler still introduces one where sharing pays.

## 9. Recursion reduces correctly and cannot be compiled

Found 2026-08-15, by trying it:

```lisp
(def countdown (fn (n) (if (int.le n 0) 0 (countdown (int.sub n 1)))))
```

```
gen: gen-countdown mentions the recursive definition(s) countdown, and no
     backend emits recursion yet — iteration is fold-range
```

Everything upstream is right. `markRecursive` finds the cycle, δ correctly declines to unfold it,
and the residual keeps `countdown` as a free name. Then **nothing downstream knows what to do with
it**: `cmd/gen` emits one function per export and has no notion of also emitting the recursive
definitions a residual reaches.

Two documents disagreed about this and neither was checked. [core-0 §6](core-0.md) says a recursive
definition "stays in the residual as a target function", so `Residual` deliberately does *not*
report it — while the emitter reported `no Go form for primitive "countdown"`, a message about a
primitive nobody ever declared. The commands now say the true thing instead.

**This is why no gauntlet program is recursive.** Iteration here is `fold-range`, which is a
primitive and compiles to a `for`. Recursion is the fallback for shapes a fold cannot express, and
that fallback does not exist yet.

It also silently swallows a typo. `(def size (fn (x) (size x)))` — a self-reference written when a
*different* `size` was meant — reduces to itself with no complaint. §3's `rec` split is what turns
that into an error, which is a second reason the split is worth having *after* emission works.

## 10. Tail calls

**Decided: we do not guarantee tail-call optimisation, and the question is not currently live.**

Not a preference — a fact about the targets:

| | proper tail calls |
|---|---|
| Go | **no**, and not planned |
| JVM | **no**; the JEP has never landed. Kotlin's `tailrec` and Scala's `@tailrec` rewrite to a loop at compile time rather than relying on the VM |
| JavaScript | specified in ES6, implemented **only** by JavaScriptCore. V8 and SpiderMonkey do not |

So guaranteeing TCO means *implementing* it ourselves, on every target, by rewriting tail recursion
into a loop. That is what Kotlin and Scala do, and it is the only portable way.

**And it would buy nothing today**, because §9: recursion is not emitted at all. You cannot
optimise tail calls in a construct that does not compile.

Worth correcting one belief while recording this: **Rust does not guarantee tail calls.** `become`
is a reserved keyword and the feature is unstable; LLVM performs the optimisation opportunistically
and nothing in the language promises it. Nobody in this neighbourhood guarantees TCO except
languages that *must* — the ones where recursion is the only loop.

### When it becomes live, the shape is known

1. **Opt-in, not implicit.** A marker like Kotlin's `tailrec`, checked at compile time: if the call
   is not in tail position, that is an **error**, not a silent fallback to a stack call. Silent is
   what makes TCO a hazard — a program that works until a refactor moves the call.
2. **Rewritten by us, to a loop.** Target-independent, because we emit the loop.
3. **Blocked on a general loop primitive.** `fold-range` is *counted*; tail recursion needs
   `while`-shaped iteration, and that is a new structural primitive
   ([target-files.md §4](target-files.md)) — the real prerequisite, and a bigger decision than TCO
   itself.

The reason it can wait is the one stated at the start of this project: **we are not paying the
price of a Lisp.** Iteration is a fold, not a recursion, so nothing depends on TCO. It is a feature
to have, not a foundation to build on. Mojo's version is good because their *memory model* makes it
good, which is a fact about ownership rather than about tail calls — and ownership is
[ADR 0013](../decisions/0013-accept-the-allocation-price.md)'s open question, not this one.

## 7. Decisions

1. **`def` is a context extension, not a term** — a definitional equality, unfolded by δ.
2. **Split `def` from `rec`.** `def` is δ-unfoldable and must not be recursive; `rec` takes a
   mutually recursive group and is never unfolded. This makes the grade readable at the
   definition site.
3. **`def` binds a term, not a function.** `(def name "hamza")` makes `name` the string;
   `(name)` applies it and is an error. `(def (f x) …)` shorthand, if adopted, is sugar.
4. **Add a well-formedness check on the residual:** an application's operator must be a λ, a
   primitive, or a recursive definition.
5. **Numbers yes, strings yes-with-caveats, symbols no.** Symbols exist to serve a runtime
   reader and `eval`; we have neither, and adding them means interning tables on every target for
   a motivation that does not apply.
6. **Recursion is not emitted, and tail calls are not guaranteed** — §9, §10. Iteration is
   `fold-range`. Neither `rec` nor TCO is worth building before recursion compiles at all.
7. **`let` is a primitive taking a continuation, not a term former.** No new syntax, no weakening
   of the normal form, and the reducer's choice to emit it is the binding-time decision.

## 8. What is deliberately still missing

Coq's conversion is βδιζη. We have β and δ, and §6 plans ζ.

- **ι** — pattern matching / case analysis. Nothing in the core matches on data yet; `if` is a
  primitive, so branching is currently opaque to reduction. This is where
  [q5b §6](q5b-filter.md)'s case-of-case problem will arrive.
- **η** — extensionality, `f ≡ (fn (x) (f x))`. Not needed yet; it will matter if uncurrying or
  arity adjustment is ever done as a reduction rather than a representation choice.

Naming the two we lack is worth as much as specifying the three we have, because it says where
the next pressure comes from.
