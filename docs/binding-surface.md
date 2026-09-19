# How a program binds and branches: `let`, several results, `seq`, `cond`, and variadic forms

Research, **no decision**. 2026-09-19, on hamza's four questions:

> **Decided and built the same day**, for `let` and `seq`: [spec/binding.md](spec/binding.md) is the
> specification and [letflat-2026-09-19](../gauntlet/results/letflat-2026-09-19.md) the result.
> Recommendation 3 (use `cond`) followed on the same day: [cond-2026-09-19](../gauntlet/results/cond-2026-09-19.md),
> which found §5 half right — `cond` is free only once a negated condition swaps its branches.
> Recommendation 4 (the reader should own arity) is **not** built.

1. Shen's `(let v1 e1 v2 e2 … body)` against ours, `(let e (fn (x) b))`, which reads like F#'s and
   Elixir's `|>`;
2. several results — `((os.ReadFile (av 1)) (fn (src err) …))` against a destructuring binding,
   `(let (tuple src err) (os.ReadFile (av 1)) …)`;
3. `seq` in the same frame;
4. the line counter's nested `if`s against `cond` — and, separately, whether Shen's variadic
   `let`/`do`/`and`/`+` (macros, there) is the neat way to get n-ary forms.

Nothing is built. Everything below is measured on today's corpus or derived, and §7 is the
recommendation.

---

## 1. What we have, measured

`examples/`, `lib/`, the differential cases and the acceptance programs:

| shape | count |
|---|---:|
| `(let …)` | **137** |
| one-parameter continuations `(fn (x) …)` — the `let` shape | **233** |
| two- and three-parameter continuations — several results, tuples | **68** and **4** |
| `(seq …)` | 59 |
| `(match …)` | 2 |
| **`(cond …)`** | **0** |

Two of those numbers are findings on their own. **`cond` is used nowhere**, though the language has
had it since ADR 0017. And the `let` chains get deep: `walk` in `tree.oro` binds **nine** names and
closes with **twenty-four** parentheses:

```lisp
(let (wl (ssl2 (go.- sp 1) 0)) (fn (n)
(let (wl (ssl2 (go.- sp 1) 1)) (fn (d)
(let (nodes (nsl n 3)) (fn (sb)
…
  (again w2 s3 (go.+ seen 1) … )))))))))))))))))))))))
```

Every name arrives **after** its value, and each binding costs two lines and two closing parens.

## 2. What `let` is, and what the choice really is

`(let e k)` desugars in the reader to `(k e)` — an application ([def.md §6](spec/def.md)). So the
construct is **the application of a continuation**, and the surface question is only which order the
two halves are written in:

| order | who writes it that way | reads as |
|---|---|---|
| **value, then binder** | ours; F#'s `|>`; Elixir's `|>`; CPS by hand | *"take this, and then with it…"* |
| **name = value, then body** | Shen, Clojure, Scheme `let*`, ML `let`, Haskell | *"let n be this; in…"* |

Both are the same term. Nothing downstream can tell, because the reader erases the difference.

**Sequential is the only sensible semantics here**, and that settles a distinction other Lisps carry.
Scheme has `let` (parallel), `let*` (sequential) and `letrec`; Clojure and Shen have only the
sequential one. Ours *is* sequential by construction — each continuation is nested inside the last,
so every binding sees the previous names — and parallel bindings would buy nothing: with no
recursion (ADR 0014) and whole-program reduction, a parallel `let` is a renaming of a sequential one.
`letrec` is refused for the same reason recursion is.

## 3. Several results is not pattern matching — it is the eliminator

`(tuple a b)` reads as `(fn (k) (k a b))` ([data.md §3](spec/data.md)), the Church presentation. So
**eliminating a tuple IS applying a continuation**, and

```lisp
((os.ReadFile (av 1)) (fn (src err) …))
```

is not a call followed by a callback; it is the tuple's eliminator, written in application order. Which
means a destructuring binding is the *same term*, spelled in binding order:

```
(let (tuple a b) e body)   ⟶   (e (fn (a b) body))
```

