# Facts: what the compiler may assume, who says so, and how it is used

> **Status, 2026-09-15: F1, F2 and F11's lower half are declarations**, `emit/lang-facts.oro`, instantiated
> by one algorithm in `emit/fact.go`; `seedDivAxioms` is deleted
> ([langfacts-2026-09-15](../gauntlet/results/langfacts-2026-09-15.md)). **And F6, F7, F8 — the
> remainder, three encodings of one law — are four facts** read by the interval layer through Theorem
> T with case splitting ([remfacts-2026-09-15](../gauntlet/results/remfacts-2026-09-15.md)). Still Go:
> F5, F10, F12 — **and those three are not facts, which moving the others is what showed**. F9, the mask,
> is two facts (`and-left`, `and-right`) read the same way.
>
> **Reclassified, each with its reason:**
> - **F5, the division contraction `|a/b| ≤ |a| / min|b|`, is Moore's interval quotient, not an axiom.**
>   Stated as a fact it is `b·q ≤ a` for `b ≥ 1`, a product of two extension terms, so it fails
>   covering and linearity at once. Over a box whose divisor excludes 0 it is exactly the interval
>   quotient, the best abstraction of `/` in the class §7.5 keeps for `+`, `−` and `·`. The sign facts it
>   also encodes (`a ≥ 0 ∧ b ≥ 1 → q ≥ 0`, `|q| ≤ |a|`) are corollaries of that quotient.
> - **F10, `x >> k = ⌊x / 2ᵏ⌋`, is F-C.** It equates two extension terms and needs `2ᵏ` as a compile-time
>   function of the literal `k`, which theories.md §7.6 reserves with `isqrt`.
> - **F12, `0·x = 0` for an unbounded `x`, is interval multiplication's convention for `0 · ±∞`**, the
>   operator's own abstraction and not a law a declaration could add.

Research, **decided 2026-09-15, nothing built**: hamza took **F-B**, and **kept F-C, F-D and F-E open**.
They are specified as reserved fragments in [spec/theories.md](spec/theories.md) §7.6, recognised
and refused by name. 2026-09-15, on hamza's *"should we research facts?
literature, math and algebra, and future extension?"*. It is written before
[spec/theories.md](spec/theories.md) §7, which it is meant to decide.

A fact is the one declaration whose mistake is a **wrong answer rather than a refusal**:
- one false fact makes a conjunctive fragment derive anything (postconditions.md, Lemma 1);
- a fact admitted in the wrong shape brings back Shen's proof search
  ([decidability-map.md](decidability-map.md) §5).

So this document does what maxlen-2026-08-28 did for octagons: **classify everything the compiler
already assumes before designing anything that adds to it.** §1 is that measurement. The rest follows
from it.

---

## 0. The answer in five sentences

1. **Of the 41 rules the analysis layer uses today, 12 are facts in the sense this document defines**
   — sentences about operators or data that could be written down — and **29 are the decision
   procedure and its induction principles**, which must stay the compiler's (§1).
2. **The 12 facts are scattered.** The remainder bound is written **three times in three encodings**,
   the square bound twice and the division bound twice. That is the shape of the recurring bug this
   repository records as *"two layers, and only one of them was ever told"* (§3).
3. **Every one of the 12 has the same logical form**: a guarded bound on one term that the linear
   fragment cannot express, in terms of terms it can. That is a **boundedness axiom of a local theory
   extension** (Sofronie-Stokkermans 2005). For such axioms, instantiating only on terms already
   present is sound, terminating and complete relative to the base procedure, and
   `seedDivAxioms` already does exactly that without the name (§2).
4. **One declaration can feed both analyses**: a boundedness fact induces a sound interval transfer
   function mechanically, so the scattering is avoidable (§3, Theorem T).
5. **`ensures` on a primitive is a fact of this form**, with the call as its trigger. So the research
   also points at the fix for the two limitations parked on it: postconditions on *pure* calls, and
   `length` moving into `ensures` (§6).

---

