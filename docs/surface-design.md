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
`{1, 2}`. abstract.md §10 has already noticed and proposed the split — *"in T′, assign is `set!` and
the buffer's store is `store`, because `set` is the set former: one spelling per operator, Scheme's
convention for a write."* So the fix is already specified; installment 2 only has to pick the write's
spelling, and §2.4's P6 differs from abstract.md there on purpose (`:=` rather than `set!`, for the
reason given at P6).

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

**Finding 3. If the kind is stated, the *constructor* decides buffer from value — and that costs
nothing.** Replace ADR 0047's "a table initializer makes a buffer" with the sharper rule:

| | means | sites today |
|---|---|---:|
| `b = table(n, f)` | a **buffer**: linear, threaded stores | **26** |
| `t = array(e…)` | an **immutable table**, a value | 0 in a scope, 0 in a `let` |
| `x := e` | a **cell** | **8** |
| `x := array(e…)` | a cell holding a table — **not expressible today** | — |
| `x := table(n, f)` | **refused**: a buffer is not a value, so it cannot be a cell's content | — |

Measured: **all 26 buffer binders use `table`, none uses `array`**, and **no `let` binder anywhere
holds an `array` or `table` constructor**. So sharpening the rule from "a table" to "the `table`
constructor" changes **zero sites**, removes an inference, and gains two forms the language cannot
currently write.

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

**P5 — a buffer, stores threaded through a loop** (the sieve's inner loop).

```lisp
(local c (table n false)                                                        ; T
  (loop ((c c) (j (* i i)))
    (< j n)  (again (set c j true) (+ j i))
    else     c))
```
```lisp
(let ((c = (table n false)))                                                    ; B′
  (loop ((c := c) (j := (* i i)))
    (< j n)  (again (store c j true) (+ j i))
    else     c))
```
```
LET c == table(n, FALSE) IN                                                     \\ D
  LOOP c := c, j := i * i :
      j < n  → AGAIN(store(c, j, TRUE), j + i)
   [] OTHER  → c
```
```go
{ c = table(n, false)                                                           // J
  loop c := c, j := i * i {
      j < n -> again(store(c, j, true), j + i)
      else  -> c
  } }
```

The store is spelled **`store`**, not `set`: §2.2's homonym is resolved by giving the two operations
two names. `store` is abstract.md §1.3's own name for it ("a buffer's threaded store `store` (today's
three-argument `set`)"), so this is the specification's spelling, not an invention.

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

**This diverges from abstract.md §10 deliberately.** That spec proposes `set!` for the write,
Scheme's convention, which solves the homonym equally well. The case for `:=` is Finding 2: if the
binder is `x := e`, then spelling the write `x := e` makes **one operator serve the binder and every
later write**, so a reader learns one thing instead of two and there is no question of which name is
a cell. `set!` would keep them two operations with two spellings. Both close §2.2's collision; only
`:=` also collapses a pair.

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

**P9 — a cell holding a table.** **Not expressible in T** (§2.2); Finding 3 makes it writable.

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
3. **The kind is stated, never inferred** (Finding 3), and the *constructor* separates a buffer from
   a table value: `table` builds a buffer, `array` builds a value.
4. **One operation, one name.** `store` for a buffer, `:=` for a cell, and `set` retires. No later
   construct may distinguish two operations by arity.
5. **A block is a statement list whose value is its last expression**, if J wins — and then `seq`
   never returns, and any later construct with a body uses the same block.

### 2.8 Open, for hamza

1. **Does the statement-list block win?** It is the installment's finding, and it is a larger change
   than a spelling: it makes a body a *sequence* rather than a slot, which removes `seq` and one
   nesting level everywhere. Everything after this installment inherits the answer. Note it is
   **separable from the braces** — see 1b.
1b. **The hybrid: `LET … IN` over a statement-list body.** D is now the shallowest by half (depth 2
   against J's 4, §2.5) and J has the sequence. Nothing stops a candidate having both, and it would
   dominate each on its own measure. Unpriced; it is the first thing installment 3 should measure.
2. **`x := table(n, f)` — refused, or a cell holding a buffer?** I propose refused: a buffer is not a
   value (ADR 0018), so it cannot be a cell's content. Worth a word, because it is the one case
   Finding 3 forbids rather than gains.
3. **Does the `=`/`:=` split want `linear` written on a buffer binder**, or does `table` imply it?
   Finding 3 reads `table` as implying it, which is why the 26 sites cost nothing. Writing it would
   make the mode explicit at 26 sites and make `linear` a thing a reader sees.
4. **ADR 0047 needs a successor** if Finding 3 is taken: its initializer rule is the thing being
   replaced, and its translation, cells-as-loop-variables and everything in local.md §3 is unchanged.
