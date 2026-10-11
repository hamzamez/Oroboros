# The language surface, designed one construct at a time

Status: **an open conversation, appended to as it goes. Nothing is decided.** Started 2026-10-11 on
hamza's request: *"let's try to come up with the best language surface we can … we start with the atom
lambda … let's find out the syntax with the least holes in it, simplest, most uniform … we are going to
propose the most right now, pick a few, drop the rest and continue the exercise with the rest of the
language, but what comes after has to fit with what we pick now."*

The semantics is fixed and is not under discussion here. It is
[abstract.md](spec/abstract.md)'s signature Σ, [settypes-research.md](settypes-research.md)'s logic
with [ADR 0049](decisions/0049-types-are-sets-and-the-logic-never-prints.md), and [fit-research.md](fit-research.md)'s fourteen
places where that semantics chooses a form. The candidate notations are
[notation-research.md](notation-research.md)'s and [syntax-research.md](syntax-research.md)'s. **Only
the spelling is open.**

**The method, per installment.** State what the construct must carry, derived from Σ. Propose many
spellings, including ones no language uses. Measure them against the same criteria. Keep two or three,
drop the rest with the reason. Then the next installment must fit what was kept.

**The criteria**, in this order:

| | criterion | how it is judged |
|---|---|---|
| **H** | **holes** — information Σ carries that the spelling has no place for | counted; a hole is disqualifying |
| **U** | **uniformity** — how many distinct shapes a reader must learn | counted |
| **F** | **fit** — does the spelling distinguish what Σ identifies, or identify what Σ distinguishes (fit-research §1.2's five misfits) | named per candidate |
| **E** | **erasure** — can a reader see what survives staging by looking? (ADR 0049: claims are inert) | argued |
| **S** | **size** — tokens, depth, longest closing run | counted by script, as syntax-research §4 |

---

## Installment 1 — the λ

### 1.1 The problem, as hamza put it

```lisp
(fn ((x S) y) e)                           ; application: ((fn ((x S) y) e) a b)
```
> *"but this format tells nothing about the returned value if we want to mention where it belongs."*

```
(λ x ∈ S, y : e)(a, b)
```
> *"same here: where would we put the type of the returned value if we wanted."*

> *"`fn(x ∈ S, y) r ∈ S { e }` — so it would be `fn/λ (args) return { body }`. I suppose we could do
> the same with `(λ (x ∈ S, y) r ∈ S : e)(a, b)`."*

The observation is correct, and it is sharper than it looks. **Today the λ cannot carry a claim at
all.** Verified against the compiler at `47eb569`:

```lisp
(fn ((x num)) (* x x))
;; line 1: fn parameter must be a name, got (x num)
```

A parameter is a bare name ([spec/def.md §4](spec/def.md)), so there is no place for a parameter's
set, let alone a result's. A *named* definition gets its claims from a second form:

```lisp
(sig sq ((n num)) num)        ; the name once, the arity once
(def sq (n) (* n n))          ; the name again, the arity again
```

and that form cannot name a λ:

```lisp
(sig (fn (x) x) ((n num)) num)
;; sig takes a name, a parameter list and a result type
```

So the hole is real, it is **two** holes, and they are the two fit-research already names:
**§3.4** (claims split across `sig` and `def`, the name and arity written twice) and this one, which
is new here: **a λ's claims are unstateable.**

### 1.2 What the λ must carry

From [abstract.md §2.4](spec/abstract.md) the λ is `lam(b̄; e) : (Bind*. Expr) → Expr` — two slots,
no claims. From §2.6 a definition is

```
def(f; b̄?; φ; r ∈ U; ψ; body)
```

and states **one membership**, the shape of a Z schema (Spivey 1992):

```
f ∈ Π( x̄ ∈ {x̄ ∈ S̄ : φ} ). { r ∈ U : ψ }
```

Writing out every piece either form may carry:

| | slot | sort | λ today | def today |
|---|---|---|---|---|
| 1 | name | 𝒩 | — | `def`'s head |
| 2 | binders: pattern, set, usage mode, passing mode | Bind* | **names only** | `sig`'s pairs |
| 3 | precondition φ, over the parameter *tuple* | Formula | **no place** | `sig`'s `where` |
| 4 | result set U | Expr | **no place** | `sig`'s last element |
| 5 | postcondition ψ, over the parameters and the result | Formula | **no place** | `ensures` |
| 6 | body | Expr | the last element | `def`'s last element |
| 7 | markers: `pure`, `variadic`, `index` | — | — | `sig`'s tail |
| 8 | a host body in place of an expression | 𝒯 | — | `(host …)` |

**Eight slots.** A spelling with no holes has a place for each, and a place that does not read as
bolted on.

### 1.3 The constraint that decides most of it: Curry, not Church

[ADR 0049](decisions/0049-types-are-sets-and-the-logic-never-prints.md) and settypes §1.3 put the types in a separate language:
a type is a **set**, and `e ∈ S` is a **claim proven about** e. That is Curry-style extrinsic typing
(Curry and Feys, *Combinatory Logic*, 1958), not Church-style intrinsic typing, where a term is typed
by construction — `λx:A. e` (Church 1940). Reynolds names the two and shows where they meet ("The
meaning of types: from intrinsic to extrinsic semantics", 2000).

Three consequences, and they are not stylistic:

**(a) A set on a binder is not part of the λ's meaning.** Erasure ε (abstract.md §5) drops every set
and every claim, and `⟦P⟧ = ⟦D(ε(P))⟧`: the logic is inert *by construction*. So whatever slot 4 gets,
**it must be erasable without touching the body**. A spelling that weaves the result set into the body
makes erasure a rewrite instead of a deletion.

**(b) `λx:A. e` is the one spelling ruled out by name.** Any candidate that reads as "this term has
this type" is Church's. The faithful reading is "this term's value is claimed to lie in this set".

**(c) The untyped λ is still whole, and is load-bearing.** Verified at `47eb569`:

```lisp
(def two    (fn (s) (fn (z) (s (s z)))))
(def three  (fn (s) (fn (z) (s (s (s z))))))
(def plus   (fn (m) (fn (n) (fn (s) (fn (z) ((m s) ((n s) z)))))))
(def to-num (fn (c) ((c (fn (k) (+ k 1))) 0)))
(to-num ((plus two) three))     ⟶   5
```

Church numerals reduce to `5` at compile time. **No claim could be written on any λ in that program,
and none is wanted**: the sets those λs range over are sets of *functions*, and V has no functions
(settypes §1.2). So slot 4 on a λ is sometimes meaningless, which argues it is optional, never
required — and argues against any spelling that makes it structural.

### 1.4 Two findings before the candidates

**Finding 1. `def` is a λ with a name. They must be one form.**

Compare slots 1–8: a `def` is a λ plus slot 1. Nothing else differs. abstract.md keeps them as two
operators only because `lam` has no claim slots; give the λ slots 3–5 and the two coincide. So the
real question is not "where does the result go in a λ" but:

> **Is `def` a named λ, or is a λ an anonymous `def`?**

Either answer is a unification; having two forms that differ by a name is not. This is
fit-research decision 3 (merge `sig`, `def`, `where`, `ensures`, `requires`) carried one step
further, to the λ. Every candidate below is judged on whether it makes the two one.

**Finding 2. Every claim position is the same shape: `binder ∈ set`.**

Slots 2, 3, 4 and 5 look like four kinds of thing. They are one kind, used in three positions:

| position | the membership |
|---|---|
| a parameter | `x ∈ S` |
| the parameter tuple, carrying φ | `x̄ ∈ {x̄ ∈ S̄ : φ}` |
| the result, carrying ψ | `r ∈ {r ∈ U : ψ}` |

Because comprehension `{x ∈ S : φ}` is already an owned operator (abstract.md §1.3, `compr`), **φ and
ψ are not slots at all. They are the sets of positions 2 and 3.** The minimum faithful λ is therefore
*two* claim positions, not four:

```
λ  x̄ ∈ P  →  r ∈ R  :  body
```

with P a set of tuples and R a set of values. Per-parameter sets and a `where` are then **sugar that
denotes P**, which is what makes them optional and droppable.

This is the strongest simplicity result available, and it reduces slots 2–5 to `in-set` and
`out-set`. It is also why `r` wants to be a *binder* and not a type annotation: abstract.md §2.6 says
"the binders' names **and r** are bound in φ, ψ and body", so the result already has a name in the
semantics. The user's `fn(args) r ∈ S { body }` had this right — `r` is a binder, in the same shape
as a parameter.

**One open question this exposes, and it decides three candidates below.** Is there a **set former for
functions** — `(-> P R)`, `P → R`, `Π`? abstract.md §1.3's list of owned set names is `int, f64, bool,
string`, ranges, `∪ ∩ ∖`, `set(…)`, `×`, `Seq`, `Map`, `option`, `subsets`. **No arrow**, and §2.6
writes `Π` only in prose, to say what a `def` means.

The question is not whether functions are values — they are not, and neither are sets (settypes §1.2:
"functions are not in V … and so do types"). It is **what level a set of operators sits at**. W9
admits exactly two levels above V, sets of values and sets of sets of values, and a Π-set is neither.

- **If an arrow is added**, a λ's whole claim is one ordinary `the`, slots 3–5 need no λ syntax at
  all (candidate E), and `def`'s membership statement in §2.6 stops being prose and becomes a term
  the language can write.
- **If not**, the λ must section its claims (candidates B, B′), and §2.6's `Π` stays a thing the
  specification says about a `def` rather than a set the program names.

**hamza decided it: yes, there are function set formers, and it is related to callbacks.** §1.8 is
the derivation, and the short version is that the link is not an analogy — a callback declaration is
the *only* place the arrow has a job, and no allowed callback tier makes a λ a value of V, so the
arrow costs W9 a clause rather than a relaxation.

### 1.5 Ten candidates, and a synthesis

The same three programs in each. They are chosen to exercise the holes: a claimed binary operation
with a precondition and a postcondition; an unclaimed higher-order λ (where claims are meaningless);
and a λ applied immediately (where the spelling meets application). Several candidates also show a
host body, a static parameter and a callback parameter, which are where the remaining slots land.

Program 1, `div`: integer division, divisor nonzero, result bounded by the dividend.
Program 2, `compose`, and a Church numeral: no claims anywhere.
Program 3: a λ applied at once to `6` and `3`.

---

#### A — positional prefix, result third (the minimal patch to today)

```lisp
(fn ((a int) (b nonzero)) int
  (/ a b))

(fn (f g) (fn (x) (f (g x))))
(fn (s) (fn (z) (s (s z))))

((fn ((a int) (b nonzero)) int (/ a b)) 6 3)
```

The result is a third positional slot. `def` stays separate, or becomes `(def name (fn …))`.

- **H: 3 holes** — φ, ψ and the markers have no place. Adding them positionally makes slots 3–5
  optional *and* positional, which is ambiguous: in `(fn (x) A B)`, is `A` the result set or the body?
  Only a set/expression distinction would tell, and ADR 0049 says there is none — sets *are*
  expressions. **This is the disqualifying argument, and it is a consequence of ADR 0049, not a
  matter of taste.**
- **U:** 1 new shape. **F:** F3 (λ and def still two forms). **E:** poor — the result set sits between
  the binders and the body with nothing marking it as a claim.

#### B — keyword-sectioned prefix

```lisp
;; 1 — claimed
(fn ((a int) (b int)) -> int
  (where   (/= b 0))
  (ensures (<= (abs r) (abs a)))
  (/ a b))

;; 2 — unclaimed: the sections are simply absent
(fn (f g) (fn (x) (f (g x))))
(fn (s) (fn (z) (s (s z))))

;; 3 — applied at once
((fn ((a int) (b int)) -> int (where (/= b 0)) (/ a b)) 6 3)

;; named: one form, the name written once
(fn div ((a int) (b int)) -> int
  (where (/= b 0))
  (/ a b))

;; a host body is a section too, so slot 8 needs nothing new
(fn Encode ((dst (array byte) linear) (src (array byte))) -> ((n int))
  (pure)
  (host expr "hex.Encode(%s, %s)" (import "encoding/hex")))
```

Claims are labelled, optional, unordered; the body is last. `def` is `fn` with a leading name — the
same operator, so Finding 1 holds.

- **H: 0.** Markers are sections too: `(pure)`, `(variadic)`.
- **U:** 1 shape plus a section vocabulary. **F:** fits, if `def` is a named `fn`. **E:** good — every
  claim is inside a labelled section, so erasure is "delete these sections".
- The cost: `-> int` is a second way to write a membership, next to `(b int)`. Finding 2 says both are
  `binder ∈ set`, so writing them differently hides the identity. Fix: name the result.

#### B′ — B with the result as a named binder (Finding 2 applied)

```lisp
(fn ((a int) (b int)) ((r int))
  (where   (/= b 0))
  (ensures (<= (abs r) (abs a)))
  (/ a b))
```

Two binder groups, **one shape for both**: a list of `(name set)`. The result group is a list because
a host call may return several values (`values`, ADR 0031).

- **H: 0. U: 1 shape, used twice.** **F:** fits. **E:** good.
- This is the Z schema's own shape: declarations, then the predicate.

#### C — the sectioned form with the claims as sets (φ and ψ absorbed)

Finding 2 taken literally: no `where`, no `ensures`, because a comprehension is a set.

```lisp
;; 1 — claimed: φ and ψ are the two sets, and there are no other slots
(fn (((a b) (set ((a int) (b int)) (/= b 0))))
    ((r (set ((r int)) (<= (abs r) (abs a)))))
  (/ a b))

;; 1 again, with no precondition — the general shape still has to be written
(fn (((a b) (× int int))) ((r int))
  (/ a b))

;; 2 — unclaimed: both groups collapse to patterns, and C reads like B′
(fn ((f) (g)) () (fn ((x) ()) (f (g x))))
```

That last line is the tell: with nothing to claim, C must still write the two groups, and the empty
result group `()` is noise in the case that is by far the most common.

- **H: 0.** **U: 1 shape** — everything is `binder ∈ set`. The most uniform candidate there is.
- **F: fits perfectly.** **S: worst** — the common case (no precondition) pays for the general one, and
  the tuple pattern must be written even when there is one parameter.
- **Drop, but keep the reading**: this is the *semantics* of B′, and B′ is its sugar. Having derived
  it, B′'s `where` is no longer an extra construct — it is a comprehension written in a shorthand.

#### D — the mathematical form, result after the binders (hamza's third sketch)

```
\* 1 — claimed
λ(a ∈ Int, b ∈ Int) r ∈ Int
  WHERE   b ≠ 0
  ENSURE  |r| ≤ |a|
  : a ÷ b

\* 2 — unclaimed
λ(f, g) : λ(x) : f(g(x))
λ(s) : λ(z) : s(s(z))

\* 3 — applied at once
(λ(a ∈ Int, b ∈ Int) r ∈ Int WHERE b ≠ 0 : a ÷ b)(6, 3)

\* named — a definition is the same form with a name
div == λ(a ∈ Int, b ∈ NonZero) r ∈ Int : a ÷ b

\* a host body
Encode == λ(dst ∈ Seq(Byte) LINEAR, src ∈ Seq(Byte)) n ∈ Nat
  PURE
  : HOST expr "hex.Encode(%s, %s)" IMPORT "encoding/hex"
```

- **H: 0. U: 1 shape. E: good** — claims are above the `:`, body below, so erasure is "delete
  everything between the result binder and the colon".
- **F:** fits. **S: the best of the nine on this program** — 29 tokens and depth 1, against B′'s 48
  and 4 (§1.6). On syntax-research §4's eight programs the same comparison was 731 tokens against
  920, so the gap here is wider than typical because this program is almost nothing but claims.
- The hazard syntax-research §5.2 names: `λ` extends as far right as it can, so `λ(x) : f(x) + 1`
  needs the reader to know where the body ends. TLA+ lives with it; it is the mathematical notation's
  one real cost, and it can produce a program that parses and means something else.

#### E — the signature as one term, claimed with `the` — **dropped by hamza**

> *"and I think I should drop E, just a matter of taste, push back if you think otherwise."*

**No push-back, and the reason is better than taste: given hamza's answer to §1.9's first question,
dropping E costs nothing.** E's one unique capability was claiming an *already-written* λ from
outside it. The arrow is now decided in (§1.8) — but where callbacks actually need it is **a
binder's set inside a declaration**, which B′, D, F and J all already have a place for. So the arrow
survives E, and nothing is left that only E could say. The derivation is kept below because it is
what established that the arrow is wanted at all.

If an arrow set former exists, slots 3–5 need nothing new:

```lisp
;; 1 — claimed: the λ keeps its two slots; the claim is an ordinary `the`
(the (-> ((a int) (b nonzero)) int)
     (fn (a b) (/ a b)))

;; 1 with a postcondition: the arrow's halves are comprehensions, as in C
(the (-> (set ((a int) (b int)) (/= b 0))
         (set ((r int)) (<= (abs r) (abs a))))
     (fn (a b) (/ a b)))

;; 2 — unclaimed: nothing at all to write, and the λ is today's λ, unchanged
(fn (f g) (fn (x) (f (g x))))
(fn (s) (fn (z) (s (s z))))

;; named: `def` is unchanged too, and the claim is a separate sentence about it
(def div (fn (a b) (/ a b)))
(the (-> ((a int) (b nonzero)) int) div)
```

The last pair is E's real character: it **does not** merge `def` and the claim, so Finding 1 is not
satisfied — but it does not need to be, because the claim is no longer a *form*. It is a sentence,
and sentences already compose.

**Why that is not enough to keep it.** A claim-as-sentence can be written anywhere, including far
from the thing it claims, and nothing in the notation holds the two together. For a `def` that is a
regression: ADR 0028 checks a declared precondition at *every call*, so the declaration and its
contract are read as one thing by the compiler and should be one thing on the page. E makes them two
sentences that happen to share a name — which is `sig`-and-`def` again, the very split §1.1 opened by
calling a hole. **Dropped.**

- **H: 0** — *if* the arrow exists. **U: 0 new shapes** — `the` and `fn` both already exist.
- **F:** the best fit on paper: a claim about a λ is a claim, written the way every other claim is.
- **The blocking question, stated precisely.** The obvious objection — "V has no functions" — is
  *not* the obstacle, because V has no sets either: settypes §1.2 says "functions are not in V … and
  so do types", so every set already lives outside V, at the static level. The real obstacle is
  **W9's stratification** (abstract.md §3): a binder's set is "a set of values or a set of sets of
  values, **never higher**". A set of *operators* is neither of those two levels. So an arrow is not
  one more set former alongside `×` and `Seq`; it is a third level, and W9 would have to say what it
  is. **Cannot be chosen until that is answered.** Kept on the shortlist, because the question is
  small and the prize is a construct removed rather than added.

#### F — ML, F#-inspired (corrected: *inspired*, not copied)

**My first draft dropped this candidate and was wrong to.** It read fit-research §3.1 as an argument
against F#'s *shape*, when §3.1's argument is against F#'s **type grammar** — `'T list`,
`Result<'T,'E>`, `when 'E :> error` — which is a second grammar and so Church-style stratification.
hamza's correction: *"nothing says that F# syntax has to dictate the semantics. rather let's call it,
F# inspired."* That is right. The shape and the type grammar are separable, and separating them is
exactly what this installment is for.

So: the ML shape, with ADR 0049's types-as-terms. Three rules make it fit, and they are the whole of
the correction:

| F# has | F#-inspired has | why |
|---|---|---|
| `'T list`, `Result<'T,'E>` | `Seq(T)`, `Result(T, E)` — ordinary application | ADR 0049: a set expression is an expression |
| `<'E when 'E :> error>` | `(E: subsets(error))` — an ordinary binder | a bound is membership (typevars-research) |
| `x: T` read as "x has type T" | `x: T` read as `x ∈ T` | Curry, not Church (§1.3) |

Only the third is a reading rather than a change, and it is the one that matters: `:` is a spelling
of `∈`, not of "has type".

```fsharp
// 1 — claimed; the result is a named binder, as Finding 2 wants
let div (a: Int) (b: Int) : (r: Int)
    where  b <> 0
    ensure abs r <= abs a
    = a / b

// 2 — unclaimed
let compose f g = fun x -> f (g x)
let two = fun s -> fun z -> s (s z)

// 3 — applied at once
(fun (a: Int) (b: NonZero) : (r: Int) -> a / b) 6 3

// a static parameter is a binder over a set of sets, not an angle bracket
let asType (E: subsets(error)) (err: error) : (r: Option(E))
    = host expr "errors.AsType[%t1](%s)" import "errors"

// a callback parameter's set is an arrow (§1.8)
let sortSlice (a: Seq(Value)) (less: (Int, Int) -> Bool) : Unit
    = host stmt "sort.Slice(%s, %s)" import "sort"
```

- **H: 0. U: 1 shape** once `:` is `∈` and a set is a term. **S: 34 tokens, depth 1** (§1.6), a third
  shorter than B′.
- **F: fits**, with the three substitutions above. Without them it is F4′, which is what my first
  draft measured.
- **The hazard is unchanged and is the reason it is not kept alone**: syntax-research §5.2 — a nested
  `match` swallowing the cases after it, because the offside rule cannot tell where the inner one
  ends. That failure can produce a program that parses, type-checks and means something else, which
  is the kind of error this project trades size to avoid.

#### G — curried, one binder per λ, claims on each (invented)

Since β is substitution and arity is exact today, try the opposite: a λ binds one name, and a claim
rides the binder.

```lisp
(fn (a int) (fn (b nonzero) (the int (/ a b))))
```

- **H: 1** — φ relates *two* parameters, and a curried λ has no place to say so until both are bound.
  `(/= b 0)` is fine; `(= (len a) (len b))` is not, and the corpus has those.
- **U: 1 shape. F:** F4′ — exact arity is the semantics (abstract.md §2.4, `lam(b̄; e)` takes a
  sequence), and currying it writes an isomorphism as if it were the thing. core-0 §0 already records
  that "currying is an isomorphism; the uncurried presentation has no semantic content", and that
  calling it a departure from λ-calculus was **wrong**.
- **Drop.**

#### H — the result on the application, not the λ (invented)

Claims belong where a value is produced, so put the result set at the call:

```lisp
(the int ((fn ((a int) (b nonzero)) (/ a b)) 6 3))
```

- **H: 0** for a direct application; **1** for a λ that is passed, where no call site is visible.
- **F: F5** — it permits what the semantics does not have: the same λ claimed differently at two call
  sites, with no statement about the λ itself. A `def`'s claim is checked at *every* call
  (ADR 0028), which requires one statement in one place.
- **Drop.** But note it is already *legal* and sometimes right: `the` on an application is how a
  caller narrows a result it knows more about than the declaration does.

#### J — Go-inspired: braces delimit the body (hamza's missing candidate)

hamza: *"and I don't see the go inspired syntax: `fn(x ∈ S, y) r ∈ S { e }`. it should be included."*
It was missing and it should not have been — it is the shape his first message proposed, and it has a
precedent worth naming.

**Go already writes the result as a binder.** `func f(a, b int) (r int) { … }` — Go's *named result
parameters* are Finding 2's result-binder, in a shipped language, and `r` is in scope in the body
exactly as abstract.md §2.6 has it. So this candidate is not a borrowed shape with a claim bolted on;
Go's own form is already two binder groups.

```go
// 1 — claimed; claims in the header, body in the braces
fn div(a ∈ Int, b ∈ Int) (r ∈ Int)
  where  b ≠ 0
  ensure |r| ≤ |a|
{
  a ÷ b
}

// 2 — unclaimed, anonymous
fn(f, g) { fn(x) { f(g(x)) } }
fn(s) { fn(z) { s(s(z)) } }

// 3 — applied at once, and the braces make the extent unambiguous
fn(a ∈ Int, b ∈ NonZero) (r ∈ Int) { a ÷ b }(6, 3)

// several results: the result group is a group, so this needs nothing new
fn divmod(a ∈ Int, b ∈ Int) (q ∈ Int, m ∈ 0..b-1)
  where b > 0
{
  (a ÷ b, a % b)
}

// a host body
fn Encode(dst ∈ Seq(Byte) linear, src ∈ Seq(Byte)) (n ∈ Nat) pure
  = host expr "hex.Encode(%s, %s)" import "encoding/hex"

// a callback parameter (§1.8)
fn sortSlice(a ∈ Seq(Value), less ∈ (Int, Int) -> Bool) ()
  = host stmt "sort.Slice(%s, %s)" import "sort"
```

- **H: 0. U: 1 shape**, used for both binder groups. **S: 33 tokens, depth 1** (§1.6) — the second
  shortest, within one token of D.
- **F: fits.** Nothing here is a second grammar: `Seq(Byte)` is an application, `0..b-1` a range
  expression, `(Int, Int) -> Bool` an arrow set (§1.8).
- **E: good** — everything between the result group and the `{` is a claim, so erasure deletes a
  contiguous region and the braces mark where it stops.

**And it fixes D's one dangerous failure.** D's hazard is that `λ` extends as far right as it can, so
`λ(x) : f(x) + 1` needs the reader to know where the body ends, and getting it wrong produces a
program that parses and means something else (syntax-research §5.2). **A brace cannot be
ambiguous about extent.** `fn(x) { f(x) } + 1` reads one way only. That is a real advantage over both
D and F, and it costs two tokens against D.

The cost: a second delimiter pair, and `{` `}` do nothing else in the language, so they are two tokens
whose only job is delimiting a body. Against that, they are the two most-trained delimiters in
existence, which is the LLM-side argument syntax-research §5.1 makes about shape.

#### I — two spellings, one tree: S-expressions to write, mathematics to read

Not a ninth shape but a choice about which of the above is *the source*. syntax-research §6's
synthesis, and Lamport's own: write B′, read D, with the printer a function of the source so the two
cannot disagree.

- **H: 0** — whichever source is chosen. **U:** two notations to *read*, one to *write*.
- **F:** fits by construction, since the rendering is a homomorphism out of Σ.
- **Cost:** a printer, which notation-research §8's translator needs anyway. **Not exclusive with
  B′ or D**: it is the answer to "which of them", if the answer is "both, in their places".

### 1.6 Measured

Counted over program 1 only (the claimed `div`), since that is the program exercising all eight slots.
A token is a word or number, or any single non-space character otherwise. Depth and closing run count
`(`, `[`, `{` and their partners, so D's `|r|` and F's `:` are not brackets. Numbers are from a script
over the fenced blocks above; **my own estimates, written before running it, were wrong in eight cells
of nine**, which is this project's own lesson about unmeasured claims arriving on schedule.

| | candidate | holes | shapes | tokens | depth | closing run | fit |
|---|---|---:|---:|---:|---:|---:|---|
| A | positional prefix | **3** | 1 | 19 | 3 | 2 | F3 |
| B | sectioned prefix | 0 | 2 | 45 | 4 | 3 | fits |
| **B′** | **sectioned, result a binder** | **0** | **1** | 48 | 4 | 3 | **fits** |
| C | claims as sets | 0 | **1** | 61 | 6 | 5 | fits |
| **D** | **mathematical** | **0** | **1** | **29** | **1** | **1** | **fits** |
| E | arrow + `the` | 0 | 0 | 30 | 4 | 3 | fits |
| **F** | **ML, F#-inspired** | **0** | **1** | 34 | **1** | **1** | **fits** |
| G | curried | **1** | 1 | 23 | 4 | 4 | **F4′** |
| H | result at the call | **1** | 1 | 26 | 5 | 2 | **F5** |
| **J** | **Go-inspired, braced** | **0** | **1** | 33 | **1** | **1** | **fits** |

What the numbers say, beyond the counts:

- **The four shortest with zero holes and a clean fit are D (29), J (33), F (34) and E (30)** — and E
  is dropped, so the live three are **D, J, F, within five tokens of each other and all at depth 1.**
  Size does not separate them; their failure modes do (§1.7).
- **B′ costs 48 tokens at depth 4**, half again more than any of those three, which is the price of
  writing claims as nested lists. It buys the S-expression failure mode: an error the reader catches
  before anything is interpreted.
- **C is the most uniform and the longest by half again** (61 tokens, depth 6, a closing run of 5).
  That is the price of writing the general case, and it is why it survives as B′'s *meaning* rather
  than as a surface.
- **A and G are the shortest of all** (19 and 23) and both are disqualified on holes. Size is the
  last criterion for a reason.
- **F's and J's depth of 1 is not a coincidence**: both put the claims in a header at one level and
  the body behind a delimiter, so nothing nests. D reaches depth 1 the same way, with `:` as the
  delimiter — and `:` is the one that cannot bound a body from the right, which is §1.7's whole
  argument.

### 1.7 Kept, and dropped

**The question that separates the live candidates is not size. It is what a wrong guess about the
body's extent costs.** All of D, F and J are ~30 tokens at depth 1. They differ in what delimits the
body on the right:

| | what ends the body | what a mistake produces |
|---|---|---|
| **D** | nothing — `:` opens the body and it runs as far right as it can | a program that **parses and means something else** (`λ(x) : f(x) + 1`) |
| **F** | indentation (the offside rule) | a nested `match` swallowing later cases: parses, type-checks, wrong (syntax-research §5.2) |
| **J** | **`}`** | a reader error, at once |
| **B′** | **`)`** | a reader error, at once |

> **§1.11 revisits this row for D, at hamza's question, and promotes it.** Two grammar rules close
> both halves of D's hazard, and the measured cost is 12 parenthesizations in the whole corpus — so D
> reaches J's and B′'s failure class after all. The table above is D *as written*; read §1.11 before
> treating this as D's verdict.

That table is the installment's real result. **The two candidates whose delimiter is a bracket are the
two whose worst failure cannot become a wrong program**, and this project trades size for exactly that
(CLAUDE.md: a path nothing runs is a path nothing checks; "not refused" is not "proven").

**Kept, three:**

1. **J — Go-inspired, braced.** Zero holes, one binder shape used for both groups, 33 tokens at
   depth 1, and the body's extent is explicit. Go's *named result parameters* are Finding 2 already
   shipped, so the result-as-binder is not an invention. It is D's size with B′'s failure mode, which
   is the combination nothing else in the table reaches.
2. **B′ — sectioned prefix, the result a named binder group.** Zero holes, one shape, the Z schema's
   own form, erasure a deletion of labelled sections, and the S-expression source syntax-research
   §5.4 ranks first for writing. It costs 15 tokens and three levels of depth over J; it buys one
   delimiter pair instead of two and a reader that already exists.
3. **D — the mathematical form.** Shortest, and the clearest to *read* claims in (syntax-research
   §5.3). First kept for reading alone; **§1.11 promotes it to a live source candidate**, because its
   hazard turns out to cost two grammar rules and twelve parentheses.

**And, orthogonally, I — one tree, two renderings.** hamza has not decided whether there are two
notations (§1.9), so this stays a shape the kept three are compatible with rather than a choice.

**F is kept on the bench rather than dropped.** Corrected, it fits, it is the most pleasant to read of
the four, and it is 34 tokens at depth 1. What keeps it off the list is its delimiter: indentation,
whose failure is the one that hides. If a later installment finds a construct where the offside rule
is clearly better, it comes back.

**Dropped, with the reason:**

| | why |
|---|---|
| A | 3 holes, and the fourth slot is *ambiguous* with the body, because ADR 0049 makes sets expressions. Not fixable by adding slots. |
| B | Superseded by B′: one more shape for three fewer tokens. |
| C | Dropped as a surface, **kept as the reading of B′**: its uniformity is B′'s meaning, and deriving it is what makes B′'s `where` a shorthand rather than a construct. |
| E | **hamza's call**, and it costs nothing: the arrow it was testing is decided in (§1.8) and lives in a binder's set, where B′, D, F and J all have a place for it. What is left — a claim as a free-standing sentence — re-splits a declaration from its contract, which is `sig`-and-`def` again. |
| G | 1 hole (a relational precondition has no place) and F4′: currying an isomorphism, which core-0 §0 already records as a wrong reading. |
| H | F5: permits a λ claimed differently per call site, where ADR 0028 needs one statement. Still legal and useful as a caller's narrowing. |

### 1.8 The arrow, and why hamza is right that it is about callbacks

> *"I think we should have function set formers. And I think this is related to callbacks."*

**Decided in, and the link to callbacks is not an analogy — it is where the arrow's only job is.**

#### What a callback declaration needs

[callbacks.md §2](spec/callbacks.md) has three tiers, and a host declaration must say, at the
parameter, what shape of λ goes there:

```lisp
(sort.Slice a (fn (i j) less))      ; tier 1: Go's own closure does the capture
(win.EnumWindows my-callback ctx)   ; tier 2: a top-level name and an explicit context word
```

For `sort.Slice`'s second parameter, the only honest statement is *"a λ of two integers yielding a
boolean"*. There is no other set that says it. **So the arrow's job is a binder's set in a
declaration**, and that is the whole of it for tiers 1 and 2.

#### The finding: the arrow does not put functions in V

This is the part that makes it cheap, and it falls out of the tiers rather than being assumed:

| tier | what the λ is | is it a value of V? |
|---|---|---|
| **1** | written at the call site, in a **structural position** — "the same kind of position a `loop`'s body or a `let`'s continuation sits in, which the backends already read positionally and never treat as a value" (callbacks.md §2) | **no** |
| **2** | a top-level definition **referenced by name**; the host gets a function pointer or a method value | **no** |
| **3** | a manufactured closure that escapes — stored in a table, partially applied | yes, and it is **refused** |

> **No tier that is allowed makes a λ a value of V.** Tier 1's λ never becomes a value in our
> residual; tier 2's is a name. Tier 3 is the one that would need a function value, and it is refused
> (ADR 0018 would have to be revisited with it).

So an arrow is **a set of operators, not a set of values**, and settypes §1.2's "functions are not in
V" stands untouched. The arrow does not smuggle in closures, and **it does not reopen tier 3**: a set
of operators is not a set of values, so a program still cannot store a callback in a table. That is
worth saying plainly, because "we have function types now" is exactly the sentence someone would read
as permission.

#### What it costs W9: a clause, not a relaxation

§1.4 called this "a third level for W9 to admit", which was the right worry and the wrong shape.
W9 reads: *a binder's set is a set of values or a set of sets of values, never higher.* The levels:

```
L0   V                      values
L1   sets of values         int, Seq(Byte), {n ∈ Int : n % 2 = 0}      ← "types"
L2   sets of sets of values subsets(error)                            ← static parameters
LΠ   sets of operators      Π(x̄ ∈ P). R,  P ∈ L1 (or a set of tuples), R ∈ L1
```

**LΠ is not above L2. It is beside it** — a different kind of thing, not a higher one. Its two
arguments are both at L1, so it builds nothing higher than what it is given, and there is no route to
`Type : Type`. W9 therefore gains a clause (*a binder's set may be a Π-set whose domain and codomain
are sets of values*) rather than loosening its bound. That is a smaller change than I first
described, and I should not have called it a third level.

#### And it is not new semantics at all

abstract.md §2.6 **already states a definition's meaning as a Π-membership**:

```
f ∈ Π( x̄ ∈ {x̄ ∈ S̄ : φ} ). { r ∈ U : ψ }
```

That sentence is in the specification today, in prose, as what a `def` means. The arrow **promotes it
from prose to a term**. So the question was never "should the language gain Π-sets" — it has them,
unwritten. It was "may a program write the one the specification already uses", and the answer being
yes removes an asymmetry rather than adding a feature.

Two consequences come free, and both are on CLAUDE.md's *deliberately not next* list, so neither is
to be built now — only noted as no longer needing a separate mechanism:

- **dependent results.** Π means R may mention x̄, so `(n ∈ Nat) -> Seq(Int, n)` is sayable. That is
  "dependent result ranges", currently deferred.
- **`ensures` is Π's codomain.** `{r ∈ U : ψ}` is the codomain, so a postcondition is not a second
  mechanism beside the arrow; it is what the arrow's right-hand side already is (Finding 2, again).

#### Where it may appear, initially

**A binder's set, and nowhere else** — a declaration's parameter (tiers 1 and 2), and a `def`'s own
membership if the surface ever states it. Restricting it to that position is a W6-shaped condition,
costs nothing the callbacks need, and keeps the arrow out of arbitrary expression positions until a
program asks. Spellings, one per kept candidate:

```lisp
(fn sortSlice ((a (array go.Value)) (less (-> ((i int) (j int)) bool))) -> ()   ; B′
  (host stmt "sort.Slice(%s, %s)" (import "sort")))
```
```
sortSlice(a ∈ Seq(Value), less ∈ (Int, Int) → Bool) ()                           \ D
let sortSlice (a: Seq(Value)) (less: (Int, Int) -> Bool) : Unit                  // F
fn sortSlice(a ∈ Seq(Value), less ∈ (Int, Int) -> Bool) ()                       // J
```

**Still open, and for a later installment:** whether the arrow's domain is a tuple set (`(Int, Int) →
Bool`) or curried (`Int → Int → Bool`). §1.5's candidate G is the argument for the tuple form — arity
is exact in Σ, and currying writes an isomorphism as if it were the thing — so the tuple form is the
one these examples use.

### 1.9 Open, and decided

**Decided by hamza, 2026-10-11:**

1. **Function set formers: yes**, and the link to callbacks is real — §1.9. The arrow's job is a
   binder's set in a declaration, no allowed callback tier makes a λ a value of V, and W9 gains a
   clause rather than a relaxation. It is abstract.md §2.6's Π promoted from prose to a term.
2. **Candidate E: dropped.** §1.7 records why it costs nothing.
3. **Design the general form.** hamza, on whether a λ ever carries a claim in practice: *"I don't
   know. we do the general form, and we never do, that is not an issue."* So **Finding 1 stands and
   λ and `def` are one form**, carrying all eight slots, whether or not an anonymous λ ever uses
   slots 3–5. This settles the fourth candidate §1.9 floated (λ keeps two slots, all eight on `def`)
   against itself: it is not proposed.
4. **F is "F#-inspired", not F#** — the shape without the type grammar (§1.5, F). My first draft
   dropped it on an argument that applies to F#'s generics rather than to its shape, and hamza
   corrected it.
5. **The Go-inspired braced form is a candidate** (§1.5, J) and is now one of the three kept.

**Still open:**

1. **Two notations or one** (§1.5, I). hamza: *"have not decided on two languages. by the end we
   will."* So the kept three stay compatible with either answer, and nothing in a later installment
   may assume one.
2. **Of J, B′ and D, which is the source.** The delimiter table in §1.7 is the argument; the
   remaining question is whether S-expressions' already-built reader and LLM-write ranking outweigh
   J's 15 fewer tokens and two fewer levels of nesting.
3. **The arrow's domain: a tuple set or curried** (§1.8's last paragraph). Σ's exact arity argues for
   the tuple form, which is what the examples use. For a later installment.

### 1.10 What installment 1 fixes for everything after it

Now that hamza has decided: Whatever surface is chosen:

1. **λ and `def` are one form**, carrying all eight slots (decision 3). `sig` disappears; a name and
   an arity are written once. Every later binding construct — `let`, `local`, `loop`, a variant's
   parameters, a host declaration — uses the *same* binder shape or uniformity breaks.
2. **A binder is `pattern ∈ set [mode]`**, the set optional, the mode from fit-research decision 2
   (`linear`, `inout`, `out`). "No set" is **not** `∈ V`: it is *no claim*, which settypes §10
   insists is different and must not be merged.
3. **The result is a binder group, not an annotation** — so several results need no new form, and
   Go's named results are the precedent (§1.5, J).
4. **φ and ψ are comprehensions over those groups** (Finding 2), so a `where` anywhere else — on a
   `variant`, a `loop`'s invariant, a `local` — is the same shorthand for the same operator.
5. **Claims are a deletable region**, between the result group and the body's delimiter. No later
   construct may interleave a claim with the terms it is about, or ε stops being a deletion.
6. **No type on a term.** No construct may read as "this term has this type" (§1.3).
7. **A set of operators is a binder's set and nothing else** (§1.8), and it does not reopen tier 3.
8. **The body has a delimiter whose failure is syntactic** (§1.7), if J or B′ wins — which is a
   constraint on every later construct that has a body, not just the λ.

### 1.11 Closing the mathematical notation's hazard

hamza, before moving on: *"what would it take to close the hazard in the mathematical notation?"*

**It takes two different fixes, because it is two hazards.** Separating them is most of the answer,
and measuring each changes the verdict of §1.7.

#### The two halves

| | hazard | the shape |
|---|---|---|
| **H1** | **the dangling body** — a keyword form extends as far right as it can, so an operator after it is swallowed | `IF c THEN a ELSE b + 1`, meaning `(IF c THEN a ELSE b) + 1`; `λ(x) : x + 1` as an operand |
| **H2** | **clause attachment** — which `[]` (or `\|`) belongs to which chain, when one chain is inside another's arm | a nested `CASE` taking the outer `CASE`'s later arms |

H1 is a precedence problem. H2 is not: no precedence rule can say which chain a clause joins, because
both readings are well-formed. **This is F#'s classic pitfall** (syntax-research §5.2), and F#'s own
community answer is to parenthesize a nested `match` — so the fix is already standard practice in the
language D and F are inspired by.

#### What the literature does

| fix | who | closes | cost |
|---|---|---|---|
| **a closing word per form** | Algol 68 `fi` `od` `esac`, Ada `end if`, shell `fi`/`done` | H1 and H2 | one token per form, and a vocabulary of them |
| **a mandatory block** | Rust — `if` is an expression but its arms must be `{ }` | H1 and H2 | this is candidate **J** |
| **layout** | Haskell, F#, Python | H1; **not H2** | the failure hides, which is F's problem |
| **checked layout** | Python's `TabError`; Scala 3's `end` markers, optional but *verified* | H1, and H2 where the marker is written | a layout algorithm, and an error class people meet often |
| **refuse the ambiguous case** | the grammar admits a keyword form only where nothing can continue it | H1 and H2 | parentheses where it bites — **measured below** |
| **a canonical formatter, checked** | `zig fmt --check`, `elm-format` in CI | neither, strictly — it catches rather than prevents | a formatter, which a rendering needs anyway |

> **Structurally there is one fix: a right delimiter.** Give D one and it becomes J, or Algol 68. So
> "close the hazard structurally" and "stop being D" are the same sentence, and the interesting
> question is what the cheapest *non*-structural fix costs.

#### Measured, on the 85 files of `examples/` and `lib/`

The corpus is prefix today, so each measurement asks what its **infix rendering** would need. In an
infix rendering a comma or a closing bracket stops the munch, so an argument of a call is already
safe; what is not safe is an operand of an operator, and only when something follows it.

| | count |
|---|---:|
| keyword forms (`if`, `loop`, `let`, `local`, `case`, `cond`, `fn`, `try`, `expect`, `seq`, `match`, `build*`) | **1,424** |
| of those, an operand of an infix operator | 2 |
| of those two, the **rightmost** operand — where munch takes exactly the right thing | 2 |
| **a non-rightmost operand, where munch misparses — H1's dangerous position** | **0** |
| | |
| clause-chain forms (`case`, `cond`, `loop`, `match`) | **149** |
| **nested inside another one's arm — H2's dangerous position** | **12** (8%) |
| nesting depth 1 / 2 / 3 | 137 / 9 / 3 |

The two H1 near-misses, both harmless, both `if` rightmost:

```lisp
(go.+ (go.* mx 10) (if (= sp 0) ok 0))                  ; examples/json/tokenize.oro
(= (nodes …) (if (= (src i) 125) 5 4))                  ; examples/json/tree.oro
```

Rendered, `mx × 10 + IF sp = 0 THEN ok ELSE 0` — the `if` is last, so munching to the end is the
reading you wanted.

**Three numbers, three measurements, and the first two were of the wrong thing.** I first counted
*any* body that is a keyword or operator form (236 of 408, 58%), then *any* keyword form in an operand
position (2 of 1,424). Neither is the dangerous position. CLAUDE.md's lesson — *a survey's first
number describes the measurer* — landed twice in one sitting.

#### So what it takes

**H1: a grammar rule, and it is free today.** Admit a keyword form as an operand only where nothing
can continue it, and require parentheses otherwise. **0 sites in the corpus**, so the rule is pure
insurance: it costs nothing to obey and converts the failure from a wrong program into a syntax
error.

**H2: a delimiter on a nested chain, and it costs 12 parenthesizations.** Require a clause chain that
sits in another's arm to be delimited — `( … )` in D, or Scala 3's checked `end`. Because the rule
fires only when nested, it touches **12 of 149 chains (8%)** and leaves the other 137 as they are.
No precedence rule can do this job, so this is the irreducible part.

Together: **two grammar rules, 12 sites, and D's failures become syntactic.** That is a smaller bill
than I assumed when §1.7 put D on the reading-only shelf.

#### What this changes

**D is promoted to a live source candidate**, and §1.7's delimiter table gains a row:

| | what ends the body | what a mistake produces |
|---|---|---|
| D, as written | nothing | a program that parses and means something else |
| **D + the two rules** | **a required paren, where and only where it is needed** | **a reader error, at once** |
| F, as written | indentation | a nested `match` swallowing later cases |
| **F + H2's rule** | indentation, plus a required paren when nested | H1 remains: layout still cannot bound a body from the right |
| J | `}` | a reader error, at once |
| B′ | `)` | a reader error, at once |

**And it does not fully rescue F.** H2's rule applies to F unchanged, but H1 does not: the offside
rule's whole mechanism is that a body ends where the indentation says, and there is no operand
position to put a rule on. F would need `end` markers to reach the same class, which is Scala 3's
answer and is a third notation's worth of vocabulary.

#### The caveat that keeps H1's zero honest

**The corpus is prefix, so H1's dangerous shape cannot be written in it.** The zero therefore measures
the *rendering* direction — D as a printer is safe, which is direct support for candidate I — and is
only indirect evidence about D as a source. What it does say is that the shape is **unnatural**: none
of 1,424 keyword forms is placed where a reader would have to know the munch rule, so a writer is
unlikely to reach for it either. And the grammar rule makes the question moot, since a writer who does
reach for it is refused.

**H2's twelve is not an artifact.** Nesting is nesting in any notation, and 12 of 149 is what the
programs actually do.

### Next installment

**`let`, `local` and the three binder kinds** — the construct that most tests constraint 1, because
fit-research §3.3 counts *eight* binder shapes today for three kinds, and §3.6 counts five forms
whose structure is read from the parity of a flat list. It is where uniformity is either won or lost,
and it is the first place the kept candidates will differ for a reason other than delimiters.

---

## Installment 2 — `let` and `local`

Constrained by installment 1's §1.10: one binder shape, the kind on the binder, claims a deletable
region, no type on a term, and a body delimiter whose failure is syntactic.

### 2.1 What the construct must carry

| | slot | `let` today | `local` today |
|---|---|---|---|
| 1 | the binder's **pattern** — a name, `_`, or a tuple pattern | yes | name only |
| 2 | the binder's **kind** — definitional, definitional-linear (a buffer), state (a cell) | definitional, implicit | **inferred from the initializer** (ADR 0047) |
| 3 | the binder's optional **claim** (a set) | **no place** | **no place** |
| 4 | the binder's optional **mode** (`linear` on a buffer) | — | implied |
| 5 | the **value** / initializer | yes | yes |
| 6 | **sequencing** — each value sees the ones before it | yes | yes |
| 7 | the **body** | yes | yes |
| 8 | a **discarded** binder | `seq`, a separate form | `seq` |
| 9 | the scope's **result**, possibly a product taken apart by a tuple pattern (ADR 0031) | — | yes |

### 2.2 The holes, verified

**A binder cannot carry a claim.** Verified at `47eb569`:

```lisp
(def f (n) (let (x int) n x))
;; line 2: let binds a name or `(tuple a b …)`; a pattern that may not match
;;         belongs in `case` (spec/binding.md §5)
```

This is **the same hole installment 1 found on the λ**, in a second construct: a claim has no place on
a binder, so it must go on the value, as `(let x (the int n) …)`. Slot 3 is a hole in both forms.

**The kind is inferred, not stated.** ADR 0047: a table initializer makes a buffer, anything else a
cell. Two consequences, both verified:

```lisp
(local b (array 1 2 3) (b 0))
⟶ (fn () (build 3 (fn (b) …)))          ; a graph in a scope is a BUFFER
(let   t (array 1 2 3) (t 0))
⟶ (fn () 1)                              ; the same graph in a let is a VALUE
```

So **one initializer means two different things depending on the head**, which is a homonym the head
resolves rather than the binder (fit-research's F2). And a cell holding a table is unwritable: the
initializer makes it a buffer, and a two-argument `set` on it is refused — late, by linearity, with a
message that names the wrong cause:

```lisp
(local x (array 1 2 3) (seq (set x (array 4 5 6)) (x 0)))
;; gen: the buffer x1 is read after it has already been handed on.
;;   `(set b i v)` CONSUMES b and returns it …
```

The real error is "that is a cell write, not a buffer store". The diagnostic cannot say so, because
`set` is one word for both.

**`set` is a homonym, measured — and about to be a three-way one.** Over the 85 files of `examples/`
and `lib/`:

| | count |
|---|---:|
| `(set c v)` — a **cell write**, two arguments | **9** |
| `(set b i v)` — a **buffer store**, three arguments | **58** |

One word, two operations, told apart by arity — the discrimination-by-arity that ADR 0024 had to
refuse the datum comment to protect (fit-research §3.6).

**And fit-research decision 4 wants `set` for a third thing**: the set former, `(set 1 2)` for
`{1, 2}`. abstract.md §10 has already noticed and proposed a split — *"in T′, assign is `set!` and
the buffer's store is `store`, because `set` is the set former"* — three operators, three names.

**§2.4's P5 and P6 take a shorter route: both writes are `:=`, with two left-hand sides.** `x := e`
writes a cell and `c[j] := e` stores into a buffer, so the two meanings of `set` become **one
operator**, `set` is free for the set former, and no `store` name is needed. That is
notation-research §4.5's decision, which this installment should have started from.

### 2.3 Three findings

**Finding 1. `let` and `local` have the same arity in Σ.** abstract.md §2.4, verbatim:

```
let(b̄; e)     : (Bind*. Expr) → Expr      definitional binders, sequential
local(b̄; e)   : (Bind*. Expr) → Expr      buffer binders (definitional, linear) and cell binders (state)
```

**Identical.** They differ only in *which binder kinds they admit* — which is information about the
binders, not about the form. So by installment 1's Finding 1 pattern they are one operator, and the
binders say which kind each is.

Measured support, and it is weak rather than strong: **3 sites** write one sequential group as two
nested forms (`let` whose body is a `local`, in `freq.oro`, `ages.oro` and `smooth-java.oro`), and no
site nests the other way or nests a form in itself. So merging flattens 3 of 95 forms. The argument
is the identical arity; the measurement only fails to contradict it.

**Finding 2. One binder shape, three operators — and installment 1's parameter is the degenerate
case.**

```
pattern  [∈ S]  = e        definitional   a let; a buffer when e is a buffer constructor
pattern  [∈ S]  := e       state          a cell
pattern   ∈ S              ranging        a parameter (installment 1), where the claim IS the right side
```

The left of the operator is **exactly installment 1's binder** — a pattern with an optional set. The
operator and right side are what installment 2 adds. So the three binder kinds of abstract.md §2.2
are one shape with three operators, the operator names the kind, and nothing is inferred.

And the parity problem disappears with it: an operator between the pattern and the value makes the
pair explicit, so `let`, `local`, `loop`'s variables and `build-map` stop being read off a flat
list's parity (fit-research §3.6 counts five such forms; **95 binders in `let` and 34 in `local`**
are two of them).

**Finding 3 — ~~the constructor decides buffer from value~~. WRONG, corrected 2026-10-11 on hamza's
question.** I claimed `table` builds a buffer and `array` builds a value, at zero cost. hamza:
*"I thought we have `(array a1 a2 …)` for array literal, and `(table n f)` for array from a rule,
regardless of if it is an immutable array or a buffer."* That is right, and verified:

| | contents given by | in a `let` | in a `local` |
|---|---|---|---|
| `(array e₀ … eₖ₋₁)` | a **graph** | `(let t (array 1 2 3) (t 0))` ⟶ `1` — a value | a buffer |
| `(table n f)`, f a function | a **rule** | `(let t (table 3 (fn (i) (* i i))) (t 2))` ⟶ `4` — a value | a buffer |

**Both constructors work in both positions.** They say *how the contents are given*, and nothing about
mutability; what decides buffer from value is **the head**, `let` or `local`. So the constructor
cannot carry the kind, and Finding 3's "zero cost" was measured on a rule that does not hold.

One asymmetry, and it is a real hole in the constructor rather than in the design:
**`(table n v)` with a literal v — the constant table — works only as a buffer initializer.**

```lisp
(let t (table 3 0) (t 1))     ⟶  (0 1)        ; the reducer applies the literal
                              ;; gen: lowering: an application of 0