## 1. The inventory — the measurement

Every rule in `emit/refine.go`, `emit/linear.go`, `emit/monotone.go`, `emit/interval.go`,
`emit/sct.go` and `emit/target.go`'s element inference, read in place, 2026-09-15.

### 1.1 The classification, derived rather than chosen

A rule the analysis uses is one of four things, and they differ in **what they quantify over**:

| class | quantifies over | example | who may state it |
|---|---|---|---|
| **F — fact** | *values*: `∀x. G(x) → C(x)` | `k·(x/k) ≤ x` | a declaration, once this is built |
| **P — proof rule** | *formulas*: from these facts, conclude that | scale a fact by a positive integer | the compiler only |
| **I — induction principle** | *programs*: for every loop of this shape, this invariant holds | loop monotonicity | the compiler only |
| **S — semantics of the language's data** | *terms of `lang`*: what `build` or `array` means | a slot is zero or the last store | the compiler only, as part of `lang`'s definition |

The distinction is not taste. **A fact is a sentence in the theory; the other three are about how
sentences are used or where they come from.** Letting a program state a proof rule or an induction
principle is letting it extend the logic, which is exactly Shen's position and what
decidability-map.md refuses. Letting it state a fact extends the *theory*, in a fragment the
compiler's own procedure still decides.

### 1.2 Class F — facts (12)

| # | the fact | where | encoding |
|---|---|---|---|
| F1 | `k > 0, x ≥ 0 ⊢ k·(x/k) ≤ x` | refine.go:99 `seedDivAxioms` | linear, instantiated per quotient |
| F2 | `k > 0, x ≥ 0 ⊢ x ≤ k·(x/k) + k − 1` | refine.go:123 | linear, same |
| F3 | `x ≤ x·x` | refine.go:224 `squareBound` | linear, used only in the shape `x·x < e` |
| F4 | `x·x ≤ e ⊢ −⌊√e⌋ ≤ x ≤ ⌊√e⌋` | interval.go:2091 `narrowSquare` | interval, guard narrowing |
| F5 | `\|a / b\| ≤ \|a\| / min\|b\|`, and `a ≥ 0, b ≥ 1 ⊢ 0 ≤ a/b` | interval.go:148 `divI` | interval transfer |
| F6 | `\|a % b\| ≤ min(\|a\|, \|b\| − 1)`, sign of the dividend | interval.go:197 `remI` | interval transfer |
| F7 | `\|a % d\| ≤ \|d\| − 1` for a literal `d` | target.go:2471 `storedRange` | syntactic element range |
| F8 | `\|a % k\| ≤ k − 1` for the bignum remainder by a word | interval.go:1322 `transfer` | interval transfer, by name |
| F9 | `m ≥ 0 constant ⊢ 0 ≤ x & m ≤ m`; `a, b ≥ 0 ⊢ 0 ≤ a & b ≤ min(a, b)` | interval.go:3063 `andI` | interval transfer |
| F10 | `x ≥ 0, 0 ≤ k ≤ 62 ⊢ x >> k = ⌊x / 2ᵏ⌋` | interval.go:3100 `shrI` | interval transfer |
| F11 | `0 ≤ len a ≤ maxlen_T` | interval.go:1226; `isLenTerm` as the source of `x ≥ 0` in F1–F2 | interval, and implicit in the linear layer |
| F12 | `0·x = 0` for unbounded `x` | interval.go:122 `mulI` | interval transfer special case |

**Not counted as facts:** the transfers for `+`, `−`, `·` on bounded intervals, and `exactArith`
in `storedRange`. Those are the **best abstraction** of the operator, computed mechanically from its
definition: the image of a box under a monotone or bilinear map, Moore's interval arithmetic.
Nothing in them is a lemma someone chose. Division, remainder, `&` and `>>` have no such mechanical
form, so each bound there *is* a lemma, which is why they appear above.

### 1.3 Class P — proof rules of the decision procedure (13)

