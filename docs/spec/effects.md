# Effects

Written **before** the code, which is the order [state.md §6](state.md) requires.

[g5 §5](../derivations/g5-bindings.md) predicted that effects would break the rewriting core in a
way purity concealed, and called it the first *correctness* defect where every earlier one had
been a performance defect. This document specifies the fix.

It also corrects g5 on one point: the derivation said effects would *arrive* with program 5.
They are already here.

---

## 1. We already have effects

`dict-inc` is declared in every target as

```lisp
(prim dict-inc (dict string) dict stmt "%s[%s]++")
```

`stmt` means *the emitted line is a statement and the value of the term is argument 0*. So
`dict-inc` **mutates a dictionary in place and returns the same dictionary**. That is not
state-passing — it is destructive update wearing state-passing's clothes.

It is correct today for a reason nothing checks: the accumulator threaded through `fold-range` is
used **linearly**, so the pre-mutation value is dead at the point of mutation. Write a program
that reads the old dictionary after the update and the emitted code is wrong.

`dict-empty` is a second one, of a different species: two occurrences denote two *distinct*
dictionaries, so **allocation is an effect too** — identity is observable even when contents are
not.

The reason no program has been miscompiled is the same reason strings were cheap
([strings.md §3](strings.md)): **nothing can observe it.** No primitive reads a dictionary. The
safety is an accident of an impoverished vocabulary, not a property of the reducer.

> Adding `print-line` does not introduce effects. It makes the ones we have **observable**, and
> turns an unchecked assumption into a load-bearing one.

## 2. The three hazards are the three structural rules

g5 §5 listed the hazards informally. Stated against the reducer as built, they are exactly the
three structural rules of a sequent calculus, and one of them g5 missed.

`core/reduce.go` substitutes an argument when `occurrences(body, p) <= 1`.

| Hazard | Where | Structural rule |
|---|---|---|
| The argument is **dropped** when it occurs 0 times | `occurrences ≤ 1` includes 0 | **weakening** |
| The argument is **copied** when `duplicable` says a λ may be | `duplicable` admits `KFn` | **contraction** |
| The argument **moves** into a loop body or conditional arm | `occurrences` counts syntactically, with no notion of execution context | **exchange** |

The third is the one the code cannot currently see: `occurrences` returns 1 whether the
occurrence is at the top of the body or nested inside a `(fn (acc i) …)` that will become a loop
header. Substituting there turns one effect into *n*.

The first is the one g5 missed entirely, and it is the one that makes sequencing possible —
see §5.

> **Purity is the licence to use the structural rules.** A pure term lives in the ordinary
> cartesian fragment and may be copied, dropped, and moved. An impure term lives in the
> **ordered** fragment: exactly once, where it was written.

That is the fifth independent arrival of the substructural question g5 §9 catalogued — and the
first one that arrives with the standard names attached, which is what makes it a specification
rather than an observation. It is also the answer to whether this project needs linear types on
values: **it does not.** It needs the structural rules to be conditional on a one-bit property of
primitives.

## 3. What is pure

**Purity is declared per primitive, in the target file, and defaults to impure.**

```lisp
(prim add        (f64 f64) f64  expr "%s + %s" pure)
(prim print-line (any) any      stmt "fmt.Println(%s)" (import "fmt"))
```

The default is the interesting choice. A third party adds a target ([requirement
3](../design-direction.md)) and forgets the marker:

- **Default pure** → their effectful primitive is silently duplicated, dropped, and reordered.
  The failure is a **miscompilation**, and it is invisible.
- **Default impure** → their pure primitive is let-bound instead of folded. The failure is a
  **slower program**, and it is visible in the emitted source.

> **The default must be the one whose failure mode is slow, not wrong.**

The cost is one word on about forty lines across four target files, and it buys a reader of
`targets/go.oro` the ability to see which primitives are pure without consulting the compiler.

### Why not derive it from `kind`

Tempting — `expr` is pure, `stmt` is effectful — and wrong in both directions. `dict-empty` is
`expr` and allocates a fresh identity. A hypothetical `rand` would be `expr`. The heuristic is
good enough to be dangerous, which is the worst kind.

### Purity is not cheapness

Two properties, orthogonal, and both already needed:

| | may be copied | must keep its position |
|---|---|---|
| pure and cheap — `(add x 1)` | yes | no |
| pure and costly — `(split-words s)` | no, on **performance** grounds | no |
| impure — `(print-line x)` | **never**, on correctness grounds | **yes** |

The second row is the existing `duplicable` test, which
[measurement](../../gauntlet/results/wordcount-2026-08-14.md) put at 615× on Go. It stays exactly
as it is. Purity is a **new, independent** axis, and conflating them would cost the byte-identical
result in [duplicate-read](../../gauntlet/results/duplicate-read-2026-08-14.md), where duplicating
a pure `a[i]` is what the host's CSE erases for free.

