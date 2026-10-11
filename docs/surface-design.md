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

**This is prior to the spelling, and it is a question for hamza.**

### 1.5 The nine candidates

The same three programs in each. They are chosen to exercise the holes: a claimed binary operation
with a precondition and a postcondition; an unclaimed higher-order λ (where claims are meaningless);
and a λ applied immediately (where the spelling meets application).

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

#### E — the signature as one term, claimed with `the` (no λ syntax at all)

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

#### F — F#'s shape

```fsharp
let div (a: Int) (b: Int) : Int =
    requires (b <> 0)
    ensures  (fun r -> abs r <= abs a)
    a / b

let compose f g = fun x -> f (g x)
```

- **H: 0.** **S:** 736 tokens, close to M's. **Pleasantness: highest**, per syntax-research §5.4.
- **F: F4′, and it is fatal.** fit-research §3.1: F#'s types are a *second grammar* (`'T list`,
  `Result<'T,'E>`, `when 'E :> error`), which is Church-style stratification. ADR 0049 says a type is
  an ordinary term. F\* fixes this; plain F# cannot, and taking F\*'s refinements without its
  term-level types is taking the shape without the thing that makes it fit.
- **Drop**, with the note that the *claims* half (`requires`/`ensures` as labelled clauses) is what
  B/B′ borrow.

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
| E | arrow + `the` | 0* | **0** | 30 | 4 | 3 | fits* |
| F | F# | 0 | 3 | 38 | **1** | **1** | **F4′** |
| G | curried | **1** | 1 | 23 | 4 | 4 | **F4′** |
| H | result at the call | **1** | 1 | 26 | 5 | 2 | **F5** |

\* E's zero holes and perfect fit are conditional on the arrow existing (§1.4's open question).

What the numbers say, beyond the counts:

- **D is 40% shorter than B′ and nests a quarter as deep** (29 tokens against 48; depth 1 against 4).
  That is syntax-research §4's ~20% gap, wider here, because this program is nothing but claims and
  claims are where infix wins most.
- **C is the most uniform and the longest by half again** (61 tokens, depth 6, a closing run of 5).
  That is the price of writing the general case, and it is why it survives as B′'s *meaning* rather
  than as a surface.
- **B′ costs three tokens over B** (48 against 45) and removes a shape. The cheapest trade in the
  table.
- **The two shallowest are D and F, at depth 1**, and F is the one dropped for fit. Shallowness is
  not what decides this.
- **A and G are the shortest prefix candidates** (19 and 23) and both are disqualified on holes. Size
  is the last criterion for a reason.

### 1.7 Kept, and dropped

**The three kept are not three points on one axis.** They are two answers to Finding 1 and one
refusal of it, which is worth seeing before the list:

| | what it does with "λ and `def` differ by a name" |
|---|---|
| B′, D | **satisfies it**: one form, with a name slot. The claims become part of the form. |
| E | **dissolves it**: the λ keeps two slots, `def` keeps two, and a claim is a *sentence about* a name rather than a part of it. Finding 1 then has nothing to merge. |

E is the only candidate where the answer to "where does the result set go in a λ" is **nowhere, and
that is correct** — which is the most Curry-style answer available, and the one ADR 0049 would
predict. It is also the one blocked on W9.

**Kept, three:**

1. **B′ — sectioned prefix, the result a named binder group.** Zero holes, one shape used twice,
   erasure is a deletion of labelled sections, and it is the Z schema's own shape. The S-expression
   source syntax-research §5.4 ranks first for writing, and the failure mode is a parenthesis the
   reader catches.
2. **D — the mathematical form, `λ(args) r ∈ R : body` with `WHERE`/`ENSURE` clauses.** Zero holes,
   shortest, shallowest, and the clearest for claims (syntax-research §5.3). Its hazard is real and
   is the reason it is not kept alone.
3. **E — the arrow set former with `the`.** Zero new shapes, and the only candidate where a λ's claim
   is *not a special case*. Blocked on one question, and worth answering because the answer is cheap
   and the prize is a construct removed rather than added.

**And, orthogonally, I — one tree, two renderings**, which is how B′ and D stop competing.

**Dropped, with the reason:**

| | why |
|---|---|
| A | 3 holes, and the fourth slot is *ambiguous* with the body, because ADR 0049 makes sets expressions. Not fixable by adding slots. |
| C | Dropped as a surface, **kept as the reading of B′**: its uniformity is B′'s meaning, and deriving it is what makes B′'s `where` a shorthand rather than a construct. |
| F | F4′: a second type grammar is Church-style stratification, against ADR 0049. Its `requires`/`ensures` clauses survive in B′. |
| G | 1 hole (a relational precondition has no place) and F4′: currying an isomorphism, which core-0 §0 already records as a wrong reading. |
| H | F5: permits a λ claimed differently per call site, where ADR 0028 needs one statement. Still legal and useful as a caller's narrowing. |

### 1.8 What installment 1 fixes for everything after it

Whatever is chosen, these are now constraints on the rest of the language:

1. **`def` is a λ with a name, and there is one form.** `sig` disappears; the name and the arity are
   written once. Every later binding construct (`let`, `local`, `loop`, a variant's parameters, a
   host declaration) must use the *same* binder shape, or U breaks.
2. **A binder is `pattern ∈ set [mode]`**, with the set optional and the mode from fit-research
   decision 2 (`linear`, `inout`, `out`). "No set" is **not** `∈ V`: it is *no claim*, which
   settypes §10 insists is different and must not be merged.
3. **The result is a binder group, not an annotation.** So a form that produces several values writes
   several result binders, and `values`/ADR 0031's product has a place already.
4. **φ and ψ are comprehensions over those groups**, so a `where` anywhere else in the language —
   on a `variant`, a `loop`'s invariant, a `local` — is the same shorthand for the same operator.
5. **Claims are a deletable region.** Any later construct must keep its claims in labelled sections
   or above a separator, never interleaved with the terms they are about, so ε stays a deletion.
6. **No type on a term.** No `(fn ((x : int)) …)` reading, in any later construct.

### 1.9 Open, for hamza

1. **Is there a function-set former** — `(-> P R)` / `P → R`? It decides candidate E, and it is prior
   to the spelling. The cost is a **third level for W9 to admit** (§1.4: not that functions are not
   values, since sets are not either, but that a set of operators is neither of W9's two levels); the
   prize is that a λ's claim stops being a special case and §2.6's `Π` becomes writable.
2. **Of B′ and D, which is the source**, or both under I (write B′, read D)?
3. **Does a λ ever carry a claim in practice?** The corpus says almost never (§1.3(c): the Church λs
   cannot and should not). If the honest answer is "only a named one", then the λ keeps *two* slots
   and all eight live on `def` — which is a fourth candidate, and the smallest of all. I have not
   proposed it as its own row because Finding 1 says they are one form; but if λ and `def` are kept
   apart deliberately, it is the right answer and it should be said so.