| # | rule | where |
|---|---|---|
| P1 | a fact `L + a ≤ 0` implies `L + b ≤ 0` when `a ≥ b` | linear.go:295 |
| P2 | one Farkas multiplier: scale a fact by a positive integer | linear.go:251 `scaleTo` |
| P3 | the sum of two facts | linear.go:308 |
| P4 | one Fourier–Motzkin elimination step | linear.go:330 |
| P5 | rewriting facts and goals by the same equations (congruence) | linear.go:285 |
| P6 | a disequality as a case split, `d ≠ 0 ⟺ d < 0 ∨ d > 0` | linear.go:186 |
| P7 | a conjunction assumed conjunct by conjunct, in erased form | refine.go:167 `assume` |
| P8 | an opaque atom discharged only by an identical atom | linear.go:107, refine.go:761 |
| P9 | a guard holds in its branch and its negation in the other | refine.go:865 `clauses`, interval.go:1934 `refine` |
| P10 | a conditional's value keeps what every branch satisfies (template join) | refine.go:620 `joinConditional` |
| P11 | a disequality moves an interval endpoint | interval.go:2030 `narrowEq` |
| P12 | join, widening and narrowing | interval.go:262–307 |
| P13 | a precondition must be *proven* before its postcondition is assumed | refine.go:735, :784 |

### 1.4 Class I — induction principles (8)

| # | principle | where |
|---|---|---|
| I1 | the relation `e ⊒ S` and Lemma A | monotone.go:26–51 |
| I2 | loop monotonicity, and its exit-bound corollary | monotone.go:62–77 |
| I3 | the reachable-set theorem for a self-contained position | monotone.go:367 |
| I4 | a counter that only adds non-negative literals stays at least its start | refine.go:898 |
| I5 | a threaded table's length is its initial length when every back edge agrees | refine.go:854 `againAgree` |
| I6 | size-change termination with an interval floor and orientation | sct.go |
| I7 | a derived step through a `let` and an `if` | monotone.go:501 `DerivedStep` |
| I8 | the trip count as a bound on an accumulator | interval.go:2916 `tripCount` |

### 1.5 Class S — the semantics of `lang`'s data (8)

| # | theorem of the semantics | where |
|---|---|---|
| S1 | a `build` slot holds the zero fill or the last `set` (read containment) | interval.go:1771 |
| S2 | an element read of a declared range lies in it | interval.go:437 |
| S3 | a map's value lies in the join of what was inserted | interval.go:1835 `insertedRange` |
| S4 | a literal table's length is its element count | interval.go:1640 `exactLen` |
| S5 | `build n`'s length is `n` | refine.go:1014 `valueLength` |
| S6 | a primitive's `(length N)` / `(length-of N)` gives its result's length | refine.go:1039 |
| S7 | a primitive's declared result range bounds its result | interval.go:1353 |
| S8 | a primitive's `where` is an obligation and its `ensures` an assumption | refine.go:735 |

**S6, S7 and S8 are already declarations** — facts a target states about a host call. They are
listed under S because the *mechanism* that reads them is fixed. §6 shows they are class F in
disguise.

### 1.6 What the count says

**41 rules: 12 facts, 13 proof rules, 8 induction principles, 8 semantic theorems.** Three
conclusions follow before any design:

- **A fact declaration would touch 12 rules and must leave 29 alone.** The spec's acceptance test
  (move `seedDivAxioms` out of Go) is F1–F2 only, a sixth of the class.
- **Every fact in the class has one logical shape** (§2.3). No fact here needs a disjunction, a
  quantifier inside a formula, or a relation between two applications of one function.
- **7 of the 12 are interval transfers, 4 are linear facts, 1 is a syntactic rule — and some facts
  appear in several places** (§3).

---

## 2. The algebra: what a fact is

### 2.1 A fact is an axiom schema of a theory extension

Take the analysis's base theory **T₀**: quantifier-free linear integer arithmetic over variables and
**atoms**. An atom is a term the fragment cannot interpret, such as `(len a)`, `(/ x k)` or
`(* i j)`, which `asLinear` turns into a variable named after the whole term (linear.go:422, :438).
The base procedure is `entails`, with P1–P6.