```

local.md §1 specifies the literal reading ("a constant is the constant function K v") for a `local`
initializer, and outside a scope nothing applies it. The program is **refused**, at emission, not
silently wrong — but the diagnostic names the wrong cause, as §2.2's cell-write diagnostic does.

**What the correction costs: Finding 3 and Finding 1 are now in tension.** Finding 1 merges `let` and
`local`, so the head stops saying which kind a binder is. Finding 3 was what would have said it
instead. With Finding 3 gone, `b = table(n, 0)` under a merged form is **ambiguous** — an immutable
table, or a buffer? Three ways out:

| | | cost | keeps |
|---|---|---:|---|
| **(a)** | **write the mode**: `b = table(n, 0) linear` | **26 sites** | Findings 1 and 2 |
| (b) | keep two heads, `let` and `local` — Finding 1 gives way | 0 sites | Finding 2 only |
| (c) | a third binder operator for a linear binder | 0 sites | Findings 1 and 2, at a fourth kind |

**I recommend (a)**, and the reason is not the site count:

- **fit-research decision 2 already wants the mode on the binder** (`linear`, `inout`, `out`), and said
  it was "implied on a buffer binder". Implied *by what*? Under ADR 0047, by the initializer. With
  Finding 3 withdrawn and Finding 1 taken, by nothing — so it must be written.
- **Linearity is the thing that is actually checked** (ADR 0018, `occurrences` on the residual).
  Writing it makes the checked property visible at the place the check happens, which is this
  project's own preference for a stated fact over an inferred one.
- (c) is a **misfit**: linearity is a *usage judgement* (settypes §2.5), not a kind of binding. Giving
  it a binder operator would put a mode in the position abstract.md §2.2 reserves for kinds.
- (b) gives up the installment's strongest finding — **identical arity in Σ** — to save 26 words,
  which is choosing the cheaper option against the algebra.

So Finding 3 is withdrawn, and in its place: **the kind is stated by the operator (`=` or `:=`) and
the mode (`linear`) is written where it holds.** The constructors keep hamza's reading — `array` is a
graph, `table` is a rule — and say nothing about mutability.

### 2.4 Nine programs, four notations

**T** today, **B′** sectioned prefix, **D** mathematical, **J** Go-inspired. J is shown in its
strongest form, where **a binder group and its body are one statement list** and the block's value is
its last expression — so slots 7 and 8 collapse into the sequence and `seq` is not a construct.

---

**P1 — a sequential chain of two names.**

```lisp
(let av (os.Args)  n (len av)  (- n 1))                                        ; T
```
```lisp
(let ((av = (os.Args))  (n = (len av)))  (- n 1))                              ; B′
```
```
LET av == os.Args()  n == Len(av)  IN  n - 1                                   \\ D
```
```go
{ av = os.Args();  n = len(av);  n - 1 }                                       // J
```

---

**P2 — a tuple pattern from a host call** (the product's eliminator, binding.md §5).

```lisp
(let (tuple src err) (os.ReadFile p) (if (= err 0) src ""))                     ; T
```
```lisp
(let (((tuple src err) = (os.ReadFile p)))  (if (= err 0) src ""))              ; B′
```
```
LET ⟨src, err⟩ == os.ReadFile(p)  IN  IF err = 0 THEN src ELSE ""               \\ D
```
```go
{ (src, err) = os.ReadFile(p);  if err = 0 { src } else { "" } }                // J
```

---

**P3 — a claim on a binder.** The hole: **T cannot write this at all.**

```lisp
(let b (the (int 0 255) (src i)) …)        ; T — the claim goes on the VALUE, not the binder
```
```lisp
(let (((b (int 0 255)) = (src i)))  …)     ; B′ — the binder's left side is installment 1's
```
```
LET b ∈ 0..255 == src[i]  IN  …                                                 \\ D
```
```go
{ b ∈ 0..255 = src[i];  … }                                                     // J
```

---

**P4 — a discarded binder.**

```lisp
(seq (io.print-line "hi") 0)                                                    ; T — a separate form
```
```lisp
(let ((_ = (io.print-line "hi")))  0)                                           ; B′
```
```
LET _ == io.print_line("hi")  IN  0                                             \\ D
```
```go
{ io.print_line("hi");  0 }                                                     // J — no construct at all
```

J's row is the finding: in a statement list a discarded binder is **an expression statement**, so
`seq` is not a form. ADR 0010 still does the work — the effect may not be dropped — but the notation
needs no name for it.

---

**P5 — a buffer and its stores** (the sieve's inner loop). T threads the buffer by hand; the others
write it in program order, which is §2.4's correction below.

```lisp
(local c (table n false)                                                        ; T
  (loop ((c c) (j (* i i)))
    (< j n)  (again (set c j true) (+ j i))
    else     c))