### The judgement

A λ is a **value**. Its body's effects happen when it is applied, not when it is written — this is
the ordinary call-by-value rule, and it is load-bearing here: `(fn (acc i) (print-line i))` passed
to `fold-range` must remain substitutable, or it would be let-bound as a bare λ and reach the
emitter as an escaping closure.

```
pure(x)              = true                       for a name
pure(ℓ)              = true                       for a literal
pure(fn (x…) b)      = true                       a λ is a value
pure((f a₁ … aₙ))    = applying(f) ∧ ⋀ᵢ pure(aᵢ)

applying(fn (x…) b)  = pure(b)                    applying a λ runs its body
applying(p)          = p is declared pure         for a primitive p
applying(d)          = applying(body of d)        for a definition d
```

`applying` on definitions is a least fixed point over the definition graph, computed once, the
same shape as `markRecursive`. Recursion is handled by the same seen-set: a definition reached
from itself contributes nothing new.

Note that `pure(name f) = true` while `applying(f)` may be false. Those are different questions —
*is this term safe to move* versus *does calling this thing do something* — and keeping them
apart is what makes the λ rule sound.

## 4. The β side condition

> **An impure argument is never substituted.** It is normalised and let-bound at the application
> site, in argument order, exactly once — whatever its occurrence count.

That is the whole change, and each clause denies exactly one structural rule:

| Clause | Denies | Prevents |
|---|---|---|
| *exactly once* | contraction | one effect becoming two |
| *whatever its occurrence count* | weakening | an effect vanishing at zero uses |
| *at the application site, in argument order* | exchange | an effect moving into a loop or an arm |

**Why the application site is the right position.** In `((fn (x) b) e)` the term `e` is written
*outside* the λ, and uses of `x` may be anywhere inside `b` — under a loop, inside one arm of a
conditional. Binding at the application site puts the effect back exactly where the programmer
wrote it, at its original loop depth and under its original guards. Binding at the *use* site
would be the bug.

This is Plotkin's βv restricted to the terms that need it: pure arguments keep call-by-need,
impure arguments get call-by-value. The two strategies coexist because purity decides between
them per argument.

**δ needs no side condition, given one restriction.** Unfolding a name copies its definition's
body to every occurrence, which is contraction. It is safe because:

> **A definition's body must be pure.**

A λ is pure by the rule above, so every definition in `examples/` already satisfies this and
`(def report (fn (label xs) …))` is fine. What it rejects is `(def x (print-line "a"))` — a name
bound to a computation rather than a value, whose two occurrences would print twice. Rejecting it
is the same decision every call-by-value language makes.

**Reduction under λ needs no side condition.** Normalising a body moves nothing across a context
boundary; it reduces in place.

**Primitive applications need no side condition.** `(if c (print-line a) (print-line b))` is an
application of a primitive, not a β-redex, so its arguments are normalised in place and never
hoisted. The arms keep their guards.

## 5. Sequencing is `let` with an ignored binder

We add **no** sequencing construct, and **no** unit type.

```lisp
(seq a b)   ⟶   ((fn (_) b) a)
```

reader sugar, the same shape as `let` ([def.md](def.md)). And it works only because of the
weakening clause in §4: `_` occurs zero times, so a pure `a` is *correctly deleted*, and an impure
`a` is *correctly kept*. The one hazard g5 did not list is the one that makes sequencing
expressible at all.

No unit type is needed because `stmt` already specifies that the value of the term is argument 0,
so `(print-line label)` has label's type and value. Nothing consumes it. This is the same
mechanism `dict-inc` has used since word count, now with a specification.

**One emitter change follows.** `emitLet` currently always writes `x := v`. When the binder is
unused it must emit the value for its effect and continue without binding, or Go will reject the
program for an unused variable. That is three small edits, one per backend, and it is the only
code the sequencing story costs.

## 6. `print-line` is Tier 2, and cannot be otherwise

All three hosts disagree on the output of the same float ([g5 §6](../derivations/g5-bindings.md)):

| value | Go | JS | Java |
|---|---|---|---|
| `1.0` | `1` | `1` | `1.0` |
| `1e8` | `1e+08` | `100000000` | `1.0E8` |
| `-0.0` | `-0` | `-0` | `-0.0` |

Three targets, three answers, on the program whose entire purpose is producing output. So:

- **`print-line` is Tier 2.** Output is the host's. No portability claim. Cost zero.
- A Tier 1 `print-f64` would require implementing shortest-round-trip formatting (Ryū, Grisu)
  ourselves, which costs binary size — [requirement 6](../design-direction.md) — on every target
  at once, to serve no program that exists.

