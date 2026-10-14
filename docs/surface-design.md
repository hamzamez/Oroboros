# The language surface, designed one construct at a time

Status: **an open conversation, appended to as it goes. The surface is not decided; its candidates are
D and J** (ADR 0056); what is decided around it is listed below. Started 2026-10-11 on
hamza's request: *"let's try to come up with the best language surface we can … we start with the atom
lambda … let's find out the syntax with the least holes in it, simplest, most uniform … we are going to
propose the most right now, pick a few, drop the rest and continue the exercise with the rest of the
language, but what comes after has to fit with what we pick now."*

The semantics is fixed and is not under discussion here. It is
[abstract.md](spec/abstract.md)'s signature Σ, [settypes-research.md](settypes-research.md)'s logic
with [ADR 0049](decisions/0049-types-are-sets-and-the-logic-never-prints.md), and [fit-research.md](fit-research.md)'s fourteen
places where that semantics chooses a form (its decisions 1–6 taken by ADR 0050; the seventh, the
surface, is this document). The candidate notations are
[notation-research.md](notation-research.md)'s and [syntax-research.md](syntax-research.md)'s. **Only
the spelling is open.**

**Decided since this document began, and binding on every installment.** A correction pass on
2026-10-11 marked each place below that one of these overturns. The reasoning is kept as written,
since the document records how the design was reached.

| | decided | by | what it fixes for the surface |
|---|---|---|---|
| 1 | fit-research decisions 2–6 | [ADR 0050](decisions/0050-the-abstract-syntax-fits-the-semantics.md) | a mode on a binder (`linear`, `inout`, `out`), never a set; one definition carrying its claims; sets formed explicitly; pairs as pairs and one guarded choice; **indexing marked**, `t[i]` against `f(x)` |
| 2 | a definition is a named λ, and the λ carries its claims | [ADR 0051](decisions/0051-a-definition-is-a-named-lambda.md) | Finding 1 (§1.4) and §1.9's decision 3; a set per binder plus φ and ψ, meaning the sets P and R of Finding 2; **the result is one binder whose pattern may be a tuple**, so a result group is sugar for that pattern; a λ's body is never a tail, and `let` binds a jump |
| 3 | the arrow; E dropped; F as F#-inspired; J a candidate | hamza, §1.9; the arrow by [ADR 0053](decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md), on [arrow-research.md](arrow-research.md) | **the arrow is a claim used as a set**, a λ's header without a body, only on a host definition's parameter or as a named claim; not an operation of the type algebra; the domain a binder list, so no `->` token is needed |
| 4 | buffers in program order | [ADR 0052](decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), on [storeorder-research.md](storeorder-research.md) | **a buffer is a state binder holding a table**: `b := table(n, 0)` a buffer, `t == table(n, 0)` an immutable table, `b[i] := v` an element write, `b := f(b, …)` a whole write; `linear` only on a parameter, a borrow passed by reference; R0–R4 in place of the ordering check; the host write-borrow's wrapper stays |
| 5 | `linear` is an ordering, not a count | [abstract2](../gauntlet/results/abstract2-2026-10-11.md) | under item 4 a scope's buffer never shows the ordering on the page; it remains the meaning of a `linear` parameter |
| 6 | in J, `=` binds and `==` compares, for now | hamza, 2026-10-11 | C's and Go's convention, in J's examples; D and K keep TLA+'s, `==` binding and `=` comparing (§2.11) |
| 7 | `let` and `local` are one scope; a definitional name may be rebound, a state name once per region; the head is `let` | [ADR 0054](decisions/0054-let-and-local-are-one-scope.md), on §2.13 | binders state their kind by their operator, and a mixed sequence is one scope; in K and J, a state binder's introduction against a write is still open (§2.13, item 4) |
| 8 | a loop's variables are bound by application, not assigned | [ADR 0055](decisions/0055-a-loop-variable-is-bound-by-application.md), hamza 2026-10-12 | `LOOP i == 0` in D and K, `loop i = 0` in J, `(loop ((i = 0)) …)` in B′: one iteration is a `let` (the unfolding law), so `:=` is only a place taking a value; `again` is an application, positional |
| 9 | the source is D or J; one arm grammar (`cond`, `match` retired); `again` positional; `;` tighter than `[]` in D; `var` marks state in J; one-armed `if` in J; J's formers `array { … }`, `set { … }`; `array(n, f)` for a rule; `Seq(S)` for the set | [ADR 0056](decisions/0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md), §3.9–3.10 | B′ and K dropped; the handled bind and a loop's invariant slot still open (§3.11) |
| 10 | a loop's invariant slot | hamza, 2026-10-12 (ADR 0056's amendment) | `LOOP i == 0 INVARIANT I :` in D, `loop i = 0 invariant I { … }` in J; optional, absent meaning ⊤; ε erases it, 𝒪 makes three obligations of it (§3.11) |
| 11 | the handled bind, a derived operator with `try` and `expect` its instances; in a loop its success arm is a tail and its handler leaves | [ADR 0057](decisions/0057-the-handled-bind-and-its-tail.md), §3.11–3.14 | J `try p = e [else ā], … { body } [else ā]`, D `TRY p == e [ELSE ā], … IN body [ELSE ā]`; no statement form of `try`; a small two-way choice stays `case`; `expect` stays a statement |
| 12 | a map buffer is a state binder holding a map; tuples `(a, b)`, `()` in J and `⟨a, b⟩`, `⟨⟩` in D; concatenation `++` in J and `∘` in D; sets in D TLA+'s, in J `set { … }`, ranges `lo..hi` | [ADR 0058](decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md), hamza 2026-10-12 (§4.7, items 4–7) | open (§4.8): the tag's mark, the forms of arrays and maps, J's set operators, tag qualification |
| 13 | a tag is a quoted symbol, `'name`; case carries no category; one former shape `former { … }`, `array(n, f)` gone; the sets `Array(V)` and `Map(K, V)` | [ADR 0059](decisions/0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md), §4.8 | open (§4.9): `:` as the former's separator, J's membership and set operators, tag qualification |
| 14 | J's membership `∈` (input `\in`) and Python's set operators `\| & - ^ <= < *`, D's TLA+'s; an imported tag qualified `alias.'tag`, a module's own and the language's unqualified, no `exposing` | [ADR 0060](decisions/0060-j-set-operators-and-qualified-tags.md), §4.9 | open (§4.10): the colon's two orders |
| 15 | inside a former, separation `set { x ∈ S : P }` and replacement `set { e : x ∈ S }`, `array { e : k ∈ 0..n-1 }` by the binder's place; `map { k : v }` a graph only; the colon set-builder's | [ADR 0061](decisions/0061-separation-and-replacement-by-the-binders-place.md), §4.10 | D writes TLA+'s; the image of a membership test is parenthesized |
| 16 | a header is the membership statement, the result a binder `(r ∈ U)` or its set `(U)`, `result` gone; bounds in binder sets, a telescope, relations in `where`; one equality in formulas, the logic's; quantifiers, `==>`, `<==>` (D TLA+'s) in claim positions only; `fn`, `spec`, `lemma` share the header, a lemma used by `assert L(…)`, a recursive one an induction; `assert` proves, `expect` checks; `the` leaves the surface; no claim ≠ `any` | [ADR 0062](decisions/0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md), §5.7 | installment 5's G1–G5 are the decided spellings, modulo D or J |

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