A **fact** extends T₀ with an axiom about an atom's function symbol:

```
    ∀x̄.  G₁(x̄) ∧ … ∧ Gₘ(x̄)  →  C(x̄)
```

The guards `Gᵢ` and the conclusion `C` are linear inequalities over base terms and **extension terms**
`f(x̄)`, where `f` is the symbol T₀ cannot interpret. **T₁ = T₀ ∪ K** for a finite set `K` of such
clauses is a theory extension in exactly the sense of hierarchic reasoning (Sofronie-Stokkermans,
*Hierarchic reasoning in local theory extensions*, CADE 2005).

### 2.2 Locality: why instantiation on present terms is enough

For a set `G` of ground formulas (a query), write `K[G]` for the instances of `K` in which every
extension term already occurs in `G`.

> **Definition (local extension).** `T₀ ⊆ T₀ ∪ K` is **local** if for every ground `G`:
> `T₀ ∪ K ∪ G` is unsatisfiable iff `T₀ ∪ K[G] ∪ G` is unsatisfiable.

For a local extension, deciding `T₁` reduces to deciding `T₀` on finitely many instances. There are
at most `|K| · |terms(G)|^arity` of them, and none creates a term that triggers another, so
**instantiation terminates and has no matching loops**. That is the property Simplify's triggers
approximate heuristically and Z3's quantifier instantiation does not guarantee (Nelson, Detlefs &
Saxe 2005; Leino & Pit-Claudel, *Trigger selection strategies to stabilize program verifiers*, CAV
2016).

**`seedDivAxioms` is `K[G]`** for `K = {F1, F2}` and `G` the whole residual. It walks the term once,
finds every quotient of a length by a positive literal, and adds the two instances for each
(refine.go:99). The code implements hierarchic instantiation already, and the literature supplies
the theorem saying why it is enough.

**Relative completeness, and the honest limit.** Locality makes `K[G]` complete *relative to a
complete decision procedure for T₀*. Ours is deliberately incomplete (refinements.md: an
undischarged obligation is reported, never assumed). So instantiation here is **sound and
terminating**, and **exactly as complete as `entails`** — it adds no incompleteness of its own. That
is the right side of the trade lowstar-lessons.md §3 argues for.

### 2.3 Boundedness axioms are local, and they are all of class F

Ihlemann, Jacobs & Sofronie-Stokkermans (*On local reasoning in verification*, TACAS 2008) prove
locality for several axiom families. The one that matters here is **guarded boundedness**:

```
    ∀x̄.  φ(x̄)  →  s(x̄) ≤ f(x̄) ≤ t(x̄)          with φ, s, t base terms
```

**Every one of F1–F12 has this form** (or is a conjunction of two such clauses), and that is the
second finding the inventory gives:

| fact | the extension term `f(x̄)` | guard `φ` | bounds `s ≤ f ≤ t` |
|---|---|---|---|
| F1, F2 | `x / k` | `k ≥ 1 ∧ x ≥ 0` | `(x − k + 1)/k ≤ q ≤ x/k`, stated as `k·q ≤ x ≤ k·q + k − 1` |
| F3 | `x · x` | — | `x ≤ x·x` |
| F4 | `x · x` | `x·x ≤ e` | `−⌊√e⌋ ≤ x ≤ ⌊√e⌋` — **not** base terms: see below |
| F5 | `a / b` | `b ≥ 1` | `\|a/b\| ≤ \|a\|` (the contraction by `min\|b\|` is a family of instances) |
| F6, F7, F8 | `a % b` | `b ≠ 0` | `−(\|b\|−1) ≤ a%b ≤ \|b\|−1`, and `a ≥ 0 → 0 ≤ a%b` |
| F9 | `x & m` | `m ≥ 0` | `0 ≤ x&m ≤ m` |
| F10 | `x >> k` | `x ≥ 0 ∧ 0 ≤ k ≤ 62` | `x >> k = x / 2ᵏ` — an equation to another atom |
| F11 | `len a` | — | `0 ≤ len a ≤ maxlen_T` |
| F12 | `0 · x` | — | `0 ≤ 0·x ≤ 0` |