No new mechanism, no pattern-match compiler, nothing for the analyses to learn. And it explains why
Clojure's destructuring feels like a different feature and ours does not: Clojure destructures *data*,
we eliminate a *function*.

**The line to hold is refutability.** A tuple's eliminator is total — one case, always applicable — so
it belongs in `let`. A variant's is not: `(ok v)` may not match, so it belongs in `case`, where
exhaustiveness is checked ([sums.md](spec/sums.md)). *Irrefutable patterns bind; refutable patterns
branch.* That is the standard line (ML, Haskell, Rust all draw it) and it falls out of the algebra
here rather than being imposed: a product has one constructor, a sum has many.

So the candidate binding form is one rule with two left-hand sides:

```lisp
(let  n            e          …body)      ; a name
(let  (tuple a b)  e          …body)      ; a tuple, irrefutable
```

## 4. `seq` is a `let` whose name is discarded

`(seq a b)` reads as `((fn (_) b) a)` — a binding nobody reads. It works **only** because ADR 0010
denies weakening to an impure term: `_` occurs zero times, so a pure `a` is correctly deleted and an
impure one is correctly kept ([effects.md §5](spec/effects.md)).

In a flat `let` it could be written `(let _ a  b)`, and Clojure's `do`, Shen's `do` and Scheme's
`begin` are all the same idea. **Keep `seq` as its own word anyway**: it is already variadic, it says
*sequence* where `let _` says *bind nothing*, and the one thing worth stating is the relationship
rather than merging the two. Merging would also make the discarded-binder rule invisible, and that
rule is load-bearing.

## 5. `cond` against nested `if` — measured

hamza's readability question, tested rather than argued. `wc.oro`'s body rewritten with `cond`:

```lisp
(cond
  (not (os.err-nil err))  (seq (io.print-line "wc: cannot read that file") 0)
  (>= (len src) cap-in)   (seq (io.print-line "wc: file is larger …") 0)
  else                    (loop …))
```

- **Same proof counts** — 2 of 2 operations, 1 of 1 loop.
- **Same emitted code**, with the branches in the order the clauses are written.
- **The cost of flatness is one `not`**: a nested `if` tests `err-nil` once and uses both arms; a flat
  clause chain has to state the negation to put the failure first.

> **Corrected on building it** ([cond-2026-09-19](../gauntlet/results/cond-2026-09-19.md)). The
> second bullet is wrong as measured here: the emitted code was **not** the same. It grew a `!` and
> its branches came out in the opposite order, because the negation stayed in the condition. `cond`
> became free only after the reader was taught to swap a negated condition's branches, which is
> case-of-case plus `(if true a b) → a` performed eagerly. The first and third bullets stand.

So `cond` is free, and the reason to prefer it is exactly hamza's: three outcomes read as three
clauses instead of a staircase. **And the language already speaks that idiom twice** — `loop`'s
clauses and `match`'s are `condition body … else body` — which is an argument for the flat `let` too:
one shape for binding and branching, rather than two.

## 6. Variadic forms: what licenses them

Shen's `let`, `do`, `and`, `+` are variadic because they are **macros**. We have no macros and do not
want them ([theories.md §7](spec/theories.md): *extend the rules, never the fragments*) — but we do not
need them, because **the reader is a closed macro expander**: `and`, `or`, `cond`, `seq`, `match` and
`tuple` are already n-ary there, and nothing downstream knows.

What *licenses* an n-ary form is algebra, not convenience:

| | law | n-ary? |
|---|---|---|
| `and`, `or` | monoid, identity `true` / `false` | **yes, already** — and `(and)` = `true` is forced by the identity, not chosen |
| `+`, `*` over ℤ | monoid, identity `0` / `1` | sound: `ℤ` is associative |
| `-`, `/` | **not associative** | no law to appeal to; CL and Scheme pick a left fold **by fiat** |
| `+` over `f64` | **not associative** (IEEE-754) | **unsound to fold in the reader**: the association changes the answer, which is ADR 0009's rule |

The float row is the interesting one: a variadic `+` would make the *reader* choose an association, and
for floats that is observable. Our `+` is the language's **integer** operator (float arithmetic is a
host name, `go.f64.add`), so a variadic integer `+` is sound — but the rule to write down is *n-ary
where the operation is associative, with the empty case exactly where there is an identity*.

