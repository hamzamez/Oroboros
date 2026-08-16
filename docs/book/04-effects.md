# 4. Effects

Chapters 1–3 built the language: [`fn`](01-fn.md), [`def`](02-def.md), [modules](03-modules.md).
This chapter is not a fourth form. **There is no effect form.** No `IO`, no `!`, no `do`, no
`unsafePerformIO`, nothing to import.

What there is instead is **one declared bit per primitive**, and a side condition on β that reads
it. That is the entire mechanism, and this chapter is about why one bit is enough — and about the
three-way choice, older than computing, that it is quietly making.

You have already seen it twice without being told. Chapter 1 §1.10 ended with two lines and the
claim that the difference between them "is the whole of the effect discipline". Chapter 2 §2.3
showed an error about a body being "a computation, not a value". Both were promissory notes. This
is the payment.

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

The teaching target now declares three impure primitives — `shout`, `whisper`, `tick` — alongside
the pure `+ - * / <` and the opaque `f g h x y z`.

---

## 4.1 The bit

Here is the whole of it, from [targets/tutorial.oro](../../targets/tutorial.oro):

```lisp
(prim +       (num num) num  expr "%s + %s" pure)
(prim shout   (num) num       stmt "shout(%s)")
(prim whisper (num) num       stmt "whisper(%s)")
(prim tick    (none) num      stmt "tick()")
```

One word. `+` has `pure`; `shout` does not. Nothing else in the language knows about effects — not
the reader, not the term type, not the type checker. A **target author** writes that word, and
everything in this chapter follows from it.

Notice what is *not* being said. `shout` is not of a different type from `+`. There is no `IO num`.
The bit is attached to the **name**, in the target file, and a term never carries it.

## 4.2 What the bit buys, in four lines

Two programs, differing only in which primitive they call.

```lisp
((fn (n) (+ 1 2)) (f 9))
```

```lisp
⟶   (+ 1 2)
```

```lisp
((fn (n) (+ 1 2)) (shout 9))
```

```lisp
⟶   (let (shout 9) (fn (n) (+ 1 2)))
```

`n` is used **zero** times in both. In the first, `(f 9)` vanished — it was never going to be
needed and it does nothing, so the compiler dropped it. In the second, `(shout 9)` was kept,
bound, and put back exactly where it was written.

That is the whole discipline in one comparison. The rest of this chapter is what "exactly where it
was written" turns out to mean, and why it needs three separate promises rather than one.

## 4.3 The three things that can go wrong

Take a redex `((fn (x) body) e)`. Reduction wants to replace `x` with `e`. If `e` has an effect,
three separate disasters are available:

| If `x` occurs… | naive β would… | and that is… |
|---|---|---|
| twice | run the effect twice | **duplication** |
| zero times | run it never | **deletion** |
| after another argument | run it in the wrong order | **reordering** |

These are not three bugs. They are three **structural rules**, and they have had names since
Gentzen wrote them down in 1934:

> **contraction** — two uses of a thing may be served by one copy.
> **weakening** — a thing you never use may be discarded.
> **exchange** — two things may be swapped.

Every ordinary programming language assumes all three, silently, everywhere. `x + x` uses `x`
twice (contraction). An unused variable is dropped (weakening). `f(a) + g(b)` may evaluate either
side first (exchange, in C at least). They feel like nothing because they are usually true.

**They are exactly what an effect breaks.** So the rule is:

> **Purity is the licence to use the structural rules.** A pure term may be copied, dropped and
> moved. An impure term is *ordered*: exactly once, where it was written.

The rest of §4 is that sentence, one clause at a time, with the output.

## 4.4 Contraction — one effect must not become two

```lisp
((fn (n) (+ n n)) (shout 9))
```

```lisp
⟶   (let (shout 9) (fn (n) (+ n n)))
```

`n` appears twice; `(shout 9)` appears once. Bound, then used twice — one shout, two uses of its
result.

Now look at a *pure* argument with the same shape:

```lisp
((fn (n) (+ n n)) (f 9))
```