**Two are not linear in their bounds, and both are specific.**
- **F4** needs `⌊√e⌋`, which no linear term is. It is used only by the interval layer, where `e` is
  already a constant. As a declared fact it would be `x·x ≤ e → x ≤ e` for `e ≥ 0` (which is F3
  rearranged) plus an interval-only refinement. Either it stays a compiler transfer, or the fact
  language allows a *constant function* of a literal bound. §5 decides.
- **F10** equates two atoms. That is still local, since it is the definition of one extension
  symbol in terms of another applied to the same arguments, but it is a flat equation rather than a
  bound.

**No fact in the class relates two applications of one function**, such as `x ≤ y → f(x) ≤ f(y)`.
Monotonicity axioms are local too (Sofronie-Stokkermans 2005), but they need a trigger of *two*
terms. The class needs none today (§8).

### 2.4 Facts, lemmas and axioms

Why3 separates `axiom` (assumed) from `lemma` (proved, then assumed). So does every serious prover,
and the separation is exactly the trust question of §4:

- **A lemma** is a fact with a proof the system checks.
- **An axiom** is a fact with a proof the system does not see.

In this language no fact is proved *by the compiler* today. `lang`'s facts are proved on paper and
checked by a harness; a host's facts are claims checked by running the host. §4 makes that
distinction the basis of who may declare what.

---

## 3. One law, several encodings — and one declaration that serves all of them

### 3.1 The scattering, located

| law | encodings | consumers |
|---|---|---|
| the remainder bound | `remI` (F6), `storedRange` (F7), `big%-small` transfer (F8) | interval analysis, element narrowing, bignum reports |
| the division bound | `seedDivAxioms` (F1, F2), `divI` (F5) | linear layer, interval layer |
| the square bound | `squareBound` (F3), `narrowSquare` (F4) | linear layer, interval layer |
| interval arithmetic | `addI`/`mulI`, `exactArith` | interval layer, element narrowing |

These are not duplicated by accident. Each consumer is a different abstract domain, and each needed
the law in its own language:
- the linear layer, a relational domain of inequalities;
- the interval layer, a non-relational domain of boxes;
- element narrowing, a syntactic domain of exact ranges.

**The cost is on the record.** CLAUDE.md names *"two layers, and only one of them was ever told"* at
a result range (multiresult-2026-09-06), at `len` (fixpoint-2026-08-27), at the checked spellings
(`arithOp`) and at `&`/`>>` (shiftdiv-2026-09-03). Each was a law added to one encoding and missing
from another.

### 3.2 The principled combination is the reduced product

Two analyses over one program that exchange facts are the **reduced product** of their abstract
domains (Cousot & Cousot, POPL 1979). Its connection to combining decision procedures is exactly the
Nelson–Oppen-style exchange of shared facts (Cousot, Cousot & Mauborgne, *The reduced product of
abstract domains and the combination of decision procedures*, FoSSaCS 2011). The linear and interval
layers are two such domains. What they have been missing is a single *source* for the facts they
both need.

### 3.3 A boundedness fact induces a sound transfer function

> **Theorem T.** Let `K ∋ ∀x̄. φ(x̄) → s(x̄) ≤ f(x̄) ≤ t(x̄)` be true in the intended model `M`.
> Let `X̄` be intervals, and let `s#`, `t#` be the interval evaluations of the base terms `s` and
> `t`. If `φ` holds at every point of `γ(X̄)`, then for every `v̄ ∈ γ(X̄)`:
> `f(v̄) ∈ γ([lo(s#(X̄)), hi(t#(X̄))])`.
>
> *Proof.* Fix `v̄ ∈ γ(X̄)`. Then `M ⊨ φ(v̄)`, so `s(v̄) ≤ f(v̄) ≤ t(v̄)`. Interval evaluation of a base
> term is sound (Moore), so `s(v̄) ≥ lo(s#(X̄))` and `t(v̄) ≤ hi(t#(X̄))`. ∎