```
```lisp
(let ((c := (table n false)))                                                   ; B′
  (loop ((j := (* i i)))
    (< j n)  (seq (c j := true) (again (+ j i)))
    else     c))
```
```
LET c := [i ∈ 0..n-1 ↦ FALSE] IN                                                \\ D
  LOOP j := i * i :
      j < n  → c[j] := TRUE ; AGAIN(j + i)
   [] OTHER  → c
```
```go
{ c := [i ∈ 0..n-1 |-> false]                                                   // J
  loop j := i * i {
      j < n -> { c[j] := true; again(j + i) }
      else  -> c
  } }
```

**The store is `c[j] := true`, in program order — corrected 2026-10-11 on hamza's question.** I had
written `store(c, j, true)`, threading the buffer by hand as today's notation does. hamza: *"I thought
indexing in j and D are going to be `c[j]`?"* — and **notation-research §4.5 had already decided it**,
which I wrote against:

> `x := e` writes a cell, and **`t[i] := e` writes a table's element** … **buffers are written in
> program order too.** Today a buffer is threaded by hand, `(again (set s j true) …)`. That form
> exists because the refinement layer's facts induct on a buffer's store chain. **The translation can
> produce that chain itself**, as it produces cells' loop variables, so the programmer writes
> `s[j] := TRUE` and the analyses still see the chain.

So the store is not a *call* at all. It is `:=` with an **indexed** left-hand side, and §2.9's own
measurement is where the gain shows: the loop **loses a variable**. `LOOP c := c, j := i * i` becomes
`LOOP j := i * i`, and `AGAIN(store(c, j, TRUE), j + i)` becomes `c[j] := TRUE ; AGAIN(j + i)`.

---

**P6 — cells and writes** (`examples/io/count.oro`, its inner loop, abridged).

```lisp
(local lines 0  words 0  in-word false                                          ; T
  (loop ((i 0))
    (>= i (len src))  (tuple lines words)
    else (let b (src i)
           (seq (if (= b 10) (set lines (+ lines 1)) (tuple))
                (again (+ i 1))))))
