# Binding: `let`, several results, and `seq`

The specification of the binding surface, derived in
[binding-surface.md](../binding-surface.md) and built on 2026-09-19
([letflat-2026-09-19](../../gauntlet/results/letflat-2026-09-19.md)).

One construct is specified here, in two spellings that are the same term:

```lisp
(let  name         value  …  body)
(let  (tuple a b)  value  …  body)
```

and `seq`, which is that construct with its name discarded.

---

## 1. What a binding is

`(let x e b)` reads as `((fn (x) b) e)`. It is **β's own redex**, so the construct adds nothing to
the reducer, nothing to the type checker and nothing to a backend — the reader erases it, and what
survives is an application ([def.md §6](def.md)).

That is the whole of the semantics, and everything below is a consequence of it rather than an
additional rule.

A `let` in a **residual** is a different object with the same name: the primitive β produced when it
declined to substitute (def.md §6, which records that conflating the two was a real bug). Since
2026-09-19 the two cannot be confused by eye either, because the source spelling is `(let x e b)` and
the residual's is `(let e (fn (x) b))`.

## 2. Grammar

```
(let LHS VALUE  LHS VALUE  …  BODY)

LHS  ::=  name  |  _  |  (tuple name name …)
```

- **k ≥ 0 bindings, then exactly one body**, so the form has an odd number of elements after `let`.
- **Sequential**: each VALUE sees every name bound to its left.

```
(let x e b)               ⟶  ((fn (x) b) e)
(let (tuple a b) e body)  ⟶  (e (fn (a b) body))
(let x e y f body)        ⟶  (let x e (let y f body))
(let body)                ⟶  body
```

## 3. Sequential, and only sequential

Scheme carries three binding forms — `let` (parallel), `let*` (sequential) and `letrec`. Clojure,
Shen and this language have one, and it is the sequential one.

- **Parallel buys nothing.** Each continuation is nested inside the last, so a parallel `let` is an
  α-renaming of a sequential one. With no recursion
  ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)) and whole-program reduction,
  no program distinguishes them.
- **`letrec` is refused** for the same reason recursion is. There is no fixpoint to take.

## 4. Why it is n-ary, and why the empty case is legal

The same law that licenses `(and a b c)` licenses `(let x e y f body)`, and it is the only thing that
does ([binding-surface.md §6](../binding-surface.md)):

| | law | consequence |
|---|---|---|
| nesting bindings | **associative** — `(let x e (let y f b))` and `(let x e y f b)` are one term | the n-ary form is a notation, not a choice |
| a body with no bindings | the **identity** — `(let b)` is `b` | the empty case is forced, exactly as `(and)` is `true` |

An operation with no law gets no n-ary form: `-` and `/` stay binary, because a left fold there would
be a decision made by the reader rather than a theorem (CL and Scheme pick one by fiat).

`(let b)` is therefore **accepted, not chosen**. It is what a generated binding list with nothing in
it has to mean.

## 5. Several results is the product's eliminator, not pattern matching

`(tuple a b)` reads as `(fn (k) (k a b))` ([data.md §3](data.md)), so eliminating a tuple **is**
applying a continuation, and

```lisp
((os.ReadFile p) (fn (src err) body))
```

is that eliminator written in application order. The binding spelling is the *same term*:

```lisp
(let (tuple src err) (os.ReadFile p) body)
```

No pattern-match compiler, no new mechanism, nothing for the analyses to learn. Clojure's
destructuring takes apart *data*; this eliminates a *function*.

**The line is refutability.** A product has **one** constructor, so its eliminator is total and
belongs in a binding. A sum has several, so `(ok v)` may fail to match and belongs in `case`, where
exhaustiveness is checked ([sums.md](sums.md)).

> *Irrefutable patterns bind; refutable patterns branch.*

ML, Haskell and Rust all draw that line. Here it is not imposed — it follows from the arity of the
type's constructor.

Consequences, each of them a refusal in §7:

- a tuple pattern binds **names**, not patterns: `(tuple a (tuple b c))` is refused, because a nested
  pattern is a second `let`;
- `(tuple a)` is refused, as the constructor is — a tuple of one is the value;
- any other list on the left is refused naming `case`.

**Arity is not checked here.** `(let (tuple a b) e body)` where `e` gives three results is an
application of arity 2 to a function of arity 3, and the type checker refuses it where it refuses
every other arity mismatch.

## 6. `seq` is a binding whose name is discarded

```lisp
(seq a b c)   ⟶   ((fn (_) ((fn (_) c) b)) a)
```

which is `(let _ a (let _ b c))`. Both spellings exist because `_` is an ordinary name and §2's rule
applies to it; **`seq` is the word to use**, it is n-ary, and it says *sequence* where `let _` says
*bind nothing*.