Several facts about one symbol give several intervals, and **their meet is sound** because each
contains the value. That is how `remI`'s `min(|a|, |b|−1)` arises: two clauses, one bounding by the
dividend and one by the divisor, met.

Whether `φ` holds on all of `γ(X̄)` is an interval entailment, and `entailsIval` already exists
(interval.go:605).

**So one declaration can serve every consumer:**
- the **linear layer** instantiates it on present terms (§2.2);
- the **interval layer** evaluates its bounds on intervals (Theorem T);
- **element narrowing** evaluates them on exact ranges, which is Theorem T restricted to point
  intervals.

What this costs, and must be measured rather than assumed: a hand-written transfer can be *tighter*
than the one induced from its facts. `divI`'s contraction by `min|b|` is a family of instances, not
one clause. §7's acceptance test compares proof counts, not just emission.

---

## 4. Soundness: who guarantees a fact is true

A fact is a claim, exactly as a `prim` is, and the question is who checks it. The answer depends on
which **model** the fact is about.

| fact about | model | kind | evidence | example |
|---|---|---|---|---|
| `lang`'s operators | the integers inside ADR 0012's window | **lemma** | a proof in the spec, **and** a harness that fails when the fact is weakened | F1–F12 |
| one target | that target's host | **axiom of the model** | a host probe | Java's `max-len` |
| a host function | the host function | **axiom** | an acceptance program against the host | hex: `len (Encode x) = 2·len x` |
| a user's own definition | the definition | **lemma** | checked against the body, as an exported `ensures` already is | a library's own law |

**Consistency needs no separate check, for a structural reason.**

> **Theorem C.** If every fact in scope when building for target `T` is true in one model `M_T`, the
> set is consistent.
>
> *Proof.* `M_T` satisfies all of them. ∎

The hypothesis is what the layering provides. A target's model facts are only in scope when building
for that target (target-system.md §7.2), and `lang`'s facts are true in every target's model because
integers.md measured all four hosts agreeing inside the window. **A consistency failure is therefore
always a single false fact, never a bad combination of true ones**, and the per-fact evidence above is
sufficient.

**Retracting a fact is safe.** A layer overriding a fact with a weaker one can only remove proofs,
and **no proof outlives a build**: every build re-derives everything from the residual. The danger
would be a cache of proved obligations keyed by program, and none exists. If one is ever built, it
must key on the fact set.

---

## 5. Candidates

**F-A. No declared facts; one table in Go.**
Consolidate each law (§3.1) into a single Go definition that all three consumers read.
- Buys: the scattering ends, and no language change is needed.
- Costs: the axioms stay in Go, against this project's own rule that host and language facts are
  data; and a host's facts (hex's length law) still have nowhere to go except `ensures`, with its
  pure-call limitation.

**F-B. Local boundedness facts** — recommended.
A `fact` is a guarded boundedness clause about one extension term, as in §2.3.
- The admission check is syntactic, and each condition is one clause of locality:
  1. **flat** — an extension term's arguments are variables or base terms;
  2. **every variable occurs in the trigger**, so instances are determined by present terms;
  3. **the guard and bounds are linear**, except where §5.1 allows a literal-constant function.
- Consumed by all three domains through §2.2 and Theorem T.
- This is ACL2's **`:linear` rule class**, which is a lemma whose conclusion is a linear inequality,
  indexed by a trigger term (its "max term"), and has been used in ACL2's arithmetic reasoning since
  Boyer & Moore's *Integrating decision procedures into heuristic theorem provers* (1988).
- Buys: all 12 facts, contracts on pure calls (§6), and every host law of the form `f(x) ≤ g(x)`.
- Costs: the admission check, one instantiation pass, and the induced transfers of Theorem T.