```lisp
⟶   (let (f 9) (fn (n) (+ n n)))
```

**The same output.** Two identical residuals, two completely different reasons:

- `(shout 9)` is bound because copying it would be **wrong**.
- `(f 9)` is bound because copying it would be **slow** — chapter 1 §1.10's call-by-need, which
  exists because [duplicating an allocating expression measured 615× on Go](../../gauntlet/results/wordcount-2026-08-14.md).

The difference shows when copying is free. Literals and names are copied without hesitation:

```lisp
((fn (n) (+ n n)) 5)        ⟶   (+ 5 5)
((fn (n) (+ n n)) x)        ⟶   (+ x x)
```

`x` is a primitive — a value from outside — and duplicating it is fine. `(shout 9)` will *never*
be treated that way no matter how cheap it looks, and `(f 9)` might be, if the cost model says so.

> **Purity and cheapness are two axes, not one.** Cheap-and-pure may be copied. Costly-and-pure
> may not, on performance grounds. Impure may not, on correctness grounds. Conflating them would
> cost real speed — a duplicated array read compiles to
> [byte-identical machine code](../../gauntlet/results/duplicate-read-2026-08-14.md), and refusing
> to duplicate it would be leaving that on the table.

## 4.5 Weakening — an effect must not vanish

```lisp
((fn (n) 7) (tick))
```

```lisp
⟶   (let (tick) (fn (n) 7))
```

`n` is used **zero** times. The function ignores its argument entirely, and the argument is kept
anyway. Against the pure version, which does not survive at all:

```lisp
((fn (n) (+ 1 2)) (f 9))    ⟶   (+ 1 2)
```

This clause is the one that makes dead-code elimination *conditional*. A compiler that drops
unused arguments is doing weakening; here it may only do so when the bit says it may.

And it is what makes sequencing possible at all — §4.9.

## 4.6 Exchange — effects must not be reordered

This is the one with the most visible output.

```lisp
((fn (a b) (+ b a)) (f 1) (g 2))
```

```lisp
⟶   (+ (g 2) (f 1))
```

Read that carefully. The source computes `(f 1)` first and `(g 2)` second; the residual has
`(g 2)` on the **left**. The two pure arguments were substituted into the body and now appear in
whatever order the body wanted. They moved. Nothing cares, because they are pure.

Same program, impure arguments:

```lisp
((fn (a b) (+ b a)) (shout 1) (whisper 2))
```

```lisp
⟶   (let (shout 1) (fn (a) (let (whisper 2) (fn (b) (+ b a)))))
```

Nested lets, in **argument order**: `shout` then `whisper`, regardless of the body using `b`
before `a`. The order the program was written in survives into the output.

Mix them and each gets its own treatment:

```lisp
((fn (a b c) (+ a (+ b c))) (shout 1) (f 2) (whisper 3))
```

```lisp
⟶   (let (shout 1) (fn (a) (let (whisper 3) (fn (c) (+ a (+ (f 2) c))))))
```

`shout` and `whisper` are pinned, in order. `(f 2)` — written between them — has been moved
*inside both* and dropped into the body where it was used. Three arguments, two different
disciplines, decided per argument by one bit.

### Deletion and ordering together

```lisp
((fn (a b) b) (shout 1) (whisper 2))
```

```lisp
⟶   (let (shout 1) (fn (a) (let (whisper 2) (fn (b) b))))
```

`a` is never used, and `(shout 1)` is kept **and kept first**. Weakening denied and exchange
denied at once, which is what "exactly once, where it was written" means when you say it precisely.

## 4.7 Where an effect is put back, and why it is the application site

The rule is:

> **An impure argument is never substituted.** It is normalised and let-bound **at the application
> site**, in argument order, exactly once, whatever its occurrence count.

"At the application site" is doing real work. In `((fn (x) body) e)`, the term `e` is written
*outside* the λ. Uses of `x` may be anywhere inside `body` — under a loop, inside one arm of a
conditional. Put the effect where `x` is *used* and you have moved it.

Watch it not move:

```lisp
((fn (n) (if (< 1 2) n 0)) (shout 5))
```

```lisp
⟶   (let (shout 5) (fn (n) (if (< 1 2) n 0)))
```

`n` is used in one arm only. Binding at the *use* site would put `shout` inside the `if`, and it
would stop happening whenever the condition was false. It is bound outside, where it was written,
and it happens unconditionally — which is what the source says.

The same for loops:

```lisp
((fn (n) (fold-range 0 10 (fn (acc i) (+ acc n)))) (shout 5))
```

```lisp
⟶   (let (shout 5) (fn (n) (fold-range 0 10 (fn (acc i) (+ acc n)))))
```

One shout, outside the loop. Substituting `n` would have given ten.

### And the reverse: an effect written *inside* stays inside

```lisp
(if (< 1 2) (shout 1) (whisper 2))
```

```lisp
⟶   (if (< 1 2) (shout 1) (whisper 2))
```

Nothing was hoisted. This is not a special case for conditionals — `if` is a **primitive
application**, not a β-redex, so there is no substitution to guard and its arguments are normalised
in place. The arms keep their guards for free.

```lisp
(fold-range 0 3 (fn (acc i) (+ acc (shout i))))
```

```lisp
⟶   (fold-range 0 3 (fn (acc i) (+ acc (shout i))))
```

Three shouts, because the source asked for three.

## 4.8 A λ is a value, even when its body is not

This is the rule that keeps everything else from collapsing.

```lisp
((fn (k) (+ (k 1) (k 2))) (fn (n) (shout n)))
```

```lisp
⟶   (+ (shout 1) (shout 2))
```

The λ was **copied** — contraction, on a term whose body shouts. And that is correct: writing a
function does not run it. Two applications, two effects. The alternative would let-bind the λ,
which reaches the emitter as an escaping closure and refuses to compile.

Unused, it disappears entirely:

```lisp
((fn (k) (+ 1 2)) (fn (n) (shout n)))
```

```lisp
⟶   (+ 1 2)
```

Weakening on a λ with an impure body — allowed, because nothing applied it, so no effect existed
to delete.

> **A λ is a value.** Its effects happen when it is *applied*, not when it is written. This is the
> ordinary call-by-value rule, and it is load-bearing: `(fn (acc i) (shout i))` passed to
> `fold-range` must stay substitutable, or the loop body could not be emitted at all.

### Which is why a definition's body must be pure

Chapter 2 §2.3 showed this error without explaining it:

```lisp
(def noisy (shout 1))
(+ noisy noisy)
```

```
the body of noisy is a computation, not a value, so unfolding it would repeat its effects
  Wrap it in (fn () …) and apply it, or bind it with let at the point of use.
```

δ copies a definition's body to every occurrence. That is contraction, unguarded — there is no
application site to bind at, because a name is not an application. So the restriction moves to
the definition: **a definition's body must be pure.** A λ is pure, so every function you have ever
written is fine.

Take the repair the message offers:

```lisp
(def later (fn () (shout 1)))
(+ (later) (later))
```

```lisp
⟶   (+ (shout 1) (shout 1))
```

Two shouts, and correctly so — the source contains two applications. The λ was duplicated; the
*effect* was not, it was invoked twice. Those are different things and the distinction is the
whole of §4.8.

### Purity propagates

You never declare a definition pure. It is computed:

```lisp
(def sh (fn (n) (shout n)))
((fn (a) (+ 1 2)) (sh 5))
```

```lisp
⟶   (let (shout 5) (fn (a) (+ 1 2)))
```

`sh` is impure because applying it reaches `shout`, so `(sh 5)` is an impure argument and survives
its unused binder. A definition that reaches only pure primitives stays pure:

```lisp
(def pure-one (fn (n) (f n)))
((fn (a) (+ 1 2)) (pure-one 5))
```

```lisp
⟶   (+ 1 2)
```

One declared bit per primitive; everything else is inferred by reachability.

## 4.9 `seq` is not a form

Sequencing looks like a language feature and is not:

```lisp
(seq (shout 1) (whisper 2))
```

```lisp
⟶   (let (shout 1) (fn (_) (whisper 2)))
```

`(seq a b)` is read as `((fn (_) b) a)` — an application whose binder is **never used**. That is
it. The reader rewrites it and nothing downstream has heard of `seq`.

And it works **only because weakening is denied**. An unused binder is exactly the case §4.5 is
about: if effects could be dropped at zero uses, `(seq a b)` would reduce to `b` and sequencing
would silently do nothing. The ordering discipline and the ability to write "do this, then that"
are *the same mechanism*.

It chains:

```lisp
(seq (shout 1) (seq (whisper 2) (tick)))
```

```lisp
⟶   (let (shout 1) (fn (_) (let (whisper 2) (fn (_) (tick)))))
```

And it is honest about doing nothing when there is nothing to sequence:

```lisp
(seq (f 1) (+ 1 2))         ⟶   (+ 1 2)
(seq (shout 1) (+ 1 2))     ⟶   (let (shout 1) (fn (_) (+ 1 2)))
```

Sequencing a *pure* computation is a no-op, because there was no effect to order. Surprising the
first time; correct on reflection.

## 4.10 The default is impure, and that is the whole risk model

A third party writes a target file and forgets the word `pure`. What happens?

[targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) is `tutorial` with exactly that
mistake — `pure` deleted from `*`, nothing else changed. Same program, two targets:

```lisp
((fn (n) 7) (* 3 4))
```

```bash
-target=tutorial          ⟶   7
-target=tutorial-sloppy   ⟶   (let (* 3 4) (fn (n) 7))
```

```lisp
((fn (a b) (+ b a)) (* 1 2) (* 3 4))
```

```bash
-target=tutorial          ⟶   (+ (* 3 4) (* 1 2))
-target=tutorial-sloppy   ⟶   (let (* 1 2) (fn (a) (let (* 3 4) (fn (b) (+ b a)))))
```

Dead code not eliminated; arithmetic pinned in place. The program is **slower**. It is not wrong,
and the damage is visible in the emitted source.

Now imagine the default the other way. A target author forgets `effect` on their logging function,
and the compiler silently duplicates it, drops it, and reorders it. The program is **wrong**, and
nothing in the output looks unusual.

> **The default must be the one whose failure mode is slow, not wrong.**

That is why purity is declared rather than derived. The tempting derivation — `expr` is pure,
`stmt` is effectful — is wrong in both directions: `dict-empty` is an `expr` and allocates a fresh
identity; a `rand` would be an `expr` too. A heuristic that is right most of the time is the worst
kind, because it is trusted.

---

## 4.11 The three-way choice, and what everyone else picked

Now the part that is not about this language.

The structural rules of §4.3 are not a compiler detail. They are the *hinges* of logic, and taking
them away is a research programme with a century of results behind it.

Gentzen's sequent calculus (1934) states them explicitly, and once they are explicit you can ask
what happens if you drop one. **Substructural logics** are the answer:

| Logic | contraction | weakening | exchange | reads as |
|---|---|---|---|---|
| classical / intuitionistic | ✓ | ✓ | ✓ | values |
| **relevant** | ✓ | ✗ | ✓ | must be used |
| **affine** | ✗ | ✓ | ✓ | used at most once |
| **linear** (Girard, 1987) | ✗ | ✗ | ✓ | used exactly once |
| **ordered** (Lambek, 1958) | ✗ | ✗ | ✗ | used exactly once, in place |

Rust's ownership is affine. Session types are linear. Lambek's calculus, the bottom row, was
invented to describe *word order in natural language* — a sentence's words must be used exactly
once, in sequence — twenty years before anyone applied it to computing.

**Look at the bottom row again.** Exactly once, in place. That is §4.7's rule, word for word. An
effectful term in this language lives in the ordered fragment, and it was not put there by
design — it was derived from three concrete hazards and turned out to be a logic somebody named
in 1958.

### What we did not build

There are two famous ways to control effects, and this language has neither.