> **Since ADR 0051** abstract.md's λ carries the claim, **lam**(b̄; r; φ; ψ; e), and a definition is
> def(f; e) with e a λ: the two forms this section compares are one. The paragraph above and the
> table below describe the spec as it stood when the installment was written.

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
a definition or a host call may return several values, a tuple (`values` was retired for `tuple`;
ADR 0031 is a scope's product). ADR 0051 D3 since made the tree's result **one binder whose pattern
may be a tuple**, so a group of result binders is sugar for `tuple(q, r) ∈ U × V`.

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

// a host body: the body slot holds a host template, in the same braces
fn Encode(dst ∈ Seq(Byte) linear, src ∈ Seq(Byte)) (n ∈ Nat) pure {
  host expr "hex.Encode(%s, %s)" import "encoding/hex"
}

// a callback parameter: its set is a claim, a header with no body (§1.8, ADR 0053)
fn sortSlice(a ∈ Seq(Value), less ∈ fn(i ∈ Int, j ∈ Int) (r ∈ Bool)) () {
  host stmt "sort.Slice(%s, %s)" import "sort"
}

// a named claim: Go's named function type, a header bound to a name (ADR 0053)
SplitFunc = fn(data ∈ Seq(Byte), atEOF ∈ Bool) (advance ∈ Int, token ∈ Seq(Byte), err ∈ Error)

// a call with a callback, tier 1: the λ is written at the call site
sortSlice(ages, fn(i, j) { ages[i] < ages[j] })
```

**One body form** (rewritten in the pass of 2026-10-11, at hamza's request). J first wrote a host body
after `=` and every other body in braces: two delimiters for one slot. In abstract.md a definition's
body slot holds an expression *or* a host template (def(f; λ; host(…)), §2.6), a sum of two sorts in
one position, so one delimiter serves both: `{ host … }` is a body whose one statement is the
template, and `host` may stand nowhere else. The header reads the same whatever the body is, and
erasure still deletes the region between the result group and `{`.

- **H: 0. U: 1 shape**, used for both binder groups. **S: 33 tokens, depth 1** (§1.6) — the second
  shortest, within one token of D.
- **F: fits.** Nothing here is a second grammar: `Seq(Byte)` is an application, `0..b-1` a range
  expression, and a callback's set is a header without a body (§1.8, ADR 0053).
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

> **ADR 0053 makes this restriction the algebra rather than a precaution** (arrow-research.md §5):
> the arrow is a claim used as a set, a λ's header without a body, so the spellings below lose their
> `->`: `less(i ∈ Int, j ∈ Int) ∈ Bool` in D, `less ∈ fn(i ∈ Int, j ∈ Int) (r ∈ Bool)` in J.

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

1. **Function set formers: yes**, and the link to callbacks is real — §1.8. The arrow's job is a
   binder's set in a declaration, no allowed callback tier makes a λ a value of V, and W9 gains a
   clause rather than a relaxation. It is abstract.md §2.6's Π promoted from prose to a term.
   **Narrowed by ADR 0053** (arrow-research.md): the meaning, a function-space claim, is kept; the
   form is a claim used as a set, not an operation of the type algebra, because no host uses ∪, ∩ or
   ∖ on arrows and a heading carries what a set cannot (call count, escape, no captured buffer).
2. **Candidate E: dropped.** §1.7 records why it costs nothing.
3. **Design the general form.** hamza, on whether a λ ever carries a claim in practice: *"I don't
   know. we do the general form, and we never do, that is not an issue."* So **Finding 1 stands and
   λ and `def` are one form**, carrying all eight slots, whether or not an anonymous λ ever uses
   slots 3–5. This settles the alternative floated before the decision (the λ keeps two slots, and
   all eight are on `def`) against itself: it is not proposed. Recorded as ADR 0051's D1.
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
3. ~~**The arrow's domain: a tuple set or curried**~~ **Settled by ADR 0053**: the arrow is a claim,
   whose domain is a binder list, so arity is exact and the form is the tuple's.

### 1.10 What installment 1 fixes for everything after it

Now that hamza has decided: Whatever surface is chosen:

1. **λ and `def` are one form**, carrying all eight slots (decision 3). `sig` disappears; a name and
   an arity are written once. Every later binding construct — `let`, `local`, `loop`, a variant's
   parameters, a host declaration — uses the *same* binder shape or uniformity breaks.
2. **A binder is `pattern ∈ set [mode]`**, the set optional, the mode from ADR 0050 (`linear`,
   `inout`, `out`), and `linear` only on a parameter (ADR 0052). "No set" is **not** `∈ V`: it is
   *no claim*, which settypes §10 insists is different and must not be merged.
3. **The result is a binder, not an annotation** — one binder whose pattern may be a tuple
   (ADR 0051 D3), so several results need no new form. A notation may write it as a group,
   `(q ∈ Int, m ∈ 0..b-1)`, as sugar for the tuple pattern; Go's named results are the precedent
   (§1.5, J).
4. **φ and ψ are comprehensions over those groups** (Finding 2), so a `where` anywhere else — on a
   `variant`, a `loop`'s invariant, a `local` — is the same shorthand for the same operator.
5. **Claims are a deletable region**, between the result group and the body's delimiter. No later
   construct may interleave a claim with the terms it is about, or ε stops being a deletion.
6. **No type on a term.** No construct may read as "this term has this type" (§1.3).
7. **A set of operators is a binder's set and nothing else** (§1.8), and it does not reopen tier 3.
8. **The body has a delimiter whose failure is syntactic** (§1.7), if J or B′ wins — which is a
   constraint on every later construct that has a body, not just the λ. §2.9 amends it: *a body's
   extent is bounded by something whose violation is a syntax error*.
9. **A λ's body is never a tail** (ADR 0051 D5). A jump is bound by `let`, never by a λ applied by
   hand, so no notation may make the two look alike.
10. **Indexing is marked** (ADR 0050, decision 6): `t[i]`, or T′'s `(at t i)`, never `t(i)`, since a
    table is a value and an operator is not.

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

**And a third hazard this section missed** (found in installment 3, §3.4's Finding 4): a keyword form
that a statement *follows*, `IF c THEN a ELSE b ; rest`, where the munch takes `rest` into the else.
H1 counted operand positions only. In sequence positions the corpus has 31 such forms, and one
precedence rule for `;` inside arms decides what they cost.

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

> **ADR 0052 changes slots 2 and 4.** A buffer is a state binder holding a table, so the kinds are
> two, definitional and state. `linear` belongs to a parameter only, so slot 4 is empty for a
> scope's binder.

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
operator**, `set` is free for the set former, and no `store` name is needed. notation-research §4.5
proposed it, and this installment should have started from it. It was not decided until
storeorder-research.md and ADR 0052.

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
is the identical arity; the measurement only fails to contradict it. (Corrected in §2.13: over the
whole corpus, 4 more chains nest the other way, `local` around `let`, so the merge flattens 7.)

**Finding 2. One binder shape, three operators — and installment 1's parameter is the degenerate
case.**

```
pattern  [∈ S]  = e        definitional   a let; an immutable table when e is a table
pattern  [∈ S]  := e       state          a cell, a loop variable, or a buffer when e is a fresh table (ADR 0052)
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

> **Superseded by ADR 0052** (hamza, 2026-10-11). The table above lacked a fourth option: with
> stores in program order, **a buffer is a state binder**, so the operator already says the binder
> may be written. `b := table(n, 0)` is the buffer and `t == table(n, 0)` the immutable table, at 0
> sites and with no fourth kind. The initializer's form survives as a legality condition, R0: an
> element write needs a fresh table, so `x := xs ; x[0] := 1` is refused and the copy is written,
> `x := table(len(xs), xs)`.

### 2.4 Nine programs, four notations

**T** today, **B′** sectioned prefix, **D** mathematical, **J** Go-inspired. B′ writes an arm as a
pair, `(guard → body)`, as ADR 0050's decision 5 requires (corrected in the pass of 2026-10-11; P5
and P6 had read their clauses off parity, as T does). D's and K's examples follow §3.4's Finding 4,
rule (a): `;` binds tighter than `[]`, so an arm's body may be a sequence, and a chain or `IF` that
something follows is parenthesized. B′ marks indexing,
`(at t i)`, as ADR 0050's decision 6 requires and abstract.md §9.4's T′ writes it (corrected in the
pass of 2026-10-11; it had written `(src i)`, today's application). T is today's notation and keeps
`(src i)`. J is shown in its
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
{ (src, err) = os.ReadFile(p);  if err == 0 { src } else { "" } }               // J
```

---

**P3 — a claim on a binder.** The hole: **T cannot write this at all.**

```lisp
(let b (the (int 0 255) (src i)) …)        ; T — the claim goes on the VALUE, not the binder
```
```lisp
(let (((b (int 0 255)) = (at src i)))  …)  ; B′ — the binder's left side is installment 1's
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
  (loop ((j = (* i i)))
    ((< j n) → (seq ((at c j) := true) (again (+ j i))))
    (else → c)))
```
```
LET c := [k ∈ 0..n-1 ↦ FALSE] IN                                                \\ D
  LOOP j == i * i :
      j < n  → c[j] := TRUE ; AGAIN(j + i)
   [] OTHER  → c
```
```go
{ c := [k ∈ 0..n-1 |-> false]                                                   // J
  loop j = i * i {
      j < n -> { c[j] := true; again(j + i) }
      else  -> c
  } }
```

**The store is `c[j] := true`, in program order — corrected 2026-10-11 on hamza's question.** I had
written `store(c, j, true)`, threading the buffer by hand as today's notation does. hamza: *"I thought
indexing in j and D are going to be `c[j]`?"* — and **notation-research §4.5 had already proposed it**,
which I wrote against. It was decided only later, by ADR 0052, after storeorder-research.md:

> `x := e` writes a cell, and **`t[i] := e` writes a table's element** … **buffers are written in
> program order too.** Today a buffer is threaded by hand, `(again (set s j true) …)`. That form
> exists because the refinement layer's facts induct on a buffer's store chain. **The translation can
> produce that chain itself**, as it produces cells' loop variables, so the programmer writes
> `s[j] := TRUE` and the analyses still see the chain.

So the store is not a *call* at all. It is `:=` with an **indexed** left-hand side, and §2.12's
measurement is where the gain shows: the loop **loses a variable**. `LOOP c == c, j == i * i` becomes
`LOOP j == i * i`, and `AGAIN(store(c, j, TRUE), j + i)` becomes `c[j] := TRUE ; AGAIN(j + i)`.

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
  (loop ((i = 0))
    ((>= i (len src)) → (tuple lines words))
    (else → (let ((b = (at src i)))
              (seq (if (= b 10) (lines := (+ lines 1)) (tuple))
                   (again (+ i 1)))))))
```
```
LET lines := 0  words := 0  in_word := FALSE IN                                 \\ D
  LOOP i == 0 :
      i ≥ Len(src) → ⟨lines, words⟩
   [] OTHER        → LET b == src[i] IN
                       (IF b = 10 THEN lines := lines + 1 ELSE ()) ;
                       AGAIN(i + 1)
```
```go
{ lines := 0;  words := 0;  in_word := false                                    // J
  loop i = 0 {
      i >= len(src) -> (lines, words)
      else -> { b = src[i]
                if b == 10 { lines := lines + 1 }
                again(i + 1) } } }
```

A write is `x := e` — **the same operator that bound the cell**, which is PlusCal's and Ada's
reading: `:=` means "this name takes this value", at the binder and at the write alike. So `set`'s
two-argument form disappears too, and §2.2's 9 cell writes and 58 buffer stores all become `:=`, as
the next paragraph says.

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
(let ((n = (len xs))  (out := (table n 0))  (count := 0))  …)                   ; B′
```
```
LET n == Len(xs)  out := table(n, 0)  count := 0  IN  …                         \\ D
```
```go
{ n = len(xs);  out := table(n, 0);  count := 0;  … }                           // J
```

Corrected in the pass of 2026-10-11: B′, D and J wrote the buffer `out` with the definitional
operator, which makes it an immutable table, while P5 and K's P7 wrote a buffer with `:=`. Under
ADR 0052 it is `out := …`. That adds one token to B′'s and J's counts in §2.5 and §2.9 (`=` becomes
`:=`), and none to D's (`==` becomes `:=`); the tables there are left as measured.

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
since `:=` says cell whatever the initializer is. Under ADR 0052 this *is* a buffer: a state binder
holding a table, replaced whole by a fresh one.

```lisp
;; T — no spelling exists
```
```lisp
(let ((cur := (array 1 2 3)))  (seq (cur := (array 4 5 6)) (at cur 0)))         ; B′
```
```
LET cur := ⟨1, 2, 3⟩ IN  ( cur := ⟨4, 5, 6⟩ ; cur[0] )                          \\ D
```
```go
{ cur := array(1, 2, 3);  cur := array(4, 5, 6);  cur[0] }                       // J
```

### 2.5 Measured

Tokens, depth and longest closing run over **P1–P8** (P9 is excluded, since it has no T column),
counted by **`go run gauntlet/surface/tokens.go`**, which reads this document's blocks, so the table
and the programs cannot drift apart. Its header states the rule. **Tokens** follow §1.6: every
non-word character is one, so `:=` is two. **Lexemes** take a run of operator characters as one, as a
lexer would.

| notation | tokens | × T | lexemes | × T | depth | closing run | holes | shapes |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| T today | 194 | 1.00 | 193 | 1.00 | 7 | 6 | **2** | **8** (fit-research §3.3) |
| B′ | 285 | **1.47** | 274 | **1.42** | 8 | 7 | 0 | 1 |
| D | 222 | 1.14 | 201 | 1.04 | **2** | **2** | 0 | 1 |
| J | 220 | 1.13 | 199 | 1.03 | 4 | 4 | 0 | 1 |

> **Recounted in the pass of 2026-10-11.** The first table here (T 229, B′ 336, D and J 232) and
> §2.9's (T 239, B′ 312, D and J 243) came from a script that was not kept, disagreed with each other
> on the same programs, and counted different programs for K and L. This one is reproducible. Since
> then B′ marks indexing, `(at t i)`, and J compares with `==` (§2.4), which the counts include.

**My estimates were wrong again, and the recount moved the conclusion once more.** I had written that
D and J were ~15% shorter than today, then measured them the same size. Counted by one kept rule,
they are **the same size in lexemes (1.03–1.04×) and 14% larger in characters**: an explicit binder
operator, `==` or `:=`, costs two characters where T's parity costs none, which is fit-research
§3.6's misfit priced. So:

- **The case for J and D is not size. It is depth, and it is the structural simplifications.** Both
  write about as many lexemes as today while dropping from depth 7 to 4 and 2, and while removing
  `seq`, the `let`-around-`local` nesting, the eight binder shapes and the two holes. The notation
  costs little and the *structure* is what improves — which is a better result than a size win would
  have been, because it cannot be argued away as formatting.
- **D is the shallowest by half**, 2 against J's 4, and this reverses installment 1, where the two
  tied at depth 1. The cause is that `LET … IN` adds no bracket while `{ … }` adds one per block, and
  P5–P8 all nest a loop inside a scope. On the one measure where J and D differed, D now wins.
- **B′ is 1.47× today's tokens (1.42× in lexemes)**, one level deeper and with a longer closing run —
  it pays nearly half again as much for no structural gain over the others. Writing its arms as
  pairs, as decision 5 requires, added a bracket per arm. The cause is structural, not incidental: a binder is
  `(pattern OP rhs)`, a pattern may itself be a list, and a claim makes the left side a list, so P2
  and P3 carry three opening brackets before the first name.
- **T's two holes** are slot 3 (a claim on a binder, unwritable) and slot 2 (the kind, inferred).
  Its eight shapes are fit-research §3.3's count, which every candidate here reduces to one.

### 2.6 Kept, and dropped

**The installment's result is structural and it is the same in all three candidates.** Findings 1–3
hold whatever the spelling: one binder shape with three operators, `let` and `local` merged, the kind
stated rather than inferred, and `set` replaced by one operator, `:=`, with two left-hand sides. That is where the two holes and
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

**B′ falls behind.** It was installment 1's co-leader. Here it is 1.47× the tokens, one level deeper
than the notation it replaces, and the cause is the shape itself rather than
the examples. Kept on the list, because the reader exists and its failure is syntactic, but it is no
longer a front-runner and installment 3 should be where it is either rescued or dropped.

**Dropped:** nothing new. P9 rules out any candidate that keeps ADR 0047's inference, and no live
candidate does.

### 2.7 What installment 2 fixes for later

1. **One binder shape, three operators** (Finding 2): `pattern [∈ S] = e`, `:= e`, or `∈ S` alone.
   Every later binder — a loop's variables, a `case` arm, a variant's parameters, a comprehension —
   uses it, or uniformity breaks again.
2. **`let` and `local` are one form** (Finding 1; decided by ADR 0054, §2.13). "This scope has state"
   is read off the binders, and the head, where there is one, is `let`.
3. **The kind is stated by the operator, never inferred from the initializer.** `=` is
   definitional, `:=` is state, and ~~**`linear` is written** where a definitional binder is a
   buffer (26 sites)~~ **a buffer is a state binder holding a fresh table** (ADR 0052, which
   supersedes the 26 markers). The constructors say only how the contents are given — `array` a
   graph, `table` a rule — and nothing about mutability (Finding 3, withdrawn).
4. **One operator, two left-hand sides.** `x := e` writes a cell, `c[j] := e` stores into a buffer,
   and `set` retires — freeing it for the set former (fit-research decision 4). No later construct
   may distinguish two operations by arity. A buffer's threading is **produced by the translation**,
   not written, so the proofs still induct on the store chain (ADR 0052).
5. **A block is a statement list whose value is its last expression**, if J wins — and then `seq`
   never returns, and any later construct with a body uses the same block.

### 2.8 Open, for hamza

1. **Does the statement-list block win?** It is the installment's finding, and it is a larger change
   than a spelling: it makes a body a *sequence* rather than a slot, which removes `seq` and one
   nesting level everywhere. Everything after this installment inherits the answer. Note it is
   **separable from the braces** — see 1b.
1b. ~~**The hybrid: `LET … IN` over a statement-list body.**~~ **Priced in §2.9, and the shape was
   wrong**: if binders are statements, a group before them is redundant. The real hybrid is
   statements with no bracket (**K**), and it wins every measure — today's size in lexemes, depth 2,
   closing run 2 — while keeping a syntactic failure, under §3.4's Finding 4 rule (a).
2. ~~**`x := table(n, f)`**~~ **— answered in §2.10, and answered again by ADR 0052**: `x := table(n, f)`
   is now the buffer itself. §2.10's answer, as written: my reason was wrong. `table(n, f)` is a
   table-valued expression, so `:=` binds a cell holding it and nothing is refused. A cell holding a
   **buffer** is refused too, but *not* because "a buffer is not a value" — a buffer is a value
   (§2.10). It is refused by **linearity**, already checked at every binder, because the
   state-passing translation duplicates the reference. Verified.
3. ~~**Decided by the correction above, and worth confirming**: `linear` is **written** on a buffer
   binder, 26 sites.~~ **Superseded by ADR 0052**: a buffer is a state binder, and `linear` is
   written only on parameters.
4. **ADR 0047 needs a successor** either way (now ADR 0052 for buffers and ADR 0054 for the merge, §2.13): its initializer rule is what the stated kind replaces,
   and its translation, cells-as-loop-variables and everything in local.md §3 are unchanged.
   **ADR 0052 amends it** for buffers; whether `let` and `local` merge, the other half, is still
   this document's question.
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
b ∈ 0..255 == src[i] ; …                                                        \\ P3
io.print_line("hi") ; 0                                                         \\ P4

lines := 0 ; words := 0 ; in_word := FALSE ;                                    \\ P6
LOOP i == 0 :
    i ≥ Len(src) → ⟨lines, words⟩
 [] OTHER        → b == src[i] ;
                   (IF b = 10 THEN lines := lines + 1 ELSE ()) ;
                   AGAIN(i + 1)

c := [k ∈ 0..n-1 ↦ FALSE] ;                                                     \\ P5
LOOP j == i * i :
    j < n  → c[j] := TRUE ; AGAIN(j + i)
 [] OTHER  → c

n == Len(xs) ; out := table(n, 0) ; count := 0 ; …                             \\ P7
age := 0 ; name := "" ; … ; ⟨age, name⟩                                         \\ P8
```

K is **D's lexicon with `;` promoted**, not a new notation: same `==`, `:=`, `∈`, `LOOP`, `⟨⟩`, same
ASCII spellings. The only change is that a binder may appear mid-sequence, so `LET` and `IN`
disappear.

#### L — a binder group, then a braced body

```
let av = os.Args(), n = len(av) { n - 1 }                                       // P1
let (src, err) = os.ReadFile(p) { if err == 0 { src } else { "" } }             // P2
let b ∈ 0..255 = src[i] { … }                                                   // P3
let _ = io.print_line("hi") { 0 }                                               // P4
let c := [k ∈ 0..n-1 |-> false] {                                               // P5
  loop j = i * i {
      j < n -> { c[j] := true; again(j + i) }
      else  -> c } }
let lines := 0, words := 0, in_word := false {                                  // P6
  loop i = 0 {
      i >= len(src) -> (lines, words)
      else -> let b = src[i] {
                if b == 10 { lines := lines + 1 }
                again(i + 1) } } }
let n = len(xs), out := table(n, 0), count := 0 { … }                           // P7
let age := 0, name := "" { …; (age, name) }                                     // P8
```

K's P3, P7 and P8 were written with calls (`f(b)`, `g(out, count)`, `h(age)`) where §2.4 writes `…`,
and L was written for P1 and P6 only, so the first version of the table below counted different
programs per row.
Both are corrected in the pass of 2026-10-11: every row counts the same eight programs.

#### Measured, P1–P8, by `gauntlet/surface/tokens.go`

| | corner | tokens | × T | lexemes | × T | depth | closing run | lines |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| T | today | 194 | 1.00 | 193 | 1.00 | 7 | 6 | 16 |
| B′ | prefix: group + parens | 285 | 1.47 | 274 | 1.42 | 8 | 7 | 15 |
| D | **(a)(i)** group + munch | 222 | 1.14 | 201 | 1.04 | 2 | 2 | 16 |
| J | **(b)(ii)** statements + braces | 220 | 1.13 | 199 | 1.03 | 4 | 4 | 16 |
| **K** | **(b)(i) statements + munch** | **214** | **1.10** | **194** | **1.01** | **2** | **2** | 16 |
| L | **(a)(ii)** group + braces | 233 | 1.20 | 212 | 1.10 | 4 | 4 | 16 |

Recounted in the pass of 2026-10-11 (§2.5's note): the first version of this table had K at 0.97×
and "the only one below today", on programs that were not the others'.

**The square separates cleanly, and the two axes drive different measures:**

- **the body delimiter drives depth**: munch → 2, braces → 4. A block is a bracket, and P5–P8
  all nest a loop inside a scope, so every block costs a level;
- **the binder placement drives tokens**: statements → fewer, because there is no `LET`/`IN` or
  `let`/`{` keyword pair per group, and a mid-sequence binder needs no nested form.

So **K takes the better option on both axes and L takes the worse on both** — L is the largest of
the four new corners while also the deepest, which is the signature of paying for a keyword pair and a
bracket and getting neither's benefit.

**K is the smallest new notation and, with D, the shallowest**: today's size in lexemes (1.01×), 10%
above it in characters, and depth 2 with a closing run of 2. (Before installment 3's Finding 4 put
parentheses around P6's `IF`, which a statement follows, it measured depth 1.) None of the six is smaller than today in characters, because each writes the binder
operator that today's parity leaves out.

#### K's failure stays syntactic — which was the objection to (i)

§1.7's argument against a munching body is that a mistake can produce a program that parses and
means something else. **K escapes it, and not by luck:** `;` is the **loosest** operator in
notation-research §4.9's precedence table (level 1), so **`;` terminates a munching keyword form.**
In K the ordinary case is bounded by a separator rather than a bracket, and what is left is exactly
what §1.11 priced:

> **Corrected by §3.4's Finding 4.** K's own loops put sequences inside arms, which "`;` terminates a
> keyword form" would cut at the first `;`. One precedence must decide, and the cheapest by the corpus
> is the guarded commands' own: `;` binds tighter than `[]`, an arm's body may be a sequence, and a
> chain or `IF` that something follows is parenthesized, **31 sites**, not free. The table below is
> as first written; the row it gets wrong is the first.

| | closed by | cost |
|---|---|---:|
| a statement's extent | `;`, looser than everything | ~~free~~ **31 sites** (Finding 4) |
| H1 — a keyword form as an operand of an infix operator | §1.11's grammar rule | **0 sites** |
| H2 — a nested clause chain | §1.11's required delimiter when nested | **12 sites** (22 over the whole corpus, §3.2) |

So K sits in the same failure class as J and B′ — a mistake is a reader error — **without a bracket**.

**K's one identified new cost**: `;` is looser than `,`, so a sequence in an *argument* position needs
parentheses, `f((a ; b))`. Measured over 4,649 name-headed forms: **0 sites**, and a crude grep for
`(f (seq …`, `(f (let …` and `(f (local …` agrees. Free today.

#### What K buys that D cannot

A binder **after** an effect, in one sequence, with no keyword pair and no parentheses. Compare P6's
arm:

```
D:  [] OTHER → LET b == src[i] IN (IF b = 10 THEN lines := lines + 1 ELSE ()) ; AGAIN(i + 1)
K:  [] OTHER → b == src[i] ; (IF b = 10 THEN lines := lines + 1 ELSE ()) ; AGAIN(i + 1)
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

**K becomes the front-runner for installment 2** (on depth and structure; on size it ties today),
and it amends installment 1's constraint 8. That
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
`core/scope.go`'s `readLocal` desugared (since step 1a, μ reads a `local` into a tree in
`core/mu.go` and D makes the core form through `localBinder`)

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
| **the reader** | **one function instead of two** (`readLet` + `readLocal`, since step 1a `muLet` + `muLocal`), and it needs a marker to know which binders are `build`s — see below | the only place that changes |
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

> **ADR 0052 makes the bill zero.** A buffer is the state binder holding a table, so `:=` is the
> marker, and the reader decides `build` from the operator and R0's fresh initializer.

#### What is not verified

These are claims about where the code keys off what, read from the source and spot-checked by the
three refusals above. **No merged reader has been built or run**, so `cmd/check` has not been through
it. What would confirm it is the smallest possible experiment: teach `readLet` to accept `linear` and
route those binders through `localBinder`, then check that emission is byte-identical on the 26 sites.
That is a few hours and it is the right first build step if the merge is taken. (Under ADR 0052 the
experiment routes `:=` binders with a fresh table instead; `readLet` is now `muLet`.)

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

`readLet` and `readLocal` both **right-fold** into a chain of single-binder forms (then
`core/scope.go`; since step 1a, D in `core/desugar.go`, `for i := len(binds) - 1; i >= 0; i--`). So "grouped" and "scattered" are one
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

**3. Linearity — also easier** (and under ADR 0052 replaced by R0–R4, which are syntactic, so the
point is stronger). `emit/linearity.go`: *"it walks in EVALUATION ORDER: a `let`'s value
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
>
> **ADR 0052 raises the stakes.** `t == table(n, 0)` against `b := table(n, 0)` is now the
> difference between an immutable table and a buffer. J's `=` was also its equality test,
> `if err = 0`, a homonym (fit-research's F2). **hamza, 2026-10-11: in J, `=` binds and `==`
> compares, for now**, C's and Go's convention, and J's examples are written so. D and K keep TLA+'s,
> `==` binding and `=` comparing. The two lexicons are opposite on these two tokens, which matters
> only if both survive (§1.9's "two notations or one").
>
> One hazard remains in J's choice, and it is Go's own: in Go `x := 1` *declares* and `x = 2`
> *assigns*, while in J `x = 1` binds an immutable name and `x := 1` a state variable. A Go reader
> reads J's two binders with roles nearly swapped. §2.13 meets the same pair from the other side.

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

On the sieve's inner loop, tokens barely move — the gain is structural. (These counts came from the
script that was not kept, and the threaded versions are not in this document, so they cannot be
recounted; the structural claim below does not rest on them.)

| | threaded by hand | `c[j] := TRUE` | |
|---|---:|---:|---|
| D | 48 | 46 | depth 2 → **1** |
| K | 47 | 45 | depth 2 → **1** |
| J | 48 | 51 | depth 4 → 4 (the arm needs its own braces) |

What the counts miss: **`c` stops being a loop variable.**

```
LOOP c == c, j == i * i :  …  AGAIN(store(c, j, TRUE), j + i)     threaded
LOOP j == i * i         :  …  c[j] := TRUE ; AGAIN(j + i)         program order
```

One fewer loop variable, one fewer argument at every back edge. That is the same kind of win as
`seq` disappearing (§2.6): a slot removed, not characters saved.

#### Consequence 1: a host write-borrow becomes a plain write

notation-research §4.5: *"`hex.Encode(buf, src)` writes `buf` and returns its count. The template no
longer returns the buffer it borrowed: today's `func(dst, src []byte) ([]byte, int) { return dst,
hex.Encode(dst, src) }(%s, %s)` becomes `hex.Encode(%s, %s)`. That is ADR 0048's stage B, for
buffers."*

~~So the surface change pays for itself in the **emitted code**: the wrapper that exists only to hand
the buffer back goes away.~~ **Corrected by storeorder-research §2.7 and ADR 0052, decision 4: it
does not follow.** The translation rebinds the buffer to the version the borrow returns, so the
residual still needs one handed back, and the template stays. Removing it needs the IR to know that
a write-borrow returns the same storage, a separate step with its own measurement.

The counts here were wrong too: they were lines matching `(buffer`, two per declaration. Go has **30
declarations** that take a buffer and hand one back: 12 write-borrows whose template wraps the call
(`encoding/hex` 2, `encoding/binary` 2, `io` 4, `strings` 2, `os` 1, `unicode/utf8` 1), and 18
growth calls (`strconv` 10, `fmt` 3, `encoding/hex` 2, `encoding/binary` 2, `unicode/utf8` 1),
which become `b := f(b, …)`.

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

This is ADR 0052's R1. It needs a companion this section missed, R0: the table a buffer receives
must be fresh, or `x := xs ; x[0] := 1` would write into the immutable `xs`. R1 forbids a second name
going out; R0 forbids one coming in.

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
syntactic condition plus that assumption — which is a real simplification of the 493 lines of
`emit/linearity.go` that walk evaluation order (the 1,180 of `emit/linear.go`, once counted here, are
the linear arithmetic, not linearity), and a claim that should be verified by building it rather than by argument.

#### What this does to §2.10's answer

Nothing, and that is worth saying. §2.10's result was that the *freeze* is at `build`'s boundary and
that the merge costs only the word `linear` (ADR 0052 has since removed even that). The program-order store changes **how a store is
written**, not where the scope ends or what freezing is. The freeze is still the retraction of a
permission at `build`'s boundary; the chain the proofs induct on is still a chain, produced by the
translation instead of typed by the programmer.

### 2.13 Merging `let` and `local`, after ADR 0052

hamza: *"should we discuss merging let and local? if so, go ahead, be mathematical and algebraic."*

**It should be discussed now, because the one argument against it is gone.** §2.3 left Finding 1
(merge: `let` and `local` have one arity in Σ) in tension with the kind. Once the initializer could
not say buffer or value, a merged form had nothing to read the kind from, and option (a) wrote
`linear` at 26 binders. ADR 0052 made a buffer a state binder, so **the binder's operator says the
kind**: `t == table(n, 0)` is a value, `b := table(n, 0)` a buffer, `x := 0` a cell. What remains is
whether the head carries anything the binders do not.

#### The algebra: a scope is a monoid action

Let Bind = Def ∪ State, the definitional and the state binders, and Bind* the free monoid of binder
words under concatenation, ε the empty word. A scope is a map

```
scope : Bind* × Expr → Expr
```

and sequential scoping is the law of a **monoid action**:

```
(A1)  scope(ε; e)          = e
(A2)  scope(β̄; scope(γ̄; e)) = scope(β̄γ̄; e)          the names of γ̄ apart from β̄'s (ADR 0036)
```

- **`let` satisfies it on Def\***: a grouped `let` and nested `let`s reduce to one term (§2.11,
  verified).
- **`local` satisfies it on State\***: the state-passing translation threads the cells in scope
  (local.md §3), and nested `build` regions are the flat one's.
- **D is a homomorphism of the action.** It is a right fold, binder by binder (`core/desugar.go`, and
  `localBinder` for one state binder), so D(scope(β̄γ̄; e)) = D(scope(β̄; scope(γ̄; e))).

**Today's two heads are the restrictions of one action to two submonoids, Def\* and State\*.** A word
that mixes kinds, such as P7's `n == len(xs), out := table(n, 0), count := 0`, lies in the free monoid
on Def ∪ State, which is the **free product** Def\* ∗ State\*. Its normal form is the alternating
reduced word (the normal form theorem for free products; Magnus, Karrass and Solitar, *Combinatorial Group
Theory*, 1966, ch. 4, for monoids as for groups). So today's notation writes a word with r maximal runs of one kind as **r nested heads**,
and a merged scope writes it as one. The merge is the action of the whole free monoid, and the two
heads become what they were all along: the cases r = 1.

#### Does the head carry anything? Four candidates, each checked

A head adds information only if it is not a function of its binder word.

| | what a head might carry | after ADR 0052 |
|---|---|---|
| (a) | **the kind** of each binder | the binder's operator, `==` or `:=`. Before ADR 0052 it was the head, for buffers |
| (b) | **the region**, where state lives and where it freezes | the binder's lexical scope (Lemma below) |
| (c) | **encapsulation**: a scope whose state is local is observationally pure (`runST`; O'Hearn and Tennent 1995) | per scope, unchanged: every state binder of a scope is local to it |
| (d) | **the residual** | none: D is a homomorphism, so a merged scope desugars to today's terms |

**Lemma (regions are invariant under the merge).** Let region(x), for a state binder x, be the
positions from x's binder to the end of the innermost scope containing it. Then (A2) preserves every
region.

*Proof.* (A2) joins a scope with the scope that is its **body**. The inner scope's end is the outer's
end, so no region gains or loses a position. A scope that is a binder's **value**, as in
`x = { b := table(n, 0); …; b }`, is not a body, is not joined, and keeps its own earlier end. ∎

So the freeze stays where ADR 0031 and R4 put it: at the end of the scope that binds the buffer, per
binder. §2.10 already found that "the head `local` was never what delimited the scope. The binder
was." The Lemma is that sentence proved.

**The head is a function of the binder word, so keeping two heads is a redundancy**: two operators
for one action, differing in a restriction the binders already show. That is the synonym
fit-research counts (F1), and Finding 1 now stands unopposed.

#### Measured

`go run gauntlet/surface/scopes.go`, over `examples/`, `lib/`, `gauntlet/stdlib/acceptance/` and
`gauntlet/differential/cases/`:

| | count |
|---|---:|
| `let` forms, binders | 127, 190 |
| `local` forms, binders | 66: 71 buffers, 13 cells |
| outermost scopes | 186 |
| … with one run | 179 |
| … with two runs (an alternating chain) | **7**: 3 `let > local`, 4 `local > let` |
| chains of one head | 0 (both forms are already n-ary) |
| groups binding a name twice | 0 |

**A correction to §2.3's Finding 1**: it counted 3 alternating chains and said "no site nests the
other way". Over `examples/` and `lib/` that was true; the 4 `local > let` chains are in
`acceptance/errors-os.oro` and three differential cases (`alloc-copies`, `buffer-swap`,
`local-tables`). The merge removes 7 nesting levels, and these 7 are its byte-identical test.

#### What the merge must settle: two rules today's notation leaves asymmetric

**(i) A name bound twice in one group.** Today the two heads disagree, verified:

```lisp
(let x 1  x (+ x n)  x)        ⟶  (+ 1 n)                       ; let: the second shadows
(local x 0  y 1  x 5  x)       ;; local binds x twice; a parameter list may not repeat a name …
```

and abstract.md's W2 says the names of every binder group are distinct, which is false for `let`.
The algebra decides it per kind:
- **a definitional binder may repeat a name.** By (A2), `x = 1, x = x + 1` is `scope(x = 1;
  scope(x = x + 1; …))`: shadowing, sound by hygiene (ADR 0036);
- **a state binder's name is bound once in its region.** Bound again by `==`, it would hide the cell,
  refused today (local.md §3). Bound again by `:=`, it would be a second place under the same name,
  and in a group that cannot be told from a write.

The corpus binds no name twice in a group, so the rule refuses nothing. W2 is corrected to say so.

**(ii) A binder against a write, in a statement list.** Σ has two operations: the binder
local(x := e; …), which extends the environment, and assign(x; e), which writes. In a form with a
group (D, L, B′) **position** separates them: a binder stands in the group, a write in the body. In a
statement list (K, J) both are `x := e`, told apart by whether x is already bound. That is resolution
by context, a homonym (F2), and it has a known failure. A write with a misspelled name, `cuont := 0`
meant to reset `count`, silently creates a new cell. It is Python's assignment-declares rule, the one
`nonlocal` and `global` patch. **This belongs to the statement list, not to the merge**: a merged
scope with a group has no such problem. But the merge is what puts state binders and definitional
ones in one sequence, so it is where the question arrives.

The literature's answer is uniform: **mark the introduction.**
- **Algol 68** has exactly this merge. A serial clause mixes *identity declarations*,
  `int n = 5`, and *variable declarations*, `int x := 0`, and writes an assignation `x := 1`. A
  declaration is marked by its leading mode, so it is never confused with an assignation. J's
  operators, `=` for a value and `:=` for a variable, are Algol 68's.
- **PlusCal** declares `variables x = 0;` and assigns `x := 1`.
- **Rust** writes `let x = …` and `let mut x = …`, then `x = …`. Swift writes `let` and `var`, and
  JavaScript `const` and `let`: a keyword marks every introduction.

So a statement-list surface needs one mark on a state binder's introduction, or binders in groups.
Installment 3 settles the statement list (a loop's arms are its first test), so the choice is listed
below and not taken here.

#### What it costs

- **The reader**: `muLet` and `muLocal` become one. D is unchanged, binder by binder.
- **The conditions**: W5's "a state binder of an enclosing `local`" becomes "of an enclosing scope";
  W2 as in (i).
- **The name of the head**, where a surface has one (D's `LET`, B′'s and L's `let`). `local` was
  hamza's word for "state lives here" (ADR 0047), and the binders now say that. K and J need no head.
- **Unchanged**: the freeze, the cell translation, R0–R4, relevance, the proofs, the IR. None of them
  ever saw a head (§2.10).
- **Not merged: a loop's variables.** loop(s̄; I; ā) is a scope iterated by `again`, with arms and a
  simultaneous rebinding. It is installment 3's operator, not this one.

#### Decided, and open

**Decided by hamza, 2026-10-11** (*"I will take 1, 2 and 3"*), as
[ADR 0054](decisions/0054-let-and-local-are-one-scope.md), not built:

1. **`let` and `local` are merged** into one scope whose binders state their kind: the head is a
   function of the binder word (the Lemma and table above), D is a homomorphism, and 7 nesting levels
   go.
2. **Repeated names, by kind**: a definitional name may be rebound (shadowing); a state name is bound
   once in its region. abstract.md's W2 is corrected so; it follows from (A2) and refuses nothing.
3. **The head is written `let`**, where a surface has one: D's `LET`, B′'s and L's `let`. K and J write
   none.

**Still open:**

4. **Binder against write in a statement list**, decided with installment 3:
   - mark the introduction of a state binder, Algol 68's and Rust's answer;
   - keep binders in a group, D's and L's answer;
   - resolve by context, Python's, with its misspelling failure.

   No recommendation yet: it is the statement list's question, and it is priced there.

ADR 0054 amends ADR 0047's head, as ADR 0052 amended its initializer rule, and abstract.md's `let`
and `local` rows, W2 and W5.

---

## Installment 3 — choice and iteration

hamza: *"go ahead"*, after ADR 0054. Constrained by everything decided so far (the table at the top):
one binder shape, the kind stated on the binder, claims a deletable region, no type on a term, a
body's extent bounded by something whose violation is a syntax error, indexing marked, buffers in
program order, one scope. And it inherits §2.13's open question: how a statement list tells a state
binder's introduction from a write.

### 3.1 What the constructs must carry

From abstract.md §2.4–2.7, choice and iteration are two primitive operators, one arm shape and a
tail sort, with five derived operators:

```
case(e; ā)          choice: the first arm whose pattern matches e and whose guard holds
loop(s̄; I; ā)       iteration: binders s̄ bound by application (ADR 0055), invariant I, arms, Tail bodies
Arm  ::= p ⟨if φ⟩ → body                body : Expr in case, Tail in loop
Tail ::= e | again(ā) | let(b̄; t) | try(…; t) | expect(…; t) | assert(φ; t)

if(c; a; b)   = case(c; true → a, false → b)
cond(φ̄ → ē)   = case(tuple(); _ if φ₁ → e₁, …)
try, expect   = case(e; s(x̄) → b, the other constructors re-built, or abandoned)
∧ ∨ ¬         = if with a boolean literal branch
```

| | slot | case | loop |
|---|---|---|---|
| 1 | the scrutinee | an expression, or none (guards only) | the state tuple, implicit |
| 2 | an arm's **pattern** | constructor, literal, tuple, `_`, a set test | over the state tuple |
| 3 | an arm's **guard** φ | optional | optional |
| 4 | an arm's **body** | an expression | a **tail**: a value (an exit) or a jump |
| 5 | the default | `_` (or `else`) | `_` (or `else`), required by ADR 0015 |
| 6 | the **loop variables** and their initializers | — | `x = e`, sequential, rebound only by `again`, by application (ADR 0055) |
| 7 | the **invariant** I | — | a formula, a claim |
| 8 | the **back edge** | — | `again(ā)`, one argument per state binder |

**One arm shape serves both eliminators.** A loop is a case over its state tuple whose arm bodies are
tails, iterated: `match` is a loop with patterns (match.md), and `cond` is a case with no scrutinee.
ADR 0050's decision 5 (one guarded choice, pairs as pairs) is what this installment implements.

### 3.2 Measured

`go run gauntlet/surface/control.go`, over `examples/`, `lib/`, `gauntlet/stdlib/acceptance/` and
`gauntlet/differential/cases/`, read as S-expressions:

| | count |
|---|---:|
| `if` | 142 |
| … one-armed, the else `(tuple)` (a statement) | 4 |
| `cond`, arms | 17, 71 |
| `case`, arms | 42, 87 |
| … a two-armed choice on a sum whose failure arm handles it (§3.4, Finding 2) | **38** |
| `match` (a loop with patterns), guards | 2, 4 |
| `try`, `expect` | 4, 5 |
| `loop`, variables, clauses | 181, 412, 517 |
| loops by number of variables: 1, 2, 3, 4, 5, 6 | 30, 97, 39, 8, 3, 4 |
| `again` sites, arguments | 258, 708 |
| … arguments for a variable that carries a scope's buffer (gone under ADR 0052) | 78, for 37 variables |
| … the other arguments that are their variable unchanged | **121 of 630** (19%) |
| clause chains (`cond`, `case`, `match`, `loop`) nested in another chain's arm | 22 of 242 |
| arm bodies that are a sequence (`seq`, `let`, `local`) | **88 of 970** |
| a chain or `if` that something follows: a non-last element of a `seq`, a binder's value | **11 + 20** |

Of the 42 `case` forms, 38 are two-armed on `result` (35) or `option` (3), and in all 38 the failure
arm computes a value of its own. **None
re-builds the failure, which is what `try` does, and none abandons, which is what `expect` does.**
31 of them write the failure arm first.

### 3.3 The holes and misfits, verified

**Two holes: an arm cannot carry what Σ's Arm carries.** Verified:

```lisp
(case r  (result.ok x) (when (> x 0)) 1  (result.ok x) 2  (result.err e) 3)
;; case: every clause is a pattern and a body, so the clause list has an even length; got 7 terms
(loop ((i 0)) (invariant (<= i n))  (< i n) (again (+ i 1))  else i)
;; loop clauses come in pairs — a condition and a result — and the last condition must be `else`
```

Today's three clause forms each carry part of the arm: `case` a pattern and no guard, `loop` a guard
and no pattern, `match` both. No form has a place for a loop's invariant I, which abstract.md's loop
carries and settypes §6.3 needs for the climb above the decided fragment.

**Synonyms.** `cond` and `case` (guards against patterns), and `match` and `loop` (patterns against
guards), are two spellings each of one operator: fit-research's F1, decided away by ADR 0050.

**Structure by parity.** `cond`, `case`, `loop` and `match` read their clauses off a flat list's
parity (fit-research §3.6). Decision 5 says an arm is written as a pair in every notation.

**A dead arm the form requires.** `examples/match/runs.oro` ends with `_ v c → again …`, which matches
every state, and then the `else` ADR 0015 requires, which can never run. A required default and an
exhaustive pattern meet there. With named loop variables (below) the last arm can be the default.

**And one bug found while probing**, flagged as a separate task: `(if c a)`, an `if` with no else, is
accepted by the reader and overflows the stack in `gen`. abstract.md's if has three operands; the
reader falls through to an ordinary application.

### 3.4 Findings

**Finding 1. One arm, two eliminators, and named state.** Σ's Arm is `p ⟨if φ⟩ → body` in both case
and loop. So every candidate writes one arm grammar, used by `case` (with a scrutinee, or with none
for guards) and by `loop` (whose arms see the loop variables by name). Two consequences:
- **`cond` and `match` need no keyword.** A case with no scrutinee is the guarded choice (Dijkstra's
  `if … fi`), and a loop's arms are its state machine.
- **A literal pattern on a named state is an equality guard**: ⟨0, _, _⟩ against ⟨st, v, c⟩ is
  `st = 0`. `match` needed positional patterns because its state was anonymous. With named state
  binders, a loop's patterns earn their place only on a constructor (a state that is a sum), and the
  2 `match` sites become guards (C7).

**Finding 2. The commonest choice in the corpus is a handled bind.** 38 of 42 `case` forms are a
two-armed choice on a sum whose success arm is the rest of the computation and whose failure arm is
a value. Written as a case, each one costs a nesting level: `roundtrip` nests two, and every reader of
a file with three such steps reads a staircase. The algebra names it. Let

```
bind(p; e; ā; b)  :=  case(e; p → b, ā)          the success arm hoisted, the other arms ā a handler
```

Then `try` and `expect` are its instances: `try` is bind with ā re-building each other constructor,
the exception monad's bind (errors.md §5), and `expect` is bind with ā abandoning (errors.md §8). So
the derived operator is not a fourth form beside them. It is the general one, and the two names stay
because each names a law. In a notation it is a refutable binder with an `else`:
`ok(src) = os.ReadFile(path) else err(k) → h`, then the rest of the scope.

The literature reached the same form from the same pressure. Rust's `let … else` (RFC 3137, stable
since 1.65) binds a refutable pattern or runs a diverging block, and Swift's `guard let … else` does
the same. Ours need not diverge: the handler's value is the scope's value, which is what `case`
already means.

*In a loop's arm*, a bind whose success continues to `again` and whose handler exits is the shape
ADR 0041 admits for `try` and `expect` ("a success arm is a tail; the other arms leave the loop").
Admitting it for a handler that exits with a value extends ADR 0041 along its own line, and would
need its own decision (§3.9).

**Finding 3. `again` is an application, so its arguments are positional.** D maps a loop to
`(loop (fn (x̄) arms) z̄)`, and `again(ā)` applies that λ: the faithful image is a positional
argument list, its arity checked by W3. Two other readings exist:
- **named and complete**, Dijkstra's concurrent assignment followed by repetition, `i, s := i + 1,
  s + xs[i]`: no argument can land on the wrong variable;
- **named and partial**: a variable not named keeps its value.

Measured: under ADR 0052, 630 arguments, 121 of them a variable unchanged (19%). 151 of 181 loops
have two or more variables, where two arguments of one sort can be swapped silently in a positional
list. Partial naming would save the 121 and lose the property that every back edge decides every
variable: a forgotten update would keep its value rather than fail. Positional keeps Σ's reading and
the arity check, and the swap is the one failure it does not catch.

> **ADR 0055 settles which pair this is** (hamza, 2026-10-12). The two readings go in pairs: loop
> variables as parameters with `again` an application (positional), or loop variables as state with
> `again` a concurrent assignment (named). A loop's variables cannot be written, so they are
> parameters, bound by application and written `=` and `==`, and `again` is positional by the same
> derivation.

**Finding 4. Where a sequence meets a chain, one precedence must decide, and installment 2 used two.**
In D and K a keyword form extends as far right as it can. §2.9 argued K was safe because "`;` is the
loosest operator, so `;` terminates a munching keyword form". But K's own loops put sequences inside
arms (`j < n → c[j] := TRUE ; AGAIN(j + i)`), and under that rule the arm, and the loop, would end at
the first `;`. D's P5 had the same shape; D's P6 parenthesized its arm. So installment 2's examples
were written under two rules, and §2.9's "a statement's extent: free" was wrong. The choice:

| | rule | what must be parenthesized | measured |
|---|---|---|---:|
| **(a)** | **`;` binds tighter than `[]`**: an arm's body may be a sequence, and a chain extends to the end of what encloses it (Dijkstra's guarded commands, without `od`) | a chain or `if` that something follows, and a nested chain | **31 + 22** |
| (b) | `;` loosest, as §2.9 said | every arm body that is a sequence, and a nested chain | 88 + 22 |
| (c) | a closing word per chain (Algol 68's `fi` `od` `esac`, Ada's `end`) | nothing, at one word per chain | 393 words |
| (d) | braces (J) | nothing: every chain and every multi-statement arm is braced | — |

**(a) is the cheapest by the corpus, and it is the precedence of the guarded commands D and K are
built on.** Under it a misplaced chain is mostly a sort error, not a silent change: a statement that
follows an arm's `AGAIN` puts the `again` in a non-tail position, which W3 refuses. A value-ended
arm swallowing what follows is the case that would parse, and that is the 31 the rule makes write
parentheses. §1.11 measured H1 at operand positions only and missed this one, the statement-level
twin of the dangling body. Installment 2's D and K examples are rewritten under (a) (§2.4, §2.9).

**Finding 5. A state binder's introduction should be marked, in a statement list.** §2.13 left it
open; arms are where it bites, because an arm's body is a sequence that may introduce a binder and
write cells. Σ has two operations, the binder local(x := e; …) and the write assign(x; e). Spelled
alike and told apart by whether x is bound, they are a homonym (F2) with Python's failure: a write
with a misspelled name creates a cell. Marking costs one word per introduction:
- **84** state binders in scopes (71 buffers, 13 cells) need the mark;
- a loop's variables need none: they are not state, but bound by application (ADR 0055);
- marking the writes instead would cost more, 123 (9 cell writes and 114 stores), and would go
  against the tradition: Algol 68 marks a declaration by its mode, Pascal and Ada by `var` or a type,
  Rust by `let mut`, Swift by `var`, JavaScript by `let`.

So K writes `VAR x := 0` and J writes `var x := 0`, and a write stays `x := e`. In D and L the group
position already marks it. A definitional binder needs no mark: `=` in J and `==` in K only bind, and
rebinding a cell by one is refused (ADR 0054). That also catches the Go habit `count = count + 1`
meant as a write.

**Finding 6. A one-armed `if` is cheap in J and costly in D and K.** if(c; a; tuple()) with a : 1 is
the statement `if c { a }`: 4 sites. In J the braces bound it. In D and K a one-armed IF brings back
Algol 60's dangling else, `IF a THEN IF b THEN x ELSE y`, which Algol 60 resolved by forbidding an
`if` directly after `then`. So D and K keep `ELSE ()`.

### 3.5 Eight programs, five notations

**T** today, **B′** sectioned prefix, **D** mathematical, **K** statements with no bracket, **J**
Go-inspired. L is not carried: §2.9 measured it the worse corner on both axes. Every program is
written under Finding 4's rule (a) and Finding 5's mark. T uses the language's own operators where the
source file uses a host's (`*` for `go.*`), so the comparison is of notations, not of targets.

---

**C1 — a guarded choice**: the sign of n. Today's `cond`.

```lisp
(cond (< n 0) -1  (= n 0) 0  else 1)                                            ; T
```
```lisp
(case ((< n 0) → -1) ((= n 0) → 0) (else → 1))                                  ; B′
```
```
CASE n < 0 → -1 [] n = 0 → 0 [] OTHER → 1                                       \\ D
```
```
CASE n < 0 → -1 [] n = 0 → 0 [] OTHER → 1                                       \\ K
```
```go
case { n < 0 -> -1; n == 0 -> 0; else -> 1 }                                    // J
```

---

**C2 — a choice on a variant** (`examples/io/roundtrip.oro`'s `kind-name`).

```lisp
(case k                                                                         ; T
  os.not-found   "not-found"
  os.exists      "exists"
  os.permission  "permission"
  (os.other e)   "other")
```
```lisp
(case k                                                                         ; B′
  (os.not-found → "not-found")
  (os.exists → "exists")
  (os.permission → "permission")
  ((os.other e) → "other"))
```
```
CASE k OF os.not_found  → "not-found"                                           \\ D
       [] os.exists     → "exists"
       [] os.permission → "permission"
       [] os.other(e)   → "other"
```
```
CASE k OF os.not_found  → "not-found"                                           \\ K
       [] os.exists     → "exists"
       [] os.permission → "permission"
       [] os.other(e)   → "other"
```
```go
case k {                                                                        // J
    os.not_found  -> "not-found"
    os.exists     -> "exists"
    os.permission -> "permission"
    os.other(e)   -> "other"
}
```

---

**C3 — two handled binds** (`roundtrip`'s `main`, abridged): the corpus's commonest choice, Finding 2.
T nests one level per step; the others hoist the success arm. D and J are in the form ADR 0057
decided (§3.14): a group of binders, each handler next to its call, then the body.

```lisp
(case (os.WriteFile path data)                                                  ; T
  (result.err k)  (seq (io.print-line (kind-name k)) 1)
  (result.ok _)
    (case (os.ReadFile path)
      (result.err k)   (seq (io.print-line (kind-name k)) 1)
      (result.ok src)  (seq (io.print (os.text-of src)) 0)))
```
```lisp
(let (((result.ok _) = (os.WriteFile path data)                                 ; B′
       (else ((result.err k) → (seq (io.print-line (kind-name k)) 1))))
      ((result.ok src) = (os.ReadFile path)
       (else ((result.err k) → (seq (io.print-line (kind-name k)) 1)))))
  (seq (io.print (os.text-of src)) 0))
```
```
TRY ok(_)   == os.WriteFile(path, data) ELSE (err(k) → print_line(kind_name(k)) ; 1),   \\ D
    ok(src) == os.ReadFile(path)        ELSE (err(k) → print_line(kind_name(k)) ; 1)
IN  print(os.text_of(src)) ; 0
```
```
ok(_) == os.WriteFile(path, data) ELSE (err(k) → print_line(kind_name(k)) ; 1) ;      \\ K
ok(src) == os.ReadFile(path) ELSE (err(k) → print_line(kind_name(k)) ; 1) ;
print(os.text_of(src)) ; 0
```
```go
try ok(_)   = os.WriteFile(path, data) else err(k) -> { print_line(kind_name(k)); 1 },   // J
    ok(src) = os.ReadFile(path)        else err(k) -> { print_line(kind_name(k)); 1 }
{
    print(os.text_of(src)); 0
}
```

---

**C4 — a loop whose tail binds with `try`** (`gauntlet/differential/cases/sum-slots.oro`'s `walk`,
ADR 0041).

```lisp
(loop ((i 0) (acc 0))                                                           ; T
  (> i 5)  (fine acc)
  else     (try (fine x) (step i n)
             (again (+ i 1) (+ acc x))))
```
```lisp
(loop ((i = 0) (acc = 0))                                                     ; B′
  ((> i 5) → (fine acc))
  (else → (try ((fine x) = (step i n)) (again (+ i 1) (+ acc x)))))
```
```
LOOP i == 0, acc == 0 :                                                         \\ D
    i > 5  → fine(acc)
 [] OTHER  → TRY fine(x) == step(i, n) IN AGAIN(i + 1, acc + x)
```
```
LOOP i == 0, acc == 0 :                                                         \\ K
    i > 5  → fine(acc)
 [] OTHER  → TRY fine(x) == step(i, n) ; AGAIN(i + 1, acc + x)
```
```go
loop i = 0, acc = 0 {                                                         // J
    i > 5 -> fine(acc)
    else  -> try fine(x) = step(i, n) { again(i + 1, acc + x) }
}
```

---

**C5 — a search with two exits**: the first index whose element is at least k, or -1.

```lisp
(loop ((i 0))                                                                   ; T
  (>= i (len xs))  -1
  (>= (xs i) k)    i
  else             (again (+ i 1)))
```
```lisp
(loop ((i = 0))                                                                ; B′
  ((>= i (len xs)) → -1)
  ((>= (at xs i) k) → i)
  (else → (again (+ i 1))))
```
```
LOOP i == 0 :                                                                   \\ D
    i ≥ Len(xs) → -1
 [] xs[i] ≥ k   → i
 [] OTHER       → AGAIN(i + 1)
```
```
LOOP i == 0 :                                                                   \\ K
    i ≥ Len(xs) → -1
 [] xs[i] ≥ k   → i
 [] OTHER       → AGAIN(i + 1)
```
```go
loop i = 0 {                                                                   // J
    i >= len(xs) -> -1
    xs[i] >= k   -> i
    else         -> again(i + 1)
}
```

---

**C6 — cells written in an arm, a one-armed choice and a nested chain** (`examples/io/count.oro`'s
inner loop, abridged). The nested `cond` is an H2 site, and the `if` is followed by more (Finding 4).

```lisp
(local lines 0  words 0  in-word false                                          ; T
  (loop ((i 0))
    (>= i (len src))  (tuple lines words)
    else
      (let b (src i)
        (seq (if (= b 10) (set lines (+ lines 1)) (tuple))
             (cond (space? b)  (set in-word false)
                   in-word     (tuple)
                   else        (seq (set in-word true) (set words (+ words 1))))
             (again (+ i 1))))))
```
```lisp
(let ((lines := 0) (words := 0) (in-word := false))                             ; B′
  (loop ((i = 0))
    ((>= i (len src)) → (tuple lines words))
    (else → (let ((b = (at src i)))
              (seq (if (= b 10) (lines := (+ lines 1)) (tuple))
                   (case ((space? b) → (in-word := false))
                         (in-word → (tuple))
                         (else → (seq (in-word := true) (words := (+ words 1)))))
                   (again (+ i 1)))))))
```
```
LET lines := 0  words := 0  in_word := FALSE IN                                 \\ D
  LOOP i == 0 :
      i ≥ Len(src) → ⟨lines, words⟩
   [] OTHER        → LET b == src[i] IN
                       (IF b = 10 THEN lines := lines + 1 ELSE ()) ;
                       (CASE space(b) → in_word := FALSE
                          [] in_word  → ()
                          [] OTHER    → in_word := TRUE ; words := words + 1) ;
                       AGAIN(i + 1)
```
```
VAR lines := 0 ; VAR words := 0 ; VAR in_word := FALSE ;                        \\ K
LOOP i == 0 :
    i ≥ Len(src) → ⟨lines, words⟩
 [] OTHER        → b == src[i] ;
                   (IF b = 10 THEN lines := lines + 1 ELSE ()) ;
                   (CASE space(b) → in_word := FALSE
                      [] in_word  → ()
                      [] OTHER    → in_word := TRUE ; words := words + 1) ;
                   AGAIN(i + 1)
```
```go
{ var lines := 0;  var words := 0;  var in_word := false                        // J
  loop i = 0 {
      i >= len(src) -> (lines, words)
      else -> { b = src[i]
                if b == 10 { lines := lines + 1 }
                case {
                    space(b) -> in_word := false
                    in_word  -> ()
                    else     -> { in_word := true; words := words + 1 }
                }
                again(i + 1) } } }
```

---

**C7 — a state machine** (`examples/match/runs.oro`, the 1-runs of n's binary representation). T's
`match` has anonymous state and positional patterns; the others name the state and write guards
(Finding 1), so the dead `else` goes.

```lisp
(match (0 n 0)                                                                  ; T
  _ 0 c                          c
  0 v c (when (= (% v 2) 1))     (again 1 (/ v 2) (+ c 1))
  _ v c (when (= (% v 2) 1))     (again 1 (/ v 2) c)
  _ v c                          (again 0 (/ v 2) c)
  else                           0)
```
```lisp
(loop ((st = 0) (v = n) (c = 0))                                             ; B′
  ((= v 0) → c)
  ((and (= st 0) (= (% v 2) 1)) → (again 1 (/ v 2) (+ c 1)))
  ((= (% v 2) 1) → (again 1 (/ v 2) c))
  (else → (again 0 (/ v 2) c)))
```
```
LOOP st == 0, v == n, c == 0 :                                                  \\ D
    v = 0               → c
 [] st = 0 ∧ v % 2 = 1  → AGAIN(1, v ÷ 2, c + 1)
 [] v % 2 = 1           → AGAIN(1, v ÷ 2, c)
 [] OTHER               → AGAIN(0, v ÷ 2, c)
```
```
LOOP st == 0, v == n, c == 0 :                                                  \\ K
    v = 0               → c
 [] st = 0 ∧ v % 2 = 1  → AGAIN(1, v ÷ 2, c + 1)
 [] v % 2 = 1           → AGAIN(1, v ÷ 2, c)
 [] OTHER               → AGAIN(0, v ÷ 2, c)
```
```go
loop st = 0, v = n, c = 0 {                                                  // J
    v == 0                 -> c
    st == 0 && v % 2 == 1  -> again(1, v / 2, c + 1)
    v % 2 == 1             -> again(1, v / 2, c)
    else                   -> again(0, v / 2, c)
}
```

---

**C8 — nested loops and a borrowed buffer** (`examples/table/sieve.oro`'s `cross` and `sieve`). In
program order `cross` borrows the buffer and writes it in place (ADR 0052), so neither loop carries it.

```lisp
(def cross (c i n)                                                              ; T
  (loop ((c c) (j (* i i)))
    (< j n)  (again (set c j true) (+ j i))
    else     c))

(def sieve (n)
  (local c (table n false)
    (loop ((c c) (i 2))
      (>= (* i i) n)  c
      (c i)           (again c (+ i 1))
      else            (again (cross c i n) (+ i 1)))))
```
```lisp
(fn cross ((c (array bool) linear) i n) ()                                      ; B′
  (loop ((j = (* i i)))
    ((< j n) → (seq ((at c j) := true) (again (+ j i))))
    (else → (tuple))))

(fn sieve (n)
  (let ((c := (table n false)))
    (loop ((i = 2))
      ((>= (* i i) n) → c)
      ((at c i) → (again (+ i 1)))
      (else → (seq (cross c i n) (again (+ i 1)))))))
```
```
cross(c ∈ Seq(Bool) LINEAR, i, n) ==                                            \\ D
  LOOP j == i * i :
      j < n  → c[j] := TRUE ; AGAIN(j + i)
   [] OTHER  → ()

sieve(n) ==
  LET c := [k ∈ 0..n-1 ↦ FALSE] IN
    LOOP i == 2 :
        i * i ≥ n → c
     [] c[i]      → AGAIN(i + 1)
     [] OTHER     → cross(c, i, n) ; AGAIN(i + 1)
```
```
cross(c ∈ Seq(Bool) LINEAR, i, n) ==                                            \\ K
  LOOP j == i * i :
      j < n  → c[j] := TRUE ; AGAIN(j + i)
   [] OTHER  → ()

sieve(n) ==
  VAR c := [k ∈ 0..n-1 ↦ FALSE] ;
  LOOP i == 2 :
      i * i ≥ n → c
   [] c[i]      → AGAIN(i + 1)
   [] OTHER     → cross(c, i, n) ; AGAIN(i + 1)
```
```go
fn cross(c ∈ Seq(Bool) linear, i, n) () {                                       // J
    loop j = i * i {
        j < n -> { c[j] := true; again(j + i) }
        else  -> ()
    }
}

fn sieve(n) {
    var c := table(n, false)
    loop i = 2 {
        i * i >= n -> c
        c[i]       -> again(i + 1)
        else       -> { cross(c, i, n); again(i + 1) }
    }
}
```

### 3.6 Measured

`go run gauntlet/surface/tokens.go -section 3`, over C1–C8, by the rule in its header:

| notation | tokens | × T | lexemes | × T | depth | closing run | lines | holes |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| T today | 487 | 1.00 | 483 | 1.00 | 8 | 6 | 46 | **2** (a case arm's guard, a loop's invariant) |
| B′ | 641 | 1.32 | 628 | 1.30 | **10** | 7 | 42 | 0 |
| D | 488 | 1.00 | 463 | 0.96 | **3** | 2 | 39 | 0 |
| K | 490 | 1.01 | 465 | 0.96 | **3** | 2 | 39 | 0 |
| J | 495 | 1.02 | 445 | **0.92** | 5 | 4 | 52 | 0 |

- **On choice and iteration the new notations cost nothing.** D, K and J are today's size in
  characters (1.00–1.02×) and smaller in lexemes (0.92–0.96×), where installment 2 measured them 1–4%
  larger. Two findings pay for the binder operators: the handled bind (C3) removes a level and an arm
  per step, and named state (C7) removes `match`'s positional patterns and its dead `else`.
- **Depth falls from 8 to 3** in D and K, and to 5 in J, whose braces cost a level per chain.
- **D and K are almost one notation here.** They differ only where a scope introduces state (C3, C6,
  C8), so this installment does not separate them; installment 2 did, on the binder axis.
- **J writes the fewest lexemes and the most lines**: its `->`, `==` and `&&` are one lexeme each, and
  each closing brace takes a line.
- **B′ is the largest and the deepest for the third installment running**: 1.30× today in lexemes and
  depth 10, two levels *deeper* than the notation it would replace. Writing an arm as a pair, which
  decision 5 requires, costs it a bracket per arm.

### 3.7 Kept, and dropped

**The installment's result is structural and holds in every candidate**: one arm grammar for both
eliminators, `cond` and `match` retired as keywords, the handled bind as the general form of `try`
and `expect`, one precedence for `;` inside arms, and a marked introduction for state. That is where
the two holes, the two synonyms and the parity reading go.

**Kept: D, K and J.** They are within 2% of each other in characters, and they differ in what this
installment does not test: D and K only in how a scope introduces state, and J in its delimiters,
with the depth (5 against 3) and the lines (52 against 39) that braces cost.

**B′ should be dropped as a source candidate.** §2.6 said this installment would rescue or drop it.
It is 1.30–1.47× today's size and deeper than today in both installments that measured it, and
writing the semantics faithfully (arms as pairs, indexing marked) is what made it so. What it offered,
a reader that exists and a failure that is syntactic, D, K and J now have by their own rules
(Finding 4, the braces). **S-expressions are not dropped**: they remain abstract.md's T′, the
rendering for tools and for interchange, and today's notation until the translator replaces it.
That is candidate I's question, one notation or two, and it stays open.

### 3.8 What installment 3 fixes for everything after it

1. **One arm grammar**, `p ⟨if φ⟩ → body`, for `case` and `loop`. A `case` with no scrutinee is the
   guarded choice, and a loop's arms are its state machine, so `cond` and `match` are not keywords.
   `if` stays, derived, because it names 𝔹's eliminator (ADR 0017).
2. **Named state.** A loop's arms see its variables by name, and a pattern on the state earns its
   place only on a constructor.
3. **The handled bind**, a refutable binder with an `else`, `p = e else ā`, is bind(p; e; ā; b) =
   case(e; p → b, ā). `try` and `expect` are its instances and keep their names.
4. **`again` is positional**, the application D makes it, its arity checked by W3.
5. **One precedence where a sequence meets a chain** (Finding 4, rule (a)): in D and K, `;` binds
   tighter than `[]`; an arm's body may be a sequence; a chain extends to the end of what encloses
   it; a chain or `IF` that something follows, and a chain nested in an arm, is parenthesized.
6. **A state binder's introduction is marked** in a statement list: `VAR x := e` in K, `var x := e`
   in J; a write stays `x := e`. A loop's variables are not state: they are bound by application
   (ADR 0055), written `LOOP i == 0` in D and K and `loop i = 0` in J, and cannot be written.
7. **A one-armed `if`** is the statement if(c; a; ()), written only where braces bound it (J).
8. **A loop's invariant has a slot** in every notation: `LOOP i == 0 INVARIANT I : …` in D and K,
   `loop i = 0 invariant I { … }` in J, as abstract.md §9.4's M′ writes it.

### 3.9 Open, for hamza

> **Decided by hamza, 2026-10-12**, as [ADR 0056](decisions/0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md):
> items 1, 3, 4, 5, 6 and 8, and **the source notation is D or J** (*"in fact moving forward it is D or
> J"*), so K is dropped with B′. Item 4 applies to D, and item 5 to J (`var`), D marking a state
> binder by its place in a `LET` group. Items 2, 2b and 7 were explained in §3.11–3.14 and then
> decided: 7 as ADR 0056's amendment, 2 and 2b as [ADR 0057](decisions/0057-the-handled-bind-and-its-tail.md).

1. **One arm grammar, and `cond` and `match` retired as keywords** (§3.8, 1–2). *Recommended*: it is
   ADR 0050's decision 5 made concrete, and it closes both holes of §3.3.
2. **The handled bind** as a derived operator, with `try` and `expect` its instances. *Recommended*:
   38 of 42 `case` forms are one, and each costs a nesting level today.
   - **2b. In a loop's arm**, a handled bind whose success continues to `again` and whose handler
     exits with a value. *Recommended*: it is ADR 0041's reasoning (the other arms leave the loop)
     applied to a handler. It changes the tail sort, so it needs its own ADR.
3. **`again` positional** (*recommended*: Σ's application and W3's arity check), or named: complete,
   as Dijkstra's concurrent assignment, or partial. 121 of 630 arguments are a variable unchanged.
4. **Rule (a) for `;` inside arms** in D and K (*recommended*: 31 + 22 parenthesized sites, the
   guarded commands' own precedence), against (b) `;` loosest (88 + 22) or (c) a closing word per
   chain (393 words). J and B′ are unaffected.
5. **Mark a state binder's introduction** in K and J, `VAR`/`var` (*recommended*: 84 marks against
   123 for marking writes, and Algol 68's, Pascal's, Ada's and Rust's answer). This closes §2.13's
   open item 4. D and L mark it by position.
6. **A one-armed `if` in J only** (*recommended*: in D and K it brings back the dangling else).
7. **A loop's invariant slot** (*recommended*: abstract.md's loop has it, and no notation today can
   write one).
8. **Drop B′ as a source candidate**, keeping S-expressions as T′ for tools and interchange
   (*recommended*: §3.7).

Not here: whether the source is D or J, and whether there are two notations. Installments 4–8
(data and sets, claims, declarations, static parameters and the arrow, lexical) come first.

### 3.10 One former for arrays, and how many shapes J has

hamza, after installment 3: *"in the sake of having a uniform syntax, where the programmer does not
have to learn many shapes … maybe we should have the literal array as `array { e1, e2, e3 … }` and
`array(n, f)`, that way we lose a keyword. … array comprehension `[k ∈ 0..n-1 ↦ false]`, keep it
around for now we might need it."*

#### What the two constructors are

A table is a function on [0, n) (tables.md §1), and a finite function has two presentations, the two
by which mathematics gives any finite object:
- **by its graph**, an enumeration: `(array e₀ … eₖ₋₁)`, the image of a tuple under Vᵏ ≅ [0, k) → V;
- **by a rule**: `(table n f)`, the tabulation of f : [0, n) → V, which makes a function into data.

A set has the same pair, extension `{a, b}` and separation `{x ∈ S : φ}` (Zermelo). The comprehension
`[k ∈ 0..n-1 ↦ e]` is the rule with its λ written in place: it is `table(n; λk. e)`, which is how
abstract.md §9.4's M′ prints table(n; f).

#### Measured

`go run gauntlet/surface/data.go`, over the same corpus:

| | count |
|---|---:|
| tables by their graph, `(array e…)` | 47 (one element: 5; two: 3) |
| tables by a rule, `(table n f)` | 77 |
| … f the zero, a zero-filled buffer | **67** |
| … f a λ | 8 |
| … f another literal, f a name (a thaw) | 1, 1 |
| **`(array T)` as a type**, spelled as a one-element graph is | **38** |

The last row is a homonym in today's notation: `(array f64)` is a set and `(array 93)` is a value,
told apart only by whether the argument names a set.

#### For the proposal

1. **J cannot use bare braces for a collection.** In J, `{ … }` is a block whose value is its last
   expression, so `{x}` would be both a block and a singleton set. ADR 0050's decision 4 needs a set
   former, so in J it must have a head, `set { 1, 2 }`. Arrays then follow the same rule, and so do
   maps (installment 4): **every finite collection given by its elements has one shape,
   `former { e, … }`**. Each is the graph of a finite function: an array's keys are implicit, 0..k−1;
   a map's are written, `k ↦ v`; a set is the graph of its characteristic function. A bare `{ … }` is
   always a block.
2. **It separates the value from the set.** `Seq(S)` names the set of sequences over S, as
   installment 1's examples already write it, and `array { … }` builds one value. The 38-site
   homonym goes, and that is ADR 0049's two languages showing in the surface: the set is in the logic,
   the value in the computation.
3. **One name instead of two**: `table` goes, and `array` names the sort's constructors, as Go's
   composite literal `[]int{1, 2, 3}` and `make` do. Size does not move: `array{1, 2}` and
   `array(1, 2)` are the same tokens, and so are `array(n, 0)` and `table(n, 0)`.
4. **It costs J no shape.** `former { contents }` is the shape `case x { arms }` and `loop … { arms }`
   already have, with an empty header. Braces delimit, and the head says what is inside: arms after a
   chain keyword, elements after a former, statements after nothing. That is S-expressions' rule,
   `(head args)` with the head deciding, carried into a notation with infix.

#### Against it, and the one hazard

**`array(n, v)` and `array{a, b}` differ by one bracket, and both are well-typed.** `array(3, 0)` is
three zeros, `array{3, 0}` is the pair. In the corpus the rule with a literal is 68 sites and the
two-element graph is 3, so the confusable form is the commonest constructor of all. Rust has the same
pair, `[0; 3]` against `[0, 3]`, and lives with it. It is not the failure class §1.7 disqualifies (a
body's extent misread), but it is a one-character difference between two well-formed programs. Three
answers:
- **accept it**, as Rust does (*recommended for now*): the brackets are visually distinct, and the
  length usually decides an index proof;
- **write every rule in braces with its binder**, `array { k ∈ 0..n-1 ↦ 0 }`, the comprehension
  moved inside the former, which makes the two presentations look as different as `set {a, b}` and
  `set {x ∈ S : φ}`. It costs about 8 tokens at each of 67 zero-filled buffers;
- **keep a second name** for the rule. That is today's `table`, which the proposal is for losing.

**The comprehension and `array(n, f)` are two spellings of one operator** (fit-research's F1):
`[k ∈ 0..n-1 ↦ e]` is `array(n, fn(k) { e })`. Keeping both is right only while something else
decides between them, and something does: notation-research's unification of tables and maps as one
finite function. `[k ∈ D ↦ e]` covers a map's domain, and `array(n, f)` cannot. That is installment 4's
question, so the comprehension stays, as hamza says, and is decided there.

**D and K do not need any of this.** Their lexicon is TLA+'s, whose braces are free: `⟨1, 2, 3⟩` is
the graph, `[k ∈ 0..n-1 ↦ e]` the rule, `{1, 2}` the set. The proposal is J's, forced by J's use of
braces for blocks.

#### J's shapes, counted

What a reader of J must learn, after the proposal:

| | shape | carries |
|---|---|---|
| 1 | a name or a literal | |
| 2 | `a op b`, `op a` | arithmetic, comparison, `∈`, `..`, `&&` |
| 3 | `f(a, …)` | application, including `again(…)`, `array(n, f)` and a host call |
| 4 | `t[i]` | indexing (ADR 0050, decision 6) |
| 5 | `(a, b)`, `(e)` | a tuple, grouping |
| 6 | `head … { contents }` | `fn`, `case`, `loop`, `if … else`, a former; a bare `{ … }` is a block |
| 7 | `p [if φ] -> e` | an arm, inside a chain |
| 8 | `p = e`, `var x := e`, `lhs := e`, and `p = e else arms`, `try p = e`, `expect p = e else w` | a statement: a binder, a write, a bind |
| 9 | `[k ∈ D ↦ e]` | the rule with its binder (for now) |
| 10 | a header's claims, `(x ∈ S, …) (r ∈ R) where φ ensure ψ` | a definition's claim |

Ten shapes, of which 2–5 are what every infix language has and 6–8 carry the semantics' own
operators. The proposal moves array literals from shape 3 (`array(1, 2, 3)`) to shape 6 and removes
one name. Whether shape 9 survives is installment 4's.

#### Proposed, and decided

**All four decided by hamza, 2026-10-12** (ADR 0056).

1. **In J, a finite collection given by its elements is `former { e, … }`**: `array { … }` now,
   `set { … }` for ADR 0050's set former, `map { k ↦ v, … }` in installment 4.
2. **The rule is `array(n, f)`**, and `table` goes, accepting the one-bracket hazard as Rust does.
3. **The set is `Seq(S)`**, never `array(S)`, so a value and its set are never spelled alike.
4. **`[k ∈ D ↦ e]` stays**, to be decided with tables-and-maps-as-one-function in installment 4.

### 3.11 Two explanations, at hamza's request: the bind and the invariant

hamza, on §3.9: *"2. mostly agree, I need more explaining … what does it mean? what does it compile
to? … 7. explain it some more."*

#### `try`, and the handled bind it is an instance of

**The program.** `gauntlet/differential/cases/sum-slots.oro`, reduced to the part C4 shows:

```lisp
(variant (outcome T E) (fine T) (failed E) (success fine))

(def step (i n)
  (if (< i n) (fine i) (failed (array i (+ 1000 i)))))

(def walk (n)
  (loop ((i 0) (acc 0))
    (> i 5)   (fine acc)
    else      (try (fine x) (step i n)
                (again (+ i 1) (+ acc x)))))
```

`outcome` is a sum, T + E, with `fine` marked as its success (ADR 0040). `step` succeeds while i < n
and fails otherwise. `walk` adds up `step`'s successes for i = 0 … 5 and returns `fine(sum)`; the
first failure stops the loop, and `walk`'s value is that failure.

**What `try` means.** abstract.md §2.7 defines it by one equation:

```
try(s(x̄); e; b)  =  case(e; s(x̄) → b,  c₁(ȳ₁) → c₁(ȳ₁), …)        the cᵢ the variant's other constructors
```

Look at e. If it is the success, bind its payload and go on with b. If it is anything else, that value,
unchanged, is the value of the whole form. That is the exception monad's bind (Moggi 1991), and in
other languages it is Rust's `?`, `let x = step(i, n)?;`, and Haskell's `x <- step i n` in a `do`
block over `Either`. In `walk`, written out as a `case`:

```go
loop i = 0, acc = 0 {
    i > 5 -> fine(acc)
    else  -> case step(i, n) {
                 fine(x)   -> again(i + 1, acc + x)
                 failed(e) -> failed(e)            // the loop's value: it leaves the loop
             }
}
```

The `case` here has an `again` in one arm, which the tail rule otherwise forbids. ADR 0041 admits it
for exactly this shape: the success arm continues the loop, and every other arm leaves it with a
value. The other arm's value becomes the loop's, so the loop yields a sum.

**What it compiles to**, verified. The reducer inlines `step`, and `case` of a known `if` of
constructors resolves by case-of-case, so the `case` disappears:

```lisp
(loop (fn (i acc)
        (if (> i 5) (fine acc)
            (if (< i n) (again (+ i 1) (+ acc i))
                (failed (array i (+ 1000 i))))))
      0 0)
```

and Go receives one loop that returns the sum as its tag and slots (ADR 0042):

```go
func GenWalk(v0 int) (int, int, []int) {
	var v14 int = 0
	var v15 int = 0
	var v11 int
	var v12 int
	var v13 []int
	for ; ; v14 = (v14 + 1) {
		v17 := (v14 > 5)
		if v17 {
			v11, v12, v13 = 0, v15, nil        // fine(acc): tag 0, the int slot
			break
		}
		v22 := (v14 < v0)
		if v22 {
			v27 := (v15 + v14)
			v15 = v27
			continue                         // again(i + 1, acc + i)
		}
		v33 := (1000 + v14)
		v34 := []int{v14, v33}
		v11, v12, v13 = 1, 0, v34            // failed(…): tag 1, the table slot
		break
	}
	return v11, v12, v13
}
```

No sum is built inside the loop and nothing is allocated for the success. `try` cost nothing; it is
the third clause.

**The handled bind** is the same equation with the other arms written by hand instead of re-built:

```
bind(p; e; ā; b)  =  case(e; p → b, ā)
```

`try` is bind with ā re-building each failure, and `expect` is bind with ā abandoning. The corpus's
commonest choice (38 of 42 `case` forms) is a bind whose ā is the program's own handler, as in
`roundtrip`:

```go
ok(src) = os.ReadFile(path) else err(k) -> { print_line(kind_name(k)); 1 }
print(os.text_of(src)); 0
```

which means exactly

```go
case os.ReadFile(path) {
    ok(src) -> { print(os.text_of(src)); 0 }
    err(k)  -> { print_line(kind_name(k)); 1 }
}
```

and compiles to what that `case` compiles to today. The surface form only hoists the success arm, so
the rest of the scope is not nested one level per step.

**2b, in a loop's arm.** `try` there is decided (ADR 0041). The question is the same shape with a
handler: `ok(x) = f(i) else err(e) -> h; again(…)`, where the success continues to `again` and the
handler leaves the loop with h's value. It is ADR 0041's reasoning unchanged (one arm continues, the
others leave), but it widens the tail sort, which is semantics, so it would get its own ADR.

#### A loop's invariant

**What it is.** A loop invariant I is a formula over the loop's variables (and the names around the
loop) that holds every time the loop is entered or goes round (Floyd 1967; Hoare's rule for `while`,
1969). It is a claim, in the logic: ε erases it, so it emits nothing and costs nothing at run time.

**Why a program needs to write one.** The analyses infer invariants today (intervals, Houdini's content
facts), and they infer what the decided fragment can: bounds, lengths, linear relations. They cannot
infer a fact that needs induction over a specification. abstract.md §12's example:

```
LOOP i == 0, s == 0 INVARIANT s = sum(xs, 0, i) :                                       \* D
    i ≥ Len(xs) → s
 [] OTHER       → AGAIN(i + 1, s + xs[i])
```
```go
loop i = 0, s = 0 invariant s == sum(xs, 0, i) {                                        // J
    i >= len(xs) -> s
    else         -> again(i + 1, s + xs[i])
}
```

with `sum` a specification function (settypes §6.3), so that `total(xs) ∈ {r : r = sum(xs, 0,
len(xs))}` can be proven. No analysis finds `s = sum(xs, 0, i)` by itself: it is above the cliff
settypes §5 draws, where the programmer climbs by assertions, invariants and lemmas.

**What the compiler does with it.** 𝒪 (abstract.md §7) turns it into three obligations, each decided
or the program refused:
1. **it holds on entry**: I with the initializers, here 0 = sum(xs, 0, 0);
2. **every `again` preserves it**: I, plus the arm's guard and the guards before it that failed,
   implies I at the `again`'s arguments: s = sum(xs, 0, i) ∧ i < len(xs) ⟹ s + xs[i] = sum(xs, 0, i + 1);
3. **every exit may use it**: I and the arm's guard are facts where the value leaves, here
   s = sum(xs, 0, len(xs)), which is the postcondition.

**Why the slot is recommended now, though no program has one.** No program has one because no notation
can write one (§3.3). abstract.md's loop has I, ADR 0049 names invariants as one of the three ways to
climb, and the next step on settypes' plan (types as static values, claims carrying sets) is where
claims start to need them. The slot is optional, one clause after the variables, and absent it means
⊤, which is today.

### 3.12 `try { … }` rather than `{ try … }`

hamza, after §3.11: *"looks to me like it should be try {} rather than {try ... }, because this got
me confused. discuss, argue and propose."*

#### Why `{ try p = e; rest }` confuses

The statement looks as if `try` applied to one line. By its equation it governs more than that:

```
try(s(x̄); e; b)  =  case(e; s(x̄) → b, the other constructors re-built)
```

and in a statement list b is **the rest of the block**. Two things are therefore invisible:
- **the region**: every statement after the `try`, to the closing brace, is its success arm;
- **the destination**: a failure becomes the value of the **innermost enclosing block**. In C4 that
  block is a loop's arm, so the failure leaves the loop (ADR 0041). In
  `x = { try p = e; f(p) }; more` it becomes x's value, not the function's.

The handled bind, `p = e else h; rest`, has the same two hidden things: h's value also becomes the
enclosing block's. **D does not have the problem**: `TRY p == e IN body` writes its region after `IN`,
as `LET` does, and the destination is the `TRY` form itself.

The literature names both halves. Benton and Kennedy ("Exceptional syntax", *JFP* 11(4), 2001) showed
that `try … catch` scopes its handler over the continuation as well as the call, and proposed a
binder whose handler covers only the call: our handled bind is their `let x ⇐ e in b unless h`. Rust's
`?` has an implicit destination, the function. The `try` blocks proposed for it (RFC 243, RFC 2388)
make the destination a written block, which is hamza's proposal.

#### The proposal

**A try block is the delimiter of early exits.** In J:

```go
try {
    p₁ = e₁                         a binder whose pattern may fail: binds, or exits the try block
    p₂ = e₂ else q -> h             with its own handler: binds, or exits with h's value
    …
    v                               the block's value when every binder matched
} else q -> h                       optional: the handler for binders without their own
```

1. **Inside a try block, a binder with a refutable pattern is a bind.** If the pattern matches, it
   binds and the block goes on. If not, the block's value is the failure, re-built (`try`'s law), or
   the binder's own handler's value, or the block's `else` arms' value.
2. **Outside a try block, a refutable binder is refused**, as today's `let` refuses one ("a pattern
   that may not match belongs in `case`", binding.md §5).
3. **A try block covers the binders of its own statement list**, not those of a block nested in it,
   which takes its own `try`. Every early exit then goes exactly one brace out.
4. **`expect p = e else w` stays a statement.** It abandons, so no value goes anywhere and there is no
   destination to show.
5. **In D the same rule reads `TRY … IN`**: `LET` binds irrefutable patterns, and a refutable one,
   with or without `ELSE`, goes in a `TRY` group. So D's C3 is `TRY ok(_) == … ELSE (…), ok(src) == …
   ELSE (…) IN …`.

The abstract syntax does not change. A try block renders a sequence of try and bind nodes, each
binder's success arm the rest of the list, and the block-level `else` is the same arms written once.

#### The examples, before and after

C4, in a loop's arm (the same tokens, the braces moved out):

```go
else -> { try fine(x) = step(i, n); again(i + 1, acc + x) }        // before
else -> try { fine(x) = step(i, n); again(i + 1, acc + x) }        // after
```

C3, two binds with their own handlers (`roundtrip`'s two messages differ):

```go
try {
    ok(_) = os.WriteFile(path, data) else err(k) -> { print_line(kind_name(k)); 1 }
    ok(src) = os.ReadFile(path) else err(k) -> { print_line(kind_name(k)); 1 }
    print(os.text_of(src)); 0
}
```

and with one handler for both, the shape every reader of `try … catch` knows:

```go
try {
    ok(_) = os.WriteFile(path, data)
    ok(src) = os.ReadFile(path)
    print(os.text_of(src)); 0
} else err(k) -> { print_line(kind_name(k)); 1 }
```

#### Against it, and the answers

- **One more level of braces** around the binds. Measured: 38 handled binds, 4 `try`, and 4 of the 38
  hold another choice in their success arm (§3.2), so most try blocks would hold one binder. It costs
  `try {` and `}` per region, and it is the price of a visible region.
- **Benton and Kennedy's objection to a block handler**: it also catches failures in the continuation.
  Here a failure comes only from a refutable binder written in the block's own list (rule 3), never
  from a call deep inside, so everything the block-level `else` covers is visible on the page.
- **Is the block-level `else` a synonym of `case` on the block's value?** No: it handles the binders'
  failures, while the block's normal value passes through untouched. A `case` would have to
  distinguish the two, which needs the normal value wrapped.

#### And 2b

In a loop's arm, `try { ok(x) = f(i) else err(e) -> h; again(…) }` exits the try block, which is the
arm, so h's value leaves the loop. That is §3.9's 2b, ADR 0041's reasoning extended from re-built
failures to handlers. It widens the tail sort, so it is decided with an ADR, and the try block is
what makes it readable: the `again` is visibly the last statement of the block a failure exits.

#### Proposed

1. **J: `try { … }`**, a try block delimiting early exits, with an optional `else` arms after it, in
   place of the `try` statement.
2. **A refutable binder only inside a try block** (J) or a `TRY` group (D); with its own
   `else arms` it is the handled bind (§3.9's item 2), whose value exits the block.
3. **`expect` stays a statement.**
4. **2b**: a try block may be a loop arm's tail, its handlers leaving the loop, by an ADR extending 0041.

### 3.13 The same tree, written three ways (§3.12 revised)

hamza: *"this is entirely different. I am even more confused now … my confused about the difference
between the s-expression having again inside try, and in J as a statement. try to explain it again.
the algebra, the pros, and the cons."*

#### It is one tree

`try` has three slots: a pattern, a value, and a body that runs only if the value matches the
pattern. The tree of C4's arm is

```
try( fine(x);  step(i, n);  again(i + 1, acc + x) )
     pattern   value        body
```

and every notation writes those three slots:

```lisp
(try (fine x) (step i n) (again (+ i 1) (+ acc x)))           ; T: the body is the third argument
```
```
TRY fine(x) == step(i, n) IN AGAIN(i + 1, acc + x)            \* D: the body follows IN
```
```go
{ try fine(x) = step(i, n); again(i + 1, acc + x) }           // J as drafted: the body is the rest of the block
```

**In J's draft the `again` is inside the `try` too.** The `;` after the `try` does not end it. It
means "the rest of this block is my body", which is how every binder in a statement list works:

```lisp
(let x (f 1) (g x))                                           ; T
```
```go
{ x = f(1); g(x) }                                            // J: g(x) is inside x's scope, written after it
```

Installment 2 chose statement lists for this (§2.9, §2.11): a sequence of binders reads top to bottom
instead of nesting one level per binder, and a statement list folds right into exactly the nested
tree (`core/desugar.go`).

#### Why it confuses with `try` and not with `=`

For `x = f(1); g(x)` the two readings of `;` coincide: "do this, then that" and "that is in this
one's scope" give the same program, because `x = f(1)` always succeeds. For `try` they differ.
Its body runs **only if the pattern matches**; otherwise the body is skipped, and the failure is the
value of the whole form, here the arm, so it leaves the loop (ADR 0041). So after a `try`, `;` means
"and if it matched, continue with …", and nothing on the page says so. In T and D the body is
visibly an argument of `try`; in J's draft it is a convention.

#### Three ways to write it in J, one tree

| | J | where the body is | pros | cons |
|---|---|---|---|---|
| **A** | `{ try fine(x) = step(i, n); again(…) }` (the draft) | the rest of the block | flat: a chain of binds reads top to bottom | the body and the destination are a convention; `;` reads as "then" |
| **B** | `try fine(x) = step(i, n) { again(…) }` | in the braces after the binder | **T's three slots, in J's own shape**, `keyword head { body }`, as `fn`, `loop` and `case` have; the destination is the `try` form itself; with a handler it reads like `if … else` | one level of braces per step that cannot be grouped |
| C | `try { fine(x) = step(i, n); again(…) }` (§3.12) | the rest of the block, inside `try { }` | flat, and the block shows where a failure goes | moves the word `try` off the binder onto a region, and needs a new rule (a fallible binder only inside a try block); the tree is the same but the page is not |

**B is the recommendation**, replacing §3.12's C. It is the S-expression's tree written in J's shape:

```go
try fine(x) = step(i, n) { again(i + 1, acc + x) }                       // C4's arm

try ok(src) = os.ReadFile(path) {                                         // a handled bind
    print(os.text_of(src)); 0
} else err(k) -> { print_line(kind_name(k)); 1 }
```

- **Without `else`**, a failure is re-built and is the form's value: `try`'s law.
- **With `else arms`**, a failure goes to the arms: the handled bind, §3.9's item 2. It is the same
  shape as `if c { a } else { b }`: `if` eliminates 𝔹 = 1 + 1, and `try … else` eliminates a sum
  T + E, with the success arm first.
- **Several binders that share one handler are one group**, as a loop's variables are:
  `try ok(_) = os.WriteFile(path, data), ok(src) = os.ReadFile(path) { … } else err(k) -> { … }`.
  That is D's `TRY ok(_) == …, ok(src) == … IN …`, so D and J have the same structure.
- **Steps with different handlers nest**, one level each, as T's `case`s do today. Measured, 4 of 38
  handled binds hold another choice in their success arm (§3.2), so the staircase is rare.
- **2b reads directly**: in a loop's arm, `try ok(x) = f(i) { again(…) } else err(e) -> h` has the
  `again` visibly inside the success braces and the handler visibly outside, which is ADR 0041's shape
  with a handler.
- **`expect` keeps its statement form**, `expect ok(n) = strconv.Atoi(s) else "not a number"`: it
  abandons, so it has no body to show and no value to deliver.

#### Proposed (replacing §3.12's)

1. **J: `try p = e, … { body } [else arms]`**, the binders a group, the body braced, the handler
   after: B.
2. **D: `TRY p == e, … IN body [ELSE arms]`**, unchanged in shape.
3. **The handled bind is `try … else`** (§3.9's item 2); `expect` stays a statement.
4. **2b**: `try … { … again(…) } else arms` as a loop arm's tail, its handlers leaving the loop, by an
   ADR extending 0041.

### 3.14 A against B: the worst of both, and the best

hamza: *"B is good, but let's compare B and A, where could we see the best of both, the worst of
both."*

#### What each gets right

| | A, the statement: `ok(src) = os.ReadFile(p) else err(k) -> h; rest` | B, the braced body: `try ok(src) = os.ReadFile(p) { rest } else err(k) -> h` |
|---|---|---|
| **the region** (what runs on a match) | the rest of the block, by convention | the braces: written |
| **the destination** (where a failure goes) | the enclosing block's value, by convention | the `try` form's value: written |
| **the handler** | next to its call | after the body, as far from its call as the body is long |
| **a chain of steps** | flat, one line each | one level of braces per step |

A is good at **adjacency and flatness**; B is good at **showing the region and the destination**.

#### Measured: the 38 handled binds are two populations

`go run gauntlet/surface/control.go`, the 38 two-armed `case` forms on `result` or `option` whose
failure arm handles it (§3.2):

| | count | example |
|---|---:|---|
| **a step**: the success arm 18 nodes or more and larger than the failure arm | **11**, 10 in tail position | `main` reads its file or prints why not, then the whole program (`count` 115 nodes against 6, `wc` 60 against 6, `roundtrip` 60 and 38 against 11) |
| **a small choice**: both arms a value or two | **27** | `(case (m k) (some v) v none 0)`, `(case r (result.ok _) true (result.err _) false)` |

And there are the 3 `try` sites in a loop's arm (ADR 0041), whose success body is the `again`.

#### The worst of both

1. **B for a step**: the handler of `count`'s `main` lands after 115 nodes of program, and `roundtrip`
   indents a level per step. That is the "pyramid of doom" Swift added `guard` to escape: `if let x
   = e { long } else { short }` puts the short handler far from the call it handles.
2. **A with an unmarked keyword**: `{ try p = e; again(…) }` hides the region and the destination, as
   §3.13 found.
3. **Both under one keyword**: `try p = e; rest` and `try p = e { rest }` would make the reader look
   past the binder to learn whether the body is the braces or the rest of the block.
4. **A small choice written as either**: `ok(v) = m(k) else none -> 0; v` dresses a choice of two
   values as a sequence. The 27 are choices, and `case m(k) { some(v) -> v; none -> 0 }` says so.

#### The best of both: B's region, A's adjacency, in one form

D already has it. A `TRY` group puts each binder's handler next to its call and the body after `IN`,
so the handlers are adjacent, a chain is flat, and the region is written:

```
TRY ok(_)   == os.WriteFile(path, data) ELSE (err(k) → print_line(kind_name(k)) ; 1),
    ok(src) == os.ReadFile(path)        ELSE (err(k) → print_line(kind_name(k)) ; 1)
IN  print(os.text_of(src)) ; 0
```

J takes the same structure, in the shape its definitions already have (installment 1's J: a header
of clauses, then `{` body `}`):

```go
try ok(_)   = os.WriteFile(path, data) else err(k) -> { print_line(kind_name(k)); 1 },
    ok(src) = os.ReadFile(path)        else err(k) -> { print_line(kind_name(k)); 1 }
{
    print(os.text_of(src)); 0
}
```

- **the header is a group of binders**, sequential as `let`'s (ADR 0054), **each with an optional
  handler next to it**: A's adjacency, and a chain of steps is one region, not a staircase;
- **the body is braced**: B's written region, and a failure's destination is the `try` form;
- **an optional handler after the body** serves the binders without their own: the shape of
  `try … catch`, safe here because a failure comes only from a binder in the header (Benton and
  Kennedy's objection does not apply);
- **no handler at all**: a failure passes through re-built, `try`'s law. C4's arm stays
  `try fine(x) = step(i, n) { again(i + 1, acc + x) }`, the `again` visibly inside.

`count`'s `main`, the largest step, then reads

```go
try ok(src) = os.ReadFile(av[1]) else err(k) -> { print_line("count: cannot read that file"); 0 }
{
    … the program …
}
```

with its handler on the first line and its body indented once, against B's handler 115 nodes below
and A's convention.

**What it costs against A**: one level of indentation for the body, once per group rather than per
step. Measured, that is the 11 steps, all in a program's `main`.

**The three layouts, by population:**

| | form | for |
|---|---|---|
| a small choice | `case e { p -> a; q -> b }` | the 27 |
| a step, or several | `try p = e [else arms], … { body } [else arms]` | the 11, and the 3 in loops' arms |
| giving up | `expect p = e else w` | `expect`'s 5 |

One eliminator, case(e; p → b, ā), in each. `try` and `case` differ in emphasis (the success
continues, against two values chosen between) and in the header, as `if` and `case` do.

#### Proposed, and decided

**All four decided by hamza, 2026-10-12** (*"I agree to all 4"*), as
[ADR 0057](decisions/0057-the-handled-bind-and-its-tail.md).

1. **J: `try p₁ = e₁ [else ā₁], … { body } [else ā]`**: binders grouped, each handler optional and next
   to its call, the body braced, a shared handler optional after it. No statement form of `try`.
2. **D: `TRY p₁ == e₁ [ELSE ā₁], … IN body [ELSE ā]`**, the same structure.
3. **A small two-way choice stays `case`.** `expect` stays a statement.
4. **2b**: a `try` as a loop arm's tail, its handlers leaving the loop, by an ADR extending 0041.

---

## Installment 4 — data and sets

hamza, opening it: *"`try fine(x) = step(i, n) { again(i + 1, acc + x) }` — fine(x) which is a tag
in here, could be confused with an application. is there a better way? discuss, argue, and propose.
if this is part of installment 4, do them together."* It is: a tag is how a variant's data is built and
taken apart. The candidates are D and J (ADR 0056), with T today's for comparison.

### 4.1 What the constructs must carry

The values of V (settypes §1.2) and the sets the logic forms from them:

| | the values | built by | taken apart by |
|---|---|---|---|
| **tuples** | products, and the unit `()`, the terminal object 1 | `tuple(ā)` | a tuple pattern (η for products) |
| **tables** | functions on [0, n) | their graph, or a rule (§3.10) | indexing `t[i]`, `len` |
| **maps** | finite functions K ⇀ V with a declared capacity | their graph; a buffer and its stores | indexing `m[k]`, which gives an `option`; `len`; `keys` |
| **variants** | Σ over a finite set of tags: a tag and its payload (ADR 0042) | a **constructor**, the injection c : S → T | a **constructor pattern** |
| **strings** | the free monoid over Unicode scalars | literals; `concat`, `string-of` | (none in the core) |
| **sets** (the logic, never printed) | subsets of V, a Boolean algebra (ADR 0049) | enumeration, comprehension, ranges, ∪ ∩ ∖ ×, `Seq`, `Map`, `option`, `subsets` | membership `∈`, a claim, or a test where decidable (W6) |

### 4.2 Measured

`go run gauntlet/surface/data.go`, over `examples/`, `lib/`, `gauntlet/stdlib/acceptance/` and
`gauntlet/differential/cases/`:

| | count |
|---|---:|
| **constructors declared** (17 in the corpus, targets and lib, with `option`'s) | 17 |
| constructors as **expressions**: applied, `c(x)` / nullary | **52 / 12** |
| constructors as **patterns**: with a payload, `c(x)` / nullary, a bare name | **88 / 8** |
| … of the patterns, in a `case` arm / a `try` or `expect` binder | 87 / 9 |
| names that are a constructor in one module and a definition in another (`at`, `rows`) | 2 |
| tables by graph / by rule (67 zero-filled) | 47 / 77 |
| `(array T)` as a type / maps by graph / map buffers / map stores / `keys` | 38 / 1 / 4 / 4 / 1 |
| tuples built / the unit `(tuple)` | 25 / 12 |
| integer ranges `(int lo hi)`, today's sets of integers | 111 |
| `concat` / `string-of` | 26 / 7 |

### 4.3 The holes and misfits

**A tag reads as a call, in both directions.** Σ's names are of two kinds:
- the **constructors** C, the free generators of a variant: they compute nothing, no equation holds
  between them, and their terms are values;
- the **defined names** D, which reduce by δ.

Term rewriting calls this a constructor discipline (O'Donnell 1977; Huet and Lévy 1979). A pattern is
a term over C and variables, matching inverts the injection, and an expression is a term over C ∪ D.
`fine(acc)` and `step(i, n)` are both applications, which is faithful, but nothing on the page says
which kind of name is applied. That is fit-research's **missing distinction** (F4). It shows three
ways:

1. **In D it is a homonym** (F2). `TRY fine(x) == step(i, n) IN …` and the definition
   `cross(c, i, n) == LOOP …` are one shape, `name(args) == expression`. Whether it binds a payload or
   defines a function depends on whether the name was declared a constructor.
2. **In J, `fine(x) = e` reads as a function definition**, the way mathematics writes one, though it
   is a pattern binding a payload.
3. **A nullary constructor in a pattern cannot be told from a binder.** In `case r { none -> 0 }`,
   `none` is a test only because a declaration says so: misspell it and it binds anything, a catch-all.
   That is Standard ML's known pitfall, which OCaml and Haskell close by requiring constructors to be
   capitalized. 8 patterns are nullary, among them every `none`.

And names cross kinds: `at` and `rows` are constructors in one module and functions in another.

**Three homonyms between a set and a value**, the shape §3.10 found for arrays: `(array T)` (38 sites,
decided `Seq(T)`), `(map K V)` against `(map (k v) …)`, and `(int lo hi)` (111 sites), a set of
integers written as if a call, which installment 1's examples already write as a range, `0..255`.

**Map buffers are outside ADR 0052.** It made a buffer a state binder holding a *table*. A map buffer is
the same discipline (maps.md §3.3: "aliasing does not care what the index set is") and is still
threaded by hand: `(again (insert m i (* i 10)) (+ i 1))`.

**The comprehension is a synonym in J** (§3.10): `[k ∈ 0..n-1 ↦ e]` is `array(n, fn(k) { e })`.

### 4.4 Findings

**Finding 1. A tag is marked `#`, in patterns and in expressions.** The marked symbols are exactly
C, the ones a pattern is built from:

```go
try #fine(x) = step(i, n) { again(i + 1, acc + x) }            // J: a pattern, visibly
case r { #some(v) -> v; #none -> 0 }                           // #none cannot be a binder
fn step(i, n) { if i < n { #fine(i) } else { #failed(array{i, 1000 + i}) } }   // building data
```
```
TRY #fine(x) == step(i, n) IN AGAIN(i + 1, acc + x)            \* D: no longer a definition's shape
```

- **A pattern is then defined by its characters**: tags, tuples, literals, names and `_`. A name in
  a pattern always binds, and a tag never does.
- **The three misfits close**: D's homonym (`#fine(x) == e` is not `f(x) == e`), J's look of a
  function definition, and the nullary pitfall.
- **`#` reads as "tag"**, as a hashtag does, and nothing in D or J uses it: D spells ≠ as `/=`
  (notation-research §4.9), and the core's internal `#cell` and `#any` never reach a surface.
- **Qualification.** A variant's tags come into scope with it, as Rust's prelude brings `Ok` and
  Haskell's imports bring constructors, so `#ok` needs no module. Where two variants in scope share a
  tag, it is written `#result.ok`. Today's `result.ok` is qualified at every use.

Why not the other marks:

| | mark | precedent | why not |
|---|---|---|---|
| (a) | **capitalized**, `Fine(x)`, `None` | OCaml (enforced), Haskell, Rust, Swift, Scala | capitals are taken three times: by **sets** (`Int`, `Seq(Byte)`, installment 1), by **host exports** (`os.ReadFile`, kept as Go spells them, ADR 0025), and in D by **keywords** (`CASE`). `Seq(S)` and `Fine(x)` would be one shape again |
| (b) | a leading dot, `.fine(x)` | Swift, Zig | the dot is qualification, so `result.ok` and `.ok` differ by what precedes them; Swift resolves `.ok` by the expected type, which our resolution (ADR 0011, before types) does not know |
| (c) | a colon, `:fine` | Elixir, Ruby, Clojure atoms | D uses `:` in `LOOP … :` and `{x ∈ S : φ}`, so `{x ∈ S :φ}` would lex a tag |
| (d) | a quote, `'fine` | Lisp: quoted data | TLA+'s prime is the next state, which D may need for concurrency (settypes §10) |
| (e) | reorder, `try step(i, n) is fine(x)` | C#'s `is`, Kotlin | fixes the binder's look, not a tag in an expression or a nullary pattern |

**Finding 2. The marks make one lexical scheme.** With Finding 1, a reader of D or J sees a name's
kind by its spelling:

| spelling | kind | examples |
|---|---|---|
| lowercase | a value, a definition, a binder | `acc`, `step`, `kind_name` |
| `#name` | a tag: a constructor, free | `#fine`, `#none`, `#not_found` |
| Capitalized | a set, in the logic | `Int`, `Seq(Byte)`, `Map(Int, String)` |
| `alias.Name` | a host name, as the host spells it | `os.ReadFile`, `strconv.Atoi` |
| ALLCAPS (D only) | a keyword | `CASE`, `TRY`, `LOOP` |

**Finding 3. Tables and maps are one finite function in the logic, and two in the computation.** In
the logic both are finite functions: Seq(V) is Map(0..n−1, V). In the computation a table has a rule
form and a map does not (maps.md §3.2): a table's index set is a number, a map's is the data. So:
- **D keeps TLA+'s three forms**, `⟨e₀, …⟩` (a table's graph), `[k ∈ 0..n-1 ↦ e]` (its rule) and
  `[k₁ ↦ v₁, …]` (a map's graph, TLA+'s record syntax, a record being a function on its field
  names). D has no `array(n, f)`.
- **J keeps `array { … }`, `array(n, f)` and `map { k ↦ v, … }`**, and **drops the comprehension**,
  its synonym (§3.10's open item 4). The map's arrow is `↦` (ASCII `|->`), not the arms' `->`: a
  map literal and a `case` on literals are both a function by its graph, but one is data and the
  other control.
- **The sets are `Seq(V)` and `Map(K, V)`** in both, never `array(V)` or `map(K, V)`.

**Finding 4. A map buffer is a state binder holding a map, written in program order.** ADR 0052's
derivation applies unchanged, since aliasing does not care what the index set is: `var m := map(8)`
in J, `m := map(8)` in a D group, a store `m[k] := v`, a read `m[k]`, and R0–R4 as for tables (a fresh
map, named only by its binder, borrowed by a call, no escaping closure, frozen at its scope's end).
That extends ADR 0052 from tables to maps, so it is a decision with an amendment.

**Finding 5. Sets: D writes TLA+'s, and J writes Python's operators.** D: `{1, 2}`, `{x ∈ S : φ}`,
`lo..hi`, `S ∪ T`, `S ∩ T`, `S \ T`, `S × T`. J: `set {1, 2}` (ADR 0056), `set { x ∈ S : φ }` (the
comprehension inside the former, as an array's elements are), `lo..hi`, `S | T`, `S & T`, `S - T`
(Python's set operators), `S × T`. Ranges replace `(int lo hi)` at 111 sites in both.

**Finding 6. Tuples: J `(a, b)` and `()`, D `⟨a, b⟩` and `⟨⟩`.** There is no tuple of one (products.md),
so `(e)` is grouping and a tuple always has a comma.

**Finding 7. Concatenation is not `+`.** `+` is commutative on ℤ, and concatenation is not: it is the
free monoid's operation. Julia writes strings' concatenation `*` for that reason, TLA+ writes a
sequence's `\o` (∘), Haskell and Elm `++`. D takes `∘` (ASCII `\o`), J takes `++`; `string_of`
stays a function.

### 4.5 Six programs, three notations

**T** today, **D** and **J**, every one in the decided forms of installments 1–3 and this
installment's proposals. The tags are written as ADR 0059 and ADR 0060 decided, `'fine` and
`os.'not_found`, in place of §4.4's first proposal, `#`.

---

**E1 — tags in a choice** (`examples/io/roundtrip.oro`'s `kind-name`).

```lisp
(case k                                                                         ; T
  os.not-found   "not-found"
  os.exists      "exists"
  os.permission  "permission"
  (os.other e)   "other")
```
```
CASE k OF os.'not_found  → "not-found"                                             \\ D
       [] os.'exists     → "exists"
       [] os.'permission → "permission"
       [] os.'other(e)   → "other"
```
```go
case k {                                                                        // J
    os.'not_found  -> "not-found"
    os.'exists     -> "exists"
    os.'permission -> "permission"
    os.'other(e)   -> "other"
}
```

---

**E2 — tags built and bound** (`gauntlet/differential/cases/sum-slots.oro`'s `step` and `walk`).

```lisp
(def step (i n)                                                                 ; T
  (if (< i n) (fine i) (failed (array i (+ 1000 i)))))

(def walk (n)
  (loop ((i 0) (acc 0))
    (> i 5)  (fine acc)
    else     (try (fine x) (step i n)
               (again (+ i 1) (+ acc x)))))
```
```
step(i, n) == IF i < n THEN 'fine(i) ELSE 'failed(⟨i, 1000 + i⟩)               \\ D

walk(n) ==
  LOOP i == 0, acc == 0 :
      i > 5  → 'fine(acc)
   [] OTHER  → TRY 'fine(x) == step(i, n) IN AGAIN(i + 1, acc + x)
```
```go
fn step(i, n) { if i < n { 'fine(i) } else { 'failed(array{i, 1000 + i}) } }   // J

fn walk(n) {
    loop i = 0, acc = 0 {
        i > 5 -> 'fine(acc)
        else  -> try 'fine(x) = step(i, n) { again(i + 1, acc + x) }
    }
}
```

---

**E3 — tables: a graph, a rule, indexing.**

```lisp
(let primes (array 2 3 5 7)                                                     ; T
     sq     (table n (fn (k) (* k k)))
  (+ (primes 0) (sq (- n 1))))
```
```
LET primes == ⟨2, 3, 5, 7⟩  sq == [k ∈ 0..n-1 ↦ k * k]  IN  primes[0] + sq[n - 1]   \\ D
```
```go
{ primes = array{2, 3, 5, 7};  sq = array{k * k : k ∈ 0..n-1};  primes[0] + sq[n - 1] }   // J
```

---

**E4 — a map buffer in program order, read as an option**
(`gauntlet/differential/cases/map-dynamic.oro`; Finding 4).

```lisp
(let m (build-map m 8                                                           ; T
         (loop ((m m) (i 0))
           (>= i n)  m
           else      (again (insert m i (* i 10)) (+ i 1))))
  (case (m 2) (some v) v none 0))
```
```
LET m := map(8) IN                                                              \\ D
  (LOOP i == 0 :
       i ≥ n → ⟨⟩
    [] OTHER → m[i] := i * 10 ; AGAIN(i + 1)) ;
  CASE m[2] OF 'some(v) → v [] 'none → 0
```
```go
{ var m := map(8)                                                               // J
  loop i = 0 {
      i >= n -> ()
      else   -> { m[i] := i * 10; again(i + 1) }
  }
  case m[2] { 'some(v) -> v; 'none -> 0 } }
```

---

**E5 — sets in a claim**: a parameter's set is a union of ranges, the result's a range.

```lisp
(sig digit-value ((c (int 0 255))) (int 0 35)                                   ; T
  (where (or (and (>= c 48) (<= c 57)) (and (>= c 97) (<= c 122)))))
(def digit-value (c) (if (<= c 57) (- c 48) (- c 87)))
```
```
digit_value(c ∈ 48..57 ∪ 97..122) ∈ 0..35 ==                                    \\ D
  IF c ≤ 57 THEN c - 48 ELSE c - 87
```
```go
fn digit_value(c ∈ 48..57 | 97..122) (r ∈ 0..35) {                              // J
    if c <= 57 { c - 48 } else { c - 87 }
}
```

---

**E6 — a tuple taken apart, and strings.**

```lisp
(let (tuple q r) (divmod a b)                                                   ; T
  (concat (concat "q=" (string-of q)) (concat " r=" (string-of r))))
```
```
LET ⟨q, r⟩ == divmod(a, b) IN "q=" ∘ string_of(q) ∘ " r=" ∘ string_of(r)       \\ D
```
```go
{ (q, r) = divmod(a, b);  "q=" ++ string_of(q) ++ " r=" ++ string_of(r) }      // J
```

### 4.6 Measured

`go run gauntlet/surface/tokens.go -section 4`, over E1–E6:

| notation | tokens | × T | lexemes | × T | depth | closing run | lines |
|---|---:|---:|---:|---:|---:|---:|---:|
| T today | 319 | 1.00 | 313 | 1.00 | 6 | 5 | 25 |
| D | 287 | **0.90** | 271 | **0.87** | **2** | 2 | 18 |
| J | 307 | 0.96 | 289 | 0.92 | 4 | 4 | 24 |

- **Data is where the new notations are smallest**: D 0.87× today in lexemes, J 0.92×. Infix
  arithmetic, ranges in place of `(int lo hi)` and the claims in the header (E5's `where` against a
  union of ranges) account for most of it. Each `'` costs a token and a qualified tag, `os.'not_found`, four (T's `os.not-found` is one); the counts include them.
- **Depth falls from 6 to 2 in D and 4 in J**; J's braces cost a level per block, as before.
- **The three homonyms between a set and a value are gone** in both: `Seq(V)`, `Map(K, V)` and
  `lo..hi` are sets, and `array {…}`, `⟨…⟩`, `map {…}` and `[k ↦ v]` are values.

### 4.7 Open, for hamza

1. **A tag is `#name`**, in patterns and expressions, D and J; a variant's tags come into scope with it,
   `#result.ok` only where two clash (Finding 1). *Recommended*: it is the constructor discipline
   made visible, it closes D's homonym, J's definition look and the nullary pitfall, and every other
   mark collides with something already decided.
2. **The lexical scheme of Finding 2**: lowercase values, `#` tags, Capitalized sets, host names as
   the host spells them, ALLCAPS keywords in D. *Recommended*.
3. **Tables and maps** (Finding 3): D `⟨…⟩`, `[k ∈ 0..n-1 ↦ e]`, `[k ↦ v, …]`; J `array {…}`,
   `array(n, f)`, `map { k ↦ v, … }`, the comprehension dropped from J's computation; `Seq(V)`,
   `Map(K, V)` the sets. *Recommended*.
4. **A map buffer is a state binder holding a map** (Finding 4), extending ADR 0052. *Recommended*;
   it needs an amendment to ADR 0052.
5. **Sets** (Finding 5): D TLA+'s, J `set { … }`, `set { x ∈ S : φ }`, `|`, `&`, `-`, `×`, ranges
   `lo..hi` for `(int lo hi)`. *Recommended*.
6. **Tuples** (Finding 6): J `(a, b)`, `()`; D `⟨a, b⟩`, `⟨⟩`. *Recommended*.
7. **Concatenation** (Finding 7): D `∘`, J `++`. *Recommended*.

### 4.8 hamza's answers, and the arguments for what is still open

hamza, on §4.7: *"I don't like #, I prefer 'fine(x) unless you have a better solution. we already
reserved it for symbols, the next step of TLA+ is written Next' not at the beginning. … we decided at
the beginning that case have no meaning in the language, if we use it for sets, it is going to
convention only. not enforced. … maybe we should keep the comprehension for array in J, and remove
the rule array(n, f), instead we use array { k \in 0..n-1 : e} … and maybe even the literal of map
{ k : v, ...} one shape to remember. maybe the convention will be Array(V) and Map(K, V)? … finding 5.
what about the membership operator in J … python, tla+ or something else … why J and D don't have
os.?"*

**Decided**: item 4 as [ADR 0058](decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md)
(a map buffer is a state binder holding a map); item 6, tuples (J `(a, b)` and `()`, D `⟨a, b⟩` and
`⟨⟩`); item 7, concatenation (D `∘`, J `++`); item 5's forms (D TLA+'s, J `set { … }`, ranges `lo..hi`),
its operators still open.

#### 1. The tag: `'fine(x)`, and I withdraw my objection

My objection (d) was wrong. TLA+'s prime is **postfix**, `x'` and `Next'`, and data.md §2.2 already
keeps it there: *"`x'` is not legal today and stays illegal, so a later decision to allow
mathematical primes is not blocked."* A prefix quote is a different token, and data.md §2.1 has already
given it its meaning:

> `symbol ::= ' identifier` … a **symbol** is a name used as data rather than as a reference: `'x`
> means *the name x*, while `x` means *the value bound to x*. That is the use–mention distinction.

**For `'`:**
- **It unifies tags with symbols, and that is the algebra.** A record is a product indexed by a finite
  set of labels, Π over ℓ ∈ L; a variant is a sum indexed by a finite set of tags, Σ over ℓ ∈ L
  (sums.md §1 already calls the sum "the exact dual of a table's Π"). In both, the index is a name used
  as data: a symbol. So `'x` projects a record's field and `'fine(x)` builds or matches a variant's
  case, one mark for the index of a Π and of a Σ. data.md §2.4 left room for exactly this: *"a later
  design may give a surviving symbol a representation, such as a string or an integer tag"*, and
  ADR 0042's tag is that integer.
- **It is quotation**: data, not a reference. A constructor's term is a value that does not compute,
  which is what quoting says.
- **It is already reserved**, so it costs no new character, and data.md's rule that `'` must be
  followed directly by an identifier keeps `'(…)`, `'3` and `'"s"` errors.
- **It closes the three misfits as `#` would**: D's `'fine(x) == e` is not a definition's shape, J's
  `'fine(x) = e` does not read as a function definition, and `'none` cannot be a binder.

**Against `'`:**
- **It is small.** The nullary pitfall's protection rests on seeing it, `'none` against `none`, where
  `#` is heavier on the page. A highlighter answers this, and Lisp, OCaml (`'a`) and Rust (`'a`) live
  with it.
- **J is Go-inspired, and Go writes `'a'` for a rune.** A reader from Go, C or Java may expect a
  closing quote, and an editor without the language's grammar colours the rest of the line as a
  character literal. The language has no character literals (strings only), so nothing is ambiguous to
  the grammar, only to a reader's habit.

**Another solution?** None better. A suffix (`fine!(x)`) reads as a macro (Rust), and a double sigil
(`::fine`) is C++'s scope operator. **Recommended: `'`**, for the unification above.

#### 2. Case has no meaning: agreed, with one consequence

Agreed. Identifiers stay case-sensitive, because a host's are (`os.ReadFile` is Go's spelling and must
be written so), but **case carries no category**. Sets are Capitalized by convention, a formatter may
suggest it, and nothing enforces it. D's reserved words are a fixed list, written in capitals as
TLA+'s are: that is the list, not a meaning of case.

The consequence supports item 1: **a tag cannot be marked by case**, since nothing would enforce the
mark. It has to be a character the grammar checks, which `'` is. §4.4's Finding 2 becomes: names of any
case (sets Capitalized by convention), `'tags`, host names as the host spells them, and D's reserved
words.

#### 3. One shape for arrays, maps and sets: agreed, with one push-back on `:`

**Agreed: the rule moves inside the former, and `array(n, f)` goes** (amending ADR 0056's item 8). It
has two gains beyond the shape:
- **the hazard ADR 0056 accepted disappears**: with no `array(…)` form, `array(3, 0)` against
  `array{3, 0}` cannot happen, because `array(` is a syntax error;
- **the binder is visible**: `array { k ∈ 0..n-1 … }` names the index the rule ranges over.

Its cost: a zero-filled buffer, the commonest rule (67 of 77), writes its binder,
`array { _ ∈ 0..n-1 … 0 }` against `array(n, 0)`, about six tokens more at each of 67 sites.

**Push-back: `:` must not mean "maps to" where it already means "such that".** In a set it is
**separation**, Zermelo's: `{x ∈ S : φ}` is the elements of S for which φ holds, a filter, and
mathematics reads that shape that way everywhere, TLA+ included. In `array { k ∈ 0..n-1 : e }` the same
characters would mean **a function**, k maps to e. The keyword tells the two apart, as you say, but
both readings are well-typed for the same body:

```go
set   { x ∈ S : x > 0 }        // the positive elements of S: a subset
map   { x ∈ S : x > 0 }        // each x of S mapped to whether it is positive: S → Bool
array { k ∈ 0..n-1 : k > 0 }   // n booleans
```

One word changes a filter into a function, and a reader trained on set-builder notation reads the
second and third as filters. That is the homonym (F2) the fit criteria count.

**Proposed instead: one shape, two separators, each with its mathematical meaning.** `:` is "such
that", only in sets; `↦` (ASCII `|->`) is "maps to", in arrays and maps, rule or graph:

```go
set   { x ∈ S : x > 0 }                  // separation
array { k ∈ 0..n-1 ↦ k * k }             // a rule
array { 2, 3, 5, 7 }                     // a graph
map   { 1 ↦ 10, 2 ↦ 20, 5 ↦ 50 }         // a graph
```

You keep one shape, `former { … }`, and the binder inside it, and what `:` and `↦` mean does not depend
on the keyword. It is TLA+'s own pair (`{x ∈ S : P}` against `[x ∈ S ↦ e]`), so D and J share it. The
cost against JSON's `{k: v}`, the most familiar map literal there is: `↦`, or `|->`, instead of `:`.

**`Array(V)` and `Map(K, V)`: agreed, and now safe.** ADR 0056 chose `Seq(V)` so that a set and a value
were never spelled alike. With `array(n, f)` gone, `Array(V)` against `array { … }` cannot be confused
silently: a mis-cased `array(V)` is a syntax error. The empty map buffer keeps its capacity in
parentheses, `map(8)` (ADR 0058), and a mis-cased `map(Int, String)` is refused by type, since a set is
not a capacity. **Recommended: the set names are `Array(V)` and `Map(K, V)` in D and J both**, one name
in Σ₀ (renaming abstract.md's `Seq`), so both notations print the same set.

#### 4. The operators: Python's, all the way, in J

D need not change: it writes TLA+'s glyphs, `∈ ∪ ∩ \ ⊆ ×` (ASCII `\in \cup \cap \ \subseteq \X`). The
question is J's.

| | membership | ∪ | ∩ | ∖ | ⊆ | × |
|---|---|---|---|---|---|---|
| **Python, all the way** | `in` | `\|` | `&` | `-` | `<=` | (none in Python) |
| TLA+'s ASCII | `\in` | `\cup` | `\cap` | `\` | `\subseteq` | `\X` |
| Unicode | `∈` | `∪` | `∩` | `∖` | `⊆` | `×` |

**For Python's, all the way:**
- **They are the Boolean algebra's own operators**: `|` and `&` are join and meet, `-` the relative
  complement, `<=` the lattice's order. A set is its characteristic function, and `&` and `|` are its
  pointwise ∧ and ∨, which is also what they mean on bits in C.
- **They are free in J**: the language has no bitwise operators (CLAUDE.md: deliberately not promoted),
  so `|`, `&` and `^` mean nothing else.
- **They are ASCII, and the most widely read set notation in programming.** TLA+'s ASCII (`\cup`,
  `\subseteq`) is made for typesetting, not for reading in a source file.

**Against:** `-` and `<=` also act on integers. That is overloading by sort, as `+` and `*` already are
between `int` and `f64`, resolved by the checker that knows the sort; sets live in claims, so the
logic's checker resolves theirs.

**The product.** Python has no set product. `S * T` is the categorical product, and |S × T| = |S| · |T|,
so `*` is the right overload by the same argument. Products are rare in sets once each binder has its
own set (ADR 0051), so the choice matters little.

**And `∈`, which you like?** A canonical spelling is one spelling (notation-research §10: a formatter
prints one). The source writes `in`, and an editor or the reading rendering may display it `∈`, as a
ligature: the source stays ASCII and the page looks as you like.

**Recommended: J writes Python's**, `in`, `|`, `&`, `-`, `^`, `<=`, `<`, and `*` for the product. D
writes TLA+'s.

#### 5. Why `'not_found` and not `'os.not_found`

§4.4's Finding 1 proposed that **a variant's tags come into scope with the variant**, as Rust's prelude
brings `Ok` and `None` and Haskell's imports bring constructors. `k` is an `os.kind`, the module `os` is
in use, and `'not_found` names its tag, the only `not_found` in scope. Where two variants in scope
share a tag, the program writes the qualified tag, `'os.not_found`, and an unqualified one is refused as
ambiguous. Measured: no file in the corpus uses `lib/result` and also declares its own `ok` or `err`, so
nothing would need qualifying today.

The other answer is to qualify always, as today's T does (`os.not-found`, and `result.ok` at every use).
That tells the reader where each tag comes from, at a module name per tag, and the scrutinee's variant
already says it. **Recommended: unqualified, refused when ambiguous.**

#### Open, for hamza

1. **The tag is `'name`**: tags and symbols one mark, the index of a Π and of a Σ. *Recommended.*
2. **Case carries no category**: sets Capitalized by convention, a tag marked by `'`, D's reserved words
   a list. *Agreed above.*
3. **One shape**: `array { k ∈ 0..n-1 ↦ e }` replacing `array(n, f)` (amending ADR 0056), `:` only for
   "such that" in sets, `↦`/`|->` for "maps to" in arrays and maps, `map { k ↦ v, … }`, the empty map
   buffer `map(cap)`; the set names `Array(V)` and `Map(K, V)` in D and J. *Recommended.*
4. **J's set operators are Python's**, `in` rendered `∈` where wanted; D's TLA+'s. *Recommended.*
5. **Tags unqualified**, qualified only where two variants in scope share one. *Recommended.*

### 4.9 The colon, pushed with everything; `in` against the modes; qualified tags

hamza, on §4.8: *"1. the tag is 'name, agreed. 2. agreed. 3. yes, but not sold on the use of :, we know
the context. show the problem with examples. push with everything you have. 4. if we go python all the
way, wouldn't in be confused with inout, and out? 5. elaborate, I think they should be qualified. push
back."* Items 1, 2 and the agreed part of 3 are
[ADR 0059](decisions/0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md).

#### The colon: every attack, and what survives

Under hamza's proposal `:` separates a binder from its body in every former, and a key from its value
in a graph:

```go
set   { x ∈ S : x > 0 }
array { k ∈ 0..n-1 : k * k }
map   { x ∈ S : f(x) }
map   { 1 : 10, 2 : 20 }
```

**A1. One word turns a filter into a function.**

```go
set { x ∈ S : x > 0 }      // the positive elements of S
map { x ∈ S : x > 0 }      // S → Bool: each element mapped to whether it is positive
```

Both are well-typed for the same body, and they are different values.

**A2. Mathematics reads `{k ∈ 0..n-1 : e}` as separation, a filter**, Zermelo's set-builder, and TLA+
does too. `array { k ∈ 0..n-1 : k * k }` would be read first as "the k whose k·k holds".

**A3. Two colons in one line**: `array { k ∈ 0..n-1 : set { x ∈ S : x > k } }`.

**A4. Quantifiers will want `:` too** (installment 5): `forall x ∈ S : φ`, TLA+'s `∀x ∈ S : φ`.

**A5. A rule and a single point differ by `∈ D`**: `map { k ∈ keys : 0 }` is a map over `keys`,
`map { k : 0 }` a one-entry map whose key is the value of a k bound outside.

**A6. D cannot follow.** D is TLA+'s, `[x ∈ S ↦ e]` against `{x ∈ S : P}`, so the same tree would print
`↦` in D and `:` in J.

**A7. The image of a set**, mathematics' `{f(x) : x ∈ S}`, writes `:` with the binder on the right.

**A8. A `case` arm is "maps to" too**: `case k { 1 -> "a" }` against `map { 1 : "a" }`, two separators for
one meaning.

**And here is the defence, which turned out stronger than the attacks.** A subset of S *is* its
characteristic function: 𝒫(S) ≅ Bool^S, A ↦ χ_A. Under that isomorphism `set { x ∈ S : φ }` is "the
subset whose characteristic function is x ↦ φ", so **`binder : body` is the λ-abstraction λx ∈ S. body
in every former**, and the keyword says what to make of the function:

| | the former | maps the function to |
|---|---|---|
| `set { x ∈ S : φ }` | χ⁻¹ | the subset where it is true (φ must be Bool) |
| `array { k ∈ 0..n-1 : e }` | tabulation on [0, n) | a table |
| `map { x ∈ D : e }` | tabulation on D | a map |
| `forall x ∈ S : φ`, `exists x ∈ S : φ` | ∀, ∃ | a truth value |

The last row is Church's ("A formulation of the simple theory of types", 1940): a quantifier is a
function applied to a propositional function, ∀(λx. φ). A graph's pair, `1 : 10`, is one point of the
same function, 1 maps to 10, which is also Python's and JSON's `{k: v}`.

So, attack by attack:
- **A1 is not a homonym.** `set` and `map` apply different operations to the *same* function x ↦ (x > 0),
  as `forall` and `exists` do to one predicate. A homonym is one symbol with two meanings; here `:` has
  one meaning and the two keywords have two. And a body that is not Bool is refused in a `set`.
- **A2 is a habit, not a meaning**: separation *is* the characteristic-function reading of the same λ,
  and the keyword `array` says a table follows.
- **A3 and A4 dissolve**: every colon is the same λ.
- **A5 is not about `:`**: `↦` has it too (`map { k ↦ 0 }` against `map { k ∈ keys ↦ 0 }`).
- **A6 stands, and matters only if both notations survive**: D keeps TLA+'s brackets, whose shape
  (`[…]` against `{…}`) already says what J's keyword says.
- **A7 stands as a rule**: in J the binder is always first, so the image of a set needs its own former
  (installment 5), never `set { f(x) : x ∈ S }`.
- **A8 is a real distinction kept**: an arm is control, applied now, and a map is data, stored. The
  separator marks which, as the heads `case` and `map` do.

**Verdict: the push fails, and my proposal was the misfit.** With `↦` for arrays and maps and `:` for
sets, J would spell one λ two ways, a synonym (F1). **Recommended now: hamza's `:`**, in every former
and in a graph's pairs:

```go
set   { x ∈ S : x > 0 }
array { k ∈ 0..n-1 : k * k }
array { _ ∈ 0..n-1 : 0 }                 // the zero-filled buffer
map   { 1 : 10, 2 : 20, 5 : 50 }
map   { x ∈ D : f(x) }
```

What it costs: A6's divergence from D, and A7's rule that a binder comes first.

#### `in` against `inout` and `out`: yes, and it decides the question

**The collision is worse than a shared prefix.** The modes are named after Ada's (ADR 0048: "Algol W,
Ada's `in out`"), and in Ada **`in` is itself a mode**, the default, read-only:
`procedure P (X : in Integer; Y : in out Integer; Z : out Integer)`. With Python's `in` for membership,
a host declaration reads as three modes:

```go
fn Sscan(str in String, a in Value inout, b in Value inout)      // Python's in
fn Sscan(str ∈ String, a ∈ Value inout, b ∈ Value inout)         // ∈
```

In the first line `in` and `inout` sit next to each other with unrelated meanings. Measured, the corpus
has 48 mode sites today (11 `(cell …)`, 37 `(out …)`, which become `inout` and `out`), against a
membership in every claimed binder.

Renaming the modes instead would not help: `inout` is Swift's word for exactly ADR 0048's
copy-in, copy-out, and `ref` would describe a reference that ADR 0048 is not.

**So not Python all the way.** Membership is not one operator among the others. It is the binder's own
relation, `x ∈ S` in every parameter and every claim, and it keeps its glyph. **Recommended for J:
`∈` for membership** (ASCII input `\in`, which a formatter prints as `∈`) **and Python's operators for the
Boolean algebra's operations**, `|` (∪), `&` (∩), `-` (∖), `^` (symmetric difference), `<=` (⊆), `<`
(⊂), and `*` (×). D writes TLA+'s.

#### Qualified tags: the push-back, and a middle way

**hamza's case for qualifying is strong, and it is uniformity.** Every imported name is qualified, a
function (`os.ReadFile`) or a set (`os.File`), so a tag is too (`'os.not_found`). The reader sees where
each tag comes from, nothing arrives by an import rule, and today's T already writes `result.ok`.

**The push-back is the cost, and where it falls.**
- **Measured: `result.ok` and `result.err` are written 99 times** (53 and 46). Every `case` on a result
  names the module once per arm, and a `try` with a handler names it twice: `try 'result.ok(src) =
  os.ReadFile(path) else 'result.err(k) -> …`.
- **The scrutinee already names the variant.** In
  `case k { 'os.not_found -> …; 'os.exists -> …; 'os.permission -> …; 'os.other(e) -> … }` the four
  `os.` repeat what k's type says. OCaml removed exactly this repetition by type-directed
  disambiguation of constructors (OCaml 4.01, 2013), and Swift by `.notFound`. Resolution here precedes
  types (ADR 0011), so that route is closed.
- **The prelude**: `option`'s `'some` and `'none` are the language's own (the reducer declares them).
  Qualifying them, `'lang.some`, would be noise, which is why Rust's prelude brings `Some`, `None`,
  `Ok` and `Err`.

**The middle way: qualified by default, unqualified by a written choice.**
1. **An imported tag is qualified**, as hamza says: `'os.not_found`, `'result.ok`. That is the default,
   uniform with functions and sets.
2. **A module's own tags are unqualified inside it**, as its own functions are.
3. **The language's tags are unqualified**: `'some`, `'none`, the prelude.
4. **A `use` may expose named tags**, Elm's `import Result exposing (Ok, Err)`:
   `use result exposing 'ok, 'err`. Inside that module they are written unqualified, and the reader
   finds where they come from one line at the top of the file. Two exposed tags of one name are refused.

Provenance stays written, once per module rather than once per use, and nothing arrives without a line
saying so.

#### Open, for hamza

1. **`:` in every former and in a graph's pairs**: the λ's separator, its meaning one under
   𝒫(S) ≅ Bool^S; the binder always first. *Recommended*, withdrawing `↦`.
2. **J's membership is `∈`** (ASCII input `\in`); **J's set operations are Python's**, `| & - ^ <= < *`; D
   writes TLA+'s. *Recommended*, because Python's `in` would sit beside the modes `inout` and `out`.
3. **Tags qualified by default**, with a module's own and the language's unqualified, and
   `use M exposing 'a, …` to choose otherwise. *Recommended*, as the middle way.

### 4.10 TLA+'s two set-builders, and what they say about the colon

hamza, on §4.9: *"so in tla+ `{x \in S : P}` is filter, `{ e : x \in S}` is map. `{ 2 * x: x \in { 1, 2,
3, 4 } }` \* { 2, 4, 6, 8 }, `{ x + y: x \in 1..2, y \in 1..2 }` \* { 2, 3, 4 }. argue some more. 2.
sure. 3. but I thought os.'not_found … no to exposing."* Items 2 and 3 are
[ADR 0060](decisions/0060-j-set-operators-and-qualified-tags.md).

#### Two operations, two orders

TLA+ has both set-builders, and they are two different operations of set theory:

| | TLA+ | the operation | the axiom |
|---|---|---|---|
| **separation**, a filter | `{x ∈ S : P}` | the elements of S for which P holds; a subset of S | Zermelo's schema of separation (1908) |
| **replacement**, an image | `{e : x ∈ S}` | the values of e as x ranges over S | Fraenkel's schema of replacement (1922) |

Both take a λ over S: separation λx. P, whose body is a truth value, and replacement λx. e, whose
body is anything. **TLA+ tells them apart by where the binder stands**: first for a filter, after the
colon for an image. And the colon reads one way in both, set-builder's "such that", because
{e : x ∈ S} is {y : ∃x ∈ S. y = e}: the term collected on the left, the condition that binds on the
right. A filter is the case where the term is the binder itself.

#### What that does to §4.9's verdict

§4.9 read `binder : body` as one λ in every former, the binder always first. TLA+ shows the cost of
that reading:
- **the image needs a second former** (A7), where TLA+ and mathematics write it in `set` itself;
- **an array's rule reads as a filter** (A2): `array { k ∈ 0..n-1 : k * k }` has a filter's shape;
- **D and J diverge** (A6).

TLA+'s order removes all three, at no cost to the λ: in each form the body is still a λ's, and the
position says which operation the former applies. **An array is an ordered image**: ⟨e(0), …, e(n−1)⟩
is the replacement of 0..n−1 under λk. e, kept in order. So it takes the image's order, which is also
the comprehension order of Python (`[k * k for k in range(n)]`), Haskell (`[k * k | k <- [0 .. n-1]]`) and
mathematics' sequences:

```go
set   { x ∈ S : x > 0 }                      // separation: the positive elements of S
set   { 2 * x : x ∈ set { 1, 2, 3, 4 } }     // replacement: { 2, 4, 6, 8 }
set   { x + y : x ∈ 1..2, y ∈ 1..2 }         // replacement over a product: { 2, 3, 4 }
array { k * k : k ∈ 0..n-1 }                 // an ordered image: a table
array { 0 : _ ∈ 0..n-1 }                     // the zero-filled buffer
map   { 1 : 10, 2 : 20, 5 : 50 }             // a graph
```

- **A map has no rule form** (maps.md §3.2: a map's index set is its data, and a rule over it would still
  store it), so `map` takes only a graph. Its pairs, `k : v`, are JSON's and Python's, and TLA+'s own
  pair is `k :> v`. Since only `map` takes a pair, a pair cannot be read as a comprehension.
- **An array's binder ranges over 0..n−1**, one binder; an image over a product is a set's.
- **D writes TLA+'s forms**, `{x ∈ S : P}` and `{e : x ∈ S}` for sets, `[k ∈ 0..n-1 ↦ e]` for a table's
  rule. J's sets are then D's, word for word inside the braces.

#### The one ambiguity TLA+ has, and its rule

When the collected term is itself a membership test, the two orders read alike: `{x ∈ T : x ∈ S}`. TLA+
reads the binder-first form, a filter, here T ∩ S. The image of the test, a set of truth values, needs
parentheses, `{(x ∈ T) : x ∈ S}`. J takes the same rule. A set of truth values is the rarest set there
is.

#### Proposed, replacing §4.9's item 1

**TLA+'s two orders, in J**:
- `set { x ∈ S : P }` filters;
- `set { e : x ∈ S, … }` and `array { e : k ∈ 0..n-1 }` take an image (a table, an ordered one);
- `map { k : v, … }` is a graph;
- the colon is set-builder's "such that" in sets and arrays, and a pair's separator in a map;
- a binder-first form beginning `x ∈ …` is a filter, and the image of a membership is parenthesized.

This withdraws §4.9's "binder always first" and keeps hamza's `:`.

---

## Installment 5 — claims

hamza: *"go ahead. always mathematical and algebraic"*, after installment 4. The candidates are D and J
(ADR 0056). This installment writes the second language of ADR 0049, the logic, which never prints: a
definition's claim, a claim on a value, an assertion, an invariant, a specification function, a lemma,
and the formulas they are written in.

### 5.1 What the logic must carry

From abstract.md §2.5–2.7 and §7, and settypes §6:

| | the form | what it means | its obligation in 𝒪 |
|---|---|---|---|
| **a definition's claim** | lam(x̄ ∈ S̄; r ∈ U; φ; ψ; e) | f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ} | the body under x̄ ∈ S̄ ∧ φ establishes r ∈ U ∧ ψ; each call establishes ā ∈ S̄ ∧ φ[ā/x̄] and gains ψ |
| **a claim on a value** | the(S; e) | e's value lies in S; the value is e's | e ∈ S |
| **an assertion** | assert(φ; e) | φ holds here, then e | φ, then φ is a fact |
| **an invariant** | loop(…; I; …) | I holds at entry and at every back edge | entry, preservation, and I at each exit (§3.11) |
| **a specification function** | spec(f; b̄; r ∈ U; e) | a function of the logic, which may recurse and never runs | its recursion is well founded (size-change) |
| **a lemma** | lemma(n; φ; steps) | a proven sentence | each step from the ones before; then φ is a fact everywhere after |

And the formulas: `Formula ::= e | e ∈ S | e ≐ e′ | ¬φ | φ ∧ ψ | φ ∨ ψ | φ ⇒ ψ | ∀ b̄. φ | ∃ b̄. φ |
app(f; ā)`, f a specification function. W6 keeps the boundary: a formula's own forms (a quantifier,
≐ at any sort, membership in a set that is not decidable, a call of a specification function) stand
only where a claim stands.

**ε erases every one of them** (abstract.md §5): ⟦P⟧ = ⟦D(ε(P))⟧, so no claim changes what a program
computes. A claim either is proven or refuses the program.

### 5.2 Measured

`go run gauntlet/surface/claims.go`, over the programs and, separately, the target files:

| | programs | targets |
|---|---:|---:|
| `sig` | 143 | 870 |
| parameters: a name and its set / and the set `any` | 170 / 13 | 649 / 22 |
| parameters: a set without a name / the set `any` without a name | — | 496 / 170 |
| results: a set / a tuple | 138 / 5 | 800 / 70 |
| `where` | 35 | 54 |
| … bounds on parameters, one set each | 17 | 39 |
| … bounds where one is a length (`len a > 2`) | 7 | 3 |
| … a relation between parameters (`len p = len q`) | **11** | **9** |
| … another formula (a disjunction) | 0 | 3 |
| `ensures` (each uses the result's name) | **0** | 18 |
| `the` in code | 0 | 9 |
| assertions, invariants, specification functions, lemmas, quantifiers | **0** | **0** |

Every formula the corpus writes is a comparison or a conjunction of comparisons over parameters, `len`
and linear arithmetic: Presburger arithmetic with lengths, the first region of settypes §5's
decidability map. **Nothing above that region is written, because no notation can write it.** The
Presburger measurement found the same from the other side: refusals lack facts, not proof strength.
Invariants, specification functions and lemmas are how a program states the facts it lacks.

### 5.3 The holes and misfits

**Four holes in today's notation**: no assertion, no invariant (§3.3), no specification function, no
lemma. And no quantifier, so a claim about every element of a table, sortedness or a search's failure,
cannot be written.

**A parameter's bound is written apart from the parameter.** `(sig get ((a (array int)) (i int)) int
(where (and (<= 0 i) (< i (len a)))))` names `i` twice and its set twice: `int` at the binder and the
bound in the `where`. Of 89 `where` clauses, 66 only bound parameters (installment 1's Finding 2,
measured).

**`result` is a name nobody bound.** The 18 `ensures` read `result`, a keyword, while ADR 0051 made the
result a binder whose pattern may be a tuple.

**Two equalities, one of them unwritten.** The computation's `=` (D) or `==` (J) is integer equality;
the logic's ≐ is identity at any sort (abstract.md §2.5). A claim that two strings or two tables are
equal has no spelling.

### 5.4 Findings

**Finding 1. The header is the membership statement.** abstract.md §2.6 reads a definition as
f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}, and the header writes exactly that, in the shape installment 1
decided:

```go
fn div(a ∈ Int, b ∈ Int) (r ∈ Int) where b != 0 ensure abs(r) <= abs(a) { a / b }     // J
```
```
div(a ∈ Int, b ∈ Int) (r ∈ Int) WHERE b ≠ 0 ENSURE |r| ≤ |a| == a ÷ b                \* D
```

- **The result is a binder** (ADR 0051), named where ψ needs it: `(r ∈ Int)`. Unnamed, the group holds
  only its set, `(Int)`. In the result position a lone term is a set, since a result binder with no set
  and no ψ would claim nothing; a name needs `∈`. `result` as a keyword goes.
- **A tuple result is a group**, `(q ∈ Int, m ∈ 0..b-1)`, the sugar ADR 0051 named for the pattern
  tuple(q, m) ∈ Int × 0..b−1.
- **D and J have one header**, different only in their keywords and in what separates the body: `{ }`
  against `==`.

**Finding 2. A bound is the parameter's set, and a binder's set may mention the binders before it.**
Sequential binders form a telescope (de Bruijn 1991; Martin-Löf's dependent sums): P is
{x̄ : x₁ ∈ S₁ ∧ x₂ ∈ S₂(x₁) ∧ …}, so a later set may name an earlier parameter:

```go
fn get(a ∈ Array(Int), i ∈ 0..len(a)-1) (Int) { a[i] }                                // J
fn dot(p ∈ Array(F64), q ∈ Array(F64)) (F64) where len(q) == len(p) { … }
```

`get`'s `where` is gone into `i`'s set. `dot` keeps a `where`, for the one kind of claim a bound
cannot state, a relation between two parameters: 20 of the 89. A relation could also be a dependent set
on the later parameter, `q ∈ { t ∈ Array(F64) : len(t) == len(p) }`, but that writes a comprehension
for an equation, and the `where` reads better. **Recommended: a bound in the binder's set, a relation
in `where`.**

**Finding 3. One equality in a formula, and it is the logic's.** In a formula `==` (J) or `=` (D) is
≐, the identity of values at any sort. The computation's equality is its restriction to the sorts whose
equality it decides (integers, booleans, tags), and the two agree there: on such a sort the
computation's `==` is the characteristic function of ≐. So no second glyph is needed. A comparison in
the computation at a sort it cannot decide (two tables) is refused, by the same rule W6 applies to a
membership test.

**Finding 4. A formula is an expression and four more things**, legal only where a claim stands:

| | J | D |
|---|---|---|
| quantifiers | `forall x ∈ S : φ`, `exists x ∈ S : φ` | `∀ x ∈ S : φ`, `∃ x ∈ S : φ` |
| implication, equivalence | `==>`, `<==>` (Dafny's) | `⇒`, `≡` |
| the connectives | `&&`, `\|\|`, `!` (the computation's) | `∧`, `∨`, `¬` |
| membership in any set | `e ∈ S` | `e ∈ S` |
| a specification function | `sum(a, 0, i)` | `sum(a, 0, i)` |

The quantifier's colon is installment 4's: Church's ∀ applied to λx. φ, the colon set-builder's.
Several binders, `forall i ∈ 0..n-1, j ∈ 0..n-1 : …`, quantify over their product. The claim positions
are `where`, `ensure`, `invariant`, `assert`, a lemma, a specification function's body, and the
condition of a set the program does not compute.

**Finding 5. Three definitions, one header.** A computation, a specification and a proof differ in what
their body is, not in their claim:

| | J | D | its body | precedent |
|---|---|---|---|---|
| a computation | `fn f(…) (…) where … ensure … { e }` | `f(…) (…) WHERE … ENSURE … == e` | runs; its claim is proven | Dafny's `method` |
| a specification | `spec f(…) (…) { e }` | `SPEC f(…) (…) == e` | defines a value of the logic; may recurse; never runs | Dafny's `function`, TLA+'s `RECURSIVE` |
| a lemma | `lemma L(…) where φ ensure ψ { steps }` | `LEMMA L(…) WHERE φ ENSURE ψ PROOF steps` | a proof: assertions, cases, lemma uses | Dafny's `lemma`, TLAPS's `THEOREM … PROOF` |

**A lemma is used like a call, by the same rule.** `assert L(a, i)` obliges L's φ at the arguments and
gains its ψ: ADR 0028's obligation at a call and postconditions.md's exact swap, for a definition whose
value is a proof. **A lemma that uses itself on smaller arguments is a proof by induction**, its
well-foundedness the same size-change check a specification function's recursion takes (Dafny's
inductive lemmas; Curry–Howard, a recursive proof is an induction). That is how a program climbs where
settypes §5's cliff is induction.

**Finding 6. `assert` proves, and `expect` checks.** The two halves are already two words:
- `assert φ` is static: proven from the path's facts, then a fact. It never runs, ε erases it, and when
  it cannot be proven the program is refused. Dafny's `assert`, a TLAPS proof step.
- `expect p = e else w` is dynamic: it runs, and on failure abandons (ADR 0040).

A reader from C, Go or Python expects `assert` to run. The confusion fails safe: an `assert` written
as a run-time check of input is refused, not silently skipped, and the refusal can name `expect`.

**Finding 7. `the` leaves the surface.** It has no use in a program (0), and a claim on an intermediate
value is a binder's set, `b ∈ 0..255 = src[i]` (installment 2's P3). The 9 uses in target files are
claims on a host's result, which a header's result set states. abstract.md keeps the operator, for μ.

**Finding 8. No claim is not ∈ V.** A binder without a set claims nothing, and its value is known only
by what the analyses derive; `any` is a set, every value, written (settypes §10). The target files have
both (`any` 192 times), and they stay two things.

### 5.5 Five programs, two notations

**D** and **J**, in the forms of installments 1–4 and this installment's proposals; **T**, today's,
where it can write the program at all.

---

**G1 — a host declaration with a precondition and a postcondition**
(`targets/go/encoding/hex.oro`'s `EncodedLen`).

```lisp
(sig EncodedLen ((n int)) int pure                                              ; T
  (where (and (<= -4611686018427387904 n) (<= n 4611686018427387903)))
  (ensures (= result (* 2 n)))
  (host expr "hex.EncodedLen(%s)" (import "encoding/hex")))
```
```
EncodedLen(n ∈ -4611686018427387904..4611686018427387903) (r ∈ Int)             \\ D
  ENSURE r = 2 * n  PURE
  == HOST expr "hex.EncodedLen(%s)" IMPORT "encoding/hex"
```
```go
fn EncodedLen(n ∈ -4611686018427387904..4611686018427387903) (r ∈ Int)          // J
    ensure r == 2 * n  pure
{
    host expr "hex.EncodedLen(%s)" import "encoding/hex"
}
```

---

**G2 — bounds in the binders' sets, a relation in `where`.**

```lisp
(sig get ((a (array int)) (i int)) int                                          ; T
  (where (and (<= 0 i) (< i (len a)))))
(def get (a i) (a i))

(sig dot ((p (array f64)) (q (array f64))) f64 (where (= (len p) (len q))))
```
```
get(a ∈ Array(Int), i ∈ 0..Len(a)-1) (Int) == a[i]                              \\ D

dot(p ∈ Array(F64), q ∈ Array(F64)) (F64) WHERE Len(q) = Len(p) == …
```
```go
fn get(a ∈ Array(Int), i ∈ 0..len(a)-1) (Int) { a[i] }                          // J

fn dot(p ∈ Array(F64), q ∈ Array(F64)) (F64) where len(q) == len(p) { … }
```

---

**G3 — a specification function, an invariant and a postcondition** (abstract.md §12's `total`). T has
no spelling.

```
SPEC sum(a ∈ Array(Int), i ∈ Nat, j ∈ Nat) (Int) ==                              \\ D
  IF i ≥ j THEN 0 ELSE a[i] + sum(a, i + 1, j)

total(xs ∈ Array(0..1000)) (r ∈ Nat) ENSURE r = sum(xs, 0, Len(xs)) ==
  LOOP i == 0, s == 0 INVARIANT s = sum(xs, 0, i) ∧ s ≥ 0 :
      i ≥ Len(xs) → s
   [] OTHER       → AGAIN(i + 1, s + xs[i])
```
```go
spec sum(a ∈ Array(Int), i ∈ Nat, j ∈ Nat) (Int) {                              // J
    if i >= j { 0 } else { a[i] + sum(a, i + 1, j) }
}

fn total(xs ∈ Array(0..1000)) (r ∈ Nat) ensure r == sum(xs, 0, len(xs)) {
    loop i = 0, s = 0 invariant s == sum(xs, 0, i) && s >= 0 {
        i >= len(xs) -> s
        else         -> again(i + 1, s + xs[i])
    }
}
```

---

**G4 — a lemma proven by induction, and used.** `sum` unfolds from the front, so preserving `total`'s
invariant needs its other end: sum(a, k, j) = sum(a, k, j − 1) + a[j − 1]. That needs induction, which
no procedure decides, so a lemma states it and proves it by using itself on a shorter range.

```
LEMMA sum_last(a ∈ Array(Int), k ∈ Nat, j ∈ 1..Len(a)) WHERE k < j               \\ D
      ENSURE sum(a, k, j) = sum(a, k, j - 1) + a[j - 1]
PROOF IF k + 1 < j THEN ASSERT sum_last(a, k + 1, j) ELSE ⟨⟩

\* in total's loop, the arm that goes round:
   [] OTHER → ASSERT sum_last(xs, 0, i + 1) ; AGAIN(i + 1, s + xs[i])
```
```go
lemma sum_last(a ∈ Array(Int), k ∈ Nat, j ∈ 1..len(a)) where k < j              // J
    ensure sum(a, k, j) == sum(a, k, j - 1) + a[j - 1]
{
    if k + 1 < j { assert sum_last(a, k + 1, j) }       // the induction hypothesis
}

// in total's loop, the arm that goes round:
        else -> { assert sum_last(xs, 0, i + 1); again(i + 1, s + xs[i]) }
```

The proof's steps: sum(a, k, j) unfolds to a[k] + sum(a, k + 1, j). When k + 1 < j, the hypothesis
rewrites sum(a, k + 1, j) as sum(a, k + 1, j − 1) + a[j − 1], and sum(a, k, j − 1) folds back to
a[k] + sum(a, k + 1, j − 1). When k + 1 = j both sides are a[k]. Each step is congruence with sum's
defining equation, inside the decided fragment.

---

**G5 — a quantifier in a postcondition**: a search's failure means the key is nowhere. T has no
spelling.

```
find(xs ∈ Array(Int), k ∈ Int) (r ∈ -1..Len(xs)-1)                              \\ D
  ENSURE (r = -1 ⇒ ∀ i ∈ 0..Len(xs)-1 : xs[i] ≠ k) ∧ (r ≥ 0 ⇒ xs[r] = k) ==
  LOOP i == 0 :
      i ≥ Len(xs) → -1
   [] xs[i] = k   → i
   [] OTHER       → AGAIN(i + 1)
```
```go
fn find(xs ∈ Array(Int), k ∈ Int) (r ∈ -1..len(xs)-1)                           // J
    ensure (r == -1 ==> forall i ∈ 0..len(xs)-1 : xs[i] != k) && (r >= 0 ==> xs[r] == k)
{
    loop i = 0 {
        i >= len(xs) -> -1
        xs[i] == k   -> i
        else         -> again(i + 1)
    }
}
```

The postcondition is in the array property fragment (Bradley, Manna and Sipma 2006): a universal over
an index range of a table, decided. The loop's invariant, `forall j ∈ 0..i-1 : xs[j] != k`, is the one
Houdini-style inference would have to find; the program may state it in the slot.

### 5.6 Measured

`go run gauntlet/surface/tokens.go -section 5`, D and J over G1–G5 (T can write only G1 and G2):

| notation | tokens | × D | lexemes | × D | depth | closing run | lines |
|---|---:|---:|---:|---:|---:|---:|---:|
| D | 430 | 1.00 | 414 | 1.00 | **2** | 2 | **21** |
| J | 456 | 1.06 | 422 | 1.02 | 4 | 4 | 31 |

- **Claims cost the two notations about the same**: J 2% more lexemes than D, the difference in its
  keywords (`ensure`, `forall`) against D's glyphs, and in braces.
- **J takes half again as many lines**, its bodies braced and its headers' clauses on lines of their own.
- **The comparison with today's notation is not a ratio**: of the five programs T writes two (G1, G2).
  The other three need an assertion, an invariant, a specification function, a lemma or a quantifier,
  none of which T has.

### 5.7 Open, for hamza

**Decided** (hamza, 2026-10-13: *"I agree to all 8 recommendations"*), as
[ADR 0062](decisions/0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md).

1. **The header is the membership statement** (Finding 1): parameters with their sets, the result a
   binder `(r ∈ U)` or its set alone `(U)`, `where φ`, `ensure ψ`; `result` as a keyword goes; one header
   in D and J. *Recommended.*
2. **A bound is the binder's set, and a later binder's set may name an earlier one** (Finding 2); a
   relation between parameters stays in `where`. *Recommended.*
3. **One equality in a formula, the logic's** (Finding 3), the computation's its restriction; an
   equality test at a sort the computation cannot decide is refused. *Recommended.*
4. **A formula's own forms** (Finding 4): J `forall`/`exists x ∈ S : φ`, `==>`, `<==>`; D TLA+'s; legal
   only in claim positions. *Recommended.*
5. **`fn`, `spec` and `lemma` share the header** (Finding 5); a lemma is used by `assert L(…)`, and a
   recursive lemma is an induction. *Recommended.*
6. **`assert` proves, `expect` checks** (Finding 6). *Recommended.*
7. **`the` leaves the surface** (Finding 7); abstract.md keeps it for μ. *Recommended.*
8. **No claim stays distinct from `any`** (Finding 8). *Recommended.*


## Installment 6 — declarations and modules

hamza: *"go ahead with installment 6. always mathematical and algebraic"*, after the assessment of
2026-10-14 set its rule: each remaining installment is tested on a real program, not on programs
written to be counted. This one's is `examples/os/du.oro`, a directory walk, written first in today's
notation and checked against its definition and against Go's `filepath.WalkDir`
([du-2026-10-14](../gauntlet/results/du-2026-10-14.md)). It needed three host packages declared, so
it is a declarations program as much as a program, which is this installment's subject: modules,
imports and exports, variants, host declarations and the model's own forms.

### 6.1 What declarations must carry

From abstract.md §2.6, theories.md and target-system.md:

| | the form | what it is | the algebra |
|---|---|---|---|
| **a definition** | def(f; e) | a name bound to a value; a function's is a named λ carrying its claims | a definitional extension, conservative (settypes §4.2) |
| **a host declaration** | def(f; λ(…); host(…)) | an operation symbol the model interprets by host text | an axiom checked against the host (ADR 0022) |
| **a host type** | `(type N (host "…"))` | a set the model supplies a carrier for | a sort's carrier in the model |
| **a manifest type** | `(type N S)` | a name for a set, unfolded by δ | a definitional extension at L1 (typevalues.md §1) |
| **a host constant** | `(const N v (host "…"))` | the host's name, and the value the analysis reads | theories.md §2's conditional cell |
| **a variant** | variant(n; b̄; c₁(S̄₁) … cₖ(S̄ₖ); success c?; relevant?) | a finite sum of tagged sets, with role markers | a coproduct; the constructors its injections, `case` its copairing |
| **an inclusion** | `(implements T I…)` | T ⊆ I₁ ∩ … ∩ Iₙ | an axiom of the order-sorted signature (Goguen and Meseguer 1992) |
| **a module** | module(path; decl*) | a theory presentation: names with their classifiers, definitions and axioms | a theory of an institution (Goguen and Burstall 1992) |
| **an import** | use(path; alias) | the imported theory's names under a prefix | an injective signature morphism σₐ : Σₚ → Σₘ, f ↦ a.f |
| **an export** | export(f̄) | what other modules may name | the reduct m\|S along the inclusion S ⊆ Σₘ (Bergstra, Heering and Klint's module algebra, 1990) |
| **a fragment** | provides(T; path; decl*) | part of target T's model of a module | glue ⊔ within a layer, override ▷ between layers (target-system.md §2) |

**Two theorems the surface must not break.**
- **Imports are a coproduct.** Qualified imports make each σₐ injective with disjoint images, so a
  module's signature is Σₘ = Σ_own + Σₐ₁ + … + Σₐₙ, and a coproduct is commutative and associative up
  to isomorphism: **the order of `use`s means nothing** (modules.md §3's reason for qualified imports).
- **A module's meaning is the reduct of its theory.** Nothing outside a module observes its unexported
  names, and resolution erases paths and aliases before reduction (ADR 0011, theories.md §3.5).
  **A path, an alias, a file and a layer are names, not content**: a spelling may move them as long as
  the qualified names it resolves to are the same.

### 6.2 Measured

`go run gauntlet/surface/decls.go`, over the programs and, separately, the target files:

| | programs | targets |
|---|---:|---:|
| files | 160 | 35 |
| definitions / with a `sig` beside them | 603 / 112 | — |
| host declarations (`sig` with a host body): `expr` / `stmt` | 33: 24 / 9 | 890: 804 / 86 |
| `export` forms / names exported | 109 / 149 | — |
| `use` / with no alias / an alias equal to the last segment / another alias | 244 / 146 / **38** / 60 | 21 / 21 / 0 / 0 |
| … `use` of a companion module (`go/bufio/Scanner`) | **40** | — |
| `(module …)` outermost / its path the file's / another path | 8 / 5 / 3 | 52 / **15** / **37** |
| `(module …)` nested (a companion or a child) | 0 | 23 |
| `(target NAME …)` wrappers / `(provides T M …)` fragments | 0 / 8 | 35 / 0 |
| `variant` / constructors / with a payload / without / with type parameters | 8 / 19 / 13 / 6 / 4 | — |
| `type`: a host type / a manifest type | — | 80 / 8 |
| `implements` / `const` / `include` | — | 13 / 11 / 3 |
| model data: `repr` / `structural` / `backend` / `link` / `fact` | — | 35 / 6 / 1 / 1 / 1 |

Read one at a time:
- **The five program modules whose path is the file's are every library file** (`lib/result`, `lib/os`,
  `lib/num/u128`, `lib/num/vec`, `lib/win/fmt`). The loader requires the header and refuses any other
  path (`core/reduce.go`, "a library file declares exactly the module its path names"). The other three
  are an entry file's (`examples/modules.oro`, two) and `examples/tally/tally.oro`.
- **The 15 target modules whose path is the file's are Go's packages** (`targets/go/io/fs.oro` holds
  `go/io/fs`). The other 37 are grouped by theme: `targets/java/lang.oro` holds ten modules
  (`java/String`, `java/Math`, …), `targets/js/methods.oro` four, and `targets/go/builtin.oro`,
  `u64.oro` and `bigint.oro` add to the root module `go`. In a target, a file is the glue of its forms
  (target-files.md §1), so where a module is written carries nothing.
- **All 13 `implements` sit at the end of their file, outside the type's module**, by absolute path;
  2 of them include a language set in a host set (`string` in `go.bytestring`, `any` in `go.Value`).
- **No module exports some of a variant's constructors and not others.** `lib/os` exports all four of
  `os-error`'s; the other variants are private to entry files.
- **An export list filters terms and constructors, not types.** `lib/result` exports `ok`, `err` and
  `err-map`, and the type `result.result` is named 95 times from other modules.

### 6.3 The holes and misfits

**Restatements the loader checks** (fit-research §1.2's fifth misfit, a checked invariant):
- a library file's `(module X)` names the path it was found at, 5 of 5, and a different X is refused.
  theories.md §3.1 says a library file "declares no header"; modules.md §0 and the loader require one.
- a target file's `(target go …)` names its directory, 35 of 35, and a different name is refused ("a
  directory is one target");
- an export names a definition written elsewhere in the file, 149 names, and a name with no definition
  is refused.

**Synonyms:**
- `(use go/os as os)` and `(use go/os)` are one import, 38 times; du alone has three (`os`, `filepath`,
  `fmt`), written before ADR 0025's default was read;
- a set named in a program is `def`, `(def Byte (int 0 255))` (ADR 0063); in a target it is `type`.

**A homonym.** `type` names two different things: a sort the model gives a carrier
(`(type DirEntry (host "fs.DirEntry"))`, 80), and a definition of a set unfolded by δ
(`(type int32 (int -2147483648 2147483647))`, 8).

**Five words for one record.** theories.md §1.1 reads every declaration as x : A [= t] [↦ ρ], a name,
its classifier, an optional definiens and an optional realization. Today's notation spells that record
with `def`, `sig`, `sig … (host …)`, `type` (twice) and `const`, and the variant as a sixth form.

**A companion is imported by its own path.** theories.md §3.4 names a companion's member
`go/io.Writer.Write`, a type's operations under the type's name. A program cannot write that:
`fs.DirEntry.Name` is "not bound", and du imports `go/io/fs/DirEntry`, `…/FileInfo` and `…/FileMode` as
three modules. 40 of the corpus's 244 imports are companions.

**An inclusion apart from its type, spelled as a verb.** `(implements go/strconv.NumError error)` is
the axiom NumError ⊆ error.

**One hole du found.** A program cannot name its own set: `(def Count (int 0 4611686018427387903))` is
accepted at the definition and refused at the first `+`, "x is Count, but int is required here". A
program's own sets come with types-as-values step 2 (typevalues.md §1), so du writes its bound three
times and its constant six.

### 6.4 Findings

**Finding 1. A file is a theory named by its path; a fragment names what it realizes.** The two kinds
of file are different objects, and the spelling should say which:
- **A library or entry file is one theory, and its path is its name** (ADR 0025's "the file is the
  path", completed). It has no `module` header: the five headers are restatements the loader checks,
  and theories.md §3.1 was right. A child module, a companion, or an entry file's extra modules are
  nested blocks: `module NumError { … }` in J, TLA+'s nested `---- MODULE NumError ---- … ====` in D.
- **A target file is a set of fragments of one model**, and its location carries nothing: 37 of its 52
  outermost modules are not its path, and must not be forced to be (Java's `java/lang` would split into
  ten files). So a fragment names its module, `module go/io/fs { … }`, with the absolute path.
- **The target is the directory.** The 35 wrappers go. A fragment outside the target's directory, in a
  library, names its target too: `provides go os/host { … }`. A file in `targets/T/` is a sequence of
  `provides T M { … }` with T read off the directory, written `module M { … }`, which is what
  `lib/os/go.oro` already says of `provides`: "exactly `(target T (module M decl…))`".

**Finding 2. An import always carries its alias, and a companion is reached through its type.**
- **The tree is use(path; alias), the alias filled by the reader** with the path's last segment when
  the source omits it, and the printer omits an alias equal to that default. So `use go/os as os` and
  `use go/os` read to one tree and print as the second: the 38 are not refused, they are normalized by
  the rendering's round-trip law (abstract.md §9.1), as redundant parentheses are.
- **`alias.T.m` names member m of T's companion**, theories.md §3.4's spelling. ADR 0063 put a set and
  its companion under one name in one namespace, so `fs.DirEntry` is the set and `fs.DirEntry.Name` its
  operation; `use go/io/fs` is enough. A companion's own import stays legal, for a short alias
  (`use go/io/fs/DirEntry as de`), and becomes optional: 40 imports.

**Finding 3. `export` is a marker on the definition.** The reduct needs a subset S ⊆ Σₘ. A list
presents S by its elements; a marker presents it by its characteristic function, pointwise:
- each exported name is written once, at its definition, and the refusal of an export with no
  definition disappears, since a marker cannot name nothing;
- 149 of the 603 definitions are exported, a quarter, so marking the exported minority writes 149
  markers where marking the private majority (TLA+'s `LOCAL`) would write 454;
- **it applies to the one namespace** (ADR 0063): exporting a set exports the set and its tags. Today
  every type is visible whatever the list says; under the marker, a set nobody exports is private.
  Nothing measured is lost: no module exports part of a variant, and types are extensional
  (ADR 0049), so a set's tags are how its values are written, not a representation to hide;
- J `export fn main()`, `export set result(T, E) = …`; D `EXPORT main() == …`.

Against it: a list shows the interface at a glance, as ML's signatures and Modula-2's definition
modules do. But the reduct is computable, so a tool prints the interface, and a list is a second copy
that drifts; the 109 lists are where the loader's refusal comes from.

**Finding 4. A declaration is one form: a header, and one body that is an expression or a host.** The
record x : A [= t] [↦ ρ] has theories.md §2's four cells, and they are the body's shape:

| body | cell | J | D |
|---|---|---|---|
| an expression | defined | `fn f(…) (U) { e }`, `let c = e`, `set S = e` | `f(…) (U) == e`, `c == e`, `S == e` |
| a host | host | `fn f(…) (U) pure { host expr "…" }`, `set S = host "…"` | `… == HOST expr "…"`, `S == HOST "…"` |
| a host, with the value as a claim | conditional | `let MaxRune ∈ {1114111} = host "utf8.MaxRune"` | `MaxRune ∈ {1114111} == HOST "utf8.MaxRune"` |
| none | a claim, covered elsewhere | `fn f(…) (U)` | `f(…) (U)` |

- **`type` dissolves.** A host type's carrier is a host body, `set DirEntry = host "fs.DirEntry"`, and a
  manifest type is a set definition, `set int32 = -2147483648..2147483647`: the homonym is gone, and
  the program's `def` and the target's `type` for a named set are one form.
- **A host constant's value is a claim**: MaxRune ∈ {1114111}, a singleton. That is exactly what
  theories.md §8.3 says it is for, "the emitter writes the host's name and the analysis reads the
  value": ρ prints the body, and the logic reads the claim. R2 already keeps such a declaration from
  being unfolded, so the value was never a definiens. The survey already reads one so, "a primitive of
  no arguments whose result is the EXACT range [v, v]" (`gauntlet/stdlib/survey.go`), and a hand
  declaration is compared with the survey's like any other.
- **J marks a set's definition `set`**, as it marks a function's `fn`, and each keyword both names and
  forms: `fn f(x) { … }` names a λ and `fn(x) { … }` forms one; `set S = …` names a set and
  `set { x ∈ S : P }` forms one (ADR 0061). Criterion E favours it: a set is erased by staging (Law S),
  and the keyword shows the reader what does not survive. A top-level value is `let c = e`, the
  definitional binder of ADR 0054 at the module's level. **D marks nothing**, as TLA+ does not: a set
  is a value, `S == e`.

**Finding 5. A variant is a set definition: a union of tagged sets.** ADR 0059 made a tag a value,
`'name`. Then 'c(S) is the set {'c} × S, a tag alone is the singleton {'c}, and

```go
export set os_error = 'not_found | 'exists | 'permission | 'other(error)          // J
```
```
EXPORT os_error == 'not_found ∪ 'exists ∪ 'permission ∪ 'other(error)            \* D
```

- **Distinct tags make the union disjoint, so it is the coproduct**, settypes' partition theorem; ADR
  0042's tag-and-slots representation is ρ of that set, chosen as for any set. `case` is the copairing.
- **Nothing is generated.** Today a variant's constructors are generated definitions (sums.md §3); a
  tag is already a value, so `'other(e)` is a tag applied, and the set says which applications belong.
- **Type parameters are a static function on sets**: `set result(T, E) = 'ok(T) | 'err(E)` is
  result ∈ Π(T, E). Set, and `result.result(FileInfo, error)` is an application, ADR 0063's
  instantiation.
- **The role markers are clauses after the set**, as `pure` follows a header: J `success 'ok relevant`,
  D `SUCCESS 'ok RELEVANT`.
- **Closed, finite, non-recursive**, as today: a set defined through itself is refused by theories.md
  §1.3's well-founded order. **A tag belongs to one set definition per module**, as a constructor does
  today; two sets sharing a tag would mean set inclusion (B ⊆ A), a door no program asks to open.

**Finding 6. An inclusion is an axiom, and only a model states one.** `implements T I₁ … Iₙ` is
T ⊆ I₁ ∩ … ∩ Iₙ:

```go
axiom NumError <= error                                                          // J
axiom File <= Reader & Writer & Closer
```
```
AXIOM NumError ⊆ error                                                           \* D
```

- J's `<=` and `&` are ADR 0060's, Python's subset and intersection; D writes TLA+'s.
- **Only a target file or a fragment may state one.** ADR 0049 extends a program by definitions and
  proven lemmas only; a model's axioms are what the tooling checks against the host (ADR 0022), as it
  checks every host declaration. The keyword says it is checked, not proven.
- Rejected: the inclusion as the set definition's claim, `NumError ∈ subsets(error) == HOST "…"`. It is
  local, but it covers 11 of the 13 (the other two include a language set defined elsewhere), and a
  header's claim is an obligation proven on the body, which a host body cannot discharge.

**Finding 7. The model's data keeps its words.** `repr` (35), `structural` (6), `backend`, `link`,
`fact`, `underlying`, `max-len`, `big-repr`, `int-repr` are clauses of ρ and the model's parameters
(abstract.md §2.6, Σ_T), written only by a target's author. They are spelled one to one, each with its
sets in the notation: `repr 0..255 host "byte"` in J, `REPR 0..255 HOST "byte"` in D.

**Finding 8. A companion is a set's theory, written as a nested module.** `module NumError { … }` holds
NumError's operations, its constructor `new` included (ADR 0063: a struct literal is the companion's
`new`). Each operation restates `self ∈ NumError`. A section variable (Coq's `Variable`, Lean's
`variable`) would bind it once, but `new` has no `self`, so it is not proposed.

### 6.5 Five programs, three notations

**T** today's, **D** and **J** in the decided forms of installments 1–5 and this installment's
proposals. Names are translated mechanically (`not-found` → `not_found`, `not-found?` → `is_not_found`);
their case is installment 8's.

---

**H1 — du**, the installment's program (`examples/os/du.oro`, its comments left out).

```lisp
(use go/os as os)                                                               ; T
(use go/io/fs/DirEntry as de)
(use go/io/fs/FileInfo as fi)
(use go/io/fs/FileMode as fm)
(use go/path/filepath as filepath)
(use go/fmt as fmt)
(use result)
(export main)

(def cap 4096)

(sig sat ((x (int 0 4611686018427387903)) (y (int 0 4611686018427387903))) (int 0 4611686018427387903))
(def sat (x y) (let s (+ x y) (if (<= s 4611686018427387903) s 4611686018427387903)))

(def length (s) (if (< s 0) 0 (if (< 4611686018427387903 s) 4611686018427387903 s)))

(def contribution (ent)
  (case (de.Info ent)
    (result.err e)    (seq (ignore e) (tuple 0 0))
    (result.ok info)  (if (fm.IsRegular (fi.Mode info))
                        (tuple (length (fi.Size info)) 1)
                        (tuple 0 0))))

(def failed (r)
  (case r
    (result.ok u)  0
    (result.err e) (seq (ignore e) 1)))

(def walk (root)
  (local stk (table cap (fn (k) root))
    (let (tuple es0 r0) (os.ReadDir root)
      (loop ((stk stk) (sp 0) (ents es0) (k 0) (dir root)
             (bytes 0) (files 0) (dirs 1) (bad (failed r0)) (lost 0))
        (and (< k (len ents)) (and (de.IsDir (ents k)) (< sp cap)))
          (again (set stk sp (filepath.Join dir (de.Name (ents k))))
                 (+ sp 1) ents (+ k 1) dir bytes files (sat dirs 1) bad lost)
        (and (< k (len ents)) (de.IsDir (ents k)))
          (again stk sp ents (+ k 1) dir bytes files (sat dirs 1) bad (sat lost 1))
        (< k (len ents))
          (let (tuple b f) (contribution (ents k))
            (again stk sp ents (+ k 1) dir (sat bytes b) (sat files f) dirs bad lost))
        (< sp 1)
          (tuple bytes files dirs bad lost)
        else
          (let next (stk (- sp 1))
               (tuple es r) (os.ReadDir next)
            (again stk (- sp 1) es 0 next bytes files dirs (sat bad (failed r)) lost))))))

(def main ()
  (let av (os.Args)
    (if (< (len av) 2)
      (fmt.Println "usage: du DIR")
      (let (tuple bytes files dirs bad lost) (walk (av 1))
        (seq (fmt.Println bytes "bytes in" files "files and" dirs "directories")
             (if (< 0 bad) (fmt.Println bad "directories could not be read") (tuple))
             (if (< 0 lost) (fmt.Println lost "directories were not entered: more than" cap "waiting") (tuple)))))))
```
```
USE go/os, go/io/fs/DirEntry AS de, go/io/fs/FileInfo AS fi,                    \\ D
    go/io/fs/FileMode AS fm, go/path/filepath, go/fmt, result

cap == 4096

sat(x ∈ 0..4611686018427387903, y ∈ 0..4611686018427387903) (0..4611686018427387903) ==
  LET s == x + y IN IF s ≤ 4611686018427387903 THEN s ELSE 4611686018427387903

length(s) == CASE s < 0 → 0 [] 4611686018427387903 < s → 4611686018427387903 [] OTHER → s

contribution(ent) ==
  CASE de.Info(ent) OF
      result.'err(e)   → ignore(e) ; ⟨0, 0⟩
   [] result.'ok(info) → IF fm.IsRegular(fi.Mode(info)) THEN ⟨length(fi.Size(info)), 1⟩ ELSE ⟨0, 0⟩

failed(r) == CASE r OF result.'ok(u) → 0 [] result.'err(e) → ignore(e) ; 1

walk(root) ==
  LET stk := [j ∈ 0..cap-1 ↦ root]  ⟨es0, r0⟩ == os.ReadDir(root) IN
    LOOP sp == 0, ents == es0, k == 0, dir == root,
         bytes == 0, files == 0, dirs == 1, bad == failed(r0), lost == 0 :
        k < Len(ents) ∧ de.IsDir(ents[k]) ∧ sp < cap →
          stk[sp] := filepath.Join(dir, de.Name(ents[k])) ;
          AGAIN(sp + 1, ents, k + 1, dir, bytes, files, sat(dirs, 1), bad, lost)
     [] k < Len(ents) ∧ de.IsDir(ents[k]) →
          AGAIN(sp, ents, k + 1, dir, bytes, files, sat(dirs, 1), bad, sat(lost, 1))
     [] k < Len(ents) →
          LET ⟨b, f⟩ == contribution(ents[k]) IN
            AGAIN(sp, ents, k + 1, dir, sat(bytes, b), sat(files, f), dirs, bad, lost)
     [] sp < 1 → ⟨bytes, files, dirs, bad, lost⟩
     [] OTHER →
          LET next == stk[sp - 1]  ⟨es, r⟩ == os.ReadDir(next) IN
            AGAIN(sp - 1, es, 0, next, bytes, files, dirs, sat(bad, failed(r)), lost)

EXPORT main() ==
  LET av == os.Args() IN
    IF Len(av) < 2 THEN fmt.Println("usage: du DIR")
    ELSE LET ⟨bytes, files, dirs, bad, lost⟩ == walk(av[1]) IN
      fmt.Println(bytes, "bytes in", files, "files and", dirs, "directories") ;
      (IF 0 < bad THEN fmt.Println(bad, "directories could not be read") ELSE ⟨⟩) ;
      (IF 0 < lost THEN fmt.Println(lost, "directories were not entered: more than", cap, "waiting")
       ELSE ⟨⟩)
```
```go
use go/os                                                                       // J
use go/io/fs/DirEntry as de
use go/io/fs/FileInfo as fi
use go/io/fs/FileMode as fm
use go/path/filepath
use go/fmt
use result

let cap = 4096

fn sat(x ∈ 0..4611686018427387903, y ∈ 0..4611686018427387903) (0..4611686018427387903) {
    s = x + y
    if s <= 4611686018427387903 { s } else { 4611686018427387903 }
}

fn length(s) {
    case { s < 0 -> 0; 4611686018427387903 < s -> 4611686018427387903; else -> s }
}

fn contribution(ent) {
    case de.Info(ent) {
        result.'err(e)   -> { ignore(e); (0, 0) }
        result.'ok(info) -> if fm.IsRegular(fi.Mode(info)) { (length(fi.Size(info)), 1) } else { (0, 0) }
    }
}

fn failed(r) {
    case r { result.'ok(u) -> 0; result.'err(e) -> { ignore(e); 1 } }
}

fn walk(root) {
    var stk := array { root : _ ∈ 0..cap-1 }
    (es0, r0) = os.ReadDir(root)
    loop sp = 0, ents = es0, k = 0, dir = root,
         bytes = 0, files = 0, dirs = 1, bad = failed(r0), lost = 0 {
        k < len(ents) && de.IsDir(ents[k]) && sp < cap ->
            { stk[sp] := filepath.Join(dir, de.Name(ents[k]))
              again(sp + 1, ents, k + 1, dir, bytes, files, sat(dirs, 1), bad, lost) }
        k < len(ents) && de.IsDir(ents[k]) ->
            again(sp, ents, k + 1, dir, bytes, files, sat(dirs, 1), bad, sat(lost, 1))
        k < len(ents) ->
            { (b, f) = contribution(ents[k])
              again(sp, ents, k + 1, dir, sat(bytes, b), sat(files, f), dirs, bad, lost) }
        sp < 1 -> (bytes, files, dirs, bad, lost)
        else ->
            { next = stk[sp - 1]
              (es, r) = os.ReadDir(next)
              again(sp - 1, es, 0, next, bytes, files, dirs, sat(bad, failed(r)), lost) }
    }
}

export fn main() {
    av = os.Args()
    if len(av) < 2 {
        fmt.Println("usage: du DIR")
    } else {
        (bytes, files, dirs, bad, lost) = walk(av[1])
        fmt.Println(bytes, "bytes in", files, "files and", dirs, "directories")
        if 0 < bad { fmt.Println(bad, "directories could not be read") }
        if 0 < lost { fmt.Println(lost, "directories were not entered: more than", cap, "waiting") }
    }
}
```

What the renderings show:
- **The stack leaves the loop** (ADR 0052): in T it is a loop variable threaded through five back edges
  with `stk` written in each; in D and J it is a state binder written once, `stk[sp] := …`, and each
  `again` has nine arguments, not ten.
- **The imports lose their restatements** (Finding 2): three of T's seven aliases are the default.
- **The set du cannot name** (§6.3) is written three times in each; under step 2 it is
  `set Count = 0..C` in J and `Count == 0..C` in D, and `sat`'s header is `(x ∈ Count, y ∈ Count)
  (Count)`.

---

**H2 — a library module with a parameterized variant** (`lib/result.oro`, its comments left out).

```lisp
(module result)                                                                 ; T

(export ok err err-map)

(variant (result T E) (ok T) (err E) (success ok) (relevant))

(def err-map (f r)
  (case r
    (ok v)  (ok v)
    (err e) (err (f e))))
```
```
EXPORT result(T, E) == 'ok(T) ∪ 'err(E)  SUCCESS 'ok  RELEVANT                  \\ D

EXPORT err_map(f, r) == CASE r OF 'ok(v) → 'ok(v) [] 'err(e) → 'err(f(e))
```
```go
export set result(T, E) = 'ok(T) | 'err(E)  success 'ok  relevant              // J

export fn err_map(f, r) {
    case r {
        'ok(v)  -> 'ok(v)
        'err(e) -> 'err(f(e))
    }
}
```

The header goes (Finding 1), the export list becomes two markers (Finding 3), and the variant is a
set definition whose tags are the module's own, so they are unqualified here and `result.'ok` outside
(ADR 0060).

---

**H3 — a sum of nullary and payload tags, and its classification** (`lib/os.oro`'s `os-error` and
`classify`; T's export of the four constructors is in a list of nine names, not shown).

```lisp
(variant os-error not-found exists permission (other error))                    ; T

(def classify (e)
  (cond (host.not-found? e)   not-found
        (host.exists? e)      exists
        (host.permission? e)  permission
        else                  (other e)))
```
```
EXPORT os_error == 'not_found ∪ 'exists ∪ 'permission ∪ 'other(error)           \\ D

classify(e) ==
  CASE host.is_not_found(e)  → 'not_found
    [] host.is_exists(e)     → 'exists
    [] host.is_permission(e) → 'permission
    [] OTHER                 → 'other(e)
```
```go
export set os_error = 'not_found | 'exists | 'permission | 'other(error)        // J

fn classify(e) {
    case {
        host.is_not_found(e)  -> 'not_found
        host.is_exists(e)     -> 'exists
        host.is_permission(e) -> 'permission
        else                  -> 'other(e)
    }
}
```

---

**H4 — host declarations, host types and companions** (`targets/go/io/fs.oro`, du's).

```lisp
(target go                                                                      ; T
  (module go/io/fs
    (type DirEntry (host "fs.DirEntry"))
    (type FileInfo (host "fs.FileInfo"))
    (type FileMode (host "fs.FileMode"))

    (module DirEntry
      (use result)
      (sig Name  ((self DirEntry)) bytestring pure (host expr "%s.Name()"))
      (sig IsDir ((self DirEntry)) bool pure (host expr "%s.IsDir()"))
      (sig Info  ((self DirEntry)) (result.result FileInfo error) (host expr "%s.Info()")))

    (module FileInfo
      (sig Name  ((self FileInfo)) bytestring pure (host expr "%s.Name()"))
      (sig Size  ((self FileInfo)) int64 pure (host expr "%s.Size()"))
      (sig Mode  ((self FileInfo)) FileMode pure (host expr "%s.Mode()"))
      (sig IsDir ((self FileInfo)) bool pure (host expr "%s.IsDir()")))

    (module FileMode
      (sig IsDir     ((self FileMode)) bool pure (host expr "%s.IsDir()"))
      (sig IsRegular ((self FileMode)) bool pure (host expr "%s.IsRegular()")))))
```
```
---- MODULE go/io/fs ----                                                       \\ D
DirEntry == HOST "fs.DirEntry"
FileInfo == HOST "fs.FileInfo"
FileMode == HOST "fs.FileMode"

---- MODULE DirEntry ----
USE result
Name(self ∈ DirEntry) (bytestring) PURE == HOST expr "%s.Name()"
IsDir(self ∈ DirEntry) (Bool) PURE == HOST expr "%s.IsDir()"
Info(self ∈ DirEntry) (result.result(FileInfo, error)) == HOST expr "%s.Info()"
====

---- MODULE FileInfo ----
Name(self ∈ FileInfo) (bytestring) PURE == HOST expr "%s.Name()"
Size(self ∈ FileInfo) (int64) PURE == HOST expr "%s.Size()"
Mode(self ∈ FileInfo) (FileMode) PURE == HOST expr "%s.Mode()"
IsDir(self ∈ FileInfo) (Bool) PURE == HOST expr "%s.IsDir()"
====

---- MODULE FileMode ----
IsDir(self ∈ FileMode) (Bool) PURE == HOST expr "%s.IsDir()"
IsRegular(self ∈ FileMode) (Bool) PURE == HOST expr "%s.IsRegular()"
====
====
```
```go
module go/io/fs {                                                               // J
    set DirEntry = host "fs.DirEntry"
    set FileInfo = host "fs.FileInfo"
    set FileMode = host "fs.FileMode"

    module DirEntry {
        use result
        fn Name(self ∈ DirEntry) (bytestring) pure { host expr "%s.Name()" }
        fn IsDir(self ∈ DirEntry) (Bool) pure { host expr "%s.IsDir()" }
        fn Info(self ∈ DirEntry) (result.result(FileInfo, error)) { host expr "%s.Info()" }
    }

    module FileInfo {
        fn Name(self ∈ FileInfo) (bytestring) pure { host expr "%s.Name()" }
        fn Size(self ∈ FileInfo) (int64) pure { host expr "%s.Size()" }
        fn Mode(self ∈ FileInfo) (FileMode) pure { host expr "%s.Mode()" }
        fn IsDir(self ∈ FileInfo) (Bool) pure { host expr "%s.IsDir()" }
    }

    module FileMode {
        fn IsDir(self ∈ FileMode) (Bool) pure { host expr "%s.IsDir()" }
        fn IsRegular(self ∈ FileMode) (Bool) pure { host expr "%s.IsRegular()" }
    }
}
```

The wrapper goes (Finding 1); `type` becomes a set with a host body (Finding 4); each declaration is
installment 5's G1 header with a host body.

---

**H5 — the model's own forms: a host type with its companion and its inclusion, and a host constant**
(`targets/go/strconv.oro`'s `NumError`, cut to two operations, and `targets/go/unicode/utf8.oro`'s
`MaxRune`).

```lisp
(target go                                                                      ; T
  (module go/strconv
    (type NumError (host "*strconv.NumError"))
    (module NumError
      (sig new ((err error) (func string) (num bytestring)) NumError
            (host expr "(&strconv.NumError{Err: %s, Func: %s, Num: %s})" (import "strconv")))
      (sig Err ((self NumError)) error pure (host expr "%s.Err" (import "strconv")))))
  (implements go/strconv.NumError error))

(target go
  (module go/unicode/utf8
    (const MaxRune 1114111 (host "utf8.MaxRune" (import "unicode/utf8")))))
```
```
---- MODULE go/strconv ----                                                     \\ D
NumError == HOST "*strconv.NumError"
AXIOM NumError ⊆ error

---- MODULE NumError ----
new(err ∈ error, func ∈ String, num ∈ bytestring) (NumError)
  == HOST expr "(&strconv.NumError{Err: %s, Func: %s, Num: %s})" IMPORT "strconv"
Err(self ∈ NumError) (error) PURE == HOST expr "%s.Err" IMPORT "strconv"
====
====

---- MODULE go/unicode/utf8 ----
MaxRune ∈ {1114111} == HOST "utf8.MaxRune" IMPORT "unicode/utf8"
====
```
```go
module go/strconv {                                                             // J
    set NumError = host "*strconv.NumError"
    axiom NumError <= error

    module NumError {
        fn new(err ∈ error, func ∈ String, num ∈ bytestring) (NumError) {
            host expr "(&strconv.NumError{Err: %s, Func: %s, Num: %s})" import "strconv"
        }
        fn Err(self ∈ NumError) (error) pure { host expr "%s.Err" import "strconv" }
    }
}

module go/unicode/utf8 {
    let MaxRune ∈ {1114111} = host "utf8.MaxRune" import "unicode/utf8"
}
```

The inclusion moves next to its type and says what it is (Finding 6); the constant's value becomes its
claim (Finding 4).

### 6.6 Measured

`go run gauntlet/surface/tokens.go -section 6`, over H1–H5 in T, D and J:

| notation | tokens | × T | lexemes | × T | depth | closing run | lines |
|---|---:|---:|---:|---:|---:|---:|---:|
| T | 1053 | 1.00 | 1052 | 1.00 | 9 | 7 | 91 |
| D | 1091 | 1.04 | 979 | 0.93 | **3** | **3** | **76** |
| J | 1032 | 0.98 | 1003 | 0.95 | 6 | 4 | 105 |

Split: H1 alone is `-section 6 -until '**H2 '`, and H2–H5 the difference:

| | T lexemes | D | × T | J | × T |
|---|---:|---:|---:|---:|---:|
| H1, du | 633 | 615 | 0.97 | 618 | 0.98 |
| H2–H5, the declarations | 419 | 364 | **0.87** | 385 | **0.92** |

- **The program costs all three about the same**, within 3%: what D and J add in commas and keywords
  they take back in parentheses. On it the state binder takes `stk` out of five back edges and the
  loop's head (ADR 0052), and three imports lose their aliases.
- **The declarations are where the notations differ**: D 13% and J 8% fewer lexemes than T, from the
  wrappers, the header, the export list and `sig`'s doubled parentheses. D's raw token count is the
  highest only because TLA+'s module rules, `----` and `====`, are a token per character; as a lexer
  reads them, D is the smallest.
- **What Findings 1–3 remove is not size**: a wrapper, a header or an export list is a few tokens. It
  is a restatement the loader has to check.
- **The depth is T's**: 9 in T against 3 in D and 6 in J, from `walk`'s loop inside a `local` inside a
  `let`, and the closing run follows it (7, 3, 4). J takes 15% more lines than T, its blocks closing on
  lines of their own; D takes 16% fewer.

### 6.7 Open, for hamza

1. **A file is a theory named by its path; a fragment names what it realizes** (Finding 1): no `module`
   header in a library or entry file; nested `module X { … }` in J, TLA+'s `---- MODULE X ---- … ====`
   in D; the target is the directory, its files fragments `module PATH { … }`; `provides T M { … }`
   elsewhere. *Recommended.*
2. **An import's alias is filled by the reader and omitted by the printer**, so a restated alias is
   normalized, not refused; **`alias.T.m` reaches a companion's member** through its type, as
   theories.md §3.4 already writes it (Finding 2). *Recommended.*
3. **`export` is a marker on the definition**, applying to the one namespace: an exported set exports
   its tags, and an unexported set is private (Finding 3). *Recommended.*
4. **One declaration form, a header and one body, an expression or a host** (Finding 4): `type`
   dissolves into set definitions; a host constant's value is a singleton claim; J writes `fn`, `let` and
   `set`, D writes none. *Recommended.*
5. **A variant is a set definition, a union of tagged sets**, its parameters a static function, its
   markers clauses after the set, one defining set per tag per module (Finding 5). *Recommended.*
6. **An inclusion is an axiom**, `axiom T <= I` in J and `AXIOM T ⊆ I` in D, legal only in a model
   (Finding 6). *Recommended.*
7. **The model's data keeps its words**, spelled in the notation's sets (Finding 7). *Recommended.*
8. **A companion is a nested module**, with no section variable for `self` (Finding 8). *Recommended.*