**F-C. Guarded Horn clauses with several triggers.**
F-B plus conclusions relating two extension terms, such as monotonicity `x ≤ y → f(x) ≤ f(y)`.
- Still local (Sofronie-Stokkermans 2005), but instantiation is quadratic in present terms, and the
  interval layer cannot use a relation between two unknowns without a relational domain.
- No fact in the inventory needs it. Deferred with that as the trigger.

**F-D. Quantified facts over arrays.**
The array property fragment (Bradley, Manna & Sipma, VMCAI 2006): `∀i. G(i) → C(a[i])`, decidable,
covering initialisation and bounded contents.
- It is what tree.oro's `d` gap needs (frozen-2026-08-28).
- A different fragment with its own decision procedure. Future, and its own research.

**F-E. Lemmas proved by unfolding.**
A fact about a user's `def`, proved by the compiler by unfolding the definition on present terms:
LiquidHaskell's *refinement reflection* with *proof by logical evaluation*, which is complete for
terminating functions (Vazou et al., POPL 2018).
- Staging already inlines definitions, so this may be the exported-`ensures` check generalised
  rather than a new mechanism.
- Future.

**F-F. User-supplied proof rules or rewrite rules.**
Shen's sequent rules, Isabelle's `simp` set, Maude's rewrite rules.
- **Refused.** A proof rule extends class P, which puts undecidability in the type checker
  (decidability-map.md §5).
- A rewrite rule used by reduction is an optimisation law such as η-tab or case-of-case. Its
  confluence and termination would have to be checked, and ADR 0009 requires it never to change an
  answer. The compiler owns these.

### 5.1 The one open design point inside F-B

F4 needs `⌊√e⌋`. The choice is:
- (a) keep `narrowSquare` as a compiler transfer, and declare only F3;
- (b) allow a bound to be a named **compile-time function of literals** — the same grammar as a
  range endpoint (unbounded-rung.md: literals, `+ − ·`, `pow`) extended with `isqrt` — applied only
  where its arguments are literals.

(b) is general. (a) is honest about having one instance. The recommendation is **(a)**, with (b)
named as the extension a second such fact would justify.

---

## 6. Contracts are facts — and that closes two parked limitations

A primitive's contract is `∀x̄. P(x̄) → Q(x̄, f(x̄))` (postconditions.md). **It has F-B's form, with the
application `(f x̄)` as its extension term** whenever `Q` bounds `f(x̄)` by base terms. That is what a
declared result range does (S7), what `(length N)` does (S6), and what hex's
`len (Encode x) = 2·len x` does.

postconditions.md records the limitation this touches:

> *"A pure call is substituted, so it has no binder, and the linear fragment cannot name a general
> application — only a literal, a parameter and `alen`. So `ensures` on a pure prim is carried as an
> opaque atom … The fix — treat a pure application as an opaque linear variable, sound by
> referential transparency — … wants its own measurement."*

**The fix it names is §2.1's atom**, the mechanism `asLinear` already uses for `(/ x k)`
(linear.go:438). A pure application is an extension term named by its printed form, and its contract
is an F-B fact instantiated where the term occurs. So:

- **Contracts on pure calls** become ordinary F-B instantiation. This is still owed its measurement:
  postconditions.md's warning is that naming more atoms can turn *propagated, not proven* into
  *refused*, since a named atom makes an obligation readable that was opaque before.
- **`length` into `ensures`** (spec/theories.md §8.4, gated) is unblocked by the same step.
- **S6, S7 and S8 stop being special mechanisms.** They are F-B facts whose declarations happen to
  sit on a `sig`.

The one condition to keep is Lemma 1: a contract's instance is assumed only where its precondition
was **proven**, not propagated. In F-B terms, a guard `φ` must be entailed, not merely consistent.

---

## 7. Scope and composition

- **A fact is a prop-level declaration of a theory** (spec/theories.md §1.2). It is in scope where
  its module is in scope, and **active** where its trigger occurs.