```
```lisp
(let ((lines := 0) (words := 0) (in-word := false))                             ; B′
  (loop ((i := 0))
    (>= i (len src))  (tuple lines words)
    else (let ((b = (src i)))
           (seq (if (= b 10) (lines := (+ lines 1)) (tuple))
                (again (+ i 1))))))
```
```
LET lines := 0  words := 0  in_word := FALSE IN                                 \\ D
  LOOP i := 0 :
      i ≥ Len(src) → ⟨lines, words⟩
   [] OTHER        → LET b == src[i] IN
                       ( IF b = 10 THEN lines := lines + 1 ELSE () ;
                         AGAIN(i + 1) )
```
```go
{ lines := 0;  words := 0;  in_word := false                                    // J
  loop i := 0 {
      i >= len(src) -> (lines, words)
      else -> { b = src[i]
                if b = 10 { lines := lines + 1 }
                again(i + 1) } } }
```

A write is `x := e` — **the same operator that bound the cell**, which is PlusCal's and Ada's
reading: `:=` means "this name takes this value", at the binder and at the write alike. So `set`'s
two-argument form disappears too, and §2.2's 9 cell writes and 58 buffer stores become `:=` and
`store`.

**This diverges from abstract.md §10 deliberately, and P5's correction makes the case stronger.**
That spec proposes `set!` for the write and `store` for the buffer, Scheme's convention — two
operations, two names. With `:=` there is **one operator and two left-hand sides**:

| | | |
|---|---|---|
| `x := e` | a **name** | a cell's write |
| `c[j] := e` | an **indexed** name | a buffer's store |

So `set` does not split into two names; it **becomes `:=` in both cases**, and §2.2's 9 cell writes
and 58 buffer stores are one operator. That is Finding 2's uniformity carried into the write, and it
is what Go, C and Python already read. `store` is not needed as a surface name at all.

---

**P7 — a mixed group**: an immutable binding, a buffer and a cell, in one sequence. **T needs two
nested forms** (the 3 sites of Finding 1).

```lisp
(let n (len xs)                                                                 ; T
  (local out (table n 0)  count 0
    …))
