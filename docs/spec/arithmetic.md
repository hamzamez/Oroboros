# Arithmetic, booleans, and comparison

Written before the code, per [state.md §6](state.md).

> **Status, 2026-08-15. Built.** All of it is data — `core/` gained nothing, as predicted. The
> four target files declare `num/f64`, `num/int` and `logic`; the seven existing examples were
> migrated to qualified names and **emit byte-identical source on every target**; three tests
> named old unqualified primitives and were updated.
>
> `examples/stencil.oro` is the first program the language could not previously express. Measured
> against hand-written Go: **7,946 ns/op generated versus 8,855 hand-written** — inside the noise
> floor, so parity — while the materialised form the gauntlet carries as the expected loser costs
> **103,509 ns/op and 512 KB**, a 13× gap that the reducer closes with no fusion rules.

Closes three holes found by [inventory.md](inventory.md): there is no integer arithmetic, there is
no boolean logic, and there is no equality. All three land in **slot 1 — the parameter `P`** —
so **nothing here changes the core.** Six term kinds, two reduction rules, unchanged.

---

## 1. Why these, and only these

Not "they are useful". They are forced twice over, which is the standard this project accepts:

| | a program needs it | the checker needs it |
|---|---|---|
| integer `+ - *` | [g7](../derivations/g7-aliasing.md)'s stencil is `src[i-1]+src[i]+src[i+1]`, and has no `.oro` file because it cannot be written | linear arithmetic is the entire content of a bounds refinement |
| integer comparison | the same | ditto |
| `and`, `or`, `not` | `if` can currently branch on exactly one thing: a comparison of two floats | the boolean structure of a `where` |
| `=` | there is no equality in the language at all | `(= (alen a) (alen b))` |

Nothing else is promoted. **No division, no bitwise operations, no shifts, no data structures, no
error model** — see §7.

## 2. Would Church booleans help?

**They would hurt, and the reason generalises into a rule worth keeping.**

Church encoding gives `true = (fn (t f) t)`, `false = (fn (t f) f)`, and then `(if c a b)` is just
`(c a b)` — no primitive at all. That is exactly the trick `vec` already uses
([inventory §4a](inventory.md)), and it erases completely.

It erases **when the constructor is statically known**. β fires only when the operator of an
application is a λ. `vec` erases because `vlen` applies the encoded value to a *known* selector, so
the redex exists. A boolean's constructor is a **runtime value** — that is what makes it a
conditional — so `(c a b)` has a *variable* in operator position, no redex exists, and reduction
is stuck on a term all three emitters reject:

> `application of a non-name: the operator must be a primitive or a recursive definition`

It is worse than stuck. Church booleans in a strict setting evaluate **both** branches, which is
wrong for effects ([effects.md §4](effects.md)) and wasteful for everything. Fixing that needs
thunks — `(c (fn () a) (fn () b))` — which are closures, which allocate, which every backend
refuses.

So, the rule:

> **An encoding is free exactly when the eliminator's scrutinee is statically known. Encode types
> whose constructor the compiler can see; make primitive those whose constructor is a runtime
> value.**

Which is the familiar "products erase, sums allocate", derived from our own mechanism rather than
imported. And it yields a characterisation of something that has looked arbitrary until now:

> **The four structural primitives — `cond`, `loop`, `loop2`, `let` — are exactly the eliminators
> whose scrutinee is dynamic.** That is why the set is what it is, and why it is closed.

## 3. Booleans

**No boolean literal, and therefore no seventh term kind.** Checked honestly: `and`/`or`/`not` as
expression primitives need no literal, `if` takes its condition from a comparison, and no `where`
clause needs a constant. [ADR 0007](../decisions/0007-exploration-over-specification.md) — add it
when a program demands it.

```lisp
(module logic
  (prim and (bool bool) bool expr "%s && %s" pure)
  (prim or  (bool bool) bool expr "%s || %s" pure)
  (prim not (bool)      bool expr "!%s"      pure))
```

**Why primitives and not `(if a b false)`.** All three hosts have `&&` and `||` natively, and they
short-circuit. Lowering to a conditional would emit an if-statement and a temporary where the host
has an operator — *lowering further than the target requires*, which CLAUDE.md names as the single
most common way to get this architecture wrong.

**Meaning.** `and` is conjunction, `or` is disjunction, `not` is negation, on the two-element
Boolean algebra. All three targets agree exactly; there is nothing to diverge.

**Short-circuiting, and one recorded concern.** The emitted `&&` short-circuits on every target.
But an *impure* argument is let-bound at the application site before the operator runs
([effects.md §4](effects.md)), so it would be evaluated unconditionally. No boolean-valued
primitive is impure today, and none is proposed. Recorded in [concerns.md](concerns.md) rather
than solved, because solving it means `(if a b false)` and that costs the native operator.

## 4. Integers

### The measurement

`int` must mean the same thing on Go, JavaScript and the JVM. Measured, 2026-08-15:

| | Go `int64` | Java `long` | JS `Number` |
|---|---|---|---|
| `4503599627370496 + 2` | 4503599627370498 | 4503599627370498 | **4503599627370498** |
| `9007199254740992 + 1` | 9007199254740993 | 9007199254740993 | **9007199254740992** |
| `3037000499 × 3037000499` | 9223372030926249001 | 9223372030926249001 | **9223372030926249000** |
| `9223372036854775807 + 1` | −9223372036854775808 | −9223372036854775808 | **9223372036854776000** |