- **`lang`'s facts ship with the compiler, written as declarations.** A file of `fact`s the loader
  reads, not a Go function per law. That is the eating-its-own-tail step the spec's acceptance test
  names.
- **A target layer may declare model facts** (`max-len`) and **may not declare facts about `lang`'s
  operators.** A target that declared `x·x ≤ 10` would make programs provable on one host and not
  another for a reason that is not about the host. Model facts are allowed because they *are* about
  the host.
- **Glue and override apply as to every declaration** (spec/theories.md §4). §4's retraction argument
  is why override is safe here.
- **A proof that used a target's model fact is not portable**, exactly as a program that used a
  target's primitive is not. Covering already reports the primitive. A fact used only on one target
  should be reported the same way — owed, and small.

---

## 8. The map of what comes later

| extension | fragment | status |
|---|---|---|
| move F1–F12 out of Go | F-B | **the acceptance test** |
| contracts on pure calls | F-B, trigger `(f x̄)` | next after that; measurement owed (§6) |
| `length`/`length-of` as `ensures` | F-B | unblocked by the row above |
| a host law: `len (Encode x) = 2·len x` | F-B, two bounds | free once F-B exists |
| `0 ≤ a ∧ 0 ≤ b → 0 ≤ a·b`, and other product signs | F-B, trigger `a·b` | free |
| `max-len`, `shift-width` as model facts | F-B, trigger `len a` / `x >> k` | free |
| monotone host functions, `x ≤ y → f(x) ≤ f(y)` | F-C | deferred: two-term trigger, no demand |
| initialised or bounded array contents; tree.oro's `d` | F-D, array property fragment | future, its own research |
| laws about a user's `def`, proved by unfolding | F-E, refinement reflection | future |
| termination measures other than size-change (freq's merge sort, `w *= 2`) | class I, not a fact: a declared ranking function like Dafny's `decreases` | future, its own construct |
| acquire/release, SAL resource contracts | linear types (ADR 0018/0020), not facts | elsewhere |
| optimisation laws: η-tab, case-of-case, fusion | compiler-owned rewrites | refused to users (F-F) |
| facts about floats | none | refused: IEEE arithmetic is not an ordered field, and ADR 0009 |
| disjunctive facts beyond `≠` | case splitting | refused: exponential, and P6 already handles the one shape needed |

---

## 9. Recommendation, and the tests that could fail

**F-B**, specified in spec/theories.md §7 from §2.3 and §5, with §5.1's option (a).

**Acceptance, in order, each able to fail:**

1. **Consolidation with no change.** F1–F12 written as `lang` facts and consumed through §2.2 and
   Theorem T; the Go encodings deleted. Pass requires:
   - **every emitted file byte-identical** on four targets;
   - **every per-program proof count identical** — integer operations bounded and loops proven, for
     every example. This is stricter than emission, because a weakened proof shows as a count before
     it shows as a refusal.
2. **The harness can fail.** Delete F2, the upper half of Euclidean division, from the fact file, and
   `emit/multiwhere_test.go`'s hex Decode witness must fail. Weaken F6 to drop the dividend bound, and
   `TestDivisionAndRemainderContain` must fail.
3. **One new capability.** An `ensures` on a pure primitive discharges an obligation that is
   *propagated, not proven* at HEAD. The witness is written first and must fail against HEAD.

**What would refute F-B:** a fact in step 1 whose induced transfer (Theorem T) proves *less* than the
hand-written one on some program, with no clause set that recovers it. That would mean a transfer
function is not always the image of its facts, and §3's single source would need a second encoding
after all.

---

## 10. What is not claimed

- **The inventory is one reading of the code on one day.** Its counts are a classification, and a
  classification is a judgement. The rules are cited by file and line so the reading can be checked.
- **§2.3's claim that all twelve are boundedness axioms is checked here by inspection, not by
  proof.** The literature's locality theorem is for complete base procedures, and §2.2 says what
  that means for an incomplete one.
- **Theorem T's precision cost is unmeasured.** §9's step 1 measures it.