It works **only** because β denies weakening to an impure term ([effects.md §4–5](effects.md)): `_`
occurs zero times, so a pure `a` is correctly deleted and an impure `a` is correctly kept. That is the
whole of the sequencing story — no statement form, no unit type — and the rule is load-bearing, which
is why merging `seq` into `let` would be a loss: it would make the discarded binder invisible.

`(seq a)` is refused. Unlike `let`'s empty case there is no identity to appeal to: `seq` sequences two
or more terms, and a `seq` of one is the term.

## 6b. `build` and `build-map` bind the same way

```lisp
(build b n  c m  body)      ⟶   (build n (fn (b) (build m (fn (c) body))))
(build-map m cap  body)     ⟶   (build-map cap (fn (m) body))
```

A scoped buffer's λ is a binding occurrence, never a function value, so the source writes it as a
binding: name, then what it binds, then one body, n-ary and sequential, exactly §2–§4. Unlike `let`,
the right-hand side is not β's redex. It is the core form the backends consume, so the reader stops at
that form and does not go further. The specification is [tables.md §2.4](tables.md). What a `build`
may return is [ADR 0031](../decisions/0031-a-builds-result-is-a-product.md).

## 7. Edge cases, and what each says

| written | why | answer |
|---|---|---|
| `(let e (fn (x) b))` | the old spelling | *"let binds a name to a value: `(let x VALUE BODY)`; the old `(let VALUE (fn (x) …))` spelling is refused (spec/binding.md)"* |
| `(let x e)` | a binding with no body — the same shape as the old spelling, and the same message serves both | as above |
| `(let x e y f)` | an even number of elements: a binding with no value, or a body missing | *"let takes NAME VALUE pairs and ONE body"* |
| `(let (f x) e b)` | a left-hand side that is neither a name nor a tuple | *"let binds a name or `(tuple a b …)`; a pattern that may not match belongs in `case`"* |
| `(let (tuple a (tuple b c)) e b)` | a nested pattern | *"a tuple pattern binds names; write a second `let` for a nested one"* |
| `(let (tuple a) e b)` | a tuple of one | *"a tuple of one is just the value"* |
| `(let (tuple a a) e b)` | a name repeated **in one pattern** | the parameter-list rule, unchanged: the second binding is unreachable |
| `(let x e x f b)` | a name repeated **across bindings** | **accepted** — these are nested binders, and shadowing is legal and deliberately unreported ([def.md §11](def.md)) |
| `(let b)` | no bindings | **accepted**, and equal to `b` — §4 |
| `(seq a)` | §6 | *"seq takes two or more terms"* |

**`again` under a binding.** [ADR 0015](../decisions/0015-loop-and-again.md) permits `again` to sit
under a `let`, and the flat form changes nothing: `(let x e y f (again …))` is nested one-name
bindings, which is the shape the rule already names. **A tuple binding is a binding too**
([ADR 0027](../decisions/0027-a-host-calls-continuation-is-a-tail.md)):

```lisp
(let (tuple h l over) (u.mulw h l k)
  (again (+ k 1) h l over))
```

After reduction its value is a host call with several results and its body is that call's
continuation, which runs once, immediately, and which every backend emits as statements after the
call. The reader recognises the binding by the mark it puts on the tuple pattern it desugars, never by
the shape: `(f (fn (a b) (again …)))`, a lambda handed to one of the program's own functions,
desugars to the same shape and is still refused with *"`again` may be a clause body, or sit under a
`let`"*.

Binding the whole tuple and projecting it, `(let t (u.mulw h l k) (again (t …) …))`, is still not the
way to write it when the tuple is built under a host call. `t` then binds a host call, not a value,
and β declines to duplicate it, so the tuple survives as a closure the emitter refuses. Destructure
with a tuple pattern instead ([u128-2026-09-23](../../gauntlet/results/u128-2026-09-23.md)).

## 8. What this costs the compiler

**Nothing below the reader**, measured: the same terms, the same proof counts, the same emitted bytes
([letflat-2026-09-19](../../gauntlet/results/letflat-2026-09-19.md)).

- No new word — `let`, `tuple` and `seq` all existed — so [inventory.md](inventory.md)'s count is
  unchanged.
- `ToForm` is shared, so `provides` blocks and target files get the same spelling.
- Diagnostics print the desugared λ, as they already do for `let`, `match` and the `def` shorthand.

## 9. Deliberately not specified

- **Parallel `let` and `letrec`** — §3.
- **Refutable patterns in a binding** — §5; that is `case`'s job.
- **Binding vectors** (`(let [x e] b)`), which would need a bracket the reader does not have.
- **User macros.** The reader is a closed expander, and `and`, `or`, `cond`, `seq`, `match`, `tuple`
  and `let` are n-ary inside it ([theories.md §7](theories.md): *extend the rules, never the
  fragments*).