```
```lisp
(let ((n = (len xs))  (out = (table n 0))  (count := 0))  …)                    ; B′
```
```
LET n == Len(xs)  out == table(n, 0)  count := 0  IN  …                         \\ D
```
```go
{ n = len(xs);  out = table(n, 0);  count := 0;  … }                            // J
```

---

**P8 — the scope's product result** (ADR 0031): two cells' final values leave together.

```lisp
(local age 0  name ""  (seq … (tuple age name)))                                ; T
```
```lisp
(let ((age := 0) (name := ""))  (seq … (tuple age name)))                       ; B′
```
```
LET age := 0  name := "" IN  ( … ; ⟨age, name⟩ )                                \\ D
```
```go
{ age := 0;  name := "";  …;  (age, name) }                                     // J
```

---

**P9 — a cell holding a table.** **Not expressible in T** (§2.2); the stated kind makes it writable,
since `:=` says cell whatever the initializer is.

```lisp
;; T — no spelling exists
```
```lisp
(let ((cur := (array 1 2 3)))  (seq (cur := (array 4 5 6)) (cur 0)))            ; B′
```
```
LET cur := ⟨1, 2, 3⟩ IN  ( cur := ⟨4, 5, 6⟩ ; cur[0] )                          \\ D
```
```go
{ cur := array(1, 2, 3);  cur := array(4, 5, 6);  cur[0] }                       // J
```

### 2.5 Measured

Tokens, depth and longest closing run over **P1–P8** (P9 is excluded, since it has no T column).
Counting as §1.6, with each block's trailing column marker stripped.

| notation | tokens | × T | depth | closing run | holes | shapes |
|---|---:|---:|---:|---:|---:|---:|
| T today | 229 | 1.00 | 7 | 6 | **2** | **8** (fit-research §3.3) |
| B′ | 336 | **1.47** | 7 | 6 | 0 | 1 |
| D | 232 | 1.01 | **2** | **2** | 0 | 1 |
| J | 232 | 1.01 | 4 | 4 | 0 | 1 |

**My estimates were wrong again, and this time the correction changes the conclusion.** I had written
that D and J were ~15% shorter than today; they are **the same size** (1.01×). So:

- **The case for J and D is not size. It is depth, and it is the structural simplifications.** Both
  write the same number of tokens as today while dropping from depth 7 to 4 and 2, and while removing
  `seq`, the `let`-around-`local` nesting, the eight binder shapes and the two holes. The notation
  costs nothing and the *structure* is what improves — which is a better result than a size win would
  have been, because it cannot be argued away as formatting.
- **D is the shallowest by half**, 2 against J's 4, and this reverses installment 1, where the two
  tied at depth 1. The cause is that `LET … IN` adds no bracket while `{ … }` adds one per block, and
  P5–P8 all nest a loop inside a scope. On the one measure where J and D differed, D now wins.
- **B′ is 1.47× today's tokens** at the same depth and the same closing run — it pays half again as
  much for no structural gain over the others. The cause is structural, not incidental: a binder is
  `(pattern OP rhs)`, a pattern may itself be a list, and a claim makes the left side a list, so P2
  and P3 carry three opening brackets before the first name.
- **T's two holes** are slot 3 (a claim on a binder, unwritable) and slot 2 (the kind, inferred).
  Its eight shapes are fit-research §3.3's count, which every candidate here reduces to one.

### 2.6 Kept, and dropped

**The installment's result is structural and it is the same in all three candidates.** Findings 1–3
hold whatever the spelling: one binder shape with three operators, `let` and `local` merged, the kind
stated rather than inferred, and `set` split into `store` and `:=`. That is where the two holes and
the eight shapes go, and it costs no tokens (§2.5).

**J gains the most from its own shape, and it is a decision rather than a spelling.** Its statement
list makes the binder group and the body one sequence, which:
- removes `seq` as a construct (P4);
- removes the `let`-around-`local` nesting (P7, 3 sites);
- makes a cell's write the same operator as its binder (P6);
- keeps `}` as the delimiter, so the failure stays syntactic (§1.7).

Four slots simplified by one decision, and none of it is available to a form that keeps binders and
body as separate slots. **But it is not free**: the braces cost two levels of depth against D (4
against 2), because a block is a bracket and these programs nest a loop inside a scope.

**D is the shallowest and holds its ground.** §1.11's two rules apply unchanged — `IN` delimits the
binder group, so `LET` munching rightward is harmless — and it reaches depth 2 where today's notation
is at 7. What it does *not* get is J's collapse of body-and-binders, because `LET … IN e` has one
body slot by construction. **A hybrid is available and worth naming**: `LET … IN` over a
statement-list body, which would take D's shallowness and J's sequence. I have not measured it; it is
the first thing installment 3 should price.

**B′ falls behind.** It was installment 1's co-leader. Here it is 1.47× the tokens at the same depth
and the same closing run as the notation it replaces, and the cause is the shape itself rather than
the examples. Kept on the list, because the reader exists and its failure is syntactic, but it is no
longer a front-runner and installment 3 should be where it is either rescued or dropped.

**Dropped:** nothing new. P9 rules out any candidate that keeps ADR 0047's inference, and no live
candidate does.

### 2.7 What installment 2 fixes for later

1. **One binder shape, three operators** (Finding 2): `pattern [∈ S] = e`, `:= e`, or `∈ S` alone.
   Every later binder — a loop's variables, a `case` arm, a variant's parameters, a comprehension —
   uses it, or uniformity breaks again.
2. **`let` and `local` are one form** (Finding 1). "This scope has state" is read off the binders.
3. **The kind is stated by the operator, never inferred from the initializer.** `=` is
   definitional, `:=` is state, and **`linear` is written** where a definitional binder is a buffer
   (26 sites). The constructors say only how the contents are given — `array` a graph, `table` a
   rule — and nothing about mutability (Finding 3, withdrawn).
4. **One operator, two left-hand sides.** `x := e` writes a cell, `c[j] := e` stores into a buffer,
   and `set` retires — freeing it for the set former (fit-research decision 4). No later construct
   may distinguish two operations by arity. A buffer's threading is **produced by the translation**,
   not written, so the proofs still induct on the store chain (notation-research §4.5).
5. **A block is a statement list whose value is its last expression**, if J wins — and then `seq`
   never returns, and any later construct with a body uses the same block.

### 2.8 Open, for hamza

1. **Does the statement-list block win?** It is the installment's finding, and it is a larger change
   than a spelling: it makes a body a *sequence* rather than a slot, which removes `seq` and one
   nesting level everywhere. Everything after this installment inherits the answer. Note it is
   **separable from the braces** — see 1b.
1b. ~~**The hybrid: `LET … IN` over a statement-list body.**~~ **Priced in §2.9, and the shape was
   wrong**: if binders are statements, a group before them is redundant. The real hybrid is
   statements with no bracket (**K**), and it wins every measure — 0.97× today's tokens, depth 2,
   closing run 1 — while keeping a syntactic failure, because `;` is the loosest operator and so
   terminates a munching form.
2. ~~**`x := table(n, f)`**~~ **— answered in §2.10, and my reason was wrong.** `table(n, f)` is a
   table-valued expression, so `:=` binds a cell holding it and nothing is refused. A cell holding a
   **buffer** is refused too, but *not* because "a buffer is not a value" — a buffer is a value
   (§2.10). It is refused by **linearity**, already checked at every binder, because the
   state-passing translation duplicates the reference. Verified.
3. **Decided by the correction above, and worth confirming**: `linear` is **written** on a buffer
   binder, 26 sites, because nothing else can say it once `let` and `local` merge. The alternatives
   are keeping the two heads (Finding 1 gives way) or a fourth binder operator (a mode in a kind's
   position). My recommendation is to write it.
4. **ADR 0047 needs a successor** either way: its initializer rule is what the stated kind replaces,
   and its translation, cells-as-loop-variables and everything in local.md §3 are unchanged.
5. **`(table n v)` with a literal is a buffer initializer only** — in a `let` it reduces to `(0 1)`
   and is refused at emission as "an application of 0". Either the constructor should accept a
   literal everywhere (the constant table as a value) or the diagnostic should name the real cause.
   A small, separate fix.

### 2.9 Pricing the hybrid

**The hybrid as §2.6 described it was incoherent, and noticing that is the first result.** I wrote
"`LET … IN` over a statement-list body" — but if binders *are* statements, a binder group before the
statements is redundant. There is nothing to put between `LET` and `IN`.

What was actually conflated is **two independent axes**:

| axis | option | option |
|---|---|---|
| **where binders live** | **(a)** in a group before the body | **(b)** interleaved as statements |
| **what ends the body** | **(i)** nothing — a keyword opens it and it munches right | **(ii)** a bracket pair |

D is **(a)(i)**. J is **(b)(ii)**. So the 2×2 has two unpriced corners, and the hybrid worth pricing
is **(b)(i)** — statements, no bracket. Call it **K**. The fourth corner, **(a)(ii)**, a binder group
with a braced body, is **L**, and pricing it completes the square.

#### K — statements separated by `;`, no bracket, no `LET`, no `IN`

```
av == os.Args() ; n == Len(av) ; n - 1                                          \\ P1
⟨src, err⟩ == os.ReadFile(p) ; IF err = 0 THEN src ELSE ""                      \\ P2
b ∈ 0..255 == src[i] ; f(b)                                                     \\ P3
io.print_line("hi") ; 0                                                         \\ P4