**Monads** (Moggi 1991, Wadler 1992) make the *type* carry the effect: `IO Int` is not `Int`, and
the type system prevents you from confusing them. Beautiful, and it costs you a type system, a
`do` notation, a bind operator, and a permanent split in your language between the two worlds.

**Effect systems** (Lucassen & Gifford 1988) annotate functions with the effects they may perform,
which means an effect *inference* pass and effect variables in every signature.

Both answer the question "what is the type of a term that shouts?" Here the question is never
asked, because nothing in the language has an effect type. `shout` has type `num → num`. The
effect is a property of a *name in a target file*, not of a term, and the only thing that reads it
is a side condition on one reduction rule.

The trade is real and worth stating plainly. Monads and effect systems let you *reason about*
effects — write a function's signature and know it cannot print. We cannot: nothing in a signature
here says anything about effects. What we get instead is that effects cost **nothing**: no wrapper,
no bind, no `runIO`, no colour on functions, and
[no measurable slowdown on six programs](../../gauntlet/results/effects-2026-08-14.md).

For a language whose requirement is parity with hand-written code, that is the right side of the
trade. For a language whose requirement is proving your database layer cannot send email, it is
not.

### Two zones, one border

There is a third thing in the literature and it is the closest fit. Benton's **Linear/Non-Linear**
logic (1994) does not choose between the structural world and the linear one — it has **both**,
side by side, joined at a border.

That is the shape here. Pure terms live in the full structural world and may be copied, dropped
and moved. Impure terms live in the ordered fragment and may not. One declared bit says which zone
a name is in, and β reads the bit per argument:

```lisp
((fn (a b c) (+ a (+ b c))) (shout 1) (f 2) (whisper 3))
```

```lisp
⟶   (let (shout 1) (fn (a) (let (whisper 3) (fn (c) (+ a (+ (f 2) c))))))
```

Two logics, one term, one reduction. `(f 2)` moved and `(shout 1)` did not, in the same β-step.

The honest caveat: this is a **resemblance**, not a claim. Benton's system is a pair of categories
joined by an adjunction and it proves things we have not stated, let alone proved. What is true is
that the *shape* — two zones with different structural rules, and a marker saying which — is a
known one, and knowing that is worth more than inventing a name for it.

### One last thing that is not a coincidence

This chapter's rules exist to stop reduction duplicating work. Writing it turned up the emitter
doing exactly that, one layer further down:

```go
fmt.Println((strings.Fields(s)))
return (strings.Fields(s))
```

The *value* of a statement is its argument, and returning the argument's expression wrote it
twice — two allocations for one source call, in a compiler whose whole call-by-need discipline
exists to prevent that. It is now bound to a local, and Go's machine code is unchanged either way
(91 instructions, identical), so the fix was free.

Contraction is easy to grant by accident. That is why it is worth naming.

---

## What to remember

- **There is no effect form.** One declared bit per primitive, in the target file, and a side
  condition on β. Nothing else in the language knows.
- **Purity is the licence to use the structural rules** — copy, drop, move. An impure term gets
  none of them.
- **An impure argument is never substituted.** It is let-bound at the *application site*, in
  argument order, exactly once, whatever its occurrence count.
- Those three clauses deny **contraction**, **weakening** and **exchange**, in that order, and
  each one prevents a specific disaster.
- **Purity and cheapness are different axes.** Both cause let-binding; only one is about
  correctness.
- **A λ is a value**, even with an impure body. Its effects happen on application. Which is why a
  **definition's body must be pure** — δ has no application site to bind at.
- **Purity propagates** by reachability. You declare primitives; definitions are inferred.
- **`seq` is a β-redex with an unused binder**, and it works *only* because weakening is denied.
- **The default is impure**, because a forgotten marker should make a program slow, not wrong.
- **An effectful term lives in the ordered fragment** — exactly once, in place — while pure terms
  keep the full structural rules. Two zones, one bit, one reduction rule.

Next chapter: types, `sig`, and refinements — the checker that is not in the language.