**And today's refusal is in the wrong place.** `(+ 1 2 3)` is read happily and refused at emission:

```
build: + takes 2 argument(s), given 3
```

That is the literal-elements shape again — a claim the language could check, left to a later layer.
Whether `+` becomes variadic or stays binary, the reader should be the one to say so.

## 7. Candidates, and what each costs

### 7.1 `let`

| | form | |
|---|---|---|
| **A** | `(let e (fn (x) b))` — today | value first; one binding per two lines; deep nesting |
| **B** | `(let x e … body)`, Shen/Clojure-shaped, sequential, with `(tuple a b)` allowed as a left-hand side | flat, names first, one closing paren; matches `cond`/`loop`/`match`'s idiom |
| **C** | a binding vector, `(let [x e y e2] body)` | Clojure's exactly; needs a bracket the reader does not have |
| **D** | both A and B | two spellings of one construct, which [data.md §10](spec/data.md) refuses |

**B is unambiguous against A by parity**: a `let` form with three elements is A, and one with an even
number of bindings plus a body is B — but supporting both is D, so B should *replace* A and refuse it
naming the new spelling, as `sum`→`variant` and `values`→`tuple` did.

`walk`'s nine bindings under B:

```lisp
(let n  (wl (ssl2 (- sp 1) 0))
     d  (wl (ssl2 (- sp 1) 1))
     sb (nodes (nsl n 3))
     …
     s3 (if (= kd 0) s2 (+ s2 1))
  (again w2 s3 (+ seen 1) (+ acc (* (nodes (nsl n 0)) d)) (+ steps 1)))
```

Nine lines instead of eighteen, one closing paren instead of twenty-four, and every name to the left of
its value.

### 7.2 What it costs the compiler

**Nothing below the reader.** Measured for the `cond` rewrite already: same term, same proof counts,
same emitted bytes. Specifically:

- **The residual is untouched.** `let` in a residual is the primitive β produced when it declined to
  substitute; the source spelling is a different object that happens to share the word
  ([def.md §6](spec/def.md), which records that conflating the two was once a real bug). Respelling the
  source half actually *separates* them.
- **No new word**, so [inventory.md](spec/inventory.md)'s count stays at 136 — `let` and `tuple` both
  exist.
- **`ToForm` is shared**, so `provides` blocks get the same spelling.
- **The corpus moves**: 137 `let`s and 72 multi-name continuations, mechanically, with emission
  byte-identical as the acceptance — which is exactly how the `def` shorthand was done.
- **Diagnostics** print the desugared λ, as they already do for `let`, `match` and the `def`
  shorthand. Positions on terms are owed anyway ([data.md §10](spec/data.md)).

### 7.3 Recommendation

1. **Adopt B**, the flat sequential `let`, with a **name or a `(tuple …)` pattern** on the left. It
   subsumes hamza's second question: several results stop needing a continuation at the call site.
2. **Keep `seq`**, and state in the spec that it is a binding whose name is discarded.
3. **Use `cond`** where three or more outcomes are chosen between, starting with the README's line
   counter — it is free, and the corpus's zero uses is a sign the examples are teaching the staircase.
4. **Make the reader own arity**: `(+ 1 2 3)` should be refused where it is written. Then decide n-ary
   `+`/`*` on the monoid law alone, and leave `-`/`/` binary, because a left fold there is a choice
   rather than a law.
5. **No user macros.** The reader is a closed expander and that is the whole of what Shen needs macros
   for here.

### 7.4 What would refute B

A program where the pipeline order reads *better* — a long chain of transformations, each feeding the
next, which is what `|>` exists for. The corpus has one shape like that (`freq`'s sort passes), and B
should be checked against it before the rewrite, not after.

## 8. Deliberately not proposed

- **Parallel `let` or `letrec`** — §2.
- **Refutable patterns in `let`** — §3; that is `case`'s job.
- **Binding vectors** (`[…]`), which would need a bracket the reader does not have.
- **User-defined macros** — §6.