lines := 0 ; words := 0 ; in_word := FALSE ;                                    \\ P6
LOOP i := 0 :
    i ≥ Len(src) → ⟨lines, words⟩
 [] OTHER        → b == src[i] ;
                   IF b = 10 THEN lines := lines + 1 ELSE () ;
                   AGAIN(i + 1)

c := [i ∈ 0..n-1 ↦ FALSE] ;                                                     \\ P5
LOOP j := i * i :
    j < n  → c[j] := TRUE ; AGAIN(j + i)
 [] OTHER  → c

n == Len(xs) ; out := [i ∈ 0..n-1 ↦ 0] ; count := 0 ; g(out, count)             \\ P7
age := 0 ; name := "" ; h(age) ; ⟨age, name⟩                                     \\ P8
```

K is **D's lexicon with `;` promoted**, not a new notation: same `==`, `:=`, `∈`, `LOOP`, `⟨⟩`, same
ASCII spellings. The only change is that a binder may appear mid-sequence, so `LET` and `IN`
disappear.

#### L — a binder group, then a braced body

```
let av = os.Args(), n = len(av) { n - 1 }                                       // P1
let lines := 0, words := 0, in_word := false {                                  // P6
  loop i := 0 {
      i >= len(src) -> (lines, words)
      else -> let b = src[i] {
                if b = 10 { lines := lines + 1 }
                again(i + 1) } } }