Three hosts, **three different failure modes at two different thresholds**: JavaScript loses
precision at 2⁵³ and silently *rounds*; Go and the JVM are exact to 2⁶³ and then silently *wrap*.
Java's 32-bit `int` wraps at 2³¹, so `long` is the only usable width.

And below 2⁵³, all three agree **exactly**.

### The specification

> **An `int` is a mathematical integer. Arithmetic on it is exact.**
>
> **The portable range is −(2⁵³−1) … 2⁵³−1.** A program whose intermediate *and* final integer
> values all lie inside it computes the same answer on every target.
>
> **Outside that range the behaviour is the target's**, differs between targets, and carries no
> portability claim.

This is [ADR 0003](../decisions/0003-range-typed-integers.md) made concrete — *mathematical
semantics, machine representation* — and it passes the three-question test
([state.md §6](state.md)) by the same move as [strings](strings.md): it is portable because it
refuses to claim the region where the targets diverge.

The bound is JavaScript's and it is not negotiable without emulation. `BigInt` would make it 2⁶³
at a cost nobody has measured and no program needs; recorded and refused.

**Note what this leaves undone deliberately.** "Stay inside the range" is an obligation on the
programmer that nothing checks — and it is *precisely* a refinement,
`{ int | -(2^53-1) ≤ n ≤ 2^53-1 }`. The specification has a hole exactly where the type system
will later plug it, which is the right shape for a hole to have.

### Spelling

| our type | Go | JS | Java |
|---|---|---|---|
| `int` | `int64` | *(untyped)* | `long` |

Go's plain `int` is 64-bit on our targets but is not guaranteed to be, so `int64` is emitted
explicitly. Java's `int` is refused outright: it wraps at 2³¹, inside the portable range.

### The primitives

```lisp
(module num/int
  (prim add (int int) int  expr "%s + %s" pure)
  (prim sub (int int) int  expr "%s - %s" pure)
  (prim mul (int int) int  expr "%s * %s" pure)
  (prim lt  (int int) bool expr "%s < %s"  pure)
  (prim le  (int int) bool expr "%s <= %s" pure)
  (prim gt  (int int) bool expr "%s > %s"  pure)
  (prim ge  (int int) bool expr "%s >= %s" pure)
  (prim eq  (int int) bool expr "%s == %s" pure))
```

On JavaScript `add`/`sub`/`mul` are `%s + %s` and so on directly, because a `Number` *is* the
representation; nothing is emulated and nothing is coerced.

## 5. Arithmetic moves into modules

Today `add`, `sub`, `mul`, `lt`, `gt` are unqualified and mean **f64**. That was never stated
anywhere and it is not survivable once there are two numeric types, because primitive names are
unique keys.

> **`num/int` and `num/f64` each export `add sub mul lt le gt ge eq`. The root module exports no
> arithmetic.**

```lisp
(use num/f64)
(def dot (fn (a b) … (f64.add acc (f64.mul x y)) …))
```

Symmetric, unambiguous, and it uses the module system rather than inventing a second mechanism.
It also makes the existing target files honest for the first time — nothing currently says that
`add` is float addition.

**Cost:** every example gains a `(use …)` and a qualifier. That churn is worth taking now rather
than later, on the principle that the interface is the part that cannot be taken back.

**The alternative, refused for now:** one overloaded `add` resolved by argument type. That is the
right end state — it is [types-sketch §5](../types-sketch.md)'s resolution rule, *among candidates
whose precondition is provable, take the strongest* — and it needs a checker that does not exist.
Qualified names cost nothing and can be unified later without changing any target file.

## 6. Equality

`eq` is **per type**, not one polymorphic operator, because the three types do not agree on what
equality means:

| | meaning | equivalence relation? |
|---|---|---|
| `num/int.eq` | mathematical, inside the portable range | yes |
| `num/f64.eq` | **IEEE-754** — so `(eq NaN NaN)` is **false** and `(eq 0.0 -0.0)` is **true** | **no** |
| `text/string.eq` | scalar-sequence equality; representation-independent, so portable ([strings.md §4](strings.md)) | yes |

`num/f64.eq` is IEEE because every host's `==` is, and emitting anything else would mean not using
the host's comparison — the same refusal as everywhere else. The consequence is stated plainly
because it will matter: **float equality is not reflexive, so it is not an equivalence relation,
and a solver must not be allowed to assume it is.** Any `where` clause mentioning `num/f64.eq` is
therefore **outside the decided fragment** and is propagated as an opaque atom
([inventory §4b](inventory.md)), never used as an equality.

`num/int.eq` and `text/string.eq` are ordinary equalities and are inside the fragment.

## 7. Deliberately absent

- **Division.** Integer division disagrees on rounding toward zero versus negative infinity for
  negative operands, and division by zero traps on Go and the JVM and yields `Infinity` on
  JavaScript. It needs its own specification and no program needs it yet.
- **Bitwise operations and shifts.** JavaScript's bitwise operators coerce to **32-bit**, which
  silently contradicts §4's range. A trap, correctly avoided.
- **`neg`, `abs`, `min`, `max`, `mod`.** Derivable, or absent until wanted.
- **`ne`.** `(logic.not (int.eq a b))`.
- **Mixed arithmetic and coercion.** There is no implicit conversion between `int` and `f64` and
  no `int→f64` primitive yet. It will be wanted; it needs a rule about which direction is lossless
  and what happens past 2⁵³, and that is a separate document.

## 8. What this does not change

`core/` gains nothing. No term kind, no reduction rule, no side condition. Every item above is a
line of data in a target file — which is the test [inventory §4b](inventory.md) proposed for any
addition, and the reason this document could be short.