Deferred per [ADR 0007](../decisions/0007-exploration-over-specification.md): add it when a
program demands it.

This is the third arrival of the same pattern, after
[q5c](q5c-representation-choice.md) and [strings](strings.md): **the core stays portable by
refusing to expose what diverges.** Three is a design rule, not a coincidence.

## 7. The three-question test

From [state.md §6](state.md), applied to this addition:

1. **What does it mean, independently of any target?** An impure term denotes a computation that
   happens exactly once, in the position it was written. Formally: impure terms admit none of
   weakening, contraction, or exchange.
2. **What does each target do with it, and do they agree?** They agree on *ordering* — all three
   emit statements in sequence and evaluate arguments left to right. They disagree on the
   *rendering* of a float.
3. **Is the disagreement observable?** Yes, and it is the entire output. So `print-line` is Tier 2
   and carries no portability claim, while **the ordering discipline of §4 is Tier 1** and holds
   on every target.

The addition splits: the *discipline* is portable, the *primitive* is not. That split is the
reason this passes the test at all.

## 7b. Contraction is easy to grant by accident

**2026-08-16, found writing [chapter 4](../book/04-effects.md).** Everything in this document is
about stopping *reduction* from duplicating an effect. The **emitter** was duplicating work one
layer down, for a reason that rhymes: the value of a `stmt` primitive is its first argument, and
the emitter returned that argument's *expression* rather than binding it.

```go
fmt.Println((strings.Fields(s)))
return (strings.Fields(s))
```

Two allocations for one source call — the 615× class the call-by-need discipline exists to
prevent. It is now bound to a local in all three backends, unless the emitted form is already an
identifier or a literal, and Go's machine code is unchanged either way (91 instructions before and
after, identical stream), so the fix cost nothing.

The general lesson is worth keeping: **a discipline enforced at one layer is not enforced at the
next.** β refuses to copy; δ refuses to copy; the emitter had never been asked.

## 7c. Exchange was easy to LOSE by accident, and a swap is what found it

The rules above assume the effect discipline can *see* an effect. It could not
see a **buffer read**.

[ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) says `(array V)`
reads are pure and `(buffer V)` reads are **impure**, which is what stops a read
being moved across a store. But `pureTerm` answers *"value"* for every bound
variable, so `(b 0)` — an application whose operator is a `KBound` — was judged
pure. The smallest program that shows it is a swap:

```lisp
(let (b 0) (fn (vx) (let (b 1) (fn (vy) (set (set b 0 vy) 1 vx)))))
```

Both reads happen before either store and the program is correct. Both were
substituted into the store positions, and Go emitted

```go
b[0] = b[1]
b[1] = b[0]     // reads what it just overwrote
```

so **a swap silently became a copy, on all four targets** — which is why the
differential suite could not have found it either. It was latent because it
needs a read of a slot, then a store to that slot, then a *use* of the read
value; the tokeniser and the tree read buffers constantly and consume each read
inside the same expression as the store that follows. The first program to need
it was a sort.

### The fix tests the DESTINATION, not the operand

Nothing in a term says which bound variables are buffers: an array read has the
identical shape and is genuinely pure. So the rule is

> **A term that reads a table through a bound variable is not substituted into an
> impure body.**

A read may move freely into a body with no effects to be reordered against, and
not into one that has. That is exactly the property at stake, and it is
decidable at the β site because the body is in hand.

**Testing the operand instead — "any application of a bound variable is impure" —
was tried and measured and is wrong.** A rule-table's rule reads its parameter
table, so `(table n f)` becomes impure, stops being substituted, and reaches the
backend **unfused**: `dot` and `smooth` on Java stop compiling. Recorded so it is
not tried again.

**Cost: 2 of 164 emitted files change**, both gaining one let-bound temporary
before an impure call, and the one benchmarked program among them
(`examples/json/tree.oro`) measures **5,842 ns against 5,840** — indistinguishable,
against a 15% noise floor.

## 8. What this does not do

- **No effect types, no monads, no regions.** Purity is one declared bit per primitive and a
  structural judgement over terms. If something later needs to distinguish reads from writes, this
  is where it would go, and nothing here forecloses it.
- **No aliasing analysis.** §1's `dict-inc` hazard — reading a dictionary after mutating it — is
  *not* fixed by this document. Ordering is preserved; the destructive update is still unchecked.
  It remains unobservable because no primitive reads a dictionary, and it becomes real the moment
  one does. That is [g7](../derivations/g7-aliasing.md)'s question and it is still open.
- **No purity checking of the target's claim.** `(prim add … pure)` is believed. A target that
  lies about a primitive miscompiles, exactly as a target that lies about an emission template
  does.