```

#### Measured, P1–P8, counting as §1.6

| | corner | tokens | × T | depth | closing run | lines |
|---|---|---:|---:|---:|---:|---:|
| T | today | 239 | 1.00 | 7 | 6 | 18 |
| B′ | prefix: group + parens | 312 | 1.31 | 7 | 6 | 16 |
| D | **(a)(i)** group + munch | 243 | 1.02 | **2** | 2 | 16 |
| J | **(b)(ii)** statements + braces | 243 | 1.02 | 4 | 4 | 17 |
| **K** | **(b)(i) statements + munch** | **233** | **0.97** | **2** | **1** | 16 |
| L | **(a)(ii)** group + braces | 248 | 1.04 | 4 | 4 | 17 |

**The square separates cleanly, and the two axes drive different measures:**

- **the body delimiter drives depth**: munch → 2, braces → 4. A block is a bracket, and P5–P8 all
  nest a loop inside a scope, so every block costs a level;
- **the binder placement drives tokens**: statements → fewer, because there is no `LET`/`IN` or
  `let`/`{` keyword pair per group, and a mid-sequence binder needs no nested form.

So **K takes the better option on both axes and L takes the worse on both** — L is the only corner
*above* today's token count while also being the deepest, which is the signature of paying for a
keyword pair and a bracket and getting neither's benefit.

**K is the only one of the six below today's tokens** (0.97×), and it has the shortest closing run of
any notation measured in this document (1).

#### K's failure stays syntactic — which was the objection to (i)

§1.7's argument against a munching body is that a mistake can produce a program that parses and
means something else. **K escapes it, and not by luck:** `;` is the **loosest** operator in
notation-research §4.9's precedence table (level 1), so **`;` terminates a munching keyword form.**
In K the ordinary case is bounded by a separator rather than a bracket, and what is left is exactly
what §1.11 priced:

| | closed by | cost |
|---|---|---:|
| a statement's extent | `;`, looser than everything | free |
| H1 — a keyword form as an operand of an infix operator | §1.11's grammar rule | **0 sites** |
| H2 — a nested clause chain | §1.11's required delimiter when nested | **12 sites** |

So K sits in the same failure class as J and B′ — a mistake is a reader error — **without a bracket**.

**K's one identified new cost**: `;` is looser than `,`, so a sequence in an *argument* position needs
parentheses, `f((a ; b))`. Measured over 4,649 name-headed forms: **0 sites**, and a crude grep for
`(f (seq …`, `(f (let …` and `(f (local …` agrees. Free today.

#### What K buys that D cannot

A binder **after** an effect, in one sequence, with no keyword pair and no parentheses. Compare P6's
arm:

```
D:  [] OTHER → LET b == src[i] IN ( IF b = 10 THEN lines := lines + 1 ELSE () ; AGAIN(i + 1) )
K:  [] OTHER → b == src[i] ; IF b = 10 THEN lines := lines + 1 ELSE () ; AGAIN(i + 1)
```

Measured: **2 sites** in the corpus bind after an effect in one sequence (`examples/u128/factorials.oro`
and `lib/win/fmt.oro`, found as a `let` past the first element of a `seq`, 2 of 44 `seq` forms). So the
extra generality is used, twice, and today it is paid for with a nested form.

#### Three honest caveats

1. **K has no `let` keyword at all.** A reader meets `x == e ;` and must know that `==` binds. D's
   `LET` and J's `let` are a word that says "a binding happens here"; K has only the operator. That
   is the price of the uniformity, and it is a legibility judgement rather than a measurement — it is
   hamza's to make.
2. **Eight programs.** These numbers describe P1–P8. They are the corpus's own shapes, but they are
   eight.
3. **K's depth of 2 is partly the examples.** A program with deeply nested data would raise every
   column together; what the measurement isolates is the *binder and body* contribution, which is
   what the installment is about.

#### What it changes

**K becomes the front-runner for installment 2**, and it amends installment 1's constraint 8. That
constraint read "the body has a delimiter whose failure is syntactic". K satisfies it with a
**separator and two grammar rules** instead of a bracket, so the constraint should read: *a body's
extent is bounded by something whose violation is a syntax error* — which a `;` at the loosest
precedence provides, and which an offside rule does not.

**And it does not settle the λ.** Installment 1's J and D differ in how a λ's *claims* are delimited,
which is a separate question from how a body ends. A surface may take K's sequence for bodies and
either J's braces or D's `:` for the λ's header. That pairing is what the decisions should settle.

### 2.10 Does the merge keep the freeze? And what a buffer is

hamza: *"let and local are scopes, in the case of local it is in the language so we can do
destructive operations. anything that leaves that scope will freeze and becomes immutable. if we
merge them together, will that still hold? what is the effect on the compiler? the checker? the
proofs? and you say buffer is not a value? what do you mean by that exactly? set right now consumes
a buffer and returns it back, a host function can borrow it and return it, and when it leaves local
scope it freezes as a table which is a value."*

#### First: "a buffer is not a value" was wrong, and fit-research already said so

hamza's list is correct in every item. A buffer is passed to `set` and returned by it, bound by a
`let`, declared as a `(buffer V)` parameter (ADR 0020), borrowed and handed back by a host call
(`B ⊸ B ⊗ R`), and frozen into a table on the way out. **It is a value.** I had cited the right
statement in installment 1 and then contradicted it — fit-research §3.2:

> `(buffer V)` and `(array V)` **have the same values**. A frozen buffer *is* a table, and what
> differs is **how the binder may be used**. Writing it as a set says that some values are buffers,
> which is false.

So the correct sentence is: **a buffer and a table have the same values; `linear` is how a binder may
be used, not what it holds.** "Buffer" is a *usage mode over the table sort*, not a sort of its own.

That is also **why the freeze costs nothing**. It is not a conversion — there is nothing to convert,
since the values are the same. The freeze is the **retraction of a permission**: after `build`'s
boundary no binder may store, so what was a linear binder is now an ordinary table. `emit/linearity.go`
says it exactly: *"That is what makes the freeze at `build`'s boundary copy nothing: linearity
guarantees nothing else holds the buffer."*

And it means **my reason for refusing a cell holding a buffer was wrong too.** It is not a sort error.
What actually happens is that the state-passing translation threads a cell's value into every tuple on
every path, duplicating the reference, and **the existing linearity check catches it**. Verified at
`0232c78` — all three are refused, with one message:

```lisp
(local b (table 3 0) (local x b (seq (set b 0 1) (x 0))))        ; a cell from a buffer
(local b (table 3 0) (let c b (seq (set c 0 1) (c 0))))          ; a buffer through a let
(local b (table 3 0) (let c (set b 0 1) (seq (set c 1 2) (b 0)))); used after its store
;; gen: the buffer b is read after it has already been handed on.
```

So the rule is **linearity, checked at every binder** — not a special case about what a cell may hold.

#### Does the freeze still hold under a merge? Yes, and the compiler is already built for it

The decisive fact is in `emit/linearity.go`'s own header, and it is not a change the merge would need
— it is already there:

> **A BUFFER IS LINEAR WHEREVER IT IS BOUND**, and the walk is seeded by **TYPE** (host-buffers.md
> §5). It used to be seeded by exactly two binders — `build`'s and a declared `(buffer V)`
> parameter's… **Every binder is now asked whether the value it binds is a buffer** — a `let`'s, a
> loop variable's, a host call's continuation parameter — and checked if so.

So the check **already treats a `let`'s binder as a possible buffer binder.** And the narrower
seeding was a *bug*, with the same shape as the one the merge would have risked: a write-borrowing
host call returns a buffer, and *"the buffer it returns was never checked, so handing it to a second
call and then reading it was accepted and observed the second write through a dead name."* The
generalisation the merge wants was forced by host write-borrows and is in the compiler today.

**Where the freeze is: at `build`'s boundary — and `build` is the core form both surfaces produce.**
`core/scope.go`'s `readLocal` desugars

```
b (table n f)        ⟶  (build n (fn (b) (let b FILL …)))
b (array e₀ … eₖ₋₁)  ⟶  (build k (fn (b) …))
c e                  ⟶  (#cell e (fn (c) …))        translated away by the loader
```

**The head `local` was never what delimited the scope. The binder was.** A merged form produces the
same `build` from the same binder, so the boundary, the freeze and ADR 0031's product result are
untouched.

#### The effect, part by part

| | effect of merging | why |
|---|---|---|
| **the reader** | **one function instead of two** (`readLet` + `readLocal`), and it needs a marker to know which binders are `build`s — see below | the only place that changes |
| **the compiler below the reader** | **none** | grepped: only `core/read.go`, `core/scope.go` and one test mention `local` or `LocalName`. Nothing in `emit/` or `ir/` knows the word. Buffers are `build`; cells are `#cell`, translated to loop variables before reduction |
| **the checker** | **none** | it runs on the residual, where there is no `local` — only `build` and applications. `(buffer V)` never enters a signature, which is ADR 0018's affordability argument |
| **the proofs** | **none** | the store-chain induction keys off the *term shape* — `(set b i v)` hands back `b` — not off a scope head. The refinement layer's facts are about `build` and `set` |
| **the IR, the backends, emission** | **none** | they never saw `local`; a cell already costs exactly a loop variable |

#### The one thing the merge actually costs

**The reader must still decide which binders become `build`s, and the initializer no longer tells
it.** Today `readLocal` decides from the initializer's form. With §2.3's correction — `array` is a
graph, `table` is a rule, neither says anything about mutability — a merged form has nothing to read
the kind from. So the marker must be written:

```
b = table(n, 0) linear      a buffer: this binder may store, and freezes at the boundary
t = table(n, 0)             an immutable table
```

**That is the whole bill: the word `linear` at 26 binders.** Everything else — the freeze, the
linearity walk, the cell translation, the proofs, the IR — is already indifferent to which head was
written.

#### What is not verified

These are claims about where the code keys off what, read from the source and spot-checked by the
three refusals above. **No merged reader has been built or run**, so `cmd/check` has not been through
it. What would confirm it is the smallest possible experiment: teach `readLet` to accept `linear` and
route those binders through `localBinder`, then check that emission is byte-identical on the 26 sites.
That is a few hours and it is the right first build step if the merge is taken.

### 2.11 Scattered binders: does a statement list break anything?

hamza: *"in the j syntax, it reads like an imperative language, the question is: because we are not
binding names in a head, a programmer can sprinkle them around the scope, will that break anything?
could that be a problem?"*

#### The structural answer: a group does not exist below the reader

A grouped `let` and nested `let`s **reduce to the same term**. Verified at `8d737af`:

```lisp
(let x (f 1) y (g 2) (h x y))            ⟶  (h (f 1) (g 2))
(let x (f 1) (let y (g 2) (h x y)))      ⟶  (h (f 1) (g 2))
```

`readLet` and `readLocal` both **right-fold** into a chain of single-binder forms
(`core/scope.go`: `for i := len(params) - 1; i >= 0; i--`). So "grouped" and "scattered" are one
term, and the group is a surface convenience with **no image in the core**. Nothing in the reducer,
the checker, the refinement layer, the IR or a backend can tell the difference, because none of them
ever saw a group.

**And rebinding a name in one group is already legal:**

```lisp
(let x 1  x (+ x 1)  x)          ⟶  2
(let x 1  x (+ x 1)  x (* x 10)  x)   ⟶  20
```

So shadowing-by-rebind is not something the flat form introduces either. It is what a sequential
`let` has always meant.

#### The three rules that could have been at risk — all three already hold positionally

**1. A binder may not hide a cell.** Refused today, by name:

```lisp
(local total 0 (seq (set total 1) (let total 9 total)))
;; f: a binder named total hides the cell total in its scope; rename one (spec/local.md §3)
```

A flat list makes this *easier to write by accident*, since there is no `local`/`let` boundary to
remind you — but it is caught, and the message names the repair.

**2. The tail rule — and the flat form makes it easier, not harder.** `again` must be the last
statement, and a non-last `again` is refused today:

```lisp
(seq (again (+ i 1)) (io.print-int i))
;; `again` may be a clause body, or sit under a `let`, but not under an `if`
;;  or inside an expression — write another clause instead
```

In a statement list **the tail is syntactically the last element**. Compare CLAUDE.md's standing
warning: *"every walker of a clause chain walks four forms — `again`, `if`, `let`, and a host call's
continuation — and one that enumerates back edges and misses a form is unsound, not imprecise."* A
sequence has one form to walk. And a binder between an effect and an `again` is legal and free —
verified, it reduces to the identical term as without it:

```lisp
(seq (io.print-int i) (let j (+ i 1) (again j)))
⟶  (let (io.print-int i) (fn (#_) (again (+ i 1))))     the same as without the binder
```

**3. Linearity — also easier.** `emit/linearity.go`: *"it walks in EVALUATION ORDER: a `let`'s value
before its body, an `if`'s condition before its branches, arguments before the operation that
consumes them."* **A statement list *is* evaluation order**, so the reading order and the checking
order coincide. A reader who follows the page follows the check.

**And hygiene covers rebinding.** ADR 0036 renames binders apart, so a rebound name in a flat list is
handled by the mechanism that already handles nested shadowing:

```lisp
(let x (shout 1)  x (shout x)  (h x x))
⟶  (let (shout 1) (fn (x) (let (shout x) (fn (x1) (h x1 x1)))))
```

The second binder came back `x1`, so the refinement layer's name-keyed facts stay sound — which is
the bug CLAUDE.md records as *"the facts about a parameter `n` were read as facts about a loop
variable `n`"*.

#### So what is the problem? Two things, and neither is correctness

**(a) The real hazard is `=` against `:=`, and it is a spelling decision.** In a flat list both read
as assignment to anyone arriving from Python or Go:

```
x = 1 ;  x = x + 1          two immutable binders — the second SHADOWS the first
x := 1 ; x := x + 1         one cell, written twice — MUTATION
```

They differ by **one character** and mean binding against mutation. A head group is less exposed,
because the group reads as a declaration list; a flat list puts them adjacent and makes them look
alike.

> **This is a reason to prefer `==` over `=` for the definitional binder.** `==` against `:=` is two
> characters apart and neither reads as C's assignment. **K already uses `==`** (it is D's lexicon),
> which is why K is less exposed here than J, which uses `=`. If a braced surface is chosen, it
> should take `==` too.

**(b) The legibility trade is real, and it is hamza's call.** With a group you see every name a scope
introduces at a glance. Scattered, you do not — you scan forward to the block's end to know where a
name dies. Against that: you bind where the value is computed, next to its use, and the **2 measured
sites** that bind after an effect (§2.9) stop needing a nested form.

#### A mitigation that costs nothing

Because the core has no group, **this is entirely a formatting question**, and it can be answered
without touching the grammar:

- a **formatter** may print a scattered sequence with its leading binders aligned, so a scope that
  *is* written as a declaration list still reads as one;
- a **lint** could require that binders with no preceding effect come first — a convention, reversible,
  and not a rule of the language.

Both are available precisely because grouping has no semantics. That is the installment's answer to
the question: **scattering cannot break the compiler, because the compiler was never told about
groups. What it can do is make two operators that differ by one character sit next to each other,
and the fix for that is the spelling, not the shape.**

### 2.12 What the program-order store changes, and one claim it does not support

§2.4's P5 correction is not only a spelling. notation-research §4.5 names two consequences, and a
third claim that needs one more rule than it states.

#### Measured: the loop loses a variable

On the sieve's inner loop, tokens barely move — the gain is structural:

| | threaded by hand | `c[j] := TRUE` | |
|---|---:|---:|---|
| D | 48 | 46 | depth 2 → **1** |
| K | 47 | 45 | depth 2 → **1** |
| J | 48 | 51 | depth 4 → 4 (the arm needs its own braces) |

What the counts miss: **`c` stops being a loop variable.**

```
LOOP c := c, j := i * i :  …  AGAIN(store(c, j, TRUE), j + i)     threaded
LOOP j := i * i         :  …  c[j] := TRUE ; AGAIN(j + i)         program order
```

One fewer loop variable, one fewer argument at every back edge. That is the same kind of win as
`seq` disappearing (§2.6): a slot removed, not characters saved.

#### Consequence 1: a host write-borrow becomes a plain write

notation-research §4.5: *"`hex.Encode(buf, src)` writes `buf` and returns its count. The template no
longer returns the buffer it borrowed: today's `func(dst, src []byte) ([]byte, int) { return dst,
hex.Encode(dst, src) }(%s, %s)` becomes `hex.Encode(%s, %s)`. That is ADR 0048's stage B, for
buffers."*

So the surface change pays for itself in the **emitted code**: the wrapper that exists only to hand
the buffer back goes away. That is a measurable claim and it is not measured yet — it is the first
thing to check if this is taken, because it touches every write-borrowing declaration
(`encoding/hex` 8, `strconv` 10, `io` 8, `unicode/utf8` 4, `encoding/binary` 4, `strings` 4,
`fmt` 3, `os` 2).

#### Consequence 2: "linearity becomes structural" — true, with one rule it does not state

notation-research §4.5 claims *"a buffer is reached only through its scope's name, so no alias can be
written."* **The alias is still writable as stated**, because a definitional binder can copy the
name:

```
d == c ;  d[j] := TRUE ;  c[k]            d rebinds, c still names the old version
```

So the claim needs an extra condition, and it is a syntactic one:

> **A buffer may be named only by its binder** — a scope's, or a `(buffer V)` parameter's. It may not
> be bound to a second name.

With that, no term holds an old version, and linearity through a body **is** structural: the aliasing
the check exists to catch becomes unwritable. That is fit-research's F5 fix — a checked invariant
becomes grammar — applied to ADR 0018.

#### And it is not structural at an exported boundary

**Two of our own definitions take a `(buffer V)` parameter**, and both are exports, which is ADR 0020's
measured point:

| | |
|---|---|
| `examples/kara/workspace.oro` | `mac-into`, a caller-supplied Karatsuba workspace |
| `examples/native/smooth-go.oro` | `smooth-into`, a caller-supplied destination |

There the buffer arrives as a **parameter**, not through a scope's name. ADR 0020 already splits the
two obligations and that split survives unchanged:

- **uniqueness in** — no other reference exists — is the *caller's* promise, and at an export the
  caller is outside the program, so it stays **assumed**;
- **linearity through the body** becomes structural, by the rule above.

So the honest form of notation-research's claim is: **linearity through a body becomes structural
given "a buffer may be named only by its binder"; the caller's uniqueness promise at an export stays
an assumption, as ADR 0020 has it.** The check `emit/linearity.go` performs would shrink to the
syntactic condition plus that assumption — which is a real simplification of the 1,000-odd lines that
walk evaluation order, and a claim that should be verified by building it rather than by argument.

#### What this does to §2.10's answer

Nothing, and that is worth saying. §2.10's result was that the *freeze* is at `build`'s boundary and
that the merge costs only the word `linear`. The program-order store changes **how a store is
written**, not where the scope ends or what freezing is. The freeze is still the retraction of a
permission at `build`'s boundary; the chain the proofs induct on is still a chain, produced by the
translation instead of typed by the programmer.
